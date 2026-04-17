package world

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

const (
	HeaderCharacterSelect uint16 = 0x0203
	HeaderEnterGame       uint16 = 0x0204
	HeaderMainCharacter   uint16 = 0x0210
	HeaderPlayerPoints    uint16 = 0x0214

	CharacterNameFieldSize = 65
	BGMNameFieldSize       = 25
	PointCount             = 255

	characterSelectPayloadSize = 1
	mainCharacterPayloadSize   = 114
	playerPointsPayloadSize    = PointCount * 4
)

var (
	ErrUnexpectedHeader = errors.New("unexpected world packet header")
	ErrInvalidPayload   = errors.New("invalid world packet payload")
	ErrStringTooLong    = errors.New("string does not fit fixed-width wire field")
)

type CharacterSelectPacket struct {
	Index uint8
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
