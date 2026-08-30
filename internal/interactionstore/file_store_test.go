package interactionstore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unicode/utf8"
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
		{Kind: KindOpenSafebox, Ref: "npc:warehouse", Text: "Store your goods safely.", Size: 2},
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

func TestFileStoreSaveThenLoadQuestFlagDefinition(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	want := Snapshot{Definitions: []Definition{{
		Kind:      KindQuestFlag,
		Ref:       "quest:first_steps",
		Text:      "You have met the village guide.",
		QuestRef:  "quest:first_steps",
		QuestFlag: "met_guide",
		QuestTo:   1,
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save quest flag definition: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load quest flag definition: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected quest flag definition snapshot:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestFileStoreSaveThenLoadQuestFlagClearDefinition(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	want := Snapshot{Definitions: []Definition{{
		Kind:      KindQuestFlag,
		Ref:       "quest:first_steps_reset",
		Text:      "Quest cleared.",
		QuestRef:  "quest:first_steps",
		QuestFlag: "met_guide",
		QuestFrom: 1,
		QuestTo:   0,
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save quest flag clear definition: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load quest flag clear definition: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected quest flag clear definition snapshot:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestFileStoreSaveThenLoadQuestFlagDefinitionWithRewardExperience(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	want := Snapshot{Definitions: []Definition{{
		Kind:             KindQuestFlag,
		Ref:              "quest:first_steps_kill_turnin",
		Text:             "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:         "quest:first_steps",
		QuestFlag:        "killed_qa_mob",
		QuestFrom:        1,
		QuestTo:          0,
		RewardExperience: 50,
		RewardGold:       100,
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save quest flag reward-experience definition: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load quest flag reward-experience definition: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected quest flag reward-experience definition snapshot:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestFileStoreSaveThenLoadQuestFlagDefinitionWithRewardGold(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	want := Snapshot{Definitions: []Definition{{
		Kind:       KindQuestFlag,
		Ref:        "quest:first_steps_kill_turnin",
		Text:       "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:   "quest:first_steps",
		QuestFlag:  "killed_qa_mob",
		QuestFrom:  1,
		QuestTo:    0,
		RewardGold: 100,
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save quest flag reward-gold definition: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load quest flag reward-gold definition: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected quest flag reward-gold definition snapshot:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestFileStoreSaveThenLoadQuestFlagDefinitionWithRewardItem(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	want := Snapshot{Definitions: []Definition{{
		Kind:       KindQuestFlag,
		Ref:        "quest:first_steps_kill_turnin",
		Text:       "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:   "quest:first_steps",
		QuestFlag:  "killed_qa_mob",
		QuestFrom:  1,
		QuestTo:    0,
		RewardGold: 100,
		RewardItems: []RewardItemEntry{
			{ItemVnum: 27001, Count: 1},
		},
	}}}

	if err := store.Save(Snapshot{Definitions: []Definition{{
		Kind:            KindQuestFlag,
		Ref:             "quest:first_steps_kill_turnin",
		Text:            "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:        "quest:first_steps",
		QuestFlag:       "killed_qa_mob",
		QuestFrom:       1,
		QuestTo:         0,
		RewardGold:      100,
		RewardItemVnum:  27001,
		RewardItemCount: 1,
	}}}); err != nil {
		t.Fatalf("save quest flag reward-item definition: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load quest flag reward-item definition: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected quest flag reward-item definition snapshot:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestFileStoreSaveThenLoadQuestFlagDefinitionWithRewardItems(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	want := Snapshot{Definitions: []Definition{{
		Kind:       KindQuestFlag,
		Ref:        "quest:first_steps_kill_turnin",
		Text:       "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:   "quest:first_steps",
		QuestFlag:  "killed_qa_mob",
		QuestFrom:  1,
		QuestTo:    0,
		RewardGold: 100,
		RewardItems: []RewardItemEntry{
			{ItemVnum: 27001, Count: 1},
			{ItemVnum: 11200, Count: 1},
		},
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save quest flag reward-items definition: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load quest flag reward-items definition: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected quest flag reward-items definition snapshot:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestFileStoreSaveThenLoadQuestFlagDefinitionWithConsumeItems(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	want := Snapshot{Definitions: []Definition{{
		Kind:       KindQuestFlag,
		Ref:        "quest:first_steps_kill_turnin",
		Text:       "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:   "quest:first_steps",
		QuestFlag:  "killed_qa_mob",
		QuestFrom:  1,
		QuestTo:    0,
		RewardGold: 100,
		RewardItems: []RewardItemEntry{
			{ItemVnum: 11200, Count: 1},
		},
		ConsumeItems: []RewardItemEntry{
			{ItemVnum: 27001, Count: 1},
		},
		ConsumeGold: 25,
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save quest flag consume-items definition: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load quest flag consume-items definition: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected quest flag consume-items definition snapshot:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestFileStoreSaveThenLoadQuestFlagDefinitionWithConsumeGold(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	want := Snapshot{Definitions: []Definition{{
		Kind:        KindQuestFlag,
		Ref:         "quest:first_steps_kill_turnin",
		Text:        "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:    "quest:first_steps",
		QuestFlag:   "killed_qa_mob",
		QuestFrom:   1,
		QuestTo:     0,
		RewardGold:  100,
		ConsumeGold: 25,
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save quest flag consume-gold definition: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load quest flag consume-gold definition: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected quest flag consume-gold definition snapshot:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestFileStoreSaveThenLoadQuestFlagDefinitionWithConsumeExperience(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	want := Snapshot{Definitions: []Definition{{
		Kind:              KindQuestFlag,
		Ref:               "quest:first_steps_kill_turnin",
		Text:              "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:          "quest:first_steps",
		QuestFlag:         "killed_qa_mob",
		QuestFrom:         1,
		QuestTo:           0,
		RewardExperience:  50,
		ConsumeExperience: 10,
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save quest flag consume-experience definition: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load quest flag consume-experience definition: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected quest flag consume-experience definition snapshot:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestFileStoreRejectsInvalidQuestFlagInteractionDefinition(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	cases := []struct {
		name       string
		definition Definition
	}{
		{
			name:       "missing quest ref",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestFlag: "met_guide", QuestTo: 1},
		},
		{
			name:       "missing quest flag",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestTo: 1},
		},
		{
			name:       "default source and target no-op",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide"},
		},
		{
			name:       "same source and target value",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestFrom: 1, QuestTo: 1},
		},
		{
			name:       "missing player feedback text",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1},
		},
		{
			name:       "catalog not allowed",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1, Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}},
		},
		{
			name:       "reward gold exceeds point-change carrier",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1, RewardGold: QuestFlagRewardGoldMax + 1},
		},
		{
			name:       "reward experience exceeds point-change carrier",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1, RewardExperience: QuestFlagRewardExperienceMax + 1},
		},
		{
			name:       "consume gold exceeds point-change carrier",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1, ConsumeGold: QuestFlagConsumeGoldMax + 1},
		},
		{
			name:       "consume experience exceeds point-change carrier",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1, ConsumeExperience: QuestFlagConsumeExperienceMax + 1},
		},
		{
			name:       "reward item count without vnum",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1, RewardItemCount: 1},
		},
		{
			name:       "reward item vnum without count",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1, RewardItemVnum: 27001},
		},
		{
			name:       "reward item count exceeds bootstrap max",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1, RewardItemVnum: 27001, RewardItemCount: 256},
		},
		{
			name: "reward items entry missing vnum",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1, RewardItems: []RewardItemEntry{
				{ItemVnum: 0, Count: 1},
			}},
		},
		{
			name: "reward items entry missing count",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1, RewardItems: []RewardItemEntry{
				{ItemVnum: 27001, Count: 0},
			}},
		},
		{
			name: "reward items mixed with scalar shorthand",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1, RewardItemVnum: 27001, RewardItemCount: 1, RewardItems: []RewardItemEntry{
				{ItemVnum: 11200, Count: 1},
			}},
		},
		{
			name: "reward items exceed max entries",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1, RewardItems: []RewardItemEntry{
				{ItemVnum: 27001, Count: 1},
				{ItemVnum: 27001, Count: 1},
				{ItemVnum: 27001, Count: 1},
				{ItemVnum: 27001, Count: 1},
				{ItemVnum: 27001, Count: 1},
				{ItemVnum: 27001, Count: 1},
				{ItemVnum: 27001, Count: 1},
				{ItemVnum: 27001, Count: 1},
				{ItemVnum: 27001, Count: 1},
			}},
		},
		{
			name: "consume items entry missing vnum",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1, ConsumeItems: []RewardItemEntry{
				{ItemVnum: 0, Count: 1},
			}},
		},
		{
			name: "consume items entry missing count",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1, ConsumeItems: []RewardItemEntry{
				{ItemVnum: 27001, Count: 0},
			}},
		},
		{
			name: "consume items exceed max entries",
			definition: Definition{Kind: KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1, ConsumeItems: []RewardItemEntry{
				{ItemVnum: 27001, Count: 1},
				{ItemVnum: 27001, Count: 1},
				{ItemVnum: 27001, Count: 1},
				{ItemVnum: 27001, Count: 1},
				{ItemVnum: 27001, Count: 1},
				{ItemVnum: 27001, Count: 1},
				{ItemVnum: 27001, Count: 1},
				{ItemVnum: 27001, Count: 1},
				{ItemVnum: 27001, Count: 1},
			}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.Save(Snapshot{Definitions: []Definition{tc.definition}}); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot, got %v", err)
			}
		})
	}
}

func TestFileStoreRejectsRewardExperienceOnNonQuestFlagDefinitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	cases := []struct {
		name       string
		definition Definition
	}{
		{
			name:       "info",
			definition: Definition{Kind: KindInfo, Ref: "lore:square", Text: "Welcome.", RewardExperience: 10},
		},
		{
			name:       "talk",
			definition: Definition{Kind: KindTalk, Ref: "npc:guide", Text: "Welcome.", RewardExperience: 10},
		},
		{
			name:       "warp",
			definition: Definition{Kind: KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 42, X: 1700, Y: 2800, RewardExperience: 10},
		},
		{
			name:       "shop_preview",
			definition: Definition{Kind: KindShopPreview, Ref: "npc:merchant", Title: "Merchant", Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}, RewardExperience: 10},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.Save(Snapshot{Definitions: []Definition{tc.definition}}); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot, got %v", err)
			}
		})
	}
}

func TestFileStoreRejectsRewardGoldOnNonQuestFlagDefinitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	cases := []struct {
		name       string
		definition Definition
	}{
		{
			name:       "info",
			definition: Definition{Kind: KindInfo, Ref: "lore:square", Text: "Welcome.", RewardGold: 10},
		},
		{
			name:       "talk",
			definition: Definition{Kind: KindTalk, Ref: "npc:guide", Text: "Welcome.", RewardGold: 10},
		},
		{
			name:       "warp",
			definition: Definition{Kind: KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 42, X: 1700, Y: 2800, RewardGold: 10},
		},
		{
			name:       "shop_preview",
			definition: Definition{Kind: KindShopPreview, Ref: "npc:merchant", Title: "Merchant", Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}, RewardGold: 10},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.Save(Snapshot{Definitions: []Definition{tc.definition}}); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot, got %v", err)
			}
		})
	}
}

func TestFileStoreRejectsRewardItemsOnNonQuestFlagDefinitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	cases := []struct {
		name       string
		definition Definition
	}{
		{
			name:       "info",
			definition: Definition{Kind: KindInfo, Ref: "lore:square", Text: "Welcome.", RewardItems: []RewardItemEntry{{ItemVnum: 27001, Count: 1}}},
		},
		{
			name:       "talk",
			definition: Definition{Kind: KindTalk, Ref: "npc:guide", Text: "Welcome.", RewardItems: []RewardItemEntry{{ItemVnum: 27001, Count: 1}}},
		},
		{
			name:       "warp",
			definition: Definition{Kind: KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 42, X: 1700, Y: 2800, RewardItems: []RewardItemEntry{{ItemVnum: 27001, Count: 1}}},
		},
		{
			name:       "shop_preview",
			definition: Definition{Kind: KindShopPreview, Ref: "npc:merchant", Title: "Merchant", Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}, RewardItems: []RewardItemEntry{{ItemVnum: 27001, Count: 1}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.Save(Snapshot{Definitions: []Definition{tc.definition}}); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot, got %v", err)
			}
		})
	}
}

func TestFileStoreRejectsConsumeItemsOnNonQuestFlagDefinitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	cases := []struct {
		name       string
		definition Definition
	}{
		{
			name:       "info",
			definition: Definition{Kind: KindInfo, Ref: "lore:square", Text: "Welcome.", ConsumeItems: []RewardItemEntry{{ItemVnum: 27001, Count: 1}}},
		},
		{
			name:       "talk",
			definition: Definition{Kind: KindTalk, Ref: "npc:guide", Text: "Welcome.", ConsumeItems: []RewardItemEntry{{ItemVnum: 27001, Count: 1}}},
		},
		{
			name:       "warp",
			definition: Definition{Kind: KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 42, X: 1700, Y: 2800, ConsumeItems: []RewardItemEntry{{ItemVnum: 27001, Count: 1}}},
		},
		{
			name:       "shop_preview",
			definition: Definition{Kind: KindShopPreview, Ref: "npc:merchant", Title: "Merchant", Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}, ConsumeItems: []RewardItemEntry{{ItemVnum: 27001, Count: 1}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.Save(Snapshot{Definitions: []Definition{tc.definition}}); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot, got %v", err)
			}
		})
	}
}

func TestFileStoreRejectsConsumeGoldOnNonQuestFlagDefinitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	cases := []struct {
		name       string
		definition Definition
	}{
		{
			name:       "info",
			definition: Definition{Kind: KindInfo, Ref: "lore:square", Text: "Welcome.", ConsumeGold: 25},
		},
		{
			name:       "talk",
			definition: Definition{Kind: KindTalk, Ref: "npc:guide", Text: "Welcome.", ConsumeGold: 25},
		},
		{
			name:       "warp",
			definition: Definition{Kind: KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 42, X: 1700, Y: 2800, ConsumeGold: 25},
		},
		{
			name:       "shop_preview",
			definition: Definition{Kind: KindShopPreview, Ref: "npc:merchant", Title: "Merchant", Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}, ConsumeGold: 25},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.Save(Snapshot{Definitions: []Definition{tc.definition}}); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot, got %v", err)
			}
		})
	}
}

func TestFileStoreRejectsConsumeExperienceOnNonQuestFlagDefinitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	cases := []struct {
		name       string
		definition Definition
	}{
		{
			name:       "info",
			definition: Definition{Kind: KindInfo, Ref: "lore:square", Text: "Welcome.", ConsumeExperience: 10},
		},
		{
			name:       "talk",
			definition: Definition{Kind: KindTalk, Ref: "npc:guide", Text: "Welcome.", ConsumeExperience: 10},
		},
		{
			name:       "warp",
			definition: Definition{Kind: KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 42, X: 1700, Y: 2800, ConsumeExperience: 10},
		},
		{
			name:       "shop_preview",
			definition: Definition{Kind: KindShopPreview, Ref: "npc:merchant", Title: "Merchant", Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}, ConsumeExperience: 10},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.Save(Snapshot{Definitions: []Definition{tc.definition}}); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot, got %v", err)
			}
		})
	}
}

func TestFileStoreSaveThenLoadQuestGatedWarpAndShopPreviewDefinitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	want := Snapshot{Definitions: []Definition{
		{
			Kind:      KindInfo,
			Ref:       "lore:gated_signpost",
			Text:      "The gated signpost describes the square.",
			QuestRef:  "quest:first_steps",
			QuestFlag: "met_guide",
			QuestFrom: 1,
		},
		{
			Kind:      KindOpenSafebox,
			Ref:       "npc:gated_warehouse",
			Text:      "The warehouse keeper unlocks the vault.",
			Size:      2,
			QuestRef:  "quest:first_steps",
			QuestFlag: "met_guide",
			QuestFrom: 1,
		},
		{
			Kind:      KindShopPreview,
			Ref:       "npc:gated_merchant",
			Title:     "Gated Merchant",
			Catalog:   []MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}},
			QuestRef:  "quest:first_steps",
			QuestFlag: "met_guide",
			QuestFrom: 1,
		},
		{
			Kind:      KindTalk,
			Ref:       "npc:gated_guide",
			Text:      "Welcome to the gated square.",
			QuestRef:  "quest:first_steps",
			QuestFlag: "met_guide",
			QuestFrom: 1,
		},
		{
			Kind:      KindWarp,
			Ref:       "npc:gated_teleporter",
			Text:      "Step through the gate.",
			MapIndex:  42,
			X:         1700,
			Y:         2800,
			QuestRef:  "quest:first_steps",
			QuestFlag: "met_guide",
			QuestFrom: 1,
		},
	}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save quest-gated service definitions: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load quest-gated service definitions: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected quest-gated service definition snapshot:\n got: %#v\nwant: %#v", got, want)
	}
	for _, definition := range got.Definitions {
		if !HasServiceQuestGate(definition) {
			t.Fatalf("expected loaded non-mutating definitions to report quest gates, got %#v", got.Definitions)
		}
	}
}

func TestFileStoreSaveThenLoadOpenSafeboxDefinitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	want := Snapshot{Definitions: []Definition{
		{Kind: KindOpenSafebox, Ref: "npc:default_warehouse", Text: "Default vault."},
		{Kind: KindOpenSafebox, Ref: "npc:warehouse", Text: "Store your goods safely.", Size: 3},
	}}
	if err := store.Save(want); err != nil {
		t.Fatalf("save open_safebox definitions: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load open_safebox definitions: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected open_safebox snapshot:\n got: %#v\nwant: %#v", got, want)
	}
	if EffectiveOpenSafeboxSize(got.Definitions[0]) != OpenSafeboxSizeMin {
		t.Fatalf("expected omitted size to default to %d, got %d", OpenSafeboxSizeMin, EffectiveOpenSafeboxSize(got.Definitions[0]))
	}
	if EffectiveOpenSafeboxSize(got.Definitions[1]) != 3 {
		t.Fatalf("expected authored size 3, got %d", EffectiveOpenSafeboxSize(got.Definitions[1]))
	}
}

func TestFileStoreRejectsInvalidOpenSafeboxDefinitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	cases := []struct {
		name       string
		definition Definition
	}{
		{
			name:       "size above max",
			definition: Definition{Kind: KindOpenSafebox, Ref: "npc:warehouse", Text: "Vault.", Size: OpenSafeboxSizeMax + 1},
		},
		{
			name:       "title not allowed",
			definition: Definition{Kind: KindOpenSafebox, Ref: "npc:warehouse", Text: "Vault.", Title: "Warehouse"},
		},
		{
			name:       "catalog not allowed",
			definition: Definition{Kind: KindOpenSafebox, Ref: "npc:warehouse", Text: "Vault.", Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}},
		},
		{
			name:       "warp coords not allowed",
			definition: Definition{Kind: KindOpenSafebox, Ref: "npc:warehouse", Text: "Vault.", MapIndex: 1, X: 100, Y: 200},
		},
		{
			name:       "reward gold not allowed",
			definition: Definition{Kind: KindOpenSafebox, Ref: "npc:warehouse", Text: "Vault.", RewardGold: 10},
		},
		{
			name:       "quest_to mutates",
			definition: Definition{Kind: KindOpenSafebox, Ref: "npc:warehouse", Text: "Vault.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestFrom: 1, QuestTo: 2},
		},
		{
			name:       "text with NUL",
			definition: Definition{Kind: KindOpenSafebox, Ref: "npc:warehouse", Text: "visible\x00hidden"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.Save(Snapshot{Definitions: []Definition{tc.definition}}); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot for %s, got %v", tc.name, err)
			}
		})
	}
}

func TestFileStoreSaveThenLoadOpenCubeDefinitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	want := Snapshot{Definitions: []Definition{
		{Kind: KindOpenCube, Ref: "npc:default_cube", Text: "The craftsman lights the forge."},
		{Kind: KindOpenCube, Ref: "npc:qa_cube", Text: "Bring materials to craft.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestFrom: 1},
	}}
	if err := store.Save(want); err != nil {
		t.Fatalf("save open_cube definitions: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load open_cube definitions: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected open_cube snapshot:\n got: %#v\nwant: %#v", got, want)
	}
	if !HasServiceQuestGate(got.Definitions[1]) {
		t.Fatalf("expected gated open_cube definition to report a service quest gate, got %#v", got.Definitions[1])
	}
}

func TestFileStoreRejectsInvalidOpenCubeDefinitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	cases := []struct {
		name       string
		definition Definition
	}{
		{
			name:       "size not allowed",
			definition: Definition{Kind: KindOpenCube, Ref: "npc:qa_cube", Text: "Craft.", Size: 1},
		},
		{
			name:       "title not allowed",
			definition: Definition{Kind: KindOpenCube, Ref: "npc:qa_cube", Text: "Craft.", Title: "Cube"},
		},
		{
			name:       "catalog not allowed",
			definition: Definition{Kind: KindOpenCube, Ref: "npc:qa_cube", Text: "Craft.", Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}},
		},
		{
			name:       "warp coords not allowed",
			definition: Definition{Kind: KindOpenCube, Ref: "npc:qa_cube", Text: "Craft.", MapIndex: 1, X: 100, Y: 200},
		},
		{
			name:       "reward gold not allowed",
			definition: Definition{Kind: KindOpenCube, Ref: "npc:qa_cube", Text: "Craft.", RewardGold: 10},
		},
		{
			name:       "quest_to mutates",
			definition: Definition{Kind: KindOpenCube, Ref: "npc:qa_cube", Text: "Craft.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestFrom: 1, QuestTo: 2},
		},
		{
			name:       "text with NUL",
			definition: Definition{Kind: KindOpenCube, Ref: "npc:qa_cube", Text: "visible\x00hidden"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.Save(Snapshot{Definitions: []Definition{tc.definition}}); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot for %s, got %v", tc.name, err)
			}
		})
	}
}

func TestFileStoreRejectsInvalidQuestGatedServiceDefinitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	baseWarp := Definition{Kind: KindWarp, Ref: "npc:gated_teleporter", Text: "Step through the gate.", MapIndex: 42, X: 1700, Y: 2800}
	baseShop := testMerchantCatalogDefinition()
	baseShop.Ref = "npc:gated_merchant"
	cases := []struct {
		name       string
		definition Definition
	}{
		{
			name:       "warp quest ref without flag",
			definition: Definition{Kind: KindWarp, Ref: "npc:gated_teleporter", Text: "Step through the gate.", MapIndex: 42, X: 1700, Y: 2800, QuestRef: "quest:first_steps"},
		},
		{
			name:       "warp quest flag without ref",
			definition: Definition{Kind: KindWarp, Ref: "npc:gated_teleporter", Text: "Step through the gate.", MapIndex: 42, X: 1700, Y: 2800, QuestFlag: "met_guide"},
		},
		{
			name:       "warp quest_to mutates",
			definition: Definition{Kind: KindWarp, Ref: "npc:gated_teleporter", Text: "Step through the gate.", MapIndex: 42, X: 1700, Y: 2800, QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestFrom: 1, QuestTo: 2},
		},
		{
			name:       "shop quest_to mutates",
			definition: Definition{Kind: KindShopPreview, Ref: "npc:gated_merchant", Title: "Gated Merchant", Catalog: baseShop.Catalog, QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestFrom: 1, QuestTo: 2},
		},
		{
			name:       "info quest ref without flag",
			definition: Definition{Kind: KindInfo, Ref: "lore:gated_signpost", Text: "The gated signpost describes the square.", QuestRef: "quest:first_steps"},
		},
		{
			name:       "talk quest flag without ref",
			definition: Definition{Kind: KindTalk, Ref: "npc:gated_guide", Text: "Welcome to the gated square.", QuestFlag: "met_guide"},
		},
		{
			name:       "info quest_to mutates",
			definition: Definition{Kind: KindInfo, Ref: "lore:gated_signpost", Text: "The gated signpost describes the square.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestFrom: 1, QuestTo: 2},
		},
		{
			name:       "talk quest_to mutates",
			definition: Definition{Kind: KindTalk, Ref: "npc:gated_guide", Text: "Welcome to the gated square.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestFrom: 1, QuestTo: 2},
		},
		{
			name:       "ungated warp with orphan quest_from",
			definition: Definition{Kind: KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 42, X: 1700, Y: 2800, QuestFrom: 1},
		},
		{
			name:       "ungated talk with orphan quest_from",
			definition: Definition{Kind: KindTalk, Ref: "npc:guide", Text: "Welcome.", QuestFrom: 1},
		},
		{
			name:       "ungated talk with orphan quest_to",
			definition: Definition{Kind: KindTalk, Ref: "npc:guide", Text: "Welcome.", QuestTo: 1},
		},
		{
			name:       "ungated warp with orphan quest_to",
			definition: Definition{Kind: KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 42, X: 1700, Y: 2800, QuestTo: 1},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.Save(Snapshot{Definitions: []Definition{tc.definition}}); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot for %s, got %v (base=%#v)", tc.name, err, baseWarp)
			}
		})
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

func TestFileStoreLoadRejectsSymlinkedCommittedInteractionSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	target := filepath.Join(t.TempDir(), "outside-interaction-definitions.json")
	if err := os.WriteFile(target, []byte(`{"definitions":[{"kind":"info","ref":"lore:alchemist","text":"The alchemist studies forgotten herbs."}]}`), 0o644); err != nil {
		t.Fatalf("write outside interaction snapshot: %v", err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	_, err := store.Load()
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for symlinked interaction snapshot, got %v", err)
	}
}

func TestFileStoreValidateRejectsSymlinkedInteractionCrashTempFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Definitions: []Definition{{Kind: KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."}}}); err != nil {
		t.Fatalf("save interaction snapshot: %v", err)
	}
	target := filepath.Join(t.TempDir(), "outside-interaction-temp.json")
	if err := os.WriteFile(target, []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write outside interaction temp target: %v", err)
	}
	link := filepath.Join(filepath.Dir(path), ".interaction-definitions-crashed.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	_, err := store.Validate()
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for symlinked interaction crash temp file, got %v", err)
	}
}

func TestFileStoreValidateRejectsSymlinkedCommittedInteractionSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	target := filepath.Join(t.TempDir(), "outside-interaction-definitions.json")
	if err := os.WriteFile(target, []byte(`{"definitions":[{"kind":"info","ref":"lore:alchemist","text":"The alchemist studies forgotten herbs."}]}`), 0o644); err != nil {
		t.Fatalf("write outside interaction snapshot: %v", err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	_, err := store.Validate()
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for symlinked interaction snapshot validation, got %v", err)
	}
}

func TestFileStoreValidateReportsDeterministicInteractionSummary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Definitions: []Definition{
		{Kind: KindTalk, Ref: "npc:village_guard", Text: "VillageGuard : Keep your blade sharp."},
		{Kind: KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
		testMerchantCatalogDefinition(),
		{Kind: KindWarp, Ref: "npc:teleporter", MapIndex: 42, X: 1700, Y: 2800},
	}}); err != nil {
		t.Fatalf("save interaction snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), ".interaction-definitions-crashed.json"), []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write interaction crash temp: %v", err)
	}

	summary, err := store.Validate()
	if err != nil {
		t.Fatalf("validate interaction store: %v", err)
	}
	want := SnapshotSummary{
		DefinitionCount: 4,
		DefinitionKeys:  []string{"info:lore:alchemist", "shop_preview:npc:merchant", "talk:npc:village_guard", "warp:npc:teleporter"},
		CrashTempCount:  1,
		CrashTempFiles:  []string{".interaction-definitions-crashed.json"},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected interaction validation summary: got %#v want %#v", summary, want)
	}
}

func TestFileStoreValidateTreatsMissingInteractionSnapshotAsEmpty(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "missing", "interaction-definitions.json"))

	summary, err := store.Validate()
	if err != nil {
		t.Fatalf("validate missing interaction store: %v", err)
	}
	want := SnapshotSummary{DefinitionKeys: []string{}}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected empty interaction summary: got %#v want %#v", summary, want)
	}
}

func TestFileStoreCleanupCrashTempFilesRemovesOnlyInteractionTempResidue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Definitions: []Definition{
		{Kind: KindTalk, Ref: "npc:village_guard", Text: "VillageGuard : Keep your blade sharp."},
		{Kind: KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
		testMerchantCatalogDefinition(),
	}}); err != nil {
		t.Fatalf("save interaction snapshot: %v", err)
	}
	for _, name := range []string{".interaction-definitions-zeta.json", ".interaction-definitions-alpha.json", ".unrelated-temp.json"} {
		if err := os.WriteFile(filepath.Join(filepath.Dir(path), name), []byte(`{"not":"committed"}`), 0o644); err != nil {
			t.Fatalf("write temp file %s: %v", name, err)
		}
	}

	summary, err := store.CleanupCrashTempFiles()
	if err != nil {
		t.Fatalf("cleanup interaction crash temp files: %v", err)
	}
	want := SnapshotSummary{DefinitionCount: 3, DefinitionKeys: []string{"info:lore:alchemist", "shop_preview:npc:merchant", "talk:npc:village_guard"}}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected post-cleanup interaction summary: got %#v want %#v", summary, want)
	}
	for _, name := range []string{".interaction-definitions-zeta.json", ".interaction-definitions-alpha.json"} {
		if _, err := os.Stat(filepath.Join(filepath.Dir(path), name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected interaction crash temp %s to be removed, stat err=%v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), ".unrelated-temp.json")); err != nil {
		t.Fatalf("expected unrelated hidden file to be preserved: %v", err)
	}
}

func TestFileStoreCleanupCrashTempFilesFailsClosedOnCorruptInteractionSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	crashTemp := filepath.Join(filepath.Dir(path), ".interaction-definitions-crashed.json")
	if err := os.WriteFile(path, []byte(`{"definitions":[`), 0o644); err != nil {
		t.Fatalf("write corrupt interaction snapshot: %v", err)
	}
	if err := os.WriteFile(crashTemp, []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write crash temp: %v", err)
	}

	_, err := store.CleanupCrashTempFiles()
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot and no cleanup for corrupt committed snapshot, got %v", err)
	}
	if _, err := os.Stat(crashTemp); err != nil {
		t.Fatalf("expected crash temp to remain after failed cleanup: %v", err)
	}
}

func TestFileStoreValidateFailsClosedOnCorruptInteractionSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"definitions":[`), 0o644); err != nil {
		t.Fatalf("write corrupt interaction snapshot: %v", err)
	}

	_, err := store.Validate()
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for corrupt interaction snapshot, got %v", err)
	}
}

func TestFileStoreLoadRejectsMalformedOrInvalidSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interaction-definitions.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("null"), 0o644); err != nil {
		t.Fatalf("write null snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for null snapshot root, got %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"definitions":null}`), 0o644); err != nil {
		t.Fatalf("write null interaction definition collection: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for null interaction definition collection, got %v", err)
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
	infoTextWithNUL := Snapshot{Definitions: []Definition{{Kind: KindInfo, Ref: "lore:alchemist", Text: "visible\x00hidden"}}}
	if err := store.Save(infoTextWithNUL); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for NUL info text, got %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"definitions":[{"kind":"info","ref":"lore:alchemist","text":"visible\u0000hidden"}]}`), 0o644); err != nil {
		t.Fatalf("write NUL info text snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for loaded NUL info text, got %v", err)
	}
	talkTextWithNUL := Snapshot{Definitions: []Definition{{Kind: KindTalk, Ref: "npc:village_guard", Text: "visible\x00hidden"}}}
	if err := store.Save(talkTextWithNUL); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for NUL talk text, got %v", err)
	}
	warpTextWithNUL := Snapshot{Definitions: []Definition{{Kind: KindWarp, Ref: "npc:teleporter", Text: "visible\x00hidden", MapIndex: 42, X: 1700, Y: 2800}}}
	if err := store.Save(warpTextWithNUL); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for NUL warp text, got %v", err)
	}
	blankShopPreviewTitle := Snapshot{Definitions: []Definition{{Kind: KindShopPreview, Ref: "npc:merchant", Title: "   ", Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}}}}
	if err := store.Save(blankShopPreviewTitle); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for blank shop preview title, got %v", err)
	}
	shopPreviewTitleWithNUL := Snapshot{Definitions: []Definition{{Kind: KindShopPreview, Ref: "npc:merchant", Title: "Village\x00Merchant", Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}}}}
	if err := store.Save(shopPreviewTitleWithNUL); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for NUL shop preview title, got %v", err)
	}
	invalidUTF8 := string([]byte{'v', 'i', 's', 'i', 'b', 'l', 'e', 0xff, 'h', 'i', 'd', 'd', 'e', 'n'})
	if utf8.ValidString(invalidUTF8) {
		t.Fatal("test fixture must contain invalid UTF-8")
	}
	infoTextWithInvalidUTF8 := Snapshot{Definitions: []Definition{{Kind: KindInfo, Ref: "lore:invalid_utf8", Text: invalidUTF8}}}
	if err := store.Save(infoTextWithInvalidUTF8); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for invalid UTF-8 info text, got %v", err)
	}
	body := []byte(`{"definitions":[{"kind":"info","ref":"lore:invalid_utf8","text":"visible`)
	body = append(body, 0xff)
	body = append(body, []byte(`hidden"}]}`)...)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write invalid UTF-8 interaction snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for loaded invalid UTF-8 info text, got %v", err)
	}
	shopPreviewTitleWithInvalidUTF8 := Snapshot{Definitions: []Definition{{Kind: KindShopPreview, Ref: "npc:invalid_utf8", Title: invalidUTF8, Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}}}}
	if err := store.Save(shopPreviewTitleWithInvalidUTF8); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for invalid UTF-8 shop preview title, got %v", err)
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
	shopPreviewOversizedPrice := Snapshot{Definitions: []Definition{{Kind: KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant", Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: MerchantCatalogMaxEntryPrice + 1, Count: 1}}}}}
	if err := store.Save(shopPreviewOversizedPrice); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for over-uint32 shop preview entry price, got %v", err)
	}
	shopPreviewOversizedCount := Snapshot{Definitions: []Definition{{Kind: KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant", Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: MerchantCatalogMaxEntryCount + 1}}}}}
	if err := store.Save(shopPreviewOversizedCount); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for over-uint8 shop preview entry count, got %v", err)
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
