package worldruntime

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestVisiblePeersSupportsEnterAndReconnectOnSameEffectiveMap(t *testing.T) {
	topology := NewBootstrapTopology(1)
	subject := visibilityCharacter("Subject", 0x02040102, 0, 1500, 2600)
	sameMap := visibilityCharacter("SameMap", 0x02040103, 1, 1700, 2800)
	otherMap := visibilityCharacter("OtherMap", 0x02040104, 42, 1900, 3000)

	peers := VisiblePeers(topology, subject, []loginticket.Character{subject, sameMap, otherMap}, subject.VID)
	if len(peers) != 1 {
		t.Fatalf("expected exactly 1 same-map visible peer, got %d", len(peers))
	}
	if peers[0].VID != sameMap.VID || peers[0].MapIndex != sameMap.MapIndex {
		t.Fatalf("unexpected visible peer: %+v", peers[0])
	}
}

func TestDiffVisiblePeersReportsRemovedPeerOnLeave(t *testing.T) {
	current := []loginticket.Character{
		visibilityCharacter("PeerOne", 0x02040101, 1, 1100, 2100),
		visibilityCharacter("PeerTwo", 0x02040102, 1, 1300, 2300),
	}
	target := []loginticket.Character{
		visibilityCharacter("PeerOne", 0x02040101, 1, 1100, 2100),
	}

	removed, added := DiffVisiblePeers(current, target)
	if len(removed) != 1 || removed[0].VID != 0x02040102 {
		t.Fatalf("expected one removed peer with VID 0x02040102, got removed=%+v", removed)
	}
	if len(added) != 0 {
		t.Fatalf("expected no added peers on leave diff, got %+v", added)
	}
}

func TestDiffVisiblePeersReportsRelocationAddsAndRemovals(t *testing.T) {
	current := []loginticket.Character{
		visibilityCharacter("OldPeer", 0x02040101, 1, 1100, 2100),
	}
	target := []loginticket.Character{
		visibilityCharacter("NewPeer", 0x02040103, 42, 1700, 2800),
	}

	removed, added := DiffVisiblePeers(current, target)
	if len(removed) != 1 || removed[0].VID != 0x02040101 {
		t.Fatalf("expected one removed old-map peer, got %+v", removed)
	}
	if len(added) != 1 || added[0].VID != 0x02040103 {
		t.Fatalf("expected one added destination-map peer, got %+v", added)
	}
}

func visibilityCharacter(name string, vid uint32, mapIndex uint32, x int32, y int32) loginticket.Character {
	return loginticket.Character{
		ID:       vid,
		VID:      vid,
		Name:     name,
		MapIndex: mapIndex,
		X:        x,
		Y:        y,
	}
}
