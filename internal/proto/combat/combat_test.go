package combat

import (
	"bytes"
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

func TestEncodeClientAttackUsesLegacyPayloadLayout(t *testing.T) {
	raw := EncodeClientAttack(ClientAttackPacket{
		AttackType:   0x03,
		TargetVID:    0x02040107,
		CRCProcPiece: 0x12,
		CRCFilePiece: 0x34,
	})
	expected := frame.Encode(HeaderClientAttack, []byte{0x03, 0x07, 0x01, 0x04, 0x02, 0x12, 0x34})
	if !bytes.Equal(raw, expected) {
		t.Fatalf("unexpected client attack encoding: got %x want %x", raw, expected)
	}

	decoded, err := DecodeClientAttack(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode client attack: %v", err)
	}
	if decoded.AttackType != 0x03 || decoded.TargetVID != 0x02040107 || decoded.CRCProcPiece != 0x12 || decoded.CRCFilePiece != 0x34 {
		t.Fatalf("unexpected client attack packet: %+v", decoded)
	}
}

func TestEncodeServerClearTargetUsesZeroTargetAndZeroHP(t *testing.T) {
	raw := EncodeServerClearTarget()
	expected := frame.Encode(HeaderServerTarget, []byte{0x00, 0x00, 0x00, 0x00, 0x00})
	if !bytes.Equal(raw, expected) {
		t.Fatalf("unexpected clear-target encoding: got %x want %x", raw, expected)
	}

	decoded, err := DecodeServerTarget(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode clear target: %v", err)
	}
	if decoded.TargetVID != 0 || decoded.HPPercent != 0 {
		t.Fatalf("unexpected clear-target packet: %+v", decoded)
	}
}

func TestEncodeServerDamageInfoUsesLegacyPayloadLayout(t *testing.T) {
	raw := EncodeServerDamageInfo(ServerDamageInfoPacket{VID: 0x02040107, Flag: 0x02, Damage: 1234})
	expected := frame.Encode(HeaderServerDamageInfo, []byte{0x07, 0x01, 0x04, 0x02, 0x02, 0xd2, 0x04, 0x00, 0x00})
	if !bytes.Equal(raw, expected) {
		t.Fatalf("unexpected server damage-info encoding: got %x want %x", raw, expected)
	}

	decoded, err := DecodeServerDamageInfo(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode server damage-info: %v", err)
	}
	if decoded.VID != 0x02040107 || decoded.Flag != 0x02 || decoded.Damage != 1234 {
		t.Fatalf("unexpected server damage-info packet: %+v", decoded)
	}
}

func TestEncodeServerDamageInfoPreservesSignedDamage(t *testing.T) {
	raw := EncodeServerDamageInfo(ServerDamageInfoPacket{VID: 0x02040107, Damage: -1})
	expected := frame.Encode(HeaderServerDamageInfo, []byte{0x07, 0x01, 0x04, 0x02, 0x00, 0xff, 0xff, 0xff, 0xff})
	if !bytes.Equal(raw, expected) {
		t.Fatalf("unexpected signed server damage-info encoding: got %x want %x", raw, expected)
	}

	decoded, err := DecodeServerDamageInfo(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode signed server damage-info: %v", err)
	}
	if decoded.Damage != -1 {
		t.Fatalf("expected signed damage to round-trip, got %+v", decoded)
	}
}

func TestDecodeClientTargetRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientTarget(frame.Frame{Header: HeaderServerTarget, Length: 8, Payload: []byte{0x07, 0x01, 0x04, 0x02, 0x64}})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientTargetRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeClientTarget(frame.Frame{Header: HeaderClientTarget, Length: 7, Payload: []byte{0x01, 0x02, 0x03}})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeServerTargetRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeServerTarget(frame.Frame{Header: HeaderClientTarget, Length: 7, Payload: []byte{0x07, 0x01, 0x04, 0x02}})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeServerTargetRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeServerTarget(frame.Frame{Header: HeaderServerTarget, Length: 8, Payload: []byte{0x01, 0x02, 0x03, 0x04}})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeClientAttackRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientAttack(frame.Frame{Header: HeaderClientTarget, Length: 7, Payload: []byte{0x07, 0x01, 0x04, 0x02}})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientAttackRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeClientAttack(frame.Frame{Header: HeaderClientAttack, Length: 10, Payload: []byte{0x01, 0x07, 0x01, 0x04, 0x02, 0x12}})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeServerDamageInfoRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeServerDamageInfo(frame.Frame{Header: HeaderServerTarget, Length: 13, Payload: []byte{0x07, 0x01, 0x04, 0x02, 0x02, 0xd2, 0x04, 0x00, 0x00}})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeServerDamageInfoRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeServerDamageInfo(frame.Frame{Header: HeaderServerDamageInfo, Length: 12, Payload: []byte{0x07, 0x01, 0x04, 0x02, 0x00, 0x01, 0x00, 0x00}})
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
