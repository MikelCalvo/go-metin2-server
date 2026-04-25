package minimal

import (
	"bytes"
	"errors"
	"net"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	authproto "github.com/MikelCalvo/go-metin2-server/internal/proto/auth"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
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

	handshakeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, secureHandshakeResponseFromStartFrames(t, startOut)))
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

func TestLoadOrCreateAccountSeedsMkmkWarInShinsooYongan(t *testing.T) {
	accounts := accountstore.NewFileStore(t.TempDir())
	account, ok := loadOrCreateAccount(accounts, StubLogin)
	if !ok {
		t.Fatal("expected bootstrap account load/create to succeed")
	}
	if len(account.Characters) == 0 {
		t.Fatal("expected seeded bootstrap characters")
	}
	mkmkWar := account.Characters[0]
	if mkmkWar.Name != "MkmkWar" {
		t.Fatalf("expected first bootstrap character MkmkWar, got %+v", mkmkWar)
	}
	if mkmkWar.MapIndex != bootstrapMapIndex || mkmkWar.X != 469300 || mkmkWar.Y != 964200 {
		t.Fatalf("expected MkmkWar to seed at Shinsoo Yongan start map=%d x=%d y=%d, got map=%d x=%d y=%d", bootstrapMapIndex, 469300, 964200, mkmkWar.MapIndex, mkmkWar.X, mkmkWar.Y)
	}
	persisted, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted bootstrap account: %v", err)
	}
	if persisted.Characters[0].MapIndex != bootstrapMapIndex || persisted.Characters[0].X != 469300 || persisted.Characters[0].Y != 964200 {
		t.Fatalf("expected persisted MkmkWar to seed at Shinsoo Yongan start map=%d x=%d y=%d, got map=%d x=%d y=%d", bootstrapMapIndex, 469300, 964200, persisted.Characters[0].MapIndex, persisted.Characters[0].X, persisted.Characters[0].Y)
	}
}

func TestLoadOrCreateAccountMigratesLegacyFakeMkmkWarPositionToShinsooYongan(t *testing.T) {
	accounts := accountstore.NewFileStore(t.TempDir())
	legacy := accountstore.Account{Login: StubLogin, Empire: 2, Characters: legacyFakeStubCharacters()}
	if err := accounts.Save(legacy); err != nil {
		t.Fatalf("save legacy bootstrap account: %v", err)
	}
	account, ok := loadOrCreateAccount(accounts, StubLogin)
	if !ok {
		t.Fatal("expected legacy bootstrap account load to succeed")
	}
	mkmkWar := account.Characters[0]
	if mkmkWar.MapIndex != bootstrapMapIndex || mkmkWar.X != 469300 || mkmkWar.Y != 964200 {
		t.Fatalf("expected migrated MkmkWar at Shinsoo Yongan start map=%d x=%d y=%d, got map=%d x=%d y=%d", bootstrapMapIndex, 469300, 964200, mkmkWar.MapIndex, mkmkWar.X, mkmkWar.Y)
	}
	persisted, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load migrated bootstrap account: %v", err)
	}
	if persisted.Characters[0].MapIndex != bootstrapMapIndex || persisted.Characters[0].X != 469300 || persisted.Characters[0].Y != 964200 {
		t.Fatalf("expected persisted migrated MkmkWar at Shinsoo Yongan start map=%d x=%d y=%d, got map=%d x=%d y=%d", bootstrapMapIndex, 469300, 964200, persisted.Characters[0].MapIndex, persisted.Characters[0].X, persisted.Characters[0].Y)
	}
}

func TestNewGameSessionFactoryExposesSecureLegacyTransportHooks(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Characters: stubCharacters()}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flow := factory()
	hooks, ok := flow.(interface {
		EncryptLegacyOutgoing([]byte) ([]byte, error)
		DecryptLegacyIncoming([]byte) ([]byte, error)
	})
	if !ok {
		t.Fatal("expected game session flow to expose secure legacy transport hooks")
	}
	if _, err := hooks.EncryptLegacyOutgoing([]byte{0x01, 0x02, 0x03}); err != nil {
		t.Fatalf("unexpected encrypt hook error: %v", err)
	}
	if _, err := hooks.DecryptLegacyIncoming([]byte{0x01, 0x02, 0x03}); err != nil {
		t.Fatalf("unexpected decrypt hook error: %v", err)
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
	_ = mustCompleteSecureHandshake(t, flow)

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

	success, err := loginproto.DecodeLoginSuccess4(decodeSingleFrame(t, loginOut[0]))
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
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: stubCharacters()}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flow := factory()
	_ = mustCompleteSecureHandshake(t, flow)

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
	if len(enterGameOut) != 5 {
		t.Fatalf("expected 5 game bootstrap frames, got %d", len(enterGameOut))
	}
	wantPhaseGame, err := control.EncodePhase(session.PhaseGame)
	if err != nil {
		t.Fatalf("unexpected game phase encode error: %v", err)
	}
	if !bytes.Equal(enterGameOut[0], wantPhaseGame) {
		t.Fatalf("unexpected game phase frame: got %x want %x", enterGameOut[0], wantPhaseGame)
	}
	added, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, enterGameOut[1]))
	if err != nil {
		t.Fatalf("decode character add: %v", err)
	}
	if added.VID != 0x01020305 || added.RaceNum != 3 || added.Type != 6 || added.X != 1200 || added.Y != 2100 {
		t.Fatalf("unexpected character add packet: %+v", added)
	}
	info, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, enterGameOut[2]))
	if err != nil {
		t.Fatalf("decode character additional info: %v", err)
	}
	if info.VID != 0x01020305 || info.Name != "MkmkSura" || info.Empire != 2 || info.Parts[0] != 102 || info.Parts[3] != 202 || info.Level != 12 {
		t.Fatalf("unexpected character additional info packet: %+v", info)
	}
	update, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, enterGameOut[3]))
	if err != nil {
		t.Fatalf("decode character update: %v", err)
	}
	if update.VID != 0x01020305 || update.Parts[0] != 102 || update.Parts[3] != 202 || update.MovingSpeed != 150 || update.AttackSpeed != 100 || update.GuildID != 0 {
		t.Fatalf("unexpected character update packet: %+v", update)
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, enterGameOut[4]))
	if err != nil {
		t.Fatalf("decode player point change: %v", err)
	}
	if pointChange.VID != 0x01020305 || pointChange.Type != 1 || pointChange.Amount != 900 || pointChange.Value != 900 {
		t.Fatalf("unexpected player point change packet: %+v", pointChange)
	}
}

func TestNewGameSessionFactoryRespondsToStateCheckerDuringHandshake(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flow := factory()
	startOut, err := flow.Start()
	if err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}
	if len(startOut) != 2 {
		t.Fatalf("expected 2 handshake start frames, got %d", len(startOut))
	}

	phaseHandshake, err := control.EncodePhase(session.PhaseHandshake)
	if err != nil {
		t.Fatalf("unexpected handshake phase encode error: %v", err)
	}
	if got := decodeSingleFrame(t, startOut[0]); got.Header != control.HeaderPhase || !bytes.Equal(startOut[0], phaseHandshake) {
		t.Fatalf("unexpected handshake phase frame: got %x want %x", startOut[0], phaseHandshake)
	}
	if got := decodeSingleFrame(t, startOut[1]); got.Header != control.HeaderKeyChallenge {
		t.Fatalf("expected key challenge header 0x%04x, got 0x%04x", control.HeaderKeyChallenge, got.Header)
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, control.EncodeStateChecker()))
	if err != nil {
		t.Fatalf("unexpected state checker handling error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 state checker frame, got %d", len(out))
	}

	packet, err := control.DecodeRespondChannelStatus(decodeSingleFrame(t, out[0]))
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

	phaseAware, ok := flow.(interface{ CurrentPhase() session.Phase })
	if !ok {
		t.Fatal("expected game session flow to expose CurrentPhase")
	}
	if got := phaseAware.CurrentPhase(); got != session.PhaseHandshake {
		t.Fatalf("expected phase %q, got %q", session.PhaseHandshake, got)
	}
}

func TestCreateCharacterInTicketUsesLegacyEmpireCreatePosition(t *testing.T) {
	tests := []struct {
		name     string
		empire   uint8
		packet   worldproto.CharacterCreatePacket
		mapIndex uint32
		x        int32
		y        int32
	}{
		{name: "shinsoo", empire: 1, packet: worldproto.CharacterCreatePacket{Index: 0, Name: "FreshShinsoo", RaceNum: 0, Shape: 1}, mapIndex: 1, x: 459800, y: 953900},
		{name: "chunjo", empire: 2, packet: worldproto.CharacterCreatePacket{Index: 0, Name: "FreshChunjo", RaceNum: 0, Shape: 1}, mapIndex: 21, x: 52070, y: 166600},
		{name: "jinno", empire: 3, packet: worldproto.CharacterCreatePacket{Index: 0, Name: "FreshJinno", RaceNum: 0, Shape: 1}, mapIndex: 41, x: 957300, y: 255200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := &loginticket.Ticket{Empire: tt.empire}
			created, failureType, ok := createCharacterInTicket(ticket, tt.packet, tt.empire)
			if !ok || failureType != 0 {
				t.Fatalf("expected createCharacterInTicket to succeed, ok=%v failureType=%d", ok, failureType)
			}
			if created.MapIndex != tt.mapIndex || created.X != tt.x || created.Y != tt.y {
				t.Fatalf("expected legacy create position map=%d x=%d y=%d, got map=%d x=%d y=%d", tt.mapIndex, tt.x, tt.y, created.MapIndex, created.X, created.Y)
			}
		})
	}
}

func TestNewGameSessionFactoryCreatesACharacterInAnEmptySlot(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: stubCharacters()}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}

	factory, err := newGameSessionFactoryWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flow := factory()
	_ = mustCompleteSecureHandshake(t, flow)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: StubLogin, LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, login2Raw)); err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}

	createRaw, err := worldproto.EncodeCharacterCreate(worldproto.CharacterCreatePacket{Index: 2, Name: "FreshSura", RaceNum: 2, Shape: 1})
	if err != nil {
		t.Fatalf("encode character create: %v", err)
	}
	createOut, err := flow.HandleClientFrame(decodeSingleFrame(t, createRaw))
	if err != nil {
		t.Fatalf("unexpected character create error: %v", err)
	}
	if len(createOut) != 1 {
		t.Fatalf("expected 1 create frame, got %d", len(createOut))
	}
	created, err := worldproto.DecodePlayerCreateSuccess(decodeSingleFrame(t, createOut[0]))
	if err != nil {
		t.Fatalf("decode player create success: %v", err)
	}
	if created.Index != 2 || created.Player.Name != "FreshSura" || created.Player.Job != 2 || created.Player.Level != 1 {
		t.Fatalf("unexpected created player packet: %+v", created)
	}
	if created.Player.X != 52070 || created.Player.Y != 166600 {
		t.Fatalf("expected created player legacy Chunjo create position x=52070 y=166600, got x=%d y=%d", created.Player.X, created.Player.Y)
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 2})))
	if err != nil {
		t.Fatalf("unexpected select after create error: %v", err)
	}
	mainCharacter, err := worldproto.DecodeMainCharacter(decodeSingleFrame(t, selectOut[1]))
	if err != nil {
		t.Fatalf("decode main character: %v", err)
	}
	if mainCharacter.Name != "FreshSura" || mainCharacter.RaceNum != 2 {
		t.Fatalf("unexpected created main character: %+v", mainCharacter)
	}
	if mainCharacter.X != 52070 || mainCharacter.Y != 166600 {
		t.Fatalf("expected selected created character legacy Chunjo create position x=52070 y=166600, got x=%d y=%d", mainCharacter.X, mainCharacter.Y)
	}

	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account after create: %v", err)
	}
	if account.Characters[2].Name != "FreshSura" {
		t.Fatalf("expected created character in persisted slot 2, got %+v", account.Characters[2])
	}
	if account.Characters[2].MapIndex != 21 || account.Characters[2].X != 52070 || account.Characters[2].Y != 166600 {
		t.Fatalf("expected created character legacy Chunjo position map=21 x=52070 y=166600, got map=%d x=%d y=%d", account.Characters[2].MapIndex, account.Characters[2].X, account.Characters[2].Y)
	}
}

func TestNewGameSessionFactoryDeletesCharacterAndPersistsTheEmptySlot(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: stubCharacters()}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}

	factory, err := newGameSessionFactoryWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flow := factory()
	_ = mustCompleteSecureHandshake(t, flow)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: StubLogin, LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, login2Raw)); err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}

	deleteRaw, err := worldproto.EncodeCharacterDelete(worldproto.CharacterDeletePacket{Index: 1, PrivateCode: "1234567"})
	if err != nil {
		t.Fatalf("encode character delete: %v", err)
	}
	deleteOut, err := flow.HandleClientFrame(decodeSingleFrame(t, deleteRaw))
	if err != nil {
		t.Fatalf("unexpected character delete error: %v", err)
	}
	if len(deleteOut) != 1 {
		t.Fatalf("expected 1 delete frame, got %d", len(deleteOut))
	}
	deleted, err := worldproto.DecodePlayerDeleteSuccess(decodeSingleFrame(t, deleteOut[0]))
	if err != nil {
		t.Fatalf("decode player delete success: %v", err)
	}
	if deleted.Index != 1 {
		t.Fatalf("unexpected delete success index: got %d want %d", deleted.Index, 1)
	}

	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if account.Characters[1] != (loginticket.Character{}) {
		t.Fatalf("expected deleted slot to be empty, got %+v", account.Characters[1])
	}
	if account.Characters[0].Name != "MkmkWar" {
		t.Fatalf("expected other slot to stay intact, got %+v", account.Characters[0])
	}
}

func TestNewGameSessionFactoryReturnsVisibleWorldBootstrapForCreatedCharacter(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: stubCharacters()}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flow := factory()
	_ = mustCompleteSecureHandshake(t, flow)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: StubLogin, LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, login2Raw)); err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}
	createRaw, err := worldproto.EncodeCharacterCreate(worldproto.CharacterCreatePacket{Index: 2, Name: "FreshSura", RaceNum: 2, Shape: 1})
	if err != nil {
		t.Fatalf("encode character create: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, createRaw)); err != nil {
		t.Fatalf("unexpected character create error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 2}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}

	enterGameOut, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame()))
	if err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}
	if len(enterGameOut) != 5 {
		t.Fatalf("expected 5 game bootstrap frames, got %d", len(enterGameOut))
	}
	added, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, enterGameOut[1]))
	if err != nil {
		t.Fatalf("decode character add: %v", err)
	}
	if added.VID != 0x01020306 || added.RaceNum != 2 || added.Type != 6 || added.X != 52070 || added.Y != 166600 {
		t.Fatalf("unexpected created character add packet: %+v", added)
	}
	info, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, enterGameOut[2]))
	if err != nil {
		t.Fatalf("decode character additional info: %v", err)
	}
	if info.VID != 0x01020306 || info.Name != "FreshSura" || info.Empire != 2 || info.Parts[0] != 1 || info.Parts[3] != 0 || info.Level != 1 {
		t.Fatalf("unexpected created character additional info packet: %+v", info)
	}
	update, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, enterGameOut[3]))
	if err != nil {
		t.Fatalf("decode character update: %v", err)
	}
	if update.VID != 0x01020306 || update.Parts[0] != 1 || update.Parts[3] != 0 || update.MovingSpeed != 150 || update.AttackSpeed != 100 || update.GuildID != 0 {
		t.Fatalf("unexpected created character update packet: %+v", update)
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, enterGameOut[4]))
	if err != nil {
		t.Fatalf("decode player point change: %v", err)
	}
	if pointChange.VID != 0x01020306 || pointChange.Type != 1 || pointChange.Amount != 650 || pointChange.Value != 650 {
		t.Fatalf("unexpected created player point change packet: %+v", pointChange)
	}
}

func TestNewGameSessionFactoryMovesTheSelectedCharacterInGame(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Characters: stubCharacters()}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flow := factory()
	_ = mustCompleteSecureHandshake(t, flow)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: StubLogin, LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, login2Raw)); err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(sampleMovePacket())))
	if err != nil {
		t.Fatalf("unexpected move error: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 move frame, got %d", len(moveOut))
	}
	ack, err := movep.DecodeMoveAck(decodeSingleFrame(t, moveOut[0]))
	if err != nil {
		t.Fatalf("decode move ack: %v", err)
	}
	if ack.VID != 0x01020305 || ack.Func != 1 || ack.Rot != 12 || ack.X != 12345 || ack.Y != 23456 || ack.Time != 0x01020304 || ack.Duration != 250 {
		t.Fatalf("unexpected move ack: %+v", ack)
	}
}

func TestNewGameSessionFactorySynchronizesTheSelectedCharacterInGame(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Characters: stubCharacters()}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flow := factory()
	_ = mustCompleteSecureHandshake(t, flow)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: StubLogin, LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, login2Raw)); err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	syncOut, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeSyncPosition(sampleSelectedSyncPositionPacket())))
	if err != nil {
		t.Fatalf("unexpected sync position error: %v", err)
	}
	if len(syncOut) != 1 {
		t.Fatalf("expected 1 sync frame, got %d", len(syncOut))
	}
	ack, err := movep.DecodeSyncPositionAck(decodeSingleFrame(t, syncOut[0]))
	if err != nil {
		t.Fatalf("decode sync position ack: %v", err)
	}
	if len(ack.Elements) != 1 {
		t.Fatalf("expected 1 sync ack element, got %d", len(ack.Elements))
	}
	if ack.Elements[0].VID != 0x01020305 || ack.Elements[0].X != 1400 || ack.Elements[0].Y != 2500 {
		t.Fatalf("unexpected sync position ack: %+v", ack.Elements[0])
	}
}

func TestNewGameSessionFactoryMovesTheCreatedCharacterInGame(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Characters: stubCharacters()}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flow := factory()
	_ = mustCompleteSecureHandshake(t, flow)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: StubLogin, LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, login2Raw)); err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}
	createRaw, err := worldproto.EncodeCharacterCreate(worldproto.CharacterCreatePacket{Index: 2, Name: "FreshSura", RaceNum: 2, Shape: 1})
	if err != nil {
		t.Fatalf("encode character create: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, createRaw)); err != nil {
		t.Fatalf("unexpected character create error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 2}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(sampleMovePacket())))
	if err != nil {
		t.Fatalf("unexpected move error: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 move frame, got %d", len(moveOut))
	}
	ack, err := movep.DecodeMoveAck(decodeSingleFrame(t, moveOut[0]))
	if err != nil {
		t.Fatalf("decode move ack: %v", err)
	}
	if ack.VID != 0x01020306 || ack.Func != 1 || ack.Rot != 12 || ack.X != 12345 || ack.Y != 23456 || ack.Time != 0x01020304 || ack.Duration != 250 {
		t.Fatalf("unexpected move ack: %+v", ack)
	}
}

func TestUpdateSelectedCharacterPositionDoesNotMutateOnSaveFailure(t *testing.T) {
	characters := stubCharacters()
	original := characters[1]
	updated, selected, ok := updateSelectedCharacterPosition(&failingAccountStore{}, StubLogin, 2, characters, 1, 1400, 2500)
	if ok {
		t.Fatal("expected position update to fail when account store save fails")
	}
	if updated != nil {
		t.Fatalf("expected no updated character slice on failure, got %+v", updated)
	}
	if selected != (loginticket.Character{}) {
		t.Fatalf("expected zero selected character on failure, got %+v", selected)
	}
	if characters[1].X != original.X || characters[1].Y != original.Y {
		t.Fatalf("expected original character position to stay (%d,%d), got (%d,%d)", original.X, original.Y, characters[1].X, characters[1].Y)
	}
}

func TestUpdateSelectedCharacterPositionReturnsPersistedCloneOnSuccess(t *testing.T) {
	store := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	updated, selected, ok := updateSelectedCharacterPosition(store, StubLogin, 2, characters, 1, 1400, 2500)
	if !ok {
		t.Fatal("expected position update to succeed")
	}
	if selected.VID != 0x01020305 || selected.X != 1400 || selected.Y != 2500 {
		t.Fatalf("unexpected updated selected character: %+v", selected)
	}
	if updated[1].X != 1400 || updated[1].Y != 2500 {
		t.Fatalf("expected updated clone position (1400,2500), got (%d,%d)", updated[1].X, updated[1].Y)
	}
	if characters[1].X != 1200 || characters[1].Y != 2100 {
		t.Fatalf("expected original slice to remain unchanged, got (%d,%d)", characters[1].X, characters[1].Y)
	}
	account, err := store.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if account.Characters[1].X != 1400 || account.Characters[1].Y != 2500 {
		t.Fatalf("expected persisted position (1400,2500), got (%d,%d)", account.Characters[1].X, account.Characters[1].Y)
	}
}

func TestUpdateSelectedCharacterLocationDoesNotMutateOnSaveFailure(t *testing.T) {
	characters := stubCharacters()
	original := characters[1]
	updated, selected, ok := updateSelectedCharacterLocation(&failingAccountStore{}, StubLogin, 2, characters, 1, 42, 1700, 2800)
	if ok {
		t.Fatal("expected location update to fail when account store save fails")
	}
	if updated != nil {
		t.Fatalf("expected no updated character slice on failure, got %+v", updated)
	}
	if selected != (loginticket.Character{}) {
		t.Fatalf("expected zero selected character on failure, got %+v", selected)
	}
	if characters[1].MapIndex != original.MapIndex || characters[1].X != original.X || characters[1].Y != original.Y {
		t.Fatalf("expected original character location to stay map=%d x=%d y=%d, got map=%d x=%d y=%d", original.MapIndex, original.X, original.Y, characters[1].MapIndex, characters[1].X, characters[1].Y)
	}
}

func TestUpdateSelectedCharacterLocationReturnsPersistedCloneOnSuccess(t *testing.T) {
	store := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	updated, selected, ok := updateSelectedCharacterLocation(store, StubLogin, 2, characters, 1, 42, 1700, 2800)
	if !ok {
		t.Fatal("expected location update to succeed")
	}
	if selected.VID != 0x01020305 || selected.MapIndex != 42 || selected.X != 1700 || selected.Y != 2800 {
		t.Fatalf("unexpected updated selected character: %+v", selected)
	}
	if updated[1].MapIndex != 42 || updated[1].X != 1700 || updated[1].Y != 2800 {
		t.Fatalf("expected updated clone location map=42 x=1700 y=2800, got map=%d x=%d y=%d", updated[1].MapIndex, updated[1].X, updated[1].Y)
	}
	if characters[1].MapIndex != bootstrapMapIndex || characters[1].X != 1200 || characters[1].Y != 2100 {
		t.Fatalf("expected original slice to remain unchanged, got map=%d x=%d y=%d", characters[1].MapIndex, characters[1].X, characters[1].Y)
	}
	account, err := store.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if account.Characters[1].MapIndex != 42 || account.Characters[1].X != 1700 || account.Characters[1].Y != 2800 {
		t.Fatalf("expected persisted location map=42 x=1700 y=2800, got map=%d x=%d y=%d", account.Characters[1].MapIndex, account.Characters[1].X, account.Characters[1].Y)
	}
}

func TestNewGameSessionFactoryRejectsInvalidPublicAddr(t *testing.T) {
	_, err := NewGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "not-an-ip"})
	if !errors.Is(err, ErrInvalidPublicAddr) {
		t.Fatalf("expected ErrInvalidPublicAddr, got %v", err)
	}
}

func TestNewGameRuntimeUsesConfiguredRadiusVisibilityPolicy(t *testing.T) {
	runtime, err := newGameRuntimeWithAccountStore(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     450,
		VisibilitySectorSize: 225,
	}, loginticket.NewFileStore(t.TempDir()), nil)
	if err != nil {
		t.Fatalf("expected configured runtime creation to succeed, got %v", err)
	}
	policy, ok := runtime.sharedWorld.topology.VisibilityPolicy().(worldruntime.RadiusVisibilityPolicy)
	if !ok {
		t.Fatalf("expected radius visibility policy, got %T", runtime.sharedWorld.topology.VisibilityPolicy())
	}
	if policy.Radius != 450 || policy.SectorSize != 225 {
		t.Fatalf("unexpected radius visibility policy config: %+v", policy)
	}
	if snapshot := runtime.RuntimeConfigSnapshot(); snapshot.VisibilityMode != "radius" || snapshot.VisibilityRadius != 450 || snapshot.VisibilitySectorSize != 225 || snapshot.LocalChannelID != 1 {
		t.Fatalf("unexpected runtime config snapshot: %+v", snapshot)
	}
}

func TestNewGameRuntimeUpdateStaticActorRejectsInvalidSeed(t *testing.T) {
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	guard, ok := runtime.RegisterStaticActor("VillageGuard", 42, 1700, 2800, 20300)
	if !ok {
		t.Fatal("expected guard registration to succeed")
	}
	if _, ok := runtime.UpdateStaticActor(guard.EntityID, "", 42, 1700, 2800, 20300); ok {
		t.Fatal("expected blank-name static actor update to fail")
	}
	if _, ok := runtime.UpdateStaticActor(guard.EntityID, "VillageGuard", 0, 1700, 2800, 20300); ok {
		t.Fatal("expected zero-map static actor update to fail")
	}
	if _, ok := runtime.UpdateStaticActor(guard.EntityID, "VillageGuard", 42, 1700, 2800, 0); ok {
		t.Fatal("expected zero-race static actor update to fail")
	}
}

func TestNewGameSessionFactoryRejectsUnknownVisibilityMode(t *testing.T) {
	_, err := NewGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", VisibilityMode: "cones"})
	if !errors.Is(err, ErrInvalidVisibilityMode) {
		t.Fatalf("expected ErrInvalidVisibilityMode, got %v", err)
	}
}

func TestNewGameSessionFactoryRejectsInvalidRadiusVisibilityConfig(t *testing.T) {
	_, err := NewGameSessionFactory(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     0,
		VisibilitySectorSize: 225,
	})
	if !errors.Is(err, ErrInvalidVisibilityRadius) {
		t.Fatalf("expected ErrInvalidVisibilityRadius, got %v", err)
	}

	_, err = NewGameSessionFactory(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     450,
		VisibilitySectorSize: 0,
	})
	if !errors.Is(err, ErrInvalidVisibilitySectorSize) {
		t.Fatalf("expected ErrInvalidVisibilitySectorSize, got %v", err)
	}
}

type failingAccountStore struct{}

func (f *failingAccountStore) Load(string) (accountstore.Account, error) {
	return accountstore.Account{}, accountstore.ErrAccountNotFound
}

func (f *failingAccountStore) Save(accountstore.Account) error {
	return errors.New("save failed")
}

type preloadedFailingAccountStore struct {
	accounts map[string]accountstore.Account
}

func newPreloadedFailingAccountStore(accounts ...accountstore.Account) *preloadedFailingAccountStore {
	cloned := make(map[string]accountstore.Account, len(accounts))
	for _, account := range accounts {
		copyAccount := account
		copyAccount.Characters = cloneCharacters(account.Characters)
		cloned[account.Login] = copyAccount
	}
	return &preloadedFailingAccountStore{accounts: cloned}
}

func (f *preloadedFailingAccountStore) Load(login string) (accountstore.Account, error) {
	account, ok := f.accounts[login]
	if !ok {
		return accountstore.Account{}, accountstore.ErrAccountNotFound
	}
	account.Characters = cloneCharacters(account.Characters)
	return account, nil
}

func (f *preloadedFailingAccountStore) Save(accountstore.Account) error {
	return errors.New("save failed")
}

func sampleMovePacket() movep.MovePacket {
	return movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 12345, Y: 23456, Time: 0x01020304}
}

func sampleSelectedSyncPositionPacket() movep.SyncPositionPacket {
	return movep.SyncPositionPacket{Elements: []movep.SyncPositionElement{{VID: 0x01020305, X: 1400, Y: 2500}}}
}

func legacyFakeStubCharacters() []loginticket.Character {
	characters := stubCharacters()
	if len(characters) > 0 {
		characters[0].X = 1000
		characters[0].Y = 2000
	}
	return characters
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
