package move

import (
	"encoding/binary"
	"errors"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

const (
	HeaderMove            uint16 = 0x0301
	HeaderMoveAck         uint16 = 0x0302
	HeaderSyncPosition    uint16 = 0x0303
	HeaderSyncPositionAck uint16 = 0x0304

	movePayloadSize         = 15
	moveAckPayloadSize      = 23
	syncPositionElementSize = 12
)

var (
	ErrUnexpectedHeader = errors.New("unexpected move packet header")
	ErrInvalidPayload   = errors.New("invalid move packet payload")
)

type MovePacket struct {
	Func uint8
	Arg  uint8
	Rot  uint8
	X    int32
	Y    int32
	Time uint32
}

type MoveAckPacket struct {
	Func     uint8
	Arg      uint8
	Rot      uint8
	VID      uint32
	X        int32
	Y        int32
	Time     uint32
	Duration uint32
}

type SyncPositionElement struct {
	VID uint32
	X   int32
	Y   int32
}

type SyncPositionPacket struct {
	Elements []SyncPositionElement
}

type SyncPositionAckPacket struct {
	Elements []SyncPositionElement
}

func EncodeMove(packet MovePacket) []byte {
	payload := make([]byte, movePayloadSize)
	payload[0] = packet.Func
	payload[1] = packet.Arg
	payload[2] = packet.Rot
	binary.LittleEndian.PutUint32(payload[3:], uint32(packet.X))
	binary.LittleEndian.PutUint32(payload[7:], uint32(packet.Y))
	binary.LittleEndian.PutUint32(payload[11:], packet.Time)
	return frame.Encode(HeaderMove, payload)
}

func DecodeMove(f frame.Frame) (MovePacket, error) {
	if f.Header != HeaderMove {
		return MovePacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != movePayloadSize {
		return MovePacket{}, ErrInvalidPayload
	}
	return MovePacket{
		Func: f.Payload[0],
		Arg:  f.Payload[1],
		Rot:  f.Payload[2],
		X:    int32(binary.LittleEndian.Uint32(f.Payload[3:])),
		Y:    int32(binary.LittleEndian.Uint32(f.Payload[7:])),
		Time: binary.LittleEndian.Uint32(f.Payload[11:]),
	}, nil
}

func EncodeMoveAck(packet MoveAckPacket) []byte {
	payload := make([]byte, moveAckPayloadSize)
	payload[0] = packet.Func
	payload[1] = packet.Arg
	payload[2] = packet.Rot
	binary.LittleEndian.PutUint32(payload[3:], packet.VID)
	binary.LittleEndian.PutUint32(payload[7:], uint32(packet.X))
	binary.LittleEndian.PutUint32(payload[11:], uint32(packet.Y))
	binary.LittleEndian.PutUint32(payload[15:], packet.Time)
	binary.LittleEndian.PutUint32(payload[19:], packet.Duration)
	return frame.Encode(HeaderMoveAck, payload)
}

func DecodeMoveAck(f frame.Frame) (MoveAckPacket, error) {
	if f.Header != HeaderMoveAck {
		return MoveAckPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != moveAckPayloadSize {
		return MoveAckPacket{}, ErrInvalidPayload
	}
	return MoveAckPacket{
		Func:     f.Payload[0],
		Arg:      f.Payload[1],
		Rot:      f.Payload[2],
		VID:      binary.LittleEndian.Uint32(f.Payload[3:]),
		X:        int32(binary.LittleEndian.Uint32(f.Payload[7:])),
		Y:        int32(binary.LittleEndian.Uint32(f.Payload[11:])),
		Time:     binary.LittleEndian.Uint32(f.Payload[15:]),
		Duration: binary.LittleEndian.Uint32(f.Payload[19:]),
	}, nil
}

func EncodeSyncPosition(packet SyncPositionPacket) []byte {
	return frame.Encode(HeaderSyncPosition, encodeSyncPositionElements(packet.Elements))
}

func DecodeSyncPosition(f frame.Frame) (SyncPositionPacket, error) {
	if f.Header != HeaderSyncPosition {
		return SyncPositionPacket{}, ErrUnexpectedHeader
	}
	elements, err := decodeSyncPositionElements(f.Payload)
	if err != nil {
		return SyncPositionPacket{}, err
	}
	return SyncPositionPacket{Elements: elements}, nil
}

func EncodeSyncPositionAck(packet SyncPositionAckPacket) []byte {
	return frame.Encode(HeaderSyncPositionAck, encodeSyncPositionElements(packet.Elements))
}

func DecodeSyncPositionAck(f frame.Frame) (SyncPositionAckPacket, error) {
	if f.Header != HeaderSyncPositionAck {
		return SyncPositionAckPacket{}, ErrUnexpectedHeader
	}
	elements, err := decodeSyncPositionElements(f.Payload)
	if err != nil {
		return SyncPositionAckPacket{}, err
	}
	return SyncPositionAckPacket{Elements: elements}, nil
}

func encodeSyncPositionElements(elements []SyncPositionElement) []byte {
	payload := make([]byte, len(elements)*syncPositionElementSize)
	offset := 0
	for _, element := range elements {
		binary.LittleEndian.PutUint32(payload[offset:], element.VID)
		offset += 4
		binary.LittleEndian.PutUint32(payload[offset:], uint32(element.X))
		offset += 4
		binary.LittleEndian.PutUint32(payload[offset:], uint32(element.Y))
		offset += 4
	}
	return payload
}

func decodeSyncPositionElements(payload []byte) ([]SyncPositionElement, error) {
	if len(payload)%syncPositionElementSize != 0 {
		return nil, ErrInvalidPayload
	}
	elements := make([]SyncPositionElement, 0, len(payload)/syncPositionElementSize)
	for offset := 0; offset < len(payload); offset += syncPositionElementSize {
		elements = append(elements, SyncPositionElement{
			VID: binary.LittleEndian.Uint32(payload[offset:]),
			X:   int32(binary.LittleEndian.Uint32(payload[offset+4:])),
			Y:   int32(binary.LittleEndian.Uint32(payload[offset+8:])),
		})
	}
	return elements, nil
}
