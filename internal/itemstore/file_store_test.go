package itemstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

func TestFileStoreSaveThenLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, EquipSlot: "weapon"},
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 50, SellCountPerGold: true, Highlight: true, AntiSell: true},
	}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestFileStoreValidateReturnsDeterministicSummaryAndCrashTempFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Templates: []Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, EquipSlot: "weapon"},
	}}); err != nil {
		t.Fatalf("save item template snapshot: %v", err)
	}
	for _, name := range []string{".item-templates-zeta.json", ".item-templates-alpha.json", ".other-temp.json"} {
		if err := os.WriteFile(filepath.Join(filepath.Dir(path), name), []byte(`{"not":"committed"}`), 0o644); err != nil {
			t.Fatalf("write temp file %s: %v", name, err)
		}
	}

	summary, err := store.Validate()
	if err != nil {
		t.Fatalf("validate item template store: %v", err)
	}
	want := SnapshotSummary{TemplateCount: 2, Vnums: []uint32{11200, 27001}, CrashTempCount: 2, CrashTempFiles: []string{".item-templates-alpha.json", ".item-templates-zeta.json"}}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected item template validation summary: got %#v want %#v", summary, want)
	}
}

func TestFileStoreValidateTreatsMissingSnapshotAsEmptyStore(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "missing", "item-templates.json"))

	summary, err := store.Validate()
	if err != nil {
		t.Fatalf("validate missing item template store: %v", err)
	}
	want := SnapshotSummary{Vnums: []uint32{}}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected missing-store summary: got %#v want %#v", summary, want)
	}
}

func TestFileStoreBackupToWritesCommittedSnapshotAndDeterministicManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	snapshot := Snapshot{Templates: []Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, EquipSlot: "weapon"},
	}}
	if err := store.Save(snapshot); err != nil {
		t.Fatalf("save item template snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), ".item-templates-crashed.json"), []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write crash temp file: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "item-template-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup item template store: %v", err)
	}

	backup := NewFileStore(filepath.Join(backupDir, "item-templates.json"))
	got, err := backup.Load()
	if err != nil {
		t.Fatalf("load backup snapshot: %v", err)
	}
	wantSnapshot := NormalizeSnapshot(snapshot)
	if !reflect.DeepEqual(got, wantSnapshot) {
		t.Fatalf("unexpected backup snapshot: got %#v want %#v", got, wantSnapshot)
	}
	if _, err := os.Stat(filepath.Join(backupDir, ".item-templates-crashed.json")); !errors.Is(err, os.ErrNotExist) {
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
	wantSummary := SnapshotSummary{TemplateCount: 2, Vnums: []uint32{11200, 27001}}
	if !reflect.DeepEqual(manifest.Summary, wantSummary) {
		t.Fatalf("unexpected manifest summary: got %#v want %#v", manifest.Summary, wantSummary)
	}
	if len(manifest.Files) != 1 || manifest.Files[0].Filename != "item-templates.json" {
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

func TestFileStoreBackupToTreatsMissingSnapshotAsEmptyAuthoredStore(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "missing", "item-templates.json"))
	backupDir := filepath.Join(t.TempDir(), "item-template-backup")

	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup missing item template store: %v", err)
	}

	if _, err := os.Stat(filepath.Join(backupDir, "item-templates.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing snapshot backup to omit committed template file, stat err=%v", err)
	}
	rawManifest, err := os.ReadFile(filepath.Join(backupDir, BackupManifestFilename))
	if err != nil {
		t.Fatalf("read missing-store backup manifest: %v", err)
	}
	var manifest BackupManifest
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		t.Fatalf("decode missing-store backup manifest: %v", err)
	}
	want := BackupManifest{Format: BackupManifestFormat, Summary: SnapshotSummary{Vnums: []uint32{}}, Files: []BackupManifestFile{}}
	if !reflect.DeepEqual(manifest, want) {
		t.Fatalf("unexpected missing-store backup manifest: got %#v want %#v", manifest, want)
	}
}

func TestFileStoreValidateBackupFromValidatesManifestWithoutMutatingTarget(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "state", "item-templates.json"))
	if err := store.Save(Snapshot{Templates: []Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, EquipSlot: "weapon"},
	}}); err != nil {
		t.Fatalf("save item template snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "item-template-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup item template store: %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "restore-target", "item-templates.json")
	target := NewFileStore(targetPath)

	summary, err := target.ValidateBackupFrom(backupDir)
	if err != nil {
		t.Fatalf("validate item template backup: %v", err)
	}
	want := SnapshotSummary{TemplateCount: 2, Vnums: []uint32{11200, 27001}}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected backup validation summary: got %#v want %#v", summary, want)
	}
	if _, err := os.Stat(filepath.Dir(targetPath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected dry-run validation not to create target dir, stat err=%v", err)
	}
}

func TestFileStoreValidateBackupFromRejectsChecksumMismatch(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "state", "item-templates.json"))
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save item template snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "item-template-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup item template store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "item-templates.json"), []byte(`{"templates":[{"vnum":27001,"name":"Tampered Potion","stackable":true,"max_count":200}]}`), 0o644); err != nil {
		t.Fatalf("tamper backup snapshot: %v", err)
	}

	_, err := NewFileStore(filepath.Join(t.TempDir(), "target", "item-templates.json")).ValidateBackupFrom(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest for checksum mismatch, got %v", err)
	}
}

func TestFileStoreValidateBackupFromRejectsUntrackedBackupEntries(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "state", "item-templates.json"))
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save item template snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "item-template-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup item template store: %v", err)
	}
	if err := os.Mkdir(filepath.Join(backupDir, "nested"), 0o755); err != nil {
		t.Fatalf("create untracked backup dir: %v", err)
	}

	_, err := NewFileStore(filepath.Join(t.TempDir(), "target", "item-templates.json")).ValidateBackupFrom(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest for untracked backup entry, got %v", err)
	}
}

func TestFileStoreRestoreFromRestoresManifestedBackupIntoEmptyStore(t *testing.T) {
	source := NewFileStore(filepath.Join(t.TempDir(), "state", "item-templates.json"))
	snapshot := Snapshot{Templates: []Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, EquipSlot: "weapon"},
	}}
	if err := source.Save(snapshot); err != nil {
		t.Fatalf("save source item templates: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "item-template-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup item templates: %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "restore-target", "item-templates.json")
	target := NewFileStore(targetPath)

	if err := target.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore item template backup: %v", err)
	}
	restored, err := target.Load()
	if err != nil {
		t.Fatalf("load restored item template snapshot: %v", err)
	}
	wantSnapshot := NormalizeSnapshot(snapshot)
	if !reflect.DeepEqual(restored, wantSnapshot) {
		t.Fatalf("unexpected restored item template snapshot:\n got: %#v\nwant: %#v", restored, wantSnapshot)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(targetPath), BackupManifestFilename)); err != nil {
		t.Fatalf("expected restored item template manifest: %v", err)
	}
	summary, err := target.ValidateBackupFrom(filepath.Dir(targetPath))
	if err != nil {
		t.Fatalf("validate restored item template manifest: %v", err)
	}
	wantSummary := SnapshotSummary{TemplateCount: 2, Vnums: []uint32{11200, 27001}}
	if !reflect.DeepEqual(summary, wantSummary) {
		t.Fatalf("unexpected restored manifest summary: got %#v want %#v", summary, wantSummary)
	}
}

func TestFileStoreRestoreFromRestoresMissingSnapshotBackupAsEmptyStore(t *testing.T) {
	source := NewFileStore(filepath.Join(t.TempDir(), "missing", "item-templates.json"))
	backupDir := filepath.Join(t.TempDir(), "item-template-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup missing item template snapshot: %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "restore-target", "item-templates.json")
	target := NewFileStore(targetPath)

	if err := target.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore empty item template backup: %v", err)
	}
	if _, err := target.Load(); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected restored empty item template store to omit snapshot, got %v", err)
	}
	summary, err := target.ValidateBackupFrom(filepath.Dir(targetPath))
	if err != nil {
		t.Fatalf("validate restored empty item template manifest: %v", err)
	}
	want := SnapshotSummary{Vnums: []uint32{}}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected restored empty backup summary: got %#v want %#v", summary, want)
	}
}

func TestFileStoreRestoreFromPreservesCommittedZeroTemplateSnapshot(t *testing.T) {
	source := NewFileStore(filepath.Join(t.TempDir(), "state", "item-templates.json"))
	if err := source.Save(Snapshot{Templates: []Template{}}); err != nil {
		t.Fatalf("save zero-template source snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "item-template-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup zero-template item templates: %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "restore-target", "item-templates.json")
	target := NewFileStore(targetPath)

	if err := target.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore zero-template item template backup: %v", err)
	}
	restored, err := target.Load()
	if err != nil {
		t.Fatalf("expected restored zero-template snapshot to remain committed: %v", err)
	}
	if !reflect.DeepEqual(restored, Snapshot{}) {
		t.Fatalf("unexpected restored zero-template snapshot: got %#v want %#v", restored, Snapshot{})
	}
	manifestRaw, err := os.ReadFile(filepath.Join(filepath.Dir(targetPath), BackupManifestFilename))
	if err != nil {
		t.Fatalf("read restored manifest: %v", err)
	}
	var manifest BackupManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatalf("decode restored manifest: %v", err)
	}
	if len(manifest.Files) != 1 || manifest.Files[0].Filename != "item-templates.json" {
		t.Fatalf("expected restored manifest to preserve committed empty snapshot file, got %#v", manifest.Files)
	}
}

func TestFileStoreRestoreFromRejectsNonEmptyTargetStore(t *testing.T) {
	source := NewFileStore(filepath.Join(t.TempDir(), "state", "item-templates.json"))
	if err := source.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save source item templates: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "item-template-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup item templates: %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "restore-target", "item-templates.json")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("create restore target dir: %v", err)
	}
	stale := filepath.Join(filepath.Dir(targetPath), "stale.json")
	if err := os.WriteFile(stale, []byte(`{"stale":true}`), 0o644); err != nil {
		t.Fatalf("write stale restore target file: %v", err)
	}

	err := NewFileStore(targetPath).RestoreFrom(backupDir)
	if !errors.Is(err, ErrRestoreDirNotEmpty) {
		t.Fatalf("expected ErrRestoreDirNotEmpty for non-empty target, got %v", err)
	}
	if raw, readErr := os.ReadFile(stale); readErr != nil || string(raw) != `{"stale":true}` {
		t.Fatalf("expected stale target file to remain untouched, readErr=%v raw=%q", readErr, string(raw))
	}
}

func TestFileStoreRestoreFromRejectsTargetInsideBackupSource(t *testing.T) {
	source := NewFileStore(filepath.Join(t.TempDir(), "state", "item-templates.json"))
	if err := source.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save source item templates: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "item-template-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup item templates: %v", err)
	}
	targetPath := filepath.Join(backupDir, "nested-restore", "item-templates.json")

	err := NewFileStore(targetPath).RestoreFrom(backupDir)
	if !errors.Is(err, ErrRestoreDirInsideSource) {
		t.Fatalf("expected ErrRestoreDirInsideSource for nested restore target, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Dir(targetPath)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected nested restore target not to be created, stat err=%v", statErr)
	}
}

func TestFileStoreCleanupCrashTempFilesRemovesOnlyCrashTemps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save item template snapshot: %v", err)
	}
	for _, name := range []string{".item-templates-zeta.json", ".item-templates-alpha.json", ".other-temp.json"} {
		if err := os.WriteFile(filepath.Join(filepath.Dir(path), name), []byte(`{"not":"committed"}`), 0o644); err != nil {
			t.Fatalf("write temp file %s: %v", name, err)
		}
	}

	summary, err := store.CleanupCrashTempFiles()
	if err != nil {
		t.Fatalf("cleanup item template crash temp files: %v", err)
	}
	want := SnapshotSummary{TemplateCount: 1, Vnums: []uint32{27001}}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected post-cleanup item template summary: got %#v want %#v", summary, want)
	}
	for _, removed := range []string{".item-templates-zeta.json", ".item-templates-alpha.json"} {
		if _, err := os.Stat(filepath.Join(filepath.Dir(path), removed)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected crash temp %s to be removed, stat err=%v", removed, err)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), ".other-temp.json")); err != nil {
		t.Fatalf("expected unrelated hidden file to be preserved: %v", err)
	}
	if _, err := store.Load(); err != nil {
		t.Fatalf("expected committed item template snapshot to remain loadable: %v", err)
	}
}

func TestFileStoreCleanupCrashTempFilesFailsClosedOnCorruptCommittedSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"templates":[`), 0o644); err != nil {
		t.Fatalf("write corrupt item template snapshot: %v", err)
	}
	crashTemp := filepath.Join(filepath.Dir(path), ".item-templates-crashed.json")
	if err := os.WriteFile(crashTemp, []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write crash temp file: %v", err)
	}

	_, err := NewFileStore(path).CleanupCrashTempFiles()
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot before cleanup, got %v", err)
	}
	if _, statErr := os.Stat(crashTemp); statErr != nil {
		t.Fatalf("expected crash temp file to remain after failed cleanup: %v", statErr)
	}
}

func TestFileStoreValidateFailsClosedForCorruptCommittedSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"templates":[`), 0o644); err != nil {
		t.Fatalf("write corrupt item template snapshot: %v", err)
	}

	_, err := NewFileStore(path).Validate()
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot from validation, got %v", err)
	}
}

func TestFileStoreSaveWritesDeterministicSortedSnapshotAndReplacesPreviousContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	first := Snapshot{Templates: []Template{
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, EquipSlot: "weapon"},
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 50, SellCountPerGold: true},
		{Vnum: 50053, Name: "Polished Helmet", Stackable: false, MaxCount: 1, EquipSlot: "head"},
	}}

	if err := store.Save(first); err != nil {
		t.Fatalf("save first snapshot: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot: %v", err)
	}
	wantFirst := "{\n  \"templates\": [\n    {\n      \"vnum\": 11200,\n      \"name\": \"Wooden Sword\",\n      \"stackable\": false,\n      \"max_count\": 1,\n      \"equip_slot\": \"weapon\"\n    },\n    {\n      \"vnum\": 27001,\n      \"name\": \"Small Red Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"shop_buy_price\": 50,\n      \"sell_count_per_gold\": true\n    },\n    {\n      \"vnum\": 50053,\n      \"name\": \"Polished Helmet\",\n      \"stackable\": false,\n      \"max_count\": 1,\n      \"equip_slot\": \"head\"\n    }\n  ]\n}\n"
	if string(raw) != wantFirst {
		t.Fatalf("unexpected deterministic first snapshot:\n got: %s\nwant: %s", string(raw), wantFirst)
	}

	second := Snapshot{Templates: []Template{{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200}}}
	if err := store.Save(second); err != nil {
		t.Fatalf("save replacement snapshot: %v", err)
	}
	raw, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replacement snapshot: %v", err)
	}
	wantSecond := "{\n  \"templates\": [\n    {\n      \"vnum\": 27002,\n      \"name\": \"Small Blue Potion\",\n      \"stackable\": true,\n      \"max_count\": 200\n    }\n  ]\n}\n"
	if string(raw) != wantSecond {
		t.Fatalf("unexpected replacement snapshot:\n got: %s\nwant: %s", string(raw), wantSecond)
	}
}

func TestFileStoreLoadReturnsNotFoundForMissingSnapshot(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "state", "item-templates.json"))
	_, err := store.Load()
	if !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected ErrSnapshotNotFound, got %v", err)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesHighlightMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:      27001,
		Name:      "Highlighted Red Potion",
		Stackable: true,
		MaxCount:  200,
		Highlight: true,
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with highlight metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with highlight metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with highlight metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with highlight metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27001,\n      \"name\": \"Highlighted Red Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"highlight\": true\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with highlight metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesClientVisibleFlagMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:           71085,
		Name:           "Rare Unique Confirm Charm",
		Stackable:      false,
		MaxCount:       1,
		Refineable:     true,
		Save:           true,
		SlowQuery:      true,
		Rare:           true,
		Unique:         true,
		MakeCount:      true,
		Irremovable:    true,
		ConfirmWhenUse: true,
		Log:            true,
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with client-visible flag metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with client-visible flag metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with client-visible flag metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with client-visible flag metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 71085,\n      \"name\": \"Rare Unique Confirm Charm\",\n      \"stackable\": false,\n      \"max_count\": 1,\n      \"refineable\": true,\n      \"save\": true,\n      \"slow_query\": true,\n      \"rare\": true,\n      \"unique\": true,\n      \"make_count\": true,\n      \"irremovable\": true,\n      \"confirm_when_use\": true,\n      \"log\": true\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with client-visible flag metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesClientVisibleUseFlagMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:       71123,
		Name:       "Quest Applicable Charm",
		Stackable:  false,
		MaxCount:   1,
		QuestUse:   true,
		Applicable: true,
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with client-visible use flag metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with client-visible use flag metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with client-visible use flag metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with client-visible use flag metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 71123,\n      \"name\": \"Quest Applicable Charm\",\n      \"stackable\": false,\n      \"max_count\": 1,\n      \"quest_use\": true,\n      \"applicable\": true\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with client-visible use flag metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesConfirmWhenUseConsumableMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:           27006,
		Name:           "Confirmable Elixir",
		Stackable:      true,
		MaxCount:       200,
		ConfirmWhenUse: true,
		UseEffect: &UseEffect{
			PointType:  7,
			PointIndex: 1,
			PointDelta: 25,
			Message:    "confirm:27006:+25",
		},
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with confirm-when-use consumable metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with confirm-when-use consumable metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with confirm-when-use consumable metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with confirm-when-use consumable metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27006,\n      \"name\": \"Confirmable Elixir\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"confirm_when_use\": true,\n      \"use_effect\": {\n        \"point_type\": 7,\n        \"point_index\": 1,\n        \"point_delta\": 25,\n        \"message\": \"confirm:27006:+25\"\n      }\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with confirm-when-use consumable metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesStorageAndShopAntiFlagMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:        71124,
		Name:        "Protected Storage Charm",
		Stackable:   false,
		MaxCount:    1,
		AntiSave:    true,
		AntiPKDrop:  true,
		AntiMyShop:  true,
		AntiSafebox: true,
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with storage/shop anti-flag metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with storage/shop anti-flag metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with storage/shop anti-flag metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with storage/shop anti-flag metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 71124,\n      \"name\": \"Protected Storage Charm\",\n      \"stackable\": false,\n      \"max_count\": 1,\n      \"anti_save\": true,\n      \"anti_pk_drop\": true,\n      \"anti_myshop\": true,\n      \"anti_safebox\": true\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with storage/shop anti-flag metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreLoadRejectsMalformedOrInvalidSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write malformed snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for malformed json, got %v", err)
	}

	unknownField := []byte("{\"templates\":[{\"vnum\":27001,\"name\":\"Small Red Potion\",\"stackable\":true,\"max_count\":200,\"unowned_effect\":true}]}")
	if err := os.WriteFile(path, unknownField, 0o644); err != nil {
		t.Fatalf("write unknown-field snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for unknown item-template field, got %v", err)
	}

	trailingJSON := []byte("{\"templates\":[{\"vnum\":27001,\"name\":\"Small Red Potion\",\"stackable\":true,\"max_count\":200}]}{}")
	if err := os.WriteFile(path, trailingJSON, 0o644); err != nil {
		t.Fatalf("write trailing-json snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for trailing item-template JSON, got %v", err)
	}

	zeroVnum := Snapshot{Templates: []Template{{Vnum: 0, Name: "Broken", Stackable: true, MaxCount: 1}}}
	if err := store.Save(zeroVnum); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for zero vnum, got %v", err)
	}
	blankName := Snapshot{Templates: []Template{{Vnum: 27001, Name: "   ", Stackable: true, MaxCount: 1}}}
	if err := store.Save(blankName); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for blank name, got %v", err)
	}
	zeroMaxCount := Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 0}}}
	if err := store.Save(zeroMaxCount); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for zero max count, got %v", err)
	}
	overClientCountRange := Snapshot{Templates: []Template{{Vnum: 27001, Name: "Huge Red Potion Stack", Stackable: true, MaxCount: 256}}}
	if err := store.Save(overClientCountRange); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for max_count beyond bootstrap client count range, got %v", err)
	}
	nonStackableMultiCount := Snapshot{Templates: []Template{{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 2, EquipSlot: "weapon"}}}
	if err := store.Save(nonStackableMultiCount); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for non-stackable max_count != 1, got %v", err)
	}
	invalidEquipSlot := Snapshot{Templates: []Template{{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, EquipSlot: "cape"}}}
	if err := store.Save(invalidEquipSlot); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for invalid equip slot, got %v", err)
	}
	equipWithUseEffect := Snapshot{Templates: []Template{{
		Vnum:      11200,
		Name:      "Consumable Wooden Sword",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: inventory.EquipmentSlotWeapon.String(),
		UseEffect: &UseEffect{PointType: 7, PointIndex: 1, PointDelta: 25, Message: "must not use equipment"},
	}}}
	if err := store.Save(equipWithUseEffect); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for equipment template with use_effect, got %v", err)
	}
	duplicate := Snapshot{Templates: []Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
		{Vnum: 27001, Name: "Duplicate Potion", Stackable: true, MaxCount: 200},
	}}
	if err := store.Save(duplicate); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for duplicate vnum, got %v", err)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesUseEffectMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:      27002,
		Name:      "Practice Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &UseEffect{
			PointType:  7,
			PointIndex: 1,
			PointDelta: 25,
			Message:    "consume:27002:+25",
		},
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with use effect metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with use effect metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with use effect metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with use effect metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27002,\n      \"name\": \"Practice Elixir\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"use_effect\": {\n        \"point_type\": 7,\n        \"point_index\": 1,\n        \"point_delta\": 25,\n        \"message\": \"consume:27002:+25\"\n      }\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with use effect metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesNegativeUseEffectMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:      27006,
		Name:      "Cursed Practice Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &UseEffect{
			PointType:  7,
			PointIndex: 1,
			PointDelta: -25,
			Message:    "consume:27006:-25",
		},
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with negative use effect metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with negative use effect metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with negative use effect metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with negative use effect metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27006,\n      \"name\": \"Cursed Practice Elixir\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"use_effect\": {\n        \"point_type\": 7,\n        \"point_index\": 1,\n        \"point_delta\": -25,\n        \"message\": \"consume:27006:-25\"\n      }\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with negative use effect metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveRejectsInvalidUseEffectMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)

	missingMessage := Snapshot{Templates: []Template{{
		Vnum:      27002,
		Name:      "Practice Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &UseEffect{PointType: 7, PointIndex: 1, PointDelta: 25},
	}}}
	if err := store.Save(missingMessage); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for missing use-effect message, got %v", err)
	}

	zeroType := Snapshot{Templates: []Template{{
		Vnum:      27002,
		Name:      "Practice Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &UseEffect{PointType: 0, PointIndex: 1, PointDelta: 25, Message: "consume:27002:+25"},
	}}}
	if err := store.Save(zeroType); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for zero use-effect point type, got %v", err)
	}

	zeroDelta := Snapshot{Templates: []Template{{
		Vnum:      27002,
		Name:      "Practice Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &UseEffect{PointType: 7, PointIndex: 1, PointDelta: 0, Message: "consume:27002:+25"},
	}}}
	if err := store.Save(zeroDelta); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for zero use-effect point delta, got %v", err)
	}

	nonReversibleDelta := Snapshot{Templates: []Template{{
		Vnum:      27002,
		Name:      "Practice Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &UseEffect{PointType: 7, PointIndex: 1, PointDelta: -1 << 31, Message: "consume:27002:min"},
	}}}
	if err := store.Save(nonReversibleDelta); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for non-reversible use-effect point delta, got %v", err)
	}

	invalidPointIndex := Snapshot{Templates: []Template{{
		Vnum:      27002,
		Name:      "Practice Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &UseEffect{PointType: 7, PointIndex: 255, PointDelta: 25, Message: "consume:27002:+25"},
	}}}
	if err := store.Save(invalidPointIndex); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for out-of-range use-effect point index, got %v", err)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesDisplaySocketAndAttributeMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:      71084,
		Name:      "Socketed Practice Charm",
		Stackable: false,
		MaxCount:  1,
		Sockets:   SocketValues{11, -2, 33},
		Attributes: AttributeValues{
			{Type: 1, Value: 25},
			{Type: 7, Value: -3},
		},
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with display socket/attribute metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with display socket/attribute metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with display socket/attribute metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with display socket/attribute metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 71084,\n      \"name\": \"Socketed Practice Charm\",\n      \"stackable\": false,\n      \"max_count\": 1,\n      \"sockets\": [\n        11,\n        -2,\n        33\n      ],\n      \"attributes\": [\n        {\n          \"type\": 1,\n          \"value\": 25\n        },\n        {\n          \"type\": 7,\n          \"value\": -3\n        },\n        {\n          \"type\": 0,\n          \"value\": 0\n        },\n        {\n          \"type\": 0,\n          \"value\": 0\n        },\n        {\n          \"type\": 0,\n          \"value\": 0\n        },\n        {\n          \"type\": 0,\n          \"value\": 0\n        },\n        {\n          \"type\": 0,\n          \"value\": 0\n        }\n      ]\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with display socket/attribute metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveRejectsInvalidDisplayAttributeMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)

	zeroTypeWithValue := Snapshot{Templates: []Template{{
		Vnum:      71084,
		Name:      "Broken Practice Charm",
		Stackable: false,
		MaxCount:  1,
		Attributes: AttributeValues{
			{Type: 0, Value: 25},
		},
	}}}
	if err := store.Save(zeroTypeWithValue); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for zero display attribute type with value, got %v", err)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesAntiFlagMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:         27003,
		Name:         "Bound Practice Potion",
		Stackable:    true,
		MaxCount:     200,
		AntiSell:     true,
		AntiDrop:     true,
		AntiGive:     true,
		AntiStack:    true,
		AntiGet:      true,
		AntiMale:     true,
		AntiFemale:   true,
		AntiWarrior:  true,
		AntiAssassin: true,
		AntiSura:     true,
		AntiShaman:   true,
		AntiEmpireA:  true,
		AntiEmpireB:  true,
		AntiEmpireC:  true,
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with anti-flag metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with anti-flag metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with anti-flag metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with anti-flag metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27003,\n      \"name\": \"Bound Practice Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"anti_sell\": true,\n      \"anti_drop\": true,\n      \"anti_give\": true,\n      \"anti_stack\": true,\n      \"anti_get\": true,\n      \"anti_male\": true,\n      \"anti_female\": true,\n      \"anti_warrior\": true,\n      \"anti_assassin\": true,\n      \"anti_sura\": true,\n      \"anti_shaman\": true,\n      \"anti_empire_a\": true,\n      \"anti_empire_b\": true,\n      \"anti_empire_c\": true\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with anti-flag metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesQuestUseMultipleFlagMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:             71124,
		Name:             "Repeatable Quest Charm",
		Stackable:        false,
		MaxCount:         1,
		QuestUseMultiple: true,
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with quest-use-multiple flag metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with quest-use-multiple flag metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with quest-use-multiple flag metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with quest-use-multiple flag metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 71124,\n      \"name\": \"Repeatable Quest Charm\",\n      \"stackable\": false,\n      \"max_count\": 1,\n      \"quest_use_multiple\": true\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with quest-use-multiple flag metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesMinLevelRestriction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:      27004,
		Name:      "Veteran Practice Potion",
		Stackable: true,
		MaxCount:  200,
		MinLevel:  10,
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with min-level metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with min-level metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with min-level metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with min-level metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27004,\n      \"name\": \"Veteran Practice Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"min_level\": 10\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with min-level metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesEquipEffectMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:      12200,
		Name:      "Practice Blade",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: "weapon",
		EquipEffect: &PointEffect{
			PointType:  1,
			PointIndex: 1,
			PointDelta: 10,
		},
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with equip effect metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with equip effect metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with equip effect metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with equip effect metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 12200,\n      \"name\": \"Practice Blade\",\n      \"stackable\": false,\n      \"max_count\": 1,\n      \"equip_slot\": \"weapon\",\n      \"equip_effect\": {\n        \"point_type\": 1,\n        \"point_index\": 1,\n        \"point_delta\": 10\n      }\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with equip effect metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesNegativeEquipEffectMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:      12201,
		Name:      "Cursed Practice Blade",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: "weapon",
		EquipEffect: &PointEffect{
			PointType:  1,
			PointIndex: 1,
			PointDelta: -10,
		},
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with negative equip effect metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with negative equip effect metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with negative equip effect metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with negative equip effect metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 12201,\n      \"name\": \"Cursed Practice Blade\",\n      \"stackable\": false,\n      \"max_count\": 1,\n      \"equip_slot\": \"weapon\",\n      \"equip_effect\": {\n        \"point_type\": 1,\n        \"point_index\": 1,\n        \"point_delta\": -10\n      }\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with negative equip effect metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveRejectsInvalidEquipEffectMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)

	missingEquipSlot := Snapshot{Templates: []Template{{
		Vnum:        12200,
		Name:        "Practice Blade",
		Stackable:   false,
		MaxCount:    1,
		EquipEffect: &PointEffect{PointType: 1, PointIndex: 1, PointDelta: 10},
	}}}
	if err := store.Save(missingEquipSlot); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for equip-effect without equip slot, got %v", err)
	}

	zeroType := Snapshot{Templates: []Template{{
		Vnum:        12200,
		Name:        "Practice Blade",
		Stackable:   false,
		MaxCount:    1,
		EquipSlot:   "weapon",
		EquipEffect: &PointEffect{PointType: 0, PointIndex: 1, PointDelta: 10},
	}}}
	if err := store.Save(zeroType); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for zero equip-effect point type, got %v", err)
	}

	zeroDelta := Snapshot{Templates: []Template{{
		Vnum:        12200,
		Name:        "Practice Blade",
		Stackable:   false,
		MaxCount:    1,
		EquipSlot:   "weapon",
		EquipEffect: &PointEffect{PointType: 1, PointIndex: 1, PointDelta: 0},
	}}}
	if err := store.Save(zeroDelta); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for zero equip-effect point delta, got %v", err)
	}

	nonReversibleDelta := Snapshot{Templates: []Template{{
		Vnum:        12200,
		Name:        "Practice Blade",
		Stackable:   false,
		MaxCount:    1,
		EquipSlot:   "weapon",
		EquipEffect: &PointEffect{PointType: 1, PointIndex: 1, PointDelta: -1 << 31},
	}}}
	if err := store.Save(nonReversibleDelta); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for non-reversible equip-effect point delta, got %v", err)
	}

	invalidPointIndex := Snapshot{Templates: []Template{{
		Vnum:        12200,
		Name:        "Practice Blade",
		Stackable:   false,
		MaxCount:    1,
		EquipSlot:   "weapon",
		EquipEffect: &PointEffect{PointType: 1, PointIndex: 255, PointDelta: 10},
	}}}
	if err := store.Save(invalidPointIndex); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for out-of-range equip-effect point index, got %v", err)
	}
}
