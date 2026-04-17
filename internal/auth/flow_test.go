package auth

import (
	"bytes"
	"errors"
	"testing"

	authproto "github.com/MikelCalvo/go-metin2-server/internal/proto/auth"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

func TestHandleClientFrameAcceptsLogin3AndReturnsAuthSuccess(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseAuth)
	flow := NewFlow(machine, Config{
		Authenticate: func(packet authproto.Login3Packet) Result {
			if packet.Login != "mkmk" {
				t.Fatalf("unexpected login: %q", packet.Login)
			}
			if packet.Password != "hunter2" {
				t.Fatalf("unexpected password: %q", packet.Password)
			}

			return Result{Accepted: true, LoginKey: 0x01020304}
		},
	})

	incoming, err := authproto.EncodeLogin3(authproto.Login3Packet{Login: "mkmk", Password: "hunter2"})
	if err != nil {
		t.Fatalf("unexpected login3 encode error: %v", err)
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, incoming))
	if err != nil {
		t.Fatalf("unexpected auth handling error: %v", err)
	}

	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing frame, got %d", len(out))
	}

	want := authproto.EncodeAuthSuccess(authproto.AuthSuccessPacket{LoginKey: 0x01020304, Result: 1})
	if !bytes.Equal(out[0], want) {
		t.Fatalf("unexpected auth success bytes: got %x want %x", out[0], want)
	}

	if machine.Current() != session.PhaseAuth {
		t.Fatalf("expected phase %q, got %q", session.PhaseAuth, machine.Current())
	}
}

func TestHandleClientFrameReturnsLoginFailureAndStaysInAuthWhenAuthenticationFails(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseAuth)
	flow := NewFlow(machine, Config{
		Authenticate: func(authproto.Login3Packet) Result {
			return Result{Accepted: false, FailureStatus: "WRONGPWD"}
		},
	})

	incoming, err := authproto.EncodeLogin3(authproto.Login3Packet{Login: "mkmk", Password: "bad"})
	if err != nil {
		t.Fatalf("unexpected login3 encode error: %v", err)
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, incoming))
	if err != nil {
		t.Fatalf("unexpected auth handling error: %v", err)
	}

	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing frame, got %d", len(out))
	}

	want, err := loginproto.EncodeLoginFailure(loginproto.LoginFailurePacket{Status: "WRONGPWD"})
	if err != nil {
		t.Fatalf("unexpected login failure encode error: %v", err)
	}

	if !bytes.Equal(out[0], want) {
		t.Fatalf("unexpected login failure bytes: got %x want %x", out[0], want)
	}

	if machine.Current() != session.PhaseAuth {
		t.Fatalf("expected phase %q, got %q", session.PhaseAuth, machine.Current())
	}
}

func TestHandleClientFrameAcceptsPongWithoutStateChange(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseAuth)
	flow := NewFlow(machine, Config{
		Authenticate: func(authproto.Login3Packet) Result {
			return Result{Accepted: true, LoginKey: 0x01020304}
		},
	})

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, control.EncodePong()))
	if err != nil {
		t.Fatalf("unexpected pong handling error: %v", err)
	}

	if len(out) != 0 {
		t.Fatalf("expected no outgoing frames, got %d", len(out))
	}

	if machine.Current() != session.PhaseAuth {
		t.Fatalf("expected phase %q, got %q", session.PhaseAuth, machine.Current())
	}
}

func TestHandleClientFrameRejectsUnexpectedPacketsInAuth(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseAuth)
	flow := NewFlow(machine, Config{
		Authenticate: func(authproto.Login3Packet) Result {
			return Result{Accepted: true, LoginKey: 0x01020304}
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

func TestHandleClientFrameRejectsPacketsOutsideAuthPhase(t *testing.T) {
	machine := session.NewStateMachineAt(session.PhaseHandshake)
	flow := NewFlow(machine, Config{
		Authenticate: func(authproto.Login3Packet) Result {
			return Result{Accepted: true, LoginKey: 0x01020304}
		},
	})

	incoming, err := authproto.EncodeLogin3(authproto.Login3Packet{Login: "mkmk", Password: "hunter2"})
	if err != nil {
		t.Fatalf("unexpected login3 encode error: %v", err)
	}

	_, err = flow.HandleClientFrame(decodeSingleFrame(t, incoming))
	if !errors.Is(err, ErrInvalidPhase) {
		t.Fatalf("expected ErrInvalidPhase, got %v", err)
	}
}

func decodeSingleFrame(t *testing.T, raw []byte) frame.Frame {
	t.Helper()

	decoder := frame.NewDecoder(1024)
	frames, err := decoder.Feed(raw)
	if err != nil {
		t.Fatalf("unexpected frame decode error: %v", err)
	}

	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}

	return frames[0]
}
