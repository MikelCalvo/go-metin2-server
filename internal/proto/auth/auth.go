package auth

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

const (
	HeaderLogin3      uint16 = 0x0102
	HeaderAuthSuccess uint16 = 0x0108

	loginFieldSize    = 31
	passwordFieldSize = 31
	login3PayloadSize = loginFieldSize + passwordFieldSize
)

var (
	ErrUnexpectedHeader = errors.New("unexpected auth packet header")
	ErrInvalidPayload   = errors.New("invalid auth packet payload")
	ErrStringTooLong    = errors.New("string does not fit fixed-width wire field")
)

type Login3Packet struct {
	Login    string
	Password string
}

type AuthSuccessPacket struct {
	LoginKey uint32
	Result   uint8
}

func EncodeLogin3(packet Login3Packet) ([]byte, error) {
	payload := make([]byte, login3PayloadSize)
	if err := putFixedString(payload[:loginFieldSize], packet.Login); err != nil {
		return nil, err
	}
	if err := putFixedString(payload[loginFieldSize:], packet.Password); err != nil {
		return nil, err
	}

	return frame.Encode(HeaderLogin3, payload), nil
}

func DecodeLogin3(f frame.Frame) (Login3Packet, error) {
	if f.Header != HeaderLogin3 {
		return Login3Packet{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != login3PayloadSize {
		return Login3Packet{}, ErrInvalidPayload
	}

	return Login3Packet{
		Login:    parseFixedString(f.Payload[:loginFieldSize]),
		Password: parseFixedString(f.Payload[loginFieldSize:]),
	}, nil
}

func EncodeAuthSuccess(packet AuthSuccessPacket) []byte {
	payload := make([]byte, 5)
	binary.LittleEndian.PutUint32(payload[:4], packet.LoginKey)
	payload[4] = packet.Result
	return frame.Encode(HeaderAuthSuccess, payload)
}

func DecodeAuthSuccess(f frame.Frame) (AuthSuccessPacket, error) {
	if f.Header != HeaderAuthSuccess {
		return AuthSuccessPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != 5 {
		return AuthSuccessPacket{}, ErrInvalidPayload
	}

	return AuthSuccessPacket{
		LoginKey: binary.LittleEndian.Uint32(f.Payload[:4]),
		Result:   f.Payload[4],
	}, nil
}

func putFixedString(dst []byte, value string) error {
	if len(value) >= len(dst) {
		return ErrStringTooLong
	}
	copy(dst, value)
	return nil
}

func parseFixedString(src []byte) string {
	if idx := bytes.IndexByte(src, 0); idx >= 0 {
		return string(src[:idx])
	}
	return string(src)
}
