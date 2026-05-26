package contentbundle

import (
	"errors"
	"sort"
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

var ErrInvalidBundle = errors.New("invalid content bundle")

type StaticActor struct {
	Name            string `json:"name"`
	MapIndex        uint32 `json:"map_index"`
	X               int32  `json:"x"`
	Y               int32  `json:"y"`
	RaceNum         uint32 `json:"race_num"`
	CombatProfile   string `json:"combat_profile,omitempty"`
	InteractionKind string `json:"interaction_kind,omitempty"`
	InteractionRef  string `json:"interaction_ref,omitempty"`
}

type SpawnGroup struct {
	Ref           string `json:"ref"`
	Name          string `json:"name,omitempty"`
	MapIndex      uint32 `json:"map_index"`
	X             int32  `json:"x"`
	Y             int32  `json:"y"`
	RaceNum       uint32 `json:"race_num"`
	CombatProfile string `json:"combat_profile"`
}

type Bundle struct {
	StaticActors           []StaticActor                 `json:"static_actors"`
	SpawnGroups            []SpawnGroup                  `json:"spawn_groups,omitempty"`
	InteractionDefinitions []interactionstore.Definition `json:"interaction_definitions"`
}

func FromSnapshots(staticActors staticstore.Snapshot, interactions interactionstore.Snapshot) (Bundle, error) {
	bundle := Bundle{InteractionDefinitions: cloneDefinitions(interactions.Definitions)}
	bundle.StaticActors = make([]StaticActor, 0, len(staticActors.StaticActors))
	bundle.SpawnGroups = make([]SpawnGroup, 0, len(staticActors.StaticActors))
	for _, actor := range staticActors.StaticActors {
		if actor.SpawnGroupRef != "" {
			bundle.SpawnGroups = append(bundle.SpawnGroups, SpawnGroup{
				Ref:           actor.SpawnGroupRef,
				Name:          actor.Name,
				MapIndex:      actor.MapIndex,
				X:             actor.X,
				Y:             actor.Y,
				RaceNum:       actor.RaceNum,
				CombatProfile: actor.CombatProfile,
			})
			continue
		}
		bundle.StaticActors = append(bundle.StaticActors, StaticActor{
			Name:            actor.Name,
			MapIndex:        actor.MapIndex,
			X:               actor.X,
			Y:               actor.Y,
			RaceNum:         actor.RaceNum,
			CombatProfile:   actor.CombatProfile,
			InteractionKind: actor.InteractionKind,
			InteractionRef:  actor.InteractionRef,
		})
	}
	return Canonicalize(bundle)
}

func Canonicalize(bundle Bundle) (Bundle, error) {
	normalized := Bundle{
		StaticActors:           normalizeStaticActors(bundle.StaticActors),
		SpawnGroups:            normalizeSpawnGroups(bundle.SpawnGroups),
		InteractionDefinitions: cloneDefinitions(bundle.InteractionDefinitions),
	}
	sort.Slice(normalized.StaticActors, func(i int, j int) bool {
		if normalized.StaticActors[i].Name == normalized.StaticActors[j].Name {
			if normalized.StaticActors[i].MapIndex == normalized.StaticActors[j].MapIndex {
				if normalized.StaticActors[i].X == normalized.StaticActors[j].X {
					if normalized.StaticActors[i].Y == normalized.StaticActors[j].Y {
						if normalized.StaticActors[i].RaceNum == normalized.StaticActors[j].RaceNum {
							if normalized.StaticActors[i].CombatProfile == normalized.StaticActors[j].CombatProfile {
								if normalized.StaticActors[i].InteractionKind == normalized.StaticActors[j].InteractionKind {
									return normalized.StaticActors[i].InteractionRef < normalized.StaticActors[j].InteractionRef
								}
								return normalized.StaticActors[i].InteractionKind < normalized.StaticActors[j].InteractionKind
							}
							return normalized.StaticActors[i].CombatProfile < normalized.StaticActors[j].CombatProfile
						}
						return normalized.StaticActors[i].RaceNum < normalized.StaticActors[j].RaceNum
					}
					return normalized.StaticActors[i].Y < normalized.StaticActors[j].Y
				}
				return normalized.StaticActors[i].X < normalized.StaticActors[j].X
			}
			return normalized.StaticActors[i].MapIndex < normalized.StaticActors[j].MapIndex
		}
		return normalized.StaticActors[i].Name < normalized.StaticActors[j].Name
	})
	sort.Slice(normalized.SpawnGroups, func(i int, j int) bool {
		if normalized.SpawnGroups[i].Ref == normalized.SpawnGroups[j].Ref {
			if normalized.SpawnGroups[i].MapIndex == normalized.SpawnGroups[j].MapIndex {
				if normalized.SpawnGroups[i].X == normalized.SpawnGroups[j].X {
					if normalized.SpawnGroups[i].Y == normalized.SpawnGroups[j].Y {
						if normalized.SpawnGroups[i].RaceNum == normalized.SpawnGroups[j].RaceNum {
							if normalized.SpawnGroups[i].CombatProfile == normalized.SpawnGroups[j].CombatProfile {
								return normalized.SpawnGroups[i].Name < normalized.SpawnGroups[j].Name
							}
							return normalized.SpawnGroups[i].CombatProfile < normalized.SpawnGroups[j].CombatProfile
						}
						return normalized.SpawnGroups[i].RaceNum < normalized.SpawnGroups[j].RaceNum
					}
					return normalized.SpawnGroups[i].Y < normalized.SpawnGroups[j].Y
				}
				return normalized.SpawnGroups[i].X < normalized.SpawnGroups[j].X
			}
			return normalized.SpawnGroups[i].MapIndex < normalized.SpawnGroups[j].MapIndex
		}
		return normalized.SpawnGroups[i].Ref < normalized.SpawnGroups[j].Ref
	})
	sort.Slice(normalized.InteractionDefinitions, func(i int, j int) bool {
		if normalized.InteractionDefinitions[i].Kind == normalized.InteractionDefinitions[j].Kind {
			return normalized.InteractionDefinitions[i].Ref < normalized.InteractionDefinitions[j].Ref
		}
		return normalized.InteractionDefinitions[i].Kind < normalized.InteractionDefinitions[j].Kind
	})
	if err := validateBundle(normalized); err != nil {
		return Bundle{}, err
	}
	return normalized, nil
}

func validateBundle(bundle Bundle) error {
	definitionsByKey := make(map[string]struct{}, len(bundle.InteractionDefinitions))
	for _, definition := range bundle.InteractionDefinitions {
		if !validDefinition(definition) {
			return ErrInvalidBundle
		}
		key := interactionDefinitionKey(definition.Kind, definition.Ref)
		if _, ok := definitionsByKey[key]; ok {
			return ErrInvalidBundle
		}
		definitionsByKey[key] = struct{}{}
	}
	spawnGroupsByRef := make(map[string]struct{}, len(bundle.SpawnGroups))
	for _, spawnGroup := range bundle.SpawnGroups {
		if !validSpawnGroup(spawnGroup) {
			return ErrInvalidBundle
		}
		if _, ok := spawnGroupsByRef[spawnGroup.Ref]; ok {
			return ErrInvalidBundle
		}
		spawnGroupsByRef[spawnGroup.Ref] = struct{}{}
	}
	for _, actor := range bundle.StaticActors {
		if strings.TrimSpace(actor.Name) == "" || actor.MapIndex == 0 || actor.RaceNum == 0 {
			return ErrInvalidBundle
		}
		if !worldruntime.ValidStaticActorCombatProfile(actor.CombatProfile) {
			return ErrInvalidBundle
		}
		if !validInteractionMetadata(actor.InteractionKind, actor.InteractionRef) {
			return ErrInvalidBundle
		}
		if actor.InteractionKind == "" && actor.InteractionRef == "" {
			continue
		}
		if _, ok := definitionsByKey[interactionDefinitionKey(actor.InteractionKind, actor.InteractionRef)]; !ok {
			return ErrInvalidBundle
		}
	}
	return nil
}

func validDefinition(definition interactionstore.Definition) bool {
	return interactionstore.ValidDefinition(definition)
}

func validInteractionMetadata(kind string, ref string) bool {
	kind = strings.TrimSpace(kind)
	ref = strings.TrimSpace(ref)
	if kind == "" && ref == "" {
		return true
	}
	return kind != "" && ref != ""
}

func validSpawnGroup(spawnGroup SpawnGroup) bool {
	if strings.TrimSpace(spawnGroup.Ref) == "" || strings.TrimSpace(spawnGroup.Name) == "" || spawnGroup.MapIndex == 0 || spawnGroup.RaceNum == 0 {
		return false
	}
	return worldruntime.ValidStaticActorCombatProfile(spawnGroup.CombatProfile) && spawnGroup.CombatProfile != ""
}

func normalizeSpawnGroups(spawnGroups []SpawnGroup) []SpawnGroup {
	if len(spawnGroups) == 0 {
		return nil
	}
	normalized := make([]SpawnGroup, len(spawnGroups))
	for i, spawnGroup := range spawnGroups {
		spawnGroup.Ref = strings.TrimSpace(spawnGroup.Ref)
		spawnGroup.Name = strings.TrimSpace(spawnGroup.Name)
		spawnGroup.CombatProfile = strings.TrimSpace(spawnGroup.CombatProfile)
		if spawnGroup.CombatProfile == "" {
			spawnGroup.CombatProfile = worldruntime.StaticActorCombatProfilePracticeMob
		}
		normalized[i] = spawnGroup
	}
	return normalized
}

func interactionDefinitionKey(kind string, ref string) string {
	return strings.TrimSpace(kind) + "\x00" + strings.TrimSpace(ref)
}

func normalizeStaticActors(actors []StaticActor) []StaticActor {
	if len(actors) == 0 {
		return nil
	}
	normalized := make([]StaticActor, len(actors))
	copy(normalized, actors)
	for i := range normalized {
		normalized[i].Name = strings.TrimSpace(normalized[i].Name)
		normalized[i].CombatProfile = strings.TrimSpace(normalized[i].CombatProfile)
		normalized[i].InteractionKind = strings.TrimSpace(normalized[i].InteractionKind)
		normalized[i].InteractionRef = strings.TrimSpace(normalized[i].InteractionRef)
	}
	return normalized
}

func cloneDefinitions(definitions []interactionstore.Definition) []interactionstore.Definition {
	if len(definitions) == 0 {
		return nil
	}
	cloned := make([]interactionstore.Definition, len(definitions))
	for i, definition := range definitions {
		cloned[i] = interactionstore.NormalizeDefinition(definition)
	}
	return cloned
}
