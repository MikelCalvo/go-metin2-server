package migratecli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
)

func TestRunQuarantineExportAcceptsEmptyValidExports(t *testing.T) {
	cases := []struct {
		kind    string
		payload string
		want    string
	}{
		{
			kind:    "account-character-roster",
			payload: `{"migration_version":2,"migration_name":"account_character_roster","accounts":[],"characters":[]}`,
			want:    `"account_count": 0`,
		},
		{
			kind:    "character-item-state",
			payload: `{"migration_version":3,"migration_name":"character_item_state","inventory_items":[],"equipment_items":[],"quickslots":[]}`,
			want:    `"inventory_item_count": 0`,
		},
		{
			kind:    "character-point-state",
			payload: `{"migration_version":11,"migration_name":"character_point_state","points":[]}`,
			want:    `"point_row_count": 0`,
		},
		{
			kind:    "character-myshop-unit-prices",
			payload: `{"migration_version":23,"migration_name":"character_myshop_unit_prices","unit_prices":[]}`,
			want:    `"price_row_count": 0`,
		},
		{
			kind:    "character-quest-state",
			payload: `{"migration_version":4,"migration_name":"character_quest_state","flags":[]}`,
			want:    `"flag_count": 0`,
		},
		{
			kind:    "character-safebox-state",
			payload: `{"migration_version":15,"migration_name":"character_safebox_money","passwords":[],"items":[]}`,
			want:    `"password_count": 0`,
		},
		{
			kind:    "auth-login-ticket-handoff",
			payload: `{"migration_version":7,"migration_name":"auth_login_ticket_handoff","tickets":[]}`,
			want:    `"ticket_count": 0`,
		},
		{
			kind:    "item-template-state",
			payload: `{"migration_version":9,"migration_name":"item_template_refine_info","templates":[],"sockets":[],"attributes":[],"use_effects":[],"equip_effects":[],"refine_infos":[],"refine_materials":[]}`,
			want:    `"template_count": 0`,
		},
		{
			kind:    "static-actor-content-state",
			payload: `{"migration_version":13,"migration_name":"static_actor_combat_profile_state","interaction_definitions":[],"merchant_catalog_entries":[],"quest_flag_reward_items":[],"quest_flag_consume_items":[],"static_actors":[],"reward_drops":[],"combat_profiles":[],"combat_profile_death_reward_drops":[]}`,
			want:    `"static_actor_count": 0`,
		},
		{
			kind:    "bootstrap-ground-item-state",
			payload: `{"migration_version":10,"migration_name":"bootstrap_ground_item_state","ground_items":[]}`,
			want:    `"ground_item_count": 0`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(
				[]string{"quarantine-export", "--kind", tc.kind, "--export", "-"},
				strings.NewReader(tc.payload),
				&stdout,
				&stderr,
			)
			if code != 0 {
				t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr, got %q", stderr.String())
			}
			body := stdout.String()
			if !strings.Contains(body, `"summary"`) || !strings.Contains(body, `"export"`) {
				t.Fatalf("expected summary/export JSON, got %s", body)
			}
			if !strings.Contains(body, tc.want) {
				t.Fatalf("expected %q in stdout %s", tc.want, body)
			}
			if strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "DROP TABLE") || strings.Contains(body, "memory://") {
				t.Fatalf("quarantine-export must not expose SQL or DSN text, got %s", body)
			}
		})
	}
}

func TestRunQuarantineExportReadsRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roster.json")
	payload := `{"migration_version":2,"migration_name":"account_character_roster","accounts":[],"characters":[]}`
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("write export: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{"quarantine-export", "--kind", "account-character-roster", "--export", path},
		nil,
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	var result accountstore.AccountCharacterRosterQuarantineResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v\nbody:\n%s", err, stdout.String())
	}
	if result.Summary.AccountCount != 0 || result.Summary.CharacterCount != 0 {
		t.Fatalf("unexpected summary %#v", result.Summary)
	}
	if result.Export.MigrationVersion != accountstore.AccountCharacterRosterMigrationVersion {
		t.Fatalf("unexpected export %#v", result.Export)
	}
}

func TestRunQuarantineExportRejectsWrongMigrationBoundary(t *testing.T) {
	cases := []struct {
		kind    string
		payload string
	}{
		{
			kind:    "account-character-roster",
			payload: `{"migration_version":3,"migration_name":"account_character_roster","accounts":[],"characters":[]}`,
		},
		{
			kind:    "character-item-state",
			payload: `{"migration_version":11,"migration_name":"character_item_state","inventory_items":[],"equipment_items":[],"quickslots":[]}`,
		},
		{
			kind:    "bootstrap-ground-item-state",
			payload: `{"migration_version":9,"migration_name":"bootstrap_ground_item_state","ground_items":[]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(
				[]string{"quarantine-export", "--kind", tc.kind, "--export", "-"},
				strings.NewReader(tc.payload),
				&stdout,
				&stderr,
			)
			if code != 1 {
				t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout on contract failure, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), "quarantine-export:") {
				t.Fatalf("expected quarantine-export error prefix, got %q", stderr.String())
			}
		})
	}
}

func TestRunQuarantineExportRejectsMalformedAndOversizedInput(t *testing.T) {
	t.Run("malformed json", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run(
			[]string{"quarantine-export", "--kind", "character-quest-state", "--export", "-"},
			strings.NewReader(`{"migration_version":4`),
			&stdout,
			&stderr,
		)
		if code != 1 {
			t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Fatalf("expected no stdout, got %q", stdout.String())
		}
	})

	t.Run("invalid utf8", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run(
			[]string{"quarantine-export", "--kind", "character-quest-state", "--export", "-"},
			bytes.NewReader([]byte{0xff, 0xfe, 0xfd}),
			&stdout,
			&stderr,
		)
		if code != 1 {
			t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Fatalf("expected no stdout, got %q", stdout.String())
		}
		if !strings.Contains(stderr.String(), "UTF-8") {
			t.Fatalf("expected UTF-8 guidance, got %q", stderr.String())
		}
	})

	t.Run("oversized", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		oversized := strings.Repeat("a", maxExportQuarantineBytes+1)
		code := Run(
			[]string{"quarantine-export", "--kind", "character-quest-state", "--export", "-"},
			strings.NewReader(oversized),
			&stdout,
			&stderr,
		)
		if code != 1 {
			t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Fatalf("expected no stdout, got %q", stdout.String())
		}
		if !strings.Contains(stderr.String(), "exceeds") {
			t.Fatalf("expected oversized guidance, got %q", stderr.String())
		}
	})

	t.Run("symlink", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "roster.json")
		link := filepath.Join(dir, "roster-link.json")
		if err := os.WriteFile(target, []byte(`{"migration_version":2,"migration_name":"account_character_roster","accounts":[],"characters":[]}`), 0o600); err != nil {
			t.Fatalf("write target: %v", err)
		}
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run(
			[]string{"quarantine-export", "--kind", "account-character-roster", "--export", link},
			nil,
			&stdout,
			&stderr,
		)
		if code != 1 {
			t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Fatalf("expected no stdout, got %q", stdout.String())
		}
		if !strings.Contains(stderr.String(), "symlink") {
			t.Fatalf("expected symlink guidance, got %q", stderr.String())
		}
	})
}

func TestRunQuarantineExportRejectsUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing flags",
			args: []string{"quarantine-export"},
			want: "--kind and --export are required",
		},
		{
			name: "unknown kind",
			args: []string{"quarantine-export", "--kind", "not-a-kind", "--export", "-"},
			want: "unsupported quarantine-export kind",
		},
		{
			name: "extra args",
			args: []string{"quarantine-export", "--kind", "character-quest-state", "--export", "-", "extra"},
			want: "unexpected quarantine-export argument",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(tc.args, strings.NewReader(`{}`), &stdout, &stderr)
			if code != 2 {
				t.Fatalf("expected usage exit 2, got %d stderr=%q", code, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("expected %q in stderr %q", tc.want, stderr.String())
			}
			if !strings.Contains(stderr.String(), "quarantine-export usage:") {
				t.Fatalf("expected quarantine-export usage guidance, got %q", stderr.String())
			}
		})
	}
}

func TestRunRejectsUnknownCommandMentionsQuarantineExport(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"frobnicate"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected usage exit 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "quarantine-export") {
		t.Fatalf("expected usage to list quarantine-export, got %q", stderr.String())
	}
}
