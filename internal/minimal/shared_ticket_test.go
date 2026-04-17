package minimal

import (
	"testing"

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
