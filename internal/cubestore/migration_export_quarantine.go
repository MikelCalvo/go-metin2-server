package cubestore

import (
	"fmt"
	"math"
	"sort"
)

// CubeRecipeStateQuarantineSummary is the metadata-only result of validating or
// quarantining a retained cube-recipe-state export. It never includes recipe
// payloads, SQL, DSNs, or cube-recipe snapshot bytes.
type CubeRecipeStateQuarantineSummary struct {
	NPCCount            int      `json:"npc_count"`
	RecipeCount         int      `json:"recipe_count"`
	MaterialCount       int      `json:"material_count"`
	MaterialOptionCount int      `json:"material_option_count"`
	NPCVnums            []uint32 `json:"npc_vnums"`
}

// CubeRecipeStateQuarantineResult pairs the metadata-only quarantine summary
// with a canonicalized export ready for later offline review or backfill tools.
type CubeRecipeStateQuarantineResult struct {
	Summary CubeRecipeStateQuarantineSummary `json:"summary"`
	Export  CubeRecipeStateExport            `json:"export"`
}

// ValidateCubeRecipeStateExport fails closed when a retained export does not
// match the 0031_cube_recipe_state shape. It does not open a database, write
// cube-recipe snapshots, or mutate the supplied export.
func ValidateCubeRecipeStateExport(export CubeRecipeStateExport) (CubeRecipeStateQuarantineSummary, error) {
	canonical, summary, err := canonicalizeCubeRecipeStateExport(export)
	if err != nil {
		return CubeRecipeStateQuarantineSummary{}, err
	}
	_ = canonical
	return summary, nil
}

// QuarantineCubeRecipeStateExport validates a retained export and returns a
// canonicalized copy ordered by ascending npc_vnum and stable child-row keys.
// Declared npc_vnums merge with npc-row-derived ids so a listed npc with zero
// recipe rows can wipe-to-empty under scoped replace. It never opens a database
// or mutates cube-recipe snapshots.
func QuarantineCubeRecipeStateExport(export CubeRecipeStateExport) (CubeRecipeStateExport, CubeRecipeStateQuarantineSummary, error) {
	return canonicalizeCubeRecipeStateExport(export)
}

func canonicalizeCubeRecipeStateExport(export CubeRecipeStateExport) (CubeRecipeStateExport, CubeRecipeStateQuarantineSummary, error) {
	if export.MigrationVersion != CubeRecipeStateMigrationVersion {
		return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: migration_version %d", ErrInvalidCubeRecipeStateExport, export.MigrationVersion)
	}
	if export.MigrationName != CubeRecipeStateMigrationName {
		return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: migration_name %q", ErrInvalidCubeRecipeStateExport, export.MigrationName)
	}
	if export.NPCs == nil {
		return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: npcs must be present", ErrInvalidCubeRecipeStateExport)
	}
	if export.Recipes == nil {
		return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: recipes must be present", ErrInvalidCubeRecipeStateExport)
	}
	if export.Materials == nil {
		return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: materials must be present", ErrInvalidCubeRecipeStateExport)
	}
	if export.MaterialOptions == nil {
		return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: material_options must be present", ErrInvalidCubeRecipeStateExport)
	}

	npcScope := make(map[uint32]struct{}, len(export.NPCs)+len(export.NPCVnums))
	if export.NPCVnums != nil {
		seenDeclared := make(map[uint32]struct{}, len(export.NPCVnums))
		for _, npcVnum := range export.NPCVnums {
			if npcVnum == 0 {
				return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: npc_vnums entries must be > 0", ErrInvalidCubeRecipeStateExport)
			}
			if _, exists := seenDeclared[npcVnum]; exists {
				return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: duplicate npc_vnums entry %d", ErrInvalidCubeRecipeStateExport, npcVnum)
			}
			seenDeclared[npcVnum] = struct{}{}
			npcScope[npcVnum] = struct{}{}
		}
	}

	npcsByVnum := make(map[uint32]struct{}, len(export.NPCs))
	for _, row := range export.NPCs {
		if row.NPCVnum == 0 {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: npc_vnum must be > 0", ErrInvalidCubeRecipeStateExport)
		}
		if _, exists := npcsByVnum[row.NPCVnum]; exists {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: duplicate npc_vnum %d", ErrInvalidCubeRecipeStateExport, row.NPCVnum)
		}
		npcsByVnum[row.NPCVnum] = struct{}{}
		npcScope[row.NPCVnum] = struct{}{}
	}

	recipesByKey := make(map[string]CubeRecipeRow, len(export.Recipes))
	recipesByNPC := make(map[uint32]map[int]CubeRecipeRow, len(export.NPCs))
	for _, row := range export.Recipes {
		if _, ok := npcsByVnum[row.NPCVnum]; !ok {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: recipe npc_vnum %d is not present in npcs", ErrInvalidCubeRecipeStateExport, row.NPCVnum)
		}
		if row.Position < 0 {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: recipe position %d out of range for npc_vnum %d", ErrInvalidCubeRecipeStateExport, row.Position, row.NPCVnum)
		}
		if row.RewardVnum == 0 {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: recipe reward_vnum must be > 0 for npc_vnum %d position %d", ErrInvalidCubeRecipeStateExport, row.NPCVnum, row.Position)
		}
		if row.RewardCount == 0 {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: recipe reward_count must be > 0 for npc_vnum %d position %d", ErrInvalidCubeRecipeStateExport, row.NPCVnum, row.Position)
		}
		if row.Percent > 100 {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: recipe percent must be in 0..100 for npc_vnum %d position %d", ErrInvalidCubeRecipeStateExport, row.NPCVnum, row.Position)
		}
		if row.Gold > uint64(math.MaxInt64) {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: recipe gold exceeds SQL BIGINT for npc_vnum %d position %d", ErrInvalidCubeRecipeStateExport, row.NPCVnum, row.Position)
		}
		key := fmt.Sprintf("%d:%d", row.NPCVnum, row.Position)
		if _, exists := recipesByKey[key]; exists {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: duplicate recipe npc_vnum=%d position=%d", ErrInvalidCubeRecipeStateExport, row.NPCVnum, row.Position)
		}
		recipesByKey[key] = row
		positions, ok := recipesByNPC[row.NPCVnum]
		if !ok {
			positions = make(map[int]CubeRecipeRow)
			recipesByNPC[row.NPCVnum] = positions
		}
		positions[row.Position] = row
	}

	materialsByRecipe := make(map[string]map[int]CubeRecipeMaterialRow, len(export.Materials))
	for _, row := range export.Materials {
		recipeKey := fmt.Sprintf("%d:%d", row.NPCVnum, row.RecipePosition)
		if _, ok := recipesByKey[recipeKey]; !ok {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: material npc_vnum %d recipe_position %d is not present in recipes", ErrInvalidCubeRecipeStateExport, row.NPCVnum, row.RecipePosition)
		}
		if row.MaterialPosition < 0 {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: material_position %d out of range for npc_vnum %d recipe_position %d", ErrInvalidCubeRecipeStateExport, row.MaterialPosition, row.NPCVnum, row.RecipePosition)
		}
		if row.ItemVnum == 0 || row.Count == 0 {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: material item_vnum/count must be > 0 for npc_vnum %d recipe_position %d material_position %d", ErrInvalidCubeRecipeStateExport, row.NPCVnum, row.RecipePosition, row.MaterialPosition)
		}
		positions, ok := materialsByRecipe[recipeKey]
		if !ok {
			positions = make(map[int]CubeRecipeMaterialRow)
			materialsByRecipe[recipeKey] = positions
		}
		if _, exists := positions[row.MaterialPosition]; exists {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: duplicate material npc_vnum=%d recipe_position=%d material_position=%d", ErrInvalidCubeRecipeStateExport, row.NPCVnum, row.RecipePosition, row.MaterialPosition)
		}
		positions[row.MaterialPosition] = row
	}

	optionsByRecipe := make(map[string]map[int]map[int]CubeRecipeMaterialOptionRow, len(export.MaterialOptions))
	for _, row := range export.MaterialOptions {
		recipeKey := fmt.Sprintf("%d:%d", row.NPCVnum, row.RecipePosition)
		if _, ok := recipesByKey[recipeKey]; !ok {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: material_option npc_vnum %d recipe_position %d is not present in recipes", ErrInvalidCubeRecipeStateExport, row.NPCVnum, row.RecipePosition)
		}
		if row.OptionIndex < 0 {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: option_index %d out of range for npc_vnum %d recipe_position %d", ErrInvalidCubeRecipeStateExport, row.OptionIndex, row.NPCVnum, row.RecipePosition)
		}
		if row.MaterialPosition < 0 {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: material_option material_position %d out of range for npc_vnum %d recipe_position %d option_index %d", ErrInvalidCubeRecipeStateExport, row.MaterialPosition, row.NPCVnum, row.RecipePosition, row.OptionIndex)
		}
		if row.ItemVnum == 0 || row.Count == 0 {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: material_option item_vnum/count must be > 0 for npc_vnum %d recipe_position %d option_index %d material_position %d", ErrInvalidCubeRecipeStateExport, row.NPCVnum, row.RecipePosition, row.OptionIndex, row.MaterialPosition)
		}
		options, ok := optionsByRecipe[recipeKey]
		if !ok {
			options = make(map[int]map[int]CubeRecipeMaterialOptionRow)
			optionsByRecipe[recipeKey] = options
		}
		group, ok := options[row.OptionIndex]
		if !ok {
			group = make(map[int]CubeRecipeMaterialOptionRow)
			options[row.OptionIndex] = group
		}
		if _, exists := group[row.MaterialPosition]; exists {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: duplicate material_option npc_vnum=%d recipe_position=%d option_index=%d material_position=%d", ErrInvalidCubeRecipeStateExport, row.NPCVnum, row.RecipePosition, row.OptionIndex, row.MaterialPosition)
		}
		group[row.MaterialPosition] = row
	}

	for recipeKey, options := range optionsByRecipe {
		if len(options) < 2 {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: material_options must have at least two alternatives for recipe %s", ErrInvalidCubeRecipeStateExport, recipeKey)
		}
		optionIndexes := make([]int, 0, len(options))
		for optionIndex := range options {
			optionIndexes = append(optionIndexes, optionIndex)
		}
		sort.Ints(optionIndexes)
		for i, optionIndex := range optionIndexes {
			if optionIndex != i {
				return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: material_options indexes must be contiguous from 0 for recipe %s", ErrInvalidCubeRecipeStateExport, recipeKey)
			}
			if len(options[optionIndex]) == 0 {
				return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: material_options[%d] must not be empty for recipe %s", ErrInvalidCubeRecipeStateExport, optionIndex, recipeKey)
			}
		}
		first := options[0]
		materials := materialsByRecipe[recipeKey]
		if !materialMapsEqual(first, materials) {
			return CubeRecipeStateExport{}, CubeRecipeStateQuarantineSummary{}, fmt.Errorf("%w: materials must match material_options[0] for recipe %s", ErrInvalidCubeRecipeStateExport, recipeKey)
		}
	}

	npcVnums := sortedUint32s(npcScope)
	canonical := CubeRecipeStateExport{
		MigrationVersion: CubeRecipeStateMigrationVersion,
		MigrationName:    CubeRecipeStateMigrationName,
		NPCVnums:         append([]uint32(nil), npcVnums...),
		NPCs:             make([]CubeRecipeNPCRow, 0, len(npcsByVnum)),
		Recipes:          make([]CubeRecipeRow, 0, len(export.Recipes)),
		Materials:        make([]CubeRecipeMaterialRow, 0, len(export.Materials)),
		MaterialOptions:  make([]CubeRecipeMaterialOptionRow, 0, len(export.MaterialOptions)),
	}
	if canonical.NPCVnums == nil {
		canonical.NPCVnums = []uint32{}
	}

	for _, npcVnum := range npcVnums {
		if _, ok := npcsByVnum[npcVnum]; !ok {
			continue
		}
		canonical.NPCs = append(canonical.NPCs, CubeRecipeNPCRow{NPCVnum: npcVnum})
		positions := recipesByNPC[npcVnum]
		recipePositions := make([]int, 0, len(positions))
		for position := range positions {
			recipePositions = append(recipePositions, position)
		}
		sort.Ints(recipePositions)
		for _, recipePosition := range recipePositions {
			canonical.Recipes = append(canonical.Recipes, positions[recipePosition])
			recipeKey := fmt.Sprintf("%d:%d", npcVnum, recipePosition)
			if materials := materialsByRecipe[recipeKey]; len(materials) > 0 {
				materialPositions := make([]int, 0, len(materials))
				for materialPosition := range materials {
					materialPositions = append(materialPositions, materialPosition)
				}
				sort.Ints(materialPositions)
				for _, materialPosition := range materialPositions {
					canonical.Materials = append(canonical.Materials, materials[materialPosition])
				}
			}
			if options := optionsByRecipe[recipeKey]; len(options) > 0 {
				optionIndexes := make([]int, 0, len(options))
				for optionIndex := range options {
					optionIndexes = append(optionIndexes, optionIndex)
				}
				sort.Ints(optionIndexes)
				for _, optionIndex := range optionIndexes {
					group := options[optionIndex]
					materialPositions := make([]int, 0, len(group))
					for materialPosition := range group {
						materialPositions = append(materialPositions, materialPosition)
					}
					sort.Ints(materialPositions)
					for _, materialPosition := range materialPositions {
						canonical.MaterialOptions = append(canonical.MaterialOptions, group[materialPosition])
					}
				}
			}
		}
	}

	summary := CubeRecipeStateQuarantineSummary{
		NPCCount:            len(canonical.NPCs),
		RecipeCount:         len(canonical.Recipes),
		MaterialCount:       len(canonical.Materials),
		MaterialOptionCount: len(canonical.MaterialOptions),
		NPCVnums:            append([]uint32(nil), canonical.NPCVnums...),
	}
	if summary.NPCVnums == nil {
		summary.NPCVnums = []uint32{}
	}
	return canonical, summary, nil
}

func materialMapsEqual(options map[int]CubeRecipeMaterialOptionRow, materials map[int]CubeRecipeMaterialRow) bool {
	if len(options) != len(materials) {
		return false
	}
	for position, option := range options {
		material, ok := materials[position]
		if !ok {
			return false
		}
		if option.ItemVnum != material.ItemVnum || option.Count != material.Count {
			return false
		}
	}
	return true
}
