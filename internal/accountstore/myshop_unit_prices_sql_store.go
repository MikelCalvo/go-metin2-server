package accountstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

// SQLMyShopUnitPricesStore is an opt-in repository for tip-0023 remembered
// private-shop unit prices. It reads roster shells from tip-0002 and replaces
// only the price rows of the characters supplied to Save/SaveAccounts. It does
// not write roster, item, point or other account state. The caller supplies the
// executor, driver, schema and external database backup. FileStore remains the
// stock rematerialize path; this store is not mounted by the daemons.
type SQLMyShopUnitPricesStore struct {
	executor dbmigrations.SQLMigrationExecutor
}

func NewSQLMyShopUnitPricesStore(executor dbmigrations.SQLMigrationExecutor) *SQLMyShopUnitPricesStore {
	return &SQLMyShopUnitPricesStore{executor: executor}
}

var (
	_ Store         = (*SQLMyShopUnitPricesStore)(nil)
	_ AccountLister = (*SQLMyShopUnitPricesStore)(nil)
)

func (s *SQLMyShopUnitPricesStore) Load(login string) (Account, error) {
	if strings.TrimSpace(login) == "" {
		return Account{}, ErrLoginRequired
	}
	if login != strings.TrimSpace(login) || containsNUL(login) {
		return Account{}, ErrInvalidAccount
	}
	accounts, err := s.List()
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

// List returns roster shells (including accounts with no characters) with the
// complete remembered price lists. It rejects malformed/orphan SQL rows rather
// than returning a partial snapshot.
func (s *SQLMyShopUnitPricesStore) List() ([]Account, error) {
	if s == nil || myShopUnitPricesImportExecutorIsNil(s.executor) {
		return nil, ErrCharacterMyShopUnitPricesImportExecutorRequired
	}
	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin myshop unit-prices SQL read: %w", err)
	}
	accounts, err := readSQLMyShopUnitPrices(ctx, tx)
	if err != nil {
		return nil, rollbackAfterMyShopUnitPricesImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit myshop unit-prices SQL read: %w", err)
	}
	return accounts, nil
}

func readSQLMyShopUnitPrices(ctx context.Context, tx *sql.Tx) ([]Account, error) {
	if err := requireCharacterMyShopUnitPricesSchema(ctx, tx); err != nil {
		return nil, err
	}
	roster, err := readSQLRoster(ctx, tx)
	if err != nil {
		return nil, err
	}
	accounts := make([]Account, len(roster.Accounts))
	byAccount := make(map[int64]int, len(accounts))
	byCharacter := make(map[uint32]*loginticket.Character, len(roster.Characters))
	for i, row := range roster.Accounts {
		accounts[i] = Account{Login: row.Login, Empire: row.Empire, Characters: make([]loginticket.Character, accountCharacterRosterPlayerSlots)}
		byAccount[row.ID] = i
	}
	for _, row := range roster.Characters {
		idx := byAccount[row.AccountID]
		character := rosterExportShell(row.ID, row.Name)
		character.Empire = row.Empire
		accounts[idx].Characters[row.Slot] = character
		byCharacter[row.ID] = &accounts[idx].Characters[row.Slot]
	}
	rows, err := tx.QueryContext(ctx, `SELECT character_id, vnum, unit_price FROM character_myshop_unit_prices ORDER BY character_id, vnum`)
	if err != nil {
		return nil, fmt.Errorf("query myshop unit prices: %w", err)
	}
	for rows.Next() {
		var id, vnum, price int64
		if err := rows.Scan(&id, &vnum, &price); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan myshop unit price: %w", err)
		}
		characterID, err := sqlPointStateUint32(id, "character_id")
		if err != nil {
			rows.Close()
			return nil, err
		}
		itemVnum, err := sqlPointStateUint32(vnum, "vnum")
		if err != nil {
			rows.Close()
			return nil, err
		}
		unitPrice, err := sqlPointStateUint32(price, "unit_price")
		if err != nil {
			rows.Close()
			return nil, err
		}
		character := byCharacter[characterID]
		if character == nil {
			rows.Close()
			return nil, fmt.Errorf("%w: myshop price character %d has no roster parent", ErrInvalidAccount, characterID)
		}
		character.MyShopUnitPrices = append(character.MyShopUnitPrices, loginticket.MyShopUnitPrice{Vnum: itemVnum, UnitPrice: unitPrice})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate myshop unit prices: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close myshop unit prices: %w", err)
	}
	for i := range accounts {
		accounts[i].Characters = normalizeAccountCharacters(accounts[i].Characters)
	}
	if _, err := ExportCharacterMyShopUnitPrices(accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}

func (s *SQLMyShopUnitPricesStore) ExportCharacterMyShopUnitPrices() (CharacterMyShopUnitPricesExport, error) {
	accounts, err := s.List()
	if err != nil {
		return CharacterMyShopUnitPricesExport{}, err
	}
	return ExportCharacterMyShopUnitPrices(accounts)
}

func (s *SQLMyShopUnitPricesStore) Save(account Account) error {
	return s.SaveAccounts([]Account{account})
}

// SaveAccounts transactionally replaces the entire price list for each supplied
// non-empty character, including wipe-to-empty. An empty account set is a
// schema-checked no-op. Omitted characters and all other tables are untouched.
// Each character must already belong to the supplied account in SQL; a foreign
// character ID cannot be used to write another account's prices.
func (s *SQLMyShopUnitPricesStore) SaveAccounts(accounts []Account) error {
	if s == nil || myShopUnitPricesImportExecutorIsNil(s.executor) {
		return ErrCharacterMyShopUnitPricesImportExecutorRequired
	}
	accounts = normalizeMyShopAccounts(accounts)
	if err := validateMyShopAccounts(accounts); err != nil {
		return err
	}
	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin myshop unit-prices SQL save: %w", err)
	}
	if err := requireCharacterMyShopUnitPricesSchema(ctx, tx); err != nil {
		return rollbackAfterMyShopUnitPricesImportFailure(tx, err)
	}
	for _, account := range accounts {
		for slot, character := range account.Characters {
			if character.IsEmptySlot() {
				continue
			}
			var loginNormalized, nameNormalized string
			var storedSlot int64
			err := tx.QueryRowContext(ctx, `SELECT accounts.login_normalized, characters.slot, characters.name_normalized FROM characters JOIN accounts ON accounts.id = characters.account_id WHERE characters.id = ?`, int64(character.ID)).Scan(&loginNormalized, &storedSlot, &nameNormalized)
			if err == sql.ErrNoRows || (err == nil && (loginNormalized != strings.ToLower(account.Login) || storedSlot != int64(slot) || nameNormalized != strings.ToLower(character.Name))) {
				return rollbackAfterMyShopUnitPricesImportFailure(tx, fmt.Errorf("%w: character %d does not belong to account %q", ErrInvalidAccount, character.ID, account.Login))
			}
			if err != nil {
				return rollbackAfterMyShopUnitPricesImportFailure(tx, fmt.Errorf("lookup myshop price owner: %w", err))
			}
			if err := deleteCharacterMyShopUnitPricesForCharacter(ctx, tx, character.ID); err != nil {
				return rollbackAfterMyShopUnitPricesImportFailure(tx, err)
			}
			for _, price := range character.MyShopUnitPrices {
				if err := insertCharacterMyShopUnitPrice(ctx, tx, CharacterMyShopUnitPriceRow{CharacterID: character.ID, Vnum: price.Vnum, UnitPrice: price.UnitPrice}); err != nil {
					return rollbackAfterMyShopUnitPricesImportFailure(tx, err)
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit myshop unit-prices SQL save: %w", err)
	}
	return nil
}

func normalizeMyShopAccounts(accounts []Account) []Account {
	if accounts == nil {
		return []Account{}
	}
	cloned := make([]Account, len(accounts))
	for i, account := range accounts {
		cloned[i] = Account{Login: account.Login, Empire: account.Empire, Characters: normalizeAccountCharacters(account.Characters)}
	}
	return cloned
}

func validateMyShopAccounts(accounts []Account) error {
	if _, err := ExportAccountCharacterRoster(accounts); err != nil {
		return err
	}
	_, err := ExportCharacterMyShopUnitPrices(accounts)
	return err
}
