package player

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestRuntimeUseItemOnItemPreservesInstancePresenceIndependently(t *testing.T) {
	sourceActiveSockets := inventory.SocketValues{11, 0, -3}
	sourceActiveAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	destActiveSockets := inventory.SocketValues{7, 0, 9}
	destActiveAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}
	template := itemcatalog.Template{
		Vnum:      27001,
		Name:      "UseToItem Presence Potion",
		Stackable: true,
		MaxCount:  10,
		Sockets:   itemcatalog.SocketValues{-11, 202, -303},
		Attributes: itemcatalog.AttributeValues{
			{Type: 12, Value: 34},
			{Type: 15, Value: -9},
		},
	}

	cases := []struct {
		name             string
		sourceCount      uint16
		destCount        uint16
		sourceSockets    *inventory.SocketValues
		sourceAttributes *inventory.AttributeValues
		destSockets      *inventory.SocketValues
		destAttributes   *inventory.AttributeValues
		wantSourceRemain bool
		wantSourceCount  uint16
		wantDestCount    uint16
	}{
		{
			name:             "partial active source remainder and destination wins",
			sourceCount:      7,
			destCount:        8,
			sourceSockets:    &sourceActiveSockets,
			sourceAttributes: &sourceActiveAttributes,
			destSockets:      &destActiveSockets,
			destAttributes:   &destActiveAttributes,
			wantSourceRemain: true,
			wantSourceCount:  5,
			wantDestCount:    10,
		},
		{
			name:             "partial explicit-zero remainder and destination wins",
			sourceCount:      7,
			destCount:        8,
			sourceSockets:    &zeroSockets,
			sourceAttributes: &zeroAttributes,
			destSockets:      &zeroSockets,
			destAttributes:   &zeroAttributes,
			wantSourceRemain: true,
			wantSourceCount:  5,
			wantDestCount:    10,
		},
		{
			name:             "partial omitted remainder and destination stay omitted",
			sourceCount:      7,
			destCount:        8,
			wantSourceRemain: true,
			wantSourceCount:  5,
			wantDestCount:    10,
		},
		{
			name:             "full merge active destination wins over different source",
			sourceCount:      4,
			destCount:        6,
			sourceSockets:    &sourceActiveSockets,
			sourceAttributes: &sourceActiveAttributes,
			destSockets:      &destActiveSockets,
			destAttributes:   &destActiveAttributes,
			wantDestCount:    10,
		},
		{
			name:             "full merge explicit-zero destination wins over active source",
			sourceCount:      4,
			destCount:        6,
			sourceSockets:    &sourceActiveSockets,
			sourceAttributes: &sourceActiveAttributes,
			destSockets:      &zeroSockets,
			destAttributes:   &zeroAttributes,
			wantDestCount:    10,
		},
		{
			name:             "full merge omitted destination stays omitted",
			sourceCount:      4,
			destCount:        6,
			sourceSockets:    &sourceActiveSockets,
			sourceAttributes: &sourceActiveAttributes,
			wantDestCount:    10,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			character := loginticket.Character{
				ID:   0x01030a01,
				VID:  0x02040a01,
				Name: "UseToItemPresence",
				Inventory: []inventory.ItemInstance{
					{ID: 941, Vnum: 27001, Count: tc.sourceCount, Slot: 5, Sockets: tc.sourceSockets, Attributes: tc.sourceAttributes},
					{ID: 942, Vnum: 27001, Count: tc.destCount, Slot: 6, Sockets: tc.destSockets, Attributes: tc.destAttributes},
				},
			}
			runtime := NewRuntime(character, SessionLink{Login: "use-to-item-presence", CharacterIndex: 0})
			if len(runtime.liveInventory) != 2 {
				t.Fatalf("expected two live inventory items before merge, got %#v", runtime.liveInventory)
			}
			beforeSource := runtime.liveInventory[findInventorySlot(runtime.liveInventory, 5)]
			beforeDest := runtime.liveInventory[findInventorySlot(runtime.liveInventory, 6)]

			result, ok := runtime.UseItemOnItem(5, 6, template)
			if !ok {
				t.Fatal("expected item-use-to-item merge to succeed")
			}
			if !result.Changed || result.From != 5 || result.To != 6 || !result.ToOccupied || !result.CountOnly {
				t.Fatalf("unexpected merge result flags: %+v", result)
			}
			if result.FromOccupied != tc.wantSourceRemain {
				t.Fatalf("unexpected source occupancy: %+v", result)
			}
			if result.ToItem.ID != 942 || result.ToItem.Count != tc.wantDestCount || result.ToItem.Slot != 6 {
				t.Fatalf("unexpected merge target item: %+v", result.ToItem)
			}
			assertIndependentUseToItemPresence(t, beforeDest, result.ToItem, "merge destination")
			if result.ToItem.Sockets == beforeSource.Sockets && beforeSource.Sockets != nil {
				t.Fatal("merged destination sockets aliased discarded source")
			}
			if result.ToItem.Attributes == beforeSource.Attributes && beforeSource.Attributes != nil {
				t.Fatal("merged destination attributes aliased discarded source")
			}

			if tc.wantSourceRemain {
				if result.FromItem.ID != 941 || result.FromItem.Count != tc.wantSourceCount || result.FromItem.Slot != 5 {
					t.Fatalf("unexpected source remainder: %+v", result.FromItem)
				}
				assertIndependentUseToItemPresence(t, beforeSource, result.FromItem, "merge source remainder")
			}

			live := runtime.LiveInventory()
			var liveSource, liveDest *inventory.ItemInstance
			for i := range live {
				switch live[i].ID {
				case 941:
					liveSource = &live[i]
				case 942:
					liveDest = &live[i]
				}
			}
			if liveDest == nil || liveDest.Count != tc.wantDestCount || liveDest.Slot != 6 {
				t.Fatalf("unexpected live destination after merge: %#v", live)
			}
			assertIndependentUseToItemPresence(t, beforeDest, *liveDest, "live destination")
			if tc.wantSourceRemain {
				if liveSource == nil || liveSource.Count != tc.wantSourceCount || liveSource.Slot != 5 {
					t.Fatalf("unexpected live source remainder: %#v", live)
				}
				assertIndependentUseToItemPresence(t, beforeSource, *liveSource, "live source remainder")
			} else if liveSource != nil {
				t.Fatalf("expected full merge to remove source, got %#v", live)
			}
			if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, character.Inventory) {
				t.Fatalf("player mutation boundary should not rewrite persisted inventory directly, got %#v", runtime.PersistedSnapshot().Inventory)
			}
		})
	}
}

func assertIndependentUseToItemPresence(t *testing.T, before, got inventory.ItemInstance, label string) {
	t.Helper()
	if got.HasSockets() != before.HasSockets() {
		t.Fatalf("%s HasSockets=%v want %v", label, got.HasSockets(), before.HasSockets())
	}
	if got.HasAttributes() != before.HasAttributes() {
		t.Fatalf("%s HasAttributes=%v want %v", label, got.HasAttributes(), before.HasAttributes())
	}
	if before.HasSockets() {
		if got.Sockets == before.Sockets {
			t.Fatalf("%s expected independent sockets clone from the pre-merge live inventory pointer", label)
		}
		want := *before.Sockets
		if *got.Sockets != want {
			t.Fatalf("%s expected sockets %+v, got %+v", label, want, *got.Sockets)
		}
		(*got.Sockets)[0] = 99
		if (*before.Sockets)[0] == 99 {
			t.Fatalf("%s mutating sockets aliased the pre-merge live inventory pointer", label)
		}
		*got.Sockets = want
	} else if got.Sockets != nil {
		t.Fatalf("%s expected omitted sockets, got %#v", label, got.Sockets)
	}
	if before.HasAttributes() {
		if got.Attributes == before.Attributes {
			t.Fatalf("%s expected independent attributes clone from the pre-merge live inventory pointer", label)
		}
		want := *before.Attributes
		if *got.Attributes != want {
			t.Fatalf("%s expected attributes %+v, got %+v", label, want, *got.Attributes)
		}
		(*got.Attributes)[0].Value = 99
		if (*before.Attributes)[0].Value == 99 {
			t.Fatalf("%s mutating attributes aliased the pre-merge live inventory pointer", label)
		}
		*got.Attributes = want
	} else if got.Attributes != nil {
		t.Fatalf("%s expected omitted attributes, got %#v", label, got.Attributes)
	}
}
