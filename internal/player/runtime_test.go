package player

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
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

func TestNilRuntimeReturnsZeroLiveCharacter(t *testing.T) {
	var runtime *Runtime
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, loginticket.Character{}) {
		t.Fatalf("expected nil runtime to return zero live character, got %+v", got)
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

	result, ok := runtime.UseItem(5)
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

	result, ok := runtime.UseItem(5)
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
