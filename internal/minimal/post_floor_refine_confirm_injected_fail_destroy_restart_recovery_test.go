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

func TestGameSessionFlowPostFloorRefineConfirmInjectedFailDestroySucceedsAfterRestartHere(t *testing.T) {
	restore := QueueRefineConfirmRollForTest(76)
	t.Cleanup(restore)

	login := "pf-refine-confirm-inj-fail"
	loginKey := uint32(0x19191c36)
	owner := peerVisibilityCharacter("DeadRefineInjFailOwner", 0x01030c36, 0x02040c36, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 973, Vnum: 11258, Count: 1, Slot: 5},
		{ID: 974, Vnum: 27001, Count: 2, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11258,
		Name:       "Dead Guard Refine Injected Fail Destroy Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{
			ResultVnum:  11259,
			Cost:        1000,
			Probability: 75,
			Materials:   []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}},
		},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11259, Name: "Dead Guard Refine Injected Fail Destroy Result", Stackable: false, MaxCount: 1}
	materialTemplate := itemcatalog.Template{Vnum: 27001, Name: "Dead Guard Refine Injected Fail Destroy Material", Stackable: true, MaxCount: 200}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, []itemcatalog.Template{sourceTemplate, resultTemplate, materialTemplate})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	refinePacket := itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 4})
	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-floor refineable injected-fail destroy REFINE preview dispatch error: %v", err)
	}
	if len(previewOut) != 0 {
		t.Fatalf("expected post-floor refineable injected-fail destroy REFINE preview to fail closed with no REFINE_INFORMATION_NEW, got %d frames", len(previewOut))
	}
	confirmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-floor refineable injected-fail destroy REFINE confirm dispatch error: %v", err)
	}
	if len(confirmOut) != 0 {
		t.Fatalf("expected post-floor refineable injected-fail destroy REFINE confirm to fail closed with no frames, got %d", len(confirmOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor refineable injected-fail destroy REFINE to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor refineable injected-fail destroy REFINE confirm")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor refine injected-fail destroy confirm: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor refine injected-fail destroy confirm, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-restart refineable injected-fail destroy REFINE preview: %v", err)
	}
	if len(reopenOut) != 1 {
		t.Fatalf("expected REFINE_INFORMATION_NEW after restart for injected-fail destroy path, got %d", len(reopenOut))
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, reopenOut[0])); err != nil {
		t.Fatalf("decode post-restart injected-fail destroy REFINE_INFORMATION_NEW: %v", err)
	}

	destroyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-restart refineable injected-fail destroy REFINE confirm: %v", err)
	}
	assertPostFloorRefineConfirmDestroyBurst(t, destroyOut, owner.VID, -1000, 4000, "post-restart refine injected-fail destroy confirm")

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart refine injected-fail destroy confirm: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after refine injected-fail destroy floor, got %+v", wantHP, account.Characters[0])
	}
	if len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected empty persisted inventory after post-restart refine injected-fail destroy, got %#v", account.Characters[0].Inventory)
	}
	if account.Characters[0].Gold != 4000 {
		t.Fatalf("unexpected persisted gold after post-restart refine injected-fail destroy: got %d want 4000", account.Characters[0].Gold)
	}
	wantQuickslots := []loginticket.Quickslot{{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5}}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted quickslots after post-restart refine injected-fail destroy: got %#v want %#v", account.Characters[0].Quickslots, wantQuickslots)
	}
}

func TestGameSessionFlowPostFloorRefineConfirmInjectedFailDestroySucceedsAfterRestartTown(t *testing.T) {
	restore := QueueRefineConfirmRollForTest(76)
	t.Cleanup(restore)

	login := "pf-refine-confirm-inj-fail-t"
	loginKey := uint32(0x19191c37)
	owner := peerVisibilityCharacter("DeadRefineInjFailTownOwner", 0x01030c37, 0x02040c37, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 975, Vnum: 11258, Count: 1, Slot: 5},
		{ID: 976, Vnum: 27001, Count: 2, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11258,
		Name:       "Dead Guard Refine Injected Fail Destroy Town Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{
			ResultVnum:  11259,
			Cost:        1000,
			Probability: 75,
			Materials:   []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}},
		},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11259, Name: "Dead Guard Refine Injected Fail Destroy Town Result", Stackable: false, MaxCount: 1}
	materialTemplate := itemcatalog.Template{Vnum: 27001, Name: "Dead Guard Refine Injected Fail Destroy Town Material", Stackable: true, MaxCount: 200}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, []itemcatalog.Template{sourceTemplate, resultTemplate, materialTemplate})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	refinePacket := itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 4})
	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-floor town refineable injected-fail destroy REFINE preview dispatch error: %v", err)
	}
	if len(previewOut) != 0 {
		t.Fatalf("expected post-floor town refineable injected-fail destroy REFINE preview to fail closed with no REFINE_INFORMATION_NEW, got %d frames", len(previewOut))
	}
	confirmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-floor town refineable injected-fail destroy REFINE confirm dispatch error: %v", err)
	}
	if len(confirmOut) != 0 {
		t.Fatalf("expected post-floor town refineable injected-fail destroy REFINE confirm to fail closed with no frames, got %d", len(confirmOut))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town refineable injected-fail destroy REFINE confirm")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor refine injected-fail destroy confirm: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor refine injected-fail destroy confirm, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor refine injected-fail destroy confirm /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after refine injected-fail destroy floor, got %+v", selfAdd)
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
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after refine injected-fail destroy floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after refine injected-fail destroy floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-restart_town refineable injected-fail destroy REFINE preview: %v", err)
	}
	if len(reopenOut) != 1 {
		t.Fatalf("expected REFINE_INFORMATION_NEW after restart_town for injected-fail destroy path, got %d", len(reopenOut))
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, reopenOut[0])); err != nil {
		t.Fatalf("decode post-restart_town injected-fail destroy REFINE_INFORMATION_NEW: %v", err)
	}

	destroyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, refinePacket))
	if err != nil {
		t.Fatalf("unexpected post-restart_town refineable injected-fail destroy REFINE confirm: %v", err)
	}
	assertPostFloorRefineConfirmDestroyBurst(t, destroyOut, owner.VID, -1000, 4000, "post-restart_town refine injected-fail destroy confirm")

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town refine injected-fail destroy confirm: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after refine injected-fail destroy floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after refine injected-fail destroy floor, got %+v", wantHP, account.Characters[0])
	}
	if len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected empty persisted inventory after post-restart_town refine injected-fail destroy, got %#v", account.Characters[0].Inventory)
	}
	if account.Characters[0].Gold != 4000 {
		t.Fatalf("unexpected persisted gold after post-restart_town refine injected-fail destroy: got %d want 4000", account.Characters[0].Gold)
	}
	wantQuickslots := []loginticket.Quickslot{{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5}}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted quickslots after post-restart_town refine injected-fail destroy: got %#v want %#v", account.Characters[0].Quickslots, wantQuickslots)
	}
}
