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
	mu               sync.Mutex
	topology         worldruntime.BootstrapTopology
	entities         *worldruntime.EntityRegistry
	sessionDirectory *worldruntime.SessionDirectory
	sessions         map[uint64]sharedWorldSession
}

type sharedWorldSession struct {
	pending  *pendingServerFrames
	relocate sharedWorldSessionRelocator
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
		topology:         topology,
		entities:         worldruntime.NewEntityRegistryWithTopology(topology),
		sessionDirectory: worldruntime.NewSessionDirectory(),
		sessions:         make(map[uint64]sharedWorldSession),
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

func (r *sharedWorldRegistry) Join(character loginticket.Character, pending *pendingServerFrames, relocate sharedWorldSessionRelocator) (uint64, []loginticket.Character) {
	if r == nil {
		return 0, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	currentCharacters := r.snapshotCharactersLocked()
	visibilityDiff := worldruntime.EnterVisibilityDiff(r.topology, character, currentCharacters)
	addedVisibleVIDs := characterVIDSet(visibilityDiff.AddedVisiblePeers)
	peerFrames := encodePeerVisibilityFrames(character)
	for peerID, session := range r.sessions {
		peerCharacter, ok := r.playerCharacter(peerID)
		if !ok {
			continue
		}
		if _, ok := addedVisibleVIDs[peerCharacter.VID]; !ok {
			continue
		}
		session.pending.enqueue(peerFrames)
	}

	registered := r.entities.RegisterPlayer(character)
	if registered.Entity.ID == 0 {
		return 0, nil
	}
	id := registered.Entity.ID
	if !registerSharedWorldSessionEntry(r.sessionDirectory, id, pending, relocate) {
		_, _ = r.entities.Remove(id)
		return 0, nil
	}
	r.sessions[id] = sharedWorldSession{pending: pending, relocate: relocate}
	return id, visibilityDiff.TargetVisiblePeers
}

func (r *sharedWorldRegistry) Leave(id uint64) {
	if r == nil || id == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.sessions[id]; !ok {
		return
	}
	currentCharacter, ok := r.playerCharacter(id)
	if !ok {
		delete(r.sessions, id)
		return
	}
	currentCharacters := r.snapshotCharactersLocked()
	visibilityDiff := worldruntime.LeaveVisibilityDiff(r.topology, currentCharacter, currentCharacters)
	delete(r.sessions, id)
	if r.sessionDirectory != nil {
		_, _ = r.sessionDirectory.Remove(id)
	}
	_, _ = r.entities.Remove(id)

	removeRaw := encodeCharacterDeleteFrame(currentCharacter)
	removedVisibleVIDs := characterVIDSet(visibilityDiff.RemovedVisiblePeers)
	for peerID, peer := range r.sessions {
		peerCharacter, ok := r.playerCharacter(peerID)
		if !ok {
			continue
		}
		if _, ok := removedVisibleVIDs[peerCharacter.VID]; !ok {
			continue
		}
		peer.pending.enqueue([][]byte{removeRaw})
	}
}

func (r *sharedWorldRegistry) UpdateCharacter(id uint64, character loginticket.Character) {
	if r == nil || id == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.sessions[id]; !ok {
		return
	}
	_ = r.entities.UpdatePlayer(id, character)
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
	session, ok := r.sessions[playerEntity.Entity.ID]
	if !ok {
		r.mu.Unlock()
		return RelocationPreview{}, false
	}
	relocate := session.relocate
	r.mu.Unlock()

	if relocate == nil {
		return RelocationPreview{}, false
	}
	return relocate(mapIndex, x, y)
}

func (r *sharedWorldRegistry) Transfer(id uint64, character loginticket.Character) (RelocationPreview, bool) {
	if r == nil || id == 0 {
		return RelocationPreview{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[id]
	if !ok {
		return RelocationPreview{}, false
	}
	previous, ok := r.playerCharacter(id)
	if !ok {
		return RelocationPreview{}, false
	}
	currentCharacters := r.snapshotCharactersLocked()

	afterCharacters := append([]loginticket.Character(nil), currentCharacters...)
	for i := range afterCharacters {
		if afterCharacters[i].VID != previous.VID {
			continue
		}
		afterCharacters[i] = character
		break
	}
	visibilityDiff := worldruntime.RelocateVisibilityDiff(r.topology, previous, currentCharacters, character, afterCharacters)
	beforeOccupancy := r.mapOccupancySnapshotsLocked()
	afterOccupancy := relocateMapOccupancySnapshots(beforeOccupancy, r.topology, previous, character)
	result := RelocationPreview{
		Applied:             true,
		Character:           connectedCharacterSnapshot(r.topology, previous),
		Target:              connectedCharacterSnapshot(r.topology, character),
		CurrentVisiblePeers: connectedCharacterSnapshots(r.topology, visibilityDiff.CurrentVisiblePeers),
		TargetVisiblePeers:  connectedCharacterSnapshots(r.topology, visibilityDiff.TargetVisiblePeers),
		RemovedVisiblePeers: connectedCharacterSnapshots(r.topology, visibilityDiff.RemovedVisiblePeers),
		AddedVisiblePeers:   connectedCharacterSnapshots(r.topology, visibilityDiff.AddedVisiblePeers),
		MapOccupancyChanges: buildMapOccupancyChanges(beforeOccupancy, afterOccupancy),
	}

	for _, peerCharacter := range visibilityDiff.RemovedVisiblePeers {
		session.pending.enqueue([][]byte{encodeCharacterDeleteFrame(peerCharacter)})
	}
	for _, peerCharacter := range visibilityDiff.AddedVisiblePeers {
		session.pending.enqueue(encodePeerVisibilityFrames(peerCharacter))
	}

	_ = r.entities.UpdatePlayer(id, character)

	removedVisibleVIDs := characterVIDSet(visibilityDiff.RemovedVisiblePeers)
	addedVisibleVIDs := characterVIDSet(visibilityDiff.AddedVisiblePeers)
	movedDelete := encodeCharacterDeleteFrame(previous)
	movedFrames := encodePeerVisibilityFrames(character)
	for peerID, peer := range r.sessions {
		if peerID == id {
			continue
		}
		peerCharacter, ok := r.playerCharacter(peerID)
		if !ok {
			continue
		}
		if _, ok := removedVisibleVIDs[peerCharacter.VID]; ok {
			peer.pending.enqueue([][]byte{movedDelete})
		}
		if _, ok := addedVisibleVIDs[peerCharacter.VID]; ok {
			peer.pending.enqueue(movedFrames)
		}
	}

	return result, true
}

func (r *sharedWorldRegistry) ConnectedCharacters() []ConnectedCharacterSnapshot {
	if r == nil {
		return nil
	}

	characters := r.snapshotCharacters()
	snapshots := make([]ConnectedCharacterSnapshot, 0, len(characters))
	for _, character := range characters {
		snapshots = append(snapshots, connectedCharacterSnapshot(r.topology, character))
	}
	sortConnectedCharacterSnapshots(snapshots)
	return snapshots
}

func (r *sharedWorldRegistry) CharacterVisibility() []CharacterVisibilitySnapshot {
	if r == nil {
		return nil
	}

	characters := r.snapshotCharacters()
	snapshots := make([]CharacterVisibilitySnapshot, 0, len(characters))
	for _, character := range characters {
		visiblePeers := connectedCharacterSnapshots(r.topology, worldruntime.VisiblePeers(r.topology, character, characters, character.VID))
		snapshots = append(snapshots, CharacterVisibilitySnapshot{
			ConnectedCharacterSnapshot: connectedCharacterSnapshot(r.topology, character),
			VisiblePeers:               visiblePeers,
		})
	}
	sortCharacterVisibilitySnapshots(snapshots)
	return snapshots
}

func (r *sharedWorldRegistry) MapOccupancy() []MapOccupancySnapshot {
	if r == nil {
		return nil
	}
	return r.mapOccupancySnapshots()
}

func (r *sharedWorldRegistry) PreviewRelocation(name string, mapIndex uint32, x int32, y int32) (RelocationPreview, bool) {
	if r == nil || name == "" || mapIndex == 0 {
		return RelocationPreview{}, false
	}

	current, ok := r.playerCharacterByName(name)
	if !ok {
		return RelocationPreview{}, false
	}
	characters := r.snapshotCharacters()
	target := current
	target.MapIndex = mapIndex
	target.X = x
	target.Y = y

	afterCharacters := append([]loginticket.Character(nil), characters...)
	for i := range afterCharacters {
		if afterCharacters[i].VID != current.VID {
			continue
		}
		afterCharacters[i] = target
		break
	}
	visibilityDiff := worldruntime.RelocateVisibilityDiff(r.topology, current, characters, target, afterCharacters)
	beforeOccupancy := r.mapOccupancySnapshots()
	afterOccupancy := relocateMapOccupancySnapshots(beforeOccupancy, r.topology, current, target)

	return RelocationPreview{
		Applied:             false,
		Character:           connectedCharacterSnapshot(r.topology, current),
		Target:              connectedCharacterSnapshot(r.topology, target),
		CurrentVisiblePeers: connectedCharacterSnapshots(r.topology, visibilityDiff.CurrentVisiblePeers),
		TargetVisiblePeers:  connectedCharacterSnapshots(r.topology, visibilityDiff.TargetVisiblePeers),
		RemovedVisiblePeers: connectedCharacterSnapshots(r.topology, visibilityDiff.RemovedVisiblePeers),
		AddedVisiblePeers:   connectedCharacterSnapshots(r.topology, visibilityDiff.AddedVisiblePeers),
		MapOccupancyChanges: buildMapOccupancyChanges(beforeOccupancy, afterOccupancy),
	}, true
}

func (r *sharedWorldRegistry) EnqueueToOtherSessions(originID uint64, frames [][]byte) {
	if r == nil || originID == 0 || len(frames) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for id, session := range r.sessions {
		if id == originID {
			continue
		}
		session.pending.enqueue(frames)
	}
}

func (r *sharedWorldRegistry) EnqueueToVisibleSessions(originID uint64, origin loginticket.Character, frames [][]byte) {
	if r == nil || originID == 0 || len(frames) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for id, session := range r.sessions {
		peerCharacter, ok := r.playerCharacter(id)
		if !ok || id == originID || !r.topology.SharesVisibleWorld(origin, peerCharacter) {
			continue
		}
		session.pending.enqueue(frames)
	}
}

func (r *sharedWorldRegistry) EnqueueToOtherSessionsInEmpire(originID uint64, origin loginticket.Character, frames [][]byte) {
	if r == nil || originID == 0 || origin.Empire == 0 || len(frames) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for id, session := range r.sessions {
		peerCharacter, ok := r.playerCharacter(id)
		if !ok || id == originID || !r.topology.SharesShoutScope(origin, peerCharacter) {
			continue
		}
		session.pending.enqueue(frames)
	}
}

func (r *sharedWorldRegistry) EnqueueToOtherSessionsInEmpireOnMap(originID uint64, origin loginticket.Character, frames [][]byte) {
	if r == nil || originID == 0 || origin.Empire == 0 || len(frames) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for id, session := range r.sessions {
		peerCharacter, ok := r.playerCharacter(id)
		if !ok || id == originID || !r.topology.SharesTalkingChatScope(origin, peerCharacter) {
			continue
		}
		session.pending.enqueue(frames)
	}
}

func (r *sharedWorldRegistry) EnqueueToOtherSessionsInGuild(originID uint64, origin loginticket.Character, frames [][]byte) {
	if r == nil || originID == 0 || origin.GuildID == 0 || len(frames) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for id, session := range r.sessions {
		peerCharacter, ok := r.playerCharacter(id)
		if !ok || id == originID || !r.topology.SharesGuildChatScope(origin, peerCharacter) {
			continue
		}
		session.pending.enqueue(frames)
	}
}

func (r *sharedWorldRegistry) EnqueueToCharacterName(name string, frames [][]byte) bool {
	if r == nil || name == "" || len(frames) == 0 {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	playerEntity, ok := r.playerEntityByName(name)
	if !ok {
		return false
	}
	session, ok := r.sessions[playerEntity.Entity.ID]
	if !ok {
		return false
	}
	session.pending.enqueue(frames)
	return true
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
	for _, session := range r.sessions {
		session.pending.enqueue([][]byte{noticeRaw})
		delivered++
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

func connectedCharacterSnapshots(topology worldruntime.BootstrapTopology, characters []loginticket.Character) []ConnectedCharacterSnapshot {
	snapshots := make([]ConnectedCharacterSnapshot, 0, len(characters))
	for _, character := range characters {
		snapshots = append(snapshots, connectedCharacterSnapshot(topology, character))
	}
	sortConnectedCharacterSnapshots(snapshots)
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
	return buildMapOccupancySnapshots(r.topology, r.entities.MapOccupancy())
}

func buildMapOccupancySnapshots(topology worldruntime.BootstrapTopology, occupancies []worldruntime.MapOccupancy) []MapOccupancySnapshot {
	snapshots := make([]MapOccupancySnapshot, 0, len(occupancies))
	for _, occupancy := range occupancies {
		snapshots = append(snapshots, MapOccupancySnapshot{
			MapIndex:       occupancy.MapIndex,
			CharacterCount: len(occupancy.Characters),
			Characters:     connectedCharacterSnapshots(topology, occupancy.Characters),
		})
	}
	sortMapOccupancySnapshots(snapshots)
	return snapshots
}

func relocateMapOccupancySnapshots(before []MapOccupancySnapshot, topology worldruntime.BootstrapTopology, current loginticket.Character, target loginticket.Character) []MapOccupancySnapshot {
	byMap := make(map[uint32][]ConnectedCharacterSnapshot, len(before)+1)
	for _, snapshot := range before {
		characters := append([]ConnectedCharacterSnapshot(nil), snapshot.Characters...)
		byMap[snapshot.MapIndex] = characters
	}

	currentSnapshot := connectedCharacterSnapshot(topology, current)
	targetSnapshot := connectedCharacterSnapshot(topology, target)

	if characters, ok := byMap[currentSnapshot.MapIndex]; ok {
		filtered := make([]ConnectedCharacterSnapshot, 0, len(characters))
		for _, character := range characters {
			if character.VID == currentSnapshot.VID {
				continue
			}
			filtered = append(filtered, character)
		}
		if len(filtered) == 0 {
			delete(byMap, currentSnapshot.MapIndex)
		} else {
			byMap[currentSnapshot.MapIndex] = filtered
		}
	}

	targetCharacters := append([]ConnectedCharacterSnapshot(nil), byMap[targetSnapshot.MapIndex]...)
	replaced := false
	for i := range targetCharacters {
		if targetCharacters[i].VID != targetSnapshot.VID {
			continue
		}
		targetCharacters[i] = targetSnapshot
		replaced = true
		break
	}
	if !replaced {
		targetCharacters = append(targetCharacters, targetSnapshot)
	}
	sortConnectedCharacterSnapshots(targetCharacters)
	byMap[targetSnapshot.MapIndex] = targetCharacters

	snapshots := make([]MapOccupancySnapshot, 0, len(byMap))
	for mapIndex, characters := range byMap {
		sortConnectedCharacterSnapshots(characters)
		snapshots = append(snapshots, MapOccupancySnapshot{
			MapIndex:       mapIndex,
			CharacterCount: len(characters),
			Characters:     characters,
		})
	}
	sortMapOccupancySnapshots(snapshots)
	return snapshots
}

func buildMapOccupancyChanges(before []MapOccupancySnapshot, after []MapOccupancySnapshot) []MapOccupancyChange {
	beforeCounts := make(map[uint32]int, len(before))
	for _, snapshot := range before {
		beforeCounts[snapshot.MapIndex] = snapshot.CharacterCount
	}
	afterCounts := make(map[uint32]int, len(after))
	for _, snapshot := range after {
		afterCounts[snapshot.MapIndex] = snapshot.CharacterCount
	}

	indices := make(map[uint32]struct{}, len(beforeCounts)+len(afterCounts))
	for mapIndex := range beforeCounts {
		indices[mapIndex] = struct{}{}
	}
	for mapIndex := range afterCounts {
		indices[mapIndex] = struct{}{}
	}

	changes := make([]MapOccupancyChange, 0, len(indices))
	for mapIndex := range indices {
		beforeCount := beforeCounts[mapIndex]
		afterCount := afterCounts[mapIndex]
		if beforeCount == afterCount {
			continue
		}
		changes = append(changes, MapOccupancyChange{MapIndex: mapIndex, BeforeCount: beforeCount, AfterCount: afterCount})
	}
	sortMapOccupancyChanges(changes)
	return changes
}

func characterVIDSet(characters []loginticket.Character) map[uint32]struct{} {
	vids := make(map[uint32]struct{}, len(characters))
	for _, character := range characters {
		vids[character.VID] = struct{}{}
	}
	return vids
}

func sortConnectedCharacterSnapshots(snapshots []ConnectedCharacterSnapshot) {
	sort.Slice(snapshots, func(i int, j int) bool {
		if snapshots[i].Name == snapshots[j].Name {
			return snapshots[i].VID < snapshots[j].VID
		}
		return snapshots[i].Name < snapshots[j].Name
	})
}

func sortCharacterVisibilitySnapshots(snapshots []CharacterVisibilitySnapshot) {
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

func sortMapOccupancyChanges(changes []MapOccupancyChange) {
	sort.Slice(changes, func(i int, j int) bool {
		return changes[i].MapIndex < changes[j].MapIndex
	})
}

func encodeCharacterDeleteFrame(character loginticket.Character) []byte {
	return worldproto.EncodeCharacterDeleteNotice(worldproto.CharacterDeleteNoticePacket{VID: character.VID})
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
