package worldruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	BackupManifestFilename = "ground-item-backup-manifest.json"
	BackupManifestFormat   = "go-metin2-ground-item-backup-v1"
)

var (
	ErrBackupDirRequired      = errors.New("ground item backup dir is required")
	ErrBackupDirNotEmpty      = errors.New("ground item backup dir is not empty")
	ErrBackupDirInsideStore   = errors.New("ground item backup dir is inside ground item store")
	ErrRestoreSourceRequired  = errors.New("ground item restore source dir is required")
	ErrRestoreSourceNotFound  = errors.New("ground item restore source dir not found")
	ErrRestoreDirNotEmpty     = errors.New("ground item restore dir is not empty")
	ErrRestoreDirInsideSource = errors.New("ground item restore dir is inside ground item backup source")
	ErrBackupManifestRequired = errors.New("ground item backup manifest is required")
	ErrInvalidBackupManifest  = errors.New("invalid ground item backup manifest")
)

type BackupManifest struct {
	Format  string                           `json:"format"`
	Summary DurableGroundItemSnapshotSummary `json:"summary"`
	Files   []BackupManifestFile             `json:"files"`
}

type BackupManifestFile struct {
	Filename  string `json:"filename"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

func (s *FileStore) BackupTo(dstDir string) error {
	if s == nil || s.path == "" {
		return ErrGroundItemStorePathRequired
	}
	if strings.TrimSpace(dstDir) == "" {
		return ErrBackupDirRequired
	}
	if err := rejectBackupDestinationInsideStore(filepath.Dir(s.path), dstDir); err != nil {
		return err
	}
	if err := ensureEmptyDir(dstDir, ErrBackupDirNotEmpty, "read ground item backup dir"); err != nil {
		return err
	}
	if err := s.validateActiveBackupManifest(); err != nil {
		return err
	}
	if _, err := s.crashTempFiles(); err != nil {
		return err
	}

	summary, snapshot, hasSnapshot, err := s.backupSourceSnapshot()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("create ground item backup dir: %w", err)
	}
	committedSnapshot := false
	backupPath := filepath.Join(dstDir, filepath.Base(s.path))
	if hasSnapshot {
		backup := NewGroundItemFileStore(backupPath)
		committedSnapshot = true
		if err := backup.Save(snapshot); err != nil {
			return s.rollbackBackupFailure(dstDir, committedSnapshot, fmt.Errorf("backup ground item snapshot: %w", err))
		}
	}
	if err := writeBackupManifest(dstDir, filepath.Base(s.path), summary, hasSnapshot); err != nil {
		return s.rollbackBackupFailure(dstDir, committedSnapshot, err)
	}
	if err := syncGroundItemStoreDir(dstDir); err != nil {
		return s.rollbackBackupFailure(dstDir, committedSnapshot, fmt.Errorf("sync ground item backup dir: %w", err))
	}
	return nil
}

func (s *FileStore) backupSourceSnapshot() (DurableGroundItemSnapshotSummary, DurableGroundItemSnapshot, bool, error) {
	summary := DurableGroundItemSnapshotSummary{VIDs: []uint32{}}
	snapshot, err := s.Load()
	if err != nil {
		if errors.Is(err, ErrGroundItemSnapshotNotFound) {
			return summary, DurableGroundItemSnapshot{}, false, nil
		}
		return DurableGroundItemSnapshotSummary{}, DurableGroundItemSnapshot{}, false, err
	}
	return SummarizeDurableGroundItemSnapshot(snapshot), snapshot, true, nil
}

func (s *FileStore) rollbackBackupFailure(dstDir string, snapshotCommitted bool, backupErr error) error {
	var rollbackErrs []error
	if snapshotCommitted {
		if err := os.Remove(filepath.Join(dstDir, filepath.Base(s.path))); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("remove backup ground item snapshot: %w", err))
		}
	}
	if err := os.Remove(filepath.Join(dstDir, BackupManifestFilename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("remove ground item backup manifest: %w", err))
	}
	if err := syncGroundItemStoreDir(dstDir); err != nil {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("sync ground item backup rollback dir: %w", err))
	}
	if len(rollbackErrs) == 0 {
		return backupErr
	}
	return errors.Join(append([]error{backupErr}, rollbackErrs...)...)
}

func writeBackupManifest(dir string, snapshotFilename string, summary DurableGroundItemSnapshotSummary, hasSnapshot bool) error {
	manifest := BackupManifest{
		Format:  BackupManifestFormat,
		Summary: summaryWithoutCrashTemps(summary),
		Files:   []BackupManifestFile{},
	}
	if hasSnapshot {
		raw, err := os.ReadFile(filepath.Join(dir, snapshotFilename))
		if err != nil {
			return fmt.Errorf("read ground item backup snapshot for manifest: %w", err)
		}
		checksum := sha256.Sum256(raw)
		manifest.Files = append(manifest.Files, BackupManifestFile{
			Filename:  snapshotFilename,
			SizeBytes: int64(len(raw)),
			SHA256:    hex.EncodeToString(checksum[:]),
		})
	}
	return writeJSONFileAtomically(dir, BackupManifestFilename, manifest, "ground item backup manifest")
}

func (s *FileStore) ValidateBackupFrom(srcDir string) (DurableGroundItemSnapshotSummary, error) {
	if s == nil || s.path == "" {
		return DurableGroundItemSnapshotSummary{}, ErrGroundItemStorePathRequired
	}
	summary, _, _, err := s.loadBackupSnapshotForRestore(srcDir)
	if err != nil {
		return DurableGroundItemSnapshotSummary{}, err
	}
	crashTempFiles, err := crashTempFilesInDir(srcDir, filepath.Base(s.path))
	if err != nil {
		return DurableGroundItemSnapshotSummary{}, err
	}
	summary.CrashTempCount = len(crashTempFiles)
	summary.CrashTempFiles = crashTempFiles
	return summary, nil
}

// RefreshActiveBackupManifest rewrites the live-store backup manifest from the
// committed snapshot. Used after restore rematerialize rewrites a filtered
// pending set so operators keep a valid restored manifest.
func (s *FileStore) RefreshActiveBackupManifest() error {
	if s == nil || s.path == "" {
		return ErrGroundItemStorePathRequired
	}
	summary, _, hasSnapshot, err := s.backupSourceSnapshot()
	if err != nil {
		return err
	}
	storeDir := filepath.Dir(s.path)
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		return fmt.Errorf("create ground item store dir: %w", err)
	}
	if err := writeBackupManifest(storeDir, filepath.Base(s.path), summary, hasSnapshot); err != nil {
		return err
	}
	if err := syncGroundItemStoreDir(storeDir); err != nil {
		return fmt.Errorf("sync ground item store dir: %w", err)
	}
	return nil
}

func (s *FileStore) RestoreFrom(srcDir string) error {
	if s == nil || s.path == "" {
		return ErrGroundItemStorePathRequired
	}
	if strings.TrimSpace(srcDir) == "" {
		return ErrRestoreSourceRequired
	}
	storeDir := filepath.Dir(s.path)
	if err := rejectRestoreDestinationInsideSource(srcDir, storeDir); err != nil {
		return err
	}
	if err := ensureEmptyDir(storeDir, ErrRestoreDirNotEmpty, "read ground item restore dir"); err != nil {
		return err
	}
	summary, snapshot, hasSnapshot, err := s.loadBackupSnapshotForRestore(srcDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		return fmt.Errorf("create ground item restore dir: %w", err)
	}

	committedSnapshot := false
	if hasSnapshot {
		if err := s.Save(snapshot); err != nil {
			return s.rollbackRestoreFailure(true, fmt.Errorf("restore ground item snapshot: %w", err))
		}
		committedSnapshot = true
	}
	if err := writeBackupManifest(storeDir, filepath.Base(s.path), summary, hasSnapshot); err != nil {
		return s.rollbackRestoreFailure(committedSnapshot, err)
	}
	if err := syncGroundItemStoreDir(storeDir); err != nil {
		return s.rollbackRestoreFailure(committedSnapshot, fmt.Errorf("sync ground item restore dir: %w", err))
	}
	return nil
}

func (s *FileStore) rollbackRestoreFailure(snapshotCommitted bool, restoreErr error) error {
	storeDir := filepath.Dir(s.path)
	var rollbackErrs []error
	if snapshotCommitted {
		if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("remove restored ground item snapshot: %w", err))
		}
	}
	if err := os.Remove(filepath.Join(storeDir, BackupManifestFilename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("remove restored ground item backup manifest: %w", err))
	}
	if err := syncGroundItemStoreDir(storeDir); err != nil {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("sync ground item restore rollback dir: %w", err))
	}
	if len(rollbackErrs) == 0 {
		return restoreErr
	}
	return errors.Join(append([]error{restoreErr}, rollbackErrs...)...)
}

func (s *FileStore) loadBackupSnapshotForRestore(srcDir string) (DurableGroundItemSnapshotSummary, DurableGroundItemSnapshot, bool, error) {
	if strings.TrimSpace(srcDir) == "" {
		return DurableGroundItemSnapshotSummary{}, DurableGroundItemSnapshot{}, false, ErrRestoreSourceRequired
	}
	if _, err := os.Stat(srcDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DurableGroundItemSnapshotSummary{}, DurableGroundItemSnapshot{}, false, ErrRestoreSourceNotFound
		}
		return DurableGroundItemSnapshotSummary{}, DurableGroundItemSnapshot{}, false, fmt.Errorf("stat ground item restore source dir: %w", err)
	}
	return s.validateBackupManifest(srcDir)
}

func (s *FileStore) validateBackupManifest(srcDir string) (DurableGroundItemSnapshotSummary, DurableGroundItemSnapshot, bool, error) {
	return s.validateBackupManifestWithCoverage(srcDir, true)
}

func (s *FileStore) validateActiveBackupManifest() error {
	if s == nil || s.path == "" {
		return ErrGroundItemStorePathRequired
	}
	storeDir := filepath.Dir(s.path)
	manifestPath := filepath.Join(storeDir, BackupManifestFilename)
	if _, err := os.Lstat(manifestPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat active ground item backup manifest: %w", err)
	}
	_, _, hasSnapshot, err := s.validateBackupManifestWithCoverage(storeDir, false)
	if err != nil {
		return err
	}
	if _, err := os.Stat(s.path); err == nil && !hasSnapshot {
		return fmt.Errorf("%w: active manifest omits committed ground item snapshot", ErrInvalidBackupManifest)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat active ground item snapshot for backup manifest: %w", err)
	}
	return nil
}

func (s *FileStore) validateBackupManifestWithCoverage(srcDir string, requireClosedDirectory bool) (DurableGroundItemSnapshotSummary, DurableGroundItemSnapshot, bool, error) {
	manifestPath := filepath.Join(srcDir, BackupManifestFilename)
	if err := rejectBackupEntrySymlink(manifestPath, "ground item backup manifest"); err != nil {
		return DurableGroundItemSnapshotSummary{}, DurableGroundItemSnapshot{}, false, err
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DurableGroundItemSnapshotSummary{}, DurableGroundItemSnapshot{}, false, ErrBackupManifestRequired
		}
		return DurableGroundItemSnapshotSummary{}, DurableGroundItemSnapshot{}, false, fmt.Errorf("read ground item backup manifest: %w", err)
	}
	var manifest BackupManifest
	if err := decodeBackupManifestStrict(raw, &manifest); err != nil {
		return DurableGroundItemSnapshotSummary{}, DurableGroundItemSnapshot{}, false, fmt.Errorf("%w: decode manifest: %v", ErrInvalidBackupManifest, err)
	}
	if manifest.Format != BackupManifestFormat {
		return DurableGroundItemSnapshotSummary{}, DurableGroundItemSnapshot{}, false, fmt.Errorf("%w: format %q", ErrInvalidBackupManifest, manifest.Format)
	}
	if len(manifest.Files) > 1 {
		return DurableGroundItemSnapshotSummary{}, DurableGroundItemSnapshot{}, false, fmt.Errorf("%w: manifest lists %d snapshot files", ErrInvalidBackupManifest, len(manifest.Files))
	}

	committedFiles := make(map[string]struct{}, len(manifest.Files))
	var summary DurableGroundItemSnapshotSummary
	var snapshot DurableGroundItemSnapshot
	hasSnapshot := false
	if len(manifest.Files) == 0 {
		summary = DurableGroundItemSnapshotSummary{VIDs: []uint32{}}
	} else {
		file := manifest.Files[0]
		if file.Filename == "" || filepath.Base(file.Filename) != file.Filename {
			return DurableGroundItemSnapshotSummary{}, DurableGroundItemSnapshot{}, false, fmt.Errorf("%w: manifest filename %q is not a base name", ErrInvalidBackupManifest, file.Filename)
		}
		if file.Filename != filepath.Base(s.path) {
			return DurableGroundItemSnapshotSummary{}, DurableGroundItemSnapshot{}, false, fmt.Errorf("%w: manifest filename %q does not match ground item snapshot filename", ErrInvalidBackupManifest, file.Filename)
		}
		committedFiles[file.Filename] = struct{}{}
		snapshotPath := filepath.Join(srcDir, file.Filename)
		if err := rejectBackupEntrySymlink(snapshotPath, fmt.Sprintf("ground item backup snapshot %q", file.Filename)); err != nil {
			return DurableGroundItemSnapshotSummary{}, DurableGroundItemSnapshot{}, false, err
		}
		rawDurableGroundItemSnapshot, err := os.ReadFile(snapshotPath)
		if err != nil {
			return DurableGroundItemSnapshotSummary{}, DurableGroundItemSnapshot{}, false, fmt.Errorf("%w: read manifest ground item snapshot: %v", ErrInvalidBackupManifest, err)
		}
		if int64(len(rawDurableGroundItemSnapshot)) != file.SizeBytes {
			return DurableGroundItemSnapshotSummary{}, DurableGroundItemSnapshot{}, false, fmt.Errorf("%w: ground item snapshot size mismatch", ErrInvalidBackupManifest)
		}
		checksum := sha256.Sum256(rawDurableGroundItemSnapshot)
		if got := hex.EncodeToString(checksum[:]); got != file.SHA256 {
			return DurableGroundItemSnapshotSummary{}, DurableGroundItemSnapshot{}, false, fmt.Errorf("%w: ground item snapshot checksum mismatch", ErrInvalidBackupManifest)
		}
		snapshot, err = NewGroundItemFileStore(snapshotPath).Load()
		if err != nil {
			return DurableGroundItemSnapshotSummary{}, DurableGroundItemSnapshot{}, false, err
		}
		hasSnapshot = true
		summary = SummarizeDurableGroundItemSnapshot(snapshot)
	}
	if !snapshotSummariesEqual(manifest.Summary, summary) {
		return DurableGroundItemSnapshotSummary{}, DurableGroundItemSnapshot{}, false, fmt.Errorf("%w: summary does not match committed snapshot", ErrInvalidBackupManifest)
	}
	if requireClosedDirectory {
		if err := validateBackupDirectoryEntries(srcDir, committedFiles); err != nil {
			return DurableGroundItemSnapshotSummary{}, DurableGroundItemSnapshot{}, false, err
		}
	}
	return summary, snapshot, hasSnapshot, nil
}

func validateBackupDirectoryEntries(srcDir string, manifestFiles map[string]struct{}) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("read ground item backup dir for manifest coverage: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == BackupManifestFilename {
			continue
		}
		if _, ok := manifestFiles[name]; ok {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: backup contains symlink entry %q", ErrInvalidBackupManifest, name)
		}
		if entry.IsDir() {
			return fmt.Errorf("%w: backup contains untracked directory %q", ErrInvalidBackupManifest, name)
		}
		if strings.HasPrefix(name, ".ground-items-") && strings.HasSuffix(name, ".json") {
			continue
		}
		return fmt.Errorf("%w: backup contains untracked entry %q", ErrInvalidBackupManifest, name)
	}
	return nil
}

func rejectBackupEntrySymlink(path string, context string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s is a symlink", ErrInvalidBackupManifest, context)
	}
	return nil
}

func decodeBackupManifestStrict(raw []byte, manifest *BackupManifest) error {
	if !utf8.Valid(raw) {
		return errors.New("invalid utf-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(manifest); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}

func summaryWithoutCrashTemps(summary DurableGroundItemSnapshotSummary) DurableGroundItemSnapshotSummary {
	summary.CrashTempCount = 0
	summary.CrashTempFiles = nil
	if summary.VIDs == nil {
		summary.VIDs = []uint32{}
	}
	return summary
}

func snapshotSummariesEqual(a, b DurableGroundItemSnapshotSummary) bool {
	a = summaryWithoutCrashTemps(a)
	b = summaryWithoutCrashTemps(b)
	if a.GroundItemCount != b.GroundItemCount || a.ItemShapedCount != b.ItemShapedCount || a.GoldShapedCount != b.GoldShapedCount {
		return false
	}
	if len(a.VIDs) != len(b.VIDs) {
		return false
	}
	for i := range a.VIDs {
		if a.VIDs[i] != b.VIDs[i] {
			return false
		}
	}
	return true
}

func writeJSONFileAtomically(dir, filename string, value any, context string) error {
	temp, err := os.CreateTemp(dir, ".ground-items-*.json")
	if err != nil {
		return fmt.Errorf("create %s temp file: %w", context, err)
	}
	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()
	encoder := json.NewEncoder(temp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode %s: %w", context, err)
	}
	if !durableGroundItemSyncDisabledForTest {
		if err := temp.Sync(); err != nil {
			return fmt.Errorf("sync %s temp file: %w", context, err)
		}
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close %s temp file: %w", context, err)
	}
	if err := os.Rename(temp.Name(), filepath.Join(dir, filename)); err != nil {
		return fmt.Errorf("commit %s file: %w", context, err)
	}
	return nil
}

func ensureEmptyDir(path string, nonEmptyErr error, readContext string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("%s: %w", readContext, err)
	}
	if len(entries) != 0 {
		return nonEmptyErr
	}
	return nil
}

func rejectBackupDestinationInsideStore(storeDir string, dstDir string) error {
	return rejectPathInsideOrEqual(storeDir, dstDir, ErrBackupDirInsideStore, "ground item store", "ground item backup")
}

func rejectRestoreDestinationInsideSource(srcDir string, storeDir string) error {
	return rejectPathInsideOrEqual(srcDir, storeDir, ErrRestoreDirInsideSource, "ground item restore source", "ground item restore")
}

func rejectPathInsideOrEqual(root string, candidate string, rejectedErr error, rootContext string, candidateContext string) error {
	rootPath, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return fmt.Errorf("resolve %s dir: %w", rootContext, err)
	}
	candidatePath, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return fmt.Errorf("resolve %s dir: %w", candidateContext, err)
	}
	inside, err := pathInsideOrEqual(rootPath, candidatePath)
	if err != nil {
		return fmt.Errorf("compare %s dir: %w", candidateContext, err)
	}
	if inside {
		return rejectedErr
	}

	resolvedRootPath, err := resolveExistingPath(rootPath)
	if err != nil {
		return fmt.Errorf("resolve %s symlinks: %w", rootContext, err)
	}
	resolvedCandidatePath, err := resolveExistingPath(candidatePath)
	if err != nil {
		return fmt.Errorf("resolve %s symlinks: %w", candidateContext, err)
	}
	inside, err = pathInsideOrEqual(resolvedRootPath, resolvedCandidatePath)
	if err != nil {
		return fmt.Errorf("compare resolved %s dir: %w", candidateContext, err)
	}
	if inside {
		return rejectedErr
	}
	return nil
}

func pathInsideOrEqual(root string, candidate string) (bool, error) {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false, err
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))), nil
}

func resolveExistingPath(path string) (string, error) {
	path, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	for range 255 {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			return filepath.Abs(filepath.Clean(resolved))
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		info, lstatErr := os.Lstat(path)
		if lstatErr == nil && info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return "", err
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(path), target)
			}
			path = filepath.Clean(target)
			continue
		}
		if lstatErr != nil && !errors.Is(lstatErr, os.ErrNotExist) {
			return "", lstatErr
		}
		parent := filepath.Dir(path)
		if parent == path {
			return filepath.Abs(filepath.Clean(path))
		}
		parentResolved, err := resolveExistingPath(parent)
		if err != nil {
			return "", err
		}
		return filepath.Abs(filepath.Clean(filepath.Join(parentResolved, filepath.Base(path))))
	}
	return "", errors.New("too many symlinks while resolving path")
}

func removeBackupManifest(dir string) error {
	if err := os.Remove(filepath.Join(dir, BackupManifestFilename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func crashTempFilesInDir(dir string, committedFilename string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read ground item store crash temp files: %w", err)
	}
	files := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if name == committedFilename {
			continue
		}
		if strings.HasPrefix(name, ".ground-items-") && strings.HasSuffix(name, ".json") {
			if entry.Type()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("%w: ground item crash temp file %q is a symlink", ErrInvalidGroundItemSnapshot, name)
			}
			if entry.IsDir() {
				continue
			}
			files = append(files, name)
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, nil
	}
	return files, nil
}
