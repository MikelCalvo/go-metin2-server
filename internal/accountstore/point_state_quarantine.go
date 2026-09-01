package accountstore

import (
	"errors"
	"fmt"
	"sort"
)

// ErrInvalidCharacterPointStateExport reports that a retained point-state
// export failed the 0011 migration-shaped quarantine contract.
var ErrInvalidCharacterPointStateExport = errors.New("invalid character point-state export")

// CharacterPointStateQuarantineSummary is the metadata-only result of validating
// or quarantining a retained character point-state export. It never includes
// point values, SQL, DSNs, or account snapshot bytes.
type CharacterPointStateQuarantineSummary struct {
	CharacterCount int      `json:"character_count"`
	PointRowCount  int      `json:"point_row_count"`
	CharacterIDs   []uint32 `json:"character_ids"`
}

// CharacterPointStateQuarantineResult pairs the metadata-only quarantine summary
// with a canonicalized export ready for later offline review or backfill tools.
type CharacterPointStateQuarantineResult struct {
	Summary CharacterPointStateQuarantineSummary `json:"summary"`
	Export  CharacterPointStateExport            `json:"export"`
}

// ValidateCharacterPointStateExport fails closed when a retained export does not
// match the 0011_character_point_state shape. It does not open a database, write
// account snapshots, or mutate the supplied export.
func ValidateCharacterPointStateExport(export CharacterPointStateExport) (CharacterPointStateQuarantineSummary, error) {
	canonical, summary, err := canonicalizeCharacterPointStateExport(export)
	if err != nil {
		return CharacterPointStateQuarantineSummary{}, err
	}
	_ = canonical
	return summary, nil
}

// QuarantineCharacterPointStateExport validates a retained export and returns a
// canonicalized copy grouped by ascending character_id with complete 0..254
// point vectors. Declared character_ids merge with point row-derived ids so a
// listed character with zero point rows can wipe-to-empty under scoped replace.
// It never opens a database or mutates account snapshots.
func QuarantineCharacterPointStateExport(export CharacterPointStateExport) (CharacterPointStateExport, CharacterPointStateQuarantineSummary, error) {
	return canonicalizeCharacterPointStateExport(export)
}

func canonicalizeCharacterPointStateExport(export CharacterPointStateExport) (CharacterPointStateExport, CharacterPointStateQuarantineSummary, error) {
	if export.MigrationVersion != CharacterPointStateMigrationVersion {
		return CharacterPointStateExport{}, CharacterPointStateQuarantineSummary{}, fmt.Errorf("%w: migration_version %d", ErrInvalidCharacterPointStateExport, export.MigrationVersion)
	}
	if export.MigrationName != CharacterPointStateMigrationName {
		return CharacterPointStateExport{}, CharacterPointStateQuarantineSummary{}, fmt.Errorf("%w: migration_name %q", ErrInvalidCharacterPointStateExport, export.MigrationName)
	}
	if export.Points == nil {
		return CharacterPointStateExport{}, CharacterPointStateQuarantineSummary{}, fmt.Errorf("%w: points must be present", ErrInvalidCharacterPointStateExport)
	}

	characterIDs := make(map[uint32]struct{}, len(export.Points)/characterPointStatePointCount+1+len(export.CharacterIDs))
	if export.CharacterIDs != nil {
		seenDeclaredIDs := make(map[uint32]struct{}, len(export.CharacterIDs))
		for _, characterID := range export.CharacterIDs {
			if characterID == 0 {
				return CharacterPointStateExport{}, CharacterPointStateQuarantineSummary{}, fmt.Errorf("%w: character_ids entries must be > 0", ErrInvalidCharacterPointStateExport)
			}
			if _, exists := seenDeclaredIDs[characterID]; exists {
				return CharacterPointStateExport{}, CharacterPointStateQuarantineSummary{}, fmt.Errorf("%w: duplicate character_ids entry %d", ErrInvalidCharacterPointStateExport, characterID)
			}
			seenDeclaredIDs[characterID] = struct{}{}
			characterIDs[characterID] = struct{}{}
		}
	}

	byCharacter := make(map[uint32]map[uint8]int32, len(export.Points)/characterPointStatePointCount+1)
	for _, row := range export.Points {
		if row.CharacterID == 0 {
			return CharacterPointStateExport{}, CharacterPointStateQuarantineSummary{}, fmt.Errorf("%w: character_id must be > 0", ErrInvalidCharacterPointStateExport)
		}
		if int(row.PointIndex) >= characterPointStatePointCount {
			return CharacterPointStateExport{}, CharacterPointStateQuarantineSummary{}, fmt.Errorf("%w: character %d point_index %d out of range", ErrInvalidCharacterPointStateExport, row.CharacterID, row.PointIndex)
		}
		points, ok := byCharacter[row.CharacterID]
		if !ok {
			points = make(map[uint8]int32, characterPointStatePointCount)
			byCharacter[row.CharacterID] = points
		}
		if _, exists := points[row.PointIndex]; exists {
			return CharacterPointStateExport{}, CharacterPointStateQuarantineSummary{}, fmt.Errorf("%w: duplicate character_id=%d point_index=%d", ErrInvalidCharacterPointStateExport, row.CharacterID, row.PointIndex)
		}
		points[row.PointIndex] = row.Value
		characterIDs[row.CharacterID] = struct{}{}
	}

	for characterID, points := range byCharacter {
		if len(points) != characterPointStatePointCount {
			return CharacterPointStateExport{}, CharacterPointStateQuarantineSummary{}, fmt.Errorf("%w: character %d has %d point rows; expected %d", ErrInvalidCharacterPointStateExport, characterID, len(points), characterPointStatePointCount)
		}
		for index := 0; index < characterPointStatePointCount; index++ {
			if _, ok := points[uint8(index)]; !ok {
				return CharacterPointStateExport{}, CharacterPointStateQuarantineSummary{}, fmt.Errorf("%w: character %d missing point_index %d", ErrInvalidCharacterPointStateExport, characterID, index)
			}
		}
	}

	sortedCharacterIDs := make([]uint32, 0, len(characterIDs))
	for characterID := range characterIDs {
		sortedCharacterIDs = append(sortedCharacterIDs, characterID)
	}
	sort.Slice(sortedCharacterIDs, func(i, j int) bool { return sortedCharacterIDs[i] < sortedCharacterIDs[j] })

	canonical := CharacterPointStateExport{
		MigrationVersion: CharacterPointStateMigrationVersion,
		MigrationName:    CharacterPointStateMigrationName,
		CharacterIDs:     append([]uint32(nil), sortedCharacterIDs...),
		Points:           make([]CharacterPointRow, 0, len(byCharacter)*characterPointStatePointCount),
	}
	if canonical.CharacterIDs == nil {
		canonical.CharacterIDs = []uint32{}
	}
	for _, characterID := range sortedCharacterIDs {
		points, ok := byCharacter[characterID]
		if !ok {
			continue
		}
		for index := 0; index < characterPointStatePointCount; index++ {
			canonical.Points = append(canonical.Points, CharacterPointRow{
				CharacterID: characterID,
				PointIndex:  uint8(index),
				Value:       points[uint8(index)],
			})
		}
	}

	summary := CharacterPointStateQuarantineSummary{
		CharacterCount: len(sortedCharacterIDs),
		PointRowCount:  len(canonical.Points),
		CharacterIDs:   sortedCharacterIDs,
	}
	if summary.CharacterIDs == nil {
		summary.CharacterIDs = []uint32{}
	}
	return canonical, summary, nil
}
