package minimal

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
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
