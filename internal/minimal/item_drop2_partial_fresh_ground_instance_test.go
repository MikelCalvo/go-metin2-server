package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
)

func TestPartialDroppedGroundItemClonesPresenceIndependently(t *testing.T) {
	activeSockets := inventory.SocketValues{7, 0, 9}
	activeAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}

	cases := []struct {
		name       string
		sockets    *inventory.SocketValues
		attributes *inventory.AttributeValues
	}{
		{name: "active sockets and attributes", sockets: &activeSockets, attributes: &activeAttributes},
		{name: "explicit zero sockets and attributes", sockets: &zeroSockets, attributes: &zeroAttributes},
		{name: "omitted presence", sockets: nil, attributes: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := inventory.ItemInstance{
				ID:         1031,
				Vnum:       27001,
				Count:      5,
				Slot:       5,
				Sockets:    tc.sockets,
				Attributes: tc.attributes,
			}
			dropped, ok := partialDroppedGroundItem(source, 1032, 2)
			if !ok {
				t.Fatal("expected partial-drop ground helper to succeed")
			}
			if dropped.ID != 1032 || dropped.Vnum != 27001 || dropped.Count != 2 || dropped.Slot != 5 {
				t.Fatalf("unexpected dropped identity: %+v", dropped)
			}
			if dropped.HasSockets() != source.HasSockets() {
				t.Fatalf("dropped HasSockets=%v want %v", dropped.HasSockets(), source.HasSockets())
			}
			if dropped.HasAttributes() != source.HasAttributes() {
				t.Fatalf("dropped HasAttributes=%v want %v", dropped.HasAttributes(), source.HasAttributes())
			}
			if source.HasSockets() {
				if dropped.Sockets == source.Sockets {
					t.Fatal("expected dropped sockets pointer to be independent of source")
				}
				if *dropped.Sockets != *source.Sockets {
					t.Fatalf("expected dropped sockets %+v, got %+v", *source.Sockets, *dropped.Sockets)
				}
				(*dropped.Sockets)[0] = 99
				if (*source.Sockets)[0] == 99 {
					t.Fatal("mutating dropped sockets aliased the source remainder")
				}
			} else if dropped.Sockets != nil {
				t.Fatalf("expected omitted dropped sockets, got %#v", dropped.Sockets)
			}
			if source.HasAttributes() {
				if dropped.Attributes == source.Attributes {
					t.Fatal("expected dropped attributes pointer to be independent of source")
				}
				if *dropped.Attributes != *source.Attributes {
					t.Fatalf("expected dropped attributes %+v, got %+v", *source.Attributes, *dropped.Attributes)
				}
				(*dropped.Attributes)[0].Value = 99
				if (*source.Attributes)[0].Value == 99 {
					t.Fatal("mutating dropped attributes aliased the source remainder")
				}
			} else if dropped.Attributes != nil {
				t.Fatalf("expected omitted dropped attributes, got %#v", dropped.Attributes)
			}
		})
	}
}

func TestPartialDroppedGroundItemRejectsWholeStackOrZero(t *testing.T) {
	source := inventory.ItemInstance{ID: 1031, Vnum: 27001, Count: 5, Slot: 5}
	if _, ok := partialDroppedGroundItem(source, 1032, 0); ok {
		t.Fatal("expected zero-count partial-drop helper to fail closed")
	}
	if _, ok := partialDroppedGroundItem(source, 1032, 5); ok {
		t.Fatal("expected whole-stack partial-drop helper to fail closed")
	}
	if _, ok := partialDroppedGroundItem(source, 0, 2); ok {
		t.Fatal("expected zero nextID partial-drop helper to fail closed")
	}
}

func TestGameRuntimeItemDrop2PartialFreshGroundInstanceIDAndClone(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DropPartialFreshID", 0x010308a1, 0x020408a1, 1250, 2250, 0, 101, 201)
	activeSockets := inventory.SocketValues{11, 0, -3}
	activeAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	owner.Inventory = []inventory.ItemInstance{{
		ID:         2031,
		Vnum:       27001,
		Count:      5,
		Slot:       5,
		Sockets:    &activeSockets,
		Attributes: &activeAttributes,
	}}
	issuePeerTicket(t, ticketStore, "drop-partial-fresh-id", 0xa1a1a1a1, owner)
	if err := accounts.Save(accountstore.Account{Login: "drop-partial-fresh-id", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partial-drop fresh-id owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected partial-drop fresh-id runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "drop-partial-fresh-id", 0xa1a1a1a1)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(5), Count: 2})))
	if err != nil {
		t.Fatalf("unexpected partial-drop fresh-id drop2 error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected partial drop to emit ITEM_UPDATE, GROUND_ADD, and OWNERSHIP, got %d frames", len(out))
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode partial-drop fresh-id ground add: %v", err)
	}

	ownerEntity, ok := runtime.sharedWorld.playerEntityByName(owner.Name)
	if !ok {
		t.Fatal("expected live owner entity after partial drop")
	}
	pickup, ok := runtime.sharedWorld.GroundItemPickupFor(ownerEntity.Entity.ID, owner, ground.VID)
	if !ok {
		t.Fatal("expected pending ground handle after partial drop")
	}
	if pickup.Item.ID == 0 || pickup.Item.ID == 2031 {
		t.Fatalf("expected fresh ground identity distinct from remainder 2031, got %d", pickup.Item.ID)
	}
	if pickup.Item.Count != 2 || pickup.Item.Vnum != 27001 {
		t.Fatalf("unexpected ground item payload: %+v", pickup.Item)
	}
	if !pickup.Item.HasSockets() || pickup.Item.Sockets == nil || *pickup.Item.Sockets != activeSockets {
		t.Fatalf("expected cloned ground sockets %+v, got %#v", activeSockets, pickup.Item.Sockets)
	}
	if !pickup.Item.HasAttributes() || pickup.Item.Attributes == nil || *pickup.Item.Attributes != activeAttributes {
		t.Fatalf("expected cloned ground attributes %+v, got %#v", activeAttributes, pickup.Item.Attributes)
	}
	(*pickup.Item.Sockets)[0] = 99
	(*pickup.Item.Attributes)[0].Value = 99

	account, err := accounts.Load("drop-partial-fresh-id")
	if err != nil {
		t.Fatalf("load partial-drop fresh-id owner account: %v", err)
	}
	if len(account.Characters[0].Inventory) != 1 {
		t.Fatalf("expected one remainder stack after partial drop, got %#v", account.Characters[0].Inventory)
	}
	remainder := account.Characters[0].Inventory[0]
	if remainder.ID != 2031 || remainder.Count != 3 {
		t.Fatalf("unexpected remainder after partial drop: %+v", remainder)
	}
	if !remainder.HasSockets() || remainder.Sockets == nil || *remainder.Sockets != activeSockets {
		t.Fatalf("expected remainder sockets unchanged %+v, got %#v", activeSockets, remainder.Sockets)
	}
	if !remainder.HasAttributes() || remainder.Attributes == nil || *remainder.Attributes != activeAttributes {
		t.Fatalf("expected remainder attributes unchanged %+v, got %#v", activeAttributes, remainder.Attributes)
	}

	durable := runtime.sharedWorld.DurableGroundItemSnapshot()
	if len(durable.GroundItems) != 1 {
		t.Fatalf("expected one durable ground row after partial drop, got %#v", durable.GroundItems)
	}
	row := durable.GroundItems[0]
	if row.ItemID != pickup.Item.ID || row.ItemID == 2031 {
		t.Fatalf("expected durable fresh ItemID %d, got %d", pickup.Item.ID, row.ItemID)
	}
	if !row.HasSockets || row.Socket0 != 11 || row.Socket1 != 0 || row.Socket2 != -3 {
		t.Fatalf("unexpected durable sockets: %+v", row)
	}
	if !row.HasAttributes || row.Attributes == nil || *row.Attributes != activeAttributes {
		t.Fatalf("unexpected durable attributes: %+v", row)
	}
}

func TestGameRuntimeItemDrop2PartialPickupPreservesClonedInstance(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DropPartialPickupClone", 0x010308a2, 0x020408a2, 1250, 2250, 0, 101, 201)
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}
	owner.Inventory = []inventory.ItemInstance{{
		ID:         2041,
		Vnum:       27001,
		Count:      4,
		Slot:       5,
		Sockets:    &zeroSockets,
		Attributes: &zeroAttributes,
	}}
	issuePeerTicket(t, ticketStore, "drop-partial-pickup-clone", 0xa2a2a2a2, owner)
	if err := accounts.Save(accountstore.Account{Login: "drop-partial-pickup-clone", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partial-drop pickup-clone owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected partial-drop pickup-clone runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "drop-partial-pickup-clone", 0xa2a2a2a2)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(5), Count: 1})))
	if err != nil {
		t.Fatalf("unexpected partial-drop pickup-clone drop2 error: %v", err)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode partial-drop pickup-clone ground add: %v", err)
	}

	ownerEntity, ok := runtime.sharedWorld.playerEntityByName(owner.Name)
	if !ok {
		t.Fatal("expected live owner entity after partial drop")
	}
	beforePickup, ok := runtime.sharedWorld.GroundItemPickupFor(ownerEntity.Entity.ID, owner, ground.VID)
	if !ok {
		t.Fatal("expected pending ground handle before owner pickup")
	}
	freshID := beforePickup.Item.ID
	if freshID == 0 || freshID == 2041 {
		t.Fatalf("expected fresh ground identity before pickup, got %d", freshID)
	}
	if !beforePickup.Item.HasSockets() || beforePickup.Item.Sockets == nil || *beforePickup.Item.Sockets != zeroSockets {
		t.Fatalf("expected cloned explicit-zero ground sockets before pickup, got %#v", beforePickup.Item.Sockets)
	}
	if !beforePickup.Item.HasAttributes() || beforePickup.Item.Attributes == nil || *beforePickup.Item.Attributes != zeroAttributes {
		t.Fatalf("expected cloned explicit-zero ground attributes before pickup, got %#v", beforePickup.Item.Attributes)
	}

	pickupOut := pickupGroundItem(t, flow, ground.VID)
	if len(pickupOut) < 3 {
		t.Fatalf("expected owner pickup of partial drop to succeed, got %d frames", len(pickupOut))
	}
	if runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected ground handle removed after accepted owner pickup")
	}

	account, err := accounts.Load("drop-partial-pickup-clone")
	if err != nil {
		t.Fatalf("load partial-drop pickup-clone owner account: %v", err)
	}
	if len(account.Characters[0].Inventory) != 1 {
		t.Fatalf("expected compatible-stack merge back into the remainder, got %#v", account.Characters[0].Inventory)
	}
	merged := account.Characters[0].Inventory[0]
	if merged.ID != 2041 || merged.Count != 4 || merged.Vnum != 27001 || merged.Slot != 5 {
		t.Fatalf("expected remainder identity 2041 count 4 after merge pickup, got %+v", merged)
	}
	if !merged.HasSockets() || merged.Sockets == nil || *merged.Sockets != zeroSockets {
		t.Fatalf("expected merged remainder sockets to keep explicit zero, got %#v", merged.Sockets)
	}
	if !merged.HasAttributes() || merged.Attributes == nil || *merged.Attributes != zeroAttributes {
		t.Fatalf("expected merged remainder attributes to keep explicit zero, got %#v", merged.Attributes)
	}
}

func TestGameRuntimeItemDrop2WholeStackKeepsSourceIdentity(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DropWholeIdentity", 0x010308a3, 0x020408a3, 1250, 2250, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 2051, Vnum: 27001, Count: 3, Slot: 5}}
	issuePeerTicket(t, ticketStore, "drop-whole-identity", 0xa3a3a3a3, owner)
	if err := accounts.Save(accountstore.Account{Login: "drop-whole-identity", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed whole-stack identity owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected whole-stack identity runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "drop-whole-identity", 0xa3a3a3a3)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(5), Count: 3})))
	if err != nil {
		t.Fatalf("unexpected whole-stack identity drop2 error: %v", err)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode whole-stack identity ground add: %v", err)
	}
	ownerEntity, ok := runtime.sharedWorld.playerEntityByName(owner.Name)
	if !ok {
		t.Fatal("expected live owner entity after whole-stack drop")
	}
	pickup, ok := runtime.sharedWorld.GroundItemPickupFor(ownerEntity.Entity.ID, owner, ground.VID)
	if !ok {
		t.Fatal("expected pending ground handle after whole-stack drop")
	}
	if pickup.Item.ID != 2051 || pickup.Item.Count != 3 {
		t.Fatalf("expected whole-stack drop to keep source identity 2051, got %+v", pickup.Item)
	}
	account, err := accounts.Load("drop-whole-identity")
	if err != nil {
		t.Fatalf("load whole-stack identity owner account: %v", err)
	}
	if len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected empty inventory after whole-stack drop, got %#v", account.Characters[0].Inventory)
	}
}
