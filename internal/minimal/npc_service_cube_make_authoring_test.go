package minimal

import (
	"reflect"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/cubestore"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
)

func TestNpcServiceBundleCubeMasterAddMakeConsumesGrantsAndPersists(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	hero := peerVisibilityCharacter("CubeCraftHero", 0x01030170, 0x02040170, 469575, 964200, 0, 101, 201)
	hero.Gold = 4242
	hero.Inventory = []inventory.ItemInstance{
		{ID: 2701, Vnum: 27002, Count: 1, Slot: 5},
		{ID: 2702, Vnum: 27002, Count: 1, Slot: 6},
	}
	login := "npc-cube-craft"
	issuePeerTicket(t, ticketStore, login, 0x70707170, hero)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: login, Empire: hero.Empire, Characters: cloneCharacters([]loginticket.Character{hero})}); err != nil {
		t.Fatalf("seed NPC-service cube-craft account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"},
		ticketStore,
		accounts,
		staticstore.NewMemoryStore(),
		interactionstore.NewMemoryStore(),
		itemcatalog.NewMemoryStore(),
		queststate.NewMemoryStore(),
		nil,
	)
	if err != nil {
		t.Fatalf("new NPC-service cube-craft runtime: %v", err)
	}
	currentTime := time.Unix(1_700_002_000, 0)
	runtime.now = func() time.Time { return currentTime }

	authored := loadBootstrapNpcServiceKillQuestCreditBundle(t, "bootstrap-npc-service-bundle.json")
	imported, err := runtime.ImportContentBundle(authored)
	if err != nil {
		t.Fatalf("import NPC service example bundle: %v", err)
	}
	if !reflect.DeepEqual(imported.CubeRecipes, cubestore.BootstrapSnapshot().NPCs) {
		t.Fatalf("unexpected imported NPC-service cube recipes: %#v", imported.CubeRecipes)
	}

	var guideVID, cubeVID uint32
	for _, actor := range runtime.StaticActors() {
		switch actor.Name {
		case "QuestGuide":
			guideVID = uint32(actor.EntityID)
		case "CubeMaster":
			cubeVID = uint32(actor.EntityID)
		}
	}
	if guideVID == 0 || cubeVID == 0 {
		t.Fatalf("expected imported QuestGuide and CubeMaster actors, got guide=%d cube=%d", guideVID, cubeVID)
	}

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707170)
	defer closeSessionFlow(t, flow)

	guideOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: guideVID})))
	if err != nil {
		t.Fatalf("unexpected QuestGuide interaction error: %v", err)
	}
	if len(guideOut) != 1 {
		t.Fatalf("expected 1 QuestGuide quest-flag frame, got %d", len(guideOut))
	}
	guideChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, guideOut[0]))
	if err != nil || guideChat.Type != chatproto.ChatTypeInfo || guideChat.Message != "Quest updated: first_steps.met_guide = 1." {
		t.Fatalf("unexpected QuestGuide chat: %+v err=%v", guideChat, err)
	}
	flag, ok, err := runtime.QuestStateFlag(hero.Name, "quest:first_steps", "met_guide")
	if err != nil || !ok || flag.Value != 1 {
		t.Fatalf("expected met_guide=1 after QuestGuide, got ok=%v flag=%+v err=%v", ok, flag, err)
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	cubeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: cubeVID})))
	if err != nil {
		t.Fatalf("unexpected CubeMaster interaction error: %v", err)
	}
	if len(cubeOut) != 2 {
		t.Fatalf("expected chat + cube open frames after QuestGuide unlock, got %d", len(cubeOut))
	}
	cubeChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, cubeOut[0]))
	if err != nil || cubeChat.Type != chatproto.ChatTypeInfo || cubeChat.VID != 0 || cubeChat.Empire != 0 || cubeChat.Message != "The craftsman lights the forge." {
		t.Fatalf("unexpected CubeMaster chat: %+v err=%v", cubeChat, err)
	}
	assertCubeCommandChatFrame(t, cubeOut[1], "cube open 20022", "npc-service CubeMaster open")

	firstAddOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube add 0 5",
	})))
	if err != nil {
		t.Fatalf("unexpected /cube add 0 5 after authored CubeMaster open: %v", err)
	}
	if len(firstAddOut) != 1 {
		t.Fatalf("expected /cube add 0 5 to emit one command chat frame, got %d", len(firstAddOut))
	}
	assertCubeCommandChatFrame(t, firstAddOut[0], "cube info 0 0 0", "npc-service partial cube add")
	assertNpcServiceCubeCraftUnchanged(t, runtime, accounts, login, hero, 4242, []inventory.ItemInstance{
		{ID: 2701, Vnum: 27002, Count: 1, Slot: 5},
		{ID: 2702, Vnum: 27002, Count: 1, Slot: 6},
	}, "after partial cube add")

	secondAddOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube add 1 6",
	})))
	if err != nil {
		t.Fatalf("unexpected /cube add 1 6 after authored CubeMaster open: %v", err)
	}
	if len(secondAddOut) != 1 {
		t.Fatalf("expected /cube add 1 6 to emit one command chat frame, got %d", len(secondAddOut))
	}
	assertCubeCommandChatFrame(t, secondAddOut[0], "cube info 100 0 0", "npc-service matched cube add")
	assertNpcServiceCubeCraftUnchanged(t, runtime, accounts, login, hero, 4242, []inventory.ItemInstance{
		{ID: 2701, Vnum: 27002, Count: 1, Slot: 5},
		{ID: 2702, Vnum: 27002, Count: 1, Slot: 6},
	}, "after matched cube add")

	makeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube make",
	})))
	if err != nil {
		t.Fatalf("unexpected /cube make after authored CubeMaster add: %v", err)
	}
	if len(makeOut) != 6 {
		t.Fatalf("expected authored CubeMaster /cube make burst of 6 frames, got %d", len(makeOut))
	}
	delA, err := itemproto.DecodeDel(decodeSingleFrame(t, makeOut[0]))
	if err != nil {
		t.Fatalf("decode material A ITEM_DEL: %v", err)
	}
	if delA.Position.WindowType != itemproto.WindowInventory || delA.Position.Cell != 5 {
		t.Fatalf("unexpected material A delete position: %+v", delA.Position)
	}
	delB, err := itemproto.DecodeDel(decodeSingleFrame(t, makeOut[1]))
	if err != nil {
		t.Fatalf("decode material B ITEM_DEL: %v", err)
	}
	if delB.Position.WindowType != itemproto.WindowInventory || delB.Position.Cell != 6 {
		t.Fatalf("unexpected material B delete position: %+v", delB.Position)
	}
	rewardSet, err := itemproto.DecodeSet(decodeSingleFrame(t, makeOut[2]))
	if err != nil {
		t.Fatalf("decode reward ITEM_SET: %v", err)
	}
	if rewardSet.Position.WindowType != itemproto.WindowInventory || rewardSet.Vnum != 27001 || rewardSet.Count != 1 {
		t.Fatalf("unexpected reward ITEM_SET: %+v", rewardSet)
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, makeOut[3]))
	if err != nil {
		t.Fatalf("decode gold PLAYER_POINT_CHANGE: %v", err)
	}
	if goldChange.VID != hero.VID || goldChange.Type != bootstrapGoldPointType || goldChange.Amount != -100 || goldChange.Value != 4142 {
		t.Fatalf("unexpected gold point change: %+v", goldChange)
	}
	assertCubeCommandChatFrame(t, makeOut[4], cubestore.FormatCubeSuccessCommand(27001, 1), "npc-service cube make success")
	assertCubeCommandChatFrame(t, makeOut[5], "cube info 0 0 0", "npc-service post-make cube info")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected authored CubeMaster /cube make to queue no peer frames, got %d", len(queued))
	}

	liveGold, ok := runtime.CurrencySnapshot(hero.Name)
	if !ok || liveGold.Gold != 4142 {
		t.Fatalf("expected live gold 4142 after authored cube make, got ok=%v snapshot=%+v", ok, liveGold)
	}
	liveInventory, ok := runtime.InventorySnapshot(hero.Name)
	if !ok || len(liveInventory.Inventory) != 1 || liveInventory.Inventory[0].Vnum != 27001 || liveInventory.Inventory[0].Count != 1 {
		t.Fatalf("expected live inventory one 27001 after authored cube make, got ok=%v snapshot=%+v", ok, liveInventory)
	}
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted NPC-service cube-craft account: %v", err)
	}
	if persisted.Characters[0].Gold != 4142 {
		t.Fatalf("unexpected persisted gold after authored cube make: %d", persisted.Characters[0].Gold)
	}
	if len(persisted.Characters[0].Inventory) != 1 || persisted.Characters[0].Inventory[0].Vnum != 27001 || persisted.Characters[0].Inventory[0].Count != 1 {
		t.Fatalf("unexpected persisted inventory after authored cube make: %+v", persisted.Characters[0].Inventory)
	}
	flag, ok, err = runtime.QuestStateFlag(hero.Name, "quest:first_steps", "met_guide")
	if err != nil || !ok || flag.Value != 1 {
		t.Fatalf("expected met_guide=1 after authored cube make, got ok=%v flag=%+v err=%v", ok, flag, err)
	}
}

func assertNpcServiceCubeCraftUnchanged(t *testing.T, runtime *gameRuntime, accounts accountstore.Store, login string, hero loginticket.Character, wantGold uint64, wantInventory []inventory.ItemInstance, label string) {
	t.Helper()
	liveGold, ok := runtime.CurrencySnapshot(hero.Name)
	if !ok || liveGold.Gold != wantGold {
		t.Fatalf("expected live gold %d %s, got ok=%v snapshot=%+v", wantGold, label, ok, liveGold)
	}
	liveInventory, ok := runtime.InventorySnapshot(hero.Name)
	if !ok || len(liveInventory.Inventory) != len(wantInventory) {
		t.Fatalf("expected live inventory %d items %s, got ok=%v snapshot=%+v", len(wantInventory), label, ok, liveInventory)
	}
	for i, want := range wantInventory {
		got := liveInventory.Inventory[i]
		if got.ID != want.ID || got.Vnum != want.Vnum || got.Count != want.Count || got.Slot != uint16(want.Slot) {
			t.Fatalf("unexpected live inventory[%d] %s: %+v want %+v", i, label, got, want)
		}
	}
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted NPC-service cube-craft account %s: %v", label, err)
	}
	if persisted.Characters[0].Gold != wantGold {
		t.Fatalf("unexpected persisted gold %s: %d want %d", label, persisted.Characters[0].Gold, wantGold)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted inventory %s:\n got: %+v\nwant: %+v", label, persisted.Characters[0].Inventory, wantInventory)
	}
}
