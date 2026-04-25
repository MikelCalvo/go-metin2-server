package staticstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
		return Snapshot{}, fmt.Errorf("read static actor snapshot: %w", err)
	}

	var snapshot Snapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("%w: decode static actor snapshot: %v", ErrInvalidSnapshot, err)
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return Snapshot{}, fmt.Errorf("%w: validate static actor snapshot", err)
	}
	return normalized, nil
}

func (s *FileStore) Save(snapshot Snapshot) error {
	if s == nil || s.path == "" {
		return ErrStorePathRequired
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create static actor store dir: %w", err)
	}

	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return fmt.Errorf("%w: validate static actor snapshot", err)
	}
	raw, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("encode static actor snapshot: %w", err)
	}
	raw = append(raw, '\n')

	temp, err := os.CreateTemp(filepath.Dir(s.path), ".static-actors-*.json")
	if err != nil {
		return fmt.Errorf("create static actor temp file: %w", err)
	}
	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()

	if _, err := temp.Write(raw); err != nil {
		return fmt.Errorf("write static actor snapshot: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close static actor temp file: %w", err)
	}
	if err := os.Rename(temp.Name(), s.path); err != nil {
		return fmt.Errorf("commit static actor snapshot: %w", err)
	}
	return nil
}
