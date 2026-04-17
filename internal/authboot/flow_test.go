package authboot

import (
	"bytes"
	"errors"
	"testing"

	authflow "github.com/MikelCalvo/go-metin2-server/internal/auth"
	"github.com/MikelCalvo/go-metin2-server/internal/handshake"
	authproto "github.com/MikelCalvo/go-metin2-server/internal/proto/auth"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

func TestAuthBootFlowCompletesHandshakeAndAuthenticatesLogin3(t *testing.T) {
	flow := NewFlow(testConfig())

	startOut, err := flow.Start()
	if err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected 1 outgoing frame, got %d", len(startOut))
	}
	wantChallenge := control.EncodeKeyChallenge(testConfig().Handshake.KeyChallenge)
	if !bytes.Equal(startOut[0], wantChallenge) {
		t.Fatalf("unexpected key challenge bytes: got %x want %x", startOut[0], wantChallenge)
	}

	handshakeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	})))
	if err != nil {
		t.Fatalf("unexpected handshake error: %v", err)
	}
	if len(handshakeOut) != 2 {
		t.Fatalf("expected 2 handshake frames, got %d", len(handshakeOut))
	}

	wantPhaseAuth, err := control.EncodePhase(session.PhaseAuth)
	if err != nil {
		t.Fatalf("unexpected phase encode error: %v", err)
	}
	wantHandshake := [][]byte{control.EncodeKeyComplete(testConfig().Handshake.KeyComplete), wantPhaseAuth}
	for i := range wantHandshake {
		if !bytes.Equal(handshakeOut[i], wantHandshake[i]) {
			t.Fatalf("unexpected handshake frame %d: got %x want %x", i, handshakeOut[i], wantHandshake[i])
		}
	}

	login3Raw, err := authproto.EncodeLogin3(authproto.Login3Packet{Login: "mkmk", Password: "hunter2"})
	if err != nil {
		t.Fatalf("unexpected login3 encode error: %v", err)
	}

	authOut, err := flow.HandleClientFrame(decodeSingleFrame(t, login3Raw))
	if err != nil {
		t.Fatalf("unexpected auth error: %v", err)
	}

	wantAuth := authproto.EncodeAuthSuccess(authproto.AuthSuccessPacket{LoginKey: 0x01020304, Result: 1})
	if len(authOut) != 1 || !bytes.Equal(authOut[0], wantAuth) {
		t.Fatalf("unexpected auth output: got %x want %x", authOut, wantAuth)
	}

	if flow.CurrentPhase() != session.PhaseAuth {
		t.Fatalf("expected phase %q, got %q", session.PhaseAuth, flow.CurrentPhase())
	}
}

func TestAuthBootFlowReturnsLoginFailureOnBadCredentials(t *testing.T) {
	cfg := testConfig()
	cfg.Auth.Authenticate = func(authproto.Login3Packet) authflow.Result {
		return authflow.Result{Accepted: false, FailureStatus: "WRONGPWD"}
	}
	flow := NewFlow(cfg)

	if _, err := flow.Start(); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	}))); err != nil {
		t.Fatalf("unexpected handshake error: %v", err)
	}

	login3Raw, err := authproto.EncodeLogin3(authproto.Login3Packet{Login: "mkmk", Password: "bad"})
	if err != nil {
		t.Fatalf("unexpected login3 encode error: %v", err)
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, login3Raw))
	if err != nil {
		t.Fatalf("unexpected auth error: %v", err)
	}

	wantFailure, err := loginproto.EncodeLoginFailure(loginproto.LoginFailurePacket{Status: "WRONGPWD"})
	if err != nil {
		t.Fatalf("unexpected login failure encode error: %v", err)
	}
	if len(out) != 1 || !bytes.Equal(out[0], wantFailure) {
		t.Fatalf("unexpected auth failure output: got %x want %x", out, wantFailure)
	}

	if flow.CurrentPhase() != session.PhaseAuth {
		t.Fatalf("expected phase %q, got %q", session.PhaseAuth, flow.CurrentPhase())
	}
}

func TestStartRejectsAConflictingHandshakeNextPhase(t *testing.T) {
	cfg := testConfig()
	cfg.Handshake.NextPhase = session.PhaseLogin
	flow := NewFlow(cfg)

	_, err := flow.Start()
	if !errors.Is(err, ErrConflictingHandshakeNextPhase) {
		t.Fatalf("expected ErrConflictingHandshakeNextPhase, got %v", err)
	}

	if flow.CurrentPhase() != session.PhaseHandshake {
		t.Fatalf("expected phase %q, got %q", session.PhaseHandshake, flow.CurrentPhase())
	}
}

func TestHandleClientFrameReturnsAuthBootUnsupportedPhaseAfterClose(t *testing.T) {
	cfg := testConfig()
	cfg.Handshake.VerifyKeyResponse = func(control.KeyResponsePacket) bool {
		return false
	}
	flow := NewFlow(cfg)

	if _, err := flow.Start(); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	_, err := flow.HandleClientFrame(decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	})))
	if !errors.Is(err, handshake.ErrKeyResponseRejected) {
		t.Fatalf("expected ErrKeyResponseRejected, got %v", err)
	}

	if flow.CurrentPhase() != session.PhaseClose {
		t.Fatalf("expected phase %q, got %q", session.PhaseClose, flow.CurrentPhase())
	}

	pong := decodeSingleFrame(t, control.EncodePong())
	_, err = flow.HandleClientFrame(pong)
	if !errors.Is(err, ErrUnsupportedPhase) {
		t.Fatalf("expected ErrUnsupportedPhase, got %v", err)
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

func testConfig() Config {
	return Config{
		Handshake: handshake.Config{
			KeyChallenge: control.KeyChallengePacket{
				ServerPublicKey: sequentialBytes32(0x00),
				Challenge:       sequentialBytes32(0x20),
				ServerTime:      0x01020304,
			},
			KeyComplete: control.KeyCompletePacket{
				EncryptedToken: sequentialBytes48(0x80),
				Nonce:          sequentialBytes24(0xb0),
			},
		},
		Auth: authflow.Config{
			Authenticate: func(packet authproto.Login3Packet) authflow.Result {
				if packet.Login == "mkmk" && packet.Password == "hunter2" {
					return authflow.Result{Accepted: true, LoginKey: 0x01020304}
				}
				return authflow.Result{Accepted: false, FailureStatus: "WRONGPWD"}
			},
		},
	}
}

func sequentialBytes32(start byte) [32]byte {
	var out [32]byte
	for i := range out {
		out[i] = start + byte(i)
	}
	return out
}

func sequentialBytes48(start byte) [48]byte {
	var out [48]byte
	for i := range out {
		out[i] = start + byte(i)
	}
	return out
}

func sequentialBytes24(start byte) [24]byte {
	var out [24]byte
	for i := range out {
		out[i] = start + byte(i)
	}
	return out
}
