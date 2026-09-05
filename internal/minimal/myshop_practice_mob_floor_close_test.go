package minimal

import (
	"testing"
	"time"

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
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenMyShop(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopImmediateFloorOwner", 0x01030891, 0x02040891, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 891, Vnum: 27001, Count: 3, Slot: 5}, {ID: 941, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("MyShopImmediateFloorPeer", 0x01030892, 0x02040892, 1120, 2120, 0, 101, 201)
	login := "myshop-immediate-floor"
	loginKey := uint32(0x70708991)
	peerLogin := "myshop-immediate-floor-peer"
	peerLoginKey := uint32(0x70708992)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, peerLogin, peerLoginKey, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop immediate floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop immediate floor-close peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Shop Potion",
		Stackable: true,
		MaxCount:  200,
	}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop immediate floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000491, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_myshop_immediate_floor_close",
		Name:          "PracticeMobMyShopImmediateFloorClose",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import myshop immediate floor-close content bundle: %v", err)
	}

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
		t.Fatalf("expected owner to receive peer-entry frames before myshop immediate floor-close, got %d", len(queued))
	}
	_ = flushServerFrames(t, peerFlow)

	actors := runtime.StaticActors()
	var targetVID uint32
	for _, actor := range actors {
		if actor.Name == "PracticeMobMyShopImmediateFloorClose" {
			targetVID = uint32(actor.EntityID)
			break
		}
	}
	if targetVID == 0 {
		t.Fatalf("expected practice mob actor after import, got %+v", actors)
	}

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected accepted MYSHOP before immediate floor-close: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `expected accepted MYSHOP before immediate floor-close to emit one SHOP_SIGN frame, got %d`)
	livePeerSign := flushServerFrames(t, peerFlow)
	if len(livePeerSign) != 1 {
		t.Fatalf("expected peer to receive one live SHOP_SIGN around-broadcast before floor, got %d", len(livePeerSign))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before myshop immediate floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before myshop immediate floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before myshop immediate floor-close: %v", err)
	}
	if len(attackOut) != 6 {
		t.Fatalf("expected target refresh, point-loss, self dead, clear-target, owner damage-info, and empty MYSHOP SHOP_SIGN, got %d", len(attackOut))
	}
	next := assertOwnerFloorDeathSequence(t, attackOut, 1, owner.VID, bootstrapPracticeMobRetaliationPointDelta, "myshop_practice_mob_floor_close owner-floor")
	assertMyShopEmptySignFrame(t, attackOut[next], owner.VID, "myshop immediate floor-close")

	peerQueued := flushServerFrames(t, peerFlow)
	if len(peerQueued) != 3 {
		t.Fatalf("expected peer DEAD, owner damage-info, plus empty MYSHOP SHOP_SIGN around-broadcast after immediate floor, got %d", len(peerQueued))
	}
	remaining := assertOwnerFloorPeerDeadFanout(t, peerQueued, owner.VID, int32(-bootstrapPracticeMobRetaliationPointDelta), "myshop_practice_mob_floor_close owner-floor peer")
	if len(remaining) != 1 {
		t.Fatalf("expected 1 extra owner-floor peer frame(s) after DEAD + damage-info, got %d leftover frames", len(remaining))
	}
	assertMyShopEmptySignFrame(t, remaining[0], owner.VID, "myshop immediate floor-close peer empty sign")

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no delayed retaliation frames after immediate myshop floor close, got %d", len(queued))
	}
	alreadyClosedOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_myshop",
	})))
	if err != nil {
		t.Fatalf("unexpected already-closed /close_myshop after immediate floor: %v", err)
	}
	if len(alreadyClosedOut) != 0 {
		t.Fatalf("expected already-closed /close_myshop after immediate floor to emit no frames, got %d", len(alreadyClosedOut))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after myshop immediate floor: %v", err)
	}
	if len(restartOut) < 4 {
		t.Fatalf("expected /restart_here recovery frames after myshop immediate floor, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart exchange start after myshop immediate floor: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected post-restart exchange start to succeed after myshop immediate floor clear, got %d frames", len(startOut))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0])); err == nil {
		t.Fatalf("expected post-restart exchange start after myshop immediate floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "post-restart exchange start after myshop immediate floor clear")
	peerStart := flushServerFrames(t, peerFlow)
	if len(peerStart) != 1 {
		t.Fatalf("expected peer exchange start after myshop immediate floor clear, got %d", len(peerStart))
	}
	assertExchangeStartFrame(t, peerStart[0], owner.VID, "peer exchange start after myshop immediate floor clear")
	assertExchangeAccountUnchanged(t, accounts, login, characterAfterMyShopBagConsume(owner), "myshop immediate floor close")
}

func TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenMyShop(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopDelayedFloorOwner", 0x01030893, 0x02040893, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 893, Vnum: 27001, Count: 3, Slot: 5}, {ID: 943, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("MyShopDelayedFloorPeer", 0x01030894, 0x02040894, 1120, 2120, 0, 101, 201)
	login := "myshop-delayed-floor"
	loginKey := uint32(0x70708993)
	peerLogin := "myshop-delayed-floor-peer"
	peerLoginKey := uint32(0x70708994)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, peerLogin, peerLoginKey, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop delayed floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop delayed floor-close peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Shop Potion",
		Stackable: true,
		MaxCount:  200,
	}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop delayed floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000493, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_myshop_delayed_floor_close",
		Name:          "PracticeMobMyShopDelayedFloorClose",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import myshop delayed floor-close content bundle: %v", err)
	}

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
		t.Fatalf("expected owner to receive peer-entry frames before myshop delayed floor-close, got %d", len(queued))
	}
	_ = flushServerFrames(t, peerFlow)

	actors := runtime.StaticActors()
	var targetVID uint32
	for _, actor := range actors {
		if actor.Name == "PracticeMobMyShopDelayedFloorClose" {
			targetVID = uint32(actor.EntityID)
			break
		}
	}
	if targetVID == 0 {
		t.Fatalf("expected practice mob actor after import, got %+v", actors)
	}

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected accepted MYSHOP before delayed floor-close: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `expected accepted MYSHOP before delayed floor-close to emit one SHOP_SIGN frame, got %d`)
	if queued := flushServerFrames(t, peerFlow); len(queued) != 1 {
		t.Fatalf("expected peer to receive one live SHOP_SIGN around-broadcast before delayed floor, got %d", len(queued))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before myshop delayed floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before myshop delayed floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before myshop delayed floor-close: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, first point-loss retaliation, and self damage-info before delayed floor, got %d frames", len(attackOut))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) == 0 {
		t.Fatal("expected peer to receive live-hit retaliation fanout before delayed floor")
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, ownerFlow)
	if len(queued) != 5 {
		t.Fatalf("expected delayed retaliation floor to queue point-loss, self dead, clear-target, owner damage-info, and empty MYSHOP SHOP_SIGN, got %d", len(queued))
	}
	next := assertOwnerFloorDeathSequence(t, queued, 0, owner.VID, bootstrapPracticeMobRetaliationPointDelta, "myshop_practice_mob_floor_close owner-floor")
	assertMyShopEmptySignFrame(t, queued[next], owner.VID, "myshop delayed floor-close")

	peerQueued := flushServerFrames(t, peerFlow)
	if len(peerQueued) != 3 {
		t.Fatalf("expected peer DEAD, owner damage-info, plus empty MYSHOP SHOP_SIGN around-broadcast after delayed floor, got %d", len(peerQueued))
	}
	remaining := assertOwnerFloorPeerDeadFanout(t, peerQueued, owner.VID, int32(-bootstrapPracticeMobRetaliationPointDelta), "myshop_practice_mob_floor_close owner-floor peer")
	if len(remaining) != 1 {
		t.Fatalf("expected 1 extra owner-floor peer frame(s) after DEAD + damage-info, got %d leftover frames", len(remaining))
	}
	assertMyShopEmptySignFrame(t, remaining[0], owner.VID, "myshop delayed floor-close peer empty sign")

	alreadyClosedOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_myshop",
	})))
	if err != nil {
		t.Fatalf("unexpected already-closed /close_myshop after delayed floor: %v", err)
	}
	if len(alreadyClosedOut) != 0 {
		t.Fatalf("expected already-closed /close_myshop after delayed floor to emit no frames, got %d", len(alreadyClosedOut))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after myshop delayed floor: %v", err)
	}
	if len(restartOut) < 4 {
		t.Fatalf("expected /restart_here recovery frames after myshop delayed floor, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart exchange start after myshop delayed floor: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected post-restart exchange start to succeed after myshop delayed floor clear, got %d frames", len(startOut))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0])); err == nil {
		t.Fatalf("expected post-restart exchange start after myshop delayed floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "post-restart exchange start after myshop delayed floor clear")
	peerStart := flushServerFrames(t, peerFlow)
	if len(peerStart) != 1 {
		t.Fatalf("expected peer exchange start after myshop delayed floor clear, got %d", len(peerStart))
	}
	assertExchangeStartFrame(t, peerStart[0], owner.VID, "peer exchange start after myshop delayed floor clear")
	assertExchangeAccountUnchanged(t, accounts, login, characterAfterMyShopBagConsume(owner), "myshop delayed floor close")
}

func TestGameSessionFlowPracticeMobImmediateRetaliationFloorQueuesGuestBrowseShopEnd(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopGuestFloorHost", 0x01030895, 0x02040895, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 895, Vnum: 27001, Count: 3, Slot: 5}, {ID: 945, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	guest := peerVisibilityCharacter("MyShopGuestFloorGuest", 0x01030896, 0x02040896, 1120, 2120, 0, 101, 201)
	guest.Gold = 22222
	ownerLogin := "myshop-guest-floor-host"
	guestLogin := "myshop-guest-floor-guest"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70708995, owner)
	issuePeerTicket(t, ticketStore, guestLogin, 0x70708996, guest)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop guest floor-close host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: guestLogin, Empire: guest.Empire, Characters: cloneCharacters([]loginticket.Character{guest})}); err != nil {
		t.Fatalf("seed myshop guest floor-close guest account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Shop Potion",
		Stackable: true,
		MaxCount:  200,
	}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop guest floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000495, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_myshop_guest_floor_close",
		Name:          "PracticeMobMyShopGuestFloorClose",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import myshop guest floor-close content bundle: %v", err)
	}

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70708995)
	if len(ownerEnter) < 8 {
		t.Fatalf("expected host bootstrap with visible practice mob, got %d frames", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	guestFlow, guestEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), guestLogin, 0x70708996)
	if len(guestEnter) < 11 {
		t.Fatalf("expected guest bootstrap with visible host and mob, got %d frames", len(guestEnter))
	}
	defer closeSessionFlow(t, guestFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected host to receive guest-entry frames before myshop guest floor-close, got %d", len(queued))
	}
	_ = flushServerFrames(t, guestFlow)

	actors := runtime.StaticActors()
	var targetVID uint32
	for _, actor := range actors {
		if actor.Name == "PracticeMobMyShopGuestFloorClose" {
			targetVID = uint32(actor.EntityID)
			break
		}
	}
	if targetVID == 0 {
		t.Fatalf("expected practice mob actor after import, got %+v", actors)
	}

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected accepted MYSHOP before guest floor-close: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `expected accepted MYSHOP before guest floor-close to emit one SHOP_SIGN frame, got %d`)
	if queued := flushServerFrames(t, guestFlow); len(queued) != 1 {
		t.Fatalf("expected guest to receive one live SHOP_SIGN around-broadcast before floor, got %d", len(queued))
	}

	browseOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: owner.VID})))
	if err != nil || len(browseOut) != 1 {
		t.Fatalf("unexpected guest browse before host floor-close: out=%d err=%v", len(browseOut), err)
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, browseOut[0])); err != nil {
		t.Fatalf("decode guest browse SHOP START before host floor-close: %v", err)
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before myshop guest floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before myshop guest floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before myshop guest floor-close: %v", err)
	}
	if len(attackOut) != 6 {
		t.Fatalf("expected target refresh, point-loss, self dead, clear-target, owner damage-info, and empty MYSHOP SHOP_SIGN, got %d", len(attackOut))
	}
	next := assertOwnerFloorDeathSequence(t, attackOut, 1, owner.VID, bootstrapPracticeMobRetaliationPointDelta, "myshop_practice_mob_floor_close owner-floor")
	assertMyShopEmptySignFrame(t, attackOut[next], owner.VID, "myshop guest floor-close host empty sign")

	guestQueued := flushServerFrames(t, guestFlow)
	if len(guestQueued) != 4 {
		t.Fatalf("expected guest DEAD, owner damage-info, SHOP END, and empty MYSHOP SHOP_SIGN after host floor, got %d", len(guestQueued))
	}
	remaining := assertOwnerFloorPeerDeadFanout(t, guestQueued, owner.VID, int32(-bootstrapPracticeMobRetaliationPointDelta), "myshop_practice_mob_floor_close owner-floor peer")
	if len(remaining) != 2 {
		t.Fatalf("expected 2 extra owner-floor peer frames after DEAD + damage-info, got %d leftover frames", len(remaining))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, remaining[0])); err != nil {
		t.Fatalf("decode guest SHOP END after myshop guest floor-close: %v", err)
	}
	assertMyShopEmptySignFrame(t, remaining[1], owner.VID, "myshop guest floor-close guest empty sign")

	secondEndOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected guest SHOP END after host floor-close: %v", err)
	}
	if len(secondEndOut) != 0 {
		t.Fatalf("expected already-closed guest SHOP END after host floor to emit no frames, got %d", len(secondEndOut))
	}
	rebrowseOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: owner.VID})))
	if err != nil {
		t.Fatalf("unexpected post-floor guest ON_CLICK: %v", err)
	}
	if len(rebrowseOut) != 0 {
		t.Fatalf("expected post-floor guest ON_CLICK against closed host to emit no frames, got %d", len(rebrowseOut))
	}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, characterAfterMyShopBagConsume(owner), "myshop guest floor-close host")
	assertExchangeAccountUnchanged(t, accounts, guestLogin, guest, "myshop guest floor-close guest")
}

func TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesGuestBrowseOnDeadGuest(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	// Keep the host outside DefaultSpawnAggroRadius of the practice mob so proximity
	// engagement does not silently block the browsing guest's TARGET / ATTACK path.
	host := peerVisibilityCharacter("MyShopDeadGuestHost", 0x01030897, 0x02040897, 900, 1900, 0, 101, 201)
	host.Gold = 5000
	host.Inventory = []inventory.ItemInstance{{ID: 897, Vnum: 27001, Count: 3, Slot: 5}, {ID: 947, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	guest := peerVisibilityCharacter("MyShopDeadGuestGuest", 0x01030898, 0x02040898, 1120, 2120, 0, 101, 201)
	guest.Points[bootstrapPlayerPointValueIndex] = 1
	guest.Gold = 22222
	hostLogin := "myshop-dead-guest-host"
	guestLogin := "myshop-dead-guest-guest"
	issuePeerTicket(t, ticketStore, hostLogin, 0x70708997, host)
	issuePeerTicket(t, ticketStore, guestLogin, 0x70708998, guest)
	if err := accounts.Save(accountstore.Account{Login: hostLogin, Empire: host.Empire, Characters: cloneCharacters([]loginticket.Character{host})}); err != nil {
		t.Fatalf("seed myshop dead-guest host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: guestLogin, Empire: guest.Empire, Characters: cloneCharacters([]loginticket.Character{guest})}); err != nil {
		t.Fatalf("seed myshop dead-guest guest account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Shop Potion",
		Stackable: true,
		MaxCount:  200,
	}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop dead-guest floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000497, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_myshop_dead_guest_floor_close",
		Name:          "PracticeMobMyShopDeadGuestFloorClose",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import myshop dead-guest floor-close content bundle: %v", err)
	}

	hostFlow, hostEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), hostLogin, 0x70708997)
	if len(hostEnter) < 8 {
		t.Fatalf("expected host bootstrap with visible practice mob, got %d frames", len(hostEnter))
	}
	defer closeSessionFlow(t, hostFlow)
	guestFlow, guestEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), guestLogin, 0x70708998)
	if len(guestEnter) < 11 {
		t.Fatalf("expected guest bootstrap with visible host and mob, got %d frames", len(guestEnter))
	}
	defer closeSessionFlow(t, guestFlow)
	if queued := flushServerFrames(t, hostFlow); len(queued) != 3 {
		t.Fatalf("expected host to receive guest-entry frames before dead-guest floor-close, got %d", len(queued))
	}
	_ = flushServerFrames(t, guestFlow)

	actors := runtime.StaticActors()
	var targetVID uint32
	for _, actor := range actors {
		if actor.Name == "PracticeMobMyShopDeadGuestFloorClose" {
			targetVID = uint32(actor.EntityID)
			break
		}
	}
	if targetVID == 0 {
		t.Fatalf("expected practice mob actor after import, got %+v", actors)
	}

	openOut, err := hostFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected MYSHOP open error: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, host.VID, 4, `unexpected accepted MYSHOP before dead-guest floor-close: out=%d err=%v`)
	if queued := flushServerFrames(t, guestFlow); len(queued) != 1 {
		t.Fatalf("expected guest to receive one live SHOP_SIGN around-broadcast before dead-guest floor, got %d", len(queued))
	}

	browseOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: host.VID})))
	if err != nil || len(browseOut) != 1 {
		t.Fatalf("unexpected guest browse before dead-guest floor-close: out=%d err=%v", len(browseOut), err)
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, browseOut[0])); err != nil {
		t.Fatalf("decode guest browse SHOP START before dead-guest floor-close: %v", err)
	}

	selectOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before dead-guest floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before dead-guest floor-close, got %d", len(selectOut))
	}

	attackOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before dead-guest floor-close: %v", err)
	}
	if len(attackOut) != 6 {
		t.Fatalf("expected target refresh, point-loss, self dead, clear-target, owner damage-info, and guest SHOP END, got %d", len(attackOut))
	}
	next := assertOwnerFloorDeathSequence(t, attackOut, 1, guest.VID, bootstrapPracticeMobRetaliationPointDelta, "myshop_practice_mob_floor_close owner-floor")
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, attackOut[next])); err != nil {
		t.Fatalf("decode dead-guest floor-close guest SHOP END: %v", err)
	}

	hostQueued := flushServerFrames(t, hostFlow)
	if len(hostQueued) != 2 {
		t.Fatalf("expected host to receive guest DEAD plus owner damage-info after dead-guest floor, got %d", len(hostQueued))
	}
	remaining := assertOwnerFloorPeerDeadFanout(t, hostQueued, guest.VID, int32(-bootstrapPracticeMobRetaliationPointDelta), "myshop_practice_mob_floor_close owner-floor peer")
	if len(remaining) != 0 {
		t.Fatalf("expected no extra owner-floor peer frames after DEAD + damage-info, got %d", len(remaining))
	}

	secondEndOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected guest SHOP END after dead-guest floor: %v", err)
	}
	if len(secondEndOut) != 0 {
		t.Fatalf("expected already-closed guest SHOP END after dead-guest floor to emit no frames, got %d", len(secondEndOut))
	}
	rebrowseOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: host.VID})))
	if err != nil {
		t.Fatalf("unexpected post-floor dead guest ON_CLICK: %v", err)
	}
	if len(rebrowseOut) != 0 {
		t.Fatalf("expected post-floor dead guest ON_CLICK to emit no frames, got %d", len(rebrowseOut))
	}
	assertExchangeAccountUnchanged(t, accounts, hostLogin, characterAfterMyShopBagConsume(host), "myshop dead-guest floor-close host")
	assertExchangeAccountUnchanged(t, accounts, guestLogin, guest, "myshop dead-guest floor-close guest")
}

func TestGameSessionFlowPracticeMobDelayedRetaliationFloorQueuesGuestBrowseShopEnd(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopGuestDelayedFloorHost", 0x010308A1, 0x020408A1, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 2209, Vnum: 27001, Count: 3, Slot: 5}, {ID: 2259, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	guest := peerVisibilityCharacter("MyShopGuestDelayedFloorGuest", 0x010308A2, 0x020408A2, 1120, 2120, 0, 101, 201)
	guest.Gold = 22222
	ownerLogin := "myshop-gdelay-host"
	guestLogin := "myshop-gdelay-guest"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70708aa1, owner)
	issuePeerTicket(t, ticketStore, guestLogin, 0x70708aa2, guest)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop guest delayed floor-close host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: guestLogin, Empire: guest.Empire, Characters: cloneCharacters([]loginticket.Character{guest})}); err != nil {
		t.Fatalf("seed myshop guest delayed floor-close guest account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Shop Potion",
		Stackable: true,
		MaxCount:  200,
	}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop guest delayed floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000501, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_myshop_guest_delayed_floor_close",
		Name:          "PracticeMobMyShopGuestDelayedFloorClose",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import myshop guest delayed floor-close content bundle: %v", err)
	}

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70708aa1)
	if len(ownerEnter) < 8 {
		t.Fatalf("expected host bootstrap with visible practice mob, got %d frames", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	guestFlow, guestEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), guestLogin, 0x70708aa2)
	if len(guestEnter) < 11 {
		t.Fatalf("expected guest bootstrap with visible host and mob, got %d frames", len(guestEnter))
	}
	defer closeSessionFlow(t, guestFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected host to receive guest-entry frames before myshop guest delayed floor-close, got %d", len(queued))
	}
	_ = flushServerFrames(t, guestFlow)

	actors := runtime.StaticActors()
	var targetVID uint32
	for _, actor := range actors {
		if actor.Name == "PracticeMobMyShopGuestDelayedFloorClose" {
			targetVID = uint32(actor.EntityID)
			break
		}
	}
	if targetVID == 0 {
		t.Fatalf("expected practice mob actor after import, got %+v", actors)
	}

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected accepted MYSHOP before guest delayed floor-close: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `expected accepted MYSHOP before guest delayed floor-close to emit one SHOP_SIGN frame, got %d`)
	if queued := flushServerFrames(t, guestFlow); len(queued) != 1 {
		t.Fatalf("expected guest to receive one live SHOP_SIGN around-broadcast before delayed floor, got %d", len(queued))
	}

	browseOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: owner.VID})))
	if err != nil || len(browseOut) != 1 {
		t.Fatalf("unexpected guest browse before host delayed floor-close: out=%d err=%v", len(browseOut), err)
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, browseOut[0])); err != nil {
		t.Fatalf("decode guest browse SHOP START before host delayed floor-close: %v", err)
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before myshop guest delayed floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before myshop guest delayed floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before myshop guest delayed floor-close: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, first point-loss retaliation, and self damage-info before delayed floor, got %d frames", len(attackOut))
	}
	if queued := flushServerFrames(t, guestFlow); len(queued) == 0 {
		t.Fatal("expected guest to receive live-hit retaliation fanout before delayed floor")
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, ownerFlow)
	if len(queued) != 5 {
		t.Fatalf("expected delayed retaliation floor to queue point-loss, self dead, clear-target, owner damage-info, and empty MYSHOP SHOP_SIGN, got %d", len(queued))
	}
	next := assertOwnerFloorDeathSequence(t, queued, 0, owner.VID, bootstrapPracticeMobRetaliationPointDelta, "myshop_practice_mob_floor_close owner-floor")
	assertMyShopEmptySignFrame(t, queued[next], owner.VID, "myshop guest delayed floor-close host empty sign")

	guestQueued := flushServerFrames(t, guestFlow)
	if len(guestQueued) != 4 {
		t.Fatalf("expected guest DEAD, owner damage-info, SHOP END, and empty MYSHOP SHOP_SIGN after host delayed floor, got %d", len(guestQueued))
	}
	remaining := assertOwnerFloorPeerDeadFanout(t, guestQueued, owner.VID, int32(-bootstrapPracticeMobRetaliationPointDelta), "myshop_practice_mob_floor_close owner-floor peer")
	if len(remaining) != 2 {
		t.Fatalf("expected 2 extra owner-floor peer frames after DEAD + damage-info, got %d leftover frames", len(remaining))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, remaining[0])); err != nil {
		t.Fatalf("decode guest SHOP END after myshop guest delayed floor-close: %v", err)
	}
	assertMyShopEmptySignFrame(t, remaining[1], owner.VID, "myshop guest delayed floor-close guest empty sign")

	secondEndOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected guest SHOP END after host delayed floor-close: %v", err)
	}
	if len(secondEndOut) != 0 {
		t.Fatalf("expected already-closed guest SHOP END after host delayed floor to emit no frames, got %d", len(secondEndOut))
	}
	rebrowseOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: owner.VID})))
	if err != nil {
		t.Fatalf("unexpected post-delayed-floor guest ON_CLICK: %v", err)
	}
	if len(rebrowseOut) != 0 {
		t.Fatalf("expected post-delayed-floor guest ON_CLICK against closed host to emit no frames, got %d", len(rebrowseOut))
	}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, characterAfterMyShopBagConsume(owner), "myshop guest delayed floor-close host")
	assertExchangeAccountUnchanged(t, accounts, guestLogin, guest, "myshop guest delayed floor-close guest")
}

func TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesGuestBrowseOnDeadGuest(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	// Keep the host outside DefaultSpawnAggroRadius of the practice mob so proximity
	// engagement does not silently block the browsing guest's TARGET / ATTACK path.
	host := peerVisibilityCharacter("MyShopDeadGuestDelayedHost", 0x010308A3, 0x020408A3, 900, 1900, 0, 101, 201)
	host.Gold = 5000
	host.Inventory = []inventory.ItemInstance{{ID: 2211, Vnum: 27001, Count: 3, Slot: 5}, {ID: 2261, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	guest := peerVisibilityCharacter("MyShopDeadGuestDelayedGuest", 0x010308A4, 0x020408A4, 1120, 2120, 0, 101, 201)
	guest.Points[bootstrapPlayerPointValueIndex] = 2
	guest.Gold = 22222
	hostLogin := "myshop-dgdelay-host"
	guestLogin := "myshop-dgdelay-guest"
	issuePeerTicket(t, ticketStore, hostLogin, 0x70708aa3, host)
	issuePeerTicket(t, ticketStore, guestLogin, 0x70708aa4, guest)
	if err := accounts.Save(accountstore.Account{Login: hostLogin, Empire: host.Empire, Characters: cloneCharacters([]loginticket.Character{host})}); err != nil {
		t.Fatalf("seed myshop dead-guest delayed host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: guestLogin, Empire: guest.Empire, Characters: cloneCharacters([]loginticket.Character{guest})}); err != nil {
		t.Fatalf("seed myshop dead-guest delayed guest account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Shop Potion",
		Stackable: true,
		MaxCount:  200,
	}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop dead-guest delayed floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000503, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_myshop_dead_guest_delayed_floor_close",
		Name:          "PracticeMobMyShopDeadGuestDelayedFloorClose",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import myshop dead-guest delayed floor-close content bundle: %v", err)
	}

	hostFlow, hostEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), hostLogin, 0x70708aa3)
	if len(hostEnter) < 8 {
		t.Fatalf("expected host bootstrap with visible practice mob, got %d frames", len(hostEnter))
	}
	defer closeSessionFlow(t, hostFlow)
	guestFlow, guestEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), guestLogin, 0x70708aa4)
	if len(guestEnter) < 11 {
		t.Fatalf("expected guest bootstrap with visible host and mob, got %d frames", len(guestEnter))
	}
	defer closeSessionFlow(t, guestFlow)
	if queued := flushServerFrames(t, hostFlow); len(queued) != 3 {
		t.Fatalf("expected host to receive guest-entry frames before dead-guest delayed floor-close, got %d", len(queued))
	}
	_ = flushServerFrames(t, guestFlow)

	actors := runtime.StaticActors()
	var targetVID uint32
	for _, actor := range actors {
		if actor.Name == "PracticeMobMyShopDeadGuestDelayedFloorClose" {
			targetVID = uint32(actor.EntityID)
			break
		}
	}
	if targetVID == 0 {
		t.Fatalf("expected practice mob actor after import, got %+v", actors)
	}

	openOut, err := hostFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected MYSHOP open error: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, host.VID, 4, `unexpected accepted MYSHOP before dead-guest delayed floor-close: out=%d err=%v`)
	if queued := flushServerFrames(t, guestFlow); len(queued) != 1 {
		t.Fatalf("expected guest to receive one live SHOP_SIGN around-broadcast before dead-guest delayed floor, got %d", len(queued))
	}

	browseOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: host.VID})))
	if err != nil || len(browseOut) != 1 {
		t.Fatalf("unexpected guest browse before dead-guest delayed floor-close: out=%d err=%v", len(browseOut), err)
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, browseOut[0])); err != nil {
		t.Fatalf("decode guest browse SHOP START before dead-guest delayed floor-close: %v", err)
	}

	selectOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before dead-guest delayed floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before dead-guest delayed floor-close, got %d", len(selectOut))
	}

	attackOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before dead-guest delayed floor-close: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, first point-loss retaliation, and self damage-info before delayed floor, got %d frames", len(attackOut))
	}
	if queued := flushServerFrames(t, hostFlow); len(queued) == 0 {
		t.Fatal("expected host to receive live-hit retaliation fanout before delayed floor")
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, guestFlow)
	if len(queued) != 5 {
		t.Fatalf("expected delayed retaliation floor to queue point-loss, self dead, clear-target, owner damage-info, and guest SHOP END, got %d", len(queued))
	}
	next := assertOwnerFloorDeathSequence(t, queued, 0, guest.VID, bootstrapPracticeMobRetaliationPointDelta, "myshop_practice_mob_floor_close owner-floor")
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, queued[next])); err != nil {
		t.Fatalf("decode dead-guest delayed floor-close guest SHOP END: %v", err)
	}

	hostQueued := flushServerFrames(t, hostFlow)
	if len(hostQueued) != 2 {
		t.Fatalf("expected host to receive guest DEAD plus owner damage-info after dead-guest delayed floor, got %d", len(hostQueued))
	}
	remaining := assertOwnerFloorPeerDeadFanout(t, hostQueued, guest.VID, int32(-bootstrapPracticeMobRetaliationPointDelta), "myshop_practice_mob_floor_close owner-floor peer")
	if len(remaining) != 0 {
		t.Fatalf("expected no extra owner-floor peer frames after DEAD + damage-info, got %d", len(remaining))
	}

	secondEndOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected guest SHOP END after dead-guest delayed floor: %v", err)
	}
	if len(secondEndOut) != 0 {
		t.Fatalf("expected already-closed guest SHOP END after dead-guest delayed floor to emit no frames, got %d", len(secondEndOut))
	}
	rebrowseOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: host.VID})))
	if err != nil {
		t.Fatalf("unexpected post-delayed-floor dead guest ON_CLICK: %v", err)
	}
	if len(rebrowseOut) != 0 {
		t.Fatalf("expected post-delayed-floor dead guest ON_CLICK to emit no frames, got %d", len(rebrowseOut))
	}
	assertExchangeAccountUnchanged(t, accounts, hostLogin, characterAfterMyShopBagConsume(host), "myshop dead-guest delayed floor-close host")
	assertExchangeAccountUnchanged(t, accounts, guestLogin, guest, "myshop dead-guest delayed floor-close guest")
}

func TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenMyShopBeforeRestartTownExchange(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopTownImmediateOwner", 0x010308a1, 0x020408a1, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 0x8a1, Vnum: 27001, Count: 3, Slot: 5}, {ID: 2259, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	sourcePeer := peerVisibilityCharacter("MyShopTownImmediateSource", 0x010308a2, 0x020408a2, 1120, 2120, 0, 101, 201)
	townPeer := peerVisibilityCharacter("MyShopTownImmediateTown", 0x010308a3, 0x020408a3, 52070, 166600, 4, 103, 203)
	townPeer.MapIndex = 21
	login := "myshop-town-immediate-floor"
	loginKey := uint32(0x70708aa1)
	sourceLogin := "myshop-town-immediate-source"
	sourceLoginKey := uint32(0x70708aa2)
	townLogin := "myshop-town-immediate-town"
	townLoginKey := uint32(0x70708aa3)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, sourceLogin, sourceLoginKey, sourcePeer)
	issuePeerTicket(t, ticketStore, townLogin, townLoginKey, townPeer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop town immediate floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: sourceLogin, Empire: sourcePeer.Empire, Characters: cloneCharacters([]loginticket.Character{sourcePeer})}); err != nil {
		t.Fatalf("seed myshop town immediate floor-close source peer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: townLogin, Empire: townPeer.Empire, Characters: cloneCharacters([]loginticket.Character{townPeer})}); err != nil {
		t.Fatalf("seed myshop town immediate floor-close town peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Shop Potion",
		Stackable: true,
		MaxCount:  200,
	}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop town immediate floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000591, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_myshop_town_immediate_floor_close",
		Name:          "PracticeMobMyShopTownImmediateFloorClose",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import myshop town immediate floor-close content bundle: %v", err)
	}

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
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive source peer-entry frames before myshop town immediate floor-close, got %d", len(queued))
	}
	_ = flushServerFrames(t, sourceFlow)
	townFlow, townEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), townLogin, townLoginKey)
	if len(townEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for town peer on destination map, got %d", len(townEnter))
	}
	defer closeSessionFlow(t, townFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected destination-map town peer join to avoid queued owner frames before floor, got %d", len(queued))
	}
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 0 {
		t.Fatalf("expected destination-map town peer join to avoid queued source frames before floor, got %d", len(queued))
	}

	actors := runtime.StaticActors()
	var targetVID uint32
	for _, actor := range actors {
		if actor.Name == "PracticeMobMyShopTownImmediateFloorClose" {
			targetVID = uint32(actor.EntityID)
			break
		}
	}
	if targetVID == 0 {
		t.Fatalf("expected practice mob actor after import, got %+v", actors)
	}

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected accepted MYSHOP before town immediate floor-close: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `expected accepted MYSHOP before town immediate floor-close to emit one SHOP_SIGN frame, got %d`)
	liveSourceSign := flushServerFrames(t, sourceFlow)
	if len(liveSourceSign) != 1 {
		t.Fatalf("expected source peer to receive one live SHOP_SIGN around-broadcast before floor, got %d", len(liveSourceSign))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before myshop town immediate floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before myshop town immediate floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before myshop town immediate floor-close: %v", err)
	}
	if len(attackOut) != 6 {
		t.Fatalf("expected target refresh, point-loss, self dead, clear-target, owner damage-info, and empty MYSHOP SHOP_SIGN, got %d", len(attackOut))
	}
	next := assertOwnerFloorDeathSequence(t, attackOut, 1, owner.VID, bootstrapPracticeMobRetaliationPointDelta, "myshop_practice_mob_floor_close owner-floor")
	assertMyShopEmptySignFrame(t, attackOut[next], owner.VID, "myshop town immediate floor-close")

	sourceQueued := flushServerFrames(t, sourceFlow)
	if len(sourceQueued) != 3 {
		t.Fatalf("expected source peer DEAD, owner damage-info, plus empty MYSHOP SHOP_SIGN around-broadcast after immediate floor, got %d", len(sourceQueued))
	}
	remaining := assertOwnerFloorPeerDeadFanout(t, sourceQueued, owner.VID, int32(-bootstrapPracticeMobRetaliationPointDelta), "myshop_practice_mob_floor_close owner-floor peer")
	if len(remaining) != 1 {
		t.Fatalf("expected 1 extra owner-floor peer frame(s) after DEAD + damage-info, got %d leftover frames", len(remaining))
	}
	assertMyShopEmptySignFrame(t, remaining[0], owner.VID, "myshop town immediate floor-close source empty sign")

	alreadyClosedOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_myshop",
	})))
	if err != nil {
		t.Fatalf("unexpected already-closed /close_myshop after town immediate floor: %v", err)
	}
	if len(alreadyClosedOut) != 0 {
		t.Fatalf("expected already-closed /close_myshop after town immediate floor to emit no frames, got %d", len(alreadyClosedOut))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after myshop town floor: %v", err)
	}
	if len(restartOut) < 9 {
		t.Fatalf("expected at least 9 self frames from /restart_town recovery after myshop town floor, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after myshop town /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after myshop floor, got %+v", selfAdd)
	}
	var (
		selfPoints    worldproto.PlayerPointChangePacket
		foundPoints   bool
		foundTownPeer bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
				continue
			}
		}
		if add, err := worldproto.DecodeCharacterAdd(fr); err == nil && add.VID == townPeer.VID {
			foundTownPeer = true
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after myshop floor, got %+v", wantHP, selfPoints)
	}
	if !foundTownPeer {
		t.Fatalf("expected /restart_town destination visibility delta to add town peer vid %d", townPeer.VID)
	}
	_ = flushServerFrames(t, sourceFlow)
	townQueued := flushServerFrames(t, townFlow)
	if len(townQueued) != 3 {
		t.Fatalf("expected destination town peer to receive 3 queued owner re-entry frames after /restart_town, got %d", len(townQueued))
	}

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      townPeer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town exchange start after myshop immediate floor: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected post-restart_town exchange start to succeed after myshop immediate floor clear, got %d frames", len(startOut))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0])); err == nil {
		t.Fatalf("expected post-restart_town exchange start after myshop immediate floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, startOut[0], townPeer.VID, "post-restart_town exchange start after myshop immediate floor clear")
	townStart := flushServerFrames(t, townFlow)
	if len(townStart) != 1 {
		t.Fatalf("expected town peer exchange start after myshop immediate floor clear, got %d", len(townStart))
	}
	assertExchangeStartFrame(t, townStart[0], owner.VID, "town peer exchange start after myshop immediate floor clear")
	assertExchangeAccountUnchanged(t, accounts, login, characterAfterMyShopBagConsume(owner), "myshop town immediate floor close inventory/gold")
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted owner after myshop town immediate /restart_town: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner after myshop town immediate /restart_town, got %+v", persisted)
	}
	if persisted.Characters[0].MapIndex != 21 || persisted.Characters[0].X != 52070 || persisted.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after myshop floor, got %+v", persisted.Characters[0])
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after myshop floor, got %+v", wantHP, persisted.Characters[0])
	}
}

func TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenMyShopBeforeRestartTownExchange(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopTownDelayedOwner", 0x010308a4, 0x020408a4, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 0x8a4, Vnum: 27001, Count: 3, Slot: 5}, {ID: 2262, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	sourcePeer := peerVisibilityCharacter("MyShopTownDelayedSource", 0x010308a5, 0x020408a5, 1120, 2120, 0, 101, 201)
	townPeer := peerVisibilityCharacter("MyShopTownDelayedTown", 0x010308a6, 0x020408a6, 52070, 166600, 4, 103, 203)
	townPeer.MapIndex = 21
	login := "myshop-town-delayed-floor"
	loginKey := uint32(0x70708aa4)
	sourceLogin := "myshop-town-delayed-source"
	sourceLoginKey := uint32(0x70708aa5)
	townLogin := "myshop-town-delayed-town"
	townLoginKey := uint32(0x70708aa6)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, sourceLogin, sourceLoginKey, sourcePeer)
	issuePeerTicket(t, ticketStore, townLogin, townLoginKey, townPeer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop town delayed floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: sourceLogin, Empire: sourcePeer.Empire, Characters: cloneCharacters([]loginticket.Character{sourcePeer})}); err != nil {
		t.Fatalf("seed myshop town delayed floor-close source peer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: townLogin, Empire: townPeer.Empire, Characters: cloneCharacters([]loginticket.Character{townPeer})}); err != nil {
		t.Fatalf("seed myshop town delayed floor-close town peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Shop Potion",
		Stackable: true,
		MaxCount:  200,
	}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop town delayed floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000594, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_myshop_town_delayed_floor_close",
		Name:          "PracticeMobMyShopTownDelayedFloorClose",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import myshop town delayed floor-close content bundle: %v", err)
	}

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
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive source peer-entry frames before myshop town delayed floor-close, got %d", len(queued))
	}
	_ = flushServerFrames(t, sourceFlow)
	townFlow, townEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), townLogin, townLoginKey)
	if len(townEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for town peer on destination map, got %d", len(townEnter))
	}
	defer closeSessionFlow(t, townFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected destination-map town peer join to avoid queued owner frames before delayed floor, got %d", len(queued))
	}
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 0 {
		t.Fatalf("expected destination-map town peer join to avoid queued source frames before delayed floor, got %d", len(queued))
	}

	actors := runtime.StaticActors()
	var targetVID uint32
	for _, actor := range actors {
		if actor.Name == "PracticeMobMyShopTownDelayedFloorClose" {
			targetVID = uint32(actor.EntityID)
			break
		}
	}
	if targetVID == 0 {
		t.Fatalf("expected practice mob actor after import, got %+v", actors)
	}

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected accepted MYSHOP before town delayed floor-close: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `expected accepted MYSHOP before town delayed floor-close to emit one SHOP_SIGN frame, got %d`)
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 1 {
		t.Fatalf("expected source peer to receive one live SHOP_SIGN around-broadcast before delayed floor, got %d", len(queued))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before myshop town delayed floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before myshop town delayed floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before myshop town delayed floor-close: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, first point-loss retaliation, and self damage-info before delayed floor, got %d frames", len(attackOut))
	}
	if queued := flushServerFrames(t, sourceFlow); len(queued) == 0 {
		t.Fatal("expected source peer to receive live-hit retaliation fanout before delayed floor")
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, ownerFlow)
	if len(queued) != 5 {
		t.Fatalf("expected delayed retaliation floor to queue point-loss, self dead, clear-target, owner damage-info, and empty MYSHOP SHOP_SIGN, got %d", len(queued))
	}
	next := assertOwnerFloorDeathSequence(t, queued, 0, owner.VID, bootstrapPracticeMobRetaliationPointDelta, "myshop_practice_mob_floor_close owner-floor")
	assertMyShopEmptySignFrame(t, queued[next], owner.VID, "myshop town delayed floor-close")

	sourceQueued := flushServerFrames(t, sourceFlow)
	if len(sourceQueued) != 3 {
		t.Fatalf("expected source peer DEAD, owner damage-info, plus empty MYSHOP SHOP_SIGN around-broadcast after delayed floor, got %d", len(sourceQueued))
	}
	remaining := assertOwnerFloorPeerDeadFanout(t, sourceQueued, owner.VID, int32(-bootstrapPracticeMobRetaliationPointDelta), "myshop_practice_mob_floor_close owner-floor peer")
	if len(remaining) != 1 {
		t.Fatalf("expected 1 extra owner-floor peer frame(s) after DEAD + damage-info, got %d leftover frames", len(remaining))
	}
	assertMyShopEmptySignFrame(t, remaining[0], owner.VID, "myshop town delayed floor-close source empty sign")

	alreadyClosedOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_myshop",
	})))
	if err != nil {
		t.Fatalf("unexpected already-closed /close_myshop after town delayed floor: %v", err)
	}
	if len(alreadyClosedOut) != 0 {
		t.Fatalf("expected already-closed /close_myshop after town delayed floor to emit no frames, got %d", len(alreadyClosedOut))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after myshop town floor: %v", err)
	}
	if len(restartOut) < 9 {
		t.Fatalf("expected at least 9 self frames from /restart_town recovery after myshop town floor, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after myshop town /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after myshop floor, got %+v", selfAdd)
	}
	var (
		selfPoints    worldproto.PlayerPointChangePacket
		foundPoints   bool
		foundTownPeer bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
				continue
			}
		}
		if add, err := worldproto.DecodeCharacterAdd(fr); err == nil && add.VID == townPeer.VID {
			foundTownPeer = true
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after myshop floor, got %+v", wantHP, selfPoints)
	}
	if !foundTownPeer {
		t.Fatalf("expected /restart_town destination visibility delta to add town peer vid %d", townPeer.VID)
	}
	_ = flushServerFrames(t, sourceFlow)
	townQueued := flushServerFrames(t, townFlow)
	if len(townQueued) != 3 {
		t.Fatalf("expected destination town peer to receive 3 queued owner re-entry frames after /restart_town, got %d", len(townQueued))
	}

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      townPeer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town exchange start after myshop delayed floor: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected post-restart_town exchange start to succeed after myshop delayed floor clear, got %d frames", len(startOut))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0])); err == nil {
		t.Fatalf("expected post-restart_town exchange start after myshop delayed floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, startOut[0], townPeer.VID, "post-restart_town exchange start after myshop delayed floor clear")
	townStart := flushServerFrames(t, townFlow)
	if len(townStart) != 1 {
		t.Fatalf("expected town peer exchange start after myshop delayed floor clear, got %d", len(townStart))
	}
	assertExchangeStartFrame(t, townStart[0], owner.VID, "town peer exchange start after myshop delayed floor clear")
	assertExchangeAccountUnchanged(t, accounts, login, characterAfterMyShopBagConsume(owner), "myshop town delayed floor close inventory/gold")
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted owner after myshop town delayed /restart_town: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner after myshop town delayed /restart_town, got %+v", persisted)
	}
	if persisted.Characters[0].MapIndex != 21 || persisted.Characters[0].X != 52070 || persisted.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after myshop delayed floor, got %+v", persisted.Characters[0])
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after myshop delayed floor, got %+v", wantHP, persisted.Characters[0])
	}
}
