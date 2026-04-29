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

type MerchantCatalogEntry struct {
	Slot     uint16 `json:"slot"`
	ItemVnum uint32 `json:"item_vnum"`
	Price    uint64 `json:"price"`
	Count    uint16 `json:"count"`
}

type Definition struct {
	Kind     string                 `json:"kind"`
	Ref      string                 `json:"ref"`
	Text     string                 `json:"text,omitempty"`
	Title    string                 `json:"title,omitempty"`
	Catalog  []MerchantCatalogEntry `json:"catalog,omitempty"`
	MapIndex uint32                 `json:"map_index,omitempty"`
	X        int32                  `json:"x,omitempty"`
	Y        int32                  `json:"y,omitempty"`
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
	for i := range normalized.Definitions {
		normalized.Definitions[i] = normalizeDefinition(normalized.Definitions[i])
	}
	sort.Slice(normalized.Definitions, func(i int, j int) bool {
		if normalized.Definitions[i].Kind == normalized.Definitions[j].Kind {
			return normalized.Definitions[i].Ref < normalized.Definitions[j].Ref
		}
		return normalized.Definitions[i].Kind < normalized.Definitions[j].Kind
	})
	return normalized
}

func normalizeDefinition(definition Definition) Definition {
	definition.Kind = strings.TrimSpace(definition.Kind)
	definition.Ref = strings.TrimSpace(definition.Ref)
	definition.Text = strings.TrimSpace(definition.Text)
	definition.Title = strings.TrimSpace(definition.Title)
	definition.Catalog = cloneCatalog(definition.Catalog)
	sort.Slice(definition.Catalog, func(i int, j int) bool {
		return definition.Catalog[i].Slot < definition.Catalog[j].Slot
	})
	return definition
}

func NormalizeDefinition(definition Definition) Definition {
	return normalizeDefinition(definition)
}

func validateSnapshot(snapshot Snapshot) error {
	seen := make(map[string]struct{}, len(snapshot.Definitions))
	for _, definition := range snapshot.Definitions {
		normalized := normalizeDefinition(definition)
		if !validDefinition(normalized) {
			return ErrInvalidSnapshot
		}
		key := normalized.Kind + "\x00" + normalized.Ref
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
	if !validKind(definition.Kind) || definition.Ref == "" {
		return false
	}
	switch definition.Kind {
	case KindInfo, KindTalk:
		return definition.Text != "" && definition.Title == "" && len(definition.Catalog) == 0 && definition.MapIndex == 0 && definition.X == 0 && definition.Y == 0
	case KindShopPreview:
		if definition.Title == "" || definition.Text != "" || definition.MapIndex != 0 || definition.X != 0 || definition.Y != 0 {
			return false
		}
		return validMerchantCatalog(definition.Catalog)
	case KindWarp:
		return definition.Title == "" && len(definition.Catalog) == 0 && definition.MapIndex != 0 && definition.X != 0 && definition.Y != 0
	default:
		return false
	}
}

func validMerchantCatalog(catalog []MerchantCatalogEntry) bool {
	if len(catalog) == 0 {
		return false
	}
	for i, entry := range catalog {
		if entry.Slot != uint16(i) {
			return false
		}
		if entry.ItemVnum == 0 || entry.Price == 0 || entry.Count == 0 {
			return false
		}
	}
	return true
}

func ValidDefinition(definition Definition) bool {
	return validDefinition(normalizeDefinition(definition))
}

func cloneDefinitions(definitions []Definition) []Definition {
	if len(definitions) == 0 {
		return nil
	}
	cloned := make([]Definition, len(definitions))
	for i, definition := range definitions {
		cloned[i] = definition
		cloned[i].Catalog = cloneCatalog(definition.Catalog)
	}
	return cloned
}

func cloneCatalog(catalog []MerchantCatalogEntry) []MerchantCatalogEntry {
	if len(catalog) == 0 {
		return nil
	}
	cloned := make([]MerchantCatalogEntry, len(catalog))
	copy(cloned, catalog)
	return cloned
}
