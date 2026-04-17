package boot

import (
	"bytes"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

func TestBootFlowCompletesHandshakeAndLoginOverTCP(t *testing.T) {
	server := startBootTestServer(t, testConfig())
	client := newBootTestClient(t, server.address())

	challenge := client.readFrame(t)
	wantChallenge := control.EncodeKeyChallenge(testConfig().Handshake.KeyChallenge)
	if !bytes.Equal(challenge.Raw, wantChallenge) {
		t.Fatalf("unexpected key challenge bytes: got %x want %x", challenge.Raw, wantChallenge)
	}

	client.writeFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	}))

	complete := client.readFrame(t)
	wantComplete := control.EncodeKeyComplete(testConfig().Handshake.KeyComplete)
	if !bytes.Equal(complete.Raw, wantComplete) {
		t.Fatalf("unexpected key complete bytes: got %x want %x", complete.Raw, wantComplete)
	}

	phaseLogin := client.readFrame(t)
	wantPhaseLogin, err := control.EncodePhase(session.PhaseLogin)
	if err != nil {
		t.Fatalf("unexpected phase encode error: %v", err)
	}
	if !bytes.Equal(phaseLogin.Raw, wantPhaseLogin) {
		t.Fatalf("unexpected login phase bytes: got %x want %x", phaseLogin.Raw, wantPhaseLogin)
	}

	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}
	client.writeFrame(t, login2Raw)

	empire := client.readFrame(t)
	wantEmpire := loginproto.EncodeEmpire(loginproto.EmpirePacket{Empire: 2})
	if !bytes.Equal(empire.Raw, wantEmpire) {
		t.Fatalf("unexpected empire bytes: got %x want %x", empire.Raw, wantEmpire)
	}

	phaseSelect := client.readFrame(t)
	wantPhaseSelect, err := control.EncodePhase(session.PhaseSelect)
	if err != nil {
		t.Fatalf("unexpected phase encode error: %v", err)
	}
	if !bytes.Equal(phaseSelect.Raw, wantPhaseSelect) {
		t.Fatalf("unexpected select phase bytes: got %x want %x", phaseSelect.Raw, wantPhaseSelect)
	}

	loginSuccess := client.readFrame(t)
	wantSuccess, err := loginproto.EncodeLoginSuccess4(sampleLoginSuccessPacket())
	if err != nil {
		t.Fatalf("unexpected login success encode error: %v", err)
	}
	if !bytes.Equal(loginSuccess.Raw, wantSuccess) {
		t.Fatalf("unexpected login success bytes: got %x want %x", loginSuccess.Raw, wantSuccess)
	}

	if got := server.currentPhase(); got != session.PhaseSelect {
		t.Fatalf("expected server phase %q, got %q", session.PhaseSelect, got)
	}
}
