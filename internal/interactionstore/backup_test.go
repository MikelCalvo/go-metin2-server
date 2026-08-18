package interactionstore

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

func sampleInteractionSnapshot() Snapshot {
	return Snapshot{Definitions: []Definition{
		{Kind: KindTalk, Ref: "npc:village_guard", Text: "Welcome to the village."},
		{Kind: KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
	}}
}

func sampleInteractionSummary() SnapshotSummary {
	return SnapshotSummary{
		DefinitionCount: 2,
		DefinitionKeys:  []string{"info:lore:alchemist", "talk:npc:village_guard"},
	}
}

func sampleInteractionSnapshotJSON() string {
	// Compact JSON that still loads as the same definitions, but differs from
	// the indented bytes written by Save so checksum validation fails closed.
	return `{"definitions":[{"kind":"info","ref":"lore:alchemist","text":"The alchemist studies forgotten herbs."},{"kind":"talk","ref":"npc:village_guard","text":"Welcome to the village."}]}` + "\n"
}

func sampleMutatedInteractionSnapshotJSON() string {
	return "{\n  \"definitions\": [\n    {\n      \"kind\": \"info\",\n      \"ref\": \"lore:alchemist\",\n      \"text\": \"The alchemist studies changed herbs.\"\n    }\n  ]\n}\n"
}

func TestFileStoreBackupToWritesCommittedSnapshotAndDeterministicManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	snapshot := sampleInteractionSnapshot()
	if err := store.Save(snapshot); err != nil {
		t.Fatalf("save interaction snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), ".interaction-definitions-crashed.json"), []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write crash temp file: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "interaction-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup interaction store: %v", err)
	}

	backup := NewFileStore(filepath.Join(backupDir, "interaction-definitions.json"))
	got, err := backup.Load()
	if err != nil {
		t.Fatalf("load backup snapshot: %v", err)
	}
	wantSnapshot := normalizeSnapshot(snapshot)
	if !reflect.DeepEqual(got, wantSnapshot) {
		t.Fatalf("unexpected backup snapshot: got %#v want %#v", got, wantSnapshot)
	}
	if _, err := os.Stat(filepath.Join(backupDir, ".interaction-definitions-crashed.json")); !errors.Is(err, os.ErrNotExist) {
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
	wantSummary := sampleInteractionSummary()
	if !reflect.DeepEqual(manifest.Summary, wantSummary) {
		t.Fatalf("unexpected manifest summary: got %#v want %#v", manifest.Summary, wantSummary)
	}
	if len(manifest.Files) != 1 || manifest.Files[0].Filename != "interaction-definitions.json" {
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
	store := NewFileStore(filepath.Join(t.TempDir(), "missing", "interaction-definitions.json"))
	backupDir := filepath.Join(t.TempDir(), "interaction-backup")

	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup missing interaction store: %v", err)
	}

	if _, err := os.Stat(filepath.Join(backupDir, "interaction-definitions.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing snapshot backup to omit committed interaction file, stat err=%v", err)
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
		Summary: SnapshotSummary{DefinitionKeys: []string{}},
		Files:   []BackupManifestFile{},
	}
	if !reflect.DeepEqual(manifest, want) {
		t.Fatalf("unexpected missing-store backup manifest: got %#v want %#v", manifest, want)
	}
}

func TestFileStoreBackupToRollsBackSnapshotWhenSaveSyncFailsAfterCommit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	if err := store.Save(sampleInteractionSnapshot()); err != nil {
		t.Fatalf("save source interaction snapshot: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "interaction-backup")
	injectedErr := errors.New("injected interaction backup snapshot sync failure")
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
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	if err := store.Save(sampleInteractionSnapshot()); err != nil {
		t.Fatalf("save source interaction snapshot: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "interaction-backup")
	injectedErr := errors.New("injected interaction final backup sync failure")
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
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	snapshot := sampleInteractionSnapshot()
	if err := store.Save(snapshot); err != nil {
		t.Fatalf("save interaction snapshot: %v", err)
	}
	summary := summarizeSnapshot(snapshot)
	if err := writeBackupManifest(filepath.Dir(path), filepath.Base(path), summary, true); err != nil {
		t.Fatalf("write backup manifest: %v", err)
	}
	if err := os.WriteFile(path, []byte(sampleMutatedInteractionSnapshotJSON()), 0o644); err != nil {
		t.Fatalf("tamper interaction snapshot after manifest write: %v", err)
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
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	if err := store.Save(sampleInteractionSnapshot()); err != nil {
		t.Fatalf("save interaction snapshot: %v", err)
	}
	if err := writeBackupManifest(filepath.Dir(path), filepath.Base(path), summarizeSnapshot(sampleInteractionSnapshot()), true); err != nil {
		t.Fatalf("write active backup manifest: %v", err)
	}
	if err := os.WriteFile(path, []byte(sampleMutatedInteractionSnapshotJSON()), 0o644); err != nil {
		t.Fatalf("tamper active interaction snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "interaction-backup")

	err := store.BackupTo(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest before backup, got %v", err)
	}
	if _, statErr := os.Stat(backupDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected rejected backup not to create destination, stat err=%v", statErr)
	}
}

func TestFileStoreValidateBackupFromValidatesManifestWithoutMutatingTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	source := NewFileStore(path)
	if err := source.Save(sampleInteractionSnapshot()); err != nil {
		t.Fatalf("save source interaction snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "interaction-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup interaction store: %v", err)
	}

	targetPath := filepath.Join(t.TempDir(), "target", "interaction-definitions.json")
	target := NewFileStore(targetPath)
	summary, err := target.ValidateBackupFrom(backupDir)
	if err != nil {
		t.Fatalf("validate interaction backup: %v", err)
	}
	want := sampleInteractionSummary()
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected backup validation summary: got %#v want %#v", summary, want)
	}
	if _, err := os.Stat(targetPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected dry-run validation not to create target snapshot, stat err=%v", err)
	}
}

func TestFileStoreValidateBackupFromReportsIgnoredCrashTempFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	source := NewFileStore(path)
	if err := source.Save(sampleInteractionSnapshot()); err != nil {
		t.Fatalf("save source interaction snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "interaction-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup interaction store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, ".interaction-definitions-crashed.json"), []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write backup crash temp: %v", err)
	}

	summary, err := NewFileStore(filepath.Join(t.TempDir(), "target", "interaction-definitions.json")).ValidateBackupFrom(backupDir)
	if err != nil {
		t.Fatalf("validate interaction backup: %v", err)
	}
	want := sampleInteractionSummary()
	want.CrashTempCount = 1
	want.CrashTempFiles = []string{".interaction-definitions-crashed.json"}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected crash-temp backup summary: got %#v want %#v", summary, want)
	}
}

func TestFileStoreValidateBackupFromRejectsChecksumMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	source := NewFileStore(path)
	if err := source.Save(sampleInteractionSnapshot()); err != nil {
		t.Fatalf("save source interaction snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "interaction-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup interaction store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "interaction-definitions.json"), []byte(sampleInteractionSnapshotJSON()), 0o644); err != nil {
		t.Fatalf("tamper backup snapshot bytes: %v", err)
	}

	_, err := NewFileStore(filepath.Join(t.TempDir(), "target", "interaction-definitions.json")).ValidateBackupFrom(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest for checksum mismatch, got %v", err)
	}
}

func TestFileStoreValidateBackupFromRejectsUntrackedBackupEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	source := NewFileStore(path)
	if err := source.Save(sampleInteractionSnapshot()); err != nil {
		t.Fatalf("save source interaction snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "interaction-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup interaction store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "extra.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write untracked backup entry: %v", err)
	}

	_, err := NewFileStore(filepath.Join(t.TempDir(), "target", "interaction-definitions.json")).ValidateBackupFrom(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest for untracked backup entry, got %v", err)
	}
}

func TestFileStoreRestoreFromRestoresManifestedBackupIntoEmptyStore(t *testing.T) {
	source := NewFileStore(filepath.Join(t.TempDir(), "state", "interaction-definitions.json"))
	backupSnapshot := sampleInteractionSnapshot()
	if err := source.Save(backupSnapshot); err != nil {
		t.Fatalf("save source interaction snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "interaction-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup interaction store: %v", err)
	}

	targetPath := filepath.Join(t.TempDir(), "restore-target", "interaction-definitions.json")
	target := NewFileStore(targetPath)
	if err := target.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore interaction store: %v", err)
	}
	got, err := target.Load()
	if err != nil {
		t.Fatalf("load restored interaction snapshot: %v", err)
	}
	wantSnapshot := normalizeSnapshot(backupSnapshot)
	if !reflect.DeepEqual(got, wantSnapshot) {
		t.Fatalf("unexpected restored interaction snapshot: got %#v want %#v", got, wantSnapshot)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(targetPath), BackupManifestFilename)); err != nil {
		t.Fatalf("expected restored backup manifest: %v", err)
	}
}

func TestFileStoreSaveRemovesStaleBackupManifestAfterMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	if err := store.Save(sampleInteractionSnapshot()); err != nil {
		t.Fatalf("save interaction snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "interaction-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup interaction store: %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "restore-target", "interaction-definitions.json")
	target := NewFileStore(targetPath)
	if err := target.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore interaction store: %v", err)
	}
	manifestPath := filepath.Join(filepath.Dir(targetPath), BackupManifestFilename)
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("expected restored backup manifest before mutation: %v", err)
	}

	if err := target.Save(Snapshot{Definitions: []Definition{
		{Kind: KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies changed herbs."},
	}}); err != nil {
		t.Fatalf("mutate restored interaction snapshot: %v", err)
	}
	if _, err := os.Stat(manifestPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected successful save to remove restored backup manifest, stat err=%v", err)
	}
}

func TestFileStoreRestoreFromRestoresMissingSnapshotBackupAsEmptyStore(t *testing.T) {
	source := NewFileStore(filepath.Join(t.TempDir(), "missing", "interaction-definitions.json"))
	backupDir := filepath.Join(t.TempDir(), "interaction-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup missing interaction store: %v", err)
	}

	targetPath := filepath.Join(t.TempDir(), "restore-target", "interaction-definitions.json")
	target := NewFileStore(targetPath)
	if err := target.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore empty interaction backup: %v", err)
	}
	if _, err := target.Load(); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected missing-snapshot restore to leave target without committed file, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(targetPath), BackupManifestFilename)); err != nil {
		t.Fatalf("expected restored empty-store backup manifest: %v", err)
	}
}

func TestFileStoreRestoreFromRejectsNonEmptyTargetStore(t *testing.T) {
	source := NewFileStore(filepath.Join(t.TempDir(), "state", "interaction-definitions.json"))
	if err := source.Save(sampleInteractionSnapshot()); err != nil {
		t.Fatalf("save source interaction snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "interaction-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup interaction store: %v", err)
	}

	targetPath := filepath.Join(t.TempDir(), "restore-target", "interaction-definitions.json")
	target := NewFileStore(targetPath)
	if err := target.Save(Snapshot{Definitions: []Definition{
		{Kind: KindInfo, Ref: "lore:existing", Text: "Existing text."},
	}}); err != nil {
		t.Fatalf("seed non-empty restore target: %v", err)
	}
	err := target.RestoreFrom(backupDir)
	if !errors.Is(err, ErrRestoreDirNotEmpty) {
		t.Fatalf("expected ErrRestoreDirNotEmpty, got %v", err)
	}
}

func TestFileStoreRestoreFromRejectsTargetInsideBackupSource(t *testing.T) {
	source := NewFileStore(filepath.Join(t.TempDir(), "state", "interaction-definitions.json"))
	if err := source.Save(sampleInteractionSnapshot()); err != nil {
		t.Fatalf("save source interaction snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "interaction-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup interaction store: %v", err)
	}

	target := NewFileStore(filepath.Join(backupDir, "nested", "interaction-definitions.json"))
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
