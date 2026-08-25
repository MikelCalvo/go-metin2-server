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
		}, {
			Reward:    Reward{Vnum: 11200, Count: 1},
			Materials: []Material{},
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
				Recipes: []Recipe{{Reward: Reward{Vnum: 1, Count: 1}, Materials: []Material{}}},
			}}},
		},
		{
			name: "zero reward vnum",
			snapshot: Snapshot{NPCs: []NPCRecipes{{
				NPCVnum: 20022,
				Recipes: []Recipe{{Reward: Reward{Vnum: 0, Count: 1}, Materials: []Material{}}},
			}}},
		},
		{
			name: "zero reward count",
			snapshot: Snapshot{NPCs: []NPCRecipes{{
				NPCVnum: 20022,
				Recipes: []Recipe{{Reward: Reward{Vnum: 1, Count: 0}, Materials: []Material{}}},
			}}},
		},
		{
			name: "duplicate npc",
			snapshot: Snapshot{NPCs: []NPCRecipes{
				{NPCVnum: 20022, Recipes: []Recipe{{Reward: Reward{Vnum: 1, Count: 1}, Materials: []Material{}}}},
				{NPCVnum: 20022, Recipes: []Recipe{{Reward: Reward{Vnum: 2, Count: 1}, Materials: []Material{}}}},
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
		t.Fatalf("unexpected bootstrap recipes: %#v", recipes)
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
