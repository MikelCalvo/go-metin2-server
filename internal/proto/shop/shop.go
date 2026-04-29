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

	ClientSubheaderEnd uint8 = 0
	ClientSubheaderBuy uint8 = 1

	ServerSubheaderStart uint8 = 0
	ServerSubheaderEnd   uint8 = 1

	ShopHostItemMax = 40

	attributeSize          = 3
	itemEntrySize          = 4 + 4 + 1 + 1 + (itemproto.ItemSocketCount * 4) + (itemproto.ItemAttributeCount * attributeSize)
	clientBuyPayloadSize   = 3
	clientEndPayloadSize   = 1
	serverStartPayloadSize = 1 + 4 + (ShopHostItemMax * itemEntrySize)
	serverEndPayloadSize   = 1
	serverStartOwnerOffset = 1
	serverStartItemsOffset = 5
	itemEntrySocketsOffset = 10
	itemEntryAttrsOffset   = itemEntrySocketsOffset + (itemproto.ItemSocketCount * 4)
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

func EncodeServerEnd() []byte {
	return frame.Encode(HeaderServerShop, []byte{ServerSubheaderEnd})
}

func DecodeServerEnd(f frame.Frame) error {
	if f.Header != HeaderServerShop {
		return ErrUnexpectedHeader
	}
	if len(f.Payload) != serverEndPayloadSize {
		return ErrInvalidPayload
	}
	if f.Payload[0] != ServerSubheaderEnd {
		return ErrUnexpectedSubheader
	}
	return nil
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
