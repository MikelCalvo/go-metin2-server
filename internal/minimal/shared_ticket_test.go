package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	authproto "github.com/MikelCalvo/go-metin2-server/internal/proto/auth"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
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
	if _, err := authFlow.Start(); err != nil {
		t.Fatalf("unexpected auth start error: %v", err)
	}
	_, err = authFlow.HandleClientFrame(decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	})))
	if err != nil {
		t.Fatalf("unexpected auth handshake error: %v", err)
	}

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
	if _, err := gameFlow.Start(); err != nil {
		t.Fatalf("unexpected game start error: %v", err)
	}
	_, err = gameFlow.HandleClientFrame(decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	})))
	if err != nil {
		t.Fatalf("unexpected game handshake error: %v", err)
	}

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

	loginSuccess, err := loginproto.DecodeLoginSuccess4(decodeSingleFrame(t, loginOut[2]))
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

func TestGameSessionFactoryRejectsAConsumedLoginTicket(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	authFactory := newAuthSessionFactory(store, func() (uint32, error) { return 0x0badf00d, nil })
	gameFactory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	authFlow := authFactory()
	if _, err := authFlow.Start(); err != nil {
		t.Fatalf("unexpected auth start error: %v", err)
	}
	_, err = authFlow.HandleClientFrame(decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	})))
	if err != nil {
		t.Fatalf("unexpected auth handshake error: %v", err)
	}
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
	if _, err := firstGameFlow.Start(); err != nil {
		t.Fatalf("unexpected game start error: %v", err)
	}
	_, err = firstGameFlow.HandleClientFrame(decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	})))
	if err != nil {
		t.Fatalf("unexpected game handshake error: %v", err)
	}
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: StubLogin, LoginKey: authSuccess.LoginKey})
	if err != nil {
		t.Fatalf("encode login2: %v", err)
	}
	if _, err := firstGameFlow.HandleClientFrame(decodeSingleFrame(t, login2Raw)); err != nil {
		t.Fatalf("unexpected first game login error: %v", err)
	}

	secondGameFlow := gameFactory()
	if _, err := secondGameFlow.Start(); err != nil {
		t.Fatalf("unexpected second game start error: %v", err)
	}
	_, err = secondGameFlow.HandleClientFrame(decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	})))
	if err != nil {
		t.Fatalf("unexpected second game handshake error: %v", err)
	}
	loginOut, err := secondGameFlow.HandleClientFrame(decodeSingleFrame(t, login2Raw))
	if err != nil {
		t.Fatalf("unexpected second game login error: %v", err)
	}
	if len(loginOut) != 1 {
		t.Fatalf("expected 1 login failure frame, got %d", len(loginOut))
	}
	failure, err := loginproto.DecodeLoginFailure(decodeSingleFrame(t, loginOut[0]))
	if err != nil {
		t.Fatalf("decode login failure: %v", err)
	}
	if failure.Status != "NOID" {
		t.Fatalf("expected NOID after consuming the ticket, got %q", failure.Status)
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
	if _, err := firstAuthFlow.Start(); err != nil {
		t.Fatalf("unexpected first auth start error: %v", err)
	}
	_, err = firstAuthFlow.HandleClientFrame(decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	})))
	if err != nil {
		t.Fatalf("unexpected first auth handshake error: %v", err)
	}
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
	if _, err := firstGameFlow.Start(); err != nil {
		t.Fatalf("unexpected first game start error: %v", err)
	}
	_, err = firstGameFlow.HandleClientFrame(decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	})))
	if err != nil {
		t.Fatalf("unexpected first game handshake error: %v", err)
	}
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
	if _, err := secondAuthFlow.Start(); err != nil {
		t.Fatalf("unexpected second auth start error: %v", err)
	}
	_, err = secondAuthFlow.HandleClientFrame(decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	})))
	if err != nil {
		t.Fatalf("unexpected second auth handshake error: %v", err)
	}
	secondAuthOut, err := secondAuthFlow.HandleClientFrame(decodeSingleFrame(t, login3Raw))
	if err != nil {
		t.Fatalf("unexpected second auth error: %v", err)
	}
	secondAuthSuccess, err := authproto.DecodeAuthSuccess(decodeSingleFrame(t, secondAuthOut[0]))
	if err != nil {
		t.Fatalf("decode second auth success: %v", err)
	}

	secondGameFlow := gameFactory()
	if _, err := secondGameFlow.Start(); err != nil {
		t.Fatalf("unexpected second game start error: %v", err)
	}
	_, err = secondGameFlow.HandleClientFrame(decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	})))
	if err != nil {
		t.Fatalf("unexpected second game handshake error: %v", err)
	}
	secondLogin2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: StubLogin, LoginKey: secondAuthSuccess.LoginKey})
	if err != nil {
		t.Fatalf("encode second login2: %v", err)
	}
	secondLoginOut, err := secondGameFlow.HandleClientFrame(decodeSingleFrame(t, secondLogin2Raw))
	if err != nil {
		t.Fatalf("unexpected second game login error: %v", err)
	}
	loginSuccess, err := loginproto.DecodeLoginSuccess4(decodeSingleFrame(t, secondLoginOut[2]))
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
