package shop

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

func TestEncodeServerStartExBuildsAFrameWithTwoTabs(t *testing.T) {
	want := loadHexFixture(t, "server-start-ex-frame.hex")
	got := EncodeServerStartEx(sampleServerStartExPacket())
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected server shop start-ex frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeServerStartExReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeServerStartEx(decodeSingleFrame(t, loadHexFixture(t, "server-start-ex-frame.hex")))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	want := sampleServerStartExPacket()
	if packet.OwnerVID != want.OwnerVID {
		t.Fatalf("unexpected owner vid: got %#x want %#x", packet.OwnerVID, want.OwnerVID)
	}
	if len(packet.Tabs) != len(want.Tabs) {
		t.Fatalf("unexpected tab count: got %d want %d", len(packet.Tabs), len(want.Tabs))
	}
	for i := range packet.Tabs {
		if packet.Tabs[i] != want.Tabs[i] {
			t.Fatalf("unexpected tab %d: got %+v want %+v", i, packet.Tabs[i], want.Tabs[i])
		}
	}
}

func TestEncodeServerUpdateItemBuildsAFrame(t *testing.T) {
	want := loadHexFixture(t, "server-update-item-frame.hex")
	got := EncodeServerUpdateItem(sampleServerUpdateItemPacket())
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected server shop update-item frame bytes: got %x want %x", got, want)
	}
}

func TestEncodeServerUpdatePriceBuildsAFrame(t *testing.T) {
	want := loadHexFixture(t, "server-update-price-frame.hex")
	got := EncodeServerUpdatePrice(ServerUpdatePricePacket{ElkAmount: -12345})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected server shop update-price frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeServerUpdatePriceReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeServerUpdatePrice(decodeSingleFrame(t, loadHexFixture(t, "server-update-price-frame.hex")))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (ServerUpdatePricePacket{ElkAmount: -12345}) {
		t.Fatalf("unexpected server shop update-price packet: %+v", packet)
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

func TestEncodeServerSoldoutBuildsAFrame(t *testing.T) {
	want := loadHexFixture(t, "server-soldout-frame.hex")
	got := EncodeServerSoldout()
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected server shop soldout frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeServerSoldoutAcceptsTheExpectedSubheader(t *testing.T) {
	if err := DecodeServerSoldout(decodeSingleFrame(t, loadHexFixture(t, "server-soldout-frame.hex"))); err != nil {
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

func TestEncodeServerSoldOutBuildsAFrame(t *testing.T) {
	want := loadHexFixture(t, "server-sold-out-frame.hex")
	got := EncodeServerSoldOut()
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected server shop sold-out frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeServerSoldOutAcceptsTheExpectedSubheader(t *testing.T) {
	if err := DecodeServerSoldOut(decodeSingleFrame(t, loadHexFixture(t, "server-sold-out-frame.hex"))); err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
}

func TestEncodeServerNotEnoughMoneyExBuildsAFrame(t *testing.T) {
	want := loadHexFixture(t, "server-not-enough-money-ex-frame.hex")
	got := EncodeServerNotEnoughMoneyEx()
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected server shop not-enough-money-ex frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeServerNotEnoughMoneyExAcceptsTheExpectedSubheader(t *testing.T) {
	if err := DecodeServerNotEnoughMoneyEx(decodeSingleFrame(t, loadHexFixture(t, "server-not-enough-money-ex-frame.hex"))); err != nil {
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

func TestDecodeServerStartExRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeServerStartEx(frame.Frame{Header: HeaderServerShop + 1, Length: 13, Payload: []byte{ServerSubheaderStartEx, 0, 0, 0, 0, 0}})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeServerStartExRejectsUnexpectedSubheader(t *testing.T) {
	_, err := DecodeServerStartEx(frame.Frame{Header: HeaderServerShop, Length: 13, Payload: []byte{ServerSubheaderStart, 0, 0, 0, 0, 0}})
	if !errors.Is(err, ErrUnexpectedSubheader) {
		t.Fatalf("expected ErrUnexpectedSubheader, got %v", err)
	}
}

func TestDecodeServerStartExRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeServerStartEx(frame.Frame{Header: HeaderServerShop, Length: 13, Payload: []byte{ServerSubheaderStartEx, 0, 0, 0, 0, 1}})
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

func TestDecodeServerUpdatePriceRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeServerUpdatePrice(frame.Frame{Header: HeaderServerShop + 1, Length: 9, Payload: []byte{ServerSubheaderUpdatePrice, 0, 0, 0, 0}})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeServerUpdatePriceRejectsUnexpectedSubheader(t *testing.T) {
	_, err := DecodeServerUpdatePrice(frame.Frame{Header: HeaderServerShop, Length: 9, Payload: []byte{ServerSubheaderUpdateItem, 0, 0, 0, 0}})
	if !errors.Is(err, ErrUnexpectedSubheader) {
		t.Fatalf("expected ErrUnexpectedSubheader, got %v", err)
	}
}

func TestDecodeServerUpdatePriceRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeServerUpdatePrice(frame.Frame{Header: HeaderServerShop, Length: 8, Payload: []byte{ServerSubheaderUpdatePrice, 0, 0, 0}})
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

func TestEncodeClientMyShopBuildsAFrameWithSignAndItemTables(t *testing.T) {
	packet := sampleClientMyShopPacket()
	got := EncodeClientMyShop(packet)
	wantPayload := make([]byte, 34+MyShopItemTableSize)
	copy(wantPayload[:ShopSignMax+1], []byte(packet.Sign))
	wantPayload[ShopSignMax+1] = 1
	binary.LittleEndian.PutUint32(wantPayload[34:], packet.Items[0].Vnum)
	wantPayload[38] = packet.Items[0].Count
	wantPayload[39] = packet.Items[0].Position.WindowType
	binary.LittleEndian.PutUint16(wantPayload[40:], packet.Items[0].Position.Cell)
	binary.LittleEndian.PutUint32(wantPayload[42:], packet.Items[0].Price)
	wantPayload[46] = packet.Items[0].DisplayPos
	want := frame.Encode(HeaderClientMyShop, wantPayload)
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected client MYSHOP frame bytes:\n got %x\nwant %x", got, want)
	}
}

func TestDecodeClientMyShopReturnsExpectedFields(t *testing.T) {
	raw := EncodeClientMyShop(sampleClientMyShopPacket())
	packet, err := DecodeClientMyShop(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet.Sign != "Private Shop" {
		t.Fatalf("unexpected sign: %q", packet.Sign)
	}
	if len(packet.Items) != 1 {
		t.Fatalf("unexpected item count: %d", len(packet.Items))
	}
	got := packet.Items[0]
	want := sampleClientMyShopPacket().Items[0]
	if got != want {
		t.Fatalf("unexpected MYSHOP item: %+v want %+v", got, want)
	}
}

func TestEncodeClientMyShopAllowsEmptyItemTables(t *testing.T) {
	got := EncodeClientMyShop(ClientMyShopPacket{Sign: "Empty"})
	wantPayload := make([]byte, 34)
	copy(wantPayload[:ShopSignMax+1], []byte("Empty"))
	want := frame.Encode(HeaderClientMyShop, wantPayload)
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected empty MYSHOP frame bytes:\n got %x\nwant %x", got, want)
	}
}

func TestDecodeClientMyShopRoundTripsMultipleItemTables(t *testing.T) {
	want := ClientMyShopPacket{
		Sign: "Multi",
		Items: []ClientMyShopItem{
			{
				Vnum:       0x11223344,
				Count:      3,
				Position:   itemproto.Position{WindowType: itemproto.WindowInventory, Cell: 7},
				Price:      1500,
				DisplayPos: 2,
			},
			{
				Vnum:       0xaabbccdd,
				Count:      1,
				Position:   itemproto.Position{WindowType: itemproto.WindowInventory, Cell: 11},
				Price:      250,
				DisplayPos: 5,
			},
		},
	}
	packet, err := DecodeClientMyShop(decodeSingleFrame(t, EncodeClientMyShop(want)))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet.Sign != want.Sign {
		t.Fatalf("unexpected sign: %q", packet.Sign)
	}
	if len(packet.Items) != len(want.Items) {
		t.Fatalf("unexpected item count: %d", len(packet.Items))
	}
	for i := range want.Items {
		if packet.Items[i] != want.Items[i] {
			t.Fatalf("unexpected MYSHOP item[%d]: %+v want %+v", i, packet.Items[i], want.Items[i])
		}
	}
}

func TestDecodeClientMyShopRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientMyShop(frame.Frame{Header: HeaderClientShop, Length: 38, Payload: make([]byte, 34)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientMyShopRejectsTruncatedPayload(t *testing.T) {
	_, err := DecodeClientMyShop(frame.Frame{Header: HeaderClientMyShop, Length: 20, Payload: make([]byte, 16)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload for truncated payload, got %v", err)
	}
}

func TestDecodeClientMyShopRejectsCountLengthMismatch(t *testing.T) {
	payload := make([]byte, 34)
	payload[ShopSignMax+1] = 1 // claims one item but trailing blob missing
	_, err := DecodeClientMyShop(frame.Frame{Header: HeaderClientMyShop, Length: uint16(4 + len(payload)), Payload: payload})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload for count/length mismatch, got %v", err)
	}
}

func TestDecodeClientMyShopRejectsOversizedCount(t *testing.T) {
	payload := make([]byte, 34+MyShopItemTableSize)
	payload[ShopSignMax+1] = uint8(ShopHostItemMax + 1)
	_, err := DecodeClientMyShop(frame.Frame{Header: HeaderClientMyShop, Length: uint16(4 + len(payload)), Payload: payload})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload for oversized count, got %v", err)
	}
}

func TestEncodeServerShopSignBuildsAFrameWithVIDAndSign(t *testing.T) {
	packet := sampleServerShopSignPacket()
	got := EncodeServerShopSign(packet)
	wantPayload := make([]byte, 4+ShopSignMax+1)
	binary.LittleEndian.PutUint32(wantPayload, packet.VID)
	copy(wantPayload[4:], []byte(packet.Sign))
	want := frame.Encode(HeaderServerShopSign, wantPayload)
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected server SHOP_SIGN frame bytes:\n got %x\nwant %x", got, want)
	}
}

func TestDecodeServerShopSignReturnsExpectedFields(t *testing.T) {
	raw := EncodeServerShopSign(sampleServerShopSignPacket())
	packet, err := DecodeServerShopSign(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != sampleServerShopSignPacket() {
		t.Fatalf("unexpected SHOP_SIGN packet: %+v", packet)
	}
}

func TestEncodeServerShopSignAllowsEmptyClearSign(t *testing.T) {
	got := EncodeServerShopSign(ServerShopSignPacket{VID: 0x01020304, Sign: ""})
	wantPayload := make([]byte, 4+ShopSignMax+1)
	binary.LittleEndian.PutUint32(wantPayload, 0x01020304)
	want := frame.Encode(HeaderServerShopSign, wantPayload)
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected empty SHOP_SIGN frame bytes:\n got %x\nwant %x", got, want)
	}
}

func TestDecodeServerShopSignRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeServerShopSign(frame.Frame{Header: HeaderServerShop, Length: 41, Payload: make([]byte, 37)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeServerShopSignRejectsTruncatedPayload(t *testing.T) {
	_, err := DecodeServerShopSign(frame.Frame{Header: HeaderServerShopSign, Length: 20, Payload: make([]byte, 16)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload for truncated payload, got %v", err)
	}
}

func TestDecodeServerShopSignRejectsOversizedPayload(t *testing.T) {
	_, err := DecodeServerShopSign(frame.Frame{Header: HeaderServerShopSign, Length: 42, Payload: make([]byte, 38)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload for oversized payload, got %v", err)
	}
}

func sampleServerShopSignPacket() ServerShopSignPacket {
	return ServerShopSignPacket{VID: 0x11223344, Sign: "Private Shop"}
}

func sampleClientMyShopPacket() ClientMyShopPacket {
	return ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []ClientMyShopItem{{
			Vnum:       0x11223344,
			Count:      3,
			Position:   itemproto.Position{WindowType: itemproto.WindowInventory, Cell: 7},
			Price:      1500,
			DisplayPos: 2,
		}},
	}
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

func sampleServerStartExPacket() ServerStartExPacket {
	firstTab := ShopTab{Name: "Weapons", CoinType: 1}
	firstTab.Items[0] = ItemEntry{
		Vnum:       0x11223344,
		Price:      2,
		Count:      3,
		DisplayPos: 0,
		Sockets:    [itemproto.ItemSocketCount]int32{0x01020304, -1, 0},
		Attributes: [itemproto.ItemAttributeCount]itemproto.Attribute{
			{Type: 5, Value: 300},
			{Type: 0, Value: -2},
		},
	}

	secondTab := ShopTab{Name: "TabTwo", CoinType: 2}
	secondTab.Items[0] = ItemEntry{
		Vnum:       0xDEADBEEF,
		Price:      99,
		Count:      1,
		DisplayPos: 1,
		Sockets:    [itemproto.ItemSocketCount]int32{1, 2, 3},
		Attributes: [itemproto.ItemAttributeCount]itemproto.Attribute{
			{Type: 10, Value: 1},
			{Type: 11, Value: 2},
			{Type: 12, Value: 3},
			{Type: 13, Value: 4},
			{Type: 14, Value: 5},
			{Type: 15, Value: 6},
			{Type: 16, Value: 7},
		},
	}

	return ServerStartExPacket{OwnerVID: 0x55667788, Tabs: []ShopTab{firstTab, secondTab}}
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
