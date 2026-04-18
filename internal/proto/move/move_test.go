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

func TestEncodeSyncPositionBuildsAClientFrame(t *testing.T) {
	packet := SyncPositionPacket{Elements: []SyncPositionElement{{VID: 0x01020304, X: 12345, Y: 23456}, {VID: 0x01020305, X: 34567, Y: 45678}}}
	want := expectedSyncPositionFrame(packet)
	got := EncodeSyncPosition(packet)
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected sync position frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeSyncPositionReturnsExpectedElements(t *testing.T) {
	packet, err := DecodeSyncPosition(decodeSingleFrame(t, expectedSyncPositionFrame(SyncPositionPacket{Elements: []SyncPositionElement{{VID: 0x01020304, X: 12345, Y: 23456}, {VID: 0x01020305, X: 34567, Y: 45678}}})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if len(packet.Elements) != 2 {
		t.Fatalf("expected 2 sync elements, got %d", len(packet.Elements))
	}
	if packet.Elements[0].VID != 0x01020304 || packet.Elements[0].X != 12345 || packet.Elements[0].Y != 23456 {
		t.Fatalf("unexpected first sync element: %+v", packet.Elements[0])
	}
	if packet.Elements[1].VID != 0x01020305 || packet.Elements[1].X != 34567 || packet.Elements[1].Y != 45678 {
		t.Fatalf("unexpected second sync element: %+v", packet.Elements[1])
	}
}

func TestEncodeSyncPositionAckBuildsAServerFrame(t *testing.T) {
	packet := SyncPositionAckPacket{Elements: []SyncPositionElement{{VID: 0x01020304, X: 12345, Y: 23456}}}
	want := expectedSyncPositionAckFrame(packet)
	got := EncodeSyncPositionAck(packet)
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected sync position ack frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeSyncPositionAckReturnsExpectedElements(t *testing.T) {
	packet, err := DecodeSyncPositionAck(decodeSingleFrame(t, expectedSyncPositionAckFrame(SyncPositionAckPacket{Elements: []SyncPositionElement{{VID: 0x01020304, X: 12345, Y: 23456}}})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if len(packet.Elements) != 1 {
		t.Fatalf("expected 1 sync ack element, got %d", len(packet.Elements))
	}
	if packet.Elements[0].VID != 0x01020304 || packet.Elements[0].X != 12345 || packet.Elements[0].Y != 23456 {
		t.Fatalf("unexpected sync ack element: %+v", packet.Elements[0])
	}
}

func TestDecodeSyncPositionRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeSyncPosition(frame.Frame{Header: HeaderSyncPositionAck, Length: 16, Payload: make([]byte, 12)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeSyncPositionRejectsPartialElements(t *testing.T) {
	_, err := DecodeSyncPosition(frame.Frame{Header: HeaderSyncPosition, Length: 17, Payload: make([]byte, 13)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
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

func expectedSyncPositionFrame(packet SyncPositionPacket) []byte {
	payload := make([]byte, len(packet.Elements)*12)
	offset := 0
	for _, element := range packet.Elements {
		binary.LittleEndian.PutUint32(payload[offset:], element.VID)
		offset += 4
		binary.LittleEndian.PutUint32(payload[offset:], uint32(element.X))
		offset += 4
		binary.LittleEndian.PutUint32(payload[offset:], uint32(element.Y))
		offset += 4
	}
	return frame.Encode(HeaderSyncPosition, payload)
}

func expectedSyncPositionAckFrame(packet SyncPositionAckPacket) []byte {
	payload := make([]byte, len(packet.Elements)*12)
	offset := 0
	for _, element := range packet.Elements {
		binary.LittleEndian.PutUint32(payload[offset:], element.VID)
		offset += 4
		binary.LittleEndian.PutUint32(payload[offset:], uint32(element.X))
		offset += 4
		binary.LittleEndian.PutUint32(payload[offset:], uint32(element.Y))
		offset += 4
	}
	return frame.Encode(HeaderSyncPositionAck, payload)
}
