package worldruntime

import (
	"sort"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

type VisibilitySnapshot struct {
	Subject      PlayerEntity
	VisiblePeers []PlayerEntity
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

func sortVisibilitySnapshots(snapshots []VisibilitySnapshot) {
	sort.Slice(snapshots, func(i int, j int) bool {
		if snapshots[i].Subject.Entity.Name == snapshots[j].Subject.Entity.Name {
			return snapshots[i].Subject.Entity.VID < snapshots[j].Subject.Entity.VID
		}
		return snapshots[i].Subject.Entity.Name < snapshots[j].Subject.Entity.Name
	})
}
