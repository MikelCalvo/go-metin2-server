package contentbundle

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
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
	Ref              string   `json:"ref"`
	Name             string   `json:"name,omitempty"`
	MapIndex         uint32   `json:"map_index"`
	X                int32    `json:"x"`
	Y                int32    `json:"y"`
	RaceNum          uint32   `json:"race_num"`
	CombatProfile    string   `json:"combat_profile"`
	RewardExperience uint64   `json:"reward_experience,omitempty"`
	RewardGold       uint64   `json:"reward_gold,omitempty"`
	RewardDropVnums  []uint32 `json:"reward_drop_vnums,omitempty"`
}

type Bundle struct {
	StaticActors           []StaticActor                                   `json:"static_actors"`
	SpawnGroups            []SpawnGroup                                    `json:"spawn_groups,omitempty"`
	CombatProfiles         []worldruntime.StaticActorCombatProfileSnapshot `json:"combat_profiles,omitempty"`
	ItemTemplates          []itemcatalog.Template                          `json:"item_templates,omitempty"`
	InteractionDefinitions []interactionstore.Definition                   `json:"interaction_definitions"`
}

type Summary struct {
	StaticActorCount                       int                                             `json:"static_actor_count"`
	InteractableStaticActorCount           int                                             `json:"interactable_static_actor_count"`
	SpawnGroupCount                        int                                             `json:"spawn_group_count"`
	CombatProfileCount                     int                                             `json:"combat_profile_count"`
	ItemTemplateCount                      int                                             `json:"item_template_count"`
	StaticActors                           []StaticActor                                   `json:"static_actors,omitempty"`
	ShopCatalogEntryCount                  int                                             `json:"shop_catalog_entry_count"`
	ShopCatalogs                           []ShopCatalogSummary                            `json:"shop_catalogs,omitempty"`
	WarpDestinationCount                   int                                             `json:"warp_destination_count"`
	WarpDestinations                       []WarpDestinationSummary                        `json:"warp_destinations,omitempty"`
	RewardExperienceTotal                  uint64                                          `json:"reward_experience_total,omitempty"`
	RewardGoldTotal                        uint64                                          `json:"reward_gold_total,omitempty"`
	RewardDropItemCount                    int                                             `json:"reward_drop_item_count,omitempty"`
	RewardDrops                            []RewardDropAggregateSummary                    `json:"reward_drops,omitempty"`
	InteractionDefinitionCount             int                                             `json:"interaction_definition_count"`
	ReferencedInteractionDefinitionCount   int                                             `json:"referenced_interaction_definition_count"`
	UnreferencedInteractionDefinitionCount int                                             `json:"unreferenced_interaction_definition_count"`
	InteractionKinds                       []InteractionKindSummary                        `json:"interaction_kinds,omitempty"`
	InteractionDefinitionPreviews          []InteractionDefinitionPreviewSummary           `json:"interaction_definition_previews,omitempty"`
	ReferencedInteractionDefinitions       []InteractionDefinitionReferenceSummary         `json:"referenced_interaction_definitions,omitempty"`
	UnreferencedInteractionDefinitions     []InteractionDefinitionReferenceSummary         `json:"unreferenced_interaction_definitions,omitempty"`
	InteractableStaticActors               []InteractableStaticActorSummary                `json:"interactable_static_actors,omitempty"`
	SpawnGroups                            []SpawnGroupReferenceSummary                    `json:"spawn_groups,omitempty"`
	CombatProfiles                         []worldruntime.StaticActorCombatProfileSnapshot `json:"combat_profiles,omitempty"`
	ItemTemplates                          []ItemTemplateReferenceSummary                  `json:"item_templates,omitempty"`
	Maps                                   []MapContentSummary                             `json:"maps,omitempty"`
}

type InteractionKindSummary struct {
	Kind              string `json:"kind"`
	Count             int    `json:"count"`
	ReferencedCount   int    `json:"referenced_count"`
	UnreferencedCount int    `json:"unreferenced_count"`
}

type InteractionDefinitionReferenceSummary struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

type InteractionDefinitionPreviewSummary struct {
	Kind    string `json:"kind"`
	Ref     string `json:"ref"`
	Preview string `json:"preview"`
}

type InteractableStaticActorSummary struct {
	Name            string `json:"name"`
	MapIndex        uint32 `json:"map_index"`
	X               int32  `json:"x"`
	Y               int32  `json:"y"`
	RaceNum         uint32 `json:"race_num"`
	InteractionKind string `json:"interaction_kind"`
	InteractionRef  string `json:"interaction_ref"`
	Preview         string `json:"preview,omitempty"`
}

type SpawnGroupReferenceSummary struct {
	Ref              string                  `json:"ref"`
	Name             string                  `json:"name"`
	MapIndex         uint32                  `json:"map_index"`
	X                int32                   `json:"x"`
	Y                int32                   `json:"y"`
	RaceNum          uint32                  `json:"race_num"`
	CombatProfile    string                  `json:"combat_profile"`
	RewardExperience uint64                  `json:"reward_experience,omitempty"`
	RewardGold       uint64                  `json:"reward_gold,omitempty"`
	RewardDropVnums  []uint32                `json:"reward_drop_vnums,omitempty"`
	RewardDropItems  []RewardDropItemSummary `json:"reward_drop_items,omitempty"`
}

type RewardDropItemSummary struct {
	ItemVnum     uint32 `json:"item_vnum"`
	ItemName     string `json:"item_name"`
	Stackable    bool   `json:"stackable"`
	MaxCount     uint16 `json:"max_count"`
	ShopBuyPrice uint64 `json:"shop_buy_price,omitempty"`
}

type RewardDropAggregateSummary struct {
	ItemVnum     uint32 `json:"item_vnum"`
	ItemName     string `json:"item_name"`
	SourceCount  int    `json:"source_count"`
	Stackable    bool   `json:"stackable"`
	MaxCount     uint16 `json:"max_count"`
	ShopBuyPrice uint64 `json:"shop_buy_price,omitempty"`
}

type ItemTemplateReferenceSummary struct {
	Vnum         uint32 `json:"vnum"`
	Name         string `json:"name"`
	Stackable    bool   `json:"stackable"`
	MaxCount     uint16 `json:"max_count"`
	ShopBuyPrice uint64 `json:"shop_buy_price,omitempty"`
}

type ShopCatalogSummary struct {
	Kind       string                    `json:"kind"`
	Ref        string                    `json:"ref"`
	Title      string                    `json:"title"`
	EntryCount int                       `json:"entry_count"`
	Entries    []ShopCatalogEntrySummary `json:"entries,omitempty"`
}

type ShopCatalogEntrySummary struct {
	Slot         uint16 `json:"slot"`
	ItemVnum     uint32 `json:"item_vnum"`
	ItemName     string `json:"item_name"`
	Count        uint16 `json:"count"`
	Price        uint64 `json:"price"`
	Stackable    bool   `json:"stackable"`
	MaxCount     uint16 `json:"max_count"`
	ShopBuyPrice uint64 `json:"shop_buy_price,omitempty"`
}

type WarpDestinationSummary struct {
	Kind     string `json:"kind"`
	Ref      string `json:"ref"`
	Text     string `json:"text,omitempty"`
	MapIndex uint32 `json:"map_index"`
	X        int32  `json:"x"`
	Y        int32  `json:"y"`
}

type MapContentSummary struct {
	MapIndex                     uint32 `json:"map_index"`
	StaticActorCount             int    `json:"static_actor_count"`
	InteractableStaticActorCount int    `json:"interactable_static_actor_count"`
	SpawnGroupCount              int    `json:"spawn_group_count"`
}

func FromSnapshots(staticActors staticstore.Snapshot, interactions interactionstore.Snapshot) (Bundle, error) {
	return FromSnapshotsWithItems(staticActors, interactions, itemcatalog.Snapshot{})
}

func FromSnapshotsWithItems(staticActors staticstore.Snapshot, interactions interactionstore.Snapshot, items itemcatalog.Snapshot) (Bundle, error) {
	bundle := Bundle{
		InteractionDefinitions: cloneDefinitions(interactions.Definitions),
	}
	bundle.StaticActors = make([]StaticActor, 0, len(staticActors.StaticActors))
	bundle.SpawnGroups = make([]SpawnGroup, 0, len(staticActors.StaticActors))
	for _, actor := range staticActors.StaticActors {
		if actor.SpawnGroupRef != "" {
			bundle.SpawnGroups = append(bundle.SpawnGroups, SpawnGroup{
				Ref:              actor.SpawnGroupRef,
				Name:             actor.Name,
				MapIndex:         actor.MapIndex,
				X:                actor.X,
				Y:                actor.Y,
				RaceNum:          actor.RaceNum,
				CombatProfile:    actor.CombatProfile,
				RewardExperience: actor.RewardExperience,
				RewardGold:       actor.RewardGold,
				RewardDropVnums:  cloneUint32s(actor.RewardDropVnums),
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
	bundle.ItemTemplates = filterReferencedItemTemplates(items.Templates, referencedItemTemplateVnums(bundle.InteractionDefinitions, bundle.SpawnGroups, bundle.CombatProfiles))
	return Canonicalize(bundle)
}

func Canonicalize(bundle Bundle) (Bundle, error) {
	normalizedStaticActors := normalizeStaticActors(bundle.StaticActors)
	normalizedCombatProfiles := normalizeCombatProfiles(bundle.CombatProfiles)
	normalizedSpawnGroups := normalizeSpawnGroups(bundle.SpawnGroups, normalizedCombatProfiles)
	normalized := Bundle{
		StaticActors:           normalizedStaticActors,
		SpawnGroups:            normalizedSpawnGroups,
		CombatProfiles:         combatProfilesForAuthoredActors(normalizedStaticActors, normalizedSpawnGroups, normalizedCombatProfiles),
		ItemTemplates:          normalizeItemTemplates(bundle.ItemTemplates),
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
								if normalized.SpawnGroups[i].RewardExperience == normalized.SpawnGroups[j].RewardExperience {
									if normalized.SpawnGroups[i].RewardGold == normalized.SpawnGroups[j].RewardGold {
										if compareUint32s(normalized.SpawnGroups[i].RewardDropVnums, normalized.SpawnGroups[j].RewardDropVnums) == 0 {
											return normalized.SpawnGroups[i].Name < normalized.SpawnGroups[j].Name
										}
										return compareUint32s(normalized.SpawnGroups[i].RewardDropVnums, normalized.SpawnGroups[j].RewardDropVnums) < 0
									}
									return normalized.SpawnGroups[i].RewardGold < normalized.SpawnGroups[j].RewardGold
								}
								return normalized.SpawnGroups[i].RewardExperience < normalized.SpawnGroups[j].RewardExperience
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
	sort.Slice(normalized.ItemTemplates, func(i int, j int) bool {
		return normalized.ItemTemplates[i].Vnum < normalized.ItemTemplates[j].Vnum
	})
	if err := validateBundle(normalized); err != nil {
		return Bundle{}, err
	}
	return normalized, nil
}

func Summarize(bundle Bundle) (Summary, error) {
	normalized, err := Canonicalize(bundle)
	if err != nil {
		return Summary{}, err
	}

	summary := Summary{
		StaticActorCount:           len(normalized.StaticActors),
		SpawnGroupCount:            len(normalized.SpawnGroups),
		CombatProfileCount:         len(normalized.CombatProfiles),
		CombatProfiles:             cloneCombatProfileSnapshots(normalized.CombatProfiles),
		ItemTemplateCount:          len(normalized.ItemTemplates),
		StaticActors:               cloneStaticActors(normalized.StaticActors),
		InteractionDefinitionCount: len(normalized.InteractionDefinitions),
	}
	itemTemplatesByVnum := itemTemplateMapByVnum(normalized.ItemTemplates)
	definitionsByKey := interactionDefinitionMapByKey(normalized.InteractionDefinitions)

	referencedDefinitions := make(map[string]struct{})
	for _, actor := range normalized.StaticActors {
		if actor.InteractionKind != "" && actor.InteractionRef != "" {
			referencedDefinitions[interactionDefinitionKey(actor.InteractionKind, actor.InteractionRef)] = struct{}{}
		}
	}
	summary.ReferencedInteractionDefinitionCount = len(referencedDefinitions)
	summary.UnreferencedInteractionDefinitionCount = summary.InteractionDefinitionCount - summary.ReferencedInteractionDefinitionCount

	interactionKindCounts := make(map[string]int)
	interactionKindReferencedCounts := make(map[string]int)
	interactionKindUnreferencedCounts := make(map[string]int)
	for _, definition := range normalized.InteractionDefinitions {
		interactionKindCounts[definition.Kind]++
		summary.InteractionDefinitionPreviews = append(summary.InteractionDefinitionPreviews, interactionDefinitionPreviewSummary(definition, itemTemplatesByVnum))
		if definition.Kind == interactionstore.KindShopPreview {
			summary.ShopCatalogEntryCount += len(definition.Catalog)
			summary.ShopCatalogs = append(summary.ShopCatalogs, shopCatalogSummary(definition, itemTemplatesByVnum))
		}
		if definition.Kind == interactionstore.KindWarp {
			summary.WarpDestinationCount++
			summary.WarpDestinations = append(summary.WarpDestinations, warpDestinationSummary(definition))
		}
		reference := InteractionDefinitionReferenceSummary{Kind: definition.Kind, Ref: definition.Ref}
		if _, ok := referencedDefinitions[interactionDefinitionKey(definition.Kind, definition.Ref)]; ok {
			interactionKindReferencedCounts[definition.Kind]++
			summary.ReferencedInteractionDefinitions = append(summary.ReferencedInteractionDefinitions, reference)
			continue
		}
		interactionKindUnreferencedCounts[definition.Kind]++
		summary.UnreferencedInteractionDefinitions = append(summary.UnreferencedInteractionDefinitions, reference)
	}
	summary.ItemTemplates = itemTemplateReferenceSummaries(normalized.ItemTemplates)
	if len(interactionKindCounts) > 0 {
		kinds := make([]string, 0, len(interactionKindCounts))
		for kind := range interactionKindCounts {
			kinds = append(kinds, kind)
		}
		sort.Strings(kinds)
		summary.InteractionKinds = make([]InteractionKindSummary, 0, len(kinds))
		for _, kind := range kinds {
			summary.InteractionKinds = append(summary.InteractionKinds, InteractionKindSummary{
				Kind:              kind,
				Count:             interactionKindCounts[kind],
				ReferencedCount:   interactionKindReferencedCounts[kind],
				UnreferencedCount: interactionKindUnreferencedCounts[kind],
			})
		}
	}

	mapCounts := make(map[uint32]*MapContentSummary)
	for _, actor := range normalized.StaticActors {
		entry := mapContentSummaryForIndex(mapCounts, actor.MapIndex)
		entry.StaticActorCount++
		if actor.InteractionKind != "" && actor.InteractionRef != "" {
			summary.InteractableStaticActorCount++
			entry.InteractableStaticActorCount++
			definition := definitionsByKey[interactionDefinitionKey(actor.InteractionKind, actor.InteractionRef)]
			summary.InteractableStaticActors = append(summary.InteractableStaticActors, interactableStaticActorSummary(actor, definition, itemTemplatesByVnum))
		}
	}
	rewardDropCountsByVnum := make(map[uint32]int)
	for _, spawnGroup := range normalized.SpawnGroups {
		entry := mapContentSummaryForIndex(mapCounts, spawnGroup.MapIndex)
		entry.SpawnGroupCount++
		summary.RewardExperienceTotal += spawnGroup.RewardExperience
		summary.RewardGoldTotal += spawnGroup.RewardGold
		summary.RewardDropItemCount += len(spawnGroup.RewardDropVnums)
		for _, vnum := range spawnGroup.RewardDropVnums {
			rewardDropCountsByVnum[vnum]++
		}
		summary.SpawnGroups = append(summary.SpawnGroups, SpawnGroupReferenceSummary{
			Ref:              spawnGroup.Ref,
			Name:             spawnGroup.Name,
			MapIndex:         spawnGroup.MapIndex,
			X:                spawnGroup.X,
			Y:                spawnGroup.Y,
			RaceNum:          spawnGroup.RaceNum,
			CombatProfile:    spawnGroup.CombatProfile,
			RewardExperience: spawnGroup.RewardExperience,
			RewardGold:       spawnGroup.RewardGold,
			RewardDropVnums:  cloneUint32s(spawnGroup.RewardDropVnums),
			RewardDropItems:  rewardDropItemSummaries(spawnGroup.RewardDropVnums, itemTemplatesByVnum),
		})
	}
	summary.RewardDrops = rewardDropAggregateSummaries(rewardDropCountsByVnum, itemTemplatesByVnum)
	if len(mapCounts) > 0 {
		mapIndexes := make([]uint32, 0, len(mapCounts))
		for mapIndex := range mapCounts {
			mapIndexes = append(mapIndexes, mapIndex)
		}
		sort.Slice(mapIndexes, func(i int, j int) bool { return mapIndexes[i] < mapIndexes[j] })
		summary.Maps = make([]MapContentSummary, 0, len(mapIndexes))
		for _, mapIndex := range mapIndexes {
			summary.Maps = append(summary.Maps, *mapCounts[mapIndex])
		}
	}

	return summary, nil
}

func itemTemplateMapByVnum(templates []itemcatalog.Template) map[uint32]itemcatalog.Template {
	byVnum := make(map[uint32]itemcatalog.Template, len(templates))
	for _, template := range templates {
		template = itemcatalog.NormalizeTemplate(template)
		byVnum[template.Vnum] = template
	}
	return byVnum
}

func interactionDefinitionMapByKey(definitions []interactionstore.Definition) map[string]interactionstore.Definition {
	byKey := make(map[string]interactionstore.Definition, len(definitions))
	for _, definition := range definitions {
		definition = interactionstore.NormalizeDefinition(definition)
		byKey[interactionDefinitionKey(definition.Kind, definition.Ref)] = definition
	}
	return byKey
}

func interactableStaticActorSummary(actor StaticActor, definition interactionstore.Definition, itemTemplatesByVnum map[uint32]itemcatalog.Template) InteractableStaticActorSummary {
	actor = normalizeStaticActors([]StaticActor{actor})[0]
	definition = interactionstore.NormalizeDefinition(definition)
	return InteractableStaticActorSummary{
		Name:            actor.Name,
		MapIndex:        actor.MapIndex,
		X:               actor.X,
		Y:               actor.Y,
		RaceNum:         actor.RaceNum,
		InteractionKind: actor.InteractionKind,
		InteractionRef:  actor.InteractionRef,
		Preview:         compactInteractionPreview(interactionDefinitionPreview(actor.Name, definition, itemTemplatesByVnum)),
	}
}

func interactionDefinitionPreview(actorName string, definition interactionstore.Definition, itemTemplatesByVnum map[uint32]itemcatalog.Template) string {
	switch definition.Kind {
	case interactionstore.KindInfo:
		return definition.Text
	case interactionstore.KindTalk:
		return fmt.Sprintf("%s:\n%s", actorName, definition.Text)
	case interactionstore.KindShopPreview:
		return shopCatalogPreview(definition, itemTemplatesByVnum)
	case interactionstore.KindWarp:
		return warpDestinationPreview(definition)
	default:
		return ""
	}
}

func interactionDefinitionPreviewSummary(definition interactionstore.Definition, itemTemplatesByVnum map[uint32]itemcatalog.Template) InteractionDefinitionPreviewSummary {
	definition = interactionstore.NormalizeDefinition(definition)
	return InteractionDefinitionPreviewSummary{
		Kind:    definition.Kind,
		Ref:     definition.Ref,
		Preview: compactInteractionPreview(interactionDefinitionCatalogPreview(definition, itemTemplatesByVnum)),
	}
}

func interactionDefinitionCatalogPreview(definition interactionstore.Definition, itemTemplatesByVnum map[uint32]itemcatalog.Template) string {
	switch definition.Kind {
	case interactionstore.KindInfo, interactionstore.KindTalk:
		return definition.Text
	case interactionstore.KindShopPreview:
		return shopCatalogPreview(definition, itemTemplatesByVnum)
	case interactionstore.KindWarp:
		return warpDestinationPreview(definition)
	default:
		return ""
	}
}

func shopCatalogPreview(definition interactionstore.Definition, itemTemplatesByVnum map[uint32]itemcatalog.Template) string {
	if definition.Kind != interactionstore.KindShopPreview || len(definition.Catalog) == 0 {
		return ""
	}
	entries := make([]string, 0, len(definition.Catalog))
	for _, entry := range definition.Catalog {
		template := itemcatalog.NormalizeTemplate(itemTemplatesByVnum[entry.ItemVnum])
		if template.Name == "" {
			return ""
		}
		entries = append(entries, fmt.Sprintf("[%d] %s x%d @ %dg", entry.Slot, template.Name, entry.Count, entry.Price))
	}
	return fmt.Sprintf("%s: %s", definition.Title, strings.Join(entries, "; "))
}

func warpDestinationPreview(definition interactionstore.Definition) string {
	summary := fmt.Sprintf("warp -> map %d @ %d,%d", definition.MapIndex, definition.X, definition.Y)
	message := strings.TrimSpace(definition.Text)
	if message == "" {
		return summary
	}
	return fmt.Sprintf("%s [%s]", message, summary)
}

func compactInteractionPreview(preview string) string {
	preview = strings.TrimSpace(preview)
	const maxPreviewLength = 160
	if len(preview) <= maxPreviewLength {
		return preview
	}
	return preview[:maxPreviewLength-3] + "..."
}

func shopCatalogSummary(definition interactionstore.Definition, itemTemplatesByVnum map[uint32]itemcatalog.Template) ShopCatalogSummary {
	definition = interactionstore.NormalizeDefinition(definition)
	summary := ShopCatalogSummary{
		Kind:       definition.Kind,
		Ref:        definition.Ref,
		Title:      definition.Title,
		EntryCount: len(definition.Catalog),
	}
	if len(definition.Catalog) == 0 {
		return summary
	}
	summary.Entries = make([]ShopCatalogEntrySummary, 0, len(definition.Catalog))
	for _, entry := range definition.Catalog {
		template := itemcatalog.NormalizeTemplate(itemTemplatesByVnum[entry.ItemVnum])
		summary.Entries = append(summary.Entries, ShopCatalogEntrySummary{
			Slot:         entry.Slot,
			ItemVnum:     entry.ItemVnum,
			ItemName:     template.Name,
			Count:        entry.Count,
			Price:        entry.Price,
			Stackable:    template.Stackable,
			MaxCount:     template.MaxCount,
			ShopBuyPrice: template.ShopBuyPrice,
		})
	}
	return summary
}

func warpDestinationSummary(definition interactionstore.Definition) WarpDestinationSummary {
	definition = interactionstore.NormalizeDefinition(definition)
	return WarpDestinationSummary{
		Kind:     definition.Kind,
		Ref:      definition.Ref,
		Text:     definition.Text,
		MapIndex: definition.MapIndex,
		X:        definition.X,
		Y:        definition.Y,
	}
}

func itemTemplateReferenceSummaries(templates []itemcatalog.Template) []ItemTemplateReferenceSummary {
	if len(templates) == 0 {
		return nil
	}
	summaries := make([]ItemTemplateReferenceSummary, 0, len(templates))
	for _, template := range templates {
		template = itemcatalog.NormalizeTemplate(template)
		summaries = append(summaries, ItemTemplateReferenceSummary{
			Vnum:         template.Vnum,
			Name:         template.Name,
			Stackable:    template.Stackable,
			MaxCount:     template.MaxCount,
			ShopBuyPrice: template.ShopBuyPrice,
		})
	}
	sort.Slice(summaries, func(i int, j int) bool {
		return summaries[i].Vnum < summaries[j].Vnum
	})
	return summaries
}

func rewardDropItemSummaries(dropVnums []uint32, itemTemplatesByVnum map[uint32]itemcatalog.Template) []RewardDropItemSummary {
	if len(dropVnums) == 0 {
		return nil
	}
	sortedVnums := cloneUint32s(dropVnums)
	summaries := make([]RewardDropItemSummary, 0, len(sortedVnums))
	for _, vnum := range sortedVnums {
		template := itemcatalog.NormalizeTemplate(itemTemplatesByVnum[vnum])
		if template.Vnum == 0 || template.Name == "" {
			continue
		}
		summaries = append(summaries, RewardDropItemSummary{
			ItemVnum:     template.Vnum,
			ItemName:     template.Name,
			Stackable:    template.Stackable,
			MaxCount:     template.MaxCount,
			ShopBuyPrice: template.ShopBuyPrice,
		})
	}
	if len(summaries) == 0 {
		return nil
	}
	return summaries
}

func rewardDropAggregateSummaries(countsByVnum map[uint32]int, itemTemplatesByVnum map[uint32]itemcatalog.Template) []RewardDropAggregateSummary {
	if len(countsByVnum) == 0 {
		return nil
	}
	vnums := make([]uint32, 0, len(countsByVnum))
	for vnum := range countsByVnum {
		vnums = append(vnums, vnum)
	}
	sort.Slice(vnums, func(i int, j int) bool { return vnums[i] < vnums[j] })
	summaries := make([]RewardDropAggregateSummary, 0, len(vnums))
	for _, vnum := range vnums {
		template := itemcatalog.NormalizeTemplate(itemTemplatesByVnum[vnum])
		if template.Vnum == 0 || template.Name == "" {
			continue
		}
		summaries = append(summaries, RewardDropAggregateSummary{
			ItemVnum:     template.Vnum,
			ItemName:     template.Name,
			SourceCount:  countsByVnum[vnum],
			Stackable:    template.Stackable,
			MaxCount:     template.MaxCount,
			ShopBuyPrice: template.ShopBuyPrice,
		})
	}
	if len(summaries) == 0 {
		return nil
	}
	return summaries
}

func mapContentSummaryForIndex(counts map[uint32]*MapContentSummary, mapIndex uint32) *MapContentSummary {
	entry, ok := counts[mapIndex]
	if ok {
		return entry
	}
	entry = &MapContentSummary{MapIndex: mapIndex}
	counts[mapIndex] = entry
	return entry
}

func validateBundle(bundle Bundle) error {
	itemTemplatesByVnum := make(map[uint32]itemcatalog.Template, len(bundle.ItemTemplates))
	for _, template := range bundle.ItemTemplates {
		normalizedTemplate := itemcatalog.NormalizeTemplate(template)
		if !itemcatalog.ValidTemplate(normalizedTemplate) {
			return ErrInvalidBundle
		}
		if _, ok := itemTemplatesByVnum[normalizedTemplate.Vnum]; ok {
			return ErrInvalidBundle
		}
		itemTemplatesByVnum[normalizedTemplate.Vnum] = normalizedTemplate
	}
	profileSnapshots := make(map[string]worldruntime.StaticActorCombatProfileSnapshot, len(bundle.CombatProfiles))
	referencedProfiles := referencedCombatProfileNames(bundle.StaticActors, bundle.SpawnGroups)
	for _, profile := range bundle.CombatProfiles {
		if !validCombatProfileSnapshot(profile) {
			return ErrInvalidBundle
		}
		if !validRewardDropRefs(profile.DeathReward.DropVnums, itemTemplatesByVnum) {
			return ErrInvalidBundle
		}
		name := strings.TrimSpace(profile.Profile)
		if _, referenced := referencedProfiles[name]; !referenced {
			return ErrInvalidBundle
		}
		if _, ok := profileSnapshots[name]; ok {
			return ErrInvalidBundle
		}
		profileSnapshots[name] = profile
	}
	definitionsByKey := make(map[string]struct{}, len(bundle.InteractionDefinitions))
	for _, definition := range bundle.InteractionDefinitions {
		if !validDefinition(definition) {
			return ErrInvalidBundle
		}
		if definition.Kind == interactionstore.KindShopPreview {
			if len(itemTemplatesByVnum) == 0 {
				return ErrInvalidBundle
			}
			for _, entry := range definition.Catalog {
				template, ok := itemTemplatesByVnum[entry.ItemVnum]
				if !ok {
					return ErrInvalidBundle
				}
				if !validMerchantCatalogCountForTemplate(entry, template) {
					return ErrInvalidBundle
				}
			}
		}
		key := interactionDefinitionKey(definition.Kind, definition.Ref)
		if _, ok := definitionsByKey[key]; ok {
			return ErrInvalidBundle
		}
		definitionsByKey[key] = struct{}{}
	}
	referencedItemVnums := referencedItemTemplateVnums(bundle.InteractionDefinitions, bundle.SpawnGroups, bundle.CombatProfiles)
	for vnum := range itemTemplatesByVnum {
		if _, referenced := referencedItemVnums[vnum]; !referenced {
			return ErrInvalidBundle
		}
	}
	spawnGroupsByRef := make(map[string]struct{}, len(bundle.SpawnGroups))
	for _, spawnGroup := range bundle.SpawnGroups {
		if !validSpawnGroup(spawnGroup, profileSnapshots) {
			return ErrInvalidBundle
		}
		if !validRewardDropRefs(spawnGroup.RewardDropVnums, itemTemplatesByVnum) {
			return ErrInvalidBundle
		}
		if _, ok := spawnGroupsByRef[spawnGroup.Ref]; ok {
			return ErrInvalidBundle
		}
		spawnGroupsByRef[spawnGroup.Ref] = struct{}{}
	}
	staticActorsByKey := make(map[string]struct{}, len(bundle.StaticActors))
	for _, actor := range bundle.StaticActors {
		if strings.TrimSpace(actor.Name) == "" || actor.MapIndex == 0 || !validBootstrapRaceNum(actor.RaceNum) {
			return ErrInvalidBundle
		}
		if !validAuthoredCombatProfile(actor.CombatProfile, profileSnapshots) {
			return ErrInvalidBundle
		}
		if !validInteractionMetadata(actor.InteractionKind, actor.InteractionRef) {
			return ErrInvalidBundle
		}
		key := staticActorAuthoringKey(actor)
		if _, ok := staticActorsByKey[key]; ok {
			return ErrInvalidBundle
		}
		staticActorsByKey[key] = struct{}{}
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
	return kind != "" && ref != "" && interactionstore.ValidRef(ref)
}

func validBootstrapRaceNum(raceNum uint32) bool {
	return worldruntime.ValidStaticActorVisibilityRaceNum(raceNum)
}

func validSpawnGroup(spawnGroup SpawnGroup, profileSnapshots map[string]worldruntime.StaticActorCombatProfileSnapshot) bool {
	if !worldruntime.ValidStaticActorSpawnGroupRef(spawnGroup.Ref) || strings.TrimSpace(spawnGroup.Ref) == "" || strings.TrimSpace(spawnGroup.Name) == "" || spawnGroup.MapIndex == 0 || !validBootstrapRaceNum(spawnGroup.RaceNum) {
		return false
	}
	if strings.TrimSpace(spawnGroup.CombatProfile) == "" || !validAuthoredCombatProfile(spawnGroup.CombatProfile, profileSnapshots) {
		return false
	}
	return validRewardDescriptor(spawnGroup)
}

func validAuthoredCombatProfile(profile string, profileSnapshots map[string]worldruntime.StaticActorCombatProfileSnapshot) bool {
	profile = strings.TrimSpace(profile)
	if profile == "" {
		return true
	}
	if worldruntime.ValidStaticActorCombatProfile(profile) {
		return true
	}
	_, ok := profileSnapshots[profile]
	return ok
}

func validCombatProfileSnapshot(profile worldruntime.StaticActorCombatProfileSnapshot) bool {
	name := strings.TrimSpace(profile.Profile)
	if name == "" || name == worldruntime.StaticActorCombatProfilePracticeMob || name == worldruntime.StaticActorCombatProfileTrainingDummy {
		return false
	}
	if profile.MaxHP == 0 || profile.AttackValue == 0 || profile.RespawnDelayMs <= 0 {
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

func validRewardDescriptor(spawnGroup SpawnGroup) bool {
	return worldruntime.ValidStaticActorDeathReward(worldruntime.StaticActorDeathReward{Experience: spawnGroup.RewardExperience, Gold: spawnGroup.RewardGold, DropVnums: spawnGroup.RewardDropVnums})
}

func validMerchantCatalogCountForTemplate(entry interactionstore.MerchantCatalogEntry, template itemcatalog.Template) bool {
	if entry.Count == 0 || template.MaxCount == 0 {
		return false
	}
	if !template.Stackable {
		return entry.Count == 1
	}
	return entry.Count <= template.MaxCount
}

func validRewardDropRefs(dropVnums []uint32, itemTemplatesByVnum map[uint32]itemcatalog.Template) bool {
	if len(dropVnums) == 0 {
		return true
	}
	if len(itemTemplatesByVnum) == 0 {
		return false
	}
	for _, vnum := range dropVnums {
		if _, ok := itemTemplatesByVnum[vnum]; !ok {
			return false
		}
	}
	return true
}

func normalizeSpawnGroups(spawnGroups []SpawnGroup, profileSnapshots []worldruntime.StaticActorCombatProfileSnapshot) []SpawnGroup {
	if len(spawnGroups) == 0 {
		return nil
	}
	profileRewards := make(map[string]worldruntime.StaticActorDeathReward, len(profileSnapshots))
	for _, snapshot := range profileSnapshots {
		profileRewards[strings.TrimSpace(snapshot.Profile)] = snapshot.DeathReward.Clone()
	}
	normalized := make([]SpawnGroup, len(spawnGroups))
	for i, spawnGroup := range spawnGroups {
		spawnGroup.Name = strings.TrimSpace(spawnGroup.Name)
		spawnGroup.CombatProfile = strings.TrimSpace(spawnGroup.CombatProfile)
		if spawnGroup.CombatProfile == "" {
			spawnGroup.CombatProfile = worldruntime.StaticActorCombatProfilePracticeMob
		}
		if spawnGroup.RewardExperience == 0 && spawnGroup.RewardGold == 0 && len(spawnGroup.RewardDropVnums) == 0 {
			if reward, ok := worldruntime.BootstrapStaticActorDeathReward(spawnGroup.CombatProfile); ok {
				spawnGroup.RewardExperience = reward.Experience
				spawnGroup.RewardGold = reward.Gold
				spawnGroup.RewardDropVnums = reward.DropVnums
			} else if reward, ok := profileRewards[spawnGroup.CombatProfile]; ok {
				spawnGroup.RewardExperience = reward.Experience
				spawnGroup.RewardGold = reward.Gold
				spawnGroup.RewardDropVnums = reward.DropVnums
			}
		}
		spawnGroup.RewardDropVnums = cloneUint32s(spawnGroup.RewardDropVnums)
		normalized[i] = spawnGroup
	}
	return normalized
}

func combatProfilesForAuthoredActors(staticActors []StaticActor, spawnGroups []SpawnGroup, importedProfiles []worldruntime.StaticActorCombatProfileSnapshot) []worldruntime.StaticActorCombatProfileSnapshot {
	seen := make(map[string]struct{})
	profiles := make([]worldruntime.StaticActorCombatProfileSnapshot, 0)
	for _, profile := range importedProfiles {
		profile.Profile = strings.TrimSpace(profile.Profile)
		profile.DeathReward = profile.DeathReward.Clone()
		profiles = append(profiles, profile)
		seen[profile.Profile] = struct{}{}
	}
	for _, actor := range staticActors {
		profiles = appendCombatProfileSnapshot(profiles, seen, actor.CombatProfile)
	}
	for _, spawnGroup := range spawnGroups {
		profiles = appendCombatProfileSnapshot(profiles, seen, spawnGroup.CombatProfile)
	}
	sort.Slice(profiles, func(i int, j int) bool {
		return profiles[i].Profile < profiles[j].Profile
	})
	if len(profiles) == 0 {
		return nil
	}
	return profiles
}

func normalizeCombatProfiles(profiles []worldruntime.StaticActorCombatProfileSnapshot) []worldruntime.StaticActorCombatProfileSnapshot {
	normalized := cloneCombatProfileSnapshots(profiles)
	if len(normalized) == 0 {
		return nil
	}
	sort.Slice(normalized, func(i int, j int) bool {
		return normalized[i].Profile < normalized[j].Profile
	})
	return normalized
}

func cloneCombatProfileSnapshots(profiles []worldruntime.StaticActorCombatProfileSnapshot) []worldruntime.StaticActorCombatProfileSnapshot {
	if len(profiles) == 0 {
		return nil
	}
	cloned := make([]worldruntime.StaticActorCombatProfileSnapshot, len(profiles))
	copy(cloned, profiles)
	for i := range cloned {
		cloned[i].Profile = strings.TrimSpace(cloned[i].Profile)
		cloned[i].DeathReward = cloned[i].DeathReward.Clone()
	}
	return cloned
}

func referencedCombatProfileNames(staticActors []StaticActor, spawnGroups []SpawnGroup) map[string]struct{} {
	referenced := make(map[string]struct{}, len(staticActors)+len(spawnGroups))
	for _, actor := range staticActors {
		profile := strings.TrimSpace(actor.CombatProfile)
		if profile != "" {
			referenced[profile] = struct{}{}
		}
	}
	for _, spawnGroup := range spawnGroups {
		profile := strings.TrimSpace(spawnGroup.CombatProfile)
		if profile != "" {
			referenced[profile] = struct{}{}
		}
	}
	return referenced
}

func appendCombatProfileSnapshot(profiles []worldruntime.StaticActorCombatProfileSnapshot, seen map[string]struct{}, profile string) []worldruntime.StaticActorCombatProfileSnapshot {
	profile = strings.TrimSpace(profile)
	if profile == "" || profile == worldruntime.StaticActorCombatProfilePracticeMob || profile == worldruntime.StaticActorCombatProfileTrainingDummy {
		return profiles
	}
	if _, ok := seen[profile]; ok {
		return profiles
	}
	for _, snapshot := range worldruntime.StaticActorCombatProfileSnapshots() {
		if snapshot.Profile != profile {
			continue
		}
		profiles = append(profiles, snapshot)
		seen[profile] = struct{}{}
		return profiles
	}
	return profiles
}

func cloneUint32s(values []uint32) []uint32 {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]uint32, len(values))
	copy(cloned, values)
	sort.Slice(cloned, func(i int, j int) bool {
		return cloned[i] < cloned[j]
	})
	return cloned
}

func compareUint32s(left []uint32, right []uint32) int {
	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}
	for i := 0; i < limit; i++ {
		if left[i] < right[i] {
			return -1
		}
		if left[i] > right[i] {
			return 1
		}
	}
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	return 0
}

func interactionDefinitionKey(kind string, ref string) string {
	return strings.TrimSpace(kind) + "\x00" + strings.TrimSpace(ref)
}

func staticActorAuthoringKey(actor StaticActor) string {
	return strings.Join([]string{
		strings.TrimSpace(actor.Name),
		fmt.Sprintf("%d", actor.MapIndex),
		fmt.Sprintf("%d", actor.X),
		fmt.Sprintf("%d", actor.Y),
		fmt.Sprintf("%d", actor.RaceNum),
		strings.TrimSpace(actor.CombatProfile),
		strings.TrimSpace(actor.InteractionKind),
		strings.TrimSpace(actor.InteractionRef),
	}, "\x00")
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
	for i, definition := range definitions {
		cloned[i] = interactionstore.NormalizeDefinition(definition)
	}
	return cloned
}

func normalizeItemTemplates(templates []itemcatalog.Template) []itemcatalog.Template {
	if len(templates) == 0 {
		return nil
	}
	normalized := make([]itemcatalog.Template, len(templates))
	for i, template := range templates {
		normalized[i] = itemcatalog.NormalizeTemplate(template)
	}
	return normalized
}

func filterReferencedItemTemplates(templates []itemcatalog.Template, referenced map[uint32]struct{}) []itemcatalog.Template {
	if len(templates) == 0 || len(referenced) == 0 {
		return nil
	}
	filtered := make([]itemcatalog.Template, 0, len(templates))
	for _, template := range normalizeItemTemplates(templates) {
		if _, ok := referenced[template.Vnum]; ok {
			filtered = append(filtered, template)
		}
	}
	return filtered
}

func referencedItemTemplateVnums(definitions []interactionstore.Definition, spawnGroups []SpawnGroup, combatProfiles []worldruntime.StaticActorCombatProfileSnapshot) map[uint32]struct{} {
	referenced := make(map[uint32]struct{})
	for _, definition := range definitions {
		definition = interactionstore.NormalizeDefinition(definition)
		if definition.Kind != interactionstore.KindShopPreview {
			continue
		}
		for _, entry := range definition.Catalog {
			if entry.ItemVnum != 0 {
				referenced[entry.ItemVnum] = struct{}{}
			}
		}
	}
	for _, spawnGroup := range spawnGroups {
		for _, vnum := range spawnGroup.RewardDropVnums {
			if vnum != 0 {
				referenced[vnum] = struct{}{}
			}
		}
	}
	for _, profile := range combatProfiles {
		for _, vnum := range profile.DeathReward.DropVnums {
			if vnum != 0 {
				referenced[vnum] = struct{}{}
			}
		}
	}
	return referenced
}
