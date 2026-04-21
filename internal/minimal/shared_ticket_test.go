package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	authproto "github.com/MikelCalvo/go-metin2-server/internal/proto/auth"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
)

func TestAuthAndGameSessionFactoriesShareIssuedLoginTicket(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	authFactory := newAuthSessionFactory(store, func() (uint32, error) { return 0x0badf00d, nil })
	gameFactory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	authFlow := authFactory()
	_ = mustCompleteSecureHandshake(t, authFlow)

	login3Raw, err := authproto.EncodeLogin3(authproto.Login3Packet{Login: StubLogin, Password: StubPassword})
	if err != nil {
		t.Fatalf("encode login3: %v", err)
	}
	authOut, err := authFlow.HandleClientFrame(decodeSingleFrame(t, login3Raw))
	if err != nil {
		t.Fatalf("unexpected auth error: %v", err)
	}
	if len(authOut) != 1 {
		t.Fatalf("expected 1 auth frame, got %d", len(authOut))
	}

	authSuccess, err := authproto.DecodeAuthSuccess(decodeSingleFrame(t, authOut[0]))
	if err != nil {
		t.Fatalf("decode auth success: %v", err)
	}
	if authSuccess.LoginKey != 0x0badf00d {
		t.Fatalf("expected issued login key 0x0badf00d, got 0x%08x", authSuccess.LoginKey)
	}

	gameFlow := gameFactory()
	_ = mustCompleteSecureHandshake(t, gameFlow)

	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: StubLogin, LoginKey: authSuccess.LoginKey})
	if err != nil {
		t.Fatalf("encode login2: %v", err)
	}
	loginOut, err := gameFlow.HandleClientFrame(decodeSingleFrame(t, login2Raw))
	if err != nil {
		t.Fatalf("unexpected game login error: %v", err)
	}
	if len(loginOut) != 3 {
		t.Fatalf("expected 3 login frames, got %d", len(loginOut))
	}

	loginSuccess, err := loginproto.DecodeLoginSuccess4(decodeSingleFrame(t, loginOut[0]))
	if err != nil {
		t.Fatalf("decode login success: %v", err)
	}
	if loginSuccess.Players[0].Name != "MkmkWar" {
		t.Fatalf("expected first character MkmkWar, got %q", loginSuccess.Players[0].Name)
	}
	if loginSuccess.Players[1].Name != "MkmkSura" {
		t.Fatalf("expected second character MkmkSura, got %q", loginSuccess.Players[1].Name)
	}

	selectOut, err := gameFlow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1})))
	if err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if len(selectOut) != 3 {
		t.Fatalf("expected 3 select frames, got %d", len(selectOut))
	}

	mainCharacter, err := worldproto.DecodeMainCharacter(decodeSingleFrame(t, selectOut[1]))
	if err != nil {
		t.Fatalf("decode main character: %v", err)
	}
	if mainCharacter.Name != "MkmkSura" {
		t.Fatalf("expected selected character MkmkSura, got %q", mainCharacter.Name)
	}
}

func TestGameSessionFactoryAllowsLoginTicketReuseAcrossFreshGameSessions(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	authFactory := newAuthSessionFactory(store, func() (uint32, error) { return 0x0badf00d, nil })
	gameFactory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	authFlow := authFactory()
	_ = mustCompleteSecureHandshake(t, authFlow)
	login3Raw, err := authproto.EncodeLogin3(authproto.Login3Packet{Login: StubLogin, Password: StubPassword})
	if err != nil {
		t.Fatalf("encode login3: %v", err)
	}
	authOut, err := authFlow.HandleClientFrame(decodeSingleFrame(t, login3Raw))
	if err != nil {
		t.Fatalf("unexpected auth error: %v", err)
	}
	authSuccess, err := authproto.DecodeAuthSuccess(decodeSingleFrame(t, authOut[0]))
	if err != nil {
		t.Fatalf("decode auth success: %v", err)
	}

	firstGameFlow := gameFactory()
	_ = mustCompleteSecureHandshake(t, firstGameFlow)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: StubLogin, LoginKey: authSuccess.LoginKey})
	if err != nil {
		t.Fatalf("encode login2: %v", err)
	}
	firstLoginOut, err := firstGameFlow.HandleClientFrame(decodeSingleFrame(t, login2Raw))
	if err != nil {
		t.Fatalf("unexpected first game login error: %v", err)
	}
	if len(firstLoginOut) != 3 {
		t.Fatalf("expected 3 first-login frames, got %d", len(firstLoginOut))
	}

	secondGameFlow := gameFactory()
	_ = mustCompleteSecureHandshake(t, secondGameFlow)
	secondLoginOut, err := secondGameFlow.HandleClientFrame(decodeSingleFrame(t, login2Raw))
	if err != nil {
		t.Fatalf("unexpected second game login error: %v", err)
	}
	if len(secondLoginOut) != 3 {
		t.Fatalf("expected 3 second-login frames, got %d", len(secondLoginOut))
	}
	loginSuccess, err := loginproto.DecodeLoginSuccess4(decodeSingleFrame(t, secondLoginOut[0]))
	if err != nil {
		t.Fatalf("decode second login success: %v", err)
	}
	if loginSuccess.Players[1].Name != "MkmkSura" {
		t.Fatalf("expected second game session to keep slot 1 available, got %q", loginSuccess.Players[1].Name)
	}

	selectOut, err := secondGameFlow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1})))
	if err != nil {
		t.Fatalf("unexpected second-session character select error: %v", err)
	}
	if len(selectOut) != 3 {
		t.Fatalf("expected 3 second-session select frames, got %d", len(selectOut))
	}
	mainCharacter, err := worldproto.DecodeMainCharacter(decodeSingleFrame(t, selectOut[1]))
	if err != nil {
		t.Fatalf("decode second-session main character: %v", err)
	}
	if mainCharacter.Name != "MkmkSura" {
		t.Fatalf("expected reused ticket to reach selected character MkmkSura, got %q", mainCharacter.Name)
	}
}

func TestCreatedCharacterPersistsAcrossFreshAuthAndGameSessions(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accountStore := accountstore.NewFileStore(t.TempDir())
	keys := []uint32{0x0badf00d, 0x0badf00e}
	keyIndex := 0
	authFactory := newAuthSessionFactoryWithAccountStore(ticketStore, accountStore, func() (uint32, error) {
		key := keys[keyIndex]
		keyIndex++
		return key, nil
	})
	gameFactory, err := newGameSessionFactoryWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accountStore)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	firstAuthFlow := authFactory()
	_ = mustCompleteSecureHandshake(t, firstAuthFlow)
	login3Raw, err := authproto.EncodeLogin3(authproto.Login3Packet{Login: StubLogin, Password: StubPassword})
	if err != nil {
		t.Fatalf("encode login3: %v", err)
	}
	firstAuthOut, err := firstAuthFlow.HandleClientFrame(decodeSingleFrame(t, login3Raw))
	if err != nil {
		t.Fatalf("unexpected first auth error: %v", err)
	}
	firstAuthSuccess, err := authproto.DecodeAuthSuccess(decodeSingleFrame(t, firstAuthOut[0]))
	if err != nil {
		t.Fatalf("decode first auth success: %v", err)
	}

	firstGameFlow := gameFactory()
	_ = mustCompleteSecureHandshake(t, firstGameFlow)
	firstLogin2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: StubLogin, LoginKey: firstAuthSuccess.LoginKey})
	if err != nil {
		t.Fatalf("encode first login2: %v", err)
	}
	if _, err := firstGameFlow.HandleClientFrame(decodeSingleFrame(t, firstLogin2Raw)); err != nil {
		t.Fatalf("unexpected first game login error: %v", err)
	}
	createRaw, err := worldproto.EncodeCharacterCreate(worldproto.CharacterCreatePacket{Index: 2, Name: "FreshSura", RaceNum: 2, Shape: 1})
	if err != nil {
		t.Fatalf("encode character create: %v", err)
	}
	createOut, err := firstGameFlow.HandleClientFrame(decodeSingleFrame(t, createRaw))
	if err != nil {
		t.Fatalf("unexpected character create error: %v", err)
	}
	created, err := worldproto.DecodePlayerCreateSuccess(decodeSingleFrame(t, createOut[0]))
	if err != nil {
		t.Fatalf("decode player create success: %v", err)
	}
	if created.Player.Name != "FreshSura" {
		t.Fatalf("expected created character FreshSura, got %q", created.Player.Name)
	}

	secondAuthFlow := authFactory()
	_ = mustCompleteSecureHandshake(t, secondAuthFlow)
	secondAuthOut, err := secondAuthFlow.HandleClientFrame(decodeSingleFrame(t, login3Raw))
	if err != nil {
		t.Fatalf("unexpected second auth error: %v", err)
	}
	secondAuthSuccess, err := authproto.DecodeAuthSuccess(decodeSingleFrame(t, secondAuthOut[0]))
	if err != nil {
		t.Fatalf("decode second auth success: %v", err)
	}

	secondGameFlow := gameFactory()
	_ = mustCompleteSecureHandshake(t, secondGameFlow)
	secondLogin2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: StubLogin, LoginKey: secondAuthSuccess.LoginKey})
	if err != nil {
		t.Fatalf("encode second login2: %v", err)
	}
	secondLoginOut, err := secondGameFlow.HandleClientFrame(decodeSingleFrame(t, secondLogin2Raw))
	if err != nil {
		t.Fatalf("unexpected second game login error: %v", err)
	}
	loginSuccess, err := loginproto.DecodeLoginSuccess4(decodeSingleFrame(t, secondLoginOut[0]))
	if err != nil {
		t.Fatalf("decode login success: %v", err)
	}
	if loginSuccess.Players[2].Name != "FreshSura" {
		t.Fatalf("expected persisted character FreshSura in slot 2, got %q", loginSuccess.Players[2].Name)
	}

	selectOut, err := secondGameFlow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 2})))
	if err != nil {
		t.Fatalf("unexpected persisted character select error: %v", err)
	}
	mainCharacter, err := worldproto.DecodeMainCharacter(decodeSingleFrame(t, selectOut[1]))
	if err != nil {
		t.Fatalf("decode persisted main character: %v", err)
	}
	if mainCharacter.Name != "FreshSura" {
		t.Fatalf("expected persisted main character FreshSura, got %q", mainCharacter.Name)
	}
}

func TestMovedCharacterPositionPersistsAcrossFreshAuthAndGameSessions(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accountStore := accountstore.NewFileStore(t.TempDir())
	keys := []uint32{0x0badf00d, 0x0badf00e}
	keyIndex := 0
	authFactory := newAuthSessionFactoryWithAccountStore(ticketStore, accountStore, func() (uint32, error) {
		key := keys[keyIndex]
		keyIndex++
		return key, nil
	})
	gameFactory, err := newGameSessionFactoryWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accountStore)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	firstAuthFlow := authFactory()
	_ = mustCompleteSecureHandshake(t, firstAuthFlow)
	login3Raw, err := authproto.EncodeLogin3(authproto.Login3Packet{Login: StubLogin, Password: StubPassword})
	if err != nil {
		t.Fatalf("encode login3: %v", err)
	}
	firstAuthOut, err := firstAuthFlow.HandleClientFrame(decodeSingleFrame(t, login3Raw))
	if err != nil {
		t.Fatalf("unexpected first auth error: %v", err)
	}
	firstAuthSuccess, err := authproto.DecodeAuthSuccess(decodeSingleFrame(t, firstAuthOut[0]))
	if err != nil {
		t.Fatalf("decode first auth success: %v", err)
	}

	firstGameFlow := gameFactory()
	_ = mustCompleteSecureHandshake(t, firstGameFlow)
	firstLogin2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: StubLogin, LoginKey: firstAuthSuccess.LoginKey})
	if err != nil {
		t.Fatalf("encode first login2: %v", err)
	}
	if _, err := firstGameFlow.HandleClientFrame(decodeSingleFrame(t, firstLogin2Raw)); err != nil {
		t.Fatalf("unexpected first game login error: %v", err)
	}
	if _, err := firstGameFlow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected first character select error: %v", err)
	}
	if _, err := firstGameFlow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected first entergame error: %v", err)
	}
	moveRaw := movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 54321, Y: 65432, Time: 0x01020304})
	moveOut, err := firstGameFlow.HandleClientFrame(decodeSingleFrame(t, moveRaw))
	if err != nil {
		t.Fatalf("unexpected first move error: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 move frame, got %d", len(moveOut))
	}

	secondAuthFlow := authFactory()
	_ = mustCompleteSecureHandshake(t, secondAuthFlow)
	secondAuthOut, err := secondAuthFlow.HandleClientFrame(decodeSingleFrame(t, login3Raw))
	if err != nil {
		t.Fatalf("unexpected second auth error: %v", err)
	}
	secondAuthSuccess, err := authproto.DecodeAuthSuccess(decodeSingleFrame(t, secondAuthOut[0]))
	if err != nil {
		t.Fatalf("decode second auth success: %v", err)
	}

	secondGameFlow := gameFactory()
	_ = mustCompleteSecureHandshake(t, secondGameFlow)
	secondLogin2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: StubLogin, LoginKey: secondAuthSuccess.LoginKey})
	if err != nil {
		t.Fatalf("encode second login2: %v", err)
	}
	secondLoginOut, err := secondGameFlow.HandleClientFrame(decodeSingleFrame(t, secondLogin2Raw))
	if err != nil {
		t.Fatalf("unexpected second game login error: %v", err)
	}
	loginSuccess, err := loginproto.DecodeLoginSuccess4(decodeSingleFrame(t, secondLoginOut[0]))
	if err != nil {
		t.Fatalf("decode second login success: %v", err)
	}
	if loginSuccess.Players[1].X != 54321 || loginSuccess.Players[1].Y != 65432 {
		t.Fatalf("expected persisted character position (54321,65432), got (%d,%d)", loginSuccess.Players[1].X, loginSuccess.Players[1].Y)
	}

	selectOut, err := secondGameFlow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1})))
	if err != nil {
		t.Fatalf("unexpected second character select error: %v", err)
	}
	mainCharacter, err := worldproto.DecodeMainCharacter(decodeSingleFrame(t, selectOut[1]))
	if err != nil {
		t.Fatalf("decode persisted moved main character: %v", err)
	}
	if mainCharacter.X != 54321 || mainCharacter.Y != 65432 {
		t.Fatalf("expected persisted main character position (54321,65432), got (%d,%d)", mainCharacter.X, mainCharacter.Y)
	}
}

func TestEmptyAccountRequiresEmpireSelectionBeforeCharacterCreate(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accountStore := accountstore.NewFileStore(t.TempDir())
	if err := accountStore.Save(accountstore.Account{Login: StubLogin}); err != nil {
		t.Fatalf("seed empty account: %v", err)
	}
	keys := []uint32{0x0badf00d, 0x0badf00e}
	keyIndex := 0
	authFactory := newAuthSessionFactoryWithAccountStore(ticketStore, accountStore, func() (uint32, error) {
		key := keys[keyIndex]
		keyIndex++
		return key, nil
	})
	gameFactory, err := newGameSessionFactoryWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accountStore)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	firstAuthFlow := authFactory()
	_ = mustCompleteSecureHandshake(t, firstAuthFlow)
	login3Raw, err := authproto.EncodeLogin3(authproto.Login3Packet{Login: StubLogin, Password: StubPassword})
	if err != nil {
		t.Fatalf("encode login3: %v", err)
	}
	firstAuthOut, err := firstAuthFlow.HandleClientFrame(decodeSingleFrame(t, login3Raw))
	if err != nil {
		t.Fatalf("unexpected first auth error: %v", err)
	}
	firstAuthSuccess, err := authproto.DecodeAuthSuccess(decodeSingleFrame(t, firstAuthOut[0]))
	if err != nil {
		t.Fatalf("decode first auth success: %v", err)
	}

	firstGameFlow := gameFactory()
	_ = mustCompleteSecureHandshake(t, firstGameFlow)
	firstLogin2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: StubLogin, LoginKey: firstAuthSuccess.LoginKey})
	if err != nil {
		t.Fatalf("encode first login2: %v", err)
	}
	firstLoginOut, err := firstGameFlow.HandleClientFrame(decodeSingleFrame(t, firstLogin2Raw))
	if err != nil {
		t.Fatalf("unexpected first game login error: %v", err)
	}
	firstEmpire, err := loginproto.DecodeEmpire(decodeSingleFrame(t, firstLoginOut[1]))
	if err != nil {
		t.Fatalf("decode first empire packet: %v", err)
	}
	if firstEmpire.Empire != 0 {
		t.Fatalf("expected empty account empire 0 before selection, got %d", firstEmpire.Empire)
	}
	firstLoginSuccess, err := loginproto.DecodeLoginSuccess4(decodeSingleFrame(t, firstLoginOut[0]))
	if err != nil {
		t.Fatalf("decode first login success: %v", err)
	}
	if firstLoginSuccess.Players[0].ID != 0 || firstLoginSuccess.Players[0].Name != "" {
		t.Fatalf("expected empty first slot before empire selection, got %+v", firstLoginSuccess.Players[0])
	}

	createRaw, err := worldproto.EncodeCharacterCreate(worldproto.CharacterCreatePacket{Index: 0, Name: "EmpireWar", RaceNum: 0, Shape: 0})
	if err != nil {
		t.Fatalf("encode character create: %v", err)
	}
	createBeforeEmpireOut, err := firstGameFlow.HandleClientFrame(decodeSingleFrame(t, createRaw))
	if err != nil {
		t.Fatalf("unexpected pre-empire create error: %v", err)
	}
	createFailure, err := worldproto.DecodePlayerCreateFailure(decodeSingleFrame(t, createBeforeEmpireOut[0]))
	if err != nil {
		t.Fatalf("decode pre-empire create failure: %v", err)
	}
	if createFailure.Type != 0 {
		t.Fatalf("expected failure type 0 before empire selection, got %d", createFailure.Type)
	}

	empireSelectOut, err := firstGameFlow.HandleClientFrame(decodeSingleFrame(t, loginproto.EncodeEmpireSelect(loginproto.EmpireSelectPacket{Empire: 3})))
	if err != nil {
		t.Fatalf("unexpected empire select error: %v", err)
	}
	selectedEmpire, err := loginproto.DecodeEmpire(decodeSingleFrame(t, empireSelectOut[0]))
	if err != nil {
		t.Fatalf("decode selected empire response: %v", err)
	}
	if selectedEmpire.Empire != 3 {
		t.Fatalf("expected selected empire 3, got %d", selectedEmpire.Empire)
	}

	createAfterEmpireOut, err := firstGameFlow.HandleClientFrame(decodeSingleFrame(t, createRaw))
	if err != nil {
		t.Fatalf("unexpected post-empire create error: %v", err)
	}
	created, err := worldproto.DecodePlayerCreateSuccess(decodeSingleFrame(t, createAfterEmpireOut[0]))
	if err != nil {
		t.Fatalf("decode post-empire create success: %v", err)
	}
	if created.Player.Name != "EmpireWar" {
		t.Fatalf("expected created character EmpireWar, got %q", created.Player.Name)
	}

	secondAuthFlow := authFactory()
	_ = mustCompleteSecureHandshake(t, secondAuthFlow)
	secondAuthOut, err := secondAuthFlow.HandleClientFrame(decodeSingleFrame(t, login3Raw))
	if err != nil {
		t.Fatalf("unexpected second auth error: %v", err)
	}
	secondAuthSuccess, err := authproto.DecodeAuthSuccess(decodeSingleFrame(t, secondAuthOut[0]))
	if err != nil {
		t.Fatalf("decode second auth success: %v", err)
	}

	secondGameFlow := gameFactory()
	_ = mustCompleteSecureHandshake(t, secondGameFlow)
	secondLogin2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: StubLogin, LoginKey: secondAuthSuccess.LoginKey})
	if err != nil {
		t.Fatalf("encode second login2: %v", err)
	}
	secondLoginOut, err := secondGameFlow.HandleClientFrame(decodeSingleFrame(t, secondLogin2Raw))
	if err != nil {
		t.Fatalf("unexpected second game login error: %v", err)
	}
	secondEmpire, err := loginproto.DecodeEmpire(decodeSingleFrame(t, secondLoginOut[1]))
	if err != nil {
		t.Fatalf("decode second empire packet: %v", err)
	}
	if secondEmpire.Empire != 3 {
		t.Fatalf("expected persisted empire 3 after selection, got %d", secondEmpire.Empire)
	}
	secondLoginSuccess, err := loginproto.DecodeLoginSuccess4(decodeSingleFrame(t, secondLoginOut[0]))
	if err != nil {
		t.Fatalf("decode second login success: %v", err)
	}
	if secondLoginSuccess.Players[0].Name != "EmpireWar" {
		t.Fatalf("expected created character EmpireWar after relog, got %q", secondLoginSuccess.Players[0].Name)
	}

	selectOut, err := secondGameFlow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 0})))
	if err != nil {
		t.Fatalf("unexpected created character select after relog: %v", err)
	}
	mainCharacter, err := worldproto.DecodeMainCharacter(decodeSingleFrame(t, selectOut[1]))
	if err != nil {
		t.Fatalf("decode main character after relog: %v", err)
	}
	if mainCharacter.Empire != 3 || mainCharacter.Name != "EmpireWar" {
		t.Fatalf("expected persisted created character with empire 3 after relog, got %+v", mainCharacter)
	}
}
