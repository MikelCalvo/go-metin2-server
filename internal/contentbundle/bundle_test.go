package contentbundle

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func testMerchantCatalogDefinition() interactionstore.Definition {
	return interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:merchant",
		Title: "Village Merchant",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
			{Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
		},
	}
}

func testMerchantItemTemplates() []itemcatalog.Template {
	return []itemcatalog.Template{
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1},
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
	}
}

func TestCanonicalJSONCanonicalizesBeforeEncoding(t *testing.T) {
	got, err := CanonicalJSON(Bundle{
		StaticActors:           []StaticActor{{Name: "  VillageGuide  ", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: " talk ", InteractionRef: " npc:guide "}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: " talk ", Ref: " npc:guide ", Text: " Welcome. "}},
	})
	if err != nil {
		t.Fatalf("canonical JSON: %v", err)
	}
	want := "{\n  \"static_actors\": [\n    {\n      \"name\": \"VillageGuide\",\n      \"map_index\": 1,\n      \"x\": 1000,\n      \"y\": 2000,\n      \"race_num\": 20302,\n      \"interaction_kind\": \"talk\",\n      \"interaction_ref\": \"npc:guide\"\n    }\n  ],\n  \"interaction_definitions\": [\n    {\n      \"kind\": \"talk\",\n      \"ref\": \"npc:guide\",\n      \"text\": \"Welcome.\"\n    }\n  ]\n}\n"
	if string(got) != want {
		t.Fatalf("unexpected canonical JSON:\n got: %s\nwant: %s", string(got), want)
	}
}

func TestCanonicalJSONMatchesBootstrapNPCServiceExample(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate contentbundle test file")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	raw, err := os.ReadFile(filepath.Join(repoRoot, "docs", "examples", "bootstrap-npc-service-bundle.json"))
	if err != nil {
		t.Fatalf("read example content bundle: %v", err)
	}
	var bundle Bundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("decode example content bundle: %v", err)
	}
	canonical, err := CanonicalJSON(bundle)
	if err != nil {
		t.Fatalf("canonicalize example content bundle: %v", err)
	}
	if !bytes.Equal(canonical, raw) {
		t.Fatalf("example content bundle is not byte-for-byte canonical\n--- got ---\n%s\n--- want ---\n%s", string(raw), string(canonical))
	}
}

func TestCanonicalJSONEmitsEmptyArraysForContractCollections(t *testing.T) {
	got, err := CanonicalJSON(Bundle{})
	if err != nil {
		t.Fatalf("canonical JSON for empty content bundle: %v", err)
	}
	want := "{\n  \"static_actors\": [],\n  \"interaction_definitions\": []\n}\n"
	if string(got) != want {
		t.Fatalf("unexpected empty canonical JSON:\n got: %s\nwant: %s", string(got), want)
	}
}

func TestSummarizeReturnsDeterministicCanonicalCounts(t *testing.T) {
	summary, err := Summarize(Bundle{
		StaticActors: []StaticActor{
			{Name: "  VillageGuide  ", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20301, InteractionKind: " talk ", InteractionRef: " npc:guide "},
			{Name: "Merchant", MapIndex: 2, X: 1200, Y: 2200, RaceNum: 20300, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"},
		},
		SpawnGroups: []SpawnGroup{{
			Ref:             "practice.reward_mob",
			Name:            "Reward Mob",
			MapIndex:        2,
			X:               1300,
			Y:               2300,
			RaceNum:         101,
			CombatProfile:   worldruntime.StaticActorCombatProfileTrainingDummy,
			RewardDropVnums: []uint32{27001},
		}},
		ItemTemplates: testMerchantItemTemplates(),
		InteractionDefinitions: []interactionstore.Definition{
			{Kind: interactionstore.KindInfo, Ref: "lore:unused", Text: "Unused lore kept for a later authored actor."},
			testMerchantCatalogDefinition(),
			{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Welcome."},
		},
	})
	if err != nil {
		t.Fatalf("summarize content bundle: %v", err)
	}
	want := Summary{
		StaticActorCount:             2,
		InteractableStaticActorCount: 2,
		SpawnGroupCount:              1,
		CombatProfileCount:           0,
		ItemTemplateCount:            2,
		ShopCatalogEntryCount:        2,
		RewardDropItemCount:          1,
		RewardDrops: []RewardDropAggregateSummary{
			{ItemVnum: 27001, ItemName: "Small Red Potion", SourceCount: 1, Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		},
		StaticActors: []StaticActor{
			{Name: "Merchant", MapIndex: 2, X: 1200, Y: 2200, RaceNum: 20300, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"},
			{Name: "VillageGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20301, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"},
		},
		ShopCatalogs: []ShopCatalogSummary{{
			Kind:       interactionstore.KindShopPreview,
			Ref:        "npc:merchant",
			Title:      "Village Merchant",
			EntryCount: 2,
			Entries: []ShopCatalogEntrySummary{
				{Slot: 0, ItemVnum: 27001, ItemName: "Small Red Potion", Count: 1, Price: 50, Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
				{Slot: 1, ItemVnum: 11200, ItemName: "Wooden Sword", Count: 1, Price: 500, Stackable: false, MaxCount: 1},
			},
		}},
		ShopRouteCount:             1,
		ShopRoutes:                 []ShopRouteSummary{{ActorName: "Merchant", SourceMapIndex: 2, SourceX: 1200, SourceY: 2200, Ref: "npc:merchant", Title: "Village Merchant", EntryCount: 2}},
		InteractionDefinitionCount: 3,
		ItemTemplates: []ItemTemplateReferenceSummary{
			{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1},
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		},
		ReferencedInteractionDefinitionCount:   2,
		UnreferencedInteractionDefinitionCount: 1,
		InteractionKinds: []InteractionKindSummary{
			{Kind: interactionstore.KindInfo, Count: 1, ReferencedCount: 0, UnreferencedCount: 1},
			{Kind: interactionstore.KindShopPreview, Count: 1, ReferencedCount: 1, UnreferencedCount: 0},
			{Kind: interactionstore.KindTalk, Count: 1, ReferencedCount: 1, UnreferencedCount: 0},
		},
		InteractionDefinitionPreviews: []InteractionDefinitionPreviewSummary{
			{Kind: interactionstore.KindInfo, Ref: "lore:unused", Preview: "Unused lore kept for a later authored actor."},
			{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Preview: "Village Merchant: [0] Small Red Potion x1 @ 50g; [1] Wooden Sword x1 @ 500g"},
			{Kind: interactionstore.KindTalk, Ref: "npc:guide", Preview: "Welcome."},
		},
		ReferencedInteractionDefinitions: []InteractionDefinitionReferenceSummary{
			{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant"},
			{Kind: interactionstore.KindTalk, Ref: "npc:guide"},
		},
		UnreferencedInteractionDefinitions: []InteractionDefinitionReferenceSummary{
			{Kind: interactionstore.KindInfo, Ref: "lore:unused"},
		},
		InteractableStaticActors: []InteractableStaticActorSummary{
			{Name: "Merchant", MapIndex: 2, X: 1200, Y: 2200, RaceNum: 20300, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant", Preview: "Village Merchant: [0] Small Red Potion x1 @ 50g; [1] Wooden Sword x1 @ 500g"},
			{Name: "VillageGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20301, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide", Preview: "VillageGuide:\nWelcome."},
		},
		SpawnGroups: []SpawnGroupReferenceSummary{
			{Ref: "practice.reward_mob", Name: "Reward Mob", MapIndex: 2, X: 1300, Y: 2300, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, RewardDropVnums: []uint32{27001}, RewardDropItems: []RewardDropItemSummary{
				{ItemVnum: 27001, ItemName: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
			}},
		},
		Maps: []MapContentSummary{
			{MapIndex: 1, StaticActorCount: 1, InteractableStaticActorCount: 1, TalkActorCount: 1, SpawnGroupCount: 0},
			{MapIndex: 2, StaticActorCount: 1, InteractableStaticActorCount: 1, ShopPreviewActorCount: 1, ShopCatalogEntryCount: 2, SpawnGroupCount: 1, RewardDropItemCount: 1},
		},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected content bundle summary:\n got: %#v\nwant: %#v", summary, want)
	}
}

func TestBuildImportPreviewReturnsDeterministicSummaryDeltas(t *testing.T) {
	preview, err := BuildImportPreview(
		Bundle{
			StaticActors: []StaticActor{
				{Name: "VillageGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"},
			},
			InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Welcome."}},
		},
		Bundle{
			StaticActors: []StaticActor{
				{Name: "Merchant", MapIndex: 1, X: 1200, Y: 2200, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"},
				{Name: "Teleporter", MapIndex: 1, X: 1300, Y: 2300, RaceNum: 20303, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:teleporter"},
			},
			ItemTemplates: testMerchantItemTemplates(),
			InteractionDefinitions: []interactionstore.Definition{
				testMerchantCatalogDefinition(),
				{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 7, X: 1300, Y: 2300},
			},
		},
	)
	if err != nil {
		t.Fatalf("build import preview: %v", err)
	}
	if preview.Current.StaticActorCount != 1 || preview.Candidate.StaticActorCount != 2 {
		t.Fatalf("unexpected preview summaries: current=%+v candidate=%+v", preview.Current, preview.Candidate)
	}
	wantDeltas := SummaryDeltas{
		StaticActorCount:             SummaryCountDelta{Current: 1, Candidate: 2, Delta: 1},
		InteractableStaticActorCount: SummaryCountDelta{Current: 1, Candidate: 2, Delta: 1},
		SpawnGroupCount:              SummaryCountDelta{},
		CombatProfileCount:           SummaryCountDelta{},
		ItemTemplateCount:            SummaryCountDelta{Current: 0, Candidate: 2, Delta: 2},
		ShopCatalogEntryCount:        SummaryCountDelta{Current: 0, Candidate: 2, Delta: 2},
		ShopCatalogs: []ShopCatalogDelta{{
			Kind:   interactionstore.KindShopPreview,
			Ref:    "npc:merchant",
			Change: "added",
			Candidate: &ShopCatalogSummary{
				Kind:       interactionstore.KindShopPreview,
				Ref:        "npc:merchant",
				Title:      "Village Merchant",
				EntryCount: 2,
				Entries: []ShopCatalogEntrySummary{
					{Slot: 0, ItemVnum: 27001, ItemName: "Small Red Potion", Count: 1, Price: 50, Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
					{Slot: 1, ItemVnum: 11200, ItemName: "Wooden Sword", Count: 1, Price: 500, Stackable: false, MaxCount: 1},
				},
			},
		}},
		ShopRouteCount:                         SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
		WarpDestinationCount:                   SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
		WarpDestinations:                       []WarpDestinationDelta{{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", Change: "added", Candidate: &WarpDestinationSummary{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 7, X: 1300, Y: 2300}}},
		WarpRouteCount:                         SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
		RewardDropItemCount:                    SummaryCountDelta{},
		InteractionDefinitionCount:             SummaryCountDelta{Current: 1, Candidate: 2, Delta: 1},
		ReferencedInteractionDefinitionCount:   SummaryCountDelta{Current: 1, Candidate: 2, Delta: 1},
		UnreferencedInteractionDefinitionCount: SummaryCountDelta{},
		StaticActors: []StaticActorDelta{
			{Change: "added", Candidate: &StaticActor{Name: "Merchant", MapIndex: 1, X: 1200, Y: 2200, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"}},
			{Change: "added", Candidate: &StaticActor{Name: "Teleporter", MapIndex: 1, X: 1300, Y: 2300, RaceNum: 20303, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:teleporter"}},
			{Change: "removed", Current: &StaticActor{Name: "VillageGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"}},
		},
		InteractionKinds: []InteractionKindDelta{
			{Kind: interactionstore.KindShopPreview, Count: SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1}, ReferencedCount: SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1}, UnreferencedCount: SummaryCountDelta{}},
			{Kind: interactionstore.KindTalk, Count: SummaryCountDelta{Current: 1, Candidate: 0, Delta: -1}, ReferencedCount: SummaryCountDelta{Current: 1, Candidate: 0, Delta: -1}, UnreferencedCount: SummaryCountDelta{}},
			{Kind: interactionstore.KindWarp, Count: SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1}, ReferencedCount: SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1}, UnreferencedCount: SummaryCountDelta{}},
		},
		InteractionDefinitions: []InteractionDefinitionDelta{
			{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Change: "added", CandidatePreview: "Village Merchant: [0] Small Red Potion x1 @ 50g; [1] Wooden Sword x1 @ 500g"},
			{Kind: interactionstore.KindTalk, Ref: "npc:guide", Change: "removed", CurrentPreview: "Welcome."},
			{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", Change: "added", CandidatePreview: "Step through the gate. [warp -> map 7 @ 1300,2300]"},
		},
		ItemTemplates: []ItemTemplateDelta{
			{Vnum: 11200, Change: "added", Candidate: &itemcatalog.Template{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1}},
			{Vnum: 27001, Change: "added", Candidate: &itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}},
		},
		ShopRoutes: []ShopRouteDelta{
			{ActorName: "Merchant", SourceMapIndex: 1, SourceX: 1200, SourceY: 2200, Ref: "npc:merchant", Change: "added", Candidate: &ShopRouteSummary{ActorName: "Merchant", SourceMapIndex: 1, SourceX: 1200, SourceY: 2200, Ref: "npc:merchant", Title: "Village Merchant", EntryCount: 2}},
		},
		WarpRoutes: []WarpRouteDelta{
			{ActorName: "Teleporter", SourceMapIndex: 1, SourceX: 1300, SourceY: 2300, Ref: "npc:teleporter", Change: "added", Candidate: &WarpRouteSummary{ActorName: "Teleporter", SourceMapIndex: 1, SourceX: 1300, SourceY: 2300, Ref: "npc:teleporter", Text: "Step through the gate.", TargetMapIndex: 7, TargetX: 1300, TargetY: 2300}},
		},
		Maps: []MapContentDelta{{
			MapIndex:                     1,
			StaticActorCount:             SummaryCountDelta{Current: 1, Candidate: 2, Delta: 1},
			InteractableStaticActorCount: SummaryCountDelta{Current: 1, Candidate: 2, Delta: 1},
			TalkActorCount:               SummaryCountDelta{Current: 1, Candidate: 0, Delta: -1},
			ShopPreviewActorCount:        SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
			ShopCatalogEntryCount:        SummaryCountDelta{Current: 0, Candidate: 2, Delta: 2},
			WarpActorCount:               SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
			StaticActors: []StaticActorDelta{
				{Change: "added", Candidate: &StaticActor{Name: "Merchant", MapIndex: 1, X: 1200, Y: 2200, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"}},
				{Change: "added", Candidate: &StaticActor{Name: "Teleporter", MapIndex: 1, X: 1300, Y: 2300, RaceNum: 20303, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:teleporter"}},
				{Change: "removed", Current: &StaticActor{Name: "VillageGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"}},
			},
		}},
	}
	if !reflect.DeepEqual(preview.Deltas, wantDeltas) {
		t.Fatalf("unexpected import preview deltas:\n got: %#v\nwant: %#v", preview.Deltas, wantDeltas)
	}
}

func TestBuildImportPreviewReturnsStaticActorDeltas(t *testing.T) {
	currentBlacksmith := StaticActor{Name: "Blacksmith", MapIndex: 1, X: 900, Y: 1900, RaceNum: 20300}
	candidateMerchant := StaticActor{Name: "Merchant", MapIndex: 1, X: 1200, Y: 2200, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"}

	preview, err := BuildImportPreview(
		Bundle{
			StaticActors: []StaticActor{
				currentBlacksmith,
				{Name: "VillageGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"},
			},
			InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Welcome."}},
		},
		Bundle{
			StaticActors: []StaticActor{
				candidateMerchant,
				{Name: "VillageGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"},
			},
			ItemTemplates: testMerchantItemTemplates(),
			InteractionDefinitions: []interactionstore.Definition{
				testMerchantCatalogDefinition(),
				{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Welcome."},
			},
		},
	)
	if err != nil {
		t.Fatalf("build import preview static actor deltas: %v", err)
	}

	want := []StaticActorDelta{
		{Change: "removed", Current: &currentBlacksmith},
		{Change: "added", Candidate: &candidateMerchant},
	}
	if !reflect.DeepEqual(preview.Deltas.StaticActors, want) {
		t.Fatalf("unexpected static-actor import preview deltas:\n got: %#v\nwant: %#v", preview.Deltas.StaticActors, want)
	}
}

func TestBuildImportPreviewReturnsInteractionKindDeltas(t *testing.T) {
	preview, err := BuildImportPreview(
		Bundle{
			StaticActors: []StaticActor{
				{Name: "VillageGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"},
			},
			InteractionDefinitions: []interactionstore.Definition{
				{Kind: interactionstore.KindInfo, Ref: "lore:unused", Text: "Unused lore."},
				{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Welcome."},
			},
		},
		Bundle{
			StaticActors: []StaticActor{
				{Name: "NoticeBoard", MapIndex: 1, X: 900, Y: 1900, RaceNum: 20304, InteractionKind: interactionstore.KindInfo, InteractionRef: "lore:notice"},
				{Name: "Merchant", MapIndex: 1, X: 1200, Y: 2200, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"},
			},
			ItemTemplates: testMerchantItemTemplates(),
			InteractionDefinitions: []interactionstore.Definition{
				{Kind: interactionstore.KindInfo, Ref: "lore:notice", Text: "Read the notice board."},
				testMerchantCatalogDefinition(),
				{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 7, X: 1300, Y: 2300},
			},
		},
	)
	if err != nil {
		t.Fatalf("build import preview interaction-kind deltas: %v", err)
	}

	want := []InteractionKindDelta{
		{
			Kind:              interactionstore.KindInfo,
			Count:             SummaryCountDelta{Current: 1, Candidate: 1, Delta: 0},
			ReferencedCount:   SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
			UnreferencedCount: SummaryCountDelta{Current: 1, Candidate: 0, Delta: -1},
		},
		{
			Kind:              interactionstore.KindShopPreview,
			Count:             SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
			ReferencedCount:   SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
			UnreferencedCount: SummaryCountDelta{Current: 0, Candidate: 0, Delta: 0},
		},
		{
			Kind:              interactionstore.KindTalk,
			Count:             SummaryCountDelta{Current: 1, Candidate: 0, Delta: -1},
			ReferencedCount:   SummaryCountDelta{Current: 1, Candidate: 0, Delta: -1},
			UnreferencedCount: SummaryCountDelta{Current: 0, Candidate: 0, Delta: 0},
		},
		{
			Kind:              interactionstore.KindWarp,
			Count:             SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
			ReferencedCount:   SummaryCountDelta{Current: 0, Candidate: 0, Delta: 0},
			UnreferencedCount: SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
		},
	}
	if !reflect.DeepEqual(preview.Deltas.InteractionKinds, want) {
		t.Fatalf("unexpected interaction-kind import preview deltas:\n got: %#v\nwant: %#v", preview.Deltas.InteractionKinds, want)
	}
}

func TestBuildImportPreviewReturnsInteractionDefinitionDeltas(t *testing.T) {
	preview, err := BuildImportPreview(
		Bundle{
			StaticActors: []StaticActor{
				{Name: "VillageGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"},
			},
			InteractionDefinitions: []interactionstore.Definition{
				{Kind: interactionstore.KindInfo, Ref: "lore:notice", Text: "Old notice."},
				{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Welcome."},
			},
		},
		Bundle{
			StaticActors: []StaticActor{
				{Name: "NoticeBoard", MapIndex: 1, X: 900, Y: 1900, RaceNum: 20304, InteractionKind: interactionstore.KindInfo, InteractionRef: "lore:notice"},
				{Name: "Merchant", MapIndex: 1, X: 1200, Y: 2200, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"},
			},
			ItemTemplates: testMerchantItemTemplates(),
			InteractionDefinitions: []interactionstore.Definition{
				{Kind: interactionstore.KindInfo, Ref: "lore:notice", Text: "New notice."},
				testMerchantCatalogDefinition(),
			},
		},
	)
	if err != nil {
		t.Fatalf("build import preview interaction-definition deltas: %v", err)
	}

	want := []InteractionDefinitionDelta{
		{Kind: interactionstore.KindInfo, Ref: "lore:notice", Change: "changed", CurrentPreview: "Old notice.", CandidatePreview: "New notice."},
		{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Change: "added", CandidatePreview: "Village Merchant: [0] Small Red Potion x1 @ 50g; [1] Wooden Sword x1 @ 500g"},
		{Kind: interactionstore.KindTalk, Ref: "npc:guide", Change: "removed", CurrentPreview: "Welcome."},
	}
	if !reflect.DeepEqual(preview.Deltas.InteractionDefinitions, want) {
		t.Fatalf("unexpected interaction-definition import preview deltas:\n got: %#v\nwant: %#v", preview.Deltas.InteractionDefinitions, want)
	}
}

func TestBuildImportPreviewReturnsServiceRouteDeltas(t *testing.T) {
	redPotion := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}
	woodenSword := itemcatalog.Template{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1}
	currentShop := interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:merchant",
		Title: "Old Merchant",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: redPotion.Vnum, Price: 50, Count: 1},
		},
	}
	candidateShop := interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:merchant",
		Title: "Village Merchant",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: redPotion.Vnum, Price: 50, Count: 1},
			{Slot: 1, ItemVnum: woodenSword.Vnum, Price: 500, Count: 1},
		},
	}
	candidateRemoteShop := interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:remote_merchant",
		Title: "Remote Merchant",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: redPotion.Vnum, Price: 75, Count: 1},
		},
	}
	currentGate := interactionstore.Definition{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "Old gate.", MapIndex: 2, X: 2000, Y: 3000}
	candidateGate := interactionstore.Definition{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "New gate.", MapIndex: 3, X: 2100, Y: 3100}
	currentOldGate := interactionstore.Definition{Kind: interactionstore.KindWarp, Ref: "npc:old_gate", Text: "Old route.", MapIndex: 4, X: 2200, Y: 3200}

	preview, err := BuildImportPreview(
		Bundle{
			StaticActors: []StaticActor{
				{Name: "Merchant", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: currentShop.Ref},
				{Name: "Gate", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 20300, InteractionKind: interactionstore.KindWarp, InteractionRef: currentGate.Ref},
				{Name: "OldGate", MapIndex: 1, X: 1200, Y: 2200, RaceNum: 20300, InteractionKind: interactionstore.KindWarp, InteractionRef: currentOldGate.Ref},
			},
			ItemTemplates: []itemcatalog.Template{redPotion},
			InteractionDefinitions: []interactionstore.Definition{
				currentShop,
				currentGate,
				currentOldGate,
			},
		},
		Bundle{
			StaticActors: []StaticActor{
				{Name: "Merchant", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: candidateShop.Ref},
				{Name: "Gate", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 20300, InteractionKind: interactionstore.KindWarp, InteractionRef: candidateGate.Ref},
				{Name: "RemoteMerchant", MapIndex: 3, X: 3000, Y: 4000, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: candidateRemoteShop.Ref},
			},
			ItemTemplates: []itemcatalog.Template{redPotion, woodenSword},
			InteractionDefinitions: []interactionstore.Definition{
				candidateShop,
				candidateGate,
				candidateRemoteShop,
			},
		},
	)
	if err != nil {
		t.Fatalf("build import preview service route deltas: %v", err)
	}

	currentMerchantRoute := ShopRouteSummary{ActorName: "Merchant", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "npc:merchant", Title: "Old Merchant", EntryCount: 1}
	candidateMerchantRoute := ShopRouteSummary{ActorName: "Merchant", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "npc:merchant", Title: "Village Merchant", EntryCount: 2}
	candidateRemoteRoute := ShopRouteSummary{ActorName: "RemoteMerchant", SourceMapIndex: 3, SourceX: 3000, SourceY: 4000, Ref: "npc:remote_merchant", Title: "Remote Merchant", EntryCount: 1}
	wantShopRoutes := []ShopRouteDelta{
		{ActorName: "Merchant", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "npc:merchant", Change: "changed", Current: &currentMerchantRoute, Candidate: &candidateMerchantRoute},
		{ActorName: "RemoteMerchant", SourceMapIndex: 3, SourceX: 3000, SourceY: 4000, Ref: "npc:remote_merchant", Change: "added", Candidate: &candidateRemoteRoute},
	}
	if !reflect.DeepEqual(preview.Deltas.ShopRoutes, wantShopRoutes) {
		t.Fatalf("unexpected shop route import preview deltas:\n got: %#v\nwant: %#v", preview.Deltas.ShopRoutes, wantShopRoutes)
	}

	currentGateRoute := WarpRouteSummary{ActorName: "Gate", SourceMapIndex: 1, SourceX: 1100, SourceY: 2100, Ref: "npc:gate", Text: "Old gate.", TargetMapIndex: 2, TargetX: 2000, TargetY: 3000}
	candidateGateRoute := WarpRouteSummary{ActorName: "Gate", SourceMapIndex: 1, SourceX: 1100, SourceY: 2100, Ref: "npc:gate", Text: "New gate.", TargetMapIndex: 3, TargetX: 2100, TargetY: 3100}
	currentOldGateRoute := WarpRouteSummary{ActorName: "OldGate", SourceMapIndex: 1, SourceX: 1200, SourceY: 2200, Ref: "npc:old_gate", Text: "Old route.", TargetMapIndex: 4, TargetX: 2200, TargetY: 3200}
	wantWarpRoutes := []WarpRouteDelta{
		{ActorName: "Gate", SourceMapIndex: 1, SourceX: 1100, SourceY: 2100, Ref: "npc:gate", Change: "changed", Current: &currentGateRoute, Candidate: &candidateGateRoute},
		{ActorName: "OldGate", SourceMapIndex: 1, SourceX: 1200, SourceY: 2200, Ref: "npc:old_gate", Change: "removed", Current: &currentOldGateRoute},
	}
	if !reflect.DeepEqual(preview.Deltas.WarpRoutes, wantWarpRoutes) {
		t.Fatalf("unexpected warp route import preview deltas:\n got: %#v\nwant: %#v", preview.Deltas.WarpRoutes, wantWarpRoutes)
	}
}

func TestBuildImportPreviewReturnsWarpDestinationDeltas(t *testing.T) {
	currentGate := interactionstore.Definition{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "Old gate.", MapIndex: 2, X: 2000, Y: 3000}
	currentOldGate := interactionstore.Definition{Kind: interactionstore.KindWarp, Ref: "npc:old_gate", Text: "Old route.", MapIndex: 4, X: 2200, Y: 3200}
	candidateGate := interactionstore.Definition{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "New gate.", MapIndex: 3, X: 2100, Y: 3100}
	candidateRemoteGate := interactionstore.Definition{Kind: interactionstore.KindWarp, Ref: "npc:remote_gate", Text: "Remote route.", MapIndex: 9, X: 9000, Y: 9100}

	preview, err := BuildImportPreview(
		Bundle{InteractionDefinitions: []interactionstore.Definition{currentOldGate, currentGate}},
		Bundle{InteractionDefinitions: []interactionstore.Definition{candidateRemoteGate, candidateGate}},
	)
	if err != nil {
		t.Fatalf("build import preview warp destination deltas: %v", err)
	}

	currentGateDestination := WarpDestinationSummary{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "Old gate.", MapIndex: 2, X: 2000, Y: 3000}
	candidateGateDestination := WarpDestinationSummary{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "New gate.", MapIndex: 3, X: 2100, Y: 3100}
	currentOldGateDestination := WarpDestinationSummary{Kind: interactionstore.KindWarp, Ref: "npc:old_gate", Text: "Old route.", MapIndex: 4, X: 2200, Y: 3200}
	candidateRemoteGateDestination := WarpDestinationSummary{Kind: interactionstore.KindWarp, Ref: "npc:remote_gate", Text: "Remote route.", MapIndex: 9, X: 9000, Y: 9100}
	want := []WarpDestinationDelta{
		{Kind: interactionstore.KindWarp, Ref: "npc:gate", Change: "changed", Current: &currentGateDestination, Candidate: &candidateGateDestination},
		{Kind: interactionstore.KindWarp, Ref: "npc:old_gate", Change: "removed", Current: &currentOldGateDestination},
		{Kind: interactionstore.KindWarp, Ref: "npc:remote_gate", Change: "added", Candidate: &candidateRemoteGateDestination},
	}
	if !reflect.DeepEqual(preview.Deltas.WarpDestinations, want) {
		t.Fatalf("unexpected warp destination import preview deltas:\n got: %#v\nwant: %#v", preview.Deltas.WarpDestinations, want)
	}
}

func TestBuildImportPreviewReturnsShopCatalogDeltas(t *testing.T) {
	redPotion := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}
	woodenSword := itemcatalog.Template{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1}
	currentShop := interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:merchant",
		Title: "Old Merchant",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: redPotion.Vnum, Price: 50, Count: 1},
		},
	}
	currentOldShop := interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:old_merchant",
		Title: "Old Remote Merchant",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: redPotion.Vnum, Price: 80, Count: 1},
		},
	}
	candidateShop := interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:merchant",
		Title: "Village Merchant",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: redPotion.Vnum, Price: 50, Count: 1},
			{Slot: 1, ItemVnum: woodenSword.Vnum, Price: 500, Count: 1},
		},
	}
	candidateRemoteShop := interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:remote_merchant",
		Title: "Remote Merchant",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: redPotion.Vnum, Price: 75, Count: 1},
		},
	}

	preview, err := BuildImportPreview(
		Bundle{
			ItemTemplates: []itemcatalog.Template{redPotion},
			InteractionDefinitions: []interactionstore.Definition{
				currentShop,
				currentOldShop,
			},
		},
		Bundle{
			ItemTemplates: []itemcatalog.Template{redPotion, woodenSword},
			InteractionDefinitions: []interactionstore.Definition{
				candidateShop,
				candidateRemoteShop,
			},
		},
	)
	if err != nil {
		t.Fatalf("build import preview shop catalog deltas: %v", err)
	}

	currentMerchantCatalog := ShopCatalogSummary{
		Kind:       interactionstore.KindShopPreview,
		Ref:        "npc:merchant",
		Title:      "Old Merchant",
		EntryCount: 1,
		Entries: []ShopCatalogEntrySummary{
			{Slot: 0, ItemVnum: 27001, ItemName: "Small Red Potion", Count: 1, Price: 50, Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		},
	}
	candidateMerchantCatalog := ShopCatalogSummary{
		Kind:       interactionstore.KindShopPreview,
		Ref:        "npc:merchant",
		Title:      "Village Merchant",
		EntryCount: 2,
		Entries: []ShopCatalogEntrySummary{
			{Slot: 0, ItemVnum: 27001, ItemName: "Small Red Potion", Count: 1, Price: 50, Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
			{Slot: 1, ItemVnum: 11200, ItemName: "Wooden Sword", Count: 1, Price: 500, Stackable: false, MaxCount: 1},
		},
	}
	currentOldCatalog := ShopCatalogSummary{
		Kind:       interactionstore.KindShopPreview,
		Ref:        "npc:old_merchant",
		Title:      "Old Remote Merchant",
		EntryCount: 1,
		Entries: []ShopCatalogEntrySummary{
			{Slot: 0, ItemVnum: 27001, ItemName: "Small Red Potion", Count: 1, Price: 80, Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		},
	}
	candidateRemoteCatalog := ShopCatalogSummary{
		Kind:       interactionstore.KindShopPreview,
		Ref:        "npc:remote_merchant",
		Title:      "Remote Merchant",
		EntryCount: 1,
		Entries: []ShopCatalogEntrySummary{
			{Slot: 0, ItemVnum: 27001, ItemName: "Small Red Potion", Count: 1, Price: 75, Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		},
	}
	want := []ShopCatalogDelta{
		{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Change: "changed", Current: &currentMerchantCatalog, Candidate: &candidateMerchantCatalog},
		{Kind: interactionstore.KindShopPreview, Ref: "npc:old_merchant", Change: "removed", Current: &currentOldCatalog},
		{Kind: interactionstore.KindShopPreview, Ref: "npc:remote_merchant", Change: "added", Candidate: &candidateRemoteCatalog},
	}
	if !reflect.DeepEqual(preview.Deltas.ShopCatalogs, want) {
		t.Fatalf("unexpected shop catalog import preview deltas:\n got: %#v\nwant: %#v", preview.Deltas.ShopCatalogs, want)
	}
}

func TestBuildImportPreviewReturnsCombatProfileDeltasForSpawnReferencedProfiles(t *testing.T) {
	currentAlpha := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_alpha_profile", MaxHP: 24, DamagePerNormalAttack: 3, AttackValue: 7, DefenseValue: 4, Level: 4, Rank: 1, RespawnDelayMs: 1500}
	currentBeta := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_beta_profile", MaxHP: 30, DamagePerNormalAttack: 5, AttackValue: 8, DefenseValue: 3, Level: 6, Rank: 2, RespawnDelayMs: 2500}
	candidateAlpha := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_alpha_profile", MaxHP: 24, DamagePerNormalAttack: 5, AttackValue: 9, DefenseValue: 4, Level: 4, Rank: 1, RespawnDelayMs: 1500}
	candidateGamma := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_gamma_profile", MaxHP: 40, DamagePerNormalAttack: 6, AttackValue: 10, DefenseValue: 4, Level: 7, Rank: 3, RespawnDelayMs: 3000}

	preview, err := BuildImportPreview(
		Bundle{
			SpawnGroups: []SpawnGroup{
				{Ref: "practice.alpha", Name: "Alpha Mob", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: currentAlpha.Profile},
				{Ref: "practice.beta", Name: "Beta Mob", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 102, CombatProfile: currentBeta.Profile},
			},
			CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{currentBeta, currentAlpha},
		},
		Bundle{
			SpawnGroups: []SpawnGroup{
				{Ref: "practice.alpha", Name: "Alpha Mob", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: candidateAlpha.Profile},
				{Ref: "practice.gamma", Name: "Gamma Mob", MapIndex: 2, X: 1200, Y: 2200, RaceNum: 103, CombatProfile: candidateGamma.Profile},
			},
			CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{candidateGamma, candidateAlpha},
		},
	)
	if err != nil {
		t.Fatalf("build import preview combat-profile deltas: %v", err)
	}

	want := []CombatProfileDelta{
		{Profile: "practice_alpha_profile", Change: "changed", Current: &currentAlpha, Candidate: &candidateAlpha},
		{Profile: "practice_beta_profile", Change: "removed", Current: &currentBeta},
		{Profile: "practice_gamma_profile", Change: "added", Candidate: &candidateGamma},
	}
	if !reflect.DeepEqual(preview.Deltas.CombatProfiles, want) {
		t.Fatalf("unexpected combat-profile import preview deltas:\n got: %#v\nwant: %#v", preview.Deltas.CombatProfiles, want)
	}
}

func TestBuildImportPreviewReturnsItemTemplateDeltas(t *testing.T) {
	currentRedPotion := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}
	currentWoodenSword := itemcatalog.Template{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1}
	candidateRedPotion := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 7}
	candidateBluePotion := itemcatalog.Template{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 6}

	preview, err := BuildImportPreview(
		Bundle{
			ItemTemplates: []itemcatalog.Template{currentWoodenSword, currentRedPotion},
			InteractionDefinitions: []interactionstore.Definition{{
				Kind:  interactionstore.KindShopPreview,
				Ref:   "npc:merchant",
				Title: "Village Merchant",
				Catalog: []interactionstore.MerchantCatalogEntry{
					{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
					{Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
				},
			}},
		},
		Bundle{
			ItemTemplates: []itemcatalog.Template{candidateBluePotion, candidateRedPotion},
			InteractionDefinitions: []interactionstore.Definition{{
				Kind:  interactionstore.KindShopPreview,
				Ref:   "npc:merchant",
				Title: "Village Merchant",
				Catalog: []interactionstore.MerchantCatalogEntry{
					{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
					{Slot: 1, ItemVnum: 27002, Price: 80, Count: 1},
				},
			}},
		},
	)
	if err != nil {
		t.Fatalf("build import preview item-template deltas: %v", err)
	}

	want := []ItemTemplateDelta{
		{Vnum: 11200, Change: "removed", Current: &currentWoodenSword},
		{Vnum: 27001, Change: "changed", Current: &currentRedPotion, Candidate: &candidateRedPotion},
		{Vnum: 27002, Change: "added", Candidate: &candidateBluePotion},
	}
	if !reflect.DeepEqual(preview.Deltas.ItemTemplates, want) {
		t.Fatalf("unexpected item-template import preview deltas:\n got: %#v\nwant: %#v", preview.Deltas.ItemTemplates, want)
	}
}

func TestBuildImportPreviewReturnsPerMapStaticActorDeltas(t *testing.T) {
	currentAlpha := StaticActor{Name: "AlphaGuide", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300}
	currentKeep := StaticActor{Name: "KeepGuide", MapIndex: 42, X: 1710, Y: 2810, RaceNum: 20301}
	currentRemote := StaticActor{Name: "RemoteOld", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 20302}
	candidateBeta := StaticActor{Name: "BetaGuide", MapIndex: 42, X: 1720, Y: 2820, RaceNum: 20303}
	candidateRemote := StaticActor{Name: "RemoteNew", MapIndex: 8, X: 1400, Y: 2400, RaceNum: 20304}

	preview, err := BuildImportPreview(
		Bundle{StaticActors: []StaticActor{currentRemote, currentKeep, currentAlpha}},
		Bundle{StaticActors: []StaticActor{candidateRemote, currentKeep, candidateBeta}},
	)
	if err != nil {
		t.Fatalf("build import preview per-map static actor deltas: %v", err)
	}

	want := []MapContentDelta{
		{
			MapIndex:         7,
			StaticActorCount: SummaryCountDelta{Current: 1, Candidate: 0, Delta: -1},
			StaticActors:     []StaticActorDelta{{Change: "removed", Current: &currentRemote}},
		},
		{
			MapIndex:         8,
			StaticActorCount: SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
			StaticActors:     []StaticActorDelta{{Change: "added", Candidate: &candidateRemote}},
		},
		{
			MapIndex:         42,
			StaticActorCount: SummaryCountDelta{Current: 2, Candidate: 2, Delta: 0},
			StaticActors: []StaticActorDelta{
				{Change: "removed", Current: &currentAlpha},
				{Change: "added", Candidate: &candidateBeta},
			},
		},
	}
	if !reflect.DeepEqual(preview.Deltas.Maps, want) {
		t.Fatalf("unexpected per-map static-actor import preview deltas:\n got: %#v\nwant: %#v", preview.Deltas.Maps, want)
	}
}

func TestBuildImportPreviewReturnsPerMapSpawnGroupDeltas(t *testing.T) {
	currentKeep := SpawnGroupReferenceSummary{Ref: "practice.keep", Name: "Keep Mob", MapIndex: 42, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob}
	currentRemoved := SpawnGroupReferenceSummary{Ref: "practice.remove", Name: "Removed Mob", MapIndex: 42, X: 1100, Y: 2100, RaceNum: 102, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob}
	candidateAdded := SpawnGroupReferenceSummary{Ref: "practice.add", Name: "Added Mob", MapIndex: 42, X: 1300, Y: 2300, RaceNum: 103, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob}
	candidateKeep := SpawnGroupReferenceSummary{Ref: "practice.keep", Name: "Keep Mob", MapIndex: 42, X: 1200, Y: 2200, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob}

	preview, err := BuildImportPreview(
		Bundle{SpawnGroups: []SpawnGroup{
			{Ref: currentKeep.Ref, Name: currentKeep.Name, MapIndex: currentKeep.MapIndex, X: currentKeep.X, Y: currentKeep.Y, RaceNum: currentKeep.RaceNum, CombatProfile: currentKeep.CombatProfile},
			{Ref: currentRemoved.Ref, Name: currentRemoved.Name, MapIndex: currentRemoved.MapIndex, X: currentRemoved.X, Y: currentRemoved.Y, RaceNum: currentRemoved.RaceNum, CombatProfile: currentRemoved.CombatProfile},
		}},
		Bundle{SpawnGroups: []SpawnGroup{
			{Ref: candidateAdded.Ref, Name: candidateAdded.Name, MapIndex: candidateAdded.MapIndex, X: candidateAdded.X, Y: candidateAdded.Y, RaceNum: candidateAdded.RaceNum, CombatProfile: candidateAdded.CombatProfile},
			{Ref: candidateKeep.Ref, Name: candidateKeep.Name, MapIndex: candidateKeep.MapIndex, X: candidateKeep.X, Y: candidateKeep.Y, RaceNum: candidateKeep.RaceNum, CombatProfile: candidateKeep.CombatProfile},
		}},
	)
	if err != nil {
		t.Fatalf("build import preview per-map spawn-group deltas: %v", err)
	}

	want := []MapContentDelta{{
		MapIndex:        42,
		SpawnGroupCount: SummaryCountDelta{Current: 2, Candidate: 2, Delta: 0},
		SpawnGroups: []SpawnGroupDelta{
			{Ref: "practice.add", Change: "added", Candidate: &candidateAdded},
			{Ref: "practice.keep", Change: "changed", Current: &currentKeep, Candidate: &candidateKeep},
			{Ref: "practice.remove", Change: "removed", Current: &currentRemoved},
		},
	}}
	if !reflect.DeepEqual(preview.Deltas.Maps, want) {
		t.Fatalf("unexpected per-map spawn-group import preview deltas:\n got: %#v\nwant: %#v", preview.Deltas.Maps, want)
	}
}

func TestBuildImportPreviewReturnsPerMapCountDeltas(t *testing.T) {
	preview, err := BuildImportPreview(
		Bundle{
			StaticActors: []StaticActor{
				{Name: "VillageGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"},
			},
			InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Welcome."}},
		},
		Bundle{
			StaticActors: []StaticActor{
				{Name: "Merchant", MapIndex: 1, X: 1200, Y: 2200, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"},
				{Name: "Teleporter", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 20303, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:teleporter"},
			},
			SpawnGroups:   []SpawnGroup{{Ref: "practice.reward_mob", Name: "Reward Mob", MapIndex: 7, X: 1400, Y: 2400, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardDropVnums: []uint32{27001}}},
			ItemTemplates: testMerchantItemTemplates(),
			InteractionDefinitions: []interactionstore.Definition{
				testMerchantCatalogDefinition(),
				{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 7, X: 1300, Y: 2300},
			},
		},
	)
	if err != nil {
		t.Fatalf("build import preview with map deltas: %v", err)
	}
	want := []MapContentDelta{
		{
			MapIndex:                     1,
			StaticActorCount:             SummaryCountDelta{Current: 1, Candidate: 1, Delta: 0},
			InteractableStaticActorCount: SummaryCountDelta{Current: 1, Candidate: 1, Delta: 0},
			TalkActorCount:               SummaryCountDelta{Current: 1, Candidate: 0, Delta: -1},
			ShopPreviewActorCount:        SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
			ShopCatalogEntryCount:        SummaryCountDelta{Current: 0, Candidate: 2, Delta: 2},
			StaticActors: []StaticActorDelta{
				{Change: "added", Candidate: &StaticActor{Name: "Merchant", MapIndex: 1, X: 1200, Y: 2200, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"}},
				{Change: "removed", Current: &StaticActor{Name: "VillageGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"}},
			},
		},
		{
			MapIndex:                     7,
			StaticActorCount:             SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
			InteractableStaticActorCount: SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
			WarpActorCount:               SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
			SpawnGroupCount:              SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
			RewardDropItemCount:          SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
			StaticActors:                 []StaticActorDelta{{Change: "added", Candidate: &StaticActor{Name: "Teleporter", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 20303, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:teleporter"}}},
			SpawnGroups:                  []SpawnGroupDelta{{Ref: "practice.reward_mob", Change: "added", Candidate: &SpawnGroupReferenceSummary{Ref: "practice.reward_mob", Name: "Reward Mob", MapIndex: 7, X: 1400, Y: 2400, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardDropVnums: []uint32{27001}, RewardDropItems: []RewardDropItemSummary{{ItemVnum: 27001, ItemName: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}}}}},
		},
	}
	if !reflect.DeepEqual(preview.Deltas.Maps, want) {
		t.Fatalf("unexpected per-map import preview deltas:\n got: %#v\nwant: %#v", preview.Deltas.Maps, want)
	}
}

func TestBuildImportPreviewReturnsRewardAmountDeltas(t *testing.T) {
	current := Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:              "practice.reward_mob",
			Name:             "Reward Mob",
			MapIndex:         42,
			X:                1785,
			Y:                2885,
			RaceNum:          101,
			CombatProfile:    worldruntime.StaticActorCombatProfilePracticeMob,
			RewardExperience: 75,
			RewardGold:       60,
		}},
	}
	candidate := Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:              "practice.reward_mob",
			Name:             "Reward Mob",
			MapIndex:         42,
			X:                1785,
			Y:                2885,
			RaceNum:          101,
			CombatProfile:    worldruntime.StaticActorCombatProfilePracticeMob,
			RewardExperience: 125,
			RewardGold:       90,
		}},
	}

	preview, err := BuildImportPreview(current, candidate)
	if err != nil {
		t.Fatalf("build import preview reward amount deltas: %v", err)
	}

	if preview.Deltas.RewardExperienceTotal != (SummaryAmountDelta{Current: 75, Candidate: 125, Delta: 50}) {
		t.Fatalf("unexpected reward experience delta: %+v", preview.Deltas.RewardExperienceTotal)
	}
	if preview.Deltas.RewardGoldTotal != (SummaryAmountDelta{Current: 60, Candidate: 90, Delta: 30}) {
		t.Fatalf("unexpected reward gold delta: %+v", preview.Deltas.RewardGoldTotal)
	}
	wantMaps := []MapContentDelta{{
		MapIndex:              42,
		SpawnGroupCount:       SummaryCountDelta{Current: 1, Candidate: 1, Delta: 0},
		RewardExperienceTotal: SummaryAmountDelta{Current: 75, Candidate: 125, Delta: 50},
		RewardGoldTotal:       SummaryAmountDelta{Current: 60, Candidate: 90, Delta: 30},
		SpawnGroups: []SpawnGroupDelta{{
			Ref:       "practice.reward_mob",
			Change:    "changed",
			Current:   &SpawnGroupReferenceSummary{Ref: "practice.reward_mob", Name: "Reward Mob", MapIndex: 42, X: 1785, Y: 2885, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 75, RewardGold: 60},
			Candidate: &SpawnGroupReferenceSummary{Ref: "practice.reward_mob", Name: "Reward Mob", MapIndex: 42, X: 1785, Y: 2885, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 125, RewardGold: 90},
		}},
	}}
	if !reflect.DeepEqual(preview.Deltas.Maps, wantMaps) {
		t.Fatalf("unexpected per-map reward amount deltas:\n got: %#v\nwant: %#v", preview.Deltas.Maps, wantMaps)
	}

	decreasePreview, err := BuildImportPreview(candidate, current)
	if err != nil {
		t.Fatalf("build import preview decreased reward amount deltas: %v", err)
	}
	if decreasePreview.Deltas.RewardExperienceTotal != (SummaryAmountDelta{Current: 125, Candidate: 75, Delta: -50}) {
		t.Fatalf("unexpected decreased reward experience delta: %+v", decreasePreview.Deltas.RewardExperienceTotal)
	}
	if decreasePreview.Deltas.RewardGoldTotal != (SummaryAmountDelta{Current: 90, Candidate: 60, Delta: -30}) {
		t.Fatalf("unexpected decreased reward gold delta: %+v", decreasePreview.Deltas.RewardGoldTotal)
	}
}

func TestBuildImportPreviewReturnsRewardDropDeltas(t *testing.T) {
	redPotion := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}
	bluePotion := itemcatalog.Template{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 7}
	greenPotion := itemcatalog.Template{Vnum: 27003, Name: "Small Green Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 9}

	preview, err := BuildImportPreview(
		Bundle{
			ItemTemplates: []itemcatalog.Template{redPotion, bluePotion},
			SpawnGroups: []SpawnGroup{
				{Ref: "practice.red", Name: "Red Drop Mob", MapIndex: 42, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardDropVnums: []uint32{27001}},
				{Ref: "practice.blue", Name: "Blue Drop Mob", MapIndex: 42, X: 1100, Y: 2100, RaceNum: 102, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardDropVnums: []uint32{27002}},
			},
		},
		Bundle{
			ItemTemplates: []itemcatalog.Template{redPotion, greenPotion},
			SpawnGroups: []SpawnGroup{
				{Ref: "practice.red", Name: "Red Drop Mob", MapIndex: 42, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardDropVnums: []uint32{27001}},
				{Ref: "practice.red_bonus", Name: "Bonus Red Drop Mob", MapIndex: 42, X: 1200, Y: 2200, RaceNum: 103, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardDropVnums: []uint32{27001}},
				{Ref: "practice.green", Name: "Green Drop Mob", MapIndex: 42, X: 1300, Y: 2300, RaceNum: 104, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardDropVnums: []uint32{27003}},
			},
		},
	)
	if err != nil {
		t.Fatalf("build import preview reward-drop deltas: %v", err)
	}

	currentRed := RewardDropAggregateSummary{ItemVnum: 27001, ItemName: "Small Red Potion", SourceCount: 1, Stackable: true, MaxCount: 200, ShopBuyPrice: 5}
	candidateRed := RewardDropAggregateSummary{ItemVnum: 27001, ItemName: "Small Red Potion", SourceCount: 2, Stackable: true, MaxCount: 200, ShopBuyPrice: 5}
	currentBlue := RewardDropAggregateSummary{ItemVnum: 27002, ItemName: "Small Blue Potion", SourceCount: 1, Stackable: true, MaxCount: 200, ShopBuyPrice: 7}
	candidateGreen := RewardDropAggregateSummary{ItemVnum: 27003, ItemName: "Small Green Potion", SourceCount: 1, Stackable: true, MaxCount: 200, ShopBuyPrice: 9}
	want := []RewardDropDelta{
		{ItemVnum: 27001, Change: "changed", Current: &currentRed, Candidate: &candidateRed},
		{ItemVnum: 27002, Change: "removed", Current: &currentBlue},
		{ItemVnum: 27003, Change: "added", Candidate: &candidateGreen},
	}
	if !reflect.DeepEqual(preview.Deltas.RewardDrops, want) {
		t.Fatalf("unexpected reward-drop import preview deltas:\n got: %#v\nwant: %#v", preview.Deltas.RewardDrops, want)
	}
}

func TestBuildImportPreviewReturnsSpawnGroupDeltas(t *testing.T) {
	redPotion := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}
	bluePotion := itemcatalog.Template{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 7}
	currentKeep := SpawnGroupReferenceSummary{Ref: "practice.keep", Name: "Keep Mob", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 10, RewardGold: 5, RewardDropVnums: []uint32{27001}, RewardDropItems: []RewardDropItemSummary{{ItemVnum: 27001, ItemName: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}}}
	currentRemoved := SpawnGroupReferenceSummary{Ref: "practice.remove", Name: "Removed Mob", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 102, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 3, RewardGold: 1}
	candidateAdded := SpawnGroupReferenceSummary{Ref: "practice.add", Name: "Added Mob", MapIndex: 2, X: 1300, Y: 2300, RaceNum: 103, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 7, RewardGold: 2, RewardDropVnums: []uint32{27002}, RewardDropItems: []RewardDropItemSummary{{ItemVnum: 27002, ItemName: "Small Blue Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 7}}}
	candidateKeep := SpawnGroupReferenceSummary{Ref: "practice.keep", Name: "Keep Mob", MapIndex: 1, X: 1200, Y: 2200, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 20, RewardGold: 8, RewardDropVnums: []uint32{27001}, RewardDropItems: []RewardDropItemSummary{{ItemVnum: 27001, ItemName: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}}}

	preview, err := BuildImportPreview(
		Bundle{
			ItemTemplates: []itemcatalog.Template{redPotion},
			SpawnGroups: []SpawnGroup{
				{Ref: currentKeep.Ref, Name: currentKeep.Name, MapIndex: currentKeep.MapIndex, X: currentKeep.X, Y: currentKeep.Y, RaceNum: currentKeep.RaceNum, CombatProfile: currentKeep.CombatProfile, RewardExperience: currentKeep.RewardExperience, RewardGold: currentKeep.RewardGold, RewardDropVnums: currentKeep.RewardDropVnums},
				{Ref: currentRemoved.Ref, Name: currentRemoved.Name, MapIndex: currentRemoved.MapIndex, X: currentRemoved.X, Y: currentRemoved.Y, RaceNum: currentRemoved.RaceNum, CombatProfile: currentRemoved.CombatProfile, RewardExperience: currentRemoved.RewardExperience, RewardGold: currentRemoved.RewardGold},
			},
		},
		Bundle{
			ItemTemplates: []itemcatalog.Template{redPotion, bluePotion},
			SpawnGroups: []SpawnGroup{
				{Ref: candidateAdded.Ref, Name: candidateAdded.Name, MapIndex: candidateAdded.MapIndex, X: candidateAdded.X, Y: candidateAdded.Y, RaceNum: candidateAdded.RaceNum, CombatProfile: candidateAdded.CombatProfile, RewardExperience: candidateAdded.RewardExperience, RewardGold: candidateAdded.RewardGold, RewardDropVnums: candidateAdded.RewardDropVnums},
				{Ref: candidateKeep.Ref, Name: candidateKeep.Name, MapIndex: candidateKeep.MapIndex, X: candidateKeep.X, Y: candidateKeep.Y, RaceNum: candidateKeep.RaceNum, CombatProfile: candidateKeep.CombatProfile, RewardExperience: candidateKeep.RewardExperience, RewardGold: candidateKeep.RewardGold, RewardDropVnums: candidateKeep.RewardDropVnums},
			},
		},
	)
	if err != nil {
		t.Fatalf("build import preview spawn-group deltas: %v", err)
	}

	want := []SpawnGroupDelta{
		{Ref: "practice.add", Change: "added", Candidate: &candidateAdded},
		{Ref: "practice.keep", Change: "changed", Current: &currentKeep, Candidate: &candidateKeep},
		{Ref: "practice.remove", Change: "removed", Current: &currentRemoved},
	}
	if !reflect.DeepEqual(preview.Deltas.SpawnGroups, want) {
		t.Fatalf("unexpected spawn-group import preview deltas:\n got: %#v\nwant: %#v", preview.Deltas.SpawnGroups, want)
	}
}

func TestBuildImportPreviewReturnsCombatProfileDeltas(t *testing.T) {
	currentKeepProfile := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_keep_profile", MaxHP: 24, DamagePerNormalAttack: 3, AttackValue: 7, DefenseValue: 4, Level: 2, Rank: 1, RespawnDelayMs: 1500}
	currentRemovedProfile := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_remove_profile", MaxHP: 20, DamagePerNormalAttack: 2, AttackValue: 6, DefenseValue: 4, Level: 1, RespawnDelayMs: 1500}
	candidateAddedProfile := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_add_profile", MaxHP: 30, DamagePerNormalAttack: 4, AttackValue: 8, DefenseValue: 4, Level: 3, Rank: 1, RespawnDelayMs: 2000}
	candidateKeepProfile := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_keep_profile", MaxHP: 28, DamagePerNormalAttack: 3, AttackValue: 7, DefenseValue: 4, Level: 2, Rank: 1, RespawnDelayMs: 1500}

	preview, err := BuildImportPreview(
		Bundle{
			CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{currentRemovedProfile, currentKeepProfile},
			SpawnGroups: []SpawnGroup{
				{Ref: "practice.keep", Name: "Keep Mob", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: currentKeepProfile.Profile},
				{Ref: "practice.remove", Name: "Removed Mob", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 102, CombatProfile: currentRemovedProfile.Profile},
			},
		},
		Bundle{
			CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{candidateKeepProfile, candidateAddedProfile},
			SpawnGroups: []SpawnGroup{
				{Ref: "practice.add", Name: "Added Mob", MapIndex: 2, X: 1300, Y: 2300, RaceNum: 103, CombatProfile: candidateAddedProfile.Profile},
				{Ref: "practice.keep", Name: "Keep Mob", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: candidateKeepProfile.Profile},
			},
		},
	)
	if err != nil {
		t.Fatalf("build import preview combat-profile deltas: %v", err)
	}

	want := []CombatProfileDelta{
		{Profile: "practice_add_profile", Change: "added", Candidate: &candidateAddedProfile},
		{Profile: "practice_keep_profile", Change: "changed", Current: &currentKeepProfile, Candidate: &candidateKeepProfile},
		{Profile: "practice_remove_profile", Change: "removed", Current: &currentRemovedProfile},
	}
	if !reflect.DeepEqual(preview.Deltas.CombatProfiles, want) {
		t.Fatalf("unexpected combat-profile import preview deltas:\n got: %#v\nwant: %#v", preview.Deltas.CombatProfiles, want)
	}
}

func TestSummarizeReturnsDeterministicStaticActorDetails(t *testing.T) {
	summary, err := Summarize(Bundle{
		StaticActors: []StaticActor{
			{Name: "TrainingDummy", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 20350, CombatProfile: " training_dummy "},
			{Name: "  VillageGuide  ", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: " talk ", InteractionRef: " npc:guide "},
			{Name: "Blacksmith", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 20300},
		},
		InteractionDefinitions: []interactionstore.Definition{
			{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Welcome."},
		},
	})
	if err != nil {
		t.Fatalf("summarize static actor details: %v", err)
	}
	want := []StaticActor{
		{Name: "Blacksmith", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 20300},
		{Name: "TrainingDummy", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy},
		{Name: "VillageGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"},
	}
	if !reflect.DeepEqual(summary.StaticActors, want) {
		t.Fatalf("unexpected static actor summaries:\n got: %#v\nwant: %#v", summary.StaticActors, want)
	}
}

func TestSummarizeReturnsDeterministicShopCatalogDetails(t *testing.T) {
	summary, err := Summarize(Bundle{
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27002, Name: " Small Blue Potion ", Stackable: true, MaxCount: 200, ShopBuyPrice: 7},
			{Vnum: 11200, Name: " Wooden Sword ", Stackable: false, MaxCount: 1},
			{Vnum: 27001, Name: " Small Red Potion ", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		},
		InteractionDefinitions: []interactionstore.Definition{
			{
				Kind:  interactionstore.KindShopPreview,
				Ref:   "npc:potion_merchant",
				Title: "Potion Merchant",
				Catalog: []interactionstore.MerchantCatalogEntry{
					{Slot: 1, ItemVnum: 27002, Price: 80, Count: 2},
					{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
				},
			},
			{
				Kind:  interactionstore.KindShopPreview,
				Ref:   "npc:arms_merchant",
				Title: "Arms Merchant",
				Catalog: []interactionstore.MerchantCatalogEntry{
					{Slot: 0, ItemVnum: 11200, Price: 500, Count: 1},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("summarize shop catalog details: %v", err)
	}
	want := []ShopCatalogSummary{
		{
			Kind:       interactionstore.KindShopPreview,
			Ref:        "npc:arms_merchant",
			Title:      "Arms Merchant",
			EntryCount: 1,
			Entries: []ShopCatalogEntrySummary{
				{Slot: 0, ItemVnum: 11200, ItemName: "Wooden Sword", Count: 1, Price: 500, Stackable: false, MaxCount: 1},
			},
		},
		{
			Kind:       interactionstore.KindShopPreview,
			Ref:        "npc:potion_merchant",
			Title:      "Potion Merchant",
			EntryCount: 2,
			Entries: []ShopCatalogEntrySummary{
				{Slot: 0, ItemVnum: 27001, ItemName: "Small Red Potion", Count: 1, Price: 50, Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
				{Slot: 1, ItemVnum: 27002, ItemName: "Small Blue Potion", Count: 2, Price: 80, Stackable: true, MaxCount: 200, ShopBuyPrice: 7},
			},
		},
	}
	if !reflect.DeepEqual(summary.ShopCatalogs, want) {
		t.Fatalf("unexpected shop catalog summaries:\n got: %#v\nwant: %#v", summary.ShopCatalogs, want)
	}
}

func TestSummarizeReturnsDeterministicShopRoutesForInteractableActors(t *testing.T) {
	summary, err := Summarize(Bundle{
		StaticActors: []StaticActor{
			{Name: "PotionMerchant", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:potion_merchant"},
			{Name: "ArmsMerchant", MapIndex: 3, X: 1200, Y: 2200, RaceNum: 20302, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:arms_merchant"},
		},
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 7},
			{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1},
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		},
		InteractionDefinitions: []interactionstore.Definition{
			{
				Kind:  interactionstore.KindShopPreview,
				Ref:   "npc:potion_merchant",
				Title: "Potion Merchant",
				Catalog: []interactionstore.MerchantCatalogEntry{
					{Slot: 1, ItemVnum: 27002, Price: 80, Count: 2},
					{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
				},
			},
			{
				Kind:  interactionstore.KindShopPreview,
				Ref:   "npc:arms_merchant",
				Title: "Arms Merchant",
				Catalog: []interactionstore.MerchantCatalogEntry{
					{Slot: 0, ItemVnum: 11200, Price: 500, Count: 1},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("summarize shop routes: %v", err)
	}
	want := []ShopRouteSummary{
		{ActorName: "ArmsMerchant", SourceMapIndex: 3, SourceX: 1200, SourceY: 2200, Ref: "npc:arms_merchant", Title: "Arms Merchant", EntryCount: 1},
		{ActorName: "PotionMerchant", SourceMapIndex: 1, SourceX: 1100, SourceY: 2100, Ref: "npc:potion_merchant", Title: "Potion Merchant", EntryCount: 2},
	}
	if summary.ShopRouteCount != len(want) {
		t.Fatalf("expected %d shop routes, got %d", len(want), summary.ShopRouteCount)
	}
	if !reflect.DeepEqual(summary.ShopRoutes, want) {
		t.Fatalf("unexpected shop route summaries:\n got: %#v\nwant: %#v", summary.ShopRoutes, want)
	}
}

func TestCanonicalizeRejectsShopCatalogEntriesThatDoNotFitShopStartCarriers(t *testing.T) {
	cases := []struct {
		name  string
		entry interactionstore.MerchantCatalogEntry
	}{
		{name: "over uint32 price", entry: interactionstore.MerchantCatalogEntry{Slot: 0, ItemVnum: 27001, Price: interactionstore.MerchantCatalogMaxEntryPrice + 1, Count: 1}},
		{name: "over uint8 count", entry: interactionstore.MerchantCatalogEntry{Slot: 0, ItemVnum: 27001, Price: 50, Count: interactionstore.MerchantCatalogMaxEntryCount + 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Canonicalize(Bundle{
				ItemTemplates: []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 255, ShopBuyPrice: 5}},
				InteractionDefinitions: []interactionstore.Definition{{
					Kind:    interactionstore.KindShopPreview,
					Ref:     "npc:merchant",
					Title:   "Village Merchant",
					Catalog: []interactionstore.MerchantCatalogEntry{tc.entry},
				}},
			})
			if !errors.Is(err, ErrInvalidBundle) {
				t.Fatalf("expected ErrInvalidBundle, got %v", err)
			}
		})
	}
}

func TestSummarizeReturnsDeterministicWarpDestinationDetails(t *testing.T) {
	summary, err := Summarize(Bundle{
		InteractionDefinitions: []interactionstore.Definition{
			{Kind: interactionstore.KindWarp, Ref: "npc:west_gate", Text: "Return to the west gate.", MapIndex: 7, X: 1300, Y: 2300},
			{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Welcome."},
			{Kind: interactionstore.KindWarp, Ref: "npc:east_gate", MapIndex: 3, X: 1200, Y: 2200},
		},
	})
	if err != nil {
		t.Fatalf("summarize warp destinations: %v", err)
	}
	want := []WarpDestinationSummary{
		{Kind: interactionstore.KindWarp, Ref: "npc:east_gate", MapIndex: 3, X: 1200, Y: 2200},
		{Kind: interactionstore.KindWarp, Ref: "npc:west_gate", MapIndex: 7, X: 1300, Y: 2300, Text: "Return to the west gate."},
	}
	if summary.WarpDestinationCount != len(want) {
		t.Fatalf("expected %d warp destinations, got %d", len(want), summary.WarpDestinationCount)
	}
	if !reflect.DeepEqual(summary.WarpDestinations, want) {
		t.Fatalf("unexpected warp destination summaries:\n got: %#v\nwant: %#v", summary.WarpDestinations, want)
	}
}

func TestSummarizeReturnsDeterministicWarpRoutesForInteractableActors(t *testing.T) {
	summary, err := Summarize(Bundle{
		StaticActors: []StaticActor{
			{Name: "EastTeleporter", MapIndex: 1, X: 1300, Y: 2300, RaceNum: 20300, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:east_gate"},
			{Name: "WestTeleporter", MapIndex: 3, X: 1200, Y: 2200, RaceNum: 20301, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:west_gate"},
		},
		InteractionDefinitions: []interactionstore.Definition{
			{Kind: interactionstore.KindWarp, Ref: "npc:west_gate", Text: "Return to the west gate.", MapIndex: 7, X: 1300, Y: 2300},
			{Kind: interactionstore.KindWarp, Ref: "npc:east_gate", MapIndex: 3, X: 1200, Y: 2200},
		},
	})
	if err != nil {
		t.Fatalf("summarize warp routes: %v", err)
	}
	want := []WarpRouteSummary{
		{ActorName: "EastTeleporter", SourceMapIndex: 1, SourceX: 1300, SourceY: 2300, Ref: "npc:east_gate", TargetMapIndex: 3, TargetX: 1200, TargetY: 2200},
		{ActorName: "WestTeleporter", SourceMapIndex: 3, SourceX: 1200, SourceY: 2200, Ref: "npc:west_gate", TargetMapIndex: 7, TargetX: 1300, TargetY: 2300, Text: "Return to the west gate."},
	}
	if summary.WarpRouteCount != len(want) {
		t.Fatalf("expected %d warp routes, got %d", len(want), summary.WarpRouteCount)
	}
	if !reflect.DeepEqual(summary.WarpRoutes, want) {
		t.Fatalf("unexpected warp route summaries:\n got: %#v\nwant: %#v", summary.WarpRoutes, want)
	}
}

func TestSummarizeAuditsServiceInteractionsPerMap(t *testing.T) {
	summary, err := Summarize(Bundle{
		StaticActors: []StaticActor{
			{Name: "Merchant", MapIndex: 1, X: 1200, Y: 2200, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"},
			{Name: "MerchantAssistant", MapIndex: 1, X: 1250, Y: 2250, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"},
			{Name: "Teleporter", MapIndex: 1, X: 1300, Y: 2300, RaceNum: 20303, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:teleporter"},
		},
		ItemTemplates: testMerchantItemTemplates(),
		InteractionDefinitions: []interactionstore.Definition{
			testMerchantCatalogDefinition(),
			{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 7, X: 1300, Y: 2300},
		},
	})
	if err != nil {
		t.Fatalf("summarize per-map service interactions: %v", err)
	}
	want := []MapContentSummary{{MapIndex: 1, StaticActorCount: 3, InteractableStaticActorCount: 3, ShopPreviewActorCount: 2, ShopCatalogEntryCount: 4, WarpActorCount: 1}}
	if !reflect.DeepEqual(summary.Maps, want) {
		t.Fatalf("unexpected per-map service interaction audit:\n got: %#v\nwant: %#v", summary.Maps, want)
	}
}

func TestSummarizeAuditsSelfOnlyInteractionKindsPerMap(t *testing.T) {
	summary, err := Summarize(Bundle{
		StaticActors: []StaticActor{
			{Name: "NoticeBoard", MapIndex: 1, X: 900, Y: 1900, RaceNum: 20304, InteractionKind: interactionstore.KindInfo, InteractionRef: "lore:notice_board"},
			{Name: "VillageGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guide"},
			{Name: "RemoteGuide", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:remote_guide"},
		},
		InteractionDefinitions: []interactionstore.Definition{
			{Kind: interactionstore.KindInfo, Ref: "lore:notice_board", Text: "Read the notices."},
			{Kind: interactionstore.KindTalk, Ref: "npc:remote_guide", Text: "The road is quiet."},
			{Kind: interactionstore.KindTalk, Ref: "npc:village_guide", Text: "Welcome."},
		},
	})
	if err != nil {
		t.Fatalf("summarize per-map self-only interactions: %v", err)
	}
	want := []MapContentSummary{
		{MapIndex: 1, StaticActorCount: 2, InteractableStaticActorCount: 2, InfoActorCount: 1, TalkActorCount: 1},
		{MapIndex: 7, StaticActorCount: 1, InteractableStaticActorCount: 1, TalkActorCount: 1},
	}
	if !reflect.DeepEqual(summary.Maps, want) {
		t.Fatalf("unexpected per-map self-only interaction audit:\n got: %#v\nwant: %#v", summary.Maps, want)
	}
}

func TestSummarizeReturnsDeterministicInteractionDefinitionPreviews(t *testing.T) {
	summary, err := Summarize(Bundle{
		ItemTemplates: testMerchantItemTemplates(),
		InteractionDefinitions: []interactionstore.Definition{
			{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Keep your blade sharp."},
			{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 7, X: 1300, Y: 2300},
			testMerchantCatalogDefinition(),
			{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
		},
	})
	if err != nil {
		t.Fatalf("summarize interaction definition previews: %v", err)
	}
	want := []InteractionDefinitionPreviewSummary{
		{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Preview: "The alchemist studies forgotten herbs."},
		{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Preview: "Village Merchant: [0] Small Red Potion x1 @ 50g; [1] Wooden Sword x1 @ 500g"},
		{Kind: interactionstore.KindTalk, Ref: "npc:guide", Preview: "Keep your blade sharp."},
		{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", Preview: "Step through the gate. [warp -> map 7 @ 1300,2300]"},
	}
	if !reflect.DeepEqual(summary.InteractionDefinitionPreviews, want) {
		t.Fatalf("unexpected interaction definition previews:\n got: %#v\nwant: %#v", summary.InteractionDefinitionPreviews, want)
	}
}

func TestSummarizeCompactsLongInteractionDefinitionPreviews(t *testing.T) {
	longText := strings.Repeat("B", 200)
	summary, err := Summarize(Bundle{
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindInfo, Ref: "lore:notice_board", Text: longText}},
	})
	if err != nil {
		t.Fatalf("summarize long interaction definition preview: %v", err)
	}
	if len(summary.InteractionDefinitionPreviews) != 1 {
		t.Fatalf("expected one interaction definition preview, got %+v", summary.InteractionDefinitionPreviews)
	}
	want := strings.Repeat("B", 157) + "..."
	if summary.InteractionDefinitionPreviews[0].Preview != want {
		t.Fatalf("unexpected compact interaction preview length=%d preview=%q", len(summary.InteractionDefinitionPreviews[0].Preview), summary.InteractionDefinitionPreviews[0].Preview)
	}
}

func TestSummarizeReturnsDeterministicInteractableStaticActorDetails(t *testing.T) {
	summary, err := Summarize(Bundle{
		StaticActors: []StaticActor{
			{Name: "VillageGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"},
			{Name: "Merchant", MapIndex: 1, X: 1200, Y: 2200, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"},
			{Name: "Blacksmith", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 20300},
			{Name: "Teleporter", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 20303, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:teleporter"},
			{Name: "Alchemist", MapIndex: 1, X: 900, Y: 1900, RaceNum: 20304, InteractionKind: interactionstore.KindInfo, InteractionRef: "lore:alchemist"},
		},
		ItemTemplates: testMerchantItemTemplates(),
		InteractionDefinitions: []interactionstore.Definition{
			{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
			testMerchantCatalogDefinition(),
			{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Welcome."},
			{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 7, X: 1300, Y: 2300},
		},
	})
	if err != nil {
		t.Fatalf("summarize interactable static actors: %v", err)
	}
	want := []InteractableStaticActorSummary{
		{Name: "Alchemist", MapIndex: 1, X: 900, Y: 1900, RaceNum: 20304, InteractionKind: interactionstore.KindInfo, InteractionRef: "lore:alchemist", Preview: "The alchemist studies forgotten herbs."},
		{Name: "Merchant", MapIndex: 1, X: 1200, Y: 2200, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant", Preview: "Village Merchant: [0] Small Red Potion x1 @ 50g; [1] Wooden Sword x1 @ 500g"},
		{Name: "Teleporter", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 20303, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:teleporter", Preview: "Step through the gate. [warp -> map 7 @ 1300,2300]"},
		{Name: "VillageGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide", Preview: "VillageGuide:\nWelcome."},
	}
	if !reflect.DeepEqual(summary.InteractableStaticActors, want) {
		t.Fatalf("unexpected interactable static actor summaries:\n got: %#v\nwant: %#v", summary.InteractableStaticActors, want)
	}
}

func TestSummarizeCompactsLongInteractableStaticActorPreviews(t *testing.T) {
	longText := strings.Repeat("A", 200)
	summary, err := Summarize(Bundle{
		StaticActors:           []StaticActor{{Name: "NoticeBoard", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindInfo, InteractionRef: "lore:notice_board"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindInfo, Ref: "lore:notice_board", Text: longText}},
	})
	if err != nil {
		t.Fatalf("summarize long-preview interactable static actor: %v", err)
	}
	if len(summary.InteractableStaticActors) != 1 {
		t.Fatalf("expected one interactable static actor summary, got %+v", summary.InteractableStaticActors)
	}
	want := strings.Repeat("A", 157) + "..."
	if summary.InteractableStaticActors[0].Preview != want {
		t.Fatalf("unexpected compact preview length=%d preview=%q", len(summary.InteractableStaticActors[0].Preview), summary.InteractableStaticActors[0].Preview)
	}
}

func TestSummarizeReturnsDeterministicSpawnGroupReferences(t *testing.T) {
	summary, err := Summarize(Bundle{
		SpawnGroups: []SpawnGroup{
			{Ref: "practice.beta", Name: "Beta Mob", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 102, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 25, RewardGold: 5, RewardDropVnums: []uint32{27002, 27001}},
			{Ref: "practice.alpha", Name: "Alpha Mob", MapIndex: 3, X: 1200, Y: 2200, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy},
		},
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
		},
	})
	if err != nil {
		t.Fatalf("summarize spawn-group references: %v", err)
	}
	want := []SpawnGroupReferenceSummary{
		{Ref: "practice.alpha", Name: "Alpha Mob", MapIndex: 3, X: 1200, Y: 2200, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy},
		{Ref: "practice.beta", Name: "Beta Mob", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 102, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 25, RewardGold: 5, RewardDropVnums: []uint32{27001, 27002}, RewardDropItems: []RewardDropItemSummary{
			{ItemVnum: 27001, ItemName: "Small Red Potion", Stackable: true, MaxCount: 200},
			{ItemVnum: 27002, ItemName: "Small Blue Potion", Stackable: true, MaxCount: 200},
		}},
	}
	if !reflect.DeepEqual(summary.SpawnGroups, want) {
		t.Fatalf("unexpected spawn-group summaries:\n got: %#v\nwant: %#v", summary.SpawnGroups, want)
	}
}

func TestSummarizeReturnsDeterministicSpawnRewardDropItemDetails(t *testing.T) {
	summary, err := Summarize(Bundle{
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27002, Name: " Small Blue Potion ", Stackable: true, MaxCount: 200, ShopBuyPrice: 7},
			{Vnum: 27001, Name: " Small Red Potion ", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		},
		SpawnGroups: []SpawnGroup{{
			Ref:             "practice.reward_mob",
			Name:            "Reward Mob",
			MapIndex:        42,
			X:               1785,
			Y:               2885,
			RaceNum:         101,
			CombatProfile:   worldruntime.StaticActorCombatProfilePracticeMob,
			RewardDropVnums: []uint32{27002, 27001},
		}},
	})
	if err != nil {
		t.Fatalf("summarize spawn reward drop item details: %v", err)
	}
	want := []RewardDropItemSummary{
		{ItemVnum: 27001, ItemName: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		{ItemVnum: 27002, ItemName: "Small Blue Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 7},
	}
	if len(summary.SpawnGroups) != 1 {
		t.Fatalf("expected one spawn-group summary, got %+v", summary.SpawnGroups)
	}
	if !reflect.DeepEqual(summary.SpawnGroups[0].RewardDropItems, want) {
		t.Fatalf("unexpected reward drop item summaries:\n got: %#v\nwant: %#v", summary.SpawnGroups[0].RewardDropItems, want)
	}
}

func TestSummarizeReturnsDeterministicRewardTotalsAndDropAggregates(t *testing.T) {
	summary, err := Summarize(Bundle{
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27002, Name: " Small Blue Potion ", Stackable: true, MaxCount: 200, ShopBuyPrice: 7},
			{Vnum: 27001, Name: " Small Red Potion ", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		},
		SpawnGroups: []SpawnGroup{
			{
				Ref:              "practice.reward_alpha",
				Name:             "Reward Alpha",
				MapIndex:         42,
				X:                1785,
				Y:                2885,
				RaceNum:          101,
				CombatProfile:    worldruntime.StaticActorCombatProfilePracticeMob,
				RewardExperience: 25,
				RewardGold:       5,
				RewardDropVnums:  []uint32{27002, 27001},
			},
			{
				Ref:              "practice.reward_beta",
				Name:             "Reward Beta",
				MapIndex:         42,
				X:                1885,
				Y:                2985,
				RaceNum:          102,
				CombatProfile:    worldruntime.StaticActorCombatProfilePracticeMob,
				RewardExperience: 75,
				RewardGold:       60,
				RewardDropVnums:  []uint32{27001},
			},
		},
	})
	if err != nil {
		t.Fatalf("summarize reward totals and drop aggregates: %v", err)
	}
	if summary.RewardExperienceTotal != 100 {
		t.Fatalf("expected reward experience total 100, got %d", summary.RewardExperienceTotal)
	}
	if summary.RewardGoldTotal != 65 {
		t.Fatalf("expected reward gold total 65, got %d", summary.RewardGoldTotal)
	}
	if summary.RewardDropItemCount != 3 {
		t.Fatalf("expected reward drop item count 3, got %d", summary.RewardDropItemCount)
	}
	want := []RewardDropAggregateSummary{
		{ItemVnum: 27001, ItemName: "Small Red Potion", SourceCount: 2, Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		{ItemVnum: 27002, ItemName: "Small Blue Potion", SourceCount: 1, Stackable: true, MaxCount: 200, ShopBuyPrice: 7},
	}
	if !reflect.DeepEqual(summary.RewardDrops, want) {
		t.Fatalf("unexpected reward drop aggregates:\n got: %#v\nwant: %#v", summary.RewardDrops, want)
	}
	wantMaps := []MapContentSummary{{MapIndex: 42, SpawnGroupCount: 2, RewardExperienceTotal: 100, RewardGoldTotal: 65, RewardDropItemCount: 3}}
	if !reflect.DeepEqual(summary.Maps, wantMaps) {
		t.Fatalf("unexpected per-map reward audit:\n got: %#v\nwant: %#v", summary.Maps, wantMaps)
	}
}

func TestSummarizeReturnsDeterministicCombatProfileDetails(t *testing.T) {
	summary, err := Summarize(Bundle{
		SpawnGroups: []SpawnGroup{
			{Ref: "practice.beta", Name: "Beta Mob", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 102, CombatProfile: "practice_beta_profile"},
			{Ref: "practice.alpha", Name: "Alpha Mob", MapIndex: 3, X: 1200, Y: 2200, RaceNum: 101, CombatProfile: "practice_alpha_profile"},
		},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{
			{Profile: "practice_beta_profile", MaxHP: 30, DamagePerNormalAttack: 5, AttackValue: 8, DefenseValue: 3, Level: 6, Rank: 2, RespawnDelayMs: 2500, DeathReward: worldruntime.StaticActorDeathReward{Experience: 7, Gold: 3}},
			{Profile: "practice_alpha_profile", MaxHP: 24, DamagePerNormalAttack: 3, AttackValue: 7, DefenseValue: 4, Level: 4, Rank: 1, RespawnDelayMs: 1500},
		},
	})
	if err != nil {
		t.Fatalf("summarize combat-profile details: %v", err)
	}
	want := []worldruntime.StaticActorCombatProfileSnapshot{
		{Profile: "practice_alpha_profile", MaxHP: 24, DamagePerNormalAttack: 3, AttackValue: 7, DefenseValue: 4, Level: 4, Rank: 1, RespawnDelayMs: 1500},
		{Profile: "practice_beta_profile", MaxHP: 30, DamagePerNormalAttack: 5, AttackValue: 8, DefenseValue: 3, Level: 6, Rank: 2, RespawnDelayMs: 2500, DeathReward: worldruntime.StaticActorDeathReward{Experience: 7, Gold: 3}},
	}
	if !reflect.DeepEqual(summary.CombatProfiles, want) {
		t.Fatalf("unexpected combat-profile summaries:\n got: %#v\nwant: %#v", summary.CombatProfiles, want)
	}
}

func TestSummarizeRejectsInvalidBundle(t *testing.T) {
	_, err := Summarize(Bundle{StaticActors: []StaticActor{{Name: "BrokenActor", RaceNum: 20300}}})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for invalid summarized bundle, got %v", err)
	}
}

func TestBootstrapNPCServiceExampleBundleIsCanonicalAndValid(t *testing.T) {
	decoded := loadBootstrapNPCServiceExampleBundle(t)

	canonical, err := Canonicalize(decoded)
	if err != nil {
		t.Fatalf("canonicalize bootstrap NPC service example bundle: %v", err)
	}
	if !reflect.DeepEqual(canonical, decoded) {
		t.Fatalf("bootstrap NPC service example bundle is not canonical:\n got: %#v\nwant: %#v", decoded, canonical)
	}
}

func TestBootstrapNPCServiceExampleBundleCoversOwnedServiceInteractionKinds(t *testing.T) {
	decoded := loadBootstrapNPCServiceExampleBundle(t)
	wantKinds := map[string]struct{}{
		interactionstore.KindInfo:        {},
		interactionstore.KindTalk:        {},
		interactionstore.KindWarp:        {},
		interactionstore.KindShopPreview: {},
	}
	seenDefinitions := make(map[string]struct{}, len(decoded.InteractionDefinitions))
	for _, definition := range decoded.InteractionDefinitions {
		seenDefinitions[definition.Kind] = struct{}{}
	}
	seenActors := make(map[string]struct{}, len(decoded.StaticActors))
	for _, actor := range decoded.StaticActors {
		if actor.InteractionKind != "" {
			seenActors[actor.InteractionKind] = struct{}{}
		}
	}
	for kind := range wantKinds {
		if _, ok := seenDefinitions[kind]; !ok {
			t.Fatalf("bootstrap NPC service example lacks %q interaction definition", kind)
		}
		if _, ok := seenActors[kind]; !ok {
			t.Fatalf("bootstrap NPC service example lacks %q static actor", kind)
		}
	}
}

func loadBootstrapNPCServiceExampleBundle(t *testing.T) Bundle {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve contentbundle test path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	raw, err := os.ReadFile(filepath.Join(root, "docs", "examples", "bootstrap-npc-service-bundle.json"))
	if err != nil {
		t.Fatalf("read bootstrap NPC service example bundle: %v", err)
	}

	var decoded Bundle
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		t.Fatalf("decode bootstrap NPC service example bundle: %v", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		t.Fatal("bootstrap NPC service example bundle has trailing JSON")
	}
	return decoded
}

func TestFromSnapshotsBuildsDeterministicPortableBundle(t *testing.T) {
	const customProfile = "practice_snapshot_guard"
	if !worldruntime.RegisterStaticActorCombatProfile(customProfile, worldruntime.StaticActorCombatProfileDefaults{
		MaxHP:                 24,
		DamagePerNormalAttack: 4,
		AttackValue:           7,
		DefenseValue:          3,
		Level:                 8,
		Rank:                  2,
		RespawnDelay:          1500 * time.Millisecond,
		DeathReward:           worldruntime.StaticActorDeathReward{Experience: 12, Gold: 7, DropVnums: []uint32{27002, 27001}},
	}) {
		t.Fatalf("expected custom snapshot combat profile %q to register", customProfile)
	}
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(customProfile) })

	bundle, err := FromSnapshotsWithItems(
		staticstore.Snapshot{StaticActors: []staticstore.StaticActor{
			{EntityID: 9, Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"},
			{EntityID: 3, Name: "Blacksmith", MapIndex: 42, X: 1750, Y: 2850, RaceNum: 20301},
			{EntityID: 7, Name: "Merchant", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20302, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"},
			{EntityID: 5, Name: "TrainingDummy", MapIndex: 42, X: 1775, Y: 2875, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy},
			{EntityID: 15, Name: "SnapshotGuard", MapIndex: 42, X: 1780, Y: 2880, RaceNum: 102, CombatProfile: customProfile},
			{EntityID: 13, Name: "RewardMob", MapIndex: 42, X: 1785, Y: 2885, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, SpawnGroupRef: "practice.reward_mob", RewardExperience: 75, RewardGold: 60, RewardDropVnums: []uint32{27001, 27002}},
			{EntityID: 11, Name: "Teleporter", MapIndex: 42, X: 1850, Y: 2950, RaceNum: 20303, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:teleporter"},
		}},
		interactionstore.Snapshot{Definitions: []interactionstore.Definition{
			{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."},
			{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
			{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", MapIndex: 42, X: 1700, Y: 2800, Text: "Step through the gate."},
			testMerchantCatalogDefinition(),
		}},
		itemcatalog.Snapshot{Templates: append(testMerchantItemTemplates(),
			itemcatalog.Template{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
		)},
	)
	if err != nil {
		t.Fatalf("from snapshots: %v", err)
	}
	want := Bundle{
		StaticActors: []StaticActor{
			{Name: "Blacksmith", MapIndex: 42, X: 1750, Y: 2850, RaceNum: 20301},
			{Name: "Merchant", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20302, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"},
			{Name: "SnapshotGuard", MapIndex: 42, X: 1780, Y: 2880, RaceNum: 102, CombatProfile: customProfile},
			{Name: "Teleporter", MapIndex: 42, X: 1850, Y: 2950, RaceNum: 20303, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:teleporter"},
			{Name: "TrainingDummy", MapIndex: 42, X: 1775, Y: 2875, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy},
			{Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"},
		},
		SpawnGroups: []SpawnGroup{
			{Ref: "practice.reward_mob", Name: "RewardMob", MapIndex: 42, X: 1785, Y: 2885, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 75, RewardGold: 60, RewardDropVnums: []uint32{27001, 27002}},
		},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:               customProfile,
			MaxHP:                 24,
			DamagePerNormalAttack: 4,
			AttackValue:           7,
			DefenseValue:          3,
			Level:                 8,
			Rank:                  2,
			RespawnDelayMs:        1500,
			DeathReward:           worldruntime.StaticActorDeathReward{Experience: 12, Gold: 7, DropVnums: []uint32{27001, 27002}},
		}},
		ItemTemplates: append(testMerchantItemTemplates(),
			itemcatalog.Template{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
		),
		InteractionDefinitions: []interactionstore.Definition{
			{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
			testMerchantCatalogDefinition(),
			{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."},
			{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", MapIndex: 42, X: 1700, Y: 2800, Text: "Step through the gate."},
		},
	}
	if !reflect.DeepEqual(bundle, want) {
		t.Fatalf("unexpected portable content bundle:\n got: %#v\nwant: %#v", bundle, want)
	}
}

func TestFromSnapshotsWithItemsKeepsTemplatesReferencedOnlyByCombatProfileRewardDefaults(t *testing.T) {
	const profile = "practice_snapshot_reward_defaults"
	worldruntime.UnregisterStaticActorCombatProfileForTest(profile)
	if !worldruntime.RegisterStaticActorCombatProfile(profile, worldruntime.StaticActorCombatProfileDefaults{
		MaxHP:        24,
		AttackValue:  8,
		DefenseValue: 3,
		RespawnDelay: 1500 * time.Millisecond,
		DeathReward:  worldruntime.StaticActorDeathReward{Experience: 12, Gold: 7, DropVnums: []uint32{27001}},
	}) {
		t.Fatalf("expected custom reward-default profile %q to register", profile)
	}
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	bundle, err := FromSnapshotsWithItems(
		staticstore.Snapshot{StaticActors: []staticstore.StaticActor{{
			EntityID:        5,
			Name:            "RewardDefaultMob",
			MapIndex:        42,
			X:               1785,
			Y:               2885,
			RaceNum:         101,
			CombatProfile:   profile,
			SpawnGroupRef:   "practice.reward_default_mob",
			RewardDropVnums: nil,
		}}},
		interactionstore.Snapshot{},
		itemcatalog.Snapshot{Templates: []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}},
	)
	if err != nil {
		t.Fatalf("from snapshots with combat-profile-default reward drop: %v", err)
	}
	want := Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:              "practice.reward_default_mob",
			Name:             "RewardDefaultMob",
			MapIndex:         42,
			X:                1785,
			Y:                2885,
			RaceNum:          101,
			CombatProfile:    profile,
			RewardExperience: 12,
			RewardGold:       7,
			RewardDropVnums:  []uint32{27001},
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:               profile,
			MaxHP:                 24,
			DamagePerNormalAttack: 5,
			AttackValue:           8,
			DefenseValue:          3,
			Level:                 worldruntime.TrainingDummyBootstrapLevel,
			RespawnDelayMs:        1500,
			DeathReward:           worldruntime.StaticActorDeathReward{Experience: 12, Gold: 7, DropVnums: []uint32{27001}},
		}},
		ItemTemplates: []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}},
	}
	if !reflect.DeepEqual(bundle, want) {
		t.Fatalf("unexpected content bundle with combat-profile-default reward drop:\n got: %#v\nwant: %#v", bundle, want)
	}
}

func TestCanonicalizeNormalizesStructuredShopPreviewCatalog(t *testing.T) {
	bundle, err := Canonicalize(Bundle{
		ItemTemplates: testMerchantItemTemplates(),
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:  interactionstore.KindShopPreview,
			Ref:   "npc:merchant",
			Title: "Village Merchant",
			Catalog: []interactionstore.MerchantCatalogEntry{
				{Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
				{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
			},
		}},
	})
	if err != nil {
		t.Fatalf("canonicalize structured shop preview bundle: %v", err)
	}
	want := Bundle{ItemTemplates: testMerchantItemTemplates(), InteractionDefinitions: []interactionstore.Definition{testMerchantCatalogDefinition()}}
	if !reflect.DeepEqual(bundle, want) {
		t.Fatalf("unexpected canonical structured shop preview bundle:\n got: %#v\nwant: %#v", bundle, want)
	}
}

func TestCanonicalizeMerchantBundleKeepsStableBuySlotAddressing(t *testing.T) {
	bundle, err := Canonicalize(Bundle{
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
			{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1},
		},
		StaticActors: []StaticActor{{Name: "Merchant", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20302, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"}},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:  interactionstore.KindShopPreview,
			Ref:   "npc:merchant",
			Title: "Village Merchant",
			Catalog: []interactionstore.MerchantCatalogEntry{
				{Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
				{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
			},
		}},
	})
	if err != nil {
		t.Fatalf("canonicalize merchant buy bundle: %v", err)
	}
	if len(bundle.ItemTemplates) != 2 {
		t.Fatalf("expected two item templates, got %d", len(bundle.ItemTemplates))
	}
	if got, want := bundle.ItemTemplates[0].Vnum, uint32(11200); got != want {
		t.Fatalf("first item template vnum = %d, want %d", got, want)
	}
	if len(bundle.InteractionDefinitions) != 1 {
		t.Fatalf("expected 1 interaction definition, got %d", len(bundle.InteractionDefinitions))
	}
	catalog := bundle.InteractionDefinitions[0].Catalog
	if len(catalog) != 2 {
		t.Fatalf("expected 2 merchant catalog entries, got %d", len(catalog))
	}
	if catalog[0].Slot != 0 || catalog[0].ItemVnum != 27001 || catalog[0].Price != 50 || catalog[0].Count != 1 {
		t.Fatalf("unexpected canonical merchant slot 0: %+v", catalog[0])
	}
	if catalog[1].Slot != 1 || catalog[1].ItemVnum != 11200 || catalog[1].Price != 500 || catalog[1].Count != 1 {
		t.Fatalf("unexpected canonical merchant slot 1: %+v", catalog[1])
	}
}

func TestCanonicalizeNormalizesItemTemplatesAndValidatesMerchantCatalogRefs(t *testing.T) {
	bundle, err := Canonicalize(Bundle{
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: " Small Red Potion ", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
			{Vnum: 11200, Name: " Wooden Sword ", Stackable: false, MaxCount: 1},
		},
		InteractionDefinitions: []interactionstore.Definition{testMerchantCatalogDefinition()},
	})
	if err != nil {
		t.Fatalf("canonicalize bundle with item templates: %v", err)
	}
	want := Bundle{
		ItemTemplates:          testMerchantItemTemplates(),
		InteractionDefinitions: []interactionstore.Definition{testMerchantCatalogDefinition()},
	}
	if !reflect.DeepEqual(bundle, want) {
		t.Fatalf("unexpected canonical item-template bundle:\n got: %#v\nwant: %#v", bundle, want)
	}
}

func TestCanonicalizeRejectsMerchantCatalogWithoutBundledItemTemplates(t *testing.T) {
	_, err := Canonicalize(Bundle{
		InteractionDefinitions: []interactionstore.Definition{testMerchantCatalogDefinition()},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for merchant catalog without bundled item templates, got %v", err)
	}
}

func TestCanonicalizeRejectsMerchantCatalogRefMissingFromBundledItemTemplates(t *testing.T) {
	_, err := Canonicalize(Bundle{
		ItemTemplates:          []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}},
		InteractionDefinitions: []interactionstore.Definition{testMerchantCatalogDefinition()},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for merchant catalog item missing from bundled item templates, got %v", err)
	}
}

func TestCanonicalizeRejectsUnreferencedItemTemplate(t *testing.T) {
	_, err := Canonicalize(Bundle{
		ItemTemplates:          append(testMerchantItemTemplates(), itemcatalog.Template{Vnum: 70001, Name: "Unused Relic", Stackable: false, MaxCount: 1}),
		InteractionDefinitions: []interactionstore.Definition{testMerchantCatalogDefinition()},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for unreferenced item template, got %v", err)
	}
}

func TestCanonicalizeRejectsDuplicateItemTemplates(t *testing.T) {
	_, err := Canonicalize(Bundle{
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			{Vnum: 27001, Name: "Duplicate Small Red Potion", Stackable: true, MaxCount: 200},
		},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:  interactionstore.KindShopPreview,
			Ref:   "npc:merchant",
			Title: "Village Merchant",
			Catalog: []interactionstore.MerchantCatalogEntry{
				{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
			},
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for duplicate item templates, got %v", err)
	}
}

func TestFromSnapshotsOmitsUnreferencedItemTemplates(t *testing.T) {
	bundle, err := FromSnapshotsWithItems(
		staticstore.Snapshot{},
		interactionstore.Snapshot{Definitions: []interactionstore.Definition{testMerchantCatalogDefinition()}},
		itemcatalog.Snapshot{Templates: append(testMerchantItemTemplates(), itemcatalog.Template{Vnum: 70001, Name: "Unused Relic", Stackable: false, MaxCount: 1})},
	)
	if err != nil {
		t.Fatalf("from snapshots with unreferenced item template: %v", err)
	}
	if !reflect.DeepEqual(bundle.ItemTemplates, testMerchantItemTemplates()) {
		t.Fatalf("unexpected exported item templates:\n got: %#v\nwant: %#v", bundle.ItemTemplates, testMerchantItemTemplates())
	}
}

func TestCanonicalizeRejectsMerchantCatalogCountAboveBundledStackLimit(t *testing.T) {
	_, err := Canonicalize(Bundle{
		ItemTemplates: []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 10}},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:  interactionstore.KindShopPreview,
			Ref:   "npc:merchant",
			Title: "Village Merchant",
			Catalog: []interactionstore.MerchantCatalogEntry{
				{Slot: 0, ItemVnum: 27001, Price: 50, Count: 11},
			},
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for merchant catalog count above stack limit, got %v", err)
	}
}

func TestCanonicalizeRejectsMerchantCatalogMultipleNonStackableBundledItem(t *testing.T) {
	_, err := Canonicalize(Bundle{
		ItemTemplates: []itemcatalog.Template{{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1}},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:  interactionstore.KindShopPreview,
			Ref:   "npc:merchant",
			Title: "Village Merchant",
			Catalog: []interactionstore.MerchantCatalogEntry{
				{Slot: 0, ItemVnum: 11200, Price: 500, Count: 2},
			},
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for merchant catalog count above non-stackable limit, got %v", err)
	}
}

func TestCanonicalizeRejectsRewardDropMissingFromBundledItemTemplates(t *testing.T) {
	_, err := Canonicalize(Bundle{
		ItemTemplates: []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}},
		SpawnGroups: []SpawnGroup{{
			Ref:             "practice.reward_mob",
			Name:            "Reward Mob",
			MapIndex:        42,
			X:               1785,
			Y:               2885,
			RaceNum:         101,
			CombatProfile:   worldruntime.StaticActorCombatProfilePracticeMob,
			RewardDropVnums: []uint32{27002},
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for reward drop missing from bundled item templates, got %v", err)
	}
}

func TestCanonicalizeRejectsRewardDropWithoutBundledItemTemplates(t *testing.T) {
	_, err := Canonicalize(Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:             "practice.reward_mob",
			Name:            "Reward Mob",
			MapIndex:        42,
			X:               1785,
			Y:               2885,
			RaceNum:         101,
			CombatProfile:   worldruntime.StaticActorCombatProfilePracticeMob,
			RewardDropVnums: []uint32{27001},
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for reward drop without bundled item templates, got %v", err)
	}
}

func TestCanonicalizeRejectsCombatProfileRewardDropWithoutBundledItemTemplates(t *testing.T) {
	const profile = "practice_reward_drop_defaults"
	_, err := Canonicalize(Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:           "practice.reward_default_mob",
			Name:          "Reward Default Mob",
			MapIndex:      42,
			X:             1785,
			Y:             2885,
			RaceNum:       101,
			CombatProfile: profile,
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:        profile,
			MaxHP:          24,
			AttackValue:    8,
			DefenseValue:   3,
			RespawnDelayMs: 1500,
			DeathReward:    worldruntime.StaticActorDeathReward{DropVnums: []uint32{27001}},
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for combat-profile reward drop without bundled item templates, got %v", err)
	}
}

func TestCanonicalizeRejectsCombatProfileRewardDuplicateDropVnums(t *testing.T) {
	const profile = "practice_duplicate_reward_defaults"
	_, err := Canonicalize(Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:           "practice.duplicate_reward_default_mob",
			Name:          "Duplicate Reward Default Mob",
			MapIndex:      42,
			X:             1785,
			Y:             2885,
			RaceNum:       101,
			CombatProfile: profile,
		}},
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
		},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:        profile,
			MaxHP:          24,
			AttackValue:    8,
			DefenseValue:   3,
			RespawnDelayMs: 1500,
			DeathReward:    worldruntime.StaticActorDeathReward{DropVnums: []uint32{27002, 27001, 27002}},
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for duplicate combat-profile reward drop vnums, got %v", err)
	}
}

func TestCanonicalizeAcceptsRewardDropsBackedByBundledItemTemplates(t *testing.T) {
	bundle, err := Canonicalize(Bundle{
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
		},
		SpawnGroups: []SpawnGroup{{
			Ref:             "practice.reward_mob",
			Name:            "Reward Mob",
			MapIndex:        42,
			X:               1785,
			Y:               2885,
			RaceNum:         101,
			CombatProfile:   worldruntime.StaticActorCombatProfilePracticeMob,
			RewardDropVnums: []uint32{27002, 27001},
		}},
	})
	if err != nil {
		t.Fatalf("canonicalize reward drops backed by item templates: %v", err)
	}
	wantDrops := []uint32{27001, 27002}
	if len(bundle.SpawnGroups) != 1 || !reflect.DeepEqual(bundle.SpawnGroups[0].RewardDropVnums, wantDrops) {
		t.Fatalf("unexpected canonical reward drops: %+v", bundle.SpawnGroups)
	}
}

func TestExampleBootstrapNPCServiceBundleStaysValid(t *testing.T) {
	raw, canonical := readCanonicalExampleBundle(t, "bootstrap-npc-service-bundle.json")
	if len(canonical.ItemTemplates) == 0 || len(canonical.SpawnGroups) == 0 || len(canonical.InteractionDefinitions) == 0 {
		t.Fatalf("example bundle should include item templates, spawn groups, and interaction definitions: %+v", canonical)
	}
	canonicalJSON, err := json.MarshalIndent(canonical, "", "  ")
	if err != nil {
		t.Fatalf("marshal canonical example content bundle: %v", err)
	}
	canonicalJSON = append(canonicalJSON, '\n')
	if string(raw) != string(canonicalJSON) {
		t.Fatalf("example content bundle is not byte-for-byte canonical; update docs/examples/bootstrap-npc-service-bundle.json to:\n%s", string(canonicalJSON))
	}
}

func readCanonicalExampleBundle(t *testing.T, name string) ([]byte, Bundle) {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate contentbundle test file")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	raw, err := os.ReadFile(filepath.Join(repoRoot, "docs", "examples", name))
	if err != nil {
		t.Fatalf("read example content bundle: %v", err)
	}
	var bundle Bundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("decode example content bundle: %v", err)
	}
	canonical, err := Canonicalize(bundle)
	if err != nil {
		t.Fatalf("canonicalize example content bundle: %v", err)
	}
	return raw, canonical
}

func TestCanonicalizeRejectsSparseMerchantCatalogSlots(t *testing.T) {
	_, err := Canonicalize(Bundle{
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:  interactionstore.KindShopPreview,
			Ref:   "npc:merchant",
			Title: "Village Merchant",
			Catalog: []interactionstore.MerchantCatalogEntry{
				{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
				{Slot: 2, ItemVnum: 11200, Price: 500, Count: 1},
			},
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for sparse merchant catalog slots, got %v", err)
	}
}

func TestCanonicalizeRejectsMerchantCatalogSlotAddressOverflow(t *testing.T) {
	_, err := Canonicalize(Bundle{
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:  interactionstore.KindShopPreview,
			Ref:   "npc:merchant",
			Title: "Village Merchant",
			Catalog: []interactionstore.MerchantCatalogEntry{
				{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
				{Slot: 1, ItemVnum: 27002, Price: 50, Count: 1},
				{Slot: 2, ItemVnum: 27003, Price: 50, Count: 1},
				{Slot: 3, ItemVnum: 27004, Price: 50, Count: 1},
				{Slot: 4, ItemVnum: 27005, Price: 50, Count: 1},
				{Slot: 5, ItemVnum: 27006, Price: 50, Count: 1},
				{Slot: 6, ItemVnum: 27007, Price: 50, Count: 1},
				{Slot: 7, ItemVnum: 27008, Price: 50, Count: 1},
				{Slot: 8, ItemVnum: 27009, Price: 50, Count: 1},
				{Slot: 9, ItemVnum: 27010, Price: 50, Count: 1},
				{Slot: 10, ItemVnum: 27011, Price: 50, Count: 1},
				{Slot: 11, ItemVnum: 27012, Price: 50, Count: 1},
				{Slot: 12, ItemVnum: 27013, Price: 50, Count: 1},
				{Slot: 13, ItemVnum: 27014, Price: 50, Count: 1},
				{Slot: 14, ItemVnum: 27015, Price: 50, Count: 1},
				{Slot: 15, ItemVnum: 27016, Price: 50, Count: 1},
				{Slot: 16, ItemVnum: 27017, Price: 50, Count: 1},
				{Slot: 17, ItemVnum: 27018, Price: 50, Count: 1},
				{Slot: 18, ItemVnum: 27019, Price: 50, Count: 1},
				{Slot: 19, ItemVnum: 27020, Price: 50, Count: 1},
				{Slot: 20, ItemVnum: 27021, Price: 50, Count: 1},
				{Slot: 21, ItemVnum: 27022, Price: 50, Count: 1},
				{Slot: 22, ItemVnum: 27023, Price: 50, Count: 1},
				{Slot: 23, ItemVnum: 27024, Price: 50, Count: 1},
				{Slot: 24, ItemVnum: 27025, Price: 50, Count: 1},
				{Slot: 25, ItemVnum: 27026, Price: 50, Count: 1},
				{Slot: 26, ItemVnum: 27027, Price: 50, Count: 1},
				{Slot: 27, ItemVnum: 27028, Price: 50, Count: 1},
				{Slot: 28, ItemVnum: 27029, Price: 50, Count: 1},
				{Slot: 29, ItemVnum: 27030, Price: 50, Count: 1},
				{Slot: 30, ItemVnum: 27031, Price: 50, Count: 1},
				{Slot: 31, ItemVnum: 27032, Price: 50, Count: 1},
				{Slot: 32, ItemVnum: 27033, Price: 50, Count: 1},
				{Slot: 33, ItemVnum: 27034, Price: 50, Count: 1},
				{Slot: 34, ItemVnum: 27035, Price: 50, Count: 1},
				{Slot: 35, ItemVnum: 27036, Price: 50, Count: 1},
				{Slot: 36, ItemVnum: 27037, Price: 50, Count: 1},
				{Slot: 37, ItemVnum: 27038, Price: 50, Count: 1},
				{Slot: 38, ItemVnum: 27039, Price: 50, Count: 1},
				{Slot: 39, ItemVnum: 27040, Price: 50, Count: 1},
				{Slot: 40, ItemVnum: 27041, Price: 50, Count: 1},
			},
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for merchant catalog beyond one shop page, got %v", err)
	}
}

func TestCanonicalizeRejectsDanglingInteractionReference(t *testing.T) {
	_, err := Canonicalize(Bundle{
		StaticActors:           []StaticActor{{Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for dangling interaction reference, got %v", err)
	}
}

func TestCanonicalizeRejectsDuplicateStaticActorAuthoringRows(t *testing.T) {
	_, err := Canonicalize(Bundle{
		StaticActors: []StaticActor{
			{Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"},
			{Name: " VillageGuard ", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: " talk ", InteractionRef: " npc:village_guard "},
		},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for duplicate authored static actor row, got %v", err)
	}
}

func TestCanonicalizeRejectsEmbeddedNULAuthoredStaticActorNames(t *testing.T) {
	cases := []struct {
		name   string
		bundle Bundle
	}{
		{
			name:   "static_actor",
			bundle: Bundle{StaticActors: []StaticActor{{Name: "Visible\x00Hidden", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300}}},
		},
		{
			name:   "spawn_group",
			bundle: Bundle{SpawnGroups: []SpawnGroup{{Ref: "practice.nul_named_mob", Name: "Visible\x00Hidden", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Canonicalize(tc.bundle)
			if !errors.Is(err, ErrInvalidBundle) {
				t.Fatalf("expected ErrInvalidBundle for embedded-NUL %s name, got %v", tc.name, err)
			}
		})
	}
}

func TestCanonicalizeRejectsDuplicateInteractionDefinitions(t *testing.T) {
	_, err := Canonicalize(Bundle{
		InteractionDefinitions: []interactionstore.Definition{
			{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "First"},
			{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "Duplicate"},
		},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for duplicate interaction definitions, got %v", err)
	}
}

func TestCanonicalizeRejectsPathAmbiguousInteractionRefs(t *testing.T) {
	_, err := Canonicalize(Bundle{
		StaticActors:           []StaticActor{{Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc/village_guard"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc/village_guard", Text: "Keep your blade sharp."}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for path-ambiguous interaction refs, got %v", err)
	}
}

func TestCanonicalizeRejectsInvalidWarpInteractionDefinition(t *testing.T) {
	_, err := Canonicalize(Bundle{
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", X: 1700, Y: 2800}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for invalid warp interaction definition, got %v", err)
	}
}

func TestCanonicalizeRejectsEmbeddedNULInteractionDefinitionTextFields(t *testing.T) {
	cases := []struct {
		name   string
		bundle Bundle
	}{
		{
			name:   "info_text",
			bundle: Bundle{InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "visible\x00hidden"}}},
		},
		{
			name:   "talk_text",
			bundle: Bundle{InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "visible\x00hidden"}}},
		},
		{
			name:   "warp_text",
			bundle: Bundle{InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", Text: "visible\x00hidden", MapIndex: 42, X: 1700, Y: 2800}}},
		},
		{
			name: "shop_preview_title",
			bundle: Bundle{
				ItemTemplates: []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}},
				InteractionDefinitions: []interactionstore.Definition{{
					Kind:  interactionstore.KindShopPreview,
					Ref:   "npc:merchant",
					Title: "Village\x00Merchant",
					Catalog: []interactionstore.MerchantCatalogEntry{
						{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
					},
				}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Canonicalize(tc.bundle)
			if !errors.Is(err, ErrInvalidBundle) {
				t.Fatalf("expected ErrInvalidBundle for NUL %s, got %v", tc.name, err)
			}
		})
	}
}

func TestCanonicalizeAcceptsReferencedCustomCombatProfileSnapshot(t *testing.T) {
	bundle, err := Canonicalize(Bundle{
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
		},
		SpawnGroups: []SpawnGroup{{
			Ref:           "practice.imported_wolf",
			Name:          "Imported Wolf",
			MapIndex:      42,
			X:             1800,
			Y:             2900,
			RaceNum:       101,
			CombatProfile: "practice_imported_wolf",
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:               " practice_imported_wolf ",
			MaxHP:                 24,
			DamagePerNormalAttack: 6,
			AttackValue:           8,
			DefenseValue:          2,
			Level:                 7,
			Rank:                  2,
			RespawnDelayMs:        1500,
			RetaliationPointDelta: -2,
			DeathReward:           worldruntime.StaticActorDeathReward{Experience: 25, Gold: 11, DropVnums: []uint32{27002, 27001}},
		}},
	})
	if err != nil {
		t.Fatalf("canonicalize referenced custom combat profile snapshot: %v", err)
	}
	want := Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:              "practice.imported_wolf",
			Name:             "Imported Wolf",
			MapIndex:         42,
			X:                1800,
			Y:                2900,
			RaceNum:          101,
			CombatProfile:    "practice_imported_wolf",
			RewardExperience: 25,
			RewardGold:       11,
			RewardDropVnums:  []uint32{27001, 27002},
		}},
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
		},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:               "practice_imported_wolf",
			MaxHP:                 24,
			DamagePerNormalAttack: 6,
			AttackValue:           8,
			DefenseValue:          2,
			Level:                 7,
			Rank:                  2,
			RespawnDelayMs:        1500,
			RetaliationPointDelta: -2,
			DeathReward:           worldruntime.StaticActorDeathReward{Experience: 25, Gold: 11, DropVnums: []uint32{27001, 27002}},
		}},
	}
	if !reflect.DeepEqual(bundle, want) {
		t.Fatalf("unexpected canonical custom combat profile bundle:\n got: %#v\nwant: %#v", bundle, want)
	}
}

func TestCanonicalizeRejectsPositiveCombatProfileRetaliationPointDelta(t *testing.T) {
	_, err := Canonicalize(Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:           "practice.positive_retaliation_wolf",
			Name:          "Positive Retaliation Wolf",
			MapIndex:      42,
			X:             1800,
			Y:             2900,
			RaceNum:       101,
			CombatProfile: "practice_positive_retaliation_wolf",
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:               "practice_positive_retaliation_wolf",
			MaxHP:                 24,
			DamagePerNormalAttack: 6,
			AttackValue:           8,
			DefenseValue:          2,
			RespawnDelayMs:        1500,
			RetaliationPointDelta: 1,
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for positive combat-profile retaliation point delta, got %v", err)
	}
}

func TestCanonicalizeRejectsInvalidCombatProfile(t *testing.T) {
	_, err := Canonicalize(Bundle{
		StaticActors: []StaticActor{{Name: "BrokenDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: "boss"}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for invalid combat profile, got %v", err)
	}
}

func TestCanonicalizeRegistersPortableCombatProfileSnapshotsBeforeValidatingActors(t *testing.T) {
	const profile = "practice_portable_wolf"

	bundle, err := Canonicalize(Bundle{
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
		},
		SpawnGroups: []SpawnGroup{{
			Ref:           "practice.portable_wolf",
			Name:          "Portable Wolf",
			MapIndex:      42,
			X:             1775,
			Y:             2875,
			RaceNum:       101,
			CombatProfile: profile,
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:        profile,
			MaxHP:          24,
			AttackValue:    8,
			DefenseValue:   3,
			Level:          7,
			Rank:           2,
			RespawnDelayMs: 1500,
			DeathReward:    worldruntime.StaticActorDeathReward{Experience: 25, Gold: 11, DropVnums: []uint32{27002, 27001}},
		}},
	})
	if err != nil {
		t.Fatalf("canonicalize portable combat profile bundle: %v", err)
	}

	want := Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:              "practice.portable_wolf",
			Name:             "Portable Wolf",
			MapIndex:         42,
			X:                1775,
			Y:                2875,
			RaceNum:          101,
			CombatProfile:    profile,
			RewardExperience: 25,
			RewardGold:       11,
			RewardDropVnums:  []uint32{27001, 27002},
		}},
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
		},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:               profile,
			MaxHP:                 24,
			DamagePerNormalAttack: 0,
			AttackValue:           8,
			DefenseValue:          3,
			Level:                 7,
			Rank:                  2,
			RespawnDelayMs:        1500,
			DeathReward:           worldruntime.StaticActorDeathReward{Experience: 25, Gold: 11, DropVnums: []uint32{27001, 27002}},
		}},
	}
	if !reflect.DeepEqual(bundle, want) {
		t.Fatalf("unexpected portable combat profile canonical bundle:\n got: %#v\nwant: %#v", bundle, want)
	}
}

func TestCanonicalizeRollsBackPortableCombatProfileOnLaterValidationFailure(t *testing.T) {
	const profile = "practice_portable_invalid_wolf"
	_, err := Canonicalize(Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:             "practice.portable_invalid_wolf",
			Name:            "Portable Invalid Wolf",
			MapIndex:        42,
			X:               1775,
			Y:               2875,
			RaceNum:         101,
			CombatProfile:   profile,
			RewardDropVnums: []uint32{27001, 27001},
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:        profile,
			MaxHP:          24,
			AttackValue:    8,
			DefenseValue:   3,
			RespawnDelayMs: 1500,
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for invalid portable bundle, got %v", err)
	}
	if worldruntime.ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected failed bundle validation not to register portable profile %q", profile)
	}
}

func TestCanonicalizeRejectsUnreferencedCombatProfileSnapshot(t *testing.T) {
	_, err := Canonicalize(Bundle{CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
		Profile:        "practice_unreferenced_wolf",
		MaxHP:          24,
		AttackValue:    8,
		RespawnDelayMs: 1500,
	}}})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for unreferenced combat profile snapshot, got %v", err)
	}
}

func TestCanonicalizeRejectsDuplicateCombatProfileSnapshots(t *testing.T) {
	_, err := Canonicalize(Bundle{
		SpawnGroups: []SpawnGroup{{Ref: "practice.imported_wolf", Name: "Imported Wolf", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: "practice_imported_wolf"}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{
			{Profile: "practice_imported_wolf", MaxHP: 24, AttackValue: 8, RespawnDelayMs: 1500},
			{Profile: " practice_imported_wolf ", MaxHP: 24, AttackValue: 8, RespawnDelayMs: 1500},
		},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for duplicate combat profile snapshots, got %v", err)
	}
}

func TestCanonicalizeRejectsConflictingRegisteredCombatProfileSnapshot(t *testing.T) {
	const profile = "practice_content_conflict_wolf"
	worldruntime.UnregisterStaticActorCombatProfileForTest(profile)
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })
	if !worldruntime.RegisterStaticActorCombatProfile(profile, worldruntime.StaticActorCombatProfileDefaults{
		MaxHP:                 24,
		DamagePerNormalAttack: 3,
		AttackValue:           7,
		DefenseValue:          4,
		RespawnDelay:          1500 * time.Millisecond,
	}) {
		t.Fatalf("expected local combat profile %q to register", profile)
	}

	_, err := Canonicalize(Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:           "practice.content_conflict_wolf",
			Name:          "Content Conflict Wolf",
			MapIndex:      42,
			X:             1800,
			Y:             2900,
			RaceNum:       101,
			CombatProfile: profile,
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:               profile,
			MaxHP:                 30,
			DamagePerNormalAttack: 3,
			AttackValue:           7,
			DefenseValue:          4,
			RespawnDelayMs:        1500,
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for conflicting registered combat profile snapshot, got %v", err)
	}
	defaults, ok := worldruntime.BootstrapStaticActorCombatProfileDefaults(profile)
	if !ok || defaults.MaxHP != 24 || defaults.DamagePerNormalAttack != 3 {
		t.Fatalf("expected existing registered profile defaults to remain unchanged, got defaults=%+v ok=%v", defaults, ok)
	}
}

func TestCanonicalizeAcceptsMatchingRegisteredCombatProfileSnapshot(t *testing.T) {
	const profile = "practice_content_matching_wolf"
	worldruntime.UnregisterStaticActorCombatProfileForTest(profile)
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })
	if !worldruntime.RegisterStaticActorCombatProfile(profile, worldruntime.StaticActorCombatProfileDefaults{
		MaxHP:                 24,
		DamagePerNormalAttack: 3,
		AttackValue:           7,
		DefenseValue:          4,
		Level:                 9,
		Rank:                  2,
		RespawnDelay:          1500 * time.Millisecond,
		DeathReward:           worldruntime.StaticActorDeathReward{Experience: 25, Gold: 11},
	}) {
		t.Fatalf("expected local combat profile %q to register", profile)
	}

	_, err := Canonicalize(Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:           "practice.content_matching_wolf",
			Name:          "Content Matching Wolf",
			MapIndex:      42,
			X:             1800,
			Y:             2900,
			RaceNum:       101,
			CombatProfile: profile,
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:               profile,
			MaxHP:                 24,
			DamagePerNormalAttack: 3,
			AttackValue:           7,
			DefenseValue:          4,
			Level:                 9,
			Rank:                  2,
			RespawnDelayMs:        1500,
			DeathReward:           worldruntime.StaticActorDeathReward{Experience: 25, Gold: 11},
		}},
	})
	if err != nil {
		t.Fatalf("expected matching registered combat profile snapshot to canonicalize, got %v", err)
	}
}

func TestCanonicalizeRejectsInvalidCombatProfileSnapshot(t *testing.T) {
	_, err := Canonicalize(Bundle{
		SpawnGroups:    []SpawnGroup{{Ref: "practice.imported_wolf", Name: "Imported Wolf", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: "practice_imported_wolf"}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{Profile: "practice_imported_wolf", AttackValue: 8, RespawnDelayMs: 1500}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for invalid combat profile snapshot, got %v", err)
	}
}

func TestCanonicalizeRejectsCombatProfileSnapshotWithConflictingLegacyDamage(t *testing.T) {
	_, err := Canonicalize(Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:           "practice.conflicting_wolf",
			Name:          "Conflicting Wolf",
			MapIndex:      42,
			X:             1800,
			Y:             2900,
			RaceNum:       101,
			CombatProfile: "practice_conflicting_wolf",
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:               "practice_conflicting_wolf",
			MaxHP:                 24,
			DamagePerNormalAttack: 3,
			AttackValue:           8,
			DefenseValue:          2,
			RespawnDelayMs:        1500,
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for conflicting combat profile damage, got %v", err)
	}
}

func TestCanonicalizeRejectsCombatProfileSnapshotFormulaDamageAboveMaxHP(t *testing.T) {
	_, err := Canonicalize(Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:           "practice.burst_wolf",
			Name:          "Burst Wolf",
			MapIndex:      42,
			X:             1800,
			Y:             2900,
			RaceNum:       101,
			CombatProfile: "practice_burst_wolf",
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:        "practice_burst_wolf",
			MaxHP:          5,
			AttackValue:    8,
			DefenseValue:   2,
			RespawnDelayMs: 1500,
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for over-max combat profile formula damage, got %v", err)
	}
}

func TestCanonicalizeTrimsStaticActorAuthoringFields(t *testing.T) {
	bundle, err := Canonicalize(Bundle{
		StaticActors: []StaticActor{{
			Name:            "  TrainingDummy  ",
			MapIndex:        42,
			X:               1800,
			Y:               2900,
			RaceNum:         20350,
			CombatProfile:   " training_dummy ",
			InteractionKind: " talk ",
			InteractionRef:  " npc:village_guard ",
		}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."}},
	})
	if err != nil {
		t.Fatalf("canonicalize static actor with padded authoring fields: %v", err)
	}
	want := Bundle{
		StaticActors:           []StaticActor{{Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."}},
	}
	if !reflect.DeepEqual(bundle, want) {
		t.Fatalf("unexpected canonical static actor fields:\n got: %#v\nwant: %#v", bundle, want)
	}
}

func TestCanonicalizeRejectsDuplicateSpawnGroupRefs(t *testing.T) {
	_, err := Canonicalize(Bundle{SpawnGroups: []SpawnGroup{
		{Ref: "practice.mob_alpha", Name: "Practice Mob Alpha", MapIndex: 42, X: 1775, Y: 2875, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy},
		{Ref: "practice.mob_alpha", Name: "Practice Mob Beta", MapIndex: 42, X: 1875, Y: 2975, RaceNum: 102, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy},
	}})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for duplicate spawn-group refs, got %v", err)
	}
}

func TestCanonicalizeRejectsNonCanonicalSpawnGroupRefs(t *testing.T) {
	for name, ref := range map[string]string{
		"single segment":    "practice",
		"uppercase segment": "practice.MobAlpha",
		"hyphen segment":    "practice.mob-alpha",
		"leading digit":     "practice.1mob_alpha",
		"trailing space":    "practice.mob_alpha ",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Canonicalize(Bundle{SpawnGroups: []SpawnGroup{{
				Ref:           ref,
				Name:          "Practice Mob Alpha",
				MapIndex:      42,
				X:             1775,
				Y:             2875,
				RaceNum:       101,
				CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy,
			}}})
			if !errors.Is(err, ErrInvalidBundle) {
				t.Fatalf("expected ErrInvalidBundle for spawn-group ref %q, got %v", ref, err)
			}
		})
	}
}

func TestCanonicalizeKeepsSpawnGroupRewardDescriptor(t *testing.T) {
	dropVnums := []uint32{27002, 27001}
	bundle, err := Canonicalize(Bundle{
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
		},
		SpawnGroups: []SpawnGroup{{
			Ref:              "practice.reward_mob",
			Name:             " Reward Mob ",
			MapIndex:         42,
			X:                1775,
			Y:                2875,
			RaceNum:          101,
			CombatProfile:    " training_dummy ",
			RewardExperience: 75,
			RewardGold:       60,
			RewardDropVnums:  dropVnums,
		}},
	})
	if err != nil {
		t.Fatalf("canonicalize reward spawn group: %v", err)
	}
	want := Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:              "practice.reward_mob",
			Name:             "Reward Mob",
			MapIndex:         42,
			X:                1775,
			Y:                2875,
			RaceNum:          101,
			CombatProfile:    worldruntime.StaticActorCombatProfileTrainingDummy,
			RewardExperience: 75,
			RewardGold:       60,
			RewardDropVnums:  []uint32{27001, 27002},
		}},
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
		},
	}
	if !reflect.DeepEqual(bundle, want) {
		t.Fatalf("unexpected canonical reward spawn group:\n got: %#v\nwant: %#v", bundle, want)
	}
	dropVnums[0] = 0
	if bundle.SpawnGroups[0].RewardDropVnums[0] != 27001 {
		t.Fatalf("expected reward drop vnums to be cloned, got %#v", bundle.SpawnGroups[0].RewardDropVnums)
	}
}

func TestCanonicalizeAppliesRegisteredProfileRewardDefaultsToSpawnGroupWithoutRewardDescriptor(t *testing.T) {
	const profile = "practice_reward_defaults"
	if !worldruntime.RegisterStaticActorCombatProfile(profile, worldruntime.StaticActorCombatProfileDefaults{
		MaxHP:                 24,
		DamagePerNormalAttack: 3,
		AttackValue:           7,
		DefenseValue:          4,
		Level:                 9,
		Rank:                  2,
		RespawnDelay:          1500 * time.Millisecond,
		DeathReward:           worldruntime.StaticActorDeathReward{Experience: 15, Gold: 10, DropVnums: []uint32{27002, 27001}},
	}) {
		t.Fatalf("expected registered reward-default profile %q", profile)
	}
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	bundle, err := Canonicalize(Bundle{
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
		},
		SpawnGroups: []SpawnGroup{{
			Ref:           "practice.mob_alpha",
			Name:          "Practice Mob Alpha",
			MapIndex:      42,
			X:             1775,
			Y:             2875,
			RaceNum:       101,
			CombatProfile: profile,
		}},
	})
	if err != nil {
		t.Fatalf("canonicalize reward-default spawn group: %v", err)
	}
	want := Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:              "practice.mob_alpha",
			Name:             "Practice Mob Alpha",
			MapIndex:         42,
			X:                1775,
			Y:                2875,
			RaceNum:          101,
			CombatProfile:    profile,
			RewardExperience: 15,
			RewardGold:       10,
			RewardDropVnums:  []uint32{27001, 27002},
		}},
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
		},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:               profile,
			MaxHP:                 24,
			DamagePerNormalAttack: 3,
			AttackValue:           7,
			DefenseValue:          4,
			Level:                 9,
			Rank:                  2,
			RespawnDelayMs:        1500,
			DeathReward:           worldruntime.StaticActorDeathReward{Experience: 15, Gold: 10, DropVnums: []uint32{27001, 27002}},
		}},
	}
	if !reflect.DeepEqual(bundle, want) {
		t.Fatalf("unexpected canonical reward-default spawn group:\n got: %#v\nwant: %#v", bundle, want)
	}
}

func TestCanonicalizeRejectsInvalidSpawnGroupRewardDescriptor(t *testing.T) {
	maxPointCarrier := uint64(^uint32(0) >> 1)
	for name, spawnGroup := range map[string]SpawnGroup{
		"experience overflow": {Ref: "practice.exp_overflow", Name: "Exp Overflow", MapIndex: 42, X: 1775, Y: 2875, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, RewardExperience: maxPointCarrier + 1},
		"gold overflow":       {Ref: "practice.gold_overflow", Name: "Gold Overflow", MapIndex: 42, X: 1775, Y: 2875, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, RewardGold: maxPointCarrier + 1},
		"zero drop vnum":      {Ref: "practice.zero_drop", Name: "Zero Drop", MapIndex: 42, X: 1775, Y: 2875, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, RewardDropVnums: []uint32{27001, 0}},
		"duplicate drop vnum": {Ref: "practice.duplicate_drop", Name: "Duplicate Drop", MapIndex: 42, X: 1775, Y: 2875, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, RewardDropVnums: []uint32{27001, 27002, 27001}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Canonicalize(Bundle{SpawnGroups: []SpawnGroup{spawnGroup}})
			if !errors.Is(err, ErrInvalidBundle) {
				t.Fatalf("expected ErrInvalidBundle for %s, got %v", name, err)
			}
		})
	}
}

func TestCanonicalizeAppliesPracticeMobDefaultsToSpawnGroupWithoutCombatProfile(t *testing.T) {
	bundle, err := Canonicalize(Bundle{SpawnGroups: []SpawnGroup{{
		Ref:      "practice.mob_alpha",
		Name:     "Practice Mob Alpha",
		MapIndex: 42,
		X:        1775,
		Y:        2875,
		RaceNum:  101,
	}}})
	if err != nil {
		t.Fatalf("expected spawn group without explicit combat profile to use practice-mob defaults, got %v", err)
	}
	if len(bundle.SpawnGroups) != 1 || bundle.SpawnGroups[0].CombatProfile != worldruntime.StaticActorCombatProfilePracticeMob {
		t.Fatalf("expected practice-mob combat profile default, got %#v", bundle.SpawnGroups)
	}
}

func TestCanonicalizeAcceptsRegisteredSpawnGroupCombatProfile(t *testing.T) {
	const profile = "practice_bundle_wolf"
	if !worldruntime.RegisterStaticActorCombatProfile(profile, worldruntime.StaticActorCombatProfileDefaults{
		MaxHP:        24,
		AttackValue:  8,
		DefenseValue: 3,
		Level:        7,
		Rank:         2,
		RespawnDelay: worldruntime.PracticeMobBootstrapRespawnDelay,
		DeathReward:  worldruntime.StaticActorDeathReward{Experience: 25, Gold: 11, DropVnums: []uint32{27002, 27001}},
	}) {
		t.Fatalf("expected registered combat profile %q to be accepted", profile)
	}
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	dropVnums := []uint32{27002, 27001}
	bundle, err := Canonicalize(Bundle{
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
		},
		SpawnGroups: []SpawnGroup{{
			Ref:              "practice.bundle_wolf",
			Name:             "Practice Bundle Wolf",
			MapIndex:         42,
			X:                1775,
			Y:                2875,
			RaceNum:          101,
			CombatProfile:    " practice_bundle_wolf ",
			RewardExperience: 75,
			RewardGold:       60,
			RewardDropVnums:  dropVnums,
		}},
	})
	if err != nil {
		t.Fatalf("expected spawn group using registered combat profile to canonicalize, got %v", err)
	}

	want := Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:              "practice.bundle_wolf",
			Name:             "Practice Bundle Wolf",
			MapIndex:         42,
			X:                1775,
			Y:                2875,
			RaceNum:          101,
			CombatProfile:    profile,
			RewardExperience: 75,
			RewardGold:       60,
			RewardDropVnums:  []uint32{27001, 27002},
		}},
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
		},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:               profile,
			MaxHP:                 24,
			DamagePerNormalAttack: 5,
			AttackValue:           8,
			DefenseValue:          3,
			Level:                 7,
			Rank:                  2,
			RespawnDelayMs:        2000,
			DeathReward:           worldruntime.StaticActorDeathReward{Experience: 25, Gold: 11, DropVnums: []uint32{27001, 27002}},
		}},
	}
	if !reflect.DeepEqual(bundle, want) {
		t.Fatalf("unexpected canonical registered-profile spawn group:\n got: %#v\nwant: %#v", bundle, want)
	}
	dropVnums[0] = 0
	if bundle.SpawnGroups[0].RewardDropVnums[0] != 27001 {
		t.Fatalf("expected registered-profile spawn reward drops to be cloned, got %#v", bundle.SpawnGroups[0].RewardDropVnums)
	}
}

func TestCanonicalizeRejectsSpawnGroupWithBlankName(t *testing.T) {
	_, err := Canonicalize(Bundle{SpawnGroups: []SpawnGroup{{
		Ref:           "practice.mob_alpha",
		MapIndex:      42,
		X:             1775,
		Y:             2875,
		RaceNum:       101,
		CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy,
	}}})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for blank spawn-group name, got %v", err)
	}
}

func TestCanonicalizeRejectsStaticActorRaceNumOutsideBootstrapWireRange(t *testing.T) {
	_, err := Canonicalize(Bundle{StaticActors: []StaticActor{{
		Name:     "OversizedActor",
		MapIndex: 42,
		X:        1775,
		Y:        2875,
		RaceNum:  uint32(^uint16(0)) + 1,
	}}})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for static actor race_num outside bootstrap wire range, got %v", err)
	}
}

func TestCanonicalizeRejectsSpawnGroupRaceNumOutsideBootstrapWireRange(t *testing.T) {
	_, err := Canonicalize(Bundle{SpawnGroups: []SpawnGroup{{
		Ref:           "practice.oversized_mob",
		Name:          "Oversized Mob",
		MapIndex:      42,
		X:             1775,
		Y:             2875,
		RaceNum:       uint32(^uint16(0)) + 1,
		CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy,
	}}})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for spawn-group race_num outside bootstrap wire range, got %v", err)
	}
}

func TestExampleBootstrapNPCServiceBundleCanonicalizes(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "examples", "bootstrap-npc-service-bundle.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read example bundle: %v", err)
	}
	var bundle Bundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("decode example bundle: %v", err)
	}
	canonical, err := Canonicalize(bundle)
	if err != nil {
		t.Fatalf("canonicalize example bundle: %v", err)
	}
	if !reflect.DeepEqual(canonical, bundle) {
		t.Fatalf("expected example bundle to already be canonical:\n got: %#v\nwant: %#v", canonical, bundle)
	}
}

func TestExampleBootstrapNPCServiceBundleCarriesMerchantItemTemplates(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "examples", "bootstrap-npc-service-bundle.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read example bundle: %v", err)
	}
	var bundle Bundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("decode example bundle: %v", err)
	}
	if len(bundle.ItemTemplates) == 0 {
		t.Fatalf("expected example bundle to carry item templates for merchant catalog refs")
	}
	templatesByVnum := make(map[uint32]struct{}, len(bundle.ItemTemplates))
	for _, template := range bundle.ItemTemplates {
		templatesByVnum[template.Vnum] = struct{}{}
	}
	for _, definition := range bundle.InteractionDefinitions {
		if definition.Kind != interactionstore.KindShopPreview {
			continue
		}
		for _, entry := range definition.Catalog {
			if _, ok := templatesByVnum[entry.ItemVnum]; !ok {
				t.Fatalf("expected example merchant catalog item vnum %d to have a bundled item template", entry.ItemVnum)
			}
		}
	}
}

func TestFromSnapshotsSeparatesSpawnGroupsFromStaticActors(t *testing.T) {
	bundle, err := FromSnapshotsWithItems(
		staticstore.Snapshot{StaticActors: []staticstore.StaticActor{
			{EntityID: 5, Name: "PracticeMobAlpha", MapIndex: 42, X: 1775, Y: 2875, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, SpawnGroupRef: "practice.mob_alpha", RewardExperience: 75, RewardGold: 60, RewardDropVnums: []uint32{27001, 27002}},
			{EntityID: 9, Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"},
		}},
		interactionstore.Snapshot{Definitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."}}},
		itemcatalog.Snapshot{Templates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
		}},
	)
	if err != nil {
		t.Fatalf("from snapshots with spawn group: %v", err)
	}
	want := Bundle{
		StaticActors: []StaticActor{{Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"}},
		SpawnGroups:  []SpawnGroup{{Ref: "practice.mob_alpha", Name: "PracticeMobAlpha", MapIndex: 42, X: 1775, Y: 2875, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, RewardExperience: 75, RewardGold: 60, RewardDropVnums: []uint32{27001, 27002}}},
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
		},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."}},
	}
	if !reflect.DeepEqual(bundle, want) {
		t.Fatalf("unexpected bundle with separated spawn groups:\n got: %#v\nwant: %#v", bundle, want)
	}
}
