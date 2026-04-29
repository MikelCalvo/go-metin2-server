package itemstore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFileStoreSaveThenLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, EquipSlot: "weapon"},
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
	}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestFileStoreSaveWritesDeterministicSortedSnapshotAndReplacesPreviousContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	first := Snapshot{Templates: []Template{
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, EquipSlot: "weapon"},
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
		{Vnum: 50053, Name: "Polished Helmet", Stackable: false, MaxCount: 1, EquipSlot: "head"},
	}}

	if err := store.Save(first); err != nil {
		t.Fatalf("save first snapshot: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot: %v", err)
	}
	wantFirst := "{\n  \"templates\": [\n    {\n      \"vnum\": 11200,\n      \"name\": \"Wooden Sword\",\n      \"stackable\": false,\n      \"max_count\": 1,\n      \"equip_slot\": \"weapon\"\n    },\n    {\n      \"vnum\": 27001,\n      \"name\": \"Small Red Potion\",\n      \"stackable\": true,\n      \"max_count\": 200\n    },\n    {\n      \"vnum\": 50053,\n      \"name\": \"Polished Helmet\",\n      \"stackable\": false,\n      \"max_count\": 1,\n      \"equip_slot\": \"head\"\n    }\n  ]\n}\n"
	if string(raw) != wantFirst {
		t.Fatalf("unexpected deterministic first snapshot:\n got: %s\nwant: %s", string(raw), wantFirst)
	}

	second := Snapshot{Templates: []Template{{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200}}}
	if err := store.Save(second); err != nil {
		t.Fatalf("save replacement snapshot: %v", err)
	}
	raw, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replacement snapshot: %v", err)
	}
	wantSecond := "{\n  \"templates\": [\n    {\n      \"vnum\": 27002,\n      \"name\": \"Small Blue Potion\",\n      \"stackable\": true,\n      \"max_count\": 200\n    }\n  ]\n}\n"
	if string(raw) != wantSecond {
		t.Fatalf("unexpected replacement snapshot:\n got: %s\nwant: %s", string(raw), wantSecond)
	}
}

func TestFileStoreLoadReturnsNotFoundForMissingSnapshot(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "state", "item-templates.json"))
	_, err := store.Load()
	if !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected ErrSnapshotNotFound, got %v", err)
	}
}

func TestFileStoreLoadRejectsMalformedOrInvalidSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write malformed snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for malformed json, got %v", err)
	}

	zeroVnum := Snapshot{Templates: []Template{{Vnum: 0, Name: "Broken", Stackable: true, MaxCount: 1}}}
	if err := store.Save(zeroVnum); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for zero vnum, got %v", err)
	}
	blankName := Snapshot{Templates: []Template{{Vnum: 27001, Name: "   ", Stackable: true, MaxCount: 1}}}
	if err := store.Save(blankName); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for blank name, got %v", err)
	}
	zeroMaxCount := Snapshot{Templates: []Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 0}}}
	if err := store.Save(zeroMaxCount); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for zero max count, got %v", err)
	}
	nonStackableMultiCount := Snapshot{Templates: []Template{{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 2, EquipSlot: "weapon"}}}
	if err := store.Save(nonStackableMultiCount); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for non-stackable max_count != 1, got %v", err)
	}
	invalidEquipSlot := Snapshot{Templates: []Template{{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, EquipSlot: "cape"}}}
	if err := store.Save(invalidEquipSlot); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for invalid equip slot, got %v", err)
	}
	duplicate := Snapshot{Templates: []Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
		{Vnum: 27001, Name: "Duplicate Potion", Stackable: true, MaxCount: 200},
	}}
	if err := store.Save(duplicate); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for duplicate vnum, got %v", err)
	}
}
