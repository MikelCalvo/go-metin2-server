package chat

import (
	"bytes"
	"errors"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

const (
	HeaderClientChat uint16 = 0x0601
	HeaderChat       uint16 = 0x0603

	ChatTypeTalking   uint8 = 0
	ChatTypeInfo      uint8 = 1
	ChatTypeNotice    uint8 = 2
	ChatTypeParty     uint8 = 3
	ChatTypeGuild     uint8 = 4
	ChatTypeCommand   uint8 = 5
	ChatTypeShout     uint8 = 6
	ChatTypeWhisper   uint8 = 7
	ChatTypeBigNotice uint8 = 8

	chatDeliveryFixedPayloadSize = 6
)

var (
	ErrUnexpectedHeader = errors.New("unexpected chat packet header")
	ErrInvalidPayload   = errors.New("invalid chat packet payload")
)

type ClientChatPacket struct {
	Type    uint8
	Message string
}

type ChatDeliveryPacket struct {
	Type    uint8
	VID     uint32
	Empire  uint8
	Message string
}

func EncodeClientChat(packet ClientChatPacket) []byte {
	payload := make([]byte, 1+len(packet.Message)+1)
	payload[0] = packet.Type
	copy(payload[1:], packet.Message)
	return frame.Encode(HeaderClientChat, payload)
}

func DecodeClientChat(f frame.Frame) (ClientChatPacket, error) {
	if f.Header != HeaderClientChat {
		return ClientChatPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) < 2 {
		return ClientChatPacket{}, ErrInvalidPayload
	}
	return ClientChatPacket{
		Type:    f.Payload[0],
		Message: string(bytes.TrimRight(f.Payload[1:], "\x00")),
	}, nil
}

func EncodeChatDelivery(packet ChatDeliveryPacket) []byte {
	payload := make([]byte, chatDeliveryFixedPayloadSize+len(packet.Message))
	payload[0] = packet.Type
	putUint32LE(payload[1:], packet.VID)
	payload[5] = packet.Empire
	copy(payload[6:], packet.Message)
	return frame.Encode(HeaderChat, payload)
}

func DecodeChatDelivery(f frame.Frame) (ChatDeliveryPacket, error) {
	if f.Header != HeaderChat {
		return ChatDeliveryPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) < chatDeliveryFixedPayloadSize {
		return ChatDeliveryPacket{}, ErrInvalidPayload
	}
	return ChatDeliveryPacket{
		Type:    f.Payload[0],
		VID:     uint32(f.Payload[1]) | uint32(f.Payload[2])<<8 | uint32(f.Payload[3])<<16 | uint32(f.Payload[4])<<24,
		Empire:  f.Payload[5],
		Message: string(bytes.TrimRight(f.Payload[6:], "\x00")),
	}, nil
}

func putUint32LE(dst []byte, value uint32) {
	dst[0] = byte(value)
	dst[1] = byte(value >> 8)
	dst[2] = byte(value >> 16)
	dst[3] = byte(value >> 24)
}
