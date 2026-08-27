package contentbundle

import (
	"errors"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func metGuideQuestFlagWriterDefinition() interactionstore.Definition {
	return interactionstore.Definition{
		Kind:      interactionstore.KindQuestFlag,
		Ref:       "quest:first_steps",
		Text:      "Quest updated: first_steps.met_guide = 1.",
		QuestRef:  "quest:first_steps",
		QuestFlag: "met_guide",
		QuestTo:   1,
	}
}

func TestCanonicalizeRejectsServiceQuestGateWithoutInBundleWriter(t *testing.T) {
	_, err := Canonicalize(Bundle{
		StaticActors: []StaticActor{
			{Name: "GatedMerchant", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:gated_merchant"},
		},
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:      interactionstore.KindShopPreview,
			Ref:       "npc:gated_merchant",
			Title:     "Gated Merchant",
			Catalog:   []interactionstore.MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}},
			QuestRef:  "quest:first_steps",
			QuestFlag: "met_guide",
			QuestFrom: 1,
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for gated shop without quest_flag/kill-quest writer, got %v", err)
	}
}

func TestCanonicalizeRejectsPartialServiceQuestGate(t *testing.T) {
	_, err := Canonicalize(Bundle{
		StaticActors: []StaticActor{
			{Name: "GatedGuide", MapIndex: 1, X: 469400, Y: 964200, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:gated_guide"},
		},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:     interactionstore.KindTalk,
			Ref:      "npc:gated_guide",
			Text:     "Welcome.",
			QuestRef: "quest:first_steps",
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for partial service quest gate, got %v", err)
	}
}

func TestCanonicalizeRejectsReversePartialServiceQuestGate(t *testing.T) {
	_, err := Canonicalize(Bundle{
		StaticActors: []StaticActor{
			{Name: "ReversePartialGatedGuide", MapIndex: 1, X: 469400, Y: 964200, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:reverse_partial_gated_guide"},
		},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:      interactionstore.KindTalk,
			Ref:       "npc:reverse_partial_gated_guide",
			Text:      "Welcome.",
			QuestFlag: "met_guide",
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for reverse partial service quest gate, got %v", err)
	}
}

func TestCanonicalizeRejectsOrphanServiceQuestFrom(t *testing.T) {
	_, err := Canonicalize(Bundle{
		StaticActors: []StaticActor{
			{Name: "OrphanQuestFromGuide", MapIndex: 1, X: 469400, Y: 964200, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:orphan_quest_from_guide"},
		},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:      interactionstore.KindTalk,
			Ref:       "npc:orphan_quest_from_guide",
			Text:      "Welcome.",
			QuestFrom: 1,
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for orphan service quest_from, got %v", err)
	}
}

func TestCanonicalizeAcceptsServiceQuestGateWhenQuestFlagWriterPresent(t *testing.T) {
	if _, err := Canonicalize(Bundle{
		StaticActors: []StaticActor{
			{Name: "GatedMerchant", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:gated_merchant"},
		},
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		},
		InteractionDefinitions: []interactionstore.Definition{
			metGuideQuestFlagWriterDefinition(),
			{
				Kind:      interactionstore.KindShopPreview,
				Ref:       "npc:gated_merchant",
				Title:     "Gated Merchant",
				Catalog:   []interactionstore.MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}},
				QuestRef:  "quest:first_steps",
				QuestFlag: "met_guide",
				QuestFrom: 1,
			},
		},
	}); err != nil {
		t.Fatalf("canonicalize gated shop with quest_flag writer: %v", err)
	}
}

func TestCanonicalizeRejectsKillQuestRequireGateWithoutInBundleWriter(t *testing.T) {
	_, err := Canonicalize(Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:              "practice.gated_kill_quest_mob",
			Name:             "GatedKillQuestMob",
			MapIndex:         42,
			X:                1800,
			Y:                2900,
			RaceNum:          101,
			CombatProfile:    worldruntime.StaticActorCombatProfilePracticeMob,
			RewardQuestRef:   "quest:first_steps",
			RewardQuestFlag:  "killed_qa_mob",
			RewardQuestTo:    1,
			RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
			RequireQuestRef:  "quest:first_steps",
			RequireQuestFlag: "met_guide",
			RequireQuestFrom: 1,
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for gated kill-quest without writer for met_guide, got %v", err)
	}
}

func TestCanonicalizeAcceptsKillQuestRequireGateWhenQuestFlagWriterPresent(t *testing.T) {
	if _, err := Canonicalize(Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:              "practice.gated_kill_quest_mob",
			Name:             "GatedKillQuestMob",
			MapIndex:         42,
			X:                1800,
			Y:                2900,
			RaceNum:          101,
			CombatProfile:    worldruntime.StaticActorCombatProfilePracticeMob,
			RewardQuestRef:   "quest:first_steps",
			RewardQuestFlag:  "killed_qa_mob",
			RewardQuestTo:    1,
			RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
			RequireQuestRef:  "quest:first_steps",
			RequireQuestFlag: "met_guide",
			RequireQuestFrom: 1,
		}},
		InteractionDefinitions: []interactionstore.Definition{metGuideQuestFlagWriterDefinition()},
	}); err != nil {
		t.Fatalf("canonicalize gated kill-quest with quest_flag writer: %v", err)
	}
}

func TestCanonicalizeAcceptsServiceQuestGateWhenKillQuestCreditWritesRequiredFlag(t *testing.T) {
	if _, err := Canonicalize(Bundle{
		StaticActors: []StaticActor{
			{Name: "HunterTalk", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:hunter_talk"},
		},
		SpawnGroups: []SpawnGroup{{
			Ref:             "practice.writer_mob",
			Name:            "WriterMob",
			MapIndex:        1,
			X:               1200,
			Y:               2200,
			RaceNum:         101,
			CombatProfile:   worldruntime.StaticActorCombatProfilePracticeMob,
			RewardQuestRef:  "quest:first_steps",
			RewardQuestFlag: "killed_qa_mob",
			RewardQuestTo:   1,
			RewardQuestText: "Quest updated: first_steps.killed_qa_mob = 1.",
		}},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:      interactionstore.KindTalk,
			Ref:       "npc:hunter_talk",
			Text:      "Bring me proof of the kill.",
			QuestRef:  "quest:first_steps",
			QuestFlag: "killed_qa_mob",
			QuestFrom: 1,
		}},
	}); err != nil {
		t.Fatalf("canonicalize gated talk with kill-quest writer: %v", err)
	}
}

func TestCanonicalizeRejectsQuestStateSeedAloneAsQuestGateWriter(t *testing.T) {
	_, err := Canonicalize(Bundle{
		QuestState: []queststate.Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "met_guide", Value: 1}},
		StaticActors: []StaticActor{
			{Name: "GatedGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:gated_guide"},
		},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:      interactionstore.KindTalk,
			Ref:       "npc:gated_guide",
			Text:      "Welcome.",
			QuestRef:  "quest:first_steps",
			QuestFlag: "met_guide",
			QuestFrom: 1,
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle when only quest_state seed backs a gate, got %v", err)
	}
}
