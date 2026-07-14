package worldruntime

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

type EntityKind string

const (
	EntityKindPlayer      EntityKind = "player"
	EntityKindStaticActor EntityKind = "static_actor"

	StaticActorCombatKindTrainingDummy    = "training_dummy"
	StaticActorCombatProfileTrainingDummy = StaticActorCombatKindTrainingDummy
	StaticActorCombatProfilePracticeMob   = "practice_mob"

	TrainingDummyBootstrapMaxHP                 uint8  = 10
	TrainingDummyBootstrapMinLiveHP             uint8  = 1
	TrainingDummyBootstrapDamagePerNormalAttack uint8  = 1
	TrainingDummyBootstrapAttackValue           uint16 = 1
	TrainingDummyBootstrapDefenseValue          uint16 = 0
	TrainingDummyBootstrapLevel                 uint16 = 1
	TrainingDummyBootstrapRank                  uint8  = 0
	TrainingDummyBootstrapRespawnDelay                 = 2 * time.Second

	PracticeMobBootstrapMaxHP                 uint8  = 10
	PracticeMobBootstrapDamagePerNormalAttack uint8  = 1
	PracticeMobBootstrapAttackValue           uint16 = 1
	PracticeMobBootstrapDefenseValue          uint16 = 0
	PracticeMobBootstrapLevel                 uint16 = 1
	PracticeMobBootstrapRank                  uint8  = 0
	PracticeMobBootstrapRespawnDelay                 = 2 * time.Second
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
	CombatProfile   string
	InteractionKind string
	InteractionRef  string
	SpawnGroupRef   string
	CombatKind      string
	DeathReward     StaticActorDeathReward
}

type StaticActorDeathReward struct {
	Experience uint64   `json:"experience"`
	Gold       uint64   `json:"gold"`
	DropVnums  []uint32 `json:"drop_vnums"`
}

func (r StaticActorDeathReward) Empty() bool {
	return r.Experience == 0 && r.Gold == 0 && len(r.DropVnums) == 0
}

func (r StaticActorDeathReward) Clone() StaticActorDeathReward {
	cloned := StaticActorDeathReward{Experience: r.Experience, Gold: r.Gold}
	if len(r.DropVnums) > 0 {
		cloned.DropVnums = append([]uint32(nil), r.DropVnums...)
		sort.Slice(cloned.DropVnums, func(i int, j int) bool {
			return cloned.DropVnums[i] < cloned.DropVnums[j]
		})
		write := 0
		for _, vnum := range cloned.DropVnums {
			if write > 0 && cloned.DropVnums[write-1] == vnum {
				continue
			}
			cloned.DropVnums[write] = vnum
			write++
		}
		cloned.DropVnums = cloned.DropVnums[:write]
	}
	return cloned
}

func ValidStaticActorDeathReward(reward StaticActorDeathReward) bool {
	maxPointCarrier := uint64(^uint32(0) >> 1)
	if reward.Experience > maxPointCarrier || reward.Gold > maxPointCarrier {
		return false
	}
	seenDropVnums := make(map[uint32]struct{}, len(reward.DropVnums))
	for _, vnum := range reward.DropVnums {
		if vnum == 0 {
			return false
		}
		if _, exists := seenDropVnums[vnum]; exists {
			return false
		}
		seenDropVnums[vnum] = struct{}{}
	}
	return true
}

type StaticActorCombatProfileDefaults struct {
	MaxHP                 uint8
	DamagePerNormalAttack uint8
	AttackValue           uint16
	DefenseValue          uint16
	Level                 uint16
	Rank                  uint8
	RespawnDelay          time.Duration
	DeathReward           StaticActorDeathReward
}

type StaticActorCombatProfileSnapshot struct {
	Profile               string                 `json:"profile"`
	MaxHP                 uint8                  `json:"max_hp"`
	DamagePerNormalAttack uint8                  `json:"damage_per_normal_attack"`
	AttackValue           uint16                 `json:"attack_value"`
	DefenseValue          uint16                 `json:"defense_value"`
	Level                 uint16                 `json:"level"`
	Rank                  uint8                  `json:"rank"`
	RespawnDelayMs        int64                  `json:"respawn_delay_ms"`
	DeathReward           StaticActorDeathReward `json:"death_reward"`
}

var staticActorCombatProfileRegistry = struct {
	sync.RWMutex
	profiles map[string]StaticActorCombatProfileDefaults
}{profiles: make(map[string]StaticActorCombatProfileDefaults)}

func RegisterStaticActorCombatProfile(profile string, defaults StaticActorCombatProfileDefaults) bool {
	if !validStaticActorCombatProfileName(profile) {
		return false
	}
	if !ValidStaticActorDeathReward(defaults.DeathReward) {
		return false
	}
	hasLegacyDamage := defaults.DamagePerNormalAttack != 0
	hasExplicitFormula := defaults.AttackValue != 0
	if hasLegacyDamage && !hasExplicitFormula && uint32(defaults.DamagePerNormalAttack)+uint32(defaults.DefenseValue) > uint32(^uint16(0)) {
		return false
	}
	defaults = cloneStaticActorCombatProfileDefaults(defaults)
	if hasLegacyDamage && hasExplicitFormula && defaults.DamagePerNormalAttack != bootstrapStaticActorNormalAttackDamage(defaults) {
		return false
	}
	if defaults.AttackValue > defaults.DefenseValue && defaults.AttackValue-defaults.DefenseValue > uint16(defaults.MaxHP) {
		return false
	}
	if profile == StaticActorCombatKindTrainingDummy || profile == StaticActorCombatProfilePracticeMob || defaults.MaxHP == 0 || (!hasLegacyDamage && !hasExplicitFormula) || defaults.AttackValue == 0 || defaults.DamagePerNormalAttack == 0 || defaults.DamagePerNormalAttack > defaults.MaxHP || defaults.RespawnDelay <= 0 {
		return false
	}
	staticActorCombatProfileRegistry.Lock()
	defer staticActorCombatProfileRegistry.Unlock()
	if _, exists := staticActorCombatProfileRegistry.profiles[profile]; exists {
		return false
	}
	staticActorCombatProfileRegistry.profiles[profile] = defaults
	return true
}

func UnregisterStaticActorCombatProfile(profile string) {
	staticActorCombatProfileRegistry.Lock()
	defer staticActorCombatProfileRegistry.Unlock()
	delete(staticActorCombatProfileRegistry.profiles, strings.TrimSpace(profile))
}

func UnregisterStaticActorCombatProfileForTest(profile string) {
	UnregisterStaticActorCombatProfile(profile)
}

func StaticActorCombatProfileSnapshots() []StaticActorCombatProfileSnapshot {
	snapshots := []StaticActorCombatProfileSnapshot{
		staticActorCombatProfileSnapshot(StaticActorCombatProfilePracticeMob, StaticActorCombatProfileDefaults{
			MaxHP:                 PracticeMobBootstrapMaxHP,
			DamagePerNormalAttack: PracticeMobBootstrapDamagePerNormalAttack,
			AttackValue:           PracticeMobBootstrapAttackValue,
			DefenseValue:          PracticeMobBootstrapDefenseValue,
			Level:                 PracticeMobBootstrapLevel,
			Rank:                  PracticeMobBootstrapRank,
			RespawnDelay:          PracticeMobBootstrapRespawnDelay,
		}),
		staticActorCombatProfileSnapshot(StaticActorCombatProfileTrainingDummy, StaticActorCombatProfileDefaults{
			MaxHP:                 TrainingDummyBootstrapMaxHP,
			DamagePerNormalAttack: TrainingDummyBootstrapDamagePerNormalAttack,
			AttackValue:           TrainingDummyBootstrapAttackValue,
			DefenseValue:          TrainingDummyBootstrapDefenseValue,
			Level:                 TrainingDummyBootstrapLevel,
			Rank:                  TrainingDummyBootstrapRank,
			RespawnDelay:          TrainingDummyBootstrapRespawnDelay,
		}),
	}
	staticActorCombatProfileRegistry.RLock()
	for profile, defaults := range staticActorCombatProfileRegistry.profiles {
		snapshots = append(snapshots, staticActorCombatProfileSnapshot(profile, cloneStaticActorCombatProfileDefaults(defaults)))
	}
	staticActorCombatProfileRegistry.RUnlock()
	sort.Slice(snapshots, func(i int, j int) bool {
		return snapshots[i].Profile < snapshots[j].Profile
	})
	return snapshots
}

func staticActorCombatProfileSnapshot(profile string, defaults StaticActorCombatProfileDefaults) StaticActorCombatProfileSnapshot {
	defaults = cloneStaticActorCombatProfileDefaults(defaults)
	return StaticActorCombatProfileSnapshot{
		Profile:               profile,
		MaxHP:                 defaults.MaxHP,
		DamagePerNormalAttack: defaults.DamagePerNormalAttack,
		AttackValue:           defaults.AttackValue,
		DefenseValue:          defaults.DefenseValue,
		Level:                 defaults.Level,
		Rank:                  defaults.Rank,
		RespawnDelayMs:        defaults.RespawnDelay.Milliseconds(),
		DeathReward:           defaults.DeathReward.Clone(),
	}
}

func validStaticActorCombatProfileName(profile string) bool {
	if profile == "" || profile != strings.TrimSpace(profile) {
		return false
	}
	first := profile[0]
	if first < 'a' || first > 'z' {
		return false
	}
	for i := 1; i < len(profile); i++ {
		c := profile[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return false
	}
	return true
}

func cloneStaticActorCombatProfileDefaults(defaults StaticActorCombatProfileDefaults) StaticActorCombatProfileDefaults {
	if defaults.AttackValue == 0 && defaults.DamagePerNormalAttack != 0 {
		defaults.AttackValue = uint16(defaults.DamagePerNormalAttack) + defaults.DefenseValue
	}
	if defaults.DamagePerNormalAttack == 0 && defaults.MaxHP > 0 {
		defaults.DamagePerNormalAttack = bootstrapStaticActorNormalAttackDamage(defaults)
	}
	if defaults.Level == 0 {
		defaults.Level = TrainingDummyBootstrapLevel
	}
	defaults.DeathReward = defaults.DeathReward.Clone()
	return defaults
}

func staticActorCombatProfile(profile string, kind string) string {
	if profile != "" {
		return profile
	}
	return kind
}

func normalizeStaticEntityCombat(actor StaticEntity) StaticEntity {
	profile := staticActorCombatProfile(actor.CombatProfile, actor.CombatKind)
	actor.CombatProfile = profile
	actor.CombatKind = profile
	actor.SpawnGroupRef = strings.TrimSpace(actor.SpawnGroupRef)
	return actor
}

func StaticActorVisibilityVID(actor StaticEntity) (uint32, bool) {
	if actor.Entity.ID == 0 || actor.Entity.ID > uint64(^uint32(0)) || actor.RaceNum > uint32(^uint16(0)) {
		return 0, false
	}
	return uint32(actor.Entity.ID), true
}

func BootstrapStaticActorCombatProfileDefaults(combatKind string) (StaticActorCombatProfileDefaults, bool) {
	switch combatKind {
	case StaticActorCombatKindTrainingDummy:
		return StaticActorCombatProfileDefaults{
			MaxHP:                 TrainingDummyBootstrapMaxHP,
			DamagePerNormalAttack: TrainingDummyBootstrapDamagePerNormalAttack,
			AttackValue:           TrainingDummyBootstrapAttackValue,
			DefenseValue:          TrainingDummyBootstrapDefenseValue,
			Level:                 TrainingDummyBootstrapLevel,
			Rank:                  TrainingDummyBootstrapRank,
			RespawnDelay:          TrainingDummyBootstrapRespawnDelay,
			DeathReward:           StaticActorDeathReward{},
		}, true
	case StaticActorCombatProfilePracticeMob:
		return StaticActorCombatProfileDefaults{
			MaxHP:                 PracticeMobBootstrapMaxHP,
			DamagePerNormalAttack: PracticeMobBootstrapDamagePerNormalAttack,
			AttackValue:           PracticeMobBootstrapAttackValue,
			DefenseValue:          PracticeMobBootstrapDefenseValue,
			Level:                 PracticeMobBootstrapLevel,
			Rank:                  PracticeMobBootstrapRank,
			RespawnDelay:          PracticeMobBootstrapRespawnDelay,
			DeathReward:           StaticActorDeathReward{},
		}, true
	default:
		staticActorCombatProfileRegistry.RLock()
		defer staticActorCombatProfileRegistry.RUnlock()
		defaults, ok := staticActorCombatProfileRegistry.profiles[combatKind]
		if !ok {
			return StaticActorCombatProfileDefaults{}, false
		}
		return cloneStaticActorCombatProfileDefaults(defaults), true
	}
}

func BootstrapStaticActorCurrentHP(combatKind string) (uint8, bool) {
	defaults, ok := BootstrapStaticActorCombatProfileDefaults(combatKind)
	if !ok {
		return 0, false
	}
	return defaults.MaxHP, true
}

func BootstrapStaticActorRespawnDelay(combatKind string) (time.Duration, bool) {
	defaults, ok := BootstrapStaticActorCombatProfileDefaults(combatKind)
	if !ok {
		return 0, false
	}
	return defaults.RespawnDelay, true
}

func BootstrapStaticActorDeathReward(combatKind string) (StaticActorDeathReward, bool) {
	defaults, ok := BootstrapStaticActorCombatProfileDefaults(combatKind)
	if !ok {
		return StaticActorDeathReward{}, false
	}
	return defaults.DeathReward.Clone(), true
}

func BootstrapStaticActorHPPercent(combatKind string, currentHP uint8) (uint8, bool) {
	defaults, ok := BootstrapStaticActorCombatProfileDefaults(combatKind)
	if !ok {
		return 0, false
	}
	return bootstrapStaticActorHPPercent(currentHP, defaults.MaxHP), true
}

func ApplyBootstrapStaticActorNormalAttack(combatKind string, currentHP uint8) (uint8, uint8, bool) {
	defaults, ok := BootstrapStaticActorCombatProfileDefaults(combatKind)
	if !ok || currentHP == 0 {
		return 0, 0, false
	}
	if currentHP > defaults.MaxHP {
		currentHP = defaults.MaxHP
	}
	damage := bootstrapStaticActorNormalAttackDamage(defaults)
	nextHP := currentHP
	if nextHP <= damage {
		nextHP = 0
	} else {
		nextHP -= damage
	}
	hpPercent := bootstrapStaticActorHPPercent(nextHP, defaults.MaxHP)
	return nextHP, hpPercent, true
}

func bootstrapStaticActorNormalAttackDamage(defaults StaticActorCombatProfileDefaults) uint8 {
	if defaults.AttackValue <= defaults.DefenseValue {
		return 1
	}
	damage := defaults.AttackValue - defaults.DefenseValue
	if damage == 0 {
		return 1
	}
	if damage > uint16(defaults.MaxHP) {
		return defaults.MaxHP
	}
	return uint8(damage)
}

func bootstrapStaticActorHPPercent(currentHP uint8, maxHP uint8) uint8 {
	if maxHP == 0 {
		return 0
	}
	if currentHP > maxHP {
		currentHP = maxHP
	}
	percent := (int(currentHP) * 100) / int(maxHP)
	if currentHP > 0 && percent == 0 {
		percent = 1
	}
	if percent > 100 {
		percent = 100
	}
	return uint8(percent)
}
