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
	mu       sync.Mutex
	nextID   uint64
	topology worldruntime.BootstrapTopology
	sessions map[uint64]sharedWorldSession
}

type sharedWorldSession struct {
	character loginticket.Character
	pending   *pendingServerFrames
	relocate  sharedWorldSessionRelocator
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
		topology: topology,
		sessions: make(map[uint64]sharedWorldSession),
	}
}

func (r *sharedWorldRegistry) Join(character loginticket.Character, pending *pendingServerFrames, relocate sharedWorldSessionRelocator) (uint64, []loginticket.Character) {
	if r == nil {
		return 0, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	existing := make([]loginticket.Character, 0, len(r.sessions))
	peerFrames := encodePeerVisibilityFrames(character)
	for _, session := range r.sessions {
		if !r.topology.SharesVisibleWorld(character, session.character) {
			continue
		}
		existing = append(existing, session.character)
		session.pending.enqueue(peerFrames)
	}

	r.nextID++
	id := r.nextID
	r.sessions[id] = sharedWorldSession{character: character, pending: pending, relocate: relocate}
	return id, existing
}

func (r *sharedWorldRegistry) Leave(id uint64) {
	if r == nil || id == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[id]
	if !ok {
		return
	}
	delete(r.sessions, id)

	removeRaw := encodeCharacterDeleteFrame(session.character)
	for _, peer := range r.sessions {
		if !r.topology.SharesVisibleWorld(session.character, peer.character) {
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

	session, ok := r.sessions[id]
	if !ok {
		return
	}
	session.character = character
	r.sessions[id] = session
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
	var relocate sharedWorldSessionRelocator
	for _, session := range r.sessions {
		if session.character.Name != name {
			continue
		}
		relocate = session.relocate
		break
	}
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

	previous := session.character
	currentCharacters := make([]loginticket.Character, 0, len(r.sessions))
	for _, candidate := range r.sessions {
		currentCharacters = append(currentCharacters, candidate.character)
	}
	currentVisibleCharacters := make([]loginticket.Character, 0)
	targetVisibleCharacters := make([]loginticket.Character, 0)
	oldPeers := make(map[uint64]sharedWorldSession)
	newPeers := make(map[uint64]sharedWorldSession)
	for peerID, peer := range r.sessions {
		if peerID == id {
			continue
		}
		if r.topology.SharesVisibleWorld(previous, peer.character) {
			oldPeers[peerID] = peer
			currentVisibleCharacters = append(currentVisibleCharacters, peer.character)
		}
		if r.topology.SharesVisibleWorld(character, peer.character) {
			newPeers[peerID] = peer
			targetVisibleCharacters = append(targetVisibleCharacters, peer.character)
		}
	}
	currentVisiblePeers := connectedCharacterSnapshots(r.topology, currentVisibleCharacters)
	targetVisiblePeers := connectedCharacterSnapshots(r.topology, targetVisibleCharacters)
	removedVisibleCharacters, addedVisibleCharacters := worldruntime.DiffVisiblePeers(currentVisibleCharacters, targetVisibleCharacters)
	removedVisiblePeers := connectedCharacterSnapshots(r.topology, removedVisibleCharacters)
	addedVisiblePeers := connectedCharacterSnapshots(r.topology, addedVisibleCharacters)

	afterCharacters := append([]loginticket.Character(nil), currentCharacters...)
	for i := range afterCharacters {
		if afterCharacters[i].VID != previous.VID {
			continue
		}
		afterCharacters[i] = character
		break
	}
	result := RelocationPreview{
		Applied:             true,
		Character:           connectedCharacterSnapshot(r.topology, previous),
		Target:              connectedCharacterSnapshot(r.topology, character),
		CurrentVisiblePeers: currentVisiblePeers,
		TargetVisiblePeers:  targetVisiblePeers,
		RemovedVisiblePeers: removedVisiblePeers,
		AddedVisiblePeers:   addedVisiblePeers,
		MapOccupancyChanges: buildMapOccupancyChanges(buildMapOccupancySnapshots(r.topology, currentCharacters), buildMapOccupancySnapshots(r.topology, afterCharacters)),
	}

	for peerID, peer := range oldPeers {
		if _, stillVisible := newPeers[peerID]; stillVisible {
			continue
		}
		session.pending.enqueue([][]byte{encodeCharacterDeleteFrame(peer.character)})
	}
	for peerID, peer := range newPeers {
		if _, alreadyVisible := oldPeers[peerID]; alreadyVisible {
			continue
		}
		session.pending.enqueue(encodePeerVisibilityFrames(peer.character))
	}

	session.character = character
	r.sessions[id] = session

	movedDelete := encodeCharacterDeleteFrame(previous)
	movedFrames := encodePeerVisibilityFrames(character)
	for peerID, peer := range oldPeers {
		if _, stillVisible := newPeers[peerID]; stillVisible {
			continue
		}
		peer.pending.enqueue([][]byte{movedDelete})
	}
	for peerID, peer := range newPeers {
		if _, alreadyVisible := oldPeers[peerID]; alreadyVisible {
			continue
		}
		peer.pending.enqueue(movedFrames)
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
	return buildMapOccupancySnapshots(r.topology, r.snapshotCharacters())
}

func (r *sharedWorldRegistry) PreviewRelocation(name string, mapIndex uint32, x int32, y int32) (RelocationPreview, bool) {
	if r == nil || name == "" || mapIndex == 0 {
		return RelocationPreview{}, false
	}

	characters := r.snapshotCharacters()
	targetIndex := -1
	for i, character := range characters {
		if character.Name != name {
			continue
		}
		targetIndex = i
		break
	}
	if targetIndex < 0 {
		return RelocationPreview{}, false
	}

	current := characters[targetIndex]
	target := current
	target.MapIndex = mapIndex
	target.X = x
	target.Y = y

	currentVisibleCharacters := worldruntime.VisiblePeers(r.topology, current, characters, current.VID)
	afterCharacters := append([]loginticket.Character(nil), characters...)
	afterCharacters[targetIndex] = target
	targetVisibleCharacters := worldruntime.VisiblePeers(r.topology, target, afterCharacters, target.VID)
	removedVisibleCharacters, addedVisibleCharacters := worldruntime.DiffVisiblePeers(currentVisibleCharacters, targetVisibleCharacters)
	currentVisiblePeers := connectedCharacterSnapshots(r.topology, currentVisibleCharacters)
	targetVisiblePeers := connectedCharacterSnapshots(r.topology, targetVisibleCharacters)
	removedVisiblePeers := connectedCharacterSnapshots(r.topology, removedVisibleCharacters)
	addedVisiblePeers := connectedCharacterSnapshots(r.topology, addedVisibleCharacters)

	return RelocationPreview{
		Applied:             false,
		Character:           connectedCharacterSnapshot(r.topology, current),
		Target:              connectedCharacterSnapshot(r.topology, target),
		CurrentVisiblePeers: currentVisiblePeers,
		TargetVisiblePeers:  targetVisiblePeers,
		RemovedVisiblePeers: removedVisiblePeers,
		AddedVisiblePeers:   addedVisiblePeers,
		MapOccupancyChanges: buildMapOccupancyChanges(buildMapOccupancySnapshots(r.topology, characters), buildMapOccupancySnapshots(r.topology, afterCharacters)),
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
		if id == originID || !r.topology.SharesVisibleWorld(origin, session.character) {
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
		if id == originID || !r.topology.SharesShoutScope(origin, session.character) {
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
		if id == originID || !r.topology.SharesTalkingChatScope(origin, session.character) {
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
		if id == originID || !r.topology.SharesGuildChatScope(origin, session.character) {
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

	for _, session := range r.sessions {
		if session.character.Name != name {
			continue
		}
		session.pending.enqueue(frames)
		return true
	}
	return false
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
	characters := make([]loginticket.Character, 0, len(r.sessions))
	for _, session := range r.sessions {
		characters = append(characters, session.character)
	}
	r.mu.Unlock()
	return characters
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

func buildMapOccupancySnapshots(topology worldruntime.BootstrapTopology, characters []loginticket.Character) []MapOccupancySnapshot {
	byMap := make(map[uint32][]ConnectedCharacterSnapshot)
	for _, character := range characters {
		mapIndex := topology.EffectiveMapIndex(character)
		byMap[mapIndex] = append(byMap[mapIndex], connectedCharacterSnapshot(topology, character))
	}

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
