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
	if len(openOut) != 1 {
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
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, point-change, self dead, and clear-target with no extra safebox frames, got %d", len(attackOut))
	}
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
