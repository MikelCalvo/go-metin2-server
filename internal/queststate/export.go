package queststate

import (
	"errors"
	"fmt"
	"strings"
)

const (
	CharacterQuestStateMigrationVersion = 4
	CharacterQuestStateMigrationName    = "character_quest_state"
)

// CharacterQuestStateExport is a deterministic, schema-shaped projection of the
// bootstrap quest-state snapshot onto the 0004_character_quest_state migration
// boundary. It is intentionally an export/backfill contract only: it does not
// open a database, emit SQL, apply migrations, or mutate the quest-state store.
type CharacterQuestStateExport struct {
	MigrationVersion int                     `json:"migration_version"`
	MigrationName    string                  `json:"migration_name"`
	Flags            []CharacterQuestFlagRow `json:"flags"`
}

// CharacterQuestFlagRow mirrors rows for the durable character_quest_flags table
// frozen by the 0004_character_quest_state migration. Character is included as a
// read-only operator aid for validating the name-to-character-id resolution used
// by the projection; a future SQL import should treat CharacterID as the durable
// foreign-key carrier.
type CharacterQuestFlagRow struct {
	CharacterID uint32 `json:"character_id"`
	Character   string `json:"character"`
	QuestRef    string `json:"quest_ref"`
	Flag        string `json:"flag"`
	Value       uint32 `json:"value"`
}

// ExportCharacterQuestState validates a quest-state snapshot and resolves every
// flag to a known non-zero character id. The supplied characterIDsByName may use
// original or normalized character-name keys; lookups are case-insensitive. Rows
// are returned in the same deterministic order as the normalized snapshot.
func ExportCharacterQuestState(snapshot Snapshot, characterIDsByName map[string]uint32) (CharacterQuestStateExport, error) {
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return CharacterQuestStateExport{}, fmt.Errorf("%w: validate character quest-state export", err)
	}

	ids := make(map[string]uint32, len(characterIDsByName))
	for name, id := range characterIDsByName {
		normalizedName := strings.ToLower(strings.TrimSpace(name))
		if normalizedName == "" || id == 0 {
			return CharacterQuestStateExport{}, fmt.Errorf("%w: invalid character id mapping for %q", ErrInvalidSnapshot, name)
		}
		if previousID, exists := ids[normalizedName]; exists && previousID != id {
			return CharacterQuestStateExport{}, fmt.Errorf("%w: duplicate character id mapping for %q", ErrInvalidSnapshot, name)
		}
		ids[normalizedName] = id
	}

	export := CharacterQuestStateExport{
		MigrationVersion: CharacterQuestStateMigrationVersion,
		MigrationName:    CharacterQuestStateMigrationName,
		Flags:            []CharacterQuestFlagRow{},
	}
	for _, flag := range normalized.Flags {
		id, ok := ids[strings.ToLower(flag.Character)]
		if !ok || id == 0 {
			return CharacterQuestStateExport{}, fmt.Errorf("%w: quest flag for unknown character %q", ErrInvalidSnapshot, flag.Character)
		}
		export.Flags = append(export.Flags, CharacterQuestFlagRow{
			CharacterID: id,
			Character:   flag.Character,
			QuestRef:    flag.QuestRef,
			Flag:        flag.Name,
			Value:       flag.Value,
		})
	}
	return export, nil
}

// ExportCharacterQuestState validates and projects the committed file-store
// snapshot onto the 0004 character quest-state migration shape. Missing
// quest-state snapshots are treated as an empty export, matching Validate().
func (s *FileStore) ExportCharacterQuestState(characterIDsByName map[string]uint32) (CharacterQuestStateExport, error) {
	snapshot, err := s.Load()
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			return CharacterQuestStateExport{
				MigrationVersion: CharacterQuestStateMigrationVersion,
				MigrationName:    CharacterQuestStateMigrationName,
				Flags:            []CharacterQuestFlagRow{},
			}, nil
		}
		return CharacterQuestStateExport{}, err
	}
	return ExportCharacterQuestState(snapshot, characterIDsByName)
}
