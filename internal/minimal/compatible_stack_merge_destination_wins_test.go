package minimal

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
)

func TestExchangePlaceIncomingDisplayedItemPreferringSlotsCompatibleMergeKeepsDestinationInstancePresence(t *testing.T) {
	destActiveSockets := inventory.SocketValues{7, 0, 9}
	destActiveAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	sourceActiveSockets := inventory.SocketValues{1, 2, 3}
	sourceActiveAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	destZeroSockets := inventory.SocketValues{}
	destZeroAttributes := inventory.AttributeValues{}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	cases := []struct {
		name              string
		destination       inventory.ItemInstance
		source            inventory.ItemInstance
		wantHasSockets    bool
		wantHasAttributes bool
		wantSockets       inventory.SocketValues
		wantAttributes    inventory.AttributeValues
	}{
		{
			name: "active destination wins over different source",
			destination: inventory.ItemInstance{
				ID: 501, Vnum: 27001, Count: 4, Slot: 5,
				Sockets: &destActiveSockets, Attributes: &destActiveAttributes,
			},
			source: inventory.ItemInstance{
				ID: 502, Vnum: 27001, Count: 3, Slot: 7,
				Sockets: &sourceActiveSockets, Attributes: &sourceActiveAttributes,
			},
			wantHasSockets: true, wantHasAttributes: true,
			wantSockets: destActiveSockets, wantAttributes: destActiveAttributes,
		},
		{
			name: "explicit-zero destination wins over active source",
			destination: inventory.ItemInstance{
				ID: 511, Vnum: 27001, Count: 4, Slot: 5,
				Sockets: &destZeroSockets, Attributes: &destZeroAttributes,
			},
			source: inventory.ItemInstance{
				ID: 512, Vnum: 27001, Count: 3, Slot: 7,
				Sockets: &sourceActiveSockets, Attributes: &sourceActiveAttributes,
			},
			wantHasSockets: true, wantHasAttributes: true,
			wantSockets: destZeroSockets, wantAttributes: destZeroAttributes,
		},
		{
			name: "omitted destination stays omitted",
			destination: inventory.ItemInstance{
				ID: 521, Vnum: 27001, Count: 4, Slot: 5,
			},
			source: inventory.ItemInstance{
				ID: 522, Vnum: 27001, Count: 3, Slot: 7,
				Sockets: &sourceActiveSockets, Attributes: &sourceActiveAttributes,
			},
			wantHasSockets: false, wantHasAttributes: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items := []inventory.ItemInstance{tc.destination}
			display := exchangeDisplayedItem{ItemID: tc.source.ID, Vnum: tc.source.Vnum, Count: tc.source.Count, Slot: tc.source.Slot}
			if !exchangePlaceIncomingDisplayedItemPreferringSlots(&items, display, template, nil, tc.source) {
				t.Fatal("expected exchange compatible merge placement to succeed")
			}
			if len(items) != 1 {
				t.Fatalf("expected count-only merge without fresh cell, got %#v", items)
			}
			got := items[0]
			if got.ID != tc.destination.ID || got.Slot != tc.destination.Slot || got.Count != tc.destination.Count+tc.source.Count {
				t.Fatalf("unexpected merged destination identity/count: %+v", got)
			}
			if got.HasSockets() != tc.wantHasSockets {
				t.Fatalf("HasSockets=%v want %v", got.HasSockets(), tc.wantHasSockets)
			}
			if got.HasAttributes() != tc.wantHasAttributes {
				t.Fatalf("HasAttributes=%v want %v", got.HasAttributes(), tc.wantHasAttributes)
			}
			if tc.wantHasSockets {
				if got.Sockets == nil || *got.Sockets != tc.wantSockets {
					t.Fatalf("expected destination sockets %+v, got %#v", tc.wantSockets, got.Sockets)
				}
				if got.Sockets == tc.source.Sockets {
					t.Fatal("merged destination sockets aliased discarded source")
				}
			} else if got.Sockets != nil {
				t.Fatalf("expected omitted destination sockets, got %#v", got.Sockets)
			}
			if tc.wantHasAttributes {
				if got.Attributes == nil || *got.Attributes != tc.wantAttributes {
					t.Fatalf("expected destination attributes %+v, got %#v", tc.wantAttributes, got.Attributes)
				}
				if got.Attributes == tc.source.Attributes {
					t.Fatal("merged destination attributes aliased discarded source")
				}
			} else if got.Attributes != nil {
				t.Fatalf("expected omitted destination attributes, got %#v", got.Attributes)
			}
		})
	}

	t.Run("free-cell preserve stays source-preserving", func(t *testing.T) {
		items := []inventory.ItemInstance{}
		sourceSockets := inventory.SocketValues{11, 0, -3}
		sourceAttributes := inventory.AttributeValues{{Type: 4, Value: 55}}
		source := inventory.ItemInstance{
			ID: 531, Vnum: 27001, Count: 3, Slot: 7,
			Sockets: &sourceSockets, Attributes: &sourceAttributes,
		}
		display := exchangeDisplayedItem{ItemID: source.ID, Vnum: source.Vnum, Count: source.Count, Slot: source.Slot}
		if !exchangePlaceIncomingDisplayedItemPreferringSlots(&items, display, template, []inventory.SlotIndex{8}, source) {
			t.Fatal("expected exchange free-cell placement to succeed")
		}
		if len(items) != 1 || items[0].ID != 531 || items[0].Slot != 8 || items[0].Count != 3 {
			t.Fatalf("unexpected free-cell placement: %#v", items)
		}
		if !items[0].HasSockets() || *items[0].Sockets != sourceSockets {
			t.Fatalf("expected source-preserving free-cell sockets, got %#v", items[0].Sockets)
		}
		if !items[0].HasAttributes() || *items[0].Attributes != sourceAttributes {
			t.Fatalf("expected source-preserving free-cell attributes, got %#v", items[0].Attributes)
		}
		if items[0].Sockets == source.Sockets || items[0].Attributes == source.Attributes {
			t.Fatal("expected free-cell placement to clone source presence independently")
		}
	})

	t.Run("locked compatible skipped", func(t *testing.T) {
		destSockets := inventory.SocketValues{7, 0, 9}
		sourceSockets := inventory.SocketValues{1, 2, 3}
		items := []inventory.ItemInstance{{
			ID: 541, Vnum: 27001, Count: 4, Slot: 5, Locked: true,
			Sockets: &destSockets,
		}}
		source := inventory.ItemInstance{
			ID: 542, Vnum: 27001, Count: 3, Slot: 7,
			Sockets: &sourceSockets,
		}
		display := exchangeDisplayedItem{ItemID: source.ID, Vnum: source.Vnum, Count: source.Count, Slot: source.Slot}
		if !exchangePlaceIncomingDisplayedItemPreferringSlots(&items, display, template, nil, source) {
			t.Fatal("expected locked destination to be skipped for free-cell placement")
		}
		if len(items) != 2 {
			t.Fatalf("expected locked cell preserved beside free-cell place, got %#v", items)
		}
		var locked, placed *inventory.ItemInstance
		for i := range items {
			switch items[i].ID {
			case 541:
				locked = &items[i]
			case 542:
				placed = &items[i]
			}
		}
		if locked == nil || !locked.Locked || locked.Count != 4 || !locked.HasSockets() || *locked.Sockets != destSockets {
			t.Fatalf("locked destination mutated: %#v", items)
		}
		if placed == nil || !placed.HasSockets() || *placed.Sockets != sourceSockets {
			t.Fatalf("expected source-preserving free-cell after locked skip: %#v", items)
		}
	})

	t.Run("already-full compatible rejects without mutation when no free cell", func(t *testing.T) {
		destSockets := inventory.SocketValues{7, 0, 9}
		sourceSockets := inventory.SocketValues{1, 2, 3}
		items := []inventory.ItemInstance{{
			ID: 551, Vnum: 27001, Count: 200, Slot: 5,
			Sockets: &destSockets,
		}}
		for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
			if slot == 5 {
				continue
			}
			items = append(items, inventory.ItemInstance{ID: uint64(2000 + slot), Vnum: 28000 + uint32(slot), Count: 1, Slot: slot})
		}
		before := append([]inventory.ItemInstance(nil), items...)
		source := inventory.ItemInstance{
			ID: 552, Vnum: 27001, Count: 2, Slot: 7,
			Sockets: &sourceSockets,
		}
		display := exchangeDisplayedItem{ItemID: source.ID, Vnum: source.Vnum, Count: source.Count, Slot: source.Slot}
		if exchangePlaceIncomingDisplayedItemPreferringSlots(&items, display, template, nil, source) {
			t.Fatal("expected already-full compatible exchange merge without free capacity to fail closed")
		}
		if !reflect.DeepEqual(items, before) {
			t.Fatalf("already-full exchange merge mutated working inventory:\ngot:  %#v\nwant: %#v", items, before)
		}
	})
}
