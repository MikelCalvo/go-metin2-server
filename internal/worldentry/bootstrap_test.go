package worldentry

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
)

func TestBuildBootstrapFramesReturnsSelectedCharacterBurst(t *testing.T) {
	character := loginticket.Character{
		ID:         0x01030102,
		VID:        0x02040102,
		Name:       "PeerTwo",
		RaceNum:    2,
		MainPart:   102,
		HairPart:   202,
		Level:      20,
		X:          1700,
		Y:          2800,
		Z:          0,
		Empire:     2,
		GuildID:    10,
		SkillGroup: 1,
	}
	character.Points[1] = 750

	frames, err := BuildBootstrapFrames(character)
	if err != nil {
		t.Fatalf("unexpected bootstrap frame error: %v", err)
	}
	if len(frames) != 4 {
		t.Fatalf("expected 4 bootstrap frames, got %d", len(frames))
	}

	added, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode bootstrap add: %v", err)
	}
	if added.VID != character.VID || added.X != character.X || added.Y != character.Y || added.RaceNum != character.RaceNum {
		t.Fatalf("unexpected bootstrap add packet: %+v", added)
	}

	info, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode bootstrap additional info: %v", err)
	}
	if info.VID != character.VID || info.Name != character.Name || info.Parts[0] != character.MainPart || info.Parts[3] != character.HairPart {
		t.Fatalf("unexpected bootstrap additional info packet: %+v", info)
	}

	update, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, frames[2]))
	if err != nil {
		t.Fatalf("decode bootstrap update: %v", err)
	}
	if update.VID != character.VID || update.Parts[0] != character.MainPart || update.Parts[3] != character.HairPart {
		t.Fatalf("unexpected bootstrap update packet: %+v", update)
	}

	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, frames[3]))
	if err != nil {
		t.Fatalf("decode bootstrap point change: %v", err)
	}
	if pointChange.VID != character.VID || pointChange.Type != 1 || pointChange.Amount != 750 || pointChange.Value != 750 {
		t.Fatalf("unexpected bootstrap point-change packet: %+v", pointChange)
	}
}
