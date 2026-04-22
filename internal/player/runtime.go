package player

import (
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

type SessionLink struct {
	Login          string
	CharacterIndex uint8
}

type Runtime struct {
	persisted   loginticket.Character
	live        worldruntime.Position
	sessionLink SessionLink
}

func NewRuntime(persisted loginticket.Character, sessionLink SessionLink) *Runtime {
	return &Runtime{
		persisted:   persisted,
		live:        worldruntime.PositionFromCharacter(persisted),
		sessionLink: sessionLink,
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
	live.MapIndex = r.live.MapIndex
	live.X = r.live.X
	live.Y = r.live.Y
	return live
}

func (r *Runtime) LivePosition() worldruntime.Position {
	if r == nil {
		return worldruntime.Position{}
	}
	return r.live
}

func (r *Runtime) SetLivePosition(mapIndex uint32, x int32, y int32) {
	if r == nil {
		return
	}
	r.live = worldruntime.NewPosition(mapIndex, x, y)
}

func (r *Runtime) ApplyPersistedSnapshot(persisted loginticket.Character) {
	if r == nil {
		return
	}
	r.persisted = persisted
	r.live = worldruntime.PositionFromCharacter(persisted)
}

func (r *Runtime) SessionLink() SessionLink {
	if r == nil {
		return SessionLink{}
	}
	return r.sessionLink
}
