package worldentry

import (
	"bytes"
	"errors"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

func TestHandleClientFrameAcceptsCharacterSelectAndTransitionsToLoading(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseSelect)
	flow := NewFlow(machine, Config{
		SelectCharacter: func(index uint8) Result {
			if index != 1 {
				t.Fatalf("unexpected character index: %d", index)
			}
			return Result{Accepted: true, MainCharacter: sampleMainCharacter(), PlayerPoints: samplePlayerPoints()}
		},
	})

	incoming := decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))
	out, err := flow.HandleClientFrame(incoming)
	if err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}

	if len(out) != 3 {
		t.Fatalf("expected 3 outgoing frames, got %d", len(out))
	}

	wantPhase, err := control.EncodePhase(session.PhaseLoading)
	if err != nil {
		t.Fatalf("unexpected phase encode error: %v", err)
	}
	wantMain, err := worldproto.EncodeMainCharacter(sampleMainCharacter())
	if err != nil {
		t.Fatalf("unexpected main character encode error: %v", err)
	}
	wantPoints := worldproto.EncodePlayerPoints(samplePlayerPoints())

	want := [][]byte{wantPhase, wantMain, wantPoints}
	for i := range want {
		if !bytes.Equal(out[i], want[i]) {
			t.Fatalf("unexpected outgoing frame %d: got %x want %x", i, out[i], want[i])
		}
	}

	if machine.Current() != session.PhaseLoading {
		t.Fatalf("expected phase %q, got %q", session.PhaseLoading, machine.Current())
	}
}

func TestHandleClientFrameAcceptsCharacterCreateAndStaysInSelect(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseSelect)
	flow := NewFlow(machine, Config{
		CreateCharacter: func(packet worldproto.CharacterCreatePacket) CreateResult {
			if packet.Index != 2 || packet.Name != "FreshSura" || packet.RaceNum != 2 || packet.Shape != 1 {
				t.Fatalf("unexpected create packet: %+v", packet)
			}
			return CreateResult{Accepted: true, Player: worldproto.PlayerCreateSuccessPacket{Index: 2, Player: sampleCreatedPlayer()}}
		},
	})

	raw, err := worldproto.EncodeCharacterCreate(worldproto.CharacterCreatePacket{Index: 2, Name: "FreshSura", RaceNum: 2, Shape: 1})
	if err != nil {
		t.Fatalf("unexpected create encode error: %v", err)
	}
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing frame, got %d", len(out))
	}
	want, err := worldproto.EncodePlayerCreateSuccess(worldproto.PlayerCreateSuccessPacket{Index: 2, Player: sampleCreatedPlayer()})
	if err != nil {
		t.Fatalf("unexpected create success encode error: %v", err)
	}
	if !bytes.Equal(out[0], want) {
		t.Fatalf("unexpected create success frame: got %x want %x", out[0], want)
	}
	if machine.Current() != session.PhaseSelect {
		t.Fatalf("expected phase %q, got %q", session.PhaseSelect, machine.Current())
	}
}

func TestHandleClientFrameAcceptsEmpireSelectAndStaysInSelect(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseSelect)
	flow := NewFlow(machine, Config{
		SelectEmpire: func(empire uint8) EmpireResult {
			if empire != 3 {
				t.Fatalf("unexpected empire selection: %d", empire)
			}
			return EmpireResult{Accepted: true, Empire: 3}
		},
	})

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, loginproto.EncodeEmpireSelect(loginproto.EmpireSelectPacket{Empire: 3})))
	if err != nil {
		t.Fatalf("unexpected empire select error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing frame, got %d", len(out))
	}
	want := loginproto.EncodeEmpire(loginproto.EmpirePacket{Empire: 3})
	if !bytes.Equal(out[0], want) {
		t.Fatalf("unexpected empire select response: got %x want %x", out[0], want)
	}
	if machine.Current() != session.PhaseSelect {
		t.Fatalf("expected phase %q, got %q", session.PhaseSelect, machine.Current())
	}
}

func TestHandleClientFrameReturnsCreateFailurePacketAndKeepsSelectPhase(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseSelect)
	flow := NewFlow(machine, Config{
		CreateCharacter: func(worldproto.CharacterCreatePacket) CreateResult {
			return CreateResult{Accepted: false, FailureType: 1}
		},
	})

	raw, err := worldproto.EncodeCharacterCreate(worldproto.CharacterCreatePacket{Index: 2, Name: "FreshSura", RaceNum: 2, Shape: 1})
	if err != nil {
		t.Fatalf("unexpected create encode error: %v", err)
	}
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("unexpected create failure error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing frame, got %d", len(out))
	}
	want := worldproto.EncodePlayerCreateFailure(worldproto.PlayerCreateFailurePacket{Type: 1})
	if !bytes.Equal(out[0], want) {
		t.Fatalf("unexpected create failure frame: got %x want %x", out[0], want)
	}
	if machine.Current() != session.PhaseSelect {
		t.Fatalf("expected phase %q, got %q", session.PhaseSelect, machine.Current())
	}
}

func TestHandleClientFrameReturnsToGameWhenEnterGameArrivesInLoading(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseLoading)
	flow := NewFlow(machine, Config{
		SelectCharacter: func(uint8) Result {
			return Result{Accepted: true, MainCharacter: sampleMainCharacter(), PlayerPoints: samplePlayerPoints()}
		},
	})

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame()))
	if err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing frame, got %d", len(out))
	}

	wantPhase, err := control.EncodePhase(session.PhaseGame)
	if err != nil {
		t.Fatalf("unexpected phase encode error: %v", err)
	}
	if !bytes.Equal(out[0], wantPhase) {
		t.Fatalf("unexpected phase bytes: got %x want %x", out[0], wantPhase)
	}

	if machine.Current() != session.PhaseGame {
		t.Fatalf("expected phase %q, got %q", session.PhaseGame, machine.Current())
	}
}

func TestHandleClientFrameKeepsTheSessionInSelectWhenCharacterSelectionFails(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseSelect)
	flow := NewFlow(machine, Config{
		SelectCharacter: func(uint8) Result {
			return Result{Accepted: false}
		},
	})

	_, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 2})))
	if !errors.Is(err, ErrCharacterSelectRejected) {
		t.Fatalf("expected ErrCharacterSelectRejected, got %v", err)
	}

	if machine.Current() != session.PhaseSelect {
		t.Fatalf("expected phase %q, got %q", session.PhaseSelect, machine.Current())
	}
}

func TestHandleClientFrameRejectsUnexpectedPacketsForTheCurrentPhase(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseSelect)
	flow := NewFlow(machine, Config{
		SelectCharacter: func(uint8) Result {
			return Result{Accepted: true, MainCharacter: sampleMainCharacter(), PlayerPoints: samplePlayerPoints()}
		},
	})

	phaseLogin, err := control.EncodePhase(session.PhaseLogin)
	if err != nil {
		t.Fatalf("unexpected phase encode error: %v", err)
	}

	_, err = flow.HandleClientFrame(decodeSingleFrame(t, phaseLogin))
	if !errors.Is(err, ErrUnexpectedClientPacket) {
		t.Fatalf("expected ErrUnexpectedClientPacket, got %v", err)
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

func sampleMainCharacter() worldproto.MainCharacterPacket {
	return worldproto.MainCharacterPacket{
		VID:        0x01020304,
		RaceNum:    2,
		Name:       "Mkmk",
		BGMName:    "",
		BGMVolume:  0,
		X:          1000,
		Y:          2000,
		Z:          0,
		Empire:     2,
		SkillGroup: 1,
	}
}

func samplePlayerPoints() worldproto.PlayerPointsPacket {
	var points worldproto.PlayerPointsPacket
	points.Points[0] = 15
	points.Points[1] = 1234
	points.Points[2] = 5678
	points.Points[3] = 900
	points.Points[4] = 1000
	points.Points[5] = 200
	points.Points[6] = 300
	points.Points[7] = 999999
	points.Points[8] = 50
	return points
}

func sampleCreatedPlayer() loginproto.SimplePlayer {
	return loginproto.SimplePlayer{
		ID:          3,
		Name:        "FreshSura",
		Job:         2,
		Level:       1,
		PlayMinutes: 0,
		ST:          5,
		HT:          3,
		DX:          3,
		IQ:          5,
		MainPart:    1,
		ChangeName:  0,
		HairPart:    0,
		Dummy:       [4]byte{},
		X:           1500,
		Y:           2500,
		Addr:        0x0100007f,
		Port:        13000,
		SkillGroup:  0,
	}
}
