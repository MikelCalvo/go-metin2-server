package observability

import (
	"bytes"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"
)

func TestStartupLogSinkMissingConfigStaysOnStdout(t *testing.T) {
	t.Setenv("METIN2_LOG_SINK", "")
	t.Setenv("METIN2_GAMED_LOG_SINK", "")
	t.Setenv("METIN2_AUTHD_LOG_SINK", "")
	SetRemoteLogSink(nil)
	t.Cleanup(func() { SetRemoteLogSink(nil) })

	var stdout bytes.Buffer
	logger, err := NewStartupServiceLogger("gamed", &stdout)
	if err != nil {
		t.Fatalf("missing sink config: %v", err)
	}
	if RemoteLogConfigured() {
		t.Fatal("missing sink config must stay disabled")
	}
	logger.Info("ops server listening", "addr", "127.0.0.1:6060")
	record := decodeLastJSONLog(t, stdout.Bytes())
	if record["service"] != "gamed" || record["msg"] != "ops server listening" {
		t.Fatalf("stdout record = %#v", record)
	}
	if strings.Contains(stdout.String(), "syslog") || strings.Contains(stdout.String(), "siem") {
		t.Fatalf("stdout named a remote system: %s", stdout.String())
	}
}

func TestStartupLogSinkCopiesRedactedJSONToLoopbackUDP(t *testing.T) {
	conn := listenLoopbackUDP(t, net.ParseIP("127.0.0.1"))
	t.Setenv("METIN2_LOG_SINK", "10.1.2.3:514")
	t.Setenv("METIN2_AUTHD_LOG_SINK", conn.LocalAddr().String())
	t.Setenv("METIN2_GAMED_LOG_SINK", "")
	SetRemoteLogSink(nil)
	t.Cleanup(func() { SetRemoteLogSink(nil) })

	var stdout bytes.Buffer
	logger, err := NewStartupServiceLogger("authd", &stdout)
	if err != nil {
		t.Fatalf("startup sink: %v", err)
	}
	if !RemoteLogConfigured() {
		t.Fatal("loopback startup sink should be configured")
	}
	logger.Error("open database failed",
		"dsn", "postgres://operator:***@10.1.2.3:5432/metin2",
		"password", "hunter2",
		"addr", "127.0.0.1:6061",
	)

	payload := readLoopbackUDP(t, conn)
	if bytes.Contains(payload, []byte("hunter2")) || bytes.Contains(payload, []byte("postgres://")) || bytes.Contains(payload, []byte(conn.LocalAddr().String())) {
		t.Fatalf("sink leaked secret or endpoint: %s", payload)
	}
	var record map[string]any
	if err := json.Unmarshal(payload, &record); err != nil {
		t.Fatalf("decode sink JSON: %v\n%s", err, payload)
	}
	if record["service"] != "authd" || record["msg"] != "open database failed" {
		t.Fatalf("sink record = %#v", record)
	}
	if record["dsn"] != "<redacted>" || record["password"] != "<redacted>" {
		t.Fatalf("redaction = %#v", record)
	}
	stdoutRecord := decodeLastJSONLog(t, stdout.Bytes())
	if stdoutRecord["msg"] != record["msg"] || stdoutRecord["password"] != "<redacted>" {
		t.Fatalf("stdout diverged from sink: %#v", stdoutRecord)
	}
}

func TestStartupLogSinkServiceOverrideBeatsSharedEndpoint(t *testing.T) {
	shared := listenLoopbackUDP(t, net.ParseIP("127.0.0.1"))
	service := listenLoopbackUDP(t, net.ParseIP("127.0.0.1"))
	t.Setenv("METIN2_LOG_SINK", shared.LocalAddr().String())
	t.Setenv("METIN2_GAMED_LOG_SINK", service.LocalAddr().String())
	SetRemoteLogSink(nil)
	t.Cleanup(func() { SetRemoteLogSink(nil) })

	var stdout bytes.Buffer
	logger, err := NewStartupServiceLogger("gamed", &stdout)
	if err != nil {
		t.Fatalf("startup sink: %v", err)
	}
	logger.Info("ops server listening", "addr", "127.0.0.1:6060")

	payload := readLoopbackUDP(t, service)
	var record map[string]any
	if err := json.Unmarshal(payload, &record); err != nil {
		t.Fatalf("decode service sink: %v\n%s", err, payload)
	}
	if record["service"] != "gamed" || record["msg"] != "ops server listening" {
		t.Fatalf("service sink record = %#v", record)
	}
	assertUDPQuiet(t, shared)
}

func TestStartupLogSinkRefusesRemoteSyslogAndSIEM(t *testing.T) {
	SetRemoteLogSink(nil)
	t.Cleanup(func() { SetRemoteLogSink(nil) })
	for _, target := range []string{
		"10.1.2.3:514",
		"syslog.example:514",
		"127.0.0.1:514?token=raw-ticket",
		"udp://127.0.0.1:514",
		"<134>127.0.0.1:514",
		"siem.example:514",
	} {
		t.Run(target, func(t *testing.T) {
			t.Setenv("METIN2_GAMED_LOG_SINK", target)
			var stdout bytes.Buffer
			logger, err := NewStartupServiceLogger("gamed", &stdout)
			if err == nil || logger != nil {
				t.Fatalf("target %q enabled a startup sink", target)
			}
			if err.Error() != "remote log sink refused" {
				t.Fatalf("target %q error = %q", target, err.Error())
			}
			if strings.Contains(err.Error(), strings.TrimSpace(target)) {
				t.Fatalf("error echoed endpoint %q: %s", target, err.Error())
			}
			if RemoteLogConfigured() {
				t.Fatal("refused startup config must leave the sink disabled")
			}
			if stdout.Len() != 0 {
				t.Fatalf("refused config wrote logs: %s", stdout.String())
			}
		})
	}
}

func listenLoopbackUDP(t *testing.T, ip net.IP) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: ip, Port: 0})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func readLoopbackUDP(t *testing.T, conn *net.UDPConn) []byte {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4096)
	n, from, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read sink: %v", err)
	}
	if from == nil || !from.IP.IsLoopback() {
		t.Fatalf("packet from %v, want loopback", from)
	}
	return buf[:n]
}

func assertUDPQuiet(t *testing.T, conn *net.UDPConn) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	buf := make([]byte, 4096)
	_, _, err := conn.ReadFromUDP(buf)
	if err == nil {
		t.Fatal("shared sink received the service-specific copy")
	}
	if ne, ok := err.(net.Error); !ok || !ne.Timeout() {
		t.Fatalf("read shared sink: %v", err)
	}
}
