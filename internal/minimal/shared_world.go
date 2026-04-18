package minimal

import (
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

func (r *sharedWorldRegistry) Join(character loginticket.Character, pending *pendingServerFrames) (uint64, []loginticket.Character) {
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
	r.sessions[id] = sharedWorldSession{character: character, pending: pending}
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

	removeRaw := worldproto.EncodeCharacterDeleteNotice(worldproto.CharacterDeleteNoticePacket{VID: session.character.VID})
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
