package worldruntime

import "github.com/MikelCalvo/go-metin2-server/internal/loginticket"

// ExchangeMaxDistance mirrors the legacy EXCHANGE_MAX_DISTANCE gate used by
// trade-start and walk-away cancel. Distances are compared with ApproxDistance.
const ExchangeMaxDistance = 1000

// ApproxDistance mirrors the legacy DISTANCE_APPROX helper: an octile-style
// approximation of Euclidean length that avoids floating-point math.
func ApproxDistance(dx int32, dy int32) int32 {
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	min, max := dx, dy
	if dx < dy {
		min, max = dx, dy
	} else {
		min, max = dy, dx
	}
	// coefficients equivalent to (123/128 * max) and (51/128 * min)
	return ((max << 8) + (max << 3) - (max << 4) - (max << 1) +
		(min << 7) - (min << 5) + (min << 3) - (min << 1)) >> 8
}

// WithinExchangeDistance reports whether two characters share an effective map
// and remain strictly inside the owned exchange-distance gate.
func WithinExchangeDistance(left loginticket.Character, right loginticket.Character) bool {
	if effectiveMapIndex(left.MapIndex) != effectiveMapIndex(right.MapIndex) {
		return false
	}
	return ApproxDistance(left.X-right.X, left.Y-right.Y) < ExchangeMaxDistance
}
