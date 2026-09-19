package cubestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

// ErrCubeRecipeStateImportExecutorRequired reports that the SQL import primitive
// was called without a transaction-capable executor.
var ErrCubeRecipeStateImportExecutorRequired = errors.New("cube-recipe-state import executor is required")

// ErrCubeRecipeStateImportSchemaRequired reports that the target database has
// not applied the 0031_cube_recipe_state migration boundary yet.
var ErrCubeRecipeStateImportSchemaRequired = errors.New("cube-recipe-state schema is not applied")

// ErrCubeRecipeStateImportRowCount reports that an INSERT affected an unexpected
// number of rows during cube-recipe-state backfill.
var ErrCubeRecipeStateImportRowCount = errors.New("cube-recipe-state import row count mismatch")

// CubeRecipeStateImportResult is the metadata-only outcome of importing a
// quarantined 0031 cube-recipe-state export. It never includes recipe payloads,
// SQL text, DSNs, or FileStore snapshot bytes.
type CubeRecipeStateImportResult struct {
	MigrationVersion    int      `json:"migration_version"`
	MigrationName       string   `json:"migration_name"`
	NPCCount            int      `json:"npc_count"`
	RecipeCount         int      `json:"recipe_count"`
	MaterialCount       int      `json:"material_count"`
	MaterialOptionCount int      `json:"material_option_count"`
	NPCVnums            []uint32 `json:"npc_vnums"`
	// Replaced is true when ImportCubeRecipeState ran with the opt-in scoped
	// replace policy (delete then insert for listed npc vnums). Omitted from
	// JSON when false so legacy insert-only import-result files stay valid.
	Replaced bool `json:"replaced,omitempty"`
}

// ImportCubeRecipeStateOptions controls opt-in mutation policy for
// ImportCubeRecipeState. The zero value keeps today's insert-only behavior.
type ImportCubeRecipeStateOptions struct {
	// Replace, when true, deletes existing tip-0031 parent+child rows for every
	// npc vnum in the quarantined export summary before inserting the
	// canonicalized export rows, all inside one transaction. NPCs not listed in
	// the export remain untouched.
	Replace bool
}

// ImportCubeRecipeState validates a retained 0031 cube-recipe-state export
// through the existing quarantine contract and inserts the canonicalized rows
// into cube_recipe_npcs plus child recipe / material / material_option tables
// inside one transaction.
//
// The caller still owns driver selection and DSN loading. This primitive does
// not mutate bootstrap file stores or live cube recipes and does not rewrite
// schema_migrations. Without options (or with Replace=false) it does not invent
// upsert / merge policy: duplicate primary keys fail closed and roll the
// transaction back. Pass ImportCubeRecipeStateOptions{Replace: true} for the
// opt-in scoped replace path.
func ImportCubeRecipeState(ctx context.Context, executor dbmigrations.SQLMigrationExecutor, export CubeRecipeStateExport, opts ...ImportCubeRecipeStateOptions) (CubeRecipeStateImportResult, error) {
	if cubeRecipeStateImportExecutorIsNil(executor) {
		return CubeRecipeStateImportResult{}, ErrCubeRecipeStateImportExecutorRequired
	}
	if len(opts) > 1 {
		return CubeRecipeStateImportResult{}, fmt.Errorf("ImportCubeRecipeState accepts at most one options value")
	}
	replace := false
	if len(opts) == 1 {
		replace = opts[0].Replace
	}

	canonical, summary, err := QuarantineCubeRecipeStateExport(export)
	if err != nil {
		return CubeRecipeStateImportResult{}, err
	}

	result := CubeRecipeStateImportResult{
		MigrationVersion:    CubeRecipeStateMigrationVersion,
		MigrationName:       CubeRecipeStateMigrationName,
		NPCCount:            summary.NPCCount,
		RecipeCount:         summary.RecipeCount,
		MaterialCount:       summary.MaterialCount,
		MaterialOptionCount: summary.MaterialOptionCount,
		NPCVnums:            append([]uint32(nil), summary.NPCVnums...),
		Replaced:            replace,
	}
	if result.NPCVnums == nil {
		result.NPCVnums = []uint32{}
	}

	tx, err := executor.BeginTx(ctx, nil)
	if err != nil {
		return CubeRecipeStateImportResult{}, fmt.Errorf("begin cube-recipe-state import transaction: %w", err)
	}

	if err := requireCubeRecipeStateSchema(ctx, tx); err != nil {
		return CubeRecipeStateImportResult{}, rollbackAfterCubeRecipeStateImportFailure(tx, err)
	}

	if replace {
		for _, npcVnum := range summary.NPCVnums {
			if err := deleteCubeRecipeStateForNPC(ctx, tx, npcVnum); err != nil {
				return CubeRecipeStateImportResult{}, rollbackAfterCubeRecipeStateImportFailure(tx, err)
			}
		}
	}

	for _, row := range canonical.NPCs {
		if err := insertCubeRecipeNPC(ctx, tx, row); err != nil {
			return CubeRecipeStateImportResult{}, rollbackAfterCubeRecipeStateImportFailure(tx, err)
		}
	}
	for _, row := range canonical.Recipes {
		if err := insertCubeRecipe(ctx, tx, row); err != nil {
			return CubeRecipeStateImportResult{}, rollbackAfterCubeRecipeStateImportFailure(tx, err)
		}
	}
	for _, row := range canonical.Materials {
		if err := insertCubeRecipeMaterial(ctx, tx, row); err != nil {
			return CubeRecipeStateImportResult{}, rollbackAfterCubeRecipeStateImportFailure(tx, err)
		}
	}
	for _, row := range canonical.MaterialOptions {
		if err := insertCubeRecipeMaterialOption(ctx, tx, row); err != nil {
			return CubeRecipeStateImportResult{}, rollbackAfterCubeRecipeStateImportFailure(tx, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return CubeRecipeStateImportResult{}, fmt.Errorf("commit cube-recipe-state import transaction: %w", err)
	}
	return result, nil
}

func requireCubeRecipeStateSchema(ctx context.Context, querier dbmigrations.SQLLedgerQuerier) error {
	ledger, err := dbmigrations.ReadSQLLedgerEntries(ctx, querier)
	if err != nil {
		return fmt.Errorf("%w: read schema_migrations: %v", ErrCubeRecipeStateImportSchemaRequired, err)
	}
	for _, entry := range ledger {
		if entry.Version == CubeRecipeStateMigrationVersion && entry.Name == CubeRecipeStateMigrationName {
			return nil
		}
	}
	latest := 0
	for _, entry := range ledger {
		if entry.Version > latest {
			latest = entry.Version
		}
	}
	return fmt.Errorf("%w: ledger tip %d missing version %d %q", ErrCubeRecipeStateImportSchemaRequired, latest, CubeRecipeStateMigrationVersion, CubeRecipeStateMigrationName)
}

func deleteCubeRecipeStateForNPC(ctx context.Context, tx *sql.Tx, npcVnum uint32) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM cube_recipe_material_options WHERE npc_vnum = ?`, int64(npcVnum)); err != nil {
		return fmt.Errorf("delete cube recipe material options npc_vnum %d: %w", npcVnum, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM cube_recipe_materials WHERE npc_vnum = ?`, int64(npcVnum)); err != nil {
		return fmt.Errorf("delete cube recipe materials npc_vnum %d: %w", npcVnum, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM cube_recipes WHERE npc_vnum = ?`, int64(npcVnum)); err != nil {
		return fmt.Errorf("delete cube recipes npc_vnum %d: %w", npcVnum, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM cube_recipe_npcs WHERE npc_vnum = ?`, int64(npcVnum)); err != nil {
		return fmt.Errorf("delete cube recipe npc npc_vnum %d: %w", npcVnum, err)
	}
	return nil
}

func insertCubeRecipeNPC(ctx context.Context, tx *sql.Tx, row CubeRecipeNPCRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO cube_recipe_npcs (
    npc_vnum
) VALUES (?)`, int64(row.NPCVnum))
	if err != nil {
		return fmt.Errorf("insert cube recipe npc %d: %w", row.NPCVnum, err)
	}
	return requireExactCubeRecipeStateImportRows(result, "insert cube recipe npc", int64(row.NPCVnum))
}

func insertCubeRecipe(ctx context.Context, tx *sql.Tx, row CubeRecipeRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO cube_recipes (
    npc_vnum, position, reward_vnum, reward_count, gold, percent
) VALUES (?, ?, ?, ?, ?, ?)`,
		int64(row.NPCVnum), row.Position, int64(row.RewardVnum), int(row.RewardCount), int64(row.Gold), int(row.Percent),
	)
	if err != nil {
		return fmt.Errorf("insert cube recipe npc %d position %d: %w", row.NPCVnum, row.Position, err)
	}
	return requireExactCubeRecipeStateImportRows(result, "insert cube recipe", int64(row.NPCVnum)*1000+int64(row.Position))
}

func insertCubeRecipeMaterial(ctx context.Context, tx *sql.Tx, row CubeRecipeMaterialRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO cube_recipe_materials (
    npc_vnum, recipe_position, material_position, item_vnum, count
) VALUES (?, ?, ?, ?, ?)`,
		int64(row.NPCVnum), row.RecipePosition, row.MaterialPosition, int64(row.ItemVnum), int(row.Count),
	)
	if err != nil {
		return fmt.Errorf("insert cube recipe material npc %d recipe_position %d material_position %d: %w", row.NPCVnum, row.RecipePosition, row.MaterialPosition, err)
	}
	return requireExactCubeRecipeStateImportRows(result, "insert cube recipe material", int64(row.NPCVnum)*100000+int64(row.RecipePosition)*100+int64(row.MaterialPosition))
}

func insertCubeRecipeMaterialOption(ctx context.Context, tx *sql.Tx, row CubeRecipeMaterialOptionRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO cube_recipe_material_options (
    npc_vnum, recipe_position, option_index, material_position, item_vnum, count
) VALUES (?, ?, ?, ?, ?, ?)`,
		int64(row.NPCVnum), row.RecipePosition, row.OptionIndex, row.MaterialPosition, int64(row.ItemVnum), int(row.Count),
	)
	if err != nil {
		return fmt.Errorf("insert cube recipe material option npc %d recipe_position %d option_index %d material_position %d: %w", row.NPCVnum, row.RecipePosition, row.OptionIndex, row.MaterialPosition, err)
	}
	return requireExactCubeRecipeStateImportRows(result, "insert cube recipe material option", int64(row.NPCVnum)*1000000+int64(row.RecipePosition)*10000+int64(row.OptionIndex)*100+int64(row.MaterialPosition))
}

func requireExactCubeRecipeStateImportRows(result sql.Result, action string, id int64) error {
	if result == nil {
		return fmt.Errorf("%w: %s id %d returned nil result", ErrCubeRecipeStateImportRowCount, action, id)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: %s id %d returned unknown row count: %v", ErrCubeRecipeStateImportRowCount, action, id, err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("%w: %s id %d affected %d rows", ErrCubeRecipeStateImportRowCount, action, id, rowsAffected)
	}
	return nil
}

func rollbackAfterCubeRecipeStateImportFailure(tx *sql.Tx, importErr error) error {
	if tx == nil {
		return importErr
	}
	if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
		return errors.Join(importErr, fmt.Errorf("rollback cube-recipe-state import transaction: %w", rollbackErr))
	}
	return importErr
}

func cubeRecipeStateImportExecutorIsNil(executor dbmigrations.SQLMigrationExecutor) bool {
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
