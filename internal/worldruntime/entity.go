package worldruntime

import (
	"strings"
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

	TrainingDummyBootstrapMaxHP                 uint8 = 10
	TrainingDummyBootstrapMinLiveHP             uint8 = 1
	TrainingDummyBootstrapDamagePerNormalAttack uint8 = 1
	TrainingDummyBootstrapRespawnDelay                = 2 * time.Second

	PracticeMobBootstrapMaxHP                 uint8 = 10
	PracticeMobBootstrapDamagePerNormalAttack uint8 = 1
	PracticeMobBootstrapRespawnDelay                = 2 * time.Second
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
}

type StaticActorDeathReward struct {
	Experience uint64
	Gold       uint64
	DropVnums  []uint32
}

func (r StaticActorDeathReward) Empty() bool {
	return r.Experience == 0 && r.Gold == 0 && len(r.DropVnums) == 0
}

func (r StaticActorDeathReward) Clone() StaticActorDeathReward {
	cloned := StaticActorDeathReward{Experience: r.Experience, Gold: r.Gold}
	if len(r.DropVnums) > 0 {
		cloned.DropVnums = append([]uint32(nil), r.DropVnums...)
	}
	return cloned
}

type StaticActorCombatProfileDefaults struct {
	MaxHP                 uint8
	DamagePerNormalAttack uint8
	RespawnDelay          time.Duration
	DeathReward           StaticActorDeathReward
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
			RespawnDelay:          TrainingDummyBootstrapRespawnDelay,
			DeathReward:           StaticActorDeathReward{},
		}, true
	case StaticActorCombatProfilePracticeMob:
		return StaticActorCombatProfileDefaults{
			MaxHP:                 PracticeMobBootstrapMaxHP,
			DamagePerNormalAttack: PracticeMobBootstrapDamagePerNormalAttack,
			RespawnDelay:          PracticeMobBootstrapRespawnDelay,
			DeathReward:           StaticActorDeathReward{},
		}, true
	default:
		return StaticActorCombatProfileDefaults{}, false
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
	switch combatKind {
	case StaticActorCombatKindTrainingDummy:
		return bootstrapStaticActorHPPercent(currentHP, TrainingDummyBootstrapMaxHP), true
	default:
		return 0, false
	}
}

func ApplyBootstrapStaticActorNormalAttack(combatKind string, currentHP uint8) (uint8, uint8, bool) {
	defaults, ok := BootstrapStaticActorCombatProfileDefaults(combatKind)
	if !ok || currentHP == 0 {
		return 0, 0, false
	}
	if currentHP > defaults.MaxHP {
		currentHP = defaults.MaxHP
	}
	nextHP := currentHP
	if nextHP <= defaults.DamagePerNormalAttack {
		nextHP = 0
	} else {
		nextHP -= defaults.DamagePerNormalAttack
	}
	hpPercent := bootstrapStaticActorHPPercent(nextHP, defaults.MaxHP)
	return nextHP, hpPercent, true
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
