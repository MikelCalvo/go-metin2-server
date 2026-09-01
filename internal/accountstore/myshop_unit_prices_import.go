package accountstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

// ErrCharacterMyShopUnitPricesImportExecutorRequired reports that the SQL import
// primitive was called without a transaction-capable executor.
var ErrCharacterMyShopUnitPricesImportExecutorRequired = errors.New("character myshop unit-prices import executor is required")

// ErrCharacterMyShopUnitPricesImportSchemaRequired reports that the target
// database has not applied the 0023_character_myshop_unit_prices migration
// boundary yet.
var ErrCharacterMyShopUnitPricesImportSchemaRequired = errors.New("character myshop unit-prices schema is not applied")

// ErrCharacterMyShopUnitPricesImportRowCount reports that an INSERT affected an
// unexpected number of rows during myshop unit-prices backfill.
var ErrCharacterMyShopUnitPricesImportRowCount = errors.New("character myshop unit-prices import row count mismatch")

// CharacterMyShopUnitPricesImportResult is the metadata-only outcome of
// importing a quarantined 0023 myshop unit-prices export. It never includes
// price payloads, SQL text, DSNs, or account snapshot bytes.
type CharacterMyShopUnitPricesImportResult struct {
	MigrationVersion int      `json:"migration_version"`
	MigrationName    string   `json:"migration_name"`
	CharacterCount   int      `json:"character_count"`
	PriceRowCount    int      `json:"price_row_count"`
	CharacterIDs     []uint32 `json:"character_ids"`
	// Replaced is true when ImportCharacterMyShopUnitPrices ran with the opt-in
	// scoped replace policy (delete then insert for listed character ids).
	// Omitted from JSON when false so legacy insert-only import-result files stay
	// valid.
	Replaced bool `json:"replaced,omitempty"`
}

// ImportCharacterMyShopUnitPricesOptions controls opt-in mutation policy for
// ImportCharacterMyShopUnitPrices. The zero value keeps today's insert-only
// behavior.
type ImportCharacterMyShopUnitPricesOptions struct {
	// Replace, when true, deletes existing tip-0023 child rows for every
	// character id in the quarantined export summary before inserting the
	// canonicalized export rows, all inside one transaction. Characters not
	// listed in the export remain untouched.
	Replace bool
}

// ImportCharacterMyShopUnitPrices validates a retained 0023 myshop unit-prices
// export through the existing quarantine contract and inserts the canonicalized
// rows into character_myshop_unit_prices inside one transaction.
//
// The caller still owns driver selection and DSN loading. Parent character rows
// from 0002 must already exist (or the engine FK check fails closed). This
// primitive does not mutate bootstrap file stores and does not rewrite
// schema_migrations. Without options (or with Replace=false) it does not invent
// upsert / merge policy: duplicate primary keys fail closed and roll the
// transaction back. Pass ImportCharacterMyShopUnitPricesOptions{Replace: true}
// for the opt-in scoped replace path frozen by the tip-0023 replace contract.
func ImportCharacterMyShopUnitPrices(ctx context.Context, executor dbmigrations.SQLMigrationExecutor, export CharacterMyShopUnitPricesExport, opts ...ImportCharacterMyShopUnitPricesOptions) (CharacterMyShopUnitPricesImportResult, error) {
	if myShopUnitPricesImportExecutorIsNil(executor) {
		return CharacterMyShopUnitPricesImportResult{}, ErrCharacterMyShopUnitPricesImportExecutorRequired
	}
	if len(opts) > 1 {
		return CharacterMyShopUnitPricesImportResult{}, fmt.Errorf("ImportCharacterMyShopUnitPrices accepts at most one options value")
	}
	replace := false
	if len(opts) == 1 {
		replace = opts[0].Replace
	}

	canonical, summary, err := QuarantineCharacterMyShopUnitPricesExport(export)
	if err != nil {
		return CharacterMyShopUnitPricesImportResult{}, err
	}

	result := CharacterMyShopUnitPricesImportResult{
		MigrationVersion: CharacterMyShopUnitPricesMigrationVersion,
		MigrationName:    CharacterMyShopUnitPricesMigrationName,
		CharacterCount:   summary.CharacterCount,
		PriceRowCount:    summary.PriceRowCount,
		CharacterIDs:     append([]uint32(nil), summary.CharacterIDs...),
		Replaced:         replace,
	}
	if result.CharacterIDs == nil {
		result.CharacterIDs = []uint32{}
	}

	tx, err := executor.BeginTx(ctx, nil)
	if err != nil {
		return CharacterMyShopUnitPricesImportResult{}, fmt.Errorf("begin character myshop unit-prices import transaction: %w", err)
	}

	if err := requireCharacterMyShopUnitPricesSchema(ctx, tx); err != nil {
		return CharacterMyShopUnitPricesImportResult{}, rollbackAfterMyShopUnitPricesImportFailure(tx, err)
	}

	if replace {
		for _, characterID := range summary.CharacterIDs {
			if err := deleteCharacterMyShopUnitPricesForCharacter(ctx, tx, characterID); err != nil {
				return CharacterMyShopUnitPricesImportResult{}, rollbackAfterMyShopUnitPricesImportFailure(tx, err)
			}
		}
	}

	for _, row := range canonical.UnitPrices {
		if err := insertCharacterMyShopUnitPrice(ctx, tx, row); err != nil {
			return CharacterMyShopUnitPricesImportResult{}, rollbackAfterMyShopUnitPricesImportFailure(tx, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return CharacterMyShopUnitPricesImportResult{}, fmt.Errorf("commit character myshop unit-prices import transaction: %w", err)
	}
	return result, nil
}

func requireCharacterMyShopUnitPricesSchema(ctx context.Context, querier dbmigrations.SQLLedgerQuerier) error {
	ledger, err := dbmigrations.ReadSQLLedgerEntries(ctx, querier)
	if err != nil {
		return fmt.Errorf("%w: read schema_migrations: %v", ErrCharacterMyShopUnitPricesImportSchemaRequired, err)
	}
	for _, entry := range ledger {
		if entry.Version == CharacterMyShopUnitPricesMigrationVersion && entry.Name == CharacterMyShopUnitPricesMigrationName {
			return nil
		}
	}
	latest := 0
	for _, entry := range ledger {
		if entry.Version > latest {
			latest = entry.Version
		}
	}
	return fmt.Errorf("%w: ledger tip %d missing version %d %q", ErrCharacterMyShopUnitPricesImportSchemaRequired, latest, CharacterMyShopUnitPricesMigrationVersion, CharacterMyShopUnitPricesMigrationName)
}

func deleteCharacterMyShopUnitPricesForCharacter(ctx context.Context, tx *sql.Tx, characterID uint32) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM character_myshop_unit_prices WHERE character_id = ?`, int64(characterID)); err != nil {
		return fmt.Errorf("delete myshop unit prices for character %d: %w", characterID, err)
	}
	return nil
}

func insertCharacterMyShopUnitPrice(ctx context.Context, tx *sql.Tx, row CharacterMyShopUnitPriceRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO character_myshop_unit_prices (
    character_id, vnum, unit_price
) VALUES (?, ?, ?)`,
		int64(row.CharacterID), int64(row.Vnum), int64(row.UnitPrice),
	)
	if err != nil {
		return fmt.Errorf("insert myshop unit price character %d vnum %d: %w", row.CharacterID, row.Vnum, err)
	}
	return requireExactMyShopUnitPricesImportRows(result, "insert myshop unit price", int64(row.CharacterID)*100000+int64(row.Vnum))
}

func requireExactMyShopUnitPricesImportRows(result sql.Result, action string, id int64) error {
	if result == nil {
		return fmt.Errorf("%w: %s id %d returned nil result", ErrCharacterMyShopUnitPricesImportRowCount, action, id)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: %s id %d returned unknown row count: %v", ErrCharacterMyShopUnitPricesImportRowCount, action, id, err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("%w: %s id %d affected %d rows", ErrCharacterMyShopUnitPricesImportRowCount, action, id, rowsAffected)
	}
	return nil
}

func rollbackAfterMyShopUnitPricesImportFailure(tx *sql.Tx, importErr error) error {
	if tx == nil {
		return importErr
	}
	if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
		return errors.Join(importErr, fmt.Errorf("rollback character myshop unit-prices import transaction: %w", rollbackErr))
	}
	return importErr
}

func myShopUnitPricesImportExecutorIsNil(executor dbmigrations.SQLMigrationExecutor) bool {
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
