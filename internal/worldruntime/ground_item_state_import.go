package worldruntime

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

// ErrBootstrapGroundItemStateImportExecutorRequired reports that the SQL import
// primitive was called without a transaction-capable executor.
var ErrBootstrapGroundItemStateImportExecutorRequired = errors.New("bootstrap ground-item-state import executor is required")

// ErrBootstrapGroundItemStateImportSchemaRequired reports that the target
// database has not applied the 0010_bootstrap_ground_item_state migration
// boundary yet.
var ErrBootstrapGroundItemStateImportSchemaRequired = errors.New("bootstrap ground-item-state schema is not applied")

// ErrBootstrapGroundItemStateImportRowCount reports that an INSERT affected an
// unexpected number of rows during ground-item-state backfill.
var ErrBootstrapGroundItemStateImportRowCount = errors.New("bootstrap ground-item-state import row count mismatch")

// BootstrapGroundItemStateImportResult is the metadata-only outcome of importing
// a quarantined 0010 ground-item-state export. It never includes ground
// payloads, SQL text, DSNs, or FileStore snapshot bytes.
type BootstrapGroundItemStateImportResult struct {
	MigrationVersion int      `json:"migration_version"`
	MigrationName    string   `json:"migration_name"`
	GroundItemCount  int      `json:"ground_item_count"`
	ItemShapedCount  int      `json:"item_shaped_count"`
	GoldShapedCount  int      `json:"gold_shaped_count"`
	VIDs             []uint32 `json:"vids"`
}

// ImportBootstrapGroundItemState validates a retained 0010 ground-item-state
// export through the existing quarantine contract and inserts the canonicalized
// rows into bootstrap_ground_items inside one transaction.
//
// The caller still owns driver selection and DSN loading. Parent character rows
// from 0002 must already exist (or the engine FK check fails closed). This
// primitive does not mutate bootstrap file stores or live shared-world handles,
// does not rewrite schema_migrations, and does not invent upsert / merge
// policy: duplicate primary keys fail closed and roll the transaction back.
func ImportBootstrapGroundItemState(ctx context.Context, executor dbmigrations.SQLMigrationExecutor, export BootstrapGroundItemStateExport) (BootstrapGroundItemStateImportResult, error) {
	if groundItemStateImportExecutorIsNil(executor) {
		return BootstrapGroundItemStateImportResult{}, ErrBootstrapGroundItemStateImportExecutorRequired
	}

	canonical, summary, err := QuarantineBootstrapGroundItemStateExport(export)
	if err != nil {
		return BootstrapGroundItemStateImportResult{}, err
	}

	result := BootstrapGroundItemStateImportResult{
		MigrationVersion: BootstrapGroundItemStateMigrationVersion,
		MigrationName:    BootstrapGroundItemStateMigrationName,
		GroundItemCount:  summary.GroundItemCount,
		ItemShapedCount:  summary.ItemShapedCount,
		GoldShapedCount:  summary.GoldShapedCount,
		VIDs:             append([]uint32(nil), summary.VIDs...),
	}
	if result.VIDs == nil {
		result.VIDs = []uint32{}
	}

	tx, err := executor.BeginTx(ctx, nil)
	if err != nil {
		return BootstrapGroundItemStateImportResult{}, fmt.Errorf("begin bootstrap ground-item-state import transaction: %w", err)
	}

	if err := requireBootstrapGroundItemStateSchema(ctx, tx); err != nil {
		return BootstrapGroundItemStateImportResult{}, rollbackAfterGroundItemStateImportFailure(tx, err)
	}

	for _, row := range canonical.GroundItems {
		if err := insertBootstrapGroundItem(ctx, tx, row); err != nil {
			return BootstrapGroundItemStateImportResult{}, rollbackAfterGroundItemStateImportFailure(tx, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return BootstrapGroundItemStateImportResult{}, fmt.Errorf("commit bootstrap ground-item-state import transaction: %w", err)
	}
	return result, nil
}

func requireBootstrapGroundItemStateSchema(ctx context.Context, querier dbmigrations.SQLLedgerQuerier) error {
	ledger, err := dbmigrations.ReadSQLLedgerEntries(ctx, querier)
	if err != nil {
		return fmt.Errorf("%w: read schema_migrations: %v", ErrBootstrapGroundItemStateImportSchemaRequired, err)
	}
	for _, entry := range ledger {
		if entry.Version == BootstrapGroundItemStateMigrationVersion && entry.Name == BootstrapGroundItemStateMigrationName {
			return nil
		}
	}
	latest := 0
	for _, entry := range ledger {
		if entry.Version > latest {
			latest = entry.Version
		}
	}
	return fmt.Errorf("%w: ledger tip %d missing version %d %q", ErrBootstrapGroundItemStateImportSchemaRequired, latest, BootstrapGroundItemStateMigrationVersion, BootstrapGroundItemStateMigrationName)
}

func insertBootstrapGroundItem(ctx context.Context, tx *sql.Tx, row BootstrapGroundItemStateRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO bootstrap_ground_items (
    vid, vnum, item_count, gold_amount, owner_login, owner_character_id, owner_vid, owner_name, map_index, x, y, z, pickup_range
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		int64(row.VID),
		int64(row.Vnum),
		nullableUint16SQL(row.ItemCount),
		nullableUint32SQL(row.GoldAmount),
		row.OwnerLogin,
		int64(row.OwnerCharacterID),
		int64(row.OwnerVID),
		row.OwnerName,
		int64(row.MapIndex),
		int64(row.X),
		int64(row.Y),
		int64(row.Z),
		row.PickupRange,
	)
	if err != nil {
		return fmt.Errorf("insert bootstrap ground item vid %d: %w", row.VID, err)
	}
	return requireExactGroundItemStateImportRows(result, "insert bootstrap ground item", int64(row.VID))
}

func nullableUint16SQL(value *uint16) any {
	if value == nil {
		return nil
	}
	return int64(*value)
}

func nullableUint32SQL(value *uint32) any {
	if value == nil {
		return nil
	}
	return int64(*value)
}

func requireExactGroundItemStateImportRows(result sql.Result, action string, id int64) error {
	if result == nil {
		return fmt.Errorf("%w: %s id %d returned nil result", ErrBootstrapGroundItemStateImportRowCount, action, id)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: %s id %d returned unknown row count: %v", ErrBootstrapGroundItemStateImportRowCount, action, id, err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("%w: %s id %d affected %d rows", ErrBootstrapGroundItemStateImportRowCount, action, id, rowsAffected)
	}
	return nil
}

func rollbackAfterGroundItemStateImportFailure(tx *sql.Tx, importErr error) error {
	if tx == nil {
		return importErr
	}
	if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
		return errors.Join(importErr, fmt.Errorf("rollback bootstrap ground-item-state import transaction: %w", rollbackErr))
	}
	return importErr
}

func groundItemStateImportExecutorIsNil(executor dbmigrations.SQLMigrationExecutor) bool {
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
