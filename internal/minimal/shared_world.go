package minimal

import (
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
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
	mu                              sync.Mutex
	topology                        worldruntime.BootstrapTopology
	entities                        *worldruntime.EntityRegistry
	sessionDirectory                *worldruntime.SessionDirectory
	staticActorCombatHP             map[uint64]uint8
	staticActorCombatRespawnAt      map[uint64]time.Time
	staticActorCombatSnapshot       map[uint64]uint64
	staticActorCombatEngagedBy      map[uint64]uint64
	staticActorDeathReward          map[uint64]worldruntime.StaticActorDeathReward
	sessionCombatTargets            map[uint64]uint32
	nextStaticActorCombatSnapshotID uint64
	lastKnownCharacters             map[uint64]loginticket.Character
	groundItemsByVID                map[uint32]sharedGroundItem
	now                             func() time.Time
}

type sharedGroundItem struct {
	VID              uint32
	OwnerID          uint64
	OwnerLogin       string
	OwnerCharacterID uint32
	OwnerVID         uint32
	OwnerName        string
	Item             inventory.ItemInstance
	GoldAmount       uint32
	MapIndex         uint32
	X                int32
	Y                int32
	Z                int32
}

type sharedGroundItemPickup struct {
	Item       inventory.ItemInstance
	GoldAmount uint32
	OwnerID    uint64
	OwnerLogin string
	OwnerName  string
	Owner      loginticket.Character
}

type sharedGroundItemVisibilityDiff struct {
	Removed []sharedGroundItem
	Added   []sharedGroundItem
}

const (
	StaticActorInteractionFailureSubjectNotFound        = "subject_not_found"
	StaticActorInteractionFailureSubjectDead            = "subject_dead"
	StaticActorInteractionFailureTargetNotVisible       = "target_not_visible"
	StaticActorInteractionFailureTargetOutOfRange       = "target_out_of_range"
	StaticActorInteractionFailureTargetHasNoInteraction = "target_has_no_interaction"

	StaticActorCombatTargetFailureSubjectNotFound     = "subject_not_found"
	StaticActorCombatTargetFailureSubjectDead         = "subject_dead"
	StaticActorCombatTargetFailureTargetNotVisible    = "target_not_visible"
	StaticActorCombatTargetFailureTargetOutOfRange    = "target_out_of_range"
	StaticActorCombatTargetFailureTargetNotTargetable = "target_not_targetable"
	StaticActorCombatTargetFailureTargetDead          = "target_dead"

	StaticActorCombatAttackFailureSubjectNotFound        = "subject_not_found"
	StaticActorCombatAttackFailureSubjectDead            = "subject_dead"
	StaticActorCombatAttackFailureNoActiveTarget         = "no_active_target"
	StaticActorCombatAttackFailureTargetMismatch         = "target_mismatch"
	StaticActorCombatAttackFailureTargetNotVisible       = "target_not_visible"
	StaticActorCombatAttackFailureTargetOutOfRange       = "target_out_of_range"
	StaticActorCombatAttackFailureTargetNotTargetable    = "target_not_targetable"
	StaticActorCombatAttackFailureTargetDead             = "target_dead"
	StaticActorCombatAttackFailureTargetSnapshotMismatch = "target_snapshot_mismatch"
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
	SubjectEntityID uint64                     `json:"subject_entity_id"`
	Subject         ConnectedCharacterSnapshot `json:"subject"`
	TargetVID       uint32                     `json:"target_vid"`
	SnapshotVersion uint64                     `json:"snapshot_version"`
	HPPercent       uint8                      `json:"hp_percent"`
	Actor           StaticActorSnapshot        `json:"actor"`
}

type StaticActorCombatAttackAttempt struct {
	Accepted                    bool
	Failure                     string
	ActiveTargetVID             uint32
	ActiveTargetSnapshotVersion uint64
	RequestedTargetVID          uint32
	HPPercent                   uint8
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
		topology:                   topology,
		entities:                   worldruntime.NewEntityRegistryWithTopology(topology),
		sessionDirectory:           worldruntime.NewSessionDirectory(),
		staticActorCombatHP:        make(map[uint64]uint8),
		staticActorCombatRespawnAt: make(map[uint64]time.Time),
		staticActorCombatSnapshot:  make(map[uint64]uint64),
		staticActorCombatEngagedBy: make(map[uint64]uint64),
		staticActorDeathReward:     make(map[uint64]worldruntime.StaticActorDeathReward),
		lastKnownCharacters:        make(map[uint64]loginticket.Character),
		groundItemsByVID:           make(map[uint32]sharedGroundItem),
		now:                        time.Now,
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
	if r.staticActorCombatEngagedBy != nil {
		delete(r.staticActorCombatEngagedBy, entityID)
	}
	if r.staticActorDeathReward != nil {
		delete(r.staticActorDeathReward, entityID)
	}
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

func (r *sharedWorldRegistry) clearStaticActorCombatEngagementsBySubjectLocked(subjectID uint64) {
	if r == nil || subjectID == 0 || len(r.staticActorCombatEngagedBy) == 0 {
		return
	}
	for entityID, engagedBy := range r.staticActorCombatEngagedBy {
		if engagedBy != subjectID {
			continue
		}
		delete(r.staticActorCombatEngagedBy, entityID)
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

func (r *sharedWorldRegistry) flushReadyStaticActorRespawnLocked(entityID uint64) {
	if r == nil || r.entities == nil || entityID == 0 {
		return
	}
	if r.staticActorCombatRespawnAt != nil {
		delete(r.staticActorCombatRespawnAt, entityID)
	}
	actor, ok := r.entities.StaticActor(entityID)
	if !ok || actor.CombatKind == "" {
		return
	}
	currentHP, ok := r.ensureStaticActorCombatCurrentHPLocked(actor)
	if !ok || currentHP > 0 {
		return
	}
	resetHP, ok := worldruntime.BootstrapStaticActorCurrentHP(actor.CombatKind)
	if !ok {
		return
	}
	if r.staticActorCombatHP == nil {
		r.staticActorCombatHP = make(map[uint64]uint8)
	}
	r.staticActorCombatHP[entityID] = resetHP
	r.assignStaticActorCombatSnapshotLocked(entityID)
	deleteRaw, encodable := encodeStaticActorDeleteFrame(actor)
	if !encodable {
		return
	}
	addFrames := encodeStaticActorVisibilityFrames(actor)
	if len(addFrames) == 0 {
		return
	}
	frames := make([][]byte, 0, 1+len(addFrames))
	frames = append(frames, deleteRaw)
	frames = append(frames, addFrames...)
	for _, target := range r.scopesLocked().VisibleTargetsForStaticActor(actor) {
		if characterAtBootstrapHPFloor(target.Character) {
			continue
		}
		r.enqueueToEntityLocked(target.Entity.ID, frames)
	}
}

func (r *sharedWorldRegistry) clearSessionCombatTargetLocked(entityID uint64) {
	if r == nil || entityID == 0 || r.sessionCombatTargets == nil {
		return
	}
	delete(r.sessionCombatTargets, entityID)
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
		r.clearSessionCombatTargetLocked(engagedBy)
		return false
	}
	if _, ok := r.playerCharacter(engagedBy); !ok {
		delete(r.staticActorCombatEngagedBy, actor.Entity.ID)
		r.clearSessionCombatTargetLocked(engagedBy)
		return false
	}
	return true
}

func staticActorSpawnGroupAggroLiteCombatKind(combatKind string) bool {
	_, ok := worldruntime.BootstrapStaticActorCombatProfileDefaults(combatKind)
	return ok
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
	currentHP, ok := r.ensureStaticActorCombatCurrentHPLocked(actor)
	if !ok {
		return CombatTargetSnapshot{}, false
	}
	hpPercent, ok := worldruntime.BootstrapStaticActorHPPercent(actor.CombatKind, currentHP)
	if !ok {
		return CombatTargetSnapshot{}, false
	}
	actorSnapshot := staticActorSnapshot(r.topology, actor)
	actorSnapshot.Dead = currentHP == 0
	return CombatTargetSnapshot{
		SubjectEntityID: entityID,
		Subject:         worldruntime.ConnectedCharacterSnapshotFor(r.topology, subject),
		TargetVID:       targetVID,
		SnapshotVersion: r.staticActorCombatSnapshot[actor.Entity.ID],
		HPPercent:       hpPercent,
		Actor:           actorSnapshot,
	}, true
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

func (r *sharedWorldRegistry) removeStaleOwnershipLocked(entityIDs []uint64) {
	if r == nil || len(entityIDs) == 0 {
		return
	}
	for _, entityID := range entityIDs {
		currentCharacter, ok := r.playerCharacter(entityID)
		if !ok {
			currentCharacter, ok = r.lastKnownCharacters[entityID]
		}
		if r.sessionDirectory != nil {
			_, _ = r.sessionDirectory.Remove(entityID)
		}
		r.clearSessionCombatTargetLocked(entityID)
		r.clearStaticActorCombatEngagementsBySubjectLocked(entityID)
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
		r.removeOwnedGroundItemsLocked(entityID, r.visiblePeersForOwnedGroundItemsLocked(entityID, visibilityDiff.RemovedVisiblePeers))
	}
}

func (r *sharedWorldRegistry) Join(character loginticket.Character, pending *pendingServerFrames, relocate sharedWorldSessionRelocator) (uint64, []loginticket.Character) {
	if r == nil {
		return 0, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	staleIDs, liveConflict := r.reclaimableStaleDuplicateIDsLocked(character)
	if liveConflict {
		return 0, nil
	}
	r.removeStaleOwnershipLocked(staleIDs)

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

	peerFrames := encodePeerVisibilityBootstrapFrames(character)
	for _, peerCharacter := range visibilityDiff.AddedVisiblePeers {
		if characterAtBootstrapHPFloor(peerCharacter) {
			continue
		}
		r.enqueueToCharacterLocked(peerCharacter, peerFrames)
	}
	return id, visibilityDiff.TargetVisiblePeers
}

func (r *sharedWorldRegistry) Leave(id uint64) {
	if r == nil || id == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	currentCharacter, ok := r.playerCharacter(id)
	if !ok {
		currentCharacter, ok = r.lastKnownCharacters[id]
	}
	if r.sessionDirectory != nil {
		_, _ = r.sessionDirectory.Remove(id)
	}
	r.clearSessionCombatTargetLocked(id)
	r.clearStaticActorCombatEngagementsBySubjectLocked(id)
	_, _ = r.entities.Remove(id)
	delete(r.lastKnownCharacters, id)
	if !ok {
		return
	}

	visibilityDiff := r.scopesLocked().LeaveVisibilityDiff(currentCharacter)
	removeRaw := encodeCharacterDeleteFrame(currentCharacter)
	for _, peerCharacter := range visibilityDiff.RemovedVisiblePeers {
		if characterAtBootstrapHPFloor(peerCharacter) {
			continue
		}
		r.enqueueToCharacterLocked(peerCharacter, [][]byte{removeRaw})
	}
	r.removeOwnedGroundItemsLocked(id, r.visiblePeersForOwnedGroundItemsLocked(id, visibilityDiff.RemovedVisiblePeers))
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

func (r *sharedWorldRegistry) removeOwnedGroundItemsLocked(ownerID uint64, visiblePeers []loginticket.Character) {
	if ownerID == 0 || len(r.groundItemsByVID) == 0 {
		return
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
		return
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
}

func (r *sharedWorldRegistry) RegisterGroundItem(ownerID uint64, ownerLogin string, character loginticket.Character, vid uint32, item inventory.ItemInstance) bool {
	const maxItemGetCountCarrier = uint16(^uint8(0))
	if item.ID == 0 || item.Vnum == 0 || item.Count == 0 || item.Count > maxItemGetCountCarrier || item.Locked || item.Equipped || item.EquipSlot != inventory.EquipmentSlotNone {
		return false
	}
	return r.registerGroundItem(ownerID, ownerLogin, character, vid, item, 0)
}

func (r *sharedWorldRegistry) RegisterGroundGold(ownerID uint64, ownerLogin string, character loginticket.Character, vid uint32, amount uint32) bool {
	const maxPointChangeCarrier = uint32(1<<31 - 1)
	if amount == 0 || amount > maxPointChangeCarrier {
		return false
	}
	return r.registerGroundItem(ownerID, ownerLogin, character, vid, inventory.ItemInstance{Vnum: 1, Count: 1}, amount)
}

func (r *sharedWorldRegistry) registerGroundItem(ownerID uint64, ownerLogin string, character loginticket.Character, vid uint32, item inventory.ItemInstance, goldAmount uint32) bool {
	if r == nil || ownerID == 0 || !validRewardOwnerMetadata(ownerLogin) || !validRewardOwnerMetadata(character.Name) || vid == 0 || item.Vnum == 0 {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	registeredOwner, ok := r.playerCharacter(ownerID)
	if !ok || characterAtBootstrapHPFloor(registeredOwner) || characterAtBootstrapHPFloor(character) || !sameGroundRewardOwnerLocation(registeredOwner, character) {
		return false
	}
	if _, exists := r.groundItemsByVID[vid]; exists {
		return false
	}
	ground := sharedGroundItem{
		VID:              vid,
		OwnerID:          ownerID,
		OwnerLogin:       ownerLogin,
		OwnerCharacterID: character.ID,
		OwnerVID:         character.VID,
		OwnerName:        character.Name,
		Item:             item,
		GoldAmount:       goldAmount,
		MapIndex:         r.topology.EffectiveMapIndex(character),
		X:                character.X,
		Y:                character.Y,
		Z:                character.Z,
	}
	r.groundItemsByVID[vid] = ground
	frames := encodeGroundItemVisibleFrames(ground)
	for _, target := range r.scopesLocked().VisibleTargets(ownerID, character) {
		if characterAtBootstrapHPFloor(target.Character) {
			continue
		}
		r.enqueueToEntityLocked(target.Entity.ID, frames)
	}
	return true
}

func validRewardOwnerMetadata(value string) bool {
	if value == "" || strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func sameGroundRewardOwnerLocation(registered loginticket.Character, supplied loginticket.Character) bool {
	return sameGroundRewardCharacterLocation(registered, supplied)
}

func sameGroundRewardCollectorLocation(registered loginticket.Character, supplied loginticket.Character) bool {
	return sameGroundRewardCharacterLocation(registered, supplied)
}

func sameGroundRewardCharacterLocation(registered loginticket.Character, supplied loginticket.Character) bool {
	return registered.ID == supplied.ID && registered.VID == supplied.VID && registered.Name == supplied.Name && registered.MapIndex == supplied.MapIndex && registered.X == supplied.X && registered.Y == supplied.Y && registered.Z == supplied.Z
}

func (r *sharedWorldRegistry) groundItemVisibleToCharacterLocked(ground sharedGroundItem, character loginticket.Character) bool {
	return r.topology.SharesVisibleWorld(character, loginticket.Character{MapIndex: ground.MapIndex, X: ground.X, Y: ground.Y})
}

func (r *sharedWorldRegistry) groundItemReachableByCharacterLocked(ground sharedGroundItem, character loginticket.Character) bool {
	if !r.groundItemVisibleToCharacterLocked(ground, character) {
		return false
	}
	const bootstrapGroundItemPickupRange = int64(300)
	dx := int64(character.X) - int64(ground.X)
	dy := int64(character.Y) - int64(ground.Y)
	return dx*dx+dy*dy <= bootstrapGroundItemPickupRange*bootstrapGroundItemPickupRange
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
		MapIndex:         ground.MapIndex,
		X:                ground.X,
		Y:                ground.Y,
		Z:                ground.Z,
	}
}

func encodeGroundItemOwnershipFrame(ground sharedGroundItem) []byte {
	return itemproto.EncodeOwnership(itemproto.OwnershipPacket{VID: ground.VID, OwnerName: ground.OwnerName})
}

func encodeGroundItemVisibleFrames(ground sharedGroundItem) [][]byte {
	return [][]byte{encodeGroundItemAddFrame(ground), encodeGroundItemOwnershipFrame(ground)}
}

func encodeGroundItemDeleteFrame(ground sharedGroundItem) []byte {
	return itemproto.EncodeGroundDel(itemproto.GroundDelPacket{VID: ground.VID})
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
	if r == nil || collectorID == 0 || vid == 0 {
		return sharedGroundItemPickup{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	registeredCollector, ok := r.playerCharacter(collectorID)
	if !ok || characterAtBootstrapHPFloor(registeredCollector) || characterAtBootstrapHPFloor(collector) || !sameGroundRewardCollectorLocation(registeredCollector, collector) {
		return sharedGroundItemPickup{}, false
	}
	ground, ok := r.groundItemsByVID[vid]
	if !ok || !r.groundItemReachableByCharacterLocked(ground, registeredCollector) {
		return sharedGroundItemPickup{}, false
	}
	ownerName := ground.OwnerName
	var ownerCharacter loginticket.Character
	if ground.OwnerID != 0 && ground.OwnerID != collectorID {
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
	return owner.ID == ground.OwnerCharacterID && owner.VID == ground.OwnerVID && owner.Name == ground.OwnerName && owner.MapIndex == ground.MapIndex && owner.X == ground.X && owner.Y == ground.Y && owner.Z == ground.Z
}

func (r *sharedWorldRegistry) RemoveGroundItem(collectorID uint64, collector loginticket.Character, vid uint32) bool {
	if r == nil || collectorID == 0 || vid == 0 {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	registeredCollector, ok := r.playerCharacter(collectorID)
	if !ok || characterAtBootstrapHPFloor(registeredCollector) || characterAtBootstrapHPFloor(collector) || !sameGroundRewardCollectorLocation(registeredCollector, collector) {
		return false
	}
	ground, ok := r.groundItemsByVID[vid]
	if !ok || !r.groundItemReachableByCharacterLocked(ground, registeredCollector) {
		return false
	}
	delete(r.groundItemsByVID, vid)
	frames := [][]byte{itemproto.EncodeGroundDel(itemproto.GroundDelPacket{VID: vid})}
	if collectorEntry, ok := r.sessionEntryLocked(collectorID); ok && collectorEntry.FrameSink != nil {
		collectorEntry.FrameSink.Enqueue(frames)
	}
	for _, target := range r.scopesLocked().VisibleTargets(collectorID, collector) {
		if characterAtBootstrapHPFloor(target.Character) {
			continue
		}
		r.enqueueToEntityLocked(target.Entity.ID, frames)
	}
	return true
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

	removedRaw := encodeCharacterDeleteFrame(previous)
	addedRaw := encodePeerVisibilityBootstrapFrames(current)
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

	originFrames := buildTransferOriginFrames(visibilityDiff.RemovedVisiblePeers, originAddedVisiblePeers)
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
	originFrames := buildTransferOriginFrames(visibilityDiff.RemovedVisiblePeers, originAddedVisiblePeers)
	originFrames = append(originFrames, r.buildStaticActorVisibilityTransitionFramesLocked(staticActorVisibilityDiff.RemovedVisibleActors, originAddedVisibleStaticActors)...)
	originFrames = append(originFrames, buildGroundItemVisibilityTransitionFrames(groundItemVisibilityDiff.Removed, originAddedGroundItems)...)
	originEntry, _ := r.sessionEntryLocked(id)
	if enqueueOrigin && originEntry.FrameSink != nil && len(originFrames) > 0 {
		originEntry.FrameSink.Enqueue(originFrames)
	}

	_ = r.entities.UpdatePlayer(id, character)
	r.lastKnownCharacters[id] = character

	movedDelete := encodeCharacterDeleteFrame(previous)
	movedFrames := encodePeerVisibilityBootstrapFrames(character)
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

func (r *sharedWorldRegistry) CharacterVisibility() []CharacterVisibilitySnapshot {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.markCharacterVisibilityStaticActorStateLocked(r.scopesLocked().CharacterVisibilitySnapshotsWithGroundItems(r.groundItemOccupanciesLocked()))
}

func (r *sharedWorldRegistry) InteractionVisibility() []worldruntime.CharacterInteractionVisibilitySnapshot {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.scopesLocked().CharacterInteractionVisibilitySnapshots()
}

func (r *sharedWorldRegistry) MapOccupancy() []MapOccupancySnapshot {
	if r == nil {
		return nil
	}
	return r.mapOccupancySnapshots()
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
	spawnGroupRef = strings.TrimSpace(spawnGroupRef)
	if r == nil || r.entities == nil || name == "" || mapIndex == 0 || raceNum == 0 || !worldruntime.ValidStaticActorInteractionMetadata(interactionKind, interactionRef) || !worldruntime.ValidStaticActorCombatKind(combatKind) || !worldruntime.ValidStaticActorSpawnGroupRef(spawnGroupRef) || !worldruntime.ValidStaticActorDeathReward(deathReward) {
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

	var (
		registered worldruntime.StaticEntity
		ok         bool
	)
	candidate := worldruntime.StaticEntity{
		Entity:          worldruntime.Entity{ID: entityID, Name: name},
		Position:        position,
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
	frames := encodeStaticActorVisibilityFrames(registered)
	if len(frames) > 0 {
		for _, target := range r.scopesLocked().VisibleTargetsForStaticActor(registered) {
			if characterAtBootstrapHPFloor(target.Character) {
				continue
			}
			r.enqueueToEntityLocked(target.Entity.ID, frames)
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
	if r == nil || r.entities == nil || entityID == 0 || name == "" || mapIndex == 0 || raceNum == 0 || !worldruntime.ValidStaticActorInteractionMetadata(interactionKind, interactionRef) || !worldruntime.ValidStaticActorCombatKind(combatKind) || !worldruntime.ValidStaticActorSpawnGroupRef(spawnGroupRef) {
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
	r.syncStaticActorCombatStateLocked(actor)

	refreshFrames := r.buildStaticActorRefreshFramesLocked(previous, actor)
	if len(refreshFrames) > 0 {
		for _, target := range targetDiff.RetainedVisibleTargets {
			if characterAtBootstrapHPFloor(target.Character) {
				continue
			}
			r.enqueueToEntityLocked(target.Entity.ID, refreshFrames)
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
	delete(r.staticActorCombatEngagedBy, actor.Entity.ID)
	if targetVID, ok := worldruntime.StaticActorVisibilityVID(previous); ok {
		r.clearSelectedCombatTargetsLocked(targetVID, 0)
	}
	return r.markStaticActorSnapshotStateLocked(staticActorSnapshot(r.topology, actor)), true
}

func (r *sharedWorldRegistry) StaticActors() []StaticActorSnapshot {
	if r == nil || r.entities == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.markStaticActorSnapshotsStateLocked(r.scopesLocked().StaticActorSnapshots())
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
	if nextHP == 0 {
		attempt.Died = true
		attempt.DeathReward = r.staticActorDeathRewardLocked(actor)
		if r.staticActorCombatEngagedBy != nil {
			delete(r.staticActorCombatEngagedBy, actor.Entity.ID)
		}
		deadRaw := worldproto.EncodeDead(worldproto.DeadPacket{VID: requestedTargetVID})
		clearRaw := combatproto.EncodeServerClearTarget()
		targetedSessionIDs := make(map[uint64]struct{})
		for entityID, targetVID := range r.sessionCombatTargets {
			if targetVID != requestedTargetVID {
				continue
			}
			targetedSessionIDs[entityID] = struct{}{}
			delete(r.sessionCombatTargets, entityID)
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
		attempt.Failure = StaticActorCombatTargetFailureTargetNotTargetable
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
	return StaticActorSnapshot{
		EntityID:         actor.Entity.ID,
		Name:             actor.Entity.Name,
		MapIndex:         topology.EffectiveMapIndex(loginticket.Character{MapIndex: actor.Position.MapIndex}),
		X:                actor.Position.X,
		Y:                actor.Position.Y,
		RaceNum:          actor.RaceNum,
		CombatProfile:    actor.CombatProfile,
		InteractionKind:  actor.InteractionKind,
		InteractionRef:   actor.InteractionRef,
		SpawnGroupRef:    actor.SpawnGroupRef,
		RewardExperience: actor.DeathReward.Experience,
		RewardGold:       actor.DeathReward.Gold,
		RewardDropVnums:  actor.DeathReward.Clone().DropVnums,
	}
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
	snapshot.Dead = r.staticActorDeadLocked(snapshot.EntityID)
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
	}
	return snapshots
}

func (r *sharedWorldRegistry) markMapOccupancyStaticActorStateLocked(snapshots []MapOccupancySnapshot) []MapOccupancySnapshot {
	for i := range snapshots {
		snapshots[i].StaticActors = r.markStaticActorSnapshotsStateLocked(snapshots[i].StaticActors)
	}
	return snapshots
}

func (r *sharedWorldRegistry) markRelocationPreviewStateLocked(preview RelocationPreview) RelocationPreview {
	preview.CurrentVisibleStaticActors = r.markStaticActorSnapshotsStateLocked(preview.CurrentVisibleStaticActors)
	preview.TargetVisibleStaticActors = r.markStaticActorSnapshotsStateLocked(preview.TargetVisibleStaticActors)
	preview.RemovedVisibleStaticActors = r.markStaticActorSnapshotsStateLocked(preview.RemovedVisibleStaticActors)
	preview.AddedVisibleStaticActors = r.markStaticActorSnapshotsStateLocked(preview.AddedVisibleStaticActors)
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
	frames := make([][]byte, 0, len(removed)+len(added)*4)
	for _, peerCharacter := range removed {
		frames = append(frames, encodeCharacterDeleteFrame(peerCharacter))
	}
	for _, peerCharacter := range added {
		frames = append(frames, encodePeerVisibilityBootstrapFrames(peerCharacter)...)
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
	infoRaw, err := worldproto.EncodeCharacterAdditionalInfo(ticketCharacterAdditionalInfoPacket(character))
	if err != nil {
		return nil
	}
	return [][]byte{
		worldproto.EncodeCharacterAdd(ticketCharacterAddPacket(character)),
		infoRaw,
		worldproto.EncodeCharacterUpdate(ticketCharacterUpdatePacket(character)),
	}
}

func encodePeerVisibilityBootstrapFrames(character loginticket.Character) [][]byte {
	frames := encodePeerVisibilityFrames(character)
	if !characterAtBootstrapHPFloor(character) {
		return frames
	}
	return append(frames, worldproto.EncodeDead(worldproto.DeadPacket{VID: character.VID}))
}
