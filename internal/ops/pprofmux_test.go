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

func TestLocalRelocateEndpointRelocatesConnectedCharacterForLoopbackPost(t *testing.T) {
	relocator := &stubCharacterRelocator{relocated: true}
	mux := NewPprofMuxWithLocalRelocation("gamed", nil, relocator.RelocateCharacter)

	req := httptest.NewRequest(http.MethodPost, "/local/relocate", strings.NewReader(`{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if relocator.calls != 1 || relocator.lastName != "PeerTwo" || relocator.lastMapIndex != 42 || relocator.lastX != 1700 || relocator.lastY != 2800 {
		t.Fatalf("unexpected relocator call state: %+v", relocator)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if string(body) != "relocated 1\n" {
		t.Fatalf("unexpected response body %q", string(body))
	}
}

func TestLocalRelocateEndpointRejectsInvalidBody(t *testing.T) {
	relocator := &stubCharacterRelocator{relocated: true}
	mux := NewPprofMuxWithLocalRelocation("gamed", nil, relocator.RelocateCharacter)

	req := httptest.NewRequest(http.MethodPost, "/local/relocate", strings.NewReader(`{"name":"PeerTwo","map_index":0,"x":1700,"y":2800}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if relocator.calls != 0 {
		t.Fatalf("expected relocator not to be called, got %d calls", relocator.calls)
	}
}

func TestLocalRelocateEndpointRejectsTrailingJSON(t *testing.T) {
	relocator := &stubCharacterRelocator{relocated: true}
	mux := NewPprofMuxWithLocalRelocation("gamed", nil, relocator.RelocateCharacter)

	req := httptest.NewRequest(http.MethodPost, "/local/relocate", strings.NewReader(`{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}{"extra":1}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if relocator.calls != 0 {
		t.Fatalf("expected relocator not to be called, got %d calls", relocator.calls)
	}
}

func TestLocalRelocateEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	relocator := &stubCharacterRelocator{relocated: true}
	mux := NewPprofMuxWithLocalRelocation("gamed", nil, relocator.RelocateCharacter)

	req := httptest.NewRequest(http.MethodPost, "/local/relocate", strings.NewReader(`{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}`))
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if relocator.calls != 0 {
		t.Fatalf("expected relocator not to be called, got %d calls", relocator.calls)
	}
}

func TestLocalRelocateEndpointReturnsNotFoundForUnknownTarget(t *testing.T) {
	relocator := &stubCharacterRelocator{relocated: false}
	mux := NewPprofMuxWithLocalRelocation("gamed", nil, relocator.RelocateCharacter)

	req := httptest.NewRequest(http.MethodPost, "/local/relocate", strings.NewReader(`{"name":"MissingPeer","map_index":42,"x":1700,"y":2800}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if relocator.calls != 1 {
		t.Fatalf("expected relocator to be called once, got %d calls", relocator.calls)
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

type stubCharacterRelocator struct {
	relocated    bool
	calls        int
	lastName     string
	lastMapIndex uint32
	lastX        int32
	lastY        int32
}

func (r *stubCharacterRelocator) RelocateCharacter(name string, mapIndex uint32, x int32, y int32) bool {
	r.calls++
	r.lastName = name
	r.lastMapIndex = mapIndex
	r.lastX = x
	r.lastY = y
	return r.relocated
}
