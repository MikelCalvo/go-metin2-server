package minimal

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/cubestore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
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

func cubeMakeItemTemplates(t *testing.T) itemcatalog.Store {
	t.Helper()
	return newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 7},
	})
}

func TestGameRuntimeCubeMakePercent100ConsumesGrantsPersistsAndEmitsBurst(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeMakeOwner", 0x010308c7, 0x020408c7, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1910, Vnum: 27002, Count: 1, Slot: 5},
		{ID: 1911, Vnum: 27002, Count: 1, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 6},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
	}
	login := "cube-make-owner"
	issuePeerTicket(t, ticketStore, login, 0x707071c7, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-make owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, cubeMakeItemTemplates(t), nil)
	if err != nil {
		t.Fatalf("unexpected cube-make runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071c7)
	defer closeSessionFlow(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube before cube make: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_cube before cube make to emit one command chat frame, got %d", len(openOut))
	}
	assertCubeCommandChatFrame(t, openOut[0], "cube open 20022", "open before cube make")

	firstAddOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube add 0 5",
	})))
	if err != nil {
		t.Fatalf("unexpected /cube add 0 5 before make: %v", err)
	}
	if len(firstAddOut) != 1 {
		t.Fatalf("expected /cube add 0 5 before make to emit one frame, got %d", len(firstAddOut))
	}
	secondAddOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube add 1 6",
	})))
	if err != nil {
		t.Fatalf("unexpected /cube add 1 6 before make: %v", err)
	}
	if len(secondAddOut) != 1 {
		t.Fatalf("expected /cube add 1 6 before make to emit one frame, got %d", len(secondAddOut))
	}
	assertCubeCommandChatFrame(t, secondAddOut[0], "cube info 100 0 0", "matched before make")

	makeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube make",
	})))
	if err != nil {
		t.Fatalf("unexpected /cube make error: %v", err)
	}
	if len(makeOut) != 8 {
		t.Fatalf("expected /cube make burst of 8 frames, got %d", len(makeOut))
	}
	delA, err := itemproto.DecodeDel(decodeSingleFrame(t, makeOut[0]))
	if err != nil {
		t.Fatalf("decode material A ITEM_DEL: %v", err)
	}
	if delA.Position.WindowType != itemproto.WindowInventory || delA.Position.Cell != 5 {
		t.Fatalf("unexpected material A delete position: %+v", delA.Position)
	}
	delB, err := itemproto.DecodeDel(decodeSingleFrame(t, makeOut[1]))
	if err != nil {
		t.Fatalf("decode material B ITEM_DEL: %v", err)
	}
	if delB.Position.WindowType != itemproto.WindowInventory || delB.Position.Cell != 6 {
		t.Fatalf("unexpected material B delete position: %+v", delB.Position)
	}
	quickslotDelA, err := quickslotproto.DecodeDel(decodeSingleFrame(t, makeOut[2]))
	if err != nil {
		t.Fatalf("decode material A QUICKSLOT_DEL: %v", err)
	}
	if quickslotDelA.Position != 2 {
		t.Fatalf("unexpected material A quickslot delete position: %+v", quickslotDelA)
	}
	quickslotDelB, err := quickslotproto.DecodeDel(decodeSingleFrame(t, makeOut[3]))
	if err != nil {
		t.Fatalf("decode material B QUICKSLOT_DEL: %v", err)
	}
	if quickslotDelB.Position != 4 {
		t.Fatalf("unexpected material B quickslot delete position: %+v", quickslotDelB)
	}
	rewardSet, err := itemproto.DecodeSet(decodeSingleFrame(t, makeOut[4]))
	if err != nil {
		t.Fatalf("decode reward ITEM_SET: %v", err)
	}
	if rewardSet.Position.WindowType != itemproto.WindowInventory || rewardSet.Vnum != 27001 || rewardSet.Count != 1 {
		t.Fatalf("unexpected reward ITEM_SET: %+v", rewardSet)
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, makeOut[5]))
	if err != nil {
		t.Fatalf("decode gold PLAYER_POINT_CHANGE: %v", err)
	}
	if goldChange.VID != owner.VID || goldChange.Type != bootstrapGoldPointType || goldChange.Amount != -100 || goldChange.Value != 4142 {
		t.Fatalf("unexpected gold point change: %+v", goldChange)
	}
	assertCubeCommandChatFrame(t, makeOut[6], "cube success 27001 1", "cube make success")
	assertCubeCommandChatFrame(t, makeOut[7], "cube info 0 0 0", "post-make cube info")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected /cube make to queue no peer frames, got %d", len(queued))
	}

	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted cube-make account: %v", err)
	}
	if persisted.Characters[0].Gold != 4142 {
		t.Fatalf("unexpected persisted gold after cube make: %d", persisted.Characters[0].Gold)
	}
	if len(persisted.Characters[0].Inventory) != 1 || persisted.Characters[0].Inventory[0].Vnum != 27001 || persisted.Characters[0].Inventory[0].Count != 1 {
		t.Fatalf("unexpected persisted inventory after cube make: %+v", persisted.Characters[0].Inventory)
	}
	wantQuickslots := []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeSkill, Slot: 6}}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted quickslots after cube make: %+v want %+v", persisted.Characters[0].Quickslots, wantQuickslots)
	}
}

func TestGameRuntimeCubeMakeFailsClosedWhenClosedUnmatchedGoldOrInventoryFull(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeMakeNegOwner", 0x010308c8, 0x020408c8, 1100, 2100, 0, 101, 201)
	owner.Gold = 50
	owner.Inventory = []inventory.ItemInstance{{ID: 1912, Vnum: 27002, Count: 2, Slot: 5}}
	login := "cube-make-neg-owner"
	issuePeerTicket(t, ticketStore, login, 0x707071c8, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-make negative owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, cubeMakeItemTemplates(t), nil)
	if err != nil {
		t.Fatalf("unexpected cube-make negative runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071c8)
	defer closeSessionFlow(t, flow)

	for _, message := range []string{"/cube make", "/cube make all", "/cube make extra"} {
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
		t.Fatalf("unexpected /open_cube before cube make negatives: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_cube before cube make negatives to emit one frame, got %d", len(openOut))
	}

	unmatchedOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube make",
	})))
	if err != nil {
		t.Fatalf("unexpected unmatched /cube make error: %v", err)
	}
	if len(unmatchedOut) != 1 {
		t.Fatalf("expected unmatched /cube make to emit one info chat frame, got %d", len(unmatchedOut))
	}
	info, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, unmatchedOut[0]))
	if err != nil {
		t.Fatalf("decode unmatched cube make info chat: %v", err)
	}
	if info.Type != chatproto.ChatTypeInfo || info.Message != cubeMakeInsufficientMaterialsInfoMessage {
		t.Fatalf("unexpected unmatched cube make info chat: %+v", info)
	}

	addOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube add 0 5",
	})))
	if err != nil {
		t.Fatalf("unexpected /cube add before insufficient gold make: %v", err)
	}
	if len(addOut) != 1 {
		t.Fatalf("expected /cube add before insufficient gold make to emit one frame, got %d", len(addOut))
	}
	assertCubeCommandChatFrame(t, addOut[0], "cube info 100 0 0", "matched before insufficient gold")

	goldOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube make",
	})))
	if err != nil {
		t.Fatalf("unexpected insufficient-gold /cube make error: %v", err)
	}
	if len(goldOut) != 1 {
		t.Fatalf("expected insufficient-gold /cube make to emit one info chat frame, got %d", len(goldOut))
	}
	goldInfo, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, goldOut[0]))
	if err != nil {
		t.Fatalf("decode insufficient-gold cube make info chat: %v", err)
	}
	if goldInfo.Type != chatproto.ChatTypeInfo || goldInfo.Message != exchangeFinalizeCheckSelfInfoMessage {
		t.Fatalf("unexpected insufficient-gold cube make info chat: %+v", goldInfo)
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "cube-make negative owner after gold reject")

	makeAllOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube make all",
	})))
	if err != nil {
		t.Fatalf("unexpected open /cube make all error: %v", err)
	}
	if len(makeAllOut) != 1 {
		t.Fatalf("expected open insufficient-gold /cube make all to emit one info chat frame, got %d", len(makeAllOut))
	}
	makeAllInfo, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, makeAllOut[0]))
	if err != nil {
		t.Fatalf("decode insufficient-gold /cube make all info chat: %v", err)
	}
	if makeAllInfo.Type != chatproto.ChatTypeInfo || makeAllInfo.Message != exchangeFinalizeCheckSelfInfoMessage {
		t.Fatalf("unexpected insufficient-gold /cube make all info chat: %+v", makeAllInfo)
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "cube-make negative owner after make all")

	extraOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube make extra",
	})))
	if err != nil {
		t.Fatalf("unexpected open /cube make extra error: %v", err)
	}
	if len(extraOut) != 0 {
		t.Fatalf("expected open /cube make extra to emit no frames, got %d", len(extraOut))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "cube-make negative owner after make extra")
}

func TestGameRuntimeCubeMakeRejectsInventoryFullWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeMakeFullOwner", 0x010308c9, 0x020408c9, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = make([]inventory.ItemInstance, 0, int(inventory.CarriedInventorySlotCount))
	for cell := inventory.SlotIndex(0); cell < inventory.CarriedInventorySlotCount; cell++ {
		vnum := uint32(27001)
		count := uint16(200)
		if cell == 5 {
			vnum = 27002
			count = 2
		}
		owner.Inventory = append(owner.Inventory, inventory.ItemInstance{ID: uint64(2000 + int(cell)), Vnum: vnum, Count: count, Slot: cell})
	}
	login := "cube-make-full-owner"
	issuePeerTicket(t, ticketStore, login, 0x707071c9, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-make full owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, cubeMakeItemTemplates(t), nil)
	if err != nil {
		t.Fatalf("unexpected cube-make full runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071c9)
	defer closeSessionFlow(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube before inventory-full make: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_cube before inventory-full make to emit one frame, got %d", len(openOut))
	}
	addOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube add 0 5",
	})))
	if err != nil {
		t.Fatalf("unexpected /cube add before inventory-full make: %v", err)
	}
	if len(addOut) != 1 {
		t.Fatalf("expected /cube add before inventory-full make to emit one frame, got %d", len(addOut))
	}
	assertCubeCommandChatFrame(t, addOut[0], "cube info 100 0 0", "matched before inventory-full make")

	fullOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube make",
	})))
	if err != nil {
		t.Fatalf("unexpected inventory-full /cube make error: %v", err)
	}
	if len(fullOut) != 1 {
		t.Fatalf("expected inventory-full /cube make to emit one info chat frame, got %d", len(fullOut))
	}
	fullInfo, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, fullOut[0]))
	if err != nil {
		t.Fatalf("decode inventory-full cube make info chat: %v", err)
	}
	if fullInfo.Type != chatproto.ChatTypeInfo || fullInfo.Message != itemPickupInventoryFullInfoMessage {
		t.Fatalf("unexpected inventory-full cube make info chat: %+v", fullInfo)
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "cube-make full owner")
}

func cubeMakePercent75RecipeSnapshot() cubestore.Snapshot {
	return cubestore.Snapshot{NPCs: []cubestore.NPCRecipes{{
		NPCVnum: bootstrapCubeOpenDefaultNPCVnum,
		Recipes: []cubestore.Recipe{{
			Reward: cubestore.Reward{Vnum: 27001, Count: 1},
			Materials: []cubestore.Material{
				{Vnum: 27002, Count: 2},
			},
			Gold:    100,
			Percent: 75,
		}},
	}}}
}

func TestGameRuntimeCubeMakePercent75RollSuccessConsumesGrantsPersistsAndEmitsBurst(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeMakeRollOK", 0x010308ca, 0x020408ca, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1920, Vnum: 27002, Count: 1, Slot: 5},
		{ID: 1921, Vnum: 27002, Count: 1, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
	}
	login := "cube-make-roll-ok"
	issuePeerTicket(t, ticketStore, login, 0x707071ca, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-make roll-success owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, cubeMakeItemTemplates(t), nil)
	if err != nil {
		t.Fatalf("unexpected cube-make roll-success runtime error: %v", err)
	}
	runtime.cubeRecipes = cubeMakePercent75RecipeSnapshot()
	restore := QueueCubeMakeRollForTest(75)
	defer restore()
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071ca)
	defer closeSessionFlow(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube before roll-success make: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_cube before roll-success make to emit one frame, got %d", len(openOut))
	}
	for _, message := range []string{"/cube add 0 5", "/cube add 1 6"} {
		addOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
			Type:    chatproto.ChatTypeTalking,
			Message: message,
		})))
		if err != nil {
			t.Fatalf("unexpected %s before roll-success make: %v", message, err)
		}
		if len(addOut) != 1 {
			t.Fatalf("expected %s before roll-success make to emit one frame, got %d", message, len(addOut))
		}
	}

	makeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube make",
	})))
	if err != nil {
		t.Fatalf("unexpected roll-success /cube make error: %v", err)
	}
	if len(makeOut) != 8 {
		t.Fatalf("expected roll-success /cube make burst of 8 frames, got %d", len(makeOut))
	}
	assertCubeCommandChatFrame(t, makeOut[6], "cube success 27001 1", "cube make roll-success")
	assertCubeCommandChatFrame(t, makeOut[7], "cube info 0 0 0", "post roll-success cube info")

	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted cube-make roll-success account: %v", err)
	}
	if persisted.Characters[0].Gold != 4142 {
		t.Fatalf("unexpected persisted gold after roll-success make: %d", persisted.Characters[0].Gold)
	}
	if len(persisted.Characters[0].Inventory) != 1 || persisted.Characters[0].Inventory[0].Vnum != 27001 || persisted.Characters[0].Inventory[0].Count != 1 {
		t.Fatalf("unexpected persisted inventory after roll-success make: %+v", persisted.Characters[0].Inventory)
	}
}

func TestGameRuntimeCubeMakePercent75RollFailureConsumesAndEmitsCubeFail(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeMakeRollFail", 0x010308cb, 0x020408cb, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1930, Vnum: 27002, Count: 1, Slot: 5},
		{ID: 1931, Vnum: 27002, Count: 1, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 6},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
	}
	login := "cube-make-roll-fail"
	issuePeerTicket(t, ticketStore, login, 0x707071cb, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-make roll-fail owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, cubeMakeItemTemplates(t), nil)
	if err != nil {
		t.Fatalf("unexpected cube-make roll-fail runtime error: %v", err)
	}
	runtime.cubeRecipes = cubeMakePercent75RecipeSnapshot()
	restore := QueueCubeMakeRollForTest(76)
	defer restore()
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071cb)
	defer closeSessionFlow(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube before roll-fail make: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_cube before roll-fail make to emit one frame, got %d", len(openOut))
	}
	for _, message := range []string{"/cube add 0 5", "/cube add 1 6"} {
		addOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
			Type:    chatproto.ChatTypeTalking,
			Message: message,
		})))
		if err != nil {
			t.Fatalf("unexpected %s before roll-fail make: %v", message, err)
		}
		if len(addOut) != 1 {
			t.Fatalf("expected %s before roll-fail make to emit one frame, got %d", message, len(addOut))
		}
	}

	makeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube make",
	})))
	if err != nil {
		t.Fatalf("unexpected roll-fail /cube make error: %v", err)
	}
	if len(makeOut) != 8 {
		t.Fatalf("expected roll-fail /cube make burst of 8 frames, got %d", len(makeOut))
	}
	delA, err := itemproto.DecodeDel(decodeSingleFrame(t, makeOut[0]))
	if err != nil {
		t.Fatalf("decode roll-fail material A ITEM_DEL: %v", err)
	}
	if delA.Position.WindowType != itemproto.WindowInventory || delA.Position.Cell != 5 {
		t.Fatalf("unexpected roll-fail material A delete position: %+v", delA.Position)
	}
	delB, err := itemproto.DecodeDel(decodeSingleFrame(t, makeOut[1]))
	if err != nil {
		t.Fatalf("decode roll-fail material B ITEM_DEL: %v", err)
	}
	if delB.Position.WindowType != itemproto.WindowInventory || delB.Position.Cell != 6 {
		t.Fatalf("unexpected roll-fail material B delete position: %+v", delB.Position)
	}
	quickslotDelA, err := quickslotproto.DecodeDel(decodeSingleFrame(t, makeOut[2]))
	if err != nil {
		t.Fatalf("decode roll-fail material A QUICKSLOT_DEL: %v", err)
	}
	if quickslotDelA.Position != 2 {
		t.Fatalf("unexpected roll-fail material A quickslot delete position: %+v", quickslotDelA)
	}
	quickslotDelB, err := quickslotproto.DecodeDel(decodeSingleFrame(t, makeOut[3]))
	if err != nil {
		t.Fatalf("decode roll-fail material B QUICKSLOT_DEL: %v", err)
	}
	if quickslotDelB.Position != 4 {
		t.Fatalf("unexpected roll-fail material B quickslot delete position: %+v", quickslotDelB)
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, makeOut[4]))
	if err != nil {
		t.Fatalf("decode roll-fail gold PLAYER_POINT_CHANGE: %v", err)
	}
	if goldChange.VID != owner.VID || goldChange.Type != bootstrapGoldPointType || goldChange.Amount != -100 || goldChange.Value != 4142 {
		t.Fatalf("unexpected roll-fail gold point change: %+v", goldChange)
	}
	failInfo, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, makeOut[5]))
	if err != nil {
		t.Fatalf("decode roll-fail info chat: %v", err)
	}
	if failInfo.Type != chatproto.ChatTypeInfo || failInfo.Message != cubeMakeFailedInfoMessage {
		t.Fatalf("unexpected roll-fail info chat: %+v", failInfo)
	}
	assertCubeCommandChatFrame(t, makeOut[6], cubestore.FormatCubeFailCommand(), "cube make roll-fail")
	assertCubeCommandChatFrame(t, makeOut[7], "cube info 0 0 0", "post roll-fail cube info")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected roll-fail /cube make to queue no peer frames, got %d", len(queued))
	}

	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted cube-make roll-fail account: %v", err)
	}
	if persisted.Characters[0].Gold != 4142 {
		t.Fatalf("unexpected persisted gold after roll-fail make: %d", persisted.Characters[0].Gold)
	}
	if len(persisted.Characters[0].Inventory) != 0 {
		t.Fatalf("unexpected persisted inventory after roll-fail make: %+v", persisted.Characters[0].Inventory)
	}
	wantQuickslots := []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeSkill, Slot: 6}}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted quickslots after roll-fail make: %+v want %+v", persisted.Characters[0].Quickslots, wantQuickslots)
	}
}

func TestGameRuntimeCubeMakePercent100IgnoresQueuedRoll(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeMakeNoRoll", 0x010308cc, 0x020408cc, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1940, Vnum: 27002, Count: 2, Slot: 5},
	}
	login := "cube-make-no-roll"
	issuePeerTicket(t, ticketStore, login, 0x707071cc, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-make no-roll owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, cubeMakeItemTemplates(t), nil)
	if err != nil {
		t.Fatalf("unexpected cube-make no-roll runtime error: %v", err)
	}
	restore := QueueCubeMakeRollForTest(1)
	defer restore()
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071cc)
	defer closeSessionFlow(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	}))); err != nil {
		t.Fatalf("unexpected /open_cube before no-roll make: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube add 0 5",
	}))); err != nil {
		t.Fatalf("unexpected /cube add before no-roll make: %v", err)
	}
	makeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube make",
	})))
	if err != nil {
		t.Fatalf("unexpected percent-100 /cube make with queued roll: %v", err)
	}
	if len(makeOut) < 2 {
		t.Fatalf("expected percent-100 /cube make to emit success burst, got %d frames", len(makeOut))
	}
	assertCubeCommandChatFrame(t, makeOut[len(makeOut)-2], "cube success 27001 1", "percent-100 ignores queued roll")
	cubeMakeRollMu.Lock()
	leftover := append([]int(nil), cubeMakeRollOverride...)
	cubeMakeRollMu.Unlock()
	if len(leftover) != 1 || leftover[0] != 1 {
		t.Fatalf("expected percent-100 make to leave queued roll untouched, got %+v", leftover)
	}
}

func TestGameRuntimeCubeMakePercent75RejectsOutOfRangeRollWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeMakeBadRoll", 0x010308cd, 0x020408cd, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1950, Vnum: 27002, Count: 2, Slot: 5},
	}
	login := "cube-make-bad-roll"
	issuePeerTicket(t, ticketStore, login, 0x707071cd, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-make bad-roll owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, cubeMakeItemTemplates(t), nil)
	if err != nil {
		t.Fatalf("unexpected cube-make bad-roll runtime error: %v", err)
	}
	runtime.cubeRecipes = cubeMakePercent75RecipeSnapshot()
	restore := QueueCubeMakeRollForTest(0)
	defer restore()
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071cd)
	defer closeSessionFlow(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	}))); err != nil {
		t.Fatalf("unexpected /open_cube before bad-roll make: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube add 0 5",
	}))); err != nil {
		t.Fatalf("unexpected /cube add before bad-roll make: %v", err)
	}
	makeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube make",
	})))
	if err != nil {
		t.Fatalf("unexpected bad-roll /cube make error: %v", err)
	}
	if len(makeOut) != 0 {
		t.Fatalf("expected out-of-range roll /cube make to emit no frames, got %d", len(makeOut))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "cube-make bad-roll owner")
}

func TestGameRuntimeCubeMakeAllLoopsTwoPercent100CraftsThenStopsOnUnmatched(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeMakeAllOwner", 0x010308d1, 0x020408d1, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1940, Vnum: 27002, Count: 2, Slot: 5},
		{ID: 1941, Vnum: 27002, Count: 2, Slot: 6},
	}
	login := "cube-make-all-owner"
	issuePeerTicket(t, ticketStore, login, 0x707071d1, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-make-all owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, cubeMakeItemTemplates(t), nil)
	if err != nil {
		t.Fatalf("unexpected cube-make-all runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071d1)
	defer closeSessionFlow(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	}))); err != nil {
		t.Fatalf("unexpected /open_cube before make all: %v", err)
	}
	for _, message := range []string{"/cube add 0 5", "/cube add 1 6"} {
		addOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
			Type:    chatproto.ChatTypeTalking,
			Message: message,
		})))
		if err != nil {
			t.Fatalf("unexpected %s before make all: %v", message, err)
		}
		if len(addOut) != 1 {
			t.Fatalf("expected %s before make all to emit one frame, got %d", message, len(addOut))
		}
	}

	makeAllOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube make all",
	})))
	if err != nil {
		t.Fatalf("unexpected /cube make all error: %v", err)
	}
	successCount := 0
	stopInfoSeen := false
	for _, frame := range makeAllOut {
		delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, frame))
		if err != nil {
			continue
		}
		if delivery.Type == chatproto.ChatTypeCommand && delivery.Message == cubestore.FormatCubeSuccessCommand(27001, 1) {
			successCount++
		}
		if delivery.Type == chatproto.ChatTypeInfo && delivery.Message == cubeMakeInsufficientMaterialsInfoMessage {
			stopInfoSeen = true
		}
	}
	if successCount != 2 {
		t.Fatalf("expected /cube make all to emit two cube success commands, got %d in %d frames", successCount, len(makeAllOut))
	}
	if !stopInfoSeen {
		t.Fatalf("expected /cube make all to end with unmatched-materials info chat")
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected /cube make all to queue no peer frames, got %d", len(queued))
	}

	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted cube-make-all account: %v", err)
	}
	if persisted.Characters[0].Gold != 4042 {
		t.Fatalf("unexpected persisted gold after make all: %d", persisted.Characters[0].Gold)
	}
	if len(persisted.Characters[0].Inventory) != 1 || persisted.Characters[0].Inventory[0].Vnum != 27001 || persisted.Characters[0].Inventory[0].Count != 2 {
		t.Fatalf("unexpected persisted inventory after make all: %+v", persisted.Characters[0].Inventory)
	}
}

func TestGameRuntimeCubeMakeAllStopsAfterFailRoll(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeMakeAllFail", 0x010308d2, 0x020408d2, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1950, Vnum: 27002, Count: 2, Slot: 5},
		{ID: 1951, Vnum: 27002, Count: 2, Slot: 6},
	}
	login := "cube-make-all-fail"
	issuePeerTicket(t, ticketStore, login, 0x707071d2, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-make-all fail owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, cubeMakeItemTemplates(t), nil)
	if err != nil {
		t.Fatalf("unexpected cube-make-all fail runtime error: %v", err)
	}
	runtime.cubeRecipes = cubeMakePercent75RecipeSnapshot()
	restore := QueueCubeMakeRollForTest(76)
	defer restore()
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071d2)
	defer closeSessionFlow(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	}))); err != nil {
		t.Fatalf("unexpected /open_cube before make-all fail: %v", err)
	}
	for _, message := range []string{"/cube add 0 5", "/cube add 1 6"} {
		if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
			Type:    chatproto.ChatTypeTalking,
			Message: message,
		}))); err != nil {
			t.Fatalf("unexpected %s before make-all fail: %v", message, err)
		}
	}

	makeAllOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube make all",
	})))
	if err != nil {
		t.Fatalf("unexpected fail-roll /cube make all error: %v", err)
	}
	// One fail burst: 2 ITEM_UPDATE/DEL + optional quickslot dels + gold + info + cube fail + cube info.
	// Materials are two count-2 stacks consuming 2 total => each stack updates (or one empties depending on consume order).
	// Mirror one-shot fail test shape: use command markers rather than exact length from uncertain quickslots.
	if len(makeAllOut) < 4 {
		t.Fatalf("expected fail-roll /cube make all to emit a fail burst, got %d frames", len(makeAllOut))
	}
	foundFail := false
	foundSuccess := false
	for _, frame := range makeAllOut {
		delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, frame))
		if err != nil {
			continue
		}
		if delivery.Type == chatproto.ChatTypeCommand && delivery.Message == cubestore.FormatCubeFailCommand() {
			foundFail = true
		}
		if delivery.Type == chatproto.ChatTypeCommand && delivery.Message == cubestore.FormatCubeSuccessCommand(27001, 1) {
			foundSuccess = true
		}
	}
	if !foundFail {
		t.Fatalf("expected fail-roll /cube make all to include cube fail command")
	}
	if foundSuccess {
		t.Fatalf("expected fail-roll /cube make all to stop without a later success")
	}

	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted cube-make-all fail account: %v", err)
	}
	if persisted.Characters[0].Gold != 4142 {
		t.Fatalf("unexpected persisted gold after fail-roll make all: %d", persisted.Characters[0].Gold)
	}
	totalMaterial := 0
	for _, item := range persisted.Characters[0].Inventory {
		if item.Vnum == 27002 {
			totalMaterial += int(item.Count)
		}
		if item.Vnum == 27001 {
			t.Fatalf("unexpected reward after fail-roll make all: %+v", persisted.Characters[0].Inventory)
		}
	}
	if totalMaterial != 2 {
		t.Fatalf("unexpected remaining material count after fail-roll make all: %d inventory=%+v", totalMaterial, persisted.Characters[0].Inventory)
	}
}

func cubeMakePercent0RecipeSnapshot() cubestore.Snapshot {
	return cubestore.Snapshot{NPCs: []cubestore.NPCRecipes{{
		NPCVnum: bootstrapCubeOpenDefaultNPCVnum,
		Recipes: []cubestore.Recipe{{
			Reward: cubestore.Reward{Vnum: 27001, Count: 1},
			Materials: []cubestore.Material{
				{Vnum: 27002, Count: 2},
			},
			Gold:    100,
			Percent: 0,
		}},
	}}}
}

func TestGameRuntimeCubeMakePercent0AlwaysFailsWithoutRoll(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeMakePct0", 0x010308d3, 0x020408d3, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1960, Vnum: 27002, Count: 1, Slot: 5},
		{ID: 1961, Vnum: 27002, Count: 1, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
	}
	login := "cube-make-pct0"
	issuePeerTicket(t, ticketStore, login, 0x707071d3, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-make percent-0 owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, cubeMakeItemTemplates(t), nil)
	if err != nil {
		t.Fatalf("unexpected cube-make percent-0 runtime error: %v", err)
	}
	runtime.cubeRecipes = cubeMakePercent0RecipeSnapshot()
	restore := QueueCubeMakeRollForTest(1)
	defer restore()
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071d3)
	defer closeSessionFlow(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	}))); err != nil {
		t.Fatalf("unexpected /open_cube before percent-0 make: %v", err)
	}
	for _, message := range []string{"/cube add 0 5", "/cube add 1 6"} {
		if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
			Type:    chatproto.ChatTypeTalking,
			Message: message,
		}))); err != nil {
			t.Fatalf("unexpected %s before percent-0 make: %v", message, err)
		}
	}

	makeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube make",
	})))
	if err != nil {
		t.Fatalf("unexpected percent-0 /cube make error: %v", err)
	}
	if len(makeOut) != 8 {
		t.Fatalf("expected percent-0 /cube make fail burst of 8 frames, got %d", len(makeOut))
	}
	failInfo, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, makeOut[5]))
	if err != nil {
		t.Fatalf("decode percent-0 fail info chat: %v", err)
	}
	if failInfo.Type != chatproto.ChatTypeInfo || failInfo.Message != cubeMakeFailedInfoMessage {
		t.Fatalf("unexpected percent-0 fail info chat: %+v", failInfo)
	}
	assertCubeCommandChatFrame(t, makeOut[6], cubestore.FormatCubeFailCommand(), "cube make percent-0")
	assertCubeCommandChatFrame(t, makeOut[7], "cube info 0 0 0", "post percent-0 cube info")
	cubeMakeRollMu.Lock()
	leftover := append([]int(nil), cubeMakeRollOverride...)
	cubeMakeRollMu.Unlock()
	if len(leftover) != 1 || leftover[0] != 1 {
		t.Fatalf("expected percent-0 make to leave queued roll untouched, got %+v", leftover)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected percent-0 /cube make to queue no peer frames, got %d", len(queued))
	}

	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted cube-make percent-0 account: %v", err)
	}
	if persisted.Characters[0].Gold != 4142 {
		t.Fatalf("unexpected persisted gold after percent-0 make: %d", persisted.Characters[0].Gold)
	}
	if len(persisted.Characters[0].Inventory) != 0 {
		t.Fatalf("unexpected persisted inventory after percent-0 make: %+v", persisted.Characters[0].Inventory)
	}
}

func TestGameRuntimeCubeMakeAllStopsAfterPercent0AlwaysFail(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeMakeAllPct0", 0x010308d4, 0x020408d4, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1970, Vnum: 27002, Count: 2, Slot: 5},
		{ID: 1971, Vnum: 27002, Count: 2, Slot: 6},
	}
	login := "cube-make-all-pct0"
	issuePeerTicket(t, ticketStore, login, 0x707071d4, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-make-all percent-0 owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, cubeMakeItemTemplates(t), nil)
	if err != nil {
		t.Fatalf("unexpected cube-make-all percent-0 runtime error: %v", err)
	}
	runtime.cubeRecipes = cubeMakePercent0RecipeSnapshot()
	restore := QueueCubeMakeRollForTest(1)
	defer restore()
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071d4)
	defer closeSessionFlow(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	}))); err != nil {
		t.Fatalf("unexpected /open_cube before make-all percent-0: %v", err)
	}
	for _, message := range []string{"/cube add 0 5", "/cube add 1 6"} {
		if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
			Type:    chatproto.ChatTypeTalking,
			Message: message,
		}))); err != nil {
			t.Fatalf("unexpected %s before make-all percent-0: %v", message, err)
		}
	}

	makeAllOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube make all",
	})))
	if err != nil {
		t.Fatalf("unexpected percent-0 /cube make all error: %v", err)
	}
	if len(makeAllOut) < 4 {
		t.Fatalf("expected percent-0 /cube make all to emit a fail burst, got %d frames", len(makeAllOut))
	}
	foundFail := false
	foundSuccess := false
	for _, frame := range makeAllOut {
		delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, frame))
		if err != nil {
			continue
		}
		if delivery.Type == chatproto.ChatTypeCommand && delivery.Message == cubestore.FormatCubeFailCommand() {
			foundFail = true
		}
		if delivery.Type == chatproto.ChatTypeCommand && delivery.Message == cubestore.FormatCubeSuccessCommand(27001, 1) {
			foundSuccess = true
		}
	}
	if !foundFail {
		t.Fatalf("expected percent-0 /cube make all to include cube fail command")
	}
	if foundSuccess {
		t.Fatalf("expected percent-0 /cube make all to stop without a later success")
	}
	cubeMakeRollMu.Lock()
	leftover := append([]int(nil), cubeMakeRollOverride...)
	cubeMakeRollMu.Unlock()
	if len(leftover) != 1 || leftover[0] != 1 {
		t.Fatalf("expected percent-0 make all to leave queued roll untouched, got %+v", leftover)
	}

	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted cube-make-all percent-0 account: %v", err)
	}
	if persisted.Characters[0].Gold != 4142 {
		t.Fatalf("unexpected persisted gold after percent-0 make all: %d", persisted.Characters[0].Gold)
	}
	totalMaterial := 0
	for _, item := range persisted.Characters[0].Inventory {
		if item.Vnum == 27002 {
			totalMaterial += int(item.Count)
		}
		if item.Vnum == 27001 {
			t.Fatalf("unexpected reward after percent-0 make all: %+v", persisted.Characters[0].Inventory)
		}
	}
	if totalMaterial != 2 {
		t.Fatalf("unexpected remaining material count after percent-0 make all: %d inventory=%+v", totalMaterial, persisted.Characters[0].Inventory)
	}
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
