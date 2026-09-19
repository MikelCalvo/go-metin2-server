//go:build sqlite_harness

package cubestore

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

func TestSQLiteHarnessCubeRecipeStateImportInsertsCanonicalRows(t *testing.T) {
	db := openSQLiteCubeRecipeStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CubeRecipeStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CubeRecipeStateMigrationVersion, err)
	}

	snapshot := Snapshot{NPCs: []NPCRecipes{
		{
			NPCVnum: BootstrapDefaultNPCVnum,
			Recipes: []Recipe{
				orMaterialPotionRecipe(),
				{
					Reward:    Reward{Vnum: 11200, Count: 1},
					Materials: []Material{},
					Percent:   0,
				},
			},
		},
		{
			NPCVnum: 20084,
			Recipes: []Recipe{{
				Reward:    Reward{Vnum: 11200, Count: 1},
				Materials: []Material{{Vnum: 27004, Count: 1}},
				Gold:      50,
				Percent:   80,
			}},
		},
	}}
	export, err := ExportCubeRecipeState(snapshot)
	if err != nil {
		t.Fatalf("ExportCubeRecipeState: %v", err)
	}

	result, err := ImportCubeRecipeState(ctx, db, export)
	if err != nil {
		t.Fatalf("ImportCubeRecipeState: %v", err)
	}
	if result.MigrationVersion != CubeRecipeStateMigrationVersion || result.MigrationName != CubeRecipeStateMigrationName {
		t.Fatalf("unexpected migration boundary in result: %+v", result)
	}
	if result.NPCCount != 2 || result.RecipeCount != 3 || result.MaterialCount != 2 || result.MaterialOptionCount != 2 {
		t.Fatalf("unexpected import counts: %+v", result)
	}
	if len(result.NPCVnums) != 2 || result.NPCVnums[0] != BootstrapDefaultNPCVnum || result.NPCVnums[1] != 20084 {
		t.Fatalf("unexpected npc vnums: %+v", result.NPCVnums)
	}

	assertCubeRecipeNPC(t, db, BootstrapDefaultNPCVnum)
	assertCubeRecipeNPC(t, db, 20084)
	assertCubeRecipe(t, db, BootstrapDefaultNPCVnum, 0, 27001, 1, 100, 100)
	assertCubeRecipe(t, db, BootstrapDefaultNPCVnum, 1, 11200, 1, 0, 0)
	assertCubeRecipe(t, db, 20084, 0, 11200, 1, 50, 80)
	assertCubeRecipeMaterial(t, db, BootstrapDefaultNPCVnum, 0, 0, 27002, 2)
	assertCubeRecipeMaterial(t, db, 20084, 0, 0, 27004, 1)
	assertCubeRecipeMaterialOption(t, db, BootstrapDefaultNPCVnum, 0, 0, 0, 27002, 2)
	assertCubeRecipeMaterialOption(t, db, BootstrapDefaultNPCVnum, 0, 1, 0, 27003, 1)
}

func TestSQLiteHarnessCubeRecipeStateImportRejectsDuplicatePrimaryKey(t *testing.T) {
	db := openSQLiteCubeRecipeStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CubeRecipeStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CubeRecipeStateMigrationVersion, err)
	}

	export, err := ExportCubeRecipeState(BootstrapSnapshot())
	if err != nil {
		t.Fatalf("ExportCubeRecipeState: %v", err)
	}
	if _, err := ImportCubeRecipeState(ctx, db, export); err != nil {
		t.Fatalf("first ImportCubeRecipeState: %v", err)
	}
	if _, err := ImportCubeRecipeState(ctx, db, export); err == nil {
		t.Fatal("second ImportCubeRecipeState succeeded, want unique conflict")
	}

	var npcRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cube_recipe_npcs`).Scan(&npcRows); err != nil {
		t.Fatalf("count npcs after failed reimport: %v", err)
	}
	if npcRows != 1 {
		t.Fatalf("npc rows after failed reimport = %d, want 1", npcRows)
	}
}

func TestSQLiteHarnessCubeRecipeStateImportRejectsMissingSchema(t *testing.T) {
	db := openSQLiteCubeRecipeStateImportDB(t)
	defer db.Close()

	export := CubeRecipeStateExport{
		MigrationVersion: CubeRecipeStateMigrationVersion,
		MigrationName:    CubeRecipeStateMigrationName,
		NPCs:             []CubeRecipeNPCRow{},
		Recipes:          []CubeRecipeRow{},
		Materials:        []CubeRecipeMaterialRow{},
		MaterialOptions:  []CubeRecipeMaterialOptionRow{},
	}
	_, err := ImportCubeRecipeState(context.Background(), db, export)
	if !errors.Is(err, ErrCubeRecipeStateImportSchemaRequired) {
		t.Fatalf("ImportCubeRecipeState on empty DB error = %v, want %v", err, ErrCubeRecipeStateImportSchemaRequired)
	}
}

func TestSQLiteHarnessCubeRecipeStateImportReplaceOverwritesCanonicalRows(t *testing.T) {
	db := openSQLiteCubeRecipeStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CubeRecipeStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CubeRecipeStateMigrationVersion, err)
	}

	first, err := ExportCubeRecipeState(BootstrapSnapshot())
	if err != nil {
		t.Fatalf("ExportCubeRecipeState: %v", err)
	}
	if _, err := ImportCubeRecipeState(ctx, db, first); err != nil {
		t.Fatalf("first insert-only ImportCubeRecipeState: %v", err)
	}

	replaced, err := ExportCubeRecipeState(Snapshot{NPCs: []NPCRecipes{{
		NPCVnum: BootstrapDefaultNPCVnum,
		Recipes: []Recipe{{
			Reward:    Reward{Vnum: 11200, Count: 1},
			Materials: []Material{{Vnum: 27004, Count: 1}},
			Gold:      25,
			Percent:   80,
		}},
	}}})
	if err != nil {
		t.Fatalf("ExportCubeRecipeState(replaced): %v", err)
	}
	result, err := ImportCubeRecipeState(ctx, db, replaced, ImportCubeRecipeStateOptions{Replace: true})
	if err != nil {
		t.Fatalf("replace ImportCubeRecipeState: %v", err)
	}
	if !result.Replaced {
		t.Fatalf("replace result.Replaced = false, want true")
	}
	if result.NPCCount != 1 || result.RecipeCount != 1 || result.MaterialCount != 1 || result.MaterialOptionCount != 0 {
		t.Fatalf("unexpected replace counts: %+v", result)
	}

	var recipeRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cube_recipes`).Scan(&recipeRows); err != nil {
		t.Fatalf("count recipes after replace: %v", err)
	}
	if recipeRows != 1 {
		t.Fatalf("recipe rows after replace = %d, want 1", recipeRows)
	}
	assertCubeRecipe(t, db, BootstrapDefaultNPCVnum, 0, 11200, 1, 25, 80)
	assertCubeRecipeMaterial(t, db, BootstrapDefaultNPCVnum, 0, 0, 27004, 1)
}

func TestSQLiteHarnessCubeRecipeStateImportReplaceLeavesUnlistedNPCsUntouched(t *testing.T) {
	db := openSQLiteCubeRecipeStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CubeRecipeStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CubeRecipeStateMigrationVersion, err)
	}

	seed, err := ExportCubeRecipeState(Snapshot{NPCs: []NPCRecipes{
		{
			NPCVnum: BootstrapDefaultNPCVnum,
			Recipes: []Recipe{{
				Reward:    Reward{Vnum: 27001, Count: 1},
				Materials: []Material{{Vnum: 27002, Count: 2}},
				Gold:      100,
				Percent:   100,
			}},
		},
		{
			NPCVnum: 20084,
			Recipes: []Recipe{{
				Reward:    Reward{Vnum: 11200, Count: 1},
				Materials: []Material{{Vnum: 27004, Count: 1}},
				Percent:   80,
			}},
		},
	}})
	if err != nil {
		t.Fatalf("ExportCubeRecipeState: %v", err)
	}
	if _, err := ImportCubeRecipeState(ctx, db, seed); err != nil {
		t.Fatalf("seed ImportCubeRecipeState: %v", err)
	}

	replaced, err := ExportCubeRecipeState(Snapshot{NPCs: []NPCRecipes{{
		NPCVnum: BootstrapDefaultNPCVnum,
		Recipes: []Recipe{{
			Reward:    Reward{Vnum: 27001, Count: 2},
			Materials: []Material{{Vnum: 27002, Count: 1}},
			Gold:      10,
			Percent:   50,
		}},
	}}})
	if err != nil {
		t.Fatalf("ExportCubeRecipeState(replaced): %v", err)
	}
	if _, err := ImportCubeRecipeState(ctx, db, replaced, ImportCubeRecipeStateOptions{Replace: true}); err != nil {
		t.Fatalf("replace ImportCubeRecipeState: %v", err)
	}

	assertCubeRecipe(t, db, BootstrapDefaultNPCVnum, 0, 27001, 2, 10, 50)
	assertCubeRecipeMaterial(t, db, BootstrapDefaultNPCVnum, 0, 0, 27002, 1)
	assertCubeRecipe(t, db, 20084, 0, 11200, 1, 0, 80)
	assertCubeRecipeMaterial(t, db, 20084, 0, 0, 27004, 1)
}

func TestSQLiteHarnessCubeRecipeStateImportReplaceWipesDeclaredNPCWithoutRows(t *testing.T) {
	db := openSQLiteCubeRecipeStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CubeRecipeStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CubeRecipeStateMigrationVersion, err)
	}

	seed, err := ExportCubeRecipeState(BootstrapSnapshot())
	if err != nil {
		t.Fatalf("ExportCubeRecipeState: %v", err)
	}
	if _, err := ImportCubeRecipeState(ctx, db, seed); err != nil {
		t.Fatalf("seed ImportCubeRecipeState: %v", err)
	}

	emptyWipe := CubeRecipeStateExport{
		MigrationVersion: CubeRecipeStateMigrationVersion,
		MigrationName:    CubeRecipeStateMigrationName,
		NPCVnums:         []uint32{BootstrapDefaultNPCVnum},
		NPCs:             []CubeRecipeNPCRow{},
		Recipes:          []CubeRecipeRow{},
		Materials:        []CubeRecipeMaterialRow{},
		MaterialOptions:  []CubeRecipeMaterialOptionRow{},
	}
	result, err := ImportCubeRecipeState(ctx, db, emptyWipe, ImportCubeRecipeStateOptions{Replace: true})
	if err != nil {
		t.Fatalf("empty wipe replace: %v", err)
	}
	if !result.Replaced || result.NPCCount != 0 || result.RecipeCount != 0 || len(result.NPCVnums) != 1 || result.NPCVnums[0] != BootstrapDefaultNPCVnum {
		t.Fatalf("unexpected empty wipe result: %+v", result)
	}

	var npcRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cube_recipe_npcs`).Scan(&npcRows); err != nil {
		t.Fatalf("count npcs after wipe: %v", err)
	}
	if npcRows != 0 {
		t.Fatalf("npc rows after empty wipe = %d, want 0", npcRows)
	}
}

func assertCubeRecipeNPC(t *testing.T, db *sql.DB, npcVnum uint32) {
	t.Helper()
	var got int64
	if err := db.QueryRowContext(context.Background(), `SELECT npc_vnum FROM cube_recipe_npcs WHERE npc_vnum = ?`, npcVnum).Scan(&got); err != nil {
		t.Fatalf("select cube recipe npc %d: %v", npcVnum, err)
	}
	if got != int64(npcVnum) {
		t.Fatalf("cube recipe npc mismatch: got %d want %d", got, npcVnum)
	}
}

func assertCubeRecipe(t *testing.T, db *sql.DB, npcVnum uint32, position int, rewardVnum uint32, rewardCount uint16, gold uint64, percent uint8) {
	t.Helper()
	var gotNPC, gotReward, gotGold int64
	var gotPosition, gotCount, gotPercent int
	if err := db.QueryRowContext(context.Background(), `
SELECT npc_vnum, position, reward_vnum, reward_count, gold, percent
FROM cube_recipes WHERE npc_vnum = ? AND position = ?`,
		npcVnum, position).Scan(&gotNPC, &gotPosition, &gotReward, &gotCount, &gotGold, &gotPercent); err != nil {
		t.Fatalf("select cube recipe npc %d position %d: %v", npcVnum, position, err)
	}
	if gotNPC != int64(npcVnum) || gotPosition != position || gotReward != int64(rewardVnum) || gotCount != int(rewardCount) || gotGold != int64(gold) || gotPercent != int(percent) {
		t.Fatalf("cube recipe mismatch for npc %d position %d", npcVnum, position)
	}
}

func assertCubeRecipeMaterial(t *testing.T, db *sql.DB, npcVnum uint32, recipePosition, materialPosition int, itemVnum uint32, count uint16) {
	t.Helper()
	var gotNPC, gotItem int64
	var gotRecipePosition, gotMaterialPosition, gotCount int
	if err := db.QueryRowContext(context.Background(), `
SELECT npc_vnum, recipe_position, material_position, item_vnum, count
FROM cube_recipe_materials WHERE npc_vnum = ? AND recipe_position = ? AND material_position = ?`,
		npcVnum, recipePosition, materialPosition).Scan(&gotNPC, &gotRecipePosition, &gotMaterialPosition, &gotItem, &gotCount); err != nil {
		t.Fatalf("select cube recipe material npc %d recipe %d material %d: %v", npcVnum, recipePosition, materialPosition, err)
	}
	if gotNPC != int64(npcVnum) || gotRecipePosition != recipePosition || gotMaterialPosition != materialPosition || gotItem != int64(itemVnum) || gotCount != int(count) {
		t.Fatalf("cube recipe material mismatch for npc %d recipe %d material %d", npcVnum, recipePosition, materialPosition)
	}
}

func assertCubeRecipeMaterialOption(t *testing.T, db *sql.DB, npcVnum uint32, recipePosition, optionIndex, materialPosition int, itemVnum uint32, count uint16) {
	t.Helper()
	var gotNPC, gotItem int64
	var gotRecipePosition, gotOptionIndex, gotMaterialPosition, gotCount int
	if err := db.QueryRowContext(context.Background(), `
SELECT npc_vnum, recipe_position, option_index, material_position, item_vnum, count
FROM cube_recipe_material_options WHERE npc_vnum = ? AND recipe_position = ? AND option_index = ? AND material_position = ?`,
		npcVnum, recipePosition, optionIndex, materialPosition).Scan(&gotNPC, &gotRecipePosition, &gotOptionIndex, &gotMaterialPosition, &gotItem, &gotCount); err != nil {
		t.Fatalf("select cube recipe material option npc %d recipe %d option %d material %d: %v", npcVnum, recipePosition, optionIndex, materialPosition, err)
	}
	if gotNPC != int64(npcVnum) || gotRecipePosition != recipePosition || gotOptionIndex != optionIndex || gotMaterialPosition != materialPosition || gotItem != int64(itemVnum) || gotCount != int(count) {
		t.Fatalf("cube recipe material option mismatch for npc %d recipe %d option %d material %d", npcVnum, recipePosition, optionIndex, materialPosition)
	}
}

func openSQLiteCubeRecipeStateImportDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cube-recipe-state-import-harness.sqlite")
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open(sqlite): %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("Ping sqlite cube-recipe-state import harness: %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		t.Fatalf("enable foreign_keys: %v", err)
	}
	return db
}
