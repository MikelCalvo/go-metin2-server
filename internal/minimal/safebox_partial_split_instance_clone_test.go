package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

func TestSafeboxPartialSplitDestinationItemClonesPresenceIndependently(t *testing.T) {
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
				ID:         820,
				Vnum:       27001,
				Count:      5,
				Slot:       0,
				Sockets:    tc.sockets,
				Attributes: tc.attributes,
			}
			destination, ok := safeboxPartialSplitDestinationItem(source, 821, 2, 2)
			if !ok {
				t.Fatal("expected partial-split destination helper to succeed")
			}
			if destination.ID != 821 || destination.Vnum != 27001 || destination.Count != 2 || destination.Slot != 2 {
				t.Fatalf("unexpected destination identity: %+v", destination)
			}
			if destination.HasSockets() != source.HasSockets() {
				t.Fatalf("destination HasSockets=%v want %v", destination.HasSockets(), source.HasSockets())
			}
			if destination.HasAttributes() != source.HasAttributes() {
				t.Fatalf("destination HasAttributes=%v want %v", destination.HasAttributes(), source.HasAttributes())
			}
			if source.HasSockets() {
				if destination.Sockets == source.Sockets {
					t.Fatal("expected destination sockets pointer to be independent of source")
				}
				if *destination.Sockets != *source.Sockets {
					t.Fatalf("expected destination sockets %+v, got %+v", *source.Sockets, *destination.Sockets)
				}
				(*destination.Sockets)[0] = 99
				if (*source.Sockets)[0] == 99 {
					t.Fatal("mutating destination sockets aliased the source remainder")
				}
			} else if destination.Sockets != nil {
				t.Fatalf("expected omitted destination sockets, got %#v", destination.Sockets)
			}
			if source.HasAttributes() {
				if destination.Attributes == source.Attributes {
					t.Fatal("expected destination attributes pointer to be independent of source")
				}
				if *destination.Attributes != *source.Attributes {
					t.Fatalf("expected destination attributes %+v, got %+v", *source.Attributes, *destination.Attributes)
				}
				(*destination.Attributes)[0].Value = 99
				if (*source.Attributes)[0].Value == 99 {
					t.Fatal("mutating destination attributes aliased the source remainder")
				}
			} else if destination.Attributes != nil {
				t.Fatalf("expected omitted destination attributes, got %#v", destination.Attributes)
			}
		})
	}
}

func TestSafeboxPartialSplitDestinationItemRejectsWholeStackOrZero(t *testing.T) {
	source := inventory.ItemInstance{ID: 820, Vnum: 27001, Count: 5, Slot: 0}
	if _, ok := safeboxPartialSplitDestinationItem(source, 821, 0, 2); ok {
		t.Fatal("expected zero-count split helper to fail closed")
	}
	if _, ok := safeboxPartialSplitDestinationItem(source, 821, 5, 2); ok {
		t.Fatal("expected whole-stack split helper to fail closed")
	}
	if _, ok := safeboxPartialSplitDestinationItem(source, 0, 2, 2); ok {
		t.Fatal("expected zero nextID split helper to fail closed")
	}
}
