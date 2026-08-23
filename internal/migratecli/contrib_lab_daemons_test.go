package migratecli

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestContribLabDaemonSamplesStayDisabledByDefault(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test caller path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	base := filepath.Join(repoRoot, "contrib", "lab-daemons")

	readmePath := filepath.Join(base, "README.md")
	authdEnvPath := filepath.Join(base, "env", "metin2-authd.env.sample")
	gamedEnvPath := filepath.Join(base, "env", "metin2-gamed.env.sample")
	rcConfPath := filepath.Join(base, "rc.d", "rc.conf.sample")
	authdRCPath := filepath.Join(base, "rc.d", "authd.sample")
	gamedRCPath := filepath.Join(base, "rc.d", "gamed.sample")
	authdServicePath := filepath.Join(base, "systemd", "authd.service.sample")
	gamedServicePath := filepath.Join(base, "systemd", "gamed.service.sample")
	authdDropInPath := filepath.Join(base, "systemd", "authd.service.d", "lab-store.conf.sample")
	gamedDropInPath := filepath.Join(base, "systemd", "gamed.service.d", "lab-store.conf.sample")

	readme := mustReadContribSample(t, readmePath)
	authdEnv := mustReadContribSample(t, authdEnvPath)
	gamedEnv := mustReadContribSample(t, gamedEnvPath)
	rcConf := mustReadContribSample(t, rcConfPath)
	authdRC := mustReadContribSample(t, authdRCPath)
	gamedRC := mustReadContribSample(t, gamedRCPath)
	authdService := mustReadContribSample(t, authdServicePath)
	gamedService := mustReadContribSample(t, gamedServicePath)
	authdDropIn := mustReadContribSample(t, authdDropInPath)
	gamedDropIn := mustReadContribSample(t, gamedDropInPath)

	for _, want := range []string{
		`authd_enable="NO"`,
		`gamed_enable="NO"`,
	} {
		if !strings.Contains(rcConf, want) {
			t.Fatalf("rc.conf sample missing %q:\n%s", want, rcConf)
		}
	}
	for _, forbidden := range []string{
		`authd_enable="YES"`,
		`gamed_enable="YES"`,
	} {
		if strings.Contains(rcConf, forbidden) {
			t.Fatalf("rc.conf sample must not contain %q:\n%s", forbidden, rcConf)
		}
	}
	assertNoForbiddenLabDaemonMarkers(t, "rc.conf", rcConf, nil)

	for _, tc := range []struct {
		label   string
		body    string
		bin     string
		rcvar   string
		enable  string
		envFile string
	}{
		{label: "rc.d/authd", body: authdRC, bin: "/usr/local/bin/authd", rcvar: "authd_enable", enable: `: "${authd_enable:=NO}"`, envFile: "/etc/metin2/metin2-authd.env"},
		{label: "rc.d/gamed", body: gamedRC, bin: "/usr/local/bin/gamed", rcvar: "gamed_enable", enable: `: "${gamed_enable:=NO}"`, envFile: "/etc/metin2/metin2-gamed.env"},
	} {
		for _, want := range []string{
			tc.bin,
			"rcvar=",
			"load_rc_config",
			"/etc/rc.subr",
			tc.enable,
			tc.envFile,
			"/usr/sbin/daemon",
		} {
			if !strings.Contains(tc.body, want) {
				t.Fatalf("%s missing %q:\n%s", tc.label, want, tc.body)
			}
		}
		if !strings.Contains(tc.body, tc.rcvar) {
			t.Fatalf("%s missing rcvar %q:\n%s", tc.label, tc.rcvar, tc.body)
		}
		assertNoForbiddenLabDaemonMarkers(t, tc.label, tc.body, map[string]struct{}{
			"# markers. Ops stay loopback-only. Stop uses SIGTERM via rc.subr (no store wipe).": {},
		})
	}

	for _, tc := range []struct {
		label string
		body  string
		bin   string
	}{
		{label: "systemd/authd", body: authdService, bin: "/usr/local/bin/authd"},
		{label: "systemd/gamed", body: gamedService, bin: "/usr/local/bin/gamed"},
	} {
		for _, want := range []string{
			"ExecStart=" + tc.bin,
			"KillSignal=SIGTERM",
			"RequiresMountsFor=/var/metin2",
			"Type=simple",
			"WantedBy=multi-user.target",
		} {
			if !strings.Contains(tc.body, want) {
				t.Fatalf("%s missing %q:\n%s", tc.label, want, tc.body)
			}
		}
		for _, line := range strings.Split(tc.body, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			if strings.HasPrefix(trimmed, "Environment=") {
				t.Fatalf("%s must not set Environment=: %q", tc.label, trimmed)
			}
			if strings.Contains(trimmed, "metin2-migrate") {
				t.Fatalf("%s must not ExecStart metin2-migrate: %q", tc.label, trimmed)
			}
		}
		assertNoForbiddenLabDaemonMarkers(t, tc.label, tc.body, map[string]struct{}{
			"# Explicit non-goals: no Environment=DSN, no metin2-migrate apply, no store wipe.": {},
		})
	}

	for _, tc := range []struct {
		label string
		body  string
		env   string
	}{
		{label: "authd-drop-in", body: authdDropIn, env: "/etc/metin2/metin2-authd.env"},
		{label: "gamed-drop-in", body: gamedDropIn, env: "/etc/metin2/metin2-gamed.env"},
	} {
		if !strings.Contains(tc.body, "EnvironmentFile=") {
			t.Fatalf("%s must set EnvironmentFile=, got:\n%s", tc.label, tc.body)
		}
		if !strings.Contains(tc.body, tc.env) {
			t.Fatalf("%s must point at %q, got:\n%s", tc.label, tc.env, tc.body)
		}
		assertNoForbiddenLabDaemonMarkers(t, tc.label, tc.body, nil)
	}

	for _, want := range []string{
		"METIN2_LOGIN_TICKET_STORE_DIR=/var/metin2/data/login-tickets",
		"METIN2_ACCOUNT_STORE_DIR=/var/metin2/data/accounts",
		"METIN2_AUTHD_PPROF_ADDR=127.0.0.1:6061",
		"METIN2_AUTHD_LEGACY_ADDR=:11002",
	} {
		if !strings.Contains(authdEnv, want) {
			t.Fatalf("authd env sample missing %q:\n%s", want, authdEnv)
		}
	}
	for _, want := range []string{
		"METIN2_LOGIN_TICKET_STORE_DIR=/var/metin2/data/login-tickets",
		"METIN2_ACCOUNT_STORE_DIR=/var/metin2/data/accounts",
		"METIN2_GAMED_STATIC_ACTOR_STORE_PATH=/var/metin2/data/static-actors/static-actors.json",
		"METIN2_GAMED_INTERACTION_STORE_PATH=/var/metin2/data/interactions/interaction-definitions.json",
		"METIN2_GAMED_ITEM_TEMPLATE_STORE_PATH=/var/metin2/data/item-templates/item-templates.json",
		"METIN2_GAMED_QUEST_STATE_STORE_PATH=/var/metin2/data/quest-state/quest-state.json",
		"METIN2_GAMED_GROUND_ITEM_STORE_PATH=/var/metin2/data/ground-items/ground-items.json",
		"METIN2_GAMED_SAFEBOX_STORE_PATH=/var/metin2/data/safebox/safebox.json",
		"METIN2_GAMED_PPROF_ADDR=127.0.0.1:6060",
		"METIN2_GAMED_LEGACY_ADDR=:13000",
	} {
		if !strings.Contains(gamedEnv, want) {
			t.Fatalf("gamed env sample missing %q:\n%s", want, gamedEnv)
		}
	}
	for _, body := range []string{authdEnv, gamedEnv} {
		assertNoForbiddenLabDaemonMarkers(t, "env-sample", body, map[string]struct{}{
			"# Never put DSNs, passwords, login keys, or executable SQL here.":                 {},
			"# Ops stay loopback-only; do not bind 0.0.0.0 / :: / public hostnames for pprof.": {},
		})
	}

	for _, want := range []string{
		"disabled-by-default",
		"authd_enable",
		"gamed_enable",
		"systemctl enable --now",
		"docs/workflow/lab-daemon-unit-samples.md",
		"docs/workflow/lab-deployment-topology.md",
		"contrib/lab-retention-gc/",
		"/usr/local/bin/authd",
		"/usr/local/bin/gamed",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("contrib README missing %q", want)
		}
	}
	assertNoForbiddenLabDaemonMarkers(t, "readme", readme, map[string]struct{}{
		"3. Ops stay loopback-only (`127.0.0.1:6061` / `127.0.0.1:6060`); never bind":    {},
		"   `0.0.0.0`, `::`, or a public hostname for pprof/ops.":                        {},
		"4. Never embed DSNs, passwords, login keys, or executable SQL in unit files,":   {},
		"5. Never `ExecStart` `metin2-migrate` apply / backup / GC / aside-rename from":  {},
		"6. Never pipe unit output into `/bin/sh`, `bash`, `csh`, or `zsh`.":             {},
		"- automatic / scheduled artifact GC deletion (see `contrib/lab-retention-gc/`)": {},
	})
}

func assertNoForbiddenLabDaemonMarkers(t *testing.T, label, body string, allowExact map[string]struct{}) {
	t.Helper()
	forbidden := []string{
		"| /bin/sh",
		"|/bin/sh",
		"| /bin/bash",
		"|/bin/bash",
		"| bash",
		"|bash",
		"find -delete",
		"unlink ",
		"rmdir ",
		".gc-aside-",
		"DROP TABLE",
		"CREATE TABLE",
		"METIN2_DB_DSN",
		"METIN2_GAMED_DB_DSN",
		"METIN2_AUTHD_DB_DSN",
		"password=",
		"0.0.0.0",
		"[::]",
		"curl ",
	}
	for _, marker := range forbidden {
		if !strings.Contains(body, marker) {
			continue
		}
		if allowExact != nil {
			allowed := false
			for exact := range allowExact {
				if strings.Contains(exact, marker) && strings.Contains(body, exact) {
					allowed = true
					break
				}
			}
			if allowed {
				continue
			}
		}
		t.Fatalf("%s contains forbidden marker %q", label, marker)
	}
}
