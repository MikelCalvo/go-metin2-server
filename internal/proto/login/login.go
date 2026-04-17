package login

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

const (
	HeaderLogin2        uint16 = 0x0101
	HeaderLoginSuccess3 uint16 = 0x0104
	HeaderLoginSuccess4 uint16 = 0x0105
	HeaderLoginFailure  uint16 = 0x0106
	HeaderLoginKey      uint16 = 0x0107
	HeaderEmpire        uint16 = 0x0109
	HeaderEmpireSelect  uint16 = 0x010A

	LoginFieldSize          = 31
	StatusFieldSize         = 9
	CharacterNameFieldSize  = 65
	GuildNameFieldSize      = 13
	PlayerCount             = 4
	simplePlayerPayloadSize = 103
	login2PayloadSize       = LoginFieldSize + 4
	loginSuccess4PayloadLen = PlayerCount*simplePlayerPayloadSize + PlayerCount*4 + PlayerCount*GuildNameFieldSize + 4 + 4
)

var (
	ErrUnexpectedHeader = errors.New("unexpected login packet header")
	ErrInvalidPayload   = errors.New("invalid login packet payload")
	ErrStringTooLong    = errors.New("string does not fit fixed-width wire field")
)

type Login2Packet struct {
	Login    string
	LoginKey uint32
}

type LoginFailurePacket struct {
	Status string
}

type EmpirePacket struct {
	Empire uint8
}

type EmpireSelectPacket struct {
	Empire uint8
}

type SimplePlayer struct {
	ID          uint32
	Name        string
	Job         uint8
	Level       uint8
	PlayMinutes uint32
	ST          uint8
	HT          uint8
	DX          uint8
	IQ          uint8
	MainPart    uint16
	ChangeName  uint8
	HairPart    uint16
	Dummy       [4]byte
	X           int32
	Y           int32
	Addr        uint32
	Port        uint16
	SkillGroup  uint8
}

type LoginSuccess4Packet struct {
	Players    [PlayerCount]SimplePlayer
	GuildIDs   [PlayerCount]uint32
	GuildNames [PlayerCount]string
	Handle     uint32
	RandomKey  uint32
}

func EncodeLogin2(packet Login2Packet) ([]byte, error) {
	payload := make([]byte, login2PayloadSize)
	if err := putFixedString(payload[:LoginFieldSize], packet.Login); err != nil {
		return nil, err
	}

	binary.LittleEndian.PutUint32(payload[LoginFieldSize:], packet.LoginKey)
	return frame.Encode(HeaderLogin2, payload), nil
}

func DecodeLogin2(f frame.Frame) (Login2Packet, error) {
	if f.Header != HeaderLogin2 {
		return Login2Packet{}, ErrUnexpectedHeader
	}

	if len(f.Payload) != login2PayloadSize {
		return Login2Packet{}, ErrInvalidPayload
	}

	return Login2Packet{
		Login:    parseFixedString(f.Payload[:LoginFieldSize]),
		LoginKey: binary.LittleEndian.Uint32(f.Payload[LoginFieldSize:]),
	}, nil
}

func EncodeLoginFailure(packet LoginFailurePacket) ([]byte, error) {
	payload := make([]byte, StatusFieldSize)
	if err := putFixedString(payload, packet.Status); err != nil {
		return nil, err
	}

	return frame.Encode(HeaderLoginFailure, payload), nil
}

func DecodeLoginFailure(f frame.Frame) (LoginFailurePacket, error) {
	if f.Header != HeaderLoginFailure {
		return LoginFailurePacket{}, ErrUnexpectedHeader
	}

	if len(f.Payload) != StatusFieldSize {
		return LoginFailurePacket{}, ErrInvalidPayload
	}

	return LoginFailurePacket{Status: parseFixedString(f.Payload)}, nil
}

func EncodeEmpire(packet EmpirePacket) []byte {
	return frame.Encode(HeaderEmpire, []byte{packet.Empire})
}

func EncodeEmpireSelect(packet EmpireSelectPacket) []byte {
	return frame.Encode(HeaderEmpireSelect, []byte{packet.Empire})
}

func DecodeEmpire(f frame.Frame) (EmpirePacket, error) {
	if f.Header != HeaderEmpire {
		return EmpirePacket{}, ErrUnexpectedHeader
	}

	if len(f.Payload) != 1 {
		return EmpirePacket{}, ErrInvalidPayload
	}

	return EmpirePacket{Empire: f.Payload[0]}, nil
}

func DecodeEmpireSelect(f frame.Frame) (EmpireSelectPacket, error) {
	if f.Header != HeaderEmpireSelect {
		return EmpireSelectPacket{}, ErrUnexpectedHeader
	}

	if len(f.Payload) != 1 {
		return EmpireSelectPacket{}, ErrInvalidPayload
	}

	return EmpireSelectPacket{Empire: f.Payload[0]}, nil
}

func EncodeLoginSuccess4(packet LoginSuccess4Packet) ([]byte, error) {
	payload := make([]byte, loginSuccess4PayloadLen)
	offset := 0

	for _, player := range packet.Players {
		encoded, err := encodeSimplePlayer(player)
		if err != nil {
			return nil, err
		}

		copy(payload[offset:], encoded)
		offset += simplePlayerPayloadSize
	}

	for _, guildID := range packet.GuildIDs {
		binary.LittleEndian.PutUint32(payload[offset:], guildID)
		offset += 4
	}

	for _, guildName := range packet.GuildNames {
		if err := putFixedString(payload[offset:offset+GuildNameFieldSize], guildName); err != nil {
			return nil, err
		}
		offset += GuildNameFieldSize
	}

	binary.LittleEndian.PutUint32(payload[offset:], packet.Handle)
	offset += 4
	binary.LittleEndian.PutUint32(payload[offset:], packet.RandomKey)

	return frame.Encode(HeaderLoginSuccess4, payload), nil
}

func DecodeLoginSuccess4(f frame.Frame) (LoginSuccess4Packet, error) {
	if f.Header != HeaderLoginSuccess4 {
		return LoginSuccess4Packet{}, ErrUnexpectedHeader
	}

	if len(f.Payload) != loginSuccess4PayloadLen {
		return LoginSuccess4Packet{}, ErrInvalidPayload
	}

	var packet LoginSuccess4Packet
	offset := 0

	for i := range packet.Players {
		player, err := decodeSimplePlayer(f.Payload[offset : offset+simplePlayerPayloadSize])
		if err != nil {
			return LoginSuccess4Packet{}, err
		}
		packet.Players[i] = player
		offset += simplePlayerPayloadSize
	}

	for i := range packet.GuildIDs {
		packet.GuildIDs[i] = binary.LittleEndian.Uint32(f.Payload[offset:])
		offset += 4
	}

	for i := range packet.GuildNames {
		packet.GuildNames[i] = parseFixedString(f.Payload[offset : offset+GuildNameFieldSize])
		offset += GuildNameFieldSize
	}

	packet.Handle = binary.LittleEndian.Uint32(f.Payload[offset:])
	offset += 4
	packet.RandomKey = binary.LittleEndian.Uint32(f.Payload[offset:])

	return packet, nil
}

func encodeSimplePlayer(player SimplePlayer) ([]byte, error) {
	payload := make([]byte, simplePlayerPayloadSize)
	offset := 0

	binary.LittleEndian.PutUint32(payload[offset:], player.ID)
	offset += 4
	if err := putFixedString(payload[offset:offset+CharacterNameFieldSize], player.Name); err != nil {
		return nil, err
	}
	offset += CharacterNameFieldSize
	payload[offset] = player.Job
	offset++
	payload[offset] = player.Level
	offset++
	binary.LittleEndian.PutUint32(payload[offset:], player.PlayMinutes)
	offset += 4
	payload[offset] = player.ST
	offset++
	payload[offset] = player.HT
	offset++
	payload[offset] = player.DX
	offset++
	payload[offset] = player.IQ
	offset++
	binary.LittleEndian.PutUint16(payload[offset:], player.MainPart)
	offset += 2
	payload[offset] = player.ChangeName
	offset++
	binary.LittleEndian.PutUint16(payload[offset:], player.HairPart)
	offset += 2
	copy(payload[offset:offset+4], player.Dummy[:])
	offset += 4
	binary.LittleEndian.PutUint32(payload[offset:], uint32(player.X))
	offset += 4
	binary.LittleEndian.PutUint32(payload[offset:], uint32(player.Y))
	offset += 4
	binary.LittleEndian.PutUint32(payload[offset:], player.Addr)
	offset += 4
	binary.LittleEndian.PutUint16(payload[offset:], player.Port)
	offset += 2
	payload[offset] = player.SkillGroup

	return payload, nil
}

func decodeSimplePlayer(payload []byte) (SimplePlayer, error) {
	if len(payload) != simplePlayerPayloadSize {
		return SimplePlayer{}, ErrInvalidPayload
	}

	player := SimplePlayer{}
	offset := 0
	player.ID = binary.LittleEndian.Uint32(payload[offset:])
	offset += 4
	player.Name = parseFixedString(payload[offset : offset+CharacterNameFieldSize])
	offset += CharacterNameFieldSize
	player.Job = payload[offset]
	offset++
	player.Level = payload[offset]
	offset++
	player.PlayMinutes = binary.LittleEndian.Uint32(payload[offset:])
	offset += 4
	player.ST = payload[offset]
	offset++
	player.HT = payload[offset]
	offset++
	player.DX = payload[offset]
	offset++
	player.IQ = payload[offset]
	offset++
	player.MainPart = binary.LittleEndian.Uint16(payload[offset:])
	offset += 2
	player.ChangeName = payload[offset]
	offset++
	player.HairPart = binary.LittleEndian.Uint16(payload[offset:])
	offset += 2
	copy(player.Dummy[:], payload[offset:offset+4])
	offset += 4
	player.X = int32(binary.LittleEndian.Uint32(payload[offset:]))
	offset += 4
	player.Y = int32(binary.LittleEndian.Uint32(payload[offset:]))
	offset += 4
	player.Addr = binary.LittleEndian.Uint32(payload[offset:])
	offset += 4
	player.Port = binary.LittleEndian.Uint16(payload[offset:])
	offset += 2
	player.SkillGroup = payload[offset]

	return player, nil
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
