package player

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestRuntimeMoveInventoryItemCountPartialSplitClonesInstancePresenceIndependently(t *testing.T) {
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
			sourceSockets := cloneOptionalSockets(tc.sockets)
			sourceAttributes := cloneOptionalAttributes(tc.attributes)
			persisted := loginticket.Character{
				ID:       0x01030981,
				VID:      0x02040981,
				Name:     "MoveSplitClone",
				MapIndex: 1,
				X:        1300,
				Y:        2300,
				Empire:   2,
				Gold:     125000,
				Inventory: []inventory.ItemInstance{{
					ID:         981,
					Vnum:       27001,
					Count:      5,
					Slot:       5,
					Sockets:    sourceSockets,
					Attributes: sourceAttributes,
				}},
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "move-split-clone", CharacterIndex: 0})

			result, ok := runtime.MoveInventoryItemCount(5, 8, 2)
			if !ok {
				t.Fatal("expected partial empty-destination ITEM_MOVE split to succeed")
			}
			if !result.Changed || !result.FromOccupied || !result.ToOccupied || result.CountOnly {
				t.Fatalf("expected split result to refresh both cells without count-only merge flags, got %+v", result)
			}
			if result.FromItem.ID != 981 || result.FromItem.Count != 3 || result.FromItem.Slot != 5 {
				t.Fatalf("unexpected source remainder: %+v", result.FromItem)
			}
			if result.ToItem.ID == 0 || result.ToItem.ID == 981 || result.ToItem.Count != 2 || result.ToItem.Slot != 8 || result.ToItem.Vnum != 27001 {
				t.Fatalf("expected fresh destination split identity, got %+v", result.ToItem)
			}

			assertIndependentPresenceClone(t, result.FromItem, result.ToItem, tc.sockets != nil, tc.attributes != nil, sourceSockets, sourceAttributes)

			live := runtime.LiveInventory()
			if len(live) != 2 {
				t.Fatalf("expected two live stacks after partial split, got %#v", live)
			}
			var liveSource, liveDest inventory.ItemInstance
			for _, item := range live {
				switch item.Slot {
				case 5:
					liveSource = item
				case 8:
					liveDest = item
				}
			}
			if liveSource.ID != 981 || liveSource.Count != 3 {
				t.Fatalf("unexpected live source remainder: %+v", liveSource)
			}
			if liveDest.ID != result.ToItem.ID || liveDest.Count != 2 {
				t.Fatalf("unexpected live destination split: %+v", liveDest)
			}
			assertIndependentPresenceClone(t, liveSource, liveDest, tc.sockets != nil, tc.attributes != nil, sourceSockets, sourceAttributes)

			if !reflectDeepEqualInventoryPresence(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
				t.Fatalf("expected persisted inventory boundary unchanged after live split, got %#v want %#v",
					runtime.PersistedSnapshot().Inventory, persisted.Inventory)
			}
		})
	}
}

func cloneOptionalSockets(in *inventory.SocketValues) *inventory.SocketValues {
	if in == nil {
		return nil
	}
	copied := *in
	return &copied
}

func cloneOptionalAttributes(in *inventory.AttributeValues) *inventory.AttributeValues {
	if in == nil {
		return nil
	}
	copied := *in
	return &copied
}

func assertIndependentPresenceClone(
	t *testing.T,
	source inventory.ItemInstance,
	destination inventory.ItemInstance,
	wantSockets bool,
	wantAttributes bool,
	wantSocketValues *inventory.SocketValues,
	wantAttributeValues *inventory.AttributeValues,
) {
	t.Helper()
	if destination.HasSockets() != wantSockets {
		t.Fatalf("destination HasSockets=%v want %v", destination.HasSockets(), wantSockets)
	}
	if source.HasSockets() != wantSockets {
		t.Fatalf("source HasSockets=%v want %v", source.HasSockets(), wantSockets)
	}
	if destination.HasAttributes() != wantAttributes {
		t.Fatalf("destination HasAttributes=%v want %v", destination.HasAttributes(), wantAttributes)
	}
	if source.HasAttributes() != wantAttributes {
		t.Fatalf("source HasAttributes=%v want %v", source.HasAttributes(), wantAttributes)
	}
	if wantSockets {
		if destination.Sockets == nil || wantSocketValues == nil || *destination.Sockets != *wantSocketValues {
			t.Fatalf("expected destination sockets %+v, got %#v", wantSocketValues, destination.Sockets)
		}
		if source.Sockets == nil || *source.Sockets != *wantSocketValues {
			t.Fatalf("expected source sockets %+v, got %#v", wantSocketValues, source.Sockets)
		}
		if destination.Sockets == source.Sockets {
			t.Fatal("expected destination sockets pointer to be independent of source")
		}
		original := (*destination.Sockets)[0]
		(*destination.Sockets)[0] = 99
		if (*source.Sockets)[0] == 99 {
			t.Fatal("mutating destination sockets aliased the source remainder")
		}
		(*destination.Sockets)[0] = original
	} else if destination.Sockets != nil || source.Sockets != nil {
		t.Fatalf("expected omitted sockets, source=%#v destination=%#v", source.Sockets, destination.Sockets)
	}
	if wantAttributes {
		if destination.Attributes == nil || wantAttributeValues == nil || *destination.Attributes != *wantAttributeValues {
			t.Fatalf("expected destination attributes %+v, got %#v", wantAttributeValues, destination.Attributes)
		}
		if source.Attributes == nil || *source.Attributes != *wantAttributeValues {
			t.Fatalf("expected source attributes %+v, got %#v", wantAttributeValues, source.Attributes)
		}
		if destination.Attributes == source.Attributes {
			t.Fatal("expected destination attributes pointer to be independent of source")
		}
		original := (*destination.Attributes)[0].Value
		(*destination.Attributes)[0].Value = 99
		if (*source.Attributes)[0].Value == 99 {
			t.Fatal("mutating destination attributes aliased the source remainder")
		}
		(*destination.Attributes)[0].Value = original
	} else if destination.Attributes != nil || source.Attributes != nil {
		t.Fatalf("expected omitted attributes, source=%#v destination=%#v", source.Attributes, destination.Attributes)
	}
}

func reflectDeepEqualInventoryPresence(got, want []inventory.ItemInstance) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i].ID != want[i].ID || got[i].Vnum != want[i].Vnum || got[i].Count != want[i].Count || got[i].Slot != want[i].Slot {
			return false
		}
		if got[i].HasSockets() != want[i].HasSockets() {
			return false
		}
		if got[i].HasSockets() && (got[i].Sockets == nil || want[i].Sockets == nil || *got[i].Sockets != *want[i].Sockets) {
			return false
		}
		if got[i].HasAttributes() != want[i].HasAttributes() {
			return false
		}
		if got[i].HasAttributes() && (got[i].Attributes == nil || want[i].Attributes == nil || *got[i].Attributes != *want[i].Attributes) {
			return false
		}
	}
	return true
}
