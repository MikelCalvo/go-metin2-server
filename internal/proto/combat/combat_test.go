package combat

import (
	"bytes"
	"encoding/binary"
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

func TestEncodeClientUseSkillUsesLegacyPayloadLayout(t *testing.T) {
	raw := EncodeClientUseSkill(ClientUseSkillPacket{SkillVnum: 0x00000023, TargetVID: 0x02040107})
	expected := frame.Encode(HeaderClientUseSkill, []byte{0x23, 0x00, 0x00, 0x00, 0x07, 0x01, 0x04, 0x02})
	if !bytes.Equal(raw, expected) {
		t.Fatalf("unexpected client use-skill encoding: got %x want %x", raw, expected)
	}

	decoded, err := DecodeClientUseSkill(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode client use-skill: %v", err)
	}
	if decoded.SkillVnum != 0x00000023 || decoded.TargetVID != 0x02040107 {
		t.Fatalf("unexpected client use-skill packet: %+v", decoded)
	}
}

func TestEncodeClientShootUsesLegacyPayloadLayout(t *testing.T) {
	raw := EncodeClientShoot(ClientShootPacket{ShootType: 0x83})
	expected := frame.Encode(HeaderClientShoot, []byte{0x83})
	if !bytes.Equal(raw, expected) {
		t.Fatalf("unexpected client shoot encoding: got %x want %x", raw, expected)
	}

	decoded, err := DecodeClientShoot(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode client shoot: %v", err)
	}
	if decoded.ShootType != 0x83 {
		t.Fatalf("unexpected client shoot packet: %+v", decoded)
	}
}

func TestEncodeClientFlyTargetingUsesLegacyPayloadLayout(t *testing.T) {
	raw := EncodeClientFlyTargeting(ClientFlyTargetingPacket{TargetVID: 0x02040107, X: 123456, Y: -234567})
	expected := frame.Encode(HeaderClientFlyTargeting, []byte{0x07, 0x01, 0x04, 0x02, 0x40, 0xe2, 0x01, 0x00, 0xb9, 0x6b, 0xfc, 0xff})
	if !bytes.Equal(raw, expected) {
		t.Fatalf("unexpected client fly-targeting encoding: got %x want %x", raw, expected)
	}

	decoded, err := DecodeClientFlyTargeting(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode client fly-targeting: %v", err)
	}
	if decoded.TargetVID != 0x02040107 || decoded.X != 123456 || decoded.Y != -234567 {
		t.Fatalf("unexpected client fly-targeting packet: %+v", decoded)
	}
}

func TestEncodeClientAddFlyTargetingUsesLegacyPayloadLayout(t *testing.T) {
	raw := EncodeClientAddFlyTargeting(ClientFlyTargetingPacket{TargetVID: 0, X: 1700, Y: -2800})
	expected := frame.Encode(HeaderClientAddFlyTargeting, []byte{0x00, 0x00, 0x00, 0x00, 0xa4, 0x06, 0x00, 0x00, 0x10, 0xf5, 0xff, 0xff})
	if !bytes.Equal(raw, expected) {
		t.Fatalf("unexpected client add-fly-targeting encoding: got %x want %x", raw, expected)
	}

	decoded, err := DecodeClientAddFlyTargeting(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode client add-fly-targeting: %v", err)
	}
	if decoded.TargetVID != 0 || decoded.X != 1700 || decoded.Y != -2800 {
		t.Fatalf("unexpected client add-fly-targeting packet: %+v", decoded)
	}
}

func TestEncodeClientCharacterPositionUsesLegacyPayloadLayout(t *testing.T) {
	raw := EncodeClientCharacterPosition(ClientCharacterPositionPacket{Position: 0x01})
	expected := frame.Encode(HeaderClientCharacterPosition, []byte{0x01})
	if !bytes.Equal(raw, expected) {
		t.Fatalf("unexpected client character-position encoding: got %x want %x", raw, expected)
	}

	decoded, err := DecodeClientCharacterPosition(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode client character-position: %v", err)
	}
	if decoded.Position != 0x01 {
		t.Fatalf("unexpected client character-position packet: %+v", decoded)
	}
}

func TestEncodeClientOnClickUsesLegacyPayloadLayout(t *testing.T) {
	raw := EncodeClientOnClick(ClientOnClickPacket{VID: 0x02040107})
	expected := frame.Encode(HeaderClientOnClick, []byte{0x07, 0x01, 0x04, 0x02})
	if !bytes.Equal(raw, expected) {
		t.Fatalf("unexpected client on-click encoding: got %x want %x", raw, expected)
	}

	decoded, err := DecodeClientOnClick(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode client on-click: %v", err)
	}
	if decoded.VID != 0x02040107 {
		t.Fatalf("unexpected client on-click packet: %+v", decoded)
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

func TestEncodeServerFlyTargetingUsesLegacyPayloadLayout(t *testing.T) {
	raw := EncodeServerFlyTargeting(ServerFlyTargetingPacket{ShooterVID: 0x01020304, TargetVID: 0x05060708, X: 123456, Y: -234567})
	expected := frame.Encode(HeaderServerFlyTargeting, []byte{0x04, 0x03, 0x02, 0x01, 0x08, 0x07, 0x06, 0x05, 0x40, 0xe2, 0x01, 0x00, 0xb9, 0x6b, 0xfc, 0xff})
	if !bytes.Equal(raw, expected) {
		t.Fatalf("unexpected server fly-targeting encoding: got %x want %x", raw, expected)
	}

	decoded, err := DecodeServerFlyTargeting(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode server fly-targeting: %v", err)
	}
	if decoded.ShooterVID != 0x01020304 || decoded.TargetVID != 0x05060708 || decoded.X != 123456 || decoded.Y != -234567 {
		t.Fatalf("unexpected server fly-targeting packet: %+v", decoded)
	}
}

func TestEncodeServerAddFlyTargetingUsesLegacyPayloadLayout(t *testing.T) {
	raw := EncodeServerAddFlyTargeting(ServerFlyTargetingPacket{ShooterVID: 0x01020304, TargetVID: 0, X: 1700, Y: -2800})
	expected := frame.Encode(HeaderServerAddFlyTargeting, []byte{0x04, 0x03, 0x02, 0x01, 0x00, 0x00, 0x00, 0x00, 0xa4, 0x06, 0x00, 0x00, 0x10, 0xf5, 0xff, 0xff})
	if !bytes.Equal(raw, expected) {
		t.Fatalf("unexpected server add-fly-targeting encoding: got %x want %x", raw, expected)
	}

	decoded, err := DecodeServerAddFlyTargeting(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode server add-fly-targeting: %v", err)
	}
	if decoded.ShooterVID != 0x01020304 || decoded.TargetVID != 0 || decoded.X != 1700 || decoded.Y != -2800 {
		t.Fatalf("unexpected server add-fly-targeting packet: %+v", decoded)
	}
}

func TestEncodeServerCreateFlyUsesLegacyPayloadLayout(t *testing.T) {
	raw := EncodeServerCreateFly(ServerCreateFlyPacket{Type: 0x09, StartVID: 0x01020304, EndVID: 0x05060708})
	expected := frame.Encode(HeaderServerCreateFly, []byte{0x09, 0x04, 0x03, 0x02, 0x01, 0x08, 0x07, 0x06, 0x05})
	if !bytes.Equal(raw, expected) {
		t.Fatalf("unexpected server create-fly encoding: got %x want %x", raw, expected)
	}

	decoded, err := DecodeServerCreateFly(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode server create-fly: %v", err)
	}
	if decoded.Type != 0x09 || decoded.StartVID != 0x01020304 || decoded.EndVID != 0x05060708 {
		t.Fatalf("unexpected server create-fly packet: %+v", decoded)
	}
}

func TestEncodeServerPVPUsesLegacyPayloadLayout(t *testing.T) {
	raw := EncodeServerPVP(ServerPVPPacket{SourceVID: 0x01020304, DestinationVID: 0x05060708, Mode: ServerPVPModeFight})
	expected := frame.Encode(HeaderServerPVP, []byte{0x04, 0x03, 0x02, 0x01, 0x08, 0x07, 0x06, 0x05, 0x02})
	if !bytes.Equal(raw, expected) {
		t.Fatalf("unexpected server pvp encoding: got %x want %x", raw, expected)
	}

	decoded, err := DecodeServerPVP(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode server pvp: %v", err)
	}
	if decoded.SourceVID != 0x01020304 || decoded.DestinationVID != 0x05060708 || decoded.Mode != ServerPVPModeFight {
		t.Fatalf("unexpected server pvp packet: %+v", decoded)
	}
}

func TestEncodeServerTargetCreateNewUsesLegacyPayloadLayout(t *testing.T) {
	raw, err := EncodeServerTargetCreateNew(ServerTargetCreateNewPacket{ID: -7, TargetName: "Practice Target", VID: 0x02040107, Type: ServerTargetMarkerTypeCharacter})
	if err != nil {
		t.Fatalf("encode server target create new: %v", err)
	}
	payload := make([]byte, serverTargetCreateNewPayloadSize)
	binary.LittleEndian.PutUint32(payload[0:4], uint32(0xfffffff9))
	copy(payload[4:37], "Practice Target")
	binary.LittleEndian.PutUint32(payload[37:41], 0x02040107)
	payload[41] = ServerTargetMarkerTypeCharacter
	expected := frame.Encode(HeaderServerTargetCreateNew, payload)
	if !bytes.Equal(raw, expected) {
		t.Fatalf("unexpected server target-create-new encoding: got %x want %x", raw, expected)
	}

	decoded, err := DecodeServerTargetCreateNew(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode server target-create-new: %v", err)
	}
	if decoded.ID != -7 || decoded.TargetName != "Practice Target" || decoded.VID != 0x02040107 || decoded.Type != ServerTargetMarkerTypeCharacter {
		t.Fatalf("unexpected server target-create-new packet: %+v", decoded)
	}
}

func TestEncodeServerTargetCreateNewAcceptsThirtyTwoByteName(t *testing.T) {
	name := "12345678901234567890123456789012"
	raw, err := EncodeServerTargetCreateNew(ServerTargetCreateNewPacket{ID: 3, TargetName: name, Type: ServerTargetMarkerTypeLocation})
	if err != nil {
		t.Fatalf("encode 32-byte server target-create-new name: %v", err)
	}
	decoded, err := DecodeServerTargetCreateNew(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode 32-byte server target-create-new name: %v", err)
	}
	if decoded.TargetName != name {
		t.Fatalf("unexpected 32-byte target name: got %q want %q", decoded.TargetName, name)
	}
}

func TestEncodeServerTargetUpdateUsesLegacyPayloadLayout(t *testing.T) {
	raw := EncodeServerTargetUpdate(ServerTargetUpdatePacket{ID: -7, X: 123456, Y: -234567})
	expected := frame.Encode(HeaderServerTargetUpdate, []byte{0xf9, 0xff, 0xff, 0xff, 0x40, 0xe2, 0x01, 0x00, 0xb9, 0x6b, 0xfc, 0xff})
	if !bytes.Equal(raw, expected) {
		t.Fatalf("unexpected server target-update encoding: got %x want %x", raw, expected)
	}

	decoded, err := DecodeServerTargetUpdate(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode server target-update: %v", err)
	}
	if decoded.ID != -7 || decoded.X != 123456 || decoded.Y != -234567 {
		t.Fatalf("unexpected server target-update packet: %+v", decoded)
	}
}

func TestEncodeServerTargetDeleteUsesLegacyPayloadLayout(t *testing.T) {
	raw := EncodeServerTargetDelete(ServerTargetDeletePacket{ID: -7})
	expected := frame.Encode(HeaderServerTargetDelete, []byte{0xf9, 0xff, 0xff, 0xff})
	if !bytes.Equal(raw, expected) {
		t.Fatalf("unexpected server target-delete encoding: got %x want %x", raw, expected)
	}

	decoded, err := DecodeServerTargetDelete(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode server target-delete: %v", err)
	}
	if decoded.ID != -7 {
		t.Fatalf("unexpected server target-delete packet: %+v", decoded)
	}
}

func TestEncodeServerDuelStartUsesLegacyVariablePayloadLayout(t *testing.T) {
	raw := EncodeServerDuelStart(ServerDuelStartPacket{OpponentVIDs: []uint32{0x01020304, 0x05060708}})
	expected := frame.Encode(HeaderServerDuelStart, []byte{0x04, 0x03, 0x02, 0x01, 0x08, 0x07, 0x06, 0x05})
	if !bytes.Equal(raw, expected) {
		t.Fatalf("unexpected server duel-start encoding: got %x want %x", raw, expected)
	}

	decoded, err := DecodeServerDuelStart(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode server duel-start: %v", err)
	}
	if len(decoded.OpponentVIDs) != 2 || decoded.OpponentVIDs[0] != 0x01020304 || decoded.OpponentVIDs[1] != 0x05060708 {
		t.Fatalf("unexpected server duel-start packet: %+v", decoded)
	}
}

func TestEncodeServerDuelStartAllowsEmptyOpponentList(t *testing.T) {
	raw := EncodeServerDuelStart(ServerDuelStartPacket{})
	expected := frame.Encode(HeaderServerDuelStart, nil)
	if !bytes.Equal(raw, expected) {
		t.Fatalf("unexpected empty server duel-start encoding: got %x want %x", raw, expected)
	}

	decoded, err := DecodeServerDuelStart(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode empty server duel-start: %v", err)
	}
	if len(decoded.OpponentVIDs) != 0 {
		t.Fatalf("unexpected empty server duel-start packet: %+v", decoded)
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

func TestDecodeClientUseSkillRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientUseSkill(frame.Frame{Header: HeaderClientAttack, Length: 12, Payload: []byte{0x23, 0x00, 0x00, 0x00, 0x07, 0x01, 0x04, 0x02}})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientUseSkillRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeClientUseSkill(frame.Frame{Header: HeaderClientUseSkill, Length: 11, Payload: []byte{0x23, 0x00, 0x00, 0x00, 0x07, 0x01, 0x04}})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeClientShootRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientShoot(frame.Frame{Header: HeaderClientAttack, Length: 5, Payload: []byte{0x01}})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientShootRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeClientShoot(frame.Frame{Header: HeaderClientShoot, Length: 6, Payload: []byte{0x01, 0x02}})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeClientFlyTargetingRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientFlyTargeting(frame.Frame{Header: HeaderClientShoot, Length: 16, Payload: make([]byte, clientFlyTargetingPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientFlyTargetingRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeClientFlyTargeting(frame.Frame{Header: HeaderClientFlyTargeting, Length: 15, Payload: make([]byte, clientFlyTargetingPayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeClientAddFlyTargetingRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientAddFlyTargeting(frame.Frame{Header: HeaderClientFlyTargeting, Length: 16, Payload: make([]byte, clientFlyTargetingPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientAddFlyTargetingRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeClientAddFlyTargeting(frame.Frame{Header: HeaderClientAddFlyTargeting, Length: 15, Payload: make([]byte, clientFlyTargetingPayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeClientCharacterPositionRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientCharacterPosition(frame.Frame{Header: HeaderClientTarget, Length: 5, Payload: []byte{0x01}})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientCharacterPositionRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeClientCharacterPosition(frame.Frame{Header: HeaderClientCharacterPosition, Length: 6, Payload: []byte{0x01, 0x02}})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeClientOnClickRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientOnClick(frame.Frame{Header: HeaderClientTarget, Length: 8, Payload: []byte{0x07, 0x01, 0x04, 0x02}})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientOnClickRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeClientOnClick(frame.Frame{Header: HeaderClientOnClick, Length: 7, Payload: []byte{0x07, 0x01, 0x04}})
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

func TestDecodeServerFlyTargetingRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeServerFlyTargeting(frame.Frame{Header: HeaderServerDamageInfo, Length: 20, Payload: make([]byte, serverFlyTargetingPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeServerFlyTargetingRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeServerFlyTargeting(frame.Frame{Header: HeaderServerFlyTargeting, Length: 19, Payload: make([]byte, serverFlyTargetingPayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeServerAddFlyTargetingRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeServerAddFlyTargeting(frame.Frame{Header: HeaderServerFlyTargeting, Length: 20, Payload: make([]byte, serverFlyTargetingPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeServerAddFlyTargetingRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeServerAddFlyTargeting(frame.Frame{Header: HeaderServerAddFlyTargeting, Length: 19, Payload: make([]byte, serverFlyTargetingPayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeServerCreateFlyRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeServerCreateFly(frame.Frame{Header: HeaderServerFlyTargeting, Length: 13, Payload: make([]byte, serverCreateFlyPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeServerCreateFlyRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeServerCreateFly(frame.Frame{Header: HeaderServerCreateFly, Length: 12, Payload: make([]byte, serverCreateFlyPayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeServerPVPRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeServerPVP(frame.Frame{Header: HeaderServerCreateFly, Length: 13, Payload: make([]byte, serverPVPPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeServerPVPRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeServerPVP(frame.Frame{Header: HeaderServerPVP, Length: 12, Payload: make([]byte, serverPVPPayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeServerDuelStartRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeServerDuelStart(frame.Frame{Header: HeaderServerPVP, Length: 8, Payload: []byte{0x04, 0x03, 0x02, 0x01}})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeServerDuelStartRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeServerDuelStart(frame.Frame{Header: HeaderServerDuelStart, Length: 7, Payload: []byte{0x04, 0x03, 0x02}})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestEncodeServerTargetCreateNewRejectsTooLongName(t *testing.T) {
	name := "123456789012345678901234567890123"
	_, err := EncodeServerTargetCreateNew(ServerTargetCreateNewPacket{TargetName: name})
	if !errors.Is(err, ErrStringTooLong) {
		t.Fatalf("expected ErrStringTooLong, got %v", err)
	}
}

func TestDecodeServerTargetCreateNewRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeServerTargetCreateNew(frame.Frame{Header: HeaderServerTargetUpdate, Length: 46, Payload: make([]byte, serverTargetCreateNewPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeServerTargetCreateNewRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeServerTargetCreateNew(frame.Frame{Header: HeaderServerTargetCreateNew, Length: 45, Payload: make([]byte, serverTargetCreateNewPayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeServerTargetUpdateRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeServerTargetUpdate(frame.Frame{Header: HeaderServerTargetDelete, Length: 16, Payload: make([]byte, serverTargetUpdatePayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeServerTargetUpdateRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeServerTargetUpdate(frame.Frame{Header: HeaderServerTargetUpdate, Length: 15, Payload: make([]byte, serverTargetUpdatePayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeServerTargetDeleteRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeServerTargetDelete(frame.Frame{Header: HeaderServerTargetUpdate, Length: 8, Payload: make([]byte, serverTargetDeletePayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeServerTargetDeleteRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeServerTargetDelete(frame.Frame{Header: HeaderServerTargetDelete, Length: 7, Payload: make([]byte, serverTargetDeletePayloadSize-1)})
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
