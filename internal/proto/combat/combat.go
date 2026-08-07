package combat

import (
	"encoding/binary"
	"errors"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

const (
	HeaderClientAttack            uint16 = 0x0401
	HeaderClientUseSkill          uint16 = 0x0402
	HeaderClientShoot             uint16 = 0x0403
	HeaderClientFlyTargeting      uint16 = 0x0404
	HeaderClientAddFlyTargeting   uint16 = 0x0405
	HeaderServerDamageInfo        uint16 = 0x0410
	HeaderClientTarget            uint16 = 0x0A01
	HeaderClientOnClick           uint16 = 0x0A02
	HeaderClientCharacterPosition uint16 = 0x0A60
	HeaderServerTarget            uint16 = 0x0A10

	ClientAttackTypeNormal uint8 = 0

	clientAttackPayloadSize            = 7
	clientUseSkillPayloadSize          = 8
	clientShootPayloadSize             = 1
	clientFlyTargetingPayloadSize      = 12
	clientOnClickPayloadSize           = 4
	clientCharacterPositionPayloadSize = 1
	clientTargetPayloadSize            = 4
	serverDamageInfoPayloadSize        = 9
	serverTargetPayloadSize            = 5
)

var (
	ErrUnexpectedHeader = errors.New("unexpected combat packet header")
	ErrInvalidPayload   = errors.New("invalid combat packet payload")
)

type ClientAttackPacket struct {
	AttackType   uint8
	TargetVID    uint32
	CRCProcPiece uint8
	CRCFilePiece uint8
}

type ClientUseSkillPacket struct {
	SkillVnum uint32
	TargetVID uint32
}

type ClientShootPacket struct {
	ShootType uint8
}

type ClientFlyTargetingPacket struct {
	TargetVID uint32
	X         int32
	Y         int32
}

type ClientOnClickPacket struct {
	VID uint32
}

type ClientCharacterPositionPacket struct {
	Position uint8
}

type ClientTargetPacket struct {
	TargetVID uint32
}

type ServerTargetPacket struct {
	TargetVID uint32
	HPPercent uint8
}

type ServerDamageInfoPacket struct {
	VID    uint32
	Flag   uint8
	Damage int32
}

func EncodeClientAttack(packet ClientAttackPacket) []byte {
	payload := make([]byte, clientAttackPayloadSize)
	payload[0] = packet.AttackType
	binary.LittleEndian.PutUint32(payload[1:5], packet.TargetVID)
	payload[5] = packet.CRCProcPiece
	payload[6] = packet.CRCFilePiece
	return frame.Encode(HeaderClientAttack, payload)
}

func DecodeClientAttack(f frame.Frame) (ClientAttackPacket, error) {
	if f.Header != HeaderClientAttack {
		return ClientAttackPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientAttackPayloadSize {
		return ClientAttackPacket{}, ErrInvalidPayload
	}
	return ClientAttackPacket{
		AttackType:   f.Payload[0],
		TargetVID:    binary.LittleEndian.Uint32(f.Payload[1:5]),
		CRCProcPiece: f.Payload[5],
		CRCFilePiece: f.Payload[6],
	}, nil
}

func EncodeClientUseSkill(packet ClientUseSkillPacket) []byte {
	payload := make([]byte, clientUseSkillPayloadSize)
	binary.LittleEndian.PutUint32(payload[0:4], packet.SkillVnum)
	binary.LittleEndian.PutUint32(payload[4:8], packet.TargetVID)
	return frame.Encode(HeaderClientUseSkill, payload)
}

func DecodeClientUseSkill(f frame.Frame) (ClientUseSkillPacket, error) {
	if f.Header != HeaderClientUseSkill {
		return ClientUseSkillPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientUseSkillPayloadSize {
		return ClientUseSkillPacket{}, ErrInvalidPayload
	}
	return ClientUseSkillPacket{
		SkillVnum: binary.LittleEndian.Uint32(f.Payload[0:4]),
		TargetVID: binary.LittleEndian.Uint32(f.Payload[4:8]),
	}, nil
}

func EncodeClientShoot(packet ClientShootPacket) []byte {
	payload := []byte{packet.ShootType}
	return frame.Encode(HeaderClientShoot, payload)
}

func DecodeClientShoot(f frame.Frame) (ClientShootPacket, error) {
	if f.Header != HeaderClientShoot {
		return ClientShootPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientShootPayloadSize {
		return ClientShootPacket{}, ErrInvalidPayload
	}
	return ClientShootPacket{ShootType: f.Payload[0]}, nil
}

func EncodeClientFlyTargeting(packet ClientFlyTargetingPacket) []byte {
	return encodeClientFlyTargeting(HeaderClientFlyTargeting, packet)
}

func DecodeClientFlyTargeting(f frame.Frame) (ClientFlyTargetingPacket, error) {
	return decodeClientFlyTargeting(f, HeaderClientFlyTargeting)
}

func EncodeClientAddFlyTargeting(packet ClientFlyTargetingPacket) []byte {
	return encodeClientFlyTargeting(HeaderClientAddFlyTargeting, packet)
}

func DecodeClientAddFlyTargeting(f frame.Frame) (ClientFlyTargetingPacket, error) {
	return decodeClientFlyTargeting(f, HeaderClientAddFlyTargeting)
}

func EncodeClientCharacterPosition(packet ClientCharacterPositionPacket) []byte {
	return frame.Encode(HeaderClientCharacterPosition, []byte{packet.Position})
}

func DecodeClientCharacterPosition(f frame.Frame) (ClientCharacterPositionPacket, error) {
	if f.Header != HeaderClientCharacterPosition {
		return ClientCharacterPositionPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientCharacterPositionPayloadSize {
		return ClientCharacterPositionPacket{}, ErrInvalidPayload
	}
	return ClientCharacterPositionPacket{Position: f.Payload[0]}, nil
}

func EncodeClientOnClick(packet ClientOnClickPacket) []byte {
	payload := make([]byte, clientOnClickPayloadSize)
	binary.LittleEndian.PutUint32(payload, packet.VID)
	return frame.Encode(HeaderClientOnClick, payload)
}

func DecodeClientOnClick(f frame.Frame) (ClientOnClickPacket, error) {
	if f.Header != HeaderClientOnClick {
		return ClientOnClickPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientOnClickPayloadSize {
		return ClientOnClickPacket{}, ErrInvalidPayload
	}
	return ClientOnClickPacket{VID: binary.LittleEndian.Uint32(f.Payload)}, nil
}

func encodeClientFlyTargeting(header uint16, packet ClientFlyTargetingPacket) []byte {
	payload := make([]byte, clientFlyTargetingPayloadSize)
	binary.LittleEndian.PutUint32(payload[0:4], packet.TargetVID)
	binary.LittleEndian.PutUint32(payload[4:8], uint32(packet.X))
	binary.LittleEndian.PutUint32(payload[8:12], uint32(packet.Y))
	return frame.Encode(header, payload)
}

func decodeClientFlyTargeting(f frame.Frame, header uint16) (ClientFlyTargetingPacket, error) {
	if f.Header != header {
		return ClientFlyTargetingPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientFlyTargetingPayloadSize {
		return ClientFlyTargetingPacket{}, ErrInvalidPayload
	}
	return ClientFlyTargetingPacket{
		TargetVID: binary.LittleEndian.Uint32(f.Payload[0:4]),
		X:         int32(binary.LittleEndian.Uint32(f.Payload[4:8])),
		Y:         int32(binary.LittleEndian.Uint32(f.Payload[8:12])),
	}, nil
}

func EncodeClientTarget(packet ClientTargetPacket) []byte {
	payload := make([]byte, clientTargetPayloadSize)
	binary.LittleEndian.PutUint32(payload, packet.TargetVID)
	return frame.Encode(HeaderClientTarget, payload)
}

func DecodeClientTarget(f frame.Frame) (ClientTargetPacket, error) {
	if f.Header != HeaderClientTarget {
		return ClientTargetPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientTargetPayloadSize {
		return ClientTargetPacket{}, ErrInvalidPayload
	}
	return ClientTargetPacket{TargetVID: binary.LittleEndian.Uint32(f.Payload)}, nil
}

func EncodeServerTarget(packet ServerTargetPacket) []byte {
	payload := make([]byte, serverTargetPayloadSize)
	binary.LittleEndian.PutUint32(payload, packet.TargetVID)
	payload[4] = packet.HPPercent
	return frame.Encode(HeaderServerTarget, payload)
}

func EncodeServerClearTarget() []byte {
	return EncodeServerTarget(ServerTargetPacket{})
}

func DecodeServerTarget(f frame.Frame) (ServerTargetPacket, error) {
	if f.Header != HeaderServerTarget {
		return ServerTargetPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != serverTargetPayloadSize {
		return ServerTargetPacket{}, ErrInvalidPayload
	}
	return ServerTargetPacket{TargetVID: binary.LittleEndian.Uint32(f.Payload), HPPercent: f.Payload[4]}, nil
}

func EncodeServerDamageInfo(packet ServerDamageInfoPacket) []byte {
	payload := make([]byte, serverDamageInfoPayloadSize)
	binary.LittleEndian.PutUint32(payload[0:4], packet.VID)
	payload[4] = packet.Flag
	binary.LittleEndian.PutUint32(payload[5:9], uint32(packet.Damage))
	return frame.Encode(HeaderServerDamageInfo, payload)
}

func DecodeServerDamageInfo(f frame.Frame) (ServerDamageInfoPacket, error) {
	if f.Header != HeaderServerDamageInfo {
		return ServerDamageInfoPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != serverDamageInfoPayloadSize {
		return ServerDamageInfoPacket{}, ErrInvalidPayload
	}
	return ServerDamageInfoPacket{
		VID:    binary.LittleEndian.Uint32(f.Payload[0:4]),
		Flag:   f.Payload[4],
		Damage: int32(binary.LittleEndian.Uint32(f.Payload[5:9])),
	}, nil
}
