package interactionstore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMemoryStoreSaveLoadWithoutFilesystem(t *testing.T) {
	dir := t.TempDir()
	store := NewMemoryStore()

	if _, err := store.Load(); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected ErrSnapshotNotFound before save, got %v", err)
	}

	seed := Snapshot{Definitions: []Definition{
		{Kind: KindTalk, Ref: "npc:village_guard", Text: "VillageGuard : Keep your blade sharp."},
		{Kind: KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant", Catalog: []MerchantCatalogEntry{
			{Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
			{Slot: 0, ItemVnum: 27001, Price: 50, Count: 2},
		}},
	}}
	if err := store.Save(seed); err != nil {
		t.Fatalf("save memory interactions: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load memory interactions: %v", err)
	}
	if len(loaded.Definitions) != 2 || loaded.Definitions[0].Kind != KindShopPreview || loaded.Definitions[1].Kind != KindTalk {
		t.Fatalf("unexpected loaded definitions: %#v", loaded.Definitions)
	}
	if len(loaded.Definitions[0].Catalog) != 2 || loaded.Definitions[0].Catalog[0].Slot != 0 {
		t.Fatalf("expected sorted catalog to round-trip, got %#v", loaded.Definitions[0].Catalog)
	}

	loaded.Definitions[0].Title = "mutated"
	loaded.Definitions[0].Catalog[0].Price = 9999

	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("reload memory interactions: %v", err)
	}
	if reloaded.Definitions[0].Title != "Village Merchant" {
		t.Fatalf("memory store leaked caller mutation into title: %#v", reloaded.Definitions[0])
	}
	if reloaded.Definitions[0].Catalog[0].Price != 50 {
		t.Fatalf("memory store leaked caller mutation into catalog: %#v", reloaded.Definitions[0].Catalog)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("memory store wrote filesystem entries: %#v", entries)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, ".interaction-definitions-*.json")); err != nil || len(matches) != 0 {
		t.Fatalf("memory store created crash-temp shaped files: matches=%v err=%v", matches, err)
	}
}

func TestMemoryStoreRejectsInvalidSave(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Save(Snapshot{Definitions: []Definition{{Kind: KindInfo, Ref: "lore:empty"}}}); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for empty info text, got %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("invalid save must leave store uncommitted, got %v", err)
	}
}

func TestMemoryStoreRoundTripMatchesFileStore(t *testing.T) {
	snapshot := Snapshot{Definitions: []Definition{
		{Kind: KindTalk, Ref: "npc:village_guard", Text: "VillageGuard : Keep your blade sharp."},
		{Kind: KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant", Catalog: []MerchantCatalogEntry{
			{Slot: 0, ItemVnum: 27001, Price: 50, Count: 2},
			{Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
		}},
	}}

	fileStore := NewFileStore(filepath.Join(t.TempDir(), "interaction-definitions.json"))
	memoryStore := NewMemoryStore()
	if err := fileStore.Save(snapshot); err != nil {
		t.Fatalf("save file interactions: %v", err)
	}
	if err := memoryStore.Save(snapshot); err != nil {
		t.Fatalf("save memory interactions: %v", err)
	}

	fileLoaded, err := fileStore.Load()
	if err != nil {
		t.Fatalf("load file interactions: %v", err)
	}
	memoryLoaded, err := memoryStore.Load()
	if err != nil {
		t.Fatalf("load memory interactions: %v", err)
	}
	if !reflect.DeepEqual(fileLoaded, memoryLoaded) {
		t.Fatalf("interaction snapshot mismatch:\n file: %#v\nmemory: %#v", fileLoaded, memoryLoaded)
	}
}

func TestMemoryStoreSatisfiesStore(t *testing.T) {
	var store Store = NewMemoryStore()
	if _, err := store.Load(); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected ErrSnapshotNotFound from empty memory store, got %v", err)
	}
}
