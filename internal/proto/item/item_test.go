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

func TestAntiFlagConstantsMatchLegacyBitPositions(t *testing.T) {
	if AntiFlagFemale != 1<<0 || AntiFlagMale != 1<<1 || AntiFlagWarrior != 1<<2 || AntiFlagAssassin != 1<<3 || AntiFlagSura != 1<<4 || AntiFlagShaman != 1<<5 {
		t.Fatalf("unexpected job/sex anti-flag bit positions")
	}
	if AntiFlagGet != 1<<6 || AntiFlagDrop != 1<<7 || AntiFlagSell != 1<<8 || AntiFlagGive != 1<<13 || AntiFlagStack != 1<<15 {
		t.Fatalf("unexpected item transfer anti-flag bit positions")
	}
	if AntiFlagSafebox != 1<<17 {
		t.Fatalf("unexpected safebox anti-flag bit position")
	}
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

func TestItemUseCarriedInventoryPositionBuildsTheInventoryWindowPosition(t *testing.T) {
	position, err := CarriedInventoryPosition(5)
	if err != nil {
		t.Fatalf("unexpected carried inventory position error: %v", err)
	}
	if position != (Position{WindowType: WindowInventory, Cell: 5}) {
		t.Fatalf("unexpected carried inventory position: %+v", position)
	}
}

func TestItemUseCarriedInventoryPositionRejectsOutOfRangeCell(t *testing.T) {
	_, err := CarriedInventoryPosition(InventoryMaxCell)
	if err == nil {
		t.Fatal("expected out-of-range carried inventory cell to fail")
	}
}

func TestEncodeClientUseBuildsAFrame(t *testing.T) {
	position, err := CarriedInventoryPosition(5)
	if err != nil {
		t.Fatalf("unexpected carried inventory position error: %v", err)
	}
	want := frame.Encode(0x0501, []byte{WindowInventory, 5, 0})
	got := EncodeClientUse(ClientUsePacket{Position: position})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected item use frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeClientUseReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeClientUse(decodeSingleFrame(t, frame.Encode(0x0501, []byte{WindowInventory, 5, 0})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (ClientUsePacket{Position: Position{WindowType: WindowInventory, Cell: 5}}) {
		t.Fatalf("unexpected item-use packet: %+v", packet)
	}
}

func TestEncodeClientUseToItemBuildsAFrame(t *testing.T) {
	source, err := CarriedInventoryPosition(5)
	if err != nil {
		t.Fatalf("unexpected source position error: %v", err)
	}
	target, err := CarriedInventoryPosition(6)
	if err != nil {
		t.Fatalf("unexpected target position error: %v", err)
	}
	want := frame.Encode(HeaderClientUseToItem, []byte{WindowInventory, 5, 0, WindowInventory, 6, 0})
	got := EncodeClientUseToItem(ClientUseToItemPacket{Source: source, Target: target})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected item use-to-item frame bytes: got %x want %x", got, want)
	}
}

func TestEncodeUseBuildsAServerFrame(t *testing.T) {
	want := frame.Encode(HeaderUse, []byte{
		WindowInventory, 5, 0,
		0x44, 0x33, 0x22, 0x11,
		0x88, 0x77, 0x66, 0x55,
		0xdd, 0xcc, 0xbb, 0xaa,
	})
	got := EncodeUse(UsePacket{Position: InventoryPosition(5), CharacterVID: 0x11223344, VictimVID: 0x55667788, Vnum: 0xaabbccdd})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected item use frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeUseReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeUse(decodeSingleFrame(t, frame.Encode(HeaderUse, []byte{
		WindowInventory, 5, 0,
		0x44, 0x33, 0x22, 0x11,
		0x88, 0x77, 0x66, 0x55,
		0xdd, 0xcc, 0xbb, 0xaa,
	})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	want := UsePacket{Position: InventoryPosition(5), CharacterVID: 0x11223344, VictimVID: 0x55667788, Vnum: 0xaabbccdd}
	if packet != want {
		t.Fatalf("unexpected item-use server packet: got %+v want %+v", packet, want)
	}
}

func TestDecodeUseRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeUse(frame.Frame{Header: HeaderUse + 1, Length: 19, Payload: make([]byte, usePayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeUseRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeUse(frame.Frame{Header: HeaderUse, Length: 18, Payload: make([]byte, usePayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeClientUseToItemReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeClientUseToItem(decodeSingleFrame(t, frame.Encode(HeaderClientUseToItem, []byte{WindowInventory, 5, 0, WindowInventory, 6, 0})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (ClientUseToItemPacket{Source: Position{WindowType: WindowInventory, Cell: 5}, Target: Position{WindowType: WindowInventory, Cell: 6}}) {
		t.Fatalf("unexpected item-use-to-item packet: %+v", packet)
	}
}

func TestDecodeClientUseToItemRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientUseToItem(frame.Frame{Header: HeaderClientUseToItem + 1, Length: 10, Payload: make([]byte, clientUseToItemPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientUseToItemRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeClientUseToItem(frame.Frame{Header: HeaderClientUseToItem, Length: 9, Payload: make([]byte, clientUseToItemPayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestEncodeClientMoveBuildsAFrame(t *testing.T) {
	from, err := CarriedInventoryPosition(5)
	if err != nil {
		t.Fatalf("unexpected source position error: %v", err)
	}
	to, err := CarriedInventoryPosition(6)
	if err != nil {
		t.Fatalf("unexpected target position error: %v", err)
	}
	want := frame.Encode(HeaderClientMove, []byte{WindowInventory, 5, 0, WindowInventory, 6, 0, 3})
	got := EncodeClientMove(ClientMovePacket{Source: from, Destination: to, Count: 3})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected item move frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeClientMoveReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeClientMove(decodeSingleFrame(t, frame.Encode(HeaderClientMove, []byte{WindowInventory, 5, 0, WindowInventory, 6, 0, 3})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (ClientMovePacket{Source: Position{WindowType: WindowInventory, Cell: 5}, Destination: Position{WindowType: WindowInventory, Cell: 6}, Count: 3}) {
		t.Fatalf("unexpected item-move packet: %+v", packet)
	}
}

func TestEncodeClientDropBuildsAFrame(t *testing.T) {
	position, err := CarriedInventoryPosition(5)
	if err != nil {
		t.Fatalf("unexpected carried inventory position error: %v", err)
	}
	want := frame.Encode(HeaderClientDrop, []byte{WindowInventory, 5, 0, 0x44, 0x33, 0x22, 0x11})
	got := EncodeClientDrop(ClientDropPacket{Position: position, Elk: 0x11223344})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected item drop frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeClientDropReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeClientDrop(decodeSingleFrame(t, frame.Encode(HeaderClientDrop, []byte{WindowInventory, 5, 0, 0x44, 0x33, 0x22, 0x11})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (ClientDropPacket{Position: Position{WindowType: WindowInventory, Cell: 5}, Elk: 0x11223344}) {
		t.Fatalf("unexpected item-drop packet: %+v", packet)
	}
}

func TestEncodeClientDrop2BuildsAFrame(t *testing.T) {
	position, err := CarriedInventoryPosition(5)
	if err != nil {
		t.Fatalf("unexpected carried inventory position error: %v", err)
	}
	want := frame.Encode(HeaderClientDrop2, []byte{WindowInventory, 5, 0, 0x44, 0x33, 0x22, 0x11, 7})
	got := EncodeClientDrop2(ClientDrop2Packet{Position: position, Gold: 0x11223344, Count: 7})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected item drop2 frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeClientDrop2ReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeClientDrop2(decodeSingleFrame(t, frame.Encode(HeaderClientDrop2, []byte{WindowInventory, 5, 0, 0x44, 0x33, 0x22, 0x11, 7})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (ClientDrop2Packet{Position: Position{WindowType: WindowInventory, Cell: 5}, Gold: 0x11223344, Count: 7}) {
		t.Fatalf("unexpected item-drop2 packet: %+v", packet)
	}
}

func TestEncodeClientPickupBuildsAFrame(t *testing.T) {
	want := frame.Encode(HeaderClientPickup, []byte{0x78, 0x56, 0x34, 0x12})
	got := EncodeClientPickup(ClientPickupPacket{VID: 0x12345678})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected item pickup frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeClientPickupReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeClientPickup(decodeSingleFrame(t, frame.Encode(HeaderClientPickup, []byte{0x78, 0x56, 0x34, 0x12})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (ClientPickupPacket{VID: 0x12345678}) {
		t.Fatalf("unexpected item-pickup packet: %+v", packet)
	}
}

func TestEncodeGroundAddBuildsAFrame(t *testing.T) {
	want := frame.Encode(HeaderGroundAdd, []byte{0x10, 0x27, 0, 0, 0x30, 0xf8, 0xff, 0xff, 0x40, 0x1f, 0, 0, 0x78, 0x56, 0x34, 0x12, 0x44, 0x33, 0x22, 0x11})
	got := EncodeGroundAdd(GroundAddPacket{VID: 0x12345678, Vnum: 0x11223344, X: 10000, Y: -2000, Z: 8000})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected item ground add frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeGroundAddReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeGroundAdd(decodeSingleFrame(t, frame.Encode(HeaderGroundAdd, []byte{0x10, 0x27, 0, 0, 0x30, 0xf8, 0xff, 0xff, 0x40, 0x1f, 0, 0, 0x78, 0x56, 0x34, 0x12, 0x44, 0x33, 0x22, 0x11})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (GroundAddPacket{VID: 0x12345678, Vnum: 0x11223344, X: 10000, Y: -2000, Z: 8000}) {
		t.Fatalf("unexpected item-ground-add packet: %+v", packet)
	}
}

func TestEncodeGroundDelBuildsAFrame(t *testing.T) {
	want := frame.Encode(HeaderGroundDel, []byte{0x78, 0x56, 0x34, 0x12})
	got := EncodeGroundDel(GroundDelPacket{VID: 0x12345678})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected item ground del frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeGroundDelReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeGroundDel(decodeSingleFrame(t, frame.Encode(HeaderGroundDel, []byte{0x78, 0x56, 0x34, 0x12})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (GroundDelPacket{VID: 0x12345678}) {
		t.Fatalf("unexpected item-ground-del packet: %+v", packet)
	}
}

func TestEncodeOwnershipBuildsAFrame(t *testing.T) {
	want := frame.Encode(HeaderOwnership, []byte{
		0x78, 0x56, 0x34, 0x12,
		'D', 'r', 'o', 'p', 'O', 'w', 'n', 'e', 'r', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	})
	got := EncodeOwnership(OwnershipPacket{VID: 0x12345678, OwnerName: "DropOwner"})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected item ownership frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeOwnershipReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeOwnership(decodeSingleFrame(t, EncodeOwnership(OwnershipPacket{VID: 0x12345678, OwnerName: "DropOwner"})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (OwnershipPacket{VID: 0x12345678, OwnerName: "DropOwner"}) {
		t.Fatalf("unexpected item ownership packet: %+v", packet)
	}
}

func TestEncodeGetBuildsANormalPickupNoticeFrame(t *testing.T) {
	want := frame.Encode(HeaderGet, []byte{
		0x44, 0x33, 0x22, 0x11,
		7,
		GetArgNormal,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	})
	got := EncodeGet(GetPacket{Vnum: 0x11223344, Count: 7})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected item get frame bytes: got %x want %x", got, want)
	}
}

func TestEncodeGetBuildsPartyDeliveryNoticeFrame(t *testing.T) {
	want := frame.Encode(HeaderGet, []byte{
		0x44, 0x33, 0x22, 0x11,
		2,
		GetArgDeliveredToPartyMember,
		'P', 'e', 'e', 'r', 'O', 'n', 'e', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	})
	got := EncodeGet(GetPacket{Vnum: 0x11223344, Count: 2, Arg: GetArgDeliveredToPartyMember, FromName: "PeerOne"})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected party delivery item get frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeGetReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeGet(decodeSingleFrame(t, EncodeGet(GetPacket{Vnum: 0x11223344, Count: 2, Arg: GetArgFromPartyMember, FromName: "PeerOne"})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (GetPacket{Vnum: 0x11223344, Count: 2, Arg: GetArgFromPartyMember, FromName: "PeerOne"}) {
		t.Fatalf("unexpected item-get packet: %+v", packet)
	}
}

func TestDecodeGetRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeGet(frame.Frame{Header: HeaderGet + 1, Length: 43, Payload: make([]byte, getPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeGetRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeGet(frame.Frame{Header: HeaderGet, Length: 42, Payload: make([]byte, getPayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
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

func TestEncodeUpdateBuildsACountRefreshFrame(t *testing.T) {
	want := frame.Encode(HeaderUpdate, []byte{WindowInventory, 5, 0, 9, 4, 3, 2, 1, 254, 255, 255, 255, 13, 12, 11, 10, 1, 52, 18, 2, 254, 255, 3, 0, 0, 4, 1, 0, 5, 0, 128, 6, 255, 127, 7, 46, 251})
	got := EncodeUpdate(sampleUpdatePacket())
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected item update frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeUpdateReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeUpdate(decodeSingleFrame(t, EncodeUpdate(sampleUpdatePacket())))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != sampleUpdatePacket() {
		t.Fatalf("unexpected item update packet: %+v", packet)
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

func TestDecodeUpdateRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeUpdate(frame.Frame{Header: HeaderUpdate + 1, Length: 41, Payload: make([]byte, updatePayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeUpdateRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeUpdate(frame.Frame{Header: HeaderUpdate, Length: 40, Payload: make([]byte, updatePayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeClientUseRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientUse(frame.Frame{Header: HeaderClientUse + 1, Length: 7, Payload: make([]byte, clientUsePayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientUseRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeClientUse(frame.Frame{Header: HeaderClientUse, Length: 6, Payload: make([]byte, clientUsePayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeClientMoveRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientMove(frame.Frame{Header: HeaderClientMove + 1, Length: 11, Payload: make([]byte, clientMovePayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientMoveRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeClientMove(frame.Frame{Header: HeaderClientMove, Length: 10, Payload: make([]byte, clientMovePayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeClientDropRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientDrop(frame.Frame{Header: HeaderClientDrop + 8, Length: 11, Payload: make([]byte, clientDropPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientDropRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeClientDrop(frame.Frame{Header: HeaderClientDrop, Length: 10, Payload: make([]byte, clientDropPayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeClientDrop2RejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientDrop2(frame.Frame{Header: HeaderClientDrop2 + 8, Length: 12, Payload: make([]byte, clientDrop2PayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientDrop2RejectsInvalidPayload(t *testing.T) {
	_, err := DecodeClientDrop2(frame.Frame{Header: HeaderClientDrop2, Length: 11, Payload: make([]byte, clientDrop2PayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeClientPickupRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientPickup(frame.Frame{Header: HeaderClientPickup + 1, Length: 8, Payload: make([]byte, clientPickupPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientPickupRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeClientPickup(frame.Frame{Header: HeaderClientPickup, Length: 7, Payload: make([]byte, clientPickupPayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeGroundAddRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeGroundAdd(frame.Frame{Header: HeaderGroundAdd + 1, Length: 24, Payload: make([]byte, groundAddPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeGroundAddRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeGroundAdd(frame.Frame{Header: HeaderGroundAdd, Length: 23, Payload: make([]byte, groundAddPayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeGroundDelRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeGroundDel(frame.Frame{Header: HeaderGroundDel + 1, Length: 8, Payload: make([]byte, groundDelPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeGroundDelRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeGroundDel(frame.Frame{Header: HeaderGroundDel, Length: 7, Payload: make([]byte, groundDelPayloadSize-1)})
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

func sampleUpdatePacket() UpdatePacket {
	return UpdatePacket{
		Position: Position{WindowType: WindowInventory, Cell: 5},
		Count:    9,
		Sockets:  [ItemSocketCount]int32{0x01020304, -2, 0x0A0B0C0D},
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
