package minimal

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPostFloorItemGiveFailsClosedBeforeAntiGiveFeedback(t *testing.T) {
	login := "post-floor-item-give-owner"
	loginKey := uint32(0x19191a40)
	owner := peerVisibilityCharacter("DeadGiveOwner", 0x01030a40, 0x02040a40, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 801, Vnum: 27042, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}
	template := itemcatalog.Template{
		Vnum:           27042,
		Name:           "Dead Guard Gift Potion",
		Stackable:      true,
		MaxCount:       200,
		AntiGive:       true,
		GiveRejectText: "You cannot give this item.",
	}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, []itemcatalog.Template{template})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{
		TargetVID: targetVID,
		Position:  itemproto.InventoryPosition(5),
		Count:     1,
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor ITEM_GIVE dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor ITEM_GIVE to fail closed before anti-give feedback, got %d frames", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor ITEM_GIVE to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor ITEM_GIVE")
}

func TestGameSessionFlowPostFloorItemRefineFailsClosedBeforeRejectFeedback(t *testing.T) {
	login := "post-floor-item-refine-owner"
	loginKey := uint32(0x19191a50)
	owner := peerVisibilityCharacter("DeadRefineOwner", 0x01030a50, 0x02040a50, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 802, Vnum: 11201, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}
	template := itemcatalog.Template{
		Vnum:             11201,
		Name:             "Dead Guard Practice Blade",
		Stackable:        false,
		MaxCount:         1,
		RefineRejectText: "This item cannot be refined yet.",
	}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, []itemcatalog.Template{template})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 2})))
	if err != nil {
		t.Fatalf("unexpected post-floor REFINE dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor REFINE to fail closed before template reject feedback, got %d frames", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor REFINE to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor REFINE")
}

func TestGameSessionFlowPostFloorStoragePacketsFailClosedWithoutMutation(t *testing.T) {
	login := "post-floor-store"
	loginKey := uint32(0x19191a60)
	owner := peerVisibilityCharacter("DeadStorageOwner", 0x01030a60, 0x02040a60, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 803, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, nil)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	requests := []struct {
		name string
		raw  []byte
	}{
		{name: "safebox checkin", raw: itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{SafeSlot: 7, Position: itemproto.InventoryPosition(5)})},
		{name: "safebox checkout", raw: itemproto.EncodeClientSafeboxCheckout(itemproto.ClientSafeboxCheckoutPacket{SafeSlot: 8, Position: itemproto.InventoryPosition(6)})},
		{name: "safebox item move", raw: itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{Source: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 7}, Destination: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 8}, Count: 1})},
		{name: "mall checkout", raw: itemproto.EncodeClientMallCheckout(itemproto.ClientMallCheckoutPacket{MallSlot: 4, Position: itemproto.InventoryPosition(9)})},
	}
	for _, request := range requests {
		t.Run(request.name, func(t *testing.T) {
			out, err := flow.HandleClientFrame(decodeSingleFrame(t, request.raw))
			if err != nil {
				t.Fatalf("unexpected post-floor %s dispatch error: %v", request.name, err)
			}
			if len(out) != 0 {
				t.Fatalf("expected post-floor %s to fail closed, got %d frames", request.name, len(out))
			}
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected post-floor %s to queue no frames, got %d", request.name, len(queued))
			}
			assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor "+request.name)
		})
	}
}

func TestGameSessionFlowPostFloorSafeboxCheckinFailsClosedBeforeAntiSafeboxFeedback(t *testing.T) {
	login := "post-floor-safe"
	loginKey := uint32(0x19191a70)
	owner := peerVisibilityCharacter("DeadSafeboxOwner", 0x01030a70, 0x02040a70, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 804, Vnum: 71127, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}
	template := itemcatalog.Template{
		Vnum:              71127,
		Name:              "Dead Guard Storage Charm",
		Stackable:         false,
		MaxCount:          1,
		AntiSafebox:       true,
		SafeboxRejectText: "This item cannot be placed in storage.",
	}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, []itemcatalog.Template{template})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{SafeSlot: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected post-floor SAFEBOX_CHECKIN dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor SAFEBOX_CHECKIN to fail closed before anti-safebox feedback, got %d frames", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor SAFEBOX_CHECKIN to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor SAFEBOX_CHECKIN")
}

func TestGameSessionFlowPostFloorItemMoveFailsClosed(t *testing.T) {
	login := "post-floor-item-move"
	loginKey := uint32(0x19191b50)
	owner := peerVisibilityCharacter("DeadItemMoveOwner", 0x01030b50, 0x02040b50, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 901, Vnum: 27001, Count: 3, Slot: 5}}
	templates := []itemcatalog.Template{{Vnum: 27001, Name: "Post Floor Move Potion", Stackable: true, MaxCount: 200}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor ITEM_MOVE dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor ITEM_MOVE to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor ITEM_MOVE to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor ITEM_MOVE")

	slashOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/inventory_move 5 6",
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor /inventory_move dispatch error: %v", err)
	}
	if len(slashOut) != 0 {
		t.Fatalf("expected post-floor /inventory_move to fail closed with no frames, got %d", len(slashOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor /inventory_move to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor /inventory_move")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor ITEM_MOVE: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor ITEM_MOVE, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart ITEM_MOVE: %v", err)
	}
	assertPostFloorItemMoveSuccessBurst(t, reuseOut, 5, 6, 27001, 3, "post-restart ITEM_MOVE")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart ITEM_MOVE: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after ITEM_MOVE floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.Inventory = []inventory.ItemInstance{{ID: 901, Vnum: 27001, Count: 3, Slot: 6}}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart ITEM_MOVE persists destination slot")
}

func TestGameSessionFlowPostFloorItemMoveFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-item-move-town"
	loginKey := uint32(0x19191b51)
	owner := peerVisibilityCharacter("DeadItemMoveTownOwner", 0x01030b51, 0x02040b51, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 902, Vnum: 27001, Count: 3, Slot: 5}}
	templates := []itemcatalog.Template{{Vnum: 27001, Name: "Post Floor Move Potion", Stackable: true, MaxCount: 200}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor town ITEM_MOVE dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town ITEM_MOVE to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town ITEM_MOVE to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town ITEM_MOVE")

	slashOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/inventory_move 5 6",
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor town /inventory_move dispatch error: %v", err)
	}
	if len(slashOut) != 0 {
		t.Fatalf("expected post-floor town /inventory_move to fail closed with no frames, got %d", len(slashOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town /inventory_move to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town /inventory_move")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor ITEM_MOVE: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor ITEM_MOVE, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor ITEM_MOVE /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after ITEM_MOVE floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after ITEM_MOVE floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after ITEM_MOVE floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town ITEM_MOVE: %v", err)
	}
	assertPostFloorItemMoveSuccessBurst(t, reuseOut, 5, 6, 27001, 3, "post-restart_town ITEM_MOVE")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town ITEM_MOVE: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after ITEM_MOVE floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after ITEM_MOVE floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Inventory = []inventory.ItemInstance{{ID: 902, Vnum: 27001, Count: 3, Slot: 6}}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town ITEM_MOVE persists destination slot")
}

func TestGameSessionFlowPostFloorEquipItemFailsClosed(t *testing.T) {
	login := "post-floor-equip-item"
	loginKey := uint32(0x19191b60)
	owner := peerVisibilityCharacter("DeadEquipOwner", 0x01030b60, 0x02040b60, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 12200, Count: 1, Slot: 8}}
	owner.Equipment = []inventory.ItemInstance{}
	templates := []itemcatalog.Template{{
		Vnum:      12200,
		Name:      "Post Floor Practice Blade",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: "weapon",
		EquipEffect: &itemcatalog.PointEffect{
			PointType:  bootstrapPlayerPointType,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: 10,
		},
	}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/equip_item 8 weapon",
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor /equip_item dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor /equip_item to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor /equip_item to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor /equip_item")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor /equip_item: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor /equip_item, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/equip_item 8 weapon",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart /equip_item: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	assertPostFloorEquipItemSuccessBurst(t, reuseOut, owner.VID, owner.MainPart, owner.HairPart, 12200, wantHP+10, "post-restart /equip_item")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart /equip_item: %v", err)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP+10 {
		t.Fatalf("expected /restart_here + /equip_item to persist recovered owner HP %d after equip floor, got %+v", wantHP+10, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP + 10
	want.Inventory = []inventory.ItemInstance{}
	want.Equipment = []inventory.ItemInstance{{ID: 1001, Vnum: 12200, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart /equip_item persists equipped weapon")
}

func TestGameSessionFlowPostFloorEquipItemFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-equip-item-town"
	loginKey := uint32(0x19191b61)
	owner := peerVisibilityCharacter("DeadEquipTownOwner", 0x01030b61, 0x02040b61, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1002, Vnum: 12200, Count: 1, Slot: 8}}
	owner.Equipment = []inventory.ItemInstance{}
	templates := []itemcatalog.Template{{
		Vnum:      12200,
		Name:      "Post Floor Practice Blade",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: "weapon",
		EquipEffect: &itemcatalog.PointEffect{
			PointType:  bootstrapPlayerPointType,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: 10,
		},
	}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/equip_item 8 weapon",
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor town /equip_item dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town /equip_item to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town /equip_item to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town /equip_item")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor /equip_item: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor /equip_item, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor /equip_item /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after equip floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after equip floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after equip floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/equip_item 8 weapon",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town /equip_item: %v", err)
	}
	assertPostFloorEquipItemSuccessBurst(t, reuseOut, owner.VID, owner.MainPart, owner.HairPart, 12200, wantHP+10, "post-restart_town /equip_item")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town /equip_item: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after equip floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP+10 {
		t.Fatalf("expected /restart_town + /equip_item to persist recovered owner HP %d after equip floor, got %+v", wantHP+10, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP + 10
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Inventory = []inventory.ItemInstance{}
	want.Equipment = []inventory.ItemInstance{{ID: 1002, Vnum: 12200, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town /equip_item persists equipped weapon")
}

func TestGameSessionFlowPostFloorUnequipItemFailsClosed(t *testing.T) {
	login := "post-floor-unequip-item"
	loginKey := uint32(0x19191b62)
	owner := peerVisibilityCharacter("DeadUnequipOwner", 0x01030b62, 0x02040b62, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{}
	owner.Equipment = []inventory.ItemInstance{{ID: 1003, Vnum: 12200, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	templates := []itemcatalog.Template{{
		Vnum:      12200,
		Name:      "Post Floor Practice Blade",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: "weapon",
		EquipEffect: &itemcatalog.PointEffect{
			PointType:  bootstrapPlayerPointType,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: 10,
		},
	}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/unequip_item weapon 4",
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor /unequip_item dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor /unequip_item to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor /unequip_item to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor /unequip_item")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor /unequip_item: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor /unequip_item, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/unequip_item weapon 4",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart /unequip_item: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	assertPostFloorUnequipItemSuccessBurst(t, reuseOut, owner.VID, owner.MainPart, owner.HairPart, 12200, 4, wantHP-10, "post-restart /unequip_item")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart /unequip_item: %v", err)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP-10 {
		t.Fatalf("expected /restart_here + /unequip_item to persist recovered owner HP %d after unequip floor, got %+v", wantHP-10, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP - 10
	want.Inventory = []inventory.ItemInstance{{ID: 1003, Vnum: 12200, Count: 1, Slot: 4}}
	want.Equipment = []inventory.ItemInstance{}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart /unequip_item persists carried weapon")
}

func TestGameSessionFlowPostFloorUnequipItemFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-unequip-item-town"
	loginKey := uint32(0x19191b63)
	owner := peerVisibilityCharacter("DeadUnequipTownOwner", 0x01030b63, 0x02040b63, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{}
	owner.Equipment = []inventory.ItemInstance{{ID: 1004, Vnum: 12200, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	templates := []itemcatalog.Template{{
		Vnum:      12200,
		Name:      "Post Floor Practice Blade",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: "weapon",
		EquipEffect: &itemcatalog.PointEffect{
			PointType:  bootstrapPlayerPointType,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: 10,
		},
	}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/unequip_item weapon 4",
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor town /unequip_item dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town /unequip_item to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town /unequip_item to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town /unequip_item")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor /unequip_item: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor /unequip_item, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor /unequip_item /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after unequip floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after unequip floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after unequip floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/unequip_item weapon 4",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town /unequip_item: %v", err)
	}
	assertPostFloorUnequipItemSuccessBurst(t, reuseOut, owner.VID, owner.MainPart, owner.HairPart, 12200, 4, wantHP-10, "post-restart_town /unequip_item")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town /unequip_item: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after unequip floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP-10 {
		t.Fatalf("expected /restart_town + /unequip_item to persist recovered owner HP %d after unequip floor, got %+v", wantHP-10, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP - 10
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Inventory = []inventory.ItemInstance{{ID: 1004, Vnum: 12200, Count: 1, Slot: 4}}
	want.Equipment = []inventory.ItemInstance{}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town /unequip_item persists carried weapon")
}

func assertPostFloorUnequipItemSuccessBurst(t *testing.T, frames [][]byte, ownerVID uint32, mainPart, hairPart, weaponVnum uint16, inventorySlot uint16, wantHP int32, context string) {
	t.Helper()
	if len(frames) != 4 {
		t.Fatalf("expected %s to emit delete+set+point-change+update, got %d frames", context, len(frames))
	}
	weaponPos, err := itemproto.EquipmentPosition(4)
	if err != nil {
		t.Fatalf("build %s weapon equipment position: %v", context, err)
	}
	itemDel, err := itemproto.DecodeDel(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode %s equipment delete: %v", context, err)
	}
	if itemDel.Position != weaponPos {
		t.Fatalf("unexpected %s equipment delete position: %+v", context, itemDel.Position)
	}
	itemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s inventory set: %v", context, err)
	}
	if itemSet.Position != itemproto.InventoryPosition(inventorySlot) || itemSet.Vnum != uint32(weaponVnum) || itemSet.Count != 1 {
		t.Fatalf("unexpected %s inventory set frame: %+v", context, itemSet)
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, frames[2]))
	if err != nil {
		t.Fatalf("decode %s point-change: %v", context, err)
	}
	if pointChange.VID != ownerVID || pointChange.Type != bootstrapPlayerPointType || pointChange.Amount != -10 || pointChange.Value != wantHP {
		t.Fatalf("unexpected %s point-change: %+v", context, pointChange)
	}
	update, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, frames[3]))
	if err != nil {
		t.Fatalf("decode %s character update: %v", context, err)
	}
	wantParts := [worldproto.CharacterEquipmentPartCount]uint16{mainPart, 0, 0, hairPart}
	if update.VID != ownerVID || update.Parts != wantParts {
		t.Fatalf("unexpected %s appearance update: %+v want parts %+v", context, update, wantParts)
	}
}

func assertPostFloorEquipItemSuccessBurst(t *testing.T, frames [][]byte, ownerVID uint32, mainPart, hairPart, weaponVnum uint16, wantHP int32, context string) {
	t.Helper()
	if len(frames) != 4 {
		t.Fatalf("expected %s to emit delete+set+point-change+update, got %d frames", context, len(frames))
	}
	itemDel, err := itemproto.DecodeDel(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode %s item delete: %v", context, err)
	}
	if itemDel.Position != itemproto.InventoryPosition(8) {
		t.Fatalf("unexpected %s item delete position: %+v", context, itemDel.Position)
	}
	weaponPos, err := itemproto.EquipmentPosition(4)
	if err != nil {
		t.Fatalf("build %s weapon equipment position: %v", context, err)
	}
	itemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s item set: %v", context, err)
	}
	if itemSet.Position != weaponPos || itemSet.Vnum != uint32(weaponVnum) || itemSet.Count != 1 {
		t.Fatalf("unexpected %s item set frame: %+v", context, itemSet)
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, frames[2]))
	if err != nil {
		t.Fatalf("decode %s point-change: %v", context, err)
	}
	if pointChange.VID != ownerVID || pointChange.Type != bootstrapPlayerPointType || pointChange.Amount != 10 || pointChange.Value != wantHP {
		t.Fatalf("unexpected %s point-change: %+v", context, pointChange)
	}
	update, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, frames[3]))
	if err != nil {
		t.Fatalf("decode %s character update: %v", context, err)
	}
	wantParts := [worldproto.CharacterEquipmentPartCount]uint16{mainPart, weaponVnum, 0, hairPart}
	if update.VID != ownerVID || update.Parts != wantParts {
		t.Fatalf("unexpected %s appearance update: %+v want parts %+v", context, update, wantParts)
	}
}

func assertPostFloorItemMoveSuccessBurst(t *testing.T, frames [][]byte, sourceSlot, destinationSlot uint16, vnum uint32, count uint8, context string) {
	t.Helper()
	if len(frames) < 2 {
		t.Fatalf("expected %s to emit item delete + item set, got %d frames", context, len(frames))
	}
	itemDel, err := itemproto.DecodeDel(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode %s item delete: %v", context, err)
	}
	if itemDel.Position != itemproto.InventoryPosition(sourceSlot) {
		t.Fatalf("unexpected %s item delete position: %+v", context, itemDel.Position)
	}
	itemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s item set: %v", context, err)
	}
	if itemSet.Position != itemproto.InventoryPosition(destinationSlot) || itemSet.Vnum != vnum || itemSet.Count != count {
		t.Fatalf("unexpected %s item set frame: %+v", context, itemSet)
	}
}

func newPostFloorItemGuardRuntime(t *testing.T, login string, loginKey uint32, owner loginticket.Character, templates []itemcatalog.Template) (*gameRuntime, accountstore.Store, uint32) {
	t.Helper()
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor item-guard account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, templates)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected post-floor item-guard runtime error: %v", err)
	}

	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_item_guard",
		Name:          "PracticeMobPostFloorItemGuard",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import post-floor item-guard practice mob: %v", err)
	}
	// ImportContentBundle replaces item templates from the bundle; re-seed the
	// authored templates needed by post-floor reopen recovery proofs.
	if len(templates) > 0 {
		if err := runtime.replaceItemTemplates(itemcatalog.Snapshot{Templates: templates}); err != nil {
			t.Fatalf("restore post-floor item-guard templates after content import: %v", err)
		}
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor item-guard practice mob, got %#v", actors)
	}
	return runtime, accounts, uint32(actors[0].EntityID)
}

func drivePracticeMobOwnerToBootstrapHPFloor(t *testing.T, flow service.SessionFlow, owner loginticket.Character, targetVID uint32) {
	t.Helper()
	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection before post-floor item guard: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected one target-selection frame before post-floor item guard, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected practice-mob attack before post-floor item guard: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, point-change, self dead, and clear-target frames at HP floor, got %d frames", len(attackOut))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode post-floor item guard point-change: %v", err)
	}
	if pointChange.VID != owner.VID || pointChange.Type != bootstrapPlayerPointType || pointChange.Amount != bootstrapPracticeMobRetaliationPointDelta || pointChange.Value != 0 {
		t.Fatalf("unexpected post-floor item guard point-change: %+v", pointChange)
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, attackOut[2]))
	if err != nil {
		t.Fatalf("decode post-floor item guard self dead: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected self DEAD for owner %#08x, got %#08x", owner.VID, dead.VID)
	}
	clearTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[3]))
	if err != nil {
		t.Fatalf("decode post-floor item guard target clear: %v", err)
	}
	if clearTarget.TargetVID != 0 || clearTarget.HPPercent != 0 {
		t.Fatalf("expected post-floor item guard to clear active target, got %+v", clearTarget)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after post-floor item guard owner death without peers, got %d", len(queued))
	}
}

func assertPostFloorItemGuardAccountUnchanged(t *testing.T, accounts accountstore.Store, login string, want loginticket.Character, context string) {
	t.Helper()
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted %s account: %v", context, err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected one persisted %s character, got %d", context, len(persisted.Characters))
	}
	got := persisted.Characters[0]
	if !samePostFloorItemGuardInventory(got.Inventory, want.Inventory) {
		t.Fatalf("%s mutated inventory: got %+v want %+v", context, got.Inventory, want.Inventory)
	}
	if !samePostFloorItemGuardQuickslots(got.Quickslots, want.Quickslots) {
		t.Fatalf("%s mutated quickslots: got %+v want %+v", context, got.Quickslots, want.Quickslots)
	}
	if !samePostFloorItemGuardInventory(got.Equipment, want.Equipment) {
		t.Fatalf("%s mutated equipment: got %+v want %+v", context, got.Equipment, want.Equipment)
	}
	if got.Gold != want.Gold {
		t.Fatalf("%s mutated gold: got %d want %d", context, got.Gold, want.Gold)
	}
	if got.Points[bootstrapPlayerPointValueIndex] != 0 {
		t.Fatalf("%s expected persisted death-floor HP 0, got %d", context, got.Points[bootstrapPlayerPointValueIndex])
	}
}

func samePostFloorItemGuardInventory(got []inventory.ItemInstance, want []inventory.ItemInstance) bool {
	if len(got) == 0 && len(want) == 0 {
		return true
	}
	return reflect.DeepEqual(got, want)
}

func samePostFloorItemGuardQuickslots(got []loginticket.Quickslot, want []loginticket.Quickslot) bool {
	if len(got) == 0 && len(want) == 0 {
		return true
	}
	return reflect.DeepEqual(got, want)
}
