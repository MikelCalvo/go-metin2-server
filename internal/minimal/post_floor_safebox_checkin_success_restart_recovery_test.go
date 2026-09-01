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

func TestGameSessionFlowPostFloorSafeboxCheckinSucceedsAfterRestartHere(t *testing.T) {
	root := t.TempDir()
	ticketStore := loginticket.NewFileStore(filepath.Join(root, "tickets"))
	accounts := accountstore.NewFileStore(filepath.Join(root, "accounts"))
	owner := peerVisibilityCharacter("DeadCheckinSuccessOwner", 0x01030a84, 0x02040a84, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 4747
	owner.Inventory = []inventory.ItemInstance{{ID: 885, Vnum: 27001, Count: 2, Slot: 5}}
	login := "post-floor-safe-checkin-ok"
	loginKey := uint32(0x19191a84)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor safebox check-in success account: %v", err)
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
		t.Fatalf("unexpected post-floor safebox check-in success runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_safe_checkin_ok",
		Name:          "PracticeMobPostFloorSafeCheckinOk",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import post-floor safebox check-in success practice mob: %v", err)
	}
	if err := runtime.replaceItemTemplates(itemcatalog.Snapshot{Templates: []itemcatalog.Template{template}}); err != nil {
		t.Fatalf("restore post-floor safebox check-in success templates after content import: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor safebox check-in success practice mob, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection before post-floor safebox check-in success: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected one target-selection frame before post-floor safebox check-in success, got %d", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack before post-floor safebox check-in success: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, point-change, self dead, and clear-target frames at HP floor, got %d frames", len(attackOut))
	}

	checkinPacket := itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	})
	checkinOut, err := flow.HandleClientFrame(decodeSingleFrame(t, checkinPacket))
	if err != nil {
		t.Fatalf("unexpected post-floor SAFEBOX_CHECKIN dispatch error: %v", err)
	}
	if len(checkinOut) != 0 {
		t.Fatalf("expected post-floor SAFEBOX_CHECKIN to fail closed with no frames, got %d", len(checkinOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor SAFEBOX_CHECKIN to queue no frames, got %d", len(queued))
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-floor SAFEBOX_CHECKIN: %v", err)
	}
	if account.Characters[0].Gold != 4747 || !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected post-floor SAFEBOX_CHECKIN to leave carried inventory unchanged, got %#v", account.Characters[0])
	}

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor SAFEBOX_CHECKIN: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor SAFEBOX_CHECKIN, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart /open_safebox: %v", err)
	}
	if len(reopenOut) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE after restart, got %d", len(reopenOut))
	}

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, checkinPacket))
	if err != nil {
		t.Fatalf("unexpected post-restart SAFEBOX_CHECKIN: %v", err)
	}
	assertPostFloorSafeboxCheckinSuccessBurst(t, reuseOut, "post-restart SAFEBOX_CHECKIN")
	account, err = accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart SAFEBOX_CHECKIN: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after SAFEBOX_CHECKIN floor, got %+v", wantHP, account.Characters[0])
	}
	if account.Characters[0].Gold != 4747 || len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected post-restart SAFEBOX_CHECKIN to persist empty carried inventory, got %#v", account.Characters[0])
	}
}

func TestGameSessionFlowPostFloorSafeboxCheckinSucceedsAfterRestartTown(t *testing.T) {
	root := t.TempDir()
	ticketStore := loginticket.NewFileStore(filepath.Join(root, "tickets"))
	accounts := accountstore.NewFileStore(filepath.Join(root, "accounts"))
	owner := peerVisibilityCharacter("DeadCheckinSuccessTownOwner", 0x01030a85, 0x02040a85, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 4848
	owner.Inventory = []inventory.ItemInstance{{ID: 886, Vnum: 27001, Count: 2, Slot: 5}}
	login := "pf-safe-checkin-ok-town"
	loginKey := uint32(0x19191a85)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor safebox check-in success town account: %v", err)
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
		t.Fatalf("unexpected post-floor safebox check-in success town runtime error: %v", err)
	}
	currentTime := time.Unix(1700000470, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_safe_checkin_ok_town",
		Name:          "PracticeMobPostFloorSafeCheckinOkTown",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import post-floor safebox check-in success town practice mob: %v", err)
	}
	if err := runtime.replaceItemTemplates(itemcatalog.Snapshot{Templates: []itemcatalog.Template{template}}); err != nil {
		t.Fatalf("restore post-floor safebox check-in success town templates after content import: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor safebox check-in success town practice mob, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil {
		t.Fatalf("unexpected target selection before post-floor town safebox check-in success: %v", err)
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack before post-floor town safebox check-in success: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected town floor attack frames at HP floor, got %d", len(attackOut))
	}

	checkinPacket := itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	})
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, checkinPacket))
	if err != nil {
		t.Fatalf("unexpected post-floor town SAFEBOX_CHECKIN dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town SAFEBOX_CHECKIN to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town SAFEBOX_CHECKIN to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town SAFEBOX_CHECKIN")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor SAFEBOX_CHECKIN: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor SAFEBOX_CHECKIN, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor SAFEBOX_CHECKIN /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after SAFEBOX_CHECKIN floor, got %+v", selfAdd)
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
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after SAFEBOX_CHECKIN floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after SAFEBOX_CHECKIN floor, got %+v", wantHP, selfPoints)
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
	if len(reopenOut) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE after restart_town, got %d", len(reopenOut))
	}

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, checkinPacket))
	if err != nil {
		t.Fatalf("unexpected post-restart_town SAFEBOX_CHECKIN: %v", err)
	}
	assertPostFloorSafeboxCheckinSuccessBurst(t, reuseOut, "post-restart_town SAFEBOX_CHECKIN")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town SAFEBOX_CHECKIN: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after SAFEBOX_CHECKIN floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town + SAFEBOX_CHECKIN to leave recovered HP %d unchanged, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Inventory = nil
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town SAFEBOX_CHECKIN persists empty carried inventory")
}

func assertPostFloorSafeboxCheckinSuccessBurst(t *testing.T, frames [][]byte, context string) {
	t.Helper()
	if len(frames) != 2 {
		t.Fatalf("expected %s to emit ITEM_DEL + SAFEBOX_SET, got %d", context, len(frames))
	}
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode %s ITEM_DEL: %v", context, err)
	}
	if del.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected %s ITEM_DEL: %+v", context, del)
	}
	set, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s SAFEBOX_SET: %v", context, err)
	}
	if set.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) || set.Vnum != 27001 || set.Count != 2 {
		t.Fatalf("unexpected %s SAFEBOX_SET: %+v", context, set)
	}
}
