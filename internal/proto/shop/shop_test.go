package shop

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
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

func TestEncodeClientBuyBuildsAFrameWithOpaqueLeadingByteAndCatalogSlot(t *testing.T) {
	want := loadHexFixture(t, "client-buy-frame.hex")
	got := EncodeClientBuy(sampleClientBuyPacket())
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected client shop buy frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeClientBuyReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeClientBuy(decodeSingleFrame(t, loadHexFixture(t, "client-buy-frame.hex")))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != sampleClientBuyPacket() {
		t.Fatalf("unexpected client shop buy packet: %+v", packet)
	}
}

func TestEncodeClientEndBuildsAFrame(t *testing.T) {
	want := loadHexFixture(t, "client-end-frame.hex")
	got := EncodeClientEnd()
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected client shop end frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeClientEndAcceptsTheExpectedSubheader(t *testing.T) {
	if err := DecodeClientEnd(decodeSingleFrame(t, loadHexFixture(t, "client-end-frame.hex"))); err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
}

func TestEncodeClientSellBuildsAFrame(t *testing.T) {
	want := loadHexFixture(t, "client-sell-frame.hex")
	got := EncodeClientSell(sampleClientSellPacket())
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected client shop sell frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeClientSellReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeClientSell(decodeSingleFrame(t, loadHexFixture(t, "client-sell-frame.hex")))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != sampleClientSellPacket() {
		t.Fatalf("unexpected client shop sell packet: %+v", packet)
	}
}

func TestEncodeClientSell2BuildsAFrame(t *testing.T) {
	want := loadHexFixture(t, "client-sell2-frame.hex")
	got := EncodeClientSell2(sampleClientSell2Packet())
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected client shop sell2 frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeClientSell2ReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeClientSell2(decodeSingleFrame(t, loadHexFixture(t, "client-sell2-frame.hex")))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != sampleClientSell2Packet() {
		t.Fatalf("unexpected client shop sell2 packet: %+v", packet)
	}
}

func TestEncodeServerStartBuildsAFrameFromTheSelectedBootstrapShape(t *testing.T) {
	want := loadHexFixture(t, "server-start-frame.hex")
	got := EncodeServerStart(sampleServerStartPacket())
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected server shop start frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeServerStartReturnsExpectedFieldsFromTheSelectedBootstrapShape(t *testing.T) {
	packet, err := DecodeServerStart(decodeSingleFrame(t, loadHexFixture(t, "server-start-frame.hex")))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != sampleServerStartPacket() {
		t.Fatalf("unexpected server shop start packet: %+v", packet)
	}
}

func TestEncodeServerUpdateItemBuildsAFrame(t *testing.T) {
	want := loadHexFixture(t, "server-update-item-frame.hex")
	got := EncodeServerUpdateItem(sampleServerUpdateItemPacket())
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected server shop update-item frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeServerUpdateItemReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeServerUpdateItem(decodeSingleFrame(t, loadHexFixture(t, "server-update-item-frame.hex")))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != sampleServerUpdateItemPacket() {
		t.Fatalf("unexpected server shop update-item packet: %+v", packet)
	}
}

func TestEncodeServerEndBuildsAFrame(t *testing.T) {
	want := loadHexFixture(t, "server-end-frame.hex")
	got := EncodeServerEnd()
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected server shop end frame bytes: got %x want %x", got, want)
	}
}

func TestEncodeServerOKBuildsAFrame(t *testing.T) {
	want := loadHexFixture(t, "server-ok-frame.hex")
	got := EncodeServerOK()
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected server shop ok frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeServerOKAcceptsTheExpectedSubheader(t *testing.T) {
	if err := DecodeServerOK(decodeSingleFrame(t, loadHexFixture(t, "server-ok-frame.hex"))); err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
}

func TestEncodeServerNotEnoughMoneyBuildsAFrame(t *testing.T) {
	want := loadHexFixture(t, "server-not-enough-money-frame.hex")
	got := EncodeServerNotEnoughMoney()
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected server shop not-enough-money frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeServerNotEnoughMoneyAcceptsTheExpectedSubheader(t *testing.T) {
	if err := DecodeServerNotEnoughMoney(decodeSingleFrame(t, loadHexFixture(t, "server-not-enough-money-frame.hex"))); err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
}

func TestEncodeServerInventoryFullBuildsAFrame(t *testing.T) {
	want := loadHexFixture(t, "server-inventory-full-frame.hex")
	got := EncodeServerInventoryFull()
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected server shop inventory-full frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeServerInventoryFullAcceptsTheExpectedSubheader(t *testing.T) {
	if err := DecodeServerInventoryFull(decodeSingleFrame(t, loadHexFixture(t, "server-inventory-full-frame.hex"))); err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
}

func TestEncodeServerInvalidPosBuildsAFrame(t *testing.T) {
	want := loadHexFixture(t, "server-invalid-pos-frame.hex")
	got := EncodeServerInvalidPos()
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected server shop invalid-pos frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeServerInvalidPosAcceptsTheExpectedSubheader(t *testing.T) {
	if err := DecodeServerInvalidPos(decodeSingleFrame(t, loadHexFixture(t, "server-invalid-pos-frame.hex"))); err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
}

func TestDecodeServerEndAcceptsTheExpectedSubheader(t *testing.T) {
	if err := DecodeServerEnd(decodeSingleFrame(t, loadHexFixture(t, "server-end-frame.hex"))); err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
}

func TestDecodeClientBuyRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientBuy(frame.Frame{Header: HeaderClientShop + 1, Length: 7, Payload: []byte{ClientSubheaderBuy, 1, 1}})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientBuyRejectsUnexpectedSubheader(t *testing.T) {
	_, err := DecodeClientBuy(frame.Frame{Header: HeaderClientShop, Length: 7, Payload: []byte{ClientSubheaderEnd, 1, 1}})
	if !errors.Is(err, ErrUnexpectedSubheader) {
		t.Fatalf("expected ErrUnexpectedSubheader, got %v", err)
	}
}

func TestDecodeClientBuyRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeClientBuy(frame.Frame{Header: HeaderClientShop, Length: 6, Payload: []byte{ClientSubheaderBuy, 1}})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeClientSellRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientSell(frame.Frame{Header: HeaderClientShop + 1, Length: 6, Payload: []byte{ClientSubheaderSell, 0}})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientSellRejectsUnexpectedSubheader(t *testing.T) {
	_, err := DecodeClientSell(frame.Frame{Header: HeaderClientShop, Length: 6, Payload: []byte{ClientSubheaderSell2, 0}})
	if !errors.Is(err, ErrUnexpectedSubheader) {
		t.Fatalf("expected ErrUnexpectedSubheader, got %v", err)
	}
}

func TestDecodeClientSellRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeClientSell(frame.Frame{Header: HeaderClientShop, Length: 5, Payload: []byte{ClientSubheaderSell}})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeClientSell2RejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientSell2(frame.Frame{Header: HeaderClientShop + 1, Length: 7, Payload: []byte{ClientSubheaderSell2, 0, 1}})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientSell2RejectsUnexpectedSubheader(t *testing.T) {
	_, err := DecodeClientSell2(frame.Frame{Header: HeaderClientShop, Length: 7, Payload: []byte{ClientSubheaderSell, 0, 1}})
	if !errors.Is(err, ErrUnexpectedSubheader) {
		t.Fatalf("expected ErrUnexpectedSubheader, got %v", err)
	}
}

func TestDecodeClientSell2RejectsInvalidPayload(t *testing.T) {
	_, err := DecodeClientSell2(frame.Frame{Header: HeaderClientShop, Length: 6, Payload: []byte{ClientSubheaderSell2, 0}})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeServerStartRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeServerStart(frame.Frame{Header: HeaderServerShop + 1, Length: 1729, Payload: make([]byte, serverStartPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeServerStartRejectsUnexpectedSubheader(t *testing.T) {
	payload := make([]byte, serverStartPayloadSize)
	payload[0] = ServerSubheaderEnd
	_, err := DecodeServerStart(frame.Frame{Header: HeaderServerShop, Length: 1729, Payload: payload})
	if !errors.Is(err, ErrUnexpectedSubheader) {
		t.Fatalf("expected ErrUnexpectedSubheader, got %v", err)
	}
}

func TestDecodeServerStartRejectsInvalidPayload(t *testing.T) {
	payload := make([]byte, serverStartPayloadSize-1)
	payload[0] = ServerSubheaderStart
	_, err := DecodeServerStart(frame.Frame{Header: HeaderServerShop, Length: 1728, Payload: payload})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeServerUpdateItemRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeServerUpdateItem(frame.Frame{Header: HeaderServerShop + 1, Length: 71, Payload: make([]byte, serverUpdateItemPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeServerUpdateItemRejectsUnexpectedSubheader(t *testing.T) {
	payload := make([]byte, serverUpdateItemPayloadSize)
	payload[0] = ServerSubheaderEnd
	_, err := DecodeServerUpdateItem(frame.Frame{Header: HeaderServerShop, Length: 71, Payload: payload})
	if !errors.Is(err, ErrUnexpectedSubheader) {
		t.Fatalf("expected ErrUnexpectedSubheader, got %v", err)
	}
}

func TestDecodeServerUpdateItemRejectsInvalidPayload(t *testing.T) {
	payload := make([]byte, serverUpdateItemPayloadSize-1)
	payload[0] = ServerSubheaderUpdateItem
	_, err := DecodeServerUpdateItem(frame.Frame{Header: HeaderServerShop, Length: 70, Payload: payload})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func sampleClientBuyPacket() ClientBuyPacket {
	return ClientBuyPacket{RawLeadingByte: 1, CatalogSlot: 1}
}

func sampleClientSellPacket() ClientSellPacket {
	return ClientSellPacket{Slot: 0}
}

func sampleClientSell2Packet() ClientSell2Packet {
	return ClientSell2Packet{Slot: 5, Count: 3}
}

func sampleServerStartPacket() ServerStartPacket {
	var items [ShopHostItemMax]ItemEntry
	items[0] = ItemEntry{
		Vnum:       0x11223344,
		Price:      50,
		Count:      1,
		DisplayPos: 0,
		Sockets:    [itemproto.ItemSocketCount]int32{0x01020304, -2, 0x0A0B0C0D},
		Attributes: [itemproto.ItemAttributeCount]itemproto.Attribute{
			{Type: 1, Value: 0x1234},
			{Type: 2, Value: -2},
			{Type: 3, Value: 0},
			{Type: 4, Value: 1},
			{Type: 5, Value: -32768},
			{Type: 6, Value: 32767},
			{Type: 7, Value: -1234},
		},
	}
	items[1] = ItemEntry{
		Vnum:       0xA1B2C3D4,
		Price:      500,
		Count:      2,
		DisplayPos: 1,
		Sockets:    [itemproto.ItemSocketCount]int32{11, 22, 33},
		Attributes: [itemproto.ItemAttributeCount]itemproto.Attribute{
			{Type: 10, Value: 100},
			{Type: 20, Value: 200},
			{Type: 30, Value: 300},
			{Type: 40, Value: 400},
			{Type: 50, Value: -500},
			{Type: 60, Value: -600},
			{Type: 70, Value: 700},
		},
	}
	return ServerStartPacket{OwnerVID: 0x02040107, Items: items}
}

func sampleServerUpdateItemPacket() ServerUpdateItemPacket {
	return ServerUpdateItemPacket{
		Position: 12,
		Item: ItemEntry{
			Vnum:       0x11223344,
			Price:      1000,
			Count:      5,
			DisplayPos: 12,
			Sockets:    [itemproto.ItemSocketCount]int32{1, 2, 3},
			Attributes: [itemproto.ItemAttributeCount]itemproto.Attribute{
				{Type: 1, Value: 10},
				{Type: 2, Value: 11},
				{Type: 3, Value: 12},
				{Type: 4, Value: 13},
				{Type: 5, Value: 14},
				{Type: 6, Value: 15},
				{Type: 7, Value: 16},
			},
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
