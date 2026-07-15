package minimal

import (
	"testing"

	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
)

func TestBootstrapItemFlagsProjectConfirmWhenUseMetadata(t *testing.T) {
	template := itemcatalog.Template{Vnum: 27005, Name: "Confirm Elixir", Stackable: true, MaxCount: 200, SlowQuery: true, ConfirmWhenUse: true, Log: true}

	flags := bootstrapItemFlags(template)
	want := itemproto.ItemFlagSlowQuery | itemproto.ItemFlagConfirmWhenUse | itemproto.ItemFlagLog
	if flags&want != want {
		t.Fatalf("expected slow_query/confirm_when_use/log template metadata to project flags %#x, got %#x", want, flags)
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

func TestBootstrapItemFlagsProjectQuestUseMultipleMetadata(t *testing.T) {
	template := itemcatalog.Template{Vnum: 71124, Name: "Repeatable Quest Charm", Stackable: false, MaxCount: 1, QuestUseMultiple: true}

	flags := bootstrapItemFlags(template)
	if flags&itemproto.ItemFlagQuestUseMultiple == 0 {
		t.Fatalf("expected quest_use_multiple template metadata to project ITEM_FLAG_QUEST_USE_MULTIPLE, got flags %#x", flags)
	}
}
