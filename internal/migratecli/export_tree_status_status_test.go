package migratecli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type exportTreeStatusStatusGot struct {
	Format                 string            `json:"format"`
	Present                bool              `json:"present"`
	ExportTreeStatusSHA256 string            `json:"export_tree_status_sha256,omitempty"`
	Status                 *exportTreeStatus `json:"status,omitempty"`
}

func TestRunExportTreeStatusStatusReportsMissingWithoutOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	missing := filepath.Join(t.TempDir(), "missing-export-tree-status.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"export-tree-status-status", "--export-tree-status", missing}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected missing export-tree-status-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected missing export-tree-status-status not to write stderr, got %q", stderr.String())
	}
	var got exportTreeStatusStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode missing export-tree-status-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != "go-metin2-export-tree-status-status-v1" || got.Present || got.Status != nil || got.ExportTreeStatusSHA256 != "" {
		t.Fatalf("unexpected missing export-tree-status-status: %#v", got)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("export-tree-status-status must not open a database target, got events %#v", events)
	}
}

func TestRunExportTreeStatusStatusReadsValidAbsentInnerSnapshot(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	missingTree := filepath.Join(t.TempDir(), "missing-export-tree")
	raw := mustCaptureExportTreeStatusJSON(t, missingTree)
	statusPath := filepath.Join(t.TempDir(), "export-tree-status-before.json")
	mustWriteFile(t, statusPath, raw)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export-tree-status-status", "--export-tree-status", statusPath}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected absent-inner export-tree-status-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on absent-inner success, got %q", stderr.String())
	}
	var got exportTreeStatusStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode absent-inner export-tree-status-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != "go-metin2-export-tree-status-status-v1" || !got.Present || got.Status == nil {
		t.Fatalf("unexpected absent-inner envelope: %#v", got)
	}
	if got.ExportTreeStatusSHA256 != sha256Hex(raw) {
		t.Fatalf("unexpected export_tree_status_sha256: got %s want %s", got.ExportTreeStatusSHA256, sha256Hex(raw))
	}
	if got.Status.Format != exportTreeStatusFormat || got.Status.Present || got.Status.ExportTree != "" || len(got.Status.Kinds) != 0 {
		t.Fatalf("unexpected inner absent snapshot: %#v", got.Status)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("export-tree-status-status must not open a database target, got events %#v", events)
	}
}

func TestRunExportTreeStatusStatusReadsValidPresentSnapshotWithoutWalkingTree(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	tree := filepath.Join(t.TempDir(), "exports", "20260905T120000Z-abcdef012345")
	if err := os.MkdirAll(tree, 0o755); err != nil {
		t.Fatalf("mkdir export-tree: %v", err)
	}
	raw := mustCaptureExportTreeStatusJSON(t, tree)
	if err := os.RemoveAll(tree); err != nil {
		t.Fatalf("remove original export-tree: %v", err)
	}
	statusPath := filepath.Join(t.TempDir(), "export-tree-status-before.json")
	mustWriteFile(t, statusPath, raw)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export-tree-status-status", "--export-tree-status", statusPath}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected present export-tree-status-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var got exportTreeStatusStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode present export-tree-status-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != "go-metin2-export-tree-status-status-v1" || !got.Present || got.Status == nil || got.ExportTreeStatusSHA256 != sha256Hex(raw) {
		t.Fatalf("unexpected present envelope: %#v", got)
	}
	if got.Status.Format != exportTreeStatusFormat || !got.Status.Present || got.Status.KindCount != len(exportQuarantineKinds) {
		t.Fatalf("unexpected inner present snapshot: %#v", got.Status)
	}
	body := stdout.String()
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "memory://", "postgres://", "password="} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("export-tree-status-status must not expose %q, got %s", forbidden, body)
		}
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("export-tree-status-status must not open a database target, got events %#v", events)
	}
}

func TestRunExportTreeStatusStatusReadsCompleteAllReplacedTwoPhaseSnapshot(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	tree := filepath.Join(t.TempDir(), "exports", "20260905T190000Z-abcdef012345")
	mustMaterializeEmptyExportTreeStatusFixtures(t, tree)
	mustWriteExportTreeWipeArtifacts(t, tree)
	mustMaterializeEmptyExportTreeImportResultFixtures(t, tree)
	mustRewriteAllExportTreeImportResultsReplaced(t, tree)
	mustRewriteAllExportTreeWipeImportResultsReplaced(t, tree)

	raw := mustCaptureExportTreeStatusJSON(t, tree)
	if err := os.RemoveAll(tree); err != nil {
		t.Fatalf("remove original export-tree: %v", err)
	}
	statusPath := filepath.Join(t.TempDir(), "export-tree-status-after.json")
	mustWriteFile(t, statusPath, raw)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export-tree-status-status", "--export-tree-status", statusPath}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected complete two-phase export-tree-status-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var got exportTreeStatusStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode complete two-phase export-tree-status-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Status == nil || !got.Status.Present || !got.Status.QuarantineComplete || !got.Status.TwoPhaseWipeArtifactsComplete || !got.Status.ImportResultArtifactsComplete || !got.Status.WipeImportArtifactsComplete || !got.Status.ImportResultOutcomesComplete || !got.Status.ImportResultAllReplaced || !got.Status.WipeImportResultOutcomesComplete || !got.Status.WipeImportResultAllReplaced {
		t.Fatalf("expected complete all-replaced two-phase inner aggregates, got %#v", got.Status)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("export-tree-status-status must not open a database target, got events %#v", events)
	}
}

func TestRunExportTreeStatusStatusRejectsInconsistentInnerSnapshots(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	tree := filepath.Join(t.TempDir(), "exports", "20260905T191000Z-abcdef012345")
	if err := os.MkdirAll(tree, 0o755); err != nil {
		t.Fatalf("mkdir export-tree: %v", err)
	}
	raw := mustCaptureExportTreeStatusJSON(t, tree)
	var base exportTreeStatus
	if err := json.Unmarshal(raw, &base); err != nil {
		t.Fatalf("decode captured export-tree-status: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*exportTreeStatus)
		want   string
	}{
		{
			name: "kind-order",
			mutate: func(status *exportTreeStatus) {
				status.Kinds[0], status.Kinds[1] = status.Kinds[1], status.Kinds[0]
			},
			want: "kind",
		},
		{
			name: "wipe-kind",
			mutate: func(status *exportTreeStatus) {
				status.Kinds[0].WipeKind = !status.Kinds[0].WipeKind
			},
			want: "wipe_kind",
		},
		{
			name: "aggregate-drift",
			mutate: func(status *exportTreeStatus) {
				status.QuarantineComplete = true
			},
			want: "quarantine_complete",
		},
		{
			name: "missing-wipe-pointer",
			mutate: func(status *exportTreeStatus) {
				for i := range status.Kinds {
					if status.Kinds[i].WipeKind {
						status.Kinds[i].WipeQuarantine = nil
						return
					}
				}
			},
			want: "wipe_quarantine",
		},
		{
			name: "outcome-without-artifact",
			mutate: func(status *exportTreeStatus) {
				status.Kinds[0].ImportResultOutcome = &exportTreeImportResultOutcome{Replaced: true, RowCount: 1}
			},
			want: "import_result_outcome",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status := base
			status.Kinds = append([]exportTreeKindStatus(nil), base.Kinds...)
			tc.mutate(&status)
			mutated, err := json.Marshal(status)
			if err != nil {
				t.Fatalf("marshal mutated status: %v", err)
			}
			statusPath := filepath.Join(t.TempDir(), "export-tree-status.json")
			mustWriteFile(t, statusPath, mutated)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run([]string{"export-tree-status-status", "--export-tree-status", statusPath}, nil, &stdout, &stderr)
			if code != exitError {
				t.Fatalf("expected inconsistent snapshot to fail closed, exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout on inconsistent snapshot, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("expected stderr to mention %q, got %q", tc.want, stderr.String())
			}
		})
	}
}

func TestRunExportTreeStatusStatusRequireFlagsFailClosedOnAbsentPathOrInnerPresentFalse(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	missing := filepath.Join(t.TempDir(), "missing-export-tree-status.json")
	absentInner := mustCaptureExportTreeStatusJSON(t, filepath.Join(t.TempDir(), "missing-export-tree"))
	absentPath := filepath.Join(t.TempDir(), "export-tree-status-before.json")
	mustWriteFile(t, absentPath, absentInner)

	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing-path-require-quarantine",
			args: []string{"export-tree-status-status", "--export-tree-status", missing, "--require-quarantine-complete"},
			want: "--require-quarantine-complete",
		},
		{
			name: "inner-absent-require-import-outcomes",
			args: []string{"export-tree-status-status", "--export-tree-status", absentPath, "--require-import-result-outcomes-complete"},
			want: "--require-import-result-outcomes-complete",
		},
		{
			name: "inner-absent-require-wipe-all-replaced",
			args: []string{"export-tree-status-status", "--export-tree-status", absentPath, "--require-wipe-import-result-all-replaced"},
			want: "--require-wipe-import-result-all-replaced",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(tc.args, nil, &stdout, &stderr)
			if code != exitError {
				t.Fatalf("expected require-gate to fail closed, exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout on require-gate failure, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("expected stderr to contain %q, got %q", tc.want, stderr.String())
			}
		})
	}
}

func TestRunExportTreeStatusStatusWipeOutcomeRequireFailsOnInsertOnlyAfterSnapshot(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	tree := filepath.Join(t.TempDir(), "exports", "20260905T192000Z-abcdef012345")
	mustMaterializeEmptyExportTreeStatusFixtures(t, tree)
	mustMaterializeEmptyExportTreeImportResultFixtures(t, tree)
	for _, kind := range importExportDrillWipeKinds {
		if err := os.Remove(filepath.Join(tree, kind, "wipe-import-result.json")); err != nil {
			t.Fatalf("remove wipe-import-result for %s: %v", kind, err)
		}
		if err := os.Remove(filepath.Join(tree, kind, "wipe-import-result-status.json")); err != nil {
			t.Fatalf("remove wipe-import-result-status for %s: %v", kind, err)
		}
	}
	raw := mustCaptureExportTreeStatusJSON(t, tree)
	statusPath := filepath.Join(t.TempDir(), "export-tree-status-after.json")
	mustWriteFile(t, statusPath, raw)

	for _, flagName := range []string{
		"--require-wipe-import-result-outcomes-complete",
		"--require-wipe-import-result-all-replaced",
	} {
		t.Run(flagName, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run([]string{"export-tree-status-status", "--export-tree-status", statusPath, flagName}, nil, &stdout, &stderr)
			if code != exitError {
				t.Fatalf("expected insert-only after snapshot to fail %s, exit=%d stderr=%q", flagName, code, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout on wipe-outcome require failure, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), flagName) {
				t.Fatalf("expected stderr to name %s, got %q", flagName, stderr.String())
			}
		})
	}
}

func TestRunExportTreeStatusStatusRejectsSymlinkOversizedUnknownFieldAndWrongFormat(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	dir := t.TempDir()

	targetPath := filepath.Join(dir, "target-export-tree-status.json")
	mustWriteFile(t, targetPath, []byte("{}\n"))
	symlinkPath := filepath.Join(dir, "export-tree-status.json")
	if err := os.Symlink(targetPath, symlinkPath); err != nil {
		t.Fatalf("create symlink export-tree-status: %v", err)
	}

	oversizedPath := filepath.Join(dir, "oversized-export-tree-status.json")
	mustWriteFile(t, oversizedPath, bytes.Repeat([]byte("a"), 128*1024+1))

	unknownFieldPath := filepath.Join(dir, "unknown-field-export-tree-status.json")
	mustWriteFile(t, unknownFieldPath, []byte(`{"format":"go-metin2-export-tree-status-v1","present":false,"extra":true}`+"\n"))

	wrongFormatPath := filepath.Join(dir, "wrong-format-export-tree-status.json")
	mustWriteFile(t, wrongFormatPath, []byte(`{"format":"go-metin2-export-tree-status-status-v1","present":false}`+"\n"))

	cases := []struct {
		name string
		path string
		want string
	}{
		{name: "symlink", path: symlinkPath, want: "must not be a symlink"},
		{name: "oversized", path: oversizedPath, want: "exceeds"},
		{name: "unknown-field", path: unknownFieldPath, want: "unknown field"},
		{name: "wrong-format", path: wrongFormatPath, want: "format"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run([]string{"export-tree-status-status", "--export-tree-status", tc.path}, nil, &stdout, &stderr)
			if code != exitError {
				t.Fatalf("expected %s to fail closed, exit=%d stdout=%q stderr=%q", tc.name, code, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout on %s rejection, got %q", tc.name, stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("expected stderr to contain %q, got %q", tc.want, stderr.String())
			}
			if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
				t.Fatalf("export-tree-status-status must not open a database target, got events %#v", events)
			}
		})
	}
}

func TestRunExportTreeStatusStatusRejectsUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing-flag", args: []string{"export-tree-status-status"}, want: "--export-tree-status"},
		{name: "unexpected-arg", args: []string{"export-tree-status-status", "--export-tree-status", "/tmp/x.json", "extra"}, want: "unexpected export-tree-status-status argument"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(tc.args, nil, &stdout, &stderr)
			if code != exitUsage {
				t.Fatalf("expected usage exit %d, got %d stderr=%q", exitUsage, code, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout on usage error, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("expected %q in stderr %q", tc.want, stderr.String())
			}
			if !strings.Contains(stderr.String(), "export-tree-status-status usage:") {
				t.Fatalf("expected export-tree-status-status usage guidance, got %q", stderr.String())
			}
		})
	}
}

func TestRunExportTreeStatusStatusUsageListsRequireFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export-tree-status-status"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d stderr=%q", exitUsage, code, stderr.String())
	}
	body := stderr.String()
	for _, want := range []string{
		"--export-tree-status",
		"--require-quarantine-complete",
		"--require-two-phase-wipe-artifacts-complete",
		"--require-import-result-artifacts-complete",
		"--require-wipe-import-artifacts-complete",
		"--require-import-result-outcomes-complete",
		"--require-import-result-all-replaced",
		"--require-wipe-import-result-outcomes-complete",
		"--require-wipe-import-result-all-replaced",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected usage to list %q, got %q", want, body)
		}
	}
}

func TestRunHelpListsExportTreeStatusStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"help"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected help exit 0, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "export-tree-status-status") {
		t.Fatalf("expected help to list export-tree-status-status, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "export-tree-status-status usage:") {
		t.Fatalf("expected export-tree-status-status usage block, got %q", stdout.String())
	}
}

func TestRunRejectsUnknownCommandMentionsExportTreeStatusStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"not-a-real-command"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d", exitUsage, code)
	}
	if !strings.Contains(stderr.String(), "export-tree-status-status") {
		t.Fatalf("expected usage to mention export-tree-status-status, got %q", stderr.String())
	}
}

func mustCaptureExportTreeStatusJSON(t *testing.T, tree string) []byte {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export-tree-status", "--export-tree", tree}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("export-tree-status capture: exit=%d stderr=%q", code, stderr.String())
	}
	return stdout.Bytes()
}

func mustWriteExportTreeWipeArtifacts(t *testing.T, tree string) {
	t.Helper()
	wipePayloads := map[string]string{
		"character-item-state":         `{"migration_version":3,"migration_name":"character_item_state","character_ids":[11],"inventory_items":[],"equipment_items":[],"quickslots":[]}`,
		"character-point-state":        `{"migration_version":11,"migration_name":"character_point_state","character_ids":[11],"points":[]}`,
		"character-myshop-unit-prices": `{"migration_version":23,"migration_name":"character_myshop_unit_prices","character_ids":[11],"unit_prices":[]}`,
		"character-quest-state":        `{"migration_version":4,"migration_name":"character_quest_state","character_ids":[11],"flags":[]}`,
		"character-safebox-state":      `{"migration_version":15,"migration_name":"character_safebox_money","character_ids":[11],"passwords":[],"items":[]}`,
		"bootstrap-ground-item-state":  `{"migration_version":10,"migration_name":"bootstrap_ground_item_state","vids":[117440556],"ground_items":[]}`,
	}
	for kind, payload := range wipePayloads {
		wipePath := filepath.Join(tree, kind, "wipe-quarantine.json")
		mustWriteFile(t, wipePath, []byte(payload))
		var wipeStdout bytes.Buffer
		var wipeStderr bytes.Buffer
		code := Run([]string{"synthesize-wipe-export-status", "--kind", kind, "--wipe-export", wipePath}, nil, &wipeStdout, &wipeStderr)
		if code != exitOK {
			t.Fatalf("synthesize-wipe-export-status for %s: exit=%d stderr=%q", kind, code, wipeStderr.String())
		}
		mustWriteFile(t, filepath.Join(tree, kind, "wipe-quarantine-status.json"), wipeStdout.Bytes())
	}
}

func mustRewriteAllExportTreeImportResultsReplaced(t *testing.T, tree string) {
	t.Helper()
	replacedPayloads := map[string]string{
		"account-character-roster":     `{"migration_version":2,"migration_name":"account_character_roster","account_count":1,"character_count":2,"account_ids":[7],"character_ids":[11,12],"replaced":true}`,
		"character-item-state":         `{"migration_version":3,"migration_name":"character_item_state","character_count":1,"inventory_item_count":3,"equipment_item_count":1,"quickslot_count":2,"character_ids":[11],"replaced":true}`,
		"character-point-state":        `{"migration_version":11,"migration_name":"character_point_state","character_count":1,"point_row_count":4,"character_ids":[11],"replaced":true}`,
		"character-myshop-unit-prices": `{"migration_version":23,"migration_name":"character_myshop_unit_prices","character_count":1,"price_row_count":5,"character_ids":[11],"replaced":true}`,
		"character-quest-state":        `{"migration_version":4,"migration_name":"character_quest_state","character_count":1,"flag_count":6,"character_ids":[11],"replaced":true}`,
		"character-safebox-state":      `{"migration_version":15,"migration_name":"character_safebox_money","character_count":1,"password_count":1,"item_count":7,"character_ids":[11],"replaced":true}`,
		"auth-login-ticket-handoff":    `{"migration_version":7,"migration_name":"auth_login_ticket_handoff","ticket_count":8,"active_ticket_count":1,"login_keys":[16909060],"replaced":true}`,
		"item-template-state":          `{"migration_version":9,"migration_name":"item_template_refine_info","template_count":9,"socket_count":0,"attribute_count":0,"use_effect_count":0,"equip_effect_count":0,"refine_info_count":0,"refine_material_count":0,"vnums":[19],"replaced":true}`,
		"static-actor-content-state":   `{"migration_version":13,"migration_name":"static_actor_combat_profile_state","interaction_definition_count":0,"merchant_catalog_entry_count":0,"quest_flag_reward_item_count":0,"quest_flag_consume_item_count":0,"static_actor_count":10,"reward_drop_count":0,"combat_profile_count":0,"combat_profile_death_reward_drop_count":0,"entity_ids":[101],"interaction_kinds":[],"combat_profiles":[],"replaced":true}`,
		"bootstrap-ground-item-state":  `{"migration_version":10,"migration_name":"bootstrap_ground_item_state","ground_item_count":11,"item_shaped_count":11,"gold_shaped_count":0,"vids":[117440556],"replaced":true}`,
	}
	for kind, payload := range replacedPayloads {
		mustRewriteExportTreeImportResult(t, tree, kind, payload)
	}
}

func mustRewriteAllExportTreeWipeImportResultsReplaced(t *testing.T, tree string) {
	t.Helper()
	replacedPayloads := map[string]string{
		"character-item-state":         `{"migration_version":3,"migration_name":"character_item_state","character_count":1,"inventory_item_count":3,"equipment_item_count":1,"quickslot_count":2,"character_ids":[11],"replaced":true}`,
		"character-point-state":        `{"migration_version":11,"migration_name":"character_point_state","character_count":1,"point_row_count":4,"character_ids":[11],"replaced":true}`,
		"character-myshop-unit-prices": `{"migration_version":23,"migration_name":"character_myshop_unit_prices","character_count":1,"price_row_count":5,"character_ids":[11],"replaced":true}`,
		"character-quest-state":        `{"migration_version":4,"migration_name":"character_quest_state","character_count":1,"flag_count":6,"character_ids":[11],"replaced":true}`,
		"character-safebox-state":      `{"migration_version":15,"migration_name":"character_safebox_money","character_count":1,"password_count":1,"item_count":7,"character_ids":[11],"replaced":true}`,
		"bootstrap-ground-item-state":  `{"migration_version":10,"migration_name":"bootstrap_ground_item_state","ground_item_count":11,"item_shaped_count":11,"gold_shaped_count":0,"vids":[117440556],"replaced":true}`,
	}
	for kind, payload := range replacedPayloads {
		mustRewriteExportTreeWipeImportResult(t, tree, kind, payload)
	}
}
