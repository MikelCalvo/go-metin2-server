package contentbundle

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
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

func TestFromSnapshotsBuildsDeterministicPortableBundle(t *testing.T) {
	bundle, err := FromSnapshots(
		staticstore.Snapshot{StaticActors: []staticstore.StaticActor{
			{EntityID: 9, Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"},
			{EntityID: 3, Name: "Blacksmith", MapIndex: 42, X: 1750, Y: 2850, RaceNum: 20301},
			{EntityID: 7, Name: "Merchant", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20302, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"},
			{EntityID: 5, Name: "TrainingDummy", MapIndex: 42, X: 1775, Y: 2875, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy},
			{EntityID: 13, Name: "RewardMob", MapIndex: 42, X: 1785, Y: 2885, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, SpawnGroupRef: "practice.reward_mob", RewardExperience: 75, RewardGold: 60, RewardDropVnums: []uint32{27001, 27002}},
			{EntityID: 11, Name: "Teleporter", MapIndex: 42, X: 1850, Y: 2950, RaceNum: 20303, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:teleporter"},
		}},
		interactionstore.Snapshot{Definitions: []interactionstore.Definition{
			{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."},
			{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
			{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", MapIndex: 42, X: 1700, Y: 2800, Text: "Step through the gate."},
			testMerchantCatalogDefinition(),
		}},
	)
	if err != nil {
		t.Fatalf("from snapshots: %v", err)
	}
	want := Bundle{
		StaticActors: []StaticActor{
			{Name: "Blacksmith", MapIndex: 42, X: 1750, Y: 2850, RaceNum: 20301},
			{Name: "Merchant", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20302, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"},
			{Name: "Teleporter", MapIndex: 42, X: 1850, Y: 2950, RaceNum: 20303, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:teleporter"},
			{Name: "TrainingDummy", MapIndex: 42, X: 1775, Y: 2875, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy},
			{Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"},
		},
		SpawnGroups: []SpawnGroup{
			{Ref: "practice.reward_mob", Name: "RewardMob", MapIndex: 42, X: 1785, Y: 2885, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 75, RewardGold: 60, RewardDropVnums: []uint32{27001, 27002}},
		},
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

func TestCanonicalizeNormalizesStructuredShopPreviewCatalog(t *testing.T) {
	bundle, err := Canonicalize(Bundle{InteractionDefinitions: []interactionstore.Definition{{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:merchant",
		Title: "Village Merchant",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
			{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
		},
	}}})
	if err != nil {
		t.Fatalf("canonicalize structured shop preview bundle: %v", err)
	}
	want := Bundle{InteractionDefinitions: []interactionstore.Definition{testMerchantCatalogDefinition()}}
	if !reflect.DeepEqual(bundle, want) {
		t.Fatalf("unexpected canonical structured shop preview bundle:\n got: %#v\nwant: %#v", bundle, want)
	}
}

func TestCanonicalizeMerchantBundleKeepsStableBuySlotAddressing(t *testing.T) {
	bundle, err := Canonicalize(Bundle{
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

func TestCanonicalizeRejectsInvalidWarpInteractionDefinition(t *testing.T) {
	_, err := Canonicalize(Bundle{
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", X: 1700, Y: 2800}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for invalid warp interaction definition, got %v", err)
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

func TestCanonicalizeKeepsSpawnGroupRewardDescriptor(t *testing.T) {
	dropVnums := []uint32{27002, 27001}
	bundle, err := Canonicalize(Bundle{SpawnGroups: []SpawnGroup{{
		Ref:              " practice.reward_mob ",
		Name:             " Reward Mob ",
		MapIndex:         42,
		X:                1775,
		Y:                2875,
		RaceNum:          101,
		CombatProfile:    " training_dummy ",
		RewardExperience: 75,
		RewardGold:       60,
		RewardDropVnums:  dropVnums,
	}}})
	if err != nil {
		t.Fatalf("canonicalize reward spawn group: %v", err)
	}
	want := Bundle{SpawnGroups: []SpawnGroup{{
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
	}}}
	if !reflect.DeepEqual(bundle, want) {
		t.Fatalf("unexpected canonical reward spawn group:\n got: %#v\nwant: %#v", bundle, want)
	}
	dropVnums[0] = 0
	if bundle.SpawnGroups[0].RewardDropVnums[0] != 27001 {
		t.Fatalf("expected reward drop vnums to be cloned, got %#v", bundle.SpawnGroups[0].RewardDropVnums)
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
		t.Fatalf("expected ErrInvalidBundle for spawn group without explicit name, got %v", err)
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

func TestFromSnapshotsSeparatesSpawnGroupsFromStaticActors(t *testing.T) {
	bundle, err := FromSnapshots(
		staticstore.Snapshot{StaticActors: []staticstore.StaticActor{
			{EntityID: 5, Name: "PracticeMobAlpha", MapIndex: 42, X: 1775, Y: 2875, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, SpawnGroupRef: "practice.mob_alpha", RewardExperience: 75, RewardGold: 60, RewardDropVnums: []uint32{27001, 27002}},
			{EntityID: 9, Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"},
		}},
		interactionstore.Snapshot{Definitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."}}},
	)
	if err != nil {
		t.Fatalf("from snapshots with spawn group: %v", err)
	}
	want := Bundle{
		StaticActors:           []StaticActor{{Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"}},
		SpawnGroups:            []SpawnGroup{{Ref: "practice.mob_alpha", Name: "PracticeMobAlpha", MapIndex: 42, X: 1775, Y: 2875, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, RewardExperience: 75, RewardGold: 60, RewardDropVnums: []uint32{27001, 27002}}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."}},
	}
	if !reflect.DeepEqual(bundle, want) {
		t.Fatalf("unexpected bundle with separated spawn groups:\n got: %#v\nwant: %#v", bundle, want)
	}
}
