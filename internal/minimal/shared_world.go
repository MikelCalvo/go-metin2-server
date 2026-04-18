package minimal

import (
	"sync"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
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
