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
		if err := validateQuarantineSafeboxInstanceSockets(row.ID, row.HasSockets, row.Socket0, row.Socket1, row.Socket2); err != nil {
			return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, err
		}
		if err := validateQuarantineSafeboxInstanceAttributes(
			row.ID,
			row.HasAttributes,
			row.Attr0Type, row.Attr0Value,
			row.Attr1Type, row.Attr1Value,
			row.Attr2Type, row.Attr2Value,
			row.Attr3Type, row.Attr3Value,
			row.Attr4Type, row.Attr4Value,
			row.Attr5Type, row.Attr5Value,
			row.Attr6Type, row.Attr6Value,
		); err != nil {
			return CharacterSafeboxStateExport{}, CharacterSafeboxStateQuarantineSummary{}, err
		}
		seenItemIDs[row.ID] = struct{}{}
		seenCells[cellKey] = struct{}{}
		seenLoginsByID[row.CharacterID] = login
		seenIDsByLogin[normalizedLogin] = row.CharacterID
		items = append(items, CharacterSafeboxItemRow{
			ID:            row.ID,
			CharacterID:   row.CharacterID,
			Login:         login,
			Cell:          row.Cell,
			Vnum:          row.Vnum,
			Count:         row.Count,
			Locked:        row.Locked,
			HasSockets:    row.HasSockets,
			Socket0:       row.Socket0,
			Socket1:       row.Socket1,
			Socket2:       row.Socket2,
			HasAttributes: row.HasAttributes,
			Attr0Type:     row.Attr0Type,
			Attr0Value:    row.Attr0Value,
			Attr1Type:     row.Attr1Type,
			Attr1Value:    row.Attr1Value,
			Attr2Type:     row.Attr2Type,
			Attr2Value:    row.Attr2Value,
			Attr3Type:     row.Attr3Type,
			Attr3Value:    row.Attr3Value,
			Attr4Type:     row.Attr4Type,
			Attr4Value:    row.Attr4Value,
			Attr5Type:     row.Attr5Type,
			Attr5Value:    row.Attr5Value,
			Attr6Type:     row.Attr6Type,
			Attr6Value:    row.Attr6Value,
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

func validateQuarantineSafeboxInstanceSockets(itemID uint64, hasSockets bool, socket0, socket1, socket2 int32) error {
	if hasSockets {
		return nil
	}
	if socket0 != 0 || socket1 != 0 || socket2 != 0 {
		return fmt.Errorf("%w: safebox item %d has non-zero sockets without has_sockets", ErrInvalidCharacterSafeboxStateExport, itemID)
	}
	return nil
}

func validateQuarantineSafeboxInstanceAttributes(
	itemID uint64,
	hasAttributes bool,
	attr0Type uint8, attr0Value int16,
	attr1Type uint8, attr1Value int16,
	attr2Type uint8, attr2Value int16,
	attr3Type uint8, attr3Value int16,
	attr4Type uint8, attr4Value int16,
	attr5Type uint8, attr5Value int16,
	attr6Type uint8, attr6Value int16,
) error {
	if hasAttributes {
		return nil
	}
	if attr0Type != 0 || attr0Value != 0 ||
		attr1Type != 0 || attr1Value != 0 ||
		attr2Type != 0 || attr2Value != 0 ||
		attr3Type != 0 || attr3Value != 0 ||
		attr4Type != 0 || attr4Value != 0 ||
		attr5Type != 0 || attr5Value != 0 ||
		attr6Type != 0 || attr6Value != 0 {
		return fmt.Errorf("%w: safebox item %d has non-zero attributes without has_attributes", ErrInvalidCharacterSafeboxStateExport, itemID)
	}
	return nil
}
