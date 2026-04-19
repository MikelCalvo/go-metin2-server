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
	return &sharedWorldRegistry{sessions: make(map[uint64]sharedWorldSession)}
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
		if !charactersShareVisibleWorld(character, session.character) {
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
		if !charactersShareVisibleWorld(session.character, peer.character) {
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
	currentVisiblePeers := make([]ConnectedCharacterSnapshot, 0)
	targetVisiblePeers := make([]ConnectedCharacterSnapshot, 0)
	oldPeers := make(map[uint64]sharedWorldSession)
	newPeers := make(map[uint64]sharedWorldSession)
	for peerID, peer := range r.sessions {
		if peerID == id {
			continue
		}
		if charactersShareVisibleWorld(previous, peer.character) {
			oldPeers[peerID] = peer
			currentVisiblePeers = append(currentVisiblePeers, connectedCharacterSnapshot(peer.character))
		}
		if charactersShareVisibleWorld(character, peer.character) {
			newPeers[peerID] = peer
			targetVisiblePeers = append(targetVisiblePeers, connectedCharacterSnapshot(peer.character))
		}
	}
	sortConnectedCharacterSnapshots(currentVisiblePeers)
	sortConnectedCharacterSnapshots(targetVisiblePeers)
	removedVisiblePeers, addedVisiblePeers := diffVisiblePeerSnapshots(currentVisiblePeers, targetVisiblePeers)

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
		Character:           connectedCharacterSnapshot(previous),
		Target:              connectedCharacterSnapshot(character),
		CurrentVisiblePeers: currentVisiblePeers,
		TargetVisiblePeers:  targetVisiblePeers,
		RemovedVisiblePeers: removedVisiblePeers,
		AddedVisiblePeers:   addedVisiblePeers,
		MapOccupancyChanges: buildMapOccupancyChanges(buildMapOccupancySnapshots(currentCharacters), buildMapOccupancySnapshots(afterCharacters)),
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
		snapshots = append(snapshots, connectedCharacterSnapshot(character))
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
		visiblePeers := buildVisiblePeerSnapshots(character, characters, character.VID)
		snapshots = append(snapshots, CharacterVisibilitySnapshot{
			ConnectedCharacterSnapshot: connectedCharacterSnapshot(character),
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
	return buildMapOccupancySnapshots(r.snapshotCharacters())
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

	currentVisiblePeers := buildVisiblePeerSnapshots(current, characters, current.VID)
	afterCharacters := append([]loginticket.Character(nil), characters...)
	afterCharacters[targetIndex] = target
	targetVisiblePeers := buildVisiblePeerSnapshots(target, afterCharacters, target.VID)
	removedVisiblePeers, addedVisiblePeers := diffVisiblePeerSnapshots(currentVisiblePeers, targetVisiblePeers)

	return RelocationPreview{
		Applied:             false,
		Character:           connectedCharacterSnapshot(current),
		Target:              connectedCharacterSnapshot(target),
		CurrentVisiblePeers: currentVisiblePeers,
		TargetVisiblePeers:  targetVisiblePeers,
		RemovedVisiblePeers: removedVisiblePeers,
		AddedVisiblePeers:   addedVisiblePeers,
		MapOccupancyChanges: buildMapOccupancyChanges(buildMapOccupancySnapshots(characters), buildMapOccupancySnapshots(afterCharacters)),
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
		if id == originID || !charactersShareVisibleWorld(origin, session.character) {
			continue
		}
		session.pending.enqueue(frames)
	}
}

func (r *sharedWorldRegistry) EnqueueToOtherSessionsInEmpire(originID uint64, empire uint8, frames [][]byte) {
	if r == nil || originID == 0 || empire == 0 || len(frames) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for id, session := range r.sessions {
		if id == originID || session.character.Empire != empire {
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
		if id == originID || session.character.Empire != origin.Empire || !charactersShareVisibleWorld(origin, session.character) {
			continue
		}
		session.pending.enqueue(frames)
	}
}

func (r *sharedWorldRegistry) EnqueueToOtherSessionsInGuild(originID uint64, guildID uint32, frames [][]byte) {
	if r == nil || originID == 0 || guildID == 0 || len(frames) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for id, session := range r.sessions {
		if id == originID || session.character.GuildID != guildID {
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

func charactersShareVisibleWorld(left loginticket.Character, right loginticket.Character) bool {
	return characterMapIndex(left) == characterMapIndex(right)
}

func characterMapIndex(character loginticket.Character) uint32 {
	if character.MapIndex == 0 {
		return bootstrapMapIndex
	}
	return character.MapIndex
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

func connectedCharacterSnapshot(character loginticket.Character) ConnectedCharacterSnapshot {
	return ConnectedCharacterSnapshot{
		Name:     character.Name,
		VID:      character.VID,
		MapIndex: characterMapIndex(character),
		X:        character.X,
		Y:        character.Y,
		Empire:   character.Empire,
		GuildID:  character.GuildID,
	}
}

func buildVisiblePeerSnapshots(character loginticket.Character, characters []loginticket.Character, excludeVID uint32) []ConnectedCharacterSnapshot {
	visiblePeers := make([]ConnectedCharacterSnapshot, 0, len(characters))
	for _, peer := range characters {
		if peer.VID == excludeVID || !charactersShareVisibleWorld(character, peer) {
			continue
		}
		visiblePeers = append(visiblePeers, connectedCharacterSnapshot(peer))
	}
	sortConnectedCharacterSnapshots(visiblePeers)
	return visiblePeers
}

func buildMapOccupancySnapshots(characters []loginticket.Character) []MapOccupancySnapshot {
	byMap := make(map[uint32][]ConnectedCharacterSnapshot)
	for _, character := range characters {
		mapIndex := characterMapIndex(character)
		byMap[mapIndex] = append(byMap[mapIndex], connectedCharacterSnapshot(character))
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

func diffVisiblePeerSnapshots(current []ConnectedCharacterSnapshot, target []ConnectedCharacterSnapshot) ([]ConnectedCharacterSnapshot, []ConnectedCharacterSnapshot) {
	currentByVID := make(map[uint32]ConnectedCharacterSnapshot, len(current))
	for _, snapshot := range current {
		currentByVID[snapshot.VID] = snapshot
	}
	targetByVID := make(map[uint32]ConnectedCharacterSnapshot, len(target))
	for _, snapshot := range target {
		targetByVID[snapshot.VID] = snapshot
	}

	removed := make([]ConnectedCharacterSnapshot, 0)
	for vid, snapshot := range currentByVID {
		if _, ok := targetByVID[vid]; ok {
			continue
		}
		removed = append(removed, snapshot)
	}
	added := make([]ConnectedCharacterSnapshot, 0)
	for vid, snapshot := range targetByVID {
		if _, ok := currentByVID[vid]; ok {
			continue
		}
		added = append(added, snapshot)
	}
	sortConnectedCharacterSnapshots(removed)
	sortConnectedCharacterSnapshots(added)
	return removed, added
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
