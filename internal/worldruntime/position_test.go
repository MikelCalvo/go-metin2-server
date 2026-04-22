package worldruntime

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestPositionTreatsZeroMapIndexAsInvalid(t *testing.T) {
	position := NewPosition(0, 1100, 2100)
	if position.Valid() {
		t.Fatalf("expected zero-map position to be invalid, got %+v", position)
	}
}

func TestPositionEqualAndSameMap(t *testing.T) {
	left := NewPosition(42, 1700, 2800)
	same := NewPosition(42, 1700, 2800)
	otherCoords := NewPosition(42, 1750, 2850)
	otherMap := NewPosition(43, 1700, 2800)

	if !left.Equal(same) {
		t.Fatalf("expected equal positions to compare equal, got left=%+v right=%+v", left, same)
	}
	if left.Equal(otherCoords) {
		t.Fatalf("expected coordinate difference to break equality, got left=%+v right=%+v", left, otherCoords)
	}
	if !left.SameMap(otherCoords) {
		t.Fatalf("expected same-map positions to report SameMap, got left=%+v right=%+v", left, otherCoords)
	}
	if left.SameMap(otherMap) {
		t.Fatalf("expected different-map positions to report different maps, got left=%+v right=%+v", left, otherMap)
	}
}

func TestPositionFromCharacterCopiesMapAndCoordinates(t *testing.T) {
	character := loginticket.Character{MapIndex: 42, X: 1700, Y: 2800}
	if got := PositionFromCharacter(character); got != NewPosition(42, 1700, 2800) {
		t.Fatalf("expected character position snapshot, got %+v", got)
	}
}
