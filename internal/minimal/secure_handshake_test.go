package minimal

import (
	"crypto/rand"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	"github.com/MikelCalvo/go-metin2-server/internal/securecipher"
)

type secureHandshakeTestFlow interface {
	Start() ([][]byte, error)
	HandleClientFrame(frame.Frame) ([][]byte, error)
}

func secureHandshakeResponseFromStartFrames(t *testing.T, startOut [][]byte) []byte {
	t.Helper()

	for _, raw := range startOut {
		frame := decodeSingleFrame(t, raw)
		if frame.Header != control.HeaderKeyChallenge {
			continue
		}
		challenge, err := control.DecodeKeyChallenge(frame)
		if err != nil {
			t.Fatalf("decode key challenge: %v", err)
		}
		client := securecipher.NewClientSession(securecipher.ClientConfig{Random: rand.Reader})
		response, err := client.HandleKeyChallenge(challenge)
		if err != nil {
			t.Fatalf("handle key challenge: %v", err)
		}
		return control.EncodeKeyResponse(response)
	}

	t.Fatalf("expected a key challenge start frame, got %d start frames", len(startOut))
	return nil
}

func mustCompleteSecureHandshake(t *testing.T, flow secureHandshakeTestFlow) [][]byte {
	t.Helper()
	startOut, err := flow.Start()
	if err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}
	handshakeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, secureHandshakeResponseFromStartFrames(t, startOut)))
	if err != nil {
		t.Fatalf("unexpected handshake error: %v", err)
	}
	return handshakeOut
}
