package worldentry

import (
	"sort"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
)

const bootstrapPlayerPointType uint8 = 1
const bootstrapPlayerPointValueIndex = 1

func BuildBootstrapFrames(character loginticket.Character) ([][]byte, error) {
	infoRaw, err := worldproto.EncodeCharacterAdditionalInfo(bootstrapCharacterAdditionalInfoPacket(character))
	if err != nil {
		return nil, err
	}
	frames := [][]byte{
		worldproto.EncodeCharacterAdd(bootstrapCharacterAddPacket(character)),
		infoRaw,
		worldproto.EncodeCharacterUpdate(bootstrapCharacterUpdatePacket(character)),
		worldproto.EncodePlayerPointChange(bootstrapPlayerPointChangePacket(character)),
	}
	for _, quickslot := range sortedBootstrapQuickslots(character.Quickslots) {
		frames = append(frames, quickslotproto.EncodeAdd(quickslotproto.AddPacket{
			Position: quickslot.Position,
			Slot: quickslotproto.Slot{
				Type:     quickslot.Type,
				Position: quickslot.Slot,
			},
		}))
	}
	return frames, nil
}

func sortedBootstrapQuickslots(quickslots []loginticket.Quickslot) []loginticket.Quickslot {
	if len(quickslots) == 0 {
		return nil
	}
	sorted := append(quickslots[:0:0], quickslots...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Position < sorted[j].Position
	})
	return sorted
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
		Parts:     bootstrapCharacterAppearanceParts(character),
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
		Parts:       bootstrapCharacterAppearanceParts(character),
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

func bootstrapCharacterAppearanceParts(character loginticket.Character) [worldproto.CharacterEquipmentPartCount]uint16 {
	parts := [worldproto.CharacterEquipmentPartCount]uint16{character.MainPart, 0, 0, character.HairPart}
	for _, instance := range character.Equipment {
		if !instance.Equipped {
			continue
		}
		switch instance.EquipSlot {
		case inventory.EquipmentSlotBody:
			parts[0] = uint16(instance.Vnum)
		case inventory.EquipmentSlotWeapon:
			parts[1] = uint16(instance.Vnum)
		case inventory.EquipmentSlotHead:
			parts[2] = uint16(instance.Vnum)
		}
	}
	return parts
}

func bootstrapPlayerPointChangePacket(character loginticket.Character) worldproto.PlayerPointChangePacket {
	return worldproto.PlayerPointChangePacket{
		VID:    character.VID,
		Type:   bootstrapPlayerPointType,
		Amount: character.Points[bootstrapPlayerPointValueIndex],
		Value:  character.Points[bootstrapPlayerPointValueIndex],
	}
}
