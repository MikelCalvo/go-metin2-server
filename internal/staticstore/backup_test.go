package staticstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func sampleStaticActorSnapshot() Snapshot {
	return Snapshot{StaticActors: []StaticActor{
		{EntityID: 2, Name: "Alchemist", MapIndex: 21, X: 52070, Y: 166600, RaceNum: 20001, InteractionKind: "info", InteractionRef: "lore:alchemist"},
		{EntityID: 7, Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy},
		{EntityID: 9, Name: "VillageGuard", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355, InteractionKind: "talk", InteractionRef: "npc:village_guard"},
	}}
}

func sampleStaticActorSummary() SnapshotSummary {
	return SnapshotSummary{
		ActorCount:             3,
		InteractableActorCount: 2,
		ActorIDs:               []uint64{2, 7, 9},
		ActorNames:             []string{"Alchemist", "TrainingDummy", "VillageGuard"},
	}
}

func TestFileStoreBackupToWritesCommittedSnapshotAndDeterministicManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	snapshot := sampleStaticActorSnapshot()
	if err := store.Save(snapshot); err != nil {
		t.Fatalf("save static actor snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), ".static-actors-crashed.json"), []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write crash temp file: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "static-actor-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup static actor store: %v", err)
	}

	backup := NewFileStore(filepath.Join(backupDir, "static-actors.json"))
	got, err := backup.Load()
	if err != nil {
		t.Fatalf("load backup snapshot: %v", err)
	}
	wantSnapshot := normalizeSnapshot(snapshot)
	if !reflect.DeepEqual(got, wantSnapshot) {
		t.Fatalf("unexpected backup snapshot: got %#v want %#v", got, wantSnapshot)
	}
	if _, err := os.Stat(filepath.Join(backupDir, ".static-actors-crashed.json")); !errors.Is(err, os.ErrNotExist) {
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
	wantSummary := sampleStaticActorSummary()
	if !reflect.DeepEqual(manifest.Summary, wantSummary) {
		t.Fatalf("unexpected manifest summary: got %#v want %#v", manifest.Summary, wantSummary)
	}
	if len(manifest.Files) != 1 || manifest.Files[0].Filename != "static-actors.json" {
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
	store := NewFileStore(filepath.Join(t.TempDir(), "missing", "static-actors.json"))
	backupDir := filepath.Join(t.TempDir(), "static-actor-backup")

	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup missing static actor store: %v", err)
	}

	if _, err := os.Stat(filepath.Join(backupDir, "static-actors.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing snapshot backup to omit committed static-actors file, stat err=%v", err)
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
		Summary: SnapshotSummary{ActorIDs: []uint64{}, ActorNames: []string{}},
		Files:   []BackupManifestFile{},
	}
	if !reflect.DeepEqual(manifest, want) {
		t.Fatalf("unexpected missing-store backup manifest: got %#v want %#v", manifest, want)
	}
}

func TestFileStoreBackupToRollsBackSnapshotWhenSaveSyncFailsAfterCommit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{StaticActors: []StaticActor{{EntityID: 7, Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy}}}); err != nil {
		t.Fatalf("save source static actor snapshot: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "static-actor-backup")
	injectedErr := errors.New("injected static-actor backup snapshot sync failure")
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
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{StaticActors: []StaticActor{{EntityID: 7, Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy}}}); err != nil {
		t.Fatalf("save source static actor snapshot: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "static-actor-backup")
	injectedErr := errors.New("injected static-actor final backup sync failure")
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
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	snapshot := Snapshot{StaticActors: []StaticActor{{EntityID: 7, Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy}}}
	if err := store.Save(snapshot); err != nil {
		t.Fatalf("save static actor snapshot: %v", err)
	}
	summary := summarizeSnapshot(snapshot)
	if err := writeBackupManifest(filepath.Dir(path), filepath.Base(path), summary, true); err != nil {
		t.Fatalf("write backup manifest: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"static_actors":[{"entity_id":9,"name":"VillageGuard","map_index":1,"x":469300,"y":964200,"race_num":20355}]}`), 0o644); err != nil {
		t.Fatalf("tamper static actor snapshot after manifest write: %v", err)
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
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{StaticActors: []StaticActor{{EntityID: 7, Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy}}}); err != nil {
		t.Fatalf("save static actor snapshot: %v", err)
	}
	if err := writeBackupManifest(filepath.Dir(path), filepath.Base(path), summarizeSnapshot(Snapshot{StaticActors: []StaticActor{{EntityID: 7, Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy}}}), true); err != nil {
		t.Fatalf("write active backup manifest: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"static_actors":[{"entity_id":9,"name":"VillageGuard","map_index":1,"x":469300,"y":964200,"race_num":20355}]}`), 0o644); err != nil {
		t.Fatalf("tamper active static actor snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "static-actor-backup")

	err := store.BackupTo(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest before backup, got %v", err)
	}
	if _, statErr := os.Stat(backupDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected rejected backup not to create destination, stat err=%v", statErr)
	}
}

func TestFileStoreValidateBackupFromValidatesManifestWithoutMutatingTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	source := NewFileStore(path)
	if err := source.Save(Snapshot{StaticActors: []StaticActor{{EntityID: 7, Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy}}}); err != nil {
		t.Fatalf("save source static actor snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "static-actor-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup static actor store: %v", err)
	}

	targetPath := filepath.Join(t.TempDir(), "target", "static-actors.json")
	target := NewFileStore(targetPath)
	summary, err := target.ValidateBackupFrom(backupDir)
	if err != nil {
		t.Fatalf("validate static actor backup: %v", err)
	}
	want := SnapshotSummary{
		ActorCount: 1,
		ActorIDs:   []uint64{7},
		ActorNames: []string{"TrainingDummy"},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected backup validation summary: got %#v want %#v", summary, want)
	}
	if _, err := os.Stat(targetPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected dry-run validation not to create target snapshot, stat err=%v", err)
	}
}

func TestFileStoreValidateBackupFromReportsIgnoredCrashTempFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	source := NewFileStore(path)
	if err := source.Save(Snapshot{StaticActors: []StaticActor{{EntityID: 7, Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy}}}); err != nil {
		t.Fatalf("save source static actor snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "static-actor-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup static actor store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, ".static-actors-crashed.json"), []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write backup crash temp: %v", err)
	}

	summary, err := NewFileStore(filepath.Join(t.TempDir(), "target", "static-actors.json")).ValidateBackupFrom(backupDir)
	if err != nil {
		t.Fatalf("validate static actor backup: %v", err)
	}
	want := SnapshotSummary{
		ActorCount:     1,
		ActorIDs:       []uint64{7},
		ActorNames:     []string{"TrainingDummy"},
		CrashTempCount: 1,
		CrashTempFiles: []string{".static-actors-crashed.json"},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected crash-temp backup summary: got %#v want %#v", summary, want)
	}
}

func TestFileStoreValidateBackupFromRejectsChecksumMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	source := NewFileStore(path)
	if err := source.Save(Snapshot{StaticActors: []StaticActor{{EntityID: 7, Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy}}}); err != nil {
		t.Fatalf("save source static actor snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "static-actor-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup static actor store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "static-actors.json"), []byte(`{"static_actors":[{"entity_id":7,"name":"TrainingDummy","map_index":42,"x":1800,"y":2900,"race_num":20350,"combat_profile":"training_dummy"}]}`), 0o644); err != nil {
		t.Fatalf("tamper backup snapshot bytes: %v", err)
	}

	_, err := NewFileStore(filepath.Join(t.TempDir(), "target", "static-actors.json")).ValidateBackupFrom(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest for checksum mismatch, got %v", err)
	}
}

func TestFileStoreValidateBackupFromRejectsUntrackedBackupEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	source := NewFileStore(path)
	if err := source.Save(Snapshot{StaticActors: []StaticActor{{EntityID: 7, Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy}}}); err != nil {
		t.Fatalf("save source static actor snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "static-actor-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup static actor store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "extra.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write untracked backup entry: %v", err)
	}

	_, err := NewFileStore(filepath.Join(t.TempDir(), "target", "static-actors.json")).ValidateBackupFrom(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest for untracked backup entry, got %v", err)
	}
}

func TestFileStoreRestoreFromRestoresManifestedBackupIntoEmptyStore(t *testing.T) {
	source := NewFileStore(filepath.Join(t.TempDir(), "state", "static-actors.json"))
	backupSnapshot := sampleStaticActorSnapshot()
	if err := source.Save(backupSnapshot); err != nil {
		t.Fatalf("save source static actor snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "static-actor-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup static actor store: %v", err)
	}

	targetPath := filepath.Join(t.TempDir(), "restore-target", "static-actors.json")
	target := NewFileStore(targetPath)
	if err := target.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore static actor store: %v", err)
	}
	got, err := target.Load()
	if err != nil {
		t.Fatalf("load restored static actor snapshot: %v", err)
	}
	wantSnapshot := normalizeSnapshot(backupSnapshot)
	if !reflect.DeepEqual(got, wantSnapshot) {
		t.Fatalf("unexpected restored static actor snapshot: got %#v want %#v", got, wantSnapshot)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(targetPath), BackupManifestFilename)); err != nil {
		t.Fatalf("expected restored backup manifest: %v", err)
	}
}

func TestFileStoreSaveRemovesStaleBackupManifestAfterMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{StaticActors: []StaticActor{{EntityID: 7, Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy}}}); err != nil {
		t.Fatalf("save static actor snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "static-actor-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup static actor store: %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "restore-target", "static-actors.json")
	target := NewFileStore(targetPath)
	if err := target.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore static actor store: %v", err)
	}
	manifestPath := filepath.Join(filepath.Dir(targetPath), BackupManifestFilename)
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("expected restored backup manifest before mutation: %v", err)
	}

	if err := target.Save(Snapshot{StaticActors: []StaticActor{{EntityID: 9, Name: "VillageGuard", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355}}}); err != nil {
		t.Fatalf("mutate restored static actor snapshot: %v", err)
	}
	if _, err := os.Stat(manifestPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected successful save to remove restored backup manifest, stat err=%v", err)
	}
}

func TestFileStoreRestoreFromRestoresMissingSnapshotBackupAsEmptyStore(t *testing.T) {
	source := NewFileStore(filepath.Join(t.TempDir(), "missing", "static-actors.json"))
	backupDir := filepath.Join(t.TempDir(), "static-actor-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup missing static actor store: %v", err)
	}

	targetPath := filepath.Join(t.TempDir(), "restore-target", "static-actors.json")
	target := NewFileStore(targetPath)
	if err := target.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore empty static actor backup: %v", err)
	}
	if _, err := target.Load(); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected missing-snapshot restore to leave target without committed file, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(targetPath), BackupManifestFilename)); err != nil {
		t.Fatalf("expected restored empty-store backup manifest: %v", err)
	}
}

func TestFileStoreRestoreFromRejectsNonEmptyTargetStore(t *testing.T) {
	source := NewFileStore(filepath.Join(t.TempDir(), "state", "static-actors.json"))
	if err := source.Save(Snapshot{StaticActors: []StaticActor{{EntityID: 7, Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy}}}); err != nil {
		t.Fatalf("save source static actor snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "static-actor-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup static actor store: %v", err)
	}

	targetPath := filepath.Join(t.TempDir(), "restore-target", "static-actors.json")
	target := NewFileStore(targetPath)
	if err := target.Save(Snapshot{StaticActors: []StaticActor{{EntityID: 42, Name: "Blacksmith", MapIndex: 41, X: 957300, Y: 255200, RaceNum: 20016}}}); err != nil {
		t.Fatalf("seed non-empty restore target: %v", err)
	}
	err := target.RestoreFrom(backupDir)
	if !errors.Is(err, ErrRestoreDirNotEmpty) {
		t.Fatalf("expected ErrRestoreDirNotEmpty, got %v", err)
	}
}

func TestFileStoreRestoreFromRejectsTargetInsideBackupSource(t *testing.T) {
	source := NewFileStore(filepath.Join(t.TempDir(), "state", "static-actors.json"))
	if err := source.Save(Snapshot{StaticActors: []StaticActor{{EntityID: 7, Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy}}}); err != nil {
		t.Fatalf("save source static actor snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "static-actor-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup static actor store: %v", err)
	}

	target := NewFileStore(filepath.Join(backupDir, "nested", "static-actors.json"))
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
