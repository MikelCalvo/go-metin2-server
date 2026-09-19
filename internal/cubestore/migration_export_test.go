package cubestore

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExportCubeRecipeStateBuildsDeterministicRowsMatchingMigrationShape(t *testing.T) {
	snapshot := Snapshot{NPCs: []NPCRecipes{
		{
			NPCVnum: 20084,
			Recipes: []Recipe{{
				Reward:    Reward{Vnum: 11200, Count: 1},
				Materials: []Material{{Vnum: 27004, Count: 1}},
				Percent:   80,
			}},
		},
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
	}}

	export, err := ExportCubeRecipeState(snapshot)
	if err != nil {
		t.Fatalf("export cube recipe state: %v", err)
	}
	if export.MigrationVersion != CubeRecipeStateMigrationVersion || export.MigrationName != CubeRecipeStateMigrationName {
		t.Fatalf("unexpected migration boundary: version=%d name=%q", export.MigrationVersion, export.MigrationName)
	}
	if export.MigrationVersion != 31 || export.MigrationName != "cube_recipe_state" {
		t.Fatalf("expected cube-recipe-state export boundary, got version=%d name=%q", export.MigrationVersion, export.MigrationName)
	}

	wantNPCVnums := []uint32{BootstrapDefaultNPCVnum, 20084}
	if !reflect.DeepEqual(export.NPCVnums, wantNPCVnums) {
		t.Fatalf("unexpected npc vnums:\n got: %#v\nwant: %#v", export.NPCVnums, wantNPCVnums)
	}
	wantNPCs := []CubeRecipeNPCRow{{NPCVnum: BootstrapDefaultNPCVnum}, {NPCVnum: 20084}}
	if !reflect.DeepEqual(export.NPCs, wantNPCs) {
		t.Fatalf("unexpected npc rows:\n got: %#v\nwant: %#v", export.NPCs, wantNPCs)
	}
	wantRecipes := []CubeRecipeRow{
		{NPCVnum: BootstrapDefaultNPCVnum, Position: 0, RewardVnum: 27001, RewardCount: 1, Gold: 100, Percent: 100},
		{NPCVnum: BootstrapDefaultNPCVnum, Position: 1, RewardVnum: 11200, RewardCount: 1, Percent: 0},
		{NPCVnum: 20084, Position: 0, RewardVnum: 11200, RewardCount: 1, Percent: 80},
	}
	if !reflect.DeepEqual(export.Recipes, wantRecipes) {
		t.Fatalf("unexpected recipe rows:\n got: %#v\nwant: %#v", export.Recipes, wantRecipes)
	}
	wantMaterials := []CubeRecipeMaterialRow{
		{NPCVnum: BootstrapDefaultNPCVnum, RecipePosition: 0, MaterialPosition: 0, ItemVnum: 27002, Count: 2},
		{NPCVnum: 20084, RecipePosition: 0, MaterialPosition: 0, ItemVnum: 27004, Count: 1},
	}
	if !reflect.DeepEqual(export.Materials, wantMaterials) {
		t.Fatalf("unexpected material rows:\n got: %#v\nwant: %#v", export.Materials, wantMaterials)
	}
	wantOptions := []CubeRecipeMaterialOptionRow{
		{NPCVnum: BootstrapDefaultNPCVnum, RecipePosition: 0, OptionIndex: 0, MaterialPosition: 0, ItemVnum: 27002, Count: 2},
		{NPCVnum: BootstrapDefaultNPCVnum, RecipePosition: 0, OptionIndex: 1, MaterialPosition: 0, ItemVnum: 27003, Count: 1},
	}
	if !reflect.DeepEqual(export.MaterialOptions, wantOptions) {
		t.Fatalf("unexpected material option rows:\n got: %#v\nwant: %#v", export.MaterialOptions, wantOptions)
	}

	exportAgain, err := ExportCubeRecipeState(snapshot)
	if err != nil {
		t.Fatalf("export cube recipe state again: %v", err)
	}
	if !reflect.DeepEqual(export, exportAgain) {
		t.Fatalf("expected deterministic cube-recipe-state export:\n first: %#v\nsecond: %#v", export, exportAgain)
	}
}

func TestExportCubeRecipeStateRejectsMalformedSnapshot(t *testing.T) {
	_, err := ExportCubeRecipeState(Snapshot{NPCs: []NPCRecipes{{
		NPCVnum: 0,
		Recipes: []Recipe{{Reward: Reward{Vnum: 27001, Count: 1}, Materials: []Material{}, Percent: 100}},
	}}})
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for zero npc_vnum, got %v", err)
	}
}

func TestFileStoreExportCubeRecipeStateReadsCommittedSnapshot(t *testing.T) {
	restore := DisableDurableSyncForTest()
	defer restore()

	path := filepath.Join(t.TempDir(), "state", "cube-recipes.json")
	store := NewFileStore(path)
	if err := store.Save(sampleAuthoredCubeSnapshot()); err != nil {
		t.Fatalf("save cube recipes: %v", err)
	}

	export, err := store.ExportCubeRecipeState()
	if err != nil {
		t.Fatalf("file-store export cube recipe state: %v", err)
	}
	want, err := ExportCubeRecipeState(sampleAuthoredCubeSnapshot())
	if err != nil {
		t.Fatalf("direct export cube recipe state: %v", err)
	}
	if !reflect.DeepEqual(export, want) {
		t.Fatalf("unexpected file-store export:\n got: %#v\nwant: %#v", export, want)
	}
}

func TestFileStoreExportCubeRecipeStateTreatsMissingSnapshotAsEmpty(t *testing.T) {
	restore := DisableDurableSyncForTest()
	defer restore()

	export, err := NewFileStore(filepath.Join(t.TempDir(), "missing", "cube-recipes.json")).ExportCubeRecipeState()
	if err != nil {
		t.Fatalf("missing snapshot export: %v", err)
	}
	if export.MigrationVersion != CubeRecipeStateMigrationVersion || len(export.NPCs) != 0 || len(export.Recipes) != 0 {
		t.Fatalf("expected empty cube-recipe-state export, got %#v", export)
	}
}

func TestQuarantineCubeRecipeStateExportMergesDeclaredNPCVnums(t *testing.T) {
	export := CubeRecipeStateExport{
		MigrationVersion: CubeRecipeStateMigrationVersion,
		MigrationName:    CubeRecipeStateMigrationName,
		NPCVnums:         []uint32{20022},
		NPCs:             []CubeRecipeNPCRow{},
		Recipes:          []CubeRecipeRow{},
		Materials:        []CubeRecipeMaterialRow{},
		MaterialOptions:  []CubeRecipeMaterialOptionRow{},
	}
	canonical, summary, err := QuarantineCubeRecipeStateExport(export)
	if err != nil {
		t.Fatalf("quarantine declared wipe export: %v", err)
	}
	if summary.NPCCount != 0 || summary.RecipeCount != 0 || summary.MaterialCount != 0 || summary.MaterialOptionCount != 0 {
		t.Fatalf("declared wipe should keep zero recipe counts: %#v", summary)
	}
	if len(summary.NPCVnums) != 1 || summary.NPCVnums[0] != 20022 {
		t.Fatalf("unexpected declared wipe summary npc vnums: %#v", summary)
	}
	if len(canonical.NPCVnums) != 1 || canonical.NPCVnums[0] != 20022 {
		t.Fatalf("unexpected canonical npc vnums: %#v", canonical.NPCVnums)
	}
}

func TestQuarantineCubeRecipeStateExportRejectsInvalidDeclaredNPCVnums(t *testing.T) {
	base := CubeRecipeStateExport{
		MigrationVersion: CubeRecipeStateMigrationVersion,
		MigrationName:    CubeRecipeStateMigrationName,
		NPCs:             []CubeRecipeNPCRow{},
		Recipes:          []CubeRecipeRow{},
		Materials:        []CubeRecipeMaterialRow{},
		MaterialOptions:  []CubeRecipeMaterialOptionRow{},
	}

	zeroVnum := base
	zeroVnum.NPCVnums = []uint32{0}
	if _, _, err := QuarantineCubeRecipeStateExport(zeroVnum); err == nil || !errors.Is(err, ErrInvalidCubeRecipeStateExport) {
		t.Fatalf("zero npc_vnums error = %v, want invalid export", err)
	}

	duplicate := base
	duplicate.NPCVnums = []uint32{20022, 20022}
	if _, _, err := QuarantineCubeRecipeStateExport(duplicate); err == nil || !errors.Is(err, ErrInvalidCubeRecipeStateExport) {
		t.Fatalf("duplicate npc_vnums error = %v, want invalid export", err)
	}
}

func TestQuarantineCubeRecipeStateExportRejectsOrphanMaterials(t *testing.T) {
	export := CubeRecipeStateExport{
		MigrationVersion: CubeRecipeStateMigrationVersion,
		MigrationName:    CubeRecipeStateMigrationName,
		NPCs:             []CubeRecipeNPCRow{{NPCVnum: 20022}},
		Recipes:          []CubeRecipeRow{},
		Materials:        []CubeRecipeMaterialRow{{NPCVnum: 20022, RecipePosition: 0, MaterialPosition: 0, ItemVnum: 27002, Count: 1}},
		MaterialOptions:  []CubeRecipeMaterialOptionRow{},
	}
	if _, _, err := QuarantineCubeRecipeStateExport(export); err == nil || !errors.Is(err, ErrInvalidCubeRecipeStateExport) {
		t.Fatalf("orphan material error = %v, want invalid export", err)
	}
}

func TestImportCubeRecipeStateRejectsNilExecutor(t *testing.T) {
	export, err := ExportCubeRecipeState(BootstrapSnapshot())
	if err != nil {
		t.Fatalf("export sample cube recipe state: %v", err)
	}
	_, err = ImportCubeRecipeState(context.Background(), nil, export)
	if !errors.Is(err, ErrCubeRecipeStateImportExecutorRequired) {
		t.Fatalf("ImportCubeRecipeState(nil) error = %v, want %v", err, ErrCubeRecipeStateImportExecutorRequired)
	}
}

func TestImportCubeRecipeStateRejectsInvalidExportBeforeOpeningTransaction(t *testing.T) {
	export := CubeRecipeStateExport{
		MigrationVersion: 99,
		MigrationName:    "not-cube-recipe-state",
		NPCs:             []CubeRecipeNPCRow{},
		Recipes:          []CubeRecipeRow{},
		Materials:        []CubeRecipeMaterialRow{},
		MaterialOptions:  []CubeRecipeMaterialOptionRow{},
	}
	_, err := ImportCubeRecipeState(context.Background(), failingCubeRecipeStateImportExecutor{}, export)
	if !errors.Is(err, ErrInvalidCubeRecipeStateExport) {
		t.Fatalf("ImportCubeRecipeState(invalid) error = %v, want %v", err, ErrInvalidCubeRecipeStateExport)
	}
}

func TestImportCubeRecipeStateRejectsTooManyOptions(t *testing.T) {
	export := CubeRecipeStateExport{
		MigrationVersion: CubeRecipeStateMigrationVersion,
		MigrationName:    CubeRecipeStateMigrationName,
		NPCs:             []CubeRecipeNPCRow{},
		Recipes:          []CubeRecipeRow{},
		Materials:        []CubeRecipeMaterialRow{},
		MaterialOptions:  []CubeRecipeMaterialOptionRow{},
	}
	_, err := ImportCubeRecipeState(
		context.Background(),
		failingCubeRecipeStateImportExecutor{},
		export,
		ImportCubeRecipeStateOptions{Replace: true},
		ImportCubeRecipeStateOptions{},
	)
	if err == nil || !strings.Contains(err.Error(), "at most one options") {
		t.Fatalf("ImportCubeRecipeState(too many options) error = %v, want at most one options", err)
	}
}

type failingCubeRecipeStateImportExecutor struct{}

func (failingCubeRecipeStateImportExecutor) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	panic("BeginTx must not be reached for invalid cube-recipe-state exports")
}
