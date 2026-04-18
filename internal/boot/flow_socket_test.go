package boot

import (
	"bytes"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

func TestBootFlowCompletesHandshakeAndLoginOverTCP(t *testing.T) {
	server := startBootTestServer(t, testConfig())
	client := newBootTestClient(t, server.address())

	challenge := client.readFrame(t)
	wantChallenge := control.EncodeKeyChallenge(testConfig().Handshake.KeyChallenge)
	if !bytes.Equal(challenge.Raw, wantChallenge) {
		t.Fatalf("unexpected key challenge bytes: got %x want %x", challenge.Raw, wantChallenge)
	}

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

	loginSuccess := client.readFrame(t)
	wantSuccess, err := loginproto.EncodeLoginSuccess4(sampleLoginSuccessPacket())
	if err != nil {
		t.Fatalf("unexpected login success encode error: %v", err)
	}
	if !bytes.Equal(loginSuccess.Raw, wantSuccess) {
		t.Fatalf("unexpected login success bytes: got %x want %x", loginSuccess.Raw, wantSuccess)
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

	challenge := client.readFrame(t)
	wantChallenge := control.EncodeKeyChallenge(cfg.Handshake.KeyChallenge)
	if !bytes.Equal(challenge.Raw, wantChallenge) {
		t.Fatalf("unexpected key challenge bytes: got %x want %x", challenge.Raw, wantChallenge)
	}

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

	challenge := client.readFrame(t)
	wantChallenge := control.EncodeKeyChallenge(testConfig().Handshake.KeyChallenge)
	if !bytes.Equal(challenge.Raw, wantChallenge) {
		t.Fatalf("unexpected key challenge bytes: got %x want %x", challenge.Raw, wantChallenge)
	}

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

	loginSuccess := client.readFrame(t)
	wantSuccess, err := loginproto.EncodeLoginSuccess4(sampleLoginSuccessPacket())
	if err != nil {
		t.Fatalf("unexpected login success encode error: %v", err)
	}
	if !bytes.Equal(loginSuccess.Raw, wantSuccess) {
		t.Fatalf("unexpected login success bytes: got %x want %x", loginSuccess.Raw, wantSuccess)
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

	challenge := client.readFrame(t)
	wantChallenge := control.EncodeKeyChallenge(testConfig().Handshake.KeyChallenge)
	if !bytes.Equal(challenge.Raw, wantChallenge) {
		t.Fatalf("unexpected key challenge bytes: got %x want %x", challenge.Raw, wantChallenge)
	}
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
	loginSuccess := client.readFrame(t)
	wantSuccess, err := loginproto.EncodeLoginSuccess4(sampleLoginSuccessPacket())
	if err != nil {
		t.Fatalf("unexpected login success encode error: %v", err)
	}
	if !bytes.Equal(loginSuccess.Raw, wantSuccess) {
		t.Fatalf("unexpected login success bytes: got %x want %x", loginSuccess.Raw, wantSuccess)
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

	_ = client.readFrame(t)
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

func TestBootFlowReturnsVisibleWorldBootstrapOverTCP(t *testing.T) {
	server := startBootTestServer(t, testVisibleWorldConfig())
	client := newBootTestClient(t, server.address())

	_ = client.readFrame(t)
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
	if got := server.currentPhase(); got != session.PhaseGame {
		t.Fatalf("expected server phase %q after visible bootstrap, got %q", session.PhaseGame, got)
	}
}
