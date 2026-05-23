package item

import (
	"encoding/binary"
	"errors"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

const (
	HeaderClientUse    uint16 = 0x0502
	HeaderClientDrop   uint16 = 0x0502
	HeaderClientDrop2  uint16 = 0x0503
	HeaderClientMove   uint16 = 0x0504
	HeaderClientPickup uint16 = 0x0505
	HeaderDel          uint16 = 0x0510
	HeaderSet          uint16 = 0x0511
	HeaderUpdate       uint16 = 0x0514
	HeaderGroundAdd    uint16 = 0x0515
	HeaderGroundDel    uint16 = 0x0516
	HeaderOwnership    uint16 = 0x0517
	HeaderGet          uint16 = 0x0518

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
	CharacterNameMaxLength           = 24

	positionSize            = 3
	attributeSize           = 3
	clientUsePayloadSize    = positionSize
	clientDropPayloadSize   = positionSize + 4
	clientDrop2PayloadSize  = positionSize + 4 + 1
	clientMovePayloadSize   = positionSize + positionSize + 1
	clientPickupPayloadSize = 4
	delPayloadSize          = positionSize
	setPayloadSize          = positionSize + 4 + 1 + 4 + 4 + 1 + (ItemSocketCount * 4) + (ItemAttributeCount * attributeSize)
	updatePayloadSize       = positionSize + 1 + (ItemSocketCount * 4) + (ItemAttributeCount * attributeSize)
	groundAddPayloadSize    = 4 + 4 + 4 + 4 + 4
	groundDelPayloadSize    = 4
	ownershipPayloadSize    = 4 + (CharacterNameMaxLength + 1)
	getPayloadSize          = 4 + 1 + 1 + (CharacterNameMaxLength + 1)
)

const (
	GetArgNormal                 uint8 = 0
	GetArgFromPartyMember        uint8 = 1
	GetArgDeliveredToPartyMember uint8 = 2
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

type ClientDropPacket struct {
	Position Position
	Elk      uint32
}

type ClientDrop2Packet struct {
	Position Position
	Gold     uint32
	Count    uint8
}

type ClientMovePacket struct {
	Source      Position
	Destination Position
	Count       uint8
}

type ClientPickupPacket struct {
	VID uint32
}

type GroundAddPacket struct {
	VID  uint32
	Vnum uint32
	X    int32
	Y    int32
	Z    int32
}

type GroundDelPacket struct {
	VID uint32
}

type OwnershipPacket struct {
	VID       uint32
	OwnerName string
}

type GetPacket struct {
	Vnum     uint32
	Count    uint8
	Arg      uint8
	FromName string
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

func EncodeClientDrop(packet ClientDropPacket) []byte {
	payload := make([]byte, clientDropPayloadSize)
	encodePosition(payload[:positionSize], packet.Position)
	binary.LittleEndian.PutUint32(payload[positionSize:], packet.Elk)
	return frame.Encode(HeaderClientDrop, payload)
}

func DecodeClientDrop(f frame.Frame) (ClientDropPacket, error) {
	if f.Header != HeaderClientDrop {
		return ClientDropPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientDropPayloadSize {
		return ClientDropPacket{}, ErrInvalidPayload
	}
	return ClientDropPacket{
		Position: decodePosition(f.Payload[:positionSize]),
		Elk:      binary.LittleEndian.Uint32(f.Payload[positionSize:]),
	}, nil
}

func EncodeClientDrop2(packet ClientDrop2Packet) []byte {
	payload := make([]byte, clientDrop2PayloadSize)
	encodePosition(payload[:positionSize], packet.Position)
	binary.LittleEndian.PutUint32(payload[positionSize:], packet.Gold)
	payload[positionSize+4] = packet.Count
	return frame.Encode(HeaderClientDrop2, payload)
}

func DecodeClientDrop2(f frame.Frame) (ClientDrop2Packet, error) {
	if f.Header != HeaderClientDrop2 {
		return ClientDrop2Packet{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientDrop2PayloadSize {
		return ClientDrop2Packet{}, ErrInvalidPayload
	}
	return ClientDrop2Packet{
		Position: decodePosition(f.Payload[:positionSize]),
		Gold:     binary.LittleEndian.Uint32(f.Payload[positionSize:]),
		Count:    f.Payload[positionSize+4],
	}, nil
}

func EncodeClientPickup(packet ClientPickupPacket) []byte {
	payload := make([]byte, clientPickupPayloadSize)
	binary.LittleEndian.PutUint32(payload, packet.VID)
	return frame.Encode(HeaderClientPickup, payload)
}

func DecodeClientPickup(f frame.Frame) (ClientPickupPacket, error) {
	if f.Header != HeaderClientPickup {
		return ClientPickupPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientPickupPayloadSize {
		return ClientPickupPacket{}, ErrInvalidPayload
	}
	return ClientPickupPacket{VID: binary.LittleEndian.Uint32(f.Payload)}, nil
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

func EncodeGroundAdd(packet GroundAddPacket) []byte {
	payload := make([]byte, groundAddPayloadSize)
	binary.LittleEndian.PutUint32(payload[0:], packet.VID)
	binary.LittleEndian.PutUint32(payload[4:], packet.Vnum)
	binary.LittleEndian.PutUint32(payload[8:], uint32(packet.X))
	binary.LittleEndian.PutUint32(payload[12:], uint32(packet.Y))
	binary.LittleEndian.PutUint32(payload[16:], uint32(packet.Z))
	return frame.Encode(HeaderGroundAdd, payload)
}

func DecodeGroundAdd(f frame.Frame) (GroundAddPacket, error) {
	if f.Header != HeaderGroundAdd {
		return GroundAddPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != groundAddPayloadSize {
		return GroundAddPacket{}, ErrInvalidPayload
	}
	return GroundAddPacket{
		VID:  binary.LittleEndian.Uint32(f.Payload[0:]),
		Vnum: binary.LittleEndian.Uint32(f.Payload[4:]),
		X:    int32(binary.LittleEndian.Uint32(f.Payload[8:])),
		Y:    int32(binary.LittleEndian.Uint32(f.Payload[12:])),
		Z:    int32(binary.LittleEndian.Uint32(f.Payload[16:])),
	}, nil
}

func EncodeGroundDel(packet GroundDelPacket) []byte {
	payload := make([]byte, groundDelPayloadSize)
	binary.LittleEndian.PutUint32(payload, packet.VID)
	return frame.Encode(HeaderGroundDel, payload)
}

func DecodeGroundDel(f frame.Frame) (GroundDelPacket, error) {
	if f.Header != HeaderGroundDel {
		return GroundDelPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != groundDelPayloadSize {
		return GroundDelPacket{}, ErrInvalidPayload
	}
	return GroundDelPacket{VID: binary.LittleEndian.Uint32(f.Payload)}, nil
}

func EncodeOwnership(packet OwnershipPacket) []byte {
	payload := make([]byte, ownershipPayloadSize)
	binary.LittleEndian.PutUint32(payload[0:], packet.VID)
	copyFixedString(payload[4:], packet.OwnerName)
	return frame.Encode(HeaderOwnership, payload)
}

func DecodeOwnership(f frame.Frame) (OwnershipPacket, error) {
	if f.Header != HeaderOwnership {
		return OwnershipPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != ownershipPayloadSize {
		return OwnershipPacket{}, ErrInvalidPayload
	}
	return OwnershipPacket{
		VID:       binary.LittleEndian.Uint32(f.Payload[0:]),
		OwnerName: decodeFixedString(f.Payload[4:]),
	}, nil
}

func EncodeGet(packet GetPacket) []byte {
	payload := make([]byte, getPayloadSize)
	binary.LittleEndian.PutUint32(payload[0:], packet.Vnum)
	payload[4] = packet.Count
	payload[5] = packet.Arg
	copyFixedString(payload[6:], packet.FromName)
	return frame.Encode(HeaderGet, payload)
}

func DecodeGet(f frame.Frame) (GetPacket, error) {
	if f.Header != HeaderGet {
		return GetPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != getPayloadSize {
		return GetPacket{}, ErrInvalidPayload
	}
	return GetPacket{
		Vnum:     binary.LittleEndian.Uint32(f.Payload[0:]),
		Count:    f.Payload[4],
		Arg:      f.Payload[5],
		FromName: decodeFixedString(f.Payload[6:]),
	}, nil
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

func copyFixedString(dst []byte, value string) {
	copy(dst, value)
}

func decodeFixedString(src []byte) string {
	end := 0
	for end < len(src) && src[end] != 0 {
		end++
	}
	return string(src[:end])
}
