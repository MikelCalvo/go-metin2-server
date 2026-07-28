package ops

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestLocalStaticActorCombatProfileEndpointRejectsOversizedBodyBeforeRegistration(t *testing.T) {
	const profile = "practice_ops_oversized_body"
	worldruntime.UnregisterStaticActorCombatProfileForTest(profile)
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })
	mux := RegisterLocalStaticActorCombatProfileEndpoint(NewPprofMux("gamed"))

	body := `{"profile":"practice_ops_oversized_body","max_hp":24,"attack_value":8,"defense_value":3,"respawn_delay_ms":1500}` + strings.Repeat(" ", maxLocalStaticActorCombatProfileBodyBytes+1)
	req := httptest.NewRequest(http.MethodPost, "/local/static-actor-combat-profiles", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d for oversized profile body, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if _, ok := worldruntime.BootstrapStaticActorCombatProfileDefaults(profile); ok {
		t.Fatalf("expected oversized combat profile request not to register %q", profile)
	}
}

func TestLocalStaticActorCombatProfileEndpointRejectsInvalidUTF8BodyBeforeRegistration(t *testing.T) {
	const profilePrefix = "practice_ops_invalid_"
	mux := RegisterLocalStaticActorCombatProfileEndpoint(NewPprofMux("gamed"))
	body := append([]byte(`{"profile":"`+profilePrefix), 0xff)
	body = append(body, []byte(`","max_hp":24,"attack_value":8,"defense_value":3,"respawn_delay_ms":1500}`)...)
	req := httptest.NewRequest(http.MethodPost, "/local/static-actor-combat-profiles", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for invalid UTF-8 profile body, got %d", http.StatusBadRequest, rec.Code)
	}
}
