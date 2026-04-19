package securecipher

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

func TestComputeChallengeResponseMatchesLibsodiumCryptoAuth(t *testing.T) {
	challenge := make([]byte, 32)
	for i := range challenge {
		challenge[i] = byte(i)
	}
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 32)
	}
	wantHex := "9297f97e0fdeab723fecc446393b98cbd7ab57c83381d36630ca620cecbd46ed"
	want, err := hex.DecodeString(wantHex)
	if err != nil {
		t.Fatalf("decode expected mac: %v", err)
	}

	got := computeChallengeResponse(challenge, key)
	if !bytes.Equal(got[:], want) {
		t.Fatalf("unexpected challenge response: got %x want %x", got, want)
	}
}

func TestServerAndClientSessionsCompleteSecureHandshakeAndExchangeEncryptedTraffic(t *testing.T) {
	server := NewServerSession(ServerConfig{
		Random:     rand.Reader,
		ServerTime: func() uint32 { return 0x01020304 },
	})
	client := NewClientSession(ClientConfig{Random: rand.Reader})

	challenge, err := server.Start()
	if err != nil {
		t.Fatalf("start server session: %v", err)
	}
	if challenge.ServerTime != 0x01020304 {
		t.Fatalf("unexpected server time: got %#08x", challenge.ServerTime)
	}

	response, err := client.HandleKeyChallenge(challenge)
	if err != nil {
		t.Fatalf("handle key challenge: %v", err)
	}

	complete, err := server.HandleKeyResponse(response)
	if err != nil {
		t.Fatalf("handle key response: %v", err)
	}

	keyCompleteRaw := control.EncodeKeyComplete(complete)
	firstOutgoing, err := server.EncryptOutgoing(keyCompleteRaw)
	if err != nil {
		t.Fatalf("encrypt key complete boundary: %v", err)
	}
	if !bytes.Equal(firstOutgoing, keyCompleteRaw) {
		t.Fatalf("expected key complete to remain plaintext at activation boundary")
	}

	if err := client.HandleKeyComplete(complete); err != nil {
		t.Fatalf("handle key complete: %v", err)
	}

	phaseLoginRaw, err := control.EncodePhase(session.PhaseLogin)
	if err != nil {
		t.Fatalf("encode phase login: %v", err)
	}
	encryptedPhaseLogin, err := server.EncryptOutgoing(phaseLoginRaw)
	if err != nil {
		t.Fatalf("encrypt phase login: %v", err)
	}
	if bytes.Equal(encryptedPhaseLogin, phaseLoginRaw) {
		t.Fatalf("expected phase login to be encrypted after key complete boundary")
	}
	decryptedPhaseLogin, err := client.DecryptIncoming(encryptedPhaseLogin)
	if err != nil {
		t.Fatalf("decrypt phase login: %v", err)
	}
	if !bytes.Equal(decryptedPhaseLogin, phaseLoginRaw) {
		t.Fatalf("unexpected decrypted phase login bytes")
	}

	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("encode login2: %v", err)
	}
	encryptedLogin2, err := client.EncryptOutgoing(login2Raw)
	if err != nil {
		t.Fatalf("encrypt login2: %v", err)
	}
	if bytes.Equal(encryptedLogin2, login2Raw) {
		t.Fatalf("expected login2 to be encrypted after client activation")
	}
	decryptedLogin2, err := server.DecryptIncoming(encryptedLogin2)
	if err != nil {
		t.Fatalf("decrypt login2: %v", err)
	}
	if !bytes.Equal(decryptedLogin2, login2Raw) {
		t.Fatalf("unexpected decrypted login2 bytes")
	}
}

func TestServerSessionRejectsTamperedChallengeResponse(t *testing.T) {
	server := NewServerSession(ServerConfig{Random: rand.Reader})
	client := NewClientSession(ClientConfig{Random: rand.Reader})

	challenge, err := server.Start()
	if err != nil {
		t.Fatalf("start server session: %v", err)
	}
	response, err := client.HandleKeyChallenge(challenge)
	if err != nil {
		t.Fatalf("handle key challenge: %v", err)
	}
	response.ChallengeResponse[0] ^= 0xff

	if _, err := server.HandleKeyResponse(response); err == nil {
		t.Fatalf("expected tampered challenge response to be rejected")
	}
}
