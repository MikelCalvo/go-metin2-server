package minimal

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
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
	sessionCombatTargets            map[uint64]uint32
	nextStaticActorCombatSnapshotID uint64
	lastKnownCharacters             map[uint64]loginticket.Character
	now                             func() time.Time
}

const (
	StaticActorInteractionFailureSubjectNotFound        = "subject_not_found"
	StaticActorInteractionFailureTargetNotVisible       = "target_not_visible"
	StaticActorInteractionFailureTargetOutOfRange       = "target_out_of_range"
	StaticActorInteractionFailureTargetHasNoInteraction = "target_has_no_interaction"

	StaticActorCombatTargetFailureSubjectNotFound     = "subject_not_found"
	StaticActorCombatTargetFailureTargetNotVisible    = "target_not_visible"
	StaticActorCombatTargetFailureTargetOutOfRange    = "target_out_of_range"
	StaticActorCombatTargetFailureTargetNotTargetable = "target_not_targetable"
	StaticActorCombatTargetFailureTargetDead          = "target_dead"

	StaticActorCombatAttackFailureSubjectNotFound        = "subject_not_found"
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

type StaticActorCombatAttackAttempt struct {
	Accepted                    bool
	Failure                     string
	ActiveTargetVID             uint32
	ActiveTargetSnapshotVersion uint64
	RequestedTargetVID          uint32
	HPPercent                   uint8
	Died                        bool
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
		lastKnownCharacters:        make(map[uint64]loginticket.Character),
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

func (r *sharedWorldRegistry) staticActorAggroLiteBlocksFreshTargetLocked(subjectID uint64, actor worldruntime.StaticEntity, targetVID uint32) bool {
	if r == nil || subjectID == 0 || actor.Entity.ID == 0 || targetVID == 0 {
		return false
	}
	if actor.SpawnGroupRef == "" || actor.CombatKind != worldruntime.StaticActorCombatKindTrainingDummy {
		return false
	}
	engagedBy, ok := r.staticActorCombatEngagedBy[actor.Entity.ID]
	if !ok || engagedBy == 0 || engagedBy == subjectID {
		return false
	}
	if r.sessionCombatTargets != nil && r.sessionCombatTargets[subjectID] == targetVID {
		return false
	}
	return true
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
			r.enqueueToCharacterLocked(peerCharacter, [][]byte{removeRaw})
		}
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

	peerFrames := encodePeerVisibilityFrames(character)
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
		r.enqueueToCharacterLocked(peerCharacter, [][]byte{removeRaw})
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

func (r *sharedWorldRegistry) UpdateCharacterWithVisibilityTransition(id uint64, previous loginticket.Character, current loginticket.Character, stableFrames [][]byte) {
	if r == nil || id == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	scopes := r.scopesLocked()
	visibilityDiff := scopes.RelocateVisibilityDiff(previous, current)
	staticActorVisibilityDiff := scopes.RelocateStaticActorVisibilityDiff(previous, current)
	_ = r.entities.UpdatePlayer(id, current)
	r.lastKnownCharacters[id] = current

	removedRaw := encodeCharacterDeleteFrame(previous)
	addedRaw := encodePeerVisibilityFrames(current)
	stablePeerVIDs := make(map[uint32]struct{}, len(visibilityDiff.AddedVisiblePeers))
	for _, peerCharacter := range visibilityDiff.AddedVisiblePeers {
		stablePeerVIDs[peerCharacter.VID] = struct{}{}
	}
	for _, peerCharacter := range visibilityDiff.RemovedVisiblePeers {
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
		r.enqueueToCharacterLocked(peerCharacter, addedRaw)
	}

	originFrames := buildTransferOriginFrames(visibilityDiff.RemovedVisiblePeers, visibilityDiff.AddedVisiblePeers)
	originFrames = append(originFrames, buildStaticActorVisibilityTransitionFrames(staticActorVisibilityDiff.RemovedVisibleActors, staticActorVisibilityDiff.AddedVisibleActors)...)
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
	result := scopes.BuildRelocationPreview(previous, character, true)

	originFrames := buildTransferOriginFrames(visibilityDiff.RemovedVisiblePeers, visibilityDiff.AddedVisiblePeers)
	originFrames = append(originFrames, buildStaticActorVisibilityTransitionFrames(staticActorVisibilityDiff.RemovedVisibleActors, staticActorVisibilityDiff.AddedVisibleActors)...)
	originEntry, _ := r.sessionEntryLocked(id)
	if enqueueOrigin && originEntry.FrameSink != nil && len(originFrames) > 0 {
		originEntry.FrameSink.Enqueue(originFrames)
	}

	_ = r.entities.UpdatePlayer(id, character)
	r.lastKnownCharacters[id] = character

	movedDelete := encodeCharacterDeleteFrame(previous)
	movedFrames := encodePeerVisibilityFrames(character)
	for _, peerCharacter := range visibilityDiff.RemovedVisiblePeers {
		r.enqueueToCharacterLocked(peerCharacter, [][]byte{movedDelete})
	}
	for _, peerCharacter := range visibilityDiff.AddedVisiblePeers {
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
	return r.scopesLocked().CharacterVisibilitySnapshots()
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
	return r.registerStaticActor(0, name, mapIndex, x, y, raceNum, "", "", "", "")
}

func (r *sharedWorldRegistry) RegisterStaticActorWithInteraction(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string) (StaticActorSnapshot, bool) {
	return r.registerStaticActor(entityID, name, mapIndex, x, y, raceNum, interactionKind, interactionRef, "", "")
}

func (r *sharedWorldRegistry) RegisterStaticActorWithCombatKind(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32, combatKind string) (StaticActorSnapshot, bool) {
	return r.registerStaticActor(entityID, name, mapIndex, x, y, raceNum, "", "", combatKind, "")
}

func (r *sharedWorldRegistry) registerStaticActor(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string, combatKind string, spawnGroupRef string) (StaticActorSnapshot, bool) {
	spawnGroupRef = strings.TrimSpace(spawnGroupRef)
	if r == nil || r.entities == nil || name == "" || mapIndex == 0 || raceNum == 0 || !worldruntime.ValidStaticActorInteractionMetadata(interactionKind, interactionRef) || !worldruntime.ValidStaticActorCombatKind(combatKind) || !worldruntime.ValidStaticActorSpawnGroupRef(spawnGroupRef) {
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
			r.enqueueToEntityLocked(target.Entity.ID, frames)
		}
	}
	return staticActorSnapshot(r.topology, registered), true
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

	refreshFrames := buildStaticActorRefreshFrames(previous, actor)
	if len(refreshFrames) > 0 {
		for _, target := range targetDiff.RetainedVisibleTargets {
			r.enqueueToEntityLocked(target.Entity.ID, refreshFrames)
		}
	}
	deleteRaw, deleteEncodable := encodeStaticActorDeleteFrame(previous)
	if deleteEncodable {
		for _, target := range targetDiff.RemovedVisibleTargets {
			r.enqueueToEntityLocked(target.Entity.ID, [][]byte{deleteRaw})
		}
	}
	addFrames := encodeStaticActorVisibilityFrames(actor)
	if len(addFrames) > 0 {
		for _, target := range targetDiff.AddedVisibleTargets {
			r.enqueueToEntityLocked(target.Entity.ID, addFrames)
		}
	}
	return staticActorSnapshot(r.topology, actor), true
}

func (r *sharedWorldRegistry) StaticActors() []StaticActorSnapshot {
	if r == nil || r.entities == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.scopesLocked().StaticActorSnapshots()
}

func (r *sharedWorldRegistry) VisibleStaticActorFrames(subject loginticket.Character) [][]byte {
	if r == nil || r.entities == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	actors := r.scopesLocked().VisibleStaticActors(subject)
	frames := make([][]byte, 0, len(actors)*3)
	for _, actor := range actors {
		frames = append(frames, encodeStaticActorVisibilityFrames(actor)...)
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
	actor, ok := r.scopesLocked().VisibleStaticActorByVID(subject, targetVID)
	if !ok {
		attempt.Failure = StaticActorInteractionFailureTargetNotVisible
		return attempt
	}
	attempt.Actor = staticActorSnapshot(r.topology, actor)
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
	targetAttempt := r.attemptStaticActorCombatTargetLocked(subjectID, subject, activeTargetVID)
	attempt.Actor = targetAttempt.Actor
	attempt.HPPercent = targetAttempt.HPPercent
	if !targetAttempt.Accepted {
		switch targetAttempt.Failure {
		case StaticActorCombatTargetFailureSubjectNotFound:
			attempt.Failure = StaticActorCombatAttackFailureSubjectNotFound
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
	attempt.Actor = staticActorSnapshot(r.topology, actor)
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
	attempt.Accepted = true
	attempt.HPPercent = hpPercent
	if nextHP == 0 {
		attempt.Died = true
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

func (r *sharedWorldRegistry) attemptStaticActorCombatTargetLocked(subjectID uint64, subject loginticket.Character, targetVID uint32) StaticActorCombatTargetAttempt {
	attempt := StaticActorCombatTargetAttempt{TargetVID: targetVID}
	actor, ok := r.scopesLocked().VisibleStaticActorByVID(subject, targetVID)
	if !ok {
		attempt.Failure = StaticActorCombatTargetFailureTargetNotVisible
		return attempt
	}
	attempt.Actor = staticActorSnapshot(r.topology, actor)
	if !worldruntime.StaticActorWithinInteractionRange(subject, actor, staticActorCombatTargetMaxDistance) {
		attempt.Failure = StaticActorCombatTargetFailureTargetOutOfRange
		return attempt
	}
	if actor.CombatKind != worldruntime.StaticActorCombatKindTrainingDummy {
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
	r.clearStaticActorCombatStateLocked(entityID)
	deleteRaw, encodable := encodeStaticActorDeleteFrame(actor)
	if encodable {
		for _, target := range r.scopesLocked().VisibleTargetsForStaticActor(actor) {
			r.enqueueToEntityLocked(target.Entity.ID, [][]byte{deleteRaw})
		}
	}
	return staticActorSnapshot(r.topology, actor), true
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

	return r.scopesLocked().BuildRelocationPreview(current, target, false), true
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

func (r *sharedWorldRegistry) playerCharacter(id uint64) (loginticket.Character, bool) {
	if r == nil || r.entities == nil {
		return loginticket.Character{}, false
	}
	playerEntity, ok := r.entities.Player(id)
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
		EntityID:        actor.Entity.ID,
		Name:            actor.Entity.Name,
		MapIndex:        topology.EffectiveMapIndex(loginticket.Character{MapIndex: actor.Position.MapIndex}),
		X:               actor.Position.X,
		Y:               actor.Position.Y,
		RaceNum:         actor.RaceNum,
		InteractionKind: actor.InteractionKind,
		InteractionRef:  actor.InteractionRef,
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

func (r *sharedWorldRegistry) mapOccupancySnapshotsLocked() []MapOccupancySnapshot {
	if r == nil || r.entities == nil {
		return nil
	}
	return r.scopesLocked().MapOccupancySnapshots()
}

func buildTransferOriginFrames(removed []loginticket.Character, added []loginticket.Character) [][]byte {
	frames := make([][]byte, 0, len(removed)+len(added)*3)
	for _, peerCharacter := range removed {
		frames = append(frames, encodeCharacterDeleteFrame(peerCharacter))
	}
	for _, peerCharacter := range added {
		frames = append(frames, encodePeerVisibilityFrames(peerCharacter)...)
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

func buildStaticActorVisibilityTransitionFrames(removed []worldruntime.StaticEntity, added []worldruntime.StaticEntity) [][]byte {
	frames := make([][]byte, 0, len(removed)+len(added)*3)
	for _, actor := range removed {
		deleteRaw, ok := encodeStaticActorDeleteFrame(actor)
		if !ok {
			continue
		}
		frames = append(frames, deleteRaw)
	}
	for _, actor := range added {
		frames = append(frames, encodeStaticActorVisibilityFrames(actor)...)
	}
	return frames
}

func buildStaticActorRefreshFrames(previous worldruntime.StaticEntity, updated worldruntime.StaticEntity) [][]byte {
	deleteRaw, ok := encodeStaticActorDeleteFrame(previous)
	if !ok {
		return nil
	}
	addFrames := encodeStaticActorVisibilityFrames(updated)
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
		Level:     0,
		Alignment: 0,
		PKMode:    0,
		MountVnum: 0,
	}
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
