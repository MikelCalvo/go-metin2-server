package staticstore

import (
	"errors"
	"fmt"
	"sort"

	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
)

const (
	StaticActorContentStateMigrationVersion = 12
	StaticActorContentStateMigrationName    = "static_actor_pve_interaction_state"
)

// StaticActorContentStateExport is a deterministic, schema-shaped projection of
// committed bootstrap static-actor and interaction-definition snapshots onto the
// 0012_static_actor_pve_interaction_state migration boundary (after the
// historical 0008_static_actor_content_state tables). It is intentionally a
// data-model/export contract only: it does not open a database, emit SQL, apply
// migrations, or mutate the file stores.
type StaticActorContentStateExport struct {
	MigrationVersion       int                                  `json:"migration_version"`
	MigrationName          string                               `json:"migration_name"`
	InteractionDefinitions []InteractionDefinitionRow           `json:"interaction_definitions"`
	MerchantCatalogEntries []InteractionMerchantCatalogEntryRow `json:"merchant_catalog_entries"`
	QuestFlagRewardItems   []InteractionQuestFlagItemRow        `json:"quest_flag_reward_items"`
	QuestFlagConsumeItems  []InteractionQuestFlagItemRow        `json:"quest_flag_consume_items"`
	StaticActors           []StaticActorContentStateRow         `json:"static_actors"`
	RewardDrops            []StaticActorRewardDropRow           `json:"reward_drops"`
}

// InteractionDefinitionRow mirrors the interaction_definitions table columns
// frozen by the 0012_static_actor_pve_interaction_state migration, excluding
// timestamps that are database-owned at insert/update time.
type InteractionDefinitionRow struct {
	Kind              string  `json:"kind"`
	Ref               string  `json:"ref"`
	Text              string  `json:"text,omitempty"`
	Title             string  `json:"title,omitempty"`
	MapIndex          *uint32 `json:"map_index,omitempty"`
	X                 *int32  `json:"x,omitempty"`
	Y                 *int32  `json:"y,omitempty"`
	Size              uint8   `json:"size,omitempty"`
	QuestRef          string  `json:"quest_ref,omitempty"`
	QuestFlag         string  `json:"quest_flag,omitempty"`
	QuestFrom         uint32  `json:"quest_from,omitempty"`
	QuestTo           uint32  `json:"quest_to,omitempty"`
	RewardExperience  uint64  `json:"reward_experience,omitempty"`
	RewardGold        uint64  `json:"reward_gold,omitempty"`
	ConsumeGold       uint64  `json:"consume_gold,omitempty"`
	ConsumeExperience uint64  `json:"consume_experience,omitempty"`
}

// InteractionMerchantCatalogEntryRow mirrors shop-preview child rows for the
// interaction_merchant_catalog_entries table frozen by migration 0008 / 0012.
type InteractionMerchantCatalogEntryRow struct {
	DefinitionKind string `json:"definition_kind"`
	DefinitionRef  string `json:"definition_ref"`
	Slot           uint16 `json:"slot"`
	ItemVnum       uint32 `json:"item_vnum"`
	Price          uint64 `json:"price"`
	Count          uint16 `json:"count"`
}

// InteractionQuestFlagItemRow mirrors ordered quest_flag reward/consume item
// child rows frozen by migration 0012.
type InteractionQuestFlagItemRow struct {
	DefinitionKind string `json:"definition_kind"`
	DefinitionRef  string `json:"definition_ref"`
	Position       uint8  `json:"position"`
	ItemVnum       uint32 `json:"item_vnum"`
	Count          uint16 `json:"count"`
}

// StaticActorContentStateRow mirrors the static_actors table columns frozen by
// migration 0012, excluding timestamps that are database-owned at insert/update
// time.
type StaticActorContentStateRow struct {
	EntityID          uint64  `json:"entity_id"`
	Name              string  `json:"name"`
	MapIndex          uint32  `json:"map_index"`
	X                 int32   `json:"x"`
	Y                 int32   `json:"y"`
	RaceNum           uint32  `json:"race_num"`
	SpawnHomeMapIndex *uint32 `json:"spawn_home_map_index,omitempty"`
	SpawnHomeX        *int32  `json:"spawn_home_x,omitempty"`
	SpawnHomeY        *int32  `json:"spawn_home_y,omitempty"`
	CombatProfile     string  `json:"combat_profile,omitempty"`
	InteractionKind   string  `json:"interaction_kind,omitempty"`
	InteractionRef    string  `json:"interaction_ref,omitempty"`
	SpawnGroupRef     string  `json:"spawn_group_ref,omitempty"`
	RewardExperience  uint64  `json:"reward_experience,omitempty"`
	RewardGold        uint64  `json:"reward_gold,omitempty"`
	RewardQuestRef    string  `json:"reward_quest_ref,omitempty"`
	RewardQuestFlag   string  `json:"reward_quest_flag,omitempty"`
	RewardQuestFrom   uint32  `json:"reward_quest_from,omitempty"`
	RewardQuestTo     uint32  `json:"reward_quest_to,omitempty"`
	RewardQuestText   string  `json:"reward_quest_text,omitempty"`
	RequireQuestRef   string  `json:"require_quest_ref,omitempty"`
	RequireQuestFlag  string  `json:"require_quest_flag,omitempty"`
	RequireQuestFrom  uint32  `json:"require_quest_from,omitempty"`
}

// StaticActorRewardDropRow mirrors ordered static_actor_reward_drops rows frozen
// by migration 0008 / 0012.
type StaticActorRewardDropRow struct {
	EntityID uint64 `json:"entity_id"`
	Position uint8  `json:"position"`
	ItemVnum uint32 `json:"item_vnum"`
}

// ExportStaticActorContentState validates bootstrap static actor and interaction
// snapshots and returns rows ordered exactly as a future backfill/import tool
// should process them: interaction definitions by kind/ref, merchant catalog
// entries by definition/slot, quest-flag item tables by definition/position,
// static actors by name/entity id, and reward drops by actor/position. All
// validation fails closed against the 0012 migration constraints so malformed
// bootstrap JSON cannot be silently coerced into a future database import.
func ExportStaticActorContentState(staticSnapshot Snapshot, interactionSnapshot interactionstore.Snapshot) (StaticActorContentStateExport, error) {
	normalizedInteractions, err := normalizedInteractionDefinitionsForExport(interactionSnapshot)
	if err != nil {
		return StaticActorContentStateExport{}, err
	}

	normalizedStaticActors := normalizeSnapshot(staticSnapshot)
	if err := validateSnapshot(normalizedStaticActors); err != nil {
		return StaticActorContentStateExport{}, fmt.Errorf("%w: validate static actor content-state export", err)
	}

	definitionKeys := make(map[string]struct{}, len(normalizedInteractions.Definitions))
	for _, definition := range normalizedInteractions.Definitions {
		definitionKeys[interactionDefinitionExportKey(definition.Kind, definition.Ref)] = struct{}{}
	}

	export := StaticActorContentStateExport{
		MigrationVersion:       StaticActorContentStateMigrationVersion,
		MigrationName:          StaticActorContentStateMigrationName,
		InteractionDefinitions: []InteractionDefinitionRow{},
		MerchantCatalogEntries: []InteractionMerchantCatalogEntryRow{},
		QuestFlagRewardItems:   []InteractionQuestFlagItemRow{},
		QuestFlagConsumeItems:  []InteractionQuestFlagItemRow{},
		StaticActors:           []StaticActorContentStateRow{},
		RewardDrops:            []StaticActorRewardDropRow{},
	}

	for _, definition := range normalizedInteractions.Definitions {
		export.InteractionDefinitions = append(export.InteractionDefinitions, interactionDefinitionRowForExport(definition))
		for _, entry := range definition.Catalog {
			export.MerchantCatalogEntries = append(export.MerchantCatalogEntries, InteractionMerchantCatalogEntryRow{
				DefinitionKind: definition.Kind,
				DefinitionRef:  definition.Ref,
				Slot:           entry.Slot,
				ItemVnum:       entry.ItemVnum,
				Price:          entry.Price,
				Count:          entry.Count,
			})
		}
		for i, entry := range interactionstore.EffectiveRewardItems(definition) {
			export.QuestFlagRewardItems = append(export.QuestFlagRewardItems, InteractionQuestFlagItemRow{
				DefinitionKind: definition.Kind,
				DefinitionRef:  definition.Ref,
				Position:       uint8(i),
				ItemVnum:       entry.ItemVnum,
				Count:          entry.Count,
			})
		}
		for i, entry := range interactionstore.EffectiveConsumeItems(definition) {
			export.QuestFlagConsumeItems = append(export.QuestFlagConsumeItems, InteractionQuestFlagItemRow{
				DefinitionKind: definition.Kind,
				DefinitionRef:  definition.Ref,
				Position:       uint8(i),
				ItemVnum:       entry.ItemVnum,
				Count:          entry.Count,
			})
		}
	}

	for _, actor := range normalizedStaticActors.StaticActors {
		if actor.InteractionKind != "" || actor.InteractionRef != "" {
			if _, ok := definitionKeys[interactionDefinitionExportKey(actor.InteractionKind, actor.InteractionRef)]; !ok {
				return StaticActorContentStateExport{}, fmt.Errorf("%w: static actor %d references missing interaction definition %s:%s", ErrInvalidSnapshot, actor.EntityID, actor.InteractionKind, actor.InteractionRef)
			}
		}
		if len(actor.RewardDropVnums) > 255 {
			return StaticActorContentStateExport{}, fmt.Errorf("%w: static actor %d has %d reward drops; migration supports 255", ErrInvalidSnapshot, actor.EntityID, len(actor.RewardDropVnums))
		}
		export.StaticActors = append(export.StaticActors, staticActorContentStateRowForExport(actor))
		for i, vnum := range actor.RewardDropVnums {
			export.RewardDrops = append(export.RewardDrops, StaticActorRewardDropRow{EntityID: actor.EntityID, Position: uint8(i), ItemVnum: vnum})
		}
	}

	return export, nil
}

// ExportStaticActorContentStateFromStores validates and projects the committed
// file-store snapshots onto the 0012 static actor PvE interaction-state
// migration shape. It reads the same committed snapshot sets as Load and
// applies no mutations.
func ExportStaticActorContentStateFromStores(staticActors Store, interactions interactionstore.Store) (StaticActorContentStateExport, error) {
	staticSnapshot, err := loadStaticActorSnapshotForExport(staticActors)
	if err != nil {
		return StaticActorContentStateExport{}, err
	}
	interactionSnapshot, err := loadInteractionSnapshotForExport(interactions)
	if err != nil {
		return StaticActorContentStateExport{}, err
	}
	return ExportStaticActorContentState(staticSnapshot, interactionSnapshot)
}

// ExportStaticActorContentState projects this FileStore's committed snapshot
// onto the 0012 static-actor PvE interaction-state migration shape, reading the
// paired interaction store through the shared FromStores helper.
func (s *FileStore) ExportStaticActorContentState(interactions interactionstore.Store) (StaticActorContentStateExport, error) {
	return ExportStaticActorContentStateFromStores(s, interactions)
}

func loadStaticActorSnapshotForExport(store Store) (Snapshot, error) {
	if store == nil {
		return Snapshot{}, nil
	}
	snapshot, err := store.Load()
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			return Snapshot{}, nil
		}
		return Snapshot{}, err
	}
	return snapshot, nil
}

func loadInteractionSnapshotForExport(store interactionstore.Store) (interactionstore.Snapshot, error) {
	if store == nil {
		return interactionstore.Snapshot{}, nil
	}
	snapshot, err := store.Load()
	if err != nil {
		if errors.Is(err, interactionstore.ErrSnapshotNotFound) {
			return interactionstore.Snapshot{}, nil
		}
		return interactionstore.Snapshot{}, err
	}
	return snapshot, nil
}

func normalizedInteractionDefinitionsForExport(snapshot interactionstore.Snapshot) (interactionstore.Snapshot, error) {
	definitions := append([]interactionstore.Definition(nil), snapshot.Definitions...)
	if definitions == nil {
		definitions = []interactionstore.Definition{}
	}
	for i := range definitions {
		definitions[i] = interactionstore.NormalizeDefinition(definitions[i])
	}
	sort.Slice(definitions, func(i int, j int) bool {
		if definitions[i].Kind == definitions[j].Kind {
			return definitions[i].Ref < definitions[j].Ref
		}
		return definitions[i].Kind < definitions[j].Kind
	})

	seen := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		if !interactionstore.ValidDefinition(definition) {
			return interactionstore.Snapshot{}, fmt.Errorf("%w: validate interaction definition content-state export", interactionstore.ErrInvalidSnapshot)
		}
		if !validStaticActorContentStateInteractionDefinition(definition) {
			return interactionstore.Snapshot{}, fmt.Errorf("%w: interaction definition %s:%s cannot target static actor content-state migration", interactionstore.ErrInvalidSnapshot, definition.Kind, definition.Ref)
		}
		key := interactionDefinitionExportKey(definition.Kind, definition.Ref)
		if _, ok := seen[key]; ok {
			return interactionstore.Snapshot{}, fmt.Errorf("%w: duplicate interaction definition %s:%s", interactionstore.ErrInvalidSnapshot, definition.Kind, definition.Ref)
		}
		seen[key] = struct{}{}
	}
	return interactionstore.Snapshot{Definitions: definitions}, nil
}

func validStaticActorContentStateInteractionDefinition(definition interactionstore.Definition) bool {
	switch definition.Kind {
	case interactionstore.KindInfo, interactionstore.KindTalk, interactionstore.KindWarp, interactionstore.KindShopPreview, interactionstore.KindOpenSafebox, interactionstore.KindQuestFlag:
		return true
	default:
		return false
	}
}

func interactionDefinitionRowForExport(definition interactionstore.Definition) InteractionDefinitionRow {
	row := InteractionDefinitionRow{
		Kind:              definition.Kind,
		Ref:               definition.Ref,
		Text:              definition.Text,
		Title:             definition.Title,
		Size:              definition.Size,
		QuestRef:          definition.QuestRef,
		QuestFlag:         definition.QuestFlag,
		QuestFrom:         definition.QuestFrom,
		QuestTo:           definition.QuestTo,
		RewardExperience:  definition.RewardExperience,
		RewardGold:        definition.RewardGold,
		ConsumeGold:       definition.ConsumeGold,
		ConsumeExperience: definition.ConsumeExperience,
	}
	if definition.Kind == interactionstore.KindWarp {
		row.MapIndex = uint32ExportPtr(definition.MapIndex)
		row.X = int32ExportPtr(definition.X)
		row.Y = int32ExportPtr(definition.Y)
	}
	return row
}

func staticActorContentStateRowForExport(actor StaticActor) StaticActorContentStateRow {
	row := StaticActorContentStateRow{
		EntityID:         actor.EntityID,
		Name:             actor.Name,
		MapIndex:         actor.MapIndex,
		X:                actor.X,
		Y:                actor.Y,
		RaceNum:          actor.RaceNum,
		CombatProfile:    actor.CombatProfile,
		InteractionKind:  actor.InteractionKind,
		InteractionRef:   actor.InteractionRef,
		SpawnGroupRef:    actor.SpawnGroupRef,
		RewardExperience: actor.RewardExperience,
		RewardGold:       actor.RewardGold,
		RewardQuestRef:   actor.RewardQuestRef,
		RewardQuestFlag:  actor.RewardQuestFlag,
		RewardQuestFrom:  actor.RewardQuestFrom,
		RewardQuestTo:    actor.RewardQuestTo,
		RewardQuestText:  actor.RewardQuestText,
		RequireQuestRef:  actor.RequireQuestRef,
		RequireQuestFlag: actor.RequireQuestFlag,
		RequireQuestFrom: actor.RequireQuestFrom,
	}
	if actor.SpawnHome != nil {
		row.SpawnHomeMapIndex = uint32ExportPtr(actor.SpawnHome.MapIndex)
		row.SpawnHomeX = int32ExportPtr(actor.SpawnHome.X)
		row.SpawnHomeY = int32ExportPtr(actor.SpawnHome.Y)
	}
	return row
}

func interactionDefinitionExportKey(kind string, ref string) string {
	return kind + "\x00" + ref
}

func uint32ExportPtr(value uint32) *uint32 { return &value }
func int32ExportPtr(value int32) *int32    { return &value }
