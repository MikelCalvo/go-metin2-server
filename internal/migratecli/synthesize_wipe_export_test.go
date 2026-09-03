package migratecli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestRunSynthesizeWipeExportEmitsBareCharacterItemWipe(t *testing.T) {
	raw := `{
  "migration_version": 3,
  "migration_name": "character_item_state",
  "inventory_items": [
    {"id": 1001, "character_id": 11, "slot": 0, "vnum": 27001, "count": 1}
  ],
  "equipment_items": [],
  "quickslots": []
}`
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{"synthesize-wipe-export", "--kind", "character-item-state", "--export", "-"},
		strings.NewReader(raw),
		&stdout,
		&stderr,
	)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d stderr=%q", exitOK, code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}
	var wipe accountstore.CharacterItemStateExport
	if err := json.Unmarshal(stdout.Bytes(), &wipe); err != nil {
		t.Fatalf("decode wipe export: %v body=%s", err, stdout.String())
	}
	if wipe.MigrationVersion != accountstore.CharacterItemStateMigrationVersion || wipe.MigrationName != accountstore.CharacterItemStateMigrationName {
		t.Fatalf("unexpected migration identity: %#v", wipe)
	}
	if len(wipe.CharacterIDs) != 1 || wipe.CharacterIDs[0] != 11 {
		t.Fatalf("unexpected character_ids: %#v", wipe.CharacterIDs)
	}
	if wipe.InventoryItems == nil || wipe.EquipmentItems == nil || wipe.Quickslots == nil {
		t.Fatalf("expected present empty row arrays, got %#v", wipe)
	}
	if len(wipe.InventoryItems) != 0 || len(wipe.EquipmentItems) != 0 || len(wipe.Quickslots) != 0 {
		t.Fatalf("expected empty wipe rows, got %#v", wipe)
	}
	if strings.Contains(stdout.String(), `"summary"`) {
		t.Fatalf("synthesize-wipe-export must emit bare export JSON, got %s", stdout.String())
	}
}

func TestRunSynthesizeWipeExportAcceptsWrappedQuarantineJSON(t *testing.T) {
	raw := `{
  "summary": {"character_count": 1, "inventory_item_count": 1, "equipment_item_count": 0, "quickslot_count": 0, "character_ids": [11]},
  "export": {
    "migration_version": 3,
    "migration_name": "character_item_state",
    "inventory_items": [
      {"id": 1001, "character_id": 11, "slot": 0, "vnum": 27001, "count": 1}
    ],
    "equipment_items": [],
    "quickslots": []
  }
}`
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{"synthesize-wipe-export", "--kind", "character-item-state", "--export", "-"},
		strings.NewReader(raw),
		&stdout,
		&stderr,
	)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d stderr=%q", exitOK, code, stderr.String())
	}
	var wipe accountstore.CharacterItemStateExport
	if err := json.Unmarshal(stdout.Bytes(), &wipe); err != nil {
		t.Fatalf("decode wipe export: %v body=%s", err, stdout.String())
	}
	if len(wipe.CharacterIDs) != 1 || wipe.CharacterIDs[0] != 11 {
		t.Fatalf("unexpected character_ids: %#v", wipe.CharacterIDs)
	}
}

func TestRunSynthesizeWipeExportEmitsGroundVIDWipe(t *testing.T) {
	raw := `{
  "migration_version": 10,
  "migration_name": "bootstrap_ground_item_state",
  "ground_items": [
    {
      "vid": 117440556,
      "vnum": 3001,
      "item_count": 1,
      "owner_login": "Alpha",
      "owner_character_id": 11,
      "owner_vid": 33816587,
      "owner_name": "AlphaWar",
      "map_index": 1,
      "x": 1100,
      "y": 2100,
      "z": 2,
      "pickup_range": 450
    }
  ]
}`
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{"synthesize-wipe-export", "--kind", "bootstrap-ground-item-state", "--export", "-"},
		strings.NewReader(raw),
		&stdout,
		&stderr,
	)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d stderr=%q", exitOK, code, stderr.String())
	}
	var wipe worldruntime.BootstrapGroundItemStateExport
	if err := json.Unmarshal(stdout.Bytes(), &wipe); err != nil {
		t.Fatalf("decode wipe export: %v body=%s", err, stdout.String())
	}
	if len(wipe.VIDs) != 1 || wipe.VIDs[0] != 117440556 {
		t.Fatalf("unexpected vids: %#v", wipe.VIDs)
	}
	if wipe.GroundItems == nil || len(wipe.GroundItems) != 0 {
		t.Fatalf("expected empty ground_items, got %#v", wipe.GroundItems)
	}
}

func TestRunSynthesizeWipeExportRejectsEmptyScope(t *testing.T) {
	raw := `{
  "migration_version": 3,
  "migration_name": "character_item_state",
  "inventory_items": [],
  "equipment_items": [],
  "quickslots": []
}`
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{"synthesize-wipe-export", "--kind", "character-item-state", "--export", "-"},
		strings.NewReader(raw),
		&stdout,
		&stderr,
	)
	if code != exitError {
		t.Fatalf("expected exit %d, got %d stderr=%q", exitError, code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on empty scope, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "wipe scope character_ids is empty") {
		t.Fatalf("expected empty-scope error, got %q", stderr.String())
	}
}

func TestRunSynthesizeWipeExportRejectsUnsupportedKind(t *testing.T) {
	raw := `{
  "migration_version": 2,
  "migration_name": "account_character_roster",
  "accounts": [],
  "characters": []
}`
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{"synthesize-wipe-export", "--kind", "account-character-roster", "--export", "-"},
		strings.NewReader(raw),
		&stdout,
		&stderr,
	)
	if code != exitUsage {
		t.Fatalf("expected exit %d, got %d stderr=%q", exitUsage, code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unsupported synthesize-wipe-export kind") {
		t.Fatalf("expected unsupported kind error, got %q", stderr.String())
	}
}

func TestRunSynthesizeWipeExportEmitsRemainingCharacterFKKinds(t *testing.T) {
	cases := []struct {
		kind  string
		raw   string
		check func(t *testing.T, body string)
	}{
		{
			kind: "character-point-state",
			raw: func() string {
				points := make([]map[string]any, 0, 255)
				for i := 0; i < 255; i++ {
					points = append(points, map[string]any{
						"character_id": 11,
						"point_index":  i,
						"value":        0,
					})
				}
				payload := map[string]any{
					"migration_version": 11,
					"migration_name":    "character_point_state",
					"points":            points,
				}
				raw, err := json.Marshal(payload)
				if err != nil {
					t.Fatalf("marshal point wipe fixture: %v", err)
				}
				return string(raw)
			}(),
			check: func(t *testing.T, body string) {
				t.Helper()
				var wipe accountstore.CharacterPointStateExport
				if err := json.Unmarshal([]byte(body), &wipe); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if len(wipe.CharacterIDs) != 1 || wipe.CharacterIDs[0] != 11 || len(wipe.Points) != 0 {
					t.Fatalf("unexpected wipe: %#v", wipe)
				}
			},
		},
		{
			kind: "character-myshop-unit-prices",
			raw: `{
  "migration_version": 23,
  "migration_name": "character_myshop_unit_prices",
  "unit_prices": [{"character_id": 11, "vnum": 19, "unit_price": 100}]
}`,
			check: func(t *testing.T, body string) {
				t.Helper()
				var wipe accountstore.CharacterMyShopUnitPricesExport
				if err := json.Unmarshal([]byte(body), &wipe); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if len(wipe.CharacterIDs) != 1 || wipe.CharacterIDs[0] != 11 || len(wipe.UnitPrices) != 0 {
					t.Fatalf("unexpected wipe: %#v", wipe)
				}
			},
		},
		{
			kind: "character-quest-state",
			raw: `{
  "migration_version": 4,
  "migration_name": "character_quest_state",
  "flags": [{"character_id": 11, "character": "AlphaWar", "quest_ref": "quest:first_steps", "flag": "step", "value": 1}]
}`,
			check: func(t *testing.T, body string) {
				t.Helper()
				var wipe queststate.CharacterQuestStateExport
				if err := json.Unmarshal([]byte(body), &wipe); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if len(wipe.CharacterIDs) != 1 || wipe.CharacterIDs[0] != 11 || len(wipe.Flags) != 0 {
					t.Fatalf("unexpected wipe: %#v", wipe)
				}
			},
		},
		{
			kind: "character-safebox-state",
			raw: `{
  "migration_version": 15,
  "migration_name": "character_safebox_money",
  "passwords": [{"character_id": 11, "login": "Alpha", "password": "000000", "money": 0}],
  "items": []
}`,
			check: func(t *testing.T, body string) {
				t.Helper()
				var wipe safeboxstore.CharacterSafeboxStateExport
				if err := json.Unmarshal([]byte(body), &wipe); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if len(wipe.CharacterIDs) != 1 || wipe.CharacterIDs[0] != 11 || len(wipe.Passwords) != 0 || len(wipe.Items) != 0 {
					t.Fatalf("unexpected wipe: %#v", wipe)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(
				[]string{"synthesize-wipe-export", "--kind", tc.kind, "--export", "-"},
				strings.NewReader(tc.raw),
				&stdout,
				&stderr,
			)
			if code != exitOK {
				t.Fatalf("expected exit %d, got %d stderr=%q", exitOK, code, stderr.String())
			}
			tc.check(t, stdout.String())
		})
	}
}

func TestRunRejectsUnknownCommandMentionsSynthesizeWipeExport(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"nope"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected exit %d, got %d", exitUsage, code)
	}
	if !strings.Contains(stderr.String(), "synthesize-wipe-export") {
		t.Fatalf("expected usage to mention synthesize-wipe-export, got %q", stderr.String())
	}
}
