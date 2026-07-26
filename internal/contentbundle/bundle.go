package contentbundle

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

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
	ShopRouteCount                         int                                             `json:"shop_route_count"`
	ShopRoutes                             []ShopRouteSummary                              `json:"shop_routes,omitempty"`
	WarpDestinationCount                   int                                             `json:"warp_destination_count"`
	WarpDestinations                       []WarpDestinationSummary                        `json:"warp_destinations,omitempty"`
	WarpRouteCount                         int                                             `json:"warp_route_count"`
	WarpRoutes                             []WarpRouteSummary                              `json:"warp_routes,omitempty"`
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

type ImportPreview struct {
	Current   Summary       `json:"current"`
	Candidate Summary       `json:"candidate"`
	Deltas    SummaryDeltas `json:"deltas"`
}

type SummaryDeltas struct {
	StaticActorCount                       SummaryCountDelta            `json:"static_actor_count"`
	InteractableStaticActorCount           SummaryCountDelta            `json:"interactable_static_actor_count"`
	SpawnGroupCount                        SummaryCountDelta            `json:"spawn_group_count"`
	CombatProfileCount                     SummaryCountDelta            `json:"combat_profile_count"`
	ItemTemplateCount                      SummaryCountDelta            `json:"item_template_count"`
	ShopCatalogEntryCount                  SummaryCountDelta            `json:"shop_catalog_entry_count"`
	ShopRouteCount                         SummaryCountDelta            `json:"shop_route_count"`
	WarpDestinationCount                   SummaryCountDelta            `json:"warp_destination_count"`
	WarpRouteCount                         SummaryCountDelta            `json:"warp_route_count"`
	RewardExperienceTotal                  SummaryAmountDelta           `json:"reward_experience_total"`
	RewardGoldTotal                        SummaryAmountDelta           `json:"reward_gold_total"`
	RewardDropItemCount                    SummaryCountDelta            `json:"reward_drop_item_count"`
	InteractionDefinitionCount             SummaryCountDelta            `json:"interaction_definition_count"`
	ReferencedInteractionDefinitionCount   SummaryCountDelta            `json:"referenced_interaction_definition_count"`
	UnreferencedInteractionDefinitionCount SummaryCountDelta            `json:"unreferenced_interaction_definition_count"`
	StaticActors                           []StaticActorDelta           `json:"static_actors,omitempty"`
	InteractionKinds                       []InteractionKindDelta       `json:"interaction_kinds,omitempty"`
	InteractionDefinitions                 []InteractionDefinitionDelta `json:"interaction_definitions,omitempty"`
	ItemTemplates                          []ItemTemplateDelta          `json:"item_templates,omitempty"`
	CombatProfiles                         []CombatProfileDelta         `json:"combat_profiles,omitempty"`
	SpawnGroups                            []SpawnGroupDelta            `json:"spawn_groups,omitempty"`
	Maps                                   []MapContentDelta            `json:"maps,omitempty"`
}

type SummaryCountDelta struct {
	Current   int `json:"current"`
	Candidate int `json:"candidate"`
	Delta     int `json:"delta"`
}

type SummaryAmountDelta struct {
	Current   uint64 `json:"current"`
	Candidate uint64 `json:"candidate"`
	Delta     int64  `json:"delta"`
}

type InteractionKindDelta struct {
	Kind              string            `json:"kind"`
	Count             SummaryCountDelta `json:"count"`
	ReferencedCount   SummaryCountDelta `json:"referenced_count"`
	UnreferencedCount SummaryCountDelta `json:"unreferenced_count"`
}

type InteractionDefinitionDelta struct {
	Kind             string `json:"kind"`
	Ref              string `json:"ref"`
	Change           string `json:"change"`
	CurrentPreview   string `json:"current_preview,omitempty"`
	CandidatePreview string `json:"candidate_preview,omitempty"`
}

type StaticActorDelta struct {
	Change    string       `json:"change"`
	Current   *StaticActor `json:"current,omitempty"`
	Candidate *StaticActor `json:"candidate,omitempty"`
}

type ItemTemplateDelta struct {
	Vnum      uint32                `json:"vnum"`
	Change    string                `json:"change"`
	Current   *itemcatalog.Template `json:"current,omitempty"`
	Candidate *itemcatalog.Template `json:"candidate,omitempty"`
}

type CombatProfileDelta struct {
	Profile   string                                         `json:"profile"`
	Change    string                                         `json:"change"`
	Current   *worldruntime.StaticActorCombatProfileSnapshot `json:"current,omitempty"`
	Candidate *worldruntime.StaticActorCombatProfileSnapshot `json:"candidate,omitempty"`
}

type SpawnGroupDelta struct {
	Ref       string                      `json:"ref"`
	Change    string                      `json:"change"`
	Current   *SpawnGroupReferenceSummary `json:"current,omitempty"`
	Candidate *SpawnGroupReferenceSummary `json:"candidate,omitempty"`
}

type MapContentDelta struct {
	MapIndex                     uint32             `json:"map_index"`
	StaticActorCount             SummaryCountDelta  `json:"static_actor_count"`
	InteractableStaticActorCount SummaryCountDelta  `json:"interactable_static_actor_count"`
	InfoActorCount               SummaryCountDelta  `json:"info_actor_count,omitempty"`
	TalkActorCount               SummaryCountDelta  `json:"talk_actor_count,omitempty"`
	ShopPreviewActorCount        SummaryCountDelta  `json:"shop_preview_actor_count,omitempty"`
	ShopCatalogEntryCount        SummaryCountDelta  `json:"shop_catalog_entry_count,omitempty"`
	WarpActorCount               SummaryCountDelta  `json:"warp_actor_count,omitempty"`
	SpawnGroupCount              SummaryCountDelta  `json:"spawn_group_count"`
	RewardExperienceTotal        SummaryAmountDelta `json:"reward_experience_total,omitempty"`
	RewardGoldTotal              SummaryAmountDelta `json:"reward_gold_total,omitempty"`
	RewardDropItemCount          SummaryCountDelta  `json:"reward_drop_item_count,omitempty"`
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

type ShopRouteSummary struct {
	ActorName      string `json:"actor_name"`
	SourceMapIndex uint32 `json:"source_map_index"`
	SourceX        int32  `json:"source_x"`
	SourceY        int32  `json:"source_y"`
	Ref            string `json:"ref"`
	Title          string `json:"title"`
	EntryCount     int    `json:"entry_count"`
}

type WarpDestinationSummary struct {
	Kind     string `json:"kind"`
	Ref      string `json:"ref"`
	Text     string `json:"text,omitempty"`
	MapIndex uint32 `json:"map_index"`
	X        int32  `json:"x"`
	Y        int32  `json:"y"`
}

type WarpRouteSummary struct {
	ActorName      string `json:"actor_name"`
	SourceMapIndex uint32 `json:"source_map_index"`
	SourceX        int32  `json:"source_x"`
	SourceY        int32  `json:"source_y"`
	Ref            string `json:"ref"`
	Text           string `json:"text,omitempty"`
	TargetMapIndex uint32 `json:"target_map_index"`
	TargetX        int32  `json:"target_x"`
	TargetY        int32  `json:"target_y"`
}

type MapContentSummary struct {
	MapIndex                     uint32 `json:"map_index"`
	StaticActorCount             int    `json:"static_actor_count"`
	InteractableStaticActorCount int    `json:"interactable_static_actor_count"`
	InfoActorCount               int    `json:"info_actor_count,omitempty"`
	TalkActorCount               int    `json:"talk_actor_count,omitempty"`
	ShopPreviewActorCount        int    `json:"shop_preview_actor_count,omitempty"`
	ShopCatalogEntryCount        int    `json:"shop_catalog_entry_count,omitempty"`
	WarpActorCount               int    `json:"warp_actor_count,omitempty"`
	SpawnGroupCount              int    `json:"spawn_group_count"`
	RewardExperienceTotal        uint64 `json:"reward_experience_total,omitempty"`
	RewardGoldTotal              uint64 `json:"reward_gold_total,omitempty"`
	RewardDropItemCount          int    `json:"reward_drop_item_count,omitempty"`
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
	normalizedStaticActors := normalizeStaticActors(bundle.StaticActors)
	normalizedSpawnGroups := normalizeSpawnGroups(bundle.SpawnGroups, nil)
	portableCombatProfiles := combatProfilesForAuthoredActors(normalizedStaticActors, normalizedSpawnGroups, nil)
	bundle.ItemTemplates = filterReferencedItemTemplates(items.Templates, referencedItemTemplateVnums(bundle.InteractionDefinitions, normalizedSpawnGroups, portableCombatProfiles))
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

func BuildImportPreview(current Bundle, candidate Bundle) (ImportPreview, error) {
	currentCanonical, err := Canonicalize(current)
	if err != nil {
		return ImportPreview{}, err
	}
	candidateCanonical, err := Canonicalize(candidate)
	if err != nil {
		return ImportPreview{}, err
	}
	currentSummary, err := Summarize(currentCanonical)
	if err != nil {
		return ImportPreview{}, err
	}
	candidateSummary, err := Summarize(candidateCanonical)
	if err != nil {
		return ImportPreview{}, err
	}
	return ImportPreview{
		Current:   currentSummary,
		Candidate: candidateSummary,
		Deltas:    buildSummaryDeltas(currentSummary, candidateSummary, currentCanonical, candidateCanonical),
	}, nil
}

func buildSummaryDeltas(current Summary, candidate Summary, currentBundle Bundle, candidateBundle Bundle) SummaryDeltas {
	return SummaryDeltas{
		StaticActorCount:                       summaryCountDelta(current.StaticActorCount, candidate.StaticActorCount),
		InteractableStaticActorCount:           summaryCountDelta(current.InteractableStaticActorCount, candidate.InteractableStaticActorCount),
		SpawnGroupCount:                        summaryCountDelta(current.SpawnGroupCount, candidate.SpawnGroupCount),
		CombatProfileCount:                     summaryCountDelta(current.CombatProfileCount, candidate.CombatProfileCount),
		ItemTemplateCount:                      summaryCountDelta(current.ItemTemplateCount, candidate.ItemTemplateCount),
		ShopCatalogEntryCount:                  summaryCountDelta(current.ShopCatalogEntryCount, candidate.ShopCatalogEntryCount),
		ShopRouteCount:                         summaryCountDelta(current.ShopRouteCount, candidate.ShopRouteCount),
		WarpDestinationCount:                   summaryCountDelta(current.WarpDestinationCount, candidate.WarpDestinationCount),
		WarpRouteCount:                         summaryCountDelta(current.WarpRouteCount, candidate.WarpRouteCount),
		RewardExperienceTotal:                  summaryAmountDelta(current.RewardExperienceTotal, candidate.RewardExperienceTotal),
		RewardGoldTotal:                        summaryAmountDelta(current.RewardGoldTotal, candidate.RewardGoldTotal),
		RewardDropItemCount:                    summaryCountDelta(current.RewardDropItemCount, candidate.RewardDropItemCount),
		InteractionDefinitionCount:             summaryCountDelta(current.InteractionDefinitionCount, candidate.InteractionDefinitionCount),
		ReferencedInteractionDefinitionCount:   summaryCountDelta(current.ReferencedInteractionDefinitionCount, candidate.ReferencedInteractionDefinitionCount),
		UnreferencedInteractionDefinitionCount: summaryCountDelta(current.UnreferencedInteractionDefinitionCount, candidate.UnreferencedInteractionDefinitionCount),
		StaticActors:                           buildStaticActorDeltas(currentBundle.StaticActors, candidateBundle.StaticActors),
		InteractionKinds:                       buildInteractionKindDeltas(current.InteractionKinds, candidate.InteractionKinds),
		InteractionDefinitions:                 buildInteractionDefinitionDeltas(currentBundle, candidateBundle),
		ItemTemplates:                          buildItemTemplateDeltas(currentBundle.ItemTemplates, candidateBundle.ItemTemplates),
		CombatProfiles:                         buildCombatProfileDeltas(currentBundle.CombatProfiles, candidateBundle.CombatProfiles),
		SpawnGroups:                            buildSpawnGroupDeltas(current.SpawnGroups, candidate.SpawnGroups),
		Maps:                                   buildMapContentDeltas(current.Maps, candidate.Maps),
	}
}

func summaryCountDelta(current int, candidate int) SummaryCountDelta {
	return SummaryCountDelta{Current: current, Candidate: candidate, Delta: candidate - current}
}

func summaryAmountDelta(current uint64, candidate uint64) SummaryAmountDelta {
	delta := int64(0)
	if candidate >= current {
		delta = int64(candidate - current)
	} else {
		delta = -int64(current - candidate)
	}
	return SummaryAmountDelta{Current: current, Candidate: candidate, Delta: delta}
}

func buildInteractionKindDeltas(currentKinds []InteractionKindSummary, candidateKinds []InteractionKindSummary) []InteractionKindDelta {
	if len(currentKinds) == 0 && len(candidateKinds) == 0 {
		return nil
	}
	currentByKind := make(map[string]InteractionKindSummary, len(currentKinds))
	candidateByKind := make(map[string]InteractionKindSummary, len(candidateKinds))
	kindsSeen := make(map[string]struct{}, len(currentKinds)+len(candidateKinds))
	for _, summary := range currentKinds {
		currentByKind[summary.Kind] = summary
		kindsSeen[summary.Kind] = struct{}{}
	}
	for _, summary := range candidateKinds {
		candidateByKind[summary.Kind] = summary
		kindsSeen[summary.Kind] = struct{}{}
	}
	kinds := make([]string, 0, len(kindsSeen))
	for kind := range kindsSeen {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	deltas := make([]InteractionKindDelta, 0, len(kinds))
	for _, kind := range kinds {
		current := currentByKind[kind]
		candidate := candidateByKind[kind]
		delta := InteractionKindDelta{
			Kind:              kind,
			Count:             summaryCountDelta(current.Count, candidate.Count),
			ReferencedCount:   summaryCountDelta(current.ReferencedCount, candidate.ReferencedCount),
			UnreferencedCount: summaryCountDelta(current.UnreferencedCount, candidate.UnreferencedCount),
		}
		if !interactionKindDeltaIsZero(delta) {
			deltas = append(deltas, delta)
		}
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func interactionKindDeltaIsZero(delta InteractionKindDelta) bool {
	return delta.Count.Delta == 0 &&
		delta.ReferencedCount.Delta == 0 &&
		delta.UnreferencedCount.Delta == 0
}

func buildStaticActorDeltas(currentActors []StaticActor, candidateActors []StaticActor) []StaticActorDelta {
	if len(currentActors) == 0 && len(candidateActors) == 0 {
		return nil
	}
	currentByKey := staticActorMapByAuthoringKey(currentActors)
	candidateByKey := staticActorMapByAuthoringKey(candidateActors)
	keysSeen := make(map[string]struct{}, len(currentByKey)+len(candidateByKey))
	for key := range currentByKey {
		keysSeen[key] = struct{}{}
	}
	for key := range candidateByKey {
		keysSeen[key] = struct{}{}
	}
	keys := make([]string, 0, len(keysSeen))
	for key := range keysSeen {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	deltas := make([]StaticActorDelta, 0, len(keys))
	for _, key := range keys {
		current, currentOK := currentByKey[key]
		candidate, candidateOK := candidateByKey[key]
		switch {
		case !currentOK:
			candidateCopy := candidate
			deltas = append(deltas, StaticActorDelta{Change: "added", Candidate: &candidateCopy})
		case !candidateOK:
			currentCopy := current
			deltas = append(deltas, StaticActorDelta{Change: "removed", Current: &currentCopy})
		case !reflect.DeepEqual(current, candidate):
			currentCopy := current
			candidateCopy := candidate
			deltas = append(deltas, StaticActorDelta{Change: "changed", Current: &currentCopy, Candidate: &candidateCopy})
		}
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func staticActorMapByAuthoringKey(actors []StaticActor) map[string]StaticActor {
	byKey := make(map[string]StaticActor, len(actors))
	for _, actor := range normalizeStaticActors(actors) {
		byKey[staticActorAuthoringKey(actor)] = actor
	}
	return byKey
}

func buildInteractionDefinitionDeltas(current Bundle, candidate Bundle) []InteractionDefinitionDelta {
	if len(current.InteractionDefinitions) == 0 && len(candidate.InteractionDefinitions) == 0 {
		return nil
	}
	currentDefinitions := interactionDefinitionMapByKey(current.InteractionDefinitions)
	candidateDefinitions := interactionDefinitionMapByKey(candidate.InteractionDefinitions)
	currentItemTemplates := itemTemplateMapByVnum(current.ItemTemplates)
	candidateItemTemplates := itemTemplateMapByVnum(candidate.ItemTemplates)

	identitiesByKey := make(map[string]InteractionDefinitionReferenceSummary, len(currentDefinitions)+len(candidateDefinitions))
	for _, definition := range current.InteractionDefinitions {
		definition = interactionstore.NormalizeDefinition(definition)
		identitiesByKey[interactionDefinitionKey(definition.Kind, definition.Ref)] = InteractionDefinitionReferenceSummary{Kind: definition.Kind, Ref: definition.Ref}
	}
	for _, definition := range candidate.InteractionDefinitions {
		definition = interactionstore.NormalizeDefinition(definition)
		identitiesByKey[interactionDefinitionKey(definition.Kind, definition.Ref)] = InteractionDefinitionReferenceSummary{Kind: definition.Kind, Ref: definition.Ref}
	}
	identities := make([]InteractionDefinitionReferenceSummary, 0, len(identitiesByKey))
	for _, identity := range identitiesByKey {
		identities = append(identities, identity)
	}
	sort.Slice(identities, func(i int, j int) bool {
		if identities[i].Kind == identities[j].Kind {
			return identities[i].Ref < identities[j].Ref
		}
		return identities[i].Kind < identities[j].Kind
	})

	deltas := make([]InteractionDefinitionDelta, 0, len(identities))
	for _, identity := range identities {
		key := interactionDefinitionKey(identity.Kind, identity.Ref)
		currentDefinition, currentOK := currentDefinitions[key]
		candidateDefinition, candidateOK := candidateDefinitions[key]
		switch {
		case !currentOK:
			deltas = append(deltas, InteractionDefinitionDelta{
				Kind:             identity.Kind,
				Ref:              identity.Ref,
				Change:           "added",
				CandidatePreview: compactInteractionPreview(interactionDefinitionCatalogPreview(candidateDefinition, candidateItemTemplates)),
			})
		case !candidateOK:
			deltas = append(deltas, InteractionDefinitionDelta{
				Kind:           identity.Kind,
				Ref:            identity.Ref,
				Change:         "removed",
				CurrentPreview: compactInteractionPreview(interactionDefinitionCatalogPreview(currentDefinition, currentItemTemplates)),
			})
		case !reflect.DeepEqual(currentDefinition, candidateDefinition):
			deltas = append(deltas, InteractionDefinitionDelta{
				Kind:             identity.Kind,
				Ref:              identity.Ref,
				Change:           "changed",
				CurrentPreview:   compactInteractionPreview(interactionDefinitionCatalogPreview(currentDefinition, currentItemTemplates)),
				CandidatePreview: compactInteractionPreview(interactionDefinitionCatalogPreview(candidateDefinition, candidateItemTemplates)),
			})
		}
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func buildItemTemplateDeltas(currentTemplates []itemcatalog.Template, candidateTemplates []itemcatalog.Template) []ItemTemplateDelta {
	if len(currentTemplates) == 0 && len(candidateTemplates) == 0 {
		return nil
	}
	currentByVnum := itemTemplateMapByVnum(currentTemplates)
	candidateByVnum := itemTemplateMapByVnum(candidateTemplates)
	vnumsSeen := make(map[uint32]struct{}, len(currentByVnum)+len(candidateByVnum))
	for vnum := range currentByVnum {
		vnumsSeen[vnum] = struct{}{}
	}
	for vnum := range candidateByVnum {
		vnumsSeen[vnum] = struct{}{}
	}
	vnums := make([]uint32, 0, len(vnumsSeen))
	for vnum := range vnumsSeen {
		vnums = append(vnums, vnum)
	}
	sort.Slice(vnums, func(i int, j int) bool { return vnums[i] < vnums[j] })

	deltas := make([]ItemTemplateDelta, 0, len(vnums))
	for _, vnum := range vnums {
		current, currentOK := currentByVnum[vnum]
		candidate, candidateOK := candidateByVnum[vnum]
		switch {
		case !currentOK:
			candidateCopy := candidate
			deltas = append(deltas, ItemTemplateDelta{Vnum: vnum, Change: "added", Candidate: &candidateCopy})
		case !candidateOK:
			currentCopy := current
			deltas = append(deltas, ItemTemplateDelta{Vnum: vnum, Change: "removed", Current: &currentCopy})
		case !reflect.DeepEqual(current, candidate):
			currentCopy := current
			candidateCopy := candidate
			deltas = append(deltas, ItemTemplateDelta{Vnum: vnum, Change: "changed", Current: &currentCopy, Candidate: &candidateCopy})
		}
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func buildCombatProfileDeltas(currentProfiles []worldruntime.StaticActorCombatProfileSnapshot, candidateProfiles []worldruntime.StaticActorCombatProfileSnapshot) []CombatProfileDelta {
	if len(currentProfiles) == 0 && len(candidateProfiles) == 0 {
		return nil
	}
	currentByProfile := combatProfileSnapshotMapByProfile(currentProfiles)
	candidateByProfile := combatProfileSnapshotMapByProfile(candidateProfiles)
	profilesSeen := make(map[string]struct{}, len(currentByProfile)+len(candidateByProfile))
	for profile := range currentByProfile {
		profilesSeen[profile] = struct{}{}
	}
	for profile := range candidateByProfile {
		profilesSeen[profile] = struct{}{}
	}
	profiles := make([]string, 0, len(profilesSeen))
	for profile := range profilesSeen {
		profiles = append(profiles, profile)
	}
	sort.Strings(profiles)

	deltas := make([]CombatProfileDelta, 0, len(profiles))
	for _, profile := range profiles {
		current, currentOK := currentByProfile[profile]
		candidate, candidateOK := candidateByProfile[profile]
		switch {
		case !currentOK:
			candidateCopy := candidate
			deltas = append(deltas, CombatProfileDelta{Profile: profile, Change: "added", Candidate: &candidateCopy})
		case !candidateOK:
			currentCopy := current
			deltas = append(deltas, CombatProfileDelta{Profile: profile, Change: "removed", Current: &currentCopy})
		case !reflect.DeepEqual(current, candidate):
			currentCopy := current
			candidateCopy := candidate
			deltas = append(deltas, CombatProfileDelta{Profile: profile, Change: "changed", Current: &currentCopy, Candidate: &candidateCopy})
		}
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func combatProfileSnapshotMapByProfile(profiles []worldruntime.StaticActorCombatProfileSnapshot) map[string]worldruntime.StaticActorCombatProfileSnapshot {
	byProfile := make(map[string]worldruntime.StaticActorCombatProfileSnapshot, len(profiles))
	for _, profile := range cloneCombatProfileSnapshots(profiles) {
		if profile.Profile == "" {
			continue
		}
		byProfile[profile.Profile] = profile
	}
	return byProfile
}

func buildSpawnGroupDeltas(currentSpawnGroups []SpawnGroupReferenceSummary, candidateSpawnGroups []SpawnGroupReferenceSummary) []SpawnGroupDelta {
	if len(currentSpawnGroups) == 0 && len(candidateSpawnGroups) == 0 {
		return nil
	}
	currentByRef := spawnGroupSummaryMapByRef(currentSpawnGroups)
	candidateByRef := spawnGroupSummaryMapByRef(candidateSpawnGroups)
	refsSeen := make(map[string]struct{}, len(currentByRef)+len(candidateByRef))
	for ref := range currentByRef {
		refsSeen[ref] = struct{}{}
	}
	for ref := range candidateByRef {
		refsSeen[ref] = struct{}{}
	}
	refs := make([]string, 0, len(refsSeen))
	for ref := range refsSeen {
		refs = append(refs, ref)
	}
	sort.Strings(refs)

	deltas := make([]SpawnGroupDelta, 0, len(refs))
	for _, ref := range refs {
		current, currentOK := currentByRef[ref]
		candidate, candidateOK := candidateByRef[ref]
		switch {
		case !currentOK:
			candidateCopy := candidate
			deltas = append(deltas, SpawnGroupDelta{Ref: ref, Change: "added", Candidate: &candidateCopy})
		case !candidateOK:
			currentCopy := current
			deltas = append(deltas, SpawnGroupDelta{Ref: ref, Change: "removed", Current: &currentCopy})
		case !reflect.DeepEqual(current, candidate):
			currentCopy := current
			candidateCopy := candidate
			deltas = append(deltas, SpawnGroupDelta{Ref: ref, Change: "changed", Current: &currentCopy, Candidate: &candidateCopy})
		}
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func spawnGroupSummaryMapByRef(spawnGroups []SpawnGroupReferenceSummary) map[string]SpawnGroupReferenceSummary {
	byRef := make(map[string]SpawnGroupReferenceSummary, len(spawnGroups))
	for _, spawnGroup := range spawnGroups {
		spawnGroup.Ref = strings.TrimSpace(spawnGroup.Ref)
		spawnGroup.Name = strings.TrimSpace(spawnGroup.Name)
		spawnGroup.CombatProfile = strings.TrimSpace(spawnGroup.CombatProfile)
		spawnGroup.RewardDropVnums = cloneUint32s(spawnGroup.RewardDropVnums)
		spawnGroup.RewardDropItems = cloneRewardDropItemSummaries(spawnGroup.RewardDropItems)
		byRef[spawnGroup.Ref] = spawnGroup
	}
	return byRef
}

func buildMapContentDeltas(currentMaps []MapContentSummary, candidateMaps []MapContentSummary) []MapContentDelta {
	if len(currentMaps) == 0 && len(candidateMaps) == 0 {
		return nil
	}
	currentByIndex := make(map[uint32]MapContentSummary, len(currentMaps))
	candidateByIndex := make(map[uint32]MapContentSummary, len(candidateMaps))
	indexesSeen := make(map[uint32]struct{}, len(currentMaps)+len(candidateMaps))
	for _, summary := range currentMaps {
		currentByIndex[summary.MapIndex] = summary
		indexesSeen[summary.MapIndex] = struct{}{}
	}
	for _, summary := range candidateMaps {
		candidateByIndex[summary.MapIndex] = summary
		indexesSeen[summary.MapIndex] = struct{}{}
	}
	indexes := make([]uint32, 0, len(indexesSeen))
	for index := range indexesSeen {
		indexes = append(indexes, index)
	}
	sort.Slice(indexes, func(i int, j int) bool { return indexes[i] < indexes[j] })
	deltas := make([]MapContentDelta, 0, len(indexes))
	for _, index := range indexes {
		current := currentByIndex[index]
		candidate := candidateByIndex[index]
		delta := MapContentDelta{
			MapIndex:                     index,
			StaticActorCount:             summaryCountDelta(current.StaticActorCount, candidate.StaticActorCount),
			InteractableStaticActorCount: summaryCountDelta(current.InteractableStaticActorCount, candidate.InteractableStaticActorCount),
			InfoActorCount:               summaryCountDelta(current.InfoActorCount, candidate.InfoActorCount),
			TalkActorCount:               summaryCountDelta(current.TalkActorCount, candidate.TalkActorCount),
			ShopPreviewActorCount:        summaryCountDelta(current.ShopPreviewActorCount, candidate.ShopPreviewActorCount),
			ShopCatalogEntryCount:        summaryCountDelta(current.ShopCatalogEntryCount, candidate.ShopCatalogEntryCount),
			WarpActorCount:               summaryCountDelta(current.WarpActorCount, candidate.WarpActorCount),
			SpawnGroupCount:              summaryCountDelta(current.SpawnGroupCount, candidate.SpawnGroupCount),
			RewardExperienceTotal:        summaryAmountDelta(current.RewardExperienceTotal, candidate.RewardExperienceTotal),
			RewardGoldTotal:              summaryAmountDelta(current.RewardGoldTotal, candidate.RewardGoldTotal),
			RewardDropItemCount:          summaryCountDelta(current.RewardDropItemCount, candidate.RewardDropItemCount),
		}
		if !mapContentDeltaIsZero(delta) {
			deltas = append(deltas, delta)
		}
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func mapContentDeltaIsZero(delta MapContentDelta) bool {
	return delta.StaticActorCount.Delta == 0 &&
		delta.InteractableStaticActorCount.Delta == 0 &&
		delta.InfoActorCount.Delta == 0 &&
		delta.TalkActorCount.Delta == 0 &&
		delta.ShopPreviewActorCount.Delta == 0 &&
		delta.ShopCatalogEntryCount.Delta == 0 &&
		delta.WarpActorCount.Delta == 0 &&
		delta.SpawnGroupCount.Delta == 0 &&
		delta.RewardExperienceTotal.Delta == 0 &&
		delta.RewardGoldTotal.Delta == 0 &&
		delta.RewardDropItemCount.Delta == 0
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
			addMapServiceInteractionSummary(entry, definition)
			if definition.Kind == interactionstore.KindShopPreview {
				summary.ShopRoutes = append(summary.ShopRoutes, shopRouteSummary(actor, definition))
			}
			if definition.Kind == interactionstore.KindWarp {
				summary.WarpRoutes = append(summary.WarpRoutes, warpRouteSummary(actor, definition))
			}
			summary.InteractableStaticActors = append(summary.InteractableStaticActors, interactableStaticActorSummary(actor, definition, itemTemplatesByVnum))
		}
	}
	summary.ShopRouteCount = len(summary.ShopRoutes)
	summary.WarpRouteCount = len(summary.WarpRoutes)
	rewardDropCountsByVnum := make(map[uint32]int)
	for _, spawnGroup := range normalized.SpawnGroups {
		entry := mapContentSummaryForIndex(mapCounts, spawnGroup.MapIndex)
		entry.SpawnGroupCount++
		entry.RewardExperienceTotal += spawnGroup.RewardExperience
		entry.RewardGoldTotal += spawnGroup.RewardGold
		entry.RewardDropItemCount += len(spawnGroup.RewardDropVnums)
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

func shopRouteSummary(actor StaticActor, definition interactionstore.Definition) ShopRouteSummary {
	actor = normalizeStaticActors([]StaticActor{actor})[0]
	definition = interactionstore.NormalizeDefinition(definition)
	return ShopRouteSummary{
		ActorName:      actor.Name,
		SourceMapIndex: actor.MapIndex,
		SourceX:        actor.X,
		SourceY:        actor.Y,
		Ref:            definition.Ref,
		Title:          definition.Title,
		EntryCount:     len(definition.Catalog),
	}
}

func warpRouteSummary(actor StaticActor, definition interactionstore.Definition) WarpRouteSummary {
	actor = normalizeStaticActors([]StaticActor{actor})[0]
	definition = interactionstore.NormalizeDefinition(definition)
	return WarpRouteSummary{
		ActorName:      actor.Name,
		SourceMapIndex: actor.MapIndex,
		SourceX:        actor.X,
		SourceY:        actor.Y,
		Ref:            definition.Ref,
		Text:           definition.Text,
		TargetMapIndex: definition.MapIndex,
		TargetX:        definition.X,
		TargetY:        definition.Y,
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

func cloneRewardDropItemSummaries(items []RewardDropItemSummary) []RewardDropItemSummary {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]RewardDropItemSummary, len(items))
	copy(cloned, items)
	sort.Slice(cloned, func(i int, j int) bool {
		if cloned[i].ItemVnum == cloned[j].ItemVnum {
			return cloned[i].ItemName < cloned[j].ItemName
		}
		return cloned[i].ItemVnum < cloned[j].ItemVnum
	})
	return cloned
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

func addMapServiceInteractionSummary(entry *MapContentSummary, definition interactionstore.Definition) {
	definition = interactionstore.NormalizeDefinition(definition)
	switch definition.Kind {
	case interactionstore.KindInfo:
		entry.InfoActorCount++
	case interactionstore.KindTalk:
		entry.TalkActorCount++
	case interactionstore.KindShopPreview:
		entry.ShopPreviewActorCount++
		entry.ShopCatalogEntryCount += len(definition.Catalog)
	case interactionstore.KindWarp:
		entry.WarpActorCount++
	}
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
		if existing, ok := worldruntime.BootstrapStaticActorCombatProfileDefaults(name); ok && !combatProfileSnapshotMatchesDefaults(profile, existing) {
			return ErrInvalidBundle
		}
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

func combatProfileSnapshotMatchesDefaults(snapshot worldruntime.StaticActorCombatProfileSnapshot, defaults worldruntime.StaticActorCombatProfileDefaults) bool {
	candidateDefaults, ok := combatProfileSnapshotDefaults(snapshot)
	return ok &&
		candidateDefaults.MaxHP == defaults.MaxHP &&
		candidateDefaults.DamagePerNormalAttack == defaults.DamagePerNormalAttack &&
		candidateDefaults.AttackValue == defaults.AttackValue &&
		candidateDefaults.DefenseValue == defaults.DefenseValue &&
		candidateDefaults.Level == defaults.Level &&
		candidateDefaults.Rank == defaults.Rank &&
		candidateDefaults.RespawnDelay == defaults.RespawnDelay &&
		reflect.DeepEqual(candidateDefaults.DeathReward.Clone(), defaults.DeathReward.Clone())
}

func combatProfileSnapshotDefaults(snapshot worldruntime.StaticActorCombatProfileSnapshot) (worldruntime.StaticActorCombatProfileDefaults, bool) {
	if strings.TrimSpace(snapshot.Profile) == "" || snapshot.MaxHP == 0 || snapshot.AttackValue == 0 || snapshot.RespawnDelayMs <= 0 {
		return worldruntime.StaticActorCombatProfileDefaults{}, false
	}
	defaults := worldruntime.StaticActorCombatProfileDefaults{
		MaxHP:                 snapshot.MaxHP,
		DamagePerNormalAttack: snapshot.DamagePerNormalAttack,
		AttackValue:           snapshot.AttackValue,
		DefenseValue:          snapshot.DefenseValue,
		Level:                 snapshot.Level,
		Rank:                  snapshot.Rank,
		RespawnDelay:          time.Duration(snapshot.RespawnDelayMs) * time.Millisecond,
		DeathReward:           snapshot.DeathReward.Clone(),
	}
	if defaults.DamagePerNormalAttack == 0 {
		defaults.DamagePerNormalAttack = combatProfileSnapshotFormulaDamage(snapshot)
	}
	if defaults.Level == 0 {
		defaults.Level = worldruntime.TrainingDummyBootstrapLevel
	}
	return defaults, true
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
