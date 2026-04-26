package contentbundle

import (
	"errors"
	"sort"
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
)

var ErrInvalidBundle = errors.New("invalid content bundle")

type StaticActor struct {
	Name            string `json:"name"`
	MapIndex        uint32 `json:"map_index"`
	X               int32  `json:"x"`
	Y               int32  `json:"y"`
	RaceNum         uint32 `json:"race_num"`
	InteractionKind string `json:"interaction_kind,omitempty"`
	InteractionRef  string `json:"interaction_ref,omitempty"`
}

type Bundle struct {
	StaticActors           []StaticActor                 `json:"static_actors"`
	InteractionDefinitions []interactionstore.Definition `json:"interaction_definitions"`
}

func FromSnapshots(staticActors staticstore.Snapshot, interactions interactionstore.Snapshot) (Bundle, error) {
	bundle := Bundle{InteractionDefinitions: cloneDefinitions(interactions.Definitions)}
	bundle.StaticActors = make([]StaticActor, 0, len(staticActors.StaticActors))
	for _, actor := range staticActors.StaticActors {
		bundle.StaticActors = append(bundle.StaticActors, StaticActor{
			Name:            actor.Name,
			MapIndex:        actor.MapIndex,
			X:               actor.X,
			Y:               actor.Y,
			RaceNum:         actor.RaceNum,
			InteractionKind: actor.InteractionKind,
			InteractionRef:  actor.InteractionRef,
		})
	}
	return Canonicalize(bundle)
}

func Canonicalize(bundle Bundle) (Bundle, error) {
	normalized := Bundle{StaticActors: cloneStaticActors(bundle.StaticActors), InteractionDefinitions: cloneDefinitions(bundle.InteractionDefinitions)}
	sort.Slice(normalized.StaticActors, func(i int, j int) bool {
		if normalized.StaticActors[i].Name == normalized.StaticActors[j].Name {
			if normalized.StaticActors[i].MapIndex == normalized.StaticActors[j].MapIndex {
				if normalized.StaticActors[i].X == normalized.StaticActors[j].X {
					if normalized.StaticActors[i].Y == normalized.StaticActors[j].Y {
						if normalized.StaticActors[i].RaceNum == normalized.StaticActors[j].RaceNum {
							if normalized.StaticActors[i].InteractionKind == normalized.StaticActors[j].InteractionKind {
								return normalized.StaticActors[i].InteractionRef < normalized.StaticActors[j].InteractionRef
							}
							return normalized.StaticActors[i].InteractionKind < normalized.StaticActors[j].InteractionKind
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
	for _, actor := range bundle.StaticActors {
		if strings.TrimSpace(actor.Name) == "" || actor.MapIndex == 0 || actor.RaceNum == 0 {
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

func interactionDefinitionKey(kind string, ref string) string {
	return strings.TrimSpace(kind) + "\x00" + strings.TrimSpace(ref)
}

func cloneStaticActors(actors []StaticActor) []StaticActor {
	if len(actors) == 0 {
		return nil
	}
	cloned := make([]StaticActor, len(actors))
	copy(cloned, actors)
	return cloned
}

func cloneDefinitions(definitions []interactionstore.Definition) []interactionstore.Definition {
	if len(definitions) == 0 {
		return nil
	}
	cloned := make([]interactionstore.Definition, len(definitions))
	copy(cloned, definitions)
	return cloned
}
