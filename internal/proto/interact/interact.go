package interact

import (
	"encoding/binary"
	"errors"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

const (
	HeaderRequest uint16 = 0x0501

	requestPayloadSize = 4
)

var (
	ErrUnexpectedHeader = errors.New("unexpected interact packet header")
	ErrInvalidPayload   = errors.New("invalid interact packet payload")
)

type RequestPacket struct {
	TargetVID uint32
}

func EncodeRequest(packet RequestPacket) []byte {
	payload := make([]byte, requestPayloadSize)
	binary.LittleEndian.PutUint32(payload, packet.TargetVID)
	return frame.Encode(HeaderRequest, payload)
}

func DecodeRequest(f frame.Frame) (RequestPacket, error) {
	if f.Header != HeaderRequest {
		return RequestPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != requestPayloadSize {
		return RequestPacket{}, ErrInvalidPayload
	}
	return RequestPacket{TargetVID: binary.LittleEndian.Uint32(f.Payload)}, nil
}
