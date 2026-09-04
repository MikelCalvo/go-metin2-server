package player

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestRuntimeEquipItemWithTemplatePreservesInstancePresenceIndependently(t *testing.T) {
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

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			character := loginticket.Character{
				ID:   1,
				Name: "EquipPresence",
				Inventory: []inventory.ItemInstance{{
					ID:         3001,
					Vnum:       0x11223344,
					Count:      1,
					Slot:       8,
					Sockets:    tc.sockets,
					Attributes: tc.attributes,
				}},
			}
			runtime := NewRuntime(character, SessionLink{Login: "equip-presence", CharacterIndex: 0})
			if len(runtime.liveInventory) != 1 {
				t.Fatalf("expected one live inventory item before equip, got %#v", runtime.liveInventory)
			}
			before := runtime.liveInventory[0]
			template := itemcatalog.Template{
				Vnum:      0x11223344,
				Name:      "Practice Armor",
				Stackable: false,
				MaxCount:  1,
				EquipSlot: inventory.EquipmentSlotBody.String(),
			}

			equipped, ok := runtime.EquipItemWithTemplate(8, inventory.EquipmentSlotBody, template)
			if !ok {
				t.Fatal("expected matching authored equip slot to allow equip")
			}
			assertIndependentEquipPresence(t, before, equipped, "equip result")
			if len(runtime.liveEquipment) != 1 {
				t.Fatalf("expected one worn item, got %#v", runtime.liveEquipment)
			}
			assertIndependentEquipPresence(t, before, runtime.liveEquipment[0], "live worn")
		})
	}
}

func TestRuntimeReplaceOccupiedEquipItemPreservesEquippedInstancePresenceIndependently(t *testing.T) {
	activeSockets := inventory.SocketValues{11, 0, -3}
	activeAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
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

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			character := loginticket.Character{
				ID:   1,
				Name: "EquipReplacePresence",
				Inventory: []inventory.ItemInstance{{
					ID:         3002,
					Vnum:       0x11223345,
					Count:      1,
					Slot:       8,
					Sockets:    tc.sockets,
					Attributes: tc.attributes,
				}},
				Equipment: []inventory.ItemInstance{{
					ID:         3001,
					Vnum:       0x11223344,
					Count:      1,
					Equipped:   true,
					EquipSlot:  inventory.EquipmentSlotBody,
					Sockets:    &wornSockets,
					Attributes: &wornAttributes,
				}},
			}
			runtime := NewRuntime(character, SessionLink{Login: "equip-replace-presence", CharacterIndex: 0})
			if len(runtime.liveInventory) != 1 {
				t.Fatalf("expected one live inventory item before replace, got %#v", runtime.liveInventory)
			}
			before := runtime.liveInventory[0]

			result, ok := runtime.ReplaceOccupiedEquipItem(8, inventory.EquipmentSlotBody)
			if !ok {
				t.Fatal("expected occupied wear swap to succeed")
			}
			if result.EquippedItem.ID != 3002 || !result.EquippedItem.Equipped || result.EquippedItem.EquipSlot != inventory.EquipmentSlotBody {
				t.Fatalf("unexpected equipped item after occupied wear swap: %#v", result.EquippedItem)
			}
			if result.UnequippedItem.ID != 3001 || result.UnequippedItem.Slot != 8 || result.UnequippedItem.Equipped {
				t.Fatalf("unexpected unequipped item after occupied wear swap: %#v", result.UnequippedItem)
			}
			assertIndependentEquipPresence(t, before, result.EquippedItem, "replace equip-side result")
			if len(runtime.liveEquipment) != 1 {
				t.Fatalf("expected one worn item after replace, got %#v", runtime.liveEquipment)
			}
			assertIndependentEquipPresence(t, before, runtime.liveEquipment[0], "live worn after replace")
			if result.UnequippedItem.Sockets == nil || *result.UnequippedItem.Sockets != wornSockets {
				t.Fatalf("expected unequipped side to keep worn presence via WithInventorySlot, got %#v", result.UnequippedItem.Sockets)
			}
		})
	}
}

func assertIndependentEquipPresence(t *testing.T, before, equipped inventory.ItemInstance, label string) {
	t.Helper()
	if equipped.HasSockets() != before.HasSockets() {
		t.Fatalf("%s HasSockets=%v want %v", label, equipped.HasSockets(), before.HasSockets())
	}
	if equipped.HasAttributes() != before.HasAttributes() {
		t.Fatalf("%s HasAttributes=%v want %v", label, equipped.HasAttributes(), before.HasAttributes())
	}
	if before.HasSockets() {
		if equipped.Sockets == before.Sockets {
			t.Fatalf("%s expected equip to clone sockets independently from the pre-equip live inventory pointer", label)
		}
		want := *before.Sockets
		if *equipped.Sockets != want {
			t.Fatalf("%s expected equipped sockets %+v, got %+v", label, want, *equipped.Sockets)
		}
		(*equipped.Sockets)[0] = 99
		if (*before.Sockets)[0] == 99 {
			t.Fatalf("%s mutating equipped sockets aliased the pre-equip live inventory pointer", label)
		}
		*equipped.Sockets = want
	} else if equipped.Sockets != nil {
		t.Fatalf("%s expected omitted equipped sockets, got %#v", label, equipped.Sockets)
	}
	if before.HasAttributes() {
		if equipped.Attributes == before.Attributes {
			t.Fatalf("%s expected equip to clone attributes independently from the pre-equip live inventory pointer", label)
		}
		want := *before.Attributes
		if *equipped.Attributes != want {
			t.Fatalf("%s expected equipped attributes %+v, got %+v", label, want, *equipped.Attributes)
		}
		(*equipped.Attributes)[0].Value = 99
		if (*before.Attributes)[0].Value == 99 {
			t.Fatalf("%s mutating equipped attributes aliased the pre-equip live inventory pointer", label)
		}
		*equipped.Attributes = want
	} else if equipped.Attributes != nil {
		t.Fatalf("%s expected omitted equipped attributes, got %#v", label, equipped.Attributes)
	}
}
