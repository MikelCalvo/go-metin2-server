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

func TestBootstrapItemFlagsProjectQuestUseAndApplicableMetadata(t *testing.T) {
	template := itemcatalog.Template{Vnum: 71123, Name: "Quest Applicable Charm", Stackable: false, MaxCount: 1, QuestUse: true, Applicable: true}

	flags := bootstrapItemFlags(template)
	want := itemproto.ItemFlagQuestUse | itemproto.ItemFlagApplicable
	if flags&want != want {
		t.Fatalf("expected quest_use/applicable template metadata to project flags %#x, got %#x", want, flags)
	}
}
