package observability

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
)

// remoteLogMaxDatagram is one JSON record. Larger lines stay on local stdout
// and are not split across datagrams.
const remoteLogMaxDatagram = 2048

// LoopbackLogSink is the only remote log target this slice accepts.
//
// A nil sink, an empty endpoint, or any non-loopback UDP host:port stays
// disabled. The daemon JSON line still goes to the local writer. This is not
// a Prometheus exporter, not an OpenTelemetry exporter, and not a SIEM.
type LoopbackLogSink struct {
	Endpoint string
}

// Configured reports whether Endpoint is an explicit loopback UDP host:port.
// Empty config and every remote, wildcard, or query-bearing target stay false.
func (s LoopbackLogSink) Configured() bool {
	_, ok := loopbackLogEndpoint(s.Endpoint)
	return ok
}

var (
	remoteLogMu       sync.Mutex
	remoteLogEndpoint string
)

// SetRemoteLogSink accepts only a configured loopback UDP sink. A nil
// argument or any other endpoint clears the sink. Missing config stays on
// local stdout. The endpoint itself is never written into a log record.
//
// Setting the flag does not attach a writer. Only
// NewServiceLoggerWithRemoteSink copies lines, and only for that logger.
func SetRemoteLogSink(sink *LoopbackLogSink) {
	remoteLogMu.Lock()
	defer remoteLogMu.Unlock()
	remoteLogEndpoint = ""
	if sink == nil {
		return
	}
	endpoint, ok := loopbackLogEndpoint(sink.Endpoint)
	if !ok {
		return
	}
	remoteLogEndpoint = endpoint
}

// RemoteLogConfigured reports whether a loopback UDP sink is set. Missing
// config stays false.
func RemoteLogConfigured() bool {
	remoteLogMu.Lock()
	defer remoteLogMu.Unlock()
	return remoteLogEndpoint != ""
}

// NewServiceLoggerWithRemoteSink returns the shared daemon JSON logger and,
// only when endpoint is a loopback UDP host:port, also copies each redacted
// JSON line to that sink.
//
// Missing or refused endpoints return an error and no logger, and they do
// not enable the package sink. The local writer still receives every line
// when the sink is accepted. The copy never dials until a record is written,
// never includes the endpoint, and never replaces stdout.
func NewServiceLoggerWithRemoteSink(serviceName string, w io.Writer, endpoint string) (*slog.Logger, error) {
	resolved, ok := loopbackLogEndpoint(endpoint)
	if !ok {
		return nil, errRemoteLogSinkRefused
	}
	SetRemoteLogSink(&LoopbackLogSink{Endpoint: resolved})
	logger := NewServiceLogger(serviceName, &teeRemoteLog{primary: w, endpoint: resolved})
	return logger, nil
}

// errRemoteLogSinkRefused is the only error this slice returns for a sink
// that is missing or not loopback UDP. It carries no endpoint text.
var errRemoteLogSinkRefused = errors.New("remote log sink refused")

// teeRemoteLog writes the already-rendered JSON line to the local writer and
// copies that same line to one loopback UDP socket. A failed or oversized
// copy never changes the local write. The socket is opened once.
type teeRemoteLog struct {
	primary  io.Writer
	endpoint string

	mu   sync.Mutex
	conn *net.UDPConn
}

func (t *teeRemoteLog) Write(p []byte) (int, error) {
	n, err := t.primary.Write(p)
	if err != nil {
		return n, err
	}
	line := bytes.TrimSpace(p)
	if len(line) == 0 || len(line) > remoteLogMaxDatagram || !bytes.HasPrefix(line, []byte("{")) {
		return n, nil
	}
	conn := t.socket()
	if conn == nil {
		return n, nil
	}
	_, _ = conn.Write(line)
	return n, nil
}

func (t *teeRemoteLog) socket() *net.UDPConn {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conn != nil {
		return t.conn
	}
	addr, err := net.ResolveUDPAddr("udp", t.endpoint)
	if err != nil || addr == nil || addr.IP == nil || !addr.IP.IsLoopback() || addr.Port == 0 {
		return nil
	}
	// Dial from the same family as the accepted sink. A fixed 127.0.0.1
	// local address cannot reach [::1]:port on this host, so a documented
	// IPv6 loopback sink would stay configured and never deliver.
	local := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	if addr.IP.To4() == nil {
		local = &net.UDPAddr{IP: net.IPv6loopback, Port: 0}
	}
	conn, err := net.DialUDP("udp", local, addr)
	if err != nil {
		return nil
	}
	t.conn = conn
	return conn
}

func loopbackLogEndpoint(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, " \t?#/\\") {
		return "", false
	}
	if strings.Contains(raw, "://") {
		return "", false
	}
	host, port, err := net.SplitHostPort(raw)
	if err != nil || port == "" || host == "" {
		return "", false
	}
	number, err := net.LookupPort("udp", port)
	if err != nil || number <= 0 || number > 65535 {
		return "", false
	}
	if port != strconv.Itoa(number) {
		return "", false
	}
	if strings.EqualFold(host, "localhost") {
		return net.JoinHostPort("127.0.0.1", port), true
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", false
	}
	return net.JoinHostPort(ip.String(), port), true
}
