package minimal

import (
	"path/filepath"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
)

func TestGameRuntimeEquipPreservesInstanceSocketsAndAttributes(t *testing.T) {
	activeSockets := inventory.SocketValues{11, 0, -3}
	activeAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}

	cases := []struct {
		name       string
		sockets    *inventory.SocketValues
		attributes *inventory.AttributeValues
	}{
		{name: "active sockets and attributes", sockets: &activeSockets, attributes: &activeAttributes},
		{name: "explicit zero sockets and attributes", sockets: &zeroSockets, attributes: &zeroAttributes},
		{name: "omitted presence"},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
			template := itemcatalog.Template{
				Vnum:       11580,
				Name:       "Preserve Practice Armor",
				Stackable:  false,
				MaxCount:   1,
				EquipSlot:  inventory.EquipmentSlotBody.String(),
				Sockets:    itemcatalog.SocketValues{91, 92, 93},
				Attributes: itemcatalog.AttributeValues{{Type: 8, Value: 44}},
			}
			if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{template}}); err != nil {
				t.Fatalf("seed equip preserve templates: %v", err)
			}

			owner := peerVisibilityCharacter("EquipPreserve", 0x010309a0+uint32(i), 0x020409a0+uint32(i), 1300, 2300, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{{
				ID:         5901,
				Vnum:       template.Vnum,
				Count:      1,
				Slot:       8,
				Sockets:    tc.sockets,
				Attributes: tc.attributes,
			}}
			login := "equip-preserve-" + string(rune('a'+i))
			loginKey := uint32(0xa0a0a0a0 + i)
			issuePeerTicket(t, ticketStore, login, loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed equip preserve account: %v", err)
			}

			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected equip preserve runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
				Type:    chatproto.ChatTypeTalking,
				Message: "/equip_item 8 body",
			})))
			if err != nil {
				t.Fatalf("unexpected equip preserve error: %v", err)
			}
			if len(out) < 2 {
				t.Fatalf("expected equip preserve frames including ITEM_SET, got %d", len(out))
			}

			bodyPosition, err := itemproto.EquipmentPosition(0)
			if err != nil {
				t.Fatalf("build body equipment position: %v", err)
			}
			var equipSet *itemproto.SetPacket
			for _, raw := range out {
				set, decodeErr := itemproto.DecodeSet(decodeSingleFrame(t, raw))
				if decodeErr != nil {
					continue
				}
				if set.Position == bodyPosition && set.Vnum == template.Vnum && set.Count == 1 {
					copied := set
					equipSet = &copied
					break
				}
			}
			if equipSet == nil {
				t.Fatalf("expected equipment ITEM_SET in equip preserve burst, frames=%d", len(out))
			}
			assertEncodedEquipPresence(t, *equipSet, tc.sockets, tc.attributes, template)

			account, err := accounts.Load(login)
			if err != nil {
				t.Fatalf("load equip preserve account: %v", err)
			}
			if len(account.Characters[0].Inventory) != 0 {
				t.Fatalf("expected empty inventory after equip preserve, got %#v", account.Characters[0].Inventory)
			}
			if len(account.Characters[0].Equipment) != 1 {
				t.Fatalf("expected one worn item after equip preserve, got %#v", account.Characters[0].Equipment)
			}
			worn := account.Characters[0].Equipment[0]
			if worn.ID != 5901 || worn.Vnum != template.Vnum || !worn.Equipped || worn.EquipSlot != inventory.EquipmentSlotBody {
				t.Fatalf("unexpected worn identity after equip preserve: %+v", worn)
			}
			assertPersistedEquipPresence(t, worn, tc.sockets, tc.attributes)
		})
	}
}

func TestGameRuntimeOccupiedReplaceEquipPreservesInstanceSocketsAndAttributes(t *testing.T) {
	activeSockets := inventory.SocketValues{7, 0, 9}
	activeAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}
	wornSockets := inventory.SocketValues{1, 2, 3}
	wornAttributes := inventory.AttributeValues{{Type: 8, Value: 12}}

	cases := []struct {
		name       string
		sockets    *inventory.SocketValues
		attributes *inventory.AttributeValues
	}{
		{name: "active sockets and attributes", sockets: &activeSockets, attributes: &activeAttributes},
		{name: "explicit zero sockets and attributes", sockets: &zeroSockets, attributes: &zeroAttributes},
		{name: "omitted presence"},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
			wornTemplate := itemcatalog.Template{
				Vnum:       11581,
				Name:       "Worn Preserve Armor",
				Stackable:  false,
				MaxCount:   1,
				EquipSlot:  inventory.EquipmentSlotBody.String(),
				Sockets:    itemcatalog.SocketValues{71, 72, 73},
				Attributes: itemcatalog.AttributeValues{{Type: 2, Value: 10}},
			}
			carriedTemplate := itemcatalog.Template{
				Vnum:       11582,
				Name:       "Carried Preserve Armor",
				Stackable:  false,
				MaxCount:   1,
				EquipSlot:  inventory.EquipmentSlotBody.String(),
				Sockets:    itemcatalog.SocketValues{91, 92, 93},
				Attributes: itemcatalog.AttributeValues{{Type: 8, Value: 44}},
			}
			if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{wornTemplate, carriedTemplate}}); err != nil {
				t.Fatalf("seed occupied replace preserve templates: %v", err)
			}

			owner := peerVisibilityCharacter("EquipReplacePreserve", 0x010309b0+uint32(i), 0x020409b0+uint32(i), 1300, 2300, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{{
				ID:         5911,
				Vnum:       carriedTemplate.Vnum,
				Count:      1,
				Slot:       5,
				Sockets:    tc.sockets,
				Attributes: tc.attributes,
			}}
			owner.Equipment = []inventory.ItemInstance{{
				ID:         5910,
				Vnum:       wornTemplate.Vnum,
				Count:      1,
				Equipped:   true,
				EquipSlot:  inventory.EquipmentSlotBody,
				Sockets:    &wornSockets,
				Attributes: &wornAttributes,
			}}
			login := "equip-replace-preserve-" + string(rune('a'+i))
			loginKey := uint32(0xb0b0b0b0 + i)
			issuePeerTicket(t, ticketStore, login, loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed occupied replace preserve account: %v", err)
			}

			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected occupied replace preserve runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
			defer closeSessionFlow(t, flow)

			bodyPosition, err := itemproto.EquipmentPosition(0)
			if err != nil {
				t.Fatalf("build body equipment position: %v", err)
			}
			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
				Source:      itemproto.InventoryPosition(5),
				Destination: bodyPosition,
			})))
			if err != nil {
				t.Fatalf("unexpected occupied replace preserve error: %v", err)
			}
			if len(out) < 2 {
				t.Fatalf("expected occupied replace preserve frames, got %d", len(out))
			}

			sourceSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[0]))
			if err != nil {
				t.Fatalf("decode occupied replace unequipped ITEM_SET: %v", err)
			}
			if sourceSet.Position != itemproto.InventoryPosition(5) || sourceSet.Vnum != wornTemplate.Vnum || sourceSet.Count != 1 {
				t.Fatalf("unexpected unequipped ITEM_SET: %+v", sourceSet)
			}
			if sourceSet.Sockets != ([itemproto.ItemSocketCount]int32{1, 2, 3}) {
				t.Fatalf("expected unequipped ITEM_SET to keep worn sockets, got %+v", sourceSet.Sockets)
			}
			if sourceSet.Attributes[0] != (itemproto.Attribute{Type: 8, Value: 12}) {
				t.Fatalf("expected unequipped ITEM_SET to keep worn attributes, got %+v", sourceSet.Attributes)
			}

			equipSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
			if err != nil {
				t.Fatalf("decode occupied replace equipped ITEM_SET: %v", err)
			}
			if equipSet.Position != bodyPosition || equipSet.Vnum != carriedTemplate.Vnum || equipSet.Count != 1 {
				t.Fatalf("unexpected equipped ITEM_SET: %+v", equipSet)
			}
			assertEncodedEquipPresence(t, equipSet, tc.sockets, tc.attributes, carriedTemplate)

			account, err := accounts.Load(login)
			if err != nil {
				t.Fatalf("load occupied replace preserve account: %v", err)
			}
			if len(account.Characters[0].Inventory) != 1 || len(account.Characters[0].Equipment) != 1 {
				t.Fatalf("unexpected inventory/equipment after occupied replace: inv=%#v equip=%#v", account.Characters[0].Inventory, account.Characters[0].Equipment)
			}
			unequipped := account.Characters[0].Inventory[0]
			if unequipped.ID != 5910 || unequipped.Slot != 5 || unequipped.Equipped {
				t.Fatalf("unexpected unequipped identity: %+v", unequipped)
			}
			if !unequipped.HasSockets() || unequipped.Sockets == nil || *unequipped.Sockets != wornSockets {
				t.Fatalf("expected unequipped sockets %+v, got %#v", wornSockets, unequipped.Sockets)
			}
			if !unequipped.HasAttributes() || unequipped.Attributes == nil || *unequipped.Attributes != wornAttributes {
				t.Fatalf("expected unequipped attributes %+v, got %#v", wornAttributes, unequipped.Attributes)
			}
			worn := account.Characters[0].Equipment[0]
			if worn.ID != 5911 || worn.Vnum != carriedTemplate.Vnum || !worn.Equipped || worn.EquipSlot != inventory.EquipmentSlotBody {
				t.Fatalf("unexpected worn identity after occupied replace: %+v", worn)
			}
			assertPersistedEquipPresence(t, worn, tc.sockets, tc.attributes)
		})
	}
}

func assertEncodedEquipPresence(t *testing.T, set itemproto.SetPacket, sockets *inventory.SocketValues, attributes *inventory.AttributeValues, template itemcatalog.Template) {
	t.Helper()
	if sockets != nil {
		want := [itemproto.ItemSocketCount]int32{(*sockets)[0], (*sockets)[1], (*sockets)[2]}
		if set.Sockets != want {
			t.Fatalf("expected equipment ITEM_SET sockets %+v from instance, got %+v", want, set.Sockets)
		}
	} else {
		want := [itemproto.ItemSocketCount]int32(template.Sockets)
		if set.Sockets != want {
			t.Fatalf("expected omit→template equipment ITEM_SET sockets %+v, got %+v", want, set.Sockets)
		}
	}
	if attributes != nil {
		for i, attr := range *attributes {
			if set.Attributes[i] != (itemproto.Attribute{Type: attr.Type, Value: attr.Value}) {
				t.Fatalf("expected equipment ITEM_SET attributes from instance, got %+v want %+v", set.Attributes, *attributes)
			}
		}
	} else if set.Attributes[0] != (itemproto.Attribute{Type: template.Attributes[0].Type, Value: template.Attributes[0].Value}) {
		t.Fatalf("expected omit→template equipment ITEM_SET attributes, got %+v", set.Attributes)
	}
}

func assertPersistedEquipPresence(t *testing.T, worn inventory.ItemInstance, sockets *inventory.SocketValues, attributes *inventory.AttributeValues) {
	t.Helper()
	if (sockets != nil) != worn.HasSockets() {
		t.Fatalf("persisted HasSockets=%v want %v", worn.HasSockets(), sockets != nil)
	}
	if sockets != nil {
		if worn.Sockets == nil || *worn.Sockets != *sockets {
			t.Fatalf("expected persisted worn sockets %+v, got %#v", *sockets, worn.Sockets)
		}
	} else if worn.Sockets != nil {
		t.Fatalf("expected omitted persisted worn sockets, got %#v", worn.Sockets)
	}
	if (attributes != nil) != worn.HasAttributes() {
		t.Fatalf("persisted HasAttributes=%v want %v", worn.HasAttributes(), attributes != nil)
	}
	if attributes != nil {
		if worn.Attributes == nil || *worn.Attributes != *attributes {
			t.Fatalf("expected persisted worn attributes %+v, got %#v", *attributes, worn.Attributes)
		}
	} else if worn.Attributes != nil {
		t.Fatalf("expected omitted persisted worn attributes, got %#v", worn.Attributes)
	}
}
