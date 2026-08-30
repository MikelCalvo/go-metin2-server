package safeboxstore

import (
	"errors"
	"fmt"
)

const (
	CharacterSafeboxStateMigrationVersion = 15
	CharacterSafeboxStateMigrationName    = "character_safebox_money"

	CharacterSafeboxItemInstanceSocketsMigrationVersion = 25
	CharacterSafeboxItemInstanceSocketsMigrationName    = "character_safebox_item_instance_sockets"
)

// CharacterSafeboxStateExport is a deterministic, schema-shaped projection of
// the durable safebox FileStore onto the 0015_character_safebox_money migration
// tip (password + warehouse money + cells). It is intentionally an
// export/backfill contract only: it does not open a database, emit SQL, apply
// migrations, or mutate the safebox store.
type CharacterSafeboxStateExport struct {
	MigrationVersion int                           `json:"migration_version"`
	MigrationName    string                        `json:"migration_name"`
	Passwords        []CharacterSafeboxPasswordRow `json:"passwords"`
	Items            []CharacterSafeboxItemRow     `json:"items"`
}

// CharacterSafeboxPasswordRow mirrors character_safebox_passwords after the
// additive 0015 money column. Login is an operator-aid identity carrier;
// CharacterID is the durable foreign key. Empty Password means bootstrap
// DefaultPassword at challenge time. Money is warehouse gold in
// [0, math.MaxInt32]; omitted / zero means 0.
type CharacterSafeboxPasswordRow struct {
	CharacterID uint32 `json:"character_id"`
	Login       string `json:"login"`
	Password    string `json:"password"`
	Money       int64  `json:"money,omitempty"`
}

// CharacterSafeboxItemRow mirrors character_safebox_items for one durable cell,
// including optional additive 0025 instance sockets. HasSockets=false / omitted
// means nil instance sockets (template fallback); HasSockets=true including
// all-zero is authoritative. Export identity stays tip-0015.
type CharacterSafeboxItemRow struct {
	ID          uint64 `json:"id"`
	CharacterID uint32 `json:"character_id"`
	Login       string `json:"login"`
	Cell        uint8  `json:"cell"`
	Vnum        uint32 `json:"vnum"`
	Count       uint16 `json:"count"`
	Locked      bool   `json:"locked,omitempty"`
	HasSockets  bool   `json:"has_sockets,omitempty"`
	Socket0     int32  `json:"socket0,omitempty"`
	Socket1     int32  `json:"socket1,omitempty"`
	Socket2     int32  `json:"socket2,omitempty"`
}

// ExportCharacterSafeboxState validates a safebox snapshot and projects every
// character row onto the 0015 migration tip. Rows are returned in the same
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
			Money:       row.Money,
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
				HasSockets:  cell.HasSockets,
				Socket0:     cell.Socket0,
				Socket1:     cell.Socket1,
				Socket2:     cell.Socket2,
			})
		}
	}
	return export, nil
}

// ExportCharacterSafeboxState validates and projects the committed file-store
// snapshot onto the 0015 character safebox-money migration tip. Missing
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
// 0015 migration tip. Missing snapshots yield an empty export.
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
