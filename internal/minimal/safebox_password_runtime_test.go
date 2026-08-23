package minimal

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
)

func TestGameSessionFlowWarehousePasswordChallengeOpensOnDefaultPassword(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PasswordOpenOwner", 0x01030901, 0x02040901, 1100, 2100, 0, 101, 201)
	peer.Inventory = []inventory.ItemInstance{{ID: 701, Vnum: 27001, Count: 1, Slot: 3}}
	issuePeerTicket(t, store, "password-open-owner", 0x91919101, peer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "password-open-owner", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed password-open account: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind: interactionstore.KindOpenSafebox,
		Ref:  "npc:warehouse",
		Text: "The warehouse keeper unlocks the vault.",
		Size: 2,
	}})
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected password-open runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Warehouse", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindOpenSafebox, "npc:warehouse")
	if !ok {
		t.Fatal("expected warehouse registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "password-open-owner", 0x91919101)
	defer closeSessionFlow(t, flow)

	interactOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected warehouse interact: %v", err)
	}
	if len(interactOut) != 2 {
		t.Fatalf("expected chat + ShowMeSafeboxPassword, got %d", len(interactOut))
	}
	prompt, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, interactOut[1]))
	if err != nil {
		t.Fatalf("decode ShowMeSafeboxPassword: %v", err)
	}
	if prompt.Type != chatproto.ChatTypeCommand || prompt.Message != safeboxShowPasswordCommandMessage {
		t.Fatalf("unexpected password prompt: %+v", prompt)
	}

	// Pending challenge must not busy-block exchange START.
	startOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      0,
	})))
	if err != nil {
		t.Fatalf("unexpected exchange start during pending password: %v", err)
	}
	if len(startOut) != 0 {
		// No partner visible — fail-closed no frames is fine; just ensure no busy chat.
		for _, frame := range startOut {
			delivery, decodeErr := chatproto.DecodeChatDelivery(decodeSingleFrame(t, frame))
			if decodeErr == nil && delivery.Message == exchangeRequesterMerchantBusyInfoMessage {
				t.Fatalf("pending password challenge must not busy-block exchange START, got %+v", delivery)
			}
		}
	}

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password " + safeboxstore.DefaultPassword,
	})))
	if err != nil {
		t.Fatalf("unexpected default password open: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE after default password, got %d", len(openOut))
	}
	size, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, openOut[0]))
	if err != nil {
		t.Fatalf("decode SAFEBOX_SIZE after default password: %v", err)
	}
	if size != (itemproto.SafeboxSizePacket{Size: 2}) {
		t.Fatalf("unexpected SAFEBOX_SIZE after default password: %+v", size)
	}
}

func TestGameSessionFlowWarehousePasswordChallengeRejectsWrongPassword(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PasswordWrongOwner", 0x01030902, 0x02040902, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "password-wrong-owner", 0x91919102, peer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "password-wrong-owner", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed password-wrong account: %v", err)
	}
	safeboxPath := filepath.Join(t.TempDir(), "state", "safebox.json")
	safebox := safeboxstore.NewFileStore(safeboxPath)
	seeded, err := safeboxstore.ReplaceCharacterPassword(safeboxstore.Snapshot{}, "password-wrong-owner", peer.ID, "secret")
	if err != nil {
		t.Fatalf("seed custom password snapshot: %v", err)
	}
	if err := safebox.Save(seeded); err != nil {
		t.Fatalf("save custom password snapshot: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind: interactionstore.KindOpenSafebox,
		Ref:  "npc:warehouse",
		Size: 1,
	}})
	cfg := config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", SafeboxStorePath: safeboxPath}
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(cfg, store, accounts, interactionStore)
	if err != nil {
		t.Fatalf("unexpected password-wrong runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Warehouse", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindOpenSafebox, "npc:warehouse")
	if !ok {
		t.Fatal("expected warehouse registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "password-wrong-owner", 0x91919102)
	defer closeSessionFlow(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)}))); err != nil {
		t.Fatalf("unexpected warehouse interact before wrong password: %v", err)
	}
	wrongOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password 000000",
	})))
	if err != nil {
		t.Fatalf("unexpected wrong password attempt: %v", err)
	}
	if len(wrongOut) != 1 {
		t.Fatalf("expected SAFEBOX_WRONG_PASSWORD, got %d frames", len(wrongOut))
	}
	if _, err := itemproto.DecodeSafeboxWrongPassword(decodeSingleFrame(t, wrongOut[0])); err != nil {
		t.Fatalf("decode SAFEBOX_WRONG_PASSWORD: %v", err)
	}

	// Challenge cleared: later password without re-prompt stays fail-closed-consume.
	retryOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password secret",
	})))
	if err != nil {
		t.Fatalf("unexpected password retry without challenge: %v", err)
	}
	if len(retryOut) != 0 {
		t.Fatalf("expected no frames without pending challenge, got %d", len(retryOut))
	}
}

func TestGameSessionFlowWarehousePasswordChallengeRejectsMalformedPasswordWithInfoChat(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PasswordMalformedOwner", 0x01030903, 0x02040903, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "password-malformed-owner", 0x91919103, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind: interactionstore.KindOpenSafebox,
		Ref:  "npc:warehouse",
		Size: 1,
	}})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected password-malformed runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Warehouse", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindOpenSafebox, "npc:warehouse")
	if !ok {
		t.Fatal("expected warehouse registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "password-malformed-owner", 0x91919103)
	defer closeSessionFlow(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)}))); err != nil {
		t.Fatalf("unexpected warehouse interact before malformed password: %v", err)
	}
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password 1234567",
	})))
	if err != nil {
		t.Fatalf("unexpected malformed password attempt: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected one info chat for malformed password, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode malformed password chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.Message != safeboxPasswordWrongInfoMessage {
		t.Fatalf("unexpected malformed password chat: %+v", delivery)
	}
}

func TestGameRuntimeSlashOpenSafeboxStillBypassesPasswordChallenge(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SlashBypassOwner", 0x01030904, 0x02040904, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "slash-bypass-owner", 0x91919104, owner)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected slash-bypass runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "slash-bypass-owner", 0x91919104)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox 2",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox bypass: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected immediate SAFEBOX_SIZE from /open_safebox, got %d", len(out))
	}
	size, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode /open_safebox SAFEBOX_SIZE: %v", err)
	}
	if size != (itemproto.SafeboxSizePacket{Size: 2}) {
		t.Fatalf("unexpected /open_safebox SAFEBOX_SIZE: %+v", size)
	}

	alreadyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password 000000",
	})))
	if err != nil {
		t.Fatalf("unexpected already-open password attempt: %v", err)
	}
	if len(alreadyOut) != 1 {
		t.Fatalf("expected already-open info chat, got %d", len(alreadyOut))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, alreadyOut[0]))
	if err != nil {
		t.Fatalf("decode already-open chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.Message != safeboxAlreadyOpenInfoMessage {
		t.Fatalf("unexpected already-open chat: %+v", delivery)
	}
}

func TestGameSessionFlowWarehousePasswordOpensCustomPasswordAndRematerializesCells(t *testing.T) {
	root := t.TempDir()
	ticketStore := loginticket.NewFileStore(filepath.Join(root, "tickets"))
	accounts := accountstore.NewFileStore(filepath.Join(root, "accounts"))
	safeboxPath := filepath.Join(root, "state", "safebox.json")
	owner := peerVisibilityCharacter("PasswordRematerialize", 0x01030905, 0x02040905, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 801, Vnum: 27001, Count: 2, Slot: 4}}
	login := "password-rematerialize"
	const loginKey uint32 = 0x91919105
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed rematerialize account: %v", err)
	}
	safebox := safeboxstore.NewFileStore(safeboxPath)
	seeded, err := safeboxstore.ReplaceCharacterPassword(safeboxstore.Snapshot{}, login, owner.ID, "vault1")
	if err != nil {
		t.Fatalf("seed rematerialize password: %v", err)
	}
	seeded, err = safeboxstore.ReplaceCharacterCells(seeded, login, owner.ID, map[uint8]inventory.ItemInstance{
		0: {ID: 8801, Vnum: 27001, Count: 2, Slot: 0},
	})
	if err != nil {
		t.Fatalf("seed rematerialize cells: %v", err)
	}
	if err := safebox.Save(seeded); err != nil {
		t.Fatalf("save rematerialize safebox: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind: interactionstore.KindOpenSafebox,
		Ref:  "npc:warehouse",
		Size: 2,
	}})
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}})
	cfg := config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", SafeboxStorePath: safeboxPath}
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(cfg, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected rematerialize runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Warehouse", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindOpenSafebox, "npc:warehouse")
	if !ok {
		t.Fatal("expected warehouse registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)}))); err != nil {
		t.Fatalf("unexpected warehouse interact before custom password: %v", err)
	}
	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password vault1",
	})))
	if err != nil {
		t.Fatalf("unexpected custom password open: %v", err)
	}
	if len(openOut) != 3 {
		t.Fatalf("expected SAFEBOX_SIZE + SAFEBOX_SET + SAFEBOX_MONEY_CHANGE after custom password, got %d", len(openOut))
	}
	size, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, openOut[0]))
	if err != nil {
		t.Fatalf("decode rematerialize SAFEBOX_SIZE: %v", err)
	}
	if size != (itemproto.SafeboxSizePacket{Size: 2}) {
		t.Fatalf("unexpected rematerialize SAFEBOX_SIZE: %+v", size)
	}
	set, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, openOut[1]))
	if err != nil {
		t.Fatalf("decode rematerialize SAFEBOX_SET: %v", err)
	}
	if set.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) || set.Vnum != 27001 || set.Count != 2 {
		t.Fatalf("unexpected rematerialize SAFEBOX_SET: %+v", set)
	}
	money, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, openOut[2]))
	if err != nil {
		t.Fatalf("decode rematerialize SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if money != (itemproto.SafeboxMoneyChangePacket{Money: 0}) {
		t.Fatalf("unexpected rematerialize SAFEBOX_MONEY_CHANGE: %+v", money)
	}
}

func TestGameSessionFlowWarehousePasswordHonorsReopenCooldown(t *testing.T) {
	currentTime := time.Unix(1_700_000_000, 0).UTC()
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PasswordCooldownOwner", 0x01030911, 0x02040911, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "password-cooldown-owner", 0x91919111, peer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "password-cooldown-owner", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed password-cooldown account: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind: interactionstore.KindOpenSafebox,
		Ref:  "npc:warehouse",
		Size: 1,
	}})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, interactionStore)
	if err != nil {
		t.Fatalf("unexpected password-cooldown runtime error: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.RegisterStaticActorWithInteraction("Warehouse", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindOpenSafebox, "npc:warehouse")
	if !ok {
		t.Fatal("expected warehouse registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "password-cooldown-owner", 0x91919111)
	defer closeSessionFlow(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)}))); err != nil {
		t.Fatalf("unexpected warehouse interact before first open: %v", err)
	}
	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password " + safeboxstore.DefaultPassword,
	})))
	if err != nil {
		t.Fatalf("unexpected first password open: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE on first open, got %d", len(openOut))
	}
	assertCloseSafeboxCommandChat(t, flow, "/close_safebox", "password-cooldown close")

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)}))); err != nil {
		t.Fatalf("unexpected warehouse interact during cooldown: %v", err)
	}
	blockedOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password " + safeboxstore.DefaultPassword,
	})))
	if err != nil {
		t.Fatalf("unexpected cooldown password attempt: %v", err)
	}
	if len(blockedOut) != 1 {
		t.Fatalf("expected cooldown info chat, got %d", len(blockedOut))
	}
	blocked, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, blockedOut[0]))
	if err != nil {
		t.Fatalf("decode cooldown chat: %v", err)
	}
	if blocked.Type != chatproto.ChatTypeInfo || blocked.Message != safeboxReopenCooldownInfoMessage {
		t.Fatalf("unexpected cooldown chat: %+v", blocked)
	}

	labOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected lab /open_safebox during cooldown: %v", err)
	}
	if len(labOut) != 2 {
		t.Fatalf("expected lab /open_safebox to stay exempt from cooldown, got %d frames", len(labOut))
	}
	assertCloseSafeboxCommandChat(t, flow, "/close_safebox", "password-cooldown lab close")

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)}))); err != nil {
		t.Fatalf("unexpected warehouse interact after lab close: %v", err)
	}
	currentTime = currentTime.Add(bootstrapSafeboxReopenCooldown)
	openAgainOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password " + safeboxstore.DefaultPassword,
	})))
	if err != nil {
		t.Fatalf("unexpected password open after cooldown: %v", err)
	}
	if len(openAgainOut) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE after cooldown, got %d", len(openAgainOut))
	}
}

func TestGameSessionFlowWarehousePasswordHonorsOpenAnchorDistance(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PasswordDistanceOwner", 0x01030912, 0x02040912, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "password-distance-owner", 0x91919112, peer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "password-distance-owner", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed password-distance account: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind: interactionstore.KindOpenSafebox,
		Ref:  "npc:warehouse",
		Size: 1,
	}})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, interactionStore)
	if err != nil {
		t.Fatalf("unexpected password-distance runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Warehouse", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindOpenSafebox, "npc:warehouse")
	if !ok {
		t.Fatal("expected warehouse registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "password-distance-owner", 0x91919112)
	defer closeSessionFlow(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)}))); err != nil {
		t.Fatalf("unexpected warehouse interact before walk-away: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 3100, Y: 2100, Time: 0x21222329}))); err != nil {
		t.Fatalf("unexpected walk-away move before password: %v", err)
	}
	tooFarOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password " + safeboxstore.DefaultPassword,
	})))
	if err != nil {
		t.Fatalf("unexpected too-far password attempt: %v", err)
	}
	if len(tooFarOut) != 1 {
		t.Fatalf("expected too-far info chat, got %d", len(tooFarOut))
	}
	tooFar, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, tooFarOut[0]))
	if err != nil {
		t.Fatalf("decode too-far chat: %v", err)
	}
	if tooFar.Type != chatproto.ChatTypeInfo || tooFar.Message != safeboxOpenTooFarInfoMessage {
		t.Fatalf("unexpected too-far chat: %+v", tooFar)
	}

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1100, Y: 2100, Time: 0x2122232a}))); err != nil {
		t.Fatalf("unexpected walk-back move before password: %v", err)
	}
	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password " + safeboxstore.DefaultPassword,
	})))
	if err != nil {
		t.Fatalf("unexpected password open after walk-back: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE after walk-back, got %d", len(openOut))
	}
}
