package player

import "github.com/MikelCalvo/go-metin2-server/internal/loginticket"

type SessionLink struct {
	Login          string
	CharacterIndex uint8
}

type Runtime struct {
	persisted    loginticket.Character
	liveMapIndex uint32
	liveX        int32
	liveY        int32
	sessionLink  SessionLink
}

func NewRuntime(persisted loginticket.Character, sessionLink SessionLink) *Runtime {
	return &Runtime{
		persisted:    persisted,
		liveMapIndex: persisted.MapIndex,
		liveX:        persisted.X,
		liveY:        persisted.Y,
		sessionLink:  sessionLink,
	}
}

func (r *Runtime) PersistedSnapshot() loginticket.Character {
	if r == nil {
		return loginticket.Character{}
	}
	return r.persisted
}

func (r *Runtime) LiveCharacter() loginticket.Character {
	if r == nil {
		return loginticket.Character{}
	}
	live := r.PersistedSnapshot()
	live.MapIndex = r.liveMapIndex
	live.X = r.liveX
	live.Y = r.liveY
	return live
}

func (r *Runtime) SetLivePosition(mapIndex uint32, x int32, y int32) {
	if r == nil {
		return
	}
	r.liveMapIndex = mapIndex
	r.liveX = x
	r.liveY = y
}

func (r *Runtime) SessionLink() SessionLink {
	if r == nil {
		return SessionLink{}
	}
	return r.sessionLink
}
