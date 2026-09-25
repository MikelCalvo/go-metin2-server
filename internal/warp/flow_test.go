package warp

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
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

func TestFlowKeepsCommitSelfFramesAfterSuccessfulApply(t *testing.T) {
	selected := loginticket.Character{ID: 0x01030102, VID: 0x02040102, Name: "PeerTwo", MapIndex: 1, X: 1500, Y: 2600}
	self := []byte{0x06, 0x03, 0x12, 0x00}
	flow := NewFlow(Config{
		Commit: func(updated loginticket.Character) (Result, bool) {
			return Result{Applied: true, Updated: updated, SelfFrames: [][]byte{self}}, true
		},
	})

	result, ok := flow.Apply(selected, Target{MapIndex: 42, X: 1700, Y: 2800})
	if !ok {
		t.Fatal("expected flow to report successful commit")
	}
	if len(result.SelfFrames) != 1 || !bytes.Equal(result.SelfFrames[0], self) {
		t.Fatalf("expected commit self frames to survive apply, got %+v", result.SelfFrames)
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

func TestFlowPersistsUpdatedSnapshotBeforeCommit(t *testing.T) {
	selected := loginticket.Character{ID: 0x01030102, VID: 0x02040102, Name: "PeerTwo", MapIndex: 1, X: 1500, Y: 2600}
	steps := make([]string, 0, 2)
	flow := NewFlow(Config{
		Persist: func(updated loginticket.Character) bool {
			steps = append(steps, "persist")
			if updated.MapIndex != 42 || updated.X != 1700 || updated.Y != 2800 {
				t.Fatalf("expected persist to receive relocated snapshot, got %+v", updated)
			}
			return true
		},
		Commit: func(updated loginticket.Character) (Result, bool) {
			steps = append(steps, "commit")
			return Result{Applied: true, Updated: updated}, true
		},
	})

	result, ok := flow.Apply(selected, Target{MapIndex: 42, X: 1700, Y: 2800})
	if !ok {
		t.Fatalf("expected flow to succeed after persist-before-commit, got %+v", result)
	}
	if len(steps) != 2 || steps[0] != "persist" || steps[1] != "commit" {
		t.Fatalf("expected persist-before-commit ordering, got %+v", steps)
	}
}

func TestFlowRejectsWhenPersistFailsBeforeCommit(t *testing.T) {
	selected := loginticket.Character{ID: 0x01030102, VID: 0x02040102, Name: "PeerTwo", MapIndex: 1, X: 1500, Y: 2600}
	commitCalled := false
	rollbackCalled := false
	flow := NewFlow(Config{
		Persist: func(updated loginticket.Character) bool {
			return false
		},
		Rollback: func(previous loginticket.Character) bool {
			rollbackCalled = true
			return true
		},
		Commit: func(updated loginticket.Character) (Result, bool) {
			commitCalled = true
			return Result{Applied: true, Updated: updated}, true
		},
	})

	result, ok := flow.Apply(selected, Target{MapIndex: 42, X: 1700, Y: 2800})
	if ok {
		t.Fatalf("expected persist failure to reject transfer, got %+v", result)
	}
	if commitCalled {
		t.Fatal("expected persist failure to stop before commit")
	}
	if rollbackCalled {
		t.Fatal("expected persist failure not to call rollback")
	}
}

func TestFlowRollsBackPersistedSnapshotWhenCommitFails(t *testing.T) {
	selected := loginticket.Character{ID: 0x01030102, VID: 0x02040102, Name: "PeerTwo", MapIndex: 1, X: 1500, Y: 2600}
	rollbackCalled := false
	var restored loginticket.Character
	flow := NewFlow(Config{
		Persist: func(updated loginticket.Character) bool {
			return true
		},
		Rollback: func(previous loginticket.Character) bool {
			rollbackCalled = true
			restored = previous
			return true
		},
		Commit: func(updated loginticket.Character) (Result, bool) {
			return Result{}, false
		},
	})

	result, ok := flow.Apply(selected, Target{MapIndex: 42, X: 1700, Y: 2800})
	if ok {
		t.Fatalf("expected commit failure to reject transfer, got %+v", result)
	}
	if !rollbackCalled {
		t.Fatal("expected commit failure after persist to trigger rollback")
	}
	if !reflect.DeepEqual(restored, selected) {
		t.Fatalf("expected rollback to receive original snapshot, got %+v want %+v", restored, selected)
	}
	if len(result.SelfFrames) != 0 {
		t.Fatalf("expected rejected transfer to carry no self warp, got %+v", result.SelfFrames)
	}
}

func TestFlowAppendsSelfWarpAfterCommitFrames(t *testing.T) {
	selected := loginticket.Character{ID: 0x01030102, VID: 0x02040102, Name: "PeerTwo", MapIndex: 1, X: 1500, Y: 2600}
	burst := []byte{0x0D, 0x00, 0x04, 0x00}
	flow := NewFlow(Config{
		Endpoint: &worldproto.WarpPacket{Addr: 0x0100007F, Port: 13000},
		Commit: func(updated loginticket.Character) (Result, bool) {
			return Result{Applied: true, Updated: updated, SelfFrames: [][]byte{burst}}, true
		},
	})

	result, ok := flow.Apply(selected, Target{MapIndex: 42, X: 1700, Y: 2800})
	if !ok {
		t.Fatal("expected flow to report successful commit")
	}
	want := worldproto.EncodeWarp(worldproto.WarpPacket{X: 1700, Y: 2800, Addr: 0x0100007F, Port: 13000})
	if len(result.SelfFrames) != 2 || !bytes.Equal(result.SelfFrames[0], burst) || !bytes.Equal(result.SelfFrames[1], want) {
		t.Fatalf("expected self warp after commit frames, got %x", result.SelfFrames)
	}
	decoded, err := worldproto.DecodeWarp(decodeWarpFrame(t, result.SelfFrames[1]))
	if err != nil {
		t.Fatalf("decode self warp: %v", err)
	}
	if decoded.X != 1700 || decoded.Y != 2800 || decoded.Addr != 0x0100007F || decoded.Port != 13000 {
		t.Fatalf("unexpected self warp: %+v", decoded)
	}
}

func TestFlowOmitsSelfWarpWhenEndpointIsUnset(t *testing.T) {
	selected := loginticket.Character{ID: 0x01030102, VID: 0x02040102, Name: "PeerTwo", MapIndex: 1, X: 1500, Y: 2600}
	flow := NewFlow(Config{
		Commit: func(updated loginticket.Character) (Result, bool) {
			return Result{Applied: true, Updated: updated}, true
		},
	})
	result, ok := flow.Apply(selected, Target{MapIndex: 42, X: 1700, Y: 2800})
	if !ok {
		t.Fatal("expected flow to report successful commit")
	}
	if len(result.SelfFrames) != 0 {
		t.Fatalf("expected no self warp without an advertised endpoint, got %+v", result.SelfFrames)
	}
}

func decodeWarpFrame(t *testing.T, raw []byte) frame.Frame {
	t.Helper()
	frames, err := frame.NewDecoder(4096).Feed(raw)
	if err != nil {
		t.Fatalf("decode warp frame: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("expected 1 warp frame, got %d", len(frames))
	}
	return frames[0]
}
