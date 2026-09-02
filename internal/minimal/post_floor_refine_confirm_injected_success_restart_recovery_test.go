package minimal

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
)

func TestGameSessionFlowPostFloorRefineConfirmInjectedSuccessSucceedsAfterRestartHere(t *testing.T) {
	restore := QueueRefineConfirmRollForTest(75)
	t.Cleanup(restore)

	login := "pf-refine-confirm-inj-ok"
	loginKey := uint32(0x19191c34)
	owner := peerVisibilityCharacter("DeadRefineInjOKOwner", 0x01030c34, 0x02040c34, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 969, Vnum: 11256, Count: 1, Slot: 5},
		{ID: 970, Vnum: 27001, Count: 2, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11256,
		Name:       "Dead Guard Refine Injected Success Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{
			ResultVnum:  11257,
			Cost:        1000,
			Probability: 75,
			Materials:   []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}},
		},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11257, Name: "Dead Guard Refine Injected Success Result", Stackable: false, MaxCount: 1}
	materialTemplate := itemcatalog.Template{Vnum: 27001, Name: "Dead Guard Refine Injected Success Material", Stackable: true, MaxCount: 200}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, []itemcatalog.Template{sourceTemplate, resultTemplate, materialTemplate})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	refinePacket := itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 6})
	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-floor refineable injected-success REFINE preview dispatch error: %v", err)
	}
	if len(previewOut) != 0 {
		t.Fatalf("expected post-floor refineable injected-success REFINE preview to fail closed with no REFINE_INFORMATION_NEW, got %d frames", len(previewOut))
	}
	confirmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-floor refineable injected-success REFINE confirm dispatch error: %v", err)
	}
	if len(confirmOut) != 0 {
		t.Fatalf("expected post-floor refineable injected-success REFINE confirm to fail closed with no frames, got %d", len(confirmOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor refineable injected-success REFINE to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor refineable injected-success REFINE confirm")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor refine injected-success confirm: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor refine injected-success confirm, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-restart refineable injected-success REFINE preview: %v", err)
	}
	if len(reopenOut) != 1 {
		t.Fatalf("expected REFINE_INFORMATION_NEW after restart for injected-success path, got %d", len(reopenOut))
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, reopenOut[0])); err != nil {
		t.Fatalf("decode post-restart injected-success REFINE_INFORMATION_NEW: %v", err)
	}

	successOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-restart refineable injected-success REFINE confirm: %v", err)
	}
	assertPostFloorRefineConfirmInjectedSuccessBurst(t, successOut, owner.VID, 11257, -1000, 4000, "post-restart refine injected-success confirm")

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart refine injected-success confirm: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after refine injected-success floor, got %+v", wantHP, account.Characters[0])
	}
	wantInventory := []inventory.ItemInstance{{ID: 969, Vnum: 11257, Count: 1, Slot: 5}}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted inventory after post-restart refine injected-success:\n got: %+v\nwant: %+v", account.Characters[0].Inventory, wantInventory)
	}
	if account.Characters[0].Gold != 4000 {
		t.Fatalf("unexpected persisted gold after post-restart refine injected-success: got %d want 4000", account.Characters[0].Gold)
	}
	wantQuickslots := []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted quickslots after post-restart refine injected-success: got %#v want %#v", account.Characters[0].Quickslots, wantQuickslots)
	}
}

func TestGameSessionFlowPostFloorRefineConfirmInjectedSuccessSucceedsAfterRestartTown(t *testing.T) {
	restore := QueueRefineConfirmRollForTest(75)
	t.Cleanup(restore)

	login := "pf-refine-confirm-inj-ok-t"
	loginKey := uint32(0x19191c35)
	owner := peerVisibilityCharacter("DeadRefineInjOKTownOwner", 0x01030c35, 0x02040c35, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 971, Vnum: 11256, Count: 1, Slot: 5},
		{ID: 972, Vnum: 27001, Count: 2, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11256,
		Name:       "Dead Guard Refine Injected Success Town Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{
			ResultVnum:  11257,
			Cost:        1000,
			Probability: 75,
			Materials:   []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}},
		},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11257, Name: "Dead Guard Refine Injected Success Town Result", Stackable: false, MaxCount: 1}
	materialTemplate := itemcatalog.Template{Vnum: 27001, Name: "Dead Guard Refine Injected Success Town Material", Stackable: true, MaxCount: 200}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, []itemcatalog.Template{sourceTemplate, resultTemplate, materialTemplate})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	refinePacket := itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 6})
	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-floor town refineable injected-success REFINE preview dispatch error: %v", err)
	}
	if len(previewOut) != 0 {
		t.Fatalf("expected post-floor town refineable injected-success REFINE preview to fail closed with no REFINE_INFORMATION_NEW, got %d frames", len(previewOut))
	}
	confirmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-floor town refineable injected-success REFINE confirm dispatch error: %v", err)
	}
	if len(confirmOut) != 0 {
		t.Fatalf("expected post-floor town refineable injected-success REFINE confirm to fail closed with no frames, got %d", len(confirmOut))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town refineable injected-success REFINE confirm")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor refine injected-success confirm: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor refine injected-success confirm, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor refine injected-success confirm /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after refine injected-success floor, got %+v", selfAdd)
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
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after refine injected-success floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after refine injected-success floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-restart_town refineable injected-success REFINE preview: %v", err)
	}
	if len(reopenOut) != 1 {
		t.Fatalf("expected REFINE_INFORMATION_NEW after restart_town for injected-success path, got %d", len(reopenOut))
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, reopenOut[0])); err != nil {
		t.Fatalf("decode post-restart_town injected-success REFINE_INFORMATION_NEW: %v", err)
	}

	successOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-restart_town refineable injected-success REFINE confirm: %v", err)
	}
	assertPostFloorRefineConfirmInjectedSuccessBurst(t, successOut, owner.VID, 11257, -1000, 4000, "post-restart_town refine injected-success confirm")

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town refine injected-success confirm: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after refine injected-success floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after refine injected-success floor, got %+v", wantHP, account.Characters[0])
	}
	wantInventory := []inventory.ItemInstance{{ID: 971, Vnum: 11257, Count: 1, Slot: 5}}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted inventory after post-restart_town refine injected-success:\n got: %+v\nwant: %+v", account.Characters[0].Inventory, wantInventory)
	}
	if account.Characters[0].Gold != 4000 {
		t.Fatalf("unexpected persisted gold after post-restart_town refine injected-success: got %d want 4000", account.Characters[0].Gold)
	}
	wantQuickslots := []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted quickslots after post-restart_town refine injected-success: got %#v want %#v", account.Characters[0].Quickslots, wantQuickslots)
	}
}

func assertPostFloorRefineConfirmInjectedSuccessBurst(t *testing.T, frames [][]byte, ownerVID uint32, resultVnum uint32, goldAmount int32, goldValue int32, label string) {
	t.Helper()
	if len(frames) != 5 {
		t.Fatalf("expected %s burst of 5 frames, got %d", label, len(frames))
	}
	materialDel, err := itemproto.DecodeDel(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode %s material ITEM_DEL: %v", label, err)
	}
	if materialDel.Position.WindowType != itemproto.WindowInventory || materialDel.Position.Cell != 6 {
		t.Fatalf("unexpected %s material delete position: %+v", label, materialDel.Position)
	}
	materialQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s material QUICKSLOT_DEL: %v", label, err)
	}
	if materialQuickslotDel.Position != 3 {
		t.Fatalf("unexpected %s material quickslot delete: %+v", label, materialQuickslotDel)
	}
	resultSet, err := itemproto.DecodeSet(decodeSingleFrame(t, frames[2]))
	if err != nil {
		t.Fatalf("decode %s result ITEM_SET: %v", label, err)
	}
	if resultSet.Position.WindowType != itemproto.WindowInventory || resultSet.Position.Cell != 5 || resultSet.Vnum != resultVnum || resultSet.Count != 1 {
		t.Fatalf("unexpected %s result ITEM_SET: %+v", label, resultSet)
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, frames[3]))
	if err != nil {
		t.Fatalf("decode %s gold PLAYER_POINT_CHANGE: %v", label, err)
	}
	if goldChange.VID != ownerVID || goldChange.Type != bootstrapGoldPointType || goldChange.Amount != goldAmount || goldChange.Value != goldValue {
		t.Fatalf("unexpected %s gold point change: %+v", label, goldChange)
	}
	assertRefineSucceededCommandChat(t, frames[4], 6, label)
}
