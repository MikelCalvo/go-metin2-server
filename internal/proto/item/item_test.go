package item

import (
	"bytes"
	"encoding/binary"
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

func TestExchangeItemMaxNumMatchesCurrentExchangeWindow(t *testing.T) {
	if ExchangeItemMaxNum != 12 {
		t.Fatalf("unexpected exchange item slot count: got %d", ExchangeItemMaxNum)
	}
}

func TestExchangeServerSubheadersMatchCurrentExchangeFamily(t *testing.T) {
	want := map[string]uint8{
		"start":     0,
		"item_add":  1,
		"item_del":  2,
		"gold_add":  3,
		"accept":    4,
		"end":       5,
		"already":   6,
		"less_gold": 7,
	}
	got := map[string]uint8{
		"start":     ExchangeServerSubheaderStart,
		"item_add":  ExchangeServerSubheaderItemAdd,
		"item_del":  ExchangeServerSubheaderItemDel,
		"gold_add":  ExchangeServerSubheaderGoldAdd,
		"accept":    ExchangeServerSubheaderAccept,
		"end":       ExchangeServerSubheaderEnd,
		"already":   ExchangeServerSubheaderAlready,
		"less_gold": ExchangeServerSubheaderLessGold,
	}
	for name, wantValue := range want {
		if gotValue := got[name]; gotValue != wantValue {
			t.Fatalf("unexpected exchange server subheader %s: got %d want %d", name, gotValue, wantValue)
		}
	}
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

func TestEncodeClientSafeboxCheckinBuildsAFrame(t *testing.T) {
	position, err := CarriedInventoryPosition(5)
	if err != nil {
		t.Fatalf("unexpected carried inventory position error: %v", err)
	}
	want := frame.Encode(HeaderClientSafeboxCheckin, []byte{7, WindowInventory, 5, 0})
	got := EncodeClientSafeboxCheckin(ClientSafeboxCheckinPacket{SafeSlot: 7, Position: position})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected safebox checkin frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeClientSafeboxCheckinReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeClientSafeboxCheckin(decodeSingleFrame(t, frame.Encode(HeaderClientSafeboxCheckin, []byte{7, WindowInventory, 5, 0})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (ClientSafeboxCheckinPacket{SafeSlot: 7, Position: InventoryPosition(5)}) {
		t.Fatalf("unexpected safebox checkin packet: %+v", packet)
	}
}

func TestEncodeClientSafeboxCheckoutBuildsAFrame(t *testing.T) {
	position, err := CarriedInventoryPosition(6)
	if err != nil {
		t.Fatalf("unexpected carried inventory position error: %v", err)
	}
	want := frame.Encode(HeaderClientSafeboxCheckout, []byte{8, WindowInventory, 6, 0})
	got := EncodeClientSafeboxCheckout(ClientSafeboxCheckoutPacket{SafeSlot: 8, Position: position})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected safebox checkout frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeClientSafeboxCheckoutReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeClientSafeboxCheckout(decodeSingleFrame(t, frame.Encode(HeaderClientSafeboxCheckout, []byte{8, WindowInventory, 6, 0})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (ClientSafeboxCheckoutPacket{SafeSlot: 8, Position: InventoryPosition(6)}) {
		t.Fatalf("unexpected safebox checkout packet: %+v", packet)
	}
}

func TestEncodeClientSafeboxItemMoveBuildsAFrame(t *testing.T) {
	want := frame.Encode(HeaderClientSafeboxItemMove, []byte{WindowInventory, 7, 0, WindowInventory, 8, 0, 3})
	got := EncodeClientSafeboxItemMove(ClientSafeboxItemMovePacket{Source: InventoryPosition(7), Destination: InventoryPosition(8), Count: 3})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected safebox item-move frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeClientSafeboxItemMoveReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeClientSafeboxItemMove(decodeSingleFrame(t, frame.Encode(HeaderClientSafeboxItemMove, []byte{WindowInventory, 7, 0, WindowInventory, 8, 0, 3})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (ClientSafeboxItemMovePacket{Source: InventoryPosition(7), Destination: InventoryPosition(8), Count: 3}) {
		t.Fatalf("unexpected safebox item-move packet: %+v", packet)
	}
}

func TestEncodeClientMallCheckoutBuildsAFrame(t *testing.T) {
	position, err := CarriedInventoryPosition(9)
	if err != nil {
		t.Fatalf("unexpected carried inventory position error: %v", err)
	}
	want := frame.Encode(HeaderClientMallCheckout, []byte{4, WindowInventory, 9, 0})
	got := EncodeClientMallCheckout(ClientMallCheckoutPacket{MallSlot: 4, Position: position})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected mall checkout frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeClientMallCheckoutReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeClientMallCheckout(decodeSingleFrame(t, frame.Encode(HeaderClientMallCheckout, []byte{4, WindowInventory, 9, 0})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (ClientMallCheckoutPacket{MallSlot: 4, Position: InventoryPosition(9)}) {
		t.Fatalf("unexpected mall checkout packet: %+v", packet)
	}
}

func TestDecodeClientStoragePacketsRejectUnexpectedHeader(t *testing.T) {
	cases := []struct {
		name   string
		decode func(frame.Frame) error
		header uint16
		size   int
	}{
		{name: "safebox checkin", decode: func(f frame.Frame) error { _, err := DecodeClientSafeboxCheckin(f); return err }, header: HeaderClientSafeboxCheckin, size: clientSafeboxTransferPayloadSize},
		{name: "safebox checkout", decode: func(f frame.Frame) error { _, err := DecodeClientSafeboxCheckout(f); return err }, header: HeaderClientSafeboxCheckout, size: clientSafeboxTransferPayloadSize},
		{name: "safebox item move", decode: func(f frame.Frame) error { _, err := DecodeClientSafeboxItemMove(f); return err }, header: HeaderClientSafeboxItemMove, size: clientSafeboxItemMovePayloadSize},
		{name: "mall checkout", decode: func(f frame.Frame) error { _, err := DecodeClientMallCheckout(f); return err }, header: HeaderClientMallCheckout, size: clientMallCheckoutPayloadSize},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.decode(frame.Frame{Header: tc.header + 1, Length: uint16(tc.size + 4), Payload: make([]byte, tc.size)})
			if !errors.Is(err, ErrUnexpectedHeader) {
				t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
			}
		})
	}
}

func TestDecodeClientStoragePacketsRejectInvalidPayload(t *testing.T) {
	cases := []struct {
		name   string
		decode func(frame.Frame) error
		header uint16
		size   int
	}{
		{name: "safebox checkin", decode: func(f frame.Frame) error { _, err := DecodeClientSafeboxCheckin(f); return err }, header: HeaderClientSafeboxCheckin, size: clientSafeboxTransferPayloadSize},
		{name: "safebox checkout", decode: func(f frame.Frame) error { _, err := DecodeClientSafeboxCheckout(f); return err }, header: HeaderClientSafeboxCheckout, size: clientSafeboxTransferPayloadSize},
		{name: "safebox item move", decode: func(f frame.Frame) error { _, err := DecodeClientSafeboxItemMove(f); return err }, header: HeaderClientSafeboxItemMove, size: clientSafeboxItemMovePayloadSize},
		{name: "mall checkout", decode: func(f frame.Frame) error { _, err := DecodeClientMallCheckout(f); return err }, header: HeaderClientMallCheckout, size: clientMallCheckoutPayloadSize},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.decode(frame.Frame{Header: tc.header, Length: uint16(tc.size + 3), Payload: make([]byte, tc.size-1)})
			if !errors.Is(err, ErrInvalidPayload) {
				t.Fatalf("expected ErrInvalidPayload, got %v", err)
			}
		})
	}
}

func TestEncodeSafeboxAndMallSetUseItemSetPayloadWithStorageHeaders(t *testing.T) {
	packet := sampleInventorySetPacket()
	setPayload := decodeSingleFrame(t, EncodeSet(packet)).Payload
	cases := []struct {
		name   string
		header uint16
		encode func(SetPacket) []byte
		decode func(frame.Frame) (SetPacket, error)
	}{
		{name: "safebox set", header: HeaderSafeboxSet, encode: EncodeSafeboxSet, decode: DecodeSafeboxSet},
		{name: "mall set", header: HeaderMallSet, encode: EncodeMallSet, decode: DecodeMallSet},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := frame.Encode(tc.header, setPayload)
			got := tc.encode(packet)
			if !bytes.Equal(got, want) {
				t.Fatalf("unexpected %s bytes: got %x want %x", tc.name, got, want)
			}
			decoded, err := tc.decode(decodeSingleFrame(t, got))
			if err != nil {
				t.Fatalf("decode %s: %v", tc.name, err)
			}
			if decoded != packet {
				t.Fatalf("unexpected %s packet: got %+v want %+v", tc.name, decoded, packet)
			}
		})
	}
}

func TestEncodeSafeboxAndMallDelUseItemDelPayloadWithStorageHeaders(t *testing.T) {
	packet := sampleDelPacket()
	delPayload := decodeSingleFrame(t, EncodeDel(packet)).Payload
	cases := []struct {
		name   string
		header uint16
		encode func(DelPacket) []byte
		decode func(frame.Frame) (DelPacket, error)
	}{
		{name: "safebox del", header: HeaderSafeboxDel, encode: EncodeSafeboxDel, decode: DecodeSafeboxDel},
		{name: "mall del", header: HeaderMallDel, encode: EncodeMallDel, decode: DecodeMallDel},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := frame.Encode(tc.header, delPayload)
			got := tc.encode(packet)
			if !bytes.Equal(got, want) {
				t.Fatalf("unexpected %s bytes: got %x want %x", tc.name, got, want)
			}
			decoded, err := tc.decode(decodeSingleFrame(t, got))
			if err != nil {
				t.Fatalf("decode %s: %v", tc.name, err)
			}
			if decoded != packet {
				t.Fatalf("unexpected %s packet: got %+v want %+v", tc.name, decoded, packet)
			}
		})
	}
}

func TestEncodeStorageStatusFramesBuildExpectedBytes(t *testing.T) {
	moneyPayload := make([]byte, 4)
	binary.LittleEndian.PutUint32(moneyPayload, uint32(^uint32(123455)))
	cases := []struct {
		name string
		got  []byte
		want []byte
	}{
		{name: "safebox wrong password", got: EncodeSafeboxWrongPassword(), want: frame.Encode(HeaderSafeboxWrongPassword, nil)},
		{name: "safebox size", got: EncodeSafeboxSize(SafeboxSizePacket{Size: 3}), want: frame.Encode(HeaderSafeboxSize, []byte{3})},
		{name: "safebox money change", got: EncodeSafeboxMoneyChange(SafeboxMoneyChangePacket{Money: -123456}), want: frame.Encode(HeaderSafeboxMoneyChange, moneyPayload)},
		{name: "mall open", got: EncodeMallOpen(MallOpenPacket{Size: 4}), want: frame.Encode(HeaderMallOpen, []byte{4})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !bytes.Equal(tc.got, tc.want) {
				t.Fatalf("unexpected %s bytes: got %x want %x", tc.name, tc.got, tc.want)
			}
		})
	}
}

func TestDecodeStorageStatusFramesReturnsExpectedFields(t *testing.T) {
	if _, err := DecodeSafeboxWrongPassword(decodeSingleFrame(t, EncodeSafeboxWrongPassword())); err != nil {
		t.Fatalf("decode safebox wrong-password: %v", err)
	}
	size, err := DecodeSafeboxSize(decodeSingleFrame(t, EncodeSafeboxSize(SafeboxSizePacket{Size: 3})))
	if err != nil {
		t.Fatalf("decode safebox size: %v", err)
	}
	if size != (SafeboxSizePacket{Size: 3}) {
		t.Fatalf("unexpected safebox size packet: %+v", size)
	}
	money, err := DecodeSafeboxMoneyChange(decodeSingleFrame(t, EncodeSafeboxMoneyChange(SafeboxMoneyChangePacket{Money: -123456})))
	if err != nil {
		t.Fatalf("decode safebox money change: %v", err)
	}
	if money != (SafeboxMoneyChangePacket{Money: -123456}) {
		t.Fatalf("unexpected safebox money packet: %+v", money)
	}
	mall, err := DecodeMallOpen(decodeSingleFrame(t, EncodeMallOpen(MallOpenPacket{Size: 4})))
	if err != nil {
		t.Fatalf("decode mall open: %v", err)
	}
	if mall != (MallOpenPacket{Size: 4}) {
		t.Fatalf("unexpected mall open packet: %+v", mall)
	}
}

func TestDecodeServerStoragePacketsRejectUnexpectedHeader(t *testing.T) {
	cases := []struct {
		name    string
		decode  func(frame.Frame) error
		header  uint16
		payload []byte
	}{
		{name: "safebox set", decode: func(f frame.Frame) error { _, err := DecodeSafeboxSet(f); return err }, header: HeaderSafeboxSet, payload: make([]byte, setPayloadSize)},
		{name: "safebox del", decode: func(f frame.Frame) error { _, err := DecodeSafeboxDel(f); return err }, header: HeaderSafeboxDel, payload: make([]byte, delPayloadSize)},
		{name: "safebox wrong password", decode: func(f frame.Frame) error { _, err := DecodeSafeboxWrongPassword(f); return err }, header: HeaderSafeboxWrongPassword, payload: nil},
		{name: "safebox size", decode: func(f frame.Frame) error { _, err := DecodeSafeboxSize(f); return err }, header: HeaderSafeboxSize, payload: make([]byte, safeboxSizePayloadSize)},
		{name: "safebox money change", decode: func(f frame.Frame) error { _, err := DecodeSafeboxMoneyChange(f); return err }, header: HeaderSafeboxMoneyChange, payload: make([]byte, safeboxMoneyChangePayloadSize)},
		{name: "mall open", decode: func(f frame.Frame) error { _, err := DecodeMallOpen(f); return err }, header: HeaderMallOpen, payload: make([]byte, mallOpenPayloadSize)},
		{name: "mall set", decode: func(f frame.Frame) error { _, err := DecodeMallSet(f); return err }, header: HeaderMallSet, payload: make([]byte, setPayloadSize)},
		{name: "mall del", decode: func(f frame.Frame) error { _, err := DecodeMallDel(f); return err }, header: HeaderMallDel, payload: make([]byte, delPayloadSize)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.decode(frame.Frame{Header: tc.header + 1, Length: uint16(len(tc.payload) + 4), Payload: tc.payload})
			if !errors.Is(err, ErrUnexpectedHeader) {
				t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
			}
		})
	}
}

func TestDecodeServerStoragePacketsRejectInvalidPayload(t *testing.T) {
	cases := []struct {
		name    string
		decode  func(frame.Frame) error
		header  uint16
		payload []byte
	}{
		{name: "safebox set", decode: func(f frame.Frame) error { _, err := DecodeSafeboxSet(f); return err }, header: HeaderSafeboxSet, payload: make([]byte, setPayloadSize-1)},
		{name: "safebox del", decode: func(f frame.Frame) error { _, err := DecodeSafeboxDel(f); return err }, header: HeaderSafeboxDel, payload: make([]byte, delPayloadSize-1)},
		{name: "safebox wrong password", decode: func(f frame.Frame) error { _, err := DecodeSafeboxWrongPassword(f); return err }, header: HeaderSafeboxWrongPassword, payload: []byte{0}},
		{name: "safebox size", decode: func(f frame.Frame) error { _, err := DecodeSafeboxSize(f); return err }, header: HeaderSafeboxSize, payload: nil},
		{name: "safebox money change", decode: func(f frame.Frame) error { _, err := DecodeSafeboxMoneyChange(f); return err }, header: HeaderSafeboxMoneyChange, payload: make([]byte, safeboxMoneyChangePayloadSize-1)},
		{name: "mall open", decode: func(f frame.Frame) error { _, err := DecodeMallOpen(f); return err }, header: HeaderMallOpen, payload: nil},
		{name: "mall set", decode: func(f frame.Frame) error { _, err := DecodeMallSet(f); return err }, header: HeaderMallSet, payload: make([]byte, setPayloadSize-1)},
		{name: "mall del", decode: func(f frame.Frame) error { _, err := DecodeMallDel(f); return err }, header: HeaderMallDel, payload: make([]byte, delPayloadSize-1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.decode(frame.Frame{Header: tc.header, Length: uint16(len(tc.payload) + 4), Payload: tc.payload})
			if !errors.Is(err, ErrInvalidPayload) {
				t.Fatalf("expected ErrInvalidPayload, got %v", err)
			}
		})
	}
}

func TestEncodeClientExchangeBuildsAFrame(t *testing.T) {
	position, err := CarriedInventoryPosition(5)
	if err != nil {
		t.Fatalf("unexpected carried inventory position error: %v", err)
	}
	want := frame.Encode(HeaderClientExchange, []byte{ExchangeSubheaderItemAdd, 0x44, 0x33, 0x22, 0x11, 7, WindowInventory, 5, 0})
	got := EncodeClientExchange(ClientExchangePacket{Subheader: ExchangeSubheaderItemAdd, Arg1: 0x11223344, Arg2: 7, Position: position})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected exchange frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeClientExchangeReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeClientExchange(decodeSingleFrame(t, frame.Encode(HeaderClientExchange, []byte{ExchangeSubheaderItemAdd, 0x44, 0x33, 0x22, 0x11, 7, WindowInventory, 5, 0})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	want := ClientExchangePacket{Subheader: ExchangeSubheaderItemAdd, Arg1: 0x11223344, Arg2: 7, Position: InventoryPosition(5)}
	if packet != want {
		t.Fatalf("unexpected exchange packet: got %+v want %+v", packet, want)
	}
}

func TestDecodeClientExchangeRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientExchange(frame.Frame{Header: HeaderClientExchange + 1, Length: 13, Payload: make([]byte, clientExchangePayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientExchangeRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeClientExchange(frame.Frame{Header: HeaderClientExchange, Length: 12, Payload: make([]byte, clientExchangePayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestEncodeServerExchangeBuildsAFrame(t *testing.T) {
	packet := ServerExchangePacket{
		Subheader:  ExchangeServerSubheaderItemAdd,
		IsMe:       1,
		Arg1:       0x11223344,
		Position:   Position{WindowType: WindowReserved, Cell: 7},
		Arg3:       0x55667788,
		Sockets:    [ItemSocketCount]int32{0x01020304, -2, 0},
		Attributes: [ItemAttributeCount]Attribute{{Type: 3, Value: 300}, {Type: 4, Value: -20}},
	}
	want := frame.Encode(HeaderServerExchange, []byte{
		ExchangeServerSubheaderItemAdd, 1,
		0x44, 0x33, 0x22, 0x11,
		WindowReserved, 7, 0,
		0x88, 0x77, 0x66, 0x55,
		0x04, 0x03, 0x02, 0x01,
		0xfe, 0xff, 0xff, 0xff,
		0x00, 0x00, 0x00, 0x00,
		3, 0x2c, 0x01,
		4, 0xec, 0xff,
		0, 0, 0,
		0, 0, 0,
		0, 0, 0,
		0, 0, 0,
		0, 0, 0,
	})
	got := EncodeServerExchange(packet)
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected server exchange frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeServerExchangeReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeServerExchange(decodeSingleFrame(t, frame.Encode(HeaderServerExchange, []byte{
		ExchangeServerSubheaderItemAdd, 1,
		0x44, 0x33, 0x22, 0x11,
		WindowReserved, 7, 0,
		0x88, 0x77, 0x66, 0x55,
		0x04, 0x03, 0x02, 0x01,
		0xfe, 0xff, 0xff, 0xff,
		0x00, 0x00, 0x00, 0x00,
		3, 0x2c, 0x01,
		4, 0xec, 0xff,
		0, 0, 0,
		0, 0, 0,
		0, 0, 0,
		0, 0, 0,
		0, 0, 0,
	})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	want := ServerExchangePacket{
		Subheader:  ExchangeServerSubheaderItemAdd,
		IsMe:       1,
		Arg1:       0x11223344,
		Position:   Position{WindowType: WindowReserved, Cell: 7},
		Arg3:       0x55667788,
		Sockets:    [ItemSocketCount]int32{0x01020304, -2, 0},
		Attributes: [ItemAttributeCount]Attribute{{Type: 3, Value: 300}, {Type: 4, Value: -20}},
	}
	if packet != want {
		t.Fatalf("unexpected server exchange packet: got %+v want %+v", packet, want)
	}
}

func TestDecodeServerExchangeRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeServerExchange(frame.Frame{Header: HeaderServerExchange + 1, Length: 50, Payload: make([]byte, serverExchangePayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeServerExchangeRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeServerExchange(frame.Frame{Header: HeaderServerExchange, Length: 49, Payload: make([]byte, serverExchangePayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
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

func TestEncodeClientGiveBuildsAFrame(t *testing.T) {
	position, err := CarriedInventoryPosition(5)
	if err != nil {
		t.Fatalf("unexpected carried inventory position error: %v", err)
	}
	want := frame.Encode(HeaderClientGive, []byte{0x78, 0x56, 0x34, 0x12, WindowInventory, 5, 0, 7})
	got := EncodeClientGive(ClientGivePacket{TargetVID: 0x12345678, Position: position, Count: 7})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected item give frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeClientGiveReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeClientGive(decodeSingleFrame(t, frame.Encode(HeaderClientGive, []byte{0x78, 0x56, 0x34, 0x12, WindowInventory, 5, 0, 7})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (ClientGivePacket{TargetVID: 0x12345678, Position: Position{WindowType: WindowInventory, Cell: 5}, Count: 7}) {
		t.Fatalf("unexpected item-give packet: %+v", packet)
	}
}

func TestDecodeClientGiveRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientGive(frame.Frame{Header: HeaderClientGive + 1, Length: 12, Payload: make([]byte, clientGivePayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientGiveRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeClientGive(frame.Frame{Header: HeaderClientGive, Length: 11, Payload: make([]byte, clientGivePayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestEncodeClientRefineBuildsAFrame(t *testing.T) {
	want := frame.Encode(HeaderClientRefine, []byte{5, 2})
	got := EncodeClientRefine(ClientRefinePacket{Position: 5, Type: 2})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected item refine frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeClientRefineReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeClientRefine(decodeSingleFrame(t, frame.Encode(HeaderClientRefine, []byte{5, 2})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (ClientRefinePacket{Position: 5, Type: 2}) {
		t.Fatalf("unexpected item-refine packet: %+v", packet)
	}
}

func TestDecodeClientRefineRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientRefine(frame.Frame{Header: HeaderClientRefine + 1, Length: 6, Payload: make([]byte, clientRefinePayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientRefineRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeClientRefine(frame.Frame{Header: HeaderClientRefine, Length: 5, Payload: make([]byte, clientRefinePayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestEncodeRefineInformationBuildsAFrame(t *testing.T) {
	want := frame.Encode(HeaderRefineInformation, sampleRefineInformationPayload())
	got, err := EncodeRefineInformation(sampleRefineInformationPacket())
	if err != nil {
		t.Fatalf("unexpected refine-information encode error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected refine-information frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeRefineInformationReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeRefineInformation(decodeSingleFrame(t, frame.Encode(HeaderRefineInformation, sampleRefineInformationPayload())))
	if err != nil {
		t.Fatalf("unexpected refine-information decode error: %v", err)
	}
	if packet != sampleRefineInformationPacket() {
		t.Fatalf("unexpected refine-information packet: %+v", packet)
	}
}

func TestEncodeRefineInformationNewBuildsAFrame(t *testing.T) {
	want := frame.Encode(HeaderRefineInformationNew, sampleRefineInformationPayload())
	got, err := EncodeRefineInformationNew(sampleRefineInformationPacket())
	if err != nil {
		t.Fatalf("unexpected refine-information-new encode error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected refine-information-new frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeRefineInformationNewReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeRefineInformationNew(decodeSingleFrame(t, frame.Encode(HeaderRefineInformationNew, sampleRefineInformationPayload())))
	if err != nil {
		t.Fatalf("unexpected refine-information-new decode error: %v", err)
	}
	if packet != sampleRefineInformationPacket() {
		t.Fatalf("unexpected refine-information-new packet: %+v", packet)
	}
}

func TestEncodeRefineInformationRejectsMaterialCountBeyondFixedTable(t *testing.T) {
	packet := sampleRefineInformationPacket()
	packet.Table.MaterialCount = RefineMaterialMaxNum + 1
	got, err := EncodeRefineInformation(packet)
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got raw=%x err=%v", got, err)
	}
}

func TestDecodeRefineInformationRejectsMaterialCountBeyondFixedTable(t *testing.T) {
	payload := sampleRefineInformationPayload()
	payload[10] = RefineMaterialMaxNum + 1
	_, err := DecodeRefineInformation(frame.Frame{Header: HeaderRefineInformation, Length: uint16(4 + len(payload)), Payload: payload})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeRefineInformationNewRejectsMaterialCountBeyondFixedTable(t *testing.T) {
	payload := sampleRefineInformationPayload()
	payload[10] = RefineMaterialMaxNum + 1
	_, err := DecodeRefineInformationNew(frame.Frame{Header: HeaderRefineInformationNew, Length: uint16(4 + len(payload)), Payload: payload})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeRefineInformationRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeRefineInformation(frame.Frame{Header: HeaderRefineInformation + 8, Length: 63, Payload: make([]byte, refineInformationPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeRefineInformationRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeRefineInformation(frame.Frame{Header: HeaderRefineInformation, Length: 62, Payload: make([]byte, refineInformationPayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeRefineInformationNewRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeRefineInformationNew(frame.Frame{Header: HeaderRefineInformationNew + 8, Length: 63, Payload: make([]byte, refineInformationPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeRefineInformationNewRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeRefineInformationNew(frame.Frame{Header: HeaderRefineInformationNew, Length: 62, Payload: make([]byte, refineInformationPayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
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

func sampleRefineInformationPacket() RefineInformationPacket {
	return RefineInformationPacket{
		Type:     3,
		Position: 5,
		Table: RefineTable{
			SourceVnum:    11200,
			ResultVnum:    11201,
			MaterialCount: 2,
			Cost:          12345,
			Probability:   87,
			Materials:     [RefineMaterialMaxNum]RefineMaterial{{Vnum: 27001, Count: 2}, {Vnum: 27002, Count: 3}},
		},
	}
}

func sampleRefineInformationPayload() []byte {
	return []byte{
		3,
		5,
		0xc0, 0x2b, 0x00, 0x00,
		0xc1, 0x2b, 0x00, 0x00,
		2,
		0x39, 0x30, 0x00, 0x00,
		0x57, 0x00, 0x00, 0x00,
		0x79, 0x69, 0x00, 0x00,
		0x02, 0x00, 0x00, 0x00,
		0x7a, 0x69, 0x00, 0x00,
		0x03, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
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
