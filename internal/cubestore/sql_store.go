package cubestore

import (
	"context"
	"database/sql"
	"fmt"
	"math"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

// SQLStore is an explicit opt-in Load/Save repository for the tip-0031 authored
// cube catalog. The caller supplies the executor; stock gamed still uses FileStore.
// Save replaces the entire catalog in one transaction, including empty saves;
// it does not merge with a FileStore or register a driver.
type SQLStore struct {
	executor dbmigrations.SQLMigrationExecutor
}

var _ Store = (*SQLStore)(nil)

func NewSQLStore(executor dbmigrations.SQLMigrationExecutor) *SQLStore {
	return &SQLStore{executor: executor}
}

// SelectRematerializeStore leaves stock FileStore selection unchanged unless
// the caller explicitly supplies a SQLStore.
func SelectRematerializeStore(fileStore Store, sqlStore *SQLStore) Store {
	if sqlStore != nil {
		return sqlStore
	}
	return fileStore
}

func (s *SQLStore) Load() (Snapshot, error) {
	if s == nil || cubeRecipeStateImportExecutorIsNil(s.executor) {
		return Snapshot{}, ErrCubeRecipeStateImportExecutorRequired
	}
	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Snapshot{}, fmt.Errorf("begin cube recipe SQL load: %w", err)
	}
	export, err := queryCubeRecipeState(ctx, tx)
	if err != nil {
		return Snapshot{}, rollbackAfterCubeRecipeStateImportFailure(tx, err)
	}
	snapshot, err := snapshotFromCubeRecipeState(export)
	if err != nil {
		return Snapshot{}, rollbackAfterCubeRecipeStateImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return Snapshot{}, fmt.Errorf("commit cube recipe SQL load: %w", err)
	}
	return snapshot, nil
}

func (s *SQLStore) Save(snapshot Snapshot) error {
	if s == nil || cubeRecipeStateImportExecutorIsNil(s.executor) {
		return ErrCubeRecipeStateImportExecutorRequired
	}
	export, err := ExportCubeRecipeState(snapshot)
	if err != nil {
		return err
	}
	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cube recipe SQL save: %w", err)
	}
	if err := requireCubeRecipeStateSchema(ctx, tx); err != nil {
		return rollbackAfterCubeRecipeStateImportFailure(tx, err)
	}
	// No cascade policy in 0031. Delete children first, then parents, even for
	// an empty snapshot. All deletes and inserts commit or roll back together.
	for _, table := range []string{"cube_recipe_material_options", "cube_recipe_materials", "cube_recipes", "cube_recipe_npcs"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return rollbackAfterCubeRecipeStateImportFailure(tx, fmt.Errorf("delete %s: %w", table, err))
		}
	}
	for _, row := range export.NPCs {
		if err := insertCubeRecipeNPC(ctx, tx, row); err != nil {
			return rollbackAfterCubeRecipeStateImportFailure(tx, err)
		}
	}
	for _, row := range export.Recipes {
		if err := insertCubeRecipe(ctx, tx, row); err != nil {
			return rollbackAfterCubeRecipeStateImportFailure(tx, err)
		}
	}
	for _, row := range export.Materials {
		if err := insertCubeRecipeMaterial(ctx, tx, row); err != nil {
			return rollbackAfterCubeRecipeStateImportFailure(tx, err)
		}
	}
	for _, row := range export.MaterialOptions {
		if err := insertCubeRecipeMaterialOption(ctx, tx, row); err != nil {
			return rollbackAfterCubeRecipeStateImportFailure(tx, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cube recipe SQL save: %w", err)
	}
	return nil
}

// queryCubeRecipeState reads a single consistent SQL snapshot. Scan into signed
// carriers first; never cast corrupt/overflowing SQL values into authored types.
func queryCubeRecipeState(ctx context.Context, tx *sql.Tx) (CubeRecipeStateExport, error) {
	if err := requireCubeRecipeStateSchema(ctx, tx); err != nil {
		return CubeRecipeStateExport{}, err
	}
	out := CubeRecipeStateExport{MigrationVersion: CubeRecipeStateMigrationVersion, MigrationName: CubeRecipeStateMigrationName,
		NPCs: []CubeRecipeNPCRow{}, Recipes: []CubeRecipeRow{}, Materials: []CubeRecipeMaterialRow{}, MaterialOptions: []CubeRecipeMaterialOptionRow{}}
	npcs, err := tx.QueryContext(ctx, `SELECT npc_vnum FROM cube_recipe_npcs ORDER BY npc_vnum`)
	if err != nil {
		return out, fmt.Errorf("query cube recipe npcs: %w", err)
	}
	for npcs.Next() {
		var n int64
		if err = npcs.Scan(&n); err != nil {
			break
		}
		if n <= 0 || n > math.MaxUint32 {
			err = fmt.Errorf("%w: SQL npc_vnum out of range", ErrInvalidSnapshot)
			break
		}
		out.NPCs = append(out.NPCs, CubeRecipeNPCRow{NPCVnum: uint32(n)})
	}
	err = finishCubeRecipeRows(npcs, err)
	if err != nil {
		return out, fmt.Errorf("read cube recipe npcs: %w", err)
	}
	recipes, err := tx.QueryContext(ctx, `SELECT npc_vnum, position, reward_vnum, reward_count, gold, percent FROM cube_recipes ORDER BY npc_vnum, position`)
	if err != nil {
		return out, fmt.Errorf("query cube recipes: %w", err)
	}
	for recipes.Next() {
		var npc, pos, reward, count, gold, percent int64
		if err = recipes.Scan(&npc, &pos, &reward, &count, &gold, &percent); err != nil {
			break
		}
		if !validCubeSQLVnum(npc) || pos < 0 || !validCubeSQLVnum(reward) || count <= 0 || count > math.MaxUint16 || gold < 0 || percent < 0 || percent > 100 {
			err = fmt.Errorf("%w: SQL cube recipe value out of range", ErrInvalidSnapshot)
			break
		}
		out.Recipes = append(out.Recipes, CubeRecipeRow{NPCVnum: uint32(npc), Position: int(pos), RewardVnum: uint32(reward), RewardCount: uint16(count), Gold: uint64(gold), Percent: uint8(percent)})
	}
	err = finishCubeRecipeRows(recipes, err)
	if err != nil {
		return out, fmt.Errorf("read cube recipes: %w", err)
	}
	materials, err := tx.QueryContext(ctx, `SELECT npc_vnum, recipe_position, material_position, item_vnum, count FROM cube_recipe_materials ORDER BY npc_vnum, recipe_position, material_position`)
	if err != nil {
		return out, fmt.Errorf("query cube recipe materials: %w", err)
	}
	for materials.Next() {
		var npc, recipe, pos, item, count int64
		if err = materials.Scan(&npc, &recipe, &pos, &item, &count); err != nil {
			break
		}
		if !validCubeSQLVnum(npc) || recipe < 0 || pos < 0 || !validCubeSQLVnum(item) || count <= 0 || count > math.MaxUint16 {
			err = fmt.Errorf("%w: SQL cube material value out of range", ErrInvalidSnapshot)
			break
		}
		out.Materials = append(out.Materials, CubeRecipeMaterialRow{NPCVnum: uint32(npc), RecipePosition: int(recipe), MaterialPosition: int(pos), ItemVnum: uint32(item), Count: uint16(count)})
	}
	err = finishCubeRecipeRows(materials, err)
	if err != nil {
		return out, fmt.Errorf("read cube recipe materials: %w", err)
	}
	options, err := tx.QueryContext(ctx, `SELECT npc_vnum, recipe_position, option_index, material_position, item_vnum, count FROM cube_recipe_material_options ORDER BY npc_vnum, recipe_position, option_index, material_position`)
	if err != nil {
		return out, fmt.Errorf("query cube recipe material options: %w", err)
	}
	for options.Next() {
		var npc, recipe, option, pos, item, count int64
		if err = options.Scan(&npc, &recipe, &option, &pos, &item, &count); err != nil {
			break
		}
		if !validCubeSQLVnum(npc) || recipe < 0 || option < 0 || pos < 0 || !validCubeSQLVnum(item) || count <= 0 || count > math.MaxUint16 {
			err = fmt.Errorf("%w: SQL cube material option value out of range", ErrInvalidSnapshot)
			break
		}
		out.MaterialOptions = append(out.MaterialOptions, CubeRecipeMaterialOptionRow{NPCVnum: uint32(npc), RecipePosition: int(recipe), OptionIndex: int(option), MaterialPosition: int(pos), ItemVnum: uint32(item), Count: uint16(count)})
	}
	err = finishCubeRecipeRows(options, err)
	if err != nil {
		return out, fmt.Errorf("read cube recipe material options: %w", err)
	}
	return out, nil
}

func validCubeSQLVnum(n int64) bool { return n > 0 && n <= math.MaxUint32 }
func finishCubeRecipeRows(rows *sql.Rows, scanErr error) error {
	iterationErr := rows.Err()
	closeErr := rows.Close()
	if scanErr != nil {
		return scanErr
	}
	if iterationErr != nil {
		return iterationErr
	}
	return closeErr
}

func snapshotFromCubeRecipeState(export CubeRecipeStateExport) (Snapshot, error) {
	canonical, _, err := QuarantineCubeRecipeStateExport(export)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{NPCs: make([]NPCRecipes, 0, len(canonical.NPCs))}
	for _, npc := range canonical.NPCs {
		snapshot.NPCs = append(snapshot.NPCs, NPCRecipes{NPCVnum: npc.NPCVnum, Recipes: []Recipe{}})
	}
	for _, row := range canonical.Recipes {
		npc, err := cubeSQLNPC(&snapshot, row.NPCVnum)
		if err != nil {
			return Snapshot{}, err
		}
		if row.Position != len(npc.Recipes) {
			return Snapshot{}, fmt.Errorf("%w: sparse recipe position for npc %d", ErrInvalidSnapshot, row.NPCVnum)
		}
		npc.Recipes = append(npc.Recipes, Recipe{Reward: Reward{Vnum: row.RewardVnum, Count: row.RewardCount}, Materials: []Material{}, Gold: row.Gold, Percent: row.Percent})
	}
	for _, row := range canonical.Materials {
		recipe, err := cubeSQLRecipe(&snapshot, row.NPCVnum, row.RecipePosition)
		if err != nil {
			return Snapshot{}, err
		}
		if row.MaterialPosition != len(recipe.Materials) {
			return Snapshot{}, fmt.Errorf("%w: sparse material position", ErrInvalidSnapshot)
		}
		recipe.Materials = append(recipe.Materials, Material{Vnum: row.ItemVnum, Count: row.Count})
	}
	for _, row := range canonical.MaterialOptions {
		recipe, err := cubeSQLRecipe(&snapshot, row.NPCVnum, row.RecipePosition)
		if err != nil {
			return Snapshot{}, err
		}
		if row.OptionIndex == len(recipe.MaterialOptions) {
			recipe.MaterialOptions = append(recipe.MaterialOptions, []Material{})
		}
		if row.OptionIndex >= len(recipe.MaterialOptions) || row.MaterialPosition != len(recipe.MaterialOptions[row.OptionIndex]) {
			return Snapshot{}, fmt.Errorf("%w: sparse material option position", ErrInvalidSnapshot)
		}
		recipe.MaterialOptions[row.OptionIndex] = append(recipe.MaterialOptions[row.OptionIndex], Material{Vnum: row.ItemVnum, Count: row.Count})
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return Snapshot{}, err
	}
	// Export enforces SQL carrier bounds and represents every authored field.
	if _, err := ExportCubeRecipeState(normalized); err != nil {
		return Snapshot{}, err
	}
	return normalized, nil
}

func cubeSQLNPC(snapshot *Snapshot, vnum uint32) (*NPCRecipes, error) {
	for i := range snapshot.NPCs {
		if snapshot.NPCs[i].NPCVnum == vnum {
			return &snapshot.NPCs[i], nil
		}
	}
	return nil, fmt.Errorf("%w: orphan recipe npc %d", ErrInvalidSnapshot, vnum)
}
func cubeSQLRecipe(snapshot *Snapshot, vnum uint32, position int) (*Recipe, error) {
	npc, err := cubeSQLNPC(snapshot, vnum)
	if err != nil {
		return nil, err
	}
	if position < 0 || position >= len(npc.Recipes) {
		return nil, fmt.Errorf("%w: orphan recipe position", ErrInvalidSnapshot)
	}
	return &npc.Recipes[position], nil
}
