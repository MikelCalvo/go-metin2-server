package itemstore

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

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
