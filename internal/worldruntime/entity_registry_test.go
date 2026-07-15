package worldruntime

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestRegisteredStaticActorCombatProfileAllowsExplicitFormulaWithoutLegacyDamage(t *testing.T) {
	const profile = "formula_only_profile"
	UnregisterStaticActorCombatProfileForTest(profile)
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	if !RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        10,
		AttackValue:  7,
		DefenseValue: 4,
		RespawnDelay: time.Second,
	}) {
		t.Fatal("expected explicit attack/defense profile to register without legacy damage_per_normal_attack")
	}

	defaults, ok := BootstrapStaticActorCombatProfileDefaults(profile)
	if !ok {
		t.Fatal("expected registered combat profile defaults")
	}
	if defaults.DamagePerNormalAttack != 3 {
		t.Fatalf("expected legacy damage fallback to be canonicalized from formula, got %d", defaults.DamagePerNormalAttack)
	}
	if nextHP, hpPercent, ok := ApplyBootstrapStaticActorNormalAttack(profile, 10); !ok || nextHP != 7 || hpPercent != 70 {
		t.Fatalf("expected formula-only profile to apply 3 damage, got nextHP=%d hpPercent=%d ok=%v", nextHP, hpPercent, ok)
	}
}

func TestEntityRegistryRegistersLooksUpUpdatesAndRemovesPlayerEntities(t *testing.T) {
	registry := NewEntityRegistry()
	alpha := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100))
	if alpha.Entity.Kind != EntityKindPlayer || alpha.Entity.ID == 0 || alpha.Entity.VID != 0x02040101 || alpha.Entity.Name != "Alpha" {
		t.Fatalf("unexpected registered player entity: %+v", alpha)
	}

	lookup, ok := registry.Player(alpha.Entity.ID)
	if !ok || lookup.Entity.Name != "Alpha" || lookup.Character.X != 1100 || lookup.Character.Y != 2100 {
		t.Fatalf("expected lookup to return registered player entity, got entity=%+v ok=%v", lookup, ok)
	}

	updated := lookup.Character
	updated.MapIndex = 42
	updated.X = 1700
	updated.Y = 2800
	if !registry.UpdatePlayer(alpha.Entity.ID, updated) {
		t.Fatal("expected player update to succeed")
	}
	lookup, ok = registry.Player(alpha.Entity.ID)
	if !ok || lookup.Character.MapIndex != 42 || lookup.Character.X != 1700 || lookup.Character.Y != 2800 {
		t.Fatalf("expected updated player entity snapshot, got entity=%+v ok=%v", lookup, ok)
	}

	removed, ok := registry.Remove(alpha.Entity.ID)
	if !ok || removed.Entity.ID != alpha.Entity.ID {
		t.Fatalf("expected remove to return registered entity, got entity=%+v ok=%v", removed, ok)
	}
	if _, ok := registry.Player(alpha.Entity.ID); ok {
		t.Fatal("expected removed player entity to disappear from lookup")
	}
}

func TestEntityRegistryLooksUpPlayersByVIDAndExactName(t *testing.T) {
	registry := NewEntityRegistry()
	alpha := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100))
	bravo := registry.RegisterPlayer(entityRegistryCharacter("Bravo", 0x02040102, 42, 1300, 2300))

	byVID, ok := registry.PlayerByVID(bravo.Entity.VID)
	if !ok || byVID.Entity.ID != bravo.Entity.ID || byVID.Entity.Name != "Bravo" {
		t.Fatalf("expected VID lookup to return Bravo, got entity=%+v ok=%v", byVID, ok)
	}
	byName, ok := registry.PlayerByName(alpha.Entity.Name)
	if !ok || byName.Entity.ID != alpha.Entity.ID || byName.Entity.VID != alpha.Entity.VID {
		t.Fatalf("expected exact-name lookup to return Alpha, got entity=%+v ok=%v", byName, ok)
	}
}

func TestEntityRegistryRejectedPlayerRegistrationDoesNotConsumeEntityID(t *testing.T) {
	registry := NewEntityRegistry()
	alpha := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100))
	if alpha.Entity.ID != 1 {
		t.Fatalf("expected first player entity ID 1, got %+v", alpha)
	}

	duplicateName := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040102, 1, 1200, 2200))
	if duplicateName.Entity.ID != 0 {
		t.Fatalf("expected duplicate-name registration to fail closed, got %+v", duplicateName)
	}

	bravo := registry.RegisterPlayer(entityRegistryCharacter("Bravo", 0x02040102, 1, 1300, 2300))
	if bravo.Entity.ID != 2 {
		t.Fatalf("expected failed registration not to consume entity ID; got Bravo entity %+v", bravo)
	}
	if next := registry.NextEntityID(); next != 3 {
		t.Fatalf("expected next entity ID 3 after two successful players, got %d", next)
	}
}

func TestEntityRegistryRejectedStaticActorRegistrationDoesNotConsumeEntityID(t *testing.T) {
	registry := NewEntityRegistry()
	alpha := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100))
	if alpha.Entity.ID != 1 {
		t.Fatalf("expected first player entity ID 1, got %+v", alpha)
	}

	collidingActor, ok := registry.RegisterStaticActorWithID(StaticEntity{Entity: Entity{ID: uint64(alpha.Entity.VID), Name: "CollidingGuard"}, Position: NewPosition(1, 1200, 2200), RaceNum: 20300})
	if ok {
		t.Fatalf("expected static actor registration with player visible VID collision to fail closed, got %+v", collidingActor)
	}

	bravo := registry.RegisterPlayer(entityRegistryCharacter("Bravo", 0x02040102, 1, 1300, 2300))
	if bravo.Entity.ID != 2 {
		t.Fatalf("expected failed static actor registration not to consume entity ID; got Bravo entity %+v", bravo)
	}
	if next := registry.NextEntityID(); next != 3 {
		t.Fatalf("expected next entity ID 3 after one rejected static actor registration, got %d", next)
	}
}

func TestEntityRegistryRejectsStaticActorIDCollisionWithRegisteredPlayer(t *testing.T) {
	registry := NewEntityRegistry()
	player := registry.RegisterPlayer(entityRegistryCharacter("CollisionOwner", 0x02040177, 1, 1100, 2100))
	if player.Entity.ID == 0 {
		t.Fatal("expected player registration to succeed before collision attempt")
	}

	actor, ok := registry.RegisterStaticActorWithID(StaticEntity{
		Entity:        Entity{ID: player.Entity.ID, Name: "CollisionMob"},
		Position:      NewPosition(1, 1200, 2200),
		RaceNum:       20300,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.collision_mob",
	})
	if ok {
		t.Fatalf("expected static actor registration with player entity ID to fail closed, got %+v", actor)
	}
	if _, ok := registry.StaticActor(player.Entity.ID); ok {
		t.Fatalf("expected no static actor to be registered with player entity ID %d", player.Entity.ID)
	}
	lookup, ok := registry.Player(player.Entity.ID)
	if !ok || lookup.Entity.Name != player.Entity.Name || lookup.Character.VID != player.Character.VID {
		t.Fatalf("expected player entity to remain intact after rejected collision, got entity=%+v ok=%v", lookup, ok)
	}
	characters := registry.MapCharacters(player.Character.MapIndex)
	if len(characters) != 1 || characters[0].Name != player.Character.Name {
		t.Fatalf("expected player map occupancy to remain intact after rejected collision, got %+v", characters)
	}
}

func TestEntityRegistryReturnsDeterministicSortedPlayerCharacters(t *testing.T) {
	registry := NewEntityRegistry()
	registry.RegisterPlayer(entityRegistryCharacter("Zulu", 0x02040103, 42, 1900, 3000))
	registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100))
	registry.RegisterPlayer(entityRegistryCharacter("Bravo", 0x02040102, 1, 1300, 2300))

	characters := registry.PlayerCharacters()
	if len(characters) != 3 {
		t.Fatalf("expected 3 player characters, got %d", len(characters))
	}
	if characters[0].Name != "Alpha" || characters[1].Name != "Bravo" || characters[2].Name != "Zulu" {
		t.Fatalf("expected deterministic player character order, got %+v", characters)
	}
}

func TestEntityRegistryTracksPlayersByEffectiveMapIndex(t *testing.T) {
	registry := NewEntityRegistry()
	alpha := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 0, 1100, 2100))
	bravo := registry.RegisterPlayer(entityRegistryCharacter("Bravo", 0x02040102, 42, 1300, 2300))

	bootstrapCharacters := registry.MapCharacters(0)
	if len(bootstrapCharacters) != 1 || bootstrapCharacters[0].Name != "Alpha" {
		t.Fatalf("expected Alpha in effective bootstrap map bucket, got %+v", bootstrapCharacters)
	}
	map42Characters := registry.MapCharacters(42)
	if len(map42Characters) != 1 || map42Characters[0].Name != "Bravo" {
		t.Fatalf("expected Bravo in map 42 bucket, got %+v", map42Characters)
	}

	moved := alpha.Character
	moved.MapIndex = 42
	moved.X = 1700
	moved.Y = 2800
	if !registry.UpdatePlayer(alpha.Entity.ID, moved) {
		t.Fatal("expected Alpha move to update the map index")
	}
	map42Characters = registry.MapCharacters(42)
	if len(map42Characters) != 2 || map42Characters[0].Name != "Alpha" || map42Characters[1].Name != "Bravo" {
		t.Fatalf("expected Alpha and Bravo in sorted map 42 bucket after relocation, got %+v", map42Characters)
	}

	removed, ok := registry.Remove(bravo.Entity.ID)
	if !ok || removed.Entity.ID != bravo.Entity.ID {
		t.Fatalf("expected remove to return Bravo, got entity=%+v ok=%v", removed, ok)
	}
	map42Characters = registry.MapCharacters(42)
	if len(map42Characters) != 1 || map42Characters[0].Name != "Alpha" {
		t.Fatalf("expected only Alpha in map 42 bucket after removal, got %+v", map42Characters)
	}

	occupancy := registry.MapOccupancy()
	if len(occupancy) != 1 || occupancy[0].MapIndex != 42 || len(occupancy[0].Characters) != 1 || occupancy[0].Characters[0].Name != "Alpha" {
		t.Fatalf("expected deterministic map-occupancy snapshot after removal, got %+v", occupancy)
	}
}

func TestEntityRegistryRemoveClearsMapOccupancyWhenPlayerDirectoryEntryAlreadyMissing(t *testing.T) {
	registry := NewEntityRegistry()
	alpha := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 42, 1700, 2800))
	if alpha.Entity.ID == 0 {
		t.Fatal("expected player registration to succeed")
	}
	if _, ok := registry.players.Remove(alpha.Entity.ID); !ok {
		t.Fatal("expected direct player-directory removal to simulate partial teardown")
	}

	removed, ok := registry.Remove(alpha.Entity.ID)
	if !ok || removed.Entity.ID != alpha.Entity.ID {
		t.Fatalf("expected entity removal to still succeed after player-directory loss, got entity=%+v ok=%v", removed, ok)
	}
	if mapCharacters := registry.MapCharacters(42); len(mapCharacters) != 0 {
		t.Fatalf("expected map occupancy to be cleared after tolerant remove, got %+v", mapCharacters)
	}
	if occupancy := registry.MapOccupancy(); len(occupancy) != 0 {
		t.Fatalf("expected no map-occupancy snapshots after tolerant remove, got %+v", occupancy)
	}
}

func TestEntityRegistryRemoveClearsPlayerLookupWhenMapIndexEntryAlreadyMissing(t *testing.T) {
	registry := NewEntityRegistry()
	alpha := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 42, 1700, 2800))
	if alpha.Entity.ID == 0 {
		t.Fatal("expected player registration to succeed")
	}
	if _, ok := registry.maps.Remove(alpha.Entity.ID); !ok {
		t.Fatal("expected direct map-index removal to simulate partial teardown")
	}

	removed, ok := registry.Remove(alpha.Entity.ID)
	if !ok || removed.Entity.ID != alpha.Entity.ID {
		t.Fatalf("expected entity removal to still succeed after map-index loss, got entity=%+v ok=%v", removed, ok)
	}
	if _, ok := registry.Player(alpha.Entity.ID); ok {
		t.Fatal("expected player lookup to be cleared after tolerant remove")
	}
	if _, ok := registry.PlayerByVID(alpha.Entity.VID); ok {
		t.Fatal("expected player VID lookup to be cleared after tolerant remove")
	}
	if occupancy := registry.MapOccupancy(); len(occupancy) != 0 {
		t.Fatalf("expected no map-occupancy snapshots after tolerant remove, got %+v", occupancy)
	}
}

func TestEntityRegistryRegistersAndLooksUpStaticActors(t *testing.T) {
	registry := NewEntityRegistry()
	registered, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300})
	if !ok {
		t.Fatal("expected static actor registration to succeed")
	}
	if registered.Entity.ID == 0 || registered.Entity.Kind != EntityKindStaticActor || registered.Entity.Name != "VillageGuard" {
		t.Fatalf("unexpected registered static actor: %+v", registered)
	}
	lookup, ok := registry.StaticActor(registered.Entity.ID)
	if !ok || lookup.Entity.ID != registered.Entity.ID || lookup.Position != registered.Position || lookup.RaceNum != 20300 {
		t.Fatalf("expected static actor lookup to round-trip, got actor=%+v ok=%v", lookup, ok)
	}
	actors := registry.StaticActors(42)
	if len(actors) != 1 || actors[0].Entity.ID != registered.Entity.ID {
		t.Fatalf("expected static actor map snapshot for map 42, got %+v", actors)
	}
}

func TestEntityRegistryRejectsUnencodableStaticActorRaceNum(t *testing.T) {
	registry := NewEntityRegistry()
	actor, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "InvisibleOverflow"}, Position: NewPosition(42, 1700, 2800), RaceNum: uint32(^uint16(0)) + 1})
	if ok {
		t.Fatalf("expected unencodable static actor race_num to fail closed, got %+v", actor)
	}
	if actor, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "ZeroRace"}, Position: NewPosition(42, 1800, 2900), RaceNum: 0}); ok {
		t.Fatalf("expected zero static actor race_num to fail closed, got %+v", actor)
	}
	if next := registry.NextEntityID(); next != 1 {
		t.Fatalf("expected rejected static actor registration not to consume an entity ID, got next=%d", next)
	}
	if actors := registry.StaticActors(42); len(actors) != 0 {
		t.Fatalf("expected rejected static actors not to enter map occupancy, got %+v", actors)
	}
}

func TestEntityRegistryRejectsStaticActorUpdateToUnencodableRaceNum(t *testing.T) {
	registry := NewEntityRegistry()
	guard, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300})
	if !ok {
		t.Fatal("expected static actor registration to succeed")
	}

	updated := guard
	updated.RaceNum = uint32(^uint16(0)) + 1
	if actor, ok := registry.UpdateStaticActor(updated); ok {
		t.Fatalf("expected unencodable static actor race_num update to fail closed, got %+v", actor)
	}
	lookup, ok := registry.StaticActor(guard.Entity.ID)
	if !ok || lookup.RaceNum != 20300 || lookup.Entity.Name != "VillageGuard" {
		t.Fatalf("expected failed race_num update to preserve original actor, got actor=%+v ok=%v", lookup, ok)
	}
	if actors := registry.StaticActors(42); len(actors) != 1 || actors[0].RaceNum != 20300 {
		t.Fatalf("expected failed race_num update to preserve map occupancy, got %+v", actors)
	}
}

func TestEntityRegistryRejectsExplicitStaticActorIDAlreadyOwnedByPlayer(t *testing.T) {
	registry := NewEntityRegistry()
	player := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 42, 1700, 2800))
	if player.Entity.ID == 0 {
		t.Fatal("expected player registration to succeed")
	}

	if actor, ok := registry.RegisterStaticActorWithID(StaticEntity{Entity: Entity{ID: player.Entity.ID, Name: "CollidingGuard"}, Position: NewPosition(42, 1800, 2900), RaceNum: 20300}); ok {
		t.Fatalf("expected explicit static actor ID colliding with player to fail closed, got %+v", actor)
	}
	if lookup, ok := registry.Player(player.Entity.ID); !ok || lookup.Entity.Name != "Alpha" {
		t.Fatalf("expected original player entity to remain registered after rejected static actor, got entity=%+v ok=%v", lookup, ok)
	}
	if actor, ok := registry.StaticActor(uint64(player.Entity.ID)); ok {
		t.Fatalf("expected colliding static actor to stay absent, got %+v", actor)
	}
}

func TestEntityRegistryRejectsExplicitStaticActorIDAlreadyOwnedByMapOnlyPlayer(t *testing.T) {
	registry := NewEntityRegistry()
	player := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 42, 1700, 2800))
	if player.Entity.ID == 0 {
		t.Fatal("expected player registration to succeed")
	}
	if _, ok := registry.players.Remove(player.Entity.ID); !ok {
		t.Fatal("expected direct player-directory removal to simulate partial index loss")
	}

	if actor, ok := registry.RegisterStaticActorWithID(StaticEntity{Entity: Entity{ID: player.Entity.ID, Name: "CollidingGuard"}, Position: NewPosition(42, 1800, 2900), RaceNum: 20300}); ok {
		t.Fatalf("expected explicit static actor ID colliding with map-only player to fail closed, got %+v", actor)
	}
	lookup, ok := registry.maps.Remove(player.Entity.ID)
	if !ok || lookup.Entity.Name != "Alpha" {
		t.Fatalf("expected map-only player presence to remain after rejected static actor, got entity=%+v ok=%v", lookup, ok)
	}
	if actor, ok := registry.StaticActor(player.Entity.ID); ok {
		t.Fatalf("expected colliding static actor to stay absent, got %+v", actor)
	}
}

func TestEntityRegistryStaticActorsPreserveInteractionMetadata(t *testing.T) {
	registry := NewEntityRegistry()
	registered, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300, InteractionKind: "talk", InteractionRef: "npc:village_guard"})
	if !ok {
		t.Fatal("expected static actor registration with interaction metadata to succeed")
	}
	lookup, ok := registry.StaticActor(registered.Entity.ID)
	if !ok {
		t.Fatal("expected static actor lookup to succeed")
	}
	if lookup.InteractionKind != "talk" || lookup.InteractionRef != "npc:village_guard" {
		t.Fatalf("expected interaction metadata to round-trip through registry lookup, got %+v", lookup)
	}

	updated := lookup
	updated.InteractionKind = "info"
	updated.InteractionRef = "lore:village_guard"
	result, ok := registry.UpdateStaticActor(updated)
	if !ok {
		t.Fatal("expected static actor update with interaction metadata to succeed")
	}
	if result.InteractionKind != "info" || result.InteractionRef != "lore:village_guard" {
		t.Fatalf("expected updated interaction metadata in result, got %+v", result)
	}
	actors := registry.AllStaticActors()
	if len(actors) != 1 || actors[0].InteractionKind != "info" || actors[0].InteractionRef != "lore:village_guard" {
		t.Fatalf("expected interaction metadata in static actor snapshot, got %+v", actors)
	}
}

func TestEntityRegistryLooksUpStaticActorsByVisibilityVID(t *testing.T) {
	registry := NewEntityRegistry()
	registered, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300})
	if !ok {
		t.Fatal("expected static actor registration to succeed")
	}

	lookup, ok := registry.StaticActorByVID(uint32(registered.Entity.ID))
	if !ok || lookup.Entity.ID != registered.Entity.ID || lookup.Entity.Name != registered.Entity.Name {
		t.Fatalf("expected static actor VID lookup to return VillageGuard, got actor=%+v ok=%v", lookup, ok)
	}
	if _, ok := registry.StaticActorByVID(999); ok {
		t.Fatal("expected missing static actor VID lookup to fail")
	}
}

func TestEntityRegistryReturnsDeterministicSortedStaticActors(t *testing.T) {
	registry := NewEntityRegistry()
	guard, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300})
	if !ok {
		t.Fatal("expected guard registration to succeed")
	}
	blacksmith, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "Blacksmith"}, Position: NewPosition(42, 1900, 3000), RaceNum: 20301})
	if !ok {
		t.Fatal("expected blacksmith registration to succeed")
	}

	actors := registry.AllStaticActors()
	if len(actors) != 2 {
		t.Fatalf("expected 2 static actors in registry snapshot, got %d", len(actors))
	}
	if actors[0].Entity.ID != blacksmith.Entity.ID || actors[0].Entity.Name != "Blacksmith" {
		t.Fatalf("expected Blacksmith first in sorted static actor snapshot, got %+v", actors[0])
	}
	if actors[1].Entity.ID != guard.Entity.ID || actors[1].Entity.Name != "VillageGuard" {
		t.Fatalf("expected VillageGuard second in sorted static actor snapshot, got %+v", actors[1])
	}
}

func TestEntityRegistryRemoveStaticActorClearsLookupAndMapPresence(t *testing.T) {
	registry := NewEntityRegistry()
	guard, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300})
	if !ok {
		t.Fatal("expected guard registration to succeed")
	}
	removed, ok := registry.RemoveStaticActor(guard.Entity.ID)
	if !ok || removed.Entity.ID != guard.Entity.ID {
		t.Fatalf("expected static actor removal to return guard, got actor=%+v ok=%v", removed, ok)
	}
	if _, ok := registry.StaticActor(guard.Entity.ID); ok {
		t.Fatal("expected static actor lookup to be cleared after removal")
	}
	if actors := registry.StaticActors(42); len(actors) != 0 {
		t.Fatalf("expected map static actor snapshot to be empty after removal, got %+v", actors)
	}
	if actors := registry.AllStaticActors(); len(actors) != 0 {
		t.Fatalf("expected global static actor snapshot to be empty after removal, got %+v", actors)
	}
}

func TestEntityRegistryRemoveStaticActorClearsMapPresenceWhenDirectoryEntryAlreadyMissing(t *testing.T) {
	registry := NewEntityRegistry()
	guard, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300})
	if !ok {
		t.Fatal("expected guard registration to succeed")
	}
	if _, ok := registry.staticActors.Remove(guard.Entity.ID); !ok {
		t.Fatal("expected direct non-player-directory removal to simulate partial teardown")
	}

	removed, ok := registry.RemoveStaticActor(guard.Entity.ID)
	if !ok || removed.Entity.ID != guard.Entity.ID {
		t.Fatalf("expected tolerant static actor removal after directory loss, got actor=%+v ok=%v", removed, ok)
	}
	if actors := registry.StaticActors(42); len(actors) != 0 {
		t.Fatalf("expected map static actor snapshot to be cleared after tolerant removal, got %+v", actors)
	}
	if occupancy := registry.MapOccupancy(); len(occupancy) != 0 {
		t.Fatalf("expected no map occupancy after tolerant static actor removal, got %+v", occupancy)
	}
}

func TestEntityRegistryRemoveStaticActorClearsDirectoryWhenMapIndexEntryAlreadyMissing(t *testing.T) {
	registry := NewEntityRegistry()
	guard, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300})
	if !ok {
		t.Fatal("expected guard registration to succeed")
	}
	if _, ok := registry.maps.RemoveStatic(guard.Entity.ID); !ok {
		t.Fatal("expected direct map-index removal to simulate partial teardown")
	}

	removed, ok := registry.RemoveStaticActor(guard.Entity.ID)
	if !ok || removed.Entity.ID != guard.Entity.ID {
		t.Fatalf("expected tolerant static actor removal after map-index loss, got actor=%+v ok=%v", removed, ok)
	}
	if _, ok := registry.StaticActor(guard.Entity.ID); ok {
		t.Fatal("expected static actor directory lookup to be cleared after tolerant removal")
	}
	if _, ok := registry.StaticActorByVID(uint32(guard.Entity.ID)); ok {
		t.Fatal("expected static actor VID lookup to be cleared after tolerant removal")
	}
	if actors := registry.StaticActors(42); len(actors) != 0 {
		t.Fatalf("expected map static actor snapshot to stay empty after tolerant removal, got %+v", actors)
	}
	if occupancy := registry.MapOccupancy(); len(occupancy) != 0 {
		t.Fatalf("expected no map occupancy after tolerant static actor removal, got %+v", occupancy)
	}
}

func TestEntityRegistryUpdateStaticActorUpdatesLookupAndMapPresence(t *testing.T) {
	registry := NewEntityRegistry()
	guard, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300})
	if !ok {
		t.Fatal("expected guard registration to succeed")
	}

	updated := guard
	updated.Entity.Name = "Blacksmith"
	updated.Position = NewPosition(99, 900, 1200)
	updated.RaceNum = 20016
	result, ok := registry.UpdateStaticActor(updated)
	if !ok {
		t.Fatal("expected static actor update to succeed")
	}
	if result.Entity.ID != guard.Entity.ID || result.Entity.Name != "Blacksmith" || result.Position != NewPosition(99, 900, 1200) || result.RaceNum != 20016 {
		t.Fatalf("unexpected updated static actor result: %+v", result)
	}
	lookup, ok := registry.StaticActor(guard.Entity.ID)
	if !ok || lookup.Entity.Name != "Blacksmith" || lookup.Position != NewPosition(99, 900, 1200) || lookup.RaceNum != 20016 {
		t.Fatalf("expected static actor lookup to reflect update, got actor=%+v ok=%v", lookup, ok)
	}
	if actors := registry.StaticActors(42); len(actors) != 0 {
		t.Fatalf("expected old map static actor snapshot to be empty after update, got %+v", actors)
	}
	actors := registry.StaticActors(99)
	if len(actors) != 1 || actors[0].Entity.ID != guard.Entity.ID || actors[0].Entity.Name != "Blacksmith" {
		t.Fatalf("expected updated static actor in map 99 snapshot, got %+v", actors)
	}
}

func TestEntityRegistryUpdateStaticActorPreservesDeathReward(t *testing.T) {
	registry := NewEntityRegistry()
	guard, ok := registry.RegisterStaticActor(StaticEntity{
		Entity:        Entity{Name: "RewardGuard"},
		Position:      NewPosition(42, 1700, 2800),
		RaceNum:       20300,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.reward_guard",
		DeathReward:   StaticActorDeathReward{Experience: 75, Gold: 60, DropVnums: []uint32{27001, 27002}},
	})
	if !ok {
		t.Fatal("expected guard registration to succeed")
	}

	updated := guard
	updated.Entity.Name = "RewardGuardMoved"
	updated.Position = NewPosition(99, 900, 1200)
	updated.RaceNum = 20016
	result, ok := registry.UpdateStaticActor(updated)
	if !ok {
		t.Fatal("expected static actor update to succeed")
	}
	if result.DeathReward.Experience != 75 || result.DeathReward.Gold != 60 || len(result.DeathReward.DropVnums) != 2 || result.DeathReward.DropVnums[0] != 27001 || result.DeathReward.DropVnums[1] != 27002 {
		t.Fatalf("expected death reward to survive static actor update, got %+v", result.DeathReward)
	}

	updated.DeathReward.DropVnums[0] = 99999
	lookup, ok := registry.StaticActor(guard.Entity.ID)
	if !ok {
		t.Fatal("expected updated static actor lookup to succeed")
	}
	if len(lookup.DeathReward.DropVnums) != 2 || lookup.DeathReward.DropVnums[0] != 27001 || lookup.DeathReward.DropVnums[1] != 27002 {
		t.Fatalf("expected stored death reward drops to be cloned on update, got %+v", lookup.DeathReward.DropVnums)
	}
}

func TestEntityRegistryRegisterStaticActorSortsDeathRewardDrops(t *testing.T) {
	registry := NewEntityRegistry()
	actor, ok := registry.RegisterStaticActor(StaticEntity{
		Entity:        Entity{Name: "SortedRewardGuard"},
		Position:      NewPosition(42, 1700, 2800),
		RaceNum:       20300,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.sorted_reward_guard",
		DeathReward:   StaticActorDeathReward{DropVnums: []uint32{27003, 27001, 27002}},
	})
	if !ok {
		t.Fatal("expected actor registration with unordered reward drops to succeed")
	}

	lookup, ok := registry.StaticActor(actor.Entity.ID)
	if !ok {
		t.Fatal("expected static actor lookup to succeed")
	}
	if len(lookup.DeathReward.DropVnums) != 3 || lookup.DeathReward.DropVnums[0] != 27001 || lookup.DeathReward.DropVnums[1] != 27002 || lookup.DeathReward.DropVnums[2] != 27003 {
		t.Fatalf("expected registered death reward drops to be sorted, got %+v", lookup.DeathReward.DropVnums)
	}
}

func TestEntityRegistryUpdateStaticActorSortsDeathRewardDrops(t *testing.T) {
	registry := NewEntityRegistry()
	actor, ok := registry.RegisterStaticActor(StaticEntity{
		Entity:        Entity{Name: "UpdatedRewardGuard"},
		Position:      NewPosition(42, 1700, 2800),
		RaceNum:       20300,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.updated_reward_guard",
		DeathReward:   StaticActorDeathReward{DropVnums: []uint32{27001}},
	})
	if !ok {
		t.Fatal("expected actor registration to succeed")
	}

	updated := actor
	updated.DeathReward = StaticActorDeathReward{DropVnums: []uint32{27003, 27001, 27002}}
	result, ok := registry.UpdateStaticActor(updated)
	if !ok {
		t.Fatal("expected actor update with unordered reward drops to succeed")
	}
	if len(result.DeathReward.DropVnums) != 3 || result.DeathReward.DropVnums[0] != 27001 || result.DeathReward.DropVnums[1] != 27002 || result.DeathReward.DropVnums[2] != 27003 {
		t.Fatalf("expected updated death reward drops to be sorted, got %+v", result.DeathReward.DropVnums)
	}
}

func TestEntityRegistryUpdateStaticActorRebuildsMissingMapPresence(t *testing.T) {
	registry := NewEntityRegistry()
	guard, ok := registry.RegisterStaticActor(StaticEntity{
		Entity:        Entity{Name: "MaplessRewardGuard"},
		Position:      NewPosition(42, 1700, 2800),
		RaceNum:       20300,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.mapless_reward_guard",
		DeathReward:   StaticActorDeathReward{Experience: 75, Gold: 60, DropVnums: []uint32{27001, 27002}},
	})
	if !ok {
		t.Fatal("expected guard registration to succeed")
	}

	registry.maps.mu.Lock()
	delete(registry.maps.staticByEntityID, guard.Entity.ID)
	for mapIndex, bucket := range registry.maps.staticByMapIndex {
		delete(bucket, guard.Entity.ID)
		if len(bucket) == 0 {
			delete(registry.maps.staticByMapIndex, mapIndex)
		}
	}
	registry.maps.mu.Unlock()

	updated := guard
	updated.Entity.Name = "MaplessRewardGuardMoved"
	updated.Position = NewPosition(99, 900, 1200)
	updated.RaceNum = 20016
	result, ok := registry.UpdateStaticActor(updated)
	if !ok {
		t.Fatal("expected static actor update to rebuild missing map presence")
	}
	if result.Entity.ID != guard.Entity.ID || result.Entity.Name != "MaplessRewardGuardMoved" || result.Position != NewPosition(99, 900, 1200) || result.RaceNum != 20016 {
		t.Fatalf("unexpected rebuilt static actor update result: %+v", result)
	}
	if result.DeathReward.Experience != 75 || result.DeathReward.Gold != 60 || len(result.DeathReward.DropVnums) != 2 || result.DeathReward.DropVnums[0] != 27001 || result.DeathReward.DropVnums[1] != 27002 {
		t.Fatalf("expected rebuilt map presence update to preserve death reward, got %+v", result.DeathReward)
	}
	actors := registry.StaticActors(99)
	if len(actors) != 1 || actors[0].Entity.ID != guard.Entity.ID || actors[0].Entity.Name != "MaplessRewardGuardMoved" {
		t.Fatalf("expected rebuilt static actor in map 99 snapshot, got %+v", actors)
	}
}

func TestEntityRegistryUpdateStaticActorRebuildsMissingDirectoryEntry(t *testing.T) {
	registry := NewEntityRegistry()
	guard, ok := registry.RegisterStaticActor(StaticEntity{
		Entity:        Entity{Name: "DirectorylessRewardGuard"},
		Position:      NewPosition(42, 1700, 2800),
		RaceNum:       20300,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.directoryless_reward_guard",
		DeathReward:   StaticActorDeathReward{Experience: 75, Gold: 60, DropVnums: []uint32{27001, 27002}},
	})
	if !ok {
		t.Fatal("expected guard registration to succeed")
	}
	if _, ok := registry.staticActors.Remove(guard.Entity.ID); !ok {
		t.Fatal("expected test setup to remove static actor directory entry")
	}

	updated := guard
	updated.Entity.Name = "DirectorylessRewardGuardMoved"
	updated.Position = NewPosition(99, 900, 1200)
	updated.RaceNum = 20016
	result, ok := registry.UpdateStaticActor(updated)
	if !ok {
		t.Fatal("expected static actor update to rebuild missing directory entry")
	}
	if result.Entity.ID != guard.Entity.ID || result.Entity.Name != "DirectorylessRewardGuardMoved" || result.Position != NewPosition(99, 900, 1200) || result.RaceNum != 20016 {
		t.Fatalf("unexpected rebuilt static actor update result: %+v", result)
	}
	if result.DeathReward.Experience != 75 || result.DeathReward.Gold != 60 || len(result.DeathReward.DropVnums) != 2 || result.DeathReward.DropVnums[0] != 27001 || result.DeathReward.DropVnums[1] != 27002 {
		t.Fatalf("expected rebuilt directory update to preserve death reward, got %+v", result.DeathReward)
	}
	lookup, ok := registry.StaticActor(guard.Entity.ID)
	if !ok || lookup.Entity.Name != "DirectorylessRewardGuardMoved" || lookup.Position != NewPosition(99, 900, 1200) || lookup.RaceNum != 20016 {
		t.Fatalf("expected rebuilt static actor directory lookup to reflect update, got actor=%+v ok=%v", lookup, ok)
	}
	if actors := registry.StaticActors(42); len(actors) != 0 {
		t.Fatalf("expected old map static actor snapshot to be empty after rebuilt directory update, got %+v", actors)
	}
	actors := registry.StaticActors(99)
	if len(actors) != 1 || actors[0].Entity.ID != guard.Entity.ID || actors[0].Entity.Name != "DirectorylessRewardGuardMoved" {
		t.Fatalf("expected updated static actor in map 99 snapshot, got %+v", actors)
	}
}

func TestEntityRegistryRejectsPlayerVIDAlreadyOwnedByStaticActorVisibilityVID(t *testing.T) {
	registry := NewEntityRegistry()
	actor, ok := registry.RegisterStaticActorWithID(StaticEntity{Entity: Entity{ID: 0x02040101, Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300})
	if !ok {
		t.Fatal("expected static actor registration with explicit visibility VID to succeed")
	}

	player := registry.RegisterPlayer(entityRegistryCharacter("Alpha", uint32(actor.Entity.ID), 42, 1800, 2900))
	if player.Entity.ID != 0 {
		t.Fatalf("expected player registration with colliding visible VID to fail closed, got %+v", player)
	}
	lookup, ok := registry.StaticActor(actor.Entity.ID)
	if !ok || lookup.Entity.Name != "VillageGuard" {
		t.Fatalf("expected original static actor to remain registered after rejected player, got actor=%+v ok=%v", lookup, ok)
	}
	if players := registry.PlayerCharacters(); len(players) != 0 {
		t.Fatalf("expected rejected player to stay out of player snapshots, got %+v", players)
	}
}

func TestEntityRegistryRejectsPlayerVIDAlreadyOwnedByMapOnlyStaticActorVisibilityVID(t *testing.T) {
	registry := NewEntityRegistry()
	actor, ok := registry.RegisterStaticActorWithID(StaticEntity{Entity: Entity{ID: 0x02040101, Name: "MapOnlyGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300})
	if !ok {
		t.Fatal("expected static actor registration with explicit visibility VID to succeed")
	}
	if _, ok := registry.staticActors.Remove(actor.Entity.ID); !ok {
		t.Fatal("expected direct static-actor directory removal to simulate partial index loss")
	}

	player := registry.RegisterPlayer(entityRegistryCharacter("Alpha", uint32(actor.Entity.ID), 42, 1800, 2900))
	if player.Entity.ID != 0 {
		t.Fatalf("expected player registration with map-only static actor visible VID collision to fail closed, got %+v", player)
	}
	lookup, ok := registry.maps.StaticActor(actor.Entity.ID)
	if !ok || lookup.Entity.Name != "MapOnlyGuard" {
		t.Fatalf("expected map-only static actor map presence to remain after rejected player, got actor=%+v ok=%v", lookup, ok)
	}
	if players := registry.PlayerCharacters(); len(players) != 0 {
		t.Fatalf("expected rejected player to stay out of player snapshots, got %+v", players)
	}
}

func TestEntityRegistryRejectsStaticActorVisibilityVIDAlreadyOwnedByPlayer(t *testing.T) {
	registry := NewEntityRegistry()
	player := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 42, 1700, 2800))
	if player.Entity.ID == 0 {
		t.Fatal("expected player registration to succeed")
	}

	actor, ok := registry.RegisterStaticActorWithID(StaticEntity{Entity: Entity{ID: uint64(player.Entity.VID), Name: "VillageGuard"}, Position: NewPosition(42, 1800, 2900), RaceNum: 20300})
	if ok {
		t.Fatalf("expected static actor registration with colliding visible VID to fail closed, got %+v", actor)
	}
	lookup, ok := registry.Player(player.Entity.ID)
	if !ok || lookup.Entity.Name != "Alpha" {
		t.Fatalf("expected original player to remain registered after rejected static actor, got player=%+v ok=%v", lookup, ok)
	}
	if actors := registry.AllStaticActors(); len(actors) != 0 {
		t.Fatalf("expected rejected static actor to stay out of static actor snapshots, got %+v", actors)
	}
}

func TestEntityRegistryRejectsStaticActorVisibilityVIDAlreadyOwnedByMapOnlyPlayer(t *testing.T) {
	registry := NewEntityRegistry()
	player := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 42, 1700, 2800))
	if player.Entity.ID == 0 {
		t.Fatal("expected player registration to succeed")
	}
	if _, ok := registry.players.Remove(player.Entity.ID); !ok {
		t.Fatal("expected direct player-directory removal to simulate partial index loss")
	}

	actor, ok := registry.RegisterStaticActorWithID(StaticEntity{Entity: Entity{ID: uint64(player.Entity.VID), Name: "VillageGuard"}, Position: NewPosition(42, 1800, 2900), RaceNum: 20300})
	if ok {
		t.Fatalf("expected static actor registration with map-only player visible VID collision to fail closed, got %+v", actor)
	}
	lookup, ok := registry.maps.Remove(player.Entity.ID)
	if !ok || lookup.Entity.Name != "Alpha" || lookup.Entity.VID != player.Entity.VID {
		t.Fatalf("expected map-only player presence to remain after rejected static actor, got player=%+v ok=%v", lookup, ok)
	}
	if actors := registry.AllStaticActors(); len(actors) != 0 {
		t.Fatalf("expected rejected static actor to stay out of static actor snapshots, got %+v", actors)
	}
}

func TestEntityRegistryRejectsStaticActorUpdateThatWouldCollideWithPlayerVID(t *testing.T) {
	registry := NewEntityRegistry()
	player := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 42, 1700, 2800))
	if player.Entity.ID == 0 {
		t.Fatal("expected player registration to succeed")
	}
	actor, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 1800, 2900), RaceNum: 20300})
	if !ok {
		t.Fatal("expected static actor registration to succeed before colliding update")
	}

	updated := actor
	updated.Entity.ID = uint64(player.Entity.VID)
	if result, ok := registry.UpdateStaticActor(updated); ok {
		t.Fatalf("expected update to static actor with player VID collision to fail closed, got %+v", result)
	}
	lookup, ok := registry.StaticActor(actor.Entity.ID)
	if !ok || lookup.Entity.ID != actor.Entity.ID || lookup.Entity.Name != "VillageGuard" {
		t.Fatalf("expected original static actor to remain unchanged, got actor=%+v ok=%v", lookup, ok)
	}
	playerLookup, ok := registry.Player(player.Entity.ID)
	if !ok || playerLookup.Entity.VID != player.Entity.VID || playerLookup.Entity.Name != "Alpha" {
		t.Fatalf("expected player entity to remain unchanged, got player=%+v ok=%v", playerLookup, ok)
	}
}

func TestEntityRegistryRejectsStaticActorUpdateThatWouldCollideWithMapOnlyPlayerVID(t *testing.T) {
	registry := NewEntityRegistry()
	player := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 42, 1700, 2800))
	if player.Entity.ID == 0 {
		t.Fatal("expected player registration to succeed")
	}
	if _, ok := registry.players.Remove(player.Entity.ID); !ok {
		t.Fatal("expected direct player-directory removal to simulate partial index loss")
	}
	actor, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 1800, 2900), RaceNum: 20300})
	if !ok {
		t.Fatal("expected static actor registration to succeed before colliding update")
	}

	updated := actor
	updated.Entity.ID = uint64(player.Entity.VID)
	if result, ok := registry.UpdateStaticActor(updated); ok {
		t.Fatalf("expected update to static actor with map-only player VID collision to fail closed, got %+v", result)
	}
	lookup, ok := registry.StaticActor(actor.Entity.ID)
	if !ok || lookup.Entity.ID != actor.Entity.ID || lookup.Entity.Name != "VillageGuard" {
		t.Fatalf("expected original static actor to remain unchanged, got actor=%+v ok=%v", lookup, ok)
	}
	playerLookup, ok := registry.maps.Remove(player.Entity.ID)
	if !ok || playerLookup.Entity.VID != player.Entity.VID || playerLookup.Entity.Name != "Alpha" {
		t.Fatalf("expected map-only player presence to remain after rejected static actor update, got player=%+v ok=%v", playerLookup, ok)
	}
}

func TestEntityRegistryRejectsPlayerUpdateThatWouldCollideWithStaticActorVisibilityVID(t *testing.T) {
	registry := NewEntityRegistry()
	actor, ok := registry.RegisterStaticActorWithID(StaticEntity{Entity: Entity{ID: 0x02040177, Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300})
	if !ok {
		t.Fatal("expected static actor registration with explicit visibility VID to succeed")
	}
	player := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 42, 1800, 2900))
	if player.Entity.ID == 0 {
		t.Fatal("expected player registration to succeed before colliding update")
	}

	updated := player.Character
	updated.VID = uint32(actor.Entity.ID)
	updated.ID = uint32(actor.Entity.ID)
	if registry.UpdatePlayer(player.Entity.ID, updated) {
		t.Fatal("expected player update with static actor visible VID collision to fail closed")
	}
	lookup, ok := registry.Player(player.Entity.ID)
	if !ok || lookup.Entity.VID != player.Entity.VID || lookup.Character.VID != player.Character.VID || lookup.Entity.Name != "Alpha" {
		t.Fatalf("expected original player to remain unchanged after rejected update, got player=%+v ok=%v", lookup, ok)
	}
	actorLookup, ok := registry.StaticActor(actor.Entity.ID)
	if !ok || actorLookup.Entity.Name != "VillageGuard" {
		t.Fatalf("expected original static actor to remain registered after rejected player update, got actor=%+v ok=%v", actorLookup, ok)
	}
}

func entityRegistryCharacter(name string, vid uint32, mapIndex uint32, x int32, y int32) loginticket.Character {
	return loginticket.Character{
		ID:       vid,
		VID:      vid,
		Name:     name,
		MapIndex: mapIndex,
		X:        x,
		Y:        y,
	}
}
