package minimal

import (
	"testing"

	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
)

func TestBootstrapItemFlagsProjectConfirmWhenUseMetadata(t *testing.T) {
	template := itemcatalog.Template{Vnum: 27005, Name: "Confirm Elixir", Stackable: true, MaxCount: 200, ConfirmWhenUse: true}

	flags := bootstrapItemFlags(template)
	if flags&itemproto.ItemFlagConfirmWhenUse == 0 {
		t.Fatalf("expected confirm_when_use template metadata to project ITEM_FLAG_CONFIRM_WHEN_USE, got flags %#x", flags)
	}
}
