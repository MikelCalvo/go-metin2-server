package worldruntime

import (
	"sort"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

type VisibilitySnapshot struct {
	Subject      PlayerEntity
	VisiblePeers []PlayerEntity
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

func (s Scopes) MapOccupancySnapshots() []MapOccupancySnapshot {
	if s.Entities == nil {
		return nil
	}
	return buildMapOccupancySnapshots(s.Topology, s.Entities.MapOccupancy())
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
