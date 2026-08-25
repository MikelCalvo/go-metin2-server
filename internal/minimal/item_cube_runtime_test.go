package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/cubestore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
)

func TestGameRuntimeOpenCubeEmitsCommandChatWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("OpenCubeOwner", 0x010308a1, 0x020408a1, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 1801, Vnum: 27001, Count: 2, Slot: 5}}
	login := "open-cube-owner"
	issuePeerTicket(t, ticketStore, login, 0x707071a1, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed open-cube owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected open-cube runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071a1)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected /open_cube to emit one command chat frame, got %d", len(out))
	}
	assertCubeCommandChatFrame(t, out[0], "cube open 20022", "default open")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected /open_cube to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "open-cube owner")
}

func TestGameRuntimeOpenCubeExplicitNPCVnumAndCloseClearsPresentation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CloseCubeOwner", 0x010308a2, 0x020408a2, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	login := "close-cube-owner"
	issuePeerTicket(t, ticketStore, login, 0x707071a2, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed close-cube owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected close-cube runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071a2)
	defer closeSessionFlow(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube 20017",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube 20017 error: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_cube 20017 to emit one command chat frame, got %d", len(openOut))
	}
	assertCubeCommandChatFrame(t, openOut[0], "cube open 20017", "explicit open")

	alreadyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected already-open /open_cube error: %v", err)
	}
	if len(alreadyOut) != 1 {
		t.Fatalf("expected already-open /open_cube to emit one info chat frame, got %d", len(alreadyOut))
	}
	info, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, alreadyOut[0]))
	if err != nil {
		t.Fatalf("decode already-open cube info chat: %v", err)
	}
	if info.Type != chatproto.ChatTypeInfo || info.VID != 0 || info.Message != cubeAlreadyOpenInfoMessage {
		t.Fatalf("unexpected already-open cube info chat: %+v", info)
	}

	assertCloseCubeCommandChat(t, flow, "/close_cube", "close-cube owner")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "close-cube owner")

	alreadyClosedOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected already-closed /close_cube error: %v", err)
	}
	if len(alreadyClosedOut) != 0 {
		t.Fatalf("expected already-closed /close_cube to emit no frames, got %d", len(alreadyClosedOut))
	}
}

func TestGameRuntimeOpenCubeRejectsBusySafeboxWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("BusyCubeOwner", 0x010308a3, 0x020408a3, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	login := "busy-cube-owner"
	issuePeerTicket(t, ticketStore, login, 0x707071a3, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed busy-cube owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected busy-cube runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071a3)
	defer closeSessionFlow(t, flow)

	openSafeboxOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox before cube error: %v", err)
	}
	if len(openSafeboxOut) == 0 {
		t.Fatal("expected /open_safebox before cube to emit presentation frames")
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected busy-shell /open_cube error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected busy-shell /open_cube to emit one info chat frame, got %d", len(out))
	}
	info, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode busy-shell cube info chat: %v", err)
	}
	if info.Type != chatproto.ChatTypeInfo || info.VID != 0 || info.Message != cubeBusyShellInfoMessage {
		t.Fatalf("unexpected busy-shell cube info chat: %+v", info)
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "busy-cube owner")
}

func TestGameRuntimeOpenCubeInvalidVnumFailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("InvalidCubeOwner", 0x010308a4, 0x020408a4, 1100, 2100, 0, 101, 201)
	login := "invalid-cube-owner"
	issuePeerTicket(t, ticketStore, login, 0x707071a4, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed invalid-cube owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected invalid-cube runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071a4)
	defer closeSessionFlow(t, flow)

	for _, message := range []string{"/open_cube 0", "/open_cube abc", "/open_cube 20022 extra"} {
		out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
			Type:    chatproto.ChatTypeTalking,
			Message: message,
		})))
		if err != nil {
			t.Fatalf("unexpected %s error: %v", message, err)
		}
		if len(out) != 0 {
			t.Fatalf("expected %s to emit no frames, got %d", message, len(out))
		}
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "invalid-cube owner")
}

func TestSharedWorldRegistrySetCubeWindowOpenRoundTripsBusyBit(t *testing.T) {
	registry := newSharedWorldRegistry()
	owner := peerVisibilityCharacter("CubeBusyBitOwner", 0x010308a5, 0x020408a5, 1100, 2100, 0, 101, 201)
	ownerPending := newPendingServerFrames()
	ownerID, _ := registry.Join(owner, ownerPending, nil)
	if ownerID == 0 {
		t.Fatal("expected Join to allocate owner entity ID")
	}
	if !registry.SetCubeWindowOpen(ownerID, true) {
		t.Fatal("expected SetCubeWindowOpen(true) to succeed")
	}
	registry.mu.Lock()
	open := registry.hasCubeWindowOpenLocked(ownerID)
	registry.mu.Unlock()
	if !open {
		t.Fatal("expected cube busy bit to be set after SetCubeWindowOpen(true)")
	}
	if !registry.SetCubeWindowOpen(ownerID, false) {
		t.Fatal("expected SetCubeWindowOpen(false) to succeed")
	}
	registry.mu.Lock()
	open = registry.hasCubeWindowOpenLocked(ownerID)
	registry.mu.Unlock()
	if open {
		t.Fatal("expected cube busy bit to clear after SetCubeWindowOpen(false)")
	}
	registry.Leave(ownerID)
}

func TestGameRuntimeItemExchangeStartRejectsActiveCubeWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchCubeStartOwner", 0x010308b1, 0x020408b1, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 1811, Vnum: 27001, Count: 3, Slot: 5}}
	peer := peerVisibilityCharacter("ExchCubeStartPeer", 0x010308b2, 0x020408b2, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "exch-cube-start-owner"
	peerLogin := "exch-cube-start-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707071b1, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707071b2, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-open exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed cube-open exchange peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected cube-open exchange runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707071b1)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707071b2)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube before exchange start: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_cube before exchange start to emit one command chat frame, got %d", len(openOut))
	}
	assertCubeCommandChatFrame(t, openOut[0], "cube open 20022", "requester open before start")

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected cube-open exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange start with open cube to emit one info chat frame, got %d", len(startOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0]))
	if err != nil {
		t.Fatalf("decode cube-open exchange start info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeRequesterMerchantBusyInfoMessage {
		t.Fatalf("unexpected cube-open exchange start info chat: %+v", infoChat)
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected exchange start with open cube to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "cube-open exchange start owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "cube-open exchange start peer")
}

func TestGameRuntimeItemExchangeStartRejectsPartnerActiveCubeWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchPartnerCubeOwner", 0x010308b3, 0x020408b3, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 1813, Vnum: 27001, Count: 3, Slot: 5}}
	peer := peerVisibilityCharacter("ExchPartnerCubePeer", 0x010308b4, 0x020408b4, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 1814, Vnum: 27001, Count: 2, Slot: 6}}
	ownerLogin := "exch-partner-cube-owner"
	peerLogin := "exch-partner-cube-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707071b3, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707071b4, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partner-cube exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed partner-cube exchange peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected partner-cube exchange runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707071b3)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707071b4)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	openOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected partner /open_cube before exchange start: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected partner /open_cube before exchange start to emit one command chat frame, got %d", len(openOut))
	}
	assertCubeCommandChatFrame(t, openOut[0], "cube open 20022", "partner open before start")

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected partner-cube exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected partner-cube exchange start to emit one info chat frame, got %d", len(startOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0]))
	if err != nil {
		t.Fatalf("decode partner-cube exchange start info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangePartnerMerchantBusyInfoMessage {
		t.Fatalf("unexpected partner-cube exchange start info chat: %+v", infoChat)
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected partner-cube exchange start to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "partner-cube exchange start owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "partner-cube exchange start peer")
}

func assertCloseCubeCommandChat(t *testing.T, flow service.SessionFlow, slash string, label string) {
	t.Helper()
	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: slash,
	})))
	if err != nil {
		t.Fatalf("unexpected %s %s error: %v", label, slash, err)
	}
	if len(closeOut) != 1 {
		t.Fatalf("expected %s %s to emit one cube close command chat, got %d", label, slash, len(closeOut))
	}
	assertCubeCommandChatFrame(t, closeOut[0], "cube close", label+" "+slash)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected %s %s to queue no peer frames, got %d", label, slash, len(queued))
	}
}

func TestGameRuntimeCubeRInfoEmitsAuthoredResultListWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeRInfoOwner", 0x010308c1, 0x020408c1, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 1901, Vnum: 27001, Count: 2, Slot: 5}}
	login := "cube-r-info-owner"
	issuePeerTicket(t, ticketStore, login, 0x707071c1, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-r-info owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected cube-r-info runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071c1)
	defer closeSessionFlow(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube before r_info: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_cube before r_info to emit one command chat frame, got %d", len(openOut))
	}
	assertCubeCommandChatFrame(t, openOut[0], "cube open 20022", "open before r_info")

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube r_info",
	})))
	if err != nil {
		t.Fatalf("unexpected /cube r_info error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected /cube r_info to emit one command chat frame, got %d", len(out))
	}
	assertCubeCommandChatFrame(t, out[0], "cube r_list 20022 1 27001,1", "authored r_list")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected /cube r_info to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "cube-r-info owner")
}

func TestGameRuntimeCubeRInfoFailsClosedWhenClosedMissingOrOversize(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeRInfoNegOwner", 0x010308c2, 0x020408c2, 1100, 2100, 0, 101, 201)
	owner.Gold = 3333
	login := "cube-r-info-neg-owner"
	issuePeerTicket(t, ticketStore, login, 0x707071c2, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-r-info negative owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected cube-r-info negative runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071c2)
	defer closeSessionFlow(t, flow)

	closedOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube r_info",
	})))
	if err != nil {
		t.Fatalf("unexpected closed /cube r_info error: %v", err)
	}
	if len(closedOut) != 0 {
		t.Fatalf("expected closed /cube r_info to emit no frames, got %d", len(closedOut))
	}

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube 20017",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube 20017 before missing-recipe r_info: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_cube 20017 to emit one command chat frame, got %d", len(openOut))
	}
	assertCubeCommandChatFrame(t, openOut[0], "cube open 20017", "open missing-recipe npc")

	missingOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube r_info",
	})))
	if err != nil {
		t.Fatalf("unexpected missing-recipe /cube r_info error: %v", err)
	}
	if len(missingOut) != 0 {
		t.Fatalf("expected missing-recipe /cube r_info to emit no frames, got %d", len(missingOut))
	}

	assertCloseCubeCommandChat(t, flow, "/close_cube", "cube-r-info negative")

	afterCloseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube r_info",
	})))
	if err != nil {
		t.Fatalf("unexpected after-close /cube r_info error: %v", err)
	}
	if len(afterCloseOut) != 0 {
		t.Fatalf("expected after-close /cube r_info to emit no frames, got %d", len(afterCloseOut))
	}

	openDefaultOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube before oversize r_info: %v", err)
	}
	if len(openDefaultOut) != 1 {
		t.Fatalf("expected /open_cube before oversize r_info to emit one command chat frame, got %d", len(openDefaultOut))
	}
	assertCubeCommandChatFrame(t, openDefaultOut[0], "cube open 20022", "open before oversize")

	oversizeRecipes := make([]cubestore.Recipe, 0, 80)
	for i := 0; i < 80; i++ {
		oversizeRecipes = append(oversizeRecipes, cubestore.Recipe{
			Reward:    cubestore.Reward{Vnum: 100000 + uint32(i), Count: 9999},
			Materials: []cubestore.Material{},
		})
	}
	runtime.cubeRecipes = cubestore.Snapshot{NPCs: []cubestore.NPCRecipes{{
		NPCVnum: bootstrapCubeOpenDefaultNPCVnum,
		Recipes: oversizeRecipes,
	}}}

	oversizeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube r_info",
	})))
	if err != nil {
		t.Fatalf("unexpected oversize /cube r_info error: %v", err)
	}
	if len(oversizeOut) != 0 {
		t.Fatalf("expected oversize /cube r_info to emit no frames, got %d", len(oversizeOut))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "cube-r-info negative owner")
}

func TestGameRuntimeCubeRInfoIndexEmitsAuthoredMaterialInfoWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeMInfoOwner", 0x010308c3, 0x020408c3, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 1903, Vnum: 27001, Count: 2, Slot: 5}}
	login := "cube-m-info-owner"
	issuePeerTicket(t, ticketStore, login, 0x707071c3, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-m-info owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected cube-m-info runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071c3)
	defer closeSessionFlow(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube before m_info: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_cube before m_info to emit one command chat frame, got %d", len(openOut))
	}
	assertCubeCommandChatFrame(t, openOut[0], "cube open 20022", "open before m_info")

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube r_info 0",
	})))
	if err != nil {
		t.Fatalf("unexpected /cube r_info 0 error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected /cube r_info 0 to emit one command chat frame, got %d", len(out))
	}
	assertCubeCommandChatFrame(t, out[0], "cube m_info 0 1 27002,2/100", "authored m_info")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected /cube r_info 0 to queue no peer frames, got %d", len(queued))
	}

	countOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube r_info 0 1",
	})))
	if err != nil {
		t.Fatalf("unexpected /cube r_info 0 1 error: %v", err)
	}
	if len(countOut) != 1 {
		t.Fatalf("expected /cube r_info 0 1 to emit one command chat frame, got %d", len(countOut))
	}
	assertCubeCommandChatFrame(t, countOut[0], "cube m_info 0 1 27002,2/100", "authored m_info with count")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "cube-m-info owner")
}

func TestGameRuntimeCubeRInfoIndexFailsClosedWhenClosedPastEndOrMalformed(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeMInfoNegOwner", 0x010308c4, 0x020408c4, 1100, 2100, 0, 101, 201)
	owner.Gold = 3333
	login := "cube-m-info-neg-owner"
	issuePeerTicket(t, ticketStore, login, 0x707071c4, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-m-info negative owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected cube-m-info negative runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071c4)
	defer closeSessionFlow(t, flow)

	closedOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube r_info 0",
	})))
	if err != nil {
		t.Fatalf("unexpected closed /cube r_info 0 error: %v", err)
	}
	if len(closedOut) != 0 {
		t.Fatalf("expected closed /cube r_info 0 to emit no frames, got %d", len(closedOut))
	}

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube before m_info negatives: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_cube before m_info negatives to emit one command chat frame, got %d", len(openOut))
	}
	assertCubeCommandChatFrame(t, openOut[0], "cube open 20022", "open before m_info negatives")

	for _, message := range []string{"/cube r_info 99", "/cube r_info abc", "/cube r_info 0 xyz", "/cube r_info 0 1 extra"} {
		out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
			Type:    chatproto.ChatTypeTalking,
			Message: message,
		})))
		if err != nil {
			t.Fatalf("unexpected %s error: %v", message, err)
		}
		if len(out) != 0 {
			t.Fatalf("expected %s to emit no frames, got %d", message, len(out))
		}
	}

	runtime.cubeRecipes = cubestore.Snapshot{NPCs: []cubestore.NPCRecipes{{
		NPCVnum: bootstrapCubeOpenDefaultNPCVnum,
		Recipes: []cubestore.Recipe{{
			Reward:    cubestore.Reward{Vnum: 27001, Count: 1},
			Materials: []cubestore.Material{},
			Gold:      100,
		}},
	}}}
	emptyMaterialsOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube r_info 0",
	})))
	if err != nil {
		t.Fatalf("unexpected empty-materials /cube r_info 0 error: %v", err)
	}
	if len(emptyMaterialsOut) != 0 {
		t.Fatalf("expected empty-materials /cube r_info 0 to emit no frames, got %d", len(emptyMaterialsOut))
	}

	oversizeRecipes := make([]cubestore.Recipe, 0, 40)
	for i := 0; i < 40; i++ {
		oversizeRecipes = append(oversizeRecipes, cubestore.Recipe{
			Reward: cubestore.Reward{Vnum: 1, Count: 1},
			Materials: []cubestore.Material{
				{Vnum: 100000 + uint32(i), Count: 9999},
				{Vnum: 200000 + uint32(i), Count: 9999},
			},
			Gold: 99999999,
		})
	}
	runtime.cubeRecipes = cubestore.Snapshot{NPCs: []cubestore.NPCRecipes{{
		NPCVnum: bootstrapCubeOpenDefaultNPCVnum,
		Recipes: oversizeRecipes,
	}}}
	oversizeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube r_info 0 40",
	})))
	if err != nil {
		t.Fatalf("unexpected oversize /cube r_info 0 40 error: %v", err)
	}
	if len(oversizeOut) != 0 {
		t.Fatalf("expected oversize /cube r_info 0 40 to emit no frames, got %d", len(oversizeOut))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "cube-m-info negative owner")
}

func TestGameRuntimeCubeAddDelEmitsAuthoredCubeInfoWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeAddDelOwner", 0x010308c5, 0x020408c5, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1905, Vnum: 27002, Count: 1, Slot: 5},
		{ID: 1906, Vnum: 27002, Count: 1, Slot: 6},
	}
	login := "cube-add-del-owner"
	issuePeerTicket(t, ticketStore, login, 0x707071c5, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-add-del owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected cube-add-del runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071c5)
	defer closeSessionFlow(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube before cube add: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_cube before cube add to emit one command chat frame, got %d", len(openOut))
	}
	assertCubeCommandChatFrame(t, openOut[0], "cube open 20022", "open before cube add")

	firstAddOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube add 0 5",
	})))
	if err != nil {
		t.Fatalf("unexpected /cube add 0 5 error: %v", err)
	}
	if len(firstAddOut) != 1 {
		t.Fatalf("expected /cube add 0 5 to emit one command chat frame, got %d", len(firstAddOut))
	}
	assertCubeCommandChatFrame(t, firstAddOut[0], "cube info 0 0 0", "partial cube add")

	secondAddOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube add 1 6",
	})))
	if err != nil {
		t.Fatalf("unexpected /cube add 1 6 error: %v", err)
	}
	if len(secondAddOut) != 1 {
		t.Fatalf("expected /cube add 1 6 to emit one command chat frame, got %d", len(secondAddOut))
	}
	assertCubeCommandChatFrame(t, secondAddOut[0], "cube info 100 0 0", "matched cube add")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected /cube add to queue no peer frames, got %d", len(queued))
	}

	delOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube del 0",
	})))
	if err != nil {
		t.Fatalf("unexpected /cube del 0 error: %v", err)
	}
	if len(delOut) != 1 {
		t.Fatalf("expected /cube del 0 to emit one command chat frame, got %d", len(delOut))
	}
	assertCubeCommandChatFrame(t, delOut[0], "cube info 0 0 0", "incomplete after del")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "cube-add-del owner")
}

func TestGameRuntimeCubeAddDelFailsClosedWhenClosedOutOfRangeOrMalformed(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeAddDelNegOwner", 0x010308c6, 0x020408c6, 1100, 2100, 0, 101, 201)
	owner.Gold = 3333
	owner.Inventory = []inventory.ItemInstance{{ID: 1907, Vnum: 27002, Count: 2, Slot: 5}}
	login := "cube-add-del-neg-owner"
	issuePeerTicket(t, ticketStore, login, 0x707071c6, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-add-del negative owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected cube-add-del negative runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071c6)
	defer closeSessionFlow(t, flow)

	for _, message := range []string{"/cube add 0 5", "/cube del 0", "/cube delete 0"} {
		out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
			Type:    chatproto.ChatTypeTalking,
			Message: message,
		})))
		if err != nil {
			t.Fatalf("unexpected closed %s error: %v", message, err)
		}
		if len(out) != 0 {
			t.Fatalf("expected closed %s to emit no frames, got %d", message, len(out))
		}
	}

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube before cube add negatives: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_cube before cube add negatives to emit one command chat frame, got %d", len(openOut))
	}
	assertCubeCommandChatFrame(t, openOut[0], "cube open 20022", "open before cube add negatives")

	for _, message := range []string{
		"/cube add 99 5",
		"/cube add 0 90",
		"/cube add 0 abc",
		"/cube add 0",
		"/cube add 0 5 extra",
		"/cube del 99",
		"/cube del abc",
		"/cube delete",
		"/cube del 0",
	} {
		out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
			Type:    chatproto.ChatTypeTalking,
			Message: message,
		})))
		if err != nil {
			t.Fatalf("unexpected %s error: %v", message, err)
		}
		if len(out) != 0 {
			t.Fatalf("expected %s to emit no frames, got %d", message, len(out))
		}
	}

	emptyCellOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube add 0 7",
	})))
	if err != nil {
		t.Fatalf("unexpected empty-cell /cube add 0 7 error: %v", err)
	}
	if len(emptyCellOut) != 0 {
		t.Fatalf("expected empty-cell /cube add 0 7 to emit no frames, got %d", len(emptyCellOut))
	}

	addOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube add 0 5",
	})))
	if err != nil {
		t.Fatalf("unexpected /cube add 0 5 before close: %v", err)
	}
	if len(addOut) != 1 {
		t.Fatalf("expected /cube add 0 5 before close to emit one command chat frame, got %d", len(addOut))
	}
	assertCubeCommandChatFrame(t, addOut[0], "cube info 100 0 0", "matched single-cell cube add")

	assertCloseCubeCommandChat(t, flow, "/close_cube", "cube-add-del negative")

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected reopen /open_cube after close: %v", err)
	}
	if len(reopenOut) != 1 {
		t.Fatalf("expected reopen /open_cube to emit one command chat frame, got %d", len(reopenOut))
	}
	assertCubeCommandChatFrame(t, reopenOut[0], "cube open 20022", "reopen after close")

	emptyDelOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube del 0",
	})))
	if err != nil {
		t.Fatalf("unexpected reopen empty /cube del 0 error: %v", err)
	}
	if len(emptyDelOut) != 0 {
		t.Fatalf("expected reopen empty /cube del 0 to emit no frames, got %d", len(emptyDelOut))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "cube-add-del negative owner")
}

func assertCubeCommandChatFrame(t *testing.T, frame []byte, wantMessage string, label string) {
	t.Helper()
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, frame))
	if err != nil {
		t.Fatalf("decode %s cube command chat: %v", label, err)
	}
	if delivery.Type != chatproto.ChatTypeCommand || delivery.VID != 0 || delivery.Empire != 0 || delivery.Message != wantMessage {
		t.Fatalf("unexpected %s cube command chat: %+v want %q", label, delivery, wantMessage)
	}
}
