package equipment

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
)

func wearable() itemcatalog.Template {
	return itemcatalog.Template{Vnum: 11200, Name: "Practice Armor", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotBody.String()}
}

func TestCanUse(t *testing.T) {
	cases := []struct {
		name     string
		template itemcatalog.Template
		subject  Subject
		want     bool
	}{
		{name: "allowed", template: wearable(), subject: Subject{Job: 0, RaceNum: 0, Empire: 1, Level: 10}, want: true},
		{name: "invalid template", template: itemcatalog.Template{}, subject: Subject{}, want: false},
		{name: "warrior anti flag", template: func() itemcatalog.Template { x := wearable(); x.AntiWarrior = true; return x }(), subject: Subject{Job: 0}, want: false},
		{name: "assassin anti flag", template: func() itemcatalog.Template { x := wearable(); x.AntiAssassin = true; return x }(), subject: Subject{Job: 1}, want: false},
		{name: "sura anti flag", template: func() itemcatalog.Template { x := wearable(); x.AntiSura = true; return x }(), subject: Subject{Job: 2}, want: false},
		{name: "shaman anti flag", template: func() itemcatalog.Template { x := wearable(); x.AntiShaman = true; return x }(), subject: Subject{Job: 3}, want: false},
		{name: "male anti flag", template: func() itemcatalog.Template { x := wearable(); x.AntiMale = true; return x }(), subject: Subject{RaceNum: 0}, want: false},
		{name: "female anti flag", template: func() itemcatalog.Template { x := wearable(); x.AntiFemale = true; return x }(), subject: Subject{RaceNum: 1}, want: false},
		{name: "empire anti flag", template: func() itemcatalog.Template { x := wearable(); x.AntiEmpireB = true; return x }(), subject: Subject{Empire: 2}, want: false},
		{name: "minimum level", template: func() itemcatalog.Template { x := wearable(); x.MinLevel = 11; return x }(), subject: Subject{Level: 10}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanUse(tc.template, tc.subject); got != tc.want {
				t.Fatalf("CanUse() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCanEquip(t *testing.T) {
	cases := []struct {
		name     string
		template itemcatalog.Template
		subject  Subject
		slot     inventory.EquipmentSlot
		want     bool
	}{
		{name: "allowed", template: wearable(), subject: Subject{Level: 1}, slot: inventory.EquipmentSlotBody, want: true},
		{name: "wrong destination", template: wearable(), subject: Subject{}, slot: inventory.EquipmentSlotWeapon, want: false},
		{name: "invalid destination", template: wearable(), subject: Subject{}, slot: inventory.EquipmentSlotNone, want: false},
		{name: "anti stack", template: func() itemcatalog.Template { x := wearable(); x.AntiStack = true; return x }(), subject: Subject{}, slot: inventory.EquipmentSlotBody, want: false},
		{name: "anti get", template: func() itemcatalog.Template { x := wearable(); x.AntiGet = true; return x }(), subject: Subject{}, slot: inventory.EquipmentSlotBody, want: false},
		{name: "anti drop", template: func() itemcatalog.Template { x := wearable(); x.AntiDrop = true; return x }(), subject: Subject{}, slot: inventory.EquipmentSlotBody, want: false},
		{name: "anti give", template: func() itemcatalog.Template { x := wearable(); x.AntiGive = true; return x }(), subject: Subject{}, slot: inventory.EquipmentSlotBody, want: false},
		{name: "anti sell", template: func() itemcatalog.Template { x := wearable(); x.AntiSell = true; return x }(), subject: Subject{}, slot: inventory.EquipmentSlotBody, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanEquip(tc.template, tc.subject, tc.slot); got != tc.want {
				t.Fatalf("CanEquip() = %v, want %v", got, tc.want)
			}
		})
	}
}
