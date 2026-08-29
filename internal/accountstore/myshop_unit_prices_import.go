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
}

// ImportCharacterMyShopUnitPrices validates a retained 0023 myshop unit-prices
// export through the existing quarantine contract and inserts the canonicalized
// rows into character_myshop_unit_prices inside one transaction.
//
// The caller still owns driver selection and DSN loading. Parent character rows
// from 0002 must already exist (or the engine FK check fails closed). This
// primitive does not mutate bootstrap file stores, does not rewrite
// schema_migrations, and does not invent upsert / merge policy: duplicate
// primary keys fail closed and roll the transaction back.
func ImportCharacterMyShopUnitPrices(ctx context.Context, executor dbmigrations.SQLMigrationExecutor, export CharacterMyShopUnitPricesExport) (CharacterMyShopUnitPricesImportResult, error) {
	if myShopUnitPricesImportExecutorIsNil(executor) {
		return CharacterMyShopUnitPricesImportResult{}, ErrCharacterMyShopUnitPricesImportExecutorRequired
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
