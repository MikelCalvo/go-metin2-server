package accountstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

// ErrCharacterPointStateImportExecutorRequired reports that the SQL import
// primitive was called without a transaction-capable executor.
var ErrCharacterPointStateImportExecutorRequired = errors.New("character point-state import executor is required")

// ErrCharacterPointStateImportSchemaRequired reports that the target database
// has not applied the 0011_character_point_state migration boundary yet.
var ErrCharacterPointStateImportSchemaRequired = errors.New("character point-state schema is not applied")

// ErrCharacterPointStateImportRowCount reports that an INSERT affected an
// unexpected number of rows during point-state backfill.
var ErrCharacterPointStateImportRowCount = errors.New("character point-state import row count mismatch")

// CharacterPointStateImportResult is the metadata-only outcome of importing a
// quarantined 0011 point-state export. It never includes point payloads, SQL
// text, DSNs, or account snapshot bytes.
type CharacterPointStateImportResult struct {
	MigrationVersion int      `json:"migration_version"`
	MigrationName    string   `json:"migration_name"`
	CharacterCount   int      `json:"character_count"`
	PointRowCount    int      `json:"point_row_count"`
	CharacterIDs     []uint32 `json:"character_ids"`
	// Replaced is true when ImportCharacterPointState ran with the opt-in scoped
	// replace policy (delete then insert for listed character ids). Omitted from
	// JSON when false so legacy insert-only import-result files stay valid.
	Replaced bool `json:"replaced,omitempty"`
}

// ImportCharacterPointStateOptions controls opt-in mutation policy for
// ImportCharacterPointState. The zero value keeps today's insert-only behavior.
type ImportCharacterPointStateOptions struct {
	// Replace, when true, deletes existing tip-0011 child rows for every
	// character id in the quarantined export summary before inserting the
	// canonicalized export rows, all inside one transaction. Characters not
	// listed in the export remain untouched.
	Replace bool
}

// ImportCharacterPointState validates a retained 0011 point-state export through
// the existing quarantine contract and inserts the canonicalized rows into
// character_points inside one transaction.
//
// The caller still owns driver selection and DSN loading. Parent character rows
// from 0002 must already exist (or the engine FK check fails closed). This
// primitive does not mutate bootstrap file stores and does not rewrite
// schema_migrations. Without options (or with Replace=false) it does not invent
// upsert / merge policy: duplicate primary keys fail closed and roll the
// transaction back. Pass ImportCharacterPointStateOptions{Replace: true} for the
// opt-in scoped replace path frozen by the tip-0011 replace contract.
func ImportCharacterPointState(ctx context.Context, executor dbmigrations.SQLMigrationExecutor, export CharacterPointStateExport, opts ...ImportCharacterPointStateOptions) (CharacterPointStateImportResult, error) {
	if pointStateImportExecutorIsNil(executor) {
		return CharacterPointStateImportResult{}, ErrCharacterPointStateImportExecutorRequired
	}
	if len(opts) > 1 {
		return CharacterPointStateImportResult{}, fmt.Errorf("ImportCharacterPointState accepts at most one options value")
	}
	replace := false
	if len(opts) == 1 {
		replace = opts[0].Replace
	}

	canonical, summary, err := QuarantineCharacterPointStateExport(export)
	if err != nil {
		return CharacterPointStateImportResult{}, err
	}

	result := CharacterPointStateImportResult{
		MigrationVersion: CharacterPointStateMigrationVersion,
		MigrationName:    CharacterPointStateMigrationName,
		CharacterCount:   summary.CharacterCount,
		PointRowCount:    summary.PointRowCount,
		CharacterIDs:     append([]uint32(nil), summary.CharacterIDs...),
		Replaced:         replace,
	}
	if result.CharacterIDs == nil {
		result.CharacterIDs = []uint32{}
	}

	tx, err := executor.BeginTx(ctx, nil)
	if err != nil {
		return CharacterPointStateImportResult{}, fmt.Errorf("begin character point-state import transaction: %w", err)
	}

	if err := requireCharacterPointStateSchema(ctx, tx); err != nil {
		return CharacterPointStateImportResult{}, rollbackAfterPointStateImportFailure(tx, err)
	}

	if replace {
		for _, characterID := range summary.CharacterIDs {
			if err := deleteCharacterPointsForCharacter(ctx, tx, characterID); err != nil {
				return CharacterPointStateImportResult{}, rollbackAfterPointStateImportFailure(tx, err)
			}
		}
	}

	for _, row := range canonical.Points {
		if err := insertCharacterPoint(ctx, tx, row); err != nil {
			return CharacterPointStateImportResult{}, rollbackAfterPointStateImportFailure(tx, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return CharacterPointStateImportResult{}, fmt.Errorf("commit character point-state import transaction: %w", err)
	}
	return result, nil
}

func requireCharacterPointStateSchema(ctx context.Context, querier dbmigrations.SQLLedgerQuerier) error {
	ledger, err := dbmigrations.ReadSQLLedgerEntries(ctx, querier)
	if err != nil {
		return fmt.Errorf("%w: read schema_migrations: %v", ErrCharacterPointStateImportSchemaRequired, err)
	}
	for _, entry := range ledger {
		if entry.Version == CharacterPointStateMigrationVersion && entry.Name == CharacterPointStateMigrationName {
			return nil
		}
	}
	latest := 0
	for _, entry := range ledger {
		if entry.Version > latest {
			latest = entry.Version
		}
	}
	return fmt.Errorf("%w: ledger tip %d missing version %d %q", ErrCharacterPointStateImportSchemaRequired, latest, CharacterPointStateMigrationVersion, CharacterPointStateMigrationName)
}

func deleteCharacterPointsForCharacter(ctx context.Context, tx *sql.Tx, characterID uint32) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM character_points WHERE character_id = ?`, int64(characterID)); err != nil {
		return fmt.Errorf("delete points for character %d: %w", characterID, err)
	}
	return nil
}

func insertCharacterPoint(ctx context.Context, tx *sql.Tx, row CharacterPointRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO character_points (
    character_id, point_index, value
) VALUES (?, ?, ?)`,
		int64(row.CharacterID), int(row.PointIndex), int64(row.Value),
	)
	if err != nil {
		return fmt.Errorf("insert point character %d index %d: %w", row.CharacterID, row.PointIndex, err)
	}
	return requireExactPointStateImportRows(result, "insert point", int64(row.CharacterID)*1000+int64(row.PointIndex))
}

func requireExactPointStateImportRows(result sql.Result, action string, id int64) error {
	if result == nil {
		return fmt.Errorf("%w: %s id %d returned nil result", ErrCharacterPointStateImportRowCount, action, id)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: %s id %d returned unknown row count: %v", ErrCharacterPointStateImportRowCount, action, id, err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("%w: %s id %d affected %d rows", ErrCharacterPointStateImportRowCount, action, id, rowsAffected)
	}
	return nil
}

func rollbackAfterPointStateImportFailure(tx *sql.Tx, importErr error) error {
	if tx == nil {
		return importErr
	}
	if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
		return errors.Join(importErr, fmt.Errorf("rollback character point-state import transaction: %w", rollbackErr))
	}
	return importErr
}

func pointStateImportExecutorIsNil(executor dbmigrations.SQLMigrationExecutor) bool {
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
