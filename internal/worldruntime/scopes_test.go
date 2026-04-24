package worldruntime

import "testing"

func TestScopesVisibleTargetsFollowConfiguredVisibilityPolicy(t *testing.T) {
	topology := NewBootstrapTopology(1).WithRadiusVisibilityPolicy(400, 200)
	registry := NewEntityRegistryWithTopology(topology)

	subjectCharacter := entityRegistryCharacter("Subject", 0x02040101, 42, 1700, 2800)
	subjectCharacter.Empire = 2
	subject := registry.RegisterPlayer(subjectCharacter)
	nearPeerCharacter := entityRegistryCharacter("NearPeer", 0x02040102, 42, 1900, 2900)
	nearPeerCharacter.Empire = 2
	nearPeer := registry.RegisterPlayer(nearPeerCharacter)
	farPeerCharacter := entityRegistryCharacter("FarPeer", 0x02040103, 42, 2600, 3800)
	farPeerCharacter.Empire = 2
	registry.RegisterPlayer(farPeerCharacter)

	scopes := NewScopes(topology, registry)
	targets := scopes.VisibleTargets(subject.Entity.ID, subject.Character)
	if len(targets) != 1 || targets[0].Entity.ID != nearPeer.Entity.ID {
		t.Fatalf("expected only near peer in visible targets, got %+v", targets)
	}
}

func TestScopesLocalTalkTargetsRequireSameEmpireAndConfiguredVisibleWorld(t *testing.T) {
	topology := NewBootstrapTopology(1).WithRadiusVisibilityPolicy(400, 200)
	registry := NewEntityRegistryWithTopology(topology)

	subjectCharacter := entityRegistryCharacter("Subject", 0x02040101, 42, 1700, 2800)
	subjectCharacter.Empire = 2
	subject := registry.RegisterPlayer(subjectCharacter)
	nearSameEmpire := entityRegistryCharacter("NearSameEmpire", 0x02040102, 42, 1900, 2900)
	nearSameEmpire.Empire = 2
	near := registry.RegisterPlayer(nearSameEmpire)
	nearOtherEmpire := entityRegistryCharacter("NearOtherEmpire", 0x02040103, 42, 1800, 2850)
	nearOtherEmpire.Empire = 3
	registry.RegisterPlayer(nearOtherEmpire)
	farSameEmpire := entityRegistryCharacter("FarSameEmpire", 0x02040104, 42, 2500, 3600)
	farSameEmpire.Empire = 2
	registry.RegisterPlayer(farSameEmpire)

	targets := NewScopes(topology, registry).LocalTalkTargets(subject.Entity.ID, subject.Character)
	if len(targets) != 1 || targets[0].Entity.ID != near.Entity.ID {
		t.Fatalf("expected only same-empire visible peer for local talk, got %+v", targets)
	}
}

func TestScopesShoutTargetsRequireSameEmpireButIgnoreMap(t *testing.T) {
	topology := NewBootstrapTopology(1)
	registry := NewEntityRegistryWithTopology(topology)

	subjectCharacter := entityRegistryCharacter("Subject", 0x02040101, 1, 1700, 2800)
	subjectCharacter.Empire = 2
	subject := registry.RegisterPlayer(subjectCharacter)
	sameEmpireOtherMap := entityRegistryCharacter("SameEmpireOtherMap", 0x02040102, 42, 1900, 2900)
	sameEmpireOtherMap.Empire = 2
	target := registry.RegisterPlayer(sameEmpireOtherMap)
	otherEmpire := entityRegistryCharacter("OtherEmpire", 0x02040103, 42, 1800, 2850)
	otherEmpire.Empire = 3
	registry.RegisterPlayer(otherEmpire)

	targets := NewScopes(topology, registry).ShoutTargets(subject.Entity.ID, subject.Character)
	if len(targets) != 1 || targets[0].Entity.ID != target.Entity.ID {
		t.Fatalf("expected only same-empire shout target, got %+v", targets)
	}
}

func TestScopesPartyTargetsReturnAllOtherConnectedPlayers(t *testing.T) {
	topology := NewBootstrapTopology(1)
	registry := NewEntityRegistryWithTopology(topology)

	subject := registry.RegisterPlayer(entityRegistryCharacter("Subject", 0x02040101, 1, 1700, 2800))
	nearPeer := registry.RegisterPlayer(entityRegistryCharacter("NearPeer", 0x02040102, 1, 1900, 2900))
	farPeer := registry.RegisterPlayer(entityRegistryCharacter("FarPeer", 0x02040103, 42, 2800, 3900))

	targets := NewScopes(topology, registry).PartyTargets(subject.Entity.ID)
	if len(targets) != 2 {
		t.Fatalf("expected 2 party targets for connected peers, got %+v", targets)
	}
	if targets[0].Entity.ID != farPeer.Entity.ID || targets[1].Entity.ID != nearPeer.Entity.ID {
		t.Fatalf("expected deterministic party targets [FarPeer NearPeer], got %+v", targets)
	}
}

func TestScopesGuildTargetsRequireSharedNonZeroGuildID(t *testing.T) {
	topology := NewBootstrapTopology(1)
	registry := NewEntityRegistryWithTopology(topology)

	subjectCharacter := entityRegistryCharacter("Subject", 0x02040101, 1, 1700, 2800)
	subjectCharacter.GuildID = 10
	subject := registry.RegisterPlayer(subjectCharacter)
	sameGuild := entityRegistryCharacter("SameGuild", 0x02040102, 42, 1900, 2900)
	sameGuild.GuildID = 10
	target := registry.RegisterPlayer(sameGuild)
	otherGuild := entityRegistryCharacter("OtherGuild", 0x02040103, 42, 1800, 2850)
	otherGuild.GuildID = 11
	registry.RegisterPlayer(otherGuild)
	zeroGuild := entityRegistryCharacter("ZeroGuild", 0x02040104, 42, 1800, 2850)
	registry.RegisterPlayer(zeroGuild)

	targets := NewScopes(topology, registry).GuildTargets(subject.Entity.ID, subject.Character)
	if len(targets) != 1 || targets[0].Entity.ID != target.Entity.ID {
		t.Fatalf("expected only same-guild target, got %+v", targets)
	}
}

func TestScopesPlayerByExactNameUsesEntityRegistry(t *testing.T) {
	topology := NewBootstrapTopology(1)
	registry := NewEntityRegistryWithTopology(topology)
	alpha := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100))
	registry.RegisterPlayer(entityRegistryCharacter("Bravo", 0x02040102, 1, 1300, 2300))

	lookup, ok := NewScopes(topology, registry).PlayerByExactName("Alpha")
	if !ok || lookup.Entity.ID != alpha.Entity.ID {
		t.Fatalf("expected exact-name scope lookup to return Alpha, got entity=%+v ok=%v", lookup, ok)
	}
	if _, ok := NewScopes(topology, registry).PlayerByExactName("Missing"); ok {
		t.Fatal("expected missing exact-name scope lookup to fail")
	}
}

func TestScopesConnectedTargetsReturnAllPlayersInDeterministicOrder(t *testing.T) {
	topology := NewBootstrapTopology(1)
	registry := NewEntityRegistryWithTopology(topology)
	registry.RegisterPlayer(entityRegistryCharacter("Zulu", 0x02040101, 42, 1700, 2800))
	alpha := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040102, 1, 1100, 2100))
	bravo := registry.RegisterPlayer(entityRegistryCharacter("Bravo", 0x02040103, 1, 1300, 2300))

	targets := NewScopes(topology, registry).ConnectedTargets()
	if len(targets) != 3 {
		t.Fatalf("expected 3 connected targets, got %+v", targets)
	}
	if targets[0].Entity.ID != alpha.Entity.ID || targets[1].Entity.ID != bravo.Entity.ID {
		t.Fatalf("expected deterministic connected targets starting with Alpha, Bravo, got %+v", targets)
	}
	if targets[2].Entity.Name != "Zulu" {
		t.Fatalf("expected Zulu as final connected target, got %+v", targets[2])
	}
}

func TestScopesVisibilitySnapshotsFollowConfiguredPolicyAndOrder(t *testing.T) {
	topology := NewBootstrapTopology(1).WithRadiusVisibilityPolicy(400, 200)
	registry := NewEntityRegistryWithTopology(topology)
	zulu := entityRegistryCharacter("Zulu", 0x02040101, 42, 1700, 2800)
	zulu.Empire = 2
	registry.RegisterPlayer(zulu)
	alpha := entityRegistryCharacter("Alpha", 0x02040102, 42, 1900, 2900)
	alpha.Empire = 2
	alphaEntity := registry.RegisterPlayer(alpha)
	bravo := entityRegistryCharacter("Bravo", 0x02040103, 42, 1750, 2820)
	bravo.Empire = 2
	bravoEntity := registry.RegisterPlayer(bravo)

	snapshots := NewScopes(topology, registry).VisibilitySnapshots()
	if len(snapshots) != 3 {
		t.Fatalf("expected 3 visibility snapshots, got %+v", snapshots)
	}
	if snapshots[0].Subject.Entity.ID != alphaEntity.Entity.ID || snapshots[1].Subject.Entity.ID != bravoEntity.Entity.ID || snapshots[2].Subject.Entity.Name != "Zulu" {
		t.Fatalf("expected deterministic subject ordering Alpha, Bravo, Zulu, got %+v", snapshots)
	}
	if len(snapshots[0].VisiblePeers) != 2 || snapshots[0].VisiblePeers[0].Entity.Name != "Bravo" || snapshots[0].VisiblePeers[1].Entity.Name != "Zulu" {
		t.Fatalf("expected Alpha to see Bravo and Zulu, got %+v", snapshots[0].VisiblePeers)
	}
	if len(snapshots[2].VisiblePeers) != 2 || snapshots[2].VisiblePeers[0].Entity.Name != "Alpha" || snapshots[2].VisiblePeers[1].Entity.Name != "Bravo" {
		t.Fatalf("expected Zulu to see Alpha and Bravo, got %+v", snapshots[2].VisiblePeers)
	}
}

func TestScopesVisibleStaticActorsFollowConfiguredVisibilityPolicyAndOrder(t *testing.T) {
	topology := NewBootstrapTopology(1).WithRadiusVisibilityPolicy(400, 200)
	registry := NewEntityRegistryWithTopology(topology)
	subjectCharacter := entityRegistryCharacter("Subject", 0x02040101, 42, 1700, 2800)
	subject := registry.RegisterPlayer(subjectCharacter)
	blacksmith, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "Blacksmith"}, Position: NewPosition(42, 1750, 2850), RaceNum: 20300})
	if !ok {
		t.Fatal("expected Blacksmith registration to succeed")
	}
	merchant, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "Merchant"}, Position: NewPosition(42, 1900, 2900), RaceNum: 20301})
	if !ok {
		t.Fatal("expected Merchant registration to succeed")
	}
	if _, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 2600, 3800), RaceNum: 20302}); !ok {
		t.Fatal("expected far static actor registration to succeed")
	}

	actors := NewScopes(topology, registry).VisibleStaticActors(subject.Character)
	if len(actors) != 2 {
		t.Fatalf("expected 2 visible static actors, got %+v", actors)
	}
	if actors[0].Entity.ID != blacksmith.Entity.ID || actors[1].Entity.ID != merchant.Entity.ID {
		t.Fatalf("expected deterministic visible static actor ordering [Blacksmith Merchant], got %+v", actors)
	}
	if actors[0].Position.MapIndex != 42 || actors[1].Position.MapIndex != 42 {
		t.Fatalf("expected visible static actors to preserve effective map presence, got %+v", actors)
	}
	if subject.Entity.ID == 0 {
		t.Fatal("expected subject entity to have a stable runtime identity")
	}
}

func TestScopesRelocateStaticActorVisibilityDiffUsesConfiguredPolicyAndOrder(t *testing.T) {
	topology := NewBootstrapTopology(1).WithRadiusVisibilityPolicy(400, 200)
	registry := NewEntityRegistryWithTopology(topology)
	current := entityRegistryCharacter("Subject", 0x02040101, 42, 1700, 2800)
	registry.RegisterPlayer(current)
	originActor, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "Blacksmith"}, Position: NewPosition(42, 1750, 2850), RaceNum: 20300})
	if !ok {
		t.Fatal("expected origin static actor registration to succeed")
	}
	if _, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 2600, 3800), RaceNum: 20301}); !ok {
		t.Fatal("expected far static actor registration to succeed")
	}
	targetActorA, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "Alchemist"}, Position: NewPosition(42, 5000, 5000), RaceNum: 20302})
	if !ok {
		t.Fatal("expected first destination static actor registration to succeed")
	}
	targetActorB, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "Merchant"}, Position: NewPosition(42, 5200, 5100), RaceNum: 20303})
	if !ok {
		t.Fatal("expected second destination static actor registration to succeed")
	}

	target := current
	target.X = 5000
	target.Y = 5000

	diff := NewScopes(topology, registry).RelocateStaticActorVisibilityDiff(current, target)
	if len(diff.CurrentVisibleActors) != 1 || diff.CurrentVisibleActors[0].Entity.ID != originActor.Entity.ID {
		t.Fatalf("expected current visible static actors [Blacksmith], got %+v", diff.CurrentVisibleActors)
	}
	if len(diff.TargetVisibleActors) != 2 || diff.TargetVisibleActors[0].Entity.ID != targetActorA.Entity.ID || diff.TargetVisibleActors[1].Entity.ID != targetActorB.Entity.ID {
		t.Fatalf("expected deterministic target visible static actors [Alchemist Merchant], got %+v", diff.TargetVisibleActors)
	}
	if len(diff.RemovedVisibleActors) != 1 || diff.RemovedVisibleActors[0].Entity.ID != originActor.Entity.ID {
		t.Fatalf("expected removed visible static actors [Blacksmith], got %+v", diff.RemovedVisibleActors)
	}
	if len(diff.AddedVisibleActors) != 2 || diff.AddedVisibleActors[0].Entity.ID != targetActorA.Entity.ID || diff.AddedVisibleActors[1].Entity.ID != targetActorB.Entity.ID {
		t.Fatalf("expected added visible static actors [Alchemist Merchant], got %+v", diff.AddedVisibleActors)
	}
}

func TestScopesEnterVisibilityDiffUsesConfiguredPolicyAndRegistrySnapshot(t *testing.T) {
	topology := NewBootstrapTopology(1).WithRadiusVisibilityPolicy(400, 200)
	registry := NewEntityRegistryWithTopology(topology)
	nearPeer := entityRegistryCharacter("NearPeer", 0x02040101, 42, 1900, 2900)
	registry.RegisterPlayer(nearPeer)
	registry.RegisterPlayer(entityRegistryCharacter("FarPeer", 0x02040102, 42, 2600, 3800))

	subject := entityRegistryCharacter("Subject", 0x02040103, 42, 1700, 2800)
	diff := NewScopes(topology, registry).EnterVisibilityDiff(subject)
	if len(diff.CurrentVisiblePeers) != 0 {
		t.Fatalf("expected no current visible peers before enter, got %+v", diff.CurrentVisiblePeers)
	}
	if len(diff.TargetVisiblePeers) != 1 || diff.TargetVisiblePeers[0].Name != "NearPeer" {
		t.Fatalf("expected target visible peers [NearPeer], got %+v", diff.TargetVisiblePeers)
	}
	if len(diff.AddedVisiblePeers) != 1 || diff.AddedVisiblePeers[0].Name != "NearPeer" {
		t.Fatalf("expected added visible peers [NearPeer], got %+v", diff.AddedVisiblePeers)
	}
	if len(diff.RemovedVisiblePeers) != 0 {
		t.Fatalf("expected no removed visible peers on enter, got %+v", diff.RemovedVisiblePeers)
	}
}

func TestScopesLeaveVisibilityDiffUsesConfiguredPolicyAndRegistrySnapshot(t *testing.T) {
	topology := NewBootstrapTopology(1).WithRadiusVisibilityPolicy(400, 200)
	registry := NewEntityRegistryWithTopology(topology)
	subjectCharacter := entityRegistryCharacter("Subject", 0x02040101, 42, 1700, 2800)
	subject := registry.RegisterPlayer(subjectCharacter)
	nearPeer := entityRegistryCharacter("NearPeer", 0x02040102, 42, 1900, 2900)
	registry.RegisterPlayer(nearPeer)
	registry.RegisterPlayer(entityRegistryCharacter("FarPeer", 0x02040103, 42, 2600, 3800))

	diff := NewScopes(topology, registry).LeaveVisibilityDiff(subject.Character)
	if len(diff.CurrentVisiblePeers) != 1 || diff.CurrentVisiblePeers[0].Name != "NearPeer" {
		t.Fatalf("expected current visible peers [NearPeer], got %+v", diff.CurrentVisiblePeers)
	}
	if len(diff.TargetVisiblePeers) != 0 {
		t.Fatalf("expected no target visible peers after leave, got %+v", diff.TargetVisiblePeers)
	}
	if len(diff.RemovedVisiblePeers) != 1 || diff.RemovedVisiblePeers[0].Name != "NearPeer" {
		t.Fatalf("expected removed visible peers [NearPeer], got %+v", diff.RemovedVisiblePeers)
	}
	if len(diff.AddedVisiblePeers) != 0 {
		t.Fatalf("expected no added visible peers on leave, got %+v", diff.AddedVisiblePeers)
	}
}

func TestScopesRelocateVisibilityDiffUsesConfiguredPolicyAndRegistrySnapshot(t *testing.T) {
	topology := NewBootstrapTopology(1).WithRadiusVisibilityPolicy(400, 200)
	registry := NewEntityRegistryWithTopology(topology)
	subjectCharacter := entityRegistryCharacter("Subject", 0x02040101, 42, 1700, 2800)
	registry.RegisterPlayer(subjectCharacter)
	registry.RegisterPlayer(entityRegistryCharacter("OriginPeer", 0x02040102, 42, 1900, 2900))
	registry.RegisterPlayer(entityRegistryCharacter("FarPeer", 0x02040103, 42, 2600, 3800))
	registry.RegisterPlayer(entityRegistryCharacter("DestinationPeer", 0x02040104, 42, 5200, 5100))

	target := subjectCharacter
	target.X = 5000
	target.Y = 5000

	diff := NewScopes(topology, registry).RelocateVisibilityDiff(subjectCharacter, target)
	if len(diff.CurrentVisiblePeers) != 1 || diff.CurrentVisiblePeers[0].Name != "OriginPeer" {
		t.Fatalf("expected current visible peers [OriginPeer], got %+v", diff.CurrentVisiblePeers)
	}
	if len(diff.TargetVisiblePeers) != 1 || diff.TargetVisiblePeers[0].Name != "DestinationPeer" {
		t.Fatalf("expected target visible peers [DestinationPeer], got %+v", diff.TargetVisiblePeers)
	}
	if len(diff.RemovedVisiblePeers) != 1 || diff.RemovedVisiblePeers[0].Name != "OriginPeer" {
		t.Fatalf("expected removed visible peers [OriginPeer], got %+v", diff.RemovedVisiblePeers)
	}
	if len(diff.AddedVisiblePeers) != 1 || diff.AddedVisiblePeers[0].Name != "DestinationPeer" {
		t.Fatalf("expected added visible peers [DestinationPeer], got %+v", diff.AddedVisiblePeers)
	}
}

func TestScopesMapOccupancySnapshotsIncludeStaticOnlyMapsAndDeterministicOrder(t *testing.T) {
	topology := NewBootstrapTopology(1)
	registry := NewEntityRegistryWithTopology(topology)
	registry.RegisterPlayer(entityRegistryCharacter("Zulu", 0x02040101, 42, 1700, 2800))
	alpha := entityRegistryCharacter("Alpha", 0x02040102, 1, 1100, 2100)
	alpha.Empire = 2
	registry.RegisterPlayer(alpha)
	bravo := entityRegistryCharacter("Bravo", 0x02040103, 1, 1300, 2300)
	bravo.Empire = 3
	registry.RegisterPlayer(bravo)
	if _, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 1800, 2900), RaceNum: 20300}); !ok {
		t.Fatal("expected destination static actor registration to succeed")
	}
	merchant, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "Merchant"}, Position: NewPosition(99, 900, 1200), RaceNum: 9001})
	if !ok {
		t.Fatal("expected static-only map actor registration to succeed")
	}
	blacksmith, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "Blacksmith"}, Position: NewPosition(1, 1500, 2500), RaceNum: 20016})
	if !ok {
		t.Fatal("expected bootstrap static actor registration to succeed")
	}

	snapshots := NewScopes(topology, registry).MapOccupancySnapshots()
	if len(snapshots) != 3 {
		t.Fatalf("expected 3 map occupancy snapshots, got %+v", snapshots)
	}
	if snapshots[0].MapIndex != 1 || snapshots[1].MapIndex != 42 || snapshots[2].MapIndex != 99 {
		t.Fatalf("expected deterministic map ordering [1 42 99], got %+v", snapshots)
	}
	if snapshots[0].CharacterCount != 2 || len(snapshots[0].Characters) != 2 || snapshots[0].Characters[0].Name != "Alpha" || snapshots[0].Characters[1].Name != "Bravo" {
		t.Fatalf("expected bootstrap map player snapshots [Alpha Bravo], got %+v", snapshots[0])
	}
	if snapshots[0].StaticActorCount != 1 || len(snapshots[0].StaticActors) != 1 || snapshots[0].StaticActors[0].EntityID != blacksmith.Entity.ID || snapshots[0].StaticActors[0].Name != "Blacksmith" {
		t.Fatalf("expected bootstrap map static actor Blacksmith, got %+v", snapshots[0])
	}
	if snapshots[1].CharacterCount != 1 || len(snapshots[1].Characters) != 1 || snapshots[1].Characters[0].Name != "Zulu" {
		t.Fatalf("expected destination map player snapshot Zulu, got %+v", snapshots[1])
	}
	if snapshots[2].CharacterCount != 0 || len(snapshots[2].Characters) != 0 || snapshots[2].StaticActorCount != 1 || len(snapshots[2].StaticActors) != 1 || snapshots[2].StaticActors[0].EntityID != merchant.Entity.ID || snapshots[2].StaticActors[0].Name != "Merchant" {
		t.Fatalf("expected static-only map snapshot for Merchant, got %+v", snapshots[2])
	}
}

func TestScopesStaticActorSnapshotsReturnDeterministicOrder(t *testing.T) {
	topology := NewBootstrapTopology(1)
	registry := NewEntityRegistryWithTopology(topology)
	zulu, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "ZuluStatue"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300})
	if !ok {
		t.Fatal("expected ZuluStatue registration to succeed")
	}
	alpha, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "AlphaForge"}, Position: NewPosition(1, 1100, 2100), RaceNum: 20016})
	if !ok {
		t.Fatal("expected AlphaForge registration to succeed")
	}
	alphaTwin, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "AlphaForge"}, Position: NewPosition(99, 900, 1200), RaceNum: 20017})
	if !ok {
		t.Fatal("expected second AlphaForge registration to succeed")
	}

	snapshots := NewScopes(topology, registry).StaticActorSnapshots()
	if len(snapshots) != 3 {
		t.Fatalf("expected 3 static actor snapshots, got %+v", snapshots)
	}
	if snapshots[0].EntityID != alpha.Entity.ID || snapshots[1].EntityID != alphaTwin.Entity.ID || snapshots[2].EntityID != zulu.Entity.ID {
		t.Fatalf("expected deterministic static actor ordering [AlphaForge AlphaForge ZuluStatue], got %+v", snapshots)
	}
	if snapshots[0].MapIndex != 1 || snapshots[1].MapIndex != 99 || snapshots[2].MapIndex != 42 {
		t.Fatalf("expected static actor snapshots to preserve effective map indices, got %+v", snapshots)
	}
}

func TestScopesBuildRelocationPreviewPreservesStaticOnlyMapsAndCharacterCountDeltas(t *testing.T) {
	topology := NewBootstrapTopology(1)
	registry := NewEntityRegistryWithTopology(topology)
	peerOne := entityRegistryCharacter("PeerOne", 0x02040101, 1, 1100, 2100)
	peerOne.Empire = 2
	registry.RegisterPlayer(peerOne)
	current := entityRegistryCharacter("PeerTwo", 0x02040102, 1, 1300, 2300)
	current.Empire = 2
	registry.RegisterPlayer(current)
	peerThree := entityRegistryCharacter("PeerThree", 0x02040103, 42, 1700, 2800)
	peerThree.Empire = 2
	registry.RegisterPlayer(peerThree)
	blacksmith, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "Blacksmith"}, Position: NewPosition(1, 1500, 2500), RaceNum: 20016})
	if !ok {
		t.Fatal("expected bootstrap static actor registration to succeed")
	}
	guard, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 1800, 2900), RaceNum: 20300})
	if !ok {
		t.Fatal("expected destination static actor registration to succeed")
	}
	merchant, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "Merchant"}, Position: NewPosition(99, 900, 1200), RaceNum: 9001})
	if !ok {
		t.Fatal("expected static-only actor registration to succeed")
	}

	target := current
	target.MapIndex = 42
	target.X = 1750
	target.Y = 2850

	preview := NewScopes(topology, registry).BuildRelocationPreview(current, target, false)
	if preview.Applied {
		t.Fatal("expected relocate preview helper to preserve applied=false")
	}
	if len(preview.CurrentVisibleStaticActors) != 1 || preview.CurrentVisibleStaticActors[0].EntityID != blacksmith.Entity.ID || preview.CurrentVisibleStaticActors[0].Name != "Blacksmith" {
		t.Fatalf("unexpected current visible static actors: %+v", preview.CurrentVisibleStaticActors)
	}
	if len(preview.TargetVisibleStaticActors) != 1 || preview.TargetVisibleStaticActors[0].EntityID != guard.Entity.ID || preview.TargetVisibleStaticActors[0].Name != "VillageGuard" {
		t.Fatalf("unexpected target visible static actors: %+v", preview.TargetVisibleStaticActors)
	}
	if len(preview.RemovedVisibleStaticActors) != 1 || preview.RemovedVisibleStaticActors[0].EntityID != blacksmith.Entity.ID || preview.RemovedVisibleStaticActors[0].Name != "Blacksmith" {
		t.Fatalf("unexpected removed visible static actors: %+v", preview.RemovedVisibleStaticActors)
	}
	if len(preview.AddedVisibleStaticActors) != 1 || preview.AddedVisibleStaticActors[0].EntityID != guard.Entity.ID || preview.AddedVisibleStaticActors[0].Name != "VillageGuard" {
		t.Fatalf("unexpected added visible static actors: %+v", preview.AddedVisibleStaticActors)
	}
	if len(preview.MapOccupancyChanges) != 2 {
		t.Fatalf("expected 2 occupancy deltas, got %+v", preview.MapOccupancyChanges)
	}
	if preview.MapOccupancyChanges[0] != (MapOccupancyChange{MapIndex: 1, BeforeCount: 2, AfterCount: 1}) {
		t.Fatalf("unexpected source occupancy delta: %+v", preview.MapOccupancyChanges[0])
	}
	if preview.MapOccupancyChanges[1] != (MapOccupancyChange{MapIndex: 42, BeforeCount: 1, AfterCount: 2}) {
		t.Fatalf("unexpected destination occupancy delta: %+v", preview.MapOccupancyChanges[1])
	}
	if len(preview.BeforeMapOccupancy) != 3 || len(preview.AfterMapOccupancy) != 3 {
		t.Fatalf("expected static-only maps to be preserved in before/after occupancy, before=%+v after=%+v", preview.BeforeMapOccupancy, preview.AfterMapOccupancy)
	}
	if preview.BeforeMapOccupancy[0].MapIndex != 1 || preview.BeforeMapOccupancy[0].StaticActorCount != 1 || preview.BeforeMapOccupancy[0].StaticActors[0].EntityID != blacksmith.Entity.ID {
		t.Fatalf("unexpected bootstrap before occupancy snapshot: %+v", preview.BeforeMapOccupancy[0])
	}
	if preview.BeforeMapOccupancy[1].MapIndex != 42 || preview.BeforeMapOccupancy[1].StaticActorCount != 1 || preview.BeforeMapOccupancy[1].StaticActors[0].EntityID != guard.Entity.ID {
		t.Fatalf("unexpected destination before occupancy snapshot: %+v", preview.BeforeMapOccupancy[1])
	}
	if preview.BeforeMapOccupancy[2].MapIndex != 99 || preview.BeforeMapOccupancy[2].CharacterCount != 0 || preview.BeforeMapOccupancy[2].StaticActorCount != 1 || preview.BeforeMapOccupancy[2].StaticActors[0].EntityID != merchant.Entity.ID {
		t.Fatalf("unexpected static-only before occupancy snapshot: %+v", preview.BeforeMapOccupancy[2])
	}
	if preview.AfterMapOccupancy[0].MapIndex != 1 || preview.AfterMapOccupancy[0].CharacterCount != 1 || len(preview.AfterMapOccupancy[0].Characters) != 1 || preview.AfterMapOccupancy[0].Characters[0].Name != "PeerOne" {
		t.Fatalf("unexpected bootstrap after occupancy snapshot: %+v", preview.AfterMapOccupancy[0])
	}
	if preview.AfterMapOccupancy[1].MapIndex != 42 || preview.AfterMapOccupancy[1].CharacterCount != 2 || len(preview.AfterMapOccupancy[1].Characters) != 2 || preview.AfterMapOccupancy[1].Characters[0].Name != "PeerThree" || preview.AfterMapOccupancy[1].Characters[1].Name != "PeerTwo" {
		t.Fatalf("unexpected destination after occupancy snapshot: %+v", preview.AfterMapOccupancy[1])
	}
	if preview.AfterMapOccupancy[2].MapIndex != 99 || preview.AfterMapOccupancy[2].CharacterCount != 0 || preview.AfterMapOccupancy[2].StaticActorCount != 1 || preview.AfterMapOccupancy[2].StaticActors[0].EntityID != merchant.Entity.ID {
		t.Fatalf("unexpected static-only after occupancy snapshot: %+v", preview.AfterMapOccupancy[2])
	}
}

func TestScopesBuildRelocationPreviewUsesAppliedFlag(t *testing.T) {
	topology := NewBootstrapTopology(1)
	registry := NewEntityRegistryWithTopology(topology)
	current := entityRegistryCharacter("PeerTwo", 0x02040102, 1, 1300, 2300)
	registry.RegisterPlayer(current)
	target := current
	target.MapIndex = 42
	target.X = 1750
	target.Y = 2850

	preview := NewScopes(topology, registry).BuildRelocationPreview(current, target, true)
	if !preview.Applied {
		t.Fatal("expected relocate preview helper to preserve applied=true")
	}
}
