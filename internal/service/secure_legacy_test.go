package service

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	authflow "github.com/MikelCalvo/go-metin2-server/internal/auth"
	"github.com/MikelCalvo/go-metin2-server/internal/authboot"
	"github.com/MikelCalvo/go-metin2-server/internal/boot"
	gameflow "github.com/MikelCalvo/go-metin2-server/internal/game"
	"github.com/MikelCalvo/go-metin2-server/internal/handshake"
	loginflow "github.com/MikelCalvo/go-metin2-server/internal/login"
	"github.com/MikelCalvo/go-metin2-server/internal/player"
	authproto "github.com/MikelCalvo/go-metin2-server/internal/proto/auth"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/securecipher"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
	worldentry "github.com/MikelCalvo/go-metin2-server/internal/worldentry"
)

func readBootHandshakeStartChallenge(t *testing.T, client *secureLegacyTestClient) control.KeyChallengePacket {
	t.Helper()

	phaseHandshakeRaw := client.readExact(t, 5)
	phaseHandshakeFrame := decodeSingleLegacyFrame(t, phaseHandshakeRaw)
	phaseHandshake, err := control.DecodePhase(phaseHandshakeFrame)
	if err != nil {
		t.Fatalf("decode handshake phase: %v", err)
	}
	if phaseHandshake.Phase != session.PhaseHandshake {
		t.Fatalf("expected phase %q, got %q", session.PhaseHandshake, phaseHandshake.Phase)
	}

	challengeRaw := client.readExact(t, 72)
	challengeFrame := decodeSingleLegacyFrame(t, challengeRaw)
	challenge, err := control.DecodeKeyChallenge(challengeFrame)
	if err != nil {
		t.Fatalf("decode key challenge: %v", err)
	}
	return challenge
}

func TestServeLegacyRejectsPlaintextPostHandshakeFramePipelinedAfterKeyResponse(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeLegacy(ctx, listener, testLogger(), func() SessionFlow {
			return boot.NewFlow(boot.Config{
				Handshake: handshake.Config{
					SecureSession: securecipher.NewServerSession(securecipher.ServerConfig{
						Random:     rand.Reader,
						ServerTime: func() uint32 { return 0x01020304 },
					}),
				},
				Login: loginflow.Config{
					Authenticate: func(packet loginproto.Login2Packet) loginflow.Result {
						if packet.Login != "mkmk" || packet.LoginKey != 0x01020304 {
							return loginflow.Result{Accepted: false, FailureStatus: "NOID"}
						}
						return loginflow.Result{
							Accepted: true,
							Empire:   2,
							LoginSuccess4: loginproto.LoginSuccess4Packet{
								Handle:    0x11223344,
								RandomKey: 0x55667788,
							},
						}
					},
				},
			})
		})
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer conn.Close()

	client := newSecureLegacyTestClient(t, conn)
	secureClient := securecipher.NewClientSession(securecipher.ClientConfig{Random: rand.Reader})

	challenge := readBootHandshakeStartChallenge(t, client)
	response, err := secureClient.HandleKeyChallenge(challenge)
	if err != nil {
		t.Fatalf("handle key challenge: %v", err)
	}
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("encode login2: %v", err)
	}
	client.writeRaw(t, append(control.EncodeKeyResponse(response), login2Raw...))

	keyCompleteRaw := client.readExact(t, 76)
	keyCompleteFrame := decodeSingleLegacyFrame(t, keyCompleteRaw)
	keyComplete, err := control.DecodeKeyComplete(keyCompleteFrame)
	if err != nil {
		t.Fatalf("decode key complete: %v", err)
	}
	if err := secureClient.HandleKeyComplete(keyComplete); err != nil {
		t.Fatalf("handle key complete: %v", err)
	}

	phaseLoginFrame := client.readEncryptedFrame(t, secureClient)
	phaseLogin, err := control.DecodePhase(phaseLoginFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase login: %v", err)
	}
	if phaseLogin.Phase != session.PhaseLogin {
		t.Fatalf("expected phase %q, got %q", session.PhaseLogin, phaseLogin.Phase)
	}

	client.expectConnectionClose(t)

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("ServeLegacy returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ServeLegacy to stop")
	}
}

func TestServeLegacyRejectsBufferedPlaintextPostHandshakeFrameAfterKeyResponse(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeLegacy(ctx, listener, testLogger(), func() SessionFlow {
			return boot.NewFlow(boot.Config{
				Handshake: handshake.Config{
					SecureSession: securecipher.NewServerSession(securecipher.ServerConfig{
						Random:     rand.Reader,
						ServerTime: func() uint32 { return 0x01020304 },
					}),
				},
				Login: loginflow.Config{
					Authenticate: func(packet loginproto.Login2Packet) loginflow.Result {
						if packet.Login != "mkmk" || packet.LoginKey != 0x01020304 {
							return loginflow.Result{Accepted: false, FailureStatus: "NOID"}
						}
						return loginflow.Result{
							Accepted: true,
							Empire:   2,
							LoginSuccess4: loginproto.LoginSuccess4Packet{
								Handle:    0x11223344,
								RandomKey: 0x55667788,
							},
						}
					},
				},
			})
		})
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer conn.Close()

	client := newSecureLegacyTestClient(t, conn)
	secureClient := securecipher.NewClientSession(securecipher.ClientConfig{Random: rand.Reader})

	challenge := readBootHandshakeStartChallenge(t, client)
	response, err := secureClient.HandleKeyChallenge(challenge)
	if err != nil {
		t.Fatalf("handle key challenge: %v", err)
	}
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("encode login2: %v", err)
	}
	client.writeRaw(t, append(control.EncodeKeyResponse(response), login2Raw[:3]...))

	keyCompleteRaw := client.readExact(t, 76)
	keyCompleteFrame := decodeSingleLegacyFrame(t, keyCompleteRaw)
	keyComplete, err := control.DecodeKeyComplete(keyCompleteFrame)
	if err != nil {
		t.Fatalf("decode key complete: %v", err)
	}
	if err := secureClient.HandleKeyComplete(keyComplete); err != nil {
		t.Fatalf("handle key complete: %v", err)
	}

	phaseLoginFrame := client.readEncryptedFrame(t, secureClient)
	phaseLogin, err := control.DecodePhase(phaseLoginFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase login: %v", err)
	}
	if phaseLogin.Phase != session.PhaseLogin {
		t.Fatalf("expected phase %q, got %q", session.PhaseLogin, phaseLogin.Phase)
	}

	client.expectConnectionClose(t)

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("ServeLegacy returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ServeLegacy to stop")
	}
}

func TestServeLegacySupportsSecureBootHandshakeAndEncryptedLogin(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeLegacy(ctx, listener, testLogger(), func() SessionFlow {
			return boot.NewFlow(boot.Config{
				Handshake: handshake.Config{
					SecureSession: securecipher.NewServerSession(securecipher.ServerConfig{
						Random:     rand.Reader,
						ServerTime: func() uint32 { return 0x01020304 },
					}),
				},
				Login: loginflow.Config{
					Authenticate: func(packet loginproto.Login2Packet) loginflow.Result {
						if packet.Login != "mkmk" || packet.LoginKey != 0x01020304 {
							return loginflow.Result{Accepted: false, FailureStatus: "NOID"}
						}
						return loginflow.Result{
							Accepted: true,
							Empire:   2,
							LoginSuccess4: loginproto.LoginSuccess4Packet{
								Handle:    0x11223344,
								RandomKey: 0x55667788,
							},
						}
					},
				},
			})
		})
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer conn.Close()

	client := newSecureLegacyTestClient(t, conn)
	secureClient := securecipher.NewClientSession(securecipher.ClientConfig{Random: rand.Reader})

	challenge := readBootHandshakeStartChallenge(t, client)

	response, err := secureClient.HandleKeyChallenge(challenge)
	if err != nil {
		t.Fatalf("handle key challenge: %v", err)
	}
	client.writeRaw(t, control.EncodeKeyResponse(response))

	keyCompleteRaw := client.readExact(t, 76)
	keyCompleteFrame := decodeSingleLegacyFrame(t, keyCompleteRaw)
	keyComplete, err := control.DecodeKeyComplete(keyCompleteFrame)
	if err != nil {
		t.Fatalf("decode key complete: %v", err)
	}
	if err := secureClient.HandleKeyComplete(keyComplete); err != nil {
		t.Fatalf("handle key complete: %v", err)
	}

	phaseLoginFrame := client.readEncryptedFrame(t, secureClient)
	phaseLogin, err := control.DecodePhase(phaseLoginFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase login: %v", err)
	}
	if phaseLogin.Phase != session.PhaseLogin {
		t.Fatalf("expected phase %q, got %q", session.PhaseLogin, phaseLogin.Phase)
	}

	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("encode login2: %v", err)
	}
	client.writeEncryptedFrame(t, secureClient, login2Raw)

	loginSuccessFrame := client.readEncryptedFrame(t, secureClient)
	if _, err := loginproto.DecodeLoginSuccess4(loginSuccessFrame); err != nil {
		t.Fatalf("decode encrypted login success: %v", err)
	}

	empireFrame := client.readEncryptedFrame(t, secureClient)
	empire, err := loginproto.DecodeEmpire(empireFrame)
	if err != nil {
		t.Fatalf("decode encrypted empire: %v", err)
	}
	if empire.Empire != 2 {
		t.Fatalf("expected empire 2, got %d", empire.Empire)
	}

	phaseSelectFrame := client.readEncryptedFrame(t, secureClient)
	phaseSelect, err := control.DecodePhase(phaseSelectFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase select: %v", err)
	}
	if phaseSelect.Phase != session.PhaseSelect {
		t.Fatalf("expected phase %q, got %q", session.PhaseSelect, phaseSelect.Phase)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("ServeLegacy returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ServeLegacy to stop")
	}
}

func TestServeLegacyKeepsTheEncryptedGameSocketAliveThroughSelectionLoadingClientVersionAndEnterGame(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer listener.Close()

	mainCharacter := worldproto.MainCharacterPacket{
		VID:        0x01020305,
		RaceNum:    3,
		Name:       "MkmkSura",
		BGMName:    "",
		BGMVolume:  0,
		X:          1200,
		Y:          2100,
		Z:          0,
		Empire:     2,
		SkillGroup: 1,
	}
	var playerPoints worldproto.PlayerPointsPacket
	playerPoints.Points[1] = 900
	characterAdd := worldproto.CharacterAddPacket{
		VID:         mainCharacter.VID,
		Angle:       0,
		X:           mainCharacter.X,
		Y:           mainCharacter.Y,
		Z:           mainCharacter.Z,
		Type:        6,
		RaceNum:     mainCharacter.RaceNum,
		MovingSpeed: 150,
		AttackSpeed: 100,
		StateFlag:   0,
	}
	characterInfo := worldproto.CharacterAdditionalInfoPacket{
		VID:       mainCharacter.VID,
		Name:      mainCharacter.Name,
		Empire:    mainCharacter.Empire,
		GuildID:   0,
		Level:     12,
		Alignment: 0,
		PKMode:    0,
		MountVnum: 0,
	}
	characterInfo.Parts[0] = 102
	characterInfo.Parts[3] = 202
	characterUpdate := worldproto.CharacterUpdatePacket{
		VID:         mainCharacter.VID,
		MovingSpeed: 150,
		AttackSpeed: 100,
		StateFlag:   0,
		GuildID:     0,
		Alignment:   0,
		PKMode:      0,
		MountVnum:   0,
	}
	characterUpdate.Parts[0] = 102
	characterUpdate.Parts[3] = 202
	pointChange := worldproto.PlayerPointChangePacket{VID: mainCharacter.VID, Type: 1, Amount: 900, Value: 900}
	characterAddRaw := worldproto.EncodeCharacterAdd(characterAdd)
	characterInfoRaw, err := worldproto.EncodeCharacterAdditionalInfo(characterInfo)
	if err != nil {
		t.Fatalf("encode character additional info: %v", err)
	}
	characterUpdateRaw := worldproto.EncodeCharacterUpdate(characterUpdate)
	pointChangeRaw := worldproto.EncodePlayerPointChange(pointChange)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeLegacy(ctx, listener, testLogger(), func() SessionFlow {
			return boot.NewFlow(boot.Config{
				Handshake: handshake.Config{
					SecureSession: securecipher.NewServerSession(securecipher.ServerConfig{
						Random:     rand.Reader,
						ServerTime: func() uint32 { return 0x01020304 },
					}),
				},
				Login: loginflow.Config{
					Authenticate: func(packet loginproto.Login2Packet) loginflow.Result {
						if packet.Login != "mkmk" || packet.LoginKey != 0x01020304 {
							return loginflow.Result{Accepted: false, FailureStatus: "NOID"}
						}
						return loginflow.Result{
							Accepted: true,
							Empire:   2,
							LoginSuccess4: loginproto.LoginSuccess4Packet{
								Handle:    0x11223344,
								RandomKey: 0x55667788,
							},
						}
					},
				},
				WorldEntry: worldentry.Config{
					SelectCharacter: func(uint8) worldentry.Result {
						return worldentry.Result{Accepted: true, MainCharacter: mainCharacter, PlayerPoints: playerPoints}
					},
					EnterGame: func(_ *player.Runtime) worldentry.EnterGameResult {
						return worldentry.EnterGameResult{
							BootstrapFrames: [][]byte{
								characterAddRaw,
								characterInfoRaw,
								characterUpdateRaw,
								pointChangeRaw,
							},
						}
					},
				},
			})
		})
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer conn.Close()

	client := newSecureLegacyTestClient(t, conn)
	secureClient := securecipher.NewClientSession(securecipher.ClientConfig{Random: rand.Reader})

	challenge := readBootHandshakeStartChallenge(t, client)
	response, err := secureClient.HandleKeyChallenge(challenge)
	if err != nil {
		t.Fatalf("handle key challenge: %v", err)
	}
	client.writeRaw(t, control.EncodeKeyResponse(response))

	keyCompleteRaw := client.readExact(t, 76)
	keyCompleteFrame := decodeSingleLegacyFrame(t, keyCompleteRaw)
	keyComplete, err := control.DecodeKeyComplete(keyCompleteFrame)
	if err != nil {
		t.Fatalf("decode key complete: %v", err)
	}
	if err := secureClient.HandleKeyComplete(keyComplete); err != nil {
		t.Fatalf("handle key complete: %v", err)
	}

	phaseLoginFrame := client.readEncryptedFrame(t, secureClient)
	phaseLogin, err := control.DecodePhase(phaseLoginFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase login: %v", err)
	}
	if phaseLogin.Phase != session.PhaseLogin {
		t.Fatalf("expected phase %q, got %q", session.PhaseLogin, phaseLogin.Phase)
	}

	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("encode login2: %v", err)
	}
	client.writeEncryptedFrame(t, secureClient, login2Raw)

	loginSuccessFrame := client.readEncryptedFrame(t, secureClient)
	if _, err := loginproto.DecodeLoginSuccess4(loginSuccessFrame); err != nil {
		t.Fatalf("decode encrypted login success: %v", err)
	}

	empireFrame := client.readEncryptedFrame(t, secureClient)
	empire, err := loginproto.DecodeEmpire(empireFrame)
	if err != nil {
		t.Fatalf("decode encrypted empire: %v", err)
	}
	if empire.Empire != 2 {
		t.Fatalf("expected empire 2, got %d", empire.Empire)
	}

	phaseSelectFrame := client.readEncryptedFrame(t, secureClient)
	phaseSelect, err := control.DecodePhase(phaseSelectFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase select: %v", err)
	}
	if phaseSelect.Phase != session.PhaseSelect {
		t.Fatalf("expected phase %q, got %q", session.PhaseSelect, phaseSelect.Phase)
	}

	client.writeEncryptedFrame(t, secureClient, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))

	phaseLoadingFrame := client.readEncryptedFrame(t, secureClient)
	phaseLoading, err := control.DecodePhase(phaseLoadingFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase loading: %v", err)
	}
	if phaseLoading.Phase != session.PhaseLoading {
		t.Fatalf("expected phase %q, got %q", session.PhaseLoading, phaseLoading.Phase)
	}

	mainCharacterFrame := client.readEncryptedFrame(t, secureClient)
	decodedMainCharacter, err := worldproto.DecodeMainCharacter(mainCharacterFrame)
	if err != nil {
		t.Fatalf("decode encrypted main character: %v", err)
	}
	if decodedMainCharacter.VID != mainCharacter.VID || decodedMainCharacter.Name != mainCharacter.Name || decodedMainCharacter.Empire != mainCharacter.Empire {
		t.Fatalf("unexpected main character packet: %+v", decodedMainCharacter)
	}

	playerPointsFrame := client.readEncryptedFrame(t, secureClient)
	decodedPlayerPoints, err := worldproto.DecodePlayerPoints(playerPointsFrame)
	if err != nil {
		t.Fatalf("decode encrypted player points: %v", err)
	}
	if decodedPlayerPoints.Points[1] != 900 {
		t.Fatalf("expected point[1]=900, got %d", decodedPlayerPoints.Points[1])
	}

	clientVersionRaw, err := control.EncodeClientVersion(control.ClientVersionPacket{ExecutableName: "metin2client.bin", Timestamp: "1215955205"})
	if err != nil {
		t.Fatalf("encode client version: %v", err)
	}
	client.writeEncryptedFrame(t, secureClient, clientVersionRaw)
	client.expectNoBytesWithin(t, 300*time.Millisecond)

	client.writeEncryptedFrame(t, secureClient, worldproto.EncodeEnterGame())

	phaseGameFrame := client.readEncryptedFrame(t, secureClient)
	phaseGame, err := control.DecodePhase(phaseGameFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase game: %v", err)
	}
	if phaseGame.Phase != session.PhaseGame {
		t.Fatalf("expected phase %q, got %q", session.PhaseGame, phaseGame.Phase)
	}

	characterAddFrame := client.readEncryptedFrame(t, secureClient)
	decodedCharacterAdd, err := worldproto.DecodeCharacterAdd(characterAddFrame)
	if err != nil {
		t.Fatalf("decode encrypted character add: %v", err)
	}
	if decodedCharacterAdd.VID != characterAdd.VID || decodedCharacterAdd.RaceNum != characterAdd.RaceNum || decodedCharacterAdd.X != characterAdd.X || decodedCharacterAdd.Y != characterAdd.Y {
		t.Fatalf("unexpected character add packet: %+v", decodedCharacterAdd)
	}

	characterInfoFrame := client.readEncryptedFrame(t, secureClient)
	decodedCharacterInfo, err := worldproto.DecodeCharacterAdditionalInfo(characterInfoFrame)
	if err != nil {
		t.Fatalf("decode encrypted character additional info: %v", err)
	}
	if decodedCharacterInfo.VID != characterInfo.VID || decodedCharacterInfo.Name != characterInfo.Name || decodedCharacterInfo.Level != characterInfo.Level || decodedCharacterInfo.Parts[0] != characterInfo.Parts[0] || decodedCharacterInfo.Parts[3] != characterInfo.Parts[3] {
		t.Fatalf("unexpected character additional info packet: %+v", decodedCharacterInfo)
	}

	characterUpdateFrame := client.readEncryptedFrame(t, secureClient)
	decodedCharacterUpdate, err := worldproto.DecodeCharacterUpdate(characterUpdateFrame)
	if err != nil {
		t.Fatalf("decode encrypted character update: %v", err)
	}
	if decodedCharacterUpdate.VID != characterUpdate.VID || decodedCharacterUpdate.MovingSpeed != characterUpdate.MovingSpeed || decodedCharacterUpdate.AttackSpeed != characterUpdate.AttackSpeed || decodedCharacterUpdate.Parts[0] != characterUpdate.Parts[0] || decodedCharacterUpdate.Parts[3] != characterUpdate.Parts[3] {
		t.Fatalf("unexpected character update packet: %+v", decodedCharacterUpdate)
	}

	pointChangeFrame := client.readEncryptedFrame(t, secureClient)
	decodedPointChange, err := worldproto.DecodePlayerPointChange(pointChangeFrame)
	if err != nil {
		t.Fatalf("decode encrypted player point change: %v", err)
	}
	if decodedPointChange != pointChange {
		t.Fatalf("unexpected player point change packet: %+v", decodedPointChange)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("ServeLegacy returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ServeLegacy to stop")
	}
}

func TestServeLegacyKeepsTheEncryptedGameSocketAliveAfterRejectedEnterGameAndAllowsRetry(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer listener.Close()

	mainCharacter := worldproto.MainCharacterPacket{
		VID:        0x01020304,
		RaceNum:    2,
		Name:       "Mkmk",
		BGMName:    "",
		BGMVolume:  0,
		X:          1000,
		Y:          2000,
		Z:          0,
		Empire:     2,
		SkillGroup: 1,
	}
	playerPoints := worldproto.PlayerPointsPacket{}
	playerPoints.Points[1] = 900
	characterAdd := worldproto.CharacterAddPacket{
		VID:         mainCharacter.VID,
		Angle:       0,
		X:           mainCharacter.X,
		Y:           mainCharacter.Y,
		Z:           mainCharacter.Z,
		RaceNum:     mainCharacter.RaceNum,
		MovingSpeed: 100,
		AttackSpeed: 100,
		StateFlag:   0,
	}
	characterInfo := worldproto.CharacterAdditionalInfoPacket{
		VID:   mainCharacter.VID,
		Name:  mainCharacter.Name,
		Level: 15,
	}
	characterInfo.Parts[0] = 102
	characterInfo.Parts[3] = 202
	characterUpdate := worldproto.CharacterUpdatePacket{
		VID:         mainCharacter.VID,
		MovingSpeed: 100,
		AttackSpeed: 100,
		StateFlag:   0,
		AffectFlags: [2]uint32{},
		GuildID:     0,
		Alignment:   0,
		PKMode:      0,
		MountVnum:   0,
	}
	characterUpdate.Parts[0] = 102
	characterUpdate.Parts[3] = 202
	pointChange := worldproto.PlayerPointChangePacket{VID: mainCharacter.VID, Type: 1, Amount: 900, Value: 900}
	characterAddRaw := worldproto.EncodeCharacterAdd(characterAdd)
	characterInfoRaw, err := worldproto.EncodeCharacterAdditionalInfo(characterInfo)
	if err != nil {
		t.Fatalf("encode character additional info: %v", err)
	}
	characterUpdateRaw := worldproto.EncodeCharacterUpdate(characterUpdate)
	pointChangeRaw := worldproto.EncodePlayerPointChange(pointChange)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		rejected := false
		errCh <- ServeLegacy(ctx, listener, testLogger(), func() SessionFlow {
			return boot.NewFlow(boot.Config{
				Handshake: handshake.Config{
					SecureSession: securecipher.NewServerSession(securecipher.ServerConfig{
						Random:     rand.Reader,
						ServerTime: func() uint32 { return 0x01020304 },
					}),
				},
				Login: loginflow.Config{
					Authenticate: func(packet loginproto.Login2Packet) loginflow.Result {
						if packet.Login != "mkmk" || packet.LoginKey != 0x01020304 {
							return loginflow.Result{Accepted: false, FailureStatus: "NOID"}
						}
						return loginflow.Result{
							Accepted: true,
							Empire:   2,
							LoginSuccess4: loginproto.LoginSuccess4Packet{
								Handle:    0x11223344,
								RandomKey: 0x55667788,
							},
						}
					},
				},
				WorldEntry: worldentry.Config{
					SelectCharacter: func(uint8) worldentry.Result {
						return worldentry.Result{Accepted: true, MainCharacter: mainCharacter, PlayerPoints: playerPoints}
					},
					EnterGame: func(_ *player.Runtime) worldentry.EnterGameResult {
						if !rejected {
							rejected = true
							return worldentry.EnterGameResult{Rejected: true}
						}
						return worldentry.EnterGameResult{
							BootstrapFrames: [][]byte{
								characterAddRaw,
								characterInfoRaw,
								characterUpdateRaw,
								pointChangeRaw,
							},
						}
					},
				},
			})
		})
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer conn.Close()

	client := newSecureLegacyTestClient(t, conn)
	secureClient := securecipher.NewClientSession(securecipher.ClientConfig{Random: rand.Reader})

	challenge := readBootHandshakeStartChallenge(t, client)
	response, err := secureClient.HandleKeyChallenge(challenge)
	if err != nil {
		t.Fatalf("handle key challenge: %v", err)
	}
	client.writeRaw(t, control.EncodeKeyResponse(response))

	keyCompleteRaw := client.readExact(t, 76)
	keyCompleteFrame := decodeSingleLegacyFrame(t, keyCompleteRaw)
	keyComplete, err := control.DecodeKeyComplete(keyCompleteFrame)
	if err != nil {
		t.Fatalf("decode key complete: %v", err)
	}
	if err := secureClient.HandleKeyComplete(keyComplete); err != nil {
		t.Fatalf("handle key complete: %v", err)
	}

	phaseLoginFrame := client.readEncryptedFrame(t, secureClient)
	phaseLogin, err := control.DecodePhase(phaseLoginFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase login: %v", err)
	}
	if phaseLogin.Phase != session.PhaseLogin {
		t.Fatalf("expected phase %q, got %q", session.PhaseLogin, phaseLogin.Phase)
	}

	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("encode login2: %v", err)
	}
	client.writeEncryptedFrame(t, secureClient, login2Raw)

	loginSuccessFrame := client.readEncryptedFrame(t, secureClient)
	if _, err := loginproto.DecodeLoginSuccess4(loginSuccessFrame); err != nil {
		t.Fatalf("decode encrypted login success: %v", err)
	}

	empireFrame := client.readEncryptedFrame(t, secureClient)
	empire, err := loginproto.DecodeEmpire(empireFrame)
	if err != nil {
		t.Fatalf("decode encrypted empire: %v", err)
	}
	if empire.Empire != 2 {
		t.Fatalf("expected empire 2, got %d", empire.Empire)
	}

	phaseSelectFrame := client.readEncryptedFrame(t, secureClient)
	phaseSelect, err := control.DecodePhase(phaseSelectFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase select: %v", err)
	}
	if phaseSelect.Phase != session.PhaseSelect {
		t.Fatalf("expected phase %q, got %q", session.PhaseSelect, phaseSelect.Phase)
	}

	client.writeEncryptedFrame(t, secureClient, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))

	phaseLoadingFrame := client.readEncryptedFrame(t, secureClient)
	phaseLoading, err := control.DecodePhase(phaseLoadingFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase loading: %v", err)
	}
	if phaseLoading.Phase != session.PhaseLoading {
		t.Fatalf("expected phase %q, got %q", session.PhaseLoading, phaseLoading.Phase)
	}

	mainCharacterFrame := client.readEncryptedFrame(t, secureClient)
	decodedMainCharacter, err := worldproto.DecodeMainCharacter(mainCharacterFrame)
	if err != nil {
		t.Fatalf("decode encrypted main character: %v", err)
	}
	if decodedMainCharacter.VID != mainCharacter.VID || decodedMainCharacter.Name != mainCharacter.Name || decodedMainCharacter.Empire != mainCharacter.Empire {
		t.Fatalf("unexpected main character packet: %+v", decodedMainCharacter)
	}

	playerPointsFrame := client.readEncryptedFrame(t, secureClient)
	decodedPlayerPoints, err := worldproto.DecodePlayerPoints(playerPointsFrame)
	if err != nil {
		t.Fatalf("decode encrypted player points: %v", err)
	}
	if decodedPlayerPoints.Points[1] != 900 {
		t.Fatalf("expected point[1]=900, got %d", decodedPlayerPoints.Points[1])
	}

	clientVersionRaw, err := control.EncodeClientVersion(control.ClientVersionPacket{ExecutableName: "metin2client.bin", Timestamp: "1215955205"})
	if err != nil {
		t.Fatalf("encode client version: %v", err)
	}
	client.writeEncryptedFrame(t, secureClient, clientVersionRaw)
	client.expectNoBytesWithin(t, 300*time.Millisecond)

	client.writeEncryptedFrame(t, secureClient, worldproto.EncodeEnterGame())
	client.expectNoBytesWithin(t, 300*time.Millisecond)

	client.writeEncryptedFrame(t, secureClient, worldproto.EncodeEnterGame())

	phaseGameFrame := client.readEncryptedFrame(t, secureClient)
	phaseGame, err := control.DecodePhase(phaseGameFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase game: %v", err)
	}
	if phaseGame.Phase != session.PhaseGame {
		t.Fatalf("expected phase %q, got %q", session.PhaseGame, phaseGame.Phase)
	}

	characterAddFrame := client.readEncryptedFrame(t, secureClient)
	decodedCharacterAdd, err := worldproto.DecodeCharacterAdd(characterAddFrame)
	if err != nil {
		t.Fatalf("decode encrypted character add: %v", err)
	}
	if decodedCharacterAdd.VID != characterAdd.VID || decodedCharacterAdd.X != characterAdd.X || decodedCharacterAdd.Y != characterAdd.Y {
		t.Fatalf("unexpected character add packet: %+v", decodedCharacterAdd)
	}

	characterInfoFrame := client.readEncryptedFrame(t, secureClient)
	decodedCharacterInfo, err := worldproto.DecodeCharacterAdditionalInfo(characterInfoFrame)
	if err != nil {
		t.Fatalf("decode encrypted character additional info: %v", err)
	}
	if decodedCharacterInfo.VID != characterInfo.VID || decodedCharacterInfo.Name != characterInfo.Name {
		t.Fatalf("unexpected character additional info packet: %+v", decodedCharacterInfo)
	}

	characterUpdateFrame := client.readEncryptedFrame(t, secureClient)
	decodedCharacterUpdate, err := worldproto.DecodeCharacterUpdate(characterUpdateFrame)
	if err != nil {
		t.Fatalf("decode encrypted character update: %v", err)
	}
	if decodedCharacterUpdate.VID != characterUpdate.VID {
		t.Fatalf("unexpected character update packet: %+v", decodedCharacterUpdate)
	}

	pointChangeFrame := client.readEncryptedFrame(t, secureClient)
	decodedPointChange, err := worldproto.DecodePlayerPointChange(pointChangeFrame)
	if err != nil {
		t.Fatalf("decode encrypted player point change: %v", err)
	}
	if decodedPointChange != pointChange {
		t.Fatalf("unexpected player point change packet: %+v", decodedPointChange)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("ServeLegacy returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ServeLegacy to stop")
	}
}

func TestServeLegacyKeepsTheEncryptedGameSocketAliveForSelectedCharacterMoveAfterGameEntry(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer listener.Close()

	mainCharacter := worldproto.MainCharacterPacket{
		VID:        0x01020305,
		RaceNum:    3,
		Name:       "MkmkSura",
		BGMName:    "",
		BGMVolume:  0,
		X:          1200,
		Y:          2100,
		Z:          0,
		Empire:     2,
		SkillGroup: 1,
	}
	var playerPoints worldproto.PlayerPointsPacket
	playerPoints.Points[1] = 900
	characterAdd := worldproto.CharacterAddPacket{
		VID:         mainCharacter.VID,
		Angle:       0,
		X:           mainCharacter.X,
		Y:           mainCharacter.Y,
		Z:           mainCharacter.Z,
		Type:        6,
		RaceNum:     mainCharacter.RaceNum,
		MovingSpeed: 150,
		AttackSpeed: 100,
		StateFlag:   0,
	}
	characterInfo := worldproto.CharacterAdditionalInfoPacket{
		VID:       mainCharacter.VID,
		Name:      mainCharacter.Name,
		Empire:    mainCharacter.Empire,
		GuildID:   0,
		Level:     12,
		Alignment: 0,
		PKMode:    0,
		MountVnum: 0,
	}
	characterInfo.Parts[0] = 102
	characterInfo.Parts[3] = 202
	characterUpdate := worldproto.CharacterUpdatePacket{
		VID:         mainCharacter.VID,
		MovingSpeed: 150,
		AttackSpeed: 100,
		StateFlag:   0,
		GuildID:     0,
		Alignment:   0,
		PKMode:      0,
		MountVnum:   0,
	}
	characterUpdate.Parts[0] = 102
	characterUpdate.Parts[3] = 202
	pointChange := worldproto.PlayerPointChangePacket{VID: mainCharacter.VID, Type: 1, Amount: 900, Value: 900}
	characterAddRaw := worldproto.EncodeCharacterAdd(characterAdd)
	characterInfoRaw, err := worldproto.EncodeCharacterAdditionalInfo(characterInfo)
	if err != nil {
		t.Fatalf("encode character additional info: %v", err)
	}
	characterUpdateRaw := worldproto.EncodeCharacterUpdate(characterUpdate)
	pointChangeRaw := worldproto.EncodePlayerPointChange(pointChange)
	movePacket := movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 12345, Y: 23456, Time: 0x01020304}
	moveSeen := make(chan movep.MovePacket, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeLegacy(ctx, listener, testLogger(), func() SessionFlow {
			return boot.NewFlow(boot.Config{
				Handshake: handshake.Config{
					SecureSession: securecipher.NewServerSession(securecipher.ServerConfig{
						Random:     rand.Reader,
						ServerTime: func() uint32 { return 0x01020304 },
					}),
				},
				Login: loginflow.Config{
					Authenticate: func(packet loginproto.Login2Packet) loginflow.Result {
						if packet.Login != "mkmk" || packet.LoginKey != 0x01020304 {
							return loginflow.Result{Accepted: false, FailureStatus: "NOID"}
						}
						return loginflow.Result{
							Accepted: true,
							Empire:   2,
							LoginSuccess4: loginproto.LoginSuccess4Packet{
								Handle:    0x11223344,
								RandomKey: 0x55667788,
							},
						}
					},
				},
				WorldEntry: worldentry.Config{
					SelectCharacter: func(uint8) worldentry.Result {
						return worldentry.Result{Accepted: true, MainCharacter: mainCharacter, PlayerPoints: playerPoints}
					},
					EnterGame: func(_ *player.Runtime) worldentry.EnterGameResult {
						return worldentry.EnterGameResult{
							BootstrapFrames: [][]byte{
								characterAddRaw,
								characterInfoRaw,
								characterUpdateRaw,
								pointChangeRaw,
							},
						}
					},
				},
				Game: gameflow.Config{
					HandleMove: func(packet movep.MovePacket) gameflow.Result {
						select {
						case moveSeen <- packet:
						default:
						}
						return gameflow.Result{Accepted: true, Replication: movep.MoveAckPacket{
							Func: packet.Func, Arg: packet.Arg, Rot: packet.Rot, VID: mainCharacter.VID, X: packet.X, Y: packet.Y, Time: packet.Time, Duration: 250,
						}}
					},
				},
			})
		})
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer conn.Close()

	client := newSecureLegacyTestClient(t, conn)
	secureClient := securecipher.NewClientSession(securecipher.ClientConfig{Random: rand.Reader})

	challenge := readBootHandshakeStartChallenge(t, client)
	response, err := secureClient.HandleKeyChallenge(challenge)
	if err != nil {
		t.Fatalf("handle key challenge: %v", err)
	}
	client.writeRaw(t, control.EncodeKeyResponse(response))

	keyCompleteRaw := client.readExact(t, 76)
	keyCompleteFrame := decodeSingleLegacyFrame(t, keyCompleteRaw)
	keyComplete, err := control.DecodeKeyComplete(keyCompleteFrame)
	if err != nil {
		t.Fatalf("decode key complete: %v", err)
	}
	if err := secureClient.HandleKeyComplete(keyComplete); err != nil {
		t.Fatalf("handle key complete: %v", err)
	}

	phaseLoginFrame := client.readEncryptedFrame(t, secureClient)
	phaseLogin, err := control.DecodePhase(phaseLoginFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase login: %v", err)
	}
	if phaseLogin.Phase != session.PhaseLogin {
		t.Fatalf("expected phase %q, got %q", session.PhaseLogin, phaseLogin.Phase)
	}

	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("encode login2: %v", err)
	}
	client.writeEncryptedFrame(t, secureClient, login2Raw)

	loginSuccessFrame := client.readEncryptedFrame(t, secureClient)
	if _, err := loginproto.DecodeLoginSuccess4(loginSuccessFrame); err != nil {
		t.Fatalf("decode encrypted login success: %v", err)
	}

	empireFrame := client.readEncryptedFrame(t, secureClient)
	empire, err := loginproto.DecodeEmpire(empireFrame)
	if err != nil {
		t.Fatalf("decode encrypted empire: %v", err)
	}
	if empire.Empire != 2 {
		t.Fatalf("expected empire 2, got %d", empire.Empire)
	}

	phaseSelectFrame := client.readEncryptedFrame(t, secureClient)
	phaseSelect, err := control.DecodePhase(phaseSelectFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase select: %v", err)
	}
	if phaseSelect.Phase != session.PhaseSelect {
		t.Fatalf("expected phase %q, got %q", session.PhaseSelect, phaseSelect.Phase)
	}

	client.writeEncryptedFrame(t, secureClient, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))

	phaseLoadingFrame := client.readEncryptedFrame(t, secureClient)
	phaseLoading, err := control.DecodePhase(phaseLoadingFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase loading: %v", err)
	}
	if phaseLoading.Phase != session.PhaseLoading {
		t.Fatalf("expected phase %q, got %q", session.PhaseLoading, phaseLoading.Phase)
	}

	if _, err := worldproto.DecodeMainCharacter(client.readEncryptedFrame(t, secureClient)); err != nil {
		t.Fatalf("decode encrypted main character: %v", err)
	}
	if _, err := worldproto.DecodePlayerPoints(client.readEncryptedFrame(t, secureClient)); err != nil {
		t.Fatalf("decode encrypted player points: %v", err)
	}

	client.writeEncryptedFrame(t, secureClient, worldproto.EncodeEnterGame())

	phaseGameFrame := client.readEncryptedFrame(t, secureClient)
	phaseGame, err := control.DecodePhase(phaseGameFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase game: %v", err)
	}
	if phaseGame.Phase != session.PhaseGame {
		t.Fatalf("expected phase %q, got %q", session.PhaseGame, phaseGame.Phase)
	}

	if _, err := worldproto.DecodeCharacterAdd(client.readEncryptedFrame(t, secureClient)); err != nil {
		t.Fatalf("decode encrypted character add: %v", err)
	}
	if _, err := worldproto.DecodeCharacterAdditionalInfo(client.readEncryptedFrame(t, secureClient)); err != nil {
		t.Fatalf("decode encrypted character additional info: %v", err)
	}
	if _, err := worldproto.DecodeCharacterUpdate(client.readEncryptedFrame(t, secureClient)); err != nil {
		t.Fatalf("decode encrypted character update: %v", err)
	}
	if _, err := worldproto.DecodePlayerPointChange(client.readEncryptedFrame(t, secureClient)); err != nil {
		t.Fatalf("decode encrypted player point change: %v", err)
	}

	moveRaw := movep.EncodeMove(movePacket)
	client.writeEncryptedFrame(t, secureClient, moveRaw)

	moveAckFrame := client.readEncryptedFrame(t, secureClient)
	moveAck, err := movep.DecodeMoveAck(moveAckFrame)
	if err != nil {
		t.Fatalf("decode encrypted move ack: %v", err)
	}
	if moveAck.Func != movePacket.Func || moveAck.Arg != movePacket.Arg || moveAck.Rot != movePacket.Rot || moveAck.VID != mainCharacter.VID || moveAck.X != movePacket.X || moveAck.Y != movePacket.Y || moveAck.Time != movePacket.Time || moveAck.Duration != 250 {
		t.Fatalf("unexpected encrypted move ack: %+v", moveAck)
	}

	select {
	case observed := <-moveSeen:
		if observed != movePacket {
			t.Fatalf("unexpected move packet seen by handler: %+v", observed)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected move handler to observe the encrypted move packet")
	}

	client.expectNoBytesWithin(t, 300*time.Millisecond)

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("ServeLegacy returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ServeLegacy to stop")
	}
}

func TestServeLegacyKeepsTheEncryptedGameSocketAliveForSelectedCharacterSyncPositionAfterGameEntry(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer listener.Close()

	mainCharacter := worldproto.MainCharacterPacket{
		VID:        0x01020305,
		RaceNum:    3,
		Name:       "MkmkSura",
		BGMName:    "",
		BGMVolume:  0,
		X:          1200,
		Y:          2100,
		Z:          0,
		Empire:     2,
		SkillGroup: 1,
	}
	var playerPoints worldproto.PlayerPointsPacket
	playerPoints.Points[1] = 900
	characterAdd := worldproto.CharacterAddPacket{
		VID:         mainCharacter.VID,
		Angle:       0,
		X:           mainCharacter.X,
		Y:           mainCharacter.Y,
		Z:           mainCharacter.Z,
		Type:        6,
		RaceNum:     mainCharacter.RaceNum,
		MovingSpeed: 150,
		AttackSpeed: 100,
		StateFlag:   0,
	}
	characterInfo := worldproto.CharacterAdditionalInfoPacket{
		VID:       mainCharacter.VID,
		Name:      mainCharacter.Name,
		Empire:    mainCharacter.Empire,
		GuildID:   0,
		Level:     12,
		Alignment: 0,
		PKMode:    0,
		MountVnum: 0,
	}
	characterInfo.Parts[0] = 102
	characterInfo.Parts[3] = 202
	characterUpdate := worldproto.CharacterUpdatePacket{
		VID:         mainCharacter.VID,
		MovingSpeed: 150,
		AttackSpeed: 100,
		StateFlag:   0,
		GuildID:     0,
		Alignment:   0,
		PKMode:      0,
		MountVnum:   0,
	}
	characterUpdate.Parts[0] = 102
	characterUpdate.Parts[3] = 202
	pointChange := worldproto.PlayerPointChangePacket{VID: mainCharacter.VID, Type: 1, Amount: 900, Value: 900}
	characterAddRaw := worldproto.EncodeCharacterAdd(characterAdd)
	characterInfoRaw, err := worldproto.EncodeCharacterAdditionalInfo(characterInfo)
	if err != nil {
		t.Fatalf("encode character additional info: %v", err)
	}
	characterUpdateRaw := worldproto.EncodeCharacterUpdate(characterUpdate)
	pointChangeRaw := worldproto.EncodePlayerPointChange(pointChange)
	syncPacket := movep.SyncPositionPacket{Elements: []movep.SyncPositionElement{{VID: mainCharacter.VID, X: 1400, Y: 2500}}}
	syncSeen := make(chan movep.SyncPositionPacket, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeLegacy(ctx, listener, testLogger(), func() SessionFlow {
			return boot.NewFlow(boot.Config{
				Handshake: handshake.Config{
					SecureSession: securecipher.NewServerSession(securecipher.ServerConfig{
						Random:     rand.Reader,
						ServerTime: func() uint32 { return 0x01020304 },
					}),
				},
				Login: loginflow.Config{
					Authenticate: func(packet loginproto.Login2Packet) loginflow.Result {
						if packet.Login != "mkmk" || packet.LoginKey != 0x01020304 {
							return loginflow.Result{Accepted: false, FailureStatus: "NOID"}
						}
						return loginflow.Result{
							Accepted: true,
							Empire:   2,
							LoginSuccess4: loginproto.LoginSuccess4Packet{
								Handle:    0x11223344,
								RandomKey: 0x55667788,
							},
						}
					},
				},
				WorldEntry: worldentry.Config{
					SelectCharacter: func(uint8) worldentry.Result {
						return worldentry.Result{Accepted: true, MainCharacter: mainCharacter, PlayerPoints: playerPoints}
					},
					EnterGame: func(_ *player.Runtime) worldentry.EnterGameResult {
						return worldentry.EnterGameResult{
							BootstrapFrames: [][]byte{
								characterAddRaw,
								characterInfoRaw,
								characterUpdateRaw,
								pointChangeRaw,
							},
						}
					},
				},
				Game: gameflow.Config{
					HandleSyncPosition: func(packet movep.SyncPositionPacket) gameflow.SyncPositionResult {
						select {
						case syncSeen <- packet:
						default:
						}
						return gameflow.SyncPositionResult{Accepted: true, Synchronization: movep.SyncPositionAckPacket{Elements: []movep.SyncPositionElement{{VID: mainCharacter.VID, X: 1400, Y: 2500}}}}
					},
				},
			})
		})
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer conn.Close()

	client := newSecureLegacyTestClient(t, conn)
	secureClient := securecipher.NewClientSession(securecipher.ClientConfig{Random: rand.Reader})

	challenge := readBootHandshakeStartChallenge(t, client)
	response, err := secureClient.HandleKeyChallenge(challenge)
	if err != nil {
		t.Fatalf("handle key challenge: %v", err)
	}
	client.writeRaw(t, control.EncodeKeyResponse(response))

	keyCompleteRaw := client.readExact(t, 76)
	keyCompleteFrame := decodeSingleLegacyFrame(t, keyCompleteRaw)
	keyComplete, err := control.DecodeKeyComplete(keyCompleteFrame)
	if err != nil {
		t.Fatalf("decode key complete: %v", err)
	}
	if err := secureClient.HandleKeyComplete(keyComplete); err != nil {
		t.Fatalf("handle key complete: %v", err)
	}

	phaseLoginFrame := client.readEncryptedFrame(t, secureClient)
	phaseLogin, err := control.DecodePhase(phaseLoginFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase login: %v", err)
	}
	if phaseLogin.Phase != session.PhaseLogin {
		t.Fatalf("expected phase %q, got %q", session.PhaseLogin, phaseLogin.Phase)
	}

	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("encode login2: %v", err)
	}
	client.writeEncryptedFrame(t, secureClient, login2Raw)

	loginSuccessFrame := client.readEncryptedFrame(t, secureClient)
	if _, err := loginproto.DecodeLoginSuccess4(loginSuccessFrame); err != nil {
		t.Fatalf("decode encrypted login success: %v", err)
	}

	empireFrame := client.readEncryptedFrame(t, secureClient)
	empire, err := loginproto.DecodeEmpire(empireFrame)
	if err != nil {
		t.Fatalf("decode encrypted empire: %v", err)
	}
	if empire.Empire != 2 {
		t.Fatalf("expected empire 2, got %d", empire.Empire)
	}

	phaseSelectFrame := client.readEncryptedFrame(t, secureClient)
	phaseSelect, err := control.DecodePhase(phaseSelectFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase select: %v", err)
	}
	if phaseSelect.Phase != session.PhaseSelect {
		t.Fatalf("expected phase %q, got %q", session.PhaseSelect, phaseSelect.Phase)
	}

	client.writeEncryptedFrame(t, secureClient, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))

	phaseLoadingFrame := client.readEncryptedFrame(t, secureClient)
	phaseLoading, err := control.DecodePhase(phaseLoadingFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase loading: %v", err)
	}
	if phaseLoading.Phase != session.PhaseLoading {
		t.Fatalf("expected phase %q, got %q", session.PhaseLoading, phaseLoading.Phase)
	}

	if _, err := worldproto.DecodeMainCharacter(client.readEncryptedFrame(t, secureClient)); err != nil {
		t.Fatalf("decode encrypted main character: %v", err)
	}
	if _, err := worldproto.DecodePlayerPoints(client.readEncryptedFrame(t, secureClient)); err != nil {
		t.Fatalf("decode encrypted player points: %v", err)
	}

	client.writeEncryptedFrame(t, secureClient, worldproto.EncodeEnterGame())

	phaseGameFrame := client.readEncryptedFrame(t, secureClient)
	phaseGame, err := control.DecodePhase(phaseGameFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase game: %v", err)
	}
	if phaseGame.Phase != session.PhaseGame {
		t.Fatalf("expected phase %q, got %q", session.PhaseGame, phaseGame.Phase)
	}

	if _, err := worldproto.DecodeCharacterAdd(client.readEncryptedFrame(t, secureClient)); err != nil {
		t.Fatalf("decode encrypted character add: %v", err)
	}
	if _, err := worldproto.DecodeCharacterAdditionalInfo(client.readEncryptedFrame(t, secureClient)); err != nil {
		t.Fatalf("decode encrypted character additional info: %v", err)
	}
	if _, err := worldproto.DecodeCharacterUpdate(client.readEncryptedFrame(t, secureClient)); err != nil {
		t.Fatalf("decode encrypted character update: %v", err)
	}
	if _, err := worldproto.DecodePlayerPointChange(client.readEncryptedFrame(t, secureClient)); err != nil {
		t.Fatalf("decode encrypted player point change: %v", err)
	}

	syncRaw := movep.EncodeSyncPosition(syncPacket)
	client.writeEncryptedFrame(t, secureClient, syncRaw)

	syncAckFrame := client.readEncryptedFrame(t, secureClient)
	syncAck, err := movep.DecodeSyncPositionAck(syncAckFrame)
	if err != nil {
		t.Fatalf("decode encrypted sync position ack: %v", err)
	}
	if len(syncAck.Elements) != 1 {
		t.Fatalf("expected 1 sync element, got %d", len(syncAck.Elements))
	}
	if syncAck.Elements[0].VID != mainCharacter.VID || syncAck.Elements[0].X != 1400 || syncAck.Elements[0].Y != 2500 {
		t.Fatalf("unexpected encrypted sync position ack: %+v", syncAck.Elements[0])
	}

	select {
	case observed := <-syncSeen:
		if len(observed.Elements) != 1 || observed.Elements[0] != syncPacket.Elements[0] {
			t.Fatalf("unexpected sync packet seen by handler: %+v", observed)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected sync handler to observe the encrypted sync-position packet")
	}

	client.expectNoBytesWithin(t, 300*time.Millisecond)

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("ServeLegacy returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ServeLegacy to stop")
	}
}

func TestServeLegacySupportsSecureAuthBootHandshakeAndEncryptedLogin3(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeLegacy(ctx, listener, testLogger(), func() SessionFlow {
			return authboot.NewFlow(authboot.Config{
				Handshake: handshake.Config{
					SecureSession: securecipher.NewServerSession(securecipher.ServerConfig{
						Random:     rand.Reader,
						ServerTime: func() uint32 { return 0x0A0B0C0D },
					}),
				},
				Auth: authflow.Config{
					Authenticate: func(packet authproto.Login3Packet) authflow.Result {
						if packet.Login != "mkmk" || packet.Password != "hunter2" {
							return authflow.Result{Accepted: false, FailureStatus: "WRONGPWD"}
						}
						return authflow.Result{Accepted: true, LoginKey: 0x01020304}
					},
				},
			})
		})
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer conn.Close()

	client := newSecureLegacyTestClient(t, conn)
	secureClient := securecipher.NewClientSession(securecipher.ClientConfig{Random: rand.Reader})

	challengeRaw := client.readExact(t, 72)
	challengeFrame := decodeSingleLegacyFrame(t, challengeRaw)
	challenge, err := control.DecodeKeyChallenge(challengeFrame)
	if err != nil {
		t.Fatalf("decode key challenge: %v", err)
	}

	response, err := secureClient.HandleKeyChallenge(challenge)
	if err != nil {
		t.Fatalf("handle key challenge: %v", err)
	}
	client.writeRaw(t, control.EncodeKeyResponse(response))

	keyCompleteRaw := client.readExact(t, 76)
	keyCompleteFrame := decodeSingleLegacyFrame(t, keyCompleteRaw)
	keyComplete, err := control.DecodeKeyComplete(keyCompleteFrame)
	if err != nil {
		t.Fatalf("decode key complete: %v", err)
	}
	if err := secureClient.HandleKeyComplete(keyComplete); err != nil {
		t.Fatalf("handle key complete: %v", err)
	}

	phaseAuthFrame := client.readEncryptedFrame(t, secureClient)
	phaseAuth, err := control.DecodePhase(phaseAuthFrame)
	if err != nil {
		t.Fatalf("decode encrypted phase auth: %v", err)
	}
	if phaseAuth.Phase != session.PhaseAuth {
		t.Fatalf("expected phase %q, got %q", session.PhaseAuth, phaseAuth.Phase)
	}

	login3Raw, err := authproto.EncodeLogin3(authproto.Login3Packet{Login: "mkmk", Password: "hunter2"})
	if err != nil {
		t.Fatalf("encode login3: %v", err)
	}
	client.writeEncryptedFrame(t, secureClient, login3Raw)

	authSuccessFrame := client.readEncryptedFrame(t, secureClient)
	if _, err := authproto.DecodeAuthSuccess(authSuccessFrame); err != nil {
		t.Fatalf("decode encrypted auth success: %v", err)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("ServeLegacy returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ServeLegacy to stop")
	}
}

type secureLegacyTestClient struct {
	conn    net.Conn
	decoder *frame.Decoder
	queued  []frame.Frame
}

func newSecureLegacyTestClient(t *testing.T, conn net.Conn) *secureLegacyTestClient {
	t.Helper()
	return &secureLegacyTestClient{conn: conn, decoder: frame.NewDecoder(8192)}
}

func (c *secureLegacyTestClient) readExact(t *testing.T, n int) []byte {
	t.Helper()
	buf := make([]byte, n)
	if err := c.conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	if _, err := io.ReadFull(c.conn, buf); err != nil {
		t.Fatalf("read exact %d bytes: %v", n, err)
	}
	return buf
}

func (c *secureLegacyTestClient) writeRaw(t *testing.T, raw []byte) {
	t.Helper()
	if err := writeAll(c.conn, raw); err != nil {
		t.Fatalf("write raw: %v", err)
	}
}

func (c *secureLegacyTestClient) writeEncryptedFrame(t *testing.T, client *securecipher.ClientSession, raw []byte) {
	t.Helper()
	encrypted, err := client.EncryptOutgoing(raw)
	if err != nil {
		t.Fatalf("encrypt outgoing frame: %v", err)
	}
	if err := writeAll(c.conn, encrypted); err != nil {
		t.Fatalf("write encrypted frame: %v", err)
	}
}

func (c *secureLegacyTestClient) readEncryptedFrame(t *testing.T, client *securecipher.ClientSession) frame.Frame {
	t.Helper()
	if len(c.queued) > 0 {
		out := c.queued[0]
		c.queued = c.queued[1:]
		return out
	}

	buffer := make([]byte, 8192)
	for {
		if err := c.conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		n, err := c.conn.Read(buffer)
		if err != nil {
			t.Fatalf("read encrypted frame: %v", err)
		}
		decrypted, err := client.DecryptIncoming(buffer[:n])
		if err != nil {
			t.Fatalf("decrypt incoming frame: %v", err)
		}
		frames, err := c.decoder.Feed(decrypted)
		if err != nil {
			t.Fatalf("decode decrypted frame: %v", err)
		}
		if len(frames) == 0 {
			continue
		}
		c.queued = append(c.queued, frames...)
		out := c.queued[0]
		c.queued = c.queued[1:]
		return out
	}
}

func (c *secureLegacyTestClient) expectNoBytesWithin(t *testing.T, timeout time.Duration) {
	t.Helper()
	if len(c.queued) > 0 {
		t.Fatalf("expected no queued frames, got %d", len(c.queued))
	}
	buffer := make([]byte, 8192)
	if err := c.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	n, err := c.conn.Read(buffer)
	if err == nil {
		t.Fatalf("expected no bytes, got %d bytes", n)
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return
	}
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		t.Fatalf("expected idle encrypted socket, got close: %v", err)
	}
	t.Fatalf("read while expecting no bytes: %v", err)
}

func (c *secureLegacyTestClient) expectConnectionClose(t *testing.T) {
	t.Helper()
	buffer := make([]byte, 8192)
	if err := c.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	for {
		n, err := c.conn.Read(buffer)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				t.Fatal("expected connection close, but connection remained open")
			}
			t.Fatalf("read while waiting for connection close: %v", err)
		}
		if n == 0 {
			continue
		}
		t.Fatalf("expected connection close without additional frames, got %d bytes", n)
	}
}

func decodeSingleLegacyFrame(t *testing.T, raw []byte) frame.Frame {
	t.Helper()
	decoder := frame.NewDecoder(8192)
	frames, err := decoder.Feed(raw)
	if err != nil {
		t.Fatalf("decode single frame: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}
	return frames[0]
}

func writeAll(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}
