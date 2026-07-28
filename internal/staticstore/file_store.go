package staticstore

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

	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Snapshot{}, ErrSnapshotNotFound
		}
		return Snapshot{}, fmt.Errorf("read static actor snapshot: %w", err)
	}
	if !utf8.Valid(raw) {
		return Snapshot{}, fmt.Errorf("%w: decode static actor snapshot: invalid utf-8", ErrInvalidSnapshot)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return Snapshot{}, fmt.Errorf("%w: decode static actor snapshot: null root", ErrInvalidSnapshot)
	}

	var rawSnapshot struct {
		StaticActors json.RawMessage `json:"static_actors"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rawSnapshot); err != nil {
		return Snapshot{}, fmt.Errorf("%w: decode static actor snapshot: %v", ErrInvalidSnapshot, err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Snapshot{}, fmt.Errorf("%w: decode static actor snapshot: trailing json value", ErrInvalidSnapshot)
		}
		return Snapshot{}, fmt.Errorf("%w: decode static actor snapshot: %v", ErrInvalidSnapshot, err)
	}
	var snapshot Snapshot
	if rawSnapshot.StaticActors != nil {
		if bytes.Equal(bytes.TrimSpace(rawSnapshot.StaticActors), []byte("null")) {
			return Snapshot{}, fmt.Errorf("%w: decode static actor snapshot: null static_actors collection", ErrInvalidSnapshot)
		}
		collectionDecoder := json.NewDecoder(bytes.NewReader(rawSnapshot.StaticActors))
		collectionDecoder.DisallowUnknownFields()
		if err := collectionDecoder.Decode(&snapshot.StaticActors); err != nil {
			return Snapshot{}, fmt.Errorf("%w: decode static actor snapshot: %v", ErrInvalidSnapshot, err)
		}
		if err := collectionDecoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			if err == nil {
				return Snapshot{}, fmt.Errorf("%w: decode static actor snapshot: trailing static_actors value", ErrInvalidSnapshot)
			}
			return Snapshot{}, fmt.Errorf("%w: decode static actor snapshot: %v", ErrInvalidSnapshot, err)
		}
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return Snapshot{}, fmt.Errorf("%w: validate static actor snapshot", err)
	}
	return normalized, nil
}

func (s *FileStore) Validate() (SnapshotSummary, error) {
	if s == nil || s.path == "" {
		return SnapshotSummary{}, ErrStorePathRequired
	}
	summary := SnapshotSummary{ActorIDs: []uint64{}, ActorNames: []string{}}
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
			return SnapshotSummary{}, fmt.Errorf("remove static actor crash temp file %q: %w", filename, err)
		}
	}
	if err := syncDir(storeDir); err != nil {
		return SnapshotSummary{}, fmt.Errorf("sync static actor store dir after crash temp cleanup: %w", err)
	}
	return s.Validate()
}

func summarizeSnapshot(snapshot Snapshot) SnapshotSummary {
	summary := SnapshotSummary{
		ActorCount: len(snapshot.StaticActors),
		ActorIDs:   make([]uint64, 0, len(snapshot.StaticActors)),
		ActorNames: make([]string, 0, len(snapshot.StaticActors)),
	}
	for _, actor := range snapshot.StaticActors {
		summary.ActorIDs = append(summary.ActorIDs, actor.EntityID)
		summary.ActorNames = append(summary.ActorNames, actor.Name)
		if actor.InteractionKind != "" && actor.InteractionRef != "" {
			summary.InteractableActorCount++
		}
		if actor.SpawnGroupRef != "" {
			summary.SpawnGroupCount++
		}
	}
	return summary
}

func (s *FileStore) crashTempFiles() ([]string, error) {
	entries, err := os.ReadDir(filepath.Dir(s.path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read static actor store crash temp files: %w", err)
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
		if strings.HasPrefix(name, ".static-actors-") && strings.HasSuffix(name, ".json") {
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
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync static actor temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close static actor temp file: %w", err)
	}
	if err := os.Rename(temp.Name(), s.path); err != nil {
		return fmt.Errorf("commit static actor snapshot: %w", err)
	}
	if err := syncDir(filepath.Dir(s.path)); err != nil {
		return fmt.Errorf("sync static actor store dir: %w", err)
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
