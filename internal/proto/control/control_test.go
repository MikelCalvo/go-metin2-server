package control

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

func loadHexFixture(t *testing.T, name string) []byte {
	t.Helper()

	path := filepath.Join("testdata", name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}

	decoded, err := hex.DecodeString(strings.TrimSpace(string(content)))
	if err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}

	return decoded
}

func TestEncodePhaseBuildsAControlFrame(t *testing.T) {
	want := loadHexFixture(t, "phase-login-frame.hex")

	got, err := EncodePhase(session.PhaseLogin)
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected phase frame bytes: got %x want %x", got, want)
	}
}

func TestDecodePhaseReturnsTheExpectedSessionPhase(t *testing.T) {
	decoder := frame.NewDecoder(1024)
	frames, err := decoder.Feed(loadHexFixture(t, "phase-login-frame.hex"))
	if err != nil {
		t.Fatalf("unexpected frame decode error: %v", err)
	}

	packet, err := DecodePhase(frames[0])
	if err != nil {
		t.Fatalf("unexpected phase decode error: %v", err)
	}

	if packet.Phase != session.PhaseLogin {
		t.Fatalf("unexpected phase: got %q want %q", packet.Phase, session.PhaseLogin)
	}
}

func TestDecodePhaseRejectsUnknownPhaseValues(t *testing.T) {
	badFrame := frame.Frame{Header: HeaderPhase, Length: 5, Payload: []byte{0xff}}

	_, err := DecodePhase(badFrame)
	if !errors.Is(err, ErrUnknownPhaseValue) {
		t.Fatalf("expected ErrUnknownPhaseValue, got %v", err)
	}
}

func TestDecodePingReturnsServerTime(t *testing.T) {
	decoder := frame.NewDecoder(1024)
	frames, err := decoder.Feed(loadHexFixture(t, "ping-frame.hex"))
	if err != nil {
		t.Fatalf("unexpected frame decode error: %v", err)
	}

	packet, err := DecodePing(frames[0])
	if err != nil {
		t.Fatalf("unexpected ping decode error: %v", err)
	}

	if packet.ServerTime != 0x01020304 {
		t.Fatalf("unexpected server time: got %#08x want %#08x", packet.ServerTime, uint32(0x01020304))
	}
}

func TestEncodePhaseSupportsDeadPhaseValue(t *testing.T) {
	want := loadHexFixture(t, "phase-dead-frame.hex")

	got, err := EncodePhase(session.PhaseDead)
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected dead phase frame bytes: got %x want %x", got, want)
	}
}

func TestEncodePongBuildsAHeaderOnlyControlFrame(t *testing.T) {
	want := loadHexFixture(t, "pong-frame.hex")
	got := EncodePong()

	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected pong frame bytes: got %x want %x", got, want)
	}
}
