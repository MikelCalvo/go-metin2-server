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
