package accountstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

// ErrCharacterItemStateImportExecutorRequired reports that the SQL import
// primitive was called without a transaction-capable executor.
var ErrCharacterItemStateImportExecutorRequired = errors.New("character item-state import executor is required")

// ErrCharacterItemStateImportSchemaRequired reports that the target database
// has not applied the 0003_character_item_state migration boundary and additive
// 0024 instance-socket columns yet.
var ErrCharacterItemStateImportSchemaRequired = errors.New("character item-state schema is not applied")

// ErrCharacterItemStateImportRowCount reports that an INSERT affected an
// unexpected number of rows during item-state backfill.
var ErrCharacterItemStateImportRowCount = errors.New("character item-state import row count mismatch")

// CharacterItemStateImportResult is the metadata-only outcome of importing a
// quarantined 0003 item-state export. It never includes item payloads, SQL
// text, DSNs, or account snapshot bytes.
type CharacterItemStateImportResult struct {
	MigrationVersion   int      `json:"migration_version"`
	MigrationName      string   `json:"migration_name"`
	CharacterCount     int      `json:"character_count"`
	InventoryItemCount int      `json:"inventory_item_count"`
	EquipmentItemCount int      `json:"equipment_item_count"`
	QuickslotCount     int      `json:"quickslot_count"`
	CharacterIDs       []uint32 `json:"character_ids"`
}

// ImportCharacterItemState validates a retained 0003 item-state export through
// the existing quarantine contract and inserts the canonicalized rows into
// character_inventory_items / character_equipment_items / character_quickslots
// inside one transaction.
//
// The caller still owns driver selection and DSN loading. Parent character rows
// from 0002 must already exist (or the engine FK check fails closed). This
// primitive does not mutate bootstrap file stores, does not rewrite
// schema_migrations, and does not invent upsert / merge policy: duplicate
// primary keys fail closed and roll the transaction back.
func ImportCharacterItemState(ctx context.Context, executor dbmigrations.SQLMigrationExecutor, export CharacterItemStateExport) (CharacterItemStateImportResult, error) {
	if itemStateImportExecutorIsNil(executor) {
		return CharacterItemStateImportResult{}, ErrCharacterItemStateImportExecutorRequired
	}

	canonical, summary, err := QuarantineCharacterItemStateExport(export)
	if err != nil {
		return CharacterItemStateImportResult{}, err
	}

	result := CharacterItemStateImportResult{
		MigrationVersion:   CharacterItemStateMigrationVersion,
		MigrationName:      CharacterItemStateMigrationName,
		CharacterCount:     summary.CharacterCount,
		InventoryItemCount: summary.InventoryItemCount,
		EquipmentItemCount: summary.EquipmentItemCount,
		QuickslotCount:     summary.QuickslotCount,
		CharacterIDs:       append([]uint32(nil), summary.CharacterIDs...),
	}
	if result.CharacterIDs == nil {
		result.CharacterIDs = []uint32{}
	}

	tx, err := executor.BeginTx(ctx, nil)
	if err != nil {
		return CharacterItemStateImportResult{}, fmt.Errorf("begin character item-state import transaction: %w", err)
	}

	if err := requireCharacterItemStateSchema(ctx, tx); err != nil {
		return CharacterItemStateImportResult{}, rollbackAfterItemStateImportFailure(tx, err)
	}

	for _, row := range canonical.InventoryItems {
		if err := insertCharacterInventoryItem(ctx, tx, row); err != nil {
			return CharacterItemStateImportResult{}, rollbackAfterItemStateImportFailure(tx, err)
		}
	}
	for _, row := range canonical.EquipmentItems {
		if err := insertCharacterEquipmentItem(ctx, tx, row); err != nil {
			return CharacterItemStateImportResult{}, rollbackAfterItemStateImportFailure(tx, err)
		}
	}
	for _, row := range canonical.Quickslots {
		if err := insertCharacterQuickslot(ctx, tx, row); err != nil {
			return CharacterItemStateImportResult{}, rollbackAfterItemStateImportFailure(tx, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return CharacterItemStateImportResult{}, fmt.Errorf("commit character item-state import transaction: %w", err)
	}
	return result, nil
}

func requireCharacterItemStateSchema(ctx context.Context, querier dbmigrations.SQLLedgerQuerier) error {
	ledger, err := dbmigrations.ReadSQLLedgerEntries(ctx, querier)
	if err != nil {
		return fmt.Errorf("%w: read schema_migrations: %v", ErrCharacterItemStateImportSchemaRequired, err)
	}
	hasItemState := false
	hasInstanceSockets := false
	latest := 0
	for _, entry := range ledger {
		if entry.Version > latest {
			latest = entry.Version
		}
		if entry.Version == CharacterItemStateMigrationVersion && entry.Name == CharacterItemStateMigrationName {
			hasItemState = true
		}
		if entry.Version == CharacterItemInstanceSocketsMigrationVersion && entry.Name == CharacterItemInstanceSocketsMigrationName {
			hasInstanceSockets = true
		}
	}
	if hasItemState && hasInstanceSockets {
		return nil
	}
	if !hasItemState {
		return fmt.Errorf("%w: ledger tip %d missing version %d %q", ErrCharacterItemStateImportSchemaRequired, latest, CharacterItemStateMigrationVersion, CharacterItemStateMigrationName)
	}
	return fmt.Errorf("%w: ledger tip %d missing version %d %q", ErrCharacterItemStateImportSchemaRequired, latest, CharacterItemInstanceSocketsMigrationVersion, CharacterItemInstanceSocketsMigrationName)
}

func insertCharacterInventoryItem(ctx context.Context, tx *sql.Tx, row CharacterInventoryItemRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO character_inventory_items (
    id, character_id, slot, vnum, count, locked, has_sockets, socket0, socket1, socket2
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		int64(row.ID), int64(row.CharacterID), int(row.Slot), int64(row.Vnum), int(row.Count), boolToSQLInt(row.Locked),
		boolToSQLInt(row.HasSockets), int64(row.Socket0), int64(row.Socket1), int64(row.Socket2),
	)
	if err != nil {
		return fmt.Errorf("insert inventory item id %d: %w", row.ID, err)
	}
	return requireExactItemStateImportRows(result, "insert inventory item", int64(row.ID))
}

func insertCharacterEquipmentItem(ctx context.Context, tx *sql.Tx, row CharacterEquipmentItemRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO character_equipment_items (
    id, character_id, equip_slot, vnum, count, locked, has_sockets, socket0, socket1, socket2
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		int64(row.ID), int64(row.CharacterID), row.EquipSlot, int64(row.Vnum), int(row.Count), boolToSQLInt(row.Locked),
		boolToSQLInt(row.HasSockets), int64(row.Socket0), int64(row.Socket1), int64(row.Socket2),
	)
	if err != nil {
		return fmt.Errorf("insert equipment item id %d: %w", row.ID, err)
	}
	return requireExactItemStateImportRows(result, "insert equipment item", int64(row.ID))
}

func insertCharacterQuickslot(ctx context.Context, tx *sql.Tx, row CharacterQuickslotRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO character_quickslots (
    character_id, position, type, slot
) VALUES (?, ?, ?, ?)`,
		int64(row.CharacterID), int(row.Position), int(row.Type), int(row.Slot),
	)
	if err != nil {
		return fmt.Errorf("insert quickslot character %d position %d: %w", row.CharacterID, row.Position, err)
	}
	return requireExactItemStateImportRows(result, "insert quickslot", int64(row.CharacterID)*1000+int64(row.Position))
}

func boolToSQLInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func requireExactItemStateImportRows(result sql.Result, action string, id int64) error {
	if result == nil {
		return fmt.Errorf("%w: %s id %d returned nil result", ErrCharacterItemStateImportRowCount, action, id)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: %s id %d returned unknown row count: %v", ErrCharacterItemStateImportRowCount, action, id, err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("%w: %s id %d affected %d rows", ErrCharacterItemStateImportRowCount, action, id, rowsAffected)
	}
	return nil
}

func rollbackAfterItemStateImportFailure(tx *sql.Tx, importErr error) error {
	if tx == nil {
		return importErr
	}
	if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
		return errors.Join(importErr, fmt.Errorf("rollback character item-state import transaction: %w", rollbackErr))
	}
	return importErr
}

func itemStateImportExecutorIsNil(executor dbmigrations.SQLMigrationExecutor) bool {
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
