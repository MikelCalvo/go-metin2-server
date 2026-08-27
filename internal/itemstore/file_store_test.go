package itemstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

func TestFileStoreSaveThenLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, EquipSlot: "weapon"},
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 50, ShopSellPrice: 13, SellCountPerGold: true, Highlight: true, AntiSell: true},
	}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot: %v", err)
	}
	if !strings.Contains(string(raw), "\"shop_buy_price\": 50,\n      \"shop_sell_price\": 13,") {
		t.Fatalf("expected shop_sell_price to persist immediately after shop_buy_price in deterministic JSON, got:\n%s", raw)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestFileStoreRejectsInvalidUTF8TemplateStrings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	invalid := string([]byte{'V', 'i', 's', 'i', 'b', 'l', 'e', 0xff, 'H', 'i', 'd', 'd', 'e', 'n'})
	if utf8.ValidString(invalid) {
		t.Fatal("test fixture must contain invalid UTF-8")
	}
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: invalid, Stackable: true, MaxCount: 200}}}); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for invalid UTF-8 item template name, got %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create item template test dir: %v", err)
	}
	body := []byte(`{"templates":[{"vnum":27001,"name":"Visible`)
	body = append(body, 0xff)
	body = append(body, []byte(`Hidden","stackable":true,"max_count":200}]}`)...)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write invalid UTF-8 item template snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for invalid UTF-8 item template name on load, got %v", err)
	}
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 27002, Name: "Practice Elixir", Stackable: true, MaxCount: 200, UseEffect: &UseEffect{PointType: 7, PointIndex: 1, PointDelta: 25, Message: invalid}}}}); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for invalid UTF-8 use-effect message, got %v", err)
	}
}

func TestFileStoreSaveEmptySnapshotWritesDeterministicEmptyTemplateArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)

	if err := store.Save(Snapshot{}); err != nil {
		t.Fatalf("save empty snapshot: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read empty snapshot: %v", err)
	}
	wantRaw := "{\n  \"templates\": []\n}\n"
	if string(raw) != wantRaw {
		t.Fatalf("unexpected empty snapshot JSON:\n got: %q\nwant: %q", string(raw), wantRaw)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load empty snapshot: %v", err)
	}
	if len(got.Templates) != 0 {
		t.Fatalf("expected empty template list, got %#v", got.Templates)
	}
}

func TestFileStoreLoadRejectsNullTemplateCollection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create item template test dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"templates":null}`), 0o644); err != nil {
		t.Fatalf("write null-template snapshot: %v", err)
	}

	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for null template collection, got %v", err)
	}
}

func TestFileStoreLoadRejectsSymlinkedCommittedItemTemplateSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create item template test dir: %v", err)
	}
	target := filepath.Join(t.TempDir(), "outside-item-templates.json")
	if err := os.WriteFile(target, []byte(`{"templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200}]}`), 0o644); err != nil {
		t.Fatalf("write outside item-template snapshot: %v", err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	_, err := store.Load()
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for symlinked item-template snapshot, got %v", err)
	}
}

func TestFileStoreValidateRejectsSymlinkedItemTemplateCrashTempFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save item template snapshot: %v", err)
	}
	target := filepath.Join(t.TempDir(), "outside-item-template-temp.json")
	if err := os.WriteFile(target, []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write outside item-template temp target: %v", err)
	}
	link := filepath.Join(filepath.Dir(path), ".item-templates-crashed.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	_, err := store.Validate()
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for symlinked item-template crash temp file, got %v", err)
	}
}

func TestFileStoreValidateRejectsSymlinkedCommittedItemTemplateSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create item template test dir: %v", err)
	}
	target := filepath.Join(t.TempDir(), "outside-item-templates.json")
	if err := os.WriteFile(target, []byte(`{"templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200}]}`), 0o644); err != nil {
		t.Fatalf("write outside item-template snapshot: %v", err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	_, err := store.Validate()
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for symlinked committed item-template snapshot, got %v", err)
	}
}

func TestFileStoreLoadRejectsMissingTemplateCollection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create item template test dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write missing-template snapshot: %v", err)
	}

	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for missing template collection, got %v", err)
	}
}

func TestFileStoreRejectsTemplateNameWithEmbeddedNUL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	invalid := Snapshot{Templates: []Template{{
		Vnum:      27001,
		Name:      "Small\x00Red Potion",
		Stackable: true,
		MaxCount:  200,
	}}}

	if err := store.Save(invalid); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for embedded-NUL template name, got %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create item template test dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"templates":[{"vnum":27001,"name":"Small\u0000Red Potion","stackable":true,"max_count":200}]}`), 0o644); err != nil {
		t.Fatalf("write embedded-NUL template-name snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot when loading embedded-NUL template name, got %v", err)
	}
}

func TestFileStoreRejectsShopBuyPriceAboveLegacyCarrier(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	snapshot := Snapshot{Templates: []Template{{
		Vnum:         27001,
		Name:         "Overflow Price Potion",
		Stackable:    true,
		MaxCount:     200,
		ShopBuyPrice: uint64(^uint32(0)) + 1,
	}}}

	if err := store.Save(snapshot); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot when saving shop_buy_price above legacy uint32 carrier, got %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create item template test dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"templates":[{"vnum":27001,"name":"Overflow Price Potion","stackable":true,"max_count":200,"shop_buy_price":4294967296}]}`), 0o644); err != nil {
		t.Fatalf("write oversized shop_buy_price snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot when loading shop_buy_price above legacy uint32 carrier, got %v", err)
	}
}

func TestFileStoreRejectsShopSellPriceAbovePointChangeCarrier(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	snapshot := Snapshot{Templates: []Template{{
		Vnum:          27001,
		Name:          "Overflow Sell Potion",
		Stackable:     true,
		MaxCount:      200,
		ShopSellPrice: uint64(1 << 31),
	}}}

	if err := store.Save(snapshot); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot when saving shop_sell_price above point-change carrier, got %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create item template test dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"templates":[{"vnum":27001,"name":"Overflow Sell Potion","stackable":true,"max_count":200,"shop_sell_price":2147483648}]}`), 0o644); err != nil {
		t.Fatalf("write oversized shop_sell_price snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot when loading shop_sell_price above point-change carrier, got %v", err)
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

func TestFileStoreBackupToRollsBackSnapshotWhenSaveSyncFailsAfterCommit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save source item template snapshot: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "item-template-backup")
	injectedErr := errors.New("injected item-template backup snapshot sync failure")
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
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save source item template snapshot: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "item-template-backup")
	injectedErr := errors.New("injected item-template final backup sync failure")
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
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	snapshot := Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}
	if err := store.Save(snapshot); err != nil {
		t.Fatalf("save item template snapshot: %v", err)
	}
	summary := SnapshotSummary{TemplateCount: 1, Vnums: []uint32{27001}}
	if err := writeBackupManifest(filepath.Dir(path), filepath.Base(path), summary, true); err != nil {
		t.Fatalf("write backup manifest: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"templates":[{"vnum":27001,"name":"Tampered Potion","stackable":true,"max_count":200}]}`), 0o644); err != nil {
		t.Fatalf("tamper item template snapshot after manifest write: %v", err)
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
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save item template snapshot: %v", err)
	}
	if err := writeBackupManifest(filepath.Dir(path), filepath.Base(path), SnapshotSummary{TemplateCount: 1, Vnums: []uint32{27001}}, true); err != nil {
		t.Fatalf("write active backup manifest: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"templates":[{"vnum":27001,"name":"Tampered Potion","stackable":true,"max_count":200}]}`), 0o644); err != nil {
		t.Fatalf("tamper active item template snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "item-template-backup")

	err := store.BackupTo(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest before backup, got %v", err)
	}
	if _, statErr := os.Stat(backupDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected rejected backup not to create destination, stat err=%v", statErr)
	}
}

func TestFileStoreBackupToRejectsDanglingActiveBackupManifestSymlinkBeforeCreatingDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save item template snapshot: %v", err)
	}
	missingTarget := filepath.Join(t.TempDir(), "missing-item-template-manifest.json")
	if err := os.Symlink(missingTarget, filepath.Join(filepath.Dir(path), BackupManifestFilename)); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "item-template-backup")

	err := store.BackupTo(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest before backup, got %v", err)
	}
	if _, statErr := os.Stat(backupDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected rejected backup not to create destination, stat err=%v", statErr)
	}
}

func TestFileStoreBackupToRejectsSymlinkedCrashTempBeforeCreatingDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save item template snapshot: %v", err)
	}
	target := filepath.Join(t.TempDir(), "outside-item-template-temp.json")
	if err := os.WriteFile(target, []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write outside item-template temp target: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(filepath.Dir(path), ".item-templates-crashed.json")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "backup")

	err := store.BackupTo(backupDir)
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for symlinked item-template crash temp before backup, got %v", err)
	}
	if _, statErr := os.Stat(backupDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected rejected backup not to create destination, stat err=%v", statErr)
	}
}

func TestFileStoreValidateRejectsMalformedBackupManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save item template snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), BackupManifestFilename), []byte(`{"format":"manual"}`), 0o644); err != nil {
		t.Fatalf("write malformed backup manifest: %v", err)
	}
	if err := store.validateActiveBackupManifest(); !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected active manifest preflight to detect malformed manifest, got %v", err)
	}

	_, err := store.Validate()
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest for malformed active manifest, got %v", err)
	}
}

func TestFileStoreValidateRejectsDanglingActiveBackupManifestSymlink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save item template snapshot: %v", err)
	}
	missingTarget := filepath.Join(t.TempDir(), "missing-item-template-manifest.json")
	if err := os.Symlink(missingTarget, filepath.Join(filepath.Dir(path), BackupManifestFilename)); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	if err := store.validateActiveBackupManifest(); !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected active manifest preflight to detect dangling symlink, got %v", err)
	}
	_, err := store.Validate()
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest for dangling active manifest symlink, got %v", err)
	}
}

func TestFileStoreValidateRejectsInvalidUTF8BackupManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save item template snapshot: %v", err)
	}
	invalidManifest := append([]byte(`{"format":"`+BackupManifestFormat), 0xff)
	invalidManifest = append(invalidManifest, []byte(`","summary":{"template_count":1,"vnums":[27001]},"files":[]}`)...)
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), BackupManifestFilename), invalidManifest, 0o644); err != nil {
		t.Fatalf("write invalid-UTF8 active backup manifest: %v", err)
	}

	_, err := store.Validate()
	if !errors.Is(err, ErrInvalidBackupManifest) || !strings.Contains(err.Error(), "invalid utf-8") {
		t.Fatalf("expected invalid-UTF8 active backup manifest to fail closed with ErrInvalidBackupManifest, got %v", err)
	}
}

func TestFileStoreValidateBackupFromRejectsInvalidUTF8BackupManifest(t *testing.T) {
	backupDir := filepath.Join(t.TempDir(), "item-template-backup")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("create backup dir: %v", err)
	}
	invalidManifest := append([]byte(`{"format":"`+BackupManifestFormat), 0xff)
	invalidManifest = append(invalidManifest, []byte(`","summary":{"vnums":[]},"files":[]}`)...)
	if err := os.WriteFile(filepath.Join(backupDir, BackupManifestFilename), invalidManifest, 0o644); err != nil {
		t.Fatalf("write invalid-UTF8 backup manifest: %v", err)
	}

	_, err := NewFileStore(filepath.Join(t.TempDir(), "target", "item-templates.json")).ValidateBackupFrom(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) || !strings.Contains(err.Error(), "invalid utf-8") {
		t.Fatalf("expected invalid-UTF8 backup manifest dry-run to fail closed with ErrInvalidBackupManifest, got %v", err)
	}
}

func TestFileStoreValidateRejectsBackupManifestOmittingActiveSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save item template snapshot: %v", err)
	}
	manifest := BackupManifest{Format: BackupManifestFormat, Summary: SnapshotSummary{Vnums: []uint32{}}, Files: []BackupManifestFile{}}
	if err := writeJSONFileAtomically(filepath.Dir(path), BackupManifestFilename, manifest, "empty item template backup manifest"); err != nil {
		t.Fatalf("write empty backup manifest: %v", err)
	}

	_, err := store.Validate()
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest for manifest omitting active snapshot, got %v", err)
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

func TestFileStoreValidateBackupFromReportsIgnoredCrashTempFiles(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "state", "item-templates.json"))
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save item template snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "item-template-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup item template store: %v", err)
	}
	for _, name := range []string{".item-templates-zeta.json", ".item-templates-alpha.json"} {
		if err := os.WriteFile(filepath.Join(backupDir, name), []byte(`{"not":"committed"}`), 0o644); err != nil {
			t.Fatalf("write backup crash temp %s: %v", name, err)
		}
	}

	summary, err := NewFileStore(filepath.Join(t.TempDir(), "target", "item-templates.json")).ValidateBackupFrom(backupDir)
	if err != nil {
		t.Fatalf("validate item template backup with crash temps: %v", err)
	}
	want := SnapshotSummary{
		TemplateCount:  1,
		Vnums:          []uint32{27001},
		CrashTempCount: 2,
		CrashTempFiles: []string{".item-templates-alpha.json", ".item-templates-zeta.json"},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected backup validation summary with crash temps: got %#v want %#v", summary, want)
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

func TestFileStoreValidateBackupFromRejectsBackupManifestSymlink(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "state", "item-templates.json"))
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save item template snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "item-template-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup item template store: %v", err)
	}
	manifestPath := filepath.Join(backupDir, BackupManifestFilename)
	manifestRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read backup manifest: %v", err)
	}
	externalManifest := filepath.Join(t.TempDir(), BackupManifestFilename)
	if err := os.WriteFile(externalManifest, manifestRaw, 0o644); err != nil {
		t.Fatalf("write external backup manifest: %v", err)
	}
	if err := os.Remove(manifestPath); err != nil {
		t.Fatalf("remove in-place backup manifest: %v", err)
	}
	if err := os.Symlink(externalManifest, manifestPath); err != nil {
		t.Fatalf("symlink backup manifest: %v", err)
	}

	_, err = NewFileStore(filepath.Join(t.TempDir(), "target", "item-templates.json")).ValidateBackupFrom(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest for symlinked backup manifest, got %v", err)
	}
}

func TestFileStoreValidateBackupFromRejectsManifestedSnapshotSymlink(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "state", "item-templates.json"))
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save item template snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "item-template-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup item template store: %v", err)
	}
	snapshotPath := filepath.Join(backupDir, filepath.Base(store.path))
	snapshotRaw, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("read backup snapshot: %v", err)
	}
	externalSnapshot := filepath.Join(t.TempDir(), filepath.Base(snapshotPath))
	if err := os.WriteFile(externalSnapshot, snapshotRaw, 0o644); err != nil {
		t.Fatalf("write external backup snapshot: %v", err)
	}
	if err := os.Remove(snapshotPath); err != nil {
		t.Fatalf("remove in-place backup snapshot: %v", err)
	}
	if err := os.Symlink(externalSnapshot, snapshotPath); err != nil {
		t.Fatalf("symlink backup snapshot: %v", err)
	}

	_, err = NewFileStore(filepath.Join(t.TempDir(), "target", "item-templates.json")).ValidateBackupFrom(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest for symlinked manifested snapshot, got %v", err)
	}
}

func TestFileStoreValidateBackupFromRejectsCrashTempSymlink(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "state", "item-templates.json"))
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save item template snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "item-template-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup item template store: %v", err)
	}
	externalTemp := filepath.Join(t.TempDir(), ".item-templates-crashed.json")
	if err := os.WriteFile(externalTemp, []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write external crash temp: %v", err)
	}
	if err := os.Symlink(externalTemp, filepath.Join(backupDir, ".item-templates-crashed.json")); err != nil {
		t.Fatalf("symlink crash-temp-shaped backup entry: %v", err)
	}

	_, err := NewFileStore(filepath.Join(t.TempDir(), "target", "item-templates.json")).ValidateBackupFrom(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest for crash-temp-shaped symlink, got %v", err)
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

func TestFileStoreSaveRemovesStaleBackupManifestAfterMutation(t *testing.T) {
	source := NewFileStore(filepath.Join(t.TempDir(), "source", "item-templates.json"))
	if err := source.Save(Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}); err != nil {
		t.Fatalf("save source item templates: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "item-template-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup item templates: %v", err)
	}

	targetPath := filepath.Join(t.TempDir(), "restore-target", "item-templates.json")
	restored := NewFileStore(targetPath)
	if err := restored.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore item templates: %v", err)
	}
	manifestPath := filepath.Join(filepath.Dir(targetPath), BackupManifestFilename)
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("expected restored manifest before mutation: %v", err)
	}

	mutated := Snapshot{Templates: []Template{{Vnum: 27002, Name: "Medium Red Potion", Stackable: true, MaxCount: 200}}}
	if err := restored.Save(mutated); err != nil {
		t.Fatalf("save mutated restored item templates: %v", err)
	}
	if _, err := os.Stat(manifestPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected stale restored backup manifest to be removed after item-template mutation, stat err=%v", err)
	}
	got, err := restored.Load()
	if err != nil {
		t.Fatalf("load mutated restored item templates: %v", err)
	}
	if !reflect.DeepEqual(got, NormalizeSnapshot(mutated)) {
		t.Fatalf("unexpected mutated item template snapshot: got %#v want %#v", got, NormalizeSnapshot(mutated))
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

func TestFileStoreSaveThenLoadRoundTripPreservesRefineRejectMessage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:             11200,
		Name:             "Practice Blade",
		Stackable:        false,
		MaxCount:         1,
		RefineRejectText: "This item cannot be refined yet.",
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with refine rejection metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with refine rejection metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with refine rejection metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with refine rejection metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 11200,\n      \"name\": \"Practice Blade\",\n      \"stackable\": false,\n      \"max_count\": 1,\n      \"refine_reject_message\": \"This item cannot be refined yet.\"\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with refine rejection metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesRefineInformationMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:       11200,
		Name:       "Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &RefineInfo{
			ResultVnum:  11201,
			Cost:        2500,
			Probability: 75,
			Materials: []RefineMaterial{
				{Vnum: 27001, Count: 2},
				{Vnum: 27002, Count: 3},
			},
		},
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with refine information metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with refine information metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with refine information metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with refine information metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 11200,\n      \"name\": \"Practice Blade\",\n      \"stackable\": false,\n      \"max_count\": 1,\n      \"refineable\": true,\n      \"refine_info\": {\n        \"result_vnum\": 11201,\n        \"cost\": 2500,\n        \"probability\": 75,\n        \"materials\": [\n          {\n            \"vnum\": 27001,\n            \"count\": 2\n          },\n          {\n            \"vnum\": 27002,\n            \"count\": 3\n          }\n        ]\n      }\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with refine information metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreRejectsInvalidRefineInformationMetadata(t *testing.T) {
	cases := []struct {
		name     string
		template Template
	}{
		{
			name: "without refineable flag",
			template: Template{
				Vnum:       11200,
				Name:       "Practice Blade",
				Stackable:  false,
				MaxCount:   1,
				RefineInfo: &RefineInfo{ResultVnum: 11201, Cost: 2500, Probability: 75},
			},
		},
		{
			name: "zero result vnum",
			template: Template{
				Vnum:       11200,
				Name:       "Practice Blade",
				Stackable:  false,
				MaxCount:   1,
				Refineable: true,
				RefineInfo: &RefineInfo{Cost: 2500, Probability: 75},
			},
		},
		{
			name: "too many material rows",
			template: Template{
				Vnum:       11200,
				Name:       "Practice Blade",
				Stackable:  false,
				MaxCount:   1,
				Refineable: true,
				RefineInfo: &RefineInfo{ResultVnum: 11201, Cost: 2500, Probability: 75, Materials: []RefineMaterial{{Vnum: 27001, Count: 1}, {Vnum: 27002, Count: 1}, {Vnum: 27003, Count: 1}, {Vnum: 27004, Count: 1}, {Vnum: 27005, Count: 1}, {Vnum: 27006, Count: 1}}},
			},
		},
		{
			name: "negative cost",
			template: Template{
				Vnum:       11200,
				Name:       "Practice Blade",
				Stackable:  false,
				MaxCount:   1,
				Refineable: true,
				RefineInfo: &RefineInfo{ResultVnum: 11201, Cost: -1, Probability: 75},
			},
		},
		{
			name: "zero material count",
			template: Template{
				Vnum:       11200,
				Name:       "Practice Blade",
				Stackable:  false,
				MaxCount:   1,
				Refineable: true,
				RefineInfo: &RefineInfo{ResultVnum: 11201, Cost: 2500, Probability: 75, Materials: []RefineMaterial{{Vnum: 27001}}},
			},
		},
		{
			name: "probability over one hundred",
			template: Template{
				Vnum:       11200,
				Name:       "Practice Blade",
				Stackable:  false,
				MaxCount:   1,
				Refineable: true,
				RefineInfo: &RefineInfo{ResultVnum: 11201, Cost: 2500, Probability: 101},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state", "item-templates.json")
			store := NewFileStore(path)
			if err := store.Save(Snapshot{Templates: []Template{tc.template}}); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot for %s, got %v", tc.name, err)
			}
		})
	}
}

func TestFileStoreRejectsContradictoryRefineRejectMessage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	snapshot := Snapshot{Templates: []Template{{
		Vnum:             11200,
		Name:             "Refineable Blade",
		Stackable:        false,
		MaxCount:         1,
		Refineable:       true,
		RefineRejectText: "This item cannot be refined yet.",
	}}}

	if err := store.Save(snapshot); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot when saving refine_reject_message on a refineable template, got %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create item template test dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"templates":[{"vnum":11200,"name":"Refineable Blade","stackable":false,"max_count":1,"refineable":true,"refine_reject_message":"This item cannot be refined yet."}]}`), 0o644); err != nil {
		t.Fatalf("write contradictory refine rejection snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot when loading refine_reject_message on a refineable template, got %v", err)
	}
}

func TestFileStoreRejectsRefineRejectMessageWithEmbeddedNUL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	snapshot := Snapshot{Templates: []Template{{
		Vnum:             11200,
		Name:             "Practice Blade",
		Stackable:        false,
		MaxCount:         1,
		RefineRejectText: "refine\x00blocked",
	}}}

	if err := store.Save(snapshot); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot when saving refine_reject_message with embedded NUL, got %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create item template test dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"templates":[{"vnum":11200,"name":"Practice Blade","stackable":false,"max_count":1,"refine_reject_message":"refine\u0000blocked"}]}`), 0o644); err != nil {
		t.Fatalf("write embedded-NUL refine rejection snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot when loading refine_reject_message with embedded NUL, got %v", err)
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
	const safeboxRejectText = "This item cannot be placed in storage."
	want := Snapshot{Templates: []Template{{
		Vnum:              71124,
		Name:              "Protected Storage Charm",
		Stackable:         false,
		MaxCount:          1,
		AntiSave:          true,
		AntiPKDrop:        true,
		AntiMyShop:        true,
		AntiSafebox:       true,
		SafeboxRejectText: safeboxRejectText,
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
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 71124,\n      \"name\": \"Protected Storage Charm\",\n      \"stackable\": false,\n      \"max_count\": 1,\n      \"anti_save\": true,\n      \"anti_pk_drop\": true,\n      \"anti_myshop\": true,\n      \"anti_safebox\": true,\n      \"safebox_reject_message\": \"This item cannot be placed in storage.\"\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with storage/shop anti-flag metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreRejectsInvalidSafeboxRejectTextMetadata(t *testing.T) {
	cases := []struct {
		name     string
		snapshot Snapshot
		rawJSON  string
	}{
		{
			name: "embedded NUL",
			snapshot: Snapshot{Templates: []Template{{
				Vnum:              71124,
				Name:              "Broken Storage Charm",
				Stackable:         false,
				MaxCount:          1,
				AntiSafebox:       true,
				SafeboxRejectText: "storage\x00blocked",
			}}},
			rawJSON: `{"templates":[{"vnum":71124,"name":"Broken Storage Charm","stackable":false,"max_count":1,"anti_safebox":true,"safebox_reject_message":"storage\u0000blocked"}]}`,
		},
		{
			name: "missing anti-safebox guard",
			snapshot: Snapshot{Templates: []Template{{
				Vnum:              71125,
				Name:              "Unguarded Storage Charm",
				Stackable:         false,
				MaxCount:          1,
				SafeboxRejectText: "This item has no safebox guard.",
			}}},
			rawJSON: `{"templates":[{"vnum":71125,"name":"Unguarded Storage Charm","stackable":false,"max_count":1,"safebox_reject_message":"This item has no safebox guard."}]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state", "item-templates.json")
			store := NewFileStore(path)
			if err := store.Save(tc.snapshot); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot when saving invalid safebox_reject_message metadata, got %v", err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("mkdir state dir: %v", err)
			}
			if err := os.WriteFile(path, []byte(tc.rawJSON), 0o644); err != nil {
				t.Fatalf("write invalid safebox rejection snapshot: %v", err)
			}
			if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot when loading invalid safebox_reject_message metadata, got %v", err)
			}
		})
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesMyShopRejectText(t *testing.T) {
	cases := []struct {
		name     string
		template Template
		wantJSON string
	}{
		{
			name: "anti_myshop",
			template: Template{
				Vnum:             27061,
				Name:             "Cash MyShop Potion",
				Stackable:        true,
				MaxCount:         200,
				AntiMyShop:       true,
				MyShopRejectText: "This cash item cannot be listed in a private shop.",
			},
			wantJSON: "{\n  \"templates\": [\n    {\n      \"vnum\": 27061,\n      \"name\": \"Cash MyShop Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"anti_myshop\": true,\n      \"myshop_reject_message\": \"This cash item cannot be listed in a private shop.\"\n    }\n  ]\n}\n",
		},
		{
			name: "anti_give",
			template: Template{
				Vnum:             27062,
				Name:             "Bound MyShop Potion",
				Stackable:        true,
				MaxCount:         200,
				AntiGive:         true,
				MyShopRejectText: "You cannot list this bound item in a private shop.",
			},
			wantJSON: "{\n  \"templates\": [\n    {\n      \"vnum\": 27062,\n      \"name\": \"Bound MyShop Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"anti_give\": true,\n      \"myshop_reject_message\": \"You cannot list this bound item in a private shop.\"\n    }\n  ]\n}\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state", "item-templates.json")
			store := NewFileStore(path)
			want := Snapshot{Templates: []Template{tc.template}}
			if err := store.Save(want); err != nil {
				t.Fatalf("save snapshot with myshop_reject_message: %v", err)
			}
			got, err := store.Load()
			if err != nil {
				t.Fatalf("load snapshot with myshop_reject_message: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("unexpected snapshot with myshop_reject_message:\n got: %#v\nwant: %#v", got, want)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read persisted snapshot with myshop_reject_message: %v", err)
			}
			if string(raw) != tc.wantJSON {
				t.Fatalf("unexpected deterministic snapshot with myshop_reject_message:\n got: %s\nwant: %s", string(raw), tc.wantJSON)
			}
		})
	}
}

func TestFileStoreRejectsInvalidMyShopRejectTextMetadata(t *testing.T) {
	cases := []struct {
		name     string
		snapshot Snapshot
		rawJSON  string
	}{
		{
			name: "embedded NUL",
			snapshot: Snapshot{Templates: []Template{{
				Vnum:             27061,
				Name:             "Broken MyShop Message Potion",
				Stackable:        true,
				MaxCount:         200,
				AntiMyShop:       true,
				MyShopRejectText: "myshop\x00blocked",
			}}},
			rawJSON: `{"templates":[{"vnum":27061,"name":"Broken MyShop Message Potion","stackable":true,"max_count":200,"anti_myshop":true,"myshop_reject_message":"myshop\u0000blocked"}]}`,
		},
		{
			name: "missing anti_myshop|anti_give guard",
			snapshot: Snapshot{Templates: []Template{{
				Vnum:             27063,
				Name:             "Unguarded MyShop Message Potion",
				Stackable:        true,
				MaxCount:         200,
				MyShopRejectText: "This item has no owned myshop rejection guard.",
			}}},
			rawJSON: `{"templates":[{"vnum":27063,"name":"Unguarded MyShop Message Potion","stackable":true,"max_count":200,"myshop_reject_message":"This item has no owned myshop rejection guard."}]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state", "item-templates.json")
			store := NewFileStore(path)
			if err := store.Save(tc.snapshot); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot when saving invalid myshop_reject_message metadata, got %v", err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("mkdir state dir: %v", err)
			}
			if err := os.WriteFile(path, []byte(tc.rawJSON), 0o644); err != nil {
				t.Fatalf("write invalid myshop rejection snapshot: %v", err)
			}
			if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot when loading invalid myshop_reject_message metadata, got %v", err)
			}
		})
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
	stackableEquipment := Snapshot{Templates: []Template{{
		Vnum:      11201,
		Name:      "Stackable Weapon Token",
		Stackable: true,
		MaxCount:  200,
		EquipSlot: inventory.EquipmentSlotWeapon.String(),
	}}}
	if err := store.Save(stackableEquipment); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for stackable equipment template, got %v", err)
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

	nulMessage := Snapshot{Templates: []Template{{
		Vnum:      27002,
		Name:      "Practice Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &UseEffect{PointType: 7, PointIndex: 1, PointDelta: 25, Message: "consume\x00hidden"},
	}}}
	if err := store.Save(nulMessage); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for NUL use-effect message, got %v", err)
	}

	nulInfoMessage := Snapshot{Templates: []Template{{
		Vnum:      27002,
		Name:      "Practice Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &UseEffect{PointType: 7, PointIndex: 1, PointDelta: 25, Message: "consume:27002:+25", InfoMessage: "visible\x00hidden"},
	}}}
	if err := store.Save(nulInfoMessage); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for NUL use-effect info_message, got %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"templates":[{"vnum":27002,"name":"Practice Elixir","stackable":true,"max_count":200,"use_effect":{"point_type":7,"point_index":1,"point_delta":25,"message":"consume\u0000hidden"}}]}`), 0o644); err != nil {
		t.Fatalf("write NUL use-effect message snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for loaded NUL use-effect message, got %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"templates":[{"vnum":27002,"name":"Practice Elixir","stackable":true,"max_count":200,"use_effect":{"point_type":7,"point_index":1,"point_delta":25,"message":"consume:27002:+25","info_message":"visible\u0000hidden"}}]}`), 0o644); err != nil {
		t.Fatalf("write NUL use-effect info_message snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for loaded NUL use-effect info_message, got %v", err)
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

func TestFileStoreSaveThenLoadRoundTripPreservesUseEffectInfoMessage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:      27008,
		Name:      "Template Message Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &UseEffect{
			PointType:         7,
			PointIndex:        1,
			PointDelta:        25,
			Message:           "effect:27008",
			InfoMessage:       "You feel steadier.",
			SpecialEffectType: 1,
		},
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with use-effect info message: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with use-effect info message: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with use-effect info message:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with use-effect info message: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27008,\n      \"name\": \"Template Message Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"use_effect\": {\n        \"point_type\": 7,\n        \"point_index\": 1,\n        \"point_delta\": 25,\n        \"message\": \"effect:27008\",\n        \"info_message\": \"You feel steadier.\",\n        \"special_effect_type\": 1\n      }\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with use-effect info message:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveRejectsInvalidUseEffectSpecialEffectType(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	snapshot := Snapshot{Templates: []Template{{
		Vnum:      27008,
		Name:      "Invalid Effect Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &UseEffect{PointType: 7, PointIndex: 1, PointDelta: 25, Message: "effect:27008", SpecialEffectType: MaxSpecialEffectType + 1},
	}}}

	if err := store.Save(snapshot); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for out-of-range use-effect special effect type, got %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create item template test dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"templates":[{"vnum":27008,"name":"Invalid Effect Potion","stackable":true,"max_count":200,"use_effect":{"point_type":7,"point_index":1,"point_delta":25,"message":"effect:27008","special_effect_type":26}}]}`), 0o644); err != nil {
		t.Fatalf("write invalid special-effect item template snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot when loading out-of-range use-effect special effect type, got %v", err)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesDropRejectText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:           27009,
		Name:           "Sealed Drop Potion",
		Stackable:      true,
		MaxCount:       200,
		AntiDrop:       true,
		DropRejectText: "The seal prevents dropping this item.",
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with drop reject message: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with drop reject message: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with drop reject message:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with drop reject message: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27009,\n      \"name\": \"Sealed Drop Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"anti_drop\": true,\n      \"drop_reject_message\": \"The seal prevents dropping this item.\"\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with drop reject message:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesDropRejectTextForSelectedCharacterGuard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:           27019,
		Name:           "Veteran Drop Potion",
		Stackable:      true,
		MaxCount:       200,
		MinLevel:       10,
		DropRejectText: "You are not experienced enough to drop this item.",
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with selected-character drop reject message: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with selected-character drop reject message: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with selected-character drop reject message:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with selected-character drop reject message: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27019,\n      \"name\": \"Veteran Drop Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"min_level\": 10,\n      \"drop_reject_message\": \"You are not experienced enough to drop this item.\"\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with selected-character drop reject message:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreRejectsInvalidDropRejectTextMetadata(t *testing.T) {
	cases := []struct {
		name     string
		invalid  Template
		rawJSON  string
		wantText string
	}{
		{
			name: "nul_message",
			invalid: Template{
				Vnum:           27009,
				Name:           "Broken Drop Message Potion",
				Stackable:      true,
				MaxCount:       200,
				AntiDrop:       true,
				DropRejectText: "bad\x00message",
			},
			rawJSON:  `{"templates":[{"vnum":27009,"name":"Broken Drop Message Potion","stackable":true,"max_count":200,"anti_drop":true,"drop_reject_message":"bad\u0000message"}]}`,
			wantText: "NUL drop reject message",
		},
		{
			name: "without_owned_drop_guard",
			invalid: Template{
				Vnum:           27020,
				Name:           "Unguarded Drop Message Potion",
				Stackable:      true,
				MaxCount:       200,
				DropRejectText: "This item has no owned drop rejection guard.",
			},
			rawJSON:  `{"templates":[{"vnum":27020,"name":"Unguarded Drop Message Potion","stackable":true,"max_count":200,"drop_reject_message":"This item has no owned drop rejection guard."}]}`,
			wantText: "drop reject message without drop guard",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state", "item-templates.json")
			store := NewFileStore(path)
			if err := store.Save(Snapshot{Templates: []Template{tc.invalid}}); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot for %s, got %v", tc.wantText, err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("create item template test dir: %v", err)
			}
			if err := os.WriteFile(path, []byte(tc.rawJSON), 0o644); err != nil {
				t.Fatalf("write invalid drop reject message snapshot: %v", err)
			}
			if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot when loading %s, got %v", tc.wantText, err)
			}
		})
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesPickupRejectText(t *testing.T) {
	cases := []struct {
		name     string
		want     Snapshot
		wantJSON string
	}{
		{
			name: "anti_get_guard",
			want: Snapshot{Templates: []Template{{
				Vnum:             27010,
				Name:             "Sealed Pickup Potion",
				Stackable:        true,
				MaxCount:         200,
				AntiGet:          true,
				PickupRejectText: "The seal prevents picking up this item.",
			}}},
			wantJSON: "{\n  \"templates\": [\n    {\n      \"vnum\": 27010,\n      \"name\": \"Sealed Pickup Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"anti_get\": true,\n      \"pickup_reject_message\": \"The seal prevents picking up this item.\"\n    }\n  ]\n}\n",
		},
		{
			name: "anti_give_currency_guard",
			want: Snapshot{Templates: []Template{{
				Vnum:             1,
				Name:             "Bound Gold Marker",
				Stackable:        true,
				MaxCount:         1,
				AntiGive:         true,
				PickupRejectText: "This gold cannot be collected by party members.",
			}}},
			wantJSON: "{\n  \"templates\": [\n    {\n      \"vnum\": 1,\n      \"name\": \"Bound Gold Marker\",\n      \"stackable\": true,\n      \"max_count\": 1,\n      \"anti_give\": true,\n      \"pickup_reject_message\": \"This gold cannot be collected by party members.\"\n    }\n  ]\n}\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state", "item-templates.json")
			store := NewFileStore(path)

			if err := store.Save(tc.want); err != nil {
				t.Fatalf("save snapshot with pickup reject message: %v", err)
			}
			got, err := store.Load()
			if err != nil {
				t.Fatalf("load snapshot with pickup reject message: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("unexpected snapshot with pickup reject message:\n got: %#v\nwant: %#v", got, tc.want)
			}

			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read persisted snapshot with pickup reject message: %v", err)
			}
			if string(raw) != tc.wantJSON {
				t.Fatalf("unexpected deterministic snapshot with pickup reject message:\n got: %s\nwant: %s", string(raw), tc.wantJSON)
			}
		})
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesPickupRange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:        27021,
		Name:        "Long Reach Pickup Potion",
		Stackable:   true,
		MaxCount:    200,
		PickupRange: 500,
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with pickup range: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with pickup range: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with pickup range:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with pickup range: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27021,\n      \"name\": \"Long Reach Pickup Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"pickup_range\": 500\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with pickup range:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreRejectsPickupRangeAboveBootstrapLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	invalid := Snapshot{Templates: []Template{{
		Vnum:        27021,
		Name:        "Too Far Pickup Potion",
		Stackable:   true,
		MaxCount:    200,
		PickupRange: MaxPickupRange + 1,
	}}}
	if err := store.Save(invalid); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for pickup_range above bootstrap limit, got %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create item template test dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"templates":[{"vnum":27021,"name":"Too Far Pickup Potion","stackable":true,"max_count":200,"pickup_range":10001}]}`), 0o644); err != nil {
		t.Fatalf("write oversized pickup_range snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot when loading pickup_range above bootstrap limit, got %v", err)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesBuyRejectText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:          27015,
		Name:          "Sealed Buy Potion",
		Stackable:     true,
		MaxCount:      200,
		AntiGet:       true,
		BuyRejectText: "The merchant will not sell this item to you.",
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with buy reject message: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with buy reject message: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with buy reject message:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with buy reject message: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27015,\n      \"name\": \"Sealed Buy Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"anti_get\": true,\n      \"buy_reject_message\": \"The merchant will not sell this item to you.\"\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with buy reject message:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesBuyRejectTextForSelectedCharacterGuard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:          27017,
		Name:          "Class Locked Buy Potion",
		Stackable:     true,
		MaxCount:      200,
		AntiWarrior:   true,
		BuyRejectText: "This merchant will not sell this potion to your class.",
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with selected-character buy reject message: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with selected-character buy reject message: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with selected-character buy reject message:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with selected-character buy reject message: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27017,\n      \"name\": \"Class Locked Buy Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"anti_warrior\": true,\n      \"buy_reject_message\": \"This merchant will not sell this potion to your class.\"\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with selected-character buy reject message:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesSellRejectText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:           27011,
		Name:           "Sealed Sell Potion",
		Stackable:      true,
		MaxCount:       200,
		AntiSell:       true,
		SellRejectText: "The merchant refuses to buy this item.",
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with sell reject message: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with sell reject message: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with sell reject message:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with sell reject message: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27011,\n      \"name\": \"Sealed Sell Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"anti_sell\": true,\n      \"sell_reject_message\": \"The merchant refuses to buy this item.\"\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with sell reject message:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesSellRejectTextForSelectedCharacterGuard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:           27019,
		Name:           "Class Locked Sell Potion",
		Stackable:      true,
		MaxCount:       200,
		AntiWarrior:    true,
		SellRejectText: "This merchant will not buy this potion from your class.",
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with selected-character sell reject message: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with selected-character sell reject message: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with selected-character sell reject message:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with selected-character sell reject message: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27019,\n      \"name\": \"Class Locked Sell Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"anti_warrior\": true,\n      \"sell_reject_message\": \"This merchant will not buy this potion from your class.\"\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with selected-character sell reject message:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesUseRejectText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:          27012,
		Name:          "Quest Use Potion",
		Stackable:     true,
		MaxCount:      200,
		QuestUse:      true,
		UseEffect:     &UseEffect{PointType: 1, PointIndex: 1, PointDelta: 50, Message: "quest:27012:+50"},
		UseRejectText: "You cannot use this quest item yet.",
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with use reject message: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with use reject message: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with use reject message:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with use reject message: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27012,\n      \"name\": \"Quest Use Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"quest_use\": true,\n      \"use_effect\": {\n        \"point_type\": 1,\n        \"point_index\": 1,\n        \"point_delta\": 50,\n        \"message\": \"quest:27012:+50\"\n      },\n      \"use_reject_message\": \"You cannot use this quest item yet.\"\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with use reject message:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesGiveRejectText(t *testing.T) {
	cases := []struct {
		name     string
		template Template
		wantJSON string
	}{
		{
			name: "anti_give",
			template: Template{
				Vnum:           27042,
				Name:           "Bound Gift Potion",
				Stackable:      true,
				MaxCount:       200,
				AntiGive:       true,
				GiveRejectText: "You cannot give this item.",
			},
			wantJSON: "{\n  \"templates\": [\n    {\n      \"vnum\": 27042,\n      \"name\": \"Bound Gift Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"anti_give\": true,\n      \"give_reject_message\": \"You cannot give this item.\"\n    }\n  ]\n}\n",
		},
		{
			name: "anti_drop exchange display guard",
			template: Template{
				Vnum:           27044,
				Name:           "Undroppable Trade Potion",
				Stackable:      true,
				MaxCount:       200,
				AntiDrop:       true,
				GiveRejectText: "You cannot trade this item.",
			},
			wantJSON: "{\n  \"templates\": [\n    {\n      \"vnum\": 27044,\n      \"name\": \"Undroppable Trade Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"anti_drop\": true,\n      \"give_reject_message\": \"You cannot trade this item.\"\n    }\n  ]\n}\n",
		},
		{
			name: "min_level exchange display guard",
			template: Template{
				Vnum:           27045,
				Name:           "Level Locked Trade Potion",
				Stackable:      true,
				MaxCount:       200,
				MinLevel:       10,
				GiveRejectText: "You cannot trade this item yet.",
			},
			wantJSON: "{\n  \"templates\": [\n    {\n      \"vnum\": 27045,\n      \"name\": \"Level Locked Trade Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"give_reject_message\": \"You cannot trade this item yet.\",\n      \"min_level\": 10\n    }\n  ]\n}\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state", "item-templates.json")
			store := NewFileStore(path)
			want := Snapshot{Templates: []Template{tc.template}}

			if err := store.Save(want); err != nil {
				t.Fatalf("save snapshot with give reject message: %v", err)
			}
			got, err := store.Load()
			if err != nil {
				t.Fatalf("load snapshot with give reject message: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("unexpected snapshot with give reject message:\n got: %#v\nwant: %#v", got, want)
			}

			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read persisted snapshot with give reject message: %v", err)
			}
			if string(raw) != tc.wantJSON {
				t.Fatalf("unexpected deterministic snapshot with give reject message:\n got: %s\nwant: %s", string(raw), tc.wantJSON)
			}
		})
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesUnequipRejectText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:              11200,
		Name:              "Sealed Armor",
		Stackable:         false,
		MaxCount:          1,
		EquipSlot:         inventory.EquipmentSlotBody.String(),
		Irremovable:       true,
		UnequipRejectText: "The seal prevents removing this item.",
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with unequip reject message: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with unequip reject message: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with unequip reject message:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with unequip reject message: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 11200,\n      \"name\": \"Sealed Armor\",\n      \"stackable\": false,\n      \"max_count\": 1,\n      \"irremovable\": true,\n      \"equip_slot\": \"body\",\n      \"unequip_reject_message\": \"The seal prevents removing this item.\"\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with unequip reject message:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesEquipRejectText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:            11500,
		Name:            "Class Locked Armor",
		Stackable:       false,
		MaxCount:        1,
		AntiWarrior:     true,
		EquipSlot:       inventory.EquipmentSlotBody.String(),
		EquipRejectText: "This armor rejects your class.",
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with equip reject message: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with equip reject message: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with equip reject message:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with equip reject message: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 11500,\n      \"name\": \"Class Locked Armor\",\n      \"stackable\": false,\n      \"max_count\": 1,\n      \"anti_warrior\": true,\n      \"equip_slot\": \"body\",\n      \"equip_reject_message\": \"This armor rejects your class.\"\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with equip reject message:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesSellRejectTextForTransferGuard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:           27031,
		Name:           "No Stack Sell Potion",
		Stackable:      true,
		MaxCount:       200,
		AntiStack:      true,
		SellRejectText: "This merchant refuses bundled items.",
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with anti-stack sell reject message: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with anti-stack sell reject message: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with anti-stack sell reject message:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with anti-stack sell reject message: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27031,\n      \"name\": \"No Stack Sell Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"anti_stack\": true,\n      \"sell_reject_message\": \"This merchant refuses bundled items.\"\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with anti-stack sell reject message:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreRejectsInvalidSellRejectTextMetadata(t *testing.T) {
	cases := []struct {
		name     string
		invalid  Template
		rawJSON  string
		wantText string
	}{
		{
			name: "nul message",
			invalid: Template{
				Vnum:           27011,
				Name:           "Broken Sell Message Potion",
				Stackable:      true,
				MaxCount:       200,
				AntiSell:       true,
				SellRejectText: "bad\x00message",
			},
			rawJSON:  `{"templates":[{"vnum":27011,"name":"Broken Sell Message Potion","stackable":true,"max_count":200,"anti_sell":true,"sell_reject_message":"bad\u0000message"}]}`,
			wantText: "NUL sell reject message",
		},
		{
			name: "without merchant-sell guard",
			invalid: Template{
				Vnum:           27018,
				Name:           "Unguarded Sell Message Potion",
				Stackable:      true,
				MaxCount:       200,
				SellRejectText: "This item has no owned sell rejection guard.",
			},
			rawJSON:  `{"templates":[{"vnum":27018,"name":"Unguarded Sell Message Potion","stackable":true,"max_count":200,"sell_reject_message":"This item has no owned sell rejection guard."}]}`,
			wantText: "sell reject message without merchant-sell guard",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state", "item-templates.json")
			store := NewFileStore(path)
			if err := store.Save(Snapshot{Templates: []Template{tc.invalid}}); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot for %s, got %v", tc.wantText, err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("create item template test dir: %v", err)
			}
			if err := os.WriteFile(path, []byte(tc.rawJSON), 0o644); err != nil {
				t.Fatalf("write invalid sell reject message snapshot: %v", err)
			}
			if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot when loading %s, got %v", tc.wantText, err)
			}
		})
	}
}

func TestFileStoreRejectsInvalidBuyRejectTextMetadata(t *testing.T) {
	cases := []struct {
		name     string
		invalid  Template
		rawJSON  string
		wantText string
	}{
		{
			name: "nul message",
			invalid: Template{
				Vnum:          27015,
				Name:          "Broken Buy Message Potion",
				Stackable:     true,
				MaxCount:      200,
				AntiGet:       true,
				BuyRejectText: "bad\x00message",
			},
			rawJSON:  `{"templates":[{"vnum":27015,"name":"Broken Buy Message Potion","stackable":true,"max_count":200,"anti_get":true,"buy_reject_message":"bad\u0000message"}]}`,
			wantText: "NUL buy reject message",
		},
		{
			name: "without merchant-buy guard",
			invalid: Template{
				Vnum:          27016,
				Name:          "Unguarded Buy Message Potion",
				Stackable:     true,
				MaxCount:      200,
				BuyRejectText: "This item has no owned buy rejection guard.",
			},
			rawJSON:  `{"templates":[{"vnum":27016,"name":"Unguarded Buy Message Potion","stackable":true,"max_count":200,"buy_reject_message":"This item has no owned buy rejection guard."}]}`,
			wantText: "buy reject message without merchant-buy guard",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state", "item-templates.json")
			store := NewFileStore(path)
			if err := store.Save(Snapshot{Templates: []Template{tc.invalid}}); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot for %s, got %v", tc.wantText, err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("create item template test dir: %v", err)
			}
			if err := os.WriteFile(path, []byte(tc.rawJSON), 0o644); err != nil {
				t.Fatalf("write invalid buy reject message snapshot: %v", err)
			}
			if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot when loading %s, got %v", tc.wantText, err)
			}
		})
	}
}

func TestFileStoreRejectsInvalidUseRejectTextMetadata(t *testing.T) {
	cases := []struct {
		name     string
		invalid  Template
		rawJSON  string
		wantText string
	}{
		{
			name: "nul message",
			invalid: Template{
				Vnum:          27012,
				Name:          "Broken Use Message Potion",
				Stackable:     true,
				MaxCount:      200,
				QuestUse:      true,
				UseEffect:     &UseEffect{PointType: 1, PointIndex: 1, PointDelta: 50, Message: "quest:27012:+50"},
				UseRejectText: "bad\x00message",
			},
			rawJSON:  `{"templates":[{"vnum":27012,"name":"Broken Use Message Potion","stackable":true,"max_count":200,"quest_use":true,"use_effect":{"point_type":1,"point_index":1,"point_delta":50,"message":"quest:27012:+50"},"use_reject_message":"bad\u0000message"}]}`,
			wantText: "NUL use reject message",
		},
		{
			name: "without use effect",
			invalid: Template{
				Vnum:          27013,
				Name:          "Message Without Effect Potion",
				Stackable:     true,
				MaxCount:      200,
				QuestUse:      true,
				UseRejectText: "This item cannot be used yet.",
			},
			rawJSON:  `{"templates":[{"vnum":27013,"name":"Message Without Effect Potion","stackable":true,"max_count":200,"quest_use":true,"use_reject_message":"This item cannot be used yet."}]}`,
			wantText: "use reject message without use effect",
		},
		{
			name: "confirm_when_use is not a direct-use reject guard",
			invalid: Template{
				Vnum:           27014,
				Name:           "Confirm Only Use Message Potion",
				Stackable:      true,
				MaxCount:       200,
				ConfirmWhenUse: true,
				UseEffect:      &UseEffect{PointType: 1, PointIndex: 1, PointDelta: 50, Message: "confirm:27014:+50"},
				UseRejectText:  "This item has no owned use rejection guard.",
			},
			rawJSON:  `{"templates":[{"vnum":27014,"name":"Confirm Only Use Message Potion","stackable":true,"max_count":200,"confirm_when_use":true,"use_effect":{"point_type":1,"point_index":1,"point_delta":50,"message":"confirm:27014:+50"},"use_reject_message":"This item has no owned use rejection guard."}]}`,
			wantText: "use reject message without direct-use guard",
		},
		{
			name: "without direct-use guard",
			invalid: Template{
				Vnum:          27015,
				Name:          "Unguarded Use Message Potion",
				Stackable:     true,
				MaxCount:      200,
				UseEffect:     &UseEffect{PointType: 1, PointIndex: 1, PointDelta: 50, Message: "consume:27015:+50"},
				UseRejectText: "This item has no owned use rejection guard.",
			},
			rawJSON:  `{"templates":[{"vnum":27015,"name":"Unguarded Use Message Potion","stackable":true,"max_count":200,"use_effect":{"point_type":1,"point_index":1,"point_delta":50,"message":"consume:27015:+50"},"use_reject_message":"This item has no owned use rejection guard."}]}`,
			wantText: "use reject message without direct-use guard",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state", "item-templates.json")
			store := NewFileStore(path)
			if err := store.Save(Snapshot{Templates: []Template{tc.invalid}}); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot for %s, got %v", tc.wantText, err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("create item template test dir: %v", err)
			}
			if err := os.WriteFile(path, []byte(tc.rawJSON), 0o644); err != nil {
				t.Fatalf("write invalid use reject message snapshot: %v", err)
			}
			if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot when loading %s, got %v", tc.wantText, err)
			}
		})
	}
}

func TestFileStoreRejectsInvalidGiveRejectTextMetadata(t *testing.T) {
	cases := []struct {
		name     string
		invalid  Template
		rawJSON  string
		wantText string
	}{
		{
			name: "nul message",
			invalid: Template{
				Vnum:           27042,
				Name:           "Broken Give Message Potion",
				Stackable:      true,
				MaxCount:       200,
				AntiGive:       true,
				GiveRejectText: "bad\x00message",
			},
			rawJSON:  `{"templates":[{"vnum":27042,"name":"Broken Give Message Potion","stackable":true,"max_count":200,"anti_give":true,"give_reject_message":"bad\u0000message"}]}`,
			wantText: "NUL give reject message",
		},
		{
			name: "without owned exchange/give reject guard",
			invalid: Template{
				Vnum:           27043,
				Name:           "Unguarded Give Message Potion",
				Stackable:      true,
				MaxCount:       200,
				GiveRejectText: "This item has no owned give rejection guard.",
			},
			rawJSON:  `{"templates":[{"vnum":27043,"name":"Unguarded Give Message Potion","stackable":true,"max_count":200,"give_reject_message":"This item has no owned give rejection guard."}]}`,
			wantText: "give reject message without owned exchange/give reject guard",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state", "item-templates.json")
			store := NewFileStore(path)
			if err := store.Save(Snapshot{Templates: []Template{tc.invalid}}); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot for %s, got %v", tc.wantText, err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("create item template test dir: %v", err)
			}
			if err := os.WriteFile(path, []byte(tc.rawJSON), 0o644); err != nil {
				t.Fatalf("write invalid give reject message snapshot: %v", err)
			}
			if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot when loading %s, got %v", tc.wantText, err)
			}
		})
	}
}

func TestFileStoreRejectsInvalidUnequipRejectTextMetadata(t *testing.T) {
	cases := []struct {
		name     string
		invalid  Template
		rawJSON  string
		wantText string
	}{
		{
			name: "nul message",
			invalid: Template{
				Vnum:              11200,
				Name:              "Broken Unequip Message Armor",
				Stackable:         false,
				MaxCount:          1,
				EquipSlot:         inventory.EquipmentSlotBody.String(),
				Irremovable:       true,
				UnequipRejectText: "bad\x00message",
			},
			rawJSON:  `{"templates":[{"vnum":11200,"name":"Broken Unequip Message Armor","stackable":false,"max_count":1,"equip_slot":"body","irremovable":true,"unequip_reject_message":"bad\u0000message"}]}`,
			wantText: "NUL unequip reject message",
		},
		{
			name: "without irremovable",
			invalid: Template{
				Vnum:              11200,
				Name:              "Movable Message Armor",
				Stackable:         false,
				MaxCount:          1,
				EquipSlot:         inventory.EquipmentSlotBody.String(),
				UnequipRejectText: "This movable item should not author removal text.",
			},
			rawJSON:  `{"templates":[{"vnum":11200,"name":"Movable Message Armor","stackable":false,"max_count":1,"equip_slot":"body","unequip_reject_message":"This movable item should not author removal text."}]}`,
			wantText: "unequip reject message without irremovable",
		},
		{
			name: "without equip slot",
			invalid: Template{
				Vnum:              27001,
				Name:              "Consumable Removal Message",
				Stackable:         true,
				MaxCount:          200,
				Irremovable:       true,
				UnequipRejectText: "This carried item should not author removal text.",
			},
			rawJSON:  `{"templates":[{"vnum":27001,"name":"Consumable Removal Message","stackable":true,"max_count":200,"irremovable":true,"unequip_reject_message":"This carried item should not author removal text."}]}`,
			wantText: "unequip reject message without equip slot",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state", "item-templates.json")
			store := NewFileStore(path)
			if err := store.Save(Snapshot{Templates: []Template{tc.invalid}}); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot for %s, got %v", tc.wantText, err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("create item template test dir: %v", err)
			}
			if err := os.WriteFile(path, []byte(tc.rawJSON), 0o644); err != nil {
				t.Fatalf("write invalid unequip reject message snapshot: %v", err)
			}
			if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot when loading %s, got %v", tc.wantText, err)
			}
		})
	}
}

func TestFileStoreRejectsInvalidEquipRejectTextMetadata(t *testing.T) {
	cases := []struct {
		name     string
		invalid  Template
		rawJSON  string
		wantText string
	}{
		{
			name: "nul message",
			invalid: Template{
				Vnum:            11500,
				Name:            "Broken Equip Message Armor",
				Stackable:       false,
				MaxCount:        1,
				AntiWarrior:     true,
				EquipSlot:       inventory.EquipmentSlotBody.String(),
				EquipRejectText: "bad\x00message",
			},
			rawJSON:  `{"templates":[{"vnum":11500,"name":"Broken Equip Message Armor","stackable":false,"max_count":1,"anti_warrior":true,"equip_slot":"body","equip_reject_message":"bad\u0000message"}]}`,
			wantText: "NUL equip reject message",
		},
		{
			name: "without equip slot",
			invalid: Template{
				Vnum:            27001,
				Name:            "Consumable Equip Message",
				Stackable:       true,
				MaxCount:        200,
				AntiWarrior:     true,
				EquipRejectText: "Consumables should not author equip text.",
			},
			rawJSON:  `{"templates":[{"vnum":27001,"name":"Consumable Equip Message","stackable":true,"max_count":200,"anti_warrior":true,"equip_reject_message":"Consumables should not author equip text."}]}`,
			wantText: "equip reject message without equip slot",
		},
		{
			name: "without equip rejection guard",
			invalid: Template{
				Vnum:            11500,
				Name:            "Unguarded Equip Message Armor",
				Stackable:       false,
				MaxCount:        1,
				EquipSlot:       inventory.EquipmentSlotBody.String(),
				EquipRejectText: "This item has no owned equip rejection guard.",
			},
			rawJSON:  `{"templates":[{"vnum":11500,"name":"Unguarded Equip Message Armor","stackable":false,"max_count":1,"equip_slot":"body","equip_reject_message":"This item has no owned equip rejection guard."}]}`,
			wantText: "equip reject message without equip rejection guard",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state", "item-templates.json")
			store := NewFileStore(path)
			if err := store.Save(Snapshot{Templates: []Template{tc.invalid}}); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot for %s, got %v", tc.wantText, err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("create item template test dir: %v", err)
			}
			if err := os.WriteFile(path, []byte(tc.rawJSON), 0o644); err != nil {
				t.Fatalf("write invalid equip reject message snapshot: %v", err)
			}
			if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot when loading %s, got %v", tc.wantText, err)
			}
		})
	}
}

func TestFileStoreRejectsInvalidPickupRejectTextMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	invalid := Snapshot{Templates: []Template{{
		Vnum:             27010,
		Name:             "Broken Pickup Message Potion",
		Stackable:        true,
		MaxCount:         200,
		AntiGet:          true,
		PickupRejectText: "bad\x00message",
	}}}

	if err := store.Save(invalid); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for NUL pickup reject message, got %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create item template test dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"templates":[{"vnum":27010,"name":"Broken Pickup Message Potion","stackable":true,"max_count":200,"anti_get":true,"pickup_reject_message":"bad\u0000message"}]}`), 0o644); err != nil {
		t.Fatalf("write invalid pickup reject message snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot when loading NUL pickup reject message, got %v", err)
	}
}

func TestFileStoreRejectsPickupRejectTextWithoutOwnedGuard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	invalid := Snapshot{Templates: []Template{{
		Vnum:             27011,
		Name:             "Unguarded Pickup Message Potion",
		Stackable:        true,
		MaxCount:         200,
		PickupRejectText: "This item has no owned pickup rejection guard.",
	}}}

	if err := store.Save(invalid); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for pickup reject message without pickup guard, got %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create item template test dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"templates":[{"vnum":27011,"name":"Unguarded Pickup Message Potion","stackable":true,"max_count":200,"pickup_reject_message":"This item has no owned pickup rejection guard."}]}`), 0o644); err != nil {
		t.Fatalf("write invalid pickup reject message snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot when loading pickup reject message without pickup guard, got %v", err)
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

func TestFileStoreSaveThenLoadRoundTripPreservesUseEffectConsumeCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:      27007,
		Name:      "Triple Dose Practice Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &UseEffect{
			PointType:    1,
			PointIndex:   1,
			PointDelta:   75,
			ConsumeCount: 3,
			Message:      "consume:27007:x3",
		},
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with use-effect consume-count metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with use-effect consume-count metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with use-effect consume-count metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with use-effect consume-count metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27007,\n      \"name\": \"Triple Dose Practice Elixir\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"use_effect\": {\n        \"point_type\": 1,\n        \"point_index\": 1,\n        \"point_delta\": 75,\n        \"consume_count\": 3,\n        \"message\": \"consume:27007:x3\"\n      }\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with use-effect consume-count metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveRejectsInvalidUseEffectConsumeCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)

	overMax := Snapshot{Templates: []Template{{
		Vnum:      27007,
		Name:      "Overdrawn Practice Elixir",
		Stackable: true,
		MaxCount:  2,
		UseEffect: &UseEffect{
			PointType:    1,
			PointIndex:   1,
			PointDelta:   75,
			ConsumeCount: 3,
			Message:      "must not load",
		},
	}}}
	if err := store.Save(overMax); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for use-effect consume-count above max_count, got %v", err)
	}

	nonStackableMultiConsume := Snapshot{Templates: []Template{{
		Vnum:      27008,
		Name:      "Overdrawn Practice Charm",
		Stackable: false,
		MaxCount:  1,
		UseEffect: &UseEffect{
			PointType:    1,
			PointIndex:   1,
			PointDelta:   75,
			ConsumeCount: 2,
			Message:      "must not load",
		},
	}}}
	if err := store.Save(nonStackableMultiConsume); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for non-stackable multi-count use effect, got %v", err)
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

func directoryEntryNames(entries []os.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
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
