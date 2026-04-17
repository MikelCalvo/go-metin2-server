package move

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

func TestEncodeMoveBuildsAClientFrame(t *testing.T) {
	packet := MovePacket{Func: 1, Arg: 0, Rot: 12, X: 12345, Y: 23456, Time: 0x01020304}
	want := expectedMoveFrame(packet)
	got := EncodeMove(packet)
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected move frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeMoveReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeMove(decodeSingleFrame(t, expectedMoveFrame(MovePacket{Func: 1, Arg: 0, Rot: 12, X: 12345, Y: 23456, Time: 0x01020304})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet.Func != 1 || packet.Arg != 0 || packet.Rot != 12 || packet.X != 12345 || packet.Y != 23456 || packet.Time != 0x01020304 {
		t.Fatalf("unexpected move packet: %+v", packet)
	}
}

func TestEncodeMoveAckBuildsAServerFrame(t *testing.T) {
	packet := MoveAckPacket{Func: 1, Arg: 0, Rot: 12, VID: 0x01020304, X: 12345, Y: 23456, Time: 0x11121314, Duration: 250}
	want := expectedMoveAckFrame(packet)
	got := EncodeMoveAck(packet)
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected move ack frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeMoveAckReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeMoveAck(decodeSingleFrame(t, expectedMoveAckFrame(MoveAckPacket{Func: 1, Arg: 0, Rot: 12, VID: 0x01020304, X: 12345, Y: 23456, Time: 0x11121314, Duration: 250})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet.VID != 0x01020304 || packet.Duration != 250 || packet.X != 12345 || packet.Y != 23456 {
		t.Fatalf("unexpected move ack packet: %+v", packet)
	}
}

func TestDecodeMoveRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeMove(frame.Frame{Header: HeaderMoveAck, Length: 23, Payload: make([]byte, moveAckPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func decodeSingleFrame(t *testing.T, raw []byte) frame.Frame {
	t.Helper()
	decoder := frame.NewDecoder(4096)
	frames, err := decoder.Feed(raw)
	if err != nil {
		t.Fatalf("unexpected frame decode error: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}
	return frames[0]
}

func expectedMoveFrame(packet MovePacket) []byte {
	payload := make([]byte, movePayloadSize)
	payload[0] = packet.Func
	payload[1] = packet.Arg
	payload[2] = packet.Rot
	binary.LittleEndian.PutUint32(payload[3:], uint32(packet.X))
	binary.LittleEndian.PutUint32(payload[7:], uint32(packet.Y))
	binary.LittleEndian.PutUint32(payload[11:], packet.Time)
	return frame.Encode(HeaderMove, payload)
}

func expectedMoveAckFrame(packet MoveAckPacket) []byte {
	payload := make([]byte, moveAckPayloadSize)
	payload[0] = packet.Func
	payload[1] = packet.Arg
	payload[2] = packet.Rot
	binary.LittleEndian.PutUint32(payload[3:], packet.VID)
	binary.LittleEndian.PutUint32(payload[7:], uint32(packet.X))
	binary.LittleEndian.PutUint32(payload[11:], uint32(packet.Y))
	binary.LittleEndian.PutUint32(payload[15:], packet.Time)
	binary.LittleEndian.PutUint32(payload[19:], packet.Duration)
	return frame.Encode(HeaderMoveAck, payload)
}
