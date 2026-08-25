package cubestore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	return syncDir(dir)
}

func (s *FileStore) syncFile(file *os.File) error {
	if s == nil || durableSyncDisabledForTest {
		return nil
	}
	return file.Sync()
}

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
	if err := os.Rename(temp.Name(), s.path); err != nil {
		return fmt.Errorf("commit cube recipe snapshot: %w", err)
	}
	if err := s.syncStoreDir(filepath.Dir(s.path)); err != nil {
		return fmt.Errorf("sync cube recipe store dir: %w", err)
	}
	return nil
}

var _ Store = (*FileStore)(nil)
