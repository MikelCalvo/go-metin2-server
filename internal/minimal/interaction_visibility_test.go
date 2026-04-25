package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestGameRuntimeInteractionVisibilityReturnsResolvedPreviewsForVisibleInteractables(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{
		{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
		{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."},
	})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("VillageGuard", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindTalk, "npc:village_guard"); !ok {
		t.Fatal("expected talk static actor registration to succeed")
	}
	if _, ok := runtime.RegisterStaticActor("Blacksmith", bootstrapMapIndex, 1250, 2250, 20301); !ok {
		t.Fatal("expected non-interactable static actor registration to succeed")
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("Alchemist", bootstrapMapIndex, 1300, 2300, 20302, interactionstore.KindInfo, "lore:alchemist"); !ok {
		t.Fatal("expected info static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 interaction visibility snapshot, got %+v", snapshots)
	}
	if snapshots[0].Name != "PeerOne" {
		t.Fatalf("expected PeerOne interaction visibility subject, got %+v", snapshots[0])
	}
	if len(snapshots[0].VisibleInteractableStaticActors) != 2 {
		t.Fatalf("expected 2 visible interactable static actors, got %+v", snapshots[0].VisibleInteractableStaticActors)
	}
	if snapshots[0].VisibleInteractableStaticActors[0].Name != "Alchemist" || snapshots[0].VisibleInteractableStaticActors[0].Preview != "The alchemist studies forgotten herbs." {
		t.Fatalf("unexpected info interaction preview snapshot: %+v", snapshots[0].VisibleInteractableStaticActors[0])
	}
	if snapshots[0].VisibleInteractableStaticActors[1].Name != "VillageGuard" || snapshots[0].VisibleInteractableStaticActors[1].Preview != "VillageGuard:\nKeep your blade sharp." {
		t.Fatalf("unexpected talk interaction preview snapshot: %+v", snapshots[0].VisibleInteractableStaticActors[1])
	}
}

func TestGameRuntimeInteractionVisibilityReportsResolutionFailureForDanglingDefinition(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, nil)

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.sharedWorld.RegisterStaticActorWithInteraction(0, "Alchemist", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindInfo, "lore:missing"); !ok {
		t.Fatal("expected direct shared-world registration with dangling ref to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 || len(snapshots[0].VisibleInteractableStaticActors) != 1 {
		t.Fatalf("expected one dangling-definition interaction visibility entry, got %+v", snapshots)
	}
	entry := snapshots[0].VisibleInteractableStaticActors[0]
	if entry.Name != "Alchemist" || entry.Preview != "" || entry.ResolutionFailure != staticActorInteractionFailureDefinitionNotFound {
		t.Fatalf("unexpected dangling-definition interaction visibility snapshot: %+v", entry)
	}
}
