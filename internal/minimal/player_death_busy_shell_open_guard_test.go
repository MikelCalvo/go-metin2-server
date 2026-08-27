package minimal

import (
	"path/filepath"
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
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPostFloorOpenSafeboxFailsClosed(t *testing.T) {
	login := "post-floor-open-safebox"
	loginKey := uint32(0x19191b10)
	owner := peerVisibilityCharacter("DeadOpenSafeboxOwner", 0x01030b10, 0x02040b10, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 12345
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, nil)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor /open_safebox dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor /open_safebox to fail closed with no SAFEBOX_SIZE frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor /open_safebox to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor /open_safebox")
}

func TestGameSessionFlowPostFloorSafeboxPasswordOpenFailsClosed(t *testing.T) {
	login := "pf-open-safebox-pwd"
	loginKey := uint32(0x19191b15)
	owner := peerVisibilityCharacter("DeadOpenSafeboxPwdOwner", 0x01030b15, 0x02040b15, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 12345
	runtime, accounts, targetVID, warehouseVID := newPostFloorSafeboxPasswordRuntime(t, login, loginKey, owner)
	currentTime := time.Unix(1700002715, 0)
	runtime.now = func() time.Time { return currentTime }
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(enterOut) < 8 {
		t.Fatalf("expected owner bootstrap with visible practice mob and warehouse, got %d frames", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	interactOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: warehouseVID})))
	if err != nil {
		t.Fatalf("unexpected warehouse interact before post-floor password open: %v", err)
	}
	if len(interactOut) != 1 {
		t.Fatalf("expected ShowMeSafeboxPassword before post-floor password open, got %d frames", len(interactOut))
	}
	prompt, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, interactOut[0]))
	if err != nil {
		t.Fatalf("decode ShowMeSafeboxPassword before post-floor password open: %v", err)
	}
	if prompt.Type != chatproto.ChatTypeCommand || prompt.Message != safeboxShowPasswordCommandMessage {
		t.Fatalf("unexpected password prompt before post-floor password open: %+v", prompt)
	}

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password " + safeboxstore.DefaultPassword,
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor /safebox_password dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor /safebox_password to fail closed with no SAFEBOX_SIZE frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor /safebox_password to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor /safebox_password")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after pending-password floor: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after pending-password floor, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	staleOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password " + safeboxstore.DefaultPassword,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart stale /safebox_password dispatch error: %v", err)
	}
	if len(staleOut) != 0 {
		t.Fatalf("expected post-restart stale /safebox_password to stay fail-closed until fresh warehouse interact, got %d frames", len(staleOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-restart stale /safebox_password to queue no frames, got %d", len(queued))
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown + time.Nanosecond)
	rechallengeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: warehouseVID})))
	if err != nil {
		t.Fatalf("unexpected warehouse re-interact after restart: %v", err)
	}
	if len(rechallengeOut) != 1 {
		t.Fatalf("expected fresh ShowMeSafeboxPassword after restart, got %d frames", len(rechallengeOut))
	}
	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password " + safeboxstore.DefaultPassword,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart fresh /safebox_password open: %v", err)
	}
	if len(openOut) < 2 {
		t.Fatalf("expected SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE after fresh post-restart password, got %d", len(openOut))
	}
	size, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, openOut[0]))
	if err != nil {
		t.Fatalf("decode SAFEBOX_SIZE after fresh post-restart password: %v", err)
	}
	if size != (itemproto.SafeboxSizePacket{Size: 2}) {
		t.Fatalf("unexpected SAFEBOX_SIZE after fresh post-restart password: %+v", size)
	}
}

func TestGameSessionFlowPostFloorSafeboxChangePasswordFailsClosed(t *testing.T) {
	login := "pf-change-safebox-pwd"
	loginKey := uint32(0x19191b16)
	owner := peerVisibilityCharacter("DeadChangeSafeboxPwdOwner", 0x01030b16, 0x02040b16, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 12345
	root := t.TempDir()
	safeboxPath := filepath.Join(root, "state", "safebox.json")
	runtime, accounts, targetVID := newPostFloorSafeboxChangePasswordRuntime(t, root, safeboxPath, login, loginKey, owner)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_change_password " + safeboxstore.DefaultPassword + " vault2",
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor /safebox_change_password dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor /safebox_change_password to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor /safebox_change_password to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor /safebox_change_password")

	snapshot, err := safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load durable safebox after post-floor change-password: %v", err)
	}
	if got := safeboxstore.CharacterPassword(snapshot, login, owner.ID); got != safeboxstore.DefaultPassword {
		t.Fatalf("post-floor /safebox_change_password mutated durable password: got %q want %q", got, safeboxstore.DefaultPassword)
	}

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor change-password: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor change-password, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	changeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_change_password " + safeboxstore.DefaultPassword + " vault2",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart /safebox_change_password: %v", err)
	}
	if len(changeOut) != 1 {
		t.Fatalf("expected one post-restart change-password success chat, got %d", len(changeOut))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, changeOut[0]))
	if err != nil {
		t.Fatalf("decode post-restart change-password success chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != safeboxPasswordChangedInfoMessage {
		t.Fatalf("unexpected post-restart change-password success chat: %+v", delivery)
	}
	snapshot, err = safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load durable safebox after post-restart change-password: %v", err)
	}
	if got := safeboxstore.CharacterPassword(snapshot, login, owner.ID); got != "vault2" {
		t.Fatalf("durable password after post-restart change=%q want vault2", got)
	}
}

func TestGameSessionFlowPostFloorSafeboxMoneyFailsClosed(t *testing.T) {
	login := "pf-safebox-money"
	loginKey := uint32(0x19191b17)
	owner := peerVisibilityCharacter("DeadSafeboxMoneyOwner", 0x01030b17, 0x02040b17, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	root := t.TempDir()
	safeboxPath := filepath.Join(root, "state", "safebox.json")
	runtime, accounts, targetVID := newPostFloorSafeboxChangePasswordRuntime(t, root, safeboxPath, login, loginKey, owner)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before post-floor money seed: %v", err)
	}
	saveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_money_save 1500",
	})))
	if err != nil {
		t.Fatalf("unexpected pre-floor /safebox_money_save: %v", err)
	}
	if len(saveOut) != 2 {
		t.Fatalf("expected gold PLAYER_POINT_CHANGE + SAFEBOX_MONEY_CHANGE before floor, got %d", len(saveOut))
	}
	preFloorGold, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, saveOut[0]))
	if err != nil {
		t.Fatalf("decode pre-floor gold PLAYER_POINT_CHANGE: %v", err)
	}
	if preFloorGold.VID != owner.VID || preFloorGold.Type != bootstrapGoldPointType || preFloorGold.Amount != -1500 || preFloorGold.Value != 3500 {
		t.Fatalf("unexpected pre-floor gold PLAYER_POINT_CHANGE: %+v", preFloorGold)
	}
	preFloorMoney, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, saveOut[1]))
	if err != nil {
		t.Fatalf("decode pre-floor SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if preFloorMoney != (itemproto.SafeboxMoneyChangePacket{Money: 1500}) {
		t.Fatalf("unexpected pre-floor SAFEBOX_MONEY_CHANGE: %+v", preFloorMoney)
	}
	owner.Gold = 3500
	assertCloseSafeboxCommandChat(t, flow, "/close_safebox", "pre-floor money close")

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor before money deny")
	snapshot, err := safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load durable safebox after floor before money deny: %v", err)
	}
	if got := safeboxstore.CharacterMoney(snapshot, login, owner.ID); got != 1500 {
		t.Fatalf("durable money after floor before money deny=%d want 1500", got)
	}

	for _, message := range []string{"/safebox_money_save 100", "/safebox_money_withdraw 100"} {
		out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
			Type:    chatproto.ChatTypeTalking,
			Message: message,
		})))
		if err != nil {
			t.Fatalf("unexpected post-floor %s dispatch error: %v", message, err)
		}
		if len(out) != 0 {
			t.Fatalf("expected post-floor %s to fail closed with no frames, got %d", message, len(out))
		}
		if queued := flushServerFrames(t, flow); len(queued) != 0 {
			t.Fatalf("expected post-floor %s to queue no frames, got %d", message, len(queued))
		}
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor safebox money")
	snapshot, err = safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load durable safebox after post-floor money deny: %v", err)
	}
	if got := safeboxstore.CharacterMoney(snapshot, login, owner.ID); got != 1500 {
		t.Fatalf("post-floor safebox money mutated durable gold: got %d want 1500", got)
	}

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor safebox money: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor safebox money, got %d", len(restartOut))
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
	reopenMoney, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode post-restart SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if reopenMoney != (itemproto.SafeboxMoneyChangePacket{Money: 1500}) {
		t.Fatalf("unexpected post-restart SAFEBOX_MONEY_CHANGE: %+v", reopenMoney)
	}

	withdrawOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_money_withdraw 500",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart /safebox_money_withdraw: %v", err)
	}
	if len(withdrawOut) != 2 {
		t.Fatalf("expected gold PLAYER_POINT_CHANGE + SAFEBOX_MONEY_CHANGE after restart withdraw, got %d", len(withdrawOut))
	}
	withdrawGold, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, withdrawOut[0]))
	if err != nil {
		t.Fatalf("decode post-restart withdraw gold PLAYER_POINT_CHANGE: %v", err)
	}
	if withdrawGold.VID != owner.VID || withdrawGold.Type != bootstrapGoldPointType || withdrawGold.Amount != 500 || withdrawGold.Value != 4000 {
		t.Fatalf("unexpected post-restart withdraw gold PLAYER_POINT_CHANGE: %+v", withdrawGold)
	}
	withdrawMoney, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, withdrawOut[1]))
	if err != nil {
		t.Fatalf("decode post-restart withdraw SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if withdrawMoney != (itemproto.SafeboxMoneyChangePacket{Money: 1000}) {
		t.Fatalf("unexpected post-restart withdraw SAFEBOX_MONEY_CHANGE: %+v", withdrawMoney)
	}
	snapshot, err = safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load durable safebox after post-restart withdraw: %v", err)
	}
	if got := safeboxstore.CharacterMoney(snapshot, login, owner.ID); got != 1000 {
		t.Fatalf("durable money after post-restart withdraw=%d want 1000", got)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart withdraw: %v", err)
	}
	if account.Characters[0].Gold != 4000 {
		t.Fatalf("carried gold after post-restart withdraw=%d want 4000", account.Characters[0].Gold)
	}
}

func TestGameSessionFlowPostFloorOpenCubeFailsClosed(t *testing.T) {
	login := "post-floor-open-cube"
	loginKey := uint32(0x19191b20)
	owner := peerVisibilityCharacter("DeadOpenCubeOwner", 0x01030b20, 0x02040b20, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 12345
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, nil)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor /open_cube dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor /open_cube to fail closed with no cube open command, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor /open_cube to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor /open_cube")
}

func TestGameSessionFlowPostFloorMyShopOpenFailsClosed(t *testing.T) {
	login := "post-floor-open-myshop"
	loginKey := uint32(0x19191b30)
	owner := peerVisibilityCharacter("DeadOpenMyShopOwner", 0x01030b30, 0x02040b30, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 931, Vnum: 27001, Count: 3, Slot: 5}}
	template := itemcatalog.Template{Vnum: 27001, Name: "Dead Guard Shop Potion", Stackable: true, MaxCount: 200}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, []itemcatalog.Template{template})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
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
		t.Fatalf("unexpected post-floor MYSHOP open dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor MYSHOP open to fail closed with no SHOP_SIGN, got %d frames", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor MYSHOP open to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor MYSHOP open")
}

func TestGameSessionFlowPostFloorRefinePreviewOpenFailsClosed(t *testing.T) {
	login := "post-floor-open-refine"
	loginKey := uint32(0x19191b35)
	owner := peerVisibilityCharacter("DeadOpenRefineOwner", 0x01030b35, 0x02040b35, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 935, Vnum: 11250, Count: 1, Slot: 5}}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11250,
		Name:       "Dead Guard Refine Preview Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{
			ResultVnum:  11251,
			Cost:        1000,
			Probability: 100,
			Materials:   []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 1}},
		},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11251, Name: "Dead Guard Refine Result Blade", Stackable: false, MaxCount: 1}
	materialTemplate := itemcatalog.Template{Vnum: 27001, Name: "Dead Guard Refine Material", Stackable: true, MaxCount: 200}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, []itemcatalog.Template{sourceTemplate, resultTemplate, materialTemplate})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected post-floor refineable REFINE preview dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor refineable REFINE preview to fail closed with no REFINE_INFORMATION_NEW, got %d frames", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor refineable REFINE preview to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor refineable REFINE preview")
}

func TestGameSessionFlowPostFloorExchangeStartFailsClosed(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DeadOpenExchangeOwner", 0x01030b40, 0x02040b40, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	peer := peerVisibilityCharacter("DeadOpenExchangePeer", 0x01030b41, 0x02040b41, 1120, 2120, 0, 101, 201)
	login := "pf-open-exchange"
	loginKey := uint32(0x19191b40)
	peerLogin := "pf-open-exchange-p"
	peerLoginKey := uint32(0x19191b41)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, peerLogin, peerLoginKey, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor exchange-open owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed post-floor exchange-open peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, newItemTemplateStore(t, nil), nil)
	if err != nil {
		t.Fatalf("unexpected post-floor exchange-open runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_exchange_open",
		Name:          "PracticeMobPostFloorExchangeOpen",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import post-floor exchange-open practice mob: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor exchange-open practice mob, got %#v", actors)
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
		t.Fatalf("expected owner to receive peer-entry frames before post-floor exchange open, got %d", len(queued))
	}
	_ = flushServerFrames(t, peerFlow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, ownerFlow, owner, targetVID)
	if queued := flushServerFrames(t, peerFlow); len(queued) != 1 {
		t.Fatalf("expected peer DEAD fanout after owner floor before exchange open, got %d", len(queued))
	}

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor EXCHANGE START dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor EXCHANGE START to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected post-floor EXCHANGE START to queue no owner frames, got %d", len(queued))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected post-floor EXCHANGE START to queue no peer frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor EXCHANGE START")
}

func TestGameSessionFlowPostFloorExchangeStartAgainstDeadPartnerFailsClosed(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DeadPartnerExchangeOwner", 0x01030b50, 0x02040b50, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	peer := peerVisibilityCharacter("DeadPartnerExchangePeer", 0x01030b51, 0x02040b51, 1120, 2120, 0, 101, 201)
	login := "pf-dead-partner-ex"
	loginKey := uint32(0x19191b50)
	peerLogin := "pf-dead-partner-ex-p"
	peerLoginKey := uint32(0x19191b51)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, peerLogin, peerLoginKey, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor dead-partner exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed post-floor dead-partner exchange peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, newItemTemplateStore(t, nil), nil)
	if err != nil {
		t.Fatalf("unexpected post-floor dead-partner exchange runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_dead_partner_exchange",
		Name:          "PracticeMobPostFloorDeadPartnerExchange",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import post-floor dead-partner exchange practice mob: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor dead-partner exchange practice mob, got %#v", actors)
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
		t.Fatalf("expected owner to receive peer-entry frames before dead-partner exchange, got %d", len(queued))
	}
	_ = flushServerFrames(t, peerFlow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, ownerFlow, owner, targetVID)
	if queued := flushServerFrames(t, peerFlow); len(queued) != 1 {
		t.Fatalf("expected peer DEAD fanout after owner floor before dead-partner exchange, got %d", len(queued))
	}

	out, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      owner.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected living-peer EXCHANGE START against dead partner dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected living-peer EXCHANGE START against dead partner to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected living-peer EXCHANGE START against dead partner to queue no peer frames, got %d", len(queued))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected living-peer EXCHANGE START against dead partner to queue no dead-owner frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "living-peer EXCHANGE START against dead partner")
}

func newPostFloorSafeboxPasswordRuntime(t *testing.T, login string, loginKey uint32, owner loginticket.Character) (*gameRuntime, accountstore.Store, uint32, uint32) {
	t.Helper()
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor safebox-password account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, nil)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected post-floor safebox-password runtime error: %v", err)
	}

	bundle := contentbundle.Bundle{
		InteractionDefinitions: []interactionstore.Definition{{
			Kind: interactionstore.KindOpenSafebox,
			Ref:  "npc:warehouse",
			Size: 2,
		}},
		StaticActors: []contentbundle.StaticActor{{
			Name:            "Warehouse",
			MapIndex:        bootstrapMapIndex,
			X:               1150,
			Y:               2150,
			RaceNum:         20300,
			InteractionKind: interactionstore.KindOpenSafebox,
			InteractionRef:  "npc:warehouse",
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.mob_post_floor_safebox_password",
			Name:          "PracticeMobPostFloorSafeboxPassword",
			MapIndex:      bootstrapMapIndex,
			X:             1200,
			Y:             2200,
			RaceNum:       101,
			CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
		}},
	}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import post-floor safebox-password content bundle: %v", err)
	}
	actors := runtime.StaticActors()
	var targetVID, warehouseVID uint32
	for _, actor := range actors {
		switch actor.Name {
		case "PracticeMobPostFloorSafeboxPassword":
			targetVID = uint32(actor.EntityID)
		case "Warehouse":
			warehouseVID = uint32(actor.EntityID)
		}
	}
	if targetVID == 0 || warehouseVID == 0 {
		t.Fatalf("expected practice mob and warehouse after post-floor safebox-password import, got %#v", actors)
	}
	return runtime, accounts, targetVID, warehouseVID
}

func newPostFloorSafeboxChangePasswordRuntime(t *testing.T, root, safeboxPath, login string, loginKey uint32, owner loginticket.Character) (*gameRuntime, accountstore.Store, uint32) {
	t.Helper()
	ticketStore := loginticket.NewFileStore(filepath.Join(root, "tickets"))
	accounts := accountstore.NewFileStore(filepath.Join(root, "accounts"))
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor safebox-change-password account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(filepath.Join(root, "static-actors.json"))
	interactionStore := interactionstore.NewFileStore(filepath.Join(root, "interaction-definitions.json"))
	itemStore := newItemTemplateStore(t, nil)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{
		LegacyAddr:       ":13000",
		PublicAddr:       "127.0.0.1",
		SafeboxStorePath: safeboxPath,
	}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected post-floor safebox-change-password runtime error: %v", err)
	}

	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_safebox_change_password",
		Name:          "PracticeMobPostFloorSafeboxChangePassword",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import post-floor safebox-change-password practice mob: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor safebox-change-password practice mob, got %#v", actors)
	}
	return runtime, accounts, uint32(actors[0].EntityID)
}
