package accountstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestFileStoreRejectsZeroCountInventoryItem(t *testing.T) {
	store := NewFileStore(t.TempDir())
	account := Account{
		Login:  "mkmk",
		Empire: 2,
		Characters: []loginticket.Character{{
			ID:        1,
			Name:      "MkmkWar",
			Inventory: []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 0, Slot: 8}},
		}},
	}

	if err := store.Save(account); err == nil {
		t.Fatal("expected zero-count inventory item snapshot to be rejected")
	}
}

func TestFileStoreLoadRejectsZeroCountInventoryItem(t *testing.T) {
	store := NewFileStore(t.TempDir())
	raw := []byte(`{"login":"mkmk","empire":2,"characters":[{"id":1,"name":"MkmkWar","inventory":[{"id":1001,"vnum":27001,"count":0,"slot":8}],"equipment":[],"quickslots":[]}]}`)
	if err := os.WriteFile(store.accountPath("mkmk"), raw, 0o644); err != nil {
		t.Fatalf("write invalid zero-count account snapshot: %v", err)
	}

	_, err := store.Load("mkmk")
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for loaded zero-count inventory item, got %v", err)
	}
}

func TestFileStoreRejectsDuplicateEquipmentSlots(t *testing.T) {
	store := NewFileStore(t.TempDir())
	account := Account{
		Login:  "mkmk",
		Empire: 2,
		Characters: []loginticket.Character{{
			ID:   1,
			Name: "MkmkWar",
			Equipment: []inventory.ItemInstance{
				{ID: 2001, Vnum: 19, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
				{ID: 2002, Vnum: 29, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
			},
		}},
	}

	if err := store.Save(account); !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for duplicate equipment slot, got %v", err)
	}
}

func TestFileStoreLoadRejectsDuplicateEquipmentSlots(t *testing.T) {
	store := NewFileStore(t.TempDir())
	raw := []byte(`{"login":"mkmk","empire":2,"characters":[{"id":1,"name":"MkmkWar","inventory":[],"equipment":[{"id":2001,"vnum":19,"count":1,"equipped":true,"equip_slot":2},{"id":2002,"vnum":29,"count":1,"equipped":true,"equip_slot":2}],"quickslots":[]}]}`)
	if err := os.WriteFile(store.accountPath("mkmk"), raw, 0o644); err != nil {
		t.Fatalf("write duplicate-equipment account snapshot: %v", err)
	}

	_, err := store.Load("mkmk")
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for duplicate equipment slot, got %v", err)
	}
}

func TestFileStoreRejectsDuplicateQuickslotPositions(t *testing.T) {
	store := NewFileStore(t.TempDir())
	account := Account{
		Login:  "mkmk",
		Empire: 2,
		Characters: []loginticket.Character{{
			ID:   1,
			Name: "MkmkWar",
			Quickslots: []loginticket.Quickslot{
				{Position: 3, Type: 1, Slot: 8},
				{Position: 3, Type: 2, Slot: 9},
			},
		}},
	}

	if err := store.Save(account); !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for duplicate quickslot position, got %v", err)
	}
}

func TestFileStoreLoadRejectsDuplicateQuickslotPositions(t *testing.T) {
	store := NewFileStore(t.TempDir())
	raw := []byte(`{"login":"mkmk","empire":2,"characters":[{"id":1,"name":"MkmkWar","inventory":[],"equipment":[],"quickslots":[{"position":3,"type":1,"slot":8},{"position":3,"type":2,"slot":9}]}]}`)
	if err := os.WriteFile(store.accountPath("mkmk"), raw, 0o644); err != nil {
		t.Fatalf("write duplicate-quickslot account snapshot: %v", err)
	}

	_, err := store.Load("mkmk")
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for duplicate quickslot position, got %v", err)
	}
}

func TestFileStoreRejectsInvalidQuickslotTuples(t *testing.T) {
	store := NewFileStore(t.TempDir())
	cases := []struct {
		name      string
		quickslot loginticket.Quickslot
	}{
		{name: "position out of range", quickslot: loginticket.Quickslot{Position: 36, Type: 1, Slot: 8}},
		{name: "unknown type", quickslot: loginticket.Quickslot{Position: 3, Type: 9, Slot: 8}},
		{name: "item slot out of range", quickslot: loginticket.Quickslot{Position: 3, Type: 1, Slot: 180}},
		{name: "skill slot out of range", quickslot: loginticket.Quickslot{Position: 3, Type: 2, Slot: 200}},
		{name: "command slot out of range", quickslot: loginticket.Quickslot{Position: 3, Type: 3, Slot: 60}},
		{name: "none keeps stale slot", quickslot: loginticket.Quickslot{Position: 3, Type: 0, Slot: 8}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			account := Account{Login: "mkmk", Empire: 2, Characters: []loginticket.Character{{ID: 1, Name: "MkmkWar", Quickslots: []loginticket.Quickslot{tc.quickslot}}}}
			if err := store.Save(account); !errors.Is(err, ErrInvalidAccount) {
				t.Fatalf("expected ErrInvalidAccount, got %v", err)
			}
		})
	}
}

func TestFileStoreLoadAllowsDuplicateCarriedSlotsForRuntimeRecovery(t *testing.T) {
	store := NewFileStore(t.TempDir())
	raw := []byte(`{"login":"mkmk","empire":2,"characters":[{"id":1,"name":"MkmkWar","inventory":[{"id":1001,"vnum":27001,"count":5,"slot":8},{"id":1002,"vnum":27002,"count":3,"slot":8}],"equipment":[],"quickslots":[]}]}`)
	if err := os.WriteFile(store.accountPath("mkmk"), raw, 0o644); err != nil {
		t.Fatalf("write duplicate-slot account snapshot: %v", err)
	}

	got, err := store.Load("mkmk")
	if err != nil {
		t.Fatalf("expected duplicate carried-slot snapshot to remain loadable for runtime recovery tests, got %v", err)
	}
	if len(got.Characters) != 1 || len(got.Characters[0].Inventory) != 2 {
		t.Fatalf("unexpected duplicate-slot account load result: %#v", got)
	}
}

func TestFileStoreLoadRejectsUnknownAccountFields(t *testing.T) {
	store := NewFileStore(t.TempDir())
	raw := []byte(`{"login":"mkmk","empire":2,"schema_version":99,"characters":[]}`)
	if err := os.WriteFile(store.accountPath("mkmk"), raw, 0o644); err != nil {
		t.Fatalf("write unknown-field account snapshot: %v", err)
	}

	_, err := store.Load("mkmk")
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for unknown account field, got %v", err)
	}
}

func TestFileStoreLoadRejectsTrailingJSONValue(t *testing.T) {
	store := NewFileStore(t.TempDir())
	raw := []byte(`{"login":"mkmk","empire":2,"characters":[]} {"login":"shadow","empire":1,"characters":[]}`)
	if err := os.WriteFile(store.accountPath("mkmk"), raw, 0o644); err != nil {
		t.Fatalf("write trailing-json account snapshot: %v", err)
	}

	_, err := store.Load("mkmk")
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for trailing JSON value, got %v", err)
	}
}

func TestFileStoreLoadRejectsEmptySnapshotLogin(t *testing.T) {
	store := NewFileStore(t.TempDir())
	raw := []byte(`{"login":"","empire":2,"characters":[]}`)
	if err := os.WriteFile(store.accountPath("mkmk"), raw, 0o644); err != nil {
		t.Fatalf("write empty-login account snapshot: %v", err)
	}

	_, err := store.Load("mkmk")
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for empty snapshot login, got %v", err)
	}
}

func TestFileStoreLoadRejectsMismatchedSnapshotLogin(t *testing.T) {
	store := NewFileStore(t.TempDir())
	raw := []byte(`{"login":"shadow","empire":2,"characters":[]}`)
	if err := os.WriteFile(store.accountPath("mkmk"), raw, 0o644); err != nil {
		t.Fatalf("write mismatched-login account snapshot: %v", err)
	}

	_, err := store.Load("mkmk")
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for mismatched snapshot login, got %v", err)
	}
}

func TestFileStoreRejectsDuplicateCharacterIDs(t *testing.T) {
	store := NewFileStore(t.TempDir())
	account := Account{Login: "mkmk", Empire: 2, Characters: []loginticket.Character{
		{ID: 1, Name: "MkmkWar"},
		{ID: 1, Name: "MkmkNinja"},
	}}

	if err := store.Save(account); !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for duplicate character id, got %v", err)
	}
}

func TestFileStoreRejectsDuplicateCharacterNamesCaseInsensitive(t *testing.T) {
	store := NewFileStore(t.TempDir())
	account := Account{Login: "mkmk", Empire: 2, Characters: []loginticket.Character{
		{ID: 1, Name: "MkmkWar"},
		{ID: 2, Name: "mkmkwar"},
	}}

	if err := store.Save(account); !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for duplicate character name, got %v", err)
	}
}

func TestFileStoreLoadRejectsDuplicateCharacterIdentity(t *testing.T) {
	store := NewFileStore(t.TempDir())
	raw := []byte(`{"login":"mkmk","empire":2,"characters":[{"id":1,"name":"MkmkWar","inventory":[],"equipment":[],"quickslots":[]},{"id":2,"name":"MKMKWAR","inventory":[],"equipment":[],"quickslots":[]}]}`)
	if err := os.WriteFile(store.accountPath("mkmk"), raw, 0o644); err != nil {
		t.Fatalf("write duplicate-character account snapshot: %v", err)
	}

	_, err := store.Load("mkmk")
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for duplicate character identity, got %v", err)
	}
}

func TestFileStoreListReturnsAccountsInDeterministicLoginOrder(t *testing.T) {
	store := NewFileStore(t.TempDir())
	accounts := []Account{
		{Login: "zeta", Empire: 3, Characters: []loginticket.Character{{ID: 3, Name: "ZetaWar"}}},
		{Login: "alpha", Empire: 1, Characters: []loginticket.Character{{ID: 1, Name: "AlphaWar"}}},
		{Login: "Beta", Empire: 2, Characters: []loginticket.Character{{ID: 2, Name: "BetaWar"}}},
	}
	for _, account := range accounts {
		if err := store.Save(account); err != nil {
			t.Fatalf("save %s: %v", account.Login, err)
		}
	}

	got, err := store.List()
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	gotLogins := make([]string, 0, len(got))
	for _, account := range got {
		gotLogins = append(gotLogins, account.Login)
	}
	wantLogins := []string{"alpha", "Beta", "zeta"}
	if !reflect.DeepEqual(gotLogins, wantLogins) {
		t.Fatalf("unexpected account order: got %#v want %#v", gotLogins, wantLogins)
	}
}

func TestFileStoreListTreatsMissingDirectoryAsEmpty(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "missing"))

	got, err := store.List()
	if err != nil {
		t.Fatalf("list missing store: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no accounts for missing store dir, got %#v", got)
	}
}

func TestFileStoreListIgnoresCrashTempFiles(t *testing.T) {
	store := NewFileStore(t.TempDir())
	if err := store.Save(Account{Login: "mkmk", Empire: 2}); err != nil {
		t.Fatalf("save account: %v", err)
	}
	if err := os.WriteFile(filepath.Join(store.dir, ".account-crashed.json"), []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	got, err := store.List()
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(got) != 1 || got[0].Login != "mkmk" {
		t.Fatalf("expected only committed account, got %#v", got)
	}
}

func TestFileStoreListRejectsCorruptCommittedSnapshot(t *testing.T) {
	store := NewFileStore(t.TempDir())
	path := store.accountPath("mkmk")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create store dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"login":"mkmk","empire":2,"characters":[]`), 0o644); err != nil {
		t.Fatalf("write corrupt account: %v", err)
	}

	_, err := store.List()
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for corrupt committed snapshot, got %v", err)
	}
}

func TestFileStoreListRejectsFilenameLoginMismatch(t *testing.T) {
	store := NewFileStore(t.TempDir())
	path := store.accountPath("mkmk")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create store dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"login":"shadow","empire":2,"characters":[]}`), 0o644); err != nil {
		t.Fatalf("write mismatched account: %v", err)
	}

	_, err := store.List()
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for mismatched account filename, got %v", err)
	}
}

func TestFileStoreValidateReportsDeterministicSnapshotSummary(t *testing.T) {
	store := NewFileStore(t.TempDir())
	accounts := []Account{
		{Login: "zeta", Empire: 3, Characters: []loginticket.Character{{ID: 3, Name: "ZetaWar"}, {ID: 4, Name: "ZetaNinja"}}},
		{Login: "alpha", Empire: 1, Characters: []loginticket.Character{{ID: 1, Name: "AlphaWar"}}},
	}
	for _, account := range accounts {
		if err := store.Save(account); err != nil {
			t.Fatalf("save %s: %v", account.Login, err)
		}
	}

	summary, err := store.Validate()
	if err != nil {
		t.Fatalf("validate account store: %v", err)
	}
	want := SnapshotSummary{AccountCount: 2, CharacterCount: 3, Logins: []string{"alpha", "zeta"}}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected validation summary: got %#v want %#v", summary, want)
	}
}

func TestFileStoreValidateTreatsMissingStoreAsEmpty(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "missing"))

	summary, err := store.Validate()
	if err != nil {
		t.Fatalf("validate missing account store: %v", err)
	}
	want := SnapshotSummary{AccountCount: 0, CharacterCount: 0, Logins: []string{}}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected empty validation summary: got %#v want %#v", summary, want)
	}
}

func TestFileStoreValidateFailsClosedOnCorruptSnapshot(t *testing.T) {
	store := NewFileStore(t.TempDir())
	path := store.accountPath("mkmk")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create store dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"login":"mkmk","empire":2,"characters":[`), 0o644); err != nil {
		t.Fatalf("write corrupt account: %v", err)
	}

	_, err := store.Validate()
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for corrupt account store validation, got %v", err)
	}
}

func TestFileStoreBackupToCopiesCommittedSnapshots(t *testing.T) {
	store := NewFileStore(t.TempDir())
	accounts := []Account{
		{Login: "mkmk", Empire: 2, Characters: []loginticket.Character{{ID: 1, Name: "MkmkWar"}}},
		{Login: "Beta", Empire: 1, Characters: []loginticket.Character{{ID: 2, Name: "BetaNinja"}}},
	}
	for _, account := range accounts {
		if err := store.Save(account); err != nil {
			t.Fatalf("save account %s: %v", account.Login, err)
		}
	}
	if err := os.WriteFile(filepath.Join(store.dir, ".account-crashed.json"), []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write crash temp file: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "account-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup accounts: %v", err)
	}

	backup := NewFileStore(backupDir)
	got, err := backup.List()
	if err != nil {
		t.Fatalf("list backup: %v", err)
	}
	gotLogins := make([]string, 0, len(got))
	for _, account := range got {
		gotLogins = append(gotLogins, account.Login)
	}
	if want := []string{"Beta", "mkmk"}; !reflect.DeepEqual(gotLogins, want) {
		t.Fatalf("unexpected backup logins: got %#v want %#v", gotLogins, want)
	}
	if _, err := os.Stat(filepath.Join(backupDir, ".account-crashed.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected crash temp file to be omitted from backup, stat err=%v", err)
	}
}

func TestFileStoreBackupToWritesDeterministicManifest(t *testing.T) {
	store := NewFileStore(t.TempDir())
	accounts := []Account{
		{Login: "zeta", Empire: 3, Characters: []loginticket.Character{{ID: 3, Name: "ZetaWar"}}},
		{Login: "alpha", Empire: 1, Characters: []loginticket.Character{{ID: 1, Name: "AlphaWar"}, {ID: 2, Name: "AlphaNinja"}}},
	}
	for _, account := range accounts {
		if err := store.Save(account); err != nil {
			t.Fatalf("save account %s: %v", account.Login, err)
		}
	}

	backupDir := filepath.Join(t.TempDir(), "account-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup accounts: %v", err)
	}

	manifestRaw, err := os.ReadFile(filepath.Join(backupDir, BackupManifestFilename))
	if err != nil {
		t.Fatalf("read backup manifest: %v", err)
	}
	var manifest BackupManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatalf("decode backup manifest: %v", err)
	}
	if manifest.Format != BackupManifestFormat {
		t.Fatalf("unexpected manifest format: got %q want %q", manifest.Format, BackupManifestFormat)
	}
	wantSummary := SnapshotSummary{AccountCount: 2, CharacterCount: 3, Logins: []string{"alpha", "zeta"}}
	if !reflect.DeepEqual(manifest.Summary, wantSummary) {
		t.Fatalf("unexpected manifest summary: got %#v want %#v", manifest.Summary, wantSummary)
	}
	wantLogins := []string{"alpha", "zeta"}
	gotLogins := make([]string, 0, len(manifest.Files))
	for _, file := range manifest.Files {
		gotLogins = append(gotLogins, file.Login)
		raw, err := os.ReadFile(filepath.Join(backupDir, file.Filename))
		if err != nil {
			t.Fatalf("read manifest account file %s: %v", file.Filename, err)
		}
		checksum := sha256.Sum256(raw)
		if gotChecksum := hex.EncodeToString(checksum[:]); gotChecksum != file.SHA256 {
			t.Fatalf("unexpected checksum for %s: got %s want %s", file.Login, file.SHA256, gotChecksum)
		}
		if int64(len(raw)) != file.SizeBytes {
			t.Fatalf("unexpected size for %s: got %d want %d", file.Login, file.SizeBytes, len(raw))
		}
	}
	if !reflect.DeepEqual(gotLogins, wantLogins) {
		t.Fatalf("unexpected manifest file order: got %#v want %#v", gotLogins, wantLogins)
	}
}

func TestFileStoreRestoreFromIgnoresBackupManifest(t *testing.T) {
	backup := NewFileStore(t.TempDir())
	if err := backup.Save(Account{Login: "mkmk", Empire: 2, Characters: []loginticket.Character{{ID: 1, Name: "MkmkWar"}}}); err != nil {
		t.Fatalf("save backup account: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backup.dir, BackupManifestFilename), []byte(`{"format":"manual"}`), 0o644); err != nil {
		t.Fatalf("write backup manifest: %v", err)
	}

	restored := NewFileStore(filepath.Join(t.TempDir(), "restored"))
	if err := restored.RestoreFrom(backup.dir); err != nil {
		t.Fatalf("restore accounts with manifest: %v", err)
	}
	got, err := restored.List()
	if err != nil {
		t.Fatalf("list restored accounts: %v", err)
	}
	if len(got) != 1 || got[0].Login != "mkmk" {
		t.Fatalf("unexpected restored accounts: %#v", got)
	}
}

func TestFileStoreBackupToRejectsCorruptSourceSnapshot(t *testing.T) {
	store := NewFileStore(t.TempDir())
	path := store.accountPath("mkmk")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create store dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"login":"mkmk","empire":2,"characters":[`), 0o644); err != nil {
		t.Fatalf("write corrupt account: %v", err)
	}

	err := store.BackupTo(filepath.Join(t.TempDir(), "backup"))
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for corrupt source snapshot, got %v", err)
	}
}

func TestFileStoreBackupToRejectsNonEmptyDestination(t *testing.T) {
	store := NewFileStore(t.TempDir())
	if err := store.Save(Account{Login: "mkmk", Empire: 2}); err != nil {
		t.Fatalf("save account: %v", err)
	}
	backupDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(backupDir, "README.txt"), []byte("operator notes"), 0o644); err != nil {
		t.Fatalf("write existing destination file: %v", err)
	}

	err := store.BackupTo(backupDir)
	if !errors.Is(err, ErrBackupDirNotEmpty) {
		t.Fatalf("expected ErrBackupDirNotEmpty, got %v", err)
	}
}

func TestFileStoreBackupToTreatsMissingSourceAsEmptyBackup(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "missing-source"))
	backupDir := filepath.Join(t.TempDir(), "backup")

	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup missing source: %v", err)
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("read backup dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != BackupManifestFilename {
		t.Fatalf("expected only backup manifest for missing source backup, got %#v", entries)
	}
	manifestRaw, err := os.ReadFile(filepath.Join(backupDir, BackupManifestFilename))
	if err != nil {
		t.Fatalf("read backup manifest: %v", err)
	}
	var manifest BackupManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatalf("decode backup manifest: %v", err)
	}
	want := SnapshotSummary{AccountCount: 0, CharacterCount: 0, Logins: []string{}}
	if !reflect.DeepEqual(manifest.Summary, want) {
		t.Fatalf("unexpected empty backup manifest summary: got %#v want %#v", manifest.Summary, want)
	}
}

func TestFileStoreRestoreFromCopiesValidatedBackupIntoEmptyStore(t *testing.T) {
	backup := NewFileStore(t.TempDir())
	accounts := []Account{
		{Login: "mkmk", Empire: 2, Characters: []loginticket.Character{{ID: 1, Name: "MkmkWar"}}},
		{Login: "Beta", Empire: 1, Characters: []loginticket.Character{{ID: 2, Name: "BetaNinja"}}},
	}
	for _, account := range accounts {
		if err := backup.Save(account); err != nil {
			t.Fatalf("save backup account %s: %v", account.Login, err)
		}
	}
	if err := os.WriteFile(filepath.Join(backup.dir, ".account-crashed.json"), []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write backup temp file: %v", err)
	}

	restoreDir := filepath.Join(t.TempDir(), "restored-accounts")
	restored := NewFileStore(restoreDir)
	if err := restored.RestoreFrom(backup.dir); err != nil {
		t.Fatalf("restore accounts: %v", err)
	}

	got, err := restored.List()
	if err != nil {
		t.Fatalf("list restored accounts: %v", err)
	}
	gotLogins := make([]string, 0, len(got))
	for _, account := range got {
		gotLogins = append(gotLogins, account.Login)
	}
	if want := []string{"Beta", "mkmk"}; !reflect.DeepEqual(gotLogins, want) {
		t.Fatalf("unexpected restored logins: got %#v want %#v", gotLogins, want)
	}
	if _, err := os.Stat(filepath.Join(restoreDir, ".account-crashed.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected crash temp file to be omitted from restore, stat err=%v", err)
	}
}

func TestFileStoreRestoreFromRejectsNonEmptyDestination(t *testing.T) {
	backup := NewFileStore(t.TempDir())
	if err := backup.Save(Account{Login: "mkmk", Empire: 2}); err != nil {
		t.Fatalf("save backup account: %v", err)
	}
	restored := NewFileStore(t.TempDir())
	if err := os.WriteFile(filepath.Join(restored.dir, "operator-note.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatalf("write destination marker: %v", err)
	}

	err := restored.RestoreFrom(backup.dir)
	if !errors.Is(err, ErrRestoreDirNotEmpty) {
		t.Fatalf("expected ErrRestoreDirNotEmpty, got %v", err)
	}
}

func TestFileStoreRestoreFromRejectsMissingBackupSource(t *testing.T) {
	restored := NewFileStore(filepath.Join(t.TempDir(), "restored"))
	err := restored.RestoreFrom(filepath.Join(t.TempDir(), "missing-backup"))
	if !errors.Is(err, ErrRestoreSourceNotFound) {
		t.Fatalf("expected ErrRestoreSourceNotFound, got %v", err)
	}
}

func TestFileStoreRestoreFromRejectsCorruptBackupSnapshot(t *testing.T) {
	backup := NewFileStore(t.TempDir())
	path := backup.accountPath("mkmk")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create backup dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"login":"mkmk","empire":2,"characters":[`), 0o644); err != nil {
		t.Fatalf("write corrupt backup account: %v", err)
	}

	restored := NewFileStore(filepath.Join(t.TempDir(), "restored"))
	err := restored.RestoreFrom(backup.dir)
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for corrupt backup snapshot, got %v", err)
	}
}

func TestFileStoreSaveThenLoadRoundTrip(t *testing.T) {
	store := NewFileStore(t.TempDir())
	want := Account{
		Login:  "mkmk",
		Empire: 2,
		Characters: []loginticket.Character{{
			ID:         1,
			VID:        0x01020304,
			Name:       "MkmkWar",
			Job:        0,
			RaceNum:    0,
			Level:      15,
			ST:         6,
			HT:         5,
			DX:         4,
			IQ:         3,
			MainPart:   101,
			HairPart:   201,
			X:          1000,
			Y:          2000,
			MapIndex:   21,
			Empire:     2,
			SkillGroup: 1,
			Gold:       88000,
			Inventory: []inventory.ItemInstance{
				{ID: 1001, Vnum: 27001, Count: 5, Slot: 8, Locked: true},
			},
			Equipment: []inventory.ItemInstance{
				{ID: 2002, Vnum: 19, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
			},
			Quickslots: []loginticket.Quickslot{
				{Position: 3, Type: 1, Slot: 8},
			},
		}},
	}

	if err := store.Save(want); err != nil {
		t.Fatalf("save account: %v", err)
	}
	got, err := store.Load("mkmk")
	if err != nil {
		t.Fatalf("load account: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected account:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestFileStoreLoadReturnsNotFoundForUnknownLogin(t *testing.T) {
	store := NewFileStore(t.TempDir())
	_, err := store.Load("ghost")
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestFileStoreLoadNormalizesMissingItemStateFromLegacySnapshot(t *testing.T) {
	store := NewFileStore(t.TempDir())
	legacyRaw := []byte("{\"login\":\"mkmk\",\"empire\":2,\"characters\":[{\"id\":1,\"name\":\"MkmkWar\"}]}")
	if err := os.WriteFile(store.accountPath("mkmk"), legacyRaw, 0o644); err != nil {
		t.Fatalf("write legacy account: %v", err)
	}

	got, err := store.Load("mkmk")
	if err != nil {
		t.Fatalf("load account: %v", err)
	}
	if len(got.Characters) != 1 {
		t.Fatalf("expected one character, got %d", len(got.Characters))
	}
	character := got.Characters[0]
	if character.Gold != 0 {
		t.Fatalf("expected zero gold, got %d", character.Gold)
	}
	if character.Inventory == nil {
		t.Fatal("expected legacy inventory to normalize to an empty slice, got nil")
	}
	if len(character.Inventory) != 0 {
		t.Fatalf("expected empty inventory, got %#v", character.Inventory)
	}
	if character.Equipment == nil {
		t.Fatal("expected legacy equipment to normalize to an empty slice, got nil")
	}
	if len(character.Equipment) != 0 {
		t.Fatalf("expected empty equipment, got %#v", character.Equipment)
	}
	if character.Quickslots == nil {
		t.Fatal("expected legacy quickslots to normalize to an empty slice, got nil")
	}
	if len(character.Quickslots) != 0 {
		t.Fatalf("expected empty quickslots, got %#v", character.Quickslots)
	}
}

func TestFileStoreSaveDoesNotMutateCallerItemState(t *testing.T) {
	store := NewFileStore(t.TempDir())
	account := Account{
		Login:  "mkmk",
		Empire: 2,
		Characters: []loginticket.Character{{
			ID:   1,
			Name: "MkmkWar",
		}},
	}

	if err := store.Save(account); err != nil {
		t.Fatalf("save account: %v", err)
	}
	if account.Characters[0].Inventory != nil {
		t.Fatalf("expected caller inventory slice to remain nil, got %#v", account.Characters[0].Inventory)
	}
	if account.Characters[0].Equipment != nil {
		t.Fatalf("expected caller equipment slice to remain nil, got %#v", account.Characters[0].Equipment)
	}
	if account.Characters[0].Quickslots != nil {
		t.Fatalf("expected caller quickslots slice to remain nil, got %#v", account.Characters[0].Quickslots)
	}
}

func TestFileStoreSaveSyncsStoreDirectoryAfterCommit(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	originalSyncStoreDir := syncStoreDir
	t.Cleanup(func() { syncStoreDir = originalSyncStoreDir })
	var synced []string
	syncStoreDir = func(path string) error {
		synced = append(synced, path)
		return nil
	}

	if err := store.Save(Account{Login: "mkmk"}); err != nil {
		t.Fatalf("save account: %v", err)
	}
	if !reflect.DeepEqual(synced, []string{dir}) {
		t.Fatalf("expected account store directory sync after commit, got %#v", synced)
	}
}

func TestFileStoreSaveReportsStoreDirectorySyncFailure(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	originalSyncStoreDir := syncStoreDir
	t.Cleanup(func() { syncStoreDir = originalSyncStoreDir })
	syncStoreDir = func(path string) error {
		return fmt.Errorf("sync %s failed", path)
	}

	if err := store.Save(Account{Login: "mkmk"}); err == nil || !strings.Contains(err.Error(), "sync account store dir") {
		t.Fatalf("expected account store directory sync failure, got %v", err)
	}
}

func TestFileStoreSavePersistsEmptyItemStateAsArrays(t *testing.T) {
	store := NewFileStore(t.TempDir())
	account := Account{
		Login:  "mkmk",
		Empire: 2,
		Characters: []loginticket.Character{{
			ID:   1,
			Name: "MkmkWar",
		}},
	}
	if err := store.Save(account); err != nil {
		t.Fatalf("save account: %v", err)
	}

	raw, err := os.ReadFile(store.accountPath("mkmk"))
	if err != nil {
		t.Fatalf("read account file: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "\"gold\":0") {
		t.Fatalf("expected explicit zero gold field, got %s", text)
	}
	if !strings.Contains(text, "\"inventory\":[]") {
		t.Fatalf("expected empty inventory array, got %s", text)
	}
	if !strings.Contains(text, "\"equipment\":[]") {
		t.Fatalf("expected empty equipment array, got %s", text)
	}
	if !strings.Contains(text, "\"quickslots\":[]") {
		t.Fatalf("expected empty quickslots array, got %s", text)
	}
}
