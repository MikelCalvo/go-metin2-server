package worldentry

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
)

func TestBuildBootstrapFramesProjectsTemplateAppearanceVnum(t *testing.T) {
	character := loginticket.Character{
		ID:       0x01030102,
		VID:      0x02040102,
		Name:     "VisibleSword",
		RaceNum:  2,
		MainPart: 101,
		HairPart: 201,
		Level:    20,
		X:        1700,
		Y:        2800,
		Equipment: []inventory.ItemInstance{
			{ID: 72, Vnum: 11200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
		},
	}
	templates := map[uint32]itemcatalog.Template{
		11200: {Vnum: 11200, Name: "Visible Practice Sword", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotWeapon.String(), AppearanceVnum: 321},
	}

	frames, err := BuildBootstrapFramesWithTemplates(character, templates)
	if err != nil {
		t.Fatalf("unexpected template appearance bootstrap frame error: %v", err)
	}
	if len(frames) != 4 {
		t.Fatalf("expected 4 bootstrap frames, got %d", len(frames))
	}

	info, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode template appearance additional info: %v", err)
	}
	wantParts := [worldproto.CharacterEquipmentPartCount]uint16{101, 321, 0, 201}
	if info.Parts != wantParts {
		t.Fatalf("expected template appearance additional-info parts %+v, got %+v", wantParts, info.Parts)
	}

	update, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, frames[2]))
	if err != nil {
		t.Fatalf("decode template appearance update: %v", err)
	}
	if update.Parts != wantParts {
		t.Fatalf("expected template appearance update parts %+v, got %+v", wantParts, update.Parts)
	}
}

func TestBuildBootstrapFramesIgnoresTemplateAppearanceVnumForMismatchedSlot(t *testing.T) {
	character := loginticket.Character{
		ID:       0x01030102,
		VID:      0x02040102,
		Name:     "VisibleSword",
		RaceNum:  2,
		MainPart: 101,
		HairPart: 201,
		Level:    20,
		X:        1700,
		Y:        2800,
		Equipment: []inventory.ItemInstance{
			{ID: 72, Vnum: 11200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
		},
	}
	templates := map[uint32]itemcatalog.Template{
		11200: {Vnum: 11200, Name: "Mismatched Appearance Armor", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotBody.String(), AppearanceVnum: 321},
	}

	frames, err := BuildBootstrapFramesWithTemplates(character, templates)
	if err != nil {
		t.Fatalf("unexpected mismatched-slot template appearance frame error: %v", err)
	}
	update, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, frames[2]))
	if err != nil {
		t.Fatalf("decode mismatched-slot appearance update: %v", err)
	}
	wantParts := [worldproto.CharacterEquipmentPartCount]uint16{101, 11200, 0, 201}
	if update.Parts != wantParts {
		t.Fatalf("expected mismatched-slot template to preserve instance-vnum parts %+v, got %+v", wantParts, update.Parts)
	}
}

func TestBuildBootstrapFramesKeepsInstanceVnumWhenTemplateHasNoAppearanceOverride(t *testing.T) {
	character := loginticket.Character{
		ID:       0x01030102,
		VID:      0x02040102,
		Name:     "VisibleSword",
		RaceNum:  2,
		MainPart: 101,
		HairPart: 201,
		Level:    20,
		X:        1700,
		Y:        2800,
		Equipment: []inventory.ItemInstance{
			{ID: 72, Vnum: 11200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
		},
	}
	templates := map[uint32]itemcatalog.Template{
		11200: {Vnum: 11200, Name: "Visible Practice Sword", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotWeapon.String()},
	}

	frames, err := BuildBootstrapFramesWithTemplates(character, templates)
	if err != nil {
		t.Fatalf("unexpected template appearance bootstrap frame error: %v", err)
	}
	update, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, frames[2]))
	if err != nil {
		t.Fatalf("decode non-overridden appearance update: %v", err)
	}
	wantParts := [worldproto.CharacterEquipmentPartCount]uint16{101, 11200, 0, 201}
	if update.Parts != wantParts {
		t.Fatalf("expected fallback instance-vnum update parts %+v, got %+v", wantParts, update.Parts)
	}
}
