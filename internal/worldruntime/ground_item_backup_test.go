package worldruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func sampleDurableGroundItemSnapshot() DurableGroundItemSnapshot {
	itemCount := uint16(2)
	goldAmount := uint32(75)
	ownershipExpires := time.Date(2026, 8, 22, 12, 0, 30, 0, time.UTC)
	despawnItem := time.Date(2026, 8, 22, 12, 5, 0, 0, time.UTC)
	despawnGold := time.Date(2026, 8, 22, 12, 6, 0, 0, time.UTC)
	return DurableGroundItemSnapshot{GroundItems: []DurableGroundItemRecord{
		{
			VID: 0x07000001, Vnum: 27001, ItemCount: &itemCount, ItemID: 0x30010001,
			OwnerLogin: "item-owner", OwnerCharacterID: 11, OwnerVID: 0x02000011, OwnerName: "ItemHero",
			MapIndex: 1, X: 1100, Y: 2100, PickupRange: 450,
			OwnershipExclusive: true, OwnershipExpiresAt: &ownershipExpires, DespawnAt: despawnItem,
		},
		{
			VID: 0x07000002, Vnum: 1, GoldAmount: &goldAmount,
			OwnerLogin: "gold-owner", OwnerCharacterID: 22, OwnerVID: 0x02000022, OwnerName: "GoldHero",
			MapIndex: 1, X: 1200, Y: 2200, PickupRange: 300,
			OwnershipExclusive: false, DespawnAt: despawnGold,
		},
	}}
}

func TestGroundItemFileStoreBackupToWritesCommittedSnapshotAndDeterministicManifest(t *testing.T) {
	defer DisableDurableGroundItemSyncForTest()()
	path := filepath.Join(t.TempDir(), "state", "ground-items.json")
	store := NewGroundItemFileStore(path)
	snapshot := sampleDurableGroundItemSnapshot()
	if err := store.Save(snapshot); err != nil {
		t.Fatalf("save ground item snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), ".ground-items-crashed.json"), []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write crash temp file: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "ground-item-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup ground item store: %v", err)
	}

	backup := NewGroundItemFileStore(filepath.Join(backupDir, "ground-items.json"))
	got, err := backup.Load()
	if err != nil {
		t.Fatalf("load backup snapshot: %v", err)
	}
	wantSnapshot := NormalizeDurableGroundItemSnapshot(snapshot)
	if !reflect.DeepEqual(got, wantSnapshot) {
		t.Fatalf("unexpected backup snapshot: got %#v want %#v", got, wantSnapshot)
	}
	if _, err := os.Stat(filepath.Join(backupDir, ".ground-items-crashed.json")); !errors.Is(err, os.ErrNotExist) {
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
	wantSummary := SummarizeDurableGroundItemSnapshot(wantSnapshot)
	if !reflect.DeepEqual(manifest.Summary, wantSummary) {
		t.Fatalf("unexpected manifest summary: got %#v want %#v", manifest.Summary, wantSummary)
	}
	if len(manifest.Files) != 1 || manifest.Files[0].Filename != "ground-items.json" {
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

func TestGroundItemFileStoreBackupToTreatsMissingSnapshotAsEmptyStore(t *testing.T) {
	defer DisableDurableGroundItemSyncForTest()()
	store := NewGroundItemFileStore(filepath.Join(t.TempDir(), "missing", "ground-items.json"))
	backupDir := filepath.Join(t.TempDir(), "ground-item-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup missing ground item store: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backupDir, "ground-items.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing snapshot backup to omit committed ground-items file, stat err=%v", err)
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
		Summary: DurableGroundItemSnapshotSummary{VIDs: []uint32{}},
		Files:   []BackupManifestFile{},
	}
	if !reflect.DeepEqual(manifest, want) {
		t.Fatalf("unexpected missing-store backup manifest: got %#v want %#v", manifest, want)
	}
}

func TestGroundItemFileStoreValidateRejectsStaleBackupManifest(t *testing.T) {
	defer DisableDurableGroundItemSyncForTest()()
	path := filepath.Join(t.TempDir(), "state", "ground-items.json")
	store := NewGroundItemFileStore(path)
	if err := store.Save(sampleDurableGroundItemSnapshot()); err != nil {
		t.Fatalf("save ground item snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "ground-item-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup ground item store: %v", err)
	}
	rawManifest, err := os.ReadFile(filepath.Join(backupDir, BackupManifestFilename))
	if err != nil {
		t.Fatalf("read backup manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), BackupManifestFilename), rawManifest, 0o644); err != nil {
		t.Fatalf("seed active backup manifest: %v", err)
	}
	mutated := sampleDurableGroundItemSnapshot()
	*mutated.GroundItems[0].ItemCount = 3
	raw, err := json.MarshalIndent(NormalizeDurableGroundItemSnapshot(mutated), "", "  ")
	if err != nil {
		t.Fatalf("encode mutated snapshot: %v", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatalf("overwrite committed snapshot without clearing manifest: %v", err)
	}
	if err := store.validateActiveBackupManifest(); !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest for stale active manifest, got %v", err)
	}
	if _, err := store.Validate(); !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected Validate to reject stale active manifest, got %v", err)
	}
}

func TestGroundItemFileStoreRestoreFromRestoresManifestedBackupIntoEmptyStore(t *testing.T) {
	defer DisableDurableGroundItemSyncForTest()()
	source := NewGroundItemFileStore(filepath.Join(t.TempDir(), "state", "ground-items.json"))
	backupSnapshot := sampleDurableGroundItemSnapshot()
	if err := source.Save(backupSnapshot); err != nil {
		t.Fatalf("save source ground item snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "ground-item-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup ground item store: %v", err)
	}

	targetPath := filepath.Join(t.TempDir(), "restore-target", "ground-items.json")
	target := NewGroundItemFileStore(targetPath)
	if err := target.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore ground item store: %v", err)
	}
	got, err := target.Load()
	if err != nil {
		t.Fatalf("load restored ground item snapshot: %v", err)
	}
	wantSnapshot := NormalizeDurableGroundItemSnapshot(backupSnapshot)
	if !reflect.DeepEqual(got, wantSnapshot) {
		t.Fatalf("unexpected restored ground item snapshot: got %#v want %#v", got, wantSnapshot)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(targetPath), BackupManifestFilename)); err != nil {
		t.Fatalf("expected restored backup manifest: %v", err)
	}
}

func TestGroundItemFileStoreSaveRemovesStaleBackupManifestAfterMutation(t *testing.T) {
	defer DisableDurableGroundItemSyncForTest()()
	path := filepath.Join(t.TempDir(), "state", "ground-items.json")
	store := NewGroundItemFileStore(path)
	if err := store.Save(sampleDurableGroundItemSnapshot()); err != nil {
		t.Fatalf("save ground item snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "ground-item-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup ground item store: %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "restore-target", "ground-items.json")
	target := NewGroundItemFileStore(targetPath)
	if err := target.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore ground item store: %v", err)
	}
	manifestPath := filepath.Join(filepath.Dir(targetPath), BackupManifestFilename)
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("expected restored backup manifest before mutation: %v", err)
	}

	mutated := sampleDurableGroundItemSnapshot()
	*mutated.GroundItems[0].ItemCount = 1
	if err := target.Save(mutated); err != nil {
		t.Fatalf("mutate restored ground item snapshot: %v", err)
	}
	if _, err := os.Stat(manifestPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected successful save to remove restored backup manifest, stat err=%v", err)
	}
}

func TestGroundItemFileStoreRestoreFromRestoresMissingSnapshotBackupAsEmptyStore(t *testing.T) {
	defer DisableDurableGroundItemSyncForTest()()
	source := NewGroundItemFileStore(filepath.Join(t.TempDir(), "missing", "ground-items.json"))
	backupDir := filepath.Join(t.TempDir(), "ground-item-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup missing ground item store: %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "restore-target", "ground-items.json")
	target := NewGroundItemFileStore(targetPath)
	if err := target.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore empty ground item backup: %v", err)
	}
	if _, err := target.Load(); !errors.Is(err, ErrGroundItemSnapshotNotFound) {
		t.Fatalf("expected missing-snapshot restore to leave target without committed file, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(targetPath), BackupManifestFilename)); err != nil {
		t.Fatalf("expected restored empty-store backup manifest: %v", err)
	}
}

func TestGroundItemFileStoreRestoreFromRejectsNonEmptyTargetStore(t *testing.T) {
	defer DisableDurableGroundItemSyncForTest()()
	source := NewGroundItemFileStore(filepath.Join(t.TempDir(), "state", "ground-items.json"))
	if err := source.Save(sampleDurableGroundItemSnapshot()); err != nil {
		t.Fatalf("save source ground item snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "ground-item-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup ground item store: %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "restore-target", "ground-items.json")
	target := NewGroundItemFileStore(targetPath)
	if err := target.Save(sampleDurableGroundItemSnapshot()); err != nil {
		t.Fatalf("seed non-empty restore target: %v", err)
	}
	err := target.RestoreFrom(backupDir)
	if !errors.Is(err, ErrRestoreDirNotEmpty) {
		t.Fatalf("expected ErrRestoreDirNotEmpty, got %v", err)
	}
}

func TestGroundItemFileStoreRestoreFromRejectsTargetInsideBackupSource(t *testing.T) {
	defer DisableDurableGroundItemSyncForTest()()
	source := NewGroundItemFileStore(filepath.Join(t.TempDir(), "state", "ground-items.json"))
	if err := source.Save(sampleDurableGroundItemSnapshot()); err != nil {
		t.Fatalf("save source ground item snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "ground-item-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup ground item store: %v", err)
	}
	target := NewGroundItemFileStore(filepath.Join(backupDir, "nested", "ground-items.json"))
	err := target.RestoreFrom(backupDir)
	if !errors.Is(err, ErrRestoreDirInsideSource) {
		t.Fatalf("expected ErrRestoreDirInsideSource, got %v", err)
	}
}

func TestGroundItemFileStoreValidateBackupFromDryRunsManifestedBackup(t *testing.T) {
	defer DisableDurableGroundItemSyncForTest()()
	source := NewGroundItemFileStore(filepath.Join(t.TempDir(), "state", "ground-items.json"))
	if err := source.Save(sampleDurableGroundItemSnapshot()); err != nil {
		t.Fatalf("save source ground item snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "ground-item-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup ground item store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, ".ground-items-leftover.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write backup crash temp: %v", err)
	}
	summary, err := NewGroundItemFileStore(filepath.Join(t.TempDir(), "other", "ground-items.json")).ValidateBackupFrom(backupDir)
	if err != nil {
		t.Fatalf("validate backup: %v", err)
	}
	want := SummarizeDurableGroundItemSnapshot(sampleDurableGroundItemSnapshot())
	want.CrashTempCount = 1
	want.CrashTempFiles = []string{".ground-items-leftover.json"}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected validate summary: got %#v want %#v", summary, want)
	}
}

func TestGroundItemFileStoreCleanupCrashTempFilesRemovesOnlyGroundItemTemps(t *testing.T) {
	defer DisableDurableGroundItemSyncForTest()()
	path := filepath.Join(t.TempDir(), "state", "ground-items.json")
	store := NewGroundItemFileStore(path)
	if err := store.Save(sampleDurableGroundItemSnapshot()); err != nil {
		t.Fatalf("save ground item snapshot: %v", err)
	}
	storeDir := filepath.Dir(path)
	if err := os.WriteFile(filepath.Join(storeDir, ".ground-items-crashed.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write crash temp: %v", err)
	}
	if err := os.WriteFile(filepath.Join(storeDir, "keep-me.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatalf("write unrelated file: %v", err)
	}
	summary, err := store.CleanupCrashTempFiles()
	if err != nil {
		t.Fatalf("cleanup crash temps: %v", err)
	}
	if summary.CrashTempCount != 0 {
		t.Fatalf("expected zero crash temps after cleanup, got %#v", summary)
	}
	if _, err := os.Stat(filepath.Join(storeDir, ".ground-items-crashed.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected crash temp removed, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(storeDir, "keep-me.txt")); err != nil {
		t.Fatalf("expected unrelated file preserved: %v", err)
	}
}
