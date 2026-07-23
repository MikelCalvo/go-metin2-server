package staticstore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestFileStoreSaveThenLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	want := Snapshot{StaticActors: []StaticActor{
		{EntityID: 2, Name: "Alchemist", MapIndex: 21, X: 52070, Y: 166600, RaceNum: 20001, InteractionKind: "info", InteractionRef: "lore:alchemist"},
		{EntityID: 7, Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy},
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
		{EntityID: 7, Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy},
		{EntityID: 5, Name: "VillageGuard", MapIndex: 1, X: 469400, Y: 964300, RaceNum: 20354},
	}}

	if err := store.Save(first); err != nil {
		t.Fatalf("save first snapshot: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot: %v", err)
	}
	wantFirst := "{\n  \"static_actors\": [\n    {\n      \"entity_id\": 2,\n      \"name\": \"Alchemist\",\n      \"map_index\": 21,\n      \"x\": 52070,\n      \"y\": 166600,\n      \"race_num\": 20001\n    },\n    {\n      \"entity_id\": 7,\n      \"name\": \"TrainingDummy\",\n      \"map_index\": 42,\n      \"x\": 1800,\n      \"y\": 2900,\n      \"race_num\": 20350,\n      \"combat_profile\": \"training_dummy\"\n    },\n    {\n      \"entity_id\": 5,\n      \"name\": \"VillageGuard\",\n      \"map_index\": 1,\n      \"x\": 469400,\n      \"y\": 964300,\n      \"race_num\": 20354\n    },\n    {\n      \"entity_id\": 9,\n      \"name\": \"VillageGuard\",\n      \"map_index\": 1,\n      \"x\": 469300,\n      \"y\": 964200,\n      \"race_num\": 20355\n    }\n  ]\n}\n"
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

func TestFileStoreSaveNormalizesRewardDropOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	input := Snapshot{StaticActors: []StaticActor{{EntityID: 22, Name: "RewardMultiDrop", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, SpawnGroupRef: "practice.reward_multi_drop", RewardDropVnums: []uint32{27003, 27001, 27002}}}}

	if err := store.Save(input); err != nil {
		t.Fatalf("save reward-drop snapshot: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load reward-drop snapshot: %v", err)
	}
	want := Snapshot{StaticActors: []StaticActor{{EntityID: 22, Name: "RewardMultiDrop", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, SpawnGroupRef: "practice.reward_multi_drop", RewardDropVnums: []uint32{27001, 27002, 27003}}}}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("expected reward drop vnums to be persisted in canonical order:\n got: %#v\nwant: %#v", loaded, want)
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
	unknownField := []byte(`{"static_actors":[{"entity_id":7,"name":"VillageGuard","map_index":1,"x":469300,"y":964200,"race_num":20355,"unknown":true}]}`)
	if err := os.WriteFile(path, unknownField, 0o644); err != nil {
		t.Fatalf("write unknown-field snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for unknown fields, got %v", err)
	}
	trailingJSON := []byte(`{"static_actors":[{"entity_id":7,"name":"VillageGuard","map_index":1,"x":469300,"y":964200,"race_num":20355}]} {}`)
	if err := os.WriteFile(path, trailingJSON, 0o644); err != nil {
		t.Fatalf("write trailing-json snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for trailing JSON, got %v", err)
	}

	invalid := Snapshot{StaticActors: []StaticActor{{EntityID: 7, Name: "", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355}}}
	if err := store.Save(invalid); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for invalid actor, got %v", err)
	}
	whitespaceName := Snapshot{StaticActors: []StaticActor{{EntityID: 24, Name: "   ", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355}}}
	if err := store.Save(whitespaceName); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for whitespace-only actor name, got %v", err)
	}
	trimmedStaticActor := Snapshot{StaticActors: []StaticActor{{EntityID: 25, Name: "  TrimmedGuard  ", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355, CombatProfile: " training_dummy ", InteractionKind: " info ", InteractionRef: " npc:trimmed_guard "}}}
	if err := store.Save(trimmedStaticActor); err != nil {
		t.Fatalf("expected trimmable actor metadata to save, got %v", err)
	}
	loadedTrimmedStaticActor, err := store.Load()
	if err != nil {
		t.Fatalf("load trimmed static actor snapshot: %v", err)
	}
	wantTrimmedStaticActor := Snapshot{StaticActors: []StaticActor{{EntityID: 25, Name: "TrimmedGuard", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, InteractionKind: "info", InteractionRef: "npc:trimmed_guard"}}}
	if !reflect.DeepEqual(loadedTrimmedStaticActor, wantTrimmedStaticActor) {
		t.Fatalf("expected static actor metadata to be trimmed on persistence:\n got: %#v\nwant: %#v", loadedTrimmedStaticActor, wantTrimmedStaticActor)
	}
	invalidRaceNum := Snapshot{StaticActors: []StaticActor{{EntityID: 23, Name: "WideRaceGuard", MapIndex: 1, X: 469300, Y: 964200, RaceNum: uint32(^uint16(0)) + 1}}}
	if err := store.Save(invalidRaceNum); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for unencodable actor race number, got %v", err)
	}
	invalidInteraction := Snapshot{StaticActors: []StaticActor{{EntityID: 8, Name: "VillageGuard", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355, InteractionKind: "talk"}}}
	if err := store.Save(invalidInteraction); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for partial interaction metadata, got %v", err)
	}
	interactionRefWithoutNamespace := Snapshot{StaticActors: []StaticActor{{EntityID: 26, Name: "VillageGuard", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355, InteractionKind: "talk", InteractionRef: "village_guard"}}}
	if err := store.Save(interactionRefWithoutNamespace); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for interaction ref without namespace, got %v", err)
	}
	pathAmbiguousInteractionRef := Snapshot{StaticActors: []StaticActor{{EntityID: 27, Name: "VillageGuard", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355, InteractionKind: "talk", InteractionRef: "npc/village_guard"}}}
	if err := store.Save(pathAmbiguousInteractionRef); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for path-ambiguous interaction ref, got %v", err)
	}
	unsupportedInteractionKind := Snapshot{StaticActors: []StaticActor{{EntityID: 10, Name: "QuestMarker", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355, InteractionKind: "quest", InteractionRef: "quest:first_steps"}}}
	if err := store.Save(unsupportedInteractionKind); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for unsupported interaction kind, got %v", err)
	}
	invalidCombatProfile := Snapshot{StaticActors: []StaticActor{{EntityID: 12, Name: "BrokenDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: "boss"}}}
	if err := store.Save(invalidCombatProfile); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for invalid combat profile, got %v", err)
	}
	invalidSpawnGroupWithoutCombatProfile := Snapshot{StaticActors: []StaticActor{{EntityID: 13, Name: "PracticeMobAlpha", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, SpawnGroupRef: "practice.mob_alpha"}}}
	if err := store.Save(invalidSpawnGroupWithoutCombatProfile); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for spawn group without combat profile, got %v", err)
	}
	invalidSpawnGroupWithInteraction := Snapshot{StaticActors: []StaticActor{{EntityID: 14, Name: "PracticeMobAlpha", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, SpawnGroupRef: "practice.mob_alpha", InteractionKind: "talk", InteractionRef: "npc:village_guard"}}}
	if err := store.Save(invalidSpawnGroupWithInteraction); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for spawn group carrying interaction metadata, got %v", err)
	}
	invalidStaticActorWithReward := Snapshot{StaticActors: []StaticActor{{EntityID: 17, Name: "RewardedStandaloneDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, RewardExperience: 10}}}
	if err := store.Save(invalidStaticActorWithReward); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for non-spawn static actor carrying reward metadata, got %v", err)
	}
	validMultiDropReward := Snapshot{StaticActors: []StaticActor{{EntityID: 22, Name: "RewardMultiDrop", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, SpawnGroupRef: "practice.reward_multi_drop", RewardDropVnums: []uint32{27001, 27002, 27003}}}}
	if err := store.Save(validMultiDropReward); err != nil {
		t.Fatalf("expected valid spawn-group reward descriptor with multiple distinct drop vnums to save, got %v", err)
	}
	loadedMultiDropReward, err := store.Load()
	if err != nil {
		t.Fatalf("load multi-drop reward snapshot: %v", err)
	}
	if !reflect.DeepEqual(loadedMultiDropReward, validMultiDropReward) {
		t.Fatalf("expected multi-drop reward snapshot to round-trip, got %#v want %#v", loadedMultiDropReward, validMultiDropReward)
	}
	invalidSpawnGroupRewardCases := map[string]StaticActor{
		"experience overflow": {EntityID: 18, Name: "RewardOverflowExp", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, SpawnGroupRef: "practice.reward_overflow_exp", RewardExperience: uint64(^uint32(0)>>1) + 1},
		"gold overflow":       {EntityID: 19, Name: "RewardOverflowGold", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, SpawnGroupRef: "practice.reward_overflow_gold", RewardGold: uint64(^uint32(0)>>1) + 1},
		"zero drop vnum":      {EntityID: 20, Name: "RewardZeroDrop", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, SpawnGroupRef: "practice.reward_zero_drop", RewardDropVnums: []uint32{27001, 0}},
		"duplicate drop vnum": {EntityID: 21, Name: "RewardDuplicateDrop", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, SpawnGroupRef: "practice.reward_duplicate_drop", RewardDropVnums: []uint32{27001, 27001}},
	}
	for name, actor := range invalidSpawnGroupRewardCases {
		if err := store.Save(Snapshot{StaticActors: []StaticActor{actor}}); !errors.Is(err, ErrInvalidSnapshot) {
			t.Fatalf("expected ErrInvalidSnapshot for spawn-group reward descriptor with %s, got %v", name, err)
		}
	}
	duplicateSpawnGroupRef := Snapshot{StaticActors: []StaticActor{
		{EntityID: 15, Name: "PracticeMobAlpha", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, SpawnGroupRef: "practice.mob_alpha"},
		{EntityID: 16, Name: "PracticeMobBeta", MapIndex: 42, X: 1850, Y: 2950, RaceNum: 102, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, SpawnGroupRef: "practice.mob_alpha"},
	}}
	if err := store.Save(duplicateSpawnGroupRef); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for duplicate spawn-group refs, got %v", err)
	}
}
