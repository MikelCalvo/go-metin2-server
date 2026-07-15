package staticstore

import (
	"errors"
	"sort"

	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

var (
	ErrStorePathRequired = errors.New("static actor store path is required")
	ErrSnapshotNotFound  = errors.New("static actor snapshot not found")
	ErrInvalidSnapshot   = errors.New("invalid static actor snapshot")
)

type StaticActor struct {
	EntityID         uint64   `json:"entity_id"`
	Name             string   `json:"name"`
	MapIndex         uint32   `json:"map_index"`
	X                int32    `json:"x"`
	Y                int32    `json:"y"`
	RaceNum          uint32   `json:"race_num"`
	CombatProfile    string   `json:"combat_profile,omitempty"`
	InteractionKind  string   `json:"interaction_kind,omitempty"`
	InteractionRef   string   `json:"interaction_ref,omitempty"`
	SpawnGroupRef    string   `json:"spawn_group_ref,omitempty"`
	RewardExperience uint64   `json:"reward_experience,omitempty"`
	RewardGold       uint64   `json:"reward_gold,omitempty"`
	RewardDropVnums  []uint32 `json:"reward_drop_vnums,omitempty"`
}

type Snapshot struct {
	StaticActors []StaticActor `json:"static_actors"`
}

type Store interface {
	Load() (Snapshot, error)
	Save(Snapshot) error
}

func normalizeSnapshot(snapshot Snapshot) Snapshot {
	normalized := Snapshot{StaticActors: cloneStaticActors(snapshot.StaticActors)}
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
	for _, actor := range snapshot.StaticActors {
		if actor.EntityID == 0 || actor.Name == "" || actor.MapIndex == 0 || !validBootstrapRaceNum(actor.RaceNum) {
			return ErrInvalidSnapshot
		}
		if !validInteractionMetadata(actor.InteractionKind, actor.InteractionRef) {
			return ErrInvalidSnapshot
		}
		if !worldruntime.ValidStaticActorCombatProfile(actor.CombatProfile) || !worldruntime.ValidStaticActorSpawnGroupRef(actor.SpawnGroupRef) {
			return ErrInvalidSnapshot
		}
		if actor.SpawnGroupRef == "" {
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

func validInteractionMetadata(kind string, ref string) bool {
	if kind == "" && ref == "" {
		return true
	}
	return kind != "" && ref != ""
}

func validBootstrapRaceNum(raceNum uint32) bool {
	return raceNum != 0 && raceNum <= uint32(^uint16(0))
}

func hasRewardDescriptor(actor StaticActor) bool {
	return actor.RewardExperience != 0 || actor.RewardGold != 0 || len(actor.RewardDropVnums) != 0
}

func validRewardDescriptor(actor StaticActor) bool {
	return worldruntime.ValidStaticActorDeathReward(worldruntime.StaticActorDeathReward{Experience: actor.RewardExperience, Gold: actor.RewardGold, DropVnums: actor.RewardDropVnums})
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
		cloned[i] = actor
	}
	return cloned
}
