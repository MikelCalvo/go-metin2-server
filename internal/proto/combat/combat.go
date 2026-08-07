package combat

import (
	"bytes"
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
	HeaderServerFlyTargeting      uint16 = 0x0411
	HeaderServerAddFlyTargeting   uint16 = 0x0412
	HeaderServerCreateFly         uint16 = 0x0413
	HeaderServerPVP               uint16 = 0x0414
	HeaderServerDuelStart         uint16 = 0x0415
	HeaderClientTarget            uint16 = 0x0A01
	HeaderClientOnClick           uint16 = 0x0A02
	HeaderClientCharacterPosition uint16 = 0x0A60
	HeaderServerTarget            uint16 = 0x0A10
	HeaderServerTargetUpdate      uint16 = 0x0A11
	HeaderServerTargetDelete      uint16 = 0x0A12
	HeaderServerTargetCreateNew   uint16 = 0x0A13

	ClientAttackTypeNormal uint8 = 0

	ServerPVPModeNone    uint8 = 0
	ServerPVPModeAgree   uint8 = 1
	ServerPVPModeFight   uint8 = 2
	ServerPVPModeRevenge uint8 = 3

	ServerTargetMarkerTypeNone      uint8 = 0
	ServerTargetMarkerTypeLocation  uint8 = 1
	ServerTargetMarkerTypeCharacter uint8 = 2

	clientAttackPayloadSize            = 7
	clientUseSkillPayloadSize          = 8
	clientShootPayloadSize             = 1
	clientFlyTargetingPayloadSize      = 12
	clientOnClickPayloadSize           = 4
	clientCharacterPositionPayloadSize = 1
	clientTargetPayloadSize            = 4
	serverDamageInfoPayloadSize        = 9
	serverFlyTargetingPayloadSize      = 16
	serverCreateFlyPayloadSize         = 9
	serverPVPPayloadSize               = 9
	serverTargetPayloadSize            = 5
	serverTargetCreateNewNameSize      = 33
	serverTargetCreateNewPayloadSize   = 4 + serverTargetCreateNewNameSize + 4 + 1
	serverTargetUpdatePayloadSize      = 12
	serverTargetDeletePayloadSize      = 4
)

var (
	ErrUnexpectedHeader = errors.New("unexpected combat packet header")
	ErrInvalidPayload   = errors.New("invalid combat packet payload")
	ErrStringTooLong    = errors.New("string does not fit fixed-width combat packet field")
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

type ServerFlyTargetingPacket struct {
	ShooterVID uint32
	TargetVID  uint32
	X          int32
	Y          int32
}

type ServerCreateFlyPacket struct {
	Type     uint8
	StartVID uint32
	EndVID   uint32
}

type ServerPVPPacket struct {
	SourceVID      uint32
	DestinationVID uint32
	Mode           uint8
}

type ServerDuelStartPacket struct {
	OpponentVIDs []uint32
}

type ServerTargetCreateNewPacket struct {
	ID         int32
	TargetName string
	VID        uint32
	Type       uint8
}

type ServerTargetUpdatePacket struct {
	ID int32
	X  int32
	Y  int32
}

type ServerTargetDeletePacket struct {
	ID int32
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

func EncodeServerFlyTargeting(packet ServerFlyTargetingPacket) []byte {
	return encodeServerFlyTargeting(HeaderServerFlyTargeting, packet)
}

func DecodeServerFlyTargeting(f frame.Frame) (ServerFlyTargetingPacket, error) {
	return decodeServerFlyTargeting(f, HeaderServerFlyTargeting)
}

func EncodeServerAddFlyTargeting(packet ServerFlyTargetingPacket) []byte {
	return encodeServerFlyTargeting(HeaderServerAddFlyTargeting, packet)
}

func DecodeServerAddFlyTargeting(f frame.Frame) (ServerFlyTargetingPacket, error) {
	return decodeServerFlyTargeting(f, HeaderServerAddFlyTargeting)
}

func EncodeServerCreateFly(packet ServerCreateFlyPacket) []byte {
	payload := make([]byte, serverCreateFlyPayloadSize)
	payload[0] = packet.Type
	binary.LittleEndian.PutUint32(payload[1:5], packet.StartVID)
	binary.LittleEndian.PutUint32(payload[5:9], packet.EndVID)
	return frame.Encode(HeaderServerCreateFly, payload)
}

func DecodeServerCreateFly(f frame.Frame) (ServerCreateFlyPacket, error) {
	if f.Header != HeaderServerCreateFly {
		return ServerCreateFlyPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != serverCreateFlyPayloadSize {
		return ServerCreateFlyPacket{}, ErrInvalidPayload
	}
	return ServerCreateFlyPacket{
		Type:     f.Payload[0],
		StartVID: binary.LittleEndian.Uint32(f.Payload[1:5]),
		EndVID:   binary.LittleEndian.Uint32(f.Payload[5:9]),
	}, nil
}

func EncodeServerPVP(packet ServerPVPPacket) []byte {
	payload := make([]byte, serverPVPPayloadSize)
	binary.LittleEndian.PutUint32(payload[0:4], packet.SourceVID)
	binary.LittleEndian.PutUint32(payload[4:8], packet.DestinationVID)
	payload[8] = packet.Mode
	return frame.Encode(HeaderServerPVP, payload)
}

func DecodeServerPVP(f frame.Frame) (ServerPVPPacket, error) {
	if f.Header != HeaderServerPVP {
		return ServerPVPPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != serverPVPPayloadSize {
		return ServerPVPPacket{}, ErrInvalidPayload
	}
	return ServerPVPPacket{
		SourceVID:      binary.LittleEndian.Uint32(f.Payload[0:4]),
		DestinationVID: binary.LittleEndian.Uint32(f.Payload[4:8]),
		Mode:           f.Payload[8],
	}, nil
}

func EncodeServerDuelStart(packet ServerDuelStartPacket) []byte {
	payload := make([]byte, len(packet.OpponentVIDs)*4)
	for i, vid := range packet.OpponentVIDs {
		binary.LittleEndian.PutUint32(payload[i*4:(i+1)*4], vid)
	}
	return frame.Encode(HeaderServerDuelStart, payload)
}

func DecodeServerDuelStart(f frame.Frame) (ServerDuelStartPacket, error) {
	if f.Header != HeaderServerDuelStart {
		return ServerDuelStartPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload)%4 != 0 {
		return ServerDuelStartPacket{}, ErrInvalidPayload
	}
	vids := make([]uint32, len(f.Payload)/4)
	for i := range vids {
		vids[i] = binary.LittleEndian.Uint32(f.Payload[i*4 : (i+1)*4])
	}
	return ServerDuelStartPacket{OpponentVIDs: vids}, nil
}

func EncodeServerTargetCreateNew(packet ServerTargetCreateNewPacket) ([]byte, error) {
	payload := make([]byte, serverTargetCreateNewPayloadSize)
	binary.LittleEndian.PutUint32(payload[0:4], uint32(packet.ID))
	if err := putFixedString(payload[4:37], packet.TargetName); err != nil {
		return nil, err
	}
	binary.LittleEndian.PutUint32(payload[37:41], packet.VID)
	payload[41] = packet.Type
	return frame.Encode(HeaderServerTargetCreateNew, payload), nil
}

func DecodeServerTargetCreateNew(f frame.Frame) (ServerTargetCreateNewPacket, error) {
	if f.Header != HeaderServerTargetCreateNew {
		return ServerTargetCreateNewPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != serverTargetCreateNewPayloadSize {
		return ServerTargetCreateNewPacket{}, ErrInvalidPayload
	}
	return ServerTargetCreateNewPacket{
		ID:         int32(binary.LittleEndian.Uint32(f.Payload[0:4])),
		TargetName: parseFixedString(f.Payload[4:37]),
		VID:        binary.LittleEndian.Uint32(f.Payload[37:41]),
		Type:       f.Payload[41],
	}, nil
}

func EncodeServerTargetUpdate(packet ServerTargetUpdatePacket) []byte {
	payload := make([]byte, serverTargetUpdatePayloadSize)
	binary.LittleEndian.PutUint32(payload[0:4], uint32(packet.ID))
	binary.LittleEndian.PutUint32(payload[4:8], uint32(packet.X))
	binary.LittleEndian.PutUint32(payload[8:12], uint32(packet.Y))
	return frame.Encode(HeaderServerTargetUpdate, payload)
}

func DecodeServerTargetUpdate(f frame.Frame) (ServerTargetUpdatePacket, error) {
	if f.Header != HeaderServerTargetUpdate {
		return ServerTargetUpdatePacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != serverTargetUpdatePayloadSize {
		return ServerTargetUpdatePacket{}, ErrInvalidPayload
	}
	return ServerTargetUpdatePacket{
		ID: int32(binary.LittleEndian.Uint32(f.Payload[0:4])),
		X:  int32(binary.LittleEndian.Uint32(f.Payload[4:8])),
		Y:  int32(binary.LittleEndian.Uint32(f.Payload[8:12])),
	}, nil
}

func EncodeServerTargetDelete(packet ServerTargetDeletePacket) []byte {
	payload := make([]byte, serverTargetDeletePayloadSize)
	binary.LittleEndian.PutUint32(payload, uint32(packet.ID))
	return frame.Encode(HeaderServerTargetDelete, payload)
}

func DecodeServerTargetDelete(f frame.Frame) (ServerTargetDeletePacket, error) {
	if f.Header != HeaderServerTargetDelete {
		return ServerTargetDeletePacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != serverTargetDeletePayloadSize {
		return ServerTargetDeletePacket{}, ErrInvalidPayload
	}
	return ServerTargetDeletePacket{ID: int32(binary.LittleEndian.Uint32(f.Payload))}, nil
}

func encodeServerFlyTargeting(header uint16, packet ServerFlyTargetingPacket) []byte {
	payload := make([]byte, serverFlyTargetingPayloadSize)
	binary.LittleEndian.PutUint32(payload[0:4], packet.ShooterVID)
	binary.LittleEndian.PutUint32(payload[4:8], packet.TargetVID)
	binary.LittleEndian.PutUint32(payload[8:12], uint32(packet.X))
	binary.LittleEndian.PutUint32(payload[12:16], uint32(packet.Y))
	return frame.Encode(header, payload)
}

func putFixedString(dst []byte, value string) error {
	if len(value) >= len(dst) {
		return ErrStringTooLong
	}
	copy(dst, value)
	return nil
}

func parseFixedString(src []byte) string {
	if idx := bytes.IndexByte(src, 0); idx >= 0 {
		return string(src[:idx])
	}
	return string(src)
}

func decodeServerFlyTargeting(f frame.Frame, header uint16) (ServerFlyTargetingPacket, error) {
	if f.Header != header {
		return ServerFlyTargetingPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != serverFlyTargetingPayloadSize {
		return ServerFlyTargetingPacket{}, ErrInvalidPayload
	}
	return ServerFlyTargetingPacket{
		ShooterVID: binary.LittleEndian.Uint32(f.Payload[0:4]),
		TargetVID:  binary.LittleEndian.Uint32(f.Payload[4:8]),
		X:          int32(binary.LittleEndian.Uint32(f.Payload[8:12])),
		Y:          int32(binary.LittleEndian.Uint32(f.Payload[12:16])),
	}, nil
}
