package minimal

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
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

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor /open_safebox: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor /open_safebox, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart /open_safebox: %v", err)
	}
	if len(reopenOut) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE after restart, got %d", len(reopenOut))
	}
	size, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, reopenOut[0]))
	if err != nil {
		t.Fatalf("decode post-restart SAFEBOX_SIZE: %v", err)
	}
	if size != (itemproto.SafeboxSizePacket{Size: bootstrapSafeboxOpenMinSize}) {
		t.Fatalf("unexpected post-restart SAFEBOX_SIZE: %+v", size)
	}
	money, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode post-restart SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if money != (itemproto.SafeboxMoneyChangePacket{Money: 0}) {
		t.Fatalf("unexpected post-restart SAFEBOX_MONEY_CHANGE: %+v", money)
	}
	assertCloseSafeboxCommandChat(t, flow, "/close_safebox", "post-restart open-safebox close")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart /open_safebox: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after open-safebox floor, got %+v", wantHP, account.Characters[0])
	}
	if account.Characters[0].Gold != owner.Gold {
		t.Fatalf("carried gold after post-restart /open_safebox=%d want %d", account.Characters[0].Gold, owner.Gold)
	}
}

func TestGameSessionFlowPostFloorOpenSafeboxFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-open-safebox-town"
	loginKey := uint32(0x19191b11)
	owner := peerVisibilityCharacter("DeadOpenSafeboxTownOwner", 0x01030b11, 0x02040b11, 1100, 2100, 0, 101, 201)
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
		t.Fatalf("unexpected post-floor town /open_safebox dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town /open_safebox to fail closed with no SAFEBOX_SIZE frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town /open_safebox to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town /open_safebox")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor /open_safebox: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor /open_safebox, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor /open_safebox /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after open-safebox floor, got %+v", selfAdd)
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
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after open-safebox floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after open-safebox floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town /open_safebox: %v", err)
	}
	if len(reopenOut) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE after restart_town, got %d", len(reopenOut))
	}
	size, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, reopenOut[0]))
	if err != nil {
		t.Fatalf("decode post-restart_town SAFEBOX_SIZE: %v", err)
	}
	if size != (itemproto.SafeboxSizePacket{Size: bootstrapSafeboxOpenMinSize}) {
		t.Fatalf("unexpected post-restart_town SAFEBOX_SIZE: %+v", size)
	}
	money, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode post-restart_town SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if money != (itemproto.SafeboxMoneyChangePacket{Money: 0}) {
		t.Fatalf("unexpected post-restart_town SAFEBOX_MONEY_CHANGE: %+v", money)
	}
	assertCloseSafeboxCommandChat(t, flow, "/close_safebox", "post-restart_town open-safebox close")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town /open_safebox: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after open-safebox floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after open-safebox floor, got %+v", wantHP, account.Characters[0])
	}
	if account.Characters[0].Gold != owner.Gold {
		t.Fatalf("carried gold after post-restart_town /open_safebox=%d want %d", account.Characters[0].Gold, owner.Gold)
	}
}

func TestGameSessionFlowPostFloorSafeboxPasswordOpenFailsClosed(t *testing.T) {
	login := "pf-open-safebox-pwd"
	loginKey := uint32(0x19191b15)
	owner := peerVisibilityCharacter("DeadOpenSafeboxPwdOwner", 0x01030b15, 0x02040b15, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 12345
	runtime, accounts, targetVID, warehouseVID, _ := newPostFloorSafeboxPasswordRuntime(t, login, loginKey, owner)
	currentTime := time.Unix(1700002715, 0)
	runtime.now = func() time.Time { return currentTime }
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(enterOut) < 8 {
		t.Fatalf("expected owner bootstrap with visible practice mob and warehouse, got %d frames", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	interactOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: warehouseVID})))
	if err != nil {
		t.Fatalf("unexpected warehouse interact before post-floor password open: %v", err)
	}
	if len(interactOut) != 1 {
		t.Fatalf("expected ShowMeSafeboxPassword before post-floor password open, got %d frames", len(interactOut))
	}
	prompt, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, interactOut[0]))
	if err != nil {
		t.Fatalf("decode ShowMeSafeboxPassword before post-floor password open: %v", err)
	}
	if prompt.Type != chatproto.ChatTypeCommand || prompt.Message != safeboxShowPasswordCommandMessage {
		t.Fatalf("unexpected password prompt before post-floor password open: %+v", prompt)
	}

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password " + safeboxstore.DefaultPassword,
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor /safebox_password dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor /safebox_password to fail closed with no SAFEBOX_SIZE frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor /safebox_password to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor /safebox_password")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after pending-password floor: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after pending-password floor, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	staleOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password " + safeboxstore.DefaultPassword,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart stale /safebox_password dispatch error: %v", err)
	}
	if len(staleOut) != 0 {
		t.Fatalf("expected post-restart stale /safebox_password to stay fail-closed until fresh warehouse interact, got %d frames", len(staleOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-restart stale /safebox_password to queue no frames, got %d", len(queued))
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown + time.Nanosecond)
	rechallengeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: warehouseVID})))
	if err != nil {
		t.Fatalf("unexpected warehouse re-interact after restart: %v", err)
	}
	if len(rechallengeOut) != 1 {
		t.Fatalf("expected fresh ShowMeSafeboxPassword after restart, got %d frames", len(rechallengeOut))
	}
	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password " + safeboxstore.DefaultPassword,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart fresh /safebox_password open: %v", err)
	}
	if len(openOut) < 2 {
		t.Fatalf("expected SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE after fresh post-restart password, got %d", len(openOut))
	}
	size, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, openOut[0]))
	if err != nil {
		t.Fatalf("decode SAFEBOX_SIZE after fresh post-restart password: %v", err)
	}
	if size != (itemproto.SafeboxSizePacket{Size: 2}) {
		t.Fatalf("unexpected SAFEBOX_SIZE after fresh post-restart password: %+v", size)
	}
}

func TestGameSessionFlowPostFloorSafeboxPasswordOpenFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-open-safebox-pwd-town"
	loginKey := uint32(0x19191b1a)
	owner := peerVisibilityCharacter("DeadOpenSafeboxPwdTownOwner", 0x01030b1a, 0x02040b1a, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 12345
	runtime, accounts, targetVID, sourceWarehouseVID, townWarehouseVID := newPostFloorSafeboxPasswordRuntime(t, login, loginKey, owner)
	currentTime := time.Unix(1700002716, 0)
	runtime.now = func() time.Time { return currentTime }
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(enterOut) < 8 {
		t.Fatalf("expected owner bootstrap with visible practice mob and warehouse, got %d frames", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	interactOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: sourceWarehouseVID})))
	if err != nil {
		t.Fatalf("unexpected warehouse interact before post-floor town password open: %v", err)
	}
	if len(interactOut) != 1 {
		t.Fatalf("expected ShowMeSafeboxPassword before post-floor town password open, got %d frames", len(interactOut))
	}
	prompt, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, interactOut[0]))
	if err != nil {
		t.Fatalf("decode ShowMeSafeboxPassword before post-floor town password open: %v", err)
	}
	if prompt.Type != chatproto.ChatTypeCommand || prompt.Message != safeboxShowPasswordCommandMessage {
		t.Fatalf("unexpected password prompt before post-floor town password open: %+v", prompt)
	}

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password " + safeboxstore.DefaultPassword,
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor town /safebox_password dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town /safebox_password to fail closed with no SAFEBOX_SIZE frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town /safebox_password to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town /safebox_password")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after pending-password floor: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after pending-password floor, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after pending-password /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after pending-password floor, got %+v", selfAdd)
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
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after pending-password floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after pending-password floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	staleOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password " + safeboxstore.DefaultPassword,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town stale /safebox_password dispatch error: %v", err)
	}
	if len(staleOut) != 0 {
		t.Fatalf("expected post-restart_town stale /safebox_password to stay fail-closed until fresh warehouse interact, got %d frames", len(staleOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-restart_town stale /safebox_password to queue no frames, got %d", len(queued))
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown + time.Nanosecond)
	rechallengeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: townWarehouseVID})))
	if err != nil {
		t.Fatalf("unexpected town warehouse re-interact after restart_town: %v", err)
	}
	if len(rechallengeOut) != 1 {
		t.Fatalf("expected fresh ShowMeSafeboxPassword after restart_town, got %d frames", len(rechallengeOut))
	}
	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password " + safeboxstore.DefaultPassword,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town fresh /safebox_password open: %v", err)
	}
	if len(openOut) < 2 {
		t.Fatalf("expected SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE after fresh post-restart_town password, got %d", len(openOut))
	}
	size, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, openOut[0]))
	if err != nil {
		t.Fatalf("decode SAFEBOX_SIZE after fresh post-restart_town password: %v", err)
	}
	if size != (itemproto.SafeboxSizePacket{Size: 2}) {
		t.Fatalf("unexpected SAFEBOX_SIZE after fresh post-restart_town password: %+v", size)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town password open: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after pending-password floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after pending-password floor, got %+v", wantHP, account.Characters[0])
	}
}

func TestGameSessionFlowPostFloorSafeboxChangePasswordFailsClosed(t *testing.T) {
	login := "pf-change-safebox-pwd"
	loginKey := uint32(0x19191b16)
	owner := peerVisibilityCharacter("DeadChangeSafeboxPwdOwner", 0x01030b16, 0x02040b16, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 12345
	root := t.TempDir()
	safeboxPath := filepath.Join(root, "state", "safebox.json")
	runtime, accounts, targetVID := newPostFloorSafeboxChangePasswordRuntime(t, root, safeboxPath, login, loginKey, owner)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_change_password " + safeboxstore.DefaultPassword + " vault2",
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor /safebox_change_password dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor /safebox_change_password to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor /safebox_change_password to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor /safebox_change_password")

	snapshot, err := safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load durable safebox after post-floor change-password: %v", err)
	}
	if got := safeboxstore.CharacterPassword(snapshot, login, owner.ID); got != safeboxstore.DefaultPassword {
		t.Fatalf("post-floor /safebox_change_password mutated durable password: got %q want %q", got, safeboxstore.DefaultPassword)
	}

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor change-password: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor change-password, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	changeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_change_password " + safeboxstore.DefaultPassword + " vault2",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart /safebox_change_password: %v", err)
	}
	if len(changeOut) != 1 {
		t.Fatalf("expected one post-restart change-password success chat, got %d", len(changeOut))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, changeOut[0]))
	if err != nil {
		t.Fatalf("decode post-restart change-password success chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != safeboxPasswordChangedInfoMessage {
		t.Fatalf("unexpected post-restart change-password success chat: %+v", delivery)
	}
	snapshot, err = safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load durable safebox after post-restart change-password: %v", err)
	}
	if got := safeboxstore.CharacterPassword(snapshot, login, owner.ID); got != "vault2" {
		t.Fatalf("durable password after post-restart change=%q want vault2", got)
	}
}

func TestGameSessionFlowPostFloorSafeboxChangePasswordFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-change-safebox-pwd-town"
	loginKey := uint32(0x19191b19)
	owner := peerVisibilityCharacter("DeadChangeSafeboxPwdTownOwner", 0x01030b19, 0x02040b19, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 12345
	root := t.TempDir()
	safeboxPath := filepath.Join(root, "state", "safebox.json")
	runtime, accounts, targetVID := newPostFloorSafeboxChangePasswordRuntime(t, root, safeboxPath, login, loginKey, owner)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_change_password " + safeboxstore.DefaultPassword + " vault2",
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor town /safebox_change_password dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town /safebox_change_password to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town /safebox_change_password to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town /safebox_change_password")

	snapshot, err := safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load durable safebox after post-floor town change-password: %v", err)
	}
	if got := safeboxstore.CharacterPassword(snapshot, login, owner.ID); got != safeboxstore.DefaultPassword {
		t.Fatalf("post-floor town /safebox_change_password mutated durable password: got %q want %q", got, safeboxstore.DefaultPassword)
	}

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor change-password: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor change-password, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor change-password /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after change-password floor, got %+v", selfAdd)
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
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after change-password floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after change-password floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	changeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_change_password " + safeboxstore.DefaultPassword + " vault2",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town /safebox_change_password: %v", err)
	}
	if len(changeOut) != 1 {
		t.Fatalf("expected one post-restart_town change-password success chat, got %d", len(changeOut))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, changeOut[0]))
	if err != nil {
		t.Fatalf("decode post-restart_town change-password success chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != safeboxPasswordChangedInfoMessage {
		t.Fatalf("unexpected post-restart_town change-password success chat: %+v", delivery)
	}
	snapshot, err = safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load durable safebox after post-restart_town change-password: %v", err)
	}
	if got := safeboxstore.CharacterPassword(snapshot, login, owner.ID); got != "vault2" {
		t.Fatalf("durable password after post-restart_town change=%q want vault2", got)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town change-password: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after change-password floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after change-password floor, got %+v", wantHP, account.Characters[0])
	}
}

func TestGameSessionFlowPostFloorSafeboxMoneyFailsClosed(t *testing.T) {
	login := "pf-safebox-money"
	loginKey := uint32(0x19191b17)
	owner := peerVisibilityCharacter("DeadSafeboxMoneyOwner", 0x01030b17, 0x02040b17, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	root := t.TempDir()
	safeboxPath := filepath.Join(root, "state", "safebox.json")
	runtime, accounts, targetVID := newPostFloorSafeboxChangePasswordRuntime(t, root, safeboxPath, login, loginKey, owner)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before post-floor money seed: %v", err)
	}
	saveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_money_save 1500",
	})))
	if err != nil {
		t.Fatalf("unexpected pre-floor /safebox_money_save: %v", err)
	}
	if len(saveOut) != 2 {
		t.Fatalf("expected gold PLAYER_POINT_CHANGE + SAFEBOX_MONEY_CHANGE before floor, got %d", len(saveOut))
	}
	preFloorGold, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, saveOut[0]))
	if err != nil {
		t.Fatalf("decode pre-floor gold PLAYER_POINT_CHANGE: %v", err)
	}
	if preFloorGold.VID != owner.VID || preFloorGold.Type != bootstrapGoldPointType || preFloorGold.Amount != -1500 || preFloorGold.Value != 3500 {
		t.Fatalf("unexpected pre-floor gold PLAYER_POINT_CHANGE: %+v", preFloorGold)
	}
	preFloorMoney, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, saveOut[1]))
	if err != nil {
		t.Fatalf("decode pre-floor SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if preFloorMoney != (itemproto.SafeboxMoneyChangePacket{Money: 1500}) {
		t.Fatalf("unexpected pre-floor SAFEBOX_MONEY_CHANGE: %+v", preFloorMoney)
	}
	owner.Gold = 3500
	assertCloseSafeboxCommandChat(t, flow, "/close_safebox", "pre-floor money close")

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor before money deny")
	snapshot, err := safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load durable safebox after floor before money deny: %v", err)
	}
	if got := safeboxstore.CharacterMoney(snapshot, login, owner.ID); got != 1500 {
		t.Fatalf("durable money after floor before money deny=%d want 1500", got)
	}

	for _, message := range []string{"/safebox_money_save 100", "/safebox_money_withdraw 100"} {
		out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
			Type:    chatproto.ChatTypeTalking,
			Message: message,
		})))
		if err != nil {
			t.Fatalf("unexpected post-floor %s dispatch error: %v", message, err)
		}
		if len(out) != 0 {
			t.Fatalf("expected post-floor %s to fail closed with no frames, got %d", message, len(out))
		}
		if queued := flushServerFrames(t, flow); len(queued) != 0 {
			t.Fatalf("expected post-floor %s to queue no frames, got %d", message, len(queued))
		}
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor safebox money")
	snapshot, err = safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load durable safebox after post-floor money deny: %v", err)
	}
	if got := safeboxstore.CharacterMoney(snapshot, login, owner.ID); got != 1500 {
		t.Fatalf("post-floor safebox money mutated durable gold: got %d want 1500", got)
	}

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor safebox money: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor safebox money, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart /open_safebox: %v", err)
	}
	if len(reopenOut) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE after restart, got %d", len(reopenOut))
	}
	reopenMoney, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode post-restart SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if reopenMoney != (itemproto.SafeboxMoneyChangePacket{Money: 1500}) {
		t.Fatalf("unexpected post-restart SAFEBOX_MONEY_CHANGE: %+v", reopenMoney)
	}

	withdrawOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_money_withdraw 500",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart /safebox_money_withdraw: %v", err)
	}
	if len(withdrawOut) != 2 {
		t.Fatalf("expected gold PLAYER_POINT_CHANGE + SAFEBOX_MONEY_CHANGE after restart withdraw, got %d", len(withdrawOut))
	}
	withdrawGold, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, withdrawOut[0]))
	if err != nil {
		t.Fatalf("decode post-restart withdraw gold PLAYER_POINT_CHANGE: %v", err)
	}
	if withdrawGold.VID != owner.VID || withdrawGold.Type != bootstrapGoldPointType || withdrawGold.Amount != 500 || withdrawGold.Value != 4000 {
		t.Fatalf("unexpected post-restart withdraw gold PLAYER_POINT_CHANGE: %+v", withdrawGold)
	}
	withdrawMoney, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, withdrawOut[1]))
	if err != nil {
		t.Fatalf("decode post-restart withdraw SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if withdrawMoney != (itemproto.SafeboxMoneyChangePacket{Money: 1000}) {
		t.Fatalf("unexpected post-restart withdraw SAFEBOX_MONEY_CHANGE: %+v", withdrawMoney)
	}
	snapshot, err = safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load durable safebox after post-restart withdraw: %v", err)
	}
	if got := safeboxstore.CharacterMoney(snapshot, login, owner.ID); got != 1000 {
		t.Fatalf("durable money after post-restart withdraw=%d want 1000", got)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart withdraw: %v", err)
	}
	if account.Characters[0].Gold != 4000 {
		t.Fatalf("carried gold after post-restart withdraw=%d want 4000", account.Characters[0].Gold)
	}
}

func TestGameSessionFlowPostFloorSafeboxMoneyFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-safebox-money-town"
	loginKey := uint32(0x19191b18)
	owner := peerVisibilityCharacter("DeadSafeboxMoneyTownOwner", 0x01030b18, 0x02040b18, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	root := t.TempDir()
	safeboxPath := filepath.Join(root, "state", "safebox.json")
	runtime, accounts, targetVID := newPostFloorSafeboxChangePasswordRuntime(t, root, safeboxPath, login, loginKey, owner)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before post-floor town money seed: %v", err)
	}
	saveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_money_save 1500",
	})))
	if err != nil {
		t.Fatalf("unexpected pre-floor /safebox_money_save before town recovery: %v", err)
	}
	if len(saveOut) != 2 {
		t.Fatalf("expected gold PLAYER_POINT_CHANGE + SAFEBOX_MONEY_CHANGE before town floor, got %d", len(saveOut))
	}
	preFloorGold, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, saveOut[0]))
	if err != nil {
		t.Fatalf("decode pre-floor town gold PLAYER_POINT_CHANGE: %v", err)
	}
	if preFloorGold.VID != owner.VID || preFloorGold.Type != bootstrapGoldPointType || preFloorGold.Amount != -1500 || preFloorGold.Value != 3500 {
		t.Fatalf("unexpected pre-floor town gold PLAYER_POINT_CHANGE: %+v", preFloorGold)
	}
	preFloorMoney, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, saveOut[1]))
	if err != nil {
		t.Fatalf("decode pre-floor town SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if preFloorMoney != (itemproto.SafeboxMoneyChangePacket{Money: 1500}) {
		t.Fatalf("unexpected pre-floor town SAFEBOX_MONEY_CHANGE: %+v", preFloorMoney)
	}
	owner.Gold = 3500
	assertCloseSafeboxCommandChat(t, flow, "/close_safebox", "pre-floor town money close")

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor before town money deny")
	snapshot, err := safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load durable safebox after floor before town money deny: %v", err)
	}
	if got := safeboxstore.CharacterMoney(snapshot, login, owner.ID); got != 1500 {
		t.Fatalf("durable money after floor before town money deny=%d want 1500", got)
	}

	for _, message := range []string{"/safebox_money_save 100", "/safebox_money_withdraw 100"} {
		out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
			Type:    chatproto.ChatTypeTalking,
			Message: message,
		})))
		if err != nil {
			t.Fatalf("unexpected post-floor town %s dispatch error: %v", message, err)
		}
		if len(out) != 0 {
			t.Fatalf("expected post-floor town %s to fail closed with no frames, got %d", message, len(out))
		}
		if queued := flushServerFrames(t, flow); len(queued) != 0 {
			t.Fatalf("expected post-floor town %s to queue no frames, got %d", message, len(queued))
		}
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town safebox money")
	snapshot, err = safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load durable safebox after post-floor town money deny: %v", err)
	}
	if got := safeboxstore.CharacterMoney(snapshot, login, owner.ID); got != 1500 {
		t.Fatalf("post-floor town safebox money mutated durable gold: got %d want 1500", got)
	}

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor safebox money: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor safebox money, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor safebox money /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after safebox money floor, got %+v", selfAdd)
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
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after safebox money floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after safebox money floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town /open_safebox: %v", err)
	}
	if len(reopenOut) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE after restart_town, got %d", len(reopenOut))
	}
	reopenMoney, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode post-restart_town SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if reopenMoney != (itemproto.SafeboxMoneyChangePacket{Money: 1500}) {
		t.Fatalf("unexpected post-restart_town SAFEBOX_MONEY_CHANGE: %+v", reopenMoney)
	}

	withdrawOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_money_withdraw 500",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town /safebox_money_withdraw: %v", err)
	}
	if len(withdrawOut) != 2 {
		t.Fatalf("expected gold PLAYER_POINT_CHANGE + SAFEBOX_MONEY_CHANGE after restart_town withdraw, got %d", len(withdrawOut))
	}
	withdrawGold, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, withdrawOut[0]))
	if err != nil {
		t.Fatalf("decode post-restart_town withdraw gold PLAYER_POINT_CHANGE: %v", err)
	}
	if withdrawGold.VID != owner.VID || withdrawGold.Type != bootstrapGoldPointType || withdrawGold.Amount != 500 || withdrawGold.Value != 4000 {
		t.Fatalf("unexpected post-restart_town withdraw gold PLAYER_POINT_CHANGE: %+v", withdrawGold)
	}
	withdrawMoney, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, withdrawOut[1]))
	if err != nil {
		t.Fatalf("decode post-restart_town withdraw SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if withdrawMoney != (itemproto.SafeboxMoneyChangePacket{Money: 1000}) {
		t.Fatalf("unexpected post-restart_town withdraw SAFEBOX_MONEY_CHANGE: %+v", withdrawMoney)
	}
	snapshot, err = safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load durable safebox after post-restart_town withdraw: %v", err)
	}
	if got := safeboxstore.CharacterMoney(snapshot, login, owner.ID); got != 1000 {
		t.Fatalf("durable money after post-restart_town withdraw=%d want 1000", got)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town withdraw: %v", err)
	}
	if account.Characters[0].Gold != 4000 {
		t.Fatalf("carried gold after post-restart_town withdraw=%d want 4000", account.Characters[0].Gold)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after safebox money floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after safebox money floor, got %+v", wantHP, account.Characters[0])
	}
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

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor /open_cube: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor /open_cube, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart /open_cube: %v", err)
	}
	if len(reopenOut) != 1 {
		t.Fatalf("expected cube open command chat after restart, got %d", len(reopenOut))
	}
	assertCubeCommandChatFrame(t, reopenOut[0], "cube open 20022", "post-restart open-cube")
	assertCloseCubeCommandChat(t, flow, "/close_cube", "post-restart open-cube close")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart /open_cube: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after open-cube floor, got %+v", wantHP, account.Characters[0])
	}
	if account.Characters[0].Gold != owner.Gold {
		t.Fatalf("carried gold after post-restart /open_cube=%d want %d", account.Characters[0].Gold, owner.Gold)
	}
}

func TestGameSessionFlowPostFloorOpenCubeFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-open-cube-town"
	loginKey := uint32(0x19191b21)
	owner := peerVisibilityCharacter("DeadOpenCubeTownOwner", 0x01030b21, 0x02040b21, 1100, 2100, 0, 101, 201)
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
		t.Fatalf("unexpected post-floor town /open_cube dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town /open_cube to fail closed with no cube open command, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town /open_cube to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town /open_cube")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor /open_cube: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor /open_cube, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor /open_cube /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after open-cube floor, got %+v", selfAdd)
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
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after open-cube floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after open-cube floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town /open_cube: %v", err)
	}
	if len(reopenOut) != 1 {
		t.Fatalf("expected cube open command chat after restart_town, got %d", len(reopenOut))
	}
	assertCubeCommandChatFrame(t, reopenOut[0], "cube open 20022", "post-restart_town open-cube")
	assertCloseCubeCommandChat(t, flow, "/close_cube", "post-restart_town open-cube close")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town /open_cube: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after open-cube floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after open-cube floor, got %+v", wantHP, account.Characters[0])
	}
	if account.Characters[0].Gold != owner.Gold {
		t.Fatalf("carried gold after post-restart_town /open_cube=%d want %d", account.Characters[0].Gold, owner.Gold)
	}
}

func TestGameSessionFlowPostFloorMyShopOpenFailsClosed(t *testing.T) {
	login := "post-floor-open-myshop"
	loginKey := uint32(0x19191b30)
	owner := peerVisibilityCharacter("DeadOpenMyShopOwner", 0x01030b30, 0x02040b30, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 931, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 981, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	templates := []itemcatalog.Template{
		{Vnum: 27001, Name: "Dead Guard Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
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

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor MYSHOP open: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor MYSHOP open, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
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
		t.Fatalf("unexpected post-restart MYSHOP open: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, reopenOut, owner.VID, 4, "post-restart MYSHOP open")
	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_myshop",
	})))
	if err != nil {
		t.Fatalf("unexpected /close_myshop after post-restart MYSHOP open: %v", err)
	}
	if len(closeOut) != 1 {
		t.Fatalf("expected /close_myshop after post-restart MYSHOP open to emit one empty SHOP_SIGN, got %d", len(closeOut))
	}
	assertMyShopEmptySignFrame(t, closeOut[0], owner.VID, "post-restart MYSHOP close")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart MYSHOP open: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after MYSHOP open floor, got %+v", wantHP, account.Characters[0])
	}
	want := characterAfterMyShopBagConsume(owner)
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart MYSHOP open after bag consume")
}

func TestGameSessionFlowPostFloorMyShopOpenFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-open-myshop-town"
	loginKey := uint32(0x19191b31)
	owner := peerVisibilityCharacter("DeadOpenMyShopTownOwner", 0x01030b31, 0x02040b31, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 932, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 982, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	templates := []itemcatalog.Template{
		{Vnum: 27001, Name: "Dead Guard Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
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
		t.Fatalf("unexpected post-floor town MYSHOP open dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town MYSHOP open to fail closed with no SHOP_SIGN, got %d frames", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town MYSHOP open to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town MYSHOP open")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor MYSHOP open: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor MYSHOP open, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor MYSHOP open /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after MYSHOP open floor, got %+v", selfAdd)
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
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after MYSHOP open floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after MYSHOP open floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
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
		t.Fatalf("unexpected post-restart_town MYSHOP open: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, reopenOut, owner.VID, 4, "post-restart_town MYSHOP open")
	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_myshop",
	})))
	if err != nil {
		t.Fatalf("unexpected /close_myshop after post-restart_town MYSHOP open: %v", err)
	}
	if len(closeOut) != 1 {
		t.Fatalf("expected /close_myshop after post-restart_town MYSHOP open to emit one empty SHOP_SIGN, got %d", len(closeOut))
	}
	assertMyShopEmptySignFrame(t, closeOut[0], owner.VID, "post-restart_town MYSHOP close")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town MYSHOP open: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after MYSHOP open floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after MYSHOP open floor, got %+v", wantHP, account.Characters[0])
	}
	want := characterAfterMyShopBagConsume(owner)
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town MYSHOP open after bag consume")
}

func TestGameSessionFlowPostFloorShopBagUseFailsClosed(t *testing.T) {
	login := "post-floor-shop-bag-use"
	loginKey := uint32(0x19191b40)
	owner := peerVisibilityCharacter("DeadShopBagUseOwner", 0x01030b40, 0x02040b40, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{
		{ID: 991, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	templates := []itemcatalog.Template{
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(4)})))
	if err != nil {
		t.Fatalf("unexpected post-floor shop bag ITEM_USE dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor shop bag ITEM_USE to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor shop bag ITEM_USE to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor shop bag ITEM_USE")

	slashOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/use_item 4",
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor shop bag /use_item dispatch error: %v", err)
	}
	if len(slashOut) != 0 {
		t.Fatalf("expected post-floor shop bag /use_item to fail closed with no frames, got %d", len(slashOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor shop bag /use_item to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor shop bag /use_item")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor shop bag USE: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor shop bag USE, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(4)})))
	if err != nil {
		t.Fatalf("unexpected post-restart shop bag ITEM_USE: %v", err)
	}
	assertMyShopBagUseCommandBurst(t, reuseOut, []string{myShopOpenPrivateShopCommandMessage}, "post-restart shop bag ITEM_USE")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart shop bag ITEM_USE: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after shop bag USE floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart shop bag ITEM_USE leaves bag unconsumed")
}

func TestGameSessionFlowPostFloorSilkBagUseFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-silk-bag-use-town"
	loginKey := uint32(0x19191b41)
	owner := peerVisibilityCharacter("DeadSilkBagUseTownOwner", 0x01030b41, 0x02040b41, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{
		{ID: 992, Vnum: myShopOpenSilkBagVnum, Count: 1, Slot: 3},
	}
	templates := []itemcatalog.Template{
		{Vnum: myShopOpenSilkBagVnum, Name: "Silk Bag", Stackable: true, MaxCount: 200},
	}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(3)})))
	if err != nil {
		t.Fatalf("unexpected post-floor town silk bag ITEM_USE dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town silk bag ITEM_USE to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town silk bag ITEM_USE to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town silk bag ITEM_USE")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor silk bag USE: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor silk bag USE, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor silk bag USE /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after silk bag USE floor, got %+v", selfAdd)
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
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after silk bag USE floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after silk bag USE floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(3)})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town silk bag ITEM_USE: %v", err)
	}
	assertMyShopBagUseCommandBurst(t, reuseOut, []string{"MyShopPriceList 1 0", myShopOpenPrivateShopCommandMessage}, "post-restart_town silk bag ITEM_USE")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town silk bag ITEM_USE: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after silk bag USE floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after silk bag USE floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town silk bag ITEM_USE leaves bag unconsumed")
}

func TestGameSessionFlowPostFloorRefinePreviewOpenFailsClosed(t *testing.T) {
	login := "post-floor-open-refine"
	loginKey := uint32(0x19191b35)
	owner := peerVisibilityCharacter("DeadOpenRefineOwner", 0x01030b35, 0x02040b35, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 935, Vnum: 11250, Count: 1, Slot: 5}}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11250,
		Name:       "Dead Guard Refine Preview Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{
			ResultVnum:  11251,
			Cost:        1000,
			Probability: 100,
			Materials:   []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 1}},
		},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11251, Name: "Dead Guard Refine Result Blade", Stackable: false, MaxCount: 1}
	materialTemplate := itemcatalog.Template{Vnum: 27001, Name: "Dead Guard Refine Material", Stackable: true, MaxCount: 200}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, []itemcatalog.Template{sourceTemplate, resultTemplate, materialTemplate})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected post-floor refineable REFINE preview dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor refineable REFINE preview to fail closed with no REFINE_INFORMATION_NEW, got %d frames", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor refineable REFINE preview to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor refineable REFINE preview")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor refine preview: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor refine preview, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected post-restart refineable REFINE preview: %v", err)
	}
	if len(reopenOut) != 1 {
		t.Fatalf("expected REFINE_INFORMATION_NEW after restart, got %d", len(reopenOut))
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, reopenOut[0])); err != nil {
		t.Fatalf("decode post-restart REFINE_INFORMATION_NEW: %v", err)
	}
	cancelOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 255})))
	if err != nil {
		t.Fatalf("unexpected post-restart refine cancel: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected silent refine cancel after post-restart preview, got %d", len(cancelOut))
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart refine preview: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after refine preview floor, got %+v", wantHP, account.Characters[0])
	}
	if account.Characters[0].Gold != owner.Gold {
		t.Fatalf("carried gold after post-restart refine preview=%d want %d", account.Characters[0].Gold, owner.Gold)
	}
}

func TestGameSessionFlowPostFloorRefinePreviewOpenFailsClosedBeforeRestartTown(t *testing.T) {
	login := "pf-open-refine-town"
	loginKey := uint32(0x19191b36)
	owner := peerVisibilityCharacter("DeadOpenRefineTownOwner", 0x01030b36, 0x02040b36, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 936, Vnum: 11250, Count: 1, Slot: 5}}
	sourceTemplate := itemcatalog.Template{
		Vnum:       11250,
		Name:       "Dead Guard Refine Preview Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{
			ResultVnum:  11251,
			Cost:        1000,
			Probability: 100,
			Materials:   []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 1}},
		},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11251, Name: "Dead Guard Refine Result Blade", Stackable: false, MaxCount: 1}
	materialTemplate := itemcatalog.Template{Vnum: 27001, Name: "Dead Guard Refine Material", Stackable: true, MaxCount: 200}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, []itemcatalog.Template{sourceTemplate, resultTemplate, materialTemplate})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected post-floor town refineable REFINE preview dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town refineable REFINE preview to fail closed with no REFINE_INFORMATION_NEW, got %d frames", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town refineable REFINE preview to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town refineable REFINE preview")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor refine preview: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor refine preview, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor refine preview /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after refine preview floor, got %+v", selfAdd)
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
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after refine preview floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after refine preview floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town refineable REFINE preview: %v", err)
	}
	if len(reopenOut) != 1 {
		t.Fatalf("expected REFINE_INFORMATION_NEW after restart_town, got %d", len(reopenOut))
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, reopenOut[0])); err != nil {
		t.Fatalf("decode post-restart_town REFINE_INFORMATION_NEW: %v", err)
	}
	cancelOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 255})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town refine cancel: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected silent refine cancel after post-restart_town preview, got %d", len(cancelOut))
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town refine preview: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after refine preview floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after refine preview floor, got %+v", wantHP, account.Characters[0])
	}
	if account.Characters[0].Gold != owner.Gold {
		t.Fatalf("carried gold after post-restart_town refine preview=%d want %d", account.Characters[0].Gold, owner.Gold)
	}
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

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor EXCHANGE START: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor EXCHANGE START, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, peerFlow)

	freshStart, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_here EXCHANGE START: %v", err)
	}
	if len(freshStart) != 1 {
		t.Fatalf("expected post-restart_here EXCHANGE START to succeed, got %d frames", len(freshStart))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, freshStart[0])); err == nil {
		t.Fatalf("expected post-restart_here EXCHANGE START success, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, freshStart[0], peer.VID, "post-restart_here EXCHANGE START after post-floor open deny")
	peerFreshStart := flushServerFrames(t, peerFlow)
	if len(peerFreshStart) != 1 {
		t.Fatalf("expected peer EXCHANGE START after post-restart_here recovery, got %d", len(peerFreshStart))
	}
	assertExchangeStartFrame(t, peerFreshStart[0], owner.VID, "peer EXCHANGE START after post-restart_here recovery")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_here EXCHANGE START: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after EXCHANGE START floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_here EXCHANGE START leaves inventory/gold unchanged")
}

func TestGameSessionFlowPostFloorExchangeStartFailsClosedBeforeRestartTown(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DeadOpenExchangeTownOwner", 0x01030b60, 0x02040b60, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	sourcePeer := peerVisibilityCharacter("DeadOpenExchangeTownSource", 0x01030b61, 0x02040b61, 1120, 2120, 0, 101, 201)
	townPeer := peerVisibilityCharacter("DeadOpenExchangeTownPeer", 0x01030b62, 0x02040b62, 52070, 166600, 4, 103, 203)
	townPeer.MapIndex = 21
	login := "pf-open-exchange-town"
	loginKey := uint32(0x19191b60)
	sourceLogin := "pf-open-exchange-town-s"
	sourceLoginKey := uint32(0x19191b61)
	townLogin := "pf-open-exchange-town-t"
	townLoginKey := uint32(0x19191b62)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, sourceLogin, sourceLoginKey, sourcePeer)
	issuePeerTicket(t, ticketStore, townLogin, townLoginKey, townPeer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor town exchange-open owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: sourceLogin, Empire: sourcePeer.Empire, Characters: cloneCharacters([]loginticket.Character{sourcePeer})}); err != nil {
		t.Fatalf("seed post-floor town exchange-open source peer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: townLogin, Empire: townPeer.Empire, Characters: cloneCharacters([]loginticket.Character{townPeer})}); err != nil {
		t.Fatalf("seed post-floor town exchange-open town peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, newItemTemplateStore(t, nil), nil)
	if err != nil {
		t.Fatalf("unexpected post-floor town exchange-open runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_exchange_open_town",
		Name:          "PracticeMobPostFloorExchangeOpenTown",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import post-floor town exchange-open practice mob: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor town exchange-open practice mob, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) < 8 {
		t.Fatalf("expected owner bootstrap with visible practice mob, got %d frames", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	sourceFlow, sourceEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), sourceLogin, sourceLoginKey)
	if len(sourceEnter) < 11 {
		t.Fatalf("expected source peer bootstrap with visible owner and mob, got %d frames", len(sourceEnter))
	}
	defer closeSessionFlow(t, sourceFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive source peer-entry frames before post-floor town exchange open, got %d", len(queued))
	}
	_ = flushServerFrames(t, sourceFlow)
	townFlow, townEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), townLogin, townLoginKey)
	if len(townEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for town peer on destination map, got %d", len(townEnter))
	}
	defer closeSessionFlow(t, townFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected destination-map town peer join to avoid queued owner frames before floor, got %d", len(queued))
	}
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 0 {
		t.Fatalf("expected destination-map town peer join to avoid queued source frames before floor, got %d", len(queued))
	}

	drivePracticeMobOwnerToBootstrapHPFloor(t, ownerFlow, owner, targetVID)
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 1 {
		t.Fatalf("expected source peer DEAD fanout after owner floor before town exchange open, got %d", len(queued))
	}

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      sourcePeer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor town EXCHANGE START dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor town EXCHANGE START to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected post-floor town EXCHANGE START to queue no owner frames, got %d", len(queued))
	}
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 0 {
		t.Fatalf("expected post-floor town EXCHANGE START to queue no source frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town EXCHANGE START")

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor EXCHANGE START: %v", err)
	}
	if len(restartOut) < 9 {
		t.Fatalf("expected at least 9 self frames from /restart_town recovery after post-floor EXCHANGE START, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor EXCHANGE START /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after EXCHANGE START floor, got %+v", selfAdd)
	}
	var (
		selfPoints    worldproto.PlayerPointChangePacket
		foundPoints   bool
		foundTownPeer bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
				continue
			}
		}
		if add, err := worldproto.DecodeCharacterAdd(fr); err == nil && add.VID == townPeer.VID {
			foundTownPeer = true
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after EXCHANGE START floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after EXCHANGE START floor, got %+v", wantHP, selfPoints)
	}
	if !foundTownPeer {
		t.Fatalf("expected /restart_town destination visibility delta to add town peer vid %d", townPeer.VID)
	}
	_ = flushServerFrames(t, sourceFlow)
	townQueued := flushServerFrames(t, townFlow)
	if len(townQueued) != 3 {
		t.Fatalf("expected destination town peer to receive 3 queued owner re-entry frames after /restart_town, got %d", len(townQueued))
	}

	freshStart, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      townPeer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town EXCHANGE START: %v", err)
	}
	if len(freshStart) != 1 {
		t.Fatalf("expected post-restart_town EXCHANGE START to succeed, got %d frames", len(freshStart))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, freshStart[0])); err == nil {
		t.Fatalf("expected post-restart_town EXCHANGE START success, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, freshStart[0], townPeer.VID, "post-restart_town EXCHANGE START after post-floor open deny")
	townStart := flushServerFrames(t, townFlow)
	if len(townStart) != 1 {
		t.Fatalf("expected town peer EXCHANGE START after post-restart_town recovery, got %d", len(townStart))
	}
	assertExchangeStartFrame(t, townStart[0], owner.VID, "town peer EXCHANGE START after post-restart_town recovery")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town EXCHANGE START: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after EXCHANGE START floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after EXCHANGE START floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town EXCHANGE START leaves inventory/gold unchanged")
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

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after living-peer EXCHANGE START against dead partner: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after living-peer EXCHANGE START against dead partner, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, peerFlow)

	freshStart, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      owner.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_here living-peer EXCHANGE START against recovered partner: %v", err)
	}
	if len(freshStart) != 1 {
		t.Fatalf("expected post-restart_here living-peer EXCHANGE START against recovered partner to succeed, got %d frames", len(freshStart))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, freshStart[0])); err == nil {
		t.Fatalf("expected post-restart_here living-peer EXCHANGE START success, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, freshStart[0], owner.VID, "post-restart_here living-peer EXCHANGE START against recovered partner")
	ownerFreshStart := flushServerFrames(t, ownerFlow)
	if len(ownerFreshStart) != 1 {
		t.Fatalf("expected recovered owner EXCHANGE START after post-restart_here living-peer recovery, got %d", len(ownerFreshStart))
	}
	assertExchangeStartFrame(t, ownerFreshStart[0], peer.VID, "recovered owner EXCHANGE START after post-restart_here living-peer recovery")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_here living-peer EXCHANGE START against recovered partner: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after dead-partner EXCHANGE START floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_here living-peer EXCHANGE START leaves inventory/gold unchanged")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "post-restart_here living-peer EXCHANGE START leaves peer inventory/gold unchanged")
}

func TestGameSessionFlowPostFloorExchangeStartAgainstDeadPartnerFailsClosedBeforeRestartTown(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DeadPartnerExchangeTownOwner", 0x01030b63, 0x02040b63, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	sourcePeer := peerVisibilityCharacter("DeadPartnerExchangeTownSource", 0x01030b64, 0x02040b64, 1120, 2120, 0, 101, 201)
	townPeer := peerVisibilityCharacter("DeadPartnerExchangeTownPeer", 0x01030b65, 0x02040b65, 52070, 166600, 4, 103, 203)
	townPeer.MapIndex = 21
	login := "pf-dead-partner-ex-town"
	loginKey := uint32(0x19191b63)
	sourceLogin := "pf-dead-partner-ex-town-s"
	sourceLoginKey := uint32(0x19191b64)
	townLogin := "pf-dead-partner-ex-town-t"
	townLoginKey := uint32(0x19191b65)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, sourceLogin, sourceLoginKey, sourcePeer)
	issuePeerTicket(t, ticketStore, townLogin, townLoginKey, townPeer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor dead-partner town exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: sourceLogin, Empire: sourcePeer.Empire, Characters: cloneCharacters([]loginticket.Character{sourcePeer})}); err != nil {
		t.Fatalf("seed post-floor dead-partner town exchange source peer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: townLogin, Empire: townPeer.Empire, Characters: cloneCharacters([]loginticket.Character{townPeer})}); err != nil {
		t.Fatalf("seed post-floor dead-partner town exchange town peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, newItemTemplateStore(t, nil), nil)
	if err != nil {
		t.Fatalf("unexpected post-floor dead-partner town exchange runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_dead_partner_exchange_town",
		Name:          "PracticeMobPostFloorDeadPartnerExchangeTown",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import post-floor dead-partner town exchange practice mob: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor dead-partner town exchange practice mob, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) < 8 {
		t.Fatalf("expected owner bootstrap with visible practice mob, got %d frames", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	sourceFlow, sourceEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), sourceLogin, sourceLoginKey)
	if len(sourceEnter) < 11 {
		t.Fatalf("expected source peer bootstrap with visible owner and mob, got %d frames", len(sourceEnter))
	}
	defer closeSessionFlow(t, sourceFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive source peer-entry frames before dead-partner town exchange, got %d", len(queued))
	}
	_ = flushServerFrames(t, sourceFlow)
	townFlow, townEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), townLogin, townLoginKey)
	if len(townEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for town peer on destination map, got %d", len(townEnter))
	}
	defer closeSessionFlow(t, townFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected destination-map town peer join to avoid queued owner frames before floor, got %d", len(queued))
	}
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 0 {
		t.Fatalf("expected destination-map town peer join to avoid queued source frames before floor, got %d", len(queued))
	}

	drivePracticeMobOwnerToBootstrapHPFloor(t, ownerFlow, owner, targetVID)
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 1 {
		t.Fatalf("expected source peer DEAD fanout after owner floor before dead-partner town exchange, got %d", len(queued))
	}

	out, err := sourceFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      owner.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected living-peer EXCHANGE START against dead partner dispatch error before restart_town: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected living-peer EXCHANGE START against dead partner to fail closed with no frames before restart_town, got %d", len(out))
	}
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 0 {
		t.Fatalf("expected living-peer EXCHANGE START against dead partner to queue no source frames before restart_town, got %d", len(queued))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected living-peer EXCHANGE START against dead partner to queue no dead-owner frames before restart_town, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "living-peer EXCHANGE START against dead partner before restart_town")

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after living-peer EXCHANGE START against dead partner: %v", err)
	}
	if len(restartOut) < 9 {
		t.Fatalf("expected at least 9 self frames from /restart_town recovery after living-peer EXCHANGE START against dead partner, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after dead-partner EXCHANGE START /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after dead-partner EXCHANGE START floor, got %+v", selfAdd)
	}
	var (
		selfPoints    worldproto.PlayerPointChangePacket
		foundPoints   bool
		foundTownPeer bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
				continue
			}
		}
		if add, err := worldproto.DecodeCharacterAdd(fr); err == nil && add.VID == townPeer.VID {
			foundTownPeer = true
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after dead-partner EXCHANGE START floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after dead-partner EXCHANGE START floor, got %+v", wantHP, selfPoints)
	}
	if !foundTownPeer {
		t.Fatalf("expected /restart_town destination visibility delta to add town peer vid %d", townPeer.VID)
	}
	_ = flushServerFrames(t, sourceFlow)
	townQueued := flushServerFrames(t, townFlow)
	if len(townQueued) != 3 {
		t.Fatalf("expected destination town peer to receive 3 queued owner re-entry frames after /restart_town, got %d", len(townQueued))
	}

	freshStart, err := townFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      owner.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town living-peer EXCHANGE START against recovered partner: %v", err)
	}
	if len(freshStart) != 1 {
		t.Fatalf("expected post-restart_town living-peer EXCHANGE START against recovered partner to succeed, got %d frames", len(freshStart))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, freshStart[0])); err == nil {
		t.Fatalf("expected post-restart_town living-peer EXCHANGE START success, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, freshStart[0], owner.VID, "post-restart_town living-peer EXCHANGE START against recovered partner")
	ownerFreshStart := flushServerFrames(t, ownerFlow)
	if len(ownerFreshStart) != 1 {
		t.Fatalf("expected recovered owner EXCHANGE START after post-restart_town living-peer recovery, got %d", len(ownerFreshStart))
	}
	assertExchangeStartFrame(t, ownerFreshStart[0], townPeer.VID, "recovered owner EXCHANGE START after post-restart_town living-peer recovery")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town living-peer EXCHANGE START against recovered partner: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after dead-partner EXCHANGE START floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after dead-partner EXCHANGE START floor, got %+v", wantHP, account.Characters[0])
	}
	want := owner
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town living-peer EXCHANGE START leaves inventory/gold unchanged")
	assertExchangeAccountUnchanged(t, accounts, townLogin, townPeer, "post-restart_town living-peer EXCHANGE START leaves town peer inventory/gold unchanged")
}

func newPostFloorSafeboxPasswordRuntime(t *testing.T, login string, loginKey uint32, owner loginticket.Character) (*gameRuntime, accountstore.Store, uint32, uint32, uint32) {
	t.Helper()
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor safebox-password account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, nil)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected post-floor safebox-password runtime error: %v", err)
	}

	bundle := contentbundle.Bundle{
		InteractionDefinitions: []interactionstore.Definition{{
			Kind: interactionstore.KindOpenSafebox,
			Ref:  "npc:warehouse",
			Size: 2,
		}},
		StaticActors: []contentbundle.StaticActor{{
			Name:            "Warehouse",
			MapIndex:        bootstrapMapIndex,
			X:               1150,
			Y:               2150,
			RaceNum:         20300,
			InteractionKind: interactionstore.KindOpenSafebox,
			InteractionRef:  "npc:warehouse",
		}, {
			Name:            "TownWarehouse",
			MapIndex:        21,
			X:               52100,
			Y:               166650,
			RaceNum:         20300,
			InteractionKind: interactionstore.KindOpenSafebox,
			InteractionRef:  "npc:warehouse",
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.mob_post_floor_safebox_password",
			Name:          "PracticeMobPostFloorSafeboxPassword",
			MapIndex:      bootstrapMapIndex,
			X:             1200,
			Y:             2200,
			RaceNum:       101,
			CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
		}},
	}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import post-floor safebox-password content bundle: %v", err)
	}
	actors := runtime.StaticActors()
	var targetVID, sourceWarehouseVID, townWarehouseVID uint32
	for _, actor := range actors {
		switch actor.Name {
		case "PracticeMobPostFloorSafeboxPassword":
			targetVID = uint32(actor.EntityID)
		case "Warehouse":
			sourceWarehouseVID = uint32(actor.EntityID)
		case "TownWarehouse":
			townWarehouseVID = uint32(actor.EntityID)
		}
	}
	if targetVID == 0 || sourceWarehouseVID == 0 || townWarehouseVID == 0 {
		t.Fatalf("expected practice mob, source warehouse, and town warehouse after post-floor safebox-password import, got %#v", actors)
	}
	return runtime, accounts, targetVID, sourceWarehouseVID, townWarehouseVID
}

func newPostFloorSafeboxChangePasswordRuntime(t *testing.T, root, safeboxPath, login string, loginKey uint32, owner loginticket.Character) (*gameRuntime, accountstore.Store, uint32) {
	t.Helper()
	ticketStore := loginticket.NewFileStore(filepath.Join(root, "tickets"))
	accounts := accountstore.NewFileStore(filepath.Join(root, "accounts"))
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor safebox-change-password account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(filepath.Join(root, "static-actors.json"))
	interactionStore := interactionstore.NewFileStore(filepath.Join(root, "interaction-definitions.json"))
	itemStore := newItemTemplateStore(t, nil)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{
		LegacyAddr:       ":13000",
		PublicAddr:       "127.0.0.1",
		SafeboxStorePath: safeboxPath,
	}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected post-floor safebox-change-password runtime error: %v", err)
	}

	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_safebox_change_password",
		Name:          "PracticeMobPostFloorSafeboxChangePassword",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import post-floor safebox-change-password practice mob: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor safebox-change-password practice mob, got %#v", actors)
	}
	return runtime, accounts, uint32(actors[0].EntityID)
}

func TestGameSessionFlowPostFloorMyShopGuestOnClickFailsClosed(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	// Keep the host outside DefaultSpawnAggroRadius of the practice mob so proximity
	// engagement does not silently block the dead guest's TARGET / ATTACK path.
	host := peerVisibilityCharacter("DeadGuestOnClickHost", 0x01030b70, 0x02040b70, 900, 1900, 0, 101, 201)
	host.Gold = 5000
	host.Inventory = []inventory.ItemInstance{
		{ID: 970, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 971, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	guest := peerVisibilityCharacter("DeadGuestOnClickGuest", 0x01030b71, 0x02040b71, 1120, 2120, 0, 101, 201)
	guest.Points[bootstrapPlayerPointValueIndex] = 1
	guest.Gold = 22222
	hostLogin := "pf-guest-onclick-h"
	hostLoginKey := uint32(0x19191b70)
	guestLogin := "pf-guest-onclick-g"
	guestLoginKey := uint32(0x19191b71)
	issuePeerTicket(t, ticketStore, hostLogin, hostLoginKey, host)
	issuePeerTicket(t, ticketStore, guestLogin, guestLoginKey, guest)
	if err := accounts.Save(accountstore.Account{Login: hostLogin, Empire: host.Empire, Characters: cloneCharacters([]loginticket.Character{host})}); err != nil {
		t.Fatalf("seed post-floor guest ON_CLICK host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: guestLogin, Empire: guest.Empire, Characters: cloneCharacters([]loginticket.Character{guest})}); err != nil {
		t.Fatalf("seed post-floor guest ON_CLICK guest account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected post-floor guest ON_CLICK runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_guest_onclick",
		Name:          "PracticeMobPostFloorGuestOnClick",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import post-floor guest ON_CLICK practice mob: %v", err)
	}
	if err := runtime.replaceItemTemplates(itemcatalog.Snapshot{Templates: []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	}}); err != nil {
		t.Fatalf("restore post-floor guest ON_CLICK templates after content import: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor guest ON_CLICK practice mob, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	hostFlow, hostEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), hostLogin, hostLoginKey)
	if len(hostEnter) < 8 {
		t.Fatalf("expected host bootstrap with visible practice mob, got %d frames", len(hostEnter))
	}
	defer closeSessionFlow(t, hostFlow)
	guestFlow, guestEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), guestLogin, guestLoginKey)
	if len(guestEnter) < 11 {
		t.Fatalf("expected guest bootstrap with visible host and mob, got %d frames", len(guestEnter))
	}
	defer closeSessionFlow(t, guestFlow)
	if queued := flushServerFrames(t, hostFlow); len(queued) != 3 {
		t.Fatalf("expected host to receive guest-entry frames before post-floor guest ON_CLICK, got %d", len(queued))
	}
	_ = flushServerFrames(t, guestFlow)

	openOut, err := hostFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
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
		t.Fatalf("unexpected MYSHOP open before post-floor guest ON_CLICK: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, host.VID, 4, "accepted MYSHOP before post-floor guest ON_CLICK")
	if queued := flushServerFrames(t, guestFlow); len(queued) != 1 {
		t.Fatalf("expected guest to receive one live SHOP_SIGN around-broadcast before floor, got %d", len(queued))
	}

	drivePracticeMobOwnerToBootstrapHPFloor(t, guestFlow, guest, targetVID)
	if queued := flushServerFrames(t, hostFlow); len(queued) != 1 {
		t.Fatalf("expected host DEAD fanout after guest floor before ON_CLICK deny, got %d", len(queued))
	}

	denyOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: host.VID})))
	if err != nil {
		t.Fatalf("unexpected post-floor guest ON_CLICK dispatch error: %v", err)
	}
	if len(denyOut) != 0 {
		t.Fatalf("expected post-floor guest ON_CLICK to fail closed with no SHOP START, got %d frames", len(denyOut))
	}
	if queued := flushServerFrames(t, guestFlow); len(queued) != 0 {
		t.Fatalf("expected post-floor guest ON_CLICK to queue no guest frames, got %d", len(queued))
	}
	if queued := flushServerFrames(t, hostFlow); len(queued) != 0 {
		t.Fatalf("expected post-floor guest ON_CLICK to queue no host frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, guestLogin, guest, "post-floor guest ON_CLICK")
	assertExchangeAccountUnchanged(t, accounts, hostLogin, characterAfterMyShopBagConsume(host), "post-floor guest ON_CLICK host")

	restartOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor guest ON_CLICK: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor guest ON_CLICK, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, hostFlow)

	browseOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: host.VID})))
	if err != nil {
		t.Fatalf("unexpected post-restart_here guest ON_CLICK: %v", err)
	}
	if len(browseOut) != 1 {
		t.Fatalf("expected post-restart_here guest ON_CLICK to emit one SHOP START, got %d frames", len(browseOut))
	}
	start, err := shopproto.DecodeServerStart(decodeSingleFrame(t, browseOut[0]))
	if err != nil {
		t.Fatalf("decode post-restart_here guest SHOP START: %v", err)
	}
	if start.OwnerVID != host.VID {
		t.Fatalf("unexpected post-restart_here guest browse OwnerVID: got %#08x want %#08x", start.OwnerVID, host.VID)
	}
	if queued := flushServerFrames(t, hostFlow); len(queued) != 0 {
		t.Fatalf("expected post-restart_here guest browse to queue no host frames, got %d", len(queued))
	}
	account, err := accounts.Load(guestLogin)
	if err != nil {
		t.Fatalf("load guest account after post-restart_here ON_CLICK: %v", err)
	}
	wantHP := initialStatsForRace(guest.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered guest HP %d after ON_CLICK floor, got %+v", wantHP, account.Characters[0])
	}
	want := guest
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	assertExchangeAccountUnchanged(t, accounts, guestLogin, want, "post-restart_here guest ON_CLICK leaves inventory/gold unchanged")
	assertExchangeAccountUnchanged(t, accounts, hostLogin, characterAfterMyShopBagConsume(host), "post-restart_here guest ON_CLICK host unchanged")
}

func TestGameSessionFlowPostFloorMyShopGuestOnClickFailsClosedBeforeRestartTown(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	sourceHost := peerVisibilityCharacter("DeadGuestOnClickTownSource", 0x01030b72, 0x02040b72, 900, 1900, 0, 101, 201)
	sourceHost.Gold = 5000
	sourceHost.Inventory = []inventory.ItemInstance{
		{ID: 972, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 973, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	guest := peerVisibilityCharacter("DeadGuestOnClickTownGuest", 0x01030b73, 0x02040b73, 1120, 2120, 0, 101, 201)
	guest.Points[bootstrapPlayerPointValueIndex] = 1
	guest.Gold = 22222
	townHost := peerVisibilityCharacter("DeadGuestOnClickTownHost", 0x01030b74, 0x02040b74, 52070, 166600, 4, 103, 203)
	townHost.MapIndex = 21
	townHost.Gold = 5000
	townHost.Inventory = []inventory.ItemInstance{
		{ID: 974, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 975, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	sourceLogin := "pf-guest-onclick-town-s"
	sourceLoginKey := uint32(0x19191b72)
	guestLogin := "pf-guest-onclick-town-g"
	guestLoginKey := uint32(0x19191b73)
	townLogin := "pf-guest-onclick-town-t"
	townLoginKey := uint32(0x19191b74)
	issuePeerTicket(t, ticketStore, sourceLogin, sourceLoginKey, sourceHost)
	issuePeerTicket(t, ticketStore, guestLogin, guestLoginKey, guest)
	issuePeerTicket(t, ticketStore, townLogin, townLoginKey, townHost)
	if err := accounts.Save(accountstore.Account{Login: sourceLogin, Empire: sourceHost.Empire, Characters: cloneCharacters([]loginticket.Character{sourceHost})}); err != nil {
		t.Fatalf("seed post-floor town guest ON_CLICK source host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: guestLogin, Empire: guest.Empire, Characters: cloneCharacters([]loginticket.Character{guest})}); err != nil {
		t.Fatalf("seed post-floor town guest ON_CLICK guest account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: townLogin, Empire: townHost.Empire, Characters: cloneCharacters([]loginticket.Character{townHost})}); err != nil {
		t.Fatalf("seed post-floor town guest ON_CLICK town host account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected post-floor town guest ON_CLICK runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_guest_onclick_town",
		Name:          "PracticeMobPostFloorGuestOnClickTown",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import post-floor town guest ON_CLICK practice mob: %v", err)
	}
	if err := runtime.replaceItemTemplates(itemcatalog.Snapshot{Templates: []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	}}); err != nil {
		t.Fatalf("restore post-floor town guest ON_CLICK templates after content import: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor town guest ON_CLICK practice mob, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	sourceFlow, sourceEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), sourceLogin, sourceLoginKey)
	if len(sourceEnter) < 8 {
		t.Fatalf("expected source host bootstrap with visible practice mob, got %d frames", len(sourceEnter))
	}
	defer closeSessionFlow(t, sourceFlow)
	guestFlow, guestEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), guestLogin, guestLoginKey)
	if len(guestEnter) < 11 {
		t.Fatalf("expected guest bootstrap with visible source host and mob, got %d frames", len(guestEnter))
	}
	defer closeSessionFlow(t, guestFlow)
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 3 {
		t.Fatalf("expected source host to receive guest-entry frames before town guest ON_CLICK, got %d", len(queued))
	}
	_ = flushServerFrames(t, guestFlow)
	townFlow, townEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), townLogin, townLoginKey)
	// Town-host bootstrap includes carried inventory ITEM_SET frames, so it is longer
	// than the empty destination-peer burst used by EXCHANGE START twins.
	if len(townEnter) < 5 {
		t.Fatalf("expected at least 5 bootstrap frames for town host on destination map, got %d", len(townEnter))
	}
	defer closeSessionFlow(t, townFlow)
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 0 {
		t.Fatalf("expected destination-map town host join to avoid queued source frames before floor, got %d", len(queued))
	}
	if queued := flushServerFrames(t, guestFlow); len(queued) != 0 {
		t.Fatalf("expected destination-map town host join to avoid queued guest frames before floor, got %d", len(queued))
	}

	sourceOpen, err := sourceFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
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
		t.Fatalf("unexpected source MYSHOP open before town guest ON_CLICK: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, sourceOpen, sourceHost.VID, 4, "accepted source MYSHOP before town guest ON_CLICK")
	if queued := flushServerFrames(t, guestFlow); len(queued) != 1 {
		t.Fatalf("expected guest to receive one live SHOP_SIGN around-broadcast before town floor, got %d", len(queued))
	}
	townOpen, err := townFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
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
		t.Fatalf("unexpected town MYSHOP open before guest ON_CLICK: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, townOpen, townHost.VID, 4, "accepted town MYSHOP before guest ON_CLICK")
	if queued := flushServerFrames(t, guestFlow); len(queued) != 0 {
		t.Fatalf("expected destination-map town MYSHOP open to avoid queued guest frames before floor, got %d", len(queued))
	}

	drivePracticeMobOwnerToBootstrapHPFloor(t, guestFlow, guest, targetVID)
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 1 {
		t.Fatalf("expected source host DEAD fanout after guest floor before town ON_CLICK deny, got %d", len(queued))
	}

	denyOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: sourceHost.VID})))
	if err != nil {
		t.Fatalf("unexpected post-floor town guest ON_CLICK dispatch error: %v", err)
	}
	if len(denyOut) != 0 {
		t.Fatalf("expected post-floor town guest ON_CLICK to fail closed with no SHOP START, got %d frames", len(denyOut))
	}
	if queued := flushServerFrames(t, guestFlow); len(queued) != 0 {
		t.Fatalf("expected post-floor town guest ON_CLICK to queue no guest frames, got %d", len(queued))
	}
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 0 {
		t.Fatalf("expected post-floor town guest ON_CLICK to queue no source frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, guestLogin, guest, "post-floor town guest ON_CLICK")

	restartOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor guest ON_CLICK: %v", err)
	}
	if len(restartOut) < 9 {
		t.Fatalf("expected at least 9 self frames from /restart_town recovery after post-floor guest ON_CLICK, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor guest ON_CLICK /restart_town: %v", err)
	}
	if selfAdd.VID != guest.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after guest ON_CLICK floor, got %+v", selfAdd)
	}
	var (
		selfPoints    worldproto.PlayerPointChangePacket
		foundPoints   bool
		foundTownHost bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
				continue
			}
		}
		if add, err := worldproto.DecodeCharacterAdd(fr); err == nil && add.VID == townHost.VID {
			foundTownHost = true
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after guest ON_CLICK floor")
	}
	wantHP := initialStatsForRace(guest.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered guest HP %d after ON_CLICK floor, got %+v", wantHP, selfPoints)
	}
	if !foundTownHost {
		t.Fatalf("expected /restart_town destination visibility delta to add town host vid %d", townHost.VID)
	}
	_ = flushServerFrames(t, sourceFlow)
	townQueued := flushServerFrames(t, townFlow)
	if len(townQueued) != 3 {
		t.Fatalf("expected destination town host to receive 3 queued guest re-entry frames after /restart_town, got %d", len(townQueued))
	}

	browseOut, err := guestFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: townHost.VID})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town guest ON_CLICK: %v", err)
	}
	if len(browseOut) != 1 {
		t.Fatalf("expected post-restart_town guest ON_CLICK to emit one SHOP START, got %d frames", len(browseOut))
	}
	start, err := shopproto.DecodeServerStart(decodeSingleFrame(t, browseOut[0]))
	if err != nil {
		t.Fatalf("decode post-restart_town guest SHOP START: %v", err)
	}
	if start.OwnerVID != townHost.VID {
		t.Fatalf("unexpected post-restart_town guest browse OwnerVID: got %#08x want %#08x", start.OwnerVID, townHost.VID)
	}
	if queued := flushServerFrames(t, townFlow); len(queued) != 0 {
		t.Fatalf("expected post-restart_town guest browse to queue no town host frames, got %d", len(queued))
	}
	account, err := accounts.Load(guestLogin)
	if err != nil {
		t.Fatalf("load guest account after post-restart_town ON_CLICK: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after guest ON_CLICK floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered guest HP %d after ON_CLICK floor, got %+v", wantHP, account.Characters[0])
	}
	want := guest
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	assertExchangeAccountUnchanged(t, accounts, guestLogin, want, "post-restart_town guest ON_CLICK leaves inventory/gold unchanged")
	assertExchangeAccountUnchanged(t, accounts, townLogin, characterAfterMyShopBagConsume(townHost), "post-restart_town guest ON_CLICK town host unchanged")
}
