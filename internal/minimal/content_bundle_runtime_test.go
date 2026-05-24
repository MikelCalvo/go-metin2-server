package minimal

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
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
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	wantBundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      42,
		X:             1800,
		Y:             2900,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
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
	if len(actors) != 1 || actors[0].Name != "PracticeMobAlpha" || actors[0].SpawnGroupRef != "practice.mob_alpha" || actors[0].CombatProfile != string(worldruntime.StaticActorCombatProfileTrainingDummy) {
		t.Fatalf("unexpected runtime practice-mob actors after import: %#v", actors)
	}
	persistedActors, err := staticActorStore.Load()
	if err != nil {
		t.Fatalf("load persisted spawn-group actors: %v", err)
	}
	if len(persistedActors.StaticActors) != 1 || persistedActors.StaticActors[0].SpawnGroupRef != "practice.mob_alpha" || persistedActors.StaticActors[0].CombatProfile != string(worldruntime.StaticActorCombatProfileTrainingDummy) {
		t.Fatalf("unexpected persisted spawn-group actors after import: %#v", persistedActors)
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
