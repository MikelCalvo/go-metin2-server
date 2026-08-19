package queststate

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMemoryStoreLoadSaveRoundTripWithoutFilesystem(t *testing.T) {
	dir := t.TempDir()
	store := NewMemoryStore()

	if _, err := store.Load(); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected ErrSnapshotNotFound before first save, got %v", err)
	}

	seed := Snapshot{Flags: []Flag{
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 2},
		{Character: "AnotherHero", QuestRef: "quest:first_steps", Name: "met_guard", Value: 1},
	}}
	if err := store.Save(seed); err != nil {
		t.Fatalf("save memory quest state: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load memory quest state: %v", err)
	}
	want := NormalizeSnapshot(seed)
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("unexpected loaded snapshot:\n got: %#v\nwant: %#v", loaded, want)
	}

	mutated := false
	for i := range loaded.Flags {
		if loaded.Flags[i].Character == "QuestHero" && loaded.Flags[i].Name == "step" {
			loaded.Flags[i].Value = 99
			mutated = true
			break
		}
	}
	if !mutated {
		t.Fatalf("expected QuestHero step flag in loaded snapshot: %#v", loaded)
	}
	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("reload memory quest state: %v", err)
	}
	for _, flag := range reloaded.Flags {
		if flag.Character == "QuestHero" && flag.Name == "step" && flag.Value != 2 {
			t.Fatalf("memory store leaked caller mutation: %#v", flag)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("memory store wrote filesystem entries: %#v", entries)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, ".quest-state-*.json")); err != nil || len(matches) != 0 {
		t.Fatalf("memory store created crash-temp shaped files: matches=%v err=%v", matches, err)
	}
}

func TestMemoryStoreRejectsInvalidSave(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Save(Snapshot{Flags: []Flag{{Character: "QuestHero", QuestRef: "bad_ref", Name: "step", Value: 1}}}); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot, got %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("invalid save must leave store uncommitted, got %v", err)
	}
}

func TestMemoryStoreExportsMatchFileStoreAndPassQuarantine(t *testing.T) {
	seed := Snapshot{Flags: []Flag{
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 2},
		{Character: "AnotherHero", QuestRef: "quest:first_steps", Name: "met_guard", Value: 1},
	}}
	characterIDsByName := map[string]uint32{
		"QuestHero":   101,
		"AnotherHero": 202,
	}

	fileStore := NewFileStore(filepath.Join(t.TempDir(), "state", "quest-state.json"))
	if err := fileStore.Save(seed); err != nil {
		t.Fatalf("save file store: %v", err)
	}
	memoryStore := NewMemoryStore()
	if err := memoryStore.Save(seed); err != nil {
		t.Fatalf("save memory store: %v", err)
	}

	fileExport, err := fileStore.ExportCharacterQuestState(characterIDsByName)
	if err != nil {
		t.Fatalf("file quest-state export: %v", err)
	}
	memoryExport, err := memoryStore.ExportCharacterQuestState(characterIDsByName)
	if err != nil {
		t.Fatalf("memory quest-state export: %v", err)
	}
	if !reflect.DeepEqual(fileExport, memoryExport) {
		t.Fatalf("quest-state export mismatch:\n file: %#v\nmemory: %#v", fileExport, memoryExport)
	}
	if _, err := ValidateCharacterQuestStateExport(memoryExport); err != nil {
		t.Fatalf("quarantine memory quest-state export: %v", err)
	}
}

func TestMemoryStoreExportTreatsMissingSnapshotAsEmpty(t *testing.T) {
	store := NewMemoryStore()
	export, err := store.ExportCharacterQuestState(nil)
	if err != nil {
		t.Fatalf("export missing memory snapshot: %v", err)
	}
	if export.MigrationVersion != CharacterQuestStateMigrationVersion || export.MigrationName != CharacterQuestStateMigrationName || len(export.Flags) != 0 {
		t.Fatalf("expected empty quest-state export, got %#v", export)
	}
}

func TestMemoryStoreApplyAndPreviewTransition(t *testing.T) {
	store := NewMemoryStore()
	applied, err := store.ApplyTransition(Transition{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Flag:      "step",
		From:      0,
		To:        1,
	})
	if err != nil {
		t.Fatalf("apply transition: %v", err)
	}
	if !applied.Result.Applied || applied.Summary.FlagCount != 1 {
		t.Fatalf("unexpected applied transition: %#v", applied)
	}

	preview, err := store.PreviewTransition(Transition{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Flag:      "step",
		From:      1,
		To:        2,
	})
	if err != nil {
		t.Fatalf("preview transition: %v", err)
	}
	if !preview.Result.Applied || preview.Summary.FlagCount != 1 {
		t.Fatalf("unexpected preview transition: %#v", preview)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load after preview: %v", err)
	}
	if len(loaded.Flags) != 1 || loaded.Flags[0].Value != 1 {
		t.Fatalf("preview mutated committed snapshot: %#v", loaded)
	}
}

func TestMemoryStoreSatisfiesCharacterQuestStateExporter(t *testing.T) {
	var exporter CharacterQuestStateExporter = NewMemoryStore()
	if _, err := exporter.ExportCharacterQuestState(nil); err != nil {
		t.Fatalf("empty quest-state export: %v", err)
	}
}
