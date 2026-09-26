package accountstore

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

// SQLPointStateStore is the first live DB-backed character point-state
// repository. It implements Store Load/Save (and AccountCharacterStateExporter's
// point-state method) against the already-owned tip-0011 character_points table
// through a caller-supplied database/sql executor. Export identity stays
// tip-0011.
//
// Load returns roster shells plus the fixed-width 0..254 point vector. Inventory,
// equipment, quickslots, gold, and other roster columns are not read from SQL.
// Save replaces character_points only for character ids present in the supplied
// snapshot: delete that character's rows, then insert the complete 255-row
// vector. Characters absent from the snapshot are left untouched. Roster and
// item-state rows are never inserted, updated, or deleted.
//
// The package does not select a driver, load a DSN, embed secrets, or register
// a production engine. Stock gamed rematerialize stays on FileStore. This is
// scoped replace of the supplied characters, not insert-only import and not an
// upsert.
type SQLPointStateStore struct {
	executor dbmigrations.SQLMigrationExecutor
}

// NewSQLPointStateStore returns an opt-in SQL point-state repository.
// A nil executor fails closed on Load/Save/Export rather than at construction.
func NewSQLPointStateStore(executor dbmigrations.SQLMigrationExecutor) *SQLPointStateStore {
	return &SQLPointStateStore{executor: executor}
}

// Load projects character_points onto Account snapshots whose roster shell comes
// from 0002. Empty point tables are an all-zero vector, not a missing FileStore.
// Schema preflight requires tip-0011.
func (s *SQLPointStateStore) Load(login string) (Account, error) {
	if strings.TrimSpace(login) == "" {
		return Account{}, ErrLoginRequired
	}
	if login != strings.TrimSpace(login) {
		return Account{}, fmt.Errorf("%w: account login %q has leading or trailing whitespace", ErrInvalidAccount, login)
	}
	if containsNUL(login) {
		return Account{}, fmt.Errorf("%w: account login contains NUL", ErrInvalidAccount)
	}
	accounts, err := s.loadAccounts()
	if err != nil {
		return Account{}, err
	}
	for _, account := range accounts {
		if strings.EqualFold(account.Login, login) {
			return account, nil
		}
	}
	return Account{}, ErrAccountNotFound
}

// Save replaces character_points for every non-empty character in account.
// Characters already stored but omitted from this account are left untouched.
func (s *SQLPointStateStore) Save(account Account) error {
	return s.SaveAccounts([]Account{account})
}

// SaveAccounts replaces character_points for every non-empty character in the
// supplied account set. The zero-length set is a no-op: it does not truncate
// the table and it does not touch roster rows.
func (s *SQLPointStateStore) SaveAccounts(accounts []Account) error {
	if s == nil || pointStateImportExecutorIsNil(s.executor) {
		return ErrCharacterPointStateImportExecutorRequired
	}
	normalized := normalizePointStateAccounts(accounts)
	if err := validatePointStateAccounts(normalized); err != nil {
		return fmt.Errorf("%w: validate point-state accounts", err)
	}
	if _, err := ExportCharacterPointState(normalized); err != nil {
		return err
	}

	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin character point-state SQL save transaction: %w", err)
	}
	if err := requireCharacterPointStateSchema(ctx, tx); err != nil {
		return rollbackAfterPointStateImportFailure(tx, err)
	}
	if err := replaceCharacterPointStateSnapshot(ctx, tx, normalized); err != nil {
		return rollbackAfterPointStateImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit character point-state SQL save transaction: %w", err)
	}
	return nil
}

// List projects every committed SQL account that owns at least one 0002
// character row. Accounts with no point rows are still returned so callers can
// tell an all-zero vector from a missing login.
func (s *SQLPointStateStore) List() ([]Account, error) {
	return s.loadAccounts()
}

// ExportCharacterPointState projects the committed SQL point rows onto the
// 0011 migration tip. Empty tables yield an empty export.
func (s *SQLPointStateStore) ExportCharacterPointState() (CharacterPointStateExport, error) {
	accounts, err := s.List()
	if err != nil {
		return CharacterPointStateExport{}, err
	}
	return ExportCharacterPointState(accounts)
}

func (s *SQLPointStateStore) loadAccounts() ([]Account, error) {
	if s == nil || pointStateImportExecutorIsNil(s.executor) {
		return nil, ErrCharacterPointStateImportExecutorRequired
	}
	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin character point-state SQL load transaction: %w", err)
	}
	accounts, err := loadCharacterPointStateAccounts(ctx, tx)
	if err != nil {
		return nil, rollbackAfterPointStateImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit character point-state SQL load transaction: %w", err)
	}
	return accounts, nil
}

func replaceCharacterPointStateSnapshot(ctx context.Context, tx *sql.Tx, accounts []Account) error {
	for _, account := range accounts {
		for _, character := range account.Characters {
			if character.IsEmptySlot() {
				continue
			}
			if err := deleteCharacterPointsForCharacter(ctx, tx, character.ID); err != nil {
				return err
			}
			for pointIndex, value := range character.Points {
				if err := insertCharacterPoint(ctx, tx, CharacterPointRow{
					CharacterID: character.ID,
					PointIndex:  uint8(pointIndex),
					Value:       value,
				}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func loadCharacterPointStateAccounts(ctx context.Context, tx *sql.Tx) ([]Account, error) {
	if err := requireCharacterPointStateSchema(ctx, tx); err != nil {
		return nil, err
	}

	rosterQuery := `
SELECT accounts.login, accounts.empire, characters.id, characters.slot, characters.name
FROM characters
JOIN accounts ON accounts.id = characters.account_id
ORDER BY accounts.login_normalized ASC, characters.slot ASC`
	rosterRows, err := tx.QueryContext(ctx, rosterQuery)
	if err != nil {
		return nil, fmt.Errorf("query character point-state roster: %w", err)
	}
	defer rosterRows.Close()

	type pointStateRosterCharacter struct {
		id      uint32
		slot    int
		account int
	}
	var roster []pointStateRosterCharacter
	accountIndex := map[string]int{}
	accounts := make([]Account, 0)
	characterSlot := map[uint32]int{}
	for rosterRows.Next() {
		var (
			login       string
			empire      int64
			characterID int64
			slot        int64
			name        string
		)
		if err := rosterRows.Scan(&login, &empire, &characterID, &slot, &name); err != nil {
			return nil, fmt.Errorf("scan character point-state roster: %w", err)
		}
		id, err := sqlPointStateUint32(characterID, "character_id")
		if err != nil {
			return nil, err
		}
		if slot < 0 || slot >= accountCharacterRosterPlayerSlots {
			return nil, fmt.Errorf("%w: character id %d slot %d outside point-state roster", ErrInvalidAccount, id, slot)
		}
		empireValue, err := sqlPointStateUint8(empire, "empire")
		if err != nil {
			return nil, err
		}
		if _, exists := characterSlot[id]; exists {
			return nil, fmt.Errorf("%w: duplicate character_id=%d roster row", ErrInvalidAccount, id)
		}
		key := strings.ToLower(login)
		idx, ok := accountIndex[key]
		if !ok {
			idx = len(accounts)
			accountIndex[key] = idx
			accounts = append(accounts, Account{
				Login:      login,
				Empire:     empireValue,
				Characters: make([]loginticket.Character, accountCharacterRosterPlayerSlots),
			})
		} else if accounts[idx].Empire != empireValue {
			return nil, fmt.Errorf("%w: account %q empire drifted between roster rows", ErrInvalidAccount, login)
		}
		character := rosterExportShell(id, name)
		character.Empire = empireValue
		character.NormalizeItemState()
		accounts[idx].Characters[slot] = character
		characterSlot[id] = len(roster)
		roster = append(roster, pointStateRosterCharacter{id: id, slot: int(slot), account: idx})
	}
	if err := rosterRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate character point-state roster: %w", err)
	}

	pointRows, err := tx.QueryContext(ctx, `
SELECT character_id, point_index, value
FROM character_points
ORDER BY character_id ASC, point_index ASC`)
	if err != nil {
		return nil, fmt.Errorf("query character points: %w", err)
	}
	defer pointRows.Close()

	seen := map[uint32]map[uint8]struct{}{}
	for pointRows.Next() {
		var characterID, pointIndex, value int64
		if err := pointRows.Scan(&characterID, &pointIndex, &value); err != nil {
			return nil, fmt.Errorf("scan character point: %w", err)
		}
		id, err := sqlPointStateUint32(characterID, "character_id")
		if err != nil {
			return nil, err
		}
		rosterIndex, ok := characterSlot[id]
		if !ok {
			return nil, fmt.Errorf("%w: point character_id=%d has no roster parent", ErrInvalidAccount, id)
		}
		if pointIndex < 0 || pointIndex >= characterPointStatePointCount {
			return nil, fmt.Errorf("%w: character %d point_index %d out of range", ErrInvalidAccount, id, pointIndex)
		}
		pointValue, err := sqlPointStateInt32(value, "value")
		if err != nil {
			return nil, err
		}
		index := uint8(pointIndex)
		if seen[id] == nil {
			seen[id] = map[uint8]struct{}{}
		}
		if _, exists := seen[id][index]; exists {
			return nil, fmt.Errorf("%w: duplicate character_id=%d point_index=%d", ErrInvalidAccount, id, index)
		}
		seen[id][index] = struct{}{}
		parent := roster[rosterIndex]
		accounts[parent.account].Characters[parent.slot].Points[index] = pointValue
	}
	if err := pointRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate character points: %w", err)
	}
	for id, indexes := range seen {
		if len(indexes) != characterPointStatePointCount {
			return nil, fmt.Errorf("%w: character %d has %d point rows; expected %d", ErrInvalidAccount, id, len(indexes), characterPointStatePointCount)
		}
	}

	normalized := normalizePointStateAccounts(accounts)
	for i := range normalized {
		for slot := range normalized[i].Characters {
			normalized[i].Characters[slot].NormalizeItemState()
		}
	}
	if err := validatePointStateAccounts(normalized); err != nil {
		return nil, fmt.Errorf("%w: validate point-state accounts", err)
	}
	if _, err := ExportCharacterPointState(normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

func normalizePointStateAccounts(accounts []Account) []Account {
	if accounts == nil {
		return []Account{}
	}
	cloned := make([]Account, len(accounts))
	for i := range accounts {
		cloned[i] = Account{
			Login:      accounts[i].Login,
			Empire:     accounts[i].Empire,
			Characters: normalizeAccountCharacters(accounts[i].Characters),
		}
	}
	return cloned
}

func validatePointStateAccounts(accounts []Account) error {
	seenLogins := map[string]string{}
	seenIDs := map[uint32]string{}
	for _, account := range accounts {
		if strings.TrimSpace(account.Login) == "" {
			return ErrLoginRequired
		}
		if account.Login != strings.TrimSpace(account.Login) {
			return fmt.Errorf("%w: account login %q has leading or trailing whitespace", ErrInvalidAccount, account.Login)
		}
		if containsNUL(account.Login) {
			return fmt.Errorf("%w: account login contains NUL", ErrInvalidAccount)
		}
		normalized := strings.ToLower(account.Login)
		if previous, ok := seenLogins[normalized]; ok {
			return fmt.Errorf("%w: account login %q duplicates %q", ErrInvalidAccount, account.Login, previous)
		}
		seenLogins[normalized] = account.Login
		if err := validateAccount(account); err != nil {
			return err
		}
		if len(account.Characters) > accountCharacterRosterPlayerSlots {
			return fmt.Errorf("%w: account %q has %d character slots; migration roster supports %d", ErrInvalidAccount, account.Login, len(account.Characters), accountCharacterRosterPlayerSlots)
		}
		for slot, character := range account.Characters {
			if character.IsEmptySlot() {
				continue
			}
			if err := validateRosterExportCharacter(account, slot, character, seenIDs, map[string]uint32{}); err != nil {
				return err
			}
			seenIDs[character.ID] = character.Name
		}
	}
	return nil
}

func sqlPointStateUint8(value int64, field string) (uint8, error) {
	if value < 0 || value > math.MaxUint8 {
		return 0, fmt.Errorf("%w: %s %d", ErrInvalidAccount, field, value)
	}
	return uint8(value), nil
}

func sqlPointStateUint32(value int64, field string) (uint32, error) {
	if value < 0 || value > math.MaxUint32 {
		return 0, fmt.Errorf("%w: %s %d", ErrInvalidAccount, field, value)
	}
	return uint32(value), nil
}

func sqlPointStateInt32(value int64, field string) (int32, error) {
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, fmt.Errorf("%w: %s %d", ErrInvalidAccount, field, value)
	}
	return int32(value), nil
}
