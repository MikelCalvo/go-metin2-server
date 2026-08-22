package minimal

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func sampleRuntimeDurableGroundItems() worldruntime.DurableGroundItemSnapshot {
	itemCount := uint16(2)
	goldAmount := uint32(75)
	now := time.Now().UTC().Truncate(time.Second)
	ownershipExpires := now.Add(30 * time.Second)
	despawnItem := now.Add(5 * time.Minute)
	despawnGold := now.Add(6 * time.Minute)
	return worldruntime.DurableGroundItemSnapshot{GroundItems: []worldruntime.DurableGroundItemRecord{
		{
			VID: 0x07000081, Vnum: 27001, ItemCount: &itemCount, ItemID: 0x30010081,
			OwnerLogin: "item-owner", OwnerCharacterID: 11, OwnerVID: 0x02000011, OwnerName: "ItemHero",
			MapIndex: 1, X: 1100, Y: 2100, PickupRange: 450,
			OwnershipExclusive: true, OwnershipExpiresAt: &ownershipExpires, DespawnAt: despawnItem,
		},
		{
			VID: 0x07000082, Vnum: 1, GoldAmount: &goldAmount,
			OwnerLogin: "gold-owner", OwnerCharacterID: 22, OwnerVID: 0x02000022, OwnerName: "GoldHero",
			MapIndex: 1, X: 1200, Y: 2200, PickupRange: 300,
			OwnershipExclusive: false, DespawnAt: despawnGold,
		},
	}}
}

func TestGameRuntimeBackupGroundItemStoreWritesManifestedBackup(t *testing.T) {
	defer worldruntime.DisableDurableGroundItemSyncForTest()()

	groundItemPath := filepath.Join(t.TempDir(), "state", "ground-items.json")
	if err := worldruntime.NewGroundItemFileStore(groundItemPath).Save(sampleRuntimeDurableGroundItems()); err != nil {
		t.Fatalf("save ground item snapshot: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", GroundItemStorePath: groundItemPath},
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
	backupDir := filepath.Join(t.TempDir(), "ground-item-backup")

	summary, err := runtime.BackupGroundItemStore(backupDir)
	if err != nil {
		t.Fatalf("backup ground item store: %v", err)
	}
	want := worldruntime.SummarizeDurableGroundItemSnapshot(sampleRuntimeDurableGroundItems())
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected ground item backup summary: got %#v want %#v", summary, want)
	}
	if _, err := os.Stat(filepath.Join(backupDir, worldruntime.BackupManifestFilename)); err != nil {
		t.Fatalf("expected ground item backup manifest: %v", err)
	}
}

func TestGameRuntimeValidateGroundItemStoreBackupDryRunsManifestedBackup(t *testing.T) {
	defer worldruntime.DisableDurableGroundItemSyncForTest()()

	sourcePath := filepath.Join(t.TempDir(), "source", "ground-items.json")
	source := worldruntime.NewGroundItemFileStore(sourcePath)
	if err := source.Save(sampleRuntimeDurableGroundItems()); err != nil {
		t.Fatalf("save source ground item snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "ground-item-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("create validated ground item backup: %v", err)
	}
	activePath := filepath.Join(t.TempDir(), "active", "ground-items.json")
	active := worldruntime.NewGroundItemFileStore(activePath)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", GroundItemStorePath: activePath},
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

	summary, err := runtime.ValidateGroundItemStoreBackup(backupDir)
	if err != nil {
		t.Fatalf("validate ground item store backup: %v", err)
	}
	want := worldruntime.SummarizeDurableGroundItemSnapshot(sampleRuntimeDurableGroundItems())
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected ground item backup validation summary: got %#v want %#v", summary, want)
	}
	if _, err := active.Load(); !errors.Is(err, worldruntime.ErrGroundItemSnapshotNotFound) {
		t.Fatalf("expected dry-run validate not to mutate active ground item store, got %v", err)
	}
}

func TestGameRuntimeRestoreGroundItemStoreRestoresManifestedBackup(t *testing.T) {
	defer worldruntime.DisableDurableGroundItemSyncForTest()()

	sourcePath := filepath.Join(t.TempDir(), "source", "ground-items.json")
	source := worldruntime.NewGroundItemFileStore(sourcePath)
	backupSnapshot := sampleRuntimeDurableGroundItems()
	if err := source.Save(backupSnapshot); err != nil {
		t.Fatalf("save source ground item snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "ground-item-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("create validated ground item backup: %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "restore-target", "ground-items.json")
	target := worldruntime.NewGroundItemFileStore(targetPath)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", GroundItemStorePath: targetPath},
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
	runtime.now = func() time.Time { return time.Now().UTC().Truncate(time.Second) }
	runtime.sharedWorld.now = runtime.now

	summary, err := runtime.RestoreGroundItemStore(backupDir)
	if err != nil {
		t.Fatalf("restore ground item store: %v", err)
	}
	wantSummary := worldruntime.SummarizeDurableGroundItemSnapshot(backupSnapshot)
	if !reflect.DeepEqual(summary, wantSummary) {
		t.Fatalf("unexpected ground item restore summary: got %#v want %#v", summary, wantSummary)
	}
	restored, err := target.Load()
	if err != nil {
		t.Fatalf("load restored ground items: %v", err)
	}
	wantSnapshot := worldruntime.NormalizeDurableGroundItemSnapshot(backupSnapshot)
	if !reflect.DeepEqual(restored, wantSnapshot) {
		t.Fatalf("unexpected restored ground items:\n got: %#v\nwant: %#v", restored, wantSnapshot)
	}
	live := runtime.sharedWorld.DurableGroundItemSnapshot()
	if !reflect.DeepEqual(live, wantSnapshot) {
		t.Fatalf("expected live shared world rematerialized from restore:\n got: %#v\nwant: %#v", live, wantSnapshot)
	}
	status := runtime.PersistenceStatus()
	if !status.GroundItemStore.Valid || !status.GroundItemStore.BackupManifest.Present {
		t.Fatalf("expected restored ground item status with backup manifest, got %#v", status.GroundItemStore)
	}
	if status.GroundItemStore.RestoreBlockedByLiveSessions {
		t.Fatalf("expected restore not blocked without live sessions")
	}
}

func TestGameRuntimeRestoreGroundItemStoreRejectsLiveSessionsWithoutMutation(t *testing.T) {
	defer worldruntime.DisableDurableGroundItemSyncForTest()()

	sourcePath := filepath.Join(t.TempDir(), "source", "ground-items.json")
	source := worldruntime.NewGroundItemFileStore(sourcePath)
	if err := source.Save(sampleRuntimeDurableGroundItems()); err != nil {
		t.Fatalf("save source ground item snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "ground-item-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("create validated ground item backup: %v", err)
	}

	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	targetPath := filepath.Join(t.TempDir(), "restore-target", "ground-items.json")
	target := worldruntime.NewGroundItemFileStore(targetPath)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", GroundItemStorePath: targetPath},
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
	owner := peerVisibilityCharacter("LiveGroundGuard", 0x01030715, 0x02040715, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "ground-item-live-restore", 0x70707015, owner)
	if err := accounts.Save(accountstore.Account{Login: "ground-item-live-restore", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed live-restore account: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "ground-item-live-restore", 0x70707015)
	defer closeSessionFlow(t, flow)

	_, err = runtime.RestoreGroundItemStore(backupDir)
	if !errors.Is(err, ErrGroundItemStoreRestoreLiveSessions) {
		t.Fatalf("expected live-session ground item restore guard, got %v", err)
	}
	if _, err := target.Load(); !errors.Is(err, worldruntime.ErrGroundItemSnapshotNotFound) {
		t.Fatalf("expected live-session restore guard to leave target store untouched, got %v", err)
	}
	if len(runtime.sharedWorld.DurableGroundItemSnapshot().GroundItems) != 0 {
		t.Fatalf("expected live shared world unchanged when restore is blocked")
	}
}

func TestGameRuntimeCleanupGroundItemStoreCrashTemps(t *testing.T) {
	defer worldruntime.DisableDurableGroundItemSyncForTest()()

	groundItemPath := filepath.Join(t.TempDir(), "state", "ground-items.json")
	store := worldruntime.NewGroundItemFileStore(groundItemPath)
	if err := store.Save(sampleRuntimeDurableGroundItems()); err != nil {
		t.Fatalf("save ground item snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(groundItemPath), ".ground-items-leftover.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write crash temp: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", GroundItemStorePath: groundItemPath},
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
	summary, err := runtime.CleanupGroundItemStoreCrashTempFiles()
	if err != nil {
		t.Fatalf("cleanup ground item crash temps: %v", err)
	}
	if summary.CrashTempCount != 0 {
		t.Fatalf("expected ground item crash temps to be removed, got %+v", summary)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(groundItemPath), ".ground-items-leftover.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected ground item crash temp to be removed, stat err=%v", err)
	}
}
