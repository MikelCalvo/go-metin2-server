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
			DeathReward:           worldruntime.StaticActorDeathReward{Experience: 25, Gold: 11, DropVnums: []uint32{27001, 27002}},
		}},
	}
	if !reflect.DeepEqual(bundle, want) {
		t.Fatalf("unexpected canonical custom combat profile bundle:\n got: %#v\nwant: %#v", bundle, want)
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
