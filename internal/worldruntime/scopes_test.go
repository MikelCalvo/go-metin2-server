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
