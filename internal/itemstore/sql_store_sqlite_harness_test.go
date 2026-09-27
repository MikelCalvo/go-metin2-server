//go:build sqlite_harness

package itemstore

import (
	"context"
	"errors"
	"reflect"
	"testing"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

func TestSQLiteHarnessSQLStoreRoundTripPersistsTemplatesAndChildren(t *testing.T) {
	db := openSQLiteItemTemplateStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, ItemTemplateRefineFailResultVnumMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", ItemTemplateRefineFailResultVnumMigrationVersion, err)
	}

	want := authoredSQLItemTemplateSnapshot()
	store := NewSQLStore(db)
	if err := store.Save(want); err != nil {
		t.Fatalf("SQLStore.Save: %v", err)
	}

	got, err := NewSQLStore(db).Load()
	if err != nil {
		t.Fatalf("SQLStore.Load: %v", err)
	}
	if !reflect.DeepEqual(got, normalizeSnapshot(want)) {
		t.Fatalf("SQLStore round trip\n got: %#v\nwant: %#v", got, normalizeSnapshot(want))
	}

	exported, err := NewSQLStore(db).ExportItemTemplateState()
	if err != nil {
		t.Fatalf("SQLStore.ExportItemTemplateState: %v", err)
	}
	if exported.MigrationVersion != ItemTemplateStateMigrationVersion || exported.MigrationName != ItemTemplateStateMigrationName {
		t.Fatalf("export identity = %d %q, want tip-0009", exported.MigrationVersion, exported.MigrationName)
	}
	if len(exported.Templates) != 4 || len(exported.Sockets) != 3 || len(exported.Attributes) != 1 ||
		len(exported.UseEffects) != 1 || len(exported.EquipEffects) != 1 ||
		len(exported.RefineInfos) != 2 || len(exported.RefineMaterials) != 3 {
		t.Fatalf("unexpected SQL export counts: templates=%d sockets=%d attributes=%d use=%d equip=%d refine=%d materials=%d",
			len(exported.Templates), len(exported.Sockets), len(exported.Attributes),
			len(exported.UseEffects), len(exported.EquipEffects), len(exported.RefineInfos), len(exported.RefineMaterials))
	}
	if exported.Templates[0].Vnum != 11199 {
		t.Fatalf("export order starts at vnum %d, want 11199", exported.Templates[0].Vnum)
	}
	if exported.Templates[3].Vnum != 27001 || !exported.Templates[3].Unique || exported.Templates[3].SafeboxRejectText != "You cannot store this." || !exported.Templates[3].AntiSafebox {
		t.Fatalf("potion SQL row lost unique or safebox reject text: %+v", exported.Templates[3])
	}
}

func TestSQLiteHarnessSQLStoreEmptyTableAndWholeSnapshotReplace(t *testing.T) {
	db := openSQLiteItemTemplateStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, ItemTemplateRefineFailResultVnumMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", ItemTemplateRefineFailResultVnumMigrationVersion, err)
	}

	store := NewSQLStore(db)
	got, err := store.Load()
	if err != nil {
		t.Fatalf("SQLStore.Load empty: %v", err)
	}
	if len(got.Templates) != 0 {
		t.Fatalf("empty table templates = %d, want 0", len(got.Templates))
	}
	exported, err := store.ExportItemTemplateState()
	if err != nil {
		t.Fatalf("SQLStore.Export empty: %v", err)
	}
	if len(exported.Templates) != 0 || exported.MigrationVersion != ItemTemplateStateMigrationVersion {
		t.Fatalf("empty export = %+v", exported)
	}

	if err := store.Save(authoredSQLItemTemplateSnapshot()); err != nil {
		t.Fatalf("first SQLStore.Save: %v", err)
	}
	replacement := Snapshot{Templates: []Template{{
		Vnum: 19, Name: "Replacement Sword", MaxCount: 1, AntiMyShop: true,
	}}}
	if err := store.Save(replacement); err != nil {
		t.Fatalf("replacement SQLStore.Save: %v", err)
	}
	got, err = NewSQLStore(db).Load()
	if err != nil {
		t.Fatalf("SQLStore.Load after replace: %v", err)
	}
	if len(got.Templates) != 1 || got.Templates[0].Vnum != 19 || !got.Templates[0].AntiMyShop || got.Templates[0].MyShopRejectText != "" {
		t.Fatalf("replacement snapshot = %#v", got.Templates)
	}
	var leftover int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM item_template_refine_infos`).Scan(&leftover); err != nil {
		t.Fatalf("count leftover refine infos: %v", err)
	}
	if leftover != 0 {
		t.Fatalf("whole-snapshot replace left %d refine infos", leftover)
	}

	if err := store.Save(Snapshot{}); err != nil {
		t.Fatalf("SQLStore.Save(empty): %v", err)
	}
	got, err = NewSQLStore(db).Load()
	if err != nil {
		t.Fatalf("SQLStore.Load after clear: %v", err)
	}
	if len(got.Templates) != 0 {
		t.Fatalf("cleared templates = %d, want 0", len(got.Templates))
	}
}

func TestSQLiteHarnessSQLStoreRejectsMissingSchema(t *testing.T) {
	db := openSQLiteItemTemplateStateImportDB(t)
	defer db.Close()

	store := NewSQLStore(db)
	if _, err := store.Load(); !errors.Is(err, ErrItemTemplateStateImportSchemaRequired) {
		t.Fatalf("SQLStore.Load empty-ledger error = %v, want %v", err, ErrItemTemplateStateImportSchemaRequired)
	}
	if err := store.Save(Snapshot{}); !errors.Is(err, ErrItemTemplateStateImportSchemaRequired) {
		t.Fatalf("SQLStore.Save empty-ledger error = %v, want %v", err, ErrItemTemplateStateImportSchemaRequired)
	}

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, ItemTemplateStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", ItemTemplateStateMigrationVersion, err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrItemTemplateStateImportSchemaRequired) {
		t.Fatalf("SQLStore.Load tip-0009-only error = %v, want %v", err, ErrItemTemplateStateImportSchemaRequired)
	}
}

func TestSQLiteHarnessSQLStoreSaveRollsBackWhenChildInsertFails(t *testing.T) {
	db := openSQLiteItemTemplateStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, ItemTemplateRefineFailResultVnumMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", ItemTemplateRefineFailResultVnumMigrationVersion, err)
	}
	store := NewSQLStore(db)
	if err := store.Save(Snapshot{Templates: []Template{{
		Vnum: 19, Name: "Kept Sword", MaxCount: 1,
	}}}); err != nil {
		t.Fatalf("seed SQLStore.Save: %v", err)
	}

	// Probability 101 fails the refine-info CHECK after the parent rows are
	// deleted. The transaction must restore the seeded template.
	err := store.Save(Snapshot{Templates: []Template{{
		Vnum: 20, Name: "Broken Refine Sword", MaxCount: 1, Refineable: true,
		RefineInfo: &RefineInfo{ResultVnum: 21, Cost: 1, Probability: 101},
	}}})
	if err == nil {
		t.Fatal("SQLStore.Save(invalid refine probability) succeeded, want CHECK failure")
	}
	got, loadErr := NewSQLStore(db).Load()
	if loadErr != nil {
		t.Fatalf("SQLStore.Load after rollback: %v", loadErr)
	}
	if len(got.Templates) != 1 || got.Templates[0].Vnum != 19 || got.Templates[0].Name != "Kept Sword" {
		t.Fatalf("rollback did not keep the seeded template: %#v", got.Templates)
	}
}

func TestSQLiteHarnessSelectRematerializeStoreUsesSQLWithoutTouchingFileStore(t *testing.T) {
	db := openSQLiteItemTemplateStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, ItemTemplateRefineFailResultVnumMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", ItemTemplateRefineFailResultVnumMigrationVersion, err)
	}

	fileStore := NewMemoryStore()
	if err := fileStore.Save(Snapshot{Templates: []Template{{
		Vnum: 19, Name: "File Sword", MaxCount: 1,
	}}}); err != nil {
		t.Fatalf("seed file store: %v", err)
	}
	sqlStore := NewSQLStore(db)
	selected := SelectRematerializeStore(fileStore, sqlStore)
	if selected != sqlStore {
		t.Fatal("opt-in selector did not return the SQLStore")
	}
	if err := selected.Save(Snapshot{Templates: []Template{{
		Vnum: 27001, Name: "SQL Potion", Stackable: true, MaxCount: 200,
	}}}); err != nil {
		t.Fatalf("opt-in save: %v", err)
	}

	reloaded, err := SelectRematerializeStore(NewMemoryStore(), NewSQLStore(db)).Load()
	if err != nil {
		t.Fatalf("opt-in reload: %v", err)
	}
	if len(reloaded.Templates) != 1 || reloaded.Templates[0].Vnum != 27001 {
		t.Fatalf("opt-in reload = %#v", reloaded.Templates)
	}
	fileSnapshot, err := fileStore.Load()
	if err != nil {
		t.Fatalf("file reload: %v", err)
	}
	if len(fileSnapshot.Templates) != 1 || fileSnapshot.Templates[0].Name != "File Sword" {
		t.Fatalf("opt-in SQL save changed the FileStore: %#v", fileSnapshot.Templates)
	}
}

func authoredSQLItemTemplateSnapshot() Snapshot {
	return Snapshot{Templates: []Template{
		{
			Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200,
			ShopBuyPrice: 50, ShopSellPrice: 13, Highlight: true, Unique: true,
			AntiSell: true, AntiGet: true, AntiSafebox: true, PickupRange: 750,
			Sockets: SocketValues{1, 2, 3}, Attributes: AttributeValues{{Type: 1, Value: 10}},
			UseEffect: &UseEffect{
				PointType: 7, PointIndex: 1, PointDelta: 25, ConsumeCount: 2,
				Message: "Recovered HP", InfoMessage: "You feel better.", SpecialEffectType: 3,
			},
			UseRejectText: "You cannot use this yet.", BuyRejectText: "The merchant will not sell this.",
			DropRejectText: "You cannot drop this.", PickupRejectText: "You cannot pick this up.",
			SellRejectText: "The merchant refuses this.", SafeboxRejectText: "You cannot store this.",
		},
		{
			Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1,
			Refineable: true, Save: true, Irremovable: true, AntiMale: true,
			AppearanceVnum: 11201, EquipSlot: "weapon",
			RefineInfo: &RefineInfo{
				ResultVnum: 11201, Cost: 2500, Probability: 75, KeepOnFail: true,
				Materials: []RefineMaterial{{Vnum: 27001, Count: 2}, {Vnum: 27002, Count: 3}},
			},
			EquipEffect:       &PointEffect{PointType: 1, PointIndex: 0, PointDelta: 4},
			EquipRejectText:   "You cannot wield this.",
			UnequipRejectText: "You cannot remove this.",
		},
		{
			Vnum: 11199, Name: "Downgrade Blade", Stackable: false, MaxCount: 1,
		},
		{
			Vnum: 11300, Name: "Downgrade Source Blade", Stackable: false, MaxCount: 1,
			Refineable: true,
			RefineInfo: &RefineInfo{
				ResultVnum: 11301, Cost: 1800, Probability: 60, FailResultVnum: 11199,
				Materials: []RefineMaterial{{Vnum: 27001, Count: 1}},
			},
		},
	}}
}
