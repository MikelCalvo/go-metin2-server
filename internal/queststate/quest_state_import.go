package queststate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

// ErrCharacterQuestStateImportExecutorRequired reports that the SQL import
// primitive was called without a transaction-capable executor.
var ErrCharacterQuestStateImportExecutorRequired = errors.New("character quest-state import executor is required")

// ErrCharacterQuestStateImportSchemaRequired reports that the target database
// has not applied the 0004_character_quest_state migration boundary yet.
var ErrCharacterQuestStateImportSchemaRequired = errors.New("character quest-state schema is not applied")

// ErrCharacterQuestStateImportRowCount reports that an INSERT affected an
// unexpected number of rows during quest-state backfill.
var ErrCharacterQuestStateImportRowCount = errors.New("character quest-state import row count mismatch")

// CharacterQuestStateImportResult is the metadata-only outcome of importing a
// quarantined 0004 quest-state export. It never includes flag payloads, SQL
// text, DSNs, or quest-state snapshot bytes.
type CharacterQuestStateImportResult struct {
	MigrationVersion int      `json:"migration_version"`
	MigrationName    string   `json:"migration_name"`
	CharacterCount   int      `json:"character_count"`
	FlagCount        int      `json:"flag_count"`
	CharacterIDs     []uint32 `json:"character_ids"`
}

// ImportCharacterQuestState validates a retained 0004 quest-state export through
// the existing quarantine contract and inserts the canonicalized rows into
// character_quest_flags inside one transaction.
//
// The caller still owns driver selection and DSN loading. Parent character rows
// from 0002 must already exist (or the engine FK check fails closed). This
// primitive does not mutate bootstrap file stores, does not rewrite
// schema_migrations, and does not invent upsert / merge policy: duplicate
// primary keys fail closed and roll the transaction back. The export's
// character name field is operator aid only and is not written to SQL.
func ImportCharacterQuestState(ctx context.Context, executor dbmigrations.SQLMigrationExecutor, export CharacterQuestStateExport) (CharacterQuestStateImportResult, error) {
	if questStateImportExecutorIsNil(executor) {
		return CharacterQuestStateImportResult{}, ErrCharacterQuestStateImportExecutorRequired
	}

	canonical, summary, err := QuarantineCharacterQuestStateExport(export)
	if err != nil {
		return CharacterQuestStateImportResult{}, err
	}

	result := CharacterQuestStateImportResult{
		MigrationVersion: CharacterQuestStateMigrationVersion,
		MigrationName:    CharacterQuestStateMigrationName,
		CharacterCount:   summary.CharacterCount,
		FlagCount:        summary.FlagCount,
		CharacterIDs:     append([]uint32(nil), summary.CharacterIDs...),
	}
	if result.CharacterIDs == nil {
		result.CharacterIDs = []uint32{}
	}

	tx, err := executor.BeginTx(ctx, nil)
	if err != nil {
		return CharacterQuestStateImportResult{}, fmt.Errorf("begin character quest-state import transaction: %w", err)
	}

	if err := requireCharacterQuestStateSchema(ctx, tx); err != nil {
		return CharacterQuestStateImportResult{}, rollbackAfterQuestStateImportFailure(tx, err)
	}

	for _, row := range canonical.Flags {
		if err := insertCharacterQuestFlag(ctx, tx, row); err != nil {
			return CharacterQuestStateImportResult{}, rollbackAfterQuestStateImportFailure(tx, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return CharacterQuestStateImportResult{}, fmt.Errorf("commit character quest-state import transaction: %w", err)
	}
	return result, nil
}

func requireCharacterQuestStateSchema(ctx context.Context, querier dbmigrations.SQLLedgerQuerier) error {
	ledger, err := dbmigrations.ReadSQLLedgerEntries(ctx, querier)
	if err != nil {
		return fmt.Errorf("%w: read schema_migrations: %v", ErrCharacterQuestStateImportSchemaRequired, err)
	}
	for _, entry := range ledger {
		if entry.Version == CharacterQuestStateMigrationVersion && entry.Name == CharacterQuestStateMigrationName {
			return nil
		}
	}
	latest := 0
	for _, entry := range ledger {
		if entry.Version > latest {
			latest = entry.Version
		}
	}
	return fmt.Errorf("%w: ledger tip %d missing version %d %q", ErrCharacterQuestStateImportSchemaRequired, latest, CharacterQuestStateMigrationVersion, CharacterQuestStateMigrationName)
}

func insertCharacterQuestFlag(ctx context.Context, tx *sql.Tx, row CharacterQuestFlagRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO character_quest_flags (
    character_id, quest_ref, flag_name, value
) VALUES (?, ?, ?, ?)`,
		int64(row.CharacterID), row.QuestRef, row.Flag, int64(row.Value),
	)
	if err != nil {
		return fmt.Errorf("insert quest flag character %d quest_ref %q flag %q: %w", row.CharacterID, row.QuestRef, row.Flag, err)
	}
	return requireExactQuestStateImportRows(result, "insert quest flag", int64(row.CharacterID))
}

func requireExactQuestStateImportRows(result sql.Result, action string, id int64) error {
	if result == nil {
		return fmt.Errorf("%w: %s id %d returned nil result", ErrCharacterQuestStateImportRowCount, action, id)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: %s id %d returned unknown row count: %v", ErrCharacterQuestStateImportRowCount, action, id, err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("%w: %s id %d affected %d rows", ErrCharacterQuestStateImportRowCount, action, id, rowsAffected)
	}
	return nil
}

func rollbackAfterQuestStateImportFailure(tx *sql.Tx, importErr error) error {
	if tx == nil {
		return importErr
	}
	if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
		return errors.Join(importErr, fmt.Errorf("rollback character quest-state import transaction: %w", rollbackErr))
	}
	return importErr
}

func questStateImportExecutorIsNil(executor dbmigrations.SQLMigrationExecutor) bool {
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
