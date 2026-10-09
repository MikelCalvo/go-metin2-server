package worldruntime

import (
	"reflect"
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

func TestRadiusVisibilityPolicyRequiresSameEffectiveMap(t *testing.T) {
	topology := NewBootstrapTopology(1).WithVisibilityPolicy(RadiusVisibilityPolicy{Radius: 400, SectorSize: 200})
	subject := visibilityCharacter("Subject", 0x02040101, 42, 1700, 2800)
	differentMapPeer := visibilityCharacter("DifferentMapPeer", 0x02040102, 43, 1701, 2801)

	peers := VisiblePeers(topology, subject, []loginticket.Character{subject, differentMapPeer}, subject.VID)
	if len(peers) != 0 {
		t.Fatalf("expected different-map peer to stay hidden even inside AOI radius, got %+v", peers)
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

func TestSectorVisibilityPolicyBucketsFanoutAndEdgeDiff(t *testing.T) {
	topology := NewBootstrapTopology(1).WithSectorVisibilityPolicy(200)
	subject := visibilityCharacter("Subject", 0x02040101, 42, 1700, 2800)
	same := visibilityCharacter("Same", 0x02040102, 42, 1799, 2999)
	adjacent := visibilityCharacter("Adjacent", 0x02040103, 42, 1800, 2800)
	otherMap := visibilityCharacter("OtherMap", 0x02040104, 43, 1700, 2800)
	peers := []loginticket.Character{otherMap, adjacent, same, subject}
	if got := VisiblePeers(topology, subject, peers, subject.VID); !reflect.DeepEqual(got, []loginticket.Character{same}) {
		t.Fatalf("sector should deliver only to same-bucket peer, got %+v", got)
	}
	if !topology.SharesVisibleWorld(same, subject) || topology.SharesVisibleWorld(adjacent, subject) || topology.SharesVisibleWorld(otherMap, subject) {
		t.Fatal("sector visibility must be symmetric and map-scoped")
	}

	moved := subject
	moved.X = 1800
	diff := RelocateVisibilityDiff(topology, subject, peers, moved, peers)
	if !reflect.DeepEqual(diff.RemovedVisiblePeers, []loginticket.Character{same}) || !reflect.DeepEqual(diff.AddedVisiblePeers, []loginticket.Character{adjacent}) {
		t.Fatalf("sector crossing should swap admitted viewers, got %+v", diff)
	}
}

func TestSectorVisibilityPolicyScopesPlayerActorAndGroundItemFanout(t *testing.T) {
	topology := NewBootstrapTopology(1).WithSectorVisibilityPolicy(200)
	registry := NewEntityRegistryWithTopology(topology)
	subject := registry.RegisterPlayer(entityRegistryCharacter("Subject", 0x02040101, 42, 1700, 2800))
	same := registry.RegisterPlayer(entityRegistryCharacter("Same", 0x02040102, 42, 1799, 2999))
	registry.RegisterPlayer(entityRegistryCharacter("Adjacent", 0x02040103, 42, 1800, 2800))
	registry.RegisterPlayer(entityRegistryCharacter("OtherMap", 0x02040104, 43, 1700, 2800))
	actor, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "SameSectorActor"}, Position: NewPosition(42, 1799, 2999), RaceNum: 20300})
	if !ok {
		t.Fatal("failed to register static actor")
	}
	_, ok = registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "AdjacentSectorActor"}, Position: NewPosition(42, 1800, 2800), RaceNum: 20300})
	if !ok {
		t.Fatal("failed to register adjacent static actor")
	}
	scopes := NewScopes(topology, registry)
	if got := scopes.VisibleTargets(subject.Entity.ID, subject.Character); len(got) != 1 || got[0].Entity.ID != same.Entity.ID {
		t.Fatalf("visibility fanout must target only same-sector session, got %+v", got)
	}
	if got := scopes.VisibleTargetsForStaticActor(actor); len(got) != 2 || (got[0].Entity.ID != same.Entity.ID && got[1].Entity.ID != same.Entity.ID) || (got[0].Entity.ID != subject.Entity.ID && got[1].Entity.ID != subject.Entity.ID) {
		t.Fatalf("actor fanout must include subject and same-sector peer, got %+v", got)
	}
	if got := scopes.VisibleStaticActors(subject.Character); len(got) != 1 || got[0].Entity.ID != actor.Entity.ID {
		t.Fatalf("actor visibility must exclude adjacent-sector actor, got %+v", got)
	}
	ground := []GroundItemOccupancy{
		{VID: 51, MapIndex: 42, X: 1799, Y: 2999},
		{VID: 52, MapIndex: 42, X: 1800, Y: 2800},
		{VID: 53, MapIndex: 43, X: 1700, Y: 2800},
	}
	if got := VisibleGroundItems(topology, subject.Character, ground); len(got) != 1 || got[0].VID != 51 {
		t.Fatalf("ground fanout must exclude adjacent sector and map, got %+v", got)
	}
}

func TestSectorVisibilityPolicyNormalizesMapZeroAndFloorsNegativeCoordinates(t *testing.T) {
	topology := NewBootstrapTopology(1).WithSectorVisibilityPolicy(200)
	subject := visibilityCharacter("Subject", 0x02040101, 0, -1, -201)
	same := visibilityCharacter("Same", 0x02040102, 1, -200, -400)
	positiveSide := visibilityCharacter("PositiveSide", 0x02040103, 1, 0, -201)
	negativeEdge := visibilityCharacter("NegativeEdge", 0x02040104, 1, -201, -201)
	if !topology.SharesVisibleWorld(subject, same) || topology.SharesVisibleWorld(subject, positiveSide) || topology.SharesVisibleWorld(subject, negativeEdge) {
		t.Fatal("sector visibility must normalize bootstrap map and floor negative coordinates")
	}
}
