package player

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestRuntimeUseItemPreservesInstancePresenceIndependently(t *testing.T) {
	activeSockets := inventory.SocketValues{11, 0, -3}
	activeAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}
	template := itemcatalog.Template{
		Vnum:      27001,
		Name:      "Use Presence Potion",
		Stackable: true,
		MaxCount:  200,
		Sockets:   itemcatalog.SocketValues{-11, 202, -303},
		Attributes: itemcatalog.AttributeValues{
			{Type: 12, Value: 34},
			{Type: 15, Value: -9},
		},
		UseEffect: &itemcatalog.UseEffect{
			PointType:  1,
			PointIndex: 1,
			PointDelta: 50,
			Message:    "consume:27001:+50",
		},
	}

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
				ID:     0x010309e1,
				VID:    0x020409e1,
				Name:   "UsePresence",
				Points: [255]int32{1: 700},
				Inventory: []inventory.ItemInstance{{
					ID:         941,
					Vnum:       27001,
					Count:      3,
					Slot:       5,
					Sockets:    tc.sockets,
					Attributes: tc.attributes,
				}},
			}
			runtime := NewRuntime(character, SessionLink{Login: "use-presence", CharacterIndex: 0})
			if len(runtime.liveInventory) != 1 {
				t.Fatalf("expected one live inventory item before use, got %#v", runtime.liveInventory)
			}
			before := runtime.liveInventory[0]

			result, ok := runtime.UseItem(5, template)
			if !ok {
				t.Fatal("expected partial item use to succeed")
			}
			if result.ItemRemoved || result.Slot != 5 || result.Item.ID != 941 || result.Item.Count != 2 || result.Item.Slot != 5 {
				t.Fatalf("unexpected partial use result: %+v", result)
			}
			assertIndependentUseRemainderPresence(t, before, result.Item, "use result remainder")
			if len(runtime.liveInventory) != 1 {
				t.Fatalf("expected one live inventory item after partial use, got %#v", runtime.liveInventory)
			}
			assertIndependentUseRemainderPresence(t, before, runtime.liveInventory[0], "live remainder")
			if runtime.LiveCharacter().Points[1] != 750 {
				t.Fatalf("expected live points[1] 750 after authored use, got %d", runtime.LiveCharacter().Points[1])
			}
		})
	}
}

func assertIndependentUseRemainderPresence(t *testing.T, before, remainder inventory.ItemInstance, label string) {
	t.Helper()
	if remainder.HasSockets() != before.HasSockets() {
		t.Fatalf("%s HasSockets=%v want %v", label, remainder.HasSockets(), before.HasSockets())
	}
	if remainder.HasAttributes() != before.HasAttributes() {
		t.Fatalf("%s HasAttributes=%v want %v", label, remainder.HasAttributes(), before.HasAttributes())
	}
	if before.HasSockets() {
		if remainder.Sockets == before.Sockets {
			t.Fatalf("%s expected use remainder to clone sockets independently from the pre-use live inventory pointer", label)
		}
		want := *before.Sockets
		if *remainder.Sockets != want {
			t.Fatalf("%s expected remainder sockets %+v, got %+v", label, want, *remainder.Sockets)
		}
		(*remainder.Sockets)[0] = 99
		if (*before.Sockets)[0] == 99 {
			t.Fatalf("%s mutating remainder sockets aliased the pre-use live inventory pointer", label)
		}
		*remainder.Sockets = want
	} else if remainder.Sockets != nil {
		t.Fatalf("%s expected omitted remainder sockets, got %#v", label, remainder.Sockets)
	}
	if before.HasAttributes() {
		if remainder.Attributes == before.Attributes {
			t.Fatalf("%s expected use remainder to clone attributes independently from the pre-use live inventory pointer", label)
		}
		want := *before.Attributes
		if *remainder.Attributes != want {
			t.Fatalf("%s expected remainder attributes %+v, got %+v", label, want, *remainder.Attributes)
		}
		(*remainder.Attributes)[0].Value = 99
		if (*before.Attributes)[0].Value == 99 {
			t.Fatalf("%s mutating remainder attributes aliased the pre-use live inventory pointer", label)
		}
		*remainder.Attributes = want
	} else if remainder.Attributes != nil {
		t.Fatalf("%s expected omitted remainder attributes, got %#v", label, remainder.Attributes)
	}
}
