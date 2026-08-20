package observability

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/buildinfo"
)

func TestNewServiceLoggerIncludesBuildIdentityAttrs(t *testing.T) {
	originalVersion := buildinfo.Version
	originalCommit := buildinfo.Commit
	originalBuildDate := buildinfo.BuildDate
	t.Cleanup(func() {
		buildinfo.Version = originalVersion
		buildinfo.Commit = originalCommit
		buildinfo.BuildDate = originalBuildDate
	})

	buildinfo.Version = "v0.1.0-test"
	buildinfo.Commit = "abcdef012345"
	buildinfo.BuildDate = "2026-08-20T15:30:45Z"

	var buf bytes.Buffer
	logger := NewServiceLogger("gamed", &buf)
	logger.Info("ops server listening", "addr", "127.0.0.1:6060")

	record := decodeLastJSONLog(t, buf.Bytes())
	if got := record["service"]; got != "gamed" {
		t.Fatalf("service = %v, want gamed", got)
	}
	if got := record["version"]; got != buildinfo.Version {
		t.Fatalf("version = %v, want %s", got, buildinfo.Version)
	}
	if got := record["commit"]; got != buildinfo.Commit {
		t.Fatalf("commit = %v, want %s", got, buildinfo.Commit)
	}
	if got := record["build_date"]; got != buildinfo.BuildDate {
		t.Fatalf("build_date = %v, want %s", got, buildinfo.BuildDate)
	}
	if got := record["addr"]; got != "127.0.0.1:6060" {
		t.Fatalf("addr = %v, want 127.0.0.1:6060", got)
	}
	if got := record["msg"]; got != "ops server listening" {
		t.Fatalf("msg = %v, want ops server listening", got)
	}
}

func TestNewServiceLoggerRedactsSensitiveAttributeKeys(t *testing.T) {
	var buf bytes.Buffer
	logger := NewServiceLogger("authd", &buf)
	logger.Error("open database failed",
		"dsn", "postgres://operator:s3cret@127.0.0.1:5432/metin2?sslmode=disable",
		"DB_DSN", "should-also-redact",
		"password", "hunter2",
		"login-key", uint32(42),
		"ticket", "raw-ticket-bytes",
		"secret", "top-secret",
		"api_key", "abcd",
		"token", "bearer-token",
		"addr", "127.0.0.1:6061",
		"err", "connection refused",
	)

	body := buf.String()
	for _, forbidden := range []string{
		"postgres://",
		"s3cret",
		"hunter2",
		"raw-ticket-bytes",
		"top-secret",
		"bearer-token",
		"should-also-redact",
		"abcd",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("log leaked sensitive material %q: %s", forbidden, body)
		}
	}

	record := decodeLastJSONLog(t, buf.Bytes())
	for _, key := range []string{"dsn", "DB_DSN", "password", "login-key", "ticket", "secret", "api_key", "token"} {
		if got := record[key]; got != "<redacted>" {
			t.Fatalf("%s = %v, want <redacted>", key, got)
		}
	}
	if got := record["addr"]; got != "127.0.0.1:6061" {
		t.Fatalf("addr = %v, want 127.0.0.1:6061", got)
	}
	if got := record["err"]; got != "connection refused" {
		t.Fatalf("err = %v, want connection refused", got)
	}
	if got := record["service"]; got != "authd" {
		t.Fatalf("service = %v, want authd", got)
	}
}

func TestIsSensitiveAttrKeyNormalizesSeparatorsAndCase(t *testing.T) {
	cases := map[string]bool{
		"dsn":       true,
		"DB_DSN":    true,
		"db-dsn":    true,
		"Login_Key": true,
		"login-key": true,
		"api key":   true,
		"addr":      false,
		"remote":    false,
		"phase":     false,
	}
	for key, want := range cases {
		if got := isSensitiveAttrKey(key); got != want {
			t.Fatalf("isSensitiveAttrKey(%q) = %v, want %v", key, got, want)
		}
	}
}

func TestRedactSensitiveAttrLeavesOrdinaryValues(t *testing.T) {
	attr := redactSensitiveAttr(nil, slog.String("remote_addr", "10.0.0.8:1234"))
	if attr.Value.String() != "10.0.0.8:1234" {
		t.Fatalf("unexpected attr %#v", attr)
	}
}

func decodeLastJSONLog(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace(raw), []byte("\n"))
	if len(lines) == 0 || len(bytes.TrimSpace(lines[len(lines)-1])) == 0 {
		t.Fatalf("expected JSON log output, got %q", raw)
	}
	var record map[string]any
	if err := json.Unmarshal(lines[len(lines)-1], &record); err != nil {
		t.Fatalf("decode log JSON: %v\nraw=%s", err, raw)
	}
	return record
}
