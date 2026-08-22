package minimal

import (
	"path/filepath"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

func TestGameRuntimeSafeboxCheckinSurvivesReconnectAndReopenEmitsRememberedSafeboxSet(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxDurableReconnect", 0x01030801, 0x02040801, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 901, Vnum: 27001, Count: 2, Slot: 5}}
	login := "safebox-durable-reconnect"
	const loginKey uint32 = 0x80808001
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed durable safebox reconnect owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected durable safebox reconnect runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before durable check-in: %v", err)
	}
	checkinOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected durable safebox check-in error: %v", err)
	}
	if len(checkinOut) < 2 {
		t.Fatalf("expected durable safebox check-in frames, got %d", len(checkinOut))
	}
	set, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, checkinOut[len(checkinOut)-1]))
	if err != nil {
		t.Fatalf("decode durable safebox check-in SAFEBOX_SET: %v", err)
	}
	if set.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) || set.Vnum != 27001 || set.Count != 2 {
		t.Fatalf("unexpected durable safebox check-in SAFEBOX_SET: %+v", set)
	}

	assertCloseSafeboxCommandChat(t, flow, "/close_safebox", "durable close before phase_select")

	phaseSelectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/phase_select",
	})))
	if err != nil {
		t.Fatalf("unexpected /phase_select after durable check-in: %v", err)
	}
	if len(phaseSelectOut) == 0 {
		t.Fatal("expected /phase_select frames after durable check-in")
	}
	phase, err := control.DecodePhase(decodeSingleFrame(t, phaseSelectOut[len(phaseSelectOut)-1]))
	if err != nil {
		t.Fatalf("decode phase after durable /phase_select: %v", err)
	}
	if phase.Phase != session.PhaseSelect {
		t.Fatalf("expected phase %q after durable /phase_select, got %q", session.PhaseSelect, phase.Phase)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 0}))); err != nil {
		t.Fatalf("unexpected character reselect after durable /phase_select: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected re-enter after durable /phase_select: %v", err)
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox after durable reconnect: %v", err)
	}
	if len(reopenOut) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE + remembered SAFEBOX_SET after durable reconnect, got %d", len(reopenOut))
	}
	reopenSize, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, reopenOut[0]))
	if err != nil {
		t.Fatalf("decode reopen SAFEBOX_SIZE after durable reconnect: %v", err)
	}
	if reopenSize != (itemproto.SafeboxSizePacket{Size: 1}) {
		t.Fatalf("unexpected reopen SAFEBOX_SIZE after durable reconnect: %+v", reopenSize)
	}
	reopenSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode reopen SAFEBOX_SET after durable reconnect: %v", err)
	}
	if reopenSet.Position != set.Position || reopenSet.Vnum != set.Vnum || reopenSet.Count != set.Count {
		t.Fatalf("unexpected reopen SAFEBOX_SET after durable reconnect: %+v want %+v", reopenSet, set)
	}
}

func TestGameRuntimeSafeboxCheckinSurvivesProcessRestartRematerializeOnOpen(t *testing.T) {
	root := t.TempDir()
	ticketDir := filepath.Join(root, "tickets")
	accountDir := filepath.Join(root, "accounts")
	safeboxPath := filepath.Join(root, "safebox", "safebox.json")
	ticketStore := loginticket.NewFileStore(ticketDir)
	accounts := accountstore.NewFileStore(accountDir)
	owner := peerVisibilityCharacter("SafeboxDurableRestart", 0x01030802, 0x02040802, 1100, 2100, 0, 101, 201)
	owner.Gold = 5151
	owner.Inventory = []inventory.ItemInstance{{ID: 902, Vnum: 27001, Count: 3, Slot: 5}}
	login := "safebox-durable-restart"
	const loginKey uint32 = 0x80808002
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed durable safebox restart owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	cfg := config.Service{
		LegacyAddr:       ":13000",
		PublicAddr:       "127.0.0.1",
		SafeboxStorePath: safeboxPath,
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(cfg, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected durable safebox restart runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	_ = flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before restart check-in: %v", err)
	}
	checkinOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 1,
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected durable safebox restart check-in error: %v", err)
	}
	set, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, checkinOut[len(checkinOut)-1]))
	if err != nil {
		t.Fatalf("decode durable safebox restart SAFEBOX_SET: %v", err)
	}
	if set.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 1}) || set.Vnum != 27001 || set.Count != 3 {
		t.Fatalf("unexpected durable safebox restart SAFEBOX_SET: %+v", set)
	}
	closeSessionFlow(t, flow)

	const postRestartLoginKey uint32 = 0x80808012
	reloadedTickets := loginticket.NewFileStore(ticketDir)
	issuePeerTicket(t, reloadedTickets, login, postRestartLoginKey, owner)
	reloadedAccounts := accountstore.NewFileStore(accountDir)
	reloadedItems := newItemTemplateStore(t, []itemcatalog.Template{template})
	reloaded, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(cfg, reloadedTickets, reloadedAccounts, nil, nil, reloadedItems, nil)
	if err != nil {
		t.Fatalf("reload runtime after durable safebox process restart: %v", err)
	}
	restartFlow, _ := enterGameWithLoginTicket(t, reloaded.SessionFactory(), login, postRestartLoginKey)
	defer closeSessionFlow(t, restartFlow)
	_ = flushServerFrames(t, restartFlow)

	reopenOut, err := restartFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox after process restart: %v", err)
	}
	if len(reopenOut) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE + remembered SAFEBOX_SET after process restart, got %d", len(reopenOut))
	}
	reopenSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode reopen SAFEBOX_SET after process restart: %v", err)
	}
	if reopenSet.Position != set.Position || reopenSet.Vnum != set.Vnum || reopenSet.Count != set.Count {
		t.Fatalf("unexpected reopen SAFEBOX_SET after process restart: %+v want %+v", reopenSet, set)
	}
}

func TestGameRuntimeSafeboxItemMovePersistsDurableCellsWithoutInventoryMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxDurableMove", 0x01030803, 0x02040803, 1100, 2100, 0, 101, 201)
	owner.Gold = 6161
	owner.Inventory = []inventory.ItemInstance{{ID: 903, Vnum: 27001, Count: 2, Slot: 5}}
	login := "safebox-durable-move"
	const loginKey uint32 = 0x80808003
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed durable safebox move owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected durable safebox move runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before durable move: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	}))); err != nil {
		t.Fatalf("unexpected check-in before durable move: %v", err)
	}
	afterCheckin := owner
	afterCheckin.Inventory = nil
	assertExchangeAccountUnchanged(t, accounts, login, afterCheckin, "durable move check-in")

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{
		Source:      itemproto.InventoryPosition(0),
		Destination: itemproto.InventoryPosition(3),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected durable safebox item-move error: %v", err)
	}
	if len(moveOut) != 2 {
		t.Fatalf("expected SAFEBOX_DEL + SAFEBOX_SET for durable item-move, got %d", len(moveOut))
	}
	set, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, moveOut[1]))
	if err != nil {
		t.Fatalf("decode durable item-move SAFEBOX_SET: %v", err)
	}
	if set.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 3}) || set.Vnum != 27001 || set.Count != 2 {
		t.Fatalf("unexpected durable item-move SAFEBOX_SET: %+v", set)
	}
	assertExchangeAccountUnchanged(t, accounts, login, afterCheckin, "durable item-move inventory")

	phaseSelectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/phase_select",
	})))
	if err != nil {
		t.Fatalf("unexpected /phase_select after durable move: %v", err)
	}
	if len(phaseSelectOut) == 0 {
		t.Fatal("expected /phase_select frames after durable move")
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 0}))); err != nil {
		t.Fatalf("unexpected reselect after durable move: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected re-enter after durable move: %v", err)
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox after durable move reconnect: %v", err)
	}
	if len(reopenOut) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE + moved SAFEBOX_SET after durable move reconnect, got %d", len(reopenOut))
	}
	reopenSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode reopen SAFEBOX_SET after durable move: %v", err)
	}
	if reopenSet.Position != set.Position || reopenSet.Vnum != set.Vnum || reopenSet.Count != set.Count {
		t.Fatalf("unexpected reopen SAFEBOX_SET after durable move: %+v want %+v", reopenSet, set)
	}
	assertExchangeAccountUnchanged(t, accounts, login, afterCheckin, "durable move reopen inventory")
}

func TestGameRuntimeSafeboxDoesNotLeakForeignCharacterRowsOnSameAccount(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	charA := peerVisibilityCharacter("SafeboxLeakA", 0x01030804, 0x02040804, 1100, 2100, 0, 101, 201)
	charA.Gold = 100
	charA.Inventory = []inventory.ItemInstance{{ID: 904, Vnum: 27001, Count: 1, Slot: 5}}
	charB := peerVisibilityCharacter("SafeboxLeakB", 0x01030805, 0x02040805, 1200, 2200, 0, 102, 202)
	charB.Gold = 200
	login := "safebox-durable-leak"
	const loginKey uint32 = 0x80808004
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
		t.Fatalf("seed multi-character account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected durable safebox leak runtime error: %v", err)
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
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("open safebox on char A: %v", err)
	}
	if _, err := flowA.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	}))); err != nil {
		t.Fatalf("check-in on char A: %v", err)
	}
	closeSessionFlow(t, flowA)

	const loginKeyB uint32 = 0x80808005
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
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("open safebox on char B: %v", err)
	}
	if len(openB) != 1 {
		t.Fatalf("expected only SAFEBOX_SIZE on char B (no leaked SAFEBOX_SET from char A), got %d frames", len(openB))
	}
	size, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, openB[0]))
	if err != nil {
		t.Fatalf("decode char B SAFEBOX_SIZE: %v", err)
	}
	if size != (itemproto.SafeboxSizePacket{Size: 1}) {
		t.Fatalf("unexpected char B SAFEBOX_SIZE: %+v", size)
	}
}
