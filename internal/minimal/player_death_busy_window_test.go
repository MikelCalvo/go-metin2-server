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
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPracticeMobDeathClearsOpenSafeboxBusyBeforeRestartExchange(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxDeathOwner", 0x01030a10, 0x02040a10, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 125
	partner := peerVisibilityCharacter("SafeboxDeathPartner", 0x01030a11, 0x02040a11, 1120, 2120, 0, 101, 201)
	partner.Gold = 22222
	issuePeerTicket(t, store, "safebox-death-owner", 0x1a1a1a10, owner)
	issuePeerTicket(t, store, "safebox-death-partner", 0x1a1a1a11, partner)
	if err := accounts.Save(accountstore.Account{Login: "safebox-death-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox-death owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "safebox-death-partner", Empire: partner.Empire, Characters: cloneCharacters([]loginticket.Character{partner})}); err != nil {
		t.Fatalf("seed safebox-death partner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected safebox-death runtime error: %v", err)
	}
	currentTime := time.Unix(1700000610, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_safebox_floor_clear",
		Name:          "PracticeMobSafeboxFloorClear",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import safebox-death content bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one safebox-death practice mob, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "safebox-death-owner", 0x1a1a1a10)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 owner bootstrap frames with visible practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	partnerFlow, partnerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "safebox-death-partner", 0x1a1a1a11)
	if len(partnerEnter) != 11 {
		t.Fatalf("expected 11 partner bootstrap frames with visible owner and mob, got %d", len(partnerEnter))
	}
	defer closeSessionFlow(t, partnerFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive partner peer-entry frames before safebox death test, got %d", len(queued))
	}
	if queued := flushServerFrames(t, partnerFlow); len(queued) != 0 {
		t.Fatalf("expected no initial partner queued frames before safebox death test, got %d", len(queued))
	}

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox before death: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected /open_safebox before death to emit one SAFEBOX_SIZE frame, got %d", len(openOut))
	}
	if _, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, openOut[0])); err != nil {
		t.Fatalf("decode /open_safebox SAFEBOX_SIZE before death: %v", err)
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection before safebox death: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected one target selection frame before safebox death, got %d", len(selectOut))
	}
	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected safebox-death attack: %v", err)
	}
	if len(attackOut) != 5 {
		t.Fatalf("expected target refresh, point-change, self dead, clear-target, and CloseSafebox command chat, got %d", len(attackOut))
	}
	assertCloseSafeboxCommandChatFrame(t, attackOut[4], "safebox-death floor")
	if queued := flushServerFrames(t, partnerFlow); len(queued) != 1 {
		t.Fatalf("expected partner visible DEAD after safebox-death floor, got %d", len(queued))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after safebox death: %v", err)
	}
	if len(restartOut) < 4 {
		t.Fatalf("expected /restart_here recovery frames after safebox death, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, partnerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      partner.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart exchange start after safebox death: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected post-restart exchange start to succeed after safebox death clear, got %d frames", len(startOut))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0])); err == nil {
		t.Fatalf("expected post-restart exchange start after safebox death clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, startOut[0], partner.VID, "post-restart exchange start after safebox death clear")
	partnerStart := flushServerFrames(t, partnerFlow)
	if len(partnerStart) != 1 {
		t.Fatalf("expected partner exchange start after safebox death clear, got %d", len(partnerStart))
	}
	assertExchangeStartFrame(t, partnerStart[0], owner.VID, "partner exchange start after safebox death clear")
}

func TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenSafebox(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxDelayedFloorOwner", 0x01030a12, 0x02040a12, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	owner.Gold = 125
	partner := peerVisibilityCharacter("SafeboxDelayedFloorPartner", 0x01030a13, 0x02040a13, 1120, 2120, 0, 101, 201)
	partner.Gold = 22222
	login := "safebox-delayed-floor"
	loginKey := uint32(0x1a1a1a12)
	issuePeerTicket(t, store, login, loginKey, owner)
	issuePeerTicket(t, store, "safebox-delayed-floor-partner", 0x1a1a1a13, partner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox delayed floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "safebox-delayed-floor-partner", Empire: partner.Empire, Characters: cloneCharacters([]loginticket.Character{partner})}); err != nil {
		t.Fatalf("seed safebox delayed floor-close partner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected safebox delayed floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000612, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_safebox_delayed_floor_close",
		Name:          "PracticeMobSafeboxDelayedFloorClose",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import safebox delayed floor-close content bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one safebox delayed floor-close practice mob, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 owner bootstrap frames with visible practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	partnerFlow, partnerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "safebox-delayed-floor-partner", 0x1a1a1a13)
	if len(partnerEnter) != 11 {
		t.Fatalf("expected 11 partner bootstrap frames with visible owner and mob, got %d", len(partnerEnter))
	}
	defer closeSessionFlow(t, partnerFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive partner peer-entry frames before safebox delayed floor-close, got %d", len(queued))
	}
	if queued := flushServerFrames(t, partnerFlow); len(queued) != 0 {
		t.Fatalf("expected no initial partner queued frames before safebox delayed floor-close, got %d", len(queued))
	}

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox before delayed floor-close: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected /open_safebox before delayed floor-close to emit SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE, got %d", len(openOut))
	}
	if _, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, openOut[0])); err != nil {
		t.Fatalf("decode /open_safebox SAFEBOX_SIZE before delayed floor-close: %v", err)
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection before safebox delayed floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected one target selection frame before safebox delayed floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack before safebox delayed floor-close: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, first point-loss retaliation, and self damage-info before delayed floor, got %d frames", len(attackOut))
	}
	if queued := flushServerFrames(t, partnerFlow); len(queued) == 0 {
		t.Fatal("expected partner to receive live-hit retaliation fanout before delayed floor")
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, ownerFlow)
	if len(queued) != 4 {
		t.Fatalf("expected delayed retaliation floor to queue point-loss, self dead, clear-target, and CloseSafebox, got %d", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode safebox delayed floor-close point-change: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected delayed retaliation floor to drop owner HP to 0, got %+v", pointChange)
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, queued[1]))
	if err != nil {
		t.Fatalf("decode safebox delayed floor-close self dead: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected safebox delayed floor-close DEAD for owner VID %d, got %+v", owner.VID, dead)
	}
	clear, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, queued[2]))
	if err != nil {
		t.Fatalf("decode safebox delayed floor-close clear target: %v", err)
	}
	if clear.TargetVID != 0 || clear.HPPercent != 0 {
		t.Fatalf("expected safebox delayed floor-close clear target, got %+v", clear)
	}
	assertCloseSafeboxCommandChatFrame(t, queued[3], "safebox delayed floor-close")

	partnerQueued := flushServerFrames(t, partnerFlow)
	if len(partnerQueued) != 1 {
		t.Fatalf("expected partner DEAD after safebox delayed floor, got %d", len(partnerQueued))
	}
	partnerDead, err := worldproto.DecodeDead(decodeSingleFrame(t, partnerQueued[0]))
	if err != nil {
		t.Fatalf("decode partner DEAD after safebox delayed floor-close: %v", err)
	}
	if partnerDead.VID != owner.VID {
		t.Fatalf("expected partner DEAD for owner VID %d, got %+v", owner.VID, partnerDead)
	}

	alreadyClosedOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected already-closed /close_safebox after delayed floor: %v", err)
	}
	if len(alreadyClosedOut) != 0 {
		t.Fatalf("expected already-closed /close_safebox after delayed floor to emit no frames, got %d", len(alreadyClosedOut))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after safebox delayed floor: %v", err)
	}
	if len(restartOut) < 4 {
		t.Fatalf("expected /restart_here recovery frames after safebox delayed floor, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, partnerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      partner.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart exchange start after safebox delayed floor: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected post-restart exchange start to succeed after safebox delayed floor clear, got %d frames", len(startOut))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0])); err == nil {
		t.Fatalf("expected post-restart exchange start after safebox delayed floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, startOut[0], partner.VID, "post-restart exchange start after safebox delayed floor clear")
	partnerStart := flushServerFrames(t, partnerFlow)
	if len(partnerStart) != 1 {
		t.Fatalf("expected partner exchange start after safebox delayed floor clear, got %d", len(partnerStart))
	}
	assertExchangeStartFrame(t, partnerStart[0], owner.VID, "partner exchange start after safebox delayed floor clear")
}

func TestGameSessionFlowPracticeMobDeathClearsOpenRefineBusyBeforeRestartExchange(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineDeathOwner", 0x01030a20, 0x02040a20, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 901, Vnum: 11240, Count: 1, Slot: 5}}
	partner := peerVisibilityCharacter("RefineDeathPartner", 0x01030a21, 0x02040a21, 1120, 2120, 0, 101, 201)
	partner.Gold = 22222
	issuePeerTicket(t, store, "refine-death-owner", 0x1a1a1a20, owner)
	issuePeerTicket(t, store, "refine-death-partner", 0x1a1a1a21, partner)
	if err := accounts.Save(accountstore.Account{Login: "refine-death-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed refine-death owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "refine-death-partner", Empire: partner.Empire, Characters: cloneCharacters([]loginticket.Character{partner})}); err != nil {
		t.Fatalf("seed refine-death partner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	sourceTemplate := itemcatalog.Template{
		Vnum:       11240,
		Name:       "Refine Death Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{
			ResultVnum:  11241,
			Cost:        1000,
			Probability: 75,
			Materials:   []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 1}},
		},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11241, Name: "Refine Death Result Blade", Stackable: false, MaxCount: 1}
	materialTemplate := itemcatalog.Template{Vnum: 27001, Name: "Refine Death Material", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{sourceTemplate, resultTemplate, materialTemplate})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected refine-death runtime error: %v", err)
	}
	currentTime := time.Unix(1700000620, 0)
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.registerStaticActorWithInteractionCombatProfileAndSpawnGroupRef(
		"PracticeMobRefineFloorClear",
		bootstrapMapIndex,
		1200,
		2200,
		101,
		"",
		"",
		string(worldruntime.StaticActorCombatProfileTrainingDummy),
		"practice.mob_refine_floor_clear",
	)
	if !ok {
		t.Fatal("expected refine-death practice mob registration to succeed")
	}
	targetVID := uint32(actor.EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "refine-death-owner", 0x1a1a1a20)
	if len(ownerEnter) != 9 {
		t.Fatalf("expected 9 owner bootstrap frames with inventory item plus visible practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	partnerFlow, partnerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "refine-death-partner", 0x1a1a1a21)
	if len(partnerEnter) != 11 {
		t.Fatalf("expected 11 partner bootstrap frames with visible owner and mob, got %d", len(partnerEnter))
	}
	defer closeSessionFlow(t, partnerFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive partner peer-entry frames before refine death test, got %d", len(queued))
	}
	if queued := flushServerFrames(t, partnerFlow); len(queued) != 0 {
		t.Fatalf("expected no initial partner queued frames before refine death test, got %d", len(queued))
	}

	previewOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected refine preview before death: %v", err)
	}
	if len(previewOut) != 1 {
		t.Fatalf("expected refine preview before death to emit one frame, got %d", len(previewOut))
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, previewOut[0])); err != nil {
		t.Fatalf("decode refine preview before death: %v", err)
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection before refine death: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected one target selection frame before refine death, got %d", len(selectOut))
	}
	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected refine-death attack: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, point-change, self dead, and clear-target with no extra refine frames, got %d", len(attackOut))
	}
	if queued := flushServerFrames(t, partnerFlow); len(queued) != 1 {
		t.Fatalf("expected partner visible DEAD after refine-death floor, got %d", len(queued))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after refine death: %v", err)
	}
	if len(restartOut) < 4 {
		t.Fatalf("expected /restart_here recovery frames after refine death, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, partnerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      partner.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart exchange start after refine death: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected post-restart exchange start to succeed after refine death clear, got %d frames", len(startOut))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0])); err == nil {
		t.Fatalf("expected post-restart exchange start after refine death clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, startOut[0], partner.VID, "post-restart exchange start after refine death clear")
	partnerStart := flushServerFrames(t, partnerFlow)
	if len(partnerStart) != 1 {
		t.Fatalf("expected partner exchange start after refine death clear, got %d", len(partnerStart))
	}
	assertExchangeStartFrame(t, partnerStart[0], owner.VID, "partner exchange start after refine death clear")
}

func TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenRefine(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineDelayedFloorOwner", 0x01030a22, 0x02040a22, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 902, Vnum: 11240, Count: 1, Slot: 5}}
	partner := peerVisibilityCharacter("RefineDelayedFloorPartner", 0x01030a23, 0x02040a23, 1120, 2120, 0, 101, 201)
	partner.Gold = 22222
	login := "refine-delayed-floor"
	loginKey := uint32(0x1a1a1a22)
	issuePeerTicket(t, store, login, loginKey, owner)
	issuePeerTicket(t, store, "refine-delayed-floor-partner", 0x1a1a1a23, partner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed refine delayed floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "refine-delayed-floor-partner", Empire: partner.Empire, Characters: cloneCharacters([]loginticket.Character{partner})}); err != nil {
		t.Fatalf("seed refine delayed floor-close partner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	sourceTemplate := itemcatalog.Template{
		Vnum:       11240,
		Name:       "Refine Delayed Floor Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{
			ResultVnum:  11241,
			Cost:        1000,
			Probability: 75,
			Materials:   []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 1}},
		},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11241, Name: "Refine Delayed Floor Result Blade", Stackable: false, MaxCount: 1}
	materialTemplate := itemcatalog.Template{Vnum: 27001, Name: "Refine Delayed Floor Material", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{sourceTemplate, resultTemplate, materialTemplate})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected refine delayed floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000622, 0)
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.registerStaticActorWithInteractionCombatProfileAndSpawnGroupRef(
		"PracticeMobRefineDelayedFloorClose",
		bootstrapMapIndex,
		1200,
		2200,
		101,
		"",
		"",
		string(worldruntime.StaticActorCombatProfileTrainingDummy),
		"practice.mob_refine_delayed_floor_close",
	)
	if !ok {
		t.Fatal("expected refine delayed floor-close practice mob registration to succeed")
	}
	targetVID := uint32(actor.EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) != 9 {
		t.Fatalf("expected 9 owner bootstrap frames with inventory item plus visible practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	partnerFlow, partnerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "refine-delayed-floor-partner", 0x1a1a1a23)
	if len(partnerEnter) != 11 {
		t.Fatalf("expected 11 partner bootstrap frames with visible owner and mob, got %d", len(partnerEnter))
	}
	defer closeSessionFlow(t, partnerFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive partner peer-entry frames before refine delayed floor-close, got %d", len(queued))
	}
	if queued := flushServerFrames(t, partnerFlow); len(queued) != 0 {
		t.Fatalf("expected no initial partner queued frames before refine delayed floor-close, got %d", len(queued))
	}

	previewOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected refine preview before delayed floor-close: %v", err)
	}
	if len(previewOut) != 1 {
		t.Fatalf("expected refine preview before delayed floor-close to emit one frame, got %d", len(previewOut))
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, previewOut[0])); err != nil {
		t.Fatalf("decode refine preview before delayed floor-close: %v", err)
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection before refine delayed floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected one target selection frame before refine delayed floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack before refine delayed floor-close: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, first point-loss retaliation, and self damage-info before delayed floor, got %d frames", len(attackOut))
	}
	if queued := flushServerFrames(t, partnerFlow); len(queued) == 0 {
		t.Fatal("expected partner to receive live-hit retaliation fanout before delayed floor")
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, ownerFlow)
	if len(queued) != 3 {
		t.Fatalf("expected delayed retaliation floor to queue point-loss, self dead, and clear-target with no extra refine frames, got %d", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode refine delayed floor-close point-change: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected delayed retaliation floor to drop owner HP to 0, got %+v", pointChange)
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, queued[1]))
	if err != nil {
		t.Fatalf("decode refine delayed floor-close self dead: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected refine delayed floor-close DEAD for owner VID %d, got %+v", owner.VID, dead)
	}
	clear, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, queued[2]))
	if err != nil {
		t.Fatalf("decode refine delayed floor-close clear target: %v", err)
	}
	if clear.TargetVID != 0 || clear.HPPercent != 0 {
		t.Fatalf("expected refine delayed floor-close clear target, got %+v", clear)
	}

	partnerQueued := flushServerFrames(t, partnerFlow)
	if len(partnerQueued) != 1 {
		t.Fatalf("expected partner DEAD after refine delayed floor, got %d", len(partnerQueued))
	}
	partnerDead, err := worldproto.DecodeDead(decodeSingleFrame(t, partnerQueued[0]))
	if err != nil {
		t.Fatalf("decode partner DEAD after refine delayed floor-close: %v", err)
	}
	if partnerDead.VID != owner.VID {
		t.Fatalf("expected partner DEAD for owner VID %d, got %+v", owner.VID, partnerDead)
	}

	alreadyClosedOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 255})))
	if err != nil {
		t.Fatalf("unexpected already-closed refine cancel after delayed floor: %v", err)
	}
	if len(alreadyClosedOut) != 0 {
		t.Fatalf("expected already-closed refine cancel after delayed floor to emit no frames, got %d", len(alreadyClosedOut))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after refine delayed floor: %v", err)
	}
	if len(restartOut) < 4 {
		t.Fatalf("expected /restart_here recovery frames after refine delayed floor, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, partnerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      partner.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart exchange start after refine delayed floor: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected post-restart exchange start to succeed after refine delayed floor clear, got %d frames", len(startOut))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0])); err == nil {
		t.Fatalf("expected post-restart exchange start after refine delayed floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, startOut[0], partner.VID, "post-restart exchange start after refine delayed floor clear")
	partnerStart := flushServerFrames(t, partnerFlow)
	if len(partnerStart) != 1 {
		t.Fatalf("expected partner exchange start after refine delayed floor clear, got %d", len(partnerStart))
	}
	assertExchangeStartFrame(t, partnerStart[0], owner.VID, "partner exchange start after refine delayed floor clear")
}
