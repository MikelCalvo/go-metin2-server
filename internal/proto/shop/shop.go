package shop

import (
	"encoding/binary"
	"errors"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
)

const (
	HeaderClientShop uint16 = 0x0801
	HeaderServerShop uint16 = 0x0810

	ClientSubheaderEnd   uint8 = 0
	ClientSubheaderBuy   uint8 = 1
	ClientSubheaderSell  uint8 = 2
	ClientSubheaderSell2 uint8 = 3

	ServerSubheaderStart            uint8 = 0
	ServerSubheaderEnd              uint8 = 1
	ServerSubheaderUpdateItem       uint8 = 2
	ServerSubheaderUpdatePrice      uint8 = 3
	ServerSubheaderOK               uint8 = 4
	ServerSubheaderNotEnoughMoney   uint8 = 5
	ServerSubheaderSoldout          uint8 = 6
	ServerSubheaderInventoryFull    uint8 = 7
	ServerSubheaderInvalidPos       uint8 = 8
	ServerSubheaderSoldOut          uint8 = 9
	ServerSubheaderStartEx          uint8 = 10
	ServerSubheaderNotEnoughMoneyEx uint8 = 11

	ShopHostItemMax = 40
	ShopTabNameMax  = 32

	attributeSize                = 3
	itemEntrySize                = 4 + 4 + 1 + 1 + (itemproto.ItemSocketCount * 4) + (itemproto.ItemAttributeCount * attributeSize)
	shopTabSize                  = ShopTabNameMax + 1 + (ShopHostItemMax * itemEntrySize)
	clientBuyPayloadSize         = 3
	clientEndPayloadSize         = 1
	clientSellPayloadSize        = 2
	clientSell2PayloadSize       = 3
	serverStartPayloadSize       = 1 + 4 + (ShopHostItemMax * itemEntrySize)
	serverStartExFixedSize       = 1 + 4 + 1
	serverUpdateItemPayloadSize  = 1 + 1 + itemEntrySize
	serverUpdatePricePayloadSize = 1 + 4
	serverEndPayloadSize         = 1
	serverStartOwnerOffset       = 1
	serverStartItemsOffset       = 5
	serverStartExTabCountOffset  = 5
	serverStartExTabsOffset      = 6
	itemEntrySocketsOffset       = 10
	itemEntryAttrsOffset         = itemEntrySocketsOffset + (itemproto.ItemSocketCount * 4)
)

var (
	ErrUnexpectedHeader    = errors.New("unexpected shop packet header")
	ErrUnexpectedSubheader = errors.New("unexpected shop packet subheader")
	ErrInvalidPayload      = errors.New("invalid shop packet payload")
)

type ClientBuyPacket struct {
	RawLeadingByte uint8
	CatalogSlot    uint8
}

type ClientSellPacket struct {
	Slot uint8
}

type ClientSell2Packet struct {
	Slot  uint8
	Count uint8
}

type ItemEntry struct {
	Vnum       uint32
	Price      uint32
	Count      uint8
	DisplayPos uint8
	Sockets    [itemproto.ItemSocketCount]int32
	Attributes [itemproto.ItemAttributeCount]itemproto.Attribute
}

type ServerStartPacket struct {
	OwnerVID uint32
	Items    [ShopHostItemMax]ItemEntry
}

type ShopTab struct {
	Name     string
	CoinType uint8
	Items    [ShopHostItemMax]ItemEntry
}

type ServerStartExPacket struct {
	OwnerVID uint32
	Tabs     []ShopTab
}

type ServerUpdateItemPacket struct {
	Position uint8
	Item     ItemEntry
}

type ServerUpdatePricePacket struct {
	ElkAmount int32
}

func EncodeClientBuy(packet ClientBuyPacket) []byte {
	return frame.Encode(HeaderClientShop, []byte{ClientSubheaderBuy, packet.RawLeadingByte, packet.CatalogSlot})
}

func DecodeClientBuy(f frame.Frame) (ClientBuyPacket, error) {
	if f.Header != HeaderClientShop {
		return ClientBuyPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientBuyPayloadSize {
		return ClientBuyPacket{}, ErrInvalidPayload
	}
	if f.Payload[0] != ClientSubheaderBuy {
		return ClientBuyPacket{}, ErrUnexpectedSubheader
	}
	return ClientBuyPacket{RawLeadingByte: f.Payload[1], CatalogSlot: f.Payload[2]}, nil
}

func EncodeClientEnd() []byte {
	return frame.Encode(HeaderClientShop, []byte{ClientSubheaderEnd})
}

func DecodeClientEnd(f frame.Frame) error {
	if f.Header != HeaderClientShop {
		return ErrUnexpectedHeader
	}
	if len(f.Payload) != clientEndPayloadSize {
		return ErrInvalidPayload
	}
	if f.Payload[0] != ClientSubheaderEnd {
		return ErrUnexpectedSubheader
	}
	return nil
}

func EncodeClientSell(packet ClientSellPacket) []byte {
	return frame.Encode(HeaderClientShop, []byte{ClientSubheaderSell, packet.Slot})
}

func DecodeClientSell(f frame.Frame) (ClientSellPacket, error) {
	if f.Header != HeaderClientShop {
		return ClientSellPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientSellPayloadSize {
		return ClientSellPacket{}, ErrInvalidPayload
	}
	if f.Payload[0] != ClientSubheaderSell {
		return ClientSellPacket{}, ErrUnexpectedSubheader
	}
	return ClientSellPacket{Slot: f.Payload[1]}, nil
}

func EncodeClientSell2(packet ClientSell2Packet) []byte {
	return frame.Encode(HeaderClientShop, []byte{ClientSubheaderSell2, packet.Slot, packet.Count})
}

func DecodeClientSell2(f frame.Frame) (ClientSell2Packet, error) {
	if f.Header != HeaderClientShop {
		return ClientSell2Packet{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientSell2PayloadSize {
		return ClientSell2Packet{}, ErrInvalidPayload
	}
	if f.Payload[0] != ClientSubheaderSell2 {
		return ClientSell2Packet{}, ErrUnexpectedSubheader
	}
	return ClientSell2Packet{Slot: f.Payload[1], Count: f.Payload[2]}, nil
}

func EncodeServerStart(packet ServerStartPacket) []byte {
	payload := make([]byte, serverStartPayloadSize)
	payload[0] = ServerSubheaderStart
	binary.LittleEndian.PutUint32(payload[serverStartOwnerOffset:], packet.OwnerVID)
	offset := serverStartItemsOffset
	for _, item := range packet.Items {
		encodeItemEntry(payload[offset:offset+itemEntrySize], item)
		offset += itemEntrySize
	}
	return frame.Encode(HeaderServerShop, payload)
}

func DecodeServerStart(f frame.Frame) (ServerStartPacket, error) {
	if f.Header != HeaderServerShop {
		return ServerStartPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != serverStartPayloadSize {
		return ServerStartPacket{}, ErrInvalidPayload
	}
	if f.Payload[0] != ServerSubheaderStart {
		return ServerStartPacket{}, ErrUnexpectedSubheader
	}
	packet := ServerStartPacket{OwnerVID: binary.LittleEndian.Uint32(f.Payload[serverStartOwnerOffset:])}
	offset := serverStartItemsOffset
	for i := range packet.Items {
		packet.Items[i] = decodeItemEntry(f.Payload[offset : offset+itemEntrySize])
		offset += itemEntrySize
	}
	return packet, nil
}

func EncodeServerStartEx(packet ServerStartExPacket) []byte {
	payload := make([]byte, serverStartExFixedSize+(len(packet.Tabs)*shopTabSize))
	payload[0] = ServerSubheaderStartEx
	binary.LittleEndian.PutUint32(payload[serverStartOwnerOffset:], packet.OwnerVID)
	payload[serverStartExTabCountOffset] = uint8(len(packet.Tabs))
	offset := serverStartExTabsOffset
	for _, tab := range packet.Tabs {
		encodeShopTab(payload[offset:offset+shopTabSize], tab)
		offset += shopTabSize
	}
	return frame.Encode(HeaderServerShop, payload)
}

func DecodeServerStartEx(f frame.Frame) (ServerStartExPacket, error) {
	if f.Header != HeaderServerShop {
		return ServerStartExPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) < serverStartExFixedSize {
		return ServerStartExPacket{}, ErrInvalidPayload
	}
	if f.Payload[0] != ServerSubheaderStartEx {
		return ServerStartExPacket{}, ErrUnexpectedSubheader
	}
	tabCount := int(f.Payload[serverStartExTabCountOffset])
	if len(f.Payload) != serverStartExFixedSize+(tabCount*shopTabSize) {
		return ServerStartExPacket{}, ErrInvalidPayload
	}
	packet := ServerStartExPacket{
		OwnerVID: binary.LittleEndian.Uint32(f.Payload[serverStartOwnerOffset:]),
		Tabs:     make([]ShopTab, tabCount),
	}
	offset := serverStartExTabsOffset
	for i := range packet.Tabs {
		packet.Tabs[i] = decodeShopTab(f.Payload[offset : offset+shopTabSize])
		offset += shopTabSize
	}
	return packet, nil
}

func EncodeServerUpdateItem(packet ServerUpdateItemPacket) []byte {
	payload := make([]byte, serverUpdateItemPayloadSize)
	payload[0] = ServerSubheaderUpdateItem
	payload[1] = packet.Position
	encodeItemEntry(payload[2:], packet.Item)
	return frame.Encode(HeaderServerShop, payload)
}

func DecodeServerUpdateItem(f frame.Frame) (ServerUpdateItemPacket, error) {
	if f.Header != HeaderServerShop {
		return ServerUpdateItemPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != serverUpdateItemPayloadSize {
		return ServerUpdateItemPacket{}, ErrInvalidPayload
	}
	if f.Payload[0] != ServerSubheaderUpdateItem {
		return ServerUpdateItemPacket{}, ErrUnexpectedSubheader
	}
	return ServerUpdateItemPacket{Position: f.Payload[1], Item: decodeItemEntry(f.Payload[2:])}, nil
}

func EncodeServerUpdatePrice(packet ServerUpdatePricePacket) []byte {
	payload := make([]byte, serverUpdatePricePayloadSize)
	payload[0] = ServerSubheaderUpdatePrice
	binary.LittleEndian.PutUint32(payload[1:], uint32(packet.ElkAmount))
	return frame.Encode(HeaderServerShop, payload)
}

func DecodeServerUpdatePrice(f frame.Frame) (ServerUpdatePricePacket, error) {
	if f.Header != HeaderServerShop {
		return ServerUpdatePricePacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != serverUpdatePricePayloadSize {
		return ServerUpdatePricePacket{}, ErrInvalidPayload
	}
	if f.Payload[0] != ServerSubheaderUpdatePrice {
		return ServerUpdatePricePacket{}, ErrUnexpectedSubheader
	}
	return ServerUpdatePricePacket{ElkAmount: int32(binary.LittleEndian.Uint32(f.Payload[1:]))}, nil
}

func EncodeServerEnd() []byte {
	return frame.Encode(HeaderServerShop, []byte{ServerSubheaderEnd})
}

func DecodeServerEnd(f frame.Frame) error {
	return decodeServerBareSubheader(f, ServerSubheaderEnd)
}

func EncodeServerOK() []byte {
	return frame.Encode(HeaderServerShop, []byte{ServerSubheaderOK})
}

func DecodeServerOK(f frame.Frame) error {
	return decodeServerBareSubheader(f, ServerSubheaderOK)
}

func EncodeServerNotEnoughMoney() []byte {
	return frame.Encode(HeaderServerShop, []byte{ServerSubheaderNotEnoughMoney})
}

func DecodeServerNotEnoughMoney(f frame.Frame) error {
	return decodeServerBareSubheader(f, ServerSubheaderNotEnoughMoney)
}

func EncodeServerSoldout() []byte {
	return frame.Encode(HeaderServerShop, []byte{ServerSubheaderSoldout})
}

func DecodeServerSoldout(f frame.Frame) error {
	return decodeServerBareSubheader(f, ServerSubheaderSoldout)
}

func EncodeServerInventoryFull() []byte {
	return frame.Encode(HeaderServerShop, []byte{ServerSubheaderInventoryFull})
}

func DecodeServerInventoryFull(f frame.Frame) error {
	return decodeServerBareSubheader(f, ServerSubheaderInventoryFull)
}

func EncodeServerInvalidPos() []byte {
	return frame.Encode(HeaderServerShop, []byte{ServerSubheaderInvalidPos})
}

func DecodeServerInvalidPos(f frame.Frame) error {
	return decodeServerBareSubheader(f, ServerSubheaderInvalidPos)
}

func EncodeServerSoldOut() []byte {
	return frame.Encode(HeaderServerShop, []byte{ServerSubheaderSoldOut})
}

func DecodeServerSoldOut(f frame.Frame) error {
	return decodeServerBareSubheader(f, ServerSubheaderSoldOut)
}

func EncodeServerNotEnoughMoneyEx() []byte {
	return frame.Encode(HeaderServerShop, []byte{ServerSubheaderNotEnoughMoneyEx})
}

func DecodeServerNotEnoughMoneyEx(f frame.Frame) error {
	return decodeServerBareSubheader(f, ServerSubheaderNotEnoughMoneyEx)
}

func decodeServerBareSubheader(f frame.Frame, subheader uint8) error {
	if f.Header != HeaderServerShop {
		return ErrUnexpectedHeader
	}
	if len(f.Payload) != serverEndPayloadSize {
		return ErrInvalidPayload
	}
	if f.Payload[0] != subheader {
		return ErrUnexpectedSubheader
	}
	return nil
}

func encodeShopTab(dst []byte, tab ShopTab) {
	copy(dst[:ShopTabNameMax], []byte(tab.Name))
	dst[ShopTabNameMax] = tab.CoinType
	offset := ShopTabNameMax + 1
	for _, item := range tab.Items {
		encodeItemEntry(dst[offset:offset+itemEntrySize], item)
		offset += itemEntrySize
	}
}

func decodeShopTab(src []byte) ShopTab {
	tab := ShopTab{
		Name:     fixedString(src[:ShopTabNameMax]),
		CoinType: src[ShopTabNameMax],
	}
	offset := ShopTabNameMax + 1
	for i := range tab.Items {
		tab.Items[i] = decodeItemEntry(src[offset : offset+itemEntrySize])
		offset += itemEntrySize
	}
	return tab
}

func encodeItemEntry(dst []byte, item ItemEntry) {
	binary.LittleEndian.PutUint32(dst[0:], item.Vnum)
	binary.LittleEndian.PutUint32(dst[4:], item.Price)
	dst[8] = item.Count
	dst[9] = item.DisplayPos
	offset := itemEntrySocketsOffset
	for _, socket := range item.Sockets {
		binary.LittleEndian.PutUint32(dst[offset:], uint32(socket))
		offset += 4
	}
	offset = itemEntryAttrsOffset
	for _, attribute := range item.Attributes {
		dst[offset] = attribute.Type
		offset++
		binary.LittleEndian.PutUint16(dst[offset:], uint16(attribute.Value))
		offset += 2
	}
}

func fixedString(src []byte) string {
	for i, b := range src {
		if b == 0 {
			return string(src[:i])
		}
	}
	return string(src)
}

func decodeItemEntry(src []byte) ItemEntry {
	item := ItemEntry{
		Vnum:       binary.LittleEndian.Uint32(src[0:]),
		Price:      binary.LittleEndian.Uint32(src[4:]),
		Count:      src[8],
		DisplayPos: src[9],
	}
	offset := itemEntrySocketsOffset
	for i := range item.Sockets {
		item.Sockets[i] = int32(binary.LittleEndian.Uint32(src[offset:]))
		offset += 4
	}
	offset = itemEntryAttrsOffset
	for i := range item.Attributes {
		item.Attributes[i].Type = src[offset]
		offset++
		item.Attributes[i].Value = int16(binary.LittleEndian.Uint16(src[offset:]))
		offset += 2
	}
	return item
}
