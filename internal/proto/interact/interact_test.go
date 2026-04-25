package interact

import (
	"errors"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

func TestEncodeDecodeRequest(t *testing.T) {
	raw := EncodeRequest(RequestPacket{TargetVID: 0x02040107})
	decoded, err := DecodeRequest(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode interaction request: %v", err)
	}
	if decoded.TargetVID != 0x02040107 {
		t.Fatalf("unexpected decoded interaction request: %+v", decoded)
	}
}

func TestDecodeRequestRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeRequest(frame.Frame{Header: HeaderRequest + 1, Length: 8, Payload: make([]byte, requestPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeRequestRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeRequest(frame.Frame{Header: HeaderRequest, Length: 7, Payload: make([]byte, requestPayloadSize-1)})
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
