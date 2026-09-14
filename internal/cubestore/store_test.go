package cubestore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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

func TestFormatCubeListInfoMessage(t *testing.T) {
	if got := FormatCubeListInfoMessage(0, 5, "Small Blue Potion"); got != "cube[0]: inventory[5]: Small Blue Potion" {
		t.Fatalf("unexpected cube list info: %q", got)
	}
	if got := FormatCubeListInfoMessage(3, 12, ""); got != "cube[3]: inventory[12]: " {
		t.Fatalf("unexpected empty-name cube list info: %q", got)
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

func sampleAuthoredCubeSnapshot() Snapshot {
	return Snapshot{NPCs: []NPCRecipes{{
		NPCVnum: BootstrapDefaultNPCVnum,
		Recipes: []Recipe{{
			Reward:    Reward{Vnum: 27001, Count: 1},
			Materials: []Material{{Vnum: 27002, Count: 2}},
			Gold:      100,
			Percent:   100,
		}, {
			Reward:    Reward{Vnum: 11200, Count: 1},
			Materials: []Material{},
			Percent:   80,
		}},
	}}}
}

func directoryEntryNames(entries []os.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestFileStoreValidateTreatsMissingSnapshotAsEmptyStore(t *testing.T) {
	restore := DisableDurableSyncForTest()
	defer restore()

	store := NewFileStore(filepath.Join(t.TempDir(), "missing", "cube-recipes.json"))
	summary, err := store.Validate()
	if err != nil {
		t.Fatalf("validate missing cube recipe store: %v", err)
	}
	want := SnapshotSummary{NPCVnums: []uint32{}}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected missing-store summary: got %#v want %#v", summary, want)
	}
}

func TestFileStoreBackupToWritesCommittedSnapshotAndDeterministicManifest(t *testing.T) {
	restore := DisableDurableSyncForTest()
	defer restore()

	path := filepath.Join(t.TempDir(), "state", "cube-recipes.json")
	store := NewFileStore(path)
	snapshot := sampleAuthoredCubeSnapshot()
	if err := store.Save(snapshot); err != nil {
		t.Fatalf("save cube recipe snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), ".cube-recipes-crashed.json"), []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write crash temp file: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "cube-recipe-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup cube recipe store: %v", err)
	}

	backup := NewFileStore(filepath.Join(backupDir, "cube-recipes.json"))
	got, err := backup.Load()
	if err != nil {
		t.Fatalf("load backup snapshot: %v", err)
	}
	wantSnapshot := NormalizeSnapshot(snapshot)
	if !reflect.DeepEqual(got, wantSnapshot) {
		t.Fatalf("unexpected backup snapshot: got %#v want %#v", got, wantSnapshot)
	}
	if _, err := os.Stat(filepath.Join(backupDir, ".cube-recipes-crashed.json")); !os.IsNotExist(err) {
		t.Fatalf("expected source crash temp file to be omitted from backup, stat err=%v", err)
	}

	rawManifest, err := os.ReadFile(filepath.Join(backupDir, BackupManifestFilename))
	if err != nil {
		t.Fatalf("read backup manifest: %v", err)
	}
	var manifest BackupManifest
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		t.Fatalf("decode backup manifest: %v", err)
	}
	if manifest.Format != BackupManifestFormat {
		t.Fatalf("unexpected manifest format: got %q want %q", manifest.Format, BackupManifestFormat)
	}
	wantSummary := SnapshotSummary{NPCCount: 1, RecipeCount: 2, NPCVnums: []uint32{BootstrapDefaultNPCVnum}}
	if !reflect.DeepEqual(manifest.Summary, wantSummary) {
		t.Fatalf("unexpected manifest summary: got %#v want %#v", manifest.Summary, wantSummary)
	}
	if len(manifest.Files) != 1 || manifest.Files[0].Filename != "cube-recipes.json" {
		t.Fatalf("unexpected manifest files: %#v", manifest.Files)
	}
	rawSnapshot, err := os.ReadFile(filepath.Join(backupDir, manifest.Files[0].Filename))
	if err != nil {
		t.Fatalf("read manifest snapshot: %v", err)
	}
	checksum := sha256.Sum256(rawSnapshot)
	if gotChecksum := hex.EncodeToString(checksum[:]); gotChecksum != manifest.Files[0].SHA256 {
		t.Fatalf("unexpected manifest checksum: got %s want %s", manifest.Files[0].SHA256, gotChecksum)
	}
	if int64(len(rawSnapshot)) != manifest.Files[0].SizeBytes {
		t.Fatalf("unexpected manifest size: got %d want %d", manifest.Files[0].SizeBytes, len(rawSnapshot))
	}
}

func TestFileStoreBackupToTreatsMissingSnapshotAsEmptyAuthoredStore(t *testing.T) {
	restore := DisableDurableSyncForTest()
	defer restore()

	store := NewFileStore(filepath.Join(t.TempDir(), "missing", "cube-recipes.json"))
	backupDir := filepath.Join(t.TempDir(), "cube-recipe-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup missing cube recipe store: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backupDir, "cube-recipes.json")); !os.IsNotExist(err) {
		t.Fatalf("expected missing snapshot backup to omit committed recipe file, stat err=%v", err)
	}
	rawManifest, err := os.ReadFile(filepath.Join(backupDir, BackupManifestFilename))
	if err != nil {
		t.Fatalf("read missing-store backup manifest: %v", err)
	}
	var manifest BackupManifest
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		t.Fatalf("decode missing-store backup manifest: %v", err)
	}
	want := BackupManifest{Format: BackupManifestFormat, Summary: SnapshotSummary{NPCVnums: []uint32{}}, Files: []BackupManifestFile{}}
	if !reflect.DeepEqual(manifest, want) {
		t.Fatalf("unexpected missing-store backup manifest: got %#v want %#v", manifest, want)
	}
}

func TestFileStoreBackupToRollsBackSnapshotWhenSaveSyncFailsAfterCommit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "cube-recipes.json")
	store := NewFileStore(path)
	if err := store.Save(sampleAuthoredCubeSnapshot()); err != nil {
		t.Fatalf("save source cube recipe snapshot: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "cube-recipe-backup")
	injectedErr := errors.New("injected cube-recipe backup snapshot sync failure")
	originalSyncStoreDir := syncStoreDir
	t.Cleanup(func() { syncStoreDir = originalSyncStoreDir })
	backupDirSyncCalls := 0
	syncStoreDir = func(path string) error {
		if path == backupDir {
			backupDirSyncCalls++
			if backupDirSyncCalls == 1 {
				return injectedErr
			}
			return nil
		}
		return syncDir(path)
	}

	err := store.BackupTo(backupDir)
	if !errors.Is(err, injectedErr) {
		t.Fatalf("expected injected snapshot sync error, got %v", err)
	}
	entries, readErr := os.ReadDir(backupDir)
	if readErr != nil {
		t.Fatalf("read backup dir after failed backup: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("expected failed snapshot-save sync to roll back committed backup files, got %#v", directoryEntryNames(entries))
	}
}

func TestFileStoreBackupToRollsBackSnapshotAndManifestWhenFinalSyncFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "cube-recipes.json")
	store := NewFileStore(path)
	if err := store.Save(sampleAuthoredCubeSnapshot()); err != nil {
		t.Fatalf("save source cube recipe snapshot: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "cube-recipe-backup")
	injectedErr := errors.New("injected cube-recipe final backup sync failure")
	originalSyncStoreDir := syncStoreDir
	t.Cleanup(func() { syncStoreDir = originalSyncStoreDir })
	backupDirSyncCalls := 0
	syncStoreDir = func(path string) error {
		if path == backupDir {
			backupDirSyncCalls++
			if backupDirSyncCalls == 2 {
				return injectedErr
			}
			return nil
		}
		return syncDir(path)
	}

	err := store.BackupTo(backupDir)
	if !errors.Is(err, injectedErr) {
		t.Fatalf("expected injected final sync error, got %v", err)
	}
	entries, readErr := os.ReadDir(backupDir)
	if readErr != nil {
		t.Fatalf("read backup dir after failed final sync: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("expected failed final sync to roll back backup snapshot and manifest, got %#v", directoryEntryNames(entries))
	}
}

func TestFileStoreBackupToRejectsStaleActiveBackupManifestBeforeCreatingDestination(t *testing.T) {
	restore := DisableDurableSyncForTest()
	defer restore()

	path := filepath.Join(t.TempDir(), "state", "cube-recipes.json")
	store := NewFileStore(path)
	if err := store.Save(sampleAuthoredCubeSnapshot()); err != nil {
		t.Fatalf("save cube recipe snapshot: %v", err)
	}
	if err := writeBackupManifest(filepath.Dir(path), filepath.Base(path), SnapshotSummary{NPCCount: 1, RecipeCount: 2, NPCVnums: []uint32{BootstrapDefaultNPCVnum}}, true); err != nil {
		t.Fatalf("write active backup manifest: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"npcs":[{"npc_vnum":20022,"recipes":[{"reward":{"vnum":27001,"count":1},"materials":[{"vnum":27002,"count":2}],"gold":100,"percent":50}]}]}`), 0o644); err != nil {
		t.Fatalf("tamper active cube recipe snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "cube-recipe-backup")

	err := store.BackupTo(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest before backup, got %v", err)
	}
	if _, statErr := os.Stat(backupDir); !os.IsNotExist(statErr) {
		t.Fatalf("expected rejected backup not to create destination, stat err=%v", statErr)
	}
}

func TestFileStoreValidateBackupFromValidatesManifestWithoutMutatingTarget(t *testing.T) {
	restore := DisableDurableSyncForTest()
	defer restore()

	store := NewFileStore(filepath.Join(t.TempDir(), "state", "cube-recipes.json"))
	if err := store.Save(sampleAuthoredCubeSnapshot()); err != nil {
		t.Fatalf("save cube recipe snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "cube-recipe-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup cube recipe store: %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "restore-target", "cube-recipes.json")
	target := NewFileStore(targetPath)

	summary, err := target.ValidateBackupFrom(backupDir)
	if err != nil {
		t.Fatalf("validate cube recipe backup: %v", err)
	}
	want := SnapshotSummary{NPCCount: 1, RecipeCount: 2, NPCVnums: []uint32{BootstrapDefaultNPCVnum}}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected backup validation summary: got %#v want %#v", summary, want)
	}
	if _, err := os.Stat(filepath.Dir(targetPath)); !os.IsNotExist(err) {
		t.Fatalf("expected dry-run validation not to create target dir, stat err=%v", err)
	}
}

func TestFileStoreValidateBackupFromReportsIgnoredCrashTempFiles(t *testing.T) {
	restore := DisableDurableSyncForTest()
	defer restore()

	store := NewFileStore(filepath.Join(t.TempDir(), "state", "cube-recipes.json"))
	if err := store.Save(sampleAuthoredCubeSnapshot()); err != nil {
		t.Fatalf("save cube recipe snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "cube-recipe-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup cube recipe store: %v", err)
	}
	for _, name := range []string{".cube-recipes-zeta.json", ".cube-recipes-alpha.json"} {
		if err := os.WriteFile(filepath.Join(backupDir, name), []byte(`{"not":"committed"}`), 0o644); err != nil {
			t.Fatalf("write backup crash temp %s: %v", name, err)
		}
	}

	summary, err := NewFileStore(filepath.Join(t.TempDir(), "target", "cube-recipes.json")).ValidateBackupFrom(backupDir)
	if err != nil {
		t.Fatalf("validate cube recipe backup with crash temps: %v", err)
	}
	want := SnapshotSummary{
		NPCCount:       1,
		RecipeCount:    2,
		NPCVnums:       []uint32{BootstrapDefaultNPCVnum},
		CrashTempCount: 2,
		CrashTempFiles: []string{".cube-recipes-alpha.json", ".cube-recipes-zeta.json"},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected backup validation summary with crash temps: got %#v want %#v", summary, want)
	}
}

func TestFileStoreValidateBackupFromRejectsChecksumMismatch(t *testing.T) {
	restore := DisableDurableSyncForTest()
	defer restore()

	store := NewFileStore(filepath.Join(t.TempDir(), "state", "cube-recipes.json"))
	if err := store.Save(sampleAuthoredCubeSnapshot()); err != nil {
		t.Fatalf("save cube recipe snapshot: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "cube-recipe-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup cube recipe store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "cube-recipes.json"), []byte(`{"npcs":[{"npc_vnum":20022,"recipes":[{"reward":{"vnum":27001,"count":1},"materials":[{"vnum":27002,"count":2}],"gold":100,"percent":50}]}]}`), 0o644); err != nil {
		t.Fatalf("tamper backup snapshot: %v", err)
	}

	_, err := NewFileStore(filepath.Join(t.TempDir(), "target", "cube-recipes.json")).ValidateBackupFrom(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest for checksum mismatch, got %v", err)
	}
}

func TestFileStoreRestoreFromRestoresManifestedBackupIntoEmptyStore(t *testing.T) {
	restore := DisableDurableSyncForTest()
	defer restore()

	source := NewFileStore(filepath.Join(t.TempDir(), "state", "cube-recipes.json"))
	snapshot := sampleAuthoredCubeSnapshot()
	if err := source.Save(snapshot); err != nil {
		t.Fatalf("save source cube recipes: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "cube-recipe-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup cube recipes: %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "restore-target", "cube-recipes.json")
	target := NewFileStore(targetPath)

	if err := target.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore cube recipe backup: %v", err)
	}
	restored, err := target.Load()
	if err != nil {
		t.Fatalf("load restored cube recipe snapshot: %v", err)
	}
	wantSnapshot := NormalizeSnapshot(snapshot)
	if !reflect.DeepEqual(restored, wantSnapshot) {
		t.Fatalf("unexpected restored cube recipe snapshot:\n got: %#v\nwant: %#v", restored, wantSnapshot)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(targetPath), BackupManifestFilename)); err != nil {
		t.Fatalf("expected restored cube recipe manifest: %v", err)
	}
	summary, err := target.ValidateBackupFrom(filepath.Dir(targetPath))
	if err != nil {
		t.Fatalf("validate restored cube recipe manifest: %v", err)
	}
	wantSummary := SnapshotSummary{NPCCount: 1, RecipeCount: 2, NPCVnums: []uint32{BootstrapDefaultNPCVnum}}
	if !reflect.DeepEqual(summary, wantSummary) {
		t.Fatalf("unexpected restored manifest summary: got %#v want %#v", summary, wantSummary)
	}
}

func TestFileStoreSaveRemovesStaleBackupManifestAfterMutation(t *testing.T) {
	restore := DisableDurableSyncForTest()
	defer restore()

	source := NewFileStore(filepath.Join(t.TempDir(), "source", "cube-recipes.json"))
	if err := source.Save(sampleAuthoredCubeSnapshot()); err != nil {
		t.Fatalf("save source cube recipes: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "cube-recipe-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup cube recipes: %v", err)
	}

	targetPath := filepath.Join(t.TempDir(), "restore-target", "cube-recipes.json")
	restored := NewFileStore(targetPath)
	if err := restored.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore cube recipes: %v", err)
	}
	manifestPath := filepath.Join(filepath.Dir(targetPath), BackupManifestFilename)
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("expected restored manifest before mutation: %v", err)
	}

	mutated := Snapshot{NPCs: []NPCRecipes{{
		NPCVnum: BootstrapDefaultNPCVnum,
		Recipes: []Recipe{{
			Reward:    Reward{Vnum: 27003, Count: 1},
			Materials: []Material{{Vnum: 27002, Count: 1}},
			Percent:   100,
		}},
	}}}
	if err := restored.Save(mutated); err != nil {
		t.Fatalf("save mutated restored cube recipes: %v", err)
	}
	if _, err := os.Stat(manifestPath); !os.IsNotExist(err) {
		t.Fatalf("expected stale restored backup manifest to be removed after cube-recipe mutation, stat err=%v", err)
	}
	got, err := restored.Load()
	if err != nil {
		t.Fatalf("load mutated restored cube recipes: %v", err)
	}
	if !reflect.DeepEqual(got, NormalizeSnapshot(mutated)) {
		t.Fatalf("unexpected mutated cube recipe snapshot: got %#v want %#v", got, NormalizeSnapshot(mutated))
	}
}

func TestFileStoreRestoreFromRestoresMissingSnapshotBackupAsEmptyStore(t *testing.T) {
	restore := DisableDurableSyncForTest()
	defer restore()

	source := NewFileStore(filepath.Join(t.TempDir(), "missing", "cube-recipes.json"))
	backupDir := filepath.Join(t.TempDir(), "cube-recipe-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup missing cube recipe snapshot: %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "restore-target", "cube-recipes.json")
	target := NewFileStore(targetPath)

	if err := target.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore empty cube recipe backup: %v", err)
	}
	if _, err := target.Load(); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected restored empty cube recipe store to omit snapshot, got %v", err)
	}
	summary, err := target.ValidateBackupFrom(filepath.Dir(targetPath))
	if err != nil {
		t.Fatalf("validate restored empty cube recipe manifest: %v", err)
	}
	want := SnapshotSummary{NPCVnums: []uint32{}}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected restored empty backup summary: got %#v want %#v", summary, want)
	}
}

func TestFileStoreRestoreFromRejectsNonEmptyTargetStore(t *testing.T) {
	restore := DisableDurableSyncForTest()
	defer restore()

	source := NewFileStore(filepath.Join(t.TempDir(), "state", "cube-recipes.json"))
	if err := source.Save(sampleAuthoredCubeSnapshot()); err != nil {
		t.Fatalf("save source cube recipes: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "cube-recipe-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup cube recipes: %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "restore-target", "cube-recipes.json")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("create restore target dir: %v", err)
	}
	stale := filepath.Join(filepath.Dir(targetPath), "stale.json")
	if err := os.WriteFile(stale, []byte(`{"stale":true}`), 0o644); err != nil {
		t.Fatalf("write stale restore target file: %v", err)
	}

	err := NewFileStore(targetPath).RestoreFrom(backupDir)
	if !errors.Is(err, ErrRestoreDirNotEmpty) {
		t.Fatalf("expected ErrRestoreDirNotEmpty for non-empty target, got %v", err)
	}
	if raw, readErr := os.ReadFile(stale); readErr != nil || string(raw) != `{"stale":true}` {
		t.Fatalf("expected stale target file to remain untouched, readErr=%v raw=%q", readErr, string(raw))
	}
}

func TestFileStoreRestoreFromRejectsTargetInsideBackupSource(t *testing.T) {
	restore := DisableDurableSyncForTest()
	defer restore()

	source := NewFileStore(filepath.Join(t.TempDir(), "state", "cube-recipes.json"))
	if err := source.Save(sampleAuthoredCubeSnapshot()); err != nil {
		t.Fatalf("save source cube recipes: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "cube-recipe-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("backup cube recipes: %v", err)
	}
	targetPath := filepath.Join(backupDir, "nested-restore", "cube-recipes.json")

	err := NewFileStore(targetPath).RestoreFrom(backupDir)
	if !errors.Is(err, ErrRestoreDirInsideSource) {
		t.Fatalf("expected ErrRestoreDirInsideSource for nested restore target, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Dir(targetPath)); !os.IsNotExist(statErr) {
		t.Fatalf("expected nested restore target not to be created, stat err=%v", statErr)
	}
}
