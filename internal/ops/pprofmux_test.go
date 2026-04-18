package ops

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthzEndpointIncludesServiceName(t *testing.T) {
	mux := NewPprofMux("gamed")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "gamed ok") {
		t.Fatalf("unexpected health body %q", rec.Body.String())
	}
}

func TestPprofIndexIsReachable(t *testing.T) {
	mux := NewPprofMux("gamed")

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "profile") {
		t.Fatalf("expected pprof index page, got %q", rec.Body.String())
	}
}

func TestLocalNoticeEndpointQueuesBroadcastForLoopbackPost(t *testing.T) {
	broadcaster := &stubNoticeBroadcaster{delivered: 2}
	mux := NewPprofMuxWithLocalNotice("gamed", broadcaster.BroadcastNotice)

	req := httptest.NewRequest(http.MethodPost, "/local/notice", strings.NewReader("server maintenance"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if broadcaster.calls != 1 || broadcaster.lastMessage != "server maintenance" {
		t.Fatalf("unexpected broadcaster call state: %+v", broadcaster)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if string(body) != "queued 2\n" {
		t.Fatalf("unexpected response body %q", string(body))
	}
}

func TestLocalNoticeEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	broadcaster := &stubNoticeBroadcaster{delivered: 2}
	mux := NewPprofMuxWithLocalNotice("gamed", broadcaster.BroadcastNotice)

	req := httptest.NewRequest(http.MethodPost, "/local/notice", strings.NewReader("server maintenance"))
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if broadcaster.calls != 0 {
		t.Fatalf("expected broadcaster not to be called, got %d calls", broadcaster.calls)
	}
}

func TestLocalNoticeEndpointRejectsEmptyMessage(t *testing.T) {
	broadcaster := &stubNoticeBroadcaster{delivered: 2}
	mux := NewPprofMuxWithLocalNotice("gamed", broadcaster.BroadcastNotice)

	req := httptest.NewRequest(http.MethodPost, "/local/notice", strings.NewReader("   \n"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if broadcaster.calls != 0 {
		t.Fatalf("expected broadcaster not to be called, got %d calls", broadcaster.calls)
	}
}

type stubNoticeBroadcaster struct {
	delivered   int
	calls       int
	lastMessage string
}

func (b *stubNoticeBroadcaster) BroadcastNotice(message string) int {
	b.calls++
	b.lastMessage = message
	return b.delivered
}
