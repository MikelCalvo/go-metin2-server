package handshake

import (
	"bytes"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

func TestHandshakeFlowCompletesOverTCP(t *testing.T) {
	server := startHandshakeTestServer(t, testConfig())
	client := newHandshakeTestClient(t, server.address())

	challenge := client.readFrame(t)
	wantChallenge := control.EncodeKeyChallenge(testConfig().KeyChallenge)
	if !bytes.Equal(challenge.Raw, wantChallenge) {
		t.Fatalf("unexpected key challenge bytes: got %x want %x", challenge.Raw, wantChallenge)
	}

	client.writeFrame(t, control.EncodePong())
	client.expectNoFrameWithin(t, 50*time.Millisecond)

	client.writeFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	}))

	complete := client.readFrame(t)
	wantComplete := control.EncodeKeyComplete(testConfig().KeyComplete)
	if !bytes.Equal(complete.Raw, wantComplete) {
		t.Fatalf("unexpected key complete bytes: got %x want %x", complete.Raw, wantComplete)
	}

	phase := client.readFrame(t)
	wantPhase, err := control.EncodePhase(session.PhaseLogin)
	if err != nil {
		t.Fatalf("unexpected phase encode error: %v", err)
	}

	if !bytes.Equal(phase.Raw, wantPhase) {
		t.Fatalf("unexpected phase bytes: got %x want %x", phase.Raw, wantPhase)
	}

	if got := server.currentPhase(); got != session.PhaseLogin {
		t.Fatalf("expected server phase %q, got %q", session.PhaseLogin, got)
	}
}

func TestHandshakeFlowClosesTheConnectionWhenKeyResponseVerificationFails(t *testing.T) {
	cfg := testConfig()
	cfg.VerifyKeyResponse = func(control.KeyResponsePacket) bool {
		return false
	}

	server := startHandshakeTestServer(t, cfg)
	client := newHandshakeTestClient(t, server.address())

	_ = client.readFrame(t)
	client.writeFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	}))

	client.expectConnectionClose(t)

	if got := server.currentPhase(); got != session.PhaseClose {
		t.Fatalf("expected server phase %q, got %q", session.PhaseClose, got)
	}
}
