package control

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

const (
	HeaderPong                 uint16 = 0x0006
	HeaderPing                 uint16 = 0x0007
	HeaderPhase                uint16 = 0x0008
	HeaderKeyResponse          uint16 = 0x000A
	HeaderKeyChallenge         uint16 = 0x000B
	HeaderKeyComplete          uint16 = 0x000C
	HeaderClientVersion        uint16 = 0x000D
	HeaderStateChecker         uint16 = 0x000F
	HeaderRespondChannelStatus uint16 = 0x0010
	ChannelStatusNormal        uint8  = 1

	keySize                 = 32
	challengeSize           = 32
	challengeResponseSize   = 32
	encryptedTokenSize      = 48
	nonceSize               = 24
	clientVersionFieldSize  = 33
	channelStatusSize       = 3
	channelStatusCountSize  = 4
	keyChallengePayloadLen  = keySize + challengeSize + 4
	keyResponsePayloadLen   = keySize + challengeResponseSize
	keyCompletePayloadLen   = encryptedTokenSize + nonceSize
	clientVersionPayloadLen = clientVersionFieldSize * 2
)

var (
	ErrUnexpectedHeader  = errors.New("unexpected control packet header")
	ErrInvalidPayload    = errors.New("invalid control packet payload")
	ErrUnknownPhaseValue = errors.New("unknown phase value")
	ErrStringTooLong     = errors.New("string does not fit fixed-width wire field")
)

type PhasePacket struct {
	Phase session.Phase
}

type PingPacket struct {
	ServerTime uint32
}

type PongPacket struct{}

type ClientVersionPacket struct {
	ExecutableName string
	Timestamp      string
}

type KeyChallengePacket struct {
	ServerPublicKey [keySize]byte
	Challenge       [challengeSize]byte
	ServerTime      uint32
}

type KeyResponsePacket struct {
	ClientPublicKey   [keySize]byte
	ChallengeResponse [challengeResponseSize]byte
}

type KeyCompletePacket struct {
	EncryptedToken [encryptedTokenSize]byte
	Nonce          [nonceSize]byte
}

type StateCheckerPacket struct{}

type ChannelStatus struct {
	Port   int16
	Status uint8
}

type RespondChannelStatusPacket struct {
	Channels []ChannelStatus
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

func EncodePing(packet PingPacket) []byte {
	payload := make([]byte, 4)
	binary.LittleEndian.PutUint32(payload, packet.ServerTime)
	return frame.Encode(HeaderPing, payload)
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

func DecodePong(f frame.Frame) (PongPacket, error) {
	if f.Header != HeaderPong {
		return PongPacket{}, ErrUnexpectedHeader
	}

	if len(f.Payload) != 0 {
		return PongPacket{}, ErrInvalidPayload
	}

	return PongPacket{}, nil
}

func EncodeClientVersion(packet ClientVersionPacket) ([]byte, error) {
	payload := make([]byte, clientVersionPayloadLen)
	if err := putFixedString(payload[:clientVersionFieldSize], packet.ExecutableName); err != nil {
		return nil, err
	}
	if err := putFixedString(payload[clientVersionFieldSize:], packet.Timestamp); err != nil {
		return nil, err
	}
	return frame.Encode(HeaderClientVersion, payload), nil
}

func DecodeClientVersion(f frame.Frame) (ClientVersionPacket, error) {
	if f.Header != HeaderClientVersion {
		return ClientVersionPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != clientVersionPayloadLen {
		return ClientVersionPacket{}, ErrInvalidPayload
	}
	return ClientVersionPacket{
		ExecutableName: parseFixedString(f.Payload[:clientVersionFieldSize]),
		Timestamp:      parseFixedString(f.Payload[clientVersionFieldSize:]),
	}, nil
}

func EncodeStateChecker() []byte {
	return frame.Encode(HeaderStateChecker, nil)
}

func DecodeStateChecker(f frame.Frame) (StateCheckerPacket, error) {
	if f.Header != HeaderStateChecker {
		return StateCheckerPacket{}, ErrUnexpectedHeader
	}

	if len(f.Payload) != 0 {
		return StateCheckerPacket{}, ErrInvalidPayload
	}

	return StateCheckerPacket{}, nil
}

func EncodeRespondChannelStatus(packet RespondChannelStatusPacket) []byte {
	payload := make([]byte, channelStatusCountSize+len(packet.Channels)*channelStatusSize)
	binary.LittleEndian.PutUint32(payload[:channelStatusCountSize], uint32(len(packet.Channels)))

	offset := channelStatusCountSize
	for _, channel := range packet.Channels {
		binary.LittleEndian.PutUint16(payload[offset:offset+2], uint16(channel.Port))
		payload[offset+2] = channel.Status
		offset += channelStatusSize
	}

	return frame.Encode(HeaderRespondChannelStatus, payload)
}

func DecodeRespondChannelStatus(f frame.Frame) (RespondChannelStatusPacket, error) {
	if f.Header != HeaderRespondChannelStatus {
		return RespondChannelStatusPacket{}, ErrUnexpectedHeader
	}

	if len(f.Payload) < channelStatusCountSize {
		return RespondChannelStatusPacket{}, ErrInvalidPayload
	}

	count := binary.LittleEndian.Uint32(f.Payload[:channelStatusCountSize])
	if count > uint32((len(f.Payload)-channelStatusCountSize)/channelStatusSize) {
		return RespondChannelStatusPacket{}, ErrInvalidPayload
	}

	expectedLen := channelStatusCountSize + int(count)*channelStatusSize
	if len(f.Payload) != expectedLen {
		return RespondChannelStatusPacket{}, ErrInvalidPayload
	}

	packet := RespondChannelStatusPacket{Channels: make([]ChannelStatus, int(count))}
	offset := channelStatusCountSize
	for i := range packet.Channels {
		packet.Channels[i] = ChannelStatus{
			Port:   int16(binary.LittleEndian.Uint16(f.Payload[offset : offset+2])),
			Status: f.Payload[offset+2],
		}
		offset += channelStatusSize
	}

	return packet, nil
}

func EncodeKeyChallenge(packet KeyChallengePacket) []byte {
	payload := make([]byte, keyChallengePayloadLen)
	copy(payload[0:keySize], packet.ServerPublicKey[:])
	copy(payload[keySize:keySize+challengeSize], packet.Challenge[:])
	binary.LittleEndian.PutUint32(payload[keySize+challengeSize:], packet.ServerTime)

	return frame.Encode(HeaderKeyChallenge, payload)
}

func DecodeKeyChallenge(f frame.Frame) (KeyChallengePacket, error) {
	if f.Header != HeaderKeyChallenge {
		return KeyChallengePacket{}, ErrUnexpectedHeader
	}

	if len(f.Payload) != keyChallengePayloadLen {
		return KeyChallengePacket{}, ErrInvalidPayload
	}

	var packet KeyChallengePacket
	copy(packet.ServerPublicKey[:], f.Payload[0:keySize])
	copy(packet.Challenge[:], f.Payload[keySize:keySize+challengeSize])
	packet.ServerTime = binary.LittleEndian.Uint32(f.Payload[keySize+challengeSize:])

	return packet, nil
}

func EncodeKeyResponse(packet KeyResponsePacket) []byte {
	payload := make([]byte, keyResponsePayloadLen)
	copy(payload[0:keySize], packet.ClientPublicKey[:])
	copy(payload[keySize:], packet.ChallengeResponse[:])

	return frame.Encode(HeaderKeyResponse, payload)
}

func DecodeKeyResponse(f frame.Frame) (KeyResponsePacket, error) {
	if f.Header != HeaderKeyResponse {
		return KeyResponsePacket{}, ErrUnexpectedHeader
	}

	if len(f.Payload) != keyResponsePayloadLen {
		return KeyResponsePacket{}, ErrInvalidPayload
	}

	var packet KeyResponsePacket
	copy(packet.ClientPublicKey[:], f.Payload[0:keySize])
	copy(packet.ChallengeResponse[:], f.Payload[keySize:])

	return packet, nil
}

func EncodeKeyComplete(packet KeyCompletePacket) []byte {
	payload := make([]byte, keyCompletePayloadLen)
	copy(payload[0:encryptedTokenSize], packet.EncryptedToken[:])
	copy(payload[encryptedTokenSize:], packet.Nonce[:])

	return frame.Encode(HeaderKeyComplete, payload)
}

func DecodeKeyComplete(f frame.Frame) (KeyCompletePacket, error) {
	if f.Header != HeaderKeyComplete {
		return KeyCompletePacket{}, ErrUnexpectedHeader
	}

	if len(f.Payload) != keyCompletePayloadLen {
		return KeyCompletePacket{}, ErrInvalidPayload
	}

	var packet KeyCompletePacket
	copy(packet.EncryptedToken[:], f.Payload[0:encryptedTokenSize])
	copy(packet.Nonce[:], f.Payload[encryptedTokenSize:])

	return packet, nil
}

func putFixedString(dst []byte, value string) error {
	if len(value) > len(dst) {
		return ErrStringTooLong
	}
	copy(dst, value)
	return nil
}

func parseFixedString(src []byte) string {
	end := len(src)
	for i, b := range src {
		if b == 0 {
			end = i
			break
		}
	}
	return string(src[:end])
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
	case session.PhaseAuth:
		return 0x0A, nil
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
	case 0x0A:
		return session.PhaseAuth, nil
	default:
		return "", fmt.Errorf("%w: 0x%02x", ErrUnknownPhaseValue, value)
	}
}
