package minimal

import (
	"sort"
	"sync"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

type queuedSessionFlow struct {
	inner     service.SessionFlow
	pending   *pendingServerFrames
	onClose   func()
	closeOnce sync.Once
	closeErr  error
}

type pendingServerFrames struct {
	mu     sync.Mutex
	frames [][]byte
}

type sharedWorldRegistry struct {
	mu                  sync.Mutex
	topology            worldruntime.BootstrapTopology
	entities            *worldruntime.EntityRegistry
	sessionDirectory    *worldruntime.SessionDirectory
	lastKnownCharacters map[uint64]loginticket.Character
}

func newQueuedSessionFlow(inner service.SessionFlow, pending *pendingServerFrames, onClose func()) *queuedSessionFlow {
	return &queuedSessionFlow{inner: inner, pending: pending, onClose: onClose}
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
		topology:            topology,
		entities:            worldruntime.NewEntityRegistryWithTopology(topology),
		sessionDirectory:    worldruntime.NewSessionDirectory(),
		lastKnownCharacters: make(map[uint64]loginticket.Character),
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

func (r *sharedWorldRegistry) MapOccupancy() []MapOccupancySnapshot {
	if r == nil {
		return nil
	}
	return r.mapOccupancySnapshots()
}

func (r *sharedWorldRegistry) RegisterStaticActor(name string, mapIndex uint32, x int32, y int32, raceNum uint32) (StaticActorSnapshot, bool) {
	if r == nil || r.entities == nil || name == "" || mapIndex == 0 || raceNum == 0 {
		return StaticActorSnapshot{}, false
	}
	position := worldruntime.NewPosition(mapIndex, x, y)
	if !position.Valid() {
		return StaticActorSnapshot{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	actor, ok := r.entities.RegisterStaticActor(worldruntime.StaticEntity{
		Entity:   worldruntime.Entity{Name: name},
		Position: position,
		RaceNum:  raceNum,
	})
	if !ok {
		return StaticActorSnapshot{}, false
	}
	return staticActorSnapshot(r.topology, actor), true
}

func (r *sharedWorldRegistry) UpdateStaticActor(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32) (StaticActorSnapshot, bool) {
	if r == nil || r.entities == nil || entityID == 0 || name == "" || mapIndex == 0 || raceNum == 0 {
		return StaticActorSnapshot{}, false
	}
	position := worldruntime.NewPosition(mapIndex, x, y)
	if !position.Valid() {
		return StaticActorSnapshot{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	actor, ok := r.entities.UpdateStaticActor(worldruntime.StaticEntity{
		Entity:   worldruntime.Entity{ID: entityID, Name: name},
		Position: position,
		RaceNum:  raceNum,
	})
	if !ok {
		return StaticActorSnapshot{}, false
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
		r.enqueueToEntityLocked(target.Entity.ID, frames)
	}
}

func (r *sharedWorldRegistry) EnqueueToCharacterName(name string, frames [][]byte) bool {
	if r == nil || name == "" || len(frames) == 0 {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	playerEntity, ok := r.scopesLocked().PlayerByExactName(name)
	if !ok {
		return false
	}
	return r.enqueueToEntityLocked(playerEntity.Entity.ID, frames)
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
		EntityID: actor.Entity.ID,
		Name:     actor.Entity.Name,
		MapIndex: topology.EffectiveMapIndex(loginticket.Character{MapIndex: actor.Position.MapIndex}),
		X:        actor.Position.X,
		Y:        actor.Position.Y,
		RaceNum:  actor.RaceNum,
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

func encodeStaticActorDeleteFrame(actor worldruntime.StaticEntity) ([]byte, bool) {
	vid, ok := staticActorVisibilityVID(actor)
	if !ok {
		return nil, false
	}
	return worldproto.EncodeCharacterDeleteNotice(worldproto.CharacterDeleteNoticePacket{VID: vid}), true
}

func staticActorVisibilityVID(actor worldruntime.StaticEntity) (uint32, bool) {
	if actor.Entity.ID == 0 || actor.Entity.ID > uint64(^uint32(0)) || actor.RaceNum > uint32(^uint16(0)) {
		return 0, false
	}
	return uint32(actor.Entity.ID), true
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
