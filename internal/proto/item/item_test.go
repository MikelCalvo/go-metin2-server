package item

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

func loadHexFixture(t *testing.T, name string) []byte {
	t.Helper()

	path := filepath.Join("testdata", name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}

	decoded, err := hex.DecodeString(strings.TrimSpace(string(content)))
	if err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}

	return decoded
}

func TestEncodeSetBuildsAnInventoryFrame(t *testing.T) {
	want := loadHexFixture(t, "item-set-inventory-frame.hex")
	got := EncodeSet(sampleInventorySetPacket())
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected inventory item set frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeSetReturnsExpectedInventoryFields(t *testing.T) {
	packet, err := DecodeSet(decodeSingleFrame(t, loadHexFixture(t, "item-set-inventory-frame.hex")))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != sampleInventorySetPacket() {
		t.Fatalf("unexpected inventory item set packet: %+v", packet)
	}
}

func TestEncodeSetBuildsAnEquipmentFrameInTheLegacyCombinedCellNamespace(t *testing.T) {
	want := loadHexFixture(t, "item-set-equipment-frame.hex")
	got := EncodeSet(sampleEquipmentSetPacket())
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected equipment item set frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeSetReturnsExpectedEquipmentFieldsInTheLegacyCombinedCellNamespace(t *testing.T) {
	packet, err := DecodeSet(decodeSingleFrame(t, loadHexFixture(t, "item-set-equipment-frame.hex")))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != sampleEquipmentSetPacket() {
		t.Fatalf("unexpected equipment item set packet: %+v", packet)
	}
}

func TestEquipmentPositionBuildsTheLegacyCombinedInventoryCellNamespace(t *testing.T) {
	position, err := EquipmentPosition(4)
	if err != nil {
		t.Fatalf("unexpected equipment position error: %v", err)
	}
	if position != (Position{WindowType: WindowInventory, Cell: InventoryMaxCell + 4}) {
		t.Fatalf("unexpected equipment position: %+v", position)
	}
}

func TestEquipmentPositionRejectsOutOfRangeWearCell(t *testing.T) {
	_, err := EquipmentPosition(WearMaxCell)
	if err == nil {
		t.Fatal("expected out-of-range wear cell to fail")
	}
}

func TestEncodeDelBuildsAFrame(t *testing.T) {
	want := loadHexFixture(t, "item-del-frame.hex")
	got := EncodeDel(sampleDelPacket())
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected item del frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeDelReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeDel(decodeSingleFrame(t, loadHexFixture(t, "item-del-frame.hex")))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != sampleDelPacket() {
		t.Fatalf("unexpected item del packet: %+v", packet)
	}
}

func TestDecodeSetRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeSet(frame.Frame{Header: HeaderSet + 1, Length: 54, Payload: make([]byte, setPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeSetRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeSet(frame.Frame{Header: HeaderSet, Length: 53, Payload: make([]byte, setPayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeDelRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeDel(frame.Frame{Header: HeaderDel + 1, Length: 7, Payload: make([]byte, delPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeDelRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeDel(frame.Frame{Header: HeaderDel, Length: 6, Payload: make([]byte, delPayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func sampleInventorySetPacket() SetPacket {
	return SetPacket{
		Position:  Position{WindowType: WindowInventory, Cell: 7},
		Vnum:      0x11223344,
		Count:     17,
		Flags:     0x55667788,
		AntiFlags: 0x99AABBCC,
		Highlight: 0,
		Sockets:   [ItemSocketCount]int32{0x01020304, -2, 0x0A0B0C0D},
		Attributes: [ItemAttributeCount]Attribute{
			{Type: 1, Value: 0x1234},
			{Type: 2, Value: -2},
			{Type: 3, Value: 0},
			{Type: 4, Value: 1},
			{Type: 5, Value: -32768},
			{Type: 6, Value: 32767},
			{Type: 7, Value: -1234},
		},
	}
}

func sampleEquipmentSetPacket() SetPacket {
	return SetPacket{
		Position:  Position{WindowType: WindowInventory, Cell: 94},
		Vnum:      0xA1B2C3D4,
		Count:     1,
		Flags:     0,
		AntiFlags: 0x01020304,
		Highlight: 0,
		Sockets:   [ItemSocketCount]int32{11, 22, 33},
		Attributes: [ItemAttributeCount]Attribute{
			{Type: 10, Value: 100},
			{Type: 20, Value: 200},
			{Type: 30, Value: 300},
			{Type: 40, Value: 400},
			{Type: 50, Value: -500},
			{Type: 60, Value: -600},
			{Type: 70, Value: 700},
		},
	}
}

func sampleDelPacket() DelPacket {
	return DelPacket{Position: Position{WindowType: WindowInventory, Cell: 94}}
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
