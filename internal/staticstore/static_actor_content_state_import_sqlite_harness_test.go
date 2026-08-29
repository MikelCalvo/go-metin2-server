//go:build sqlite_harness

package staticstore

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestSQLiteHarnessStaticActorContentStateImportInsertsTip0013Rows(t *testing.T) {
	db := openSQLiteStaticActorContentStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, 20); err != nil {
		t.Fatalf("ApplyToVersion(20): %v", err)
	}

	export := sampleTip0013StaticActorContentStateImportExport(t)
	result, err := ImportStaticActorContentState(ctx, db, export)
	if err != nil {
		t.Fatalf("ImportStaticActorContentState: %v", err)
	}
	if result.MigrationVersion != StaticActorContentStateMigrationVersion || result.MigrationName != StaticActorContentStateMigrationName {
		t.Fatalf("unexpected migration boundary in result: %+v", result)
	}
	if result.InteractionDefinitionCount != 6 || result.MerchantCatalogEntryCount != 2 ||
		result.QuestFlagRewardItemCount != 1 || result.QuestFlagConsumeItemCount != 1 ||
		result.StaticActorCount != 5 || result.RewardDropCount != 2 ||
		result.CombatProfileCount != 1 || result.CombatProfileDeathRewardDropCount != 2 {
		t.Fatalf("unexpected import counts: %+v", result)
	}
	// Export/quarantine order is by actor name, then entity id.
	if len(result.EntityIDs) != 5 || result.EntityIDs[0] != 23 || result.EntityIDs[1] != 13 ||
		result.EntityIDs[2] != 7 || result.EntityIDs[3] != 12 || result.EntityIDs[4] != 11 {
		t.Fatalf("unexpected entity ids: %+v", result.EntityIDs)
	}
	if len(result.CombatProfiles) != 1 || result.CombatProfiles[0] != "practice_static_store_import_wolf" {
		t.Fatalf("unexpected combat profiles: %+v", result.CombatProfiles)
	}

	assertInteractionDefinitionRow(t, db, "info", "lore:alchemist", "The alchemist studies forgotten herbs.", "", nil, nil, nil, 0, "", "", 0, 0, 0, 0, 0, 0)
	assertInteractionDefinitionRow(t, db, "open_safebox", "npc:warehouse", "Open your warehouse.", "", nil, nil, nil, 2, "quest:first_steps", "met_guide", 1, 0, 0, 0, 0, 0)
	assertInteractionDefinitionRow(t, db, "quest_flag", "quest:first_steps", "Quest updated.", "", nil, nil, nil, 0, "quest:first_steps", "met_guide", 0, 1, 0, 50, 0, 10)
	assertInteractionDefinitionRow(t, db, "shop_preview", "npc:merchant", "", "Village Merchant", nil, nil, nil, 0, "", "", 0, 0, 0, 0, 0, 0)
	assertInteractionDefinitionRow(t, db, "talk", "npc:village_guard", "VillageGuard : Keep your blade sharp.", "", nil, nil, nil, 0, "", "", 0, 0, 0, 0, 0, 0)
	assertInteractionDefinitionRow(t, db, "warp", "npc:teleporter", "Step through the gate.", "", uint32Ptr(42), int32Ptr(1700), int32Ptr(2800), 0, "", "", 0, 0, 0, 0, 0, 0)

	assertMerchantCatalogEntry(t, db, "shop_preview", "npc:merchant", 0, 27001, 50, 2)
	assertMerchantCatalogEntry(t, db, "shop_preview", "npc:merchant", 1, 11200, 500, 1)
	assertQuestFlagRewardItem(t, db, "quest_flag", "quest:first_steps", 0, 27001, 2)
	assertQuestFlagConsumeItem(t, db, "quest_flag", "quest:first_steps", 0, 27002, 1)

	assertStaticActorRow(t, db, 7, "PracticeMob", 42, 1800, 2900, 101, uint32Ptr(42), int32Ptr(1700), int32Ptr(2800), worldruntime.StaticActorCombatProfilePracticeMob, nil, nil, stringPtr("practice.reward_mob"), 25, 12, "", "", 0, 0, "", "", "", 0)
	assertStaticActorRow(t, db, 11, "Warehouse", 1, 100, 200, 20010, nil, nil, nil, "", stringPtr("open_safebox"), stringPtr("npc:warehouse"), nil, 0, 0, "", "", 0, 0, "", "", "", 0)
	assertStaticActorRow(t, db, 12, "QuestGuide", 1, 120, 220, 20302, nil, nil, nil, "", stringPtr("quest_flag"), stringPtr("quest:first_steps"), nil, 0, 0, "", "", 0, 0, "", "", "", 0)
	assertStaticActorRow(t, db, 13, "KillQuestMob", 42, 1800, 2900, 101, nil, nil, nil, worldruntime.StaticActorCombatProfilePracticeMob, nil, nil, stringPtr("practice.kill_quest_mob"), 0, 0, "quest:first_steps", "killed_qa_mob", 0, 1, "Quest updated.", "quest:first_steps", "met_guide", 1)
	assertStaticActorRow(t, db, 23, "FormulaWolf", 42, 1800, 2900, 101, nil, nil, nil, "practice_static_store_import_wolf", nil, nil, stringPtr("practice.formula_wolf"), 0, 0, "", "", 0, 0, "", "", "", 0)

	assertStaticActorRewardDrop(t, db, 7, 0, 27001)
	assertStaticActorRewardDrop(t, db, 7, 1, 27002)
	assertCombatProfileRow(t, db, "practice_static_store_import_wolf", 24, 5, 9, 4, int(worldruntime.TrainingDummyBootstrapLevel), 0, 1500, 0, 0, 0, 0, 0, 0, 0, 0, 15, 7)
	assertCombatProfileDeathRewardDrop(t, db, "practice_static_store_import_wolf", 0, 27001)
	assertCombatProfileDeathRewardDrop(t, db, "practice_static_store_import_wolf", 1, 27002)
}

func TestSQLiteHarnessStaticActorContentStateImportRejectsDuplicatePrimaryKey(t *testing.T) {
	db := openSQLiteStaticActorContentStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, 20); err != nil {
		t.Fatalf("ApplyToVersion(20): %v", err)
	}

	export := sampleTip0013StaticActorContentStateImportExport(t)
	if _, err := ImportStaticActorContentState(ctx, db, export); err != nil {
		t.Fatalf("first ImportStaticActorContentState: %v", err)
	}

	_, err := ImportStaticActorContentState(ctx, db, export)
	if err == nil {
		t.Fatal("second ImportStaticActorContentState succeeded, want unique conflict")
	}

	var actorRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM static_actors`).Scan(&actorRows); err != nil {
		t.Fatalf("count static actors after failed reimport: %v", err)
	}
	if actorRows != 5 {
		t.Fatalf("static actor rows after failed reimport = %d, want 5 (no partial second import)", actorRows)
	}
}

func TestSQLiteHarnessStaticActorContentStateImportRejectsMissingSchema(t *testing.T) {
	db := openSQLiteStaticActorContentStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	export := emptyStaticActorContentStateExport()
	_, err := ImportStaticActorContentState(ctx, db, export)
	if !errors.Is(err, ErrStaticActorContentStateImportSchemaRequired) {
		t.Fatalf("ImportStaticActorContentState on empty DB error = %v, want %v", err, ErrStaticActorContentStateImportSchemaRequired)
	}
}

func TestSQLiteHarnessStaticActorContentStateImportRejectsTip0013WithoutChaseDelaySchema(t *testing.T) {
	db := openSQLiteStaticActorContentStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, StaticActorContentStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", StaticActorContentStateMigrationVersion, err)
	}

	export := sampleTip0013StaticActorContentStateImportExport(t)
	_, err := ImportStaticActorContentState(ctx, db, export)
	if !errors.Is(err, ErrStaticActorContentStateImportSchemaRequired) {
		t.Fatalf("ImportStaticActorContentState after tip-0013-only apply error = %v, want %v", err, ErrStaticActorContentStateImportSchemaRequired)
	}
	if !strings.Contains(err.Error(), "16") || !strings.Contains(err.Error(), StaticActorCombatProfileChaseDelayMigrationName) {
		t.Fatalf("SchemaRequired error = %v, want missing chase-delay version/name", err)
	}

	var profileRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM static_actor_combat_profiles`).Scan(&profileRows); err != nil {
		t.Fatalf("count combat profiles after fail-closed import: %v", err)
	}
	if profileRows != 0 {
		t.Fatalf("combat profile rows after tip-0013-only reject = %d, want 0", profileRows)
	}
}

func TestSQLiteHarnessStaticActorContentStateImportRejectsTip0016WithoutReturnDelaySchema(t *testing.T) {
	db := openSQLiteStaticActorContentStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, StaticActorCombatProfileChaseDelayMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", StaticActorCombatProfileChaseDelayMigrationVersion, err)
	}

	export := sampleTip0013StaticActorContentStateImportExport(t)
	_, err := ImportStaticActorContentState(ctx, db, export)
	if !errors.Is(err, ErrStaticActorContentStateImportSchemaRequired) {
		t.Fatalf("ImportStaticActorContentState after tip-0016-only apply error = %v, want %v", err, ErrStaticActorContentStateImportSchemaRequired)
	}
	if !strings.Contains(err.Error(), "17") || !strings.Contains(err.Error(), StaticActorCombatProfileReturnDelayMigrationName) {
		t.Fatalf("SchemaRequired error = %v, want missing return-delay version/name", err)
	}

	var profileRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM static_actor_combat_profiles`).Scan(&profileRows); err != nil {
		t.Fatalf("count combat profiles after fail-closed import: %v", err)
	}
	if profileRows != 0 {
		t.Fatalf("combat profile rows after tip-0016-only reject = %d, want 0", profileRows)
	}
}

func TestSQLiteHarnessStaticActorContentStateImportRejectsTip0017WithoutHomewardDelaySchema(t *testing.T) {
	db := openSQLiteStaticActorContentStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, StaticActorCombatProfileReturnDelayMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", StaticActorCombatProfileReturnDelayMigrationVersion, err)
	}

	export := sampleTip0013StaticActorContentStateImportExport(t)
	_, err := ImportStaticActorContentState(ctx, db, export)
	if !errors.Is(err, ErrStaticActorContentStateImportSchemaRequired) {
		t.Fatalf("ImportStaticActorContentState after tip-0017-only apply error = %v, want %v", err, ErrStaticActorContentStateImportSchemaRequired)
	}
	if !strings.Contains(err.Error(), "18") || !strings.Contains(err.Error(), StaticActorCombatProfileHomewardDelayMigrationName) {
		t.Fatalf("SchemaRequired error = %v, want missing homeward-delay version/name", err)
	}

	var profileRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM static_actor_combat_profiles`).Scan(&profileRows); err != nil {
		t.Fatalf("count combat profiles after fail-closed import: %v", err)
	}
	if profileRows != 0 {
		t.Fatalf("combat profile rows after tip-0017-only reject = %d, want 0", profileRows)
	}
}

func TestSQLiteHarnessStaticActorContentStateImportRejectsTip0018WithoutMaxStepSchema(t *testing.T) {
	db := openSQLiteStaticActorContentStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, StaticActorCombatProfileHomewardDelayMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", StaticActorCombatProfileHomewardDelayMigrationVersion, err)
	}

	export := sampleTip0013StaticActorContentStateImportExport(t)
	_, err := ImportStaticActorContentState(ctx, db, export)
	if !errors.Is(err, ErrStaticActorContentStateImportSchemaRequired) {
		t.Fatalf("ImportStaticActorContentState after tip-0018-only apply error = %v, want %v", err, ErrStaticActorContentStateImportSchemaRequired)
	}
	if !strings.Contains(err.Error(), "19") || !strings.Contains(err.Error(), StaticActorCombatProfileMaxStepMigrationName) {
		t.Fatalf("SchemaRequired error = %v, want missing max-step version/name", err)
	}

	var profileRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM static_actor_combat_profiles`).Scan(&profileRows); err != nil {
		t.Fatalf("count combat profiles after fail-closed import: %v", err)
	}
	if profileRows != 0 {
		t.Fatalf("combat profile rows after tip-0018-only reject = %d, want 0", profileRows)
	}
}

func TestSQLiteHarnessStaticActorContentStateImportRejectsTip0019WithoutReactionDelaySchema(t *testing.T) {
	db := openSQLiteStaticActorContentStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, StaticActorCombatProfileMaxStepMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", StaticActorCombatProfileMaxStepMigrationVersion, err)
	}

	export := sampleTip0013StaticActorContentStateImportExport(t)
	_, err := ImportStaticActorContentState(ctx, db, export)
	if !errors.Is(err, ErrStaticActorContentStateImportSchemaRequired) {
		t.Fatalf("ImportStaticActorContentState after tip-0019-only apply error = %v, want %v", err, ErrStaticActorContentStateImportSchemaRequired)
	}
	if !strings.Contains(err.Error(), "20") || !strings.Contains(err.Error(), StaticActorCombatProfileReactionDelayMigrationName) {
		t.Fatalf("SchemaRequired error = %v, want missing reaction-delay version/name", err)
	}

	var profileRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM static_actor_combat_profiles`).Scan(&profileRows); err != nil {
		t.Fatalf("count combat profiles after fail-closed import: %v", err)
	}
	if profileRows != 0 {
		t.Fatalf("combat profile rows after tip-0019-only reject = %d, want 0", profileRows)
	}
}

func TestSQLiteHarnessStaticActorContentStateImportAcceptsEmptyExport(t *testing.T) {
	db := openSQLiteStaticActorContentStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, 20); err != nil {
		t.Fatalf("ApplyToVersion(20): %v", err)
	}

	result, err := ImportStaticActorContentState(ctx, db, emptyStaticActorContentStateExport())
	if err != nil {
		t.Fatalf("ImportStaticActorContentState(empty): %v", err)
	}
	if result.InteractionDefinitionCount != 0 || result.MerchantCatalogEntryCount != 0 ||
		result.QuestFlagRewardItemCount != 0 || result.QuestFlagConsumeItemCount != 0 ||
		result.StaticActorCount != 0 || result.RewardDropCount != 0 ||
		result.CombatProfileCount != 0 || result.CombatProfileDeathRewardDropCount != 0 {
		t.Fatalf("empty import result = %+v, want zero counts", result)
	}
	if len(result.EntityIDs) != 0 || len(result.InteractionKinds) != 0 || len(result.CombatProfiles) != 0 {
		t.Fatalf("empty import identity lists = %+v", result)
	}
}

func sampleTip0013StaticActorContentStateImportExport(t *testing.T) StaticActorContentStateExport {
	t.Helper()

	const profile = "practice_static_store_import_wolf"
	interactionSnapshot := interactionstore.Snapshot{Definitions: []interactionstore.Definition{
		{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
		{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant", Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: 27001, Price: 50, Count: 2},
			{Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
		}},
		{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "VillageGuard : Keep your blade sharp."},
		{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 42, X: 1700, Y: 2800},
		{Kind: interactionstore.KindOpenSafebox, Ref: "npc:warehouse", Text: "Open your warehouse.", Size: 2, QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestFrom: 1},
		{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestFrom: 0, QuestTo: 1, RewardGold: 50, RewardItems: []interactionstore.RewardItemEntry{{ItemVnum: 27001, Count: 2}}, ConsumeItems: []interactionstore.RewardItemEntry{{ItemVnum: 27002, Count: 1}}, ConsumeExperience: 10},
	}}
	staticSnapshot := Snapshot{
		StaticActors: []StaticActor{
			{EntityID: 7, Name: "PracticeMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, SpawnGroupRef: "practice.reward_mob", SpawnHome: &worldruntime.PositionSnapshot{MapIndex: 42, X: 1700, Y: 2800}, RewardExperience: 25, RewardGold: 12, RewardDropVnums: []uint32{27001, 27002}},
			{EntityID: 11, Name: "Warehouse", MapIndex: 1, X: 100, Y: 200, RaceNum: 20010, InteractionKind: interactionstore.KindOpenSafebox, InteractionRef: "npc:warehouse"},
			{EntityID: 12, Name: "QuestGuide", MapIndex: 1, X: 120, Y: 220, RaceNum: 20302, InteractionKind: interactionstore.KindQuestFlag, InteractionRef: "quest:first_steps"},
			{EntityID: 13, Name: "KillQuestMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, SpawnGroupRef: "practice.kill_quest_mob", RewardQuestRef: "quest:first_steps", RewardQuestFlag: "killed_qa_mob", RewardQuestTo: 1, RewardQuestText: "Quest updated.", RequireQuestRef: "quest:first_steps", RequireQuestFlag: "met_guide", RequireQuestFrom: 1},
			{EntityID: 23, Name: "FormulaWolf", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: profile, SpawnGroupRef: "practice.formula_wolf"},
		},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:        profile,
			MaxHP:          24,
			AttackValue:    9,
			DefenseValue:   4,
			RespawnDelayMs: 1500,
			DeathReward: worldruntime.StaticActorDeathReward{
				Experience: 15,
				Gold:       7,
				DropVnums:  []uint32{27001, 27002},
			},
		}},
	}

	export, err := ExportStaticActorContentState(staticSnapshot, interactionSnapshot)
	if err != nil {
		t.Fatalf("ExportStaticActorContentState sample: %v", err)
	}
	return export
}

func emptyStaticActorContentStateExport() StaticActorContentStateExport {
	return StaticActorContentStateExport{
		MigrationVersion:              StaticActorContentStateMigrationVersion,
		MigrationName:                 StaticActorContentStateMigrationName,
		InteractionDefinitions:        []InteractionDefinitionRow{},
		MerchantCatalogEntries:        []InteractionMerchantCatalogEntryRow{},
		QuestFlagRewardItems:          []InteractionQuestFlagItemRow{},
		QuestFlagConsumeItems:         []InteractionQuestFlagItemRow{},
		StaticActors:                  []StaticActorContentStateRow{},
		RewardDrops:                   []StaticActorRewardDropRow{},
		CombatProfiles:                []StaticActorCombatProfileRow{},
		CombatProfileDeathRewardDrops: []StaticActorCombatProfileDropRow{},
	}
}

func assertInteractionDefinitionRow(
	t *testing.T,
	db *sql.DB,
	kind, ref, text, title string,
	mapIndex *uint32,
	x, y *int32,
	size int,
	questRef, questFlag string,
	questFrom, questTo int64,
	rewardExperience, rewardGold, consumeGold, consumeExperience int64,
) {
	t.Helper()

	var (
		gotKind, gotRef, gotText, gotTitle, gotQuestRef, gotQuestFlag string
		gotMapIndex, gotX, gotY                                       sql.NullInt64
		gotSize                                                       int
		gotQuestFrom, gotQuestTo                                      int64
		gotRewardExperience, gotRewardGold                            int64
		gotConsumeGold, gotConsumeExperience                          int64
	)
	if err := db.QueryRowContext(context.Background(), `
SELECT kind, ref, text, title, map_index, x, y, size, quest_ref, quest_flag, quest_from, quest_to,
       reward_experience, reward_gold, consume_gold, consume_experience
FROM interaction_definitions WHERE kind = ? AND ref = ?`, kind, ref).Scan(
		&gotKind, &gotRef, &gotText, &gotTitle, &gotMapIndex, &gotX, &gotY, &gotSize, &gotQuestRef, &gotQuestFlag,
		&gotQuestFrom, &gotQuestTo, &gotRewardExperience, &gotRewardGold, &gotConsumeGold, &gotConsumeExperience,
	); err != nil {
		t.Fatalf("select interaction definition %s:%s: %v", kind, ref, err)
	}
	if gotKind != kind || gotRef != ref || gotText != text || gotTitle != title || gotSize != size ||
		gotQuestRef != questRef || gotQuestFlag != questFlag || gotQuestFrom != questFrom || gotQuestTo != questTo ||
		gotRewardExperience != rewardExperience || gotRewardGold != rewardGold ||
		gotConsumeGold != consumeGold || gotConsumeExperience != consumeExperience {
		t.Fatalf("interaction definition mismatch for %s:%s", kind, ref)
	}
	assertNullableInt64(t, "map_index", gotMapIndex, mapIndex)
	assertNullableInt32(t, "x", gotX, x)
	assertNullableInt32(t, "y", gotY, y)
}

func assertMerchantCatalogEntry(t *testing.T, db *sql.DB, kind, ref string, slot int, itemVnum, price uint64, count int) {
	t.Helper()
	var gotKind, gotRef string
	var gotSlot, gotCount int
	var gotItemVnum, gotPrice int64
	if err := db.QueryRowContext(context.Background(), `
SELECT definition_kind, definition_ref, slot, item_vnum, price, count
FROM interaction_merchant_catalog_entries WHERE definition_kind = ? AND definition_ref = ? AND slot = ?`,
		kind, ref, slot).Scan(&gotKind, &gotRef, &gotSlot, &gotItemVnum, &gotPrice, &gotCount); err != nil {
		t.Fatalf("select merchant catalog %s:%s slot %d: %v", kind, ref, slot, err)
	}
	if gotKind != kind || gotRef != ref || gotSlot != slot || gotItemVnum != int64(itemVnum) || gotPrice != int64(price) || gotCount != count {
		t.Fatalf("merchant catalog mismatch for %s:%s slot %d", kind, ref, slot)
	}
}

func assertQuestFlagRewardItem(t *testing.T, db *sql.DB, kind, ref string, position int, itemVnum uint32, count int) {
	t.Helper()
	var gotKind, gotRef string
	var gotPosition, gotCount int
	var gotItemVnum int64
	if err := db.QueryRowContext(context.Background(), `
SELECT definition_kind, definition_ref, position, item_vnum, count
FROM interaction_quest_flag_reward_items WHERE definition_kind = ? AND definition_ref = ? AND position = ?`,
		kind, ref, position).Scan(&gotKind, &gotRef, &gotPosition, &gotItemVnum, &gotCount); err != nil {
		t.Fatalf("select quest-flag reward %s:%s position %d: %v", kind, ref, position, err)
	}
	if gotKind != kind || gotRef != ref || gotPosition != position || gotItemVnum != int64(itemVnum) || gotCount != count {
		t.Fatalf("quest-flag reward mismatch for %s:%s position %d", kind, ref, position)
	}
}

func assertQuestFlagConsumeItem(t *testing.T, db *sql.DB, kind, ref string, position int, itemVnum uint32, count int) {
	t.Helper()
	var gotKind, gotRef string
	var gotPosition, gotCount int
	var gotItemVnum int64
	if err := db.QueryRowContext(context.Background(), `
SELECT definition_kind, definition_ref, position, item_vnum, count
FROM interaction_quest_flag_consume_items WHERE definition_kind = ? AND definition_ref = ? AND position = ?`,
		kind, ref, position).Scan(&gotKind, &gotRef, &gotPosition, &gotItemVnum, &gotCount); err != nil {
		t.Fatalf("select quest-flag consume %s:%s position %d: %v", kind, ref, position, err)
	}
	if gotKind != kind || gotRef != ref || gotPosition != position || gotItemVnum != int64(itemVnum) || gotCount != count {
		t.Fatalf("quest-flag consume mismatch for %s:%s position %d", kind, ref, position)
	}
}

func assertStaticActorRow(
	t *testing.T,
	db *sql.DB,
	entityID uint64,
	name string,
	mapIndex uint32,
	x, y int32,
	raceNum uint32,
	spawnHomeMapIndex *uint32,
	spawnHomeX, spawnHomeY *int32,
	combatProfile string,
	interactionKind, interactionRef, spawnGroupRef *string,
	rewardExperience, rewardGold uint64,
	rewardQuestRef, rewardQuestFlag string,
	rewardQuestFrom, rewardQuestTo uint32,
	rewardQuestText, requireQuestRef, requireQuestFlag string,
	requireQuestFrom uint32,
) {
	t.Helper()

	var (
		gotEntityID                                               int64
		gotName, gotCombatProfile                                 string
		gotMapIndex, gotX, gotY, gotRaceNum                       int64
		gotSpawnHomeMapIndex, gotSpawnHomeX, gotSpawnHomeY        sql.NullInt64
		gotInteractionKind, gotInteractionRef, gotSpawnGroupRef   sql.NullString
		gotRewardExperience, gotRewardGold                        int64
		gotRewardQuestRef, gotRewardQuestFlag, gotRewardQuestText string
		gotRewardQuestFrom, gotRewardQuestTo                      int64
		gotRequireQuestRef, gotRequireQuestFlag                   string
		gotRequireQuestFrom                                       int64
	)
	if err := db.QueryRowContext(context.Background(), `
SELECT entity_id, name, map_index, x, y, race_num,
       spawn_home_map_index, spawn_home_x, spawn_home_y,
       combat_profile, interaction_kind, interaction_ref, spawn_group_ref,
       reward_experience, reward_gold, reward_quest_ref, reward_quest_flag, reward_quest_from, reward_quest_to, reward_quest_text,
       require_quest_ref, require_quest_flag, require_quest_from
FROM static_actors WHERE entity_id = ?`, entityID).Scan(
		&gotEntityID, &gotName, &gotMapIndex, &gotX, &gotY, &gotRaceNum,
		&gotSpawnHomeMapIndex, &gotSpawnHomeX, &gotSpawnHomeY,
		&gotCombatProfile, &gotInteractionKind, &gotInteractionRef, &gotSpawnGroupRef,
		&gotRewardExperience, &gotRewardGold, &gotRewardQuestRef, &gotRewardQuestFlag, &gotRewardQuestFrom, &gotRewardQuestTo, &gotRewardQuestText,
		&gotRequireQuestRef, &gotRequireQuestFlag, &gotRequireQuestFrom,
	); err != nil {
		t.Fatalf("select static actor %d: %v", entityID, err)
	}
	if gotEntityID != int64(entityID) || gotName != name || gotMapIndex != int64(mapIndex) || gotX != int64(x) || gotY != int64(y) ||
		gotRaceNum != int64(raceNum) || gotCombatProfile != combatProfile ||
		gotRewardExperience != int64(rewardExperience) || gotRewardGold != int64(rewardGold) ||
		gotRewardQuestRef != rewardQuestRef || gotRewardQuestFlag != rewardQuestFlag ||
		gotRewardQuestFrom != int64(rewardQuestFrom) || gotRewardQuestTo != int64(rewardQuestTo) ||
		gotRewardQuestText != rewardQuestText || gotRequireQuestRef != requireQuestRef ||
		gotRequireQuestFlag != requireQuestFlag || gotRequireQuestFrom != int64(requireQuestFrom) {
		t.Fatalf("static actor mismatch for entity_id %d", entityID)
	}
	assertNullableInt64(t, "spawn_home_map_index", gotSpawnHomeMapIndex, spawnHomeMapIndex)
	assertNullableInt32(t, "spawn_home_x", gotSpawnHomeX, spawnHomeX)
	assertNullableInt32(t, "spawn_home_y", gotSpawnHomeY, spawnHomeY)
	assertNullableString(t, "interaction_kind", gotInteractionKind, interactionKind)
	assertNullableString(t, "interaction_ref", gotInteractionRef, interactionRef)
	assertNullableString(t, "spawn_group_ref", gotSpawnGroupRef, spawnGroupRef)
}

func assertStaticActorRewardDrop(t *testing.T, db *sql.DB, entityID uint64, position int, itemVnum uint32) {
	t.Helper()
	var gotEntityID, gotItemVnum int64
	var gotPosition int
	if err := db.QueryRowContext(context.Background(), `
SELECT entity_id, position, item_vnum FROM static_actor_reward_drops WHERE entity_id = ? AND position = ?`,
		entityID, position).Scan(&gotEntityID, &gotPosition, &gotItemVnum); err != nil {
		t.Fatalf("select reward drop entity_id %d position %d: %v", entityID, position, err)
	}
	if gotEntityID != int64(entityID) || gotPosition != position || gotItemVnum != int64(itemVnum) {
		t.Fatalf("reward drop mismatch for entity_id %d position %d", entityID, position)
	}
}

func assertCombatProfileRow(
	t *testing.T,
	db *sql.DB,
	profile string,
	maxHP, damage, attack, defense, level, rank int,
	respawnDelayMs int64,
	aggro, leash, chaseDelayMs, returnDelayMs, homewardDelayMs int64,
	maxStep int32,
	reactionDelayMs int64,
	retaliation int64,
	deathExperience, deathGold int64,
) {
	t.Helper()
	var (
		gotProfile                                                                          string
		gotMaxHP, gotDamage, gotAttack, gotDefense, gotLevel, gotRank                       int
		gotRespawn                                                                          int64
		gotAggro, gotLeash, gotChaseDelay, gotReturnDelay, gotHomewardDelay, gotRetaliation int64
		gotMaxStep                                                                          int64
		gotReactionDelay                                                                    int64
		gotDeathExperience, gotDeathGold                                                    int64
	)
	if err := db.QueryRowContext(context.Background(), `
SELECT profile, max_hp, damage_per_normal_attack, attack_value, defense_value, level, rank,
       respawn_delay_ms, aggro_radius, leash_radius, chase_delay_ms, return_delay_ms, homeward_delay_ms, max_step, reaction_delay_ms, retaliation_point_delta, death_reward_experience, death_reward_gold
FROM static_actor_combat_profiles WHERE profile = ?`, profile).Scan(
		&gotProfile, &gotMaxHP, &gotDamage, &gotAttack, &gotDefense, &gotLevel, &gotRank,
		&gotRespawn, &gotAggro, &gotLeash, &gotChaseDelay, &gotReturnDelay, &gotHomewardDelay, &gotMaxStep, &gotReactionDelay, &gotRetaliation, &gotDeathExperience, &gotDeathGold,
	); err != nil {
		t.Fatalf("select combat profile %q: %v", profile, err)
	}
	if gotProfile != profile || gotMaxHP != maxHP || gotDamage != damage || gotAttack != attack || gotDefense != defense ||
		gotLevel != level || gotRank != rank || gotRespawn != respawnDelayMs || gotAggro != aggro || gotLeash != leash ||
		gotChaseDelay != chaseDelayMs || gotReturnDelay != returnDelayMs || gotHomewardDelay != homewardDelayMs || gotMaxStep != int64(maxStep) || gotReactionDelay != reactionDelayMs || gotRetaliation != retaliation || gotDeathExperience != deathExperience || gotDeathGold != deathGold {
		t.Fatalf("combat profile mismatch for %q", profile)
	}
}

func assertCombatProfileDeathRewardDrop(t *testing.T, db *sql.DB, profile string, position int, itemVnum uint32) {
	t.Helper()
	var gotProfile string
	var gotPosition int
	var gotItemVnum int64
	if err := db.QueryRowContext(context.Background(), `
SELECT profile, position, item_vnum FROM static_actor_combat_profile_death_reward_drops WHERE profile = ? AND position = ?`,
		profile, position).Scan(&gotProfile, &gotPosition, &gotItemVnum); err != nil {
		t.Fatalf("select death-reward drop %q position %d: %v", profile, position, err)
	}
	if gotProfile != profile || gotPosition != position || gotItemVnum != int64(itemVnum) {
		t.Fatalf("death-reward drop mismatch for %q position %d", profile, position)
	}
}

func assertNullableInt64(t *testing.T, name string, got sql.NullInt64, want *uint32) {
	t.Helper()
	if want == nil {
		if got.Valid {
			t.Fatalf("%s = %d, want NULL", name, got.Int64)
		}
		return
	}
	if !got.Valid || got.Int64 != int64(*want) {
		t.Fatalf("%s = %#v, want %d", name, got, *want)
	}
}

func assertNullableInt32(t *testing.T, name string, got sql.NullInt64, want *int32) {
	t.Helper()
	if want == nil {
		if got.Valid {
			t.Fatalf("%s = %d, want NULL", name, got.Int64)
		}
		return
	}
	if !got.Valid || got.Int64 != int64(*want) {
		t.Fatalf("%s = %#v, want %d", name, got, *want)
	}
}

func assertNullableString(t *testing.T, name string, got sql.NullString, want *string) {
	t.Helper()
	if want == nil {
		if got.Valid {
			t.Fatalf("%s = %q, want NULL", name, got.String)
		}
		return
	}
	if !got.Valid || got.String != *want {
		t.Fatalf("%s = %#v, want %q", name, got, *want)
	}
}

func stringPtr(value string) *string { return &value }

func openSQLiteStaticActorContentStateImportDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "static-actor-content-state-import-harness.sqlite")
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open(sqlite): %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("Ping sqlite static-actor content-state import harness: %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		t.Fatalf("enable foreign_keys: %v", err)
	}
	return db
}
