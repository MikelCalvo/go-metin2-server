package cubestore

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	ErrStorePathRequired      = errors.New("cube recipe store path is required")
	ErrSnapshotNotFound       = errors.New("cube recipe snapshot not found")
	ErrInvalidSnapshot        = errors.New("invalid cube recipe snapshot")
	ErrBackupDirRequired      = errors.New("cube recipe backup dir is required")
	ErrBackupDirNotEmpty      = errors.New("cube recipe backup dir is not empty")
	ErrBackupDirInsideStore   = errors.New("cube recipe backup dir is inside cube recipe store")
	ErrRestoreSourceRequired  = errors.New("cube recipe restore source dir is required")
	ErrRestoreSourceNotFound  = errors.New("cube recipe restore source dir not found")
	ErrRestoreDirNotEmpty     = errors.New("cube recipe restore dir is not empty")
	ErrRestoreDirInsideSource = errors.New("cube recipe restore dir is inside cube recipe backup source")
	ErrBackupManifestRequired = errors.New("cube recipe backup manifest is required")
	ErrInvalidBackupManifest  = errors.New("invalid cube recipe backup manifest")
)

const (
	// ChatMaxLen mirrors the external oracle CHAT_MAX_LEN used by cube r_list.
	ChatMaxLen = 512
	// ResultListTextOverheadReserve mirrors the oracle's
	// `resultText.size() - 20 >= CHAT_MAX_LEN` oversize gate.
	ResultListTextOverheadReserve = 20
	// BootstrapDefaultNPCVnum is the lab /open_cube default NPC race.
	BootstrapDefaultNPCVnum uint32 = 20022
	// CubeMaxNum mirrors the external oracle CUBE_MAX_NUM craft-slot bound.
	CubeMaxNum = 24
	// BackupManifestFilename is the deterministic restored-backup marker.
	BackupManifestFilename = "cube-recipe-backup-manifest.json"
	// BackupManifestFormat identifies a closed cube-recipe FileStore backup.
	BackupManifestFormat = "go-metin2-cube-recipe-backup-v1"
)

// Reward is one craftable result row for cube r_list.
type Reward struct {
	Vnum  uint32 `json:"vnum"`
	Count uint16 `json:"count"`
}

// Material is authored material detail consumed by cube m_info encoding.
type Material struct {
	Vnum  uint32 `json:"vnum"`
	Count uint16 `json:"count"`
}

// Recipe is one NPC craftable result. Materials/gold drive cube m_info text;
// Reward drives cube r_list. Percent gates /cube make (bootstrap owns
// deterministic 100, injected-roll 1..99, and store-accepted always-fail 0).
// MaterialOptions authors OR-alternatives (two or more AND-groups); matching
// and make consume the first covering group and leave unused alternatives.
type Recipe struct {
	Reward    Reward     `json:"reward"`
	Materials []Material `json:"materials,omitempty"`
	// MaterialOptions is the authored OR-material companion: each inner
	// slice is one AND-group, groups join with `|` in cube m_info.
	MaterialOptions [][]Material `json:"material_options,omitempty"`
	Gold            uint64       `json:"gold,omitempty"`
	// Percent is persisted explicitly (including 0) so authored always-fail
	// recipes round-trip instead of collapsing through omitempty.
	Percent uint8 `json:"percent"`
}

// NPCRecipes is the authored recipe list for one cube NPC vnum.
type NPCRecipes struct {
	NPCVnum uint32   `json:"npc_vnum"`
	Recipes []Recipe `json:"recipes"`
}

// Snapshot is the committed cube-recipe FileStore / MemoryStore payload.
type Snapshot struct {
	NPCs []NPCRecipes `json:"npcs"`
}

// SnapshotSummary is the deterministic backup/validate projection.
type SnapshotSummary struct {
	NPCCount       int      `json:"npc_count"`
	RecipeCount    int      `json:"recipe_count"`
	NPCVnums       []uint32 `json:"npc_vnums"`
	CrashTempCount int      `json:"crash_temp_count,omitempty"`
	CrashTempFiles []string `json:"crash_temp_files,omitempty"`
}

// BackupManifest is the closed cube-recipe backup audit artifact.
type BackupManifest struct {
	Format  string               `json:"format"`
	Summary SnapshotSummary      `json:"summary"`
	Files   []BackupManifestFile `json:"files"`
}

// BackupManifestFile records one manifested snapshot payload.
type BackupManifestFile struct {
	Filename  string `json:"filename"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

// Store is the Load/Save seam used by gamed bootstrap and focused tests.
type Store interface {
	Load() (Snapshot, error)
	Save(Snapshot) error
}

// BootstrapSnapshot returns the deterministic lab default recipe list for NPC
// 20022. Runtime boot uses this when no authored cube-recipe file is present.
func BootstrapSnapshot() Snapshot {
	return Snapshot{NPCs: []NPCRecipes{{
		NPCVnum: BootstrapDefaultNPCVnum,
		Recipes: []Recipe{{
			Reward: Reward{Vnum: 27001, Count: 1},
			Materials: []Material{
				{Vnum: 27002, Count: 2},
			},
			Gold:    100,
			Percent: 100,
		}},
	}}}
}

// RecipesForNPC returns a cloned recipe slice for npcVnum, or nil when missing/empty.
func RecipesForNPC(snapshot Snapshot, npcVnum uint32) []Recipe {
	for _, npc := range normalizeSnapshot(snapshot).NPCs {
		if npc.NPCVnum == npcVnum {
			if len(npc.Recipes) == 0 {
				return nil
			}
			return cloneRecipes(npc.Recipes)
		}
	}
	return nil
}

// FormatResultListCommand builds the self-only CHAT_TYPE_COMMAND payload
// `cube r_list <npcVnum> <resultCount> <vnum,count/...>`.
// ok is false for empty recipes or oversize entry text (fail-closed; no partial list).
func FormatResultListCommand(npcVnum uint32, recipes []Recipe) (string, bool) {
	if len(recipes) == 0 {
		return "", false
	}
	entries := make([]string, 0, len(recipes))
	for _, recipe := range recipes {
		entries = append(entries, fmt.Sprintf("%d,%d", recipe.Reward.Vnum, recipe.Reward.Count))
	}
	entryText := strings.Join(entries, "/")
	if len(entryText) >= ChatMaxLen+ResultListTextOverheadReserve {
		return "", false
	}
	return fmt.Sprintf("cube r_list %d %d %s", npcVnum, len(recipes), entryText), true
}

// FormatRecipeMaterialInfoText encodes one recipe's materials+gold as
// `vnum,count[&vnum,count...][|vnum,count[&...]][/gold]`.
// OR-material recipes join AND-groups with `|`. Gold appends once at the end
// when authored gold is non-zero. ok is false when there are no materials.
func FormatRecipeMaterialInfoText(recipe Recipe) (string, bool) {
	sets := recipeMaterialSets(recipe)
	if len(sets) == 0 {
		return "", false
	}
	encoded := make([]string, 0, len(sets))
	for _, set := range sets {
		text, ok := formatMaterialSetText(set)
		if !ok {
			return "", false
		}
		encoded = append(encoded, text)
	}
	text := strings.Join(encoded, "|")
	if recipe.Gold > 0 {
		text += fmt.Sprintf("/%d", recipe.Gold)
	}
	return text, true
}

func formatMaterialSetText(materials []Material) (string, bool) {
	if len(materials) == 0 {
		return "", false
	}
	parts := make([]string, 0, len(materials))
	for _, material := range materials {
		if material.Vnum == 0 || material.Count == 0 {
			return "", false
		}
		parts = append(parts, fmt.Sprintf("%d,%d", material.Vnum, material.Count))
	}
	return strings.Join(parts, "&"), true
}

// FormatMaterialInfoCommand builds the self-only CHAT_TYPE_COMMAND payload
// `cube m_info <startIndex> <requestCount> <infoText[@infoText...]>`.
// ok is false when the window is empty/past-end, any selected recipe lacks
// material text, or the encoded entry text is oversize (fail-closed).
func FormatMaterialInfoCommand(startIndex int, requestCount int, recipes []Recipe) (string, bool) {
	if requestCount <= 0 || startIndex < 0 || startIndex >= len(recipes) {
		return "", false
	}
	end := startIndex + requestCount
	if end > len(recipes) {
		end = len(recipes)
	}
	entries := make([]string, 0, end-startIndex)
	for _, recipe := range recipes[startIndex:end] {
		infoText, ok := FormatRecipeMaterialInfoText(recipe)
		if !ok {
			return "", false
		}
		entries = append(entries, infoText)
	}
	if len(entries) == 0 {
		return "", false
	}
	entryText := strings.Join(entries, "@")
	if len(entryText) >= ChatMaxLen+ResultListTextOverheadReserve {
		return "", false
	}
	return fmt.Sprintf("cube m_info %d %d %s", startIndex, requestCount, entryText), true
}

// BoundMaterial is one live inventory cell contribution used by craft-slot
// `cube info` gold resolution.
type BoundMaterial struct {
	Vnum  uint32
	Count uint16
}

// MatchSimpleRecipe returns the first recipe whose required materials are
// covered by the bound live cells (order-insensitive, aggregated by vnum,
// oracle-shaped `count >= need` per required vnum). Extra bound vnums are
// allowed. OR-material recipes match the first covering AND-group; the
// returned recipe.Materials is that covering group so make consumes only it.
// ok is false when no recipe is covered.
func MatchSimpleRecipe(recipes []Recipe, bound []BoundMaterial) (Recipe, bool) {
	boundCounts := aggregateMaterialCounts(bound)
	if len(boundCounts) == 0 {
		return Recipe{}, false
	}
	for _, recipe := range recipes {
		for _, set := range recipeMaterialSets(recipe) {
			needCounts := materialSetNeedCounts(set)
			if len(needCounts) == 0 {
				continue
			}
			if materialCountsCover(boundCounts, needCounts) {
				matched := recipe
				matched.Materials = cloneMaterials(set)
				matched.MaterialOptions = cloneMaterialOptions(recipe.MaterialOptions)
				return matched, true
			}
		}
	}
	return Recipe{}, false
}

func recipeMaterialSets(recipe Recipe) [][]Material {
	if len(recipe.MaterialOptions) > 0 {
		sets := make([][]Material, 0, len(recipe.MaterialOptions))
		sets = append(sets, recipe.MaterialOptions...)
		return sets
	}
	if len(recipe.Materials) == 0 {
		return nil
	}
	return [][]Material{recipe.Materials}
}

func materialSetNeedCounts(materials []Material) map[uint32]uint32 {
	needCounts := make(map[uint32]uint32, len(materials))
	for _, material := range materials {
		if material.Vnum == 0 || material.Count == 0 {
			return nil
		}
		needCounts[material.Vnum] += uint32(material.Count)
	}
	if len(needCounts) == 0 {
		return nil
	}
	return needCounts
}

// MatchSimpleRecipeGold returns the authored gold for the first recipe whose
// required materials (simple AND-list or first covering OR-group) are covered
// by the bound live cells (order-insensitive, aggregated by vnum). ok is false
// when no recipe matches.
func MatchSimpleRecipeGold(recipes []Recipe, bound []BoundMaterial) (uint64, bool) {
	recipe, ok := MatchSimpleRecipe(recipes, bound)
	if !ok {
		return 0, false
	}
	return recipe.Gold, true
}

// FormatCubeInfoCommand builds the self-only CHAT_TYPE_COMMAND payload
// `cube info <gold> 0 0` used after successful craft-slot add/del.
func FormatCubeInfoCommand(gold uint64) string {
	return fmt.Sprintf("cube info %d 0 0", gold)
}

// FormatCubeSuccessCommand builds the self-only CHAT_TYPE_COMMAND payload
// `cube success <rewardVnum> <rewardCount>` emitted after a successful make.
func FormatCubeSuccessCommand(rewardVnum uint32, rewardCount uint16) string {
	return fmt.Sprintf("cube success %d %d", rewardVnum, rewardCount)
}

// FormatCubeFailCommand builds the self-only CHAT_TYPE_COMMAND payload
// `cube fail` emitted after a failed make roll (materials/gold already consumed).
func FormatCubeFailCommand() string {
	return "cube fail"
}

// FormatCubeListInfoMessage builds one self-only CHAT_TYPE_INFO payload for
// `/cube list`: `cube[<cubeIndex>]: inventory[<invenCell>]: <itemName>`.
func FormatCubeListInfoMessage(cubeIndex uint16, invenCell uint16, itemName string) string {
	return fmt.Sprintf("cube[%d]: inventory[%d]: %s", cubeIndex, invenCell, itemName)
}

func aggregateMaterialCounts(bound []BoundMaterial) map[uint32]uint32 {
	counts := make(map[uint32]uint32)
	for _, material := range bound {
		if material.Vnum == 0 || material.Count == 0 {
			continue
		}
		counts[material.Vnum] += uint32(material.Count)
	}
	return counts
}

// materialCountsCover reports whether boundCounts provides at least needCounts
// for every required vnum (oracle FN_check_item_count). Extra bound vnums ok.
func materialCountsCover(boundCounts, needCounts map[uint32]uint32) bool {
	if len(needCounts) == 0 {
		return false
	}
	for vnum, need := range needCounts {
		if boundCounts[vnum] < need {
			return false
		}
	}
	return true
}

func summarizeSnapshot(snapshot Snapshot) SnapshotSummary {
	normalized := normalizeSnapshot(snapshot)
	summary := SnapshotSummary{
		NPCCount:    len(normalized.NPCs),
		NPCVnums:    make([]uint32, 0, len(normalized.NPCs)),
		RecipeCount: 0,
	}
	for _, npc := range normalized.NPCs {
		summary.NPCVnums = append(summary.NPCVnums, npc.NPCVnum)
		summary.RecipeCount += len(npc.Recipes)
	}
	return summary
}

func NormalizeSnapshot(snapshot Snapshot) Snapshot {
	return normalizeSnapshot(snapshot)
}

func ValidSnapshot(snapshot Snapshot) bool {
	return validateSnapshot(normalizeSnapshot(snapshot)) == nil
}

func normalizeSnapshot(snapshot Snapshot) Snapshot {
	normalized := Snapshot{NPCs: cloneNPCRecipes(snapshot.NPCs)}
	if normalized.NPCs == nil {
		normalized.NPCs = []NPCRecipes{}
	}
	for i := range normalized.NPCs {
		normalized.NPCs[i].Recipes = cloneRecipes(normalized.NPCs[i].Recipes)
		if normalized.NPCs[i].Recipes == nil {
			normalized.NPCs[i].Recipes = []Recipe{}
		}
		for j := range normalized.NPCs[i].Recipes {
			recipe := &normalized.NPCs[i].Recipes[j]
			recipe.Materials = cloneMaterials(recipe.Materials)
			if recipe.Materials == nil {
				recipe.Materials = []Material{}
			}
			recipe.MaterialOptions = cloneMaterialOptions(recipe.MaterialOptions)
			for k := range recipe.MaterialOptions {
				if recipe.MaterialOptions[k] == nil {
					recipe.MaterialOptions[k] = []Material{}
				}
			}
			if len(recipe.MaterialOptions) > 0 && len(recipe.Materials) == 0 {
				recipe.Materials = cloneMaterials(recipe.MaterialOptions[0])
				if recipe.Materials == nil {
					recipe.Materials = []Material{}
				}
			}
		}
	}
	sort.SliceStable(normalized.NPCs, func(i, j int) bool {
		return normalized.NPCs[i].NPCVnum < normalized.NPCs[j].NPCVnum
	})
	return normalized
}

func validateSnapshot(snapshot Snapshot) error {
	seen := make(map[uint32]struct{}, len(snapshot.NPCs))
	for _, npc := range snapshot.NPCs {
		if npc.NPCVnum == 0 {
			return fmt.Errorf("%w: npc_vnum must be non-zero", ErrInvalidSnapshot)
		}
		if _, exists := seen[npc.NPCVnum]; exists {
			return fmt.Errorf("%w: duplicate npc_vnum %d", ErrInvalidSnapshot, npc.NPCVnum)
		}
		seen[npc.NPCVnum] = struct{}{}
		if npc.Recipes == nil {
			return fmt.Errorf("%w: recipes collection must not be null for npc_vnum %d", ErrInvalidSnapshot, npc.NPCVnum)
		}
		for i, recipe := range npc.Recipes {
			if recipe.Reward.Vnum == 0 {
				return fmt.Errorf("%w: recipe[%d] reward.vnum must be non-zero for npc_vnum %d", ErrInvalidSnapshot, i, npc.NPCVnum)
			}
			if recipe.Reward.Count == 0 {
				return fmt.Errorf("%w: recipe[%d] reward.count must be non-zero for npc_vnum %d", ErrInvalidSnapshot, i, npc.NPCVnum)
			}
			if recipe.Percent > 100 {
				return fmt.Errorf("%w: recipe[%d] percent must be in 0..100 for npc_vnum %d", ErrInvalidSnapshot, i, npc.NPCVnum)
			}
			if recipe.Materials == nil {
				return fmt.Errorf("%w: recipe[%d] materials collection must not be null for npc_vnum %d", ErrInvalidSnapshot, i, npc.NPCVnum)
			}
			for j, material := range recipe.Materials {
				if material.Vnum == 0 {
					return fmt.Errorf("%w: recipe[%d] materials[%d].vnum must be non-zero for npc_vnum %d", ErrInvalidSnapshot, i, j, npc.NPCVnum)
				}
				if material.Count == 0 {
					return fmt.Errorf("%w: recipe[%d] materials[%d].count must be non-zero for npc_vnum %d", ErrInvalidSnapshot, i, j, npc.NPCVnum)
				}
			}
			if err := validateRecipeMaterialOptions(recipe, i, npc.NPCVnum); err != nil {
				return err
			}
		}
	}
	return nil
}

func cloneNPCRecipes(npcs []NPCRecipes) []NPCRecipes {
	if npcs == nil {
		return nil
	}
	cloned := make([]NPCRecipes, len(npcs))
	copy(cloned, npcs)
	for i := range cloned {
		cloned[i].Recipes = cloneRecipes(cloned[i].Recipes)
	}
	return cloned
}

func cloneRecipes(recipes []Recipe) []Recipe {
	if recipes == nil {
		return nil
	}
	cloned := make([]Recipe, len(recipes))
	copy(cloned, recipes)
	for i := range cloned {
		cloned[i].Materials = cloneMaterials(cloned[i].Materials)
		cloned[i].MaterialOptions = cloneMaterialOptions(cloned[i].MaterialOptions)
	}
	return cloned
}

func cloneMaterialOptions(options [][]Material) [][]Material {
	if options == nil {
		return nil
	}
	cloned := make([][]Material, len(options))
	for i := range options {
		cloned[i] = cloneMaterials(options[i])
	}
	return cloned
}

func materialSlicesEqual(left, right []Material) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].Vnum != right[i].Vnum || left[i].Count != right[i].Count {
			return false
		}
	}
	return true
}

func validateRecipeMaterialOptions(recipe Recipe, recipeIndex int, npcVnum uint32) error {
	if len(recipe.MaterialOptions) == 0 {
		return nil
	}
	if len(recipe.MaterialOptions) < 2 {
		return fmt.Errorf("%w: recipe[%d] material_options must have at least two alternatives for npc_vnum %d", ErrInvalidSnapshot, recipeIndex, npcVnum)
	}
	for optionIndex, option := range recipe.MaterialOptions {
		if option == nil {
			return fmt.Errorf("%w: recipe[%d] material_options[%d] must not be null for npc_vnum %d", ErrInvalidSnapshot, recipeIndex, optionIndex, npcVnum)
		}
		if len(option) == 0 {
			return fmt.Errorf("%w: recipe[%d] material_options[%d] must not be empty for npc_vnum %d", ErrInvalidSnapshot, recipeIndex, optionIndex, npcVnum)
		}
		for materialIndex, material := range option {
			if material.Vnum == 0 {
				return fmt.Errorf("%w: recipe[%d] material_options[%d][%d].vnum must be non-zero for npc_vnum %d", ErrInvalidSnapshot, recipeIndex, optionIndex, materialIndex, npcVnum)
			}
			if material.Count == 0 {
				return fmt.Errorf("%w: recipe[%d] material_options[%d][%d].count must be non-zero for npc_vnum %d", ErrInvalidSnapshot, recipeIndex, optionIndex, materialIndex, npcVnum)
			}
		}
	}
	if len(recipe.Materials) > 0 && !materialSlicesEqual(recipe.Materials, recipe.MaterialOptions[0]) {
		return fmt.Errorf("%w: recipe[%d] materials must match material_options[0] for npc_vnum %d", ErrInvalidSnapshot, recipeIndex, npcVnum)
	}
	return nil
}

func cloneMaterials(materials []Material) []Material {
	if materials == nil {
		return nil
	}
	cloned := make([]Material, len(materials))
	copy(cloned, materials)
	return cloned
}
