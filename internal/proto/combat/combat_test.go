package combat

import (
	"errors"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

func TestEncodeClientTargetRoundTrip(t *testing.T) {
	raw := EncodeClientTarget(ClientTargetPacket{TargetVID: 0x02040107})
	decoded, err := DecodeClientTarget(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode client target: %v", err)
	}
	if decoded.TargetVID != 0x02040107 {
		t.Fatalf("unexpected client target packet: %+v", decoded)
	}
}

func TestEncodeServerTargetRoundTrip(t *testing.T) {
	raw := EncodeServerTarget(ServerTargetPacket{TargetVID: 0x02040107, HPPercent: 100})
	decoded, err := DecodeServerTarget(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode server target: %v", err)
	}
	if decoded.TargetVID != 0x02040107 || decoded.HPPercent != 100 {
		t.Fatalf("unexpected server target packet: %+v", decoded)
	}
}

func TestDecodeClientTargetRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeClientTarget(frame.Frame{Header: HeaderClientTarget, Length: 7, Payload: []byte{0x01, 0x02, 0x03}})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeServerTargetRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeServerTarget(frame.Frame{Header: HeaderServerTarget, Length: 8, Payload: []byte{0x01, 0x02, 0x03, 0x04}})
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
