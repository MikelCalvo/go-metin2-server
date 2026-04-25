package game

import (
	"errors"
	"testing"

	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
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

func TestHandleClientFrameAcceptsMoveInGameWithExplicitEmptyFrames(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseGame)
	flow := NewFlow(machine, Config{
		HandleMove: func(packet movep.MovePacket) Result {
			if packet.X != 12345 || packet.Y != 23456 {
				t.Fatalf("unexpected move packet: %+v", packet)
			}
			return Result{Accepted: true, Frames: [][]byte{}}
		},
	})

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 12345, Y: 23456, Time: 0x01020304})))
	if err != nil {
		t.Fatalf("unexpected move error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected no outgoing frames, got %d", len(out))
	}
	if machine.Current() != session.PhaseGame {
		t.Fatalf("expected phase %q, got %q", session.PhaseGame, machine.Current())
	}
}

func TestHandleClientFrameAcceptsSyncPositionInGameAndReturnsSynchronization(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseGame)
	flow := NewFlow(machine, Config{
		HandleSyncPosition: func(packet movep.SyncPositionPacket) SyncPositionResult {
			if len(packet.Elements) != 2 {
				t.Fatalf("expected 2 sync elements, got %d", len(packet.Elements))
			}
			if packet.Elements[0].VID != 0x01020304 || packet.Elements[0].X != 12345 || packet.Elements[0].Y != 23456 {
				t.Fatalf("unexpected first sync element: %+v", packet.Elements[0])
			}
			return SyncPositionResult{Accepted: true, Synchronization: movep.SyncPositionAckPacket{Elements: []movep.SyncPositionElement{{VID: 0x01020304, X: 12345, Y: 23456}}}}
		},
	})

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeSyncPosition(movep.SyncPositionPacket{Elements: []movep.SyncPositionElement{{VID: 0x01020304, X: 12345, Y: 23456}, {VID: 0x01020305, X: 34567, Y: 45678}}})))
	if err != nil {
		t.Fatalf("unexpected sync position error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing frame, got %d", len(out))
	}
	ack, err := movep.DecodeSyncPositionAck(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode sync position ack: %v", err)
	}
	if len(ack.Elements) != 1 {
		t.Fatalf("expected 1 sync ack element, got %d", len(ack.Elements))
	}
	if ack.Elements[0].VID != 0x01020304 || ack.Elements[0].X != 12345 || ack.Elements[0].Y != 23456 {
		t.Fatalf("unexpected sync ack element: %+v", ack.Elements[0])
	}
	if machine.Current() != session.PhaseGame {
		t.Fatalf("expected phase %q, got %q", session.PhaseGame, machine.Current())
	}
}

func TestHandleClientFrameAcceptsSyncPositionInGameWithExplicitEmptyFrames(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseGame)
	flow := NewFlow(machine, Config{
		HandleSyncPosition: func(packet movep.SyncPositionPacket) SyncPositionResult {
			if len(packet.Elements) != 1 || packet.Elements[0].VID != 0x01020304 || packet.Elements[0].X != 12345 || packet.Elements[0].Y != 23456 {
				t.Fatalf("unexpected sync position packet: %+v", packet)
			}
			return SyncPositionResult{Accepted: true, Frames: [][]byte{}}
		},
	})

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeSyncPosition(movep.SyncPositionPacket{Elements: []movep.SyncPositionElement{{VID: 0x01020304, X: 12345, Y: 23456}}})))
	if err != nil {
		t.Fatalf("unexpected sync position error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected no outgoing frames, got %d", len(out))
	}
	if machine.Current() != session.PhaseGame {
		t.Fatalf("expected phase %q, got %q", session.PhaseGame, machine.Current())
	}
}

func TestHandleClientFrameAcceptsChatInGameAndReturnsDelivery(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseGame)
	flow := NewFlow(machine, Config{
		HandleChat: func(packet chatproto.ClientChatPacket) ChatResult {
			if packet.Type != chatproto.ChatTypeTalking || packet.Message != "hola" {
				t.Fatalf("unexpected chat packet: %+v", packet)
			}
			delivery := chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeTalking, VID: 0x02040102, Empire: 0, Message: "PeerTwo : hola"}
			return ChatResult{Accepted: true, Delivery: &delivery}
		},
	})

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "hola"})))
	if err != nil {
		t.Fatalf("unexpected chat error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing frame, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode chat delivery: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeTalking || delivery.VID != 0x02040102 || delivery.Message != "PeerTwo : hola" {
		t.Fatalf("unexpected chat delivery: %+v", delivery)
	}
	if machine.Current() != session.PhaseGame {
		t.Fatalf("expected phase %q, got %q", session.PhaseGame, machine.Current())
	}
}

func TestHandleClientFrameChatCanTransitionBackToSelect(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseGame)
	flow := NewFlow(machine, Config{
		HandleChat: func(packet chatproto.ClientChatPacket) ChatResult {
			if packet.Type != chatproto.ChatTypeTalking || packet.Message != "/phase_select" {
				t.Fatalf("unexpected chat packet: %+v", packet)
			}
			return ChatResult{Accepted: true, NextPhase: session.PhaseSelect}
		},
	})

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/phase_select"})))
	if err != nil {
		t.Fatalf("unexpected chat command error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing phase-select frame, got %d", len(out))
	}
	phase, err := control.DecodePhase(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode phase-select frame: %v", err)
	}
	if phase.Phase != session.PhaseSelect {
		t.Fatalf("expected phase %q, got %q", session.PhaseSelect, phase.Phase)
	}
	if machine.Current() != session.PhaseSelect {
		t.Fatalf("expected machine phase %q after chat command, got %q", session.PhaseSelect, machine.Current())
	}
}

func TestHandleClientFrameChatCanTransitionToClose(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseGame)
	flow := NewFlow(machine, Config{
		HandleChat: func(packet chatproto.ClientChatPacket) ChatResult {
			if packet.Type != chatproto.ChatTypeTalking || packet.Message != "/logout" {
				t.Fatalf("unexpected chat packet: %+v", packet)
			}
			return ChatResult{Accepted: true, NextPhase: session.PhaseClose}
		},
	})

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/logout"})))
	if err != nil {
		t.Fatalf("unexpected chat logout error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing phase-close frame, got %d", len(out))
	}
	phase, err := control.DecodePhase(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode phase-close frame: %v", err)
	}
	if phase.Phase != session.PhaseClose {
		t.Fatalf("expected phase %q, got %q", session.PhaseClose, phase.Phase)
	}
	if machine.Current() != session.PhaseClose {
		t.Fatalf("expected machine phase %q after logout command, got %q", session.PhaseClose, machine.Current())
	}
}

func TestHandleClientFrameAcceptsWhisperInGameAndReturnsDelivery(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseGame)
	flow := NewFlow(machine, Config{
		HandleWhisper: func(packet chatproto.ClientWhisperPacket) WhisperResult {
			if packet.Target != "PeerOne" || packet.Message != "hola privado" {
				t.Fatalf("unexpected whisper packet: %+v", packet)
			}
			return WhisperResult{Accepted: true, Delivery: &chatproto.ServerWhisperPacket{Type: chatproto.WhisperTypeChat, FromName: "PeerTwo", Message: "hola privado"}}
		},
	})

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientWhisper(chatproto.ClientWhisperPacket{Target: "PeerOne", Message: "hola privado"})))
	if err != nil {
		t.Fatalf("unexpected whisper error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing frame, got %d", len(out))
	}
	delivery, err := chatproto.DecodeServerWhisper(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode whisper delivery: %v", err)
	}
	if delivery.Type != chatproto.WhisperTypeChat || delivery.FromName != "PeerTwo" || delivery.Message != "hola privado" {
		t.Fatalf("unexpected whisper delivery: %+v", delivery)
	}
	if machine.Current() != session.PhaseGame {
		t.Fatalf("expected phase %q, got %q", session.PhaseGame, machine.Current())
	}
}

func TestHandleClientFrameAcceptsPartyChatInGameAndReturnsDelivery(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseGame)
	flow := NewFlow(machine, Config{
		HandleChat: func(packet chatproto.ClientChatPacket) ChatResult {
			if packet.Type != chatproto.ChatTypeParty || packet.Message != "hola party" {
				t.Fatalf("unexpected party chat packet: %+v", packet)
			}
			delivery := chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeParty, VID: 0x02040102, Empire: 0, Message: "PeerTwo : hola party"}
			return ChatResult{Accepted: true, Delivery: &delivery}
		},
	})

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeParty, Message: "hola party"})))
	if err != nil {
		t.Fatalf("unexpected party chat error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing frame, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode party chat delivery: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeParty || delivery.VID != 0x02040102 || delivery.Message != "PeerTwo : hola party" {
		t.Fatalf("unexpected party chat delivery: %+v", delivery)
	}
	if machine.Current() != session.PhaseGame {
		t.Fatalf("expected phase %q, got %q", session.PhaseGame, machine.Current())
	}
}

func TestHandleClientFrameAcceptsGuildChatInGameAndReturnsDelivery(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseGame)
	flow := NewFlow(machine, Config{
		HandleChat: func(packet chatproto.ClientChatPacket) ChatResult {
			if packet.Type != chatproto.ChatTypeGuild || packet.Message != "hola guild" {
				t.Fatalf("unexpected guild chat packet: %+v", packet)
			}
			delivery := chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeGuild, VID: 0x02040102, Empire: 0, Message: "PeerTwo : hola guild"}
			return ChatResult{Accepted: true, Delivery: &delivery}
		},
	})

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeGuild, Message: "hola guild"})))
	if err != nil {
		t.Fatalf("unexpected guild chat error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing frame, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode guild chat delivery: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeGuild || delivery.VID != 0x02040102 || delivery.Message != "PeerTwo : hola guild" {
		t.Fatalf("unexpected guild chat delivery: %+v", delivery)
	}
	if machine.Current() != session.PhaseGame {
		t.Fatalf("expected phase %q, got %q", session.PhaseGame, machine.Current())
	}
}

func TestHandleClientFrameAcceptsShoutChatInGameAndReturnsDelivery(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseGame)
	flow := NewFlow(machine, Config{
		HandleChat: func(packet chatproto.ClientChatPacket) ChatResult {
			if packet.Type != chatproto.ChatTypeShout || packet.Message != "hola shout" {
				t.Fatalf("unexpected shout chat packet: %+v", packet)
			}
			delivery := chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeShout, VID: 0x02040102, Empire: 0, Message: "PeerTwo : hola shout"}
			return ChatResult{Accepted: true, Delivery: &delivery}
		},
	})

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeShout, Message: "hola shout"})))
	if err != nil {
		t.Fatalf("unexpected shout chat error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing frame, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode shout chat delivery: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeShout || delivery.VID != 0x02040102 || delivery.Message != "PeerTwo : hola shout" {
		t.Fatalf("unexpected shout chat delivery: %+v", delivery)
	}
	if machine.Current() != session.PhaseGame {
		t.Fatalf("expected phase %q, got %q", session.PhaseGame, machine.Current())
	}
}

func TestHandleClientFrameAcceptsInfoChatInGameAndReturnsDelivery(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseGame)
	flow := NewFlow(machine, Config{
		HandleChat: func(packet chatproto.ClientChatPacket) ChatResult {
			if packet.Type != chatproto.ChatTypeInfo || packet.Message != "mensaje info" {
				t.Fatalf("unexpected info chat packet: %+v", packet)
			}
			delivery := chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: "mensaje info"}
			return ChatResult{Accepted: true, Delivery: &delivery}
		},
	})

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeInfo, Message: "mensaje info"})))
	if err != nil {
		t.Fatalf("unexpected info chat error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing frame, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode info chat delivery: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != "mensaje info" {
		t.Fatalf("unexpected info chat delivery: %+v", delivery)
	}
	if machine.Current() != session.PhaseGame {
		t.Fatalf("expected phase %q, got %q", session.PhaseGame, machine.Current())
	}
}

func TestHandleClientFrameAcceptsNoticeChatInGameAndReturnsDelivery(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseGame)
	flow := NewFlow(machine, Config{
		HandleChat: func(packet chatproto.ClientChatPacket) ChatResult {
			if packet.Type != chatproto.ChatTypeNotice || packet.Message != "mensaje notice" {
				t.Fatalf("unexpected notice chat packet: %+v", packet)
			}
			delivery := chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeNotice, VID: 0, Empire: 0, Message: "mensaje notice"}
			return ChatResult{Accepted: true, Delivery: &delivery}
		},
	})

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeNotice, Message: "mensaje notice"})))
	if err != nil {
		t.Fatalf("unexpected notice chat error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing frame, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode notice chat delivery: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeNotice || delivery.VID != 0 || delivery.Message != "mensaje notice" {
		t.Fatalf("unexpected notice chat delivery: %+v", delivery)
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

func TestHandleClientFrameAcceptsPongInGameAsNoOp(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseGame)
	flow := NewFlow(machine, Config{})
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, control.EncodePong()))
	if err != nil {
		t.Fatalf("unexpected pong error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected no outgoing frames for pong, got %d", len(out))
	}
	if machine.Current() != session.PhaseGame {
		t.Fatalf("expected phase %q, got %q", session.PhaseGame, machine.Current())
	}
}

func TestHandleClientFrameRejectsMalformedPongInGame(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseGame)
	flow := NewFlow(machine, Config{})
	_, err := flow.HandleClientFrame(frame.Frame{Header: control.HeaderPong, Length: 5, Payload: []byte{0x01}})
	if !errors.Is(err, control.ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
	if machine.Current() != session.PhaseGame {
		t.Fatalf("expected phase %q, got %q", session.PhaseGame, machine.Current())
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
