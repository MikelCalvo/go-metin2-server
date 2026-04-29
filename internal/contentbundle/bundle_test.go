package contentbundle

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
)

func testMerchantCatalogDefinition() interactionstore.Definition {
	return interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:merchant",
		Title: "Village Merchant",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
			{Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
		},
	}
}

func TestFromSnapshotsBuildsDeterministicPortableBundle(t *testing.T) {
	bundle, err := FromSnapshots(
		staticstore.Snapshot{StaticActors: []staticstore.StaticActor{
			{EntityID: 9, Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"},
			{EntityID: 3, Name: "Blacksmith", MapIndex: 42, X: 1750, Y: 2850, RaceNum: 20301},
			{EntityID: 7, Name: "Merchant", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20302, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"},
			{EntityID: 11, Name: "Teleporter", MapIndex: 42, X: 1850, Y: 2950, RaceNum: 20303, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:teleporter"},
		}},
		interactionstore.Snapshot{Definitions: []interactionstore.Definition{
			{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."},
			{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
			{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", MapIndex: 42, X: 1700, Y: 2800, Text: "Step through the gate."},
			testMerchantCatalogDefinition(),
		}},
	)
	if err != nil {
		t.Fatalf("from snapshots: %v", err)
	}
	want := Bundle{
		StaticActors: []StaticActor{
			{Name: "Blacksmith", MapIndex: 42, X: 1750, Y: 2850, RaceNum: 20301},
			{Name: "Merchant", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20302, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"},
			{Name: "Teleporter", MapIndex: 42, X: 1850, Y: 2950, RaceNum: 20303, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:teleporter"},
			{Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"},
		},
		InteractionDefinitions: []interactionstore.Definition{
			{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
			testMerchantCatalogDefinition(),
			{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."},
			{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", MapIndex: 42, X: 1700, Y: 2800, Text: "Step through the gate."},
		},
	}
	if !reflect.DeepEqual(bundle, want) {
		t.Fatalf("unexpected portable content bundle:\n got: %#v\nwant: %#v", bundle, want)
	}
}

func TestCanonicalizeNormalizesStructuredShopPreviewCatalog(t *testing.T) {
	bundle, err := Canonicalize(Bundle{InteractionDefinitions: []interactionstore.Definition{{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:merchant",
		Title: "Village Merchant",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
			{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
		},
	}}})
	if err != nil {
		t.Fatalf("canonicalize structured shop preview bundle: %v", err)
	}
	want := Bundle{InteractionDefinitions: []interactionstore.Definition{testMerchantCatalogDefinition()}}
	if !reflect.DeepEqual(bundle, want) {
		t.Fatalf("unexpected canonical structured shop preview bundle:\n got: %#v\nwant: %#v", bundle, want)
	}
}

func TestCanonicalizeMerchantBundleKeepsStableBuySlotAddressing(t *testing.T) {
	bundle, err := Canonicalize(Bundle{
		StaticActors: []StaticActor{{Name: "Merchant", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20302, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"}},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:  interactionstore.KindShopPreview,
			Ref:   "npc:merchant",
			Title: "Village Merchant",
			Catalog: []interactionstore.MerchantCatalogEntry{
				{Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
				{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
			},
		}},
	})
	if err != nil {
		t.Fatalf("canonicalize merchant buy bundle: %v", err)
	}
	if len(bundle.InteractionDefinitions) != 1 {
		t.Fatalf("expected 1 interaction definition, got %d", len(bundle.InteractionDefinitions))
	}
	catalog := bundle.InteractionDefinitions[0].Catalog
	if len(catalog) != 2 {
		t.Fatalf("expected 2 merchant catalog entries, got %d", len(catalog))
	}
	if catalog[0].Slot != 0 || catalog[0].ItemVnum != 27001 || catalog[0].Price != 50 || catalog[0].Count != 1 {
		t.Fatalf("unexpected canonical merchant slot 0: %+v", catalog[0])
	}
	if catalog[1].Slot != 1 || catalog[1].ItemVnum != 11200 || catalog[1].Price != 500 || catalog[1].Count != 1 {
		t.Fatalf("unexpected canonical merchant slot 1: %+v", catalog[1])
	}
}

func TestCanonicalizeRejectsDanglingInteractionReference(t *testing.T) {
	_, err := Canonicalize(Bundle{
		StaticActors:           []StaticActor{{Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for dangling interaction reference, got %v", err)
	}
}

func TestCanonicalizeRejectsDuplicateInteractionDefinitions(t *testing.T) {
	_, err := Canonicalize(Bundle{
		InteractionDefinitions: []interactionstore.Definition{
			{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "First"},
			{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "Duplicate"},
		},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for duplicate interaction definitions, got %v", err)
	}
}

func TestCanonicalizeRejectsInvalidWarpInteractionDefinition(t *testing.T) {
	_, err := Canonicalize(Bundle{
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", X: 1700, Y: 2800}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for invalid warp interaction definition, got %v", err)
	}
}

func TestExampleBootstrapNPCServiceBundleCanonicalizes(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "examples", "bootstrap-npc-service-bundle.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read example bundle: %v", err)
	}
	var bundle Bundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("decode example bundle: %v", err)
	}
	canonical, err := Canonicalize(bundle)
	if err != nil {
		t.Fatalf("canonicalize example bundle: %v", err)
	}
	if !reflect.DeepEqual(canonical, bundle) {
		t.Fatalf("expected example bundle to already be canonical:\n got: %#v\nwant: %#v", canonical, bundle)
	}
}
