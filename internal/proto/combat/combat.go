package combat

import (
	"encoding/binary"
	"errors"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

const (
	HeaderClientTarget uint16 = 0x0A01
	HeaderServerTarget uint16 = 0x0A10

	clientTargetPayloadSize = 4
	serverTargetPayloadSize = 5
)

var (
	ErrUnexpectedHeader = errors.New("unexpected combat packet header")
	ErrInvalidPayload   = errors.New("invalid combat packet payload")
)

type ClientTargetPacket struct {
	TargetVID uint32
}

type ServerTargetPacket struct {
	TargetVID uint32
	HPPercent uint8
}

func EncodeClientTarget(packet ClientTargetPacket) []byte {
	payload := make([]byte, clientTargetPayloadSize)
	binary.LittleEndian.PutUint32(payload, packet.TargetVID)
	return frame.Encode(HeaderClientTarget, payload)
}

func DecodeClientTarget(f frame.Frame) (ClientTargetPacket, error) {
	if f.Header != HeaderClientTarget {
		return ClientTargetPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientTargetPayloadSize {
		return ClientTargetPacket{}, ErrInvalidPayload
	}
	return ClientTargetPacket{TargetVID: binary.LittleEndian.Uint32(f.Payload)}, nil
}

func EncodeServerTarget(packet ServerTargetPacket) []byte {
	payload := make([]byte, serverTargetPayloadSize)
	binary.LittleEndian.PutUint32(payload, packet.TargetVID)
	payload[4] = packet.HPPercent
	return frame.Encode(HeaderServerTarget, payload)
}

func DecodeServerTarget(f frame.Frame) (ServerTargetPacket, error) {
	if f.Header != HeaderServerTarget {
		return ServerTargetPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != serverTargetPayloadSize {
		return ServerTargetPacket{}, ErrInvalidPayload
	}
	return ServerTargetPacket{TargetVID: binary.LittleEndian.Uint32(f.Payload), HPPercent: f.Payload[4]}, nil
}
