//go:build sqlite_harness

package staticstore

import (
	"context"
	"errors"
	"reflect"
	"testing"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestSQLiteHarnessSQLStoreRoundTripPersistsActorsInteractionsAndProfiles(t *testing.T) {
	db := openSQLiteStaticActorContentStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, StaticActorCombatProfileReactionDelayMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", StaticActorCombatProfileReactionDelayMigrationVersion, err)
	}

	want := authoredSQLStaticActorSnapshot(t)
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

	exported, err := NewSQLStore(db).ExportStaticActorContentState(nil)
	if err != nil {
		t.Fatalf("SQLStore.ExportStaticActorContentState: %v", err)
	}
	if exported.MigrationVersion != StaticActorContentStateMigrationVersion || exported.MigrationName != StaticActorContentStateMigrationName {
		t.Fatalf("export identity = %d %q, want tip-0013", exported.MigrationVersion, exported.MigrationName)
	}
	if len(exported.InteractionDefinitions) != 0 || len(exported.StaticActors) != 2 ||
		len(exported.RewardDrops) != 2 || len(exported.CombatProfiles) != 1 ||
		len(exported.CombatProfileDeathRewardDrops) != 2 {
		t.Fatalf("unexpected SQL export counts: definitions=%d actors=%d drops=%d profiles=%d profile_drops=%d",
			len(exported.InteractionDefinitions), len(exported.StaticActors), len(exported.RewardDrops),
			len(exported.CombatProfiles), len(exported.CombatProfileDeathRewardDrops))
	}
	if exported.StaticActors[0].Name != "FormulaWolf" || exported.StaticActors[1].EntityID != 7 || exported.StaticActors[1].Name != "PracticeMob" {
		t.Fatalf("export actor order = %+v / %+v, want FormulaWolf then PracticeMob", exported.StaticActors[0], exported.StaticActors[1])
	}
	if exported.CombatProfiles[0].Profile != "practice_static_store_sql_wolf" || exported.CombatProfiles[0].ChaseDelayMs != 1500 {
		t.Fatalf("portable profile lost chase delay: %+v", exported.CombatProfiles[0])
	}
	if worldruntime.ValidStaticActorCombatProfile("practice_static_store_sql_wolf") {
		t.Fatal("SQLStore.Save auto-registered the portable combat profile")
	}
}

func TestSQLiteHarnessSQLStoreEmptyTableAndWholeSnapshotReplace(t *testing.T) {
	db := openSQLiteStaticActorContentStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, StaticActorCombatProfileReactionDelayMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", StaticActorCombatProfileReactionDelayMigrationVersion, err)
	}

	store := NewSQLStore(db)
	got, err := store.Load()
	if err != nil {
		t.Fatalf("SQLStore.Load empty: %v", err)
	}
	if len(got.StaticActors) != 0 || len(got.CombatProfiles) != 0 {
		t.Fatalf("empty table snapshot = %#v, want empty", got)
	}
	exported, err := store.ExportStaticActorContentState(nil)
	if err != nil {
		t.Fatalf("SQLStore.Export empty: %v", err)
	}
	if len(exported.StaticActors) != 0 || exported.MigrationVersion != StaticActorContentStateMigrationVersion {
		t.Fatalf("empty export = %+v", exported)
	}

	if err := store.Save(authoredSQLStaticActorSnapshot(t)); err != nil {
		t.Fatalf("first SQLStore.Save: %v", err)
	}
	replacement := Snapshot{StaticActors: []StaticActor{{
		EntityID: 19, Name: "ReplacementGuide", MapIndex: 1, X: 10, Y: 20, RaceNum: 20011,
	}}}
	if err := store.Save(replacement); err != nil {
		t.Fatalf("replacement SQLStore.Save: %v", err)
	}
	got, err = NewSQLStore(db).Load()
	if err != nil {
		t.Fatalf("SQLStore.Load after replace: %v", err)
	}
	if len(got.StaticActors) != 1 || got.StaticActors[0].EntityID != 19 || len(got.CombatProfiles) != 0 {
		t.Fatalf("replacement snapshot = %#v", got)
	}
	var leftoverProfiles, leftoverActors int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM static_actor_combat_profiles`).Scan(&leftoverProfiles); err != nil {
		t.Fatalf("count leftover combat profiles: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM static_actors`).Scan(&leftoverActors); err != nil {
		t.Fatalf("count leftover static actors: %v", err)
	}
	if leftoverProfiles != 0 || leftoverActors != 1 {
		t.Fatalf("whole-snapshot replace left profiles=%d actors=%d", leftoverProfiles, leftoverActors)
	}

	if err := store.Save(Snapshot{}); err != nil {
		t.Fatalf("SQLStore.Save(empty): %v", err)
	}
	got, err = NewSQLStore(db).Load()
	if err != nil {
		t.Fatalf("SQLStore.Load after clear: %v", err)
	}
	if len(got.StaticActors) != 0 {
		t.Fatalf("cleared actors = %d, want 0", len(got.StaticActors))
	}
}

func TestSQLiteHarnessSQLStoreRejectsMissingSchema(t *testing.T) {
	db := openSQLiteStaticActorContentStateImportDB(t)
	defer db.Close()

	store := NewSQLStore(db)
	if _, err := store.Load(); !errors.Is(err, ErrStaticActorContentStateImportSchemaRequired) {
		t.Fatalf("SQLStore.Load empty-ledger error = %v, want %v", err, ErrStaticActorContentStateImportSchemaRequired)
	}
	if err := store.Save(Snapshot{}); !errors.Is(err, ErrStaticActorContentStateImportSchemaRequired) {
		t.Fatalf("SQLStore.Save empty-ledger error = %v, want %v", err, ErrStaticActorContentStateImportSchemaRequired)
	}

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, StaticActorContentStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", StaticActorContentStateMigrationVersion, err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrStaticActorContentStateImportSchemaRequired) {
		t.Fatalf("SQLStore.Load tip-0013-only error = %v, want %v", err, ErrStaticActorContentStateImportSchemaRequired)
	}
}

func TestSQLiteHarnessSQLStoreSaveRollsBackWhenChildInsertFails(t *testing.T) {
	db := openSQLiteStaticActorContentStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, StaticActorCombatProfileReactionDelayMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", StaticActorCombatProfileReactionDelayMigrationVersion, err)
	}
	store := NewSQLStore(db)
	if err := store.Save(Snapshot{StaticActors: []StaticActor{{
		EntityID: 19, Name: "KeptGuide", MapIndex: 1, X: 10, Y: 20, RaceNum: 20011,
	}}}); err != nil {
		t.Fatalf("seed SQLStore.Save: %v", err)
	}

	// chase_delay_ms 1 fails the additive 0016 CHECK after the parent rows are
	// deleted. The transaction must restore the seeded actor.
	err := store.Save(Snapshot{
		StaticActors: []StaticActor{{
			EntityID: 23, Name: "BrokenWolf", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101,
			CombatProfile: "practice_static_store_sql_broken", SpawnGroupRef: "practice.broken_wolf",
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile: "practice_static_store_sql_broken", MaxHP: 24, AttackValue: 9, DefenseValue: 4,
			RespawnDelayMs: 1500, ChaseDelayMs: 1,
		}},
	})
	if err == nil {
		t.Fatal("SQLStore.Save(invalid chase delay) succeeded, want CHECK failure")
	}
	got, loadErr := NewSQLStore(db).Load()
	if loadErr != nil {
		t.Fatalf("SQLStore.Load after rollback: %v", loadErr)
	}
	if len(got.StaticActors) != 1 || got.StaticActors[0].EntityID != 19 || got.StaticActors[0].Name != "KeptGuide" {
		t.Fatalf("rollback did not keep the seeded actor: %#v", got.StaticActors)
	}
	if len(got.CombatProfiles) != 0 {
		t.Fatalf("rollback left a combat profile: %#v", got.CombatProfiles)
	}
}

func TestSQLiteHarnessSelectRematerializeStoreUsesSQLWithoutTouchingFileStore(t *testing.T) {
	db := openSQLiteStaticActorContentStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, StaticActorCombatProfileReactionDelayMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", StaticActorCombatProfileReactionDelayMigrationVersion, err)
	}

	fileStore := NewMemoryStore()
	if err := fileStore.Save(Snapshot{StaticActors: []StaticActor{{
		EntityID: 19, Name: "FileGuide", MapIndex: 1, X: 10, Y: 20, RaceNum: 20011,
	}}}); err != nil {
		t.Fatalf("seed file store: %v", err)
	}
	sqlStore := NewSQLStore(db)
	selected := SelectRematerializeStore(fileStore, sqlStore)
	if selected != sqlStore {
		t.Fatal("opt-in selector did not return the SQLStore")
	}
	if err := selected.Save(Snapshot{StaticActors: []StaticActor{{
		EntityID: 11, Name: "SQLGuide", MapIndex: 1, X: 100, Y: 200, RaceNum: 20010,
	}}}); err != nil {
		t.Fatalf("opt-in save: %v", err)
	}

	reloaded, err := SelectRematerializeStore(NewMemoryStore(), NewSQLStore(db)).Load()
	if err != nil {
		t.Fatalf("opt-in reload: %v", err)
	}
	if len(reloaded.StaticActors) != 1 || reloaded.StaticActors[0].Name != "SQLGuide" {
		t.Fatalf("opt-in reload = %#v", reloaded.StaticActors)
	}
	fileSnapshot, err := fileStore.Load()
	if err != nil {
		t.Fatalf("file reload: %v", err)
	}
	if len(fileSnapshot.StaticActors) != 1 || fileSnapshot.StaticActors[0].Name != "FileGuide" {
		t.Fatalf("opt-in SQL save changed the FileStore: %#v", fileSnapshot.StaticActors)
	}
}

func TestSQLiteHarnessSQLStoreDoesNotRegisterCombatProfiles(t *testing.T) {
	db := openSQLiteStaticActorContentStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, StaticActorCombatProfileReactionDelayMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", StaticActorCombatProfileReactionDelayMigrationVersion, err)
	}
	const profile = "practice_static_store_sql_unregistered"
	worldruntime.UnregisterStaticActorCombatProfileForTest(profile)
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	if err := NewSQLStore(db).Save(Snapshot{
		StaticActors: []StaticActor{{
			EntityID: 23, Name: "FormulaWolf", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101,
			CombatProfile: profile, SpawnGroupRef: "practice.formula_wolf",
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile: profile, MaxHP: 24, AttackValue: 9, DefenseValue: 4, RespawnDelayMs: 1500,
		}},
	}); err != nil {
		t.Fatalf("SQLStore.Save portable profile: %v", err)
	}
	if _, err := NewSQLStore(db).Load(); err != nil {
		t.Fatalf("SQLStore.Load portable profile: %v", err)
	}
	if worldruntime.ValidStaticActorCombatProfile(profile) {
		t.Fatal("SQL load/save registered the portable combat profile")
	}
}

func TestSQLiteHarnessSQLStoreSaveLeavesImportedInteractionDefinitionsUntouched(t *testing.T) {
	db := openSQLiteStaticActorContentStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, StaticActorCombatProfileReactionDelayMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", StaticActorCombatProfileReactionDelayMigrationVersion, err)
	}
	imported := sampleTip0013StaticActorContentStateImportExport(t)
	if _, err := ImportStaticActorContentState(ctx, db, imported); err != nil {
		t.Fatalf("seed tip-0013 import: %v", err)
	}

	if err := NewSQLStore(db).Save(authoredSQLStaticActorSnapshot(t)); err != nil {
		t.Fatalf("SQLStore.Save beside imported interactions: %v", err)
	}
	var definitionCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM interaction_definitions`).Scan(&definitionCount); err != nil {
		t.Fatalf("count interaction definitions: %v", err)
	}
	if definitionCount != len(imported.InteractionDefinitions) {
		t.Fatalf("SQL save changed interaction definitions to %d, want %d", definitionCount, len(imported.InteractionDefinitions))
	}
	exported, err := NewSQLStore(db).ExportStaticActorContentState(nil)
	if err != nil {
		t.Fatalf("SQLStore.Export after save: %v", err)
	}
	if len(exported.InteractionDefinitions) != len(imported.InteractionDefinitions) {
		t.Fatalf("export definitions = %d, want the imported %d", len(exported.InteractionDefinitions), len(imported.InteractionDefinitions))
	}
	foundTalk := false
	for _, definition := range exported.InteractionDefinitions {
		if definition.Kind == interactionstore.KindTalk && definition.Ref == "npc:village_guard" {
			foundTalk = true
		}
	}
	if !foundTalk {
		t.Fatal("SQL save dropped the imported village-guard talk definition")
	}
}

func authoredSQLStaticActorSnapshot(t *testing.T) Snapshot {
	t.Helper()
	const profile = "practice_static_store_sql_wolf"
	snapshot := Snapshot{
		StaticActors: []StaticActor{
			{
				EntityID: 7, Name: "PracticeMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101,
				CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, SpawnGroupRef: "practice.reward_mob",
				SpawnHome:        &worldruntime.PositionSnapshot{MapIndex: 42, X: 1700, Y: 2800},
				RewardExperience: 25,
				RewardGold:       12,
				RewardDropVnums:  []uint32{27001, 27002},
			},
			{
				EntityID: 23, Name: "FormulaWolf", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101,
				CombatProfile: profile, SpawnGroupRef: "practice.formula_wolf",
			},
		},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:        profile,
			MaxHP:          24,
			AttackValue:    9,
			DefenseValue:   4,
			RespawnDelayMs: 1500,
			ChaseDelayMs:   1500,
			DeathReward: worldruntime.StaticActorDeathReward{
				Experience: 15,
				Gold:       7,
				DropVnums:  []uint32{27001, 27002},
			},
		}},
	}
	if err := validateSnapshot(normalizeSnapshot(snapshot)); err != nil {
		t.Fatalf("authored SQL static-actor snapshot is invalid: %v", err)
	}
	return snapshot
}
