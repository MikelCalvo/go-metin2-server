package player

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestRuntimeMoveInventoryItemPartialMergePreservesInstancePresenceIndependently(t *testing.T) {
	sourceActiveSockets := inventory.SocketValues{11, 0, -3}
	sourceActiveAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	destActiveSockets := inventory.SocketValues{7, 0, 9}
	destActiveAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}

	cases := []struct {
		name             string
		sourceSockets    *inventory.SocketValues
		sourceAttributes *inventory.AttributeValues
		destSockets      *inventory.SocketValues
		destAttributes   *inventory.AttributeValues
	}{
		{
			name:             "active source remainder clones independently of destination wins",
			sourceSockets:    &sourceActiveSockets,
			sourceAttributes: &sourceActiveAttributes,
			destSockets:      &destActiveSockets,
			destAttributes:   &destActiveAttributes,
		},
		{
			name:             "explicit-zero source remainder clones independently",
			sourceSockets:    &zeroSockets,
			sourceAttributes: &zeroAttributes,
			destSockets:      &zeroSockets,
			destAttributes:   &zeroAttributes,
		},
		{
			name: "omitted source remainder stays omitted",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			character := loginticket.Character{
				ID:   0x01030b01,
				VID:  0x02040b01,
				Name: "MovePartialMergePresence",
				Inventory: []inventory.ItemInstance{
					{ID: 961, Vnum: 27001, Count: 4, Slot: 5, Sockets: tc.sourceSockets, Attributes: tc.sourceAttributes},
					{ID: 962, Vnum: 27001, Count: 8, Slot: 8, Sockets: tc.destSockets, Attributes: tc.destAttributes},
				},
			}
			movers := []struct {
				name string
				move func(*Runtime) (inventory.MoveResult, bool)
			}{
				{name: "counted", move: func(runtime *Runtime) (inventory.MoveResult, bool) {
					return runtime.MoveInventoryItemCountBounded(5, 8, 2, 10)
				}},
				{name: "zero-count", move: func(runtime *Runtime) (inventory.MoveResult, bool) {
					return runtime.MoveInventoryItemBounded(5, 8, 10)
				}},
			}
			for _, mover := range movers {
				t.Run(mover.name, func(t *testing.T) {
					runtime := NewRuntime(character, SessionLink{Login: "move-partial-merge-presence", CharacterIndex: 0})
					if len(runtime.liveInventory) != 2 {
						t.Fatalf("expected two live inventory items before merge, got %#v", runtime.liveInventory)
					}
					beforeSource := runtime.liveInventory[findInventorySlot(runtime.liveInventory, 5)]
					beforeDest := runtime.liveInventory[findInventorySlot(runtime.liveInventory, 8)]

					result, ok := mover.move(runtime)
					if !ok {
						t.Fatal("expected partial compatible item-move merge to succeed")
					}
					if !result.Changed || !result.FromOccupied || !result.ToOccupied || !result.CountOnly {
						t.Fatalf("unexpected partial merge result flags: %+v", result)
					}
					if result.FromItem.ID != 961 || result.FromItem.Count != 2 || result.FromItem.Slot != 5 {
						t.Fatalf("unexpected source remainder: %+v", result.FromItem)
					}
					if result.ToItem.ID != 962 || result.ToItem.Count != 10 || result.ToItem.Slot != 8 {
						t.Fatalf("unexpected merge destination: %+v", result.ToItem)
					}
					assertIndependentMoveMergeRemainderPresence(t, beforeSource, result.FromItem, "merge source remainder")
					if result.ToItem.HasSockets() != beforeDest.HasSockets() {
						t.Fatalf("destination HasSockets=%v want %v", result.ToItem.HasSockets(), beforeDest.HasSockets())
					}
					if result.ToItem.HasAttributes() != beforeDest.HasAttributes() {
						t.Fatalf("destination HasAttributes=%v want %v", result.ToItem.HasAttributes(), beforeDest.HasAttributes())
					}

					live := runtime.liveInventory
					var liveSource, liveDest *inventory.ItemInstance
					for i := range live {
						switch live[i].ID {
						case 961:
							liveSource = &live[i]
						case 962:
							liveDest = &live[i]
						}
					}
					if liveSource == nil || liveSource.Count != 2 || liveSource.Slot != 5 {
						t.Fatalf("unexpected live source remainder: %#v", live)
					}
					if liveDest == nil || liveDest.Count != 10 || liveDest.Slot != 8 {
						t.Fatalf("unexpected live destination after merge: %#v", live)
					}
					assertIndependentMoveMergeRemainderPresence(t, beforeSource, *liveSource, "live source remainder")
					if liveDest.HasSockets() != beforeDest.HasSockets() {
						t.Fatalf("live destination HasSockets=%v want %v", liveDest.HasSockets(), beforeDest.HasSockets())
					}
					if liveDest.HasAttributes() != beforeDest.HasAttributes() {
						t.Fatalf("live destination HasAttributes=%v want %v", liveDest.HasAttributes(), beforeDest.HasAttributes())
					}
					if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, character.Inventory) {
						t.Fatalf("player mutation boundary should not rewrite persisted inventory directly, got %#v", runtime.PersistedSnapshot().Inventory)
					}
				})
			}
		})
	}
}

func assertIndependentMoveMergeRemainderPresence(t *testing.T, before, remainder inventory.ItemInstance, label string) {
	t.Helper()
	if remainder.HasSockets() != before.HasSockets() {
		t.Fatalf("%s HasSockets=%v want %v", label, remainder.HasSockets(), before.HasSockets())
	}
	if remainder.HasAttributes() != before.HasAttributes() {
		t.Fatalf("%s HasAttributes=%v want %v", label, remainder.HasAttributes(), before.HasAttributes())
	}
	if before.HasSockets() {
		if remainder.Sockets == before.Sockets {
			t.Fatalf("%s expected independent sockets clone from the pre-merge live inventory pointer", label)
		}
		want := *before.Sockets
		if *remainder.Sockets != want {
			t.Fatalf("%s expected sockets %+v, got %+v", label, want, *remainder.Sockets)
		}
		(*remainder.Sockets)[0] = 99
		if (*before.Sockets)[0] == 99 {
			t.Fatalf("%s mutating remainder sockets aliased the pre-merge live inventory pointer", label)
		}
		*remainder.Sockets = want
	} else if remainder.Sockets != nil {
		t.Fatalf("%s expected omitted sockets, got %#v", label, remainder.Sockets)
	}
	if before.HasAttributes() {
		if remainder.Attributes == before.Attributes {
			t.Fatalf("%s expected independent attributes clone from the pre-merge live inventory pointer", label)
		}
		want := *before.Attributes
		if *remainder.Attributes != want {
			t.Fatalf("%s expected attributes %+v, got %+v", label, want, *remainder.Attributes)
		}
		(*remainder.Attributes)[0].Value = 99
		if (*before.Attributes)[0].Value == 99 {
			t.Fatalf("%s mutating remainder attributes aliased the pre-merge live inventory pointer", label)
		}
		*remainder.Attributes = want
	} else if remainder.Attributes != nil {
		t.Fatalf("%s expected omitted attributes, got %#v", label, remainder.Attributes)
	}
}
