package item

import (
	"encoding/binary"
	"errors"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

const (
	HeaderClientUse             uint16 = 0x0501
	HeaderClientDrop            uint16 = 0x0502
	HeaderClientDrop2           uint16 = 0x0503
	HeaderClientMove            uint16 = 0x0504
	HeaderClientPickup          uint16 = 0x0505
	HeaderClientUseToItem       uint16 = 0x0506
	HeaderClientGive            uint16 = 0x0507
	HeaderClientExchange        uint16 = 0x0508
	HeaderClientRefine          uint16 = 0x050C
	HeaderClientSafeboxCheckin  uint16 = 0x0820
	HeaderClientSafeboxCheckout uint16 = 0x0821
	HeaderClientSafeboxItemMove uint16 = 0x0822
	HeaderClientMallCheckout    uint16 = 0x0840
	HeaderDel                   uint16 = 0x0510
	HeaderSet                   uint16 = 0x0511
	HeaderUse                   uint16 = 0x0512
	HeaderUpdate                uint16 = 0x0514
	HeaderGroundAdd             uint16 = 0x0515
	HeaderGroundDel             uint16 = 0x0516
	HeaderOwnership             uint16 = 0x0517
	HeaderGet                   uint16 = 0x0518
	HeaderServerExchange        uint16 = 0x051C
	HeaderRefineInformation     uint16 = 0x051D
	HeaderRefineInformationNew  uint16 = 0x051E
	HeaderSafeboxSet            uint16 = 0x0830
	HeaderSafeboxDel            uint16 = 0x0831
	HeaderSafeboxWrongPassword  uint16 = 0x0832
	HeaderSafeboxSize           uint16 = 0x0833
	HeaderSafeboxMoneyChange    uint16 = 0x0834
	HeaderMallOpen              uint16 = 0x0841
	HeaderMallSet               uint16 = 0x0842
	HeaderMallDel               uint16 = 0x0843

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
	RefineMaterialMaxNum             = 5

	positionSize                     = 3
	attributeSize                    = 3
	clientUsePayloadSize             = positionSize
	clientDropPayloadSize            = positionSize + 4
	clientDrop2PayloadSize           = positionSize + 4 + 1
	clientMovePayloadSize            = positionSize + positionSize + 1
	clientPickupPayloadSize          = 4
	clientUseToItemPayloadSize       = positionSize + positionSize
	clientGivePayloadSize            = 4 + positionSize + 1
	clientExchangePayloadSize        = 1 + 4 + 1 + positionSize
	clientRefinePayloadSize          = 2
	clientSafeboxTransferPayloadSize = 1 + positionSize
	clientSafeboxItemMovePayloadSize = positionSize + positionSize + 1
	clientMallCheckoutPayloadSize    = 1 + positionSize
	serverExchangePayloadSize        = 1 + 1 + 4 + positionSize + 4 + (ItemSocketCount * 4) + (ItemAttributeCount * attributeSize)
	refineMaterialPayloadSize        = 4 + 4
	refineTablePayloadSize           = 4 + 4 + 1 + 4 + 4 + (RefineMaterialMaxNum * refineMaterialPayloadSize)
	refineInformationPayloadSize     = 1 + 1 + refineTablePayloadSize
	delPayloadSize                   = positionSize
	setPayloadSize                   = positionSize + 4 + 1 + 4 + 4 + 1 + (ItemSocketCount * 4) + (ItemAttributeCount * attributeSize)
	usePayloadSize                   = positionSize + 4 + 4 + 4
	updatePayloadSize                = positionSize + 1 + (ItemSocketCount * 4) + (ItemAttributeCount * attributeSize)
	groundAddPayloadSize             = 4 + 4 + 4 + 4 + 4
	groundDelPayloadSize             = 4
	ownershipPayloadSize             = 4 + (CharacterNameMaxLength + 1)
	getPayloadSize                   = 4 + 1 + 1 + (CharacterNameMaxLength + 1)
	safeboxSizePayloadSize           = 1
	safeboxMoneyChangePayloadSize    = 4
	mallOpenPayloadSize              = 1
)

const (
	GetArgNormal                 uint8 = 0
	GetArgFromPartyMember        uint8 = 1
	GetArgDeliveredToPartyMember uint8 = 2
)

const ExchangeItemMaxNum uint8 = 12

const (
	ExchangeSubheaderStart uint8 = iota
	ExchangeSubheaderItemAdd
	ExchangeSubheaderItemDel
	ExchangeSubheaderGoldAdd
	ExchangeSubheaderAccept
	ExchangeSubheaderCancel
)

const (
	ExchangeServerSubheaderStart uint8 = iota
	ExchangeServerSubheaderItemAdd
	ExchangeServerSubheaderItemDel
	ExchangeServerSubheaderGoldAdd
	ExchangeServerSubheaderAccept
	ExchangeServerSubheaderEnd
	ExchangeServerSubheaderAlready
	ExchangeServerSubheaderLessGold
)

const (
	ItemFlagRefineable uint32 = 1 << iota
	ItemFlagSave
	ItemFlagStackable
	ItemFlagCountPerGold
	ItemFlagSlowQuery
	ItemFlagRare
	ItemFlagUnique
	ItemFlagMakeCount
	ItemFlagIrremovable
	ItemFlagConfirmWhenUse
	ItemFlagQuestUse
	ItemFlagQuestUseMultiple
	ItemFlagUnused03
	ItemFlagLog
	ItemFlagApplicable
)

const (
	AntiFlagFemale uint32 = 1 << iota
	AntiFlagMale
	AntiFlagWarrior
	AntiFlagAssassin
	AntiFlagSura
	AntiFlagShaman
	AntiFlagGet
	AntiFlagDrop
	AntiFlagSell
	AntiFlagEmpireA
	AntiFlagEmpireB
	AntiFlagEmpireC
	AntiFlagSave
	AntiFlagGive
	AntiFlagPKDrop
	AntiFlagStack
	AntiFlagMyShop
	AntiFlagSafebox
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

type UsePacket struct {
	Position     Position
	CharacterVID uint32
	VictimVID    uint32
	Vnum         uint32
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

type ClientUseToItemPacket struct {
	Source Position
	Target Position
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

type ClientGivePacket struct {
	TargetVID uint32
	Position  Position
	Count     uint8
}

type ClientExchangePacket struct {
	Subheader uint8
	Arg1      uint32
	Arg2      uint8
	Position  Position
}

type ClientRefinePacket struct {
	Position uint8
	Type     uint8
}

type RefineMaterial struct {
	Vnum  uint32
	Count int32
}

type RefineTable struct {
	SourceVnum    uint32
	ResultVnum    uint32
	MaterialCount uint8
	Cost          int32
	Probability   int32
	Materials     [RefineMaterialMaxNum]RefineMaterial
}

type RefineInformationPacket struct {
	Type     uint8
	Position uint8
	Table    RefineTable
}

type ClientSafeboxCheckinPacket struct {
	SafeSlot uint8
	Position Position
}

type ClientSafeboxCheckoutPacket struct {
	SafeSlot uint8
	Position Position
}

type ClientSafeboxItemMovePacket struct {
	Source      Position
	Destination Position
	Count       uint8
}

type ClientMallCheckoutPacket struct {
	MallSlot uint8
	Position Position
}

type SafeboxWrongPasswordPacket struct{}

type SafeboxSizePacket struct {
	Size uint8
}

type SafeboxMoneyChangePacket struct {
	Money int32
}

type MallOpenPacket struct {
	Size uint8
}

type ServerExchangePacket struct {
	Subheader  uint8
	IsMe       uint8
	Arg1       uint32
	Position   Position
	Arg3       uint32
	Sockets    [ItemSocketCount]int32
	Attributes [ItemAttributeCount]Attribute
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

func EncodeClientUseToItem(packet ClientUseToItemPacket) []byte {
	payload := make([]byte, clientUseToItemPayloadSize)
	encodePosition(payload[:positionSize], packet.Source)
	encodePosition(payload[positionSize:positionSize+positionSize], packet.Target)
	return frame.Encode(HeaderClientUseToItem, payload)
}

func DecodeClientUseToItem(f frame.Frame) (ClientUseToItemPacket, error) {
	if f.Header != HeaderClientUseToItem {
		return ClientUseToItemPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientUseToItemPayloadSize {
		return ClientUseToItemPacket{}, ErrInvalidPayload
	}
	return ClientUseToItemPacket{
		Source: decodePosition(f.Payload[:positionSize]),
		Target: decodePosition(f.Payload[positionSize : positionSize+positionSize]),
	}, nil
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

func EncodeClientGive(packet ClientGivePacket) []byte {
	payload := make([]byte, clientGivePayloadSize)
	binary.LittleEndian.PutUint32(payload[0:], packet.TargetVID)
	encodePosition(payload[4:4+positionSize], packet.Position)
	payload[4+positionSize] = packet.Count
	return frame.Encode(HeaderClientGive, payload)
}

func DecodeClientGive(f frame.Frame) (ClientGivePacket, error) {
	if f.Header != HeaderClientGive {
		return ClientGivePacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientGivePayloadSize {
		return ClientGivePacket{}, ErrInvalidPayload
	}
	return ClientGivePacket{
		TargetVID: binary.LittleEndian.Uint32(f.Payload[0:]),
		Position:  decodePosition(f.Payload[4 : 4+positionSize]),
		Count:     f.Payload[4+positionSize],
	}, nil
}

func EncodeClientExchange(packet ClientExchangePacket) []byte {
	payload := make([]byte, clientExchangePayloadSize)
	payload[0] = packet.Subheader
	binary.LittleEndian.PutUint32(payload[1:], packet.Arg1)
	payload[5] = packet.Arg2
	encodePosition(payload[6:], packet.Position)
	return frame.Encode(HeaderClientExchange, payload)
}

func DecodeClientExchange(f frame.Frame) (ClientExchangePacket, error) {
	if f.Header != HeaderClientExchange {
		return ClientExchangePacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientExchangePayloadSize {
		return ClientExchangePacket{}, ErrInvalidPayload
	}
	return ClientExchangePacket{
		Subheader: f.Payload[0],
		Arg1:      binary.LittleEndian.Uint32(f.Payload[1:]),
		Arg2:      f.Payload[5],
		Position:  decodePosition(f.Payload[6:]),
	}, nil
}

func EncodeClientRefine(packet ClientRefinePacket) []byte {
	payload := []byte{packet.Position, packet.Type}
	return frame.Encode(HeaderClientRefine, payload)
}

func DecodeClientRefine(f frame.Frame) (ClientRefinePacket, error) {
	if f.Header != HeaderClientRefine {
		return ClientRefinePacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientRefinePayloadSize {
		return ClientRefinePacket{}, ErrInvalidPayload
	}
	return ClientRefinePacket{Position: f.Payload[0], Type: f.Payload[1]}, nil
}

func EncodeRefineInformation(packet RefineInformationPacket) ([]byte, error) {
	return encodeRefineInformation(HeaderRefineInformation, packet)
}

func DecodeRefineInformation(f frame.Frame) (RefineInformationPacket, error) {
	return decodeRefineInformation(f, HeaderRefineInformation)
}

func EncodeRefineInformationNew(packet RefineInformationPacket) ([]byte, error) {
	return encodeRefineInformation(HeaderRefineInformationNew, packet)
}

func DecodeRefineInformationNew(f frame.Frame) (RefineInformationPacket, error) {
	return decodeRefineInformation(f, HeaderRefineInformationNew)
}

func encodeRefineInformation(header uint16, packet RefineInformationPacket) ([]byte, error) {
	if packet.Table.MaterialCount > RefineMaterialMaxNum {
		return nil, ErrInvalidPayload
	}
	payload := make([]byte, refineInformationPayloadSize)
	payload[0] = packet.Type
	payload[1] = packet.Position
	encodeRefineTable(payload[2:], packet.Table)
	return frame.Encode(header, payload), nil
}

func decodeRefineInformation(f frame.Frame, header uint16) (RefineInformationPacket, error) {
	if f.Header != header {
		return RefineInformationPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != refineInformationPayloadSize {
		return RefineInformationPacket{}, ErrInvalidPayload
	}
	packet := RefineInformationPacket{Type: f.Payload[0], Position: f.Payload[1], Table: decodeRefineTable(f.Payload[2:])}
	if packet.Table.MaterialCount > RefineMaterialMaxNum {
		return RefineInformationPacket{}, ErrInvalidPayload
	}
	return packet, nil
}

func encodeRefineTable(payload []byte, table RefineTable) {
	binary.LittleEndian.PutUint32(payload[0:4], table.SourceVnum)
	binary.LittleEndian.PutUint32(payload[4:8], table.ResultVnum)
	payload[8] = table.MaterialCount
	binary.LittleEndian.PutUint32(payload[9:13], uint32(table.Cost))
	binary.LittleEndian.PutUint32(payload[13:17], uint32(table.Probability))
	offset := 17
	for _, material := range table.Materials {
		binary.LittleEndian.PutUint32(payload[offset:offset+4], material.Vnum)
		binary.LittleEndian.PutUint32(payload[offset+4:offset+8], uint32(material.Count))
		offset += refineMaterialPayloadSize
	}
}

func decodeRefineTable(payload []byte) RefineTable {
	table := RefineTable{
		SourceVnum:    binary.LittleEndian.Uint32(payload[0:4]),
		ResultVnum:    binary.LittleEndian.Uint32(payload[4:8]),
		MaterialCount: payload[8],
		Cost:          int32(binary.LittleEndian.Uint32(payload[9:13])),
		Probability:   int32(binary.LittleEndian.Uint32(payload[13:17])),
	}
	offset := 17
	for i := range table.Materials {
		table.Materials[i] = RefineMaterial{Vnum: binary.LittleEndian.Uint32(payload[offset : offset+4]), Count: int32(binary.LittleEndian.Uint32(payload[offset+4 : offset+8]))}
		offset += refineMaterialPayloadSize
	}
	return table
}

func EncodeClientSafeboxCheckin(packet ClientSafeboxCheckinPacket) []byte {
	payload := make([]byte, clientSafeboxTransferPayloadSize)
	payload[0] = packet.SafeSlot
	encodePosition(payload[1:], packet.Position)
	return frame.Encode(HeaderClientSafeboxCheckin, payload)
}

func DecodeClientSafeboxCheckin(f frame.Frame) (ClientSafeboxCheckinPacket, error) {
	if f.Header != HeaderClientSafeboxCheckin {
		return ClientSafeboxCheckinPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientSafeboxTransferPayloadSize {
		return ClientSafeboxCheckinPacket{}, ErrInvalidPayload
	}
	return ClientSafeboxCheckinPacket{SafeSlot: f.Payload[0], Position: decodePosition(f.Payload[1:])}, nil
}

func EncodeClientSafeboxCheckout(packet ClientSafeboxCheckoutPacket) []byte {
	payload := make([]byte, clientSafeboxTransferPayloadSize)
	payload[0] = packet.SafeSlot
	encodePosition(payload[1:], packet.Position)
	return frame.Encode(HeaderClientSafeboxCheckout, payload)
}

func DecodeClientSafeboxCheckout(f frame.Frame) (ClientSafeboxCheckoutPacket, error) {
	if f.Header != HeaderClientSafeboxCheckout {
		return ClientSafeboxCheckoutPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientSafeboxTransferPayloadSize {
		return ClientSafeboxCheckoutPacket{}, ErrInvalidPayload
	}
	return ClientSafeboxCheckoutPacket{SafeSlot: f.Payload[0], Position: decodePosition(f.Payload[1:])}, nil
}

func EncodeClientSafeboxItemMove(packet ClientSafeboxItemMovePacket) []byte {
	payload := make([]byte, clientSafeboxItemMovePayloadSize)
	encodePosition(payload[:positionSize], packet.Source)
	encodePosition(payload[positionSize:positionSize+positionSize], packet.Destination)
	payload[positionSize+positionSize] = packet.Count
	return frame.Encode(HeaderClientSafeboxItemMove, payload)
}

func DecodeClientSafeboxItemMove(f frame.Frame) (ClientSafeboxItemMovePacket, error) {
	if f.Header != HeaderClientSafeboxItemMove {
		return ClientSafeboxItemMovePacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientSafeboxItemMovePayloadSize {
		return ClientSafeboxItemMovePacket{}, ErrInvalidPayload
	}
	return ClientSafeboxItemMovePacket{
		Source:      decodePosition(f.Payload[:positionSize]),
		Destination: decodePosition(f.Payload[positionSize : positionSize+positionSize]),
		Count:       f.Payload[positionSize+positionSize],
	}, nil
}

func EncodeClientMallCheckout(packet ClientMallCheckoutPacket) []byte {
	payload := make([]byte, clientMallCheckoutPayloadSize)
	payload[0] = packet.MallSlot
	encodePosition(payload[1:], packet.Position)
	return frame.Encode(HeaderClientMallCheckout, payload)
}

func DecodeClientMallCheckout(f frame.Frame) (ClientMallCheckoutPacket, error) {
	if f.Header != HeaderClientMallCheckout {
		return ClientMallCheckoutPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientMallCheckoutPayloadSize {
		return ClientMallCheckoutPacket{}, ErrInvalidPayload
	}
	return ClientMallCheckoutPacket{MallSlot: f.Payload[0], Position: decodePosition(f.Payload[1:])}, nil
}

func EncodeSafeboxSet(packet SetPacket) []byte {
	payload := encodeSetPayload(packet)
	return frame.Encode(HeaderSafeboxSet, payload)
}

func DecodeSafeboxSet(f frame.Frame) (SetPacket, error) {
	return decodeSetWithHeader(f, HeaderSafeboxSet)
}

func EncodeSafeboxDel(packet DelPacket) []byte {
	payload := encodeDelPayload(packet)
	return frame.Encode(HeaderSafeboxDel, payload)
}

func DecodeSafeboxDel(f frame.Frame) (DelPacket, error) {
	return decodeDelWithHeader(f, HeaderSafeboxDel)
}

func EncodeSafeboxWrongPassword() []byte {
	return frame.Encode(HeaderSafeboxWrongPassword, nil)
}

func DecodeSafeboxWrongPassword(f frame.Frame) (SafeboxWrongPasswordPacket, error) {
	if f.Header != HeaderSafeboxWrongPassword {
		return SafeboxWrongPasswordPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != 0 {
		return SafeboxWrongPasswordPacket{}, ErrInvalidPayload
	}
	return SafeboxWrongPasswordPacket{}, nil
}

func EncodeSafeboxSize(packet SafeboxSizePacket) []byte {
	return frame.Encode(HeaderSafeboxSize, []byte{packet.Size})
}

func DecodeSafeboxSize(f frame.Frame) (SafeboxSizePacket, error) {
	if f.Header != HeaderSafeboxSize {
		return SafeboxSizePacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != safeboxSizePayloadSize {
		return SafeboxSizePacket{}, ErrInvalidPayload
	}
	return SafeboxSizePacket{Size: f.Payload[0]}, nil
}

func EncodeSafeboxMoneyChange(packet SafeboxMoneyChangePacket) []byte {
	payload := make([]byte, safeboxMoneyChangePayloadSize)
	binary.LittleEndian.PutUint32(payload, uint32(packet.Money))
	return frame.Encode(HeaderSafeboxMoneyChange, payload)
}

func DecodeSafeboxMoneyChange(f frame.Frame) (SafeboxMoneyChangePacket, error) {
	if f.Header != HeaderSafeboxMoneyChange {
		return SafeboxMoneyChangePacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != safeboxMoneyChangePayloadSize {
		return SafeboxMoneyChangePacket{}, ErrInvalidPayload
	}
	return SafeboxMoneyChangePacket{Money: int32(binary.LittleEndian.Uint32(f.Payload))}, nil
}

func EncodeMallOpen(packet MallOpenPacket) []byte {
	return frame.Encode(HeaderMallOpen, []byte{packet.Size})
}

func DecodeMallOpen(f frame.Frame) (MallOpenPacket, error) {
	if f.Header != HeaderMallOpen {
		return MallOpenPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != mallOpenPayloadSize {
		return MallOpenPacket{}, ErrInvalidPayload
	}
	return MallOpenPacket{Size: f.Payload[0]}, nil
}

func EncodeMallSet(packet SetPacket) []byte {
	payload := encodeSetPayload(packet)
	return frame.Encode(HeaderMallSet, payload)
}

func DecodeMallSet(f frame.Frame) (SetPacket, error) {
	return decodeSetWithHeader(f, HeaderMallSet)
}

func EncodeMallDel(packet DelPacket) []byte {
	payload := encodeDelPayload(packet)
	return frame.Encode(HeaderMallDel, payload)
}

func DecodeMallDel(f frame.Frame) (DelPacket, error) {
	return decodeDelWithHeader(f, HeaderMallDel)
}

func EncodeServerExchange(packet ServerExchangePacket) []byte {
	payload := make([]byte, serverExchangePayloadSize)
	payload[0] = packet.Subheader
	payload[1] = packet.IsMe
	binary.LittleEndian.PutUint32(payload[2:], packet.Arg1)
	encodePosition(payload[6:6+positionSize], packet.Position)
	offset := 6 + positionSize
	binary.LittleEndian.PutUint32(payload[offset:], packet.Arg3)
	offset += 4
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
	return frame.Encode(HeaderServerExchange, payload)
}

func DecodeServerExchange(f frame.Frame) (ServerExchangePacket, error) {
	if f.Header != HeaderServerExchange {
		return ServerExchangePacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != serverExchangePayloadSize {
		return ServerExchangePacket{}, ErrInvalidPayload
	}
	packet := ServerExchangePacket{
		Subheader: f.Payload[0],
		IsMe:      f.Payload[1],
		Arg1:      binary.LittleEndian.Uint32(f.Payload[2:]),
		Position:  decodePosition(f.Payload[6 : 6+positionSize]),
	}
	offset := 6 + positionSize
	packet.Arg3 = binary.LittleEndian.Uint32(f.Payload[offset:])
	offset += 4
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

func EncodeSet(packet SetPacket) []byte {
	return frame.Encode(HeaderSet, encodeSetPayload(packet))
}

func encodeSetPayload(packet SetPacket) []byte {
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
	return payload
}

func DecodeSet(f frame.Frame) (SetPacket, error) {
	return decodeSetWithHeader(f, HeaderSet)
}

func decodeSetWithHeader(f frame.Frame, header uint16) (SetPacket, error) {
	if f.Header != header {
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
	return frame.Encode(HeaderDel, encodeDelPayload(packet))
}

func encodeDelPayload(packet DelPacket) []byte {
	payload := make([]byte, delPayloadSize)
	encodePosition(payload, packet.Position)
	return payload
}

func DecodeDel(f frame.Frame) (DelPacket, error) {
	return decodeDelWithHeader(f, HeaderDel)
}

func decodeDelWithHeader(f frame.Frame, header uint16) (DelPacket, error) {
	if f.Header != header {
		return DelPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != delPayloadSize {
		return DelPacket{}, ErrInvalidPayload
	}
	return DelPacket{Position: decodePosition(f.Payload)}, nil
}

func EncodeUse(packet UsePacket) []byte {
	payload := make([]byte, usePayloadSize)
	encodePosition(payload[:positionSize], packet.Position)
	offset := positionSize
	binary.LittleEndian.PutUint32(payload[offset:], packet.CharacterVID)
	offset += 4
	binary.LittleEndian.PutUint32(payload[offset:], packet.VictimVID)
	offset += 4
	binary.LittleEndian.PutUint32(payload[offset:], packet.Vnum)
	return frame.Encode(HeaderUse, payload)
}

func DecodeUse(f frame.Frame) (UsePacket, error) {
	if f.Header != HeaderUse {
		return UsePacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != usePayloadSize {
		return UsePacket{}, ErrInvalidPayload
	}
	offset := positionSize
	packet := UsePacket{Position: decodePosition(f.Payload[:positionSize])}
	packet.CharacterVID = binary.LittleEndian.Uint32(f.Payload[offset:])
	offset += 4
	packet.VictimVID = binary.LittleEndian.Uint32(f.Payload[offset:])
	offset += 4
	packet.Vnum = binary.LittleEndian.Uint32(f.Payload[offset:])
	return packet, nil
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
	binary.LittleEndian.PutUint32(payload[0:], uint32(packet.X))
	binary.LittleEndian.PutUint32(payload[4:], uint32(packet.Y))
	binary.LittleEndian.PutUint32(payload[8:], uint32(packet.Z))
	binary.LittleEndian.PutUint32(payload[12:], packet.VID)
	binary.LittleEndian.PutUint32(payload[16:], packet.Vnum)
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
		X:    int32(binary.LittleEndian.Uint32(f.Payload[0:])),
		Y:    int32(binary.LittleEndian.Uint32(f.Payload[4:])),
		Z:    int32(binary.LittleEndian.Uint32(f.Payload[8:])),
		VID:  binary.LittleEndian.Uint32(f.Payload[12:]),
		Vnum: binary.LittleEndian.Uint32(f.Payload[16:]),
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
