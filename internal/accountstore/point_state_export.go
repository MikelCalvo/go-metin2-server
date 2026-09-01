package accountstore

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

const (
	CharacterPointStateMigrationVersion = 11
	CharacterPointStateMigrationName    = "character_point_state"
	characterPointStatePointCount       = 255
)

// CharacterPointStateExport is a deterministic, schema-shaped projection of the
// fixed-width bootstrap character point vector onto the
// 0011_character_point_state migration boundary. It is intentionally a
// data-model/export contract only: it does not open a database, emit SQL, apply
// migrations, or mutate the file store.
type CharacterPointStateExport struct {
	MigrationVersion int    `json:"migration_version"`
	MigrationName    string `json:"migration_name"`
	// CharacterIDs optionally declares the replace/wipe character scope for
	// ImportCharacterPointState(..., Replace: true). When omitted or empty,
	// quarantine derives the scope from point rows (legacy insert-only exports
	// stay valid). Explicit ids merge with point row-derived ids so a listed
	// character with zero point rows can still wipe-to-empty.
	CharacterIDs []uint32            `json:"character_ids,omitempty"`
	Points       []CharacterPointRow `json:"points"`
}

// CharacterPointRow mirrors one fixed-width character point slot frozen by the
// 0011_character_point_state migration.
type CharacterPointRow struct {
	CharacterID uint32 `json:"character_id"`
	PointIndex  uint8  `json:"point_index"`
	Value       int32  `json:"value"`
}

// ExportCharacterPointState validates bootstrap account snapshots and returns
// rows ordered exactly as a future backfill/import tool should process them:
// accounts by normalized login, characters by select-screen slot, and all 255
// point indices in ascending order for each non-empty character. Zero-valued
// points are emitted deliberately so the migration-shaped payload preserves the
// fixed-width client/server point vector without sparse-row ambiguity.
func ExportCharacterPointState(accounts []Account) (CharacterPointStateExport, error) {
	if _, err := ExportAccountCharacterRoster(accounts); err != nil {
		return CharacterPointStateExport{}, err
	}

	ordered := append([]Account(nil), accounts...)
	sort.Slice(ordered, func(i, j int) bool {
		left := strings.ToLower(ordered[i].Login)
		right := strings.ToLower(ordered[j].Login)
		if left != right {
			return left < right
		}
		return ordered[i].Login < ordered[j].Login
	})

	export := CharacterPointStateExport{
		MigrationVersion: CharacterPointStateMigrationVersion,
		MigrationName:    CharacterPointStateMigrationName,
		Points:           []CharacterPointRow{},
	}

	for _, account := range ordered {
		for slot, character := range account.Characters {
			if character.IsEmptySlot() {
				continue
			}
			if slot < 0 || slot >= accountCharacterRosterPlayerSlots {
				return CharacterPointStateExport{}, fmt.Errorf("%w: account %q character slot %d outside point-state roster", ErrInvalidAccount, account.Login, slot)
			}
			if err := validateRosterExportCharacter(account, slot, character, map[uint32]string{}, map[string]uint32{}); err != nil {
				return CharacterPointStateExport{}, err
			}
			appendCharacterPointRows(&export, character)
		}
	}

	return export, nil
}

// ExportCharacterPointState validates and projects the committed file-store
// snapshots onto the 0011 character point-state migration shape. It reads the
// same committed snapshot set as List and applies no mutations.
func (s *FileStore) ExportCharacterPointState() (CharacterPointStateExport, error) {
	accounts, err := s.List()
	if err != nil {
		return CharacterPointStateExport{}, err
	}
	return ExportCharacterPointState(accounts)
}

func appendCharacterPointRows(export *CharacterPointStateExport, character loginticket.Character) {
	for pointIndex, value := range character.Points {
		export.Points = append(export.Points, CharacterPointRow{
			CharacterID: character.ID,
			PointIndex:  uint8(pointIndex),
			Value:       value,
		})
	}
}
