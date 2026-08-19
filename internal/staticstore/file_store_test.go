package staticstore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
	"unicode/utf8"

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

func TestFileStoreSaveLoadRoundTripIncludesCustomCombatProfiles(t *testing.T) {
	const profile = "practice_static_store_formula_wolf"
	worldruntime.UnregisterStaticActorCombatProfileForTest(profile)
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	input := Snapshot{
		StaticActors: []StaticActor{{EntityID: 23, Name: "FormulaWolf", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: profile, SpawnGroupRef: "practice.formula_wolf"}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:        profile,
			MaxHP:          24,
			AttackValue:    9,
			DefenseValue:   4,
			RespawnDelayMs: 1500,
		}},
	}

	if err := store.Save(input); err != nil {
		t.Fatalf("save custom combat-profile static actor snapshot: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load custom combat-profile static actor snapshot: %v", err)
	}
	want := Snapshot{
		StaticActors: []StaticActor{{EntityID: 23, Name: "FormulaWolf", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: profile, SpawnGroupRef: "practice.formula_wolf"}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:               profile,
			MaxHP:                 24,
			DamagePerNormalAttack: 5,
			AttackValue:           9,
			DefenseValue:          4,
			Level:                 worldruntime.TrainingDummyBootstrapLevel,
			RespawnDelayMs:        1500,
		}},
	}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("unexpected custom combat-profile static actor snapshot:\n got: %#v\nwant: %#v", loaded, want)
	}
}

func TestFileStoreRejectsNonCanonicalCustomCombatProfileSnapshotIdentity(t *testing.T) {
	const profile = "practice_static_store_padded_wolf"
	saveStore := NewFileStore(filepath.Join(t.TempDir(), "state", "static-actors.json"))
	paddedSnapshot := Snapshot{
		StaticActors: []StaticActor{{EntityID: 31, Name: "PaddedProfileWolf", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: profile, SpawnGroupRef: "practice.padded_profile_wolf"}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:        " " + profile + " ",
			MaxHP:          24,
			AttackValue:    9,
			DefenseValue:   4,
			RespawnDelayMs: 1500,
		}},
	}
	if err := saveStore.Save(paddedSnapshot); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot when saving padded custom combat profile identity, got %v", err)
	}

	loadPath := filepath.Join(t.TempDir(), "state", "static-actors.json")
	if err := os.MkdirAll(filepath.Dir(loadPath), 0o755); err != nil {
		t.Fatalf("create hand-edited snapshot dir: %v", err)
	}
	raw := `{
  "static_actors": [
    {
      "entity_id": 31,
      "name": "PaddedProfileWolf",
      "map_index": 42,
      "x": 1800,
      "y": 2900,
      "race_num": 101,
      "combat_profile": "practice_static_store_padded_wolf",
      "spawn_group_ref": "practice.padded_profile_wolf"
    }
  ],
  "combat_profiles": [
    {
      "profile": " practice_static_store_padded_wolf ",
      "max_hp": 24,
      "attack_value": 9,
      "defense_value": 4,
      "respawn_delay_ms": 1500
    }
  ]
}
`
	if err := os.WriteFile(loadPath, []byte(raw), 0o644); err != nil {
		t.Fatalf("write hand-edited padded profile snapshot: %v", err)
	}
	if _, err := NewFileStore(loadPath).Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot when loading padded custom combat profile identity, got %v", err)
	}
}

func TestFileStoreRoundTripsSpawnGroupAuthoredHome(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	input := Snapshot{StaticActors: []StaticActor{{
		EntityID:      22,
		Name:          "LeashPersistMob",
		MapIndex:      42,
		X:             2301,
		Y:             2800,
		RaceNum:       101,
		CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.leash_persist",
		SpawnHome:     &worldruntime.PositionSnapshot{MapIndex: 42, X: 1700, Y: 2800},
	}}}

	if err := store.Save(input); err != nil {
		t.Fatalf("save spawn-home snapshot: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load spawn-home snapshot: %v", err)
	}
	if !reflect.DeepEqual(loaded, input) {
		t.Fatalf("expected authored spawn home to round-trip:\n got: %#v\nwant: %#v", loaded, input)
	}
}

func TestFileStoreRoundTripsStillDeadSpawnGroupCombatState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	currentHP := uint8(0)
	readyAt := time.Unix(1700000400, 0).UTC()
	input := Snapshot{StaticActors: []StaticActor{{
		EntityID:        33,
		Name:            "StillDeadPersistMob",
		MapIndex:        42,
		X:               1200,
		Y:               2200,
		RaceNum:         101,
		CombatProfile:   worldruntime.StaticActorCombatProfileTrainingDummy,
		SpawnGroupRef:   "practice.still_dead_persist",
		SpawnHome:       &worldruntime.PositionSnapshot{MapIndex: 42, X: 1200, Y: 2200},
		CombatCurrentHP: &currentHP,
		RespawnReadyAt:  &readyAt,
	}}}

	if err := store.Save(input); err != nil {
		t.Fatalf("save still-dead spawn-group snapshot: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load still-dead spawn-group snapshot: %v", err)
	}
	if !reflect.DeepEqual(loaded, input) {
		t.Fatalf("expected still-dead combat state to round-trip:\n got: %#v\nwant: %#v", loaded, input)
	}
}

func TestFileStoreRejectsMalformedStillDeadSpawnGroupCombatState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	currentHP := uint8(5)
	readyAt := time.Unix(1700000400, 0).UTC()
	err := store.Save(Snapshot{StaticActors: []StaticActor{{
		EntityID:        34,
		Name:            "MalformedStillDeadMob",
		MapIndex:        42,
		X:               1200,
		Y:               2200,
		RaceNum:         101,
		CombatProfile:   worldruntime.StaticActorCombatProfileTrainingDummy,
		SpawnGroupRef:   "practice.malformed_still_dead",
		CombatCurrentHP: &currentHP,
		RespawnReadyAt:  &readyAt,
	}}})
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for damaged HP still-dead state, got %v", err)
	}

	zeroHP := uint8(0)
	err = store.Save(Snapshot{StaticActors: []StaticActor{{
		EntityID:        7,
		Name:            "TrainingDummy",
		MapIndex:        42,
		X:               1800,
		Y:               2900,
		RaceNum:         20350,
		CombatProfile:   worldruntime.StaticActorCombatProfileTrainingDummy,
		CombatCurrentHP: &zeroHP,
		RespawnReadyAt:  &readyAt,
	}}})
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for non-spawn still-dead state, got %v", err)
	}
}

func TestFileStoreLoadReturnsNotFoundForMissingSnapshot(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "state", "static-actors.json"))
	_, err := store.Load()
	if !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected ErrSnapshotNotFound, got %v", err)
	}
}

func TestFileStoreLoadRejectsSymlinkedCommittedStaticActorSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	target := filepath.Join(t.TempDir(), "outside-static-actors.json")
	if err := os.WriteFile(target, []byte(`{"static_actors":[{"entity_id":7,"name":"TrainingDummy","map_index":42,"x":1800,"y":2900,"race_num":20350,"combat_profile":"training_dummy"}]}`), 0o644); err != nil {
		t.Fatalf("write outside static actor snapshot: %v", err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	_, err := store.Load()
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for symlinked static actor snapshot, got %v", err)
	}
}

func TestFileStoreValidateRejectsSymlinkedStaticActorCrashTempFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{StaticActors: []StaticActor{{EntityID: 7, Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy}}}); err != nil {
		t.Fatalf("save static actor snapshot: %v", err)
	}
	target := filepath.Join(t.TempDir(), "outside-static-actor-temp.json")
	if err := os.WriteFile(target, []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write outside static actor temp target: %v", err)
	}
	link := filepath.Join(filepath.Dir(path), ".static-actors-crashed.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	_, err := store.Validate()
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for symlinked static actor crash temp file, got %v", err)
	}
}

func TestFileStoreValidateRejectsSymlinkedCommittedStaticActorSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	target := filepath.Join(t.TempDir(), "outside-static-actors.json")
	if err := os.WriteFile(target, []byte(`{"static_actors":[{"entity_id":7,"name":"TrainingDummy","map_index":42,"x":1800,"y":2900,"race_num":20350,"combat_profile":"training_dummy"}]}`), 0o644); err != nil {
		t.Fatalf("write outside static actor snapshot: %v", err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	_, err := store.Validate()
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for symlinked static actor snapshot validation, got %v", err)
	}
}

func TestFileStoreRejectsNULStaticActorNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	nulName := Snapshot{StaticActors: []StaticActor{{EntityID: 7, Name: "Visible\x00Hidden", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355}}}
	if err := store.Save(nulName); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for NUL static actor name on save, got %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"static_actors":[{"entity_id":7,"name":"Visible\u0000Hidden","map_index":1,"x":469300,"y":964200,"race_num":20355}]}`), 0o644); err != nil {
		t.Fatalf("write NUL static actor snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for NUL static actor name on load, got %v", err)
	}
}

func TestFileStoreRejectsInvalidUTF8StaticActorNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	invalid := string([]byte{'V', 'i', 's', 'i', 'b', 'l', 'e', 0xff, 'H', 'i', 'd', 'd', 'e', 'n'})
	if utf8.ValidString(invalid) {
		t.Fatal("test fixture must contain invalid UTF-8")
	}
	if err := store.Save(Snapshot{StaticActors: []StaticActor{{EntityID: 7, Name: invalid, MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355}}}); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for invalid UTF-8 static actor name on save, got %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	body := []byte(`{"static_actors":[{"entity_id":7,"name":"Visible`)
	body = append(body, 0xff)
	body = append(body, []byte(`Hidden","map_index":1,"x":469300,"y":964200,"race_num":20355}]}`)...)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write invalid UTF-8 static actor snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for invalid UTF-8 static actor name on load, got %v", err)
	}
}

func TestFileStoreValidateReportsDeterministicStaticActorSummary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{StaticActors: []StaticActor{
		{EntityID: 9, Name: "VillageGuard", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355, InteractionKind: "talk", InteractionRef: "npc:village_guard"},
		{EntityID: 7, Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy},
		{EntityID: 22, Name: "RewardMob", MapIndex: 42, X: 1850, Y: 2950, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, SpawnGroupRef: "practice.reward_mob", RewardDropVnums: []uint32{27001}},
	}}); err != nil {
		t.Fatalf("save static actor snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), ".static-actors-crashed.json"), []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write static actor crash temp: %v", err)
	}

	summary, err := store.Validate()
	if err != nil {
		t.Fatalf("validate static actor store: %v", err)
	}
	want := SnapshotSummary{
		ActorCount:             3,
		InteractableActorCount: 1,
		SpawnGroupCount:        1,
		ActorIDs:               []uint64{22, 7, 9},
		ActorNames:             []string{"RewardMob", "TrainingDummy", "VillageGuard"},
		CrashTempCount:         1,
		CrashTempFiles:         []string{".static-actors-crashed.json"},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected static actor validation summary: got %#v want %#v", summary, want)
	}
}

func TestFileStoreValidateTreatsMissingStaticActorSnapshotAsEmpty(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "missing", "static-actors.json"))

	summary, err := store.Validate()
	if err != nil {
		t.Fatalf("validate missing static actor store: %v", err)
	}
	want := SnapshotSummary{ActorIDs: []uint64{}, ActorNames: []string{}}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected empty static actor summary: got %#v want %#v", summary, want)
	}
}

func TestFileStoreCleanupCrashTempFilesRemovesOnlyStaticActorTempResidue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{StaticActors: []StaticActor{
		{EntityID: 9, Name: "VillageGuard", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355, InteractionKind: "talk", InteractionRef: "npc:village_guard"},
		{EntityID: 7, Name: "TrainingDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy},
	}}); err != nil {
		t.Fatalf("save static actor snapshot: %v", err)
	}
	for _, name := range []string{".static-actors-zeta.json", ".static-actors-alpha.json", ".unrelated-temp.json"} {
		if err := os.WriteFile(filepath.Join(filepath.Dir(path), name), []byte(`{"not":"committed"}`), 0o644); err != nil {
			t.Fatalf("write temp file %s: %v", name, err)
		}
	}

	summary, err := store.CleanupCrashTempFiles()
	if err != nil {
		t.Fatalf("cleanup static actor crash temp files: %v", err)
	}
	want := SnapshotSummary{ActorCount: 2, InteractableActorCount: 1, ActorIDs: []uint64{7, 9}, ActorNames: []string{"TrainingDummy", "VillageGuard"}}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected post-cleanup static actor summary: got %#v want %#v", summary, want)
	}
	for _, name := range []string{".static-actors-zeta.json", ".static-actors-alpha.json"} {
		if _, err := os.Stat(filepath.Join(filepath.Dir(path), name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected static actor crash temp %s to be removed, stat err=%v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), ".unrelated-temp.json")); err != nil {
		t.Fatalf("expected unrelated hidden file to be preserved: %v", err)
	}
}

func TestFileStoreCleanupCrashTempFilesFailsClosedOnCorruptStaticActorSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	crashTemp := filepath.Join(filepath.Dir(path), ".static-actors-crashed.json")
	if err := os.WriteFile(path, []byte(`{"static_actors":[`), 0o644); err != nil {
		t.Fatalf("write corrupt static actor snapshot: %v", err)
	}
	if err := os.WriteFile(crashTemp, []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write crash temp: %v", err)
	}

	_, err := store.CleanupCrashTempFiles()
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot and no cleanup for corrupt committed snapshot, got %v", err)
	}
	if _, err := os.Stat(crashTemp); err != nil {
		t.Fatalf("expected crash temp to remain after failed cleanup: %v", err)
	}
}

func TestFileStoreValidateFailsClosedOnCorruptStaticActorSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"static_actors":[`), 0o644); err != nil {
		t.Fatalf("write corrupt static actor snapshot: %v", err)
	}

	_, err := store.Validate()
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for corrupt static actor snapshot, got %v", err)
	}
}

func TestFileStoreLoadRejectsMalformedOrInvalidSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "static-actors.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("null"), 0o644); err != nil {
		t.Fatalf("write null snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for null snapshot root, got %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"static_actors":null}`), 0o644); err != nil {
		t.Fatalf("write null static actor collection: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for null static actor collection, got %v", err)
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
	invalidEntityID := Snapshot{StaticActors: []StaticActor{{EntityID: uint64(^uint32(0)) + 1, Name: "WideIDGuard", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355}}}
	if err := store.Save(invalidEntityID); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for unencodable actor entity ID, got %v", err)
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
	invalidStaticActorWithKillQuest := Snapshot{StaticActors: []StaticActor{{EntityID: 27, Name: "QuestStandaloneDummy", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, RewardQuestRef: "quest:first_steps", RewardQuestFlag: "killed_qa_mob", RewardQuestTo: 1, RewardQuestText: "Quest updated."}}}
	if err := store.Save(invalidStaticActorWithKillQuest); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for non-spawn static actor carrying kill quest credit, got %v", err)
	}
	validKillQuestCredit := Snapshot{StaticActors: []StaticActor{{EntityID: 28, Name: "KillQuestMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, SpawnGroupRef: "practice.kill_quest_mob", RewardQuestRef: "quest:first_steps", RewardQuestFlag: "killed_qa_mob", RewardQuestTo: 1, RewardQuestText: "Quest updated: first_steps.killed_qa_mob = 1."}}}
	if err := store.Save(validKillQuestCredit); err != nil {
		t.Fatalf("expected valid spawn-group kill quest credit to save, got %v", err)
	}
	loadedKillQuestCredit, err := store.Load()
	if err != nil {
		t.Fatalf("load kill quest credit snapshot: %v", err)
	}
	if !reflect.DeepEqual(loadedKillQuestCredit, validKillQuestCredit) {
		t.Fatalf("expected kill quest credit snapshot to round-trip, got %#v want %#v", loadedKillQuestCredit, validKillQuestCredit)
	}
	partialKillQuestCredit := Snapshot{StaticActors: []StaticActor{{EntityID: 29, Name: "PartialKillQuestMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, SpawnGroupRef: "practice.partial_kill_quest_mob", RewardQuestRef: "quest:first_steps", RewardQuestFlag: "killed_qa_mob", RewardQuestTo: 1}}}
	if err := store.Save(partialKillQuestCredit); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for partial spawn-group kill quest credit, got %v", err)
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
	invalidNonSpawnActorWithSpawnHome := Snapshot{StaticActors: []StaticActor{{EntityID: 23, Name: "LeashedStandalone", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, SpawnHome: &worldruntime.PositionSnapshot{MapIndex: 42, X: 1700, Y: 2800}}}}
	if err := store.Save(invalidNonSpawnActorWithSpawnHome); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for non-spawn actor carrying spawn_home, got %v", err)
	}
	invalidSpawnGroupWithZeroMapHome := Snapshot{StaticActors: []StaticActor{{EntityID: 24, Name: "BadHomeMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, SpawnGroupRef: "practice.bad_home", SpawnHome: &worldruntime.PositionSnapshot{MapIndex: 0, X: 1700, Y: 2800}}}}
	if err := store.Save(invalidSpawnGroupWithZeroMapHome); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for spawn group carrying invalid spawn_home, got %v", err)
	}
	duplicateSpawnGroupRef := Snapshot{StaticActors: []StaticActor{
		{EntityID: 15, Name: "PracticeMobAlpha", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, SpawnGroupRef: "practice.mob_alpha"},
		{EntityID: 16, Name: "PracticeMobBeta", MapIndex: 42, X: 1850, Y: 2950, RaceNum: 102, CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy, SpawnGroupRef: "practice.mob_alpha"},
	}}
	if err := store.Save(duplicateSpawnGroupRef); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for duplicate spawn-group refs, got %v", err)
	}
}
