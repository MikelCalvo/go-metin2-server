package staticstore

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestValidateStaticActorContentStateExportAcceptsCanonicalExport(t *testing.T) {
	export := sampleStaticActorContentStateExport()

	summary, err := ValidateStaticActorContentStateExport(export)
	if err != nil {
		t.Fatalf("validate static actor content-state export: %v", err)
	}
	want := StaticActorContentStateQuarantineSummary{
		InteractionDefinitionCount:        4,
		MerchantCatalogEntryCount:         2,
		QuestFlagRewardItemCount:          0,
		QuestFlagConsumeItemCount:         0,
		StaticActorCount:                  3,
		RewardDropCount:                   2,
		CombatProfileCount:                0,
		CombatProfileDeathRewardDropCount: 0,
		EntityIDs:                         []uint64{2, 7, 9},
		InteractionKinds:                  []string{interactionstore.KindInfo, interactionstore.KindShopPreview, interactionstore.KindTalk, interactionstore.KindWarp},
		CombatProfiles:                    []string{},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected quarantine summary:\n got: %#v\nwant: %#v", summary, want)
	}
}

func TestValidateStaticActorContentStateExportAcceptsEmptyCollections(t *testing.T) {
	export := StaticActorContentStateExport{
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

	summary, err := ValidateStaticActorContentStateExport(export)
	if err != nil {
		t.Fatalf("validate empty static actor content-state export: %v", err)
	}
	want := StaticActorContentStateQuarantineSummary{
		InteractionDefinitionCount:        0,
		MerchantCatalogEntryCount:         0,
		QuestFlagRewardItemCount:          0,
		QuestFlagConsumeItemCount:         0,
		StaticActorCount:                  0,
		RewardDropCount:                   0,
		CombatProfileCount:                0,
		CombatProfileDeathRewardDropCount: 0,
		EntityIDs:                         []uint64{},
		InteractionKinds:                  []string{},
		CombatProfiles:                    []string{},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected empty quarantine summary:\n got: %#v\nwant: %#v", summary, want)
	}
}

func TestValidateStaticActorContentStateExportRejectsWrongMigrationBoundary(t *testing.T) {
	export := sampleStaticActorContentStateExport()
	export.MigrationVersion = 7
	if _, err := ValidateStaticActorContentStateExport(export); !errors.Is(err, ErrInvalidStaticActorContentStateExport) {
		t.Fatalf("expected invalid export error, got %v", err)
	}

	export = sampleStaticActorContentStateExport()
	export.MigrationName = "character_quest_state"
	if _, err := ValidateStaticActorContentStateExport(export); !errors.Is(err, ErrInvalidStaticActorContentStateExport) {
		t.Fatalf("expected invalid export error, got %v", err)
	}
}

func TestValidateStaticActorContentStateExportRejectsMalformedRows(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*StaticActorContentStateExport)
	}{
		{
			name: "nil interaction definitions",
			mutate: func(export *StaticActorContentStateExport) {
				export.InteractionDefinitions = nil
			},
		},
		{
			name: "nil merchant catalog",
			mutate: func(export *StaticActorContentStateExport) {
				export.MerchantCatalogEntries = nil
			},
		},
		{
			name: "nil static actors",
			mutate: func(export *StaticActorContentStateExport) {
				export.StaticActors = nil
			},
		},
		{
			name: "nil reward drops",
			mutate: func(export *StaticActorContentStateExport) {
				export.RewardDrops = nil
			},
		},
		{
			name: "nil combat profiles",
			mutate: func(export *StaticActorContentStateExport) {
				export.CombatProfiles = nil
			},
		},
		{
			name: "nil combat profile death reward drops",
			mutate: func(export *StaticActorContentStateExport) {
				export.CombatProfileDeathRewardDrops = nil
			},
		},
		{
			name: "nil quest flag reward items",
			mutate: func(export *StaticActorContentStateExport) {
				export.QuestFlagRewardItems = nil
			},
		},
		{
			name: "nil quest flag consume items",
			mutate: func(export *StaticActorContentStateExport) {
				export.QuestFlagConsumeItems = nil
			},
		},
		{
			name: "incomplete quest_flag definition",
			mutate: func(export *StaticActorContentStateExport) {
				export.InteractionDefinitions = append(export.InteractionDefinitions, InteractionDefinitionRow{
					Kind: interactionstore.KindQuestFlag,
					Ref:  "quest:first_steps",
					Text: "Quest updated.",
				})
			},
		},
		{
			name: "dangling actor interaction ref",
			mutate: func(export *StaticActorContentStateExport) {
				export.StaticActors = append(export.StaticActors, StaticActorContentStateRow{
					EntityID:        99,
					Name:            "MissingGuide",
					MapIndex:        1,
					X:               100,
					Y:               200,
					RaceNum:         20302,
					InteractionKind: interactionstore.KindTalk,
					InteractionRef:  "npc:missing",
				})
			},
		},
		{
			name: "reward drop for unknown entity",
			mutate: func(export *StaticActorContentStateExport) {
				export.RewardDrops = append(export.RewardDrops, StaticActorRewardDropRow{EntityID: 404, Position: 0, ItemVnum: 27001})
			},
		},
		{
			name: "non-contiguous reward drop positions",
			mutate: func(export *StaticActorContentStateExport) {
				export.RewardDrops = []StaticActorRewardDropRow{
					{EntityID: 7, Position: 0, ItemVnum: 27001},
					{EntityID: 7, Position: 2, ItemVnum: 27002},
				}
			},
		},
		{
			name: "duplicate entity ids",
			mutate: func(export *StaticActorContentStateExport) {
				export.StaticActors = append(export.StaticActors, export.StaticActors[0])
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			export := sampleStaticActorContentStateExport()
			tc.mutate(&export)
			if _, err := ValidateStaticActorContentStateExport(export); !errors.Is(err, ErrInvalidStaticActorContentStateExport) {
				t.Fatalf("expected %v, got %v", ErrInvalidStaticActorContentStateExport, err)
			}
		})
	}
}

func TestQuarantineStaticActorContentStateExportCanonicalizesRowOrder(t *testing.T) {
	export := sampleStaticActorContentStateExport()
	// Shuffle relative order so quarantine must re-sort through ExportStaticActorContentState.
	export.InteractionDefinitions = []InteractionDefinitionRow{
		export.InteractionDefinitions[3],
		export.InteractionDefinitions[1],
		export.InteractionDefinitions[2],
		export.InteractionDefinitions[0],
	}
	export.MerchantCatalogEntries = []InteractionMerchantCatalogEntryRow{
		export.MerchantCatalogEntries[1],
		export.MerchantCatalogEntries[0],
	}
	export.StaticActors = []StaticActorContentStateRow{
		export.StaticActors[2],
		export.StaticActors[0],
		export.StaticActors[1],
	}
	export.RewardDrops = []StaticActorRewardDropRow{
		export.RewardDrops[1],
		export.RewardDrops[0],
	}

	quarantined, summary, err := QuarantineStaticActorContentStateExport(export)
	if err != nil {
		t.Fatalf("quarantine static actor content-state export: %v", err)
	}
	wantExport := sampleStaticActorContentStateExport()
	if !reflect.DeepEqual(quarantined, wantExport) {
		t.Fatalf("unexpected canonical quarantine export:\n got: %#v\nwant: %#v", quarantined, wantExport)
	}
	wantSummary := StaticActorContentStateQuarantineSummary{
		InteractionDefinitionCount:        4,
		MerchantCatalogEntryCount:         2,
		QuestFlagRewardItemCount:          0,
		QuestFlagConsumeItemCount:         0,
		StaticActorCount:                  3,
		RewardDropCount:                   2,
		CombatProfileCount:                0,
		CombatProfileDeathRewardDropCount: 0,
		EntityIDs:                         []uint64{2, 7, 9},
		InteractionKinds:                  []string{interactionstore.KindInfo, interactionstore.KindShopPreview, interactionstore.KindTalk, interactionstore.KindWarp},
		CombatProfiles:                    []string{},
	}
	if !reflect.DeepEqual(summary, wantSummary) {
		t.Fatalf("unexpected quarantine summary:\n got: %#v\nwant: %#v", summary, wantSummary)
	}
}

func sampleStaticActorContentStateExport() StaticActorContentStateExport {
	return StaticActorContentStateExport{
		MigrationVersion: StaticActorContentStateMigrationVersion,
		MigrationName:    StaticActorContentStateMigrationName,
		EntityIDs:        []uint64{2, 7, 9},
		InteractionDefinitionKeys: []InteractionDefinitionKey{
			{Kind: interactionstore.KindInfo, Ref: "lore:alchemist"},
			{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant"},
			{Kind: interactionstore.KindTalk, Ref: "npc:village_guard"},
			{Kind: interactionstore.KindWarp, Ref: "npc:teleporter"},
		},
		CombatProfileNames: []string{},
		InteractionDefinitions: []InteractionDefinitionRow{
			{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
			{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant"},
			{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "VillageGuard : Keep your blade sharp."},
			{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: uint32Ptr(42), X: int32Ptr(1700), Y: int32Ptr(2800)},
		},
		MerchantCatalogEntries: []InteractionMerchantCatalogEntryRow{
			{DefinitionKind: interactionstore.KindShopPreview, DefinitionRef: "npc:merchant", Slot: 0, ItemVnum: 27001, Price: 50, Count: 2},
			{DefinitionKind: interactionstore.KindShopPreview, DefinitionRef: "npc:merchant", Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
		},
		QuestFlagRewardItems:  []InteractionQuestFlagItemRow{},
		QuestFlagConsumeItems: []InteractionQuestFlagItemRow{},
		StaticActors: []StaticActorContentStateRow{
			{EntityID: 7, Name: "PracticeMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, SpawnHomeMapIndex: uint32Ptr(42), SpawnHomeX: int32Ptr(1700), SpawnHomeY: int32Ptr(2800), CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, SpawnGroupRef: "practice.reward_mob", RewardExperience: 25, RewardGold: 12},
			{EntityID: 9, Name: "VillageGuard", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"},
			{EntityID: 2, Name: "VillageMerchant", MapIndex: 1, X: 469500, Y: 964300, RaceNum: 20001, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"},
		},
		RewardDrops: []StaticActorRewardDropRow{
			{EntityID: 7, Position: 0, ItemVnum: 27001},
			{EntityID: 7, Position: 1, ItemVnum: 27002},
		},
		CombatProfiles:                []StaticActorCombatProfileRow{},
		CombatProfileDeathRewardDrops: []StaticActorCombatProfileDropRow{},
	}
}
