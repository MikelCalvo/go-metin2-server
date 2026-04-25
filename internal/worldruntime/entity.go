package worldruntime

import "github.com/MikelCalvo/go-metin2-server/internal/loginticket"

type EntityKind string

const (
	EntityKindPlayer      EntityKind = "player"
	EntityKindStaticActor EntityKind = "static_actor"
)

type Entity struct {
	ID   uint64
	Kind EntityKind
	VID  uint32
	Name string
}

type PlayerEntity struct {
	Entity    Entity
	Character loginticket.Character
}

func (p PlayerEntity) Position() Position {
	return PositionFromCharacter(p.Character)
}

type StaticEntity struct {
	Entity          Entity
	Position        Position
	RaceNum         uint32
	InteractionKind string
	InteractionRef  string
}

func StaticActorVisibilityVID(actor StaticEntity) (uint32, bool) {
	if actor.Entity.ID == 0 || actor.Entity.ID > uint64(^uint32(0)) || actor.RaceNum > uint32(^uint16(0)) {
		return 0, false
	}
	return uint32(actor.Entity.ID), true
}
