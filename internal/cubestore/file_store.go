package cubestore

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
	cubeCrashTempPrefix = ".cube-recipes-"
	cubeCrashTempSuffix = ".json"
)

// FileStore persists authored cube recipes beside other bootstrap JSON stores.
type FileStore struct {
	path string
}

var durableSyncDisabledForTest bool

// DisableDurableSyncForTest skips fsync calls until restore runs.
func DisableDurableSyncForTest() func() {
	previous := durableSyncDisabledForTest
	durableSyncDisabledForTest = true
	return func() { durableSyncDisabledForTest = previous }
}

// NewFileStore returns a FileStore rooted at path.
func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

// Path returns the configured snapshot path.
func (s *FileStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *FileStore) syncStoreDir(dir string) error {
	if s == nil || durableSyncDisabledForTest {
		return nil
	}
	return syncStoreDir(dir)
}

func (s *FileStore) syncFile(file *os.File) error {
	if s == nil || durableSyncDisabledForTest {
		return nil
	}
	return file.Sync()
}

var syncStoreDir = syncDir

func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func rejectCommittedSnapshotSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat cube recipe snapshot: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: committed cube recipe snapshot must not be a symlink", ErrInvalidSnapshot)
	}
	return nil
}

// Load reads and validates the committed cube-recipe snapshot.
func (s *FileStore) Load() (Snapshot, error) {
	if s == nil || s.path == "" {
		return Snapshot{}, ErrStorePathRequired
	}
	if err := rejectCommittedSnapshotSymlink(s.path); err != nil {
		return Snapshot{}, err
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Snapshot{}, ErrSnapshotNotFound
		}
		return Snapshot{}, fmt.Errorf("read cube recipe snapshot: %w", err)
	}
	if !utf8.Valid(raw) {
		return Snapshot{}, fmt.Errorf("%w: decode cube recipe snapshot: invalid utf-8", ErrInvalidSnapshot)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return Snapshot{}, fmt.Errorf("%w: decode cube recipe snapshot: null root", ErrInvalidSnapshot)
	}

	var rawSnapshot struct {
		NPCs json.RawMessage `json:"npcs"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rawSnapshot); err != nil {
		return Snapshot{}, fmt.Errorf("%w: decode cube recipe snapshot: %v", ErrInvalidSnapshot, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Snapshot{}, fmt.Errorf("%w: trailing cube recipe snapshot content", ErrInvalidSnapshot)
	}

	var snapshot Snapshot
	if rawSnapshot.NPCs != nil {
		if bytes.Equal(bytes.TrimSpace(rawSnapshot.NPCs), []byte("null")) {
			return Snapshot{}, fmt.Errorf("%w: decode cube recipe snapshot: null npcs collection", ErrInvalidSnapshot)
		}
		collectionDecoder := json.NewDecoder(bytes.NewReader(rawSnapshot.NPCs))
		collectionDecoder.DisallowUnknownFields()
		if err := collectionDecoder.Decode(&snapshot.NPCs); err != nil {
			return Snapshot{}, fmt.Errorf("%w: decode cube recipe snapshot: %v", ErrInvalidSnapshot, err)
		}
		if err := collectionDecoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			return Snapshot{}, fmt.Errorf("%w: trailing cube recipe npcs content", ErrInvalidSnapshot)
		}
	}

	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return Snapshot{}, fmt.Errorf("%w: validate cube recipe snapshot", err)
	}
	return normalized, nil
}

// Validate reports a deterministic authored-recipe summary plus crash-temp residue.
func (s *FileStore) Validate() (SnapshotSummary, error) {
	if s == nil || s.path == "" {
		return SnapshotSummary{}, ErrStorePathRequired
	}
	summary := SnapshotSummary{NPCVnums: []uint32{}}
	snapshot, err := s.Load()
	if err != nil {
		if !errors.Is(err, ErrSnapshotNotFound) {
			return SnapshotSummary{}, err
		}
	} else {
		summary = summarizeSnapshot(snapshot)
	}
	crashTempFiles, err := s.crashTempFiles()
	if err != nil {
		return SnapshotSummary{}, err
	}
	summary.CrashTempCount = len(crashTempFiles)
	summary.CrashTempFiles = crashTempFiles
	if err := s.validateActiveBackupManifest(); err != nil {
		return SnapshotSummary{}, err
	}
	return summary, nil
}

func (s *FileStore) validateActiveBackupManifest() error {
	if s == nil || s.path == "" {
		return ErrStorePathRequired
	}
	storeDir := filepath.Dir(s.path)
	manifestPath := filepath.Join(storeDir, BackupManifestFilename)
	if _, err := os.Lstat(manifestPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat active cube recipe backup manifest: %w", err)
	}
	_, _, hasSnapshot, err := s.validateBackupManifestWithCoverage(storeDir, false)
	if err != nil {
		return err
	}
	if _, err := os.Stat(s.path); err == nil && !hasSnapshot {
		return fmt.Errorf("%w: active manifest omits committed cube recipe snapshot", ErrInvalidBackupManifest)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat active cube recipe snapshot for backup manifest: %w", err)
	}
	return nil
}

// CleanupCrashTempFiles removes same-directory crash temps after Validate succeeds.
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
			return SnapshotSummary{}, fmt.Errorf("remove cube recipe crash temp file %q: %w", filename, err)
		}
	}
	if err := s.syncStoreDir(storeDir); err != nil {
		return SnapshotSummary{}, fmt.Errorf("sync cube recipe store dir after crash temp cleanup: %w", err)
	}
	return s.Validate()
}

func (s *FileStore) crashTempFiles() ([]string, error) {
	return crashTempFilesInDir(filepath.Dir(s.path), filepath.Base(s.path))
}

func crashTempFilesInDir(dir string, committedFilename string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read cube recipe store crash temp files: %w", err)
	}
	files := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if name == committedFilename {
			continue
		}
		if strings.HasPrefix(name, cubeCrashTempPrefix) && strings.HasSuffix(name, cubeCrashTempSuffix) {
			if entry.Type()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("%w: cube recipe crash temp file %q is a symlink", ErrInvalidSnapshot, name)
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

// Save writes the cube-recipe snapshot via crash-temp + rename + fsync.
func (s *FileStore) Save(snapshot Snapshot) error {
	if s == nil || s.path == "" {
		return ErrStorePathRequired
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create cube recipe store dir: %w", err)
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return fmt.Errorf("%w: validate cube recipe snapshot", err)
	}
	raw, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("encode cube recipe snapshot: %w", err)
	}
	raw = append(raw, '\n')

	temp, err := os.CreateTemp(filepath.Dir(s.path), cubeCrashTempPrefix+"*"+cubeCrashTempSuffix)
	if err != nil {
		return fmt.Errorf("create cube recipe temp file: %w", err)
	}
	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()
	if _, err := temp.Write(raw); err != nil {
		return fmt.Errorf("write cube recipe snapshot: %w", err)
	}
	if err := s.syncFile(temp); err != nil {
		return fmt.Errorf("sync cube recipe temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close cube recipe temp file: %w", err)
	}
	storeDir := filepath.Dir(s.path)
	if err := os.Rename(temp.Name(), s.path); err != nil {
		return fmt.Errorf("commit cube recipe snapshot: %w", err)
	}
	if err := removeBackupManifest(storeDir); err != nil {
		return fmt.Errorf("remove stale cube recipe backup manifest: %w", err)
	}
	if err := s.syncStoreDir(storeDir); err != nil {
		return fmt.Errorf("sync cube recipe store dir: %w", err)
	}
	return nil
}

func removeBackupManifest(dir string) error {
	if err := os.Remove(filepath.Join(dir, BackupManifestFilename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// BackupTo copies the committed snapshot into an empty destination with a closed manifest.
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
	if err := ensureEmptyDir(dstDir, ErrBackupDirNotEmpty, "read cube recipe backup dir"); err != nil {
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
		return fmt.Errorf("create cube recipe backup dir: %w", err)
	}
	committedSnapshot := false
	backupPath := filepath.Join(dstDir, filepath.Base(s.path))
	if hasSnapshot {
		backup := NewFileStore(backupPath)
		committedSnapshot = true
		if err := backup.Save(snapshot); err != nil {
			return s.rollbackBackupFailure(dstDir, committedSnapshot, fmt.Errorf("backup cube recipe snapshot: %w", err))
		}
	}
	if err := writeBackupManifest(dstDir, filepath.Base(s.path), summary, hasSnapshot); err != nil {
		return s.rollbackBackupFailure(dstDir, committedSnapshot, err)
	}
	if err := s.syncStoreDir(dstDir); err != nil {
		return s.rollbackBackupFailure(dstDir, committedSnapshot, fmt.Errorf("sync cube recipe backup dir: %w", err))
	}
	return nil
}

func (s *FileStore) backupSourceSnapshot() (SnapshotSummary, Snapshot, bool, error) {
	summary := SnapshotSummary{NPCVnums: []uint32{}}
	snapshot, err := s.Load()
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			return summary, Snapshot{}, false, nil
		}
		return SnapshotSummary{}, Snapshot{}, false, err
	}
	return summarizeSnapshot(snapshot), snapshot, true, nil
}

func (s *FileStore) rollbackBackupFailure(dstDir string, snapshotCommitted bool, backupErr error) error {
	var rollbackErrs []error
	if snapshotCommitted {
		if err := os.Remove(filepath.Join(dstDir, filepath.Base(s.path))); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("remove backup cube recipe snapshot: %w", err))
		}
	}
	if err := os.Remove(filepath.Join(dstDir, BackupManifestFilename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("remove cube recipe backup manifest: %w", err))
	}
	if err := s.syncStoreDir(dstDir); err != nil {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("sync cube recipe backup rollback dir: %w", err))
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
			return fmt.Errorf("read cube recipe backup snapshot for manifest: %w", err)
		}
		checksum := sha256.Sum256(raw)
		manifest.Files = append(manifest.Files, BackupManifestFile{
			Filename:  snapshotFilename,
			SizeBytes: int64(len(raw)),
			SHA256:    hex.EncodeToString(checksum[:]),
		})
	}
	return writeJSONFileAtomically(dir, BackupManifestFilename, manifest, "cube recipe backup manifest")
}

// ValidateBackupFrom dry-runs a closed cube-recipe backup without mutating the live store.
func (s *FileStore) ValidateBackupFrom(srcDir string) (SnapshotSummary, error) {
	if s == nil || s.path == "" {
		return SnapshotSummary{}, ErrStorePathRequired
	}
	summary, _, _, err := s.loadBackupSnapshotForRestore(srcDir)
	if err != nil {
		return SnapshotSummary{}, err
	}
	crashTempFiles, err := crashTempFilesInDir(srcDir, filepath.Base(s.path))
	if err != nil {
		return SnapshotSummary{}, err
	}
	summary.CrashTempCount = len(crashTempFiles)
	summary.CrashTempFiles = crashTempFiles
	return summary, nil
}

// RestoreFrom restores a manifested cube-recipe backup into an empty live store directory.
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
	if err := ensureEmptyDir(storeDir, ErrRestoreDirNotEmpty, "read cube recipe restore dir"); err != nil {
		return err
	}
	summary, snapshot, hasSnapshot, err := s.loadBackupSnapshotForRestore(srcDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		return fmt.Errorf("create cube recipe restore dir: %w", err)
	}

	committedSnapshot := false
	if hasSnapshot {
		if err := s.Save(snapshot); err != nil {
			return s.rollbackRestoreFailure(true, fmt.Errorf("restore cube recipe snapshot: %w", err))
		}
		committedSnapshot = true
	}
	if err := writeBackupManifest(storeDir, filepath.Base(s.path), summary, hasSnapshot); err != nil {
		return s.rollbackRestoreFailure(committedSnapshot, err)
	}
	if err := s.syncStoreDir(storeDir); err != nil {
		return s.rollbackRestoreFailure(committedSnapshot, fmt.Errorf("sync cube recipe restore dir: %w", err))
	}
	return nil
}

func (s *FileStore) rollbackRestoreFailure(snapshotCommitted bool, restoreErr error) error {
	storeDir := filepath.Dir(s.path)
	var rollbackErrs []error
	if snapshotCommitted {
		if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("remove restored cube recipe snapshot: %w", err))
		}
	}
	if err := os.Remove(filepath.Join(storeDir, BackupManifestFilename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("remove restored cube recipe backup manifest: %w", err))
	}
	if err := s.syncStoreDir(storeDir); err != nil {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("sync cube recipe restore rollback dir: %w", err))
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
		return SnapshotSummary{}, Snapshot{}, false, fmt.Errorf("stat cube recipe restore source dir: %w", err)
	}
	return s.validateBackupManifest(srcDir)
}

func (s *FileStore) validateBackupManifest(srcDir string) (SnapshotSummary, Snapshot, bool, error) {
	return s.validateBackupManifestWithCoverage(srcDir, true)
}

func (s *FileStore) validateBackupManifestWithCoverage(srcDir string, requireClosedDirectory bool) (SnapshotSummary, Snapshot, bool, error) {
	manifestPath := filepath.Join(srcDir, BackupManifestFilename)
	if err := rejectBackupEntrySymlink(manifestPath, "cube recipe backup manifest"); err != nil {
		return SnapshotSummary{}, Snapshot{}, false, err
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SnapshotSummary{}, Snapshot{}, false, ErrBackupManifestRequired
		}
		return SnapshotSummary{}, Snapshot{}, false, fmt.Errorf("read cube recipe backup manifest: %w", err)
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
		summary = SnapshotSummary{NPCVnums: []uint32{}}
	} else {
		file := manifest.Files[0]
		if file.Filename == "" || filepath.Base(file.Filename) != file.Filename {
			return SnapshotSummary{}, Snapshot{}, false, fmt.Errorf("%w: manifest filename %q is not a base name", ErrInvalidBackupManifest, file.Filename)
		}
		if file.Filename != filepath.Base(s.path) {
			return SnapshotSummary{}, Snapshot{}, false, fmt.Errorf("%w: manifest filename %q does not match cube recipe snapshot filename", ErrInvalidBackupManifest, file.Filename)
		}
		committedFiles[file.Filename] = struct{}{}
		snapshotPath := filepath.Join(srcDir, file.Filename)
		if err := rejectBackupEntrySymlink(snapshotPath, fmt.Sprintf("cube recipe backup snapshot %q", file.Filename)); err != nil {
			return SnapshotSummary{}, Snapshot{}, false, err
		}
		rawSnapshot, err := os.ReadFile(snapshotPath)
		if err != nil {
			return SnapshotSummary{}, Snapshot{}, false, fmt.Errorf("%w: read manifest cube recipe snapshot: %v", ErrInvalidBackupManifest, err)
		}
		if int64(len(rawSnapshot)) != file.SizeBytes {
			return SnapshotSummary{}, Snapshot{}, false, fmt.Errorf("%w: cube recipe snapshot size mismatch", ErrInvalidBackupManifest)
		}
		checksum := sha256.Sum256(rawSnapshot)
		if got := hex.EncodeToString(checksum[:]); got != file.SHA256 {
			return SnapshotSummary{}, Snapshot{}, false, fmt.Errorf("%w: cube recipe snapshot checksum mismatch", ErrInvalidBackupManifest)
		}
		snapshot, err = NewFileStore(snapshotPath).Load()
		if err != nil {
			return SnapshotSummary{}, Snapshot{}, false, err
		}
		hasSnapshot = true
		summary = summarizeSnapshot(snapshot)
	}
	if !snapshotSummariesEqual(manifest.Summary, summary) {
		return SnapshotSummary{}, Snapshot{}, false, fmt.Errorf("%w: summary does not match committed snapshot", ErrInvalidBackupManifest)
	}
	if requireClosedDirectory {
		if err := validateBackupDirectoryEntries(srcDir, committedFiles); err != nil {
			return SnapshotSummary{}, Snapshot{}, false, err
		}
	}
	return summary, snapshot, hasSnapshot, nil
}

func validateBackupDirectoryEntries(srcDir string, manifestFiles map[string]struct{}) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("read cube recipe backup dir for manifest coverage: %w", err)
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
		if strings.HasPrefix(name, cubeCrashTempPrefix) && strings.HasSuffix(name, cubeCrashTempSuffix) {
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

func snapshotSummariesEqual(a, b SnapshotSummary) bool {
	if a.NPCCount != b.NPCCount || a.RecipeCount != b.RecipeCount || a.CrashTempCount != b.CrashTempCount || len(a.NPCVnums) != len(b.NPCVnums) || len(a.CrashTempFiles) != len(b.CrashTempFiles) {
		return false
	}
	for i := range a.NPCVnums {
		if a.NPCVnums[i] != b.NPCVnums[i] {
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
	temp, err := os.CreateTemp(dir, cubeCrashTempPrefix+"*"+cubeCrashTempSuffix)
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
	return rejectPathInsideOrEqual(storeDir, dstDir, ErrBackupDirInsideStore, "cube recipe store", "cube recipe backup")
}

func rejectRestoreDestinationInsideSource(srcDir string, storeDir string) error {
	return rejectPathInsideOrEqual(srcDir, storeDir, ErrRestoreDirInsideSource, "cube recipe restore source", "cube recipe restore")
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

var _ Store = (*FileStore)(nil)
