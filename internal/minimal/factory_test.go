package minimal

import (
	"bytes"
	"errors"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/player"
	authproto "github.com/MikelCalvo/go-metin2-server/internal/proto/auth"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

type staticItemTemplateStore struct {
	snapshot itemcatalog.Snapshot
}

func (s staticItemTemplateStore) Load() (itemcatalog.Snapshot, error) {
	return s.snapshot, nil
}

func (s staticItemTemplateStore) Save(itemcatalog.Snapshot) error {
	return nil
}

func newStartedGameFlowWithItemStore(t *testing.T, store loginticket.Store, accounts accountstore.Store, itemTemplates itemcatalog.Store) service.SessionFlow {
	t.Helper()
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, nil, itemTemplates)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	flow := runtime.SessionFactory()()
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
	return flow
}

func TestBootstrapTopologyFromConfigDefaultsToWholeMapVisibility(t *testing.T) {
	topology, err := bootstrapTopologyFromConfig(config.Service{})
	if err != nil {
		t.Fatalf("unexpected topology error: %v", err)
	}
	if _, ok := topology.VisibilityPolicy().(worldruntime.WholeMapVisibilityPolicy); !ok {
		t.Fatalf("expected whole-map visibility policy by default, got %T", topology.VisibilityPolicy())
	}
}

func TestBootstrapTopologyFromConfigNormalizesRadiusModeAndRejectsInvalidValues(t *testing.T) {
	topology, err := bootstrapTopologyFromConfig(config.Service{VisibilityMode: " Radius ", VisibilityRadius: 400, VisibilitySectorSize: 200})
	if err != nil {
		t.Fatalf("unexpected topology error: %v", err)
	}
	policy, ok := topology.VisibilityPolicy().(worldruntime.RadiusVisibilityPolicy)
	if !ok {
		t.Fatalf("expected radius visibility policy, got %T", topology.VisibilityPolicy())
	}
	if policy.Radius != 400 || policy.SectorSize != 200 {
		t.Fatalf("unexpected radius policy: %+v", policy)
	}

	if _, err := bootstrapTopologyFromConfig(config.Service{VisibilityMode: "radius", VisibilityRadius: 0, VisibilitySectorSize: 200}); !errors.Is(err, ErrInvalidVisibilityRadius) {
		t.Fatalf("expected ErrInvalidVisibilityRadius, got %v", err)
	}
	if _, err := bootstrapTopologyFromConfig(config.Service{VisibilityMode: "radius", VisibilityRadius: 400, VisibilitySectorSize: 0}); !errors.Is(err, ErrInvalidVisibilitySectorSize) {
		t.Fatalf("expected ErrInvalidVisibilitySectorSize, got %v", err)
	}
	if _, err := bootstrapTopologyFromConfig(config.Service{VisibilityMode: "sector"}); !errors.Is(err, ErrInvalidVisibilityMode) {
		t.Fatalf("expected ErrInvalidVisibilityMode, got %v", err)
	}
}

func TestGameRuntimeConfigSnapshotReportsRadiusPolicy(t *testing.T) {
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", VisibilityMode: "radius", VisibilityRadius: 400, VisibilitySectorSize: 200},
		loginticket.NewFileStore(t.TempDir()),
		nil,
		nil,
		nil,
		staticItemTemplateStore{},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	snapshot := runtime.RuntimeConfigSnapshot()
	if snapshot.LocalChannelID != 1 || snapshot.VisibilityMode != "radius" || snapshot.VisibilityRadius != 400 || snapshot.VisibilitySectorSize != 200 {
		t.Fatalf("unexpected runtime config snapshot: %+v", snapshot)
	}
}

func TestSlashGameCommandRejectsArgumentsForOwnedRestartAndLeaveCommands(t *testing.T) {
	for _, message := range []string{
		"/quit now",
		"/logout later",
		"/phase_select 1",
		"/restart_here please",
		"/restart_town 2",
	} {
		t.Run(message, func(t *testing.T) {
			if command, ok := slashGameCommand(message); ok {
				t.Fatalf("expected %q with arguments to stay outside owned slash-command ingress, got command %q", message, command)
			}
		})
	}
}

func TestSlashGameCommandAcceptsExactOwnedRestartAndLeaveCommands(t *testing.T) {
	for _, message := range []string{
		"/quit",
		"/logout",
		"/phase_select",
		"/restart_here",
		"/restart_town",
	} {
		t.Run(message, func(t *testing.T) {
			command, ok := slashGameCommand(message)
			if !ok || command != strings.TrimPrefix(message, "/") {
				t.Fatalf("expected exact command %q to parse, got command=%q ok=%v", message, command, ok)
			}
		})
	}
}

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

func TestNewGameSessionFactoryProjectsEquippedAppearanceIntoBootstrapFrames(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Equipment = []inventory.ItemInstance{
		{ID: 71, Vnum: 11500, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotBody},
		{ID: 72, Vnum: 11200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
		{ID: 73, Vnum: 50053, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotHead},
	}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
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
	if _, err = flow.HandleClientFrame(decodeSingleFrame(t, login2Raw)); err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}
	if _, err = flow.HandleClientFrame(decodeSingleFrame(t, frame.Encode(worldproto.HeaderCharacterSelect, []byte{1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}

	enterGameOut, err := flow.HandleClientFrame(decodeSingleFrame(t, frame.Encode(worldproto.HeaderEnterGame, nil)))
	if err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}
	if len(enterGameOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with equipped items, got %d", len(enterGameOut))
	}

	info, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, enterGameOut[2]))
	if err != nil {
		t.Fatalf("decode character additional info: %v", err)
	}
	if info.Parts != [worldproto.CharacterEquipmentPartCount]uint16{11500, 11200, 50053, 202} {
		t.Fatalf("unexpected projected additional-info parts: %+v", info.Parts)
	}

	update, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, enterGameOut[3]))
	if err != nil {
		t.Fatalf("decode character update: %v", err)
	}
	if update.Parts != [worldproto.CharacterEquipmentPartCount]uint16{11500, 11200, 50053, 202} {
		t.Fatalf("unexpected projected update parts: %+v", update.Parts)
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
	if account.Characters[1].ID != 0 || account.Characters[1].VID != 0 || account.Characters[1].Name != "" {
		t.Fatalf("expected deleted slot identity fields to be empty, got %+v", account.Characters[1])
	}
	if len(account.Characters[1].Inventory) != 0 || len(account.Characters[1].Equipment) != 0 {
		t.Fatalf("expected deleted slot item state to normalize to empty slices, got %+v", account.Characters[1])
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

func TestNewGameSessionFactoryReturnsItemBootstrapForSelectedCharacter(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 0x11223344, Count: 3, Slot: 7}}
	characters[1].Equipment = []inventory.ItemInstance{{ID: 2002, Vnum: 0x55667788, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotShield}}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
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

	enterGameOut, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame()))
	if err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}
	if len(enterGameOut) != 7 {
		t.Fatalf("expected 7 game bootstrap frames, got %d", len(enterGameOut))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, enterGameOut[4]))
	if err != nil {
		t.Fatalf("decode player point change: %v", err)
	}
	if pointChange.VID != 0x01020305 || pointChange.Type != 1 || pointChange.Amount != 900 || pointChange.Value != 900 {
		t.Fatalf("unexpected selected player point change packet: %+v", pointChange)
	}
	inventorySet, err := itemproto.DecodeSet(decodeSingleFrame(t, enterGameOut[5]))
	if err != nil {
		t.Fatalf("decode inventory item bootstrap: %v", err)
	}
	if inventorySet.Position.WindowType != itemproto.WindowInventory || inventorySet.Position.Cell != 7 || inventorySet.Vnum != 0x11223344 || inventorySet.Count != 3 {
		t.Fatalf("unexpected inventory item bootstrap packet: %+v", inventorySet)
	}
	equipmentSet, err := itemproto.DecodeSet(decodeSingleFrame(t, enterGameOut[6]))
	if err != nil {
		t.Fatalf("decode equipment item bootstrap: %v", err)
	}
	if equipmentSet.Position.WindowType != itemproto.WindowInventory || equipmentSet.Position.Cell != itemproto.InventoryMaxCell+10 || equipmentSet.Vnum != 0x55667788 || equipmentSet.Count != 1 {
		t.Fatalf("unexpected equipment item bootstrap packet: %+v", equipmentSet)
	}
}

func TestNewGameSessionFactoryReturnsQuickslotBootstrapForSelectedCharacter(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Quickslots = []loginticket.Quickslot{
		{Position: 5, Type: quickslotproto.TypeItem, Slot: 7},
		{Position: 2, Type: quickslotproto.TypeSkill, Slot: 1},
	}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
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

	enterGameOut, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame()))
	if err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}
	if len(enterGameOut) != 7 {
		t.Fatalf("expected 7 game bootstrap frames with two quickslots, got %d", len(enterGameOut))
	}
	first, err := quickslotproto.DecodeAdd(decodeSingleFrame(t, enterGameOut[5]))
	if err != nil {
		t.Fatalf("decode first quickslot bootstrap: %v", err)
	}
	if first.Position != 2 || first.Slot.Type != quickslotproto.TypeSkill || first.Slot.Position != 1 {
		t.Fatalf("unexpected first quickslot bootstrap: %+v", first)
	}
	second, err := quickslotproto.DecodeAdd(decodeSingleFrame(t, enterGameOut[6]))
	if err != nil {
		t.Fatalf("decode second quickslot bootstrap: %v", err)
	}
	if second.Position != 5 || second.Slot.Type != quickslotproto.TypeItem || second.Slot.Position != 7 {
		t.Fatalf("unexpected second quickslot bootstrap: %+v", second)
	}
}

func TestNewGameSessionFactoryQuickslotAddPersistsAndEmitsAdd(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = append(characters[1].Inventory, inventory.ItemInstance{ID: 1001, Vnum: 0x11223344, Count: 1, Slot: 7})
	characters[1].Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeSkill, Slot: 1}}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
	}

	flow := newStartedGameFlow(t, store, accounts)
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, quickslotproto.EncodeClientAdd(quickslotproto.ClientAddPacket{Position: 5, Slot: quickslotproto.Slot{Type: quickslotproto.TypeItem, Position: 7}})))
	if err != nil {
		t.Fatalf("unexpected quickslot add error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected one quickslot add frame, got %d", len(out))
	}
	add, err := quickslotproto.DecodeAdd(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode quickslot add response: %v", err)
	}
	if add.Position != 5 || add.Slot.Type != quickslotproto.TypeItem || add.Slot.Position != 7 {
		t.Fatalf("unexpected quickslot add response: %+v", add)
	}

	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load account after quickslot add: %v", err)
	}
	if got := account.Characters[1].Quickslots; !reflect.DeepEqual(got, []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeSkill, Slot: 1},
		{Position: 5, Type: quickslotproto.TypeItem, Slot: 7},
	}) {
		t.Fatalf("unexpected persisted quickslots after add: %#v", got)
	}
}

func TestNewGameSessionFactoryQuickslotAddRejectsItemSlotWithoutInventoryItem(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 0x11223344, Count: 3, Slot: 5}}
	characters[1].Equipment = nil
	characters[1].Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeSkill, Slot: 1}}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
	}

	flow := newStartedGameFlow(t, store, accounts)
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, quickslotproto.EncodeClientAdd(quickslotproto.ClientAddPacket{Position: 5, Slot: quickslotproto.Slot{Type: quickslotproto.TypeItem, Position: 7}})))
	if err != nil {
		t.Fatalf("unexpected rejected quickslot add error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected missing-item quickslot add to emit no frames, got %d", len(out))
	}

	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load account after rejected missing-item quickslot add: %v", err)
	}
	if got := account.Characters[1].Quickslots; !reflect.DeepEqual(got, characters[1].Quickslots) {
		t.Fatalf("expected missing-item quickslot add to preserve snapshot, got %#v", got)
	}
}

func TestNewGameSessionFactoryQuickslotDelAndSwapPersistAndEmitFrames(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeSkill, Slot: 1},
		{Position: 5, Type: quickslotproto.TypeItem, Slot: 7},
	}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
	}

	flow := newStartedGameFlow(t, store, accounts)
	deleteOut, err := flow.HandleClientFrame(decodeSingleFrame(t, quickslotproto.EncodeClientDel(quickslotproto.ClientDelPacket{Position: 2})))
	if err != nil {
		t.Fatalf("unexpected quickslot del error: %v", err)
	}
	if len(deleteOut) != 1 {
		t.Fatalf("expected one quickslot del frame, got %d", len(deleteOut))
	}
	deleted, err := quickslotproto.DecodeDel(decodeSingleFrame(t, deleteOut[0]))
	if err != nil {
		t.Fatalf("decode quickslot del response: %v", err)
	}
	if deleted.Position != 2 {
		t.Fatalf("unexpected quickslot del response: %+v", deleted)
	}

	swapOut, err := flow.HandleClientFrame(decodeSingleFrame(t, quickslotproto.EncodeClientSwap(quickslotproto.ClientSwapPacket{Position: 5, TargetPosition: 7})))
	if err != nil {
		t.Fatalf("unexpected quickslot swap error: %v", err)
	}
	if len(swapOut) != 1 {
		t.Fatalf("expected one quickslot swap frame, got %d", len(swapOut))
	}
	swapped, err := quickslotproto.DecodeSwap(decodeSingleFrame(t, swapOut[0]))
	if err != nil {
		t.Fatalf("decode quickslot swap response: %v", err)
	}
	if swapped.Position != 5 || swapped.TargetPosition != 7 {
		t.Fatalf("unexpected quickslot swap response: %+v", swapped)
	}

	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load account after quickslot mutations: %v", err)
	}
	if got := account.Characters[1].Quickslots; !reflect.DeepEqual(got, []loginticket.Quickslot{{Position: 7, Type: quickslotproto.TypeItem, Slot: 7}}) {
		t.Fatalf("unexpected persisted quickslots after del+swap: %#v", got)
	}
}

func TestNewGameSessionFactoryQuickslotAddRejectsInvalidInputWithoutPersisting(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeSkill, Slot: 1}}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
	}

	flow := newStartedGameFlow(t, store, accounts)
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, quickslotproto.EncodeClientAdd(quickslotproto.ClientAddPacket{Position: 36, Slot: quickslotproto.Slot{Type: quickslotproto.TypeItem, Position: 7}})))
	if err != nil {
		t.Fatalf("unexpected rejected quickslot add error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected invalid quickslot add to emit no frames, got %d", len(out))
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load account after rejected quickslot add: %v", err)
	}
	if got := account.Characters[1].Quickslots; !reflect.DeepEqual(got, characters[1].Quickslots) {
		t.Fatalf("expected rejected quickslot add to preserve snapshot, got %#v", got)
	}
}

func TestNewGameSessionFactoryItemMoveSyncsMatchingItemQuickslot(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 0x11223344, Count: 3, Slot: 5}}
	characters[1].Equipment = []inventory.ItemInstance{}
	characters[1].Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
	}

	flow := newStartedGameFlow(t, store, accounts)
	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{Source: itemproto.InventoryPosition(5), Destination: itemproto.InventoryPosition(6), Count: 0})))
	if err != nil {
		t.Fatalf("unexpected item move error: %v", err)
	}
	if len(moveOut) != 3 {
		t.Fatalf("expected delete+set+quickslot add frames for item move, got %d", len(moveOut))
	}
	quickslotRefresh, err := quickslotproto.DecodeAdd(decodeSingleFrame(t, moveOut[2]))
	if err != nil {
		t.Fatalf("decode quickslot refresh after item move: %v", err)
	}
	if quickslotRefresh.Position != 2 || quickslotRefresh.Slot.Type != quickslotproto.TypeItem || quickslotRefresh.Slot.Position != 6 {
		t.Fatalf("unexpected quickslot refresh after item move: %+v", quickslotRefresh)
	}

	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load account after item move quickslot sync: %v", err)
	}
	if got := account.Characters[1].Quickslots; !reflect.DeepEqual(got, []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}) {
		t.Fatalf("unexpected persisted quickslots after item move sync: %#v", got)
	}
}

func newStartedGameFlow(t *testing.T, store loginticket.Store, accounts accountstore.Store) service.SessionFlow {
	t.Helper()
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}
	return flow
}

func TestNewGameSessionFactoryInventoryMovePersistsAndEmitsDeleteThenSet(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 0x11223344, Count: 3, Slot: 5}}
	characters[1].Equipment = []inventory.ItemInstance{}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/inventory_move 5 6"})))
	if err != nil {
		t.Fatalf("unexpected inventory move error: %v", err)
	}
	if len(moveOut) != 2 {
		t.Fatalf("expected delete+set frames for inventory move, got %d", len(moveOut))
	}
	delPacket, err := itemproto.DecodeDel(decodeSingleFrame(t, moveOut[0]))
	if err != nil {
		t.Fatalf("decode inventory move delete: %v", err)
	}
	if delPacket.Position.WindowType != itemproto.WindowInventory || delPacket.Position.Cell != 5 {
		t.Fatalf("unexpected inventory move delete packet: %+v", delPacket)
	}
	setPacket, err := itemproto.DecodeSet(decodeSingleFrame(t, moveOut[1]))
	if err != nil {
		t.Fatalf("decode inventory move set: %v", err)
	}
	if setPacket.Position.WindowType != itemproto.WindowInventory || setPacket.Position.Cell != 6 || setPacket.Vnum != 0x11223344 || setPacket.Count != 3 {
		t.Fatalf("unexpected inventory move set packet: %+v", setPacket)
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[1].Inventory, []inventory.ItemInstance{{ID: 1001, Vnum: 0x11223344, Count: 3, Slot: 6}}) {
		t.Fatalf("unexpected persisted inventory after move: %#v", account.Characters[1].Inventory)
	}
}

func TestNewGameSessionFactoryItemMovePacketSwapsIncompatibleOccupiedDestinationWithoutCount(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{
		{ID: 1001, Vnum: 0x11223344, Count: 3, Slot: 5},
		{ID: 2002, Vnum: 0x55667788, Count: 1, Slot: 8},
	}
	characters[1].Equipment = []inventory.ItemInstance{}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(8),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected occupied-destination item move packet error: %v", err)
	}
	if len(moveOut) != 2 {
		t.Fatalf("expected two item-set frames for incompatible occupied-destination item move swap without count, got %d", len(moveOut))
	}
	fromPacket, err := itemproto.DecodeSet(decodeSingleFrame(t, moveOut[0]))
	if err != nil {
		t.Fatalf("decode item-move swap source refresh: %v", err)
	}
	if fromPacket.Position != itemproto.InventoryPosition(5) || fromPacket.Vnum != 0x55667788 || fromPacket.Count != 1 {
		t.Fatalf("unexpected item-move swap source refresh: %+v", fromPacket)
	}
	toPacket, err := itemproto.DecodeSet(decodeSingleFrame(t, moveOut[1]))
	if err != nil {
		t.Fatalf("decode item-move swap destination refresh: %v", err)
	}
	if toPacket.Position != itemproto.InventoryPosition(8) || toPacket.Vnum != 0x11223344 || toPacket.Count != 3 {
		t.Fatalf("unexpected item-move swap destination refresh: %+v", toPacket)
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[1].Inventory, []inventory.ItemInstance{
		{ID: 2002, Vnum: 0x55667788, Count: 1, Slot: 5},
		{ID: 1001, Vnum: 0x11223344, Count: 3, Slot: 8},
	}) {
		t.Fatalf("unexpected persisted inventory after item-move swap: %#v", account.Characters[1].Inventory)
	}
}

func TestNewGameSessionFactoryItemMovePacketZeroCountMergesCompatibleOccupiedDestination(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{
		{ID: 1001, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 2002, Vnum: 27001, Count: 198, Slot: 8},
	}
	characters[1].Equipment = []inventory.ItemInstance{}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(8),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected compatible occupied-destination item move packet error: %v", err)
	}
	if len(moveOut) != 2 {
		t.Fatalf("expected two update frames for zero-count compatible occupied-destination merge, got %d", len(moveOut))
	}
	fromPacket, err := itemproto.DecodeUpdate(decodeSingleFrame(t, moveOut[0]))
	if err != nil {
		t.Fatalf("decode zero-count merge source refresh: %v", err)
	}
	if fromPacket.Position != itemproto.InventoryPosition(5) || fromPacket.Count != 1 {
		t.Fatalf("unexpected zero-count merge source refresh: %+v", fromPacket)
	}
	toPacket, err := itemproto.DecodeUpdate(decodeSingleFrame(t, moveOut[1]))
	if err != nil {
		t.Fatalf("decode zero-count merge destination refresh: %v", err)
	}
	if toPacket.Position != itemproto.InventoryPosition(8) || toPacket.Count != 200 {
		t.Fatalf("unexpected zero-count merge destination refresh: %+v", toPacket)
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[1].Inventory, []inventory.ItemInstance{
		{ID: 1001, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 2002, Vnum: 27001, Count: 200, Slot: 8},
	}) {
		t.Fatalf("unexpected persisted inventory after zero-count compatible merge: %#v", account.Characters[1].Inventory)
	}
}

func TestNewGameSessionFactoryItemMovePacketSplitsPartialStackPersistsAndEmitsTwoSetFrames(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 3, Slot: 5}}
	characters[1].Equipment = []inventory.ItemInstance{}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       2,
	})))
	if err != nil {
		t.Fatalf("unexpected item move packet error: %v", err)
	}
	if len(moveOut) != 2 {
		t.Fatalf("expected two set frames for partial stack split item move, got %d", len(moveOut))
	}
	fromPacket, err := itemproto.DecodeSet(decodeSingleFrame(t, moveOut[0]))
	if err != nil {
		t.Fatalf("decode partial item move source refresh: %v", err)
	}
	if fromPacket.Position.WindowType != itemproto.WindowInventory || fromPacket.Position.Cell != 5 || fromPacket.Vnum != 27001 || fromPacket.Count != 1 {
		t.Fatalf("unexpected partial item move source refresh: %+v", fromPacket)
	}
	toPacket, err := itemproto.DecodeSet(decodeSingleFrame(t, moveOut[1]))
	if err != nil {
		t.Fatalf("decode partial item move destination refresh: %v", err)
	}
	if toPacket.Position.WindowType != itemproto.WindowInventory || toPacket.Position.Cell != 6 || toPacket.Vnum != 27001 || toPacket.Count != 2 {
		t.Fatalf("unexpected partial item move destination refresh: %+v", toPacket)
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if len(account.Characters[1].Inventory) != 2 {
		t.Fatalf("expected two persisted inventory stacks after partial item move, got %#v", account.Characters[1].Inventory)
	}
	if account.Characters[1].Inventory[0] != (inventory.ItemInstance{ID: 1001, Vnum: 27001, Count: 1, Slot: 5}) {
		t.Fatalf("unexpected persisted source stack after partial item move: %#v", account.Characters[1].Inventory)
	}
	if account.Characters[1].Inventory[1].ID == 0 || account.Characters[1].Inventory[1].ID == 1001 || account.Characters[1].Inventory[1].Vnum != 27001 || account.Characters[1].Inventory[1].Count != 2 || account.Characters[1].Inventory[1].Slot != 6 {
		t.Fatalf("unexpected persisted destination split stack after partial item move: %#v", account.Characters[1].Inventory)
	}
}

func TestNewGameSessionFactoryItemMovePacketMergesPartialStackPersistsAndEmitsTwoUpdateFrames(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{
		{ID: 1001, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 2002, Vnum: 27001, Count: 7, Slot: 8},
	}
	characters[1].Equipment = []inventory.ItemInstance{}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(8),
		Count:       2,
	})))
	if err != nil {
		t.Fatalf("unexpected item move packet error: %v", err)
	}
	if len(moveOut) != 2 {
		t.Fatalf("expected two update frames for partial stack merge item move, got %d", len(moveOut))
	}
	fromPacket, err := itemproto.DecodeUpdate(decodeSingleFrame(t, moveOut[0]))
	if err != nil {
		t.Fatalf("decode partial item merge source refresh: %v", err)
	}
	if fromPacket.Position.WindowType != itemproto.WindowInventory || fromPacket.Position.Cell != 5 || fromPacket.Count != 1 {
		t.Fatalf("unexpected partial item merge source refresh: %+v", fromPacket)
	}
	toPacket, err := itemproto.DecodeUpdate(decodeSingleFrame(t, moveOut[1]))
	if err != nil {
		t.Fatalf("decode partial item merge destination refresh: %v", err)
	}
	if toPacket.Position.WindowType != itemproto.WindowInventory || toPacket.Position.Cell != 8 || toPacket.Count != 9 {
		t.Fatalf("unexpected partial item merge destination refresh: %+v", toPacket)
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[1].Inventory, []inventory.ItemInstance{
		{ID: 1001, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 2002, Vnum: 27001, Count: 9, Slot: 8},
	}) {
		t.Fatalf("unexpected persisted inventory after partial item merge: %#v", account.Characters[1].Inventory)
	}
}

func TestNewGameSessionFactoryItemMovePacketRejectsLockedDestinationWithoutMutation(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{
		{ID: 1001, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 2002, Vnum: 1120, Count: 1, Slot: 8, Locked: true},
	}
	characters[1].Equipment = []inventory.ItemInstance{}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(8),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected locked-destination item move packet error: %v", err)
	}
	if len(moveOut) != 0 {
		t.Fatalf("expected locked-destination item move to fail closed with no frames, got %d", len(moveOut))
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[1].Inventory, characters[1].Inventory) {
		t.Fatalf("expected locked-destination item move to leave persisted inventory unchanged, got %#v want %#v", account.Characters[1].Inventory, characters[1].Inventory)
	}
}

func TestNewGameSessionFactoryItemMovePacketRejectsIncompatiblePartialStackDestinationWithoutMutation(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{
		{ID: 1001, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 2002, Vnum: 1120, Count: 1, Slot: 8},
	}
	characters[1].Equipment = []inventory.ItemInstance{}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(8),
		Count:       2,
	})))
	if err != nil {
		t.Fatalf("unexpected incompatible item move packet error: %v", err)
	}
	if len(moveOut) != 0 {
		t.Fatalf("expected incompatible partial-stack item move to fail closed with no frames, got %d", len(moveOut))
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[1].Inventory, characters[1].Inventory) {
		t.Fatalf("expected incompatible partial-stack item move to leave persisted inventory unchanged, got %#v want %#v", account.Characters[1].Inventory, characters[1].Inventory)
	}
}

func TestNewGameSessionFactoryItemMovePacketRejectsTemplateMismatchedEquipSlotWithoutMutation(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	itemTemplates := staticItemTemplateStore{snapshot: itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:      0x11223344,
		Name:      "Practice Sword",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: inventory.EquipmentSlotWeapon.String(),
	}}}}
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 0x11223344, Count: 1, Slot: 8}}
	characters[1].Equipment = []inventory.ItemInstance{}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
	}

	flow := newStartedGameFlowWithItemStore(t, store, accounts, itemTemplates)

	equipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{Source: itemproto.InventoryPosition(8), Destination: mustEquipmentPosition(t, 0)})))
	if err != nil {
		t.Fatalf("unexpected mismatched equip-slot item-move error: %v", err)
	}
	if len(equipOut) != 0 {
		t.Fatalf("expected mismatched equip-slot item move to fail closed with no frames, got %d", len(equipOut))
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if !sameItemInstances(account.Characters[1].Inventory, characters[1].Inventory) {
		t.Fatalf("expected mismatched equip-slot item move to leave inventory unchanged, got %#v want %#v", account.Characters[1].Inventory, characters[1].Inventory)
	}
	if len(account.Characters[1].Equipment) != 0 {
		t.Fatalf("expected mismatched equip-slot item move to leave equipment empty, got %#v", account.Characters[1].Equipment)
	}
}

func TestNewGameSessionFactoryItemMovePacketRejectsTemplateAntiFlagWithoutMutation(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	itemTemplates := staticItemTemplateStore{snapshot: itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:        0x11223344,
		Name:        "Restricted Practice Armor",
		Stackable:   false,
		MaxCount:    1,
		EquipSlot:   inventory.EquipmentSlotBody.String(),
		AntiWarrior: true,
	}}}}
	characters := stubCharacters()
	characters[1].Job = 0
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 0x11223344, Count: 1, Slot: 8}}
	characters[1].Equipment = []inventory.ItemInstance{}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
	}

	flow := newStartedGameFlowWithItemStore(t, store, accounts, itemTemplates)

	equipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{Source: itemproto.InventoryPosition(8), Destination: mustEquipmentPosition(t, 0)})))
	if err != nil {
		t.Fatalf("unexpected anti-flagged item-move error: %v", err)
	}
	if len(equipOut) != 0 {
		t.Fatalf("expected anti-flagged item move to fail closed with no frames, got %d", len(equipOut))
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if !sameItemInstances(account.Characters[1].Inventory, characters[1].Inventory) {
		t.Fatalf("expected anti-flagged item move to leave inventory unchanged, got %#v want %#v", account.Characters[1].Inventory, characters[1].Inventory)
	}
	if len(account.Characters[1].Equipment) != 0 {
		t.Fatalf("expected anti-flagged item move to leave equipment empty, got %#v", account.Characters[1].Equipment)
	}
}

func TestNewGameSessionFactoryItemMovePacketEquipsInventoryItem(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 0x11223344, Count: 1, Slot: 8}}
	characters[1].Equipment = []inventory.ItemInstance{}
	characters[1].Quickslots = []loginticket.Quickslot{{Position: 4, Type: quickslotproto.TypeItem, Slot: 8}}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}
	bodyPosition, err := itemproto.EquipmentPosition(0)
	if err != nil {
		t.Fatalf("resolve body equipment position: %v", err)
	}

	equipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{Source: itemproto.InventoryPosition(8), Destination: bodyPosition})))
	if err != nil {
		t.Fatalf("unexpected item-move equip error: %v", err)
	}
	if len(equipOut) != 4 {
		t.Fatalf("expected delete+set+update+quickslot-del frames for packet equip, got %d", len(equipOut))
	}
	delPacket, err := itemproto.DecodeDel(decodeSingleFrame(t, equipOut[0]))
	if err != nil {
		t.Fatalf("decode packet equip inventory delete: %v", err)
	}
	if delPacket.Position.WindowType != itemproto.WindowInventory || delPacket.Position.Cell != 8 {
		t.Fatalf("unexpected packet equip inventory delete packet: %+v", delPacket)
	}
	setPacket, err := itemproto.DecodeSet(decodeSingleFrame(t, equipOut[1]))
	if err != nil {
		t.Fatalf("decode packet equip equipment set: %v", err)
	}
	if setPacket.Position != bodyPosition || setPacket.Vnum != 0x11223344 || setPacket.Count != 1 {
		t.Fatalf("unexpected packet equip equipment set packet: %+v", setPacket)
	}
	if _, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, equipOut[2])); err != nil {
		t.Fatalf("decode packet equip character update: %v", err)
	}
	quickslotDelPacket, err := quickslotproto.DecodeDel(decodeSingleFrame(t, equipOut[3]))
	if err != nil {
		t.Fatalf("decode packet equip quickslot delete: %v", err)
	}
	if quickslotDelPacket.Position != 4 {
		t.Fatalf("unexpected packet equip quickslot delete packet: %+v", quickslotDelPacket)
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if len(account.Characters[1].Inventory) != 0 {
		t.Fatalf("expected persisted inventory to be empty after packet equip, got %#v", account.Characters[1].Inventory)
	}
	if !reflect.DeepEqual(account.Characters[1].Equipment, []inventory.ItemInstance{{ID: 1001, Vnum: 0x11223344, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotBody}}) {
		t.Fatalf("unexpected persisted equipment after packet equip: %#v", account.Characters[1].Equipment)
	}
	if len(account.Characters[1].Quickslots) != 0 {
		t.Fatalf("expected persisted quickslots to delete equipped item reference, got %#v", account.Characters[1].Quickslots)
	}
}

func TestNewGameSessionFactoryItemMovePacketUnequipsEquipmentItem(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{}
	characters[1].Equipment = []inventory.ItemInstance{{ID: 1001, Vnum: 0x11223344, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotBody}}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}
	bodyPosition, err := itemproto.EquipmentPosition(0)
	if err != nil {
		t.Fatalf("resolve body equipment position: %v", err)
	}

	unequipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{Source: bodyPosition, Destination: itemproto.InventoryPosition(8)})))
	if err != nil {
		t.Fatalf("unexpected item-move unequip error: %v", err)
	}
	if len(unequipOut) != 3 {
		t.Fatalf("expected delete+set+update frames for packet unequip, got %d", len(unequipOut))
	}
	delPacket, err := itemproto.DecodeDel(decodeSingleFrame(t, unequipOut[0]))
	if err != nil {
		t.Fatalf("decode packet unequip equipment delete: %v", err)
	}
	if delPacket.Position != bodyPosition {
		t.Fatalf("unexpected packet unequip equipment delete packet: %+v", delPacket)
	}
	setPacket, err := itemproto.DecodeSet(decodeSingleFrame(t, unequipOut[1]))
	if err != nil {
		t.Fatalf("decode packet unequip inventory set: %v", err)
	}
	if setPacket.Position != itemproto.InventoryPosition(8) || setPacket.Vnum != 0x11223344 || setPacket.Count != 1 {
		t.Fatalf("unexpected packet unequip inventory set packet: %+v", setPacket)
	}
	if _, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, unequipOut[2])); err != nil {
		t.Fatalf("decode packet unequip character update: %v", err)
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if len(account.Characters[1].Equipment) != 0 {
		t.Fatalf("expected persisted equipment to be empty after packet unequip, got %#v", account.Characters[1].Equipment)
	}
	if !reflect.DeepEqual(account.Characters[1].Inventory, []inventory.ItemInstance{{ID: 1001, Vnum: 0x11223344, Count: 1, Slot: 8}}) {
		t.Fatalf("unexpected persisted inventory after packet unequip: %#v", account.Characters[1].Inventory)
	}
}

func TestNewGameSessionFactoryItemMovePacketRejectsCountedEquipOrUnequipWithoutMutation(t *testing.T) {
	cases := []struct {
		name      string
		inventory []inventory.ItemInstance
		equipment []inventory.ItemInstance
		packet    itemproto.ClientMovePacket
	}{
		{
			name:      "counted equip",
			inventory: []inventory.ItemInstance{{ID: 1001, Vnum: 0x11223344, Count: 1, Slot: 8}},
			packet: itemproto.ClientMovePacket{
				Source:      itemproto.InventoryPosition(8),
				Destination: mustEquipmentPosition(t, 0),
				Count:       1,
			},
		},
		{
			name:      "counted unequip",
			equipment: []inventory.ItemInstance{{ID: 1002, Vnum: 0x55667788, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotBody}},
			packet: itemproto.ClientMovePacket{
				Source:      mustEquipmentPosition(t, 0),
				Destination: itemproto.InventoryPosition(8),
				Count:       1,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			characters := stubCharacters()
			characters[1].Inventory = append([]inventory.ItemInstance(nil), tc.inventory...)
			characters[1].Equipment = append([]inventory.ItemInstance(nil), tc.equipment...)
			if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
				t.Fatalf("issue login ticket: %v", err)
			}
			if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
				t.Fatalf("seed account store: %v", err)
			}

			flow := newStartedGameFlow(t, store, accounts)
			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(tc.packet)))
			if err != nil {
				t.Fatalf("unexpected counted equipment item move error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s to fail closed with no frames, got %d", tc.name, len(out))
			}
			account, err := accounts.Load(StubLogin)
			if err != nil {
				t.Fatalf("load account after counted equipment item move: %v", err)
			}
			if !sameItemInstances(account.Characters[1].Inventory, tc.inventory) {
				t.Fatalf("expected %s to leave inventory unchanged, got %#v want %#v", tc.name, account.Characters[1].Inventory, tc.inventory)
			}
			if !sameItemInstances(account.Characters[1].Equipment, tc.equipment) {
				t.Fatalf("expected %s to leave equipment unchanged, got %#v want %#v", tc.name, account.Characters[1].Equipment, tc.equipment)
			}
		})
	}
}

func mustEquipmentPosition(t *testing.T, wearCell uint16) itemproto.Position {
	t.Helper()
	position, err := itemproto.EquipmentPosition(wearCell)
	if err != nil {
		t.Fatalf("resolve equipment position: %v", err)
	}
	return position
}

func sameItemInstances(a, b []inventory.ItemInstance) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestNewGameSessionFactoryEquipPersistsAndEmitsInventoryDeleteThenEquipmentSet(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 0x11223344, Count: 1, Slot: 8}}
	characters[1].Equipment = []inventory.ItemInstance{}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	equipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/equip_item 8 body"})))
	if err != nil {
		t.Fatalf("unexpected equip error: %v", err)
	}
	if len(equipOut) != 3 {
		t.Fatalf("expected delete+set+update frames for equip, got %d", len(equipOut))
	}
	delPacket, err := itemproto.DecodeDel(decodeSingleFrame(t, equipOut[0]))
	if err != nil {
		t.Fatalf("decode equip inventory delete: %v", err)
	}
	if delPacket.Position.WindowType != itemproto.WindowInventory || delPacket.Position.Cell != 8 {
		t.Fatalf("unexpected equip inventory delete packet: %+v", delPacket)
	}
	setPacket, err := itemproto.DecodeSet(decodeSingleFrame(t, equipOut[1]))
	if err != nil {
		t.Fatalf("decode equip equipment set: %v", err)
	}
	wantBodyPosition, err := itemproto.EquipmentPosition(0)
	if err != nil {
		t.Fatalf("resolve body equipment position: %v", err)
	}
	if setPacket.Position != wantBodyPosition || setPacket.Vnum != 0x11223344 || setPacket.Count != 1 {
		t.Fatalf("unexpected equip equipment set packet: %+v", setPacket)
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if len(account.Characters[1].Inventory) != 0 {
		t.Fatalf("expected persisted inventory to be empty after equip, got %#v", account.Characters[1].Inventory)
	}
	if !reflect.DeepEqual(account.Characters[1].Equipment, []inventory.ItemInstance{{ID: 1001, Vnum: 0x11223344, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotBody}}) {
		t.Fatalf("unexpected persisted equipment after equip: %#v", account.Characters[1].Equipment)
	}
}

func TestNewGameSessionFactoryEquipDeletesOnlySourceItemQuickslot(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 0x11223344, Count: 1, Slot: 8}}
	characters[1].Equipment = []inventory.ItemInstance{}
	characters[1].Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 8},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 8},
	}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	equipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/equip_item 8 body"})))
	if err != nil {
		t.Fatalf("unexpected equip quickslot sync error: %v", err)
	}
	if len(equipOut) != 4 {
		t.Fatalf("expected delete+set+update+quickslot delete frames for equip, got %d", len(equipOut))
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, equipOut[3]))
	if err != nil {
		t.Fatalf("decode equip quickslot delete: %v", err)
	}
	if quickslotDel.Position != 2 {
		t.Fatalf("expected equip to delete only item quickslot position 2, got %+v", quickslotDel)
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if got := account.Characters[1].Quickslots; !reflect.DeepEqual(got, []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeSkill, Slot: 8}}) {
		t.Fatalf("expected equip to preserve only non-item source quickslot, got %#v", got)
	}
}

func TestNewGameSessionFactoryUnequipPersistsAndEmitsEquipmentDeleteThenInventorySet(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{}
	characters[1].Equipment = []inventory.ItemInstance{{ID: 2002, Vnum: 0x55667788, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	unequipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/unequip_item weapon 4"})))
	if err != nil {
		t.Fatalf("unexpected unequip error: %v", err)
	}
	if len(unequipOut) != 3 {
		t.Fatalf("expected delete+set+update frames for unequip, got %d", len(unequipOut))
	}
	delPacket, err := itemproto.DecodeDel(decodeSingleFrame(t, unequipOut[0]))
	if err != nil {
		t.Fatalf("decode unequip equipment delete: %v", err)
	}
	wantWeaponPosition, err := itemproto.EquipmentPosition(4)
	if err != nil {
		t.Fatalf("resolve weapon equipment position: %v", err)
	}
	if delPacket.Position != wantWeaponPosition {
		t.Fatalf("unexpected unequip equipment delete packet: %+v", delPacket)
	}
	setPacket, err := itemproto.DecodeSet(decodeSingleFrame(t, unequipOut[1]))
	if err != nil {
		t.Fatalf("decode unequip inventory set: %v", err)
	}
	if setPacket.Position.WindowType != itemproto.WindowInventory || setPacket.Position.Cell != 4 || setPacket.Vnum != 0x55667788 || setPacket.Count != 1 {
		t.Fatalf("unexpected unequip inventory set packet: %+v", setPacket)
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[1].Inventory, []inventory.ItemInstance{{ID: 2002, Vnum: 0x55667788, Count: 1, Slot: 4}}) {
		t.Fatalf("unexpected persisted inventory after unequip: %#v", account.Characters[1].Inventory)
	}
	if len(account.Characters[1].Equipment) != 0 {
		t.Fatalf("expected persisted equipment to be empty after unequip, got %#v", account.Characters[1].Equipment)
	}
}

func TestNewGameSessionFactoryEquipAppendsCharacterUpdateWithProjectedAppearance(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 11500, Count: 1, Slot: 8}}
	characters[1].Equipment = []inventory.ItemInstance{}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	equipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/equip_item 8 body"})))
	if err != nil {
		t.Fatalf("unexpected equip error: %v", err)
	}
	if len(equipOut) != 3 {
		t.Fatalf("expected delete+set+update frames for equip, got %d", len(equipOut))
	}
	update, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, equipOut[2]))
	if err != nil {
		t.Fatalf("decode equip character update: %v", err)
	}
	if update.VID != characters[1].VID || update.Parts != [worldproto.CharacterEquipmentPartCount]uint16{11500, 0, 0, 202} {
		t.Fatalf("unexpected equip appearance update packet: %+v", update)
	}
}

func TestNewGameSessionFactoryUnequipAppendsCharacterUpdateClearingProjectedAppearance(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{}
	characters[1].Equipment = []inventory.ItemInstance{{ID: 2002, Vnum: 11200, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	unequipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/unequip_item weapon 4"})))
	if err != nil {
		t.Fatalf("unexpected unequip error: %v", err)
	}
	if len(unequipOut) != 3 {
		t.Fatalf("expected delete+set+update frames for unequip, got %d", len(unequipOut))
	}
	update, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, unequipOut[2]))
	if err != nil {
		t.Fatalf("decode unequip character update: %v", err)
	}
	if update.VID != characters[1].VID || update.Parts != [worldproto.CharacterEquipmentPartCount]uint16{102, 0, 0, 202} {
		t.Fatalf("unexpected unequip appearance update packet: %+v", update)
	}
}

func TestNewGameSessionFactoryEquipAppendsPlayerPointChangeForTemplateBackedEffect(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Points[bootstrapPlayerPointValueIndex] = 700
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 12200, Count: 1, Slot: 8}}
	characters[1].Equipment = []inventory.ItemInstance{}
	wantVID := characters[1].VID
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	equipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/equip_item 8 weapon"})))
	if err != nil {
		t.Fatalf("unexpected equip error: %v", err)
	}
	if len(equipOut) != 4 {
		t.Fatalf("expected delete+set+point-change+update frames for equip, got %d", len(equipOut))
	}
	pointPacket, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, equipOut[2]))
	if err != nil {
		t.Fatalf("decode equip point-change frame: %v", err)
	}
	if pointPacket.VID != wantVID || pointPacket.Type != bootstrapPlayerPointType || pointPacket.Amount != 10 || pointPacket.Value != 710 {
		t.Fatalf("unexpected equip point-change packet: %+v", pointPacket)
	}
	update, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, equipOut[3]))
	if err != nil {
		t.Fatalf("decode equip character update: %v", err)
	}
	if update.VID != wantVID || update.Parts != [worldproto.CharacterEquipmentPartCount]uint16{102, 12200, 0, 202} {
		t.Fatalf("unexpected equip appearance update packet: %+v", update)
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if got := account.Characters[1].Points[bootstrapPlayerPointValueIndex]; got != 710 {
		t.Fatalf("expected persisted points[1] to be 710 after equip, got %d", got)
	}
}

func TestNewGameSessionFactoryItemMovePacketRejectsTemplateMismatchedEffectSlotWithoutMutation(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Points[bootstrapPlayerPointValueIndex] = 700
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 12200, Count: 1, Slot: 8}}
	characters[1].Equipment = []inventory.ItemInstance{}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	equipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{Source: itemproto.InventoryPosition(8), Destination: mustEquipmentPosition(t, 0)})))
	if err != nil {
		t.Fatalf("unexpected mismatched-slot packet equip error: %v", err)
	}
	if len(equipOut) != 0 {
		t.Fatalf("expected mismatched-slot packet equip to fail closed with no frames, got %d", len(equipOut))
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if got := account.Characters[1].Points[bootstrapPlayerPointValueIndex]; got != 700 {
		t.Fatalf("expected persisted points[1] to stay 700 after mismatched-slot packet equip, got %d", got)
	}
	if !sameItemInstances(account.Characters[1].Inventory, characters[1].Inventory) {
		t.Fatalf("expected mismatched-slot packet equip to leave inventory unchanged, got %#v want %#v", account.Characters[1].Inventory, characters[1].Inventory)
	}
	if len(account.Characters[1].Equipment) != 0 {
		t.Fatalf("expected mismatched-slot packet equip to leave equipment empty, got %#v", account.Characters[1].Equipment)
	}
}

func TestNewGameSessionFactoryUnequipAppendsPlayerPointChangeForTemplateBackedEffect(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Points[bootstrapPlayerPointValueIndex] = 710
	characters[1].Inventory = []inventory.ItemInstance{}
	characters[1].Equipment = []inventory.ItemInstance{{ID: 2002, Vnum: 12200, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	wantVID := characters[1].VID
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	unequipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/unequip_item weapon 4"})))
	if err != nil {
		t.Fatalf("unexpected unequip error: %v", err)
	}
	if len(unequipOut) != 4 {
		t.Fatalf("expected delete+set+point-change+update frames for unequip, got %d", len(unequipOut))
	}
	pointPacket, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, unequipOut[2]))
	if err != nil {
		t.Fatalf("decode unequip point-change frame: %v", err)
	}
	if pointPacket.VID != wantVID || pointPacket.Type != bootstrapPlayerPointType || pointPacket.Amount != -10 || pointPacket.Value != 700 {
		t.Fatalf("unexpected unequip point-change packet: %+v", pointPacket)
	}
	update, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, unequipOut[3]))
	if err != nil {
		t.Fatalf("decode unequip character update: %v", err)
	}
	if update.VID != wantVID || update.Parts != [worldproto.CharacterEquipmentPartCount]uint16{102, 0, 0, 202} {
		t.Fatalf("unexpected unequip appearance update packet: %+v", update)
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if got := account.Characters[1].Points[bootstrapPlayerPointValueIndex]; got != 700 {
		t.Fatalf("expected persisted points[1] to be 700 after unequip, got %d", got)
	}
}

func TestNewGameSessionFactoryItemUsePersistsPointChangeAndDecrementsTheConsumableStack(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Points[bootstrapPlayerPointValueIndex] = 700
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 3, Slot: 5}}
	characters[1].Equipment = []inventory.ItemInstance{}
	wantVID := characters[1].VID
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	useOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/use_item 5"})))
	if err != nil {
		t.Fatalf("unexpected item use error: %v", err)
	}
	if len(useOut) != 3 {
		t.Fatalf("expected point-change + item-set + info frames for item use, got %d", len(useOut))
	}
	pointPacket, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, useOut[0]))
	if err != nil {
		t.Fatalf("decode point-change frame: %v", err)
	}
	if pointPacket.VID != wantVID || pointPacket.Type != bootstrapPlayerPointType || pointPacket.Amount != 50 || pointPacket.Value != 750 {
		t.Fatalf("unexpected point-change packet: %+v", pointPacket)
	}
	setPacket, err := itemproto.DecodeSet(decodeSingleFrame(t, useOut[1]))
	if err != nil {
		t.Fatalf("decode item-use set frame: %v", err)
	}
	if setPacket.Position.WindowType != itemproto.WindowInventory || setPacket.Position.Cell != 5 || setPacket.Vnum != 27001 || setPacket.Count != 2 {
		t.Fatalf("unexpected item-use set packet: %+v", setPacket)
	}
	infoPacket, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, useOut[2]))
	if err != nil {
		t.Fatalf("decode item-use info frame: %v", err)
	}
	if infoPacket.Type != chatproto.ChatTypeInfo || infoPacket.VID != 0 || infoPacket.Message != "consume:27001:+50" {
		t.Fatalf("unexpected item-use info packet: %+v", infoPacket)
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if got := account.Characters[1].Points[bootstrapPlayerPointValueIndex]; got != 750 {
		t.Fatalf("expected persisted points[1] to be 750 after item use, got %d", got)
	}
	if !reflect.DeepEqual(account.Characters[1].Inventory, []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 2, Slot: 5}}) {
		t.Fatalf("unexpected persisted inventory after item use: %#v", account.Characters[1].Inventory)
	}
}

func TestNewGameSessionFactoryItemUsePacketPersistsPointChangeAndDecrementsTheConsumableStack(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Points[bootstrapPlayerPointValueIndex] = 700
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 3, Slot: 5}}
	characters[1].Equipment = []inventory.ItemInstance{}
	wantVID := characters[1].VID
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	useOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.Position{WindowType: itemproto.WindowInventory, Cell: 5}})))
	if err != nil {
		t.Fatalf("unexpected packet item-use error: %v", err)
	}
	if len(useOut) != 3 {
		t.Fatalf("expected point-change + item-set + info frames for packet item use, got %d", len(useOut))
	}
	pointPacket, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, useOut[0]))
	if err != nil {
		t.Fatalf("decode point-change frame: %v", err)
	}
	if pointPacket.VID != wantVID || pointPacket.Type != bootstrapPlayerPointType || pointPacket.Amount != 50 || pointPacket.Value != 750 {
		t.Fatalf("unexpected point-change packet: %+v", pointPacket)
	}
	setPacket, err := itemproto.DecodeSet(decodeSingleFrame(t, useOut[1]))
	if err != nil {
		t.Fatalf("decode packet item-use set frame: %v", err)
	}
	if setPacket.Position.WindowType != itemproto.WindowInventory || setPacket.Position.Cell != 5 || setPacket.Vnum != 27001 || setPacket.Count != 2 {
		t.Fatalf("unexpected packet item-use set packet: %+v", setPacket)
	}
	infoPacket, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, useOut[2]))
	if err != nil {
		t.Fatalf("decode packet item-use info frame: %v", err)
	}
	if infoPacket.Type != chatproto.ChatTypeInfo || infoPacket.VID != 0 || infoPacket.Message != "consume:27001:+50" {
		t.Fatalf("unexpected packet item-use info packet: %+v", infoPacket)
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if got := account.Characters[1].Points[bootstrapPlayerPointValueIndex]; got != 750 {
		t.Fatalf("expected persisted points[1] to be 750 after packet item use, got %d", got)
	}
	if !reflect.DeepEqual(account.Characters[1].Inventory, []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 2, Slot: 5}}) {
		t.Fatalf("unexpected persisted inventory after packet item use: %#v", account.Characters[1].Inventory)
	}
}

func TestNewGameSessionFactoryItemUsePacketRejectsAuthoredJobAntiFlagWithoutMutation(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Points[bootstrapPlayerPointValueIndex] = 700
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 3, Slot: 5}}
	characters[1].Equipment = []inventory.ItemInstance{}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
	}
	itemTemplates := staticItemTemplateStore{snapshot: itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:       27001,
		Name:       "Shaman-Restricted Potion",
		Stackable:  true,
		MaxCount:   200,
		AntiShaman: true,
		UseEffect: &itemcatalog.UseEffect{
			PointType:  bootstrapPlayerPointType,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: 50,
			Message:    "consume:27001:+50",
		},
	}}}}
	flow := newStartedGameFlowWithItemStore(t, store, accounts, itemTemplates)

	useOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.Position{WindowType: itemproto.WindowInventory, Cell: 5}})))
	if err != nil {
		t.Fatalf("unexpected packet item-use error: %v", err)
	}
	if len(useOut) != 0 {
		t.Fatalf("expected authored anti-shaman item-use to fail closed with no frames, got %d", len(useOut))
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if got := account.Characters[1].Points[bootstrapPlayerPointValueIndex]; got != 700 {
		t.Fatalf("expected persisted points[1] to stay unchanged after anti-flag item-use attempt, got %d", got)
	}
	if !reflect.DeepEqual(account.Characters[1].Inventory, []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 3, Slot: 5}}) {
		t.Fatalf("unexpected persisted inventory after anti-flag item-use attempt: %#v", account.Characters[1].Inventory)
	}
}

func TestNewGameSessionFactoryItemUsePacketRejectsEquipmentPosition(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Points[bootstrapPlayerPointValueIndex] = 700
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 3, Slot: 5}}
	characters[1].Equipment = []inventory.ItemInstance{}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}
	position, err := itemproto.EquipmentPosition(4)
	if err != nil {
		t.Fatalf("unexpected equipment position error: %v", err)
	}

	useOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: position})))
	if err != nil {
		t.Fatalf("unexpected packet item-use error for equipment position: %v", err)
	}
	if len(useOut) != 0 {
		t.Fatalf("expected packet item-use to fail closed for equipment position, got %d frames", len(useOut))
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if got := account.Characters[1].Points[bootstrapPlayerPointValueIndex]; got != 700 {
		t.Fatalf("expected persisted points[1] to stay unchanged after equipment-position item-use attempt, got %d", got)
	}
	if !reflect.DeepEqual(account.Characters[1].Inventory, []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 3, Slot: 5}}) {
		t.Fatalf("unexpected persisted inventory after equipment-position item-use attempt: %#v", account.Characters[1].Inventory)
	}
}

func TestNewGameSessionFactoryItemUseConsumesTheLastStackAndEmitsDelete(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Points[bootstrapPlayerPointValueIndex] = 700
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 1, Slot: 5}}
	characters[1].Equipment = []inventory.ItemInstance{}
	characters[1].Quickslots = []loginticket.Quickslot{{Position: 4, Type: quickslotproto.TypeItem, Slot: 5}}
	wantVID := characters[1].VID
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	useOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/use_item 5"})))
	if err != nil {
		t.Fatalf("unexpected item use error: %v", err)
	}
	if len(useOut) != 4 {
		t.Fatalf("expected point-change + item-del + quickslot-del + info frames for last-stack item use, got %d", len(useOut))
	}
	pointPacket, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, useOut[0]))
	if err != nil {
		t.Fatalf("decode point-change frame: %v", err)
	}
	if pointPacket.VID != wantVID || pointPacket.Type != bootstrapPlayerPointType || pointPacket.Amount != 50 || pointPacket.Value != 750 {
		t.Fatalf("unexpected point-change packet: %+v", pointPacket)
	}
	delPacket, err := itemproto.DecodeDel(decodeSingleFrame(t, useOut[1]))
	if err != nil {
		t.Fatalf("decode item-use del frame: %v", err)
	}
	if delPacket.Position.WindowType != itemproto.WindowInventory || delPacket.Position.Cell != 5 {
		t.Fatalf("unexpected item-use del packet: %+v", delPacket)
	}
	quickslotDelPacket, err := quickslotproto.DecodeDel(decodeSingleFrame(t, useOut[2]))
	if err != nil {
		t.Fatalf("decode item-use quickslot delete frame: %v", err)
	}
	if quickslotDelPacket.Position != 4 {
		t.Fatalf("unexpected item-use quickslot delete packet: %+v", quickslotDelPacket)
	}
	infoPacket, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, useOut[3]))
	if err != nil {
		t.Fatalf("decode item-use info frame: %v", err)
	}
	if infoPacket.Type != chatproto.ChatTypeInfo || infoPacket.VID != 0 || infoPacket.Message != "consume:27001:+50" {
		t.Fatalf("unexpected item-use info packet: %+v", infoPacket)
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if got := account.Characters[1].Points[bootstrapPlayerPointValueIndex]; got != 750 {
		t.Fatalf("expected persisted points[1] to be 750 after item use, got %d", got)
	}
	if len(account.Characters[1].Inventory) != 0 {
		t.Fatalf("expected persisted inventory to be empty after consuming the last stack, got %#v", account.Characters[1].Inventory)
	}
	if len(account.Characters[1].Quickslots) != 0 {
		t.Fatalf("expected persisted quickslots to delete removed item reference, got %#v", account.Characters[1].Quickslots)
	}
}

func TestNewGameSessionFactoryItemUsePacketConsumesLastStackAndDeletesOnlyItemQuickslots(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Points[bootstrapPlayerPointValueIndex] = 700
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 1, Slot: 5}}
	characters[1].Equipment = []inventory.ItemInstance{}
	characters[1].Quickslots = []loginticket.Quickslot{
		{Position: 1, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
	}
	flow := newStartedGameFlowWithItemStore(t, store, accounts, staticItemTemplateStore{snapshot: itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Template Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{
			PointType:  bootstrapPlayerPointType,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: 50,
			Message:    "consume:27001:+50",
		},
	}}}})

	useOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected packet item-use last-stack error: %v", err)
	}
	if len(useOut) != 5 {
		t.Fatalf("expected point-change, item-del, two quickslot-del frames, and info for packet last-stack item use, got %d", len(useOut))
	}
	if _, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, useOut[0])); err != nil {
		t.Fatalf("decode packet last-stack point-change: %v", err)
	}
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, useOut[1]))
	if err != nil {
		t.Fatalf("decode packet last-stack item del: %v", err)
	}
	if del.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected packet last-stack item del: %+v", del)
	}
	firstQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, useOut[2]))
	if err != nil {
		t.Fatalf("decode first packet last-stack quickslot del: %v", err)
	}
	secondQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, useOut[3]))
	if err != nil {
		t.Fatalf("decode second packet last-stack quickslot del: %v", err)
	}
	if firstQuickslotDel.Position != 1 || secondQuickslotDel.Position != 2 {
		t.Fatalf("unexpected packet last-stack quickslot deletes: %+v %+v", firstQuickslotDel, secondQuickslotDel)
	}
	info, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, useOut[4]))
	if err != nil {
		t.Fatalf("decode packet last-stack info: %v", err)
	}
	if info.Type != chatproto.ChatTypeInfo || info.VID != 0 || info.Message != "consume:27001:+50" {
		t.Fatalf("unexpected packet last-stack info: %+v", info)
	}

	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load account after packet last-stack item use: %v", err)
	}
	if got := account.Characters[1].Points[bootstrapPlayerPointValueIndex]; got != 750 {
		t.Fatalf("expected persisted points[1] to be 750 after packet last-stack item use, got %d", got)
	}
	if len(account.Characters[1].Inventory) != 0 {
		t.Fatalf("expected packet last-stack item use to clear inventory, got %#v", account.Characters[1].Inventory)
	}
	wantQuickslots := []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5}}
	if !reflect.DeepEqual(account.Characters[1].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted quickslots after packet last-stack item use: got %#v want %#v", account.Characters[1].Quickslots, wantQuickslots)
	}
}

func TestNewGameSessionFactoryItemUseOnlyExecutesThroughTalkingChat(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Points[bootstrapPlayerPointValueIndex] = 700
	characters[1].Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 3, Slot: 5}}
	characters[1].Equipment = []inventory.ItemInstance{}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("seed account store: %v", err)
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	partyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeParty, Message: "/use_item 5"})))
	if err != nil {
		t.Fatalf("unexpected party chat error: %v", err)
	}
	if len(partyOut) != 1 {
		t.Fatalf("expected /use_item over party chat to stay a normal party delivery, got %d frames", len(partyOut))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, partyOut[0]))
	if err != nil {
		t.Fatalf("decode party chat delivery: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeParty || !strings.HasSuffix(delivery.Message, "/use_item 5") {
		t.Fatalf("unexpected party chat delivery: %+v", delivery)
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if got := account.Characters[1].Points[bootstrapPlayerPointValueIndex]; got != 700 {
		t.Fatalf("expected persisted points[1] to stay unchanged after non-talking item-use attempt, got %d", got)
	}
	if !reflect.DeepEqual(account.Characters[1].Inventory, []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 3, Slot: 5}}) {
		t.Fatalf("unexpected persisted inventory after non-talking item-use attempt: %#v", account.Characters[1].Inventory)
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
	if !reflect.DeepEqual(selected, loginticket.Character{}) {
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
	if !reflect.DeepEqual(selected, loginticket.Character{}) {
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

func TestNewGameRuntimeExportsSpawnGroupsInContentBundle(t *testing.T) {
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithStoresAndTransferTriggers(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"},
		loginticket.NewFileStore(t.TempDir()),
		nil,
		staticActorStore,
		interactionStore,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.registerStaticActorWithInteractionCombatProfileAndSpawnGroupRef("Practice Wolf", 3, 1200, 2200, 101, "", "", worldruntime.StaticActorCombatProfilePracticeMob, "spawn:wolf:1"); !ok {
		t.Fatal("expected spawn-backed static actor registration to succeed")
	}

	bundle, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("unexpected content bundle export error: %v", err)
	}
	if len(bundle.StaticActors) != 0 {
		t.Fatalf("expected spawn-backed actors to be exported as spawn groups, got static actors %+v", bundle.StaticActors)
	}
	if len(bundle.SpawnGroups) != 1 {
		t.Fatalf("expected one exported spawn group, got %+v", bundle.SpawnGroups)
	}
	spawnGroup := bundle.SpawnGroups[0]
	if spawnGroup.Ref != "spawn:wolf:1" || spawnGroup.Name != "Practice Wolf" || spawnGroup.MapIndex != 3 || spawnGroup.X != 1200 || spawnGroup.Y != 2200 || spawnGroup.RaceNum != 101 || spawnGroup.CombatProfile != worldruntime.StaticActorCombatProfilePracticeMob {
		t.Fatalf("unexpected exported spawn group: %+v", spawnGroup)
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

func TestSelectedCharacterSnapshotUpdateCarriesLiveRuntimeItemStateIntoPersistedSlice(t *testing.T) {
	characters := stubCharacters()
	characters[1].Gold = 125000
	characters[1].Inventory = []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
	}
	characters[1].Equipment = []inventory.ItemInstance{
		{ID: 21, Vnum: 19, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
	}
	original := characters[1]
	runtime := player.NewRuntime(characters[1], player.SessionLink{Login: StubLogin, CharacterIndex: 1})
	if _, ok := runtime.MoveInventoryItem(5, 6); !ok {
		t.Fatal("expected inventory move to succeed")
	}
	if _, ok := runtime.EquipItem(8, inventory.EquipmentSlotBody); !ok {
		t.Fatal("expected equip to succeed")
	}
	runtime.SetLiveGold(88000)
	live := runtime.LiveCharacter()
	live.MapIndex = 42
	live.X = 1700
	live.Y = 2800

	updated, ok := selectedCharacterSnapshotUpdate(characters, 1, live)
	if !ok {
		t.Fatal("expected selected snapshot update to succeed")
	}
	if !reflect.DeepEqual(updated[1], live) {
		t.Fatalf("expected full live runtime snapshot to be persisted, got %#v want %#v", updated[1], live)
	}
	if !reflect.DeepEqual(characters[1], original) {
		t.Fatalf("expected original character slice to stay unchanged, got %#v want %#v", characters[1], original)
	}
}

func TestSelectedCharacterSnapshotUpdateRejectsInvalidSelection(t *testing.T) {
	characters := stubCharacters()
	updated, ok := selectedCharacterSnapshotUpdate(characters, 9, loginticket.Character{ID: 99})
	if ok {
		t.Fatal("expected invalid selected index to fail")
	}
	if updated != nil {
		t.Fatalf("expected no updated slice on failure, got %#v", updated)
	}
}

func TestSelectedCharacterSnapshotUpdateRejectsIdentityMismatch(t *testing.T) {
	characters := stubCharacters()
	updatedCharacter := characters[1]
	updatedCharacter.ID = 99

	updated, ok := selectedCharacterSnapshotUpdate(characters, 1, updatedCharacter)
	if ok {
		t.Fatal("expected selected snapshot update to reject mismatched identity")
	}
	if updated != nil {
		t.Fatalf("expected no updated slice on mismatch, got %#v", updated)
	}
}

func TestSelectedCharacterSnapshotUpdateClonesUpdatedItemState(t *testing.T) {
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}}
	characters[1].Equipment = []inventory.ItemInstance{{ID: 21, Vnum: 19, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	updatedCharacter := characters[1]
	updatedCharacter.X = 1700
	updatedCharacter.Y = 2800

	updated, ok := selectedCharacterSnapshotUpdate(characters, 1, updatedCharacter)
	if !ok {
		t.Fatal("expected selected snapshot update to succeed")
	}
	updatedCharacter.Inventory[0].Count = 99
	updatedCharacter.Equipment[0].Vnum = 9999

	if got := updated[1].Inventory[0].Count; got != 3 {
		t.Fatalf("expected persisted inventory clone to stay unchanged, got %d", got)
	}
	if got := updated[1].Equipment[0].Vnum; got != 19 {
		t.Fatalf("expected persisted equipment clone to stay unchanged, got %d", got)
	}
}

func TestSelectedCharacterSnapshotUpdateCarriesMerchantPurchaseStateIntoPersistedSlice(t *testing.T) {
	characters := stubCharacters()
	characters[1].Gold = 125
	characters[1].Inventory = []inventory.ItemInstance{}
	characters[1].Equipment = []inventory.ItemInstance{}
	updatedCharacter := characters[1]
	updatedCharacter.Gold = 75
	updatedCharacter.Inventory = []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 1, Slot: 0}}

	updated, ok := selectedCharacterSnapshotUpdate(characters, 1, updatedCharacter)
	if !ok {
		t.Fatal("expected selected snapshot update to carry merchant purchase state")
	}
	if got := updated[1].Gold; got != 75 {
		t.Fatalf("expected persisted gold 75 after merchant purchase state update, got %d", got)
	}
	if !reflect.DeepEqual(updated[1].Inventory, []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 1, Slot: 0}}) {
		t.Fatalf("unexpected persisted inventory after merchant purchase state update: %#v", updated[1].Inventory)
	}
	if got := characters[1].Gold; got != 125 {
		t.Fatalf("expected original character gold to stay unchanged, got %d", got)
	}
	if len(characters[1].Inventory) != 0 {
		t.Fatalf("expected original character inventory to stay empty, got %#v", characters[1].Inventory)
	}
}

func TestNewGameRuntimeExposesSelectedCharacterItemStateSnapshotsAfterSelect(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Gold = 88000
	characters[1].Inventory = []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
	}
	characters[1].Equipment = []inventory.ItemInstance{
		{ID: 21, Vnum: 19, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
	}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	flow := runtime.SessionFactory()()
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

	inventorySnapshot, ok := runtime.InventorySnapshot("MkmkSura")
	if !ok {
		t.Fatal("expected inventory snapshot to be available after select")
	}
	if inventorySnapshot.Name != "MkmkSura" || len(inventorySnapshot.Inventory) != 2 {
		t.Fatalf("unexpected inventory snapshot: %+v", inventorySnapshot)
	}
	if inventorySnapshot.Inventory[0].ID != 11 || inventorySnapshot.Inventory[0].Slot != 5 || inventorySnapshot.Inventory[1].Slot != 8 {
		t.Fatalf("unexpected inventory items: %+v", inventorySnapshot.Inventory)
	}

	equipmentSnapshot, ok := runtime.EquipmentSnapshot("MkmkSura")
	if !ok {
		t.Fatal("expected equipment snapshot to be available after select")
	}
	if equipmentSnapshot.Name != "MkmkSura" || len(equipmentSnapshot.Equipment) != 1 {
		t.Fatalf("unexpected equipment snapshot: %+v", equipmentSnapshot)
	}
	if equipmentSnapshot.Equipment[0].ID != 21 || equipmentSnapshot.Equipment[0].EquipSlot != inventory.EquipmentSlotWeapon.String() {
		t.Fatalf("unexpected equipment items: %+v", equipmentSnapshot.Equipment)
	}

	currencySnapshot, ok := runtime.CurrencySnapshot("MkmkSura")
	if !ok {
		t.Fatal("expected currency snapshot to be available after select")
	}
	if currencySnapshot.Name != "MkmkSura" || currencySnapshot.Gold != 88000 {
		t.Fatalf("unexpected currency snapshot: %+v", currencySnapshot)
	}
	if _, ok := runtime.InventorySnapshot("Unknown"); ok {
		t.Fatal("expected unknown character inventory snapshot lookup to fail")
	}
}

func TestNewGameSessionFactoryItemMovePartialMergeKeepsSourceQuickslotStable(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 5, Slot: 5},
		{ID: 12, Vnum: 27001, Count: 198, Slot: 8},
	}
	characters[1].Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("save account: %v", err)
	}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}

	flow := newStartedGameFlow(t, store, accounts)
	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(8),
		Count:       2,
	})))
	if err != nil {
		t.Fatalf("unexpected item move error: %v", err)
	}
	if len(moveOut) != 2 {
		t.Fatalf("expected source and destination ITEM_UPDATE frames without quickslot mutation, got %d frames", len(moveOut))
	}
	sourceUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, moveOut[0]))
	if err != nil {
		t.Fatalf("decode source update: %v", err)
	}
	if sourceUpdate.Position != itemproto.InventoryPosition(5) || sourceUpdate.Count != 3 {
		t.Fatalf("expected source slot 5 count 3 update, got %+v", sourceUpdate)
	}
	destinationUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, moveOut[1]))
	if err != nil {
		t.Fatalf("decode destination update: %v", err)
	}
	if destinationUpdate.Position != itemproto.InventoryPosition(8) || destinationUpdate.Count != 200 {
		t.Fatalf("expected destination slot 8 count 200 update, got %+v", destinationUpdate)
	}

	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if got := account.Characters[1].Quickslots; !reflect.DeepEqual(got, characters[1].Quickslots) {
		t.Fatalf("partial merge should keep source quickslot stable, got %#v", got)
	}
	if !reflect.DeepEqual(account.Characters[1].Inventory, []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 12, Vnum: 27001, Count: 200, Slot: 8},
	}) {
		t.Fatalf("unexpected persisted inventory after partial counted merge: %#v", account.Characters[1].Inventory)
	}
}

func TestNewGameSessionFactoryItemUseToItemRejectsLockedSourceOrTargetWithoutMutation(t *testing.T) {
	cases := []struct {
		name      string
		inventory []inventory.ItemInstance
	}{
		{
			name: "locked source",
			inventory: []inventory.ItemInstance{
				{ID: 11, Vnum: 27001, Count: 3, Slot: 5, Locked: true},
				{ID: 12, Vnum: 27001, Count: 2, Slot: 8},
			},
		},
		{
			name: "locked target",
			inventory: []inventory.ItemInstance{
				{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
				{ID: 12, Vnum: 27001, Count: 2, Slot: 8, Locked: true},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			characters := stubCharacters()
			characters[1].Inventory = append([]inventory.ItemInstance(nil), tc.inventory...)
			characters[1].Quickslots = []loginticket.Quickslot{
				{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
				{Position: 3, Type: quickslotproto.TypeItem, Slot: 8},
			}
			if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
				t.Fatalf("save account: %v", err)
			}
			if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
				t.Fatalf("issue login ticket: %v", err)
			}
			flow := newStartedGameFlow(t, store, accounts)

			mergeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
				Source: itemproto.InventoryPosition(5),
				Target: itemproto.InventoryPosition(8),
			})))
			if err != nil {
				t.Fatalf("unexpected locked %s use-to-item error: %v", tc.name, err)
			}
			if len(mergeOut) != 0 {
				t.Fatalf("expected locked %s use-to-item to fail closed with no frames, got %d", tc.name, len(mergeOut))
			}
			account, err := accounts.Load(StubLogin)
			if err != nil {
				t.Fatalf("load persisted account: %v", err)
			}
			if !reflect.DeepEqual(account.Characters[1].Inventory, characters[1].Inventory) {
				t.Fatalf("unexpected persisted inventory after locked %s use-to-item attempt: %#v", tc.name, account.Characters[1].Inventory)
			}
			if !reflect.DeepEqual(account.Characters[1].Quickslots, characters[1].Quickslots) {
				t.Fatalf("unexpected persisted quickslots after locked %s use-to-item attempt: %#v", tc.name, account.Characters[1].Quickslots)
			}
		})
	}
}

func TestNewGameSessionFactoryItemUseToItemRejectsNonStackableTemplateWithoutMutation(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 12, Vnum: 27001, Count: 1, Slot: 8},
	}
	characters[1].Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 8},
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("save account: %v", err)
	}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	itemTemplates := staticItemTemplateStore{snapshot: itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Non-Stackable Potion-Like Item",
		Stackable: false,
		MaxCount:  1,
	}}}}
	flow := newStartedGameFlowWithItemStore(t, store, accounts, itemTemplates)

	mergeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
		Source: itemproto.InventoryPosition(5),
		Target: itemproto.InventoryPosition(8),
	})))
	if err != nil {
		t.Fatalf("unexpected non-stackable use-to-item error: %v", err)
	}
	if len(mergeOut) != 0 {
		t.Fatalf("expected non-stackable use-to-item to fail closed with no frames, got %d", len(mergeOut))
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[1].Inventory, characters[1].Inventory) {
		t.Fatalf("unexpected persisted inventory after non-stackable use-to-item attempt: %#v", account.Characters[1].Inventory)
	}
	if !reflect.DeepEqual(account.Characters[1].Quickslots, characters[1].Quickslots) {
		t.Fatalf("unexpected persisted quickslots after non-stackable use-to-item attempt: %#v", account.Characters[1].Quickslots)
	}
}

func TestNewGameSessionFactoryItemUseToItemRejectsAuthoredSexAntiFlagWithoutMutation(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 12, Vnum: 27001, Count: 2, Slot: 8},
	}
	characters[1].Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 8},
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("save account: %v", err)
	}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	itemTemplates := staticItemTemplateStore{snapshot: itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:       27001,
		Name:       "Female-Restricted Potion Stack",
		Stackable:  true,
		MaxCount:   200,
		AntiFemale: true,
	}}}}
	flow := newStartedGameFlowWithItemStore(t, store, accounts, itemTemplates)

	mergeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
		Source: itemproto.InventoryPosition(5),
		Target: itemproto.InventoryPosition(8),
	})))
	if err != nil {
		t.Fatalf("unexpected anti-flag item use-to-item error: %v", err)
	}
	if len(mergeOut) != 0 {
		t.Fatalf("expected authored anti-female use-to-item to fail closed with no frames, got %d", len(mergeOut))
	}
	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[1].Inventory, characters[1].Inventory) {
		t.Fatalf("unexpected persisted inventory after anti-flag use-to-item attempt: %#v", account.Characters[1].Inventory)
	}
	if !reflect.DeepEqual(account.Characters[1].Quickslots, characters[1].Quickslots) {
		t.Fatalf("unexpected persisted quickslots after anti-flag use-to-item attempt: %#v", account.Characters[1].Quickslots)
	}
}

func TestNewGameSessionFactoryItemUseToItemFullMergeDeletesSourceQuickslotAndPersists(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 12, Vnum: 27001, Count: 2, Slot: 8},
	}
	characters[1].Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 8},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: cloneCharacters(characters)}); err != nil {
		t.Fatalf("save account: %v", err)
	}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}

	flow := newStartedGameFlow(t, store, accounts)
	mergeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
		Source: itemproto.InventoryPosition(5),
		Target: itemproto.InventoryPosition(8),
	})))
	if err != nil {
		t.Fatalf("unexpected item use-to-item error: %v", err)
	}
	if len(mergeOut) != 3 {
		t.Fatalf("expected source delete, target set, and source quickslot delete, got %d frames", len(mergeOut))
	}
	sourceDel, err := itemproto.DecodeDel(decodeSingleFrame(t, mergeOut[0]))
	if err != nil {
		t.Fatalf("decode source delete: %v", err)
	}
	if sourceDel.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("expected source slot 5 delete, got %+v", sourceDel.Position)
	}
	targetSet, err := itemproto.DecodeSet(decodeSingleFrame(t, mergeOut[1]))
	if err != nil {
		t.Fatalf("decode target set: %v", err)
	}
	if targetSet.Position != itemproto.InventoryPosition(8) || targetSet.Vnum != 27001 || targetSet.Count != 5 {
		t.Fatalf("expected target slot 8 count 5 set, got %+v", targetSet)
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, mergeOut[2]))
	if err != nil {
		t.Fatalf("decode source quickslot delete: %v", err)
	}
	if quickslotDel.Position != 2 {
		t.Fatalf("expected source item quickslot position 2 delete, got %+v", quickslotDel)
	}

	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[1].Inventory, []inventory.ItemInstance{{ID: 12, Vnum: 27001, Count: 5, Slot: 8}}) {
		t.Fatalf("unexpected persisted inventory after use-to-item full merge: %#v", account.Characters[1].Inventory)
	}
	if !reflect.DeepEqual(account.Characters[1].Quickslots, []loginticket.Quickslot{
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 8},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}) {
		t.Fatalf("unexpected persisted quickslots after use-to-item full merge: %#v", account.Characters[1].Quickslots)
	}
}

func TestNewGameSessionFactoryItemMoveExactCountMergesFullStackIntoCompatibleDestination(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accountDir := t.TempDir()
	accounts := accountstore.NewFileStore(accountDir)
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 12, Vnum: 27001, Count: 2, Slot: 8},
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("save account: %v", err)
	}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	flow := runtime.SessionFactory()()
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, frame.Encode(worldproto.HeaderEnterGame, nil))); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	moveRaw := itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(8),
		Count:       3,
	})
	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, moveRaw))
	if err != nil {
		t.Fatalf("unexpected item move error: %v", err)
	}
	if len(moveOut) != 2 {
		t.Fatalf("expected source delete plus destination update, got %d frames", len(moveOut))
	}
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, moveOut[0]))
	if err != nil {
		t.Fatalf("decode source delete: %v", err)
	}
	if del.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("expected source slot 5 delete, got %+v", del.Position)
	}
	update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, moveOut[1]))
	if err != nil {
		t.Fatalf("decode destination update: %v", err)
	}
	if update.Position != itemproto.InventoryPosition(8) || update.Count != 5 {
		t.Fatalf("expected destination slot 8 count 5 update, got %+v", update)
	}

	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[1].Inventory, []inventory.ItemInstance{{ID: 12, Vnum: 27001, Count: 5, Slot: 8}}) {
		t.Fatalf("unexpected persisted inventory after exact counted merge: %#v", account.Characters[1].Inventory)
	}
}

func TestNewGameSessionFactoryItemMoveExactCountSwapsIncompatibleFullStackDestination(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accountDir := t.TempDir()
	accounts := accountstore.NewFileStore(accountDir)
	characters := stubCharacters()
	characters[1].Inventory = []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
	}
	if err := accounts.Save(accountstore.Account{Login: StubLogin, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("save account: %v", err)
	}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	flow := runtime.SessionFactory()()
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
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, frame.Encode(worldproto.HeaderEnterGame, nil))); err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}

	moveRaw := itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(8),
		Count:       3,
	})
	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, moveRaw))
	if err != nil {
		t.Fatalf("unexpected item move error: %v", err)
	}
	if len(moveOut) != 2 {
		t.Fatalf("expected source and destination item set frames for incompatible full-stack swap, got %d frames", len(moveOut))
	}
	sourceSet, err := itemproto.DecodeSet(decodeSingleFrame(t, moveOut[0]))
	if err != nil {
		t.Fatalf("decode source set: %v", err)
	}
	if sourceSet.Position != itemproto.InventoryPosition(5) || sourceSet.Vnum != 1120 || sourceSet.Count != 1 {
		t.Fatalf("expected destination item to move into source slot, got %+v", sourceSet)
	}
	destinationSet, err := itemproto.DecodeSet(decodeSingleFrame(t, moveOut[1]))
	if err != nil {
		t.Fatalf("decode destination set: %v", err)
	}
	if destinationSet.Position != itemproto.InventoryPosition(8) || destinationSet.Vnum != 27001 || destinationSet.Count != 3 {
		t.Fatalf("expected source stack to move into destination slot, got %+v", destinationSet)
	}

	account, err := accounts.Load(StubLogin)
	if err != nil {
		t.Fatalf("load persisted account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[1].Inventory, []inventory.ItemInstance{
		{ID: 12, Vnum: 1120, Count: 1, Slot: 5},
		{ID: 11, Vnum: 27001, Count: 3, Slot: 8},
	}) {
		t.Fatalf("unexpected persisted inventory after exact counted incompatible full-stack swap: %#v", account.Characters[1].Inventory)
	}
}

func TestNewGameRuntimeClearsSelectedCharacterItemStateSnapshotsOnClose(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	characters := stubCharacters()
	characters[1].Gold = 88000
	characters[1].Inventory = []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}}
	characters[1].Equipment = []inventory.ItemInstance{{ID: 21, Vnum: 19, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	if err := store.Issue(loginticket.Ticket{Login: StubLogin, LoginKey: 0x01020304, Empire: 2, Characters: characters}); err != nil {
		t.Fatalf("issue login ticket: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	flow := runtime.SessionFactory()()
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
	if _, ok := runtime.InventorySnapshot("MkmkSura"); !ok {
		t.Fatal("expected inventory snapshot to exist before close")
	}
	closer, ok := flow.(interface{ Close() error })
	if !ok {
		t.Fatal("expected session flow to expose Close")
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("close session flow: %v", err)
	}
	if _, ok := runtime.InventorySnapshot("MkmkSura"); ok {
		t.Fatal("expected inventory snapshot to be removed after close")
	}
	if _, ok := runtime.EquipmentSnapshot("MkmkSura"); ok {
		t.Fatal("expected equipment snapshot to be removed after close")
	}
	if _, ok := runtime.CurrencySnapshot("MkmkSura"); ok {
		t.Fatal("expected currency snapshot to be removed after close")
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

func TestItemUseToItemQuickslotSyncDeletesConsumedSourceSlot(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}, {ID: 12, Vnum: 27001, Count: 7, Slot: 6}},
		Quickslots: []loginticket.Quickslot{
			{Position: 3, Type: quickslotproto.TypeItem, Slot: 5},
			{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
			{Position: 5, Type: quickslotproto.TypeSkill, Slot: 5},
		},
	}
	selectedPlayer := player.NewRuntime(persisted, player.SessionLink{Login: "peer-two", CharacterIndex: 1})
	mergeResult := inventory.MoveResult{
		Changed:    true,
		From:       5,
		To:         6,
		ToOccupied: true,
		ToItem:     inventory.ItemInstance{ID: 12, Vnum: 27001, Count: 10, Slot: 6},
	}

	frames, ok := itemUseToItemQuickslotSyncFrames(selectedPlayer, mergeResult)
	if !ok {
		t.Fatal("expected use-to-item quickslot sync to succeed")
	}
	if len(frames) != 1 {
		t.Fatalf("expected one quickslot delete for consumed source stack, got %d", len(frames))
	}
	deleted, err := quickslotproto.DecodeDel(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode quickslot delete: %v", err)
	}
	if deleted.Position != 3 {
		t.Fatalf("expected source item quickslot position 3 to be deleted, got %d", deleted.Position)
	}
	if got := selectedPlayer.LiveQuickslots(); !reflect.DeepEqual(got, []loginticket.Quickslot{
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 5, Type: quickslotproto.TypeSkill, Slot: 5},
	}) {
		t.Fatalf("unexpected live quickslots after use-to-item full merge: %#v", got)
	}
}

func TestItemUseToItemQuickslotSyncPreservesChangedTargetSlotOnPartialMerge(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030103,
		VID:       0x02040103,
		Name:      "PeerThree",
		Inventory: []inventory.ItemInstance{{ID: 21, Vnum: 27001, Count: 7, Slot: 5}, {ID: 22, Vnum: 27001, Count: 8, Slot: 6}},
		Quickslots: []loginticket.Quickslot{
			{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
			{Position: 7, Type: quickslotproto.TypeItem, Slot: 6},
			{Position: 8, Type: quickslotproto.TypeItem, Slot: 6},
			{Position: 9, Type: quickslotproto.TypeSkill, Slot: 6},
		},
	}
	selectedPlayer := player.NewRuntime(persisted, player.SessionLink{Login: "peer-three", CharacterIndex: 1})
	mergeResult := inventory.MoveResult{
		Changed:      true,
		From:         5,
		To:           6,
		FromOccupied: true,
		FromItem:     inventory.ItemInstance{ID: 21, Vnum: 27001, Count: 5, Slot: 5},
		ToOccupied:   true,
		ToItem:       inventory.ItemInstance{ID: 22, Vnum: 27001, Count: 10, Slot: 6},
		CountOnly:    true,
	}

	frames, ok := itemUseToItemQuickslotSyncFrames(selectedPlayer, mergeResult)
	if !ok {
		t.Fatal("expected partial use-to-item quickslot sync to succeed")
	}
	if len(frames) != 0 {
		t.Fatalf("expected no quickslot deletes for changed target count-only stack, got %d", len(frames))
	}
	wantLiveQuickslots := []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 7, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 8, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 9, Type: quickslotproto.TypeSkill, Slot: 6},
	}
	if got := selectedPlayer.LiveQuickslots(); !reflect.DeepEqual(got, wantLiveQuickslots) {
		t.Fatalf("unexpected live quickslots after partial use-to-item merge: got %#v want %#v", got, wantLiveQuickslots)
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
