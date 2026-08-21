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
	KindOpenSafebox = "open_safebox"
	KindQuestFlag   = "quest_flag"

	MerchantCatalogMaxEntryPrice uint64 = 1<<32 - 1
	MerchantCatalogMaxEntryCount uint16 = 1<<8 - 1
	// OpenSafeboxSizeMin / OpenSafeboxSizeMax mirror the bootstrap /open_safebox
	// page-count range. Authored size 0 means "default to 1" at runtime.
	OpenSafeboxSizeMin uint8 = 1
	OpenSafeboxSizeMax uint8 = 3
	// QuestFlagRewardGoldMax is the maximum authored gold grant that still fits
	// the bootstrap PLAYER_POINT_CHANGE gold carrier used by death rewards.
	QuestFlagRewardGoldMax uint64 = 1<<31 - 1
	// QuestFlagRewardExperienceMax is the maximum authored experience grant that
	// still fits the bootstrap PLAYER_POINT_CHANGE experience carrier used by
	// death rewards.
	QuestFlagRewardExperienceMax uint64 = 1<<31 - 1
	// QuestFlagRewardItemsMax is the maximum number of carried-item entries a
	// single quest_flag turn-in may author.
	QuestFlagRewardItemsMax = 8
	// QuestFlagConsumeItemsMax is the maximum number of carried-item consume
	// entries a single quest_flag turn-in may author.
	QuestFlagConsumeItemsMax = 8
	// QuestFlagConsumeGoldMax is the maximum authored gold debit that still fits
	// the bootstrap PLAYER_POINT_CHANGE gold carrier used by death rewards and
	// reward_gold grants.
	QuestFlagConsumeGoldMax uint64 = 1<<31 - 1
	// QuestFlagConsumeExperienceMax is the maximum authored experience debit that
	// still fits the bootstrap PLAYER_POINT_CHANGE experience carrier used by
	// death rewards and reward_experience grants.
	QuestFlagConsumeExperienceMax uint64 = 1<<31 - 1
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

// RewardItemEntry is one carried-inventory grant authored on a quest_flag turn-in.
type RewardItemEntry struct {
	ItemVnum uint32 `json:"item_vnum"`
	Count    uint16 `json:"count"`
}

type Definition struct {
	Kind              string                 `json:"kind"`
	Ref               string                 `json:"ref"`
	Text              string                 `json:"text,omitempty"`
	Title             string                 `json:"title,omitempty"`
	Catalog           []MerchantCatalogEntry `json:"catalog,omitempty"`
	MapIndex          uint32                 `json:"map_index,omitempty"`
	X                 int32                  `json:"x,omitempty"`
	Y                 int32                  `json:"y,omitempty"`
	Size              uint8                  `json:"size,omitempty"`
	QuestRef          string                 `json:"quest_ref,omitempty"`
	QuestFlag         string                 `json:"quest_flag,omitempty"`
	QuestFrom         uint32                 `json:"quest_from,omitempty"`
	QuestTo           uint32                 `json:"quest_to,omitempty"`
	RewardExperience  uint64                 `json:"reward_experience,omitempty"`
	RewardGold        uint64                 `json:"reward_gold,omitempty"`
	RewardItemVnum    uint32                 `json:"reward_item_vnum,omitempty"`
	RewardItemCount   uint16                 `json:"reward_item_count,omitempty"`
	RewardItems       []RewardItemEntry      `json:"reward_items,omitempty"`
	ConsumeItems      []RewardItemEntry      `json:"consume_items,omitempty"`
	ConsumeGold       uint64                 `json:"consume_gold,omitempty"`
	ConsumeExperience uint64                 `json:"consume_experience,omitempty"`
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
	definition.RewardItems = cloneRewardItems(definition.RewardItems)
	definition.ConsumeItems = cloneRewardItems(definition.ConsumeItems)
	hasScalar := definition.RewardItemVnum != 0 || definition.RewardItemCount != 0
	hasTable := len(definition.RewardItems) > 0
	if hasTable && hasScalar {
		// Leave both forms present so ValidDefinition rejects the mixed authoring.
		return definition
	}
	if !hasTable && definition.RewardItemVnum != 0 {
		definition.RewardItems = []RewardItemEntry{{
			ItemVnum: definition.RewardItemVnum,
			Count:    definition.RewardItemCount,
		}}
	}
	if len(definition.RewardItems) > 0 {
		definition.RewardItemVnum = 0
		definition.RewardItemCount = 0
	}
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
	case KindInfo, KindTalk, KindWarp, KindShopPreview, KindOpenSafebox, KindQuestFlag:
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
		return definition.Text != "" && validDefinitionText(definition.Text) && definition.Title == "" && len(definition.Catalog) == 0 && definition.MapIndex == 0 && definition.X == 0 && definition.Y == 0 && definition.Size == 0 && definition.RewardExperience == 0 && definition.RewardGold == 0 && definition.ConsumeGold == 0 && definition.ConsumeExperience == 0 && !hasRewardItems(definition) && !hasConsumeItems(definition) && validOptionalServiceQuestGate(definition)
	case KindShopPreview:
		if definition.Title == "" || !validDefinitionText(definition.Title) || definition.Text != "" || definition.MapIndex != 0 || definition.X != 0 || definition.Y != 0 || definition.Size != 0 || definition.RewardExperience != 0 || definition.RewardGold != 0 || definition.ConsumeGold != 0 || definition.ConsumeExperience != 0 || hasRewardItems(definition) || hasConsumeItems(definition) || !validOptionalServiceQuestGate(definition) {
			return false
		}
		return validMerchantCatalog(definition.Catalog)
	case KindWarp:
		return definition.Title == "" && validDefinitionText(definition.Text) && len(definition.Catalog) == 0 && definition.MapIndex != 0 && definition.X != 0 && definition.Y != 0 && definition.Size == 0 && definition.RewardExperience == 0 && definition.RewardGold == 0 && definition.ConsumeGold == 0 && definition.ConsumeExperience == 0 && !hasRewardItems(definition) && !hasConsumeItems(definition) && validOptionalServiceQuestGate(definition)
	case KindOpenSafebox:
		return definition.Title == "" && validDefinitionText(definition.Text) && len(definition.Catalog) == 0 && definition.MapIndex == 0 && definition.X == 0 && definition.Y == 0 && definition.Size <= OpenSafeboxSizeMax && definition.RewardExperience == 0 && definition.RewardGold == 0 && definition.ConsumeGold == 0 && definition.ConsumeExperience == 0 && !hasRewardItems(definition) && !hasConsumeItems(definition) && validOptionalServiceQuestGate(definition)
	case KindQuestFlag:
		return definition.Text != "" && validDefinitionText(definition.Text) && definition.Title == "" && len(definition.Catalog) == 0 && definition.MapIndex == 0 && definition.X == 0 && definition.Y == 0 && definition.Size == 0 && queststate.ValidQuestRef(definition.QuestRef) && queststate.ValidFlagName(definition.QuestFlag) && definition.QuestFrom != definition.QuestTo && definition.RewardExperience <= QuestFlagRewardExperienceMax && definition.RewardGold <= QuestFlagRewardGoldMax && definition.ConsumeGold <= QuestFlagConsumeGoldMax && definition.ConsumeExperience <= QuestFlagConsumeExperienceMax && validOptionalRewardItems(definition) && validOptionalConsumeItems(definition)
	default:
		return false
	}
}

func hasRewardItems(definition Definition) bool {
	return definition.RewardItemVnum != 0 || definition.RewardItemCount != 0 || len(definition.RewardItems) > 0
}

func hasConsumeItems(definition Definition) bool {
	return len(definition.ConsumeItems) > 0
}

// EffectiveRewardItems returns the canonical carried-item grant table for a
// quest_flag definition. Scalar reward_item_vnum/count remains a one-entry
// authoring shorthand that expands into this table during normalize.
func EffectiveRewardItems(definition Definition) []RewardItemEntry {
	definition = normalizeDefinition(definition)
	return cloneRewardItems(definition.RewardItems)
}

// EffectiveConsumeItems returns the canonical carried-item consume table for a
// quest_flag definition.
func EffectiveConsumeItems(definition Definition) []RewardItemEntry {
	definition = normalizeDefinition(definition)
	return cloneRewardItems(definition.ConsumeItems)
}

func validOptionalRewardItems(definition Definition) bool {
	if definition.RewardItemVnum != 0 || definition.RewardItemCount != 0 {
		// Normalize expands scalars into RewardItems and clears them. Seeing
		// both forms after normalize means the author mixed scalar + table.
		return false
	}
	if len(definition.RewardItems) > QuestFlagRewardItemsMax {
		return false
	}
	for _, entry := range definition.RewardItems {
		if entry.ItemVnum == 0 || entry.Count < 1 || entry.Count > 255 {
			return false
		}
	}
	return true
}

func validOptionalConsumeItems(definition Definition) bool {
	if len(definition.ConsumeItems) > QuestFlagConsumeItemsMax {
		return false
	}
	for _, entry := range definition.ConsumeItems {
		if entry.ItemVnum == 0 || entry.Count < 1 || entry.Count > 255 {
			return false
		}
	}
	return true
}

// HasServiceQuestGate reports whether an info/talk/warp/shop_preview/open_safebox
// definition carries an optional selected-character quest-flag prerequisite. The
// gate is present only when both quest_ref and quest_flag are authored; quest_from
// defaults to 0 and quest_to must remain 0 because gated non-mutating
// interactions do not change quest state.
func HasServiceQuestGate(definition Definition) bool {
	definition = normalizeDefinition(definition)
	return definition.QuestRef != "" && definition.QuestFlag != ""
}

// EffectiveOpenSafeboxSize returns the bootstrap page count used by an
// open_safebox definition. Authored size 0 defaults to OpenSafeboxSizeMin.
func EffectiveOpenSafeboxSize(definition Definition) uint8 {
	definition = normalizeDefinition(definition)
	if definition.Size == 0 {
		return OpenSafeboxSizeMin
	}
	return definition.Size
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
		cloned[i].RewardItems = cloneRewardItems(definition.RewardItems)
		cloned[i].ConsumeItems = cloneRewardItems(definition.ConsumeItems)
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

func cloneRewardItems(entries []RewardItemEntry) []RewardItemEntry {
	if len(entries) == 0 {
		return nil
	}
	cloned := make([]RewardItemEntry, len(entries))
	copy(cloned, entries)
	return cloned
}
