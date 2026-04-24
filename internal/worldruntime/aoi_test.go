package worldruntime

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestRadiusVisibilityPolicyAllowsPeersInsideRadius(t *testing.T) {
	topology := NewBootstrapTopology(1).WithVisibilityPolicy(RadiusVisibilityPolicy{Radius: 400, SectorSize: 200})
	subject := visibilityCharacter("Subject", 0x02040101, 42, 1700, 2800)
	nearPeer := visibilityCharacter("NearPeer", 0x02040102, 42, 1900, 2900)

	peers := VisiblePeers(topology, subject, []loginticket.Character{subject, nearPeer}, subject.VID)
	if len(peers) != 1 || peers[0].Name != "NearPeer" {
		t.Fatalf("expected near peer to stay visible inside AOI radius, got %+v", peers)
	}
}

func TestRadiusVisibilityPolicyRejectsPeersOutsideRadius(t *testing.T) {
	topology := NewBootstrapTopology(1).WithVisibilityPolicy(RadiusVisibilityPolicy{Radius: 300, SectorSize: 200})
	subject := visibilityCharacter("Subject", 0x02040101, 42, 1700, 2800)
	farPeer := visibilityCharacter("FarPeer", 0x02040102, 42, 2500, 3600)

	peers := VisiblePeers(topology, subject, []loginticket.Character{subject, farPeer}, subject.VID)
	if len(peers) != 0 {
		t.Fatalf("expected far peer to be filtered out by AOI radius, got %+v", peers)
	}
}

func TestSectorKeyForPositionIsStable(t *testing.T) {
	position := NewPosition(42, 1700, 2800)
	if got := SectorKeyForPosition(position, 200); got != (SectorKey{MapIndex: 42, SX: 8, SY: 14}) {
		t.Fatalf("unexpected sector key for position: %+v", got)
	}
}

func TestSectorKeyForPositionFloorsNegativeCoordinates(t *testing.T) {
	position := NewPosition(42, -1, -201)
	if got := SectorKeyForPosition(position, 200); got != (SectorKey{MapIndex: 42, SX: -1, SY: -2}) {
		t.Fatalf("expected floored negative sector key, got %+v", got)
	}
}
