package staticstore

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestExportStaticActorContentStateBuildsDeterministicRowsMatchingMigrationShape(t *testing.T) {
	staticSnapshot := Snapshot{StaticActors: []StaticActor{
		{EntityID: 9, Name: "VillageGuard", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"},
		{EntityID: 7, Name: "PracticeMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, SpawnGroupRef: "practice.reward_mob", SpawnHome: &worldruntime.PositionSnapshot{MapIndex: 42, X: 1700, Y: 2800}, RewardExperience: 25, RewardGold: 12, RewardDropVnums: []uint32{27002, 27001}},
		{EntityID: 2, Name: "VillageMerchant", MapIndex: 1, X: 469500, Y: 964300, RaceNum: 20001, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"},
	}}
	interactionSnapshot := interactionstore.Snapshot{Definitions: []interactionstore.Definition{
		{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "VillageGuard : Keep your blade sharp."},
		{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
		{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 42, X: 1700, Y: 2800},
		{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant", Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
			{Slot: 0, ItemVnum: 27001, Price: 50, Count: 2},
		}},
	}}

	export, err := ExportStaticActorContentState(staticSnapshot, interactionSnapshot)
	if err != nil {
		t.Fatalf("export static actor content state: %v", err)
	}
	if export.MigrationVersion != StaticActorContentStateMigrationVersion || export.MigrationName != StaticActorContentStateMigrationName {
		t.Fatalf("unexpected migration boundary: version=%d name=%q", export.MigrationVersion, export.MigrationName)
	}
	wantDefinitions := []InteractionDefinitionRow{
		{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
		{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant"},
		{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "VillageGuard : Keep your blade sharp."},
		{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: uint32Ptr(42), X: int32Ptr(1700), Y: int32Ptr(2800)},
	}
	if !reflect.DeepEqual(export.InteractionDefinitions, wantDefinitions) {
		t.Fatalf("unexpected interaction definition rows:\n got: %#v\nwant: %#v", export.InteractionDefinitions, wantDefinitions)
	}
	wantCatalog := []InteractionMerchantCatalogEntryRow{
		{DefinitionKind: interactionstore.KindShopPreview, DefinitionRef: "npc:merchant", Slot: 0, ItemVnum: 27001, Price: 50, Count: 2},
		{DefinitionKind: interactionstore.KindShopPreview, DefinitionRef: "npc:merchant", Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
	}
	if !reflect.DeepEqual(export.MerchantCatalogEntries, wantCatalog) {
		t.Fatalf("unexpected merchant catalog rows:\n got: %#v\nwant: %#v", export.MerchantCatalogEntries, wantCatalog)
	}
	if len(export.QuestFlagRewardItems) != 0 || len(export.QuestFlagConsumeItems) != 0 {
		t.Fatalf("expected empty quest-flag item tables for historical kinds, got rewards=%#v consumes=%#v", export.QuestFlagRewardItems, export.QuestFlagConsumeItems)
	}
	wantActors := []StaticActorContentStateRow{
		{EntityID: 7, Name: "PracticeMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, SpawnHomeMapIndex: uint32Ptr(42), SpawnHomeX: int32Ptr(1700), SpawnHomeY: int32Ptr(2800), CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, SpawnGroupRef: "practice.reward_mob", RewardExperience: 25, RewardGold: 12},
		{EntityID: 9, Name: "VillageGuard", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"},
		{EntityID: 2, Name: "VillageMerchant", MapIndex: 1, X: 469500, Y: 964300, RaceNum: 20001, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"},
	}
	if !reflect.DeepEqual(export.StaticActors, wantActors) {
		t.Fatalf("unexpected static actor rows:\n got: %#v\nwant: %#v", export.StaticActors, wantActors)
	}
	wantDrops := []StaticActorRewardDropRow{
		{EntityID: 7, Position: 0, ItemVnum: 27001},
		{EntityID: 7, Position: 1, ItemVnum: 27002},
	}
	if !reflect.DeepEqual(export.RewardDrops, wantDrops) {
		t.Fatalf("unexpected reward drop rows:\n got: %#v\nwant: %#v", export.RewardDrops, wantDrops)
	}

	exportAgain, err := ExportStaticActorContentState(staticSnapshot, interactionSnapshot)
	if err != nil {
		t.Fatalf("export static actor content state again: %v", err)
	}
	if !reflect.DeepEqual(export, exportAgain) {
		t.Fatalf("expected deterministic static actor content-state export:\n first: %#v\nsecond: %#v", export, exportAgain)
	}
}

func TestExportStaticActorContentStateRejectsRowsThatCannotTargetMigrationSchema(t *testing.T) {
	validDefinitions := interactionstore.Snapshot{Definitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "VillageGuard : Keep your blade sharp."}}}
	missingDefinitionActor := Snapshot{StaticActors: []StaticActor{{EntityID: 9, Name: "VillageGuard", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"}}}

	tooManyDrops := make([]uint32, 256)
	for i := range tooManyDrops {
		tooManyDrops[i] = uint32(27000 + i)
	}
	tooManyDropActor := Snapshot{StaticActors: []StaticActor{{EntityID: 7, Name: "PracticeMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, SpawnGroupRef: "practice.reward_mob", RewardDropVnums: tooManyDrops}}}

	duplicateDefinitions := interactionstore.Snapshot{Definitions: []interactionstore.Definition{
		{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "VillageGuard : Keep your blade sharp."},
		{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "VillageGuard : Keep your spear sharp."},
	}}
	incompleteQuestFlagDefinitions := interactionstore.Snapshot{Definitions: []interactionstore.Definition{{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated."}}}

	cases := []struct {
		name        string
		actors      Snapshot
		definitions interactionstore.Snapshot
		wantErr     error
	}{
		{name: "dangling actor interaction ref", actors: missingDefinitionActor, definitions: interactionstore.Snapshot{}, wantErr: ErrInvalidSnapshot},
		{name: "too many reward drops for migration position column", actors: tooManyDropActor, definitions: validDefinitions, wantErr: ErrInvalidSnapshot},
		{name: "duplicate interaction definition key", actors: Snapshot{}, definitions: duplicateDefinitions, wantErr: interactionstore.ErrInvalidSnapshot},
		{name: "invalid interaction definition body", actors: Snapshot{}, definitions: interactionstore.Snapshot{Definitions: []interactionstore.Definition{{Kind: interactionstore.KindInfo, Ref: "lore:empty"}}}, wantErr: interactionstore.ErrInvalidSnapshot},
		{name: "incomplete quest_flag definition", actors: Snapshot{}, definitions: incompleteQuestFlagDefinitions, wantErr: interactionstore.ErrInvalidSnapshot},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ExportStaticActorContentState(tc.actors, tc.definitions)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestExportStaticActorContentStateProjectsPvEQuestFlagAndOpenSafebox(t *testing.T) {
	interactionSnapshot := interactionstore.Snapshot{Definitions: []interactionstore.Definition{
		{Kind: interactionstore.KindOpenSafebox, Ref: "npc:warehouse", Text: "Open your warehouse.", Size: 2, QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestFrom: 1},
		{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestFrom: 0, QuestTo: 1, RewardGold: 50, RewardItems: []interactionstore.RewardItemEntry{{ItemVnum: 27001, Count: 2}}, ConsumeItems: []interactionstore.RewardItemEntry{{ItemVnum: 27002, Count: 1}}, ConsumeExperience: 10},
	}}
	staticSnapshot := Snapshot{StaticActors: []StaticActor{
		{EntityID: 11, Name: "Warehouse", MapIndex: 1, X: 100, Y: 200, RaceNum: 20010, InteractionKind: interactionstore.KindOpenSafebox, InteractionRef: "npc:warehouse"},
		{EntityID: 12, Name: "QuestGuide", MapIndex: 1, X: 120, Y: 220, RaceNum: 20302, InteractionKind: interactionstore.KindQuestFlag, InteractionRef: "quest:first_steps"},
		{EntityID: 13, Name: "KillQuestMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, SpawnGroupRef: "practice.kill_quest_mob", RewardQuestRef: "quest:first_steps", RewardQuestFlag: "killed_qa_mob", RewardQuestTo: 1, RewardQuestText: "Quest updated.", RequireQuestRef: "quest:first_steps", RequireQuestFlag: "met_guide", RequireQuestFrom: 1},
	}}

	export, err := ExportStaticActorContentState(staticSnapshot, interactionSnapshot)
	if err != nil {
		t.Fatalf("export PvE static actor content state: %v", err)
	}
	if export.MigrationVersion != 12 || export.MigrationName != "static_actor_pve_interaction_state" {
		t.Fatalf("unexpected migration boundary: %#v", export)
	}
	if len(export.InteractionDefinitions) != 2 || export.InteractionDefinitions[0].Kind != interactionstore.KindOpenSafebox || export.InteractionDefinitions[1].Kind != interactionstore.KindQuestFlag {
		t.Fatalf("unexpected interaction definitions: %#v", export.InteractionDefinitions)
	}
	if len(export.QuestFlagRewardItems) != 1 || export.QuestFlagRewardItems[0].ItemVnum != 27001 || export.QuestFlagRewardItems[0].Count != 2 {
		t.Fatalf("unexpected quest flag reward items: %#v", export.QuestFlagRewardItems)
	}
	if len(export.QuestFlagConsumeItems) != 1 || export.QuestFlagConsumeItems[0].ItemVnum != 27002 || export.QuestFlagConsumeItems[0].Count != 1 {
		t.Fatalf("unexpected quest flag consume items: %#v", export.QuestFlagConsumeItems)
	}
	if len(export.StaticActors) != 3 {
		t.Fatalf("unexpected static actors: %#v", export.StaticActors)
	}
	killQuest := export.StaticActors[0]
	if killQuest.Name != "KillQuestMob" || killQuest.RewardQuestFlag != "killed_qa_mob" || killQuest.RequireQuestFlag != "met_guide" {
		t.Fatalf("unexpected kill-quest actor projection: %#v", killQuest)
	}
	if _, err := ValidateStaticActorContentStateExport(export); err != nil {
		t.Fatalf("validate PvE export: %v", err)
	}
}

func TestExportStaticActorContentStateFromStoresReadsCommittedSnapshots(t *testing.T) {
	staticActors := NewFileStore(filepath.Join(t.TempDir(), "state", "static-actors.json"))
	interactions := interactionstore.NewFileStore(filepath.Join(t.TempDir(), "state", "interaction-definitions.json"))
	if err := interactions.Save(interactionstore.Snapshot{Definitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "VillageGuard : Keep your blade sharp."}}}); err != nil {
		t.Fatalf("save interaction definitions: %v", err)
	}
	if err := staticActors.Save(Snapshot{StaticActors: []StaticActor{{EntityID: 9, Name: "VillageGuard", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"}}}); err != nil {
		t.Fatalf("save static actors: %v", err)
	}

	export, err := ExportStaticActorContentStateFromStores(staticActors, interactions)
	if err != nil {
		t.Fatalf("file-store static actor content-state export: %v", err)
	}
	if export.MigrationVersion != StaticActorContentStateMigrationVersion || export.MigrationName != StaticActorContentStateMigrationName {
		t.Fatalf("unexpected migration boundary: %#v", export)
	}
	if len(export.InteractionDefinitions) != 1 || export.InteractionDefinitions[0].Ref != "npc:village_guard" {
		t.Fatalf("unexpected interaction definition rows: %#v", export.InteractionDefinitions)
	}
	if len(export.StaticActors) != 1 || export.StaticActors[0].EntityID != 9 || export.StaticActors[0].InteractionRef != "npc:village_guard" {
		t.Fatalf("unexpected static actor rows: %#v", export.StaticActors)
	}
}

func TestExportStaticActorContentStateFromStoresTreatsMissingSnapshotsAsEmpty(t *testing.T) {
	staticActors := NewFileStore(filepath.Join(t.TempDir(), "missing", "static-actors.json"))
	interactions := interactionstore.NewFileStore(filepath.Join(t.TempDir(), "missing", "interaction-definitions.json"))

	export, err := ExportStaticActorContentStateFromStores(staticActors, interactions)
	if err != nil {
		t.Fatalf("export missing content snapshots: %v", err)
	}
	if export.MigrationVersion != StaticActorContentStateMigrationVersion || export.MigrationName != StaticActorContentStateMigrationName {
		t.Fatalf("unexpected migration boundary: %#v", export)
	}
	if len(export.InteractionDefinitions) != 0 || len(export.MerchantCatalogEntries) != 0 || len(export.QuestFlagRewardItems) != 0 || len(export.QuestFlagConsumeItems) != 0 || len(export.StaticActors) != 0 || len(export.RewardDrops) != 0 {
		t.Fatalf("expected empty export for missing snapshots, got %#v", export)
	}
}

func uint32Ptr(value uint32) *uint32 { return &value }
func int32Ptr(value int32) *int32    { return &value }
