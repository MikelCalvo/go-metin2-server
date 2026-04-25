package staticstore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFileStoreSaveThenLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	want := Snapshot{StaticActors: []StaticActor{
		{EntityID: 2, Name: "Alchemist", MapIndex: 21, X: 52070, Y: 166600, RaceNum: 20001, InteractionKind: "info", InteractionRef: "lore:alchemist"},
		{EntityID: 9, Name: "VillageGuard", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355, InteractionKind: "talk", InteractionRef: "npc:village_guard"},
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
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	first := Snapshot{StaticActors: []StaticActor{
		{EntityID: 9, Name: "VillageGuard", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355},
		{EntityID: 2, Name: "Alchemist", MapIndex: 21, X: 52070, Y: 166600, RaceNum: 20001},
		{EntityID: 5, Name: "VillageGuard", MapIndex: 1, X: 469400, Y: 964300, RaceNum: 20354},
	}}

	if err := store.Save(first); err != nil {
		t.Fatalf("save first snapshot: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot: %v", err)
	}
	wantFirst := "{\n  \"static_actors\": [\n    {\n      \"entity_id\": 2,\n      \"name\": \"Alchemist\",\n      \"map_index\": 21,\n      \"x\": 52070,\n      \"y\": 166600,\n      \"race_num\": 20001\n    },\n    {\n      \"entity_id\": 5,\n      \"name\": \"VillageGuard\",\n      \"map_index\": 1,\n      \"x\": 469400,\n      \"y\": 964300,\n      \"race_num\": 20354\n    },\n    {\n      \"entity_id\": 9,\n      \"name\": \"VillageGuard\",\n      \"map_index\": 1,\n      \"x\": 469300,\n      \"y\": 964200,\n      \"race_num\": 20355\n    }\n  ]\n}\n"
	if string(raw) != wantFirst {
		t.Fatalf("unexpected deterministic first snapshot:\n got: %s\nwant: %s", string(raw), wantFirst)
	}

	second := Snapshot{StaticActors: []StaticActor{{EntityID: 42, Name: "Blacksmith", MapIndex: 41, X: 957300, Y: 255200, RaceNum: 20016}}}
	if err := store.Save(second); err != nil {
		t.Fatalf("save replacement snapshot: %v", err)
	}
	raw, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replacement snapshot: %v", err)
	}
	wantSecond := "{\n  \"static_actors\": [\n    {\n      \"entity_id\": 42,\n      \"name\": \"Blacksmith\",\n      \"map_index\": 41,\n      \"x\": 957300,\n      \"y\": 255200,\n      \"race_num\": 20016\n    }\n  ]\n}\n"
	if string(raw) != wantSecond {
		t.Fatalf("unexpected replacement snapshot:\n got: %s\nwant: %s", string(raw), wantSecond)
	}
}

func TestFileStoreLoadReturnsNotFoundForMissingSnapshot(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "state", "static-actors.json"))
	_, err := store.Load()
	if !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected ErrSnapshotNotFound, got %v", err)
	}
}

func TestFileStoreLoadRejectsMalformedOrInvalidSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
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

	invalid := Snapshot{StaticActors: []StaticActor{{EntityID: 7, Name: "", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355}}}
	if err := store.Save(invalid); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for invalid actor, got %v", err)
	}
	invalidInteraction := Snapshot{StaticActors: []StaticActor{{EntityID: 8, Name: "VillageGuard", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355, InteractionKind: "talk"}}}
	if err := store.Save(invalidInteraction); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for partial interaction metadata, got %v", err)
	}
}
