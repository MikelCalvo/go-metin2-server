package minimal

import (
	"path/filepath"
	"reflect"
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

func TestGameSessionFlowPostFloorSafeboxCheckoutFailsClosed(t *testing.T) {
	root := t.TempDir()
	ticketStore := loginticket.NewFileStore(filepath.Join(root, "tickets"))
	accounts := accountstore.NewFileStore(filepath.Join(root, "accounts"))
	owner := peerVisibilityCharacter("DeadCheckoutOwner", 0x01030a80, 0x02040a80, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 881, Vnum: 27001, Count: 2, Slot: 5}}
	login := "post-floor-safe-checkout"
	loginKey := uint32(0x19191a80)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor safebox checkout account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{
		LegacyAddr:       ":13000",
		PublicAddr:       "127.0.0.1",
		SafeboxStorePath: filepath.Join(root, "safebox", "safebox.json"),
	}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected post-floor safebox checkout runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_safe_checkout",
		Name:          "PracticeMobPostFloorSafeCheckout",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import post-floor safebox checkout practice mob: %v", err)
	}
	if err := runtime.replaceItemTemplates(itemcatalog.Snapshot{Templates: []itemcatalog.Template{template}}); err != nil {
		t.Fatalf("restore post-floor safebox checkout templates after content import: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor safebox checkout practice mob, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox before post-floor checkout seed: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected /open_safebox before post-floor checkout seed to emit SAFEBOX_SIZE + money, got %d", len(openOut))
	}
	checkinOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected SAFEBOX_CHECKIN before post-floor checkout seed: %v", err)
	}
	if len(checkinOut) < 2 {
		t.Fatalf("expected SAFEBOX_CHECKIN before post-floor checkout seed to emit ITEM_DEL + SAFEBOX_SET, got %d", len(checkinOut))
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection before post-floor safebox checkout: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected one target-selection frame before post-floor safebox checkout, got %d", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack before post-floor safebox checkout: %v", err)
	}
	if len(attackOut) < 5 {
		t.Fatalf("expected floor attack to include CloseSafebox companion before post-floor safebox checkout, got %d frames", len(attackOut))
	}
	closeChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, attackOut[len(attackOut)-1]))
	if err != nil || closeChat.Type != chatproto.ChatTypeCommand || closeChat.Message != "CloseSafebox" {
		t.Fatalf("expected floor attack to append CloseSafebox before post-floor safebox checkout, got %#v err=%v", attackOut[len(attackOut)-1], err)
	}

	checkoutPacket := itemproto.EncodeClientSafeboxCheckout(itemproto.ClientSafeboxCheckoutPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(7),
	})
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, checkoutPacket))
	if err != nil {
		t.Fatalf("unexpected post-floor SAFEBOX_CHECKOUT dispatch error: %v", err)
	}
	if len(buyOut) != 0 {
		t.Fatalf("expected post-floor SAFEBOX_CHECKOUT to fail closed with no frames, got %d", len(buyOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor SAFEBOX_CHECKOUT to queue no frames, got %d", len(queued))
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-floor SAFEBOX_CHECKOUT: %v", err)
	}
	if account.Characters[0].Gold != 4242 || len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected post-floor SAFEBOX_CHECKOUT to leave carried inventory empty and gold unchanged, got %#v", account.Characters[0])
	}

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor SAFEBOX_CHECKOUT: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor SAFEBOX_CHECKOUT, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart /open_safebox: %v", err)
	}
	if len(reopenOut) != 3 {
		t.Fatalf("expected SAFEBOX_SIZE + remembered SAFEBOX_SET + money after restart, got %d", len(reopenOut))
	}
	reopenSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode post-restart SAFEBOX_SET: %v", err)
	}
	if reopenSet.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) || reopenSet.Vnum != 27001 || reopenSet.Count != 2 {
		t.Fatalf("unexpected post-restart SAFEBOX_SET: %+v", reopenSet)
	}

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, checkoutPacket))
	if err != nil {
		t.Fatalf("unexpected post-restart SAFEBOX_CHECKOUT: %v", err)
	}
	assertPostFloorSafeboxCheckoutSuccessBurst(t, reuseOut, "post-restart SAFEBOX_CHECKOUT")
	account, err = accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart SAFEBOX_CHECKOUT: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after SAFEBOX_CHECKOUT floor, got %+v", wantHP, account.Characters[0])
	}
	if account.Characters[0].Gold != 4242 || !reflect.DeepEqual(account.Characters[0].Inventory, []inventory.ItemInstance{{ID: 881, Vnum: 27001, Count: 2, Slot: 7}}) {
		t.Fatalf("expected post-restart SAFEBOX_CHECKOUT to persist returned stack in slot 7, got %#v", account.Characters[0])
	}
}

func TestGameSessionFlowPostFloorSafeboxCheckoutFailsClosedBeforeRestartTown(t *testing.T) {
	root := t.TempDir()
	ticketStore := loginticket.NewFileStore(filepath.Join(root, "tickets"))
	accounts := accountstore.NewFileStore(filepath.Join(root, "accounts"))
	owner := peerVisibilityCharacter("DeadCheckoutTownOwner", 0x01030a81, 0x02040a81, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 4343
	owner.Inventory = []inventory.ItemInstance{{ID: 882, Vnum: 27001, Count: 2, Slot: 5}}
	login := "pf-safe-checkout-town"
	loginKey := uint32(0x19191a81)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor safebox checkout town account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{
		LegacyAddr:       ":13000",
		PublicAddr:       "127.0.0.1",
		SafeboxStorePath: filepath.Join(root, "safebox", "safebox.json"),
	}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected post-floor safebox checkout town runtime error: %v", err)
	}
	currentTime := time.Unix(1700000470, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_safe_checkout_town",
		Name:          "PracticeMobPostFloorSafeCheckoutTown",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import post-floor safebox checkout town practice mob: %v", err)
	}
	if err := runtime.replaceItemTemplates(itemcatalog.Snapshot{Templates: []itemcatalog.Template{template}}); err != nil {
		t.Fatalf("restore post-floor safebox checkout town templates after content import: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor safebox checkout town practice mob, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before post-floor town checkout seed: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	}))); err != nil {
		t.Fatalf("unexpected SAFEBOX_CHECKIN before post-floor town checkout seed: %v", err)
	}

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil {
		t.Fatalf("unexpected target selection before post-floor town safebox checkout: %v", err)
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack before post-floor town safebox checkout: %v", err)
	}
	if len(attackOut) < 5 {
		t.Fatalf("expected floor attack to include CloseSafebox companion before post-floor town safebox checkout, got %d frames", len(attackOut))
	}

	checkoutPacket := itemproto.EncodeClientSafeboxCheckout(itemproto.ClientSafeboxCheckoutPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(7),
	})
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, checkoutPacket))
	if err != nil {
		t.Fatalf("unexpected post-floor town SAFEBOX_CHECKOUT dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town SAFEBOX_CHECKOUT to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town SAFEBOX_CHECKOUT to queue no frames, got %d", len(queued))
	}
	wantFloor := owner
	wantFloor.Inventory = nil
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, wantFloor, "post-floor town SAFEBOX_CHECKOUT")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor SAFEBOX_CHECKOUT: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor SAFEBOX_CHECKOUT, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor SAFEBOX_CHECKOUT /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after SAFEBOX_CHECKOUT floor, got %+v", selfAdd)
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
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after SAFEBOX_CHECKOUT floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after SAFEBOX_CHECKOUT floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	// Floor CloseSafebox arms the owned reopen cooldown; wait it out before town reopen.
	currentTime = currentTime.Add(11 * time.Second)
	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town /open_safebox: %v", err)
	}
	if len(reopenOut) != 3 {
		t.Fatalf("expected SAFEBOX_SIZE + remembered SAFEBOX_SET + money after restart_town, got %d", len(reopenOut))
	}

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, checkoutPacket))
	if err != nil {
		t.Fatalf("unexpected post-restart_town SAFEBOX_CHECKOUT: %v", err)
	}
	assertPostFloorSafeboxCheckoutSuccessBurst(t, reuseOut, "post-restart_town SAFEBOX_CHECKOUT")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town SAFEBOX_CHECKOUT: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after SAFEBOX_CHECKOUT floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town + SAFEBOX_CHECKOUT to leave recovered HP %d unchanged, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Inventory = []inventory.ItemInstance{{ID: 882, Vnum: 27001, Count: 2, Slot: 7}}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town SAFEBOX_CHECKOUT persists checkout")
}

func assertPostFloorSafeboxCheckoutSuccessBurst(t *testing.T, frames [][]byte, context string) {
	t.Helper()
	if len(frames) != 2 {
		t.Fatalf("expected %s to emit SAFEBOX_DEL + ITEM_SET, got %d", context, len(frames))
	}
	del, err := itemproto.DecodeSafeboxDel(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode %s SAFEBOX_DEL: %v", context, err)
	}
	if del.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) {
		t.Fatalf("unexpected %s SAFEBOX_DEL: %+v", context, del.Position)
	}
	set, err := itemproto.DecodeSet(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s ITEM_SET: %v", context, err)
	}
	if set.Position != itemproto.InventoryPosition(7) || set.Vnum != 27001 || set.Count != 2 {
		t.Fatalf("unexpected %s ITEM_SET: %+v", context, set)
	}
}
