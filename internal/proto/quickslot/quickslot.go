package quickslot

import (
	"errors"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

const (
	HeaderClientAdd  uint16 = 0x0509
	HeaderClientDel  uint16 = 0x050A
	HeaderClientSwap uint16 = 0x050B

	HeaderAdd  uint16 = 0x0519
	HeaderDel  uint16 = 0x051A
	HeaderSwap uint16 = 0x051B

	TypeNone    uint8 = 0
	TypeItem    uint8 = 1
	TypeSkill   uint8 = 2
	TypeCommand uint8 = 3

	slotPayloadSize = 2
	addPayloadSize  = 1 + slotPayloadSize
	delPayloadSize  = 1
	swapPayloadSize = 2
)

var (
	ErrUnexpectedHeader = errors.New("unexpected quickslot packet header")
	ErrInvalidPayload   = errors.New("invalid quickslot packet payload")
)

type Slot struct {
	Type     uint8
	Position uint8
}

type ClientAddPacket struct {
	Position uint8
	Slot     Slot
}

type ClientDelPacket struct {
	Position uint8
}

type ClientSwapPacket struct {
	Position       uint8
	TargetPosition uint8
}

type AddPacket struct {
	Position uint8
	Slot     Slot
}

type DelPacket struct {
	Position uint8
}

type SwapPacket struct {
	Position       uint8
	TargetPosition uint8
}

func EncodeClientAdd(packet ClientAddPacket) []byte {
	payload := []byte{packet.Position, packet.Slot.Type, packet.Slot.Position}
	return frame.Encode(HeaderClientAdd, payload)
}

func DecodeClientAdd(f frame.Frame) (ClientAddPacket, error) {
	if f.Header != HeaderClientAdd {
		return ClientAddPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != addPayloadSize {
		return ClientAddPacket{}, ErrInvalidPayload
	}
	return ClientAddPacket{Position: f.Payload[0], Slot: Slot{Type: f.Payload[1], Position: f.Payload[2]}}, nil
}

func EncodeClientDel(packet ClientDelPacket) []byte {
	return frame.Encode(HeaderClientDel, []byte{packet.Position})
}

func DecodeClientDel(f frame.Frame) (ClientDelPacket, error) {
	if f.Header != HeaderClientDel {
		return ClientDelPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != delPayloadSize {
		return ClientDelPacket{}, ErrInvalidPayload
	}
	return ClientDelPacket{Position: f.Payload[0]}, nil
}

func EncodeClientSwap(packet ClientSwapPacket) []byte {
	return frame.Encode(HeaderClientSwap, []byte{packet.Position, packet.TargetPosition})
}

func DecodeClientSwap(f frame.Frame) (ClientSwapPacket, error) {
	if f.Header != HeaderClientSwap {
		return ClientSwapPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != swapPayloadSize {
		return ClientSwapPacket{}, ErrInvalidPayload
	}
	return ClientSwapPacket{Position: f.Payload[0], TargetPosition: f.Payload[1]}, nil
}

func EncodeAdd(packet AddPacket) []byte {
	payload := []byte{packet.Position, packet.Slot.Type, packet.Slot.Position}
	return frame.Encode(HeaderAdd, payload)
}

func DecodeAdd(f frame.Frame) (AddPacket, error) {
	if f.Header != HeaderAdd {
		return AddPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != addPayloadSize {
		return AddPacket{}, ErrInvalidPayload
	}
	return AddPacket{Position: f.Payload[0], Slot: Slot{Type: f.Payload[1], Position: f.Payload[2]}}, nil
}

func EncodeDel(packet DelPacket) []byte {
	return frame.Encode(HeaderDel, []byte{packet.Position})
}

func DecodeDel(f frame.Frame) (DelPacket, error) {
	if f.Header != HeaderDel {
		return DelPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != delPayloadSize {
		return DelPacket{}, ErrInvalidPayload
	}
	return DelPacket{Position: f.Payload[0]}, nil
}

func EncodeSwap(packet SwapPacket) []byte {
	return frame.Encode(HeaderSwap, []byte{packet.Position, packet.TargetPosition})
}

func DecodeSwap(f frame.Frame) (SwapPacket, error) {
	if f.Header != HeaderSwap {
		return SwapPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != swapPayloadSize {
		return SwapPacket{}, ErrInvalidPayload
	}
	return SwapPacket{Position: f.Payload[0], TargetPosition: f.Payload[1]}, nil
}
