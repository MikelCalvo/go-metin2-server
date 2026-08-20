package interactionstore

import (
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
)

const (
	KindInfo        = "info"
	KindTalk        = "talk"
	KindWarp        = "warp"
	KindShopPreview = "shop_preview"
	KindQuestFlag   = "quest_flag"

	MerchantCatalogMaxEntryPrice uint64 = 1<<32 - 1
	MerchantCatalogMaxEntryCount uint16 = 1<<8 - 1
	// QuestFlagRewardGoldMax is the maximum authored gold grant that still fits
	// the bootstrap PLAYER_POINT_CHANGE gold carrier used by death rewards.
	QuestFlagRewardGoldMax uint64 = 1<<31 - 1
	// QuestFlagRewardExperienceMax is the maximum authored experience grant that
	// still fits the bootstrap PLAYER_POINT_CHANGE experience carrier used by
	// death rewards.
	QuestFlagRewardExperienceMax uint64 = 1<<31 - 1
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
	Kind             string                 `json:"kind"`
	Ref              string                 `json:"ref"`
	Text             string                 `json:"text,omitempty"`
	Title            string                 `json:"title,omitempty"`
	Catalog          []MerchantCatalogEntry `json:"catalog,omitempty"`
	MapIndex         uint32                 `json:"map_index,omitempty"`
	X                int32                  `json:"x,omitempty"`
	Y                int32                  `json:"y,omitempty"`
	QuestRef         string                 `json:"quest_ref,omitempty"`
	QuestFlag        string                 `json:"quest_flag,omitempty"`
	QuestFrom        uint32                 `json:"quest_from,omitempty"`
	QuestTo          uint32                 `json:"quest_to,omitempty"`
	RewardExperience uint64                 `json:"reward_experience,omitempty"`
	RewardGold       uint64                 `json:"reward_gold,omitempty"`
	RewardItemVnum   uint32                 `json:"reward_item_vnum,omitempty"`
	RewardItemCount  uint16                 `json:"reward_item_count,omitempty"`
}

type Snapshot struct {
	Definitions []Definition `json:"definitions"`
}

type SnapshotSummary struct {
	DefinitionCount int      `json:"definition_count"`
	DefinitionKeys  []string `json:"definition_keys"`
	CrashTempCount  int      `json:"crash_temp_count,omitempty"`
	CrashTempFiles  []string `json:"crash_temp_files,omitempty"`
}

type Store interface {
	Load() (Snapshot, error)
	Save(Snapshot) error
}

func normalizeSnapshot(snapshot Snapshot) Snapshot {
	normalized := Snapshot{Definitions: cloneDefinitions(snapshot.Definitions)}
	if normalized.Definitions == nil {
		normalized.Definitions = []Definition{}
	}
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
	definition.QuestRef = strings.TrimSpace(definition.QuestRef)
	definition.QuestFlag = strings.TrimSpace(definition.QuestFlag)
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
	case KindInfo, KindTalk, KindWarp, KindShopPreview, KindQuestFlag:
		return true
	default:
		return false
	}
}

func ValidKind(kind string) bool {
	return validKind(strings.TrimSpace(kind))
}

func validRef(ref string) bool {
	ref = strings.TrimSpace(ref)
	parts := strings.Split(ref, ":")
	if len(parts) != 2 {
		return false
	}
	return validRefSegment(parts[0]) && validRefSegment(parts[1])
}

func validRefSegment(segment string) bool {
	if segment == "" {
		return false
	}
	first := segment[0]
	if first < 'a' || first > 'z' {
		return false
	}
	for i := 1; i < len(segment); i++ {
		c := segment[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return false
	}
	return true
}

func ValidRef(ref string) bool {
	return validRef(ref)
}

func validDefinition(definition Definition) bool {
	if !validKind(definition.Kind) || !validRef(definition.Ref) {
		return false
	}
	switch definition.Kind {
	case KindInfo, KindTalk:
		return definition.Text != "" && validDefinitionText(definition.Text) && definition.Title == "" && len(definition.Catalog) == 0 && definition.MapIndex == 0 && definition.X == 0 && definition.Y == 0 && definition.RewardExperience == 0 && definition.RewardGold == 0 && !hasRewardItem(definition) && validOptionalServiceQuestGate(definition)
	case KindShopPreview:
		if definition.Title == "" || !validDefinitionText(definition.Title) || definition.Text != "" || definition.MapIndex != 0 || definition.X != 0 || definition.Y != 0 || definition.RewardExperience != 0 || definition.RewardGold != 0 || hasRewardItem(definition) || !validOptionalServiceQuestGate(definition) {
			return false
		}
		return validMerchantCatalog(definition.Catalog)
	case KindWarp:
		return definition.Title == "" && validDefinitionText(definition.Text) && len(definition.Catalog) == 0 && definition.MapIndex != 0 && definition.X != 0 && definition.Y != 0 && definition.RewardExperience == 0 && definition.RewardGold == 0 && !hasRewardItem(definition) && validOptionalServiceQuestGate(definition)
	case KindQuestFlag:
		return definition.Text != "" && validDefinitionText(definition.Text) && definition.Title == "" && len(definition.Catalog) == 0 && definition.MapIndex == 0 && definition.X == 0 && definition.Y == 0 && queststate.ValidQuestRef(definition.QuestRef) && queststate.ValidFlagName(definition.QuestFlag) && definition.QuestFrom != definition.QuestTo && definition.RewardExperience <= QuestFlagRewardExperienceMax && definition.RewardGold <= QuestFlagRewardGoldMax && validOptionalRewardItem(definition)
	default:
		return false
	}
}

func hasRewardItem(definition Definition) bool {
	return definition.RewardItemVnum != 0 || definition.RewardItemCount != 0
}

func validOptionalRewardItem(definition Definition) bool {
	if definition.RewardItemVnum == 0 {
		return definition.RewardItemCount == 0
	}
	return definition.RewardItemCount >= 1 && definition.RewardItemCount <= 255
}

// HasServiceQuestGate reports whether an info/talk/warp/shop_preview definition
// carries an optional selected-character quest-flag prerequisite. The gate is
// present only when both quest_ref and quest_flag are authored; quest_from
// defaults to 0 and quest_to must remain 0 because gated non-mutating
// interactions do not change quest state.
func HasServiceQuestGate(definition Definition) bool {
	definition = normalizeDefinition(definition)
	return definition.QuestRef != "" && definition.QuestFlag != ""
}

func validOptionalServiceQuestGate(definition Definition) bool {
	hasRef := definition.QuestRef != ""
	hasFlag := definition.QuestFlag != ""
	if !hasRef && !hasFlag {
		return definition.QuestFrom == 0 && definition.QuestTo == 0
	}
	if !hasRef || !hasFlag {
		return false
	}
	return queststate.ValidQuestRef(definition.QuestRef) && queststate.ValidFlagName(definition.QuestFlag) && definition.QuestTo == 0
}

func validDefinitionText(text string) bool {
	return utf8.ValidString(text) && !strings.ContainsRune(text, '\x00')
}

func validMerchantCatalog(catalog []MerchantCatalogEntry) bool {
	if len(catalog) == 0 || len(catalog) > 40 {
		return false
	}
	for i, entry := range catalog {
		if entry.Slot != uint16(i) {
			return false
		}
		if entry.ItemVnum == 0 || entry.Price == 0 || entry.Price > MerchantCatalogMaxEntryPrice || entry.Count == 0 || entry.Count > MerchantCatalogMaxEntryCount {
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
