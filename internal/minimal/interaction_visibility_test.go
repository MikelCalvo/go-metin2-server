package minimal

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameRuntimeInteractionVisibilityReturnsResolvedPreviewsForVisibleInteractables(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{
		{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
		{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."},
	})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("VillageGuard", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindTalk, "npc:village_guard"); !ok {
		t.Fatal("expected talk static actor registration to succeed")
	}
	if _, ok := runtime.RegisterStaticActor("Blacksmith", bootstrapMapIndex, 1250, 2250, 20301); !ok {
		t.Fatal("expected non-interactable static actor registration to succeed")
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("Alchemist", bootstrapMapIndex, 1300, 2300, 20302, interactionstore.KindInfo, "lore:alchemist"); !ok {
		t.Fatal("expected info static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 interaction visibility snapshot, got %+v", snapshots)
	}
	if snapshots[0].Name != "PeerOne" {
		t.Fatalf("expected PeerOne interaction visibility subject, got %+v", snapshots[0])
	}
	if len(snapshots[0].VisibleInteractableStaticActors) != 2 {
		t.Fatalf("expected 2 visible interactable static actors, got %+v", snapshots[0].VisibleInteractableStaticActors)
	}
	if snapshots[0].VisibleInteractableStaticActors[0].Name != "Alchemist" || snapshots[0].VisibleInteractableStaticActors[0].Preview != "The alchemist studies forgotten herbs." {
		t.Fatalf("unexpected info interaction preview snapshot: %+v", snapshots[0].VisibleInteractableStaticActors[0])
	}
	if snapshots[0].VisibleInteractableStaticActors[1].Name != "VillageGuard" || snapshots[0].VisibleInteractableStaticActors[1].Preview != "VillageGuard:\nKeep your blade sharp." {
		t.Fatalf("unexpected talk interaction preview snapshot: %+v", snapshots[0].VisibleInteractableStaticActors[1])
	}
}

func TestGameRuntimeInteractionVisibilityReportsResolutionFailureForDanglingDefinition(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, nil)

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.sharedWorld.RegisterStaticActorWithInteraction(0, "Alchemist", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindInfo, "lore:missing"); !ok {
		t.Fatal("expected direct shared-world registration with dangling ref to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 || len(snapshots[0].VisibleInteractableStaticActors) != 1 {
		t.Fatalf("expected one dangling-definition interaction visibility entry, got %+v", snapshots)
	}
	entry := snapshots[0].VisibleInteractableStaticActors[0]
	if entry.Name != "Alchemist" || entry.Preview != "" || entry.ResolutionFailure != staticActorInteractionFailureDefinitionNotFound {
		t.Fatalf("unexpected dangling-definition interaction visibility snapshot: %+v", entry)
	}
}

func TestGameRuntimeInteractionVisibilityReturnsServicePreviewsForVisibleWarpAndShopPreviewActors(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{
		defaultMerchantCatalogDefinition(),
		{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", MapIndex: 42, X: 1700, Y: 2800, Text: "Step through the gate."},
	})
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("Merchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, "npc:merchant"); !ok {
		t.Fatal("expected shop preview static actor registration to succeed")
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("Teleporter", bootstrapMapIndex, 1250, 2250, 20301, interactionstore.KindWarp, "npc:teleporter"); !ok {
		t.Fatal("expected warp static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 interaction visibility snapshot, got %+v", snapshots)
	}
	entries := snapshots[0].VisibleInteractableStaticActors
	if len(entries) != 2 {
		t.Fatalf("expected 2 visible service interactables, got %+v", entries)
	}
	byName := make(map[string]InteractableStaticActorVisibilitySnapshot, len(entries))
	for _, entry := range entries {
		byName[entry.Name] = entry
	}
	merchant, ok := byName["Merchant"]
	if !ok {
		t.Fatalf("expected Merchant interaction visibility entry, got %+v", entries)
	}
	if merchant.Preview != defaultMerchantPreview || merchant.ResolutionFailure != "" {
		t.Fatalf("unexpected shop preview interaction visibility entry: %+v", merchant)
	}
	teleporter, ok := byName["Teleporter"]
	if !ok {
		t.Fatalf("expected Teleporter interaction visibility entry, got %+v", entries)
	}
	if teleporter.Preview != "Step through the gate. [warp -> map 42 @ 1700,2800]" || teleporter.ResolutionFailure != "" {
		t.Fatalf("unexpected warp interaction visibility entry: %+v", teleporter)
	}
}

func TestGameRuntimeInteractionVisibilityReturnsQuestGatedWarpMismatchPreviewWithoutMutatingQuestState(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	before := queststate.Snapshot{Flags: []queststate.Flag{{Character: "PeerOne", QuestRef: "quest:first_steps", Name: "met_guide", Value: 1}}}
	if err := queststate.NewFileStore(questStatePath).Save(before); err != nil {
		t.Fatalf("seed quest state: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind:      interactionstore.KindWarp,
		Ref:       "npc:gated_teleporter",
		Text:      "Step through the gate.",
		MapIndex:  42,
		X:         1700,
		Y:         2800,
		QuestRef:  "quest:first_steps",
		QuestFlag: "met_guide",
		QuestFrom: 0,
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("Teleporter", bootstrapMapIndex, 1250, 2250, 20301, interactionstore.KindWarp, "npc:gated_teleporter"); !ok {
		t.Fatal("expected gated warp static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 || len(snapshots[0].VisibleInteractableStaticActors) != 1 {
		t.Fatalf("expected one visible gated warp interactable, got %+v", snapshots)
	}
	entry := snapshots[0].VisibleInteractableStaticActors[0]
	if entry.Name != "Teleporter" || entry.Preview != "Quest requirements are not met." || entry.ResolutionFailure != "" {
		t.Fatalf("unexpected gated warp mismatch interaction visibility entry: %+v", entry)
	}
	loaded, err := queststate.NewFileStore(questStatePath).Load()
	if err != nil {
		t.Fatalf("load quest state after gated warp visibility preview: %v", err)
	}
	if !reflect.DeepEqual(loaded, before) {
		t.Fatalf("gated warp interaction visibility preview mutated quest-state:\n got: %#v\nwant: %#v", loaded, before)
	}
}

func TestGameRuntimeInteractionVisibilityReturnsQuestGatedShopMismatchPreviewWithoutMutatingQuestState(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	before := queststate.Snapshot{Flags: []queststate.Flag{{Character: "PeerOne", QuestRef: "quest:first_steps", Name: "met_guide", Value: 1}}}
	if err := queststate.NewFileStore(questStatePath).Save(before); err != nil {
		t.Fatalf("seed quest state: %v", err)
	}
	catalog := defaultMerchantCatalogDefinition()
	catalog.Ref = "npc:gated_merchant"
	catalog.Title = "Gated Merchant"
	catalog.QuestRef = "quest:first_steps"
	catalog.QuestFlag = "met_guide"
	catalog.QuestFrom = 0
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{catalog})
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath}, store, nil, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("Merchant", bootstrapMapIndex, 1250, 2250, 20301, interactionstore.KindShopPreview, "npc:gated_merchant"); !ok {
		t.Fatal("expected gated shop static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 || len(snapshots[0].VisibleInteractableStaticActors) != 1 {
		t.Fatalf("expected one visible gated shop interactable, got %+v", snapshots)
	}
	entry := snapshots[0].VisibleInteractableStaticActors[0]
	if entry.Name != "Merchant" || entry.Preview != "Quest requirements are not met." || entry.ResolutionFailure != "" {
		t.Fatalf("unexpected gated shop mismatch interaction visibility entry: %+v", entry)
	}
	loaded, err := queststate.NewFileStore(questStatePath).Load()
	if err != nil {
		t.Fatalf("load quest state after gated shop visibility preview: %v", err)
	}
	if !reflect.DeepEqual(loaded, before) {
		t.Fatalf("gated shop interaction visibility preview mutated quest-state:\n got: %#v\nwant: %#v", loaded, before)
	}
}

func TestGameRuntimeInteractionVisibilityReturnsQuestGatedTalkMismatchPreviewWithoutMutatingQuestState(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	before := queststate.Snapshot{Flags: []queststate.Flag{{Character: "PeerOne", QuestRef: "quest:first_steps", Name: "met_guide", Value: 1}}}
	if err := queststate.NewFileStore(questStatePath).Save(before); err != nil {
		t.Fatalf("seed quest state: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind:      interactionstore.KindTalk,
		Ref:       "npc:gated_guide",
		Text:      "Welcome to the gated square.",
		QuestRef:  "quest:first_steps",
		QuestFlag: "met_guide",
		QuestFrom: 0,
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("VillageGuide", bootstrapMapIndex, 1250, 2250, 20301, interactionstore.KindTalk, "npc:gated_guide"); !ok {
		t.Fatal("expected gated talk static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 || len(snapshots[0].VisibleInteractableStaticActors) != 1 {
		t.Fatalf("expected one visible gated talk interactable, got %+v", snapshots)
	}
	entry := snapshots[0].VisibleInteractableStaticActors[0]
	if entry.Name != "VillageGuide" || entry.Preview != "Quest requirements are not met." || entry.ResolutionFailure != "" {
		t.Fatalf("unexpected gated talk mismatch interaction visibility entry: %+v", entry)
	}
	loaded, err := queststate.NewFileStore(questStatePath).Load()
	if err != nil {
		t.Fatalf("load quest state after gated talk visibility preview: %v", err)
	}
	if !reflect.DeepEqual(loaded, before) {
		t.Fatalf("gated talk interaction visibility preview mutated quest-state:\n got: %#v\nwant: %#v", loaded, before)
	}
}

func TestGameRuntimeInteractionVisibilityReturnsQuestGatedInfoMismatchPreviewWithoutMutatingQuestState(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	before := queststate.Snapshot{Flags: []queststate.Flag{{Character: "PeerOne", QuestRef: "quest:first_steps", Name: "met_guide", Value: 1}}}
	if err := queststate.NewFileStore(questStatePath).Save(before); err != nil {
		t.Fatalf("seed quest state: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind:      interactionstore.KindInfo,
		Ref:       "lore:gated_signpost",
		Text:      "The gated signpost describes the square.",
		QuestRef:  "quest:first_steps",
		QuestFlag: "met_guide",
		QuestFrom: 0,
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("VillageSignpost", bootstrapMapIndex, 1250, 2250, 20301, interactionstore.KindInfo, "lore:gated_signpost"); !ok {
		t.Fatal("expected gated info static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 || len(snapshots[0].VisibleInteractableStaticActors) != 1 {
		t.Fatalf("expected one visible gated info interactable, got %+v", snapshots)
	}
	entry := snapshots[0].VisibleInteractableStaticActors[0]
	if entry.Name != "VillageSignpost" || entry.Preview != "Quest requirements are not met." || entry.ResolutionFailure != "" {
		t.Fatalf("unexpected gated info mismatch interaction visibility entry: %+v", entry)
	}
	loaded, err := queststate.NewFileStore(questStatePath).Load()
	if err != nil {
		t.Fatalf("load quest state after gated info visibility preview: %v", err)
	}
	if !reflect.DeepEqual(loaded, before) {
		t.Fatalf("gated info interaction visibility preview mutated quest-state:\n got: %#v\nwant: %#v", loaded, before)
	}
}

func TestGameRuntimeInteractionVisibilityReturnsQuestFlagPreview(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind:      interactionstore.KindQuestFlag,
		Ref:       "quest:first_steps",
		Text:      "Quest updated: first_steps.met_guide = 1.",
		QuestRef:  "quest:first_steps",
		QuestFlag: "met_guide",
		QuestTo:   1,
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("QuestGuide", bootstrapMapIndex, 1250, 2250, 20301, interactionstore.KindQuestFlag, "quest:first_steps"); !ok {
		t.Fatal("expected quest flag static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 || len(snapshots[0].VisibleInteractableStaticActors) != 1 {
		t.Fatalf("expected one visible quest-flag interactable, got %+v", snapshots)
	}
	entry := snapshots[0].VisibleInteractableStaticActors[0]
	if entry.Name != "QuestGuide" || entry.Preview != "Quest updated: first_steps.met_guide = 1." || entry.ResolutionFailure != "" {
		t.Fatalf("unexpected quest-flag interaction visibility entry: %+v", entry)
	}
}

func TestGameRuntimeInteractionVisibilityReturnsQuestFlagRewardGoldPreview(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030121, 0x02040121, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one-reward-gold", 0x15151515, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind:             interactionstore.KindQuestFlag,
		Ref:              "quest:first_steps_kill_turnin",
		Text:             "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:         "quest:first_steps",
		QuestFlag:        "killed_qa_mob",
		QuestFrom:        1,
		QuestTo:          0,
		RewardExperience: 50,
		RewardGold:       100,
		RewardItems: []interactionstore.RewardItemEntry{
			{ItemVnum: 27001, Count: 1},
			{ItemVnum: 11200, Count: 1},
		},
	}})
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, ShopBuyPrice: 50},
	}}); err != nil {
		t.Fatalf("seed quest-flag reward-item preview templates: %v", err)
	}
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	if err := queststate.NewFileStore(questStatePath).Save(queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "PeerOne",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}); err != nil {
		t.Fatalf("seed quest-state for reward-gold preview: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath}, store, nil, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1250, 2250, 20301, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin"); !ok {
		t.Fatal("expected quest-flag reward-gold static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one-reward-gold", 0x15151515)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 || len(snapshots[0].VisibleInteractableStaticActors) != 1 {
		t.Fatalf("expected one visible quest-flag reward-gold interactable, got %+v", snapshots)
	}
	entry := snapshots[0].VisibleInteractableStaticActors[0]
	if entry.Name != "QuestHunter" || entry.Preview != "Quest updated: first_steps.killed_qa_mob = 0. [reward_gold 100] [reward_experience 50] [reward_item Small Red Potion x1] [reward_item Wooden Sword x1]" || entry.ResolutionFailure != "" {
		t.Fatalf("unexpected quest-flag reward-gold interaction visibility entry: %+v", entry)
	}
}

func TestGameRuntimeInteractionVisibilityReturnsQuestFlagConsumeItemPreview(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030122, 0x02040122, 1100, 2100, 0, 101, 201)
	peer.Inventory = []inventory.ItemInstance{{ID: 51, Vnum: 27001, Count: 1, Slot: 0}}
	peer.Gold = 40
	issuePeerTicket(t, store, "peer-one-consume-items", 0x16161616, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind:             interactionstore.KindQuestFlag,
		Ref:              "quest:first_steps_kill_turnin",
		Text:             "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:         "quest:first_steps",
		QuestFlag:        "killed_qa_mob",
		QuestFrom:        1,
		QuestTo:          0,
		RewardExperience: 50,
		RewardGold:       100,
		RewardItems: []interactionstore.RewardItemEntry{
			{ItemVnum: 11200, Count: 1},
		},
		ConsumeItems: []interactionstore.RewardItemEntry{
			{ItemVnum: 27001, Count: 1},
		},
		ConsumeGold: 25,
	}})
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, ShopBuyPrice: 50},
	}}); err != nil {
		t.Fatalf("seed quest-flag consume-item preview templates: %v", err)
	}
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	if err := queststate.NewFileStore(questStatePath).Save(queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "PeerOne",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}); err != nil {
		t.Fatalf("seed quest-state for consume-item preview: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath}, store, nil, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1250, 2250, 20301, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin"); !ok {
		t.Fatal("expected quest-flag consume-item static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one-consume-items", 0x16161616)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 || len(snapshots[0].VisibleInteractableStaticActors) != 1 {
		t.Fatalf("expected one visible quest-flag consume-item interactable, got %+v", snapshots)
	}
	entry := snapshots[0].VisibleInteractableStaticActors[0]
	if entry.Name != "QuestHunter" || entry.Preview != "Quest updated: first_steps.killed_qa_mob = 0. [reward_gold 100] [reward_experience 50] [reward_item Wooden Sword x1] [consume_gold 25] [consume_item Small Re..." || entry.ResolutionFailure != "" {
		t.Fatalf("unexpected quest-flag consume-item interaction visibility entry: %+v", entry)
	}
}

func TestGameRuntimeInteractionVisibilityReturnsQuestFlagConsumeGoldPreview(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030123, 0x02040123, 1100, 2100, 0, 101, 201)
	peer.Inventory = []inventory.ItemInstance{{ID: 52, Vnum: 27001, Count: 1, Slot: 0}}
	peer.Gold = 40
	issuePeerTicket(t, store, "peer-one-consume-gold", 0x17171717, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind:             interactionstore.KindQuestFlag,
		Ref:              "quest:first_steps_kill_turnin",
		Text:             "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:         "quest:first_steps",
		QuestFlag:        "killed_qa_mob",
		QuestFrom:        1,
		QuestTo:          0,
		RewardExperience: 50,
		RewardGold:       100,
		RewardItems: []interactionstore.RewardItemEntry{
			{ItemVnum: 11200, Count: 1},
		},
		ConsumeItems: []interactionstore.RewardItemEntry{
			{ItemVnum: 27001, Count: 1},
		},
		ConsumeGold: 25,
	}})
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, ShopBuyPrice: 50},
	}}); err != nil {
		t.Fatalf("seed quest-flag consume-gold preview templates: %v", err)
	}
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	if err := queststate.NewFileStore(questStatePath).Save(queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "PeerOne",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}); err != nil {
		t.Fatalf("seed quest-state for consume-gold preview: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath}, store, nil, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1250, 2250, 20301, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin"); !ok {
		t.Fatal("expected quest-flag consume-gold static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one-consume-gold", 0x17171717)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 || len(snapshots[0].VisibleInteractableStaticActors) != 1 {
		t.Fatalf("expected one visible quest-flag consume-gold interactable, got %+v", snapshots)
	}
	entry := snapshots[0].VisibleInteractableStaticActors[0]
	if entry.Name != "QuestHunter" || entry.Preview != "Quest updated: first_steps.killed_qa_mob = 0. [reward_gold 100] [reward_experience 50] [reward_item Wooden Sword x1] [consume_gold 25] [consume_item Small Re..." || entry.ResolutionFailure != "" {
		t.Fatalf("unexpected quest-flag consume-gold interaction visibility entry: %+v", entry)
	}
}

func TestGameRuntimeInteractionVisibilityReturnsQuestFlagInsufficientConsumeGoldPreview(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030124, 0x02040124, 1100, 2100, 0, 101, 201)
	peer.Inventory = []inventory.ItemInstance{{ID: 53, Vnum: 27001, Count: 1, Slot: 0}}
	peer.Gold = 10
	issuePeerTicket(t, store, "peer-one-consume-gold-miss", 0x18181818, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind:             interactionstore.KindQuestFlag,
		Ref:              "quest:first_steps_kill_turnin",
		Text:             "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:         "quest:first_steps",
		QuestFlag:        "killed_qa_mob",
		QuestFrom:        1,
		QuestTo:          0,
		RewardExperience: 50,
		RewardGold:       100,
		RewardItems: []interactionstore.RewardItemEntry{
			{ItemVnum: 11200, Count: 1},
		},
		ConsumeItems: []interactionstore.RewardItemEntry{
			{ItemVnum: 27001, Count: 1},
		},
		ConsumeGold: 25,
	}})
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, ShopBuyPrice: 50},
	}}); err != nil {
		t.Fatalf("seed quest-flag consume-gold mismatch preview templates: %v", err)
	}
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	if err := queststate.NewFileStore(questStatePath).Save(queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "PeerOne",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}); err != nil {
		t.Fatalf("seed quest-state for consume-gold mismatch preview: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath}, store, nil, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1250, 2250, 20301, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin"); !ok {
		t.Fatal("expected quest-flag consume-gold mismatch static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one-consume-gold-miss", 0x18181818)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 || len(snapshots[0].VisibleInteractableStaticActors) != 1 {
		t.Fatalf("expected one visible quest-flag consume-gold mismatch interactable, got %+v", snapshots)
	}
	entry := snapshots[0].VisibleInteractableStaticActors[0]
	if entry.Name != "QuestHunter" || entry.Preview != "Quest requirements are not met." || entry.ResolutionFailure != "" {
		t.Fatalf("unexpected quest-flag consume-gold mismatch interaction visibility entry: %+v", entry)
	}
}

func TestGameRuntimeInteractionVisibilityReturnsQuestFlagMismatchPreviewWithoutMutatingQuestState(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	before := queststate.Snapshot{Flags: []queststate.Flag{{Character: "PeerOne", QuestRef: "quest:first_steps", Name: "met_guide", Value: 1}}}
	if err := queststate.NewFileStore(questStatePath).Save(before); err != nil {
		t.Fatalf("seed quest state: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind:      interactionstore.KindQuestFlag,
		Ref:       "quest:first_steps",
		Text:      "Quest updated: first_steps.met_guide = 1.",
		QuestRef:  "quest:first_steps",
		QuestFlag: "met_guide",
		QuestTo:   1,
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("QuestGuide", bootstrapMapIndex, 1250, 2250, 20301, interactionstore.KindQuestFlag, "quest:first_steps"); !ok {
		t.Fatal("expected quest flag static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 || len(snapshots[0].VisibleInteractableStaticActors) != 1 {
		t.Fatalf("expected one visible quest-flag interactable, got %+v", snapshots)
	}
	entry := snapshots[0].VisibleInteractableStaticActors[0]
	if entry.Name != "QuestGuide" || entry.Preview != "Quest requirements are not met." || entry.ResolutionFailure != "" {
		t.Fatalf("unexpected quest-flag mismatch interaction visibility entry: %+v", entry)
	}
	loaded, err := queststate.NewFileStore(questStatePath).Load()
	if err != nil {
		t.Fatalf("load quest state after interaction-visibility preview: %v", err)
	}
	if !reflect.DeepEqual(loaded, before) {
		t.Fatalf("quest-flag interaction visibility preview mutated quest-state:\n got: %#v\nwant: %#v", loaded, before)
	}
}

func TestGameRuntimeInteractionVisibilityReturnsQuestFlagRewardInventoryFullPreviewWithoutMutatingQuestState(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	peer := peerVisibilityCharacter("PeerOne", 0x01030123, 0x02040123, 1100, 2100, 0, 101, 201)
	peer.Inventory = merchantBuyerInventoryLeavingOneFreeSlot()
	issuePeerTicket(t, store, "peer-one-reward-full", 0x17171717, peer)
	before := queststate.Snapshot{Flags: []queststate.Flag{{Character: "PeerOne", QuestRef: "quest:first_steps", Name: "killed_qa_mob", Value: 1}}}
	if err := queststate.NewFileStore(questStatePath).Save(before); err != nil {
		t.Fatalf("seed quest state: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind:      interactionstore.KindQuestFlag,
		Ref:       "quest:first_steps_kill_turnin",
		Text:      "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:  "quest:first_steps",
		QuestFlag: "killed_qa_mob",
		QuestFrom: 1,
		QuestTo:   0,
		RewardItems: []interactionstore.RewardItemEntry{
			{ItemVnum: 27001, Count: 1},
			{ItemVnum: 11200, Count: 1},
		},
	}})
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, ShopBuyPrice: 50},
	}}); err != nil {
		t.Fatalf("seed reward inventory-full preview templates: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath}, store, nil, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1250, 2250, 20301, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin"); !ok {
		t.Fatal("expected quest flag static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one-reward-full", 0x17171717)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 || len(snapshots[0].VisibleInteractableStaticActors) != 1 {
		t.Fatalf("expected one visible quest-flag reward inventory-full interactable, got %+v", snapshots)
	}
	entry := snapshots[0].VisibleInteractableStaticActors[0]
	if entry.Name != "QuestHunter" || entry.Preview != itemPickupInventoryFullInfoMessage || entry.ResolutionFailure != "" {
		t.Fatalf("unexpected quest-flag reward inventory-full interaction visibility entry: %+v", entry)
	}
	loaded, err := queststate.NewFileStore(questStatePath).Load()
	if err != nil {
		t.Fatalf("load quest state after reward inventory-full preview: %v", err)
	}
	if !reflect.DeepEqual(loaded, before) {
		t.Fatalf("quest-flag reward inventory-full preview mutated quest-state:\n got: %#v\nwant: %#v", loaded, before)
	}
}

func TestGameRuntimeInteractionVisibilityReturnsQuestFlagRewardRestrictedPreviewWithoutMutatingQuestState(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	peer := peerVisibilityCharacter("PeerOne", 0x01030124, 0x02040124, 1100, 2100, 0, 101, 201)
	peer.Level = 5
	issuePeerTicket(t, store, "peer-one-reward-restricted", 0x18181818, peer)
	before := queststate.Snapshot{Flags: []queststate.Flag{{Character: "PeerOne", QuestRef: "quest:first_steps", Name: "killed_qa_mob", Value: 1}}}
	if err := queststate.NewFileStore(questStatePath).Save(before); err != nil {
		t.Fatalf("seed quest state: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind:            interactionstore.KindQuestFlag,
		Ref:             "quest:first_steps_kill_turnin",
		Text:            "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:        "quest:first_steps",
		QuestFlag:       "killed_qa_mob",
		QuestFrom:       1,
		QuestTo:         0,
		RewardItemVnum:  27001,
		RewardItemCount: 1,
	}})
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum: 27001, Name: "High-Level Reward Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5, MinLevel: 10,
		BuyRejectText: "This reward is sealed against you.",
	}}}); err != nil {
		t.Fatalf("seed reward restricted preview templates: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath}, store, nil, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1250, 2250, 20301, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin"); !ok {
		t.Fatal("expected quest flag static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one-reward-restricted", 0x18181818)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 || len(snapshots[0].VisibleInteractableStaticActors) != 1 {
		t.Fatalf("expected one visible quest-flag reward restricted interactable, got %+v", snapshots)
	}
	entry := snapshots[0].VisibleInteractableStaticActors[0]
	if entry.Name != "QuestHunter" || entry.Preview != "This reward is sealed against you." || entry.ResolutionFailure != "" {
		t.Fatalf("unexpected quest-flag reward restricted interaction visibility entry: %+v", entry)
	}
	loaded, err := queststate.NewFileStore(questStatePath).Load()
	if err != nil {
		t.Fatalf("load quest state after reward restricted preview: %v", err)
	}
	if !reflect.DeepEqual(loaded, before) {
		t.Fatalf("quest-flag reward restricted preview mutated quest-state:\n got: %#v\nwant: %#v", loaded, before)
	}
}

func TestGameRuntimeInteractionVisibilityReturnsWarpDestinationPreviewWhenWarpTextIsBlank(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", MapIndex: 42, X: 1700, Y: 2800, Text: "   "}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("Teleporter", bootstrapMapIndex, 1250, 2250, 20301, interactionstore.KindWarp, "npc:teleporter"); !ok {
		t.Fatal("expected warp static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 || len(snapshots[0].VisibleInteractableStaticActors) != 1 {
		t.Fatalf("expected one visible warp interactable, got %+v", snapshots)
	}
	entry := snapshots[0].VisibleInteractableStaticActors[0]
	if entry.Name != "Teleporter" || entry.Preview != "warp -> map 42 @ 1700,2800" || entry.ResolutionFailure != "" {
		t.Fatalf("unexpected blank-text warp interaction visibility entry: %+v", entry)
	}
}

func TestGameRuntimeInteractionVisibilityCompactsUnicodePreviewsOnRuneBoundaries(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	longText := strings.Repeat("界", 200)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindInfo, Ref: "lore:unicode_notice", Text: longText}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.RegisterStaticActorWithInteraction("UnicodeNotice", bootstrapMapIndex, 1250, 2250, 20301, interactionstore.KindInfo, "lore:unicode_notice"); !ok {
		t.Fatal("expected unicode notice actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 || len(snapshots[0].VisibleInteractableStaticActors) != 1 {
		t.Fatalf("expected one visible unicode interactable, got %+v", snapshots)
	}
	preview := snapshots[0].VisibleInteractableStaticActors[0].Preview
	want := strings.Repeat("界", 157) + "..."
	if preview != want || !utf8.ValidString(preview) {
		t.Fatalf("unexpected unicode compact preview valid_utf8=%v runes=%d preview=%q", utf8.ValidString(preview), len([]rune(preview)), preview)
	}
}

func TestGameRuntimeInteractionVisibilityMarksDeadInteractableStaticActor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:fallen_guard", Text: "..."}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.sharedWorld.registerStaticActor(0, "FallenGuard", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindTalk, "npc:fallen_guard", worldruntime.StaticActorCombatKindTrainingDummy, "", worldruntime.StaticActorDeathReward{})
	if !ok {
		t.Fatal("expected interactable combat actor registration to succeed")
	}
	runtime.sharedWorld.mu.Lock()
	runtime.sharedWorld.staticActorCombatHP[actor.EntityID] = 0
	runtime.sharedWorld.mu.Unlock()
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	snapshots := runtime.InteractionVisibility()
	if len(snapshots) != 1 || len(snapshots[0].VisibleInteractableStaticActors) != 1 {
		t.Fatalf("expected one visible dead interactable, got %+v", snapshots)
	}
	entry := snapshots[0].VisibleInteractableStaticActors[0]
	if entry.Name != "FallenGuard" || !entry.Dead || entry.Preview != "FallenGuard:\n..." || entry.ResolutionFailure != "" {
		t.Fatalf("expected dead interactable snapshot to preserve state and preview, got %+v", entry)
	}
}

func TestGameRuntimeInteractionVisibilitySnapshotReturnsExactConnectedCharacter(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	alpha := peerVisibilityCharacter("Alpha", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	beta := peerVisibilityCharacter("Beta", 0x01030102, 0x02040102, 1200, 2200, 0, 102, 202)
	issuePeerTicket(t, store, "alpha", 0x11111111, alpha)
	issuePeerTicket(t, store, "beta", 0x22222222, beta)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{
		{Kind: interactionstore.KindInfo, Ref: "lore:guide", Text: "Read the village sign."},
	})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("VillageSign", bootstrapMapIndex, 1150, 2150, 20300, interactionstore.KindInfo, "lore:guide")
	if !ok {
		t.Fatal("expected static actor registration to succeed")
	}
	factory := runtime.SessionFactory()
	alphaFlow, _ := enterGameWithLoginTicket(t, factory, "alpha", 0x11111111)
	defer closeSessionFlow(t, alphaFlow)
	betaFlow, _ := enterGameWithLoginTicket(t, factory, "beta", 0x22222222)
	defer closeSessionFlow(t, betaFlow)

	snapshot, ok := runtime.InteractionVisibilitySnapshot("Alpha")
	if !ok {
		t.Fatal("expected exact Alpha interaction visibility snapshot to resolve")
	}
	if snapshot.Name != "Alpha" || snapshot.VID != alpha.VID || snapshot.MapIndex != bootstrapMapIndex {
		t.Fatalf("unexpected Alpha interaction visibility subject snapshot: %+v", snapshot)
	}
	if len(snapshot.VisibleInteractableStaticActors) != 1 {
		t.Fatalf("expected exact Alpha interaction visibility snapshot to include one interactable, got %+v", snapshot.VisibleInteractableStaticActors)
	}
	visible := snapshot.VisibleInteractableStaticActors[0]
	if visible.EntityID != actor.EntityID || visible.Name != "VillageSign" || visible.Preview != "Read the village sign." || visible.ResolutionFailure != "" {
		t.Fatalf("unexpected exact Alpha interaction visibility actor snapshot: %+v", visible)
	}

	if missing, ok := runtime.InteractionVisibilitySnapshot("Missing"); ok || missing.Name != "" {
		t.Fatalf("expected missing exact interaction visibility snapshot to fail closed, got snapshot=%+v ok=%v", missing, ok)
	}
}
