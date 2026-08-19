package queststate

import (
	"errors"
	"reflect"
	"testing"
)

func TestValidateCharacterQuestStateExportAcceptsCanonicalExport(t *testing.T) {
	export := CharacterQuestStateExport{
		MigrationVersion: CharacterQuestStateMigrationVersion,
		MigrationName:    CharacterQuestStateMigrationName,
		Flags: []CharacterQuestFlagRow{
			{CharacterID: 101, Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "step", Value: 1},
			{CharacterID: 202, Character: "AnotherHero", QuestRef: "quest:first_steps", Flag: "met_guard", Value: 1},
		},
	}

	summary, err := ValidateCharacterQuestStateExport(export)
	if err != nil {
		t.Fatalf("validate character quest-state export: %v", err)
	}
	want := CharacterQuestStateQuarantineSummary{
		CharacterCount: 2,
		FlagCount:      2,
		CharacterIDs:   []uint32{101, 202},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected quarantine summary:\n got: %#v\nwant: %#v", summary, want)
	}
}

func TestValidateCharacterQuestStateExportAcceptsEmptyFlags(t *testing.T) {
	export := CharacterQuestStateExport{
		MigrationVersion: CharacterQuestStateMigrationVersion,
		MigrationName:    CharacterQuestStateMigrationName,
		Flags:            []CharacterQuestFlagRow{},
	}

	summary, err := ValidateCharacterQuestStateExport(export)
	if err != nil {
		t.Fatalf("validate empty character quest-state export: %v", err)
	}
	want := CharacterQuestStateQuarantineSummary{
		CharacterCount: 0,
		FlagCount:      0,
		CharacterIDs:   []uint32{},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected empty quarantine summary:\n got: %#v\nwant: %#v", summary, want)
	}
}

func TestValidateCharacterQuestStateExportRejectsWrongMigrationBoundary(t *testing.T) {
	export := CharacterQuestStateExport{
		MigrationVersion: 3,
		MigrationName:    CharacterQuestStateMigrationName,
		Flags:            []CharacterQuestFlagRow{},
	}
	if _, err := ValidateCharacterQuestStateExport(export); !errors.Is(err, ErrInvalidCharacterQuestStateExport) {
		t.Fatalf("expected ErrInvalidCharacterQuestStateExport, got %v", err)
	}
}

func TestValidateCharacterQuestStateExportRejectsMalformedRows(t *testing.T) {
	cases := []struct {
		name   string
		export CharacterQuestStateExport
	}{
		{
			name: "nil flags",
			export: CharacterQuestStateExport{
				MigrationVersion: CharacterQuestStateMigrationVersion,
				MigrationName:    CharacterQuestStateMigrationName,
			},
		},
		{
			name: "zero character id",
			export: CharacterQuestStateExport{
				MigrationVersion: CharacterQuestStateMigrationVersion,
				MigrationName:    CharacterQuestStateMigrationName,
				Flags: []CharacterQuestFlagRow{
					{CharacterID: 0, Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "step", Value: 1},
				},
			},
		},
		{
			name: "invalid quest ref",
			export: CharacterQuestStateExport{
				MigrationVersion: CharacterQuestStateMigrationVersion,
				MigrationName:    CharacterQuestStateMigrationName,
				Flags: []CharacterQuestFlagRow{
					{CharacterID: 101, Character: "QuestHero", QuestRef: "bad_ref", Flag: "step", Value: 1},
				},
			},
		},
		{
			name: "zero value",
			export: CharacterQuestStateExport{
				MigrationVersion: CharacterQuestStateMigrationVersion,
				MigrationName:    CharacterQuestStateMigrationName,
				Flags: []CharacterQuestFlagRow{
					{CharacterID: 101, Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "step", Value: 0},
				},
			},
		},
		{
			name: "duplicate key",
			export: CharacterQuestStateExport{
				MigrationVersion: CharacterQuestStateMigrationVersion,
				MigrationName:    CharacterQuestStateMigrationName,
				Flags: []CharacterQuestFlagRow{
					{CharacterID: 101, Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "step", Value: 1},
					{CharacterID: 101, Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "step", Value: 2},
				},
			},
		},
		{
			name: "character id name mismatch",
			export: CharacterQuestStateExport{
				MigrationVersion: CharacterQuestStateMigrationVersion,
				MigrationName:    CharacterQuestStateMigrationName,
				Flags: []CharacterQuestFlagRow{
					{CharacterID: 101, Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "step", Value: 1},
					{CharacterID: 101, Character: "OtherHero", QuestRef: "quest:first_steps", Flag: "met_guide", Value: 1},
				},
			},
		},
		{
			name: "character name id mismatch",
			export: CharacterQuestStateExport{
				MigrationVersion: CharacterQuestStateMigrationVersion,
				MigrationName:    CharacterQuestStateMigrationName,
				Flags: []CharacterQuestFlagRow{
					{CharacterID: 101, Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "step", Value: 1},
					{CharacterID: 202, Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "met_guide", Value: 1},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateCharacterQuestStateExport(tc.export); !errors.Is(err, ErrInvalidCharacterQuestStateExport) {
				t.Fatalf("expected ErrInvalidCharacterQuestStateExport, got %v", err)
			}
		})
	}
}

func TestQuarantineCharacterQuestStateExportCanonicalizesRowOrder(t *testing.T) {
	export := CharacterQuestStateExport{
		MigrationVersion: CharacterQuestStateMigrationVersion,
		MigrationName:    CharacterQuestStateMigrationName,
		Flags: []CharacterQuestFlagRow{
			{CharacterID: 202, Character: "AnotherHero", QuestRef: "quest:second_arc", Flag: "zeta", Value: 3},
			{CharacterID: 101, Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "step", Value: 2},
			{CharacterID: 202, Character: "AnotherHero", QuestRef: "quest:first_steps", Flag: "met_guard", Value: 1},
			{CharacterID: 101, Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "met_guide", Value: 1},
		},
	}

	quarantined, summary, err := QuarantineCharacterQuestStateExport(export)
	if err != nil {
		t.Fatalf("quarantine character quest-state export: %v", err)
	}
	wantSummary := CharacterQuestStateQuarantineSummary{
		CharacterCount: 2,
		FlagCount:      4,
		CharacterIDs:   []uint32{101, 202},
	}
	if !reflect.DeepEqual(summary, wantSummary) {
		t.Fatalf("unexpected quarantine summary:\n got: %#v\nwant: %#v", summary, wantSummary)
	}
	wantFlags := []CharacterQuestFlagRow{
		{CharacterID: 101, Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "met_guide", Value: 1},
		{CharacterID: 101, Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "step", Value: 2},
		{CharacterID: 202, Character: "AnotherHero", QuestRef: "quest:first_steps", Flag: "met_guard", Value: 1},
		{CharacterID: 202, Character: "AnotherHero", QuestRef: "quest:second_arc", Flag: "zeta", Value: 3},
	}
	if !reflect.DeepEqual(quarantined.Flags, wantFlags) {
		t.Fatalf("unexpected canonicalized flags:\n got: %#v\nwant: %#v", quarantined.Flags, wantFlags)
	}
}
