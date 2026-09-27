package observability

import (
	"bytes"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"
)

func TestRemoteLogSinkDisabledByDefaultStaysOnStdout(t *testing.T) {
	var stdout bytes.Buffer
	logger := NewServiceLogger("gamed", &stdout)
	if logger == nil {
		t.Fatal("expected service logger")
	}
	if RemoteLogConfigured() {
		t.Fatal("missing sink config must stay disabled")
	}
	SetRemoteLogSink(&LoopbackLogSink{Endpoint: "127.0.0.1:1"})
	t.Cleanup(func() { SetRemoteLogSink(nil) })
	if !RemoteLogConfigured() {
		t.Fatal("explicit loopback endpoint should be accepted")
	}
	SetRemoteLogSink(nil)
	if RemoteLogConfigured() {
		t.Fatal("clearing the sink must disable it again")
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

func TestRemoteLogSinkCopiesRedactedJSONToLoopbackUDP(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer conn.Close()
	endpoint := conn.LocalAddr().String()

	var stdout bytes.Buffer
	logger, err := NewServiceLoggerWithRemoteSink("authd", &stdout, endpoint)
	if err != nil {
		t.Fatalf("enable sink: %v", err)
	}
	if !RemoteLogConfigured() {
		t.Fatal("loopback sink should be configured")
	}
	t.Cleanup(func() { SetRemoteLogSink(nil) })
	logger.Error("open database failed",
		"dsn", "postgres://operator:s3cret@10.1.2.3:5432/metin2",
		"password", "hunter2",
		"addr", "127.0.0.1:6061",
	)

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4096)
	n, from, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read sink: %v", err)
	}
	if from == nil || !from.IP.IsLoopback() {
		t.Fatalf("packet from %v, want loopback", from)
	}
	payload := buf[:n]
	if bytes.Contains(payload, []byte("s3cret")) || bytes.Contains(payload, []byte("hunter2")) || bytes.Contains(payload, []byte("postgres://")) {
		t.Fatalf("sink leaked secret: %s", payload)
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
	if record["addr"] != "127.0.0.1:6061" {
		t.Fatalf("addr = %v", record["addr"])
	}
	stdoutRecord := decodeLastJSONLog(t, stdout.Bytes())
	if stdoutRecord["msg"] != record["msg"] || stdoutRecord["dsn"] != "<redacted>" {
		t.Fatalf("stdout diverged from sink: %#v", stdoutRecord)
	}
}

func TestRemoteLogSinkCopiesRedactedJSONToIPv6Loopback(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv6loopback, Port: 0})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer conn.Close()
	_, port, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		t.Fatalf("local port: %v", err)
	}
	endpoint := net.JoinHostPort("::1", port)

	var stdout bytes.Buffer
	logger, err := NewServiceLoggerWithRemoteSink("gamed", &stdout, endpoint)
	if err != nil {
		t.Fatalf("enable sink: %v", err)
	}
	if !RemoteLogConfigured() {
		t.Fatal("[::1] sink should be configured")
	}
	t.Cleanup(func() { SetRemoteLogSink(nil) })
	logger.Info("ops server listening", "password", "hunter2", "addr", "[::1]:6060")

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4096)
	n, from, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read [::1] sink: %v", err)
	}
	if from == nil || !from.IP.IsLoopback() || from.IP.To4() != nil {
		t.Fatalf("packet from %v, want IPv6 loopback", from)
	}
	payload := buf[:n]
	if bytes.Contains(payload, []byte("hunter2")) {
		t.Fatalf("sink leaked secret: %s", payload)
	}
	var record map[string]any
	if err := json.Unmarshal(payload, &record); err != nil {
		t.Fatalf("decode sink JSON: %v\n%s", err, payload)
	}
	if record["service"] != "gamed" || record["msg"] != "ops server listening" {
		t.Fatalf("sink record = %#v", record)
	}
	if record["password"] != "<redacted>" || record["addr"] != "[::1]:6060" {
		t.Fatalf("record = %#v", record)
	}
	stdoutRecord := decodeLastJSONLog(t, stdout.Bytes())
	if stdoutRecord["msg"] != record["msg"] || stdoutRecord["password"] != "<redacted>" {
		t.Fatalf("stdout diverged from sink: %#v", stdoutRecord)
	}
}

func TestRemoteLogSinkRejectsMissingAndRemoteTargets(t *testing.T) {
	SetRemoteLogSink(nil)
	t.Cleanup(func() { SetRemoteLogSink(nil) })
	var stdout bytes.Buffer
	for _, target := range []string{
		"",
		"   ",
		"10.1.2.3:514",
		"syslog.example:514",
		"127.0.0.1:514?token=raw-ticket",
		"127.0.0.1:syslog",
		"127.0.0.1:0",
		"https://127.0.0.1/v1/logs",
	} {
		logger, err := NewServiceLoggerWithRemoteSink("gamed", &stdout, target)
		if err == nil || logger != nil {
			t.Fatalf("target %q enabled a sink", target)
		}
		if err.Error() != "remote log sink refused" {
			t.Fatalf("target %q error = %q", target, err.Error())
		}
		trimmed := strings.TrimSpace(target)
		if trimmed != "" && strings.Contains(err.Error(), trimmed) {
			t.Fatalf("error echoed endpoint %q: %s", target, err.Error())
		}
	}
	if RemoteLogConfigured() {
		t.Fatal("rejected targets must leave the sink disabled")
	}
	if stdout.Len() != 0 {
		t.Fatalf("rejected config wrote logs: %s", stdout.String())
	}
}

func TestRemoteLogSinkDropsOversizedCopyButKeepsStdout(t *testing.T) {
	SetRemoteLogSink(nil)
	t.Cleanup(func() { SetRemoteLogSink(nil) })

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer conn.Close()

	var stdout bytes.Buffer
	logger, err := NewServiceLoggerWithRemoteSink("gamed", &stdout, conn.LocalAddr().String())
	if err != nil {
		t.Fatalf("enable sink: %v", err)
	}
	logger.Info("ops server listening", "note", strings.Repeat("n", remoteLogMaxDatagram))

	record := decodeLastJSONLog(t, stdout.Bytes())
	if record["msg"] != "ops server listening" || record["service"] != "gamed" {
		t.Fatalf("stdout = %#v", record)
	}
	if strings.Contains(stdout.String(), conn.LocalAddr().String()) {
		t.Fatalf("stdout named the sink endpoint: %s", stdout.String())
	}

	_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	buf := make([]byte, 8192)
	_, _, err = conn.ReadFromUDP(buf)
	if err == nil {
		t.Fatal("oversized line was copied to the sink")
	}
	if ne, ok := err.(net.Error); !ok || !ne.Timeout() {
		t.Fatalf("read sink: %v", err)
	}
}
