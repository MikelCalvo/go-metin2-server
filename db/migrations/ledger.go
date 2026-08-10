package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
)

const SchemaMigrationsLedgerQuery = `SELECT version, name, up_sha256
FROM schema_migrations
ORDER BY version ASC`

var ErrMigrationLedgerReaderRequired = errors.New("migration ledger reader is required")

// SQLLedgerQuerier is the narrow database/sql-compatible read boundary required
// to inspect the schema_migrations ledger. *sql.DB, *sql.Conn, and *sql.Tx
// satisfy this shape without forcing the migration planner to own driver
// configuration, connection lifetimes, or migration execution policy.
type SQLLedgerQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type ledgerRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

// ReadSQLLedgerEntries loads applied migration rows from schema_migrations using
// a metadata-only query. It does not execute migration SQL or mutate the
// database.
func ReadSQLLedgerEntries(ctx context.Context, querier SQLLedgerQuerier) ([]LedgerEntry, error) {
	if querier == nil {
		return nil, ErrMigrationLedgerReaderRequired
	}

	rows, err := querier.QueryContext(ctx, SchemaMigrationsLedgerQuery)
	if err != nil {
		return nil, fmt.Errorf("query schema_migrations ledger: %w", err)
	}
	return readLedgerEntriesFromRows(rows)
}

func readLedgerEntriesFromRows(rows ledgerRows) (entries []LedgerEntry, err error) {
	if ledgerRowsIsNil(rows) {
		return nil, fmt.Errorf("%w: schema_migrations query returned nil rows", ErrMigrationLedgerReaderRequired)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close schema_migrations ledger rows: %w", closeErr)
		}
	}()

	for rows.Next() {
		var entry LedgerEntry
		if err := rows.Scan(&entry.Version, &entry.Name, &entry.UpSHA256); err != nil {
			return nil, fmt.Errorf("scan schema_migrations ledger row: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema_migrations ledger rows: %w", err)
	}
	return entries, nil
}

func ledgerRowsIsNil(rows ledgerRows) bool {
	if rows == nil {
		return true
	}
	value := reflect.ValueOf(rows)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// PlanUpToLatestFromSQLLedger validates the built-in catalog against the
// database-read schema_migrations ledger and returns a dry-run plan. It reads
// only ledger metadata and applies nothing.
func PlanUpToLatestFromSQLLedger(ctx context.Context, querier SQLLedgerQuerier) (Plan, error) {
	catalog, err := Catalog()
	if err != nil {
		return Plan{}, err
	}
	return PlanCatalogUpToLatestFromSQLLedger(ctx, catalog, querier)
}

// PlanCatalogUpToLatestFromSQLLedger validates a supplied catalog against a
// queried schema_migrations ledger and returns pending up-migration metadata.
func PlanCatalogUpToLatestFromSQLLedger(ctx context.Context, catalog []Migration, querier SQLLedgerQuerier) (Plan, error) {
	return PlanCatalogToVersionFromSQLLedger(ctx, catalog, querier, len(catalog))
}

// PlanToVersionFromSQLLedger validates the built-in catalog against the
// database-read schema_migrations ledger and returns a metadata-only plan toward
// targetVersion. It reads ledger metadata only and applies nothing.
func PlanToVersionFromSQLLedger(ctx context.Context, querier SQLLedgerQuerier, targetVersion int) (Plan, error) {
	catalog, err := Catalog()
	if err != nil {
		return Plan{}, err
	}
	return PlanCatalogToVersionFromSQLLedger(ctx, catalog, querier, targetVersion)
}

// PlanCatalogToVersionFromSQLLedger validates a supplied catalog against a
// queried schema_migrations ledger and returns pending up/down metadata for the
// requested target version.
func PlanCatalogToVersionFromSQLLedger(ctx context.Context, catalog []Migration, querier SQLLedgerQuerier, targetVersion int) (Plan, error) {
	ledger, err := ReadSQLLedgerEntries(ctx, querier)
	if err != nil {
		return Plan{}, err
	}
	return PlanCatalogToVersion(catalog, ledger, targetVersion)
}
