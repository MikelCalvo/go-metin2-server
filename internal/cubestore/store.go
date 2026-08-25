package cubestore

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	ErrStorePathRequired = errors.New("cube recipe store path is required")
	ErrSnapshotNotFound  = errors.New("cube recipe snapshot not found")
	ErrInvalidSnapshot   = errors.New("invalid cube recipe snapshot")
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
// Reward drives cube r_list. Percent gates /cube make (bootstrap owns 100 only).
type Recipe struct {
	Reward    Reward     `json:"reward"`
	Materials []Material `json:"materials,omitempty"`
	Gold      uint64     `json:"gold,omitempty"`
	Percent   uint8      `json:"percent,omitempty"`
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

// FormatRecipeMaterialInfoText encodes one simple (non-complicated) recipe's
// materials+gold as `vnum,count[&vnum,count...][/gold]`.
// ok is false when there are no materials (empty infoText).
func FormatRecipeMaterialInfoText(recipe Recipe) (string, bool) {
	if len(recipe.Materials) == 0 {
		return "", false
	}
	parts := make([]string, 0, len(recipe.Materials))
	for _, material := range recipe.Materials {
		parts = append(parts, fmt.Sprintf("%d,%d", material.Vnum, material.Count))
	}
	text := strings.Join(parts, "&")
	if recipe.Gold > 0 {
		text += fmt.Sprintf("/%d", recipe.Gold)
	}
	return text, true
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

// MatchSimpleRecipe returns the first simple recipe whose material multiset
// exactly matches the bound live cells (order-insensitive, aggregated by vnum).
// ok is false when no recipe matches.
func MatchSimpleRecipe(recipes []Recipe, bound []BoundMaterial) (Recipe, bool) {
	boundCounts := aggregateMaterialCounts(bound)
	if len(boundCounts) == 0 {
		return Recipe{}, false
	}
	for _, recipe := range recipes {
		if len(recipe.Materials) == 0 {
			continue
		}
		needCounts := make(map[uint32]uint32, len(recipe.Materials))
		for _, material := range recipe.Materials {
			if material.Vnum == 0 || material.Count == 0 {
				needCounts = nil
				break
			}
			needCounts[material.Vnum] += uint32(material.Count)
		}
		if needCounts == nil || len(needCounts) == 0 {
			continue
		}
		if materialCountsEqual(boundCounts, needCounts) {
			return recipe, true
		}
	}
	return Recipe{}, false
}

// MatchSimpleRecipeGold returns the authored gold for the first simple recipe
// whose material multiset exactly matches the bound live cells
// (order-insensitive, aggregated by vnum). ok is false when no recipe matches.
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
// `cube success <rewardVnum> <rewardCount>` emitted after percent=100 make.
func FormatCubeSuccessCommand(rewardVnum uint32, rewardCount uint16) string {
	return fmt.Sprintf("cube success %d %d", rewardVnum, rewardCount)
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

func materialCountsEqual(left, right map[uint32]uint32) bool {
	if len(left) != len(right) {
		return false
	}
	for vnum, count := range left {
		if right[vnum] != count {
			return false
		}
	}
	return true
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
			normalized.NPCs[i].Recipes[j].Materials = cloneMaterials(normalized.NPCs[i].Recipes[j].Materials)
			if normalized.NPCs[i].Recipes[j].Materials == nil {
				normalized.NPCs[i].Recipes[j].Materials = []Material{}
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
			if recipe.Percent == 0 || recipe.Percent > 100 {
				return fmt.Errorf("%w: recipe[%d] percent must be in 1..100 for npc_vnum %d", ErrInvalidSnapshot, i, npc.NPCVnum)
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
	}
	return cloned
}

func cloneMaterials(materials []Material) []Material {
	if materials == nil {
		return nil
	}
	cloned := make([]Material, len(materials))
	copy(cloned, materials)
	return cloned
}
