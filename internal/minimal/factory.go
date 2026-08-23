package minimal

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	authflow "github.com/MikelCalvo/go-metin2-server/internal/auth"
	"github.com/MikelCalvo/go-metin2-server/internal/authboot"
	"github.com/MikelCalvo/go-metin2-server/internal/boot"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	contentbundle "github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	gameflow "github.com/MikelCalvo/go-metin2-server/internal/game"
	"github.com/MikelCalvo/go-metin2-server/internal/handshake"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	loginflow "github.com/MikelCalvo/go-metin2-server/internal/login"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/player"
	authproto "github.com/MikelCalvo/go-metin2-server/internal/proto/auth"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	effectproto "github.com/MikelCalvo/go-metin2-server/internal/proto/effect"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
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
const bootstrapExperiencePointType = player.ExperiencePointIndex
const bootstrapGoldPointType uint8 = 11
const bootstrapPracticeMobRetaliationPointDelta int32 = -1
const bootstrapNormalAttackCadenceWindow = 250 * time.Millisecond
const bootstrapPracticeMobServerOriginRetaliationDelay = time.Second
const bootstrapSpawnGroupReturnStepDelay = time.Second
const bootstrapSpawnGroupReturnStepMaxStep int32 = 100
const bootstrapSpawnGroupChaseStepDelay = 5 * time.Second
const bootstrapSpawnGroupHomewardStepDelay = time.Second
const bootstrapSpawnGroupHomewardStepMaxStep int32 = 100
const bootstrapSpawnGroupChaseStepMaxStep int32 = 100

const bootstrapCharacterPositionGeneral uint8 = 0
const bootstrapCharacterPositionSittingChair uint8 = 3
const bootstrapCharacterPositionSittingGround uint8 = 4
const itemDropRejectedInfoMessage = "You cannot drop this item."
const itemPickupInventoryFullInfoMessage = "You have too many items."
const itemBuyRejectedInfoMessage = "The merchant will not sell this item to you."
const itemSellRejectedInfoMessage = "The merchant refuses to buy this item."
const itemUnequipRejectedInfoMessage = "You cannot remove this item."
const itemEquipOccupiedWearSlotInfoMessage = "You are already wearing equipment."
const safeboxPasswordWrongInfoMessage = "You have entered the wrong password."
const safeboxPasswordChangedInfoMessage = "The warehouse password has been changed."
const safeboxAlreadyOpenInfoMessage = "The warehouse is already open."
const safeboxShowPasswordCommandMessage = "ShowMeSafeboxPassword"
const questFlagRewardRestrictedInfoMessage = "You cannot receive this quest reward."
const questFlagInsufficientGoldInfoMessage = "You do not have enough gold."
const questFlagInsufficientExperienceInfoMessage = "You do not have enough experience."
const questFlagInsufficientMaterialsInfoMessage = "You do not have the required items."
const questFlagRewardGoldOverflowInfoMessage = "You cannot carry any more gold."
const questFlagRewardExperienceOverflowInfoMessage = "You cannot gain any more experience."
const exchangePartnerMerchantBusyInfoMessage = "That player cannot trade right now."
const exchangeRequesterMerchantBusyInfoMessage = "You cannot trade while another trade window is open."
const exchangeRequesterGoldCarrierCapInfoMessage = "You have more than 2 Billion Yang. You cannot trade."
const exchangePartnerGoldCarrierCapInfoMessage = "The player has more than 2 Billion Yang. You cannot trade with him."
const exchangeFinalizeCheckSelfInfoMessage = "Not enough Yang or the item is not in place."
const exchangeFinalizeCheckOtherInfoMessage = "The other player does not have enough Yang or their item is not in place."
const exchangeFinalizeSpaceSelfInfoMessage = "There isn't enough space in your inventory."
const exchangeFinalizeSpaceOtherInfoMessage = "The other person has no space left in their inventory."
const exchangeFinalizeGoldOverflowSelfInfoMessage = questFlagRewardGoldOverflowInfoMessage
const exchangeFinalizeGoldOverflowOtherInfoMessage = "The other person cannot carry any more gold."
const exchangeFinalizeOtherInfoMessage = "Unknown error"
const exchangeFinalizeSuccessInfoMessageFormat = "The trade with %s has been successful."
const bootstrapSafeboxOpenMinSize uint8 = 1
const bootstrapSafeboxOpenMaxSize uint8 = 3
const bootstrapSafeboxCellsPerPage uint8 = 5
const bootstrapMapIndex uint32 = 1
const bootstrapShinsooYonganStartX int32 = 469300
const bootstrapShinsooYonganStartY int32 = 964200
const legacyFakeStubMkmkWarX int32 = 1000
const legacyFakeStubMkmkWarY int32 = 2000

var (
	ErrInvalidLegacyAddr                    = errors.New("invalid legacy addr")
	ErrInvalidPublicAddr                    = errors.New("invalid public addr")
	ErrInvalidVisibilityMode                = errors.New("invalid visibility mode")
	ErrInvalidVisibilityRadius              = errors.New("invalid visibility radius")
	ErrInvalidVisibilitySectorSize          = errors.New("invalid visibility sector size")
	ErrInteractionDefinitionsUnavailable    = errors.New("interaction definitions unavailable")
	ErrInteractionDefinitionExists          = errors.New("interaction definition already exists")
	ErrInteractionDefinitionNotFound        = errors.New("interaction definition not found")
	ErrInteractionDefinitionReferenced      = errors.New("interaction definition referenced by static actor")
	ErrContentBundleUnavailable             = errors.New("content bundle unavailable")
	ErrItemTemplateStoreRestoreLiveSessions = errors.New("item template store restore requires no live sessions")
	ErrAccountStoreRestoreLiveSessions      = errors.New("account store restore requires no live sessions")
	ErrLoginTicketStoreRestoreLiveSessions  = errors.New("login ticket store restore requires no live sessions")
	ErrQuestStateStoreRestoreLiveSessions   = errors.New("quest state store restore requires no live sessions")
	ErrStaticActorStoreRestoreLiveSessions  = errors.New("static actor store restore requires no live sessions")
	ErrInteractionStoreRestoreLiveSessions  = errors.New("interaction store restore requires no live sessions")
	ErrGroundItemStoreRestoreLiveSessions   = errors.New("ground item store restore requires no live sessions")
	ErrSafeboxStoreRestoreLiveSessions      = errors.New("safebox store restore requires no live sessions")

	refineConfirmRollMu       sync.Mutex
	refineConfirmRollOverride []int
)

// QueueRefineConfirmRollForTest appends one injected refine confirm roll in
// 1..100 for the next probability 1..99 confirm. Tests should restore via the
// returned cleanup so leftover injections cannot leak across cases.
func QueueRefineConfirmRollForTest(roll int) func() {
	refineConfirmRollMu.Lock()
	defer refineConfirmRollMu.Unlock()
	previous := append([]int(nil), refineConfirmRollOverride...)
	refineConfirmRollOverride = append(refineConfirmRollOverride, roll)
	return func() {
		refineConfirmRollMu.Lock()
		defer refineConfirmRollMu.Unlock()
		refineConfirmRollOverride = previous
	}
}

func takeRefineConfirmRoll() (int, bool) {
	refineConfirmRollMu.Lock()
	if len(refineConfirmRollOverride) > 0 {
		roll := refineConfirmRollOverride[0]
		refineConfirmRollOverride = append([]int(nil), refineConfirmRollOverride[1:]...)
		refineConfirmRollMu.Unlock()
		return roll, true
	}
	refineConfirmRollMu.Unlock()

	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0, false
	}
	return int(binary.BigEndian.Uint64(buf[:])%100) + 1, true
}

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

type GroundItemSnapshot = worldruntime.GroundItemSnapshot

type StaticActorSnapshot = worldruntime.StaticActorSnapshot

type PositionSnapshot = worldruntime.PositionSnapshot

type SpawnLeashSnapshot = worldruntime.SpawnLeashSnapshot

type SpawnGroupLeashSnapshot struct {
	Actor StaticActorSnapshot `json:"actor"`
	SpawnLeashSnapshot
}

type SpawnGroupReturnStepSnapshot struct {
	Actor StaticActorSnapshot          `json:"actor"`
	Step  SpawnLeashReturnStepSnapshot `json:"step"`
}

type SpawnGroupPendingReturnStepSnapshot struct {
	EntityID    uint64                       `json:"entity_id"`
	ReadyAt     time.Time                    `json:"ready_at"`
	RemainingMs int64                        `json:"remaining_ms"`
	Actor       StaticActorSnapshot          `json:"actor"`
	Step        SpawnLeashReturnStepSnapshot `json:"step"`
}

type SpawnGroupPendingChaseStepSnapshot struct {
	EntityID    uint64                       `json:"entity_id"`
	ReadyAt     time.Time                    `json:"ready_at"`
	RemainingMs int64                        `json:"remaining_ms"`
	Actor       StaticActorSnapshot          `json:"actor"`
	Step        SpawnLeashReturnStepSnapshot `json:"step"`
}

type SpawnGroupPendingHomewardStepSnapshot struct {
	EntityID    uint64                       `json:"entity_id"`
	ReadyAt     time.Time                    `json:"ready_at"`
	RemainingMs int64                        `json:"remaining_ms"`
	Actor       StaticActorSnapshot          `json:"actor"`
	Step        SpawnLeashReturnStepSnapshot `json:"step"`
}

type SpawnLeashReturnStepSnapshot struct {
	SpawnLeashSnapshot
	Next     worldruntime.PositionSnapshot `json:"next"`
	Complete bool                          `json:"complete"`
}

type InteractionDefinition = interactionstore.Definition

type QuestFlagSnapshot = queststate.FlagSnapshot

type CharacterQuestStateSnapshot = queststate.CharacterSnapshot

type InteractableStaticActorVisibilitySnapshot struct {
	StaticActorSnapshot
	Preview           string `json:"preview,omitempty"`
	ResolutionFailure string `json:"resolution_failure,omitempty"`
}

type CharacterInteractionVisibilitySnapshot struct {
	ConnectedCharacterSnapshot
	VisibleInteractableStaticActors []InteractableStaticActorVisibilitySnapshot `json:"visible_interactable_static_actors"`
}

type InventoryItemSnapshot struct {
	ID     uint64 `json:"id"`
	Vnum   uint32 `json:"vnum"`
	Count  uint16 `json:"count"`
	Slot   uint16 `json:"slot"`
	Locked bool   `json:"locked,omitempty"`
}

type EquipmentItemSnapshot struct {
	ID        uint64 `json:"id"`
	Vnum      uint32 `json:"vnum"`
	Count     uint16 `json:"count"`
	EquipSlot string `json:"equip_slot"`
	Locked    bool   `json:"locked,omitempty"`
}

type QuickslotSnapshot struct {
	Position uint8 `json:"position"`
	Type     uint8 `json:"type"`
	Slot     uint8 `json:"slot"`
}

type CharacterInventorySnapshot struct {
	Name      string                  `json:"name"`
	Inventory []InventoryItemSnapshot `json:"inventory"`
}

type CharacterEquipmentSnapshot struct {
	Name      string                  `json:"name"`
	Equipment []EquipmentItemSnapshot `json:"equipment"`
}

type CharacterQuickslotsSnapshot struct {
	Name       string              `json:"name"`
	Quickslots []QuickslotSnapshot `json:"quickslots"`
}

type CharacterCurrencySnapshot struct {
	Name string `json:"name"`
	Gold uint64 `json:"gold"`
}

type CharacterPointsSnapshot struct {
	Name   string     `json:"name"`
	Points [255]int32 `json:"points"`
}

type StaticActorRespawnSnapshot struct {
	EntityID    uint64              `json:"entity_id"`
	ReadyAt     time.Time           `json:"ready_at"`
	RemainingMs int64               `json:"remaining_ms"`
	Actor       StaticActorSnapshot `json:"actor"`
}

const (
	staticActorInteractionFailureDefinitionNotFound            = "interaction_definition_not_found"
	staticActorInteractionFailureUnsupportedKind               = "unsupported_interaction_kind"
	staticActorInteractionFailureWarpDestinationInvalid        = "warp_destination_invalid"
	staticActorInteractionFailureWarpNotApplied                = "warp_not_applied"
	staticActorInteractionFailureQuestCurrentValueMismatch     = "quest_current_value_mismatch"
	staticActorInteractionFailureQuestInsufficientGold         = "quest_insufficient_gold"
	staticActorInteractionFailureQuestInsufficientExperience   = "quest_insufficient_experience"
	staticActorInteractionFailureQuestInsufficientMaterials    = "quest_insufficient_materials"
	staticActorInteractionFailureQuestRewardInventoryFull      = "quest_reward_inventory_full"
	staticActorInteractionFailureQuestRewardRestricted         = "quest_reward_restricted"
	staticActorInteractionFailureQuestRewardGoldOverflow       = "quest_reward_gold_overflow"
	staticActorInteractionFailureQuestRewardExperienceOverflow = "quest_reward_experience_overflow"
	staticActorInteractionCooldown                             = time.Second
)

type staticActorInteractionResolution struct {
	Accepted   bool
	Failure    string
	TargetVID  uint32
	Actor      StaticActorSnapshot
	Definition InteractionDefinition
	Delivery   *chatproto.ChatDeliveryPacket
}

type staticActorCombatTargetResolution struct {
	Accepted        bool
	Failure         string
	TargetVID       uint32
	SnapshotVersion uint64
	Actor           StaticActorSnapshot
	Packet          *combatproto.ServerTargetPacket
}

type staticActorCombatAttackResolution struct {
	Accepted                    bool
	Failure                     string
	ActiveTargetVID             uint32
	ActiveTargetSnapshotVersion uint64
	RequestedTargetVID          uint32
	Actor                       StaticActorSnapshot
	DeathReward                 worldruntime.StaticActorDeathReward
	Damage                      uint8
	Packet                      *combatproto.ServerTargetPacket
	Frames                      [][]byte
	SelfPostMutationFrames      [][]byte
	PeerPostMutationFrames      [][]byte
	ClearActiveTarget           bool
}

type merchantBuyContext struct {
	TargetVID  uint32
	Definition InteractionDefinition
}

type refineDialogPresentation struct {
	Pos        uint8
	Type       uint8
	SourceID   uint64
	SourceVnum uint32
	Cell       inventory.SlotIndex
	RefineInfo itemcatalog.RefineInfo
}

type RuntimeConfigSnapshot struct {
	LocalChannelID       uint8                     `json:"local_channel_id"`
	VisibilityMode       string                    `json:"visibility_mode"`
	VisibilityRadius     int32                     `json:"visibility_radius"`
	VisibilitySectorSize int32                     `json:"visibility_sector_size"`
	Persistence          PersistenceConfigSnapshot `json:"persistence"`
	Database             DatabaseConfigSnapshot    `json:"database"`
}

type PersistenceConfigSnapshot struct {
	LoginTicketStoreDir   string `json:"login_ticket_store_dir"`
	AccountStoreDir       string `json:"account_store_dir"`
	StaticActorStorePath  string `json:"static_actor_store_path"`
	InteractionStorePath  string `json:"interaction_store_path"`
	ItemTemplateStorePath string `json:"item_template_store_path"`
	QuestStateStorePath   string `json:"quest_state_store_path"`
	GroundItemStorePath   string `json:"ground_item_store_path"`
	SafeboxStorePath      string `json:"safebox_store_path"`
}

type DatabaseConfigSnapshot struct {
	Configured    bool   `json:"configured"`
	Driver        string `json:"driver,omitempty"`
	DSNConfigured bool   `json:"dsn_configured"`
}

type PersistenceStatusSnapshot struct {
	OK                         bool                    `json:"ok"`
	LiveSelectedCharacterCount int                     `json:"live_selected_character_count"`
	AccountStore               AccountStoreStatus      `json:"account_store"`
	LoginTicketStore           LoginTicketStoreStatus  `json:"login_ticket_store"`
	ItemTemplateStore          ItemTemplateStoreStatus `json:"item_template_store"`
	StaticActorStore           StaticActorStoreStatus  `json:"static_actor_store"`
	InteractionStore           InteractionStoreStatus  `json:"interaction_store"`
	QuestStateStore            QuestStateStoreStatus   `json:"quest_state_store"`
	GroundItemStore            GroundItemStoreStatus   `json:"ground_item_store"`
	SafeboxStore               SafeboxStoreStatus      `json:"safebox_store"`
}

type AccountStoreStatus struct {
	Path                         string                       `json:"path"`
	Valid                        bool                         `json:"valid"`
	Summary                      accountstore.SnapshotSummary `json:"summary"`
	BackupManifest               BackupManifestStatus         `json:"backup_manifest"`
	RestoreBlockedByLiveSessions bool                         `json:"restore_blocked_by_live_sessions"`
	Error                        string                       `json:"error,omitempty"`
}

type LoginTicketStoreStatus struct {
	Path                         string                      `json:"path"`
	Valid                        bool                        `json:"valid"`
	Summary                      loginticket.SnapshotSummary `json:"summary"`
	BackupManifest               BackupManifestStatus        `json:"backup_manifest"`
	RestoreBlockedByLiveSessions bool                        `json:"restore_blocked_by_live_sessions"`
	Error                        string                      `json:"error,omitempty"`
}

type ItemTemplateStoreStatus struct {
	Path                         string                      `json:"path"`
	Valid                        bool                        `json:"valid"`
	Summary                      itemcatalog.SnapshotSummary `json:"summary"`
	BackupManifest               BackupManifestStatus        `json:"backup_manifest"`
	RestoreBlockedByLiveSessions bool                        `json:"restore_blocked_by_live_sessions"`
	Error                        string                      `json:"error,omitempty"`
}

type BackupManifestStatus struct {
	Present           bool   `json:"present"`
	Path              string `json:"path,omitempty"`
	Format            string `json:"format,omitempty"`
	FileCount         int    `json:"file_count,omitempty"`
	SnapshotSizeBytes int64  `json:"snapshot_size_bytes,omitempty"`
	ManifestSizeBytes int64  `json:"manifest_size_bytes,omitempty"`
	ManifestSHA256    string `json:"manifest_sha256,omitempty"`
}

type StaticActorStoreStatus struct {
	Path                         string                      `json:"path"`
	Valid                        bool                        `json:"valid"`
	Summary                      staticstore.SnapshotSummary `json:"summary"`
	BackupManifest               BackupManifestStatus        `json:"backup_manifest"`
	RestoreBlockedByLiveSessions bool                        `json:"restore_blocked_by_live_sessions"`
	Error                        string                      `json:"error,omitempty"`
}

type InteractionStoreStatus struct {
	Path                         string                           `json:"path"`
	Valid                        bool                             `json:"valid"`
	Summary                      interactionstore.SnapshotSummary `json:"summary"`
	BackupManifest               BackupManifestStatus             `json:"backup_manifest"`
	RestoreBlockedByLiveSessions bool                             `json:"restore_blocked_by_live_sessions"`
	Error                        string                           `json:"error,omitempty"`
}

type QuestStateStoreStatus struct {
	Path                         string                     `json:"path"`
	Valid                        bool                       `json:"valid"`
	Summary                      queststate.SnapshotSummary `json:"summary"`
	BackupManifest               BackupManifestStatus       `json:"backup_manifest"`
	RestoreBlockedByLiveSessions bool                       `json:"restore_blocked_by_live_sessions"`
	Error                        string                     `json:"error,omitempty"`
}

type GroundItemStoreStatus struct {
	Path                         string                                        `json:"path"`
	Valid                        bool                                          `json:"valid"`
	Summary                      worldruntime.DurableGroundItemSnapshotSummary `json:"summary"`
	BackupManifest               BackupManifestStatus                          `json:"backup_manifest"`
	RestoreBlockedByLiveSessions bool                                          `json:"restore_blocked_by_live_sessions"`
	Error                        string                                        `json:"error,omitempty"`
}

type SafeboxStoreStatus struct {
	Path                         string                       `json:"path"`
	Valid                        bool                         `json:"valid"`
	Summary                      safeboxstore.SnapshotSummary `json:"summary"`
	BackupManifest               BackupManifestStatus         `json:"backup_manifest"`
	RestoreBlockedByLiveSessions bool                         `json:"restore_blocked_by_live_sessions"`
	Error                        string                       `json:"error,omitempty"`
}

type MapOccupancyChange = worldruntime.MapOccupancyChange

type RelocationPreview = worldruntime.RelocationPreview

type gameRuntime struct {
	sessionFactory          service.SessionFactory
	sharedWorld             *sharedWorldRegistry
	config                  config.Service
	staticStore             staticstore.Store
	loginTicketStore        loginticket.Store
	accountStore            accountstore.Store
	itemStore               itemcatalog.Store
	interactionStore        interactionstore.Store
	questStateStore         queststate.Store
	groundItemStore         worldruntime.GroundItemStore
	groundItemExporter      worldruntime.BootstrapGroundItemStateExporter
	groundItemPersistMu     sync.Mutex
	safeboxStore            safeboxstore.Store
	safeboxPersistMu        sync.Mutex
	itemTemplates           map[uint32]itemcatalog.Template
	itemTemplatesAuthored   bool
	liveCharacterMu         sync.RWMutex
	liveCharacterNextID     uint64
	liveCharactersByName    map[string]liveCharacterRegistration
	interactionDefinitionMu sync.RWMutex
	interactionDefinitions  map[string]interactionstore.Definition
	questStateMu            sync.Mutex
	staticActorMu           sync.Mutex
	spawnReturnMu           sync.Mutex
	spawnReturnStepDueAt    map[uint64]time.Time
	spawnChaseMu            sync.Mutex
	spawnChaseStepDueAt     map[uint64]time.Time
	spawnHomewardMu         sync.Mutex
	spawnHomewardStepDueAt  map[uint64]time.Time
	now                     func() time.Time
}

type liveCharacterStateSnapshot struct {
	Name       string
	Level      uint8
	Job        uint8
	RaceNum    uint16
	Empire     uint8
	Gold       uint64
	Points     [255]int32
	Inventory  []InventoryItemSnapshot
	Equipment  []EquipmentItemSnapshot
	Quickslots []QuickslotSnapshot
}

type liveCharacterStateSnapshotter func() (liveCharacterStateSnapshot, bool)
type liveCharacterPersistedSnapshotApplier func(loginticket.Character) bool

type liveCharacterRegistration struct {
	id                     uint64
	login                  string
	snapshotter            liveCharacterStateSnapshotter
	applyPersistedSnapshot liveCharacterPersistedSnapshotApplier
}

func NewGameRuntime(cfg config.Service) (*gameRuntime, error) {
	if err := config.ValidateOpsConfig(cfg); err != nil {
		return nil, err
	}
	if err := config.ValidateDatabaseDriverAvailability(cfg); err != nil {
		return nil, err
	}
	return newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		cfg,
		loginticket.NewFileStore(serviceLoginTicketStoreDir(cfg)),
		accountstore.NewFileStore(serviceAccountStoreDir(cfg)),
		staticstore.NewFileStore(serviceStaticActorStorePath(cfg)),
		interactionstore.NewFileStore(serviceInteractionStorePath(cfg)),
		itemcatalog.NewFileStore(serviceItemTemplateStorePath(cfg)),
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

func (r *gameRuntime) MigrationStatus() (dbmigrations.Plan, error) {
	if r != nil && strings.TrimSpace(r.config.DatabaseDriver) != "" {
		db, err := sql.Open(strings.TrimSpace(r.config.DatabaseDriver), strings.TrimSpace(r.config.DatabaseDSN))
		if err != nil {
			return dbmigrations.Plan{}, fmt.Errorf("%w: %v", config.ErrDatabaseDriverUnavailable, err)
		}
		defer db.Close()
		return dbmigrations.PlanUpToLatestFromSQLLedger(context.Background(), db)
	}
	return dbmigrations.PlanUpToLatest(nil)
}

func (r *gameRuntime) MigrationCatalogSummary() (dbmigrations.CatalogSummaryPayload, error) {
	return dbmigrations.BuiltInCatalogSummary()
}

func (r *gameRuntime) MigrationPlanToVersion(targetVersion int) (dbmigrations.Plan, error) {
	if r != nil && strings.TrimSpace(r.config.DatabaseDriver) != "" {
		db, err := sql.Open(strings.TrimSpace(r.config.DatabaseDriver), strings.TrimSpace(r.config.DatabaseDSN))
		if err != nil {
			return dbmigrations.Plan{}, fmt.Errorf("%w: %v", config.ErrDatabaseDriverUnavailable, err)
		}
		defer db.Close()
		return dbmigrations.PlanToVersionFromSQLLedger(context.Background(), db, targetVersion)
	}
	return dbmigrations.PlanToVersion(nil, targetVersion)
}

func (r *gameRuntime) MigrationPlanFromLedgerSnapshot(snapshot dbmigrations.LedgerSnapshot, targetVersion int) (dbmigrations.Plan, error) {
	return dbmigrations.PlanToVersionFromLedgerSnapshot(snapshot, targetVersion)
}

func (r *gameRuntime) MigrationLedgerSnapshot() (dbmigrations.LedgerSnapshot, error) {
	if r != nil && strings.TrimSpace(r.config.DatabaseDriver) != "" {
		db, err := sql.Open(strings.TrimSpace(r.config.DatabaseDriver), strings.TrimSpace(r.config.DatabaseDSN))
		if err != nil {
			return dbmigrations.LedgerSnapshot{}, fmt.Errorf("%w: %v", config.ErrDatabaseDriverUnavailable, err)
		}
		defer db.Close()
		return dbmigrations.LedgerSnapshotFromSQLLedger(context.Background(), db)
	}
	return dbmigrations.LedgerSnapshot{
		Format:  dbmigrations.LedgerSnapshotFormat,
		Entries: []dbmigrations.LedgerEntry{},
	}, nil
}

func (r *gameRuntime) PersistenceStatus() PersistenceStatusSnapshot {
	if r == nil {
		return PersistenceStatusSnapshot{}
	}
	liveSelectedCharacterCount := r.liveSelectedCharacterCount()
	accountStatus := r.accountStoreStatus(liveSelectedCharacterCount)
	loginTicketStatus := r.loginTicketStoreStatus(liveSelectedCharacterCount)
	itemTemplateStatus := r.itemTemplateStoreStatus(liveSelectedCharacterCount)
	staticActorStatus := r.staticActorStoreStatus(liveSelectedCharacterCount)
	interactionStatus := r.interactionStoreStatus(liveSelectedCharacterCount)
	questStateStatus := r.questStateStoreStatus(liveSelectedCharacterCount)
	groundItemStatus := r.groundItemStoreStatus()
	safeboxStatus := r.safeboxStoreStatus()
	return PersistenceStatusSnapshot{
		OK:                         accountStatus.Valid && loginTicketStatus.Valid && itemTemplateStatus.Valid && staticActorStatus.Valid && interactionStatus.Valid && questStateStatus.Valid && groundItemStatus.Valid && safeboxStatus.Valid,
		LiveSelectedCharacterCount: liveSelectedCharacterCount,
		AccountStore:               accountStatus,
		LoginTicketStore:           loginTicketStatus,
		ItemTemplateStore:          itemTemplateStatus,
		StaticActorStore:           staticActorStatus,
		InteractionStore:           interactionStatus,
		QuestStateStore:            questStateStatus,
		GroundItemStore:            groundItemStatus,
		SafeboxStore:               safeboxStatus,
	}
}

func (r *gameRuntime) accountStoreStatus(liveSelectedCharacterCount int) AccountStoreStatus {
	status := AccountStoreStatus{Path: accountStoreDir(nil)}
	if r != nil {
		status.Path = accountStoreDir(r.accountStore)
	}
	status.BackupManifest = accountBackupManifestStatus(status.Path)
	status.RestoreBlockedByLiveSessions = liveSelectedCharacterCount != 0
	summary, err := r.ValidateAccountStore()
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Valid = true
	status.Summary = summary
	return status
}

func (r *gameRuntime) loginTicketStoreStatus(liveSelectedCharacterCount int) LoginTicketStoreStatus {
	status := LoginTicketStoreStatus{Path: loginTicketStoreDir(nil)}
	if r != nil {
		status.Path = loginTicketStoreDir(r.loginTicketStore)
	}
	status.BackupManifest = loginTicketBackupManifestStatus(status.Path)
	status.RestoreBlockedByLiveSessions = liveSelectedCharacterCount != 0
	summary, err := r.ValidateLoginTicketStore()
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Valid = true
	status.Summary = summary
	return status
}

func (r *gameRuntime) itemTemplateStoreStatus(liveSelectedCharacterCount int) ItemTemplateStoreStatus {
	status := ItemTemplateStoreStatus{Path: itemTemplateStorePath(nil)}
	if r != nil {
		status.Path = itemTemplateStorePath(r.itemStore)
	}
	status.BackupManifest = itemTemplateBackupManifestStatus(status.Path)
	status.RestoreBlockedByLiveSessions = liveSelectedCharacterCount != 0
	summary, err := r.ValidateItemTemplateStore()
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Valid = true
	status.Summary = summary
	return status
}

func (r *gameRuntime) staticActorStoreStatus(liveSelectedCharacterCount int) StaticActorStoreStatus {
	status := StaticActorStoreStatus{Path: staticActorStorePath(nil)}
	if r != nil {
		status.Path = staticActorStorePath(r.staticStore)
	}
	status.BackupManifest = staticActorBackupManifestStatus(status.Path)
	status.RestoreBlockedByLiveSessions = liveSelectedCharacterCount != 0
	summary, err := r.ValidateStaticActorStore()
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Valid = true
	status.Summary = summary
	return status
}

func (r *gameRuntime) interactionStoreStatus(liveSelectedCharacterCount int) InteractionStoreStatus {
	status := InteractionStoreStatus{Path: interactionStorePath(nil)}
	if r != nil {
		status.Path = interactionStorePath(r.interactionStore)
	}
	status.BackupManifest = interactionBackupManifestStatus(status.Path)
	status.RestoreBlockedByLiveSessions = liveSelectedCharacterCount != 0
	summary, err := r.ValidateInteractionStore()
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Valid = true
	status.Summary = summary
	return status
}

func (r *gameRuntime) questStateStoreStatus(liveSelectedCharacterCount int) QuestStateStoreStatus {
	status := QuestStateStoreStatus{Path: questStateStorePath(nil)}
	if r != nil {
		status.Path = questStateStorePath(r.questStateStore)
	}
	status.BackupManifest = questStateBackupManifestStatus(status.Path)
	status.RestoreBlockedByLiveSessions = liveSelectedCharacterCount != 0
	summary, err := r.ValidateQuestStateStore()
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Valid = true
	status.Summary = summary
	return status
}

func (r *gameRuntime) groundItemStoreStatus() GroundItemStoreStatus {
	liveSelectedCharacterCount := 0
	if r != nil {
		liveSelectedCharacterCount = r.liveSelectedCharacterCount()
	}
	status := GroundItemStoreStatus{Path: groundItemStorePath(nil), Summary: worldruntime.DurableGroundItemSnapshotSummary{VIDs: []uint32{}}}
	if r != nil {
		status.Path = groundItemStorePath(r.groundItemStore)
	}
	status.BackupManifest = groundItemBackupManifestStatus(status.Path)
	status.RestoreBlockedByLiveSessions = liveSelectedCharacterCount != 0
	summary, err := r.ValidateGroundItemStore()
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Valid = true
	status.Summary = summary
	return status
}

func (r *gameRuntime) safeboxStoreStatus() SafeboxStoreStatus {
	liveSelectedCharacterCount := 0
	if r != nil {
		liveSelectedCharacterCount = r.liveSelectedCharacterCount()
	}
	status := SafeboxStoreStatus{Path: safeboxStorePath(nil), Summary: safeboxstore.SnapshotSummary{Logins: []string{}, CharacterKeys: []string{}}}
	if r != nil {
		status.Path = safeboxStorePath(r.safeboxStore)
	}
	status.BackupManifest = safeboxBackupManifestStatus(status.Path)
	status.RestoreBlockedByLiveSessions = liveSelectedCharacterCount != 0
	summary, err := r.ValidateSafeboxStore()
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Valid = true
	status.Summary = summary
	return status
}

func (r *gameRuntime) ValidateGroundItemStore() (worldruntime.DurableGroundItemSnapshotSummary, error) {
	if r == nil || r.groundItemStore == nil {
		return worldruntime.DurableGroundItemSnapshotSummary{VIDs: []uint32{}}, nil
	}
	if validator, ok := r.groundItemStore.(interface {
		Validate() (worldruntime.DurableGroundItemSnapshotSummary, error)
	}); ok {
		return validator.Validate()
	}
	snapshot, err := r.groundItemStore.Load()
	if err != nil {
		if errors.Is(err, worldruntime.ErrGroundItemSnapshotNotFound) {
			return worldruntime.DurableGroundItemSnapshotSummary{VIDs: []uint32{}}, nil
		}
		return worldruntime.DurableGroundItemSnapshotSummary{}, err
	}
	return worldruntime.SummarizeDurableGroundItemSnapshot(snapshot), nil
}

func (r *gameRuntime) ValidateAccountStore() (accountstore.SnapshotSummary, error) {
	if r == nil || r.accountStore == nil {
		return accountstore.SnapshotSummary{Logins: []string{}}, nil
	}
	validator, ok := r.accountStore.(interface {
		Validate() (accountstore.SnapshotSummary, error)
	})
	if !ok {
		return accountstore.SnapshotSummary{}, fmt.Errorf("account store validation is not supported")
	}
	return validator.Validate()
}

func (r *gameRuntime) CleanupAccountStoreCrashTempFiles() (accountstore.SnapshotSummary, error) {
	if r == nil || r.accountStore == nil {
		return accountstore.SnapshotSummary{Logins: []string{}}, nil
	}
	cleaner, ok := r.accountStore.(interface {
		CleanupCrashTempFiles() (accountstore.SnapshotSummary, error)
	})
	if !ok {
		return accountstore.SnapshotSummary{}, fmt.Errorf("account store crash temp cleanup is not supported")
	}
	return cleaner.CleanupCrashTempFiles()
}

func (r *gameRuntime) ValidateLoginTicketStore() (loginticket.SnapshotSummary, error) {
	if r == nil || r.loginTicketStore == nil {
		return loginticket.SnapshotSummary{Logins: []string{}, LoginKeys: []uint32{}}, nil
	}
	validator, ok := r.loginTicketStore.(interface {
		Validate() (loginticket.SnapshotSummary, error)
	})
	if !ok {
		return loginticket.SnapshotSummary{}, fmt.Errorf("login ticket store validation is not supported")
	}
	return validator.Validate()
}

func (r *gameRuntime) CleanupLoginTicketStoreCrashTempFiles() (loginticket.SnapshotSummary, error) {
	if r == nil || r.loginTicketStore == nil {
		return loginticket.SnapshotSummary{Logins: []string{}, LoginKeys: []uint32{}}, nil
	}
	cleaner, ok := r.loginTicketStore.(interface {
		CleanupCrashTempFiles() (loginticket.SnapshotSummary, error)
	})
	if !ok {
		return loginticket.SnapshotSummary{}, fmt.Errorf("login ticket store crash temp cleanup is not supported")
	}
	return cleaner.CleanupCrashTempFiles()
}

func (r *gameRuntime) PreviewLoginTicketStoreIssuedBefore(issuedBefore time.Time) (loginticket.IssuedBeforePreviewSummary, error) {
	if r == nil || r.loginTicketStore == nil {
		return loginticket.IssuedBeforePreviewSummary{
			IssuedBefore:   issuedBefore,
			StaleLogins:    []string{},
			StaleLoginKeys: []uint32{},
			Current:        loginticket.SnapshotSummary{Logins: []string{}, LoginKeys: []uint32{}},
		}, nil
	}
	previewer, ok := r.loginTicketStore.(interface {
		PreviewIssuedBefore(time.Time) (loginticket.IssuedBeforePreviewSummary, error)
	})
	if !ok {
		return loginticket.IssuedBeforePreviewSummary{}, fmt.Errorf("login ticket issued-before preview is not supported")
	}
	return previewer.PreviewIssuedBefore(issuedBefore)
}

func (r *gameRuntime) CleanupLoginTicketStoreIssuedBefore(issuedBefore time.Time) (loginticket.IssuedBeforeCleanupSummary, error) {
	if r == nil || r.loginTicketStore == nil {
		return loginticket.IssuedBeforeCleanupSummary{
			IssuedBefore:     issuedBefore,
			RemovedLogins:    []string{},
			RemovedLoginKeys: []uint32{},
			Remaining:        loginticket.SnapshotSummary{Logins: []string{}, LoginKeys: []uint32{}},
		}, nil
	}
	cleaner, ok := r.loginTicketStore.(interface {
		CleanupIssuedBefore(time.Time) (loginticket.IssuedBeforeCleanupSummary, error)
	})
	if !ok {
		return loginticket.IssuedBeforeCleanupSummary{}, fmt.Errorf("login ticket issued-before cleanup is not supported")
	}
	return cleaner.CleanupIssuedBefore(issuedBefore)
}

func (r *gameRuntime) ValidateItemTemplateStore() (itemcatalog.SnapshotSummary, error) {
	if r == nil || r.itemStore == nil {
		return itemcatalog.SnapshotSummary{Vnums: []uint32{}}, nil
	}
	validator, ok := r.itemStore.(interface {
		Validate() (itemcatalog.SnapshotSummary, error)
	})
	if !ok {
		return itemcatalog.SnapshotSummary{}, fmt.Errorf("item template store validation is not supported")
	}
	return validator.Validate()
}

func (r *gameRuntime) CleanupItemTemplateStoreCrashTempFiles() (itemcatalog.SnapshotSummary, error) {
	if r == nil || r.itemStore == nil {
		return itemcatalog.SnapshotSummary{Vnums: []uint32{}}, nil
	}
	cleaner, ok := r.itemStore.(interface {
		CleanupCrashTempFiles() (itemcatalog.SnapshotSummary, error)
	})
	if !ok {
		return itemcatalog.SnapshotSummary{}, fmt.Errorf("item template store crash temp cleanup is not supported")
	}
	return cleaner.CleanupCrashTempFiles()
}

func (r *gameRuntime) CleanupStaticActorStoreCrashTempFiles() (staticstore.SnapshotSummary, error) {
	if r == nil || r.staticStore == nil {
		return staticstore.SnapshotSummary{ActorIDs: []uint64{}, ActorNames: []string{}}, nil
	}
	cleaner, ok := r.staticStore.(interface {
		CleanupCrashTempFiles() (staticstore.SnapshotSummary, error)
	})
	if !ok {
		return staticstore.SnapshotSummary{}, fmt.Errorf("static actor store crash temp cleanup is not supported")
	}
	r.staticActorMu.Lock()
	defer r.staticActorMu.Unlock()
	return cleaner.CleanupCrashTempFiles()
}

func (r *gameRuntime) CleanupInteractionStoreCrashTempFiles() (interactionstore.SnapshotSummary, error) {
	if r == nil || r.interactionStore == nil {
		return interactionstore.SnapshotSummary{DefinitionKeys: []string{}}, nil
	}
	cleaner, ok := r.interactionStore.(interface {
		CleanupCrashTempFiles() (interactionstore.SnapshotSummary, error)
	})
	if !ok {
		return interactionstore.SnapshotSummary{}, fmt.Errorf("interaction store crash temp cleanup is not supported")
	}
	r.interactionDefinitionMu.Lock()
	defer r.interactionDefinitionMu.Unlock()
	return cleaner.CleanupCrashTempFiles()
}

func (r *gameRuntime) CleanupQuestStateStoreCrashTempFiles() (queststate.SnapshotSummary, error) {
	if r == nil || r.questStateStore == nil {
		return queststate.SnapshotSummary{Characters: []string{}, QuestRefs: []string{}, FlagKeys: []string{}}, nil
	}
	cleaner, ok := r.questStateStore.(interface {
		CleanupCrashTempFiles() (queststate.SnapshotSummary, error)
	})
	if !ok {
		return queststate.SnapshotSummary{}, fmt.Errorf("quest state store crash temp cleanup is not supported")
	}
	r.questStateMu.Lock()
	defer r.questStateMu.Unlock()
	return cleaner.CleanupCrashTempFiles()
}

func (r *gameRuntime) ApplyQuestStateTransition(transition queststate.Transition) (queststate.TransitionApplyResult, error) {
	if r == nil || r.questStateStore == nil {
		return queststate.TransitionApplyResult{}, fmt.Errorf("quest state transition is not supported")
	}
	applier, ok := r.questStateStore.(interface {
		ApplyTransition(queststate.Transition) (queststate.TransitionApplyResult, error)
	})
	if !ok {
		return queststate.TransitionApplyResult{}, fmt.Errorf("quest state transition is not supported")
	}
	r.questStateMu.Lock()
	defer r.questStateMu.Unlock()
	return applier.ApplyTransition(transition)
}

func (r *gameRuntime) PreviewQuestStateTransition(transition queststate.Transition) (queststate.TransitionApplyResult, error) {
	if r == nil || r.questStateStore == nil {
		return queststate.TransitionApplyResult{}, fmt.Errorf("quest state transition preview is not supported")
	}
	previewer, ok := r.questStateStore.(interface {
		PreviewTransition(queststate.Transition) (queststate.TransitionApplyResult, error)
	})
	if !ok {
		return queststate.TransitionApplyResult{}, fmt.Errorf("quest state transition preview is not supported")
	}
	r.questStateMu.Lock()
	defer r.questStateMu.Unlock()
	return previewer.PreviewTransition(transition)
}

func (r *gameRuntime) QuestStateOverview() (queststate.Overview, error) {
	if r == nil || r.questStateStore == nil {
		return queststate.Overview{QuestRefs: []string{}}, nil
	}
	r.questStateMu.Lock()
	defer r.questStateMu.Unlock()
	snapshot, err := r.questStateStore.Load()
	if err != nil {
		if errors.Is(err, queststate.ErrSnapshotNotFound) {
			return queststate.OverviewSnapshot(queststate.Snapshot{Flags: []queststate.Flag{}})
		}
		return queststate.Overview{}, err
	}
	return queststate.OverviewSnapshot(snapshot)
}

func (r *gameRuntime) QuestState(character string) (CharacterQuestStateSnapshot, bool, error) {
	character = strings.TrimSpace(character)
	if r == nil || r.questStateStore == nil || character == "" {
		return CharacterQuestStateSnapshot{}, false, nil
	}
	r.questStateMu.Lock()
	defer r.questStateMu.Unlock()
	snapshot, err := r.questStateStore.Load()
	if err != nil {
		if errors.Is(err, queststate.ErrSnapshotNotFound) {
			return CharacterQuestStateSnapshot{}, false, nil
		}
		return CharacterQuestStateSnapshot{}, false, err
	}
	return queststate.CharacterSnapshotFor(snapshot, character)
}

func (r *gameRuntime) QuestStateByQuest(questRef string) (queststate.QuestSnapshot, bool, error) {
	questRef = strings.TrimSpace(questRef)
	if r == nil || r.questStateStore == nil || questRef == "" {
		return queststate.QuestSnapshot{}, false, nil
	}
	r.questStateMu.Lock()
	defer r.questStateMu.Unlock()
	snapshot, err := r.questStateStore.Load()
	if err != nil {
		if errors.Is(err, queststate.ErrSnapshotNotFound) {
			return queststate.QuestSnapshot{}, false, nil
		}
		return queststate.QuestSnapshot{}, false, err
	}
	return queststate.QuestSnapshotFor(snapshot, questRef)
}

func (r *gameRuntime) QuestStateFlag(character string, questRef string, flagName string) (queststate.Flag, bool, error) {
	character = strings.TrimSpace(character)
	questRef = strings.TrimSpace(questRef)
	flagName = strings.TrimSpace(flagName)
	if r == nil || r.questStateStore == nil || character == "" || questRef == "" || flagName == "" {
		return queststate.Flag{}, false, nil
	}
	r.questStateMu.Lock()
	defer r.questStateMu.Unlock()
	snapshot, err := r.questStateStore.Load()
	if err != nil {
		if errors.Is(err, queststate.ErrSnapshotNotFound) {
			return queststate.Flag{}, false, nil
		}
		return queststate.Flag{}, false, err
	}
	return queststate.ExactFlag(snapshot, character, questRef, flagName)
}

func (r *gameRuntime) ValidateStaticActorStore() (staticstore.SnapshotSummary, error) {
	if r == nil || r.staticStore == nil {
		return staticstore.SnapshotSummary{ActorIDs: []uint64{}, ActorNames: []string{}}, nil
	}
	validator, ok := r.staticStore.(interface {
		Validate() (staticstore.SnapshotSummary, error)
	})
	if !ok {
		return staticstore.SnapshotSummary{}, fmt.Errorf("static actor store validation is not supported")
	}
	return validator.Validate()
}

func (r *gameRuntime) BackupStaticActorStore(dstDir string) (staticstore.SnapshotSummary, error) {
	if r == nil || r.staticStore == nil {
		return staticstore.SnapshotSummary{ActorIDs: []uint64{}, ActorNames: []string{}}, nil
	}
	backer, ok := r.staticStore.(interface {
		BackupTo(string) error
		ValidateBackupFrom(string) (staticstore.SnapshotSummary, error)
	})
	if !ok {
		return staticstore.SnapshotSummary{}, fmt.Errorf("static actor store backup is not supported")
	}
	r.staticActorMu.Lock()
	defer r.staticActorMu.Unlock()
	if err := backer.BackupTo(dstDir); err != nil {
		return staticstore.SnapshotSummary{}, err
	}
	return backer.ValidateBackupFrom(dstDir)
}

func (r *gameRuntime) ValidateStaticActorStoreBackup(srcDir string) (staticstore.SnapshotSummary, error) {
	if r == nil || r.staticStore == nil {
		return staticstore.SnapshotSummary{ActorIDs: []uint64{}, ActorNames: []string{}}, nil
	}
	validator, ok := r.staticStore.(interface {
		ValidateBackupFrom(string) (staticstore.SnapshotSummary, error)
	})
	if !ok {
		return staticstore.SnapshotSummary{}, fmt.Errorf("static actor store backup validation is not supported")
	}
	r.staticActorMu.Lock()
	defer r.staticActorMu.Unlock()
	return validator.ValidateBackupFrom(srcDir)
}

func (r *gameRuntime) RestoreStaticActorStore(srcDir string) (staticstore.SnapshotSummary, error) {
	if r == nil || r.staticStore == nil {
		return staticstore.SnapshotSummary{ActorIDs: []uint64{}, ActorNames: []string{}}, nil
	}
	restorer, ok := r.staticStore.(interface {
		RestoreFrom(string) error
		Validate() (staticstore.SnapshotSummary, error)
		Load() (staticstore.Snapshot, error)
	})
	if !ok {
		return staticstore.SnapshotSummary{}, fmt.Errorf("static actor store restore is not supported")
	}
	r.liveCharacterMu.Lock()
	defer r.liveCharacterMu.Unlock()
	if len(r.liveCharactersByName) != 0 {
		return staticstore.SnapshotSummary{}, ErrStaticActorStoreRestoreLiveSessions
	}
	r.staticActorMu.Lock()
	defer r.staticActorMu.Unlock()
	if err := restorer.RestoreFrom(srcDir); err != nil {
		return staticstore.SnapshotSummary{}, err
	}
	if err := r.reloadPersistedStaticActorsLocked(); err != nil {
		return staticstore.SnapshotSummary{}, err
	}
	return restorer.Validate()
}

func (r *gameRuntime) reloadPersistedStaticActorsLocked() error {
	if r == nil || r.sharedWorld == nil {
		return nil
	}
	previousActors := r.StaticActors()
	for _, actor := range previousActors {
		if _, ok := r.sharedWorld.entities.RemoveStaticActor(actor.EntityID); !ok {
			return fmt.Errorf("%w: clear live static actors before restore", ErrContentBundleUnavailable)
		}
		r.sharedWorld.mu.Lock()
		r.sharedWorld.clearStaticActorCombatStateLocked(actor.EntityID)
		r.sharedWorld.mu.Unlock()
		r.clearSpawnGroupReturnStep(actor.EntityID)
		r.clearSpawnGroupChaseStep(actor.EntityID)
		r.clearSpawnGroupHomewardStep(actor.EntityID)
	}
	if err := r.loadPersistedStaticActors(); err != nil {
		return err
	}
	r.pruneSpawnGroupReturnStepSchedules()
	r.pruneSpawnGroupChaseStepSchedules()
	r.pruneSpawnGroupHomewardStepSchedules()
	return nil
}

func (r *gameRuntime) BackupInteractionStore(dstDir string) (interactionstore.SnapshotSummary, error) {
	if r == nil || r.interactionStore == nil {
		return interactionstore.SnapshotSummary{DefinitionKeys: []string{}}, nil
	}
	backer, ok := r.interactionStore.(interface {
		BackupTo(string) error
		ValidateBackupFrom(string) (interactionstore.SnapshotSummary, error)
	})
	if !ok {
		return interactionstore.SnapshotSummary{}, fmt.Errorf("interaction store backup is not supported")
	}
	r.interactionDefinitionMu.Lock()
	defer r.interactionDefinitionMu.Unlock()
	if err := backer.BackupTo(dstDir); err != nil {
		return interactionstore.SnapshotSummary{}, err
	}
	return backer.ValidateBackupFrom(dstDir)
}

func (r *gameRuntime) ValidateInteractionStoreBackup(srcDir string) (interactionstore.SnapshotSummary, error) {
	if r == nil || r.interactionStore == nil {
		return interactionstore.SnapshotSummary{DefinitionKeys: []string{}}, nil
	}
	validator, ok := r.interactionStore.(interface {
		ValidateBackupFrom(string) (interactionstore.SnapshotSummary, error)
	})
	if !ok {
		return interactionstore.SnapshotSummary{}, fmt.Errorf("interaction store backup validation is not supported")
	}
	r.interactionDefinitionMu.Lock()
	defer r.interactionDefinitionMu.Unlock()
	return validator.ValidateBackupFrom(srcDir)
}

func (r *gameRuntime) RestoreInteractionStore(srcDir string) (interactionstore.SnapshotSummary, error) {
	if r == nil || r.interactionStore == nil {
		return interactionstore.SnapshotSummary{DefinitionKeys: []string{}}, nil
	}
	restorer, ok := r.interactionStore.(interface {
		RestoreFrom(string) error
		Validate() (interactionstore.SnapshotSummary, error)
	})
	if !ok {
		return interactionstore.SnapshotSummary{}, fmt.Errorf("interaction store restore is not supported")
	}
	r.liveCharacterMu.Lock()
	defer r.liveCharacterMu.Unlock()
	if len(r.liveCharactersByName) != 0 {
		return interactionstore.SnapshotSummary{}, ErrInteractionStoreRestoreLiveSessions
	}
	r.interactionDefinitionMu.Lock()
	defer r.interactionDefinitionMu.Unlock()
	if err := restorer.RestoreFrom(srcDir); err != nil {
		return interactionstore.SnapshotSummary{}, err
	}
	if err := r.reloadPersistedInteractionDefinitionsLocked(); err != nil {
		return interactionstore.SnapshotSummary{}, err
	}
	return restorer.Validate()
}

func (r *gameRuntime) reloadPersistedInteractionDefinitionsLocked() error {
	if r == nil || r.interactionStore == nil {
		return nil
	}
	snapshot, err := r.interactionStore.Load()
	if err != nil {
		if errors.Is(err, interactionstore.ErrSnapshotNotFound) {
			r.interactionDefinitions = nil
			return nil
		}
		return err
	}
	if err := r.validateInteractionDefinitions(snapshot); err != nil {
		return err
	}
	r.interactionDefinitions = buildInteractionDefinitionIndex(snapshot)
	return nil
}

func (r *gameRuntime) ValidateInteractionStore() (interactionstore.SnapshotSummary, error) {
	if r == nil || r.interactionStore == nil {
		return interactionstore.SnapshotSummary{DefinitionKeys: []string{}}, nil
	}
	validator, ok := r.interactionStore.(interface {
		Validate() (interactionstore.SnapshotSummary, error)
	})
	if !ok {
		return interactionstore.SnapshotSummary{}, fmt.Errorf("interaction store validation is not supported")
	}
	return validator.Validate()
}

func (r *gameRuntime) ValidateQuestStateStore() (queststate.SnapshotSummary, error) {
	if r == nil || r.questStateStore == nil {
		return queststate.SnapshotSummary{Characters: []string{}, QuestRefs: []string{}, FlagKeys: []string{}}, nil
	}
	validator, ok := r.questStateStore.(interface {
		Validate() (queststate.SnapshotSummary, error)
	})
	if !ok {
		return queststate.SnapshotSummary{}, fmt.Errorf("quest state store validation is not supported")
	}
	r.questStateMu.Lock()
	defer r.questStateMu.Unlock()
	return validator.Validate()
}

func (r *gameRuntime) BackupQuestStateStore(dstDir string) (queststate.SnapshotSummary, error) {
	if r == nil || r.questStateStore == nil {
		return queststate.SnapshotSummary{Characters: []string{}, QuestRefs: []string{}, FlagKeys: []string{}}, nil
	}
	backer, ok := r.questStateStore.(interface {
		BackupTo(string) error
		ValidateBackupFrom(string) (queststate.SnapshotSummary, error)
	})
	if !ok {
		return queststate.SnapshotSummary{}, fmt.Errorf("quest state store backup is not supported")
	}
	r.questStateMu.Lock()
	defer r.questStateMu.Unlock()
	if err := backer.BackupTo(dstDir); err != nil {
		return queststate.SnapshotSummary{}, err
	}
	return backer.ValidateBackupFrom(dstDir)
}

func (r *gameRuntime) ValidateQuestStateStoreBackup(srcDir string) (queststate.SnapshotSummary, error) {
	if r == nil || r.questStateStore == nil {
		return queststate.SnapshotSummary{Characters: []string{}, QuestRefs: []string{}, FlagKeys: []string{}}, nil
	}
	validator, ok := r.questStateStore.(interface {
		ValidateBackupFrom(string) (queststate.SnapshotSummary, error)
	})
	if !ok {
		return queststate.SnapshotSummary{}, fmt.Errorf("quest state store backup validation is not supported")
	}
	r.questStateMu.Lock()
	defer r.questStateMu.Unlock()
	return validator.ValidateBackupFrom(srcDir)
}

func (r *gameRuntime) RestoreQuestStateStore(srcDir string) (queststate.SnapshotSummary, error) {
	if r == nil || r.questStateStore == nil {
		return queststate.SnapshotSummary{Characters: []string{}, QuestRefs: []string{}, FlagKeys: []string{}}, nil
	}
	restorer, ok := r.questStateStore.(interface {
		RestoreFrom(string) error
		Validate() (queststate.SnapshotSummary, error)
	})
	if !ok {
		return queststate.SnapshotSummary{}, fmt.Errorf("quest state store restore is not supported")
	}
	r.liveCharacterMu.Lock()
	defer r.liveCharacterMu.Unlock()
	if len(r.liveCharactersByName) != 0 {
		return queststate.SnapshotSummary{}, ErrQuestStateStoreRestoreLiveSessions
	}
	r.questStateMu.Lock()
	defer r.questStateMu.Unlock()
	if err := restorer.RestoreFrom(srcDir); err != nil {
		return queststate.SnapshotSummary{}, err
	}
	return restorer.Validate()
}

func (r *gameRuntime) CleanupGroundItemStoreCrashTempFiles() (worldruntime.DurableGroundItemSnapshotSummary, error) {
	if r == nil || r.groundItemStore == nil {
		return worldruntime.DurableGroundItemSnapshotSummary{VIDs: []uint32{}}, nil
	}
	cleaner, ok := r.groundItemStore.(interface {
		CleanupCrashTempFiles() (worldruntime.DurableGroundItemSnapshotSummary, error)
	})
	if !ok {
		return worldruntime.DurableGroundItemSnapshotSummary{}, fmt.Errorf("ground item store crash temp cleanup is not supported")
	}
	r.groundItemPersistMu.Lock()
	defer r.groundItemPersistMu.Unlock()
	return cleaner.CleanupCrashTempFiles()
}

func (r *gameRuntime) BackupGroundItemStore(dstDir string) (worldruntime.DurableGroundItemSnapshotSummary, error) {
	if r == nil || r.groundItemStore == nil {
		return worldruntime.DurableGroundItemSnapshotSummary{VIDs: []uint32{}}, nil
	}
	backer, ok := r.groundItemStore.(interface {
		BackupTo(string) error
		ValidateBackupFrom(string) (worldruntime.DurableGroundItemSnapshotSummary, error)
	})
	if !ok {
		return worldruntime.DurableGroundItemSnapshotSummary{}, fmt.Errorf("ground item store backup is not supported")
	}
	r.groundItemPersistMu.Lock()
	defer r.groundItemPersistMu.Unlock()
	if err := backer.BackupTo(dstDir); err != nil {
		return worldruntime.DurableGroundItemSnapshotSummary{}, err
	}
	return backer.ValidateBackupFrom(dstDir)
}

func (r *gameRuntime) ValidateGroundItemStoreBackup(srcDir string) (worldruntime.DurableGroundItemSnapshotSummary, error) {
	if r == nil || r.groundItemStore == nil {
		return worldruntime.DurableGroundItemSnapshotSummary{VIDs: []uint32{}}, nil
	}
	validator, ok := r.groundItemStore.(interface {
		ValidateBackupFrom(string) (worldruntime.DurableGroundItemSnapshotSummary, error)
	})
	if !ok {
		return worldruntime.DurableGroundItemSnapshotSummary{}, fmt.Errorf("ground item store backup validation is not supported")
	}
	r.groundItemPersistMu.Lock()
	defer r.groundItemPersistMu.Unlock()
	return validator.ValidateBackupFrom(srcDir)
}

func (r *gameRuntime) RestoreGroundItemStore(srcDir string) (worldruntime.DurableGroundItemSnapshotSummary, error) {
	if r == nil || r.groundItemStore == nil {
		return worldruntime.DurableGroundItemSnapshotSummary{VIDs: []uint32{}}, nil
	}
	restorer, ok := r.groundItemStore.(interface {
		RestoreFrom(string) error
		Validate() (worldruntime.DurableGroundItemSnapshotSummary, error)
		Load() (worldruntime.DurableGroundItemSnapshot, error)
	})
	if !ok {
		return worldruntime.DurableGroundItemSnapshotSummary{}, fmt.Errorf("ground item store restore is not supported")
	}
	r.liveCharacterMu.Lock()
	defer r.liveCharacterMu.Unlock()
	if len(r.liveCharactersByName) != 0 {
		return worldruntime.DurableGroundItemSnapshotSummary{}, ErrGroundItemStoreRestoreLiveSessions
	}
	r.groundItemPersistMu.Lock()
	defer r.groundItemPersistMu.Unlock()
	if err := restorer.RestoreFrom(srcDir); err != nil {
		return worldruntime.DurableGroundItemSnapshotSummary{}, err
	}
	if err := r.rematerializeGroundItemsAfterRestoreLocked(restorer); err != nil {
		return worldruntime.DurableGroundItemSnapshotSummary{}, err
	}
	return restorer.Validate()
}

func (r *gameRuntime) ValidateSafeboxStore() (safeboxstore.SnapshotSummary, error) {
	if r == nil || r.safeboxStore == nil {
		return safeboxstore.SnapshotSummary{Logins: []string{}, CharacterKeys: []string{}}, nil
	}
	if validator, ok := r.safeboxStore.(interface {
		Validate() (safeboxstore.SnapshotSummary, error)
	}); ok {
		r.safeboxPersistMu.Lock()
		defer r.safeboxPersistMu.Unlock()
		return validator.Validate()
	}
	snapshot, err := r.safeboxStore.Load()
	if err != nil {
		if errors.Is(err, safeboxstore.ErrSnapshotNotFound) {
			return safeboxstore.SnapshotSummary{Logins: []string{}, CharacterKeys: []string{}}, nil
		}
		return safeboxstore.SnapshotSummary{}, err
	}
	return safeboxstore.SummarizeSnapshot(snapshot)
}

func (r *gameRuntime) CleanupSafeboxStoreCrashTempFiles() (safeboxstore.SnapshotSummary, error) {
	if r == nil || r.safeboxStore == nil {
		return safeboxstore.SnapshotSummary{Logins: []string{}, CharacterKeys: []string{}}, nil
	}
	cleaner, ok := r.safeboxStore.(interface {
		CleanupCrashTempFiles() (safeboxstore.SnapshotSummary, error)
	})
	if !ok {
		return safeboxstore.SnapshotSummary{}, fmt.Errorf("safebox store crash temp cleanup is not supported")
	}
	r.safeboxPersistMu.Lock()
	defer r.safeboxPersistMu.Unlock()
	return cleaner.CleanupCrashTempFiles()
}

func (r *gameRuntime) BackupSafeboxStore(dstDir string) (safeboxstore.SnapshotSummary, error) {
	if r == nil || r.safeboxStore == nil {
		return safeboxstore.SnapshotSummary{Logins: []string{}, CharacterKeys: []string{}}, nil
	}
	backer, ok := r.safeboxStore.(interface {
		BackupTo(string) error
		ValidateBackupFrom(string) (safeboxstore.SnapshotSummary, error)
	})
	if !ok {
		return safeboxstore.SnapshotSummary{}, fmt.Errorf("safebox store backup is not supported")
	}
	r.safeboxPersistMu.Lock()
	defer r.safeboxPersistMu.Unlock()
	if err := backer.BackupTo(dstDir); err != nil {
		return safeboxstore.SnapshotSummary{}, err
	}
	return backer.ValidateBackupFrom(dstDir)
}

func (r *gameRuntime) ValidateSafeboxStoreBackup(srcDir string) (safeboxstore.SnapshotSummary, error) {
	if r == nil || r.safeboxStore == nil {
		return safeboxstore.SnapshotSummary{Logins: []string{}, CharacterKeys: []string{}}, nil
	}
	validator, ok := r.safeboxStore.(interface {
		ValidateBackupFrom(string) (safeboxstore.SnapshotSummary, error)
	})
	if !ok {
		return safeboxstore.SnapshotSummary{}, fmt.Errorf("safebox store backup validation is not supported")
	}
	r.safeboxPersistMu.Lock()
	defer r.safeboxPersistMu.Unlock()
	return validator.ValidateBackupFrom(srcDir)
}

func (r *gameRuntime) RestoreSafeboxStore(srcDir string) (safeboxstore.SnapshotSummary, error) {
	if r == nil || r.safeboxStore == nil {
		return safeboxstore.SnapshotSummary{Logins: []string{}, CharacterKeys: []string{}}, nil
	}
	restorer, ok := r.safeboxStore.(interface {
		RestoreFrom(string) error
		Validate() (safeboxstore.SnapshotSummary, error)
	})
	if !ok {
		return safeboxstore.SnapshotSummary{}, fmt.Errorf("safebox store restore is not supported")
	}
	r.liveCharacterMu.Lock()
	defer r.liveCharacterMu.Unlock()
	if len(r.liveCharactersByName) != 0 {
		return safeboxstore.SnapshotSummary{}, ErrSafeboxStoreRestoreLiveSessions
	}
	r.safeboxPersistMu.Lock()
	defer r.safeboxPersistMu.Unlock()
	if err := restorer.RestoreFrom(srcDir); err != nil {
		return safeboxstore.SnapshotSummary{}, err
	}
	return restorer.Validate()
}

func (r *gameRuntime) rematerializeGroundItemsAfterRestoreLocked(loader interface {
	Load() (worldruntime.DurableGroundItemSnapshot, error)
}) error {
	if r == nil || r.sharedWorld == nil || loader == nil {
		return nil
	}
	r.sharedWorld.ClearPersistedGroundItems()
	snapshot, err := loader.Load()
	if err != nil {
		if errors.Is(err, worldruntime.ErrGroundItemSnapshotNotFound) {
			return nil
		}
		return err
	}
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	filtered := worldruntime.FilterDurableGroundItemSnapshotForRestore(snapshot, now)
	if err := r.sharedWorld.RestorePersistedGroundItems(filtered.GroundItems); err != nil {
		return err
	}
	live := r.sharedWorld.DurableGroundItemSnapshot()
	if err := r.groundItemStore.Save(live); err != nil {
		return err
	}
	// Save clears a restored manifest; rewrite it against the filtered live set.
	if refresher, ok := r.groundItemStore.(interface {
		RefreshActiveBackupManifest() error
	}); ok {
		return refresher.RefreshActiveBackupManifest()
	}
	return nil
}

func (r *gameRuntime) BackupItemTemplateStore(dstDir string) (itemcatalog.SnapshotSummary, error) {
	if r == nil || r.itemStore == nil {
		return itemcatalog.SnapshotSummary{Vnums: []uint32{}}, nil
	}
	backer, ok := r.itemStore.(interface {
		BackupTo(string) error
		ValidateBackupFrom(string) (itemcatalog.SnapshotSummary, error)
	})
	if !ok {
		return itemcatalog.SnapshotSummary{}, fmt.Errorf("item template store backup is not supported")
	}
	if err := backer.BackupTo(dstDir); err != nil {
		return itemcatalog.SnapshotSummary{}, err
	}
	return backer.ValidateBackupFrom(dstDir)
}

func (r *gameRuntime) ValidateItemTemplateStoreBackup(srcDir string) (itemcatalog.SnapshotSummary, error) {
	if r == nil || r.itemStore == nil {
		return itemcatalog.SnapshotSummary{Vnums: []uint32{}}, nil
	}
	validator, ok := r.itemStore.(interface {
		ValidateBackupFrom(string) (itemcatalog.SnapshotSummary, error)
	})
	if !ok {
		return itemcatalog.SnapshotSummary{}, fmt.Errorf("item template store backup validation is not supported")
	}
	return validator.ValidateBackupFrom(srcDir)
}

func (r *gameRuntime) RestoreItemTemplateStore(srcDir string) (itemcatalog.SnapshotSummary, error) {
	if r == nil || r.itemStore == nil {
		return itemcatalog.SnapshotSummary{Vnums: []uint32{}}, nil
	}
	restorer, ok := r.itemStore.(interface {
		RestoreFrom(string) error
		Validate() (itemcatalog.SnapshotSummary, error)
	})
	if !ok {
		return itemcatalog.SnapshotSummary{}, fmt.Errorf("item template store restore is not supported")
	}
	r.liveCharacterMu.Lock()
	defer r.liveCharacterMu.Unlock()
	if len(r.liveCharactersByName) != 0 {
		return itemcatalog.SnapshotSummary{}, ErrItemTemplateStoreRestoreLiveSessions
	}
	if err := restorer.RestoreFrom(srcDir); err != nil {
		return itemcatalog.SnapshotSummary{}, err
	}
	if err := r.loadItemTemplates(); err != nil {
		return itemcatalog.SnapshotSummary{}, err
	}
	return restorer.Validate()
}

func (r *gameRuntime) BackupAccountStore(dstDir string) (accountstore.SnapshotSummary, error) {
	if r == nil || r.accountStore == nil {
		return accountstore.SnapshotSummary{Logins: []string{}}, nil
	}
	backer, ok := r.accountStore.(interface {
		BackupTo(string) error
	})
	if !ok {
		return accountstore.SnapshotSummary{}, fmt.Errorf("account store backup is not supported")
	}
	if err := backer.BackupTo(dstDir); err != nil {
		return accountstore.SnapshotSummary{}, err
	}
	backup := accountstore.NewFileStore(dstDir)
	return backup.Validate()
}

func (r *gameRuntime) ValidateAccountStoreBackup(srcDir string) (accountstore.SnapshotSummary, error) {
	if r == nil || r.accountStore == nil {
		return accountstore.SnapshotSummary{Logins: []string{}}, nil
	}
	validator, ok := r.accountStore.(interface {
		ValidateBackupFrom(string) (accountstore.SnapshotSummary, error)
	})
	if !ok {
		return accountstore.SnapshotSummary{}, fmt.Errorf("account store backup validation is not supported")
	}
	return validator.ValidateBackupFrom(srcDir)
}

func (r *gameRuntime) RestoreAccountStore(srcDir string) (accountstore.SnapshotSummary, error) {
	if r == nil || r.accountStore == nil {
		return accountstore.SnapshotSummary{Logins: []string{}}, nil
	}
	restorer, ok := r.accountStore.(interface {
		RestoreFrom(string) error
	})
	if !ok {
		return accountstore.SnapshotSummary{}, fmt.Errorf("account store restore is not supported")
	}
	r.liveCharacterMu.Lock()
	defer r.liveCharacterMu.Unlock()
	if len(r.liveCharactersByName) != 0 {
		return accountstore.SnapshotSummary{}, ErrAccountStoreRestoreLiveSessions
	}
	if err := restorer.RestoreFrom(srcDir); err != nil {
		return accountstore.SnapshotSummary{}, err
	}
	return r.ValidateAccountStore()
}

func (r *gameRuntime) BackupLoginTicketStore(dstDir string) (loginticket.SnapshotSummary, error) {
	if r == nil || r.loginTicketStore == nil {
		return loginticket.SnapshotSummary{Logins: []string{}, LoginKeys: []uint32{}}, nil
	}
	backer, ok := r.loginTicketStore.(interface {
		BackupTo(string) error
	})
	if !ok {
		return loginticket.SnapshotSummary{}, fmt.Errorf("login ticket store backup is not supported")
	}
	if err := backer.BackupTo(dstDir); err != nil {
		return loginticket.SnapshotSummary{}, err
	}
	backup := loginticket.NewFileStore(dstDir)
	return backup.Validate()
}

func (r *gameRuntime) ValidateLoginTicketStoreBackup(srcDir string) (loginticket.SnapshotSummary, error) {
	if r == nil || r.loginTicketStore == nil {
		return loginticket.SnapshotSummary{Logins: []string{}, LoginKeys: []uint32{}}, nil
	}
	validator, ok := r.loginTicketStore.(interface {
		ValidateBackupFrom(string) (loginticket.SnapshotSummary, error)
	})
	if !ok {
		return loginticket.SnapshotSummary{}, fmt.Errorf("login ticket store backup validation is not supported")
	}
	return validator.ValidateBackupFrom(srcDir)
}

func (r *gameRuntime) RestoreLoginTicketStore(srcDir string) (loginticket.SnapshotSummary, error) {
	if r == nil || r.loginTicketStore == nil {
		return loginticket.SnapshotSummary{Logins: []string{}, LoginKeys: []uint32{}}, nil
	}
	restorer, ok := r.loginTicketStore.(interface {
		RestoreFrom(string) error
	})
	if !ok {
		return loginticket.SnapshotSummary{}, fmt.Errorf("login ticket store restore is not supported")
	}
	r.liveCharacterMu.Lock()
	defer r.liveCharacterMu.Unlock()
	if len(r.liveCharactersByName) != 0 {
		return loginticket.SnapshotSummary{}, ErrLoginTicketStoreRestoreLiveSessions
	}
	if err := restorer.RestoreFrom(srcDir); err != nil {
		return loginticket.SnapshotSummary{}, err
	}
	return r.ValidateLoginTicketStore()
}

func (r *gameRuntime) ExportAccountCharacterRoster() (accountstore.AccountCharacterRosterExport, error) {
	if r == nil || r.accountStore == nil {
		return accountstore.ExportAccountCharacterRoster(nil)
	}
	exporter, ok := r.accountStore.(accountstore.AccountCharacterStateExporter)
	if !ok {
		return accountstore.AccountCharacterRosterExport{}, fmt.Errorf("account/character roster export is not supported")
	}
	return exporter.ExportAccountCharacterRoster()
}

func (r *gameRuntime) ExportCharacterItemState() (accountstore.CharacterItemStateExport, error) {
	if r == nil || r.accountStore == nil {
		return accountstore.ExportCharacterItemState(nil)
	}
	exporter, ok := r.accountStore.(accountstore.AccountCharacterStateExporter)
	if !ok {
		return accountstore.CharacterItemStateExport{}, fmt.Errorf("character item-state export is not supported")
	}
	return exporter.ExportCharacterItemState()
}

func (r *gameRuntime) ExportCharacterPointState() (accountstore.CharacterPointStateExport, error) {
	if r == nil || r.accountStore == nil {
		return accountstore.ExportCharacterPointState(nil)
	}
	exporter, ok := r.accountStore.(accountstore.AccountCharacterStateExporter)
	if !ok {
		return accountstore.CharacterPointStateExport{}, fmt.Errorf("character point-state export is not supported")
	}
	return exporter.ExportCharacterPointState()
}

func (r *gameRuntime) ExportAuthLoginTicketHandoff() (loginticket.AuthLoginTicketHandoffExport, error) {
	if r == nil || r.loginTicketStore == nil {
		return loginticket.ExportAuthLoginTicketHandoff(nil)
	}
	exporter, ok := r.loginTicketStore.(loginticket.AuthLoginTicketHandoffExporter)
	if !ok {
		return loginticket.AuthLoginTicketHandoffExport{}, fmt.Errorf("auth login-ticket handoff export is not supported")
	}
	return exporter.ExportAuthLoginTicketHandoff()
}

func (r *gameRuntime) ExportCharacterQuestState() (queststate.CharacterQuestStateExport, error) {
	if r == nil || r.questStateStore == nil {
		return queststate.ExportCharacterQuestState(queststate.Snapshot{}, nil)
	}

	roster, err := r.ExportAccountCharacterRoster()
	if err != nil {
		return queststate.CharacterQuestStateExport{}, err
	}
	characterIDsByName := make(map[string]uint32, len(roster.Characters)*2)
	for _, character := range roster.Characters {
		characterIDsByName[character.Name] = character.ID
		characterIDsByName[character.NameNormalized] = character.ID
	}

	exporter, ok := r.questStateStore.(queststate.CharacterQuestStateExporter)
	if !ok {
		return queststate.CharacterQuestStateExport{}, fmt.Errorf("character quest-state export is not supported")
	}
	r.questStateMu.Lock()
	defer r.questStateMu.Unlock()
	return exporter.ExportCharacterQuestState(characterIDsByName)
}

func (r *gameRuntime) ExportCharacterSafeboxState() (safeboxstore.CharacterSafeboxStateExport, error) {
	if r == nil || r.safeboxStore == nil {
		return safeboxstore.ExportCharacterSafeboxState(safeboxstore.Snapshot{})
	}
	exporter, ok := r.safeboxStore.(safeboxstore.CharacterSafeboxStateExporter)
	if !ok {
		return safeboxstore.CharacterSafeboxStateExport{}, fmt.Errorf("character safebox-state export is not supported")
	}
	return exporter.ExportCharacterSafeboxState()
}

func (r *gameRuntime) ExportItemTemplateState() (itemcatalog.ItemTemplateStateExport, error) {
	if r == nil || r.itemStore == nil {
		return itemcatalog.ExportItemTemplateState(itemcatalog.Snapshot{})
	}
	exporter, ok := r.itemStore.(itemcatalog.ItemTemplateStateExporter)
	if !ok {
		return itemcatalog.ItemTemplateStateExport{}, fmt.Errorf("item-template-state export is not supported")
	}
	return exporter.ExportItemTemplateState()
}

func (r *gameRuntime) ExportStaticActorContentState() (staticstore.StaticActorContentStateExport, error) {
	if r == nil || r.staticStore == nil {
		return staticstore.ExportStaticActorContentState(staticstore.Snapshot{}, interactionstore.Snapshot{})
	}
	exporter, ok := r.staticStore.(staticstore.StaticActorContentStateExporter)
	if !ok {
		return staticstore.StaticActorContentStateExport{}, fmt.Errorf("static-actor content-state export is not supported")
	}
	return exporter.ExportStaticActorContentState(r.interactionStore)
}

func (r *gameRuntime) ExportBootstrapGroundItemState() (worldruntime.BootstrapGroundItemStateExport, error) {
	if r == nil {
		return worldruntime.ExportBootstrapGroundItemState(nil)
	}
	if r.groundItemExporter != nil {
		return r.groundItemExporter.ExportBootstrapGroundItemState()
	}
	return worldruntime.ExportBootstrapGroundItemState(r.GroundItems())
}

func (r *gameRuntime) flushReadyStaticActorRespawns() {
	if r == nil || r.sharedWorld == nil {
		return
	}
	if r.staticStore == nil {
		now := r.spawnGroupReturnStepNow()
		for _, respawn := range r.sharedWorld.StaticActorRespawns() {
			if respawn.EntityID == 0 || respawn.ReadyAt.IsZero() || now.Before(respawn.ReadyAt) {
				continue
			}
			if r.sharedWorld.FlushReadyStaticActorRespawn(respawn.EntityID) {
				r.syncSpawnGroupReturnStepScheduleForEntity(respawn.EntityID)
			}
		}
		return
	}

	r.staticActorMu.Lock()
	defer r.staticActorMu.Unlock()

	now := r.spawnGroupReturnStepNow()
	for _, respawn := range r.sharedWorld.StaticActorRespawns() {
		if respawn.EntityID == 0 || respawn.ReadyAt.IsZero() || now.Before(respawn.ReadyAt) {
			continue
		}
		current := r.sharedWorld.StaticActors()
		idx := staticActorSnapshotIndex(current, respawn.EntityID)
		if idx == -1 {
			if r.sharedWorld.FlushReadyStaticActorRespawn(respawn.EntityID) {
				r.syncSpawnGroupReturnStepScheduleForEntity(respawn.EntityID)
			}
			continue
		}
		target := cloneStaticActorSnapshots(current)
		if target[idx].SpawnGroupRef != "" && target[idx].SpawnHome != nil {
			home := *target[idx].SpawnHome
			target[idx].MapIndex = home.MapIndex
			target[idx].X = home.X
			target[idx].Y = home.Y
		}
		if !r.persistStaticActorSnapshot(target) {
			continue
		}
		if !r.sharedWorld.FlushReadyStaticActorRespawn(respawn.EntityID) {
			_ = r.persistStaticActorSnapshot(current)
			continue
		}
		r.syncSpawnGroupReturnStepScheduleForEntity(respawn.EntityID)
		r.clearSpawnGroupChaseStep(respawn.EntityID)
		r.clearSpawnGroupHomewardStep(respawn.EntityID)
		// Persist again after the live rebuild so still-dead HP / absolute deadline
		// fields are cleared from the static-actor snapshot before the next restart.
		_ = r.persistStaticActorSnapshot(r.sharedWorld.StaticActors())
	}
}

func (r *gameRuntime) syncSpawnGroupReturnStepScheduleForEntity(entityID uint64) {
	if r == nil || entityID == 0 {
		return
	}
	actor, ok := r.SpawnGroup(entityID)
	if !ok {
		r.clearSpawnGroupReturnStep(entityID)
		return
	}
	r.syncSpawnGroupReturnStepSchedule(actor)
}

func (r *gameRuntime) scheduleSpawnGroupReturnStep(entityID uint64) {
	if r == nil || entityID == 0 || bootstrapSpawnGroupReturnStepDelay <= 0 {
		return
	}
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	r.spawnReturnMu.Lock()
	defer r.spawnReturnMu.Unlock()
	if r.spawnReturnStepDueAt == nil {
		r.spawnReturnStepDueAt = make(map[uint64]time.Time)
	}
	r.spawnReturnStepDueAt[entityID] = now.Add(bootstrapSpawnGroupReturnStepDelay)
}

func (r *gameRuntime) clearSpawnGroupReturnStep(entityID uint64) {
	if r == nil || entityID == 0 {
		return
	}
	r.spawnReturnMu.Lock()
	defer r.spawnReturnMu.Unlock()
	delete(r.spawnReturnStepDueAt, entityID)
}

func (r *gameRuntime) syncSpawnGroupReturnStepSchedule(actor StaticActorSnapshot) {
	if r == nil || actor.EntityID == 0 {
		return
	}
	if actor.SpawnGroupRef != "" && actor.SpawnLeash != nil && actor.SpawnLeash.ReturnRequired && !actor.Dead {
		r.scheduleSpawnGroupReturnStep(actor.EntityID)
		return
	}
	r.clearSpawnGroupReturnStep(actor.EntityID)
}

func (r *gameRuntime) dueSpawnGroupReturnStepIDs() []uint64 {
	if r == nil {
		return nil
	}
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	r.spawnReturnMu.Lock()
	defer r.spawnReturnMu.Unlock()
	if len(r.spawnReturnStepDueAt) == 0 {
		return nil
	}
	dueIDs := make([]uint64, 0, len(r.spawnReturnStepDueAt))
	for entityID, dueAt := range r.spawnReturnStepDueAt {
		if dueAt.IsZero() || now.Before(dueAt) {
			continue
		}
		dueIDs = append(dueIDs, entityID)
	}
	sort.Slice(dueIDs, func(i, j int) bool { return dueIDs[i] < dueIDs[j] })
	return dueIDs
}

func (r *gameRuntime) SpawnGroupReturnSteps() []SpawnGroupPendingReturnStepSnapshot {
	if r == nil {
		return nil
	}
	dueAtByID := r.spawnGroupReturnStepDueAtSnapshot()
	if len(dueAtByID) == 0 {
		return nil
	}
	entityIDs := make([]uint64, 0, len(dueAtByID))
	for entityID := range dueAtByID {
		entityIDs = append(entityIDs, entityID)
	}
	sort.Slice(entityIDs, func(i, j int) bool { return entityIDs[i] < entityIDs[j] })

	now := r.spawnGroupReturnStepNow()
	snapshots := make([]SpawnGroupPendingReturnStepSnapshot, 0, len(entityIDs))
	for _, entityID := range entityIDs {
		snapshot, ok := r.spawnGroupReturnStepSnapshot(entityID, dueAtByID[entityID], now)
		if !ok {
			continue
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func (r *gameRuntime) SpawnGroupReturnStep(entityID uint64) (SpawnGroupPendingReturnStepSnapshot, bool) {
	if r == nil || entityID == 0 {
		return SpawnGroupPendingReturnStepSnapshot{}, false
	}
	r.spawnReturnMu.Lock()
	dueAt, ok := r.spawnReturnStepDueAt[entityID]
	r.spawnReturnMu.Unlock()
	if !ok {
		return SpawnGroupPendingReturnStepSnapshot{}, false
	}
	return r.spawnGroupReturnStepSnapshot(entityID, dueAt, r.spawnGroupReturnStepNow())
}

func (r *gameRuntime) SpawnGroupReturnStepsForMap(mapIndex uint32) ([]SpawnGroupPendingReturnStepSnapshot, bool) {
	if r == nil || r.sharedWorld == nil || mapIndex == 0 {
		return nil, false
	}
	if _, ok := r.MapOccupancySnapshot(mapIndex); !ok {
		return nil, false
	}

	all := r.SpawnGroupReturnSteps()
	if len(all) == 0 {
		return []SpawnGroupPendingReturnStepSnapshot{}, true
	}
	filtered := make([]SpawnGroupPendingReturnStepSnapshot, 0, len(all))
	for _, snapshot := range all {
		if snapshot.Actor.MapIndex != mapIndex {
			continue
		}
		filtered = append(filtered, snapshot)
	}
	return filtered, true
}

func (r *gameRuntime) spawnGroupReturnStepDueAtSnapshot() map[uint64]time.Time {
	if r == nil {
		return nil
	}
	r.spawnReturnMu.Lock()
	defer r.spawnReturnMu.Unlock()
	if len(r.spawnReturnStepDueAt) == 0 {
		return nil
	}
	snapshot := make(map[uint64]time.Time, len(r.spawnReturnStepDueAt))
	for entityID, dueAt := range r.spawnReturnStepDueAt {
		snapshot[entityID] = dueAt
	}
	return snapshot
}

func (r *gameRuntime) restoreSpawnGroupReturnStepDueAtSnapshot(snapshot map[uint64]time.Time) {
	if r == nil {
		return
	}
	restored := make(map[uint64]time.Time, len(snapshot))
	for entityID, dueAt := range snapshot {
		if entityID == 0 || dueAt.IsZero() || !r.spawnGroupReturnStepStillRequired(entityID) {
			continue
		}
		restored[entityID] = dueAt
	}
	r.spawnReturnMu.Lock()
	defer r.spawnReturnMu.Unlock()
	if r.spawnReturnStepDueAt == nil {
		r.spawnReturnStepDueAt = make(map[uint64]time.Time, len(restored))
	}
	for entityID := range r.spawnReturnStepDueAt {
		delete(r.spawnReturnStepDueAt, entityID)
	}
	for entityID, dueAt := range restored {
		r.spawnReturnStepDueAt[entityID] = dueAt
	}
}

func (r *gameRuntime) pruneSpawnGroupReturnStepSchedules() {
	r.restoreSpawnGroupReturnStepDueAtSnapshot(r.spawnGroupReturnStepDueAtSnapshot())
}

func (r *gameRuntime) spawnGroupReturnStepNow() time.Time {
	now := time.Now()
	if r != nil && r.now != nil {
		now = r.now()
	}
	return now
}

func (r *gameRuntime) spawnGroupReturnStepSnapshot(entityID uint64, dueAt time.Time, now time.Time) (SpawnGroupPendingReturnStepSnapshot, bool) {
	if r == nil || r.sharedWorld == nil || entityID == 0 || dueAt.IsZero() {
		return SpawnGroupPendingReturnStepSnapshot{}, false
	}
	actor, ok := r.SpawnGroup(entityID)
	if !ok || actor.SpawnLeash == nil || !actor.SpawnLeash.ReturnRequired {
		return SpawnGroupPendingReturnStepSnapshot{}, false
	}
	plan, ok := r.sharedWorld.PlanSpawnGroupReturnHomeStep(entityID, bootstrapSpawnGroupReturnStepMaxStep)
	if !ok || !plan.Evaluation.ReturnRequired {
		return SpawnGroupPendingReturnStepSnapshot{}, false
	}
	remaining := dueAt.Sub(now).Milliseconds()
	if remaining < 0 {
		remaining = 0
	}
	return SpawnGroupPendingReturnStepSnapshot{
		EntityID:    entityID,
		ReadyAt:     dueAt,
		RemainingMs: remaining,
		Actor:       actor,
		Step: SpawnLeashReturnStepSnapshot{
			SpawnLeashSnapshot: worldruntime.SpawnLeashSnapshotFromEvaluation(plan.Evaluation),
			Next:               worldruntime.PositionSnapshotFromPosition(plan.Next),
			Complete:           plan.Complete,
		},
	}, true
}

func (r *gameRuntime) flushDueSpawnGroupReturnSteps() {
	if r == nil {
		return
	}
	for _, entityID := range r.dueSpawnGroupReturnStepIDs() {
		step, ok := r.stepSpawnGroupReturnHome(entityID, bootstrapSpawnGroupReturnStepMaxStep, true)
		if !ok {
			if r.spawnGroupReturnStepStillRequired(entityID) {
				r.scheduleSpawnGroupReturnStep(entityID)
				continue
			}
			r.clearSpawnGroupReturnStep(entityID)
			continue
		}
		if step.Step.Complete {
			r.clearSpawnGroupReturnStep(entityID)
			continue
		}
	}
}

func (r *gameRuntime) spawnGroupReturnStepStillRequired(entityID uint64) bool {
	if r == nil || entityID == 0 {
		return false
	}
	snapshot, ok := r.SpawnGroup(entityID)
	return ok && !snapshot.Dead && snapshot.SpawnLeash != nil && snapshot.SpawnLeash.ReturnRequired
}

func (r *gameRuntime) scheduleSpawnGroupChaseStep(entityID uint64) {
	if r == nil || entityID == 0 || bootstrapSpawnGroupChaseStepDelay <= 0 {
		return
	}
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	r.spawnChaseMu.Lock()
	defer r.spawnChaseMu.Unlock()
	if r.spawnChaseStepDueAt == nil {
		r.spawnChaseStepDueAt = make(map[uint64]time.Time)
	}
	r.spawnChaseStepDueAt[entityID] = now.Add(bootstrapSpawnGroupChaseStepDelay)
}

func (r *gameRuntime) clearSpawnGroupChaseStep(entityID uint64) {
	if r == nil || entityID == 0 {
		return
	}
	r.spawnChaseMu.Lock()
	defer r.spawnChaseMu.Unlock()
	delete(r.spawnChaseStepDueAt, entityID)
}

func (r *gameRuntime) syncSpawnGroupChaseStepScheduleForEntity(entityID uint64) {
	if r == nil || entityID == 0 || r.sharedWorld == nil {
		return
	}
	actor, ok := r.SpawnGroup(entityID)
	if !ok || actor.Dead || actor.SpawnGroupRef == "" || actor.SpawnLeash == nil || actor.SpawnLeash.ReturnRequired {
		r.clearSpawnGroupChaseStep(entityID)
		r.clearSpawnGroupHomewardStep(entityID)
		return
	}
	r.sharedWorld.mu.Lock()
	engagedBy, engaged := r.sharedWorld.staticActorCombatEngagedBy[entityID]
	r.sharedWorld.mu.Unlock()
	if !engaged || engagedBy == 0 {
		r.clearSpawnGroupChaseStep(entityID)
		r.syncSpawnGroupHomewardStepScheduleForEntity(entityID)
		return
	}
	owner, ok := r.sharedWorld.playerCharacter(engagedBy)
	if !ok || characterAtBootstrapHPFloor(owner) {
		r.clearSpawnGroupChaseStep(entityID)
		r.syncSpawnGroupHomewardStepScheduleForEntity(entityID)
		return
	}
	ownerPos := worldruntime.NewPosition(owner.MapIndex, owner.X, owner.Y)
	if _, ok := r.sharedWorld.PlanSpawnGroupChaseStep(entityID, ownerPos, bootstrapSpawnGroupChaseStepMaxStep); !ok {
		r.clearSpawnGroupChaseStep(entityID)
		r.syncSpawnGroupHomewardStepScheduleForEntity(entityID)
		return
	}
	r.clearSpawnGroupHomewardStep(entityID)
	r.scheduleSpawnGroupChaseStep(entityID)
}

func (r *gameRuntime) dueSpawnGroupChaseStepIDs() []uint64 {
	if r == nil {
		return nil
	}
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	r.spawnChaseMu.Lock()
	defer r.spawnChaseMu.Unlock()
	if len(r.spawnChaseStepDueAt) == 0 {
		return nil
	}
	dueIDs := make([]uint64, 0, len(r.spawnChaseStepDueAt))
	for entityID, dueAt := range r.spawnChaseStepDueAt {
		if dueAt.IsZero() || now.Before(dueAt) {
			continue
		}
		dueIDs = append(dueIDs, entityID)
	}
	sort.Slice(dueIDs, func(i, j int) bool { return dueIDs[i] < dueIDs[j] })
	return dueIDs
}

func (r *gameRuntime) spawnGroupChaseStepDueAtSnapshot() map[uint64]time.Time {
	if r == nil {
		return nil
	}
	r.spawnChaseMu.Lock()
	defer r.spawnChaseMu.Unlock()
	if len(r.spawnChaseStepDueAt) == 0 {
		return nil
	}
	snapshot := make(map[uint64]time.Time, len(r.spawnChaseStepDueAt))
	for entityID, dueAt := range r.spawnChaseStepDueAt {
		snapshot[entityID] = dueAt
	}
	return snapshot
}

func (r *gameRuntime) restoreSpawnGroupChaseStepDueAtSnapshot(snapshot map[uint64]time.Time) {
	if r == nil {
		return
	}
	restored := make(map[uint64]time.Time, len(snapshot))
	for entityID, dueAt := range snapshot {
		if entityID == 0 || dueAt.IsZero() || !r.spawnGroupChaseStepStillEligible(entityID) {
			continue
		}
		restored[entityID] = dueAt
	}
	r.spawnChaseMu.Lock()
	defer r.spawnChaseMu.Unlock()
	if r.spawnChaseStepDueAt == nil {
		r.spawnChaseStepDueAt = make(map[uint64]time.Time, len(restored))
	}
	for entityID := range r.spawnChaseStepDueAt {
		delete(r.spawnChaseStepDueAt, entityID)
	}
	for entityID, dueAt := range restored {
		r.spawnChaseStepDueAt[entityID] = dueAt
	}
}

func (r *gameRuntime) pruneSpawnGroupChaseStepSchedules() {
	r.restoreSpawnGroupChaseStepDueAtSnapshot(r.spawnGroupChaseStepDueAtSnapshot())
}

func (r *gameRuntime) spawnGroupChaseStepStillEligible(entityID uint64) bool {
	if r == nil || entityID == 0 || r.sharedWorld == nil {
		return false
	}
	snapshot, ok := r.SpawnGroup(entityID)
	if !ok || snapshot.Dead || snapshot.SpawnGroupRef == "" || snapshot.SpawnLeash == nil || snapshot.SpawnLeash.ReturnRequired {
		return false
	}
	r.sharedWorld.mu.Lock()
	engagedBy, engaged := r.sharedWorld.staticActorCombatEngagedBy[entityID]
	r.sharedWorld.mu.Unlock()
	if !engaged || engagedBy == 0 {
		return false
	}
	owner, ok := r.sharedWorld.playerCharacter(engagedBy)
	if !ok || characterAtBootstrapHPFloor(owner) {
		return false
	}
	ownerPos := worldruntime.NewPosition(owner.MapIndex, owner.X, owner.Y)
	_, ok = r.sharedWorld.PlanSpawnGroupChaseStep(entityID, ownerPos, bootstrapSpawnGroupChaseStepMaxStep)
	return ok
}

func (r *gameRuntime) SpawnGroupChaseSteps() []SpawnGroupPendingChaseStepSnapshot {
	if r == nil {
		return nil
	}
	dueAtByID := r.spawnGroupChaseStepDueAtSnapshot()
	if len(dueAtByID) == 0 {
		return nil
	}
	entityIDs := make([]uint64, 0, len(dueAtByID))
	for entityID := range dueAtByID {
		entityIDs = append(entityIDs, entityID)
	}
	sort.Slice(entityIDs, func(i, j int) bool { return entityIDs[i] < entityIDs[j] })

	now := r.spawnGroupChaseStepNow()
	snapshots := make([]SpawnGroupPendingChaseStepSnapshot, 0, len(entityIDs))
	for _, entityID := range entityIDs {
		snapshot, ok := r.spawnGroupChaseStepSnapshot(entityID, dueAtByID[entityID], now)
		if !ok {
			continue
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func (r *gameRuntime) SpawnGroupChaseStep(entityID uint64) (SpawnGroupPendingChaseStepSnapshot, bool) {
	if r == nil || entityID == 0 {
		return SpawnGroupPendingChaseStepSnapshot{}, false
	}
	r.spawnChaseMu.Lock()
	dueAt, ok := r.spawnChaseStepDueAt[entityID]
	r.spawnChaseMu.Unlock()
	if !ok {
		return SpawnGroupPendingChaseStepSnapshot{}, false
	}
	return r.spawnGroupChaseStepSnapshot(entityID, dueAt, r.spawnGroupChaseStepNow())
}

func (r *gameRuntime) SpawnGroupChaseStepsForMap(mapIndex uint32) ([]SpawnGroupPendingChaseStepSnapshot, bool) {
	if r == nil || r.sharedWorld == nil || mapIndex == 0 {
		return nil, false
	}
	if _, ok := r.MapOccupancySnapshot(mapIndex); !ok {
		return nil, false
	}

	all := r.SpawnGroupChaseSteps()
	if len(all) == 0 {
		return []SpawnGroupPendingChaseStepSnapshot{}, true
	}
	filtered := make([]SpawnGroupPendingChaseStepSnapshot, 0, len(all))
	for _, snapshot := range all {
		if snapshot.Actor.MapIndex != mapIndex {
			continue
		}
		filtered = append(filtered, snapshot)
	}
	return filtered, true
}

func (r *gameRuntime) spawnGroupChaseStepNow() time.Time {
	now := time.Now()
	if r != nil && r.now != nil {
		now = r.now()
	}
	return now
}

func (r *gameRuntime) spawnGroupChaseStepSnapshot(entityID uint64, dueAt time.Time, now time.Time) (SpawnGroupPendingChaseStepSnapshot, bool) {
	if r == nil || r.sharedWorld == nil || entityID == 0 || dueAt.IsZero() {
		return SpawnGroupPendingChaseStepSnapshot{}, false
	}
	actor, ok := r.SpawnGroup(entityID)
	if !ok || actor.Dead || actor.SpawnGroupRef == "" || actor.SpawnLeash == nil || actor.SpawnLeash.ReturnRequired {
		return SpawnGroupPendingChaseStepSnapshot{}, false
	}
	r.sharedWorld.mu.Lock()
	engagedBy, engaged := r.sharedWorld.staticActorCombatEngagedBy[entityID]
	r.sharedWorld.mu.Unlock()
	if !engaged || engagedBy == 0 {
		return SpawnGroupPendingChaseStepSnapshot{}, false
	}
	owner, ok := r.sharedWorld.playerCharacter(engagedBy)
	if !ok || characterAtBootstrapHPFloor(owner) {
		return SpawnGroupPendingChaseStepSnapshot{}, false
	}
	ownerPos := worldruntime.NewPosition(owner.MapIndex, owner.X, owner.Y)
	plan, ok := r.sharedWorld.PlanSpawnGroupChaseStep(entityID, ownerPos, bootstrapSpawnGroupChaseStepMaxStep)
	if !ok {
		return SpawnGroupPendingChaseStepSnapshot{}, false
	}
	remaining := dueAt.Sub(now).Milliseconds()
	if remaining < 0 {
		remaining = 0
	}
	return SpawnGroupPendingChaseStepSnapshot{
		EntityID:    entityID,
		ReadyAt:     dueAt,
		RemainingMs: remaining,
		Actor:       actor,
		Step: SpawnLeashReturnStepSnapshot{
			SpawnLeashSnapshot: worldruntime.SpawnLeashSnapshotFromEvaluation(plan.Evaluation),
			Next:               worldruntime.PositionSnapshotFromPosition(plan.Next),
			Complete:           plan.Complete,
		},
	}, true
}

func (r *gameRuntime) flushDueSpawnGroupChaseSteps() {
	if r == nil {
		return
	}
	for _, entityID := range r.dueSpawnGroupChaseStepIDs() {
		step, ok := r.stepSpawnGroupChase(entityID, bootstrapSpawnGroupChaseStepMaxStep, true)
		if !ok {
			r.clearSpawnGroupChaseStep(entityID)
			r.clearSpawnGroupHomewardStep(entityID)
			continue
		}
		if step.Step.Complete || !r.spawnGroupChaseStepStillEligible(entityID) {
			r.clearSpawnGroupChaseStep(entityID)
			r.clearSpawnGroupHomewardStep(entityID)
			continue
		}
	}
}

func (r *gameRuntime) scheduleSpawnGroupHomewardStep(entityID uint64) {
	if r == nil || entityID == 0 || bootstrapSpawnGroupHomewardStepDelay <= 0 {
		return
	}
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	r.spawnHomewardMu.Lock()
	defer r.spawnHomewardMu.Unlock()
	if r.spawnHomewardStepDueAt == nil {
		r.spawnHomewardStepDueAt = make(map[uint64]time.Time)
	}
	r.spawnHomewardStepDueAt[entityID] = now.Add(bootstrapSpawnGroupHomewardStepDelay)
}

func (r *gameRuntime) clearSpawnGroupHomewardStep(entityID uint64) {
	if r == nil || entityID == 0 {
		return
	}
	r.spawnHomewardMu.Lock()
	defer r.spawnHomewardMu.Unlock()
	delete(r.spawnHomewardStepDueAt, entityID)
}

func (r *gameRuntime) syncSpawnGroupHomewardStepScheduleForEntity(entityID uint64) {
	if r == nil || entityID == 0 || r.sharedWorld == nil {
		return
	}
	actor, ok := r.SpawnGroup(entityID)
	if !ok || actor.Dead || actor.SpawnGroupRef == "" || actor.SpawnLeash == nil {
		r.clearSpawnGroupHomewardStep(entityID)
		return
	}
	if actor.SpawnLeash.ReturnRequired || actor.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius {
		r.clearSpawnGroupHomewardStep(entityID)
		return
	}
	r.sharedWorld.mu.Lock()
	engagedBy := r.sharedWorld.staticActorCombatEngagedBy[entityID]
	r.sharedWorld.mu.Unlock()
	if engagedBy != 0 {
		r.clearSpawnGroupHomewardStep(entityID)
		return
	}
	if _, ok := r.sharedWorld.PlanSpawnGroupHomewardStep(entityID, bootstrapSpawnGroupHomewardStepMaxStep); !ok {
		r.clearSpawnGroupHomewardStep(entityID)
		return
	}
	r.scheduleSpawnGroupHomewardStep(entityID)
}

func (r *gameRuntime) dueSpawnGroupHomewardStepIDs() []uint64 {
	if r == nil {
		return nil
	}
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	r.spawnHomewardMu.Lock()
	defer r.spawnHomewardMu.Unlock()
	if len(r.spawnHomewardStepDueAt) == 0 {
		return nil
	}
	dueIDs := make([]uint64, 0, len(r.spawnHomewardStepDueAt))
	for entityID, dueAt := range r.spawnHomewardStepDueAt {
		if dueAt.IsZero() || now.Before(dueAt) {
			continue
		}
		dueIDs = append(dueIDs, entityID)
	}
	sort.Slice(dueIDs, func(i, j int) bool { return dueIDs[i] < dueIDs[j] })
	return dueIDs
}

func (r *gameRuntime) spawnGroupHomewardStepStillEligible(entityID uint64) bool {
	if r == nil || entityID == 0 || r.sharedWorld == nil {
		return false
	}
	actor, ok := r.SpawnGroup(entityID)
	if !ok || actor.Dead || actor.SpawnGroupRef == "" || actor.SpawnLeash == nil {
		return false
	}
	if actor.SpawnLeash.ReturnRequired || actor.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius {
		return false
	}
	r.sharedWorld.mu.Lock()
	engagedBy := r.sharedWorld.staticActorCombatEngagedBy[entityID]
	r.sharedWorld.mu.Unlock()
	if engagedBy != 0 {
		return false
	}
	_, ok = r.sharedWorld.PlanSpawnGroupHomewardStep(entityID, bootstrapSpawnGroupHomewardStepMaxStep)
	return ok
}

func (r *gameRuntime) SpawnGroupHomewardSteps() []SpawnGroupPendingHomewardStepSnapshot {
	if r == nil {
		return nil
	}
	dueAtByID := r.spawnGroupHomewardStepDueAtSnapshot()
	if len(dueAtByID) == 0 {
		return nil
	}
	entityIDs := make([]uint64, 0, len(dueAtByID))
	for entityID := range dueAtByID {
		entityIDs = append(entityIDs, entityID)
	}
	sort.Slice(entityIDs, func(i, j int) bool { return entityIDs[i] < entityIDs[j] })

	now := r.spawnGroupHomewardStepNow()
	snapshots := make([]SpawnGroupPendingHomewardStepSnapshot, 0, len(entityIDs))
	for _, entityID := range entityIDs {
		snapshot, ok := r.spawnGroupHomewardStepSnapshot(entityID, dueAtByID[entityID], now)
		if !ok {
			continue
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func (r *gameRuntime) SpawnGroupHomewardStep(entityID uint64) (SpawnGroupPendingHomewardStepSnapshot, bool) {
	if r == nil || entityID == 0 {
		return SpawnGroupPendingHomewardStepSnapshot{}, false
	}
	r.spawnHomewardMu.Lock()
	dueAt, ok := r.spawnHomewardStepDueAt[entityID]
	r.spawnHomewardMu.Unlock()
	if !ok {
		return SpawnGroupPendingHomewardStepSnapshot{}, false
	}
	return r.spawnGroupHomewardStepSnapshot(entityID, dueAt, r.spawnGroupHomewardStepNow())
}

func (r *gameRuntime) SpawnGroupHomewardStepsForMap(mapIndex uint32) ([]SpawnGroupPendingHomewardStepSnapshot, bool) {
	if r == nil || r.sharedWorld == nil || mapIndex == 0 {
		return nil, false
	}
	if _, ok := r.MapOccupancySnapshot(mapIndex); !ok {
		return nil, false
	}

	all := r.SpawnGroupHomewardSteps()
	if len(all) == 0 {
		return []SpawnGroupPendingHomewardStepSnapshot{}, true
	}
	filtered := make([]SpawnGroupPendingHomewardStepSnapshot, 0, len(all))
	for _, snapshot := range all {
		if snapshot.Actor.MapIndex != mapIndex {
			continue
		}
		filtered = append(filtered, snapshot)
	}
	return filtered, true
}

func (r *gameRuntime) spawnGroupHomewardStepDueAtSnapshot() map[uint64]time.Time {
	if r == nil {
		return nil
	}
	r.spawnHomewardMu.Lock()
	defer r.spawnHomewardMu.Unlock()
	if len(r.spawnHomewardStepDueAt) == 0 {
		return nil
	}
	snapshot := make(map[uint64]time.Time, len(r.spawnHomewardStepDueAt))
	for entityID, dueAt := range r.spawnHomewardStepDueAt {
		snapshot[entityID] = dueAt
	}
	return snapshot
}

func (r *gameRuntime) restoreSpawnGroupHomewardStepDueAtSnapshot(snapshot map[uint64]time.Time) {
	if r == nil {
		return
	}
	restored := make(map[uint64]time.Time, len(snapshot))
	for entityID, dueAt := range snapshot {
		if entityID == 0 || dueAt.IsZero() || !r.spawnGroupHomewardStepStillEligible(entityID) {
			continue
		}
		restored[entityID] = dueAt
	}
	r.spawnHomewardMu.Lock()
	defer r.spawnHomewardMu.Unlock()
	if r.spawnHomewardStepDueAt == nil {
		r.spawnHomewardStepDueAt = make(map[uint64]time.Time, len(restored))
	}
	for entityID := range r.spawnHomewardStepDueAt {
		delete(r.spawnHomewardStepDueAt, entityID)
	}
	for entityID, dueAt := range restored {
		r.spawnHomewardStepDueAt[entityID] = dueAt
	}
}

func (r *gameRuntime) spawnGroupHomewardStepNow() time.Time {
	now := time.Now()
	if r != nil && r.now != nil {
		now = r.now()
	}
	return now
}

func (r *gameRuntime) spawnGroupHomewardStepSnapshot(entityID uint64, dueAt time.Time, now time.Time) (SpawnGroupPendingHomewardStepSnapshot, bool) {
	if r == nil || r.sharedWorld == nil || entityID == 0 || dueAt.IsZero() {
		return SpawnGroupPendingHomewardStepSnapshot{}, false
	}
	if !r.spawnGroupHomewardStepStillEligible(entityID) {
		return SpawnGroupPendingHomewardStepSnapshot{}, false
	}
	actor, ok := r.SpawnGroup(entityID)
	if !ok {
		return SpawnGroupPendingHomewardStepSnapshot{}, false
	}
	plan, ok := r.sharedWorld.PlanSpawnGroupHomewardStep(entityID, bootstrapSpawnGroupHomewardStepMaxStep)
	if !ok {
		return SpawnGroupPendingHomewardStepSnapshot{}, false
	}
	remaining := dueAt.Sub(now).Milliseconds()
	if remaining < 0 {
		remaining = 0
	}
	return SpawnGroupPendingHomewardStepSnapshot{
		EntityID:    entityID,
		ReadyAt:     dueAt,
		RemainingMs: remaining,
		Actor:       actor,
		Step: SpawnLeashReturnStepSnapshot{
			SpawnLeashSnapshot: worldruntime.SpawnLeashSnapshotFromEvaluation(plan.Evaluation),
			Next:               worldruntime.PositionSnapshotFromPosition(plan.Next),
			Complete:           plan.Complete,
		},
	}, true
}

func (r *gameRuntime) pruneSpawnGroupHomewardStepSchedules() {
	r.restoreSpawnGroupHomewardStepDueAtSnapshot(r.spawnGroupHomewardStepDueAtSnapshot())
}

func (r *gameRuntime) flushDueSpawnGroupHomewardSteps() {
	if r == nil {
		return
	}
	for _, entityID := range r.dueSpawnGroupHomewardStepIDs() {
		step, ok := r.stepSpawnGroupHomeward(entityID, bootstrapSpawnGroupHomewardStepMaxStep, true)
		if !ok {
			if r.spawnGroupHomewardStepStillEligible(entityID) {
				r.scheduleSpawnGroupHomewardStep(entityID)
				continue
			}
			r.clearSpawnGroupHomewardStep(entityID)
			continue
		}
		if step.Step.Complete || !r.spawnGroupHomewardStepStillEligible(entityID) {
			r.clearSpawnGroupHomewardStep(entityID)
			continue
		}
	}
}

func (r *gameRuntime) stepSpawnGroupHomeward(entityID uint64, maxStep int32, reschedule bool) (SpawnGroupReturnStepSnapshot, bool) {
	if r == nil || r.sharedWorld == nil || entityID == 0 || maxStep <= 0 {
		return SpawnGroupReturnStepSnapshot{}, false
	}

	r.staticActorMu.Lock()
	defer r.staticActorMu.Unlock()

	current := r.sharedWorld.StaticActors()
	idx := staticActorSnapshotIndex(current, entityID)
	if idx == -1 || current[idx].SpawnGroupRef == "" || current[idx].Dead {
		r.clearSpawnGroupHomewardStep(entityID)
		return SpawnGroupReturnStepSnapshot{}, false
	}
	if current[idx].SpawnLeash == nil || current[idx].SpawnLeash.ReturnRequired || current[idx].SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius {
		r.clearSpawnGroupHomewardStep(entityID)
		return SpawnGroupReturnStepSnapshot{}, false
	}

	r.sharedWorld.mu.Lock()
	engagedBy := r.sharedWorld.staticActorCombatEngagedBy[entityID]
	r.sharedWorld.mu.Unlock()
	if engagedBy != 0 {
		r.clearSpawnGroupHomewardStep(entityID)
		return SpawnGroupReturnStepSnapshot{}, false
	}

	plan, ok := r.sharedWorld.PlanSpawnGroupHomewardStep(entityID, maxStep)
	if !ok {
		r.clearSpawnGroupHomewardStep(entityID)
		return SpawnGroupReturnStepSnapshot{}, false
	}
	if plan.Complete && plan.Next.Equal(worldruntime.NewPosition(current[idx].MapIndex, current[idx].X, current[idx].Y)) {
		r.clearSpawnGroupHomewardStep(entityID)
		return SpawnGroupReturnStepSnapshot{
			Actor: current[idx],
			Step: SpawnLeashReturnStepSnapshot{
				SpawnLeashSnapshot: worldruntime.SpawnLeashSnapshotFromEvaluation(plan.Evaluation),
				Next:               worldruntime.PositionSnapshotFromPosition(plan.Next),
				Complete:           true,
			},
		}, true
	}

	target := cloneStaticActorSnapshots(current)
	target[idx].MapIndex = plan.Next.MapIndex
	target[idx].X = plan.Next.X
	target[idx].Y = plan.Next.Y
	if !r.persistStaticActorSnapshot(target) {
		return SpawnGroupReturnStepSnapshot{}, false
	}
	stepped, ok := r.sharedWorld.StepSpawnGroupHomeward(entityID, maxStep)
	if !ok {
		_ = r.persistStaticActorSnapshot(current)
		return SpawnGroupReturnStepSnapshot{}, false
	}
	if stepped.Step.Complete || stepped.Actor.SpawnLeash == nil || stepped.Actor.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius || !r.spawnGroupHomewardStepStillEligible(entityID) {
		r.clearSpawnGroupHomewardStep(entityID)
	} else if reschedule {
		r.scheduleSpawnGroupHomewardStep(entityID)
	}
	return stepped, true
}

func (r *gameRuntime) flushProximitySpawnGroupAggroAcquisition() {
	if r == nil || r.sharedWorld == nil {
		return
	}
	for _, entityID := range r.sharedWorld.AcquireProximitySpawnGroupAggro() {
		r.syncSpawnGroupChaseStepScheduleForEntity(entityID)
	}
}

func (r *gameRuntime) stepSpawnGroupChase(entityID uint64, maxStep int32, reschedule bool) (SpawnGroupReturnStepSnapshot, bool) {
	if r == nil || r.sharedWorld == nil || entityID == 0 || maxStep <= 0 {
		return SpawnGroupReturnStepSnapshot{}, false
	}

	r.staticActorMu.Lock()
	defer r.staticActorMu.Unlock()

	current := r.sharedWorld.StaticActors()
	idx := staticActorSnapshotIndex(current, entityID)
	if idx == -1 || current[idx].SpawnGroupRef == "" || current[idx].Dead {
		r.clearSpawnGroupChaseStep(entityID)
		r.clearSpawnGroupHomewardStep(entityID)
		return SpawnGroupReturnStepSnapshot{}, false
	}
	if current[idx].SpawnLeash != nil && current[idx].SpawnLeash.ReturnRequired {
		r.clearSpawnGroupChaseStep(entityID)
		r.clearSpawnGroupHomewardStep(entityID)
		return SpawnGroupReturnStepSnapshot{}, false
	}

	r.sharedWorld.mu.Lock()
	engagedBy, engaged := r.sharedWorld.staticActorCombatEngagedBy[entityID]
	r.sharedWorld.mu.Unlock()
	if !engaged || engagedBy == 0 {
		r.clearSpawnGroupChaseStep(entityID)
		r.clearSpawnGroupHomewardStep(entityID)
		return SpawnGroupReturnStepSnapshot{}, false
	}
	owner, ok := r.sharedWorld.playerCharacter(engagedBy)
	if !ok || characterAtBootstrapHPFloor(owner) {
		r.clearSpawnGroupChaseStep(entityID)
		r.clearSpawnGroupHomewardStep(entityID)
		return SpawnGroupReturnStepSnapshot{}, false
	}
	ownerPos := worldruntime.NewPosition(owner.MapIndex, owner.X, owner.Y)
	plan, ok := r.sharedWorld.PlanSpawnGroupChaseStep(entityID, ownerPos, maxStep)
	if !ok {
		r.clearSpawnGroupChaseStep(entityID)
		r.clearSpawnGroupHomewardStep(entityID)
		return SpawnGroupReturnStepSnapshot{}, false
	}
	if plan.Complete && plan.Next.Equal(worldruntime.NewPosition(current[idx].MapIndex, current[idx].X, current[idx].Y)) {
		r.clearSpawnGroupChaseStep(entityID)
		r.clearSpawnGroupHomewardStep(entityID)
		return SpawnGroupReturnStepSnapshot{
			Actor: current[idx],
			Step: SpawnLeashReturnStepSnapshot{
				SpawnLeashSnapshot: worldruntime.SpawnLeashSnapshotFromEvaluation(plan.Evaluation),
				Next:               worldruntime.PositionSnapshotFromPosition(plan.Next),
				Complete:           true,
			},
		}, true
	}

	target := cloneStaticActorSnapshots(current)
	target[idx].MapIndex = plan.Next.MapIndex
	target[idx].X = plan.Next.X
	target[idx].Y = plan.Next.Y
	if !r.persistStaticActorSnapshot(target) {
		return SpawnGroupReturnStepSnapshot{}, false
	}
	stepped, ok := r.sharedWorld.StepSpawnGroupChase(entityID, ownerPos, maxStep)
	if !ok {
		_ = r.persistStaticActorSnapshot(current)
		return SpawnGroupReturnStepSnapshot{}, false
	}
	if stepped.Step.Complete || stepped.Actor.SpawnLeash == nil || stepped.Actor.SpawnLeash.ReturnRequired || !r.spawnGroupChaseStepStillEligible(entityID) {
		r.clearSpawnGroupChaseStep(entityID)
		r.clearSpawnGroupHomewardStep(entityID)
	} else if reschedule {
		r.scheduleSpawnGroupChaseStep(entityID)
	}
	return stepped, true
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

func (r *gameRuntime) ConnectedCharacterSnapshot(name string) (ConnectedCharacterSnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return ConnectedCharacterSnapshot{}, false
	}
	return r.sharedWorld.ConnectedCharacterSnapshot(name)
}

func (r *gameRuntime) CharacterVisibility() []CharacterVisibilitySnapshot {
	if r == nil || r.sharedWorld == nil {
		return nil
	}
	return r.sharedWorld.CharacterVisibility()
}

func (r *gameRuntime) CharacterVisibilitySnapshot(name string) (CharacterVisibilitySnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return CharacterVisibilitySnapshot{}, false
	}
	return r.sharedWorld.CharacterVisibilitySnapshot(name)
}

func (r *gameRuntime) CombatTargetSnapshots() []CombatTargetSnapshot {
	if r == nil || r.sharedWorld == nil {
		return nil
	}
	return r.sharedWorld.CombatTargetSnapshots()
}

func (r *gameRuntime) CombatTargetSnapshotsForMap(mapIndex uint32) ([]CombatTargetSnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return nil, false
	}
	return r.sharedWorld.CombatTargetSnapshotsForMap(mapIndex)
}

func (r *gameRuntime) StaticActorRespawns() []StaticActorRespawnSnapshot {
	if r == nil || r.sharedWorld == nil {
		return nil
	}
	return r.sharedWorld.StaticActorRespawns()
}

func (r *gameRuntime) StaticActorRespawn(entityID uint64) (StaticActorRespawnSnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return StaticActorRespawnSnapshot{}, false
	}
	return r.sharedWorld.StaticActorRespawn(entityID)
}

func (r *gameRuntime) StaticActorRespawnsForMap(mapIndex uint32) ([]StaticActorRespawnSnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return nil, false
	}
	return r.sharedWorld.StaticActorRespawnsForMap(mapIndex)
}

func (r *gameRuntime) InteractionVisibility() []CharacterInteractionVisibilitySnapshot {
	if r == nil || r.sharedWorld == nil {
		return nil
	}
	base := r.sharedWorld.InteractionVisibility()
	out := make([]CharacterInteractionVisibilitySnapshot, 0, len(base))
	for _, entry := range base {
		out = append(out, r.resolveInteractionVisibilitySnapshot(entry))
	}
	return out
}

func (r *gameRuntime) InteractionVisibilitySnapshot(name string) (CharacterInteractionVisibilitySnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return CharacterInteractionVisibilitySnapshot{}, false
	}
	entry, ok := r.sharedWorld.InteractionVisibilitySnapshot(name)
	if !ok {
		return CharacterInteractionVisibilitySnapshot{}, false
	}
	return r.resolveInteractionVisibilitySnapshot(entry), true
}

func (r *gameRuntime) resolveInteractionVisibilitySnapshot(entry worldruntime.CharacterInteractionVisibilitySnapshot) CharacterInteractionVisibilitySnapshot {
	resolved := make([]InteractableStaticActorVisibilitySnapshot, 0, len(entry.VisibleInteractableStaticActors))
	for _, actor := range entry.VisibleInteractableStaticActors {
		resolvedActor := InteractableStaticActorVisibilitySnapshot{StaticActorSnapshot: actor}
		definition, ok := r.ResolveInteractionDefinition(actor.InteractionKind, actor.InteractionRef)
		if !ok {
			resolvedActor.ResolutionFailure = staticActorInteractionFailureDefinitionNotFound
			resolved = append(resolved, resolvedActor)
			continue
		}
		preview, ok := r.interactionDefinitionVisibilityPreview(entry.Name, actor.Name, definition)
		if !ok {
			resolvedActor.ResolutionFailure = staticActorInteractionFailureUnsupportedKind
			resolved = append(resolved, resolvedActor)
			continue
		}
		resolvedActor.Preview = compactInteractionPreview(preview)
		resolved = append(resolved, resolvedActor)
	}
	return CharacterInteractionVisibilitySnapshot{ConnectedCharacterSnapshot: entry.ConnectedCharacterSnapshot, VisibleInteractableStaticActors: resolved}
}

func (r *gameRuntime) MapOccupancy() []MapOccupancySnapshot {
	if r == nil || r.sharedWorld == nil {
		return nil
	}
	return r.sharedWorld.MapOccupancy()
}

func (r *gameRuntime) MapOccupancySnapshot(mapIndex uint32) (MapOccupancySnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return MapOccupancySnapshot{}, false
	}
	return r.sharedWorld.MapOccupancySnapshot(mapIndex)
}

func (r *gameRuntime) GroundItems() []GroundItemSnapshot {
	if r == nil || r.sharedWorld == nil {
		return nil
	}
	return r.sharedWorld.GroundItems()
}

func (r *gameRuntime) GroundItem(vid uint32) (GroundItemSnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return GroundItemSnapshot{}, false
	}
	return r.sharedWorld.GroundItem(vid)
}

func (r *gameRuntime) GroundItemsForMap(mapIndex uint32) ([]GroundItemSnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return nil, false
	}
	return r.sharedWorld.GroundItemsForMap(mapIndex)
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
	snapshot.Persistence = runtimePersistenceConfigSnapshot(r)
	snapshot.Database = runtimeDatabaseConfigSnapshot(r)
	return snapshot
}

func runtimeDatabaseConfigSnapshot(r *gameRuntime) DatabaseConfigSnapshot {
	if r == nil {
		return DatabaseConfigSnapshot{}
	}
	driver := strings.TrimSpace(r.config.DatabaseDriver)
	dsn := strings.TrimSpace(r.config.DatabaseDSN)
	return DatabaseConfigSnapshot{
		Configured:    driver != "" && dsn != "",
		Driver:        driver,
		DSNConfigured: dsn != "",
	}
}

func runtimePersistenceConfigSnapshot(r *gameRuntime) PersistenceConfigSnapshot {
	if r == nil {
		return PersistenceConfigSnapshot{}
	}
	return PersistenceConfigSnapshot{
		LoginTicketStoreDir:   loginTicketStoreDir(r.loginTicketStore),
		AccountStoreDir:       accountStoreDir(r.accountStore),
		StaticActorStorePath:  staticActorStorePath(r.staticStore),
		InteractionStorePath:  interactionStorePath(r.interactionStore),
		ItemTemplateStorePath: itemTemplateStorePath(r.itemStore),
		QuestStateStorePath:   questStateStorePath(r.questStateStore),
		GroundItemStorePath:   groundItemStorePath(r.groundItemStore),
		SafeboxStorePath:      safeboxStorePath(r.safeboxStore),
	}
}

func loginTicketStoreDir(store loginticket.Store) string {
	if store == nil {
		return ""
	}
	if locator, ok := store.(interface{ Dir() string }); ok {
		return locator.Dir()
	}
	return ""
}

func accountStoreDir(store accountstore.Store) string {
	if store == nil {
		return ""
	}
	if locator, ok := store.(interface{ Dir() string }); ok {
		return locator.Dir()
	}
	return ""
}

func staticActorStorePath(store staticstore.Store) string {
	if store == nil {
		return ""
	}
	if locator, ok := store.(interface{ Path() string }); ok {
		return locator.Path()
	}
	return ""
}

func interactionStorePath(store interactionstore.Store) string {
	if store == nil {
		return ""
	}
	if locator, ok := store.(interface{ Path() string }); ok {
		return locator.Path()
	}
	return ""
}

func itemTemplateStorePath(store itemcatalog.Store) string {
	if store == nil {
		return ""
	}
	if locator, ok := store.(interface{ Path() string }); ok {
		return locator.Path()
	}
	return ""
}

func questStateStorePath(store queststate.Store) string {
	if store == nil {
		return ""
	}
	if locator, ok := store.(interface{ Path() string }); ok {
		return locator.Path()
	}
	return ""
}

func groundItemStorePath(store worldruntime.GroundItemStore) string {
	if store == nil {
		return ""
	}
	if locator, ok := store.(interface{ Path() string }); ok {
		return locator.Path()
	}
	return ""
}

func safeboxStorePath(store safeboxstore.Store) string {
	if store == nil {
		return ""
	}
	if locator, ok := store.(interface{ Path() string }); ok {
		return locator.Path()
	}
	return ""
}

func accountBackupManifestStatus(accountStoreDir string) BackupManifestStatus {
	if strings.TrimSpace(accountStoreDir) == "" {
		return BackupManifestStatus{}
	}
	path := filepath.Join(accountStoreDir, accountstore.BackupManifestFilename)
	raw, status, ok := readBackupManifestStatusRaw(path)
	if !ok {
		return status
	}
	var manifest accountstore.BackupManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return status
	}
	status.Format = manifest.Format
	status.FileCount = len(manifest.Files)
	for _, file := range manifest.Files {
		status.SnapshotSizeBytes += file.SizeBytes
	}
	return status
}

func loginTicketBackupManifestStatus(loginTicketStoreDir string) BackupManifestStatus {
	if strings.TrimSpace(loginTicketStoreDir) == "" {
		return BackupManifestStatus{}
	}
	path := filepath.Join(loginTicketStoreDir, loginticket.BackupManifestFilename)
	raw, status, ok := readBackupManifestStatusRaw(path)
	if !ok {
		return status
	}
	var manifest loginticket.BackupManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return status
	}
	status.Format = manifest.Format
	status.FileCount = len(manifest.Files)
	for _, file := range manifest.Files {
		status.SnapshotSizeBytes += file.SizeBytes
	}
	return status
}

func itemTemplateBackupManifestStatus(itemTemplatePath string) BackupManifestStatus {
	if strings.TrimSpace(itemTemplatePath) == "" {
		return BackupManifestStatus{}
	}
	path := filepath.Join(filepath.Dir(itemTemplatePath), itemcatalog.BackupManifestFilename)
	raw, status, ok := readBackupManifestStatusRaw(path)
	if !ok {
		return status
	}
	var manifest itemcatalog.BackupManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return status
	}
	status.Format = manifest.Format
	status.FileCount = len(manifest.Files)
	for _, file := range manifest.Files {
		status.SnapshotSizeBytes += file.SizeBytes
	}
	return status
}

func questStateBackupManifestStatus(questStatePath string) BackupManifestStatus {
	if strings.TrimSpace(questStatePath) == "" {
		return BackupManifestStatus{}
	}
	path := filepath.Join(filepath.Dir(questStatePath), queststate.BackupManifestFilename)
	raw, status, ok := readBackupManifestStatusRaw(path)
	if !ok {
		return status
	}
	var manifest queststate.BackupManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return status
	}
	status.Format = manifest.Format
	status.FileCount = len(manifest.Files)
	for _, file := range manifest.Files {
		status.SnapshotSizeBytes += file.SizeBytes
	}
	return status
}

func groundItemBackupManifestStatus(groundItemPath string) BackupManifestStatus {
	if strings.TrimSpace(groundItemPath) == "" {
		return BackupManifestStatus{}
	}
	path := filepath.Join(filepath.Dir(groundItemPath), worldruntime.BackupManifestFilename)
	raw, status, ok := readBackupManifestStatusRaw(path)
	if !ok {
		return status
	}
	var manifest worldruntime.BackupManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return status
	}
	status.Format = manifest.Format
	status.FileCount = len(manifest.Files)
	for _, file := range manifest.Files {
		status.SnapshotSizeBytes += file.SizeBytes
	}
	return status
}

func safeboxBackupManifestStatus(safeboxPath string) BackupManifestStatus {
	if strings.TrimSpace(safeboxPath) == "" {
		return BackupManifestStatus{}
	}
	path := filepath.Join(filepath.Dir(safeboxPath), safeboxstore.BackupManifestFilename)
	raw, status, ok := readBackupManifestStatusRaw(path)
	if !ok {
		return status
	}
	var manifest safeboxstore.BackupManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return status
	}
	status.Format = manifest.Format
	status.FileCount = len(manifest.Files)
	for _, file := range manifest.Files {
		status.SnapshotSizeBytes += file.SizeBytes
	}
	return status
}

func staticActorBackupManifestStatus(staticActorPath string) BackupManifestStatus {
	if strings.TrimSpace(staticActorPath) == "" {
		return BackupManifestStatus{}
	}
	path := filepath.Join(filepath.Dir(staticActorPath), staticstore.BackupManifestFilename)
	raw, status, ok := readBackupManifestStatusRaw(path)
	if !ok {
		return status
	}
	var manifest staticstore.BackupManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return status
	}
	status.Format = manifest.Format
	status.FileCount = len(manifest.Files)
	for _, file := range manifest.Files {
		status.SnapshotSizeBytes += file.SizeBytes
	}
	return status
}

func interactionBackupManifestStatus(interactionPath string) BackupManifestStatus {
	if strings.TrimSpace(interactionPath) == "" {
		return BackupManifestStatus{}
	}
	path := filepath.Join(filepath.Dir(interactionPath), interactionstore.BackupManifestFilename)
	raw, status, ok := readBackupManifestStatusRaw(path)
	if !ok {
		return status
	}
	var manifest interactionstore.BackupManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return status
	}
	status.Format = manifest.Format
	status.FileCount = len(manifest.Files)
	for _, file := range manifest.Files {
		status.SnapshotSizeBytes += file.SizeBytes
	}
	return status
}

func readBackupManifestStatusRaw(path string) ([]byte, BackupManifestStatus, bool) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, BackupManifestStatus{}, false
	}
	status := BackupManifestStatus{Present: true, Path: path}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, status, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, status, false
	}
	return raw, backupManifestStatusFromRaw(path, raw), true
}

func backupManifestStatusFromRaw(path string, raw []byte) BackupManifestStatus {
	checksum := sha256.Sum256(raw)
	return BackupManifestStatus{
		Present:           true,
		Path:              path,
		ManifestSizeBytes: int64(len(raw)),
		ManifestSHA256:    hex.EncodeToString(checksum[:]),
	}
}

func (r *gameRuntime) CombatTargetSnapshot(name string) (CombatTargetSnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return CombatTargetSnapshot{}, false
	}
	return r.sharedWorld.CombatTargetSnapshotByName(name)
}

func (r *gameRuntime) InventorySnapshot(name string) (CharacterInventorySnapshot, bool) {
	state, ok := r.liveCharacterState(name)
	if !ok {
		return CharacterInventorySnapshot{}, false
	}
	return CharacterInventorySnapshot{
		Name:      state.Name,
		Inventory: append([]InventoryItemSnapshot(nil), state.Inventory...),
	}, true
}

func (r *gameRuntime) EquipmentSnapshot(name string) (CharacterEquipmentSnapshot, bool) {
	state, ok := r.liveCharacterState(name)
	if !ok {
		return CharacterEquipmentSnapshot{}, false
	}
	return CharacterEquipmentSnapshot{
		Name:      state.Name,
		Equipment: append([]EquipmentItemSnapshot(nil), state.Equipment...),
	}, true
}

func (r *gameRuntime) QuickslotsSnapshot(name string) (CharacterQuickslotsSnapshot, bool) {
	state, ok := r.liveCharacterState(name)
	if !ok {
		return CharacterQuickslotsSnapshot{}, false
	}
	return CharacterQuickslotsSnapshot{
		Name:       state.Name,
		Quickslots: append([]QuickslotSnapshot(nil), state.Quickslots...),
	}, true
}

func (r *gameRuntime) CurrencySnapshot(name string) (CharacterCurrencySnapshot, bool) {
	state, ok := r.liveCharacterState(name)
	if !ok {
		return CharacterCurrencySnapshot{}, false
	}
	return CharacterCurrencySnapshot{Name: state.Name, Gold: state.Gold}, true
}

func (r *gameRuntime) PointsSnapshot(name string) (CharacterPointsSnapshot, bool) {
	state, ok := r.liveCharacterState(name)
	if !ok {
		return CharacterPointsSnapshot{}, false
	}
	return CharacterPointsSnapshot{Name: state.Name, Points: state.Points}, true
}

func (r *gameRuntime) registerLiveCharacterSnapshotter(name string, login string, snapshotter liveCharacterStateSnapshotter, applyPersistedSnapshot liveCharacterPersistedSnapshotApplier) uint64 {
	if r == nil || snapshotter == nil {
		return 0
	}
	name = normalizeLiveCharacterName(name)
	if name == "" {
		return 0
	}
	r.liveCharacterMu.Lock()
	defer r.liveCharacterMu.Unlock()
	if r.liveCharactersByName == nil {
		r.liveCharactersByName = make(map[string]liveCharacterRegistration)
	}
	r.liveCharacterNextID++
	registrationID := r.liveCharacterNextID
	r.liveCharactersByName[name] = liveCharacterRegistration{
		id:                     registrationID,
		login:                  strings.TrimSpace(login),
		snapshotter:            snapshotter,
		applyPersistedSnapshot: applyPersistedSnapshot,
	}
	return registrationID
}

func (r *gameRuntime) unregisterLiveCharacterSnapshotter(name string, registrationID uint64) {
	if r == nil || registrationID == 0 {
		return
	}
	name = normalizeLiveCharacterName(name)
	if name == "" {
		return
	}
	r.liveCharacterMu.Lock()
	defer r.liveCharacterMu.Unlock()
	registration, ok := r.liveCharactersByName[name]
	if !ok || registration.id != registrationID {
		return
	}
	delete(r.liveCharactersByName, name)
	if len(r.liveCharactersByName) == 0 {
		r.liveCharactersByName = nil
	}
}

func (r *gameRuntime) liveSelectedCharacterCount() int {
	if r == nil {
		return 0
	}
	r.liveCharacterMu.RLock()
	defer r.liveCharacterMu.RUnlock()
	return len(r.liveCharactersByName)
}

func (r *gameRuntime) liveCharacterState(name string) (liveCharacterStateSnapshot, bool) {
	if r == nil {
		return liveCharacterStateSnapshot{}, false
	}
	name = normalizeLiveCharacterName(name)
	if name == "" {
		return liveCharacterStateSnapshot{}, false
	}
	r.liveCharacterMu.RLock()
	registration, ok := r.liveCharactersByName[name]
	r.liveCharacterMu.RUnlock()
	if !ok || registration.snapshotter == nil {
		return liveCharacterStateSnapshot{}, false
	}
	return registration.snapshotter()
}

func (r *gameRuntime) applyLiveCharacterPersistedSnapshot(name string, updated loginticket.Character) bool {
	if r == nil {
		return false
	}
	name = normalizeLiveCharacterName(name)
	if name == "" {
		return false
	}
	r.liveCharacterMu.RLock()
	registration, ok := r.liveCharactersByName[name]
	r.liveCharacterMu.RUnlock()
	if !ok || registration.applyPersistedSnapshot == nil {
		return false
	}
	return registration.applyPersistedSnapshot(updated)
}

func (r *gameRuntime) liveCharacterLogin(name string) (string, bool) {
	if r == nil {
		return "", false
	}
	name = normalizeLiveCharacterName(name)
	if name == "" {
		return "", false
	}
	r.liveCharacterMu.RLock()
	registration, ok := r.liveCharactersByName[name]
	r.liveCharacterMu.RUnlock()
	if !ok || strings.TrimSpace(registration.login) == "" {
		return "", false
	}
	return registration.login, true
}

func normalizeLiveCharacterName(name string) string {
	return strings.TrimSpace(name)
}

func buildLiveCharacterStateSnapshot(character loginticket.Character) liveCharacterStateSnapshot {
	state := liveCharacterStateSnapshot{
		Name:       character.Name,
		Level:      character.Level,
		Job:        character.Job,
		RaceNum:    character.RaceNum,
		Empire:     character.Empire,
		Gold:       character.Gold,
		Points:     character.Points,
		Inventory:  make([]InventoryItemSnapshot, 0, len(character.Inventory)),
		Equipment:  make([]EquipmentItemSnapshot, 0, len(character.Equipment)),
		Quickslots: make([]QuickslotSnapshot, 0, len(character.Quickslots)),
	}
	for _, item := range character.Inventory {
		state.Inventory = append(state.Inventory, InventoryItemSnapshot{
			ID:     item.ID,
			Vnum:   item.Vnum,
			Count:  item.Count,
			Slot:   uint16(item.Slot),
			Locked: item.Locked,
		})
	}
	for _, item := range character.Equipment {
		state.Equipment = append(state.Equipment, EquipmentItemSnapshot{
			ID:        item.ID,
			Vnum:      item.Vnum,
			Count:     item.Count,
			EquipSlot: item.EquipSlot.String(),
			Locked:    item.Locked,
		})
	}
	for _, quickslot := range character.Quickslots {
		state.Quickslots = append(state.Quickslots, QuickslotSnapshot{
			Position: quickslot.Position,
			Type:     quickslot.Type,
			Slot:     quickslot.Slot,
		})
	}
	return state
}

func (r *gameRuntime) RegisterStaticActor(name string, mapIndex uint32, x int32, y int32, raceNum uint32) (StaticActorSnapshot, bool) {
	return r.RegisterStaticActorWithInteractionAndCombatProfile(name, mapIndex, x, y, raceNum, "", "", "")
}

func (r *gameRuntime) RegisterStaticActorWithInteraction(name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string) (StaticActorSnapshot, bool) {
	return r.RegisterStaticActorWithInteractionAndCombatProfile(name, mapIndex, x, y, raceNum, interactionKind, interactionRef, "")
}

func (r *gameRuntime) RegisterStaticActorWithInteractionAndCombatProfile(name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string, combatProfile string) (StaticActorSnapshot, bool) {
	return r.registerStaticActorWithInteractionCombatProfileAndSpawnGroupRef(name, mapIndex, x, y, raceNum, interactionKind, interactionRef, combatProfile, "")
}

func (r *gameRuntime) registerStaticActorWithInteractionCombatProfileAndSpawnGroupRef(name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string, combatProfile string, spawnGroupRef string) (StaticActorSnapshot, bool) {
	return r.registerStaticActorWithInteractionCombatProfileSpawnGroupRefAndReward(name, mapIndex, x, y, raceNum, interactionKind, interactionRef, combatProfile, spawnGroupRef, worldruntime.StaticActorDeathReward{})
}

func (r *gameRuntime) registerStaticActorWithInteractionCombatProfileSpawnGroupRefAndReward(name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string, combatProfile string, spawnGroupRef string, deathReward worldruntime.StaticActorDeathReward) (StaticActorSnapshot, bool) {
	return r.registerStaticActorWithInteractionCombatProfileSpawnGroupRefHomeAndReward(name, mapIndex, x, y, raceNum, interactionKind, interactionRef, combatProfile, spawnGroupRef, nil, deathReward)
}

func (r *gameRuntime) registerStaticActorWithInteractionCombatProfileSpawnGroupRefHomeAndReward(name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string, combatProfile string, spawnGroupRef string, spawnHome *worldruntime.PositionSnapshot, deathReward worldruntime.StaticActorDeathReward) (StaticActorSnapshot, bool) {
	return r.registerStaticActorWithInteractionCombatProfileSpawnGroupRefHomeRewardAndKillQuestCredit(name, mapIndex, x, y, raceNum, interactionKind, interactionRef, combatProfile, spawnGroupRef, spawnHome, deathReward, staticActorKillQuestCredit{})
}

func (r *gameRuntime) registerStaticActorWithInteractionCombatProfileSpawnGroupRefHomeRewardAndKillQuestCredit(name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string, combatProfile string, spawnGroupRef string, spawnHome *worldruntime.PositionSnapshot, deathReward worldruntime.StaticActorDeathReward, killQuestCredit staticActorKillQuestCredit) (StaticActorSnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return StaticActorSnapshot{}, false
	}
	name = strings.TrimSpace(name)
	interactionKind = strings.TrimSpace(interactionKind)
	interactionRef = strings.TrimSpace(interactionRef)
	combatProfile = strings.TrimSpace(combatProfile)
	spawnGroupRef = strings.TrimSpace(spawnGroupRef)
	killQuestCredit = killQuestCredit.Clone()
	if !validStaticActorRuntimeName(name) || mapIndex == 0 || raceNum == 0 || !worldruntime.ValidStaticActorInteractionMetadata(interactionKind, interactionRef) || !r.interactionDefinitionExists(interactionKind, interactionRef) || !worldruntime.ValidStaticActorCombatProfile(combatProfile) || !worldruntime.ValidStaticActorSpawnGroupRef(spawnGroupRef) || !validStaticActorKillQuestCredit(killQuestCredit) {
		return StaticActorSnapshot{}, false
	}
	if spawnGroupRef != "" && (combatProfile == "" || interactionKind != "" || interactionRef != "") {
		return StaticActorSnapshot{}, false
	}
	if spawnGroupRef == "" && !killQuestCredit.Empty() {
		return StaticActorSnapshot{}, false
	}

	r.staticActorMu.Lock()
	defer r.staticActorMu.Unlock()

	current := r.sharedWorld.StaticActors()
	nextEntityID := r.sharedWorld.NextStaticActorEntityID()
	if nextEntityID == 0 {
		return StaticActorSnapshot{}, false
	}
	staticActorSnapshot := StaticActorSnapshot{EntityID: nextEntityID, Name: name, MapIndex: mapIndex, X: x, Y: y, RaceNum: raceNum, CombatProfile: combatProfile, InteractionKind: interactionKind, InteractionRef: interactionRef, SpawnGroupRef: spawnGroupRef, RewardExperience: deathReward.Experience, RewardGold: deathReward.Gold, RewardDropVnums: append([]uint32(nil), deathReward.DropVnums...), RewardQuestRef: killQuestCredit.QuestRef, RewardQuestFlag: killQuestCredit.QuestFlag, RewardQuestFrom: killQuestCredit.QuestFrom, RewardQuestTo: killQuestCredit.QuestTo, RewardQuestText: killQuestCredit.Text, RequireQuestRef: killQuestCredit.RequireQuestRef, RequireQuestFlag: killQuestCredit.RequireQuestFlag, RequireQuestFrom: killQuestCredit.RequireQuestFrom}
	if spawnHome != nil {
		clonedSpawnHome := *spawnHome
		staticActorSnapshot.SpawnHome = &clonedSpawnHome
	}
	target := appendStaticActorSnapshot(current, staticActorSnapshot)
	if !r.persistStaticActorSnapshot(target) {
		return StaticActorSnapshot{}, false
	}
	registered, ok := r.sharedWorld.registerStaticActorWithSpawnHomeAndKillQuestCredit(nextEntityID, name, mapIndex, x, y, raceNum, interactionKind, interactionRef, combatProfile, spawnGroupRef, spawnHome, deathReward, killQuestCredit)
	if !ok {
		_ = r.persistStaticActorSnapshot(current)
		return StaticActorSnapshot{}, false
	}
	r.syncSpawnGroupReturnStepSchedule(registered)
	return registered, true
}

func (r *gameRuntime) UpdateStaticActor(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32) (StaticActorSnapshot, bool) {
	return r.UpdateStaticActorWithInteractionAndCombatProfile(entityID, name, mapIndex, x, y, raceNum, "", "", "")
}

func (r *gameRuntime) UpdateStaticActorWithInteraction(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string) (StaticActorSnapshot, bool) {
	return r.UpdateStaticActorWithInteractionAndCombatProfile(entityID, name, mapIndex, x, y, raceNum, interactionKind, interactionRef, "")
}

func (r *gameRuntime) UpdateStaticActorWithInteractionAndCombatProfile(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string, combatProfile string) (StaticActorSnapshot, bool) {
	return r.updateStaticActorWithInteractionCombatProfileAndSpawnGroupRef(entityID, name, mapIndex, x, y, raceNum, interactionKind, interactionRef, combatProfile, "")
}

func (r *gameRuntime) updateStaticActorWithInteractionCombatProfileAndSpawnGroupRef(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string, combatProfile string, spawnGroupRef string) (StaticActorSnapshot, bool) {
	if r == nil || r.sharedWorld == nil || entityID == 0 {
		return StaticActorSnapshot{}, false
	}
	name = strings.TrimSpace(name)
	interactionKind = strings.TrimSpace(interactionKind)
	interactionRef = strings.TrimSpace(interactionRef)
	combatProfile = strings.TrimSpace(combatProfile)
	spawnGroupRef = strings.TrimSpace(spawnGroupRef)
	if !validStaticActorRuntimeName(name) || mapIndex == 0 || raceNum == 0 || !worldruntime.ValidStaticActorInteractionMetadata(interactionKind, interactionRef) || !r.interactionDefinitionExists(interactionKind, interactionRef) || !worldruntime.ValidStaticActorCombatProfile(combatProfile) || !worldruntime.ValidStaticActorSpawnGroupRef(spawnGroupRef) {
		return StaticActorSnapshot{}, false
	}
	if spawnGroupRef != "" && (combatProfile == "" || interactionKind != "" || interactionRef != "") {
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
	deathReward := worldruntime.StaticActorDeathReward{Experience: current[idx].RewardExperience, Gold: current[idx].RewardGold, DropVnums: append([]uint32(nil), current[idx].RewardDropVnums...)}
	killQuestCredit := staticActorKillQuestCredit{QuestRef: current[idx].RewardQuestRef, QuestFlag: current[idx].RewardQuestFlag, QuestFrom: current[idx].RewardQuestFrom, QuestTo: current[idx].RewardQuestTo, Text: current[idx].RewardQuestText, RequireQuestRef: current[idx].RequireQuestRef, RequireQuestFlag: current[idx].RequireQuestFlag, RequireQuestFrom: current[idx].RequireQuestFrom}
	if combatProfile == "" && current[idx].SpawnGroupRef != "" {
		combatProfile = current[idx].CombatProfile
	}
	if spawnGroupRef == "" {
		spawnGroupRef = current[idx].SpawnGroupRef
	}
	target[idx] = StaticActorSnapshot{EntityID: entityID, Name: name, MapIndex: mapIndex, X: x, Y: y, RaceNum: raceNum, CombatProfile: combatProfile, InteractionKind: interactionKind, InteractionRef: interactionRef, SpawnGroupRef: spawnGroupRef, RewardExperience: deathReward.Experience, RewardGold: deathReward.Gold, RewardDropVnums: append([]uint32(nil), deathReward.DropVnums...), RewardQuestRef: killQuestCredit.QuestRef, RewardQuestFlag: killQuestCredit.QuestFlag, RewardQuestFrom: killQuestCredit.QuestFrom, RewardQuestTo: killQuestCredit.QuestTo, RewardQuestText: killQuestCredit.Text, RequireQuestRef: killQuestCredit.RequireQuestRef, RequireQuestFlag: killQuestCredit.RequireQuestFlag, RequireQuestFrom: killQuestCredit.RequireQuestFrom}
	if current[idx].SpawnHome != nil {
		spawnHome := *current[idx].SpawnHome
		target[idx].SpawnHome = &spawnHome
	}
	if !r.persistStaticActorSnapshot(target) {
		return StaticActorSnapshot{}, false
	}
	updated, ok := r.sharedWorld.updateStaticActor(entityID, name, mapIndex, x, y, raceNum, interactionKind, interactionRef, combatProfile, spawnGroupRef)
	if !ok {
		_ = r.persistStaticActorSnapshot(current)
		return StaticActorSnapshot{}, false
	}
	if !r.sharedWorld.setStaticActorKillQuestCredit(entityID, killQuestCredit) {
		_ = r.persistStaticActorSnapshot(current)
		return StaticActorSnapshot{}, false
	}
	r.syncSpawnGroupReturnStepSchedule(updated)
	// Operator/runtime update releases engagement / selected-target ownership in
	// shared-world; clear any pending chase deadline so a stale 5s chase MOVE
	// cannot fire after that owned reset boundary (matches return-home / remove).
	r.clearSpawnGroupChaseStep(entityID)
	// Mirror return_required return-step re-arm: a same-map displace that leaves
	// the actor live, unengaged, and within_radius must re-arm pending homeward
	// instead of only clearing the deadline. at_home / return_required / dead /
	// engaged / non-spawn outcomes still clear through the shared eligibility sync.
	r.syncSpawnGroupHomewardStepScheduleForEntity(entityID)
	if credit, ok := r.sharedWorld.StaticActorKillQuestCredit(entityID); ok {
		updated.RewardQuestRef = credit.QuestRef
		updated.RewardQuestFlag = credit.QuestFlag
		updated.RewardQuestFrom = credit.QuestFrom
		updated.RewardQuestTo = credit.QuestTo
		updated.RewardQuestText = credit.Text
		updated.RequireQuestRef = credit.RequireQuestRef
		updated.RequireQuestFlag = credit.RequireQuestFlag
		updated.RequireQuestFrom = credit.RequireQuestFrom
	} else {
		updated.RewardQuestRef = ""
		updated.RewardQuestFlag = ""
		updated.RewardQuestFrom = 0
		updated.RewardQuestTo = 0
		updated.RewardQuestText = ""
		updated.RequireQuestRef = ""
		updated.RequireQuestFlag = ""
		updated.RequireQuestFrom = 0
	}
	return updated, true
}

func (r *gameRuntime) StaticActors() []StaticActorSnapshot {
	if r == nil || r.sharedWorld == nil {
		return nil
	}
	return r.sharedWorld.StaticActors()
}

func (r *gameRuntime) SpawnGroups() []StaticActorSnapshot {
	if r == nil || r.sharedWorld == nil {
		return nil
	}
	return r.sharedWorld.SpawnGroups()
}

func (r *gameRuntime) SpawnGroupsForMap(mapIndex uint32) ([]StaticActorSnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return nil, false
	}
	return r.sharedWorld.SpawnGroupsForMap(mapIndex)
}

func (r *gameRuntime) SpawnGroupLeashesForMap(mapIndex uint32, radius int32) ([]SpawnGroupLeashSnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return nil, false
	}
	return r.sharedWorld.SpawnGroupLeashesForMap(mapIndex, radius)
}

func (r *gameRuntime) StaticActorsForMap(mapIndex uint32) ([]StaticActorSnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return nil, false
	}
	return r.sharedWorld.StaticActorsForMap(mapIndex)
}

func (r *gameRuntime) SpawnGroup(entityID uint64) (StaticActorSnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return StaticActorSnapshot{}, false
	}
	return r.sharedWorld.SpawnGroup(entityID)
}

func (r *gameRuntime) SpawnGroupByRef(ref string) (StaticActorSnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return StaticActorSnapshot{}, false
	}
	return r.sharedWorld.SpawnGroupByRef(ref)
}

func (r *gameRuntime) SpawnGroupLeash(entityID uint64, radius int32) (SpawnGroupLeashSnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return SpawnGroupLeashSnapshot{}, false
	}
	return r.sharedWorld.SpawnGroupLeash(entityID, radius)
}

func (r *gameRuntime) ReturnSpawnGroupHome(entityID uint64) (SpawnGroupLeashSnapshot, bool) {
	if r == nil || r.sharedWorld == nil || entityID == 0 {
		return SpawnGroupLeashSnapshot{}, false
	}

	r.staticActorMu.Lock()
	defer r.staticActorMu.Unlock()

	current := r.sharedWorld.StaticActors()
	idx := staticActorSnapshotIndex(current, entityID)
	if idx == -1 || current[idx].SpawnGroupRef == "" || current[idx].SpawnHome == nil {
		return SpawnGroupLeashSnapshot{}, false
	}
	target := cloneStaticActorSnapshots(current)
	home := *current[idx].SpawnHome
	target[idx].MapIndex = home.MapIndex
	target[idx].X = home.X
	target[idx].Y = home.Y
	positionUnchanged := target[idx].MapIndex == current[idx].MapIndex && target[idx].X == current[idx].X && target[idx].Y == current[idx].Y
	if !positionUnchanged {
		if !r.persistStaticActorSnapshot(target) {
			return SpawnGroupLeashSnapshot{}, false
		}
	}
	returned, ok := r.sharedWorld.ReturnSpawnGroupHome(entityID)
	if !ok {
		if !positionUnchanged {
			_ = r.persistStaticActorSnapshot(current)
		}
		return SpawnGroupLeashSnapshot{}, false
	}
	r.syncSpawnGroupReturnStepSchedule(returned.Actor)
	r.clearSpawnGroupChaseStep(entityID)
	r.clearSpawnGroupHomewardStep(entityID)
	return returned, true
}

func (r *gameRuntime) StepSpawnGroupReturnHome(entityID uint64, maxStep int32) (SpawnGroupReturnStepSnapshot, bool) {
	return r.stepSpawnGroupReturnHome(entityID, maxStep, true)
}

func (r *gameRuntime) stepSpawnGroupReturnHome(entityID uint64, maxStep int32, reschedule bool) (SpawnGroupReturnStepSnapshot, bool) {
	if r == nil || r.sharedWorld == nil || entityID == 0 || maxStep <= 0 {
		return SpawnGroupReturnStepSnapshot{}, false
	}

	r.staticActorMu.Lock()
	defer r.staticActorMu.Unlock()

	current := r.sharedWorld.StaticActors()
	idx := staticActorSnapshotIndex(current, entityID)
	if idx == -1 || current[idx].SpawnGroupRef == "" || current[idx].SpawnHome == nil {
		return SpawnGroupReturnStepSnapshot{}, false
	}
	plan, ok := r.sharedWorld.PlanSpawnGroupReturnHomeStep(entityID, maxStep)
	if !ok {
		r.clearSpawnGroupReturnStep(entityID)
		return SpawnGroupReturnStepSnapshot{}, false
	}
	if plan.Complete && !plan.Evaluation.ReturnRequired {
		r.clearSpawnGroupReturnStep(entityID)
		currentLeash := worldruntime.SpawnLeashSnapshotFromEvaluation(plan.Evaluation)
		return SpawnGroupReturnStepSnapshot{
			Actor: current[idx],
			Step: SpawnLeashReturnStepSnapshot{
				SpawnLeashSnapshot: currentLeash,
				Next:               worldruntime.PositionSnapshotFromPosition(plan.Next),
				Complete:           true,
			},
		}, true
	}

	target := cloneStaticActorSnapshots(current)
	target[idx].MapIndex = plan.Next.MapIndex
	target[idx].X = plan.Next.X
	target[idx].Y = plan.Next.Y
	if !r.persistStaticActorSnapshot(target) {
		return SpawnGroupReturnStepSnapshot{}, false
	}
	stepped, ok := r.sharedWorld.StepSpawnGroupReturnHome(entityID, maxStep)
	if !ok {
		_ = r.persistStaticActorSnapshot(current)
		return SpawnGroupReturnStepSnapshot{}, false
	}
	r.clearSpawnGroupChaseStep(entityID)
	r.clearSpawnGroupHomewardStep(entityID)
	if stepped.Step.Complete || stepped.Actor.SpawnLeash == nil || !stepped.Actor.SpawnLeash.ReturnRequired {
		r.clearSpawnGroupReturnStep(entityID)
	} else if reschedule {
		r.scheduleSpawnGroupReturnStep(entityID)
	}
	return stepped, true
}

func (r *gameRuntime) StaticActor(entityID uint64) (StaticActorSnapshot, bool) {
	if r == nil || r.sharedWorld == nil {
		return StaticActorSnapshot{}, false
	}
	return r.sharedWorld.StaticActor(entityID)
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
	r.clearSpawnGroupReturnStep(entityID)
	r.clearSpawnGroupChaseStep(entityID)
	r.clearSpawnGroupHomewardStep(entityID)
	return removed, true
}

func NewAuthSessionFactory() service.SessionFactory {
	cfg := config.LoadService("authd", "127.0.0.1:6061", ":11002", "127.0.0.1")
	return NewAuthSessionFactoryWithConfig(cfg)
}

func NewAuthSessionFactoryWithConfig(cfg config.Service) service.SessionFactory {
	return newAuthSessionFactoryWithAccountStore(
		loginticket.NewFileStore(serviceLoginTicketStoreDir(cfg)),
		accountstore.NewFileStore(serviceAccountStoreDir(cfg)),
		randomLoginKey,
	)
}

func NewAuthSessionFactoryWithValidatedConfig(cfg config.Service) (service.SessionFactory, error) {
	if err := config.ValidateOpsConfig(cfg); err != nil {
		return nil, err
	}
	cfg = servicePersistenceConfigWithDefaults(cfg)
	if err := config.ValidateHandoffPersistenceConfig(cfg); err != nil {
		return nil, err
	}
	return newAuthSessionFactoryWithAccountStore(
		loginticket.NewFileStore(cfg.LoginTicketStoreDir),
		accountstore.NewFileStore(cfg.AccountStoreDir),
		randomLoginKey,
	), nil
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
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		cfg,
		loginticket.NewFileStore(serviceLoginTicketStoreDir(cfg)),
		accountstore.NewFileStore(serviceAccountStoreDir(cfg)),
		nil,
		nil,
		itemcatalog.NewFileStore(serviceItemTemplateStorePath(cfg)),
		nil,
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
	return newGameRuntimeWithStoresAndTransferTriggersAndItemStore(cfg, store, accounts, nil, nil, nil, nil)
}

func newGameRuntimeWithAccountStoreAndStaticStore(cfg config.Service, store loginticket.Store, accounts accountstore.Store, staticActors staticstore.Store) (*gameRuntime, error) {
	return newGameRuntimeWithStoresAndTransferTriggersAndItemStore(cfg, store, accounts, staticActors, nil, nil, nil)
}

func newGameRuntimeWithAccountStoreAndInteractionStore(cfg config.Service, store loginticket.Store, accounts accountstore.Store, interactions interactionstore.Store) (*gameRuntime, error) {
	return newGameRuntimeWithStoresAndTransferTriggersAndItemStore(cfg, store, accounts, nil, interactions, nil, nil)
}

func newGameRuntimeWithAccountStoreAndInteractionAndItemStore(cfg config.Service, store loginticket.Store, accounts accountstore.Store, interactions interactionstore.Store, items itemcatalog.Store) (*gameRuntime, error) {
	return newGameRuntimeWithStoresAndTransferTriggersAndItemStore(cfg, store, accounts, nil, interactions, items, nil)
}

func newGameRuntimeWithAccountStoreAndContentStores(cfg config.Service, store loginticket.Store, accounts accountstore.Store, staticActors staticstore.Store, interactions interactionstore.Store) (*gameRuntime, error) {
	return newGameRuntimeWithStoresAndTransferTriggersAndItemStore(cfg, store, accounts, staticActors, interactions, nil, nil)
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
	return newGameRuntimeWithStoresAndTransferTriggers(cfg, store, accounts, nil, nil, transferTriggers)
}

func newGameRuntimeWithStoresAndTransferTriggers(cfg config.Service, store loginticket.Store, accounts accountstore.Store, staticActors staticstore.Store, interactions interactionstore.Store, transferTriggers []bootstrapTransferTrigger) (*gameRuntime, error) {
	return newGameRuntimeWithStoresAndTransferTriggersAndItemStore(cfg, store, accounts, staticActors, interactions, nil, transferTriggers)
}

func newGameRuntimeWithStoresAndTransferTriggersAndItemStore(cfg config.Service, store loginticket.Store, accounts accountstore.Store, staticActors staticstore.Store, interactions interactionstore.Store, items itemcatalog.Store, transferTriggers []bootstrapTransferTrigger) (*gameRuntime, error) {
	return newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore(cfg, store, accounts, staticActors, interactions, items, nil, transferTriggers)
}

// newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore is the deepest
// test/runtime constructor. A non-nil questState is used as-is so hermetic
// gameplay tests can inject queststate.MemoryStore without construct-and-discard
// of a path-isolated FileStore. Nil keeps the ordinary FileStore default.
func newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore(cfg config.Service, store loginticket.Store, accounts accountstore.Store, staticActors staticstore.Store, interactions interactionstore.Store, items itemcatalog.Store, questState queststate.Store, transferTriggers []bootstrapTransferTrigger) (*gameRuntime, error) {
	if err := validateRuntimePersistenceConfig(cfg); err != nil {
		return nil, err
	}
	if err := config.ValidateDatabaseConfig(cfg); err != nil {
		return nil, err
	}

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
		store = loginticket.NewFileStore(serviceLoginTicketStoreDir(cfg))
	}
	if items == nil {
		items = itemcatalog.NewFileStore(serviceItemTemplateStorePath(cfg))
	}
	if questState == nil {
		questState = queststate.NewFileStore(serviceQuestStateStorePath(cfg))
	}
	groundItemPath := serviceGroundItemStorePath(cfg)
	if strings.TrimSpace(cfg.GroundItemStorePath) == "" {
		// Hermetic constructors that omit an explicit path must not share the
		// process-wide default FileStore; otherwise one test's pending drops
		// rematerialize into the next runtime via /tmp pollution.
		groundItemPath = filepath.Join(os.TempDir(), fmt.Sprintf("go-metin2-ground-items-%d-%d", os.Getpid(), time.Now().UnixNano()), "ground-items.json")
	}
	groundItems := worldruntime.NewGroundItemFileStore(groundItemPath)
	safeboxPath := serviceSafeboxStorePath(cfg)
	if strings.TrimSpace(cfg.SafeboxStorePath) == "" {
		// Hermetic constructors that omit an explicit path must not share the
		// process-wide default FileStore; otherwise one test's safebox cells
		// rematerialize into the next runtime via /tmp pollution.
		safeboxPath = filepath.Join(os.TempDir(), fmt.Sprintf("go-metin2-safebox-%d-%d", os.Getpid(), time.Now().UnixNano()), "safebox.json")
	}
	safeboxItems := safeboxstore.NewFileStore(safeboxPath)
	sharedWorld := newSharedWorldRegistryWithTopology(topology)
	runtime := &gameRuntime{
		sharedWorld:            sharedWorld,
		config:                 cfg,
		staticStore:            staticActors,
		loginTicketStore:       store,
		accountStore:           accounts,
		itemStore:              items,
		interactionStore:       interactions,
		questStateStore:        questState,
		groundItemStore:        groundItems,
		safeboxStore:           safeboxItems,
		liveCharactersByName:   make(map[string]liveCharacterRegistration),
		spawnReturnStepDueAt:   make(map[uint64]time.Time),
		spawnChaseStepDueAt:    make(map[uint64]time.Time),
		spawnHomewardStepDueAt: make(map[uint64]time.Time),
		now:                    time.Now,
	}
	sharedWorld.now = func() time.Time {
		if runtime != nil && runtime.now != nil {
			return runtime.now()
		}
		return time.Now()
	}
	sharedWorld.SetGroundItemsChangedHook(runtime.persistPendingGroundItems)
	if err := runtime.loadItemTemplates(); err != nil {
		return nil, err
	}
	sharedWorld.SetItemTemplates(runtime.itemTemplates)
	if err := runtime.loadInteractionDefinitions(); err != nil {
		return nil, err
	}
	if err := runtime.loadPersistedStaticActors(); err != nil {
		return nil, err
	}
	if err := runtime.loadPersistedGroundItems(); err != nil {
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
		var liveCharacterRegistrationName string
		var liveCharacterRegistrationID uint64
		pending := newPendingServerFrames()
		var sharedWorldID uint64
		var joinedSharedWorld bool
		var activeCombatTargetVID uint32
		var activeCombatTargetSnapshotVersion uint64
		var nextAllowedNormalAttackAt time.Time
		activeCharacterPosition := bootstrapCharacterPositionGeneral
		var pendingPracticeMobServerOriginRetaliation bool
		var pendingPracticeMobServerOriginRetaliationAt time.Time
		var pendingPracticeMobServerOriginRetaliationTargetVID uint32
		var pendingPracticeMobServerOriginRetaliationSnapshotVersion uint64
		var issuedPracticeMobServerOriginRetaliationSnapshotVersion uint64
		var activeMerchantBuy merchantBuyContext
		var hasActiveMerchantBuy bool
		var hasActiveSafeboxOpen bool
		var activeSafeboxSize uint8
		activeSafeboxItems := make(map[uint8]inventory.ItemInstance)
		var pendingSafeboxPasswordChallenge bool
		var pendingSafeboxPasswordSize uint8
		var activeRefineDialog refineDialogPresentation
		var hasActiveRefineDialog bool
		bootstrapSafeboxCapacity := func(size uint8) uint8 {
			if size < bootstrapSafeboxOpenMinSize || size > bootstrapSafeboxOpenMaxSize {
				return 0
			}
			return size * bootstrapSafeboxCellsPerPage
		}
		// TMP4 SendSafeBoxItemMovePacket packs both TItemPos fields with the
		// inventory window type and safebox-slot cell bytes. Explicit tooling
		// may also name WindowSafebox. Cells are always interpreted as same-
		// session safebox slot indices while the presentation is open.
		acceptedSafeboxItemMoveWindow := func(windowType uint8) bool {
			return windowType == itemproto.WindowInventory || windowType == itemproto.WindowSafebox
		}
		clearActiveSafeboxItems := func() {
			if len(activeSafeboxItems) == 0 {
				return
			}
			activeSafeboxItems = make(map[uint8]inventory.ItemInstance)
		}
		clearPendingSafeboxPasswordChallenge := func() {
			pendingSafeboxPasswordChallenge = false
			pendingSafeboxPasswordSize = 0
		}
		setPendingSafeboxPasswordChallenge := func(size uint8) {
			pendingSafeboxPasswordChallenge = true
			pendingSafeboxPasswordSize = size
		}
		durableSafeboxPassword := func(selected *player.Runtime) string {
			if runtime == nil || runtime.safeboxStore == nil || !hasTicket || selected == nil {
				return safeboxstore.DefaultPassword
			}
			snapshot, err := safeboxstore.LoadOrEmpty(runtime.safeboxStore)
			if err != nil {
				return safeboxstore.DefaultPassword
			}
			return safeboxstore.CharacterPassword(snapshot, sessionTicket.Login, selected.LiveCharacter().ID)
		}
		durableSafeboxMoney := func(selected *player.Runtime) int64 {
			if runtime == nil || runtime.safeboxStore == nil || !hasTicket || selected == nil {
				return 0
			}
			snapshot, err := safeboxstore.LoadOrEmpty(runtime.safeboxStore)
			if err != nil {
				return 0
			}
			return safeboxstore.CharacterMoney(snapshot, sessionTicket.Login, selected.LiveCharacter().ID)
		}
		cloneActiveSafeboxItems := func() map[uint8]inventory.ItemInstance {
			if len(activeSafeboxItems) == 0 {
				return make(map[uint8]inventory.ItemInstance)
			}
			cloned := make(map[uint8]inventory.ItemInstance, len(activeSafeboxItems))
			for slot, item := range activeSafeboxItems {
				cloned[slot] = item
			}
			return cloned
		}
		hydrateActiveSafeboxFromStore := func() {
			if runtime == nil || runtime.safeboxStore == nil || !hasTicket || selectedPlayer == nil {
				activeSafeboxItems = make(map[uint8]inventory.ItemInstance)
				return
			}
			snapshot, err := safeboxstore.LoadOrEmpty(runtime.safeboxStore)
			if err != nil {
				activeSafeboxItems = make(map[uint8]inventory.ItemInstance)
				return
			}
			activeSafeboxItems = safeboxstore.CharacterCells(snapshot, sessionTicket.Login, selectedPlayer.LiveCharacter().ID)
			if activeSafeboxItems == nil {
				activeSafeboxItems = make(map[uint8]inventory.ItemInstance)
			}
		}
		persistActiveSafeboxCells := func(selected *player.Runtime) error {
			if runtime == nil || runtime.safeboxStore == nil || selected == nil {
				return nil
			}
			if !hasTicket {
				return nil
			}
			runtime.safeboxPersistMu.Lock()
			defer runtime.safeboxPersistMu.Unlock()
			snapshot, err := safeboxstore.LoadOrEmpty(runtime.safeboxStore)
			if err != nil {
				return err
			}
			next, err := safeboxstore.ReplaceCharacterCells(snapshot, sessionTicket.Login, selected.LiveCharacter().ID, activeSafeboxItems)
			if err != nil {
				return err
			}
			return runtime.safeboxStore.Save(next)
		}
		persistDurableSafeboxPassword := func(selected *player.Runtime, password string) error {
			if runtime == nil || runtime.safeboxStore == nil || selected == nil || !hasTicket {
				return safeboxstore.ErrInvalidSnapshot
			}
			runtime.safeboxPersistMu.Lock()
			defer runtime.safeboxPersistMu.Unlock()
			snapshot, err := safeboxstore.LoadOrEmpty(runtime.safeboxStore)
			if err != nil {
				return err
			}
			next, err := safeboxstore.ReplaceCharacterPassword(snapshot, sessionTicket.Login, selected.LiveCharacter().ID, password)
			if err != nil {
				return err
			}
			return runtime.safeboxStore.Save(next)
		}
		persistDurableSafeboxMoney := func(selected *player.Runtime, money int64) error {
			if runtime == nil || runtime.safeboxStore == nil || selected == nil || !hasTicket {
				return safeboxstore.ErrInvalidSnapshot
			}
			runtime.safeboxPersistMu.Lock()
			defer runtime.safeboxPersistMu.Unlock()
			snapshot, err := safeboxstore.LoadOrEmpty(runtime.safeboxStore)
			if err != nil {
				return err
			}
			next, err := safeboxstore.ReplaceCharacterMoney(snapshot, sessionTicket.Login, selected.LiveCharacter().ID, money)
			if err != nil {
				return err
			}
			return runtime.safeboxStore.Save(next)
		}
		encodeActiveSafeboxSetFrames := func() [][]byte {
			if len(activeSafeboxItems) == 0 {
				return nil
			}
			capacity := bootstrapSafeboxCapacity(activeSafeboxSize)
			slots := make([]uint8, 0, len(activeSafeboxItems))
			for slot := range activeSafeboxItems {
				if capacity > 0 && slot >= capacity {
					continue
				}
				slots = append(slots, slot)
			}
			sort.Slice(slots, func(i, j int) bool { return slots[i] < slots[j] })
			frames := make([][]byte, 0, len(slots))
			for _, slot := range slots {
				item := activeSafeboxItems[slot]
				frame, err := encodeBootstrapSafeboxSetFrame(itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: uint16(slot)}, item, runtime.itemTemplates)
				if err != nil {
					continue
				}
				frames = append(frames, frame)
			}
			return frames
		}
		interactionCooldowns := make(map[uint32]time.Time)
		sessionNow := func() time.Time {
			if runtime != nil && runtime.now != nil {
				return runtime.now()
			}
			return time.Now()
		}
		interactionNow := func() time.Time {
			return sessionNow()
		}
		interactionOnCooldown := func(targetVID uint32) bool {
			until, ok := interactionCooldowns[targetVID]
			return ok && interactionNow().Before(until)
		}
		markInteractionCooldown := func(targetVID uint32) {
			if targetVID == 0 {
				return
			}
			interactionCooldowns[targetVID] = interactionNow().Add(staticActorInteractionCooldown)
		}
		clearActiveMerchantBuy := func() {
			activeMerchantBuy = merchantBuyContext{}
			hasActiveMerchantBuy = false
			if joinedSharedWorld && sharedWorld != nil && sharedWorldID != 0 {
				sharedWorld.SetMerchantWindowOpen(sharedWorldID, false)
			}
		}
		setActiveSafeboxOpen := func(size uint8, open bool) {
			if !open {
				hasActiveSafeboxOpen = false
				activeSafeboxSize = 0
				if joinedSharedWorld && sharedWorld != nil && sharedWorldID != 0 {
					sharedWorld.SetSafeboxWindowOpen(sharedWorldID, false)
				}
				return
			}
			hasActiveSafeboxOpen = true
			activeSafeboxSize = size
			if joinedSharedWorld && sharedWorld != nil && sharedWorldID != 0 {
				sharedWorld.SetSafeboxWindowOpen(sharedWorldID, true)
			}
		}
		openSafeboxPresentation := func(size uint8) [][]byte {
			setActiveSafeboxOpen(size, true)
			hydrateActiveSafeboxFromStore()
			money := durableSafeboxMoney(selectedPlayer)
			frames := [][]byte{itemproto.EncodeSafeboxSize(itemproto.SafeboxSizePacket{Size: size})}
			frames = append(frames, encodeActiveSafeboxSetFrames()...)
			frames = append(frames, itemproto.EncodeSafeboxMoneyChange(itemproto.SafeboxMoneyChangePacket{Money: int32(money)}))
			return frames
		}
		setActiveRefineDialog := func(dialog refineDialogPresentation, open bool) {
			if !open {
				hasActiveRefineDialog = false
				activeRefineDialog = refineDialogPresentation{}
				if joinedSharedWorld && sharedWorld != nil && sharedWorldID != 0 {
					sharedWorld.SetRefineWindowOpen(sharedWorldID, false)
				}
				return
			}
			activeRefineDialog = dialog
			hasActiveRefineDialog = true
			if joinedSharedWorld && sharedWorld != nil && sharedWorldID != 0 {
				sharedWorld.SetRefineWindowOpen(sharedWorldID, true)
			}
		}
		appendPostFloorMerchantCloseFrame := func(frames [][]byte, clearTarget bool) [][]byte {
			if !clearTarget || !hasActiveMerchantBuy || activeMerchantBuy.TargetVID == 0 {
				return frames
			}
			clearActiveMerchantBuy()
			return append(frames, shopproto.EncodeServerEnd())
		}
		appendPostFloorSafeboxCloseFrame := func(frames [][]byte, clearTarget bool) [][]byte {
			if !clearTarget {
				return frames
			}
			clearPendingSafeboxPasswordChallenge()
			if !hasActiveSafeboxOpen {
				return frames
			}
			setActiveSafeboxOpen(0, false)
			return append(frames, chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
				Type:    chatproto.ChatTypeCommand,
				Message: "CloseSafebox",
			}))
		}
		appendPostFloorExchangeCloseFrame := func(frames [][]byte, clearTarget bool) [][]byte {
			if !clearTarget || sharedWorld == nil || !joinedSharedWorld || sharedWorldID == 0 || !sharedWorld.HasLiveSession(sharedWorldID) {
				return frames
			}
			closeFrames, ok := sharedWorld.CloseExchange(sharedWorldID)
			if !ok || len(closeFrames) == 0 {
				return frames
			}
			return append(frames, closeFrames...)
		}
		appendPostFloorContextCloseFrames := func(frames [][]byte, clearTarget bool) [][]byte {
			frames = appendPostFloorMerchantCloseFrame(frames, clearTarget)
			frames = appendPostFloorSafeboxCloseFrame(frames, clearTarget)
			frames = appendPostFloorExchangeCloseFrame(frames, clearTarget)
			if clearTarget {
				setActiveRefineDialog(refineDialogPresentation{}, false)
			}
			return frames
		}
		prependMerchantCloseFrame := func(frames [][]byte) [][]byte {
			if !hasActiveMerchantBuy || activeMerchantBuy.TargetVID == 0 {
				return frames
			}
			clearActiveMerchantBuy()
			return append([][]byte{shopproto.EncodeServerEnd()}, frames...)
		}
		prependSafeboxCloseFrame := func(frames [][]byte) [][]byte {
			clearPendingSafeboxPasswordChallenge()
			if !hasActiveSafeboxOpen {
				return frames
			}
			setActiveSafeboxOpen(0, false)
			return append([][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
				Type:    chatproto.ChatTypeCommand,
				Message: "CloseSafebox",
			})}, frames...)
		}
		prependExchangeCloseFrame := func(frames [][]byte) [][]byte {
			if sharedWorld == nil || !joinedSharedWorld || sharedWorldID == 0 || !sharedWorld.HasLiveSession(sharedWorldID) {
				return frames
			}
			closeFrames, ok := sharedWorld.CloseExchange(sharedWorldID)
			if !ok || len(closeFrames) == 0 {
				return frames
			}
			return append(closeFrames, frames...)
		}
		exchangeDisplaysCarriedSlot := func(slot inventory.SlotIndex) bool {
			if sharedWorld == nil || !joinedSharedWorld || sharedWorldID == 0 || !sharedWorld.HasLiveSession(sharedWorldID) {
				return false
			}
			return sharedWorld.HasExchangeDisplayedCarriedSlot(sharedWorldID, slot)
		}
		closeTransferExchangeShell := func() [][]byte {
			if sharedWorld == nil || !joinedSharedWorld || sharedWorldID == 0 || !sharedWorld.HasLiveSession(sharedWorldID) {
				return nil
			}
			closeFrames, ok := sharedWorld.CloseExchange(sharedWorldID)
			if !ok || len(closeFrames) == 0 {
				return nil
			}
			return closeFrames
		}
		prependTransferBusyCloseFrames := func(frames [][]byte, exchangeCloseFrames [][]byte, rebootstrap bool) [][]byte {
			if !rebootstrap {
				return frames
			}
			if len(exchangeCloseFrames) > 0 {
				frames = append(append([][]byte{}, exchangeCloseFrames...), frames...)
			}
			frames = prependSafeboxCloseFrame(frames)
			return prependMerchantCloseFrame(frames)
		}
		clearActiveCombatTarget := func() {
			engagedEntityIDs := make([]uint64, 0)
			if sharedWorld != nil && sharedWorldID != 0 {
				sharedWorld.mu.Lock()
				for entityID, engagedBy := range sharedWorld.staticActorCombatEngagedBy {
					if engagedBy == sharedWorldID {
						engagedEntityIDs = append(engagedEntityIDs, entityID)
					}
				}
				sharedWorld.mu.Unlock()
				for _, entityID := range engagedEntityIDs {
					runtime.clearSpawnGroupChaseStep(entityID)
				}
			}
			activeCombatTargetVID = 0
			activeCombatTargetSnapshotVersion = 0
			nextAllowedNormalAttackAt = time.Time{}
			pendingPracticeMobServerOriginRetaliation = false
			pendingPracticeMobServerOriginRetaliationAt = time.Time{}
			pendingPracticeMobServerOriginRetaliationTargetVID = 0
			pendingPracticeMobServerOriginRetaliationSnapshotVersion = 0
			issuedPracticeMobServerOriginRetaliationSnapshotVersion = 0
			if sharedWorld != nil && sharedWorldID != 0 {
				sharedWorld.ClearSessionCombatTarget(sharedWorldID)
				sharedWorld.ClearStaticActorCombatEngagementsBySubject(sharedWorldID)
			}
			// Transfer/relocate may already have released engagement ownership before
			// this helper runs; prune drops any chase deadlines that lost eligibility.
			runtime.pruneSpawnGroupChaseStepSchedules()
			for _, entityID := range engagedEntityIDs {
				runtime.syncSpawnGroupHomewardStepScheduleForEntity(entityID)
			}
			runtime.pruneSpawnGroupHomewardStepSchedules()
		}
		clearPendingPracticeMobServerOriginRetaliation := func() {
			pendingPracticeMobServerOriginRetaliation = false
			pendingPracticeMobServerOriginRetaliationAt = time.Time{}
			pendingPracticeMobServerOriginRetaliationTargetVID = 0
			pendingPracticeMobServerOriginRetaliationSnapshotVersion = 0
			if sharedWorld != nil && sharedWorldID != 0 {
				sharedWorld.ClearSessionCombatRetaliation(sharedWorldID)
			}
		}
		resetPracticeMobServerOriginRetaliationState := func() {
			clearPendingPracticeMobServerOriginRetaliation()
			issuedPracticeMobServerOriginRetaliationSnapshotVersion = 0
		}
		scheduleFirstPracticeMobServerOriginRetaliation := func(targetVID uint32, snapshotVersion uint64) {
			if targetVID == 0 || snapshotVersion == 0 || issuedPracticeMobServerOriginRetaliationSnapshotVersion == snapshotVersion {
				return
			}
			if pendingPracticeMobServerOriginRetaliation && pendingPracticeMobServerOriginRetaliationTargetVID == targetVID && pendingPracticeMobServerOriginRetaliationSnapshotVersion == snapshotVersion {
				return
			}
			now := sessionNow()
			readyAt := now.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
			pendingPracticeMobServerOriginRetaliation = true
			pendingPracticeMobServerOriginRetaliationAt = readyAt
			pendingPracticeMobServerOriginRetaliationTargetVID = targetVID
			pendingPracticeMobServerOriginRetaliationSnapshotVersion = snapshotVersion
			issuedPracticeMobServerOriginRetaliationSnapshotVersion = snapshotVersion
			if sharedWorld != nil && sharedWorldID != 0 {
				sharedWorld.SetSessionCombatRetaliation(sharedWorldID, targetVID, snapshotVersion, readyAt)
			}
		}
		var armPracticeMobServerOriginRetaliationFromProximityEngagement func()
		var flushPendingPracticeMobServerOriginRetaliation func(pending *pendingServerFrames)
		enqueueCombatTargetClear := func() {
			pending.Enqueue([][]byte{combatproto.EncodeServerClearTarget()})
		}
		clearInvalidActiveCombatTargetAfterMovement := func() {
			if runtime == nil || !joinedSharedWorld || sharedWorldID == 0 || sharedWorld == nil || !sharedWorld.HasLiveSession(sharedWorldID) {
				return
			}
			if activeCombatTargetVID != 0 {
				resolution := runtime.resolveStaticActorCombatTarget(sharedWorldID, activeCombatTargetVID)
				if resolution.Accepted && resolution.Packet != nil && resolution.Packet.TargetVID == activeCombatTargetVID {
					return
				}
				clearActiveCombatTarget()
				enqueueCombatTargetClear()
				return
			}
			// Proximity-only engagement has no selected target. Leaving the
			// default aggro radius must still cancel delayed retaliation and
			// release engaged_by without inventing TARGET(0, 0).
			released := sharedWorld.ReleaseProximitySpawnGroupEngagementsOutsideAggroRadius(sharedWorldID)
			if len(released) == 0 {
				return
			}
			clearPendingPracticeMobServerOriginRetaliation()
			issuedPracticeMobServerOriginRetaliationSnapshotVersion = 0
			for _, entityID := range released {
				runtime.clearSpawnGroupChaseStep(entityID)
				runtime.syncSpawnGroupHomewardStepScheduleForEntity(entityID)
			}
			runtime.pruneSpawnGroupChaseStepSchedules()
			runtime.pruneSpawnGroupHomewardStepSchedules()
		}
		clearInvalidActiveMerchantBuyAfterMovement := func() {
			if !hasActiveMerchantBuy || activeMerchantBuy.TargetVID == 0 || runtime == nil || !joinedSharedWorld || sharedWorldID == 0 || sharedWorld == nil || !sharedWorld.HasLiveSession(sharedWorldID) {
				return
			}
			resolution := runtime.resolveStaticActorInteraction(sharedWorldID, activeMerchantBuy.TargetVID)
			if resolution.Accepted && resolution.Definition.Kind == interactionstore.KindShopPreview && reflect.DeepEqual(activeMerchantBuy.Definition, resolution.Definition) {
				return
			}
			clearActiveMerchantBuy()
			pending.Enqueue([][]byte{shopproto.EncodeServerEnd()})
		}
		clearLiveCharacterRegistration := func() {
			if liveCharacterRegistrationID == 0 {
				return
			}
			runtime.unregisterLiveCharacterSnapshotter(liveCharacterRegistrationName, liveCharacterRegistrationID)
			liveCharacterRegistrationName = ""
			liveCharacterRegistrationID = 0
		}
		refreshLiveCharacterRegistration := func() {
			clearLiveCharacterRegistration()
			if runtime == nil || !hasSelected || selectedPlayer == nil {
				return
			}
			selected := selectedPlayer.LiveCharacter()
			if selected.ID == 0 {
				return
			}
			name := normalizeLiveCharacterName(selected.Name)
			if name == "" {
				return
			}
			liveCharacterRegistrationName = name
			liveCharacterRegistrationID = runtime.registerLiveCharacterSnapshotter(name, sessionTicket.Login, func() (liveCharacterStateSnapshot, bool) {
				stateMu.Lock()
				defer stateMu.Unlock()
				if !hasSelected || selectedPlayer == nil {
					return liveCharacterStateSnapshot{}, false
				}
				current := selectedPlayer.LiveCharacter()
				if current.ID == 0 || normalizeLiveCharacterName(current.Name) != name {
					return liveCharacterStateSnapshot{}, false
				}
				return buildLiveCharacterStateSnapshot(current), true
			}, func(updated loginticket.Character) bool {
				stateMu.Lock()
				defer stateMu.Unlock()
				if !hasSelected || selectedPlayer == nil {
					return false
				}
				current := selectedPlayer.LiveCharacter()
				if current.ID == 0 || updated.ID == 0 || current.ID != updated.ID || normalizeLiveCharacterName(current.Name) != name {
					return false
				}
				link := selectedPlayer.SessionLink()
				updatedCharacters, ok := selectedCharacterSnapshotUpdate(sessionTicket.Characters, link.CharacterIndex, updated)
				if !ok {
					return false
				}
				sessionTicket.Characters = updatedCharacters
				selectedPlayer.ApplyPersistedSnapshot(updated)
				return true
			})
		}
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
				clearLiveCharacterRegistration()
				return false
			}
			sessionTicket.Empire = account.Empire
			sessionTicket.Characters = cloneCharacters(account.Characters)
			if int(selectedIndex) >= len(sessionTicket.Characters) {
				selectedPlayer = nil
				clearLiveCharacterRegistration()
				return false
			}
			selected := sessionTicket.Characters[selectedIndex]
			if selected.ID == 0 {
				selectedPlayer = nil
				clearLiveCharacterRegistration()
				return false
			}
			if selectedPlayer == nil {
				selectedPlayer = player.NewRuntime(selected, player.SessionLink{Login: sessionTicket.Login, CharacterIndex: selectedIndex})
			} else {
				selectedPlayer.ApplyPersistedSnapshot(selected)
			}
			refreshLiveCharacterRegistration()
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
			refreshLiveCharacterRegistration()
			return selectedPlayer, true
		}
		selectedPlayerAtBootstrapHPFloor := func(selected *player.Runtime) bool {
			if selected == nil {
				return false
			}
			return selected.LiveCharacter().Points[bootstrapPlayerPointValueIndex] <= 0
		}
		ownsLiveSharedWorldSession := func() bool {
			return joinedSharedWorld && sharedWorldID != 0 && sharedWorld.HasLiveSession(sharedWorldID)
		}
		armPracticeMobServerOriginRetaliationFromProximityEngagement = func() {
			if !ownsLiveSharedWorldSession() || sharedWorld == nil || sharedWorldID == 0 {
				return
			}
			if pendingPracticeMobServerOriginRetaliation {
				return
			}
			for _, target := range sharedWorld.EngagedSpawnGroupRetaliationArmTargets(sharedWorldID) {
				scheduleFirstPracticeMobServerOriginRetaliation(target.TargetVID, target.SnapshotVersion)
				if pendingPracticeMobServerOriginRetaliation {
					return
				}
			}
		}
		applySelectedCharacterTransfer := func(mapIndex uint32, x int32, y int32, rebootstrap bool) (RelocationPreview, [][]byte, bool) {
			selectedPlayer, ok := currentSelectedPlayer()
			if !ok || !joinedSharedWorld || sharedWorldID == 0 {
				return RelocationPreview{}, nil, false
			}
			selected := selectedPlayer.PersistedSnapshot()
			selectedLink := selectedPlayer.SessionLink()
			buildUpdatedSelection := func(updated loginticket.Character) ([]loginticket.Character, loginticket.Character, loginticket.Character, bool) {
				updatedPersisted := selectedPlayer.PersistedSnapshot()
				updatedLive := selectedPlayer.LiveCharacter()
				if updatedPersisted.ID == 0 || updatedLive.ID == 0 {
					return nil, loginticket.Character{}, loginticket.Character{}, false
				}
				updatedPersisted.MapIndex = updated.MapIndex
				updatedPersisted.X = updated.X
				updatedPersisted.Y = updated.Y
				updatedLive.MapIndex = updated.MapIndex
				updatedLive.X = updated.X
				updatedLive.Y = updated.Y
				updatedCharacters, ok := selectedCharacterSnapshotUpdate(sessionTicket.Characters, selectedLink.CharacterIndex, updatedPersisted)
				if !ok {
					return nil, loginticket.Character{}, loginticket.Character{}, false
				}
				return updatedCharacters, updatedPersisted, updatedLive, true
			}
			var transferResult RelocationPreview
			var transferFrames [][]byte
			var transferExchangeCloseFrames [][]byte
			transferFlow := warp.NewFlow(warp.Config{
				Persist: func(updated loginticket.Character) bool {
					updatedCharacters, _, _, ok := buildUpdatedSelection(updated)
					if !ok {
						return false
					}
					return saveAccountSnapshot(accounts, sessionTicket.Login, sessionTicket.Empire, updatedCharacters)
				},
				Rollback: func(previous loginticket.Character) bool {
					return saveAccountSnapshot(accounts, sessionTicket.Login, sessionTicket.Empire, sessionTicket.Characters)
				},
				Commit: func(updated loginticket.Character) (warp.Result, bool) {
					updatedCharacters, updatedPersisted, updatedLive, ok := buildUpdatedSelection(updated)
					if !ok {
						return warp.Result{}, false
					}
					if rebootstrap {
						// Close the exchange shell before Transfer so out-of-range
						// teardown cannot enqueue a late END after the burst, and so
						// same-map in-range destinations still tear the shell down.
						transferExchangeCloseFrames = closeTransferExchangeShell()
						runtime.flushReadyStaticActorRespawns()
						runtime.flushDueSpawnGroupReturnSteps()
						runtime.flushDueSpawnGroupHomewardSteps()
						runtime.flushDueSpawnGroupChaseSteps()
						runtime.flushProximitySpawnGroupAggroAcquisition()
						bootstrapFrames, err := worldentry.BuildBootstrapFramesWithTemplates(updatedLive, runtime.itemTemplates)
						if err != nil {
							return warp.Result{}, false
						}
						transferPreview, originFrames, ok := sharedWorld.TransferWithOriginFrames(sharedWorldID, updatedLive)
						if !ok {
							return warp.Result{}, false
						}
						transferResult = transferPreview
						transferFrames = append(append([][]byte(nil), bootstrapFrames...), originFrames...)
					} else {
						transferPreview, ok := sharedWorld.Transfer(sharedWorldID, updatedLive)
						if !ok {
							return warp.Result{}, false
						}
						transferResult = transferPreview
					}
					sessionTicket.Characters = updatedCharacters
					selectedPlayer.SetPersistedSnapshot(updatedPersisted)
					selectedPlayer.SetLivePosition(updatedLive.MapIndex, updatedLive.X, updatedLive.Y)
					return warp.Result{Applied: true, Updated: selectedPlayer.LiveCharacter()}, true
				},
			})
			if _, ok := transferFlow.Apply(selected, warp.Target{MapIndex: mapIndex, X: x, Y: y}); !ok {
				return RelocationPreview{}, nil, false
			}
			transferFrames = prependTransferBusyCloseFrames(transferFrames, transferExchangeCloseFrames, rebootstrap)
			clearActiveCombatTarget()
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
			updatedPersisted := selectedPlayer.PersistedSnapshot()
			updatedLive := selectedPlayer.LiveCharacter()
			if updatedPersisted.ID == 0 || updatedLive.ID == 0 {
				return loginticket.Character{}, false
			}
			updatedPersisted.X = x
			updatedPersisted.Y = y
			updatedLive.X = x
			updatedLive.Y = y
			updatedCharacters, ok := selectedCharacterSnapshotUpdate(sessionTicket.Characters, selectedPlayer.SessionLink().CharacterIndex, updatedPersisted)
			if !ok || !saveAccountSnapshot(accounts, sessionTicket.Login, sessionTicket.Empire, updatedCharacters) {
				return loginticket.Character{}, false
			}
			sessionTicket.Characters = updatedCharacters
			selectedPlayer.SetPersistedSnapshot(updatedPersisted)
			selectedPlayer.SetLivePosition(updatedLive.MapIndex, x, y)
			return selectedPlayer.LiveCharacter(), true
		}
		commitSelectedNonPointItemMutationFrames := func(selectedPlayer *player.Runtime, previousSelected loginticket.Character, frames [][]byte, stablePeerFrames [][]byte) ([][]byte, bool) {
			if selectedPlayer == nil {
				return nil, false
			}
			persistedSelected := selectedPlayer.PersistedSnapshot()
			updatedSelected := selectedPlayer.LiveCharacter()
			if persistedSelected.ID == 0 || updatedSelected.ID == 0 {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return nil, false
			}
			if !ownsLiveSharedWorldSession() {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return frames, true
			}
			persistedSelected.Gold = updatedSelected.Gold
			persistedSelected.Inventory = updatedSelected.Inventory
			persistedSelected.Equipment = updatedSelected.Equipment
			persistedSelected.Quickslots = updatedSelected.Quickslots
			updatedCharacters, ok := selectedCharacterSnapshotUpdate(sessionTicket.Characters, selectedPlayer.SessionLink().CharacterIndex, persistedSelected)
			if !ok {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return nil, false
			}
			if !saveAccountSnapshot(accounts, sessionTicket.Login, sessionTicket.Empire, updatedCharacters) {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return nil, false
			}
			sessionTicket.Characters = updatedCharacters
			selectedPlayer.SetPersistedSnapshot(persistedSelected)
			refreshLiveCharacterRegistration()
			if ownsLiveSharedWorldSession() {
				sharedWorld.UpdateCharacterWithVisibilityTransition(sharedWorldID, previousSelected, updatedSelected, stablePeerFrames)
			}
			return frames, true
		}
		commitSelectedPointBearingItemMutationFrames := func(selectedPlayer *player.Runtime, previousSelected loginticket.Character, frames [][]byte, stablePeerFrames [][]byte) ([][]byte, bool) {
			if selectedPlayer == nil {
				return nil, false
			}
			persistedSelected := selectedPlayer.PersistedSnapshot()
			updatedSelected := selectedPlayer.LiveCharacter()
			if persistedSelected.ID == 0 || updatedSelected.ID == 0 || previousSelected.ID == 0 {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return nil, false
			}
			if !ownsLiveSharedWorldSession() {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return frames, true
			}
			persistedSelected.Gold = updatedSelected.Gold
			persistedSelected.Inventory = updatedSelected.Inventory
			persistedSelected.Equipment = updatedSelected.Equipment
			persistedSelected.Quickslots = updatedSelected.Quickslots
			for pointIndex := range persistedSelected.Points {
				pointDelta := int64(updatedSelected.Points[pointIndex]) - int64(previousSelected.Points[pointIndex])
				pointValue := int64(persistedSelected.Points[pointIndex]) + pointDelta
				if pointValue < math.MinInt32 || pointValue > math.MaxInt32 {
					selectedPlayer.ApplyPersistedSnapshot(previousSelected)
					refreshLiveCharacterRegistration()
					return nil, false
				}
				persistedSelected.Points[pointIndex] = int32(pointValue)
			}
			updatedCharacters, ok := selectedCharacterSnapshotUpdate(sessionTicket.Characters, selectedPlayer.SessionLink().CharacterIndex, persistedSelected)
			if !ok {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return nil, false
			}
			if !saveAccountSnapshot(accounts, sessionTicket.Login, sessionTicket.Empire, updatedCharacters) {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return nil, false
			}
			sessionTicket.Characters = updatedCharacters
			selectedPlayer.SetPersistedSnapshot(persistedSelected)
			refreshLiveCharacterRegistration()
			if ownsLiveSharedWorldSession() {
				sharedWorld.UpdateCharacterWithVisibilityTransition(sharedWorldID, previousSelected, updatedSelected, stablePeerFrames)
			}
			return frames, true
		}
		commitSelectedRuntimeOnlyMutationFrames := func(selectedPlayer *player.Runtime, previousSelected loginticket.Character, frames [][]byte, stablePeerFrames [][]byte) ([][]byte, bool) {
			if selectedPlayer == nil {
				return nil, false
			}
			updatedSelected := selectedPlayer.LiveCharacter()
			if updatedSelected.ID == 0 {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return nil, false
			}
			refreshLiveCharacterRegistration()
			if ownsLiveSharedWorldSession() {
				sharedWorld.UpdateCharacterWithVisibilityTransition(sharedWorldID, previousSelected, updatedSelected, stablePeerFrames)
			}
			return frames, true
		}
		commitSelectedDeathFloorPersistenceFrames := func(selectedPlayer *player.Runtime, previousSelected loginticket.Character, frames [][]byte, stablePeerFrames [][]byte) ([][]byte, bool) {
			if selectedPlayer == nil {
				return nil, false
			}
			persistedSelected := selectedPlayer.PersistedSnapshot()
			updatedSelected := selectedPlayer.LiveCharacter()
			if persistedSelected.ID == 0 || updatedSelected.ID == 0 || previousSelected.ID == 0 {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return nil, false
			}
			if updatedSelected.Points[bootstrapPlayerPointValueIndex] > 0 {
				return commitSelectedRuntimeOnlyMutationFrames(selectedPlayer, previousSelected, frames, stablePeerFrames)
			}
			if !ownsLiveSharedWorldSession() {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return frames, true
			}
			persistedSelected.Points[bootstrapPlayerPointValueIndex] = updatedSelected.Points[bootstrapPlayerPointValueIndex]
			updatedCharacters, ok := selectedCharacterSnapshotUpdate(sessionTicket.Characters, selectedPlayer.SessionLink().CharacterIndex, persistedSelected)
			if ok && saveAccountSnapshot(accounts, sessionTicket.Login, sessionTicket.Empire, updatedCharacters) {
				sessionTicket.Characters = updatedCharacters
				selectedPlayer.SetPersistedSnapshot(persistedSelected)
			}
			// Live death/clear frames already advanced; keep them even if account persistence fails.
			refreshLiveCharacterRegistration()
			sharedWorld.UpdateCharacterWithVisibilityTransition(sharedWorldID, previousSelected, updatedSelected, stablePeerFrames)
			return frames, true
		}
		commitSelectedNonPointItemMutation := func(selectedPlayer *player.Runtime, previousSelected loginticket.Character, frames [][]byte) gameflow.ChatResult {
			frames, ok := commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
			if !ok {
				return gameflow.ChatResult{Accepted: false}
			}
			return gameflow.ChatResult{Accepted: true, Frames: frames}
		}
		flushPendingPracticeMobServerOriginRetaliation = func(pending *pendingServerFrames) {
			if !pendingPracticeMobServerOriginRetaliation {
				return
			}
			now := sessionNow()
			if now.Before(pendingPracticeMobServerOriginRetaliationAt) {
				return
			}
			targetVID := pendingPracticeMobServerOriginRetaliationTargetVID
			snapshotVersion := pendingPracticeMobServerOriginRetaliationSnapshotVersion
			clearPendingPracticeMobServerOriginRetaliation()
			if !ownsLiveSharedWorldSession() {
				issuedPracticeMobServerOriginRetaliationSnapshotVersion = 0
				return
			}
			selectedPlayer, ok := currentSelectedPlayer()
			if !ok {
				issuedPracticeMobServerOriginRetaliationSnapshotVersion = 0
				return
			}
			previousSelected := selectedPlayer.LiveCharacter()
			if previousSelected.ID == 0 {
				issuedPracticeMobServerOriginRetaliationSnapshotVersion = 0
				return
			}
			resolution := runtime.resolveStaticActorCombatTarget(sharedWorldID, targetVID)
			if !resolution.Accepted || resolution.SnapshotVersion == 0 || resolution.SnapshotVersion != snapshotVersion {
				issuedPracticeMobServerOriginRetaliationSnapshotVersion = 0
				return
			}
			if !sharedWorld.StaticActorCombatEngagedBySubject(resolution.Actor.EntityID, sharedWorldID) {
				issuedPracticeMobServerOriginRetaliationSnapshotVersion = 0
				return
			}
			retaliation, ok, clearTarget := contentPracticeMobRetaliationPointChange(runtime, selectedPlayer, resolution.Actor, false)
			if !ok {
				issuedPracticeMobServerOriginRetaliationSnapshotVersion = 0
				return
			}
			frames := [][]byte{encodePlayerPointChangeFrame(previousSelected.VID, retaliation)}
			var ownerRetaliationDamageInfo []byte
			if !clearTarget {
				ownerRetaliationDamageInfo = encodePracticeMobOwnerRetaliationDamageInfoFrame(previousSelected.VID, retaliation)
				frames = append(frames, ownerRetaliationDamageInfo)
			}
			var stablePeerFrames [][]byte
			if clearTarget {
				clearActiveCombatTarget()
				sharedWorld.ClearStaticActorCombatEngagement(resolution.Actor.EntityID, sharedWorldID)
				runtime.clearSpawnGroupChaseStep(resolution.Actor.EntityID)
				deadRaw := worldproto.EncodeDead(worldproto.DeadPacket{VID: previousSelected.VID})
				frames = append(frames, deadRaw)
				frames = append(frames, combatproto.EncodeServerClearTarget())
				stablePeerFrames = [][]byte{deadRaw}
			}
			frames, ok = commitSelectedDeathFloorPersistenceFrames(selectedPlayer, previousSelected, frames, stablePeerFrames)
			if !ok {
				issuedPracticeMobServerOriginRetaliationSnapshotVersion = 0
				return
			}
			frames = appendPostFloorContextCloseFrames(frames, clearTarget)
			if pending == nil {
				issuedPracticeMobServerOriginRetaliationSnapshotVersion = 0
				return
			}
			pending.Enqueue(frames)
			if !clearTarget && len(ownerRetaliationDamageInfo) != 0 && ownsLiveSharedWorldSession() {
				sharedWorld.EnqueueToVisibleSessions(sharedWorldID, selectedPlayer.LiveCharacter(), [][]byte{ownerRetaliationDamageInfo})
			}
			issuedPracticeMobServerOriginRetaliationSnapshotVersion = 0
			if !clearTarget {
				scheduleFirstPracticeMobServerOriginRetaliation(targetVID, snapshotVersion)
			}
		}
		activeMerchantBuyContextStillValid := func(packetShopFrames bool) (bool, [][]byte) {
			if !ownsLiveSharedWorldSession() || runtime == nil {
				return true, nil
			}
			resolution := runtime.resolveStaticActorInteraction(sharedWorldID, activeMerchantBuy.TargetVID)
			if !resolution.Accepted || resolution.Definition.Kind != interactionstore.KindShopPreview || !reflect.DeepEqual(activeMerchantBuy.Definition, resolution.Definition) {
				clearActiveMerchantBuy()
				if packetShopFrames {
					return false, [][]byte{shopproto.EncodeServerEnd()}
				}
				return false, nil
			}
			return true, nil
		}
		executeActiveMerchantBuy := func(selectedPlayer *player.Runtime, catalogSlot uint16, packetShopFrames bool) ([][]byte, bool) {
			if selectedPlayer == nil || selectedPlayerAtBootstrapHPFloor(selectedPlayer) || !hasActiveMerchantBuy || activeMerchantBuy.Definition.Kind != interactionstore.KindShopPreview || activeMerchantBuy.TargetVID == 0 {
				return nil, false
			}
			if ok, frames := activeMerchantBuyContextStillValid(packetShopFrames); !ok {
				if len(frames) != 0 {
					return frames, true
				}
				return nil, false
			}
			entry, ok := merchantCatalogEntryBySlot(activeMerchantBuy.Definition, catalogSlot)
			if !ok {
				frames, ok := merchantBuyFailureFrames(player.MerchantBuyFailureInvalid, packetShopFrames)
				if !ok {
					return nil, false
				}
				return frames, true
			}
			template, ok := runtime.itemTemplates[entry.ItemVnum]
			if !ok {
				return nil, false
			}
			if failure := selectedPlayer.ValidateMerchantBuy(template, entry.Count, entry.Price); failure != "" {
				frames, ok := merchantBuyFailureFrames(failure, packetShopFrames)
				if !ok {
					return nil, false
				}
				if failure == player.MerchantBuyFailureInvalid {
					if message, ok := runtimeTemplateBuyRejectText(template, selectedPlayer); ok {
						frames = append(frames, chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: message}))
					}
				}
				return frames, true
			}
			previousSelected := selectedPlayer.LiveCharacter()
			buyResult, ok := selectedPlayer.BuyMerchantItem(template, entry.Count, entry.Price)
			if !ok {
				return nil, false
			}
			frames, err := merchantBuyResultFrames(buyResult, runtime.itemTemplates)
			if err != nil {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return nil, false
			}
			if !ownsLiveSharedWorldSession() {
				return frames, true
			}
			frames, ok = commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
			if !ok {
				return nil, false
			}
			frames = prependExchangeCloseFrame(frames)
			return frames, true
		}
		executeActiveMerchantSell := func(selectedPlayer *player.Runtime, slot inventory.SlotIndex, count uint16, explicitCount bool, packetShopFrames bool) ([][]byte, bool) {
			if selectedPlayer == nil || selectedPlayerAtBootstrapHPFloor(selectedPlayer) || !hasActiveMerchantBuy || activeMerchantBuy.Definition.Kind != interactionstore.KindShopPreview || activeMerchantBuy.TargetVID == 0 {
				return nil, false
			}
			if exchangeDisplaysCarriedSlot(slot) {
				return nil, false
			}
			if ok, frames := activeMerchantBuyContextStillValid(packetShopFrames); !ok {
				if len(frames) != 0 {
					return frames, true
				}
				return nil, false
			}
			if explicitCount && count == 0 {
				frames, ok := merchantBuyFailureFrames(player.MerchantBuyFailureInvalid, packetShopFrames)
				if !ok {
					return nil, false
				}
				return frames, true
			}
			soldCount, ok := selectedPlayer.MerchantSellCount(slot, count)
			if !ok {
				frames, ok := merchantBuyFailureFrames(player.MerchantBuyFailureInvalid, packetShopFrames)
				if !ok {
					return nil, false
				}
				return frames, true
			}
			template, ok := merchantSellTemplateForSlot(runtime.itemTemplates, selectedPlayer, slot)
			if !ok || template.AntiGet || template.AntiDrop || template.AntiGive || template.AntiStack {
				frames, ok := merchantBuyFailureFrames(player.MerchantBuyFailureInvalid, packetShopFrames)
				if !ok {
					return nil, false
				}
				if message, ok := runtimeTemplateSellRejectText(template, selectedPlayer); ok {
					frames = append(frames, chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: message}))
				}
				return frames, true
			}
			if !selectedPlayer.CanUseTemplate(template) || template.AntiSell {
				frames, ok := merchantBuyFailureFrames(player.MerchantBuyFailureInvalid, packetShopFrames)
				if !ok {
					return nil, false
				}
				if message, ok := runtimeTemplateSellRejectText(template, selectedPlayer); ok {
					frames = append(frames, chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: message}))
				}
				return frames, true
			}
			credit, ok := player.MerchantSellCredit(template, soldCount)
			if !ok {
				frames, ok := merchantBuyFailureFrames(player.MerchantBuyFailureInvalid, packetShopFrames)
				if !ok {
					return nil, false
				}
				return frames, true
			}
			previousSelected := selectedPlayer.LiveCharacter()
			sellResult, ok := selectedPlayer.SellMerchantItemForCredit(slot, count, credit)
			if !ok {
				frames, ok := merchantBuyFailureFrames(player.MerchantBuyFailureInvalid, packetShopFrames)
				if !ok {
					return nil, false
				}
				return frames, true
			}
			frames, err := merchantSellResultFrames(selectedPlayer.LiveCharacter(), sellResult, runtime.itemTemplates)
			if err != nil {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return nil, false
			}
			if sellResult.ItemRemoved {
				quickslotFrames, ok := itemRemovalQuickslotSyncFrames(selectedPlayer, sellResult.Slot)
				if !ok {
					selectedPlayer.ApplyPersistedSnapshot(previousSelected)
					refreshLiveCharacterRegistration()
					return nil, false
				}
				if len(quickslotFrames) > 0 {
					frames = append(frames[:1], append(quickslotFrames, frames[1:]...)...)
				}
			}
			if !ownsLiveSharedWorldSession() {
				return frames, true
			}
			frames, ok = commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
			if !ok {
				return nil, false
			}
			frames = prependExchangeCloseFrame(frames)
			return frames, true
		}
		executeSelectedItemUse := func(position itemproto.Position, emitUseEcho bool) gameflow.ItemUseResult {
			selectedPlayer, ok := currentSelectedPlayer()
			if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
				return gameflow.ItemUseResult{Accepted: false}
			}
			if position.WindowType != itemproto.WindowInventory || position.Cell >= itemproto.InventoryMaxCell {
				return gameflow.ItemUseResult{Accepted: false}
			}
			slot := inventory.SlotIndex(position.Cell)
			if exchangeDisplaysCarriedSlot(slot) {
				return gameflow.ItemUseResult{Accepted: false}
			}
			previousSelected := selectedPlayer.LiveCharacter()
			template, ok := runtime.resolveRuntimeUseTemplate(selectedPlayer, slot)
			if !ok {
				return gameflow.ItemUseResult{Accepted: false}
			}
			useResult, ok := selectedPlayer.UseItem(slot, template)
			if !ok {
				if message, ok := selectedPlayer.UseItemRejectText(slot, template); ok {
					frames := [][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: message})}
					frames = prependMerchantCloseFrame(prependExchangeCloseFrame(frames))
					return gameflow.ItemUseResult{Accepted: true, Frames: frames}
				}
				return gameflow.ItemUseResult{Accepted: false}
			}
			frames, err := itemUseResultFrames(selectedPlayer.LiveCharacter(), useResult, runtime.itemTemplates, emitUseEcho)
			if err != nil {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return gameflow.ItemUseResult{Accepted: false}
			}
			if useResult.ItemRemoved {
				quickslotFrames, ok := itemRemovalQuickslotSyncFrames(selectedPlayer, useResult.Slot)
				if !ok {
					selectedPlayer.ApplyPersistedSnapshot(previousSelected)
					refreshLiveCharacterRegistration()
					return gameflow.ItemUseResult{Accepted: false}
				}
				if len(quickslotFrames) > 0 {
					insertAt := 2
					if emitUseEcho {
						insertAt = 3
					}
					frames = append(frames[:insertAt], append(quickslotFrames, frames[insertAt:]...)...)
				}
			}
			if !ownsLiveSharedWorldSession() {
				return gameflow.ItemUseResult{Accepted: true, Frames: frames}
			}
			frames, ok = commitSelectedPointBearingItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
			if !ok {
				return gameflow.ItemUseResult{Accepted: false}
			}
			frames = prependExchangeCloseFrame(frames)
			return gameflow.ItemUseResult{Accepted: true, Frames: frames}
		}
		executeSelectedItemUseToItem := func(source itemproto.Position, target itemproto.Position) gameflow.ItemUseToItemResult {
			if source.WindowType != itemproto.WindowInventory || target.WindowType != itemproto.WindowInventory {
				return gameflow.ItemUseToItemResult{Accepted: false}
			}
			if inventory.SlotIndex(source.Cell) >= inventory.CarriedInventorySlotCount || inventory.SlotIndex(target.Cell) >= inventory.CarriedInventorySlotCount {
				return gameflow.ItemUseToItemResult{Accepted: false}
			}
			selectedPlayer, ok := currentSelectedPlayer()
			if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
				return gameflow.ItemUseToItemResult{Accepted: false}
			}
			sourceSlot := inventory.SlotIndex(source.Cell)
			targetSlot := inventory.SlotIndex(target.Cell)
			if exchangeDisplaysCarriedSlot(sourceSlot) || exchangeDisplaysCarriedSlot(targetSlot) {
				return gameflow.ItemUseToItemResult{Accepted: false}
			}
			previousSelected := selectedPlayer.LiveCharacter()
			template, ok := runtime.resolveRuntimeItemTemplate(selectedPlayer, sourceSlot)
			if !ok || !template.Stackable || template.MaxCount == 0 {
				return gameflow.ItemUseToItemResult{Accepted: false}
			}
			moveResult, ok := selectedPlayer.UseItemOnItem(sourceSlot, targetSlot, template)
			if !ok {
				return gameflow.ItemUseToItemResult{Accepted: false}
			}
			if !moveResult.Changed {
				return gameflow.ItemUseToItemResult{Accepted: true}
			}
			frames, err := inventoryMoveResultFrames(moveResult, runtime.itemTemplates)
			if err != nil {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return gameflow.ItemUseToItemResult{Accepted: false}
			}
			if quickslotFrames, ok := itemUseToItemQuickslotSyncFrames(selectedPlayer, moveResult); !ok {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return gameflow.ItemUseToItemResult{Accepted: false}
			} else {
				frames = append(frames, quickslotFrames...)
			}
			frames, ok = commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
			if !ok {
				return gameflow.ItemUseToItemResult{Accepted: false}
			}
			frames = prependExchangeCloseFrame(frames)
			return gameflow.ItemUseToItemResult{Accepted: true, Frames: frames}
		}
		executeSelectedGoldDrop := func(amount uint32) ([][]byte, bool) {
			if amount == 0 {
				return nil, false
			}
			selectedPlayer, ok := currentSelectedPlayer()
			if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
				return nil, false
			}
			previousSelected := selectedPlayer.LiveCharacter()
			if previousSelected.ID == 0 || uint64(amount) > selectedPlayer.LiveGold() || selectedPlayer.LiveGold() > uint64(math.MaxInt32) {
				return nil, false
			}
			groundVID := bootstrapGroundItemVID(previousSelected, inventory.SlotIndex(amount%uint32(inventory.CarriedInventorySlotCount)))
			if groundVID == 0 {
				return nil, false
			}
			pickupRange := templatePickupRange(runtime, 1)
			if ownsLiveSharedWorldSession() && !sharedWorld.CanRegisterGroundGoldWithPickupRange(sharedWorldID, sessionTicket.Login, previousSelected, groundVID, amount, pickupRange) {
				return nil, false
			}
			selectedPlayer.SetLiveGold(selectedPlayer.LiveGold() - uint64(amount))
			updatedSelected := selectedPlayer.LiveCharacter()
			if updatedSelected.Gold > uint64(math.MaxInt32) {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return nil, false
			}
			frames := [][]byte{
				worldproto.EncodePlayerPointChange(worldproto.PlayerPointChangePacket{VID: previousSelected.VID, Type: bootstrapGoldPointType, Amount: -int32(amount), Value: int32(updatedSelected.Gold)}),
				itemproto.EncodeGroundAdd(itemproto.GroundAddPacket{VID: groundVID, Vnum: 1, X: previousSelected.X, Y: previousSelected.Y, Z: previousSelected.Z}),
				itemproto.EncodeOwnership(itemproto.OwnershipPacket{VID: groundVID, OwnerName: previousSelected.Name}),
			}
			frames, ok = commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
			if !ok {
				return nil, false
			}
			if ownsLiveSharedWorldSession() {
				sharedWorld.RegisterGroundGoldWithPickupRange(sharedWorldID, sessionTicket.Login, previousSelected, groundVID, amount, pickupRange)
			}
			return frames, true
		}
		executeSelectedItemDrop := func(cell uint16, count uint16) ([][]byte, bool) {
			slot := inventory.SlotIndex(cell)
			if slot >= inventory.CarriedInventorySlotCount {
				return nil, false
			}
			selectedPlayer, ok := currentSelectedPlayer()
			if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
				return nil, false
			}
			if exchangeDisplaysCarriedSlot(slot) {
				return nil, false
			}
			previousSelected := selectedPlayer.LiveCharacter()
			dropTemplate, hasDropTemplate := itemDropTemplateForSlot(runtime.itemTemplates, previousSelected, slot)
			if runtime.itemTemplatesAuthored && !hasDropTemplate {
				return nil, false
			}
			if hasDropTemplate {
				if message, ok := runtimeTemplateDropRejectText(dropTemplate, selectedPlayer); ok {
					frames := [][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: message})}
					frames = prependExchangeCloseFrame(frames)
					return frames, true
				}
				if !selectedPlayer.CanUseTemplate(dropTemplate) {
					return nil, false
				}
			}
			for _, item := range selectedPlayer.LiveInventory() {
				if item.Slot == slot && !item.Equipped {
					if count == 0 || count > item.Count {
						count = item.Count
					}
					break
				}
			}
			var result inventory.MoveResult
			if hasDropTemplate {
				result, ok = selectedPlayer.DropInventoryItemWithTemplate(slot, count, dropTemplate)
			} else {
				result, ok = selectedPlayer.DropInventoryItem(slot, count)
			}
			if !ok || !result.Changed {
				return nil, false
			}
			liveSharedWorld := ownsLiveSharedWorldSession()
			var droppedItem inventory.ItemInstance
			var groundVID uint32
			if liveSharedWorld {
				droppedItem, ok = droppedInventoryItem(previousSelected, result.From, count)
				if !ok {
					selectedPlayer.ApplyPersistedSnapshot(previousSelected)
					refreshLiveCharacterRegistration()
					return nil, false
				}
				groundVID = bootstrapGroundItemVID(previousSelected, result.From)
				if groundVID == 0 || !sharedWorld.CanRegisterGroundItem(sharedWorldID, sessionTicket.Login, previousSelected, groundVID, droppedItem) {
					selectedPlayer.ApplyPersistedSnapshot(previousSelected)
					refreshLiveCharacterRegistration()
					return nil, false
				}
			}
			frames, err := itemDropInventoryResultFramesWithTemplates(result, runtime.itemTemplates)
			if err == nil && liveSharedWorld {
				frames, err = itemDropResultFramesWithTemplates(previousSelected, result, droppedItem, runtime.itemTemplates)
			}
			if err != nil {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return nil, false
			}
			if !result.FromOccupied {
				quickslotFrames, ok := itemRemovalQuickslotSyncFrames(selectedPlayer, slot)
				if !ok {
					selectedPlayer.ApplyPersistedSnapshot(previousSelected)
					refreshLiveCharacterRegistration()
					return nil, false
				}
				if len(quickslotFrames) != 0 {
					frames = append(frames[:1], append(quickslotFrames, frames[1:]...)...)
				}
			}
			frames, ok = commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
			if !ok {
				return nil, false
			}
			frames = prependExchangeCloseFrame(frames)
			if ownsLiveSharedWorldSession() {
				sharedWorld.RegisterGroundItemWithPickupRange(sharedWorldID, sessionTicket.Login, previousSelected, groundVID, droppedItem, templatePickupRange(runtime, droppedItem.Vnum))
			}
			return frames, true
		}
		executeSelectedItemPickup := func(vid uint32) ([][]byte, bool) {
			if vid == 0 {
				return nil, false
			}
			selectedPlayer, ok := currentSelectedPlayer()
			if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
				return nil, false
			}
			previousSelected := selectedPlayer.LiveCharacter()
			pickup, ok := sharedWorld.GroundItemPickupFor(sharedWorldID, previousSelected, vid)
			if !ok {
				return nil, false
			}
			if pickup.GoldAmount != 0 {
				if pickup.OwnerID != 0 && pickup.OwnerID != sharedWorldID {
					if message, ok := runtimeTemplateGoldPeerPickupRejectText(runtime, pickup.Item); ok {
						frames := [][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: message})}
						frames = prependExchangeCloseFrame(frames)
						return frames, true
					}
				}
				if pickup.OwnerID != 0 && pickup.OwnerID != sharedWorldID && pickup.Owner.ID != 0 {
					ownerSelected := pickup.Owner
					if ownerSelected.Gold > uint64(math.MaxInt32)-uint64(pickup.GoldAmount) {
						return nil, false
					}
					ownerRuntime := player.NewRuntime(ownerSelected, player.SessionLink{Login: pickup.OwnerLogin})
					ownerRuntime.SetLiveGold(ownerRuntime.LiveGold() + uint64(pickup.GoldAmount))
					updatedOwner := ownerRuntime.LiveCharacter()
					if accounts == nil || pickup.OwnerLogin == "" {
						return nil, false
					}
					ownerAccount, err := accounts.Load(pickup.OwnerLogin)
					if err != nil {
						return nil, false
					}
					updatedCharacters, ok := selectedCharacterSnapshotByIDUpdate(ownerAccount.Characters, ownerSelected.ID, updatedOwner)
					if !ok || !saveAccountSnapshot(accounts, ownerAccount.Login, ownerAccount.Empire, updatedCharacters) {
						return nil, false
					}
					if !runtime.applyLiveCharacterPersistedSnapshot(ownerSelected.Name, updatedOwner) {
						return nil, false
					}
					sharedWorld.UpdateCharacterWithVisibilityTransition(pickup.OwnerID, ownerSelected, updatedOwner, nil)
					collectorGetFrame, err := encodeBootstrapItemGetFrameWithPartyArg(pickup.Item, itemproto.GetArgDeliveredToPartyMember, pickup.OwnerName)
					if err != nil {
						return nil, false
					}
					ownerGetFrame, err := encodeBootstrapItemGetFrameWithPartyArg(pickup.Item, itemproto.GetArgFromPartyMember, previousSelected.Name)
					if err != nil {
						return nil, false
					}
					collectorFrames := prependExchangeCloseFrame([][]byte{itemproto.EncodeGroundDel(itemproto.GroundDelPacket{VID: vid}), collectorGetFrame})
					if !sharedWorld.RemoveGroundItem(sharedWorldID, previousSelected, vid) {
						return nil, false
					}
					ownerFrames := [][]byte{
						worldproto.EncodePlayerPointChange(worldproto.PlayerPointChangePacket{VID: ownerSelected.VID, Type: bootstrapGoldPointType, Amount: int32(pickup.GoldAmount), Value: int32(updatedOwner.Gold)}),
						ownerGetFrame,
					}
					sharedWorld.EnqueueToEntity(pickup.OwnerID, ownerFrames)
					return collectorFrames, true
				}
				updatedGold, ok := selectedPlayer.AddLiveGold(uint64(pickup.GoldAmount))
				if !ok {
					return nil, false
				}
				getFrame, err := encodeBootstrapItemGetFrame(pickup.Item)
				if err != nil {
					return nil, false
				}
				frames := [][]byte{
					itemproto.EncodeGroundDel(itemproto.GroundDelPacket{VID: vid}),
					worldproto.EncodePlayerPointChange(worldproto.PlayerPointChangePacket{VID: previousSelected.VID, Type: bootstrapGoldPointType, Amount: int32(pickup.GoldAmount), Value: int32(updatedGold)}),
					getFrame,
				}
				frames, ok = commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
				if !ok {
					return nil, false
				}
				frames = prependExchangeCloseFrame(frames)
				if ownsLiveSharedWorldSession() && !sharedWorld.RemoveGroundItem(sharedWorldID, previousSelected, vid) {
					return nil, false
				}
				return frames, true
			}
			if pickup.OwnerID != 0 && pickup.OwnerID != sharedWorldID && pickup.Owner.ID != 0 {
				ownerSelected := pickup.Owner
				ownerRuntime := player.NewRuntime(ownerSelected, player.SessionLink{Login: pickup.OwnerLogin})
				pickupMaxCount := uint16(0)
				if runtime != nil {
					if template, ok := runtime.itemTemplates[pickup.Item.Vnum]; ok {
						if !itemcatalog.ValidTemplate(template) || template.Vnum != pickup.Item.Vnum {
							return nil, false
						}
						if template.AntiGet || template.AntiDrop || template.AntiGive || template.AntiSell || template.AntiStack || !ownerRuntime.CanUseTemplate(template) {
							frames := [][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: itemPickupRejectText(template)})}
							frames = prependExchangeCloseFrame(frames)
							return frames, true
						}
						if pickup.Item.Count > template.MaxCount {
							return nil, false
						}
						if template.Stackable {
							pickupMaxCount = template.MaxCount
						}
					} else if runtime.itemTemplatesAuthored {
						return nil, false
					}
				}
				pickupResult, ok := ownerRuntime.PickupGroundItem(pickup.Item, pickup.Item.Slot, pickupMaxCount)
				if !ok {
					collectorResult, collectorOK := selectedPlayer.PickupGroundItem(pickup.Item, pickup.Item.Slot, pickupMaxCount)
					if !collectorOK {
						frames := [][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: itemPickupInventoryFullInfoMessage})}
						frames = prependExchangeCloseFrame(frames)
						return frames, true
					}
					collectorItemFrames, collectorOK := encodeBootstrapGroundPickupInventoryFrames(collectorResult, runtime.itemTemplates)
					if !collectorOK {
						selectedPlayer.ApplyPersistedSnapshot(previousSelected)
						refreshLiveCharacterRegistration()
						return nil, false
					}
					collectorGetFrame, err := encodeBootstrapItemGetFrame(collectorResult.Item)
					if err != nil {
						selectedPlayer.ApplyPersistedSnapshot(previousSelected)
						refreshLiveCharacterRegistration()
						return nil, false
					}
					collectorFrames := append([][]byte{itemproto.EncodeGroundDel(itemproto.GroundDelPacket{VID: vid})}, collectorItemFrames...)
					collectorFrames = append(collectorFrames, collectorGetFrame)
					collectorFrames, collectorOK = commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, collectorFrames, nil)
					if !collectorOK {
						return nil, false
					}
					collectorFrames = prependExchangeCloseFrame(collectorFrames)
					if !sharedWorld.RemoveGroundItem(sharedWorldID, previousSelected, vid) {
						return nil, false
					}
					return collectorFrames, true
				}
				itemFrames, ok := encodeBootstrapGroundPickupInventoryFrames(pickupResult, runtime.itemTemplates)
				if !ok {
					return nil, false
				}
				updatedOwner := ownerRuntime.LiveCharacter()
				if accounts == nil {
					return nil, false
				}
				ownerLogin := pickup.OwnerLogin
				if ownerLogin == "" {
					return nil, false
				}
				ownerAccount, err := accounts.Load(ownerLogin)
				if err != nil {
					return nil, false
				}
				updatedCharacters, ok := selectedCharacterSnapshotByIDUpdate(ownerAccount.Characters, ownerSelected.ID, updatedOwner)
				if !ok || !saveAccountSnapshot(accounts, ownerAccount.Login, ownerAccount.Empire, updatedCharacters) {
					return nil, false
				}
				if !runtime.applyLiveCharacterPersistedSnapshot(ownerSelected.Name, updatedOwner) {
					return nil, false
				}
				sharedWorld.UpdateCharacterWithVisibilityTransition(pickup.OwnerID, ownerSelected, updatedOwner, nil)
				collectorGetFrame, err := encodeBootstrapItemGetFrameWithPartyArg(pickup.Item, itemproto.GetArgDeliveredToPartyMember, pickup.OwnerName)
				if err != nil {
					return nil, false
				}
				ownerGetFrame, err := encodeBootstrapItemGetFrameWithPartyArg(pickup.Item, itemproto.GetArgFromPartyMember, previousSelected.Name)
				if err != nil {
					return nil, false
				}
				collectorFrames := prependExchangeCloseFrame([][]byte{itemproto.EncodeGroundDel(itemproto.GroundDelPacket{VID: vid}), collectorGetFrame})
				if !sharedWorld.RemoveGroundItem(sharedWorldID, previousSelected, vid) {
					return nil, false
				}
				ownerFrames := append([][]byte(nil), itemFrames...)
				ownerFrames = append(ownerFrames, ownerGetFrame)
				sharedWorld.EnqueueToEntity(pickup.OwnerID, ownerFrames)
				return collectorFrames, true
			}
			pickupMaxCount := uint16(0)
			if runtime != nil {
				if template, ok := runtime.itemTemplates[pickup.Item.Vnum]; ok {
					if !itemcatalog.ValidTemplate(template) || template.Vnum != pickup.Item.Vnum {
						return nil, false
					}
					if template.AntiGet || template.AntiDrop || template.AntiGive || template.AntiSell || template.AntiStack || !selectedPlayer.CanUseTemplate(template) {
						frames := [][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: itemPickupRejectText(template)})}
						frames = prependExchangeCloseFrame(frames)
						return frames, true
					}
					if pickup.Item.Count > template.MaxCount {
						return nil, false
					}
					if template.Stackable {
						pickupMaxCount = template.MaxCount
					}
				} else if runtime.itemTemplatesAuthored {
					return nil, false
				}
			}
			pickupResult, ok := selectedPlayer.PickupGroundItem(pickup.Item, pickup.Item.Slot, pickupMaxCount)
			if !ok {
				frames := [][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: itemPickupInventoryFullInfoMessage})}
				frames = prependExchangeCloseFrame(frames)
				return frames, true
			}
			itemFrames, ok := encodeBootstrapGroundPickupInventoryFrames(pickupResult, runtime.itemTemplates)
			if !ok {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return nil, false
			}
			getFrame, err := encodeBootstrapItemGetFrame(pickupResult.Item)
			if err != nil {
				selectedPlayer.ApplyPersistedSnapshot(previousSelected)
				refreshLiveCharacterRegistration()
				return nil, false
			}
			frames := append([][]byte{itemproto.EncodeGroundDel(itemproto.GroundDelPacket{VID: vid})}, itemFrames...)
			frames = append(frames, getFrame)
			frames, ok = commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
			if !ok {
				return nil, false
			}
			frames = prependExchangeCloseFrame(frames)
			if ownsLiveSharedWorldSession() && !sharedWorld.RemoveGroundItem(sharedWorldID, previousSelected, vid) {
				return nil, false
			}
			return frames, true
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
					activeCharacterPosition = bootstrapCharacterPositionGeneral
					clearActiveMerchantBuy()
					clearActiveCombatTarget()
					selectedPlayer = nil
					clearLiveCharacterRegistration()

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
						clearLiveCharacterRegistration()
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
					activeCharacterPosition = bootstrapCharacterPositionGeneral
					clearActiveCombatTarget()
					refreshLiveCharacterRegistration()
					return worldentry.Result{
						Accepted:      true,
						Player:        selectedPlayer,
						MainCharacter: ticketMainCharacterPacket(selectedPlayer.LiveCharacter()),
						PlayerPoints:  ticketPlayerPointsPacket(selectedPlayer.PersistedSnapshot()),
					}
				},
				EnterGame: func(_ *player.Runtime) worldentry.EnterGameResult {
					runtime.flushReadyStaticActorRespawns()
					runtime.flushDueSpawnGroupReturnSteps()
					runtime.flushDueSpawnGroupHomewardSteps()
					runtime.flushDueSpawnGroupChaseSteps()
					runtime.flushProximitySpawnGroupAggroAcquisition()

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
					bootstrapFrames, err := worldentry.BuildBootstrapFramesWithTemplates(selected, runtime.itemTemplates)
					if err != nil {
						return worldentry.EnterGameResult{Rejected: true}
					}
					itemBootstrapFrames, err := buildSelectedItemBootstrapFrames(selected, runtime.itemTemplates)
					if err != nil {
						return worldentry.EnterGameResult{Rejected: true}
					}
					bootstrapFrames = append(bootstrapFrames, itemBootstrapFrames...)
					if characterAtBootstrapHPFloor(selected) {
						bootstrapFrames = append(bootstrapFrames, worldproto.EncodeDead(worldproto.DeadPacket{VID: selected.VID}))
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
						// EnterGame reclaim / Join can drop stale practice-mob engagement
						// without running the live session leave helper, so prune chase
						// deadlines that lost eligibility before encoding visibility.
						runtime.pruneSpawnGroupChaseStepSchedules()
						for _, peer := range existingPeers {
							trailingFrames = append(trailingFrames, encodePeerVisibilityBootstrapFramesWithTemplates(peer, runtime.itemTemplates)...)
						}
					}
					trailingFrames = append(trailingFrames, sharedWorld.VisibleStaticActorFrames(selected)...)
					trailingFrames = append(trailingFrames, sharedWorld.VisibleGroundItemFrames(selected)...)
					return worldentry.EnterGameResult{BootstrapFrames: bootstrapFrames, TrailingFrames: trailingFrames}
				},
			},
			Game: gameflow.Config{
				HandleMove: func(packet movep.MovePacket) gameflow.Result {
					stateMu.Lock()
					defer stateMu.Unlock()

					selectedPlayer, ok := currentSelectedPlayer()
					if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
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
						clearInvalidActiveCombatTargetAfterMovement()
						clearInvalidActiveMerchantBuyAfterMovement()
					}
					return gameflow.Result{Accepted: true, Replication: ack}
				},
				HandleSyncPosition: func(packet movep.SyncPositionPacket) gameflow.SyncPositionResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					selectedPlayer, ok := currentSelectedPlayer()
					if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
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
							clearInvalidActiveCombatTargetAfterMovement()
							clearInvalidActiveMerchantBuyAfterMovement()
						}
						return gameflow.SyncPositionResult{Accepted: true, Synchronization: ack}
					}
					return gameflow.SyncPositionResult{Accepted: false}
				},
				HandleChat: func(packet chatproto.ClientChatPacket) gameflow.ChatResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					if fromSlot, toSlot, ok := slashInventoryMoveCommand(packet.Message); ok {
						selectedPlayer, ok := currentSelectedPlayer()
						if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
							return gameflow.ChatResult{Accepted: false}
						}
						if exchangeDisplaysCarriedSlot(fromSlot) || exchangeDisplaysCarriedSlot(toSlot) {
							return gameflow.ChatResult{Accepted: false}
						}
						previousSelected := selectedPlayer.LiveCharacter()
						liveInventory := selectedPlayer.LiveInventory()
						if !runtime.authoredInventoryMoveSlotCountsFitTemplates(liveInventory, fromSlot, toSlot) {
							return gameflow.ChatResult{Accepted: false}
						}
						if !runtime.authoredIncompatibleInventorySwapTemplatesResolve(liveInventory, fromSlot, toSlot) {
							return gameflow.ChatResult{Accepted: false}
						}
						moveResult, ok := selectedPlayer.MoveInventoryItem(fromSlot, toSlot)
						if !ok {
							return gameflow.ChatResult{Accepted: false}
						}
						if !moveResult.Changed {
							return gameflow.ChatResult{Accepted: true}
						}
						frames, err := inventoryMoveResultFrames(moveResult, runtime.itemTemplates)
						if err != nil {
							selectedPlayer.ApplyPersistedSnapshot(previousSelected)
							refreshLiveCharacterRegistration()
							return gameflow.ChatResult{Accepted: false}
						}
						if quickslotFrames, ok := itemMoveQuickslotSyncFrames(selectedPlayer, moveResult); !ok {
							selectedPlayer.ApplyPersistedSnapshot(previousSelected)
							refreshLiveCharacterRegistration()
							return gameflow.ChatResult{Accepted: false}
						} else {
							frames = append(frames, quickslotFrames...)
						}
						chatResult := commitSelectedNonPointItemMutation(selectedPlayer, previousSelected, frames)
						if !chatResult.Accepted {
							return chatResult
						}
						chatResult.Frames = prependExchangeCloseFrame(chatResult.Frames)
						return chatResult
					}

					if fromSlot, equipSlot, ok := slashEquipItemCommand(packet.Message); ok {
						selectedPlayer, ok := currentSelectedPlayer()
						if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
							return gameflow.ChatResult{Accepted: false}
						}
						if exchangeDisplaysCarriedSlot(fromSlot) {
							return gameflow.ChatResult{Accepted: false}
						}
						previousSelected := selectedPlayer.LiveCharacter()
						template, requiresTemplate, ok := runtime.resolveRuntimeEquipTemplate(selectedPlayer, fromSlot, equipSlot)
						if !ok {
							return gameflow.ChatResult{Accepted: false}
						}
						if requiresTemplate && !runtimeTemplateAllowsEquip(template, selectedPlayer, equipSlot) {
							if message, ok := runtimeTemplateEquipRejectText(template, selectedPlayer, equipSlot); ok {
								frames := [][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: message})}
								frames = prependExchangeCloseFrame(frames)
								return gameflow.ChatResult{Accepted: true, Frames: frames}
							}
							return gameflow.ChatResult{Accepted: false}
						}
						if selectedPlayer.EquipmentSlotOccupied(equipSlot) {
							frames := [][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: itemEquipOccupiedWearSlotInfoMessage})}
							frames = prependExchangeCloseFrame(frames)
							return gameflow.ChatResult{Accepted: true, Frames: frames}
						}
						var equippedItem inventory.ItemInstance
						if requiresTemplate {
							equippedItem, ok = selectedPlayer.EquipItemWithTemplate(fromSlot, equipSlot, template)
						} else {
							equippedItem, ok = selectedPlayer.EquipItem(fromSlot, equipSlot)
						}
						if !ok {
							return gameflow.ChatResult{Accepted: false}
						}
						var pointChange *player.PointChangeResult
						if template.EquipEffect != nil {
							result, ok := selectedPlayer.ApplyEquipTemplateEffect(template, equipSlot)
							if !ok {
								selectedPlayer.ApplyPersistedSnapshot(previousSelected)
								refreshLiveCharacterRegistration()
								return gameflow.ChatResult{Accepted: false}
							}
							pointChange = &result
						}
						frames, err := equipResultFrames(selectedPlayer.LiveCharacter(), fromSlot, equippedItem, pointChange, runtime.itemTemplates)
						if err != nil {
							selectedPlayer.ApplyPersistedSnapshot(previousSelected)
							refreshLiveCharacterRegistration()
							return gameflow.ChatResult{Accepted: false}
						}
						if quickslotFrames, ok := itemRemovalQuickslotSyncFrames(selectedPlayer, fromSlot); !ok {
							selectedPlayer.ApplyPersistedSnapshot(previousSelected)
							refreshLiveCharacterRegistration()
							return gameflow.ChatResult{Accepted: false}
						} else {
							frames = append(frames, quickslotFrames...)
						}
						if !ownsLiveSharedWorldSession() {
							return gameflow.ChatResult{Accepted: true, Frames: frames}
						}
						stablePeerFrames := projectedAppearanceStablePeerFrames(selectedPlayer.LiveCharacter(), equippedItem.EquipSlot, runtime.itemTemplates)
						frames, ok = commitSelectedPointBearingItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
						if !ok {
							return gameflow.ChatResult{Accepted: false}
						}
						frames = prependExchangeCloseFrame(frames)
						if ownsLiveSharedWorldSession() {
							sharedWorld.EnqueueToVisibleSessions(sharedWorldID, selectedPlayer.LiveCharacter(), stablePeerFrames)
						}
						return gameflow.ChatResult{Accepted: true, Frames: frames}
					}

					if equipSlot, toSlot, ok := slashUnequipItemCommand(packet.Message); ok {
						selectedPlayer, ok := currentSelectedPlayer()
						if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
							return gameflow.ChatResult{Accepted: false}
						}
						if exchangeDisplaysCarriedSlot(toSlot) {
							return gameflow.ChatResult{Accepted: false}
						}
						previousSelected := selectedPlayer.LiveCharacter()
						template, hasUnequipTemplate, ok := runtime.resolveRuntimeUnequipTemplate(selectedPlayer, equipSlot)
						if !ok {
							return gameflow.ChatResult{Accepted: false}
						}
						var inventoryItem inventory.ItemInstance
						if hasUnequipTemplate && template.Irremovable {
							frames := [][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: itemUnequipRejectText(template)})}
							frames = prependExchangeCloseFrame(frames)
							return gameflow.ChatResult{Accepted: true, Frames: frames}
						}
						if hasUnequipTemplate {
							inventoryItem, ok = selectedPlayer.UnequipItemWithTemplate(equipSlot, toSlot, template)
						} else {
							inventoryItem, ok = selectedPlayer.UnequipItem(equipSlot, toSlot)
						}
						if !ok {
							return gameflow.ChatResult{Accepted: false}
						}
						var pointChange *player.PointChangeResult
						if hasUnequipTemplate && template.EquipEffect != nil {
							result, ok := selectedPlayer.RemoveEquipTemplateEffectFromItem(template, equipSlot, inventoryItem)
							if !ok {
								selectedPlayer.ApplyPersistedSnapshot(previousSelected)
								refreshLiveCharacterRegistration()
								return gameflow.ChatResult{Accepted: false}
							}
							pointChange = &result
						}
						frames, err := unequipResultFrames(selectedPlayer.LiveCharacter(), equipSlot, inventoryItem, pointChange, runtime.itemTemplates)
						if err != nil {
							selectedPlayer.ApplyPersistedSnapshot(previousSelected)
							refreshLiveCharacterRegistration()
							return gameflow.ChatResult{Accepted: false}
						}
						if !ownsLiveSharedWorldSession() {
							return gameflow.ChatResult{Accepted: true, Frames: frames}
						}
						stablePeerFrames := projectedAppearanceStablePeerFrames(selectedPlayer.LiveCharacter(), equipSlot, runtime.itemTemplates)
						frames, ok = commitSelectedPointBearingItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
						if !ok {
							return gameflow.ChatResult{Accepted: false}
						}
						frames = prependExchangeCloseFrame(frames)
						if ownsLiveSharedWorldSession() {
							sharedWorld.EnqueueToVisibleSessions(sharedWorldID, selectedPlayer.LiveCharacter(), stablePeerFrames)
						}
						return gameflow.ChatResult{Accepted: true, Frames: frames}
					}

					if packet.Type == chatproto.ChatTypeTalking {
						if catalogSlot, ok := slashShopBuyCommand(packet.Message); ok {
							selectedPlayer, ok := currentSelectedPlayer()
							if !ok {
								return gameflow.ChatResult{Accepted: false}
							}
							frames, ok := executeActiveMerchantBuy(selectedPlayer, catalogSlot, false)
							if !ok {
								return gameflow.ChatResult{Accepted: false}
							}
							return gameflow.ChatResult{Accepted: true, Frames: frames}
						}
						if slot, ok := slashUseItemCommand(packet.Message); ok {
							result := executeSelectedItemUse(itemproto.Position{WindowType: itemproto.WindowInventory, Cell: uint16(slot)}, false)
							if !result.Accepted {
								return gameflow.ChatResult{Accepted: false}
							}
							return gameflow.ChatResult{Accepted: true, Frames: result.Frames}
						}
						if size, sizeExplicit, ok := slashOpenSafeboxCommand(packet.Message); ok {
							selectedPlayer, selectedOK := currentSelectedPlayer()
							if !selectedOK || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
								return gameflow.ChatResult{Accepted: false}
							}
							if size < bootstrapSafeboxOpenMinSize || size > bootstrapSafeboxOpenMaxSize {
								// Recognized /open_safebox with an out-of-range or otherwise invalid
								// size must stay fail-closed: consume the slash so it does not fall
								// through as ordinary talking chat, emit no SAFEBOX_SIZE, and leave
								// the same-socket open/busy presentation flag untouched.
								return gameflow.ChatResult{Accepted: true}
							}
							if hasActiveSafeboxOpen && !sizeExplicit {
								size = activeSafeboxSize
							}
							clearPendingSafeboxPasswordChallenge()
							frames := openSafeboxPresentation(size)
							return gameflow.ChatResult{
								Accepted: true,
								Frames:   frames,
							}
						}
						if password, ok := slashSafeboxPasswordCommand(packet.Message); ok {
							selectedPlayer, selectedOK := currentSelectedPlayer()
							if !selectedOK || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
								return gameflow.ChatResult{Accepted: false}
							}
							if hasActiveSafeboxOpen {
								clearPendingSafeboxPasswordChallenge()
								delivery := chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, Message: safeboxAlreadyOpenInfoMessage}
								return gameflow.ChatResult{Accepted: true, Delivery: &delivery}
							}
							if !pendingSafeboxPasswordChallenge {
								// No pending warehouse challenge: consume fail-closed so the
								// slash does not fall through as ordinary talking chat.
								return gameflow.ChatResult{Accepted: true}
							}
							if password == "" || len(password) > safeboxstore.MaxPasswordLen {
								clearPendingSafeboxPasswordChallenge()
								delivery := chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, Message: safeboxPasswordWrongInfoMessage}
								return gameflow.ChatResult{Accepted: true, Delivery: &delivery}
							}
							size := pendingSafeboxPasswordSize
							if size < bootstrapSafeboxOpenMinSize || size > bootstrapSafeboxOpenMaxSize {
								size = bootstrapSafeboxOpenMinSize
							}
							want := durableSafeboxPassword(selectedPlayer)
							if password != want {
								clearPendingSafeboxPasswordChallenge()
								return gameflow.ChatResult{Accepted: true, Frames: [][]byte{itemproto.EncodeSafeboxWrongPassword()}}
							}
							clearPendingSafeboxPasswordChallenge()
							return gameflow.ChatResult{Accepted: true, Frames: openSafeboxPresentation(size)}
						}
						if oldPassword, newPassword, ok := slashSafeboxChangePasswordCommand(packet.Message); ok {
							selectedPlayer, selectedOK := currentSelectedPlayer()
							if !selectedOK || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
								return gameflow.ChatResult{Accepted: false}
							}
							rejectWrong := func() gameflow.ChatResult {
								delivery := chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, Message: safeboxPasswordWrongInfoMessage}
								return gameflow.ChatResult{Accepted: true, Delivery: &delivery}
							}
							// Change-password never opens/closes presentation and never clears a
							// pending ShowMeSafeboxPassword challenge; malformed / mismatch /
							// persist failure stay fail-closed with the same wrong-password chat.
							if oldPassword == "" || len(oldPassword) > safeboxstore.MaxPasswordLen ||
								newPassword == "" || len(newPassword) > safeboxstore.MaxPasswordLen {
								return rejectWrong()
							}
							if oldPassword != durableSafeboxPassword(selectedPlayer) {
								return rejectWrong()
							}
							if err := persistDurableSafeboxPassword(selectedPlayer, newPassword); err != nil {
								return rejectWrong()
							}
							delivery := chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, Message: safeboxPasswordChangedInfoMessage}
							return gameflow.ChatResult{Accepted: true, Delivery: &delivery}
						}
						if amount, ok := slashSafeboxMoneySaveCommand(packet.Message); ok {
							selectedPlayer, selectedOK := currentSelectedPlayer()
							if !selectedOK || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
								return gameflow.ChatResult{Accepted: false}
							}
							// Closed / pending-only / malformed / insufficient / overflow stay
							// fail-closed-consume: no ordinary talking-chat fallthrough and no frames.
							if !hasActiveSafeboxOpen || amount == 0 || amount > uint64(math.MaxInt32) {
								return gameflow.ChatResult{Accepted: true}
							}
							previousSelected := selectedPlayer.LiveCharacter()
							previousMoney := durableSafeboxMoney(selectedPlayer)
							if previousSelected.Gold < amount {
								return gameflow.ChatResult{Accepted: true}
							}
							nextMoney := previousMoney + int64(amount)
							if nextMoney < previousMoney || nextMoney > math.MaxInt32 {
								return gameflow.ChatResult{Accepted: true}
							}
							updatedGold, ok := selectedPlayer.DeductLiveGold(amount)
							if !ok {
								return gameflow.ChatResult{Accepted: true}
							}
							if err := persistDurableSafeboxMoney(selectedPlayer, nextMoney); err != nil {
								selectedPlayer.ApplyPersistedSnapshot(previousSelected)
								refreshLiveCharacterRegistration()
								return gameflow.ChatResult{Accepted: true}
							}
							frames := [][]byte{
								worldproto.EncodePlayerPointChange(worldproto.PlayerPointChangePacket{
									VID:    previousSelected.VID,
									Type:   bootstrapGoldPointType,
									Amount: -int32(amount),
									Value:  int32(updatedGold),
								}),
								itemproto.EncodeSafeboxMoneyChange(itemproto.SafeboxMoneyChangePacket{Money: int32(nextMoney)}),
							}
							frames, ok = commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
							if !ok {
								_ = persistDurableSafeboxMoney(selectedPlayer, previousMoney)
								return gameflow.ChatResult{Accepted: true}
							}
							return gameflow.ChatResult{Accepted: true, Frames: frames}
						}
						if amount, ok := slashSafeboxMoneyWithdrawCommand(packet.Message); ok {
							selectedPlayer, selectedOK := currentSelectedPlayer()
							if !selectedOK || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
								return gameflow.ChatResult{Accepted: false}
							}
							if !hasActiveSafeboxOpen || amount == 0 || amount > uint64(math.MaxInt32) {
								return gameflow.ChatResult{Accepted: true}
							}
							previousSelected := selectedPlayer.LiveCharacter()
							previousMoney := durableSafeboxMoney(selectedPlayer)
							if previousMoney < int64(amount) {
								return gameflow.ChatResult{Accepted: true}
							}
							updatedGold, ok := selectedPlayer.AddLiveGold(amount)
							if !ok {
								return gameflow.ChatResult{Accepted: true}
							}
							nextMoney := previousMoney - int64(amount)
							if err := persistDurableSafeboxMoney(selectedPlayer, nextMoney); err != nil {
								selectedPlayer.ApplyPersistedSnapshot(previousSelected)
								refreshLiveCharacterRegistration()
								return gameflow.ChatResult{Accepted: true}
							}
							frames := [][]byte{
								worldproto.EncodePlayerPointChange(worldproto.PlayerPointChangePacket{
									VID:    previousSelected.VID,
									Type:   bootstrapGoldPointType,
									Amount: int32(amount),
									Value:  int32(updatedGold),
								}),
								itemproto.EncodeSafeboxMoneyChange(itemproto.SafeboxMoneyChangePacket{Money: int32(nextMoney)}),
							}
							frames, ok = commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
							if !ok {
								_ = persistDurableSafeboxMoney(selectedPlayer, previousMoney)
								return gameflow.ChatResult{Accepted: true}
							}
							return gameflow.ChatResult{Accepted: true, Frames: frames}
						}
						if slashCloseSafeboxCommand(packet.Message) {
							if !hasActiveSafeboxOpen {
								// Already-closed close attempts stay fail-closed-consume: no
								// ordinary talking-chat fallthrough and no CloseSafebox echo.
								return gameflow.ChatResult{Accepted: true}
							}
							clearPendingSafeboxPasswordChallenge()
							setActiveSafeboxOpen(0, false)
							delivery := chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeCommand, Message: "CloseSafebox"}
							return gameflow.ChatResult{Accepted: true, Delivery: &delivery}
						}
					}

					if command, ok := slashGameCommand(packet.Message); ok {
						if packet.Type != chatproto.ChatTypeTalking {
							return gameflow.ChatResult{Accepted: false}
						}
						leaveSharedWorld := func() {
							if joinedSharedWorld && sharedWorldID != 0 {
								sharedWorld.Leave(sharedWorldID)
								joinedSharedWorld = false
								sharedWorldID = 0
							}
							clearPendingSafeboxPasswordChallenge()
							setActiveSafeboxOpen(0, false)
							clearActiveSafeboxItems()
							setActiveRefineDialog(refineDialogPresentation{}, false)
						}
						switch command {
						case "quit":
							quitFrames := prependMerchantCloseFrame(prependSafeboxCloseFrame(prependExchangeCloseFrame(nil)))
							leaveSharedWorld()
							hasSelected = false
							selectedPlayer = nil
							activeCharacterPosition = bootstrapCharacterPositionGeneral
							clearActiveCombatTarget()
							clearLiveCharacterRegistration()
							delivery := chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeCommand, Message: "quit"}
							return gameflow.ChatResult{Accepted: true, Frames: quitFrames, Delivery: &delivery}
						case "logout":
							logoutFrames := prependMerchantCloseFrame(prependSafeboxCloseFrame(prependExchangeCloseFrame(nil)))
							leaveSharedWorld()
							hasSelected = false
							selectedPlayer = nil
							activeCharacterPosition = bootstrapCharacterPositionGeneral
							clearActiveCombatTarget()
							clearLiveCharacterRegistration()
							return gameflow.ChatResult{Accepted: true, Frames: logoutFrames, NextPhase: session.PhaseClose}
						case "phase_select":
							phaseSelectFrames := prependMerchantCloseFrame(prependSafeboxCloseFrame(prependExchangeCloseFrame(nil)))
							leaveSharedWorld()
							hasSelected = false
							selectedPlayer = nil
							activeCharacterPosition = bootstrapCharacterPositionGeneral
							clearActiveCombatTarget()
							clearLiveCharacterRegistration()
							return gameflow.ChatResult{Accepted: true, Frames: phaseSelectFrames, NextPhase: session.PhaseSelect}
						case "restart_here":
							selectedPlayer, ok := currentSelectedPlayer()
							if !ok || !ownsLiveSharedWorldSession() || !selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
								return gameflow.ChatResult{Accepted: false}
							}
							previousSelected := selectedPlayer.LiveCharacter()
							if previousSelected.ID == 0 {
								return gameflow.ChatResult{Accepted: false}
							}
							restartedSelected := selectedPlayer.PersistedSnapshot()
							if restartedSelected.ID == 0 {
								return gameflow.ChatResult{Accepted: false}
							}
							recoveredHP := initialStatsForRace(restartedSelected.RaceNum).MaxHP
							if recoveredHP <= 0 {
								return gameflow.ChatResult{Accepted: false}
							}
							restartedSelected.Points[bootstrapPlayerPointValueIndex] = recoveredHP
							restartedSelected.MapIndex = previousSelected.MapIndex
							restartedSelected.X = previousSelected.X
							restartedSelected.Y = previousSelected.Y
							restartedSelected.Z = previousSelected.Z
							updatedCharacters, ok := selectedCharacterSnapshotUpdate(sessionTicket.Characters, selectedPlayer.SessionLink().CharacterIndex, restartedSelected)
							if !ok || !saveAccountSnapshot(accounts, sessionTicket.Login, sessionTicket.Empire, updatedCharacters) {
								return gameflow.ChatResult{Accepted: false}
							}
							rollbackPersistedRestartHere := func() {
								_ = saveAccountSnapshot(accounts, sessionTicket.Login, sessionTicket.Empire, sessionTicket.Characters)
							}
							runtime.flushReadyStaticActorRespawns()
							runtime.flushDueSpawnGroupReturnSteps()
							runtime.flushDueSpawnGroupHomewardSteps()
							runtime.flushDueSpawnGroupChaseSteps()
							runtime.flushProximitySpawnGroupAggroAcquisition()
							bootstrapFrames, err := worldentry.BuildBootstrapFramesWithTemplates(restartedSelected, runtime.itemTemplates)
							if err != nil {
								rollbackPersistedRestartHere()
								return gameflow.ChatResult{Accepted: false}
							}
							peerRefreshFrames := encodePeerVisibilityFramesWithTemplates(restartedSelected, runtime.itemTemplates)
							if len(peerRefreshFrames) == 0 {
								rollbackPersistedRestartHere()
								return gameflow.ChatResult{Accepted: false}
							}
							sessionTicket.Characters = updatedCharacters
							selectedPlayer.ApplyPersistedSnapshot(restartedSelected)
							refreshLiveCharacterRegistration()
							restartedLive := selectedPlayer.LiveCharacter()
							sharedWorld.UpdateCharacter(sharedWorldID, restartedLive)
							sharedWorld.EnqueueToVisibleSessions(sharedWorldID, restartedLive, [][]byte{encodeCharacterDeleteFrame(previousSelected)})
							sharedWorld.EnqueueToVisibleSessions(sharedWorldID, restartedLive, peerRefreshFrames)
							activeCharacterPosition = bootstrapCharacterPositionGeneral
							clearActiveCombatTarget()
							setActiveSafeboxOpen(0, false)
							setActiveRefineDialog(refineDialogPresentation{}, false)
							staticRefreshFrames := sharedWorld.VisibleStaticActorRefreshFrames(restartedLive)
							frames := append(append([][]byte(nil), bootstrapFrames...), staticRefreshFrames...)
							return gameflow.ChatResult{Accepted: true, Frames: frames}
						case "restart_town":
							selectedPlayer, ok := currentSelectedPlayer()
							if !ok || !ownsLiveSharedWorldSession() || !selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
								return gameflow.ChatResult{Accepted: false}
							}
							restartedSelected := selectedPlayer.PersistedSnapshot()
							if restartedSelected.ID == 0 {
								return gameflow.ChatResult{Accepted: false}
							}
							recoveredHP := initialStatsForRace(restartedSelected.RaceNum).MaxHP
							if recoveredHP <= 0 {
								return gameflow.ChatResult{Accepted: false}
							}
							restartedSelected.Points[bootstrapPlayerPointValueIndex] = recoveredHP
							restartEmpire := restartedSelected.Empire
							if restartEmpire == 0 {
								restartEmpire = ticketEmpire(sessionTicket)
							}
							restartedSelected.MapIndex, restartedSelected.X, restartedSelected.Y = legacyCreatePositionForEmpire(restartEmpire)
							restartedSelected.Z = 0
							updatedCharacters, ok := selectedCharacterSnapshotUpdate(sessionTicket.Characters, selectedPlayer.SessionLink().CharacterIndex, restartedSelected)
							if !ok || !saveAccountSnapshot(accounts, sessionTicket.Login, sessionTicket.Empire, updatedCharacters) {
								return gameflow.ChatResult{Accepted: false}
							}
							rollbackPersistedTownRestart := func() {
								_ = saveAccountSnapshot(accounts, sessionTicket.Login, sessionTicket.Empire, sessionTicket.Characters)
							}
							runtime.flushReadyStaticActorRespawns()
							runtime.flushDueSpawnGroupReturnSteps()
							runtime.flushDueSpawnGroupHomewardSteps()
							runtime.flushDueSpawnGroupChaseSteps()
							runtime.flushProximitySpawnGroupAggroAcquisition()
							bootstrapFrames, err := worldentry.BuildBootstrapFramesWithTemplates(restartedSelected, runtime.itemTemplates)
							if err != nil {
								rollbackPersistedTownRestart()
								return gameflow.ChatResult{Accepted: false}
							}
							_, transferFrames, ok := sharedWorld.TransferWithOriginFrames(sharedWorldID, restartedSelected)
							if !ok {
								rollbackPersistedTownRestart()
								return gameflow.ChatResult{Accepted: false}
							}
							sessionTicket.Characters = updatedCharacters
							selectedPlayer.ApplyPersistedSnapshot(restartedSelected)
							refreshLiveCharacterRegistration()
							activeCharacterPosition = bootstrapCharacterPositionGeneral
							clearActiveCombatTarget()
							setActiveSafeboxOpen(0, false)
							setActiveRefineDialog(refineDialogPresentation{}, false)
							frames := append(append([][]byte(nil), bootstrapFrames...), transferFrames...)
							return gameflow.ChatResult{Accepted: true, Frames: frames}
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
					if selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
						switch packet.Type {
						case chatproto.ChatTypeTalking, chatproto.ChatTypeParty, chatproto.ChatTypeGuild, chatproto.ChatTypeShout, chatproto.ChatTypeInfo:
							return gameflow.ChatResult{Accepted: false}
						}
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
				HandleItemUse: func(packet itemproto.ClientUsePacket) gameflow.ItemUseResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					return executeSelectedItemUse(packet.Position, true)
				},
				HandleItemUseToItem: func(packet itemproto.ClientUseToItemPacket) gameflow.ItemUseToItemResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					return executeSelectedItemUseToItem(packet.Source, packet.Target)
				},
				HandleItemRefine: func(packet itemproto.ClientRefinePacket) gameflow.ItemRefineResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					selectedPlayer, ok := currentSelectedPlayer()
					if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) || packet.Position >= uint8(inventory.CarriedInventorySlotCount) {
						return gameflow.ItemRefineResult{Accepted: false}
					}
					if packet.Type == 255 {
						if !hasActiveRefineDialog {
							return gameflow.ItemRefineResult{Accepted: false}
						}
						setActiveRefineDialog(refineDialogPresentation{}, false)
						return gameflow.ItemRefineResult{Accepted: true}
					}
					if hasActiveRefineDialog && packet.Position == activeRefineDialog.Pos && packet.Type == activeRefineDialog.Type {
						busy := hasActiveMerchantBuy || hasActiveSafeboxOpen || (ownsLiveSharedWorldSession() && sharedWorld != nil && sharedWorld.hasActiveExchange(sharedWorldID))
						if busy {
							return gameflow.ItemRefineResult{Accepted: false}
						}
						sourceTemplate, ok := runtime.resolveRuntimeItemTemplate(selectedPlayer, inventory.SlotIndex(packet.Position))
						if !ok {
							return gameflow.ItemRefineResult{Accepted: false}
						}
						resultTemplate, ok := runtime.itemTemplates[activeRefineDialog.RefineInfo.ResultVnum]
						if !ok || !itemcatalog.ValidTemplate(resultTemplate) {
							return gameflow.ItemRefineResult{Accepted: false}
						}
						previousSelected := selectedPlayer.LiveCharacter()
						switch activeRefineDialog.RefineInfo.Probability {
						case 100:
							result, ok := selectedPlayer.ApplyRefineSuccess(inventory.SlotIndex(packet.Position), packet.Type, activeRefineDialog.SourceID, activeRefineDialog.RefineInfo, sourceTemplate, resultTemplate)
							if !ok {
								return gameflow.ItemRefineResult{Accepted: false}
							}
							frames, err := refineSuccessResultFrames(previousSelected, result, runtime.itemTemplates, packet.Type)
							if err != nil {
								selectedPlayer.ApplyPersistedSnapshot(previousSelected)
								return gameflow.ItemRefineResult{Accepted: false}
							}
							var materialQuickslotFrames [][]byte
							for _, change := range result.MaterialChanges {
								if !change.ItemRemoved {
									continue
								}
								quickslotFrames, ok := itemRemovalQuickslotSyncFrames(selectedPlayer, change.Slot)
								if !ok {
									selectedPlayer.ApplyPersistedSnapshot(previousSelected)
									return gameflow.ItemRefineResult{Accepted: false}
								}
								materialQuickslotFrames = append(materialQuickslotFrames, quickslotFrames...)
							}
							if len(materialQuickslotFrames) > 0 {
								insertAt := len(result.MaterialChanges)
								frames = append(frames[:insertAt], append(materialQuickslotFrames, frames[insertAt:]...)...)
							}
							committed, ok := commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
							if !ok {
								return gameflow.ItemRefineResult{Accepted: false}
							}
							setActiveRefineDialog(refineDialogPresentation{}, false)
							return gameflow.ItemRefineResult{Accepted: true, Frames: committed}
						case 0:
							result, ok := selectedPlayer.ApplyRefineDestroyFailure(inventory.SlotIndex(packet.Position), packet.Type, activeRefineDialog.SourceID, activeRefineDialog.RefineInfo, sourceTemplate, resultTemplate)
							if !ok {
								return gameflow.ItemRefineResult{Accepted: false}
							}
							frames, err := refineDestroyFailureResultFrames(previousSelected, result, runtime.itemTemplates, packet.Type)
							if err != nil {
								selectedPlayer.ApplyPersistedSnapshot(previousSelected)
								return gameflow.ItemRefineResult{Accepted: false}
							}
							var materialQuickslotFrames [][]byte
							for _, change := range result.MaterialChanges {
								if !change.ItemRemoved {
									continue
								}
								quickslotFrames, ok := itemRemovalQuickslotSyncFrames(selectedPlayer, change.Slot)
								if !ok {
									selectedPlayer.ApplyPersistedSnapshot(previousSelected)
									return gameflow.ItemRefineResult{Accepted: false}
								}
								materialQuickslotFrames = append(materialQuickslotFrames, quickslotFrames...)
							}
							if len(materialQuickslotFrames) > 0 {
								insertAt := len(result.MaterialChanges)
								frames = append(frames[:insertAt], append(materialQuickslotFrames, frames[insertAt:]...)...)
							}
							sourceQuickslotFrames, ok := itemRemovalQuickslotSyncFrames(selectedPlayer, result.SourceSlot)
							if !ok {
								selectedPlayer.ApplyPersistedSnapshot(previousSelected)
								return gameflow.ItemRefineResult{Accepted: false}
							}
							if len(sourceQuickslotFrames) > 0 {
								insertAt := len(result.MaterialChanges) + len(materialQuickslotFrames) + 1
								frames = append(frames[:insertAt], append(sourceQuickslotFrames, frames[insertAt:]...)...)
							}
							committed, ok := commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
							if !ok {
								return gameflow.ItemRefineResult{Accepted: false}
							}
							setActiveRefineDialog(refineDialogPresentation{}, false)
							return gameflow.ItemRefineResult{Accepted: true, Frames: committed}
						default:
							if activeRefineDialog.RefineInfo.Probability < 1 || activeRefineDialog.RefineInfo.Probability > 99 {
								return gameflow.ItemRefineResult{Accepted: false}
							}
							roll, ok := takeRefineConfirmRoll()
							if !ok {
								return gameflow.ItemRefineResult{Accepted: false}
							}
							outcome, ok := selectedPlayer.ApplyRefineWithRoll(inventory.SlotIndex(packet.Position), packet.Type, activeRefineDialog.SourceID, activeRefineDialog.RefineInfo, sourceTemplate, resultTemplate, roll)
							if !ok {
								return gameflow.ItemRefineResult{Accepted: false}
							}
							if outcome.Succeeded {
								result := outcome.Success
								frames, err := refineSuccessResultFrames(previousSelected, result, runtime.itemTemplates, packet.Type)
								if err != nil {
									selectedPlayer.ApplyPersistedSnapshot(previousSelected)
									return gameflow.ItemRefineResult{Accepted: false}
								}
								var materialQuickslotFrames [][]byte
								for _, change := range result.MaterialChanges {
									if !change.ItemRemoved {
										continue
									}
									quickslotFrames, ok := itemRemovalQuickslotSyncFrames(selectedPlayer, change.Slot)
									if !ok {
										selectedPlayer.ApplyPersistedSnapshot(previousSelected)
										return gameflow.ItemRefineResult{Accepted: false}
									}
									materialQuickslotFrames = append(materialQuickslotFrames, quickslotFrames...)
								}
								if len(materialQuickslotFrames) > 0 {
									insertAt := len(result.MaterialChanges)
									frames = append(frames[:insertAt], append(materialQuickslotFrames, frames[insertAt:]...)...)
								}
								committed, ok := commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
								if !ok {
									return gameflow.ItemRefineResult{Accepted: false}
								}
								setActiveRefineDialog(refineDialogPresentation{}, false)
								return gameflow.ItemRefineResult{Accepted: true, Frames: committed}
							}
							if !outcome.Destroyed {
								selectedPlayer.ApplyPersistedSnapshot(previousSelected)
								return gameflow.ItemRefineResult{Accepted: false}
							}
							result := outcome.Destroy
							frames, err := refineDestroyFailureResultFrames(previousSelected, result, runtime.itemTemplates, packet.Type)
							if err != nil {
								selectedPlayer.ApplyPersistedSnapshot(previousSelected)
								return gameflow.ItemRefineResult{Accepted: false}
							}
							var materialQuickslotFrames [][]byte
							for _, change := range result.MaterialChanges {
								if !change.ItemRemoved {
									continue
								}
								quickslotFrames, ok := itemRemovalQuickslotSyncFrames(selectedPlayer, change.Slot)
								if !ok {
									selectedPlayer.ApplyPersistedSnapshot(previousSelected)
									return gameflow.ItemRefineResult{Accepted: false}
								}
								materialQuickslotFrames = append(materialQuickslotFrames, quickslotFrames...)
							}
							if len(materialQuickslotFrames) > 0 {
								insertAt := len(result.MaterialChanges)
								frames = append(frames[:insertAt], append(materialQuickslotFrames, frames[insertAt:]...)...)
							}
							sourceQuickslotFrames, ok := itemRemovalQuickslotSyncFrames(selectedPlayer, result.SourceSlot)
							if !ok {
								selectedPlayer.ApplyPersistedSnapshot(previousSelected)
								return gameflow.ItemRefineResult{Accepted: false}
							}
							if len(sourceQuickslotFrames) > 0 {
								insertAt := len(result.MaterialChanges) + len(materialQuickslotFrames) + 1
								frames = append(frames[:insertAt], append(sourceQuickslotFrames, frames[insertAt:]...)...)
							}
							committed, ok := commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
							if !ok {
								return gameflow.ItemRefineResult{Accepted: false}
							}
							setActiveRefineDialog(refineDialogPresentation{}, false)
							return gameflow.ItemRefineResult{Accepted: true, Frames: committed}
						}
					}
					template, ok := runtime.resolveRuntimeItemTemplate(selectedPlayer, inventory.SlotIndex(packet.Position))
					if !ok {
						return gameflow.ItemRefineResult{Accepted: false}
					}
					if message, ok := selectedPlayer.RefineRejectText(inventory.SlotIndex(packet.Position), template); ok {
						frames := [][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: message})}
						frames = prependMerchantCloseFrame(prependExchangeCloseFrame(frames))
						return gameflow.ItemRefineResult{Accepted: true, Frames: frames}
					}
					if info, ok := selectedPlayer.RefineInformation(inventory.SlotIndex(packet.Position), packet.Type, template); ok {
						frame, err := refineInformationFrame(info)
						if err != nil {
							return gameflow.ItemRefineResult{Accepted: false}
						}
						sourceItem, ok := refineDialogSourceItem(selectedPlayer, inventory.SlotIndex(packet.Position), info.SourceVnum)
						if !ok {
							return gameflow.ItemRefineResult{Accepted: false}
						}
						activeRefineDialogPresentation := refineDialogPresentation{
							Pos:        packet.Position,
							Type:       packet.Type,
							SourceID:   sourceItem.ID,
							SourceVnum: sourceItem.Vnum,
							Cell:       inventory.SlotIndex(packet.Position),
							RefineInfo: itemcatalog.RefineInfo{
								ResultVnum:  info.ResultVnum,
								Cost:        info.Cost,
								Probability: info.Probability,
								Materials:   append([]itemcatalog.RefineMaterial(nil), info.Materials...),
							},
						}
						setActiveRefineDialog(activeRefineDialogPresentation, true)
						frames := prependMerchantCloseFrame(prependExchangeCloseFrame([][]byte{frame}))
						return gameflow.ItemRefineResult{Accepted: true, Frames: frames}
					}
					return gameflow.ItemRefineResult{Accepted: false}
				},
				HandleSafeboxCheckin: func(packet itemproto.ClientSafeboxCheckinPacket) gameflow.SafeboxCheckinResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					selectedPlayer, ok := currentSelectedPlayer()
					if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) || packet.Position.WindowType != itemproto.WindowInventory || packet.Position.Cell >= itemproto.InventoryMaxCell {
						return gameflow.SafeboxCheckinResult{Accepted: false}
					}
					template, ok := runtime.resolveRuntimeItemTemplate(selectedPlayer, inventory.SlotIndex(packet.Position.Cell))
					if !ok {
						return gameflow.SafeboxCheckinResult{Accepted: false}
					}
					if message, ok := selectedPlayer.SafeboxCheckinRejectText(inventory.SlotIndex(packet.Position.Cell), template); ok {
						frames := [][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: message})}
						frames = prependMerchantCloseFrame(prependExchangeCloseFrame(frames))
						return gameflow.SafeboxCheckinResult{Accepted: true, Frames: frames}
					}
					if !hasActiveSafeboxOpen {
						return gameflow.SafeboxCheckinResult{Accepted: false}
					}
					capacity := bootstrapSafeboxCapacity(activeSafeboxSize)
					if capacity == 0 || packet.SafeSlot >= capacity {
						return gameflow.SafeboxCheckinResult{Accepted: false}
					}
					if _, occupied := activeSafeboxItems[packet.SafeSlot]; occupied {
						return gameflow.SafeboxCheckinResult{Accepted: false}
					}
					slot := inventory.SlotIndex(packet.Position.Cell)
					if exchangeDisplaysCarriedSlot(slot) {
						return gameflow.SafeboxCheckinResult{Accepted: false}
					}
					previousSelected := selectedPlayer.LiveCharacter()
					checkin, ok := selectedPlayer.SafeboxCheckinItem(slot, template)
					if !ok {
						return gameflow.SafeboxCheckinResult{Accepted: false}
					}
					safeboxFrame, err := encodeBootstrapSafeboxSetFrame(itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: uint16(packet.SafeSlot)}, checkin.Item, runtime.itemTemplates)
					if err != nil {
						selectedPlayer.ApplyPersistedSnapshot(previousSelected)
						refreshLiveCharacterRegistration()
						return gameflow.SafeboxCheckinResult{Accepted: false}
					}
					frames := [][]byte{itemproto.EncodeDel(itemproto.DelPacket{Position: itemproto.InventoryPosition(uint16(slot))})}
					quickslotFrames, ok := itemRemovalQuickslotSyncFrames(selectedPlayer, slot)
					if !ok {
						selectedPlayer.ApplyPersistedSnapshot(previousSelected)
						refreshLiveCharacterRegistration()
						return gameflow.SafeboxCheckinResult{Accepted: false}
					}
					frames = append(frames, quickslotFrames...)
					frames = append(frames, safeboxFrame)
					previousSafeboxItems := cloneActiveSafeboxItems()
					activeSafeboxItems[packet.SafeSlot] = checkin.Item
					if err := persistActiveSafeboxCells(selectedPlayer); err != nil {
						selectedPlayer.ApplyPersistedSnapshot(previousSelected)
						refreshLiveCharacterRegistration()
						activeSafeboxItems = previousSafeboxItems
						_ = persistActiveSafeboxCells(selectedPlayer)
						return gameflow.SafeboxCheckinResult{Accepted: false}
					}
					frames, ok = commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
					if !ok {
						activeSafeboxItems = previousSafeboxItems
						_ = persistActiveSafeboxCells(selectedPlayer)
						return gameflow.SafeboxCheckinResult{Accepted: false}
					}
					frames = prependMerchantCloseFrame(prependExchangeCloseFrame(frames))
					return gameflow.SafeboxCheckinResult{Accepted: true, Frames: frames}
				},
				HandleSafeboxCheckout: func(packet itemproto.ClientSafeboxCheckoutPacket) gameflow.SafeboxCheckoutResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					selectedPlayer, ok := currentSelectedPlayer()
					if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) || !hasActiveSafeboxOpen || packet.Position.WindowType != itemproto.WindowInventory || packet.Position.Cell >= itemproto.InventoryMaxCell {
						return gameflow.SafeboxCheckoutResult{Accepted: false}
					}
					capacity := bootstrapSafeboxCapacity(activeSafeboxSize)
					if capacity == 0 || packet.SafeSlot >= capacity {
						return gameflow.SafeboxCheckoutResult{Accepted: false}
					}
					safeboxItem, occupied := activeSafeboxItems[packet.SafeSlot]
					if !occupied {
						return gameflow.SafeboxCheckoutResult{Accepted: false}
					}
					template, ok := runtime.itemTemplates[safeboxItem.Vnum]
					if !ok || !itemcatalog.ValidTemplate(template) || safeboxItem.Vnum != template.Vnum || safeboxItem.Count == 0 || safeboxItem.Count > template.MaxCount {
						return gameflow.SafeboxCheckoutResult{Accepted: false}
					}
					destination := inventory.SlotIndex(packet.Position.Cell)
					if exchangeDisplaysCarriedSlot(destination) {
						return gameflow.SafeboxCheckoutResult{Accepted: false}
					}
					previousSelected := selectedPlayer.LiveCharacter()
					checkout, ok := selectedPlayer.SafeboxCheckoutItem(destination, safeboxItem, template)
					if !ok {
						return gameflow.SafeboxCheckoutResult{Accepted: false}
					}
					frames := [][]byte{itemproto.EncodeSafeboxDel(itemproto.DelPacket{Position: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: uint16(packet.SafeSlot)}})}
					if checkout.Merged {
						updateFrame, err := encodeInventoryItemUpdateFrameWithTemplates(checkout.Item, runtime.itemTemplates)
						if err != nil {
							selectedPlayer.ApplyPersistedSnapshot(previousSelected)
							refreshLiveCharacterRegistration()
							return gameflow.SafeboxCheckoutResult{Accepted: false}
						}
						frames = append(frames, updateFrame)
					} else {
						setFrame, err := encodeBootstrapItemFrameWithTemplates(itemproto.InventoryPosition(uint16(checkout.Item.Slot)), checkout.Item, runtime.itemTemplates)
						if err != nil {
							selectedPlayer.ApplyPersistedSnapshot(previousSelected)
							refreshLiveCharacterRegistration()
							return gameflow.SafeboxCheckoutResult{Accepted: false}
						}
						frames = append(frames, setFrame)
					}
					previousSafeboxItems := cloneActiveSafeboxItems()
					delete(activeSafeboxItems, packet.SafeSlot)
					if err := persistActiveSafeboxCells(selectedPlayer); err != nil {
						selectedPlayer.ApplyPersistedSnapshot(previousSelected)
						refreshLiveCharacterRegistration()
						activeSafeboxItems = previousSafeboxItems
						_ = persistActiveSafeboxCells(selectedPlayer)
						return gameflow.SafeboxCheckoutResult{Accepted: false}
					}
					frames, ok = commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
					if !ok {
						activeSafeboxItems = previousSafeboxItems
						_ = persistActiveSafeboxCells(selectedPlayer)
						return gameflow.SafeboxCheckoutResult{Accepted: false}
					}
					frames = prependMerchantCloseFrame(prependExchangeCloseFrame(frames))
					return gameflow.SafeboxCheckoutResult{Accepted: true, Frames: frames}
				},
				HandleSafeboxItemMove: func(packet itemproto.ClientSafeboxItemMovePacket) gameflow.SafeboxItemMoveResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					selectedPlayer, ok := currentSelectedPlayer()
					if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) || !hasActiveSafeboxOpen {
						return gameflow.SafeboxItemMoveResult{Accepted: false}
					}
					if !acceptedSafeboxItemMoveWindow(packet.Source.WindowType) || !acceptedSafeboxItemMoveWindow(packet.Destination.WindowType) {
						return gameflow.SafeboxItemMoveResult{Accepted: false}
					}
					if packet.Source.Cell > 0xff || packet.Destination.Cell > 0xff {
						return gameflow.SafeboxItemMoveResult{Accepted: false}
					}
					sourceSlot := uint8(packet.Source.Cell)
					destinationSlot := uint8(packet.Destination.Cell)
					if sourceSlot == destinationSlot {
						return gameflow.SafeboxItemMoveResult{Accepted: false}
					}
					capacity := bootstrapSafeboxCapacity(activeSafeboxSize)
					if capacity == 0 || sourceSlot >= capacity || destinationSlot >= capacity {
						return gameflow.SafeboxItemMoveResult{Accepted: false}
					}
					sourceItem, occupied := activeSafeboxItems[sourceSlot]
					if !occupied {
						return gameflow.SafeboxItemMoveResult{Accepted: false}
					}
					template, ok := runtime.itemTemplates[sourceItem.Vnum]
					if !ok || !itemcatalog.ValidTemplate(template) || sourceItem.Vnum != template.Vnum || sourceItem.Count == 0 || sourceItem.Count > template.MaxCount {
						return gameflow.SafeboxItemMoveResult{Accepted: false}
					}
					if sourceItem.Equipped || sourceItem.Locked {
						return gameflow.SafeboxItemMoveResult{Accepted: false}
					}
					if err := sourceItem.Validate(); err != nil {
						return gameflow.SafeboxItemMoveResult{Accepted: false}
					}
					moveCount := sourceItem.Count
					if packet.Count != 0 {
						moveCount = uint16(packet.Count)
					}
					if moveCount == 0 || moveCount > sourceItem.Count {
						return gameflow.SafeboxItemMoveResult{Accepted: false}
					}
					wholeStack := moveCount == sourceItem.Count

					destinationItem, destinationOccupied := activeSafeboxItems[destinationSlot]
					if wholeStack {
						var resultItem inventory.ItemInstance
						if !destinationOccupied {
							resultItem = sourceItem
							resultItem.Slot = inventory.SlotIndex(destinationSlot)
							if err := resultItem.Validate(); err != nil {
								return gameflow.SafeboxItemMoveResult{Accepted: false}
							}
						} else {
							if destinationItem.Equipped || destinationItem.Locked || destinationItem.Vnum != sourceItem.Vnum || destinationItem.Count == 0 || destinationItem.Count > template.MaxCount {
								return gameflow.SafeboxItemMoveResult{Accepted: false}
							}
							if err := destinationItem.Validate(); err != nil {
								return gameflow.SafeboxItemMoveResult{Accepted: false}
							}
							if uint32(destinationItem.Count)+uint32(sourceItem.Count) > uint32(template.MaxCount) {
								return gameflow.SafeboxItemMoveResult{Accepted: false}
							}
							resultItem = destinationItem
							resultItem.Count += sourceItem.Count
							if err := resultItem.Validate(); err != nil {
								return gameflow.SafeboxItemMoveResult{Accepted: false}
							}
						}

						setFrame, err := encodeBootstrapSafeboxSetFrame(itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: uint16(destinationSlot)}, resultItem, runtime.itemTemplates)
						if err != nil {
							return gameflow.SafeboxItemMoveResult{Accepted: false}
						}
						frames := [][]byte{
							itemproto.EncodeSafeboxDel(itemproto.DelPacket{Position: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: uint16(sourceSlot)}}),
							setFrame,
						}
						previousSafeboxItems := cloneActiveSafeboxItems()
						delete(activeSafeboxItems, sourceSlot)
						activeSafeboxItems[destinationSlot] = resultItem
						if err := persistActiveSafeboxCells(selectedPlayer); err != nil {
							activeSafeboxItems = previousSafeboxItems
							return gameflow.SafeboxItemMoveResult{Accepted: false}
						}
						frames = prependMerchantCloseFrame(prependExchangeCloseFrame(frames))
						return gameflow.SafeboxItemMoveResult{Accepted: true, Frames: frames}
					}

					sourceRemainder := sourceItem
					sourceRemainder.Count -= moveCount
					if err := sourceRemainder.Validate(); err != nil {
						return gameflow.SafeboxItemMoveResult{Accepted: false}
					}
					var resultItem inventory.ItemInstance
					if !destinationOccupied {
						nextID := nextSafeboxSplitItemID(selectedPlayer, activeSafeboxItems)
						if nextID == 0 {
							return gameflow.SafeboxItemMoveResult{Accepted: false}
						}
						resultItem = sourceItem
						resultItem.ID = nextID
						resultItem.Count = moveCount
						resultItem.Slot = inventory.SlotIndex(destinationSlot)
						if err := resultItem.Validate(); err != nil {
							return gameflow.SafeboxItemMoveResult{Accepted: false}
						}
					} else {
						if destinationItem.Equipped || destinationItem.Locked || destinationItem.Vnum != sourceItem.Vnum || destinationItem.Count == 0 || destinationItem.Count > template.MaxCount {
							return gameflow.SafeboxItemMoveResult{Accepted: false}
						}
						if err := destinationItem.Validate(); err != nil {
							return gameflow.SafeboxItemMoveResult{Accepted: false}
						}
						if uint32(destinationItem.Count)+uint32(moveCount) > uint32(template.MaxCount) {
							return gameflow.SafeboxItemMoveResult{Accepted: false}
						}
						resultItem = destinationItem
						resultItem.Count += moveCount
						if err := resultItem.Validate(); err != nil {
							return gameflow.SafeboxItemMoveResult{Accepted: false}
						}
					}

					sourceFrame, err := encodeBootstrapSafeboxSetFrame(itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: uint16(sourceSlot)}, sourceRemainder, runtime.itemTemplates)
					if err != nil {
						return gameflow.SafeboxItemMoveResult{Accepted: false}
					}
					destinationFrame, err := encodeBootstrapSafeboxSetFrame(itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: uint16(destinationSlot)}, resultItem, runtime.itemTemplates)
					if err != nil {
						return gameflow.SafeboxItemMoveResult{Accepted: false}
					}
					frames := [][]byte{sourceFrame, destinationFrame}
					previousSafeboxItems := cloneActiveSafeboxItems()
					activeSafeboxItems[sourceSlot] = sourceRemainder
					activeSafeboxItems[destinationSlot] = resultItem
					if err := persistActiveSafeboxCells(selectedPlayer); err != nil {
						activeSafeboxItems = previousSafeboxItems
						return gameflow.SafeboxItemMoveResult{Accepted: false}
					}
					frames = prependMerchantCloseFrame(prependExchangeCloseFrame(frames))
					return gameflow.SafeboxItemMoveResult{Accepted: true, Frames: frames}
				},
				HandleItemDrop: func(packet itemproto.ClientDropPacket) gameflow.ItemDropResult {
					stateMu.Lock()
					defer stateMu.Unlock()
					if packet.Elk != 0 {
						frames, accepted := executeSelectedGoldDrop(packet.Elk)
						return gameflow.ItemDropResult{Accepted: accepted, Frames: frames}
					}
					if packet.Position.WindowType != itemproto.WindowInventory {
						return gameflow.ItemDropResult{Accepted: false}
					}
					frames, accepted := executeSelectedItemDrop(packet.Position.Cell, 0)
					return gameflow.ItemDropResult{Accepted: accepted, Frames: frames}
				},
				HandleItemDrop2: func(packet itemproto.ClientDrop2Packet) gameflow.ItemDrop2Result {
					stateMu.Lock()
					defer stateMu.Unlock()
					if packet.Gold != 0 {
						frames, accepted := executeSelectedGoldDrop(packet.Gold)
						return gameflow.ItemDrop2Result{Accepted: accepted, Frames: frames}
					}
					if packet.Position.WindowType != itemproto.WindowInventory {
						return gameflow.ItemDrop2Result{Accepted: false}
					}
					frames, accepted := executeSelectedItemDrop(packet.Position.Cell, uint16(packet.Count))
					return gameflow.ItemDrop2Result{Accepted: accepted, Frames: frames}
				},
				HandleItemMove: func(packet itemproto.ClientMovePacket) gameflow.ItemMoveResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					if packet.Source.WindowType != itemproto.WindowInventory {
						return gameflow.ItemMoveResult{Accepted: false}
					}
					selectedPlayer, ok := currentSelectedPlayer()
					if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
						return gameflow.ItemMoveResult{Accepted: false}
					}
					previousSelected := selectedPlayer.LiveCharacter()
					if packet.Count != 0 && (packet.Source.Cell >= itemproto.InventoryMaxCell || packet.Destination.Cell >= itemproto.InventoryMaxCell) {
						return gameflow.ItemMoveResult{Accepted: false}
					}
					if packet.Source.Cell >= itemproto.InventoryMaxCell && packet.Destination.WindowType == itemproto.WindowInventory && inventory.SlotIndex(packet.Destination.Cell) < inventory.CarriedInventorySlotCount {
						equipWearCell := packet.Source.Cell - itemproto.InventoryMaxCell
						equipSlot, ok := equipmentBootstrapSlot(equipWearCell)
						if !ok {
							return gameflow.ItemMoveResult{Accepted: false}
						}
						if exchangeDisplaysCarriedSlot(inventory.SlotIndex(packet.Destination.Cell)) {
							return gameflow.ItemMoveResult{Accepted: false}
						}
						template, hasUnequipTemplate, ok := runtime.resolveRuntimeUnequipTemplate(selectedPlayer, equipSlot)
						if !ok {
							return gameflow.ItemMoveResult{Accepted: false}
						}
						var inventoryItem inventory.ItemInstance
						if hasUnequipTemplate && template.Irremovable {
							frames := [][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: itemUnequipRejectText(template)})}
							frames = prependExchangeCloseFrame(frames)
							return gameflow.ItemMoveResult{Accepted: true, Frames: frames}
						}
						if hasUnequipTemplate {
							inventoryItem, ok = selectedPlayer.UnequipItemWithTemplate(equipSlot, inventory.SlotIndex(packet.Destination.Cell), template)
						} else {
							inventoryItem, ok = selectedPlayer.UnequipItem(equipSlot, inventory.SlotIndex(packet.Destination.Cell))
						}
						if !ok {
							return gameflow.ItemMoveResult{Accepted: false}
						}
						var pointChange *player.PointChangeResult
						if hasUnequipTemplate && template.EquipEffect != nil {
							result, ok := selectedPlayer.RemoveEquipTemplateEffectFromItem(template, equipSlot, inventoryItem)
							if !ok {
								selectedPlayer.ApplyPersistedSnapshot(previousSelected)
								refreshLiveCharacterRegistration()
								return gameflow.ItemMoveResult{Accepted: false}
							}
							pointChange = &result
						}
						frames, err := unequipResultFrames(selectedPlayer.LiveCharacter(), equipSlot, inventoryItem, pointChange, runtime.itemTemplates)
						if err != nil {
							selectedPlayer.ApplyPersistedSnapshot(previousSelected)
							refreshLiveCharacterRegistration()
							return gameflow.ItemMoveResult{Accepted: false}
						}
						stablePeerFrames := projectedAppearanceStablePeerFrames(selectedPlayer.LiveCharacter(), equipSlot, runtime.itemTemplates)
						frames, ok = commitSelectedPointBearingItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
						if !ok {
							return gameflow.ItemMoveResult{Accepted: false}
						}
						frames = prependExchangeCloseFrame(frames)
						if ownsLiveSharedWorldSession() {
							sharedWorld.EnqueueToVisibleSessions(sharedWorldID, selectedPlayer.LiveCharacter(), stablePeerFrames)
						}
						return gameflow.ItemMoveResult{Accepted: true, Frames: frames}
					}
					if inventory.SlotIndex(packet.Source.Cell) >= inventory.CarriedInventorySlotCount {
						return gameflow.ItemMoveResult{Accepted: false}
					}
					if exchangeDisplaysCarriedSlot(inventory.SlotIndex(packet.Source.Cell)) {
						return gameflow.ItemMoveResult{Accepted: false}
					}
					if packet.Destination.WindowType == itemproto.WindowInventory && packet.Destination.Cell >= itemproto.InventoryMaxCell {
						equipWearCell := packet.Destination.Cell - itemproto.InventoryMaxCell
						equipSlot, ok := equipmentBootstrapSlot(equipWearCell)
						if !ok {
							return gameflow.ItemMoveResult{Accepted: false}
						}
						template, requiresTemplate, ok := runtime.resolveRuntimeEquipTemplate(selectedPlayer, inventory.SlotIndex(packet.Source.Cell), equipSlot)
						if !ok {
							return gameflow.ItemMoveResult{Accepted: false}
						}
						if requiresTemplate && !runtimeTemplateAllowsEquip(template, selectedPlayer, equipSlot) {
							if message, ok := runtimeTemplateEquipRejectText(template, selectedPlayer, equipSlot); ok {
								frames := [][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: message})}
								frames = prependExchangeCloseFrame(frames)
								return gameflow.ItemMoveResult{Accepted: true, Frames: frames}
							}
							return gameflow.ItemMoveResult{Accepted: false}
						}
						if selectedPlayer.EquipmentSlotOccupied(equipSlot) {
							frames := [][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: itemEquipOccupiedWearSlotInfoMessage})}
							frames = prependExchangeCloseFrame(frames)
							return gameflow.ItemMoveResult{Accepted: true, Frames: frames}
						}
						fromSlot := inventory.SlotIndex(packet.Source.Cell)
						var equippedItem inventory.ItemInstance
						if requiresTemplate {
							equippedItem, ok = selectedPlayer.EquipItemWithTemplate(fromSlot, equipSlot, template)
						} else {
							equippedItem, ok = selectedPlayer.EquipItem(fromSlot, equipSlot)
						}
						if !ok {
							return gameflow.ItemMoveResult{Accepted: false}
						}
						var pointChange *player.PointChangeResult
						if requiresTemplate && template.EquipEffect != nil {
							result, ok := selectedPlayer.ApplyEquipTemplateEffect(template, equipSlot)
							if !ok {
								selectedPlayer.ApplyPersistedSnapshot(previousSelected)
								refreshLiveCharacterRegistration()
								return gameflow.ItemMoveResult{Accepted: false}
							}
							pointChange = &result
						}
						frames, err := equipResultFrames(selectedPlayer.LiveCharacter(), fromSlot, equippedItem, pointChange, runtime.itemTemplates)
						if err != nil {
							selectedPlayer.ApplyPersistedSnapshot(previousSelected)
							refreshLiveCharacterRegistration()
							return gameflow.ItemMoveResult{Accepted: false}
						}
						if quickslotFrames, ok := itemRemovalQuickslotSyncFrames(selectedPlayer, fromSlot); !ok {
							selectedPlayer.ApplyPersistedSnapshot(previousSelected)
							refreshLiveCharacterRegistration()
							return gameflow.ItemMoveResult{Accepted: false}
						} else {
							frames = append(frames, quickslotFrames...)
						}
						stablePeerFrames := projectedAppearanceStablePeerFrames(selectedPlayer.LiveCharacter(), equippedItem.EquipSlot, runtime.itemTemplates)
						frames, ok = commitSelectedPointBearingItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
						if !ok {
							return gameflow.ItemMoveResult{Accepted: false}
						}
						frames = prependExchangeCloseFrame(frames)
						if ownsLiveSharedWorldSession() {
							sharedWorld.EnqueueToVisibleSessions(sharedWorldID, selectedPlayer.LiveCharacter(), stablePeerFrames)
						}
						return gameflow.ItemMoveResult{Accepted: true, Frames: frames}
					}
					if packet.Destination.WindowType != itemproto.WindowInventory || inventory.SlotIndex(packet.Destination.Cell) >= inventory.CarriedInventorySlotCount {
						return gameflow.ItemMoveResult{Accepted: false}
					}
					fromSlot := inventory.SlotIndex(packet.Source.Cell)
					toSlot := inventory.SlotIndex(packet.Destination.Cell)
					if exchangeDisplaysCarriedSlot(toSlot) {
						return gameflow.ItemMoveResult{Accepted: false}
					}
					var moveResult inventory.MoveResult
					maxCount := ^uint16(0)
					forceSameVnumSwap := false
					liveInventory := selectedPlayer.LiveInventory()
					if !runtime.authoredInventoryMoveSlotCountsFitTemplates(liveInventory, fromSlot, toSlot) {
						return gameflow.ItemMoveResult{Accepted: false}
					}
					if !runtime.authoredIncompatibleInventorySwapTemplatesResolve(liveInventory, fromSlot, toSlot) {
						return gameflow.ItemMoveResult{Accepted: false}
					}
					for _, sourceItem := range liveInventory {
						if sourceItem.Slot != fromSlot {
							continue
						}
						if template, ok := runtime.itemTemplates[sourceItem.Vnum]; ok {
							if !itemcatalog.ValidTemplate(template) {
								for _, targetItem := range liveInventory {
									if targetItem.Slot == toSlot && targetItem.Vnum == sourceItem.Vnum {
										maxCount = 0
										break
									}
								}
							} else if template.AntiStack {
								maxCount = 0
								forceSameVnumSwap = true
							} else if !template.Stackable {
								maxCount = 0
								forceSameVnumSwap = true
							} else if template.AntiGet || template.AntiDrop || template.AntiGive || template.AntiSell || template.EquipSlot != "" || !selectedPlayer.CanUseTemplate(template) {
								maxCount = 0
							} else if template.MaxCount > 0 {
								maxCount = template.MaxCount
							}
						} else if runtime.itemTemplatesAuthored {
							for _, targetItem := range liveInventory {
								if targetItem.Slot == toSlot && targetItem.Vnum == sourceItem.Vnum {
									maxCount = 0
									break
								}
							}
						}
						break
					}
					if maxCount == 0 && !forceSameVnumSwap {
						return gameflow.ItemMoveResult{Accepted: false}
					}
					if packet.Count == 0 {
						moveResult, ok = selectedPlayer.MoveInventoryItemBounded(fromSlot, toSlot, maxCount)
					} else {
						moveCount := uint16(packet.Count)
						moveResult, ok = selectedPlayer.MoveInventoryItemCountBounded(fromSlot, toSlot, moveCount, maxCount)
					}
					if !ok {
						return gameflow.ItemMoveResult{Accepted: false}
					}
					if !moveResult.Changed {
						return gameflow.ItemMoveResult{Accepted: true}
					}
					frames, err := inventoryMoveResultFrames(moveResult, runtime.itemTemplates)
					if err != nil {
						selectedPlayer.ApplyPersistedSnapshot(previousSelected)
						refreshLiveCharacterRegistration()
						return gameflow.ItemMoveResult{Accepted: false}
					}
					if quickslotFrames, ok := itemMoveQuickslotSyncFrames(selectedPlayer, moveResult); !ok {
						selectedPlayer.ApplyPersistedSnapshot(previousSelected)
						refreshLiveCharacterRegistration()
						return gameflow.ItemMoveResult{Accepted: false}
					} else {
						frames = append(frames, quickslotFrames...)
					}
					chatResult := commitSelectedNonPointItemMutation(selectedPlayer, previousSelected, frames)
					if !chatResult.Accepted {
						return gameflow.ItemMoveResult{Accepted: false, Frames: chatResult.Frames}
					}
					chatResult.Frames = prependExchangeCloseFrame(chatResult.Frames)
					return gameflow.ItemMoveResult{Accepted: true, Frames: chatResult.Frames}
				},
				HandleItemPickup: func(packet itemproto.ClientPickupPacket) gameflow.ItemPickupResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					frames, accepted := executeSelectedItemPickup(packet.VID)
					return gameflow.ItemPickupResult{Accepted: accepted, Frames: frames}
				},
				HandleItemGive: func(packet itemproto.ClientGivePacket) gameflow.ItemGiveResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					selectedPlayer, ok := currentSelectedPlayer()
					if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) || packet.Position.WindowType != itemproto.WindowInventory || packet.Position.Cell >= itemproto.InventoryMaxCell {
						return gameflow.ItemGiveResult{Accepted: false}
					}
					if !ownsLiveSharedWorldSession() || !sharedWorld.HasVisiblePlayerTarget(sharedWorldID, packet.TargetVID) {
						return gameflow.ItemGiveResult{Accepted: false}
					}
					template, ok := runtime.resolveRuntimeItemTemplate(selectedPlayer, inventory.SlotIndex(packet.Position.Cell))
					if !ok {
						return gameflow.ItemGiveResult{Accepted: false}
					}
					if message, ok := selectedPlayer.GiveRejectText(inventory.SlotIndex(packet.Position.Cell), uint16(packet.Count), template); ok {
						frames := [][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: message})}
						frames = prependMerchantCloseFrame(prependExchangeCloseFrame(frames))
						return gameflow.ItemGiveResult{Accepted: true, Frames: frames}
					}
					return gameflow.ItemGiveResult{Accepted: false}
				},
				HandleItemExchange: func(packet itemproto.ClientExchangePacket) gameflow.ItemExchangeResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					switch packet.Subheader {
					case itemproto.ExchangeSubheaderStart:
						if !ownsLiveSharedWorldSession() {
							return gameflow.ItemExchangeResult{Accepted: false}
						}
						if hasActiveMerchantBuy || hasActiveSafeboxOpen || hasActiveRefineDialog {
							return gameflow.ItemExchangeResult{
								Accepted: true,
								Frames: [][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
									Type:    chatproto.ChatTypeInfo,
									VID:     0,
									Empire:  0,
									Message: exchangeRequesterMerchantBusyInfoMessage,
								})},
							}
						}
						frames, ok := sharedWorld.StartExchange(sharedWorldID, packet.Arg1)
						if !ok {
							return gameflow.ItemExchangeResult{Accepted: false}
						}
						return gameflow.ItemExchangeResult{Accepted: true, Frames: frames}
					case itemproto.ExchangeSubheaderCancel:
						if !ownsLiveSharedWorldSession() {
							return gameflow.ItemExchangeResult{Accepted: false}
						}
						frames, ok := sharedWorld.CancelExchange(sharedWorldID)
						if !ok {
							return gameflow.ItemExchangeResult{Accepted: false}
						}
						return gameflow.ItemExchangeResult{Accepted: true, Frames: frames}
					case itemproto.ExchangeSubheaderItemDel:
						if !ownsLiveSharedWorldSession() || packet.Arg1 >= uint32(itemproto.ExchangeItemMaxNum) {
							return gameflow.ItemExchangeResult{Accepted: false}
						}
						frames, ok := sharedWorld.RemoveExchangeItem(sharedWorldID, uint8(packet.Arg1))
						if !ok {
							return gameflow.ItemExchangeResult{Accepted: false}
						}
						return gameflow.ItemExchangeResult{Accepted: true, Frames: frames}
					case itemproto.ExchangeSubheaderGoldAdd:
						selectedPlayer, ok := currentSelectedPlayer()
						if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) || !ownsLiveSharedWorldSession() {
							return gameflow.ItemExchangeResult{Accepted: false}
						}
						frames, ok := sharedWorld.AddExchangeGold(sharedWorldID, packet.Arg1, selectedPlayer.LiveGold())
						if !ok {
							return gameflow.ItemExchangeResult{Accepted: false}
						}
						return gameflow.ItemExchangeResult{Accepted: true, Frames: frames}
					case itemproto.ExchangeSubheaderAccept:
						selectedPlayer, ok := currentSelectedPlayer()
						if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) || !ownsLiveSharedWorldSession() {
							return gameflow.ItemExchangeResult{Accepted: false}
						}
						// Same-socket busy presentations already published into shared-world
						// START eligibility must also fail closed for ACCEPT / mutual-accept
						// finalize so a later-opened merchant/safebox/refine window cannot
						// sneak past the frozen busy-window trade policy. Mirror START's
						// requester busy info-chat so the reject is client-visible.
						if hasActiveMerchantBuy || hasActiveSafeboxOpen || hasActiveRefineDialog {
							return gameflow.ItemExchangeResult{
								Accepted: true,
								Frames: [][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
									Type:    chatproto.ChatTypeInfo,
									VID:     0,
									Empire:  0,
									Message: exchangeRequesterMerchantBusyInfoMessage,
								})},
							}
						}
						frames, finalizePlan, ok := sharedWorld.AcceptExchange(sharedWorldID, selectedPlayer.LiveGold(), selectedPlayer.LiveCharacter())
						if !ok {
							return gameflow.ItemExchangeResult{Accepted: false}
						}
						if finalizePlan == nil {
							return gameflow.ItemExchangeResult{Accepted: true, Frames: frames}
						}
						selfFrames, applied := applyExchangeFinalize(runtime, accounts, sharedWorld, selectedPlayer, &sessionTicket, finalizePlan)
						if !applied {
							// Commit-time busy-window or Check/Space drift returns
							// reject info-chat while leaving the shell open.
							if len(selfFrames) > 0 {
								return gameflow.ItemExchangeResult{Accepted: true, Frames: selfFrames}
							}
							return gameflow.ItemExchangeResult{Accepted: false}
						}
						refreshLiveCharacterRegistration()
						return gameflow.ItemExchangeResult{Accepted: true, Frames: selfFrames}
					}

					if packet.Subheader != itemproto.ExchangeSubheaderItemAdd || packet.Arg2 >= itemproto.ExchangeItemMaxNum || packet.Position.WindowType != itemproto.WindowInventory || packet.Position.Cell >= itemproto.InventoryMaxCell {
						return gameflow.ItemExchangeResult{Accepted: false}
					}
					selectedPlayer, ok := currentSelectedPlayer()
					if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) || !ownsLiveSharedWorldSession() || !sharedWorld.hasActiveExchange(sharedWorldID) {
						return gameflow.ItemExchangeResult{Accepted: false}
					}
					template, ok := runtime.resolveRuntimeItemTemplate(selectedPlayer, inventory.SlotIndex(packet.Position.Cell))
					if !ok {
						return gameflow.ItemExchangeResult{Accepted: false}
					}
					if message, ok := selectedPlayer.ExchangeItemAddRejectText(inventory.SlotIndex(packet.Position.Cell), template); ok {
						return gameflow.ItemExchangeResult{Accepted: true, Frames: [][]byte{chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: message})}}
					}
					display, ok := selectedPlayer.ExchangeItemAddDisplay(inventory.SlotIndex(packet.Position.Cell), template)
					if !ok {
						return gameflow.ItemExchangeResult{Accepted: false}
					}
					frames, ok := sharedWorld.AddExchangeItem(sharedWorldID, packet.Arg2, display)
					if !ok {
						return gameflow.ItemExchangeResult{Accepted: false}
					}
					return gameflow.ItemExchangeResult{Accepted: true, Frames: frames}
				},
				HandleQuickslotAdd: func(packet quickslotproto.ClientAddPacket) gameflow.QuickslotResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					selectedPlayer, ok := currentSelectedPlayer()
					if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
						return gameflow.QuickslotResult{Accepted: false}
					}
					previousSelected := selectedPlayer.LiveCharacter()
					previousQuickslots := selectedPlayer.LiveQuickslots()
					quickslot := loginticket.Quickslot{Type: packet.Slot.Type, Slot: packet.Slot.Position}
					var result loginticket.Quickslot
					if packet.Slot.Type == quickslotproto.TypeItem {
						if packet.Slot.Position >= uint8(inventory.CarriedInventorySlotCount) {
							return gameflow.QuickslotResult{Accepted: false}
						}
						template, resolved := runtime.resolveRuntimeItemTemplate(selectedPlayer, inventory.SlotIndex(packet.Slot.Position))
						if resolved {
							result, ok = selectedPlayer.SetQuickslotWithTemplate(packet.Position, quickslot, template)
						} else if runtime.itemTemplatesAuthored {
							return gameflow.QuickslotResult{Accepted: false}
						} else {
							result, ok = selectedPlayer.SetQuickslot(packet.Position, quickslot)
						}
					} else {
						result, ok = selectedPlayer.SetQuickslot(packet.Position, quickslot)
					}
					if !ok {
						return gameflow.QuickslotResult{Accepted: false}
					}
					frames := make([][]byte, 0, len(previousQuickslots)+1)
					if packet.Slot.Type == quickslotproto.TypeNone {
						frames = append(frames, quickslotproto.EncodeDel(quickslotproto.DelPacket{Position: result.Position}))
					} else {
						for _, quickslot := range previousQuickslots {
							if quickslot.Type == packet.Slot.Type && quickslot.Slot == packet.Slot.Position && quickslot.Position != packet.Position {
								frames = append(frames, quickslotproto.EncodeDel(quickslotproto.DelPacket{Position: quickslot.Position}))
							}
						}
						frames = append(frames, quickslotproto.EncodeAdd(quickslotproto.AddPacket{Position: result.Position, Slot: quickslotproto.Slot{Type: result.Type, Position: result.Slot}}))
					}
					frames, ok = commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
					if !ok {
						return gameflow.QuickslotResult{Accepted: false}
					}
					frames = prependExchangeCloseFrame(frames)
					return gameflow.QuickslotResult{Accepted: true, Frames: frames}
				},
				HandleQuickslotDel: func(packet quickslotproto.ClientDelPacket) gameflow.QuickslotResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					selectedPlayer, ok := currentSelectedPlayer()
					if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
						return gameflow.QuickslotResult{Accepted: false}
					}
					previousSelected := selectedPlayer.LiveCharacter()
					result, ok := selectedPlayer.DeleteQuickslot(packet.Position)
					if !ok {
						return gameflow.QuickslotResult{Accepted: false}
					}
					frames := [][]byte{quickslotproto.EncodeDel(quickslotproto.DelPacket{Position: result.Position})}
					frames, ok = commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
					if !ok {
						return gameflow.QuickslotResult{Accepted: false}
					}
					frames = prependExchangeCloseFrame(frames)
					return gameflow.QuickslotResult{Accepted: true, Frames: frames}
				},
				HandleQuickslotSwap: func(packet quickslotproto.ClientSwapPacket) gameflow.QuickslotResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					selectedPlayer, ok := currentSelectedPlayer()
					if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
						return gameflow.QuickslotResult{Accepted: false}
					}
					previousSelected := selectedPlayer.LiveCharacter()
					result, ok := selectedPlayer.SwapQuickslots(packet.Position, packet.TargetPosition)
					if !ok {
						return gameflow.QuickslotResult{Accepted: false}
					}
					frames := [][]byte{quickslotproto.EncodeSwap(quickslotproto.SwapPacket{Position: result.Position, TargetPosition: result.TargetPosition})}
					frames, ok = commitSelectedNonPointItemMutationFrames(selectedPlayer, previousSelected, frames, nil)
					if !ok {
						return gameflow.QuickslotResult{Accepted: false}
					}
					frames = prependExchangeCloseFrame(frames)
					return gameflow.QuickslotResult{Accepted: true, Frames: frames}
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
					if selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
						return gameflow.WhisperResult{Accepted: false}
					}
					if packet.Target == selected.Name {
						return gameflow.WhisperResult{Accepted: true}
					}
					if !ownsLiveSharedWorldSession() {
						return gameflow.WhisperResult{Accepted: true}
					}
					delivery := ticketWhisperDeliveryPacket(selected, packet)
					delivered, missing := sharedWorld.EnqueueToCharacterName(packet.Target, [][]byte{chatproto.EncodeServerWhisper(delivery)})
					if delivered {
						return gameflow.WhisperResult{Accepted: true}
					}
					if !missing {
						return gameflow.WhisperResult{Accepted: false}
					}
					notFound := ticketWhisperNotExistPacket(packet.Target)
					return gameflow.WhisperResult{Accepted: true, Delivery: &notFound}
				},
				HandleCharacterPosition: func(packet combatproto.ClientCharacterPositionPacket) gameflow.CharacterPositionResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					if packet.Position != bootstrapCharacterPositionGeneral && packet.Position != bootstrapCharacterPositionSittingGround {
						switch packet.Position {
						case bootstrapCharacterPositionSittingChair:
							packet.Position = bootstrapCharacterPositionSittingGround
						default:
							return gameflow.CharacterPositionResult{Accepted: false}
						}
					}
					if !ownsLiveSharedWorldSession() {
						return gameflow.CharacterPositionResult{Accepted: false}
					}
					selectedPlayer, ok := currentSelectedPlayer()
					if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
						return gameflow.CharacterPositionResult{Accepted: false}
					}
					selected := selectedPlayer.LiveCharacter()
					if selected.ID == 0 || selected.VID == 0 {
						return gameflow.CharacterPositionResult{Accepted: false}
					}
					if activeCharacterPosition == packet.Position {
						return gameflow.CharacterPositionResult{Accepted: true}
					}
					activeCharacterPosition = packet.Position

					frame := worldproto.EncodeCharacterPosition(worldproto.CharacterPositionPacket{VID: selected.VID, Position: packet.Position})
					if ownsLiveSharedWorldSession() {
						sharedWorld.EnqueueToVisibleSessions(sharedWorldID, selected, [][]byte{frame})
					}
					return gameflow.CharacterPositionResult{Accepted: true, Frames: [][]byte{frame}}
				},
				HandleInteraction: func(packet interactproto.RequestPacket) gameflow.InteractionResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					if !ownsLiveSharedWorldSession() {
						return gameflow.InteractionResult{Accepted: false}
					}
					selectedPlayer, ok := currentSelectedPlayer()
					if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
						return gameflow.InteractionResult{Accepted: false}
					}
					if interactionOnCooldown(packet.TargetVID) {
						return gameflow.InteractionResult{Accepted: true}
					}
					resolution := runtime.resolveStaticActorInteraction(sharedWorldID, packet.TargetVID)
					if !resolution.Accepted {
						if resolution.Delivery == nil {
							clearActiveMerchantBuy()
							return gameflow.InteractionResult{Accepted: false}
						}
						markInteractionCooldown(packet.TargetVID)
						frames := prependMerchantCloseFrame([][]byte{chatproto.EncodeChatDelivery(*resolution.Delivery)})
						return gameflow.InteractionResult{Accepted: true, Frames: frames}
					}
					if resolution.Definition.Kind == interactionstore.KindWarp {
						_, transferFrames, ok := applySelectedCharacterTransfer(resolution.Definition.MapIndex, resolution.Definition.X, resolution.Definition.Y, true)
						if !ok {
							failureDelivery := staticActorInteractionFailureDelivery(staticActorInteractionFailureWarpNotApplied)
							if failureDelivery == nil {
								return gameflow.InteractionResult{Accepted: false}
							}
							markInteractionCooldown(packet.TargetVID)
							return gameflow.InteractionResult{Accepted: true, Frames: [][]byte{chatproto.EncodeChatDelivery(*failureDelivery)}}
						}
						frames := make([][]byte, 0, len(transferFrames)+1)
						if resolution.Delivery != nil {
							frames = append(frames, chatproto.EncodeChatDelivery(*resolution.Delivery))
						}
						frames = append(frames, transferFrames...)
						markInteractionCooldown(packet.TargetVID)
						return gameflow.InteractionResult{Accepted: true, Frames: frames}
					}
					if resolution.Definition.Kind == interactionstore.KindQuestFlag {
						if resolution.Delivery == nil {
							return gameflow.InteractionResult{Accepted: false}
						}
						selected := selectedPlayer.LiveCharacter()
						if selected.ID == 0 {
							return gameflow.InteractionResult{Accepted: false}
						}
						rewardGold := resolution.Definition.RewardGold
						if rewardGold > interactionstore.QuestFlagRewardGoldMax {
							return gameflow.InteractionResult{Accepted: false}
						}
						rewardExperience := resolution.Definition.RewardExperience
						if rewardExperience > interactionstore.QuestFlagRewardExperienceMax {
							return gameflow.InteractionResult{Accepted: false}
						}
						consumeGold := resolution.Definition.ConsumeGold
						if consumeGold > interactionstore.QuestFlagConsumeGoldMax {
							return gameflow.InteractionResult{Accepted: false}
						}
						consumeExperience := resolution.Definition.ConsumeExperience
						if consumeExperience > interactionstore.QuestFlagConsumeExperienceMax {
							return gameflow.InteractionResult{Accepted: false}
						}
						rewardItems := interactionstore.EffectiveRewardItems(resolution.Definition)
						rewardItemTemplates := make([]itemcatalog.Template, 0, len(rewardItems))
						for _, entry := range rewardItems {
							template, ok := runtime.itemTemplates[entry.ItemVnum]
							if !ok {
								return gameflow.InteractionResult{Accepted: false}
							}
							rewardItemTemplates = append(rewardItemTemplates, template)
						}
						consumeRequirements := questFlagConsumeRequirements(resolution.Definition)
						for _, requirement := range consumeRequirements {
							if _, ok := runtime.itemTemplates[requirement.ItemVnum]; !ok {
								return gameflow.InteractionResult{Accepted: false}
							}
						}
						previousSelected := selected
						if consumeGold != 0 {
							if previousSelected.Gold > uint64(math.MaxInt32) || previousSelected.Gold < consumeGold {
								failureDelivery := staticActorInteractionFailureDelivery(staticActorInteractionFailureQuestInsufficientGold)
								if failureDelivery == nil {
									return gameflow.InteractionResult{Accepted: false}
								}
								markInteractionCooldown(packet.TargetVID)
								frames := prependMerchantCloseFrame([][]byte{chatproto.EncodeChatDelivery(*failureDelivery)})
								return gameflow.InteractionResult{Accepted: true, Frames: frames}
							}
						}
						experienceBefore := previousSelected.Points[bootstrapExperiencePointType]
						if consumeExperience != 0 {
							if experienceBefore < 0 || uint64(experienceBefore) < consumeExperience {
								failureDelivery := staticActorInteractionFailureDelivery(staticActorInteractionFailureQuestInsufficientExperience)
								if failureDelivery == nil {
									return gameflow.InteractionResult{Accepted: false}
								}
								markInteractionCooldown(packet.TargetVID)
								frames := prependMerchantCloseFrame([][]byte{chatproto.EncodeChatDelivery(*failureDelivery)})
								return gameflow.InteractionResult{Accepted: true, Frames: frames}
							}
						}
						if rewardGold != 0 {
							goldAfterConsume := previousSelected.Gold - consumeGold
							if goldAfterConsume > uint64(math.MaxInt32) || goldAfterConsume > uint64(math.MaxInt32)-rewardGold {
								failureDelivery := staticActorInteractionFailureDelivery(staticActorInteractionFailureQuestRewardGoldOverflow)
								if failureDelivery == nil {
									return gameflow.InteractionResult{Accepted: false}
								}
								markInteractionCooldown(packet.TargetVID)
								frames := prependMerchantCloseFrame([][]byte{chatproto.EncodeChatDelivery(*failureDelivery)})
								return gameflow.InteractionResult{Accepted: true, Frames: frames}
							}
						}
						if rewardExperience != 0 {
							if rewardExperience > uint64(math.MaxInt32) {
								return gameflow.InteractionResult{Accepted: false}
							}
							experienceAfterConsume := int64(experienceBefore) - int64(consumeExperience)
							nextExperience := experienceAfterConsume + int64(rewardExperience)
							if nextExperience > math.MaxInt32 {
								failureDelivery := staticActorInteractionFailureDelivery(staticActorInteractionFailureQuestRewardExperienceOverflow)
								if failureDelivery == nil {
									return gameflow.InteractionResult{Accepted: false}
								}
								markInteractionCooldown(packet.TargetVID)
								frames := prependMerchantCloseFrame([][]byte{chatproto.EncodeChatDelivery(*failureDelivery)})
								return gameflow.InteractionResult{Accepted: true, Frames: frames}
							}
						}
						if len(consumeRequirements) > 0 {
							if failure := selectedPlayer.ValidateCarriedItemConsume(consumeRequirements); failure != "" {
								if failure != player.CarriedItemConsumeFailureInsufficientMaterials {
									return gameflow.InteractionResult{Accepted: false}
								}
								failureDelivery := staticActorInteractionFailureDelivery(staticActorInteractionFailureQuestInsufficientMaterials)
								if failureDelivery == nil {
									return gameflow.InteractionResult{Accepted: false}
								}
								markInteractionCooldown(packet.TargetVID)
								frames := prependMerchantCloseFrame([][]byte{chatproto.EncodeChatDelivery(*failureDelivery)})
								return gameflow.InteractionResult{Accepted: true, Frames: frames}
							}
						}
						if len(rewardItems) > 0 || len(consumeRequirements) > 0 {
							scratch := player.NewRuntime(previousSelected, selectedPlayer.SessionLink())
							if len(consumeRequirements) > 0 {
								if _, ok := scratch.ConsumeCarriedItems(consumeRequirements); !ok {
									return gameflow.InteractionResult{Accepted: false}
								}
							}
							for i, entry := range rewardItems {
								failure := scratch.ValidateCarriedItemGrant(rewardItemTemplates[i], entry.Count)
								if failure != "" {
									var failureDelivery *chatproto.ChatDeliveryPacket
									switch failure {
									case player.CarriedItemGrantFailureNoValidPlacement:
										failureDelivery = staticActorInteractionFailureDelivery(staticActorInteractionFailureQuestRewardInventoryFull)
									case player.CarriedItemGrantFailureInvalid:
										failureDelivery = questFlagRewardRestrictedDelivery(rewardItemTemplates[i])
									default:
										return gameflow.InteractionResult{Accepted: false}
									}
									if failureDelivery == nil {
										return gameflow.InteractionResult{Accepted: false}
									}
									markInteractionCooldown(packet.TargetVID)
									frames := prependMerchantCloseFrame([][]byte{chatproto.EncodeChatDelivery(*failureDelivery)})
									return gameflow.InteractionResult{Accepted: true, Frames: frames}
								}
								if _, ok := scratch.GrantCarriedItem(rewardItemTemplates[i], entry.Count); !ok {
									return gameflow.InteractionResult{Accepted: false}
								}
							}
						}
						transitionResult, err := runtime.ApplyQuestStateTransition(queststate.Transition{Character: selected.Name, QuestRef: resolution.Definition.QuestRef, Flag: resolution.Definition.QuestFlag, From: resolution.Definition.QuestFrom, To: resolution.Definition.QuestTo})
						if err != nil {
							return gameflow.InteractionResult{Accepted: false}
						}
						if !transitionResult.Result.Applied {
							if transitionResult.Result.Reason != queststate.TransitionReasonCurrentValueMismatch {
								return gameflow.InteractionResult{Accepted: false}
							}
							failureDelivery := staticActorInteractionFailureDelivery(staticActorInteractionFailureQuestCurrentValueMismatch)
							if failureDelivery == nil {
								return gameflow.InteractionResult{Accepted: false}
							}
							markInteractionCooldown(packet.TargetVID)
							frames := prependMerchantCloseFrame([][]byte{chatproto.EncodeChatDelivery(*failureDelivery)})
							return gameflow.InteractionResult{Accepted: true, Frames: frames}
						}
						frames := prependMerchantCloseFrame([][]byte{chatproto.EncodeChatDelivery(*resolution.Delivery)})
						rollbackQuestFlagRewards := func() {
							selectedPlayer.ApplyPersistedSnapshot(previousSelected)
							refreshLiveCharacterRegistration()
							_, _ = runtime.ApplyQuestStateTransition(queststate.Transition{Character: previousSelected.Name, QuestRef: resolution.Definition.QuestRef, Flag: resolution.Definition.QuestFlag, From: resolution.Definition.QuestTo, To: resolution.Definition.QuestFrom})
						}
						var consumeGoldAfter uint64
						if consumeGold != 0 {
							updatedGold, ok := selectedPlayer.DeductLiveGold(consumeGold)
							if !ok {
								rollbackQuestFlagRewards()
								return gameflow.InteractionResult{Accepted: false}
							}
							consumeGoldAfter = updatedGold
						}
						var consumeExperienceAfter int32
						if consumeExperience != 0 {
							updatedExperience, ok := selectedPlayer.DeductLiveExperience(consumeExperience)
							if !ok {
								rollbackQuestFlagRewards()
								return gameflow.InteractionResult{Accepted: false}
							}
							consumeExperienceAfter = updatedExperience
						}
						var scalarReward player.DeathRewardResult
						if rewardGold != 0 || rewardExperience != 0 {
							reward, rewardOK := selectedPlayer.ApplyStaticActorDeathReward(worldruntime.StaticActorDeathReward{Experience: rewardExperience, Gold: rewardGold})
							if !rewardOK || reward.GoldAfter > uint64(math.MaxInt32) || reward.GoldAfter < reward.GoldBefore || reward.Gold != rewardGold || reward.Experience != rewardExperience || reward.ExperienceAfter < reward.ExperienceBefore {
								rollbackQuestFlagRewards()
								return gameflow.InteractionResult{Accepted: false}
							}
							scalarReward = reward
						}
						var consumeFrames [][]byte
						if len(consumeRequirements) > 0 {
							consumeResult, ok := selectedPlayer.ConsumeCarriedItems(consumeRequirements)
							if !ok {
								rollbackQuestFlagRewards()
								return gameflow.InteractionResult{Accepted: false}
							}
							encodedConsumeFrames, err := carriedItemConsumeResultFrames(consumeResult, runtime.itemTemplates)
							if err != nil {
								rollbackQuestFlagRewards()
								return gameflow.InteractionResult{Accepted: false}
							}
							consumeFrames = append(consumeFrames, encodedConsumeFrames...)
							for _, change := range consumeResult.Changes {
								if !change.ItemRemoved {
									continue
								}
								quickslotFrames, ok := itemRemovalQuickslotSyncFrames(selectedPlayer, change.Slot)
								if !ok {
									rollbackQuestFlagRewards()
									return gameflow.InteractionResult{Accepted: false}
								}
								consumeFrames = append(consumeFrames, quickslotFrames...)
							}
						}
						var itemFrames [][]byte
						for i, entry := range rewardItems {
							grant, ok := selectedPlayer.GrantCarriedItem(rewardItemTemplates[i], entry.Count)
							if !ok {
								rollbackQuestFlagRewards()
								return gameflow.InteractionResult{Accepted: false}
							}
							encodedItemFrames, err := merchantBuyResultFrames(player.MerchantBuyResult{Items: grant.Items, ItemChanges: grant.ItemChanges}, runtime.itemTemplates)
							if err != nil {
								rollbackQuestFlagRewards()
								return gameflow.InteractionResult{Accepted: false}
							}
							itemFrames = append(itemFrames, encodedItemFrames...)
						}
						if rewardGold != 0 || rewardExperience != 0 || consumeGold != 0 || consumeExperience != 0 || len(rewardItems) > 0 || len(consumeRequirements) > 0 {
							updatedSelected := selectedPlayer.LiveCharacter()
							persistedSelected := selectedPlayer.PersistedSnapshot()
							persistedSelected.Gold = updatedSelected.Gold
							persistedSelected.Inventory = updatedSelected.Inventory
							persistedSelected.Quickslots = updatedSelected.Quickslots
							persistedSelected.Points[bootstrapExperiencePointType] = updatedSelected.Points[bootstrapExperiencePointType]
							updatedCharacters, ok := selectedCharacterSnapshotUpdate(sessionTicket.Characters, selectedPlayer.SessionLink().CharacterIndex, persistedSelected)
							if !ok || !saveAccountSnapshot(accounts, sessionTicket.Login, sessionTicket.Empire, updatedCharacters) {
								rollbackQuestFlagRewards()
								return gameflow.InteractionResult{Accepted: false}
							}
							sessionTicket.Characters = updatedCharacters
							selectedPlayer.SetPersistedSnapshot(persistedSelected)
							refreshLiveCharacterRegistration()
							if ownsLiveSharedWorldSession() {
								sharedWorld.UpdateCharacter(sharedWorldID, updatedSelected)
							}
							if consumeGold != 0 {
								frames = append(frames, worldproto.EncodePlayerPointChange(worldproto.PlayerPointChangePacket{
									VID:    previousSelected.VID,
									Type:   bootstrapGoldPointType,
									Amount: -int32(consumeGold),
									Value:  int32(consumeGoldAfter),
								}))
							}
							if consumeExperience != 0 {
								frames = append(frames, worldproto.EncodePlayerPointChange(worldproto.PlayerPointChangePacket{
									VID:    previousSelected.VID,
									Type:   bootstrapExperiencePointType,
									Amount: -int32(consumeExperience),
									Value:  consumeExperienceAfter,
								}))
							}
							if rewardGold != 0 {
								frames = append(frames, worldproto.EncodePlayerPointChange(worldproto.PlayerPointChangePacket{
									VID:    previousSelected.VID,
									Type:   bootstrapGoldPointType,
									Amount: int32(scalarReward.Gold),
									Value:  int32(scalarReward.GoldAfter),
								}))
							}
							if rewardExperience != 0 {
								frames = append(frames, worldproto.EncodePlayerPointChange(worldproto.PlayerPointChangePacket{
									VID:    previousSelected.VID,
									Type:   bootstrapExperiencePointType,
									Amount: int32(scalarReward.Experience),
									Value:  scalarReward.ExperienceAfter,
								}))
							}
							frames = append(frames, consumeFrames...)
							frames = append(frames, itemFrames...)
						}
						markInteractionCooldown(packet.TargetVID)
						return gameflow.InteractionResult{Accepted: true, Frames: frames}
					}
					if resolution.Definition.Kind == interactionstore.KindOpenSafebox {
						size := interactionstore.EffectiveOpenSafeboxSize(resolution.Definition)
						setPendingSafeboxPasswordChallenge(size)
						frames := make([][]byte, 0, 3)
						if resolution.Delivery != nil {
							frames = append(frames, chatproto.EncodeChatDelivery(*resolution.Delivery))
						}
						frames = append(frames, chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
							Type:    chatproto.ChatTypeCommand,
							Message: safeboxShowPasswordCommandMessage,
						}))
						frames = prependMerchantCloseFrame(frames)
						markInteractionCooldown(packet.TargetVID)
						return gameflow.InteractionResult{Accepted: true, Frames: frames}
					}
					if resolution.Delivery == nil {
						return gameflow.InteractionResult{Accepted: false}
					}
					if resolution.Definition.Kind == interactionstore.KindShopPreview {
						start, ok := merchantShopStartPacket(uint32(resolution.Actor.EntityID), resolution.Definition, runtime.itemTemplates)
						if !ok {
							clearActiveMerchantBuy()
							return gameflow.InteractionResult{Accepted: false}
						}
						activeMerchantBuy = merchantBuyContext{TargetVID: packet.TargetVID, Definition: resolution.Definition}
						hasActiveMerchantBuy = true
						if joinedSharedWorld && sharedWorld != nil && sharedWorldID != 0 {
							sharedWorld.SetMerchantWindowOpen(sharedWorldID, true)
						}
						markInteractionCooldown(packet.TargetVID)
						return gameflow.InteractionResult{Accepted: true, Frames: [][]byte{shopproto.EncodeServerStart(start)}}
					}
					markInteractionCooldown(packet.TargetVID)
					frames := prependMerchantCloseFrame([][]byte{chatproto.EncodeChatDelivery(*resolution.Delivery)})
					return gameflow.InteractionResult{Accepted: true, Frames: frames}
				},
				HandleTarget: func(packet combatproto.ClientTargetPacket) gameflow.TargetResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					if !ownsLiveSharedWorldSession() {
						return gameflow.TargetResult{Accepted: false}
					}
					selectedPlayer, ok := currentSelectedPlayer()
					if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
						return gameflow.TargetResult{Accepted: false}
					}
					if packet.TargetVID == 0 {
						clearActiveCombatTarget()
						return gameflow.TargetResult{Accepted: true}
					}
					resolution := runtime.resolveStaticActorCombatTarget(sharedWorldID, packet.TargetVID)
					if !resolution.Accepted || resolution.Packet == nil {
						return gameflow.TargetResult{Accepted: false}
					}
					if activeCombatTargetVID != resolution.Packet.TargetVID || activeCombatTargetSnapshotVersion != resolution.SnapshotVersion {
						preservingProximityArmedRetaliation := activeCombatTargetVID == 0 &&
							pendingPracticeMobServerOriginRetaliation &&
							pendingPracticeMobServerOriginRetaliationTargetVID == resolution.Packet.TargetVID &&
							pendingPracticeMobServerOriginRetaliationSnapshotVersion == resolution.SnapshotVersion
						if !preservingProximityArmedRetaliation {
							resetPracticeMobServerOriginRetaliationState()
						}
					}
					activeCombatTargetVID = resolution.Packet.TargetVID
					activeCombatTargetSnapshotVersion = resolution.SnapshotVersion
					if sharedWorld != nil && sharedWorldID != 0 {
						sharedWorld.SetSessionCombatTarget(sharedWorldID, resolution.Packet.TargetVID)
					}
					return gameflow.TargetResult{Accepted: true, Frames: [][]byte{combatproto.EncodeServerTarget(*resolution.Packet)}}
				},
				HandleAttack: func(packet combatproto.ClientAttackPacket) gameflow.AttackResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					if !ownsLiveSharedWorldSession() {
						return gameflow.AttackResult{Accepted: false}
					}
					if packet.AttackType != combatproto.ClientAttackTypeNormal {
						return gameflow.AttackResult{Accepted: false}
					}
					selectedPlayer, ok := currentSelectedPlayer()
					if !ok || selectedPlayerAtBootstrapHPFloor(selectedPlayer) {
						return gameflow.AttackResult{Accepted: false}
					}
					if !nextAllowedNormalAttackAt.IsZero() && sessionNow().Before(nextAllowedNormalAttackAt) {
						return gameflow.AttackResult{Accepted: false}
					}
					previousSelected := selectedPlayer.LiveCharacter()
					resolution := runtime.resolveSelectedStaticActorNormalAttack(sharedWorldID, activeCombatTargetVID, activeCombatTargetSnapshotVersion, packet.TargetVID)
					if !resolution.Accepted {
						return gameflow.AttackResult{Accepted: false}
					}
					if resolution.ClearActiveTarget {
						clearActiveCombatTarget()
					} else {
						nextAllowedNormalAttackAt = sessionNow().Add(bootstrapNormalAttackCadenceWindow)
					}
					frames := append([][]byte(nil), resolution.Frames...)
					if len(frames) == 0 {
						if resolution.Packet == nil {
							return gameflow.AttackResult{Accepted: false}
						}
						frames = append(frames, combatproto.EncodeServerTarget(*resolution.Packet))
					}
					if !resolution.DeathReward.Empty() {
						attackFrames := append([][]byte(nil), frames...)
						type rewardDrop struct {
							vid  uint32
							item inventory.ItemInstance
						}
						rewardDrops := make([]rewardDrop, 0, len(resolution.DeathReward.DropVnums))
						dropOwnerMetadataValid := validRewardOwnerMetadata(sessionTicket.Login) && validRewardDropOwnerNameMetadata(previousSelected.Name)
						if len(resolution.DeathReward.DropVnums) != 0 && dropOwnerMetadataValid {
							seenVIDs := make(map[uint32]struct{}, len(resolution.DeathReward.DropVnums))
							for index, vnum := range resolution.DeathReward.DropVnums {
								if vnum == 0 {
									return gameflow.AttackResult{Accepted: true, Frames: attackFrames}
								}
								groundVID := bootstrapRewardGroundItemVID(previousSelected, vnum, index)
								if groundVID == 0 {
									return gameflow.AttackResult{Accepted: true, Frames: attackFrames}
								}
								if _, exists := seenVIDs[groundVID]; exists {
									return gameflow.AttackResult{Accepted: true, Frames: attackFrames}
								}
								seenVIDs[groundVID] = struct{}{}
								if !runtime.rewardDropTemplateAllowed(selectedPlayer, vnum) {
									continue
								}
								item := inventory.ItemInstance{ID: uint64(groundVID), Vnum: vnum, Count: 1}
								if sharedWorld != nil && sharedWorld.GroundItemExists(groundVID) {
									continue
								}
								rewardDrops = append(rewardDrops, rewardDrop{vid: groundVID, item: item})
							}
						}
						if resolution.DeathReward.Experience != 0 || resolution.DeathReward.Gold != 0 {
							reward, rewardOK := selectedPlayer.ApplyStaticActorDeathReward(worldruntime.StaticActorDeathReward{Experience: resolution.DeathReward.Experience, Gold: resolution.DeathReward.Gold})
							if !rewardOK || reward.GoldAfter > uint64(math.MaxInt32) || reward.GoldAfter < reward.GoldBefore || reward.Gold > uint64(math.MaxInt32) || reward.Experience > uint64(math.MaxInt32) {
								selectedPlayer.SetLiveGold(previousSelected.Gold)
								selectedPlayer.SetLivePoint(bootstrapExperiencePointType, previousSelected.Points[bootstrapExperiencePointType])
								refreshLiveCharacterRegistration()
							} else {
								updatedSelected := selectedPlayer.LiveCharacter()
								persistedSelected := selectedPlayer.PersistedSnapshot()
								persistedSelected.Gold = updatedSelected.Gold
								persistedSelected.Points[bootstrapExperiencePointType] = updatedSelected.Points[bootstrapExperiencePointType]
								updatedCharacters, ok := selectedCharacterSnapshotUpdate(sessionTicket.Characters, selectedPlayer.SessionLink().CharacterIndex, persistedSelected)
								if !ok || !saveAccountSnapshot(accounts, sessionTicket.Login, sessionTicket.Empire, updatedCharacters) {
									selectedPlayer.SetLiveGold(previousSelected.Gold)
									selectedPlayer.SetLivePoint(bootstrapExperiencePointType, previousSelected.Points[bootstrapExperiencePointType])
									refreshLiveCharacterRegistration()
								} else {
									sessionTicket.Characters = updatedCharacters
									selectedPlayer.SetPersistedSnapshot(persistedSelected)
									refreshLiveCharacterRegistration()
									if ownsLiveSharedWorldSession() {
										sharedWorld.UpdateCharacter(sharedWorldID, updatedSelected)
									}
									if reward.Experience != 0 {
										frames = append(frames, worldproto.EncodePlayerPointChange(worldproto.PlayerPointChangePacket{
											VID:    previousSelected.VID,
											Type:   bootstrapExperiencePointType,
											Amount: int32(reward.Experience),
											Value:  reward.ExperienceAfter,
										}))
									}
									if reward.Gold != 0 {
										frames = append(frames, worldproto.EncodePlayerPointChange(worldproto.PlayerPointChangePacket{
											VID:    previousSelected.VID,
											Type:   bootstrapGoldPointType,
											Amount: int32(reward.Gold),
											Value:  int32(reward.GoldAfter),
										}))
									}
								}
							}
						}
						if len(rewardDrops) != 0 {
							for _, drop := range rewardDrops {
								frames = append(frames,
									itemproto.EncodeGroundAdd(itemproto.GroundAddPacket{VID: drop.vid, Vnum: drop.item.Vnum, X: previousSelected.X, Y: previousSelected.Y, Z: previousSelected.Z}),
									itemproto.EncodeOwnership(itemproto.OwnershipPacket{VID: drop.vid, OwnerName: previousSelected.Name}),
								)
							}
							if ownsLiveSharedWorldSession() {
								for _, drop := range rewardDrops {
									sharedWorld.RegisterGroundItemWithPickupRange(sharedWorldID, sessionTicket.Login, previousSelected, drop.vid, drop.item, templatePickupRange(runtime, drop.item.Vnum))
								}
							}
						}
					}
					if resolution.ClearActiveTarget {
						if credit, ok := runtime.sharedWorld.StaticActorKillQuestCredit(resolution.Actor.EntityID); ok {
							if ok, err := runtime.killQuestRequireGateSatisfied(previousSelected.Name, credit); err == nil && ok {
								transitionResult, err := runtime.ApplyQuestStateTransition(queststate.Transition{
									Character: previousSelected.Name,
									QuestRef:  credit.QuestRef,
									Flag:      credit.QuestFlag,
									From:      credit.QuestFrom,
									To:        credit.QuestTo,
								})
								if err == nil && transitionResult.Result.Applied {
									frames = append(frames, chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
										Type:    chatproto.ChatTypeInfo,
										VID:     0,
										Empire:  0,
										Message: credit.Text,
									}))
								}
							}
						}
					}
					attackFrames := append([][]byte(nil), frames...)
					if resolution.ClearActiveTarget {
						if resolution.Actor.EntityID != 0 {
							runtime.clearSpawnGroupChaseStep(resolution.Actor.EntityID)
							runtime.clearSpawnGroupHomewardStep(resolution.Actor.EntityID)
						}
					} else if resolution.Actor.EntityID != 0 {
						runtime.syncSpawnGroupChaseStepScheduleForEntity(resolution.Actor.EntityID)
					}
					retaliation, ok, clearTarget := contentPracticeMobRetaliationPointChange(runtime, selectedPlayer, resolution.Actor, resolution.ClearActiveTarget)
					if !ok {
						return gameflow.AttackResult{Accepted: true, Frames: frames}
					}
					frames = append(frames, encodePlayerPointChangeFrame(previousSelected.VID, retaliation))
					var stablePeerFrames [][]byte
					if clearTarget {
						clearActiveCombatTarget()
						sharedWorld.ClearStaticActorCombatEngagement(resolution.Actor.EntityID, sharedWorldID)
						runtime.clearSpawnGroupChaseStep(resolution.Actor.EntityID)
						deadRaw := worldproto.EncodeDead(worldproto.DeadPacket{VID: previousSelected.VID})
						frames = append(frames, deadRaw)
						frames = append(frames, combatproto.EncodeServerClearTarget())
						stablePeerFrames = [][]byte{deadRaw}
					}
					persistedFrames, ok := commitSelectedDeathFloorPersistenceFrames(selectedPlayer, previousSelected, frames, stablePeerFrames)
					if !ok {
						return gameflow.AttackResult{Accepted: true, Frames: attackFrames}
					}
					if !clearTarget && len(resolution.SelfPostMutationFrames) != 0 {
						persistedFrames = append(persistedFrames, resolution.SelfPostMutationFrames...)
						if ownsLiveSharedWorldSession() && len(resolution.PeerPostMutationFrames) != 0 {
							sharedWorld.EnqueueStaticActorFramesToVisiblePeers(resolution.Actor.EntityID, sharedWorldID, resolution.PeerPostMutationFrames)
						}
					}
					if !clearTarget {
						ownerRetaliationDamageInfo := encodePracticeMobOwnerRetaliationDamageInfoFrame(previousSelected.VID, retaliation)
						persistedFrames = append(persistedFrames, ownerRetaliationDamageInfo)
						if ownsLiveSharedWorldSession() {
							sharedWorld.EnqueueToVisibleSessions(sharedWorldID, selectedPlayer.LiveCharacter(), [][]byte{ownerRetaliationDamageInfo})
						}
					}
					persistedFrames = appendPostFloorContextCloseFrames(persistedFrames, clearTarget)
					if !resolution.ClearActiveTarget && !clearTarget {
						scheduleFirstPracticeMobServerOriginRetaliation(resolution.ActiveTargetVID, resolution.ActiveTargetSnapshotVersion)
					}
					return gameflow.AttackResult{Accepted: true, Frames: persistedFrames}
				},
				HandleShopBuy: func(packet shopproto.ClientBuyPacket) gameflow.ShopResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					selectedPlayer, ok := currentSelectedPlayer()
					if !ok {
						return gameflow.ShopResult{Accepted: false}
					}
					frames, ok := executeActiveMerchantBuy(selectedPlayer, uint16(packet.CatalogSlot), true)
					if !ok {
						return gameflow.ShopResult{Accepted: false}
					}
					return gameflow.ShopResult{Accepted: true, Frames: frames}
				},
				HandleShopSell: func(packet shopproto.ClientSellPacket) gameflow.ShopResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					selectedPlayer, ok := currentSelectedPlayer()
					if !ok {
						return gameflow.ShopResult{Accepted: false}
					}
					frames, ok := executeActiveMerchantSell(selectedPlayer, inventory.SlotIndex(packet.Slot), 0, false, true)
					if !ok {
						return gameflow.ShopResult{Accepted: false}
					}
					return gameflow.ShopResult{Accepted: true, Frames: frames}
				},
				HandleShopSell2: func(packet shopproto.ClientSell2Packet) gameflow.ShopResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					selectedPlayer, ok := currentSelectedPlayer()
					if !ok {
						return gameflow.ShopResult{Accepted: false}
					}
					frames, ok := executeActiveMerchantSell(selectedPlayer, inventory.SlotIndex(packet.Slot), uint16(packet.Count), true, true)
					if !ok {
						return gameflow.ShopResult{Accepted: false}
					}
					return gameflow.ShopResult{Accepted: true, Frames: frames}
				},
				HandleShopClose: func() gameflow.ShopResult {
					stateMu.Lock()
					defer stateMu.Unlock()

					if !hasActiveMerchantBuy || activeMerchantBuy.TargetVID == 0 {
						return gameflow.ShopResult{Accepted: false}
					}
					clearActiveMerchantBuy()
					return gameflow.ShopResult{Accepted: true, Frames: [][]byte{shopproto.EncodeServerEnd()}}
				},
			},
		})
		return newQueuedSessionFlow(inner, pending, func() {
			runtime.flushReadyStaticActorRespawns()
			runtime.flushDueSpawnGroupReturnSteps()
			runtime.flushDueSpawnGroupHomewardSteps()
			runtime.flushDueSpawnGroupChaseSteps()
			runtime.flushProximitySpawnGroupAggroAcquisition()
			if runtime.sharedWorld != nil {
				runtime.sharedWorld.FlushDueGroundItemOwnershipReleases()
			}
			stateMu.Lock()
			defer stateMu.Unlock()
			armPracticeMobServerOriginRetaliationFromProximityEngagement()
			flushPendingPracticeMobServerOriginRetaliation(pending)
		}, func() {
			stateMu.Lock()
			leaveID := sharedWorldID
			shouldLeave := joinedSharedWorld
			joinedSharedWorld = false
			clearActiveMerchantBuy()
			clearPendingSafeboxPasswordChallenge()
			setActiveSafeboxOpen(0, false)
			clearActiveSafeboxItems()
			setActiveRefineDialog(refineDialogPresentation{}, false)
			clearActiveCombatTarget()
			clearLiveCharacterRegistration()
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
	selected.X = x
	selected.Y = y
	updatedCharacters, ok := selectedCharacterSnapshotUpdate(characters, selectedIndex, selected)
	if !ok {
		return nil, loginticket.Character{}, false
	}
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
	selected.MapIndex = mapIndex
	selected.X = x
	selected.Y = y
	updatedCharacters, ok := selectedCharacterSnapshotUpdate(characters, selectedIndex, selected)
	if !ok {
		return nil, loginticket.Character{}, false
	}
	return updatedCharacters, selected, true
}

func selectedCharacterSnapshotUpdate(characters []loginticket.Character, selectedIndex uint8, updated loginticket.Character) ([]loginticket.Character, bool) {
	index := int(selectedIndex)
	if index < 0 || index >= len(characters) || updated.ID == 0 || characters[index].ID == 0 || updated.ID != characters[index].ID {
		return nil, false
	}
	clonedUpdated := loginticket.CloneCharacters([]loginticket.Character{updated})
	if len(clonedUpdated) != 1 {
		return nil, false
	}
	clonedUpdated[0].NormalizeItemState()
	updatedCharacters := cloneCharacters(characters)
	updatedCharacters[index] = clonedUpdated[0]
	return updatedCharacters, true
}

func selectedCharacterSnapshotByIDUpdate(characters []loginticket.Character, characterID uint32, updated loginticket.Character) ([]loginticket.Character, bool) {
	if characterID == 0 || updated.ID == 0 || updated.ID != characterID {
		return nil, false
	}
	clonedUpdated := loginticket.CloneCharacters([]loginticket.Character{updated})
	if len(clonedUpdated) != 1 {
		return nil, false
	}
	clonedUpdated[0].NormalizeItemState()
	updatedCharacters := cloneCharacters(characters)
	for i := range updatedCharacters {
		if updatedCharacters[i].ID == characterID {
			updatedCharacters[i] = clonedUpdated[0]
			return updatedCharacters, true
		}
	}
	return nil, false
}

func cloneCharacters(characters []loginticket.Character) []loginticket.Character {
	return loginticket.CloneCharacters(characters)
}

func applyExchangeFinalize(runtime *gameRuntime, accounts accountstore.Store, sharedWorld *sharedWorldRegistry, selectedPlayer *player.Runtime, sessionTicket *loginticket.Ticket, plan *exchangeFinalizePlan) ([][]byte, bool) {
	if runtime == nil || accounts == nil || sharedWorld == nil || selectedPlayer == nil || sessionTicket == nil || plan == nil {
		return nil, false
	}
	selected := selectedPlayer.LiveCharacter()
	if selected.ID == 0 {
		return nil, false
	}
	originLogin, ok := runtime.liveCharacterLogin(plan.Origin.Name)
	if !ok {
		return nil, false
	}
	partnerLogin, ok := runtime.liveCharacterLogin(plan.Partner.Name)
	if !ok {
		return nil, false
	}
	templates := runtime.itemTemplates
	updatedOrigin, originFrames, ok := buildExchangeFinalizeSide(plan.Origin, plan.OriginItems, plan.PartnerItems, plan.OriginGold, plan.PartnerGold, templates)
	if !ok {
		return nil, false
	}
	updatedPartner, partnerFrames, ok := buildExchangeFinalizeSide(plan.Partner, plan.PartnerItems, plan.OriginItems, plan.PartnerGold, plan.OriginGold, templates)
	if !ok {
		return nil, false
	}

	originAccount, err := accounts.Load(originLogin)
	if err != nil {
		return nil, false
	}
	partnerAccount, err := accounts.Load(partnerLogin)
	if err != nil {
		return nil, false
	}
	updatedOriginCharacters, ok := selectedCharacterSnapshotByIDUpdate(originAccount.Characters, plan.Origin.ID, updatedOrigin)
	if !ok {
		return nil, false
	}
	updatedPartnerCharacters, ok := selectedCharacterSnapshotByIDUpdate(partnerAccount.Characters, plan.Partner.ID, updatedPartner)
	if !ok {
		return nil, false
	}
	if !saveAccountSnapshot(accounts, originAccount.Login, originAccount.Empire, updatedOriginCharacters) {
		return nil, false
	}
	if !saveAccountSnapshot(accounts, partnerAccount.Login, partnerAccount.Empire, updatedPartnerCharacters) {
		_ = saveAccountSnapshot(accounts, originAccount.Login, originAccount.Empire, originAccount.Characters)
		return nil, false
	}

	applyLocalSnapshot := func(updated loginticket.Character) bool {
		current := selectedPlayer.LiveCharacter()
		if current.ID == 0 || updated.ID == 0 || current.ID != updated.ID {
			return false
		}
		updatedCharacters, ok := selectedCharacterSnapshotUpdate(sessionTicket.Characters, selectedPlayer.SessionLink().CharacterIndex, updated)
		if !ok {
			return false
		}
		sessionTicket.Characters = updatedCharacters
		selectedPlayer.ApplyPersistedSnapshot(updated)
		return true
	}
	rollbackAccounts := func() {
		_ = saveAccountSnapshot(accounts, originAccount.Login, originAccount.Empire, originAccount.Characters)
		_ = saveAccountSnapshot(accounts, partnerAccount.Login, partnerAccount.Empire, partnerAccount.Characters)
	}

	var selfFrames [][]byte
	var peerFrames [][]byte
	switch selected.ID {
	case plan.Origin.ID:
		if !applyLocalSnapshot(updatedOrigin) {
			rollbackAccounts()
			return nil, false
		}
		if !runtime.applyLiveCharacterPersistedSnapshot(plan.Partner.Name, updatedPartner) {
			rollbackAccounts()
			_ = applyLocalSnapshot(plan.Origin)
			return nil, false
		}
		selfFrames = make([][]byte, 0, 3+len(originFrames))
		selfFrames = append(selfFrames, encodeExchangeAcceptFrame(1))
		selfFrames = append(selfFrames, originFrames...)
		selfFrames = append(selfFrames, encodeExchangeFinalizeSuccessInfoFrame(plan.Partner.Name))
		selfFrames = append(selfFrames, encodeExchangeEndFrame())
		peerFrames = make([][]byte, 0, 3+len(partnerFrames))
		peerFrames = append(peerFrames, encodeExchangeAcceptFrame(0))
		peerFrames = append(peerFrames, partnerFrames...)
		peerFrames = append(peerFrames, encodeExchangeFinalizeSuccessInfoFrame(plan.Origin.Name))
		peerFrames = append(peerFrames, encodeExchangeEndFrame())
	case plan.Partner.ID:
		if !applyLocalSnapshot(updatedPartner) {
			rollbackAccounts()
			return nil, false
		}
		if !runtime.applyLiveCharacterPersistedSnapshot(plan.Origin.Name, updatedOrigin) {
			rollbackAccounts()
			_ = applyLocalSnapshot(plan.Partner)
			return nil, false
		}
		selfFrames = make([][]byte, 0, 3+len(partnerFrames))
		selfFrames = append(selfFrames, encodeExchangeAcceptFrame(1))
		selfFrames = append(selfFrames, partnerFrames...)
		selfFrames = append(selfFrames, encodeExchangeFinalizeSuccessInfoFrame(plan.Origin.Name))
		selfFrames = append(selfFrames, encodeExchangeEndFrame())
		peerFrames = make([][]byte, 0, 3+len(originFrames))
		peerFrames = append(peerFrames, encodeExchangeAcceptFrame(0))
		peerFrames = append(peerFrames, originFrames...)
		peerFrames = append(peerFrames, encodeExchangeFinalizeSuccessInfoFrame(plan.Partner.Name))
		peerFrames = append(peerFrames, encodeExchangeEndFrame())
	default:
		rollbackAccounts()
		return nil, false
	}

	busyFrames, committed := sharedWorld.CommitExchangeFinalize(plan, updatedOrigin, updatedPartner, peerFrames)
	if !committed {
		rollbackAccounts()
		switch selected.ID {
		case plan.Origin.ID:
			_ = applyLocalSnapshot(plan.Origin)
			_ = runtime.applyLiveCharacterPersistedSnapshot(plan.Partner.Name, plan.Partner)
		case plan.Partner.ID:
			_ = applyLocalSnapshot(plan.Partner)
			_ = runtime.applyLiveCharacterPersistedSnapshot(plan.Origin.Name, plan.Origin)
		}
		if len(busyFrames) > 0 {
			return busyFrames, false
		}
		return nil, false
	}
	return selfFrames, true
}

func buildExchangeFinalizeSide(
	self loginticket.Character,
	outgoing map[uint8]exchangeDisplayedItem,
	incoming map[uint8]exchangeDisplayedItem,
	outgoingGold uint32,
	incomingGold uint32,
	templates map[uint32]itemcatalog.Template,
) (loginticket.Character, [][]byte, bool) {
	updated := cloneExchangeCharacter(self)
	workingInventory := append([]inventory.ItemInstance(nil), updated.Inventory...)
	removedSlots := make([]inventory.SlotIndex, 0, len(outgoing))

	for _, displaySlot := range sortedExchangeDisplaySlots(outgoing) {
		display := outgoing[displaySlot]
		idx, ok := exchangeInventoryIndexForDisplayedItem(workingInventory, display)
		if !ok {
			return loginticket.Character{}, nil, false
		}
		removedSlots = append(removedSlots, workingInventory[idx].Slot)
		workingInventory = append(workingInventory[:idx], workingInventory[idx+1:]...)
	}

	beforePlace := append([]inventory.ItemInstance(nil), workingInventory...)
	for _, displaySlot := range sortedExchangeDisplaySlots(incoming) {
		display := incoming[displaySlot]
		template, ok := templates[display.Vnum]
		if !ok || !itemcatalog.ValidTemplate(template) || template.Vnum != display.Vnum {
			return loginticket.Character{}, nil, false
		}
		if !exchangePlaceIncomingDisplayedItemPreferringSlots(&workingInventory, display, template, removedSlots) {
			return loginticket.Character{}, nil, false
		}
	}

	netGoldDelta := int64(incomingGold) - int64(outgoingGold)
	if netGoldDelta < 0 {
		debit := uint64(-netGoldDelta)
		if updated.Gold < debit {
			return loginticket.Character{}, nil, false
		}
		updated.Gold -= debit
	} else if netGoldDelta > 0 {
		credit := uint64(netGoldDelta)
		if credit > exchangeGoldPointChangeCarrierMax || updated.Gold > exchangeGoldPointChangeCarrierMax || updated.Gold > exchangeGoldPointChangeCarrierMax-credit {
			return loginticket.Character{}, nil, false
		}
		updated.Gold += credit
	}

	updated.Inventory = workingInventory
	updated.NormalizeItemState()

	runtimeSide := player.NewRuntime(updated, player.SessionLink{})
	quickslotFrames := make([][]byte, 0)
	seenRemovedSlots := make(map[inventory.SlotIndex]struct{}, len(removedSlots))
	for _, slot := range removedSlots {
		if _, seen := seenRemovedSlots[slot]; seen {
			continue
		}
		seenRemovedSlots[slot] = struct{}{}
		frames, ok := itemRemovalQuickslotSyncFrames(runtimeSide, slot)
		if !ok {
			return loginticket.Character{}, nil, false
		}
		quickslotFrames = append(quickslotFrames, frames...)
	}
	updated = runtimeSide.LiveCharacter()

	itemFrames, ok := exchangeFinalizeInventoryFrames(self.Inventory, beforePlace, updated.Inventory, templates)
	if !ok {
		return loginticket.Character{}, nil, false
	}

	frames := make([][]byte, 0, len(itemFrames)+len(quickslotFrames)+1)
	frames = append(frames, itemFrames...)
	frames = append(frames, quickslotFrames...)
	if netGoldDelta != 0 {
		frames = append(frames, worldproto.EncodePlayerPointChange(worldproto.PlayerPointChangePacket{
			VID:    updated.VID,
			Type:   bootstrapGoldPointType,
			Amount: int32(netGoldDelta),
			Value:  int32(updated.Gold),
		}))
	}
	return updated, frames, true
}

func exchangeInventoryIndexForDisplayedItem(items []inventory.ItemInstance, display exchangeDisplayedItem) (int, bool) {
	matches := -1
	for idx, item := range items {
		if item.Equipped || item.Locked || item.ID != display.ItemID || item.Vnum != display.Vnum || item.Count != display.Count || item.Slot != display.Slot {
			continue
		}
		if matches != -1 {
			return -1, false
		}
		matches = idx
	}
	if matches < 0 {
		return -1, false
	}
	return matches, true
}

func exchangePlaceIncomingDisplayedItemPreferringSlots(items *[]inventory.ItemInstance, display exchangeDisplayedItem, template itemcatalog.Template, preferredSlots []inventory.SlotIndex) bool {
	if items == nil || display.Count == 0 || display.Count > template.MaxCount {
		return false
	}
	remaining := display.Count
	if template.Stackable {
		for idx := range *items {
			item := (*items)[idx]
			if item.Vnum == display.Vnum && item.Count > template.MaxCount {
				return false
			}
			if item.Equipped || item.Locked || item.Vnum != display.Vnum || item.Count >= template.MaxCount {
				continue
			}
			room := template.MaxCount - item.Count
			if room > remaining {
				room = remaining
			}
			item.Count += room
			if err := item.Validate(); err != nil {
				return false
			}
			(*items)[idx] = item
			remaining -= room
			if remaining == 0 {
				return true
			}
		}
	}
	if remaining == 0 {
		return true
	}
	if !template.Stackable && remaining != 1 {
		return false
	}
	slot, ok := exchangePreferredOrNextFreeInventorySlot(*items, preferredSlots)
	if !ok {
		return false
	}
	placed, err := (inventory.ItemInstance{ID: display.ItemID, Vnum: display.Vnum, Count: remaining}).WithInventorySlot(slot)
	if err != nil {
		return false
	}
	*items = append(*items, placed)
	sort.Slice(*items, func(i int, j int) bool {
		if (*items)[i].Slot != (*items)[j].Slot {
			return (*items)[i].Slot < (*items)[j].Slot
		}
		return (*items)[i].ID < (*items)[j].ID
	})
	return true
}

func exchangePreferredOrNextFreeInventorySlot(items []inventory.ItemInstance, preferredSlots []inventory.SlotIndex) (inventory.SlotIndex, bool) {
	occupied := make(map[inventory.SlotIndex]struct{}, len(items))
	for _, item := range items {
		if item.Equipped || item.Slot >= inventory.CarriedInventorySlotCount {
			return 0, false
		}
		occupied[item.Slot] = struct{}{}
	}
	for _, slot := range preferredSlots {
		if slot >= inventory.CarriedInventorySlotCount {
			continue
		}
		if _, exists := occupied[slot]; !exists {
			return slot, true
		}
	}
	for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
		if _, exists := occupied[slot]; !exists {
			return slot, true
		}
	}
	return 0, false
}

func exchangeFinalizeInventoryFrames(
	previous []inventory.ItemInstance,
	afterRemoval []inventory.ItemInstance,
	final []inventory.ItemInstance,
	templates map[uint32]itemcatalog.Template,
) ([][]byte, bool) {
	frames := make([][]byte, 0)
	previousByID := make(map[uint64]inventory.ItemInstance, len(previous))
	for _, item := range previous {
		previousByID[item.ID] = item
	}
	afterRemovalByID := make(map[uint64]inventory.ItemInstance, len(afterRemoval))
	for _, item := range afterRemoval {
		afterRemovalByID[item.ID] = item
	}

	removedIDs := make([]uint64, 0)
	for id := range previousByID {
		if _, stillPresent := afterRemovalByID[id]; !stillPresent {
			removedIDs = append(removedIDs, id)
		}
	}
	sort.Slice(removedIDs, func(i, j int) bool { return removedIDs[i] < removedIDs[j] })
	for _, id := range removedIDs {
		item := previousByID[id]
		frames = append(frames, itemproto.EncodeDel(itemproto.DelPacket{Position: itemproto.InventoryPosition(uint16(item.Slot))}))
	}

	type placedChange struct {
		before inventory.ItemInstance
		after  inventory.ItemInstance
		merged bool
	}
	changes := make([]placedChange, 0)
	seen := make(map[uint64]struct{})
	for _, item := range final {
		if _, ok := seen[item.ID]; ok {
			continue
		}
		seen[item.ID] = struct{}{}
		before, existedBefore := afterRemovalByID[item.ID]
		if !existedBefore {
			changes = append(changes, placedChange{after: item, merged: false})
			continue
		}
		if before.Count != item.Count || before.Slot != item.Slot || before.Vnum != item.Vnum {
			changes = append(changes, placedChange{before: before, after: item, merged: true})
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].after.Slot != changes[j].after.Slot {
			return changes[i].after.Slot < changes[j].after.Slot
		}
		return changes[i].after.ID < changes[j].after.ID
	})
	for _, change := range changes {
		if change.merged {
			frame, err := encodeInventoryItemUpdateFrameWithTemplates(change.after, templates)
			if err != nil {
				return nil, false
			}
			frames = append(frames, frame)
			continue
		}
		frame, err := encodeBootstrapInventoryItemFrameWithTemplates(change.after, templates)
		if err != nil {
			return nil, false
		}
		frames = append(frames, frame)
	}
	return frames, true
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
	if !strings.HasPrefix(message, "/") || message != strings.TrimSpace(message) {
		return "", false
	}
	command := message[1:]
	if command == "" || strings.ContainsAny(command, " 	\n\r\v\f") {
		return "", false
	}
	switch command {
	case "quit", "logout", "phase_select", "restart_here", "restart_town":
		return command, true
	default:
		return "", false
	}
}

func slashInventoryMoveCommand(message string) (inventory.SlotIndex, inventory.SlotIndex, bool) {
	if !strings.HasPrefix(message, "/") {
		return 0, 0, false
	}
	fields := strings.Fields(strings.TrimSpace(message[1:]))
	if len(fields) != 3 || fields[0] != "inventory_move" {
		return 0, 0, false
	}
	from, err := strconv.ParseUint(fields[1], 10, 16)
	if err != nil {
		return 0, 0, false
	}
	to, err := strconv.ParseUint(fields[2], 10, 16)
	if err != nil {
		return 0, 0, false
	}
	return inventory.SlotIndex(from), inventory.SlotIndex(to), true
}

func slashEquipItemCommand(message string) (inventory.SlotIndex, inventory.EquipmentSlot, bool) {
	if !strings.HasPrefix(message, "/") {
		return 0, inventory.EquipmentSlotNone, false
	}
	fields := strings.Fields(strings.TrimSpace(message[1:]))
	if len(fields) != 3 || fields[0] != "equip_item" {
		return 0, inventory.EquipmentSlotNone, false
	}
	from, err := strconv.ParseUint(fields[1], 10, 16)
	if err != nil {
		return 0, inventory.EquipmentSlotNone, false
	}
	equipSlot, ok := inventory.ParseEquipmentSlot(fields[2])
	if !ok || !equipSlot.Valid() {
		return 0, inventory.EquipmentSlotNone, false
	}
	return inventory.SlotIndex(from), equipSlot, true
}

func slashUnequipItemCommand(message string) (inventory.EquipmentSlot, inventory.SlotIndex, bool) {
	if !strings.HasPrefix(message, "/") {
		return inventory.EquipmentSlotNone, 0, false
	}
	fields := strings.Fields(strings.TrimSpace(message[1:]))
	if len(fields) != 3 || fields[0] != "unequip_item" {
		return inventory.EquipmentSlotNone, 0, false
	}
	equipSlot, ok := inventory.ParseEquipmentSlot(fields[1])
	if !ok || !equipSlot.Valid() {
		return inventory.EquipmentSlotNone, 0, false
	}
	to, err := strconv.ParseUint(fields[2], 10, 16)
	if err != nil {
		return inventory.EquipmentSlotNone, 0, false
	}
	return equipSlot, inventory.SlotIndex(to), true
}

func slashUseItemCommand(message string) (inventory.SlotIndex, bool) {
	if !strings.HasPrefix(message, "/") {
		return 0, false
	}
	fields := strings.Fields(strings.TrimSpace(message[1:]))
	if len(fields) != 2 || fields[0] != "use_item" {
		return 0, false
	}
	slot, err := strconv.ParseUint(fields[1], 10, 16)
	if err != nil {
		return 0, false
	}
	return inventory.SlotIndex(slot), true
}

func slashShopBuyCommand(message string) (uint16, bool) {
	if !strings.HasPrefix(message, "/") {
		return 0, false
	}
	fields := strings.Fields(strings.TrimSpace(message[1:]))
	if len(fields) != 2 || fields[0] != "shop_buy" {
		return 0, false
	}
	slot, err := strconv.ParseUint(fields[1], 10, 16)
	if err != nil {
		return 0, false
	}
	return uint16(slot), true
}

func slashOpenSafeboxCommand(message string) (uint8, bool, bool) {
	if !strings.HasPrefix(message, "/") {
		return 0, false, false
	}
	fields := strings.Fields(strings.TrimSpace(message[1:]))
	if len(fields) == 0 || fields[0] != "open_safebox" {
		return 0, false, false
	}
	switch len(fields) {
	case 1:
		return bootstrapSafeboxOpenMinSize, false, true
	case 2:
		parsed, err := strconv.ParseUint(fields[1], 10, 8)
		if err != nil {
			// Recognized command with a non-uint8 size token: keep ok=true so the
			// chat handler can fail closed instead of broadcasting ordinary talk.
			return 0, true, true
		}
		return uint8(parsed), true, true
	default:
		// Extra args are still a recognized /open_safebox attempt; fail closed in
		// the handler rather than falling through as ordinary talking chat.
		return 0, false, true
	}
}

func slashCloseSafeboxCommand(message string) bool {
	if !strings.HasPrefix(message, "/") {
		return false
	}
	fields := strings.Fields(strings.TrimSpace(message[1:]))
	if len(fields) != 1 {
		return false
	}
	switch fields[0] {
	case "close_safebox", "safebox_close":
		return true
	default:
		return false
	}
}

func slashSafeboxPasswordCommand(message string) (string, bool) {
	if !strings.HasPrefix(message, "/") {
		return "", false
	}
	fields := strings.Fields(strings.TrimSpace(message[1:]))
	if len(fields) == 0 || fields[0] != "safebox_password" {
		return "", false
	}
	switch len(fields) {
	case 1:
		// Recognized command with missing password: consume and reject in handler.
		return "", true
	case 2:
		return fields[1], true
	default:
		// Extra args stay recognized so the handler can fail closed instead of
		// falling through as ordinary talking chat.
		return "", true
	}
}

func slashSafeboxChangePasswordCommand(message string) (string, string, bool) {
	if !strings.HasPrefix(message, "/") {
		return "", "", false
	}
	fields := strings.Fields(strings.TrimSpace(message[1:]))
	if len(fields) == 0 || fields[0] != "safebox_change_password" {
		return "", "", false
	}
	switch len(fields) {
	case 1:
		// Recognized command with missing passwords: consume and reject in handler.
		return "", "", true
	case 2:
		return fields[1], "", true
	case 3:
		return fields[1], fields[2], true
	default:
		// Extra args stay recognized so the handler can fail closed instead of
		// falling through as ordinary talking chat.
		return "", "", true
	}
}

func slashSafeboxMoneySaveCommand(message string) (uint64, bool) {
	return slashSafeboxMoneyAmountCommand(message, "safebox_money_save")
}

func slashSafeboxMoneyWithdrawCommand(message string) (uint64, bool) {
	return slashSafeboxMoneyAmountCommand(message, "safebox_money_withdraw")
}

func slashSafeboxMoneyAmountCommand(message string, command string) (uint64, bool) {
	if !strings.HasPrefix(message, "/") {
		return 0, false
	}
	fields := strings.Fields(strings.TrimSpace(message[1:]))
	if len(fields) == 0 || fields[0] != command {
		return 0, false
	}
	switch len(fields) {
	case 1:
		// Recognized command with missing amount: consume and reject in handler.
		return 0, true
	case 2:
		parsed, err := strconv.ParseUint(fields[1], 10, 32)
		if err != nil {
			return 0, true
		}
		return parsed, true
	default:
		// Extra args stay recognized so the handler can fail closed instead of
		// falling through as ordinary talking chat.
		return 0, true
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
	return ticketCharacterAdditionalInfoPacketWithTemplates(character, nil)
}

func ticketCharacterAdditionalInfoPacketWithTemplates(character loginticket.Character, templates map[uint32]itemcatalog.Template) worldproto.CharacterAdditionalInfoPacket {
	return worldproto.CharacterAdditionalInfoPacket{
		VID:       character.VID,
		Name:      character.Name,
		Parts:     ticketCharacterAppearanceParts(character, templates),
		Empire:    character.Empire,
		GuildID:   character.GuildID,
		Level:     uint32(character.Level),
		Alignment: 0,
		PKMode:    0,
		MountVnum: 0,
	}
}

func ticketCharacterUpdatePacket(character loginticket.Character) worldproto.CharacterUpdatePacket {
	return ticketCharacterUpdatePacketWithTemplates(character, nil)
}

func ticketCharacterUpdatePacketWithTemplates(character loginticket.Character, templates map[uint32]itemcatalog.Template) worldproto.CharacterUpdatePacket {
	return worldproto.CharacterUpdatePacket{
		VID:         character.VID,
		Parts:       ticketCharacterAppearanceParts(character, templates),
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

func ticketCharacterAppearanceParts(character loginticket.Character, templates map[uint32]itemcatalog.Template) [worldproto.CharacterEquipmentPartCount]uint16 {
	parts := [worldproto.CharacterEquipmentPartCount]uint16{character.MainPart, 0, 0, character.HairPart}
	for _, instance := range character.Equipment {
		if !instance.Equipped {
			continue
		}
		switch instance.EquipSlot {
		case inventory.EquipmentSlotBody:
			parts[0] = ticketEquipmentAppearanceVnum(instance, templates)
		case inventory.EquipmentSlotWeapon:
			parts[1] = ticketEquipmentAppearanceVnum(instance, templates)
		case inventory.EquipmentSlotHead:
			parts[2] = ticketEquipmentAppearanceVnum(instance, templates)
		case inventory.EquipmentSlotHair:
			parts[3] = ticketEquipmentAppearanceVnum(instance, templates)
		}
	}
	return parts
}

func ticketEquipmentAppearanceVnum(instance inventory.ItemInstance, templates map[uint32]itemcatalog.Template) uint16 {
	template, ok := templates[instance.Vnum]
	if !ok || template.AppearanceVnum == 0 || !itemcatalog.ValidTemplate(template) {
		return uint16(instance.Vnum)
	}
	slot, ok := inventory.ParseEquipmentSlot(template.EquipSlot)
	if !ok || slot != instance.EquipSlot {
		return uint16(instance.Vnum)
	}
	return uint16(template.AppearanceVnum)
}

func projectedAppearanceStablePeerFrames(character loginticket.Character, slot inventory.EquipmentSlot, templates map[uint32]itemcatalog.Template) [][]byte {
	if !projectedAppearanceEquipmentSlot(slot) {
		return nil
	}
	return [][]byte{worldproto.EncodeCharacterUpdate(ticketCharacterUpdatePacketWithTemplates(character, templates))}
}

func projectedAppearanceEquipmentSlot(slot inventory.EquipmentSlot) bool {
	switch slot {
	case inventory.EquipmentSlotBody, inventory.EquipmentSlotWeapon, inventory.EquipmentSlotHead, inventory.EquipmentSlotHair:
		return true
	default:
		return false
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

func buildSelectedItemBootstrapFrames(character loginticket.Character, templates map[uint32]itemcatalog.Template) ([][]byte, error) {
	if len(character.Inventory) == 0 && len(character.Equipment) == 0 {
		return nil, nil
	}

	frames := make([][]byte, 0, len(character.Inventory)+len(character.Equipment))
	carried := append([]inventory.ItemInstance(nil), character.Inventory...)
	sort.Slice(carried, func(i int, j int) bool {
		return carried[i].Slot < carried[j].Slot
	})
	for _, instance := range carried {
		raw, err := encodeBootstrapInventoryItemFrameWithTemplates(instance, templates)
		if err != nil {
			return nil, err
		}
		frames = append(frames, raw)
	}
	equipped := append([]inventory.ItemInstance(nil), character.Equipment...)
	sort.Slice(equipped, func(i int, j int) bool {
		leftPosition, leftOK := equipmentBootstrapPosition(equipped[i].EquipSlot)
		rightPosition, rightOK := equipmentBootstrapPosition(equipped[j].EquipSlot)
		if !leftOK || !rightOK {
			return equipped[i].EquipSlot < equipped[j].EquipSlot
		}
		return leftPosition.Cell < rightPosition.Cell
	})
	for _, instance := range equipped {
		raw, err := encodeBootstrapEquipmentItemFrameWithTemplates(instance, templates)
		if err != nil {
			return nil, err
		}
		frames = append(frames, raw)
	}
	return frames, nil
}

func itemMoveQuickslotSyncFrames(selectedPlayer *player.Runtime, result inventory.MoveResult) ([][]byte, bool) {
	if selectedPlayer == nil || !result.Changed || result.From == result.To {
		return nil, true
	}
	if result.CountOnly {
		if result.FromOccupied {
			return nil, true
		}
		return itemRemovalQuickslotSyncFrames(selectedPlayer, result.From)
	}
	if result.FromOccupied && result.ToOccupied && result.FromItem.Vnum == result.ToItem.Vnum && !result.CompatibleSwap {
		return nil, true
	}
	return inventoryMoveQuickslotSyncFrames(selectedPlayer, result.From, result.To)
}

func itemUseToItemQuickslotSyncFrames(selectedPlayer *player.Runtime, result inventory.MoveResult) ([][]byte, bool) {
	if selectedPlayer == nil || !result.Changed || result.From == result.To {
		return nil, true
	}
	if !result.FromOccupied {
		return itemRemovalQuickslotSyncFrames(selectedPlayer, result.From)
	}
	if result.CountOnly {
		return nil, true
	}
	return inventoryMoveQuickslotSyncFrames(selectedPlayer, result.From, result.To)
}

func inventoryMoveQuickslotSyncFrames(selectedPlayer *player.Runtime, from inventory.SlotIndex, to inventory.SlotIndex) ([][]byte, bool) {
	changed, deleted, ok := selectedPlayer.SyncItemQuickslotsForInventoryMove(from, to)
	if !ok || len(changed)+len(deleted) == 0 {
		return nil, ok
	}
	frames := make([][]byte, 0, len(deleted)+len(changed))
	for _, slot := range deleted {
		frames = append(frames, quickslotproto.EncodeDel(quickslotproto.DelPacket{Position: slot.Position}))
	}
	for _, slot := range changed {
		frames = append(frames, quickslotproto.EncodeAdd(quickslotproto.AddPacket{Position: slot.Position, Slot: quickslotproto.Slot{Type: slot.Type, Position: slot.Slot}}))
	}
	return frames, true
}

func itemRemovalQuickslotSyncFrames(selectedPlayer *player.Runtime, slot inventory.SlotIndex) ([][]byte, bool) {
	if selectedPlayer == nil {
		return nil, true
	}
	deleted, ok := selectedPlayer.SyncItemQuickslotsForItemRemoval(slot)
	if !ok || len(deleted) == 0 {
		return nil, ok
	}
	frames := make([][]byte, 0, len(deleted))
	for _, quickslot := range deleted {
		frames = append(frames, quickslotproto.EncodeDel(quickslotproto.DelPacket{Position: quickslot.Position}))
	}
	return frames, true
}

func inventoryMoveResultFrames(result inventory.MoveResult, templates map[uint32]itemcatalog.Template) ([][]byte, error) {
	if !result.Changed {
		return nil, nil
	}
	frames := make([][]byte, 0, 2)
	if result.CountOnly {
		if result.FromOccupied {
			frame, err := encodeInventoryItemUpdateFrameWithTemplates(result.FromItem, templates)
			if err != nil {
				return nil, err
			}
			frames = append(frames, frame)
		} else {
			frames = append(frames, itemproto.EncodeDel(itemproto.DelPacket{Position: itemproto.InventoryPosition(uint16(result.From))}))
		}
		if result.ToOccupied {
			frame, err := encodeInventoryItemUpdateFrameWithTemplates(result.ToItem, templates)
			if err != nil {
				return nil, err
			}
			frames = append(frames, frame)
		}
		return frames, nil
	}
	if result.FromOccupied {
		frame, err := encodeBootstrapInventoryItemFrameWithTemplates(result.FromItem, templates)
		if err != nil {
			return nil, err
		}
		frames = append(frames, frame)
	} else {
		frames = append(frames, itemproto.EncodeDel(itemproto.DelPacket{Position: itemproto.InventoryPosition(uint16(result.From))}))
	}
	if result.ToOccupied {
		frame, err := encodeBootstrapInventoryItemFrameWithTemplates(result.ToItem, templates)
		if err != nil {
			return nil, err
		}
		frames = append(frames, frame)
	}
	return frames, nil
}

func encodeInventoryItemUpdateFrameWithTemplates(item inventory.ItemInstance, templates map[uint32]itemcatalog.Template) ([]byte, error) {
	position, err := itemproto.CarriedInventoryPosition(uint16(item.Slot))
	if err != nil {
		return nil, err
	}
	template := templates[item.Vnum]
	return itemproto.EncodeUpdate(itemproto.UpdatePacket{
		Position:   position,
		Count:      uint8(item.Count),
		Sockets:    bootstrapItemSockets(template),
		Attributes: bootstrapItemAttributes(template),
	}), nil
}

func refineInformationFrame(info player.RefineInformation) ([]byte, error) {
	if len(info.Materials) > itemproto.RefineMaterialMaxNum {
		return nil, fmt.Errorf("bootstrap refine material count exceeds fixed table: %d", len(info.Materials))
	}
	packet := itemproto.RefineInformationPacket{
		Type:     info.Type,
		Position: uint8(info.Position),
		Table: itemproto.RefineTable{
			SourceVnum:    info.SourceVnum,
			ResultVnum:    info.ResultVnum,
			MaterialCount: uint8(len(info.Materials)),
			Cost:          info.Cost,
			Probability:   info.Probability,
		},
	}
	for i, material := range info.Materials {
		packet.Table.Materials[i] = itemproto.RefineMaterial{Vnum: material.Vnum, Count: material.Count}
	}
	return itemproto.EncodeRefineInformationNew(packet)
}

func equipResultFrames(character loginticket.Character, from inventory.SlotIndex, equippedItem inventory.ItemInstance, pointChange *player.PointChangeResult, templates map[uint32]itemcatalog.Template) ([][]byte, error) {
	setFrame, err := encodeBootstrapEquipmentItemFrameWithTemplates(equippedItem, templates)
	if err != nil {
		return nil, err
	}
	frames := make([][]byte, 0, 4)
	frames = append(frames,
		itemproto.EncodeDel(itemproto.DelPacket{Position: itemproto.InventoryPosition(uint16(from))}),
		setFrame,
	)
	if pointChange != nil {
		frames = append(frames, encodePlayerPointChangeFrame(character.VID, *pointChange))
	}
	frames = append(frames, worldproto.EncodeCharacterUpdate(ticketCharacterUpdatePacketWithTemplates(character, templates)))
	return frames, nil
}

func unequipResultFrames(character loginticket.Character, from inventory.EquipmentSlot, inventoryItem inventory.ItemInstance, pointChange *player.PointChangeResult, templates map[uint32]itemcatalog.Template) ([][]byte, error) {
	position, ok := equipmentBootstrapPosition(from)
	if !ok {
		return nil, fmt.Errorf("bootstrap equipment slot unsupported: %s", from.String())
	}
	setFrame, err := encodeBootstrapInventoryItemFrameWithTemplates(inventoryItem, templates)
	if err != nil {
		return nil, err
	}
	frames := make([][]byte, 0, 4)
	frames = append(frames,
		itemproto.EncodeDel(itemproto.DelPacket{Position: position}),
		setFrame,
	)
	if pointChange != nil {
		frames = append(frames, encodePlayerPointChangeFrame(character.VID, *pointChange))
	}
	frames = append(frames, worldproto.EncodeCharacterUpdate(ticketCharacterUpdatePacketWithTemplates(character, templates)))
	return frames, nil
}

func itemUseResultFrames(character loginticket.Character, result player.ItemUseResult, templates map[uint32]itemcatalog.Template, emitUseEcho bool) ([][]byte, error) {
	position, err := itemproto.CarriedInventoryPosition(uint16(result.Slot))
	if err != nil {
		return nil, err
	}
	frames := make([][]byte, 0, 4)
	if emitUseEcho {
		frames = append(frames, itemproto.EncodeUse(itemproto.UsePacket{Position: position, CharacterVID: character.VID, VictimVID: character.VID, Vnum: result.Vnum}))
	}
	frames = append(frames, encodePlayerPointChangeFrame(character.VID, player.PointChangeResult{
		PointType:   result.PointType,
		PointAmount: result.PointAmount,
		PointValue:  result.PointValue,
	}))
	if result.ItemRemoved {
		frames = append(frames, itemproto.EncodeDel(itemproto.DelPacket{Position: position}))
	} else {
		updateFrame, err := encodeInventoryItemUpdateFrameWithTemplates(result.Item, templates)
		if err != nil {
			return nil, err
		}
		frames = append(frames, updateFrame)
	}
	if result.SpecialEffectType != 0 {
		frames = append(frames, effectproto.EncodeSpecial(effectproto.SpecialPacket{Type: result.SpecialEffectType, VID: character.VID}))
	}
	frames = append(frames, chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, Message: result.EffectMessage}))
	return frames, nil
}

func contentPracticeMobRetaliationProfile(profile string) bool {
	_, ok := worldruntime.BootstrapStaticActorCombatProfileDefaults(profile)
	return ok
}

func contentPracticeMobRetaliationPointChange(runtime *gameRuntime, selectedPlayer *player.Runtime, actor StaticActorSnapshot, targetDied bool) (player.PointChangeResult, bool, bool) {
	if selectedPlayer == nil || targetDied {
		return player.PointChangeResult{}, false, false
	}
	if actor.SpawnGroupRef == "" && runtime != nil {
		currentActors := runtime.StaticActors()
		if idx := staticActorSnapshotIndex(currentActors, actor.EntityID); idx >= 0 {
			actor = currentActors[idx]
		}
	}
	if actor.SpawnGroupRef == "" || !contentPracticeMobRetaliationProfile(actor.CombatProfile) {
		return player.PointChangeResult{}, false, false
	}
	currentPointValue := selectedPlayer.LiveCharacter().Points[bootstrapPlayerPointValueIndex]
	if currentPointValue <= 0 {
		return player.PointChangeResult{}, false, false
	}
	pointDelta, ok := worldruntime.BootstrapStaticActorRetaliationPointDelta(actor.CombatProfile)
	if !ok {
		return player.PointChangeResult{}, false, false
	}
	if pointDelta < 0 {
		minimumDelta := -currentPointValue
		if pointDelta < minimumDelta {
			pointDelta = minimumDelta
		}
	}
	if pointDelta == 0 {
		return player.PointChangeResult{}, false, false
	}
	pointChange, ok := selectedPlayer.ApplyPointDelta(bootstrapPlayerPointType, bootstrapPlayerPointValueIndex, pointDelta)
	return pointChange, ok, ok && pointChange.PointValue == 0
}

func encodePlayerPointChangeFrame(vid uint32, result player.PointChangeResult) []byte {
	return worldproto.EncodePlayerPointChange(worldproto.PlayerPointChangePacket{
		VID:    vid,
		Type:   result.PointType,
		Amount: result.PointAmount,
		Value:  result.PointValue,
	})
}

func encodePracticeMobOwnerRetaliationDamageInfoFrame(ownerVID uint32, result player.PointChangeResult) []byte {
	damage := result.PointAmount
	if damage < 0 {
		damage = -damage
	}
	return combatproto.EncodeServerDamageInfo(combatproto.ServerDamageInfoPacket{
		VID:    ownerVID,
		Flag:   0,
		Damage: damage,
	})
}

func merchantBuyResultFrames(result player.MerchantBuyResult, templates map[uint32]itemcatalog.Template) ([][]byte, error) {
	changes := result.ItemChanges
	if len(changes) == 0 && len(result.Items) != 0 {
		changes = make([]player.MerchantBuyItemChange, 0, len(result.Items))
		for _, item := range result.Items {
			changes = append(changes, player.MerchantBuyItemChange{Item: item, Created: true})
		}
	}
	frames := make([][]byte, 0, len(changes)+1)
	for _, change := range changes {
		if change.Created {
			setFrame, err := encodeBootstrapInventoryItemFrameWithTemplates(change.Item, templates)
			if err != nil {
				return nil, err
			}
			frames = append(frames, setFrame)
			continue
		}
		position, err := itemproto.CarriedInventoryPosition(uint16(change.Item.Slot))
		if err != nil {
			return nil, err
		}
		updateFrame, err := encodeBootstrapItemUpdateFrameWithTemplates(position, change.Item, templates)
		if err != nil {
			return nil, err
		}
		frames = append(frames, updateFrame)
	}
	return frames, nil
}

func itemDropRejectText(template itemcatalog.Template) string {
	if template.DropRejectText != "" {
		return template.DropRejectText
	}
	return itemDropRejectedInfoMessage
}

func runtimeTemplateDropRejectText(template itemcatalog.Template, selectedPlayer *player.Runtime) (string, bool) {
	if selectedPlayer == nil {
		return "", false
	}
	if template.AntiGet || template.AntiDrop || template.AntiGive || template.AntiSell || template.AntiStack {
		return itemDropRejectText(template), true
	}
	if template.DropRejectText == "" || selectedPlayer.CanUseTemplate(template) {
		return "", false
	}
	return itemDropRejectText(template), true
}

func itemPickupRejectText(template itemcatalog.Template) string {
	if template.PickupRejectText != "" {
		return template.PickupRejectText
	}
	return itemPickupInventoryFullInfoMessage
}

func templatePickupRange(runtime *gameRuntime, vnum uint32) int64 {
	if runtime == nil || vnum == 0 {
		return bootstrapGroundItemPickupRange
	}
	template, ok := runtime.itemTemplates[vnum]
	if !ok || template.PickupRange == 0 {
		return bootstrapGroundItemPickupRange
	}
	return int64(template.PickupRange)
}

func runtimeTemplateGoldPeerPickupRejectText(runtime *gameRuntime, item inventory.ItemInstance) (string, bool) {
	if runtime == nil || item.Vnum == 0 {
		return "", false
	}
	template, ok := runtime.itemTemplates[item.Vnum]
	if !ok {
		return "", false
	}
	if !itemcatalog.ValidTemplate(template) || template.Vnum != item.Vnum || item.Count == 0 || item.Count > template.MaxCount {
		return itemPickupInventoryFullInfoMessage, true
	}
	if template.AntiGive {
		return itemPickupRejectText(template), true
	}
	return "", false
}

func itemBuyRejectText(template itemcatalog.Template) string {
	if template.BuyRejectText != "" {
		return template.BuyRejectText
	}
	return itemBuyRejectedInfoMessage
}

func runtimeTemplateBuyRejectText(template itemcatalog.Template, selectedPlayer *player.Runtime) (string, bool) {
	if selectedPlayer == nil {
		return "", false
	}
	if template.AntiGet {
		return itemBuyRejectText(template), true
	}
	if template.BuyRejectText == "" || selectedPlayer.CanUseTemplate(template) {
		return "", false
	}
	return itemBuyRejectText(template), true
}

func itemSellRejectText(template itemcatalog.Template) string {
	if template.SellRejectText != "" {
		return template.SellRejectText
	}
	return itemSellRejectedInfoMessage
}

func runtimeTemplateSellRejectText(template itemcatalog.Template, selectedPlayer *player.Runtime) (string, bool) {
	if selectedPlayer == nil {
		return "", false
	}
	if template.AntiSell {
		return itemSellRejectText(template), true
	}
	if template.SellRejectText == "" {
		return "", false
	}
	if template.AntiGet || template.AntiDrop || template.AntiGive || template.AntiStack || !selectedPlayer.CanUseTemplate(template) {
		return itemSellRejectText(template), true
	}
	return "", false
}

func runtimeTemplateEquipRejectText(template itemcatalog.Template, selectedPlayer *player.Runtime, equipSlot inventory.EquipmentSlot) (string, bool) {
	if template.EquipRejectText == "" || selectedPlayer == nil || !templateAuthoredForRuntimeEquipSlot(template, equipSlot) {
		return "", false
	}
	if selectedPlayer.CanUseTemplate(template) && !template.AntiStack && !template.AntiGet && !template.AntiDrop && !template.AntiGive && !template.AntiSell {
		return "", false
	}
	return template.EquipRejectText, true
}

func itemUnequipRejectText(template itemcatalog.Template) string {
	if template.UnequipRejectText != "" {
		return template.UnequipRejectText
	}
	return itemUnequipRejectedInfoMessage
}

func merchantSellTemplateForSlot(templates map[uint32]itemcatalog.Template, selectedPlayer *player.Runtime, slot inventory.SlotIndex) (itemcatalog.Template, bool) {
	if selectedPlayer == nil {
		return itemcatalog.Template{}, false
	}
	for _, item := range selectedPlayer.LiveInventory() {
		if item.Equipped || item.Slot != slot {
			continue
		}
		template, ok := templates[item.Vnum]
		if !ok || item.Count > template.MaxCount {
			return itemcatalog.Template{}, false
		}
		return template, true
	}
	return itemcatalog.Template{}, false
}

func merchantSellResultFrames(character loginticket.Character, result player.MerchantSellResult, templates map[uint32]itemcatalog.Template) ([][]byte, error) {
	position, err := itemproto.CarriedInventoryPosition(uint16(result.Slot))
	if err != nil {
		return nil, err
	}
	frames := make([][]byte, 0, 3)
	if result.ItemRemoved {
		frames = append(frames, itemproto.EncodeDel(itemproto.DelPacket{Position: position}))
	} else {
		updateFrame, err := encodeBootstrapItemUpdateFrameWithTemplates(position, result.Item, templates)
		if err != nil {
			return nil, err
		}
		frames = append(frames, updateFrame)
	}
	if result.Gold < result.GoldBefore || result.Gold > uint64(math.MaxInt32) || result.Gold-result.GoldBefore > uint64(math.MaxInt32) {
		return nil, fmt.Errorf("merchant sell gold point-change out of range")
	}
	frames = append(frames, worldproto.EncodePlayerPointChange(worldproto.PlayerPointChangePacket{
		VID:    character.VID,
		Type:   bootstrapGoldPointType,
		Amount: int32(result.Gold - result.GoldBefore),
		Value:  int32(result.Gold),
	}))
	return frames, nil
}

func refineDialogSourceItem(selectedPlayer *player.Runtime, slot inventory.SlotIndex, sourceVnum uint32) (inventory.ItemInstance, bool) {
	if selectedPlayer == nil {
		return inventory.ItemInstance{}, false
	}
	for _, item := range selectedPlayer.LiveInventory() {
		if item.Equipped || item.Slot != slot || item.Vnum != sourceVnum || item.ID == 0 || item.Count != 1 || item.Locked {
			continue
		}
		if err := item.Validate(); err != nil {
			return inventory.ItemInstance{}, false
		}
		return item, true
	}
	return inventory.ItemInstance{}, false
}

func carriedItemConsumeResultFrames(result player.CarriedItemConsumeResult, templates map[uint32]itemcatalog.Template) ([][]byte, error) {
	frames := make([][]byte, 0, len(result.Changes))
	for _, change := range result.Changes {
		position, err := itemproto.CarriedInventoryPosition(uint16(change.Slot))
		if err != nil {
			return nil, err
		}
		if change.ItemRemoved {
			frames = append(frames, itemproto.EncodeDel(itemproto.DelPacket{Position: position}))
			continue
		}
		updateFrame, err := encodeBootstrapItemUpdateFrameWithTemplates(position, change.Item, templates)
		if err != nil {
			return nil, err
		}
		frames = append(frames, updateFrame)
	}
	return frames, nil
}

func refineSuccessResultFrames(character loginticket.Character, result player.RefineSuccessResult, templates map[uint32]itemcatalog.Template, refineType uint8) ([][]byte, error) {
	frames := make([][]byte, 0, len(result.MaterialChanges)+3)
	for _, change := range result.MaterialChanges {
		position, err := itemproto.CarriedInventoryPosition(uint16(change.Slot))
		if err != nil {
			return nil, err
		}
		if change.ItemRemoved {
			frames = append(frames, itemproto.EncodeDel(itemproto.DelPacket{Position: position}))
			continue
		}
		updateFrame, err := encodeBootstrapItemUpdateFrameWithTemplates(position, change.Item, templates)
		if err != nil {
			return nil, err
		}
		frames = append(frames, updateFrame)
	}
	resultFrame, err := encodeBootstrapInventoryItemFrameWithTemplates(result.ResultItem, templates)
	if err != nil {
		return nil, err
	}
	frames = append(frames, resultFrame)
	if result.GoldBefore < result.Gold || result.GoldBefore-result.Gold != uint64(result.Cost) || result.Gold > uint64(math.MaxInt32) || result.Cost < 0 {
		return nil, fmt.Errorf("refine success gold point-change out of range")
	}
	frames = append(frames, worldproto.EncodePlayerPointChange(worldproto.PlayerPointChangePacket{
		VID:    character.VID,
		Type:   bootstrapGoldPointType,
		Amount: -result.Cost,
		Value:  int32(result.Gold),
	}))
	// Legacy TMP4 clients listen for CHAT_TYPE_COMMAND "RefineSuceeded <type>"
	// (intentional historical spelling) to play the success popup/sound.
	frames = append(frames, chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
		Type:    chatproto.ChatTypeCommand,
		VID:     0,
		Empire:  0,
		Message: fmt.Sprintf("RefineSuceeded %d", refineType),
	}))
	return frames, nil
}

func refineDestroyFailureResultFrames(character loginticket.Character, result player.RefineDestroyFailureResult, templates map[uint32]itemcatalog.Template, refineType uint8) ([][]byte, error) {
	frames := make([][]byte, 0, len(result.MaterialChanges)+3)
	for _, change := range result.MaterialChanges {
		position, err := itemproto.CarriedInventoryPosition(uint16(change.Slot))
		if err != nil {
			return nil, err
		}
		if change.ItemRemoved {
			frames = append(frames, itemproto.EncodeDel(itemproto.DelPacket{Position: position}))
			continue
		}
		updateFrame, err := encodeBootstrapItemUpdateFrameWithTemplates(position, change.Item, templates)
		if err != nil {
			return nil, err
		}
		frames = append(frames, updateFrame)
	}
	sourcePosition, err := itemproto.CarriedInventoryPosition(uint16(result.SourceSlot))
	if err != nil {
		return nil, err
	}
	frames = append(frames, itemproto.EncodeDel(itemproto.DelPacket{Position: sourcePosition}))
	if result.GoldBefore < result.Gold || result.GoldBefore-result.Gold != uint64(result.Cost) || result.Gold > uint64(math.MaxInt32) || result.Cost < 0 {
		return nil, fmt.Errorf("refine destroy gold point-change out of range")
	}
	frames = append(frames, worldproto.EncodePlayerPointChange(worldproto.PlayerPointChangePacket{
		VID:    character.VID,
		Type:   bootstrapGoldPointType,
		Amount: -result.Cost,
		Value:  int32(result.Gold),
	}))
	// Legacy TMP4 clients listen for CHAT_TYPE_COMMAND "RefineFailed <type>"
	// to play the failure popup/sound.
	frames = append(frames, chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
		Type:    chatproto.ChatTypeCommand,
		VID:     0,
		Empire:  0,
		Message: fmt.Sprintf("RefineFailed %d", refineType),
	}))
	return frames, nil
}

func itemDropTemplateForSlot(templates map[uint32]itemcatalog.Template, character loginticket.Character, slot inventory.SlotIndex) (itemcatalog.Template, bool) {
	for _, item := range character.Inventory {
		if item.Slot != slot || item.Equipped || item.Vnum == 0 {
			continue
		}
		template, ok := templates[item.Vnum]
		if !ok || !itemcatalog.ValidTemplate(template) {
			return itemcatalog.Template{}, false
		}
		return template, true
	}
	return itemcatalog.Template{}, false
}

func itemDropResultFrames(character loginticket.Character, result inventory.MoveResult, droppedItem inventory.ItemInstance) ([][]byte, error) {
	return itemDropResultFramesWithTemplates(character, result, droppedItem, nil)
}

func itemDropInventoryResultFramesWithTemplates(result inventory.MoveResult, templates map[uint32]itemcatalog.Template) ([][]byte, error) {
	if !result.Changed {
		return nil, nil
	}
	position, err := itemproto.CarriedInventoryPosition(uint16(result.From))
	if err != nil {
		return nil, err
	}
	if result.FromOccupied {
		updateFrame, err := encodeBootstrapItemUpdateFrameWithTemplates(position, result.FromItem, templates)
		if err != nil {
			return nil, err
		}
		return [][]byte{updateFrame}, nil
	}
	return [][]byte{itemproto.EncodeDel(itemproto.DelPacket{Position: position})}, nil
}

func itemDropResultFramesWithTemplates(character loginticket.Character, result inventory.MoveResult, droppedItem inventory.ItemInstance, templates map[uint32]itemcatalog.Template) ([][]byte, error) {
	frames, err := itemDropInventoryResultFramesWithTemplates(result, templates)
	if err != nil {
		return nil, err
	}
	if len(frames) == 0 {
		return nil, nil
	}
	if droppedItem.Vnum == 0 {
		return nil, fmt.Errorf("item drop source item not found for slot %d", result.From)
	}
	ground := sharedGroundItem{
		VID:                bootstrapGroundItemVID(character, result.From),
		OwnerName:          character.Name,
		Item:               droppedItem,
		X:                  character.X,
		Y:                  character.Y,
		Z:                  character.Z,
		OwnershipExclusive: true,
	}
	frames = append(frames, encodeGroundItemVisibleFrames(ground)...)
	return frames, nil
}

func droppedInventoryItem(character loginticket.Character, slot inventory.SlotIndex, count uint16) (inventory.ItemInstance, bool) {
	for _, item := range character.Inventory {
		if item.Slot != slot || item.Equipped || item.Locked {
			continue
		}
		if count == 0 || count > item.Count {
			return inventory.ItemInstance{}, false
		}
		dropped := item
		dropped.Count = count
		if err := dropped.Validate(); err != nil {
			return inventory.ItemInstance{}, false
		}
		return dropped, true
	}
	return inventory.ItemInstance{}, false
}

func cloneInventoryItems(items []inventory.ItemInstance) []inventory.ItemInstance {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]inventory.ItemInstance, len(items))
	copy(cloned, items)
	return cloned
}

func sortInventoryItemsBySlot(items []inventory.ItemInstance) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Slot == items[j].Slot {
			return items[i].ID < items[j].ID
		}
		return items[i].Slot < items[j].Slot
	})
}

func characterInventorySlotOccupied(items []inventory.ItemInstance, slot inventory.SlotIndex) bool {
	for _, item := range items {
		if item.Slot == slot && !item.Equipped {
			return true
		}
	}
	return false
}

func firstAvailableCarriedInventorySlot(items []inventory.ItemInstance, preferred inventory.SlotIndex) (inventory.SlotIndex, bool) {
	if preferred < inventory.CarriedInventorySlotCount && !characterInventorySlotOccupied(items, preferred) {
		return preferred, true
	}
	for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
		if !characterInventorySlotOccupied(items, slot) {
			return slot, true
		}
	}
	return 0, false
}

func bootstrapGroundItemVID(character loginticket.Character, slot inventory.SlotIndex) uint32 {
	vid := character.VID ^ 0x80000000 ^ uint32(slot+1)
	if vid == 0 {
		return uint32(slot) + 1
	}
	return vid
}

func bootstrapRewardGroundItemVID(character loginticket.Character, vnum uint32, index int) uint32 {
	vid := character.VID ^ 0x81000000 ^ (vnum << 8) ^ uint32(index+1)
	if vid == 0 {
		return uint32(index) + 1
	}
	return vid
}

func (r *gameRuntime) rewardDropTemplateAllowed(selectedPlayer *player.Runtime, vnum uint32) bool {
	if r == nil || selectedPlayer == nil || vnum == 0 {
		return false
	}
	template, ok := r.itemTemplates[vnum]
	if !ok {
		return !r.itemTemplatesAuthored
	}
	if !itemcatalog.ValidTemplate(template) || template.Vnum != vnum || template.EquipSlot != "" || template.AntiGet || template.AntiDrop || template.AntiGive || template.AntiSell || template.AntiStack {
		return false
	}
	return selectedPlayer.CanUseTemplate(template)
}

func merchantBuyFailureFrames(failure player.MerchantBuyFailure, packetFailureFrames bool) ([][]byte, bool) {
	switch failure {
	case player.MerchantBuyFailureInsufficientGold:
		return [][]byte{shopproto.EncodeServerNotEnoughMoney()}, true
	case player.MerchantBuyFailureNoValidPlacement:
		return [][]byte{shopproto.EncodeServerInventoryFull()}, true
	case player.MerchantBuyFailureInvalid:
		return [][]byte{shopproto.EncodeServerInvalidPos()}, true
	}
	return nil, false
}

func merchantShopStartPacket(ownerVID uint32, definition InteractionDefinition, templates map[uint32]itemcatalog.Template) (shopproto.ServerStartPacket, bool) {
	if !interactionstore.ValidDefinition(definition) || definition.Kind != interactionstore.KindShopPreview {
		return shopproto.ServerStartPacket{}, false
	}
	packet := shopproto.ServerStartPacket{OwnerVID: ownerVID}
	for _, entry := range definition.Catalog {
		if entry.Slot >= shopproto.ShopHostItemMax {
			return shopproto.ServerStartPacket{}, false
		}
		if entry.Price > interactionstore.MerchantCatalogMaxEntryPrice || entry.Count > interactionstore.MerchantCatalogMaxEntryCount {
			return shopproto.ServerStartPacket{}, false
		}
		template, ok := templates[entry.ItemVnum]
		if !ok || !itemcatalog.ValidTemplate(template) {
			return shopproto.ServerStartPacket{}, false
		}
		packet.Items[entry.Slot] = shopproto.ItemEntry{
			Vnum:       entry.ItemVnum,
			Price:      uint32(entry.Price),
			Count:      uint8(entry.Count),
			DisplayPos: uint8(entry.Slot),
			Sockets:    bootstrapItemSockets(template),
			Attributes: bootstrapItemAttributes(template),
		}
	}
	return packet, true
}

func encodeBootstrapInventoryItemFrame(instance inventory.ItemInstance) ([]byte, error) {
	return encodeBootstrapInventoryItemFrameWithTemplates(instance, nil)
}

func encodeBootstrapInventoryItemFrameWithTemplates(instance inventory.ItemInstance, templates map[uint32]itemcatalog.Template) ([]byte, error) {
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	if instance.Equipped {
		return nil, fmt.Errorf("bootstrap inventory item must be unequipped: %d", instance.ID)
	}
	position, err := itemproto.CarriedInventoryPosition(uint16(instance.Slot))
	if err != nil {
		return nil, fmt.Errorf("bootstrap inventory slot out of range: %d", instance.Slot)
	}
	return encodeBootstrapItemFrameWithTemplates(position, instance, templates)
}

func encodeBootstrapEquipmentItemFrame(instance inventory.ItemInstance) ([]byte, error) {
	return encodeBootstrapEquipmentItemFrameWithTemplates(instance, nil)
}

func encodeBootstrapEquipmentItemFrameWithTemplates(instance inventory.ItemInstance, templates map[uint32]itemcatalog.Template) ([]byte, error) {
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	if !instance.Equipped {
		return nil, fmt.Errorf("bootstrap equipment item must be equipped: %d", instance.ID)
	}
	position, ok := equipmentBootstrapPosition(instance.EquipSlot)
	if !ok {
		return nil, fmt.Errorf("bootstrap equipment slot unsupported: %s", instance.EquipSlot.String())
	}
	return encodeBootstrapItemFrameWithTemplates(position, instance, templates)
}

func encodeBootstrapGroundPickupInventoryFrames(result player.GroundItemPickupResult, templates map[uint32]itemcatalog.Template) ([][]byte, bool) {
	frames := make([][]byte, 0, len(result.UpdatedItems)+1)
	if result.Merged {
		frame, ok := encodeBootstrapGroundPickupUpdateFrame(result.Updated, templates)
		if !ok {
			return nil, false
		}
		frames = append(frames, frame)
	} else if result.Split {
		for _, updated := range result.UpdatedItems {
			frame, ok := encodeBootstrapGroundPickupUpdateFrame(updated, templates)
			if !ok {
				return nil, false
			}
			frames = append(frames, frame)
		}
	}
	if result.Placed.ID != 0 {
		frame, ok := encodeBootstrapGroundPickupSetFrame(result.Placed, templates)
		if !ok {
			return nil, false
		}
		frames = append(frames, frame)
	}
	return frames, len(frames) > 0
}

func encodeBootstrapGroundPickupSetFrame(instance inventory.ItemInstance, templates map[uint32]itemcatalog.Template) ([]byte, bool) {
	position, err := itemproto.CarriedInventoryPosition(uint16(instance.Slot))
	if err != nil {
		return nil, false
	}
	frame, err := encodeBootstrapItemFrameWithTemplates(position, instance, templates)
	if err != nil {
		return nil, false
	}
	return frame, true
}

func encodeBootstrapGroundPickupUpdateFrame(instance inventory.ItemInstance, templates map[uint32]itemcatalog.Template) ([]byte, bool) {
	position, err := itemproto.CarriedInventoryPosition(uint16(instance.Slot))
	if err != nil {
		return nil, false
	}
	frame, err := encodeBootstrapItemUpdateFrameWithTemplates(position, instance, templates)
	if err != nil {
		return nil, false
	}
	return frame, true
}

func encodeBootstrapItemFrame(position itemproto.Position, instance inventory.ItemInstance) ([]byte, error) {
	return encodeBootstrapItemFrameWithTemplates(position, instance, nil)
}

func encodeBootstrapItemFrameWithTemplates(position itemproto.Position, instance inventory.ItemInstance, templates map[uint32]itemcatalog.Template) ([]byte, error) {
	packet, err := bootstrapItemSetPacket(position, instance, templates)
	if err != nil {
		return nil, err
	}
	return itemproto.EncodeSet(packet), nil
}

func nextSafeboxSplitItemID(selectedPlayer *player.Runtime, safeboxItems map[uint8]inventory.ItemInstance) uint64 {
	var maxID uint64
	if selectedPlayer != nil {
		for _, item := range selectedPlayer.LiveInventory() {
			if item.ID > maxID {
				maxID = item.ID
			}
		}
		for _, item := range selectedPlayer.LiveEquipment() {
			if item.ID > maxID {
				maxID = item.ID
			}
		}
	}
	for _, item := range safeboxItems {
		if item.ID > maxID {
			maxID = item.ID
		}
	}
	if maxID == ^uint64(0) {
		return 0
	}
	return maxID + 1
}

func encodeBootstrapSafeboxSetFrame(position itemproto.Position, instance inventory.ItemInstance, templates map[uint32]itemcatalog.Template) ([]byte, error) {
	packet, err := bootstrapItemSetPacket(position, instance, templates)
	if err != nil {
		return nil, err
	}
	return itemproto.EncodeSafeboxSet(packet), nil
}

func bootstrapItemSetPacket(position itemproto.Position, instance inventory.ItemInstance, templates map[uint32]itemcatalog.Template) (itemproto.SetPacket, error) {
	if instance.Count > 255 {
		return itemproto.SetPacket{}, fmt.Errorf("bootstrap item count exceeds legacy uint8: %d", instance.Count)
	}
	template := templates[instance.Vnum]
	return itemproto.SetPacket{
		Position:   position,
		Vnum:       instance.Vnum,
		Count:      uint8(instance.Count),
		Flags:      bootstrapItemFlags(template),
		AntiFlags:  bootstrapItemAntiFlags(template),
		Highlight:  bootstrapItemHighlight(template),
		Sockets:    bootstrapItemSockets(template),
		Attributes: bootstrapItemAttributes(template),
	}, nil
}

func bootstrapItemFlags(template itemcatalog.Template) uint32 {
	var flags uint32
	if template.Refineable {
		flags |= itemproto.ItemFlagRefineable
	}
	if template.Save {
		flags |= itemproto.ItemFlagSave
	}
	if template.Stackable {
		flags |= itemproto.ItemFlagStackable
	}
	if template.SellCountPerGold {
		flags |= itemproto.ItemFlagCountPerGold
	}
	if template.SlowQuery {
		flags |= itemproto.ItemFlagSlowQuery
	}
	if template.Rare {
		flags |= itemproto.ItemFlagRare
	}
	if template.Unique {
		flags |= itemproto.ItemFlagUnique
	}
	if template.MakeCount {
		flags |= itemproto.ItemFlagMakeCount
	}
	if template.Irremovable {
		flags |= itemproto.ItemFlagIrremovable
	}
	if template.ConfirmWhenUse {
		flags |= itemproto.ItemFlagConfirmWhenUse
	}
	if template.QuestUse {
		flags |= itemproto.ItemFlagQuestUse
	}
	if template.QuestUseMultiple {
		flags |= itemproto.ItemFlagQuestUseMultiple
	}
	if template.Log {
		flags |= itemproto.ItemFlagLog
	}
	if template.Applicable {
		flags |= itemproto.ItemFlagApplicable
	}
	return flags
}

func bootstrapItemHighlight(template itemcatalog.Template) uint8 {
	if template.Highlight {
		return 1
	}
	return 0
}

func bootstrapItemSockets(template itemcatalog.Template) [itemproto.ItemSocketCount]int32 {
	return [itemproto.ItemSocketCount]int32(template.Sockets)
}

func bootstrapItemAttributes(template itemcatalog.Template) [itemproto.ItemAttributeCount]itemproto.Attribute {
	var attributes [itemproto.ItemAttributeCount]itemproto.Attribute
	for i, attribute := range template.Attributes {
		attributes[i] = itemproto.Attribute{Type: attribute.Type, Value: attribute.Value}
	}
	return attributes
}

func bootstrapItemAntiFlags(template itemcatalog.Template) uint32 {
	var flags uint32
	if template.AntiFemale {
		flags |= itemproto.AntiFlagFemale
	}
	if template.AntiMale {
		flags |= itemproto.AntiFlagMale
	}
	if template.AntiWarrior {
		flags |= itemproto.AntiFlagWarrior
	}
	if template.AntiAssassin {
		flags |= itemproto.AntiFlagAssassin
	}
	if template.AntiSura {
		flags |= itemproto.AntiFlagSura
	}
	if template.AntiShaman {
		flags |= itemproto.AntiFlagShaman
	}
	if template.AntiEmpireA {
		flags |= itemproto.AntiFlagEmpireA
	}
	if template.AntiEmpireB {
		flags |= itemproto.AntiFlagEmpireB
	}
	if template.AntiEmpireC {
		flags |= itemproto.AntiFlagEmpireC
	}
	if template.AntiDrop {
		flags |= itemproto.AntiFlagDrop
	}
	if template.AntiSell {
		flags |= itemproto.AntiFlagSell
	}
	if template.AntiGive {
		flags |= itemproto.AntiFlagGive
	}
	if template.AntiStack {
		flags |= itemproto.AntiFlagStack
	}
	if template.AntiGet {
		flags |= itemproto.AntiFlagGet
	}
	if template.AntiSave {
		flags |= itemproto.AntiFlagSave
	}
	if template.AntiPKDrop {
		flags |= itemproto.AntiFlagPKDrop
	}
	if template.AntiMyShop {
		flags |= itemproto.AntiFlagMyShop
	}
	if template.AntiSafebox {
		flags |= itemproto.AntiFlagSafebox
	}
	return flags
}

func encodeBootstrapItemUpdateFrame(position itemproto.Position, instance inventory.ItemInstance) ([]byte, error) {
	return encodeBootstrapItemUpdateFrameWithTemplates(position, instance, nil)
}

func encodeBootstrapItemUpdateFrameWithTemplates(position itemproto.Position, instance inventory.ItemInstance, templates map[uint32]itemcatalog.Template) ([]byte, error) {
	if instance.Count > 255 {
		return nil, fmt.Errorf("bootstrap item count exceeds legacy uint8: %d", instance.Count)
	}
	template := templates[instance.Vnum]
	return itemproto.EncodeUpdate(itemproto.UpdatePacket{
		Position:   position,
		Count:      uint8(instance.Count),
		Sockets:    bootstrapItemSockets(template),
		Attributes: bootstrapItemAttributes(template),
	}), nil
}

func encodeBootstrapItemGetFrame(instance inventory.ItemInstance) ([]byte, error) {
	return encodeBootstrapItemGetFrameWithPartyArg(instance, itemproto.GetArgNormal, "")
}

func encodeBootstrapItemGetFrameWithPartyArg(instance inventory.ItemInstance, arg uint8, fromName string) ([]byte, error) {
	if instance.Count > 255 {
		return nil, fmt.Errorf("bootstrap item count exceeds legacy uint8: %d", instance.Count)
	}
	return itemproto.EncodeGet(itemproto.GetPacket{
		Vnum:     instance.Vnum,
		Count:    uint8(instance.Count),
		Arg:      arg,
		FromName: fromName,
	}), nil
}

func equipmentBootstrapPosition(slot inventory.EquipmentSlot) (itemproto.Position, bool) {
	wearIndex, ok := equipmentBootstrapWearIndex(slot)
	if !ok {
		return itemproto.Position{}, false
	}
	position, err := itemproto.EquipmentPosition(wearIndex)
	if err != nil {
		return itemproto.Position{}, false
	}
	return position, true
}

func equipmentBootstrapWearIndex(slot inventory.EquipmentSlot) (uint16, bool) {
	const costumeHairWearIndex uint16 = 20
	var wearIndex uint16
	switch slot {
	case inventory.EquipmentSlotBody:
		wearIndex = 0
	case inventory.EquipmentSlotHead:
		wearIndex = 1
	case inventory.EquipmentSlotShoes:
		wearIndex = 2
	case inventory.EquipmentSlotWrist:
		wearIndex = 3
	case inventory.EquipmentSlotWeapon:
		wearIndex = 4
	case inventory.EquipmentSlotNeck:
		wearIndex = 5
	case inventory.EquipmentSlotEar:
		wearIndex = 6
	case inventory.EquipmentSlotUnique1:
		wearIndex = 7
	case inventory.EquipmentSlotUnique2:
		wearIndex = 8
	case inventory.EquipmentSlotArrow:
		wearIndex = 9
	case inventory.EquipmentSlotShield:
		wearIndex = 10
	case inventory.EquipmentSlotHair:
		wearIndex = costumeHairWearIndex
	default:
		return 0, false
	}
	return wearIndex, true
}

func equipmentBootstrapSlot(wearIndex uint16) (inventory.EquipmentSlot, bool) {
	const costumeHairWearIndex uint16 = 20
	switch wearIndex {
	case 0:
		return inventory.EquipmentSlotBody, true
	case 1:
		return inventory.EquipmentSlotHead, true
	case 2:
		return inventory.EquipmentSlotShoes, true
	case 3:
		return inventory.EquipmentSlotWrist, true
	case 4:
		return inventory.EquipmentSlotWeapon, true
	case 5:
		return inventory.EquipmentSlotNeck, true
	case 6:
		return inventory.EquipmentSlotEar, true
	case 7:
		return inventory.EquipmentSlotUnique1, true
	case 8:
		return inventory.EquipmentSlotUnique2, true
	case 9:
		return inventory.EquipmentSlotArrow, true
	case 10:
		return inventory.EquipmentSlotShield, true
	case costumeHairWearIndex:
		return inventory.EquipmentSlotHair, true
	default:
		return inventory.EquipmentSlotNone, false
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

func (r *gameRuntime) loadInteractionDefinitions() error {
	if r == nil || r.interactionStore == nil {
		return nil
	}
	snapshot, err := r.interactionStore.Load()
	if err != nil {
		if errors.Is(err, interactionstore.ErrSnapshotNotFound) {
			r.interactionDefinitionMu.Lock()
			r.interactionDefinitions = nil
			r.interactionDefinitionMu.Unlock()
			return nil
		}
		return err
	}
	if err := r.validateInteractionDefinitions(snapshot); err != nil {
		return err
	}
	definitions := buildInteractionDefinitionIndex(snapshot)
	r.interactionDefinitionMu.Lock()
	r.interactionDefinitions = definitions
	r.interactionDefinitionMu.Unlock()
	return nil
}

func (r *gameRuntime) loadItemTemplates() error {
	if r == nil || r.itemStore == nil {
		return nil
	}
	snapshot, err := r.itemStore.Load()
	if err != nil {
		if errors.Is(err, itemcatalog.ErrSnapshotNotFound) {
			r.itemTemplates = buildItemTemplateIndex(defaultBootstrapItemTemplateSnapshot())
			r.itemTemplatesAuthored = false
			r.sharedWorld.SetItemTemplates(r.itemTemplates)
			return nil
		}
		return err
	}
	if len(snapshot.Templates) == 0 {
		r.itemTemplates = buildItemTemplateIndex(defaultBootstrapItemTemplateSnapshot())
		r.itemTemplatesAuthored = false
		r.sharedWorld.SetItemTemplates(r.itemTemplates)
		return nil
	}
	r.itemTemplates = buildItemTemplateIndex(snapshot)
	r.itemTemplatesAuthored = true
	r.sharedWorld.SetItemTemplates(r.itemTemplates)
	return nil
}

func (r *gameRuntime) buildItemTemplateSnapshot() itemcatalog.Snapshot {
	if r == nil || !r.itemTemplatesAuthored || len(r.itemTemplates) == 0 {
		return itemcatalog.Snapshot{}
	}
	templates := make([]itemcatalog.Template, 0, len(r.itemTemplates))
	for _, template := range r.itemTemplates {
		templates = append(templates, template)
	}
	return itemcatalog.NormalizeSnapshot(itemcatalog.Snapshot{Templates: templates})
}

func (r *gameRuntime) replaceItemTemplates(snapshot itemcatalog.Snapshot) error {
	if r == nil || r.itemStore == nil {
		return ErrContentBundleUnavailable
	}
	normalized := itemcatalog.NormalizeSnapshot(snapshot)
	if err := r.itemStore.Save(normalized); err != nil {
		return err
	}
	r.itemTemplates = buildItemTemplateIndex(normalized)
	r.itemTemplatesAuthored = len(normalized.Templates) > 0
	if !r.itemTemplatesAuthored {
		r.itemTemplates = buildItemTemplateIndex(defaultBootstrapItemTemplateSnapshot())
	}
	r.sharedWorld.SetItemTemplates(r.itemTemplates)
	return nil
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
	rollbackProfiles, err := registerStaticStoreCombatProfiles(snapshot.CombatProfiles, snapshot.StaticActors)
	if err != nil {
		return err
	}
	loaded := false
	defer func() {
		if !loaded {
			rollbackProfiles()
		}
	}()
	for _, actor := range snapshot.StaticActors {
		if !r.interactionDefinitionExists(actor.InteractionKind, actor.InteractionRef) {
			return fmt.Errorf("%w: validate static actor interaction refs", staticstore.ErrInvalidSnapshot)
		}
		deathReward := worldruntime.StaticActorDeathReward{Experience: actor.RewardExperience, Gold: actor.RewardGold, DropVnums: append([]uint32(nil), actor.RewardDropVnums...)}
		killQuestCredit := staticActorKillQuestCredit{QuestRef: actor.RewardQuestRef, QuestFlag: actor.RewardQuestFlag, QuestFrom: actor.RewardQuestFrom, QuestTo: actor.RewardQuestTo, Text: actor.RewardQuestText, RequireQuestRef: actor.RequireQuestRef, RequireQuestFlag: actor.RequireQuestFlag, RequireQuestFrom: actor.RequireQuestFrom}
		registered, ok := r.sharedWorld.registerStaticActorWithSpawnHomeAndKillQuestCredit(actor.EntityID, actor.Name, actor.MapIndex, actor.X, actor.Y, actor.RaceNum, actor.InteractionKind, actor.InteractionRef, actor.CombatProfile, actor.SpawnGroupRef, actor.SpawnHome, deathReward, killQuestCredit)
		if !ok {
			return fmt.Errorf("%w: apply static actor snapshot", staticstore.ErrInvalidSnapshot)
		}
		if actor.SpawnGroupRef != "" && actor.CombatCurrentHP != nil && actor.RespawnReadyAt != nil && *actor.CombatCurrentHP == 0 && !actor.RespawnReadyAt.IsZero() {
			if !r.sharedWorld.restoreStillDeadSpawnGroupCombatState(registered.EntityID, actor.RespawnReadyAt.UTC()) {
				return fmt.Errorf("%w: restore still-dead spawn-group combat state", staticstore.ErrInvalidSnapshot)
			}
		} else if actor.SpawnGroupRef != "" && actor.CombatCurrentHP != nil && *actor.CombatCurrentHP > 0 && (actor.RespawnReadyAt == nil || actor.RespawnReadyAt.IsZero()) {
			if !r.sharedWorld.restoreDamagedSpawnGroupCombatState(registered.EntityID, *actor.CombatCurrentHP) {
				return fmt.Errorf("%w: restore damaged spawn-group combat state", staticstore.ErrInvalidSnapshot)
			}
		}
		r.syncSpawnGroupReturnStepSchedule(registered)
		r.syncSpawnGroupHomewardStepScheduleForEntity(registered.EntityID)
	}
	loaded = true
	return nil
}

func buildInteractionDefinitionIndex(snapshot interactionstore.Snapshot) map[string]interactionstore.Definition {
	if len(snapshot.Definitions) == 0 {
		return nil
	}
	definitions := make(map[string]interactionstore.Definition, len(snapshot.Definitions))
	for _, definition := range snapshot.Definitions {
		definition = interactionstore.NormalizeDefinition(definition)
		definitions[interactionDefinitionKey(definition.Kind, definition.Ref)] = definition
	}
	return definitions
}

func buildItemTemplateIndex(snapshot itemcatalog.Snapshot) map[uint32]itemcatalog.Template {
	if len(snapshot.Templates) == 0 {
		return nil
	}
	templates := make(map[uint32]itemcatalog.Template, len(snapshot.Templates))
	for _, template := range snapshot.Templates {
		templates[template.Vnum] = template
	}
	return templates
}

func defaultBootstrapItemTemplateSnapshot() itemcatalog.Snapshot {
	return itemcatalog.Snapshot{Templates: []itemcatalog.Template{
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotWeapon.String()},
		{
			Vnum:      12200,
			Name:      "Practice Blade",
			Stackable: false,
			MaxCount:  1,
			EquipSlot: inventory.EquipmentSlotWeapon.String(),
			EquipEffect: &itemcatalog.PointEffect{
				PointType:  bootstrapPlayerPointType,
				PointIndex: bootstrapPlayerPointValueIndex,
				PointDelta: 10,
			},
		},
		{
			Vnum:      12201,
			Name:      "Cursed Practice Blade",
			Stackable: false,
			MaxCount:  1,
			EquipSlot: inventory.EquipmentSlotWeapon.String(),
			EquipEffect: &itemcatalog.PointEffect{
				PointType:  bootstrapPlayerPointType,
				PointIndex: bootstrapPlayerPointValueIndex,
				PointDelta: -10,
			},
		},
		{
			Vnum:         27001,
			Name:         "Small Red Potion",
			Stackable:    true,
			MaxCount:     200,
			ShopBuyPrice: 5,
			UseEffect:    &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27001:+50"},
		},
	}}
}

func (r *gameRuntime) resolveRuntimeUseTemplate(selectedPlayer *player.Runtime, slot inventory.SlotIndex) (itemcatalog.Template, bool) {
	template, ok := r.resolveRuntimeItemTemplate(selectedPlayer, slot)
	if !ok || template.UseEffect == nil {
		return itemcatalog.Template{}, false
	}
	return template, true
}

func (r *gameRuntime) resolveRuntimeItemTemplate(selectedPlayer *player.Runtime, slot inventory.SlotIndex) (itemcatalog.Template, bool) {
	if r == nil || selectedPlayer == nil {
		return itemcatalog.Template{}, false
	}
	for _, item := range selectedPlayer.LiveInventory() {
		if item.Equipped || item.Slot != slot {
			continue
		}
		template, ok := r.itemTemplates[item.Vnum]
		if !ok || !itemcatalog.ValidTemplate(template) {
			return itemcatalog.Template{}, false
		}
		return template, true
	}
	return itemcatalog.Template{}, false
}

func (r *gameRuntime) authoredInventoryMoveSlotCountsFitTemplates(items []inventory.ItemInstance, from inventory.SlotIndex, to inventory.SlotIndex) bool {
	if r == nil || !r.itemTemplatesAuthored || from == to || from >= inventory.CarriedInventorySlotCount || to >= inventory.CarriedInventorySlotCount {
		return true
	}
	sourceFound := false
	targetFound := false
	for _, item := range items {
		if item.Equipped {
			continue
		}
		switch item.Slot {
		case from:
			sourceFound = true
		case to:
			targetFound = true
		}
	}
	if !sourceFound || !targetFound {
		return true
	}
	for _, item := range items {
		if item.Equipped || (item.Slot != from && item.Slot != to) {
			continue
		}
		template, ok := r.itemTemplates[item.Vnum]
		if !ok || !itemcatalog.ValidTemplate(template) || template.Vnum != item.Vnum || item.Count > template.MaxCount {
			return false
		}
	}
	return true
}

func (r *gameRuntime) authoredIncompatibleInventorySwapTemplatesResolve(items []inventory.ItemInstance, from inventory.SlotIndex, to inventory.SlotIndex) bool {
	if r == nil || !r.itemTemplatesAuthored || from == to || from >= inventory.CarriedInventorySlotCount || to >= inventory.CarriedInventorySlotCount {
		return true
	}
	var sourceItem inventory.ItemInstance
	sourceFound := false
	var targetItem inventory.ItemInstance
	targetFound := false
	for _, item := range items {
		if item.Equipped {
			continue
		}
		switch item.Slot {
		case from:
			if sourceFound {
				return true
			}
			sourceItem = item
			sourceFound = true
		case to:
			if targetFound {
				return true
			}
			targetItem = item
			targetFound = true
		}
	}
	if !sourceFound || !targetFound || sourceItem.Vnum == targetItem.Vnum {
		return true
	}
	sourceTemplate, sourceOK := r.itemTemplates[sourceItem.Vnum]
	if !sourceOK || !itemcatalog.ValidTemplate(sourceTemplate) || sourceTemplate.Vnum != sourceItem.Vnum {
		return false
	}
	targetTemplate, targetOK := r.itemTemplates[targetItem.Vnum]
	if !targetOK || !itemcatalog.ValidTemplate(targetTemplate) || targetTemplate.Vnum != targetItem.Vnum {
		return false
	}
	return true
}

func (r *gameRuntime) resolveRuntimeEquipTemplate(selectedPlayer *player.Runtime, slot inventory.SlotIndex, equipSlot inventory.EquipmentSlot) (itemcatalog.Template, bool, bool) {
	if r == nil || selectedPlayer == nil || !equipSlot.Valid() {
		return itemcatalog.Template{}, false, false
	}
	for _, item := range selectedPlayer.LiveInventory() {
		if item.Equipped || item.Slot != slot {
			continue
		}
		template, ok := r.itemTemplates[item.Vnum]
		if !ok || !itemcatalog.ValidTemplate(template) || template.EquipSlot == "" {
			return itemcatalog.Template{}, false, !r.itemTemplatesAuthored
		}
		return template, true, true
	}
	return itemcatalog.Template{}, false, false
}

func (r *gameRuntime) resolveRuntimeUnequipTemplate(selectedPlayer *player.Runtime, equipSlot inventory.EquipmentSlot) (itemcatalog.Template, bool, bool) {
	if r == nil || selectedPlayer == nil || !equipSlot.Valid() {
		return itemcatalog.Template{}, false, false
	}
	for _, item := range selectedPlayer.LiveEquipment() {
		if !item.Equipped || item.EquipSlot != equipSlot {
			continue
		}
		template, ok := r.itemTemplates[item.Vnum]
		if !ok || !itemcatalog.ValidTemplate(template) {
			return itemcatalog.Template{}, false, !r.itemTemplatesAuthored
		}
		if template.EquipSlot == "" {
			return itemcatalog.Template{}, false, !r.itemTemplatesAuthored
		}
		if template.EquipEffect == nil && !template.Irremovable && !r.itemTemplatesAuthored {
			return itemcatalog.Template{}, false, true
		}
		return template, true, true
	}
	return itemcatalog.Template{}, false, false
}

func (r *gameRuntime) resolveRuntimeTemplateBackedEquipEffect(vnum uint32, equipSlot inventory.EquipmentSlot) (itemcatalog.Template, bool) {
	if r == nil || !equipSlot.Valid() {
		return itemcatalog.Template{}, false
	}
	template, ok := r.itemTemplates[vnum]
	if !ok || !itemcatalog.ValidTemplate(template) || template.EquipEffect == nil {
		return itemcatalog.Template{}, false
	}
	if !templateAuthoredForRuntimeEquipSlot(template, equipSlot) {
		return itemcatalog.Template{}, false
	}
	return template, true
}

func runtimeTemplateAllowsEquip(template itemcatalog.Template, selectedPlayer *player.Runtime, equipSlot inventory.EquipmentSlot) bool {
	if selectedPlayer == nil || !selectedPlayer.CanUseTemplate(template) || !templateAuthoredForRuntimeEquipSlot(template, equipSlot) {
		return false
	}
	return !template.AntiStack && !template.AntiGet && !template.AntiDrop && !template.AntiGive && !template.AntiSell
}

func templateAuthoredForRuntimeEquipSlot(template itemcatalog.Template, equipSlot inventory.EquipmentSlot) bool {
	if !equipSlot.Valid() || !itemcatalog.ValidTemplate(template) || template.EquipSlot == "" {
		return false
	}
	templateSlot, ok := inventory.ParseEquipmentSlot(template.EquipSlot)
	return ok && templateSlot == equipSlot
}

func (r *gameRuntime) validateInteractionDefinitions(snapshot interactionstore.Snapshot) error {
	for _, definition := range snapshot.Definitions {
		if err := r.validateInteractionDefinition(interactionstore.NormalizeDefinition(definition)); err != nil {
			return err
		}
	}
	return nil
}

func (r *gameRuntime) validateInteractionDefinition(definition interactionstore.Definition) error {
	if !interactionstore.ValidDefinition(definition) {
		return interactionstore.ErrInvalidSnapshot
	}
	if definition.Kind != interactionstore.KindShopPreview {
		return nil
	}
	for _, entry := range definition.Catalog {
		template, ok := r.itemTemplates[entry.ItemVnum]
		if !ok {
			return interactionstore.ErrInvalidSnapshot
		}
		if template.Stackable {
			if entry.Count > template.MaxCount {
				return interactionstore.ErrInvalidSnapshot
			}
			continue
		}
		if entry.Count != 1 {
			return interactionstore.ErrInvalidSnapshot
		}
	}
	return nil
}

func interactionDefinitionKey(kind string, ref string) string {
	return strings.TrimSpace(kind) + "\x00" + strings.TrimSpace(ref)
}

func (r *gameRuntime) ResolveInteractionDefinition(kind string, ref string) (InteractionDefinition, bool) {
	if r == nil || r.interactionStore == nil {
		return InteractionDefinition{}, false
	}
	r.interactionDefinitionMu.RLock()
	defer r.interactionDefinitionMu.RUnlock()
	definition, ok := r.interactionDefinitions[interactionDefinitionKey(kind, ref)]
	if !ok {
		return InteractionDefinition{}, false
	}
	return definition, true
}

func (r *gameRuntime) InteractionDefinitions() []InteractionDefinition {
	if r == nil || r.interactionStore == nil {
		return nil
	}
	r.interactionDefinitionMu.RLock()
	defer r.interactionDefinitionMu.RUnlock()
	return sortedInteractionDefinitions(r.interactionDefinitions)
}

func (r *gameRuntime) InteractionDefinition(kind string, ref string) (InteractionDefinition, bool) {
	return r.ResolveInteractionDefinition(kind, ref)
}

func (r *gameRuntime) ExportContentBundle() (contentbundle.Bundle, error) {
	if r == nil || r.staticStore == nil || r.interactionStore == nil {
		return contentbundle.Bundle{}, ErrContentBundleUnavailable
	}
	bundle, err := contentbundle.FromSnapshotsWithItems(buildStaticActorStoreSnapshot(r.StaticActors()), buildInteractionDefinitionSnapshot(r.interactionDefinitions), r.buildItemTemplateSnapshot())
	if err != nil {
		return contentbundle.Bundle{}, err
	}
	questState, err := r.loadQuestStateForContentBundle()
	if err != nil {
		return contentbundle.Bundle{}, err
	}
	bundle.QuestState = questState.Flags
	return contentbundle.Canonicalize(bundle)
}

func (r *gameRuntime) loadQuestStateForContentBundle() (queststate.Snapshot, error) {
	if r == nil || r.questStateStore == nil {
		return queststate.Snapshot{Flags: []queststate.Flag{}}, nil
	}
	r.questStateMu.Lock()
	defer r.questStateMu.Unlock()
	snapshot, err := r.questStateStore.Load()
	if err != nil {
		if errors.Is(err, queststate.ErrSnapshotNotFound) {
			return queststate.Snapshot{Flags: []queststate.Flag{}}, nil
		}
		return queststate.Snapshot{}, err
	}
	return snapshot, nil
}

func (r *gameRuntime) replaceQuestStateFromBundle(snapshot queststate.Snapshot) error {
	if r == nil || r.questStateStore == nil {
		return nil
	}
	r.questStateMu.Lock()
	defer r.questStateMu.Unlock()
	return r.questStateStore.Save(snapshot)
}

func (r *gameRuntime) ExportContentBundleSummary() (contentbundle.Summary, error) {
	bundle, err := r.ExportContentBundle()
	if err != nil {
		return contentbundle.Summary{}, err
	}
	return contentbundle.Summarize(bundle)
}

func (r *gameRuntime) PreviewContentBundleImport(candidate contentbundle.Bundle) (contentbundle.ImportPreview, error) {
	current, err := r.ExportContentBundle()
	if err != nil {
		return contentbundle.ImportPreview{}, err
	}
	return contentbundle.BuildImportPreview(current, candidate)
}

func (r *gameRuntime) ImportContentBundle(bundle contentbundle.Bundle) (contentbundle.Bundle, error) {
	if r == nil || r.staticStore == nil || r.interactionStore == nil {
		return contentbundle.Bundle{}, ErrContentBundleUnavailable
	}
	referencedProfiles := contentBundleReferencedCombatProfiles(bundle)
	rollbackProfiles, err := registerContentBundleCombatProfiles(bundle.CombatProfiles, referencedProfiles)
	if err != nil {
		return contentbundle.Bundle{}, err
	}
	normalized, err := contentbundle.Canonicalize(bundle)
	if err != nil {
		rollbackProfiles()
		return contentbundle.Bundle{}, err
	}
	previousBundle, err := r.ExportContentBundle()
	if err != nil {
		rollbackProfiles()
		return contentbundle.Bundle{}, err
	}
	if reflect.DeepEqual(previousBundle, normalized) {
		r.pruneSpawnGroupReturnStepSchedules()
		r.pruneSpawnGroupChaseStepSchedules()
		r.pruneSpawnGroupHomewardStepSchedules()
		return normalized, nil
	}
	previousActors := r.StaticActors()
	previousSpawnReturnStepDueAt := r.spawnGroupReturnStepDueAtSnapshot()
	previousSpawnChaseStepDueAt := r.spawnGroupChaseStepDueAtSnapshot()
	previousSpawnHomewardStepDueAt := r.spawnGroupHomewardStepDueAtSnapshot()
	var previousCombatState staticActorCombatStateSnapshot
	if r.sharedWorld != nil {
		r.sharedWorld.mu.Lock()
		previousCombatState = r.sharedWorld.captureStaticActorCombatStateLocked()
		r.sharedWorld.mu.Unlock()
	}
	if err := r.replaceItemTemplates(itemcatalog.Snapshot{Templates: normalized.ItemTemplates}); err != nil {
		rollbackProfiles()
		return contentbundle.Bundle{}, err
	}
	if err := r.replaceInteractionDefinitions(interactionstore.Snapshot{Definitions: normalized.InteractionDefinitions}); err != nil {
		_ = r.replaceItemTemplates(itemcatalog.Snapshot{Templates: previousBundle.ItemTemplates})
		rollbackProfiles()
		return contentbundle.Bundle{}, err
	}
	if r.sharedWorld != nil {
		r.sharedWorld.suppressStaticActorFanout = true
	}
	replaceErr := r.replaceStaticActorsFromBundle(normalized)
	if r.sharedWorld != nil {
		r.sharedWorld.suppressStaticActorFanout = false
	}
	if replaceErr == nil {
		replaceErr = r.replaceQuestStateFromBundle(queststate.Snapshot{Flags: normalized.QuestState})
	}
	if replaceErr != nil {
		if r.sharedWorld != nil {
			r.sharedWorld.suppressStaticActorFanout = true
			r.sharedWorld.clearStaticActorsForContentImportRollback()
		}
		rollbackErr := r.replaceItemTemplates(itemcatalog.Snapshot{Templates: previousBundle.ItemTemplates})
		rollbackErr = errors.Join(rollbackErr, r.replaceInteractionDefinitions(interactionstore.Snapshot{Definitions: previousBundle.InteractionDefinitions}))
		rollbackErr = errors.Join(rollbackErr, r.replaceQuestStateFromBundle(queststate.Snapshot{Flags: previousBundle.QuestState}))
		if r.sharedWorld != nil {
			for _, actor := range previousActors {
				_, ok := r.sharedWorld.registerStaticActorWithSpawnHomeAndKillQuestCredit(actor.EntityID, actor.Name, actor.MapIndex, actor.X, actor.Y, actor.RaceNum, actor.InteractionKind, actor.InteractionRef, actor.CombatProfile, actor.SpawnGroupRef, actor.SpawnHome, worldruntime.StaticActorDeathReward{Experience: actor.RewardExperience, Gold: actor.RewardGold, DropVnums: append([]uint32(nil), actor.RewardDropVnums...)}, staticActorKillQuestCredit{QuestRef: actor.RewardQuestRef, QuestFlag: actor.RewardQuestFlag, QuestFrom: actor.RewardQuestFrom, QuestTo: actor.RewardQuestTo, Text: actor.RewardQuestText, RequireQuestRef: actor.RequireQuestRef, RequireQuestFlag: actor.RequireQuestFlag, RequireQuestFrom: actor.RequireQuestFrom})
				if !ok {
					rollbackErr = errors.Join(rollbackErr, ErrContentBundleUnavailable)
				}
			}
			r.sharedWorld.mu.Lock()
			r.sharedWorld.restoreStaticActorCombatStateLocked(previousCombatState)
			r.sharedWorld.mu.Unlock()
			r.sharedWorld.suppressStaticActorFanout = false
		}
		if !r.persistStaticActorSnapshot(previousActors) {
			rollbackErr = errors.Join(rollbackErr, ErrContentBundleUnavailable)
		}
		r.restoreSpawnGroupReturnStepDueAtSnapshot(previousSpawnReturnStepDueAt)
		r.restoreSpawnGroupChaseStepDueAtSnapshot(previousSpawnChaseStepDueAt)
		r.restoreSpawnGroupHomewardStepDueAtSnapshot(previousSpawnHomewardStepDueAt)
		if r.sharedWorld != nil {
			r.sharedWorld.discardStaticActorImportFanout()
		}
		rollbackProfiles()
		if rollbackErr != nil {
			return contentbundle.Bundle{}, errors.Join(replaceErr, rollbackErr)
		}
		return contentbundle.Bundle{}, replaceErr
	}
	if r.sharedWorld != nil {
		r.sharedWorld.remapSpawnGroupCombatState(previousActors, previousCombatState)
	}
	r.pruneSpawnGroupReturnStepSchedules()
	r.pruneSpawnGroupChaseStepSchedules()
	r.pruneSpawnGroupHomewardStepSchedules()
	if r.sharedWorld != nil {
		r.sharedWorld.flushStaticActorImportFanout()
	}
	if !r.persistStaticActorSnapshot(r.StaticActors()) {
		return contentbundle.Bundle{}, ErrContentBundleUnavailable
	}
	return normalized, nil
}

func contentBundleCombatProfileSnapshotMatchesDefaults(snapshot worldruntime.StaticActorCombatProfileSnapshot, defaults worldruntime.StaticActorCombatProfileDefaults) bool {
	normalized, ok := contentBundleCombatProfileSnapshotDefaults(snapshot)
	return ok &&
		normalized.MaxHP == defaults.MaxHP &&
		normalized.DamagePerNormalAttack == defaults.DamagePerNormalAttack &&
		normalized.AttackValue == defaults.AttackValue &&
		normalized.DefenseValue == defaults.DefenseValue &&
		normalized.Level == defaults.Level &&
		normalized.Rank == defaults.Rank &&
		normalized.RespawnDelay == defaults.RespawnDelay &&
		normalized.AggroRadius == defaults.AggroRadius &&
		normalized.LeashRadius == defaults.LeashRadius &&
		normalized.RetaliationPointDelta == defaults.RetaliationPointDelta &&
		reflect.DeepEqual(normalized.DeathReward.Clone(), defaults.DeathReward.Clone())
}

func contentBundleCombatProfileSnapshotDefaults(snapshot worldruntime.StaticActorCombatProfileSnapshot) (worldruntime.StaticActorCombatProfileDefaults, bool) {
	respawnDelay, ok := worldruntime.StaticActorCombatProfileRespawnDelay(snapshot.RespawnDelayMs)
	if strings.TrimSpace(snapshot.Profile) == "" || snapshot.MaxHP == 0 || snapshot.AttackValue == 0 || !ok {
		return worldruntime.StaticActorCombatProfileDefaults{}, false
	}
	if !worldruntime.ValidStaticActorCombatProfileAggroRadius(snapshot.AggroRadius, snapshot.LeashRadius) {
		return worldruntime.StaticActorCombatProfileDefaults{}, false
	}
	if !worldruntime.ValidStaticActorCombatProfileLeashRadius(snapshot.LeashRadius, snapshot.AggroRadius) {
		return worldruntime.StaticActorCombatProfileDefaults{}, false
	}
	defaults := worldruntime.StaticActorCombatProfileDefaults{
		MaxHP:                 snapshot.MaxHP,
		DamagePerNormalAttack: snapshot.DamagePerNormalAttack,
		AttackValue:           snapshot.AttackValue,
		DefenseValue:          snapshot.DefenseValue,
		Level:                 snapshot.Level,
		Rank:                  snapshot.Rank,
		RespawnDelay:          respawnDelay,
		AggroRadius:           snapshot.AggroRadius,
		LeashRadius:           snapshot.LeashRadius,
		RetaliationPointDelta: snapshot.RetaliationPointDelta,
		DeathReward:           snapshot.DeathReward.Clone(),
	}
	if defaults.DamagePerNormalAttack == 0 {
		defaults.DamagePerNormalAttack = contentBundleCombatProfileSnapshotFormulaDamage(snapshot)
	}
	if defaults.Level == 0 {
		defaults.Level = worldruntime.TrainingDummyBootstrapLevel
	}
	if defaults.RetaliationPointDelta == 0 {
		defaults.RetaliationPointDelta = worldruntime.PracticeMobBootstrapRetaliationPointDelta
	}
	return defaults, true
}

func contentBundleCombatProfileSnapshotFormulaDamage(snapshot worldruntime.StaticActorCombatProfileSnapshot) uint8 {
	if snapshot.AttackValue <= snapshot.DefenseValue {
		return 1
	}
	damage := snapshot.AttackValue - snapshot.DefenseValue
	if damage == 0 {
		return 1
	}
	if damage > uint16(snapshot.MaxHP) {
		return snapshot.MaxHP
	}
	return uint8(damage)
}

func contentBundleReferencedCombatProfiles(bundle contentbundle.Bundle) map[string]struct{} {
	referenced := make(map[string]struct{}, len(bundle.StaticActors)+len(bundle.SpawnGroups)+len(bundle.RegenSpawns))
	for _, actor := range bundle.StaticActors {
		profile := strings.TrimSpace(actor.CombatProfile)
		if profile != "" {
			referenced[profile] = struct{}{}
		}
	}
	for _, spawnGroup := range bundle.SpawnGroups {
		profile := strings.TrimSpace(spawnGroup.CombatProfile)
		if profile != "" {
			referenced[profile] = struct{}{}
		}
	}
	for _, regenSpawn := range bundle.RegenSpawns {
		profile := strings.TrimSpace(regenSpawn.CombatProfile)
		if profile != "" {
			referenced[profile] = struct{}{}
		}
	}
	return referenced
}

func registerContentBundleCombatProfiles(profiles []worldruntime.StaticActorCombatProfileSnapshot, referencedProfiles map[string]struct{}) (func(), error) {
	registered := make([]string, 0, len(profiles))
	seen := make(map[string]struct{}, len(profiles))
	rollback := func() {
		for i := len(registered) - 1; i >= 0; i-- {
			worldruntime.UnregisterStaticActorCombatProfile(registered[i])
		}
	}
	for _, snapshot := range profiles {
		profile := snapshot.Profile
		if !worldruntime.ValidStaticActorCombatProfileName(profile) || profile == worldruntime.StaticActorCombatProfilePracticeMob || profile == worldruntime.StaticActorCombatProfileTrainingDummy {
			rollback()
			return nil, contentbundle.ErrInvalidBundle
		}
		if _, referenced := referencedProfiles[profile]; !referenced {
			rollback()
			return nil, contentbundle.ErrInvalidBundle
		}
		if _, exists := seen[profile]; exists {
			rollback()
			return nil, contentbundle.ErrInvalidBundle
		}
		seen[profile] = struct{}{}
		if worldruntime.ValidStaticActorCombatProfile(profile) {
			existing, ok := worldruntime.BootstrapStaticActorCombatProfileDefaults(profile)
			if !ok || !contentBundleCombatProfileSnapshotMatchesDefaults(snapshot, existing) {
				rollback()
				return nil, contentbundle.ErrInvalidBundle
			}
			continue
		}
		respawnDelay, ok := worldruntime.StaticActorCombatProfileRespawnDelay(snapshot.RespawnDelayMs)
		if snapshot.MaxHP == 0 || snapshot.AttackValue == 0 || !ok {
			rollback()
			return nil, contentbundle.ErrInvalidBundle
		}
		if snapshot.RetaliationPointDelta > 0 {
			rollback()
			return nil, contentbundle.ErrInvalidBundle
		}
		if !worldruntime.ValidStaticActorCombatProfileAggroRadius(snapshot.AggroRadius, snapshot.LeashRadius) {
			rollback()
			return nil, contentbundle.ErrInvalidBundle
		}
		if !worldruntime.ValidStaticActorCombatProfileLeashRadius(snapshot.LeashRadius, snapshot.AggroRadius) {
			rollback()
			return nil, contentbundle.ErrInvalidBundle
		}
		if !worldruntime.RegisterStaticActorCombatProfile(profile, worldruntime.StaticActorCombatProfileDefaults{
			MaxHP:                 snapshot.MaxHP,
			DamagePerNormalAttack: snapshot.DamagePerNormalAttack,
			AttackValue:           snapshot.AttackValue,
			DefenseValue:          snapshot.DefenseValue,
			Level:                 snapshot.Level,
			Rank:                  snapshot.Rank,
			RespawnDelay:          respawnDelay,
			AggroRadius:           snapshot.AggroRadius,
			LeashRadius:           snapshot.LeashRadius,
			RetaliationPointDelta: snapshot.RetaliationPointDelta,
			DeathReward:           snapshot.DeathReward,
		}) {
			rollback()
			return nil, contentbundle.ErrInvalidBundle
		}
		registered = append(registered, profile)
	}
	return rollback, nil
}

func registerStaticStoreCombatProfiles(profiles []worldruntime.StaticActorCombatProfileSnapshot, actors []staticstore.StaticActor) (func(), error) {
	referencedProfiles := make(map[string]struct{}, len(actors))
	for _, actor := range actors {
		profile := strings.TrimSpace(actor.CombatProfile)
		if profile != "" {
			referencedProfiles[profile] = struct{}{}
		}
	}
	return registerContentBundleCombatProfiles(profiles, referencedProfiles)
}

func (r *gameRuntime) CreateInteractionDefinition(definition InteractionDefinition) (InteractionDefinition, error) {
	if r == nil || r.interactionStore == nil {
		return InteractionDefinition{}, ErrInteractionDefinitionsUnavailable
	}
	definition = interactionstore.NormalizeDefinition(definition)
	if err := r.validateInteractionDefinition(definition); err != nil {
		return InteractionDefinition{}, err
	}
	key := interactionDefinitionKey(definition.Kind, definition.Ref)

	r.interactionDefinitionMu.Lock()
	defer r.interactionDefinitionMu.Unlock()
	if _, ok := r.interactionDefinitions[key]; ok {
		return InteractionDefinition{}, ErrInteractionDefinitionExists
	}
	snapshot := buildInteractionDefinitionSnapshot(r.interactionDefinitions)
	snapshot.Definitions = append(snapshot.Definitions, definition)
	if err := r.interactionStore.Save(snapshot); err != nil {
		return InteractionDefinition{}, err
	}
	if r.interactionDefinitions == nil {
		r.interactionDefinitions = make(map[string]interactionstore.Definition)
	}
	r.interactionDefinitions[key] = definition
	return definition, nil
}

func (r *gameRuntime) UpsertInteractionDefinition(definition InteractionDefinition) (InteractionDefinition, error) {
	if r == nil || r.interactionStore == nil {
		return InteractionDefinition{}, ErrInteractionDefinitionsUnavailable
	}
	definition = interactionstore.NormalizeDefinition(definition)
	if err := r.validateInteractionDefinition(definition); err != nil {
		return InteractionDefinition{}, err
	}
	key := interactionDefinitionKey(definition.Kind, definition.Ref)

	r.interactionDefinitionMu.Lock()
	defer r.interactionDefinitionMu.Unlock()
	next := make(map[string]interactionstore.Definition, len(r.interactionDefinitions)+1)
	for existingKey, existingDefinition := range r.interactionDefinitions {
		next[existingKey] = existingDefinition
	}
	next[key] = definition
	if err := r.interactionStore.Save(buildInteractionDefinitionSnapshot(next)); err != nil {
		return InteractionDefinition{}, err
	}
	r.interactionDefinitions = next
	return definition, nil
}

func (r *gameRuntime) RemoveInteractionDefinition(kind string, ref string) (InteractionDefinition, error) {
	if r == nil || r.interactionStore == nil {
		return InteractionDefinition{}, ErrInteractionDefinitionsUnavailable
	}
	kind = strings.TrimSpace(kind)
	ref = strings.TrimSpace(ref)
	if kind == "" || ref == "" {
		return InteractionDefinition{}, interactionstore.ErrInvalidSnapshot
	}

	r.staticActorMu.Lock()
	defer r.staticActorMu.Unlock()
	r.interactionDefinitionMu.Lock()
	defer r.interactionDefinitionMu.Unlock()

	key := interactionDefinitionKey(kind, ref)
	definition, ok := r.interactionDefinitions[key]
	if !ok {
		return InteractionDefinition{}, ErrInteractionDefinitionNotFound
	}
	if interactionDefinitionReferencedByStaticActor(r.sharedWorld.StaticActors(), kind, ref) {
		return InteractionDefinition{}, ErrInteractionDefinitionReferenced
	}
	next := make(map[string]interactionstore.Definition, len(r.interactionDefinitions)-1)
	for existingKey, existingDefinition := range r.interactionDefinitions {
		if existingKey == key {
			continue
		}
		next[existingKey] = existingDefinition
	}
	if err := r.interactionStore.Save(buildInteractionDefinitionSnapshot(next)); err != nil {
		return InteractionDefinition{}, err
	}
	if len(next) == 0 {
		r.interactionDefinitions = nil
	} else {
		r.interactionDefinitions = next
	}
	return definition, nil
}

func buildInteractionDefinitionSnapshot(definitions map[string]interactionstore.Definition) interactionstore.Snapshot {
	return interactionstore.Snapshot{Definitions: sortedInteractionDefinitions(definitions)}
}

func (r *gameRuntime) replaceInteractionDefinitions(snapshot interactionstore.Snapshot) error {
	if r == nil || r.interactionStore == nil {
		return ErrInteractionDefinitionsUnavailable
	}
	if err := r.validateInteractionDefinitions(snapshot); err != nil {
		return err
	}
	if err := r.interactionStore.Save(snapshot); err != nil {
		return err
	}
	definitions := buildInteractionDefinitionIndex(snapshot)
	r.interactionDefinitionMu.Lock()
	r.interactionDefinitions = definitions
	r.interactionDefinitionMu.Unlock()
	return nil
}

func (r *gameRuntime) replaceStaticActorsFromBundle(bundle contentbundle.Bundle) error {
	if r == nil {
		return ErrContentBundleUnavailable
	}
	for _, actor := range r.StaticActors() {
		if _, ok := r.RemoveStaticActor(actor.EntityID); !ok {
			return ErrContentBundleUnavailable
		}
	}
	for _, actor := range bundle.StaticActors {
		if _, ok := r.RegisterStaticActorWithInteractionAndCombatProfile(actor.Name, actor.MapIndex, actor.X, actor.Y, actor.RaceNum, actor.InteractionKind, actor.InteractionRef, actor.CombatProfile); !ok {
			return ErrContentBundleUnavailable
		}
	}
	for _, spawnGroup := range bundle.SpawnGroups {
		deathReward := worldruntime.StaticActorDeathReward{Experience: spawnGroup.RewardExperience, Gold: spawnGroup.RewardGold, DropVnums: append([]uint32(nil), spawnGroup.RewardDropVnums...)}
		killQuestCredit := staticActorKillQuestCredit{QuestRef: spawnGroup.RewardQuestRef, QuestFlag: spawnGroup.RewardQuestFlag, QuestFrom: spawnGroup.RewardQuestFrom, QuestTo: spawnGroup.RewardQuestTo, Text: spawnGroup.RewardQuestText, RequireQuestRef: spawnGroup.RequireQuestRef, RequireQuestFlag: spawnGroup.RequireQuestFlag, RequireQuestFrom: spawnGroup.RequireQuestFrom}
		spawnHome := worldruntime.PositionSnapshot{MapIndex: spawnGroup.MapIndex, X: spawnGroup.X, Y: spawnGroup.Y}
		if _, ok := r.registerStaticActorWithInteractionCombatProfileSpawnGroupRefHomeRewardAndKillQuestCredit(spawnGroup.Name, spawnGroup.MapIndex, spawnGroup.X, spawnGroup.Y, spawnGroup.RaceNum, "", "", spawnGroup.CombatProfile, spawnGroup.Ref, &spawnHome, deathReward, killQuestCredit); !ok {
			return ErrContentBundleUnavailable
		}
	}
	return nil
}

func sortedInteractionDefinitions(definitions map[string]interactionstore.Definition) []InteractionDefinition {
	if len(definitions) == 0 {
		return nil
	}
	ordered := make([]InteractionDefinition, 0, len(definitions))
	for _, definition := range definitions {
		ordered = append(ordered, definition)
	}
	sort.Slice(ordered, func(i int, j int) bool {
		if ordered[i].Kind == ordered[j].Kind {
			return ordered[i].Ref < ordered[j].Ref
		}
		return ordered[i].Kind < ordered[j].Kind
	})
	return ordered
}

func interactionDefinitionReferencedByStaticActor(actors []StaticActorSnapshot, kind string, ref string) bool {
	for _, actor := range actors {
		if actor.InteractionKind == kind && actor.InteractionRef == ref {
			return true
		}
	}
	return false
}

func (r *gameRuntime) resolveStaticActorInteraction(subjectID uint64, targetVID uint32) staticActorInteractionResolution {
	resolution := staticActorInteractionResolution{TargetVID: targetVID}
	if r == nil || r.sharedWorld == nil {
		resolution.Failure = StaticActorInteractionFailureSubjectNotFound
		resolution.Delivery = staticActorInteractionFailureDelivery(resolution.Failure)
		return resolution
	}
	attempt := r.sharedWorld.AttemptStaticActorInteraction(subjectID, targetVID)
	resolution.Actor = attempt.Actor
	if !attempt.Accepted {
		resolution.Failure = attempt.Failure
		resolution.Delivery = staticActorInteractionFailureDelivery(resolution.Failure)
		return resolution
	}
	definition, ok := r.ResolveInteractionDefinition(attempt.Actor.InteractionKind, attempt.Actor.InteractionRef)
	if !ok {
		resolution.Failure = staticActorInteractionFailureDefinitionNotFound
		resolution.Delivery = staticActorInteractionFailureDelivery(resolution.Failure)
		return resolution
	}
	resolution.Definition = definition
	characterName := ""
	if subject, ok := r.sharedWorld.playerCharacter(subjectID); ok {
		characterName = subject.Name
	}
	if definition.Kind == interactionstore.KindWarp {
		if !interactionstore.ValidDefinition(definition) {
			resolution.Failure = staticActorInteractionFailureWarpDestinationInvalid
			resolution.Delivery = staticActorInteractionFailureDelivery(resolution.Failure)
			return resolution
		}
		if ok, err := r.serviceQuestGateSatisfied(characterName, definition); err != nil {
			resolution.Failure = staticActorInteractionFailureUnsupportedKind
			resolution.Delivery = staticActorInteractionFailureDelivery(resolution.Failure)
			return resolution
		} else if !ok {
			resolution.Failure = staticActorInteractionFailureQuestCurrentValueMismatch
			resolution.Delivery = staticActorInteractionFailureDelivery(resolution.Failure)
			return resolution
		}
		resolution.Accepted = true
		message := strings.TrimSpace(definition.Text)
		if message != "" {
			delivery := chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: message}
			resolution.Delivery = &delivery
		}
		return resolution
	}
	if definition.Kind == interactionstore.KindQuestFlag {
		if !interactionstore.ValidDefinition(definition) {
			resolution.Failure = staticActorInteractionFailureUnsupportedKind
			resolution.Delivery = staticActorInteractionFailureDelivery(resolution.Failure)
			return resolution
		}
		resolution.Accepted = true
		delivery := chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: definition.Text}
		resolution.Delivery = &delivery
		return resolution
	}
	if definition.Kind == interactionstore.KindShopPreview {
		if !interactionstore.ValidDefinition(definition) {
			resolution.Failure = staticActorInteractionFailureUnsupportedKind
			resolution.Delivery = staticActorInteractionFailureDelivery(resolution.Failure)
			return resolution
		}
		if ok, err := r.serviceQuestGateSatisfied(characterName, definition); err != nil {
			resolution.Failure = staticActorInteractionFailureUnsupportedKind
			resolution.Delivery = staticActorInteractionFailureDelivery(resolution.Failure)
			return resolution
		} else if !ok {
			resolution.Failure = staticActorInteractionFailureQuestCurrentValueMismatch
			resolution.Delivery = staticActorInteractionFailureDelivery(resolution.Failure)
			return resolution
		}
	}
	if definition.Kind == interactionstore.KindOpenSafebox {
		if !interactionstore.ValidDefinition(definition) {
			resolution.Failure = staticActorInteractionFailureUnsupportedKind
			resolution.Delivery = staticActorInteractionFailureDelivery(resolution.Failure)
			return resolution
		}
		if ok, err := r.serviceQuestGateSatisfied(characterName, definition); err != nil {
			resolution.Failure = staticActorInteractionFailureUnsupportedKind
			resolution.Delivery = staticActorInteractionFailureDelivery(resolution.Failure)
			return resolution
		} else if !ok {
			resolution.Failure = staticActorInteractionFailureQuestCurrentValueMismatch
			resolution.Delivery = staticActorInteractionFailureDelivery(resolution.Failure)
			return resolution
		}
		resolution.Accepted = true
		message := strings.TrimSpace(definition.Text)
		if message != "" {
			delivery := chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: message}
			resolution.Delivery = &delivery
		}
		return resolution
	}
	if definition.Kind == interactionstore.KindInfo || definition.Kind == interactionstore.KindTalk {
		if !interactionstore.ValidDefinition(definition) {
			resolution.Failure = staticActorInteractionFailureUnsupportedKind
			resolution.Delivery = staticActorInteractionFailureDelivery(resolution.Failure)
			return resolution
		}
		if ok, err := r.serviceQuestGateSatisfied(characterName, definition); err != nil {
			resolution.Failure = staticActorInteractionFailureUnsupportedKind
			resolution.Delivery = staticActorInteractionFailureDelivery(resolution.Failure)
			return resolution
		} else if !ok {
			resolution.Failure = staticActorInteractionFailureQuestCurrentValueMismatch
			resolution.Delivery = staticActorInteractionFailureDelivery(resolution.Failure)
			return resolution
		}
	}
	preview, ok := r.interactionDefinitionPreview(attempt.Actor.Name, definition)
	if !ok {
		resolution.Failure = staticActorInteractionFailureUnsupportedKind
		resolution.Delivery = staticActorInteractionFailureDelivery(resolution.Failure)
		return resolution
	}
	delivery := chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: preview}
	resolution.Accepted = true
	resolution.Delivery = &delivery
	return resolution
}

func (r *gameRuntime) serviceQuestGateSatisfied(characterName string, definition InteractionDefinition) (bool, error) {
	if !interactionstore.HasServiceQuestGate(definition) {
		return true, nil
	}
	characterName = strings.TrimSpace(characterName)
	if characterName == "" {
		return false, fmt.Errorf("service quest gate requires a selected character")
	}
	flag, ok, err := r.QuestStateFlag(characterName, definition.QuestRef, definition.QuestFlag)
	if err != nil {
		return false, err
	}
	current := uint32(0)
	if ok {
		current = flag.Value
	}
	return current == definition.QuestFrom, nil
}

func (r *gameRuntime) killQuestRequireGateSatisfied(characterName string, credit staticActorKillQuestCredit) (bool, error) {
	if !credit.HasRequireGate() {
		return true, nil
	}
	characterName = strings.TrimSpace(characterName)
	if characterName == "" {
		return false, fmt.Errorf("kill quest require gate requires a selected character")
	}
	flag, ok, err := r.QuestStateFlag(characterName, credit.RequireQuestRef, credit.RequireQuestFlag)
	if err != nil {
		return false, err
	}
	current := uint32(0)
	if ok {
		current = flag.Value
	}
	return current == credit.RequireQuestFrom, nil
}

func (r *gameRuntime) resolveStaticActorCombatTarget(subjectID uint64, targetVID uint32) staticActorCombatTargetResolution {
	resolution := staticActorCombatTargetResolution{TargetVID: targetVID}
	if r == nil || r.sharedWorld == nil {
		resolution.Failure = StaticActorCombatTargetFailureSubjectNotFound
		return resolution
	}
	attempt := r.sharedWorld.AttemptStaticActorCombatTarget(subjectID, targetVID)
	resolution.Actor = attempt.Actor
	resolution.SnapshotVersion = attempt.SnapshotVersion
	if !attempt.Accepted {
		resolution.Failure = attempt.Failure
		return resolution
	}
	packet := combatproto.ServerTargetPacket{TargetVID: attempt.TargetVID, HPPercent: attempt.HPPercent}
	resolution.Accepted = true
	resolution.Packet = &packet
	return resolution
}

func (r *gameRuntime) resolveSelectedStaticActorNormalAttack(subjectID uint64, activeTargetVID uint32, activeTargetSnapshotVersion uint64, requestedTargetVID uint32) staticActorCombatAttackResolution {
	resolution := staticActorCombatAttackResolution{ActiveTargetVID: activeTargetVID, ActiveTargetSnapshotVersion: activeTargetSnapshotVersion, RequestedTargetVID: requestedTargetVID}
	if r == nil || r.sharedWorld == nil {
		resolution.Failure = StaticActorCombatAttackFailureSubjectNotFound
		return resolution
	}
	attempt := r.sharedWorld.AttemptSelectedStaticActorAttack(subjectID, activeTargetVID, activeTargetSnapshotVersion, requestedTargetVID)
	resolution.Actor = attempt.Actor
	if !attempt.Accepted {
		resolution.Failure = attempt.Failure
		return resolution
	}
	resolution.Accepted = true
	resolution.Damage = attempt.Damage
	if attempt.Died {
		resolution.ClearActiveTarget = true
		resolution.DeathReward = attempt.DeathReward
		resolution.Frames = [][]byte{
			worldproto.EncodeDead(worldproto.DeadPacket{VID: activeTargetVID}),
			combatproto.EncodeServerClearTarget(),
		}
		if attempt.Actor.SpawnGroupRef != "" {
			_ = r.persistSpawnGroupCombatState(attempt.Actor.EntityID)
		}
		return resolution
	}
	if attempt.Actor.SpawnGroupRef != "" {
		_ = r.persistSpawnGroupCombatState(attempt.Actor.EntityID)
	}
	packet := combatproto.ServerTargetPacket{TargetVID: activeTargetVID, HPPercent: attempt.HPPercent}
	resolution.Packet = &packet
	damageInfoFrame := combatproto.EncodeServerDamageInfo(combatproto.ServerDamageInfoPacket{VID: activeTargetVID, Flag: 0, Damage: int32(attempt.Damage)})
	if staticActorSpawnBackedSelfDamageInfoRuntimeEmissionOwned(attempt.Actor) {
		resolution.Frames = [][]byte{
			combatproto.EncodeServerTarget(packet),
		}
		resolution.SelfPostMutationFrames = [][]byte{damageInfoFrame}
		resolution.PeerPostMutationFrames = [][]byte{damageInfoFrame}
		return resolution
	}
	if staticActorDamageInfoRuntimeEmissionOwned(attempt.Actor) {
		resolution.Frames = [][]byte{
			combatproto.EncodeServerTarget(packet),
			damageInfoFrame,
		}
		r.sharedWorld.EnqueueStaticActorFramesToVisiblePeers(attempt.Actor.EntityID, subjectID, [][]byte{damageInfoFrame})
	}
	return resolution
}

func staticActorSpawnBackedSelfDamageInfoRuntimeEmissionOwned(actor StaticActorSnapshot) bool {
	if actor.SpawnGroupRef == "" {
		return false
	}
	_, ok := worldruntime.BootstrapStaticActorCombatProfileDefaults(actor.CombatProfile)
	return ok
}

func staticActorDamageInfoRuntimeEmissionOwned(actor StaticActorSnapshot) bool {
	if actor.SpawnGroupRef != "" {
		return false
	}
	_, ok := worldruntime.BootstrapStaticActorCombatProfileDefaults(actor.CombatProfile)
	return ok
}

func staticActorInteractionFailureDelivery(failure string) *chatproto.ChatDeliveryPacket {
	message, ok := staticActorInteractionFailureMessage(failure)
	if !ok {
		return nil
	}
	delivery := chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: message}
	return &delivery
}

func questFlagRewardRestrictedDelivery(template itemcatalog.Template) *chatproto.ChatDeliveryPacket {
	message := questFlagRewardRestrictedInfoMessage
	if text := strings.TrimSpace(template.BuyRejectText); text != "" {
		message = text
	}
	delivery := chatproto.ChatDeliveryPacket{Type: chatproto.ChatTypeInfo, VID: 0, Empire: 0, Message: message}
	return &delivery
}

func staticActorInteractionFailureMessage(failure string) (string, bool) {
	switch failure {
	case StaticActorInteractionFailureSubjectNotFound, StaticActorInteractionFailureSubjectDead:
		return "Interaction unavailable right now.", true
	case StaticActorInteractionFailureTargetNotVisible:
		return "You cannot interact with that target right now.", true
	case StaticActorInteractionFailureTargetOutOfRange:
		return "You are too far away to interact with that target.", true
	case StaticActorInteractionFailureTargetDead:
		return "That target is unavailable right now.", true
	case StaticActorInteractionFailureTargetHasNoInteraction:
		return "Nothing happens.", true
	case staticActorInteractionFailureDefinitionNotFound:
		return "Interaction content is missing.", true
	case staticActorInteractionFailureUnsupportedKind:
		return "Interaction not supported yet.", true
	case staticActorInteractionFailureWarpDestinationInvalid:
		return "Warp destination is invalid.", true
	case staticActorInteractionFailureWarpNotApplied:
		return "Warp unavailable right now.", true
	case staticActorInteractionFailureQuestCurrentValueMismatch:
		return "Quest requirements are not met.", true
	case staticActorInteractionFailureQuestInsufficientGold:
		return questFlagInsufficientGoldInfoMessage, true
	case staticActorInteractionFailureQuestInsufficientExperience:
		return questFlagInsufficientExperienceInfoMessage, true
	case staticActorInteractionFailureQuestInsufficientMaterials:
		return questFlagInsufficientMaterialsInfoMessage, true
	case staticActorInteractionFailureQuestRewardInventoryFull:
		return itemPickupInventoryFullInfoMessage, true
	case staticActorInteractionFailureQuestRewardRestricted:
		return questFlagRewardRestrictedInfoMessage, true
	case staticActorInteractionFailureQuestRewardGoldOverflow:
		return questFlagRewardGoldOverflowInfoMessage, true
	case staticActorInteractionFailureQuestRewardExperienceOverflow:
		return questFlagRewardExperienceOverflowInfoMessage, true
	default:
		return "", false
	}
}

func (r *gameRuntime) interactionDefinitionPreview(actorName string, definition InteractionDefinition) (string, bool) {
	return r.interactionDefinitionVisibilityPreview("", actorName, definition)
}

func (r *gameRuntime) interactionDefinitionVisibilityPreview(characterName string, actorName string, definition InteractionDefinition) (string, bool) {
	switch definition.Kind {
	case interactionstore.KindInfo:
		if characterName != "" && interactionstore.HasServiceQuestGate(definition) {
			ok, err := r.serviceQuestGateSatisfied(characterName, definition)
			if err != nil {
				return "", false
			}
			if !ok {
				message, ok := staticActorInteractionFailureMessage(staticActorInteractionFailureQuestCurrentValueMismatch)
				if !ok {
					return "", false
				}
				return message, true
			}
		}
		return definition.Text, true
	case interactionstore.KindTalk:
		if characterName != "" && interactionstore.HasServiceQuestGate(definition) {
			ok, err := r.serviceQuestGateSatisfied(characterName, definition)
			if err != nil {
				return "", false
			}
			if !ok {
				message, ok := staticActorInteractionFailureMessage(staticActorInteractionFailureQuestCurrentValueMismatch)
				if !ok {
					return "", false
				}
				return message, true
			}
		}
		return fmt.Sprintf("%s:\n%s", actorName, definition.Text), true
	case interactionstore.KindShopPreview:
		if characterName != "" && interactionstore.HasServiceQuestGate(definition) {
			ok, err := r.serviceQuestGateSatisfied(characterName, definition)
			if err != nil {
				return "", false
			}
			if !ok {
				message, ok := staticActorInteractionFailureMessage(staticActorInteractionFailureQuestCurrentValueMismatch)
				if !ok {
					return "", false
				}
				return message, true
			}
		}
		return r.shopPreviewInteractionPreview(definition)
	case interactionstore.KindWarp:
		if characterName != "" && interactionstore.HasServiceQuestGate(definition) {
			ok, err := r.serviceQuestGateSatisfied(characterName, definition)
			if err != nil {
				return "", false
			}
			if !ok {
				message, ok := staticActorInteractionFailureMessage(staticActorInteractionFailureQuestCurrentValueMismatch)
				if !ok {
					return "", false
				}
				return message, true
			}
		}
		summary := fmt.Sprintf("warp -> map %d @ %d,%d", definition.MapIndex, definition.X, definition.Y)
		message := strings.TrimSpace(definition.Text)
		if message == "" {
			return summary, true
		}
		return fmt.Sprintf("%s [%s]", message, summary), true
	case interactionstore.KindOpenSafebox:
		if characterName != "" && interactionstore.HasServiceQuestGate(definition) {
			ok, err := r.serviceQuestGateSatisfied(characterName, definition)
			if err != nil {
				return "", false
			}
			if !ok {
				message, ok := staticActorInteractionFailureMessage(staticActorInteractionFailureQuestCurrentValueMismatch)
				if !ok {
					return "", false
				}
				return message, true
			}
		}
		summary := fmt.Sprintf("open_safebox size %d", interactionstore.EffectiveOpenSafeboxSize(definition))
		message := strings.TrimSpace(definition.Text)
		if message == "" {
			return summary, true
		}
		return fmt.Sprintf("%s [%s]", message, summary), true
	case interactionstore.KindQuestFlag:
		if characterName == "" {
			return questFlagRewardPreview(definition.Text, definition, r.itemTemplates), true
		}
		preview, err := r.previewQuestFlagInteraction(characterName, definition)
		if err != nil {
			return "", false
		}
		return preview, true
	default:
		return "", false
	}
}

func (r *gameRuntime) previewQuestFlagInteraction(characterName string, definition InteractionDefinition) (string, error) {
	if !interactionstore.ValidDefinition(definition) {
		return "", fmt.Errorf("invalid quest flag interaction definition")
	}
	result, err := r.PreviewQuestStateTransition(queststate.Transition{Character: characterName, QuestRef: definition.QuestRef, Flag: definition.QuestFlag, From: definition.QuestFrom, To: definition.QuestTo})
	if err != nil {
		return "", err
	}
	if result.Result.Applied {
		if definition.ConsumeGold != 0 {
			state, ok := r.liveCharacterState(characterName)
			if !ok || state.Gold > uint64(math.MaxInt32) || state.Gold < definition.ConsumeGold {
				message, ok := staticActorInteractionFailureMessage(staticActorInteractionFailureQuestInsufficientGold)
				if !ok {
					return "", fmt.Errorf("quest flag insufficient-gold preview is unsupported")
				}
				return message, nil
			}
		}
		if definition.ConsumeExperience != 0 {
			state, ok := r.liveCharacterState(characterName)
			if !ok || state.Points[bootstrapExperiencePointType] < 0 || uint64(state.Points[bootstrapExperiencePointType]) < definition.ConsumeExperience {
				message, ok := staticActorInteractionFailureMessage(staticActorInteractionFailureQuestInsufficientExperience)
				if !ok {
					return "", fmt.Errorf("quest flag insufficient-experience preview is unsupported")
				}
				return message, nil
			}
		}
		if consumeRequirements := questFlagConsumeRequirements(definition); len(consumeRequirements) > 0 {
			if !r.characterCanSupplyQuestFlagConsumeItems(characterName, consumeRequirements) {
				message, ok := staticActorInteractionFailureMessage(staticActorInteractionFailureQuestInsufficientMaterials)
				if !ok {
					return "", fmt.Errorf("quest flag insufficient-materials preview is unsupported")
				}
				return message, nil
			}
		}
		if message, ok := r.questFlagRewardScalarOverflowPreviewFailure(characterName, definition); ok {
			return message, nil
		}
		if message, ok := r.questFlagRewardGrantPreviewFailure(characterName, definition); ok {
			return message, nil
		}
		return questFlagRewardPreview(definition.Text, definition, r.itemTemplates), nil
	}
	if result.Result.Reason == queststate.TransitionReasonCurrentValueMismatch {
		message, ok := staticActorInteractionFailureMessage(staticActorInteractionFailureQuestCurrentValueMismatch)
		if !ok {
			return "", fmt.Errorf("quest flag mismatch preview is unsupported")
		}
		return message, nil
	}
	return "", fmt.Errorf("quest flag transition preview failed: %s", result.Result.Reason)
}

func (r *gameRuntime) questFlagRewardScalarOverflowPreviewFailure(characterName string, definition InteractionDefinition) (string, bool) {
	state, ok := r.liveCharacterState(characterName)
	if !ok {
		return "", false
	}
	if definition.RewardGold != 0 {
		goldAfterConsume := state.Gold
		if definition.ConsumeGold != 0 {
			if state.Gold > uint64(math.MaxInt32) || state.Gold < definition.ConsumeGold {
				return "", false
			}
			goldAfterConsume = state.Gold - definition.ConsumeGold
		}
		if goldAfterConsume > uint64(math.MaxInt32) || goldAfterConsume > uint64(math.MaxInt32)-definition.RewardGold {
			message, ok := staticActorInteractionFailureMessage(staticActorInteractionFailureQuestRewardGoldOverflow)
			return message, ok
		}
	}
	if definition.RewardExperience != 0 {
		if definition.RewardExperience > uint64(math.MaxInt32) {
			return "", false
		}
		experienceBefore := state.Points[bootstrapExperiencePointType]
		if definition.ConsumeExperience != 0 {
			if experienceBefore < 0 || uint64(experienceBefore) < definition.ConsumeExperience {
				return "", false
			}
		}
		experienceAfterConsume := int64(experienceBefore) - int64(definition.ConsumeExperience)
		nextExperience := experienceAfterConsume + int64(definition.RewardExperience)
		if nextExperience > math.MaxInt32 {
			message, ok := staticActorInteractionFailureMessage(staticActorInteractionFailureQuestRewardExperienceOverflow)
			return message, ok
		}
	}
	return "", false
}

func (r *gameRuntime) questFlagRewardGrantPreviewFailure(characterName string, definition InteractionDefinition) (string, bool) {
	rewardItems := interactionstore.EffectiveRewardItems(definition)
	if len(rewardItems) == 0 {
		return "", false
	}
	state, ok := r.liveCharacterState(characterName)
	if !ok {
		return "", false
	}
	items := make([]inventory.ItemInstance, 0, len(state.Inventory))
	for _, item := range state.Inventory {
		items = append(items, inventory.ItemInstance{ID: item.ID, Vnum: item.Vnum, Count: item.Count, Slot: inventory.SlotIndex(item.Slot), Locked: item.Locked})
	}
	scratch := player.NewRuntime(loginticket.Character{
		Name:      characterName,
		Level:     state.Level,
		Job:       state.Job,
		RaceNum:   state.RaceNum,
		Empire:    state.Empire,
		Inventory: items,
	}, player.SessionLink{})
	if consumeRequirements := questFlagConsumeRequirements(definition); len(consumeRequirements) > 0 {
		if _, ok := scratch.ConsumeCarriedItems(consumeRequirements); !ok {
			return "", false
		}
	}
	for _, entry := range rewardItems {
		template, ok := r.itemTemplates[entry.ItemVnum]
		if !ok {
			return "", false
		}
		failure := scratch.ValidateCarriedItemGrant(template, entry.Count)
		if failure == "" {
			if _, ok := scratch.GrantCarriedItem(template, entry.Count); !ok {
				return "", false
			}
			continue
		}
		switch failure {
		case player.CarriedItemGrantFailureNoValidPlacement:
			message, ok := staticActorInteractionFailureMessage(staticActorInteractionFailureQuestRewardInventoryFull)
			return message, ok
		case player.CarriedItemGrantFailureInvalid:
			delivery := questFlagRewardRestrictedDelivery(template)
			if delivery == nil {
				return "", false
			}
			return delivery.Message, true
		default:
			return "", false
		}
	}
	return "", false
}

func questFlagConsumeRequirements(definition InteractionDefinition) []player.CarriedItemConsumeRequirement {
	entries := interactionstore.EffectiveConsumeItems(definition)
	if len(entries) == 0 {
		return nil
	}
	requirements := make([]player.CarriedItemConsumeRequirement, 0, len(entries))
	for _, entry := range entries {
		requirements = append(requirements, player.CarriedItemConsumeRequirement{ItemVnum: entry.ItemVnum, Count: entry.Count})
	}
	return requirements
}

func (r *gameRuntime) characterCanSupplyQuestFlagConsumeItems(characterName string, requirements []player.CarriedItemConsumeRequirement) bool {
	if r == nil || characterName == "" {
		return false
	}
	state, ok := r.liveCharacterState(characterName)
	if !ok {
		return false
	}
	items := make([]inventory.ItemInstance, 0, len(state.Inventory))
	for _, item := range state.Inventory {
		items = append(items, inventory.ItemInstance{ID: item.ID, Vnum: item.Vnum, Count: item.Count, Slot: inventory.SlotIndex(item.Slot), Locked: item.Locked})
	}
	scratch := player.NewRuntime(loginticket.Character{Name: characterName, Inventory: items}, player.SessionLink{})
	return scratch.ValidateCarriedItemConsume(requirements) == ""
}

func questFlagRewardPreview(text string, definition InteractionDefinition, itemTemplates map[uint32]itemcatalog.Template) string {
	preview := text
	if definition.RewardGold != 0 {
		preview = fmt.Sprintf("%s [reward_gold %d]", preview, definition.RewardGold)
	}
	if definition.RewardExperience != 0 {
		preview = fmt.Sprintf("%s [reward_experience %d]", preview, definition.RewardExperience)
	}
	for _, entry := range interactionstore.EffectiveRewardItems(definition) {
		itemLabel := fmt.Sprintf("vnum %d", entry.ItemVnum)
		if template, ok := itemTemplates[entry.ItemVnum]; ok {
			name := strings.TrimSpace(template.Name)
			if name != "" {
				itemLabel = name
			}
		}
		count := entry.Count
		if count == 0 {
			count = 1
		}
		preview = fmt.Sprintf("%s [reward_item %s x%d]", preview, itemLabel, count)
	}
	if definition.ConsumeGold != 0 {
		preview = fmt.Sprintf("%s [consume_gold %d]", preview, definition.ConsumeGold)
	}
	if definition.ConsumeExperience != 0 {
		preview = fmt.Sprintf("%s [consume_experience %d]", preview, definition.ConsumeExperience)
	}
	for _, entry := range interactionstore.EffectiveConsumeItems(definition) {
		itemLabel := fmt.Sprintf("vnum %d", entry.ItemVnum)
		if template, ok := itemTemplates[entry.ItemVnum]; ok {
			name := strings.TrimSpace(template.Name)
			if name != "" {
				itemLabel = name
			}
		}
		count := entry.Count
		if count == 0 {
			count = 1
		}
		preview = fmt.Sprintf("%s [consume_item %s x%d]", preview, itemLabel, count)
	}
	return preview
}

func (r *gameRuntime) shopPreviewInteractionPreview(definition InteractionDefinition) (string, bool) {
	if !interactionstore.ValidDefinition(definition) || definition.Kind != interactionstore.KindShopPreview {
		return "", false
	}
	entries := make([]string, 0, len(definition.Catalog))
	for _, entry := range definition.Catalog {
		template, ok := r.itemTemplates[entry.ItemVnum]
		if !ok {
			return "", false
		}
		entries = append(entries, fmt.Sprintf("[%d] %s x%d @ %dg", entry.Slot, template.Name, entry.Count, entry.Price))
	}
	if len(entries) == 0 {
		return "", false
	}
	return fmt.Sprintf("%s: %s", definition.Title, strings.Join(entries, "; ")), true
}

func merchantCatalogEntryBySlot(definition InteractionDefinition, slot uint16) (interactionstore.MerchantCatalogEntry, bool) {
	if !interactionstore.ValidDefinition(definition) || definition.Kind != interactionstore.KindShopPreview {
		return interactionstore.MerchantCatalogEntry{}, false
	}
	for _, entry := range definition.Catalog {
		if entry.Slot == slot {
			return entry, true
		}
	}
	return interactionstore.MerchantCatalogEntry{}, false
}

func compactInteractionPreview(preview string) string {
	preview = strings.TrimSpace(preview)
	const maxPreviewLength = 160
	previewRunes := []rune(preview)
	if len(previewRunes) <= maxPreviewLength {
		return preview
	}
	return string(previewRunes[:maxPreviewLength-3]) + "..."
}

func (r *gameRuntime) interactionDefinitionExists(kind string, ref string) bool {
	if kind == "" && ref == "" {
		return true
	}
	if !worldruntime.ValidStaticActorInteractionMetadata(kind, ref) {
		return false
	}
	if r == nil || r.interactionStore == nil {
		return true
	}
	_, ok := r.ResolveInteractionDefinition(kind, ref)
	return ok
}

func validStaticActorRuntimeName(name string) bool {
	return worldruntime.ValidStaticActorName(name)
}

func (r *gameRuntime) persistStaticActorSnapshot(snapshot []StaticActorSnapshot) bool {
	if r == nil || r.staticStore == nil {
		return true
	}
	var combatState map[uint64]spawnGroupCombatPersistenceState
	if r.sharedWorld != nil {
		combatState = r.sharedWorld.spawnGroupCombatPersistenceState()
	}
	return r.staticStore.Save(buildStaticActorStoreSnapshotWithSpawnGroupCombatState(snapshot, combatState)) == nil
}

func (r *gameRuntime) persistSpawnGroupCombatState(entityID uint64) bool {
	if r == nil || r.sharedWorld == nil || entityID == 0 || r.staticStore == nil {
		return true
	}
	r.staticActorMu.Lock()
	defer r.staticActorMu.Unlock()
	return r.persistStaticActorSnapshot(r.sharedWorld.StaticActors())
}

func buildStaticActorStoreSnapshot(snapshot []StaticActorSnapshot) staticstore.Snapshot {
	return buildStaticActorStoreSnapshotWithSpawnGroupCombatState(snapshot, nil)
}

func buildStaticActorStoreSnapshotWithSpawnGroupCombatState(snapshot []StaticActorSnapshot, combatState map[uint64]spawnGroupCombatPersistenceState) staticstore.Snapshot {
	actors := make([]staticstore.StaticActor, 0, len(snapshot))
	for _, actor := range snapshot {
		actors = append(actors, staticstore.StaticActor{
			EntityID:         actor.EntityID,
			Name:             actor.Name,
			MapIndex:         actor.MapIndex,
			X:                actor.X,
			Y:                actor.Y,
			RaceNum:          actor.RaceNum,
			CombatProfile:    actor.CombatProfile,
			InteractionKind:  actor.InteractionKind,
			InteractionRef:   actor.InteractionRef,
			SpawnGroupRef:    actor.SpawnGroupRef,
			RewardExperience: actor.RewardExperience,
			RewardGold:       actor.RewardGold,
			RewardDropVnums:  append([]uint32(nil), actor.RewardDropVnums...),
			RewardQuestRef:   actor.RewardQuestRef,
			RewardQuestFlag:  actor.RewardQuestFlag,
			RewardQuestFrom:  actor.RewardQuestFrom,
			RewardQuestTo:    actor.RewardQuestTo,
			RewardQuestText:  actor.RewardQuestText,
			RequireQuestRef:  actor.RequireQuestRef,
			RequireQuestFlag: actor.RequireQuestFlag,
			RequireQuestFrom: actor.RequireQuestFrom,
		})
		if actor.SpawnHome != nil && actor.SpawnGroupRef != "" {
			spawnHome := *actor.SpawnHome
			actors[len(actors)-1].SpawnHome = &spawnHome
		}
		if state, ok := combatState[actor.EntityID]; ok && actor.SpawnGroupRef != "" {
			if state.HP == 0 && !state.RespawnAt.IsZero() {
				currentHP := uint8(0)
				readyAt := state.RespawnAt.UTC()
				actors[len(actors)-1].CombatCurrentHP = &currentHP
				actors[len(actors)-1].RespawnReadyAt = &readyAt
			} else if state.HP > 0 && state.RespawnAt.IsZero() {
				currentHP := state.HP
				actors[len(actors)-1].CombatCurrentHP = &currentHP
			}
		}
	}
	return staticstore.Snapshot{StaticActors: actors, CombatProfiles: staticActorStoreCombatProfiles(snapshot)}
}

func staticActorStoreCombatProfiles(snapshot []StaticActorSnapshot) []worldruntime.StaticActorCombatProfileSnapshot {
	if len(snapshot) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(snapshot))
	profiles := make([]worldruntime.StaticActorCombatProfileSnapshot, 0)
	for _, actor := range snapshot {
		profile := strings.TrimSpace(actor.CombatProfile)
		if profile == "" || profile == worldruntime.StaticActorCombatProfilePracticeMob || profile == worldruntime.StaticActorCombatProfileTrainingDummy {
			continue
		}
		if _, ok := seen[profile]; ok {
			continue
		}
		for _, snapshot := range worldruntime.StaticActorCombatProfileSnapshots() {
			if snapshot.Profile != profile {
				continue
			}
			profiles = append(profiles, snapshot)
			seen[profile] = struct{}{}
			break
		}
	}
	if len(profiles) == 0 {
		return nil
	}
	sort.Slice(profiles, func(i int, j int) bool {
		return profiles[i].Profile < profiles[j].Profile
	})
	return profiles
}

func cloneStaticActorSnapshots(snapshot []StaticActorSnapshot) []StaticActorSnapshot {
	if len(snapshot) == 0 {
		return nil
	}
	cloned := make([]StaticActorSnapshot, len(snapshot))
	copy(cloned, snapshot)
	for i := range cloned {
		if len(cloned[i].RewardDropVnums) > 0 {
			cloned[i].RewardDropVnums = append([]uint32(nil), cloned[i].RewardDropVnums...)
		}
		if cloned[i].SpawnHome != nil {
			spawnHome := *cloned[i].SpawnHome
			cloned[i].SpawnHome = &spawnHome
		}
		if cloned[i].SpawnLeash != nil {
			spawnLeash := *cloned[i].SpawnLeash
			if spawnLeash.ReturnTarget != nil {
				returnTarget := *spawnLeash.ReturnTarget
				spawnLeash.ReturnTarget = &returnTarget
			}
			cloned[i].SpawnLeash = &spawnLeash
		}
	}
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
	return config.DefaultLoginTicketStoreDir()
}

func serviceLoginTicketStoreDir(cfg config.Service) string {
	if dir := strings.TrimSpace(cfg.LoginTicketStoreDir); dir != "" {
		return dir
	}
	return defaultTicketStoreDir()
}

func servicePersistenceConfigWithDefaults(cfg config.Service) config.Service {
	cfg.LoginTicketStoreDir = serviceLoginTicketStoreDir(cfg)
	cfg.AccountStoreDir = serviceAccountStoreDir(cfg)
	cfg.StaticActorStorePath = serviceStaticActorStorePath(cfg)
	cfg.InteractionStorePath = serviceInteractionStorePath(cfg)
	cfg.ItemTemplateStorePath = serviceItemTemplateStorePath(cfg)
	cfg.QuestStateStorePath = serviceQuestStateStorePath(cfg)
	cfg.GroundItemStorePath = serviceGroundItemStorePath(cfg)
	cfg.SafeboxStorePath = serviceSafeboxStorePath(cfg)
	return cfg
}

func validateRuntimePersistenceConfig(cfg config.Service) error {
	return config.ValidatePersistenceConfig(servicePersistenceConfigWithDefaults(cfg))
}

func defaultAccountStoreDir() string {
	return config.DefaultAccountStoreDir()
}

func serviceAccountStoreDir(cfg config.Service) string {
	if dir := strings.TrimSpace(cfg.AccountStoreDir); dir != "" {
		return dir
	}
	return defaultAccountStoreDir()
}

func defaultStaticActorStorePath() string {
	return config.DefaultStaticActorStorePath()
}

func serviceStaticActorStorePath(cfg config.Service) string {
	if path := strings.TrimSpace(cfg.StaticActorStorePath); path != "" {
		return path
	}
	return defaultStaticActorStorePath()
}

func defaultInteractionStorePath() string {
	return config.DefaultInteractionStorePath()
}

func serviceInteractionStorePath(cfg config.Service) string {
	if path := strings.TrimSpace(cfg.InteractionStorePath); path != "" {
		return path
	}
	return defaultInteractionStorePath()
}

func defaultItemTemplateStorePath() string {
	return config.DefaultItemTemplateStorePath()
}

func serviceItemTemplateStorePath(cfg config.Service) string {
	if path := strings.TrimSpace(cfg.ItemTemplateStorePath); path != "" {
		return path
	}
	return defaultItemTemplateStorePath()
}

func defaultQuestStateStorePath() string {
	return config.DefaultQuestStateStorePath()
}

func serviceQuestStateStorePath(cfg config.Service) string {
	if path := strings.TrimSpace(cfg.QuestStateStorePath); path != "" {
		return path
	}
	return defaultQuestStateStorePath()
}

func defaultGroundItemStorePath() string {
	return config.DefaultGroundItemStorePath()
}

func serviceGroundItemStorePath(cfg config.Service) string {
	if path := strings.TrimSpace(cfg.GroundItemStorePath); path != "" {
		return path
	}
	return defaultGroundItemStorePath()
}

func defaultSafeboxStorePath() string {
	return config.DefaultSafeboxStorePath()
}

func serviceSafeboxStorePath(cfg config.Service) string {
	if path := strings.TrimSpace(cfg.SafeboxStorePath); path != "" {
		return path
	}
	return defaultSafeboxStorePath()
}

func (r *gameRuntime) loadPersistedGroundItems() error {
	if r == nil || r.groundItemStore == nil || r.sharedWorld == nil {
		return nil
	}
	snapshot, err := r.groundItemStore.Load()
	if err != nil {
		if errors.Is(err, worldruntime.ErrGroundItemSnapshotNotFound) {
			return nil
		}
		return err
	}
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	filtered := worldruntime.FilterDurableGroundItemSnapshotForRestore(snapshot, now)
	if err := r.sharedWorld.RestorePersistedGroundItems(filtered.GroundItems); err != nil {
		return err
	}
	// Rewrite filtered/publicized pending set so crash restart does not revive expired ownership.
	return r.persistPendingGroundItemsLocked()
}

func (r *gameRuntime) persistPendingGroundItems() {
	if r == nil {
		return
	}
	_ = r.persistPendingGroundItemsLocked()
}

func (r *gameRuntime) persistPendingGroundItemsLocked() error {
	if r == nil || r.groundItemStore == nil || r.sharedWorld == nil {
		return nil
	}
	r.groundItemPersistMu.Lock()
	defer r.groundItemPersistMu.Unlock()
	return r.groundItemStore.Save(r.sharedWorld.DurableGroundItemSnapshot())
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
