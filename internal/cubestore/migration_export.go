package cubestore

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

const (
	// CubeRecipeStateMigrationVersion / Name pin the first schema-only
	// cube-recipe SQL catalog companion beside already-owned cubestore FileStore.
	// This is a data-model / export identity only: it does not open a database,
	// emit SQL, apply migrations, or mutate authored cube-recipe snapshots.
	CubeRecipeStateMigrationVersion = 31
	CubeRecipeStateMigrationName    = "cube_recipe_state"
)

// ErrInvalidCubeRecipeStateExport reports that a retained cube-recipe-state
// export failed the 0031 migration-shaped contract.
var ErrInvalidCubeRecipeStateExport = errors.New("invalid cube recipe state export")

// CubeRecipeStateExport is a deterministic, schema-shaped projection of authored
// cube-recipe FileStore snapshots onto the 0031_cube_recipe_state migration
// boundary. It is intentionally a data-model/export contract only: it does not
// open a database, emit SQL, apply migrations, or mutate the file store.
type CubeRecipeStateExport struct {
	MigrationVersion int    `json:"migration_version"`
	MigrationName    string `json:"migration_name"`
	// NPCVnums optionally declares the replace/wipe NPC vnum scope for
	// ImportCubeRecipeState(..., Replace: true). When omitted or empty,
	// quarantine derives the scope from npc rows (legacy insert-only exports
	// stay valid). Explicit ids merge with npc-row-derived vnums so a listed
	// npc with zero recipe rows can still wipe-to-empty.
	NPCVnums        []uint32                      `json:"npc_vnums,omitempty"`
	NPCs            []CubeRecipeNPCRow            `json:"npcs"`
	Recipes         []CubeRecipeRow               `json:"recipes"`
	Materials       []CubeRecipeMaterialRow       `json:"materials"`
	MaterialOptions []CubeRecipeMaterialOptionRow `json:"material_options"`
}

// CubeRecipeNPCRow mirrors one cube_recipe_npcs parent row frozen by
// 0031_cube_recipe_state.
type CubeRecipeNPCRow struct {
	NPCVnum uint32 `json:"npc_vnum"`
}

// CubeRecipeRow mirrors one cube_recipes child row frozen by
// 0031_cube_recipe_state.
type CubeRecipeRow struct {
	NPCVnum     uint32 `json:"npc_vnum"`
	Position    int    `json:"position"`
	RewardVnum  uint32 `json:"reward_vnum"`
	RewardCount uint16 `json:"reward_count"`
	Gold        uint64 `json:"gold,omitempty"`
	Percent     uint8  `json:"percent"`
}

// CubeRecipeMaterialRow mirrors one cube_recipe_materials child row frozen by
// 0031_cube_recipe_state. Simple AND-list recipes emit these rows; OR-material
// recipes also emit them for material_options[0] so FileStore round-trip
// identity stays intact.
type CubeRecipeMaterialRow struct {
	NPCVnum          uint32 `json:"npc_vnum"`
	RecipePosition   int    `json:"recipe_position"`
	MaterialPosition int    `json:"material_position"`
	ItemVnum         uint32 `json:"item_vnum"`
	Count            uint16 `json:"count"`
}

// CubeRecipeMaterialOptionRow mirrors one cube_recipe_material_options child
// row frozen by 0031_cube_recipe_state. Simple AND-list recipes emit no option
// rows; OR-material recipes emit every alternative AND-group.
type CubeRecipeMaterialOptionRow struct {
	NPCVnum          uint32 `json:"npc_vnum"`
	RecipePosition   int    `json:"recipe_position"`
	OptionIndex      int    `json:"option_index"`
	MaterialPosition int    `json:"material_position"`
	ItemVnum         uint32 `json:"item_vnum"`
	Count            uint16 `json:"count"`
}

// ExportCubeRecipeState validates authored cube-recipe snapshots and returns
// rows ordered exactly as a future backfill/import tool should process them:
// NPCs by ascending npc_vnum, recipes by authored list position, then child
// materials / material_options by position. Empty snapshots emit empty
// collections, not null. All validation fails closed against FileStore bounds
// so malformed bootstrap JSON cannot be silently coerced into a future
// database import.
func ExportCubeRecipeState(snapshot Snapshot) (CubeRecipeStateExport, error) {
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return CubeRecipeStateExport{}, fmt.Errorf("%w: validate cube recipe migration export", err)
	}
	for _, npc := range normalized.NPCs {
		for recipePosition, recipe := range npc.Recipes {
			if recipe.Gold > uint64(math.MaxInt64) {
				return CubeRecipeStateExport{}, fmt.Errorf("%w: recipe[%d] gold exceeds SQL BIGINT for npc_vnum %d", ErrInvalidSnapshot, recipePosition, npc.NPCVnum)
			}
		}
	}

	export := CubeRecipeStateExport{
		MigrationVersion: CubeRecipeStateMigrationVersion,
		MigrationName:    CubeRecipeStateMigrationName,
		NPCVnums:         []uint32{},
		NPCs:             []CubeRecipeNPCRow{},
		Recipes:          []CubeRecipeRow{},
		Materials:        []CubeRecipeMaterialRow{},
		MaterialOptions:  []CubeRecipeMaterialOptionRow{},
	}

	for _, npc := range normalized.NPCs {
		export.NPCVnums = append(export.NPCVnums, npc.NPCVnum)
		export.NPCs = append(export.NPCs, CubeRecipeNPCRow{NPCVnum: npc.NPCVnum})
		for recipePosition, recipe := range npc.Recipes {
			export.Recipes = append(export.Recipes, CubeRecipeRow{
				NPCVnum:     npc.NPCVnum,
				Position:    recipePosition,
				RewardVnum:  recipe.Reward.Vnum,
				RewardCount: recipe.Reward.Count,
				Gold:        recipe.Gold,
				Percent:     recipe.Percent,
			})
			for materialPosition, material := range recipe.Materials {
				export.Materials = append(export.Materials, CubeRecipeMaterialRow{
					NPCVnum:          npc.NPCVnum,
					RecipePosition:   recipePosition,
					MaterialPosition: materialPosition,
					ItemVnum:         material.Vnum,
					Count:            material.Count,
				})
			}
			for optionIndex, option := range recipe.MaterialOptions {
				for materialPosition, material := range option {
					export.MaterialOptions = append(export.MaterialOptions, CubeRecipeMaterialOptionRow{
						NPCVnum:          npc.NPCVnum,
						RecipePosition:   recipePosition,
						OptionIndex:      optionIndex,
						MaterialPosition: materialPosition,
						ItemVnum:         material.Vnum,
						Count:            material.Count,
					})
				}
			}
		}
	}

	return export, nil
}

// ExportCubeRecipeState validates and projects the committed file-store
// snapshot onto the 0031 cube-recipe-state migration shape. It reads the same
// committed snapshot as Load and applies no mutations. A missing snapshot
// exports as an empty authored store.
func (s *FileStore) ExportCubeRecipeState() (CubeRecipeStateExport, error) {
	snapshot, err := s.Load()
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			return ExportCubeRecipeState(Snapshot{NPCs: []NPCRecipes{}})
		}
		return CubeRecipeStateExport{}, err
	}
	return ExportCubeRecipeState(snapshot)
}

func sortedUint32s(values map[uint32]struct{}) []uint32 {
	out := make([]uint32, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
