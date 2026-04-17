package frame

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestEncodeBuildsALittleEndianLengthPrefixedFrame(t *testing.T) {
	payload := []byte{0x01}
	want := loadHexFixture(t, "phase-frame.hex")

	got := Encode(0x0008, payload)

	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected frame bytes: got %x want %x", got, want)
	}
}

func TestDecoderEmitsASingleFrameFromOneChunk(t *testing.T) {
	decoder := NewDecoder(1024)

	frames, err := decoder.Feed(loadHexFixture(t, "phase-frame.hex"))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}

	if len(frames) != 1 {
		t.Fatalf("expected one frame, got %d", len(frames))
	}

	if frames[0].Header != 0x0008 {
		t.Fatalf("unexpected header: got %#04x", frames[0].Header)
	}

	if frames[0].Length != 5 {
		t.Fatalf("unexpected length: got %d want %d", frames[0].Length, 5)
	}

	if !bytes.Equal(frames[0].Payload, []byte{0x01}) {
		t.Fatalf("unexpected payload: got %x want %x", frames[0].Payload, []byte{0x01})
	}
}

func TestDecoderBuffersFragmentedFramesUntilComplete(t *testing.T) {
	raw := loadHexFixture(t, "ping-frame.hex")
	decoder := NewDecoder(1024)

	frames, err := decoder.Feed(raw[:3])
	if err != nil {
		t.Fatalf("unexpected decode error on partial frame: %v", err)
	}

	if len(frames) != 0 {
		t.Fatalf("expected zero frames from partial input, got %d", len(frames))
	}

	frames, err = decoder.Feed(raw[3:])
	if err != nil {
		t.Fatalf("unexpected decode error on completed frame: %v", err)
	}

	if len(frames) != 1 {
		t.Fatalf("expected one frame after completing input, got %d", len(frames))
	}

	if frames[0].Header != 0x0007 {
		t.Fatalf("unexpected header: got %#04x", frames[0].Header)
	}
}

func TestDecoderEmitsMultipleFramesFromOneChunk(t *testing.T) {
	decoder := NewDecoder(1024)

	frames, err := decoder.Feed(loadHexFixture(t, "phase-and-ping.hex"))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}

	if len(frames) != 2 {
		t.Fatalf("expected two frames, got %d", len(frames))
	}

	if frames[0].Header != 0x0008 || frames[1].Header != 0x0007 {
		t.Fatalf("unexpected headers: got %#04x and %#04x", frames[0].Header, frames[1].Header)
	}
}

func TestDecoderRejectsLengthsShorterThanTheEnvelope(t *testing.T) {
	decoder := NewDecoder(1024)

	_, err := decoder.Feed([]byte{0x08, 0x00, 0x03, 0x00})
	if !errors.Is(err, ErrInvalidLength) {
		t.Fatalf("expected ErrInvalidLength, got %v", err)
	}
}

func TestDecoderRejectsFramesLargerThanTheConfiguredMaximum(t *testing.T) {
	decoder := NewDecoder(8)

	raw := []byte{0x08, 0x00, 0x10, 0x00, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	_, err := decoder.Feed(raw)
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("expected ErrFrameTooLarge, got %v", err)
	}
}
