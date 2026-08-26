package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
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
