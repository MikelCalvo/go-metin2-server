//go:build sqlite_harness

package worldruntime

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestSQLiteHarnessSQLGroundItemStoreRoundTripPersistsPendingGroundHandles(t *testing.T) {
	db := openSQLiteGroundItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, BootstrapGroundItemOwnershipTimerMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", BootstrapGroundItemOwnershipTimerMigrationVersion, err)
	}
	seedGroundItemSQLRoster(t, db)

	count := uint16(2)
	gold := uint32(250)
	ownershipExpires := time.Date(2026, 9, 18, 12, 0, 30, 0, time.FixedZone("local", 3600))
	despawnItem := time.Date(2026, 9, 18, 12, 5, 0, 0, time.UTC)
	despawnGold := time.Date(2026, 9, 18, 12, 6, 0, 0, time.UTC)
	snapshots := []GroundItemSnapshot{
		{
			VID: 0x0700002d, Vnum: 1, GoldAmount: gold,
			OwnerName: "GroundGoldOwner", OwnerLogin: "ground-gold-owner",
			OwnerCharacterID: 0x0103019d, OwnerVID: 0x0204019d,
			PickupRange: 750, MapIndex: 42, X: 1200, Y: 2200, Z: 3,
			DespawnAt: &despawnGold,
		},
		{
			VID: 0x0700002c, Vnum: 3001, Count: count,
			OwnerName: "GroundItemOwner", OwnerLogin: "ground-item-owner",
			OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c,
			PickupRange: 450, MapIndex: 1, X: 1100, Y: 2100, Z: 2,
			HasSockets: true, Socket0: 1, Socket1: 0, Socket2: 7,
			HasAttributes: true, Attr0Type: 1, Attr0Value: 10, Attr6Type: 7, Attr6Value: -3,
			OwnershipExclusive: true, OwnershipExpiresAt: &ownershipExpires, DespawnAt: &despawnItem,
		},
		{
			VID: 0x0700002e, Vnum: 27001, Count: 1,
			OwnerName: "GroundItemOwner", OwnerLogin: "ground-item-owner",
			OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c,
			PickupRange: 300, MapIndex: 1, X: 1, Y: 2, Z: 0,
			HasSockets: true, HasAttributes: true,
		},
	}

	store := NewSQLGroundItemStore(db)
	if err := store.SaveGroundItems(snapshots); err != nil {
		t.Fatalf("SQLGroundItemStore.SaveGroundItems: %v", err)
	}

	loaded, err := store.LoadGroundItems()
	if err != nil {
		t.Fatalf("SQLGroundItemStore.LoadGroundItems: %v", err)
	}
	want, err := ExportBootstrapGroundItemState(snapshots)
	if err != nil {
		t.Fatalf("ExportBootstrapGroundItemState: %v", err)
	}
	gotExport, err := store.ExportBootstrapGroundItemState()
	if err != nil {
		t.Fatalf("SQLGroundItemStore.ExportBootstrapGroundItemState: %v", err)
	}
	if gotExport.MigrationVersion != BootstrapGroundItemStateMigrationVersion || gotExport.MigrationName != BootstrapGroundItemStateMigrationName {
		t.Fatalf("export identity = %d %q, want tip-0010", gotExport.MigrationVersion, gotExport.MigrationName)
	}
	if !reflect.DeepEqual(gotExport.GroundItems, want.GroundItems) {
		t.Fatalf("unexpected SQL export rows:\n got: %#v\nwant: %#v", gotExport.GroundItems, want.GroundItems)
	}
	reexported, err := ExportBootstrapGroundItemState(loaded)
	if err != nil {
		t.Fatalf("reexport loaded snapshots: %v", err)
	}
	if !reflect.DeepEqual(reexported.GroundItems, want.GroundItems) {
		t.Fatalf("loaded snapshots drifted:\n got: %#v\nwant: %#v", reexported.GroundItems, want.GroundItems)
	}
	if len(loaded) != 3 || loaded[0].VID != 0x0700002c || loaded[1].VID != 0x0700002d || loaded[2].VID != 0x0700002e {
		t.Fatalf("loaded vids = %#v, want ascending vid order", loaded)
	}
	if loaded[2].HasSockets != true || loaded[2].Socket0 != 0 || loaded[2].HasAttributes != true || loaded[2].Attr0Type != 0 {
		t.Fatalf("explicit-zero sockets/attributes were not authoritative: %#v", loaded[2])
	}
	if loaded[0].OwnershipExpiresAt == nil || !loaded[0].OwnershipExpiresAt.Equal(ownershipExpires.UTC()) {
		t.Fatalf("ownership expiry = %v, want %v", loaded[0].OwnershipExpiresAt, ownershipExpires.UTC())
	}
}

func TestSQLiteHarnessSQLGroundItemStoreEmptyTableAndWholeSnapshotReplace(t *testing.T) {
	db := openSQLiteGroundItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, BootstrapGroundItemOwnershipTimerMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", BootstrapGroundItemOwnershipTimerMigrationVersion, err)
	}
	seedGroundItemSQLRoster(t, db)

	store := NewSQLGroundItemStore(db)
	loaded, err := store.LoadGroundItems()
	if err != nil {
		t.Fatalf("SQLGroundItemStore.LoadGroundItems empty: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("empty table snapshots = %#v, want none", loaded)
	}
	export, err := store.ExportBootstrapGroundItemState()
	if err != nil {
		t.Fatalf("SQLGroundItemStore.Export empty: %v", err)
	}
	if export.MigrationVersion != BootstrapGroundItemStateMigrationVersion || len(export.GroundItems) != 0 {
		t.Fatalf("empty SQL export = %#v", export)
	}

	first := groundItemSQLSample(0x0700002c, 3001, 1)
	second := groundItemSQLSample(0x0700002e, 27001, 4)
	if err := store.SaveGroundItems([]GroundItemSnapshot{first, second}); err != nil {
		t.Fatalf("first SQLGroundItemStore.SaveGroundItems: %v", err)
	}
	kept := groundItemSQLSample(0x0700002e, 27002, 2)
	if err := store.SaveGroundItems([]GroundItemSnapshot{kept}); err != nil {
		t.Fatalf("replacement SQLGroundItemStore.SaveGroundItems: %v", err)
	}
	loaded, err = store.LoadGroundItems()
	if err != nil {
		t.Fatalf("SQLGroundItemStore.LoadGroundItems after replace: %v", err)
	}
	if len(loaded) != 1 || loaded[0].VID != kept.VID || loaded[0].Vnum != kept.Vnum || loaded[0].Count != kept.Count {
		t.Fatalf("whole-snapshot replace left unexpected rows: %#v", loaded)
	}
	if err := store.SaveGroundItems(nil); err != nil {
		t.Fatalf("SQLGroundItemStore.SaveGroundItems(nil): %v", err)
	}
	loaded, err = store.LoadGroundItems()
	if err != nil {
		t.Fatalf("SQLGroundItemStore.LoadGroundItems after clear: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("cleared snapshots = %#v, want none", loaded)
	}
}

func TestSQLiteHarnessSQLGroundItemStoreRejectsMissingSchema(t *testing.T) {
	db := openSQLiteGroundItemStateImportDB(t)
	defer db.Close()

	store := NewSQLGroundItemStore(db)
	if _, err := store.LoadGroundItems(); !errors.Is(err, ErrBootstrapGroundItemStateImportSchemaRequired) {
		t.Fatalf("SQLGroundItemStore.LoadGroundItems empty-ledger error = %v, want %v", err, ErrBootstrapGroundItemStateImportSchemaRequired)
	}
	if err := store.SaveGroundItems(nil); !errors.Is(err, ErrBootstrapGroundItemStateImportSchemaRequired) {
		t.Fatalf("SQLGroundItemStore.SaveGroundItems empty-ledger error = %v, want %v", err, ErrBootstrapGroundItemStateImportSchemaRequired)
	}

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, BootstrapGroundItemStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", BootstrapGroundItemStateMigrationVersion, err)
	}
	if _, err := store.LoadGroundItems(); !errors.Is(err, ErrBootstrapGroundItemStateImportSchemaRequired) {
		t.Fatalf("SQLGroundItemStore.LoadGroundItems tip-0010-only error = %v, want %v", err, ErrBootstrapGroundItemStateImportSchemaRequired)
	}
}

func TestSQLiteHarnessSQLGroundItemStoreSaveRollsBackWhenOwnerIsMissing(t *testing.T) {
	db := openSQLiteGroundItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, BootstrapGroundItemOwnershipTimerMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", BootstrapGroundItemOwnershipTimerMigrationVersion, err)
	}
	seedGroundItemSQLRoster(t, db)

	store := NewSQLGroundItemStore(db)
	original := groundItemSQLSample(0x0700002c, 3001, 1)
	if err := store.SaveGroundItems([]GroundItemSnapshot{original}); err != nil {
		t.Fatalf("seed SQLGroundItemStore.SaveGroundItems: %v", err)
	}
	orphan := groundItemSQLSample(0x0700002e, 27001, 1)
	orphan.OwnerCharacterID = 99
	orphan.OwnerLogin = "missing-owner"
	orphan.OwnerName = "MissingOwner"
	if err := store.SaveGroundItems([]GroundItemSnapshot{orphan}); err == nil {
		t.Fatal("SQLGroundItemStore.SaveGroundItems(missing owner) succeeded, want foreign-key failure")
	}
	loaded, err := store.LoadGroundItems()
	if err != nil {
		t.Fatalf("SQLGroundItemStore.LoadGroundItems after rollback: %v", err)
	}
	if len(loaded) != 1 || loaded[0].VID != original.VID || loaded[0].Vnum != original.Vnum {
		t.Fatalf("rolled-back snapshots = %#v, want original vid %d", loaded, original.VID)
	}
}

func seedGroundItemSQLRoster(t *testing.T, executor dbmigrations.SQLMigrationExecutor) {
	t.Helper()
	accounts := []accountstore.Account{
		{
			Login:  "ground-item-owner",
			Empire: 1,
			Characters: []loginticket.Character{
				groundItemStateImportCharacter(0x0103019c, "GroundItemOwner"),
			},
		},
		{
			Login:  "ground-gold-owner",
			Empire: 2,
			Characters: []loginticket.Character{
				{},
				groundItemStateImportCharacter(0x0103019d, "GroundGoldOwner"),
			},
		},
	}
	rosterExport, err := accountstore.ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := accountstore.ImportAccountCharacterRoster(context.Background(), executor, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}
}

func groundItemSQLSample(vid, vnum uint32, count uint16) GroundItemSnapshot {
	despawn := time.Date(2026, 9, 18, 12, 5, 0, 0, time.UTC)
	return GroundItemSnapshot{
		VID: vid, Vnum: vnum, Count: count,
		OwnerName: "GroundItemOwner", OwnerLogin: "ground-item-owner",
		OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c,
		PickupRange: 300, MapIndex: 1, X: 10, Y: 20, Z: 0,
		DespawnAt: &despawn,
	}
}
