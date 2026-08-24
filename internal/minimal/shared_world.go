package minimal

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/player"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

const (
	bootstrapSpawnGroupChaseMoveFunc     uint8  = 1
	bootstrapSpawnGroupChaseMoveDuration uint32 = 250
)

type queuedSessionFlow struct {
	inner       service.SessionFlow
	pending     *pendingServerFrames
	beforeFlush func()
	onClose     func()
	closeOnce   sync.Once
	closeErr    error
}

type pendingServerFrames struct {
	mu     sync.Mutex
	frames [][]byte
}

type sharedWorldRegistry struct {
	mu                                sync.Mutex
	topology                          worldruntime.BootstrapTopology
	entities                          *worldruntime.EntityRegistry
	sessionDirectory                  *worldruntime.SessionDirectory
	staticActorCombatHP               map[uint64]uint8
	staticActorCombatRespawnAt        map[uint64]time.Time
	staticActorCombatSnapshot         map[uint64]uint64
	staticActorCombatEngagedBy        map[uint64]uint64
	staticActorProximityAggroSuppress map[uint64]map[uint64]struct{}
	// pendingProximityAggroSuppressByVID parks actor suppress membership across
	// Leave → fresh Join identity changes (e.g. /phase_select). Live suppress
	// stays keyed by subject entity ID; VID is only the handoff key.
	pendingProximityAggroSuppressByVID map[uint32]map[uint64]struct{}
	staticActorDeathReward             map[uint64]worldruntime.StaticActorDeathReward
	staticActorKillQuestCredit         map[uint64]staticActorKillQuestCredit
	sessionCombatTargets               map[uint64]uint32
	sessionCombatRetaliations          map[uint64]combatRetaliationTimer
	sessionMerchantWindows             map[uint64]bool
	sessionSafeboxWindows              map[uint64]bool
	sessionRefineWindows               map[uint64]bool
	// sessionMyShopWindows remembers open private-shop presentations keyed by
	// shared-world entity ID. Presence means busy/open; the value is the
	// accepted non-empty live sign rematerialized on peer view-entry.
	sessionMyShopWindows            map[uint64]string
	exchangePartners                map[uint64]uint64
	exchangeItems                   map[uint64]map[uint8]exchangeDisplayedItem
	exchangeGold                    map[uint64]uint32
	exchangeAccepted                map[uint64]bool
	nextStaticActorCombatSnapshotID uint64
	lastKnownCharacters             map[uint64]loginticket.Character
	groundItemsByVID                map[uint32]sharedGroundItem
	itemTemplates                   map[uint32]itemcatalog.Template
	suppressStaticActorFanout       bool
	pendingStaticActorImportDeletes []worldruntime.StaticEntity
	now                             func() time.Time
	onGroundItemsChanged            func()
}

type sharedGroundItem struct {
	VID                uint32
	OwnerID            uint64
	OwnerLogin         string
	OwnerCharacterID   uint32
	OwnerVID           uint32
	OwnerName          string
	OwnerHPPoint       int32
	Item               inventory.ItemInstance
	GoldAmount         uint32
	PickupRange        int64
	MapIndex           uint32
	X                  int32
	Y                  int32
	Z                  int32
	OwnershipExclusive bool
	OwnershipExpiresAt time.Time
	DespawnAt          time.Time
}

type sharedGroundItemPickup struct {
	Item       inventory.ItemInstance
	GoldAmount uint32
	OwnerID    uint64
	OwnerLogin string
	OwnerName  string
	Owner      loginticket.Character
}

type exchangeDisplayedItem struct {
	ItemID uint64
	Vnum   uint32
	Count  uint16
	Slot   inventory.SlotIndex
}

// exchangeFinalizePlan captures a validated second-accept trade without mutating
// the exchange shell yet. The factory applies inventory/gold/quickslot mutation
// and persistence, then CommitExchangeFinalize closes the shell and emits frames.
type exchangeFinalizePlan struct {
	OriginID     uint64
	PartnerID    uint64
	Origin       loginticket.Character
	Partner      loginticket.Character
	OriginItems  map[uint8]exchangeDisplayedItem
	PartnerItems map[uint8]exchangeDisplayedItem
	OriginGold   uint32
	PartnerGold  uint32
}

type sharedGroundItemVisibilityDiff struct {
	Removed []sharedGroundItem
	Added   []sharedGroundItem
}

type combatRetaliationTimer struct {
	TargetVID       uint32
	SnapshotVersion uint64
	ReadyAt         time.Time
}

type staticActorKillQuestCredit struct {
	QuestRef         string
	QuestFlag        string
	QuestFrom        uint32
	QuestTo          uint32
	Text             string
	RequireQuestRef  string
	RequireQuestFlag string
	RequireQuestFrom uint32
}

func (c staticActorKillQuestCredit) Empty() bool {
	return strings.TrimSpace(c.QuestRef) == "" &&
		strings.TrimSpace(c.QuestFlag) == "" &&
		c.QuestFrom == 0 &&
		c.QuestTo == 0 &&
		strings.TrimSpace(c.Text) == "" &&
		strings.TrimSpace(c.RequireQuestRef) == "" &&
		strings.TrimSpace(c.RequireQuestFlag) == "" &&
		c.RequireQuestFrom == 0
}

func (c staticActorKillQuestCredit) Clone() staticActorKillQuestCredit {
	return staticActorKillQuestCredit{
		QuestRef:         strings.TrimSpace(c.QuestRef),
		QuestFlag:        strings.TrimSpace(c.QuestFlag),
		QuestFrom:        c.QuestFrom,
		QuestTo:          c.QuestTo,
		Text:             strings.TrimSpace(c.Text),
		RequireQuestRef:  strings.TrimSpace(c.RequireQuestRef),
		RequireQuestFlag: strings.TrimSpace(c.RequireQuestFlag),
		RequireQuestFrom: c.RequireQuestFrom,
	}
}

func (c staticActorKillQuestCredit) HasRequireGate() bool {
	credit := c.Clone()
	return credit.RequireQuestRef != "" && credit.RequireQuestFlag != ""
}

func validStaticActorKillQuestRequireGate(credit staticActorKillQuestCredit) bool {
	credit = credit.Clone()
	hasRef := credit.RequireQuestRef != ""
	hasFlag := credit.RequireQuestFlag != ""
	if !hasRef && !hasFlag {
		return credit.RequireQuestFrom == 0
	}
	if !hasRef || !hasFlag {
		return false
	}
	return queststate.ValidQuestRef(credit.RequireQuestRef) && queststate.ValidFlagName(credit.RequireQuestFlag)
}

func validStaticActorKillQuestCredit(credit staticActorKillQuestCredit) bool {
	credit = credit.Clone()
	if credit.Empty() {
		return true
	}
	return queststate.ValidQuestRef(credit.QuestRef) &&
		queststate.ValidFlagName(credit.QuestFlag) &&
		credit.QuestFrom != credit.QuestTo &&
		credit.Text != "" &&
		utf8.ValidString(credit.Text) &&
		!strings.ContainsRune(credit.Text, '\x00') &&
		validStaticActorKillQuestRequireGate(credit)
}

type engagedSpawnGroupRetaliationArmTarget struct {
	TargetVID       uint32
	SnapshotVersion uint64
}

type staticActorCombatStateSnapshot struct {
	HP                     map[uint64]uint8
	RespawnAt              map[uint64]time.Time
	Snapshot               map[uint64]uint64
	EngagedBy              map[uint64]uint64
	ProximityAggroSuppress map[uint64]map[uint64]struct{}
	DeathReward            map[uint64]worldruntime.StaticActorDeathReward
	SessionTargets         map[uint64]uint32
	SessionRetaliations    map[uint64]combatRetaliationTimer
	NextSnapshotID         uint64
}

const (
	StaticActorInteractionFailureSubjectNotFound        = "subject_not_found"
	StaticActorInteractionFailureSubjectDead            = "subject_dead"
	StaticActorInteractionFailureTargetNotVisible       = "target_not_visible"
	StaticActorInteractionFailureTargetOutOfRange       = "target_out_of_range"
	StaticActorInteractionFailureTargetDead             = "target_dead"
	StaticActorInteractionFailureTargetHasNoInteraction = "target_has_no_interaction"

	StaticActorCombatTargetFailureSubjectNotFound      = "subject_not_found"
	StaticActorCombatTargetFailureSubjectDead          = "subject_dead"
	StaticActorCombatTargetFailureTargetNotVisible     = "target_not_visible"
	StaticActorCombatTargetFailureTargetOutOfRange     = "target_out_of_range"
	StaticActorCombatTargetFailureTargetNotTargetable  = "target_not_targetable"
	StaticActorCombatTargetFailureTargetReturnRequired = "target_return_required"
	StaticActorCombatTargetFailureTargetEngaged        = "target_engaged"
	StaticActorCombatTargetFailureTargetDead           = "target_dead"

	StaticActorCombatAttackFailureSubjectNotFound        = "subject_not_found"
	StaticActorCombatAttackFailureSubjectDead            = "subject_dead"
	StaticActorCombatAttackFailureNoActiveTarget         = "no_active_target"
	StaticActorCombatAttackFailureTargetMismatch         = "target_mismatch"
	StaticActorCombatAttackFailureTargetNotVisible       = "target_not_visible"
	StaticActorCombatAttackFailureTargetOutOfRange       = "target_out_of_range"
	StaticActorCombatAttackFailureTargetNotTargetable    = "target_not_targetable"
	StaticActorCombatAttackFailureTargetReturnRequired   = "target_return_required"
	StaticActorCombatAttackFailureTargetEngaged          = "target_engaged"
	StaticActorCombatAttackFailureTargetDead             = "target_dead"
	StaticActorCombatAttackFailureTargetSnapshotMismatch = "target_snapshot_mismatch"

	bootstrapGroundItemPickupRange       = int64(300)
	bootstrapGroundItemOwnershipDuration = 30 * time.Second
	bootstrapGroundItemDespawnDuration   = 300 * time.Second
)

const (
	staticActorInteractionMaxDistance  int32 = 300
	staticActorCombatTargetMaxDistance int32 = 300
)

type StaticActorInteractionAttempt struct {
	Accepted  bool
	Failure   string
	TargetVID uint32
	Actor     StaticActorSnapshot
}

type StaticActorCombatTargetAttempt struct {
	Accepted        bool
	Failure         string
	TargetVID       uint32
	SnapshotVersion uint64
	HPPercent       uint8
	Actor           StaticActorSnapshot
}

type CombatTargetSnapshot struct {
	SubjectEntityID         uint64                      `json:"subject_entity_id"`
	Subject                 ConnectedCharacterSnapshot  `json:"subject"`
	TargetVID               uint32                      `json:"target_vid"`
	SnapshotVersion         uint64                      `json:"snapshot_version"`
	HPPercent               uint8                       `json:"hp_percent"`
	TargetCurrentHP         uint8                       `json:"target_current_hp,omitempty"`
	TargetMaxHP             uint8                       `json:"target_max_hp,omitempty"`
	NormalAttackDamage      uint8                       `json:"normal_attack_damage,omitempty"`
	TargetAttackValue       uint16                      `json:"target_attack_value"`
	TargetDefenseValue      uint16                      `json:"target_defense_value"`
	Actor                   StaticActorSnapshot         `json:"actor"`
	EngagedByEntityID       uint64                      `json:"engaged_by_entity_id,omitempty"`
	EngagedBy               *ConnectedCharacterSnapshot `json:"engaged_by,omitempty"`
	RetaliationPointDelta   int32                       `json:"retaliation_point_delta,omitempty"`
	RetaliationServerOrigin bool                        `json:"retaliation_server_origin,omitempty"`
	RetaliationPending      bool                        `json:"retaliation_pending,omitempty"`
	RetaliationReadyAt      *time.Time                  `json:"retaliation_ready_at,omitempty"`
	RetaliationRemainingMs  *int64                      `json:"retaliation_remaining_ms,omitempty"`
}

type StaticActorCombatAttackAttempt struct {
	Accepted                    bool
	Failure                     string
	ActiveTargetVID             uint32
	ActiveTargetSnapshotVersion uint64
	RequestedTargetVID          uint32
	HPPercent                   uint8
	Damage                      uint8
	Died                        bool
	DeathReward                 worldruntime.StaticActorDeathReward
	Actor                       StaticActorSnapshot
}

func newQueuedSessionFlow(inner service.SessionFlow, pending *pendingServerFrames, beforeFlush func(), onClose func()) *queuedSessionFlow {
	return &queuedSessionFlow{inner: inner, pending: pending, beforeFlush: beforeFlush, onClose: onClose}
}

func (f *queuedSessionFlow) Start() ([][]byte, error) {
	return f.inner.Start()
}

func (f *queuedSessionFlow) HandleClientFrame(in frame.Frame) ([][]byte, error) {
	return f.inner.HandleClientFrame(in)
}

func (f *queuedSessionFlow) CurrentPhase() session.Phase {
	phaseAware, ok := f.inner.(interface{ CurrentPhase() session.Phase })
	if !ok {
		return ""
	}
	return phaseAware.CurrentPhase()
}

func (f *queuedSessionFlow) FlushServerFrames() ([][]byte, error) {
	if f.beforeFlush != nil {
		f.beforeFlush()
	}
	if f.pending == nil {
		return nil, nil
	}
	return f.pending.flush(), nil
}

func (f *queuedSessionFlow) EncryptLegacyOutgoing(raw []byte) ([]byte, error) {
	secureFlow, ok := f.inner.(interface {
		EncryptLegacyOutgoing([]byte) ([]byte, error)
	})
	if !ok {
		return append([]byte(nil), raw...), nil
	}
	return secureFlow.EncryptLegacyOutgoing(raw)
}

func (f *queuedSessionFlow) DecryptLegacyIncoming(raw []byte) ([]byte, error) {
	secureFlow, ok := f.inner.(interface {
		DecryptLegacyIncoming([]byte) ([]byte, error)
	})
	if !ok {
		return append([]byte(nil), raw...), nil
	}
	return secureFlow.DecryptLegacyIncoming(raw)
}

func (f *queuedSessionFlow) Close() error {
	f.closeOnce.Do(func() {
		if f.onClose != nil {
			f.onClose()
		}
		if closer, ok := f.inner.(interface{ Close() error }); ok {
			f.closeErr = closer.Close()
		}
	})
	return f.closeErr
}

func newPendingServerFrames() *pendingServerFrames {
	return &pendingServerFrames{}
}

func (q *pendingServerFrames) enqueue(frames [][]byte) {
	if q == nil || len(frames) == 0 {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, raw := range frames {
		q.frames = append(q.frames, append([]byte(nil), raw...))
	}
}

func (q *pendingServerFrames) Enqueue(frames [][]byte) {
	q.enqueue(frames)
}

func (q *pendingServerFrames) flush() [][]byte {
	if q == nil {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	out := q.frames
	q.frames = nil
	return out
}

func newSharedWorldRegistry() *sharedWorldRegistry {
	return newSharedWorldRegistryWithTopology(worldruntime.NewBootstrapTopology(0))
}

func newSharedWorldRegistryWithTopology(topology worldruntime.BootstrapTopology) *sharedWorldRegistry {
	return &sharedWorldRegistry{
		topology:                           topology,
		entities:                           worldruntime.NewEntityRegistryWithTopology(topology),
		sessionDirectory:                   worldruntime.NewSessionDirectory(),
		staticActorCombatHP:                make(map[uint64]uint8),
		staticActorCombatRespawnAt:         make(map[uint64]time.Time),
		staticActorCombatSnapshot:          make(map[uint64]uint64),
		staticActorCombatEngagedBy:         make(map[uint64]uint64),
		staticActorProximityAggroSuppress:  make(map[uint64]map[uint64]struct{}),
		pendingProximityAggroSuppressByVID: make(map[uint32]map[uint64]struct{}),
		staticActorDeathReward:             make(map[uint64]worldruntime.StaticActorDeathReward),
		staticActorKillQuestCredit:         make(map[uint64]staticActorKillQuestCredit),
		sessionCombatRetaliations:          make(map[uint64]combatRetaliationTimer),
		exchangePartners:                   make(map[uint64]uint64),
		exchangeAccepted:                   make(map[uint64]bool),
		exchangeGold:                       make(map[uint64]uint32),
		lastKnownCharacters:                make(map[uint64]loginticket.Character),
		groundItemsByVID:                   make(map[uint32]sharedGroundItem),
		now:                                time.Now,
	}
}

func newSharedWorldSessionEntry(pending *pendingServerFrames, relocate sharedWorldSessionRelocator) worldruntime.SessionEntry {
	var entry worldruntime.SessionEntry
	if pending != nil {
		entry.FrameSink = pending
	}
	if relocate != nil {
		entry.Relocator = func(mapIndex uint32, x int32, y int32) (any, bool) {
			return relocate(mapIndex, x, y)
		}
	}
	return entry
}

func registerSharedWorldSessionEntry(directory *worldruntime.SessionDirectory, entityID uint64, pending *pendingServerFrames, relocate sharedWorldSessionRelocator) bool {
	if directory == nil {
		return true
	}
	entry := newSharedWorldSessionEntry(pending, relocate)
	if entry.FrameSink == nil && entry.Relocator == nil {
		return true
	}
	return directory.Register(entityID, entry)
}

func (r *sharedWorldRegistry) SetItemTemplates(templates map[uint32]itemcatalog.Template) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.itemTemplates = templates
}

func (r *sharedWorldRegistry) scopesLocked() worldruntime.Scopes {
	if r == nil {
		return worldruntime.Scopes{}
	}
	return worldruntime.NewScopes(r.topology, r.entities)
}

func (r *sharedWorldRegistry) sessionEntryLocked(entityID uint64) (worldruntime.SessionEntry, bool) {
	if r == nil || r.sessionDirectory == nil || entityID == 0 {
		return worldruntime.SessionEntry{}, false
	}
	return r.sessionDirectory.Lookup(entityID)
}

func (r *sharedWorldRegistry) HasLiveSession(entityID uint64) bool {
	if r == nil || entityID == 0 {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.sessionEntryLocked(entityID); !ok {
		return false
	}
	_, ok := r.playerCharacter(entityID)
	return ok
}

func (r *sharedWorldRegistry) HasVisiblePlayerTarget(originID uint64, targetVID uint32) bool {
	if r == nil || originID == 0 || targetVID == 0 {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.sessionEntryLocked(originID); !ok {
		return false
	}
	origin, ok := r.playerCharacter(originID)
	if !ok || characterAtBootstrapHPFloor(origin) {
		return false
	}
	for _, candidate := range r.scopesLocked().VisibleTargets(originID, origin) {
		if candidate.Character.VID != targetVID || characterAtBootstrapHPFloor(candidate.Character) {
			continue
		}
		if _, ok := r.sessionEntryLocked(candidate.Entity.ID); !ok {
			return false
		}
		return true
	}
	return false
}

func (r *sharedWorldRegistry) StartExchange(originID uint64, targetVID uint32) ([][]byte, bool) {
	if r == nil || originID == 0 || targetVID == 0 {
		return nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.sessionEntryLocked(originID); !ok {
		return nil, false
	}
	origin, ok := r.playerCharacter(originID)
	if !ok || characterAtBootstrapHPFloor(origin) {
		return nil, false
	}
	if r.exchangePartners == nil {
		r.exchangePartners = make(map[uint64]uint64)
	}
	if _, busy := r.exchangePartners[originID]; busy {
		return nil, false
	}

	var target worldruntime.PlayerEntity
	found := false
	for _, candidate := range r.scopesLocked().VisibleTargets(originID, origin) {
		if candidate.Character.VID != targetVID {
			continue
		}
		target = candidate
		found = true
		break
	}
	if !found || target.Entity.ID == 0 || target.Entity.ID == originID || characterAtBootstrapHPFloor(target.Character) {
		return nil, false
	}
	if !worldruntime.WithinExchangeDistance(origin, target.Character) {
		return nil, false
	}
	if _, ok := r.sessionEntryLocked(target.Entity.ID); !ok {
		return nil, false
	}
	if r.hasMerchantWindowOpenLocked(target.Entity.ID) || r.hasSafeboxWindowOpenLocked(target.Entity.ID) || r.hasRefineWindowOpenLocked(target.Entity.ID) || r.hasMyShopWindowOpenLocked(target.Entity.ID) {
		return [][]byte{encodeExchangePartnerMerchantBusyInfoFrame()}, true
	}
	if _, busy := r.exchangePartners[target.Entity.ID]; busy {
		return [][]byte{encodeExchangeAlreadyFrame()}, true
	}
	// Gold-carrier-cap gate stays after busy-window / ALREADY so those owned
	// rejects keep local-first precedence. When both sides are already at or
	// above the signed point-change carrier max, the requester-side string wins.
	if origin.Gold >= exchangeGoldPointChangeCarrierMax {
		return [][]byte{encodeExchangeRequesterGoldCarrierCapInfoFrame()}, true
	}
	if target.Character.Gold >= exchangeGoldPointChangeCarrierMax {
		return [][]byte{encodeExchangePartnerGoldCarrierCapInfoFrame()}, true
	}

	originFrames := [][]byte{encodeExchangeStartFrame(target.Character.VID)}
	targetFrames := [][]byte{encodeExchangeStartFrame(origin.VID)}
	if !r.enqueueToEntityLocked(target.Entity.ID, targetFrames) {
		return nil, false
	}
	r.exchangePartners[originID] = target.Entity.ID
	r.exchangePartners[target.Entity.ID] = originID
	r.setExchangeGoldLocked(originID, 0)
	r.setExchangeGoldLocked(target.Entity.ID, 0)
	r.setExchangeAcceptedLocked(originID, false)
	r.setExchangeAcceptedLocked(target.Entity.ID, false)
	return originFrames, true
}

func (r *sharedWorldRegistry) CancelExchange(originID uint64) ([][]byte, bool) {
	if r == nil || originID == 0 {
		return nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.clearExchangeLocked(originID, true) {
		return nil, false
	}
	return [][]byte{encodeExchangeEndFrame()}, true
}

func (r *sharedWorldRegistry) CloseExchange(originID uint64) ([][]byte, bool) {
	if r == nil || originID == 0 {
		return nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.clearExchangeLocked(originID, true) {
		return nil, false
	}
	return [][]byte{encodeExchangeEndFrame()}, true
}

func (r *sharedWorldRegistry) hasActiveExchange(originID uint64) bool {
	if r == nil || originID == 0 {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	partnerID, ok := r.exchangePartners[originID]
	if !ok || partnerID == 0 {
		return false
	}
	if _, ok := r.sessionEntryLocked(originID); !ok {
		return false
	}
	if _, ok := r.sessionEntryLocked(partnerID); !ok {
		return false
	}
	origin, ok := r.playerCharacter(originID)
	if !ok || characterAtBootstrapHPFloor(origin) {
		return false
	}
	partner, ok := r.playerCharacter(partnerID)
	if !ok || characterAtBootstrapHPFloor(partner) {
		return false
	}
	return true
}

func (r *sharedWorldRegistry) AddExchangeItem(originID uint64, displaySlot uint8, display player.ExchangeItemAddDisplay) ([][]byte, bool) {
	if r == nil || originID == 0 || displaySlot >= itemproto.ExchangeItemMaxNum {
		return nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	partnerID, ok := r.exchangePartners[originID]
	if !ok || partnerID == 0 {
		return nil, false
	}
	if _, ok := r.sessionEntryLocked(originID); !ok {
		return nil, false
	}
	if _, ok := r.sessionEntryLocked(partnerID); !ok {
		return nil, false
	}
	origin, ok := r.playerCharacter(originID)
	if !ok || characterAtBootstrapHPFloor(origin) {
		return nil, false
	}
	partner, ok := r.playerCharacter(partnerID)
	if !ok || characterAtBootstrapHPFloor(partner) {
		return nil, false
	}
	if display.Item.ID == 0 {
		return nil, false
	}
	if r.exchangeItems == nil {
		r.exchangeItems = make(map[uint64]map[uint8]exchangeDisplayedItem)
	}
	originItems := r.exchangeItems[originID]
	if originItems == nil {
		originItems = make(map[uint8]exchangeDisplayedItem)
		r.exchangeItems[originID] = originItems
	}
	if _, occupied := originItems[displaySlot]; occupied {
		return nil, false
	}
	for _, displayedItem := range originItems {
		if displayedItem.ItemID == display.Item.ID {
			return nil, false
		}
	}
	selfFrame := encodeExchangeItemAddFrame(1, displaySlot, display)
	peerFrame := encodeExchangeItemAddFrame(0, displaySlot, display)
	selfFrames, peerFrames := r.exchangeAcceptResetFramesLocked(originID, partnerID)
	selfFrames = append(selfFrames, selfFrame)
	peerFrames = append(peerFrames, peerFrame)
	if !r.enqueueToEntityLocked(partnerID, peerFrames) {
		return nil, false
	}
	originItems[displaySlot] = exchangeDisplayedItem{ItemID: display.Item.ID, Vnum: display.Item.Vnum, Count: display.Item.Count, Slot: display.Item.Slot}
	r.setExchangeAcceptedLocked(originID, false)
	r.setExchangeAcceptedLocked(partnerID, false)
	return selfFrames, true
}

func (r *sharedWorldRegistry) AddExchangeGold(originID uint64, amount uint32, availableGold uint64) ([][]byte, bool) {
	if r == nil || originID == 0 || amount == 0 {
		return nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	partnerID, ok := r.exchangePartners[originID]
	if !ok || partnerID == 0 {
		return nil, false
	}
	if _, ok := r.sessionEntryLocked(originID); !ok {
		return nil, false
	}
	if _, ok := r.sessionEntryLocked(partnerID); !ok {
		return nil, false
	}
	origin, ok := r.playerCharacter(originID)
	if !ok || characterAtBootstrapHPFloor(origin) {
		return nil, false
	}
	partner, ok := r.playerCharacter(partnerID)
	if !ok || characterAtBootstrapHPFloor(partner) {
		return nil, false
	}
	if uint64(amount) > availableGold {
		return [][]byte{encodeExchangeLessGoldFrame()}, true
	}

	selfFrame := encodeExchangeGoldAddFrame(1, amount)
	peerFrame := encodeExchangeGoldAddFrame(0, amount)
	selfFrames, peerFrames := r.exchangeAcceptResetFramesLocked(originID, partnerID)
	selfFrames = append(selfFrames, selfFrame)
	peerFrames = append(peerFrames, peerFrame)
	if !r.enqueueToEntityLocked(partnerID, peerFrames) {
		return nil, false
	}
	r.setExchangeGoldLocked(originID, amount)
	r.setExchangeAcceptedLocked(originID, false)
	r.setExchangeAcceptedLocked(partnerID, false)
	return selfFrames, true
}

func (r *sharedWorldRegistry) AcceptExchange(originID uint64, availableGold uint64, live loginticket.Character) ([][]byte, *exchangeFinalizePlan, bool) {
	if r == nil || originID == 0 {
		return nil, nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	partnerID, ok := r.exchangePartners[originID]
	if !ok || partnerID == 0 {
		return nil, nil, false
	}
	if _, ok := r.sessionEntryLocked(originID); !ok {
		return nil, nil, false
	}
	if _, ok := r.sessionEntryLocked(partnerID); !ok {
		return nil, nil, false
	}
	origin, ok := r.playerCharacter(originID)
	if !ok || characterAtBootstrapHPFloor(origin) {
		return nil, nil, false
	}
	partner, ok := r.playerCharacter(partnerID)
	if !ok || characterAtBootstrapHPFloor(partner) {
		return nil, nil, false
	}
	if live.ID == 0 || live.ID != origin.ID || live.VID != origin.VID || normalizeLiveCharacterName(live.Name) != normalizeLiveCharacterName(origin.Name) {
		return nil, nil, false
	}
	// Fail closed before accept markers or mutual-accept finalize when either paired
	// side currently has an open merchant / safebox / refine busy presentation.
	// Mirror START busy info-chat: requester-local busy wins, otherwise partner busy.
	if r.hasMerchantWindowOpenLocked(originID) || r.hasSafeboxWindowOpenLocked(originID) || r.hasRefineWindowOpenLocked(originID) || r.hasMyShopWindowOpenLocked(originID) {
		return [][]byte{encodeExchangeRequesterMerchantBusyInfoFrame()}, nil, true
	}
	if r.hasMerchantWindowOpenLocked(partnerID) || r.hasSafeboxWindowOpenLocked(partnerID) || r.hasRefineWindowOpenLocked(partnerID) || r.hasMyShopWindowOpenLocked(partnerID) {
		return [][]byte{encodeExchangePartnerMerchantBusyInfoFrame()}, nil, true
	}
	// Gold-carrier-cap gate stays after busy-window rejects and ahead of Check /
	// displayed-gold / finalization preconditions. Local-first requester-wins
	// matches START / busy ordering when both sides are already over the cap.
	if origin.Gold >= exchangeGoldPointChangeCarrierMax {
		return [][]byte{encodeExchangeRequesterGoldCarrierCapInfoFrame()}, nil, true
	}
	if partner.Gold >= exchangeGoldPointChangeCarrierMax {
		return [][]byte{encodeExchangePartnerGoldCarrierCapInfoFrame()}, nil, true
	}
	if !exchangeDisplayedItemsStillLive(r.exchangeItems[originID], live, r.itemTemplates) {
		// Second-accept Check failure is dual-sided; first-side accept stays silent.
		if r.exchangeAccepted[partnerID] {
			return r.exchangeEmitFinalizeCheckRejectLocked(originID, partnerID)
		}
		return nil, nil, false
	}
	if displayedGold := r.exchangeGold[originID]; displayedGold != 0 && uint64(displayedGold) > availableGold {
		return [][]byte{encodeExchangeLessGoldFrame()}, nil, true
	}
	if r.exchangeAccepted[partnerID] {
		if !exchangeDisplayedItemsStillLive(r.exchangeItems[partnerID], partner, r.itemTemplates) {
			frames, ok := r.exchangeEmitFinalizeCheckRejectForCallerLocked(originID, partnerID, originID)
			if !ok {
				return nil, nil, false
			}
			return frames, nil, true
		}
		if displayedGold := r.exchangeGold[partnerID]; displayedGold != 0 && uint64(displayedGold) > partner.Gold {
			frames, ok := r.exchangeEmitFinalizeCheckRejectForCallerLocked(originID, partnerID, originID)
			if !ok {
				return nil, nil, false
			}
			return frames, nil, true
		}
		if frames, rejected := r.exchangeFinalizationRejectLocked(originID, partnerID, origin, partner); rejected {
			return frames, nil, len(frames) > 0
		}
		return nil, &exchangeFinalizePlan{
			OriginID:     originID,
			PartnerID:    partnerID,
			Origin:       cloneExchangeCharacter(origin),
			Partner:      cloneExchangeCharacter(partner),
			OriginItems:  cloneExchangeDisplayedItems(r.exchangeItems[originID]),
			PartnerItems: cloneExchangeDisplayedItems(r.exchangeItems[partnerID]),
			OriginGold:   r.exchangeGold[originID],
			PartnerGold:  r.exchangeGold[partnerID],
		}, true
	}

	selfFrame := encodeExchangeAcceptFrame(1)
	peerFrame := encodeExchangeAcceptFrame(0)
	if !r.enqueueToEntityLocked(partnerID, [][]byte{peerFrame}) {
		return nil, nil, false
	}
	r.setExchangeAcceptedLocked(originID, true)
	return [][]byte{selfFrame}, nil, true
}

// CommitExchangeFinalize updates both characters, clears the exchange shell without
// partner-notify END (caller supplies END frames), and enqueues peer finalize frames.
// On commit-time busy-window drift it returns the same self-only START/ACCEPT busy
// info-chat frames for the commit requester (plan.OriginID) and leaves the shell open.
// On commit-time Check/Space/gold-overflow drift it returns dual-sided info-chat
// (self return + peer enqueue) and leaves the shell open.
func (r *sharedWorldRegistry) CommitExchangeFinalize(plan *exchangeFinalizePlan, updatedOrigin loginticket.Character, updatedPartner loginticket.Character, peerFrames [][]byte) ([][]byte, bool) {
	if r == nil || plan == nil || plan.OriginID == 0 || plan.PartnerID == 0 {
		return nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	partnerID, ok := r.exchangePartners[plan.OriginID]
	if !ok || partnerID != plan.PartnerID {
		return nil, false
	}
	if reverse, ok := r.exchangePartners[plan.PartnerID]; !ok || reverse != plan.OriginID {
		return nil, false
	}
	if _, ok := r.sessionEntryLocked(plan.OriginID); !ok {
		return nil, false
	}
	if _, ok := r.sessionEntryLocked(plan.PartnerID); !ok {
		return nil, false
	}
	// Revalidate busy windows / displayed item-gold / receiver preconditions at
	// commit time so post-AcceptExchange drift cannot finalize a stale plan.
	// Busy-window drift reuses ACCEPT's local-first requester/partner info-chat.
	if frames, busy := r.exchangeFinalizeCommitBusyRejectLocked(plan); busy {
		return frames, false
	}
	if frames, capped := r.exchangeFinalizeCommitGoldCarrierRejectLocked(plan); capped {
		return frames, false
	}
	if frames, rejected := r.exchangeFinalizeCommitCheckSpaceRejectLocked(plan); rejected {
		return frames, false
	}
	if !r.enqueueToEntityLocked(plan.PartnerID, peerFrames) {
		return nil, false
	}
	_ = r.entities.UpdatePlayer(plan.OriginID, updatedOrigin)
	r.lastKnownCharacters[plan.OriginID] = updatedOrigin
	_ = r.entities.UpdatePlayer(plan.PartnerID, updatedPartner)
	r.lastKnownCharacters[plan.PartnerID] = updatedPartner
	if !r.clearExchangeLocked(plan.OriginID, false) {
		return nil, false
	}
	return nil, true
}

func (r *sharedWorldRegistry) exchangeFinalizeCommitBusyRejectLocked(plan *exchangeFinalizePlan) ([][]byte, bool) {
	if r == nil || plan == nil || plan.OriginID == 0 || plan.PartnerID == 0 {
		return nil, false
	}
	// Mirror ACCEPT / START busy ordering: commit-requester (plan.OriginID) busy
	// wins over partner busy when both presentations are open.
	if r.hasMerchantWindowOpenLocked(plan.OriginID) || r.hasSafeboxWindowOpenLocked(plan.OriginID) || r.hasRefineWindowOpenLocked(plan.OriginID) || r.hasMyShopWindowOpenLocked(plan.OriginID) {
		return [][]byte{encodeExchangeRequesterMerchantBusyInfoFrame()}, true
	}
	if r.hasMerchantWindowOpenLocked(plan.PartnerID) || r.hasSafeboxWindowOpenLocked(plan.PartnerID) || r.hasRefineWindowOpenLocked(plan.PartnerID) || r.hasMyShopWindowOpenLocked(plan.PartnerID) {
		return [][]byte{encodeExchangePartnerMerchantBusyInfoFrame()}, true
	}
	return nil, false
}

func (r *sharedWorldRegistry) exchangeFinalizeCommitGoldCarrierRejectLocked(plan *exchangeFinalizePlan) ([][]byte, bool) {
	if r == nil || plan == nil || plan.OriginID == 0 || plan.PartnerID == 0 {
		return nil, false
	}
	origin, ok := r.playerCharacter(plan.OriginID)
	if !ok || characterAtBootstrapHPFloor(origin) {
		return nil, false
	}
	partner, ok := r.playerCharacter(plan.PartnerID)
	if !ok || characterAtBootstrapHPFloor(partner) {
		return nil, false
	}
	// Local-first: commit-requester over-cap wins when both sides drifted.
	if origin.Gold >= exchangeGoldPointChangeCarrierMax {
		return [][]byte{encodeExchangeRequesterGoldCarrierCapInfoFrame()}, true
	}
	if partner.Gold >= exchangeGoldPointChangeCarrierMax {
		return [][]byte{encodeExchangePartnerGoldCarrierCapInfoFrame()}, true
	}
	return nil, false
}

// exchangeFinalizeCommitCheckSpaceRejectLocked revalidates displayed item/gold and
// receiver preconditions. Check/Space/gold-overflow/Other failures emit dual-sided
// info-chat. Returns rejected=true when commit must fail closed (with or without frames).
func (r *sharedWorldRegistry) exchangeFinalizeCommitCheckSpaceRejectLocked(plan *exchangeFinalizePlan) ([][]byte, bool) {
	if r == nil || plan == nil || plan.OriginID == 0 || plan.PartnerID == 0 {
		return nil, true
	}
	origin, ok := r.playerCharacter(plan.OriginID)
	if !ok || characterAtBootstrapHPFloor(origin) {
		return nil, true
	}
	partner, ok := r.playerCharacter(plan.PartnerID)
	if !ok || characterAtBootstrapHPFloor(partner) {
		return nil, true
	}
	if !exchangeDisplayedItemsStillLive(plan.OriginItems, origin, r.itemTemplates) {
		return r.exchangeEmitFinalizeCheckRejectForCallerLocked(plan.OriginID, plan.OriginID, plan.PartnerID)
	}
	if !exchangeDisplayedItemsStillLive(plan.PartnerItems, partner, r.itemTemplates) {
		return r.exchangeEmitFinalizeCheckRejectForCallerLocked(plan.OriginID, plan.PartnerID, plan.OriginID)
	}
	if plan.OriginGold != 0 && uint64(plan.OriginGold) > origin.Gold {
		return r.exchangeEmitFinalizeCheckRejectForCallerLocked(plan.OriginID, plan.OriginID, plan.PartnerID)
	}
	if plan.PartnerGold != 0 && uint64(plan.PartnerGold) > partner.Gold {
		return r.exchangeEmitFinalizeCheckRejectForCallerLocked(plan.OriginID, plan.PartnerID, plan.OriginID)
	}
	// Oracle CheckSpace order: partner can take origin items, then origin can take
	// partner items. Space/gold-overflow/Other get dual-sided chat.
	switch r.exchangeRecipientRejectReasonLocked(partner, plan.OriginItems, plan.OriginGold) {
	case exchangeRecipientRejectSpace:
		return r.exchangeEmitFinalizeSpaceRejectForCallerLocked(plan.OriginID, plan.PartnerID, plan.OriginID)
	case exchangeRecipientRejectGoldOverflow:
		return r.exchangeEmitFinalizeGoldOverflowRejectForCallerLocked(plan.OriginID, plan.PartnerID, plan.OriginID)
	case exchangeRecipientRejectOther:
		return r.exchangeEmitFinalizeOtherRejectForCallerLocked(plan.OriginID, plan.PartnerID)
	}
	switch r.exchangeRecipientRejectReasonLocked(origin, plan.PartnerItems, plan.PartnerGold) {
	case exchangeRecipientRejectSpace:
		return r.exchangeEmitFinalizeSpaceRejectForCallerLocked(plan.OriginID, plan.OriginID, plan.PartnerID)
	case exchangeRecipientRejectGoldOverflow:
		return r.exchangeEmitFinalizeGoldOverflowRejectForCallerLocked(plan.OriginID, plan.OriginID, plan.PartnerID)
	case exchangeRecipientRejectOther:
		return r.exchangeEmitFinalizeOtherRejectForCallerLocked(plan.OriginID, plan.PartnerID)
	}
	return nil, false
}

func (r *sharedWorldRegistry) exchangeFinalizationPreconditionsLocked(originID uint64, partnerID uint64, origin loginticket.Character, partner loginticket.Character) bool {
	frames, rejected := r.exchangeFinalizationRejectLocked(originID, partnerID, origin, partner)
	_ = frames
	return !rejected
}

// exchangeFinalizationRejectLocked returns dual-sided Space/gold-overflow/Other chat
// when a receiver fails finalization preconditions. Returned frames are for the
// AcceptExchange caller (originID / second accepter).
func (r *sharedWorldRegistry) exchangeFinalizationRejectLocked(originID uint64, partnerID uint64, origin loginticket.Character, partner loginticket.Character) ([][]byte, bool) {
	if r == nil || originID == 0 || partnerID == 0 {
		return nil, true
	}
	switch r.exchangeRecipientRejectReasonLocked(partner, r.exchangeItems[originID], r.exchangeGold[originID]) {
	case exchangeRecipientRejectSpace:
		return r.exchangeEmitFinalizeSpaceRejectForCallerLocked(originID, partnerID, originID)
	case exchangeRecipientRejectGoldOverflow:
		return r.exchangeEmitFinalizeGoldOverflowRejectForCallerLocked(originID, partnerID, originID)
	case exchangeRecipientRejectOther:
		return r.exchangeEmitFinalizeOtherRejectForCallerLocked(originID, partnerID)
	}
	switch r.exchangeRecipientRejectReasonLocked(origin, r.exchangeItems[partnerID], r.exchangeGold[partnerID]) {
	case exchangeRecipientRejectSpace:
		return r.exchangeEmitFinalizeSpaceRejectForCallerLocked(originID, originID, partnerID)
	case exchangeRecipientRejectGoldOverflow:
		return r.exchangeEmitFinalizeGoldOverflowRejectForCallerLocked(originID, originID, partnerID)
	case exchangeRecipientRejectOther:
		return r.exchangeEmitFinalizeOtherRejectForCallerLocked(originID, partnerID)
	}
	return nil, false
}

// exchangeEmitFinalizeCheckRejectLocked is used by AcceptExchange second-accept
// Check failures. failedID receives CheckSelf; otherID receives CheckOther via
// queued frames. Returned frames go to the AcceptExchange caller (failedID when
// the caller's own Check failed, or the peer when the already-accepted partner
// Check failed — callers pass accordingly and return the self slice).
func (r *sharedWorldRegistry) exchangeEmitFinalizeCheckRejectLocked(failedID uint64, otherID uint64) ([][]byte, *exchangeFinalizePlan, bool) {
	frames, ok := r.exchangeEmitFinalizeCheckRejectForCallerLocked(failedID, failedID, otherID)
	if !ok {
		return nil, nil, false
	}
	return frames, nil, true
}

// exchangeEmitFinalizeCheckRejectForCallerLocked delivers CheckSelf to failedID
// and CheckOther to otherID. The slice returned is always the caller's self frame.
func (r *sharedWorldRegistry) exchangeEmitFinalizeCheckRejectForCallerLocked(callerID uint64, failedID uint64, otherID uint64) ([][]byte, bool) {
	selfFrame := encodeExchangeFinalizeCheckSelfInfoFrame()
	otherFrame := encodeExchangeFinalizeCheckOtherInfoFrame()
	switch callerID {
	case failedID:
		if !r.enqueueToEntityLocked(otherID, [][]byte{otherFrame}) {
			return nil, false
		}
		return [][]byte{selfFrame}, true
	case otherID:
		if !r.enqueueToEntityLocked(failedID, [][]byte{selfFrame}) {
			return nil, false
		}
		return [][]byte{otherFrame}, true
	default:
		return nil, false
	}
}

// exchangeEmitFinalizeSpaceRejectForCallerLocked delivers SpaceSelf to the full
// receiver and SpaceOther to the paired side. Returned frames are for callerID.
func (r *sharedWorldRegistry) exchangeEmitFinalizeSpaceRejectForCallerLocked(callerID uint64, fullReceiverID uint64, otherID uint64) ([][]byte, bool) {
	selfFrame := encodeExchangeFinalizeSpaceSelfInfoFrame()
	otherFrame := encodeExchangeFinalizeSpaceOtherInfoFrame()
	switch callerID {
	case fullReceiverID:
		if !r.enqueueToEntityLocked(otherID, [][]byte{otherFrame}) {
			return nil, false
		}
		return [][]byte{selfFrame}, true
	case otherID:
		if !r.enqueueToEntityLocked(fullReceiverID, [][]byte{selfFrame}) {
			return nil, false
		}
		return [][]byte{otherFrame}, true
	default:
		return nil, false
	}
}

// exchangeEmitFinalizeGoldOverflowRejectForCallerLocked delivers GoldOverflowSelf
// to the overflow receiver and GoldOverflowOther to the paired side.
func (r *sharedWorldRegistry) exchangeEmitFinalizeGoldOverflowRejectForCallerLocked(callerID uint64, overflowReceiverID uint64, otherID uint64) ([][]byte, bool) {
	selfFrame := encodeExchangeFinalizeGoldOverflowSelfInfoFrame()
	otherFrame := encodeExchangeFinalizeGoldOverflowOtherInfoFrame()
	switch callerID {
	case overflowReceiverID:
		if !r.enqueueToEntityLocked(otherID, [][]byte{otherFrame}) {
			return nil, false
		}
		return [][]byte{selfFrame}, true
	case otherID:
		if !r.enqueueToEntityLocked(overflowReceiverID, [][]byte{selfFrame}) {
			return nil, false
		}
		return [][]byte{otherFrame}, true
	default:
		return nil, false
	}
}

// exchangeEmitFinalizeOtherRejectForCallerLocked delivers the same catch-all
// Unknown error info-chat to both paired sides (oracle DB-dead / non-Check/Space abort wording).
func (r *sharedWorldRegistry) exchangeEmitFinalizeOtherRejectForCallerLocked(callerID uint64, partnerID uint64) ([][]byte, bool) {
	frame := encodeExchangeFinalizeOtherInfoFrame()
	if callerID == 0 || partnerID == 0 || callerID == partnerID {
		return nil, false
	}
	if !r.enqueueToEntityLocked(partnerID, [][]byte{frame}) {
		return nil, false
	}
	return [][]byte{frame}, true
}

func cloneExchangeCharacter(character loginticket.Character) loginticket.Character {
	cloned := loginticket.CloneCharacters([]loginticket.Character{character})
	if len(cloned) == 0 {
		return loginticket.Character{}
	}
	return cloned[0]
}

func cloneExchangeDisplayedItems(displayed map[uint8]exchangeDisplayedItem) map[uint8]exchangeDisplayedItem {
	if len(displayed) == 0 {
		return nil
	}
	cloned := make(map[uint8]exchangeDisplayedItem, len(displayed))
	for slot, item := range displayed {
		cloned[slot] = item
	}
	return cloned
}

const exchangeGoldPointChangeCarrierMax = uint64(1<<31 - 1)

type exchangeRecipientRejectReason int

const (
	exchangeRecipientRejectNone exchangeRecipientRejectReason = iota
	exchangeRecipientRejectSpace
	exchangeRecipientRejectGoldOverflow
	exchangeRecipientRejectOther
)

func (r *sharedWorldRegistry) exchangeRecipientCanAcceptLocked(recipient loginticket.Character, incoming map[uint8]exchangeDisplayedItem, incomingGold uint32) bool {
	return r.exchangeRecipientRejectReasonLocked(recipient, incoming, incomingGold) == exchangeRecipientRejectNone
}

// exchangeRecipientRejectReasonLocked classifies why a receiver cannot accept
// incoming displayed items/gold. Space is inventory placement/capacity failure
// after gold/id/template gates pass; GoldOverflow is signed gold-carrier overflow;
// Other covers id collision, selected-character/transfer guards, and invalid snapshots.
func (r *sharedWorldRegistry) exchangeRecipientRejectReasonLocked(recipient loginticket.Character, incoming map[uint8]exchangeDisplayedItem, incomingGold uint32) exchangeRecipientRejectReason {
	if r == nil || recipient.ID == 0 || characterAtBootstrapHPFloor(recipient) {
		return exchangeRecipientRejectOther
	}
	if incomingGold != 0 {
		if uint64(incomingGold) > exchangeGoldPointChangeCarrierMax || recipient.Gold > exchangeGoldPointChangeCarrierMax || recipient.Gold > exchangeGoldPointChangeCarrierMax-uint64(incomingGold) {
			return exchangeRecipientRejectGoldOverflow
		}
	}
	if len(incoming) == 0 {
		return exchangeRecipientRejectNone
	}
	if exchangeInventorySnapshotInvalidForDisplayedItems(recipient.Inventory) {
		return exchangeRecipientRejectOther
	}
	if exchangeEquipmentSnapshotInvalidForIncomingItems(recipient.Equipment) {
		return exchangeRecipientRejectOther
	}

	working := append([]inventory.ItemInstance(nil), recipient.Inventory...)
	seenIncomingIDs := make(map[uint64]struct{}, len(incoming))
	for _, displaySlot := range sortedExchangeDisplaySlots(incoming) {
		display := incoming[displaySlot]
		if display.ItemID == 0 || display.Vnum == 0 || display.Count == 0 || display.Slot >= inventory.CarriedInventorySlotCount {
			return exchangeRecipientRejectOther
		}
		if _, duplicate := seenIncomingIDs[display.ItemID]; duplicate {
			return exchangeRecipientRejectOther
		}
		seenIncomingIDs[display.ItemID] = struct{}{}
		if exchangeInventoryHasItemID(working, display.ItemID) || exchangeEquipmentHasItemID(recipient.Equipment, display.ItemID) {
			return exchangeRecipientRejectOther
		}
		template, ok := r.itemTemplates[display.Vnum]
		if !ok || !itemcatalog.ValidTemplate(template) || template.Vnum != display.Vnum || template.AntiStack || template.AntiGet || template.AntiDrop || template.AntiGive || template.AntiSell || !templateUsableByCharacter(recipient, template) || display.Count > template.MaxCount {
			return exchangeRecipientRejectOther
		}
		if reason := exchangePlaceIncomingDisplayedItemReason(&working, display, template); reason != exchangeRecipientRejectNone {
			return reason
		}
	}
	return exchangeRecipientRejectNone
}

func sortedExchangeDisplaySlots(displayed map[uint8]exchangeDisplayedItem) []uint8 {
	if len(displayed) == 0 {
		return nil
	}
	slots := make([]uint8, 0, len(displayed))
	for slot := range displayed {
		slots = append(slots, slot)
	}
	sort.Slice(slots, func(i int, j int) bool { return slots[i] < slots[j] })
	return slots
}

func exchangePlaceIncomingDisplayedItem(items *[]inventory.ItemInstance, display exchangeDisplayedItem, template itemcatalog.Template) bool {
	return exchangePlaceIncomingDisplayedItemReason(items, display, template) == exchangeRecipientRejectNone
}

// exchangePlaceIncomingDisplayedItemReason places one incoming displayed item into
// the working inventory. Only a missing free carried cell after merge attempts is
// classified as Space; over-template-max stacks, locked-only merge dead-ends that
// still fail for non-capacity reasons, and validation errors stay Other so they
// remain silent/no-frame beside the owned Space chat.
func exchangePlaceIncomingDisplayedItemReason(items *[]inventory.ItemInstance, display exchangeDisplayedItem, template itemcatalog.Template) exchangeRecipientRejectReason {
	if items == nil || display.Count == 0 || display.Count > template.MaxCount {
		return exchangeRecipientRejectOther
	}
	remaining := display.Count
	if template.Stackable {
		for idx := range *items {
			item := (*items)[idx]
			if item.Vnum == display.Vnum && item.Count > template.MaxCount {
				return exchangeRecipientRejectOther
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
				return exchangeRecipientRejectOther
			}
			(*items)[idx] = item
			remaining -= room
			if remaining == 0 {
				return exchangeRecipientRejectNone
			}
		}
	}
	if remaining == 0 {
		return exchangeRecipientRejectNone
	}
	if !template.Stackable && remaining != 1 {
		return exchangeRecipientRejectOther
	}
	slot, ok := exchangeNextFreeInventorySlot(*items)
	if !ok {
		// Locked compatible stacks are skipped for merge; when that leaves no free
		// cell, keep the older silent/no-frame reject instead of Space chat.
		if template.Stackable && exchangeHasLockedCompatibleStack(*items, display.Vnum) {
			return exchangeRecipientRejectOther
		}
		return exchangeRecipientRejectSpace
	}
	placed, err := (inventory.ItemInstance{ID: display.ItemID, Vnum: display.Vnum, Count: remaining}).WithInventorySlot(slot)
	if err != nil {
		return exchangeRecipientRejectOther
	}
	*items = append(*items, placed)
	sort.Slice(*items, func(i int, j int) bool {
		if (*items)[i].Slot != (*items)[j].Slot {
			return (*items)[i].Slot < (*items)[j].Slot
		}
		return (*items)[i].ID < (*items)[j].ID
	})
	return exchangeRecipientRejectNone
}

func exchangeNextFreeInventorySlot(items []inventory.ItemInstance) (inventory.SlotIndex, bool) {
	occupied := make(map[inventory.SlotIndex]struct{}, len(items))
	for _, item := range items {
		if item.Equipped || item.Slot >= inventory.CarriedInventorySlotCount {
			return 0, false
		}
		occupied[item.Slot] = struct{}{}
	}
	for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
		if _, exists := occupied[slot]; !exists {
			return slot, true
		}
	}
	return 0, false
}

func exchangeHasLockedCompatibleStack(items []inventory.ItemInstance, vnum uint32) bool {
	if vnum == 0 {
		return false
	}
	for _, item := range items {
		if item.Locked && !item.Equipped && item.Vnum == vnum {
			return true
		}
	}
	return false
}

func exchangeInventoryHasItemID(items []inventory.ItemInstance, id uint64) bool {
	if id == 0 {
		return false
	}
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func exchangeEquipmentHasItemID(items []inventory.ItemInstance, id uint64) bool {
	if id == 0 {
		return false
	}
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func exchangeDisplayedItemsStillLive(displayed map[uint8]exchangeDisplayedItem, live loginticket.Character, templates map[uint32]itemcatalog.Template) bool {
	if len(displayed) == 0 {
		return true
	}
	if exchangeInventorySnapshotInvalidForDisplayedItems(live.Inventory) {
		return false
	}
	for _, display := range displayed {
		if display.ItemID == 0 || display.Vnum == 0 || display.Count == 0 || display.Slot >= inventory.CarriedInventorySlotCount {
			return false
		}
		template, ok := templates[display.Vnum]
		if !ok || !itemcatalog.ValidTemplate(template) || template.Vnum != display.Vnum || template.AntiStack || template.AntiGet || template.AntiDrop || template.AntiGive || template.AntiSell || !templateUsableByCharacter(live, template) {
			return false
		}
		matches := 0
		for _, liveItem := range live.Inventory {
			if liveItem.Equipped || liveItem.Slot != display.Slot || liveItem.ID != display.ItemID || liveItem.Vnum != display.Vnum || liveItem.Count != display.Count || liveItem.Locked || liveItem.Count > template.MaxCount {
				continue
			}
			if err := liveItem.Validate(); err != nil {
				return false
			}
			matches++
		}
		if matches != 1 {
			return false
		}
	}
	return true
}

func templateUsableByCharacter(character loginticket.Character, template itemcatalog.Template) bool {
	return player.NewRuntime(character, player.SessionLink{}).CanUseTemplate(template)
}

func exchangeInventorySnapshotInvalidForDisplayedItems(items []inventory.ItemInstance) bool {
	seenSlots := make(map[inventory.SlotIndex]struct{}, len(items))
	seenIDs := make(map[uint64]struct{}, len(items))
	for _, item := range items {
		if item.Equipped || item.Slot >= inventory.CarriedInventorySlotCount {
			return true
		}
		if err := item.Validate(); err != nil {
			return true
		}
		if _, exists := seenSlots[item.Slot]; exists {
			return true
		}
		if _, exists := seenIDs[item.ID]; exists {
			return true
		}
		seenSlots[item.Slot] = struct{}{}
		seenIDs[item.ID] = struct{}{}
	}
	return false
}

func exchangeEquipmentSnapshotInvalidForIncomingItems(items []inventory.ItemInstance) bool {
	seenSlots := make(map[inventory.EquipmentSlot]struct{}, len(items))
	seenIDs := make(map[uint64]struct{}, len(items))
	for _, item := range items {
		if !item.Equipped || !item.EquipSlot.Valid() {
			return true
		}
		if err := item.Validate(); err != nil {
			return true
		}
		if _, exists := seenSlots[item.EquipSlot]; exists {
			return true
		}
		if _, exists := seenIDs[item.ID]; exists {
			return true
		}
		seenSlots[item.EquipSlot] = struct{}{}
		seenIDs[item.ID] = struct{}{}
	}
	return false
}

// HasExchangeDisplayedCarriedSlot reports whether the requester currently shows a
// carried inventory cell in the active exchange display shell. Displayed cells are
// fail-closed for same-socket mutations that would change that live item identity.
func (r *sharedWorldRegistry) HasExchangeDisplayedCarriedSlot(originID uint64, slot inventory.SlotIndex) bool {
	if r == nil || originID == 0 {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.exchangePartners[originID]; !ok {
		return false
	}
	for _, displayed := range r.exchangeItems[originID] {
		if displayed.Slot == slot && displayed.ItemID != 0 {
			return true
		}
	}
	return false
}

func (r *sharedWorldRegistry) RemoveExchangeItem(originID uint64, displaySlot uint8) ([][]byte, bool) {
	if r == nil || originID == 0 || displaySlot >= itemproto.ExchangeItemMaxNum {
		return nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	partnerID, ok := r.exchangePartners[originID]
	if !ok || partnerID == 0 {
		return nil, false
	}
	if _, ok := r.sessionEntryLocked(originID); !ok {
		return nil, false
	}
	if _, ok := r.sessionEntryLocked(partnerID); !ok {
		return nil, false
	}
	origin, ok := r.playerCharacter(originID)
	if !ok || characterAtBootstrapHPFloor(origin) {
		return nil, false
	}
	partner, ok := r.playerCharacter(partnerID)
	if !ok || characterAtBootstrapHPFloor(partner) {
		return nil, false
	}
	originItems := r.exchangeItems[originID]
	_, occupied := originItems[displaySlot]
	if !occupied {
		return nil, false
	}
	selfFrame := encodeExchangeItemDelFrame(1, displaySlot)
	peerFrame := encodeExchangeItemDelFrame(0, displaySlot)
	resetSelfFrames, resetPeerFrames := r.exchangeAcceptResetFramesLocked(originID, partnerID)
	selfFrames := append(resetSelfFrames, selfFrame)
	peerFrames := append(resetPeerFrames, peerFrame)
	if !r.enqueueToEntityLocked(partnerID, peerFrames) {
		return nil, false
	}
	delete(originItems, displaySlot)
	if len(originItems) == 0 {
		delete(r.exchangeItems, originID)
	}
	r.setExchangeAcceptedLocked(originID, false)
	r.setExchangeAcceptedLocked(partnerID, false)
	return selfFrames, true
}

func (r *sharedWorldRegistry) exchangeAcceptResetFramesLocked(originID uint64, partnerID uint64) ([][]byte, [][]byte) {
	if r == nil || originID == 0 || partnerID == 0 || len(r.exchangeAccepted) == 0 {
		return nil, nil
	}

	var selfFrames [][]byte
	var peerFrames [][]byte
	if r.exchangeAccepted[originID] {
		selfFrames = append(selfFrames, encodeExchangeAcceptFrameWithValue(1, 0))
		peerFrames = append(peerFrames, encodeExchangeAcceptFrameWithValue(0, 0))
	}
	if r.exchangeAccepted[partnerID] {
		peerFrames = append(peerFrames, encodeExchangeAcceptFrameWithValue(1, 0))
		selfFrames = append(selfFrames, encodeExchangeAcceptFrameWithValue(0, 0))
	}
	return selfFrames, peerFrames
}

func (r *sharedWorldRegistry) setExchangeAcceptedLocked(entityID uint64, accepted bool) {
	if r == nil || entityID == 0 {
		return
	}
	if r.exchangeAccepted == nil {
		r.exchangeAccepted = make(map[uint64]bool)
	}
	if accepted {
		r.exchangeAccepted[entityID] = true
		return
	}
	delete(r.exchangeAccepted, entityID)
}

func (r *sharedWorldRegistry) setExchangeGoldLocked(entityID uint64, amount uint32) {
	if r == nil || entityID == 0 {
		return
	}
	if r.exchangeGold == nil {
		r.exchangeGold = make(map[uint64]uint32)
	}
	if amount == 0 {
		delete(r.exchangeGold, entityID)
		return
	}
	r.exchangeGold[entityID] = amount
}

func (r *sharedWorldRegistry) clearExchangeLocked(originID uint64, notifyPartner bool) bool {
	if r == nil || originID == 0 || len(r.exchangePartners) == 0 {
		return false
	}
	partnerID, ok := r.exchangePartners[originID]
	if !ok || partnerID == 0 {
		return false
	}
	delete(r.exchangePartners, originID)
	delete(r.exchangePartners, partnerID)
	if r.exchangeItems != nil {
		delete(r.exchangeItems, originID)
		delete(r.exchangeItems, partnerID)
	}
	if r.exchangeAccepted != nil {
		delete(r.exchangeAccepted, originID)
		delete(r.exchangeAccepted, partnerID)
	}
	if r.exchangeGold != nil {
		delete(r.exchangeGold, originID)
		delete(r.exchangeGold, partnerID)
	}
	if notifyPartner {
		r.enqueueToEntityLocked(partnerID, [][]byte{encodeExchangeEndFrame()})
	}
	return true
}

func (r *sharedWorldRegistry) closeExchangeIfOutOfRangeLocked(originID uint64) {
	if r == nil || originID == 0 || len(r.exchangePartners) == 0 {
		return
	}
	partnerID, ok := r.exchangePartners[originID]
	if !ok || partnerID == 0 {
		return
	}
	origin, ok := r.playerCharacter(originID)
	if !ok {
		_ = r.clearExchangeLocked(originID, true)
		return
	}
	partner, ok := r.playerCharacter(partnerID)
	if !ok || !worldruntime.WithinExchangeDistance(origin, partner) {
		if r.clearExchangeLocked(originID, true) {
			r.enqueueToEntityLocked(originID, [][]byte{encodeExchangeEndFrame()})
		}
	}
}

func (r *sharedWorldRegistry) playerEntityForCharacterLocked(character loginticket.Character) (worldruntime.PlayerEntity, bool) {
	if r == nil || r.entities == nil || character.VID == 0 {
		return worldruntime.PlayerEntity{}, false
	}
	return r.entities.PlayerByVID(character.VID)
}

func (r *sharedWorldRegistry) sessionEntryForCharacterLocked(character loginticket.Character) (worldruntime.SessionEntry, bool) {
	playerEntity, ok := r.playerEntityForCharacterLocked(character)
	if !ok {
		return worldruntime.SessionEntry{}, false
	}
	return r.sessionEntryLocked(playerEntity.Entity.ID)
}

func (r *sharedWorldRegistry) clearStaticActorCombatStateLocked(entityID uint64) {
	if r == nil || entityID == 0 {
		return
	}
	if r.staticActorCombatHP != nil {
		delete(r.staticActorCombatHP, entityID)
	}
	if r.staticActorCombatRespawnAt != nil {
		delete(r.staticActorCombatRespawnAt, entityID)
	}
	if r.staticActorCombatSnapshot != nil {
		delete(r.staticActorCombatSnapshot, entityID)
	}
	if engagedBy := r.staticActorCombatEngagedBy[entityID]; engagedBy != 0 {
		delete(r.staticActorCombatEngagedBy, entityID)
		r.clearProximityAggroSuppressForActorLocked(entityID)
	}
	if r.staticActorProximityAggroSuppress != nil {
		delete(r.staticActorProximityAggroSuppress, entityID)
	}
	if r.staticActorDeathReward != nil {
		delete(r.staticActorDeathReward, entityID)
	}
}

func (r *sharedWorldRegistry) captureStaticActorCombatStateLocked() staticActorCombatStateSnapshot {
	if r == nil {
		return staticActorCombatStateSnapshot{}
	}
	return staticActorCombatStateSnapshot{
		HP:                     cloneUint64Uint8Map(r.staticActorCombatHP),
		RespawnAt:              cloneUint64TimeMap(r.staticActorCombatRespawnAt),
		Snapshot:               cloneUint64Uint64Map(r.staticActorCombatSnapshot),
		EngagedBy:              cloneUint64Uint64Map(r.staticActorCombatEngagedBy),
		ProximityAggroSuppress: cloneUint64Uint64SetMap(r.staticActorProximityAggroSuppress),
		DeathReward:            cloneStaticActorDeathRewardMap(r.staticActorDeathReward),
		SessionTargets:         cloneUint64Uint32Map(r.sessionCombatTargets),
		SessionRetaliations:    cloneCombatRetaliationTimerMap(r.sessionCombatRetaliations),
		NextSnapshotID:         r.nextStaticActorCombatSnapshotID,
	}
}

func (r *sharedWorldRegistry) restoreStaticActorCombatStateLocked(snapshot staticActorCombatStateSnapshot) {
	if r == nil {
		return
	}
	r.staticActorCombatHP = cloneUint64Uint8Map(snapshot.HP)
	r.staticActorCombatRespawnAt = cloneUint64TimeMap(snapshot.RespawnAt)
	r.staticActorCombatSnapshot = cloneUint64Uint64Map(snapshot.Snapshot)
	r.staticActorCombatEngagedBy = cloneUint64Uint64Map(snapshot.EngagedBy)
	r.staticActorProximityAggroSuppress = cloneUint64Uint64SetMap(snapshot.ProximityAggroSuppress)
	r.staticActorDeathReward = cloneStaticActorDeathRewardMap(snapshot.DeathReward)
	r.sessionCombatTargets = cloneUint64Uint32Map(snapshot.SessionTargets)
	r.sessionCombatRetaliations = cloneCombatRetaliationTimerMap(snapshot.SessionRetaliations)
	r.nextStaticActorCombatSnapshotID = snapshot.NextSnapshotID
}

// remapSpawnGroupCombatState overlays pending still-dead / live-damaged combat state plus
// proximity-suppress membership from a previous content-bundle snapshot onto newly registered
// actors that keep the same authored spawn_group_ref. Engagement and session combat ownership
// stay intentionally fail-closed across replacement; proximity suppress is remapped only for
// still-connected subject entity IDs so a still-inside owner cannot instantly reacquire.
func (r *sharedWorldRegistry) remapSpawnGroupCombatState(previousActors []StaticActorSnapshot, previous staticActorCombatStateSnapshot) {
	if r == nil || r.entities == nil || len(previousActors) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.remapSpawnGroupCombatStateLocked(previousActors, previous)
}

// remapStillDeadSpawnGroupCombatState keeps the historical still-dead-only call site name as an
// alias while content-bundle replacement remaps both still-dead and live-damaged HP.
func (r *sharedWorldRegistry) remapStillDeadSpawnGroupCombatState(previousActors []StaticActorSnapshot, previous staticActorCombatStateSnapshot) {
	r.remapSpawnGroupCombatState(previousActors, previous)
}

func (r *sharedWorldRegistry) remapSpawnGroupCombatStateLocked(previousActors []StaticActorSnapshot, previous staticActorCombatStateSnapshot) {
	if r == nil || r.entities == nil || len(previousActors) == 0 {
		return
	}
	stillDeadByRef := make(map[string]time.Time)
	damagedByRef := make(map[string]uint8)
	suppressByRef := make(map[string]map[uint64]struct{})
	for _, actor := range previousActors {
		ref := strings.TrimSpace(actor.SpawnGroupRef)
		if ref == "" || actor.EntityID == 0 {
			continue
		}
		if subjects := previous.ProximityAggroSuppress[actor.EntityID]; len(subjects) > 0 {
			cloned := make(map[uint64]struct{}, len(subjects))
			for subjectID := range subjects {
				if subjectID == 0 {
					continue
				}
				cloned[subjectID] = struct{}{}
			}
			if len(cloned) > 0 {
				suppressByRef[ref] = cloned
			}
		}
		currentHP, hpOK := previous.HP[actor.EntityID]
		respawnAt, respawnOK := previous.RespawnAt[actor.EntityID]
		if !hpOK {
			continue
		}
		if currentHP == 0 {
			if !respawnOK || respawnAt.IsZero() {
				continue
			}
			stillDeadByRef[ref] = respawnAt
			continue
		}
		if respawnOK && !respawnAt.IsZero() {
			continue
		}
		maxHP, ok := worldruntime.BootstrapStaticActorCurrentHP(actor.CombatProfile)
		if !ok || currentHP >= maxHP {
			continue
		}
		if _, percentOK := worldruntime.BootstrapStaticActorHPPercent(actor.CombatProfile, currentHP); !percentOK {
			continue
		}
		damagedByRef[ref] = currentHP
	}
	if len(stillDeadByRef) == 0 && len(damagedByRef) == 0 && len(suppressByRef) == 0 {
		return
	}
	for _, actor := range r.entities.AllStaticActors() {
		ref := strings.TrimSpace(actor.SpawnGroupRef)
		if ref == "" || actor.Entity.ID == 0 {
			continue
		}
		if respawnAt, ok := stillDeadByRef[ref]; ok {
			r.restoreStillDeadSpawnGroupCombatStateLocked(actor.Entity.ID, respawnAt)
		} else if currentHP, ok := damagedByRef[ref]; ok {
			r.restoreDamagedSpawnGroupCombatStateLocked(actor.Entity.ID, currentHP)
		}
		if subjects, ok := suppressByRef[ref]; ok {
			r.restoreProximityAggroSuppressForSpawnGroupLocked(actor.Entity.ID, subjects)
		}
	}
}

func (r *sharedWorldRegistry) restoreProximityAggroSuppressForSpawnGroupLocked(entityID uint64, subjects map[uint64]struct{}) {
	if r == nil || entityID == 0 || len(subjects) == 0 {
		return
	}
	actor, ok := r.entities.StaticActor(entityID)
	if !ok || strings.TrimSpace(actor.SpawnGroupRef) == "" {
		return
	}
	for subjectID := range subjects {
		if subjectID == 0 {
			continue
		}
		if r.sessionDirectory == nil {
			continue
		}
		if _, ok := r.sessionDirectory.Lookup(subjectID); !ok {
			continue
		}
		r.markProximityAggroSuppressLocked(entityID, subjectID)
	}
}

func (r *sharedWorldRegistry) restoreStillDeadSpawnGroupCombatState(entityID uint64, respawnAt time.Time) bool {
	if r == nil || entityID == 0 || respawnAt.IsZero() {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.restoreStillDeadSpawnGroupCombatStateLocked(entityID, respawnAt)
}

func (r *sharedWorldRegistry) restoreStillDeadSpawnGroupCombatStateLocked(entityID uint64, respawnAt time.Time) bool {
	if r == nil || entityID == 0 || respawnAt.IsZero() {
		return false
	}
	actor, ok := r.entities.StaticActor(entityID)
	if !ok || strings.TrimSpace(actor.SpawnGroupRef) == "" {
		return false
	}
	if r.staticActorCombatHP == nil {
		r.staticActorCombatHP = make(map[uint64]uint8)
	}
	if r.staticActorCombatRespawnAt == nil {
		r.staticActorCombatRespawnAt = make(map[uint64]time.Time)
	}
	r.staticActorCombatHP[entityID] = 0
	r.staticActorCombatRespawnAt[entityID] = respawnAt
	r.assignStaticActorCombatSnapshotLocked(entityID)
	if engagedBy := r.staticActorCombatEngagedBy[entityID]; engagedBy != 0 {
		delete(r.staticActorCombatEngagedBy, entityID)
		r.clearProximityAggroSuppressForActorLocked(entityID)
	}
	if r.staticActorProximityAggroSuppress != nil {
		delete(r.staticActorProximityAggroSuppress, entityID)
	}
	return true
}

type spawnGroupCombatPersistenceState struct {
	HP        uint8
	RespawnAt time.Time
}

func (r *sharedWorldRegistry) spawnGroupCombatPersistenceState() map[uint64]spawnGroupCombatPersistenceState {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.spawnGroupCombatPersistenceStateLocked()
}

func (r *sharedWorldRegistry) spawnGroupCombatPersistenceStateLocked() map[uint64]spawnGroupCombatPersistenceState {
	if r == nil || r.entities == nil || len(r.staticActorCombatHP) == 0 {
		return nil
	}
	out := make(map[uint64]spawnGroupCombatPersistenceState)
	for _, actor := range r.entities.AllStaticActors() {
		if strings.TrimSpace(actor.SpawnGroupRef) == "" || actor.Entity.ID == 0 {
			continue
		}
		currentHP, hpOK := r.staticActorCombatHP[actor.Entity.ID]
		if !hpOK {
			continue
		}
		respawnAt, respawnOK := r.staticActorCombatRespawnAt[actor.Entity.ID]
		if currentHP == 0 {
			if !respawnOK || respawnAt.IsZero() {
				continue
			}
			out[actor.Entity.ID] = spawnGroupCombatPersistenceState{HP: 0, RespawnAt: respawnAt}
			continue
		}
		if respawnOK && !respawnAt.IsZero() {
			continue
		}
		maxHP, ok := worldruntime.BootstrapStaticActorCurrentHP(actor.CombatKind)
		if !ok || currentHP >= maxHP {
			continue
		}
		if _, percentOK := worldruntime.BootstrapStaticActorHPPercent(actor.CombatKind, currentHP); !percentOK {
			continue
		}
		out[actor.Entity.ID] = spawnGroupCombatPersistenceState{HP: currentHP}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (r *sharedWorldRegistry) restoreDamagedSpawnGroupCombatState(entityID uint64, currentHP uint8) bool {
	if r == nil || entityID == 0 || currentHP == 0 {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.restoreDamagedSpawnGroupCombatStateLocked(entityID, currentHP)
}

func (r *sharedWorldRegistry) restoreDamagedSpawnGroupCombatStateLocked(entityID uint64, currentHP uint8) bool {
	if r == nil || entityID == 0 || currentHP == 0 {
		return false
	}
	actor, ok := r.entities.StaticActor(entityID)
	if !ok || strings.TrimSpace(actor.SpawnGroupRef) == "" {
		return false
	}
	maxHP, ok := worldruntime.BootstrapStaticActorCurrentHP(actor.CombatKind)
	if !ok || currentHP >= maxHP {
		return false
	}
	if _, percentOK := worldruntime.BootstrapStaticActorHPPercent(actor.CombatKind, currentHP); !percentOK {
		return false
	}
	if r.staticActorCombatHP == nil {
		r.staticActorCombatHP = make(map[uint64]uint8)
	}
	r.staticActorCombatHP[entityID] = currentHP
	if r.staticActorCombatRespawnAt != nil {
		delete(r.staticActorCombatRespawnAt, entityID)
	}
	r.assignStaticActorCombatSnapshotLocked(entityID)
	return true
}

func cloneUint64Uint8Map(in map[uint64]uint8) map[uint64]uint8 {
	if len(in) == 0 {
		return nil
	}
	out := make(map[uint64]uint8, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneUint64Uint64Map(in map[uint64]uint64) map[uint64]uint64 {
	if len(in) == 0 {
		return nil
	}
	out := make(map[uint64]uint64, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneUint64Uint64SetMap(in map[uint64]map[uint64]struct{}) map[uint64]map[uint64]struct{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[uint64]map[uint64]struct{}, len(in))
	for key, values := range in {
		if len(values) == 0 {
			continue
		}
		cloned := make(map[uint64]struct{}, len(values))
		for value := range values {
			cloned[value] = struct{}{}
		}
		out[key] = cloned
	}
	return out
}

func cloneUint64Uint32Map(in map[uint64]uint32) map[uint64]uint32 {
	if len(in) == 0 {
		return nil
	}
	out := make(map[uint64]uint32, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneCombatRetaliationTimerMap(in map[uint64]combatRetaliationTimer) map[uint64]combatRetaliationTimer {
	if len(in) == 0 {
		return nil
	}
	out := make(map[uint64]combatRetaliationTimer, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneUint64TimeMap(in map[uint64]time.Time) map[uint64]time.Time {
	if len(in) == 0 {
		return nil
	}
	out := make(map[uint64]time.Time, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneStaticActorDeathRewardMap(in map[uint64]worldruntime.StaticActorDeathReward) map[uint64]worldruntime.StaticActorDeathReward {
	if len(in) == 0 {
		return nil
	}
	out := make(map[uint64]worldruntime.StaticActorDeathReward, len(in))
	for key, value := range in {
		out[key] = value.Clone()
	}
	return out
}

func (r *sharedWorldRegistry) setStaticActorCombatEngagementLocked(entityID uint64, subjectID uint64) {
	if r == nil || entityID == 0 || subjectID == 0 {
		return
	}
	if r.staticActorCombatEngagedBy == nil {
		r.staticActorCombatEngagedBy = make(map[uint64]uint64)
	}
	if existing := r.staticActorCombatEngagedBy[entityID]; existing != 0 {
		return
	}
	r.staticActorCombatEngagedBy[entityID] = subjectID
	r.clearProximityAggroSuppressForActorLocked(entityID)
}

func (r *sharedWorldRegistry) StaticActorCombatEngagedBySubject(entityID uint64, subjectID uint64) bool {
	if r == nil || entityID == 0 || subjectID == 0 {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.staticActorCombatEngagedBy[entityID] == subjectID
}

func (r *sharedWorldRegistry) ClearStaticActorCombatEngagement(entityID uint64, subjectID uint64) bool {
	if r == nil || entityID == 0 || subjectID == 0 {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.staticActorCombatEngagedBy[entityID] != subjectID {
		return false
	}
	delete(r.staticActorCombatEngagedBy, entityID)
	r.markProximityAggroSuppressLocked(entityID, subjectID)
	return true
}

func (r *sharedWorldRegistry) ClearStaticActorCombatEngagementsBySubject(subjectID uint64) {
	if r == nil || subjectID == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clearStaticActorCombatEngagementsBySubjectLocked(subjectID)
}

// ReleaseProximitySpawnGroupEngagementsOutsideAggroRadius releases proximity-only
// aggro-lite engagements owned by subjectID when that owner is no longer inside
// the engaged spawn-backed actor's effective aggro radius. Selected-target
// ownership is intentionally ignored here; callers use this after MOVE /
// SYNC_POSITION when activeCombatTargetVID == 0. Released entity IDs are
// returned so the session can cancel delayed retaliation and chase schedules.
func (r *sharedWorldRegistry) ReleaseProximitySpawnGroupEngagementsOutsideAggroRadius(subjectID uint64) []uint64 {
	if r == nil || subjectID == 0 || r.entities == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	subject, ok := r.playerCharacter(subjectID)
	if !ok || characterAtBootstrapHPFloor(subject) {
		return nil
	}
	subjectPos := worldruntime.PositionFromCharacter(subject)
	if !subjectPos.Valid() {
		return nil
	}

	released := make([]uint64, 0)
	for entityID, engagedBy := range r.staticActorCombatEngagedBy {
		if engagedBy != subjectID || entityID == 0 {
			continue
		}
		actor, ok := r.entities.StaticActor(entityID)
		if !ok || actor.SpawnGroupRef == "" || !staticActorSpawnGroupAggroLiteCombatKind(actor.CombatKind) {
			continue
		}
		evaluation, ok := worldruntime.EvaluateStaticActorSpawnAggroAcquisition(actor, subjectPos, worldruntime.EffectiveStaticActorSpawnAggroRadiusForActor(actor))
		if ok && evaluation.Acquired {
			continue
		}
		r.releaseStaticActorCombatEngagementLocked(actor, false)
		released = append(released, entityID)
	}
	sort.Slice(released, func(i, j int) bool { return released[i] < released[j] })
	return released
}

func (r *sharedWorldRegistry) clearStaticActorCombatEngagementsBySubjectLocked(subjectID uint64) {
	if r == nil || subjectID == 0 || len(r.staticActorCombatEngagedBy) == 0 {
		return
	}
	for entityID, engagedBy := range r.staticActorCombatEngagedBy {
		if engagedBy != subjectID {
			continue
		}
		delete(r.staticActorCombatEngagedBy, entityID)
		// Always mark the releasing subject. seedProximity skips bootstrap HP-floor
		// candidates, but death-floor /restart_here recovery needs that same owner
		// suppressed while still inside aggro radius after live HP is restored.
		r.markProximityAggroSuppressLocked(entityID, subjectID)
		if actor, ok := r.entities.StaticActor(entityID); ok {
			r.seedProximityAggroSuppressForInsideCandidatesLocked(actor)
		}
	}
}

func (r *sharedWorldRegistry) markProximityAggroSuppressLocked(entityID uint64, subjectID uint64) {
	if r == nil || entityID == 0 || subjectID == 0 {
		return
	}
	if r.staticActorProximityAggroSuppress == nil {
		r.staticActorProximityAggroSuppress = make(map[uint64]map[uint64]struct{})
	}
	subjects := r.staticActorProximityAggroSuppress[entityID]
	if subjects == nil {
		subjects = make(map[uint64]struct{})
		r.staticActorProximityAggroSuppress[entityID] = subjects
	}
	subjects[subjectID] = struct{}{}
}

func (r *sharedWorldRegistry) clearProximityAggroSuppressForActorLocked(entityID uint64) {
	if r == nil || entityID == 0 {
		return
	}
	if r.staticActorProximityAggroSuppress != nil {
		delete(r.staticActorProximityAggroSuppress, entityID)
	}
	if r.pendingProximityAggroSuppressByVID == nil {
		return
	}
	for vid, actors := range r.pendingProximityAggroSuppressByVID {
		delete(actors, entityID)
		if len(actors) == 0 {
			delete(r.pendingProximityAggroSuppressByVID, vid)
		}
	}
}

// detachProximityAggroSuppressSubjectLocked removes subjectID from every live
// actor suppress set and, when vid is non-zero, parks those actor IDs under
// that VID so a later Join can rematerialize suppress under a new subject ID.
func (r *sharedWorldRegistry) detachProximityAggroSuppressSubjectLocked(subjectID uint64, vid uint32) {
	if r == nil || subjectID == 0 || r.staticActorProximityAggroSuppress == nil {
		return
	}
	for actorID, subjects := range r.staticActorProximityAggroSuppress {
		if _, ok := subjects[subjectID]; !ok {
			continue
		}
		delete(subjects, subjectID)
		if len(subjects) == 0 {
			delete(r.staticActorProximityAggroSuppress, actorID)
		}
		if vid == 0 {
			continue
		}
		if r.pendingProximityAggroSuppressByVID == nil {
			r.pendingProximityAggroSuppressByVID = make(map[uint32]map[uint64]struct{})
		}
		actors := r.pendingProximityAggroSuppressByVID[vid]
		if actors == nil {
			actors = make(map[uint64]struct{})
			r.pendingProximityAggroSuppressByVID[vid] = actors
		}
		actors[actorID] = struct{}{}
	}
}

// claimPendingProximityAggroSuppressLocked rematerializes suppress parked under
// vid onto newSubjectID after Leave → Join identity change.
func (r *sharedWorldRegistry) claimPendingProximityAggroSuppressLocked(vid uint32, newSubjectID uint64) {
	if r == nil || vid == 0 || newSubjectID == 0 || r.pendingProximityAggroSuppressByVID == nil {
		return
	}
	actors := r.pendingProximityAggroSuppressByVID[vid]
	delete(r.pendingProximityAggroSuppressByVID, vid)
	for actorID := range actors {
		r.markProximityAggroSuppressLocked(actorID, newSubjectID)
	}
}

func (r *sharedWorldRegistry) proximityAggroSuppressActiveLocked(entityID uint64, subjectID uint64) bool {
	if r == nil || entityID == 0 || subjectID == 0 || r.staticActorProximityAggroSuppress == nil {
		return false
	}
	_, ok := r.staticActorProximityAggroSuppress[entityID][subjectID]
	return ok
}

func (r *sharedWorldRegistry) clearProximityAggroSuppressIfOutsideRadiusLocked(actor worldruntime.StaticEntity, candidates []worldruntime.SpawnAggroCandidate) {
	if r == nil || actor.Entity.ID == 0 || r.staticActorProximityAggroSuppress == nil {
		return
	}
	subjects := r.staticActorProximityAggroSuppress[actor.Entity.ID]
	if len(subjects) == 0 {
		return
	}
	inside := make(map[uint64]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate.EntityID == 0 {
			continue
		}
		evaluation, ok := worldruntime.EvaluateStaticActorSpawnAggroAcquisition(actor, candidate.Position, worldruntime.EffectiveStaticActorSpawnAggroRadiusForActor(actor))
		if !ok || !evaluation.Acquired {
			continue
		}
		inside[candidate.EntityID] = struct{}{}
	}
	for subjectID := range subjects {
		if _, stillInside := inside[subjectID]; stillInside {
			continue
		}
		delete(subjects, subjectID)
	}
	if len(subjects) == 0 {
		delete(r.staticActorProximityAggroSuppress, actor.Entity.ID)
	}
}

func (r *sharedWorldRegistry) seedProximityAggroSuppressForInsideCandidatesLocked(actor worldruntime.StaticEntity) {
	if r == nil || actor.Entity.ID == 0 || r.sessionDirectory == nil {
		return
	}
	for _, sessionID := range r.sessionDirectory.EntityIDs() {
		player, ok := r.playerCharacter(sessionID)
		if !ok || characterAtBootstrapHPFloor(player) {
			continue
		}
		evaluation, ok := worldruntime.EvaluateStaticActorSpawnAggroAcquisition(actor, worldruntime.PositionFromCharacter(player), worldruntime.EffectiveStaticActorSpawnAggroRadiusForActor(actor))
		if !ok || !evaluation.Acquired {
			continue
		}
		r.markProximityAggroSuppressLocked(actor.Entity.ID, sessionID)
	}
}

func (r *sharedWorldRegistry) releaseStaticActorCombatEngagementLocked(actor worldruntime.StaticEntity, seedInsideSuppress bool) {
	if r == nil || actor.Entity.ID == 0 {
		return
	}
	engagedBy := r.staticActorCombatEngagedBy[actor.Entity.ID]
	if engagedBy != 0 {
		delete(r.staticActorCombatEngagedBy, actor.Entity.ID)
	}
	if seedInsideSuppress {
		r.seedProximityAggroSuppressForInsideCandidatesLocked(actor)
		return
	}
	if engagedBy != 0 {
		r.markProximityAggroSuppressLocked(actor.Entity.ID, engagedBy)
	}
}

func (r *sharedWorldRegistry) scheduleStaticActorCombatRespawnLocked(actor worldruntime.StaticEntity) {
	if r == nil || actor.Entity.ID == 0 {
		return
	}
	delay, ok := worldruntime.BootstrapStaticActorRespawnDelay(actor.CombatKind)
	if !ok || delay <= 0 {
		if r.staticActorCombatRespawnAt != nil {
			delete(r.staticActorCombatRespawnAt, actor.Entity.ID)
		}
		return
	}
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	if r.staticActorCombatRespawnAt == nil {
		r.staticActorCombatRespawnAt = make(map[uint64]time.Time)
	}
	r.staticActorCombatRespawnAt[actor.Entity.ID] = now.Add(delay)
}

func (r *sharedWorldRegistry) assignStaticActorCombatSnapshotLocked(entityID uint64) uint64 {
	if r == nil || entityID == 0 {
		return 0
	}
	if r.staticActorCombatSnapshot == nil {
		r.staticActorCombatSnapshot = make(map[uint64]uint64)
	}
	r.nextStaticActorCombatSnapshotID++
	if r.nextStaticActorCombatSnapshotID == 0 {
		r.nextStaticActorCombatSnapshotID = 1
	}
	r.staticActorCombatSnapshot[entityID] = r.nextStaticActorCombatSnapshotID
	return r.nextStaticActorCombatSnapshotID
}

func (r *sharedWorldRegistry) staticActorCombatSnapshotLocked(entityID uint64) uint64 {
	if r == nil || entityID == 0 || r.staticActorCombatSnapshot == nil {
		return 0
	}
	return r.staticActorCombatSnapshot[entityID]
}

func (r *sharedWorldRegistry) staticActorDeathRewardLocked(actor worldruntime.StaticEntity) worldruntime.StaticActorDeathReward {
	if r == nil || actor.Entity.ID == 0 {
		return worldruntime.StaticActorDeathReward{}
	}
	if r.staticActorDeathReward != nil {
		if reward, ok := r.staticActorDeathReward[actor.Entity.ID]; ok {
			return reward.Clone()
		}
	}
	if !actor.DeathReward.Empty() {
		return actor.DeathReward.Clone()
	}
	if actor.SpawnGroupRef == "" {
		return worldruntime.StaticActorDeathReward{}
	}
	reward, _ := worldruntime.BootstrapStaticActorDeathReward(actor.CombatKind)
	return reward
}

func (r *sharedWorldRegistry) overrideStaticActorDeathReward(entityID uint64, reward worldruntime.StaticActorDeathReward) bool {
	if r == nil || entityID == 0 || !worldruntime.ValidStaticActorDeathReward(reward) {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.entities.StaticActor(entityID); !ok {
		return false
	}
	if r.staticActorDeathReward == nil {
		r.staticActorDeathReward = make(map[uint64]worldruntime.StaticActorDeathReward)
	}
	r.staticActorDeathReward[entityID] = reward.Clone()
	return true
}

func (r *sharedWorldRegistry) setStaticActorKillQuestCreditLocked(entityID uint64, credit staticActorKillQuestCredit) bool {
	if r == nil || entityID == 0 || !validStaticActorKillQuestCredit(credit) {
		return false
	}
	credit = credit.Clone()
	if credit.Empty() {
		if r.staticActorKillQuestCredit != nil {
			delete(r.staticActorKillQuestCredit, entityID)
		}
		return true
	}
	if r.staticActorKillQuestCredit == nil {
		r.staticActorKillQuestCredit = make(map[uint64]staticActorKillQuestCredit)
	}
	r.staticActorKillQuestCredit[entityID] = credit
	return true
}

func (r *sharedWorldRegistry) staticActorKillQuestCreditLocked(entityID uint64) staticActorKillQuestCredit {
	if r == nil || entityID == 0 || r.staticActorKillQuestCredit == nil {
		return staticActorKillQuestCredit{}
	}
	credit, ok := r.staticActorKillQuestCredit[entityID]
	if !ok {
		return staticActorKillQuestCredit{}
	}
	return credit.Clone()
}

func (r *sharedWorldRegistry) StaticActorKillQuestCredit(entityID uint64) (staticActorKillQuestCredit, bool) {
	if r == nil || entityID == 0 {
		return staticActorKillQuestCredit{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	credit := r.staticActorKillQuestCreditLocked(entityID)
	if credit.Empty() {
		return staticActorKillQuestCredit{}, false
	}
	return credit, true
}

func (r *sharedWorldRegistry) setStaticActorKillQuestCredit(entityID uint64, credit staticActorKillQuestCredit) bool {
	if r == nil || entityID == 0 {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.entities.StaticActor(entityID); !ok {
		return false
	}
	return r.setStaticActorKillQuestCreditLocked(entityID, credit)
}

func (r *sharedWorldRegistry) ensureStaticActorCombatCurrentHPLocked(actor worldruntime.StaticEntity) (uint8, bool) {
	if r == nil || actor.Entity.ID == 0 || actor.CombatKind == "" {
		return 0, false
	}
	if currentHP, ok := r.staticActorCombatHP[actor.Entity.ID]; ok {
		if _, percentOK := worldruntime.BootstrapStaticActorHPPercent(actor.CombatKind, currentHP); percentOK {
			return currentHP, true
		}
	}
	currentHP, ok := worldruntime.BootstrapStaticActorCurrentHP(actor.CombatKind)
	if !ok {
		return 0, false
	}
	if r.staticActorCombatHP == nil {
		r.staticActorCombatHP = make(map[uint64]uint8)
	}
	r.staticActorCombatHP[actor.Entity.ID] = currentHP
	return currentHP, true
}

func (r *sharedWorldRegistry) syncStaticActorCombatStateLocked(actor worldruntime.StaticEntity) {
	if r == nil || actor.Entity.ID == 0 {
		return
	}
	if actor.CombatKind == "" {
		r.clearStaticActorCombatStateLocked(actor.Entity.ID)
		return
	}
	currentHP, ok := r.ensureStaticActorCombatCurrentHPLocked(actor)
	if !ok {
		r.clearStaticActorCombatStateLocked(actor.Entity.ID)
		return
	}
	if currentHP > 0 && r.staticActorCombatRespawnAt != nil {
		delete(r.staticActorCombatRespawnAt, actor.Entity.ID)
	}
	r.assignStaticActorCombatSnapshotLocked(actor.Entity.ID)
}

func (r *sharedWorldRegistry) FlushReadyStaticActorRespawns() {
	if r == nil || r.entities == nil {
		return
	}
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.staticActorCombatRespawnAt) == 0 {
		return
	}
	dueIDs := make([]uint64, 0, len(r.staticActorCombatRespawnAt))
	for entityID, readyAt := range r.staticActorCombatRespawnAt {
		if readyAt.IsZero() || now.Before(readyAt) {
			continue
		}
		dueIDs = append(dueIDs, entityID)
	}
	sort.Slice(dueIDs, func(i int, j int) bool {
		return dueIDs[i] < dueIDs[j]
	})
	for _, entityID := range dueIDs {
		r.flushReadyStaticActorRespawnLocked(entityID)
	}
}

func (r *sharedWorldRegistry) FlushReadyStaticActorRespawn(entityID uint64) bool {
	if r == nil || r.entities == nil || entityID == 0 {
		return false
	}
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	readyAt, ok := r.staticActorCombatRespawnAt[entityID]
	if !ok || readyAt.IsZero() || now.Before(readyAt) {
		return false
	}
	return r.flushReadyStaticActorRespawnLocked(entityID)
}

func (r *sharedWorldRegistry) flushReadyStaticActorRespawnLocked(entityID uint64) bool {
	if r == nil || r.entities == nil || entityID == 0 {
		return false
	}
	actor, ok := r.entities.StaticActor(entityID)
	if !ok || actor.CombatKind == "" {
		if r.staticActorCombatRespawnAt != nil {
			delete(r.staticActorCombatRespawnAt, entityID)
		}
		return false
	}
	currentHP, ok := r.ensureStaticActorCombatCurrentHPLocked(actor)
	if !ok || currentHP > 0 {
		if r.staticActorCombatRespawnAt != nil {
			delete(r.staticActorCombatRespawnAt, entityID)
		}
		return false
	}
	resetHP, ok := worldruntime.BootstrapStaticActorCurrentHP(actor.CombatKind)
	if !ok {
		if r.staticActorCombatRespawnAt != nil {
			delete(r.staticActorCombatRespawnAt, entityID)
		}
		return false
	}
	respawnActor := actor
	if actor.SpawnGroupRef != "" {
		if actor.SpawnHome.Valid() {
			respawnActor.Position = actor.SpawnHome
		} else {
			respawnActor.SpawnHome = actor.Position
		}
	}
	deleteRaw, encodable := encodeStaticActorDeleteFrame(actor)
	if !encodable {
		return false
	}
	addFrames := encodeStaticActorVisibilityFrames(respawnActor)
	if len(addFrames) == 0 {
		return false
	}
	targetDiff := r.scopesLocked().RelocateStaticActorTargetDiff(actor, respawnActor)
	if !respawnActor.Position.Equal(actor.Position) || !respawnActor.SpawnHome.Equal(actor.SpawnHome) {
		updated, ok := r.entities.UpdateStaticActor(respawnActor)
		if !ok {
			return false
		}
		respawnActor = updated
	}
	if r.staticActorCombatRespawnAt != nil {
		delete(r.staticActorCombatRespawnAt, entityID)
	}
	if r.staticActorCombatHP == nil {
		r.staticActorCombatHP = make(map[uint64]uint8)
	}
	r.staticActorCombatHP[entityID] = resetHP
	r.assignStaticActorCombatSnapshotLocked(entityID)
	r.releaseStaticActorCombatEngagementLocked(respawnActor, true)
	if targetVID, ok := worldruntime.StaticActorVisibilityVID(actor); ok {
		r.clearSelectedCombatTargetsLocked(targetVID, 0)
	}
	refreshFrames := make([][]byte, 0, 1+len(addFrames))
	refreshFrames = append(refreshFrames, deleteRaw)
	refreshFrames = append(refreshFrames, addFrames...)
	for _, target := range targetDiff.RetainedVisibleTargets {
		if characterAtBootstrapHPFloor(target.Character) {
			continue
		}
		r.enqueueToEntityLocked(target.Entity.ID, refreshFrames)
	}
	for _, target := range targetDiff.RemovedVisibleTargets {
		if characterAtBootstrapHPFloor(target.Character) {
			continue
		}
		r.enqueueToEntityLocked(target.Entity.ID, [][]byte{deleteRaw})
	}
	for _, target := range targetDiff.AddedVisibleTargets {
		if characterAtBootstrapHPFloor(target.Character) {
			continue
		}
		r.enqueueToEntityLocked(target.Entity.ID, addFrames)
	}
	return true
}

func (r *sharedWorldRegistry) clearSessionCombatTargetLocked(entityID uint64) {
	if r == nil || entityID == 0 {
		return
	}
	if r.sessionCombatTargets != nil {
		delete(r.sessionCombatTargets, entityID)
	}
	if r.sessionCombatRetaliations != nil {
		delete(r.sessionCombatRetaliations, entityID)
	}
}

func (r *sharedWorldRegistry) clearInvalidSessionCombatTargetLocked(entityID uint64) [][]byte {
	if r == nil || entityID == 0 {
		return nil
	}
	if _, ok := r.sessionCombatTargetLocked(entityID); !ok {
		return nil
	}
	if _, ok := r.combatTargetSnapshotLocked(entityID); ok {
		return nil
	}
	r.clearSessionCombatTargetLocked(entityID)
	r.clearStaticActorCombatEngagementsBySubjectLocked(entityID)
	return [][]byte{combatproto.EncodeServerClearTarget()}
}

func (r *sharedWorldRegistry) setSessionCombatTargetLocked(entityID uint64, targetVID uint32) {
	if r == nil || entityID == 0 {
		return
	}
	if targetVID == 0 {
		r.clearSessionCombatTargetLocked(entityID)
		return
	}
	if r.sessionCombatTargets == nil {
		r.sessionCombatTargets = make(map[uint64]uint32)
	}
	r.sessionCombatTargets[entityID] = targetVID
}

func (r *sharedWorldRegistry) sessionCombatTargetLocked(entityID uint64) (uint32, bool) {
	if r == nil || entityID == 0 || r.sessionCombatTargets == nil {
		return 0, false
	}
	targetVID, ok := r.sessionCombatTargets[entityID]
	if !ok || targetVID == 0 {
		return 0, false
	}
	return targetVID, true
}

func (r *sharedWorldRegistry) clearSelectedCombatTargetsLocked(targetVID uint32, excludeEntityID uint64) {
	if r == nil || targetVID == 0 || len(r.sessionCombatTargets) == 0 {
		return
	}
	clearTargetRaw := combatproto.EncodeServerClearTarget()
	for entityID, selectedTargetVID := range r.sessionCombatTargets {
		if selectedTargetVID != targetVID || (excludeEntityID != 0 && entityID == excludeEntityID) {
			continue
		}
		delete(r.sessionCombatTargets, entityID)
		if r.sessionCombatRetaliations != nil {
			delete(r.sessionCombatRetaliations, entityID)
		}
		if clearTargetRaw != nil {
			r.enqueueToEntityLocked(entityID, [][]byte{clearTargetRaw})
		}
	}
}

func (r *sharedWorldRegistry) clearOtherSessionCombatTargetsLocked(ownerID uint64, targetVID uint32) {
	if ownerID == 0 {
		return
	}
	r.clearSelectedCombatTargetsLocked(targetVID, ownerID)
}

func (r *sharedWorldRegistry) staticActorAggroLiteBlocksFreshTargetLocked(subjectID uint64, actor worldruntime.StaticEntity, targetVID uint32) bool {
	if r == nil || subjectID == 0 || actor.Entity.ID == 0 || targetVID == 0 {
		return false
	}
	if actor.SpawnGroupRef == "" || !staticActorSpawnGroupAggroLiteCombatKind(actor.CombatKind) {
		return false
	}
	engagedBy, ok := r.staticActorCombatEngagedBy[actor.Entity.ID]
	if !ok || engagedBy == 0 || engagedBy == subjectID {
		return false
	}
	if _, ok := r.sessionEntryLocked(engagedBy); !ok {
		delete(r.staticActorCombatEngagedBy, actor.Entity.ID)
		r.markProximityAggroSuppressLocked(actor.Entity.ID, engagedBy)
		r.clearSessionCombatTargetLocked(engagedBy)
		return false
	}
	engagedOwner, ok := r.playerCharacter(engagedBy)
	if !ok || characterAtBootstrapHPFloor(engagedOwner) {
		delete(r.staticActorCombatEngagedBy, actor.Entity.ID)
		r.markProximityAggroSuppressLocked(actor.Entity.ID, engagedBy)
		r.clearSessionCombatTargetLocked(engagedBy)
		return false
	}
	return true
}

func (r *sharedWorldRegistry) staticActorAggroLiteBlocksFreshTargetReadOnlyLocked(subjectID uint64, actor worldruntime.StaticEntity, targetVID uint32) bool {
	if r == nil || subjectID == 0 || actor.Entity.ID == 0 || targetVID == 0 {
		return false
	}
	if actor.SpawnGroupRef == "" || !staticActorSpawnGroupAggroLiteCombatKind(actor.CombatKind) {
		return false
	}
	engagedBy, ok := r.staticActorCombatEngagedBy[actor.Entity.ID]
	if !ok || engagedBy == 0 || engagedBy == subjectID {
		return false
	}
	if _, ok := r.sessionEntryLocked(engagedBy); !ok {
		return false
	}
	engagedOwner, ok := r.playerCharacter(engagedBy)
	return ok && !characterAtBootstrapHPFloor(engagedOwner)
}

func staticActorSpawnGroupAggroLiteCombatKind(combatKind string) bool {
	_, ok := worldruntime.BootstrapStaticActorCombatProfileDefaults(combatKind)
	return ok
}

// AcquireProximitySpawnGroupAggro scans live unengaged spawn-backed practice
// mobs and establishes aggro-lite engagement for the nearest eligible live
// same-map session inside the actor's effective aggro radius. Acquisition
// itself stays pure: it does not invent selected-target ownership, emit
// immediate retaliation, or arm delayed retaliation. Chase scheduling is synced
// by the runtime consumer after engagement is newly established, and the engaged
// owner's session may separately arm the delayed server-origin retaliation
// cadence from that same engagement. After an explicit engagement release, the
// same candidate stays suppressed until it leaves the aggro radius and
// re-enters.
func (r *sharedWorldRegistry) AcquireProximitySpawnGroupAggro() []uint64 {
	if r == nil || r.entities == nil || r.sessionDirectory == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	sessionIDs := r.sessionDirectory.EntityIDs()
	if len(sessionIDs) == 0 {
		return nil
	}
	candidates := make([]worldruntime.SpawnAggroCandidate, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		player, ok := r.playerCharacter(sessionID)
		if !ok || characterAtBootstrapHPFloor(player) {
			continue
		}
		candidates = append(candidates, worldruntime.SpawnAggroCandidate{
			EntityID: sessionID,
			Position: worldruntime.PositionFromCharacter(player),
		})
	}
	if len(candidates) == 0 {
		return nil
	}

	acquired := make([]uint64, 0)
	for _, actor := range r.entities.AllStaticActors() {
		if actor.Entity.ID == 0 || actor.SpawnGroupRef == "" || !staticActorSpawnGroupAggroLiteCombatKind(actor.CombatKind) {
			continue
		}
		r.clearProximityAggroSuppressIfOutsideRadiusLocked(actor, candidates)
		if existing := r.staticActorCombatEngagedBy[actor.Entity.ID]; existing != 0 {
			continue
		}
		if currentHP, ok := r.staticActorCombatHP[actor.Entity.ID]; ok && currentHP == 0 {
			continue
		}
		if _, waiting := r.staticActorCombatRespawnAt[actor.Entity.ID]; waiting {
			continue
		}
		eligible := make([]worldruntime.SpawnAggroCandidate, 0, len(candidates))
		for _, candidate := range candidates {
			if r.proximityAggroSuppressActiveLocked(actor.Entity.ID, candidate.EntityID) {
				continue
			}
			eligible = append(eligible, candidate)
		}
		selected, ok := worldruntime.SelectStaticActorSpawnAggroCandidate(actor, eligible, worldruntime.EffectiveStaticActorSpawnAggroRadiusForActor(actor))
		if !ok {
			continue
		}
		before := r.staticActorCombatEngagedBy[actor.Entity.ID]
		r.setStaticActorCombatEngagementLocked(actor.Entity.ID, selected.EntityID)
		if before == 0 && r.staticActorCombatEngagedBy[actor.Entity.ID] == selected.EntityID {
			acquired = append(acquired, actor.Entity.ID)
		}
	}
	sort.Slice(acquired, func(i, j int) bool { return acquired[i] < acquired[j] })
	return acquired
}

// EngagedSpawnGroupRetaliationArmTargets returns deterministic delayed-retaliation
// arm descriptors for live spawn-backed practice mobs currently engaged by
// subjectID. It does not invent selected-target ownership and does not mutate
// retaliation timers; the session/runtime consumer decides whether to arm.
func (r *sharedWorldRegistry) EngagedSpawnGroupRetaliationArmTargets(subjectID uint64) []engagedSpawnGroupRetaliationArmTarget {
	if r == nil || subjectID == 0 || r.entities == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	targets := make([]engagedSpawnGroupRetaliationArmTarget, 0)
	for _, actor := range r.entities.AllStaticActors() {
		if actor.Entity.ID == 0 || actor.SpawnGroupRef == "" || !staticActorSpawnGroupAggroLiteCombatKind(actor.CombatKind) {
			continue
		}
		if r.staticActorCombatEngagedBy[actor.Entity.ID] != subjectID {
			continue
		}
		if currentHP, ok := r.staticActorCombatHP[actor.Entity.ID]; ok && currentHP == 0 {
			continue
		}
		if _, waiting := r.staticActorCombatRespawnAt[actor.Entity.ID]; waiting {
			continue
		}
		snapshotVersion := r.staticActorCombatSnapshotLocked(actor.Entity.ID)
		if snapshotVersion == 0 {
			continue
		}
		targets = append(targets, engagedSpawnGroupRetaliationArmTarget{
			TargetVID:       uint32(actor.Entity.ID),
			SnapshotVersion: snapshotVersion,
		})
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].TargetVID == targets[j].TargetVID {
			return targets[i].SnapshotVersion < targets[j].SnapshotVersion
		}
		return targets[i].TargetVID < targets[j].TargetVID
	})
	return targets
}

func (r *sharedWorldRegistry) SetSessionCombatTarget(entityID uint64, targetVID uint32) bool {
	if r == nil || entityID == 0 {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.sessionEntryLocked(entityID); !ok {
		return false
	}
	r.setSessionCombatTargetLocked(entityID, targetVID)
	return true
}

func (r *sharedWorldRegistry) SetMerchantWindowOpen(entityID uint64, open bool) bool {
	if r == nil || entityID == 0 {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.sessionEntryLocked(entityID); !ok {
		return false
	}
	r.setMerchantWindowOpenLocked(entityID, open)
	return true
}

func (r *sharedWorldRegistry) setMerchantWindowOpenLocked(entityID uint64, open bool) {
	if r == nil || entityID == 0 {
		return
	}
	if !open {
		if r.sessionMerchantWindows != nil {
			delete(r.sessionMerchantWindows, entityID)
		}
		return
	}
	if r.sessionMerchantWindows == nil {
		r.sessionMerchantWindows = make(map[uint64]bool)
	}
	r.sessionMerchantWindows[entityID] = true
}

func (r *sharedWorldRegistry) hasMerchantWindowOpenLocked(entityID uint64) bool {
	if r == nil || entityID == 0 || r.sessionMerchantWindows == nil {
		return false
	}
	return r.sessionMerchantWindows[entityID]
}

func (r *sharedWorldRegistry) clearMerchantWindowOpenLocked(entityID uint64) {
	r.setMerchantWindowOpenLocked(entityID, false)
}

func (r *sharedWorldRegistry) SetSafeboxWindowOpen(entityID uint64, open bool) bool {
	if r == nil || entityID == 0 {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.sessionEntryLocked(entityID); !ok {
		return false
	}
	r.setSafeboxWindowOpenLocked(entityID, open)
	return true
}

func (r *sharedWorldRegistry) setSafeboxWindowOpenLocked(entityID uint64, open bool) {
	if r == nil || entityID == 0 {
		return
	}
	if !open {
		if r.sessionSafeboxWindows != nil {
			delete(r.sessionSafeboxWindows, entityID)
		}
		return
	}
	if r.sessionSafeboxWindows == nil {
		r.sessionSafeboxWindows = make(map[uint64]bool)
	}
	r.sessionSafeboxWindows[entityID] = true
}

func (r *sharedWorldRegistry) hasSafeboxWindowOpenLocked(entityID uint64) bool {
	if r == nil || entityID == 0 || r.sessionSafeboxWindows == nil {
		return false
	}
	return r.sessionSafeboxWindows[entityID]
}

func (r *sharedWorldRegistry) clearSafeboxWindowOpenLocked(entityID uint64) {
	r.setSafeboxWindowOpenLocked(entityID, false)
}

func (r *sharedWorldRegistry) SetRefineWindowOpen(entityID uint64, open bool) bool {
	if r == nil || entityID == 0 {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.sessionEntryLocked(entityID); !ok {
		return false
	}
	r.setRefineWindowOpenLocked(entityID, open)
	return true
}

func (r *sharedWorldRegistry) setRefineWindowOpenLocked(entityID uint64, open bool) {
	if r == nil || entityID == 0 {
		return
	}
	if !open {
		if r.sessionRefineWindows != nil {
			delete(r.sessionRefineWindows, entityID)
		}
		return
	}
	if r.sessionRefineWindows == nil {
		r.sessionRefineWindows = make(map[uint64]bool)
	}
	r.sessionRefineWindows[entityID] = true
}

func (r *sharedWorldRegistry) hasRefineWindowOpenLocked(entityID uint64) bool {
	if r == nil || entityID == 0 || r.sessionRefineWindows == nil {
		return false
	}
	return r.sessionRefineWindows[entityID]
}

func (r *sharedWorldRegistry) clearRefineWindowOpenLocked(entityID uint64) {
	r.setRefineWindowOpenLocked(entityID, false)
}

func (r *sharedWorldRegistry) SetMyShopWindowOpen(entityID uint64, open bool, sign string) bool {
	if r == nil || entityID == 0 {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.sessionEntryLocked(entityID); !ok {
		return false
	}
	r.setMyShopWindowOpenLocked(entityID, open, sign)
	return true
}

func (r *sharedWorldRegistry) setMyShopWindowOpenLocked(entityID uint64, open bool, sign string) {
	if r == nil || entityID == 0 {
		return
	}
	if !open {
		if r.sessionMyShopWindows != nil {
			delete(r.sessionMyShopWindows, entityID)
		}
		return
	}
	sign = strings.TrimSpace(sign)
	if sign == "" {
		if r.sessionMyShopWindows != nil {
			delete(r.sessionMyShopWindows, entityID)
		}
		return
	}
	if r.sessionMyShopWindows == nil {
		r.sessionMyShopWindows = make(map[uint64]string)
	}
	r.sessionMyShopWindows[entityID] = sign
}

func (r *sharedWorldRegistry) hasMyShopWindowOpenLocked(entityID uint64) bool {
	if r == nil || entityID == 0 || r.sessionMyShopWindows == nil {
		return false
	}
	_, ok := r.sessionMyShopWindows[entityID]
	return ok
}

func (r *sharedWorldRegistry) myShopSignLocked(entityID uint64) (string, bool) {
	if r == nil || entityID == 0 || r.sessionMyShopWindows == nil {
		return "", false
	}
	sign, ok := r.sessionMyShopWindows[entityID]
	if !ok || strings.TrimSpace(sign) == "" {
		return "", false
	}
	return sign, true
}

func (r *sharedWorldRegistry) clearMyShopWindowOpenLocked(entityID uint64) {
	r.setMyShopWindowOpenLocked(entityID, false, "")
}

func (r *sharedWorldRegistry) encodeMyShopSignRematerializationFrameLocked(host loginticket.Character) ([]byte, bool) {
	if r == nil || characterAtBootstrapHPFloor(host) {
		return nil, false
	}
	hostEntity, ok := r.playerEntityForCharacterLocked(host)
	if !ok || hostEntity.Entity.ID == 0 {
		return nil, false
	}
	sign, ok := r.myShopSignLocked(hostEntity.Entity.ID)
	if !ok {
		return nil, false
	}
	return shopproto.EncodeServerShopSign(shopproto.ServerShopSignPacket{
		VID:  host.VID,
		Sign: sign,
	}), true
}

func (r *sharedWorldRegistry) appendMyShopSignRematerializationLocked(frames [][]byte, host loginticket.Character) [][]byte {
	raw, ok := r.encodeMyShopSignRematerializationFrameLocked(host)
	if !ok {
		return frames
	}
	return append(frames, raw)
}

// PeerVisibilityBootstrapFramesWithMyShopSign builds ordinary peer add/info/update
// bootstrap frames and, when the peer still has an open private shop, appends one
// rematerialized live GC::SHOP_SIGN after those frames.
func (r *sharedWorldRegistry) PeerVisibilityBootstrapFramesWithMyShopSign(peer loginticket.Character) [][]byte {
	if r == nil {
		return encodePeerVisibilityBootstrapFramesWithTemplates(peer, nil)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	frames := encodePeerVisibilityBootstrapFramesWithTemplates(peer, r.itemTemplates)
	return r.appendMyShopSignRematerializationLocked(frames, peer)
}

func (r *sharedWorldRegistry) SetSessionCombatRetaliation(entityID uint64, targetVID uint32, snapshotVersion uint64, readyAt time.Time) bool {
	if r == nil || entityID == 0 || targetVID == 0 || snapshotVersion == 0 || readyAt.IsZero() {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.sessionEntryLocked(entityID); !ok {
		return false
	}
	selectedTarget, ok := r.sessionCombatTargetLocked(entityID)
	if !ok || selectedTarget != targetVID {
		return false
	}
	if r.sessionCombatRetaliations == nil {
		r.sessionCombatRetaliations = make(map[uint64]combatRetaliationTimer)
	}
	r.sessionCombatRetaliations[entityID] = combatRetaliationTimer{TargetVID: targetVID, SnapshotVersion: snapshotVersion, ReadyAt: readyAt}
	return true
}

func (r *sharedWorldRegistry) ClearSessionCombatRetaliation(entityID uint64) {
	if r == nil || entityID == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sessionCombatRetaliations != nil {
		delete(r.sessionCombatRetaliations, entityID)
	}
}

func (r *sharedWorldRegistry) CombatTargetSnapshot(entityID uint64) (CombatTargetSnapshot, bool) {
	if r == nil || entityID == 0 {
		return CombatTargetSnapshot{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	return r.combatTargetSnapshotLocked(entityID)
}

func (r *sharedWorldRegistry) CombatTargetSnapshotByName(name string) (CombatTargetSnapshot, bool) {
	if r == nil || name == "" {
		return CombatTargetSnapshot{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	player, ok := r.scopesLocked().PlayerByExactName(name)
	if !ok || player.Entity.ID == 0 {
		return CombatTargetSnapshot{}, false
	}
	return r.combatTargetSnapshotLocked(player.Entity.ID)
}

func (r *sharedWorldRegistry) CombatTargetSnapshots() []CombatTargetSnapshot {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.sessionCombatTargets) == 0 {
		return nil
	}
	entityIDs := make([]uint64, 0, len(r.sessionCombatTargets))
	for entityID := range r.sessionCombatTargets {
		entityIDs = append(entityIDs, entityID)
	}
	sort.Slice(entityIDs, func(i, j int) bool { return entityIDs[i] < entityIDs[j] })

	snapshots := make([]CombatTargetSnapshot, 0, len(entityIDs))
	for _, entityID := range entityIDs {
		snapshot, ok := r.combatTargetSnapshotLocked(entityID)
		if !ok {
			continue
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func (r *sharedWorldRegistry) CombatTargetSnapshotsForMap(mapIndex uint32) ([]CombatTargetSnapshot, bool) {
	if r == nil || mapIndex == 0 {
		return nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	knownMap := false
	for _, snapshot := range r.mapOccupancySnapshotsLocked() {
		if snapshot.MapIndex == mapIndex {
			knownMap = true
			break
		}
	}
	if !knownMap {
		return nil, false
	}
	if len(r.sessionCombatTargets) == 0 {
		return []CombatTargetSnapshot{}, true
	}

	entityIDs := make([]uint64, 0, len(r.sessionCombatTargets))
	for entityID := range r.sessionCombatTargets {
		entityIDs = append(entityIDs, entityID)
	}
	sort.Slice(entityIDs, func(i, j int) bool { return entityIDs[i] < entityIDs[j] })

	snapshots := make([]CombatTargetSnapshot, 0)
	for _, entityID := range entityIDs {
		snapshot, ok := r.combatTargetSnapshotLocked(entityID)
		if !ok || snapshot.Subject.MapIndex != mapIndex {
			continue
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, true
}

func (r *sharedWorldRegistry) StaticActorRespawns() []StaticActorRespawnSnapshot {
	if r == nil || r.entities == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.staticActorCombatRespawnAt) == 0 {
		return nil
	}
	entityIDs := make([]uint64, 0, len(r.staticActorCombatRespawnAt))
	for entityID := range r.staticActorCombatRespawnAt {
		entityIDs = append(entityIDs, entityID)
	}
	sort.Slice(entityIDs, func(i, j int) bool { return entityIDs[i] < entityIDs[j] })

	respawns := make([]StaticActorRespawnSnapshot, 0, len(entityIDs))
	for _, entityID := range entityIDs {
		respawn, ok := r.staticActorRespawnLocked(entityID)
		if !ok {
			continue
		}
		respawns = append(respawns, respawn)
	}
	return respawns
}

func (r *sharedWorldRegistry) StaticActorRespawn(entityID uint64) (StaticActorRespawnSnapshot, bool) {
	if r == nil || r.entities == nil || entityID == 0 {
		return StaticActorRespawnSnapshot{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.staticActorRespawnLocked(entityID)
}

func (r *sharedWorldRegistry) StaticActorRespawnsForMap(mapIndex uint32) ([]StaticActorRespawnSnapshot, bool) {
	if r == nil || r.entities == nil || mapIndex == 0 {
		return nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.scopesLocked().StaticActorsForMap(mapIndex); !ok {
		return nil, false
	}
	if len(r.staticActorCombatRespawnAt) == 0 {
		return []StaticActorRespawnSnapshot{}, true
	}
	entityIDs := make([]uint64, 0, len(r.staticActorCombatRespawnAt))
	for entityID := range r.staticActorCombatRespawnAt {
		entityIDs = append(entityIDs, entityID)
	}
	sort.Slice(entityIDs, func(i, j int) bool { return entityIDs[i] < entityIDs[j] })

	respawns := make([]StaticActorRespawnSnapshot, 0)
	for _, entityID := range entityIDs {
		respawn, ok := r.staticActorRespawnLocked(entityID)
		if !ok || respawn.Actor.MapIndex != mapIndex {
			continue
		}
		respawns = append(respawns, respawn)
	}
	return respawns, true
}

func (r *sharedWorldRegistry) staticActorRespawnLocked(entityID uint64) (StaticActorRespawnSnapshot, bool) {
	if r == nil || r.entities == nil || entityID == 0 {
		return StaticActorRespawnSnapshot{}, false
	}
	readyAt, ok := r.staticActorCombatRespawnAt[entityID]
	if !ok {
		return StaticActorRespawnSnapshot{}, false
	}
	actor, ok := r.entities.StaticActor(entityID)
	if !ok {
		return StaticActorRespawnSnapshot{}, false
	}
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	remaining := readyAt.Sub(now).Milliseconds()
	if remaining < 0 {
		remaining = 0
	}
	return StaticActorRespawnSnapshot{
		EntityID:    entityID,
		ReadyAt:     readyAt,
		RemainingMs: remaining,
		Actor:       r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, actor)),
	}, true
}

func (r *sharedWorldRegistry) combatTargetSnapshotLocked(entityID uint64) (CombatTargetSnapshot, bool) {
	if _, ok := r.sessionEntryLocked(entityID); !ok {
		return CombatTargetSnapshot{}, false
	}
	targetVID, ok := r.sessionCombatTargets[entityID]
	if !ok || targetVID == 0 {
		return CombatTargetSnapshot{}, false
	}
	subject, ok := r.playerCharacter(entityID)
	if !ok || characterAtBootstrapHPFloor(subject) {
		return CombatTargetSnapshot{}, false
	}
	actor, ok := r.scopesLocked().VisibleStaticActorByVID(subject, targetVID)
	if !ok || actor.Entity.ID == 0 {
		return CombatTargetSnapshot{}, false
	}
	if !worldruntime.StaticActorWithinInteractionRange(subject, actor, staticActorCombatTargetMaxDistance) {
		return CombatTargetSnapshot{}, false
	}
	if !staticActorCombatKindTargetable(actor.CombatKind) {
		return CombatTargetSnapshot{}, false
	}
	if actor.SpawnGroupRef != "" {
		leash, ok := worldruntime.EvaluateStaticActorCurrentSpawnLeash(actor, worldruntime.EffectiveStaticActorSpawnLeashRadiusForActor(actor))
		if !ok || leash.ReturnRequired {
			return CombatTargetSnapshot{}, false
		}
	}
	if r.staticActorAggroLiteBlocksFreshTargetReadOnlyLocked(entityID, actor, targetVID) {
		return CombatTargetSnapshot{}, false
	}
	currentSnapshotVersion := r.staticActorCombatSnapshotLocked(actor.Entity.ID)
	if currentSnapshotVersion == 0 {
		return CombatTargetSnapshot{}, false
	}
	currentHP, ok := r.ensureStaticActorCombatCurrentHPLocked(actor)
	if !ok || currentHP == 0 {
		return CombatTargetSnapshot{}, false
	}
	hpPercent, ok := worldruntime.BootstrapStaticActorHPPercent(actor.CombatKind, currentHP)
	if !ok {
		return CombatTargetSnapshot{}, false
	}
	actorSnapshot := r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, actor))
	defaults, defaultsOK := worldruntime.BootstrapStaticActorCombatProfileDefaults(actor.CombatKind)
	normalAttackDamage, damageOK := worldruntime.BootstrapStaticActorNormalAttackDamage(actor.CombatKind)
	snapshot := CombatTargetSnapshot{
		SubjectEntityID: entityID,
		Subject:         worldruntime.ConnectedCharacterSnapshotFor(r.topology, subject),
		TargetVID:       targetVID,
		SnapshotVersion: currentSnapshotVersion,
		HPPercent:       hpPercent,
		TargetCurrentHP: currentHP,
		Actor:           actorSnapshot,
	}
	if defaultsOK {
		snapshot.TargetMaxHP = defaults.MaxHP
		snapshot.TargetAttackValue = defaults.AttackValue
		snapshot.TargetDefenseValue = defaults.DefenseValue
	}
	if damageOK {
		snapshot.NormalAttackDamage = normalAttackDamage
	}
	engagedBy := r.staticActorCombatEngagedBy[actor.Entity.ID]
	if engagedBy != 0 {
		snapshot.EngagedByEntityID = engagedBy
		if engagedCharacter, ok := r.playerCharacter(engagedBy); ok {
			engagedSnapshot := worldruntime.ConnectedCharacterSnapshotFor(r.topology, engagedCharacter)
			snapshot.EngagedBy = &engagedSnapshot
		}
		if actor.SpawnGroupRef != "" && staticActorSpawnGroupAggroLiteCombatKind(actor.CombatKind) {
			if retaliationPointDelta, ok := worldruntime.BootstrapStaticActorRetaliationPointDelta(actor.CombatKind); ok {
				snapshot.RetaliationPointDelta = retaliationPointDelta
				snapshot.RetaliationServerOrigin = true
				if timer, ok := r.sessionCombatRetaliations[entityID]; ok && timer.TargetVID == targetVID && timer.SnapshotVersion == currentSnapshotVersion && !timer.ReadyAt.IsZero() {
					readyAt := timer.ReadyAt
					snapshot.RetaliationPending = true
					snapshot.RetaliationReadyAt = &readyAt
					now := time.Now()
					if r.now != nil {
						now = r.now()
					}
					remaining := readyAt.Sub(now).Milliseconds()
					if remaining < 0 {
						remaining = 0
					}
					snapshot.RetaliationRemainingMs = &remaining
				}
			}
		}
	}
	return snapshot, true
}

func (r *sharedWorldRegistry) ClearSessionCombatTarget(entityID uint64) {
	if r == nil || entityID == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.clearSessionCombatTargetLocked(entityID)
}

func (r *sharedWorldRegistry) enqueueToEntityLocked(entityID uint64, frames [][]byte) bool {
	entry, ok := r.sessionEntryLocked(entityID)
	if !ok || entry.FrameSink == nil {
		return false
	}
	entry.FrameSink.Enqueue(frames)
	return true
}

func (r *sharedWorldRegistry) EnqueueToEntity(entityID uint64, frames [][]byte) bool {
	if r == nil || entityID == 0 || len(frames) == 0 {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.enqueueToEntityLocked(entityID, frames)
}

func (r *sharedWorldRegistry) EnqueueStaticActorFramesToVisiblePeers(actorEntityID uint64, excludeEntityID uint64, frames [][]byte) int {
	if r == nil || r.entities == nil || actorEntityID == 0 || len(frames) == 0 {
		return 0
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	actor, ok := r.entities.StaticActor(actorEntityID)
	if !ok {
		return 0
	}
	delivered := 0
	for _, target := range r.scopesLocked().VisibleTargetsForStaticActor(actor) {
		if target.Entity.ID == excludeEntityID || characterAtBootstrapHPFloor(target.Character) {
			continue
		}
		if r.enqueueToEntityLocked(target.Entity.ID, frames) {
			delivered++
		}
	}
	return delivered
}

func (r *sharedWorldRegistry) enqueueToCharacterLocked(character loginticket.Character, frames [][]byte) bool {
	entry, ok := r.sessionEntryForCharacterLocked(character)
	if !ok || entry.FrameSink == nil {
		return false
	}
	entry.FrameSink.Enqueue(frames)
	return true
}

func invokeSessionRelocator(entry worldruntime.SessionEntry, mapIndex uint32, x int32, y int32) (RelocationPreview, bool) {
	if entry.Relocator == nil {
		return RelocationPreview{}, false
	}
	result, ok := entry.Relocator(mapIndex, x, y)
	if !ok {
		return RelocationPreview{}, false
	}
	preview, ok := result.(RelocationPreview)
	if !ok {
		return RelocationPreview{}, false
	}
	return preview, true
}

func (r *sharedWorldRegistry) reclaimableStaleDuplicateIDsLocked(character loginticket.Character) ([]uint64, bool) {
	if r == nil || r.entities == nil {
		return nil, false
	}

	candidateIDs := make(map[uint64]struct{}, 2)
	if character.VID != 0 {
		if playerEntity, ok := r.entities.PlayerByVID(character.VID); ok {
			candidateIDs[playerEntity.Entity.ID] = struct{}{}
		}
	}
	if character.Name != "" {
		if playerEntity, ok := r.entities.PlayerByName(character.Name); ok {
			candidateIDs[playerEntity.Entity.ID] = struct{}{}
		}
	}
	for entityID, known := range r.lastKnownCharacters {
		if character.VID != 0 && known.VID == character.VID {
			candidateIDs[entityID] = struct{}{}
			continue
		}
		if character.Name != "" && known.Name == character.Name {
			candidateIDs[entityID] = struct{}{}
		}
	}
	if len(candidateIDs) == 0 {
		return nil, false
	}

	staleIDs := make([]uint64, 0, len(candidateIDs))
	for entityID := range candidateIDs {
		if _, ok := r.sessionEntryLocked(entityID); ok {
			return nil, true
		}
		staleIDs = append(staleIDs, entityID)
	}
	sort.Slice(staleIDs, func(i int, j int) bool {
		return staleIDs[i] < staleIDs[j]
	})
	return staleIDs, false
}

func (r *sharedWorldRegistry) removeStaleOwnershipLocked(entityIDs []uint64) bool {
	if r == nil || len(entityIDs) == 0 {
		return false
	}
	groundChanged := false
	for _, entityID := range entityIDs {
		currentCharacter, ok := r.playerCharacter(entityID)
		if !ok {
			currentCharacter, ok = r.lastKnownCharacters[entityID]
		}
		if r.sessionDirectory != nil {
			_, _ = r.sessionDirectory.Remove(entityID)
		}
		r.clearSessionCombatTargetLocked(entityID)
		r.clearMerchantWindowOpenLocked(entityID)
		r.clearSafeboxWindowOpenLocked(entityID)
		r.clearRefineWindowOpenLocked(entityID)
		r.clearMyShopWindowOpenLocked(entityID)
		r.clearStaticActorCombatEngagementsBySubjectLocked(entityID)
		r.clearExchangeLocked(entityID, true)
		r.detachProximityAggroSuppressSubjectLocked(entityID, currentCharacter.VID)
		_, _ = r.entities.Remove(entityID)
		delete(r.lastKnownCharacters, entityID)
		if !ok {
			continue
		}
		visibilityDiff := r.scopesLocked().LeaveVisibilityDiff(currentCharacter)
		removeRaw := encodeCharacterDeleteFrame(currentCharacter)
		for _, peerCharacter := range visibilityDiff.RemovedVisiblePeers {
			if characterAtBootstrapHPFloor(peerCharacter) {
				continue
			}
			r.enqueueToCharacterLocked(peerCharacter, [][]byte{removeRaw})
		}
		if r.removeOwnedGroundItemsLocked(entityID, r.visiblePeersForOwnedGroundItemsLocked(entityID, visibilityDiff.RemovedVisiblePeers)) {
			groundChanged = true
		}
	}
	return groundChanged
}

func (r *sharedWorldRegistry) Join(character loginticket.Character, pending *pendingServerFrames, relocate sharedWorldSessionRelocator) (uint64, []loginticket.Character) {
	if r == nil {
		return 0, nil
	}

	r.mu.Lock()
	groundChanged := false
	defer func() {
		hook := r.onGroundItemsChanged
		r.mu.Unlock()
		if groundChanged && hook != nil {
			hook()
		}
	}()

	staleIDs, liveConflict := r.reclaimableStaleDuplicateIDsLocked(character)
	if liveConflict {
		return 0, nil
	}
	groundChanged = r.removeStaleOwnershipLocked(staleIDs)

	visibilityDiff := r.scopesLocked().EnterVisibilityDiff(character)
	registered := r.entities.RegisterPlayer(character)
	if registered.Entity.ID == 0 {
		return 0, nil
	}
	id := registered.Entity.ID
	r.lastKnownCharacters[id] = character
	if !registerSharedWorldSessionEntry(r.sessionDirectory, id, pending, relocate) {
		delete(r.lastKnownCharacters, id)
		_, _ = r.entities.Remove(id)
		return 0, nil
	}
	r.claimPendingProximityAggroSuppressLocked(character.VID, id)
	r.rebindExclusiveGroundOwnerIDLocked(id, character)

	peerFrames := encodePeerVisibilityBootstrapFramesWithTemplates(character, r.itemTemplates)
	peerFrames = r.appendMyShopSignRematerializationLocked(peerFrames, character)
	for _, peerCharacter := range visibilityDiff.AddedVisiblePeers {
		if characterAtBootstrapHPFloor(peerCharacter) {
			continue
		}
		r.enqueueToCharacterLocked(peerCharacter, peerFrames)
	}
	return id, visibilityDiff.TargetVisiblePeers
}

// rebindExclusiveGroundOwnerIDLocked attaches process-local OwnerID to rematerialized
// exclusive ground handles whose durable owner identity matches the joining character.
// Public handles and already-bound OwnerID values are left untouched. Absolute timers
// and durable FileStore rows stay unchanged (OwnerID remains process-local only).
func (r *sharedWorldRegistry) rebindExclusiveGroundOwnerIDLocked(ownerID uint64, character loginticket.Character) {
	if r == nil || ownerID == 0 || len(r.groundItemsByVID) == 0 {
		return
	}
	for vid, ground := range r.groundItemsByVID {
		if !ground.OwnershipExclusive || ground.OwnerID != 0 {
			continue
		}
		if !sameGroundItemOwnerIdentity(ground, character) {
			continue
		}
		ground.OwnerID = ownerID
		r.groundItemsByVID[vid] = ground
	}
}

func (r *sharedWorldRegistry) Leave(id uint64) {
	if r == nil || id == 0 {
		return
	}

	r.mu.Lock()
	groundChanged := false
	currentCharacter, ok := r.playerCharacter(id)
	if !ok {
		currentCharacter, ok = r.lastKnownCharacters[id]
	}
	if r.sessionDirectory != nil {
		_, _ = r.sessionDirectory.Remove(id)
	}
	r.clearSessionCombatTargetLocked(id)
	r.clearMerchantWindowOpenLocked(id)
	r.clearSafeboxWindowOpenLocked(id)
	r.clearRefineWindowOpenLocked(id)
	r.clearMyShopWindowOpenLocked(id)
	r.clearStaticActorCombatEngagementsBySubjectLocked(id)
	r.clearExchangeLocked(id, true)
	r.detachProximityAggroSuppressSubjectLocked(id, currentCharacter.VID)
	_, _ = r.entities.Remove(id)
	delete(r.lastKnownCharacters, id)
	if ok {
		visibilityDiff := r.scopesLocked().LeaveVisibilityDiff(currentCharacter)
		removeRaw := encodeCharacterDeleteFrame(currentCharacter)
		for _, peerCharacter := range visibilityDiff.RemovedVisiblePeers {
			if characterAtBootstrapHPFloor(peerCharacter) {
				continue
			}
			r.enqueueToCharacterLocked(peerCharacter, [][]byte{removeRaw})
		}
		groundChanged = r.removeOwnedGroundItemsLocked(id, r.visiblePeersForOwnedGroundItemsLocked(id, visibilityDiff.RemovedVisiblePeers))
	}
	hook := r.onGroundItemsChanged
	r.mu.Unlock()
	if groundChanged && hook != nil {
		hook()
	}
}

func (r *sharedWorldRegistry) UpdateCharacter(id uint64, character loginticket.Character) {
	if r == nil || id == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	_ = r.entities.UpdatePlayer(id, character)
	r.lastKnownCharacters[id] = character
}

func (r *sharedWorldRegistry) visiblePeersForOwnedGroundItemsLocked(ownerID uint64, fallback []loginticket.Character) []loginticket.Character {
	if ownerID == 0 || len(r.groundItemsByVID) == 0 {
		return fallback
	}
	peersByVID := make(map[uint32]loginticket.Character)
	for _, peer := range fallback {
		peersByVID[peer.VID] = peer
	}
	for _, ground := range r.groundItemsByVID {
		if ground.OwnerID != ownerID {
			continue
		}
		groundCharacter := loginticket.Character{MapIndex: ground.MapIndex, X: ground.X, Y: ground.Y, Z: ground.Z}
		for _, candidate := range r.scopesLocked().VisibleTargets(0, groundCharacter) {
			if candidate.Character.VID == ground.OwnerVID {
				continue
			}
			peersByVID[candidate.Character.VID] = candidate.Character
		}
	}
	if len(peersByVID) == 0 {
		return nil
	}
	peers := make([]loginticket.Character, 0, len(peersByVID))
	for _, peer := range peersByVID {
		peers = append(peers, peer)
	}
	sort.Slice(peers, func(i int, j int) bool {
		if peers[i].Name == peers[j].Name {
			return peers[i].VID < peers[j].VID
		}
		return peers[i].Name < peers[j].Name
	})
	return peers
}

func (r *sharedWorldRegistry) removeOwnedGroundItemsLocked(ownerID uint64, visiblePeers []loginticket.Character) bool {
	if ownerID == 0 || len(r.groundItemsByVID) == 0 {
		return false
	}
	removed := make([]sharedGroundItem, 0)
	for vid, ground := range r.groundItemsByVID {
		if ground.OwnerID != ownerID {
			continue
		}
		removed = append(removed, ground)
		delete(r.groundItemsByVID, vid)
	}
	if len(removed) == 0 {
		return false
	}
	sortSharedGroundItemsByVID(removed)
	frames := make([][]byte, 0, len(removed))
	for _, ground := range removed {
		frames = append(frames, encodeGroundItemDeleteFrame(ground))
	}
	for _, peer := range visiblePeers {
		if characterAtBootstrapHPFloor(peer) {
			continue
		}
		r.enqueueToCharacterLocked(peer, frames)
	}
	return true
}

func (r *sharedWorldRegistry) CanRegisterGroundItem(ownerID uint64, ownerLogin string, character loginticket.Character, vid uint32, item inventory.ItemInstance) bool {
	const maxItemGetCountCarrier = uint16(^uint8(0))
	if item.ID == 0 || item.Vnum == 0 || item.Count == 0 || item.Count > maxItemGetCountCarrier || item.Locked || item.Equipped || item.EquipSlot != inventory.EquipmentSlotNone {
		return false
	}
	return r.canRegisterGroundItem(ownerID, ownerLogin, character, vid, item, 0)
}

func (r *sharedWorldRegistry) RegisterGroundItem(ownerID uint64, ownerLogin string, character loginticket.Character, vid uint32, item inventory.ItemInstance) bool {
	const maxItemGetCountCarrier = uint16(^uint8(0))
	if item.ID == 0 || item.Vnum == 0 || item.Count == 0 || item.Count > maxItemGetCountCarrier || item.Locked || item.Equipped || item.EquipSlot != inventory.EquipmentSlotNone {
		return false
	}
	return r.RegisterGroundItemWithPickupRange(ownerID, ownerLogin, character, vid, item, bootstrapGroundItemPickupRange)
}

func (r *sharedWorldRegistry) RegisterGroundItemWithPickupRange(ownerID uint64, ownerLogin string, character loginticket.Character, vid uint32, item inventory.ItemInstance, pickupRange int64) bool {
	const maxItemGetCountCarrier = uint16(^uint8(0))
	if item.ID == 0 || item.Vnum == 0 || item.Count == 0 || item.Count > maxItemGetCountCarrier || item.Locked || item.Equipped || item.EquipSlot != inventory.EquipmentSlotNone || pickupRange < 0 {
		return false
	}
	return r.registerGroundItem(ownerID, ownerLogin, character, vid, item, 0, pickupRange)
}

func (r *sharedWorldRegistry) CanRegisterGroundGold(ownerID uint64, ownerLogin string, character loginticket.Character, vid uint32, amount uint32) bool {
	return r.CanRegisterGroundGoldWithPickupRange(ownerID, ownerLogin, character, vid, amount, bootstrapGroundItemPickupRange)
}

func (r *sharedWorldRegistry) CanRegisterGroundGoldWithPickupRange(ownerID uint64, ownerLogin string, character loginticket.Character, vid uint32, amount uint32, pickupRange int64) bool {
	const maxPointChangeCarrier = uint32(1<<31 - 1)
	if amount == 0 || amount > maxPointChangeCarrier || pickupRange < 0 {
		return false
	}
	return r.canRegisterGroundItem(ownerID, ownerLogin, character, vid, inventory.ItemInstance{Vnum: 1, Count: 1}, amount)
}

func (r *sharedWorldRegistry) RegisterGroundGold(ownerID uint64, ownerLogin string, character loginticket.Character, vid uint32, amount uint32) bool {
	return r.RegisterGroundGoldWithPickupRange(ownerID, ownerLogin, character, vid, amount, bootstrapGroundItemPickupRange)
}

func (r *sharedWorldRegistry) RegisterGroundGoldWithPickupRange(ownerID uint64, ownerLogin string, character loginticket.Character, vid uint32, amount uint32, pickupRange int64) bool {
	const maxPointChangeCarrier = uint32(1<<31 - 1)
	if amount == 0 || amount > maxPointChangeCarrier || pickupRange < 0 {
		return false
	}
	return r.registerGroundItem(ownerID, ownerLogin, character, vid, inventory.ItemInstance{Vnum: 1, Count: 1}, amount, pickupRange)
}

func (r *sharedWorldRegistry) canRegisterGroundItem(ownerID uint64, ownerLogin string, character loginticket.Character, vid uint32, item inventory.ItemInstance, goldAmount uint32) bool {
	if r == nil || ownerID == 0 || !validRewardOwnerMetadata(ownerLogin) || !validRewardOwnerMetadata(character.Name) || vid == 0 || item.Vnum == 0 {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.canRegisterGroundItemLocked(ownerID, character, vid)
}

func (r *sharedWorldRegistry) canRegisterGroundItemLocked(ownerID uint64, character loginticket.Character, vid uint32) bool {
	registeredOwner, ok := r.playerCharacter(ownerID)
	if !ok || characterAtBootstrapHPFloor(registeredOwner) || characterAtBootstrapHPFloor(character) || !sameGroundRewardOwnerSnapshot(registeredOwner, character) {
		return false
	}
	if _, exists := r.groundItemsByVID[vid]; exists {
		return false
	}
	return true
}

func (r *sharedWorldRegistry) registerGroundItem(ownerID uint64, ownerLogin string, character loginticket.Character, vid uint32, item inventory.ItemInstance, goldAmount uint32, pickupRange int64) bool {
	if r == nil || ownerID == 0 || !validRewardOwnerMetadata(ownerLogin) || !validRewardOwnerMetadata(character.Name) || vid == 0 || item.Vnum == 0 {
		return false
	}

	r.mu.Lock()
	if !r.canRegisterGroundItemLocked(ownerID, character, vid) {
		r.mu.Unlock()
		return false
	}
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	ground := sharedGroundItem{
		VID:                vid,
		OwnerID:            ownerID,
		OwnerLogin:         ownerLogin,
		OwnerCharacterID:   character.ID,
		OwnerVID:           character.VID,
		OwnerName:          character.Name,
		OwnerHPPoint:       character.Points[bootstrapPlayerPointValueIndex],
		Item:               item,
		GoldAmount:         goldAmount,
		PickupRange:        pickupRange,
		MapIndex:           r.topology.EffectiveMapIndex(character),
		X:                  character.X,
		Y:                  character.Y,
		Z:                  character.Z,
		OwnershipExclusive: true,
		OwnershipExpiresAt: now.Add(bootstrapGroundItemOwnershipDuration),
		DespawnAt:          now.Add(bootstrapGroundItemDespawnDuration),
	}
	r.groundItemsByVID[vid] = ground
	frames := encodeGroundItemVisibleFrames(ground)
	for _, target := range r.scopesLocked().VisibleTargets(ownerID, character) {
		if characterAtBootstrapHPFloor(target.Character) {
			continue
		}
		r.enqueueToEntityLocked(target.Entity.ID, frames)
	}
	hook := r.onGroundItemsChanged
	r.mu.Unlock()
	if hook != nil {
		hook()
	}
	return true
}

func validRewardOwnerMetadata(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if r == 0 || unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func validRewardDropOwnerNameMetadata(value string) bool {
	if !validRewardOwnerMetadata(value) {
		return false
	}
	return len(value) <= itemproto.CharacterNameMaxLength+1
}

func sameGroundRewardOwnerLocation(registered loginticket.Character, supplied loginticket.Character) bool {
	return sameGroundRewardCharacterLocation(registered, supplied)
}

func sameGroundRewardOwnerSnapshot(registered loginticket.Character, supplied loginticket.Character) bool {
	return sameGroundRewardOwnerLocation(registered, supplied) && registered.Points[bootstrapPlayerPointValueIndex] == supplied.Points[bootstrapPlayerPointValueIndex]
}

func sameGroundRewardCollectorLocation(registered loginticket.Character, supplied loginticket.Character) bool {
	return sameGroundRewardCharacterLocation(registered, supplied)
}

func sameGroundRewardCollectorSnapshot(registered loginticket.Character, supplied loginticket.Character) bool {
	return sameGroundRewardCollectorLocation(registered, supplied) && registered.Points[bootstrapPlayerPointValueIndex] == supplied.Points[bootstrapPlayerPointValueIndex]
}

func sameGroundRewardCharacterLocation(registered loginticket.Character, supplied loginticket.Character) bool {
	return registered.ID == supplied.ID && registered.VID == supplied.VID && registered.Name == supplied.Name && registered.MapIndex == supplied.MapIndex && registered.X == supplied.X && registered.Y == supplied.Y && registered.Z == supplied.Z
}

func (r *sharedWorldRegistry) groundItemVisibleToCharacterLocked(ground sharedGroundItem, character loginticket.Character) bool {
	return r.topology.SharesVisibleWorld(character, loginticket.Character{MapIndex: ground.MapIndex, X: ground.X, Y: ground.Y})
}

func (r *sharedWorldRegistry) groundItemReachableByCharacterLocked(ground sharedGroundItem, character loginticket.Character) bool {
	pickupRange := ground.PickupRange
	if pickupRange == 0 {
		pickupRange = bootstrapGroundItemPickupRange
	}
	return r.groundItemReachableByCharacterWithRangeLocked(ground, character, pickupRange)
}

func (r *sharedWorldRegistry) groundItemReachableByCharacterWithRangeLocked(ground sharedGroundItem, character loginticket.Character, pickupRange int64) bool {
	if !r.groundItemVisibleToCharacterLocked(ground, character) {
		return false
	}
	if pickupRange < 0 {
		return false
	}
	dx := int64(character.X) - int64(ground.X)
	dy := int64(character.Y) - int64(ground.Y)
	return dx*dx+dy*dy <= pickupRange*pickupRange
}

func (r *sharedWorldRegistry) groundItemVisibilityDiffLocked(previous loginticket.Character, current loginticket.Character, groundItems ...[]worldruntime.GroundItemOccupancy) sharedGroundItemVisibilityDiff {
	if r == nil || len(r.groundItemsByVID) == 0 {
		return sharedGroundItemVisibilityDiff{}
	}
	groundItemOccupancies := r.groundItemOccupanciesLocked()
	if len(groundItems) > 0 {
		groundItemOccupancies = groundItems[0]
	}
	groundItemsByVID := make(map[uint32]sharedGroundItem, len(r.groundItemsByVID))
	for _, ground := range r.groundItemsByVID {
		groundItemsByVID[ground.VID] = ground
	}
	visibilityDiff := r.scopesLocked().RelocateGroundItemVisibilityDiff(previous, current, groundItemOccupancies)
	diff := sharedGroundItemVisibilityDiff{
		Removed: sharedGroundItemsFromSnapshots(visibilityDiff.RemovedVisibleItems, groundItemsByVID),
		Added:   sharedGroundItemsFromSnapshots(visibilityDiff.AddedVisibleItems, groundItemsByVID),
	}
	sortSharedGroundItemsByVID(diff.Removed)
	sortSharedGroundItemsByVID(diff.Added)
	return diff
}

func (r *sharedWorldRegistry) groundItemOccupanciesLocked() []worldruntime.GroundItemOccupancy {
	if r == nil || len(r.groundItemsByVID) == 0 {
		return nil
	}
	groundItems := make([]worldruntime.GroundItemOccupancy, 0, len(r.groundItemsByVID))
	for _, ground := range r.groundItemsByVID {
		groundItems = append(groundItems, sharedGroundItemOccupancy(ground))
	}
	return groundItems
}

func sharedGroundItemsFromSnapshots(snapshots []worldruntime.GroundItemSnapshot, groundItemsByVID map[uint32]sharedGroundItem) []sharedGroundItem {
	items := make([]sharedGroundItem, 0, len(snapshots))
	for _, snapshot := range snapshots {
		ground, ok := groundItemsByVID[snapshot.VID]
		if !ok {
			continue
		}
		items = append(items, ground)
	}
	return items
}

func sortSharedGroundItemsByVID(items []sharedGroundItem) {
	sort.Slice(items, func(i int, j int) bool {
		return items[i].VID < items[j].VID
	})
}

func encodeGroundItemAddFrame(ground sharedGroundItem) []byte {
	return itemproto.EncodeGroundAdd(itemproto.GroundAddPacket{VID: ground.VID, Vnum: ground.Item.Vnum, X: ground.X, Y: ground.Y, Z: ground.Z})
}

func groundItemSnapshot(ground sharedGroundItem) GroundItemSnapshot {
	return sharedGroundItemOccupancy(ground)
}

func sharedGroundItemOccupancy(ground sharedGroundItem) worldruntime.GroundItemOccupancy {
	count := ground.Item.Count
	if ground.GoldAmount != 0 {
		count = 0
	}
	return worldruntime.GroundItemOccupancy{
		VID:              ground.VID,
		Vnum:             ground.Item.Vnum,
		Count:            count,
		OwnerName:        ground.OwnerName,
		OwnerLogin:       ground.OwnerLogin,
		OwnerCharacterID: ground.OwnerCharacterID,
		OwnerVID:         ground.OwnerVID,
		GoldAmount:       ground.GoldAmount,
		PickupRange:      ground.PickupRange,
		MapIndex:         ground.MapIndex,
		X:                ground.X,
		Y:                ground.Y,
		Z:                ground.Z,
	}
}

func encodeGroundItemOwnershipFrame(ground sharedGroundItem) []byte {
	ownerName := ground.OwnerName
	if !ground.OwnershipExclusive {
		ownerName = ""
	}
	return itemproto.EncodeOwnership(itemproto.OwnershipPacket{VID: ground.VID, OwnerName: ownerName})
}

func encodeGroundItemVisibleFrames(ground sharedGroundItem) [][]byte {
	return [][]byte{encodeGroundItemAddFrame(ground), encodeGroundItemOwnershipFrame(ground)}
}

func encodeGroundItemDeleteFrame(ground sharedGroundItem) []byte {
	return itemproto.EncodeGroundDel(itemproto.GroundDelPacket{VID: ground.VID})
}

func encodeGroundItemPublicOwnershipFrame(ground sharedGroundItem) []byte {
	return itemproto.EncodeOwnership(itemproto.OwnershipPacket{VID: ground.VID, OwnerName: ""})
}

func (r *sharedWorldRegistry) currentTimeLocked() time.Time {
	if r != nil && r.now != nil {
		return r.now()
	}
	return time.Now()
}

func (r *sharedWorldRegistry) FlushDueGroundItemOwnershipReleases() {
	if r == nil {
		return
	}
	r.mu.Lock()
	changed := r.flushDueGroundItemOwnershipReleasesLocked()
	changed = r.flushDueGroundItemDespawnsLocked() || changed
	hook := r.onGroundItemsChanged
	r.mu.Unlock()
	if changed && hook != nil {
		hook()
	}
}

func (r *sharedWorldRegistry) flushDueGroundItemOwnershipReleasesLocked() bool {
	if r == nil || len(r.groundItemsByVID) == 0 {
		return false
	}
	now := r.currentTimeLocked()
	released := make([]sharedGroundItem, 0)
	for vid, ground := range r.groundItemsByVID {
		if !ground.OwnershipExclusive || ground.OwnershipExpiresAt.IsZero() || now.Before(ground.OwnershipExpiresAt) {
			continue
		}
		ground.OwnershipExclusive = false
		ground.OwnershipExpiresAt = time.Time{}
		r.groundItemsByVID[vid] = ground
		released = append(released, ground)
	}
	sortSharedGroundItemsByVID(released)
	for _, ground := range released {
		frames := [][]byte{encodeGroundItemPublicOwnershipFrame(ground)}
		groundCharacter := loginticket.Character{MapIndex: ground.MapIndex, X: ground.X, Y: ground.Y, Z: ground.Z}
		for _, target := range r.scopesLocked().VisibleTargets(0, groundCharacter) {
			if characterAtBootstrapHPFloor(target.Character) {
				continue
			}
			r.enqueueToEntityLocked(target.Entity.ID, frames)
		}
	}
	return len(released) > 0
}

func (r *sharedWorldRegistry) flushDueGroundItemDespawnsLocked() bool {
	if r == nil || len(r.groundItemsByVID) == 0 {
		return false
	}
	now := r.currentTimeLocked()
	despawned := make([]sharedGroundItem, 0)
	for vid, ground := range r.groundItemsByVID {
		if ground.DespawnAt.IsZero() || now.Before(ground.DespawnAt) {
			continue
		}
		delete(r.groundItemsByVID, vid)
		despawned = append(despawned, ground)
	}
	sortSharedGroundItemsByVID(despawned)
	for _, ground := range despawned {
		frames := [][]byte{encodeGroundItemDeleteFrame(ground)}
		groundCharacter := loginticket.Character{MapIndex: ground.MapIndex, X: ground.X, Y: ground.Y, Z: ground.Z}
		for _, target := range r.scopesLocked().VisibleTargets(0, groundCharacter) {
			if characterAtBootstrapHPFloor(target.Character) {
				continue
			}
			r.enqueueToEntityLocked(target.Entity.ID, frames)
		}
	}
	return len(despawned) > 0
}

func groundItemExclusiveOwnershipBlocksCollector(ground sharedGroundItem, collectorID uint64, collector loginticket.Character) bool {
	if !ground.OwnershipExclusive {
		return false
	}
	if ground.OwnerID != 0 {
		return ground.OwnerID != collectorID
	}
	// Rematerialized exclusive handles have no process-local OwnerID until the
	// original owner rejoins; keep peers blocked via durable owner identity.
	return !sameGroundItemOwnerIdentity(ground, collector)
}

func sameGroundItemOwnerIdentity(ground sharedGroundItem, character loginticket.Character) bool {
	return character.ID == ground.OwnerCharacterID && character.VID == ground.OwnerVID && character.Name == ground.OwnerName
}

func buildGroundItemVisibilityTransitionFrames(removed []sharedGroundItem, added []sharedGroundItem) [][]byte {
	frames := make([][]byte, 0, len(removed)+(len(added)*2))
	for _, ground := range removed {
		frames = append(frames, encodeGroundItemDeleteFrame(ground))
	}
	for _, ground := range added {
		frames = append(frames, encodeGroundItemVisibleFrames(ground)...)
	}
	return frames
}

func (r *sharedWorldRegistry) GroundItemExists(vid uint32) bool {
	if r == nil || vid == 0 {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.groundItemsByVID[vid]
	return ok
}

func (r *sharedWorldRegistry) GroundItemVisibleTo(collectorID uint64, collector loginticket.Character, vid uint32) (inventory.ItemInstance, bool) {
	if r == nil || collectorID == 0 || vid == 0 {
		return inventory.ItemInstance{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	registeredCollector, ok := r.playerCharacter(collectorID)
	if !ok || characterAtBootstrapHPFloor(registeredCollector) || characterAtBootstrapHPFloor(collector) || !sameGroundRewardCollectorLocation(registeredCollector, collector) {
		return inventory.ItemInstance{}, false
	}
	ground, ok := r.groundItemsByVID[vid]
	if !ok || !r.groundItemVisibleToCharacterLocked(ground, registeredCollector) {
		return inventory.ItemInstance{}, false
	}
	return ground.Item, true
}

func (r *sharedWorldRegistry) GroundItemPickupFor(collectorID uint64, collector loginticket.Character, vid uint32) (sharedGroundItemPickup, bool) {
	return r.groundItemPickupFor(collectorID, collector, vid, 0, false)
}

func (r *sharedWorldRegistry) GroundItemPickupForWithRange(collectorID uint64, collector loginticket.Character, vid uint32, pickupRange int64) (sharedGroundItemPickup, bool) {
	return r.groundItemPickupFor(collectorID, collector, vid, pickupRange, true)
}

func (r *sharedWorldRegistry) groundItemPickupFor(collectorID uint64, collector loginticket.Character, vid uint32, pickupRange int64, explicitRange bool) (sharedGroundItemPickup, bool) {
	if r == nil || collectorID == 0 || vid == 0 {
		return sharedGroundItemPickup{}, false
	}
	if explicitRange && pickupRange < 0 {
		return sharedGroundItemPickup{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.flushDueGroundItemOwnershipReleasesLocked()

	registeredCollector, ok := r.playerCharacter(collectorID)
	if !ok || characterAtBootstrapHPFloor(registeredCollector) || characterAtBootstrapHPFloor(collector) || !sameGroundRewardCollectorSnapshot(registeredCollector, collector) {
		return sharedGroundItemPickup{}, false
	}
	ground, ok := r.groundItemsByVID[vid]
	if !ok {
		return sharedGroundItemPickup{}, false
	}
	if groundItemExclusiveOwnershipBlocksCollector(ground, collectorID, registeredCollector) {
		return sharedGroundItemPickup{}, false
	}
	if explicitRange {
		if !r.groundItemReachableByCharacterWithRangeLocked(ground, registeredCollector, pickupRange) {
			return sharedGroundItemPickup{}, false
		}
	} else if !r.groundItemReachableByCharacterLocked(ground, registeredCollector) {
		return sharedGroundItemPickup{}, false
	}
	ownerName := ground.OwnerName
	var ownerCharacter loginticket.Character
	if ground.OwnershipExclusive && ground.OwnerID != 0 && ground.OwnerID != collectorID {
		owner, ok := r.entities.Player(ground.OwnerID)
		if ok && !characterAtBootstrapHPFloor(owner.Character) && groundItemOwnerStillMatches(ground, owner.Character) && r.topology.SharesVisibleWorld(collector, owner.Character) {
			ownerCharacter = owner.Character
			if ownerName == "" {
				ownerName = owner.Character.Name
			}
		}
	}
	return sharedGroundItemPickup{Item: ground.Item, GoldAmount: ground.GoldAmount, OwnerID: ground.OwnerID, OwnerLogin: ground.OwnerLogin, OwnerName: ownerName, Owner: ownerCharacter}, true
}

func groundItemOwnerStillMatches(ground sharedGroundItem, owner loginticket.Character) bool {
	return owner.ID == ground.OwnerCharacterID && owner.VID == ground.OwnerVID && owner.Name == ground.OwnerName && owner.Points[bootstrapPlayerPointValueIndex] == ground.OwnerHPPoint && owner.MapIndex == ground.MapIndex && owner.X == ground.X && owner.Y == ground.Y && owner.Z == ground.Z
}

func (r *sharedWorldRegistry) RemoveGroundItem(collectorID uint64, collector loginticket.Character, vid uint32) bool {
	if r == nil || collectorID == 0 || vid == 0 {
		return false
	}

	r.mu.Lock()
	registeredCollector, ok := r.playerCharacter(collectorID)
	if !ok || characterAtBootstrapHPFloor(registeredCollector) || characterAtBootstrapHPFloor(collector) || !sameGroundRewardCollectorSnapshot(registeredCollector, collector) {
		r.mu.Unlock()
		return false
	}
	ground, ok := r.groundItemsByVID[vid]
	if !ok || !r.groundItemReachableByCharacterLocked(ground, registeredCollector) {
		r.mu.Unlock()
		return false
	}
	delete(r.groundItemsByVID, vid)
	frames := [][]byte{itemproto.EncodeGroundDel(itemproto.GroundDelPacket{VID: vid})}
	for _, target := range r.scopesLocked().VisibleTargets(collectorID, collector) {
		if characterAtBootstrapHPFloor(target.Character) {
			continue
		}
		r.enqueueToEntityLocked(target.Entity.ID, frames)
	}
	hook := r.onGroundItemsChanged
	r.mu.Unlock()
	if hook != nil {
		hook()
	}
	return true
}

// RestorePersistedGroundItems inserts durable pending ground handles without
// requiring a live owner session. It is the offline rematerialize path used at
// gamed startup; live drops continue to use RegisterGroundItem*.
func (r *sharedWorldRegistry) RestorePersistedGroundItems(records []worldruntime.DurableGroundItemRecord) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, record := range records {
		if err := r.restorePersistedGroundItemLocked(record); err != nil {
			return err
		}
	}
	return nil
}

// ClearPersistedGroundItems drops all pending ground handles. Used by operator
// restore before rematerializing a manifested backup into the live shared world.
func (r *sharedWorldRegistry) ClearPersistedGroundItems() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.groundItemsByVID = make(map[uint32]sharedGroundItem)
}

func (r *sharedWorldRegistry) restorePersistedGroundItemLocked(record worldruntime.DurableGroundItemRecord) error {
	record = worldruntime.NormalizeDurableGroundItemSnapshot(worldruntime.DurableGroundItemSnapshot{GroundItems: []worldruntime.DurableGroundItemRecord{record}}).GroundItems[0]
	if err := worldruntime.ValidateDurableGroundItemSnapshot(worldruntime.DurableGroundItemSnapshot{GroundItems: []worldruntime.DurableGroundItemRecord{record}}); err != nil {
		return err
	}
	if _, exists := r.groundItemsByVID[record.VID]; exists {
		return fmt.Errorf("%w: ground vid %d already exists", worldruntime.ErrInvalidGroundItemSnapshot, record.VID)
	}

	item := inventory.ItemInstance{}
	var goldAmount uint32
	if record.GoldAmount != nil {
		goldAmount = *record.GoldAmount
		item = inventory.ItemInstance{Vnum: 1, Count: 1}
	} else {
		item = inventory.ItemInstance{ID: record.ItemID, Vnum: record.Vnum, Count: *record.ItemCount}
	}

	ground := sharedGroundItem{
		VID:                record.VID,
		OwnerID:            0,
		OwnerLogin:         record.OwnerLogin,
		OwnerCharacterID:   record.OwnerCharacterID,
		OwnerVID:           record.OwnerVID,
		OwnerName:          record.OwnerName,
		Item:               item,
		GoldAmount:         goldAmount,
		PickupRange:        record.PickupRange,
		MapIndex:           record.MapIndex,
		X:                  record.X,
		Y:                  record.Y,
		Z:                  record.Z,
		OwnershipExclusive: record.OwnershipExclusive,
		DespawnAt:          record.DespawnAt.UTC(),
	}
	if record.OwnershipExclusive && record.OwnershipExpiresAt != nil {
		ground.OwnershipExpiresAt = record.OwnershipExpiresAt.UTC()
	}
	r.groundItemsByVID[record.VID] = ground
	return nil
}

func (r *sharedWorldRegistry) DurableGroundItemSnapshot() worldruntime.DurableGroundItemSnapshot {
	if r == nil {
		return worldruntime.DurableGroundItemSnapshot{GroundItems: []worldruntime.DurableGroundItemRecord{}}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.durableGroundItemSnapshotLocked()
}

func (r *sharedWorldRegistry) durableGroundItemSnapshotLocked() worldruntime.DurableGroundItemSnapshot {
	records := make([]worldruntime.DurableGroundItemRecord, 0, len(r.groundItemsByVID))
	for _, ground := range r.groundItemsByVID {
		records = append(records, durableGroundItemRecordFromShared(ground))
	}
	return worldruntime.NormalizeDurableGroundItemSnapshot(worldruntime.DurableGroundItemSnapshot{GroundItems: records})
}

func durableGroundItemRecordFromShared(ground sharedGroundItem) worldruntime.DurableGroundItemRecord {
	record := worldruntime.DurableGroundItemRecord{
		VID:                ground.VID,
		Vnum:               ground.Item.Vnum,
		OwnerLogin:         ground.OwnerLogin,
		OwnerCharacterID:   ground.OwnerCharacterID,
		OwnerVID:           ground.OwnerVID,
		OwnerName:          ground.OwnerName,
		MapIndex:           ground.MapIndex,
		X:                  ground.X,
		Y:                  ground.Y,
		Z:                  ground.Z,
		PickupRange:        ground.PickupRange,
		OwnershipExclusive: ground.OwnershipExclusive,
		DespawnAt:          ground.DespawnAt.UTC(),
	}
	if ground.GoldAmount != 0 {
		gold := ground.GoldAmount
		record.GoldAmount = &gold
		record.Vnum = 1
	} else {
		count := ground.Item.Count
		record.ItemCount = &count
		record.ItemID = ground.Item.ID
	}
	if ground.OwnershipExclusive && !ground.OwnershipExpiresAt.IsZero() {
		expires := ground.OwnershipExpiresAt.UTC()
		record.OwnershipExpiresAt = &expires
	}
	return record
}

func (r *sharedWorldRegistry) SetGroundItemsChangedHook(hook func()) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onGroundItemsChanged = hook
}

func (r *sharedWorldRegistry) UpdateCharacterWithVisibilityTransition(id uint64, previous loginticket.Character, current loginticket.Character, stableFrames [][]byte) {
	if r == nil || id == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	scopes := r.scopesLocked()
	visibilityDiff := scopes.RelocateVisibilityDiff(previous, current)
	staticActorVisibilityDiff := scopes.RelocateStaticActorVisibilityDiff(previous, current)
	groundItemVisibilityDiff := r.groundItemVisibilityDiffLocked(previous, current)
	originAddedVisiblePeers := visibilityDiff.AddedVisiblePeers
	originAddedVisibleStaticActors := staticActorVisibilityDiff.AddedVisibleActors
	originAddedGroundItems := groundItemVisibilityDiff.Added
	if characterAtBootstrapHPFloor(current) {
		originAddedVisiblePeers = nil
		originAddedVisibleStaticActors = nil
		originAddedGroundItems = nil
	}
	_ = r.entities.UpdatePlayer(id, current)
	r.lastKnownCharacters[id] = current
	r.closeExchangeIfOutOfRangeLocked(id)
	var combatTargetClearFrames [][]byte
	if len(stableFrames) == 0 {
		combatTargetClearFrames = r.clearInvalidSessionCombatTargetLocked(id)
	}

	removedRaw := encodeCharacterDeleteFrame(previous)
	addedRaw := encodePeerVisibilityBootstrapFramesWithTemplates(current, r.itemTemplates)
	addedRaw = r.appendMyShopSignRematerializationLocked(addedRaw, current)
	stablePeerVIDs := make(map[uint32]struct{}, len(visibilityDiff.AddedVisiblePeers))
	for _, peerCharacter := range visibilityDiff.AddedVisiblePeers {
		stablePeerVIDs[peerCharacter.VID] = struct{}{}
	}
	for _, peerCharacter := range visibilityDiff.RemovedVisiblePeers {
		if characterAtBootstrapHPFloor(peerCharacter) {
			continue
		}
		r.enqueueToCharacterLocked(peerCharacter, [][]byte{removedRaw})
	}
	for _, peerCharacter := range visibilityDiff.TargetVisiblePeers {
		if _, added := stablePeerVIDs[peerCharacter.VID]; added {
			continue
		}
		if characterAtBootstrapHPFloor(peerCharacter) {
			continue
		}
		if len(stableFrames) > 0 {
			r.enqueueToCharacterLocked(peerCharacter, stableFrames)
		}
	}
	for _, peerCharacter := range visibilityDiff.AddedVisiblePeers {
		if characterAtBootstrapHPFloor(peerCharacter) {
			continue
		}
		r.enqueueToCharacterLocked(peerCharacter, addedRaw)
	}

	originFrames := append([][]byte(nil), combatTargetClearFrames...)
	originFrames = append(originFrames, buildTransferOriginFramesWithTemplates(visibilityDiff.RemovedVisiblePeers, originAddedVisiblePeers, r.itemTemplates)...)
	for _, peerCharacter := range originAddedVisiblePeers {
		originFrames = r.appendMyShopSignRematerializationLocked(originFrames, peerCharacter)
	}
	originFrames = append(originFrames, r.buildStaticActorVisibilityTransitionFramesLocked(staticActorVisibilityDiff.RemovedVisibleActors, originAddedVisibleStaticActors)...)
	originFrames = append(originFrames, buildGroundItemVisibilityTransitionFrames(groundItemVisibilityDiff.Removed, originAddedGroundItems)...)
	if len(originFrames) == 0 {
		return
	}
	originEntry, ok := r.sessionEntryLocked(id)
	if !ok || originEntry.FrameSink == nil {
		return
	}
	originEntry.FrameSink.Enqueue(originFrames)
}

func (r *sharedWorldRegistry) Relocate(id uint64, character loginticket.Character) bool {
	_, ok := r.Transfer(id, character)
	return ok
}

func (r *sharedWorldRegistry) RelocateCharacter(name string, mapIndex uint32, x int32, y int32) bool {
	_, ok := r.TransferCharacter(name, mapIndex, x, y)
	return ok
}

func (r *sharedWorldRegistry) TransferCharacter(name string, mapIndex uint32, x int32, y int32) (RelocationPreview, bool) {
	if r == nil || name == "" || mapIndex == 0 {
		return RelocationPreview{}, false
	}

	r.mu.Lock()
	playerEntity, ok := r.playerEntityByName(name)
	if !ok {
		r.mu.Unlock()
		return RelocationPreview{}, false
	}
	entry, ok := r.sessionEntryLocked(playerEntity.Entity.ID)
	if !ok {
		r.mu.Unlock()
		return RelocationPreview{}, false
	}
	r.mu.Unlock()

	return invokeSessionRelocator(entry, mapIndex, x, y)
}

func (r *sharedWorldRegistry) Transfer(id uint64, character loginticket.Character) (RelocationPreview, bool) {
	preview, _, ok := r.transfer(id, character, true)
	return preview, ok
}

func (r *sharedWorldRegistry) TransferWithOriginFrames(id uint64, character loginticket.Character) (RelocationPreview, [][]byte, bool) {
	return r.transfer(id, character, false)
}

func (r *sharedWorldRegistry) transfer(id uint64, character loginticket.Character, enqueueOrigin bool) (RelocationPreview, [][]byte, bool) {
	if r == nil || id == 0 {
		return RelocationPreview{}, nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	previous, ok := r.playerCharacter(id)
	if !ok {
		return RelocationPreview{}, nil, false
	}
	var combatTargetClearFrames [][]byte
	if targetVID, selected := r.sessionCombatTargetLocked(id); selected {
		r.clearSessionCombatTargetLocked(id)
		if clearRaw := combatproto.EncodeServerClearTarget(); targetVID != 0 && clearRaw != nil {
			combatTargetClearFrames = [][]byte{clearRaw}
		}
	}
	r.clearStaticActorCombatEngagementsBySubjectLocked(id)
	scopes := r.scopesLocked()
	visibilityDiff := scopes.RelocateVisibilityDiff(previous, character)
	staticActorVisibilityDiff := scopes.RelocateStaticActorVisibilityDiff(previous, character)
	groundItemOccupancies := r.groundItemOccupanciesLocked()
	result := r.markRelocationPreviewStateLocked(scopes.BuildRelocationPreviewWithGroundItems(previous, character, true, groundItemOccupancies))

	groundItemVisibilityDiff := r.groundItemVisibilityDiffLocked(previous, character, groundItemOccupancies)
	originAddedVisiblePeers := visibilityDiff.AddedVisiblePeers
	originAddedVisibleStaticActors := staticActorVisibilityDiff.AddedVisibleActors
	originAddedGroundItems := groundItemVisibilityDiff.Added
	if characterAtBootstrapHPFloor(character) {
		originAddedVisiblePeers = nil
		originAddedVisibleStaticActors = nil
		originAddedGroundItems = nil
	}
	originFrames := append([][]byte(nil), combatTargetClearFrames...)
	originFrames = append(originFrames, buildTransferOriginFramesWithTemplates(visibilityDiff.RemovedVisiblePeers, originAddedVisiblePeers, r.itemTemplates)...)
	for _, peerCharacter := range originAddedVisiblePeers {
		originFrames = r.appendMyShopSignRematerializationLocked(originFrames, peerCharacter)
	}
	originFrames = append(originFrames, r.buildStaticActorVisibilityTransitionFramesLocked(staticActorVisibilityDiff.RemovedVisibleActors, originAddedVisibleStaticActors)...)
	originFrames = append(originFrames, buildGroundItemVisibilityTransitionFrames(groundItemVisibilityDiff.Removed, originAddedGroundItems)...)
	originEntry, _ := r.sessionEntryLocked(id)
	if enqueueOrigin && originEntry.FrameSink != nil && len(originFrames) > 0 {
		originEntry.FrameSink.Enqueue(originFrames)
	}

	_ = r.entities.UpdatePlayer(id, character)
	r.lastKnownCharacters[id] = character
	r.closeExchangeIfOutOfRangeLocked(id)

	movedDelete := encodeCharacterDeleteFrame(previous)
	movedFrames := encodePeerVisibilityBootstrapFramesWithTemplates(character, r.itemTemplates)
	movedFrames = r.appendMyShopSignRematerializationLocked(movedFrames, character)
	for _, peerCharacter := range visibilityDiff.RemovedVisiblePeers {
		if characterAtBootstrapHPFloor(peerCharacter) {
			continue
		}
		r.enqueueToCharacterLocked(peerCharacter, [][]byte{movedDelete})
	}
	for _, peerCharacter := range visibilityDiff.AddedVisiblePeers {
		if characterAtBootstrapHPFloor(peerCharacter) {
			continue
		}
		r.enqueueToCharacterLocked(peerCharacter, movedFrames)
	}

	return result, originFrames, true
}

func (r *sharedWorldRegistry) ConnectedCharacters() []ConnectedCharacterSnapshot {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.scopesLocked().ConnectedCharacterSnapshots()
}

func (r *sharedWorldRegistry) ConnectedCharacterSnapshot(name string) (ConnectedCharacterSnapshot, bool) {
	if r == nil || name == "" {
		return ConnectedCharacterSnapshot{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.scopesLocked().ConnectedCharacterSnapshotByExactName(name)
}

func (r *sharedWorldRegistry) CharacterVisibility() []CharacterVisibilitySnapshot {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.markCharacterVisibilityStaticActorStateLocked(r.scopesLocked().CharacterVisibilitySnapshotsWithGroundItems(r.groundItemOccupanciesLocked()))
}

func (r *sharedWorldRegistry) CharacterVisibilitySnapshot(name string) (CharacterVisibilitySnapshot, bool) {
	if r == nil || name == "" {
		return CharacterVisibilitySnapshot{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, snapshot := range r.markCharacterVisibilityStaticActorStateLocked(r.scopesLocked().CharacterVisibilitySnapshotsWithGroundItems(r.groundItemOccupanciesLocked())) {
		if snapshot.Name == name {
			return snapshot, true
		}
	}
	return CharacterVisibilitySnapshot{}, false
}

func (r *sharedWorldRegistry) InteractionVisibility() []worldruntime.CharacterInteractionVisibilitySnapshot {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.markCharacterInteractionVisibilityStaticActorStateLocked(r.scopesLocked().CharacterInteractionVisibilitySnapshots())
}

func (r *sharedWorldRegistry) InteractionVisibilitySnapshot(name string) (worldruntime.CharacterInteractionVisibilitySnapshot, bool) {
	if r == nil || name == "" {
		return worldruntime.CharacterInteractionVisibilitySnapshot{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, snapshot := range r.markCharacterInteractionVisibilityStaticActorStateLocked(r.scopesLocked().CharacterInteractionVisibilitySnapshots()) {
		if snapshot.Name == name {
			return snapshot, true
		}
	}
	return worldruntime.CharacterInteractionVisibilitySnapshot{}, false
}

func (r *sharedWorldRegistry) MapOccupancy() []MapOccupancySnapshot {
	if r == nil {
		return nil
	}
	return r.mapOccupancySnapshots()
}

func (r *sharedWorldRegistry) MapOccupancySnapshot(mapIndex uint32) (MapOccupancySnapshot, bool) {
	if r == nil || mapIndex == 0 {
		return MapOccupancySnapshot{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, snapshot := range r.mapOccupancySnapshotsLocked() {
		if snapshot.MapIndex == mapIndex {
			return snapshot, true
		}
	}
	return MapOccupancySnapshot{}, false
}

func (r *sharedWorldRegistry) NextStaticActorEntityID() uint64 {
	if r == nil || r.entities == nil {
		return 0
	}
	return r.entities.NextEntityID()
}

func (r *sharedWorldRegistry) RegisterStaticActor(name string, mapIndex uint32, x int32, y int32, raceNum uint32) (StaticActorSnapshot, bool) {
	return r.registerStaticActor(0, name, mapIndex, x, y, raceNum, "", "", "", "", worldruntime.StaticActorDeathReward{})
}

func (r *sharedWorldRegistry) RegisterStaticActorWithInteraction(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string) (StaticActorSnapshot, bool) {
	return r.registerStaticActor(entityID, name, mapIndex, x, y, raceNum, interactionKind, interactionRef, "", "", worldruntime.StaticActorDeathReward{})
}

func (r *sharedWorldRegistry) RegisterStaticActorWithCombatKind(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32, combatKind string) (StaticActorSnapshot, bool) {
	return r.registerStaticActor(entityID, name, mapIndex, x, y, raceNum, "", "", combatKind, "", worldruntime.StaticActorDeathReward{})
}

func (r *sharedWorldRegistry) registerStaticActor(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string, combatKind string, spawnGroupRef string, deathReward worldruntime.StaticActorDeathReward) (StaticActorSnapshot, bool) {
	return r.registerStaticActorWithSpawnHome(entityID, name, mapIndex, x, y, raceNum, interactionKind, interactionRef, combatKind, spawnGroupRef, nil, deathReward)
}

func (r *sharedWorldRegistry) registerStaticActorWithSpawnHome(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string, combatKind string, spawnGroupRef string, spawnHome *worldruntime.PositionSnapshot, deathReward worldruntime.StaticActorDeathReward) (StaticActorSnapshot, bool) {
	return r.registerStaticActorWithSpawnHomeAndKillQuestCredit(entityID, name, mapIndex, x, y, raceNum, interactionKind, interactionRef, combatKind, spawnGroupRef, spawnHome, deathReward, staticActorKillQuestCredit{})
}

func (r *sharedWorldRegistry) registerStaticActorWithSpawnHomeAndKillQuestCredit(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string, combatKind string, spawnGroupRef string, spawnHome *worldruntime.PositionSnapshot, deathReward worldruntime.StaticActorDeathReward, killQuestCredit staticActorKillQuestCredit) (StaticActorSnapshot, bool) {
	spawnGroupRef = strings.TrimSpace(spawnGroupRef)
	if r == nil || r.entities == nil || !validStaticActorRuntimeName(name) || mapIndex == 0 || raceNum == 0 || !worldruntime.ValidStaticActorInteractionMetadata(interactionKind, interactionRef) || !worldruntime.ValidStaticActorCombatKind(combatKind) || !worldruntime.ValidStaticActorSpawnGroupRef(spawnGroupRef) || !worldruntime.ValidStaticActorDeathReward(deathReward) || !validStaticActorKillQuestCredit(killQuestCredit) {
		return StaticActorSnapshot{}, false
	}
	if spawnGroupRef != "" && (combatKind == "" || interactionKind != "" || interactionRef != "") {
		return StaticActorSnapshot{}, false
	}
	if spawnGroupRef == "" && !killQuestCredit.Empty() {
		return StaticActorSnapshot{}, false
	}
	position := worldruntime.NewPosition(mapIndex, x, y)
	if !position.Valid() {
		return StaticActorSnapshot{}, false
	}
	var authoredHome worldruntime.Position
	if spawnHome != nil {
		authoredHome = worldruntime.NewPosition(spawnHome.MapIndex, spawnHome.X, spawnHome.Y)
		if !authoredHome.Valid() || spawnGroupRef == "" {
			return StaticActorSnapshot{}, false
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var (
		registered worldruntime.StaticEntity
		ok         bool
	)
	candidate := worldruntime.StaticEntity{
		Entity:          worldruntime.Entity{ID: entityID, Name: name},
		Position:        position,
		SpawnHome:       authoredHome,
		RaceNum:         raceNum,
		InteractionKind: interactionKind,
		InteractionRef:  interactionRef,
		SpawnGroupRef:   spawnGroupRef,
		CombatKind:      combatKind,
		DeathReward:     deathReward.Clone(),
	}
	if entityID == 0 {
		registered, ok = r.entities.RegisterStaticActor(candidate)
	} else {
		registered, ok = r.entities.RegisterStaticActorWithID(candidate)
	}
	if !ok {
		return StaticActorSnapshot{}, false
	}
	r.syncStaticActorCombatStateLocked(registered)
	if !r.setStaticActorKillQuestCreditLocked(registered.Entity.ID, killQuestCredit) {
		_, _ = r.entities.RemoveStaticActor(registered.Entity.ID)
		r.clearStaticActorCombatStateLocked(registered.Entity.ID)
		return StaticActorSnapshot{}, false
	}
	if !r.suppressStaticActorFanout {
		frames := encodeStaticActorVisibilityFrames(registered)
		if len(frames) > 0 {
			for _, target := range r.scopesLocked().VisibleTargetsForStaticActor(registered) {
				if characterAtBootstrapHPFloor(target.Character) {
					continue
				}
				r.enqueueToEntityLocked(target.Entity.ID, frames)
			}
		}
	}
	stored, ok := r.entities.StaticActor(registered.Entity.ID)
	if !ok {
		stored = registered
	}
	return r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, stored)), true
}

func (r *sharedWorldRegistry) UpdateStaticActor(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32) (StaticActorSnapshot, bool) {
	return r.updateStaticActor(entityID, name, mapIndex, x, y, raceNum, "", "", "", "")
}

func (r *sharedWorldRegistry) UpdateStaticActorWithInteraction(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string) (StaticActorSnapshot, bool) {
	return r.updateStaticActor(entityID, name, mapIndex, x, y, raceNum, interactionKind, interactionRef, "", "")
}

func (r *sharedWorldRegistry) UpdateStaticActorWithCombatKind(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32, combatKind string) (StaticActorSnapshot, bool) {
	return r.updateStaticActor(entityID, name, mapIndex, x, y, raceNum, "", "", combatKind, "")
}

func (r *sharedWorldRegistry) updateStaticActor(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string, combatKind string, spawnGroupRef string) (StaticActorSnapshot, bool) {
	spawnGroupRef = strings.TrimSpace(spawnGroupRef)
	if r == nil || r.entities == nil || entityID == 0 || !validStaticActorRuntimeName(name) || mapIndex == 0 || raceNum == 0 || !worldruntime.ValidStaticActorInteractionMetadata(interactionKind, interactionRef) || !worldruntime.ValidStaticActorCombatKind(combatKind) || !worldruntime.ValidStaticActorSpawnGroupRef(spawnGroupRef) {
		return StaticActorSnapshot{}, false
	}
	if spawnGroupRef != "" && (combatKind == "" || interactionKind != "" || interactionRef != "") {
		return StaticActorSnapshot{}, false
	}
	position := worldruntime.NewPosition(mapIndex, x, y)
	if !position.Valid() {
		return StaticActorSnapshot{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	previous, ok := r.entities.StaticActor(entityID)
	if !ok {
		return StaticActorSnapshot{}, false
	}
	if combatKind == "" {
		combatKind = previous.CombatKind
	}
	if spawnGroupRef == "" {
		spawnGroupRef = previous.SpawnGroupRef
	}
	targetActor := worldruntime.StaticEntity{
		Entity:          worldruntime.Entity{ID: entityID, Name: name},
		Position:        position,
		SpawnHome:       previous.SpawnHome,
		RaceNum:         raceNum,
		InteractionKind: interactionKind,
		InteractionRef:  interactionRef,
		SpawnGroupRef:   spawnGroupRef,
		CombatKind:      combatKind,
	}
	targetDiff := r.scopesLocked().RelocateStaticActorTargetDiff(previous, targetActor)
	actor, ok := r.entities.UpdateStaticActor(targetActor)
	if !ok {
		return StaticActorSnapshot{}, false
	}
	if previous.CombatKind != actor.CombatKind {
		r.clearStaticActorCombatStateLocked(actor.Entity.ID)
	}
	r.syncStaticActorCombatStateLocked(actor)

	// Same-map live spawn-backed position-only updates reuse retained-viewer MOVE.
	// Presentation/name/race/combat-profile refreshes, dead actors, cross-map
	// updates, and non-spawn static actors stay on delete/readd. Engagement /
	// selected-target clear remain on the already-owned update lifecycle.
	sameMapPositionOnlyMove := actor.SpawnGroupRef != "" &&
		previous.Position.SameMap(actor.Position) &&
		previous.Entity.Name == actor.Entity.Name &&
		previous.RaceNum == actor.RaceNum &&
		previous.CombatKind == actor.CombatKind &&
		!previous.Position.Equal(actor.Position) &&
		!r.staticActorDeadLocked(actor.Entity.ID)
	if sameMapPositionOnlyMove {
		if moveRaw, moveEncodable := encodeStaticActorChaseMoveFrame(actor); moveEncodable {
			for _, target := range targetDiff.RetainedVisibleTargets {
				if characterAtBootstrapHPFloor(target.Character) {
					continue
				}
				r.enqueueToEntityLocked(target.Entity.ID, [][]byte{moveRaw})
			}
		}
	} else {
		refreshFrames := r.buildStaticActorRefreshFramesLocked(previous, actor)
		if len(refreshFrames) > 0 {
			for _, target := range targetDiff.RetainedVisibleTargets {
				if characterAtBootstrapHPFloor(target.Character) {
					continue
				}
				r.enqueueToEntityLocked(target.Entity.ID, refreshFrames)
			}
		}
	}
	deleteRaw, deleteEncodable := encodeStaticActorDeleteFrame(previous)
	if deleteEncodable {
		for _, target := range targetDiff.RemovedVisibleTargets {
			if characterAtBootstrapHPFloor(target.Character) {
				continue
			}
			r.enqueueToEntityLocked(target.Entity.ID, [][]byte{deleteRaw})
		}
	}
	addFrames := r.encodeStaticActorVisibilityStateFramesLocked(actor)
	if len(addFrames) > 0 {
		for _, target := range targetDiff.AddedVisibleTargets {
			if characterAtBootstrapHPFloor(target.Character) {
				continue
			}
			r.enqueueToEntityLocked(target.Entity.ID, addFrames)
		}
	}
	if engagedBy := r.staticActorCombatEngagedBy[actor.Entity.ID]; engagedBy != 0 {
		delete(r.staticActorCombatEngagedBy, actor.Entity.ID)
		r.markProximityAggroSuppressLocked(actor.Entity.ID, engagedBy)
	}
	if targetVID, ok := worldruntime.StaticActorVisibilityVID(previous); ok {
		r.clearSelectedCombatTargetsLocked(targetVID, 0)
	}
	return r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, actor)), true
}

func (r *sharedWorldRegistry) clearStaticActorsForContentImportRollback() {
	if r == nil || r.entities == nil {
		return
	}
	removed := r.entities.AllStaticActors()
	for _, actor := range removed {
		_, _ = r.entities.RemoveStaticActor(actor.Entity.ID)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, actor := range removed {
		r.clearStaticActorCombatStateLocked(actor.Entity.ID)
		if r.staticActorKillQuestCredit != nil {
			delete(r.staticActorKillQuestCredit, actor.Entity.ID)
		}
	}
	r.pendingStaticActorImportDeletes = nil
}

func (r *sharedWorldRegistry) discardStaticActorImportFanout() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pendingStaticActorImportDeletes = nil
}

func (r *sharedWorldRegistry) flushStaticActorImportFanout() {
	if r == nil || r.entities == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	deletedActors := append([]worldruntime.StaticEntity(nil), r.pendingStaticActorImportDeletes...)
	r.pendingStaticActorImportDeletes = nil
	for _, actor := range deletedActors {
		deleteRaw, encodable := encodeStaticActorDeleteFrame(actor)
		if encodable {
			for _, target := range r.scopesLocked().VisibleTargetsForStaticActor(actor) {
				if characterAtBootstrapHPFloor(target.Character) {
					continue
				}
				r.enqueueToEntityLocked(target.Entity.ID, [][]byte{deleteRaw})
			}
		}
		if targetVID, ok := worldruntime.StaticActorVisibilityVID(actor); ok {
			r.clearSelectedCombatTargetsLocked(targetVID, 0)
		}
	}
	for _, actor := range r.entities.AllStaticActors() {
		frames := r.encodeStaticActorVisibilityStateFramesLocked(actor)
		if len(frames) == 0 {
			continue
		}
		for _, target := range r.scopesLocked().VisibleTargetsForStaticActor(actor) {
			if characterAtBootstrapHPFloor(target.Character) {
				continue
			}
			r.enqueueToEntityLocked(target.Entity.ID, frames)
		}
	}
}

func (r *sharedWorldRegistry) StaticActors() []StaticActorSnapshot {
	if r == nil || r.entities == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.markStaticActorSnapshotsStateLocked(r.scopesLocked().StaticActorSnapshots())
}

func (r *sharedWorldRegistry) SpawnGroups() []StaticActorSnapshot {
	if r == nil || r.entities == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.markStaticActorSnapshotsStateLocked(r.scopesLocked().SpawnGroupSnapshots())
}

func (r *sharedWorldRegistry) SpawnGroupsForMap(mapIndex uint32) ([]StaticActorSnapshot, bool) {
	if r == nil || r.entities == nil || mapIndex == 0 {
		return nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	groups, ok := r.scopesLocked().SpawnGroupsForMap(mapIndex)
	if !ok {
		return nil, false
	}
	return r.markStaticActorSnapshotsStateLocked(groups), true
}

func (r *sharedWorldRegistry) SpawnGroupLeashesForMap(mapIndex uint32, radius int32) ([]SpawnGroupLeashSnapshot, bool) {
	if r == nil || r.entities == nil || mapIndex == 0 || radius < 0 {
		return nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	groups, ok := r.scopesLocked().SpawnGroupsForMap(mapIndex)
	if !ok {
		return nil, false
	}
	leashes := make([]SpawnGroupLeashSnapshot, 0, len(groups))
	for _, group := range groups {
		actor, ok := r.entities.StaticActor(group.EntityID)
		if !ok || actor.SpawnGroupRef == "" {
			continue
		}
		effectiveRadius := radius
		if effectiveRadius == 0 {
			effectiveRadius = worldruntime.EffectiveStaticActorSpawnLeashRadiusForActor(actor)
		}
		evaluation, ok := worldruntime.EvaluateStaticActorCurrentSpawnLeash(actor, effectiveRadius)
		if !ok {
			continue
		}
		leashes = append(leashes, SpawnGroupLeashSnapshot{
			Actor:              r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, actor)),
			SpawnLeashSnapshot: worldruntime.SpawnLeashSnapshotFromEvaluation(evaluation),
		})
	}
	return leashes, true
}

func (r *sharedWorldRegistry) StaticActorsForMap(mapIndex uint32) ([]StaticActorSnapshot, bool) {
	if r == nil || r.entities == nil || mapIndex == 0 {
		return nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	actors, ok := r.scopesLocked().StaticActorsForMap(mapIndex)
	if !ok {
		return nil, false
	}
	return r.markStaticActorSnapshotsStateLocked(actors), true
}

func (r *sharedWorldRegistry) SpawnGroup(entityID uint64) (StaticActorSnapshot, bool) {
	if r == nil || r.entities == nil || entityID == 0 {
		return StaticActorSnapshot{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	actor, ok := r.entities.StaticActor(entityID)
	if !ok || actor.SpawnGroupRef == "" {
		return StaticActorSnapshot{}, false
	}
	return r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, actor)), true
}

func (r *sharedWorldRegistry) SpawnGroupByRef(ref string) (StaticActorSnapshot, bool) {
	if r == nil || r.entities == nil || !worldruntime.ValidStaticActorSpawnGroupRef(ref) || ref == "" {
		return StaticActorSnapshot{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot, ok := r.scopesLocked().SpawnGroupSnapshotByRef(ref)
	if !ok {
		return StaticActorSnapshot{}, false
	}
	return r.markStaticActorSnapshotStateLocked(snapshot), true
}

func (r *sharedWorldRegistry) SpawnGroupLeash(entityID uint64, radius int32) (SpawnGroupLeashSnapshot, bool) {
	if r == nil || r.entities == nil || entityID == 0 || radius < 0 {
		return SpawnGroupLeashSnapshot{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	actor, ok := r.entities.StaticActor(entityID)
	if !ok || actor.SpawnGroupRef == "" {
		return SpawnGroupLeashSnapshot{}, false
	}
	effectiveRadius := radius
	if effectiveRadius == 0 {
		effectiveRadius = worldruntime.EffectiveStaticActorSpawnLeashRadiusForActor(actor)
	}
	evaluation, ok := worldruntime.EvaluateStaticActorCurrentSpawnLeash(actor, effectiveRadius)
	if !ok {
		return SpawnGroupLeashSnapshot{}, false
	}
	return SpawnGroupLeashSnapshot{
		Actor:              r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, actor)),
		SpawnLeashSnapshot: worldruntime.SpawnLeashSnapshotFromEvaluation(evaluation),
	}, true
}

func (r *sharedWorldRegistry) PlanSpawnGroupReturnHomeStep(entityID uint64, maxStep int32) (worldruntime.SpawnLeashReturnStepPlan, bool) {
	if r == nil || r.entities == nil || entityID == 0 || maxStep <= 0 {
		return worldruntime.SpawnLeashReturnStepPlan{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	actor, ok := r.entities.StaticActor(entityID)
	if !ok || actor.SpawnGroupRef == "" {
		return worldruntime.SpawnLeashReturnStepPlan{}, false
	}
	currentHP, ok := r.ensureStaticActorCombatCurrentHPLocked(actor)
	if !ok || currentHP == 0 {
		return worldruntime.SpawnLeashReturnStepPlan{}, false
	}
	return worldruntime.PlanStaticActorSpawnLeashReturnStep(actor, worldruntime.EffectiveStaticActorSpawnLeashRadiusForActor(actor), maxStep)
}

func (r *sharedWorldRegistry) StepSpawnGroupReturnHome(entityID uint64, maxStep int32) (SpawnGroupReturnStepSnapshot, bool) {
	if r == nil || r.entities == nil || entityID == 0 || maxStep <= 0 {
		return SpawnGroupReturnStepSnapshot{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	actor, ok := r.entities.StaticActor(entityID)
	if !ok || actor.SpawnGroupRef == "" {
		return SpawnGroupReturnStepSnapshot{}, false
	}
	currentHP, ok := r.ensureStaticActorCombatCurrentHPLocked(actor)
	if !ok || currentHP == 0 {
		return SpawnGroupReturnStepSnapshot{}, false
	}
	plan, ok := worldruntime.PlanStaticActorSpawnLeashReturnStep(actor, worldruntime.EffectiveStaticActorSpawnLeashRadiusForActor(actor), maxStep)
	if !ok {
		return SpawnGroupReturnStepSnapshot{}, false
	}
	if plan.Complete && !plan.Evaluation.ReturnRequired {
		return r.spawnGroupReturnStepSnapshotLocked(actor, plan), true
	}

	steppedActor := actor
	steppedActor.Position = plan.Next
	targetDiff := r.scopesLocked().RelocateStaticActorTargetDiff(actor, steppedActor)
	updated, ok := r.entities.UpdateStaticActor(steppedActor)
	if !ok {
		return SpawnGroupReturnStepSnapshot{}, false
	}
	r.syncStaticActorCombatStateLocked(updated)

	// Same-map retained viewers reuse server MOVE replication; remove/add stay on
	// delete/bootstrap. Cross-map return-step keeps delete/readd because no return
	// warp packet seam is owned yet. Engagement / selected-target clear remain.
	sameMapReturn := actor.Position.SameMap(updated.Position)
	if sameMapReturn {
		if moveRaw, moveEncodable := encodeStaticActorChaseMoveFrame(updated); moveEncodable {
			for _, target := range targetDiff.RetainedVisibleTargets {
				if characterAtBootstrapHPFloor(target.Character) {
					continue
				}
				r.enqueueToEntityLocked(target.Entity.ID, [][]byte{moveRaw})
			}
		}
	} else {
		refreshFrames := r.buildStaticActorRefreshFramesLocked(actor, updated)
		if len(refreshFrames) > 0 {
			for _, target := range targetDiff.RetainedVisibleTargets {
				if characterAtBootstrapHPFloor(target.Character) {
					continue
				}
				r.enqueueToEntityLocked(target.Entity.ID, refreshFrames)
			}
		}
	}
	deleteRaw, deleteEncodable := encodeStaticActorDeleteFrame(actor)
	if deleteEncodable {
		for _, target := range targetDiff.RemovedVisibleTargets {
			if characterAtBootstrapHPFloor(target.Character) {
				continue
			}
			r.enqueueToEntityLocked(target.Entity.ID, [][]byte{deleteRaw})
		}
	}
	addFrames := r.encodeStaticActorVisibilityStateFramesLocked(updated)
	if len(addFrames) > 0 {
		for _, target := range targetDiff.AddedVisibleTargets {
			if characterAtBootstrapHPFloor(target.Character) {
				continue
			}
			r.enqueueToEntityLocked(target.Entity.ID, addFrames)
		}
	}
	if engagedBy := r.staticActorCombatEngagedBy[updated.Entity.ID]; engagedBy != 0 {
		delete(r.staticActorCombatEngagedBy, updated.Entity.ID)
		r.markProximityAggroSuppressLocked(updated.Entity.ID, engagedBy)
	}
	if targetVID, ok := worldruntime.StaticActorVisibilityVID(actor); ok {
		r.clearSelectedCombatTargetsLocked(targetVID, 0)
	}
	return r.spawnGroupReturnStepSnapshotLocked(updated, plan), true
}

func (r *sharedWorldRegistry) spawnGroupReturnStepSnapshotLocked(actor worldruntime.StaticEntity, plan worldruntime.SpawnLeashReturnStepPlan) SpawnGroupReturnStepSnapshot {
	return SpawnGroupReturnStepSnapshot{
		Actor: r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, actor)),
		Step: SpawnLeashReturnStepSnapshot{
			SpawnLeashSnapshot: worldruntime.SpawnLeashSnapshotFromEvaluation(plan.Evaluation),
			Next:               worldruntime.PositionSnapshotFromPosition(plan.Next),
			Complete:           plan.Complete,
		},
	}
}

func (r *sharedWorldRegistry) PlanSpawnGroupChaseStep(entityID uint64, owner worldruntime.Position, maxStep int32) (worldruntime.SpawnChaseStepPlan, bool) {
	if r == nil || r.entities == nil || entityID == 0 || maxStep <= 0 || !owner.Valid() {
		return worldruntime.SpawnChaseStepPlan{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	actor, ok := r.entities.StaticActor(entityID)
	if !ok || actor.SpawnGroupRef == "" {
		return worldruntime.SpawnChaseStepPlan{}, false
	}
	currentHP, ok := r.ensureStaticActorCombatCurrentHPLocked(actor)
	if !ok || currentHP == 0 {
		return worldruntime.SpawnChaseStepPlan{}, false
	}
	return worldruntime.PlanStaticActorSpawnChaseStep(actor, owner, worldruntime.EffectiveStaticActorSpawnLeashRadiusForActor(actor), maxStep)
}

func (r *sharedWorldRegistry) PlanSpawnGroupHomewardStep(entityID uint64, maxStep int32) (worldruntime.SpawnLeashHomewardStepPlan, bool) {
	if r == nil || r.entities == nil || entityID == 0 || maxStep <= 0 {
		return worldruntime.SpawnLeashHomewardStepPlan{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	actor, ok := r.entities.StaticActor(entityID)
	if !ok || actor.SpawnGroupRef == "" {
		return worldruntime.SpawnLeashHomewardStepPlan{}, false
	}
	currentHP, ok := r.ensureStaticActorCombatCurrentHPLocked(actor)
	if !ok || currentHP == 0 {
		return worldruntime.SpawnLeashHomewardStepPlan{}, false
	}
	if engagedBy := r.staticActorCombatEngagedBy[entityID]; engagedBy != 0 {
		return worldruntime.SpawnLeashHomewardStepPlan{}, false
	}
	return worldruntime.PlanStaticActorSpawnLeashHomewardStep(actor, worldruntime.EffectiveStaticActorSpawnLeashRadiusForActor(actor), maxStep)
}

// StepSpawnGroupHomeward applies one planned within-radius homeward step toward
// authored home for an unengaged live spawn-backed actor. Same-map retained
// viewers reuse server MOVE replication; engagement/selected-target ownership
// stay cleared (homeward never invents chase-style preservation).
func (r *sharedWorldRegistry) StepSpawnGroupHomeward(entityID uint64, maxStep int32) (SpawnGroupReturnStepSnapshot, bool) {
	if r == nil || r.entities == nil || entityID == 0 || maxStep <= 0 {
		return SpawnGroupReturnStepSnapshot{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	actor, ok := r.entities.StaticActor(entityID)
	if !ok || actor.SpawnGroupRef == "" {
		return SpawnGroupReturnStepSnapshot{}, false
	}
	currentHP, ok := r.ensureStaticActorCombatCurrentHPLocked(actor)
	if !ok || currentHP == 0 {
		return SpawnGroupReturnStepSnapshot{}, false
	}
	if engagedBy := r.staticActorCombatEngagedBy[entityID]; engagedBy != 0 {
		return SpawnGroupReturnStepSnapshot{}, false
	}
	plan, ok := worldruntime.PlanStaticActorSpawnLeashHomewardStep(actor, worldruntime.EffectiveStaticActorSpawnLeashRadiusForActor(actor), maxStep)
	if !ok {
		return SpawnGroupReturnStepSnapshot{}, false
	}
	if plan.Complete && plan.Next.Equal(actor.Position) {
		return SpawnGroupReturnStepSnapshot{
			Actor: r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, actor)),
			Step: SpawnLeashReturnStepSnapshot{
				SpawnLeashSnapshot: worldruntime.SpawnLeashSnapshotFromEvaluation(plan.Evaluation),
				Next:               worldruntime.PositionSnapshotFromPosition(plan.Next),
				Complete:           true,
			},
		}, true
	}

	steppedActor := actor
	steppedActor.Position = plan.Next
	targetDiff := r.scopesLocked().RelocateStaticActorTargetDiff(actor, steppedActor)
	updated, ok := r.entities.UpdateStaticActor(steppedActor)
	if !ok {
		return SpawnGroupReturnStepSnapshot{}, false
	}
	r.syncStaticActorCombatStateLocked(updated)

	if moveRaw, moveEncodable := encodeStaticActorChaseMoveFrame(updated); moveEncodable {
		for _, target := range targetDiff.RetainedVisibleTargets {
			if characterAtBootstrapHPFloor(target.Character) {
				continue
			}
			r.enqueueToEntityLocked(target.Entity.ID, [][]byte{moveRaw})
		}
	}
	deleteRaw, deleteEncodable := encodeStaticActorDeleteFrame(actor)
	if deleteEncodable {
		for _, target := range targetDiff.RemovedVisibleTargets {
			if characterAtBootstrapHPFloor(target.Character) {
				continue
			}
			r.enqueueToEntityLocked(target.Entity.ID, [][]byte{deleteRaw})
		}
	}
	addFrames := r.encodeStaticActorVisibilityStateFramesLocked(updated)
	if len(addFrames) > 0 {
		for _, target := range targetDiff.AddedVisibleTargets {
			if characterAtBootstrapHPFloor(target.Character) {
				continue
			}
			r.enqueueToEntityLocked(target.Entity.ID, addFrames)
		}
	}
	if engagedBy := r.staticActorCombatEngagedBy[updated.Entity.ID]; engagedBy != 0 {
		delete(r.staticActorCombatEngagedBy, updated.Entity.ID)
		r.markProximityAggroSuppressLocked(updated.Entity.ID, engagedBy)
	}
	if targetVID, ok := worldruntime.StaticActorVisibilityVID(actor); ok {
		r.clearSelectedCombatTargetsLocked(targetVID, 0)
	}
	return SpawnGroupReturnStepSnapshot{
		Actor: r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, updated)),
		Step: SpawnLeashReturnStepSnapshot{
			SpawnLeashSnapshot: worldruntime.SpawnLeashSnapshotFromEvaluation(plan.Evaluation),
			Next:               worldruntime.PositionSnapshotFromPosition(plan.Next),
			Complete:           plan.Complete,
		},
	}, true
}

// StepSpawnGroupChase applies one planned chase step toward the engaged owner.
// Unlike return-step recovery, a successful chase move preserves engagement and
// selected-target ownership and does not advance the combat snapshot version.
func (r *sharedWorldRegistry) StepSpawnGroupChase(entityID uint64, owner worldruntime.Position, maxStep int32) (SpawnGroupReturnStepSnapshot, bool) {
	if r == nil || r.entities == nil || entityID == 0 || maxStep <= 0 || !owner.Valid() {
		return SpawnGroupReturnStepSnapshot{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	actor, ok := r.entities.StaticActor(entityID)
	if !ok || actor.SpawnGroupRef == "" {
		return SpawnGroupReturnStepSnapshot{}, false
	}
	currentHP, ok := r.ensureStaticActorCombatCurrentHPLocked(actor)
	if !ok || currentHP == 0 {
		return SpawnGroupReturnStepSnapshot{}, false
	}
	plan, ok := worldruntime.PlanStaticActorSpawnChaseStep(actor, owner, worldruntime.EffectiveStaticActorSpawnLeashRadiusForActor(actor), maxStep)
	if !ok {
		return SpawnGroupReturnStepSnapshot{}, false
	}
	if plan.Complete && plan.Next.Equal(actor.Position) {
		return SpawnGroupReturnStepSnapshot{
			Actor: r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, actor)),
			Step: SpawnLeashReturnStepSnapshot{
				SpawnLeashSnapshot: worldruntime.SpawnLeashSnapshotFromEvaluation(plan.Evaluation),
				Next:               worldruntime.PositionSnapshotFromPosition(plan.Next),
				Complete:           true,
			},
		}, true
	}

	steppedActor := actor
	steppedActor.Position = plan.Next
	targetDiff := r.scopesLocked().RelocateStaticActorTargetDiff(actor, steppedActor)
	updated, ok := r.entities.UpdateStaticActor(steppedActor)
	if !ok {
		return SpawnGroupReturnStepSnapshot{}, false
	}
	// Preserve live combat snapshot ownership across chase movement so selected
	// attacks and delayed retaliation timers stay valid for the same engagement.
	// Retained viewers reuse server MOVE replication instead of delete/readd;
	// remove/add visibility membership still uses the ordinary delete/bootstrap path.

	if moveRaw, moveEncodable := encodeStaticActorChaseMoveFrame(updated); moveEncodable {
		for _, target := range targetDiff.RetainedVisibleTargets {
			if characterAtBootstrapHPFloor(target.Character) {
				continue
			}
			r.enqueueToEntityLocked(target.Entity.ID, [][]byte{moveRaw})
		}
	}
	deleteRaw, deleteEncodable := encodeStaticActorDeleteFrame(actor)
	if deleteEncodable {
		for _, target := range targetDiff.RemovedVisibleTargets {
			if characterAtBootstrapHPFloor(target.Character) {
				continue
			}
			r.enqueueToEntityLocked(target.Entity.ID, [][]byte{deleteRaw})
		}
	}
	addFrames := r.encodeStaticActorVisibilityStateFramesLocked(updated)
	if len(addFrames) > 0 {
		for _, target := range targetDiff.AddedVisibleTargets {
			if characterAtBootstrapHPFloor(target.Character) {
				continue
			}
			r.enqueueToEntityLocked(target.Entity.ID, addFrames)
		}
	}
	return SpawnGroupReturnStepSnapshot{
		Actor: r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, updated)),
		Step: SpawnLeashReturnStepSnapshot{
			SpawnLeashSnapshot: worldruntime.SpawnLeashSnapshotFromEvaluation(plan.Evaluation),
			Next:               worldruntime.PositionSnapshotFromPosition(plan.Next),
			Complete:           plan.Complete,
		},
	}, true
}

func (r *sharedWorldRegistry) ReturnSpawnGroupHome(entityID uint64) (SpawnGroupLeashSnapshot, bool) {
	if r == nil || r.entities == nil || entityID == 0 {
		return SpawnGroupLeashSnapshot{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	actor, ok := r.entities.StaticActor(entityID)
	if !ok || actor.SpawnGroupRef == "" {
		return SpawnGroupLeashSnapshot{}, false
	}
	evaluation, ok := worldruntime.EvaluateStaticActorCurrentSpawnLeash(actor, worldruntime.EffectiveStaticActorSpawnLeashRadiusForActor(actor))
	if !ok {
		return SpawnGroupLeashSnapshot{}, false
	}
	currentHP, ok := r.ensureStaticActorCombatCurrentHPLocked(actor)
	if !ok || currentHP == 0 {
		return SpawnGroupLeashSnapshot{}, false
	}
	if !evaluation.ReturnRequired && evaluation.Status == worldruntime.SpawnLeashStatusAtHome {
		r.syncStaticActorCombatStateLocked(actor)
		if engagedBy := r.staticActorCombatEngagedBy[actor.Entity.ID]; engagedBy != 0 {
			delete(r.staticActorCombatEngagedBy, actor.Entity.ID)
			r.markProximityAggroSuppressLocked(actor.Entity.ID, engagedBy)
		}
		if targetVID, ok := worldruntime.StaticActorVisibilityVID(actor); ok {
			r.clearSelectedCombatTargetsLocked(targetVID, 0)
		}
		return SpawnGroupLeashSnapshot{
			Actor:              r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, actor)),
			SpawnLeashSnapshot: worldruntime.SpawnLeashSnapshotFromEvaluation(evaluation),
		}, true
	}

	returnedActor := actor
	returnedActor.Position = evaluation.Home
	if returnedActor.Position.Equal(actor.Position) {
		return SpawnGroupLeashSnapshot{
			Actor:              r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, actor)),
			SpawnLeashSnapshot: worldruntime.SpawnLeashSnapshotFromEvaluation(evaluation),
		}, true
	}

	targetDiff := r.scopesLocked().RelocateStaticActorTargetDiff(actor, returnedActor)
	updated, ok := r.entities.UpdateStaticActor(returnedActor)
	if !ok {
		return SpawnGroupLeashSnapshot{}, false
	}
	r.syncStaticActorCombatStateLocked(updated)

	// Same-map retained viewers reuse server MOVE replication for exact return-home;
	// remove/add stay on delete/bootstrap. Cross-map return-home keeps delete/readd.
	sameMapReturn := actor.Position.SameMap(updated.Position)
	if sameMapReturn {
		if moveRaw, moveEncodable := encodeStaticActorChaseMoveFrame(updated); moveEncodable {
			for _, target := range targetDiff.RetainedVisibleTargets {
				if characterAtBootstrapHPFloor(target.Character) {
					continue
				}
				r.enqueueToEntityLocked(target.Entity.ID, [][]byte{moveRaw})
			}
		}
	} else {
		refreshFrames := r.buildStaticActorRefreshFramesLocked(actor, updated)
		if len(refreshFrames) > 0 {
			for _, target := range targetDiff.RetainedVisibleTargets {
				if characterAtBootstrapHPFloor(target.Character) {
					continue
				}
				r.enqueueToEntityLocked(target.Entity.ID, refreshFrames)
			}
		}
	}
	deleteRaw, deleteEncodable := encodeStaticActorDeleteFrame(actor)
	if deleteEncodable {
		for _, target := range targetDiff.RemovedVisibleTargets {
			if characterAtBootstrapHPFloor(target.Character) {
				continue
			}
			r.enqueueToEntityLocked(target.Entity.ID, [][]byte{deleteRaw})
		}
	}
	addFrames := r.encodeStaticActorVisibilityStateFramesLocked(updated)
	if len(addFrames) > 0 {
		for _, target := range targetDiff.AddedVisibleTargets {
			if characterAtBootstrapHPFloor(target.Character) {
				continue
			}
			r.enqueueToEntityLocked(target.Entity.ID, addFrames)
		}
	}
	if engagedBy := r.staticActorCombatEngagedBy[updated.Entity.ID]; engagedBy != 0 {
		delete(r.staticActorCombatEngagedBy, updated.Entity.ID)
		r.markProximityAggroSuppressLocked(updated.Entity.ID, engagedBy)
	}
	if targetVID, ok := worldruntime.StaticActorVisibilityVID(actor); ok {
		r.clearSelectedCombatTargetsLocked(targetVID, 0)
	}
	return r.spawnGroupLeashLocked(updated, worldruntime.EffectiveStaticActorSpawnLeashRadiusForActor(updated))
}

func (r *sharedWorldRegistry) spawnGroupLeashLocked(actor worldruntime.StaticEntity, radius int32) (SpawnGroupLeashSnapshot, bool) {
	if r == nil || actor.Entity.ID == 0 || actor.SpawnGroupRef == "" {
		return SpawnGroupLeashSnapshot{}, false
	}
	evaluation, ok := worldruntime.EvaluateStaticActorCurrentSpawnLeash(actor, radius)
	if !ok {
		return SpawnGroupLeashSnapshot{}, false
	}
	return SpawnGroupLeashSnapshot{
		Actor:              r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, actor)),
		SpawnLeashSnapshot: worldruntime.SpawnLeashSnapshotFromEvaluation(evaluation),
	}, true
}

func (r *sharedWorldRegistry) StaticActor(entityID uint64) (StaticActorSnapshot, bool) {
	if r == nil || r.entities == nil || entityID == 0 {
		return StaticActorSnapshot{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	actor, ok := r.entities.StaticActor(entityID)
	if !ok {
		return StaticActorSnapshot{}, false
	}
	return r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, actor)), true
}

func (r *sharedWorldRegistry) VisibleStaticActorFrames(subject loginticket.Character) [][]byte {
	if r == nil || r.entities == nil || characterAtBootstrapHPFloor(subject) {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	actors := r.scopesLocked().VisibleStaticActors(subject)
	frames := make([][]byte, 0, len(actors)*4)
	for _, actor := range actors {
		frames = append(frames, r.encodeStaticActorVisibilityStateFramesLocked(actor)...)
	}
	return frames
}

// VisibleStaticActorRefreshFrames rebuilds currently visible static actors for a
// recovered live subject with delete-plus-state catch-up frames. This is the
// same-socket /restart_here companion for zero-HP recipient skips that withheld
// later practice-mob / static-actor lifecycle delivery while the owner was dead.
func (r *sharedWorldRegistry) VisibleStaticActorRefreshFrames(subject loginticket.Character) [][]byte {
	if r == nil || r.entities == nil || characterAtBootstrapHPFloor(subject) {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	actors := r.scopesLocked().VisibleStaticActors(subject)
	frames := make([][]byte, 0, len(actors)*5)
	for _, actor := range actors {
		frames = append(frames, r.buildStaticActorRefreshFramesLocked(actor, actor)...)
	}
	return frames
}

func (r *sharedWorldRegistry) VisibleGroundItemFrames(subject loginticket.Character) [][]byte {
	if r == nil || len(r.groundItemsByVID) == 0 || characterAtBootstrapHPFloor(subject) {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	visible := make([]sharedGroundItem, 0)
	for _, ground := range r.groundItemsByVID {
		if r.groundItemVisibleToCharacterLocked(ground, subject) {
			visible = append(visible, ground)
		}
	}
	sortSharedGroundItemsByVID(visible)
	frames := make([][]byte, 0, len(visible)*2)
	for _, ground := range visible {
		frames = append(frames, encodeGroundItemVisibleFrames(ground)...)
	}
	return frames
}

func (r *sharedWorldRegistry) AttemptStaticActorInteraction(subjectID uint64, targetVID uint32) StaticActorInteractionAttempt {
	attempt := StaticActorInteractionAttempt{TargetVID: targetVID}
	if r == nil || r.entities == nil {
		attempt.Failure = StaticActorInteractionFailureSubjectNotFound
		return attempt
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	subject, ok := r.playerCharacter(subjectID)
	if !ok {
		attempt.Failure = StaticActorInteractionFailureSubjectNotFound
		return attempt
	}
	if characterAtBootstrapHPFloor(subject) {
		attempt.Failure = StaticActorInteractionFailureSubjectDead
		return attempt
	}
	actor, ok := r.scopesLocked().VisibleStaticActorByVID(subject, targetVID)
	if !ok {
		attempt.Failure = StaticActorInteractionFailureTargetNotVisible
		return attempt
	}
	attempt.Actor = r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, actor))
	if !worldruntime.StaticActorWithinInteractionRange(subject, actor, staticActorInteractionMaxDistance) {
		attempt.Failure = StaticActorInteractionFailureTargetOutOfRange
		return attempt
	}
	if actor.InteractionKind == "" || actor.InteractionRef == "" {
		attempt.Failure = StaticActorInteractionFailureTargetHasNoInteraction
		return attempt
	}
	if r.staticActorDeadLocked(actor.Entity.ID) {
		attempt.Failure = StaticActorInteractionFailureTargetDead
		return attempt
	}
	attempt.Accepted = true
	return attempt
}

func (r *sharedWorldRegistry) AttemptStaticActorCombatTarget(subjectID uint64, targetVID uint32) StaticActorCombatTargetAttempt {
	attempt := StaticActorCombatTargetAttempt{TargetVID: targetVID}
	if r == nil || r.entities == nil {
		attempt.Failure = StaticActorCombatTargetFailureSubjectNotFound
		return attempt
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	subject, ok := r.playerCharacter(subjectID)
	if !ok {
		attempt.Failure = StaticActorCombatTargetFailureSubjectNotFound
		return attempt
	}
	return r.attemptStaticActorCombatTargetLocked(subjectID, subject, targetVID)
}

func (r *sharedWorldRegistry) AttemptSelectedStaticActorAttack(subjectID uint64, activeTargetVID uint32, activeTargetSnapshotVersion uint64, requestedTargetVID uint32) StaticActorCombatAttackAttempt {
	attempt := StaticActorCombatAttackAttempt{ActiveTargetVID: activeTargetVID, ActiveTargetSnapshotVersion: activeTargetSnapshotVersion, RequestedTargetVID: requestedTargetVID}
	if r == nil || r.entities == nil {
		attempt.Failure = StaticActorCombatAttackFailureSubjectNotFound
		return attempt
	}
	if activeTargetVID == 0 || activeTargetSnapshotVersion == 0 {
		attempt.Failure = StaticActorCombatAttackFailureNoActiveTarget
		return attempt
	}
	if requestedTargetVID == 0 || requestedTargetVID != activeTargetVID {
		attempt.Failure = StaticActorCombatAttackFailureTargetMismatch
		return attempt
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	subject, ok := r.playerCharacter(subjectID)
	if !ok {
		attempt.Failure = StaticActorCombatAttackFailureSubjectNotFound
		return attempt
	}
	if characterAtBootstrapHPFloor(subject) {
		attempt.Failure = StaticActorCombatAttackFailureSubjectDead
		return attempt
	}
	selectedTargetVID, ok := r.sessionCombatTargetLocked(subjectID)
	if !ok || selectedTargetVID != activeTargetVID {
		attempt.Failure = StaticActorCombatAttackFailureNoActiveTarget
		return attempt
	}
	targetAttempt := r.attemptStaticActorCombatTargetLocked(subjectID, subject, activeTargetVID)
	attempt.Actor = targetAttempt.Actor
	attempt.HPPercent = targetAttempt.HPPercent
	if !targetAttempt.Accepted {
		switch targetAttempt.Failure {
		case StaticActorCombatTargetFailureSubjectNotFound:
			attempt.Failure = StaticActorCombatAttackFailureSubjectNotFound
		case StaticActorCombatTargetFailureSubjectDead:
			attempt.Failure = StaticActorCombatAttackFailureSubjectDead
		case StaticActorCombatTargetFailureTargetNotVisible:
			attempt.Failure = StaticActorCombatAttackFailureTargetNotVisible
		case StaticActorCombatTargetFailureTargetOutOfRange:
			attempt.Failure = StaticActorCombatAttackFailureTargetOutOfRange
		case StaticActorCombatTargetFailureTargetNotTargetable:
			attempt.Failure = StaticActorCombatAttackFailureTargetNotTargetable
		case StaticActorCombatTargetFailureTargetReturnRequired:
			attempt.Failure = StaticActorCombatAttackFailureTargetReturnRequired
		case StaticActorCombatTargetFailureTargetEngaged:
			attempt.Failure = StaticActorCombatAttackFailureTargetEngaged
		case StaticActorCombatTargetFailureTargetDead:
			attempt.Failure = StaticActorCombatAttackFailureTargetDead
		default:
			attempt.Failure = targetAttempt.Failure
		}
		return attempt
	}
	actor, ok := r.scopesLocked().VisibleStaticActorByVID(subject, activeTargetVID)
	if !ok {
		attempt.Accepted = false
		attempt.Failure = StaticActorCombatAttackFailureTargetNotVisible
		return attempt
	}
	attempt.Actor = r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, actor))
	currentSnapshotVersion := r.staticActorCombatSnapshotLocked(actor.Entity.ID)
	if currentSnapshotVersion == 0 || currentSnapshotVersion != activeTargetSnapshotVersion {
		attempt.Accepted = false
		attempt.Failure = StaticActorCombatAttackFailureTargetSnapshotMismatch
		return attempt
	}
	currentHP, ok := r.ensureStaticActorCombatCurrentHPLocked(actor)
	if !ok {
		attempt.Accepted = false
		attempt.Failure = StaticActorCombatAttackFailureTargetNotTargetable
		return attempt
	}
	if currentHP == 0 {
		attempt.Accepted = false
		attempt.Failure = StaticActorCombatAttackFailureTargetDead
		return attempt
	}
	damage, ok := worldruntime.BootstrapStaticActorNormalAttackDamage(actor.CombatKind)
	if !ok {
		attempt.Accepted = false
		attempt.Failure = StaticActorCombatAttackFailureTargetNotTargetable
		return attempt
	}
	nextHP, hpPercent, ok := worldruntime.ApplyBootstrapStaticActorNormalAttack(actor.CombatKind, currentHP)
	if !ok {
		attempt.Accepted = false
		attempt.Failure = StaticActorCombatAttackFailureTargetNotTargetable
		return attempt
	}
	r.staticActorCombatHP[actor.Entity.ID] = nextHP
	r.setStaticActorCombatEngagementLocked(actor.Entity.ID, subjectID)
	if actor.SpawnGroupRef != "" && staticActorSpawnGroupAggroLiteCombatKind(actor.CombatKind) {
		r.clearOtherSessionCombatTargetsLocked(subjectID, activeTargetVID)
	}
	attempt.Accepted = true
	attempt.HPPercent = hpPercent
	attempt.Damage = damage
	if nextHP == 0 {
		attempt.Died = true
		attempt.DeathReward = r.staticActorDeathRewardLocked(actor)
		r.releaseStaticActorCombatEngagementLocked(actor, true)
		deadRaw := worldproto.EncodeDead(worldproto.DeadPacket{VID: requestedTargetVID})
		clearRaw := combatproto.EncodeServerClearTarget()
		targetedSessionIDs := make(map[uint64]struct{})
		for entityID, targetVID := range r.sessionCombatTargets {
			if targetVID != requestedTargetVID {
				continue
			}
			targetedSessionIDs[entityID] = struct{}{}
			delete(r.sessionCombatTargets, entityID)
			if r.sessionCombatRetaliations != nil {
				delete(r.sessionCombatRetaliations, entityID)
			}
		}
		for _, target := range r.scopesLocked().VisibleTargetsForStaticActor(actor) {
			if target.Entity.ID == subjectID {
				continue
			}
			if characterAtBootstrapHPFloor(target.Character) {
				continue
			}
			frames := [][]byte{deadRaw}
			if _, ok := targetedSessionIDs[target.Entity.ID]; ok {
				frames = append(frames, clearRaw)
			}
			r.enqueueToEntityLocked(target.Entity.ID, frames)
		}
		r.scheduleStaticActorCombatRespawnLocked(actor)
	}
	return attempt
}

func staticActorCombatKindTargetable(combatKind string) bool {
	_, ok := worldruntime.BootstrapStaticActorCombatProfileDefaults(combatKind)
	return ok
}

func (r *sharedWorldRegistry) attemptStaticActorCombatTargetLocked(subjectID uint64, subject loginticket.Character, targetVID uint32) StaticActorCombatTargetAttempt {
	attempt := StaticActorCombatTargetAttempt{TargetVID: targetVID}
	if characterAtBootstrapHPFloor(subject) {
		attempt.Failure = StaticActorCombatTargetFailureSubjectDead
		return attempt
	}
	actor, ok := r.scopesLocked().VisibleStaticActorByVID(subject, targetVID)
	if !ok {
		attempt.Failure = StaticActorCombatTargetFailureTargetNotVisible
		return attempt
	}
	attempt.Actor = r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, actor))
	if !worldruntime.StaticActorWithinInteractionRange(subject, actor, staticActorCombatTargetMaxDistance) {
		attempt.Failure = StaticActorCombatTargetFailureTargetOutOfRange
		return attempt
	}
	if !staticActorCombatKindTargetable(actor.CombatKind) {
		attempt.Failure = StaticActorCombatTargetFailureTargetNotTargetable
		return attempt
	}
	if actor.SpawnGroupRef != "" {
		leash, ok := worldruntime.EvaluateStaticActorCurrentSpawnLeash(actor, worldruntime.EffectiveStaticActorSpawnLeashRadiusForActor(actor))
		if !ok {
			attempt.Failure = StaticActorCombatTargetFailureTargetNotTargetable
			return attempt
		}
		if leash.ReturnRequired {
			attempt.Failure = StaticActorCombatTargetFailureTargetReturnRequired
			return attempt
		}
	}
	currentHP, ok := r.ensureStaticActorCombatCurrentHPLocked(actor)
	if !ok {
		attempt.Failure = StaticActorCombatTargetFailureTargetNotTargetable
		return attempt
	}
	if currentHP == 0 {
		attempt.Failure = StaticActorCombatTargetFailureTargetDead
		return attempt
	}
	if r.staticActorAggroLiteBlocksFreshTargetLocked(subjectID, actor, targetVID) {
		attempt.Failure = StaticActorCombatTargetFailureTargetEngaged
		return attempt
	}
	hpPercent, ok := worldruntime.BootstrapStaticActorHPPercent(actor.CombatKind, currentHP)
	if !ok {
		attempt.Failure = StaticActorCombatTargetFailureTargetNotTargetable
		return attempt
	}
	attempt.Accepted = true
	attempt.SnapshotVersion = r.staticActorCombatSnapshotLocked(actor.Entity.ID)
	attempt.HPPercent = hpPercent
	return attempt
}

func (r *sharedWorldRegistry) RemoveStaticActor(entityID uint64) (StaticActorSnapshot, bool) {
	if r == nil || r.entities == nil || entityID == 0 {
		return StaticActorSnapshot{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	actor, ok := r.entities.RemoveStaticActor(entityID)
	if !ok {
		return StaticActorSnapshot{}, false
	}
	removedSnapshot := r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, actor))
	r.clearStaticActorCombatStateLocked(entityID)
	if r.staticActorKillQuestCredit != nil {
		delete(r.staticActorKillQuestCredit, entityID)
	}
	if r.suppressStaticActorFanout {
		r.pendingStaticActorImportDeletes = append(r.pendingStaticActorImportDeletes, actor)
	} else {
		deleteRaw, encodable := encodeStaticActorDeleteFrame(actor)
		if encodable {
			for _, target := range r.scopesLocked().VisibleTargetsForStaticActor(actor) {
				if characterAtBootstrapHPFloor(target.Character) {
					continue
				}
				r.enqueueToEntityLocked(target.Entity.ID, [][]byte{deleteRaw})
			}
		}
		if targetVID, ok := worldruntime.StaticActorVisibilityVID(actor); ok {
			r.clearSelectedCombatTargetsLocked(targetVID, 0)
		}
	}
	return removedSnapshot, true
}

func (r *sharedWorldRegistry) PreviewRelocation(name string, mapIndex uint32, x int32, y int32) (RelocationPreview, bool) {
	if r == nil || name == "" || mapIndex == 0 {
		return RelocationPreview{}, false
	}

	current, ok := r.playerCharacterByName(name)
	if !ok {
		return RelocationPreview{}, false
	}
	target := current
	target.MapIndex = mapIndex
	target.X = x
	target.Y = y

	groundItemOccupancies := r.groundItemOccupanciesLocked()
	return r.markRelocationPreviewStateLocked(r.scopesLocked().BuildRelocationPreviewWithGroundItems(current, target, false, groundItemOccupancies)), true
}

func (r *sharedWorldRegistry) EnqueueToOtherSessions(originID uint64, frames [][]byte) {
	if r == nil || originID == 0 || len(frames) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, target := range r.scopesLocked().PartyTargets(originID) {
		if characterAtBootstrapHPFloor(target.Character) {
			continue
		}
		r.enqueueToEntityLocked(target.Entity.ID, frames)
	}
}

func (r *sharedWorldRegistry) EnqueueToVisibleSessions(originID uint64, origin loginticket.Character, frames [][]byte) {
	if r == nil || originID == 0 || len(frames) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, target := range r.scopesLocked().VisibleTargets(originID, origin) {
		if characterAtBootstrapHPFloor(target.Character) {
			continue
		}
		r.enqueueToEntityLocked(target.Entity.ID, frames)
	}
}

func (r *sharedWorldRegistry) EnqueueToOtherSessionsInEmpire(originID uint64, origin loginticket.Character, frames [][]byte) {
	if r == nil || originID == 0 || origin.Empire == 0 || len(frames) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, target := range r.scopesLocked().ShoutTargets(originID, origin) {
		if characterAtBootstrapHPFloor(target.Character) {
			continue
		}
		r.enqueueToEntityLocked(target.Entity.ID, frames)
	}
}

func (r *sharedWorldRegistry) EnqueueToOtherSessionsInEmpireOnMap(originID uint64, origin loginticket.Character, frames [][]byte) {
	if r == nil || originID == 0 || origin.Empire == 0 || len(frames) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, target := range r.scopesLocked().LocalTalkTargets(originID, origin) {
		if characterAtBootstrapHPFloor(target.Character) {
			continue
		}
		r.enqueueToEntityLocked(target.Entity.ID, frames)
	}
}

func (r *sharedWorldRegistry) EnqueueToOtherSessionsInGuild(originID uint64, origin loginticket.Character, frames [][]byte) {
	if r == nil || originID == 0 || origin.GuildID == 0 || len(frames) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, target := range r.scopesLocked().GuildTargets(originID, origin) {
		if characterAtBootstrapHPFloor(target.Character) {
			continue
		}
		r.enqueueToEntityLocked(target.Entity.ID, frames)
	}
}

func (r *sharedWorldRegistry) EnqueueToCharacterName(name string, frames [][]byte) (bool, bool) {
	if r == nil || name == "" || len(frames) == 0 {
		return false, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	playerEntity, ok := r.scopesLocked().PlayerByExactName(name)
	if !ok {
		return false, true
	}
	if characterAtBootstrapHPFloor(playerEntity.Character) {
		return false, false
	}
	if !r.enqueueToEntityLocked(playerEntity.Entity.ID, frames) {
		return false, false
	}
	return true, false
}

func (r *sharedWorldRegistry) EnqueueSystemNotice(message string) int {
	message = strings.TrimSpace(message)
	if r == nil || message == "" {
		return 0
	}

	noticeRaw := chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
		Type:    chatproto.ChatTypeNotice,
		VID:     0,
		Empire:  0,
		Message: message,
	})

	r.mu.Lock()
	defer r.mu.Unlock()

	delivered := 0
	for _, target := range r.scopesLocked().ConnectedTargets() {
		if characterAtBootstrapHPFloor(target.Character) {
			continue
		}
		if r.enqueueToEntityLocked(target.Entity.ID, [][]byte{noticeRaw}) {
			delivered++
		}
	}
	return delivered
}

func (r *sharedWorldRegistry) snapshotCharacters() []loginticket.Character {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotCharactersLocked()
}

func (r *sharedWorldRegistry) snapshotCharactersLocked() []loginticket.Character {
	if r == nil || r.entities == nil {
		return nil
	}
	return r.entities.PlayerCharacters()
}

func (r *sharedWorldRegistry) playerEntity(id uint64) (worldruntime.PlayerEntity, bool) {
	if r == nil || r.entities == nil {
		return worldruntime.PlayerEntity{}, false
	}
	return r.entities.Player(id)
}

func (r *sharedWorldRegistry) playerCharacter(id uint64) (loginticket.Character, bool) {
	playerEntity, ok := r.playerEntity(id)
	if !ok {
		return loginticket.Character{}, false
	}
	return playerEntity.Character, true
}

func (r *sharedWorldRegistry) playerEntityByName(name string) (worldruntime.PlayerEntity, bool) {
	if r == nil || r.entities == nil || name == "" {
		return worldruntime.PlayerEntity{}, false
	}
	return r.entities.PlayerByName(name)
}

func (r *sharedWorldRegistry) playerCharacterByName(name string) (loginticket.Character, bool) {
	playerEntity, ok := r.playerEntityByName(name)
	if !ok {
		return loginticket.Character{}, false
	}
	return playerEntity.Character, true
}

func characterAtBootstrapHPFloor(character loginticket.Character) bool {
	return character.Points[bootstrapPlayerPointValueIndex] <= 0
}

func connectedCharacterSnapshot(topology worldruntime.BootstrapTopology, character loginticket.Character) ConnectedCharacterSnapshot {
	return ConnectedCharacterSnapshot{
		Name:     character.Name,
		VID:      character.VID,
		MapIndex: topology.EffectiveMapIndex(character),
		X:        character.X,
		Y:        character.Y,
		Empire:   character.Empire,
		GuildID:  character.GuildID,
	}
}

func staticActorSnapshot(topology worldruntime.BootstrapTopology, actor worldruntime.StaticEntity) StaticActorSnapshot {
	combatProfile := actor.CombatProfile
	if combatProfile == "" {
		combatProfile = actor.CombatKind
	}
	var combatMaxHP uint8
	var combatNormalDamage uint8
	var combatAttackValue uint16
	var combatDefenseValue uint16
	var combatLevel uint16
	var combatRank uint8
	var retaliationPointDelta int32
	if defaults, ok := worldruntime.BootstrapStaticActorCombatProfileDefaults(combatProfile); ok {
		combatMaxHP = defaults.MaxHP
		combatNormalDamage = defaults.DamagePerNormalAttack
		combatAttackValue = defaults.AttackValue
		combatDefenseValue = defaults.DefenseValue
		combatLevel = defaults.Level
		combatRank = defaults.Rank
		if defaults.RetaliationPointDelta != worldruntime.PracticeMobBootstrapRetaliationPointDelta {
			retaliationPointDelta = defaults.RetaliationPointDelta
		}
	}
	snapshot := StaticActorSnapshot{
		EntityID:              actor.Entity.ID,
		Name:                  actor.Entity.Name,
		MapIndex:              topology.EffectiveMapIndex(loginticket.Character{MapIndex: actor.Position.MapIndex}),
		X:                     actor.Position.X,
		Y:                     actor.Position.Y,
		RaceNum:               actor.RaceNum,
		CombatProfile:         combatProfile,
		CombatMaxHP:           combatMaxHP,
		CombatNormalDamage:    combatNormalDamage,
		CombatAttackValue:     combatAttackValue,
		CombatDefenseValue:    combatDefenseValue,
		CombatLevel:           combatLevel,
		CombatRank:            combatRank,
		RetaliationPointDelta: retaliationPointDelta,
		InteractionKind:       actor.InteractionKind,
		InteractionRef:        actor.InteractionRef,
		SpawnGroupRef:         actor.SpawnGroupRef,
		RewardExperience:      actor.DeathReward.Experience,
		RewardGold:            actor.DeathReward.Gold,
		RewardDropVnums:       actor.DeathReward.Clone().DropVnums,
	}
	if leash, ok := worldruntime.EvaluateStaticActorCurrentSpawnLeash(actor, worldruntime.EffectiveStaticActorSpawnLeashRadiusForActor(actor)); ok {
		leashSnapshot := worldruntime.SpawnLeashSnapshotFromEvaluation(leash)
		spawnHome := leashSnapshot.Home
		snapshot.SpawnHome = &spawnHome
		snapshot.SpawnLeash = &leashSnapshot
	}
	return snapshot
}

func connectedCharacterSnapshots(topology worldruntime.BootstrapTopology, characters []loginticket.Character) []ConnectedCharacterSnapshot {
	snapshots := make([]ConnectedCharacterSnapshot, 0, len(characters))
	for _, character := range characters {
		snapshots = append(snapshots, connectedCharacterSnapshot(topology, character))
	}
	sortConnectedCharacterSnapshots(snapshots)
	return snapshots
}

func staticActorSnapshots(topology worldruntime.BootstrapTopology, actors []worldruntime.StaticEntity) []StaticActorSnapshot {
	snapshots := make([]StaticActorSnapshot, 0, len(actors))
	for _, actor := range actors {
		snapshots = append(snapshots, staticActorSnapshot(topology, actor))
	}
	sortStaticActorSnapshots(snapshots)
	return snapshots
}

func (r *sharedWorldRegistry) mapOccupancySnapshots() []MapOccupancySnapshot {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.mapOccupancySnapshotsLocked()
}

func (r *sharedWorldRegistry) GroundItems() []GroundItemSnapshot {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	snapshots := make([]GroundItemSnapshot, 0, len(r.groundItemsByVID))
	for _, ground := range r.groundItemsByVID {
		snapshots = append(snapshots, groundItemSnapshot(ground))
	}
	sortGroundItemSnapshots(snapshots)
	return snapshots
}

func (r *sharedWorldRegistry) GroundItem(vid uint32) (GroundItemSnapshot, bool) {
	if r == nil || vid == 0 {
		return GroundItemSnapshot{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	ground, ok := r.groundItemsByVID[vid]
	if !ok {
		return GroundItemSnapshot{}, false
	}
	return groundItemSnapshot(ground), true
}

func (r *sharedWorldRegistry) GroundItemsForMap(mapIndex uint32) ([]GroundItemSnapshot, bool) {
	if r == nil || mapIndex == 0 {
		return nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, snapshot := range r.mapOccupancySnapshotsLocked() {
		if snapshot.MapIndex != mapIndex {
			continue
		}
		if len(snapshot.GroundItems) == 0 {
			return []GroundItemSnapshot{}, true
		}
		groundItems := append([]GroundItemSnapshot(nil), snapshot.GroundItems...)
		sortGroundItemSnapshots(groundItems)
		return groundItems, true
	}
	return nil, false
}

func (r *sharedWorldRegistry) mapOccupancySnapshotsLocked() []MapOccupancySnapshot {
	if r == nil || r.entities == nil {
		return nil
	}
	return r.markMapOccupancySnapshotStateLocked(r.scopesLocked().MapOccupancySnapshots())
}

func appendGroundItemsToMapOccupancySnapshots(topology worldruntime.BootstrapTopology, snapshots []MapOccupancySnapshot, groundItems map[uint32]sharedGroundItem) []MapOccupancySnapshot {
	groundOccupancy := make([]worldruntime.GroundItemOccupancy, 0, len(groundItems))
	for _, ground := range groundItems {
		groundOccupancy = append(groundOccupancy, sharedGroundItemOccupancy(ground))
	}
	return worldruntime.AppendGroundItemsToMapOccupancySnapshots(topology, snapshots, groundOccupancy)
}

func (r *sharedWorldRegistry) staticActorDeadLocked(entityID uint64) bool {
	if r == nil || entityID == 0 || r.staticActorCombatHP == nil {
		return false
	}
	currentHP, ok := r.staticActorCombatHP[entityID]
	return ok && currentHP == 0
}

func (r *sharedWorldRegistry) markStaticActorSnapshotStateLocked(snapshot StaticActorSnapshot) StaticActorSnapshot {
	if snapshot.EntityID == 0 {
		return snapshot
	}
	if credit := r.staticActorKillQuestCreditLocked(snapshot.EntityID); !credit.Empty() {
		snapshot.RewardQuestRef = credit.QuestRef
		snapshot.RewardQuestFlag = credit.QuestFlag
		snapshot.RewardQuestFrom = credit.QuestFrom
		snapshot.RewardQuestTo = credit.QuestTo
		snapshot.RewardQuestText = credit.Text
		snapshot.RequireQuestRef = credit.RequireQuestRef
		snapshot.RequireQuestFlag = credit.RequireQuestFlag
		snapshot.RequireQuestFrom = credit.RequireQuestFrom
	}
	if r == nil || r.staticActorCombatHP == nil {
		return snapshot
	}
	currentHP, ok := r.staticActorCombatHP[snapshot.EntityID]
	if !ok {
		return snapshot
	}
	snapshot.Dead = currentHP == 0
	if snapshot.CombatProfile != "" {
		if hpPercent, ok := worldruntime.BootstrapStaticActorHPPercent(snapshot.CombatProfile, currentHP); ok {
			snapshot.CombatHPPercent = hpPercent
		}
	}
	return snapshot
}

func (r *sharedWorldRegistry) markStaticActorSnapshotsStateLocked(snapshots []StaticActorSnapshot) []StaticActorSnapshot {
	for i := range snapshots {
		snapshots[i] = r.markStaticActorSnapshotStateLocked(snapshots[i])
	}
	return snapshots
}

func (r *sharedWorldRegistry) markCharacterVisibilityStaticActorStateLocked(snapshots []CharacterVisibilitySnapshot) []CharacterVisibilitySnapshot {
	for i := range snapshots {
		snapshots[i].VisibleStaticActors = r.markStaticActorSnapshotsStateLocked(snapshots[i].VisibleStaticActors)
		snapshots[i].VisibleSpawnGroups = r.markStaticActorSnapshotsStateLocked(snapshots[i].VisibleSpawnGroups)
	}
	return snapshots
}

func (r *sharedWorldRegistry) markCharacterInteractionVisibilityStaticActorStateLocked(snapshots []worldruntime.CharacterInteractionVisibilitySnapshot) []worldruntime.CharacterInteractionVisibilitySnapshot {
	for i := range snapshots {
		snapshots[i].VisibleInteractableStaticActors = r.markStaticActorSnapshotsStateLocked(snapshots[i].VisibleInteractableStaticActors)
	}
	return snapshots
}

func (r *sharedWorldRegistry) markMapOccupancyStaticActorStateLocked(snapshots []MapOccupancySnapshot) []MapOccupancySnapshot {
	for i := range snapshots {
		snapshots[i].StaticActors = r.markStaticActorSnapshotsStateLocked(snapshots[i].StaticActors)
		snapshots[i].SpawnGroups = r.markStaticActorSnapshotsStateLocked(snapshots[i].SpawnGroups)
		snapshots[i].SpawnGroupCount = len(snapshots[i].SpawnGroups)
	}
	return snapshots
}

func (r *sharedWorldRegistry) markRelocationPreviewStateLocked(preview RelocationPreview) RelocationPreview {
	preview.CurrentVisibleStaticActors = r.markStaticActorSnapshotsStateLocked(preview.CurrentVisibleStaticActors)
	preview.TargetVisibleStaticActors = r.markStaticActorSnapshotsStateLocked(preview.TargetVisibleStaticActors)
	preview.RemovedVisibleStaticActors = r.markStaticActorSnapshotsStateLocked(preview.RemovedVisibleStaticActors)
	preview.AddedVisibleStaticActors = r.markStaticActorSnapshotsStateLocked(preview.AddedVisibleStaticActors)
	preview.CurrentVisibleSpawnGroups = r.markStaticActorSnapshotsStateLocked(preview.CurrentVisibleSpawnGroups)
	preview.TargetVisibleSpawnGroups = r.markStaticActorSnapshotsStateLocked(preview.TargetVisibleSpawnGroups)
	preview.RemovedVisibleSpawnGroups = r.markStaticActorSnapshotsStateLocked(preview.RemovedVisibleSpawnGroups)
	preview.AddedVisibleSpawnGroups = r.markStaticActorSnapshotsStateLocked(preview.AddedVisibleSpawnGroups)
	preview.BeforeMapOccupancy = r.markMapOccupancyStaticActorStateLocked(preview.BeforeMapOccupancy)
	preview.AfterMapOccupancy = r.markMapOccupancyStaticActorStateLocked(preview.AfterMapOccupancy)
	return preview
}

func (r *sharedWorldRegistry) markMapOccupancySnapshotStateLocked(snapshots []MapOccupancySnapshot) []MapOccupancySnapshot {
	snapshots = r.markMapOccupancyStaticActorStateLocked(snapshots)
	if len(r.groundItemsByVID) == 0 {
		return snapshots
	}
	return appendGroundItemsToMapOccupancySnapshots(r.topology, snapshots, r.groundItemsByVID)
}

func buildTransferOriginFrames(removed []loginticket.Character, added []loginticket.Character) [][]byte {
	return buildTransferOriginFramesWithTemplates(removed, added, nil)
}

func buildTransferOriginFramesWithTemplates(removed []loginticket.Character, added []loginticket.Character, templates map[uint32]itemcatalog.Template) [][]byte {
	frames := make([][]byte, 0, len(removed)+len(added)*4)
	for _, peerCharacter := range removed {
		frames = append(frames, encodeCharacterDeleteFrame(peerCharacter))
	}
	for _, peerCharacter := range added {
		frames = append(frames, encodePeerVisibilityBootstrapFramesWithTemplates(peerCharacter, templates)...)
	}
	return frames
}

func sortConnectedCharacterSnapshots(snapshots []ConnectedCharacterSnapshot) {
	sort.Slice(snapshots, func(i int, j int) bool {
		if snapshots[i].Name == snapshots[j].Name {
			return snapshots[i].VID < snapshots[j].VID
		}
		return snapshots[i].Name < snapshots[j].Name
	})
}

func sortMapOccupancySnapshots(snapshots []MapOccupancySnapshot) {
	sort.Slice(snapshots, func(i int, j int) bool {
		return snapshots[i].MapIndex < snapshots[j].MapIndex
	})
}

func sortStaticActorSnapshots(snapshots []StaticActorSnapshot) {
	sort.Slice(snapshots, func(i int, j int) bool {
		if snapshots[i].Name == snapshots[j].Name {
			return snapshots[i].EntityID < snapshots[j].EntityID
		}
		return snapshots[i].Name < snapshots[j].Name
	})
}

func sortGroundItemSnapshots(snapshots []GroundItemSnapshot) {
	sort.Slice(snapshots, func(i int, j int) bool {
		return snapshots[i].VID < snapshots[j].VID
	})
}

func encodeCharacterDeleteFrame(character loginticket.Character) []byte {
	return worldproto.EncodeCharacterDeleteNotice(worldproto.CharacterDeleteNoticePacket{VID: character.VID})
}

func encodeExchangeStartFrame(peerVID uint32) []byte {
	return itemproto.EncodeServerExchange(itemproto.ServerExchangePacket{Subheader: itemproto.ExchangeServerSubheaderStart, Arg1: peerVID})
}

func encodeExchangeAlreadyFrame() []byte {
	return itemproto.EncodeServerExchange(itemproto.ServerExchangePacket{Subheader: itemproto.ExchangeServerSubheaderAlready})
}

func encodeExchangePartnerMerchantBusyInfoFrame() []byte {
	return chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
		Type:    chatproto.ChatTypeInfo,
		VID:     0,
		Empire:  0,
		Message: exchangePartnerMerchantBusyInfoMessage,
	})
}

func encodeExchangeRequesterMerchantBusyInfoFrame() []byte {
	return chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
		Type:    chatproto.ChatTypeInfo,
		VID:     0,
		Empire:  0,
		Message: exchangeRequesterMerchantBusyInfoMessage,
	})
}

func encodeExchangeRequesterGoldCarrierCapInfoFrame() []byte {
	return chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
		Type:    chatproto.ChatTypeInfo,
		VID:     0,
		Empire:  0,
		Message: exchangeRequesterGoldCarrierCapInfoMessage,
	})
}

func encodeExchangePartnerGoldCarrierCapInfoFrame() []byte {
	return chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
		Type:    chatproto.ChatTypeInfo,
		VID:     0,
		Empire:  0,
		Message: exchangePartnerGoldCarrierCapInfoMessage,
	})
}

func encodeExchangeFinalizeCheckSelfInfoFrame() []byte {
	return chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
		Type:    chatproto.ChatTypeInfo,
		VID:     0,
		Empire:  0,
		Message: exchangeFinalizeCheckSelfInfoMessage,
	})
}

func encodeExchangeFinalizeCheckOtherInfoFrame() []byte {
	return chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
		Type:    chatproto.ChatTypeInfo,
		VID:     0,
		Empire:  0,
		Message: exchangeFinalizeCheckOtherInfoMessage,
	})
}

func encodeExchangeFinalizeSpaceSelfInfoFrame() []byte {
	return chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
		Type:    chatproto.ChatTypeInfo,
		VID:     0,
		Empire:  0,
		Message: exchangeFinalizeSpaceSelfInfoMessage,
	})
}

func encodeExchangeFinalizeSpaceOtherInfoFrame() []byte {
	return chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
		Type:    chatproto.ChatTypeInfo,
		VID:     0,
		Empire:  0,
		Message: exchangeFinalizeSpaceOtherInfoMessage,
	})
}

func encodeExchangeFinalizeGoldOverflowSelfInfoFrame() []byte {
	return chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
		Type:    chatproto.ChatTypeInfo,
		VID:     0,
		Empire:  0,
		Message: exchangeFinalizeGoldOverflowSelfInfoMessage,
	})
}

func encodeExchangeFinalizeGoldOverflowOtherInfoFrame() []byte {
	return chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
		Type:    chatproto.ChatTypeInfo,
		VID:     0,
		Empire:  0,
		Message: exchangeFinalizeGoldOverflowOtherInfoMessage,
	})
}

func encodeExchangeFinalizeOtherInfoFrame() []byte {
	return chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
		Type:    chatproto.ChatTypeInfo,
		VID:     0,
		Empire:  0,
		Message: exchangeFinalizeOtherInfoMessage,
	})
}

func encodeExchangeFinalizeSuccessInfoFrame(partnerName string) []byte {
	return chatproto.EncodeChatDelivery(chatproto.ChatDeliveryPacket{
		Type:    chatproto.ChatTypeInfo,
		VID:     0,
		Empire:  0,
		Message: exchangeFinalizeSuccessInfoMessage(partnerName),
	})
}

func exchangeFinalizeSuccessInfoMessage(partnerName string) string {
	return fmt.Sprintf(exchangeFinalizeSuccessInfoMessageFormat, normalizeLiveCharacterName(partnerName))
}

func encodeExchangeEndFrame() []byte {
	return itemproto.EncodeServerExchange(itemproto.ServerExchangePacket{Subheader: itemproto.ExchangeServerSubheaderEnd})
}

func encodeExchangeItemDelFrame(isMe uint8, displaySlot uint8) []byte {
	return itemproto.EncodeServerExchange(itemproto.ServerExchangePacket{
		Subheader: itemproto.ExchangeServerSubheaderItemDel,
		IsMe:      isMe,
		Arg1:      uint32(displaySlot),
	})
}

func encodeExchangeGoldAddFrame(isMe uint8, gold uint32) []byte {
	return itemproto.EncodeServerExchange(itemproto.ServerExchangePacket{
		Subheader: itemproto.ExchangeServerSubheaderGoldAdd,
		IsMe:      isMe,
		Arg1:      gold,
	})
}

func encodeExchangeAcceptFrame(isMe uint8) []byte {
	return encodeExchangeAcceptFrameWithValue(isMe, 1)
}

func encodeExchangeAcceptFrameWithValue(isMe uint8, value uint32) []byte {
	return itemproto.EncodeServerExchange(itemproto.ServerExchangePacket{
		Subheader: itemproto.ExchangeServerSubheaderAccept,
		IsMe:      isMe,
		Arg1:      value,
	})
}

func encodeExchangeLessGoldFrame() []byte {
	return itemproto.EncodeServerExchange(itemproto.ServerExchangePacket{Subheader: itemproto.ExchangeServerSubheaderLessGold})
}

func encodeExchangeItemAddFrame(isMe uint8, displaySlot uint8, display player.ExchangeItemAddDisplay) []byte {
	return itemproto.EncodeServerExchange(itemproto.ServerExchangePacket{
		Subheader:  itemproto.ExchangeServerSubheaderItemAdd,
		IsMe:       isMe,
		Arg1:       display.Item.Vnum,
		Position:   itemproto.Position{WindowType: itemproto.WindowReserved, Cell: uint16(displaySlot)},
		Arg3:       uint32(display.Item.Count),
		Sockets:    [itemproto.ItemSocketCount]int32(display.Sockets),
		Attributes: exchangeItemDisplayAttributes(display.Attributes),
	})
}

func exchangeItemDisplayAttributes(attributes itemcatalog.AttributeValues) [itemproto.ItemAttributeCount]itemproto.Attribute {
	var out [itemproto.ItemAttributeCount]itemproto.Attribute
	for i, attribute := range attributes {
		out[i] = itemproto.Attribute{Type: attribute.Type, Value: attribute.Value}
	}
	return out
}

func encodeStaticActorVisibilityFrames(actor worldruntime.StaticEntity) [][]byte {
	vid, ok := staticActorVisibilityVID(actor)
	if !ok {
		return nil
	}
	infoRaw, err := worldproto.EncodeCharacterAdditionalInfo(staticActorCharacterAdditionalInfoPacket(actor, vid))
	if err != nil {
		return nil
	}
	return [][]byte{
		worldproto.EncodeCharacterAdd(staticActorCharacterAddPacket(actor, vid)),
		infoRaw,
		worldproto.EncodeCharacterUpdate(staticActorCharacterUpdatePacket(actor, vid)),
	}
}

func (r *sharedWorldRegistry) encodeStaticActorVisibilityStateFramesLocked(actor worldruntime.StaticEntity) [][]byte {
	frames := encodeStaticActorVisibilityFrames(actor)
	if r == nil || len(frames) == 0 {
		return frames
	}
	currentHP, ok := r.ensureStaticActorCombatCurrentHPLocked(actor)
	if !ok || currentHP > 0 {
		return frames
	}
	vid, encodable := staticActorVisibilityVID(actor)
	if !encodable {
		return frames
	}
	return append(frames, worldproto.EncodeDead(worldproto.DeadPacket{VID: vid}))
}

func (r *sharedWorldRegistry) buildStaticActorVisibilityTransitionFramesLocked(removed []worldruntime.StaticEntity, added []worldruntime.StaticEntity) [][]byte {
	frames := make([][]byte, 0, len(removed)+len(added)*4)
	for _, actor := range removed {
		deleteRaw, ok := encodeStaticActorDeleteFrame(actor)
		if !ok {
			continue
		}
		frames = append(frames, deleteRaw)
	}
	for _, actor := range added {
		frames = append(frames, r.encodeStaticActorVisibilityStateFramesLocked(actor)...)
	}
	return frames
}

func (r *sharedWorldRegistry) buildStaticActorRefreshFramesLocked(previous worldruntime.StaticEntity, updated worldruntime.StaticEntity) [][]byte {
	deleteRaw, ok := encodeStaticActorDeleteFrame(previous)
	if !ok {
		return nil
	}
	addFrames := r.encodeStaticActorVisibilityStateFramesLocked(updated)
	if len(addFrames) == 0 {
		return nil
	}
	frames := make([][]byte, 0, 1+len(addFrames))
	frames = append(frames, deleteRaw)
	frames = append(frames, addFrames...)
	return frames
}

func encodeStaticActorDeleteFrame(actor worldruntime.StaticEntity) ([]byte, bool) {
	vid, ok := staticActorVisibilityVID(actor)
	if !ok {
		return nil, false
	}
	return worldproto.EncodeCharacterDeleteNotice(worldproto.CharacterDeleteNoticePacket{VID: vid}), true
}

func encodeStaticActorChaseMoveFrame(actor worldruntime.StaticEntity) ([]byte, bool) {
	vid, ok := staticActorVisibilityVID(actor)
	if !ok {
		return nil, false
	}
	return movep.EncodeMoveAck(movep.MoveAckPacket{
		Func:     bootstrapSpawnGroupChaseMoveFunc,
		Arg:      0,
		Rot:      0,
		VID:      vid,
		X:        actor.Position.X,
		Y:        actor.Position.Y,
		Time:     0,
		Duration: bootstrapSpawnGroupChaseMoveDuration,
	}), true
}

func staticActorVisibilityVID(actor worldruntime.StaticEntity) (uint32, bool) {
	return worldruntime.StaticActorVisibilityVID(actor)
}

func staticActorCharacterAddPacket(actor worldruntime.StaticEntity, vid uint32) worldproto.CharacterAddPacket {
	return worldproto.CharacterAddPacket{
		VID:         vid,
		Angle:       0,
		X:           actor.Position.X,
		Y:           actor.Position.Y,
		Z:           0,
		Type:        1,
		RaceNum:     uint16(actor.RaceNum),
		MovingSpeed: 150,
		AttackSpeed: 100,
		StateFlag:   0,
		AffectFlags: [worldproto.AffectFlagCount]uint32{},
	}
}

func staticActorCharacterAdditionalInfoPacket(actor worldruntime.StaticEntity, vid uint32) worldproto.CharacterAdditionalInfoPacket {
	return worldproto.CharacterAdditionalInfoPacket{
		VID:       vid,
		Name:      actor.Entity.Name,
		Parts:     [worldproto.CharacterEquipmentPartCount]uint16{},
		Empire:    0,
		GuildID:   0,
		Level:     staticActorCharacterAdditionalInfoLevel(actor),
		Alignment: 0,
		PKMode:    0,
		MountVnum: 0,
	}
}

func staticActorCharacterAdditionalInfoLevel(actor worldruntime.StaticEntity) uint32 {
	combatProfile := actor.CombatProfile
	if combatProfile == "" {
		combatProfile = actor.CombatKind
	}
	defaults, ok := worldruntime.BootstrapStaticActorCombatProfileDefaults(combatProfile)
	if !ok {
		return 0
	}
	return uint32(defaults.Level)
}

func staticActorCharacterUpdatePacket(actor worldruntime.StaticEntity, vid uint32) worldproto.CharacterUpdatePacket {
	return worldproto.CharacterUpdatePacket{
		VID:         vid,
		Parts:       [worldproto.CharacterEquipmentPartCount]uint16{},
		MovingSpeed: 150,
		AttackSpeed: 100,
		StateFlag:   0,
		AffectFlags: [worldproto.AffectFlagCount]uint32{},
		GuildID:     0,
		Alignment:   0,
		PKMode:      0,
		MountVnum:   0,
	}
}

func encodePeerVisibilityFrames(character loginticket.Character) [][]byte {
	return encodePeerVisibilityFramesWithTemplates(character, nil)
}

func encodePeerVisibilityFramesWithTemplates(character loginticket.Character, templates map[uint32]itemcatalog.Template) [][]byte {
	infoRaw, err := worldproto.EncodeCharacterAdditionalInfo(ticketCharacterAdditionalInfoPacketWithTemplates(character, templates))
	if err != nil {
		return nil
	}
	return [][]byte{
		worldproto.EncodeCharacterAdd(ticketCharacterAddPacket(character)),
		infoRaw,
		worldproto.EncodeCharacterUpdate(ticketCharacterUpdatePacketWithTemplates(character, templates)),
	}
}

func encodePeerVisibilityBootstrapFrames(character loginticket.Character) [][]byte {
	return encodePeerVisibilityBootstrapFramesWithTemplates(character, nil)
}

func encodePeerVisibilityBootstrapFramesWithTemplates(character loginticket.Character, templates map[uint32]itemcatalog.Template) [][]byte {
	frames := encodePeerVisibilityFramesWithTemplates(character, templates)
	if !characterAtBootstrapHPFloor(character) {
		return frames
	}
	return append(frames, worldproto.EncodeDead(worldproto.DeadPacket{VID: character.VID}))
}
