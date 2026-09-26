package observability

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

// LocalTracePath is the loopback-only JSON view of one completed /local/*
// OpenTelemetry-shaped span. It sits beside daemon JSON logs and
// GET /local/metrics.
//
// It is not a Prometheus exporter, not a remote OTLP endpoint, and not a
// remote admin surface. Daemons do not register it until a later slice
// mounts Handler on the ops mux. With no loopback exporter configured,
// FinishSpan stays in memory and Export refuses to ship anywhere.
const LocalTracePath = "/local/trace"

const (
	otelTraceScopeName    = "go-metin2-server/ops"
	otelTraceSpanName     = "ops.local.request"
	maxTraceSpanNameLen   = 64
	maxTracePathAttrLen   = 128
	defaultTraceSpanLimit = 8
)

// LoopbackTraceExporter is the only exporter this slice accepts. A nil
// exporter, any other name, or a non-loopback endpoint is fail-closed:
// spans stay local and are never shipped.
type LoopbackTraceExporter struct {
	Endpoint string
}

// Configured reports whether Endpoint is an explicit loopback HTTP target.
// Empty config and every remote or wildcard target stay false.
func (e LoopbackTraceExporter) Configured() bool {
	return loopbackTraceEndpoint(e.Endpoint)
}

// OpsTraceSpan is one metadata-only span for a completed /local/* request.
// It never carries query strings, bodies, header values, remote addresses,
// or secrets. StatusCode follows the OTel span status convention: 1 is OK
// and 2 is ERROR (HTTP status >= 400).
type OpsTraceSpan struct {
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	Name       string            `json:"name"`
	Kind       string            `json:"kind"`
	StartUnix  int64             `json:"start_unix_nano"`
	EndUnix    int64             `json:"end_unix_nano"`
	Attributes map[string]string `json:"attributes"`
	StatusCode int               `json:"status_code"`
}

// OpsTraceSnapshot is the document returned by GET LocalTracePath.
type OpsTraceSnapshot struct {
	Service   string         `json:"service"`
	Scope     string         `json:"scope"`
	Exporter  string         `json:"exporter"`
	SpanCount int            `json:"span_count"`
	Spans     []OpsTraceSpan `json:"spans"`
}

// OpsTrace records completed /local/* requests as in-memory spans.
// Nil receivers are safe no-ops so callers can wire the companion beside
// an optional process logger and OpsMetrics.
type OpsTrace struct {
	mu       sync.Mutex
	service  string
	exporter LoopbackTraceExporter
	spans    []OpsTraceSpan
	limit    int
	seq      uint64
	now      func() time.Time
}

// NewOpsTrace returns an empty in-memory tracer. Export stays fail-closed
// until SetExporter receives a loopback endpoint. Service stays blank until
// SetService names the daemon ("authd" or "gamed").
func NewOpsTrace() *OpsTrace {
	return &OpsTrace{limit: defaultTraceSpanLimit, now: time.Now}
}

// SetService records the daemon name copied onto each span as service.name.
// Empty and whitespace-only names are ignored.
func (t *OpsTrace) SetService(service string) {
	if t == nil {
		return
	}
	service = strings.TrimSpace(service)
	if service == "" || len(service) > 64 {
		return
	}
	t.mu.Lock()
	t.service = service
	t.mu.Unlock()
}

// SetExporter accepts only a configured loopback exporter. A nil argument
// or any other endpoint clears the exporter and leaves later FinishSpan
// calls local-only.
func (t *OpsTrace) SetExporter(exporter *LoopbackTraceExporter) {
	if t == nil {
		return
	}
	var next LoopbackTraceExporter
	if exporter != nil && exporter.Configured() {
		next = *exporter
	}
	t.mu.Lock()
	t.exporter = next
	t.mu.Unlock()
}

// StartSpan begins one span. Returned context carries the span so FinishSpan
// can close the same one. A nil receiver returns ctx unchanged and a nil span.
//
// The span name and attribute keys are metadata only. Query strings are
// stripped before an http.target attribute is stored, and sensitive keys are
// dropped rather than copied.
func (t *OpsTrace) StartSpan(ctx context.Context, name string, attrs map[string]string) (context.Context, *OpsTraceSpan) {
	if t == nil {
		return ctx, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	name = cleanTraceSpanName(name)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.seq++
	span := &OpsTraceSpan{
		TraceID:    formatTraceID(t.seq),
		SpanID:     formatSpanID(t.seq),
		Name:       name,
		Kind:       "SPAN_KIND_SERVER",
		StartUnix:  t.nowUnixNano(),
		Attributes: cleanTraceAttrs(attrs),
		StatusCode: 1,
	}
	if t.service != "" {
		span.Attributes["service.name"] = t.service
	}
	return context.WithValue(ctx, opsTraceSpanKey{}, span), span
}

// FinishSpan closes the span stored in ctx. HTTP status >= 400 marks the
// span ERROR. Closed spans are appended to the local ring. Configuring a
// loopback exporter only unlocks Export; spans are never written to the
// network from this process.
func (t *OpsTrace) FinishSpan(ctx context.Context, httpStatus int) *OpsTraceSpan {
	if t == nil || ctx == nil {
		return nil
	}
	span, _ := ctx.Value(opsTraceSpanKey{}).(*OpsTraceSpan)
	if span == nil || span.EndUnix != 0 {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if span.EndUnix != 0 {
		return nil
	}
	span.EndUnix = t.nowUnixNano()
	if span.EndUnix < span.StartUnix {
		span.EndUnix = span.StartUnix
	}
	if httpStatus >= 400 {
		span.StatusCode = 2
	} else {
		span.StatusCode = 1
	}
	finished := *span
	finished.Attributes = copyTraceAttrs(span.Attributes)
	t.spans = append(t.spans, finished)
	if len(t.spans) > t.spanLimit() {
		t.spans = t.spans[len(t.spans)-t.spanLimit():]
	}
	return &finished
}

// Snapshot copies the spans still held in memory. The newest span is last.
func (t *OpsTrace) Snapshot() OpsTraceSnapshot {
	if t == nil {
		return OpsTraceSnapshot{Scope: otelTraceScopeName, Exporter: "fail-closed", Spans: []OpsTraceSpan{}}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	spans := make([]OpsTraceSpan, len(t.spans))
	for i := range t.spans {
		spans[i] = t.spans[i]
		spans[i].Attributes = copyTraceAttrs(t.spans[i].Attributes)
	}
	exporter := "fail-closed"
	if t.exporter.Configured() {
		exporter = "loopback"
	}
	return OpsTraceSnapshot{
		Service:   t.service,
		Scope:     otelTraceScopeName,
		Exporter:  exporter,
		SpanCount: len(spans),
		Spans:     spans,
	}
}

// Export copies currently held spans only when a loopback exporter is
// configured. Missing or remote exporter config returns ok=false and does
// not reveal the refused endpoint. The copy never leaves this process.
func (t *OpsTrace) Export() (spans []OpsTraceSpan, ok bool) {
	if t == nil {
		return nil, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.exporter.Configured() {
		return nil, false
	}
	spans = make([]OpsTraceSpan, len(t.spans))
	for i := range t.spans {
		spans[i] = t.spans[i]
		spans[i].Attributes = copyTraceAttrs(t.spans[i].Attributes)
	}
	return spans, true
}

// Handler serves GET LocalTracePath to loopback callers only.
//
// Non-GET methods return 405 with an empty body. Non-loopback callers return
// 403 with an empty body. The JSON document is Snapshot and never echoes the
// request URL, query, or body.
func (t *OpsTrace) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r == nil || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !metricsLoopback(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		snap := t.Snapshot()
		if snap.Spans == nil {
			snap.Spans = []OpsTraceSpan{}
		}
		encoded, err := json.Marshal(snap)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(encoded)
	})
}

// Wrap returns a handler that records one ops.local.request span for each
// completed /local/* response and leaves the downstream handler unchanged.
//
// A nil receiver returns next. A nil next stays nil. /healthz, /debug/pprof/*,
// and every other non-/local/ path are not traced. Bodies are never read.
// Missing exporter config still records the local span; it does not enable
// shipping.
func (t *OpsTrace) Wrap(next http.Handler) http.Handler {
	if t == nil || next == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r == nil || r.URL == nil || !strings.HasPrefix(r.URL.Path, "/local/") {
			next.ServeHTTP(w, r)
			return
		}
		path := r.URL.Path
		ctx, _ := t.StartSpan(r.Context(), otelTraceSpanName, map[string]string{
			"http.method": r.Method,
			"http.target": path,
		})
		recorder := &opsAccessResponseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r.WithContext(ctx))
		t.FinishSpan(ctx, recorder.status)
	})
}

type opsTraceSpanKey struct{}

func (t *OpsTrace) nowUnixNano() int64 {
	now := time.Now
	if t.now != nil {
		now = t.now
	}
	return now().UnixNano()
}

func (t *OpsTrace) spanLimit() int {
	if t.limit <= 0 {
		return defaultTraceSpanLimit
	}
	return t.limit
}

func copyTraceAttrs(attrs map[string]string) map[string]string {
	if len(attrs) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(attrs))
	for key, value := range attrs {
		out[key] = value
	}
	return out
}

func cleanTraceSpanName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxTraceSpanNameLen || strings.ContainsAny(name, "?# \t/") {
		return otelTraceSpanName
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-':
		default:
			return otelTraceSpanName
		}
	}
	return name
}

func cleanTraceAttrs(attrs map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range attrs {
		key = strings.TrimSpace(key)
		if key == "" || isSensitiveAttrKey(key) || len(key) > 64 {
			continue
		}
		if key == "http.target" || key == "url.path" {
			cleaned, ok := metricsPathKey(value)
			if !ok {
				continue
			}
			out[key] = cleaned
			continue
		}
		value = strings.TrimSpace(value)
		if strings.ContainsAny(value, "?#") || len(value) > maxTracePathAttrLen {
			continue
		}
		out[key] = value
	}
	return out
}

func formatTraceID(seq uint64) string {
	const hexdigits = "0123456789abcdef"
	var raw [16]byte
	raw[15] = byte(seq)
	raw[14] = byte(seq >> 8)
	raw[13] = byte(seq >> 16)
	raw[12] = byte(seq >> 24)
	raw[11] = byte(seq >> 32)
	raw[10] = byte(seq >> 40)
	raw[9] = byte(seq >> 48)
	raw[8] = byte(seq >> 56)
	raw[0] = 0x4d
	raw[1] = 0x32
	var out [32]byte
	for i, b := range raw {
		out[i*2] = hexdigits[b>>4]
		out[i*2+1] = hexdigits[b&0x0f]
	}
	return string(out[:])
}

func formatSpanID(seq uint64) string {
	const hexdigits = "0123456789abcdef"
	var raw [8]byte
	raw[7] = byte(seq)
	raw[6] = byte(seq >> 8)
	raw[5] = byte(seq >> 16)
	raw[4] = byte(seq >> 24)
	raw[3] = byte(seq >> 32)
	raw[2] = byte(seq >> 40)
	raw[1] = byte(seq >> 48)
	raw[0] = byte(seq >> 56)
	var out [16]byte
	for i, b := range raw {
		out[i*2] = hexdigits[b>>4]
		out[i*2+1] = hexdigits[b&0x0f]
	}
	return string(out[:])
}

func loopbackTraceEndpoint(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, " ") || strings.ContainsAny(raw, "?#") {
		return false
	}
	lower := strings.ToLower(raw)
	if !strings.HasPrefix(lower, "http://") {
		return false
	}
	rest := raw[len("http://"):]
	hostport, path, _ := strings.Cut(rest, "/")
	if hostport == "" || path != "" && path != "v1/traces" {
		return false
	}
	return metricsLoopback(hostport)
}
