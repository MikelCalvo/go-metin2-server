package minimal

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPostFloorItemGiveFailsClosedBeforeAntiGiveFeedback(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DeadGiveOwner", 0x01030a40, 0x02040a40, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 801, Vnum: 27042, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}
	peer := peerVisibilityCharacter("DeadGivePeer", 0x01030a41, 0x02040a41, 1120, 2120, 0, 101, 201)
	login := "post-floor-item-give-owner"
	loginKey := uint32(0x19191a40)
	peerLogin := "post-floor-item-give-peer"
	peerLoginKey := uint32(0x19191a41)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, peerLogin, peerLoginKey, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor item-give owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed post-floor item-give peer account: %v", err)
	}

	template := itemcatalog.Template{
		Vnum:           27042,
		Name:           "Dead Guard Gift Potion",
		Stackable:      true,
		MaxCount:       200,
		AntiGive:       true,
		GiveRejectText: "You cannot give this item.",
	}
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected post-floor item-give runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_item_give",
		Name:          "PracticeMobPostFloorItemGive",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import post-floor item-give practice mob: %v", err)
	}
	if err := runtime.replaceItemTemplates(itemcatalog.Snapshot{Templates: []itemcatalog.Template{template}}); err != nil {
		t.Fatalf("restore post-floor item-give templates after content import: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor item-give practice mob, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) < 8 {
		t.Fatalf("expected owner bootstrap with visible practice mob, got %d frames", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, peerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, peerLoginKey)
	if len(peerEnter) < 11 {
		t.Fatalf("expected peer bootstrap with visible owner and mob, got %d frames", len(peerEnter))
	}
	defer closeSessionFlow(t, peerFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive peer-entry frames before post-floor ITEM_GIVE, got %d", len(queued))
	}
	_ = flushServerFrames(t, peerFlow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, ownerFlow, owner, targetVID)
	if queued := flushServerFrames(t, peerFlow); len(queued) != 1 {
		t.Fatalf("expected peer DEAD fanout after owner floor before ITEM_GIVE, got %d", len(queued))
	}

	givePacket := itemproto.EncodeClientGive(itemproto.ClientGivePacket{
		TargetVID: peer.VID,
		Position:  itemproto.InventoryPosition(5),
		Count:     1,
	})
	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, givePacket))
	if err != nil {
		t.Fatalf("unexpected post-floor ITEM_GIVE dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor ITEM_GIVE to fail closed before anti-give feedback, got %d frames", len(out))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected post-floor ITEM_GIVE to queue no owner frames, got %d", len(queued))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected post-floor ITEM_GIVE to queue no peer frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor ITEM_GIVE")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "post-floor ITEM_GIVE peer")

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor ITEM_GIVE: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor ITEM_GIVE, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, peerFlow)

	reuseOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, givePacket))
	if err != nil {
		t.Fatalf("unexpected post-restart ITEM_GIVE: %v", err)
	}
	if len(reuseOut) != 1 {
		t.Fatalf("expected post-restart anti-give ITEM_GIVE to emit one info-chat frame, got %d", len(reuseOut))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, reuseOut[0]))
	if err != nil {
		t.Fatalf("decode post-restart anti-give ITEM_GIVE rejection info chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != template.GiveRejectText {
		t.Fatalf("unexpected post-restart anti-give ITEM_GIVE rejection chat: %+v", delivery)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no queued owner frames after post-restart anti-give ITEM_GIVE, got %d", len(queued))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames after post-restart anti-give ITEM_GIVE, got %d", len(queued))
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart ITEM_GIVE: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after ITEM_GIVE floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart ITEM_GIVE leaves inventory unchanged")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "post-restart ITEM_GIVE peer unchanged")
}

func TestGameSessionFlowPostFloorItemGiveFailsClosedBeforeRestartTown(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DeadGiveTownOwner", 0x01030a42, 0x02040a42, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 802, Vnum: 27042, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}
	sourcePeer := peerVisibilityCharacter("DeadGiveTownSource", 0x01030a43, 0x02040a43, 1120, 2120, 0, 101, 201)
	townPeer := peerVisibilityCharacter("DeadGiveTownPeer", 0x01030a44, 0x02040a44, 52070, 166600, 4, 103, 203)
	townPeer.MapIndex = 21
	login := "pf-item-give-town"
	loginKey := uint32(0x19191a42)
	sourceLogin := "pf-item-give-town-s"
	sourceLoginKey := uint32(0x19191a43)
	townLogin := "pf-item-give-town-t"
	townLoginKey := uint32(0x19191a44)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, sourceLogin, sourceLoginKey, sourcePeer)
	issuePeerTicket(t, ticketStore, townLogin, townLoginKey, townPeer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor town item-give owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: sourceLogin, Empire: sourcePeer.Empire, Characters: cloneCharacters([]loginticket.Character{sourcePeer})}); err != nil {
		t.Fatalf("seed post-floor town item-give source peer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: townLogin, Empire: townPeer.Empire, Characters: cloneCharacters([]loginticket.Character{townPeer})}); err != nil {
		t.Fatalf("seed post-floor town item-give town peer account: %v", err)
	}

	template := itemcatalog.Template{
		Vnum:           27042,
		Name:           "Dead Guard Gift Potion",
		Stackable:      true,
		MaxCount:       200,
		AntiGive:       true,
		GiveRejectText: "You cannot give this item.",
	}
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected post-floor town item-give runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_item_give_town",
		Name:          "PracticeMobPostFloorItemGiveTown",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import post-floor town item-give practice mob: %v", err)
	}
	if err := runtime.replaceItemTemplates(itemcatalog.Snapshot{Templates: []itemcatalog.Template{template}}); err != nil {
		t.Fatalf("restore post-floor town item-give templates after content import: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor town item-give practice mob, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) < 8 {
		t.Fatalf("expected owner bootstrap with visible practice mob, got %d frames", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	sourceFlow, sourceEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), sourceLogin, sourceLoginKey)
	if len(sourceEnter) < 11 {
		t.Fatalf("expected source peer bootstrap with visible owner and mob, got %d frames", len(sourceEnter))
	}
	defer closeSessionFlow(t, sourceFlow)
	townFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), townLogin, townLoginKey)
	defer closeSessionFlow(t, townFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive source peer-entry frames before post-floor town ITEM_GIVE, got %d", len(queued))
	}
	_ = flushServerFrames(t, sourceFlow)
	_ = flushServerFrames(t, townFlow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, ownerFlow, owner, targetVID)
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 1 {
		t.Fatalf("expected source peer DEAD fanout after owner floor before town ITEM_GIVE, got %d", len(queued))
	}

	sourceGivePacket := itemproto.EncodeClientGive(itemproto.ClientGivePacket{
		TargetVID: sourcePeer.VID,
		Position:  itemproto.InventoryPosition(5),
		Count:     1,
	})
	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, sourceGivePacket))
	if err != nil {
		t.Fatalf("unexpected post-floor town ITEM_GIVE dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town ITEM_GIVE to fail closed before anti-give feedback, got %d frames", len(out))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected post-floor town ITEM_GIVE to queue no owner frames, got %d", len(queued))
	}
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 0 {
		t.Fatalf("expected post-floor town ITEM_GIVE to queue no source peer frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town ITEM_GIVE")
	assertExchangeAccountUnchanged(t, accounts, sourceLogin, sourcePeer, "post-floor town ITEM_GIVE source peer")

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor ITEM_GIVE: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor ITEM_GIVE, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor ITEM_GIVE /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after ITEM_GIVE floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after ITEM_GIVE floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after ITEM_GIVE floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, sourceFlow)
	_ = flushServerFrames(t, townFlow)

	townGivePacket := itemproto.EncodeClientGive(itemproto.ClientGivePacket{
		TargetVID: townPeer.VID,
		Position:  itemproto.InventoryPosition(5),
		Count:     1,
	})
	reuseOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, townGivePacket))
	if err != nil {
		t.Fatalf("unexpected post-restart_town ITEM_GIVE: %v", err)
	}
	if len(reuseOut) != 1 {
		t.Fatalf("expected post-restart_town anti-give ITEM_GIVE to emit one info-chat frame, got %d", len(reuseOut))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, reuseOut[0]))
	if err != nil {
		t.Fatalf("decode post-restart_town anti-give ITEM_GIVE rejection info chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != template.GiveRejectText {
		t.Fatalf("unexpected post-restart_town anti-give ITEM_GIVE rejection chat: %+v", delivery)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no queued owner frames after post-restart_town anti-give ITEM_GIVE, got %d", len(queued))
	}
	if queued := flushServerFrames(t, townFlow); len(queued) != 0 {
		t.Fatalf("expected no queued town peer frames after post-restart_town anti-give ITEM_GIVE, got %d", len(queued))
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town ITEM_GIVE: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after ITEM_GIVE floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town + ITEM_GIVE to persist recovered owner HP %d after ITEM_GIVE floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town ITEM_GIVE leaves inventory unchanged")
	assertExchangeAccountUnchanged(t, accounts, townLogin, townPeer, "post-restart_town ITEM_GIVE town peer unchanged")
}

func TestGameSessionFlowPostFloorItemRefineFailsClosedBeforeRejectFeedback(t *testing.T) {
	login := "post-floor-item-refine-owner"
	loginKey := uint32(0x19191a50)
	owner := peerVisibilityCharacter("DeadRefineOwner", 0x01030a50, 0x02040a50, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 802, Vnum: 11201, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}
	template := itemcatalog.Template{
		Vnum:             11201,
		Name:             "Dead Guard Practice Blade",
		Stackable:        false,
		MaxCount:         1,
		RefineRejectText: "This item cannot be refined yet.",
	}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, []itemcatalog.Template{template})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 2})))
	if err != nil {
		t.Fatalf("unexpected post-floor REFINE dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor REFINE to fail closed before template reject feedback, got %d frames", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor REFINE to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor REFINE")
}

func TestGameSessionFlowPostFloorStoragePacketsFailClosedWithoutMutation(t *testing.T) {
	login := "post-floor-store"
	loginKey := uint32(0x19191a60)
	owner := peerVisibilityCharacter("DeadStorageOwner", 0x01030a60, 0x02040a60, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 803, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, nil)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	requests := []struct {
		name string
		raw  []byte
	}{
		{name: "safebox checkin", raw: itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{SafeSlot: 7, Position: itemproto.InventoryPosition(5)})},
		{name: "safebox checkout", raw: itemproto.EncodeClientSafeboxCheckout(itemproto.ClientSafeboxCheckoutPacket{SafeSlot: 8, Position: itemproto.InventoryPosition(6)})},
		{name: "safebox item move", raw: itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{Source: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 7}, Destination: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 8}, Count: 1})},
		{name: "mall checkout", raw: itemproto.EncodeClientMallCheckout(itemproto.ClientMallCheckoutPacket{MallSlot: 4, Position: itemproto.InventoryPosition(9)})},
	}
	for _, request := range requests {
		t.Run(request.name, func(t *testing.T) {
			out, err := flow.HandleClientFrame(decodeSingleFrame(t, request.raw))
			if err != nil {
				t.Fatalf("unexpected post-floor %s dispatch error: %v", request.name, err)
			}
			if len(out) != 0 {
				t.Fatalf("expected post-floor %s to fail closed, got %d frames", request.name, len(out))
			}
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected post-floor %s to queue no frames, got %d", request.name, len(queued))
			}
			assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor "+request.name)
		})
	}
}

func TestGameSessionFlowPostFloorSafeboxCheckinFailsClosedBeforeAntiSafeboxFeedback(t *testing.T) {
	login := "post-floor-safe"
	loginKey := uint32(0x19191a70)
	owner := peerVisibilityCharacter("DeadSafeboxOwner", 0x01030a70, 0x02040a70, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 804, Vnum: 71127, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}
	template := itemcatalog.Template{
		Vnum:              71127,
		Name:              "Dead Guard Storage Charm",
		Stackable:         false,
		MaxCount:          1,
		AntiSafebox:       true,
		SafeboxRejectText: "This item cannot be placed in storage.",
	}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, []itemcatalog.Template{template})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{SafeSlot: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected post-floor SAFEBOX_CHECKIN dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor SAFEBOX_CHECKIN to fail closed before anti-safebox feedback, got %d frames", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor SAFEBOX_CHECKIN to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor SAFEBOX_CHECKIN")
}

func TestGameSessionFlowPostFloorItemMoveFailsClosed(t *testing.T) {
	login := "post-floor-item-move"
	loginKey := uint32(0x19191b50)
	owner := peerVisibilityCharacter("DeadItemMoveOwner", 0x01030b50, 0x02040b50, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 901, Vnum: 27001, Count: 3, Slot: 5}}
	templates := []itemcatalog.Template{{Vnum: 27001, Name: "Post Floor Move Potion", Stackable: true, MaxCount: 200}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor ITEM_MOVE dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor ITEM_MOVE to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor ITEM_MOVE to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor ITEM_MOVE")

	slashOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/inventory_move 5 6",
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor /inventory_move dispatch error: %v", err)
	}
	if len(slashOut) != 0 {
		t.Fatalf("expected post-floor /inventory_move to fail closed with no frames, got %d", len(slashOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor /inventory_move to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor /inventory_move")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor ITEM_MOVE: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor ITEM_MOVE, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart ITEM_MOVE: %v", err)
	}
	assertPostFloorItemMoveSuccessBurst(t, reuseOut, 5, 6, 27001, 3, "post-restart ITEM_MOVE")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart ITEM_MOVE: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after ITEM_MOVE floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.Inventory = []inventory.ItemInstance{{ID: 901, Vnum: 27001, Count: 3, Slot: 6}}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart ITEM_MOVE persists destination slot")
}

func TestGameSessionFlowPostFloorItemMoveFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-item-move-town"
	loginKey := uint32(0x19191b51)
	owner := peerVisibilityCharacter("DeadItemMoveTownOwner", 0x01030b51, 0x02040b51, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 902, Vnum: 27001, Count: 3, Slot: 5}}
	templates := []itemcatalog.Template{{Vnum: 27001, Name: "Post Floor Move Potion", Stackable: true, MaxCount: 200}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor town ITEM_MOVE dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town ITEM_MOVE to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town ITEM_MOVE to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town ITEM_MOVE")

	slashOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/inventory_move 5 6",
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor town /inventory_move dispatch error: %v", err)
	}
	if len(slashOut) != 0 {
		t.Fatalf("expected post-floor town /inventory_move to fail closed with no frames, got %d", len(slashOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town /inventory_move to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town /inventory_move")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor ITEM_MOVE: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor ITEM_MOVE, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor ITEM_MOVE /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after ITEM_MOVE floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after ITEM_MOVE floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after ITEM_MOVE floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town ITEM_MOVE: %v", err)
	}
	assertPostFloorItemMoveSuccessBurst(t, reuseOut, 5, 6, 27001, 3, "post-restart_town ITEM_MOVE")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town ITEM_MOVE: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after ITEM_MOVE floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after ITEM_MOVE floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Inventory = []inventory.ItemInstance{{ID: 902, Vnum: 27001, Count: 3, Slot: 6}}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town ITEM_MOVE persists destination slot")
}

func TestGameSessionFlowPostFloorEquipItemFailsClosed(t *testing.T) {
	login := "post-floor-equip-item"
	loginKey := uint32(0x19191b60)
	owner := peerVisibilityCharacter("DeadEquipOwner", 0x01030b60, 0x02040b60, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 12200, Count: 1, Slot: 8}}
	owner.Equipment = []inventory.ItemInstance{}
	templates := []itemcatalog.Template{{
		Vnum:      12200,
		Name:      "Post Floor Practice Blade",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: "weapon",
		EquipEffect: &itemcatalog.PointEffect{
			PointType:  bootstrapPlayerPointType,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: 10,
		},
	}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/equip_item 8 weapon",
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor /equip_item dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor /equip_item to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor /equip_item to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor /equip_item")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor /equip_item: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor /equip_item, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/equip_item 8 weapon",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart /equip_item: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	assertPostFloorEquipItemSuccessBurst(t, reuseOut, owner.VID, owner.MainPart, owner.HairPart, 12200, wantHP+10, "post-restart /equip_item")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart /equip_item: %v", err)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP+10 {
		t.Fatalf("expected /restart_here + /equip_item to persist recovered owner HP %d after equip floor, got %+v", wantHP+10, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP + 10
	want.Inventory = []inventory.ItemInstance{}
	want.Equipment = []inventory.ItemInstance{{ID: 1001, Vnum: 12200, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart /equip_item persists equipped weapon")
}

func TestGameSessionFlowPostFloorEquipItemFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-equip-item-town"
	loginKey := uint32(0x19191b61)
	owner := peerVisibilityCharacter("DeadEquipTownOwner", 0x01030b61, 0x02040b61, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1002, Vnum: 12200, Count: 1, Slot: 8}}
	owner.Equipment = []inventory.ItemInstance{}
	templates := []itemcatalog.Template{{
		Vnum:      12200,
		Name:      "Post Floor Practice Blade",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: "weapon",
		EquipEffect: &itemcatalog.PointEffect{
			PointType:  bootstrapPlayerPointType,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: 10,
		},
	}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/equip_item 8 weapon",
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor town /equip_item dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town /equip_item to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town /equip_item to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town /equip_item")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor /equip_item: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor /equip_item, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor /equip_item /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after equip floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after equip floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after equip floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/equip_item 8 weapon",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town /equip_item: %v", err)
	}
	assertPostFloorEquipItemSuccessBurst(t, reuseOut, owner.VID, owner.MainPart, owner.HairPart, 12200, wantHP+10, "post-restart_town /equip_item")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town /equip_item: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after equip floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP+10 {
		t.Fatalf("expected /restart_town + /equip_item to persist recovered owner HP %d after equip floor, got %+v", wantHP+10, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP + 10
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Inventory = []inventory.ItemInstance{}
	want.Equipment = []inventory.ItemInstance{{ID: 1002, Vnum: 12200, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town /equip_item persists equipped weapon")
}

func TestGameSessionFlowPostFloorUnequipItemFailsClosed(t *testing.T) {
	login := "post-floor-unequip-item"
	loginKey := uint32(0x19191b62)
	owner := peerVisibilityCharacter("DeadUnequipOwner", 0x01030b62, 0x02040b62, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{}
	owner.Equipment = []inventory.ItemInstance{{ID: 1003, Vnum: 12200, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	templates := []itemcatalog.Template{{
		Vnum:      12200,
		Name:      "Post Floor Practice Blade",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: "weapon",
		EquipEffect: &itemcatalog.PointEffect{
			PointType:  bootstrapPlayerPointType,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: 10,
		},
	}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/unequip_item weapon 4",
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor /unequip_item dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor /unequip_item to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor /unequip_item to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor /unequip_item")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor /unequip_item: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor /unequip_item, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/unequip_item weapon 4",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart /unequip_item: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	assertPostFloorUnequipItemSuccessBurst(t, reuseOut, owner.VID, owner.MainPart, owner.HairPart, 12200, 4, wantHP-10, "post-restart /unequip_item")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart /unequip_item: %v", err)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP-10 {
		t.Fatalf("expected /restart_here + /unequip_item to persist recovered owner HP %d after unequip floor, got %+v", wantHP-10, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP - 10
	want.Inventory = []inventory.ItemInstance{{ID: 1003, Vnum: 12200, Count: 1, Slot: 4}}
	want.Equipment = []inventory.ItemInstance{}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart /unequip_item persists carried weapon")
}

func TestGameSessionFlowPostFloorUnequipItemFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-unequip-item-town"
	loginKey := uint32(0x19191b63)
	owner := peerVisibilityCharacter("DeadUnequipTownOwner", 0x01030b63, 0x02040b63, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{}
	owner.Equipment = []inventory.ItemInstance{{ID: 1004, Vnum: 12200, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	templates := []itemcatalog.Template{{
		Vnum:      12200,
		Name:      "Post Floor Practice Blade",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: "weapon",
		EquipEffect: &itemcatalog.PointEffect{
			PointType:  bootstrapPlayerPointType,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: 10,
		},
	}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/unequip_item weapon 4",
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor town /unequip_item dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town /unequip_item to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town /unequip_item to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town /unequip_item")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor /unequip_item: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor /unequip_item, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor /unequip_item /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after unequip floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after unequip floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after unequip floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/unequip_item weapon 4",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town /unequip_item: %v", err)
	}
	assertPostFloorUnequipItemSuccessBurst(t, reuseOut, owner.VID, owner.MainPart, owner.HairPart, 12200, 4, wantHP-10, "post-restart_town /unequip_item")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town /unequip_item: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after unequip floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP-10 {
		t.Fatalf("expected /restart_town + /unequip_item to persist recovered owner HP %d after unequip floor, got %+v", wantHP-10, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP - 10
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Inventory = []inventory.ItemInstance{{ID: 1004, Vnum: 12200, Count: 1, Slot: 4}}
	want.Equipment = []inventory.ItemInstance{}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town /unequip_item persists carried weapon")
}

func TestGameSessionFlowPostFloorItemDropFailsClosed(t *testing.T) {
	login := "post-floor-item-drop"
	loginKey := uint32(0x19191b64)
	owner := peerVisibilityCharacter("DeadItemDropOwner", 0x01030b64, 0x02040b64, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1101, Vnum: 27001, Count: 3, Slot: 5}}
	templates := []itemcatalog.Template{{Vnum: 27001, Name: "Post Floor Drop Potion", Stackable: true, MaxCount: 200}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor ITEM_DROP dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor ITEM_DROP to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor ITEM_DROP to queue no frames, got %d", len(queued))
	}
	if ground := runtime.GroundItems(); len(ground) != 0 {
		t.Fatalf("expected post-floor ITEM_DROP to register no ground items, got %#v", ground)
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor ITEM_DROP")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor ITEM_DROP: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor ITEM_DROP, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart ITEM_DROP: %v", err)
	}
	assertPostFloorItemDropSuccessBurst(t, reuseOut, owner.Name, owner.X, owner.Y, owner.Z, 5, 27001, "post-restart ITEM_DROP")
	if ground := runtime.GroundItems(); len(ground) != 1 || ground[0].Vnum != 27001 {
		t.Fatalf("expected one post-restart ground item after ITEM_DROP recovery, got %#v", ground)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart ITEM_DROP: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after ITEM_DROP floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.Inventory = []inventory.ItemInstance{}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart ITEM_DROP clears carried inventory")
}

func TestGameSessionFlowPostFloorItemDropFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-item-drop-town"
	loginKey := uint32(0x19191b65)
	owner := peerVisibilityCharacter("DeadItemDropTownOwner", 0x01030b65, 0x02040b65, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1102, Vnum: 27001, Count: 3, Slot: 5}}
	templates := []itemcatalog.Template{{Vnum: 27001, Name: "Post Floor Drop Potion", Stackable: true, MaxCount: 200}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor town ITEM_DROP dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town ITEM_DROP to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town ITEM_DROP to queue no frames, got %d", len(queued))
	}
	if ground := runtime.GroundItems(); len(ground) != 0 {
		t.Fatalf("expected post-floor town ITEM_DROP to register no ground items, got %#v", ground)
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town ITEM_DROP")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor ITEM_DROP: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor ITEM_DROP, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor ITEM_DROP /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after ITEM_DROP floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after ITEM_DROP floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after ITEM_DROP floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town ITEM_DROP: %v", err)
	}
	assertPostFloorItemDropSuccessBurst(t, reuseOut, owner.Name, 52070, 166600, owner.Z, 5, 27001, "post-restart_town ITEM_DROP")
	if ground := runtime.GroundItems(); len(ground) != 1 || ground[0].Vnum != 27001 {
		t.Fatalf("expected one post-restart_town ground item after ITEM_DROP recovery, got %#v", ground)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town ITEM_DROP: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after ITEM_DROP floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town + ITEM_DROP to persist recovered owner HP %d after ITEM_DROP floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Inventory = []inventory.ItemInstance{}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town ITEM_DROP clears carried inventory")
}

func assertPostFloorItemDropSuccessBurst(t *testing.T, frames [][]byte, ownerName string, x, y, z int32, sourceSlot uint16, vnum uint32, context string) {
	t.Helper()
	if len(frames) != 3 {
		t.Fatalf("expected %s to emit ITEM_DEL + GROUND_ADD + OWNERSHIP, got %d frames", context, len(frames))
	}
	itemDel, err := itemproto.DecodeDel(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode %s item delete: %v", context, err)
	}
	if itemDel.Position != itemproto.InventoryPosition(sourceSlot) {
		t.Fatalf("unexpected %s item delete position: %+v", context, itemDel.Position)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s ground add: %v", context, err)
	}
	if ground.VID == 0 || ground.Vnum != vnum || ground.X != x || ground.Y != y || ground.Z != z {
		t.Fatalf("unexpected %s ground add: %+v", context, ground)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, frames[2]))
	if err != nil {
		t.Fatalf("decode %s ownership: %v", context, err)
	}
	if ownership != (itemproto.OwnershipPacket{VID: ground.VID, OwnerName: ownerName}) {
		t.Fatalf("unexpected %s ownership: got %+v want vid %d owner %q", context, ownership, ground.VID, ownerName)
	}
}

func TestGameSessionFlowPostFloorItemDrop2FailsClosed(t *testing.T) {
	login := "post-floor-item-drop2"
	loginKey := uint32(0x19191b68)
	owner := peerVisibilityCharacter("DeadItemDrop2Owner", 0x01030b68, 0x02040b68, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1103, Vnum: 27001, Count: 5, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	templates := []itemcatalog.Template{{Vnum: 27001, Name: "Post Floor Drop2 Potion", Stackable: true, MaxCount: 200}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{
		Position: itemproto.InventoryPosition(5),
		Count:    2,
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor ITEM_DROP2 dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor ITEM_DROP2 to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor ITEM_DROP2 to queue no frames, got %d", len(queued))
	}
	if ground := runtime.GroundItems(); len(ground) != 0 {
		t.Fatalf("expected post-floor ITEM_DROP2 to register no ground items, got %#v", ground)
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor ITEM_DROP2")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor ITEM_DROP2: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor ITEM_DROP2, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{
		Position: itemproto.InventoryPosition(5),
		Count:    2,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart ITEM_DROP2: %v", err)
	}
	assertPostFloorItemDrop2SuccessBurst(t, reuseOut, owner.Name, owner.X, owner.Y, owner.Z, 5, 27001, 3, "post-restart ITEM_DROP2")
	if ground := runtime.GroundItems(); len(ground) != 1 || ground[0].Vnum != 27001 || ground[0].Count != 2 {
		t.Fatalf("expected one post-restart partial ground item after ITEM_DROP2 recovery, got %#v", ground)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart ITEM_DROP2: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after ITEM_DROP2 floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.Inventory = []inventory.ItemInstance{{ID: 1103, Vnum: 27001, Count: 3, Slot: 5}}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart ITEM_DROP2 preserves remaining stack and quickslots")
}

func TestGameSessionFlowPostFloorItemDrop2FailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-item-drop2-town"
	loginKey := uint32(0x19191b69)
	owner := peerVisibilityCharacter("DeadItemDrop2TownOwner", 0x01030b69, 0x02040b69, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1104, Vnum: 27001, Count: 5, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	templates := []itemcatalog.Template{{Vnum: 27001, Name: "Post Floor Drop2 Potion", Stackable: true, MaxCount: 200}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{
		Position: itemproto.InventoryPosition(5),
		Count:    2,
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor town ITEM_DROP2 dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town ITEM_DROP2 to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town ITEM_DROP2 to queue no frames, got %d", len(queued))
	}
	if ground := runtime.GroundItems(); len(ground) != 0 {
		t.Fatalf("expected post-floor town ITEM_DROP2 to register no ground items, got %#v", ground)
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town ITEM_DROP2")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor ITEM_DROP2: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor ITEM_DROP2, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor ITEM_DROP2 /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after ITEM_DROP2 floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after ITEM_DROP2 floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after ITEM_DROP2 floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{
		Position: itemproto.InventoryPosition(5),
		Count:    2,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town ITEM_DROP2: %v", err)
	}
	assertPostFloorItemDrop2SuccessBurst(t, reuseOut, owner.Name, 52070, 166600, owner.Z, 5, 27001, 3, "post-restart_town ITEM_DROP2")
	if ground := runtime.GroundItems(); len(ground) != 1 || ground[0].Vnum != 27001 || ground[0].Count != 2 {
		t.Fatalf("expected one post-restart_town partial ground item after ITEM_DROP2 recovery, got %#v", ground)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town ITEM_DROP2: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after ITEM_DROP2 floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town + ITEM_DROP2 to persist recovered owner HP %d after ITEM_DROP2 floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Inventory = []inventory.ItemInstance{{ID: 1104, Vnum: 27001, Count: 3, Slot: 5}}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town ITEM_DROP2 preserves remaining stack and quickslots")
}

func assertPostFloorItemDrop2SuccessBurst(t *testing.T, frames [][]byte, ownerName string, x, y, z int32, sourceSlot uint16, vnum uint32, remainingCount uint8, context string) {
	t.Helper()
	if len(frames) != 3 {
		t.Fatalf("expected %s to emit ITEM_UPDATE + GROUND_ADD + OWNERSHIP, got %d frames", context, len(frames))
	}
	update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode %s item update: %v", context, err)
	}
	if update.Position != itemproto.InventoryPosition(sourceSlot) || update.Count != remainingCount {
		t.Fatalf("unexpected %s item update: %+v want position=%+v count=%d", context, update, itemproto.InventoryPosition(sourceSlot), remainingCount)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s ground add: %v", context, err)
	}
	if ground.VID == 0 || ground.Vnum != vnum || ground.X != x || ground.Y != y || ground.Z != z {
		t.Fatalf("unexpected %s ground add: %+v", context, ground)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, frames[2]))
	if err != nil {
		t.Fatalf("decode %s ownership: %v", context, err)
	}
	if ownership != (itemproto.OwnershipPacket{VID: ground.VID, OwnerName: ownerName}) {
		t.Fatalf("unexpected %s ownership: got %+v want vid %d owner %q", context, ownership, ground.VID, ownerName)
	}
}

func TestGameSessionFlowPostFloorGoldDropFailsClosed(t *testing.T) {
	login := "post-floor-gold-drop"
	loginKey := uint32(0x19191b66)
	owner := peerVisibilityCharacter("DeadGoldDropOwner", 0x01030b66, 0x02040b66, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, nil)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Elk: 1200})))
	if err != nil {
		t.Fatalf("unexpected post-floor gold ITEM_DROP dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor gold ITEM_DROP to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor gold ITEM_DROP to queue no frames, got %d", len(queued))
	}
	if ground := runtime.GroundItems(); len(ground) != 0 {
		t.Fatalf("expected post-floor gold ITEM_DROP to register no ground items, got %#v", ground)
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor gold ITEM_DROP")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor gold ITEM_DROP: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor gold ITEM_DROP, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Elk: 1200})))
	if err != nil {
		t.Fatalf("unexpected post-restart gold ITEM_DROP: %v", err)
	}
	assertPostFloorGoldDropSuccessBurst(t, reuseOut, owner.VID, owner.Name, owner.X, owner.Y, owner.Z, 1200, 3800, "post-restart gold ITEM_DROP")
	if ground := runtime.GroundItems(); len(ground) != 1 || ground[0].Vnum != 1 || ground[0].GoldAmount != 1200 {
		t.Fatalf("expected one post-restart gold ground marker after ITEM_DROP recovery, got %#v", ground)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart gold ITEM_DROP: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after gold ITEM_DROP floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.Gold = 3800
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart gold ITEM_DROP debits carried gold")
}

func TestGameSessionFlowPostFloorGoldDropFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-gold-drop-town"
	loginKey := uint32(0x19191b67)
	owner := peerVisibilityCharacter("DeadGoldDropTownOwner", 0x01030b67, 0x02040b67, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, nil)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Elk: 1200})))
	if err != nil {
		t.Fatalf("unexpected post-floor town gold ITEM_DROP dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town gold ITEM_DROP to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town gold ITEM_DROP to queue no frames, got %d", len(queued))
	}
	if ground := runtime.GroundItems(); len(ground) != 0 {
		t.Fatalf("expected post-floor town gold ITEM_DROP to register no ground items, got %#v", ground)
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town gold ITEM_DROP")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor gold ITEM_DROP: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor gold ITEM_DROP, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor gold ITEM_DROP /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after gold ITEM_DROP floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after gold ITEM_DROP floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after gold ITEM_DROP floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Elk: 1200})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town gold ITEM_DROP: %v", err)
	}
	assertPostFloorGoldDropSuccessBurst(t, reuseOut, owner.VID, owner.Name, 52070, 166600, owner.Z, 1200, 3800, "post-restart_town gold ITEM_DROP")
	if ground := runtime.GroundItems(); len(ground) != 1 || ground[0].Vnum != 1 || ground[0].GoldAmount != 1200 {
		t.Fatalf("expected one post-restart_town gold ground marker after ITEM_DROP recovery, got %#v", ground)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town gold ITEM_DROP: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after gold ITEM_DROP floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town + gold ITEM_DROP to persist recovered owner HP %d after gold ITEM_DROP floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Gold = 3800
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town gold ITEM_DROP debits carried gold")
}

func TestGameSessionFlowPostFloorQuickslotAddFailsClosed(t *testing.T) {
	login := "post-floor-qs-add"
	loginKey := uint32(0x19191b68)
	owner := peerVisibilityCharacter("DeadQuickslotAddOwner", 0x01030b68, 0x02040b68, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1201, Vnum: 27001, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeSkill, Slot: 5}}
	templates := []itemcatalog.Template{{Vnum: 27001, Name: "Post Floor Quickslot Potion", Stackable: true, MaxCount: 200}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	addPacket := quickslotproto.EncodeClientAdd(quickslotproto.ClientAddPacket{
		Position: 4,
		Slot:     quickslotproto.Slot{Type: quickslotproto.TypeItem, Position: 5},
	})
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, addPacket))
	if err != nil {
		t.Fatalf("unexpected post-floor QUICKSLOT_ADD dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor QUICKSLOT_ADD to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor QUICKSLOT_ADD to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor QUICKSLOT_ADD")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor QUICKSLOT_ADD: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor QUICKSLOT_ADD, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, addPacket))
	if err != nil {
		t.Fatalf("unexpected post-restart QUICKSLOT_ADD: %v", err)
	}
	wantFrames := [][]byte{
		quickslotproto.EncodeAdd(quickslotproto.AddPacket{Position: 4, Slot: quickslotproto.Slot{Type: quickslotproto.TypeItem, Position: 5}}),
	}
	if !reflect.DeepEqual(reuseOut, wantFrames) {
		t.Fatalf("unexpected post-restart QUICKSLOT_ADD frames:\n got %#v\nwant %#v", reuseOut, wantFrames)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart QUICKSLOT_ADD: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after QUICKSLOT_ADD floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 5},
	}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart QUICKSLOT_ADD persists new binding")
}

func TestGameSessionFlowPostFloorQuickslotAddFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-qs-add-town"
	loginKey := uint32(0x19191b69)
	owner := peerVisibilityCharacter("DeadQuickslotAddTownOwner", 0x01030b69, 0x02040b69, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1202, Vnum: 27001, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeSkill, Slot: 5}}
	templates := []itemcatalog.Template{{Vnum: 27001, Name: "Post Floor Quickslot Potion", Stackable: true, MaxCount: 200}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	addPacket := quickslotproto.EncodeClientAdd(quickslotproto.ClientAddPacket{
		Position: 4,
		Slot:     quickslotproto.Slot{Type: quickslotproto.TypeItem, Position: 5},
	})
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, addPacket))
	if err != nil {
		t.Fatalf("unexpected post-floor town QUICKSLOT_ADD dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town QUICKSLOT_ADD to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town QUICKSLOT_ADD to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town QUICKSLOT_ADD")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor QUICKSLOT_ADD: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor QUICKSLOT_ADD, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor QUICKSLOT_ADD /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after QUICKSLOT_ADD floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after QUICKSLOT_ADD floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after QUICKSLOT_ADD floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, addPacket))
	if err != nil {
		t.Fatalf("unexpected post-restart_town QUICKSLOT_ADD: %v", err)
	}
	wantFrames := [][]byte{
		quickslotproto.EncodeAdd(quickslotproto.AddPacket{Position: 4, Slot: quickslotproto.Slot{Type: quickslotproto.TypeItem, Position: 5}}),
	}
	if !reflect.DeepEqual(reuseOut, wantFrames) {
		t.Fatalf("unexpected post-restart_town QUICKSLOT_ADD frames:\n got %#v\nwant %#v", reuseOut, wantFrames)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town QUICKSLOT_ADD: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after QUICKSLOT_ADD floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town + QUICKSLOT_ADD to persist recovered owner HP %d after QUICKSLOT_ADD floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 5},
	}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town QUICKSLOT_ADD persists new binding")
}

func TestGameSessionFlowPostFloorQuickslotDelFailsClosed(t *testing.T) {
	login := "post-floor-qs-del"
	loginKey := uint32(0x19191b72)
	owner := peerVisibilityCharacter("DeadQuickslotDelOwner", 0x01030b72, 0x02040b72, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1203, Vnum: 27001, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	templates := []itemcatalog.Template{{Vnum: 27001, Name: "Post Floor Quickslot Potion", Stackable: true, MaxCount: 200}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	delPacket := quickslotproto.EncodeClientDel(quickslotproto.ClientDelPacket{Position: 2})
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, delPacket))
	if err != nil {
		t.Fatalf("unexpected post-floor QUICKSLOT_DEL dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor QUICKSLOT_DEL to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor QUICKSLOT_DEL to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor QUICKSLOT_DEL")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor QUICKSLOT_DEL: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor QUICKSLOT_DEL, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, delPacket))
	if err != nil {
		t.Fatalf("unexpected post-restart QUICKSLOT_DEL: %v", err)
	}
	wantFrames := [][]byte{
		quickslotproto.EncodeDel(quickslotproto.DelPacket{Position: 2}),
	}
	if !reflect.DeepEqual(reuseOut, wantFrames) {
		t.Fatalf("unexpected post-restart QUICKSLOT_DEL frames:\n got %#v\nwant %#v", reuseOut, wantFrames)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart QUICKSLOT_DEL: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after QUICKSLOT_DEL floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.Quickslots = []loginticket.Quickslot{
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart QUICKSLOT_DEL clears item binding")
}

func TestGameSessionFlowPostFloorQuickslotDelFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-qs-del-town"
	loginKey := uint32(0x19191b73)
	owner := peerVisibilityCharacter("DeadQuickslotDelTownOwner", 0x01030b73, 0x02040b73, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1204, Vnum: 27001, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	templates := []itemcatalog.Template{{Vnum: 27001, Name: "Post Floor Quickslot Potion", Stackable: true, MaxCount: 200}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	delPacket := quickslotproto.EncodeClientDel(quickslotproto.ClientDelPacket{Position: 2})
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, delPacket))
	if err != nil {
		t.Fatalf("unexpected post-floor town QUICKSLOT_DEL dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town QUICKSLOT_DEL to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town QUICKSLOT_DEL to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town QUICKSLOT_DEL")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor QUICKSLOT_DEL: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor QUICKSLOT_DEL, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor QUICKSLOT_DEL /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after QUICKSLOT_DEL floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after QUICKSLOT_DEL floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after QUICKSLOT_DEL floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, delPacket))
	if err != nil {
		t.Fatalf("unexpected post-restart_town QUICKSLOT_DEL: %v", err)
	}
	wantFrames := [][]byte{
		quickslotproto.EncodeDel(quickslotproto.DelPacket{Position: 2}),
	}
	if !reflect.DeepEqual(reuseOut, wantFrames) {
		t.Fatalf("unexpected post-restart_town QUICKSLOT_DEL frames:\n got %#v\nwant %#v", reuseOut, wantFrames)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town QUICKSLOT_DEL: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after QUICKSLOT_DEL floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town + QUICKSLOT_DEL to persist recovered owner HP %d after QUICKSLOT_DEL floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Quickslots = []loginticket.Quickslot{
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town QUICKSLOT_DEL clears item binding")
}

func TestGameSessionFlowPostFloorQuickslotSwapFailsClosed(t *testing.T) {
	login := "post-floor-qs-swap"
	loginKey := uint32(0x19191b74)
	owner := peerVisibilityCharacter("DeadQuickslotSwapOwner", 0x01030b74, 0x02040b74, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1205, Vnum: 27001, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	templates := []itemcatalog.Template{{Vnum: 27001, Name: "Post Floor Quickslot Potion", Stackable: true, MaxCount: 200}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	swapPacket := quickslotproto.EncodeClientSwap(quickslotproto.ClientSwapPacket{Position: 2, TargetPosition: 3})
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, swapPacket))
	if err != nil {
		t.Fatalf("unexpected post-floor QUICKSLOT_SWAP dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor QUICKSLOT_SWAP to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor QUICKSLOT_SWAP to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor QUICKSLOT_SWAP")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor QUICKSLOT_SWAP: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor QUICKSLOT_SWAP, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, swapPacket))
	if err != nil {
		t.Fatalf("unexpected post-restart QUICKSLOT_SWAP: %v", err)
	}
	wantFrames := [][]byte{
		quickslotproto.EncodeSwap(quickslotproto.SwapPacket{Position: 2, TargetPosition: 3}),
	}
	if !reflect.DeepEqual(reuseOut, wantFrames) {
		t.Fatalf("unexpected post-restart QUICKSLOT_SWAP frames:\n got %#v\nwant %#v", reuseOut, wantFrames)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart QUICKSLOT_SWAP: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after QUICKSLOT_SWAP floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 5},
	}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart QUICKSLOT_SWAP exchanges bindings")
}

func TestGameSessionFlowPostFloorQuickslotSwapFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-qs-swap-town"
	loginKey := uint32(0x19191b75)
	owner := peerVisibilityCharacter("DeadQuickslotSwapTownOwner", 0x01030b75, 0x02040b75, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1206, Vnum: 27001, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	templates := []itemcatalog.Template{{Vnum: 27001, Name: "Post Floor Quickslot Potion", Stackable: true, MaxCount: 200}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	swapPacket := quickslotproto.EncodeClientSwap(quickslotproto.ClientSwapPacket{Position: 2, TargetPosition: 3})
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, swapPacket))
	if err != nil {
		t.Fatalf("unexpected post-floor town QUICKSLOT_SWAP dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town QUICKSLOT_SWAP to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town QUICKSLOT_SWAP to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town QUICKSLOT_SWAP")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor QUICKSLOT_SWAP: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor QUICKSLOT_SWAP, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor QUICKSLOT_SWAP /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after QUICKSLOT_SWAP floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after QUICKSLOT_SWAP floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after QUICKSLOT_SWAP floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, swapPacket))
	if err != nil {
		t.Fatalf("unexpected post-restart_town QUICKSLOT_SWAP: %v", err)
	}
	wantFrames := [][]byte{
		quickslotproto.EncodeSwap(quickslotproto.SwapPacket{Position: 2, TargetPosition: 3}),
	}
	if !reflect.DeepEqual(reuseOut, wantFrames) {
		t.Fatalf("unexpected post-restart_town QUICKSLOT_SWAP frames:\n got %#v\nwant %#v", reuseOut, wantFrames)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town QUICKSLOT_SWAP: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after QUICKSLOT_SWAP floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town + QUICKSLOT_SWAP to persist recovered owner HP %d after QUICKSLOT_SWAP floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 5},
	}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town QUICKSLOT_SWAP exchanges bindings")
}

func TestGameSessionFlowPostFloorItemUseFailsClosed(t *testing.T) {
	login := "post-floor-item-use"
	loginKey := uint32(0x19191b70)
	owner := peerVisibilityCharacter("DeadItemUseOwner", 0x01030b70, 0x02040b70, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1301, Vnum: 27006, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	templates := []itemcatalog.Template{{
		Vnum:      27006,
		Name:      "Post Floor Cursed Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{
			PointType:  bootstrapPlayerPointType,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: -25,
			Message:    "consume:27006:-25",
		},
	}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	usePacket := itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(5)})
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, usePacket))
	if err != nil {
		t.Fatalf("unexpected post-floor ITEM_USE dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor ITEM_USE to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor ITEM_USE to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor ITEM_USE")

	slashOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/use_item 5",
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor /use_item dispatch error: %v", err)
	}
	if len(slashOut) != 0 {
		t.Fatalf("expected post-floor /use_item to fail closed with no frames, got %d", len(slashOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor /use_item to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor /use_item")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor ITEM_USE: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor ITEM_USE, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, usePacket))
	if err != nil {
		t.Fatalf("unexpected post-restart ITEM_USE: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	assertPostFloorItemUseSuccessBurst(t, reuseOut, owner.VID, itemproto.InventoryPosition(5), 27006, -25, wantHP-25, 1, "consume:27006:-25", "post-restart ITEM_USE")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart ITEM_USE: %v", err)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP-25 {
		t.Fatalf("expected post-restart ITEM_USE to persist HP %d, got %+v", wantHP-25, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP - 25
	want.Inventory = []inventory.ItemInstance{{ID: 1301, Vnum: 27006, Count: 1, Slot: 5}}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart ITEM_USE persists stack decrement")
}

func TestGameSessionFlowPostFloorItemUseFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-item-use-town"
	loginKey := uint32(0x19191b71)
	owner := peerVisibilityCharacter("DeadItemUseTownOwner", 0x01030b71, 0x02040b71, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1302, Vnum: 27006, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	templates := []itemcatalog.Template{{
		Vnum:      27006,
		Name:      "Post Floor Cursed Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{
			PointType:  bootstrapPlayerPointType,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: -25,
			Message:    "consume:27006:-25",
		},
	}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	usePacket := itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(5)})
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, usePacket))
	if err != nil {
		t.Fatalf("unexpected post-floor town ITEM_USE dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town ITEM_USE to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town ITEM_USE to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town ITEM_USE")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor ITEM_USE: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor ITEM_USE, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor ITEM_USE /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after ITEM_USE floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after ITEM_USE floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after ITEM_USE floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, usePacket))
	if err != nil {
		t.Fatalf("unexpected post-restart_town ITEM_USE: %v", err)
	}
	assertPostFloorItemUseSuccessBurst(t, reuseOut, owner.VID, itemproto.InventoryPosition(5), 27006, -25, wantHP-25, 1, "consume:27006:-25", "post-restart_town ITEM_USE")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town ITEM_USE: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after ITEM_USE floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP-25 {
		t.Fatalf("expected /restart_town + ITEM_USE to persist HP %d after ITEM_USE floor, got %+v", wantHP-25, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP - 25
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Inventory = []inventory.ItemInstance{{ID: 1302, Vnum: 27006, Count: 1, Slot: 5}}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town ITEM_USE persists stack decrement")
}

func TestGameSessionFlowPostFloorItemUseToItemFailsClosed(t *testing.T) {
	login := "post-floor-use-to-item"
	loginKey := uint32(0x19191b72)
	owner := peerVisibilityCharacter("DeadUseToItemOwner", 0x01030b72, 0x02040b72, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1401, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 1402, Vnum: 27001, Count: 4, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
	}
	templates := []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Post Floor Merge Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{
			PointType:  bootstrapPlayerPointType,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: 50,
			Message:    "consume:27001:+50",
		},
	}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	useToItemPacket := itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
		Source: itemproto.InventoryPosition(5),
		Target: itemproto.InventoryPosition(6),
	})
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, useToItemPacket))
	if err != nil {
		t.Fatalf("unexpected post-floor ITEM_USE_TO_ITEM dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor ITEM_USE_TO_ITEM to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor ITEM_USE_TO_ITEM to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor ITEM_USE_TO_ITEM")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor ITEM_USE_TO_ITEM: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor ITEM_USE_TO_ITEM, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, useToItemPacket))
	if err != nil {
		t.Fatalf("unexpected post-restart ITEM_USE_TO_ITEM: %v", err)
	}
	assertPostFloorItemUseToItemFullMergeBurst(t, reuseOut, itemproto.InventoryPosition(5), itemproto.InventoryPosition(6), 7, 2, "post-restart ITEM_USE_TO_ITEM")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart ITEM_USE_TO_ITEM: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected post-restart ITEM_USE_TO_ITEM to leave recovered HP %d unchanged, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.Inventory = []inventory.ItemInstance{{ID: 1402, Vnum: 27001, Count: 7, Slot: 6}}
	want.Quickslots = []loginticket.Quickslot{
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
	}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart ITEM_USE_TO_ITEM persists full merge")
}

func TestGameSessionFlowPostFloorItemUseToItemFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-use-to-item-town"
	loginKey := uint32(0x19191b73)
	owner := peerVisibilityCharacter("DeadUseToItemTownOwner", 0x01030b73, 0x02040b73, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1403, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 1404, Vnum: 27001, Count: 4, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
	}
	templates := []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Post Floor Merge Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{
			PointType:  bootstrapPlayerPointType,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: 50,
			Message:    "consume:27001:+50",
		},
	}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	useToItemPacket := itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
		Source: itemproto.InventoryPosition(5),
		Target: itemproto.InventoryPosition(6),
	})
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, useToItemPacket))
	if err != nil {
		t.Fatalf("unexpected post-floor town ITEM_USE_TO_ITEM dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town ITEM_USE_TO_ITEM to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town ITEM_USE_TO_ITEM to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town ITEM_USE_TO_ITEM")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor ITEM_USE_TO_ITEM: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor ITEM_USE_TO_ITEM, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor ITEM_USE_TO_ITEM /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after ITEM_USE_TO_ITEM floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after ITEM_USE_TO_ITEM floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after ITEM_USE_TO_ITEM floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, useToItemPacket))
	if err != nil {
		t.Fatalf("unexpected post-restart_town ITEM_USE_TO_ITEM: %v", err)
	}
	assertPostFloorItemUseToItemFullMergeBurst(t, reuseOut, itemproto.InventoryPosition(5), itemproto.InventoryPosition(6), 7, 2, "post-restart_town ITEM_USE_TO_ITEM")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town ITEM_USE_TO_ITEM: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after ITEM_USE_TO_ITEM floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town + ITEM_USE_TO_ITEM to leave recovered HP %d unchanged, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Inventory = []inventory.ItemInstance{{ID: 1404, Vnum: 27001, Count: 7, Slot: 6}}
	want.Quickslots = []loginticket.Quickslot{
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
	}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town ITEM_USE_TO_ITEM persists full merge")
}

func assertPostFloorItemUseToItemFullMergeBurst(t *testing.T, frames [][]byte, source, target itemproto.Position, mergedCount uint8, quickslotPosition uint8, context string) {
	t.Helper()
	if len(frames) != 3 {
		t.Fatalf("expected %s to emit ITEM_DEL + ITEM_UPDATE + QUICKSLOT_DEL, got %d frames", context, len(frames))
	}
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode %s source del: %v", context, err)
	}
	if del.Position != source {
		t.Fatalf("unexpected %s source del: %+v", context, del)
	}
	update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s target update: %v", context, err)
	}
	if update.Position != target || update.Count != mergedCount {
		t.Fatalf("unexpected %s target update: %+v", context, update)
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, frames[2]))
	if err != nil {
		t.Fatalf("decode %s source quickslot del: %v", context, err)
	}
	if quickslotDel.Position != quickslotPosition {
		t.Fatalf("expected %s to delete only source item quickslot position %d, got %+v", context, quickslotPosition, quickslotDel)
	}
}

func TestGameSessionFlowPostFloorItemUseToItemPartialFailsClosed(t *testing.T) {
	login := "post-floor-use-to-item-partial"
	loginKey := uint32(0x19191b82)
	owner := peerVisibilityCharacter("DeadUseToItemPartialOwner", 0x01030b82, 0x02040b82, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1411, Vnum: 27001, Count: 7, Slot: 5},
		{ID: 1412, Vnum: 27001, Count: 8, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 5, Type: quickslotproto.TypeSkill, Slot: 6},
	}
	templates := []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Post Floor Stack Potion",
		Stackable: true,
		MaxCount:  10,
	}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	useToItemPacket := itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
		Source: itemproto.InventoryPosition(5),
		Target: itemproto.InventoryPosition(6),
	})
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, useToItemPacket))
	if err != nil {
		t.Fatalf("unexpected post-floor ITEM_USE_TO_ITEM dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor ITEM_USE_TO_ITEM to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor ITEM_USE_TO_ITEM to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor ITEM_USE_TO_ITEM")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor ITEM_USE_TO_ITEM: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor ITEM_USE_TO_ITEM, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, useToItemPacket))
	if err != nil {
		t.Fatalf("unexpected post-restart ITEM_USE_TO_ITEM: %v", err)
	}
	assertPostFloorItemUseToItemPartialSuccessBurst(t, reuseOut, "post-restart ITEM_USE_TO_ITEM")
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.Inventory = []inventory.ItemInstance{
		{ID: 1411, Vnum: 27001, Count: 5, Slot: 5},
		{ID: 1412, Vnum: 27001, Count: 10, Slot: 6},
	}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart ITEM_USE_TO_ITEM persists partial merge")
}

func TestGameSessionFlowPostFloorItemUseToItemPartialFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-use-to-item-partial-town"
	loginKey := uint32(0x19191b83)
	owner := peerVisibilityCharacter("DeadUseToItemPartialTownOwner", 0x01030b83, 0x02040b83, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1413, Vnum: 27001, Count: 7, Slot: 5},
		{ID: 1414, Vnum: 27001, Count: 8, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 5, Type: quickslotproto.TypeSkill, Slot: 6},
	}
	templates := []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Post Floor Stack Potion",
		Stackable: true,
		MaxCount:  10,
	}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	useToItemPacket := itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
		Source: itemproto.InventoryPosition(5),
		Target: itemproto.InventoryPosition(6),
	})
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, useToItemPacket))
	if err != nil {
		t.Fatalf("unexpected post-floor town ITEM_USE_TO_ITEM dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town ITEM_USE_TO_ITEM to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town ITEM_USE_TO_ITEM to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town ITEM_USE_TO_ITEM")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor ITEM_USE_TO_ITEM: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor ITEM_USE_TO_ITEM, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor ITEM_USE_TO_ITEM /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after ITEM_USE_TO_ITEM floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after ITEM_USE_TO_ITEM floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after ITEM_USE_TO_ITEM floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, useToItemPacket))
	if err != nil {
		t.Fatalf("unexpected post-restart_town ITEM_USE_TO_ITEM: %v", err)
	}
	assertPostFloorItemUseToItemPartialSuccessBurst(t, reuseOut, "post-restart_town ITEM_USE_TO_ITEM")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town ITEM_USE_TO_ITEM: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after ITEM_USE_TO_ITEM floor, got %+v", account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Inventory = []inventory.ItemInstance{
		{ID: 1413, Vnum: 27001, Count: 5, Slot: 5},
		{ID: 1414, Vnum: 27001, Count: 10, Slot: 6},
	}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town ITEM_USE_TO_ITEM persists partial merge")
}

func assertPostFloorItemUseToItemPartialSuccessBurst(t *testing.T, frames [][]byte, context string) {
	t.Helper()
	if len(frames) != 2 {
		t.Fatalf("expected %s to emit source and target ITEM_UPDATE only, got %d frames", context, len(frames))
	}
	sourceUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode %s source update: %v", context, err)
	}
	if sourceUpdate.Position != itemproto.InventoryPosition(5) || sourceUpdate.Count != 5 {
		t.Fatalf("unexpected %s source update: %+v", context, sourceUpdate)
	}
	targetUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s target update: %v", context, err)
	}
	if targetUpdate.Position != itemproto.InventoryPosition(6) || targetUpdate.Count != 10 {
		t.Fatalf("unexpected %s target update: %+v", context, targetUpdate)
	}
}

func assertPostFloorItemUseSuccessBurst(t *testing.T, frames [][]byte, ownerVID uint32, position itemproto.Position, vnum uint32, pointDelta int32, pointValue int32, remainingCount uint8, message string, context string) {
	t.Helper()
	if len(frames) != 4 {
		t.Fatalf("expected %s to emit use echo + POINT_CHANGE + ITEM_UPDATE + INFO chat, got %d frames", context, len(frames))
	}
	useEcho, err := itemproto.DecodeUse(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode %s use echo: %v", context, err)
	}
	if useEcho.Position != position || useEcho.CharacterVID != ownerVID || useEcho.VictimVID != ownerVID || useEcho.Vnum != vnum {
		t.Fatalf("unexpected %s use echo: %+v", context, useEcho)
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s point-change: %v", context, err)
	}
	if pointChange.VID != ownerVID || pointChange.Type != bootstrapPlayerPointType || pointChange.Amount != pointDelta || pointChange.Value != pointValue {
		t.Fatalf("unexpected %s point-change: %+v", context, pointChange)
	}
	updated, err := itemproto.DecodeUpdate(decodeSingleFrame(t, frames[2]))
	if err != nil {
		t.Fatalf("decode %s item update: %v", context, err)
	}
	if updated.Position != position || updated.Count != remainingCount {
		t.Fatalf("unexpected %s item update: %+v", context, updated)
	}
	info, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, frames[3]))
	if err != nil {
		t.Fatalf("decode %s info chat: %v", context, err)
	}
	if info.Type != chatproto.ChatTypeInfo || info.VID != 0 || info.Message != message {
		t.Fatalf("unexpected %s info chat: %+v", context, info)
	}
}

func assertPostFloorGoldDropSuccessBurst(t *testing.T, frames [][]byte, ownerVID uint32, ownerName string, x, y, z int32, amount int32, remainingGold int32, context string) {
	t.Helper()
	if len(frames) != 3 {
		t.Fatalf("expected %s to emit POINT_CHANGE + GROUND_ADD + OWNERSHIP, got %d frames", context, len(frames))
	}
	point, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode %s gold point change: %v", context, err)
	}
	if point != (worldproto.PlayerPointChangePacket{VID: ownerVID, Type: bootstrapGoldPointType, Amount: -amount, Value: remainingGold}) {
		t.Fatalf("unexpected %s gold point change: %+v", context, point)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s gold ground add: %v", context, err)
	}
	if ground.VID == 0 || ground.Vnum != 1 || ground.X != x || ground.Y != y || ground.Z != z {
		t.Fatalf("unexpected %s gold ground add: %+v", context, ground)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, frames[2]))
	if err != nil {
		t.Fatalf("decode %s gold ownership: %v", context, err)
	}
	if ownership != (itemproto.OwnershipPacket{VID: ground.VID, OwnerName: ownerName}) {
		t.Fatalf("unexpected %s gold ownership: got %+v want vid %d owner %q", context, ownership, ground.VID, ownerName)
	}
}

func TestGameSessionFlowPostFloorItemPickupFailsClosed(t *testing.T) {
	login := "post-floor-item-pickup"
	loginKey := uint32(0x19191b68)
	owner := peerVisibilityCharacter("DeadItemPickupOwner", 0x01030b68, 0x02040b68, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1103, Vnum: 27001, Count: 3, Slot: 5}}
	templates := []itemcatalog.Template{{Vnum: 27001, Name: "Post Floor Pickup Potion", Stackable: true, MaxCount: 200}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	ground := dropAndDecodeGroundAdd(t, flow, itemproto.InventoryPosition(5))
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected pre-floor dropped ground item to stay registered before pickup denial")
	}
	emptyOwner := owner
	emptyOwner.Inventory = []inventory.ItemInstance{}
	assertExchangeAccountUnchanged(t, accounts, login, emptyOwner, "pre-floor ITEM_DROP clears carried inventory")

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientPickup(itemproto.ClientPickupPacket{VID: ground.VID})))
	if err != nil {
		t.Fatalf("unexpected post-floor ITEM_PICKUP dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor ITEM_PICKUP to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor ITEM_PICKUP to queue no frames, got %d", len(queued))
	}
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected post-floor ITEM_PICKUP denial to leave ground item registered")
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, emptyOwner, "post-floor ITEM_PICKUP")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor ITEM_PICKUP: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor ITEM_PICKUP, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientPickup(itemproto.ClientPickupPacket{VID: ground.VID})))
	if err != nil {
		t.Fatalf("unexpected post-restart ITEM_PICKUP: %v", err)
	}
	assertPostFloorItemPickupSuccessBurst(t, reuseOut, 5, 27001, 3, "post-restart ITEM_PICKUP")
	if runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected post-restart ITEM_PICKUP to remove the ground handle")
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart ITEM_PICKUP: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after ITEM_PICKUP floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.Inventory = []inventory.ItemInstance{{ID: 1103, Vnum: 27001, Count: 3, Slot: 5}}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart ITEM_PICKUP restores carried inventory")
}

func TestGameSessionFlowPostFloorItemPickupFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-item-pickup-town"
	loginKey := uint32(0x19191b69)
	owner := peerVisibilityCharacter("DeadItemPickupTownOwner", 0x01030b69, 0x02040b69, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1104, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 1105, Vnum: 27001, Count: 2, Slot: 6},
	}
	templates := []itemcatalog.Template{{Vnum: 27001, Name: "Post Floor Pickup Potion", Stackable: true, MaxCount: 200}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	ground := dropAndDecodeGroundAdd(t, flow, itemproto.InventoryPosition(5))
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected pre-floor dropped ground item to stay registered before town pickup denial")
	}
	remainingOwner := owner
	remainingOwner.Inventory = []inventory.ItemInstance{{ID: 1105, Vnum: 27001, Count: 2, Slot: 6}}
	assertExchangeAccountUnchanged(t, accounts, login, remainingOwner, "pre-floor town ITEM_DROP leaves second stack")

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientPickup(itemproto.ClientPickupPacket{VID: ground.VID})))
	if err != nil {
		t.Fatalf("unexpected post-floor town ITEM_PICKUP dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town ITEM_PICKUP to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town ITEM_PICKUP to queue no frames, got %d", len(queued))
	}
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected post-floor town ITEM_PICKUP denial to leave ground item registered")
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, remainingOwner, "post-floor town ITEM_PICKUP")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor ITEM_PICKUP: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor ITEM_PICKUP, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor ITEM_PICKUP /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after ITEM_PICKUP floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after ITEM_PICKUP floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after ITEM_PICKUP floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	townGround := dropAndDecodeGroundAdd(t, flow, itemproto.InventoryPosition(6))
	if townGround.X != 52070 || townGround.Y != 166600 {
		t.Fatalf("expected post-restart_town drop at empire town position, got %+v", townGround)
	}
	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientPickup(itemproto.ClientPickupPacket{VID: townGround.VID})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town ITEM_PICKUP: %v", err)
	}
	assertPostFloorItemPickupSuccessBurst(t, reuseOut, 6, 27001, 2, "post-restart_town ITEM_PICKUP")
	if runtime.sharedWorld.GroundItemExists(townGround.VID) {
		t.Fatal("expected post-restart_town ITEM_PICKUP to remove the town ground handle")
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town ITEM_PICKUP: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after ITEM_PICKUP floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town + ITEM_PICKUP to persist recovered owner HP %d after ITEM_PICKUP floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Inventory = []inventory.ItemInstance{{ID: 1105, Vnum: 27001, Count: 2, Slot: 6}}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town ITEM_PICKUP restores town-dropped stack")
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected source-map pre-floor ground handle to remain pending after town pickup recovery")
	}
}

func TestGameSessionFlowPostFloorGoldPickupFailsClosed(t *testing.T) {
	login := "post-floor-gold-pickup"
	loginKey := uint32(0x19191b6a)
	owner := peerVisibilityCharacter("DeadGoldPickupOwner", 0x01030b6a, 0x02040b6a, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, nil)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	ground := dropAndDecodeGoldGroundAdd(t, flow, 1200)
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected pre-floor dropped gold marker to stay registered before pickup denial")
	}
	debitedOwner := owner
	debitedOwner.Gold = 3800
	assertExchangeAccountUnchanged(t, accounts, login, debitedOwner, "pre-floor gold ITEM_DROP debits carried gold")

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientPickup(itemproto.ClientPickupPacket{VID: ground.VID})))
	if err != nil {
		t.Fatalf("unexpected post-floor gold ITEM_PICKUP dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor gold ITEM_PICKUP to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor gold ITEM_PICKUP to queue no frames, got %d", len(queued))
	}
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected post-floor gold ITEM_PICKUP denial to leave gold marker registered")
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, debitedOwner, "post-floor gold ITEM_PICKUP")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor gold ITEM_PICKUP: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor gold ITEM_PICKUP, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientPickup(itemproto.ClientPickupPacket{VID: ground.VID})))
	if err != nil {
		t.Fatalf("unexpected post-restart gold ITEM_PICKUP: %v", err)
	}
	assertPostFloorGoldPickupSuccessBurst(t, reuseOut, owner.VID, 1200, 5000, "post-restart gold ITEM_PICKUP")
	if runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected post-restart gold ITEM_PICKUP to remove the gold marker")
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart gold ITEM_PICKUP: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after gold ITEM_PICKUP floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.Gold = 5000
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart gold ITEM_PICKUP restores carried gold")
}

func TestGameSessionFlowPostFloorGoldPickupFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-gold-pickup-town"
	loginKey := uint32(0x19191b6b)
	owner := peerVisibilityCharacter("DeadGoldPickupTownOwner", 0x01030b6b, 0x02040b6b, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, nil)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	ground := dropAndDecodeGoldGroundAdd(t, flow, 1200)
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected pre-floor dropped gold marker to stay registered before town pickup denial")
	}
	debitedOwner := owner
	debitedOwner.Gold = 3800
	assertExchangeAccountUnchanged(t, accounts, login, debitedOwner, "pre-floor town gold ITEM_DROP debits carried gold")

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientPickup(itemproto.ClientPickupPacket{VID: ground.VID})))
	if err != nil {
		t.Fatalf("unexpected post-floor town gold ITEM_PICKUP dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town gold ITEM_PICKUP to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town gold ITEM_PICKUP to queue no frames, got %d", len(queued))
	}
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected post-floor town gold ITEM_PICKUP denial to leave gold marker registered")
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, debitedOwner, "post-floor town gold ITEM_PICKUP")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor gold ITEM_PICKUP: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor gold ITEM_PICKUP, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor gold ITEM_PICKUP /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after gold ITEM_PICKUP floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after gold ITEM_PICKUP floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after gold ITEM_PICKUP floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	townGround := dropAndDecodeGoldGroundAdd(t, flow, 800)
	if townGround.X != 52070 || townGround.Y != 166600 {
		t.Fatalf("expected post-restart_town gold drop at empire town position, got %+v", townGround)
	}
	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientPickup(itemproto.ClientPickupPacket{VID: townGround.VID})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town gold ITEM_PICKUP: %v", err)
	}
	assertPostFloorGoldPickupSuccessBurst(t, reuseOut, owner.VID, 800, 3800, "post-restart_town gold ITEM_PICKUP")
	if runtime.sharedWorld.GroundItemExists(townGround.VID) {
		t.Fatal("expected post-restart_town gold ITEM_PICKUP to remove the town gold marker")
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town gold ITEM_PICKUP: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after gold ITEM_PICKUP floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town + gold ITEM_PICKUP to persist recovered owner HP %d after gold ITEM_PICKUP floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Gold = 3800
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town gold ITEM_PICKUP restores town-dropped gold")
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected source-map pre-floor gold marker to remain pending after town pickup recovery")
	}
}

func dropAndDecodeGoldGroundAdd(t *testing.T, flow service.SessionFlow, amount uint32) itemproto.GroundAddPacket {
	t.Helper()
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Elk: amount})))
	if err != nil {
		t.Fatalf("unexpected gold drop error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected gold drop to emit POINT_CHANGE, GROUND_ADD, and OWNERSHIP, got %d frames", len(out))
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode gold drop ground add: %v", err)
	}
	if ground.VID == 0 || ground.Vnum != 1 {
		t.Fatalf("unexpected gold drop ground add: %+v", ground)
	}
	return ground
}

func assertPostFloorGoldPickupSuccessBurst(t *testing.T, frames [][]byte, ownerVID uint32, amount int32, restoredGold int32, context string) {
	t.Helper()
	if len(frames) != 3 {
		t.Fatalf("expected %s to emit GROUND_DEL + POINT_CHANGE + ITEM_GET, got %d frames", context, len(frames))
	}
	if _, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, frames[0])); err != nil {
		t.Fatalf("decode %s gold ground delete: %v", context, err)
	}
	point, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s gold point change: %v", context, err)
	}
	if point != (worldproto.PlayerPointChangePacket{VID: ownerVID, Type: bootstrapGoldPointType, Amount: amount, Value: restoredGold}) {
		t.Fatalf("unexpected %s gold point change: %+v", context, point)
	}
	get, err := itemproto.DecodeGet(decodeSingleFrame(t, frames[2]))
	if err != nil {
		t.Fatalf("decode %s gold item get: %v", context, err)
	}
	if get != (itemproto.GetPacket{Vnum: 1, Count: 1, Arg: itemproto.GetArgNormal}) {
		t.Fatalf("unexpected %s gold item get: %+v", context, get)
	}
}

func assertPostFloorItemPickupSuccessBurst(t *testing.T, frames [][]byte, inventorySlot uint16, vnum uint32, count uint8, context string) {
	t.Helper()
	if len(frames) != 3 {
		t.Fatalf("expected %s to emit GROUND_DEL + ITEM_SET + ITEM_GET, got %d frames", context, len(frames))
	}
	if _, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, frames[0])); err != nil {
		t.Fatalf("decode %s ground delete: %v", context, err)
	}
	itemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s item set: %v", context, err)
	}
	if itemSet.Position != itemproto.InventoryPosition(inventorySlot) || itemSet.Vnum != vnum || itemSet.Count != count {
		t.Fatalf("unexpected %s item set: %+v", context, itemSet)
	}
	get, err := itemproto.DecodeGet(decodeSingleFrame(t, frames[2]))
	if err != nil {
		t.Fatalf("decode %s item get: %v", context, err)
	}
	if get != (itemproto.GetPacket{Vnum: vnum, Count: count, Arg: itemproto.GetArgNormal}) {
		t.Fatalf("unexpected %s item get: %+v", context, get)
	}
}

func assertPostFloorUnequipItemSuccessBurst(t *testing.T, frames [][]byte, ownerVID uint32, mainPart, hairPart, weaponVnum uint16, inventorySlot uint16, wantHP int32, context string) {
	t.Helper()
	if len(frames) != 4 {
		t.Fatalf("expected %s to emit delete+set+point-change+update, got %d frames", context, len(frames))
	}
	weaponPos, err := itemproto.EquipmentPosition(4)
	if err != nil {
		t.Fatalf("build %s weapon equipment position: %v", context, err)
	}
	itemDel, err := itemproto.DecodeDel(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode %s equipment delete: %v", context, err)
	}
	if itemDel.Position != weaponPos {
		t.Fatalf("unexpected %s equipment delete position: %+v", context, itemDel.Position)
	}
	itemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s inventory set: %v", context, err)
	}
	if itemSet.Position != itemproto.InventoryPosition(inventorySlot) || itemSet.Vnum != uint32(weaponVnum) || itemSet.Count != 1 {
		t.Fatalf("unexpected %s inventory set frame: %+v", context, itemSet)
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, frames[2]))
	if err != nil {
		t.Fatalf("decode %s point-change: %v", context, err)
	}
	if pointChange.VID != ownerVID || pointChange.Type != bootstrapPlayerPointType || pointChange.Amount != -10 || pointChange.Value != wantHP {
		t.Fatalf("unexpected %s point-change: %+v", context, pointChange)
	}
	update, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, frames[3]))
	if err != nil {
		t.Fatalf("decode %s character update: %v", context, err)
	}
	wantParts := [worldproto.CharacterEquipmentPartCount]uint16{mainPart, 0, 0, hairPart}
	if update.VID != ownerVID || update.Parts != wantParts {
		t.Fatalf("unexpected %s appearance update: %+v want parts %+v", context, update, wantParts)
	}
}

func assertPostFloorEquipItemSuccessBurst(t *testing.T, frames [][]byte, ownerVID uint32, mainPart, hairPart, weaponVnum uint16, wantHP int32, context string) {
	t.Helper()
	if len(frames) != 4 {
		t.Fatalf("expected %s to emit delete+set+point-change+update, got %d frames", context, len(frames))
	}
	itemDel, err := itemproto.DecodeDel(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode %s item delete: %v", context, err)
	}
	if itemDel.Position != itemproto.InventoryPosition(8) {
		t.Fatalf("unexpected %s item delete position: %+v", context, itemDel.Position)
	}
	weaponPos, err := itemproto.EquipmentPosition(4)
	if err != nil {
		t.Fatalf("build %s weapon equipment position: %v", context, err)
	}
	itemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s item set: %v", context, err)
	}
	if itemSet.Position != weaponPos || itemSet.Vnum != uint32(weaponVnum) || itemSet.Count != 1 {
		t.Fatalf("unexpected %s item set frame: %+v", context, itemSet)
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, frames[2]))
	if err != nil {
		t.Fatalf("decode %s point-change: %v", context, err)
	}
	if pointChange.VID != ownerVID || pointChange.Type != bootstrapPlayerPointType || pointChange.Amount != 10 || pointChange.Value != wantHP {
		t.Fatalf("unexpected %s point-change: %+v", context, pointChange)
	}
	update, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, frames[3]))
	if err != nil {
		t.Fatalf("decode %s character update: %v", context, err)
	}
	wantParts := [worldproto.CharacterEquipmentPartCount]uint16{mainPart, weaponVnum, 0, hairPart}
	if update.VID != ownerVID || update.Parts != wantParts {
		t.Fatalf("unexpected %s appearance update: %+v want parts %+v", context, update, wantParts)
	}
}

func assertPostFloorItemMoveSuccessBurst(t *testing.T, frames [][]byte, sourceSlot, destinationSlot uint16, vnum uint32, count uint8, context string) {
	t.Helper()
	if len(frames) < 2 {
		t.Fatalf("expected %s to emit item delete + item set, got %d frames", context, len(frames))
	}
	itemDel, err := itemproto.DecodeDel(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode %s item delete: %v", context, err)
	}
	if itemDel.Position != itemproto.InventoryPosition(sourceSlot) {
		t.Fatalf("unexpected %s item delete position: %+v", context, itemDel.Position)
	}
	itemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s item set: %v", context, err)
	}
	if itemSet.Position != itemproto.InventoryPosition(destinationSlot) || itemSet.Vnum != vnum || itemSet.Count != count {
		t.Fatalf("unexpected %s item set frame: %+v", context, itemSet)
	}
}

func newPostFloorItemGuardRuntime(t *testing.T, login string, loginKey uint32, owner loginticket.Character, templates []itemcatalog.Template) (*gameRuntime, accountstore.Store, uint32) {
	t.Helper()
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor item-guard account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, templates)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected post-floor item-guard runtime error: %v", err)
	}

	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_item_guard",
		Name:          "PracticeMobPostFloorItemGuard",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import post-floor item-guard practice mob: %v", err)
	}
	// ImportContentBundle replaces item templates from the bundle; re-seed the
	// authored templates needed by post-floor reopen recovery proofs.
	if len(templates) > 0 {
		if err := runtime.replaceItemTemplates(itemcatalog.Snapshot{Templates: templates}); err != nil {
			t.Fatalf("restore post-floor item-guard templates after content import: %v", err)
		}
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor item-guard practice mob, got %#v", actors)
	}
	return runtime, accounts, uint32(actors[0].EntityID)
}

func drivePracticeMobOwnerToBootstrapHPFloor(t *testing.T, flow service.SessionFlow, owner loginticket.Character, targetVID uint32) {
	t.Helper()
	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection before post-floor item guard: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected one target-selection frame before post-floor item guard, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected practice-mob attack before post-floor item guard: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, point-change, self dead, and clear-target frames at HP floor, got %d frames", len(attackOut))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode post-floor item guard point-change: %v", err)
	}
	if pointChange.VID != owner.VID || pointChange.Type != bootstrapPlayerPointType || pointChange.Amount != bootstrapPracticeMobRetaliationPointDelta || pointChange.Value != 0 {
		t.Fatalf("unexpected post-floor item guard point-change: %+v", pointChange)
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, attackOut[2]))
	if err != nil {
		t.Fatalf("decode post-floor item guard self dead: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected self DEAD for owner %#08x, got %#08x", owner.VID, dead.VID)
	}
	clearTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[3]))
	if err != nil {
		t.Fatalf("decode post-floor item guard target clear: %v", err)
	}
	if clearTarget.TargetVID != 0 || clearTarget.HPPercent != 0 {
		t.Fatalf("expected post-floor item guard to clear active target, got %+v", clearTarget)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after post-floor item guard owner death without peers, got %d", len(queued))
	}
}

func assertPostFloorItemGuardAccountUnchanged(t *testing.T, accounts accountstore.Store, login string, want loginticket.Character, context string) {
	t.Helper()
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted %s account: %v", context, err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected one persisted %s character, got %d", context, len(persisted.Characters))
	}
	got := persisted.Characters[0]
	if !samePostFloorItemGuardInventory(got.Inventory, want.Inventory) {
		t.Fatalf("%s mutated inventory: got %+v want %+v", context, got.Inventory, want.Inventory)
	}
	if !samePostFloorItemGuardQuickslots(got.Quickslots, want.Quickslots) {
		t.Fatalf("%s mutated quickslots: got %+v want %+v", context, got.Quickslots, want.Quickslots)
	}
	if !samePostFloorItemGuardInventory(got.Equipment, want.Equipment) {
		t.Fatalf("%s mutated equipment: got %+v want %+v", context, got.Equipment, want.Equipment)
	}
	if got.Gold != want.Gold {
		t.Fatalf("%s mutated gold: got %d want %d", context, got.Gold, want.Gold)
	}
	if got.Points[bootstrapPlayerPointValueIndex] != 0 {
		t.Fatalf("%s expected persisted death-floor HP 0, got %d", context, got.Points[bootstrapPlayerPointValueIndex])
	}
}

func samePostFloorItemGuardInventory(got []inventory.ItemInstance, want []inventory.ItemInstance) bool {
	if len(got) == 0 && len(want) == 0 {
		return true
	}
	return reflect.DeepEqual(got, want)
}

func samePostFloorItemGuardQuickslots(got []loginticket.Quickslot, want []loginticket.Quickslot) bool {
	if len(got) == 0 && len(want) == 0 {
		return true
	}
	return reflect.DeepEqual(got, want)
}
