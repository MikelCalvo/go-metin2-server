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

func TestFileStoreValidateSummarizesQuestFlagsAndCrashTemps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Flags: []Flag{
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 2},
		{Character: "AnotherHero", QuestRef: "quest:first_steps", Name: "met_guard", Value: 1},
		{Character: "QuestHero", QuestRef: "quest:daily_check", Name: "talked_to_guide", Value: 1},
	}}); err != nil {
		t.Fatalf("save quest state: %v", err)
	}
	for _, name := range []string{".quest-state-b.json", ".quest-state-a.json", ".not-quest-state.json"} {
		if err := os.WriteFile(filepath.Join(filepath.Dir(path), name), []byte(`{"flags":[]}`), 0o644); err != nil {
			t.Fatalf("write crash temp %s: %v", name, err)
		}
	}

	summary, err := store.Validate()
	if err != nil {
		t.Fatalf("validate quest state store: %v", err)
	}
	want := SnapshotSummary{
		FlagCount: 3,
		Characters: []string{
			"AnotherHero",
			"QuestHero",
		},
		QuestRefs: []string{
			"quest:daily_check",
			"quest:first_steps",
		},
		FlagKeys: []string{
			"AnotherHero:quest:first_steps:met_guard",
			"QuestHero:quest:daily_check:talked_to_guide",
			"QuestHero:quest:first_steps:step",
		},
		CrashTempCount: 2,
		CrashTempFiles: []string{
			".quest-state-a.json",
			".quest-state-b.json",
		},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected quest state summary:\n got: %#v\nwant: %#v", summary, want)
	}
}

func TestFileStoreValidateTreatsMissingSnapshotAsEmptySummary(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "missing", "quest-state.json"))
	summary, err := store.Validate()
	if err != nil {
		t.Fatalf("validate missing quest state snapshot: %v", err)
	}
	want := SnapshotSummary{Characters: []string{}, QuestRefs: []string{}, FlagKeys: []string{}}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected missing-snapshot summary:\n got: %#v\nwant: %#v", summary, want)
	}
}

func TestFileStoreCleanupCrashTempFilesRemovesOnlyQuestStateTemps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}); err != nil {
		t.Fatalf("save quest state: %v", err)
	}
	for _, name := range []string{".quest-state-a.json", ".quest-state-b.json", ".quest-state-ignore.tmp"} {
		if err := os.WriteFile(filepath.Join(filepath.Dir(path), name), []byte(`{"flags":[]}`), 0o644); err != nil {
			t.Fatalf("write temp %s: %v", name, err)
		}
	}

	summary, err := store.CleanupCrashTempFiles()
	if err != nil {
		t.Fatalf("cleanup quest state crash temps: %v", err)
	}
	if summary.CrashTempCount != 0 || len(summary.CrashTempFiles) != 0 {
		t.Fatalf("expected no crash temps after cleanup, got %+v", summary)
	}
	for _, name := range []string{".quest-state-a.json", ".quest-state-b.json"} {
		if _, err := os.Stat(filepath.Join(filepath.Dir(path), name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected %s to be removed, stat err=%v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), ".quest-state-ignore.tmp")); err != nil {
		t.Fatalf("expected non-crash temp to remain, stat err=%v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected committed quest state snapshot to remain, stat err=%v", err)
	}
}

func TestFileStoreApplyTransitionInitializesMissingSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	store := NewFileStore(path)

	result, err := store.ApplyTransition(Transition{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Flag:      "step",
		From:      0,
		To:        1,
	})
	if err != nil {
		t.Fatalf("apply quest state transition: %v", err)
	}
	if !result.Result.Applied || result.Result.Reason != "" || result.Result.CurrentValue != 0 {
		t.Fatalf("unexpected transition result: %+v", result.Result)
	}
	wantSummary := SnapshotSummary{FlagCount: 1, Characters: []string{"QuestHero"}, QuestRefs: []string{"quest:first_steps"}, FlagKeys: []string{"QuestHero:quest:first_steps:step"}}
	if !reflect.DeepEqual(result.Summary, wantSummary) {
		t.Fatalf("unexpected post-transition summary:\n got: %#v\nwant: %#v", result.Summary, wantSummary)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load applied quest state snapshot: %v", err)
	}
	wantSnapshot := Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}
	if !reflect.DeepEqual(loaded, wantSnapshot) {
		t.Fatalf("unexpected applied quest state snapshot:\n got: %#v\nwant: %#v", loaded, wantSnapshot)
	}
}

func TestFileStoreApplyTransitionMismatchDoesNotRewriteSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	store := NewFileStore(path)
	before := Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}
	if err := store.Save(before); err != nil {
		t.Fatalf("save initial quest state snapshot: %v", err)
	}
	rawBefore, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read initial quest state snapshot: %v", err)
	}

	result, err := store.ApplyTransition(Transition{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Flag:      "step",
		From:      0,
		To:        2,
	})
	if err != nil {
		t.Fatalf("apply mismatched quest state transition: %v", err)
	}
	if result.Result.Applied || result.Result.Reason != TransitionReasonCurrentValueMismatch || result.Result.CurrentValue != 1 {
		t.Fatalf("unexpected mismatch transition result: %+v", result.Result)
	}
	rawAfter, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read post-mismatch quest state snapshot: %v", err)
	}
	if string(rawAfter) != string(rawBefore) {
		t.Fatalf("expected mismatched transition not to rewrite snapshot:\n before: %s\n after: %s", string(rawBefore), string(rawAfter))
	}
}

func TestFileStoreApplyTransitionInvalidIdentityDoesNotCreateSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	store := NewFileStore(path)

	result, err := store.ApplyTransition(Transition{
		Character: "QuestHero",
		QuestRef:  "first_steps",
		Flag:      "step",
		From:      0,
		To:        1,
	})
	if err != nil {
		t.Fatalf("apply invalid quest state transition: %v", err)
	}
	if result.Result.Applied || result.Result.Reason != TransitionReasonInvalidTransition {
		t.Fatalf("unexpected invalid transition result: %+v", result.Result)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected invalid transition not to create snapshot, stat err=%v", err)
	}
}

func TestExportCharacterQuestStateBuildsDeterministicRowsMatchingMigrationShape(t *testing.T) {
	snapshot := Snapshot{Flags: []Flag{
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 2},
		{Character: "AnotherHero", QuestRef: "quest:first_steps", Name: "met_guard", Value: 1},
	}}

	export, err := ExportCharacterQuestState(snapshot, map[string]uint32{
		"questhero":   101,
		"AnotherHero": 202,
	})
	if err != nil {
		t.Fatalf("export character quest state: %v", err)
	}
	if export.MigrationVersion != CharacterQuestStateMigrationVersion || export.MigrationName != CharacterQuestStateMigrationName {
		t.Fatalf("unexpected migration boundary: version=%d name=%q", export.MigrationVersion, export.MigrationName)
	}
	want := []CharacterQuestFlagRow{
		{CharacterID: 202, Character: "AnotherHero", QuestRef: "quest:first_steps", Flag: "met_guard", Value: 1},
		{CharacterID: 101, Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "step", Value: 2},
	}
	if !reflect.DeepEqual(export.Flags, want) {
		t.Fatalf("unexpected character quest-state rows:\n got: %#v\nwant: %#v", export.Flags, want)
	}

	exportAgain, err := ExportCharacterQuestState(snapshot, map[string]uint32{"AnotherHero": 202, "QuestHero": 101})
	if err != nil {
		t.Fatalf("export character quest state again: %v", err)
	}
	if !reflect.DeepEqual(export, exportAgain) {
		t.Fatalf("expected deterministic quest-state export:\n first: %#v\nsecond: %#v", export, exportAgain)
	}
}

func TestExportCharacterQuestStateRejectsRowsThatCannotTargetMigrationSchema(t *testing.T) {
	validSnapshot := Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}

	cases := []struct {
		name               string
		snapshot           Snapshot
		characterIDsByName map[string]uint32
	}{
		{
			name:               "unknown character",
			snapshot:           validSnapshot,
			characterIDsByName: map[string]uint32{"AnotherHero": 202},
		},
		{
			name:               "zero character id",
			snapshot:           validSnapshot,
			characterIDsByName: map[string]uint32{"QuestHero": 0},
		},
		{
			name:               "invalid snapshot",
			snapshot:           Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "bad_ref", Name: "step", Value: 1}}},
			characterIDsByName: map[string]uint32{"QuestHero": 101},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ExportCharacterQuestState(tc.snapshot, tc.characterIDsByName)
			if !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot, got %v", err)
			}
		})
	}
}

func TestFileStoreExportCharacterQuestStateReadsCommittedSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}); err != nil {
		t.Fatalf("save quest state snapshot: %v", err)
	}

	export, err := store.ExportCharacterQuestState(map[string]uint32{"QuestHero": 101})
	if err != nil {
		t.Fatalf("file-store character quest-state export: %v", err)
	}
	if len(export.Flags) != 1 || export.Flags[0].CharacterID != 101 || export.Flags[0].Character != "QuestHero" || export.Flags[0].Flag != "step" {
		t.Fatalf("unexpected file-store quest-state export rows: %#v", export.Flags)
	}
}

func TestFileStoreExportCharacterQuestStateTreatsMissingSnapshotAsEmptyExport(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "missing", "quest-state.json"))

	export, err := store.ExportCharacterQuestState(nil)
	if err != nil {
		t.Fatalf("export missing quest state snapshot: %v", err)
	}
	if export.MigrationVersion != CharacterQuestStateMigrationVersion || len(export.Flags) != 0 {
		t.Fatalf("expected empty quest-state export for missing snapshot, got %#v", export)
	}
}

func TestFileStoreValidateRejectsSymlinkedQuestStateCrashTemp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "quest-state.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	target := filepath.Join(t.TempDir(), "outside-quest-state-temp.json")
	if err := os.WriteFile(target, []byte(`{"flags":[]}`), 0o644); err != nil {
		t.Fatalf("write outside quest temp: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(filepath.Dir(path), ".quest-state-symlink.json")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	_, err := store.Validate()
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for symlinked quest crash temp, got %v", err)
	}
}
