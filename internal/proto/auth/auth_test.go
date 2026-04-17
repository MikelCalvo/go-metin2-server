package auth

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
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

func TestEncodeLogin3BuildsAClientFrame(t *testing.T) {
	want := loadHexFixture(t, "login3-frame.hex")

	got, err := EncodeLogin3(Login3Packet{Login: "mkmk", Password: "hunter2"})
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected login3 frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeLogin3ReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeLogin3(decodeSingleFrame(t, loadHexFixture(t, "login3-frame.hex")))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}

	if packet.Login != "mkmk" {
		t.Fatalf("unexpected login: got %q want %q", packet.Login, "mkmk")
	}

	if packet.Password != "hunter2" {
		t.Fatalf("unexpected password: got %q want %q", packet.Password, "hunter2")
	}
}

func TestEncodeAuthSuccessBuildsAServerFrame(t *testing.T) {
	want := loadHexFixture(t, "auth-success-frame.hex")

	got := EncodeAuthSuccess(AuthSuccessPacket{LoginKey: 0x01020304, Result: 1})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected auth success frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeAuthSuccessReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeAuthSuccess(decodeSingleFrame(t, loadHexFixture(t, "auth-success-frame.hex")))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}

	if packet.LoginKey != 0x01020304 {
		t.Fatalf("unexpected login key: got %#08x want %#08x", packet.LoginKey, uint32(0x01020304))
	}

	if packet.Result != 1 {
		t.Fatalf("unexpected result: got %d want %d", packet.Result, 1)
	}
}

func TestDecodeLogin3RejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeLogin3(frame.Frame{Header: HeaderAuthSuccess, Length: 9, Payload: make([]byte, 5)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
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
