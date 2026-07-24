package minimal

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameRuntimeExportContentBundleBuildsDeterministicPortableBundle(t *testing.T) {
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{
		{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."},
		{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
		{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", MapIndex: 42, X: 1700, Y: 2800, Text: "Step through the gate."},
	})
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("VillageGuard", 42, 1700, 2800, 20300, interactionstore.KindTalk, "npc:village_guard"); !ok {
		t.Fatal("expected talk static actor registration to succeed")
	}
	if _, ok := runtime.RegisterStaticActor("Blacksmith", 42, 1750, 2850, 20301); !ok {
		t.Fatal("expected plain static actor registration to succeed")
	}

	bundle, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("export content bundle: %v", err)
	}
	want := contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{
			{Name: "Blacksmith", MapIndex: 42, X: 1750, Y: 2850, RaceNum: 20301},
			{Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"},
		},
		InteractionDefinitions: []interactionstore.Definition{
			{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
			{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."},
			{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", MapIndex: 42, X: 1700, Y: 2800, Text: "Step through the gate."},
		},
	}
	if !reflect.DeepEqual(bundle, want) {
		t.Fatalf("unexpected exported content bundle:\n got: %#v\nwant: %#v", bundle, want)
	}
}

func TestGameRuntimeExportContentBundleSummaryReturnsDeterministicCounts(t *testing.T) {
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{
		{Kind: interactionstore.KindInfo, Ref: "lore:unused", Text: "Unused lore kept for later QA."},
		{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."},
	})
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("VillageGuard", 42, 1700, 2800, 20300, interactionstore.KindTalk, "npc:village_guard"); !ok {
		t.Fatal("expected talk static actor registration to succeed")
	}
	if _, ok := runtime.RegisterStaticActor("Blacksmith", 42, 1750, 2850, 20301); !ok {
		t.Fatal("expected plain static actor registration to succeed")
	}

	summary, err := runtime.ExportContentBundleSummary()
	if err != nil {
		t.Fatalf("export content bundle summary: %v", err)
	}
	want := contentbundle.Summary{
		StaticActorCount:                       2,
		InteractableStaticActorCount:           1,
		InteractionDefinitionCount:             2,
		ReferencedInteractionDefinitionCount:   1,
		UnreferencedInteractionDefinitionCount: 1,
		StaticActors: []contentbundle.StaticActor{
			{Name: "Blacksmith", MapIndex: 42, X: 1750, Y: 2850, RaceNum: 20301},
			{Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"},
		},
		InteractionKinds: []contentbundle.InteractionKindSummary{
			{Kind: interactionstore.KindInfo, Count: 1, ReferencedCount: 0, UnreferencedCount: 1},
			{Kind: interactionstore.KindTalk, Count: 1, ReferencedCount: 1, UnreferencedCount: 0},
		},
		InteractionDefinitionPreviews: []contentbundle.InteractionDefinitionPreviewSummary{
			{Kind: interactionstore.KindInfo, Ref: "lore:unused", Preview: "Unused lore kept for later QA."},
			{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Preview: "Keep your blade sharp."},
		},
		ReferencedInteractionDefinitions: []contentbundle.InteractionDefinitionReferenceSummary{
			{Kind: interactionstore.KindTalk, Ref: "npc:village_guard"},
		},
		UnreferencedInteractionDefinitions: []contentbundle.InteractionDefinitionReferenceSummary{
			{Kind: interactionstore.KindInfo, Ref: "lore:unused"},
		},
		InteractableStaticActors: []contentbundle.InteractableStaticActorSummary{
			{Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard", Preview: "VillageGuard:\nKeep your blade sharp."},
		},
		Maps: []contentbundle.MapContentSummary{{MapIndex: 42, StaticActorCount: 2, InteractableStaticActorCount: 1}},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected exported content bundle summary:\n got: %#v\nwant: %#v", summary, want)
	}
}

func TestGameRuntimeExportContentBundleSummaryIncludesSpawnGroupDetails(t *testing.T) {
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.summary_mob",
		Name:          "SummaryMob",
		MapIndex:      42,
		X:             1800,
		Y:             2900,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}})
	if err != nil {
		t.Fatalf("import spawn-group content bundle: %v", err)
	}

	summary, err := runtime.ExportContentBundleSummary()
	if err != nil {
		t.Fatalf("export content bundle summary: %v", err)
	}
	want := []contentbundle.SpawnGroupReferenceSummary{
		{Ref: "practice.summary_mob", Name: "SummaryMob", MapIndex: 42, CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy)},
	}
	if !reflect.DeepEqual(summary.SpawnGroups, want) {
		t.Fatalf("unexpected runtime summary spawn groups:\n got: %#v\nwant: %#v", summary.SpawnGroups, want)
	}
}

func TestGameRuntimeExportContentBundleSummaryIncludesPortableCombatProfiles(t *testing.T) {
	const profile = "practice_summary_profile"
	worldruntime.UnregisterStaticActorCombatProfileForTest(profile)
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	_, err = runtime.ImportContentBundle(contentbundle.Bundle{
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.summary_profile_mob",
			Name:          "SummaryProfileMob",
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
			Level:                 4,
			Rank:                  1,
			RespawnDelayMs:        1500,
		}},
	})
	if err != nil {
		t.Fatalf("import spawn-group content bundle with combat profile: %v", err)
	}

	summary, err := runtime.ExportContentBundleSummary()
	if err != nil {
		t.Fatalf("export content bundle summary with combat profile: %v", err)
	}
	want := []worldruntime.StaticActorCombatProfileSnapshot{{
		Profile:               profile,
		MaxHP:                 24,
		DamagePerNormalAttack: 3,
		AttackValue:           7,
		DefenseValue:          4,
		Level:                 4,
		Rank:                  1,
		RespawnDelayMs:        1500,
	}}
	if !reflect.DeepEqual(summary.CombatProfiles, want) {
		t.Fatalf("unexpected runtime summary combat profiles:\n got: %#v\nwant: %#v", summary.CombatProfiles, want)
	}
}

func TestGameRuntimeImportContentBundleReplacesRuntimeStateAndPersistsStores(t *testing.T) {
	staticPath := t.TempDir() + "/static-actors.json"
	staticActorStore := staticstore.NewFileStore(staticPath)
	interactionPath := t.TempDir() + "/interaction-definitions.json"
	interactionStore := interactionstore.NewFileStore(interactionPath)
	if err := interactionStore.Save(interactionstore.Snapshot{Definitions: []interactionstore.Definition{{Kind: interactionstore.KindInfo, Ref: "old:lore", Text: "Old lore."}}}); err != nil {
		t.Fatalf("save old interaction definitions: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("OldGuard", 1, 1200, 2200, 20300, interactionstore.KindInfo, "old:lore"); !ok {
		t.Fatal("expected old static actor registration to succeed")
	}

	imported, err := runtime.ImportContentBundle(contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."}},
	})
	if err != nil {
		t.Fatalf("import content bundle: %v", err)
	}
	wantBundle := contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."}},
	}
	if !reflect.DeepEqual(imported, wantBundle) {
		t.Fatalf("unexpected imported bundle:\n got: %#v\nwant: %#v", imported, wantBundle)
	}
	if bundle, err := runtime.ExportContentBundle(); err != nil {
		t.Fatalf("re-export content bundle: %v", err)
	} else if !reflect.DeepEqual(bundle, wantBundle) {
		t.Fatalf("unexpected re-exported content bundle:\n got: %#v\nwant: %#v", bundle, wantBundle)
	}
	persistedDefs, err := interactionStore.Load()
	if err != nil {
		t.Fatalf("load persisted interaction definitions: %v", err)
	}
	if !reflect.DeepEqual(persistedDefs, interactionstore.Snapshot{Definitions: wantBundle.InteractionDefinitions}) {
		t.Fatalf("unexpected persisted interaction definitions after import:\n got: %#v\nwant: %#v", persistedDefs, interactionstore.Snapshot{Definitions: wantBundle.InteractionDefinitions})
	}
	persistedActors, err := staticActorStore.Load()
	if err != nil {
		t.Fatalf("load persisted static actors: %v", err)
	}
	if len(persistedActors.StaticActors) != 1 || persistedActors.StaticActors[0].Name != "VillageGuard" || persistedActors.StaticActors[0].EntityID == 0 || persistedActors.StaticActors[0].InteractionRef != "npc:village_guard" {
		t.Fatalf("unexpected persisted static actors after import: %#v", persistedActors)
	}
}

func TestGameRuntimeImportContentBundlePersistsBundledItemTemplates(t *testing.T) {
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemPath := t.TempDir() + "/item-templates.json"
	itemStore := itemcatalog.NewFileStore(itemPath)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	imported, err := runtime.ImportContentBundle(contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "Merchant", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"}},
		ItemTemplates:          defaultMerchantItemTemplates(),
		InteractionDefinitions: []interactionstore.Definition{defaultMerchantCatalogDefinition()},
	})
	if err != nil {
		t.Fatalf("import content bundle with item templates: %v", err)
	}
	wantBundle := contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "Merchant", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"}},
		ItemTemplates:          defaultMerchantItemTemplates(),
		InteractionDefinitions: []interactionstore.Definition{defaultMerchantCatalogDefinition()},
	}
	if !reflect.DeepEqual(imported, wantBundle) {
		t.Fatalf("unexpected imported bundle with item templates:\n got: %#v\nwant: %#v", imported, wantBundle)
	}
	if exported, err := runtime.ExportContentBundle(); err != nil {
		t.Fatalf("export content bundle with item templates: %v", err)
	} else if !reflect.DeepEqual(exported, wantBundle) {
		t.Fatalf("unexpected exported bundle with item templates:\n got: %#v\nwant: %#v", exported, wantBundle)
	}
	persisted, err := itemStore.Load()
	if err != nil {
		t.Fatalf("load persisted item templates: %v", err)
	}
	if !reflect.DeepEqual(persisted, itemcatalog.Snapshot{Templates: defaultMerchantItemTemplates()}) {
		t.Fatalf("unexpected persisted item templates:\n got: %#v\nwant: %#v", persisted, itemcatalog.Snapshot{Templates: defaultMerchantItemTemplates()})
	}
}

func TestGameRuntimeExportContentBundleSummaryIncludesItemTemplateDetails(t *testing.T) {
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := itemcatalog.NewFileStore(t.TempDir() + "/item-templates.json")
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	_, err = runtime.ImportContentBundle(contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "Merchant", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"}},
		ItemTemplates:          defaultMerchantItemTemplates(),
		InteractionDefinitions: []interactionstore.Definition{defaultMerchantCatalogDefinition()},
	})
	if err != nil {
		t.Fatalf("import merchant content bundle: %v", err)
	}

	summary, err := runtime.ExportContentBundleSummary()
	if err != nil {
		t.Fatalf("export content bundle summary with item templates: %v", err)
	}
	want := []contentbundle.ItemTemplateReferenceSummary{
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1},
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
	}
	if !reflect.DeepEqual(summary.ItemTemplates, want) {
		t.Fatalf("unexpected runtime summary item templates:\n got: %#v\nwant: %#v", summary.ItemTemplates, want)
	}
}

func TestGameRuntimeExportContentBundleSummaryIncludesWarpDestinationDetails(t *testing.T) {
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	_, err = runtime.ImportContentBundle(contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{Name: "Teleporter", MapIndex: 42, X: 1850, Y: 2950, RaceNum: 20303, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:teleporter"}},
		InteractionDefinitions: []interactionstore.Definition{
			{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 7, X: 1300, Y: 2300},
			{Kind: interactionstore.KindInfo, Ref: "lore:unused", Text: "Unused lore kept for later QA."},
		},
	})
	if err != nil {
		t.Fatalf("import warp content bundle: %v", err)
	}

	summary, err := runtime.ExportContentBundleSummary()
	if err != nil {
		t.Fatalf("export content bundle summary with warp destination: %v", err)
	}
	want := []contentbundle.WarpDestinationSummary{{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 7, X: 1300, Y: 2300}}
	if summary.WarpDestinationCount != len(want) {
		t.Fatalf("expected %d warp destinations, got %d", len(want), summary.WarpDestinationCount)
	}
	if !reflect.DeepEqual(summary.WarpDestinations, want) {
		t.Fatalf("unexpected runtime summary warp destinations:\n got: %#v\nwant: %#v", summary.WarpDestinations, want)
	}
}

func TestGameRuntimeImportContentBundleRejectsDanglingInteractionReference(t *testing.T) {
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{StaticActors: []contentbundle.StaticActor{{Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"}}}); !errors.Is(err, contentbundle.ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle on dangling interaction ref, got %v", err)
	}
}

func TestGameRuntimeImportContentBundleRejectsInvalidWarpInteractionDefinition(t *testing.T) {
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActor("OldGuard", 1, 1200, 2200, 20300); !ok {
		t.Fatal("expected old static actor registration to succeed")
	}
	previous, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("export previous content bundle: %v", err)
	}

	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", X: 1700, Y: 2800}}}); !errors.Is(err, contentbundle.ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle on invalid warp interaction definition, got %v", err)
	}
	current, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("re-export content bundle after failed import: %v", err)
	}
	if !reflect.DeepEqual(current, previous) {
		t.Fatalf("expected runtime content bundle to remain unchanged after failed warp import:\n got: %#v\nwant: %#v", current, previous)
	}
}

func TestGameRuntimeExportContentBundleIncludesStaticActorCombatProfile(t *testing.T) {
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummy", 42, 1800, 2900, 20350, string(worldruntime.StaticActorCombatProfileTrainingDummy)); !ok {
		t.Fatal("expected training-dummy static actor registration to succeed")
	}

	bundle, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("export content bundle: %v", err)
	}
	want := contentbundle.Bundle{StaticActors: []contentbundle.StaticActor{{Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy)}}}
	if !reflect.DeepEqual(bundle, want) {
		t.Fatalf("unexpected exported combat-profile content bundle:\n got: %#v\nwant: %#v", bundle, want)
	}
}

func TestGameRuntimeRegisterStaticActorReturnsCombatProfileSnapshot(t *testing.T) {
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	actor, ok := runtime.RegisterStaticActorWithInteractionAndCombatProfile("TrainingDummy", 42, 1800, 2900, 20350, "", "", string(worldruntime.StaticActorCombatProfileTrainingDummy))
	if !ok {
		t.Fatal("expected combat-profile static actor registration to succeed")
	}
	if actor.CombatProfile != string(worldruntime.StaticActorCombatProfileTrainingDummy) {
		t.Fatalf("expected registration snapshot to preserve combat profile, got %#v", actor)
	}
}

func TestGameRuntimeImportContentBundlePreservesCombatProfileActors(t *testing.T) {
	staticPath := t.TempDir() + "/static-actors.json"
	staticActorStore := staticstore.NewFileStore(staticPath)
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	wantBundle := contentbundle.Bundle{StaticActors: []contentbundle.StaticActor{{Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy)}}}
	imported, err := runtime.ImportContentBundle(wantBundle)
	if err != nil {
		t.Fatalf("import combat-profile content bundle: %v", err)
	}
	if !reflect.DeepEqual(imported, wantBundle) {
		t.Fatalf("unexpected imported combat-profile bundle:\n got: %#v\nwant: %#v", imported, wantBundle)
	}
	if bundle, err := runtime.ExportContentBundle(); err != nil {
		t.Fatalf("re-export combat-profile content bundle: %v", err)
	} else if !reflect.DeepEqual(bundle, wantBundle) {
		t.Fatalf("unexpected re-exported combat-profile bundle:\n got: %#v\nwant: %#v", bundle, wantBundle)
	}
	persistedActors, err := staticActorStore.Load()
	if err != nil {
		t.Fatalf("load persisted static actors: %v", err)
	}
	if len(persistedActors.StaticActors) != 1 || persistedActors.StaticActors[0].Name != "TrainingDummy" || persistedActors.StaticActors[0].EntityID == 0 || persistedActors.StaticActors[0].CombatProfile != string(worldruntime.StaticActorCombatProfileTrainingDummy) {
		t.Fatalf("unexpected persisted combat-profile static actors after import: %#v", persistedActors)
	}
}

func TestGameRuntimeImportContentBundleMaterializesSpawnGroupsAsAttackablePracticeMobs(t *testing.T) {
	staticPath := t.TempDir() + "/static-actors.json"
	staticActorStore := staticstore.NewFileStore(staticPath)
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := itemcatalog.NewFileStore(t.TempDir() + "/item-templates.json")
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	wantBundle := contentbundle.Bundle{
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:              "practice.mob_alpha",
			Name:             "PracticeMobAlpha",
			MapIndex:         42,
			X:                1800,
			Y:                2900,
			RaceNum:          101,
			CombatProfile:    string(worldruntime.StaticActorCombatProfileTrainingDummy),
			RewardExperience: 75,
			RewardGold:       60,
			RewardDropVnums:  []uint32{27001},
		}},
		ItemTemplates: []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}},
	}
	imported, err := runtime.ImportContentBundle(wantBundle)
	if err != nil {
		t.Fatalf("import spawn-group content bundle: %v", err)
	}
	if !reflect.DeepEqual(imported, wantBundle) {
		t.Fatalf("unexpected imported spawn-group bundle:\n got: %#v\nwant: %#v", imported, wantBundle)
	}
	if bundle, err := runtime.ExportContentBundle(); err != nil {
		t.Fatalf("re-export spawn-group content bundle: %v", err)
	} else if !reflect.DeepEqual(bundle, wantBundle) {
		t.Fatalf("unexpected re-exported spawn-group bundle:\n got: %#v\nwant: %#v", bundle, wantBundle)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 || actors[0].Name != "PracticeMobAlpha" || actors[0].SpawnGroupRef != "practice.mob_alpha" || actors[0].CombatProfile != string(worldruntime.StaticActorCombatProfileTrainingDummy) || actors[0].RewardExperience != 75 || actors[0].RewardGold != 60 || !reflect.DeepEqual(actors[0].RewardDropVnums, []uint32{27001}) {
		t.Fatalf("unexpected runtime practice-mob actors after import: %#v", actors)
	}
	if actor, ok := runtime.sharedWorld.entities.StaticActor(actors[0].EntityID); !ok || actor.SpawnGroupRef != "practice.mob_alpha" || actor.CombatProfile != string(worldruntime.StaticActorCombatProfileTrainingDummy) || actor.DeathReward.Experience != 75 || actor.DeathReward.Gold != 60 || !reflect.DeepEqual(actor.DeathReward.DropVnums, []uint32{27001}) {
		t.Fatalf("expected runtime entity to preserve spawn-group combat/reward metadata, got actor=%+v ok=%v", actor, ok)
	}
	persistedActors, err := staticActorStore.Load()
	if err != nil {
		t.Fatalf("load persisted spawn-group actors: %v", err)
	}
	if len(persistedActors.StaticActors) != 1 || persistedActors.StaticActors[0].SpawnGroupRef != "practice.mob_alpha" || persistedActors.StaticActors[0].CombatProfile != string(worldruntime.StaticActorCombatProfileTrainingDummy) || persistedActors.StaticActors[0].RewardExperience != 75 || persistedActors.StaticActors[0].RewardGold != 60 || !reflect.DeepEqual(persistedActors.StaticActors[0].RewardDropVnums, []uint32{27001}) {
		t.Fatalf("unexpected persisted spawn-group actors after import: %#v", persistedActors)
	}
}

func TestGameRuntimeImportContentBundleRejectsInvalidSpawnGroupWithoutMutatingRuntime(t *testing.T) {
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	initial := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      42,
		X:             1800,
		Y:             2900,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(initial); err != nil {
		t.Fatalf("import initial spawn-group content bundle: %v", err)
	}
	previous, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("export previous content bundle: %v", err)
	}

	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:             "practice.invalid_reward_mob",
		Name:            "InvalidRewardMob",
		MapIndex:        42,
		X:               1810,
		Y:               2910,
		RaceNum:         102,
		CombatProfile:   string(worldruntime.StaticActorCombatProfileTrainingDummy),
		RewardDropVnums: []uint32{0},
	}}})
	if !errors.Is(err, contentbundle.ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for invalid spawn-group reward descriptor, got %v", err)
	}
	current, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("re-export content bundle after failed invalid spawn-group import: %v", err)
	}
	if !reflect.DeepEqual(current, previous) {
		t.Fatalf("expected runtime content bundle to remain unchanged after invalid spawn-group import:\n got: %#v\nwant: %#v", current, previous)
	}
}

func TestGameRuntimeImportContentBundleRejectsRewardDropsWithoutBundledItemTemplates(t *testing.T) {
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	initial := contentbundle.Bundle{StaticActors: []contentbundle.StaticActor{{Name: "VillageGuide", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300}}}
	if _, err := runtime.ImportContentBundle(initial); err != nil {
		t.Fatalf("import initial content bundle: %v", err)
	}
	previous, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("export previous content bundle: %v", err)
	}

	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:             "practice.reward_mob",
		Name:            "Reward Mob",
		MapIndex:        42,
		X:               1785,
		Y:               2885,
		RaceNum:         101,
		CombatProfile:   string(worldruntime.StaticActorCombatProfilePracticeMob),
		RewardDropVnums: []uint32{27001},
	}}})
	if !errors.Is(err, contentbundle.ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for reward-drop import without bundled item templates, got %v", err)
	}
	current, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("re-export content bundle after failed reward-drop import: %v", err)
	}
	if !reflect.DeepEqual(current, previous) {
		t.Fatalf("expected runtime content bundle to remain unchanged after reward-drop import without item templates:\n got: %#v\nwant: %#v", current, previous)
	}
}

func TestGameRuntimeImportContentBundleRejectsDuplicateSpawnGroupRefsWithoutMutatingRuntime(t *testing.T) {
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      42,
		X:             1800,
		Y:             2900,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import initial spawn-group content bundle: %v", err)
	}
	previous, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("export previous content bundle: %v", err)
	}

	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{
		{Ref: "practice.mob_alpha", Name: "PracticeMobAlpha", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy)},
		{Ref: "practice.mob_alpha", Name: "PracticeMobBeta", MapIndex: 42, X: 1810, Y: 2910, RaceNum: 102, CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy)},
	}})
	if !errors.Is(err, contentbundle.ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for duplicate spawn-group refs, got %v", err)
	}
	current, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("re-export content bundle after failed duplicate spawn-group import: %v", err)
	}
	if !reflect.DeepEqual(current, previous) {
		t.Fatalf("expected runtime content bundle to remain unchanged after duplicate spawn-group import:\n got: %#v\nwant: %#v", current, previous)
	}
}

func TestGameRuntimeImportContentBundleRejectsUnreferencedCombatProfileWithoutRegisteringIt(t *testing.T) {
	const profile = "practice_unreferenced_import_wolf"
	worldruntime.UnregisterStaticActorCombatProfileForTest(profile)
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	_, err = runtime.ImportContentBundle(contentbundle.Bundle{CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
		Profile:               profile,
		MaxHP:                 24,
		DamagePerNormalAttack: 3,
		AttackValue:           7,
		DefenseValue:          4,
		RespawnDelayMs:        1500,
	}}})
	if !errors.Is(err, contentbundle.ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for unreferenced imported combat profile, got %v", err)
	}
	if worldruntime.ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected failed unreferenced combat-profile import not to register profile %q", profile)
	}
	if bundle, err := runtime.ExportContentBundle(); err != nil {
		t.Fatalf("export content bundle after rejected unreferenced profile: %v", err)
	} else if !reflect.DeepEqual(bundle, contentbundle.Bundle{}) {
		t.Fatalf("expected rejected unreferenced profile import not to mutate runtime, got %#v", bundle)
	}
}

func TestGameRuntimeImportContentBundleRollsBackImportedCombatProfileWhenPreviousExportFails(t *testing.T) {
	const profile = "practice_export_preflight_wolf"
	worldruntime.UnregisterStaticActorCombatProfileForTest(profile)
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	runtime.interactionDefinitionMu.Lock()
	if runtime.interactionDefinitions == nil {
		runtime.interactionDefinitions = make(map[string]interactionstore.Definition)
	}
	runtime.interactionDefinitions[interactionDefinitionKey(interactionstore.KindShopPreview, "npc:merchant")] = defaultMerchantCatalogDefinition()
	runtime.interactionDefinitionMu.Unlock()

	_, err = runtime.ImportContentBundle(contentbundle.Bundle{
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.export_preflight_wolf",
			Name:          "ExportPreflightWolf",
			MapIndex:      42,
			X:             1810,
			Y:             2910,
			RaceNum:       101,
			CombatProfile: profile,
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:        profile,
			MaxHP:          24,
			AttackValue:    7,
			DefenseValue:   4,
			RespawnDelayMs: 1500,
		}},
	})
	if err == nil {
		t.Fatal("expected content bundle import to fail while exporting the previous invalid runtime snapshot")
	}
	if worldruntime.ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected failed previous-export preflight not to leak registered combat profile %q", profile)
	}
	if actors := runtime.StaticActors(); len(actors) != 0 {
		t.Fatalf("expected failed previous-export preflight not to materialize imported actors, got %+v", actors)
	}
}

func TestGameRuntimeImportContentBundleRejectsDuplicateCombatProfilesWithoutMutatingRuntime(t *testing.T) {
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	initial := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      42,
		X:             1800,
		Y:             2900,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(initial); err != nil {
		t.Fatalf("import initial spawn-group content bundle: %v", err)
	}
	previous, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("export previous content bundle: %v", err)
	}

	_, err = runtime.ImportContentBundle(contentbundle.Bundle{
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.mob_beta",
			Name:          "PracticeMobBeta",
			MapIndex:      42,
			X:             1810,
			Y:             2910,
			RaceNum:       102,
			CombatProfile: "wolf_profile",
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{
			{Profile: "wolf_profile", MaxHP: 5, DamagePerNormalAttack: 1, AttackValue: 2, DefenseValue: 1, Level: 2, RespawnDelayMs: 2000},
			{Profile: "wolf_profile", MaxHP: 5, DamagePerNormalAttack: 1, AttackValue: 2, DefenseValue: 1, Level: 2, RespawnDelayMs: 2000},
		},
	})
	if !errors.Is(err, contentbundle.ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for duplicate combat-profile snapshots, got %v", err)
	}
	if worldruntime.ValidStaticActorCombatProfile("wolf_profile") {
		worldruntime.UnregisterStaticActorCombatProfileForTest("wolf_profile")
		t.Fatal("expected failed duplicate combat-profile import not to register profile")
	}
	current, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("re-export content bundle after failed duplicate combat-profile import: %v", err)
	}
	if !reflect.DeepEqual(current, previous) {
		t.Fatalf("expected runtime content bundle to remain unchanged after duplicate combat-profile import:\n got: %#v\nwant: %#v", current, previous)
	}
}

type failOnSaveStaticActorStore struct {
	delegate   staticstore.Store
	failOnSave int
	saveCalls  int
	err        error
}

func (s *failOnSaveStaticActorStore) Load() (staticstore.Snapshot, error) {
	if s == nil || s.delegate == nil {
		return staticstore.Snapshot{}, staticstore.ErrSnapshotNotFound
	}
	return s.delegate.Load()
}

func (s *failOnSaveStaticActorStore) Save(snapshot staticstore.Snapshot) error {
	if s == nil || s.delegate == nil {
		return staticstore.ErrStorePathRequired
	}
	s.saveCalls++
	if s.failOnSave > 0 && s.saveCalls == s.failOnSave {
		if s.err != nil {
			return s.err
		}
		return errors.New("static actor save failed")
	}
	return s.delegate.Save(snapshot)
}

func TestGameRuntimeImportContentBundleRestoresPreviousContentWhenStaticActorPersistenceFails(t *testing.T) {
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	initial := contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "VillageGuide", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Welcome."}},
	}
	if _, err := runtime.ImportContentBundle(initial); err != nil {
		t.Fatalf("import initial content bundle: %v", err)
	}
	previous, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("export previous content bundle: %v", err)
	}

	runtime.staticStore = &failingStaticActorStore{}
	_, err = runtime.ImportContentBundle(contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "Blacksmith", MapIndex: 42, X: 1750, Y: 2850, RaceNum: 20301}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindInfo, Ref: "lore:forge", Text: "The forge is cold."}},
	})
	if err == nil {
		t.Fatal("expected content bundle import to fail when static actor persistence fails")
	}

	runtime.staticStore = staticActorStore
	current, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("re-export content bundle after failed persistence import: %v", err)
	}
	if !reflect.DeepEqual(current, previous) {
		t.Fatalf("expected runtime content bundle to restore previous content after static actor persistence failure:\n got: %#v\nwant: %#v", current, previous)
	}
	persisted, err := staticActorStore.Load()
	if err != nil {
		t.Fatalf("load restored static actor snapshot: %v", err)
	}
	if len(persisted.StaticActors) != 1 || persisted.StaticActors[0].Name != "VillageGuide" || persisted.StaticActors[0].InteractionRef != "npc:guide" {
		t.Fatalf("expected previous static actor snapshot to be restored after failed import, got %#v", persisted)
	}
}

func TestGameRuntimeImportContentBundleFlushesStaticActorReplacementFanoutAfterSuccess(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	player := peerVisibilityCharacter("BundleWatcher", 0x01036002, 0x02046002, 1800, 2900, 0, 101, 201)
	player.MapIndex = 42
	issuePeerTicket(t, store, "bundle-watcher", 0x60600202, player)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	initial := contentbundle.Bundle{StaticActors: []contentbundle.StaticActor{{Name: "OldGuide", MapIndex: 42, X: 1810, Y: 2910, RaceNum: 20300}}}
	if _, err := runtime.ImportContentBundle(initial); err != nil {
		t.Fatalf("import initial static-actor content bundle: %v", err)
	}
	oldActors := runtime.StaticActors()
	if len(oldActors) != 1 {
		t.Fatalf("expected one initial actor before replacement import, got %+v", oldActors)
	}

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "bundle-watcher", 0x60600202)
	defer closeSessionFlow(t, flow)
	if len(enterOut) != 8 {
		t.Fatalf("expected self bootstrap plus initial static actor visibility, got %d frames", len(enterOut))
	}
	flushServerFrames(t, flow)

	_, err = runtime.ImportContentBundle(contentbundle.Bundle{StaticActors: []contentbundle.StaticActor{{Name: "NewGuide", MapIndex: 42, X: 1820, Y: 2920, RaceNum: 20301}}})
	if err != nil {
		t.Fatalf("replace static actor content bundle: %v", err)
	}
	queued := flushServerFrames(t, flow)
	if len(queued) != 4 {
		t.Fatalf("expected old actor delete plus new actor bootstrap after successful import, got %d frames", len(queued))
	}
	oldDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode old actor delete after successful replacement import: %v", err)
	}
	if oldDelete.VID != uint32(oldActors[0].EntityID) {
		t.Fatalf("unexpected old actor delete after successful replacement import: %+v", oldDelete)
	}
	newAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, queued[1]))
	if err != nil {
		t.Fatalf("decode new actor add after successful replacement import: %v", err)
	}
	if newAdd.Type != 1 || newAdd.X != 1820 || newAdd.Y != 2920 || newAdd.RaceNum != 20301 {
		t.Fatalf("unexpected new actor add after successful replacement import: %+v", newAdd)
	}
	newInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, queued[2]))
	if err != nil {
		t.Fatalf("decode new actor info after successful replacement import: %v", err)
	}
	if newInfo.VID != newAdd.VID || newInfo.Name != "NewGuide" {
		t.Fatalf("unexpected new actor info after successful replacement import: %+v add=%+v", newInfo, newAdd)
	}
	newUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, queued[3]))
	if err != nil {
		t.Fatalf("decode new actor update after successful replacement import: %v", err)
	}
	if newUpdate.VID != newAdd.VID {
		t.Fatalf("unexpected new actor update after successful replacement import: %+v add=%+v", newUpdate, newAdd)
	}
}

func TestGameRuntimeImportContentBundleDoesNotLeakStaticActorFanoutWhenReplacementPersistenceFails(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	player := peerVisibilityCharacter("BundleWatcher", 0x01036001, 0x02046001, 1800, 2900, 0, 101, 201)
	player.MapIndex = 42
	issuePeerTicket(t, store, "bundle-watcher", 0x60600101, player)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	initial := contentbundle.Bundle{StaticActors: []contentbundle.StaticActor{{Name: "OldGuide", MapIndex: 42, X: 1810, Y: 2910, RaceNum: 20300}}}
	if _, err := runtime.ImportContentBundle(initial); err != nil {
		t.Fatalf("import initial static-actor content bundle: %v", err)
	}
	previous, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("export previous content bundle: %v", err)
	}

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "bundle-watcher", 0x60600101)
	defer closeSessionFlow(t, flow)
	if len(enterOut) != 8 {
		t.Fatalf("expected self bootstrap plus initial static actor visibility, got %d frames", len(enterOut))
	}
	flushServerFrames(t, flow)

	runtime.staticStore = &failOnSaveStaticActorStore{delegate: staticActorStore, failOnSave: 3, err: errors.New("static actor rollback persistence failure")}
	_, err = runtime.ImportContentBundle(contentbundle.Bundle{StaticActors: []contentbundle.StaticActor{
		{Name: "ImportedGuide", MapIndex: 42, X: 1820, Y: 2920, RaceNum: 20301},
		{Name: "ImportedSmith", MapIndex: 42, X: 1830, Y: 2930, RaceNum: 20302},
	}})
	if err == nil {
		t.Fatal("expected content bundle import to fail during replacement persistence")
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected failed content-bundle import not to leak delete/add visibility frames, got %d", len(queued))
	}
	current, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("re-export content bundle after failed live import: %v", err)
	}
	if !reflect.DeepEqual(current, previous) {
		t.Fatalf("expected runtime content bundle to restore previous content after failed live import:\n got: %#v\nwant: %#v", current, previous)
	}
}

func TestGameRuntimeUpdateSpawnGroupStaticActorPreservesRewardDescriptor(t *testing.T) {
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := itemcatalog.NewFileStore(t.TempDir() + "/item-templates.json")
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:              "practice.mob_alpha",
			Name:             "PracticeMobAlpha",
			MapIndex:         42,
			X:                1800,
			Y:                2900,
			RaceNum:          101,
			CombatProfile:    string(worldruntime.StaticActorCombatProfileTrainingDummy),
			RewardExperience: 75,
			RewardGold:       60,
			RewardDropVnums:  []uint32{27001},
		}},
		ItemTemplates: []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}},
	}); err != nil {
		t.Fatalf("import spawn-group content bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one imported spawn actor, got %+v", actors)
	}

	updated, ok := runtime.updateStaticActorWithInteractionCombatProfileAndSpawnGroupRef(actors[0].EntityID, "PracticeMobAlphaMoved", 42, 1810, 2910, 101, "", "", string(worldruntime.StaticActorCombatProfileTrainingDummy), "practice.mob_alpha")
	if !ok {
		t.Fatal("expected spawn-group static actor update to succeed")
	}
	if updated.RewardExperience != 75 || updated.RewardGold != 60 || !reflect.DeepEqual(updated.RewardDropVnums, []uint32{27001}) {
		t.Fatalf("expected update result to preserve reward descriptor, got %+v", updated)
	}
	persisted, err := staticActorStore.Load()
	if err != nil {
		t.Fatalf("load persisted static actor snapshot: %v", err)
	}
	if len(persisted.StaticActors) != 1 || persisted.StaticActors[0].RewardExperience != 75 || persisted.StaticActors[0].RewardGold != 60 || !reflect.DeepEqual(persisted.StaticActors[0].RewardDropVnums, []uint32{27001}) {
		t.Fatalf("expected persisted update to preserve reward descriptor, got %+v", persisted)
	}
	reexported, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("re-export content bundle after spawn actor update: %v", err)
	}
	if len(reexported.SpawnGroups) != 1 || reexported.SpawnGroups[0].RewardExperience != 75 || reexported.SpawnGroups[0].RewardGold != 60 || !reflect.DeepEqual(reexported.SpawnGroups[0].RewardDropVnums, []uint32{27001}) {
		t.Fatalf("expected re-export to preserve reward descriptor, got %+v", reexported)
	}
}
