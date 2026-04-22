package worldruntime

import "github.com/MikelCalvo/go-metin2-server/internal/loginticket"

type EntityKind string

const EntityKindPlayer EntityKind = "player"

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
