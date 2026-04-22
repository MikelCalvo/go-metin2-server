package worldruntime

import "github.com/MikelCalvo/go-metin2-server/internal/loginticket"

type Position struct {
	MapIndex uint32
	X        int32
	Y        int32
}

func NewPosition(mapIndex uint32, x int32, y int32) Position {
	return Position{MapIndex: mapIndex, X: x, Y: y}
}

func PositionFromCharacter(character loginticket.Character) Position {
	return NewPosition(character.MapIndex, character.X, character.Y)
}

func (p Position) Valid() bool {
	return p.MapIndex != 0
}

func (p Position) Equal(other Position) bool {
	return p == other
}

func (p Position) SameMap(other Position) bool {
	return p.Valid() && other.Valid() && p.MapIndex == other.MapIndex
}
