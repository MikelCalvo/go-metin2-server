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
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
)

func TestGameRuntimeItemRefineFailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineOwner", 0x01030750, 0x02040750, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 25
	owner.Inventory = []inventory.ItemInstance{{ID: 601, Vnum: 11200, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "item-refine-owner", 0x70707050, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-refine-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-refine account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected item-refine runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-refine-owner", 0x70707050)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 2})))
	if err != nil {
		t.Fatalf("unexpected item-refine packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected unsupported REFINE to emit no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after unsupported REFINE, got %d", len(queued))
	}
	persisted, err := accounts.Load("item-refine-owner")
	if err != nil {
		t.Fatalf("load persisted item-refine account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("unsupported REFINE mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("unsupported REFINE mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("unsupported REFINE mutated point value: got %d want %d", persisted.Characters[0].Points[bootstrapPlayerPointValueIndex], owner.Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameRuntimeItemRefineTemplateRejectMessageWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineBound", 0x01030751, 0x02040751, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 25
	owner.Inventory = []inventory.ItemInstance{{ID: 602, Vnum: 11201, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "item-refine-bound", 0x70707051, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-refine-bound", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-refine bound account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:             11201,
		Name:             "Non Refine Practice Blade",
		Stackable:        false,
		MaxCount:         1,
		RefineRejectText: "This item cannot be refined yet.",
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-refine bound runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-refine-bound", 0x70707051)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 2})))
	if err != nil {
		t.Fatalf("unexpected item-refine template rejection packet error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected template-backed REFINE rejection to emit one info-chat frame, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode template-backed item-refine rejection info chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != template.RefineRejectText {
		t.Fatalf("unexpected template-backed item-refine rejection chat: %+v", delivery)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after template-backed REFINE rejection, got %d", len(queued))
	}
	persisted, err := accounts.Load("item-refine-bound")
	if err != nil {
		t.Fatalf("load persisted item-refine bound account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("template-backed REFINE rejection mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("template-backed REFINE rejection mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("template-backed REFINE rejection mutated point value: got %d want %d", persisted.Characters[0].Points[bootstrapPlayerPointValueIndex], owner.Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameRuntimeItemRefineRejectMessageClosesActiveMerchantWindowWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := merchantBuyerCharacter("RefineMerchantReject", 0x01030754, 0x02040754, 12345, []inventory.ItemInstance{{ID: 605, Vnum: 11205, Count: 1, Slot: 5}})
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "item-refine-merchant-reject", 0x70707054, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-refine-merchant-reject", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-refine merchant reject account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:             11205,
		Name:             "Merchant Non Refine Practice Blade",
		Stackable:        false,
		MaxCount:         1,
		RefineRejectText: "This item cannot be refined while shopping.",
	}
	templates := append(defaultMerchantItemTemplates(), template)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{defaultMerchantCatalogDefinition()})
	itemStore := newItemTemplateStore(t, templates)
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected item-refine merchant reject runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Merchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, "npc:merchant")
	if !ok {
		t.Fatal("expected merchant static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-refine-merchant-reject", 0x70707054)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)
	interactWithMerchantForBuy(t, flow, actor.EntityID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 2})))
	if err != nil {
		t.Fatalf("unexpected item-refine merchant rejection packet error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected template-backed REFINE rejection to close SHOP before info chat, got %d frames", len(out))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, out[0])); err != nil {
		t.Fatalf("decode merchant SHOP END before refine rejection chat: %v", err)
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode merchant refine rejection chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != template.RefineRejectText {
		t.Fatalf("unexpected merchant refine rejection chat: %+v", delivery)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after merchant refine rejection, got %d", len(queued))
	}
	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected post-refine-reject merchant SHOP END error: %v", err)
	}
	if len(closeOut) != 0 {
		t.Fatalf("expected post-refine-reject merchant SHOP END to emit no frames after refine closed the shell, got %d", len(closeOut))
	}
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{RawLeadingByte: 1, CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected post-refine-reject merchant SHOP BUY error: %v", err)
	}
	if len(buyOut) != 0 {
		t.Fatalf("expected post-refine-reject merchant SHOP BUY to fail closed until reopen, got %d", len(buyOut))
	}
	persisted, err := accounts.Load("item-refine-merchant-reject")
	if err != nil {
		t.Fatalf("load persisted item-refine merchant reject account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) || !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) || persisted.Characters[0].Gold != owner.Gold || persisted.Characters[0].Points != owner.Points {
		t.Fatalf("merchant refine rejection mutated persisted character:\n got: %+v\nwant: %+v", persisted.Characters[0], owner)
	}
}

func TestGameRuntimeItemRefineInformationClosesActiveMerchantWindowWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := merchantBuyerCharacter("RefineMerchantOpen", 0x01030753, 0x02040753, 12345, []inventory.ItemInstance{{ID: 604, Vnum: 11203, Count: 1, Slot: 5}})
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "item-refine-merchant-open", 0x70707053, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-refine-merchant-open", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-refine merchant account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:       11203,
		Name:       "Merchant Refineable Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{ResultVnum: 11204, Cost: 1250, Probability: 65, Materials: []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 1}}},
	}
	templates := append(defaultMerchantItemTemplates(), template)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{defaultMerchantCatalogDefinition()})
	itemStore := newItemTemplateStore(t, templates)
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected item-refine merchant runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Merchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, "npc:merchant")
	if !ok {
		t.Fatal("expected merchant static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-refine-merchant-open", 0x70707053)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)
	interactWithMerchantForBuy(t, flow, actor.EntityID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 4})))
	if err != nil {
		t.Fatalf("unexpected item-refine merchant information packet error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected template-backed REFINE information to close SHOP before preview, got %d frames", len(out))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, out[0])); err != nil {
		t.Fatalf("decode merchant SHOP END before refine information frame: %v", err)
	}
	packet, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode merchant refine information frame: %v", err)
	}
	want := itemproto.RefineInformationPacket{
		Type:     4,
		Position: 5,
		Table: itemproto.RefineTable{
			SourceVnum:    11203,
			ResultVnum:    11204,
			MaterialCount: 1,
			Cost:          1250,
			Probability:   65,
			Materials:     [itemproto.RefineMaterialMaxNum]itemproto.RefineMaterial{{Vnum: 27001, Count: 1}},
		},
	}
	if packet != want {
		t.Fatalf("unexpected merchant refine information frame:\n got: %+v\nwant: %+v", packet, want)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after merchant refine information, got %d", len(queued))
	}
	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected post-refine merchant SHOP END error: %v", err)
	}
	if len(closeOut) != 0 {
		t.Fatalf("expected post-refine merchant SHOP END to emit no frames after refine closed the shell, got %d", len(closeOut))
	}
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{RawLeadingByte: 1, CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected post-refine merchant SHOP BUY error: %v", err)
	}
	if len(buyOut) != 0 {
		t.Fatalf("expected post-refine merchant SHOP BUY to fail closed until reopen, got %d", len(buyOut))
	}
	persisted, err := accounts.Load("item-refine-merchant-open")
	if err != nil {
		t.Fatalf("load persisted item-refine merchant account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) || !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) || persisted.Characters[0].Gold != owner.Gold || persisted.Characters[0].Points != owner.Points {
		t.Fatalf("merchant refine information mutated persisted character:\n got: %+v\nwant: %+v", persisted.Characters[0], owner)
	}
}

func TestGameRuntimeItemRefineTemplateInformationFrameWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineOpen", 0x01030752, 0x02040752, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 25
	owner.Inventory = []inventory.ItemInstance{{ID: 603, Vnum: 11202, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "item-refine-open", 0x70707052, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-refine-open", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-refine open account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:       11202,
		Name:       "Refineable Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{
			ResultVnum:  11203,
			Cost:        2500,
			Probability: 75,
			Materials:   []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}, {Vnum: 27002, Count: 3}},
		},
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-refine open runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-refine-open", 0x70707052)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected item-refine information packet error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected template-backed REFINE information to emit one frame, got %d", len(out))
	}
	packet, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode template-backed refine information frame: %v", err)
	}
	want := itemproto.RefineInformationPacket{
		Type:     3,
		Position: 5,
		Table: itemproto.RefineTable{
			SourceVnum:    11202,
			ResultVnum:    11203,
			MaterialCount: 2,
			Cost:          2500,
			Probability:   75,
			Materials:     [itemproto.RefineMaterialMaxNum]itemproto.RefineMaterial{{Vnum: 27001, Count: 2}, {Vnum: 27002, Count: 3}},
		},
	}
	if packet != want {
		t.Fatalf("unexpected template-backed refine information frame:\n got: %+v\nwant: %+v", packet, want)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after template-backed REFINE information, got %d", len(queued))
	}
	persisted, err := accounts.Load("item-refine-open")
	if err != nil {
		t.Fatalf("load persisted item-refine open account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("template-backed REFINE information mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("template-backed REFINE information mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("template-backed REFINE information mutated point value: got %d want %d", persisted.Characters[0].Points[bootstrapPlayerPointValueIndex], owner.Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameRuntimeItemRefineRejectMessageClosesActiveExchangeShellWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineExchangeReject", 0x01030790, 0x02040790, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 620, Vnum: 11220, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("RefineExchangePeer", 0x01030791, 0x02040791, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 621, Vnum: 27001, Count: 2, Slot: 6}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	issuePeerTicket(t, ticketStore, "ref-ex-rej", 0x70707090, owner)
	issuePeerTicket(t, ticketStore, "ref-ex-r-peer", 0x70707091, peer)
	if err := accounts.Save(accountstore.Account{Login: "ref-ex-rej", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed refine exchange reject owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "ref-ex-r-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed refine exchange reject peer account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:             11220,
		Name:             "Non Refine Exchange Blade",
		Stackable:        false,
		MaxCount:         1,
		RefineRejectText: "This item cannot be refined while trading.",
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected refine exchange reject runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "ref-ex-rej", 0x70707090)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "ref-ex-r-peer", 0x70707091)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected refine exchange reject start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected refine exchange reject start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "refine reject owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected refine exchange reject peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "refine reject peer start")

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 2})))
	if err != nil {
		t.Fatalf("unexpected refine exchange reject packet error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected refine exchange reject to emit exchange END plus reject chat, got %d", len(out))
	}
	assertExchangeEndFrame(t, out[0], "refine reject owner close")
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode refine exchange reject chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != template.RefineRejectText {
		t.Fatalf("unexpected refine exchange reject chat: %+v", delivery)
	}
	queuedClose := flushServerFrames(t, peerFlow)
	if len(queuedClose) != 1 {
		t.Fatalf("expected refine exchange reject peer to receive one queued END, got %d", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "refine reject peer close")

	assertExchangeAccountUnchanged(t, accounts, "ref-ex-rej", owner, "owner refine exchange reject")
	assertExchangeAccountUnchanged(t, accounts, "ref-ex-r-peer", peer, "peer refine exchange reject")
}

func TestGameRuntimeItemRefineInformationClosesActiveExchangeShellWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineExchangeOpen", 0x01030792, 0x02040792, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 622, Vnum: 11222, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("RefineExchangeOpenPeer", 0x01030793, 0x02040793, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 623, Vnum: 27001, Count: 2, Slot: 6}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	issuePeerTicket(t, ticketStore, "ref-ex-open", 0x70707092, owner)
	issuePeerTicket(t, ticketStore, "ref-ex-o-peer", 0x70707093, peer)
	if err := accounts.Save(accountstore.Account{Login: "ref-ex-open", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed refine exchange open owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "ref-ex-o-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed refine exchange open peer account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:       11222,
		Name:       "Refineable Exchange Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{
			ResultVnum:  11223,
			Cost:        2500,
			Probability: 75,
			Materials:   []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}, {Vnum: 27002, Count: 3}},
		},
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected refine exchange open runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "ref-ex-open", 0x70707092)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "ref-ex-o-peer", 0x70707093)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected refine exchange open start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected refine exchange open start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "refine information owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected refine exchange information peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "refine information peer start")

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected refine exchange information packet error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected refine exchange information to emit exchange END plus information frame, got %d", len(out))
	}
	assertExchangeEndFrame(t, out[0], "refine information owner close")
	packet, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode refine exchange information frame: %v", err)
	}
	want := itemproto.RefineInformationPacket{
		Type:     3,
		Position: 5,
		Table: itemproto.RefineTable{
			SourceVnum:    11222,
			ResultVnum:    11223,
			MaterialCount: 2,
			Cost:          2500,
			Probability:   75,
			Materials:     [itemproto.RefineMaterialMaxNum]itemproto.RefineMaterial{{Vnum: 27001, Count: 2}, {Vnum: 27002, Count: 3}},
		},
	}
	if packet != want {
		t.Fatalf("unexpected refine exchange information frame:\n got: %+v\nwant: %+v", packet, want)
	}
	queuedClose := flushServerFrames(t, peerFlow)
	if len(queuedClose) != 1 {
		t.Fatalf("expected refine exchange information peer to receive one queued END, got %d", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "refine information peer close")

	assertExchangeAccountUnchanged(t, accounts, "ref-ex-open", owner, "owner refine exchange information")
	assertExchangeAccountUnchanged(t, accounts, "ref-ex-o-peer", peer, "peer refine exchange information")
}

func TestGameRuntimeItemRefineTemplateInformationGuardedTemplateFailsClosedWithoutMutation(t *testing.T) {
	cases := []struct {
		name       string
		login      string
		key        uint32
		ownerRace  uint16
		template   itemcatalog.Template
		wantReason string
	}{
		{name: "anti give", login: "item-refine-preview-anti-give", key: 0x70707053, template: guardedRefineRuntimeTemplate(11204, func(t *itemcatalog.Template) { t.AntiGive = true }), wantReason: "anti-give"},
		{name: "selected class", login: "item-refine-preview-class", key: 0x70707054, ownerRace: 0, template: guardedRefineRuntimeTemplate(11205, func(t *itemcatalog.Template) { t.AntiWarrior = true }), wantReason: "class"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("RefineGuarded", 0x01030753, 0x02040753, 1100, 2100, tc.ownerRace, 101, 201)
			owner.Points[bootstrapPlayerPointValueIndex] = 25
			owner.Inventory = []inventory.ItemInstance{{ID: 604, Vnum: tc.template.Vnum, Count: 1, Slot: 5}}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			issuePeerTicket(t, ticketStore, tc.login, tc.key, owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed %s item-refine account: %v", tc.name, err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{tc.template})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected %s item-refine runtime error: %v", tc.name, err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, tc.key)
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
			if err != nil {
				t.Fatalf("unexpected %s item-refine packet error: %v", tc.name, err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s guarded REFINE preview to emit no frames, got %d", tc.wantReason, len(out))
			}
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected no queued frames after %s guarded REFINE preview, got %d", tc.wantReason, len(queued))
			}
			persisted, err := accounts.Load(tc.login)
			if err != nil {
				t.Fatalf("load persisted %s item-refine account: %v", tc.name, err)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("%s guarded REFINE preview mutated inventory: got %+v want %+v", tc.wantReason, persisted.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("%s guarded REFINE preview mutated quickslots: got %+v want %+v", tc.wantReason, persisted.Characters[0].Quickslots, owner.Quickslots)
			}
			if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
				t.Fatalf("%s guarded REFINE preview mutated point value: got %d want %d", tc.wantReason, persisted.Characters[0].Points[bootstrapPlayerPointValueIndex], owner.Points[bootstrapPlayerPointValueIndex])
			}
		})
	}
}

func guardedRefineRuntimeTemplate(vnum uint32, mutate func(*itemcatalog.Template)) itemcatalog.Template {
	template := itemcatalog.Template{
		Vnum:       vnum,
		Name:       "Guarded Refineable Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{
			ResultVnum:  vnum + 1,
			Cost:        2500,
			Probability: 75,
			Materials:   []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}},
		},
	}
	mutate(&template)
	return template
}

func TestGameRuntimeItemRefineConfirmAfterPreviewProbability100PersistsAndEmitsBurst(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineConfirm", 0x01030760, 0x02040760, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Points[bootstrapPlayerPointValueIndex] = 25
	owner.Inventory = []inventory.ItemInstance{
		{ID: 630, Vnum: 11230, Count: 1, Slot: 5},
		{ID: 631, Vnum: 27001, Count: 2, Slot: 6},
		{ID: 632, Vnum: 27002, Count: 1, Slot: 7},
		{ID: 633, Vnum: 27002, Count: 4, Slot: 8},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 1, Type: quickslotproto.TypeSkill, Slot: 6},
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 7},
		{Position: 5, Type: quickslotproto.TypeItem, Slot: 8},
		{Position: 6, Type: quickslotproto.TypeCommand, Slot: 7},
	}
	issuePeerTicket(t, ticketStore, "item-refine-confirm", 0x70707060, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-refine-confirm", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-refine confirm account: %v", err)
	}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11230,
		Name:       "Confirmable Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{
			ResultVnum:  11231,
			Cost:        2500,
			Probability: 100,
			Materials:   []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}, {Vnum: 27002, Count: 3}},
		},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11231, Name: "Confirmed Practice Blade", Stackable: false, MaxCount: 1}
	materialA := itemcatalog.Template{Vnum: 27001, Name: "Refine Material A", Stackable: true, MaxCount: 200}
	materialB := itemcatalog.Template{Vnum: 27002, Name: "Refine Material B", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{sourceTemplate, resultTemplate, materialA, materialB})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-refine confirm runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-refine-confirm", 0x70707060)
	defer closeSessionFlow(t, flow)

	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected refine preview packet error: %v", err)
	}
	if len(previewOut) != 1 {
		t.Fatalf("expected refine preview to emit one frame, got %d", len(previewOut))
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, previewOut[0])); err != nil {
		t.Fatalf("decode refine preview frame: %v", err)
	}

	confirmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected refine confirm packet error: %v", err)
	}
	if len(confirmOut) != 7 {
		t.Fatalf("expected refine confirm burst of 7 frames, got %d", len(confirmOut))
	}
	delA, err := itemproto.DecodeDel(decodeSingleFrame(t, confirmOut[0]))
	if err != nil {
		t.Fatalf("decode material A ITEM_DEL: %v", err)
	}
	if delA.Position.WindowType != itemproto.WindowInventory || delA.Position.Cell != 6 {
		t.Fatalf("unexpected material A delete position: %+v", delA.Position)
	}
	delB, err := itemproto.DecodeDel(decodeSingleFrame(t, confirmOut[1]))
	if err != nil {
		t.Fatalf("decode material B first ITEM_DEL: %v", err)
	}
	if delB.Position.WindowType != itemproto.WindowInventory || delB.Position.Cell != 7 {
		t.Fatalf("unexpected material B delete position: %+v", delB.Position)
	}
	updateB, err := itemproto.DecodeUpdate(decodeSingleFrame(t, confirmOut[2]))
	if err != nil {
		t.Fatalf("decode material B ITEM_UPDATE: %v", err)
	}
	if updateB.Position.WindowType != itemproto.WindowInventory || updateB.Position.Cell != 8 || updateB.Count != 2 {
		t.Fatalf("unexpected material B update: %+v", updateB)
	}
	quickslotDelA, err := quickslotproto.DecodeDel(decodeSingleFrame(t, confirmOut[3]))
	if err != nil {
		t.Fatalf("decode material A QUICKSLOT_DEL: %v", err)
	}
	if quickslotDelA.Position != 3 {
		t.Fatalf("unexpected material A quickslot delete position: %+v", quickslotDelA)
	}
	quickslotDelB, err := quickslotproto.DecodeDel(decodeSingleFrame(t, confirmOut[4]))
	if err != nil {
		t.Fatalf("decode material B QUICKSLOT_DEL: %v", err)
	}
	if quickslotDelB.Position != 4 {
		t.Fatalf("unexpected material B quickslot delete position: %+v", quickslotDelB)
	}
	resultSet, err := itemproto.DecodeSet(decodeSingleFrame(t, confirmOut[5]))
	if err != nil {
		t.Fatalf("decode result ITEM_SET: %v", err)
	}
	if resultSet.Position.WindowType != itemproto.WindowInventory || resultSet.Position.Cell != 5 || resultSet.Vnum != 11231 || resultSet.Count != 1 {
		t.Fatalf("unexpected result ITEM_SET: %+v", resultSet)
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, confirmOut[6]))
	if err != nil {
		t.Fatalf("decode gold PLAYER_POINT_CHANGE: %v", err)
	}
	if goldChange.VID != owner.VID || goldChange.Type != bootstrapGoldPointType || goldChange.Amount != -2500 || goldChange.Value != 2500 {
		t.Fatalf("unexpected gold point change: %+v", goldChange)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after refine confirm, got %d", len(queued))
	}

	persisted, err := accounts.Load("item-refine-confirm")
	if err != nil {
		t.Fatalf("load persisted item-refine confirm account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{
		{ID: 630, Vnum: 11231, Count: 1, Slot: 5},
		{ID: 633, Vnum: 27002, Count: 2, Slot: 8},
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted inventory after refine confirm:\n got: %+v\nwant: %+v", persisted.Characters[0].Inventory, wantInventory)
	}
	wantQuickslots := []loginticket.Quickslot{
		{Position: 1, Type: quickslotproto.TypeSkill, Slot: 6},
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 5, Type: quickslotproto.TypeItem, Slot: 8},
		{Position: 6, Type: quickslotproto.TypeCommand, Slot: 7},
	}
	if persisted.Characters[0].Gold != 2500 || !reflect.DeepEqual(persisted.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted scalars after refine confirm: gold=%d quickslots=%+v want=%+v", persisted.Characters[0].Gold, persisted.Characters[0].Quickslots, wantQuickslots)
	}

	repeatOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected post-success refine packet error: %v", err)
	}
	if len(repeatOut) != 0 {
		t.Fatalf("expected cleared refine dialog to fail closed on repeat confirm, got %d frames", len(repeatOut))
	}
}

func TestGameRuntimeItemRefineConfirmCancelType255LeavesStateUnchanged(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineCancel", 0x01030761, 0x02040761, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 640, Vnum: 11232, Count: 1, Slot: 5},
		{ID: 641, Vnum: 27001, Count: 2, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "item-refine-cancel", 0x70707061, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-refine-cancel", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-refine cancel account: %v", err)
	}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11232,
		Name:       "Cancelable Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{ResultVnum: 11233, Cost: 1000, Probability: 100, Materials: []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}}},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11233, Name: "Canceled Result Blade", Stackable: false, MaxCount: 1}
	material := itemcatalog.Template{Vnum: 27001, Name: "Refine Material A", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{sourceTemplate, resultTemplate, material})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-refine cancel runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-refine-cancel", 0x70707061)
	defer closeSessionFlow(t, flow)

	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 4})))
	if err != nil || len(previewOut) != 1 {
		t.Fatalf("expected refine cancel preview to emit one frame, got %d err=%v", len(previewOut), err)
	}
	cancelOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 255})))
	if err != nil {
		t.Fatalf("unexpected refine cancel packet error: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected refine cancel to emit no frames, got %d", len(cancelOut))
	}
	confirmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 4})))
	if err != nil {
		t.Fatalf("unexpected post-cancel confirm packet error: %v", err)
	}
	if len(confirmOut) != 1 {
		t.Fatalf("expected post-cancel matching refine to reopen preview only, got %d", len(confirmOut))
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, confirmOut[0])); err != nil {
		t.Fatalf("decode post-cancel refine preview: %v", err)
	}
	assertExchangeAccountUnchanged(t, accounts, "item-refine-cancel", owner, "refine cancel")
}

func TestGameRuntimeItemRefineConfirmBusyWindowsFailClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := merchantBuyerCharacter("RefineBusy", 0x01030762, 0x02040762, 5000, []inventory.ItemInstance{
		{ID: 650, Vnum: 11234, Count: 1, Slot: 5},
		{ID: 651, Vnum: 27001, Count: 2, Slot: 6},
	})
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "item-refine-busy", 0x70707062, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-refine-busy", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-refine busy account: %v", err)
	}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11234,
		Name:       "Busy Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{ResultVnum: 11235, Cost: 1000, Probability: 100, Materials: []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}}},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11235, Name: "Busy Result Blade", Stackable: false, MaxCount: 1}
	templates := append(defaultMerchantItemTemplates(), sourceTemplate, resultTemplate)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{defaultMerchantCatalogDefinition()})
	itemStore := newItemTemplateStore(t, templates)
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected item-refine busy runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Merchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, "npc:merchant")
	if !ok {
		t.Fatal("expected merchant static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-refine-busy", 0x70707062)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil || len(previewOut) != 1 {
		t.Fatalf("expected refine busy preview to emit one frame, got %d err=%v", len(previewOut), err)
	}
	interactWithMerchantForBuy(t, flow, actor.EntityID)
	busyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected busy refine confirm packet error: %v", err)
	}
	if len(busyOut) != 0 {
		t.Fatalf("expected busy refine confirm to fail closed, got %d frames", len(busyOut))
	}
	assertExchangeAccountUnchanged(t, accounts, "item-refine-busy", owner, "busy refine confirm")

	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected merchant close after busy refine: %v", err)
	}
	if len(closeOut) != 1 {
		t.Fatalf("expected merchant close after busy refine to emit SHOP END, got %d", len(closeOut))
	}
	cancelOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 255})))
	if err != nil {
		t.Fatalf("unexpected busy refine cancel packet error: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected busy refine cancel to emit no frames, got %d", len(cancelOut))
	}
	assertExchangeAccountUnchanged(t, accounts, "item-refine-busy", owner, "busy refine cancel")
}

func TestGameRuntimeItemRefineConfirmBusySafeboxFailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineBusySafebox", 0x010307d0, 0x020407d0, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 652, Vnum: 11238, Count: 1, Slot: 5},
		{ID: 653, Vnum: 27001, Count: 2, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	ownerLogin := "item-refine-busy-sb"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070d0, owner)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-refine busy-safebox account: %v", err)
	}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11238,
		Name:       "Busy Safebox Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{ResultVnum: 11239, Cost: 1000, Probability: 100, Materials: []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}}},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11239, Name: "Busy Safebox Result Blade", Stackable: false, MaxCount: 1}
	material := itemcatalog.Template{Vnum: 27001, Name: "Refine Material A", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{sourceTemplate, resultTemplate, material})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-refine busy-safebox runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070d0)
	defer closeSessionFlow(t, flow)

	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil || len(previewOut) != 1 {
		t.Fatalf("expected refine busy-safebox preview to emit one frame, got %d err=%v", len(previewOut), err)
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, previewOut[0])); err != nil {
		t.Fatalf("decode refine busy-safebox preview: %v", err)
	}

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected busy-safebox /open_safebox error: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected busy-safebox /open_safebox to emit one SAFEBOX_SIZE frame, got %d", len(openOut))
	}
	if _, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, openOut[0])); err != nil {
		t.Fatalf("decode busy-safebox /open_safebox SAFEBOX_SIZE: %v", err)
	}

	busyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected busy-safebox refine confirm packet error: %v", err)
	}
	if len(busyOut) != 0 {
		t.Fatalf("expected busy-safebox refine confirm to fail closed with no frames, got %d", len(busyOut))
	}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "busy-safebox refine confirm")

	assertCloseSafeboxCommandChat(t, flow, "/close_safebox", "busy-safebox close")

	confirmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected post-safebox-close refine confirm packet error: %v", err)
	}
	if len(confirmOut) != 3 {
		t.Fatalf("expected post-safebox-close refine confirm burst of 3 frames, got %d", len(confirmOut))
	}
	materialDel, err := itemproto.DecodeDel(decodeSingleFrame(t, confirmOut[0]))
	if err != nil {
		t.Fatalf("decode post-safebox-close material ITEM_DEL: %v", err)
	}
	if materialDel.Position.WindowType != itemproto.WindowInventory || materialDel.Position.Cell != 6 {
		t.Fatalf("unexpected post-safebox-close material delete position: %+v", materialDel.Position)
	}
	resultSet, err := itemproto.DecodeSet(decodeSingleFrame(t, confirmOut[1]))
	if err != nil {
		t.Fatalf("decode post-safebox-close result ITEM_SET: %v", err)
	}
	if resultSet.Position.WindowType != itemproto.WindowInventory || resultSet.Position.Cell != 5 || resultSet.Vnum != 11239 || resultSet.Count != 1 {
		t.Fatalf("unexpected post-safebox-close result ITEM_SET: %+v", resultSet)
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, confirmOut[2]))
	if err != nil {
		t.Fatalf("decode post-safebox-close gold PLAYER_POINT_CHANGE: %v", err)
	}
	if goldChange.VID != owner.VID || goldChange.Type != bootstrapGoldPointType || goldChange.Amount != -1000 || goldChange.Value != 4000 {
		t.Fatalf("unexpected post-safebox-close gold point change: %+v", goldChange)
	}

	persisted, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load persisted busy-safebox refine account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{{ID: 652, Vnum: 11239, Count: 1, Slot: 5}}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted inventory after busy-safebox refine confirm:\n got: %+v\nwant: %+v", persisted.Characters[0].Inventory, wantInventory)
	}
	if persisted.Characters[0].Gold != 4000 || !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("unexpected persisted scalars after busy-safebox refine confirm: gold=%d quickslots=%+v", persisted.Characters[0].Gold, persisted.Characters[0].Quickslots)
	}
}

func TestGameRuntimeItemRefineConfirmProbabilityBelow100FailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineLowProb", 0x01030763, 0x02040763, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 660, Vnum: 11236, Count: 1, Slot: 5},
		{ID: 661, Vnum: 27001, Count: 2, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "item-refine-lowprob", 0x70707063, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-refine-lowprob", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-refine low-prob account: %v", err)
	}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11236,
		Name:       "Low Prob Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{ResultVnum: 11237, Cost: 1000, Probability: 75, Materials: []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}}},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11237, Name: "Low Prob Result Blade", Stackable: false, MaxCount: 1}
	material := itemcatalog.Template{Vnum: 27001, Name: "Refine Material A", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{sourceTemplate, resultTemplate, material})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-refine low-prob runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-refine-lowprob", 0x70707063)
	defer closeSessionFlow(t, flow)

	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil || len(previewOut) != 1 {
		t.Fatalf("expected low-prob refine preview to emit one frame, got %d err=%v", len(previewOut), err)
	}
	confirmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected low-prob refine confirm packet error: %v", err)
	}
	if len(confirmOut) != 0 {
		t.Fatalf("expected low-prob refine confirm to fail closed, got %d frames", len(confirmOut))
	}
	assertExchangeAccountUnchanged(t, accounts, "item-refine-lowprob", owner, "low-prob refine confirm")
	cancelOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 255})))
	if err != nil || len(cancelOut) != 0 {
		t.Fatalf("expected low-prob refine cancel to emit no frames, got %d err=%v", len(cancelOut), err)
	}
	assertExchangeAccountUnchanged(t, accounts, "item-refine-lowprob", owner, "low-prob refine cancel")
}
