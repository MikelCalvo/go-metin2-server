package boot

import (
	"bytes"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/player"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
	worldentry "github.com/MikelCalvo/go-metin2-server/internal/worldentry"
)

func expectBootHandshakeStart(t *testing.T, client *bootTestClient, challenge control.KeyChallengePacket) {
	t.Helper()

	phaseHandshake := client.readFrame(t)
	wantPhaseHandshake, err := control.EncodePhase(session.PhaseHandshake)
	if err != nil {
		t.Fatalf("unexpected handshake phase encode error: %v", err)
	}
	if !bytes.Equal(phaseHandshake.Raw, wantPhaseHandshake) {
		t.Fatalf("unexpected handshake phase bytes: got %x want %x", phaseHandshake.Raw, wantPhaseHandshake)
	}

	challengeFrame := client.readFrame(t)
	wantChallenge := control.EncodeKeyChallenge(challenge)
	if !bytes.Equal(challengeFrame.Raw, wantChallenge) {
		t.Fatalf("unexpected key challenge bytes: got %x want %x", challengeFrame.Raw, wantChallenge)
	}
}

func TestBootFlowCompletesHandshakeAndLoginOverTCP(t *testing.T) {
	server := startBootTestServer(t, testConfig())
	client := newBootTestClient(t, server.address())

	expectBootHandshakeStart(t, client, testConfig().Handshake.KeyChallenge)

	client.writeFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	}))

	complete := client.readFrame(t)
	wantComplete := control.EncodeKeyComplete(testConfig().Handshake.KeyComplete)
	if !bytes.Equal(complete.Raw, wantComplete) {
		t.Fatalf("unexpected key complete bytes: got %x want %x", complete.Raw, wantComplete)
	}

	phaseLogin := client.readFrame(t)
	wantPhaseLogin, err := control.EncodePhase(session.PhaseLogin)
	if err != nil {
		t.Fatalf("unexpected phase encode error: %v", err)
	}
	if !bytes.Equal(phaseLogin.Raw, wantPhaseLogin) {
		t.Fatalf("unexpected login phase bytes: got %x want %x", phaseLogin.Raw, wantPhaseLogin)
	}

	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}
	client.writeFrame(t, login2Raw)

	loginSuccess := client.readFrame(t)
	wantSuccess, err := loginproto.EncodeLoginSuccess4(sampleLoginSuccessPacket())
	if err != nil {
		t.Fatalf("unexpected login success encode error: %v", err)
	}
	if !bytes.Equal(loginSuccess.Raw, wantSuccess) {
		t.Fatalf("unexpected login success bytes: got %x want %x", loginSuccess.Raw, wantSuccess)
	}

	empire := client.readFrame(t)
	wantEmpire := loginproto.EncodeEmpire(loginproto.EmpirePacket{Empire: 2})
	if !bytes.Equal(empire.Raw, wantEmpire) {
		t.Fatalf("unexpected empire bytes: got %x want %x", empire.Raw, wantEmpire)
	}

	phaseSelect := client.readFrame(t)
	wantPhaseSelect, err := control.EncodePhase(session.PhaseSelect)
	if err != nil {
		t.Fatalf("unexpected phase encode error: %v", err)
	}
	if !bytes.Equal(phaseSelect.Raw, wantPhaseSelect) {
		t.Fatalf("unexpected select phase bytes: got %x want %x", phaseSelect.Raw, wantPhaseSelect)
	}

	if got := server.currentPhase(); got != session.PhaseSelect {
		t.Fatalf("expected server phase %q, got %q", session.PhaseSelect, got)
	}
}

func TestBootFlowRespondsToStateCheckerProbeOverTCP(t *testing.T) {
	cfg := testConfig()
	cfg.StateChecker = StateCheckerConfig{
		Channels: []control.ChannelStatus{{Port: 13000, Status: control.ChannelStatusNormal}},
	}

	server := startBootTestServer(t, cfg)
	client := newBootTestClient(t, server.address())

	expectBootHandshakeStart(t, client, cfg.Handshake.KeyChallenge)

	client.writeFrame(t, control.EncodeStateChecker())

	respond := client.readFrame(t)
	packet, err := control.DecodeRespondChannelStatus(respond.Frame)
	if err != nil {
		t.Fatalf("decode respond channel status: %v", err)
	}
	if len(packet.Channels) != 1 {
		t.Fatalf("expected 1 channel status entry, got %d", len(packet.Channels))
	}
	if packet.Channels[0].Port != 13000 {
		t.Fatalf("expected channel port 13000, got %d", packet.Channels[0].Port)
	}
	if packet.Channels[0].Status != control.ChannelStatusNormal {
		t.Fatalf("expected channel status %d, got %d", control.ChannelStatusNormal, packet.Channels[0].Status)
	}

	if got := server.currentPhase(); got != session.PhaseHandshake {
		t.Fatalf("expected server phase %q, got %q", session.PhaseHandshake, got)
	}
}

func TestBootFlowEntersGameOverTCP(t *testing.T) {
	server := startBootTestServer(t, testConfig())
	client := newBootTestClient(t, server.address())

	expectBootHandshakeStart(t, client, testConfig().Handshake.KeyChallenge)

	client.writeFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	}))

	complete := client.readFrame(t)
	wantComplete := control.EncodeKeyComplete(testConfig().Handshake.KeyComplete)
	if !bytes.Equal(complete.Raw, wantComplete) {
		t.Fatalf("unexpected key complete bytes: got %x want %x", complete.Raw, wantComplete)
	}

	phaseLogin := client.readFrame(t)
	wantPhaseLogin, err := control.EncodePhase(session.PhaseLogin)
	if err != nil {
		t.Fatalf("unexpected phase encode error: %v", err)
	}
	if !bytes.Equal(phaseLogin.Raw, wantPhaseLogin) {
		t.Fatalf("unexpected login phase bytes: got %x want %x", phaseLogin.Raw, wantPhaseLogin)
	}

	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}
	client.writeFrame(t, login2Raw)

	loginSuccess := client.readFrame(t)
	wantSuccess, err := loginproto.EncodeLoginSuccess4(sampleLoginSuccessPacket())
	if err != nil {
		t.Fatalf("unexpected login success encode error: %v", err)
	}
	if !bytes.Equal(loginSuccess.Raw, wantSuccess) {
		t.Fatalf("unexpected login success bytes: got %x want %x", loginSuccess.Raw, wantSuccess)
	}

	empire := client.readFrame(t)
	wantEmpire := loginproto.EncodeEmpire(loginproto.EmpirePacket{Empire: 2})
	if !bytes.Equal(empire.Raw, wantEmpire) {
		t.Fatalf("unexpected empire bytes: got %x want %x", empire.Raw, wantEmpire)
	}

	phaseSelect := client.readFrame(t)
	wantPhaseSelect, err := control.EncodePhase(session.PhaseSelect)
	if err != nil {
		t.Fatalf("unexpected phase encode error: %v", err)
	}
	if !bytes.Equal(phaseSelect.Raw, wantPhaseSelect) {
		t.Fatalf("unexpected select phase bytes: got %x want %x", phaseSelect.Raw, wantPhaseSelect)
	}

	client.writeFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))

	phaseLoading := client.readFrame(t)
	wantPhaseLoading, err := control.EncodePhase(session.PhaseLoading)
	if err != nil {
		t.Fatalf("unexpected loading phase encode error: %v", err)
	}
	if !bytes.Equal(phaseLoading.Raw, wantPhaseLoading) {
		t.Fatalf("unexpected loading phase bytes: got %x want %x", phaseLoading.Raw, wantPhaseLoading)
	}

	mainCharacter := client.readFrame(t)
	wantMainCharacter, err := worldproto.EncodeMainCharacter(sampleMainCharacter())
	if err != nil {
		t.Fatalf("unexpected main character encode error: %v", err)
	}
	if !bytes.Equal(mainCharacter.Raw, wantMainCharacter) {
		t.Fatalf("unexpected main character bytes: got %x want %x", mainCharacter.Raw, wantMainCharacter)
	}

	playerPoints := client.readFrame(t)
	wantPlayerPoints := worldproto.EncodePlayerPoints(samplePlayerPoints())
	if !bytes.Equal(playerPoints.Raw, wantPlayerPoints) {
		t.Fatalf("unexpected player points bytes: got %x want %x", playerPoints.Raw, wantPlayerPoints)
	}

	client.writeFrame(t, worldproto.EncodeEnterGame())

	phaseGame := client.readFrame(t)
	wantPhaseGame, err := control.EncodePhase(session.PhaseGame)
	if err != nil {
		t.Fatalf("unexpected game phase encode error: %v", err)
	}
	if !bytes.Equal(phaseGame.Raw, wantPhaseGame) {
		t.Fatalf("unexpected game phase bytes: got %x want %x", phaseGame.Raw, wantPhaseGame)
	}

	if got := server.currentPhase(); got != session.PhaseGame {
		t.Fatalf("expected server phase %q, got %q", session.PhaseGame, got)
	}
}

func TestBootFlowCreatesCharacterAndEntersGameOverTCP(t *testing.T) {
	server := startBootTestServer(t, testConfig())
	client := newBootTestClient(t, server.address())

	expectBootHandshakeStart(t, client, testConfig().Handshake.KeyChallenge)
	client.writeFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	}))
	complete := client.readFrame(t)
	wantComplete := control.EncodeKeyComplete(testConfig().Handshake.KeyComplete)
	if !bytes.Equal(complete.Raw, wantComplete) {
		t.Fatalf("unexpected key complete bytes: got %x want %x", complete.Raw, wantComplete)
	}
	phaseLogin := client.readFrame(t)
	wantPhaseLogin, err := control.EncodePhase(session.PhaseLogin)
	if err != nil {
		t.Fatalf("unexpected phase encode error: %v", err)
	}
	if !bytes.Equal(phaseLogin.Raw, wantPhaseLogin) {
		t.Fatalf("unexpected login phase bytes: got %x want %x", phaseLogin.Raw, wantPhaseLogin)
	}
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}
	client.writeFrame(t, login2Raw)
	loginSuccess := client.readFrame(t)
	wantSuccess, err := loginproto.EncodeLoginSuccess4(sampleLoginSuccessPacket())
	if err != nil {
		t.Fatalf("unexpected login success encode error: %v", err)
	}
	if !bytes.Equal(loginSuccess.Raw, wantSuccess) {
		t.Fatalf("unexpected login success bytes: got %x want %x", loginSuccess.Raw, wantSuccess)
	}
	if empire := client.readFrame(t); !bytes.Equal(empire.Raw, loginproto.EncodeEmpire(loginproto.EmpirePacket{Empire: 2})) {
		t.Fatalf("unexpected empire bytes: got %x", empire.Raw)
	}
	phaseSelect := client.readFrame(t)
	wantPhaseSelect, err := control.EncodePhase(session.PhaseSelect)
	if err != nil {
		t.Fatalf("unexpected phase encode error: %v", err)
	}
	if !bytes.Equal(phaseSelect.Raw, wantPhaseSelect) {
		t.Fatalf("unexpected select phase bytes: got %x want %x", phaseSelect.Raw, wantPhaseSelect)
	}

	createRaw, err := worldproto.EncodeCharacterCreate(worldproto.CharacterCreatePacket{Index: 2, Name: "FreshSura", RaceNum: 2, Shape: 1})
	if err != nil {
		t.Fatalf("unexpected create encode error: %v", err)
	}
	client.writeFrame(t, createRaw)
	createSuccess := client.readFrame(t)
	wantCreateSuccess, err := worldproto.EncodePlayerCreateSuccess(worldproto.PlayerCreateSuccessPacket{Index: 2, Player: sampleCreatedPlayer()})
	if err != nil {
		t.Fatalf("unexpected create success encode error: %v", err)
	}
	if !bytes.Equal(createSuccess.Raw, wantCreateSuccess) {
		t.Fatalf("unexpected create success bytes: got %x want %x", createSuccess.Raw, wantCreateSuccess)
	}

	client.writeFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 2}))
	phaseLoading := client.readFrame(t)
	wantPhaseLoading, err := control.EncodePhase(session.PhaseLoading)
	if err != nil {
		t.Fatalf("unexpected loading phase encode error: %v", err)
	}
	if !bytes.Equal(phaseLoading.Raw, wantPhaseLoading) {
		t.Fatalf("unexpected loading phase bytes: got %x want %x", phaseLoading.Raw, wantPhaseLoading)
	}
	mainCharacter := client.readFrame(t)
	wantMainCharacter, err := worldproto.EncodeMainCharacter(sampleCreatedMainCharacter())
	if err != nil {
		t.Fatalf("unexpected created main character encode error: %v", err)
	}
	if !bytes.Equal(mainCharacter.Raw, wantMainCharacter) {
		t.Fatalf("unexpected created main character bytes: got %x want %x", mainCharacter.Raw, wantMainCharacter)
	}
	playerPoints := client.readFrame(t)
	wantPlayerPoints := worldproto.EncodePlayerPoints(sampleCreatedPlayerPoints())
	if !bytes.Equal(playerPoints.Raw, wantPlayerPoints) {
		t.Fatalf("unexpected created player points bytes: got %x want %x", playerPoints.Raw, wantPlayerPoints)
	}
	client.writeFrame(t, worldproto.EncodeEnterGame())
	phaseGame := client.readFrame(t)
	wantPhaseGame, err := control.EncodePhase(session.PhaseGame)
	if err != nil {
		t.Fatalf("unexpected game phase encode error: %v", err)
	}
	if !bytes.Equal(phaseGame.Raw, wantPhaseGame) {
		t.Fatalf("unexpected game phase bytes: got %x want %x", phaseGame.Raw, wantPhaseGame)
	}
	if got := server.currentPhase(); got != session.PhaseGame {
		t.Fatalf("expected server phase %q, got %q", session.PhaseGame, got)
	}
}

func TestBootFlowMovesInGameOverTCP(t *testing.T) {
	server := startBootTestServer(t, testConfig())
	client := newBootTestClient(t, server.address())

	expectBootHandshakeStart(t, client, testConfig().Handshake.KeyChallenge)
	client.writeFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	}))
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}
	client.writeFrame(t, login2Raw)
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	client.writeFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	client.writeFrame(t, worldproto.EncodeEnterGame())
	phaseGame := client.readFrame(t)
	wantPhaseGame, err := control.EncodePhase(session.PhaseGame)
	if err != nil {
		t.Fatalf("unexpected game phase encode error: %v", err)
	}
	if !bytes.Equal(phaseGame.Raw, wantPhaseGame) {
		t.Fatalf("unexpected game phase bytes: got %x want %x", phaseGame.Raw, wantPhaseGame)
	}

	client.writeFrame(t, movep.EncodeMove(sampleMovePacket()))
	moveAck := client.readFrame(t)
	wantMoveAck := movep.EncodeMoveAck(sampleMoveAckPacket())
	if !bytes.Equal(moveAck.Raw, wantMoveAck) {
		t.Fatalf("unexpected move ack bytes: got %x want %x", moveAck.Raw, wantMoveAck)
	}
	if got := server.currentPhase(); got != session.PhaseGame {
		t.Fatalf("expected server phase %q after move, got %q", session.PhaseGame, got)
	}
}

func TestBootFlowDeletesCharacterInSelectOverTCP(t *testing.T) {
	server := startBootTestServer(t, testConfig())
	client := newBootTestClient(t, server.address())

	expectBootHandshakeStart(t, client, testConfig().Handshake.KeyChallenge)
	client.writeFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	}))
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}
	client.writeFrame(t, login2Raw)
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	phaseSelect := client.readFrame(t)
	wantPhaseSelect, err := control.EncodePhase(session.PhaseSelect)
	if err != nil {
		t.Fatalf("unexpected select phase encode error: %v", err)
	}
	if !bytes.Equal(phaseSelect.Raw, wantPhaseSelect) {
		t.Fatalf("unexpected select phase bytes: got %x want %x", phaseSelect.Raw, wantPhaseSelect)
	}

	deleteRaw, err := worldproto.EncodeCharacterDelete(worldproto.CharacterDeletePacket{Index: 1, PrivateCode: "1234567"})
	if err != nil {
		t.Fatalf("unexpected delete encode error: %v", err)
	}
	client.writeFrame(t, deleteRaw)
	deleteSuccess := client.readFrame(t)
	wantDelete := worldproto.EncodePlayerDeleteSuccess(worldproto.PlayerDeleteSuccessPacket{Index: 1})
	if !bytes.Equal(deleteSuccess.Raw, wantDelete) {
		t.Fatalf("unexpected delete success bytes: got %x want %x", deleteSuccess.Raw, wantDelete)
	}
	if got := server.currentPhase(); got != session.PhaseSelect {
		t.Fatalf("expected server phase %q after delete, got %q", session.PhaseSelect, got)
	}
}

func TestBootFlowAcceptsClientVersionDuringLoadingOverTCP(t *testing.T) {
	server := startBootTestServer(t, testConfig())
	client := newBootTestClient(t, server.address())

	expectBootHandshakeStart(t, client, testConfig().Handshake.KeyChallenge)
	client.writeFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	}))
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}
	client.writeFrame(t, login2Raw)
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	client.writeFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))
	phaseLoading := client.readFrame(t)
	wantPhaseLoading, err := control.EncodePhase(session.PhaseLoading)
	if err != nil {
		t.Fatalf("unexpected loading phase encode error: %v", err)
	}
	if !bytes.Equal(phaseLoading.Raw, wantPhaseLoading) {
		t.Fatalf("unexpected loading phase bytes: got %x want %x", phaseLoading.Raw, wantPhaseLoading)
	}
	_ = client.readFrame(t)
	_ = client.readFrame(t)

	clientVersionRaw, err := control.EncodeClientVersion(control.ClientVersionPacket{ExecutableName: "metin2client.bin", Timestamp: "1215955205"})
	if err != nil {
		t.Fatalf("unexpected client version encode error: %v", err)
	}
	client.writeFrame(t, clientVersionRaw)
	client.writeFrame(t, worldproto.EncodeEnterGame())

	phaseGame := client.readFrame(t)
	wantPhaseGame, err := control.EncodePhase(session.PhaseGame)
	if err != nil {
		t.Fatalf("unexpected game phase encode error: %v", err)
	}
	if !bytes.Equal(phaseGame.Raw, wantPhaseGame) {
		t.Fatalf("unexpected game phase bytes after client version: got %x want %x", phaseGame.Raw, wantPhaseGame)
	}
	if got := server.currentPhase(); got != session.PhaseGame {
		t.Fatalf("expected server phase %q after client version, got %q", session.PhaseGame, got)
	}
}

func TestBootFlowSynchronizesInGameOverTCP(t *testing.T) {
	server := startBootTestServer(t, testConfig())
	client := newBootTestClient(t, server.address())

	expectBootHandshakeStart(t, client, testConfig().Handshake.KeyChallenge)
	client.writeFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	}))
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}
	client.writeFrame(t, login2Raw)
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	client.writeFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	client.writeFrame(t, worldproto.EncodeEnterGame())
	phaseGame := client.readFrame(t)
	wantPhaseGame, err := control.EncodePhase(session.PhaseGame)
	if err != nil {
		t.Fatalf("unexpected game phase encode error: %v", err)
	}
	if !bytes.Equal(phaseGame.Raw, wantPhaseGame) {
		t.Fatalf("unexpected game phase bytes: got %x want %x", phaseGame.Raw, wantPhaseGame)
	}

	client.writeFrame(t, movep.EncodeSyncPosition(sampleSyncPositionPacket()))
	syncAck := client.readFrame(t)
	wantSyncAck := movep.EncodeSyncPositionAck(sampleSyncPositionAckPacket())
	if !bytes.Equal(syncAck.Raw, wantSyncAck) {
		t.Fatalf("unexpected sync position ack bytes: got %x want %x", syncAck.Raw, wantSyncAck)
	}
	if got := server.currentPhase(); got != session.PhaseGame {
		t.Fatalf("expected server phase %q after sync position, got %q", session.PhaseGame, got)
	}
}

func TestBootFlowReturnsItemBootstrapAfterPointChangeOverTCP(t *testing.T) {
	cfg := testConfig()
	selectedPlayer := player.NewRuntime(loginticket.Character{
		ID:       2,
		VID:      sampleMainCharacter().VID,
		Name:     sampleMainCharacter().Name,
		MapIndex: 1,
		X:        sampleMainCharacter().X,
		Y:        sampleMainCharacter().Y,
		Inventory: []inventory.ItemInstance{{
			ID:    1001,
			Vnum:  0x11223344,
			Count: 3,
			Slot:  7,
		}},
		Equipment: []inventory.ItemInstance{{
			ID:        2002,
			Vnum:      0x55667788,
			Count:     1,
			Equipped:  true,
			EquipSlot: inventory.EquipmentSlotWeapon,
		}},
	}, player.SessionLink{Login: "mkmk", CharacterIndex: 1})
	cfg.WorldEntry.SelectCharacter = func(index uint8) worldentry.Result {
		if index != 1 {
			return worldentry.Result{Accepted: false}
		}
		return worldentry.Result{Accepted: true, Player: selectedPlayer, MainCharacter: sampleMainCharacter(), PlayerPoints: samplePlayerPoints()}
	}
	cfg.WorldEntry.EnterGame = func(got *player.Runtime) worldentry.EnterGameResult {
		if got != selectedPlayer {
			t.Fatalf("expected selected runtime %p, got %p", selectedPlayer, got)
		}
		selected := got.LiveCharacter()
		addRaw := worldproto.EncodeCharacterAdd(sampleVisibleCharacterAddPacket())
		infoRaw, err := worldproto.EncodeCharacterAdditionalInfo(sampleVisibleCharacterAdditionalInfoPacket())
		if err != nil {
			t.Fatalf("encode additional info: %v", err)
		}
		updateRaw := worldproto.EncodeCharacterUpdate(sampleVisibleCharacterUpdatePacket())
		pointChangeRaw := worldproto.EncodePlayerPointChange(sampleVisiblePlayerPointChangePacket())
		inventoryRaw := itemproto.EncodeSet(itemproto.SetPacket{
			Position: itemproto.Position{WindowType: itemproto.WindowInventory, Cell: uint16(selected.Inventory[0].Slot)},
			Vnum:     selected.Inventory[0].Vnum,
			Count:    uint8(selected.Inventory[0].Count),
		})
		equipmentRaw := itemproto.EncodeSet(itemproto.SetPacket{
			Position: itemproto.Position{WindowType: itemproto.WindowInventory, Cell: itemproto.InventoryMaxCell + 4},
			Vnum:     selected.Equipment[0].Vnum,
			Count:    uint8(selected.Equipment[0].Count),
		})
		return worldentry.EnterGameResult{BootstrapFrames: [][]byte{addRaw, infoRaw, updateRaw, pointChangeRaw, inventoryRaw, equipmentRaw}}
	}

	server := startBootTestServer(t, cfg)
	client := newBootTestClient(t, server.address())

	expectBootHandshakeStart(t, client, cfg.Handshake.KeyChallenge)
	client.writeFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	}))
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}
	client.writeFrame(t, login2Raw)
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	client.writeFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	client.writeFrame(t, worldproto.EncodeEnterGame())

	phaseGame := client.readFrame(t)
	wantPhaseGame, err := control.EncodePhase(session.PhaseGame)
	if err != nil {
		t.Fatalf("unexpected game phase encode error: %v", err)
	}
	if !bytes.Equal(phaseGame.Raw, wantPhaseGame) {
		t.Fatalf("unexpected game phase bytes: got %x want %x", phaseGame.Raw, wantPhaseGame)
	}
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	pointChange := client.readFrame(t)
	wantPointChange := worldproto.EncodePlayerPointChange(sampleVisiblePlayerPointChangePacket())
	if !bytes.Equal(pointChange.Raw, wantPointChange) {
		t.Fatalf("unexpected player point change bytes: got %x want %x", pointChange.Raw, wantPointChange)
	}
	inventoryItem := client.readFrame(t)
	inventorySet, err := itemproto.DecodeSet(inventoryItem.Frame)
	if err != nil {
		t.Fatalf("decode inventory item bootstrap: %v", err)
	}
	if inventorySet.Position.WindowType != itemproto.WindowInventory || inventorySet.Position.Cell != 7 || inventorySet.Vnum != 0x11223344 || inventorySet.Count != 3 {
		t.Fatalf("unexpected inventory item bootstrap packet: %+v", inventorySet)
	}
	equipmentItem := client.readFrame(t)
	equipmentSet, err := itemproto.DecodeSet(equipmentItem.Frame)
	if err != nil {
		t.Fatalf("decode equipment item bootstrap: %v", err)
	}
	if equipmentSet.Position.WindowType != itemproto.WindowInventory || equipmentSet.Position.Cell != itemproto.InventoryMaxCell+4 || equipmentSet.Vnum != 0x55667788 || equipmentSet.Count != 1 {
		t.Fatalf("unexpected equipment item bootstrap packet: %+v", equipmentSet)
	}
	if got := server.currentPhase(); got != session.PhaseGame {
		t.Fatalf("expected server phase %q after item bootstrap, got %q", session.PhaseGame, got)
	}
}

func TestBootFlowReturnsVisibleWorldBootstrapOverTCP(t *testing.T) {
	server := startBootTestServer(t, testVisibleWorldConfig())
	client := newBootTestClient(t, server.address())

	expectBootHandshakeStart(t, client, testVisibleWorldConfig().Handshake.KeyChallenge)
	client.writeFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	}))
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}
	client.writeFrame(t, login2Raw)
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	client.writeFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	_ = client.readFrame(t)
	client.writeFrame(t, worldproto.EncodeEnterGame())

	phaseGame := client.readFrame(t)
	wantPhaseGame, err := control.EncodePhase(session.PhaseGame)
	if err != nil {
		t.Fatalf("unexpected game phase encode error: %v", err)
	}
	if !bytes.Equal(phaseGame.Raw, wantPhaseGame) {
		t.Fatalf("unexpected game phase bytes: got %x want %x", phaseGame.Raw, wantPhaseGame)
	}
	characterAdd := client.readFrame(t)
	wantCharacterAdd := worldproto.EncodeCharacterAdd(sampleVisibleCharacterAddPacket())
	if !bytes.Equal(characterAdd.Raw, wantCharacterAdd) {
		t.Fatalf("unexpected character add bytes: got %x want %x", characterAdd.Raw, wantCharacterAdd)
	}
	characterInfo := client.readFrame(t)
	wantCharacterInfo, err := worldproto.EncodeCharacterAdditionalInfo(sampleVisibleCharacterAdditionalInfoPacket())
	if err != nil {
		t.Fatalf("unexpected character additional info encode error: %v", err)
	}
	if !bytes.Equal(characterInfo.Raw, wantCharacterInfo) {
		t.Fatalf("unexpected character additional info bytes: got %x want %x", characterInfo.Raw, wantCharacterInfo)
	}
	characterUpdate := client.readFrame(t)
	wantCharacterUpdate := worldproto.EncodeCharacterUpdate(sampleVisibleCharacterUpdatePacket())
	if !bytes.Equal(characterUpdate.Raw, wantCharacterUpdate) {
		t.Fatalf("unexpected character update bytes: got %x want %x", characterUpdate.Raw, wantCharacterUpdate)
	}
	pointChange := client.readFrame(t)
	wantPointChange := worldproto.EncodePlayerPointChange(sampleVisiblePlayerPointChangePacket())
	if !bytes.Equal(pointChange.Raw, wantPointChange) {
		t.Fatalf("unexpected player point change bytes: got %x want %x", pointChange.Raw, wantPointChange)
	}
	if got := server.currentPhase(); got != session.PhaseGame {
		t.Fatalf("expected server phase %q after visible bootstrap, got %q", session.PhaseGame, got)
	}
}
