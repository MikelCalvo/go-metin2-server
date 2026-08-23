package minimal

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
)

func sampleRuntimeDurableSafebox() safeboxstore.Snapshot {
	return safeboxstore.Snapshot{Characters: []safeboxstore.CharacterRow{{
		Login:       "safebox-owner",
		CharacterID: 42,
		Cells: []safeboxstore.Cell{
			{Cell: 0, ID: 9001, Vnum: 27002, Count: 1, Locked: true},
			{Cell: 2, ID: 9002, Vnum: 27001, Count: 3},
		},
	}}}
}

func TestGameRuntimeBackupSafeboxStoreWritesManifestedBackup(t *testing.T) {
	safeboxPath := filepath.Join(t.TempDir(), "state", "safebox.json")
	if err := safeboxstore.NewFileStore(safeboxPath).Save(sampleRuntimeDurableSafebox()); err != nil {
		t.Fatalf("save safebox snapshot: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", SafeboxStorePath: safeboxPath},
		loginticket.NewFileStore(t.TempDir()),
		accountstore.NewFileStore(t.TempDir()),
		nil,
		nil,
		itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json")),
		nil,
	)
	if err != nil {
		t.Fatalf("new game runtime: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "safebox-backup")

	summary, err := runtime.BackupSafeboxStore(backupDir)
	if err != nil {
		t.Fatalf("backup safebox store: %v", err)
	}
	want, err := safeboxstore.SummarizeSnapshot(sampleRuntimeDurableSafebox())
	if err != nil {
		t.Fatalf("summarize expected safebox snapshot: %v", err)
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected safebox backup summary: got %#v want %#v", summary, want)
	}
	if _, err := os.Stat(filepath.Join(backupDir, safeboxstore.BackupManifestFilename)); err != nil {
		t.Fatalf("expected safebox backup manifest: %v", err)
	}
}

func TestGameRuntimeValidateSafeboxStoreBackupDryRunsManifestedBackup(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "source", "safebox.json")
	source := safeboxstore.NewFileStore(sourcePath)
	if err := source.Save(sampleRuntimeDurableSafebox()); err != nil {
		t.Fatalf("save source safebox snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "safebox-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("create validated safebox backup: %v", err)
	}
	activePath := filepath.Join(t.TempDir(), "active", "safebox.json")
	active := safeboxstore.NewFileStore(activePath)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", SafeboxStorePath: activePath},
		loginticket.NewFileStore(t.TempDir()),
		accountstore.NewFileStore(t.TempDir()),
		nil,
		nil,
		itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json")),
		nil,
	)
	if err != nil {
		t.Fatalf("new game runtime: %v", err)
	}

	summary, err := runtime.ValidateSafeboxStoreBackup(backupDir)
	if err != nil {
		t.Fatalf("validate safebox store backup: %v", err)
	}
	want, err := safeboxstore.SummarizeSnapshot(sampleRuntimeDurableSafebox())
	if err != nil {
		t.Fatalf("summarize expected safebox snapshot: %v", err)
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected safebox backup validation summary: got %#v want %#v", summary, want)
	}
	if _, err := active.Load(); !errors.Is(err, safeboxstore.ErrSnapshotNotFound) {
		t.Fatalf("expected dry-run validate not to mutate active safebox store, got %v", err)
	}
}

func TestGameRuntimeRestoreSafeboxStoreRestoresManifestedBackup(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "source", "safebox.json")
	source := safeboxstore.NewFileStore(sourcePath)
	backupSnapshot := sampleRuntimeDurableSafebox()
	if err := source.Save(backupSnapshot); err != nil {
		t.Fatalf("save source safebox snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "safebox-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("create validated safebox backup: %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "restore-target", "safebox.json")
	target := safeboxstore.NewFileStore(targetPath)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", SafeboxStorePath: targetPath},
		loginticket.NewFileStore(t.TempDir()),
		accountstore.NewFileStore(t.TempDir()),
		nil,
		nil,
		itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json")),
		nil,
	)
	if err != nil {
		t.Fatalf("new game runtime: %v", err)
	}

	summary, err := runtime.RestoreSafeboxStore(backupDir)
	if err != nil {
		t.Fatalf("restore safebox store: %v", err)
	}
	wantSummary, err := safeboxstore.SummarizeSnapshot(backupSnapshot)
	if err != nil {
		t.Fatalf("summarize expected safebox snapshot: %v", err)
	}
	if !reflect.DeepEqual(summary, wantSummary) {
		t.Fatalf("unexpected safebox restore summary: got %#v want %#v", summary, wantSummary)
	}
	restored, err := target.Load()
	if err != nil {
		t.Fatalf("load restored safebox: %v", err)
	}
	if !reflect.DeepEqual(restored, backupSnapshot) {
		t.Fatalf("unexpected restored safebox:\n got: %#v\nwant: %#v", restored, backupSnapshot)
	}
	status := runtime.PersistenceStatus()
	if !status.SafeboxStore.Valid || !status.SafeboxStore.BackupManifest.Present {
		t.Fatalf("expected restored safebox status with backup manifest, got %#v", status.SafeboxStore)
	}
	if status.SafeboxStore.RestoreBlockedByLiveSessions {
		t.Fatalf("expected restore not blocked without live sessions")
	}
	if !reflect.DeepEqual(status.SafeboxStore.Summary, wantSummary) {
		t.Fatalf("unexpected safebox persistence status summary: got %#v want %#v", status.SafeboxStore.Summary, wantSummary)
	}
}

func TestGameRuntimeRestoreSafeboxStoreRejectsLiveSessionsWithoutMutation(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "source", "safebox.json")
	source := safeboxstore.NewFileStore(sourcePath)
	if err := source.Save(sampleRuntimeDurableSafebox()); err != nil {
		t.Fatalf("save source safebox snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "safebox-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("create validated safebox backup: %v", err)
	}

	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	targetPath := filepath.Join(t.TempDir(), "restore-target", "safebox.json")
	target := safeboxstore.NewFileStore(targetPath)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", SafeboxStorePath: targetPath},
		ticketStore,
		accounts,
		nil,
		nil,
		itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json")),
		nil,
	)
	if err != nil {
		t.Fatalf("new game runtime: %v", err)
	}
	owner := peerVisibilityCharacter("LiveSafeboxGuard", 0x01030815, 0x02040815, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "safebox-live-restore", 0x70707025, owner)
	if err := accounts.Save(accountstore.Account{Login: "safebox-live-restore", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed live-restore account: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "safebox-live-restore", 0x70707025)
	defer closeSessionFlow(t, flow)

	_, err = runtime.RestoreSafeboxStore(backupDir)
	if !errors.Is(err, ErrSafeboxStoreRestoreLiveSessions) {
		t.Fatalf("expected live-session safebox restore guard, got %v", err)
	}
	if _, err := target.Load(); !errors.Is(err, safeboxstore.ErrSnapshotNotFound) {
		t.Fatalf("expected live-session restore guard to leave target store untouched, got %v", err)
	}
	status := runtime.PersistenceStatus()
	if !status.SafeboxStore.RestoreBlockedByLiveSessions {
		t.Fatalf("expected persistence status to report safebox restore blocked by live sessions: %#v", status.SafeboxStore)
	}
}

func TestGameRuntimeCleanupSafeboxStoreCrashTemps(t *testing.T) {
	safeboxPath := filepath.Join(t.TempDir(), "state", "safebox.json")
	store := safeboxstore.NewFileStore(safeboxPath)
	if err := store.Save(sampleRuntimeDurableSafebox()); err != nil {
		t.Fatalf("save safebox snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(safeboxPath), ".safebox-leftover.json"), []byte(`{"characters":[]}`), 0o644); err != nil {
		t.Fatalf("write crash temp: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", SafeboxStorePath: safeboxPath},
		loginticket.NewFileStore(t.TempDir()),
		accountstore.NewFileStore(t.TempDir()),
		nil,
		nil,
		itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json")),
		nil,
	)
	if err != nil {
		t.Fatalf("new game runtime: %v", err)
	}
	summary, err := runtime.CleanupSafeboxStoreCrashTempFiles()
	if err != nil {
		t.Fatalf("cleanup safebox crash temps: %v", err)
	}
	if summary.CrashTempCount != 0 {
		t.Fatalf("expected safebox crash temps to be removed, got %+v", summary)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(safeboxPath), ".safebox-leftover.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected safebox crash temp to be removed, stat err=%v", err)
	}
}
