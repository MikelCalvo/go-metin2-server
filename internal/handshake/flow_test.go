package handshake

import (
	"bytes"
	"errors"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

func TestStartEmitsKeyChallengeAndKeepsTheSessionInHandshake(t *testing.T) {
	machine := session.NewStateMachine()
	flow := NewFlow(machine, testConfig())

	out, err := flow.Start()
	if err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing frame, got %d", len(out))
	}

	want := control.EncodeKeyChallenge(testConfig().KeyChallenge)
	if !bytes.Equal(out[0], want) {
		t.Fatalf("unexpected key challenge bytes: got %x want %x", out[0], want)
	}

	if machine.Current() != session.PhaseHandshake {
		t.Fatalf("expected phase %q, got %q", session.PhaseHandshake, machine.Current())
	}
}

func TestHandleClientFrameAcceptsPongWithoutAdvancingTheSession(t *testing.T) {
	machine := session.NewStateMachine()
	flow := NewFlow(machine, testConfig())

	if _, err := flow.Start(); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, control.EncodePong()))
	if err != nil {
		t.Fatalf("unexpected pong handling error: %v", err)
	}

	if len(out) != 0 {
		t.Fatalf("expected no outgoing frames, got %d", len(out))
	}

	if machine.Current() != session.PhaseHandshake {
		t.Fatalf("expected phase %q, got %q", session.PhaseHandshake, machine.Current())
	}
}

func TestHandleClientFrameCompletesTheHandshakeAndTransitionsToLogin(t *testing.T) {
	cfg := testConfig()
	machine := session.NewStateMachine()
	flow := NewFlow(machine, cfg)

	if _, err := flow.Start(); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	incoming := decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	}))

	out, err := flow.HandleClientFrame(incoming)
	if err != nil {
		t.Fatalf("unexpected key response handling error: %v", err)
	}

	if len(out) != 2 {
		t.Fatalf("expected 2 outgoing frames, got %d", len(out))
	}

	wantPhase, err := control.EncodePhase(session.PhaseLogin)
	if err != nil {
		t.Fatalf("unexpected phase encode error: %v", err)
	}

	want := [][]byte{
		control.EncodeKeyComplete(cfg.KeyComplete),
		wantPhase,
	}

	for i := range want {
		if !bytes.Equal(out[i], want[i]) {
			t.Fatalf("unexpected outgoing frame %d: got %x want %x", i, out[i], want[i])
		}
	}

	if machine.Current() != session.PhaseLogin {
		t.Fatalf("expected phase %q, got %q", session.PhaseLogin, machine.Current())
	}
}

func TestHandleClientFrameRejectsKeyResponseBeforeTheChallengeIsStarted(t *testing.T) {
	machine := session.NewStateMachine()
	flow := NewFlow(machine, testConfig())

	incoming := decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	}))

	_, err := flow.HandleClientFrame(incoming)
	if !errors.Is(err, ErrHandshakeNotStarted) {
		t.Fatalf("expected ErrHandshakeNotStarted, got %v", err)
	}

	if machine.Current() != session.PhaseHandshake {
		t.Fatalf("expected phase %q, got %q", session.PhaseHandshake, machine.Current())
	}
}

func TestHandleClientFrameRejectsUnexpectedPacketsDuringHandshake(t *testing.T) {
	machine := session.NewStateMachine()
	flow := NewFlow(machine, testConfig())

	if _, err := flow.Start(); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	phaseLogin, err := control.EncodePhase(session.PhaseLogin)
	if err != nil {
		t.Fatalf("unexpected phase encode error: %v", err)
	}

	_, err = flow.HandleClientFrame(decodeSingleFrame(t, phaseLogin))
	if !errors.Is(err, ErrUnexpectedClientPacket) {
		t.Fatalf("expected ErrUnexpectedClientPacket, got %v", err)
	}
}

func TestHandleClientFrameClosesTheSessionWhenKeyResponseVerificationFails(t *testing.T) {
	cfg := testConfig()
	cfg.VerifyKeyResponse = func(control.KeyResponsePacket) bool {
		return false
	}

	machine := session.NewStateMachine()
	flow := NewFlow(machine, cfg)

	if _, err := flow.Start(); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	incoming := decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	}))

	_, err := flow.HandleClientFrame(incoming)
	if !errors.Is(err, ErrKeyResponseRejected) {
		t.Fatalf("expected ErrKeyResponseRejected, got %v", err)
	}

	if machine.Current() != session.PhaseClose {
		t.Fatalf("expected phase %q, got %q", session.PhaseClose, machine.Current())
	}
}

func testConfig() Config {
	return Config{
		KeyChallenge: control.KeyChallengePacket{
			ServerPublicKey: sequentialBytes32(0x00),
			Challenge:       sequentialBytes32(0x20),
			ServerTime:      0x01020304,
		},
		KeyComplete: control.KeyCompletePacket{
			EncryptedToken: sequentialBytes48(0x80),
			Nonce:          sequentialBytes24(0xb0),
		},
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
