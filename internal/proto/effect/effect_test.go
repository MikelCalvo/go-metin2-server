package effect

import (
	"bytes"
	"errors"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

func TestEncodeDecodeSpecialEffectRoundTrip(t *testing.T) {
	packet := SpecialPacket{Type: SpecialEffectHPUpRed, VID: 0x01020304}
	raw := EncodeSpecial(packet)
	want := []byte{0x30, 0x0A, 0x09, 0x00, 0x01, 0x04, 0x03, 0x02, 0x01}
	if !bytes.Equal(raw, want) {
		t.Fatalf("unexpected special-effect frame:\n got: % x\nwant: % x", raw, want)
	}
	got, err := DecodeSpecial(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode special-effect frame: %v", err)
	}
	if got != packet {
		t.Fatalf("unexpected special-effect packet: got %+v want %+v", got, packet)
	}
}

func TestDecodeSpecialEffectRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeSpecial(frame.Frame{Header: HeaderSpecificEffect, Length: 9, Payload: make([]byte, specialPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeSpecialEffectRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeSpecial(frame.Frame{Header: HeaderSpecialEffect, Length: 8, Payload: make([]byte, specialPayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
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
