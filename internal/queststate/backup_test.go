package queststate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFileStoreBackupToWritesCommittedSnapshotAndDeterministicManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	store := NewFileStore(path)
	snapshot := Snapshot{Flags: []Flag{
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1},
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "talked", Value: 1},
		{Character: "OtherHero", QuestRef: "quest:side_quest", Name: "done", Value: 2},
	}}
	if err := store.Save(snapshot); err != nil {
		t.Fatalf("save quest state snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), ".quest-state-crashed.json"), []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write crash temp file: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "quest-state-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup quest state store: %v", err)
	}

	backup := NewFileStore(filepath.Join(backupDir, "quest-state.json"))
	got, err := backup.Load()
	if err != nil {
		t.Fatalf("load backup snapshot: %v", err)
	}
	wantSnapshot := NormalizeSnapshot(snapshot)
	if !reflect.DeepEqual(got, wantSnapshot) {
		t.Fatalf("unexpected backup snapshot: got %#v want %#v", got, wantSnapshot)
	}
	if _, err := os.Stat(filepath.Join(backupDir, ".quest-state-crashed.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected source crash temp file to be omitted from backup, stat err=%v", err)
	}

	rawManifest, err := os.ReadFile(filepath.Join(backupDir, BackupManifestFilename))
	if err != nil {
		t.Fatalf("read backup manifest: %v", err)
	}
	var manifest BackupManifest
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		t.Fatalf("decode backup manifest: %v", err)
	}
	if manifest.Format != BackupManifestFormat {
		t.Fatalf("unexpected manifest format: got %q want %q", manifest.Format, BackupManifestFormat)
	}
	wantSummary := SnapshotSummary{
		FlagCount:  3,
		Characters: []string{"OtherHero", "QuestHero"},
		QuestRefs:  []string{"quest:first_steps", "quest:side_quest"},
		FlagKeys: []string{
			"OtherHero:quest:side_quest:done",
			"QuestHero:quest:first_steps:step",
			"QuestHero:quest:first_steps:talked",
		},
	}
	if !reflect.DeepEqual(manifest.Summary, wantSummary) {
		t.Fatalf("unexpected manifest summary: got %#v want %#v", manifest.Summary, wantSummary)
	}
	if len(manifest.Files) != 1 || manifest.Files[0].Filename != "quest-state.json" {
		t.Fatalf("unexpected manifest files: %#v", manifest.Files)
	}
	rawSnapshot, err := os.ReadFile(filepath.Join(backupDir, manifest.Files[0].Filename))
	if err != nil {
		t.Fatalf("read manifest snapshot: %v", err)
	}
	checksum := sha256.Sum256(rawSnapshot)
	if gotChecksum := hex.EncodeToString(checksum[:]); gotChecksum != manifest.Files[0].SHA256 {
		t.Fatalf("unexpected manifest checksum: got %s want %s", manifest.Files[0].SHA256, gotChecksum)
	}
	if int64(len(rawSnapshot)) != manifest.Files[0].SizeBytes {
		t.Fatalf("unexpected manifest size: got %d want %d", manifest.Files[0].SizeBytes, len(rawSnapshot))
	}
}

func TestFileStoreBackupToTreatsMissingSnapshotAsEmptyStore(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "missing", "quest-state.json"))
	backupDir := filepath.Join(t.TempDir(), "quest-state-backup")

	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup missing quest state store: %v", err)
	}

	if _, err := os.Stat(filepath.Join(backupDir, "quest-state.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing snapshot backup to omit committed quest-state file, stat err=%v", err)
	}
	rawManifest, err := os.ReadFile(filepath.Join(backupDir, BackupManifestFilename))
	if err != nil {
		t.Fatalf("read missing-store backup manifest: %v", err)
	}
	var manifest BackupManifest
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		t.Fatalf("decode missing-store backup manifest: %v", err)
	}
	want := BackupManifest{
		Format:  BackupManifestFormat,
		Summary: SnapshotSummary{Characters: []string{}, QuestRefs: []string{}, FlagKeys: []string{}},
		Files:   []BackupManifestFile{},
	}
	if !reflect.DeepEqual(manifest, want) {
		t.Fatalf("unexpected missing-store backup manifest: got %#v want %#v", manifest, want)
	}
}

func TestFileStoreBackupToRollsBackSnapshotWhenSaveSyncFailsAfterCommit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}); err != nil {
		t.Fatalf("save source quest state snapshot: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "quest-state-backup")
	injectedErr := errors.New("injected quest-state backup snapshot sync failure")
	originalSyncStoreDir := syncStoreDir
	t.Cleanup(func() { syncStoreDir = originalSyncStoreDir })
	backupDirSyncCalls := 0
	syncStoreDir = func(path string) error {
		if path == backupDir {
			backupDirSyncCalls++
			if backupDirSyncCalls == 1 {
				return injectedErr
			}
			return nil
		}
		return syncDir(path)
	}

	err := store.BackupTo(backupDir)
	if !errors.Is(err, injectedErr) {
		t.Fatalf("expected injected snapshot sync error, got %v", err)
	}
	entries, readErr := os.ReadDir(backupDir)
	if readErr != nil {
		t.Fatalf("read backup dir after failed backup: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("expected failed snapshot-save sync to roll back committed backup files, got %#v", directoryEntryNames(entries))
	}
}

func TestFileStoreBackupToRollsBackSnapshotAndManifestWhenFinalSyncFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}); err != nil {
		t.Fatalf("save source quest state snapshot: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "quest-state-backup")
	injectedErr := errors.New("injected quest-state final backup sync failure")
	originalSyncStoreDir := syncStoreDir
	t.Cleanup(func() { syncStoreDir = originalSyncStoreDir })
	backupDirSyncCalls := 0
	syncStoreDir = func(path string) error {
		if path == backupDir {
			backupDirSyncCalls++
			if backupDirSyncCalls == 2 {
				return injectedErr
			}
			return nil
		}
		return syncDir(path)
	}

	err := store.BackupTo(backupDir)
	if !errors.Is(err, injectedErr) {
		t.Fatalf("expected injected final sync error, got %v", err)
	}
	entries, readErr := os.ReadDir(backupDir)
	if readErr != nil {
		t.Fatalf("read backup dir after failed final sync: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("expected failed final sync to roll back backup snapshot and manifest, got %#v", directoryEntryNames(entries))
	}
}

func TestFileStoreValidateRejectsStaleBackupManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	store := NewFileStore(path)
	snapshot := Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}
	if err := store.Save(snapshot); err != nil {
		t.Fatalf("save quest state snapshot: %v", err)
	}
	summary := summarizeSnapshot(snapshot)
	if err := writeBackupManifest(filepath.Dir(path), filepath.Base(path), summary, true); err != nil {
		t.Fatalf("write backup manifest: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"flags":[{"character":"QuestHero","quest_ref":"quest:first_steps","name":"step","value":2}]}`), 0o644); err != nil {
		t.Fatalf("tamper quest state snapshot after manifest write: %v", err)
	}
	if err := store.validateActiveBackupManifest(); !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected active manifest preflight to detect stale manifest, got %v", err)
	}

	_, err := store.Validate()
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest for stale active manifest, got %v", err)
	}
}

func TestFileStoreBackupToRejectsStaleActiveBackupManifestBeforeCreatingDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}); err != nil {
		t.Fatalf("save quest state snapshot: %v", err)
	}
	if err := writeBackupManifest(filepath.Dir(path), filepath.Base(path), summarizeSnapshot(Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}), true); err != nil {
		t.Fatalf("write active backup manifest: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"flags":[{"character":"QuestHero","quest_ref":"quest:first_steps","name":"step","value":2}]}`), 0o644); err != nil {
		t.Fatalf("tamper active quest state snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "quest-state-backup")

	err := store.BackupTo(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest before backup, got %v", err)
	}
	if _, statErr := os.Stat(backupDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected rejected backup not to create destination, stat err=%v", statErr)
	}
}

func TestFileStoreValidateBackupFromValidatesManifestWithoutMutatingTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	source := NewFileStore(path)
	if err := source.Save(Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}); err != nil {
		t.Fatalf("save source quest state snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "quest-state-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup quest state store: %v", err)
	}

	targetPath := filepath.Join(t.TempDir(), "target", "quest-state.json")
	target := NewFileStore(targetPath)
	summary, err := target.ValidateBackupFrom(backupDir)
	if err != nil {
		t.Fatalf("validate quest state backup: %v", err)
	}
	want := SnapshotSummary{
		FlagCount:  1,
		Characters: []string{"QuestHero"},
		QuestRefs:  []string{"quest:first_steps"},
		FlagKeys:   []string{"QuestHero:quest:first_steps:step"},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected backup validation summary: got %#v want %#v", summary, want)
	}
	if _, err := os.Stat(targetPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected dry-run validation not to create target snapshot, stat err=%v", err)
	}
}

func TestFileStoreValidateBackupFromReportsIgnoredCrashTempFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	source := NewFileStore(path)
	if err := source.Save(Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}); err != nil {
		t.Fatalf("save source quest state snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "quest-state-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup quest state store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, ".quest-state-crashed.json"), []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write backup crash temp: %v", err)
	}

	summary, err := NewFileStore(filepath.Join(t.TempDir(), "target", "quest-state.json")).ValidateBackupFrom(backupDir)
	if err != nil {
		t.Fatalf("validate quest state backup: %v", err)
	}
	want := SnapshotSummary{
		FlagCount:      1,
		Characters:     []string{"QuestHero"},
		QuestRefs:      []string{"quest:first_steps"},
		FlagKeys:       []string{"QuestHero:quest:first_steps:step"},
		CrashTempCount: 1,
		CrashTempFiles: []string{".quest-state-crashed.json"},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected crash-temp backup summary: got %#v want %#v", summary, want)
	}
}

func TestFileStoreValidateBackupFromRejectsChecksumMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	source := NewFileStore(path)
	if err := source.Save(Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}); err != nil {
		t.Fatalf("save source quest state snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "quest-state-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup quest state store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "quest-state.json"), []byte(`{"flags":[{"character":"QuestHero","quest_ref":"quest:first_steps","name":"step","value":1}]}`), 0o644); err != nil {
		t.Fatalf("tamper backup snapshot bytes: %v", err)
	}

	_, err := NewFileStore(filepath.Join(t.TempDir(), "target", "quest-state.json")).ValidateBackupFrom(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest for checksum mismatch, got %v", err)
	}
}

func TestFileStoreValidateBackupFromRejectsUntrackedBackupEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	source := NewFileStore(path)
	if err := source.Save(Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}); err != nil {
		t.Fatalf("save source quest state snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "quest-state-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup quest state store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "extra.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write untracked backup entry: %v", err)
	}

	_, err := NewFileStore(filepath.Join(t.TempDir(), "target", "quest-state.json")).ValidateBackupFrom(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest for untracked backup entry, got %v", err)
	}
}

func TestFileStoreRestoreFromRestoresManifestedBackupIntoEmptyStore(t *testing.T) {
	source := NewFileStore(filepath.Join(t.TempDir(), "state", "quest-state.json"))
	backupSnapshot := Snapshot{Flags: []Flag{
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1},
		{Character: "OtherHero", QuestRef: "quest:side_quest", Name: "done", Value: 2},
	}}
	if err := source.Save(backupSnapshot); err != nil {
		t.Fatalf("save source quest state snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "quest-state-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup quest state store: %v", err)
	}

	targetPath := filepath.Join(t.TempDir(), "restore-target", "quest-state.json")
	target := NewFileStore(targetPath)
	if err := target.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore quest state store: %v", err)
	}
	got, err := target.Load()
	if err != nil {
		t.Fatalf("load restored quest state snapshot: %v", err)
	}
	wantSnapshot := NormalizeSnapshot(backupSnapshot)
	if !reflect.DeepEqual(got, wantSnapshot) {
		t.Fatalf("unexpected restored quest state snapshot: got %#v want %#v", got, wantSnapshot)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(targetPath), BackupManifestFilename)); err != nil {
		t.Fatalf("expected restored backup manifest: %v", err)
	}
}

func TestFileStoreSaveRemovesStaleBackupManifestAfterMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}); err != nil {
		t.Fatalf("save quest state snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "quest-state-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup quest state store: %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "restore-target", "quest-state.json")
	target := NewFileStore(targetPath)
	if err := target.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore quest state store: %v", err)
	}
	manifestPath := filepath.Join(filepath.Dir(targetPath), BackupManifestFilename)
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("expected restored backup manifest before mutation: %v", err)
	}

	if err := target.Save(Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 2}}}); err != nil {
		t.Fatalf("mutate restored quest state snapshot: %v", err)
	}
	if _, err := os.Stat(manifestPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected successful save to remove restored backup manifest, stat err=%v", err)
	}
}

func TestFileStoreRestoreFromRestoresMissingSnapshotBackupAsEmptyStore(t *testing.T) {
	source := NewFileStore(filepath.Join(t.TempDir(), "missing", "quest-state.json"))
	backupDir := filepath.Join(t.TempDir(), "quest-state-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup missing quest state store: %v", err)
	}

	targetPath := filepath.Join(t.TempDir(), "restore-target", "quest-state.json")
	target := NewFileStore(targetPath)
	if err := target.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore empty quest state backup: %v", err)
	}
	if _, err := target.Load(); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected missing-snapshot restore to leave target without committed file, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(targetPath), BackupManifestFilename)); err != nil {
		t.Fatalf("expected restored empty-store backup manifest: %v", err)
	}
}

func TestFileStoreRestoreFromRejectsNonEmptyTargetStore(t *testing.T) {
	source := NewFileStore(filepath.Join(t.TempDir(), "state", "quest-state.json"))
	if err := source.Save(Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}); err != nil {
		t.Fatalf("save source quest state snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "quest-state-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup quest state store: %v", err)
	}

	targetPath := filepath.Join(t.TempDir(), "restore-target", "quest-state.json")
	target := NewFileStore(targetPath)
	if err := target.Save(Snapshot{Flags: []Flag{{Character: "Existing", QuestRef: "quest:old", Name: "flag", Value: 1}}}); err != nil {
		t.Fatalf("seed non-empty restore target: %v", err)
	}
	err := target.RestoreFrom(backupDir)
	if !errors.Is(err, ErrRestoreDirNotEmpty) {
		t.Fatalf("expected ErrRestoreDirNotEmpty, got %v", err)
	}
}

func TestFileStoreRestoreFromRejectsTargetInsideBackupSource(t *testing.T) {
	source := NewFileStore(filepath.Join(t.TempDir(), "state", "quest-state.json"))
	if err := source.Save(Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}); err != nil {
		t.Fatalf("save source quest state snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "quest-state-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup quest state store: %v", err)
	}

	target := NewFileStore(filepath.Join(backupDir, "nested", "quest-state.json"))
	err := target.RestoreFrom(backupDir)
	if !errors.Is(err, ErrRestoreDirInsideSource) {
		t.Fatalf("expected ErrRestoreDirInsideSource, got %v", err)
	}
}

func directoryEntryNames(entries []os.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
