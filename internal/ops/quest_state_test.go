package ops

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
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

func TestLocalQuestStateTransitionEndpointReturnsResultForLoopbackPost(t *testing.T) {
	applier := &stubQuestStateTransitionApplier{result: queststate.TransitionApplyResult{
		Transition: queststate.Transition{Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "step", From: 0, To: 1},
		Result:     queststate.TransitionResult{Applied: true, CurrentValue: 0},
		Summary:    queststate.SnapshotSummary{FlagCount: 1, Characters: []string{"QuestHero"}, QuestRefs: []string{"quest:first_steps"}, FlagKeys: []string{"QuestHero:quest:first_steps:step"}},
	}}
	mux := RegisterLocalQuestStateTransitionEndpoint(NewPprofMux("gamed"), applier.Apply)

	req := httptest.NewRequest(http.MethodPost, "/local/quest-state/transition", strings.NewReader(`{"character":"QuestHero","quest_ref":"quest:first_steps","flag":"step","from":0,"to":1}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if applier.calls != 1 {
		t.Fatalf("expected applier to be called once, got %d", applier.calls)
	}
	wantTransition := queststate.Transition{Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "step", From: 0, To: 1}
	if applier.lastTransition != wantTransition {
		t.Fatalf("unexpected transition passed to applier:\n got: %#v\nwant: %#v", applier.lastTransition, wantTransition)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"applied":true`) || !strings.Contains(body, `"flag_count":1`) || !strings.Contains(body, `"QuestHero"`) {
		t.Fatalf("unexpected quest transition response body %q", body)
	}
}

func TestLocalQuestStateTransitionEndpointReturnsCompareAndSetFailureAsOK(t *testing.T) {
	applier := &stubQuestStateTransitionApplier{result: queststate.TransitionApplyResult{
		Transition: queststate.Transition{Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "step", From: 0, To: 2},
		Result:     queststate.TransitionResult{Reason: queststate.TransitionReasonCurrentValueMismatch, CurrentValue: 1},
		Summary:    queststate.SnapshotSummary{FlagCount: 1, Characters: []string{"QuestHero"}, QuestRefs: []string{"quest:first_steps"}, FlagKeys: []string{"QuestHero:quest:first_steps:step"}},
	}}
	mux := RegisterLocalQuestStateTransitionEndpoint(NewPprofMux("gamed"), applier.Apply)

	req := httptest.NewRequest(http.MethodPost, "/local/quest-state/transition", strings.NewReader(`{"character":"QuestHero","quest_ref":"quest:first_steps","flag":"step","from":0,"to":2}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d for compare-and-set failure result, got %d", http.StatusOK, rec.Code)
	}
	if applier.calls != 1 {
		t.Fatalf("expected applier to be called once, got %d", applier.calls)
	}
	if !strings.Contains(rec.Body.String(), `"reason":"current_value_mismatch"`) || !strings.Contains(rec.Body.String(), `"current_value":1`) {
		t.Fatalf("unexpected quest transition mismatch response body %q", rec.Body.String())
	}
}

func TestLocalQuestStateTransitionPreviewEndpointReturnsDryRunResultForLoopbackPost(t *testing.T) {
	previewer := &stubQuestStateTransitionApplier{result: queststate.TransitionApplyResult{
		Transition: queststate.Transition{Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "step", From: 0, To: 1},
		Result:     queststate.TransitionResult{Applied: true, CurrentValue: 0},
		Summary:    queststate.SnapshotSummary{FlagCount: 1, Characters: []string{"QuestHero"}, QuestRefs: []string{"quest:first_steps"}, FlagKeys: []string{"QuestHero:quest:first_steps:step"}},
	}}
	mux := RegisterLocalQuestStateTransitionPreviewEndpoint(NewPprofMux("gamed"), previewer.Apply)

	req := httptest.NewRequest(http.MethodPost, "/local/quest-state/transition-preview", strings.NewReader(`{"character":"QuestHero","quest_ref":"quest:first_steps","flag":"step","from":0,"to":1}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected previewer to be called once, got %d", previewer.calls)
	}
	wantTransition := queststate.Transition{Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "step", From: 0, To: 1}
	if previewer.lastTransition != wantTransition {
		t.Fatalf("unexpected transition passed to previewer:\n got: %#v\nwant: %#v", previewer.lastTransition, wantTransition)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"applied":true`) || !strings.Contains(body, `"flag_count":1`) || !strings.Contains(body, `"QuestHero"`) {
		t.Fatalf("unexpected quest transition preview response body %q", body)
	}
}

func TestLocalQuestStateTransitionPreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubQuestStateTransitionApplier{}
	mux := RegisterLocalQuestStateTransitionPreviewEndpoint(NewPprofMux("gamed"), previewer.Apply)

	req := httptest.NewRequest(http.MethodPost, "/local/quest-state/transition-preview", strings.NewReader(`{"character":"QuestHero","quest_ref":"quest:first_steps","flag":"step","from":0,"to":1}`))
	req.RemoteAddr = "192.0.2.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected non-loopback request not to call previewer, got %d", previewer.calls)
	}
}

func TestLocalQuestStateTransitionPreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubQuestStateTransitionApplier{}
	mux := RegisterLocalQuestStateTransitionPreviewEndpoint(NewPprofMux("gamed"), previewer.Apply)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state/transition-preview", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected wrong method not to call previewer, got %d", previewer.calls)
	}
}

func TestLocalQuestStateTransitionEndpointRejectsInvalidBodyBeforeCallback(t *testing.T) {
	applier := &stubQuestStateTransitionApplier{}
	mux := RegisterLocalQuestStateTransitionEndpoint(NewPprofMux("gamed"), applier.Apply)

	req := httptest.NewRequest(http.MethodPost, "/local/quest-state/transition", strings.NewReader(`{"character":"QuestHero","quest_ref":"quest:first_steps","flag":"step","from":0,"to":1} {}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if applier.calls != 0 {
		t.Fatalf("expected invalid body not to call applier, got %d", applier.calls)
	}
}

func TestLocalQuestStateTransitionEndpointRejectsOversizedBodyBeforeCallback(t *testing.T) {
	applier := &stubQuestStateTransitionApplier{}
	mux := RegisterLocalQuestStateTransitionEndpoint(NewPprofMux("gamed"), applier.Apply)

	body := `{"character":"QuestHero","quest_ref":"quest:first_steps","flag":"step","from":0,"to":1}` + strings.Repeat(" ", 4096)
	req := httptest.NewRequest(http.MethodPost, "/local/quest-state/transition", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if applier.calls != 0 {
		t.Fatalf("expected oversized body not to call applier, got %d", applier.calls)
	}
}

func TestLocalQuestStateTransitionEndpointReturnsConflictOnApplyError(t *testing.T) {
	applier := &stubQuestStateTransitionApplier{err: errors.New("quest state unavailable")}
	mux := RegisterLocalQuestStateTransitionEndpoint(NewPprofMux("gamed"), applier.Apply)

	req := httptest.NewRequest(http.MethodPost, "/local/quest-state/transition", strings.NewReader(`{"character":"QuestHero","quest_ref":"quest:first_steps","flag":"step","from":0,"to":1}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if applier.calls != 1 {
		t.Fatalf("expected applier to be called once, got %d", applier.calls)
	}
}

func TestLocalQuestStateTransitionEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	applier := &stubQuestStateTransitionApplier{}
	mux := RegisterLocalQuestStateTransitionEndpoint(NewPprofMux("gamed"), applier.Apply)

	req := httptest.NewRequest(http.MethodPost, "/local/quest-state/transition", strings.NewReader(`{"character":"QuestHero","quest_ref":"quest:first_steps","flag":"step","from":0,"to":1}`))
	req.RemoteAddr = "192.0.2.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if applier.calls != 0 {
		t.Fatalf("expected non-loopback request not to call applier, got %d", applier.calls)
	}
}

func TestLocalQuestStateTransitionEndpointRejectsWrongMethod(t *testing.T) {
	applier := &stubQuestStateTransitionApplier{}
	mux := RegisterLocalQuestStateTransitionEndpoint(NewPprofMux("gamed"), applier.Apply)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state/transition", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if applier.calls != 0 {
		t.Fatalf("expected wrong method not to call applier, got %d", applier.calls)
	}
}

func TestLocalQuestStateOverviewEndpointReturnsOverviewForLoopbackGet(t *testing.T) {
	reader := &stubQuestStateOverviewReader{overview: queststate.Overview{
		FlagCount:      2,
		CharacterCount: 2,
		QuestCount:     1,
		QuestRefs:      []string{"quest:first_steps"},
		Characters: []queststate.CharacterSnapshot{
			{Character: "AnotherHero", Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "met_guard", Value: 1}}},
			{Character: "QuestHero", Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "step", Value: 2}}},
		},
	}}
	mux := RegisterLocalQuestStateOverviewEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if reader.calls != 1 {
		t.Fatalf("expected overview reader to be called once, got %d", reader.calls)
	}
	var got queststate.Overview
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode quest-state overview response body: %v", err)
	}
	if !reflect.DeepEqual(got, reader.overview) {
		t.Fatalf("unexpected quest-state overview response:\n got: %#v\nwant: %#v", got, reader.overview)
	}
}

func TestLocalQuestStateOverviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	reader := &stubQuestStateOverviewReader{}
	mux := RegisterLocalQuestStateOverviewEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state", nil)
	req.RemoteAddr = "192.0.2.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if reader.calls != 0 {
		t.Fatalf("expected non-loopback request not to call overview reader, got %d", reader.calls)
	}
}

func TestLocalQuestStateOverviewEndpointRejectsWrongMethod(t *testing.T) {
	reader := &stubQuestStateOverviewReader{}
	mux := RegisterLocalQuestStateOverviewEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodPost, "/local/quest-state", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if reader.calls != 0 {
		t.Fatalf("expected wrong method not to call overview reader, got %d", reader.calls)
	}
}

func TestLocalQuestStateOverviewEndpointReturnsConflictOnReadError(t *testing.T) {
	reader := &stubQuestStateOverviewReader{err: errors.New("invalid quest state")}
	mux := RegisterLocalQuestStateOverviewEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if reader.calls != 1 {
		t.Fatalf("expected overview reader to be called once, got %d", reader.calls)
	}
}

func TestLocalQuestStateCharacterEndpointReturnsSnapshotForLoopbackGet(t *testing.T) {
	reader := &stubQuestStateCharacterReader{snapshot: queststate.CharacterSnapshot{
		Character: "QuestHero",
		Flags: []queststate.FlagSnapshot{
			{QuestRef: "quest:first_steps", Name: "step", Value: 2},
		},
	}, ok: true}
	mux := RegisterLocalQuestStateCharacterEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state/characters/QuestHero", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if reader.calls != 1 || reader.lastCharacter != "QuestHero" {
		t.Fatalf("unexpected reader call state: calls=%d last_character=%q", reader.calls, reader.lastCharacter)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"character":"QuestHero"`) || !strings.Contains(body, `"quest_ref":"quest:first_steps"`) || !strings.Contains(body, `"name":"step"`) || !strings.Contains(body, `"value":2`) {
		t.Fatalf("unexpected quest-state character response body %q", body)
	}
}

func TestLocalQuestStateCharacterEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	reader := &stubQuestStateCharacterReader{ok: true}
	mux := RegisterLocalQuestStateCharacterEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state/characters/QuestHero", nil)
	req.RemoteAddr = "192.0.2.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if reader.calls != 0 {
		t.Fatalf("expected non-loopback request not to call reader, got %d", reader.calls)
	}
}

func TestLocalQuestStateCharacterEndpointRejectsAmbiguousCharacterName(t *testing.T) {
	reader := &stubQuestStateCharacterReader{ok: true}
	mux := RegisterLocalQuestStateCharacterEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state/characters/Bad%2FName", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if reader.calls != 0 {
		t.Fatalf("expected ambiguous character name not to call reader, got %d", reader.calls)
	}
}

func TestLocalQuestStateCharacterEndpointReturnsNotFoundForMissingSnapshot(t *testing.T) {
	reader := &stubQuestStateCharacterReader{}
	mux := RegisterLocalQuestStateCharacterEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state/characters/MissingHero", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if reader.calls != 1 || reader.lastCharacter != "MissingHero" {
		t.Fatalf("unexpected reader call state for missing snapshot: calls=%d last_character=%q", reader.calls, reader.lastCharacter)
	}
}

func TestLocalQuestStateCharacterEndpointReturnsConflictOnReadError(t *testing.T) {
	reader := &stubQuestStateCharacterReader{err: errors.New("invalid quest state")}
	mux := RegisterLocalQuestStateCharacterEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state/characters/QuestHero", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if reader.calls != 1 || reader.lastCharacter != "QuestHero" {
		t.Fatalf("unexpected reader call state for read error: calls=%d last_character=%q", reader.calls, reader.lastCharacter)
	}
}

func TestLocalQuestStateCharacterEndpointRejectsWrongMethod(t *testing.T) {
	reader := &stubQuestStateCharacterReader{ok: true}
	mux := RegisterLocalQuestStateCharacterEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodPost, "/local/quest-state/characters/QuestHero", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if reader.calls != 0 {
		t.Fatalf("expected wrong method not to call reader, got %d", reader.calls)
	}
}

func TestLocalQuestStateQuestEndpointReturnsSnapshotForLoopbackGet(t *testing.T) {
	reader := &stubQuestStateQuestReader{snapshot: queststate.QuestSnapshot{
		QuestRef:  "quest:first_steps",
		FlagCount: 2,
		Characters: []queststate.CharacterSnapshot{
			{Character: "AnotherHero", Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "met_guard", Value: 1}}},
			{Character: "QuestHero", Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "step", Value: 2}}},
		},
	}, ok: true}
	mux := RegisterLocalQuestStateQuestEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state/quests/quest:first_steps", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if reader.calls != 1 || reader.lastQuestRef != "quest:first_steps" {
		t.Fatalf("unexpected quest reader call state: calls=%d last_quest_ref=%q", reader.calls, reader.lastQuestRef)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"quest_ref":"quest:first_steps"`) || !strings.Contains(body, `"flag_count":2`) || !strings.Contains(body, `"AnotherHero"`) || !strings.Contains(body, `"QuestHero"`) {
		t.Fatalf("unexpected quest-state quest response body %q", body)
	}
}

func TestLocalQuestStateQuestEndpointRejectsInvalidQuestRef(t *testing.T) {
	reader := &stubQuestStateQuestReader{ok: true}
	mux := RegisterLocalQuestStateQuestEndpoint(NewPprofMux("gamed"), reader.Read)

	for _, path := range []string{
		"/local/quest-state/quests/",
		"/local/quest-state/quests/first_steps",
		"/local/quest-state/quests/quest%2Ffirst_steps",
		"/local/quest-state/quests/quest:first_steps/extra",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for invalid quest ref path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if reader.calls != 0 {
		t.Fatalf("expected invalid quest refs not to call reader, got %d", reader.calls)
	}
}

func TestLocalQuestStateQuestEndpointReturnsNotFoundForMissingQuest(t *testing.T) {
	reader := &stubQuestStateQuestReader{}
	mux := RegisterLocalQuestStateQuestEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state/quests/quest:missing_steps", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if reader.calls != 1 || reader.lastQuestRef != "quest:missing_steps" {
		t.Fatalf("unexpected quest reader call state for missing quest: calls=%d last_quest_ref=%q", reader.calls, reader.lastQuestRef)
	}
}

func TestLocalQuestStateQuestEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	reader := &stubQuestStateQuestReader{ok: true}
	mux := RegisterLocalQuestStateQuestEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state/quests/quest:first_steps", nil)
	req.RemoteAddr = "192.0.2.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if reader.calls != 0 {
		t.Fatalf("expected non-loopback request not to call quest reader, got %d", reader.calls)
	}
}

func TestLocalQuestStateQuestEndpointRejectsWrongMethod(t *testing.T) {
	reader := &stubQuestStateQuestReader{ok: true}
	mux := RegisterLocalQuestStateQuestEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodPost, "/local/quest-state/quests/quest:first_steps", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if reader.calls != 0 {
		t.Fatalf("expected wrong method not to call quest reader, got %d", reader.calls)
	}
}

func TestLocalQuestStateQuestEndpointReturnsConflictOnReadError(t *testing.T) {
	reader := &stubQuestStateQuestReader{err: errors.New("invalid quest state")}
	mux := RegisterLocalQuestStateQuestEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state/quests/quest:first_steps", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if reader.calls != 1 || reader.lastQuestRef != "quest:first_steps" {
		t.Fatalf("unexpected quest reader call state for read error: calls=%d last_quest_ref=%q", reader.calls, reader.lastQuestRef)
	}
}

func TestLocalQuestStateFlagEndpointReturnsExactFlagForLoopbackGet(t *testing.T) {
	reader := &stubQuestStateFlagReader{flag: queststate.Flag{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 2}, ok: true}
	mux := RegisterLocalQuestStateFlagEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state/flags/QuestHero/quest:first_steps/step", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if reader.calls != 1 || reader.lastCharacter != "QuestHero" || reader.lastQuestRef != "quest:first_steps" || reader.lastFlag != "step" {
		t.Fatalf("unexpected flag reader call state: calls=%d character=%q quest_ref=%q flag=%q", reader.calls, reader.lastCharacter, reader.lastQuestRef, reader.lastFlag)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"character":"QuestHero"`) || !strings.Contains(body, `"quest_ref":"quest:first_steps"`) || !strings.Contains(body, `"name":"step"`) || !strings.Contains(body, `"value":2`) {
		t.Fatalf("unexpected quest-state flag response body %q", body)
	}
}

func TestLocalQuestStateFlagEndpointRejectsInvalidIdentity(t *testing.T) {
	reader := &stubQuestStateFlagReader{ok: true}
	mux := RegisterLocalQuestStateFlagEndpoint(NewPprofMux("gamed"), reader.Read)

	for _, path := range []string{
		"/local/quest-state/flags/QuestHero/quest:first_steps",
		"/local/quest-state/flags/Bad%2FName/quest:first_steps/step",
		"/local/quest-state/flags/QuestHero/first_steps/step",
		"/local/quest-state/flags/QuestHero/quest%2Ffirst_steps/step",
		"/local/quest-state/flags/QuestHero/quest:first_steps/BadFlag",
		"/local/quest-state/flags/QuestHero/quest:first_steps/step/extra",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for invalid flag path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if reader.calls != 0 {
		t.Fatalf("expected invalid flag identities not to call reader, got %d", reader.calls)
	}
}

func TestLocalQuestStateFlagEndpointReturnsNotFoundForMissingFlag(t *testing.T) {
	reader := &stubQuestStateFlagReader{}
	mux := RegisterLocalQuestStateFlagEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state/flags/QuestHero/quest:first_steps/missing_flag", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if reader.calls != 1 || reader.lastCharacter != "QuestHero" || reader.lastQuestRef != "quest:first_steps" || reader.lastFlag != "missing_flag" {
		t.Fatalf("unexpected flag reader call state for missing flag: calls=%d character=%q quest_ref=%q flag=%q", reader.calls, reader.lastCharacter, reader.lastQuestRef, reader.lastFlag)
	}
}

func TestLocalQuestStateFlagEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	reader := &stubQuestStateFlagReader{ok: true}
	mux := RegisterLocalQuestStateFlagEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state/flags/QuestHero/quest:first_steps/step", nil)
	req.RemoteAddr = "192.0.2.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if reader.calls != 0 {
		t.Fatalf("expected non-loopback request not to call flag reader, got %d", reader.calls)
	}
}

func TestLocalQuestStateFlagEndpointRejectsWrongMethod(t *testing.T) {
	reader := &stubQuestStateFlagReader{ok: true}
	mux := RegisterLocalQuestStateFlagEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodPost, "/local/quest-state/flags/QuestHero/quest:first_steps/step", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if reader.calls != 0 {
		t.Fatalf("expected wrong method not to call flag reader, got %d", reader.calls)
	}
}

func TestLocalQuestStateFlagEndpointReturnsConflictOnReadError(t *testing.T) {
	reader := &stubQuestStateFlagReader{err: errors.New("invalid quest state")}
	mux := RegisterLocalQuestStateFlagEndpoint(NewPprofMux("gamed"), reader.Read)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state/flags/QuestHero/quest:first_steps/step", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if reader.calls != 1 || reader.lastCharacter != "QuestHero" || reader.lastQuestRef != "quest:first_steps" || reader.lastFlag != "step" {
		t.Fatalf("unexpected flag reader call state for read error: calls=%d character=%q quest_ref=%q flag=%q", reader.calls, reader.lastCharacter, reader.lastQuestRef, reader.lastFlag)
	}
}

func TestLocalCharacterQuestStateExportEndpointReturnsLoopbackJSON(t *testing.T) {
	exporter := &stubCharacterQuestStateExporter{export: queststate.CharacterQuestStateExport{
		MigrationVersion: queststate.CharacterQuestStateMigrationVersion,
		MigrationName:    queststate.CharacterQuestStateMigrationName,
		Flags: []queststate.CharacterQuestFlagRow{
			{CharacterID: 7, Character: "QuestHero", QuestRef: "quest:first_steps", Flag: "step", Value: 2},
		},
	}}
	mux := RegisterLocalCharacterQuestStateExportEndpoint(NewPprofMux("gamed"), exporter.Export)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state/exports/character-quest-state", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if exporter.calls != 1 {
		t.Fatalf("expected exporter to be called once, got %d", exporter.calls)
	}
	body := rec.Body.String()
	for _, want := range []string{`"migration_version":4`, `"migration_name":"character_quest_state"`, `"character_id":7`, `"character":"QuestHero"`, `"quest_ref":"quest:first_steps"`, `"flag":"step"`, `"value":2`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected character quest-state export body to contain %s, got %s", want, body)
		}
	}
}

func TestLocalCharacterQuestStateExportEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	exporter := &stubCharacterQuestStateExporter{export: queststate.CharacterQuestStateExport{MigrationVersion: 4}}
	mux := RegisterLocalCharacterQuestStateExportEndpoint(NewPprofMux("gamed"), exporter.Export)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state/exports/character-quest-state", nil)
	req.RemoteAddr = "192.0.2.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if exporter.calls != 0 {
		t.Fatalf("expected exporter not to be called, got %d", exporter.calls)
	}
}

func TestLocalCharacterQuestStateExportEndpointRejectsWrongMethod(t *testing.T) {
	exporter := &stubCharacterQuestStateExporter{export: queststate.CharacterQuestStateExport{MigrationVersion: 4}}
	mux := RegisterLocalCharacterQuestStateExportEndpoint(NewPprofMux("gamed"), exporter.Export)

	req := httptest.NewRequest(http.MethodPost, "/local/quest-state/exports/character-quest-state", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if exporter.calls != 0 {
		t.Fatalf("expected exporter not to be called, got %d", exporter.calls)
	}
}

func TestLocalCharacterQuestStateExportEndpointReportsExporterFailure(t *testing.T) {
	exporter := &stubCharacterQuestStateExporter{err: errors.New("invalid quest export")}
	mux := RegisterLocalCharacterQuestStateExportEndpoint(NewPprofMux("gamed"), exporter.Export)

	req := httptest.NewRequest(http.MethodGet, "/local/quest-state/exports/character-quest-state", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if exporter.calls != 1 {
		t.Fatalf("expected exporter to be called once, got %d", exporter.calls)
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

type stubQuestStateTransitionApplier struct {
	calls          int
	lastTransition queststate.Transition
	result         queststate.TransitionApplyResult
	err            error
}

func (s *stubQuestStateTransitionApplier) Apply(transition queststate.Transition) (any, error) {
	s.calls++
	s.lastTransition = transition
	return s.result, s.err
}

type stubQuestStateOverviewReader struct {
	calls    int
	overview queststate.Overview
	err      error
}

func (s *stubQuestStateOverviewReader) Read() (any, error) {
	s.calls++
	return s.overview, s.err
}

type stubQuestStateCharacterReader struct {
	calls         int
	lastCharacter string
	snapshot      queststate.CharacterSnapshot
	ok            bool
	err           error
}

func (s *stubQuestStateCharacterReader) Read(character string) (any, bool, error) {
	s.calls++
	s.lastCharacter = character
	return s.snapshot, s.ok, s.err
}

type stubQuestStateQuestReader struct {
	calls        int
	lastQuestRef string
	snapshot     queststate.QuestSnapshot
	ok           bool
	err          error
}

func (s *stubQuestStateQuestReader) Read(questRef string) (any, bool, error) {
	s.calls++
	s.lastQuestRef = questRef
	return s.snapshot, s.ok, s.err
}

type stubQuestStateFlagReader struct {
	calls         int
	lastCharacter string
	lastQuestRef  string
	lastFlag      string
	flag          queststate.Flag
	ok            bool
	err           error
}

func (s *stubQuestStateFlagReader) Read(character string, questRef string, flag string) (any, bool, error) {
	s.calls++
	s.lastCharacter = character
	s.lastQuestRef = questRef
	s.lastFlag = flag
	return s.flag, s.ok, s.err
}

type stubCharacterQuestStateExporter struct {
	calls  int
	export queststate.CharacterQuestStateExport
	err    error
}

func (s *stubCharacterQuestStateExporter) Export() (queststate.CharacterQuestStateExport, error) {
	s.calls++
	return s.export, s.err
}
