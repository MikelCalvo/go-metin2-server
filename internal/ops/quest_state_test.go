package ops

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
)

func TestLocalQuestStateStoreValidateEndpointReturnsSummaryForLoopbackPost(t *testing.T) {
	validator := &stubQuestStateStoreValidator{summary: queststate.SnapshotSummary{
		FlagCount:  1,
		Characters: []string{"QuestHero"},
		QuestRefs:  []string{"quest:first_steps"},
		FlagKeys:   []string{"QuestHero:quest:first_steps:step"},
	}}
	mux := RegisterLocalQuestStateStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/quest-state/validate", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if validator.calls != 1 {
		t.Fatalf("expected validator to be called once, got %d", validator.calls)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"flag_count":1`) || !strings.Contains(body, `"QuestHero"`) || !strings.Contains(body, `"quest:first_steps"`) {
		t.Fatalf("unexpected quest state validation response body %q", body)
	}
}

func TestLocalQuestStateStoreValidateEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	validator := &stubQuestStateStoreValidator{}
	mux := RegisterLocalQuestStateStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/quest-state/validate", nil)
	req.RemoteAddr = "192.0.2.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected non-loopback request not to call validator, got %d", validator.calls)
	}
}

func TestLocalQuestStateStoreValidateEndpointRejectsNonEmptyBodyBeforeCallback(t *testing.T) {
	validator := &stubQuestStateStoreValidator{}
	mux := RegisterLocalQuestStateStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/quest-state/validate", strings.NewReader("not empty"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected non-empty body not to call validator, got %d", validator.calls)
	}
}

func TestLocalQuestStateStoreValidateEndpointReturnsConflictOnValidationError(t *testing.T) {
	validator := &stubQuestStateStoreValidator{err: errors.New("invalid quest state")}
	mux := RegisterLocalQuestStateStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/quest-state/validate", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if validator.calls != 1 {
		t.Fatalf("expected validator to be called once, got %d", validator.calls)
	}
}

func TestLocalQuestStateStoreCrashTempCleanupEndpointReturnsSummaryForLoopbackPost(t *testing.T) {
	cleaner := &stubQuestStateStoreValidator{summary: queststate.SnapshotSummary{Characters: []string{}, QuestRefs: []string{}, FlagKeys: []string{}}}
	mux := RegisterLocalQuestStateStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/quest-state/crash-temps/cleanup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if cleaner.calls != 1 {
		t.Fatalf("expected cleaner to be called once, got %d", cleaner.calls)
	}
	if !strings.Contains(rec.Body.String(), `"flag_count":0`) {
		t.Fatalf("unexpected quest state cleanup response body %q", rec.Body.String())
	}
}

func TestLocalQuestStateStoreCrashTempCleanupEndpointRejectsWrongMethod(t *testing.T) {
	cleaner := &stubQuestStateStoreValidator{}
	mux := RegisterLocalQuestStateStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Validate)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state/crash-temps/cleanup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected wrong method not to call cleaner, got %d", cleaner.calls)
	}
}

type stubQuestStateStoreValidator struct {
	calls   int
	summary queststate.SnapshotSummary
	err     error
}

func (s *stubQuestStateStoreValidator) Validate() (any, error) {
	s.calls++
	return s.summary, s.err
}
