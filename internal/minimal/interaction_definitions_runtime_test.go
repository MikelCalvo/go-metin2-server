package minimal

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestGameRuntimeInteractionDefinitionsReturnsSortedSnapshot(t *testing.T) {
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{
		{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "VillageGuard:\nKeep your blade sharp."},
		{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
	})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	got := runtime.InteractionDefinitions()
	want := []InteractionDefinition{
		{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
		{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "VillageGuard:\nKeep your blade sharp."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected sorted interaction definitions:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestGameRuntimeCreateInteractionDefinitionPersistsSnapshotAndResolvesDefinition(t *testing.T) {
	interactionStore := newInteractionDefinitionStore(t, nil)
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	definition, err := runtime.CreateInteractionDefinition(interactionstore.KindInfo, "lore:alchemist", "The alchemist studies forgotten herbs.")
	if err != nil {
		t.Fatalf("create interaction definition: %v", err)
	}
	if definition.Kind != interactionstore.KindInfo || definition.Ref != "lore:alchemist" || definition.Text != "The alchemist studies forgotten herbs." {
		t.Fatalf("unexpected created interaction definition: %+v", definition)
	}
	resolved, ok := runtime.ResolveInteractionDefinition(interactionstore.KindInfo, "lore:alchemist")
	if !ok || !reflect.DeepEqual(resolved, definition) {
		t.Fatalf("expected created interaction definition to resolve, got definition=%+v ok=%v", resolved, ok)
	}
	persisted, err := interactionStore.Load()
	if err != nil {
		t.Fatalf("load persisted interaction definitions: %v", err)
	}
	want := interactionstore.Snapshot{Definitions: []interactionstore.Definition{{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."}}}
	if !reflect.DeepEqual(persisted, want) {
		t.Fatalf("unexpected persisted interaction definitions:\n got: %#v\nwant: %#v", persisted, want)
	}
}

func TestGameRuntimeCreateInteractionDefinitionRejectsDuplicateDefinition(t *testing.T) {
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."}})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	if _, err := runtime.CreateInteractionDefinition(interactionstore.KindInfo, "lore:alchemist", "Duplicate"); !errors.Is(err, ErrInteractionDefinitionExists) {
		t.Fatalf("expected ErrInteractionDefinitionExists, got %v", err)
	}
}

func TestGameRuntimeUpsertInteractionDefinitionPersistsDefinitionText(t *testing.T) {
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Old text."}})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	definition, err := runtime.UpsertInteractionDefinition(interactionstore.KindTalk, "npc:village_guard", "Keep your blade sharp.")
	if err != nil {
		t.Fatalf("upsert interaction definition: %v", err)
	}
	if definition.Kind != interactionstore.KindTalk || definition.Ref != "npc:village_guard" || definition.Text != "Keep your blade sharp." {
		t.Fatalf("unexpected upserted interaction definition: %+v", definition)
	}
	resolved, ok := runtime.ResolveInteractionDefinition(interactionstore.KindTalk, "npc:village_guard")
	if !ok || !reflect.DeepEqual(resolved, definition) {
		t.Fatalf("expected upserted interaction definition to resolve, got definition=%+v ok=%v", resolved, ok)
	}
	persisted, err := interactionStore.Load()
	if err != nil {
		t.Fatalf("load persisted interaction definitions: %v", err)
	}
	want := interactionstore.Snapshot{Definitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."}}}
	if !reflect.DeepEqual(persisted, want) {
		t.Fatalf("unexpected persisted interaction definitions after upsert:\n got: %#v\nwant: %#v", persisted, want)
	}
}

func TestGameRuntimeRemoveInteractionDefinitionRejectsReferencedDefinition(t *testing.T) {
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."}})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("VillageGuard", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindTalk, "npc:village_guard"); !ok {
		t.Fatal("expected static actor registration with interaction metadata to succeed")
	}

	if _, err := runtime.RemoveInteractionDefinition(interactionstore.KindTalk, "npc:village_guard"); !errors.Is(err, ErrInteractionDefinitionReferenced) {
		t.Fatalf("expected ErrInteractionDefinitionReferenced, got %v", err)
	}
	persisted, err := interactionStore.Load()
	if err != nil {
		t.Fatalf("load persisted interaction definitions: %v", err)
	}
	want := interactionstore.Snapshot{Definitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."}}}
	if !reflect.DeepEqual(persisted, want) {
		t.Fatalf("expected referenced interaction definition snapshot to remain unchanged:\n got: %#v\nwant: %#v", persisted, want)
	}
}

func TestGameRuntimeRemoveInteractionDefinitionPersistsSnapshotOnSuccess(t *testing.T) {
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{
		{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
		{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."},
	})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	definition, err := runtime.RemoveInteractionDefinition(interactionstore.KindInfo, "lore:alchemist")
	if err != nil {
		t.Fatalf("remove interaction definition: %v", err)
	}
	if definition.Kind != interactionstore.KindInfo || definition.Ref != "lore:alchemist" {
		t.Fatalf("unexpected removed interaction definition: %+v", definition)
	}
	if _, ok := runtime.ResolveInteractionDefinition(interactionstore.KindInfo, "lore:alchemist"); ok {
		t.Fatal("expected removed interaction definition to stop resolving")
	}
	persisted, err := interactionStore.Load()
	if err != nil {
		t.Fatalf("load persisted interaction definitions: %v", err)
	}
	want := interactionstore.Snapshot{Definitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."}}}
	if !reflect.DeepEqual(persisted, want) {
		t.Fatalf("unexpected persisted interaction definitions after remove:\n got: %#v\nwant: %#v", persisted, want)
	}
}
