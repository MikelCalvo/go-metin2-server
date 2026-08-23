package safeboxstore

import (
	"errors"
	"fmt"
)

const (
	CharacterSafeboxStateMigrationVersion = 14
	CharacterSafeboxStateMigrationName    = "character_safebox_state"
)

// CharacterSafeboxStateExport is a deterministic, schema-shaped projection of
// the durable safebox FileStore onto the 0014_character_safebox_state migration
// boundary. It is intentionally an export/backfill contract only: it does not
// open a database, emit SQL, apply migrations, or mutate the safebox store.
type CharacterSafeboxStateExport struct {
	MigrationVersion int                           `json:"migration_version"`
	MigrationName    string                        `json:"migration_name"`
	Passwords        []CharacterSafeboxPasswordRow `json:"passwords"`
	Items            []CharacterSafeboxItemRow     `json:"items"`
}

// CharacterSafeboxPasswordRow mirrors character_safebox_passwords. Login is an
// operator-aid identity carrier; CharacterID is the durable foreign key.
// Empty Password means bootstrap DefaultPassword at challenge time.
type CharacterSafeboxPasswordRow struct {
	CharacterID uint32 `json:"character_id"`
	Login       string `json:"login"`
	Password    string `json:"password"`
}

// CharacterSafeboxItemRow mirrors character_safebox_items for one durable cell.
type CharacterSafeboxItemRow struct {
	ID          uint64 `json:"id"`
	CharacterID uint32 `json:"character_id"`
	Login       string `json:"login"`
	Cell        uint8  `json:"cell"`
	Vnum        uint32 `json:"vnum"`
	Count       uint16 `json:"count"`
	Locked      bool   `json:"locked,omitempty"`
}

// ExportCharacterSafeboxState validates a safebox snapshot and projects every
// character row onto the 0014 migration shape. Rows are returned in the same
// deterministic order as NormalizeSnapshot (login, character_id, then cell).
func ExportCharacterSafeboxState(snapshot Snapshot) (CharacterSafeboxStateExport, error) {
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return CharacterSafeboxStateExport{}, fmt.Errorf("%w: validate character safebox-state export", err)
	}

	export := CharacterSafeboxStateExport{
		MigrationVersion: CharacterSafeboxStateMigrationVersion,
		MigrationName:    CharacterSafeboxStateMigrationName,
		Passwords:        []CharacterSafeboxPasswordRow{},
		Items:            []CharacterSafeboxItemRow{},
	}
	for _, row := range normalized.Characters {
		export.Passwords = append(export.Passwords, CharacterSafeboxPasswordRow{
			CharacterID: row.CharacterID,
			Login:       row.Login,
			Password:    row.Password,
		})
		for _, cell := range row.Cells {
			export.Items = append(export.Items, CharacterSafeboxItemRow{
				ID:          cell.ID,
				CharacterID: row.CharacterID,
				Login:       row.Login,
				Cell:        cell.Cell,
				Vnum:        cell.Vnum,
				Count:       cell.Count,
				Locked:      cell.Locked,
			})
		}
	}
	return export, nil
}

// ExportCharacterSafeboxState validates and projects the committed file-store
// snapshot onto the 0014 character safebox-state migration shape. Missing
// safebox snapshots are treated as an empty export, matching Validate().
func (s *FileStore) ExportCharacterSafeboxState() (CharacterSafeboxStateExport, error) {
	snapshot, err := s.Load()
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			return emptyCharacterSafeboxStateExport(), nil
		}
		return CharacterSafeboxStateExport{}, err
	}
	return ExportCharacterSafeboxState(snapshot)
}

// ExportCharacterSafeboxState projects the committed hermetic snapshot onto the
// 0014 migration shape. Missing snapshots yield an empty export.
func (s *MemoryStore) ExportCharacterSafeboxState() (CharacterSafeboxStateExport, error) {
	snapshot, err := s.Load()
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			return emptyCharacterSafeboxStateExport(), nil
		}
		return CharacterSafeboxStateExport{}, err
	}
	return ExportCharacterSafeboxState(snapshot)
}

func emptyCharacterSafeboxStateExport() CharacterSafeboxStateExport {
	return CharacterSafeboxStateExport{
		MigrationVersion: CharacterSafeboxStateMigrationVersion,
		MigrationName:    CharacterSafeboxStateMigrationName,
		Passwords:        []CharacterSafeboxPasswordRow{},
		Items:            []CharacterSafeboxItemRow{},
	}
}
