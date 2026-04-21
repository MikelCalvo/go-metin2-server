package warp

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestFlowAppliesDestinationSnapshotBeforeCommit(t *testing.T) {
	selected := loginticket.Character{
		ID:       0x01030102,
		VID:      0x02040102,
		Name:     "PeerTwo",
		MapIndex: 1,
		X:        1500,
		Y:        2600,
	}

	var committed loginticket.Character
	flow := NewFlow(Config{
		Commit: func(updated loginticket.Character) (Result, bool) {
			committed = updated
			return Result{Applied: true, Updated: updated}, true
		},
	})

	result, ok := flow.Apply(selected, Target{MapIndex: 42, X: 1700, Y: 2800})
	if !ok {
		t.Fatal("expected flow to report successful commit")
	}
	if committed.MapIndex != 42 || committed.X != 1700 || committed.Y != 2800 {
		t.Fatalf("expected commit to receive relocated snapshot, got %+v", committed)
	}
	if !result.Applied || result.Updated.MapIndex != 42 || result.Updated.X != 1700 || result.Updated.Y != 2800 {
		t.Fatalf("expected successful relocated result, got %+v", result)
	}
}

func TestFlowRejectsZeroTargetMapWithoutCallingCommit(t *testing.T) {
	selected := loginticket.Character{ID: 0x01030102, VID: 0x02040102, Name: "PeerTwo", MapIndex: 1, X: 1500, Y: 2600}
	called := false
	flow := NewFlow(Config{
		Commit: func(updated loginticket.Character) (Result, bool) {
			called = true
			return Result{Applied: true, Updated: updated}, true
		},
	})

	result, ok := flow.Apply(selected, Target{MapIndex: 0, X: 1700, Y: 2800})
	if ok {
		t.Fatalf("expected zero-map transfer target to be rejected, got %+v", result)
	}
	if called {
		t.Fatal("expected zero-map transfer target to be rejected before commit callback")
	}
}
