package minimal

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
)

func TestGameRuntimeItemGivePlacesWholeStackOnVisiblePeer(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GiveRuntimeOwner", 0x01030d80, 0x02040d80, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1808, Vnum: 27001, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("GiveRuntimePeer", 0x01030d81, 0x02040d81, 1120, 2120, 0, 101, 201)
	peer.Inventory = []inventory.ItemInstance{{ID: 1809, Vnum: 27002, Count: 1, Slot: 6}}
	issuePeerTicket(t, ticketStore, "item-give-runtime-owner", 0x70707d80, owner)
	issuePeerTicket(t, ticketStore, "item-give-runtime-peer", 0x70707d81, peer)
	if err := accounts.Save(accountstore.Account{Login: "item-give-runtime-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-give runtime owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-give-runtime-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed item-give runtime peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected item-give runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-runtime-owner", 0x70707d80)
	defer closeSessionFlow(t, flow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-runtime-peer", 0x70707d81)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, flow)
	_ = flushServerFrames(t, peerFlow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{TargetVID: peer.VID, Position: itemproto.InventoryPosition(5), Count: 3})))
	if err != nil {
		t.Fatalf("unexpected accepted item-give packet error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected accepted ITEM_GIVE to emit ITEM_DEL plus QUICKSLOT_DEL, got %d", len(out))
	}
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode giver ITEM_DEL: %v", err)
	}
	if del.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected giver ITEM_DEL: %+v", del.Position)
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode giver QUICKSLOT_DEL: %v", err)
	}
	if quickslotDel.Position != 2 {
		t.Fatalf("unexpected giver QUICKSLOT_DEL: %+v", quickslotDel)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected giver to queue no extra frames after accepted ITEM_GIVE, got %d", len(queued))
	}
	queued := flushServerFrames(t, peerFlow)
	if len(queued) != 2 {
		t.Fatalf("expected recipient to receive ITEM_SET plus ITEM_GET, got %d", len(queued))
	}
	set, err := itemproto.DecodeSet(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode recipient ITEM_SET: %v", err)
	}
	if set.Position != itemproto.InventoryPosition(5) || set.Vnum != 27001 || set.Count != 3 {
		t.Fatalf("unexpected recipient ITEM_SET: %+v", set)
	}
	got, err := itemproto.DecodeGet(decodeSingleFrame(t, queued[1]))
	if err != nil {
		t.Fatalf("decode recipient ITEM_GET: %v", err)
	}
	if got.Vnum != 27001 || got.Count != 3 || got.Arg != itemproto.GetArgNormal || got.FromName != "" {
		t.Fatalf("unexpected recipient ITEM_GET: %+v", got)
	}

	wantOwner := owner
	wantOwner.Inventory = nil
	wantOwner.Quickslots = nil
	wantPeer := peer
	wantPeer.Inventory = []inventory.ItemInstance{
		{ID: 1808, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 1809, Vnum: 27002, Count: 1, Slot: 6},
	}
	assertExchangeAccountUnchanged(t, accounts, "item-give-runtime-owner", wantOwner, "accepted ITEM_GIVE owner")
	assertExchangeAccountUnchanged(t, accounts, "item-give-runtime-peer", wantPeer, "accepted ITEM_GIVE peer")
	assertExchangeLiveStateUnchanged(t, runtime, wantOwner, "accepted ITEM_GIVE live owner")
	assertExchangeLiveStateUnchanged(t, runtime, wantPeer, "accepted ITEM_GIVE live peer")
}

func TestGameRuntimeItemGivePlacesPartialStackOnVisiblePeer(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GivePartialOwner", 0x01030da0, 0x02040da0, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1808, Vnum: 27001, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	peer := peerVisibilityCharacter("GivePartialPeer", 0x01030da1, 0x02040da1, 1120, 2120, 0, 101, 201)
	peer.Inventory = []inventory.ItemInstance{{ID: 1809, Vnum: 27002, Count: 1, Slot: 6}}
	issuePeerTicket(t, ticketStore, "item-give-partial-owner", 0x70707da0, owner)
	issuePeerTicket(t, ticketStore, "item-give-partial-peer", 0x70707da1, peer)
	if err := accounts.Save(accountstore.Account{Login: "item-give-partial-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partial item-give owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-give-partial-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed partial item-give peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected partial item-give runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-partial-owner", 0x70707da0)
	defer closeSessionFlow(t, flow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-partial-peer", 0x70707da1)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, flow)
	_ = flushServerFrames(t, peerFlow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{TargetVID: peer.VID, Position: itemproto.InventoryPosition(5), Count: 1})))
	if err != nil {
		t.Fatalf("unexpected accepted partial item-give packet error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected accepted partial ITEM_GIVE to emit remainder ITEM_UPDATE without QUICKSLOT_DEL, got %d", len(out))
	}
	update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode giver remainder ITEM_UPDATE: %v", err)
	}
	if update.Position != itemproto.InventoryPosition(5) || update.Count != 2 {
		t.Fatalf("unexpected giver remainder ITEM_UPDATE: %+v", update)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected giver to queue no extra frames after accepted partial ITEM_GIVE, got %d", len(queued))
	}
	queued := flushServerFrames(t, peerFlow)
	if len(queued) != 2 {
		t.Fatalf("expected recipient to receive ITEM_SET plus ITEM_GET, got %d", len(queued))
	}
	set, err := itemproto.DecodeSet(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode recipient ITEM_SET: %v", err)
	}
	if set.Position != itemproto.InventoryPosition(5) || set.Vnum != 27001 || set.Count != 1 {
		t.Fatalf("unexpected recipient ITEM_SET: %+v", set)
	}
	got, err := itemproto.DecodeGet(decodeSingleFrame(t, queued[1]))
	if err != nil {
		t.Fatalf("decode recipient ITEM_GET: %v", err)
	}
	if got.Vnum != 27001 || got.Count != 1 || got.Arg != itemproto.GetArgNormal || got.FromName != "" {
		t.Fatalf("unexpected recipient ITEM_GET: %+v", got)
	}

	ownerAccount, err := accounts.Load("item-give-partial-owner")
	if err != nil {
		t.Fatalf("load partial item-give owner account: %v", err)
	}
	persistedOwner := findPersistedCharacter(t, ownerAccount, owner.Name)
	if !reflect.DeepEqual(persistedOwner.Inventory, []inventory.ItemInstance{{ID: 1808, Vnum: 27001, Count: 2, Slot: 5}}) {
		t.Fatalf("expected partial ITEM_GIVE to persist giver remainder, got %+v", persistedOwner.Inventory)
	}
	if !reflect.DeepEqual(persistedOwner.Quickslots, owner.Quickslots) {
		t.Fatalf("expected partial ITEM_GIVE to keep source item quickslots on the still-occupied cell, got %+v", persistedOwner.Quickslots)
	}
	peerAccount, err := accounts.Load("item-give-partial-peer")
	if err != nil {
		t.Fatalf("load partial item-give peer account: %v", err)
	}
	persistedPeer := findPersistedCharacter(t, peerAccount, peer.Name)
	if len(persistedPeer.Inventory) != 2 {
		t.Fatalf("unexpected partial ITEM_GIVE peer inventory: %+v", persistedPeer.Inventory)
	}
	var gotGiven, gotKept inventory.ItemInstance
	for _, item := range persistedPeer.Inventory {
		switch item.Vnum {
		case 27001:
			gotGiven = item
		case 27002:
			gotKept = item
		}
	}
	if gotGiven.ID == 0 || gotGiven.ID == 1808 || gotGiven.ID == 1809 || gotGiven.Count != 1 || gotGiven.Slot != 5 {
		t.Fatalf("expected fresh transferred identity distinct from remainder 1808 and peer 1809, got %+v", gotGiven)
	}
	if gotKept.ID != 1809 || gotKept.Count != 1 || gotKept.Slot != 6 {
		t.Fatalf("unexpected peer remaining stack: %+v", gotKept)
	}
	wantOwner := owner
	wantOwner.Inventory = []inventory.ItemInstance{{ID: 1808, Vnum: 27001, Count: 2, Slot: 5}}
	wantPeer := peer
	wantPeer.Inventory = []inventory.ItemInstance{gotGiven, gotKept}
	assertExchangeLiveStateUnchanged(t, runtime, wantOwner, "accepted partial ITEM_GIVE live owner")
	assertExchangeLiveStateUnchanged(t, runtime, wantPeer, "accepted partial ITEM_GIVE live peer")
}

func TestGameRuntimeItemGivePlacesPartialStackMergesIntoCompatiblePeerStack(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GivePartialMergeOwner", 0x01030da2, 0x02040da2, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1812, Vnum: 27001, Count: 3, Slot: 5}}
	peer := peerVisibilityCharacter("GivePartialMergePeer", 0x01030da3, 0x02040da3, 1120, 2120, 0, 101, 201)
	peer.Inventory = []inventory.ItemInstance{{ID: 1813, Vnum: 27001, Count: 2, Slot: 6}}
	issuePeerTicket(t, ticketStore, "item-give-partial-merge-owner", 0x70707da2, owner)
	issuePeerTicket(t, ticketStore, "item-give-partial-merge-peer", 0x70707da3, peer)
	if err := accounts.Save(accountstore.Account{Login: "item-give-partial-merge-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partial-merge item-give owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-give-partial-merge-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed partial-merge item-give peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected partial-merge item-give runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-partial-merge-owner", 0x70707da2)
	defer closeSessionFlow(t, flow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-partial-merge-peer", 0x70707da3)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, flow)
	_ = flushServerFrames(t, peerFlow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{TargetVID: peer.VID, Position: itemproto.InventoryPosition(5), Count: 1})))
	if err != nil {
		t.Fatalf("unexpected accepted partial-merge item-give packet error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected accepted partial-merge ITEM_GIVE to emit remainder ITEM_UPDATE, got %d", len(out))
	}
	queued := flushServerFrames(t, peerFlow)
	if len(queued) != 2 {
		t.Fatalf("expected recipient to receive ITEM_UPDATE plus ITEM_GET, got %d", len(queued))
	}
	update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode recipient merge ITEM_UPDATE: %v", err)
	}
	if update.Position != itemproto.InventoryPosition(6) || update.Count != 3 {
		t.Fatalf("unexpected recipient merge ITEM_UPDATE: %+v", update)
	}
	got, err := itemproto.DecodeGet(decodeSingleFrame(t, queued[1]))
	if err != nil {
		t.Fatalf("decode recipient merge ITEM_GET: %v", err)
	}
	if got.Vnum != 27001 || got.Count != 1 {
		t.Fatalf("unexpected recipient merge ITEM_GET: %+v", got)
	}

	wantOwner := owner
	wantOwner.Inventory = []inventory.ItemInstance{{ID: 1812, Vnum: 27001, Count: 2, Slot: 5}}
	wantPeer := peer
	wantPeer.Inventory = []inventory.ItemInstance{{ID: 1813, Vnum: 27001, Count: 3, Slot: 6}}
	assertExchangeAccountUnchanged(t, accounts, "item-give-partial-merge-owner", wantOwner, "accepted partial-merge ITEM_GIVE owner")
	assertExchangeAccountUnchanged(t, accounts, "item-give-partial-merge-peer", wantPeer, "accepted partial-merge ITEM_GIVE peer")
	assertExchangeLiveStateUnchanged(t, runtime, wantOwner, "accepted partial-merge ITEM_GIVE live owner")
	assertExchangeLiveStateUnchanged(t, runtime, wantPeer, "accepted partial-merge ITEM_GIVE live peer")
}

func TestGameRuntimeItemGivePlacesPartialStackPreservesInstanceSocketsAndAttributes(t *testing.T) {
	giverSockets := inventory.SocketValues{11, 0, -3}
	giverAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	peerBagSockets := inventory.SocketValues{21, 22, 0}
	peerBagAttributes := inventory.AttributeValues{{Type: 2, Value: 8}}
	peerEquipSockets := inventory.SocketValues{7, 0, 9}
	peerEquipAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}

	cases := []struct {
		name              string
		giverSockets      *inventory.SocketValues
		giverAttributes   *inventory.AttributeValues
		peerBagSockets    *inventory.SocketValues
		peerBagAttributes *inventory.AttributeValues
		wantWireSockets   [itemproto.ItemSocketCount]int32
		wantWireAttr0     itemproto.Attribute
		wantWireAttr1     itemproto.Attribute
	}{
		{
			name:              "active sockets and attributes",
			giverSockets:      &giverSockets,
			giverAttributes:   &giverAttributes,
			peerBagSockets:    &peerBagSockets,
			peerBagAttributes: &peerBagAttributes,
			wantWireSockets:   [itemproto.ItemSocketCount]int32{11, 0, -3},
			wantWireAttr0:     itemproto.Attribute{Type: 4, Value: 55},
			wantWireAttr1:     itemproto.Attribute{Type: 9, Value: -7},
		},
		{
			name:            "explicit zero sockets and attributes",
			giverSockets:    &zeroSockets,
			giverAttributes: &zeroAttributes,
			peerBagSockets:  &zeroSockets,
			wantWireSockets: [itemproto.ItemSocketCount]int32{},
		},
		{
			name:            "omitted presence keeps template fallback on the transferred clone",
			wantWireSockets: [itemproto.ItemSocketCount]int32{-11, 202, -303},
			wantWireAttr0:   itemproto.Attribute{Type: 12, Value: 34},
			wantWireAttr1:   itemproto.Attribute{Type: 15, Value: -9},
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("GivePartialPreserveOwner", 0x01030db0+uint32(i), 0x02040db0+uint32(i), 1100, 2100, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{{
				ID: 1830, Vnum: 27001, Count: 3, Slot: 5,
				Sockets: tc.giverSockets, Attributes: tc.giverAttributes,
			}}
			peer := peerVisibilityCharacter("GivePartialPreservePeer", 0x01030db4+uint32(i), 0x02040db4+uint32(i), 1120, 2120, 0, 101, 201)
			peer.Inventory = []inventory.ItemInstance{{
				ID: 1831, Vnum: 27002, Count: 1, Slot: 6,
				Sockets: tc.peerBagSockets, Attributes: tc.peerBagAttributes,
			}}
			peer.Equipment = []inventory.ItemInstance{{
				ID: 1832, Vnum: 12200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon,
				Sockets: &peerEquipSockets, Attributes: &peerEquipAttributes,
			}}
			ownerLogin := "give-part-own-" + string(rune('a'+i))
			peerLogin := "give-part-peer-" + string(rune('a'+i))
			ownerKey := uint32(0x70707db0 + i)
			peerKey := uint32(0x70707db4 + i)
			issuePeerTicket(t, ticketStore, ownerLogin, ownerKey, owner)
			issuePeerTicket(t, ticketStore, peerLogin, peerKey, peer)
			if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed preserve partial item-give owner account: %v", err)
			}
			if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
				t.Fatalf("seed preserve partial item-give peer account: %v", err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{
				{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotWeapon.String()},
				{
					Vnum:      12200,
					Name:      "Practice Blade",
					Stackable: false,
					MaxCount:  1,
					EquipSlot: inventory.EquipmentSlotWeapon.String(),
					Sockets:   itemcatalog.SocketValues{1, 2, 3},
					Attributes: itemcatalog.AttributeValues{
						{Type: 3, Value: 30},
					},
				},
				{
					Vnum:       27001,
					Name:       "Small Red Potion",
					Stackable:  true,
					MaxCount:   200,
					Sockets:    itemcatalog.SocketValues{-11, 202, -303},
					Attributes: itemcatalog.AttributeValues{{Type: 12, Value: 34}, {Type: 15, Value: -9}},
				},
				{
					Vnum:       27002,
					Name:       "Peer Potion",
					Stackable:  true,
					MaxCount:   200,
					Sockets:    itemcatalog.SocketValues{41, 42, 43},
					Attributes: itemcatalog.AttributeValues{{Type: 8, Value: 44}},
				},
			})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected preserve partial item-give runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, ownerKey)
			defer closeSessionFlow(t, flow)
			peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, peerKey)
			defer closeSessionFlow(t, peerFlow)
			_ = flushServerFrames(t, flow)
			_ = flushServerFrames(t, peerFlow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{TargetVID: peer.VID, Position: itemproto.InventoryPosition(5), Count: 1})))
			if err != nil {
				t.Fatalf("unexpected preserve partial item-give packet error: %v", err)
			}
			if len(out) != 1 {
				t.Fatalf("expected preserve partial ITEM_GIVE to emit ITEM_UPDATE, got %d", len(out))
			}
			update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[0]))
			if err != nil {
				t.Fatalf("decode preserve remainder ITEM_UPDATE: %v", err)
			}
			if update.Position != itemproto.InventoryPosition(5) || update.Count != 2 {
				t.Fatalf("unexpected preserve remainder ITEM_UPDATE: %+v", update)
			}
			if update.Sockets != tc.wantWireSockets {
				t.Fatalf("expected preserve remainder ITEM_UPDATE sockets %+v, got %+v", tc.wantWireSockets, update.Sockets)
			}
			if update.Attributes[0] != tc.wantWireAttr0 || update.Attributes[1] != tc.wantWireAttr1 {
				t.Fatalf("expected preserve remainder ITEM_UPDATE attributes %+v %+v, got %+v", tc.wantWireAttr0, tc.wantWireAttr1, update.Attributes)
			}
			queued := flushServerFrames(t, peerFlow)
			if len(queued) != 2 {
				t.Fatalf("expected preserve recipient to receive ITEM_SET plus ITEM_GET, got %d", len(queued))
			}
			set, err := itemproto.DecodeSet(decodeSingleFrame(t, queued[0]))
			if err != nil {
				t.Fatalf("decode preserve recipient ITEM_SET: %v", err)
			}
			if set.Position != itemproto.InventoryPosition(5) || set.Vnum != 27001 || set.Count != 1 {
				t.Fatalf("unexpected preserve recipient ITEM_SET: %+v", set)
			}
			if set.Sockets != tc.wantWireSockets {
				t.Fatalf("expected preserve recipient ITEM_SET sockets %+v, got %+v", tc.wantWireSockets, set.Sockets)
			}
			if set.Attributes[0] != tc.wantWireAttr0 || set.Attributes[1] != tc.wantWireAttr1 {
				t.Fatalf("expected preserve recipient ITEM_SET attributes %+v %+v, got %+v", tc.wantWireAttr0, tc.wantWireAttr1, set.Attributes)
			}

			ownerAccount, err := accounts.Load(ownerLogin)
			if err != nil {
				t.Fatalf("load preserve partial owner account: %v", err)
			}
			persistedOwner := findPersistedCharacter(t, ownerAccount, owner.Name)
			if len(persistedOwner.Inventory) != 1 {
				t.Fatalf("expected preserve owner remainder after partial give, got %+v", persistedOwner.Inventory)
			}
			gotRemainder := persistedOwner.Inventory[0]
			if gotRemainder.ID != 1830 || gotRemainder.Vnum != 27001 || gotRemainder.Count != 2 || gotRemainder.Slot != 5 {
				t.Fatalf("unexpected preserve remainder stack: %+v", gotRemainder)
			}
			assertGiveInstancePresence(t, gotRemainder, tc.giverSockets, tc.giverAttributes, "giver remainder")

			peerAccount, err := accounts.Load(peerLogin)
			if err != nil {
				t.Fatalf("load preserve partial peer account: %v", err)
			}
			persistedPeer := findPersistedCharacter(t, peerAccount, peer.Name)
			if len(persistedPeer.Inventory) != 2 {
				t.Fatalf("unexpected preserve peer inventory after partial give: %+v", persistedPeer.Inventory)
			}
			var gotGiven, gotKept inventory.ItemInstance
			for _, item := range persistedPeer.Inventory {
				switch item.Vnum {
				case 27001:
					gotGiven = item
				case 27002:
					gotKept = item
				}
			}
			if gotGiven.ID == 0 || gotGiven.ID == 1830 || gotGiven.ID == 1831 || gotGiven.ID == 1832 || gotGiven.Count != 1 || gotGiven.Slot != 5 {
				t.Fatalf("unexpected preserve transferred clone: %+v", gotGiven)
			}
			assertGiveInstancePresence(t, gotGiven, tc.giverSockets, tc.giverAttributes, "transferred clone")
			if gotKept.ID != 1831 || gotKept.Vnum != 27002 || gotKept.Count != 1 || gotKept.Slot != 6 {
				t.Fatalf("unexpected preserve peer remaining stack: %+v", gotKept)
			}
			assertGiveInstancePresence(t, gotKept, tc.peerBagSockets, tc.peerBagAttributes, "recipient remaining stack")
			if len(persistedPeer.Equipment) != 1 || persistedPeer.Equipment[0].ID != 1832 || persistedPeer.Equipment[0].Vnum != 12200 || persistedPeer.Equipment[0].EquipSlot != inventory.EquipmentSlotWeapon {
				t.Fatalf("unexpected preserve peer equipment after partial give: %+v", persistedPeer.Equipment)
			}
			assertGiveInstancePresence(t, persistedPeer.Equipment[0], &peerEquipSockets, &peerEquipAttributes, "recipient equipment")
			if gotRemainder.HasSockets() && gotGiven.HasSockets() && gotRemainder.Sockets == gotGiven.Sockets {
				t.Fatal("expected transferred clone sockets pointer to be independent of giver remainder")
			}
			if gotRemainder.HasAttributes() && gotGiven.HasAttributes() && gotRemainder.Attributes == gotGiven.Attributes {
				t.Fatal("expected transferred clone attributes pointer to be independent of giver remainder")
			}
		})
	}
}

func TestGameRuntimeItemGivePartialStackFailsClosedWhenPeerInventoryIsFull(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GivePartialFullOwner", 0x01030da4, 0x02040da4, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1814, Vnum: 27001, Count: 3, Slot: 5}}
	peer := peerVisibilityCharacter("GivePartialFullPeer", 0x01030da5, 0x02040da5, 1120, 2120, 0, 101, 201)
	peer.Inventory = make([]inventory.ItemInstance, 0, int(inventory.CarriedInventorySlotCount))
	for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
		peer.Inventory = append(peer.Inventory, inventory.ItemInstance{ID: 2100 + uint64(slot), Vnum: 40000 + uint32(slot), Count: 1, Slot: slot})
	}
	issuePeerTicket(t, ticketStore, "item-give-partial-full-owner", 0x70707da4, owner)
	issuePeerTicket(t, ticketStore, "item-give-partial-full-peer", 0x70707da5, peer)
	if err := accounts.Save(accountstore.Account{Login: "item-give-partial-full-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed full-inventory partial item-give owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-give-partial-full-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed full-inventory partial item-give peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected full-inventory partial item-give runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-partial-full-owner", 0x70707da4)
	defer closeSessionFlow(t, flow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-partial-full-peer", 0x70707da5)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, flow)
	_ = flushServerFrames(t, peerFlow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{TargetVID: peer.VID, Position: itemproto.InventoryPosition(5), Count: 1})))
	if err != nil {
		t.Fatalf("unexpected full-inventory partial item-give packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected full-inventory partial ITEM_GIVE to emit no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued giver frames after full-inventory partial ITEM_GIVE, got %d", len(queued))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames after full-inventory partial ITEM_GIVE, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, "item-give-partial-full-owner", owner, "full-inventory partial ITEM_GIVE owner")
	assertExchangeAccountUnchanged(t, accounts, "item-give-partial-full-peer", peer, "full-inventory partial ITEM_GIVE peer")
}

func TestGameRuntimeItemGivePlacesWholeStackPreservesInstanceSocketsAndAttributes(t *testing.T) {
	giverSockets := inventory.SocketValues{11, 0, -3}
	giverAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	peerBagSockets := inventory.SocketValues{21, 22, 0}
	peerBagAttributes := inventory.AttributeValues{{Type: 2, Value: 8}}
	peerEquipSockets := inventory.SocketValues{7, 0, 9}
	peerEquipAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}

	cases := []struct {
		name              string
		giverSockets      *inventory.SocketValues
		giverAttributes   *inventory.AttributeValues
		peerBagSockets    *inventory.SocketValues
		peerBagAttributes *inventory.AttributeValues
		wantWireSockets   [itemproto.ItemSocketCount]int32
		wantWireAttr0     itemproto.Attribute
		wantWireAttr1     itemproto.Attribute
	}{
		{
			name:              "active sockets and attributes",
			giverSockets:      &giverSockets,
			giverAttributes:   &giverAttributes,
			peerBagSockets:    &peerBagSockets,
			peerBagAttributes: &peerBagAttributes,
			wantWireSockets:   [itemproto.ItemSocketCount]int32{11, 0, -3},
			wantWireAttr0:     itemproto.Attribute{Type: 4, Value: 55},
			wantWireAttr1:     itemproto.Attribute{Type: 9, Value: -7},
		},
		{
			name:            "explicit zero sockets and attributes",
			giverSockets:    &zeroSockets,
			giverAttributes: &zeroAttributes,
			peerBagSockets:  &zeroSockets,
			wantWireSockets: [itemproto.ItemSocketCount]int32{},
		},
		{
			name:            "omitted presence keeps template fallback on the transferred stack",
			wantWireSockets: [itemproto.ItemSocketCount]int32{-11, 202, -303},
			wantWireAttr0:   itemproto.Attribute{Type: 12, Value: 34},
			wantWireAttr1:   itemproto.Attribute{Type: 15, Value: -9},
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("GivePreserveOwner", 0x01030d90+uint32(i), 0x02040d90+uint32(i), 1100, 2100, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{{
				ID: 1820, Vnum: 27001, Count: 3, Slot: 5,
				Sockets: tc.giverSockets, Attributes: tc.giverAttributes,
			}}
			peer := peerVisibilityCharacter("GivePreservePeer", 0x01030d94+uint32(i), 0x02040d94+uint32(i), 1120, 2120, 0, 101, 201)
			peer.Inventory = []inventory.ItemInstance{{
				ID: 1821, Vnum: 27002, Count: 1, Slot: 6,
				Sockets: tc.peerBagSockets, Attributes: tc.peerBagAttributes,
			}}
			peer.Equipment = []inventory.ItemInstance{{
				ID: 1822, Vnum: 12200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon,
				Sockets: &peerEquipSockets, Attributes: &peerEquipAttributes,
			}}
			ownerLogin := "item-give-preserve-owner-" + string(rune('a'+i))
			peerLogin := "item-give-preserve-peer-" + string(rune('a'+i))
			ownerKey := uint32(0x70707d90 + i)
			peerKey := uint32(0x70707d94 + i)
			issuePeerTicket(t, ticketStore, ownerLogin, ownerKey, owner)
			issuePeerTicket(t, ticketStore, peerLogin, peerKey, peer)
			if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed preserve item-give owner account: %v", err)
			}
			if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
				t.Fatalf("seed preserve item-give peer account: %v", err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{
				{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotWeapon.String()},
				{
					Vnum:      12200,
					Name:      "Practice Blade",
					Stackable: false,
					MaxCount:  1,
					EquipSlot: inventory.EquipmentSlotWeapon.String(),
					Sockets:   itemcatalog.SocketValues{1, 2, 3},
					Attributes: itemcatalog.AttributeValues{
						{Type: 3, Value: 30},
					},
				},
				{
					Vnum:       27001,
					Name:       "Small Red Potion",
					Stackable:  true,
					MaxCount:   200,
					Sockets:    itemcatalog.SocketValues{-11, 202, -303},
					Attributes: itemcatalog.AttributeValues{{Type: 12, Value: 34}, {Type: 15, Value: -9}},
				},
				{
					Vnum:       27002,
					Name:       "Peer Potion",
					Stackable:  true,
					MaxCount:   200,
					Sockets:    itemcatalog.SocketValues{41, 42, 43},
					Attributes: itemcatalog.AttributeValues{{Type: 8, Value: 44}},
				},
			})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected preserve item-give runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, ownerKey)
			defer closeSessionFlow(t, flow)
			peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, peerKey)
			defer closeSessionFlow(t, peerFlow)
			_ = flushServerFrames(t, flow)
			_ = flushServerFrames(t, peerFlow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{TargetVID: peer.VID, Position: itemproto.InventoryPosition(5), Count: 3})))
			if err != nil {
				t.Fatalf("unexpected preserve item-give packet error: %v", err)
			}
			if len(out) != 1 {
				t.Fatalf("expected preserve ITEM_GIVE to emit ITEM_DEL, got %d", len(out))
			}
			queued := flushServerFrames(t, peerFlow)
			if len(queued) != 2 {
				t.Fatalf("expected preserve recipient to receive ITEM_SET plus ITEM_GET, got %d", len(queued))
			}
			set, err := itemproto.DecodeSet(decodeSingleFrame(t, queued[0]))
			if err != nil {
				t.Fatalf("decode preserve recipient ITEM_SET: %v", err)
			}
			if set.Position != itemproto.InventoryPosition(5) || set.Vnum != 27001 || set.Count != 3 {
				t.Fatalf("unexpected preserve recipient ITEM_SET: %+v", set)
			}
			if set.Sockets != tc.wantWireSockets {
				t.Fatalf("expected preserve recipient ITEM_SET sockets %+v, got %+v", tc.wantWireSockets, set.Sockets)
			}
			if set.Attributes[0] != tc.wantWireAttr0 || set.Attributes[1] != tc.wantWireAttr1 {
				t.Fatalf("expected preserve recipient ITEM_SET attributes %+v %+v, got %+v", tc.wantWireAttr0, tc.wantWireAttr1, set.Attributes)
			}

			ownerAccount, err := accounts.Load(ownerLogin)
			if err != nil {
				t.Fatalf("load preserve owner account: %v", err)
			}
			persistedOwner := findPersistedCharacter(t, ownerAccount, owner.Name)
			if len(persistedOwner.Inventory) != 0 {
				t.Fatalf("expected preserve owner inventory empty after give, got %+v", persistedOwner.Inventory)
			}

			peerAccount, err := accounts.Load(peerLogin)
			if err != nil {
				t.Fatalf("load preserve peer account: %v", err)
			}
			persistedPeer := findPersistedCharacter(t, peerAccount, peer.Name)
			if len(persistedPeer.Inventory) != 2 {
				t.Fatalf("unexpected preserve peer inventory after give: %+v", persistedPeer.Inventory)
			}
			var gotGiven, gotKept inventory.ItemInstance
			for _, item := range persistedPeer.Inventory {
				switch item.ID {
				case 1820:
					gotGiven = item
				case 1821:
					gotKept = item
				}
			}
			if gotGiven.ID != 1820 || gotGiven.Vnum != 27001 || gotGiven.Count != 3 || gotGiven.Slot != 5 {
				t.Fatalf("unexpected preserve transferred stack: %+v", gotGiven)
			}
			assertGiveInstancePresence(t, gotGiven, tc.giverSockets, tc.giverAttributes, "transferred stack")
			if gotKept.ID != 1821 || gotKept.Vnum != 27002 || gotKept.Count != 1 || gotKept.Slot != 6 {
				t.Fatalf("unexpected preserve peer remaining stack: %+v", gotKept)
			}
			assertGiveInstancePresence(t, gotKept, tc.peerBagSockets, tc.peerBagAttributes, "recipient remaining stack")
			if len(persistedPeer.Equipment) != 1 || persistedPeer.Equipment[0].ID != 1822 || persistedPeer.Equipment[0].Vnum != 12200 || persistedPeer.Equipment[0].EquipSlot != inventory.EquipmentSlotWeapon {
				t.Fatalf("unexpected preserve peer equipment after give: %+v", persistedPeer.Equipment)
			}
			assertGiveInstancePresence(t, persistedPeer.Equipment[0], &peerEquipSockets, &peerEquipAttributes, "recipient equipment")
		})
	}
}

func assertGiveInstancePresence(t *testing.T, got inventory.ItemInstance, wantSockets *inventory.SocketValues, wantAttributes *inventory.AttributeValues, context string) {
	t.Helper()
	if (wantSockets != nil) != got.HasSockets() {
		t.Fatalf("%s HasSockets=%v want %v", context, got.HasSockets(), wantSockets != nil)
	}
	if wantSockets != nil {
		if got.Sockets == nil || *got.Sockets != *wantSockets {
			t.Fatalf("expected %s sockets %+v, got %#v", context, *wantSockets, got.Sockets)
		}
	} else if got.Sockets != nil {
		t.Fatalf("expected omitted %s sockets, got %#v", context, got.Sockets)
	}
	if (wantAttributes != nil) != got.HasAttributes() {
		t.Fatalf("%s HasAttributes=%v want %v", context, got.HasAttributes(), wantAttributes != nil)
	}
	if wantAttributes != nil {
		if got.Attributes == nil || *got.Attributes != *wantAttributes {
			t.Fatalf("expected %s attributes %+v, got %#v", context, *wantAttributes, got.Attributes)
		}
	} else if got.Attributes != nil {
		t.Fatalf("expected omitted %s attributes, got %#v", context, got.Attributes)
	}
}

func TestGameRuntimeItemGiveFailsClosedWhenPeerInventoryIsFull(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GiveFullOwner", 0x01030d82, 0x02040d82, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1810, Vnum: 27001, Count: 3, Slot: 5}}
	peer := peerVisibilityCharacter("GiveFullPeer", 0x01030d83, 0x02040d83, 1120, 2120, 0, 101, 201)
	peer.Inventory = make([]inventory.ItemInstance, 0, int(inventory.CarriedInventorySlotCount))
	for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
		peer.Inventory = append(peer.Inventory, inventory.ItemInstance{ID: 2000 + uint64(slot), Vnum: 40000 + uint32(slot), Count: 1, Slot: slot})
	}
	issuePeerTicket(t, ticketStore, "item-give-full-owner", 0x70707d82, owner)
	issuePeerTicket(t, ticketStore, "item-give-full-peer", 0x70707d83, peer)
	if err := accounts.Save(accountstore.Account{Login: "item-give-full-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed full-inventory item-give owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-give-full-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed full-inventory item-give peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected full-inventory item-give runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-full-owner", 0x70707d82)
	defer closeSessionFlow(t, flow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-full-peer", 0x70707d83)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, flow)
	_ = flushServerFrames(t, peerFlow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{TargetVID: peer.VID, Position: itemproto.InventoryPosition(5), Count: 3})))
	if err != nil {
		t.Fatalf("unexpected full-inventory item-give packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected full-inventory ITEM_GIVE to emit no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued giver frames after full-inventory ITEM_GIVE, got %d", len(queued))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames after full-inventory ITEM_GIVE, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, "item-give-full-owner", owner, "full-inventory ITEM_GIVE owner")
	assertExchangeAccountUnchanged(t, accounts, "item-give-full-peer", peer, "full-inventory ITEM_GIVE peer")
}

func TestGameRuntimeItemGiveFailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GiveOwner", 0x01030740, 0x02040740, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 501, Vnum: 27001, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "item-give-owner", 0x70707040, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-give-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-give account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected item-give runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-owner", 0x70707040)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{TargetVID: 0x02040741, Position: itemproto.InventoryPosition(5), Count: 1})))
	if err != nil {
		t.Fatalf("unexpected item-give packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected unsupported ITEM_GIVE to emit no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after unsupported ITEM_GIVE, got %d", len(queued))
	}
	persisted, err := accounts.Load("item-give-owner")
	if err != nil {
		t.Fatalf("load persisted item-give account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("unsupported ITEM_GIVE mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("unsupported ITEM_GIVE mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameRuntimeItemGiveAntiGiveTemplateReturnsAuthoredRejectTextWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GiveBound", 0x01030741, 0x02040741, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 502, Vnum: 27042, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	target := peerVisibilityCharacter("GiveBoundTarget", 0x01030744, 0x02040744, 1120, 2120, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "item-give-bound", 0x70707041, owner)
	issuePeerTicket(t, ticketStore, "item-give-bound-target", 0x70707044, target)
	if err := accounts.Save(accountstore.Account{Login: "item-give-bound", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed bound item-give account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-give-bound-target", Empire: target.Empire, Characters: cloneCharacters([]loginticket.Character{target})}); err != nil {
		t.Fatalf("seed bound item-give target account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:           27042,
		Name:           "Bound Gift Potion",
		Stackable:      true,
		MaxCount:       200,
		AntiGive:       true,
		GiveRejectText: "You cannot give this item.",
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected bound item-give runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-bound", 0x70707041)
	defer closeSessionFlow(t, flow)
	targetFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-bound-target", 0x70707044)
	defer closeSessionFlow(t, targetFlow)
	_ = flushServerFrames(t, flow)
	_ = flushServerFrames(t, targetFlow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{TargetVID: target.VID, Position: itemproto.InventoryPosition(5), Count: 1})))
	if err != nil {
		t.Fatalf("unexpected anti-give item-give packet error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected anti-give ITEM_GIVE to emit one info-chat frame, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode anti-give item-give rejection info chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != template.GiveRejectText {
		t.Fatalf("unexpected anti-give item-give rejection chat: %+v", delivery)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after anti-give ITEM_GIVE rejection, got %d", len(queued))
	}
	if queued := flushServerFrames(t, targetFlow); len(queued) != 0 {
		t.Fatalf("expected visible target to receive no queued frames after anti-give ITEM_GIVE rejection, got %d", len(queued))
	}
	persisted, err := accounts.Load("item-give-bound")
	if err != nil {
		t.Fatalf("load persisted bound item-give account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("anti-give ITEM_GIVE mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("anti-give ITEM_GIVE mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
	assertExchangeAccountUnchanged(t, accounts, "item-give-bound-target", target, "anti-give ITEM_GIVE visible target")
}

func TestGameRuntimeItemGiveAntiGiveTemplateClosesActiveMerchantWindowWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := merchantBuyerCharacter("GiveMerchantBound", 0x01030749, 0x02040749, 12345, []inventory.ItemInstance{{ID: 507, Vnum: 27044, Count: 3, Slot: 5}})
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	target := peerVisibilityCharacter("GiveMerchantTarget", 0x0103074a, 0x0204074a, 1120, 2120, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "item-give-merchant-bound", 0x70707049, owner)
	issuePeerTicket(t, ticketStore, "item-give-merchant-target", 0x7070704a, target)
	if err := accounts.Save(accountstore.Account{Login: "item-give-merchant-bound", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed merchant-bound item-give account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-give-merchant-target", Empire: target.Empire, Characters: cloneCharacters([]loginticket.Character{target})}); err != nil {
		t.Fatalf("seed merchant-bound item-give target account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:           27044,
		Name:           "Merchant Bound Gift Potion",
		Stackable:      true,
		MaxCount:       200,
		AntiGive:       true,
		GiveRejectText: "You cannot give this item while shopping.",
	}
	templates := append(defaultMerchantItemTemplates(), template)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{defaultMerchantCatalogDefinition()})
	itemStore := newItemTemplateStore(t, templates)
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected merchant-bound item-give runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Merchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, "npc:merchant")
	if !ok {
		t.Fatal("expected merchant static actor registration to succeed")
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-merchant-bound", 0x70707049)
	defer closeSessionFlow(t, ownerFlow)
	targetFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-merchant-target", 0x7070704a)
	defer closeSessionFlow(t, targetFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, targetFlow)
	interactWithMerchantForBuy(t, ownerFlow, actor.EntityID)

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{TargetVID: target.VID, Position: itemproto.InventoryPosition(5), Count: 1})))
	if err != nil {
		t.Fatalf("unexpected merchant anti-give item-give packet error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected anti-give ITEM_GIVE to emit SHOP END plus info-chat frame, got %d", len(out))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, out[0])); err != nil {
		t.Fatalf("decode item-give merchant SHOP END before rejection info chat: %v", err)
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode merchant anti-give item-give rejection info chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != template.GiveRejectText {
		t.Fatalf("unexpected merchant anti-give item-give rejection chat: %+v", delivery)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no queued owner frames after merchant anti-give ITEM_GIVE rejection, got %d", len(queued))
	}
	if queued := flushServerFrames(t, targetFlow); len(queued) != 0 {
		t.Fatalf("expected visible target to receive no queued frames after merchant anti-give ITEM_GIVE rejection, got %d", len(queued))
	}

	closeOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected post-item-give merchant SHOP END error: %v", err)
	}
	if len(closeOut) != 0 {
		t.Fatalf("expected post-item-give merchant SHOP END to emit no frames after shell close, got %d", len(closeOut))
	}
	buyOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{RawLeadingByte: 1, CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected post-item-give merchant SHOP BUY error: %v", err)
	}
	if len(buyOut) != 0 {
		t.Fatalf("expected post-item-give merchant SHOP BUY to fail closed until reopen, got %d", len(buyOut))
	}
	assertExchangeAccountUnchanged(t, accounts, "item-give-merchant-bound", owner, "owner item-give merchant close")
	assertExchangeAccountUnchanged(t, accounts, "item-give-merchant-target", target, "target item-give merchant close")
}

func TestGameRuntimeItemGiveAntiGiveTemplateClosesActiveExchangeShellWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GiveExchangeBound", 0x01030747, 0x02040747, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 505, Vnum: 27043, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	target := peerVisibilityCharacter("GiveExchangeTarget", 0x01030748, 0x02040748, 1120, 2120, 0, 101, 201)
	target.Inventory = []inventory.ItemInstance{{ID: 506, Vnum: 27001, Count: 2, Slot: 6}}
	target.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	issuePeerTicket(t, ticketStore, "item-give-exchange-bound", 0x70707047, owner)
	issuePeerTicket(t, ticketStore, "item-give-exchange-target", 0x70707048, target)
	if err := accounts.Save(accountstore.Account{Login: "item-give-exchange-bound", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange-bound item-give account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-give-exchange-target", Empire: target.Empire, Characters: cloneCharacters([]loginticket.Character{target})}); err != nil {
		t.Fatalf("seed exchange-bound item-give target account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:           27043,
		Name:           "Exchange Bound Gift Potion",
		Stackable:      true,
		MaxCount:       200,
		AntiGive:       true,
		GiveRejectText: "You cannot give this item while trading.",
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange-bound item-give runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-exchange-bound", 0x70707047)
	defer closeSessionFlow(t, flow)
	targetFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-exchange-target", 0x70707048)
	defer closeSessionFlow(t, targetFlow)
	_ = flushServerFrames(t, flow)
	_ = flushServerFrames(t, targetFlow)

	startOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: target.VID})))
	if err != nil {
		t.Fatalf("unexpected item-give exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected item-give exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], target.VID, "item-give exchange owner start")
	queuedStart := flushServerFrames(t, targetFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected item-give exchange target start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "item-give exchange target start")

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{TargetVID: target.VID, Position: itemproto.InventoryPosition(5), Count: 1})))
	if err != nil {
		t.Fatalf("unexpected exchange anti-give item-give packet error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected anti-give ITEM_GIVE to emit exchange END plus info-chat frame, got %d", len(out))
	}
	assertExchangeEndFrame(t, out[0], "item-give exchange owner close")
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode exchange anti-give item-give rejection info chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != template.GiveRejectText {
		t.Fatalf("unexpected exchange anti-give item-give rejection chat: %+v", delivery)
	}
	queuedClose := flushServerFrames(t, targetFlow)
	if len(queuedClose) != 1 {
		t.Fatalf("expected item-give exchange target to receive one queued END, got %d", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "item-give exchange target close")

	cancelOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected post-item-give exchange cancel error: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected post-item-give exchange cancel to emit no frames after shell close, got %d", len(cancelOut))
	}
	assertExchangeAccountUnchanged(t, accounts, "item-give-exchange-bound", owner, "owner item-give exchange close")
	assertExchangeAccountUnchanged(t, accounts, "item-give-exchange-target", target, "target item-give exchange close")
}

func TestGameRuntimeItemGiveAntiGiveRejectTextRequiresVisibleTargetWithoutMutation(t *testing.T) {
	cases := []struct {
		name      string
		login     string
		key       uint32
		targetVID uint32
	}{
		{name: "zero target", login: "item-give-zero-target", key: 0x70707045, targetVID: 0},
		{name: "invisible target", login: "item-give-invisible-target", key: 0x70707046, targetVID: 0x02049999},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("GiveTargetGuard", 0x01030745, 0x02040745, 1100, 2100, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{{ID: 504, Vnum: 27042, Count: 3, Slot: 5}}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			issuePeerTicket(t, ticketStore, tc.login, tc.key, owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed %s item-give account: %v", tc.name, err)
			}
			template := itemcatalog.Template{
				Vnum:           27042,
				Name:           "Bound Gift Potion",
				Stackable:      true,
				MaxCount:       200,
				AntiGive:       true,
				GiveRejectText: "You cannot give this item.",
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected %s item-give runtime error: %v", tc.name, err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, tc.key)
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{TargetVID: tc.targetVID, Position: itemproto.InventoryPosition(5), Count: 1})))
			if err != nil {
				t.Fatalf("unexpected %s anti-give item-give packet error: %v", tc.name, err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s anti-give ITEM_GIVE to emit no frames, got %d", tc.name, len(out))
			}
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected no queued frames after %s anti-give ITEM_GIVE rejection, got %d", tc.name, len(queued))
			}
			persisted, err := accounts.Load(tc.login)
			if err != nil {
				t.Fatalf("load persisted %s item-give account: %v", tc.name, err)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("%s anti-give ITEM_GIVE mutated inventory: got %+v want %+v", tc.name, persisted.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("%s anti-give ITEM_GIVE mutated quickslots: got %+v want %+v", tc.name, persisted.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}

func TestGameRuntimeItemGiveAntiGiveRejectTextRequiresValidRequestedCountWithoutMutation(t *testing.T) {
	cases := []struct {
		name  string
		login string
		key   uint32
		count uint8
	}{
		{name: "zero count", login: "item-give-zero-count", key: 0x70707042, count: 0},
		{name: "oversized count", login: "item-give-oversized-count", key: 0x70707043, count: 4},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("GiveCountGuard", 0x01030742, 0x02040742, 1100, 2100, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{{ID: 503, Vnum: 27042, Count: 3, Slot: 5}}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			issuePeerTicket(t, ticketStore, tc.login, tc.key, owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed %s item-give account: %v", tc.name, err)
			}
			template := itemcatalog.Template{
				Vnum:           27042,
				Name:           "Bound Gift Potion",
				Stackable:      true,
				MaxCount:       200,
				AntiGive:       true,
				GiveRejectText: "You cannot give this item.",
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected %s item-give runtime error: %v", tc.name, err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, tc.key)
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{TargetVID: 0x02040744, Position: itemproto.InventoryPosition(5), Count: tc.count})))
			if err != nil {
				t.Fatalf("unexpected %s anti-give item-give packet error: %v", tc.name, err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s anti-give ITEM_GIVE to emit no frames, got %d", tc.name, len(out))
			}
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected no queued frames after %s anti-give ITEM_GIVE rejection, got %d", tc.name, len(queued))
			}
			persisted, err := accounts.Load(tc.login)
			if err != nil {
				t.Fatalf("load persisted %s item-give account: %v", tc.name, err)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("%s anti-give ITEM_GIVE mutated inventory: got %+v want %+v", tc.name, persisted.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("%s anti-give ITEM_GIVE mutated quickslots: got %+v want %+v", tc.name, persisted.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}
