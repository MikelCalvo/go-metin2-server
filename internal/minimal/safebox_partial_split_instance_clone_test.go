package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

func TestSafeboxWholeStackRelocateItemClonesPresenceIndependently(t *testing.T) {
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
			destination, ok := safeboxWholeStackRelocateItem(source, 3)
			if !ok {
				t.Fatal("expected whole-stack relocate helper to succeed")
			}
			if destination.ID != 820 || destination.Vnum != 27001 || destination.Count != 5 || destination.Slot != 3 {
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
					t.Fatal("mutating destination sockets aliased the source seed")
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
					t.Fatal("mutating destination attributes aliased the source seed")
				}
			} else if destination.Attributes != nil {
				t.Fatalf("expected omitted destination attributes, got %#v", destination.Attributes)
			}
		})
	}
}

func TestSafeboxWholeStackRelocateItemRejectsZeroCount(t *testing.T) {
	source := inventory.ItemInstance{ID: 820, Vnum: 27001, Count: 0, Slot: 0}
	if _, ok := safeboxWholeStackRelocateItem(source, 3); ok {
		t.Fatal("expected zero-count whole-stack relocate helper to fail closed")
	}
}

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

func TestSafeboxPartialSplitRemainderItemClonesPresenceIndependently(t *testing.T) {
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
			remainder, ok := safeboxPartialSplitRemainderItem(source, 2)
			if !ok {
				t.Fatal("expected partial-split remainder helper to succeed")
			}
			if remainder.ID != 820 || remainder.Vnum != 27001 || remainder.Count != 3 || remainder.Slot != 0 {
				t.Fatalf("unexpected remainder identity: %+v", remainder)
			}
			if remainder.HasSockets() != source.HasSockets() {
				t.Fatalf("remainder HasSockets=%v want %v", remainder.HasSockets(), source.HasSockets())
			}
			if remainder.HasAttributes() != source.HasAttributes() {
				t.Fatalf("remainder HasAttributes=%v want %v", remainder.HasAttributes(), source.HasAttributes())
			}
			if source.HasSockets() {
				if remainder.Sockets == source.Sockets {
					t.Fatal("expected remainder sockets pointer to be independent of the pre-split source")
				}
				if *remainder.Sockets != *source.Sockets {
					t.Fatalf("expected remainder sockets %+v, got %+v", *source.Sockets, *remainder.Sockets)
				}
				(*remainder.Sockets)[0] = 99
				if (*source.Sockets)[0] == 99 {
					t.Fatal("mutating remainder sockets aliased the pre-split source pointer")
				}
			} else if remainder.Sockets != nil {
				t.Fatalf("expected omitted remainder sockets, got %#v", remainder.Sockets)
			}
			if source.HasAttributes() {
				if remainder.Attributes == source.Attributes {
					t.Fatal("expected remainder attributes pointer to be independent of the pre-split source")
				}
				if *remainder.Attributes != *source.Attributes {
					t.Fatalf("expected remainder attributes %+v, got %+v", *source.Attributes, *remainder.Attributes)
				}
				(*remainder.Attributes)[0].Value = 99
				if (*source.Attributes)[0].Value == 99 {
					t.Fatal("mutating remainder attributes aliased the pre-split source pointer")
				}
			} else if remainder.Attributes != nil {
				t.Fatalf("expected omitted remainder attributes, got %#v", remainder.Attributes)
			}
		})
	}
}

func TestSafeboxPartialSplitRemainderItemRejectsWholeStackOrZero(t *testing.T) {
	source := inventory.ItemInstance{ID: 820, Vnum: 27001, Count: 5, Slot: 0}
	if _, ok := safeboxPartialSplitRemainderItem(source, 0); ok {
		t.Fatal("expected zero-count split remainder helper to fail closed")
	}
	if _, ok := safeboxPartialSplitRemainderItem(source, 5); ok {
		t.Fatal("expected whole-stack split remainder helper to fail closed")
	}
}

func TestSafeboxPartialMergeRemainderItemClonesPresenceIndependently(t *testing.T) {
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
		{name: "omitted presence", sockets: nil, attributes: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := inventory.ItemInstance{
				ID:         821,
				Vnum:       27001,
				Count:      4,
				Slot:       0,
				Sockets:    tc.sockets,
				Attributes: tc.attributes,
			}
			remainder, ok := safeboxPartialMergeRemainderItem(source, 2)
			if !ok {
				t.Fatal("expected partial-merge remainder helper to succeed")
			}
			if remainder.ID != 821 || remainder.Vnum != 27001 || remainder.Count != 2 || remainder.Slot != 0 {
				t.Fatalf("unexpected remainder identity: %+v", remainder)
			}
			if remainder.HasSockets() != source.HasSockets() {
				t.Fatalf("remainder HasSockets=%v want %v", remainder.HasSockets(), source.HasSockets())
			}
			if remainder.HasAttributes() != source.HasAttributes() {
				t.Fatalf("remainder HasAttributes=%v want %v", remainder.HasAttributes(), source.HasAttributes())
			}
			if source.HasSockets() {
				if remainder.Sockets == source.Sockets {
					t.Fatal("expected remainder sockets pointer to be independent of the pre-merge source")
				}
				if *remainder.Sockets != *source.Sockets {
					t.Fatalf("expected remainder sockets %+v, got %+v", *source.Sockets, *remainder.Sockets)
				}
				(*remainder.Sockets)[0] = 99
				if (*source.Sockets)[0] == 99 {
					t.Fatal("mutating remainder sockets aliased the pre-merge source pointer")
				}
			} else if remainder.Sockets != nil {
				t.Fatalf("expected omitted remainder sockets, got %#v", remainder.Sockets)
			}
			if source.HasAttributes() {
				if remainder.Attributes == source.Attributes {
					t.Fatal("expected remainder attributes pointer to be independent of the pre-merge source")
				}
				if *remainder.Attributes != *source.Attributes {
					t.Fatalf("expected remainder attributes %+v, got %+v", *source.Attributes, *remainder.Attributes)
				}
				(*remainder.Attributes)[0].Value = 99
				if (*source.Attributes)[0].Value == 99 {
					t.Fatal("mutating remainder attributes aliased the pre-merge source pointer")
				}
			} else if remainder.Attributes != nil {
				t.Fatalf("expected omitted remainder attributes, got %#v", remainder.Attributes)
			}
		})
	}
}

func TestSafeboxPartialMergeRemainderItemRejectsWholeStackOrZero(t *testing.T) {
	source := inventory.ItemInstance{ID: 821, Vnum: 27001, Count: 4, Slot: 0}
	if _, ok := safeboxPartialMergeRemainderItem(source, 0); ok {
		t.Fatal("expected zero-count merge remainder helper to fail closed")
	}
	if _, ok := safeboxPartialMergeRemainderItem(source, 4); ok {
		t.Fatal("expected whole-stack merge remainder helper to fail closed")
	}
}
