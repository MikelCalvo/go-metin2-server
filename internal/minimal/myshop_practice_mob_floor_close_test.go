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
	owner.Inventory = []inventory.ItemInstance{{ID: 891, Vnum: 27001, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("MyShopImmediateFloorPeer", 0x01030892, 0x02040892, 1120, 2120, 0, 101, 201)
	login := "myshop-immediate-floor"
	loginKey := uint32(0x70708991)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, "myshop-immediate-floor-peer", 0x70708992, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop immediate floor-close owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Shop Potion",
		Stackable: true,
		MaxCount:  200,
	}})
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
	peerFlow, peerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "myshop-immediate-floor-peer", 0x70708992)
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
	if len(openOut) != 1 {
		t.Fatalf("expected accepted MYSHOP before immediate floor-close to emit one SHOP_SIGN frame, got %d", len(openOut))
	}
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
	if len(attackOut) != 5 {
		t.Fatalf("expected target refresh, point-loss, self dead, clear-target, and empty MYSHOP SHOP_SIGN, got %d", len(attackOut))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode myshop immediate floor-close point-change: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected immediate retaliation floor to drop owner HP to 0, got %+v", pointChange)
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, attackOut[2]))
	if err != nil {
		t.Fatalf("decode myshop immediate floor-close self dead: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected myshop immediate floor-close DEAD for owner VID %d, got %+v", owner.VID, dead)
	}
	clear, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[3]))
	if err != nil {
		t.Fatalf("decode myshop immediate floor-close clear target: %v", err)
	}
	if clear.TargetVID != 0 || clear.HPPercent != 0 {
		t.Fatalf("expected myshop immediate floor-close clear target, got %+v", clear)
	}
	assertMyShopEmptySignFrame(t, attackOut[4], owner.VID, "myshop immediate floor-close")

	peerQueued := flushServerFrames(t, peerFlow)
	if len(peerQueued) != 2 {
		t.Fatalf("expected peer DEAD plus empty MYSHOP SHOP_SIGN around-broadcast after immediate floor, got %d", len(peerQueued))
	}
	peerDead, err := worldproto.DecodeDead(decodeSingleFrame(t, peerQueued[0]))
	if err != nil {
		t.Fatalf("decode peer DEAD after myshop immediate floor-close: %v", err)
	}
	if peerDead.VID != owner.VID {
		t.Fatalf("expected peer DEAD for owner VID %d, got %+v", owner.VID, peerDead)
	}
	assertMyShopEmptySignFrame(t, peerQueued[1], owner.VID, "myshop immediate floor-close peer empty sign")

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
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop immediate floor close")
}

func TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenMyShop(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopDelayedFloorOwner", 0x01030893, 0x02040893, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 893, Vnum: 27001, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("MyShopDelayedFloorPeer", 0x01030894, 0x02040894, 1120, 2120, 0, 101, 201)
	login := "myshop-delayed-floor"
	loginKey := uint32(0x70708993)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, "myshop-delayed-floor-peer", 0x70708994, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop delayed floor-close owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Shop Potion",
		Stackable: true,
		MaxCount:  200,
	}})
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
	peerFlow, peerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "myshop-delayed-floor-peer", 0x70708994)
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
	if len(openOut) != 1 {
		t.Fatalf("expected accepted MYSHOP before delayed floor-close to emit one SHOP_SIGN frame, got %d", len(openOut))
	}
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
	if len(queued) != 4 {
		t.Fatalf("expected delayed retaliation floor to queue point-loss, self dead, clear-target, and empty MYSHOP SHOP_SIGN, got %d", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode myshop delayed floor-close point-change: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected delayed retaliation floor to drop owner HP to 0, got %+v", pointChange)
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, queued[1]))
	if err != nil {
		t.Fatalf("decode myshop delayed floor-close self dead: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected myshop delayed floor-close DEAD for owner VID %d, got %+v", owner.VID, dead)
	}
	clear, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, queued[2]))
	if err != nil {
		t.Fatalf("decode myshop delayed floor-close clear target: %v", err)
	}
	if clear.TargetVID != 0 || clear.HPPercent != 0 {
		t.Fatalf("expected myshop delayed floor-close clear target, got %+v", clear)
	}
	assertMyShopEmptySignFrame(t, queued[3], owner.VID, "myshop delayed floor-close")

	peerQueued := flushServerFrames(t, peerFlow)
	if len(peerQueued) != 2 {
		t.Fatalf("expected peer DEAD plus empty MYSHOP SHOP_SIGN around-broadcast after delayed floor, got %d", len(peerQueued))
	}
	peerDead, err := worldproto.DecodeDead(decodeSingleFrame(t, peerQueued[0]))
	if err != nil {
		t.Fatalf("decode peer DEAD after myshop delayed floor-close: %v", err)
	}
	if peerDead.VID != owner.VID {
		t.Fatalf("expected peer DEAD for owner VID %d, got %+v", owner.VID, peerDead)
	}
	assertMyShopEmptySignFrame(t, peerQueued[1], owner.VID, "myshop delayed floor-close peer empty sign")

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
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop delayed floor close")
}

func TestGameSessionFlowPracticeMobImmediateRetaliationFloorQueuesGuestBrowseShopEnd(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopGuestFloorHost", 0x01030895, 0x02040895, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 895, Vnum: 27001, Count: 3, Slot: 5}}
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
	}})
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
	if len(openOut) != 1 {
		t.Fatalf("expected accepted MYSHOP before guest floor-close to emit one SHOP_SIGN frame, got %d", len(openOut))
	}
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
	if len(attackOut) != 5 {
		t.Fatalf("expected target refresh, point-loss, self dead, clear-target, and empty MYSHOP SHOP_SIGN, got %d", len(attackOut))
	}
	assertMyShopEmptySignFrame(t, attackOut[4], owner.VID, "myshop guest floor-close host empty sign")

	guestQueued := flushServerFrames(t, guestFlow)
	if len(guestQueued) != 3 {
		t.Fatalf("expected guest DEAD, SHOP END, and empty MYSHOP SHOP_SIGN after host floor, got %d", len(guestQueued))
	}
	guestDead, err := worldproto.DecodeDead(decodeSingleFrame(t, guestQueued[0]))
	if err != nil {
		t.Fatalf("decode guest DEAD after myshop guest floor-close: %v", err)
	}
	if guestDead.VID != owner.VID {
		t.Fatalf("expected guest DEAD for host VID %d, got %+v", owner.VID, guestDead)
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, guestQueued[1])); err != nil {
		t.Fatalf("decode guest SHOP END after myshop guest floor-close: %v", err)
	}
	assertMyShopEmptySignFrame(t, guestQueued[2], owner.VID, "myshop guest floor-close guest empty sign")

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
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "myshop guest floor-close host")
	assertExchangeAccountUnchanged(t, accounts, guestLogin, guest, "myshop guest floor-close guest")
}

func TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesGuestBrowseOnDeadGuest(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	// Keep the host outside DefaultSpawnAggroRadius of the practice mob so proximity
	// engagement does not silently block the browsing guest's TARGET / ATTACK path.
	host := peerVisibilityCharacter("MyShopDeadGuestHost", 0x01030897, 0x02040897, 900, 1900, 0, 101, 201)
	host.Gold = 5000
	host.Inventory = []inventory.ItemInstance{{ID: 897, Vnum: 27001, Count: 3, Slot: 5}}
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
	}})
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
	if err != nil || len(openOut) != 1 {
		t.Fatalf("unexpected accepted MYSHOP before dead-guest floor-close: out=%d err=%v", len(openOut), err)
	}
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
	if len(attackOut) != 5 {
		t.Fatalf("expected target refresh, point-loss, self dead, clear-target, and guest SHOP END, got %d", len(attackOut))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode dead-guest floor-close point-change: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected immediate retaliation floor to drop guest HP to 0, got %+v", pointChange)
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, attackOut[2]))
	if err != nil {
		t.Fatalf("decode dead-guest floor-close self dead: %v", err)
	}
	if dead.VID != guest.VID {
		t.Fatalf("expected dead-guest floor-close DEAD for guest VID %d, got %+v", guest.VID, dead)
	}
	clear, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[3]))
	if err != nil {
		t.Fatalf("decode dead-guest floor-close clear target: %v", err)
	}
	if clear.TargetVID != 0 || clear.HPPercent != 0 {
		t.Fatalf("expected dead-guest floor-close clear target, got %+v", clear)
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, attackOut[4])); err != nil {
		t.Fatalf("decode dead-guest floor-close guest SHOP END: %v", err)
	}

	hostQueued := flushServerFrames(t, hostFlow)
	if len(hostQueued) != 1 {
		t.Fatalf("expected host to receive only guest DEAD after dead-guest floor, got %d", len(hostQueued))
	}
	hostDead, err := worldproto.DecodeDead(decodeSingleFrame(t, hostQueued[0]))
	if err != nil {
		t.Fatalf("decode host DEAD after dead-guest floor-close: %v", err)
	}
	if hostDead.VID != guest.VID {
		t.Fatalf("expected host DEAD for guest VID %d, got %+v", guest.VID, hostDead)
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
	assertExchangeAccountUnchanged(t, accounts, hostLogin, host, "myshop dead-guest floor-close host")
	assertExchangeAccountUnchanged(t, accounts, guestLogin, guest, "myshop dead-guest floor-close guest")
}
