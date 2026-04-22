package worldentry

import (
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
)

const bootstrapPlayerPointType uint8 = 1
const bootstrapPlayerPointValueIndex = 1

func BuildBootstrapFrames(character loginticket.Character) ([][]byte, error) {
	infoRaw, err := worldproto.EncodeCharacterAdditionalInfo(bootstrapCharacterAdditionalInfoPacket(character))
	if err != nil {
		return nil, err
	}
	return [][]byte{
		worldproto.EncodeCharacterAdd(bootstrapCharacterAddPacket(character)),
		infoRaw,
		worldproto.EncodeCharacterUpdate(bootstrapCharacterUpdatePacket(character)),
		worldproto.EncodePlayerPointChange(bootstrapPlayerPointChangePacket(character)),
	}, nil
}

func bootstrapCharacterAddPacket(character loginticket.Character) worldproto.CharacterAddPacket {
	return worldproto.CharacterAddPacket{
		VID:         character.VID,
		Angle:       90.5,
		X:           character.X,
		Y:           character.Y,
		Z:           character.Z,
		Type:        6,
		RaceNum:     character.RaceNum,
		MovingSpeed: 150,
		AttackSpeed: 100,
		StateFlag:   2,
		AffectFlags: [worldproto.AffectFlagCount]uint32{0x11111111, 0x22222222},
	}
}

func bootstrapCharacterAdditionalInfoPacket(character loginticket.Character) worldproto.CharacterAdditionalInfoPacket {
	return worldproto.CharacterAdditionalInfoPacket{
		VID:       character.VID,
		Name:      character.Name,
		Parts:     [worldproto.CharacterEquipmentPartCount]uint16{character.MainPart, 0, 0, character.HairPart},
		Empire:    character.Empire,
		GuildID:   character.GuildID,
		Level:     uint32(character.Level),
		Alignment: 0,
		PKMode:    0,
		MountVnum: 0,
	}
}

func bootstrapCharacterUpdatePacket(character loginticket.Character) worldproto.CharacterUpdatePacket {
	return worldproto.CharacterUpdatePacket{
		VID:         character.VID,
		Parts:       [worldproto.CharacterEquipmentPartCount]uint16{character.MainPart, 0, 0, character.HairPart},
		MovingSpeed: 150,
		AttackSpeed: 100,
		StateFlag:   2,
		AffectFlags: [worldproto.AffectFlagCount]uint32{0x11111111, 0x22222222},
		GuildID:     character.GuildID,
		Alignment:   0,
		PKMode:      0,
		MountVnum:   0,
	}
}

func bootstrapPlayerPointChangePacket(character loginticket.Character) worldproto.PlayerPointChangePacket {
	return worldproto.PlayerPointChangePacket{
		VID:    character.VID,
		Type:   bootstrapPlayerPointType,
		Amount: character.Points[bootstrapPlayerPointValueIndex],
		Value:  character.Points[bootstrapPlayerPointValueIndex],
	}
}
