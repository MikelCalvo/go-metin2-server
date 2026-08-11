package worldruntime

// SpawnLeashStatus describes the first bootstrap-owned position-vs-authored-home
// classification for stationary spawn-backed combatants.
type SpawnLeashStatus string

const (
	SpawnLeashStatusAtHome         SpawnLeashStatus = "at_home"
	SpawnLeashStatusWithinRadius   SpawnLeashStatus = "within_radius"
	SpawnLeashStatusReturnRequired SpawnLeashStatus = "return_required"
)

// SpawnLeashEvaluation is a pure planning result for the first mob lifecycle
// leash seam. It does not move actors, emit packets, or start timers.
type SpawnLeashEvaluation struct {
	Home           Position
	Current        Position
	Radius         int32
	Status         SpawnLeashStatus
	ReturnRequired bool
	ReturnTarget   Position
}

// EvaluateStaticActorSpawnLeash classifies a spawn-backed static actor's current
// position against its authored spawn position.
func EvaluateStaticActorSpawnLeash(actor StaticEntity, current Position, radius int32) (SpawnLeashEvaluation, bool) {
	profile := staticActorCombatProfile(actor.CombatProfile, actor.CombatKind)
	if actor.SpawnGroupRef == "" || !ValidStaticActorSpawnGroupRef(actor.SpawnGroupRef) || profile == "" || !ValidStaticActorCombatKind(profile) {
		return SpawnLeashEvaluation{}, false
	}
	return EvaluateSpawnLeash(actor.Position, current, radius)
}

// EvaluateSpawnLeash classifies the current position against one authored home
// position. The first policy is intentionally tiny: same-map positions inside
// the radius remain live in-place, while cross-map or out-of-radius positions
// require a return to the authored home position.
func EvaluateSpawnLeash(home Position, current Position, radius int32) (SpawnLeashEvaluation, bool) {
	if !home.Valid() || !current.Valid() || radius <= 0 {
		return SpawnLeashEvaluation{}, false
	}
	evaluation := SpawnLeashEvaluation{
		Home:         home,
		Current:      current,
		Radius:       radius,
		ReturnTarget: home,
	}
	if home.Equal(current) {
		evaluation.Status = SpawnLeashStatusAtHome
		return evaluation, true
	}
	if !home.SameMap(current) || !positionWithinRadius(home, current, radius) {
		evaluation.Status = SpawnLeashStatusReturnRequired
		evaluation.ReturnRequired = true
		return evaluation, true
	}
	evaluation.Status = SpawnLeashStatusWithinRadius
	return evaluation, true
}

func positionWithinRadius(left Position, right Position, radius int32) bool {
	if radius < 0 {
		return false
	}
	dx := int64(left.X) - int64(right.X)
	dy := int64(left.Y) - int64(right.Y)
	limit := int64(radius)
	if absInt64(dx) > limit || absInt64(dy) > limit {
		return false
	}
	return dx*dx+dy*dy <= limit*limit
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
