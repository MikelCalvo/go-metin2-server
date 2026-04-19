package login

import (
	"bytes"
	"errors"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

func TestHandleClientFrameAcceptsLogin2AndTransitionsToSelect(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseLogin)
	flow := NewFlow(machine, Config{
		Authenticate: func(packet loginproto.Login2Packet) Result {
			if packet.Login != "mkmk" {
				t.Fatalf("unexpected login value: %q", packet.Login)
			}
			if packet.LoginKey != 0x01020304 {
				t.Fatalf("unexpected login key: %#08x", packet.LoginKey)
			}

			return Result{
				Accepted:      true,
				Empire:        2,
				LoginSuccess4: sampleLoginSuccessPacket(),
			}
		},
	})

	incoming, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, incoming))
	if err != nil {
		t.Fatalf("unexpected login handling error: %v", err)
	}

	if len(out) != 3 {
		t.Fatalf("expected 3 outgoing frames, got %d", len(out))
	}

	wantEmpire := loginproto.EncodeEmpire(loginproto.EmpirePacket{Empire: 2})
	wantPhase, err := control.EncodePhase(session.PhaseSelect)
	if err != nil {
		t.Fatalf("unexpected phase encode error: %v", err)
	}
	wantSuccess, err := loginproto.EncodeLoginSuccess4(sampleLoginSuccessPacket())
	if err != nil {
		t.Fatalf("unexpected login success encode error: %v", err)
	}

	want := [][]byte{wantSuccess, wantEmpire, wantPhase}
	for i := range want {
		if !bytes.Equal(out[i], want[i]) {
			t.Fatalf("unexpected outgoing frame %d: got %x want %x", i, out[i], want[i])
		}
	}

	if machine.Current() != session.PhaseSelect {
		t.Fatalf("expected phase %q, got %q", session.PhaseSelect, machine.Current())
	}
}

func TestHandleClientFrameReturnsLoginFailureAndStaysInLoginWhenAuthenticationFails(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseLogin)
	flow := NewFlow(machine, Config{
		Authenticate: func(loginproto.Login2Packet) Result {
			return Result{Accepted: false, FailureStatus: "NOID"}
		},
	})

	incoming, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "ghost", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, incoming))
	if err != nil {
		t.Fatalf("unexpected login handling error: %v", err)
	}

	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing frame, got %d", len(out))
	}

	wantFailure, err := loginproto.EncodeLoginFailure(loginproto.LoginFailurePacket{Status: "NOID"})
	if err != nil {
		t.Fatalf("unexpected login failure encode error: %v", err)
	}

	if !bytes.Equal(out[0], wantFailure) {
		t.Fatalf("unexpected login failure bytes: got %x want %x", out[0], wantFailure)
	}

	if machine.Current() != session.PhaseLogin {
		t.Fatalf("expected phase %q, got %q", session.PhaseLogin, machine.Current())
	}
}

func TestHandleClientFrameKeepsTheSessionInLoginWhenSuccessEncodingFails(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseLogin)
	flow := NewFlow(machine, Config{
		Authenticate: func(loginproto.Login2Packet) Result {
			result := Result{Accepted: true, Empire: 2, LoginSuccess4: sampleLoginSuccessPacket()}
			result.LoginSuccess4.GuildNames[0] = "this-name-is-too-long"
			return result
		},
	})

	incoming, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}

	_, err = flow.HandleClientFrame(decodeSingleFrame(t, incoming))
	if !errors.Is(err, loginproto.ErrStringTooLong) {
		t.Fatalf("expected ErrStringTooLong, got %v", err)
	}

	if machine.Current() != session.PhaseLogin {
		t.Fatalf("expected phase %q after failed success encoding, got %q", session.PhaseLogin, machine.Current())
	}
}

func TestHandleClientFrameRejectsUnexpectedPacketsInLogin(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseLogin)
	flow := NewFlow(machine, Config{
		Authenticate: func(loginproto.Login2Packet) Result {
			return Result{Accepted: true, Empire: 2, LoginSuccess4: sampleLoginSuccessPacket()}
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

func TestHandleClientFrameRejectsPacketsOutsideLoginPhase(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseHandshake)
	flow := NewFlow(machine, Config{
		Authenticate: func(loginproto.Login2Packet) Result {
			return Result{Accepted: true, Empire: 2, LoginSuccess4: sampleLoginSuccessPacket()}
		},
	})

	incoming, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}

	_, err = flow.HandleClientFrame(decodeSingleFrame(t, incoming))
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

func sampleLoginSuccessPacket() loginproto.LoginSuccess4Packet {
	packet := loginproto.LoginSuccess4Packet{
		GuildIDs: [loginproto.PlayerCount]uint32{10, 20, 0, 0},
		GuildNames: [loginproto.PlayerCount]string{
			"Alpha",
			"Beta",
			"",
			"",
		},
		Handle:    0x11223344,
		RandomKey: 0x55667788,
	}

	packet.Players[0] = loginproto.SimplePlayer{
		ID:          1,
		Name:        "Chris",
		Job:         2,
		Level:       30,
		PlayMinutes: 1234,
		ST:          3,
		HT:          4,
		DX:          5,
		IQ:          6,
		MainPart:    100,
		ChangeName:  0,
		HairPart:    200,
		Dummy:       [4]byte{9, 8, 7, 6},
		X:           1000,
		Y:           2000,
		Addr:        0x0100007f,
		Port:        13000,
		SkillGroup:  1,
	}

	packet.Players[1] = loginproto.SimplePlayer{
		ID:          2,
		Name:        "Mkmk",
		Job:         1,
		Level:       15,
		PlayMinutes: 4321,
		ST:          6,
		HT:          5,
		DX:          4,
		IQ:          3,
		MainPart:    101,
		ChangeName:  1,
		HairPart:    201,
		Dummy:       [4]byte{1, 2, 3, 4},
		X:           3000,
		Y:           4000,
		Addr:        0x0200007f,
		Port:        13001,
		SkillGroup:  2,
	}

	return packet
}
