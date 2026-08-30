package safeboxstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

// ErrCharacterSafeboxStateImportExecutorRequired reports that the SQL import
// primitive was called without a transaction-capable executor.
var ErrCharacterSafeboxStateImportExecutorRequired = errors.New("character safebox-state import executor is required")

// ErrCharacterSafeboxStateImportSchemaRequired reports that the target database
// has not applied the 0015_character_safebox_money tip plus additive
// 0025_character_safebox_item_instance_sockets boundary yet.
var ErrCharacterSafeboxStateImportSchemaRequired = errors.New("character safebox-state schema is not applied")

// ErrCharacterSafeboxStateImportRowCount reports that an INSERT affected an
// unexpected number of rows during safebox-state backfill.
var ErrCharacterSafeboxStateImportRowCount = errors.New("character safebox-state import row count mismatch")

// CharacterSafeboxStateImportResult is the metadata-only outcome of importing a
// quarantined 0015 safebox-state export. It never includes password/item
// payloads, SQL text, DSNs, or safebox snapshot bytes.
type CharacterSafeboxStateImportResult struct {
	MigrationVersion int      `json:"migration_version"`
	MigrationName    string   `json:"migration_name"`
	CharacterCount   int      `json:"character_count"`
	PasswordCount    int      `json:"password_count"`
	ItemCount        int      `json:"item_count"`
	CharacterIDs     []uint32 `json:"character_ids"`
}

// ImportCharacterSafeboxState validates a retained 0015 safebox-state export
// through the existing quarantine contract and inserts the canonicalized rows
// into character_safebox_passwords / character_safebox_items inside one
// transaction.
//
// The caller still owns driver selection and DSN loading. Parent character rows
// from 0002 must already exist (or the engine FK check fails closed). This
// primitive does not mutate bootstrap file stores, does not rewrite
// schema_migrations, and does not invent upsert / merge policy: duplicate
// primary keys fail closed and roll the transaction back.
func ImportCharacterSafeboxState(ctx context.Context, executor dbmigrations.SQLMigrationExecutor, export CharacterSafeboxStateExport) (CharacterSafeboxStateImportResult, error) {
	if safeboxStateImportExecutorIsNil(executor) {
		return CharacterSafeboxStateImportResult{}, ErrCharacterSafeboxStateImportExecutorRequired
	}

	canonical, summary, err := QuarantineCharacterSafeboxStateExport(export)
	if err != nil {
		return CharacterSafeboxStateImportResult{}, err
	}

	result := CharacterSafeboxStateImportResult{
		MigrationVersion: CharacterSafeboxStateMigrationVersion,
		MigrationName:    CharacterSafeboxStateMigrationName,
		CharacterCount:   summary.CharacterCount,
		PasswordCount:    summary.PasswordCount,
		ItemCount:        summary.ItemCount,
		CharacterIDs:     append([]uint32(nil), summary.CharacterIDs...),
	}
	if result.CharacterIDs == nil {
		result.CharacterIDs = []uint32{}
	}

	tx, err := executor.BeginTx(ctx, nil)
	if err != nil {
		return CharacterSafeboxStateImportResult{}, fmt.Errorf("begin character safebox-state import transaction: %w", err)
	}

	if err := requireCharacterSafeboxStateSchema(ctx, tx); err != nil {
		return CharacterSafeboxStateImportResult{}, rollbackAfterSafeboxStateImportFailure(tx, err)
	}

	for _, row := range canonical.Passwords {
		if err := insertCharacterSafeboxPassword(ctx, tx, row); err != nil {
			return CharacterSafeboxStateImportResult{}, rollbackAfterSafeboxStateImportFailure(tx, err)
		}
	}
	for _, row := range canonical.Items {
		if err := insertCharacterSafeboxItem(ctx, tx, row); err != nil {
			return CharacterSafeboxStateImportResult{}, rollbackAfterSafeboxStateImportFailure(tx, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return CharacterSafeboxStateImportResult{}, fmt.Errorf("commit character safebox-state import transaction: %w", err)
	}
	return result, nil
}

func requireCharacterSafeboxStateSchema(ctx context.Context, querier dbmigrations.SQLLedgerQuerier) error {
	ledger, err := dbmigrations.ReadSQLLedgerEntries(ctx, querier)
	if err != nil {
		return fmt.Errorf("%w: read schema_migrations: %v", ErrCharacterSafeboxStateImportSchemaRequired, err)
	}
	hasSafeboxMoney := false
	hasInstanceSockets := false
	latest := 0
	for _, entry := range ledger {
		if entry.Version > latest {
			latest = entry.Version
		}
		if entry.Version == CharacterSafeboxStateMigrationVersion && entry.Name == CharacterSafeboxStateMigrationName {
			hasSafeboxMoney = true
		}
		if entry.Version == CharacterSafeboxItemInstanceSocketsMigrationVersion && entry.Name == CharacterSafeboxItemInstanceSocketsMigrationName {
			hasInstanceSockets = true
		}
	}
	if hasSafeboxMoney && hasInstanceSockets {
		return nil
	}
	if !hasSafeboxMoney {
		return fmt.Errorf("%w: ledger tip %d missing version %d %q", ErrCharacterSafeboxStateImportSchemaRequired, latest, CharacterSafeboxStateMigrationVersion, CharacterSafeboxStateMigrationName)
	}
	return fmt.Errorf("%w: ledger tip %d missing version %d %q", ErrCharacterSafeboxStateImportSchemaRequired, latest, CharacterSafeboxItemInstanceSocketsMigrationVersion, CharacterSafeboxItemInstanceSocketsMigrationName)
}

func insertCharacterSafeboxPassword(ctx context.Context, tx *sql.Tx, row CharacterSafeboxPasswordRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO character_safebox_passwords (
    character_id, login, password, money
) VALUES (?, ?, ?, ?)`,
		int64(row.CharacterID), row.Login, row.Password, row.Money,
	)
	if err != nil {
		return fmt.Errorf("insert safebox password character %d: %w", row.CharacterID, err)
	}
	return requireExactSafeboxStateImportRows(result, "insert safebox password", int64(row.CharacterID))
}

func insertCharacterSafeboxItem(ctx context.Context, tx *sql.Tx, row CharacterSafeboxItemRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO character_safebox_items (
    id, character_id, login, cell, vnum, count, locked, has_sockets, socket0, socket1, socket2
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		int64(row.ID), int64(row.CharacterID), row.Login, int(row.Cell), int64(row.Vnum), int(row.Count), boolToSQLInt(row.Locked),
		boolToSQLInt(row.HasSockets), int64(row.Socket0), int64(row.Socket1), int64(row.Socket2),
	)
	if err != nil {
		return fmt.Errorf("insert safebox item id %d character %d cell %d: %w", row.ID, row.CharacterID, row.Cell, err)
	}
	return requireExactSafeboxStateImportRows(result, "insert safebox item", int64(row.ID))
}

func boolToSQLInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func requireExactSafeboxStateImportRows(result sql.Result, action string, id int64) error {
	if result == nil {
		return fmt.Errorf("%w: %s id %d returned nil result", ErrCharacterSafeboxStateImportRowCount, action, id)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: %s id %d returned unknown row count: %v", ErrCharacterSafeboxStateImportRowCount, action, id, err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("%w: %s id %d affected %d rows", ErrCharacterSafeboxStateImportRowCount, action, id, rowsAffected)
	}
	return nil
}

func rollbackAfterSafeboxStateImportFailure(tx *sql.Tx, importErr error) error {
	if tx == nil {
		return importErr
	}
	if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
		return errors.Join(importErr, fmt.Errorf("rollback character safebox-state import transaction: %w", rollbackErr))
	}
	return importErr
}

func safeboxStateImportExecutorIsNil(executor dbmigrations.SQLMigrationExecutor) bool {
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
