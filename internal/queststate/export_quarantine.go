package queststate

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrInvalidCharacterQuestStateExport reports that a retained quest-state
// export failed the 0004 migration-shaped quarantine contract.
var ErrInvalidCharacterQuestStateExport = errors.New("invalid character quest-state export")

// CharacterQuestStateQuarantineSummary is the metadata-only result of validating
// or quarantining a retained character quest-state export. It never includes
// quest scripts, SQL, DSNs, or quest-state snapshot bytes.
type CharacterQuestStateQuarantineSummary struct {
	CharacterCount int      `json:"character_count"`
	FlagCount      int      `json:"flag_count"`
	CharacterIDs   []uint32 `json:"character_ids"`
}

// CharacterQuestStateQuarantineResult pairs the metadata-only quarantine summary
// with a canonicalized export ready for later offline review or backfill tools.
type CharacterQuestStateQuarantineResult struct {
	Summary CharacterQuestStateQuarantineSummary `json:"summary"`
	Export  CharacterQuestStateExport            `json:"export"`
}

// ValidateCharacterQuestStateExport fails closed when a retained export does not
// match the 0004_character_quest_state shape. It does not open a database, write
// quest-state snapshots, or mutate the supplied export.
func ValidateCharacterQuestStateExport(export CharacterQuestStateExport) (CharacterQuestStateQuarantineSummary, error) {
	canonical, summary, err := canonicalizeCharacterQuestStateExport(export)
	if err != nil {
		return CharacterQuestStateQuarantineSummary{}, err
	}
	_ = canonical
	return summary, nil
}

// QuarantineCharacterQuestStateExport validates a retained export and returns a
// canonicalized copy ordered by ascending character_id, then quest_ref, then
// flag. It never opens a database or mutates quest-state snapshots.
func QuarantineCharacterQuestStateExport(export CharacterQuestStateExport) (CharacterQuestStateExport, CharacterQuestStateQuarantineSummary, error) {
	return canonicalizeCharacterQuestStateExport(export)
}

func canonicalizeCharacterQuestStateExport(export CharacterQuestStateExport) (CharacterQuestStateExport, CharacterQuestStateQuarantineSummary, error) {
	if export.MigrationVersion != CharacterQuestStateMigrationVersion {
		return CharacterQuestStateExport{}, CharacterQuestStateQuarantineSummary{}, fmt.Errorf("%w: migration_version %d", ErrInvalidCharacterQuestStateExport, export.MigrationVersion)
	}
	if export.MigrationName != CharacterQuestStateMigrationName {
		return CharacterQuestStateExport{}, CharacterQuestStateQuarantineSummary{}, fmt.Errorf("%w: migration_name %q", ErrInvalidCharacterQuestStateExport, export.MigrationName)
	}
	if export.Flags == nil {
		return CharacterQuestStateExport{}, CharacterQuestStateQuarantineSummary{}, fmt.Errorf("%w: flags must be present", ErrInvalidCharacterQuestStateExport)
	}

	seenKeys := make(map[string]struct{}, len(export.Flags))
	characterNamesByID := make(map[uint32]string, len(export.Flags))
	characterIDsByName := make(map[string]uint32, len(export.Flags))
	characterIDs := make(map[uint32]struct{}, len(export.Flags))
	flags := make([]CharacterQuestFlagRow, 0, len(export.Flags))

	for _, row := range export.Flags {
		characterName := strings.TrimSpace(row.Character)
		questRef := strings.TrimSpace(row.QuestRef)
		flagName := strings.TrimSpace(row.Flag)
		if row.CharacterID == 0 {
			return CharacterQuestStateExport{}, CharacterQuestStateQuarantineSummary{}, fmt.Errorf("%w: character_id must be > 0", ErrInvalidCharacterQuestStateExport)
		}
		if !validCharacterName(characterName) {
			return CharacterQuestStateExport{}, CharacterQuestStateQuarantineSummary{}, fmt.Errorf("%w: invalid character %q", ErrInvalidCharacterQuestStateExport, row.Character)
		}
		if !validQuestRef(questRef) {
			return CharacterQuestStateExport{}, CharacterQuestStateQuarantineSummary{}, fmt.Errorf("%w: invalid quest_ref %q", ErrInvalidCharacterQuestStateExport, row.QuestRef)
		}
		if !validFlagName(flagName) {
			return CharacterQuestStateExport{}, CharacterQuestStateQuarantineSummary{}, fmt.Errorf("%w: invalid flag %q", ErrInvalidCharacterQuestStateExport, row.Flag)
		}
		if row.Value == 0 {
			return CharacterQuestStateExport{}, CharacterQuestStateQuarantineSummary{}, fmt.Errorf("%w: flag value must be > 0", ErrInvalidCharacterQuestStateExport)
		}

		if previousName, ok := characterNamesByID[row.CharacterID]; ok && previousName != characterName {
			return CharacterQuestStateExport{}, CharacterQuestStateQuarantineSummary{}, fmt.Errorf("%w: character_id %d maps to both %q and %q", ErrInvalidCharacterQuestStateExport, row.CharacterID, previousName, characterName)
		}
		normalizedName := strings.ToLower(characterName)
		if previousID, ok := characterIDsByName[normalizedName]; ok && previousID != row.CharacterID {
			return CharacterQuestStateExport{}, CharacterQuestStateQuarantineSummary{}, fmt.Errorf("%w: character %q maps to both ids %d and %d", ErrInvalidCharacterQuestStateExport, characterName, previousID, row.CharacterID)
		}
		key := fmt.Sprintf("%d\x00%s\x00%s", row.CharacterID, questRef, flagName)
		if _, exists := seenKeys[key]; exists {
			return CharacterQuestStateExport{}, CharacterQuestStateQuarantineSummary{}, fmt.Errorf("%w: duplicate character_id=%d quest_ref=%q flag=%q", ErrInvalidCharacterQuestStateExport, row.CharacterID, questRef, flagName)
		}

		seenKeys[key] = struct{}{}
		characterNamesByID[row.CharacterID] = characterName
		characterIDsByName[normalizedName] = row.CharacterID
		characterIDs[row.CharacterID] = struct{}{}
		flags = append(flags, CharacterQuestFlagRow{
			CharacterID: row.CharacterID,
			Character:   characterName,
			QuestRef:    questRef,
			Flag:        flagName,
			Value:       row.Value,
		})
	}

	sort.Slice(flags, func(i, j int) bool {
		if flags[i].CharacterID == flags[j].CharacterID {
			if flags[i].QuestRef == flags[j].QuestRef {
				return flags[i].Flag < flags[j].Flag
			}
			return flags[i].QuestRef < flags[j].QuestRef
		}
		return flags[i].CharacterID < flags[j].CharacterID
	})

	sortedCharacterIDs := make([]uint32, 0, len(characterIDs))
	for characterID := range characterIDs {
		sortedCharacterIDs = append(sortedCharacterIDs, characterID)
	}
	sort.Slice(sortedCharacterIDs, func(i, j int) bool { return sortedCharacterIDs[i] < sortedCharacterIDs[j] })

	canonical := CharacterQuestStateExport{
		MigrationVersion: CharacterQuestStateMigrationVersion,
		MigrationName:    CharacterQuestStateMigrationName,
		Flags:            flags,
	}
	summary := CharacterQuestStateQuarantineSummary{
		CharacterCount: len(sortedCharacterIDs),
		FlagCount:      len(canonical.Flags),
		CharacterIDs:   sortedCharacterIDs,
	}
	if summary.CharacterIDs == nil {
		summary.CharacterIDs = []uint32{}
	}
	return canonical, summary, nil
}
