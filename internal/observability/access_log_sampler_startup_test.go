package observability

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestStartupAccessLogSamplerMissingOrBlankKeepsEveryRequest(t *testing.T) {
	t.Setenv("METIN2_ACCESS_LOG_SAMPLE_INTERVAL", "1h")
	t.Setenv("METIN2_AUTHD_ACCESS_LOG_SAMPLE_INTERVAL", "")
	t.Setenv("METIN2_GAMED_ACCESS_LOG_SAMPLE_INTERVAL", "")
	SetAccessLogSampler(nil)
	t.Cleanup(func() { SetAccessLogSampler(nil) })

	// A blank service-specific override disables, even if the shared setting is present.
	for _, service := range []string{"authd", "gamed"} {
		if err := ConfigureStartupAccessLogSampler(service); err != nil {
			t.Fatalf("%s startup: %v", service, err)
		}
		if AccessLogSamplerConfigured() {
			t.Fatalf("%s blank override enabled the sampler", service)
		}
		var stdout bytes.Buffer
		calls := 0
		handler := WrapOpsAccessLog(NewServiceLogger(service, &stdout), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls++
			w.WriteHeader(http.StatusNoContent)
		}))
		for range 3 {
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/local/build-info", nil))
		}
		if calls != 3 || len(accessLogLines(t, stdout.Bytes())) != 3 {
			t.Fatalf("%s: calls=%d lines=%d, want 3 each", service, calls, len(accessLogLines(t, stdout.Bytes())))
		}
	}
	t.Setenv("METIN2_ACCESS_LOG_SAMPLE_INTERVAL", "")
	// Unset config is also disabled; a blank service override takes precedence.
	if err := os.Unsetenv("METIN2_AUTHD_ACCESS_LOG_SAMPLE_INTERVAL"); err != nil {
		t.Fatal(err)
	}
	if err := ConfigureStartupAccessLogSampler("authd"); err != nil || AccessLogSamplerConfigured() {
		t.Fatalf("unset config: configured=%t err=%v", AccessLogSamplerConfigured(), err)
	}
}

func TestStartupAccessLogSamplerSharedIntervalAndServiceOverride(t *testing.T) {
	t.Setenv("METIN2_ACCESS_LOG_SAMPLE_INTERVAL", "1h")
	t.Setenv("METIN2_GAMED_ACCESS_LOG_SAMPLE_INTERVAL", "250ms")
	t.Setenv("METIN2_AUTHD_ACCESS_LOG_SAMPLE_INTERVAL", "1h")
	SetAccessLogSampler(nil)
	t.Cleanup(func() { SetAccessLogSampler(nil) })

	for _, tc := range []struct {
		service string
		gap     time.Duration
	}{
		{"authd", time.Hour},
		{"gamed", 250 * time.Millisecond},
	} {
		if err := ConfigureStartupAccessLogSampler(tc.service); err != nil {
			t.Fatalf("%s startup: %v", tc.service, err)
		}
		if !AccessLogSamplerConfigured() {
			t.Fatalf("%s startup did not set sampler", tc.service)
		}
		var stdout bytes.Buffer
		calls := 0
		handler := WrapOpsAccessLog(NewServiceLogger(tc.service, &stdout), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls++
			w.WriteHeader(http.StatusNoContent)
		}))
		for range 3 {
			req := httptest.NewRequest(http.MethodPost, "/local/notice?token=private", strings.NewReader("private-body"))
			handler.ServeHTTP(httptest.NewRecorder(), req)
		}
		for _, quiet := range []string{"/healthz", "/debug/pprof/heap"} {
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, quiet, nil))
		}
		lines := accessLogLines(t, stdout.Bytes())
		if calls != 5 || len(lines) != 1 || lines[0]["path"] != "/local/notice" {
			t.Fatalf("%s: calls=%d lines=%v", tc.service, calls, lines)
		}
		for _, forbidden := range []string{"private", "token=", "sample", tc.gap.String(), "pprof", "healthz"} {
			if strings.Contains(stdout.String(), forbidden) {
				t.Fatalf("%s access line leaked %q", tc.service, forbidden)
			}
		}
		// Check the actual override window without sleeping or clock-dependent waits.
		SetAccessLogSampler(nil)
		if err := ConfigureStartupAccessLogSampler(tc.service); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		if !accessLogLineAllowed(start) || accessLogLineAllowed(start.Add(tc.gap-time.Nanosecond)) || !accessLogLineAllowed(start.Add(tc.gap)) {
			t.Fatalf("%s did not apply interval %s", tc.service, tc.gap)
		}
	}

	// Unset service override uses the shared interval instead.
	if err := os.Unsetenv("METIN2_GAMED_ACCESS_LOG_SAMPLE_INTERVAL"); err != nil {
		t.Fatal(err)
	}
	if err := ConfigureStartupAccessLogSampler("gamed"); err != nil || !AccessLogSamplerConfigured() {
		t.Fatalf("shared config: configured=%t err=%v", AccessLogSamplerConfigured(), err)
	}
}

func TestStartupAccessLogSamplerRejectsBadIntervalsWithoutLeakingValue(t *testing.T) {
	t.Setenv("METIN2_ACCESS_LOG_SAMPLE_INTERVAL", "1h")
	SetAccessLogSampler(nil)
	t.Cleanup(func() { SetAccessLogSampler(nil) })
	for _, raw := range []string{"0", "-1s", "10", "bogus-private-token", "1s?token=private", "999999999999999999999999h"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("METIN2_AUTHD_ACCESS_LOG_SAMPLE_INTERVAL", raw)
			if err := ConfigureStartupAccessLogSampler("authd"); err == nil || err.Error() != "access log sampler interval refused" || strings.Contains(err.Error(), raw) {
				t.Fatalf("invalid interval was not safely refused: %v", err)
			}
			if AccessLogSamplerConfigured() {
				t.Fatal("invalid interval enabled sampler")
			}
		})
	}
}
