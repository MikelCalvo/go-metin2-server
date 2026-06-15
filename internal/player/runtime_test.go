package player

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestRuntimeSeparatesLivePositionFromPersistedSnapshot(t *testing.T) {
	persisted := loginticket.Character{
		ID:       0x01030102,
		VID:      0x02040102,
		Name:     "PeerTwo",
		MapIndex: 1,
		X:        1300,
		Y:        2300,
		Empire:   2,
		GuildID:  15,
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	runtime.SetLivePosition(42, 1700, 2800)

	gotPersisted := runtime.PersistedSnapshot()
	if gotPersisted.MapIndex != 1 || gotPersisted.X != 1300 || gotPersisted.Y != 2300 {
		t.Fatalf("expected persisted snapshot to stay unchanged, got %+v", gotPersisted)
	}
	gotLive := runtime.LiveCharacter()
	if gotLive.MapIndex != 42 || gotLive.X != 1700 || gotLive.Y != 2800 {
		t.Fatalf("expected live character location to change independently, got %+v", gotLive)
	}
	if gotPosition := runtime.LivePosition(); gotPosition != worldruntime.NewPosition(42, 1700, 2800) {
		t.Fatalf("expected live position value object for relocated character, got %+v", gotPosition)
	}
	if gotLive.ID != persisted.ID || gotLive.VID != persisted.VID || gotLive.Name != persisted.Name || gotLive.Empire != persisted.Empire || gotLive.GuildID != persisted.GuildID {
		t.Fatalf("expected live character identity to remain anchored to persisted snapshot, got %+v", gotLive)
	}
	if link := runtime.SessionLink(); link.Login != "peer-two" || link.CharacterIndex != 1 {
		t.Fatalf("unexpected session link: %+v", link)
	}
	if persisted.MapIndex != 1 || persisted.X != 1300 || persisted.Y != 2300 {
		t.Fatalf("expected original persisted input to stay unchanged, got %+v", persisted)
	}
}

func TestRuntimeCanRefreshPersistedAndLiveSnapshotTogether(t *testing.T) {
	persisted := loginticket.Character{
		ID:       0x01030102,
		VID:      0x02040102,
		Name:     "PeerTwo",
		MapIndex: 1,
		X:        1300,
		Y:        2300,
		Empire:   2,
		GuildID:  15,
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	runtime.SetLivePosition(42, 1700, 2800)

	updated := persisted
	updated.MapIndex = 43
	updated.X = 1900
	updated.Y = 3100
	runtime.ApplyPersistedSnapshot(updated)

	if gotPersisted := runtime.PersistedSnapshot(); gotPersisted.MapIndex != 43 || gotPersisted.X != 1900 || gotPersisted.Y != 3100 {
		t.Fatalf("expected refreshed persisted snapshot, got %+v", gotPersisted)
	}
	if gotLive := runtime.LiveCharacter(); gotLive.MapIndex != 43 || gotLive.X != 1900 || gotLive.Y != 3100 {
		t.Fatalf("expected live character to realign with refreshed persisted snapshot, got %+v", gotLive)
	}
}

func TestRuntimeCanRefreshPersistedSnapshotWithoutClobberingLiveState(t *testing.T) {
	persisted := loginticket.Character{
		ID:       0x01030102,
		VID:      0x02040102,
		Name:     "PeerTwo",
		MapIndex: 1,
		X:        1300,
		Y:        2300,
		Empire:   2,
		GuildID:  15,
		Points: [255]int32{
			1: 700,
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	runtime.SetLivePosition(42, 1700, 2800)
	if _, ok := runtime.ApplyPointDelta(1, 1, -50); !ok {
		t.Fatal("expected live point delta to succeed before persisted-only refresh")
	}

	updated := persisted
	updated.MapIndex = 43
	updated.X = 1900
	updated.Y = 3100
	runtime.SetPersistedSnapshot(updated)

	if gotPersisted := runtime.PersistedSnapshot(); gotPersisted.MapIndex != 43 || gotPersisted.X != 1900 || gotPersisted.Y != 3100 {
		t.Fatalf("expected persisted snapshot to refresh without clobbering live state, got %+v", gotPersisted)
	}
	gotLive := runtime.LiveCharacter()
	if gotLive.MapIndex != 42 || gotLive.X != 1700 || gotLive.Y != 2800 {
		t.Fatalf("expected live position to stay unchanged after persisted-only refresh, got %+v", gotLive)
	}
	if gotLive.Points[1] != 650 {
		t.Fatalf("expected live points[1] to stay at 650 after persisted-only refresh, got %d", gotLive.Points[1])
	}
}

func TestNilRuntimeReturnsZeroLiveCharacter(t *testing.T) {
	var runtime *Runtime
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, loginticket.Character{}) {
		t.Fatalf("expected nil runtime to return zero live character, got %+v", got)
	}
}

func TestRuntimeAppliesExperienceOnlyDeathRewardToLiveState(t *testing.T) {
	persisted := loginticket.Character{
		ID:     0x01030101,
		VID:    0x02040101,
		Name:   "PeerOne",
		Points: [255]int32{ExperiencePointIndex: 25},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-one", CharacterIndex: 0})

	result, ok := runtime.ApplyStaticActorDeathReward(worldruntime.StaticActorDeathReward{Experience: 75})
	if !ok {
		t.Fatal("expected experience-only death reward to apply to live runtime state")
	}
	if result.ExperienceBefore != 25 || result.ExperienceAfter != 100 || result.Experience != 75 {
		t.Fatalf("unexpected experience reward result: %+v", result)
	}
	if got := runtime.LiveCharacter().Points[ExperiencePointIndex]; got != 100 {
		t.Fatalf("expected live experience point to increase to 100, got %d", got)
	}
	if got := runtime.PersistedSnapshot().Points[ExperiencePointIndex]; got != 25 {
		t.Fatalf("expected persisted experience point to remain unchanged before an explicit save, got %d", got)
	}
}

func TestRuntimeAppliesGoldOnlyDeathRewardToLiveState(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Gold: 25,
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.ApplyStaticActorDeathReward(worldruntime.StaticActorDeathReward{Gold: 75})
	if !ok {
		t.Fatal("expected gold-only death reward to apply to live runtime state")
	}
	if result.GoldBefore != 25 || result.GoldAfter != 100 || result.Gold != 75 {
		t.Fatalf("unexpected gold reward result: %+v", result)
	}
	if got := runtime.LiveGold(); got != 100 {
		t.Fatalf("expected live gold to increase to 100, got %d", got)
	}
	if got := runtime.PersistedSnapshot().Gold; got != 25 {
		t.Fatalf("expected persisted gold to remain unchanged before an explicit save, got %d", got)
	}
}

func TestRuntimeRejectsOverflowingExperienceDeathRewardWithoutMutation(t *testing.T) {
	persisted := loginticket.Character{
		ID:     0x01030101,
		VID:    0x02040101,
		Name:   "PeerOne",
		Points: [255]int32{ExperiencePointIndex: 1<<31 - 10},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-one", CharacterIndex: 0})

	result, ok := runtime.ApplyStaticActorDeathReward(worldruntime.StaticActorDeathReward{Experience: 11})
	if ok {
		t.Fatalf("expected overflowing experience death reward to fail closed, got %+v", result)
	}
	if got := runtime.LiveCharacter().Points[ExperiencePointIndex]; got != 1<<31-10 {
		t.Fatalf("expected live experience point to remain unchanged after overflow rejection, got %d", got)
	}
}

func TestRuntimeRejectsOverflowingGoldDeathRewardWithoutMutation(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Gold: ^uint64(0) - 10,
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.ApplyStaticActorDeathReward(worldruntime.StaticActorDeathReward{Gold: 11})
	if ok {
		t.Fatalf("expected overflowing gold death reward to fail closed, got %+v", result)
	}
	if got := runtime.LiveGold(); got != ^uint64(0)-10 {
		t.Fatalf("expected live gold to remain unchanged after overflow rejection, got %d", got)
	}
}

func TestRuntimeCanRestoreLiveRewardScalarsWithoutClobberingLivePosition(t *testing.T) {
	persisted := loginticket.Character{
		ID:       0x01030103,
		VID:      0x02040103,
		Name:     "PeerThree",
		MapIndex: 1,
		X:        1300,
		Y:        2300,
		Gold:     25,
		Points: [255]int32{
			ExperiencePointIndex: 75,
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-three", CharacterIndex: 2})
	runtime.SetLivePosition(42, 1700, 2800)
	if _, ok := runtime.ApplyStaticActorDeathReward(worldruntime.StaticActorDeathReward{Experience: 25, Gold: 50}); !ok {
		t.Fatal("expected death reward to apply before scalar restore")
	}

	if !runtime.SetLivePoint(ExperiencePointIndex, persisted.Points[ExperiencePointIndex]) {
		t.Fatal("expected live experience restore to succeed")
	}
	runtime.SetLiveGold(persisted.Gold)

	gotLive := runtime.LiveCharacter()
	if gotLive.Points[ExperiencePointIndex] != 75 || gotLive.Gold != 25 {
		t.Fatalf("expected live reward scalars to be restored, got exp=%d gold=%d", gotLive.Points[ExperiencePointIndex], gotLive.Gold)
	}
	if gotLive.MapIndex != 42 || gotLive.X != 1700 || gotLive.Y != 2800 {
		t.Fatalf("expected live position to remain unchanged after scalar restore, got %+v", gotLive)
	}
	if gotPersisted := runtime.PersistedSnapshot(); gotPersisted.Points[ExperiencePointIndex] != 75 || gotPersisted.Gold != 25 || gotPersisted.MapIndex != 1 {
		t.Fatalf("expected persisted snapshot to remain unchanged after scalar restore, got %+v", gotPersisted)
	}
}

func TestRuntimeRejectsGoldDeathRewardAbovePointChangeCarrierWithoutMutation(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030104,
		VID:  0x02040104,
		Name: "PeerFour",
		Gold: 25,
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-four", CharacterIndex: 3})

	result, ok := runtime.ApplyStaticActorDeathReward(worldruntime.StaticActorDeathReward{Gold: uint64(1 << 31)})
	if ok {
		t.Fatalf("expected gold reward above point-change carrier to fail closed, got %+v", result)
	}
	if got := runtime.LiveGold(); got != 25 {
		t.Fatalf("expected live gold to remain unchanged after carrier overflow rejection, got %d", got)
	}
}

func TestRuntimeRejectsNegativeExperienceDeathRewardOverflowWithoutMutation(t *testing.T) {
	persisted := loginticket.Character{
		ID:     0x01030103,
		VID:    0x02040103,
		Name:   "PeerThree",
		Points: [255]int32{ExperiencePointIndex: -5},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-three", CharacterIndex: 2})

	result, ok := runtime.ApplyStaticActorDeathReward(worldruntime.StaticActorDeathReward{Experience: uint64(1 << 63)})
	if ok {
		t.Fatalf("expected negative experience overflow reward to fail closed, got %+v", result)
	}
	if got := runtime.LiveCharacter().Points[ExperiencePointIndex]; got != -5 {
		t.Fatalf("expected live experience point to remain unchanged after negative overflow rejection, got %d", got)
	}
}

func TestRuntimeAppliesScalarDeathRewardWhenDropVnumsArePresent(t *testing.T) {
	persisted := loginticket.Character{
		ID:     0x01030105,
		VID:    0x02040105,
		Name:   "PeerFive",
		Gold:   25,
		Points: [255]int32{ExperiencePointIndex: 40},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-five", CharacterIndex: 4})

	result, ok := runtime.ApplyStaticActorDeathReward(worldruntime.StaticActorDeathReward{Experience: 35, Gold: 60, DropVnums: []uint32{27001}})
	if !ok {
		t.Fatal("expected scalar death reward to apply even when fixed drops are present")
	}
	if result.ExperienceBefore != 40 || result.ExperienceAfter != 75 || result.Experience != 35 {
		t.Fatalf("unexpected experience reward result: %+v", result)
	}
	if result.GoldBefore != 25 || result.GoldAfter != 85 || result.Gold != 60 {
		t.Fatalf("unexpected gold reward result: %+v", result)
	}
	live := runtime.LiveCharacter()
	if live.Points[ExperiencePointIndex] != 75 || live.Gold != 85 {
		t.Fatalf("expected live scalar reward state exp=75 gold=85, got exp=%d gold=%d", live.Points[ExperiencePointIndex], live.Gold)
	}
}

func bootstrapConsumableTemplate(vnum uint32, pointType uint8, pointIndex uint8, pointDelta int32, message string) itemcatalog.Template {
	return itemcatalog.Template{
		Vnum:      vnum,
		Name:      "Template Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{
			PointType:  pointType,
			PointIndex: pointIndex,
			PointDelta: pointDelta,
			Message:    message,
		},
	}
}

func antiFlaggedConsumableTemplate(vnum uint32, mutate func(*itemcatalog.Template)) itemcatalog.Template {
	template := bootstrapConsumableTemplate(vnum, 1, 1, 50, "consume:27001:+50")
	mutate(&template)
	return template
}

func bootstrapEquipmentPointTemplate(vnum uint32, equipSlot inventory.EquipmentSlot, pointType uint8, pointIndex uint8, pointDelta int32) itemcatalog.Template {
	return itemcatalog.Template{
		Vnum:      vnum,
		Name:      "Template Blade",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: equipSlot.String(),
		EquipEffect: &itemcatalog.PointEffect{
			PointType:  pointType,
			PointIndex: pointIndex,
			PointDelta: pointDelta,
		},
	}
}

func TestRuntimeItemUseConsumesBootstrapConsumableWithoutMutatingPersistedPoints(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Points: [255]int32{
			1: 700,
		},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.UseItem(5, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50"))
	if !ok {
		t.Fatal("expected bootstrap consumable use to succeed")
	}
	if result.ItemRemoved {
		t.Fatal("expected stacked consumable to remain in inventory after one use")
	}
	if result.Item.ID != 11 || result.Item.Vnum != 27001 || result.Item.Count != 2 || result.Item.Slot != 5 {
		t.Fatalf("unexpected updated inventory item: %+v", result.Item)
	}
	if result.PointAmount != 50 || result.PointValue != 750 || result.PointType != 1 {
		t.Fatalf("unexpected point change result: %+v", result)
	}
	if result.EffectMessage != "consume:27001:+50" {
		t.Fatalf("unexpected effect message: %q", result.EffectMessage)
	}
	if got := runtime.PersistedSnapshot().Points[1]; got != 700 {
		t.Fatalf("expected persisted points to remain unchanged, got %d", got)
	}
	live := runtime.LiveCharacter()
	if live.Points[1] != 750 {
		t.Fatalf("expected live points[1] to be incremented to 750, got %d", live.Points[1])
	}
	if !reflect.DeepEqual(live.Inventory, []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 2, Slot: 5}}) {
		t.Fatalf("unexpected live inventory after use: %#v", live.Inventory)
	}
	if !reflect.DeepEqual(persisted.Inventory, []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}}) {
		t.Fatalf("expected original persisted inventory input to stay unchanged, got %#v", persisted.Inventory)
	}
}

func TestRuntimeItemUseRemovesTheLastConsumableStack(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Points: [255]int32{
			1: 700,
		},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 1, Slot: 5}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.UseItem(5, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50"))
	if !ok {
		t.Fatal("expected final-stack consumable use to succeed")
	}
	if !result.ItemRemoved {
		t.Fatal("expected final-stack consumable use to remove the inventory slot")
	}
	if result.PointAmount != 50 || result.PointValue != 750 || result.PointType != 1 {
		t.Fatalf("unexpected point change result: %+v", result)
	}
	live := runtime.LiveCharacter()
	if len(live.Inventory) != 0 {
		t.Fatalf("expected live inventory to be empty after consuming the last stack, got %#v", live.Inventory)
	}
	if live.Points[1] != 750 {
		t.Fatalf("expected live points[1] to be incremented to 750, got %d", live.Points[1])
	}
}

func TestRuntimeItemUseLastStackRemovesOnlyItemQuickslotsForConsumedSlot(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 1, Slot: 5}},
		Quickslots: []loginticket.Quickslot{
			{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
			{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
			{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.UseItem(5, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50"))
	if !ok {
		t.Fatal("expected final-stack consumable use to succeed")
	}
	if !result.ItemRemoved {
		t.Fatal("expected final-stack consumable use to remove the inventory slot")
	}
	deletedQuickslots, ok := runtime.SyncItemQuickslotsForItemRemoval(5)
	if !ok {
		t.Fatal("expected removed item quickslot sync to succeed")
	}
	if !reflect.DeepEqual(deletedQuickslots, []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}) {
		t.Fatalf("unexpected deleted quickslots after final-stack consume: %#v", deletedQuickslots)
	}
	if !reflect.DeepEqual(runtime.LiveQuickslots(), []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5}, {Position: 4, Type: quickslotproto.TypeItem, Slot: 6}}) {
		t.Fatalf("unexpected live quickslots after final-stack consume: %#v", runtime.LiveQuickslots())
	}
}

func TestRuntimeItemUseRejectsAuthoredJobAndSexAntiFlagsWithoutMutation(t *testing.T) {
	cases := []struct {
		name     string
		job      uint8
		raceNum  uint16
		template itemcatalog.Template
	}{
		{name: "anti warrior", job: 0, raceNum: 0, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiWarrior = true })},
		{name: "anti assassin", job: 1, raceNum: 1, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiAssassin = true })},
		{name: "anti sura", job: 2, raceNum: 2, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiSura = true })},
		{name: "anti shaman", job: 3, raceNum: 3, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiShaman = true })},
		{name: "anti male", job: 0, raceNum: 0, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiMale = true })},
		{name: "anti female", job: 1, raceNum: 1, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiFemale = true })},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := loginticket.Character{
				ID:        0x01030102,
				VID:       0x02040102,
				Name:      "RestrictedPeer",
				Job:       tc.job,
				RaceNum:   tc.raceNum,
				Points:    [255]int32{1: 700},
				Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}},
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "restricted-peer", CharacterIndex: 1})
			before := runtime.LiveCharacter()

			if _, ok := runtime.UseItem(5, tc.template); ok {
				t.Fatalf("expected %s item use to fail closed", tc.name)
			}
			if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
				t.Fatalf("%s item use mutated live character: got %#v want %#v", tc.name, got, before)
			}
		})
	}
}

func TestRuntimeUseItemRejectsPointOverflowWithoutMutation(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Inventory: []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 2, Slot: 5}},
	}
	persisted.Points[1] = 1<<31 - 10
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	before := runtime.LiveCharacter()

	if result, ok := runtime.UseItem(5, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")); ok {
		t.Fatalf("expected overflowing item use to fail closed, got %+v", result)
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
		t.Fatalf("overflowing item use mutated live character: got %#v want %#v", got, before)
	}
}

func TestRuntimeUseItemRejectsMalformedLiveItemWithoutMutation(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 0, Vnum: 27001, Count: 1, Slot: 5}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	before := runtime.LiveCharacter()

	if result, ok := runtime.UseItem(5, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")); ok {
		t.Fatalf("expected malformed live item use to fail closed, got %+v", result)
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got.Inventory, before.Inventory) || got.Points[1] != before.Points[1] {
		t.Fatalf("malformed live item use mutated live state: got inventory=%#v points[1]=%d want inventory=%#v points[1]=%d", got.Inventory, got.Points[1], before.Inventory, before.Points[1])
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("malformed live item use mutated persisted inventory: got %#v want %#v", runtime.PersistedSnapshot().Inventory, persisted.Inventory)
	}
}

func TestRuntimeUseItemRejectsOverTemplateMaxStackWithoutMutation(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "OverMaxPeer",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 201, Slot: 5}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "over-max-peer", CharacterIndex: 1})
	before := runtime.LiveCharacter()
	template := bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")
	template.MaxCount = 200

	if result, ok := runtime.UseItem(5, template); ok {
		t.Fatalf("expected over-template-max live stack item use to fail closed, got %+v", result)
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
		t.Fatalf("over-template-max item use mutated live character: got %#v want %#v", got, before)
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("over-template-max item use mutated persisted inventory: got %#v want %#v", runtime.PersistedSnapshot().Inventory, persisted.Inventory)
	}
}

func TestRuntimeUseItemRejectsOverUint8TemplateMaxWithoutMutation(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "WideMaxPeer",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 12, Vnum: 27001, Count: 2, Slot: 5}},
		Quickslots: []loginticket.Quickslot{
			{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "wide-max-peer", CharacterIndex: 1})
	before := runtime.LiveCharacter()
	template := bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")
	template.MaxCount = 256

	if result, ok := runtime.UseItem(5, template); ok {
		t.Fatalf("expected over-uint8 template max item use to fail closed, got %+v", result)
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
		t.Fatalf("over-uint8 template max item use mutated live character: got %#v want %#v", got, before)
	}
	if got := runtime.PersistedSnapshot(); !reflect.DeepEqual(got.Inventory, persisted.Inventory) || !reflect.DeepEqual(got.Quickslots, persisted.Quickslots) || got.Points[1] != persisted.Points[1] {
		t.Fatalf("over-uint8 template max item use mutated persisted state: inventory=%#v quickslots=%#v points[1]=%d", got.Inventory, got.Quickslots, got.Points[1])
	}
}

func TestRuntimeUseItemRejectsEquippableTemplateWithoutMutation(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 11200, Count: 1, Slot: 5}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	before := runtime.LiveCharacter()
	template := itemcatalog.Template{
		Vnum:      11200,
		Name:      "Equippable Consumable-Like Sword",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: inventory.EquipmentSlotWeapon.String(),
		UseEffect: &itemcatalog.UseEffect{PointType: 1, PointIndex: 1, PointDelta: 50, Message: "consume:11200:+50"},
	}

	if result, ok := runtime.UseItem(5, template); ok {
		t.Fatalf("expected equippable-template item use to fail closed, got %+v", result)
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
		t.Fatalf("equippable-template item use mutated live character: got %#v want %#v", got, before)
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("equippable-template item use mutated persisted inventory: got %#v want %#v", runtime.PersistedSnapshot().Inventory, persisted.Inventory)
	}
}

func TestRuntimeUseItemRejectsAuthoredJobSexLevelAndTransferAntiFlagsWithoutMutation(t *testing.T) {
	cases := []struct {
		name     string
		job      uint8
		raceNum  uint16
		level    uint8
		template itemcatalog.Template
	}{
		{name: "anti warrior", job: 0, raceNum: 0, level: 1, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiWarrior = true })},
		{name: "anti assassin", job: 1, raceNum: 1, level: 1, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiAssassin = true })},
		{name: "anti sura", job: 2, raceNum: 2, level: 1, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiSura = true })},
		{name: "anti shaman", job: 3, raceNum: 3, level: 1, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiShaman = true })},
		{name: "anti male", job: 0, raceNum: 0, level: 1, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiMale = true })},
		{name: "anti female", job: 1, raceNum: 1, level: 1, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiFemale = true })},
		{name: "min level", job: 0, raceNum: 0, level: 9, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.MinLevel = 10 })},
		{name: "anti stack", job: 0, raceNum: 0, level: 1, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiStack = true })},
		{name: "anti drop", job: 0, raceNum: 0, level: 1, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiDrop = true })},
		{name: "anti give", job: 0, raceNum: 0, level: 1, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiGive = true })},
		{name: "anti sell", job: 0, raceNum: 0, level: 1, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiSell = true })},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := loginticket.Character{
				ID:      0x01030102,
				VID:     0x02040102,
				Name:    "RestrictedUsePeer",
				Job:     tc.job,
				RaceNum: tc.raceNum,
				Level:   tc.level,
				Points:  [255]int32{1: 700},
				Inventory: []inventory.ItemInstance{
					{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
				},
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "restricted-use-peer", CharacterIndex: 1})
			before := runtime.LiveCharacter()

			if result, ok := runtime.UseItem(5, tc.template); ok {
				t.Fatalf("expected %s item use to fail closed, got %+v", tc.name, result)
			}
			if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
				t.Fatalf("%s item use mutated live character: got %#v want %#v", tc.name, got, before)
			}
			if got := runtime.PersistedSnapshot(); !reflect.DeepEqual(got.Inventory, persisted.Inventory) || got.Points[1] != persisted.Points[1] {
				t.Fatalf("%s item use mutated persisted state: inventory=%#v points[1]=%d", tc.name, got.Inventory, got.Points[1])
			}
		})
	}
}

func TestRuntimeDropInventoryItemRejectsDuplicateSlotOccupancyWithoutMutation(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "DropDuplicateSlot",
		Inventory: []inventory.ItemInstance{
			{ID: 901, Vnum: 27001, Count: 5, Slot: 5},
			{ID: 902, Vnum: 27002, Count: 1, Slot: 5},
		},
		Quickslots: []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "drop-duplicate-slot", CharacterIndex: 0})
	before := runtime.LiveCharacter()

	if _, ok := runtime.DropInventoryItem(5, 1); ok {
		t.Fatal("expected item drop to reject duplicate carried-slot occupancy")
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
		t.Fatalf("duplicate-slot drop mutated live state:\n got: %#v\nwant: %#v", got, before)
	}
}

func TestRuntimeUseItemOnItemRejectsAuthoredJobSexAndLevelRestrictionsWithoutMutation(t *testing.T) {
	cases := []struct {
		name     string
		job      uint8
		raceNum  uint16
		level    uint8
		template itemcatalog.Template
	}{
		{name: "anti warrior", job: 0, raceNum: 0, level: 1, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiWarrior = true })},
		{name: "anti assassin", job: 1, raceNum: 1, level: 1, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiAssassin = true })},
		{name: "anti sura", job: 2, raceNum: 2, level: 1, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiSura = true })},
		{name: "anti shaman", job: 3, raceNum: 3, level: 1, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiShaman = true })},
		{name: "anti male", job: 0, raceNum: 0, level: 1, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiMale = true })},
		{name: "anti female", job: 1, raceNum: 1, level: 1, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.AntiFemale = true })},
		{name: "min level", job: 0, raceNum: 0, level: 9, template: antiFlaggedConsumableTemplate(27001, func(t *itemcatalog.Template) { t.MinLevel = 10 })},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := loginticket.Character{
				ID:      0x01030102,
				VID:     0x02040102,
				Name:    "RestrictedPeer",
				Job:     tc.job,
				RaceNum: tc.raceNum,
				Level:   tc.level,
				Inventory: []inventory.ItemInstance{
					{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
					{ID: 12, Vnum: 27001, Count: 2, Slot: 8},
				},
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "restricted-peer", CharacterIndex: 1})
			before := runtime.LiveCharacter()

			if _, ok := runtime.UseItemOnItem(5, 8, tc.template); ok {
				t.Fatalf("expected %s item use-to-item to fail closed", tc.name)
			}
			if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
				t.Fatalf("%s item use-to-item mutated live character: got %#v want %#v", tc.name, got, before)
			}
		})
	}
}

func TestRuntimeUseItemOnItemRejectsEquippableTemplateWithoutMutation(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}, {ID: 12, Vnum: 27001, Count: 4, Slot: 6}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	before := runtime.LiveCharacter()
	template := itemcatalog.Template{Vnum: 27001, Name: "Equippable Stack", Stackable: true, MaxCount: 200, EquipSlot: inventory.EquipmentSlotBody.String()}

	if _, ok := runtime.UseItemOnItem(5, 6, template); ok {
		t.Fatal("expected use-to-item to reject equippable templates even when stacks otherwise match")
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
		t.Fatalf("equippable-template use-to-item mutated live character: got %#v want %#v", got, before)
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("equippable-template use-to-item mutated persisted inventory: got %#v", runtime.PersistedSnapshot().Inventory)
	}
}

func TestRuntimeUseItemOnItemRejectsInvalidSourceOrTargetWithoutMutation(t *testing.T) {
	cases := []struct {
		name   string
		source inventory.SlotIndex
		target inventory.SlotIndex
	}{
		{name: "same source and target slot", source: 5, target: 5},
		{name: "source out of carried range", source: inventory.CarriedInventorySlotCount, target: 6},
		{name: "target out of carried range", source: 5, target: inventory.CarriedInventorySlotCount},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := loginticket.Character{
				ID:        0x01030102,
				VID:       0x02040102,
				Name:      "PeerTwo",
				Points:    [255]int32{1: 700},
				Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}, {ID: 12, Vnum: 27001, Count: 4, Slot: 6}},
				Quickslots: []loginticket.Quickslot{
					{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
					{Position: 3, Type: quickslotproto.TypeItem, Slot: 6},
				},
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
			before := runtime.LiveCharacter()

			if result, ok := runtime.UseItemOnItem(tc.source, tc.target, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")); ok {
				t.Fatalf("expected use-to-item to reject %s, got %+v", tc.name, result)
			}
			if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
				t.Fatalf("expected rejected %s use-to-item to leave live state unchanged, got %#v want %#v", tc.name, got, before)
			}
			if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
				t.Fatalf("expected rejected %s use-to-item to leave persisted inventory unchanged, got %#v", tc.name, runtime.PersistedSnapshot().Inventory)
			}
			if !reflect.DeepEqual(runtime.PersistedSnapshot().Quickslots, persisted.Quickslots) {
				t.Fatalf("expected rejected %s use-to-item to leave persisted quickslots unchanged, got %#v", tc.name, runtime.PersistedSnapshot().Quickslots)
			}
		})
	}
}

func TestRuntimeUseItemOnItemMergesCompatibleStacksWithoutPointEffect(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}, {ID: 12, Vnum: 27001, Count: 4, Slot: 6}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.UseItemOnItem(5, 6, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50"))
	if !ok {
		t.Fatal("expected compatible stack use-to-item merge to succeed")
	}
	if !result.Changed || result.CountOnly || result.FromOccupied {
		t.Fatalf("unexpected use-to-item merge result metadata: %+v", result)
	}
	if result.ToItem.ID != 12 || result.ToItem.Vnum != 27001 || result.ToItem.Count != 7 || result.ToItem.Slot != 6 {
		t.Fatalf("unexpected destination item after use-to-item merge: %+v", result.ToItem)
	}
	live := runtime.LiveCharacter()
	if live.Points[1] != 700 {
		t.Fatalf("expected use-to-item merge to avoid point effects, got points[1]=%d", live.Points[1])
	}
	if !reflect.DeepEqual(live.Inventory, []inventory.ItemInstance{{ID: 12, Vnum: 27001, Count: 7, Slot: 6}}) {
		t.Fatalf("unexpected live inventory after use-to-item merge: %#v", live.Inventory)
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected persisted inventory to remain unchanged before save-back, got %#v", runtime.PersistedSnapshot().Inventory)
	}
}

func TestRuntimeUseItemOnItemFullMergeRemovesOnlySourceItemQuickslots(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}, {ID: 12, Vnum: 27001, Count: 4, Slot: 6}},
		Quickslots: []loginticket.Quickslot{
			{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
			{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
			{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.UseItemOnItem(5, 6, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50"))
	if !ok {
		t.Fatal("expected full use-to-item merge to succeed")
	}
	if !result.Changed || result.FromOccupied || result.CountOnly {
		t.Fatalf("unexpected full use-to-item result metadata: %+v", result)
	}
	deletedQuickslots, ok := runtime.SyncItemQuickslotsForItemRemoval(5)
	if !ok {
		t.Fatal("expected full use-to-item source quickslot sync to succeed")
	}
	if !reflect.DeepEqual(deletedQuickslots, []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}) {
		t.Fatalf("unexpected deleted quickslots after full use-to-item merge: %#v", deletedQuickslots)
	}
	if !reflect.DeepEqual(runtime.LiveQuickslots(), []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5}, {Position: 4, Type: quickslotproto.TypeItem, Slot: 6}}) {
		t.Fatalf("unexpected live quickslots after full use-to-item merge: %#v", runtime.LiveQuickslots())
	}
}

func TestRuntimeUseItemOnItemMergesPartialStackWhenTargetHasLimitedRoom(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 7, Slot: 5}, {ID: 12, Vnum: 27001, Count: 8, Slot: 6}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	template := bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")
	template.MaxCount = 10
	result, ok := runtime.UseItemOnItem(5, 6, template)
	if !ok {
		t.Fatal("expected partial compatible stack use-to-item merge to succeed")
	}
	if !result.Changed || !result.CountOnly || !result.FromOccupied || !result.ToOccupied {
		t.Fatalf("unexpected partial use-to-item metadata: %+v", result)
	}
	if result.FromItem.ID != 11 || result.FromItem.Count != 5 || result.FromItem.Slot != 5 {
		t.Fatalf("unexpected source remainder after partial merge: %+v", result.FromItem)
	}
	if result.ToItem.ID != 12 || result.ToItem.Count != 10 || result.ToItem.Slot != 6 {
		t.Fatalf("unexpected target stack after partial merge: %+v", result.ToItem)
	}
	live := runtime.LiveCharacter()
	if live.Points[1] != 700 {
		t.Fatalf("expected partial use-to-item merge to avoid point effects, got points[1]=%d", live.Points[1])
	}
	if !reflect.DeepEqual(live.Inventory, []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 5, Slot: 5}, {ID: 12, Vnum: 27001, Count: 10, Slot: 6}}) {
		t.Fatalf("unexpected live inventory after partial use-to-item merge: %#v", live.Inventory)
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected persisted inventory to remain unchanged before save-back, got %#v", runtime.PersistedSnapshot().Inventory)
	}
}

func TestRuntimeUseItemOnItemLatePartialMergeValidationFailureLeavesSourceAndTargetUnchanged(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 7, Slot: 5}, {ID: 12, Vnum: 27001, Count: 8, Slot: 6}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")
	template.MaxCount = 10
	before := runtime.LiveCharacter()

	result, ok := runtime.useItemOnItem(5, 6, template, func(item inventory.ItemInstance) inventory.ItemInstance {
		if item.Slot == 5 {
			item.ID = 0
		}
		return item
	})
	if ok {
		t.Fatalf("expected late source validation failure to reject partial use-to-item merge, got %+v", result)
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
		t.Fatalf("expected late source validation failure to leave live character unchanged, got %#v want %#v", got, before)
	}
	if got := runtime.PersistedSnapshot(); !reflect.DeepEqual(got.Inventory, persisted.Inventory) || got.Points[1] != persisted.Points[1] {
		t.Fatalf("expected late source validation failure to leave persisted inventory/points unchanged, got inventory=%#v points[1]=%d", got.Inventory, got.Points[1])
	}
}

func TestRuntimeUseItemOnItemRejectsEmptySourceOrTargetWithoutMutatingState(t *testing.T) {
	cases := []struct {
		name      string
		source    inventory.SlotIndex
		target    inventory.SlotIndex
		inventory []inventory.ItemInstance
	}{
		{
			name:      "empty source",
			source:    5,
			target:    6,
			inventory: []inventory.ItemInstance{{ID: 12, Vnum: 27001, Count: 4, Slot: 6}},
		},
		{
			name:      "empty target",
			source:    5,
			target:    6,
			inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}},
		},
		{
			name:      "same source and target",
			source:    5,
			target:    5,
			inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}},
		},
		{
			name:   "zero-count source",
			source: 5,
			target: 6,
			inventory: []inventory.ItemInstance{
				{ID: 11, Vnum: 27001, Count: 0, Slot: 5},
				{ID: 12, Vnum: 27001, Count: 4, Slot: 6},
			},
		},
		{
			name:   "zero-count target",
			source: 5,
			target: 6,
			inventory: []inventory.ItemInstance{
				{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
				{ID: 12, Vnum: 27001, Count: 0, Slot: 6},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := loginticket.Character{
				ID:        0x01030102,
				VID:       0x02040102,
				Name:      "PeerTwo",
				Points:    [255]int32{1: 700},
				Inventory: tc.inventory,
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

			if _, ok := runtime.UseItemOnItem(tc.source, tc.target, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")); ok {
				t.Fatalf("expected use-to-item to reject %s", tc.name)
			}
			if got := runtime.LiveCharacter(); !reflect.DeepEqual(got.Inventory, persisted.Inventory) || got.Points[1] != 700 {
				t.Fatalf("expected rejected empty/same-slot use-to-item to leave live state unchanged, got %#v points[1]=%d", got.Inventory, got.Points[1])
			}
			if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
				t.Fatalf("expected rejected empty/same-slot use-to-item to leave persisted inventory unchanged, got %#v", runtime.PersistedSnapshot().Inventory)
			}
		})
	}
}

func TestRuntimeUseItemOnItemRejectsPointUseTemplateWithoutCompatibleTarget(t *testing.T) {
	cases := []struct {
		name      string
		inventory []inventory.ItemInstance
	}{
		{
			name:      "different target vnum",
			inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}, {ID: 12, Vnum: 27002, Count: 4, Slot: 6}},
		},
		{
			name:      "duplicate source and target item id",
			inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}, {ID: 11, Vnum: 27001, Count: 4, Slot: 6}},
		},
		{
			name:      "duplicate source slot occupancy",
			inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}, {ID: 13, Vnum: 27001, Count: 2, Slot: 5}, {ID: 12, Vnum: 27001, Count: 4, Slot: 6}},
		},
		{
			name:      "duplicate target slot occupancy",
			inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}, {ID: 12, Vnum: 27001, Count: 4, Slot: 6}, {ID: 13, Vnum: 27001, Count: 2, Slot: 6}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := loginticket.Character{
				ID:        0x01030102,
				VID:       0x02040102,
				Name:      "PeerTwo",
				Points:    [255]int32{1: 700},
				Inventory: tc.inventory,
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

			if _, ok := runtime.UseItemOnItem(5, 6, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")); ok {
				t.Fatalf("expected use-to-item to reject %s instead of falling back to normal item use", tc.name)
			}
			live := runtime.LiveCharacter()
			if live.Points[1] != 700 {
				t.Fatalf("expected rejected use-to-item to avoid point effects, got points[1]=%d", live.Points[1])
			}
			if !reflect.DeepEqual(live.Inventory, persisted.Inventory) {
				t.Fatalf("expected rejected use-to-item to leave inventory unchanged, got %#v", live.Inventory)
			}
		})
	}
}

func TestRuntimeUseItemOnItemRejectsNonStackableTemplateWithoutMutatingState(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 11200, Count: 2, Slot: 5}, {ID: 12, Vnum: 11200, Count: 1, Slot: 6}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := itemcatalog.Template{Vnum: 11200, Name: "non-stackable sword", MaxCount: 1, Stackable: false}

	if _, ok := runtime.UseItemOnItem(5, 6, template); ok {
		t.Fatal("expected use-to-item to reject non-stackable templates even when counts could otherwise fit")
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got.Inventory, persisted.Inventory) || got.Points[1] != 700 {
		t.Fatalf("expected rejected non-stackable use-to-item to leave live state unchanged, got %#v points[1]=%d", got.Inventory, got.Points[1])
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected rejected non-stackable use-to-item to leave persisted inventory unchanged, got %#v", runtime.PersistedSnapshot().Inventory)
	}
}

func TestRuntimeUseItemOnItemRejectsInvalidTemplateWithoutMutatingState(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27003, Count: 3, Slot: 5}, {ID: 12, Vnum: 27003, Count: 4, Slot: 6}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	invalidTemplate := itemcatalog.Template{Vnum: 27003, Stackable: true, MaxCount: 200}

	if _, ok := runtime.UseItemOnItem(5, 6, invalidTemplate); ok {
		t.Fatal("expected use-to-item to reject invalid template metadata even when stacks otherwise match")
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got.Inventory, persisted.Inventory) || got.Points[1] != 700 {
		t.Fatalf("expected invalid-template use-to-item to leave live state unchanged, got %#v points[1]=%d", got.Inventory, got.Points[1])
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected invalid-template use-to-item to leave persisted inventory unchanged, got %#v", runtime.PersistedSnapshot().Inventory)
	}
}

func TestRuntimeUseItemOnItemRejectsAntiStackTransferTemplatesWithoutMutatingState(t *testing.T) {
	cases := []struct {
		name       string
		configure  func(*itemcatalog.Template)
		failureMsg string
	}{
		{
			name: "anti-stack",
			configure: func(template *itemcatalog.Template) {
				template.AntiStack = true
			},
			failureMsg: "anti-stack templates",
		},
		{
			name: "anti-drop",
			configure: func(template *itemcatalog.Template) {
				template.AntiDrop = true
			},
			failureMsg: "anti-drop templates",
		},
		{
			name: "anti-give",
			configure: func(template *itemcatalog.Template) {
				template.AntiGive = true
			},
			failureMsg: "anti-give templates",
		},
		{
			name: "anti-sell",
			configure: func(template *itemcatalog.Template) {
				template.AntiSell = true
			},
			failureMsg: "anti-sell templates",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := loginticket.Character{
				ID:        0x01030102,
				VID:       0x02040102,
				Name:      "PeerTwo",
				Points:    [255]int32{1: 700},
				Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27003, Count: 3, Slot: 5}, {ID: 12, Vnum: 27003, Count: 4, Slot: 6}},
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
			template := bootstrapConsumableTemplate(27003, 1, 1, 50, "consume:27003:+50")
			tc.configure(&template)

			if _, ok := runtime.UseItemOnItem(5, 6, template); ok {
				t.Fatalf("expected use-to-item to reject %s even when source and target are otherwise mergeable", tc.failureMsg)
			}
			if got := runtime.LiveCharacter(); !reflect.DeepEqual(got.Inventory, persisted.Inventory) || got.Points[1] != 700 {
				t.Fatalf("expected rejected %s use-to-item to leave live state unchanged, got %#v points[1]=%d", tc.name, got.Inventory, got.Points[1])
			}
			if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
				t.Fatalf("expected rejected %s use-to-item to leave persisted inventory unchanged, got %#v", tc.name, runtime.PersistedSnapshot().Inventory)
			}
		})
	}
}

func TestRuntimeUseItemOnItemFullMergeDeletesAllSourceItemQuickslots(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}, {ID: 12, Vnum: 27001, Count: 4, Slot: 6}},
		Quickslots: []loginticket.Quickslot{
			{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
			{Position: 4, Type: quickslotproto.TypeItem, Slot: 5},
			{Position: 5, Type: quickslotproto.TypeSkill, Slot: 5},
			{Position: 6, Type: quickslotproto.TypeItem, Slot: 6},
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.UseItemOnItem(5, 6, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50"))
	if !ok {
		t.Fatal("expected full use-to-item merge to succeed")
	}
	if !result.Changed || result.FromOccupied || result.CountOnly {
		t.Fatalf("unexpected full use-to-item result metadata: %+v", result)
	}
	deletedQuickslots, ok := runtime.SyncItemQuickslotsForItemRemoval(5)
	if !ok {
		t.Fatal("expected full use-to-item source quickslot sync to succeed")
	}
	wantDeleted := []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 5},
	}
	if !reflect.DeepEqual(deletedQuickslots, wantDeleted) {
		t.Fatalf("unexpected deleted quickslots after full use-to-item merge: got %#v want %#v", deletedQuickslots, wantDeleted)
	}
	wantQuickslots := []loginticket.Quickslot{
		{Position: 5, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 6, Type: quickslotproto.TypeItem, Slot: 6},
	}
	if !reflect.DeepEqual(runtime.LiveQuickslots(), wantQuickslots) {
		t.Fatalf("unexpected live quickslots after full use-to-item merge: got %#v want %#v", runtime.LiveQuickslots(), wantQuickslots)
	}
}

func TestRuntimeUseItemOnItemRejectsLockedSourceOrTargetWithoutMutatingState(t *testing.T) {
	cases := []struct {
		name      string
		inventory []inventory.ItemInstance
	}{
		{
			name:      "locked source",
			inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5, Locked: true}, {ID: 12, Vnum: 27001, Count: 4, Slot: 6}},
		},
		{
			name:      "locked target",
			inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}, {ID: 12, Vnum: 27001, Count: 4, Slot: 6, Locked: true}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := loginticket.Character{
				ID:        0x01030102,
				VID:       0x02040102,
				Name:      "PeerTwo",
				Points:    [255]int32{1: 700},
				Inventory: tc.inventory,
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

			if _, ok := runtime.UseItemOnItem(5, 6, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")); ok {
				t.Fatalf("expected use-to-item to reject %s", tc.name)
			}
			if got := runtime.LiveCharacter(); !reflect.DeepEqual(got.Inventory, persisted.Inventory) || got.Points[1] != 700 {
				t.Fatalf("expected rejected locked use-to-item to leave live state unchanged, got %#v points[1]=%d", got.Inventory, got.Points[1])
			}
			if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
				t.Fatalf("expected rejected locked use-to-item to leave persisted inventory unchanged, got %#v", runtime.PersistedSnapshot().Inventory)
			}
		})
	}
}

func TestRuntimeUseItemOnItemRejectsEquippedSourceOrTargetWithoutMutatingState(t *testing.T) {
	cases := []struct {
		name      string
		inventory []inventory.ItemInstance
	}{
		{
			name: "equipped source",
			inventory: []inventory.ItemInstance{
				{ID: 11, Vnum: 27001, Count: 3, Slot: 5, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
				{ID: 12, Vnum: 27001, Count: 4, Slot: 6},
			},
		},
		{
			name: "equipped target",
			inventory: []inventory.ItemInstance{
				{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
				{ID: 12, Vnum: 27001, Count: 4, Slot: 6, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := loginticket.Character{
				ID:        0x01030102,
				VID:       0x02040102,
				Name:      "PeerTwo",
				Points:    [255]int32{1: 700},
				Inventory: tc.inventory,
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
			before := runtime.LiveCharacter()

			if result, ok := runtime.UseItemOnItem(5, 6, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")); ok {
				t.Fatalf("expected use-to-item to reject %s, got %+v", tc.name, result)
			}
			if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
				t.Fatalf("expected rejected %s use-to-item to leave live state unchanged, got %#v want %#v", tc.name, got, before)
			}
			if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
				t.Fatalf("expected rejected %s use-to-item to leave persisted inventory unchanged, got %#v", tc.name, runtime.PersistedSnapshot().Inventory)
			}
		})
	}
}

func TestRuntimeUseItemOnItemRejectsOverMaxSourceStackWithoutMutatingState(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 201, Slot: 5}, {ID: 12, Vnum: 27001, Count: 4, Slot: 6}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")
	template.MaxCount = 200

	if _, ok := runtime.UseItemOnItem(5, 6, template); ok {
		t.Fatal("expected use-to-item to reject source stacks that already exceed template max_count")
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got.Inventory, persisted.Inventory) || got.Points[1] != 700 {
		t.Fatalf("expected rejected over-max use-to-item to leave live state unchanged, got %#v points[1]=%d", got.Inventory, got.Points[1])
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected rejected over-max use-to-item to leave persisted inventory unchanged, got %#v", runtime.PersistedSnapshot().Inventory)
	}
}

func TestRuntimeUseItemOnItemRejectsAlreadyFullTargetStackWithoutMutatingState(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 2, Slot: 5}, {ID: 12, Vnum: 27001, Count: 200, Slot: 6}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")
	template.MaxCount = 200

	if result, ok := runtime.UseItemOnItem(5, 6, template); ok {
		t.Fatalf("expected use-to-item to reject already-full target stacks, got %+v", result)
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got.Inventory, persisted.Inventory) || got.Points[1] != 700 {
		t.Fatalf("expected rejected full-target use-to-item to leave live state unchanged, got %#v points[1]=%d", got.Inventory, got.Points[1])
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected rejected full-target use-to-item to leave persisted inventory unchanged, got %#v", runtime.PersistedSnapshot().Inventory)
	}
}

func TestRuntimeUseItemRejectsMaxCountBeyondClientCountRangeWithoutMutatingState(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 299, Slot: 5}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")
	template.MaxCount = 300

	if result, ok := runtime.UseItem(5, template); ok {
		t.Fatalf("expected item use to reject template max_count values that cannot be represented by owned item refresh packets, got %+v", result)
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got.Inventory, persisted.Inventory) || got.Points[1] != 700 {
		t.Fatalf("expected rejected oversized-template item use to leave live state unchanged, got %#v points[1]=%d", got.Inventory, got.Points[1])
	}
	if got := runtime.PersistedSnapshot(); !reflect.DeepEqual(got.Inventory, persisted.Inventory) || got.Points[1] != persisted.Points[1] {
		t.Fatalf("expected rejected oversized-template item use to leave persisted state unchanged, got inventory=%#v points[1]=%d", got.Inventory, got.Points[1])
	}
}

func TestRuntimeUseItemOnItemRejectsMaxCountBeyondClientCountRangeWithoutMutatingState(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 2, Slot: 5}, {ID: 12, Vnum: 27001, Count: 250, Slot: 6}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")
	template.MaxCount = 300

	if _, ok := runtime.UseItemOnItem(5, 6, template); ok {
		t.Fatal("expected use-to-item to reject template max_count values that cannot be represented by owned item refresh packets")
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got.Inventory, persisted.Inventory) || got.Points[1] != 700 {
		t.Fatalf("expected rejected oversized-template use-to-item to leave live state unchanged, got %#v points[1]=%d", got.Inventory, got.Points[1])
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected rejected oversized-template use-to-item to leave persisted inventory unchanged, got %#v", runtime.PersistedSnapshot().Inventory)
	}
}

func TestRuntimeUseItemOnItemRejectsFullTargetStackWithoutMutatingState(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 2, Slot: 5}, {ID: 12, Vnum: 27001, Count: 200, Slot: 6}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")
	template.MaxCount = 200

	if _, ok := runtime.UseItemOnItem(5, 6, template); ok {
		t.Fatal("expected use-to-item to reject target stacks that are already at template max_count")
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got.Inventory, persisted.Inventory) || got.Points[1] != 700 {
		t.Fatalf("expected rejected full-target use-to-item to leave live state unchanged, got %#v points[1]=%d", got.Inventory, got.Points[1])
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected rejected full-target use-to-item to leave persisted inventory unchanged, got %#v", runtime.PersistedSnapshot().Inventory)
	}
}

func TestRuntimeUseItemOnItemRejectsZeroCountSourceOrTargetWithoutMutatingState(t *testing.T) {
	cases := []struct {
		name      string
		inventory []inventory.ItemInstance
	}{
		{
			name:      "zero-count source",
			inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 0, Slot: 5}, {ID: 12, Vnum: 27001, Count: 2, Slot: 6}},
		},
		{
			name:      "zero-count target",
			inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 2, Slot: 5}, {ID: 12, Vnum: 27001, Count: 0, Slot: 6}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := loginticket.Character{
				ID:        0x01030102,
				VID:       0x02040102,
				Name:      "PeerTwo",
				Points:    [255]int32{1: 700},
				Inventory: tc.inventory,
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
			template := bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")
			template.MaxCount = 200
			before := runtime.LiveCharacter()

			if result, ok := runtime.UseItemOnItem(5, 6, template); ok {
				t.Fatalf("expected use-to-item to reject %s without mutation, got %+v", tc.name, result)
			}
			if got := runtime.LiveCharacter(); !reflect.DeepEqual(got.Inventory, before.Inventory) || got.Points[1] != before.Points[1] {
				t.Fatalf("expected rejected %s use-to-item to leave live state unchanged, got %#v points[1]=%d", tc.name, got.Inventory, got.Points[1])
			}
			if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, before.Inventory) {
				t.Fatalf("expected rejected %s use-to-item to leave persisted inventory unchanged, got %#v", tc.name, runtime.PersistedSnapshot().Inventory)
			}
		})
	}
}

func TestRuntimeUseItemOnItemRejectsMalformedSourceOrTargetWithoutMutatingState(t *testing.T) {
	cases := []struct {
		name      string
		inventory []inventory.ItemInstance
	}{
		{
			name:      "malformed source",
			inventory: []inventory.ItemInstance{{ID: 0, Vnum: 27001, Count: 2, Slot: 5}, {ID: 12, Vnum: 27001, Count: 3, Slot: 6}},
		},
		{
			name:      "malformed target",
			inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 2, Slot: 5}, {ID: 0, Vnum: 27001, Count: 3, Slot: 6}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := loginticket.Character{
				ID:        0x01030102,
				VID:       0x02040102,
				Name:      "PeerTwo",
				Points:    [255]int32{1: 700},
				Inventory: tc.inventory,
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
			template := bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")
			template.MaxCount = 200
			before := runtime.LiveCharacter()

			if result, ok := runtime.UseItemOnItem(5, 6, template); ok {
				t.Fatalf("expected use-to-item to reject %s without mutation, got %+v", tc.name, result)
			}
			if got := runtime.LiveCharacter(); !reflect.DeepEqual(got.Inventory, before.Inventory) || got.Points[1] != before.Points[1] {
				t.Fatalf("expected rejected %s use-to-item to leave live state unchanged, got %#v points[1]=%d", tc.name, got.Inventory, got.Points[1])
			}
			if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, before.Inventory) {
				t.Fatalf("expected rejected %s use-to-item to leave persisted inventory unchanged, got %#v", tc.name, runtime.PersistedSnapshot().Inventory)
			}
		})
	}
}

func TestRuntimeUseItemOnItemRejectsOverMaxTargetStackWithoutMutatingState(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 2, Slot: 5}, {ID: 12, Vnum: 27001, Count: 201, Slot: 6}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")
	template.MaxCount = 200

	if _, ok := runtime.UseItemOnItem(5, 6, template); ok {
		t.Fatal("expected use-to-item to reject target stacks that already exceed template max_count")
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got.Inventory, persisted.Inventory) || got.Points[1] != 700 {
		t.Fatalf("expected rejected over-max target use-to-item to leave live state unchanged, got %#v points[1]=%d", got.Inventory, got.Points[1])
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected rejected over-max target use-to-item to leave persisted inventory unchanged, got %#v", runtime.PersistedSnapshot().Inventory)
	}
}

func TestRuntimeItemUseResolvesPointEffectFromTemplateMetadata(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Points: [255]int32{
			1: 700,
		},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27002, Count: 3, Slot: 5}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.UseItem(5, bootstrapConsumableTemplate(27002, 7, 1, 25, "consume:27002:+25"))
	if !ok {
		t.Fatal("expected template-defined consumable use to succeed")
	}
	if result.ItemRemoved {
		t.Fatal("expected stacked template-defined consumable to remain in inventory after one use")
	}
	if result.PointType != 7 || result.PointAmount != 25 || result.PointValue != 725 {
		t.Fatalf("expected template-defined point change, got %+v", result)
	}
	if result.EffectMessage != "consume:27002:+25" {
		t.Fatalf("unexpected template-defined effect message: %q", result.EffectMessage)
	}
	if result.Item.ID != 11 || result.Item.Vnum != 27002 || result.Item.Count != 2 || result.Item.Slot != 5 {
		t.Fatalf("unexpected updated template-defined inventory item: %+v", result.Item)
	}
	if got := runtime.PersistedSnapshot().Points[1]; got != 700 {
		t.Fatalf("expected persisted points to remain unchanged, got %d", got)
	}
	live := runtime.LiveCharacter()
	if live.Points[1] != 725 {
		t.Fatalf("expected live points[1] to follow template-defined delta, got %d", live.Points[1])
	}
	if !reflect.DeepEqual(live.Inventory, []inventory.ItemInstance{{ID: 11, Vnum: 27002, Count: 2, Slot: 5}}) {
		t.Fatalf("unexpected live inventory after template-defined use: %#v", live.Inventory)
	}
}

func TestRuntimePickupGroundItemDistributesStackAcrossCompatibleStacksBeforeFreshSlot(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Inventory: []inventory.ItemInstance{
			{ID: 11, Vnum: 27001, Count: 198, Slot: 0},
			{ID: 12, Vnum: 27001, Count: 199, Slot: 2},
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.PickupGroundItem(inventory.ItemInstance{ID: 13, Vnum: 27001, Count: 5, Slot: 6}, 6, 200)
	if !ok {
		t.Fatal("expected pickup to fill compatible stacks and place the remainder")
	}
	if !result.Split || result.Merged || len(result.UpdatedItems) != 2 {
		t.Fatalf("expected split pickup result with two updated stacks, got %+v", result)
	}
	wantUpdated := []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 200, Slot: 0},
		{ID: 12, Vnum: 27001, Count: 200, Slot: 2},
	}
	if !reflect.DeepEqual(result.UpdatedItems, wantUpdated) {
		t.Fatalf("unexpected split pickup updated stacks: got %#v want %#v", result.UpdatedItems, wantUpdated)
	}
	if result.Placed != (inventory.ItemInstance{ID: 13, Vnum: 27001, Count: 2, Slot: 6}) {
		t.Fatalf("unexpected split pickup remainder placement: %+v", result.Placed)
	}
	wantLive := []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 200, Slot: 0},
		{ID: 12, Vnum: 27001, Count: 200, Slot: 2},
		{ID: 13, Vnum: 27001, Count: 2, Slot: 6},
	}
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, wantLive) {
		t.Fatalf("unexpected live inventory after split pickup: got %#v want %#v", got, wantLive)
	}
}

func TestRuntimePickupGroundItemMergesIntoCompatibleStackBeforeFreshSlot(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
	}
	persisted.Inventory = []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 4, Slot: 0}}
	persisted.Inventory = append(persisted.Inventory, occupiedInventorySlotsExcept(0, 6)...)
	persisted.Inventory = append(persisted.Inventory, inventory.ItemInstance{ID: 12, Vnum: 27002, Count: 1, Slot: 6})
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.PickupGroundItem(inventory.ItemInstance{ID: 13, Vnum: 27001, Count: 3, Slot: 6}, 6, 200)
	if !ok {
		t.Fatal("expected pickup to merge into the compatible stack")
	}
	if !result.Merged || result.Updated.Slot != 0 || result.Updated.Count != 7 || result.Placed.ID != 0 {
		t.Fatalf("unexpected merge pickup result: %+v", result)
	}
	inventoryItems := runtime.LiveInventory()
	if got := inventoryItems[0]; got.Slot != 0 || got.Count != 7 {
		t.Fatalf("expected slot 0 stack count 7 after pickup, got %+v", got)
	}
	if len(inventoryItems) != len(persisted.Inventory) {
		t.Fatalf("expected no fresh slot placement, got inventory len %d want %d", len(inventoryItems), len(persisted.Inventory))
	}
}

func TestRuntimePickupGroundItemRejectsDuplicateMergeSlotOccupancyWithoutMutation(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
	}
	persisted.Inventory = []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 4, Slot: 0},
		{ID: 12, Vnum: 27001, Count: 5, Slot: 0},
	}
	persisted.Inventory = append(persisted.Inventory, occupiedInventorySlotsExcept(0, 6)...)
	persisted.Inventory = append(persisted.Inventory, inventory.ItemInstance{ID: 13, Vnum: 27002, Count: 1, Slot: 6})
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	before := runtime.LiveCharacter()

	if result, ok := runtime.PickupGroundItem(inventory.ItemInstance{ID: 14, Vnum: 27001, Count: 3, Slot: 6}, 6, 200); ok {
		t.Fatalf("expected duplicate merge-slot pickup to fail closed, got %+v", result)
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
		t.Fatalf("duplicate merge-slot pickup mutated live state:\ngot:  %+v\nwant: %+v", got, before)
	}
}

func TestRuntimePickupGroundItemSkipsLockedCompatibleStacks(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Inventory: []inventory.ItemInstance{
			{ID: 11, Vnum: 27001, Count: 4, Slot: 0, Locked: true},
			{ID: 12, Vnum: 27001, Count: 6, Slot: 2},
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.PickupGroundItem(inventory.ItemInstance{ID: 13, Vnum: 27001, Count: 3, Slot: 6}, 6, 200)
	if !ok {
		t.Fatal("expected pickup to skip locked stack and merge into unlocked compatible stack")
	}
	if !result.Merged || result.Split || result.Placed.ID != 0 {
		t.Fatalf("expected locked-stack pickup to remain a pure merge, got %+v", result)
	}
	if result.Updated != (inventory.ItemInstance{ID: 12, Vnum: 27001, Count: 9, Slot: 2}) {
		t.Fatalf("unexpected locked-stack pickup merge result: %+v", result.Updated)
	}
	wantLive := []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 4, Slot: 0, Locked: true},
		{ID: 12, Vnum: 27001, Count: 9, Slot: 2},
	}
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, wantLive) {
		t.Fatalf("unexpected live inventory after locked-stack pickup: got %#v want %#v", got, wantLive)
	}
}

func TestRuntimePickupGroundItemFailsWhenOnlyCompatibleCapacityIsLocked(t *testing.T) {
	persisted := loginticket.Character{ID: 0x01030102, VID: 0x02040102, Name: "PeerTwo"}
	persisted.Inventory = []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 4, Slot: 0, Locked: true}}
	for slot := inventory.SlotIndex(1); slot < inventory.CarriedInventorySlotCount; slot++ {
		persisted.Inventory = append(persisted.Inventory, inventory.ItemInstance{ID: uint64(1000 + slot), Vnum: 28000 + uint32(slot), Count: 1, Slot: slot})
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	if result, ok := runtime.PickupGroundItem(inventory.ItemInstance{ID: 13, Vnum: 27001, Count: 3, Slot: 6}, 6, 200); ok {
		t.Fatalf("expected pickup to fail when only compatible capacity is locked and no fresh slot exists, got %+v", result)
	}
	want := append([]inventory.ItemInstance(nil), persisted.Inventory...)
	sortInventoryItems(want)
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected failed locked-stack pickup to leave live inventory unchanged, got %#v want %#v", got, want)
	}
}

func TestRuntimePickupGroundItemFailsWhenPartialStacksNeedRemainderButNoFreshSlot(t *testing.T) {
	persisted := loginticket.Character{ID: 0x01030102, VID: 0x02040102, Name: "PeerTwo"}
	persisted.Inventory = []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 198, Slot: 0},
		{ID: 12, Vnum: 27001, Count: 199, Slot: 2},
	}
	for slot := inventory.SlotIndex(1); slot < inventory.CarriedInventorySlotCount; slot++ {
		if slot == 2 {
			continue
		}
		persisted.Inventory = append(persisted.Inventory, inventory.ItemInstance{ID: uint64(1000 + slot), Vnum: 28000 + uint32(slot), Count: 1, Slot: slot})
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	if result, ok := runtime.PickupGroundItem(inventory.ItemInstance{ID: 13, Vnum: 27001, Count: 5, Slot: 6}, 6, 200); ok {
		t.Fatalf("expected split pickup with no fresh remainder slot to fail closed, got %+v", result)
	}
	want := append([]inventory.ItemInstance(nil), persisted.Inventory...)
	sortInventoryItems(want)
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected failed split pickup to leave live inventory unchanged, got %#v want %#v", got, want)
	}
}

func TestRuntimeApplyEquipTemplateEffectAdjustsLivePointsWithoutMutatingPersistedSnapshot(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Points: [255]int32{
			1: 700,
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.ApplyEquipTemplateEffect(bootstrapEquipmentPointTemplate(12200, inventory.EquipmentSlotWeapon, 1, 1, 10), inventory.EquipmentSlotWeapon)
	if !ok {
		t.Fatal("expected equip template effect to succeed")
	}
	if result.PointType != 1 || result.PointAmount != 10 || result.PointValue != 710 {
		t.Fatalf("unexpected equip point change result: %+v", result)
	}
	if got := runtime.PersistedSnapshot().Points[1]; got != 700 {
		t.Fatalf("expected persisted points to remain unchanged, got %d", got)
	}
	if got := runtime.LiveCharacter().Points[1]; got != 710 {
		t.Fatalf("expected live points[1] to be incremented to 710, got %d", got)
	}
}

func TestRuntimeEquipTemplateEffectRejectsAuthoredSlotMismatchWithoutMutatingLivePoints(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Points: [255]int32{
			1: 700,
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := bootstrapEquipmentPointTemplate(12200, inventory.EquipmentSlotWeapon, 1, 1, 10)

	if _, ok := runtime.ApplyEquipTemplateEffect(template, inventory.EquipmentSlotBody); ok {
		t.Fatal("expected equip effect with mismatched authored slot to fail closed")
	}
	if got := runtime.LiveCharacter().Points[1]; got != 700 {
		t.Fatalf("mismatched equip effect mutated live points: got %d want 700", got)
	}
	if _, ok := runtime.RemoveEquipTemplateEffect(template, inventory.EquipmentSlotBody); ok {
		t.Fatal("expected equip effect removal with mismatched authored slot to fail closed")
	}
	if got := runtime.LiveCharacter().Points[1]; got != 700 {
		t.Fatalf("mismatched equip effect removal mutated live points: got %d want 700", got)
	}
}

func TestRuntimeApplyEquipTemplateEffectRejectsPointOverflowWithoutMutation(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Points: [255]int32{
			1: 1<<31 - 5,
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	before := runtime.LiveCharacter()

	if result, ok := runtime.ApplyEquipTemplateEffect(bootstrapEquipmentPointTemplate(12200, inventory.EquipmentSlotWeapon, 1, 1, 10), inventory.EquipmentSlotWeapon); ok {
		t.Fatalf("expected overflowing equip template effect to fail closed, got %+v", result)
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
		t.Fatalf("overflowing equip template effect mutated live character: got %#v want %#v", got, before)
	}
}

func occupiedInventorySlotsExcept(skip ...inventory.SlotIndex) []inventory.ItemInstance {
	skipped := make(map[inventory.SlotIndex]bool, len(skip))
	for _, slot := range skip {
		skipped[slot] = true
	}
	items := make([]inventory.ItemInstance, 0, int(inventory.CarriedInventorySlotCount)-len(skipped))
	var id uint64 = 1000
	for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
		if skipped[slot] {
			continue
		}
		items = append(items, inventory.ItemInstance{ID: id, Vnum: 28000 + uint32(slot), Count: 1, Slot: slot})
		id++
	}
	return items
}

func TestRuntimeApplyPointDeltaAdjustsLivePointsWithoutMutatingPersistedSnapshot(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Points: [255]int32{
			1: 700,
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.ApplyPointDelta(1, 1, -1)
	if !ok {
		t.Fatal("expected point delta application to succeed")
	}
	if result.PointType != 1 || result.PointAmount != -1 || result.PointValue != 699 {
		t.Fatalf("unexpected point delta result: %+v", result)
	}
	if got := runtime.PersistedSnapshot().Points[1]; got != 700 {
		t.Fatalf("expected persisted points to remain unchanged, got %d", got)
	}
	if got := runtime.LiveCharacter().Points[1]; got != 699 {
		t.Fatalf("expected live points[1] to be decremented to 699, got %d", got)
	}
}

func TestRuntimeRemoveEquipTemplateEffectRevertsLivePointsWithoutMutatingPersistedSnapshot(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Points: [255]int32{
			1: 700,
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	if _, ok := runtime.ApplyEquipTemplateEffect(bootstrapEquipmentPointTemplate(12200, inventory.EquipmentSlotWeapon, 1, 1, 10), inventory.EquipmentSlotWeapon); !ok {
		t.Fatal("expected template-backed equip effect application to succeed before removal")
	}
	result, ok := runtime.RemoveEquipTemplateEffect(bootstrapEquipmentPointTemplate(12200, inventory.EquipmentSlotWeapon, 1, 1, 10), inventory.EquipmentSlotWeapon)
	if !ok {
		t.Fatal("expected template-backed equip effect removal to succeed")
	}
	if result.PointType != 1 || result.PointAmount != -10 || result.PointValue != 700 {
		t.Fatalf("unexpected equip-derived point removal result: %+v", result)
	}
	if got := runtime.PersistedSnapshot().Points[1]; got != 700 {
		t.Fatalf("expected persisted points to remain unchanged, got %d", got)
	}
	if got := runtime.LiveCharacter().Points[1]; got != 700 {
		t.Fatalf("expected live points[1] to be restored to 700, got %d", got)
	}
}
