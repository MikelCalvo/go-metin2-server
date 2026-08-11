package worldruntime

// SpawnLeashStatus describes the first bootstrap-owned position-vs-authored-home
// classification for stationary spawn-backed combatants.
type SpawnLeashStatus string

const (
	SpawnLeashStatusAtHome         SpawnLeashStatus = "at_home"
	SpawnLeashStatusWithinRadius   SpawnLeashStatus = "within_radius"
	SpawnLeashStatusReturnRequired SpawnLeashStatus = "return_required"

	DefaultSpawnLeashRadius int32 = 400
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

type PositionSnapshot struct {
	MapIndex uint32 `json:"map_index"`
	X        int32  `json:"x"`
	Y        int32  `json:"y"`
}

type SpawnLeashSnapshot struct {
	Home           PositionSnapshot  `json:"home"`
	Current        PositionSnapshot  `json:"current"`
	Radius         int32             `json:"radius"`
	Status         SpawnLeashStatus  `json:"status"`
	ReturnRequired bool              `json:"return_required"`
	ReturnTarget   *PositionSnapshot `json:"return_target,omitempty"`
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

// EvaluateStaticActorCurrentSpawnLeash classifies a spawn-backed static actor's
// current position against its preserved authored home position.
func EvaluateStaticActorCurrentSpawnLeash(actor StaticEntity, radius int32) (SpawnLeashEvaluation, bool) {
	profile := staticActorCombatProfile(actor.CombatProfile, actor.CombatKind)
	if actor.SpawnGroupRef == "" || !ValidStaticActorSpawnGroupRef(actor.SpawnGroupRef) || profile == "" || !ValidStaticActorCombatKind(profile) {
		return SpawnLeashEvaluation{}, false
	}
	home := actor.SpawnHome
	if !home.Valid() {
		home = actor.Position
	}
	return EvaluateSpawnLeash(home, actor.Position, radius)
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

func SpawnLeashSnapshotFromEvaluation(evaluation SpawnLeashEvaluation) SpawnLeashSnapshot {
	snapshot := SpawnLeashSnapshot{
		Home:           PositionSnapshotFromPosition(evaluation.Home),
		Current:        PositionSnapshotFromPosition(evaluation.Current),
		Radius:         evaluation.Radius,
		Status:         evaluation.Status,
		ReturnRequired: evaluation.ReturnRequired,
	}
	if evaluation.ReturnRequired {
		returnTarget := PositionSnapshotFromPosition(evaluation.ReturnTarget)
		snapshot.ReturnTarget = &returnTarget
	}
	return snapshot
}

func PositionSnapshotFromPosition(position Position) PositionSnapshot {
	return PositionSnapshot{MapIndex: position.MapIndex, X: position.X, Y: position.Y}
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
