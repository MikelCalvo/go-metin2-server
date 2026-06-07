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
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 50, SellCountPerGold: true, AntiSell: true},
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
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 50, SellCountPerGold: true},
		{Vnum: 50053, Name: "Polished Helmet", Stackable: false, MaxCount: 1, EquipSlot: "head"},
	}}

	if err := store.Save(first); err != nil {
		t.Fatalf("save first snapshot: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot: %v", err)
	}
	wantFirst := "{\n  \"templates\": [\n    {\n      \"vnum\": 11200,\n      \"name\": \"Wooden Sword\",\n      \"stackable\": false,\n      \"max_count\": 1,\n      \"equip_slot\": \"weapon\"\n    },\n    {\n      \"vnum\": 27001,\n      \"name\": \"Small Red Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"shop_buy_price\": 50,\n      \"sell_count_per_gold\": true\n    },\n    {\n      \"vnum\": 50053,\n      \"name\": \"Polished Helmet\",\n      \"stackable\": false,\n      \"max_count\": 1,\n      \"equip_slot\": \"head\"\n    }\n  ]\n}\n"
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

	unknownField := []byte("{\"templates\":[{\"vnum\":27001,\"name\":\"Small Red Potion\",\"stackable\":true,\"max_count\":200,\"unowned_effect\":true}]}")
	if err := os.WriteFile(path, unknownField, 0o644); err != nil {
		t.Fatalf("write unknown-field snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for unknown item-template field, got %v", err)
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
	overClientCountRange := Snapshot{Templates: []Template{{Vnum: 27001, Name: "Huge Red Potion Stack", Stackable: true, MaxCount: 256}}}
	if err := store.Save(overClientCountRange); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for max_count beyond bootstrap client count range, got %v", err)
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

func TestFileStoreSaveThenLoadRoundTripPreservesUseEffectMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:      27002,
		Name:      "Practice Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &UseEffect{
			PointType:  7,
			PointIndex: 1,
			PointDelta: 25,
			Message:    "consume:27002:+25",
		},
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with use effect metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with use effect metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with use effect metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with use effect metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27002,\n      \"name\": \"Practice Elixir\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"use_effect\": {\n        \"point_type\": 7,\n        \"point_index\": 1,\n        \"point_delta\": 25,\n        \"message\": \"consume:27002:+25\"\n      }\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with use effect metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveRejectsInvalidUseEffectMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)

	missingMessage := Snapshot{Templates: []Template{{
		Vnum:      27002,
		Name:      "Practice Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &UseEffect{PointType: 7, PointIndex: 1, PointDelta: 25},
	}}}
	if err := store.Save(missingMessage); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for missing use-effect message, got %v", err)
	}

	zeroType := Snapshot{Templates: []Template{{
		Vnum:      27002,
		Name:      "Practice Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &UseEffect{PointType: 0, PointIndex: 1, PointDelta: 25, Message: "consume:27002:+25"},
	}}}
	if err := store.Save(zeroType); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for zero use-effect point type, got %v", err)
	}

	zeroDelta := Snapshot{Templates: []Template{{
		Vnum:      27002,
		Name:      "Practice Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &UseEffect{PointType: 7, PointIndex: 1, PointDelta: 0, Message: "consume:27002:+25"},
	}}}
	if err := store.Save(zeroDelta); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for zero use-effect point delta, got %v", err)
	}

	invalidPointIndex := Snapshot{Templates: []Template{{
		Vnum:      27002,
		Name:      "Practice Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &UseEffect{PointType: 7, PointIndex: 255, PointDelta: 25, Message: "consume:27002:+25"},
	}}}
	if err := store.Save(invalidPointIndex); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for out-of-range use-effect point index, got %v", err)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesAntiFlagMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:         27003,
		Name:         "Bound Practice Potion",
		Stackable:    true,
		MaxCount:     200,
		AntiSell:     true,
		AntiDrop:     true,
		AntiGive:     true,
		AntiStack:    true,
		AntiMale:     true,
		AntiFemale:   true,
		AntiWarrior:  true,
		AntiAssassin: true,
		AntiSura:     true,
		AntiShaman:   true,
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with anti-flag metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with anti-flag metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with anti-flag metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with anti-flag metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27003,\n      \"name\": \"Bound Practice Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"anti_sell\": true,\n      \"anti_drop\": true,\n      \"anti_give\": true,\n      \"anti_stack\": true,\n      \"anti_male\": true,\n      \"anti_female\": true,\n      \"anti_warrior\": true,\n      \"anti_assassin\": true,\n      \"anti_sura\": true,\n      \"anti_shaman\": true\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with anti-flag metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesMinLevelRestriction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:      27004,
		Name:      "Veteran Practice Potion",
		Stackable: true,
		MaxCount:  200,
		MinLevel:  10,
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with min-level metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with min-level metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with min-level metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with min-level metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 27004,\n      \"name\": \"Veteran Practice Potion\",\n      \"stackable\": true,\n      \"max_count\": 200,\n      \"min_level\": 10\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with min-level metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveThenLoadRoundTripPreservesEquipEffectMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:      12200,
		Name:      "Practice Blade",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: "weapon",
		EquipEffect: &PointEffect{
			PointType:  1,
			PointIndex: 1,
			PointDelta: 10,
		},
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot with equip effect metadata: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot with equip effect metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot with equip effect metadata:\n got: %#v\nwant: %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot with equip effect metadata: %v", err)
	}
	wantJSON := "{\n  \"templates\": [\n    {\n      \"vnum\": 12200,\n      \"name\": \"Practice Blade\",\n      \"stackable\": false,\n      \"max_count\": 1,\n      \"equip_slot\": \"weapon\",\n      \"equip_effect\": {\n        \"point_type\": 1,\n        \"point_index\": 1,\n        \"point_delta\": 10\n      }\n    }\n  ]\n}\n"
	if string(raw) != wantJSON {
		t.Fatalf("unexpected deterministic snapshot with equip effect metadata:\n got: %s\nwant: %s", string(raw), wantJSON)
	}
}

func TestFileStoreSaveRejectsInvalidEquipEffectMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)

	missingEquipSlot := Snapshot{Templates: []Template{{
		Vnum:        12200,
		Name:        "Practice Blade",
		Stackable:   false,
		MaxCount:    1,
		EquipEffect: &PointEffect{PointType: 1, PointIndex: 1, PointDelta: 10},
	}}}
	if err := store.Save(missingEquipSlot); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for equip-effect without equip slot, got %v", err)
	}

	zeroType := Snapshot{Templates: []Template{{
		Vnum:        12200,
		Name:        "Practice Blade",
		Stackable:   false,
		MaxCount:    1,
		EquipSlot:   "weapon",
		EquipEffect: &PointEffect{PointType: 0, PointIndex: 1, PointDelta: 10},
	}}}
	if err := store.Save(zeroType); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for zero equip-effect point type, got %v", err)
	}

	zeroDelta := Snapshot{Templates: []Template{{
		Vnum:        12200,
		Name:        "Practice Blade",
		Stackable:   false,
		MaxCount:    1,
		EquipSlot:   "weapon",
		EquipEffect: &PointEffect{PointType: 1, PointIndex: 1, PointDelta: 0},
	}}}
	if err := store.Save(zeroDelta); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for zero equip-effect point delta, got %v", err)
	}

	invalidPointIndex := Snapshot{Templates: []Template{{
		Vnum:        12200,
		Name:        "Practice Blade",
		Stackable:   false,
		MaxCount:    1,
		EquipSlot:   "weapon",
		EquipEffect: &PointEffect{PointType: 1, PointIndex: 255, PointDelta: 10},
	}}}
	if err := store.Save(invalidPointIndex); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for out-of-range equip-effect point index, got %v", err)
	}
}
