package minimal

import (
	"fmt"
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
	if len(confirmOut) != 8 {
		t.Fatalf("expected refine confirm burst of 8 frames, got %d", len(confirmOut))
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
	assertRefineSucceededCommandChat(t, confirmOut[7], 3, "probability-100 refine confirm")
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

func TestGameRuntimeItemRefineConfirmAfterPreviewProbability100PreservesInstanceSocketsAndAttributes(t *testing.T) {
	activeSockets := inventory.SocketValues{11, 0, -3}
	activeAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	cases := []struct {
		name       string
		sockets    *inventory.SocketValues
		attributes *inventory.AttributeValues
		wantWireS  [itemproto.ItemSocketCount]int32
		wantWireA0 itemproto.Attribute
		wantWireA1 itemproto.Attribute
	}{
		{
			name:    "active sockets and attributes",
			sockets: &activeSockets, attributes: &activeAttributes,
			wantWireS:  [itemproto.ItemSocketCount]int32{11, 0, -3},
			wantWireA0: itemproto.Attribute{Type: 4, Value: 55},
			wantWireA1: itemproto.Attribute{Type: 9, Value: -7},
		},
		{
			name:       "omitted sockets and attributes use result template fallback",
			wantWireS:  [itemproto.ItemSocketCount]int32{21, 22, 23},
			wantWireA0: itemproto.Attribute{Type: 2, Value: 8},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("RefinePreserve", 0x01030770, 0x02040770, 1100, 2100, 0, 101, 201)
			owner.Gold = 5000
			owner.Inventory = []inventory.ItemInstance{
				{ID: 630, Vnum: 11230, Count: 1, Slot: 5, Sockets: tc.sockets, Attributes: tc.attributes},
				{ID: 631, Vnum: 27001, Count: 2, Slot: 6},
				{ID: 632, Vnum: 27002, Count: 3, Slot: 7},
			}
			issuePeerTicket(t, ticketStore, "item-refine-preserve", 0x70707070, owner)
			if err := accounts.Save(accountstore.Account{Login: "item-refine-preserve", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed item-refine preserve account: %v", err)
			}
			sourceTemplate := itemcatalog.Template{
				Vnum: 11230, Name: "Preserve Practice Blade", Stackable: false, MaxCount: 1, Refineable: true,
				RefineInfo: &itemcatalog.RefineInfo{
					ResultVnum: 11231, Cost: 2500, Probability: 100,
					Materials: []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}, {Vnum: 27002, Count: 3}},
				},
				Sockets:    itemcatalog.SocketValues{1, 2, 3},
				Attributes: itemcatalog.AttributeValues{{Type: 1, Value: 1}},
			}
			resultTemplate := itemcatalog.Template{
				Vnum: 11231, Name: "Preserved Practice Blade", Stackable: false, MaxCount: 1,
				Sockets:    itemcatalog.SocketValues{21, 22, 23},
				Attributes: itemcatalog.AttributeValues{{Type: 2, Value: 8}},
			}
			materialA := itemcatalog.Template{Vnum: 27001, Name: "Refine Material A", Stackable: true, MaxCount: 200}
			materialB := itemcatalog.Template{Vnum: 27002, Name: "Refine Material B", Stackable: true, MaxCount: 200}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{sourceTemplate, resultTemplate, materialA, materialB})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected item-refine preserve runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-refine-preserve", 0x70707070)
			defer closeSessionFlow(t, flow)

			previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
			if err != nil || len(previewOut) != 1 {
				t.Fatalf("expected refine preserve preview one frame, got %d err=%v", len(previewOut), err)
			}
			confirmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
			if err != nil {
				t.Fatalf("unexpected refine preserve confirm error: %v", err)
			}
			if len(confirmOut) != 5 {
				t.Fatalf("expected refine preserve burst of 5 frames (no quickslot dels), got %d", len(confirmOut))
			}
			resultSet, err := itemproto.DecodeSet(decodeSingleFrame(t, confirmOut[2]))
			if err != nil {
				t.Fatalf("decode refine preserve result ITEM_SET: %v", err)
			}
			if resultSet.Position.Cell != 5 || resultSet.Vnum != 11231 || resultSet.Count != 1 {
				t.Fatalf("unexpected refine preserve result ITEM_SET: %+v", resultSet)
			}
			if resultSet.Sockets != tc.wantWireS {
				t.Fatalf("unexpected refine preserve ITEM_SET sockets %+v want %+v", resultSet.Sockets, tc.wantWireS)
			}
			if resultSet.Attributes[0] != tc.wantWireA0 || resultSet.Attributes[1] != tc.wantWireA1 {
				t.Fatalf("unexpected refine preserve ITEM_SET attributes %+v want [%+v %+v]", resultSet.Attributes, tc.wantWireA0, tc.wantWireA1)
			}
			persisted, err := accounts.Load("item-refine-preserve")
			if err != nil {
				t.Fatalf("load refine preserve account: %v", err)
			}
			got := persisted.Characters[0].Inventory[0]
			if got.ID != 630 || got.Vnum != 11231 || got.Count != 1 || got.Slot != 5 {
				t.Fatalf("unexpected persisted result cell: %#v", got)
			}
			if (tc.sockets != nil) != got.HasSockets() {
				t.Fatalf("persisted HasSockets=%v want %v", got.HasSockets(), tc.sockets != nil)
			}
			if (tc.attributes != nil) != got.HasAttributes() {
				t.Fatalf("persisted HasAttributes=%v want %v", got.HasAttributes(), tc.attributes != nil)
			}
			if tc.sockets != nil {
				if got.Sockets == nil || *got.Sockets != *tc.sockets {
					t.Fatalf("expected persisted sockets %+v, got %#v", *tc.sockets, got.Sockets)
				}
			} else if got.Sockets != nil {
				t.Fatalf("expected omitted persisted sockets, got %#v", got.Sockets)
			}
			if tc.attributes != nil {
				if got.Attributes == nil || *got.Attributes != *tc.attributes {
					t.Fatalf("expected persisted attributes %+v, got %#v", *tc.attributes, got.Attributes)
				}
			} else if got.Attributes != nil {
				t.Fatalf("expected omitted persisted attributes, got %#v", got.Attributes)
			}
		})
	}
}

func TestGameRuntimeItemRefineConfirmAfterPreviewFailResultVnumPreservesInstanceSocketsAndAttributes(t *testing.T) {
	restore := QueueRefineConfirmRollForTest(76)
	t.Cleanup(restore)

	activeSockets := inventory.SocketValues{7, 0, 9}
	activeAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineDownPreserve", 0x01030771, 0x02040771, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 666, Vnum: 11246, Count: 1, Slot: 5, Sockets: &activeSockets, Attributes: &activeAttributes},
		{ID: 667, Vnum: 27001, Count: 2, Slot: 6},
	}
	issuePeerTicket(t, ticketStore, "item-refine-down-preserve", 0x70707071, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-refine-down-preserve", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-refine downgrade-preserve account: %v", err)
	}
	sourceTemplate := itemcatalog.Template{
		Vnum: 11246, Name: "Downgrade Preserve Blade", Stackable: false, MaxCount: 1, Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{ResultVnum: 11247, Cost: 1000, Probability: 75, FailResultVnum: 11240, Materials: []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}}},
		Sockets:    itemcatalog.SocketValues{1, 2, 3},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11247, Name: "Unreached Result", Stackable: false, MaxCount: 1, Sockets: itemcatalog.SocketValues{30, 31, 32}}
	failResultTemplate := itemcatalog.Template{Vnum: 11240, Name: "Downgraded Preserve Blade", Stackable: false, MaxCount: 1, Sockets: itemcatalog.SocketValues{40, 41, 42}, Attributes: itemcatalog.AttributeValues{{Type: 8, Value: 9}}}
	material := itemcatalog.Template{Vnum: 27001, Name: "Refine Material A", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{sourceTemplate, resultTemplate, failResultTemplate, material})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-refine downgrade-preserve runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-refine-down-preserve", 0x70707071)
	defer closeSessionFlow(t, flow)

	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 6})))
	if err != nil || len(previewOut) != 1 {
		t.Fatalf("expected downgrade-preserve preview one frame, got %d err=%v", len(previewOut), err)
	}
	confirmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 6})))
	if err != nil {
		t.Fatalf("unexpected downgrade-preserve confirm error: %v", err)
	}
	if len(confirmOut) != 4 {
		t.Fatalf("expected downgrade-preserve burst of 4 frames, got %d", len(confirmOut))
	}
	resultSet, err := itemproto.DecodeSet(decodeSingleFrame(t, confirmOut[1]))
	if err != nil {
		t.Fatalf("decode downgrade-preserve result ITEM_SET: %v", err)
	}
	if resultSet.Vnum != 11240 || resultSet.Count != 1 || resultSet.Position.Cell != 5 {
		t.Fatalf("unexpected downgrade-preserve ITEM_SET: %+v", resultSet)
	}
	wantSockets := [itemproto.ItemSocketCount]int32{7, 0, 9}
	if resultSet.Sockets != wantSockets {
		t.Fatalf("expected preserved downgrade ITEM_SET sockets %+v, got %+v", wantSockets, resultSet.Sockets)
	}
	if resultSet.Attributes[0] != (itemproto.Attribute{Type: 1, Value: 25}) || resultSet.Attributes[1] != (itemproto.Attribute{Type: 7, Value: -3}) {
		t.Fatalf("unexpected downgrade-preserve ITEM_SET attributes %+v", resultSet.Attributes)
	}
	account, err := accounts.Load("item-refine-down-preserve")
	if err != nil {
		t.Fatalf("load downgrade-preserve account: %v", err)
	}
	got := account.Characters[0].Inventory[0]
	if got.ID != 666 || got.Vnum != 11240 || !got.HasSockets() || !got.HasAttributes() || *got.Sockets != activeSockets || *got.Attributes != activeAttributes {
		t.Fatalf("expected persisted downgrade preserve presence, got %#v", got)
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

func TestGameRuntimeItemRefineConfirmBusyCubeFailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineBusyCube", 0x010307d2, 0x020407d2, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 654, Vnum: 11240, Count: 1, Slot: 5},
		{ID: 655, Vnum: 27001, Count: 2, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	ownerLogin := "item-refine-busy-cube"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070d2, owner)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-refine busy-cube account: %v", err)
	}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11240,
		Name:       "Busy Cube Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{ResultVnum: 11241, Cost: 1000, Probability: 100, Materials: []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}}},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11241, Name: "Busy Cube Result Blade", Stackable: false, MaxCount: 1}
	material := itemcatalog.Template{Vnum: 27001, Name: "Refine Material A", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{sourceTemplate, resultTemplate, material})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-refine busy-cube runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070d2)
	defer closeSessionFlow(t, flow)

	openCubeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected busy-cube /open_cube error: %v", err)
	}
	if len(openCubeOut) != 1 {
		t.Fatalf("expected busy-cube /open_cube to emit one command chat frame, got %d", len(openCubeOut))
	}
	assertCubeCommandChatFrame(t, openCubeOut[0], "cube open 20022", "refine busy-cube open")

	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil || len(previewOut) != 1 {
		t.Fatalf("expected refine busy-cube preview to emit one frame, got %d err=%v", len(previewOut), err)
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, previewOut[0])); err != nil {
		t.Fatalf("decode refine busy-cube preview: %v", err)
	}

	busyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected busy-cube refine confirm packet error: %v", err)
	}
	if len(busyOut) != 0 {
		t.Fatalf("expected busy-cube refine confirm to fail closed with no frames, got %d", len(busyOut))
	}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "busy-cube refine confirm")

	closeCubeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected busy-cube /close_cube error: %v", err)
	}
	if len(closeCubeOut) != 1 {
		t.Fatalf("expected busy-cube /close_cube to emit one command chat frame, got %d", len(closeCubeOut))
	}
	assertCubeCommandChatFrame(t, closeCubeOut[0], "cube close", "refine busy-cube close")

	confirmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected post-cube-close refine confirm packet error: %v", err)
	}
	if len(confirmOut) != 4 {
		t.Fatalf("expected post-cube-close refine confirm burst of 4 frames, got %d", len(confirmOut))
	}
	materialDel, err := itemproto.DecodeDel(decodeSingleFrame(t, confirmOut[0]))
	if err != nil {
		t.Fatalf("decode post-cube-close material ITEM_DEL: %v", err)
	}
	if materialDel.Position.WindowType != itemproto.WindowInventory || materialDel.Position.Cell != 6 {
		t.Fatalf("unexpected post-cube-close material delete position: %+v", materialDel.Position)
	}
	resultSet, err := itemproto.DecodeSet(decodeSingleFrame(t, confirmOut[1]))
	if err != nil {
		t.Fatalf("decode post-cube-close result ITEM_SET: %v", err)
	}
	if resultSet.Position.WindowType != itemproto.WindowInventory || resultSet.Position.Cell != 5 || resultSet.Vnum != 11241 || resultSet.Count != 1 {
		t.Fatalf("unexpected post-cube-close result ITEM_SET: %+v", resultSet)
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, confirmOut[2]))
	if err != nil {
		t.Fatalf("decode post-cube-close gold PLAYER_POINT_CHANGE: %v", err)
	}
	if goldChange.VID != owner.VID || goldChange.Type != bootstrapGoldPointType || goldChange.Amount != -1000 || goldChange.Value != 4000 {
		t.Fatalf("unexpected post-cube-close gold point change: %+v", goldChange)
	}
	assertRefineSucceededCommandChat(t, confirmOut[3], 3, "post-cube-close refine confirm")
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
	if len(openOut) != 2 {
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
	if len(confirmOut) != 4 {
		t.Fatalf("expected post-safebox-close refine confirm burst of 4 frames, got %d", len(confirmOut))
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
	assertRefineSucceededCommandChat(t, confirmOut[3], 3, "post-safebox-close refine confirm")

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

func TestGameRuntimeItemRefineConfirmAfterPreviewProbability0DestroysAndEmitsRefineFailed(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineFailConfirm", 0x01030766, 0x02040766, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 670, Vnum: 11240, Count: 1, Slot: 5},
		{ID: 671, Vnum: 27001, Count: 2, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "item-refine-fail-confirm", 0x70707066, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-refine-fail-confirm", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-refine fail-confirm account: %v", err)
	}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11240,
		Name:       "Always-Fail Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{ResultVnum: 11241, Cost: 1000, Probability: 0, Materials: []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}}},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11241, Name: "Unreached Result Blade", Stackable: false, MaxCount: 1}
	material := itemcatalog.Template{Vnum: 27001, Name: "Refine Material A", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{sourceTemplate, resultTemplate, material})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-refine fail-confirm runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-refine-fail-confirm", 0x70707066)
	defer closeSessionFlow(t, flow)

	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 4})))
	if err != nil || len(previewOut) != 1 {
		t.Fatalf("expected probability-0 refine preview to emit one frame, got %d err=%v", len(previewOut), err)
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, previewOut[0])); err != nil {
		t.Fatalf("decode probability-0 refine preview: %v", err)
	}

	confirmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 4})))
	if err != nil {
		t.Fatalf("unexpected probability-0 refine confirm packet error: %v", err)
	}
	if len(confirmOut) != 6 {
		t.Fatalf("expected probability-0 refine destroy burst of 6 frames, got %d", len(confirmOut))
	}
	materialDel, err := itemproto.DecodeDel(decodeSingleFrame(t, confirmOut[0]))
	if err != nil {
		t.Fatalf("decode probability-0 material ITEM_DEL: %v", err)
	}
	if materialDel.Position.WindowType != itemproto.WindowInventory || materialDel.Position.Cell != 6 {
		t.Fatalf("unexpected probability-0 material delete position: %+v", materialDel.Position)
	}
	materialQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, confirmOut[1]))
	if err != nil {
		t.Fatalf("decode probability-0 material QUICKSLOT_DEL: %v", err)
	}
	if materialQuickslotDel.Position != 3 {
		t.Fatalf("unexpected probability-0 material quickslot delete: %+v", materialQuickslotDel)
	}
	sourceDel, err := itemproto.DecodeDel(decodeSingleFrame(t, confirmOut[2]))
	if err != nil {
		t.Fatalf("decode probability-0 source ITEM_DEL: %v", err)
	}
	if sourceDel.Position.WindowType != itemproto.WindowInventory || sourceDel.Position.Cell != 5 {
		t.Fatalf("unexpected probability-0 source delete position: %+v", sourceDel.Position)
	}
	sourceQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, confirmOut[3]))
	if err != nil {
		t.Fatalf("decode probability-0 source QUICKSLOT_DEL: %v", err)
	}
	if sourceQuickslotDel.Position != 2 {
		t.Fatalf("unexpected probability-0 source quickslot delete: %+v", sourceQuickslotDel)
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, confirmOut[4]))
	if err != nil {
		t.Fatalf("decode probability-0 gold point change: %v", err)
	}
	if goldChange.VID != owner.VID || goldChange.Type != bootstrapGoldPointType || goldChange.Amount != -1000 || goldChange.Value != 4000 {
		t.Fatalf("unexpected probability-0 gold point change: %+v", goldChange)
	}
	assertRefineFailedCommandChat(t, confirmOut[5], 4, "probability-0 confirm")

	account, err := accounts.Load("item-refine-fail-confirm")
	if err != nil {
		t.Fatalf("load probability-0 refine account: %v", err)
	}
	if account.Characters[0].Gold != 4000 {
		t.Fatalf("expected persisted gold 4000 after probability-0 destroy, got %d", account.Characters[0].Gold)
	}
	if len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected empty persisted inventory after probability-0 destroy, got %#v", account.Characters[0].Inventory)
	}
	wantQuickslots := []loginticket.Quickslot{{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5}}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted quickslots after probability-0 destroy: got %#v want %#v", account.Characters[0].Quickslots, wantQuickslots)
	}
}

func assertRefineFailedCommandChat(t *testing.T, frame []byte, refineType uint8, label string) {
	t.Helper()
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, frame))
	if err != nil {
		t.Fatalf("decode %s RefineFailed chat: %v", label, err)
	}
	wantMessage := fmt.Sprintf("RefineFailed %d", refineType)
	if delivery.Type != chatproto.ChatTypeCommand || delivery.VID != 0 || delivery.Empire != 0 || delivery.Message != wantMessage {
		t.Fatalf("unexpected %s RefineFailed chat: %+v want message=%q", label, delivery, wantMessage)
	}
}

func TestGameRuntimeItemRefineConfirmAfterPreviewProbability75RollSuccessPersistsAndEmitsBurst(t *testing.T) {
	restore := QueueRefineConfirmRollForTest(75)
	t.Cleanup(restore)

	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineRollOK", 0x01030763, 0x02040763, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 660, Vnum: 11236, Count: 1, Slot: 5},
		{ID: 661, Vnum: 27001, Count: 2, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "item-refine-roll-ok", 0x70707063, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-refine-roll-ok", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-refine roll-success account: %v", err)
	}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11236,
		Name:       "Roll Success Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{ResultVnum: 11237, Cost: 1000, Probability: 75, Materials: []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}}},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11237, Name: "Roll Success Result Blade", Stackable: false, MaxCount: 1}
	material := itemcatalog.Template{Vnum: 27001, Name: "Refine Material A", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{sourceTemplate, resultTemplate, material})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-refine roll-success runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-refine-roll-ok", 0x70707063)
	defer closeSessionFlow(t, flow)

	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil || len(previewOut) != 1 {
		t.Fatalf("expected probability-75 roll-success preview to emit one frame, got %d err=%v", len(previewOut), err)
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, previewOut[0])); err != nil {
		t.Fatalf("decode probability-75 roll-success preview: %v", err)
	}

	confirmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected probability-75 roll-success confirm packet error: %v", err)
	}
	if len(confirmOut) != 5 {
		t.Fatalf("expected probability-75 roll-success burst of 5 frames, got %d", len(confirmOut))
	}
	materialDel, err := itemproto.DecodeDel(decodeSingleFrame(t, confirmOut[0]))
	if err != nil {
		t.Fatalf("decode probability-75 roll-success material ITEM_DEL: %v", err)
	}
	if materialDel.Position.WindowType != itemproto.WindowInventory || materialDel.Position.Cell != 6 {
		t.Fatalf("unexpected probability-75 roll-success material delete position: %+v", materialDel.Position)
	}
	materialQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, confirmOut[1]))
	if err != nil {
		t.Fatalf("decode probability-75 roll-success material QUICKSLOT_DEL: %v", err)
	}
	if materialQuickslotDel.Position != 3 {
		t.Fatalf("unexpected probability-75 roll-success material quickslot delete: %+v", materialQuickslotDel)
	}
	resultSet, err := itemproto.DecodeSet(decodeSingleFrame(t, confirmOut[2]))
	if err != nil {
		t.Fatalf("decode probability-75 roll-success result ITEM_SET: %v", err)
	}
	if resultSet.Position.WindowType != itemproto.WindowInventory || resultSet.Position.Cell != 5 || resultSet.Vnum != 11237 || resultSet.Count != 1 {
		t.Fatalf("unexpected probability-75 roll-success result ITEM_SET: %+v", resultSet)
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, confirmOut[3]))
	if err != nil {
		t.Fatalf("decode probability-75 roll-success gold PLAYER_POINT_CHANGE: %v", err)
	}
	if goldChange.VID != owner.VID || goldChange.Type != bootstrapGoldPointType || goldChange.Amount != -1000 || goldChange.Value != 4000 {
		t.Fatalf("unexpected probability-75 roll-success gold point change: %+v", goldChange)
	}
	assertRefineSucceededCommandChat(t, confirmOut[4], 3, "probability-75 roll-success confirm")

	persisted, err := accounts.Load("item-refine-roll-ok")
	if err != nil {
		t.Fatalf("load persisted probability-75 roll-success account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{{ID: 660, Vnum: 11237, Count: 1, Slot: 5}}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted inventory after probability-75 roll-success:\n got: %+v\nwant: %+v", persisted.Characters[0].Inventory, wantInventory)
	}
	wantQuickslots := []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	if persisted.Characters[0].Gold != 4000 || !reflect.DeepEqual(persisted.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted scalars after probability-75 roll-success: gold=%d quickslots=%+v want=%+v", persisted.Characters[0].Gold, persisted.Characters[0].Quickslots, wantQuickslots)
	}
}

func TestGameRuntimeItemRefineConfirmAfterPreviewProbability75RollFailureDestroysAndEmitsRefineFailed(t *testing.T) {
	restore := QueueRefineConfirmRollForTest(76)
	t.Cleanup(restore)

	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineRollFail", 0x01030764, 0x02040764, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 662, Vnum: 11242, Count: 1, Slot: 5},
		{ID: 663, Vnum: 27001, Count: 2, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "item-refine-roll-fail", 0x70707064, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-refine-roll-fail", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-refine roll-fail account: %v", err)
	}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11242,
		Name:       "Roll Fail Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{ResultVnum: 11243, Cost: 1000, Probability: 75, Materials: []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}}},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11243, Name: "Unreached Roll Result Blade", Stackable: false, MaxCount: 1}
	material := itemcatalog.Template{Vnum: 27001, Name: "Refine Material A", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{sourceTemplate, resultTemplate, material})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-refine roll-fail runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-refine-roll-fail", 0x70707064)
	defer closeSessionFlow(t, flow)

	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 4})))
	if err != nil || len(previewOut) != 1 {
		t.Fatalf("expected probability-75 roll-fail preview to emit one frame, got %d err=%v", len(previewOut), err)
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, previewOut[0])); err != nil {
		t.Fatalf("decode probability-75 roll-fail preview: %v", err)
	}

	confirmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 4})))
	if err != nil {
		t.Fatalf("unexpected probability-75 roll-fail confirm packet error: %v", err)
	}
	if len(confirmOut) != 6 {
		t.Fatalf("expected probability-75 roll-fail destroy burst of 6 frames, got %d", len(confirmOut))
	}
	materialDel, err := itemproto.DecodeDel(decodeSingleFrame(t, confirmOut[0]))
	if err != nil {
		t.Fatalf("decode probability-75 roll-fail material ITEM_DEL: %v", err)
	}
	if materialDel.Position.WindowType != itemproto.WindowInventory || materialDel.Position.Cell != 6 {
		t.Fatalf("unexpected probability-75 roll-fail material delete position: %+v", materialDel.Position)
	}
	materialQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, confirmOut[1]))
	if err != nil {
		t.Fatalf("decode probability-75 roll-fail material QUICKSLOT_DEL: %v", err)
	}
	if materialQuickslotDel.Position != 3 {
		t.Fatalf("unexpected probability-75 roll-fail material quickslot delete: %+v", materialQuickslotDel)
	}
	sourceDel, err := itemproto.DecodeDel(decodeSingleFrame(t, confirmOut[2]))
	if err != nil {
		t.Fatalf("decode probability-75 roll-fail source ITEM_DEL: %v", err)
	}
	if sourceDel.Position.WindowType != itemproto.WindowInventory || sourceDel.Position.Cell != 5 {
		t.Fatalf("unexpected probability-75 roll-fail source delete position: %+v", sourceDel.Position)
	}
	sourceQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, confirmOut[3]))
	if err != nil {
		t.Fatalf("decode probability-75 roll-fail source QUICKSLOT_DEL: %v", err)
	}
	if sourceQuickslotDel.Position != 2 {
		t.Fatalf("unexpected probability-75 roll-fail source quickslot delete: %+v", sourceQuickslotDel)
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, confirmOut[4]))
	if err != nil {
		t.Fatalf("decode probability-75 roll-fail gold point change: %v", err)
	}
	if goldChange.VID != owner.VID || goldChange.Type != bootstrapGoldPointType || goldChange.Amount != -1000 || goldChange.Value != 4000 {
		t.Fatalf("unexpected probability-75 roll-fail gold point change: %+v", goldChange)
	}
	assertRefineFailedCommandChat(t, confirmOut[5], 4, "probability-75 roll-fail confirm")

	account, err := accounts.Load("item-refine-roll-fail")
	if err != nil {
		t.Fatalf("load probability-75 roll-fail refine account: %v", err)
	}
	if account.Characters[0].Gold != 4000 {
		t.Fatalf("expected persisted gold 4000 after probability-75 roll-fail destroy, got %d", account.Characters[0].Gold)
	}
	if len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected empty persisted inventory after probability-75 roll-fail destroy, got %#v", account.Characters[0].Inventory)
	}
	wantQuickslots := []loginticket.Quickslot{{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5}}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted quickslots after probability-75 roll-fail destroy: got %#v want %#v", account.Characters[0].Quickslots, wantQuickslots)
	}
}

func TestGameRuntimeItemRefineConfirmAfterPreviewProbability75KeepOnFailKeepsSourceAndEmitsRefineFailed(t *testing.T) {
	restore := QueueRefineConfirmRollForTest(76)
	t.Cleanup(restore)

	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineRollKeep", 0x01030765, 0x02040765, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 664, Vnum: 11244, Count: 1, Slot: 5},
		{ID: 665, Vnum: 27001, Count: 2, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "item-refine-roll-keep", 0x70707065, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-refine-roll-keep", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-refine roll-keep account: %v", err)
	}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11244,
		Name:       "Keep Fail Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{ResultVnum: 11245, Cost: 1000, Probability: 75, KeepOnFail: true, Materials: []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}}},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11245, Name: "Unreached Keep Result Blade", Stackable: false, MaxCount: 1}
	material := itemcatalog.Template{Vnum: 27001, Name: "Refine Material A", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{sourceTemplate, resultTemplate, material})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-refine roll-keep runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-refine-roll-keep", 0x70707065)
	defer closeSessionFlow(t, flow)

	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 5})))
	if err != nil || len(previewOut) != 1 {
		t.Fatalf("expected probability-75 keep_on_fail preview to emit one frame, got %d err=%v", len(previewOut), err)
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, previewOut[0])); err != nil {
		t.Fatalf("decode probability-75 keep_on_fail preview: %v", err)
	}

	confirmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 5})))
	if err != nil {
		t.Fatalf("unexpected probability-75 keep_on_fail confirm packet error: %v", err)
	}
	if len(confirmOut) != 4 {
		t.Fatalf("expected probability-75 keep_on_fail burst of 4 frames, got %d", len(confirmOut))
	}
	materialDel, err := itemproto.DecodeDel(decodeSingleFrame(t, confirmOut[0]))
	if err != nil {
		t.Fatalf("decode probability-75 keep_on_fail material ITEM_DEL: %v", err)
	}
	if materialDel.Position.WindowType != itemproto.WindowInventory || materialDel.Position.Cell != 6 {
		t.Fatalf("unexpected probability-75 keep_on_fail material delete position: %+v", materialDel.Position)
	}
	materialQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, confirmOut[1]))
	if err != nil {
		t.Fatalf("decode probability-75 keep_on_fail material QUICKSLOT_DEL: %v", err)
	}
	if materialQuickslotDel.Position != 3 {
		t.Fatalf("unexpected probability-75 keep_on_fail material quickslot delete: %+v", materialQuickslotDel)
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, confirmOut[2]))
	if err != nil {
		t.Fatalf("decode probability-75 keep_on_fail gold point change: %v", err)
	}
	if goldChange.VID != owner.VID || goldChange.Type != bootstrapGoldPointType || goldChange.Amount != -1000 || goldChange.Value != 4000 {
		t.Fatalf("unexpected probability-75 keep_on_fail gold point change: %+v", goldChange)
	}
	assertRefineFailedCommandChat(t, confirmOut[3], 5, "probability-75 keep_on_fail confirm")

	account, err := accounts.Load("item-refine-roll-keep")
	if err != nil {
		t.Fatalf("load probability-75 keep_on_fail refine account: %v", err)
	}
	if account.Characters[0].Gold != 4000 {
		t.Fatalf("expected persisted gold 4000 after probability-75 keep_on_fail, got %d", account.Characters[0].Gold)
	}
	wantInventory := []inventory.ItemInstance{{ID: 664, Vnum: 11244, Count: 1, Slot: 5}}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("expected source kept after probability-75 keep_on_fail, got %#v", account.Characters[0].Inventory)
	}
	wantQuickslots := []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted quickslots after probability-75 keep_on_fail: got %#v want %#v", account.Characters[0].Quickslots, wantQuickslots)
	}
}

func TestGameRuntimeItemRefineConfirmAfterPreviewProbability75FailResultVnumDowngradesAndEmitsRefineFailed(t *testing.T) {
	restore := QueueRefineConfirmRollForTest(76)
	t.Cleanup(restore)

	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineRollDown", 0x01030766, 0x02040766, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 666, Vnum: 11246, Count: 1, Slot: 5},
		{ID: 667, Vnum: 27001, Count: 2, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "item-refine-roll-downgrade", 0x70707066, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-refine-roll-downgrade", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-refine roll-downgrade account: %v", err)
	}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11246,
		Name:       "Downgrade Fail Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{ResultVnum: 11247, Cost: 1000, Probability: 75, FailResultVnum: 11240, Materials: []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}}},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11247, Name: "Unreached Downgrade Result Blade", Stackable: false, MaxCount: 1}
	failResultTemplate := itemcatalog.Template{Vnum: 11240, Name: "Downgraded Practice Blade", Stackable: false, MaxCount: 1}
	material := itemcatalog.Template{Vnum: 27001, Name: "Refine Material A", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{sourceTemplate, resultTemplate, failResultTemplate, material})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-refine roll-downgrade runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-refine-roll-downgrade", 0x70707066)
	defer closeSessionFlow(t, flow)

	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 6})))
	if err != nil || len(previewOut) != 1 {
		t.Fatalf("expected probability-75 fail_result_vnum preview to emit one frame, got %d err=%v", len(previewOut), err)
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, previewOut[0])); err != nil {
		t.Fatalf("decode probability-75 fail_result_vnum preview: %v", err)
	}

	confirmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 6})))
	if err != nil {
		t.Fatalf("unexpected probability-75 fail_result_vnum confirm packet error: %v", err)
	}
	if len(confirmOut) != 5 {
		t.Fatalf("expected probability-75 fail_result_vnum burst of 5 frames, got %d", len(confirmOut))
	}
	materialDel, err := itemproto.DecodeDel(decodeSingleFrame(t, confirmOut[0]))
	if err != nil {
		t.Fatalf("decode probability-75 fail_result_vnum material ITEM_DEL: %v", err)
	}
	if materialDel.Position.WindowType != itemproto.WindowInventory || materialDel.Position.Cell != 6 {
		t.Fatalf("unexpected probability-75 fail_result_vnum material delete position: %+v", materialDel.Position)
	}
	materialQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, confirmOut[1]))
	if err != nil {
		t.Fatalf("decode probability-75 fail_result_vnum material QUICKSLOT_DEL: %v", err)
	}
	if materialQuickslotDel.Position != 3 {
		t.Fatalf("unexpected probability-75 fail_result_vnum material quickslot delete: %+v", materialQuickslotDel)
	}
	resultSet, err := itemproto.DecodeSet(decodeSingleFrame(t, confirmOut[2]))
	if err != nil {
		t.Fatalf("decode probability-75 fail_result_vnum result ITEM_SET: %v", err)
	}
	if resultSet.Position.WindowType != itemproto.WindowInventory || resultSet.Position.Cell != 5 || resultSet.Vnum != 11240 || resultSet.Count != 1 {
		t.Fatalf("unexpected probability-75 fail_result_vnum result ITEM_SET: %+v", resultSet)
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, confirmOut[3]))
	if err != nil {
		t.Fatalf("decode probability-75 fail_result_vnum gold point change: %v", err)
	}
	if goldChange.VID != owner.VID || goldChange.Type != bootstrapGoldPointType || goldChange.Amount != -1000 || goldChange.Value != 4000 {
		t.Fatalf("unexpected probability-75 fail_result_vnum gold point change: %+v", goldChange)
	}
	assertRefineFailedCommandChat(t, confirmOut[4], 6, "probability-75 fail_result_vnum confirm")

	account, err := accounts.Load("item-refine-roll-downgrade")
	if err != nil {
		t.Fatalf("load probability-75 fail_result_vnum refine account: %v", err)
	}
	if account.Characters[0].Gold != 4000 {
		t.Fatalf("expected persisted gold 4000 after probability-75 fail_result_vnum, got %d", account.Characters[0].Gold)
	}
	wantInventory := []inventory.ItemInstance{{ID: 666, Vnum: 11240, Count: 1, Slot: 5}}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("expected source downgraded after probability-75 fail_result_vnum, got %#v", account.Characters[0].Inventory)
	}
	wantQuickslots := []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted quickslots after probability-75 fail_result_vnum: got %#v want %#v", account.Characters[0].Quickslots, wantQuickslots)
	}
}

func TestGameRuntimeItemRefineConfirmAfterPreviewProbability75FailResultVnumMissingTemplateFailsClosed(t *testing.T) {
	restore := QueueRefineConfirmRollForTest(76)
	t.Cleanup(restore)

	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineRollMiss", 0x01030767, 0x02040767, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 668, Vnum: 11248, Count: 1, Slot: 5},
		{ID: 669, Vnum: 27001, Count: 2, Slot: 6},
	}
	issuePeerTicket(t, ticketStore, "item-refine-roll-down-miss", 0x70707067, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-refine-roll-down-miss", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-refine roll-downgrade-missing account: %v", err)
	}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11248,
		Name:       "Missing Fail Result Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{ResultVnum: 11249, Cost: 1000, Probability: 75, FailResultVnum: 11239, Materials: []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}}},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11249, Name: "Unreached Missing Fail Result Blade", Stackable: false, MaxCount: 1}
	material := itemcatalog.Template{Vnum: 27001, Name: "Refine Material A", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{sourceTemplate, resultTemplate, material})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-refine roll-downgrade-missing runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-refine-roll-down-miss", 0x70707067)
	defer closeSessionFlow(t, flow)

	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 7})))
	if err != nil || len(previewOut) != 1 {
		t.Fatalf("expected missing fail_result preview to emit one frame, got %d err=%v", len(previewOut), err)
	}

	confirmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 7})))
	if err != nil {
		t.Fatalf("unexpected missing fail_result confirm packet error: %v", err)
	}
	if len(confirmOut) != 0 {
		t.Fatalf("expected missing fail_result confirm to fail closed with no frames, got %d", len(confirmOut))
	}

	account, err := accounts.Load("item-refine-roll-down-miss")
	if err != nil {
		t.Fatalf("load missing fail_result refine account: %v", err)
	}
	if account.Characters[0].Gold != 5000 {
		t.Fatalf("expected unchanged gold after missing fail_result confirm, got %d", account.Characters[0].Gold)
	}
	wantInventory := []inventory.ItemInstance{
		{ID: 668, Vnum: 11248, Count: 1, Slot: 5},
		{ID: 669, Vnum: 27001, Count: 2, Slot: 6},
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("expected unchanged inventory after missing fail_result confirm, got %#v", account.Characters[0].Inventory)
	}
}

func assertRefineSucceededCommandChat(t *testing.T, frame []byte, refineType uint8, label string) {
	t.Helper()
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, frame))
	if err != nil {
		t.Fatalf("decode %s RefineSuceeded chat: %v", label, err)
	}
	wantMessage := fmt.Sprintf("RefineSuceeded %d", refineType)
	if delivery.Type != chatproto.ChatTypeCommand || delivery.VID != 0 || delivery.Empire != 0 || delivery.Message != wantMessage {
		t.Fatalf("unexpected %s RefineSuceeded chat: %+v want message=%q", label, delivery, wantMessage)
	}
}
