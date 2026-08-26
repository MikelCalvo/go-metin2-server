package cubestore

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFileStoreSaveThenLoadRoundTrip(t *testing.T) {
	restore := DisableDurableSyncForTest()
	defer restore()

	path := filepath.Join(t.TempDir(), "state", "cube-recipes.json")
	store := NewFileStore(path)
	want := Snapshot{NPCs: []NPCRecipes{{
		NPCVnum: BootstrapDefaultNPCVnum,
		Recipes: []Recipe{{
			Reward:    Reward{Vnum: 27001, Count: 1},
			Materials: []Material{{Vnum: 27002, Count: 2}},
			Gold:      100,
			Percent:   100,
		}, {
			Reward:    Reward{Vnum: 11200, Count: 1},
			Materials: []Material{},
			Percent:   100,
		}},
	}}}
	if err := store.Save(want); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected snapshot:\n got: %#v\nwant: %#v", got, want)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read raw snapshot: %v", err)
	}
	if !strings.Contains(string(raw), `"npc_vnum": 20022`) {
		t.Fatalf("expected deterministic JSON to persist npc_vnum, got:\n%s", raw)
	}
	if strings.Contains(string(raw), `"npcs": null`) {
		t.Fatalf("expected non-null npcs collection, got:\n%s", raw)
	}
}

func TestFileStoreRoundTripsExplicitPercentZeroAlwaysFail(t *testing.T) {
	restore := DisableDurableSyncForTest()
	defer restore()

	path := filepath.Join(t.TempDir(), "state", "cube-recipes.json")
	store := NewFileStore(path)
	want := Snapshot{NPCs: []NPCRecipes{{
		NPCVnum: BootstrapDefaultNPCVnum,
		Recipes: []Recipe{{
			Reward:    Reward{Vnum: 27001, Count: 1},
			Materials: []Material{{Vnum: 27002, Count: 2}},
			Gold:      100,
			Percent:   0,
		}},
	}}}
	if err := store.Save(want); err != nil {
		t.Fatalf("save percent-0 snapshot: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load percent-0 snapshot: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected percent-0 snapshot:\n got: %#v\nwant: %#v", got, want)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read percent-0 snapshot: %v", err)
	}
	if !strings.Contains(string(raw), `"percent": 0`) {
		t.Fatalf("expected explicit percent 0 to persist, got:\n%s", raw)
	}
}

func TestFileStoreLoadTreatsOmittedPercentAsZeroAlwaysFail(t *testing.T) {
	restore := DisableDurableSyncForTest()
	defer restore()

	path := filepath.Join(t.TempDir(), "state", "cube-recipes.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for omitted-percent fixture: %v", err)
	}
	raw := `{
  "npcs": [
    {
      "npc_vnum": 20022,
      "recipes": [
        {
          "reward": {
            "vnum": 27001,
            "count": 1
          },
          "materials": [
            {
              "vnum": 27002,
              "count": 2
            }
          ],
          "gold": 100
        }
      ]
    }
  ]
}
`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write omitted-percent fixture: %v", err)
	}
	got, err := NewFileStore(path).Load()
	if err != nil {
		t.Fatalf("load omitted-percent snapshot: %v", err)
	}
	if len(got.NPCs) != 1 || len(got.NPCs[0].Recipes) != 1 {
		t.Fatalf("unexpected omitted-percent snapshot shape: %+v", got)
	}
	if got.NPCs[0].Recipes[0].Percent != 0 {
		t.Fatalf("expected omitted percent to load as 0, got %d", got.NPCs[0].Recipes[0].Percent)
	}
}

func TestFileStoreRejectsMalformedRecipesFailClosed(t *testing.T) {
	restore := DisableDurableSyncForTest()
	defer restore()

	path := filepath.Join(t.TempDir(), "state", "cube-recipes.json")
	store := NewFileStore(path)
	cases := []struct {
		name     string
		snapshot Snapshot
	}{
		{
			name: "zero npc vnum",
			snapshot: Snapshot{NPCs: []NPCRecipes{{
				NPCVnum: 0,
				Recipes: []Recipe{{Reward: Reward{Vnum: 1, Count: 1}, Materials: []Material{}, Percent: 100}},
			}}},
		},
		{
			name: "zero reward vnum",
			snapshot: Snapshot{NPCs: []NPCRecipes{{
				NPCVnum: 20022,
				Recipes: []Recipe{{Reward: Reward{Vnum: 0, Count: 1}, Materials: []Material{}, Percent: 100}},
			}}},
		},
		{
			name: "zero reward count",
			snapshot: Snapshot{NPCs: []NPCRecipes{{
				NPCVnum: 20022,
				Recipes: []Recipe{{Reward: Reward{Vnum: 1, Count: 0}, Materials: []Material{}, Percent: 100}},
			}}},
		},
		{
			name: "percent above 100",
			snapshot: Snapshot{NPCs: []NPCRecipes{{
				NPCVnum: 20022,
				Recipes: []Recipe{{Reward: Reward{Vnum: 1, Count: 1}, Materials: []Material{}, Percent: 101}},
			}}},
		},
		{
			name: "duplicate npc",
			snapshot: Snapshot{NPCs: []NPCRecipes{
				{NPCVnum: 20022, Recipes: []Recipe{{Reward: Reward{Vnum: 1, Count: 1}, Materials: []Material{}, Percent: 100}}},
				{NPCVnum: 20022, Recipes: []Recipe{{Reward: Reward{Vnum: 2, Count: 1}, Materials: []Material{}, Percent: 100}}},
			}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.Save(tc.snapshot); err == nil {
				t.Fatal("expected save to reject malformed snapshot")
			}
		})
	}
}

func TestFileStoreSaveEmptySnapshotWritesDeterministicEmptyNPCArray(t *testing.T) {
	restore := DisableDurableSyncForTest()
	defer restore()

	path := filepath.Join(t.TempDir(), "state", "cube-recipes.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{}); err != nil {
		t.Fatalf("save empty snapshot: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read empty snapshot: %v", err)
	}
	if !strings.Contains(string(raw), `"npcs": []`) {
		t.Fatalf("expected deterministic empty npcs array, got:\n%s", raw)
	}
}

func TestMemoryStoreRoundTripsBootstrapSnapshot(t *testing.T) {
	store := NewMemoryStore()
	want := BootstrapSnapshot()
	if err := store.Save(want); err != nil {
		t.Fatalf("save bootstrap snapshot: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load bootstrap snapshot: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected bootstrap snapshot:\n got: %#v\nwant: %#v", got, want)
	}
	recipes := RecipesForNPC(got, BootstrapDefaultNPCVnum)
	if len(recipes) != 1 || recipes[0].Reward.Vnum != 27001 || recipes[0].Reward.Count != 1 {
		t.Fatalf("unexpected bootstrap recipes for default NPC: %+v", recipes)
	}
	if recipes[0].Percent != 100 || recipes[0].Gold != 100 {
		t.Fatalf("expected bootstrap recipe percent 100 gold 100, got percent=%d gold=%d", recipes[0].Percent, recipes[0].Gold)
	}
}

func TestFormatResultListCommandMatchesAuthoredFixture(t *testing.T) {
	message, ok := FormatResultListCommand(BootstrapDefaultNPCVnum, BootstrapSnapshot().NPCs[0].Recipes)
	if !ok {
		t.Fatal("expected bootstrap recipes to encode")
	}
	if message != "cube r_list 20022 1 27001,1" {
		t.Fatalf("unexpected r_list command: %q", message)
	}
}

func TestFormatResultListCommandRejectsEmptyAndOversizeFailClosed(t *testing.T) {
	if _, ok := FormatResultListCommand(20022, nil); ok {
		t.Fatal("expected empty recipes to fail closed")
	}
	recipes := make([]Recipe, 0, 80)
	for i := 0; i < 80; i++ {
		recipes = append(recipes, Recipe{Reward: Reward{Vnum: 100000 + uint32(i), Count: 9999}, Materials: []Material{}})
	}
	if _, ok := FormatResultListCommand(20022, recipes); ok {
		t.Fatal("expected oversize entry text to fail closed")
	}
}

func TestFormatRecipeMaterialInfoTextMatchesBootstrapFixture(t *testing.T) {
	text, ok := FormatRecipeMaterialInfoText(BootstrapSnapshot().NPCs[0].Recipes[0])
	if !ok {
		t.Fatal("expected bootstrap recipe materials to encode")
	}
	if text != "27002,2/100" {
		t.Fatalf("unexpected material infoText: %q", text)
	}
}

func TestFormatRecipeMaterialInfoTextJoinsMaterialsAndOmitsZeroGold(t *testing.T) {
	text, ok := FormatRecipeMaterialInfoText(Recipe{
		Reward: Reward{Vnum: 1, Count: 1},
		Materials: []Material{
			{Vnum: 10, Count: 1},
			{Vnum: 11, Count: 2},
		},
	})
	if !ok {
		t.Fatal("expected multi-material recipe to encode")
	}
	if text != "10,1&11,2" {
		t.Fatalf("unexpected multi-material infoText: %q", text)
	}
	if _, ok := FormatRecipeMaterialInfoText(Recipe{Reward: Reward{Vnum: 1, Count: 1}, Materials: []Material{}, Gold: 50}); ok {
		t.Fatal("expected empty materials to fail closed even with gold")
	}
}

func TestFormatMaterialInfoCommandMatchesBootstrapFixture(t *testing.T) {
	message, ok := FormatMaterialInfoCommand(0, 1, BootstrapSnapshot().NPCs[0].Recipes)
	if !ok {
		t.Fatal("expected bootstrap material info to encode")
	}
	if message != "cube m_info 0 1 27002,2/100" {
		t.Fatalf("unexpected m_info command: %q", message)
	}
}

func TestFormatMaterialInfoCommandJoinsWindowAndRejectsPastEndOrOversize(t *testing.T) {
	recipes := []Recipe{
		{Reward: Reward{Vnum: 1, Count: 1}, Materials: []Material{{Vnum: 10, Count: 1}}, Gold: 5},
		{Reward: Reward{Vnum: 2, Count: 1}, Materials: []Material{{Vnum: 20, Count: 2}}},
	}
	message, ok := FormatMaterialInfoCommand(0, 2, recipes)
	if !ok {
		t.Fatal("expected two-recipe window to encode")
	}
	if message != "cube m_info 0 2 10,1/5@20,2" {
		t.Fatalf("unexpected multi m_info command: %q", message)
	}
	if _, ok := FormatMaterialInfoCommand(2, 1, recipes); ok {
		t.Fatal("expected past-end start index to fail closed")
	}
	if _, ok := FormatMaterialInfoCommand(0, 0, recipes); ok {
		t.Fatal("expected zero request count to fail closed")
	}
	oversize := make([]Recipe, 0, 40)
	for i := 0; i < 40; i++ {
		oversize = append(oversize, Recipe{
			Reward:    Reward{Vnum: 1, Count: 1},
			Materials: []Material{{Vnum: 100000 + uint32(i), Count: 9999}, {Vnum: 200000 + uint32(i), Count: 9999}},
			Gold:      99999999,
		})
	}
	if _, ok := FormatMaterialInfoCommand(0, 40, oversize); ok {
		t.Fatal("expected oversize material entry text to fail closed")
	}
}

func TestMatchSimpleRecipeGoldMatchesBootstrapCoversSurplusAndRejectsPartial(t *testing.T) {
	recipes := BootstrapSnapshot().NPCs[0].Recipes
	gold, ok := MatchSimpleRecipeGold(recipes, []BoundMaterial{{Vnum: 27002, Count: 2}})
	if !ok || gold != 100 {
		t.Fatalf("expected bootstrap bound materials to match gold 100, got gold=%d ok=%v", gold, ok)
	}
	gold, ok = MatchSimpleRecipeGold(recipes, []BoundMaterial{
		{Vnum: 27002, Count: 1},
		{Vnum: 27002, Count: 1},
	})
	if !ok || gold != 100 {
		t.Fatalf("expected aggregated bootstrap materials to match gold 100, got gold=%d ok=%v", gold, ok)
	}
	if _, ok := MatchSimpleRecipeGold(recipes, []BoundMaterial{{Vnum: 27002, Count: 1}}); ok {
		t.Fatal("expected partial materials to fail closed")
	}
	gold, ok = MatchSimpleRecipeGold(recipes, []BoundMaterial{
		{Vnum: 27002, Count: 2},
		{Vnum: 27003, Count: 1},
	})
	if !ok || gold != 100 {
		t.Fatalf("expected surplus/extra bound materials to still cover recipe gold 100, got gold=%d ok=%v", gold, ok)
	}
	gold, ok = MatchSimpleRecipeGold(recipes, []BoundMaterial{{Vnum: 27002, Count: 4}})
	if !ok || gold != 100 {
		t.Fatalf("expected surplus same-vnum materials to cover recipe gold 100, got gold=%d ok=%v", gold, ok)
	}
	if _, ok := MatchSimpleRecipeGold(recipes, nil); ok {
		t.Fatal("expected empty bindings to fail closed")
	}
}

func TestFormatCubeInfoCommand(t *testing.T) {
	if got := FormatCubeInfoCommand(100); got != "cube info 100 0 0" {
		t.Fatalf("unexpected cube info command: %q", got)
	}
	if got := FormatCubeInfoCommand(0); got != "cube info 0 0 0" {
		t.Fatalf("unexpected zero cube info command: %q", got)
	}
}

func TestFormatCubeSuccessCommand(t *testing.T) {
	if got := FormatCubeSuccessCommand(27001, 1); got != "cube success 27001 1" {
		t.Fatalf("unexpected cube success command: %q", got)
	}
}

func TestFormatCubeFailCommand(t *testing.T) {
	if got := FormatCubeFailCommand(); got != "cube fail" {
		t.Fatalf("unexpected cube fail command: %q", got)
	}
}

func TestMatchSimpleRecipeReturnsBootstrapPercent100(t *testing.T) {
	recipes := BootstrapSnapshot().NPCs[0].Recipes
	recipe, ok := MatchSimpleRecipe(recipes, []BoundMaterial{{Vnum: 27002, Count: 2}})
	if !ok {
		t.Fatal("expected bootstrap bound materials to match")
	}
	if recipe.Percent != 100 || recipe.Gold != 100 || recipe.Reward.Vnum != 27001 || recipe.Reward.Count != 1 {
		t.Fatalf("unexpected matched recipe: %+v", recipe)
	}
	if _, ok := MatchSimpleRecipe(recipes, []BoundMaterial{{Vnum: 27002, Count: 1}}); ok {
		t.Fatal("expected partial materials to fail closed")
	}
	recipe, ok = MatchSimpleRecipe(recipes, []BoundMaterial{{Vnum: 27002, Count: 4}})
	if !ok || recipe.Percent != 100 {
		t.Fatalf("expected surplus materials to cover bootstrap recipe, got ok=%v recipe=%+v", ok, recipe)
	}
}
