package worldruntime

import (
	"math"
	"time"
)

// SpawnLeashStatus describes the first bootstrap-owned position-vs-authored-home
// classification for stationary spawn-backed combatants.
type SpawnLeashStatus string

const (
	SpawnLeashStatusAtHome         SpawnLeashStatus = "at_home"
	SpawnLeashStatusWithinRadius   SpawnLeashStatus = "within_radius"
	SpawnLeashStatusReturnRequired SpawnLeashStatus = "return_required"

	DefaultSpawnLeashRadius int32 = 400
	// DefaultSpawnAggroRadius is deliberately smaller than DefaultSpawnLeashRadius so a
	// player can enter proximity acquisition without immediately forcing leash/return
	// pressure at the outer leash boundary.
	DefaultSpawnAggroRadius int32 = 200
	// DefaultSpawnChaseDelay is the bootstrap chase arming / re-arm delay. It stays
	// longer than the owned 1s delayed retaliation beat so multi-beat hostility
	// remains independently observable before the first chase step.
	DefaultSpawnChaseDelay = 5 * time.Second
	// MaxSpawnChaseDelay is the bootstrap upper bound for optional authored
	// combat_profiles.chase_delay_ms on this Track A seam.
	MaxSpawnChaseDelay = 60 * time.Second
	// DefaultSpawnReturnDelay is the bootstrap return-step arming / re-arm delay.
	DefaultSpawnReturnDelay = time.Second
	// MinSpawnReturnDelay is the bootstrap lower bound for optional authored
	// combat_profiles.return_delay_ms so return cadence stays independently
	// observable beside the owned flush order.
	MinSpawnReturnDelay = 250 * time.Millisecond
	// MaxSpawnReturnDelay is the bootstrap upper bound for optional authored
	// combat_profiles.return_delay_ms on this Track A seam.
	MaxSpawnReturnDelay = 60 * time.Second
	// DefaultSpawnHomewardDelay is the bootstrap homeward-step arming / re-arm delay.
	DefaultSpawnHomewardDelay = time.Second
	// MinSpawnHomewardDelay is the bootstrap lower bound for optional authored
	// combat_profiles.homeward_delay_ms so homeward cadence stays independently
	// observable beside the owned flush order.
	MinSpawnHomewardDelay = 250 * time.Millisecond
	// MaxSpawnHomewardDelay is the bootstrap upper bound for optional authored
	// combat_profiles.homeward_delay_ms on this Track A seam.
	MaxSpawnHomewardDelay = 60 * time.Second
	// DefaultSpawnMaxStep is the bootstrap chase / return / homeward planner step
	// cap shared by the three executors.
	DefaultSpawnMaxStep int32 = 100
	// MinSpawnMaxStep is the bootstrap lower bound for optional authored
	// combat_profiles.max_step.
	MinSpawnMaxStep int32 = 1
	// MaxSpawnMaxStep is the bootstrap upper bound for optional authored
	// combat_profiles.max_step on this Track A seam.
	MaxSpawnMaxStep int32 = 1000
	// DefaultSpawnReactionDelay is the bootstrap delayed server-origin
	// retaliation arming / re-arm delay (matches
	// bootstrapPracticeMobServerOriginRetaliationDelay).
	DefaultSpawnReactionDelay = time.Second
	// MinSpawnReactionDelay is the bootstrap lower bound for optional authored
	// combat_profiles.reaction_delay_ms so reaction cadence stays independently
	// observable beside the owned flush order.
	MinSpawnReactionDelay = 250 * time.Millisecond
	// MaxSpawnReactionDelay is the bootstrap upper bound for optional authored
	// combat_profiles.reaction_delay_ms on this Track A seam.
	MaxSpawnReactionDelay = 60 * time.Second
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

// SpawnLeashReturnStepPlan is a pure planning result for the first future
// chase/return-home seam. It does not mutate actor state or emit packets.
type SpawnLeashReturnStepPlan struct {
	Evaluation SpawnLeashEvaluation
	Next       Position
	Complete   bool
}

// SpawnChaseStepPlan is a pure planning result for the first engaged chase-step
// seam. It does not mutate actor state or emit packets.
type SpawnChaseStepPlan struct {
	Evaluation SpawnLeashEvaluation
	Next       Position
	Complete   bool
}

// SpawnLeashHomewardStepPlan is a pure planning result for the first unengaged
// within-radius homeward seam after chase/engagement release. It does not
// mutate actor state or emit packets. return_required recovery stays owned by
// PlanStaticActorSpawnLeashReturnStep.
type SpawnLeashHomewardStepPlan struct {
	Evaluation SpawnLeashEvaluation
	Next       Position
	Complete   bool
}

// SpawnAggroAcquisitionEvaluation is a pure planning result for the first
// proximity aggro-radius acquisition seam. It does not mutate actor state,
// set engagement, arm retaliation/chase timers, or emit packets.
type SpawnAggroAcquisitionEvaluation struct {
	Current   Position
	Candidate Position
	Radius    int32
	Acquired  bool
}

// SpawnAggroCandidate is one live same-map player candidate for proximity
// aggro-radius acquisition selection.
type SpawnAggroCandidate struct {
	EntityID uint64
	Position Position
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

// PlanStaticActorSpawnLeashReturnStep computes one deterministic return-home
// step for a spawn-backed actor using its preserved authored home. Same-map
// actors move toward home by at most maxStep on the current bootstrap x/y plane;
// cross-map returns target authored home directly because no client warp/chase
// choreography is owned yet.
func PlanStaticActorSpawnLeashReturnStep(actor StaticEntity, radius int32, maxStep int32) (SpawnLeashReturnStepPlan, bool) {
	if maxStep <= 0 {
		return SpawnLeashReturnStepPlan{}, false
	}
	evaluation, ok := EvaluateStaticActorCurrentSpawnLeash(actor, radius)
	if !ok {
		return SpawnLeashReturnStepPlan{}, false
	}
	plan := SpawnLeashReturnStepPlan{Evaluation: evaluation, Next: evaluation.Current}
	if !evaluation.ReturnRequired {
		plan.Complete = true
		return plan, true
	}
	if !evaluation.Home.SameMap(evaluation.Current) {
		plan.Next = evaluation.Home
		plan.Complete = true
		return plan, true
	}
	next, complete := returnStepTowardHome(evaluation.Current, evaluation.Home, maxStep)
	plan.Next = next
	plan.Complete = complete
	return plan, true
}

// PlanStaticActorSpawnLeashHomewardStep computes one deterministic homeward step
// toward authored home for a live spawn-backed actor that currently classifies
// within_radius. at_home plans are complete no-ops; return_required and invalid
// inputs fail closed so outside-leash recovery stays with the return-step seam.
// The planner never mutates actor state.
func PlanStaticActorSpawnLeashHomewardStep(actor StaticEntity, radius int32, maxStep int32) (SpawnLeashHomewardStepPlan, bool) {
	if maxStep <= 0 {
		return SpawnLeashHomewardStepPlan{}, false
	}
	evaluation, ok := EvaluateStaticActorCurrentSpawnLeash(actor, radius)
	if !ok {
		return SpawnLeashHomewardStepPlan{}, false
	}
	if evaluation.ReturnRequired {
		return SpawnLeashHomewardStepPlan{}, false
	}
	plan := SpawnLeashHomewardStepPlan{Evaluation: evaluation, Next: evaluation.Current}
	if evaluation.Status == SpawnLeashStatusAtHome || evaluation.Current.Equal(evaluation.Home) {
		plan.Complete = true
		return plan, true
	}
	if evaluation.Status != SpawnLeashStatusWithinRadius || !evaluation.Home.SameMap(evaluation.Current) {
		return SpawnLeashHomewardStepPlan{}, false
	}
	next, complete := returnStepTowardHome(evaluation.Current, evaluation.Home, maxStep)
	plan.Next = next
	plan.Complete = complete
	return plan, true
}

// PlanStaticActorSpawnChaseStep computes one deterministic chase step toward an
// engaged owner's current position for a live spawn-backed actor that still
// classifies at_home or within_radius. Return-required actors, cross-map owners,
// and invalid inputs fail closed; the planner never mutates actor state.
func PlanStaticActorSpawnChaseStep(actor StaticEntity, owner Position, radius int32, maxStep int32) (SpawnChaseStepPlan, bool) {
	if maxStep <= 0 || !owner.Valid() {
		return SpawnChaseStepPlan{}, false
	}
	evaluation, ok := EvaluateStaticActorCurrentSpawnLeash(actor, radius)
	if !ok {
		return SpawnChaseStepPlan{}, false
	}
	if evaluation.ReturnRequired || !evaluation.Current.SameMap(owner) {
		return SpawnChaseStepPlan{}, false
	}
	plan := SpawnChaseStepPlan{Evaluation: evaluation, Next: evaluation.Current}
	if evaluation.Current.Equal(owner) {
		plan.Complete = true
		return plan, true
	}
	candidate, landedOnOwner := returnStepTowardHome(evaluation.Current, owner, maxStep)
	if positionWithinRadius(evaluation.Home, candidate, radius) {
		plan.Next = candidate
		plan.Complete = landedOnOwner
		return plan, true
	}
	plan.Next = farthestPointOnSegmentInsideLeash(evaluation.Home, evaluation.Current, candidate, radius)
	plan.Complete = true
	return plan, true
}

// EvaluateStaticActorSpawnAggroAcquisition decides whether one candidate position
// is inside the aggro radius of a live spawn-backed actor that still classifies
// at_home or within_radius. Return-required actors, cross-map candidates, and
// invalid inputs fail closed. The helper never mutates actor state.
func EvaluateStaticActorSpawnAggroAcquisition(actor StaticEntity, candidate Position, radius int32) (SpawnAggroAcquisitionEvaluation, bool) {
	if radius <= 0 || !candidate.Valid() {
		return SpawnAggroAcquisitionEvaluation{}, false
	}
	evaluation, ok := EvaluateStaticActorCurrentSpawnLeash(actor, EffectiveStaticActorSpawnLeashRadiusForActor(actor))
	if !ok {
		return SpawnAggroAcquisitionEvaluation{}, false
	}
	result := SpawnAggroAcquisitionEvaluation{
		Current:   evaluation.Current,
		Candidate: candidate,
		Radius:    radius,
	}
	if evaluation.ReturnRequired || !evaluation.Current.SameMap(candidate) {
		return result, true
	}
	result.Acquired = positionWithinRadius(evaluation.Current, candidate, radius)
	return result, true
}

// SelectStaticActorSpawnAggroCandidate chooses the nearest eligible same-map
// candidate inside the aggro radius, breaking ties by ascending entity ID.
// Candidates with EntityID == 0 or invalid positions are ignored. The helper
// never mutates actor state.
func SelectStaticActorSpawnAggroCandidate(actor StaticEntity, candidates []SpawnAggroCandidate, radius int32) (SpawnAggroCandidate, bool) {
	best := SpawnAggroCandidate{}
	bestDistance := int64(-1)
	found := false
	for _, candidate := range candidates {
		if candidate.EntityID == 0 || !candidate.Position.Valid() {
			continue
		}
		evaluation, ok := EvaluateStaticActorSpawnAggroAcquisition(actor, candidate.Position, radius)
		if !ok || !evaluation.Acquired {
			continue
		}
		distance := squaredDistanceXY(evaluation.Current, candidate.Position)
		if !found || distance < bestDistance || (distance == bestDistance && candidate.EntityID < best.EntityID) {
			best = candidate
			bestDistance = distance
			found = true
		}
	}
	return best, found
}

func squaredDistanceXY(left Position, right Position) int64 {
	dx := int64(left.X) - int64(right.X)
	dy := int64(left.Y) - int64(right.Y)
	return dx*dx + dy*dy
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

func returnStepTowardHome(current Position, home Position, maxStep int32) (Position, bool) {
	if !current.SameMap(home) {
		return home, true
	}
	dx := int64(home.X) - int64(current.X)
	dy := int64(home.Y) - int64(current.Y)
	distance := math.Hypot(float64(dx), float64(dy))
	if distance == 0 || distance <= float64(maxStep) {
		return home, true
	}
	scale := float64(maxStep) / distance
	xStep := int64(float64(dx) * scale)
	yStep := int64(float64(dy) * scale)
	if xStep == 0 && yStep == 0 {
		if absInt64(dx) >= absInt64(dy) {
			xStep = signInt64(dx)
		} else {
			yStep = signInt64(dy)
		}
	}
	nextX := int64(current.X) + xStep
	nextY := int64(current.Y) + yStep
	nextX = clampStepCoordinate(nextX, int64(current.X), int64(home.X))
	nextY = clampStepCoordinate(nextY, int64(current.Y), int64(home.Y))
	next := NewPosition(current.MapIndex, int32(nextX), int32(nextY))
	return next, next.Equal(home)
}

func farthestPointOnSegmentInsideLeash(home Position, current Position, toward Position, radius int32) Position {
	if !current.SameMap(toward) || !home.SameMap(current) || radius <= 0 {
		return current
	}
	if positionWithinRadius(home, toward, radius) {
		return toward
	}
	if !positionWithinRadius(home, current, radius) {
		return current
	}
	dx := int64(toward.X) - int64(current.X)
	dy := int64(toward.Y) - int64(current.Y)
	distance := math.Hypot(float64(dx), float64(dy))
	if distance == 0 {
		return current
	}
	lo, hi := 0.0, distance
	best := current
	for i := 0; i < 48; i++ {
		mid := (lo + hi) / 2
		scale := mid / distance
		nextX := int64(current.X) + int64(float64(dx)*scale)
		nextY := int64(current.Y) + int64(float64(dy)*scale)
		nextX = clampStepCoordinate(nextX, int64(current.X), int64(toward.X))
		nextY = clampStepCoordinate(nextY, int64(current.Y), int64(toward.Y))
		candidate := NewPosition(current.MapIndex, int32(nextX), int32(nextY))
		if positionWithinRadius(home, candidate, radius) {
			best = candidate
			lo = mid
			continue
		}
		hi = mid
	}
	return best
}

func clampStepCoordinate(value int64, current int64, home int64) int64 {
	if current < home && value > home {
		return home
	}
	if current > home && value < home {
		return home
	}
	return value
}

func signInt64(value int64) int64 {
	if value < 0 {
		return -1
	}
	if value > 0 {
		return 1
	}
	return 0
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
