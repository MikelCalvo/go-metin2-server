package safeboxstore

import (
	"path/filepath"
	"testing"
)

func TestExportCharacterSafeboxStateBuildsDeterministicRowsMatchingMigrationShape(t *testing.T) {
	snapshot := Snapshot{Characters: []CharacterRow{{
		Login:       "Alpha",
		CharacterID: 7,
		Password:    "secret",
		Cells: []Cell{
			{Cell: 1, ID: 1002, Vnum: 27002, Count: 1},
			{Cell: 0, ID: 1001, Vnum: 27001, Count: 2, Locked: true},
		},
	}, {
		Login:       "Beta",
		CharacterID: 9,
		Cells:       []Cell{},
	}}}

	export, err := ExportCharacterSafeboxState(snapshot)
	if err != nil {
		t.Fatalf("export character safebox state: %v", err)
	}
	if export.MigrationVersion != CharacterSafeboxStateMigrationVersion || export.MigrationName != CharacterSafeboxStateMigrationName {
		t.Fatalf("unexpected migration boundary: %#v", export)
	}
	if len(export.Passwords) != 2 {
		t.Fatalf("unexpected password rows: %#v", export.Passwords)
	}
	if export.Passwords[0].CharacterID != 7 || export.Passwords[0].Login != "Alpha" || export.Passwords[0].Password != "secret" {
		t.Fatalf("unexpected first password row: %#v", export.Passwords[0])
	}
	if export.Passwords[1].CharacterID != 9 || export.Passwords[1].Login != "Beta" || export.Passwords[1].Password != "" {
		t.Fatalf("unexpected second password row: %#v", export.Passwords[1])
	}
	if len(export.Items) != 2 {
		t.Fatalf("unexpected item rows: %#v", export.Items)
	}
	if export.Items[0].Cell != 0 || export.Items[0].ID != 1001 || !export.Items[0].Locked {
		t.Fatalf("expected cells sorted ascending, got %#v", export.Items)
	}
	if export.Items[1].Cell != 1 || export.Items[1].ID != 1002 {
		t.Fatalf("unexpected second item row: %#v", export.Items[1])
	}

	again, err := ExportCharacterSafeboxState(snapshot)
	if err != nil {
		t.Fatalf("re-export character safebox state: %v", err)
	}
	if len(again.Items) != len(export.Items) || again.Items[0] != export.Items[0] || again.Items[1] != export.Items[1] {
		t.Fatalf("expected deterministic export, first=%#v again=%#v", export.Items, again.Items)
	}
}

func TestExportCharacterSafeboxStateRejectsInvalidSnapshot(t *testing.T) {
	_, err := ExportCharacterSafeboxState(Snapshot{Characters: []CharacterRow{{
		Login:       "Alpha",
		CharacterID: 7,
		Cells:       []Cell{{Cell: 15, ID: 1, Vnum: 27001, Count: 1}},
	}}})
	if err == nil {
		t.Fatal("expected invalid cell to fail closed")
	}
}

func TestFileStoreAndMemoryStoreExportCharacterSafeboxState(t *testing.T) {
	snapshot := Snapshot{Characters: []CharacterRow{{
		Login:       "Alpha",
		CharacterID: 7,
		Password:    "pass01",
		Cells:       []Cell{{Cell: 0, ID: 42, Vnum: 27001, Count: 3}},
	}}}

	memory := NewMemoryStore()
	if err := memory.Save(snapshot); err != nil {
		t.Fatalf("save memory safebox snapshot: %v", err)
	}
	memoryExport, err := memory.ExportCharacterSafeboxState()
	if err != nil {
		t.Fatalf("memory export: %v", err)
	}
	if len(memoryExport.Passwords) != 1 || len(memoryExport.Items) != 1 || memoryExport.Items[0].ID != 42 {
		t.Fatalf("unexpected memory export: %#v", memoryExport)
	}

	emptyMemory := NewMemoryStore()
	emptyExport, err := emptyMemory.ExportCharacterSafeboxState()
	if err != nil {
		t.Fatalf("empty memory export: %v", err)
	}
	if emptyExport.MigrationVersion != CharacterSafeboxStateMigrationVersion || len(emptyExport.Passwords) != 0 || len(emptyExport.Items) != 0 {
		t.Fatalf("unexpected empty memory export: %#v", emptyExport)
	}

	path := filepath.Join(t.TempDir(), "safebox.json")
	store := NewFileStore(path)
	if err := store.Save(snapshot); err != nil {
		t.Fatalf("save file safebox snapshot: %v", err)
	}
	fileExport, err := store.ExportCharacterSafeboxState()
	if err != nil {
		t.Fatalf("file export: %v", err)
	}
	if fileExport.MigrationVersion != CharacterSafeboxStateMigrationVersion || len(fileExport.Items) != 1 || fileExport.Passwords[0].Password != "pass01" {
		t.Fatalf("unexpected file export: %#v", fileExport)
	}

	missing := NewFileStore(filepath.Join(t.TempDir(), "missing.json"))
	missingExport, err := missing.ExportCharacterSafeboxState()
	if err != nil {
		t.Fatalf("missing file export: %v", err)
	}
	if len(missingExport.Passwords) != 0 || len(missingExport.Items) != 0 {
		t.Fatalf("unexpected missing export: %#v", missingExport)
	}
}

func TestQuarantineCharacterSafeboxStateExportCanonicalizesAndRejectsDrift(t *testing.T) {
	export := CharacterSafeboxStateExport{
		MigrationVersion: CharacterSafeboxStateMigrationVersion,
		MigrationName:    CharacterSafeboxStateMigrationName,
		Passwords: []CharacterSafeboxPasswordRow{
			{CharacterID: 9, Login: "Beta", Password: ""},
			{CharacterID: 7, Login: "Alpha", Password: "secret"},
		},
		Items: []CharacterSafeboxItemRow{
			{ID: 1002, CharacterID: 7, Login: "Alpha", Cell: 1, Vnum: 27002, Count: 1},
			{ID: 1001, CharacterID: 7, Login: "Alpha", Cell: 0, Vnum: 27001, Count: 2, Locked: true},
		},
	}

	quarantined, summary, err := QuarantineCharacterSafeboxStateExport(export)
	if err != nil {
		t.Fatalf("quarantine export: %v", err)
	}
	if summary.CharacterCount != 2 || summary.PasswordCount != 2 || summary.ItemCount != 2 {
		t.Fatalf("unexpected quarantine summary: %#v", summary)
	}
	if quarantined.Passwords[0].CharacterID != 7 || quarantined.Items[0].Cell != 0 {
		t.Fatalf("expected canonical ordering, got %#v", quarantined)
	}
	if _, err := ValidateCharacterSafeboxStateExport(quarantined); err != nil {
		t.Fatalf("validate quarantined export: %v", err)
	}

	badVersion := export
	badVersion.MigrationVersion = 13
	if _, _, err := QuarantineCharacterSafeboxStateExport(badVersion); err == nil {
		t.Fatal("expected migration tip mismatch to fail closed")
	}

	orphanItem := export
	orphanItem.Items = append(append([]CharacterSafeboxItemRow{}, export.Items...), CharacterSafeboxItemRow{
		ID: 2001, CharacterID: 99, Login: "Ghost", Cell: 0, Vnum: 27001, Count: 1,
	})
	if _, _, err := QuarantineCharacterSafeboxStateExport(orphanItem); err == nil {
		t.Fatal("expected orphan item character_id to fail closed")
	}
}
