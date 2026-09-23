package accountstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

// ErrAccountCharacterRosterImportExecutorRequired reports that the SQL import
// primitive was called without a transaction-capable executor.
var ErrAccountCharacterRosterImportExecutorRequired = errors.New("account character roster import executor is required")

// ErrAccountCharacterRosterImportSchemaRequired reports that the target database
// has not applied the 0002_account_character_roster migration boundary yet.
var ErrAccountCharacterRosterImportSchemaRequired = errors.New("account character roster schema is not applied")

// ErrAccountCharacterRosterImportRowCount reports that an INSERT affected an
// unexpected number of rows during roster backfill.
var ErrAccountCharacterRosterImportRowCount = errors.New("account character roster import row count mismatch")

// AccountCharacterRosterImportResult is the metadata-only outcome of importing a
// quarantined 0002 roster export. It never includes passwords, SQL text, DSNs,
// or account snapshot bytes.
type AccountCharacterRosterImportResult struct {
	MigrationVersion int      `json:"migration_version"`
	MigrationName    string   `json:"migration_name"`
	AccountCount     int      `json:"account_count"`
	CharacterCount   int      `json:"character_count"`
	AccountIDs       []int64  `json:"account_ids"`
	CharacterIDs     []uint32 `json:"character_ids"`
	// Replaced is true when ImportAccountCharacterRoster ran with the opt-in
	// scoped replace policy (delete then insert for listed account ids).
	// Omitted from JSON when false so legacy insert-only import-result files stay
	// valid.
	Replaced bool `json:"replaced,omitempty"`
	// ItemStateCascadeDeleted is true when that replace also deleted tip-0003
	// inventory / equipment / quickslot rows for characters of the listed
	// accounts before deleting characters / accounts. Omitted from JSON when
	// false so replace results without the companion stay valid.
	ItemStateCascadeDeleted bool `json:"item_state_cascade_deleted,omitempty"`
}

// ImportAccountCharacterRosterOptions controls opt-in mutation policy for
// ImportAccountCharacterRoster. The zero value keeps today's insert-only
// behavior.
type ImportAccountCharacterRosterOptions struct {
	// Replace, when true, deletes existing tip-0002 accounts/characters rows for
	// every account id in the quarantined export summary before inserting the
	// canonicalized export rows, all inside one transaction. Accounts not listed
	// in the export remain untouched. Without CascadeDeleteItemState, child tip
	// domains are not cascade-deleted; FK dependents fail closed and roll the
	// transaction back.
	Replace bool
	// CascadeDeleteItemState, when true together with Replace, deletes tip-0003
	// character_inventory_items / character_equipment_items / character_quickslots
	// for every character currently owned by a listed account before the tip-0002
	// character and account deletes, still inside the same transaction. It is a
	// no-op when those tip-0003 tables are absent. Other child tip domains stay
	// fail-closed. Ignored unless Replace is also true.
	CascadeDeleteItemState bool
}

// ImportAccountCharacterRoster validates a retained 0002 roster export through
// the existing quarantine contract and inserts the canonicalized rows into
// accounts / characters inside one transaction.
//
// The caller still owns driver selection and DSN loading. This primitive does
// not mutate bootstrap file stores and does not rewrite schema_migrations.
// Without options (or with Replace=false) it does not invent upsert / merge
// policy: duplicate primary keys fail closed and roll the transaction back.
// Pass ImportAccountCharacterRosterOptions{Replace: true} for the opt-in scoped
// replace path frozen by the tip-0002 replace contract. Set CascadeDeleteItemState
// on that same options value to also delete tip-0003 item rows for characters of
// the listed accounts before the roster delete. The two-phase wipe→roster→reimport
// drill is unchanged and does not enable this companion.
func ImportAccountCharacterRoster(ctx context.Context, executor dbmigrations.SQLMigrationExecutor, export AccountCharacterRosterExport, opts ...ImportAccountCharacterRosterOptions) (AccountCharacterRosterImportResult, error) {
	if rosterImportExecutorIsNil(executor) {
		return AccountCharacterRosterImportResult{}, ErrAccountCharacterRosterImportExecutorRequired
	}
	if len(opts) > 1 {
		return AccountCharacterRosterImportResult{}, fmt.Errorf("ImportAccountCharacterRoster accepts at most one options value")
	}
	replace := false
	cascadeDeleteItemState := false
	if len(opts) == 1 {
		replace = opts[0].Replace
		cascadeDeleteItemState = replace && opts[0].CascadeDeleteItemState
	}

	canonical, summary, err := QuarantineAccountCharacterRosterExport(export)
	if err != nil {
		return AccountCharacterRosterImportResult{}, err
	}

	result := AccountCharacterRosterImportResult{
		MigrationVersion:        AccountCharacterRosterMigrationVersion,
		MigrationName:           AccountCharacterRosterMigrationName,
		AccountCount:            summary.AccountCount,
		CharacterCount:          summary.CharacterCount,
		AccountIDs:              append([]int64(nil), summary.AccountIDs...),
		CharacterIDs:            append([]uint32(nil), summary.CharacterIDs...),
		Replaced:                replace,
		ItemStateCascadeDeleted: cascadeDeleteItemState,
	}
	if result.AccountIDs == nil {
		result.AccountIDs = []int64{}
	}
	if result.CharacterIDs == nil {
		result.CharacterIDs = []uint32{}
	}

	tx, err := executor.BeginTx(ctx, nil)
	if err != nil {
		return AccountCharacterRosterImportResult{}, fmt.Errorf("begin account character roster import transaction: %w", err)
	}

	if err := requireAccountCharacterRosterSchema(ctx, tx); err != nil {
		return AccountCharacterRosterImportResult{}, rollbackAfterRosterImportFailure(tx, err)
	}

	if replace {
		for _, accountID := range summary.AccountIDs {
			if cascadeDeleteItemState {
				if err := deleteAccountCharacterItemStateForAccount(ctx, tx, accountID); err != nil {
					return AccountCharacterRosterImportResult{}, rollbackAfterRosterImportFailure(tx, err)
				}
			}
			if err := deleteAccountCharacterRosterForAccount(ctx, tx, accountID); err != nil {
				return AccountCharacterRosterImportResult{}, rollbackAfterRosterImportFailure(tx, err)
			}
		}
	}

	for _, account := range canonical.Accounts {
		if err := insertAccountCharacterRosterAccount(ctx, tx, account); err != nil {
			return AccountCharacterRosterImportResult{}, rollbackAfterRosterImportFailure(tx, err)
		}
	}
	for _, character := range canonical.Characters {
		if err := insertAccountCharacterRosterCharacter(ctx, tx, character); err != nil {
			return AccountCharacterRosterImportResult{}, rollbackAfterRosterImportFailure(tx, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return AccountCharacterRosterImportResult{}, fmt.Errorf("commit account character roster import transaction: %w", err)
	}
	return result, nil
}

func requireAccountCharacterRosterSchema(ctx context.Context, querier dbmigrations.SQLLedgerQuerier) error {
	ledger, err := dbmigrations.ReadSQLLedgerEntries(ctx, querier)
	if err != nil {
		return fmt.Errorf("%w: read schema_migrations: %v", ErrAccountCharacterRosterImportSchemaRequired, err)
	}
	for _, entry := range ledger {
		if entry.Version == AccountCharacterRosterMigrationVersion && entry.Name == AccountCharacterRosterMigrationName {
			return nil
		}
	}
	latest := 0
	for _, entry := range ledger {
		if entry.Version > latest {
			latest = entry.Version
		}
	}
	return fmt.Errorf("%w: ledger tip %d missing version %d %q", ErrAccountCharacterRosterImportSchemaRequired, latest, AccountCharacterRosterMigrationVersion, AccountCharacterRosterMigrationName)
}

func deleteAccountCharacterItemStateForAccount(ctx context.Context, tx *sql.Tx, accountID int64) error {
	// Tip-0003 only. Delete inventory, equipment, then quickslots for characters
	// currently owned by this account so the following tip-0002 character DELETE
	// is not blocked by those FKs. Missing tables are a no-op (schema may still
	// be tip-0002). Other child domains stay fail-closed.
	const inventory = `DELETE FROM character_inventory_items WHERE character_id IN (SELECT id FROM characters WHERE account_id = ?)`
	const equipment = `DELETE FROM character_equipment_items WHERE character_id IN (SELECT id FROM characters WHERE account_id = ?)`
	const quickslots = `DELETE FROM character_quickslots WHERE character_id IN (SELECT id FROM characters WHERE account_id = ?)`
	if err := execRosterItemStateDelete(ctx, tx, inventory, accountID, "inventory items"); err != nil {
		return err
	}
	if err := execRosterItemStateDelete(ctx, tx, equipment, accountID, "equipment items"); err != nil {
		return err
	}
	return execRosterItemStateDelete(ctx, tx, quickslots, accountID, "quickslots")
}

func execRosterItemStateDelete(ctx context.Context, tx *sql.Tx, query string, accountID int64, what string) error {
	if _, err := tx.ExecContext(ctx, query, accountID); err != nil {
		if rosterItemStateTableMissing(err) {
			return nil
		}
		return fmt.Errorf("delete %s for account %d: %w", what, accountID, err)
	}
	return nil
}

func rosterItemStateTableMissing(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	if !strings.Contains(message, "no such table") && !strings.Contains(message, "doesn't exist") && !strings.Contains(message, "does not exist") && !strings.Contains(message, "undefined_table") {
		return false
	}
	return strings.Contains(message, "character_inventory_items") || strings.Contains(message, "character_equipment_items") || strings.Contains(message, "character_quickslots")
}

func deleteAccountCharacterRosterForAccount(ctx context.Context, tx *sql.Tx, accountID int64) error {
	// Delete characters before accounts so the tip-0002 FK
	// (characters.account_id → accounts.id) cannot fail closed mid replace.
	// Unless CascadeDeleteItemState was requested, child tip domains are not
	// cascade-deleted; remaining FK dependents fail closed here and roll the
	// surrounding transaction back.
	if _, err := tx.ExecContext(ctx, `DELETE FROM characters WHERE account_id = ?`, accountID); err != nil {
		return fmt.Errorf("delete characters for account %d: %w", accountID, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM accounts WHERE id = ?`, accountID); err != nil {
		return fmt.Errorf("delete account %d: %w", accountID, err)
	}
	return nil
}

func insertAccountCharacterRosterAccount(ctx context.Context, tx *sql.Tx, row AccountCharacterRosterAccountRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO accounts (
    id, login, login_normalized, empire
) VALUES (?, ?, ?, ?)`,
		row.ID, row.Login, row.LoginNormalized, row.Empire,
	)
	if err != nil {
		return fmt.Errorf("insert account id %d: %w", row.ID, err)
	}
	return requireExactRosterImportRows(result, "insert account", row.ID)
}

func insertAccountCharacterRosterCharacter(ctx context.Context, tx *sql.Tx, row AccountCharacterRosterCharacterRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO characters (
    id, account_id, slot, name, name_normalized,
    job, race_num, level, play_minutes, st, ht, dx, iq,
    main_part, change_name, hair_part, x, y, z, map_index,
    empire, skill_group, guild_id, guild_name, gold
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.ID, row.AccountID, row.Slot, row.Name, row.NameNormalized,
		row.Job, row.RaceNum, row.Level, row.PlayMinutes, row.ST, row.HT, row.DX, row.IQ,
		row.MainPart, row.ChangeName, row.HairPart, row.X, row.Y, row.Z, row.MapIndex,
		row.Empire, row.SkillGroup, row.GuildID, row.GuildName, row.Gold,
	)
	if err != nil {
		return fmt.Errorf("insert character id %d: %w", row.ID, err)
	}
	return requireExactRosterImportRows(result, "insert character", int64(row.ID))
}

func requireExactRosterImportRows(result sql.Result, action string, id int64) error {
	if result == nil {
		return fmt.Errorf("%w: %s id %d returned nil result", ErrAccountCharacterRosterImportRowCount, action, id)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: %s id %d returned unknown row count: %v", ErrAccountCharacterRosterImportRowCount, action, id, err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("%w: %s id %d affected %d rows", ErrAccountCharacterRosterImportRowCount, action, id, rowsAffected)
	}
	return nil
}

func rollbackAfterRosterImportFailure(tx *sql.Tx, importErr error) error {
	if tx == nil {
		return importErr
	}
	if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
		return errors.Join(importErr, fmt.Errorf("rollback account character roster import transaction: %w", rollbackErr))
	}
	return importErr
}

func rosterImportExecutorIsNil(executor dbmigrations.SQLMigrationExecutor) bool {
	if executor == nil {
		return true
	}
	value := reflect.ValueOf(executor)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
