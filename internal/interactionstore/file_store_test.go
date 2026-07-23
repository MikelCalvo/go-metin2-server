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

func TestValidRefAcceptsOnlyNamespacedLowerSnakeInteractionRefs(t *testing.T) {
	for _, ref := range []string{"lore:alchemist", "npc:village_guard", "old:lore", "npc:qa_merchant2"} {
		if !ValidRef(ref) {
			t.Fatalf("expected interaction ref %q to be valid", ref)
		}
	}

	for _, ref := range []string{"", "alchemist", "npc/village_guard", "npc:VillageGuard", "npc:village-guard", "npc:village.guard", "npc:village guard", "npc:", ":merchant", "npc:merchant:extra"} {
		if ValidRef(ref) {
			t.Fatalf("expected interaction ref %q to be invalid", ref)
		}
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

func TestFileStoreSaveThenLoadMerchantCatalogKeepsStableBuySlotAddressing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Definitions: []Definition{testMerchantCatalogDefinition()}}); err != nil {
		t.Fatalf("save merchant snapshot: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load merchant snapshot: %v", err)
	}
	if len(loaded.Definitions) != 1 {
		t.Fatalf("expected 1 merchant definition, got %d", len(loaded.Definitions))
	}
	catalog := loaded.Definitions[0].Catalog
	if len(catalog) != 2 {
		t.Fatalf("expected 2 merchant catalog entries, got %d", len(catalog))
	}
	if catalog[0].Slot != 0 || catalog[0].ItemVnum != 27001 || catalog[0].Price != 50 || catalog[0].Count != 1 {
		t.Fatalf("unexpected merchant catalog slot 0 after round-trip: %+v", catalog[0])
	}
	if catalog[1].Slot != 1 || catalog[1].ItemVnum != 11200 || catalog[1].Price != 500 || catalog[1].Count != 1 {
		t.Fatalf("unexpected merchant catalog slot 1 after round-trip: %+v", catalog[1])
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

	if err := os.WriteFile(path, []byte("{\"definitions\":[],\"unknown\":true}"), 0o644); err != nil {
		t.Fatalf("write unknown-field snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for unknown snapshot field, got %v", err)
	}

	if err := os.WriteFile(path, []byte("{\"definitions\":[]}{}"), 0o644); err != nil {
		t.Fatalf("write trailing-json snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for trailing json, got %v", err)
	}

	invalidKind := Snapshot{Definitions: []Definition{{Kind: "shop", Ref: "npc:merchant", Text: "not yet"}}}
	if err := store.Save(invalidKind); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for invalid kind, got %v", err)
	}
	blankRef := Snapshot{Definitions: []Definition{{Kind: KindInfo, Ref: "", Text: "missing ref"}}}
	if err := store.Save(blankRef); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for blank ref, got %v", err)
	}
	refWithoutNamespace := Snapshot{Definitions: []Definition{{Kind: KindInfo, Ref: "alchemist", Text: "missing namespace"}}}
	if err := store.Save(refWithoutNamespace); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for interaction ref without namespace, got %v", err)
	}
	pathAmbiguousRef := Snapshot{Definitions: []Definition{{Kind: KindTalk, Ref: "npc/village_guard", Text: "path ambiguous ref"}}}
	if err := store.Save(pathAmbiguousRef); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for path-ambiguous interaction ref, got %v", err)
	}
	whitespaceRef := Snapshot{Definitions: []Definition{{Kind: KindInfo, Ref: "lore:al chemist", Text: "whitespace ref"}}}
	if err := store.Save(whitespaceRef); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for interaction ref with embedded whitespace, got %v", err)
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
	oversizedShopPreviewCatalog := Snapshot{Definitions: []Definition{{
		Kind:  KindShopPreview,
		Ref:   "npc:merchant",
		Title: "Village Merchant",
		Catalog: []MerchantCatalogEntry{
			{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}, {Slot: 1, ItemVnum: 27002, Price: 50, Count: 1},
			{Slot: 2, ItemVnum: 27003, Price: 50, Count: 1}, {Slot: 3, ItemVnum: 27004, Price: 50, Count: 1},
			{Slot: 4, ItemVnum: 27005, Price: 50, Count: 1}, {Slot: 5, ItemVnum: 27006, Price: 50, Count: 1},
			{Slot: 6, ItemVnum: 27007, Price: 50, Count: 1}, {Slot: 7, ItemVnum: 27008, Price: 50, Count: 1},
			{Slot: 8, ItemVnum: 27009, Price: 50, Count: 1}, {Slot: 9, ItemVnum: 27010, Price: 50, Count: 1},
			{Slot: 10, ItemVnum: 27011, Price: 50, Count: 1}, {Slot: 11, ItemVnum: 27012, Price: 50, Count: 1},
			{Slot: 12, ItemVnum: 27013, Price: 50, Count: 1}, {Slot: 13, ItemVnum: 27014, Price: 50, Count: 1},
			{Slot: 14, ItemVnum: 27015, Price: 50, Count: 1}, {Slot: 15, ItemVnum: 27016, Price: 50, Count: 1},
			{Slot: 16, ItemVnum: 27017, Price: 50, Count: 1}, {Slot: 17, ItemVnum: 27018, Price: 50, Count: 1},
			{Slot: 18, ItemVnum: 27019, Price: 50, Count: 1}, {Slot: 19, ItemVnum: 27020, Price: 50, Count: 1},
			{Slot: 20, ItemVnum: 27021, Price: 50, Count: 1}, {Slot: 21, ItemVnum: 27022, Price: 50, Count: 1},
			{Slot: 22, ItemVnum: 27023, Price: 50, Count: 1}, {Slot: 23, ItemVnum: 27024, Price: 50, Count: 1},
			{Slot: 24, ItemVnum: 27025, Price: 50, Count: 1}, {Slot: 25, ItemVnum: 27026, Price: 50, Count: 1},
			{Slot: 26, ItemVnum: 27027, Price: 50, Count: 1}, {Slot: 27, ItemVnum: 27028, Price: 50, Count: 1},
			{Slot: 28, ItemVnum: 27029, Price: 50, Count: 1}, {Slot: 29, ItemVnum: 27030, Price: 50, Count: 1},
			{Slot: 30, ItemVnum: 27031, Price: 50, Count: 1}, {Slot: 31, ItemVnum: 27032, Price: 50, Count: 1},
			{Slot: 32, ItemVnum: 27033, Price: 50, Count: 1}, {Slot: 33, ItemVnum: 27034, Price: 50, Count: 1},
			{Slot: 34, ItemVnum: 27035, Price: 50, Count: 1}, {Slot: 35, ItemVnum: 27036, Price: 50, Count: 1},
			{Slot: 36, ItemVnum: 27037, Price: 50, Count: 1}, {Slot: 37, ItemVnum: 27038, Price: 50, Count: 1},
			{Slot: 38, ItemVnum: 27039, Price: 50, Count: 1}, {Slot: 39, ItemVnum: 27040, Price: 50, Count: 1},
			{Slot: 40, ItemVnum: 27041, Price: 50, Count: 1},
		},
	}}}
	if err := store.Save(oversizedShopPreviewCatalog); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for oversized shop preview catalog, got %v", err)
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
