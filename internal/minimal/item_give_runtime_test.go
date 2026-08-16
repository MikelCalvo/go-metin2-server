package minimal

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
)

func TestGameRuntimeItemGiveFailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GiveOwner", 0x01030740, 0x02040740, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 501, Vnum: 27001, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "item-give-owner", 0x70707040, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-give-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-give account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected item-give runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-owner", 0x70707040)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{TargetVID: 0x02040741, Position: itemproto.InventoryPosition(5), Count: 1})))
	if err != nil {
		t.Fatalf("unexpected item-give packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected unsupported ITEM_GIVE to emit no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after unsupported ITEM_GIVE, got %d", len(queued))
	}
	persisted, err := accounts.Load("item-give-owner")
	if err != nil {
		t.Fatalf("load persisted item-give account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("unsupported ITEM_GIVE mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("unsupported ITEM_GIVE mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameRuntimeItemGiveAntiGiveTemplateReturnsAuthoredRejectTextWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GiveBound", 0x01030741, 0x02040741, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 502, Vnum: 27042, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	target := peerVisibilityCharacter("GiveBoundTarget", 0x01030744, 0x02040744, 1120, 2120, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "item-give-bound", 0x70707041, owner)
	issuePeerTicket(t, ticketStore, "item-give-bound-target", 0x70707044, target)
	if err := accounts.Save(accountstore.Account{Login: "item-give-bound", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed bound item-give account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-give-bound-target", Empire: target.Empire, Characters: cloneCharacters([]loginticket.Character{target})}); err != nil {
		t.Fatalf("seed bound item-give target account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:           27042,
		Name:           "Bound Gift Potion",
		Stackable:      true,
		MaxCount:       200,
		AntiGive:       true,
		GiveRejectText: "You cannot give this item.",
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected bound item-give runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-bound", 0x70707041)
	defer closeSessionFlow(t, flow)
	targetFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-bound-target", 0x70707044)
	defer closeSessionFlow(t, targetFlow)
	_ = flushServerFrames(t, flow)
	_ = flushServerFrames(t, targetFlow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{TargetVID: target.VID, Position: itemproto.InventoryPosition(5), Count: 1})))
	if err != nil {
		t.Fatalf("unexpected anti-give item-give packet error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected anti-give ITEM_GIVE to emit one info-chat frame, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode anti-give item-give rejection info chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != template.GiveRejectText {
		t.Fatalf("unexpected anti-give item-give rejection chat: %+v", delivery)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after anti-give ITEM_GIVE rejection, got %d", len(queued))
	}
	if queued := flushServerFrames(t, targetFlow); len(queued) != 0 {
		t.Fatalf("expected visible target to receive no queued frames after anti-give ITEM_GIVE rejection, got %d", len(queued))
	}
	persisted, err := accounts.Load("item-give-bound")
	if err != nil {
		t.Fatalf("load persisted bound item-give account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("anti-give ITEM_GIVE mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("anti-give ITEM_GIVE mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
	assertExchangeAccountUnchanged(t, accounts, "item-give-bound-target", target, "anti-give ITEM_GIVE visible target")
}

func TestGameRuntimeItemGiveAntiGiveTemplateClosesActiveMerchantWindowWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := merchantBuyerCharacter("GiveMerchantBound", 0x01030749, 0x02040749, 12345, []inventory.ItemInstance{{ID: 507, Vnum: 27044, Count: 3, Slot: 5}})
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	target := peerVisibilityCharacter("GiveMerchantTarget", 0x0103074a, 0x0204074a, 1120, 2120, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "item-give-merchant-bound", 0x70707049, owner)
	issuePeerTicket(t, ticketStore, "item-give-merchant-target", 0x7070704a, target)
	if err := accounts.Save(accountstore.Account{Login: "item-give-merchant-bound", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed merchant-bound item-give account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-give-merchant-target", Empire: target.Empire, Characters: cloneCharacters([]loginticket.Character{target})}); err != nil {
		t.Fatalf("seed merchant-bound item-give target account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:           27044,
		Name:           "Merchant Bound Gift Potion",
		Stackable:      true,
		MaxCount:       200,
		AntiGive:       true,
		GiveRejectText: "You cannot give this item while shopping.",
	}
	templates := append(defaultMerchantItemTemplates(), template)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{defaultMerchantCatalogDefinition()})
	itemStore := newItemTemplateStore(t, templates)
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected merchant-bound item-give runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Merchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, "npc:merchant")
	if !ok {
		t.Fatal("expected merchant static actor registration to succeed")
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-merchant-bound", 0x70707049)
	defer closeSessionFlow(t, ownerFlow)
	targetFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-merchant-target", 0x7070704a)
	defer closeSessionFlow(t, targetFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, targetFlow)
	interactWithMerchantForBuy(t, ownerFlow, actor.EntityID)

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{TargetVID: target.VID, Position: itemproto.InventoryPosition(5), Count: 1})))
	if err != nil {
		t.Fatalf("unexpected merchant anti-give item-give packet error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected anti-give ITEM_GIVE to emit SHOP END plus info-chat frame, got %d", len(out))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, out[0])); err != nil {
		t.Fatalf("decode item-give merchant SHOP END before rejection info chat: %v", err)
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode merchant anti-give item-give rejection info chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != template.GiveRejectText {
		t.Fatalf("unexpected merchant anti-give item-give rejection chat: %+v", delivery)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no queued owner frames after merchant anti-give ITEM_GIVE rejection, got %d", len(queued))
	}
	if queued := flushServerFrames(t, targetFlow); len(queued) != 0 {
		t.Fatalf("expected visible target to receive no queued frames after merchant anti-give ITEM_GIVE rejection, got %d", len(queued))
	}

	closeOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected post-item-give merchant SHOP END error: %v", err)
	}
	if len(closeOut) != 0 {
		t.Fatalf("expected post-item-give merchant SHOP END to emit no frames after shell close, got %d", len(closeOut))
	}
	buyOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{RawLeadingByte: 1, CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected post-item-give merchant SHOP BUY error: %v", err)
	}
	if len(buyOut) != 0 {
		t.Fatalf("expected post-item-give merchant SHOP BUY to fail closed until reopen, got %d", len(buyOut))
	}
	assertExchangeAccountUnchanged(t, accounts, "item-give-merchant-bound", owner, "owner item-give merchant close")
	assertExchangeAccountUnchanged(t, accounts, "item-give-merchant-target", target, "target item-give merchant close")
}

func TestGameRuntimeItemGiveAntiGiveTemplateClosesActiveExchangeShellWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GiveExchangeBound", 0x01030747, 0x02040747, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 505, Vnum: 27043, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	target := peerVisibilityCharacter("GiveExchangeTarget", 0x01030748, 0x02040748, 1120, 2120, 0, 101, 201)
	target.Inventory = []inventory.ItemInstance{{ID: 506, Vnum: 27001, Count: 2, Slot: 6}}
	target.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	issuePeerTicket(t, ticketStore, "item-give-exchange-bound", 0x70707047, owner)
	issuePeerTicket(t, ticketStore, "item-give-exchange-target", 0x70707048, target)
	if err := accounts.Save(accountstore.Account{Login: "item-give-exchange-bound", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange-bound item-give account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-give-exchange-target", Empire: target.Empire, Characters: cloneCharacters([]loginticket.Character{target})}); err != nil {
		t.Fatalf("seed exchange-bound item-give target account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:           27043,
		Name:           "Exchange Bound Gift Potion",
		Stackable:      true,
		MaxCount:       200,
		AntiGive:       true,
		GiveRejectText: "You cannot give this item while trading.",
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange-bound item-give runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-exchange-bound", 0x70707047)
	defer closeSessionFlow(t, flow)
	targetFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-give-exchange-target", 0x70707048)
	defer closeSessionFlow(t, targetFlow)
	_ = flushServerFrames(t, flow)
	_ = flushServerFrames(t, targetFlow)

	startOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: target.VID})))
	if err != nil {
		t.Fatalf("unexpected item-give exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected item-give exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], target.VID, "item-give exchange owner start")
	queuedStart := flushServerFrames(t, targetFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected item-give exchange target start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "item-give exchange target start")

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{TargetVID: target.VID, Position: itemproto.InventoryPosition(5), Count: 1})))
	if err != nil {
		t.Fatalf("unexpected exchange anti-give item-give packet error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected anti-give ITEM_GIVE to emit exchange END plus info-chat frame, got %d", len(out))
	}
	assertExchangeEndFrame(t, out[0], "item-give exchange owner close")
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode exchange anti-give item-give rejection info chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != template.GiveRejectText {
		t.Fatalf("unexpected exchange anti-give item-give rejection chat: %+v", delivery)
	}
	queuedClose := flushServerFrames(t, targetFlow)
	if len(queuedClose) != 1 {
		t.Fatalf("expected item-give exchange target to receive one queued END, got %d", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "item-give exchange target close")

	cancelOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected post-item-give exchange cancel error: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected post-item-give exchange cancel to emit no frames after shell close, got %d", len(cancelOut))
	}
	assertExchangeAccountUnchanged(t, accounts, "item-give-exchange-bound", owner, "owner item-give exchange close")
	assertExchangeAccountUnchanged(t, accounts, "item-give-exchange-target", target, "target item-give exchange close")
}

func TestGameRuntimeItemGiveAntiGiveRejectTextRequiresVisibleTargetWithoutMutation(t *testing.T) {
	cases := []struct {
		name      string
		login     string
		key       uint32
		targetVID uint32
	}{
		{name: "zero target", login: "item-give-zero-target", key: 0x70707045, targetVID: 0},
		{name: "invisible target", login: "item-give-invisible-target", key: 0x70707046, targetVID: 0x02049999},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("GiveTargetGuard", 0x01030745, 0x02040745, 1100, 2100, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{{ID: 504, Vnum: 27042, Count: 3, Slot: 5}}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			issuePeerTicket(t, ticketStore, tc.login, tc.key, owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed %s item-give account: %v", tc.name, err)
			}
			template := itemcatalog.Template{
				Vnum:           27042,
				Name:           "Bound Gift Potion",
				Stackable:      true,
				MaxCount:       200,
				AntiGive:       true,
				GiveRejectText: "You cannot give this item.",
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected %s item-give runtime error: %v", tc.name, err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, tc.key)
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{TargetVID: tc.targetVID, Position: itemproto.InventoryPosition(5), Count: 1})))
			if err != nil {
				t.Fatalf("unexpected %s anti-give item-give packet error: %v", tc.name, err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s anti-give ITEM_GIVE to emit no frames, got %d", tc.name, len(out))
			}
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected no queued frames after %s anti-give ITEM_GIVE rejection, got %d", tc.name, len(queued))
			}
			persisted, err := accounts.Load(tc.login)
			if err != nil {
				t.Fatalf("load persisted %s item-give account: %v", tc.name, err)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("%s anti-give ITEM_GIVE mutated inventory: got %+v want %+v", tc.name, persisted.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("%s anti-give ITEM_GIVE mutated quickslots: got %+v want %+v", tc.name, persisted.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}

func TestGameRuntimeItemGiveAntiGiveRejectTextRequiresValidRequestedCountWithoutMutation(t *testing.T) {
	cases := []struct {
		name  string
		login string
		key   uint32
		count uint8
	}{
		{name: "zero count", login: "item-give-zero-count", key: 0x70707042, count: 0},
		{name: "oversized count", login: "item-give-oversized-count", key: 0x70707043, count: 4},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("GiveCountGuard", 0x01030742, 0x02040742, 1100, 2100, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{{ID: 503, Vnum: 27042, Count: 3, Slot: 5}}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			issuePeerTicket(t, ticketStore, tc.login, tc.key, owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed %s item-give account: %v", tc.name, err)
			}
			template := itemcatalog.Template{
				Vnum:           27042,
				Name:           "Bound Gift Potion",
				Stackable:      true,
				MaxCount:       200,
				AntiGive:       true,
				GiveRejectText: "You cannot give this item.",
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected %s item-give runtime error: %v", tc.name, err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, tc.key)
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{TargetVID: 0x02040744, Position: itemproto.InventoryPosition(5), Count: tc.count})))
			if err != nil {
				t.Fatalf("unexpected %s anti-give item-give packet error: %v", tc.name, err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s anti-give ITEM_GIVE to emit no frames, got %d", tc.name, len(out))
			}
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected no queued frames after %s anti-give ITEM_GIVE rejection, got %d", tc.name, len(queued))
			}
			persisted, err := accounts.Load(tc.login)
			if err != nil {
				t.Fatalf("load persisted %s item-give account: %v", tc.name, err)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("%s anti-give ITEM_GIVE mutated inventory: got %+v want %+v", tc.name, persisted.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("%s anti-give ITEM_GIVE mutated quickslots: got %+v want %+v", tc.name, persisted.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}
