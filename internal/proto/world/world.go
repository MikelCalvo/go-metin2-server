package world

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"

	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

const (
	HeaderCharacterCreate      uint16 = 0x0201
	HeaderCharacterSelect      uint16 = 0x0203
	HeaderEnterGame            uint16 = 0x0204
	HeaderPlayerCreateSuccess  uint16 = 0x020C
	HeaderPlayerCreateFailure  uint16 = 0x020D
	HeaderMainCharacter        uint16 = 0x0210
	HeaderPlayerPoints         uint16 = 0x0214

	CharacterNameFieldSize = 65
	BGMNameFieldSize       = 25
	PointCount             = 255

	characterCreatePayloadSize     = 1 + CharacterNameFieldSize + 2 + 1 + 4
	characterSelectPayloadSize     = 1
	playerCreateSuccessPayloadSize = 1 + simplePlayerPayloadSize
	playerCreateFailurePayloadSize = 1
	mainCharacterPayloadSize       = 114
	playerPointsPayloadSize        = PointCount * 4
	simplePlayerPayloadSize        = 103
)

var (
	ErrUnexpectedHeader = errors.New("unexpected world packet header")
	ErrInvalidPayload   = errors.New("invalid world packet payload")
	ErrStringTooLong    = errors.New("string does not fit fixed-width wire field")
)

type CharacterCreatePacket struct {
	Index   uint8
	Name    string
	RaceNum uint16
	Shape   uint8
	Con     uint8
	Int     uint8
	Str     uint8
	Dex     uint8
}

type CharacterSelectPacket struct {
	Index uint8
}

type PlayerCreateSuccessPacket struct {
	Index  uint8
	Player loginproto.SimplePlayer
}

type PlayerCreateFailurePacket struct {
	Type uint8
}

type MainCharacterPacket struct {
	VID        uint32
	RaceNum    uint16
	Name       string
	BGMName    string
	BGMVolume  float32
	X          int32
	Y          int32
	Z          int32
	Empire     uint8
	SkillGroup uint8
}

type PlayerPointsPacket struct {
	Points [PointCount]int32
}

func EncodeCharacterCreate(packet CharacterCreatePacket) ([]byte, error) {
	payload := make([]byte, characterCreatePayloadSize)
	payload[0] = packet.Index
	if err := putFixedString(payload[1:1+CharacterNameFieldSize], packet.Name); err != nil {
		return nil, err
	}
	offset := 1 + CharacterNameFieldSize
	binary.LittleEndian.PutUint16(payload[offset:], packet.RaceNum)
	offset += 2
	payload[offset] = packet.Shape
	offset++
	payload[offset] = packet.Con
	offset++
	payload[offset] = packet.Int
	offset++
	payload[offset] = packet.Str
	offset++
	payload[offset] = packet.Dex
	return frame.Encode(HeaderCharacterCreate, payload), nil
}

func DecodeCharacterCreate(f frame.Frame) (CharacterCreatePacket, error) {
	if f.Header != HeaderCharacterCreate {
		return CharacterCreatePacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != characterCreatePayloadSize {
		return CharacterCreatePacket{}, ErrInvalidPayload
	}

	packet := CharacterCreatePacket{Index: f.Payload[0], Name: parseFixedString(f.Payload[1 : 1+CharacterNameFieldSize])}
	offset := 1 + CharacterNameFieldSize
	packet.RaceNum = binary.LittleEndian.Uint16(f.Payload[offset:])
	offset += 2
	packet.Shape = f.Payload[offset]
	offset++
	packet.Con = f.Payload[offset]
	offset++
	packet.Int = f.Payload[offset]
	offset++
	packet.Str = f.Payload[offset]
	offset++
	packet.Dex = f.Payload[offset]
	return packet, nil
}

func EncodeCharacterSelect(packet CharacterSelectPacket) []byte {
	return frame.Encode(HeaderCharacterSelect, []byte{packet.Index})
}

func DecodeCharacterSelect(f frame.Frame) (CharacterSelectPacket, error) {
	if f.Header != HeaderCharacterSelect {
		return CharacterSelectPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != characterSelectPayloadSize {
		return CharacterSelectPacket{}, ErrInvalidPayload
	}
	return CharacterSelectPacket{Index: f.Payload[0]}, nil
}

func EncodeEnterGame() []byte {
	return frame.Encode(HeaderEnterGame, nil)
}

func DecodeEnterGame(f frame.Frame) error {
	if f.Header != HeaderEnterGame {
		return ErrUnexpectedHeader
	}
	if len(f.Payload) != 0 {
		return ErrInvalidPayload
	}
	return nil
}

func EncodePlayerCreateSuccess(packet PlayerCreateSuccessPacket) ([]byte, error) {
	payload := make([]byte, playerCreateSuccessPayloadSize)
	payload[0] = packet.Index
	playerPayload, err := encodeSimplePlayerPayload(packet.Player)
	if err != nil {
		return nil, err
	}
	copy(payload[1:], playerPayload)
	return frame.Encode(HeaderPlayerCreateSuccess, payload), nil
}

func DecodePlayerCreateSuccess(f frame.Frame) (PlayerCreateSuccessPacket, error) {
	if f.Header != HeaderPlayerCreateSuccess {
		return PlayerCreateSuccessPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != playerCreateSuccessPayloadSize {
		return PlayerCreateSuccessPacket{}, ErrInvalidPayload
	}
	player, err := decodeSimplePlayerPayload(f.Payload[1:])
	if err != nil {
		return PlayerCreateSuccessPacket{}, err
	}
	return PlayerCreateSuccessPacket{Index: f.Payload[0], Player: player}, nil
}

func EncodePlayerCreateFailure(packet PlayerCreateFailurePacket) []byte {
	return frame.Encode(HeaderPlayerCreateFailure, []byte{packet.Type})
}

func DecodePlayerCreateFailure(f frame.Frame) (PlayerCreateFailurePacket, error) {
	if f.Header != HeaderPlayerCreateFailure {
		return PlayerCreateFailurePacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != playerCreateFailurePayloadSize {
		return PlayerCreateFailurePacket{}, ErrInvalidPayload
	}
	return PlayerCreateFailurePacket{Type: f.Payload[0]}, nil
}

func EncodeMainCharacter(packet MainCharacterPacket) ([]byte, error) {
	payload := make([]byte, mainCharacterPayloadSize)
	offset := 0
	binary.LittleEndian.PutUint32(payload[offset:], packet.VID)
	offset += 4
	binary.LittleEndian.PutUint16(payload[offset:], packet.RaceNum)
	offset += 2
	if err := putFixedString(payload[offset:offset+CharacterNameFieldSize], packet.Name); err != nil {
		return nil, err
	}
	offset += CharacterNameFieldSize
	if err := putFixedString(payload[offset:offset+BGMNameFieldSize], packet.BGMName); err != nil {
		return nil, err
	}
	offset += BGMNameFieldSize
	binary.LittleEndian.PutUint32(payload[offset:], math.Float32bits(packet.BGMVolume))
	offset += 4
	binary.LittleEndian.PutUint32(payload[offset:], uint32(packet.X))
	offset += 4
	binary.LittleEndian.PutUint32(payload[offset:], uint32(packet.Y))
	offset += 4
	binary.LittleEndian.PutUint32(payload[offset:], uint32(packet.Z))
	offset += 4
	payload[offset] = packet.Empire
	offset++
	payload[offset] = packet.SkillGroup
	return frame.Encode(HeaderMainCharacter, payload), nil
}

func DecodeMainCharacter(f frame.Frame) (MainCharacterPacket, error) {
	if f.Header != HeaderMainCharacter {
		return MainCharacterPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != mainCharacterPayloadSize {
		return MainCharacterPacket{}, ErrInvalidPayload
	}
	var packet MainCharacterPacket
	offset := 0
	packet.VID = binary.LittleEndian.Uint32(f.Payload[offset:])
	offset += 4
	packet.RaceNum = binary.LittleEndian.Uint16(f.Payload[offset:])
	offset += 2
	packet.Name = parseFixedString(f.Payload[offset : offset+CharacterNameFieldSize])
	offset += CharacterNameFieldSize
	packet.BGMName = parseFixedString(f.Payload[offset : offset+BGMNameFieldSize])
	offset += BGMNameFieldSize
	packet.BGMVolume = math.Float32frombits(binary.LittleEndian.Uint32(f.Payload[offset:]))
	offset += 4
	packet.X = int32(binary.LittleEndian.Uint32(f.Payload[offset:]))
	offset += 4
	packet.Y = int32(binary.LittleEndian.Uint32(f.Payload[offset:]))
	offset += 4
	packet.Z = int32(binary.LittleEndian.Uint32(f.Payload[offset:]))
	offset += 4
	packet.Empire = f.Payload[offset]
	offset++
	packet.SkillGroup = f.Payload[offset]
	return packet, nil
}

func EncodePlayerPoints(packet PlayerPointsPacket) []byte {
	payload := make([]byte, playerPointsPayloadSize)
	offset := 0
	for _, value := range packet.Points {
		binary.LittleEndian.PutUint32(payload[offset:], uint32(value))
		offset += 4
	}
	return frame.Encode(HeaderPlayerPoints, payload)
}

func DecodePlayerPoints(f frame.Frame) (PlayerPointsPacket, error) {
	if f.Header != HeaderPlayerPoints {
		return PlayerPointsPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != playerPointsPayloadSize {
		return PlayerPointsPacket{}, ErrInvalidPayload
	}
	var packet PlayerPointsPacket
	offset := 0
	for i := range packet.Points {
		packet.Points[i] = int32(binary.LittleEndian.Uint32(f.Payload[offset:]))
		offset += 4
	}
	return packet, nil
}

func encodeSimplePlayerPayload(player loginproto.SimplePlayer) ([]byte, error) {
	payload := make([]byte, simplePlayerPayloadSize)
	offset := 0
	binary.LittleEndian.PutUint32(payload[offset:], player.ID)
	offset += 4
	if err := putFixedString(payload[offset:offset+loginproto.CharacterNameFieldSize], player.Name); err != nil {
		return nil, err
	}
	offset += loginproto.CharacterNameFieldSize
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

func decodeSimplePlayerPayload(payload []byte) (loginproto.SimplePlayer, error) {
	if len(payload) != simplePlayerPayloadSize {
		return loginproto.SimplePlayer{}, ErrInvalidPayload
	}

	player := loginproto.SimplePlayer{}
	offset := 0
	player.ID = binary.LittleEndian.Uint32(payload[offset:])
	offset += 4
	player.Name = parseFixedString(payload[offset : offset+loginproto.CharacterNameFieldSize])
	offset += loginproto.CharacterNameFieldSize
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
