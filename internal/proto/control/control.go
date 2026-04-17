package control

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

const (
	HeaderPong  uint16 = 0x0006
	HeaderPing  uint16 = 0x0007
	HeaderPhase uint16 = 0x0008
)

var (
	ErrUnexpectedHeader  = errors.New("unexpected control packet header")
	ErrInvalidPayload    = errors.New("invalid control packet payload")
	ErrUnknownPhaseValue = errors.New("unknown phase value")
)

type PhasePacket struct {
	Phase session.Phase
}

type PingPacket struct {
	ServerTime uint32
}

func EncodePhase(phase session.Phase) ([]byte, error) {
	value, err := encodePhaseValue(phase)
	if err != nil {
		return nil, err
	}

	return frame.Encode(HeaderPhase, []byte{value}), nil
}

func DecodePhase(f frame.Frame) (PhasePacket, error) {
	if f.Header != HeaderPhase {
		return PhasePacket{}, ErrUnexpectedHeader
	}

	if len(f.Payload) != 1 {
		return PhasePacket{}, ErrInvalidPayload
	}

	phase, err := decodePhaseValue(f.Payload[0])
	if err != nil {
		return PhasePacket{}, err
	}

	return PhasePacket{Phase: phase}, nil
}

func DecodePing(f frame.Frame) (PingPacket, error) {
	if f.Header != HeaderPing {
		return PingPacket{}, ErrUnexpectedHeader
	}

	if len(f.Payload) != 4 {
		return PingPacket{}, ErrInvalidPayload
	}

	return PingPacket{ServerTime: binary.LittleEndian.Uint32(f.Payload)}, nil
}

func EncodePong() []byte {
	return frame.Encode(HeaderPong, nil)
}

func encodePhaseValue(phase session.Phase) (byte, error) {
	switch phase {
	case session.PhaseClose:
		return 0x00, nil
	case session.PhaseHandshake:
		return 0x01, nil
	case session.PhaseLogin:
		return 0x02, nil
	case session.PhaseSelect:
		return 0x03, nil
	case session.PhaseLoading:
		return 0x04, nil
	case session.PhaseGame:
		return 0x05, nil
	case session.PhaseDead:
		return 0x06, nil
	default:
		return 0, fmt.Errorf("%w: %q", ErrUnknownPhaseValue, phase)
	}
}

func decodePhaseValue(value byte) (session.Phase, error) {
	switch value {
	case 0x00:
		return session.PhaseClose, nil
	case 0x01:
		return session.PhaseHandshake, nil
	case 0x02:
		return session.PhaseLogin, nil
	case 0x03:
		return session.PhaseSelect, nil
	case 0x04:
		return session.PhaseLoading, nil
	case 0x05:
		return session.PhaseGame, nil
	case 0x06:
		return session.PhaseDead, nil
	default:
		return "", fmt.Errorf("%w: 0x%02x", ErrUnknownPhaseValue, value)
	}
}
