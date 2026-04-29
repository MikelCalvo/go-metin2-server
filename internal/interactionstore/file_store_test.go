package interactionstore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func testMerchantCatalogDefinition() Definition {
	return Definition{
		Kind:  KindShopPreview,
		Ref:   "npc:merchant",
		Title: "Village Merchant",
		Catalog: []MerchantCatalogEntry{
			{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
			{Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
		},
	}
}

func TestFileStoreSaveThenLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	want := Snapshot{Definitions: []Definition{
		{Kind: KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
		testMerchantCatalogDefinition(),
		{Kind: KindTalk, Ref: "npc:village_guard", Text: "VillageGuard : Keep your blade sharp."},
		{Kind: KindWarp, Ref: "npc:teleporter", MapIndex: 42, X: 1700, Y: 2800},
	}}

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
}

func TestFileStoreSaveWritesDeterministicSortedSnapshotAndReplacesPreviousContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	first := Snapshot{Definitions: []Definition{
		{Kind: KindTalk, Ref: "npc:village_guard", Text: "VillageGuard : Keep your blade sharp."},
		{Kind: KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
		{
			Kind:  KindShopPreview,
			Ref:   "npc:merchant",
			Title: "Village Merchant",
			Catalog: []MerchantCatalogEntry{
				{Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
				{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
			},
		},
		{Kind: KindTalk, Ref: "npc:blacksmith", Text: "Blacksmith : Bring me good ore."},
		{Kind: KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 42, X: 1700, Y: 2800},
	}}

	if err := store.Save(first); err != nil {
		t.Fatalf("save first snapshot: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted snapshot: %v", err)
	}
	wantFirst := "{\n  \"definitions\": [\n    {\n      \"kind\": \"info\",\n      \"ref\": \"lore:alchemist\",\n      \"text\": \"The alchemist studies forgotten herbs.\"\n    },\n    {\n      \"kind\": \"shop_preview\",\n      \"ref\": \"npc:merchant\",\n      \"title\": \"Village Merchant\",\n      \"catalog\": [\n        {\n          \"slot\": 0,\n          \"item_vnum\": 27001,\n          \"price\": 50,\n          \"count\": 1\n        },\n        {\n          \"slot\": 1,\n          \"item_vnum\": 11200,\n          \"price\": 500,\n          \"count\": 1\n        }\n      ]\n    },\n    {\n      \"kind\": \"talk\",\n      \"ref\": \"npc:blacksmith\",\n      \"text\": \"Blacksmith : Bring me good ore.\"\n    },\n    {\n      \"kind\": \"talk\",\n      \"ref\": \"npc:village_guard\",\n      \"text\": \"VillageGuard : Keep your blade sharp.\"\n    },\n    {\n      \"kind\": \"warp\",\n      \"ref\": \"npc:teleporter\",\n      \"text\": \"Step through the gate.\",\n      \"map_index\": 42,\n      \"x\": 1700,\n      \"y\": 2800\n    }\n  ]\n}\n"
	if string(raw) != wantFirst {
		t.Fatalf("unexpected deterministic first snapshot:\n got: %s\nwant: %s", string(raw), wantFirst)
	}

	second := Snapshot{Definitions: []Definition{{Kind: KindInfo, Ref: "lore:merchant", Text: "The merchant knows every road."}}}
	if err := store.Save(second); err != nil {
		t.Fatalf("save replacement snapshot: %v", err)
	}
	raw, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replacement snapshot: %v", err)
	}
	wantSecond := "{\n  \"definitions\": [\n    {\n      \"kind\": \"info\",\n      \"ref\": \"lore:merchant\",\n      \"text\": \"The merchant knows every road.\"\n    }\n  ]\n}\n"
	if string(raw) != wantSecond {
		t.Fatalf("unexpected replacement snapshot:\n got: %s\nwant: %s", string(raw), wantSecond)
	}
}

func TestFileStoreLoadReturnsNotFoundForMissingSnapshot(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "state", "interaction-definitions.json"))
	_, err := store.Load()
	if !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected ErrSnapshotNotFound, got %v", err)
	}
}

func TestFileStoreLoadRejectsMalformedOrInvalidSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write malformed snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for malformed json, got %v", err)
	}

	invalidKind := Snapshot{Definitions: []Definition{{Kind: "shop", Ref: "npc:merchant", Text: "not yet"}}}
	if err := store.Save(invalidKind); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for invalid kind, got %v", err)
	}
	blankRef := Snapshot{Definitions: []Definition{{Kind: KindInfo, Ref: "", Text: "missing ref"}}}
	if err := store.Save(blankRef); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for blank ref, got %v", err)
	}
	blankText := Snapshot{Definitions: []Definition{{Kind: KindTalk, Ref: "npc:village_guard", Text: "   "}}}
	if err := store.Save(blankText); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for blank text, got %v", err)
	}
	blankShopPreviewTitle := Snapshot{Definitions: []Definition{{Kind: KindShopPreview, Ref: "npc:merchant", Title: "   ", Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}}}}
	if err := store.Save(blankShopPreviewTitle); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for blank shop preview title, got %v", err)
	}
	emptyShopPreviewCatalog := Snapshot{Definitions: []Definition{{Kind: KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant"}}}
	if err := store.Save(emptyShopPreviewCatalog); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for empty shop preview catalog, got %v", err)
	}
	shopPreviewLegacyText := Snapshot{Definitions: []Definition{{Kind: KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant", Text: "Browse wares.", Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}}}}
	if err := store.Save(shopPreviewLegacyText); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for legacy shop preview text, got %v", err)
	}
	shopPreviewSparseSlots := Snapshot{Definitions: []Definition{{Kind: KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant", Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}, {Slot: 2, ItemVnum: 11200, Price: 500, Count: 1}}}}}
	if err := store.Save(shopPreviewSparseSlots); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for sparse shop preview slots, got %v", err)
	}
	shopPreviewZeroPrice := Snapshot{Definitions: []Definition{{Kind: KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant", Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 0, Count: 1}}}}}
	if err := store.Save(shopPreviewZeroPrice); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for zero-price shop preview entry, got %v", err)
	}
	warpMissingMap := Snapshot{Definitions: []Definition{{Kind: KindWarp, Ref: "npc:teleporter", X: 1700, Y: 2800}}}
	if err := store.Save(warpMissingMap); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for warp definition missing map index, got %v", err)
	}
	warpMissingCoordinates := Snapshot{Definitions: []Definition{{Kind: KindWarp, Ref: "npc:teleporter", MapIndex: 42, X: 0, Y: 2800}}}
	if err := store.Save(warpMissingCoordinates); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for warp definition missing required coordinates, got %v", err)
	}
	duplicate := Snapshot{Definitions: []Definition{
		{Kind: KindInfo, Ref: "lore:alchemist", Text: "one"},
		{Kind: KindInfo, Ref: "lore:alchemist", Text: "two"},
	}}
	if err := store.Save(duplicate); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for duplicate definition key, got %v", err)
	}
}
