package safeboxstore

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrInvalidCharacterSafeboxStateExport reports that a retained safebox-state
// export failed the 0015 migration-shaped quarantine contract.
var ErrInvalidCharacterSafeboxStateExport = errors.New("invalid character safebox-state export")

// CharacterSafeboxStateQuarantineSummary is the metadata-only result of
// validating or quarantining a retained character safebox-state export. It
// never includes SQL, DSNs, or safebox snapshot bytes.
type CharacterSafeboxStateQuarantineSummary struct {
	CharacterCount int      `json:"character_count"`
	PasswordCount  int      `json:"password_count"`
	ItemCount      int      `json:"item_count"`
	CharacterIDs   []uint32 `json:"character_ids"`
	Logins         []string `json:"logins"`
}

// CharacterSafeboxStateQuarantineResult pairs the metadata-only quarantine
// summary with a canonicalized export ready for later offline review or
// backfill tools.
type CharacterSafeboxStateQuarantineResult struct {
	Summary CharacterSafeboxStateQuarantineSummary `json:"summary"`
	Export  CharacterSafeboxStateExport            `json:"export"`
}

// ValidateCharacterSafeboxStateExport fails closed when a retained export does
// not match the 0015_character_safebox_money tip. It does not open a database,
// write safebox snapshots, or mutate the supplied export.
func ValidateCharacterSafeboxStateExport(export CharacterSafeboxStateExport) (CharacterSafeboxStateQuarantineSummary, error) {
	canonical, summary, err := canonicalizeCharacterSafeboxStateExport(export)
	if err != nil {
		return CharacterSafeboxStateQuarantineSummary{}, err
	}
	_ = canonical
	return summary, nil
}

// QuarantineCharacterSafeboxStateExport validates a retained export and returns
// a canonicalized copy ordered by ascending character_id / login for passwords
// and character_id / cell for items. It never opens a database or mutates
// safebox snapshots.
func QuarantineCharacterSafeboxStateExport(export CharacterSafeboxStateExport) (CharacterSafeboxStateExport, CharacterSafeboxStateQuarantineSummary, error) {
	return canonicalizeCharacterSafeboxStateExport(export)
}

func canonicalizeCharacterSafeboxStateExport(export CharacterSafeboxStateExport) (CharacterSafeboxStateExport, CharacterSafeboxStateQuarantineSummary, error) {
	if export.MigrationVersion != CharacterSafeboxStateMigrationVersion {
		return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, fmt.Errorf("%w: migration_version %d", ErrInvalidCharacterSafeboxStateExport, export.MigrationVersion)
	}
	if export.MigrationName != CharacterSafeboxStateMigrationName {
		return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, fmt.Errorf("%w: migration_name %q", ErrInvalidCharacterSafeboxStateExport, export.MigrationName)
	}
	if export.Passwords == nil {
		return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, fmt.Errorf("%w: passwords must be present", ErrInvalidCharacterSafeboxStateExport)
	}
	if export.Items == nil {
		return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, fmt.Errorf("%w: items must be present", ErrInvalidCharacterSafeboxStateExport)
	}

	seenCharacterIDs := make(map[uint32]string, len(export.Passwords))
	seenLoginsByID := make(map[uint32]string, len(export.Passwords))
	seenIDsByLogin := make(map[string]uint32, len(export.Passwords))
	passwords := make([]CharacterSafeboxPasswordRow, 0, len(export.Passwords))
	for _, row := range export.Passwords {
		login := strings.TrimSpace(row.Login)
		password := strings.TrimSpace(row.Password)
		if row.CharacterID == 0 {
			return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, fmt.Errorf("%w: character_id must be > 0", ErrInvalidCharacterSafeboxStateExport)
		}
		if !validLogin(login) {
			return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, fmt.Errorf("%w: invalid login %q", ErrInvalidCharacterSafeboxStateExport, row.Login)
		}
		if !validPassword(password) {
			return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, fmt.Errorf("%w: invalid password for character_id=%d", ErrInvalidCharacterSafeboxStateExport, row.CharacterID)
		}
		if !validMoney(row.Money) {
			return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, fmt.Errorf("%w: invalid money for character_id=%d", ErrInvalidCharacterSafeboxStateExport, row.CharacterID)
		}
		if previousLogin, ok := seenLoginsByID[row.CharacterID]; ok && previousLogin != login {
			return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, fmt.Errorf("%w: character_id %d maps to both %q and %q", ErrInvalidCharacterSafeboxStateExport, row.CharacterID, previousLogin, login)
		}
		normalizedLogin := strings.ToLower(login)
		if previousID, ok := seenIDsByLogin[normalizedLogin]; ok && previousID != row.CharacterID {
			return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, fmt.Errorf("%w: login %q maps to both ids %d and %d", ErrInvalidCharacterSafeboxStateExport, login, previousID, row.CharacterID)
		}
		if _, exists := seenCharacterIDs[row.CharacterID]; exists {
			return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, fmt.Errorf("%w: duplicate character_id=%d password row", ErrInvalidCharacterSafeboxStateExport, row.CharacterID)
		}
		seenCharacterIDs[row.CharacterID] = login
		seenLoginsByID[row.CharacterID] = login
		seenIDsByLogin[normalizedLogin] = row.CharacterID
		passwords = append(passwords, CharacterSafeboxPasswordRow{
			CharacterID: row.CharacterID,
			Login:       login,
			Password:    password,
			Money:       row.Money,
		})
	}

	seenItemIDs := make(map[uint64]struct{}, len(export.Items))
	seenCells := make(map[string]struct{}, len(export.Items))
	items := make([]CharacterSafeboxItemRow, 0, len(export.Items))
	for _, row := range export.Items {
		login := strings.TrimSpace(row.Login)
		if row.CharacterID == 0 {
			return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, fmt.Errorf("%w: character_id must be > 0", ErrInvalidCharacterSafeboxStateExport)
		}
		if !validLogin(login) {
			return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, fmt.Errorf("%w: invalid login %q", ErrInvalidCharacterSafeboxStateExport, row.Login)
		}
		if row.ID == 0 || row.Vnum == 0 || row.Count == 0 || row.Cell >= MaxDurableCellExclusive {
			return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, fmt.Errorf("%w: invalid item row character_id=%d cell=%d", ErrInvalidCharacterSafeboxStateExport, row.CharacterID, row.Cell)
		}
		if previousLogin, ok := seenLoginsByID[row.CharacterID]; ok && previousLogin != login {
			return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, fmt.Errorf("%w: character_id %d maps to both %q and %q", ErrInvalidCharacterSafeboxStateExport, row.CharacterID, previousLogin, login)
		}
		normalizedLogin := strings.ToLower(login)
		if previousID, ok := seenIDsByLogin[normalizedLogin]; ok && previousID != row.CharacterID {
			return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, fmt.Errorf("%w: login %q maps to both ids %d and %d", ErrInvalidCharacterSafeboxStateExport, login, previousID, row.CharacterID)
		}
		if _, ok := seenCharacterIDs[row.CharacterID]; !ok {
			return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, fmt.Errorf("%w: item for unknown character_id=%d", ErrInvalidCharacterSafeboxStateExport, row.CharacterID)
		}
		if _, exists := seenItemIDs[row.ID]; exists {
			return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, fmt.Errorf("%w: duplicate item id=%d", ErrInvalidCharacterSafeboxStateExport, row.ID)
		}
		cellKey := fmt.Sprintf("%d\x00%d", row.CharacterID, row.Cell)
		if _, exists := seenCells[cellKey]; exists {
			return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, fmt.Errorf("%w: duplicate character_id=%d cell=%d", ErrInvalidCharacterSafeboxStateExport, row.CharacterID, row.Cell)
		}
		seenItemIDs[row.ID] = struct{}{}
		seenCells[cellKey] = struct{}{}
		seenLoginsByID[row.CharacterID] = login
		seenIDsByLogin[normalizedLogin] = row.CharacterID
		items = append(items, CharacterSafeboxItemRow{
			ID:          row.ID,
			CharacterID: row.CharacterID,
			Login:       login,
			Cell:        row.Cell,
			Vnum:        row.Vnum,
			Count:       row.Count,
			Locked:      row.Locked,
		})
	}

	sort.Slice(passwords, func(i, j int) bool {
		if passwords[i].CharacterID == passwords[j].CharacterID {
			return passwords[i].Login < passwords[j].Login
		}
		return passwords[i].CharacterID < passwords[j].CharacterID
	})
	sort.Slice(items, func(i, j int) bool {
		if items[i].CharacterID == items[j].CharacterID {
			return items[i].Cell < items[j].Cell
		}
		return items[i].CharacterID < items[j].CharacterID
	})

	characterIDs := make([]uint32, 0, len(seenCharacterIDs))
	logins := make([]string, 0, len(seenCharacterIDs))
	for characterID, login := range seenCharacterIDs {
		characterIDs = append(characterIDs, characterID)
		logins = append(logins, login)
	}
	sort.Slice(characterIDs, func(i, j int) bool { return characterIDs[i] < characterIDs[j] })
	sort.Strings(logins)

	canonical := CharacterSafeboxStateExport{
		MigrationVersion: CharacterSafeboxStateMigrationVersion,
		MigrationName:    CharacterSafeboxStateMigrationName,
		Passwords:        passwords,
		Items:            items,
	}
	summary := CharacterSafeboxStateQuarantineSummary{
		CharacterCount: len(characterIDs),
		PasswordCount:  len(passwords),
		ItemCount:      len(items),
		CharacterIDs:   characterIDs,
		Logins:         logins,
	}
	if summary.CharacterIDs == nil {
		summary.CharacterIDs = []uint32{}
	}
	if summary.Logins == nil {
		summary.Logins = []string{}
	}
	return canonical, summary, nil
}
