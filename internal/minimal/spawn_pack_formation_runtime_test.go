package minimal

import (
	"errors"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
)

// Opt-in regen_spawns.formation = "line" places the same multi-count members
// in one file along Y. Omitted and "grid" keep the owned pack_spacing grid.
// One-count plus line, unknown names, and pack_spacing overflow fail closed
// before runtime mutation. This slice does not add pack AI assist.
func TestGameRuntimeOptInLineFormationPlacesPackMembersAlongY(t *testing.T) {
	runtime := newPackFormationRuntime(t)
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		RegenSpawns: []contentbundle.RegenSpawn{{
			Ref:         "practice.line_pack",
			Name:        "LinePack",
			MapIndex:    42,
			X:           1700,
			Y:           2800,
			RaceNum:     20350,
			Count:       3,
			PackSpacing: 100,
			Formation:   "line",
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:      "practice.line_pack_other",
			Name:     "LinePackOther",
			MapIndex: 42,
			X:        1600,
			Y:        2800,
			RaceNum:  20350,
		}},
	}); err != nil {
		t.Fatalf("import line formation pack: %v", err)
	}

	first, ok := runtime.SpawnGroupByRef("practice.line_pack.m01")
	if !ok || first.Dead || first.X != 1700 || first.Y != 2800 {
		t.Fatalf("expected live line member .m01 at authored origin, ok=%v snapshot=%+v", ok, first)
	}
	second, ok := runtime.SpawnGroupByRef("practice.line_pack.m02")
	if !ok || second.Dead || second.X != 1700 || second.Y != 2900 {
		t.Fatalf("expected live line member .m02 one pack_spacing along Y, ok=%v snapshot=%+v", ok, second)
	}
	third, ok := runtime.SpawnGroupByRef("practice.line_pack.m03")
	if !ok || third.Dead || third.X != 1700 || third.Y != 3000 {
		t.Fatalf("expected live line member .m03 two pack_spacing along Y, ok=%v snapshot=%+v", ok, third)
	}
	other, ok := runtime.SpawnGroupByRef("practice.line_pack_other")
	if !ok || other.Dead || other.X != 1600 || other.Y != 2800 {
		t.Fatalf("expected independent spawn group to stay at authored origin, ok=%v snapshot=%+v", ok, other)
	}
}

func TestGameRuntimeOmittedAndGridFormationKeepPackSpacingGrid(t *testing.T) {
	runtime := newPackFormationRuntime(t)
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		RegenSpawns: []contentbundle.RegenSpawn{
			{
				Ref:         "practice.omitted_grid_pack",
				Name:        "OmittedGridPack",
				MapIndex:    42,
				X:           1700,
				Y:           2800,
				RaceNum:     20350,
				Count:       2,
				PackSpacing: 100,
			},
			{
				Ref:         "practice.explicit_grid_pack",
				Name:        "ExplicitGridPack",
				MapIndex:    42,
				X:           1800,
				Y:           2800,
				RaceNum:     20350,
				Count:       2,
				PackSpacing: 100,
				Formation:   "grid",
			},
		},
	}); err != nil {
		t.Fatalf("import omitted and grid formation packs: %v", err)
	}

	omitted, ok := runtime.SpawnGroupByRef("practice.omitted_grid_pack.m02")
	if !ok || omitted.Dead || omitted.X != 1800 || omitted.Y != 2800 {
		t.Fatalf("expected omitted formation .m02 on the owned X grid, ok=%v snapshot=%+v", ok, omitted)
	}
	explicit, ok := runtime.SpawnGroupByRef("practice.explicit_grid_pack.m02")
	if !ok || explicit.Dead || explicit.X != 1900 || explicit.Y != 2800 {
		t.Fatalf("expected explicit grid .m02 on the owned X grid, ok=%v snapshot=%+v", ok, explicit)
	}
}

func TestGameRuntimeRejectsMalformedPackFormationBeforeMutation(t *testing.T) {
	runtime := newPackFormationRuntime(t)
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:      "practice.formation_anchor",
			Name:     "FormationAnchor",
			MapIndex: 42,
			X:        1600,
			Y:        2800,
			RaceNum:  20350,
		}},
	}); err != nil {
		t.Fatalf("import formation anchor: %v", err)
	}
	before, ok := runtime.SpawnGroupByRef("practice.formation_anchor")
	if !ok || before.X != 1600 || before.Y != 2800 {
		t.Fatalf("expected formation anchor before rejected imports, ok=%v snapshot=%+v", ok, before)
	}

	rejected := []contentbundle.RegenSpawn{
		{
			Ref:       "practice.one_count_line",
			Name:      "OneCountLine",
			MapIndex:  42,
			X:         2000,
			Y:         2800,
			RaceNum:   20350,
			Count:     1,
			Formation: "line",
		},
		{
			Ref:         "practice.unknown_formation",
			Name:        "UnknownFormation",
			MapIndex:    42,
			X:           2100,
			Y:           2800,
			RaceNum:     20350,
			Count:       2,
			PackSpacing: 100,
			Formation:   "wedge",
		},
		{
			Ref:         "practice.overflow_formation",
			Name:        "OverflowFormation",
			MapIndex:    42,
			X:           2200,
			Y:           2147483647 - 10,
			RaceNum:     20350,
			Count:       2,
			PackSpacing: 100,
			Formation:   "line",
		},
	}
	for _, regen := range rejected {
		if _, err := runtime.ImportContentBundle(contentbundle.Bundle{RegenSpawns: []contentbundle.RegenSpawn{regen}}); !errors.Is(err, contentbundle.ErrInvalidBundle) {
			t.Fatalf("expected ErrInvalidBundle for %s, got %v", regen.Ref, err)
		}
		if _, ok := runtime.SpawnGroupByRef(regen.Ref); ok {
			t.Fatalf("expected rejected formation %s not to materialize", regen.Ref)
		}
		if _, ok := runtime.SpawnGroupByRef(regen.Ref + ".m01"); ok {
			t.Fatalf("expected rejected formation %s not to materialize a pack member", regen.Ref)
		}
	}
	after, ok := runtime.SpawnGroupByRef("practice.formation_anchor")
	if !ok || after.X != before.X || after.Y != before.Y || after.EntityID != before.EntityID {
		t.Fatalf("expected rejected formation imports to leave the anchor unmoved, ok=%v before=%+v after=%+v", ok, before, after)
	}
}

func newPackFormationRuntime(t *testing.T) *gameRuntime {
	t.Helper()
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PackFormationOwner", 0x01030661, 0x02040661, 1850, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "pack-formation-owner", 0xf6f6f6f6, owner)

	currentTime := time.Unix(1700005000, 0)
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(
		config.Service{
			LegacyAddr:           ":13000",
			PublicAddr:           "127.0.0.1",
			VisibilityMode:       "radius",
			VisibilityRadius:     400,
			VisibilitySectorSize: 200,
		},
		store,
		nil,
		staticstore.NewMemoryStore(),
		interactionstore.NewMemoryStore(),
	)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	return runtime
}
