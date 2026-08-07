package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestGameRuntimeCharacterVisibilitySnapshotReturnsExactConnectedCharacter(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	alpha := peerVisibilityCharacter("Alpha", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	beta := peerVisibilityCharacter("Beta", 0x01030102, 0x02040102, 1200, 2200, 0, 102, 202)
	issuePeerTicket(t, store, "alpha", 0x11111111, alpha)
	issuePeerTicket(t, store, "beta", 0x22222222, beta)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActor("VillageGuard", bootstrapMapIndex, 1150, 2150, 20300)
	if !ok {
		t.Fatal("expected static actor registration to succeed")
	}
	factory := runtime.SessionFactory()
	alphaFlow, _ := enterGameWithLoginTicket(t, factory, "alpha", 0x11111111)
	defer closeSessionFlow(t, alphaFlow)
	betaFlow, _ := enterGameWithLoginTicket(t, factory, "beta", 0x22222222)
	defer closeSessionFlow(t, betaFlow)

	snapshot, ok := runtime.CharacterVisibilitySnapshot("Alpha")
	if !ok {
		t.Fatal("expected exact Alpha visibility snapshot to resolve")
	}
	if snapshot.Name != "Alpha" || snapshot.VID != alpha.VID || snapshot.MapIndex != bootstrapMapIndex {
		t.Fatalf("unexpected Alpha visibility subject snapshot: %+v", snapshot)
	}
	if len(snapshot.VisiblePeers) != 1 || snapshot.VisiblePeers[0].Name != "Beta" || snapshot.VisiblePeers[0].VID != beta.VID {
		t.Fatalf("expected exact Alpha visibility snapshot to include Beta peer, got %+v", snapshot.VisiblePeers)
	}
	if len(snapshot.VisibleStaticActors) != 1 || snapshot.VisibleStaticActors[0].EntityID != actor.EntityID || snapshot.VisibleStaticActors[0].Name != "VillageGuard" {
		t.Fatalf("expected exact Alpha visibility snapshot to include VillageGuard, got %+v", snapshot.VisibleStaticActors)
	}

	if missing, ok := runtime.CharacterVisibilitySnapshot("Missing"); ok || missing.Name != "" {
		t.Fatalf("expected missing exact visibility snapshot to fail closed, got snapshot=%+v ok=%v", missing, ok)
	}
}
