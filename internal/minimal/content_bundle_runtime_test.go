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
)

func TestGameRuntimeExportContentBundleBuildsDeterministicPortableBundle(t *testing.T) {
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{
		{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."},
		{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
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
