package staticstore

import (
	"errors"
	"sort"
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

var (
	ErrStorePathRequired = errors.New("static actor store path is required")
	ErrSnapshotNotFound  = errors.New("static actor snapshot not found")
	ErrInvalidSnapshot   = errors.New("invalid static actor snapshot")
)

type StaticActor struct {
	EntityID         uint64                         `json:"entity_id"`
	Name             string                         `json:"name"`
	MapIndex         uint32                         `json:"map_index"`
	X                int32                          `json:"x"`
	Y                int32                          `json:"y"`
	RaceNum          uint32                         `json:"race_num"`
	SpawnHome        *worldruntime.PositionSnapshot `json:"spawn_home,omitempty"`
	CombatProfile    string                         `json:"combat_profile,omitempty"`
	InteractionKind  string                         `json:"interaction_kind,omitempty"`
	InteractionRef   string                         `json:"interaction_ref,omitempty"`
	SpawnGroupRef    string                         `json:"spawn_group_ref,omitempty"`
	RewardExperience uint64                         `json:"reward_experience,omitempty"`
	RewardGold       uint64                         `json:"reward_gold,omitempty"`
	RewardDropVnums  []uint32                       `json:"reward_drop_vnums,omitempty"`
}

type Snapshot struct {
	StaticActors   []StaticActor                                   `json:"static_actors"`
	CombatProfiles []worldruntime.StaticActorCombatProfileSnapshot `json:"combat_profiles,omitempty"`
}

type SnapshotSummary struct {
	ActorCount             int      `json:"actor_count"`
	InteractableActorCount int      `json:"interactable_actor_count,omitempty"`
	SpawnGroupCount        int      `json:"spawn_group_count,omitempty"`
	ActorIDs               []uint64 `json:"actor_ids"`
	ActorNames             []string `json:"actor_names"`
	CrashTempCount         int      `json:"crash_temp_count,omitempty"`
	CrashTempFiles         []string `json:"crash_temp_files,omitempty"`
}

type Store interface {
	Load() (Snapshot, error)
	Save(Snapshot) error
}

func normalizeSnapshot(snapshot Snapshot) Snapshot {
	normalized := Snapshot{StaticActors: cloneStaticActors(snapshot.StaticActors), CombatProfiles: normalizeCombatProfiles(snapshot.CombatProfiles)}
	if normalized.StaticActors == nil {
		normalized.StaticActors = []StaticActor{}
	}
	for i := range normalized.StaticActors {
		normalized.StaticActors[i] = normalizeStaticActor(normalized.StaticActors[i])
	}
	sort.Slice(normalized.StaticActors, func(i int, j int) bool {
		if normalized.StaticActors[i].Name == normalized.StaticActors[j].Name {
			return normalized.StaticActors[i].EntityID < normalized.StaticActors[j].EntityID
		}
		return normalized.StaticActors[i].Name < normalized.StaticActors[j].Name
	})
	return normalized
}

func validateSnapshot(snapshot Snapshot) error {
	seen := make(map[uint64]struct{}, len(snapshot.StaticActors))
	spawnGroupRefs := make(map[string]struct{}, len(snapshot.StaticActors))
	profileSnapshots, err := validateCombatProfiles(snapshot.CombatProfiles, snapshot.StaticActors)
	if err != nil {
		return err
	}
	for _, actor := range snapshot.StaticActors {
		if !validBootstrapEntityID(actor.EntityID) || !validStaticActorName(actor.Name) || actor.MapIndex == 0 || !validBootstrapRaceNum(actor.RaceNum) {
			return ErrInvalidSnapshot
		}
		if !validInteractionMetadata(actor.InteractionKind, actor.InteractionRef) {
			return ErrInvalidSnapshot
		}
		if !validSnapshotCombatProfile(actor.CombatProfile, profileSnapshots) || !worldruntime.ValidStaticActorSpawnGroupRef(actor.SpawnGroupRef) {
			return ErrInvalidSnapshot
		}
		if actor.SpawnGroupRef == "" {
			if actor.SpawnHome != nil {
				return ErrInvalidSnapshot
			}
			if hasRewardDescriptor(actor) {
				return ErrInvalidSnapshot
			}
		} else {
			if actor.CombatProfile == "" || actor.InteractionKind != "" || actor.InteractionRef != "" {
				return ErrInvalidSnapshot
			}
			if !validRewardDescriptor(actor) {
				return ErrInvalidSnapshot
			}
			if actor.SpawnHome != nil && actor.SpawnHome.MapIndex == 0 {
				return ErrInvalidSnapshot
			}
			if _, ok := spawnGroupRefs[actor.SpawnGroupRef]; ok {
				return ErrInvalidSnapshot
			}
			spawnGroupRefs[actor.SpawnGroupRef] = struct{}{}
		}
		if _, ok := seen[actor.EntityID]; ok {
			return ErrInvalidSnapshot
		}
		seen[actor.EntityID] = struct{}{}
	}
	return nil
}

func validateCombatProfiles(profiles []worldruntime.StaticActorCombatProfileSnapshot, actors []StaticActor) (map[string]worldruntime.StaticActorCombatProfileSnapshot, error) {
	profileSnapshots := make(map[string]worldruntime.StaticActorCombatProfileSnapshot, len(profiles))
	referencedProfiles := referencedCombatProfileNames(actors)
	for _, profile := range profiles {
		if !validCombatProfileSnapshot(profile) {
			return nil, ErrInvalidSnapshot
		}
		name := strings.TrimSpace(profile.Profile)
		if _, referenced := referencedProfiles[name]; !referenced {
			return nil, ErrInvalidSnapshot
		}
		if existing, ok := worldruntime.BootstrapStaticActorCombatProfileDefaults(name); ok && !combatProfileSnapshotMatchesDefaults(profile, existing) {
			return nil, ErrInvalidSnapshot
		}
		if _, ok := profileSnapshots[name]; ok {
			return nil, ErrInvalidSnapshot
		}
		profileSnapshots[name] = profile
	}
	return profileSnapshots, nil
}

func combatProfileSnapshotIdentitiesAreCanonical(profiles []worldruntime.StaticActorCombatProfileSnapshot) bool {
	for _, profile := range profiles {
		if !worldruntime.ValidStaticActorCombatProfileName(profile.Profile) {
			return false
		}
	}
	return true
}

func referencedCombatProfileNames(actors []StaticActor) map[string]struct{} {
	referenced := make(map[string]struct{}, len(actors))
	for _, actor := range actors {
		profile := strings.TrimSpace(actor.CombatProfile)
		if profile != "" {
			referenced[profile] = struct{}{}
		}
	}
	return referenced
}

func validSnapshotCombatProfile(profile string, profileSnapshots map[string]worldruntime.StaticActorCombatProfileSnapshot) bool {
	profile = strings.TrimSpace(profile)
	if profile == "" || worldruntime.ValidStaticActorCombatProfile(profile) {
		return true
	}
	_, ok := profileSnapshots[profile]
	return ok
}

func validCombatProfileSnapshot(profile worldruntime.StaticActorCombatProfileSnapshot) bool {
	name := strings.TrimSpace(profile.Profile)
	if !worldruntime.ValidStaticActorCombatProfileName(name) || name == worldruntime.StaticActorCombatProfilePracticeMob || name == worldruntime.StaticActorCombatProfileTrainingDummy {
		return false
	}
	if profile.RetaliationPointDelta > 0 {
		return false
	}
	if profile.MaxHP == 0 || profile.AttackValue == 0 || !worldruntime.ValidStaticActorCombatProfileRespawnDelayMs(profile.RespawnDelayMs) {
		return false
	}
	if profile.AttackValue > profile.DefenseValue && profile.AttackValue-profile.DefenseValue > uint16(profile.MaxHP) {
		return false
	}
	if profile.DamagePerNormalAttack != 0 {
		expectedDamage := combatProfileSnapshotFormulaDamage(profile)
		if profile.DamagePerNormalAttack != expectedDamage || profile.DamagePerNormalAttack > profile.MaxHP {
			return false
		}
	}
	return worldruntime.ValidStaticActorDeathReward(profile.DeathReward)
}

func combatProfileSnapshotMatchesDefaults(snapshot worldruntime.StaticActorCombatProfileSnapshot, defaults worldruntime.StaticActorCombatProfileDefaults) bool {
	candidateDefaults, ok := combatProfileSnapshotDefaults(snapshot)
	return ok &&
		candidateDefaults.MaxHP == defaults.MaxHP &&
		candidateDefaults.DamagePerNormalAttack == defaults.DamagePerNormalAttack &&
		candidateDefaults.AttackValue == defaults.AttackValue &&
		candidateDefaults.DefenseValue == defaults.DefenseValue &&
		candidateDefaults.Level == defaults.Level &&
		candidateDefaults.Rank == defaults.Rank &&
		candidateDefaults.RespawnDelay == defaults.RespawnDelay &&
		candidateDefaults.RetaliationPointDelta == defaults.RetaliationPointDelta &&
		candidateDefaults.DeathReward.Experience == defaults.DeathReward.Experience &&
		candidateDefaults.DeathReward.Gold == defaults.DeathReward.Gold &&
		uint32SlicesEqual(candidateDefaults.DeathReward.Clone().DropVnums, defaults.DeathReward.Clone().DropVnums)
}

func combatProfileSnapshotDefaults(snapshot worldruntime.StaticActorCombatProfileSnapshot) (worldruntime.StaticActorCombatProfileDefaults, bool) {
	respawnDelay, ok := worldruntime.StaticActorCombatProfileRespawnDelay(snapshot.RespawnDelayMs)
	if strings.TrimSpace(snapshot.Profile) == "" || snapshot.MaxHP == 0 || snapshot.AttackValue == 0 || !ok {
		return worldruntime.StaticActorCombatProfileDefaults{}, false
	}
	defaults := worldruntime.StaticActorCombatProfileDefaults{
		MaxHP:                 snapshot.MaxHP,
		DamagePerNormalAttack: snapshot.DamagePerNormalAttack,
		AttackValue:           snapshot.AttackValue,
		DefenseValue:          snapshot.DefenseValue,
		Level:                 snapshot.Level,
		Rank:                  snapshot.Rank,
		RespawnDelay:          respawnDelay,
		RetaliationPointDelta: snapshot.RetaliationPointDelta,
		DeathReward:           cloneDeathRewardPreservingDropMultiplicity(snapshot.DeathReward),
	}
	if defaults.DamagePerNormalAttack == 0 {
		defaults.DamagePerNormalAttack = combatProfileSnapshotFormulaDamage(snapshot)
	}
	if defaults.Level == 0 {
		defaults.Level = worldruntime.TrainingDummyBootstrapLevel
	}
	if defaults.RetaliationPointDelta == 0 {
		defaults.RetaliationPointDelta = worldruntime.PracticeMobBootstrapRetaliationPointDelta
	}
	return defaults, true
}

func combatProfileSnapshotFormulaDamage(profile worldruntime.StaticActorCombatProfileSnapshot) uint8 {
	if profile.AttackValue <= profile.DefenseValue {
		return 1
	}
	damage := profile.AttackValue - profile.DefenseValue
	if damage == 0 {
		return 1
	}
	if damage > uint16(profile.MaxHP) {
		return profile.MaxHP
	}
	return uint8(damage)
}

func validInteractionMetadata(kind string, ref string) bool {
	if kind == "" && ref == "" {
		return true
	}
	return kind != "" && ref != "" && interactionstore.ValidKind(kind) && interactionstore.ValidRef(ref)
}

func validStaticActorName(name string) bool {
	return worldruntime.ValidStaticActorName(name)
}

func validBootstrapEntityID(entityID uint64) bool {
	return worldruntime.ValidStaticActorVisibilityEntityID(entityID)
}

func validBootstrapRaceNum(raceNum uint32) bool {
	return worldruntime.ValidStaticActorVisibilityRaceNum(raceNum)
}

func hasRewardDescriptor(actor StaticActor) bool {
	return actor.RewardExperience != 0 || actor.RewardGold != 0 || len(actor.RewardDropVnums) != 0
}

func validRewardDescriptor(actor StaticActor) bool {
	return worldruntime.ValidStaticActorDeathReward(worldruntime.StaticActorDeathReward{Experience: actor.RewardExperience, Gold: actor.RewardGold, DropVnums: actor.RewardDropVnums})
}

func normalizeStaticActor(actor StaticActor) StaticActor {
	actor.Name = strings.TrimSpace(actor.Name)
	actor.CombatProfile = strings.TrimSpace(actor.CombatProfile)
	actor.InteractionKind = strings.TrimSpace(actor.InteractionKind)
	actor.InteractionRef = strings.TrimSpace(actor.InteractionRef)
	actor.SpawnGroupRef = strings.TrimSpace(actor.SpawnGroupRef)
	return actor
}

func normalizeCombatProfiles(profiles []worldruntime.StaticActorCombatProfileSnapshot) []worldruntime.StaticActorCombatProfileSnapshot {
	if len(profiles) == 0 {
		return nil
	}
	normalized := cloneCombatProfileSnapshotsPreservingRewardDropMultiplicity(profiles)
	for i := range normalized {
		if defaults, ok := combatProfileSnapshotDefaults(normalized[i]); ok {
			normalized[i].DamagePerNormalAttack = defaults.DamagePerNormalAttack
			normalized[i].Level = defaults.Level
			if defaults.RetaliationPointDelta == worldruntime.PracticeMobBootstrapRetaliationPointDelta {
				normalized[i].RetaliationPointDelta = 0
			}
		}
	}
	sort.Slice(normalized, func(i int, j int) bool {
		return normalized[i].Profile < normalized[j].Profile
	})
	return normalized
}

func cloneCombatProfileSnapshotsPreservingRewardDropMultiplicity(profiles []worldruntime.StaticActorCombatProfileSnapshot) []worldruntime.StaticActorCombatProfileSnapshot {
	if len(profiles) == 0 {
		return nil
	}
	cloned := make([]worldruntime.StaticActorCombatProfileSnapshot, len(profiles))
	copy(cloned, profiles)
	for i := range cloned {
		cloned[i].Profile = strings.TrimSpace(cloned[i].Profile)
		cloned[i].DeathReward = cloneDeathRewardPreservingDropMultiplicity(cloned[i].DeathReward)
	}
	return cloned
}

func cloneDeathRewardPreservingDropMultiplicity(reward worldruntime.StaticActorDeathReward) worldruntime.StaticActorDeathReward {
	cloned := worldruntime.StaticActorDeathReward{Experience: reward.Experience, Gold: reward.Gold}
	if len(reward.DropVnums) > 0 {
		cloned.DropVnums = append([]uint32(nil), reward.DropVnums...)
		sort.Slice(cloned.DropVnums, func(i int, j int) bool {
			return cloned.DropVnums[i] < cloned.DropVnums[j]
		})
	}
	return cloned
}

func uint32SlicesEqual(left []uint32, right []uint32) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func cloneStaticActors(actors []StaticActor) []StaticActor {
	if len(actors) == 0 {
		return nil
	}
	cloned := make([]StaticActor, len(actors))
	for i, actor := range actors {
		if len(actor.RewardDropVnums) > 0 {
			actor.RewardDropVnums = append([]uint32(nil), actor.RewardDropVnums...)
			sort.Slice(actor.RewardDropVnums, func(i int, j int) bool {
				return actor.RewardDropVnums[i] < actor.RewardDropVnums[j]
			})
		}
		if actor.SpawnHome != nil {
			spawnHome := *actor.SpawnHome
			actor.SpawnHome = &spawnHome
		}
		cloned[i] = actor
	}
	return cloned
}
