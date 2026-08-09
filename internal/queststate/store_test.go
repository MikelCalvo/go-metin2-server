package queststate

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestApplyTransitionInitializesMissingQuestFlag(t *testing.T) {
	got, result := ApplyTransition(Snapshot{}, Transition{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Flag:      "step",
		From:      0,
		To:        1,
	})
	if !result.Applied || result.Reason != "" || result.CurrentValue != 0 {
		t.Fatalf("unexpected transition result: %+v", result)
	}
	want := Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected initialized quest snapshot:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestApplyTransitionClearsExistingQuestFlag(t *testing.T) {
	got, result := ApplyTransition(Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}, Transition{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Flag:      "step",
		From:      1,
		To:        0,
	})
	if !result.Applied || result.Reason != "" || result.CurrentValue != 1 {
		t.Fatalf("unexpected clear transition result: %+v", result)
	}
	want := Snapshot{Flags: []Flag{}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected cleared quest snapshot:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestApplyTransitionFailsClosedWhenCurrentValueDoesNotMatch(t *testing.T) {
	before := Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}
	got, result := ApplyTransition(before, Transition{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Flag:      "step",
		From:      0,
		To:        2,
	})
	if result.Applied || result.Reason != TransitionReasonCurrentValueMismatch || result.CurrentValue != 1 {
		t.Fatalf("unexpected mismatch transition result: %+v", result)
	}
	if !reflect.DeepEqual(got, before) {
		t.Fatalf("expected failed transition not to mutate snapshot:\n got: %#v\nwant: %#v", got, before)
	}
}

func TestApplyTransitionRejectsInvalidIdentityWithoutMutation(t *testing.T) {
	before := Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}
	for _, transition := range []Transition{
		{Character: "", QuestRef: "quest:first_steps", Flag: "step", From: 1, To: 2},
		{Character: "QuestHero", QuestRef: "first_steps", Flag: "step", From: 1, To: 2},
		{Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "Step", From: 1, To: 2},
	} {
		got, result := ApplyTransition(before, transition)
		if result.Applied || result.Reason != TransitionReasonInvalidTransition {
			t.Fatalf("expected invalid transition to fail closed, got result=%+v transition=%+v", result, transition)
		}
		if !reflect.DeepEqual(got, before) {
			t.Fatalf("expected invalid transition not to mutate snapshot:\n got: %#v\nwant: %#v", got, before)
		}
	}
}

func TestFileStoreSaveThenLoadRoundTripDeterministicQuestFlags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	store := NewFileStore(path)
	input := Snapshot{Flags: []Flag{
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 2},
		{Character: "AnotherHero", QuestRef: "quest:first_steps", Name: "met_guard", Value: 1},
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "met_guard", Value: 1},
	}}
	if err := store.Save(input); err != nil {
		t.Fatalf("save quest state: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read quest state snapshot: %v", err)
	}
	wantRaw := "{\n  \"flags\": [\n    {\n      \"character\": \"AnotherHero\",\n      \"quest_ref\": \"quest:first_steps\",\n      \"name\": \"met_guard\",\n      \"value\": 1\n    },\n    {\n      \"character\": \"QuestHero\",\n      \"quest_ref\": \"quest:first_steps\",\n      \"name\": \"met_guard\",\n      \"value\": 1\n    },\n    {\n      \"character\": \"QuestHero\",\n      \"quest_ref\": \"quest:first_steps\",\n      \"name\": \"step\",\n      \"value\": 2\n    }\n  ]\n}\n"
	if string(raw) != wantRaw {
		t.Fatalf("unexpected deterministic quest state JSON:\n got: %s\nwant: %s", string(raw), wantRaw)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load quest state: %v", err)
	}
	want := Snapshot{Flags: []Flag{
		{Character: "AnotherHero", QuestRef: "quest:first_steps", Name: "met_guard", Value: 1},
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "met_guard", Value: 1},
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 2},
	}}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("unexpected loaded quest snapshot:\n got: %#v\nwant: %#v", loaded, want)
	}
}

func TestFileStoreLoadReturnsNotFoundForMissingSnapshot(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "missing", "quest-state.json"))
	_, err := store.Load()
	if !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected ErrSnapshotNotFound, got %v", err)
	}
}

func TestFileStoreRejectsMalformedOrInvalidQuestSnapshots(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "null_root", body: `null`},
		{name: "null_flags", body: `{"flags":null}`},
		{name: "unknown_root_field", body: `{"flags":[],"unknown":true}`},
		{name: "unknown_flag_field", body: `{"flags":[{"character":"QuestHero","quest_ref":"quest:first_steps","name":"step","value":1,"unknown":true}]}`},
		{name: "trailing_json", body: `{"flags":[]} {}`},
		{name: "blank_character", body: `{"flags":[{"character":" ","quest_ref":"quest:first_steps","name":"step","value":1}]}`},
		{name: "bad_quest_ref", body: `{"flags":[{"character":"QuestHero","quest_ref":"first_steps","name":"step","value":1}]}`},
		{name: "bad_flag_name", body: `{"flags":[{"character":"QuestHero","quest_ref":"quest:first_steps","name":"Step","value":1}]}`},
		{name: "duplicate_flag", body: `{"flags":[{"character":"QuestHero","quest_ref":"quest:first_steps","name":"step","value":1},{"character":" QuestHero ","quest_ref":" quest:first_steps ","name":" step ","value":2}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
				t.Fatalf("write %s snapshot: %v", tc.name, err)
			}
			_, err := store.Load()
			if !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot for %s, got %v", tc.name, err)
			}
		})
	}
}

func TestFileStoreRejectsSymlinkedCommittedQuestSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	target := filepath.Join(t.TempDir(), "outside-quest-state.json")
	if err := os.WriteFile(target, []byte(`{"flags":[]}`), 0o644); err != nil {
		t.Fatalf("write outside quest snapshot: %v", err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	_, err := store.Load()
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for symlinked quest snapshot, got %v", err)
	}
}
