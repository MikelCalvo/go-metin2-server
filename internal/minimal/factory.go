package minimal

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	authflow "github.com/MikelCalvo/go-metin2-server/internal/auth"
	"github.com/MikelCalvo/go-metin2-server/internal/authboot"
	"github.com/MikelCalvo/go-metin2-server/internal/boot"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	gameflow "github.com/MikelCalvo/go-metin2-server/internal/game"
	"github.com/MikelCalvo/go-metin2-server/internal/handshake"
	loginflow "github.com/MikelCalvo/go-metin2-server/internal/login"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/player"
	authproto "github.com/MikelCalvo/go-metin2-server/internal/proto/auth"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/securecipher"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/warp"
	worldentry "github.com/MikelCalvo/go-metin2-server/internal/worldentry"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

const (
	StubLogin    = "mkmk"
	StubPassword = "hunter2"
)

const bootstrapPlayerPointType uint8 = 1
const bootstrapPlayerPointValueIndex = 1
const bootstrapMapIndex uint32 = 1
const bootstrapShinsooYonganStartX int32 = 469300
const bootstrapShinsooYonganStartY int32 = 964200
const legacyFakeStubMkmkWarX int32 = 1000
const legacyFakeStubMkmkWarY int32 = 2000

var (
	ErrInvalidLegacyAddr           = errors.New("invalid legacy addr")
	ErrInvalidPublicAddr           = errors.New("invalid public addr")
	ErrInvalidVisibilityMode       = errors.New("invalid visibility mode")
	ErrInvalidVisibilityRadius     = errors.New("invalid visibility radius")
	ErrInvalidVisibilitySectorSize = errors.New("invalid visibility sector size")
)

type loginKeyGenerator func() (uint32, error)

type sharedWorldSessionRelocator func(mapIndex uint32, x int32, y int32) (RelocationPreview, bool)

type bootstrapTransferTrigger struct {
	SourceMapIndex uint32
	SourceX        int32
	SourceY        int32
	TargetMapIndex uint32
	TargetX        int32
	TargetY        int32
}

type ConnectedCharacterSnapshot = worldruntime.ConnectedCharacterSnapshot

type CharacterVisibilitySnapshot = worldruntime.CharacterVisibilitySnapshot

type MapOccupancySnapshot = worldruntime.MapOccupancySnapshot

type StaticActorSnapshot = worldruntime.StaticActorSnapshot

type RuntimeConfigSnapshot struct {
	LocalChannelID       uint8  `json:"local_channel_id"`
	VisibilityMode       string `json:"visibility_mode"`
	VisibilityRadius     int32  `json:"visibility_radius"`
	VisibilitySectorSize int32  `json:"visibility_sector_size"`
}

type MapOccupancyChange = worldruntime.MapOccupancyChange

type RelocationPreview = worldruntime.RelocationPreview

type gameRuntime struct {
	sessionFactory service.SessionFactory
	sharedWorld    *sharedWorldRegistry
	staticStore    staticstore.Store
	staticActorMu  sync.Mutex
}

func NewGameRuntime(cfg config.Service) (*gameRuntime, error) {
	return newGameRuntimeWithStoresAndTransferTriggers(
		cfg,
		loginticket.NewFileStore(defaultTicketStoreDir()),
		accountstore.NewFileStore(defaultAccountStoreDir()),
		staticstore.NewFileStore(defaultStaticActorStorePath()),
		nil,
	)
}

func (r *gameRuntime) SessionFactory() service.SessionFactory {
	if r == nil {
		return nil
	}
	return r.sessionFactory
}

func (r *gameRuntime) BroadcastNotice(message string) int {
	if r == nil || r.sharedWorld == nil {
		return 0
	}
	return r.sharedWorld.EnqueueSystemNotice(message)
}

func (r *gameRuntime) RelocateCharacter(name string, mapIndex uint32, x int32, y int32) bool {
	_, ok := r.TransferCharacter(name, mapIndex, x, y)
	return ok
}

func (r *gameRuntime) TransferCharacter(name string, mapIndex uint32, x int32, y int32) (RelocationPreview, bool) {
	if r == nil || r.sharedWorld == nil {
		return RelocationPreview{}, false
	}
	return r.sharedWorld.TransferCharacter(name, mapIndex, x, y)
}

func (r *gameRuntime) PreviewRelocation(name string, mapIndex uint32, x int32, y int32) (RelocationPreview, bool) {
	if r == nil || r.sharedWorld == nil {
		return RelocationPreview{}, false
	}
	return r.sharedWorld.PreviewRelocation(name, mapIndex, x, y)
}

func (r *gameRuntime) ConnectedCharacters() []ConnectedCharacterSnapshot {
	if r == nil || r.sharedWorld == nil {
		return nil
	}
	return r.sharedWorld.ConnectedCharacters()
}

func (r *gameRuntime) CharacterVisibility() []CharacterVisibilitySnapshot {
	if r == nil || r.sharedWorld == nil {
		return nil
	}
	return r.sharedWorld.CharacterVisibility()
}

func (r *gameRuntime) MapOccupancy() []MapOccupancySnapshot {
	if r == nil || r.sharedWorld == nil {
		return nil
	}
	return r.sharedWorld.MapOccupancy()
}

func (r *gameRuntime) RuntimeConfigSnapshot() RuntimeConfigSnapshot {
	if r == nil || r.sharedWorld == nil {
		return RuntimeConfigSnapshot{}
	}
	topology := r.sharedWorld.topology
	snapshot := RuntimeConfigSnapshot{
		LocalChannelID: topology.LocalChannelID(),
		VisibilityMode: "whole_map",
	}
	switch policy := topology.VisibilityPolicy().(type) {
	case worldruntime.RadiusVisibilityPolicy:
		snapshot.VisibilityMode = "radius"
		snapshot.VisibilityRadius = policy.Radius
		snapshot.VisibilitySectorSize = policy.SectorSize
	case worldruntime.WholeMapVisibilityPolicy:
		// keep defaults
	default:
		snapshot.VisibilityMode = "custom"
	}
	return snapshot
}

func (r *gameRuntime) RegisterStaticActor(name string, mapIndex uint32, x int32, y int32, raceNum uint32) (StaticActorSnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return StaticActorSnapshot{}, false
	}
	name = strings.TrimSpace(name)
	if name == "" || mapIndex == 0 || raceNum == 0 {
		return StaticActorSnapshot{}, false
	}

	r.staticActorMu.Lock()
	defer r.staticActorMu.Unlock()

	current := r.sharedWorld.StaticActors()
	nextEntityID := r.sharedWorld.NextStaticActorEntityID()
	if nextEntityID == 0 {
		return StaticActorSnapshot{}, false
	}
	target := appendStaticActorSnapshot(current, StaticActorSnapshot{EntityID: nextEntityID, Name: name, MapIndex: mapIndex, X: x, Y: y, RaceNum: raceNum})
	if !r.persistStaticActorSnapshot(target) {
		return StaticActorSnapshot{}, false
	}
	registered, ok := r.sharedWorld.RegisterStaticActorWithEntityID(nextEntityID, name, mapIndex, x, y, raceNum)
	if !ok {
		_ = r.persistStaticActorSnapshot(current)
		return StaticActorSnapshot{}, false
	}
	return registered, true
}

func (r *gameRuntime) UpdateStaticActor(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32) (StaticActorSnapshot, bool) {
	if r == nil || r.sharedWorld == nil || entityID == 0 {
		return StaticActorSnapshot{}, false
	}
	name = strings.TrimSpace(name)
	if name == "" || mapIndex == 0 || raceNum == 0 {
		return StaticActorSnapshot{}, false
	}

	r.staticActorMu.Lock()
	defer r.staticActorMu.Unlock()

	current := r.sharedWorld.StaticActors()
	idx := staticActorSnapshotIndex(current, entityID)
	if idx == -1 {
		return StaticActorSnapshot{}, false
	}
	target := cloneStaticActorSnapshots(current)
	target[idx] = StaticActorSnapshot{EntityID: entityID, Name: name, MapIndex: mapIndex, X: x, Y: y, RaceNum: raceNum}
	if !r.persistStaticActorSnapshot(target) {
		return StaticActorSnapshot{}, false
	}
	updated, ok := r.sharedWorld.UpdateStaticActor(entityID, name, mapIndex, x, y, raceNum)
	if !ok {
		_ = r.persistStaticActorSnapshot(current)
		return StaticActorSnapshot{}, false
	}
	return updated, true
}

func (r *gameRuntime) StaticActors() []StaticActorSnapshot {
	if r == nil || r.sharedWorld == nil {
		return nil
	}
	return r.sharedWorld.StaticActors()
}

func (r *gameRuntime) RemoveStaticActor(entityID uint64) (StaticActorSnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return StaticActorSnapshot{}, false
	}

	r.staticActorMu.Lock()
	defer r.staticActorMu.Unlock()

	current := r.sharedWorld.StaticActors()
	idx := staticActorSnapshotIndex(current, entityID)
	if idx == -1 {
		return StaticActorSnapshot{}, false
	}
	target := append(cloneStaticActorSnapshots(current[:idx]), cloneStaticActorSnapshots(current[idx+1:])...)
	if !r.persistStaticActorSnapshot(target) {
		return StaticActorSnapshot{}, false
	}
	removed, ok := r.sharedWorld.RemoveStaticActor(entityID)
	if !ok {
		_ = r.persistStaticActorSnapshot(current)
		return StaticActorSnapshot{}, false
	}
	return removed, true
}

func NewAuthSessionFactory() service.SessionFactory {
	return newAuthSessionFactoryWithAccountStore(
		loginticket.NewFileStore(defaultTicketStoreDir()),
		accountstore.NewFileStore(defaultAccountStoreDir()),
		randomLoginKey,
	)
}

func newAuthSessionFactory(store loginticket.Store, generateLoginKey loginKeyGenerator) service.SessionFactory {
	return newAuthSessionFactoryWithAccountStore(store, nil, generateLoginKey)
}

func newAuthSessionFactoryWithAccountStore(store loginticket.Store, accounts accountstore.Store, generateLoginKey loginKeyGenerator) service.SessionFactory {
	if store == nil {
		store = loginticket.NewFileStore(defaultTicketStoreDir())
	}
	if generateLoginKey == nil {
		generateLoginKey = randomLoginKey
	}

	return func() service.SessionFlow {
		return authboot.NewFlow(authboot.Config{
			Handshake: handshake.Config{
				SecureSession: securecipher.NewServerSession(securecipher.ServerConfig{
					Random:     rand.Reader,
					ServerTime: currentServerTimeMillis,
				}),
			},
			Auth: authflow.Config{
				Authenticate: func(packet authproto.Login3Packet) authflow.Result {
					if packet.Login != StubLogin || packet.Password != StubPassword {
						return authflow.Result{Accepted: false, FailureStatus: "WRONGPWD"}
					}

					account, ok := loadOrCreateAccount(accounts, packet.Login)
					if !ok {
						return authflow.Result{Accepted: false, FailureStatus: "FAILED"}
					}
					loginKey, ok := issueLoginTicket(store, account.Login, account.Empire, account.Characters, generateLoginKey)
					if !ok {
						return authflow.Result{Accepted: false, FailureStatus: "FAILED"}
					}

					return authflow.Result{Accepted: true, LoginKey: loginKey}
				},
			},
		})
	}
}

func NewGameSessionFactory(cfg config.Service) (service.SessionFactory, error) {
	runtime, err := newGameRuntimeWithAccountStore(
		cfg,
		loginticket.NewFileStore(defaultTicketStoreDir()),
		accountstore.NewFileStore(defaultAccountStoreDir()),
	)
	if err != nil {
		return nil, err
	}
	return runtime.SessionFactory(), nil
}

func newGameSessionFactory(cfg config.Service, store loginticket.Store) (service.SessionFactory, error) {
	runtime, err := newGameRuntimeWithAccountStore(cfg, store, nil)
	if err != nil {
		return nil, err
	}
	return runtime.SessionFactory(), nil
}

func newGameSessionFactoryWithAccountStore(cfg config.Service, store loginticket.Store, accounts accountstore.Store) (service.SessionFactory, error) {
	runtime, err := newGameRuntimeWithAccountStore(cfg, store, accounts)
	if err != nil {
		return nil, err
	}
	return runtime.SessionFactory(), nil
}

func newGameRuntimeWithAccountStore(cfg config.Service, store loginticket.Store, accounts accountstore.Store) (*gameRuntime, error) {
	return newGameRuntimeWithStoresAndTransferTriggers(cfg, store, accounts, nil, nil)
}

func newGameRuntimeWithAccountStoreAndStaticStore(cfg config.Service, store loginticket.Store, accounts accountstore.Store, staticActors staticstore.Store) (*gameRuntime, error) {
	return newGameRuntimeWithStoresAndTransferTriggers(cfg, store, accounts, staticActors, nil)
}

func bootstrapTopologyFromConfig(cfg config.Service) (worldruntime.BootstrapTopology, error) {
	topology := worldruntime.NewBootstrapTopology(0)
	mode := strings.TrimSpace(strings.ToLower(cfg.VisibilityMode))
	mode = strings.ReplaceAll(mode, "-", "_")
	if mode == "" {
		mode = "whole_map"
	}

	switch mode {
	case "whole_map":
		return topology.WithWholeMapVisibilityPolicy(), nil
	case "radius":
		if cfg.VisibilityRadius <= 0 {
			return worldruntime.BootstrapTopology{}, ErrInvalidVisibilityRadius
		}
		if cfg.VisibilitySectorSize <= 0 {
			return worldruntime.BootstrapTopology{}, ErrInvalidVisibilitySectorSize
		}
		return topology.WithRadiusVisibilityPolicy(cfg.VisibilityRadius, cfg.VisibilitySectorSize), nil
	default:
		return worldruntime.BootstrapTopology{}, ErrInvalidVisibilityMode
	}
}

func newGameRuntimeWithAccountStoreAndTransferTriggers(cfg config.Service, store loginticket.Store, accounts accountstore.Store, transferTriggers []bootstrapTransferTrigger) (*gameRuntime, error) {
	return newGameRuntimeWithStoresAndTransferTriggers(cfg, store, accounts, nil, transferTriggers)
}

func newGameRuntimeWithStoresAndTransferTriggers(cfg config.Service, store loginticket.Store, accounts accountstore.Store, staticActors staticstore.Store, transferTriggers []bootstrapTransferTrigger) (*gameRuntime, error) {
	advertisedPort, err := parsePort(cfg.LegacyAddr)
	if err != nil {
		return nil, err
	}

	advertisedAddr, err := parseIPv4(cfg.PublicAddr)
	if err != nil {
		return nil, err
	}

	topology, err := bootstrapTopologyFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	if store == nil {
		store = loginticket.NewFileStore(defaultTicketStoreDir())
	}
	sharedWorld := newSharedWorldRegistryWithTopology(topology)
	runtime := &gameRuntime{sharedWorld: sharedWorld, staticStore: staticActors}
	if err := runtime.loadPersistedStaticActors(); err != nil {
		return nil, err
	}
	transferTriggers = cloneBootstrapTransferTriggers(transferTriggers)

	runtime.sessionFactory = func() service.SessionFlow {
		var sessionTicket loginticket.Ticket
		var hasTicket bool
		var selectedIndex uint8
		var hasSelected bool
		var selectedPlayer *player.Runtime
		var stateMu sync.Mutex
		pending := newPendingServerFrames()
		var sharedWorldID uint64
		var joinedSharedWorld bool
		refreshSelectedPlayerFromAccountSnapshot := func() bool {
			if accounts == nil {
				return true
			}
			if !hasTicket || !hasSelected {
				return false
			}
			account, ok := loadOrCreateAccount(accounts, sessionTicket.Login)
			if !ok {
				selectedPlayer = nil
				return false
			}
			sessionTicket.Empire = account.Empire
			sessionTicket.Characters = cloneCharacters(account.Characters)
			if int(selectedIndex) >= len(sessionTicket.Characters) {
				selectedPlayer = nil
				return false
			}
			selected := sessionTicket.Characters[selectedIndex]
			if selected.ID == 0 {
				selectedPlayer = nil
				return false
			}
			if selectedPlayer == nil {
				selectedPlayer = player.NewRuntime(selected, player.SessionLink{Login: sessionTicket.Login, CharacterIndex: selectedIndex})
			} else {
				selectedPlayer.ApplyPersistedSnapshot(selected)
			}
			return true
		}
		currentSelectedPlayer := func() (*player.Runtime, bool) {
			if !hasTicket || !hasSelected || int(selectedIndex) >= len(sessionTicket.Characters) {
				return nil, false
			}
			if !joinedSharedWorld && !refreshSelectedPlayerFromAccountSnapshot() {
				return nil, false
			}
			if selectedPlayer != nil {
				return selectedPlayer, true
			}
			selected := sessionTicket.Characters[selectedIndex]
			if selected.ID == 0 {
				return nil, false
			}
			selectedPlayer = player.NewRuntime(selected, player.SessionLink{Login: sessionTicket.Login, CharacterIndex: selectedIndex})
			return selectedPlayer, true
		}
		ownsLiveSharedWorldSession := func() bool {
			return joinedSharedWorld && sharedWorldID != 0 && sharedWorld.HasLiveSession(sharedWorldID)
		}
		applySelectedCharacterTransfer := func(mapIndex uint32, x int32, y int32, rebootstrap bool) (RelocationPreview, [][]byte, bool) {
			selectedPlayer, ok := currentSelectedPlayer()
			if !ok || !joinedSharedWorld || sharedWorldID == 0 {
				return RelocationPreview{}, nil, false
			}
			selected := selectedPlayer.PersistedSnapshot()
			selectedLink := selectedPlayer.SessionLink()
			buildUpdatedSelection := func(updated loginticket.Character) ([]loginticket.Character, loginticket.Character, bool) {
				return selectedCharacterLocationUpdate(sessionTicket.Characters, selectedLink.CharacterIndex, updated.MapIndex, updated.X, updated.Y)
			}
			var transferResult RelocationPreview
			var transferFrames [][]byte
			transferFlow := warp.NewFlow(warp.Config{
				Persist: func(updated loginticket.Character) bool {
					updatedCharacters, _, ok := buildUpdatedSelection(updated)
					if !ok {
						return false
					}
					return saveAccountSnapshot(accounts, sessionTicket.Login, sessionTicket.Empire, updatedCharacters)
				},
				Rollback: func(previous loginticket.Character) bool {
					return saveAccountSnapshot(accounts, sessionTicket.Login, sessionTicket.Empire, sessionTicket.Characters)
				},
				Commit: func(updated loginticket.Character) (warp.Result, bool) {
					updatedCharacters, updatedSelected, ok := buildUpdatedSelection(updated)
					if !ok {
						return warp.Result{}, false
					}
					if rebootstrap {
						bootstrapFrames, err := worldentry.BuildBootstrapFrames(updatedSelected)
						if err != nil {
							return warp.Result{}, false
						}
						transferPreview, originFrames, ok := sharedWorld.TransferWithOriginFrames(sharedWorldID, updatedSelected)
						if !ok {
							return warp.Result{}, false
						}
						transferResult = transferPreview
						transferFrames = append(append([][]byte(nil), bootstrapFrames...), originFrames...)
					} else {
						transferPreview, ok := sharedWorld.Transfer(sharedWorldID, updatedSelected)
						if !ok {
							return warp.Result{}, false
						}
						transferResult = transferPreview
					}
					sessionTicket.Characters = updatedCharacters
					selectedPlayer.ApplyPersistedSnapshot(updatedSelected)
					return warp.Result{Applied: true, Updated: selectedPlayer.LiveCharacter()}, true
				},
			})
			if _, ok := transferFlow.Apply(selected, warp.Target{MapIndex: mapIndex, X: x, Y: y}); !ok {
				return RelocationPreview{}, nil, false
			}
			return transferResult, transferFrames, true
		}
		applySelectedCharacterPosition := func(selectedPlayer *player.Runtime, x int32, y int32, persist bool) (loginticket.Character, bool) {
			if selectedPlayer == nil {
				return loginticket.Character{}, false
			}
			if !persist {
				selected := selectedPlayer.LiveCharacter()
				if selected.ID == 0 {
					return loginticket.Character{}, false
				}
				selectedPlayer.SetLivePosition(selected.MapIndex, x, y)
				return selectedPlayer.LiveCharacter(), true
			}
			updatedCharacters, updatedSelected, ok := updateSelectedCharacterPosition(accounts, sessionTicket.Login, sessionTicket.Empire, sessionTicket.Characters, selectedPlayer.SessionLink().CharacterIndex, x, y)
			if !ok {
				return loginticket.Character{}, false
			}
			sessionTicket.Characters = updatedCharacters
			selectedPlayer.ApplyPersistedSnapshot(updatedSelected)
			return selectedPlayer.LiveCharacter(), true
		}

		inner := boot.NewFlow(boot.Config{
			Handshake: handshake.Config{
				SecureSession: securecipher.NewServerSession(securecipher.ServerConfig{
					Random:     rand.Reader,
					ServerTime: currentServerTimeMillis,
				}),
			},
			Login: loginflow.Config{
				Authenticate: func(packet loginproto.Login2Packet) loginflow.Result {
					stateMu.Lock()
					defer stateMu.Unlock()

					ticket, err := store.Load(packet.Login, packet.LoginKey)
					if err != nil {
						return loginflow.Result{Accepted: false, FailureStatus: "NOID"}
					}
					if accounts != nil {
						account, ok := loadOrCreateAccount(accounts, packet.Login)
						if !ok {
							return loginflow.Result{Accepted: false, FailureStatus: "FAILED"}
						}
						ticket.Empire = account.Empire
						ticket.Characters = cloneCharacters(account.Characters)
					}

					sessionTicket = ticket
					hasTicket = true
					hasSelected = false
					selectedPlayer = nil
					selectedIndex = 0
					return loginflow.Result{
						Accepted:      true,
						Empire:        ticketEmpire(ticket),
						LoginSuccess4: ticketLoginSuccessPacket(ticket, advertisedAddr, advertisedPort),
					}
				},
			},
			StateChecker: boot.StateCheckerConfig{
				Channels: []control.ChannelStatus{{Port: int16(advertisedPort), Status: control.ChannelStatusNormal}},
			},
			WorldEntry: worldentry.Config{
				SelectEmpire: func(empire uint8) worldentry.EmpireResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					if !hasTicket || !isValidEmpire(empire) || hasAnyCharacters(sessionTicket.Characters) {
						return worldentry.EmpireResult{Accepted: false}
					}
					sessionTicket.Empire = empire
					if !saveAccountSnapshot(accounts, sessionTicket.Login, sessionTicket.Empire, sessionTicket.Characters) {
						return worldentry.EmpireResult{Accepted: false}
					}
					return worldentry.EmpireResult{Accepted: true, Empire: empire}
				},
				CreateCharacter: func(packet worldproto.CharacterCreatePacket) worldentry.CreateResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					if !hasTicket {
						return worldentry.CreateResult{Accepted: false, FailureType: 0}
					}
					created, failureType, ok := createCharacterInTicket(&sessionTicket, packet, ticketEmpire(sessionTicket))
					if !ok {
						return worldentry.CreateResult{Accepted: false, FailureType: failureType}
					}
					if !saveAccountSnapshot(accounts, sessionTicket.Login, sessionTicket.Empire, sessionTicket.Characters) {
						return worldentry.CreateResult{Accepted: false, FailureType: 0}
					}
					return worldentry.CreateResult{
						Accepted: true,
						Player:   ticketPlayerCreateSuccessPacket(created, packet.Index, advertisedAddr, advertisedPort),
					}
				},
				DeleteCharacter: func(packet worldproto.CharacterDeletePacket) worldentry.DeleteResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					if !hasTicket {
						return worldentry.DeleteResult{Accepted: false}
					}
					updatedCharacters, deletedIndex, ok := deleteCharacterFromTicket(accounts, sessionTicket.Login, sessionTicket.Empire, sessionTicket.Characters, packet)
					if !ok {
						return worldentry.DeleteResult{Accepted: false}
					}
					sessionTicket.Characters = updatedCharacters
					if hasSelected && selectedIndex == deletedIndex {
						hasSelected = false
						selectedPlayer = nil
					}
					return worldentry.DeleteResult{Accepted: true, Index: deletedIndex}
				},
				SelectCharacter: func(index uint8) worldentry.Result {
					stateMu.Lock()
					defer stateMu.Unlock()

					if !hasTicket || int(index) >= len(sessionTicket.Characters) {
						return worldentry.Result{Accepted: false}
					}

					selected := sessionTicket.Characters[index]
					if selected.ID == 0 {
						return worldentry.Result{Accepted: false}
					}
					selectedIndex = index
					hasSelected = true
					selectedPlayer = player.NewRuntime(selected, player.SessionLink{Login: sessionTicket.Login, CharacterIndex: index})
					return worldentry.Result{
						Accepted:      true,
						Player:        selectedPlayer,
						MainCharacter: ticketMainCharacterPacket(selectedPlayer.LiveCharacter()),
						PlayerPoints:  ticketPlayerPointsPacket(selectedPlayer.PersistedSnapshot()),
					}
				},
				EnterGame: func() worldentry.EnterGameResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					selectedPlayer, ok := currentSelectedPlayer()
					if !ok {
						return worldentry.EnterGameResult{Rejected: true}
					}
					selected := selectedPlayer.LiveCharacter()
					if selected.ID == 0 {
						return worldentry.EnterGameResult{Rejected: true}
					}
					bootstrapFrames, err := worldentry.BuildBootstrapFrames(selected)
					if err != nil {
						return worldentry.EnterGameResult{Rejected: true}
					}
					var trailingFrames [][]byte
					if !joinedSharedWorld {
						var existingPeers []loginticket.Character
						sharedWorldID, existingPeers = sharedWorld.Join(selected, pending, func(mapIndex uint32, x int32, y int32) (RelocationPreview, bool) {
							stateMu.Lock()
							defer stateMu.Unlock()
							preview, _, ok := applySelectedCharacterTransfer(mapIndex, x, y, false)
							return preview, ok
						})
						joinedSharedWorld = sharedWorldID != 0
						if !joinedSharedWorld {
							return worldentry.EnterGameResult{Rejected: true}
						}
						for _, peer := range existingPeers {
							trailingFrames = append(trailingFrames, encodePeerVisibilityFrames(peer)...)
						}
					}
					trailingFrames = append(trailingFrames, sharedWorld.VisibleStaticActorFrames(selected)...)
					return worldentry.EnterGameResult{BootstrapFrames: bootstrapFrames, TrailingFrames: trailingFrames}
				},
			},
			Game: gameflow.Config{
				HandleMove: func(packet movep.MovePacket) gameflow.Result {
					stateMu.Lock()
					defer stateMu.Unlock()

					selectedPlayer, ok := currentSelectedPlayer()
					if !ok {
						return gameflow.Result{Accepted: false}
					}
					selected := selectedPlayer.LiveCharacter()
					if selected.ID == 0 {
						return gameflow.Result{Accepted: false}
					}
					previous := selected
					liveSharedWorld := ownsLiveSharedWorldSession()
					if liveSharedWorld {
						if trigger, ok := findBootstrapTransferTrigger(transferTriggers, selected, packet.X, packet.Y); ok {
							if _, transferFrames, ok := applySelectedCharacterTransfer(trigger.TargetMapIndex, trigger.TargetX, trigger.TargetY, true); !ok {
								return gameflow.Result{Accepted: false}
							} else {
								return gameflow.Result{Accepted: true, Frames: transferFrames}
							}
						}
					}

					selected, ok = applySelectedCharacterPosition(selectedPlayer, packet.X, packet.Y, liveSharedWorld)
					if !ok {
						return gameflow.Result{Accepted: false}
					}
					ack := ticketMoveAckPacket(selected, packet)
					if liveSharedWorld {
						sharedWorld.UpdateCharacterWithVisibilityTransition(sharedWorldID, previous, selected, [][]byte{movep.EncodeMoveAck(ack)})
					}
					return gameflow.Result{Accepted: true, Replication: ack}
				},
				HandleSyncPosition: func(packet movep.SyncPositionPacket) gameflow.SyncPositionResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					selectedPlayer, ok := currentSelectedPlayer()
					if !ok {
						return gameflow.SyncPositionResult{Accepted: false}
					}
					selected := selectedPlayer.LiveCharacter()
					if selected.ID == 0 {
						return gameflow.SyncPositionResult{Accepted: false}
					}
					previous := selected
					liveSharedWorld := ownsLiveSharedWorldSession()
					for _, element := range packet.Elements {
						if element.VID != selected.VID {
							continue
						}
						if liveSharedWorld {
							if trigger, ok := findBootstrapTransferTrigger(transferTriggers, selected, element.X, element.Y); ok {
								if _, transferFrames, ok := applySelectedCharacterTransfer(trigger.TargetMapIndex, trigger.TargetX, trigger.TargetY, true); !ok {
									return gameflow.SyncPositionResult{Accepted: false}
								} else {
									return gameflow.SyncPositionResult{Accepted: true, Frames: transferFrames}
								}
							}
						}
						selected, ok = applySelectedCharacterPosition(selectedPlayer, element.X, element.Y, liveSharedWorld)
						if !ok {
							return gameflow.SyncPositionResult{Accepted: false}
						}
						ack := ticketSyncPositionAckPacket(selected)
						if liveSharedWorld {
							sharedWorld.UpdateCharacterWithVisibilityTransition(sharedWorldID, previous, selected, [][]byte{movep.EncodeSyncPositionAck(ack)})
						}
						return gameflow.SyncPositionResult{Accepted: true, Synchronization: ack}
					}
					return gameflow.SyncPositionResult{Accepted: false}
				},
				HandleChat: func(packet chatproto.ClientChatPacket) gameflow.ChatResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					if command, ok := slashGameCommand(packet.Message); ok {
						leaveSharedWorld := func() {
							if !joinedSharedWorld || sharedWorldID == 0 {
								return
							}
							sharedWorld.Leave(sharedWorldID)
							joinedSharedWorld = false
							sharedWorldID = 0
						}
						switch command {
						case "quit":
							delivery := chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeCommand, Message: "quit"}
							return gameflow.ChatResult{Accepted: true, Delivery: &delivery}
						case "logout":
							leaveSharedWorld()
							hasSelected = false
							selectedPlayer = nil
							return gameflow.ChatResult{Accepted: true, NextPhase: session.PhaseClose}
						case "phase_select":
							leaveSharedWorld()
							hasSelected = false
							selectedPlayer = nil
							return gameflow.ChatResult{Accepted: true, NextPhase: session.PhaseSelect}
						}
					}

					selectedPlayer, ok := currentSelectedPlayer()
					if !ok {
						return gameflow.ChatResult{Accepted: false}
					}
					selected := selectedPlayer.LiveCharacter()
					if selected.ID == 0 || packet.Message == "" {
						return gameflow.ChatResult{Accepted: false}
					}
					liveSharedWorld := ownsLiveSharedWorldSession()
					switch packet.Type {
					case chatproto.ChatTypeTalking:
						chatDelivery := ticketActorChatDeliveryPacket(selected, packet)
						if liveSharedWorld {
							sharedWorld.EnqueueToOtherSessionsInEmpireOnMap(sharedWorldID, selected, [][]byte{chatproto.EncodeChatDelivery(chatDelivery)})
						}
						return gameflow.ChatResult{Accepted: true, Delivery: &chatDelivery}
					case chatproto.ChatTypeParty:
						chatDelivery := ticketActorChatDeliveryPacket(selected, packet)
						if liveSharedWorld {
							sharedWorld.EnqueueToOtherSessions(sharedWorldID, [][]byte{chatproto.EncodeChatDelivery(chatDelivery)})
						}
						return gameflow.ChatResult{Accepted: true, Delivery: &chatDelivery}
					case chatproto.ChatTypeGuild:
						if selected.GuildID == 0 {
							return gameflow.ChatResult{Accepted: false}
						}
						chatDelivery := ticketActorChatDeliveryPacket(selected, packet)
						if liveSharedWorld {
							sharedWorld.EnqueueToOtherSessionsInGuild(sharedWorldID, selected, [][]byte{chatproto.EncodeChatDelivery(chatDelivery)})
						}
						return gameflow.ChatResult{Accepted: true, Delivery: &chatDelivery}
					case chatproto.ChatTypeShout:
						chatDelivery := ticketActorChatDeliveryPacket(selected, packet)
						if liveSharedWorld {
							sharedWorld.EnqueueToOtherSessionsInEmpire(sharedWorldID, selected, [][]byte{chatproto.EncodeChatDelivery(chatDelivery)})
						}
						return gameflow.ChatResult{Accepted: true, Delivery: &chatDelivery}
					case chatproto.ChatTypeInfo:
						delivery := ticketSystemChatDeliveryPacket(packet)
						return gameflow.ChatResult{Accepted: true, Delivery: &delivery}
					default:
						return gameflow.ChatResult{Accepted: false}
					}
				},
				HandleWhisper: func(packet chatproto.ClientWhisperPacket) gameflow.WhisperResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					selectedPlayer, ok := currentSelectedPlayer()
					if !ok {
						return gameflow.WhisperResult{Accepted: false}
					}
					selected := selectedPlayer.LiveCharacter()
					if selected.ID == 0 || packet.Target == "" || packet.Message == "" {
						return gameflow.WhisperResult{Accepted: false}
					}
					if packet.Target == selected.Name {
						return gameflow.WhisperResult{Accepted: true}
					}
					if !ownsLiveSharedWorldSession() {
						return gameflow.WhisperResult{Accepted: true}
					}
					delivery := ticketWhisperDeliveryPacket(selected, packet)
					if sharedWorld.EnqueueToCharacterName(packet.Target, [][]byte{chatproto.EncodeServerWhisper(delivery)}) {
						return gameflow.WhisperResult{Accepted: true}
					}
					notFound := ticketWhisperNotExistPacket(packet.Target)
					return gameflow.WhisperResult{Accepted: true, Delivery: &notFound}
				},
			},
		})
		return newQueuedSessionFlow(inner, pending, func() {
			stateMu.Lock()
			leaveID := sharedWorldID
			shouldLeave := joinedSharedWorld
			joinedSharedWorld = false
			stateMu.Unlock()
			if shouldLeave {
				sharedWorld.Leave(leaveID)
			}
		})
	}
	return runtime, nil
}

func parsePort(addr string) (uint16, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, ErrInvalidLegacyAddr
	}

	parsed, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return 0, ErrInvalidLegacyAddr
	}

	return uint16(parsed), nil
}

func parseIPv4(addr string) (uint32, error) {
	ip := net.ParseIP(addr).To4()
	if ip == nil {
		return 0, ErrInvalidPublicAddr
	}

	return binary.LittleEndian.Uint32(ip), nil
}

func defaultKeyChallenge() control.KeyChallengePacket {
	return control.KeyChallengePacket{
		ServerPublicKey: sequentialBytes32(0x00),
		Challenge:       sequentialBytes32(0x20),
		ServerTime:      0x01020304,
	}
}

func defaultKeyComplete() control.KeyCompletePacket {
	return control.KeyCompletePacket{
		EncryptedToken: sequentialBytes48(0x80),
		Nonce:          sequentialBytes24(0xb0),
	}
}

func issueLoginTicket(store loginticket.Store, login string, empire uint8, characters []loginticket.Character, generateLoginKey loginKeyGenerator) (uint32, bool) {
	for range 8 {
		loginKey, err := generateLoginKey()
		if err != nil || loginKey == 0 {
			continue
		}

		err = store.Issue(loginticket.Ticket{
			Login:      login,
			LoginKey:   loginKey,
			Empire:     empire,
			Characters: cloneCharacters(characters),
		})
		if err == nil {
			return loginKey, true
		}
		if errors.Is(err, loginticket.ErrTicketExists) {
			continue
		}
		return 0, false
	}

	return 0, false
}

func loadOrCreateAccount(store accountstore.Store, login string) (accountstore.Account, bool) {
	if store == nil {
		characters := cloneCharacters(stubCharacters())
		return accountstore.Account{Login: login, Empire: ticketEmpire(loginticket.Ticket{Characters: characters}), Characters: characters}, true
	}
	account, err := store.Load(login)
	if err == nil {
		if normalized, changed := normalizeBootstrapStubAccount(account); changed {
			if err := store.Save(normalized); err != nil {
				return accountstore.Account{}, false
			}
			account = normalized
		}
		account.Characters = cloneCharacters(account.Characters)
		return account, true
	}
	if !errors.Is(err, accountstore.ErrAccountNotFound) {
		return accountstore.Account{}, false
	}
	characters := cloneCharacters(stubCharacters())
	account = accountstore.Account{Login: login, Empire: ticketEmpire(loginticket.Ticket{Characters: characters}), Characters: characters}
	if err := store.Save(account); err != nil {
		return accountstore.Account{}, false
	}
	account.Characters = cloneCharacters(account.Characters)
	return account, true
}

func normalizeBootstrapStubAccount(account accountstore.Account) (accountstore.Account, bool) {
	if !strings.EqualFold(account.Login, StubLogin) {
		return account, false
	}
	characters := cloneCharacters(account.Characters)
	changed := false
	for i, character := range characters {
		if character.ID == 0 || character.Name != "MkmkWar" {
			continue
		}
		if character.MapIndex != bootstrapMapIndex || character.X != legacyFakeStubMkmkWarX || character.Y != legacyFakeStubMkmkWarY {
			continue
		}
		character.X = bootstrapShinsooYonganStartX
		character.Y = bootstrapShinsooYonganStartY
		characters[i] = character
		changed = true
	}
	if !changed {
		return account, false
	}
	account.Characters = characters
	return account, true
}

func saveAccountSnapshot(store accountstore.Store, login string, empire uint8, characters []loginticket.Character) bool {
	if store == nil {
		return true
	}
	return store.Save(accountstore.Account{Login: login, Empire: empire, Characters: cloneCharacters(characters)}) == nil
}

func deleteCharacterFromTicket(store accountstore.Store, login string, empire uint8, characters []loginticket.Character, packet worldproto.CharacterDeletePacket) ([]loginticket.Character, uint8, bool) {
	index := int(packet.Index)
	if index < 0 || index >= len(characters) {
		return nil, 0, false
	}
	if strings.TrimSpace(packet.PrivateCode) == "" {
		return nil, 0, false
	}
	if characters[index].ID == 0 {
		return nil, 0, false
	}
	updatedCharacters := cloneCharacters(characters)
	updatedCharacters[index] = loginticket.Character{}
	if !saveAccountSnapshot(store, login, empire, updatedCharacters) {
		return nil, 0, false
	}
	return updatedCharacters, packet.Index, true
}

func cloneBootstrapTransferTriggers(triggers []bootstrapTransferTrigger) []bootstrapTransferTrigger {
	if len(triggers) == 0 {
		return nil
	}
	cloned := make([]bootstrapTransferTrigger, len(triggers))
	copy(cloned, triggers)
	return cloned
}

func findBootstrapTransferTrigger(triggers []bootstrapTransferTrigger, selected loginticket.Character, x int32, y int32) (bootstrapTransferTrigger, bool) {
	for _, trigger := range triggers {
		if trigger.SourceMapIndex != selected.MapIndex || trigger.SourceMapIndex == 0 {
			continue
		}
		if trigger.SourceX != x || trigger.SourceY != y || trigger.TargetMapIndex == 0 {
			continue
		}
		return trigger, true
	}
	return bootstrapTransferTrigger{}, false
}

func updateSelectedCharacterPosition(store accountstore.Store, login string, empire uint8, characters []loginticket.Character, selectedIndex uint8, x int32, y int32) ([]loginticket.Character, loginticket.Character, bool) {
	index := int(selectedIndex)
	if index < 0 || index >= len(characters) {
		return nil, loginticket.Character{}, false
	}
	selected := characters[index]
	if selected.ID == 0 {
		return nil, loginticket.Character{}, false
	}
	updatedCharacters := cloneCharacters(characters)
	selected.X = x
	selected.Y = y
	updatedCharacters[index] = selected
	if !saveAccountSnapshot(store, login, empire, updatedCharacters) {
		return nil, loginticket.Character{}, false
	}
	return updatedCharacters, selected, true
}

func updateSelectedCharacterLocation(store accountstore.Store, login string, empire uint8, characters []loginticket.Character, selectedIndex uint8, mapIndex uint32, x int32, y int32) ([]loginticket.Character, loginticket.Character, bool) {
	updatedCharacters, selected, ok := selectedCharacterLocationUpdate(characters, selectedIndex, mapIndex, x, y)
	if !ok {
		return nil, loginticket.Character{}, false
	}
	if !saveAccountSnapshot(store, login, empire, updatedCharacters) {
		return nil, loginticket.Character{}, false
	}
	return updatedCharacters, selected, true
}

func selectedCharacterLocationUpdate(characters []loginticket.Character, selectedIndex uint8, mapIndex uint32, x int32, y int32) ([]loginticket.Character, loginticket.Character, bool) {
	index := int(selectedIndex)
	if index < 0 || index >= len(characters) || mapIndex == 0 {
		return nil, loginticket.Character{}, false
	}
	selected := characters[index]
	if selected.ID == 0 {
		return nil, loginticket.Character{}, false
	}
	updatedCharacters := cloneCharacters(characters)
	selected.MapIndex = mapIndex
	selected.X = x
	selected.Y = y
	updatedCharacters[index] = selected
	return updatedCharacters, selected, true
}

func cloneCharacters(characters []loginticket.Character) []loginticket.Character {
	if len(characters) == 0 {
		return nil
	}
	cloned := make([]loginticket.Character, len(characters))
	copy(cloned, characters)
	return cloned
}

func randomLoginKey() (uint32, error) {
	var raw [4]byte
	for range 8 {
		if _, err := rand.Read(raw[:]); err != nil {
			return 0, err
		}
		loginKey := binary.LittleEndian.Uint32(raw[:])
		if loginKey != 0 {
			return loginKey, nil
		}
	}

	return 0, errors.New("failed to generate non-zero login key")
}

func ticketEmpire(ticket loginticket.Ticket) uint8 {
	if ticket.Empire != 0 {
		return ticket.Empire
	}
	for _, character := range ticket.Characters {
		if character.ID != 0 && character.Empire != 0 {
			return character.Empire
		}
	}
	return 0
}

func slashGameCommand(message string) (string, bool) {
	if !strings.HasPrefix(message, "/") {
		return "", false
	}
	fields := strings.Fields(strings.TrimSpace(message[1:]))
	if len(fields) == 0 {
		return "", false
	}
	switch fields[0] {
	case "quit", "logout", "phase_select":
		return fields[0], true
	default:
		return "", false
	}
}

func ticketLoginSuccessPacket(ticket loginticket.Ticket, addr uint32, port uint16) loginproto.LoginSuccess4Packet {
	packet := loginproto.LoginSuccess4Packet{
		Handle:    0x11223344,
		RandomKey: 0x55667788,
	}

	for i, character := range ticket.Characters {
		if i >= loginproto.PlayerCount {
			break
		}

		packet.Players[i] = loginproto.SimplePlayer{
			ID:          character.ID,
			Name:        character.Name,
			Job:         character.Job,
			Level:       character.Level,
			PlayMinutes: character.PlayMinutes,
			ST:          character.ST,
			HT:          character.HT,
			DX:          character.DX,
			IQ:          character.IQ,
			MainPart:    character.MainPart,
			ChangeName:  character.ChangeName,
			HairPart:    character.HairPart,
			Dummy:       character.Dummy,
			X:           character.X,
			Y:           character.Y,
			Addr:        addr,
			Port:        port,
			SkillGroup:  character.SkillGroup,
		}
		packet.GuildIDs[i] = character.GuildID
		packet.GuildNames[i] = character.GuildName
	}

	return packet
}

func ticketMainCharacterPacket(character loginticket.Character) worldproto.MainCharacterPacket {
	return worldproto.MainCharacterPacket{
		VID:        character.VID,
		RaceNum:    character.RaceNum,
		Name:       character.Name,
		BGMName:    "",
		BGMVolume:  math.Float32frombits(0),
		X:          character.X,
		Y:          character.Y,
		Z:          character.Z,
		Empire:     character.Empire,
		SkillGroup: character.SkillGroup,
	}
}

func ticketPlayerPointsPacket(character loginticket.Character) worldproto.PlayerPointsPacket {
	return worldproto.PlayerPointsPacket{Points: character.Points}
}

func ticketCharacterAddPacket(character loginticket.Character) worldproto.CharacterAddPacket {
	return worldproto.CharacterAddPacket{
		VID:         character.VID,
		Angle:       90.5,
		X:           character.X,
		Y:           character.Y,
		Z:           character.Z,
		Type:        6,
		RaceNum:     character.RaceNum,
		MovingSpeed: 150,
		AttackSpeed: 100,
		StateFlag:   2,
		AffectFlags: [worldproto.AffectFlagCount]uint32{0x11111111, 0x22222222},
	}
}

func ticketCharacterAdditionalInfoPacket(character loginticket.Character) worldproto.CharacterAdditionalInfoPacket {
	return worldproto.CharacterAdditionalInfoPacket{
		VID:       character.VID,
		Name:      character.Name,
		Parts:     [worldproto.CharacterEquipmentPartCount]uint16{character.MainPart, 0, 0, character.HairPart},
		Empire:    character.Empire,
		GuildID:   character.GuildID,
		Level:     uint32(character.Level),
		Alignment: 0,
		PKMode:    0,
		MountVnum: 0,
	}
}

func ticketCharacterUpdatePacket(character loginticket.Character) worldproto.CharacterUpdatePacket {
	return worldproto.CharacterUpdatePacket{
		VID:         character.VID,
		Parts:       [worldproto.CharacterEquipmentPartCount]uint16{character.MainPart, 0, 0, character.HairPart},
		MovingSpeed: 150,
		AttackSpeed: 100,
		StateFlag:   2,
		AffectFlags: [worldproto.AffectFlagCount]uint32{0x11111111, 0x22222222},
		GuildID:     character.GuildID,
		Alignment:   0,
		PKMode:      0,
		MountVnum:   0,
	}
}

func ticketPlayerPointChangePacket(character loginticket.Character) worldproto.PlayerPointChangePacket {
	return worldproto.PlayerPointChangePacket{
		VID:    character.VID,
		Type:   bootstrapPlayerPointType,
		Amount: character.Points[bootstrapPlayerPointValueIndex],
		Value:  character.Points[bootstrapPlayerPointValueIndex],
	}
}

func ticketMoveAckPacket(character loginticket.Character, packet movep.MovePacket) movep.MoveAckPacket {
	return movep.MoveAckPacket{
		Func:     packet.Func,
		Arg:      packet.Arg,
		Rot:      packet.Rot,
		VID:      character.VID,
		X:        packet.X,
		Y:        packet.Y,
		Time:     packet.Time,
		Duration: 250,
	}
}

func ticketSyncPositionAckPacket(character loginticket.Character) movep.SyncPositionAckPacket {
	return movep.SyncPositionAckPacket{Elements: []movep.SyncPositionElement{{VID: character.VID, X: character.X, Y: character.Y}}}
}

func ticketActorChatDeliveryPacket(character loginticket.Character, packet chatproto.ClientChatPacket) chatproto.ChatDeliveryPacket {
	return chatproto.ChatDeliveryPacket{
		Type:    packet.Type,
		VID:     character.VID,
		Empire:  0,
		Message: fmt.Sprintf("%s : %s", character.Name, packet.Message),
	}
}

func ticketSystemChatDeliveryPacket(packet chatproto.ClientChatPacket) chatproto.ChatDeliveryPacket {
	return chatproto.ChatDeliveryPacket{
		Type:    packet.Type,
		VID:     0,
		Empire:  0,
		Message: packet.Message,
	}
}

func ticketWhisperDeliveryPacket(character loginticket.Character, packet chatproto.ClientWhisperPacket) chatproto.ServerWhisperPacket {
	return chatproto.ServerWhisperPacket{
		Type:     chatproto.WhisperTypeChat,
		FromName: character.Name,
		Message:  packet.Message,
	}
}

func ticketWhisperNotExistPacket(target string) chatproto.ServerWhisperPacket {
	return chatproto.ServerWhisperPacket{
		Type:     chatproto.WhisperTypeNotExist,
		FromName: target,
	}
}

func ticketPlayerCreateSuccessPacket(character loginticket.Character, index uint8, addr uint32, port uint16) worldproto.PlayerCreateSuccessPacket {
	return worldproto.PlayerCreateSuccessPacket{
		Index: index,
		Player: loginproto.SimplePlayer{
			ID:          character.ID,
			Name:        character.Name,
			Job:         character.Job,
			Level:       character.Level,
			PlayMinutes: character.PlayMinutes,
			ST:          character.ST,
			HT:          character.HT,
			DX:          character.DX,
			IQ:          character.IQ,
			MainPart:    character.MainPart,
			ChangeName:  character.ChangeName,
			HairPart:    character.HairPart,
			Dummy:       character.Dummy,
			X:           character.X,
			Y:           character.Y,
			Addr:        addr,
			Port:        port,
			SkillGroup:  character.SkillGroup,
		},
	}
}

func createCharacterInTicket(ticket *loginticket.Ticket, packet worldproto.CharacterCreatePacket, empire uint8) (loginticket.Character, uint8, bool) {
	if ticket == nil || packet.Index >= loginproto.PlayerCount {
		return loginticket.Character{}, 0, false
	}
	if !isValidEmpire(empire) {
		return loginticket.Character{}, 0, false
	}
	if !isValidCharacterName(packet.Name) || !isValidCreateRace(packet.RaceNum) || packet.Shape > 1 {
		return loginticket.Character{}, 0, false
	}
	if hasDuplicateCharacterName(ticket.Characters, packet.Name) {
		return loginticket.Character{}, 1, false
	}

	index := int(packet.Index)
	if index < len(ticket.Characters) && ticket.Characters[index].ID != 0 {
		return loginticket.Character{}, 0, false
	}
	if len(ticket.Characters) <= index {
		extended := make([]loginticket.Character, index+1)
		copy(extended, ticket.Characters)
		ticket.Characters = extended
	}

	character := buildCreatedCharacter(nextCharacterID(ticket.Characters), nextCharacterVID(ticket.Characters), packet, empire)
	ticket.Characters[index] = character
	return character, 0, true
}

type initialCharacterStats struct {
	ST    uint8
	HT    uint8
	DX    uint8
	IQ    uint8
	MaxHP int32
	MaxSP int32
}

func buildCreatedCharacter(id uint32, vid uint32, packet worldproto.CharacterCreatePacket, empire uint8) loginticket.Character {
	stats := initialStatsForRace(packet.RaceNum)
	mapIndex, x, y := legacyCreatePositionForEmpire(empire)
	points := initialPointsForRace(packet.RaceNum)
	return loginticket.Character{
		ID:          id,
		VID:         vid,
		Name:        packet.Name,
		Job:         uint8(packet.RaceNum),
		RaceNum:     packet.RaceNum,
		Level:       1,
		PlayMinutes: 0,
		ST:          stats.ST,
		HT:          stats.HT,
		DX:          stats.DX,
		IQ:          stats.IQ,
		MainPart:    uint16(packet.Shape),
		ChangeName:  0,
		HairPart:    0,
		Dummy:       [4]byte{},
		X:           x,
		Y:           y,
		Z:           0,
		MapIndex:    mapIndex,
		Empire:      empire,
		SkillGroup:  0,
		Points:      points,
	}
}

func legacyCreatePositionForEmpire(empire uint8) (uint32, int32, int32) {
	switch empire {
	case 1:
		return bootstrapMapIndex, 459800, 953900
	case 2:
		return 21, 52070, 166600
	case 3:
		return 41, 957300, 255200
	default:
		return bootstrapMapIndex, 459800, 953900
	}
}

func initialStatsForRace(race uint16) initialCharacterStats {
	switch race {
	case 0, 4:
		return initialCharacterStats{ST: 6, HT: 4, DX: 3, IQ: 3, MaxHP: 600, MaxSP: 200}
	case 1, 5:
		return initialCharacterStats{ST: 4, HT: 3, DX: 6, IQ: 3, MaxHP: 650, MaxSP: 200}
	case 2, 6:
		return initialCharacterStats{ST: 5, HT: 3, DX: 3, IQ: 5, MaxHP: 650, MaxSP: 200}
	case 3, 7:
		return initialCharacterStats{ST: 3, HT: 4, DX: 3, IQ: 6, MaxHP: 700, MaxSP: 200}
	default:
		return initialCharacterStats{}
	}
}

func initialPointsForRace(race uint16) [worldproto.PointCount]int32 {
	stats := initialStatsForRace(race)
	var points [worldproto.PointCount]int32
	points[0] = 1
	points[1] = stats.MaxHP
	points[2] = stats.MaxSP
	return points
}

func nextCharacterID(characters []loginticket.Character) uint32 {
	var maxID uint32
	for _, character := range characters {
		if character.ID > maxID {
			maxID = character.ID
		}
	}
	if maxID == 0 {
		return 1
	}
	return maxID + 1
}

func nextCharacterVID(characters []loginticket.Character) uint32 {
	var maxVID uint32
	for _, character := range characters {
		if character.VID > maxVID {
			maxVID = character.VID
		}
	}
	if maxVID == 0 {
		return 0x01020304
	}
	return maxVID + 1
}

func isValidEmpire(empire uint8) bool {
	switch empire {
	case 1, 2, 3:
		return true
	default:
		return false
	}
}

func hasAnyCharacters(characters []loginticket.Character) bool {
	for _, character := range characters {
		if character.ID != 0 {
			return true
		}
	}
	return false
}

func isValidCreateRace(race uint16) bool {
	switch race {
	case 0, 1, 2, 3, 4, 5, 6, 7:
		return true
	default:
		return false
	}
}

func isValidCharacterName(name string) bool {
	if name == "" || len(name) >= worldproto.CharacterNameFieldSize {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_':
		default:
			return false
		}
	}
	return true
}

func hasDuplicateCharacterName(characters []loginticket.Character, name string) bool {
	for _, character := range characters {
		if character.ID != 0 && strings.EqualFold(character.Name, name) {
			return true
		}
	}
	return false
}

func stubCharacters() []loginticket.Character {
	first := loginticket.Character{
		ID:          1,
		VID:         0x01020304,
		Name:        "MkmkWar",
		Job:         0,
		RaceNum:     0,
		Level:       15,
		PlayMinutes: 4321,
		ST:          6,
		HT:          5,
		DX:          4,
		IQ:          3,
		MainPart:    101,
		ChangeName:  0,
		HairPart:    201,
		Dummy:       [4]byte{1, 2, 3, 4},
		X:           bootstrapShinsooYonganStartX,
		Y:           bootstrapShinsooYonganStartY,
		Z:           0,
		MapIndex:    bootstrapMapIndex,
		Empire:      2,
		SkillGroup:  1,
		GuildID:     10,
		GuildName:   "Alpha",
	}
	first.Points[0] = 15
	first.Points[1] = 1234
	first.Points[2] = 5678
	first.Points[3] = 900
	first.Points[4] = 1000
	first.Points[5] = 200
	first.Points[6] = 300
	first.Points[7] = 999999
	first.Points[8] = 50

	second := loginticket.Character{
		ID:          2,
		VID:         0x01020305,
		Name:        "MkmkSura",
		Job:         3,
		RaceNum:     3,
		Level:       12,
		PlayMinutes: 2100,
		ST:          4,
		HT:          5,
		DX:          3,
		IQ:          8,
		MainPart:    102,
		ChangeName:  0,
		HairPart:    202,
		Dummy:       [4]byte{5, 6, 7, 8},
		X:           1200,
		Y:           2100,
		Z:           0,
		MapIndex:    bootstrapMapIndex,
		Empire:      2,
		SkillGroup:  2,
	}
	second.Points[0] = 12
	second.Points[1] = 900
	second.Points[2] = 1800
	second.Points[3] = 700
	second.Points[4] = 800
	second.Points[5] = 150
	second.Points[6] = 120
	second.Points[7] = 500000
	second.Points[8] = 20

	return []loginticket.Character{first, second}
}

func (r *gameRuntime) loadPersistedStaticActors() error {
	if r == nil || r.staticStore == nil || r.sharedWorld == nil {
		return nil
	}
	snapshot, err := r.staticStore.Load()
	if err != nil {
		if errors.Is(err, staticstore.ErrSnapshotNotFound) {
			return nil
		}
		return err
	}
	for _, actor := range snapshot.StaticActors {
		if _, ok := r.sharedWorld.RegisterStaticActorWithEntityID(actor.EntityID, actor.Name, actor.MapIndex, actor.X, actor.Y, actor.RaceNum); !ok {
			return fmt.Errorf("%w: apply static actor snapshot", staticstore.ErrInvalidSnapshot)
		}
	}
	return nil
}

func (r *gameRuntime) persistStaticActorSnapshot(snapshot []StaticActorSnapshot) bool {
	if r == nil || r.staticStore == nil {
		return true
	}
	return r.staticStore.Save(buildStaticActorStoreSnapshot(snapshot)) == nil
}

func buildStaticActorStoreSnapshot(snapshot []StaticActorSnapshot) staticstore.Snapshot {
	actors := make([]staticstore.StaticActor, 0, len(snapshot))
	for _, actor := range snapshot {
		actors = append(actors, staticstore.StaticActor{
			EntityID: actor.EntityID,
			Name:     actor.Name,
			MapIndex: actor.MapIndex,
			X:        actor.X,
			Y:        actor.Y,
			RaceNum:  actor.RaceNum,
		})
	}
	return staticstore.Snapshot{StaticActors: actors}
}

func cloneStaticActorSnapshots(snapshot []StaticActorSnapshot) []StaticActorSnapshot {
	if len(snapshot) == 0 {
		return nil
	}
	cloned := make([]StaticActorSnapshot, len(snapshot))
	copy(cloned, snapshot)
	return cloned
}

func appendStaticActorSnapshot(snapshot []StaticActorSnapshot, actor StaticActorSnapshot) []StaticActorSnapshot {
	cloned := cloneStaticActorSnapshots(snapshot)
	return append(cloned, actor)
}

func staticActorSnapshotIndex(snapshot []StaticActorSnapshot, entityID uint64) int {
	for i, actor := range snapshot {
		if actor.EntityID == entityID {
			return i
		}
	}
	return -1
}

func currentServerTimeMillis() uint32 {
	return uint32(time.Now().UnixMilli())
}

func defaultTicketStoreDir() string {
	return filepath.Join(os.TempDir(), "go-metin2-server-login-tickets")
}

func defaultAccountStoreDir() string {
	return filepath.Join(os.TempDir(), "go-metin2-server-accounts")
}

func defaultStaticActorStorePath() string {
	return filepath.Join(os.TempDir(), "go-metin2-server-static-actors.json")
}

func sequentialBytes32(start byte) [32]byte {
	var out [32]byte
	for i := range out {
		out[i] = start + byte(i)
	}
	return out
}

func sequentialBytes48(start byte) [48]byte {
	var out [48]byte
	for i := range out {
		out[i] = start + byte(i)
	}
	return out
}

func sequentialBytes24(start byte) [24]byte {
	var out [24]byte
	for i := range out {
		out[i] = start + byte(i)
	}
	return out
}
