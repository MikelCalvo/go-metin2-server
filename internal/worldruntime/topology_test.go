package worldruntime

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestNewBootstrapTopologyDefaultsLocalChannelToOne(t *testing.T) {
	topology := NewBootstrapTopology(0)

	if got := topology.LocalChannelID(); got != 1 {
		t.Fatalf("expected default local channel id 1, got %d", got)
	}
	if got := topology.EffectiveChannelID(loginticket.Character{}); got != 1 {
		t.Fatalf("expected effective local channel id 1 for bootstrap character, got %d", got)
	}
}

func TestBootstrapTopologyTreatsZeroMapIndexAsBootstrapMap(t *testing.T) {
	topology := NewBootstrapTopology(7)

	if got := topology.EffectiveMapIndex(loginticket.Character{}); got != 1 {
		t.Fatalf("expected zero map index to normalize to bootstrap map 1, got %d", got)
	}
	if got := topology.EffectiveMapIndex(loginticket.Character{MapIndex: 42}); got != 42 {
		t.Fatalf("expected non-zero map index to remain unchanged, got %d", got)
	}
}

func TestBootstrapTopologySharesVisibleWorldOnlyOnSameEffectiveMap(t *testing.T) {
	topology := NewBootstrapTopology(3)

	bootstrapZero := loginticket.Character{MapIndex: 0}
	bootstrapOne := loginticket.Character{MapIndex: 1}
	otherMap := loginticket.Character{MapIndex: 42}

	if !topology.SharesVisibleWorld(bootstrapZero, bootstrapOne) {
		t.Fatalf("expected bootstrap map aliases to share visible world")
	}
	if topology.SharesVisibleWorld(bootstrapZero, otherMap) {
		t.Fatalf("did not expect different effective maps to share visible world")
	}
}

func TestBootstrapTopologyTalkingChatScopeRequiresSameVisibleWorldAndEmpire(t *testing.T) {
	topology := NewBootstrapTopology(1)

	left := loginticket.Character{MapIndex: 1, Empire: 3}
	sameScope := loginticket.Character{MapIndex: 1, Empire: 3}
	otherEmpire := loginticket.Character{MapIndex: 1, Empire: 1}
	otherMap := loginticket.Character{MapIndex: 42, Empire: 3}
	zeroEmpire := loginticket.Character{MapIndex: 1, Empire: 0}

	if !topology.SharesTalkingChatScope(left, sameScope) {
		t.Fatalf("expected same-map same-empire players to share local talking scope")
	}
	if topology.SharesTalkingChatScope(left, otherEmpire) {
		t.Fatalf("did not expect different empires to share local talking scope")
	}
	if topology.SharesTalkingChatScope(left, otherMap) {
		t.Fatalf("did not expect different maps to share local talking scope")
	}
	if topology.SharesTalkingChatScope(left, zeroEmpire) {
		t.Fatalf("did not expect zero-empire peer to share local talking scope")
	}
}

func TestBootstrapTopologyShoutScopeIgnoresMapButRequiresEmpire(t *testing.T) {
	topology := NewBootstrapTopology(1)

	left := loginticket.Character{MapIndex: 1, Empire: 3}
	sameEmpireOtherMap := loginticket.Character{MapIndex: 42, Empire: 3}
	otherEmpire := loginticket.Character{MapIndex: 42, Empire: 1}
	zeroEmpire := loginticket.Character{MapIndex: 42, Empire: 0}

	if !topology.SharesShoutScope(left, sameEmpireOtherMap) {
		t.Fatalf("expected same-empire peers on the local channel to share shout scope across maps")
	}
	if topology.SharesShoutScope(left, otherEmpire) {
		t.Fatalf("did not expect different empires to share shout scope")
	}
	if topology.SharesShoutScope(left, zeroEmpire) {
		t.Fatalf("did not expect zero-empire peer to share shout scope")
	}
}

func TestBootstrapTopologyGuildChatScopeRequiresNonZeroSharedGuildID(t *testing.T) {
	topology := NewBootstrapTopology(1)

	left := loginticket.Character{GuildID: 99}
	sameGuild := loginticket.Character{GuildID: 99}
	otherGuild := loginticket.Character{GuildID: 77}
	zeroGuild := loginticket.Character{GuildID: 0}

	if !topology.SharesGuildChatScope(left, sameGuild) {
		t.Fatalf("expected same non-zero guild members to share guild chat scope")
	}
	if topology.SharesGuildChatScope(left, otherGuild) {
		t.Fatalf("did not expect different guild ids to share guild chat scope")
	}
	if topology.SharesGuildChatScope(left, zeroGuild) {
		t.Fatalf("did not expect zero-guild peer to share guild chat scope")
	}
}
