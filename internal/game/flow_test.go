package game

import (
	"errors"
	"testing"

	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

func TestHandleClientFrameAcceptsMoveInGameAndReturnsReplication(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseGame)
	flow := NewFlow(machine, Config{
		HandleMove: func(packet movep.MovePacket) Result {
			if packet.Func != 1 || packet.Rot != 12 || packet.X != 12345 || packet.Y != 23456 {
				t.Fatalf("unexpected move packet: %+v", packet)
			}
			return Result{Accepted: true, Replication: movep.MoveAckPacket{
				Func: 1, Arg: 0, Rot: 12, VID: 0x01020304, X: 12345, Y: 23456, Time: 0x11121314, Duration: 250,
			}}
		},
	})

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 12345, Y: 23456, Time: 0x01020304})))
	if err != nil {
		t.Fatalf("unexpected move error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing frame, got %d", len(out))
	}
	ack, err := movep.DecodeMoveAck(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode move ack: %v", err)
	}
	if ack.VID != 0x01020304 || ack.Duration != 250 || ack.X != 12345 || ack.Y != 23456 {
		t.Fatalf("unexpected move ack: %+v", ack)
	}
	if machine.Current() != session.PhaseGame {
		t.Fatalf("expected phase %q, got %q", session.PhaseGame, machine.Current())
	}
}

func TestHandleClientFrameRejectsUnexpectedPacketsInGame(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseGame)
	flow := NewFlow(machine, Config{})
	_, err := flow.HandleClientFrame(frame.Frame{Header: movep.HeaderMoveAck, Length: 23, Payload: make([]byte, 23)})
	if !errors.Is(err, ErrUnexpectedClientPacket) {
		t.Fatalf("expected ErrUnexpectedClientPacket, got %v", err)
	}
}

func TestHandleClientFrameRejectsCallsOutsideGame(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseLoading)
	flow := NewFlow(machine, Config{})
	_, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, X: 10, Y: 20})))
	if !errors.Is(err, ErrInvalidPhase) {
		t.Fatalf("expected ErrInvalidPhase, got %v", err)
	}
}

func decodeSingleFrame(t *testing.T, raw []byte) frame.Frame {
	t.Helper()
	decoder := frame.NewDecoder(4096)
	frames, err := decoder.Feed(raw)
	if err != nil {
		t.Fatalf("unexpected frame decode error: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}
	return frames[0]
}
