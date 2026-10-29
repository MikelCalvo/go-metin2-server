//go:build sqlite_harness

package cubestore

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

func TestSQLiteHarnessSQLStoreWholeSnapshotRoundTrip(t *testing.T) {
	db := openSQLiteCubeRecipeStateImportDB(t)
	defer db.Close()
	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CubeRecipeStateMigrationVersion); err != nil {
		t.Fatal(err)
	}
	file := NewFileStore(filepath.Join(t.TempDir(), "cube-recipes.json"))
	if err := file.Save(BootstrapSnapshot()); err != nil {
		t.Fatal(err)
	}
	store := NewSQLStore(db)
	if got := SelectRematerializeStore(file, nil); got != file {
		t.Fatal("stock store changed")
	}
	if got := SelectRematerializeStore(file, store); got != store {
		t.Fatal("opt-in store not selected")
	}
	original := Snapshot{NPCs: []NPCRecipes{
		{NPCVnum: 20084, Recipes: []Recipe{}},
		{NPCVnum: BootstrapDefaultNPCVnum, Recipes: []Recipe{orMaterialPotionRecipe(), {Reward: Reward{Vnum: 11200, Count: 1}, Materials: []Material{}, Percent: 0}}},
	}}
	if err := store.Save(original); err != nil {
		t.Fatal(err)
	}
	loaded, err := NewSQLStore(db).Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := NormalizeSnapshot(original); !reflect.DeepEqual(loaded, want) {
		t.Fatalf("SQL roundtrip = %+v, want %+v", loaded, want)
	}
	if fileSnapshot, err := file.Load(); err != nil || !reflect.DeepEqual(fileSnapshot, NormalizeSnapshot(BootstrapSnapshot())) {
		t.Fatalf("stock FileStore changed: %+v, %v", fileSnapshot, err)
	}
	if err := store.Save(Snapshot{NPCs: []NPCRecipes{{NPCVnum: 20084, Recipes: []Recipe{}}}}); err != nil {
		t.Fatal(err)
	}
	loaded, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, NormalizeSnapshot(Snapshot{NPCs: []NPCRecipes{{NPCVnum: 20084, Recipes: []Recipe{}}}})) {
		t.Fatalf("replace did not clear omitted recipe: %+v", loaded)
	}
	if err := store.Save(Snapshot{}); err != nil {
		t.Fatal(err)
	}
	loaded, err = store.Load()
	if err != nil || len(loaded.NPCs) != 0 {
		t.Fatalf("empty replacement = %+v, %v", loaded, err)
	}
}

func TestSQLiteHarnessSQLStoreRejectsMissingSchemaAndCorruptRows(t *testing.T) {
	db := openSQLiteCubeRecipeStateImportDB(t)
	defer db.Close()
	store := NewSQLStore(db)
	if _, err := store.Load(); !errors.Is(err, ErrCubeRecipeStateImportSchemaRequired) {
		t.Fatalf("load without schema = %v", err)
	}
	if err := store.Save(BootstrapSnapshot()); !errors.Is(err, ErrCubeRecipeStateImportSchemaRequired) {
		t.Fatalf("save without schema = %v", err)
	}
	if _, err := dbmigrations.ApplyToVersion(context.Background(), db, nil, CubeRecipeStateMigrationVersion); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(BootstrapSnapshot()); err != nil {
		t.Fatal(err)
	}
	// SQL permits sparse positions but the authored list cannot represent them.
	if _, err := db.Exec(`INSERT INTO cube_recipes (npc_vnum, position, reward_vnum, reward_count, gold, percent) VALUES (?, 9, 11200, 1, 0, 0)`, BootstrapDefaultNPCVnum); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("sparse authored recipe positions accepted")
	}
	if err := store.Save(Snapshot{NPCs: []NPCRecipes{{NPCVnum: 20084, Recipes: []Recipe{{Reward: Reward{Vnum: 11200, Count: 1}, Gold: ^uint64(0), Materials: []Material{}}}}}}); err == nil {
		t.Fatal("overflow accepted")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM cube_recipe_npcs WHERE npc_vnum = ?`, BootstrapDefaultNPCVnum).Scan(&count); err != nil || count != 1 {
		t.Fatalf("failed save changed rows: %d, %v", count, err)
	}
}

func TestSQLiteHarnessSQLStoreRollbackOnInsertFailure(t *testing.T) {
	db := openSQLiteCubeRecipeStateImportDB(t)
	defer db.Close()
	if _, err := dbmigrations.ApplyToVersion(context.Background(), db, nil, CubeRecipeStateMigrationVersion); err != nil {
		t.Fatal(err)
	}
	store := NewSQLStore(db)
	if err := store.Save(BootstrapSnapshot()); err != nil {
		t.Fatal(err)
	}
	// Simulate an error after the delete, on the first child insert.
	if _, err := db.Exec(`CREATE TRIGGER block_new_recipe BEFORE INSERT ON cube_recipes BEGIN SELECT RAISE(ABORT, 'test insert refusal'); END`); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(Snapshot{NPCs: []NPCRecipes{{NPCVnum: 20084, Recipes: []Recipe{{Reward: Reward{Vnum: 11200, Count: 1}, Materials: []Material{}}}}}}); err == nil {
		t.Fatal("insert failure accepted")
	}
	got, err := store.Load()
	if err != nil || !reflect.DeepEqual(got, NormalizeSnapshot(BootstrapSnapshot())) {
		t.Fatalf("rollback lost prior snapshot: %+v, %v", got, err)
	}
}

func TestSQLiteHarnessSQLStoreRejectsCorruptMaterialOptionAndRetainsSavedRows(t *testing.T) {
	db := openSQLiteCubeRecipeStateImportDB(t)
	defer db.Close()
	if _, err := dbmigrations.ApplyToVersion(context.Background(), db, nil, CubeRecipeStateMigrationVersion); err != nil {
		t.Fatal(err)
	}
	store := NewSQLStore(db)
	snapshot := Snapshot{NPCs: []NPCRecipes{{NPCVnum: BootstrapDefaultNPCVnum, Recipes: []Recipe{orMaterialPotionRecipe()}}}}
	if err := store.Save(snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM cube_recipe_material_options WHERE npc_vnum = ? AND option_index = 1`, BootstrapDefaultNPCVnum); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidCubeRecipeStateExport) {
		t.Fatalf("incomplete OR-group load = %v", err)
	}
	if err := store.Save(snapshot); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Load(); err != nil || !reflect.DeepEqual(got, NormalizeSnapshot(snapshot)) {
		t.Fatalf("repaired roundtrip = %+v, %v", got, err)
	}
}

func TestSQLiteHarnessSQLStoreLoadsQuarantinedImport(t *testing.T) {
	db := openSQLiteCubeRecipeStateImportDB(t)
	defer db.Close()
	if _, err := dbmigrations.ApplyToVersion(context.Background(), db, nil, CubeRecipeStateMigrationVersion); err != nil {
		t.Fatal(err)
	}
	snapshot := Snapshot{NPCs: []NPCRecipes{{NPCVnum: BootstrapDefaultNPCVnum, Recipes: []Recipe{orMaterialPotionRecipe()}}}}
	export, err := ExportCubeRecipeState(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ImportCubeRecipeState(context.Background(), db, export); err != nil {
		t.Fatal(err)
	}
	got, err := NewSQLStore(db).Load()
	if err != nil || !reflect.DeepEqual(got, NormalizeSnapshot(snapshot)) {
		t.Fatalf("imported catalog did not load: %+v, %v", got, err)
	}
}
