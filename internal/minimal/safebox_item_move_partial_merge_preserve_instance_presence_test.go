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

func TestGameRuntimeSafeboxItemMovePartialMergePreservesInstanceSocketsAndAttributes(t *testing.T) {
	sourceActiveSockets := inventory.SocketValues{11, 0, -3}
	sourceActiveAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	destActiveSockets := inventory.SocketValues{7, 0, 9}
	destActiveAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}
	templateSockets := itemcatalog.SocketValues{-11, 202, -303}
	templateAttributes := itemcatalog.AttributeValues{
		{Type: 12, Value: 34},
		{Type: 15, Value: -9},
	}

	cases := []struct {
		name              string
		sourceSockets     *inventory.SocketValues
		sourceAttributes  *inventory.AttributeValues
		destSockets       *inventory.SocketValues
		destAttributes    *inventory.AttributeValues
		wantSourceSockets [itemproto.ItemSocketCount]int32
		wantSourceAttrs   [itemproto.ItemAttributeCount]itemproto.Attribute
		wantDestSockets   [itemproto.ItemSocketCount]int32
		wantDestAttrs     [itemproto.ItemAttributeCount]itemproto.Attribute
	}{
		{
			name:              "active instance presence wins over template",
			sourceSockets:     &sourceActiveSockets,
			sourceAttributes:  &sourceActiveAttributes,
			destSockets:       &destActiveSockets,
			destAttributes:    &destActiveAttributes,
			wantSourceSockets: [itemproto.ItemSocketCount]int32{11, 0, -3},
			wantSourceAttrs: [itemproto.ItemAttributeCount]itemproto.Attribute{
				{Type: 4, Value: 55},
				{Type: 9, Value: -7},
			},
			wantDestSockets: [itemproto.ItemSocketCount]int32{7, 0, 9},
			wantDestAttrs: [itemproto.ItemAttributeCount]itemproto.Attribute{
				{Type: 1, Value: 25},
				{Type: 7, Value: -3},
			},
		},
		{
			name:              "explicit zero instance presence wins over template",
			sourceSockets:     &zeroSockets,
			sourceAttributes:  &zeroAttributes,
			destSockets:       &zeroSockets,
			destAttributes:    &zeroAttributes,
			wantSourceSockets: [itemproto.ItemSocketCount]int32{},
			wantSourceAttrs:   [itemproto.ItemAttributeCount]itemproto.Attribute{},
			wantDestSockets:   [itemproto.ItemSocketCount]int32{},
			wantDestAttrs:     [itemproto.ItemAttributeCount]itemproto.Attribute{},
		},
		{
			name:              "omitted instance keeps template fallback",
			wantSourceSockets: [itemproto.ItemSocketCount]int32{-11, 202, -303},
			wantSourceAttrs: [itemproto.ItemAttributeCount]itemproto.Attribute{
				{Type: 12, Value: 34},
				{Type: 15, Value: -9},
			},
			wantDestSockets: [itemproto.ItemSocketCount]int32{-11, 202, -303},
			wantDestAttrs: [itemproto.ItemAttributeCount]itemproto.Attribute{
				{Type: 12, Value: 34},
				{Type: 15, Value: -9},
			},
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("SafeboxPartialMergePresence", 0x01030c10+uint32(i), 0x02040c10+uint32(i), 1100, 2100, 0, 101, 201)
			owner.Gold = 1414
			owner.Inventory = []inventory.ItemInstance{
				{ID: 821, Vnum: 27001, Count: 4, Slot: 5, Sockets: tc.sourceSockets, Attributes: tc.sourceAttributes},
				{ID: 822, Vnum: 27001, Count: 3, Slot: 6, Sockets: tc.destSockets, Attributes: tc.destAttributes},
			}
			login := "sbx-pmrg-pres-" + string(rune('a'+i))
			loginKey := uint32(0xb0c0c010 + i)
			issuePeerTicket(t, ticketStore, login, loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed safebox partial-merge presence owner account: %v", err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
				Vnum:       27001,
				Name:       "Safebox Partial Merge Presence Potion",
				Stackable:  true,
				MaxCount:   10,
				Sockets:    templateSockets,
				Attributes: templateAttributes,
			}})

			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected safebox partial-merge presence runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
			defer closeSessionFlow(t, flow)
			_ = flushServerFrames(t, flow)

			if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
				Type:    chatproto.ChatTypeTalking,
				Message: "/open_safebox",
			}))); err != nil {
				t.Fatalf("unexpected /open_safebox before preserve partial-merge error: %v", err)
			}
			if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
				SafeSlot: 0,
				Position: itemproto.InventoryPosition(5),
			}))); err != nil {
				t.Fatalf("unexpected first safebox check-in before preserve partial-merge error: %v", err)
			}
			if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
				SafeSlot: 1,
				Position: itemproto.InventoryPosition(6),
			}))); err != nil {
				t.Fatalf("unexpected second safebox check-in before preserve partial-merge error: %v", err)
			}

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{
				Source:      itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0},
				Destination: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 1},
				Count:       2,
			})))
			if err != nil {
				t.Fatalf("unexpected preserve partial-merge safebox item-move error: %v", err)
			}
			if len(out) != 2 {
				t.Fatalf("expected preserve partial-merge safebox item-move to emit two SAFEBOX_SET frames, got %d", len(out))
			}
			sourceSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[0]))
			if err != nil {
				t.Fatalf("decode preserve partial-merge source SAFEBOX_SET: %v", err)
			}
			if sourceSet.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) || sourceSet.Vnum != 27001 || sourceSet.Count != 2 {
				t.Fatalf("unexpected preserve partial-merge source SAFEBOX_SET identity/count: %+v", sourceSet)
			}
			if sourceSet.Sockets != tc.wantSourceSockets {
				t.Fatalf("expected source SAFEBOX_SET sockets %+v, got %+v", tc.wantSourceSockets, sourceSet.Sockets)
			}
			if sourceSet.Attributes != tc.wantSourceAttrs {
				t.Fatalf("expected source SAFEBOX_SET attributes %+v, got %+v", tc.wantSourceAttrs, sourceSet.Attributes)
			}
			destSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[1]))
			if err != nil {
				t.Fatalf("decode preserve partial-merge destination SAFEBOX_SET: %v", err)
			}
			if destSet.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 1}) || destSet.Vnum != 27001 || destSet.Count != 5 {
				t.Fatalf("unexpected preserve partial-merge destination SAFEBOX_SET identity/count: %+v", destSet)
			}
			if destSet.Sockets != tc.wantDestSockets {
				t.Fatalf("expected destination SAFEBOX_SET sockets %+v, got %+v", tc.wantDestSockets, destSet.Sockets)
			}
			if destSet.Attributes != tc.wantDestAttrs {
				t.Fatalf("expected destination SAFEBOX_SET attributes %+v, got %+v", tc.wantDestAttrs, destSet.Attributes)
			}

			snapshot, err := safeboxstore.LoadOrEmpty(runtime.safeboxStore)
			if err != nil {
				t.Fatalf("load preserve partial-merge safebox FileStore: %v", err)
			}
			cells := safeboxstore.CharacterCells(snapshot, login, owner.ID)
			assertPersistedSafeboxPartialMergePresence(t, cells, 821, 0, 2, tc.sourceSockets, tc.sourceAttributes)
			assertPersistedSafeboxPartialMergePresence(t, cells, 822, 1, 5, tc.destSockets, tc.destAttributes)

			assertCloseSafeboxCommandChat(t, flow, "/close_safebox", "close-safebox after preserve partial-merge")

			reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
				Type:    chatproto.ChatTypeTalking,
				Message: "/open_safebox",
			})))
			if err != nil {
				t.Fatalf("unexpected /open_safebox reopen after preserve partial-merge error: %v", err)
			}
			if len(reopenOut) < 3 {
				t.Fatalf("expected reopen SAFEBOX_SIZE plus two SAFEBOX_SET rows after preserve partial-merge, got %d", len(reopenOut))
			}
			reopenSource, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, reopenOut[1]))
			if err != nil {
				t.Fatalf("decode reopen source SAFEBOX_SET after preserve partial-merge: %v", err)
			}
			reopenDestination, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, reopenOut[2]))
			if err != nil {
				t.Fatalf("decode reopen destination SAFEBOX_SET after preserve partial-merge: %v", err)
			}
			if reopenSource.Position != sourceSet.Position || reopenSource.Vnum != sourceSet.Vnum || reopenSource.Count != sourceSet.Count {
				t.Fatalf("unexpected reopen source SAFEBOX_SET after preserve partial-merge: %+v want %+v", reopenSource, sourceSet)
			}
			if reopenSource.Sockets != tc.wantSourceSockets {
				t.Fatalf("unexpected reopen source SAFEBOX_SET sockets %+v want %+v", reopenSource.Sockets, tc.wantSourceSockets)
			}
			if reopenSource.Attributes != tc.wantSourceAttrs {
				t.Fatalf("unexpected reopen source SAFEBOX_SET attributes %+v want %+v", reopenSource.Attributes, tc.wantSourceAttrs)
			}
			if reopenDestination.Position != destSet.Position || reopenDestination.Vnum != destSet.Vnum || reopenDestination.Count != destSet.Count {
				t.Fatalf("unexpected reopen destination SAFEBOX_SET after preserve partial-merge: %+v want %+v", reopenDestination, destSet)
			}
			if reopenDestination.Sockets != tc.wantDestSockets {
				t.Fatalf("unexpected reopen destination SAFEBOX_SET sockets %+v want %+v", reopenDestination.Sockets, tc.wantDestSockets)
			}
			if reopenDestination.Attributes != tc.wantDestAttrs {
				t.Fatalf("unexpected reopen destination SAFEBOX_SET attributes %+v want %+v", reopenDestination.Attributes, tc.wantDestAttrs)
			}
		})
	}
}

func assertPersistedSafeboxPartialMergePresence(
	t *testing.T,
	cells map[uint8]inventory.ItemInstance,
	id uint64,
	slot uint8,
	count uint16,
	wantSockets *inventory.SocketValues,
	wantAttributes *inventory.AttributeValues,
) {
	t.Helper()
	got, ok := cells[slot]
	if !ok || got.ID != id || got.Vnum != 27001 || got.Count != count || got.Slot != inventory.SlotIndex(slot) {
		t.Fatalf("unexpected persisted safebox cell slot=%d id=%d count=%d: %#v", slot, id, count, cells)
	}
	if (wantSockets != nil) != got.HasSockets() {
		t.Fatalf("persisted slot=%d HasSockets=%v want %v", slot, got.HasSockets(), wantSockets != nil)
	}
	if wantSockets != nil {
		if got.Sockets == nil || *got.Sockets != *wantSockets {
			t.Fatalf("expected persisted slot=%d sockets %+v, got %#v", slot, *wantSockets, got.Sockets)
		}
	} else if got.Sockets != nil {
		t.Fatalf("expected omitted persisted slot=%d sockets, got %#v", slot, got.Sockets)
	}
	if (wantAttributes != nil) != got.HasAttributes() {
		t.Fatalf("persisted slot=%d HasAttributes=%v want %v", slot, got.HasAttributes(), wantAttributes != nil)
	}
	if wantAttributes != nil {
		if got.Attributes == nil || *got.Attributes != *wantAttributes {
			t.Fatalf("expected persisted slot=%d attributes %+v, got %#v", slot, *wantAttributes, got.Attributes)
		}
	} else if got.Attributes != nil {
		t.Fatalf("expected omitted persisted slot=%d attributes, got %#v", slot, got.Attributes)
	}
}
