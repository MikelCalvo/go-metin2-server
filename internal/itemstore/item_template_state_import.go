package itemstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

// ErrItemTemplateStateImportExecutorRequired reports that the SQL import
// primitive was called without a transaction-capable executor.
var ErrItemTemplateStateImportExecutorRequired = errors.New("item-template-state import executor is required")

// ErrItemTemplateStateImportSchemaRequired reports that the target database has
// not applied the tip-0009 item_template_refine_info boundary and/or the
// additive 0021 keep_on_fail / 0022 fail_result_vnum columns yet.
var ErrItemTemplateStateImportSchemaRequired = errors.New("item-template-state schema is not applied")

// ErrItemTemplateStateImportRowCount reports that an INSERT affected an
// unexpected number of rows during item-template-state backfill.
var ErrItemTemplateStateImportRowCount = errors.New("item-template-state import row count mismatch")

// ItemTemplateStateImportResult is the metadata-only outcome of importing a
// quarantined 0009 item-template-state export. It never includes template
// payloads, SQL text, DSNs, or FileStore snapshot bytes.
type ItemTemplateStateImportResult struct {
	MigrationVersion    int      `json:"migration_version"`
	MigrationName       string   `json:"migration_name"`
	TemplateCount       int      `json:"template_count"`
	SocketCount         int      `json:"socket_count"`
	AttributeCount      int      `json:"attribute_count"`
	UseEffectCount      int      `json:"use_effect_count"`
	EquipEffectCount    int      `json:"equip_effect_count"`
	RefineInfoCount     int      `json:"refine_info_count"`
	RefineMaterialCount int      `json:"refine_material_count"`
	Vnums               []uint32 `json:"vnums"`
	// Replaced is true when ImportItemTemplateState ran with the opt-in
	// scoped replace policy (delete then insert for listed template vnums).
	// Omitted from JSON when false so legacy insert-only import-result files stay
	// valid.
	Replaced bool `json:"replaced,omitempty"`
}

// ImportItemTemplateStateOptions controls opt-in mutation policy for
// ImportItemTemplateState. The zero value keeps today's insert-only behavior.
type ImportItemTemplateStateOptions struct {
	// Replace, when true, deletes existing tip-0009 parent+child rows for every
	// template vnum in the quarantined export summary before inserting the
	// canonicalized export rows, all inside one transaction. Vnums not listed in
	// the export remain untouched.
	Replace bool
}

// ImportItemTemplateState validates a retained 0009 item-template-state export
// through the existing quarantine contract and inserts the canonicalized rows
// into item_templates plus child socket / attribute / effect / refine tables
// inside one transaction.
//
// The caller still owns driver selection and DSN loading. This primitive does
// not mutate bootstrap file stores or live template indexes and does not rewrite
// schema_migrations. Without options (or with Replace=false) it does not invent
// upsert / merge policy: duplicate primary keys fail closed and roll the
// transaction back. Pass ImportItemTemplateStateOptions{Replace: true} for the
// opt-in scoped replace path frozen by the tip-0009 replace contract.
func ImportItemTemplateState(ctx context.Context, executor dbmigrations.SQLMigrationExecutor, export ItemTemplateStateExport, opts ...ImportItemTemplateStateOptions) (ItemTemplateStateImportResult, error) {
	if itemTemplateStateImportExecutorIsNil(executor) {
		return ItemTemplateStateImportResult{}, ErrItemTemplateStateImportExecutorRequired
	}
	if len(opts) > 1 {
		return ItemTemplateStateImportResult{}, fmt.Errorf("ImportItemTemplateState accepts at most one options value")
	}
	replace := false
	if len(opts) == 1 {
		replace = opts[0].Replace
	}

	canonical, summary, err := QuarantineItemTemplateStateExport(export)
	if err != nil {
		return ItemTemplateStateImportResult{}, err
	}

	result := ItemTemplateStateImportResult{
		MigrationVersion:    ItemTemplateStateMigrationVersion,
		MigrationName:       ItemTemplateStateMigrationName,
		TemplateCount:       summary.TemplateCount,
		SocketCount:         summary.SocketCount,
		AttributeCount:      summary.AttributeCount,
		UseEffectCount:      summary.UseEffectCount,
		EquipEffectCount:    summary.EquipEffectCount,
		RefineInfoCount:     summary.RefineInfoCount,
		RefineMaterialCount: summary.RefineMaterialCount,
		Vnums:               append([]uint32(nil), summary.Vnums...),
		Replaced:            replace,
	}
	if result.Vnums == nil {
		result.Vnums = []uint32{}
	}

	tx, err := executor.BeginTx(ctx, nil)
	if err != nil {
		return ItemTemplateStateImportResult{}, fmt.Errorf("begin item-template-state import transaction: %w", err)
	}

	if err := requireItemTemplateStateSchema(ctx, tx); err != nil {
		return ItemTemplateStateImportResult{}, rollbackAfterItemTemplateStateImportFailure(tx, err)
	}

	if replace {
		for _, vnum := range summary.Vnums {
			if err := deleteItemTemplateStateForVnum(ctx, tx, vnum); err != nil {
				return ItemTemplateStateImportResult{}, rollbackAfterItemTemplateStateImportFailure(tx, err)
			}
		}
	}

	for _, row := range canonical.Templates {
		if err := insertItemTemplate(ctx, tx, row); err != nil {
			return ItemTemplateStateImportResult{}, rollbackAfterItemTemplateStateImportFailure(tx, err)
		}
	}
	for _, row := range canonical.Sockets {
		if err := insertItemTemplateSocket(ctx, tx, row); err != nil {
			return ItemTemplateStateImportResult{}, rollbackAfterItemTemplateStateImportFailure(tx, err)
		}
	}
	for _, row := range canonical.Attributes {
		if err := insertItemTemplateAttribute(ctx, tx, row); err != nil {
			return ItemTemplateStateImportResult{}, rollbackAfterItemTemplateStateImportFailure(tx, err)
		}
	}
	for _, row := range canonical.UseEffects {
		if err := insertItemTemplateUseEffect(ctx, tx, row); err != nil {
			return ItemTemplateStateImportResult{}, rollbackAfterItemTemplateStateImportFailure(tx, err)
		}
	}
	for _, row := range canonical.EquipEffects {
		if err := insertItemTemplateEquipEffect(ctx, tx, row); err != nil {
			return ItemTemplateStateImportResult{}, rollbackAfterItemTemplateStateImportFailure(tx, err)
		}
	}
	for _, row := range canonical.RefineInfos {
		if err := insertItemTemplateRefineInfo(ctx, tx, row); err != nil {
			return ItemTemplateStateImportResult{}, rollbackAfterItemTemplateStateImportFailure(tx, err)
		}
	}
	for _, row := range canonical.RefineMaterials {
		if err := insertItemTemplateRefineMaterial(ctx, tx, row); err != nil {
			return ItemTemplateStateImportResult{}, rollbackAfterItemTemplateStateImportFailure(tx, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return ItemTemplateStateImportResult{}, fmt.Errorf("commit item-template-state import transaction: %w", err)
	}
	return result, nil
}

func requireItemTemplateStateSchema(ctx context.Context, querier dbmigrations.SQLLedgerQuerier) error {
	ledger, err := dbmigrations.ReadSQLLedgerEntries(ctx, querier)
	if err != nil {
		return fmt.Errorf("%w: read schema_migrations: %v", ErrItemTemplateStateImportSchemaRequired, err)
	}
	hasRefineInfo := false
	hasKeepOnFail := false
	hasFailResultVnum := false
	latest := 0
	for _, entry := range ledger {
		if entry.Version > latest {
			latest = entry.Version
		}
		if entry.Version == ItemTemplateStateMigrationVersion && entry.Name == ItemTemplateStateMigrationName {
			hasRefineInfo = true
		}
		if entry.Version == ItemTemplateRefineKeepOnFailMigrationVersion && entry.Name == ItemTemplateRefineKeepOnFailMigrationName {
			hasKeepOnFail = true
		}
		if entry.Version == ItemTemplateRefineFailResultVnumMigrationVersion && entry.Name == ItemTemplateRefineFailResultVnumMigrationName {
			hasFailResultVnum = true
		}
	}
	if hasRefineInfo && hasKeepOnFail && hasFailResultVnum {
		return nil
	}
	if !hasRefineInfo {
		return fmt.Errorf("%w: ledger tip %d missing version %d %q", ErrItemTemplateStateImportSchemaRequired, latest, ItemTemplateStateMigrationVersion, ItemTemplateStateMigrationName)
	}
	if !hasKeepOnFail {
		return fmt.Errorf("%w: ledger tip %d missing version %d %q", ErrItemTemplateStateImportSchemaRequired, latest, ItemTemplateRefineKeepOnFailMigrationVersion, ItemTemplateRefineKeepOnFailMigrationName)
	}
	return fmt.Errorf("%w: ledger tip %d missing version %d %q", ErrItemTemplateStateImportSchemaRequired, latest, ItemTemplateRefineFailResultVnumMigrationVersion, ItemTemplateRefineFailResultVnumMigrationName)
}

func deleteItemTemplateStateForVnum(ctx context.Context, tx *sql.Tx, vnum uint32) error {
	// Child tables declare FOREIGN KEY (... ) REFERENCES item_templates(vnum)
	// without ON DELETE CASCADE, so delete children before the parent.
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_template_refine_materials WHERE vnum = ?`, int64(vnum)); err != nil {
		return fmt.Errorf("delete item template refine materials vnum %d: %w", vnum, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_template_refine_infos WHERE vnum = ?`, int64(vnum)); err != nil {
		return fmt.Errorf("delete item template refine infos vnum %d: %w", vnum, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_template_sockets WHERE vnum = ?`, int64(vnum)); err != nil {
		return fmt.Errorf("delete item template sockets vnum %d: %w", vnum, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_template_attributes WHERE vnum = ?`, int64(vnum)); err != nil {
		return fmt.Errorf("delete item template attributes vnum %d: %w", vnum, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_template_use_effects WHERE vnum = ?`, int64(vnum)); err != nil {
		return fmt.Errorf("delete item template use effects vnum %d: %w", vnum, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_template_equip_effects WHERE vnum = ?`, int64(vnum)); err != nil {
		return fmt.Errorf("delete item template equip effects vnum %d: %w", vnum, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_templates WHERE vnum = ?`, int64(vnum)); err != nil {
		return fmt.Errorf("delete item template vnum %d: %w", vnum, err)
	}
	return nil
}

func insertItemTemplate(ctx context.Context, tx *sql.Tx, row ItemTemplateRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO item_templates (
    vnum, name, stackable, max_count, shop_buy_price, shop_sell_price, refineable, refine_reject_message,
    save, sell_count_per_gold, slow_query, highlight, rare, unique_item, make_count, irremovable,
    confirm_when_use, quest_use, quest_use_multiple, log, applicable, appearance_vnum,
    anti_sell, anti_drop, anti_give, anti_stack, anti_get, anti_male, anti_female,
    anti_warrior, anti_assassin, anti_sura, anti_shaman, anti_empire_a, anti_empire_b, anti_empire_c,
    anti_save, anti_pk_drop, anti_myshop, anti_safebox, safebox_reject_message, min_level, equip_slot,
    use_reject_message, buy_reject_message, drop_reject_message, give_reject_message,
    pickup_reject_message, sell_reject_message, equip_reject_message, unequip_reject_message, pickup_range
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		int64(row.Vnum),
		row.Name,
		boolToSQLInt(row.Stackable),
		int(row.MaxCount),
		int64(row.ShopBuyPrice),
		int64(row.ShopSellPrice),
		boolToSQLInt(row.Refineable),
		row.RefineRejectText,
		boolToSQLInt(row.Save),
		boolToSQLInt(row.SellCountPerGold),
		boolToSQLInt(row.SlowQuery),
		boolToSQLInt(row.Highlight),
		boolToSQLInt(row.Rare),
		boolToSQLInt(row.Unique),
		boolToSQLInt(row.MakeCount),
		boolToSQLInt(row.Irremovable),
		boolToSQLInt(row.ConfirmWhenUse),
		boolToSQLInt(row.QuestUse),
		boolToSQLInt(row.QuestUseMultiple),
		boolToSQLInt(row.Log),
		boolToSQLInt(row.Applicable),
		int64(row.AppearanceVnum),
		boolToSQLInt(row.AntiSell),
		boolToSQLInt(row.AntiDrop),
		boolToSQLInt(row.AntiGive),
		boolToSQLInt(row.AntiStack),
		boolToSQLInt(row.AntiGet),
		boolToSQLInt(row.AntiMale),
		boolToSQLInt(row.AntiFemale),
		boolToSQLInt(row.AntiWarrior),
		boolToSQLInt(row.AntiAssassin),
		boolToSQLInt(row.AntiSura),
		boolToSQLInt(row.AntiShaman),
		boolToSQLInt(row.AntiEmpireA),
		boolToSQLInt(row.AntiEmpireB),
		boolToSQLInt(row.AntiEmpireC),
		boolToSQLInt(row.AntiSave),
		boolToSQLInt(row.AntiPKDrop),
		boolToSQLInt(row.AntiMyShop),
		boolToSQLInt(row.AntiSafebox),
		row.SafeboxRejectText,
		int(row.MinLevel),
		row.EquipSlot,
		row.UseRejectText,
		row.BuyRejectText,
		row.DropRejectText,
		row.GiveRejectText,
		row.PickupRejectText,
		row.SellRejectText,
		row.EquipRejectText,
		row.UnequipRejectText,
		int(row.PickupRange),
	)
	if err != nil {
		return fmt.Errorf("insert item template vnum %d: %w", row.Vnum, err)
	}
	return requireExactItemTemplateStateImportRows(result, "insert item template", int64(row.Vnum))
}

func insertItemTemplateSocket(ctx context.Context, tx *sql.Tx, row ItemTemplateSocketRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO item_template_sockets (
    vnum, position, value
) VALUES (?, ?, ?)`,
		int64(row.Vnum), int(row.Position), int(row.Value),
	)
	if err != nil {
		return fmt.Errorf("insert item template socket vnum %d position %d: %w", row.Vnum, row.Position, err)
	}
	return requireExactItemTemplateStateImportRows(result, "insert item template socket", int64(row.Vnum)*10+int64(row.Position))
}

func insertItemTemplateAttribute(ctx context.Context, tx *sql.Tx, row ItemTemplateAttributeRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO item_template_attributes (
    vnum, position, type, value
) VALUES (?, ?, ?, ?)`,
		int64(row.Vnum), int(row.Position), int(row.Type), int(row.Value),
	)
	if err != nil {
		return fmt.Errorf("insert item template attribute vnum %d position %d: %w", row.Vnum, row.Position, err)
	}
	return requireExactItemTemplateStateImportRows(result, "insert item template attribute", int64(row.Vnum)*10+int64(row.Position))
}

func insertItemTemplateUseEffect(ctx context.Context, tx *sql.Tx, row ItemTemplateUseEffectRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO item_template_use_effects (
    vnum, point_type, point_index, point_delta, consume_count, message, info_message, special_effect_type
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		int64(row.Vnum), int(row.PointType), int(row.PointIndex), int(row.PointDelta),
		int(row.ConsumeCount), row.Message, row.InfoMessage, int(row.SpecialEffectType),
	)
	if err != nil {
		return fmt.Errorf("insert item template use effect vnum %d: %w", row.Vnum, err)
	}
	return requireExactItemTemplateStateImportRows(result, "insert item template use effect", int64(row.Vnum))
}

func insertItemTemplateEquipEffect(ctx context.Context, tx *sql.Tx, row ItemTemplateEquipEffectRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO item_template_equip_effects (
    vnum, point_type, point_index, point_delta
) VALUES (?, ?, ?, ?)`,
		int64(row.Vnum), int(row.PointType), int(row.PointIndex), int(row.PointDelta),
	)
	if err != nil {
		return fmt.Errorf("insert item template equip effect vnum %d: %w", row.Vnum, err)
	}
	return requireExactItemTemplateStateImportRows(result, "insert item template equip effect", int64(row.Vnum))
}

func insertItemTemplateRefineInfo(ctx context.Context, tx *sql.Tx, row ItemTemplateRefineInfoRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO item_template_refine_infos (
    vnum, result_vnum, cost, probability, keep_on_fail, fail_result_vnum
) VALUES (?, ?, ?, ?, ?, ?)`,
		int64(row.Vnum), int64(row.ResultVnum), int(row.Cost), int(row.Probability), boolToSQLInt(row.KeepOnFail), int64(row.FailResultVnum),
	)
	if err != nil {
		return fmt.Errorf("insert item template refine info vnum %d: %w", row.Vnum, err)
	}
	return requireExactItemTemplateStateImportRows(result, "insert item template refine info", int64(row.Vnum))
}

func insertItemTemplateRefineMaterial(ctx context.Context, tx *sql.Tx, row ItemTemplateRefineMaterialRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO item_template_refine_materials (
    vnum, position, item_vnum, count
) VALUES (?, ?, ?, ?)`,
		int64(row.Vnum), int(row.Position), int64(row.ItemVnum), int(row.Count),
	)
	if err != nil {
		return fmt.Errorf("insert item template refine material vnum %d position %d: %w", row.Vnum, row.Position, err)
	}
	return requireExactItemTemplateStateImportRows(result, "insert item template refine material", int64(row.Vnum)*10+int64(row.Position))
}

func boolToSQLInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func requireExactItemTemplateStateImportRows(result sql.Result, action string, id int64) error {
	if result == nil {
		return fmt.Errorf("%w: %s id %d returned nil result", ErrItemTemplateStateImportRowCount, action, id)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: %s id %d returned unknown row count: %v", ErrItemTemplateStateImportRowCount, action, id, err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("%w: %s id %d affected %d rows", ErrItemTemplateStateImportRowCount, action, id, rowsAffected)
	}
	return nil
}

func rollbackAfterItemTemplateStateImportFailure(tx *sql.Tx, importErr error) error {
	if tx == nil {
		return importErr
	}
	if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
		return errors.Join(importErr, fmt.Errorf("rollback item-template-state import transaction: %w", rollbackErr))
	}
	return importErr
}

func itemTemplateStateImportExecutorIsNil(executor dbmigrations.SQLMigrationExecutor) bool {
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
