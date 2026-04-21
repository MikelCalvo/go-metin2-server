package worldruntime

import "github.com/MikelCalvo/go-metin2-server/internal/loginticket"

const (
	bootstrapMapIndex     uint32 = 1
	defaultLocalChannelID uint8  = 1
)

type BootstrapTopology struct {
	localChannelID uint8
}

func NewBootstrapTopology(localChannelID uint8) BootstrapTopology {
	if localChannelID == 0 {
		localChannelID = defaultLocalChannelID
	}
	return BootstrapTopology{localChannelID: localChannelID}
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

func (t BootstrapTopology) SharesVisibleWorld(left loginticket.Character, right loginticket.Character) bool {
	return t.EffectiveChannelID(left) == t.EffectiveChannelID(right) && t.EffectiveMapIndex(left) == t.EffectiveMapIndex(right)
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
