package item

import (
	"encoding/binary"
	"errors"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

const (
	HeaderClientUse  uint16 = 0x0502
	HeaderClientMove uint16 = 0x0504
	HeaderDel        uint16 = 0x0510
	HeaderSet        uint16 = 0x0511
	HeaderUpdate     uint16 = 0x0514

	WindowReserved            uint8  = 0
	WindowInventory           uint8  = 1
	WindowEquipment           uint8  = 2
	WindowSafebox             uint8  = 3
	WindowMall                uint8  = 4
	WindowDragonSoulInventory uint8  = 5
	WindowBeltInventory       uint8  = 6
	WindowGround              uint8  = 7
	InventoryMaxCell          uint16 = 90
	WearMaxCell               uint16 = 32
	ItemSocketCount                  = 3
	ItemAttributeCount               = 7

	positionSize          = 3
	attributeSize         = 3
	clientUsePayloadSize  = positionSize
	clientMovePayloadSize = positionSize + positionSize + 1
	delPayloadSize        = positionSize
	setPayloadSize        = positionSize + 4 + 1 + 4 + 4 + 1 + (ItemSocketCount * 4) + (ItemAttributeCount * attributeSize)
	updatePayloadSize     = positionSize + 1 + (ItemSocketCount * 4) + (ItemAttributeCount * attributeSize)
)

var (
	ErrUnexpectedHeader       = errors.New("unexpected item packet header")
	ErrInvalidPayload         = errors.New("invalid item packet payload")
	ErrInventoryCellRange     = errors.New("inventory cell is out of range")
	ErrEquipmentWearCellRange = errors.New("equipment wear cell is out of range")
)

type Position struct {
	WindowType uint8
	Cell       uint16
}

type Attribute struct {
	Type  uint8
	Value int16
}

type SetPacket struct {
	Position   Position
	Vnum       uint32
	Count      uint8
	Flags      uint32
	AntiFlags  uint32
	Highlight  uint8
	Sockets    [ItemSocketCount]int32
	Attributes [ItemAttributeCount]Attribute
}

type DelPacket struct {
	Position Position
}

type UpdatePacket struct {
	Position   Position
	Count      uint8
	Sockets    [ItemSocketCount]int32
	Attributes [ItemAttributeCount]Attribute
}

type ClientUsePacket struct {
	Position Position
}

type ClientMovePacket struct {
	Source      Position
	Destination Position
	Count       uint8
}

func InventoryPosition(cell uint16) Position {
	return Position{WindowType: WindowInventory, Cell: cell}
}

func CarriedInventoryPosition(cell uint16) (Position, error) {
	if cell >= InventoryMaxCell {
		return Position{}, ErrInventoryCellRange
	}
	return InventoryPosition(cell), nil
}

func EquipmentPosition(wearCell uint16) (Position, error) {
	if wearCell >= WearMaxCell {
		return Position{}, ErrEquipmentWearCellRange
	}
	return InventoryPosition(InventoryMaxCell + wearCell), nil
}

func EncodeClientUse(packet ClientUsePacket) []byte {
	payload := make([]byte, clientUsePayloadSize)
	encodePosition(payload, packet.Position)
	return frame.Encode(HeaderClientUse, payload)
}

func DecodeClientUse(f frame.Frame) (ClientUsePacket, error) {
	if f.Header != HeaderClientUse {
		return ClientUsePacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientUsePayloadSize {
		return ClientUsePacket{}, ErrInvalidPayload
	}
	return ClientUsePacket{Position: decodePosition(f.Payload)}, nil
}

func EncodeClientMove(packet ClientMovePacket) []byte {
	payload := make([]byte, clientMovePayloadSize)
	encodePosition(payload[:positionSize], packet.Source)
	encodePosition(payload[positionSize:positionSize+positionSize], packet.Destination)
	payload[positionSize+positionSize] = packet.Count
	return frame.Encode(HeaderClientMove, payload)
}

func DecodeClientMove(f frame.Frame) (ClientMovePacket, error) {
	if f.Header != HeaderClientMove {
		return ClientMovePacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientMovePayloadSize {
		return ClientMovePacket{}, ErrInvalidPayload
	}
	return ClientMovePacket{
		Source:      decodePosition(f.Payload[:positionSize]),
		Destination: decodePosition(f.Payload[positionSize : positionSize+positionSize]),
		Count:       f.Payload[positionSize+positionSize],
	}, nil
}

func EncodeSet(packet SetPacket) []byte {
	payload := make([]byte, setPayloadSize)
	encodePosition(payload[:positionSize], packet.Position)
	offset := positionSize
	binary.LittleEndian.PutUint32(payload[offset:], packet.Vnum)
	offset += 4
	payload[offset] = packet.Count
	offset++
	binary.LittleEndian.PutUint32(payload[offset:], packet.Flags)
	offset += 4
	binary.LittleEndian.PutUint32(payload[offset:], packet.AntiFlags)
	offset += 4
	payload[offset] = packet.Highlight
	offset++
	for _, socket := range packet.Sockets {
		binary.LittleEndian.PutUint32(payload[offset:], uint32(socket))
		offset += 4
	}
	for _, attribute := range packet.Attributes {
		payload[offset] = attribute.Type
		offset++
		binary.LittleEndian.PutUint16(payload[offset:], uint16(attribute.Value))
		offset += 2
	}
	return frame.Encode(HeaderSet, payload)
}

func DecodeSet(f frame.Frame) (SetPacket, error) {
	if f.Header != HeaderSet {
		return SetPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != setPayloadSize {
		return SetPacket{}, ErrInvalidPayload
	}
	packet := SetPacket{Position: decodePosition(f.Payload[:positionSize])}
	offset := positionSize
	packet.Vnum = binary.LittleEndian.Uint32(f.Payload[offset:])
	offset += 4
	packet.Count = f.Payload[offset]
	offset++
	packet.Flags = binary.LittleEndian.Uint32(f.Payload[offset:])
	offset += 4
	packet.AntiFlags = binary.LittleEndian.Uint32(f.Payload[offset:])
	offset += 4
	packet.Highlight = f.Payload[offset]
	offset++
	for i := range packet.Sockets {
		packet.Sockets[i] = int32(binary.LittleEndian.Uint32(f.Payload[offset:]))
		offset += 4
	}
	for i := range packet.Attributes {
		packet.Attributes[i].Type = f.Payload[offset]
		offset++
		packet.Attributes[i].Value = int16(binary.LittleEndian.Uint16(f.Payload[offset:]))
		offset += 2
	}
	return packet, nil
}

func EncodeDel(packet DelPacket) []byte {
	payload := make([]byte, delPayloadSize)
	encodePosition(payload, packet.Position)
	return frame.Encode(HeaderDel, payload)
}

func DecodeDel(f frame.Frame) (DelPacket, error) {
	if f.Header != HeaderDel {
		return DelPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != delPayloadSize {
		return DelPacket{}, ErrInvalidPayload
	}
	return DelPacket{Position: decodePosition(f.Payload)}, nil
}

func EncodeUpdate(packet UpdatePacket) []byte {
	payload := make([]byte, updatePayloadSize)
	encodePosition(payload[:positionSize], packet.Position)
	offset := positionSize
	payload[offset] = packet.Count
	offset++
	for _, socket := range packet.Sockets {
		binary.LittleEndian.PutUint32(payload[offset:], uint32(socket))
		offset += 4
	}
	for _, attribute := range packet.Attributes {
		payload[offset] = attribute.Type
		offset++
		binary.LittleEndian.PutUint16(payload[offset:], uint16(attribute.Value))
		offset += 2
	}
	return frame.Encode(HeaderUpdate, payload)
}

func DecodeUpdate(f frame.Frame) (UpdatePacket, error) {
	if f.Header != HeaderUpdate {
		return UpdatePacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != updatePayloadSize {
		return UpdatePacket{}, ErrInvalidPayload
	}
	packet := UpdatePacket{Position: decodePosition(f.Payload[:positionSize])}
	offset := positionSize
	packet.Count = f.Payload[offset]
	offset++
	for i := range packet.Sockets {
		packet.Sockets[i] = int32(binary.LittleEndian.Uint32(f.Payload[offset:]))
		offset += 4
	}
	for i := range packet.Attributes {
		packet.Attributes[i].Type = f.Payload[offset]
		offset++
		packet.Attributes[i].Value = int16(binary.LittleEndian.Uint16(f.Payload[offset:]))
		offset += 2
	}
	return packet, nil
}

func encodePosition(dst []byte, position Position) {
	dst[0] = position.WindowType
	binary.LittleEndian.PutUint16(dst[1:], position.Cell)
}

func decodePosition(src []byte) Position {
	return Position{
		WindowType: src[0],
		Cell:       binary.LittleEndian.Uint16(src[1:]),
	}
}
