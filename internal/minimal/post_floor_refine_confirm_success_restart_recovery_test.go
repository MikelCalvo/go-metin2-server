package minimal

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
)

func TestGameSessionFlowPostFloorRefineConfirmSucceedsAfterRestartHere(t *testing.T) {
	login := "pf-refine-confirm-ok"
	loginKey := uint32(0x19191c10)
	owner := peerVisibilityCharacter("DeadRefineConfirmOwner", 0x01030c10, 0x02040c10, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 941, Vnum: 11250, Count: 1, Slot: 5},
		{ID: 942, Vnum: 27001, Count: 2, Slot: 6},
	}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11250,
		Name:       "Dead Guard Refine Confirm Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{
			ResultVnum:  11251,
			Cost:        1000,
			Probability: 100,
			Materials:   []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}},
		},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11251, Name: "Dead Guard Refine Confirm Result", Stackable: false, MaxCount: 1}
	materialTemplate := itemcatalog.Template{Vnum: 27001, Name: "Dead Guard Refine Confirm Material", Stackable: true, MaxCount: 200}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, []itemcatalog.Template{sourceTemplate, resultTemplate, materialTemplate})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	refinePacket := itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})
	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-floor refineable REFINE preview dispatch error: %v", err)
	}
	if len(previewOut) != 0 {
		t.Fatalf("expected post-floor refineable REFINE preview to fail closed with no REFINE_INFORMATION_NEW, got %d frames", len(previewOut))
	}
	confirmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-floor refineable REFINE confirm dispatch error: %v", err)
	}
	if len(confirmOut) != 0 {
		t.Fatalf("expected post-floor refineable REFINE confirm to fail closed with no frames, got %d", len(confirmOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor refineable REFINE to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor refineable REFINE confirm")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor refine confirm: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor refine confirm, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-restart refineable REFINE preview: %v", err)
	}
	if len(reopenOut) != 1 {
		t.Fatalf("expected REFINE_INFORMATION_NEW after restart, got %d", len(reopenOut))
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, reopenOut[0])); err != nil {
		t.Fatalf("decode post-restart REFINE_INFORMATION_NEW: %v", err)
	}

	successOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-restart refineable REFINE confirm: %v", err)
	}
	assertPostFloorRefineConfirmSuccessBurst(t, successOut, owner.VID, 11251, -1000, 4000, "post-restart refine confirm")

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart refine confirm: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after refine confirm floor, got %+v", wantHP, account.Characters[0])
	}
	wantInventory := []inventory.ItemInstance{{ID: 941, Vnum: 11251, Count: 1, Slot: 5}}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted inventory after post-restart refine confirm:\n got: %+v\nwant: %+v", account.Characters[0].Inventory, wantInventory)
	}
	if account.Characters[0].Gold != 4000 {
		t.Fatalf("unexpected persisted gold after post-restart refine confirm: got %d want 4000", account.Characters[0].Gold)
	}
}

func TestGameSessionFlowPostFloorRefineConfirmSucceedsAfterRestartTown(t *testing.T) {
	login := "pf-refine-confirm-ok-t"
	loginKey := uint32(0x19191c11)
	owner := peerVisibilityCharacter("DeadRefineConfirmTownOwner", 0x01030c11, 0x02040c11, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 943, Vnum: 11250, Count: 1, Slot: 5},
		{ID: 944, Vnum: 27001, Count: 2, Slot: 6},
	}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11250,
		Name:       "Dead Guard Refine Confirm Town Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{
			ResultVnum:  11251,
			Cost:        1000,
			Probability: 100,
			Materials:   []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}},
		},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11251, Name: "Dead Guard Refine Confirm Town Result", Stackable: false, MaxCount: 1}
	materialTemplate := itemcatalog.Template{Vnum: 27001, Name: "Dead Guard Refine Confirm Town Material", Stackable: true, MaxCount: 200}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, []itemcatalog.Template{sourceTemplate, resultTemplate, materialTemplate})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	refinePacket := itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})
	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-floor town refineable REFINE preview dispatch error: %v", err)
	}
	if len(previewOut) != 0 {
		t.Fatalf("expected post-floor town refineable REFINE preview to fail closed with no REFINE_INFORMATION_NEW, got %d frames", len(previewOut))
	}
	confirmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-floor town refineable REFINE confirm dispatch error: %v", err)
	}
	if len(confirmOut) != 0 {
		t.Fatalf("expected post-floor town refineable REFINE confirm to fail closed with no frames, got %d", len(confirmOut))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town refineable REFINE confirm")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor refine confirm: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor refine confirm, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor refine confirm /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after refine confirm floor, got %+v", selfAdd)
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
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after refine confirm floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after refine confirm floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-restart_town refineable REFINE preview: %v", err)
	}
	if len(reopenOut) != 1 {
		t.Fatalf("expected REFINE_INFORMATION_NEW after restart_town, got %d", len(reopenOut))
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, reopenOut[0])); err != nil {
		t.Fatalf("decode post-restart_town REFINE_INFORMATION_NEW: %v", err)
	}

	successOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-restart_town refineable REFINE confirm: %v", err)
	}
	assertPostFloorRefineConfirmSuccessBurst(t, successOut, owner.VID, 11251, -1000, 4000, "post-restart_town refine confirm")

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town refine confirm: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after refine confirm floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after refine confirm floor, got %+v", wantHP, account.Characters[0])
	}
	wantInventory := []inventory.ItemInstance{{ID: 943, Vnum: 11251, Count: 1, Slot: 5}}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted inventory after post-restart_town refine confirm:\n got: %+v\nwant: %+v", account.Characters[0].Inventory, wantInventory)
	}
	if account.Characters[0].Gold != 4000 {
		t.Fatalf("unexpected persisted gold after post-restart_town refine confirm: got %d want 4000", account.Characters[0].Gold)
	}
}

func assertPostFloorRefineConfirmSuccessBurst(t *testing.T, frames [][]byte, ownerVID uint32, resultVnum uint32, goldAmount int32, goldValue int32, label string) {
	t.Helper()
	if len(frames) != 4 {
		t.Fatalf("expected %s burst of 4 frames, got %d", label, len(frames))
	}
	materialDel, err := itemproto.DecodeDel(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode %s material ITEM_DEL: %v", label, err)
	}
	if materialDel.Position.WindowType != itemproto.WindowInventory || materialDel.Position.Cell != 6 {
		t.Fatalf("unexpected %s material delete position: %+v", label, materialDel.Position)
	}
	resultSet, err := itemproto.DecodeSet(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s result ITEM_SET: %v", label, err)
	}
	if resultSet.Position.WindowType != itemproto.WindowInventory || resultSet.Position.Cell != 5 || resultSet.Vnum != resultVnum || resultSet.Count != 1 {
		t.Fatalf("unexpected %s result ITEM_SET: %+v", label, resultSet)
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, frames[2]))
	if err != nil {
		t.Fatalf("decode %s gold PLAYER_POINT_CHANGE: %v", label, err)
	}
	if goldChange.VID != ownerVID || goldChange.Type != bootstrapGoldPointType || goldChange.Amount != goldAmount || goldChange.Value != goldValue {
		t.Fatalf("unexpected %s gold point change: %+v", label, goldChange)
	}
	assertRefineSucceededCommandChat(t, frames[3], 3, label)
}
