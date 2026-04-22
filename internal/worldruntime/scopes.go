package worldruntime

import "github.com/MikelCalvo/go-metin2-server/internal/loginticket"

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

func (s Scopes) GuildTargets(originID uint64, origin loginticket.Character) []PlayerEntity {
	return s.filterTargets(originID, origin, s.Topology.SharesGuildChatScope)
}

func (s Scopes) PlayerByExactName(name string) (PlayerEntity, bool) {
	if s.Entities == nil || name == "" {
		return PlayerEntity{}, false
	}
	return s.Entities.PlayerByName(name)
}

func (s Scopes) filterTargets(originID uint64, origin loginticket.Character, predicate func(loginticket.Character, loginticket.Character) bool) []PlayerEntity {
	if s.Entities == nil || originID == 0 {
		return nil
	}
	characters := s.Entities.PlayerCharacters()
	targets := make([]PlayerEntity, 0, len(characters))
	for _, peerCharacter := range characters {
		peerEntity, ok := s.Entities.PlayerByVID(peerCharacter.VID)
		if !ok || peerEntity.Entity.ID == originID || !predicate(origin, peerCharacter) {
			continue
		}
		targets = append(targets, peerEntity)
	}
	return targets
}
