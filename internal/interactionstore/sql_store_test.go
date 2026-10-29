package interactionstore

import (
	"errors"
	"testing"
)

func TestSQLStoreRejectsUnrepresentableContentBeforeExecutor(t *testing.T) {
	for _, definition := range []Definition{
		{Kind: KindOpenCube, Ref: "cube:guide", Text: "Craft"},
		{Kind: KindShopPreview, Ref: "shop:guide", Title: "Guide", Tabs: []MerchantCatalogTab{{Name: "Goods"}}, Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 1, Price: 10, Count: 1}}},
	} {
		err := NewSQLStore(nil).Save(Snapshot{Definitions: []Definition{definition}})
		if !errors.Is(err, ErrInvalidSnapshot) {
			t.Fatalf("Save(%s) = %v, want unsupported snapshot before SQL", definition.Kind, err)
		}
	}
	if _, err := NewSQLStore(nil).Load(); !errors.Is(err, ErrSQLExecutorRequired) {
		t.Fatalf("nil executor Load = %v", err)
	}
	if err := NewSQLStore(nil).Save(Snapshot{}); !errors.Is(err, ErrSQLExecutorRequired) {
		t.Fatalf("nil executor Save = %v", err)
	}
}

func TestSelectInteractionRematerializeStoreDefaultsToFile(t *testing.T) {
	file := NewMemoryStore()
	if got := SelectRematerializeStore(file, nil); got != file {
		t.Fatalf("default selection = %T, want file seam", got)
	}
	sql := NewSQLStore(nil)
	if got := SelectRematerializeStore(file, sql); got != sql {
		t.Fatalf("explicit selection = %T, want SQL seam", got)
	}
}
