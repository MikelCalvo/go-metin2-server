package worldruntime

import (
	"sort"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

type VisibilitySnapshot struct {
	Subject      PlayerEntity
	VisiblePeers []PlayerEntity
}

type StaticActorVisibilityDiff struct {
	CurrentVisibleActors []StaticEntity
	TargetVisibleActors  []StaticEntity
	RemovedVisibleActors []StaticEntity
	AddedVisibleActors   []StaticEntity
}

type StaticActorTargetDiff struct {
	CurrentVisibleTargets  []PlayerEntity
	TargetVisibleTargets   []PlayerEntity
	RetainedVisibleTargets []PlayerEntity
	RemovedVisibleTargets  []PlayerEntity
	AddedVisibleTargets    []PlayerEntity
}

type ConnectedCharacterSnapshot struct {
	Name     string `json:"name"`
	VID      uint32 `json:"vid"`
	MapIndex uint32 `json:"map_index"`
	X        int32  `json:"x"`
	Y        int32  `json:"y"`
	Empire   uint8  `json:"empire"`
	GuildID  uint32 `json:"guild_id"`
}

type CharacterVisibilitySnapshot struct {
	ConnectedCharacterSnapshot
	VisiblePeers []ConnectedCharacterSnapshot `json:"visible_peers"`
}

type StaticActorSnapshot struct {
	EntityID uint64 `json:"entity_id"`
	Name     string `json:"name"`
	MapIndex uint32 `json:"map_index"`
	X        int32  `json:"x"`
	Y        int32  `json:"y"`
	RaceNum  uint32 `json:"race_num"`
}

type MapOccupancySnapshot struct {
	MapIndex         uint32                       `json:"map_index"`
	CharacterCount   int                          `json:"character_count"`
	Characters       []ConnectedCharacterSnapshot `json:"characters"`
	StaticActorCount int                          `json:"static_actor_count"`
	StaticActors     []StaticActorSnapshot        `json:"static_actors"`
}

type MapOccupancyChange struct {
	MapIndex    uint32 `json:"map_index"`
	BeforeCount int    `json:"before_count"`
	AfterCount  int    `json:"after_count"`
}

type RelocationPreview struct {
	Applied                    bool                         `json:"applied"`
	Character                  ConnectedCharacterSnapshot   `json:"character"`
	Target                     ConnectedCharacterSnapshot   `json:"target"`
	CurrentVisiblePeers        []ConnectedCharacterSnapshot `json:"current_visible_peers"`
	TargetVisiblePeers         []ConnectedCharacterSnapshot `json:"target_visible_peers"`
	RemovedVisiblePeers        []ConnectedCharacterSnapshot `json:"removed_visible_peers"`
	AddedVisiblePeers          []ConnectedCharacterSnapshot `json:"added_visible_peers"`
	CurrentVisibleStaticActors []StaticActorSnapshot        `json:"current_visible_static_actors"`
	TargetVisibleStaticActors  []StaticActorSnapshot        `json:"target_visible_static_actors"`
	RemovedVisibleStaticActors []StaticActorSnapshot        `json:"removed_visible_static_actors"`
	AddedVisibleStaticActors   []StaticActorSnapshot        `json:"added_visible_static_actors"`
	MapOccupancyChanges        []MapOccupancyChange         `json:"map_occupancy_changes"`
	BeforeMapOccupancy         []MapOccupancySnapshot       `json:"before_map_occupancy"`
	AfterMapOccupancy          []MapOccupancySnapshot       `json:"after_map_occupancy"`
}

type Scopes struct {
	Topology BootstrapTopology
	Entities *EntityRegistry
}

func NewScopes(topology BootstrapTopology, entities *EntityRegistry) Scopes {
	return Scopes{Topology: topology, Entities: entities}
}

func (s Scopes) VisibleTargets(originID uint64, origin loginticket.Character) []PlayerEntity {
	return s.filterTargets(originID, origin, s.Topology.SharesVisibleWorld)
}

func (s Scopes) LocalTalkTargets(originID uint64, origin loginticket.Character) []PlayerEntity {
	return s.filterTargets(originID, origin, s.Topology.SharesTalkingChatScope)
}

func (s Scopes) ShoutTargets(originID uint64, origin loginticket.Character) []PlayerEntity {
	return s.filterTargets(originID, origin, s.Topology.SharesShoutScope)
}

func (s Scopes) PartyTargets(originID uint64) []PlayerEntity {
	return s.filterTargets(originID, loginticket.Character{}, func(loginticket.Character, loginticket.Character) bool {
		return true
	})
}

func (s Scopes) GuildTargets(originID uint64, origin loginticket.Character) []PlayerEntity {
	return s.filterTargets(originID, origin, s.Topology.SharesGuildChatScope)
}

func (s Scopes) PlayerByExactName(name string) (PlayerEntity, bool) {
	if s.Entities == nil || name == "" {
		return PlayerEntity{}, false
	}
	return s.Entities.PlayerByName(name)
}

func (s Scopes) ConnectedTargets() []PlayerEntity {
	return s.filterTargets(0, loginticket.Character{}, func(loginticket.Character, loginticket.Character) bool {
		return true
	})
}

func (s Scopes) EnterVisibilityDiff(subject loginticket.Character) VisibilityDiff {
	return BuildVisibilityDiff(nil, s.visiblePeers(subject))
}

func (s Scopes) LeaveVisibilityDiff(subject loginticket.Character) VisibilityDiff {
	return BuildVisibilityDiff(s.visiblePeers(subject), nil)
}

func (s Scopes) RelocateVisibilityDiff(current, target loginticket.Character) VisibilityDiff {
	currentCharacters, targetCharacters := s.relocatedCharacters(current, target)
	return BuildVisibilityDiff(
		VisiblePeers(s.Topology, current, currentCharacters, current.VID),
		VisiblePeers(s.Topology, target, targetCharacters, target.VID),
	)
}

func (s Scopes) ConnectedCharacterSnapshots() []ConnectedCharacterSnapshot {
	targets := s.ConnectedTargets()
	snapshots := make([]ConnectedCharacterSnapshot, 0, len(targets))
	for _, target := range targets {
		snapshots = append(snapshots, connectedCharacterSnapshot(s.Topology, target.Character))
	}
	sortConnectedCharacterSnapshots(snapshots)
	return snapshots
}

func (s Scopes) VisibilitySnapshots() []VisibilitySnapshot {
	if s.Entities == nil {
		return nil
	}
	targets := s.ConnectedTargets()
	snapshots := make([]VisibilitySnapshot, 0, len(targets))
	for _, subject := range targets {
		snapshots = append(snapshots, VisibilitySnapshot{
			Subject:      subject,
			VisiblePeers: s.VisibleTargets(subject.Entity.ID, subject.Character),
		})
	}
	sortVisibilitySnapshots(snapshots)
	return snapshots
}

func (s Scopes) CharacterVisibilitySnapshots() []CharacterVisibilitySnapshot {
	visibility := s.VisibilitySnapshots()
	snapshots := make([]CharacterVisibilitySnapshot, 0, len(visibility))
	for _, entry := range visibility {
		snapshots = append(snapshots, CharacterVisibilitySnapshot{
			ConnectedCharacterSnapshot: connectedCharacterSnapshot(s.Topology, entry.Subject.Character),
			VisiblePeers:               connectedCharacterSnapshots(s.Topology, playerEntitiesToCharacters(entry.VisiblePeers)),
		})
	}
	sortCharacterVisibilitySnapshots(snapshots)
	return snapshots
}

func (s Scopes) StaticActorSnapshots() []StaticActorSnapshot {
	if s.Entities == nil {
		return nil
	}
	return staticActorSnapshots(s.Topology, s.Entities.AllStaticActors())
}

func (s Scopes) VisibleStaticActors(subject loginticket.Character) []StaticEntity {
	if s.Entities == nil {
		return nil
	}
	actors := s.Entities.AllStaticActors()
	visible := make([]StaticEntity, 0, len(actors))
	for _, actor := range actors {
		if !s.Topology.SharesVisibleWorld(subject, staticActorVisibilityCharacter(actor)) {
			continue
		}
		visible = append(visible, actor)
	}
	sortStaticEntities(visible)
	return visible
}

func (s Scopes) VisibleTargetsForStaticActor(actor StaticEntity) []PlayerEntity {
	return s.filterTargets(0, staticActorVisibilityCharacter(actor), s.Topology.SharesVisibleWorld)
}

func (s Scopes) RelocateStaticActorTargetDiff(current, target StaticEntity) StaticActorTargetDiff {
	return buildStaticActorTargetDiff(s.VisibleTargetsForStaticActor(current), s.VisibleTargetsForStaticActor(target))
}

func (s Scopes) RelocateStaticActorVisibilityDiff(current, target loginticket.Character) StaticActorVisibilityDiff {
	return buildStaticActorVisibilityDiff(s.VisibleStaticActors(current), s.VisibleStaticActors(target))
}

func (s Scopes) MapOccupancySnapshots() []MapOccupancySnapshot {
	if s.Entities == nil {
		return nil
	}
	return buildMapOccupancySnapshots(s.Topology, s.Entities.MapOccupancy())
}

func (s Scopes) BuildRelocationPreview(current, target loginticket.Character, applied bool) RelocationPreview {
	if s.Entities == nil {
		return RelocationPreview{Applied: applied, Character: connectedCharacterSnapshot(s.Topology, current), Target: connectedCharacterSnapshot(s.Topology, target)}
	}
	visibilityDiff := s.RelocateVisibilityDiff(current, target)
	staticActorVisibilityDiff := s.RelocateStaticActorVisibilityDiff(current, target)
	beforeOccupancy := s.MapOccupancySnapshots()
	afterOccupancy := relocateMapOccupancySnapshots(beforeOccupancy, s.Topology, current, target)
	return RelocationPreview{
		Applied:                    applied,
		Character:                  connectedCharacterSnapshot(s.Topology, current),
		Target:                     connectedCharacterSnapshot(s.Topology, target),
		CurrentVisiblePeers:        connectedCharacterSnapshots(s.Topology, visibilityDiff.CurrentVisiblePeers),
		TargetVisiblePeers:         connectedCharacterSnapshots(s.Topology, visibilityDiff.TargetVisiblePeers),
		RemovedVisiblePeers:        connectedCharacterSnapshots(s.Topology, visibilityDiff.RemovedVisiblePeers),
		AddedVisiblePeers:          connectedCharacterSnapshots(s.Topology, visibilityDiff.AddedVisiblePeers),
		CurrentVisibleStaticActors: staticActorSnapshots(s.Topology, staticActorVisibilityDiff.CurrentVisibleActors),
		TargetVisibleStaticActors:  staticActorSnapshots(s.Topology, staticActorVisibilityDiff.TargetVisibleActors),
		RemovedVisibleStaticActors: staticActorSnapshots(s.Topology, staticActorVisibilityDiff.RemovedVisibleActors),
		AddedVisibleStaticActors:   staticActorSnapshots(s.Topology, staticActorVisibilityDiff.AddedVisibleActors),
		MapOccupancyChanges:        buildMapOccupancyChanges(beforeOccupancy, afterOccupancy),
		BeforeMapOccupancy:         beforeOccupancy,
		AfterMapOccupancy:          afterOccupancy,
	}
}

func (s Scopes) visiblePeers(subject loginticket.Character) []loginticket.Character {
	if s.Entities == nil {
		return nil
	}
	return VisiblePeers(s.Topology, subject, s.Entities.PlayerCharacters(), subject.VID)
}

func (s Scopes) relocatedCharacters(current, target loginticket.Character) ([]loginticket.Character, []loginticket.Character) {
	if s.Entities == nil {
		return nil, nil
	}
	currentCharacters := s.Entities.PlayerCharacters()
	targetCharacters := append([]loginticket.Character(nil), currentCharacters...)
	for i := range targetCharacters {
		if targetCharacters[i].VID != current.VID {
			continue
		}
		targetCharacters[i] = target
		break
	}
	return currentCharacters, targetCharacters
}

func (s Scopes) filterTargets(originID uint64, origin loginticket.Character, predicate func(loginticket.Character, loginticket.Character) bool) []PlayerEntity {
	if s.Entities == nil {
		return nil
	}
	characters := s.Entities.PlayerCharacters()
	targets := make([]PlayerEntity, 0, len(characters))
	for _, peerCharacter := range characters {
		peerEntity, ok := s.Entities.PlayerByVID(peerCharacter.VID)
		if !ok || (originID != 0 && peerEntity.Entity.ID == originID) || !predicate(origin, peerCharacter) {
			continue
		}
		targets = append(targets, peerEntity)
	}
	return targets
}

func connectedCharacterSnapshot(topology BootstrapTopology, character loginticket.Character) ConnectedCharacterSnapshot {
	return ConnectedCharacterSnapshot{
		Name:     character.Name,
		VID:      character.VID,
		MapIndex: topology.EffectiveMapIndex(character),
		X:        character.X,
		Y:        character.Y,
		Empire:   character.Empire,
		GuildID:  character.GuildID,
	}
}

func staticActorSnapshot(topology BootstrapTopology, actor StaticEntity) StaticActorSnapshot {
	return StaticActorSnapshot{
		EntityID: actor.Entity.ID,
		Name:     actor.Entity.Name,
		MapIndex: topology.EffectiveMapIndex(loginticket.Character{MapIndex: actor.Position.MapIndex}),
		X:        actor.Position.X,
		Y:        actor.Position.Y,
		RaceNum:  actor.RaceNum,
	}
}

func connectedCharacterSnapshots(topology BootstrapTopology, characters []loginticket.Character) []ConnectedCharacterSnapshot {
	snapshots := make([]ConnectedCharacterSnapshot, 0, len(characters))
	for _, character := range characters {
		snapshots = append(snapshots, connectedCharacterSnapshot(topology, character))
	}
	sortConnectedCharacterSnapshots(snapshots)
	return snapshots
}

func staticActorSnapshots(topology BootstrapTopology, actors []StaticEntity) []StaticActorSnapshot {
	snapshots := make([]StaticActorSnapshot, 0, len(actors))
	for _, actor := range actors {
		snapshots = append(snapshots, staticActorSnapshot(topology, actor))
	}
	sortStaticActorSnapshots(snapshots)
	return snapshots
}

func staticActorVisibilityCharacter(actor StaticEntity) loginticket.Character {
	return loginticket.Character{MapIndex: actor.Position.MapIndex, X: actor.Position.X, Y: actor.Position.Y}
}

func buildStaticActorVisibilityDiff(currentVisibleActors []StaticEntity, targetVisibleActors []StaticEntity) StaticActorVisibilityDiff {
	currentVisibleActors = cloneStaticActors(currentVisibleActors)
	targetVisibleActors = cloneStaticActors(targetVisibleActors)
	removedVisibleActors, addedVisibleActors := diffVisibleStaticActors(currentVisibleActors, targetVisibleActors)
	return StaticActorVisibilityDiff{
		CurrentVisibleActors: currentVisibleActors,
		TargetVisibleActors:  targetVisibleActors,
		RemovedVisibleActors: removedVisibleActors,
		AddedVisibleActors:   addedVisibleActors,
	}
}

func buildStaticActorTargetDiff(currentVisibleTargets []PlayerEntity, targetVisibleTargets []PlayerEntity) StaticActorTargetDiff {
	currentVisibleTargets = clonePlayerEntities(currentVisibleTargets)
	targetVisibleTargets = clonePlayerEntities(targetVisibleTargets)
	currentByID := make(map[uint64]PlayerEntity, len(currentVisibleTargets))
	for _, target := range currentVisibleTargets {
		currentByID[target.Entity.ID] = target
	}
	targetByID := make(map[uint64]PlayerEntity, len(targetVisibleTargets))
	for _, target := range targetVisibleTargets {
		targetByID[target.Entity.ID] = target
	}

	retainedVisibleTargets := make([]PlayerEntity, 0)
	removedVisibleTargets := make([]PlayerEntity, 0)
	for _, target := range currentVisibleTargets {
		if _, ok := targetByID[target.Entity.ID]; ok {
			retainedVisibleTargets = append(retainedVisibleTargets, target)
			continue
		}
		removedVisibleTargets = append(removedVisibleTargets, target)
	}
	addedVisibleTargets := make([]PlayerEntity, 0)
	for _, target := range targetVisibleTargets {
		if _, ok := currentByID[target.Entity.ID]; ok {
			continue
		}
		addedVisibleTargets = append(addedVisibleTargets, target)
	}
	sortPlayerEntities(retainedVisibleTargets)
	sortPlayerEntities(removedVisibleTargets)
	sortPlayerEntities(addedVisibleTargets)
	return StaticActorTargetDiff{
		CurrentVisibleTargets:  currentVisibleTargets,
		TargetVisibleTargets:   targetVisibleTargets,
		RetainedVisibleTargets: retainedVisibleTargets,
		RemovedVisibleTargets:  removedVisibleTargets,
		AddedVisibleTargets:    addedVisibleTargets,
	}
}

func diffVisibleStaticActors(current []StaticEntity, target []StaticEntity) ([]StaticEntity, []StaticEntity) {
	currentByID := make(map[uint64]StaticEntity, len(current))
	for _, actor := range current {
		currentByID[actor.Entity.ID] = actor
	}
	targetByID := make(map[uint64]StaticEntity, len(target))
	for _, actor := range target {
		targetByID[actor.Entity.ID] = actor
	}

	removed := make([]StaticEntity, 0)
	for entityID, actor := range currentByID {
		if _, ok := targetByID[entityID]; ok {
			continue
		}
		removed = append(removed, actor)
	}
	added := make([]StaticEntity, 0)
	for entityID, actor := range targetByID {
		if _, ok := currentByID[entityID]; ok {
			continue
		}
		added = append(added, actor)
	}
	sortStaticEntities(removed)
	sortStaticEntities(added)
	return removed, added
}

func cloneStaticActors(actors []StaticEntity) []StaticEntity {
	if len(actors) == 0 {
		return nil
	}
	cloned := append([]StaticEntity(nil), actors...)
	sortStaticEntities(cloned)
	return cloned
}

func clonePlayerEntities(players []PlayerEntity) []PlayerEntity {
	if len(players) == 0 {
		return nil
	}
	cloned := append([]PlayerEntity(nil), players...)
	sortPlayerEntities(cloned)
	return cloned
}

func sortPlayerEntities(players []PlayerEntity) {
	sort.Slice(players, func(i int, j int) bool {
		if players[i].Entity.Name == players[j].Entity.Name {
			return players[i].Entity.ID < players[j].Entity.ID
		}
		return players[i].Entity.Name < players[j].Entity.Name
	})
}

func buildMapOccupancySnapshots(topology BootstrapTopology, occupancies []MapOccupancy) []MapOccupancySnapshot {
	snapshots := make([]MapOccupancySnapshot, 0, len(occupancies))
	for _, occupancy := range occupancies {
		staticActors := staticActorSnapshots(topology, occupancy.StaticActors)
		snapshots = append(snapshots, MapOccupancySnapshot{
			MapIndex:         occupancy.MapIndex,
			CharacterCount:   len(occupancy.Characters),
			Characters:       connectedCharacterSnapshots(topology, occupancy.Characters),
			StaticActorCount: len(occupancy.StaticActors),
			StaticActors:     staticActors,
		})
	}
	sortMapOccupancySnapshots(snapshots)
	return snapshots
}

func relocateMapOccupancySnapshots(before []MapOccupancySnapshot, topology BootstrapTopology, current loginticket.Character, target loginticket.Character) []MapOccupancySnapshot {
	byMap := make(map[uint32]MapOccupancySnapshot, len(before)+1)
	for _, snapshot := range before {
		byMap[snapshot.MapIndex] = MapOccupancySnapshot{
			MapIndex:         snapshot.MapIndex,
			CharacterCount:   snapshot.CharacterCount,
			Characters:       append([]ConnectedCharacterSnapshot(nil), snapshot.Characters...),
			StaticActorCount: snapshot.StaticActorCount,
			StaticActors:     append([]StaticActorSnapshot(nil), snapshot.StaticActors...),
		}
	}

	currentSnapshot := connectedCharacterSnapshot(topology, current)
	targetSnapshot := connectedCharacterSnapshot(topology, target)

	if snapshot, ok := byMap[currentSnapshot.MapIndex]; ok {
		filtered := make([]ConnectedCharacterSnapshot, 0, len(snapshot.Characters))
		for _, character := range snapshot.Characters {
			if character.VID == currentSnapshot.VID {
				continue
			}
			filtered = append(filtered, character)
		}
		snapshot.Characters = filtered
		snapshot.CharacterCount = len(filtered)
		if snapshot.CharacterCount == 0 && snapshot.StaticActorCount == 0 {
			delete(byMap, currentSnapshot.MapIndex)
		} else {
			byMap[currentSnapshot.MapIndex] = snapshot
		}
	}

	targetOccupancy := byMap[targetSnapshot.MapIndex]
	targetCharacters := append([]ConnectedCharacterSnapshot(nil), targetOccupancy.Characters...)
	replaced := false
	for i := range targetCharacters {
		if targetCharacters[i].VID != targetSnapshot.VID {
			continue
		}
		targetCharacters[i] = targetSnapshot
		replaced = true
		break
	}
	if !replaced {
		targetCharacters = append(targetCharacters, targetSnapshot)
	}
	sortConnectedCharacterSnapshots(targetCharacters)
	targetOccupancy.MapIndex = targetSnapshot.MapIndex
	targetOccupancy.Characters = targetCharacters
	targetOccupancy.CharacterCount = len(targetCharacters)
	byMap[targetSnapshot.MapIndex] = targetOccupancy

	snapshots := make([]MapOccupancySnapshot, 0, len(byMap))
	for mapIndex, snapshot := range byMap {
		sortConnectedCharacterSnapshots(snapshot.Characters)
		sortStaticActorSnapshots(snapshot.StaticActors)
		snapshot.MapIndex = mapIndex
		snapshot.CharacterCount = len(snapshot.Characters)
		snapshot.StaticActorCount = len(snapshot.StaticActors)
		snapshots = append(snapshots, snapshot)
	}
	sortMapOccupancySnapshots(snapshots)
	return snapshots
}

func buildMapOccupancyChanges(before []MapOccupancySnapshot, after []MapOccupancySnapshot) []MapOccupancyChange {
	beforeCounts := make(map[uint32]int, len(before))
	for _, snapshot := range before {
		beforeCounts[snapshot.MapIndex] = snapshot.CharacterCount
	}
	afterCounts := make(map[uint32]int, len(after))
	for _, snapshot := range after {
		afterCounts[snapshot.MapIndex] = snapshot.CharacterCount
	}

	indices := make(map[uint32]struct{}, len(beforeCounts)+len(afterCounts))
	for mapIndex := range beforeCounts {
		indices[mapIndex] = struct{}{}
	}
	for mapIndex := range afterCounts {
		indices[mapIndex] = struct{}{}
	}

	changes := make([]MapOccupancyChange, 0, len(indices))
	for mapIndex := range indices {
		beforeCount := beforeCounts[mapIndex]
		afterCount := afterCounts[mapIndex]
		if beforeCount == afterCount {
			continue
		}
		changes = append(changes, MapOccupancyChange{MapIndex: mapIndex, BeforeCount: beforeCount, AfterCount: afterCount})
	}
	sortMapOccupancyChanges(changes)
	return changes
}

func playerEntitiesToCharacters(players []PlayerEntity) []loginticket.Character {
	characters := make([]loginticket.Character, 0, len(players))
	for _, player := range players {
		characters = append(characters, player.Character)
	}
	return characters
}

func sortVisibilitySnapshots(snapshots []VisibilitySnapshot) {
	sort.Slice(snapshots, func(i int, j int) bool {
		if snapshots[i].Subject.Entity.Name == snapshots[j].Subject.Entity.Name {
			return snapshots[i].Subject.Entity.VID < snapshots[j].Subject.Entity.VID
		}
		return snapshots[i].Subject.Entity.Name < snapshots[j].Subject.Entity.Name
	})
}

func sortConnectedCharacterSnapshots(snapshots []ConnectedCharacterSnapshot) {
	sort.Slice(snapshots, func(i int, j int) bool {
		if snapshots[i].Name == snapshots[j].Name {
			return snapshots[i].VID < snapshots[j].VID
		}
		return snapshots[i].Name < snapshots[j].Name
	})
}

func sortCharacterVisibilitySnapshots(snapshots []CharacterVisibilitySnapshot) {
	sort.Slice(snapshots, func(i int, j int) bool {
		if snapshots[i].Name == snapshots[j].Name {
			return snapshots[i].VID < snapshots[j].VID
		}
		return snapshots[i].Name < snapshots[j].Name
	})
}

func sortStaticActorSnapshots(snapshots []StaticActorSnapshot) {
	sort.Slice(snapshots, func(i int, j int) bool {
		if snapshots[i].Name == snapshots[j].Name {
			return snapshots[i].EntityID < snapshots[j].EntityID
		}
		return snapshots[i].Name < snapshots[j].Name
	})
}

func sortMapOccupancySnapshots(snapshots []MapOccupancySnapshot) {
	sort.Slice(snapshots, func(i int, j int) bool {
		return snapshots[i].MapIndex < snapshots[j].MapIndex
	})
}

func sortMapOccupancyChanges(changes []MapOccupancyChange) {
	sort.Slice(changes, func(i int, j int) bool {
		return changes[i].MapIndex < changes[j].MapIndex
	})
}
