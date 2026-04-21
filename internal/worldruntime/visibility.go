package worldruntime

import (
	"sort"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

type VisibilityPolicy interface {
	CanSee(topology BootstrapTopology, subject loginticket.Character, peer loginticket.Character) bool
}

type WholeMapVisibilityPolicy struct{}

func (WholeMapVisibilityPolicy) CanSee(topology BootstrapTopology, subject loginticket.Character, peer loginticket.Character) bool {
	return topology.EffectiveChannelID(subject) == topology.EffectiveChannelID(peer) && topology.EffectiveMapIndex(subject) == topology.EffectiveMapIndex(peer)
}

type VisibilityDiff struct {
	CurrentVisiblePeers []loginticket.Character
	TargetVisiblePeers  []loginticket.Character
	RemovedVisiblePeers []loginticket.Character
	AddedVisiblePeers   []loginticket.Character
}

func EnterVisibilityDiff(topology BootstrapTopology, subject loginticket.Character, characters []loginticket.Character) VisibilityDiff {
	return BuildVisibilityDiff(nil, VisiblePeers(topology, subject, characters, subject.VID))
}

func LeaveVisibilityDiff(topology BootstrapTopology, subject loginticket.Character, characters []loginticket.Character) VisibilityDiff {
	return BuildVisibilityDiff(VisiblePeers(topology, subject, characters, subject.VID), nil)
}

func RelocateVisibilityDiff(topology BootstrapTopology, current loginticket.Character, currentCharacters []loginticket.Character, target loginticket.Character, targetCharacters []loginticket.Character) VisibilityDiff {
	return BuildVisibilityDiff(
		VisiblePeers(topology, current, currentCharacters, current.VID),
		VisiblePeers(topology, target, targetCharacters, target.VID),
	)
}

func BuildVisibilityDiff(currentVisiblePeers []loginticket.Character, targetVisiblePeers []loginticket.Character) VisibilityDiff {
	currentVisiblePeers = cloneCharacters(currentVisiblePeers)
	targetVisiblePeers = cloneCharacters(targetVisiblePeers)
	removedVisiblePeers, addedVisiblePeers := DiffVisiblePeers(currentVisiblePeers, targetVisiblePeers)
	return VisibilityDiff{
		CurrentVisiblePeers: currentVisiblePeers,
		TargetVisiblePeers:  targetVisiblePeers,
		RemovedVisiblePeers: removedVisiblePeers,
		AddedVisiblePeers:   addedVisiblePeers,
	}
}

func VisiblePeers(topology BootstrapTopology, subject loginticket.Character, characters []loginticket.Character, excludeVID uint32) []loginticket.Character {
	visiblePeers := make([]loginticket.Character, 0, len(characters))
	for _, peer := range characters {
		if peer.VID == excludeVID || !topology.SharesVisibleWorld(subject, peer) {
			continue
		}
		visiblePeers = append(visiblePeers, peer)
	}
	sortCharacters(visiblePeers)
	return visiblePeers
}

func DiffVisiblePeers(current []loginticket.Character, target []loginticket.Character) ([]loginticket.Character, []loginticket.Character) {
	currentByVID := make(map[uint32]loginticket.Character, len(current))
	for _, character := range current {
		currentByVID[character.VID] = character
	}
	targetByVID := make(map[uint32]loginticket.Character, len(target))
	for _, character := range target {
		targetByVID[character.VID] = character
	}

	removed := make([]loginticket.Character, 0)
	for vid, character := range currentByVID {
		if _, ok := targetByVID[vid]; ok {
			continue
		}
		removed = append(removed, character)
	}
	added := make([]loginticket.Character, 0)
	for vid, character := range targetByVID {
		if _, ok := currentByVID[vid]; ok {
			continue
		}
		added = append(added, character)
	}
	sortCharacters(removed)
	sortCharacters(added)
	return removed, added
}

func cloneCharacters(characters []loginticket.Character) []loginticket.Character {
	if len(characters) == 0 {
		return nil
	}
	cloned := append([]loginticket.Character(nil), characters...)
	sortCharacters(cloned)
	return cloned
}

func sortCharacters(characters []loginticket.Character) {
	sort.Slice(characters, func(i int, j int) bool {
		if characters[i].Name == characters[j].Name {
			return characters[i].VID < characters[j].VID
		}
		return characters[i].Name < characters[j].Name
	})
}
