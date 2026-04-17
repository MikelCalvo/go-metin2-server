package minimal

import (
	"bytes"
	"errors"
	"net"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	authproto "github.com/MikelCalvo/go-metin2-server/internal/proto/auth"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

func TestNewAuthSessionFactoryAcceptsStubCredentials(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	flow := newAuthSessionFactory(store, func() (uint32, error) { return 0x01020304, nil })()

	startOut, err := flow.Start()
	if err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected 1 start frame, got %d", len(startOut))
	}
	challenge := decodeSingleFrame(t, startOut[0])
	if challenge.Header != control.HeaderKeyChallenge {
		t.Fatalf("expected key challenge header 0x%04x, got 0x%04x", control.HeaderKeyChallenge, challenge.Header)
	}

	handshakeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	})))
	if err != nil {
		t.Fatalf("unexpected handshake error: %v", err)
	}
	if len(handshakeOut) != 2 {
		t.Fatalf("expected 2 handshake frames, got %d", len(handshakeOut))
	}

	phaseAuth := decodeSingleFrame(t, handshakeOut[1])
	wantPhaseAuth, err := control.EncodePhase(session.PhaseAuth)
	if err != nil {
		t.Fatalf("unexpected phase encode error: %v", err)
	}
	if !bytes.Equal(handshakeOut[1], wantPhaseAuth) || phaseAuth.Header != control.HeaderPhase {
		t.Fatalf("unexpected phase(auth) frame: got %x want %x", handshakeOut[1], wantPhaseAuth)
	}

	login3Raw, err := authproto.EncodeLogin3(authproto.Login3Packet{Login: StubLogin, Password: StubPassword})
	if err != nil {
		t.Fatalf("unexpected login3 encode error: %v", err)
	}
	authOut, err := flow.HandleClientFrame(decodeSingleFrame(t, login3Raw))
	if err != nil {
		t.Fatalf("unexpected auth error: %v", err)
	}
	if len(authOut) != 1 {
		t.Fatalf("expected 1 auth frame, got %d", len(authOut))
	}

	success, err := authproto.DecodeAuthSuccess(decodeSingleFrame(t, authOut[0]))
	if err != nil {
		t.Fatalf("unexpected auth success decode error: %v", err)
	}
	if success.LoginKey != 0x01020304 || success.Result != 1 {
		t.Fatalf("unexpected auth success packet: %+v", success)
	}

	issued, err := store.Consume(StubLogin, success.LoginKey)
	if err != nil {
		t.Fatalf("expected issued login ticket, got error: %v", err)
	}
	if len(issued.Characters) != 2 {
		t.Fatalf("expected 2 stub characters in issued ticket, got %d", len(issued.Characters))
	}
}

func TestNewGameSessionFactoryAdvertisesConfiguredPublicAddrAndPort(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Characters: stubCharacters()}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "192.168.1.101"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flow := factory()
	if _, err := flow.Start(); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	_, err = flow.HandleClientFrame(decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	})))
	if err != nil {
		t.Fatalf("unexpected handshake error: %v", err)
	}

	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: StubLogin, LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}
	loginOut, err := flow.HandleClientFrame(decodeSingleFrame(t, login2Raw))
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}
	if len(loginOut) != 3 {
		t.Fatalf("expected 3 login frames, got %d", len(loginOut))
	}

	success, err := loginproto.DecodeLoginSuccess4(decodeSingleFrame(t, loginOut[2]))
	if err != nil {
		t.Fatalf("unexpected login success decode error: %v", err)
	}

	ip := net.ParseIP("192.168.1.101").To4()
	if ip == nil {
		t.Fatal("failed to parse test IP")
	}
	wantAddr := uint32(ip[0]) | uint32(ip[1])<<8 | uint32(ip[2])<<16 | uint32(ip[3])<<24
	if success.Players[0].Addr != wantAddr {
		t.Fatalf("expected advertised addr 0x%08x, got 0x%08x", wantAddr, success.Players[0].Addr)
	}
	if success.Players[0].Port != 13000 {
		t.Fatalf("expected advertised port 13000, got %d", success.Players[0].Port)
	}
	if success.Players[0].Name != "MkmkWar" {
		t.Fatalf("expected first advertised character MkmkWar, got %q", success.Players[0].Name)
	}
	if success.Players[1].Name != "MkmkSura" {
		t.Fatalf("expected second advertised character MkmkSura, got %q", success.Players[1].Name)
	}
}

func TestNewGameSessionFactoryReachesGamePhase(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Characters: stubCharacters()}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flow := factory()
	if _, err := flow.Start(); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}
	_, err = flow.HandleClientFrame(decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	})))
	if err != nil {
		t.Fatalf("unexpected handshake error: %v", err)
	}

	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: StubLogin, LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}
	_, err = flow.HandleClientFrame(decodeSingleFrame(t, login2Raw))
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, frame.Encode(worldproto.HeaderCharacterSelect, []byte{1})))
	if err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if len(selectOut) != 3 {
		t.Fatalf("expected 3 select frames, got %d", len(selectOut))
	}
	wantPhaseLoading, err := control.EncodePhase(session.PhaseLoading)
	if err != nil {
		t.Fatalf("unexpected loading phase encode error: %v", err)
	}
	if !bytes.Equal(selectOut[0], wantPhaseLoading) {
		t.Fatalf("unexpected loading phase frame: got %x want %x", selectOut[0], wantPhaseLoading)
	}
	mainCharacter, err := worldproto.DecodeMainCharacter(decodeSingleFrame(t, selectOut[1]))
	if err != nil {
		t.Fatalf("decode main character: %v", err)
	}
	if mainCharacter.Name != "MkmkSura" {
		t.Fatalf("expected selected character MkmkSura, got %q", mainCharacter.Name)
	}

	enterGameOut, err := flow.HandleClientFrame(decodeSingleFrame(t, frame.Encode(worldproto.HeaderEnterGame, nil)))
	if err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}
	if len(enterGameOut) != 1 {
		t.Fatalf("expected 1 game frame, got %d", len(enterGameOut))
	}
	wantPhaseGame, err := control.EncodePhase(session.PhaseGame)
	if err != nil {
		t.Fatalf("unexpected game phase encode error: %v", err)
	}
	if !bytes.Equal(enterGameOut[0], wantPhaseGame) {
		t.Fatalf("unexpected game phase frame: got %x want %x", enterGameOut[0], wantPhaseGame)
	}
}

func TestNewGameSessionFactoryRejectsInvalidPublicAddr(t *testing.T) {
	_, err := NewGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "not-an-ip"})
	if !errors.Is(err, ErrInvalidPublicAddr) {
		t.Fatalf("expected ErrInvalidPublicAddr, got %v", err)
	}
}

func decodeSingleFrame(t *testing.T, raw []byte) frame.Frame {
	t.Helper()

	decoder := frame.NewDecoder(4096)
	frames, err := decoder.Feed(raw)
	if err != nil {
		t.Fatalf("unexpected frame decode error: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}
	return frames[0]
}
