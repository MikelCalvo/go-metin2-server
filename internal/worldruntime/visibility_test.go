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

func TestVisiblePeersUsesConfiguredVisibilityPolicyBoundary(t *testing.T) {
	topology := NewBootstrapTopology(1).WithVisibilityPolicy(sameEmpireVisibilityPolicy{})
	subject := visibilityCharacter("Subject", 0x02040102, 1, 1500, 2600)
	subject.Empire = 2
	otherMapSameEmpire := visibilityCharacter("OtherMapSameEmpire", 0x02040103, 42, 1700, 2800)
	otherMapSameEmpire.Empire = 2
	sameMapOtherEmpire := visibilityCharacter("SameMapOtherEmpire", 0x02040104, 1, 1900, 3000)
	sameMapOtherEmpire.Empire = 3

	peers := VisiblePeers(topology, subject, []loginticket.Character{subject, sameMapOtherEmpire, otherMapSameEmpire}, subject.VID)
	if len(peers) != 1 || peers[0].Name != "OtherMapSameEmpire" {
		t.Fatalf("expected configured policy to allow cross-map same-empire peer only, got %+v", peers)
	}
}

func TestEnterVisibilityDiffReportsSelfFacingAddedPeers(t *testing.T) {
	topology := NewBootstrapTopology(1)
	subject := visibilityCharacter("Subject", 0x02040102, 1, 1500, 2600)
	peerAlpha := visibilityCharacter("PeerAlpha", 0x02040103, 1, 1700, 2800)
	peerZulu := visibilityCharacter("PeerZulu", 0x02040104, 1, 1900, 3000)
	otherMap := visibilityCharacter("OtherMap", 0x02040105, 42, 2100, 3200)

	diff := EnterVisibilityDiff(topology, subject, []loginticket.Character{peerZulu, otherMap, peerAlpha})
	if len(diff.CurrentVisiblePeers) != 0 {
		t.Fatalf("expected no current visible peers before enter, got %+v", diff.CurrentVisiblePeers)
	}
	if len(diff.TargetVisiblePeers) != 2 || diff.TargetVisiblePeers[0].Name != "PeerAlpha" || diff.TargetVisiblePeers[1].Name != "PeerZulu" {
		t.Fatalf("unexpected target visible peers on enter: %+v", diff.TargetVisiblePeers)
	}
	if len(diff.RemovedVisiblePeers) != 0 {
		t.Fatalf("expected no removed visible peers on enter, got %+v", diff.RemovedVisiblePeers)
	}
	if len(diff.AddedVisiblePeers) != 2 || diff.AddedVisiblePeers[0].Name != "PeerAlpha" || diff.AddedVisiblePeers[1].Name != "PeerZulu" {
		t.Fatalf("unexpected added visible peers on enter: %+v", diff.AddedVisiblePeers)
	}
}

func TestLeaveVisibilityDiffReportsSelfFacingRemovedPeers(t *testing.T) {
	topology := NewBootstrapTopology(1)
	subject := visibilityCharacter("Subject", 0x02040102, 1, 1500, 2600)
	peerAlpha := visibilityCharacter("PeerAlpha", 0x02040103, 1, 1700, 2800)
	peerZulu := visibilityCharacter("PeerZulu", 0x02040104, 1, 1900, 3000)
	otherMap := visibilityCharacter("OtherMap", 0x02040105, 42, 2100, 3200)

	diff := LeaveVisibilityDiff(topology, subject, []loginticket.Character{subject, peerZulu, otherMap, peerAlpha})
	if len(diff.CurrentVisiblePeers) != 2 || diff.CurrentVisiblePeers[0].Name != "PeerAlpha" || diff.CurrentVisiblePeers[1].Name != "PeerZulu" {
		t.Fatalf("unexpected current visible peers on leave: %+v", diff.CurrentVisiblePeers)
	}
	if len(diff.TargetVisiblePeers) != 0 {
		t.Fatalf("expected no target visible peers after leave, got %+v", diff.TargetVisiblePeers)
	}
	if len(diff.RemovedVisiblePeers) != 2 || diff.RemovedVisiblePeers[0].Name != "PeerAlpha" || diff.RemovedVisiblePeers[1].Name != "PeerZulu" {
		t.Fatalf("unexpected removed visible peers on leave: %+v", diff.RemovedVisiblePeers)
	}
	if len(diff.AddedVisiblePeers) != 0 {
		t.Fatalf("expected no added visible peers on leave, got %+v", diff.AddedVisiblePeers)
	}
}

func TestRelocateVisibilityDiffReportsCurrentTargetAddsAndRemovals(t *testing.T) {
	topology := NewBootstrapTopology(1)
	current := visibilityCharacter("Subject", 0x02040102, 1, 1500, 2600)
	target := current
	target.MapIndex = 42
	target.X = 1700
	target.Y = 2800
	oldPeer := visibilityCharacter("OldPeer", 0x02040101, 1, 1100, 2100)
	newPeer := visibilityCharacter("NewPeer", 0x02040103, 42, 1900, 3000)

	diff := RelocateVisibilityDiff(
		topology,
		current,
		[]loginticket.Character{current, oldPeer, newPeer},
		target,
		[]loginticket.Character{target, oldPeer, newPeer},
	)
	if len(diff.CurrentVisiblePeers) != 1 || diff.CurrentVisiblePeers[0].Name != "OldPeer" {
		t.Fatalf("unexpected current visible peers on relocate: %+v", diff.CurrentVisiblePeers)
	}
	if len(diff.TargetVisiblePeers) != 1 || diff.TargetVisiblePeers[0].Name != "NewPeer" {
		t.Fatalf("unexpected target visible peers on relocate: %+v", diff.TargetVisiblePeers)
	}
	if len(diff.RemovedVisiblePeers) != 1 || diff.RemovedVisiblePeers[0].Name != "OldPeer" {
		t.Fatalf("unexpected removed visible peers on relocate: %+v", diff.RemovedVisiblePeers)
	}
	if len(diff.AddedVisiblePeers) != 1 || diff.AddedVisiblePeers[0].Name != "NewPeer" {
		t.Fatalf("unexpected added visible peers on relocate: %+v", diff.AddedVisiblePeers)
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

type sameEmpireVisibilityPolicy struct{}

func (sameEmpireVisibilityPolicy) CanSee(_ BootstrapTopology, subject loginticket.Character, peer loginticket.Character) bool {
	return subject.Empire != 0 && subject.Empire == peer.Empire
}
