package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
)

func TestGameRuntimeSafeboxItemMovePartialSplitPreservesInstanceSocketsAndAttributes(t *testing.T) {
	sourceActiveSockets := inventory.SocketValues{7, 0, 9}
	sourceActiveAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}
	templateSockets := itemcatalog.SocketValues{-11, 202, -303}
	templateAttributes := itemcatalog.AttributeValues{
		{Type: 12, Value: 34},
		{Type: 15, Value: -9},
	}

	cases := []struct {
		name             string
		sourceSockets    *inventory.SocketValues
		sourceAttributes *inventory.AttributeValues
		wantSockets      [itemproto.ItemSocketCount]int32
		wantAttrs        [itemproto.ItemAttributeCount]itemproto.Attribute
	}{
		{
			name:             "active instance presence wins over template",
			sourceSockets:    &sourceActiveSockets,
			sourceAttributes: &sourceActiveAttributes,
			wantSockets:      [itemproto.ItemSocketCount]int32{7, 0, 9},
			wantAttrs: [itemproto.ItemAttributeCount]itemproto.Attribute{
				{Type: 1, Value: 25},
				{Type: 7, Value: -3},
			},
		},
		{
			name:             "explicit zero instance presence wins over template",
			sourceSockets:    &zeroSockets,
			sourceAttributes: &zeroAttributes,
			wantSockets:      [itemproto.ItemSocketCount]int32{},
			wantAttrs:        [itemproto.ItemAttributeCount]itemproto.Attribute{},
		},
		{
			name:        "omitted instance keeps template fallback",
			wantSockets: [itemproto.ItemSocketCount]int32{-11, 202, -303},
			wantAttrs: [itemproto.ItemAttributeCount]itemproto.Attribute{
				{Type: 12, Value: 34},
				{Type: 15, Value: -9},
			},
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("SafeboxPartialSplitPresence", 0x01030c20+uint32(i), 0x02040c20+uint32(i), 1100, 2100, 0, 101, 201)
			owner.Gold = 1515
			owner.Inventory = []inventory.ItemInstance{
				{ID: 820, Vnum: 27001, Count: 5, Slot: 5, Sockets: tc.sourceSockets, Attributes: tc.sourceAttributes},
			}
			login := "sbx-pspl-pres-" + string(rune('a'+i))
			loginKey := uint32(0xb0c0c020 + i)
			issuePeerTicket(t, ticketStore, login, loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed safebox partial-split presence owner account: %v", err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
				Vnum:       27001,
				Name:       "Safebox Partial Split Presence Potion",
				Stackable:  true,
				MaxCount:   10,
				Sockets:    templateSockets,
				Attributes: templateAttributes,
			}})

			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected safebox partial-split presence runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
			defer closeSessionFlow(t, flow)
			_ = flushServerFrames(t, flow)

			if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
				Type:    chatproto.ChatTypeTalking,
				Message: "/open_safebox",
			}))); err != nil {
				t.Fatalf("unexpected /open_safebox before preserve partial-split error: %v", err)
			}
			if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
				SafeSlot: 0,
				Position: itemproto.InventoryPosition(5),
			}))); err != nil {
				t.Fatalf("unexpected safebox check-in before preserve partial-split error: %v", err)
			}

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{
				Source:      itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0},
				Destination: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 2},
				Count:       2,
			})))
			if err != nil {
				t.Fatalf("unexpected preserve partial-split safebox item-move error: %v", err)
			}
			if len(out) != 2 {
				t.Fatalf("expected preserve partial-split safebox item-move to emit two SAFEBOX_SET frames, got %d", len(out))
			}
			sourceSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[0]))
			if err != nil {
				t.Fatalf("decode preserve partial-split source SAFEBOX_SET: %v", err)
			}
			if sourceSet.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) || sourceSet.Vnum != 27001 || sourceSet.Count != 3 {
				t.Fatalf("unexpected preserve partial-split source SAFEBOX_SET identity/count: %+v", sourceSet)
			}
			if sourceSet.Sockets != tc.wantSockets {
				t.Fatalf("expected source SAFEBOX_SET sockets %+v, got %+v", tc.wantSockets, sourceSet.Sockets)
			}
			if sourceSet.Attributes != tc.wantAttrs {
				t.Fatalf("expected source SAFEBOX_SET attributes %+v, got %+v", tc.wantAttrs, sourceSet.Attributes)
			}
			destSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[1]))
			if err != nil {
				t.Fatalf("decode preserve partial-split destination SAFEBOX_SET: %v", err)
			}
			if destSet.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 2}) || destSet.Vnum != 27001 || destSet.Count != 2 {
				t.Fatalf("unexpected preserve partial-split destination SAFEBOX_SET identity/count: %+v", destSet)
			}
			if destSet.Sockets != tc.wantSockets {
				t.Fatalf("expected destination SAFEBOX_SET sockets %+v, got %+v", tc.wantSockets, destSet.Sockets)
			}
			if destSet.Attributes != tc.wantAttrs {
				t.Fatalf("expected destination SAFEBOX_SET attributes %+v, got %+v", tc.wantAttrs, destSet.Attributes)
			}

			snapshot, err := safeboxstore.LoadOrEmpty(runtime.safeboxStore)
			if err != nil {
				t.Fatalf("load preserve partial-split safebox FileStore: %v", err)
			}
			cells := safeboxstore.CharacterCells(snapshot, login, owner.ID)
			assertPersistedSafeboxPartialMergePresence(t, cells, 820, 0, 3, tc.sourceSockets, tc.sourceAttributes)
			assertPersistedSafeboxPartialMergePresence(t, cells, 821, 2, 2, tc.sourceSockets, tc.sourceAttributes)
			if cells[0].Sockets != nil && cells[2].Sockets != nil && cells[0].Sockets == cells[2].Sockets {
				t.Fatal("expected persisted remainder sockets pointer to be independent of destination")
			}
			if cells[0].Attributes != nil && cells[2].Attributes != nil && cells[0].Attributes == cells[2].Attributes {
				t.Fatal("expected persisted remainder attributes pointer to be independent of destination")
			}

			assertCloseSafeboxCommandChat(t, flow, "/close_safebox", "close-safebox after preserve partial-split")

			reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
				Type:    chatproto.ChatTypeTalking,
				Message: "/open_safebox",
			})))
			if err != nil {
				t.Fatalf("unexpected /open_safebox reopen after preserve partial-split error: %v", err)
			}
			if len(reopenOut) < 3 {
				t.Fatalf("expected reopen SAFEBOX_SIZE plus two SAFEBOX_SET rows after preserve partial-split, got %d", len(reopenOut))
			}
			reopenSource, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, reopenOut[1]))
			if err != nil {
				t.Fatalf("decode reopen source SAFEBOX_SET after preserve partial-split: %v", err)
			}
			reopenDestination, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, reopenOut[2]))
			if err != nil {
				t.Fatalf("decode reopen destination SAFEBOX_SET after preserve partial-split: %v", err)
			}
			if reopenSource.Position != sourceSet.Position || reopenSource.Vnum != sourceSet.Vnum || reopenSource.Count != sourceSet.Count {
				t.Fatalf("unexpected reopen source SAFEBOX_SET after preserve partial-split: %+v want %+v", reopenSource, sourceSet)
			}
			if reopenSource.Sockets != tc.wantSockets {
				t.Fatalf("unexpected reopen source SAFEBOX_SET sockets %+v want %+v", reopenSource.Sockets, tc.wantSockets)
			}
			if reopenSource.Attributes != tc.wantAttrs {
				t.Fatalf("unexpected reopen source SAFEBOX_SET attributes %+v want %+v", reopenSource.Attributes, tc.wantAttrs)
			}
			if reopenDestination.Position != destSet.Position || reopenDestination.Vnum != destSet.Vnum || reopenDestination.Count != destSet.Count {
				t.Fatalf("unexpected reopen destination SAFEBOX_SET after preserve partial-split: %+v want %+v", reopenDestination, destSet)
			}
			if reopenDestination.Sockets != tc.wantSockets {
				t.Fatalf("unexpected reopen destination SAFEBOX_SET sockets %+v want %+v", reopenDestination.Sockets, tc.wantSockets)
			}
			if reopenDestination.Attributes != tc.wantAttrs {
				t.Fatalf("unexpected reopen destination SAFEBOX_SET attributes %+v want %+v", reopenDestination.Attributes, tc.wantAttrs)
			}
		})
	}
}
