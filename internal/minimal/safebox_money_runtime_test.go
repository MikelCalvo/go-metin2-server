package minimal

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
)

func TestGameSessionFlowSafeboxOpenBurstEmitsMoneyChangeDefaultZero(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxMoneyOpen", 0x01030921, 0x02040921, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	login := "safebox-money-open"
	const loginKey uint32 = 0x92929321
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected safebox money open runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox money open error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE, got %d", len(out))
	}
	size, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode SAFEBOX_SIZE: %v", err)
	}
	if size != (itemproto.SafeboxSizePacket{Size: 1}) {
		t.Fatalf("unexpected SAFEBOX_SIZE: %+v", size)
	}
	money, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if money != (itemproto.SafeboxMoneyChangePacket{Money: 0}) {
		t.Fatalf("unexpected SAFEBOX_MONEY_CHANGE: %+v", money)
	}
}

func TestGameSessionFlowSafeboxMoneySaveWithdrawAndReopenRematerialize(t *testing.T) {
	root := t.TempDir()
	ticketStore := loginticket.NewFileStore(filepath.Join(root, "tickets"))
	accounts := accountstore.NewFileStore(filepath.Join(root, "accounts"))
	safeboxPath := filepath.Join(root, "state", "safebox.json")
	owner := peerVisibilityCharacter("SafeboxMoneySave", 0x01030922, 0x02040922, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 922, Vnum: 27001, Count: 1, Slot: 2}}
	login := "safebox-money-save"
	const loginKey uint32 = 0x92929322
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox money account: %v", err)
	}
	cfg := config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", SafeboxStorePath: safeboxPath}
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(cfg, ticketStore, accounts, nil)
	if err != nil {
		t.Fatalf("unexpected safebox money save runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before money save: %v", err)
	}

	saveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_money_save 1500",
	})))
	if err != nil {
		t.Fatalf("unexpected /safebox_money_save error: %v", err)
	}
	if len(saveOut) != 2 {
		t.Fatalf("expected gold PLAYER_POINT_CHANGE + SAFEBOX_MONEY_CHANGE on save, got %d", len(saveOut))
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, saveOut[0]))
	if err != nil {
		t.Fatalf("decode save gold PLAYER_POINT_CHANGE: %v", err)
	}
	if goldChange.VID != owner.VID || goldChange.Type != bootstrapGoldPointType || goldChange.Amount != -1500 || goldChange.Value != 3500 {
		t.Fatalf("unexpected save gold PLAYER_POINT_CHANGE: %+v", goldChange)
	}
	moneyChange, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, saveOut[1]))
	if err != nil {
		t.Fatalf("decode save SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if moneyChange != (itemproto.SafeboxMoneyChangePacket{Money: 1500}) {
		t.Fatalf("unexpected save SAFEBOX_MONEY_CHANGE: %+v", moneyChange)
	}

	snapshot, err := safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load safebox after money save: %v", err)
	}
	if got := safeboxstore.CharacterMoney(snapshot, login, owner.ID); got != 1500 {
		t.Fatalf("durable money after save=%d want 1500", got)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after money save: %v", err)
	}
	if account.Characters[0].Gold != 3500 {
		t.Fatalf("carried gold after save=%d want 3500", account.Characters[0].Gold)
	}

	withdrawOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_money_withdraw 500",
	})))
	if err != nil {
		t.Fatalf("unexpected /safebox_money_withdraw error: %v", err)
	}
	if len(withdrawOut) != 2 {
		t.Fatalf("expected gold PLAYER_POINT_CHANGE + SAFEBOX_MONEY_CHANGE on withdraw, got %d", len(withdrawOut))
	}
	withdrawGold, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, withdrawOut[0]))
	if err != nil {
		t.Fatalf("decode withdraw gold PLAYER_POINT_CHANGE: %v", err)
	}
	if withdrawGold.VID != owner.VID || withdrawGold.Type != bootstrapGoldPointType || withdrawGold.Amount != 500 || withdrawGold.Value != 4000 {
		t.Fatalf("unexpected withdraw gold PLAYER_POINT_CHANGE: %+v", withdrawGold)
	}
	withdrawMoney, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, withdrawOut[1]))
	if err != nil {
		t.Fatalf("decode withdraw SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if withdrawMoney != (itemproto.SafeboxMoneyChangePacket{Money: 1000}) {
		t.Fatalf("unexpected withdraw SAFEBOX_MONEY_CHANGE: %+v", withdrawMoney)
	}

	assertCloseSafeboxCommandChat(t, flow, "/close_safebox", "money close before reopen")
	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox after money withdraw: %v", err)
	}
	if len(reopenOut) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE on reopen, got %d", len(reopenOut))
	}
	reopenMoney, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode reopen SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if reopenMoney != (itemproto.SafeboxMoneyChangePacket{Money: 1000}) {
		t.Fatalf("unexpected reopen SAFEBOX_MONEY_CHANGE: %+v", reopenMoney)
	}
}

func TestGameSessionFlowSafeboxMoneyRejectsClosedInsufficientAndOverflow(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxMoneyReject", 0x01030923, 0x02040923, 1100, 2100, 0, 101, 201)
	owner.Gold = 100
	login := "safebox-money-reject"
	const loginKey uint32 = 0x92929323
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected safebox money reject runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	closedOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_money_save 10",
	})))
	if err != nil {
		t.Fatalf("unexpected closed money save error: %v", err)
	}
	if len(closedOut) != 0 {
		t.Fatalf("expected closed money save to emit no frames, got %d", len(closedOut))
	}

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before reject cases: %v", err)
	}

	for _, msg := range []string{
		"/safebox_money_save",
		"/safebox_money_save 0",
		"/safebox_money_save -1",
		"/safebox_money_save abc",
		"/safebox_money_save 10 extra",
		"/safebox_money_save 101",
		"/safebox_money_withdraw 1",
		"/safebox_money_withdraw",
	} {
		out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
			Type:    chatproto.ChatTypeTalking,
			Message: msg,
		})))
		if err != nil {
			t.Fatalf("unexpected reject for %q: %v", msg, err)
		}
		if len(out) != 0 {
			t.Fatalf("expected fail-closed-consume for %q, got %d frames", msg, len(out))
		}
	}
}

func TestGameSessionFlowSafeboxMoneyPendingPasswordDoesNotEmitMoneyFrame(t *testing.T) {
	root := t.TempDir()
	ticketStore := loginticket.NewFileStore(filepath.Join(root, "tickets"))
	accounts := accountstore.NewFileStore(filepath.Join(root, "accounts"))
	safeboxPath := filepath.Join(root, "state", "safebox.json")
	owner := peerVisibilityCharacter("SafeboxMoneyPending", 0x01030924, 0x02040924, 1100, 2100, 0, 101, 201)
	owner.Gold = 2500
	login := "safebox-money-pending"
	const loginKey uint32 = 0x92929324
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed pending money account: %v", err)
	}
	if err := safeboxstore.NewFileStore(safeboxPath).Save(safeboxstore.Snapshot{Characters: []safeboxstore.CharacterRow{{
		Login: login, CharacterID: owner.ID, Money: 777,
	}}}); err != nil {
		t.Fatalf("seed durable warehouse money: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind: interactionstore.KindOpenSafebox,
		Ref:  "npc:warehouse",
		Size: 1,
	}})
	cfg := config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", SafeboxStorePath: safeboxPath}
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(cfg, ticketStore, accounts, interactionStore)
	if err != nil {
		t.Fatalf("unexpected pending money runtime error: %v", err)
	}
	currentTime := time.Unix(1700000924, 0)
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.RegisterStaticActorWithInteraction("Warehouse", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindOpenSafebox, "npc:warehouse")
	if !ok {
		t.Fatal("expected warehouse registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	interactOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected warehouse interact: %v", err)
	}
	for _, frame := range interactOut {
		if _, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, frame)); err == nil {
			t.Fatalf("pending password challenge must not emit SAFEBOX_MONEY_CHANGE")
		}
	}

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password 000000",
	})))
	if err != nil {
		t.Fatalf("unexpected password open: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE after password, got %d", len(openOut))
	}
	money, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, openOut[1]))
	if err != nil {
		t.Fatalf("decode password-open SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if money != (itemproto.SafeboxMoneyChangePacket{Money: 777}) {
		t.Fatalf("unexpected password-open SAFEBOX_MONEY_CHANGE: %+v", money)
	}
}

func TestGameSessionFlowSafeboxMoneySecondCharacterIsolated(t *testing.T) {
	root := t.TempDir()
	ticketStore := loginticket.NewFileStore(filepath.Join(root, "tickets"))
	accounts := accountstore.NewFileStore(filepath.Join(root, "accounts"))
	safeboxPath := filepath.Join(root, "state", "safebox.json")
	charA := peerVisibilityCharacter("MoneyCharA", 0x01030925, 0x02040925, 1100, 2100, 0, 101, 201)
	charA.Gold = 3000
	charB := peerVisibilityCharacter("MoneyCharB", 0x01030926, 0x02040926, 1200, 2200, 0, 102, 202)
	charB.Gold = 4000
	login := "safebox-money-isolation"
	const loginKey uint32 = 0x92929325
	if err := ticketStore.Issue(loginticket.Ticket{
		Login:      login,
		LoginKey:   loginKey,
		Empire:     charA.Empire,
		Characters: cloneCharacters([]loginticket.Character{charA, charB}),
	}); err != nil {
		t.Fatalf("issue multi-character ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{
		Login:      login,
		Empire:     charA.Empire,
		Characters: cloneCharacters([]loginticket.Character{charA, charB}),
	}); err != nil {
		t.Fatalf("seed isolation money account: %v", err)
	}
	cfg := config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", SafeboxStorePath: safeboxPath}
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(cfg, ticketStore, accounts, nil)
	if err != nil {
		t.Fatalf("unexpected isolation money runtime error: %v", err)
	}

	flowA := runtime.SessionFactory()()
	_ = mustCompleteSecureHandshake(t, flowA)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: login, LoginKey: loginKey})
	if err != nil {
		t.Fatalf("encode login2 for char A: %v", err)
	}
	if _, err := flowA.HandleClientFrame(decodeSingleFrame(t, login2Raw)); err != nil {
		t.Fatalf("login char A: %v", err)
	}
	if _, err := flowA.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 0}))); err != nil {
		t.Fatalf("select char A: %v", err)
	}
	if _, err := flowA.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("enter char A: %v", err)
	}
	_ = flushServerFrames(t, flowA)
	if _, err := flowA.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type: chatproto.ChatTypeTalking, Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("open char A: %v", err)
	}
	if _, err := flowA.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type: chatproto.ChatTypeTalking, Message: "/safebox_money_save 900",
	}))); err != nil {
		t.Fatalf("save char A money: %v", err)
	}
	closeSessionFlow(t, flowA)

	const loginKeyB uint32 = 0x92929326
	if err := ticketStore.Issue(loginticket.Ticket{
		Login:      login,
		LoginKey:   loginKeyB,
		Empire:     charA.Empire,
		Characters: cloneCharacters([]loginticket.Character{charA, charB}),
	}); err != nil {
		t.Fatalf("reissue ticket for char B: %v", err)
	}
	flowB := runtime.SessionFactory()()
	defer closeSessionFlow(t, flowB)
	_ = mustCompleteSecureHandshake(t, flowB)
	login2RawB, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: login, LoginKey: loginKeyB})
	if err != nil {
		t.Fatalf("encode login2 for char B: %v", err)
	}
	if _, err := flowB.HandleClientFrame(decodeSingleFrame(t, login2RawB)); err != nil {
		t.Fatalf("login char B: %v", err)
	}
	if _, err := flowB.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("select char B: %v", err)
	}
	if _, err := flowB.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("enter char B: %v", err)
	}
	_ = flushServerFrames(t, flowB)
	openB, err := flowB.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type: chatproto.ChatTypeTalking, Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("open char B: %v", err)
	}
	if len(openB) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE for char B, got %d", len(openB))
	}
	moneyB, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, openB[1]))
	if err != nil {
		t.Fatalf("decode char B SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if moneyB != (itemproto.SafeboxMoneyChangePacket{Money: 0}) {
		t.Fatalf("char B must not see char A warehouse money, got %+v", moneyB)
	}
}

func TestGameRuntimeSafeboxMoneySurvivesProcessRestartRematerializeOnOpen(t *testing.T) {
	root := t.TempDir()
	ticketDir := filepath.Join(root, "tickets")
	accountDir := filepath.Join(root, "accounts")
	safeboxPath := filepath.Join(root, "safebox", "safebox.json")
	ticketStore := loginticket.NewFileStore(ticketDir)
	accounts := accountstore.NewFileStore(accountDir)
	owner := peerVisibilityCharacter("SafeboxMoneyRestart", 0x01030925, 0x02040925, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	login := "safebox-money-restart"
	const loginKey uint32 = 0x92929325
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox money restart owner account: %v", err)
	}
	cfg := config.Service{
		LegacyAddr:       ":13000",
		PublicAddr:       "127.0.0.1",
		SafeboxStorePath: safeboxPath,
	}
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(cfg, ticketStore, accounts, nil)
	if err != nil {
		t.Fatalf("unexpected safebox money restart runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	_ = flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before money restart deposit: %v", err)
	}
	saveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_money_save 1750",
	})))
	if err != nil {
		t.Fatalf("unexpected /safebox_money_save before process restart: %v", err)
	}
	if len(saveOut) != 2 {
		t.Fatalf("expected gold PLAYER_POINT_CHANGE + SAFEBOX_MONEY_CHANGE before restart, got %d", len(saveOut))
	}
	moneyChange, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, saveOut[1]))
	if err != nil {
		t.Fatalf("decode pre-restart SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if moneyChange != (itemproto.SafeboxMoneyChangePacket{Money: 1750}) {
		t.Fatalf("unexpected pre-restart SAFEBOX_MONEY_CHANGE: %+v", moneyChange)
	}
	snapshot, err := safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load safebox after money deposit before restart: %v", err)
	}
	if got := safeboxstore.CharacterMoney(snapshot, login, owner.ID); got != 1750 {
		t.Fatalf("durable money before restart=%d want 1750", got)
	}
	closeSessionFlow(t, flow)

	const postRestartLoginKey uint32 = 0x92929335
	reloadedTickets := loginticket.NewFileStore(ticketDir)
	issuePeerTicket(t, reloadedTickets, login, postRestartLoginKey, owner)
	reloadedAccounts := accountstore.NewFileStore(accountDir)
	reloaded, err := newGameRuntimeWithAccountStoreAndInteractionStore(cfg, reloadedTickets, reloadedAccounts, nil)
	if err != nil {
		t.Fatalf("reload runtime after safebox money process restart: %v", err)
	}
	restartFlow, _ := enterGameWithLoginTicket(t, reloaded.SessionFactory(), login, postRestartLoginKey)
	defer closeSessionFlow(t, restartFlow)
	_ = flushServerFrames(t, restartFlow)

	reopenOut, err := restartFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox after money process restart: %v", err)
	}
	if len(reopenOut) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE after process restart, got %d", len(reopenOut))
	}
	reopenMoney, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode reopen SAFEBOX_MONEY_CHANGE after process restart: %v", err)
	}
	if reopenMoney != (itemproto.SafeboxMoneyChangePacket{Money: 1750}) {
		t.Fatalf("unexpected reopen SAFEBOX_MONEY_CHANGE after process restart: %+v", reopenMoney)
	}
	account, err := reloadedAccounts.Load(login)
	if err != nil {
		t.Fatalf("load account after money process restart: %v", err)
	}
	if account.Characters[0].Gold != 3250 {
		t.Fatalf("carried gold after money process restart=%d want 3250", account.Characters[0].Gold)
	}
	reloadedSnapshot, err := safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load safebox after money process restart: %v", err)
	}
	if got := safeboxstore.CharacterMoney(reloadedSnapshot, login, owner.ID); got != 1750 {
		t.Fatalf("durable money after process restart=%d want 1750", got)
	}
}

func TestGameRuntimeSafeboxMoneySaveClosesActiveExchangeShellOnSuccess(t *testing.T) {
	root := t.TempDir()
	ticketStore := loginticket.NewFileStore(filepath.Join(root, "tickets"))
	accounts := accountstore.NewFileStore(filepath.Join(root, "accounts"))
	safeboxPath := filepath.Join(root, "state", "safebox.json")
	owner := peerVisibilityCharacter("SafeboxMoneyExchSave", 0x01030926, 0x02040926, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	peer := peerVisibilityCharacter("SafeboxMoneyExchPeer", 0x01030927, 0x02040927, 1120, 2120, 0, 101, 201)
	peer.Gold = 4444
	issuePeerTicket(t, ticketStore, "safebox-money-exch-save", 0x92929326, owner)
	issuePeerTicket(t, ticketStore, "safebox-money-exch-peer", 0x92929327, peer)
	if err := accounts.Save(accountstore.Account{Login: "safebox-money-exch-save", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox money exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "safebox-money-exch-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed safebox money exchange peer account: %v", err)
	}
	cfg := config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", SafeboxStorePath: safeboxPath}
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(cfg, ticketStore, accounts, nil)
	if err != nil {
		t.Fatalf("unexpected safebox money exchange save runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "safebox-money-exch-save", 0x92929326)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "safebox-money-exch-peer", 0x92929327)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected safebox money exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected safebox money exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "safebox money exchange owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected safebox money exchange peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "safebox money exchange peer start")

	if _, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox during money exchange save: %v", err)
	}

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_money_save 1500",
	})))
	if err != nil {
		t.Fatalf("unexpected exchange-open safebox money save error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected exchange-open safebox money save to emit END, gold PLAYER_POINT_CHANGE, and SAFEBOX_MONEY_CHANGE, got %d", len(out))
	}
	assertExchangeEndFrame(t, out[0], "safebox money exchange owner close before save")
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode exchange-open safebox money save gold change: %v", err)
	}
	if goldChange.Amount != -1500 || goldChange.Value != 3500 {
		t.Fatalf("unexpected exchange-open safebox money save gold change: %+v", goldChange)
	}
	moneyChange, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode exchange-open safebox money save SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if moneyChange != (itemproto.SafeboxMoneyChangePacket{Money: 1500}) {
		t.Fatalf("unexpected exchange-open safebox money save SAFEBOX_MONEY_CHANGE: %+v", moneyChange)
	}
	queuedClose := flushServerFrames(t, peerFlow)
	if len(queuedClose) != 1 {
		t.Fatalf("expected safebox money exchange peer to receive one queued END, got %d", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "safebox money exchange peer close before save")

	wantOwner := owner
	wantOwner.Gold = 3500
	assertExchangeAccountUnchanged(t, accounts, "safebox-money-exch-save", wantOwner, "exchange-open safebox money save owner")
	assertExchangeAccountUnchanged(t, accounts, "safebox-money-exch-peer", peer, "exchange-open safebox money save peer")
	snapshot, err := safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load safebox after money exchange save: %v", err)
	}
	if got := safeboxstore.CharacterMoney(snapshot, "safebox-money-exch-save", owner.ID); got != 1500 {
		t.Fatalf("durable money after exchange-open save=%d want 1500", got)
	}
}

func TestGameRuntimeSafeboxMoneyWithdrawClosesActiveMerchantWindowOnSuccess(t *testing.T) {
	root := t.TempDir()
	ticketStore := loginticket.NewFileStore(filepath.Join(root, "tickets"))
	accounts := accountstore.NewFileStore(filepath.Join(root, "accounts"))
	safeboxPath := filepath.Join(root, "state", "safebox.json")
	owner := merchantBuyerCharacter("SafeboxMoneyMerchWithdraw", 0x01030928, 0x02040928, 2000, nil)
	login := "safebox-money-merch-withdraw"
	issuePeerTicket(t, ticketStore, login, 0x92929328, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox money merchant withdraw owner account: %v", err)
	}
	store := safeboxstore.NewFileStore(safeboxPath)
	if err := store.Save(safeboxstore.Snapshot{Characters: []safeboxstore.CharacterRow{{
		Login: login, CharacterID: owner.ID, Money: 900,
	}}}); err != nil {
		t.Fatalf("seed durable warehouse money before merchant withdraw: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{defaultMerchantCatalogDefinition()})
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())
	cfg := config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", SafeboxStorePath: safeboxPath}
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(cfg, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected safebox money merchant withdraw runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Merchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, "npc:merchant")
	if !ok {
		t.Fatal("expected merchant static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x92929328)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)
	interactWithMerchantForBuy(t, flow, actor.EntityID)

	if openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox during merchant money withdraw: %v", err)
	} else if len(openOut) < 2 {
		t.Fatalf("expected /open_safebox during merchant money withdraw to emit SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE, got %d", len(openOut))
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_money_withdraw 400",
	})))
	if err != nil {
		t.Fatalf("unexpected merchant-open safebox money withdraw error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected merchant-open safebox money withdraw to emit SHOP END, gold PLAYER_POINT_CHANGE, and SAFEBOX_MONEY_CHANGE, got %d", len(out))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, out[0])); err != nil {
		t.Fatalf("decode merchant SHOP END before accepted safebox money withdraw: %v", err)
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode merchant-open safebox money withdraw gold change: %v", err)
	}
	if goldChange.Amount != 400 || goldChange.Value != 2400 {
		t.Fatalf("unexpected merchant-open safebox money withdraw gold change: %+v", goldChange)
	}
	moneyChange, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode merchant-open safebox money withdraw SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if moneyChange != (itemproto.SafeboxMoneyChangePacket{Money: 500}) {
		t.Fatalf("unexpected merchant-open safebox money withdraw SAFEBOX_MONEY_CHANGE: %+v", moneyChange)
	}

	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected post-withdraw merchant SHOP END error: %v", err)
	}
	if len(closeOut) != 0 {
		t.Fatalf("expected post-withdraw merchant SHOP END to emit no frames after shell close, got %d", len(closeOut))
	}
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{RawLeadingByte: 1, CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected post-withdraw merchant SHOP BUY error: %v", err)
	}
	if len(buyOut) != 0 {
		t.Fatalf("expected post-withdraw merchant SHOP BUY to fail closed until reopen, got %d", len(buyOut))
	}

	wantOwner := owner
	wantOwner.Gold = 2400
	assertExchangeAccountUnchanged(t, accounts, login, wantOwner, "merchant-open safebox money withdraw owner")
	snapshot, err := safeboxstore.LoadOrEmpty(store)
	if err != nil {
		t.Fatalf("load safebox after merchant money withdraw: %v", err)
	}
	if got := safeboxstore.CharacterMoney(snapshot, login, owner.ID); got != 500 {
		t.Fatalf("durable money after merchant-open withdraw=%d want 500", got)
	}
}

func TestGameRuntimeSafeboxMoneyRejectLeavesOpenExchangeShellUntouched(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxMoneyExchReject", 0x01030929, 0x02040929, 1100, 2100, 0, 101, 201)
	owner.Gold = 50
	peer := peerVisibilityCharacter("SafeboxMoneyExchRejectPeer", 0x0103092a, 0x0204092a, 1120, 2120, 0, 101, 201)
	peer.Gold = 4444
	issuePeerTicket(t, ticketStore, "safebox-money-exch-reject", 0x92929329, owner)
	issuePeerTicket(t, ticketStore, "safebox-money-exch-reject-peer", 0x9292932a, peer)
	if err := accounts.Save(accountstore.Account{Login: "safebox-money-exch-reject", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox money exchange reject owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "safebox-money-exch-reject-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed safebox money exchange reject peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil)
	if err != nil {
		t.Fatalf("unexpected safebox money exchange reject runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "safebox-money-exch-reject", 0x92929329)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "safebox-money-exch-reject-peer", 0x9292932a)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected safebox money reject exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected safebox money reject exchange start to emit one owner frame, got %d", len(startOut))
	}
	_ = flushServerFrames(t, peerFlow)

	if _, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox during money exchange reject: %v", err)
	}

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_money_save 100",
	})))
	if err != nil {
		t.Fatalf("unexpected insufficient safebox money save during exchange error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected insufficient safebox money save during exchange to emit no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected insufficient safebox money save to queue no peer frames, got %d", len(queued))
	}

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected exchange cancel after money reject error: %v", err)
	}
	if len(cancelOut) != 1 {
		t.Fatalf("expected exchange cancel after money reject to emit one END, got %d", len(cancelOut))
	}
	assertExchangeEndFrame(t, cancelOut[0], "safebox money reject leaves exchange cancellable")
	assertExchangeAccountUnchanged(t, accounts, "safebox-money-exch-reject", owner, "insufficient money save during exchange owner")
	assertExchangeAccountUnchanged(t, accounts, "safebox-money-exch-reject-peer", peer, "insufficient money save during exchange peer")
}
