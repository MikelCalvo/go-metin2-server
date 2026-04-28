package interactionstore

import (
	"errors"
	"sort"
	"strings"
)

const (
	KindInfo        = "info"
	KindTalk        = "talk"
	KindWarp        = "warp"
	KindShopPreview = "shop_preview"
)

var (
	ErrStorePathRequired = errors.New("interaction store path is required")
	ErrSnapshotNotFound  = errors.New("interaction snapshot not found")
	ErrInvalidSnapshot   = errors.New("invalid interaction snapshot")
)

type Definition struct {
	Kind     string `json:"kind"`
	Ref      string `json:"ref"`
	Text     string `json:"text"`
	MapIndex uint32 `json:"map_index,omitempty"`
	X        int32  `json:"x,omitempty"`
	Y        int32  `json:"y,omitempty"`
}

type Snapshot struct {
	Definitions []Definition `json:"definitions"`
}

type Store interface {
	Load() (Snapshot, error)
	Save(Snapshot) error
}

func normalizeSnapshot(snapshot Snapshot) Snapshot {
	normalized := Snapshot{Definitions: cloneDefinitions(snapshot.Definitions)}
	sort.Slice(normalized.Definitions, func(i int, j int) bool {
		if normalized.Definitions[i].Kind == normalized.Definitions[j].Kind {
			return normalized.Definitions[i].Ref < normalized.Definitions[j].Ref
		}
		return normalized.Definitions[i].Kind < normalized.Definitions[j].Kind
	})
	return normalized
}

func validateSnapshot(snapshot Snapshot) error {
	seen := make(map[string]struct{}, len(snapshot.Definitions))
	for _, definition := range snapshot.Definitions {
		if !validDefinition(definition) {
			return ErrInvalidSnapshot
		}
		key := definition.Kind + "\x00" + definition.Ref
		if _, ok := seen[key]; ok {
			return ErrInvalidSnapshot
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validKind(kind string) bool {
	switch kind {
	case KindInfo, KindTalk, KindWarp, KindShopPreview:
		return true
	default:
		return false
	}
}

func validDefinition(definition Definition) bool {
	if !validKind(definition.Kind) || strings.TrimSpace(definition.Ref) == "" {
		return false
	}
	switch definition.Kind {
	case KindInfo, KindTalk, KindShopPreview:
		return strings.TrimSpace(definition.Text) != "" && definition.MapIndex == 0 && definition.X == 0 && definition.Y == 0
	case KindWarp:
		return definition.MapIndex != 0 && definition.X != 0 && definition.Y != 0
	default:
		return false
	}
}

func ValidDefinition(definition Definition) bool {
	return validDefinition(definition)
}

func cloneDefinitions(definitions []Definition) []Definition {
	if len(definitions) == 0 {
		return nil
	}
	cloned := make([]Definition, len(definitions))
	copy(cloned, definitions)
	return cloned
}
