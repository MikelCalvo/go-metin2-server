package staticstore

import (
	"errors"
	"sort"
)

var (
	ErrStorePathRequired = errors.New("static actor store path is required")
	ErrSnapshotNotFound  = errors.New("static actor snapshot not found")
	ErrInvalidSnapshot   = errors.New("invalid static actor snapshot")
)

type StaticActor struct {
	EntityID uint64 `json:"entity_id"`
	Name     string `json:"name"`
	MapIndex uint32 `json:"map_index"`
	X        int32  `json:"x"`
	Y        int32  `json:"y"`
	RaceNum  uint32 `json:"race_num"`
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
	for _, actor := range snapshot.StaticActors {
		if actor.EntityID == 0 || actor.Name == "" || actor.MapIndex == 0 || actor.RaceNum == 0 {
			return ErrInvalidSnapshot
		}
		if _, ok := seen[actor.EntityID]; ok {
			return ErrInvalidSnapshot
		}
		seen[actor.EntityID] = struct{}{}
	}
	return nil
}

func cloneStaticActors(actors []StaticActor) []StaticActor {
	if len(actors) == 0 {
		return nil
	}
	cloned := make([]StaticActor, len(actors))
	copy(cloned, actors)
	return cloned
}
