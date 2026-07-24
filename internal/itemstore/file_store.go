package itemstore

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
)

type FileStore struct {
	path string
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func (s *FileStore) Load() (Snapshot, error) {
	if s == nil || s.path == "" {
		return Snapshot{}, ErrStorePathRequired
	}

	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Snapshot{}, ErrSnapshotNotFound
		}
		return Snapshot{}, fmt.Errorf("read item template snapshot: %w", err)
	}

	var snapshot Snapshot
	if err := decodeSnapshotStrict(raw, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("%w: decode item template snapshot: %v", ErrInvalidSnapshot, err)
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return Snapshot{}, fmt.Errorf("%w: validate item template snapshot", err)
	}
	return normalized, nil
}

func (s *FileStore) Validate() (SnapshotSummary, error) {
	if s == nil || s.path == "" {
		return SnapshotSummary{}, ErrStorePathRequired
	}
	summary := SnapshotSummary{Vnums: []uint32{}}
	snapshot, err := s.Load()
	if err != nil {
		if !errors.Is(err, ErrSnapshotNotFound) {
			return SnapshotSummary{}, err
		}
	} else {
		summary.TemplateCount = len(snapshot.Templates)
		summary.Vnums = make([]uint32, 0, len(snapshot.Templates))
		for _, template := range snapshot.Templates {
			summary.Vnums = append(summary.Vnums, template.Vnum)
		}
	}
	crashTempFiles, err := s.crashTempFiles()
	if err != nil {
		return SnapshotSummary{}, err
	}
	summary.CrashTempCount = len(crashTempFiles)
	summary.CrashTempFiles = crashTempFiles
	return summary, nil
}

func (s *FileStore) CleanupCrashTempFiles() (SnapshotSummary, error) {
	if s == nil || s.path == "" {
		return SnapshotSummary{}, ErrStorePathRequired
	}
	if _, err := s.Validate(); err != nil {
		return SnapshotSummary{}, err
	}
	crashTempFiles, err := s.crashTempFiles()
	if err != nil {
		return SnapshotSummary{}, err
	}
	if len(crashTempFiles) == 0 {
		return s.Validate()
	}
	storeDir := filepath.Dir(s.path)
	for _, filename := range crashTempFiles {
		if err := os.Remove(filepath.Join(storeDir, filename)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return SnapshotSummary{}, fmt.Errorf("remove item template crash temp file %q: %w", filename, err)
		}
	}
	if err := syncDir(storeDir); err != nil {
		return SnapshotSummary{}, fmt.Errorf("sync item template store dir after crash temp cleanup: %w", err)
	}
	return s.Validate()
}

func (s *FileStore) crashTempFiles() ([]string, error) {
	entries, err := os.ReadDir(filepath.Dir(s.path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read item template store crash temp files: %w", err)
	}
	files := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == filepath.Base(s.path) {
			continue
		}
		if strings.HasPrefix(name, ".item-templates-") && strings.HasSuffix(name, ".json") {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, nil
	}
	return files, nil
}

func (s *FileStore) Save(snapshot Snapshot) error {
	if s == nil || s.path == "" {
		return ErrStorePathRequired
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create item template store dir: %w", err)
	}

	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return fmt.Errorf("%w: validate item template snapshot", err)
	}
	raw, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("encode item template snapshot: %w", err)
	}
	raw = append(raw, '\n')

	temp, err := os.CreateTemp(filepath.Dir(s.path), ".item-templates-*.json")
	if err != nil {
		return fmt.Errorf("create item template temp file: %w", err)
	}
	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()

	if _, err := temp.Write(raw); err != nil {
		return fmt.Errorf("write item template snapshot: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync item template temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close item template temp file: %w", err)
	}
	if err := os.Rename(temp.Name(), s.path); err != nil {
		return fmt.Errorf("commit item template snapshot: %w", err)
	}
	if err := syncDir(filepath.Dir(s.path)); err != nil {
		return fmt.Errorf("sync item template store dir: %w", err)
	}
	return nil
}

func (s *FileStore) BackupTo(dstDir string) error {
	if s == nil || s.path == "" {
		return ErrStorePathRequired
	}
	if strings.TrimSpace(dstDir) == "" {
		return ErrBackupDirRequired
	}
	if err := rejectBackupDestinationInsideStore(filepath.Dir(s.path), dstDir); err != nil {
		return err
	}
	if err := ensureEmptyDir(dstDir, ErrBackupDirNotEmpty, "read item template backup dir"); err != nil {
		return err
	}

	summary, snapshot, hasSnapshot, err := s.backupSourceSnapshot()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("create item template backup dir: %w", err)
	}
	committedSnapshot := false
	backupPath := filepath.Join(dstDir, filepath.Base(s.path))
	if hasSnapshot {
		backup := NewFileStore(backupPath)
		if err := backup.Save(snapshot); err != nil {
			return s.rollbackBackupFailure(dstDir, committedSnapshot, fmt.Errorf("backup item template snapshot: %w", err))
		}
		committedSnapshot = true
	}
	if err := writeBackupManifest(dstDir, filepath.Base(s.path), summary, hasSnapshot); err != nil {
		return s.rollbackBackupFailure(dstDir, committedSnapshot, err)
	}
	if err := syncDir(dstDir); err != nil {
		return s.rollbackBackupFailure(dstDir, committedSnapshot, fmt.Errorf("sync item template backup dir: %w", err))
	}
	return nil
}

func (s *FileStore) backupSourceSnapshot() (SnapshotSummary, Snapshot, bool, error) {
	summary := SnapshotSummary{Vnums: []uint32{}}
	snapshot, err := s.Load()
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			return summary, Snapshot{}, false, nil
		}
		return SnapshotSummary{}, Snapshot{}, false, err
	}
	summary.TemplateCount = len(snapshot.Templates)
	summary.Vnums = make([]uint32, 0, len(snapshot.Templates))
	for _, template := range snapshot.Templates {
		summary.Vnums = append(summary.Vnums, template.Vnum)
	}
	return summary, snapshot, true, nil
}

func (s *FileStore) rollbackBackupFailure(dstDir string, snapshotCommitted bool, backupErr error) error {
	var rollbackErrs []error
	if snapshotCommitted {
		if err := os.Remove(filepath.Join(dstDir, filepath.Base(s.path))); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("remove backup item template snapshot: %w", err))
		}
	}
	if err := os.Remove(filepath.Join(dstDir, BackupManifestFilename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("remove item template backup manifest: %w", err))
	}
	if err := syncDir(dstDir); err != nil {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("sync item template backup rollback dir: %w", err))
	}
	if len(rollbackErrs) == 0 {
		return backupErr
	}
	return errors.Join(append([]error{backupErr}, rollbackErrs...)...)
}

func writeBackupManifest(dir string, snapshotFilename string, summary SnapshotSummary, hasSnapshot bool) error {
	manifest := BackupManifest{
		Format:  BackupManifestFormat,
		Summary: summary,
		Files:   []BackupManifestFile{},
	}
	if hasSnapshot {
		raw, err := os.ReadFile(filepath.Join(dir, snapshotFilename))
		if err != nil {
			return fmt.Errorf("read item template backup snapshot for manifest: %w", err)
		}
		checksum := sha256.Sum256(raw)
		manifest.Files = append(manifest.Files, BackupManifestFile{
			Filename:  snapshotFilename,
			SizeBytes: int64(len(raw)),
			SHA256:    hex.EncodeToString(checksum[:]),
		})
	}
	return writeJSONFileAtomically(dir, BackupManifestFilename, manifest, "item template backup manifest")
}

func (s *FileStore) ValidateBackupFrom(srcDir string) (SnapshotSummary, error) {
	if s == nil || s.path == "" {
		return SnapshotSummary{}, ErrStorePathRequired
	}
	summary, _, _, err := s.loadBackupSnapshotForRestore(srcDir)
	return summary, err
}

func (s *FileStore) RestoreFrom(srcDir string) error {
	if s == nil || s.path == "" {
		return ErrStorePathRequired
	}
	if strings.TrimSpace(srcDir) == "" {
		return ErrRestoreSourceRequired
	}
	storeDir := filepath.Dir(s.path)
	if err := rejectRestoreDestinationInsideSource(srcDir, storeDir); err != nil {
		return err
	}
	if err := ensureEmptyDir(storeDir, ErrRestoreDirNotEmpty, "read item template restore dir"); err != nil {
		return err
	}
	summary, snapshot, hasSnapshot, err := s.loadBackupSnapshotForRestore(srcDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		return fmt.Errorf("create item template restore dir: %w", err)
	}

	committedSnapshot := false
	if hasSnapshot {
		if err := s.Save(snapshot); err != nil {
			return s.rollbackRestoreFailure(true, fmt.Errorf("restore item template snapshot: %w", err))
		}
		committedSnapshot = true
	}
	if err := writeBackupManifest(storeDir, filepath.Base(s.path), summary, hasSnapshot); err != nil {
		return s.rollbackRestoreFailure(committedSnapshot, err)
	}
	if err := syncDir(storeDir); err != nil {
		return s.rollbackRestoreFailure(committedSnapshot, fmt.Errorf("sync item template restore dir: %w", err))
	}
	return nil
}

func (s *FileStore) rollbackRestoreFailure(snapshotCommitted bool, restoreErr error) error {
	storeDir := filepath.Dir(s.path)
	var rollbackErrs []error
	if snapshotCommitted {
		if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("remove restored item template snapshot: %w", err))
		}
	}
	if err := os.Remove(filepath.Join(storeDir, BackupManifestFilename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("remove restored item template backup manifest: %w", err))
	}
	if err := syncDir(storeDir); err != nil {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("sync item template restore rollback dir: %w", err))
	}
	if len(rollbackErrs) == 0 {
		return restoreErr
	}
	return errors.Join(append([]error{restoreErr}, rollbackErrs...)...)
}

func (s *FileStore) loadBackupSnapshotForRestore(srcDir string) (SnapshotSummary, Snapshot, bool, error) {
	if strings.TrimSpace(srcDir) == "" {
		return SnapshotSummary{}, Snapshot{}, false, ErrRestoreSourceRequired
	}
	if _, err := os.Stat(srcDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SnapshotSummary{}, Snapshot{}, false, ErrRestoreSourceNotFound
		}
		return SnapshotSummary{}, Snapshot{}, false, fmt.Errorf("stat item template restore source dir: %w", err)
	}
	return s.validateBackupManifest(srcDir)
}

func (s *FileStore) validateBackupManifest(srcDir string) (SnapshotSummary, Snapshot, bool, error) {
	manifestPath := filepath.Join(srcDir, BackupManifestFilename)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SnapshotSummary{}, Snapshot{}, false, ErrBackupManifestRequired
		}
		return SnapshotSummary{}, Snapshot{}, false, fmt.Errorf("read item template backup manifest: %w", err)
	}
	var manifest BackupManifest
	if err := decodeBackupManifestStrict(raw, &manifest); err != nil {
		return SnapshotSummary{}, Snapshot{}, false, fmt.Errorf("%w: decode manifest: %v", ErrInvalidBackupManifest, err)
	}
	if manifest.Format != BackupManifestFormat {
		return SnapshotSummary{}, Snapshot{}, false, fmt.Errorf("%w: format %q", ErrInvalidBackupManifest, manifest.Format)
	}
	if len(manifest.Files) > 1 {
		return SnapshotSummary{}, Snapshot{}, false, fmt.Errorf("%w: manifest lists %d snapshot files", ErrInvalidBackupManifest, len(manifest.Files))
	}

	committedFiles := make(map[string]struct{}, len(manifest.Files))
	var summary SnapshotSummary
	var snapshot Snapshot
	hasSnapshot := false
	if len(manifest.Files) == 0 {
		summary = SnapshotSummary{Vnums: []uint32{}}
	} else {
		file := manifest.Files[0]
		if file.Filename == "" || filepath.Base(file.Filename) != file.Filename {
			return SnapshotSummary{}, Snapshot{}, false, fmt.Errorf("%w: manifest filename %q is not a base name", ErrInvalidBackupManifest, file.Filename)
		}
		if file.Filename != filepath.Base(s.path) {
			return SnapshotSummary{}, Snapshot{}, false, fmt.Errorf("%w: manifest filename %q does not match item template snapshot filename", ErrInvalidBackupManifest, file.Filename)
		}
		committedFiles[file.Filename] = struct{}{}
		snapshotPath := filepath.Join(srcDir, file.Filename)
		rawSnapshot, err := os.ReadFile(snapshotPath)
		if err != nil {
			return SnapshotSummary{}, Snapshot{}, false, fmt.Errorf("%w: read manifest item template snapshot: %v", ErrInvalidBackupManifest, err)
		}
		if int64(len(rawSnapshot)) != file.SizeBytes {
			return SnapshotSummary{}, Snapshot{}, false, fmt.Errorf("%w: item template snapshot size mismatch", ErrInvalidBackupManifest)
		}
		checksum := sha256.Sum256(rawSnapshot)
		if got := hex.EncodeToString(checksum[:]); got != file.SHA256 {
			return SnapshotSummary{}, Snapshot{}, false, fmt.Errorf("%w: item template snapshot checksum mismatch", ErrInvalidBackupManifest)
		}
		snapshot, err = NewFileStore(snapshotPath).Load()
		if err != nil {
			return SnapshotSummary{}, Snapshot{}, false, err
		}
		hasSnapshot = true
		summary = SnapshotSummary{TemplateCount: len(snapshot.Templates), Vnums: make([]uint32, 0, len(snapshot.Templates))}
		for _, template := range snapshot.Templates {
			summary.Vnums = append(summary.Vnums, template.Vnum)
		}
	}
	if !snapshotSummariesEqual(manifest.Summary, summary) {
		return SnapshotSummary{}, Snapshot{}, false, fmt.Errorf("%w: summary does not match committed snapshot", ErrInvalidBackupManifest)
	}
	if err := validateBackupDirectoryEntries(srcDir, committedFiles); err != nil {
		return SnapshotSummary{}, Snapshot{}, false, err
	}
	return summary, snapshot, hasSnapshot, nil
}

func validateBackupDirectoryEntries(srcDir string, manifestFiles map[string]struct{}) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("read item template backup dir for manifest coverage: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == BackupManifestFilename {
			continue
		}
		if _, ok := manifestFiles[name]; ok {
			continue
		}
		if entry.IsDir() {
			return fmt.Errorf("%w: backup contains untracked directory %q", ErrInvalidBackupManifest, name)
		}
		if strings.HasPrefix(name, ".item-templates-") && strings.HasSuffix(name, ".json") {
			continue
		}
		return fmt.Errorf("%w: backup contains untracked entry %q", ErrInvalidBackupManifest, name)
	}
	return nil
}

func decodeSnapshotStrict(raw []byte, snapshot *Snapshot) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(snapshot); err != nil {
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

func decodeBackupManifestStrict(raw []byte, manifest *BackupManifest) error {
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

func snapshotSummariesEqual(a, b SnapshotSummary) bool {
	if a.TemplateCount != b.TemplateCount || a.CrashTempCount != b.CrashTempCount || len(a.Vnums) != len(b.Vnums) || len(a.CrashTempFiles) != len(b.CrashTempFiles) {
		return false
	}
	for i := range a.Vnums {
		if a.Vnums[i] != b.Vnums[i] {
			return false
		}
	}
	for i := range a.CrashTempFiles {
		if a.CrashTempFiles[i] != b.CrashTempFiles[i] {
			return false
		}
	}
	return true
}

func writeJSONFileAtomically(dir, filename string, value any, context string) error {
	temp, err := os.CreateTemp(dir, ".item-templates-*.json")
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
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync %s temp file: %w", context, err)
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
	return rejectPathInsideOrEqual(storeDir, dstDir, ErrBackupDirInsideStore, "item template store", "item template backup")
}

func rejectRestoreDestinationInsideSource(srcDir string, storeDir string) error {
	return rejectPathInsideOrEqual(srcDir, storeDir, ErrRestoreDirInsideSource, "item template restore source", "item template restore")
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

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
