package worldruntime

import "github.com/MikelCalvo/go-metin2-server/internal/loginticket"

const (
	bootstrapMapIndex     uint32 = 1
	defaultLocalChannelID uint8  = 1
)

type BootstrapTopology struct {
	localChannelID   uint8
	visibilityPolicy VisibilityPolicy
}

func NewBootstrapTopology(localChannelID uint8) BootstrapTopology {
	if localChannelID == 0 {
		localChannelID = defaultLocalChannelID
	}
	return BootstrapTopology{localChannelID: localChannelID, visibilityPolicy: WholeMapVisibilityPolicy{}}
}

func (t BootstrapTopology) LocalChannelID() uint8 {
	if t.localChannelID == 0 {
		return defaultLocalChannelID
	}
	return t.localChannelID
}

func (t BootstrapTopology) EffectiveChannelID(loginticket.Character) uint8 {
	return t.LocalChannelID()
}

func (t BootstrapTopology) EffectiveMapIndex(character loginticket.Character) uint32 {
	if character.MapIndex == 0 {
		return bootstrapMapIndex
	}
	return character.MapIndex
}

func (t BootstrapTopology) VisibilityPolicy() VisibilityPolicy {
	if t.visibilityPolicy == nil {
		return WholeMapVisibilityPolicy{}
	}
	return t.visibilityPolicy
}

func (t BootstrapTopology) WithVisibilityPolicy(policy VisibilityPolicy) BootstrapTopology {
	if policy == nil {
		policy = WholeMapVisibilityPolicy{}
	}
	t.visibilityPolicy = policy
	return t
}

func (t BootstrapTopology) WithWholeMapVisibilityPolicy() BootstrapTopology {
	return t.WithVisibilityPolicy(WholeMapVisibilityPolicy{})
}

func (t BootstrapTopology) WithRadiusVisibilityPolicy(radius int32, sectorSize int32) BootstrapTopology {
	return t.WithVisibilityPolicy(RadiusVisibilityPolicy{Radius: radius, SectorSize: sectorSize})
}

func (t BootstrapTopology) SharesVisibleWorld(left loginticket.Character, right loginticket.Character) bool {
	return t.VisibilityPolicy().CanSee(t, left, right)
}

func (t BootstrapTopology) SharesTalkingChatScope(left loginticket.Character, right loginticket.Character) bool {
	return left.Empire != 0 && left.Empire == right.Empire && t.SharesVisibleWorld(left, right)
}

func (t BootstrapTopology) SharesShoutScope(left loginticket.Character, right loginticket.Character) bool {
	return left.Empire != 0 && left.Empire == right.Empire && t.EffectiveChannelID(left) == t.EffectiveChannelID(right)
}

func (t BootstrapTopology) SharesGuildChatScope(left loginticket.Character, right loginticket.Character) bool {
	return left.GuildID != 0 && left.GuildID == right.GuildID && t.EffectiveChannelID(left) == t.EffectiveChannelID(right)
}
