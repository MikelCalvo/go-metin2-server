package itemstore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

func TestFileStoreRoundTripsEquipmentAppearanceVnum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	want := Snapshot{Templates: []Template{{
		Vnum:           11200,
		Name:           "Visible Practice Sword",
		Stackable:      false,
		MaxCount:       1,
		EquipSlot:      inventory.EquipmentSlotWeapon.String(),
		AppearanceVnum: 321,
	}}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save appearance-vnum template snapshot: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read appearance-vnum template snapshot: %v", err)
	}
	if !strings.Contains(string(raw), "\"appearance_vnum\": 321") {
		t.Fatalf("expected deterministic JSON to persist appearance_vnum, got:\n%s", raw)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load appearance-vnum template snapshot: %v", err)
	}
	if !reflect.DeepEqual(got, NormalizeSnapshot(want)) {
		t.Fatalf("unexpected appearance-vnum snapshot:\n got: %#v\nwant: %#v", got, NormalizeSnapshot(want))
	}
}

func TestFileStoreRejectsAppearanceVnumWithoutProjectedEquipmentSlot(t *testing.T) {
	cases := []struct {
		name     string
		template Template
		raw      string
	}{
		{
			name: "non-equipment",
			template: Template{
				Vnum:           27001,
				Name:           "Invalid Potion Appearance",
				Stackable:      true,
				MaxCount:       200,
				AppearanceVnum: 321,
			},
			raw: `{"templates":[{"vnum":27001,"name":"Invalid Potion Appearance","stackable":true,"max_count":200,"appearance_vnum":321}]}`,
		},
		{
			name: "unprojected equipment slot",
			template: Template{
				Vnum:           13000,
				Name:           "Invalid Shield Appearance",
				Stackable:      false,
				MaxCount:       1,
				EquipSlot:      inventory.EquipmentSlotShield.String(),
				AppearanceVnum: 321,
			},
			raw: `{"templates":[{"vnum":13000,"name":"Invalid Shield Appearance","stackable":false,"max_count":1,"equip_slot":"shield","appearance_vnum":321}]}`,
		},
		{
			name: "carrier overflow",
			template: Template{
				Vnum:           11200,
				Name:           "Overflow Appearance Sword",
				Stackable:      false,
				MaxCount:       1,
				EquipSlot:      inventory.EquipmentSlotWeapon.String(),
				AppearanceVnum: uint32(^uint16(0)) + 1,
			},
			raw: `{"templates":[{"vnum":11200,"name":"Overflow Appearance Sword","stackable":false,"max_count":1,"equip_slot":"weapon","appearance_vnum":65536}]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state", "item-templates.json")
			store := NewFileStore(path)
			if err := store.Save(Snapshot{Templates: []Template{tc.template}}); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot when saving %s appearance_vnum, got %v", tc.name, err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("create item template test dir: %v", err)
			}
			if err := os.WriteFile(path, []byte(tc.raw), 0o644); err != nil {
				t.Fatalf("write invalid appearance_vnum snapshot: %v", err)
			}
			if _, err := store.Load(); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot when loading %s appearance_vnum, got %v", tc.name, err)
			}
		})
	}
}
