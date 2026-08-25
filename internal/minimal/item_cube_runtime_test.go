package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
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
