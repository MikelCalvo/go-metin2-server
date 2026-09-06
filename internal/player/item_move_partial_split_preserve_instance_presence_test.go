package player

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestRuntimeMoveInventoryItemCountPartialSplitRemainderPreservesInstancePresenceIndependently(t *testing.T) {
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
			character := loginticket.Character{
				ID:   0x01030991,
				VID:  0x02040991,
				Name: "MovePartialSplitRemainder",
				Inventory: []inventory.ItemInstance{{
					ID:         991,
					Vnum:       27001,
					Count:      5,
					Slot:       5,
					Sockets:    tc.sockets,
					Attributes: tc.attributes,
				}},
			}
			runtime := NewRuntime(character, SessionLink{Login: "move-partial-split-remainder", CharacterIndex: 0})
			if len(runtime.liveInventory) != 1 {
				t.Fatalf("expected one live inventory item before split, got %#v", runtime.liveInventory)
			}
			beforeSource := runtime.liveInventory[findInventorySlot(runtime.liveInventory, 5)]

			result, ok := runtime.MoveInventoryItemCount(5, 8, 2)
			if !ok {
				t.Fatal("expected partial empty-destination ITEM_MOVE split to succeed")
			}
			if !result.Changed || !result.FromOccupied || !result.ToOccupied || result.CountOnly {
				t.Fatalf("expected split result to refresh both cells without count-only merge flags, got %+v", result)
			}
			if result.FromItem.ID != 991 || result.FromItem.Count != 3 || result.FromItem.Slot != 5 {
				t.Fatalf("unexpected source remainder: %+v", result.FromItem)
			}
			if result.ToItem.ID == 0 || result.ToItem.ID == 991 || result.ToItem.Count != 2 || result.ToItem.Slot != 8 || result.ToItem.Vnum != 27001 {
				t.Fatalf("expected fresh destination split identity, got %+v", result.ToItem)
			}
			assertIndependentSplitRemainderPresence(t, beforeSource, result.FromItem, "split source remainder")

			live := runtime.liveInventory
			var liveSource, liveDest *inventory.ItemInstance
			for i := range live {
				switch live[i].Slot {
				case 5:
					liveSource = &live[i]
				case 8:
					liveDest = &live[i]
				}
			}
			if liveSource == nil || liveSource.ID != 991 || liveSource.Count != 3 {
				t.Fatalf("unexpected live source remainder: %#v", live)
			}
			if liveDest == nil || liveDest.ID != result.ToItem.ID || liveDest.Count != 2 {
				t.Fatalf("unexpected live destination split: %#v", live)
			}
			assertIndependentSplitRemainderPresence(t, beforeSource, *liveSource, "live source remainder")
			assertIndependentPresenceClone(t, *liveSource, *liveDest, tc.sockets != nil, tc.attributes != nil, tc.sockets, tc.attributes)

			if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, character.Inventory) {
				t.Fatalf("player mutation boundary should not rewrite persisted inventory directly, got %#v", runtime.PersistedSnapshot().Inventory)
			}
		})
	}
}

func assertIndependentSplitRemainderPresence(t *testing.T, before, remainder inventory.ItemInstance, label string) {
	t.Helper()
	if remainder.HasSockets() != before.HasSockets() {
		t.Fatalf("%s HasSockets=%v want %v", label, remainder.HasSockets(), before.HasSockets())
	}
	if remainder.HasAttributes() != before.HasAttributes() {
		t.Fatalf("%s HasAttributes=%v want %v", label, remainder.HasAttributes(), before.HasAttributes())
	}
	if before.HasSockets() {
		if remainder.Sockets == before.Sockets {
			t.Fatalf("%s expected independent sockets clone from the pre-split live inventory pointer", label)
		}
		want := *before.Sockets
		if *remainder.Sockets != want {
			t.Fatalf("%s expected sockets %+v, got %+v", label, want, *remainder.Sockets)
		}
		(*remainder.Sockets)[0] = 99
		if (*before.Sockets)[0] == 99 {
			t.Fatalf("%s mutating remainder sockets aliased the pre-split live inventory pointer", label)
		}
		*remainder.Sockets = want
	} else if remainder.Sockets != nil {
		t.Fatalf("%s expected omitted sockets, got %#v", label, remainder.Sockets)
	}
	if before.HasAttributes() {
		if remainder.Attributes == before.Attributes {
			t.Fatalf("%s expected independent attributes clone from the pre-split live inventory pointer", label)
		}
		want := *before.Attributes
		if *remainder.Attributes != want {
			t.Fatalf("%s expected attributes %+v, got %+v", label, want, *remainder.Attributes)
		}
		(*remainder.Attributes)[0].Value = 99
		if (*before.Attributes)[0].Value == 99 {
			t.Fatalf("%s mutating remainder attributes aliased the pre-split live inventory pointer", label)
		}
		*remainder.Attributes = want
	} else if remainder.Attributes != nil {
		t.Fatalf("%s expected omitted attributes, got %#v", label, remainder.Attributes)
	}
}
