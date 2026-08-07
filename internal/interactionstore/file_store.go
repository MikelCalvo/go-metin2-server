package interactionstore

import (
	"bytes"
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

type FileStore struct {
	path string
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func (s *FileStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

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
		return Snapshot{}, fmt.Errorf("read interaction snapshot: %w", err)
	}
	if !utf8.Valid(raw) {
		return Snapshot{}, fmt.Errorf("%w: decode interaction snapshot: invalid utf-8", ErrInvalidSnapshot)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return Snapshot{}, fmt.Errorf("%w: decode interaction snapshot: null root", ErrInvalidSnapshot)
	}

	var rawSnapshot struct {
		Definitions json.RawMessage `json:"definitions"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rawSnapshot); err != nil {
		return Snapshot{}, fmt.Errorf("%w: decode interaction snapshot: %v", ErrInvalidSnapshot, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Snapshot{}, fmt.Errorf("%w: trailing interaction snapshot content", ErrInvalidSnapshot)
	}
	var snapshot Snapshot
	if rawSnapshot.Definitions != nil {
		if bytes.Equal(bytes.TrimSpace(rawSnapshot.Definitions), []byte("null")) {
			return Snapshot{}, fmt.Errorf("%w: decode interaction snapshot: null definitions collection", ErrInvalidSnapshot)
		}
		collectionDecoder := json.NewDecoder(bytes.NewReader(rawSnapshot.Definitions))
		collectionDecoder.DisallowUnknownFields()
		if err := collectionDecoder.Decode(&snapshot.Definitions); err != nil {
			return Snapshot{}, fmt.Errorf("%w: decode interaction snapshot: %v", ErrInvalidSnapshot, err)
		}
		if err := collectionDecoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			return Snapshot{}, fmt.Errorf("%w: trailing interaction definitions content", ErrInvalidSnapshot)
		}
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return Snapshot{}, fmt.Errorf("%w: validate interaction snapshot", err)
	}
	return normalized, nil
}

func (s *FileStore) Validate() (SnapshotSummary, error) {
	if s == nil || s.path == "" {
		return SnapshotSummary{}, ErrStorePathRequired
	}
	summary := SnapshotSummary{DefinitionKeys: []string{}}
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
			return SnapshotSummary{}, fmt.Errorf("remove interaction crash temp file %q: %w", filename, err)
		}
	}
	if err := syncDir(storeDir); err != nil {
		return SnapshotSummary{}, fmt.Errorf("sync interaction store dir after crash temp cleanup: %w", err)
	}
	return s.Validate()
}

func summarizeSnapshot(snapshot Snapshot) SnapshotSummary {
	summary := SnapshotSummary{
		DefinitionCount: len(snapshot.Definitions),
		DefinitionKeys:  make([]string, 0, len(snapshot.Definitions)),
	}
	for _, definition := range snapshot.Definitions {
		summary.DefinitionKeys = append(summary.DefinitionKeys, definition.Kind+":"+definition.Ref)
	}
	return summary
}

func (s *FileStore) crashTempFiles() ([]string, error) {
	entries, err := os.ReadDir(filepath.Dir(s.path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read interaction store crash temp files: %w", err)
	}
	files := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if name == filepath.Base(s.path) {
			continue
		}
		if strings.HasPrefix(name, ".interaction-definitions-") && strings.HasSuffix(name, ".json") {
			if entry.Type()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("%w: interaction crash temp file %q is a symlink", ErrInvalidSnapshot, name)
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

func (s *FileStore) Save(snapshot Snapshot) error {
	if s == nil || s.path == "" {
		return ErrStorePathRequired
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create interaction store dir: %w", err)
	}

	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return fmt.Errorf("%w: validate interaction snapshot", err)
	}
	raw, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("encode interaction snapshot: %w", err)
	}
	raw = append(raw, '\n')

	temp, err := os.CreateTemp(filepath.Dir(s.path), ".interaction-definitions-*.json")
	if err != nil {
		return fmt.Errorf("create interaction temp file: %w", err)
	}
	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()

	if _, err := temp.Write(raw); err != nil {
		return fmt.Errorf("write interaction snapshot: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync interaction temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close interaction temp file: %w", err)
	}
	if err := os.Rename(temp.Name(), s.path); err != nil {
		return fmt.Errorf("commit interaction snapshot: %w", err)
	}
	if err := syncDir(filepath.Dir(s.path)); err != nil {
		return fmt.Errorf("sync interaction store dir: %w", err)
	}
	return nil
}

func rejectCommittedSnapshotSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat interaction snapshot: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: interaction snapshot %q is a symlink", ErrInvalidSnapshot, filepath.Base(path))
	}
	return nil
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
