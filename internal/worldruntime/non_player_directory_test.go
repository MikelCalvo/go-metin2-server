package worldruntime

import "testing"

func TestNonPlayerDirectoryRegistersLooksUpAndRemovesStaticActors(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:   Entity{ID: 1, Kind: EntityKindStaticActor, Name: "VillageGuard"},
		Position: NewPosition(42, 1700, 2800),
		RaceNum:  20300,
	}

	if !directory.Register(actor) {
		t.Fatal("expected static actor registration to succeed")
	}
	lookup, ok := directory.ByEntityID(actor.Entity.ID)
	if !ok || lookup.Entity.Name != "VillageGuard" || lookup.RaceNum != 20300 {
		t.Fatalf("expected static actor lookup to return VillageGuard, got actor=%+v ok=%v", lookup, ok)
	}
	removed, ok := directory.Remove(actor.Entity.ID)
	if !ok || removed.Entity.ID != actor.Entity.ID {
		t.Fatalf("expected static actor remove to return VillageGuard, got actor=%+v ok=%v", removed, ok)
	}
	if _, ok := directory.ByEntityID(actor.Entity.ID); ok {
		t.Fatal("expected static actor lookup to be cleared after removal")
	}
}

func TestNonPlayerDirectoryLooksUpStaticActorsByVisibilityVID(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:   Entity{ID: 7, Kind: EntityKindStaticActor, Name: "VillageGuard"},
		Position: NewPosition(42, 1700, 2800),
		RaceNum:  20300,
	}
	if !directory.Register(actor) {
		t.Fatal("expected static actor registration to succeed")
	}

	lookup, ok := directory.ByVID(7)
	if !ok || lookup.Entity.ID != actor.Entity.ID || lookup.Entity.Name != actor.Entity.Name {
		t.Fatalf("expected static actor VID lookup to return VillageGuard, got actor=%+v ok=%v", lookup, ok)
	}
	if _, ok := directory.ByVID(999); ok {
		t.Fatal("expected missing static actor VID lookup to fail")
	}
}

func TestNonPlayerDirectoryUpdateReplacesStaticActorByEntityID(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:   Entity{ID: 7, Kind: EntityKindStaticActor, Name: "VillageGuard"},
		Position: NewPosition(42, 1700, 2800),
		RaceNum:  20300,
	}
	if !directory.Register(actor) {
		t.Fatal("expected static actor registration to succeed")
	}

	updated := actor
	updated.Entity.Name = "Blacksmith"
	updated.Position = NewPosition(99, 900, 1200)
	updated.RaceNum = 20016
	if !directory.Update(updated) {
		t.Fatal("expected static actor update to succeed")
	}

	lookup, ok := directory.ByEntityID(actor.Entity.ID)
	if !ok {
		t.Fatal("expected static actor lookup to succeed after update")
	}
	if lookup.Entity.Name != "Blacksmith" || lookup.Position != NewPosition(99, 900, 1200) || lookup.RaceNum != 20016 {
		t.Fatalf("unexpected static actor after update: %+v", lookup)
	}
}

func TestNonPlayerDirectoryUpdateClearsVisibilityVIDLookupWhenActorStopsBeingEncodable(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:   Entity{ID: 7, Kind: EntityKindStaticActor, Name: "VillageGuard"},
		Position: NewPosition(42, 1700, 2800),
		RaceNum:  20300,
	}
	if !directory.Register(actor) {
		t.Fatal("expected static actor registration to succeed")
	}
	if _, ok := directory.ByVID(7); !ok {
		t.Fatal("expected static actor VID lookup to exist before update")
	}

	updated := actor
	updated.RaceNum = uint32(^uint16(0)) + 1
	if !directory.Update(updated) {
		t.Fatal("expected static actor update to succeed even when actor stops being visibility-encodable")
	}
	if _, ok := directory.ByVID(7); ok {
		t.Fatal("expected static actor VID lookup to be cleared after actor stops being visibility-encodable")
	}
}
