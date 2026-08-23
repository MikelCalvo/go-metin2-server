package ops

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
)

type stubCharacterSafeboxStateExporter struct {
	export safeboxstore.CharacterSafeboxStateExport
	err    error
	calls  int
}

func (s *stubCharacterSafeboxStateExporter) Export() (safeboxstore.CharacterSafeboxStateExport, error) {
	s.calls++
	return s.export, s.err
}

func TestLocalCharacterSafeboxStateExportEndpointReturnsLoopbackJSON(t *testing.T) {
	exporter := &stubCharacterSafeboxStateExporter{export: safeboxstore.CharacterSafeboxStateExport{
		MigrationVersion: safeboxstore.CharacterSafeboxStateMigrationVersion,
		MigrationName:    safeboxstore.CharacterSafeboxStateMigrationName,
		Passwords: []safeboxstore.CharacterSafeboxPasswordRow{
			{CharacterID: 7, Login: "Alpha", Password: "secret"},
		},
		Items: []safeboxstore.CharacterSafeboxItemRow{
			{ID: 1001, CharacterID: 7, Login: "Alpha", Cell: 0, Vnum: 27001, Count: 2},
		},
	}}
	mux := RegisterLocalCharacterSafeboxStateExportEndpoint(NewPprofMux("gamed"), exporter.Export)

	req := httptest.NewRequest(http.MethodGet, "/local/safebox-store/exports/character-safebox-state", nil)
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
	for _, want := range []string{`"migration_version":15`, `"migration_name":"character_safebox_money"`, `"character_id":7`, `"login":"Alpha"`, `"password":"secret"`, `"vnum":27001`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected character safebox-state export body to contain %s, got %s", want, body)
		}
	}
}

func TestLocalCharacterSafeboxStateExportEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	exporter := &stubCharacterSafeboxStateExporter{export: safeboxstore.CharacterSafeboxStateExport{MigrationVersion: 14}}
	mux := RegisterLocalCharacterSafeboxStateExportEndpoint(NewPprofMux("gamed"), exporter.Export)
	req := httptest.NewRequest(http.MethodGet, "/local/safebox-store/exports/character-safebox-state", nil)
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

func TestLocalCharacterSafeboxStateExportEndpointRejectsWrongMethod(t *testing.T) {
	exporter := &stubCharacterSafeboxStateExporter{export: safeboxstore.CharacterSafeboxStateExport{MigrationVersion: 14}}
	mux := RegisterLocalCharacterSafeboxStateExportEndpoint(NewPprofMux("gamed"), exporter.Export)
	req := httptest.NewRequest(http.MethodPost, "/local/safebox-store/exports/character-safebox-state", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestLocalCharacterSafeboxStateExportEndpointReportsExporterFailure(t *testing.T) {
	exporter := &stubCharacterSafeboxStateExporter{err: errors.New("invalid safebox export")}
	mux := RegisterLocalCharacterSafeboxStateExportEndpoint(NewPprofMux("gamed"), exporter.Export)
	req := httptest.NewRequest(http.MethodGet, "/local/safebox-store/exports/character-safebox-state", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
}

func TestLocalCharacterSafeboxStateQuarantineEndpointReturnsCanonicalJSON(t *testing.T) {
	payload := safeboxstore.CharacterSafeboxStateExport{
		MigrationVersion: safeboxstore.CharacterSafeboxStateMigrationVersion,
		MigrationName:    safeboxstore.CharacterSafeboxStateMigrationName,
		Passwords: []safeboxstore.CharacterSafeboxPasswordRow{
			{CharacterID: 9, Login: "Beta", Password: ""},
			{CharacterID: 7, Login: "Alpha", Password: "secret"},
		},
		Items: []safeboxstore.CharacterSafeboxItemRow{
			{ID: 1002, CharacterID: 7, Login: "Alpha", Cell: 1, Vnum: 27002, Count: 1},
			{ID: 1001, CharacterID: 7, Login: "Alpha", Cell: 0, Vnum: 27001, Count: 2},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	mux := RegisterLocalCharacterSafeboxStateQuarantineEndpoint(NewPprofMux("gamed"))
	req := httptest.NewRequest(http.MethodPost, "/local/safebox-store/exports/character-safebox-state/quarantine", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	responseBody := rec.Body.String()
	for _, want := range []string{`"character_count":2`, `"password_count":2`, `"item_count":2`, `"character_id":7`, `"login":"Alpha"`} {
		if !strings.Contains(responseBody, want) {
			t.Fatalf("expected quarantine body to contain %s, got %s", want, responseBody)
		}
	}
}

func TestLocalCharacterSafeboxStateQuarantineEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	mux := RegisterLocalCharacterSafeboxStateQuarantineEndpoint(NewPprofMux("gamed"))
	req := httptest.NewRequest(http.MethodPost, "/local/safebox-store/exports/character-safebox-state/quarantine", strings.NewReader(`{"migration_version":15,"migration_name":"character_safebox_money","passwords":[],"items":[]}`))
	req.RemoteAddr = "192.0.2.10:12345"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestLocalCharacterSafeboxStateQuarantineEndpointRejectsWrongMethod(t *testing.T) {
	mux := RegisterLocalCharacterSafeboxStateQuarantineEndpoint(NewPprofMux("gamed"))
	req := httptest.NewRequest(http.MethodGet, "/local/safebox-store/exports/character-safebox-state/quarantine", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestLocalCharacterSafeboxStateQuarantineEndpointRejectsInvalidExport(t *testing.T) {
	mux := RegisterLocalCharacterSafeboxStateQuarantineEndpoint(NewPprofMux("gamed"))
	req := httptest.NewRequest(http.MethodPost, "/local/safebox-store/exports/character-safebox-state/quarantine", strings.NewReader(`{"migration_version":14,"migration_name":"character_safebox_state","passwords":[],"items":[]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
}

func TestNewPprofMuxDoesNotExposeLocalCharacterSafeboxStateByDefault(t *testing.T) {
	mux := NewPprofMux("gamed")
	req := httptest.NewRequest(http.MethodGet, "/local/safebox-store/exports/character-safebox-state", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("default mux must not expose character safebox-state export, got %d", rec.Code)
	}
}
