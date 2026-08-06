package ops

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
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

func TestLocalAccountStoreValidateEndpointReturnsSummaryForLoopbackPost(t *testing.T) {
	validator := &stubAccountStoreValidator{summary: map[string]any{"account_count": 2, "character_count": 3, "logins": []string{"alpha", "zeta"}, "crash_temp_count": 1, "crash_temp_files": []string{".account-crashed.json"}}}
	mux := RegisterLocalAccountStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/validate", nil)
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
	for _, want := range []string{`"account_count":2`, `"character_count":3`, `"logins":["alpha","zeta"]`, `"crash_temp_count":1`, `"crash_temp_files":[".account-crashed.json"]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected JSON content type, got %q", got)
	}
}

func TestLocalAccountStoreValidateEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	validator := &stubAccountStoreValidator{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/validate", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalAccountStoreValidateEndpointRejectsWrongMethod(t *testing.T) {
	validator := &stubAccountStoreValidator{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodGet, "/local/account-store/validate", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalAccountStoreValidateEndpointRejectsUnexpectedBody(t *testing.T) {
	validator := &stubAccountStoreValidator{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/validate", strings.NewReader(`{"confirm":true}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalAccountStoreValidateEndpointRejectsOversizedBody(t *testing.T) {
	validator := &stubAccountStoreValidator{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/validate", strings.NewReader(strings.Repeat(" ", maxLocalAccountStoreMutationBodyBytes+1)))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalAccountStoreValidateEndpointReportsValidationFailure(t *testing.T) {
	validator := &stubAccountStoreValidator{err: errStubAccountStoreInvalid}
	mux := RegisterLocalAccountStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/validate", nil)
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

func TestLocalAccountStoreCrashTempCleanupEndpointReturnsSummaryForLoopbackPost(t *testing.T) {
	cleaner := &stubAccountStoreCrashTempCleaner{summary: map[string]any{"account_count": 2, "character_count": 3, "logins": []string{"alpha", "zeta"}}}
	mux := RegisterLocalAccountStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/crash-temps/cleanup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if cleaner.calls != 1 {
		t.Fatalf("expected cleaner to be called once, got %d", cleaner.calls)
	}
	body := rec.Body.String()
	for _, want := range []string{`"account_count":2`, `"character_count":3`, `"logins":["alpha","zeta"]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected JSON content type, got %q", got)
	}
}

func TestLocalAccountStoreCrashTempCleanupEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	cleaner := &stubAccountStoreCrashTempCleaner{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/crash-temps/cleanup", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalAccountStoreCrashTempCleanupEndpointRejectsWrongMethod(t *testing.T) {
	cleaner := &stubAccountStoreCrashTempCleaner{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodGet, "/local/account-store/crash-temps/cleanup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalAccountStoreCrashTempCleanupEndpointRejectsUnexpectedBody(t *testing.T) {
	cleaner := &stubAccountStoreCrashTempCleaner{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/crash-temps/cleanup", strings.NewReader(`{"confirm":true}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalAccountStoreCrashTempCleanupEndpointRejectsOversizedBody(t *testing.T) {
	cleaner := &stubAccountStoreCrashTempCleaner{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/crash-temps/cleanup", strings.NewReader(strings.Repeat(" ", maxLocalAccountStoreMutationBodyBytes+1)))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalAccountStoreCrashTempCleanupEndpointReportsCleanupFailure(t *testing.T) {
	cleaner := &stubAccountStoreCrashTempCleaner{err: errStubAccountStoreInvalid}
	mux := RegisterLocalAccountStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/crash-temps/cleanup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if cleaner.calls != 1 {
		t.Fatalf("expected cleaner to be called once, got %d", cleaner.calls)
	}
}

func TestLocalAccountStoreBackupEndpointBacksUpToLoopbackRequestedDirectory(t *testing.T) {
	backer := &stubAccountStoreBacker{summary: map[string]any{"account_count": 2, "character_count": 3, "logins": []string{"alpha", "zeta"}}}
	mux := RegisterLocalAccountStoreBackupEndpoint(NewPprofMux("gamed"), backer.Backup)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/backup", strings.NewReader(`{"dst_dir":"/tmp/account-backup"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if backer.calls != 1 || backer.dstDir != "/tmp/account-backup" {
		t.Fatalf("expected backup callback once with requested dst dir, calls=%d dst=%q", backer.calls, backer.dstDir)
	}
	body := rec.Body.String()
	for _, want := range []string{`"account_count":2`, `"character_count":3`, `"logins":["alpha","zeta"]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected JSON content type, got %q", got)
	}
}

func TestLocalAccountStoreBackupEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	backer := &stubAccountStoreBacker{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreBackupEndpoint(NewPprofMux("gamed"), backer.Backup)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/backup", strings.NewReader(`{"dst_dir":"/tmp/account-backup"}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if backer.calls != 0 {
		t.Fatalf("expected backup callback not to be called, got %d", backer.calls)
	}
}

func TestLocalAccountStoreBackupEndpointRejectsInvalidBody(t *testing.T) {
	backer := &stubAccountStoreBacker{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreBackupEndpoint(NewPprofMux("gamed"), backer.Backup)

	for _, body := range []string{``, `{"dst_dir":"   "}`, `{"dst_dir":"/tmp/account-backup","extra":true}`, `{"dst_dir":"/tmp/account-backup"} {}`} {
		t.Run(body, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/local/account-store/backup", strings.NewReader(body))
			req.RemoteAddr = "127.0.0.1:12345"
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}
		})
	}
	if backer.calls != 0 {
		t.Fatalf("expected backup callback not to be called, got %d", backer.calls)
	}
}

func TestLocalAccountStoreBackupEndpointRejectsOversizedBody(t *testing.T) {
	backer := &stubAccountStoreBacker{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreBackupEndpoint(NewPprofMux("gamed"), backer.Backup)
	body := `{"dst_dir":"` + strings.Repeat("a", 4097) + `"}`

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/backup", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if backer.calls != 0 {
		t.Fatalf("expected backup callback not to be called, got %d", backer.calls)
	}
}

func TestLocalAccountStoreBackupEndpointRejectsInvalidUTF8Body(t *testing.T) {
	backer := &stubAccountStoreBacker{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreBackupEndpoint(NewPprofMux("gamed"), backer.Backup)
	body := append([]byte(`{"dst_dir":"/tmp/account-`), 0xff)
	body = append(body, []byte(`-backup"}`)...)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/backup", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if backer.calls != 0 {
		t.Fatalf("expected backup callback not to be called, got %d", backer.calls)
	}
}

func TestDecodeStrictLocalAccountStoreMutationRequestRejectsInvalidUTF8(t *testing.T) {
	raw := append([]byte(`{"dst_dir":"/tmp/account-`), 0xff)
	raw = append(raw, []byte(`-backup"}`)...)

	var request localAccountStoreBackupRequest
	if decodeStrictLocalAccountStoreMutationRequest(raw, &request) {
		t.Fatalf("expected invalid UTF-8 mutation request body to be rejected, got %+v", request)
	}
}

func TestLocalAccountStoreBackupEndpointReportsBackupFailure(t *testing.T) {
	backer := &stubAccountStoreBacker{err: errStubAccountStoreInvalid}
	mux := RegisterLocalAccountStoreBackupEndpoint(NewPprofMux("gamed"), backer.Backup)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/backup", strings.NewReader(`{"dst_dir":"/tmp/account-backup"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if backer.calls != 1 {
		t.Fatalf("expected backup callback to be called once, got %d", backer.calls)
	}
}

func TestLocalAccountStoreBackupEndpointRejectsWrongMethod(t *testing.T) {
	backer := &stubAccountStoreBacker{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreBackupEndpoint(NewPprofMux("gamed"), backer.Backup)

	req := httptest.NewRequest(http.MethodGet, "/local/account-store/backup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if backer.calls != 0 {
		t.Fatalf("expected backup callback not to be called, got %d", backer.calls)
	}
}

func TestLocalAccountStoreBackupValidateEndpointDryRunsLoopbackRequestedSource(t *testing.T) {
	validator := &stubAccountStoreBackupValidator{summary: map[string]any{"account_count": 2, "character_count": 3, "logins": []string{"alpha", "zeta"}, "crash_temp_count": 1, "crash_temp_files": []string{".account-crashed.json"}}}
	mux := RegisterLocalAccountStoreBackupValidateEndpoint(NewPprofMux("gamed"), validator.ValidateBackup)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/backup/validate", strings.NewReader(`{"src_dir":"/tmp/account-backup"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if validator.calls != 1 || validator.srcDir != "/tmp/account-backup" {
		t.Fatalf("expected validate callback once with requested src dir, calls=%d src=%q", validator.calls, validator.srcDir)
	}
	body := rec.Body.String()
	for _, want := range []string{`"account_count":2`, `"character_count":3`, `"logins":["alpha","zeta"]`, `"crash_temp_count":1`, `"crash_temp_files":[".account-crashed.json"]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
}

func TestLocalAccountStoreBackupValidateEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	validator := &stubAccountStoreBackupValidator{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreBackupValidateEndpoint(NewPprofMux("gamed"), validator.ValidateBackup)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/backup/validate", strings.NewReader(`{"src_dir":"/tmp/account-backup"}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validate callback not to be called, got %d", validator.calls)
	}
}

func TestLocalAccountStoreBackupValidateEndpointRejectsInvalidBody(t *testing.T) {
	validator := &stubAccountStoreBackupValidator{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreBackupValidateEndpoint(NewPprofMux("gamed"), validator.ValidateBackup)

	for _, body := range []string{``, `{"src_dir":"   "}`, `{"src_dir":"/tmp/account-backup","extra":true}`, `{"src_dir":"/tmp/account-backup"} {}`} {
		t.Run(body, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/local/account-store/backup/validate", strings.NewReader(body))
			req.RemoteAddr = "127.0.0.1:12345"
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}
		})
	}
	if validator.calls != 0 {
		t.Fatalf("expected validate callback not to be called, got %d", validator.calls)
	}
}

func TestLocalAccountStoreBackupValidateEndpointRejectsOversizedBody(t *testing.T) {
	validator := &stubAccountStoreBackupValidator{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreBackupValidateEndpoint(NewPprofMux("gamed"), validator.ValidateBackup)
	body := `{"src_dir":"` + strings.Repeat("a", 4097) + `"}`

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/backup/validate", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validate callback not to be called, got %d", validator.calls)
	}
}

func TestLocalAccountStoreBackupValidateEndpointReportsValidationFailure(t *testing.T) {
	validator := &stubAccountStoreBackupValidator{err: errStubAccountStoreInvalid}
	mux := RegisterLocalAccountStoreBackupValidateEndpoint(NewPprofMux("gamed"), validator.ValidateBackup)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/backup/validate", strings.NewReader(`{"src_dir":"/tmp/account-backup"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if validator.calls != 1 {
		t.Fatalf("expected validate callback to be called once, got %d", validator.calls)
	}
}

func TestLocalAccountStoreBackupValidateEndpointRejectsWrongMethod(t *testing.T) {
	validator := &stubAccountStoreBackupValidator{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreBackupValidateEndpoint(NewPprofMux("gamed"), validator.ValidateBackup)

	req := httptest.NewRequest(http.MethodGet, "/local/account-store/backup/validate", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validate callback not to be called, got %d", validator.calls)
	}
}

func TestLocalAccountStoreRestoreEndpointRestoresFromLoopbackRequestedDirectory(t *testing.T) {
	restorer := &stubAccountStoreRestorer{summary: map[string]any{"account_count": 2, "character_count": 3, "logins": []string{"alpha", "zeta"}}}
	mux := RegisterLocalAccountStoreRestoreEndpoint(NewPprofMux("gamed"), restorer.Restore)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/restore", strings.NewReader(`{"src_dir":"/tmp/account-backup"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if restorer.calls != 1 || restorer.srcDir != "/tmp/account-backup" {
		t.Fatalf("expected restore callback once with requested src dir, calls=%d src=%q", restorer.calls, restorer.srcDir)
	}
	body := rec.Body.String()
	for _, want := range []string{`"account_count":2`, `"character_count":3`, `"logins":["alpha","zeta"]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected JSON content type, got %q", got)
	}
}

func TestLocalAccountStoreRestoreEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	restorer := &stubAccountStoreRestorer{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreRestoreEndpoint(NewPprofMux("gamed"), restorer.Restore)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/restore", strings.NewReader(`{"src_dir":"/tmp/account-backup"}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if restorer.calls != 0 {
		t.Fatalf("expected restore callback not to be called, got %d", restorer.calls)
	}
}

func TestLocalAccountStoreRestoreEndpointRejectsInvalidBody(t *testing.T) {
	restorer := &stubAccountStoreRestorer{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreRestoreEndpoint(NewPprofMux("gamed"), restorer.Restore)

	for _, body := range []string{``, `{"src_dir":"   "}`, `{"src_dir":"/tmp/account-backup","extra":true}`, `{"src_dir":"/tmp/account-backup"} {}`} {
		t.Run(body, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/local/account-store/restore", strings.NewReader(body))
			req.RemoteAddr = "127.0.0.1:12345"
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}
		})
	}
	if restorer.calls != 0 {
		t.Fatalf("expected restore callback not to be called, got %d", restorer.calls)
	}
}

func TestLocalAccountStoreRestoreEndpointRejectsOversizedBody(t *testing.T) {
	restorer := &stubAccountStoreRestorer{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreRestoreEndpoint(NewPprofMux("gamed"), restorer.Restore)
	body := `{"src_dir":"` + strings.Repeat("a", 4097) + `"}`

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/restore", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if restorer.calls != 0 {
		t.Fatalf("expected restore callback not to be called, got %d", restorer.calls)
	}
}

func TestLocalAccountStoreRestoreEndpointReportsRestoreFailure(t *testing.T) {
	restorer := &stubAccountStoreRestorer{err: errStubAccountStoreInvalid}
	mux := RegisterLocalAccountStoreRestoreEndpoint(NewPprofMux("gamed"), restorer.Restore)

	req := httptest.NewRequest(http.MethodPost, "/local/account-store/restore", strings.NewReader(`{"src_dir":"/tmp/account-backup"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if restorer.calls != 1 {
		t.Fatalf("expected restore callback to be called once, got %d", restorer.calls)
	}
}

func TestLocalAccountStoreRestoreEndpointRejectsWrongMethod(t *testing.T) {
	restorer := &stubAccountStoreRestorer{summary: map[string]any{"account_count": 1}}
	mux := RegisterLocalAccountStoreRestoreEndpoint(NewPprofMux("gamed"), restorer.Restore)

	req := httptest.NewRequest(http.MethodGet, "/local/account-store/restore", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if restorer.calls != 0 {
		t.Fatalf("expected restore callback not to be called, got %d", restorer.calls)
	}
}

func TestLocalLoginTicketStoreValidateEndpointReturnsSummaryForLoopbackPost(t *testing.T) {
	validator := &stubLoginTicketStoreValidator{summary: map[string]any{"ticket_count": 2, "character_count": 5, "empty_character_slot_count": 1, "logins": []string{"alpha", "zeta"}, "login_keys": []uint32{0x01000000, 0x02000000}, "crash_temp_count": 1, "crash_temp_files": []string{".ticket-crashed.json"}}}
	mux := RegisterLocalLoginTicketStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/validate", nil)
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
	for _, want := range []string{`"ticket_count":2`, `"character_count":5`, `"empty_character_slot_count":1`, `"logins":["alpha","zeta"]`, `"login_keys":[16777216,33554432]`, `"crash_temp_count":1`, `"crash_temp_files":[".ticket-crashed.json"]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected JSON content type, got %q", got)
	}
}

func TestLocalLoginTicketStoreValidateEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	validator := &stubLoginTicketStoreValidator{summary: map[string]any{"ticket_count": 1}}
	mux := RegisterLocalLoginTicketStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/validate", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalLoginTicketStoreValidateEndpointRejectsWrongMethod(t *testing.T) {
	validator := &stubLoginTicketStoreValidator{summary: map[string]any{"ticket_count": 1}}
	mux := RegisterLocalLoginTicketStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodGet, "/local/login-tickets/validate", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalLoginTicketStoreValidateEndpointRejectsUnexpectedBody(t *testing.T) {
	validator := &stubLoginTicketStoreValidator{summary: map[string]any{"ticket_count": 1}}
	mux := RegisterLocalLoginTicketStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/validate", strings.NewReader(`{"confirm":true}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalLoginTicketStoreValidateEndpointRejectsOversizedBody(t *testing.T) {
	validator := &stubLoginTicketStoreValidator{summary: map[string]any{"ticket_count": 1}}
	mux := RegisterLocalLoginTicketStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/validate", strings.NewReader(strings.Repeat(" ", maxLocalAccountStoreMutationBodyBytes+1)))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalLoginTicketStoreValidateEndpointReportsValidationFailure(t *testing.T) {
	validator := &stubLoginTicketStoreValidator{err: errStubLoginTicketStoreInvalid}
	mux := RegisterLocalLoginTicketStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/validate", nil)
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

func TestLocalLoginTicketStoreCrashTempCleanupEndpointReturnsSummaryForLoopbackPost(t *testing.T) {
	cleaner := &stubLoginTicketStoreCrashTempCleaner{summary: map[string]any{"ticket_count": 2, "logins": []string{"alpha", "zeta"}, "login_keys": []uint32{0x01000000, 0x02000000}}}
	mux := RegisterLocalLoginTicketStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/crash-temps/cleanup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if cleaner.calls != 1 {
		t.Fatalf("expected cleaner to be called once, got %d", cleaner.calls)
	}
	body := rec.Body.String()
	for _, want := range []string{`"ticket_count":2`, `"logins":["alpha","zeta"]`, `"login_keys":[16777216,33554432]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected JSON content type, got %q", got)
	}
}

func TestLocalLoginTicketStoreCrashTempCleanupEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	cleaner := &stubLoginTicketStoreCrashTempCleaner{summary: map[string]any{"ticket_count": 1}}
	mux := RegisterLocalLoginTicketStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/crash-temps/cleanup", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalLoginTicketStoreCrashTempCleanupEndpointRejectsWrongMethod(t *testing.T) {
	cleaner := &stubLoginTicketStoreCrashTempCleaner{summary: map[string]any{"ticket_count": 1}}
	mux := RegisterLocalLoginTicketStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodGet, "/local/login-tickets/crash-temps/cleanup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalLoginTicketStoreCrashTempCleanupEndpointRejectsUnexpectedBody(t *testing.T) {
	cleaner := &stubLoginTicketStoreCrashTempCleaner{summary: map[string]any{"ticket_count": 1}}
	mux := RegisterLocalLoginTicketStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/crash-temps/cleanup", strings.NewReader(`{"confirm":true}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalLoginTicketStoreCrashTempCleanupEndpointRejectsOversizedBody(t *testing.T) {
	cleaner := &stubLoginTicketStoreCrashTempCleaner{summary: map[string]any{"ticket_count": 1}}
	mux := RegisterLocalLoginTicketStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/crash-temps/cleanup", strings.NewReader(strings.Repeat(" ", maxLocalAccountStoreMutationBodyBytes+1)))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalLoginTicketStoreCrashTempCleanupEndpointReportsCleanupFailure(t *testing.T) {
	cleaner := &stubLoginTicketStoreCrashTempCleaner{err: errStubLoginTicketStoreInvalid}
	mux := RegisterLocalLoginTicketStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/crash-temps/cleanup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if cleaner.calls != 1 {
		t.Fatalf("expected cleaner to be called once, got %d", cleaner.calls)
	}
}

func TestLocalLoginTicketStoreIssuedBeforePreviewEndpointReturnsSummaryForLoopbackPost(t *testing.T) {
	previewer := &stubLoginTicketStoreIssuedBeforePreviewer{summary: map[string]any{"stale_count": 1, "stale_logins": []string{"old"}, "stale_login_keys": []uint32{0x01000000}, "current": map[string]any{"ticket_count": 2, "logins": []string{"new", "old"}, "login_keys": []uint32{0x02000000, 0x01000000}}}}
	mux := RegisterLocalLoginTicketStoreIssuedBeforePreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewIssuedBefore)

	req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/issued-before/preview", strings.NewReader(`{"issued_before":"2026-04-17T11:00:00Z"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected previewer to be called once, got %d", previewer.calls)
	}
	wantCutoff := time.Date(2026, 4, 17, 11, 0, 0, 0, time.UTC)
	if !previewer.issuedBefore.Equal(wantCutoff) {
		t.Fatalf("unexpected issued-before cutoff: got %s want %s", previewer.issuedBefore, wantCutoff)
	}
	body := rec.Body.String()
	for _, want := range []string{`"stale_count":1`, `"stale_logins":["old"]`, `"stale_login_keys":[16777216]`, `"ticket_count":2`, `"logins":["new","old"]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected JSON content type, got %q", got)
	}
}

func TestLocalLoginTicketStoreIssuedBeforePreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubLoginTicketStoreIssuedBeforePreviewer{summary: map[string]any{"stale_count": 1}}
	mux := RegisterLocalLoginTicketStoreIssuedBeforePreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewIssuedBefore)

	req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/issued-before/preview", strings.NewReader(`{"issued_before":"2026-04-17T11:00:00Z"}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected previewer not to be called, got %d", previewer.calls)
	}
}

func TestLocalLoginTicketStoreIssuedBeforePreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubLoginTicketStoreIssuedBeforePreviewer{summary: map[string]any{"stale_count": 1}}
	mux := RegisterLocalLoginTicketStoreIssuedBeforePreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewIssuedBefore)

	req := httptest.NewRequest(http.MethodGet, "/local/login-tickets/issued-before/preview", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected previewer not to be called, got %d", previewer.calls)
	}
}

func TestLocalLoginTicketStoreIssuedBeforePreviewEndpointRejectsInvalidBody(t *testing.T) {
	previewer := &stubLoginTicketStoreIssuedBeforePreviewer{summary: map[string]any{"stale_count": 1}}
	mux := RegisterLocalLoginTicketStoreIssuedBeforePreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewIssuedBefore)

	for _, body := range []string{``, `{"issued_before":""}`, `{"issued_before":"not-time"}`, `{"issued_before":"2026-04-17T11:00:00Z","extra":true}`, `{"issued_before":"2026-04-17T11:00:00Z"} {}`} {
		t.Run(body, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/issued-before/preview", strings.NewReader(body))
			req.RemoteAddr = "127.0.0.1:12345"
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}
		})
	}
	if previewer.calls != 0 {
		t.Fatalf("expected previewer not to be called, got %d", previewer.calls)
	}
}

func TestLocalLoginTicketStoreIssuedBeforePreviewEndpointRejectsOversizedBody(t *testing.T) {
	previewer := &stubLoginTicketStoreIssuedBeforePreviewer{summary: map[string]any{"stale_count": 1}}
	mux := RegisterLocalLoginTicketStoreIssuedBeforePreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewIssuedBefore)
	body := `{"issued_before":"` + strings.Repeat("a", 4097) + `"}`

	req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/issued-before/preview", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected previewer not to be called, got %d", previewer.calls)
	}
}

func TestLocalLoginTicketStoreIssuedBeforePreviewEndpointReportsPreviewFailure(t *testing.T) {
	previewer := &stubLoginTicketStoreIssuedBeforePreviewer{err: errStubLoginTicketStoreInvalid}
	mux := RegisterLocalLoginTicketStoreIssuedBeforePreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewIssuedBefore)

	req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/issued-before/preview", strings.NewReader(`{"issued_before":"2026-04-17T11:00:00Z"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected previewer to be called once, got %d", previewer.calls)
	}
}

func TestLocalLoginTicketStoreIssuedBeforeCleanupEndpointReturnsSummaryForLoopbackPost(t *testing.T) {
	cleaner := &stubLoginTicketStoreIssuedBeforeCleaner{summary: map[string]any{"removed_count": 1, "removed_logins": []string{"old"}, "removed_login_keys": []uint32{0x01000000}, "remaining": map[string]any{"ticket_count": 1, "logins": []string{"new"}, "login_keys": []uint32{0x02000000}}}}
	mux := RegisterLocalLoginTicketStoreIssuedBeforeCleanupEndpoint(NewPprofMux("gamed"), cleaner.CleanupIssuedBefore)

	req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/issued-before/cleanup", strings.NewReader(`{"issued_before":"2026-04-17T11:00:00Z"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if cleaner.calls != 1 {
		t.Fatalf("expected cleaner to be called once, got %d", cleaner.calls)
	}
	wantCutoff := time.Date(2026, 4, 17, 11, 0, 0, 0, time.UTC)
	if !cleaner.issuedBefore.Equal(wantCutoff) {
		t.Fatalf("unexpected issued-before cutoff: got %s want %s", cleaner.issuedBefore, wantCutoff)
	}
	body := rec.Body.String()
	for _, want := range []string{`"removed_count":1`, `"removed_logins":["old"]`, `"removed_login_keys":[16777216]`, `"ticket_count":1`, `"logins":["new"]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected JSON content type, got %q", got)
	}
}

func TestLocalLoginTicketStoreIssuedBeforeCleanupEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	cleaner := &stubLoginTicketStoreIssuedBeforeCleaner{summary: map[string]any{"removed_count": 1}}
	mux := RegisterLocalLoginTicketStoreIssuedBeforeCleanupEndpoint(NewPprofMux("gamed"), cleaner.CleanupIssuedBefore)

	req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/issued-before/cleanup", strings.NewReader(`{"issued_before":"2026-04-17T11:00:00Z"}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalLoginTicketStoreIssuedBeforeCleanupEndpointRejectsWrongMethod(t *testing.T) {
	cleaner := &stubLoginTicketStoreIssuedBeforeCleaner{summary: map[string]any{"removed_count": 1}}
	mux := RegisterLocalLoginTicketStoreIssuedBeforeCleanupEndpoint(NewPprofMux("gamed"), cleaner.CleanupIssuedBefore)

	req := httptest.NewRequest(http.MethodGet, "/local/login-tickets/issued-before/cleanup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalLoginTicketStoreIssuedBeforeCleanupEndpointRejectsInvalidBody(t *testing.T) {
	cleaner := &stubLoginTicketStoreIssuedBeforeCleaner{summary: map[string]any{"removed_count": 1}}
	mux := RegisterLocalLoginTicketStoreIssuedBeforeCleanupEndpoint(NewPprofMux("gamed"), cleaner.CleanupIssuedBefore)

	for _, body := range []string{``, `{"issued_before":""}`, `{"issued_before":"not-time"}`, `{"issued_before":"2026-04-17T11:00:00Z","extra":true}`, `{"issued_before":"2026-04-17T11:00:00Z"} {}`} {
		t.Run(body, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/issued-before/cleanup", strings.NewReader(body))
			req.RemoteAddr = "127.0.0.1:12345"
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}
		})
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalLoginTicketStoreIssuedBeforeCleanupEndpointRejectsOversizedBody(t *testing.T) {
	cleaner := &stubLoginTicketStoreIssuedBeforeCleaner{summary: map[string]any{"removed_count": 1}}
	mux := RegisterLocalLoginTicketStoreIssuedBeforeCleanupEndpoint(NewPprofMux("gamed"), cleaner.CleanupIssuedBefore)
	body := `{"issued_before":"` + strings.Repeat("a", 4097) + `"}`

	req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/issued-before/cleanup", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalLoginTicketStoreIssuedBeforeCleanupEndpointReportsCleanupFailure(t *testing.T) {
	cleaner := &stubLoginTicketStoreIssuedBeforeCleaner{err: errStubLoginTicketStoreInvalid}
	mux := RegisterLocalLoginTicketStoreIssuedBeforeCleanupEndpoint(NewPprofMux("gamed"), cleaner.CleanupIssuedBefore)

	req := httptest.NewRequest(http.MethodPost, "/local/login-tickets/issued-before/cleanup", strings.NewReader(`{"issued_before":"2026-04-17T11:00:00Z"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if cleaner.calls != 1 {
		t.Fatalf("expected cleaner to be called once, got %d", cleaner.calls)
	}
}

func TestLocalItemTemplateStoreValidateEndpointReturnsSummaryForLoopbackPost(t *testing.T) {
	validator := &stubItemTemplateStoreValidator{summary: map[string]any{"template_count": 2, "vnums": []uint32{11200, 27001}, "crash_temp_count": 1, "crash_temp_files": []string{".item-templates-crashed.json"}}}
	mux := RegisterLocalItemTemplateStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/validate", nil)
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
	for _, want := range []string{`"template_count":2`, `"vnums":[11200,27001]`, `"crash_temp_count":1`, `"crash_temp_files":[".item-templates-crashed.json"]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected JSON content type, got %q", got)
	}
}

func TestLocalItemTemplateStoreValidateEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	validator := &stubItemTemplateStoreValidator{summary: map[string]any{"template_count": 1}}
	mux := RegisterLocalItemTemplateStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/validate", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalItemTemplateStoreValidateEndpointRejectsWrongMethod(t *testing.T) {
	validator := &stubItemTemplateStoreValidator{summary: map[string]any{"template_count": 1}}
	mux := RegisterLocalItemTemplateStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodGet, "/local/item-templates/validate", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalItemTemplateStoreValidateEndpointRejectsUnexpectedBody(t *testing.T) {
	validator := &stubItemTemplateStoreValidator{summary: map[string]any{"template_count": 1}}
	mux := RegisterLocalItemTemplateStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/validate", strings.NewReader(`{"confirm":true}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalItemTemplateStoreValidateEndpointRejectsOversizedBody(t *testing.T) {
	validator := &stubItemTemplateStoreValidator{summary: map[string]any{"template_count": 1}}
	mux := RegisterLocalItemTemplateStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/validate", strings.NewReader(strings.Repeat(" ", maxLocalAccountStoreMutationBodyBytes+1)))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalItemTemplateStoreValidateEndpointReportsValidationFailure(t *testing.T) {
	validator := &stubItemTemplateStoreValidator{err: errStubItemTemplateStoreInvalid}
	mux := RegisterLocalItemTemplateStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/validate", nil)
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

func TestLocalStaticActorStoreValidateEndpointReturnsSummaryForLoopbackPost(t *testing.T) {
	validator := &stubStaticActorStoreValidator{summary: map[string]any{"actor_count": 2, "actor_ids": []uint64{7, 9}, "actor_names": []string{"TrainingDummy", "VillageGuard"}, "crash_temp_count": 1, "crash_temp_files": []string{".static-actors-crashed.json"}}}
	mux := RegisterLocalStaticActorStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actor-store/validate", nil)
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
	for _, want := range []string{`"actor_count":2`, `"actor_ids":[7,9]`, `"actor_names":["TrainingDummy","VillageGuard"]`, `"crash_temp_count":1`, `"crash_temp_files":[".static-actors-crashed.json"]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected JSON content type, got %q", got)
	}
}

func TestLocalStaticActorStoreValidateEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	validator := &stubStaticActorStoreValidator{summary: map[string]any{"actor_count": 1}}
	mux := RegisterLocalStaticActorStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actor-store/validate", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalStaticActorStoreValidateEndpointRejectsWrongMethod(t *testing.T) {
	validator := &stubStaticActorStoreValidator{summary: map[string]any{"actor_count": 1}}
	mux := RegisterLocalStaticActorStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodGet, "/local/static-actor-store/validate", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalStaticActorStoreValidateEndpointRejectsUnexpectedBody(t *testing.T) {
	validator := &stubStaticActorStoreValidator{summary: map[string]any{"actor_count": 1}}
	mux := RegisterLocalStaticActorStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actor-store/validate", strings.NewReader(`{"confirm":true}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalStaticActorStoreValidateEndpointRejectsOversizedBody(t *testing.T) {
	validator := &stubStaticActorStoreValidator{summary: map[string]any{"actor_count": 1}}
	mux := RegisterLocalStaticActorStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actor-store/validate", strings.NewReader(strings.Repeat(" ", maxLocalAccountStoreMutationBodyBytes+1)))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalStaticActorStoreValidateEndpointReportsValidationFailure(t *testing.T) {
	validator := &stubStaticActorStoreValidator{err: errStubStaticActorStoreInvalid}
	mux := RegisterLocalStaticActorStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actor-store/validate", nil)
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

func TestLocalInteractionStoreValidateEndpointReturnsSummaryForLoopbackPost(t *testing.T) {
	validator := &stubInteractionStoreValidator{summary: map[string]any{"definition_count": 2, "definition_keys": []string{"info:lore:alchemist", "talk:npc:village_guard"}, "crash_temp_count": 1, "crash_temp_files": []string{".interaction-definitions-crashed.json"}}}
	mux := RegisterLocalInteractionStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/interaction-store/validate", nil)
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
	for _, want := range []string{`"definition_count":2`, `"definition_keys":["info:lore:alchemist","talk:npc:village_guard"]`, `"crash_temp_count":1`, `"crash_temp_files":[".interaction-definitions-crashed.json"]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected JSON content type, got %q", got)
	}
}

func TestLocalInteractionStoreValidateEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	validator := &stubInteractionStoreValidator{summary: map[string]any{"definition_count": 1}}
	mux := RegisterLocalInteractionStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/interaction-store/validate", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalInteractionStoreValidateEndpointRejectsWrongMethod(t *testing.T) {
	validator := &stubInteractionStoreValidator{summary: map[string]any{"definition_count": 1}}
	mux := RegisterLocalInteractionStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodGet, "/local/interaction-store/validate", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalInteractionStoreValidateEndpointRejectsUnexpectedBody(t *testing.T) {
	validator := &stubInteractionStoreValidator{summary: map[string]any{"definition_count": 1}}
	mux := RegisterLocalInteractionStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/interaction-store/validate", strings.NewReader(`{"confirm":true}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalInteractionStoreValidateEndpointRejectsOversizedBody(t *testing.T) {
	validator := &stubInteractionStoreValidator{summary: map[string]any{"definition_count": 1}}
	mux := RegisterLocalInteractionStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/interaction-store/validate", strings.NewReader(strings.Repeat(" ", maxLocalAccountStoreMutationBodyBytes+1)))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d", validator.calls)
	}
}

func TestLocalInteractionStoreValidateEndpointReportsValidationFailure(t *testing.T) {
	validator := &stubInteractionStoreValidator{err: errStubInteractionStoreInvalid}
	mux := RegisterLocalInteractionStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)

	req := httptest.NewRequest(http.MethodPost, "/local/interaction-store/validate", nil)
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

func TestLocalItemTemplateStoreCrashTempCleanupEndpointReturnsSummaryForLoopbackPost(t *testing.T) {
	cleaner := &stubItemTemplateStoreCrashTempCleaner{summary: map[string]any{"template_count": 2, "vnums": []uint32{11200, 27001}}}
	mux := RegisterLocalItemTemplateStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/crash-temps/cleanup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if cleaner.calls != 1 {
		t.Fatalf("expected cleaner to be called once, got %d", cleaner.calls)
	}
	body := rec.Body.String()
	for _, want := range []string{`"template_count":2`, `"vnums":[11200,27001]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected JSON content type, got %q", got)
	}
}

func TestLocalItemTemplateStoreCrashTempCleanupEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	cleaner := &stubItemTemplateStoreCrashTempCleaner{summary: map[string]any{"template_count": 1}}
	mux := RegisterLocalItemTemplateStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/crash-temps/cleanup", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalItemTemplateStoreCrashTempCleanupEndpointRejectsWrongMethod(t *testing.T) {
	cleaner := &stubItemTemplateStoreCrashTempCleaner{summary: map[string]any{"template_count": 1}}
	mux := RegisterLocalItemTemplateStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodGet, "/local/item-templates/crash-temps/cleanup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalItemTemplateStoreCrashTempCleanupEndpointRejectsUnexpectedBody(t *testing.T) {
	cleaner := &stubItemTemplateStoreCrashTempCleaner{summary: map[string]any{"template_count": 1}}
	mux := RegisterLocalItemTemplateStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/crash-temps/cleanup", strings.NewReader(`{"confirm":true}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalItemTemplateStoreCrashTempCleanupEndpointRejectsOversizedBody(t *testing.T) {
	cleaner := &stubItemTemplateStoreCrashTempCleaner{summary: map[string]any{"template_count": 1}}
	mux := RegisterLocalItemTemplateStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/crash-temps/cleanup", strings.NewReader(strings.Repeat(" ", maxLocalAccountStoreMutationBodyBytes+1)))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalItemTemplateStoreCrashTempCleanupEndpointReportsCleanupFailure(t *testing.T) {
	cleaner := &stubItemTemplateStoreCrashTempCleaner{err: errStubItemTemplateStoreInvalid}
	mux := RegisterLocalItemTemplateStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/crash-temps/cleanup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if cleaner.calls != 1 {
		t.Fatalf("expected cleaner to be called once, got %d", cleaner.calls)
	}
}

func TestLocalStaticActorStoreCrashTempCleanupEndpointReturnsSummaryForLoopbackPost(t *testing.T) {
	cleaner := &stubStaticActorStoreCrashTempCleaner{summary: map[string]any{"actor_count": 2, "actor_ids": []uint64{7, 9}, "actor_names": []string{"TrainingDummy", "VillageGuard"}}}
	mux := RegisterLocalStaticActorStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actor-store/crash-temps/cleanup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if cleaner.calls != 1 {
		t.Fatalf("expected cleaner to be called once, got %d", cleaner.calls)
	}
	body := rec.Body.String()
	for _, want := range []string{`"actor_count":2`, `"actor_ids":[7,9]`, `"actor_names":["TrainingDummy","VillageGuard"]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected JSON content type, got %q", got)
	}
}

func TestLocalStaticActorStoreCrashTempCleanupEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	cleaner := &stubStaticActorStoreCrashTempCleaner{summary: map[string]any{"actor_count": 1}}
	mux := RegisterLocalStaticActorStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actor-store/crash-temps/cleanup", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalStaticActorStoreCrashTempCleanupEndpointRejectsWrongMethod(t *testing.T) {
	cleaner := &stubStaticActorStoreCrashTempCleaner{summary: map[string]any{"actor_count": 1}}
	mux := RegisterLocalStaticActorStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodGet, "/local/static-actor-store/crash-temps/cleanup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalStaticActorStoreCrashTempCleanupEndpointRejectsUnexpectedBody(t *testing.T) {
	cleaner := &stubStaticActorStoreCrashTempCleaner{summary: map[string]any{"actor_count": 1}}
	mux := RegisterLocalStaticActorStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actor-store/crash-temps/cleanup", strings.NewReader(`{"confirm":true}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalStaticActorStoreCrashTempCleanupEndpointRejectsOversizedBody(t *testing.T) {
	cleaner := &stubStaticActorStoreCrashTempCleaner{summary: map[string]any{"actor_count": 1}}
	mux := RegisterLocalStaticActorStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actor-store/crash-temps/cleanup", strings.NewReader(strings.Repeat(" ", maxLocalAccountStoreMutationBodyBytes+1)))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalStaticActorStoreCrashTempCleanupEndpointReportsCleanupFailure(t *testing.T) {
	cleaner := &stubStaticActorStoreCrashTempCleaner{err: errStubStaticActorStoreInvalid}
	mux := RegisterLocalStaticActorStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actor-store/crash-temps/cleanup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if cleaner.calls != 1 {
		t.Fatalf("expected cleaner to be called once, got %d", cleaner.calls)
	}
}

func TestLocalInteractionStoreCrashTempCleanupEndpointReturnsSummaryForLoopbackPost(t *testing.T) {
	cleaner := &stubInteractionStoreCrashTempCleaner{summary: map[string]any{"definition_count": 2, "definition_keys": []string{"info:lore:alchemist", "talk:npc:village_guard"}}}
	mux := RegisterLocalInteractionStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/interaction-store/crash-temps/cleanup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if cleaner.calls != 1 {
		t.Fatalf("expected cleaner to be called once, got %d", cleaner.calls)
	}
	body := rec.Body.String()
	for _, want := range []string{`"definition_count":2`, `"definition_keys":["info:lore:alchemist","talk:npc:village_guard"]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected JSON content type, got %q", got)
	}
}

func TestLocalInteractionStoreCrashTempCleanupEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	cleaner := &stubInteractionStoreCrashTempCleaner{summary: map[string]any{"definition_count": 1}}
	mux := RegisterLocalInteractionStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/interaction-store/crash-temps/cleanup", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalInteractionStoreCrashTempCleanupEndpointRejectsWrongMethod(t *testing.T) {
	cleaner := &stubInteractionStoreCrashTempCleaner{summary: map[string]any{"definition_count": 1}}
	mux := RegisterLocalInteractionStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodGet, "/local/interaction-store/crash-temps/cleanup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalInteractionStoreCrashTempCleanupEndpointRejectsUnexpectedBody(t *testing.T) {
	cleaner := &stubInteractionStoreCrashTempCleaner{summary: map[string]any{"definition_count": 1}}
	mux := RegisterLocalInteractionStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/interaction-store/crash-temps/cleanup", strings.NewReader(`{"confirm":true}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalInteractionStoreCrashTempCleanupEndpointRejectsOversizedBody(t *testing.T) {
	cleaner := &stubInteractionStoreCrashTempCleaner{summary: map[string]any{"definition_count": 1}}
	mux := RegisterLocalInteractionStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/interaction-store/crash-temps/cleanup", strings.NewReader(strings.Repeat(" ", maxLocalAccountStoreMutationBodyBytes+1)))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if cleaner.calls != 0 {
		t.Fatalf("expected cleaner not to be called, got %d", cleaner.calls)
	}
}

func TestLocalInteractionStoreCrashTempCleanupEndpointReportsCleanupFailure(t *testing.T) {
	cleaner := &stubInteractionStoreCrashTempCleaner{err: errStubInteractionStoreInvalid}
	mux := RegisterLocalInteractionStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Cleanup)

	req := httptest.NewRequest(http.MethodPost, "/local/interaction-store/crash-temps/cleanup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if cleaner.calls != 1 {
		t.Fatalf("expected cleaner to be called once, got %d", cleaner.calls)
	}
}

func TestLocalItemTemplateStoreBackupEndpointBacksUpToLoopbackRequestedDirectory(t *testing.T) {
	backer := &stubItemTemplateStoreBacker{summary: map[string]any{"template_count": 2, "vnums": []uint32{11200, 27001}}}
	mux := RegisterLocalItemTemplateStoreBackupEndpoint(NewPprofMux("gamed"), backer.Backup)

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/backup", strings.NewReader(`{"dst_dir":"/tmp/item-template-backup"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if backer.calls != 1 || backer.dstDir != "/tmp/item-template-backup" {
		t.Fatalf("expected backup callback once with requested dst dir, calls=%d dst=%q", backer.calls, backer.dstDir)
	}
	body := rec.Body.String()
	for _, want := range []string{`"template_count":2`, `"vnums":[11200,27001]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected JSON content type, got %q", got)
	}
}

func TestLocalItemTemplateStoreBackupEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	backer := &stubItemTemplateStoreBacker{summary: map[string]any{"template_count": 1}}
	mux := RegisterLocalItemTemplateStoreBackupEndpoint(NewPprofMux("gamed"), backer.Backup)

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/backup", strings.NewReader(`{"dst_dir":"/tmp/item-template-backup"}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if backer.calls != 0 {
		t.Fatalf("expected backup callback not to be called, got %d", backer.calls)
	}
}

func TestLocalItemTemplateStoreBackupEndpointRejectsInvalidBody(t *testing.T) {
	backer := &stubItemTemplateStoreBacker{summary: map[string]any{"template_count": 1}}
	mux := RegisterLocalItemTemplateStoreBackupEndpoint(NewPprofMux("gamed"), backer.Backup)

	for _, body := range []string{``, `{"dst_dir":"   "}`, `{"dst_dir":"/tmp/item-template-backup","extra":true}`, `{"dst_dir":"/tmp/item-template-backup"} {}`} {
		t.Run(body, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/local/item-templates/backup", strings.NewReader(body))
			req.RemoteAddr = "127.0.0.1:12345"
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}
		})
	}
	if backer.calls != 0 {
		t.Fatalf("expected backup callback not to be called, got %d", backer.calls)
	}
}

func TestLocalItemTemplateStoreBackupEndpointReportsBackupFailure(t *testing.T) {
	backer := &stubItemTemplateStoreBacker{err: errStubItemTemplateStoreInvalid}
	mux := RegisterLocalItemTemplateStoreBackupEndpoint(NewPprofMux("gamed"), backer.Backup)

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/backup", strings.NewReader(`{"dst_dir":"/tmp/item-template-backup"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if backer.calls != 1 {
		t.Fatalf("expected backup callback to be called once, got %d", backer.calls)
	}
}

func TestLocalItemTemplateStoreBackupEndpointRejectsWrongMethod(t *testing.T) {
	backer := &stubItemTemplateStoreBacker{summary: map[string]any{"template_count": 1}}
	mux := RegisterLocalItemTemplateStoreBackupEndpoint(NewPprofMux("gamed"), backer.Backup)

	req := httptest.NewRequest(http.MethodGet, "/local/item-templates/backup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if backer.calls != 0 {
		t.Fatalf("expected backup callback not to be called, got %d", backer.calls)
	}
}

func TestLocalItemTemplateStoreBackupValidateEndpointDryRunsLoopbackRequestedSource(t *testing.T) {
	validator := &stubItemTemplateStoreBackupValidator{summary: map[string]any{"template_count": 2, "vnums": []uint32{11200, 27001}, "crash_temp_count": 1, "crash_temp_files": []string{".item-templates-crashed.json"}}}
	mux := RegisterLocalItemTemplateStoreBackupValidateEndpoint(NewPprofMux("gamed"), validator.ValidateBackup)

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/backup/validate", strings.NewReader(`{"src_dir":"/tmp/item-template-backup"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if validator.calls != 1 || validator.srcDir != "/tmp/item-template-backup" {
		t.Fatalf("expected validate callback once with requested src dir, calls=%d src=%q", validator.calls, validator.srcDir)
	}
	body := rec.Body.String()
	for _, want := range []string{`"template_count":2`, `"vnums":[11200,27001]`, `"crash_temp_count":1`, `"crash_temp_files":[".item-templates-crashed.json"]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
}

func TestLocalItemTemplateStoreBackupValidateEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	validator := &stubItemTemplateStoreBackupValidator{summary: map[string]any{"template_count": 1}}
	mux := RegisterLocalItemTemplateStoreBackupValidateEndpoint(NewPprofMux("gamed"), validator.ValidateBackup)

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/backup/validate", strings.NewReader(`{"src_dir":"/tmp/item-template-backup"}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validate callback not to be called, got %d", validator.calls)
	}
}

func TestLocalItemTemplateStoreBackupValidateEndpointRejectsInvalidBody(t *testing.T) {
	validator := &stubItemTemplateStoreBackupValidator{summary: map[string]any{"template_count": 1}}
	mux := RegisterLocalItemTemplateStoreBackupValidateEndpoint(NewPprofMux("gamed"), validator.ValidateBackup)

	for _, body := range []string{``, `{"src_dir":"   "}`, `{"src_dir":"/tmp/item-template-backup","extra":true}`, `{"src_dir":"/tmp/item-template-backup"} {}`} {
		t.Run(body, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/local/item-templates/backup/validate", strings.NewReader(body))
			req.RemoteAddr = "127.0.0.1:12345"
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}
		})
	}
	if validator.calls != 0 {
		t.Fatalf("expected validate callback not to be called, got %d", validator.calls)
	}
}

func TestLocalItemTemplateStoreBackupValidateEndpointReportsValidationFailure(t *testing.T) {
	validator := &stubItemTemplateStoreBackupValidator{err: errStubItemTemplateStoreInvalid}
	mux := RegisterLocalItemTemplateStoreBackupValidateEndpoint(NewPprofMux("gamed"), validator.ValidateBackup)

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/backup/validate", strings.NewReader(`{"src_dir":"/tmp/item-template-backup"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if validator.calls != 1 {
		t.Fatalf("expected validate callback to be called once, got %d", validator.calls)
	}
}

func TestLocalItemTemplateStoreBackupValidateEndpointRejectsWrongMethod(t *testing.T) {
	validator := &stubItemTemplateStoreBackupValidator{summary: map[string]any{"template_count": 1}}
	mux := RegisterLocalItemTemplateStoreBackupValidateEndpoint(NewPprofMux("gamed"), validator.ValidateBackup)

	req := httptest.NewRequest(http.MethodGet, "/local/item-templates/backup/validate", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validate callback not to be called, got %d", validator.calls)
	}
}

func TestLocalItemTemplateStoreRestoreEndpointRestoresFromLoopbackRequestedSource(t *testing.T) {
	restorer := &stubItemTemplateStoreRestorer{summary: map[string]any{"template_count": 2, "vnums": []uint32{11200, 27001}}}
	mux := RegisterLocalItemTemplateStoreRestoreEndpoint(NewPprofMux("gamed"), restorer.Restore)

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/restore", strings.NewReader(`{"src_dir":"/tmp/item-template-backup"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if restorer.calls != 1 || restorer.srcDir != "/tmp/item-template-backup" {
		t.Fatalf("expected restore callback once with requested src dir, calls=%d src=%q", restorer.calls, restorer.srcDir)
	}
	body := rec.Body.String()
	for _, want := range []string{`"template_count":2`, `"vnums":[11200,27001]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected JSON content type, got %q", got)
	}
}

func TestLocalItemTemplateStoreRestoreEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	restorer := &stubItemTemplateStoreRestorer{summary: map[string]any{"template_count": 1}}
	mux := RegisterLocalItemTemplateStoreRestoreEndpoint(NewPprofMux("gamed"), restorer.Restore)

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/restore", strings.NewReader(`{"src_dir":"/tmp/item-template-backup"}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if restorer.calls != 0 {
		t.Fatalf("expected restore callback not to be called, got %d", restorer.calls)
	}
}

func TestLocalItemTemplateStoreRestoreEndpointRejectsInvalidBody(t *testing.T) {
	restorer := &stubItemTemplateStoreRestorer{summary: map[string]any{"template_count": 1}}
	mux := RegisterLocalItemTemplateStoreRestoreEndpoint(NewPprofMux("gamed"), restorer.Restore)

	for _, body := range []string{``, `{"src_dir":"   "}`, `{"src_dir":"/tmp/item-template-backup","extra":true}`, `{"src_dir":"/tmp/item-template-backup"} {}`} {
		t.Run(body, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/local/item-templates/restore", strings.NewReader(body))
			req.RemoteAddr = "127.0.0.1:12345"
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}
		})
	}
	if restorer.calls != 0 {
		t.Fatalf("expected restore callback not to be called, got %d", restorer.calls)
	}
}

func TestLocalItemTemplateStoreRestoreEndpointRejectsOversizedBody(t *testing.T) {
	restorer := &stubItemTemplateStoreRestorer{summary: map[string]any{"template_count": 1}}
	mux := RegisterLocalItemTemplateStoreRestoreEndpoint(NewPprofMux("gamed"), restorer.Restore)
	body := `{"src_dir":"` + strings.Repeat("a", maxLocalAccountStoreMutationBodyBytes+1) + `"}`

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/restore", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if restorer.calls != 0 {
		t.Fatalf("expected restore callback not to be called, got %d", restorer.calls)
	}
}

func TestLocalItemTemplateStoreRestoreEndpointReportsRestoreFailure(t *testing.T) {
	restorer := &stubItemTemplateStoreRestorer{err: errStubItemTemplateStoreInvalid}
	mux := RegisterLocalItemTemplateStoreRestoreEndpoint(NewPprofMux("gamed"), restorer.Restore)

	req := httptest.NewRequest(http.MethodPost, "/local/item-templates/restore", strings.NewReader(`{"src_dir":"/tmp/item-template-backup"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if restorer.calls != 1 {
		t.Fatalf("expected restore callback to be called once, got %d", restorer.calls)
	}
}

func TestLocalItemTemplateStoreRestoreEndpointRejectsWrongMethod(t *testing.T) {
	restorer := &stubItemTemplateStoreRestorer{summary: map[string]any{"template_count": 1}}
	mux := RegisterLocalItemTemplateStoreRestoreEndpoint(NewPprofMux("gamed"), restorer.Restore)

	req := httptest.NewRequest(http.MethodGet, "/local/item-templates/restore", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if restorer.calls != 0 {
		t.Fatalf("expected restore callback not to be called, got %d", restorer.calls)
	}
}

func TestLocalQuickslotsEndpointReturnsNamedSnapshotForLoopbackGet(t *testing.T) {
	calledName := ""
	mux := RegisterLocalQuickslotsEndpoint(NewPprofMux("gamed"), func(name string) (any, bool) {
		calledName = name
		if name != "MkmkWar" {
			return nil, false
		}
		return map[string]any{
			"name": "MkmkWar",
			"quickslots": []map[string]any{{
				"position": 2,
				"type":     1,
				"slot":     5,
			}},
		}, true
	})

	req := httptest.NewRequest(http.MethodGet, "/local/quickslots/MkmkWar", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if calledName != "MkmkWar" {
		t.Fatalf("expected snapshot callback for MkmkWar, got %q", calledName)
	}
	body := rec.Body.String()
	for _, want := range []string{`"name":"MkmkWar"`, `"quickslots":[{"position":2,"slot":5,"type":1}]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected JSON content type, got %q", got)
	}
}

func TestLocalQuickslotsEndpointRejectsNonLoopbackAndUnknownNames(t *testing.T) {
	calls := 0
	mux := RegisterLocalQuickslotsEndpoint(NewPprofMux("gamed"), func(name string) (any, bool) {
		calls++
		return nil, false
	})

	nonLoopback := httptest.NewRequest(http.MethodGet, "/local/quickslots/MkmkWar", nil)
	nonLoopback.RemoteAddr = "203.0.113.10:12345"
	nonLoopbackRec := httptest.NewRecorder()
	mux.ServeHTTP(nonLoopbackRec, nonLoopback)
	if nonLoopbackRec.Code != http.StatusForbidden {
		t.Fatalf("expected non-loopback status %d, got %d", http.StatusForbidden, nonLoopbackRec.Code)
	}
	if calls != 0 {
		t.Fatalf("expected callback not to be called for non-loopback request, got %d calls", calls)
	}

	missing := httptest.NewRequest(http.MethodGet, "/local/quickslots/MissingWar", nil)
	missing.RemoteAddr = "127.0.0.1:12345"
	missingRec := httptest.NewRecorder()
	mux.ServeHTTP(missingRec, missing)
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("expected missing quickslot snapshot status %d, got %d", http.StatusNotFound, missingRec.Code)
	}
	if calls != 1 {
		t.Fatalf("expected callback to be called once for missing loopback request, got %d", calls)
	}
}

type stubAccountStoreValidator struct {
	summary any
	err     error
	calls   int
}

func (s *stubAccountStoreValidator) Validate() (any, error) {
	s.calls++
	return s.summary, s.err
}

var errStubAccountStoreInvalid = errors.New("account store invalid")

type stubAccountStoreCrashTempCleaner struct {
	summary any
	err     error
	calls   int
}

func (s *stubAccountStoreCrashTempCleaner) Cleanup() (any, error) {
	s.calls++
	return s.summary, s.err
}

type stubAccountStoreBacker struct {
	summary any
	err     error
	calls   int
	dstDir  string
}

func (s *stubAccountStoreBacker) Backup(dstDir string) (any, error) {
	s.calls++
	s.dstDir = dstDir
	return s.summary, s.err
}

type stubAccountStoreBackupValidator struct {
	summary any
	err     error
	calls   int
	srcDir  string
}

func (s *stubAccountStoreBackupValidator) ValidateBackup(srcDir string) (any, error) {
	s.calls++
	s.srcDir = srcDir
	return s.summary, s.err
}

type stubAccountStoreRestorer struct {
	summary any
	err     error
	calls   int
	srcDir  string
}

func (s *stubAccountStoreRestorer) Restore(srcDir string) (any, error) {
	s.calls++
	s.srcDir = srcDir
	return s.summary, s.err
}

type stubLoginTicketStoreValidator struct {
	summary any
	err     error
	calls   int
}

func (s *stubLoginTicketStoreValidator) Validate() (any, error) {
	s.calls++
	return s.summary, s.err
}

var errStubLoginTicketStoreInvalid = errors.New("login ticket store invalid")

type stubLoginTicketStoreCrashTempCleaner struct {
	summary any
	err     error
	calls   int
}

func (s *stubLoginTicketStoreCrashTempCleaner) Cleanup() (any, error) {
	s.calls++
	return s.summary, s.err
}

type stubLoginTicketStoreIssuedBeforePreviewer struct {
	summary      any
	err          error
	calls        int
	issuedBefore time.Time
}

func (s *stubLoginTicketStoreIssuedBeforePreviewer) PreviewIssuedBefore(issuedBefore time.Time) (any, error) {
	s.calls++
	s.issuedBefore = issuedBefore
	return s.summary, s.err
}

type stubLoginTicketStoreIssuedBeforeCleaner struct {
	summary      any
	err          error
	calls        int
	issuedBefore time.Time
}

func (s *stubLoginTicketStoreIssuedBeforeCleaner) CleanupIssuedBefore(issuedBefore time.Time) (any, error) {
	s.calls++
	s.issuedBefore = issuedBefore
	return s.summary, s.err
}

type stubItemTemplateStoreValidator struct {
	summary any
	err     error
	calls   int
}

func (s *stubItemTemplateStoreValidator) Validate() (any, error) {
	s.calls++
	return s.summary, s.err
}

var errStubItemTemplateStoreInvalid = errors.New("item template store invalid")

type stubItemTemplateStoreCrashTempCleaner struct {
	summary any
	err     error
	calls   int
}

func (s *stubItemTemplateStoreCrashTempCleaner) Cleanup() (any, error) {
	s.calls++
	return s.summary, s.err
}

type stubStaticActorStoreValidator struct {
	summary any
	err     error
	calls   int
}

func (s *stubStaticActorStoreValidator) Validate() (any, error) {
	s.calls++
	return s.summary, s.err
}

type stubStaticActorStoreCrashTempCleaner struct {
	summary any
	err     error
	calls   int
}

func (s *stubStaticActorStoreCrashTempCleaner) Cleanup() (any, error) {
	s.calls++
	return s.summary, s.err
}

var errStubStaticActorStoreInvalid = errors.New("static actor store invalid")

type stubInteractionStoreValidator struct {
	summary any
	err     error
	calls   int
}

func (s *stubInteractionStoreValidator) Validate() (any, error) {
	s.calls++
	return s.summary, s.err
}

type stubInteractionStoreCrashTempCleaner struct {
	summary any
	err     error
	calls   int
}

func (s *stubInteractionStoreCrashTempCleaner) Cleanup() (any, error) {
	s.calls++
	return s.summary, s.err
}

var errStubInteractionStoreInvalid = errors.New("interaction store invalid")

type stubItemTemplateStoreBacker struct {
	summary any
	err     error
	calls   int
	dstDir  string
}

func (s *stubItemTemplateStoreBacker) Backup(dstDir string) (any, error) {
	s.calls++
	s.dstDir = dstDir
	return s.summary, s.err
}

type stubItemTemplateStoreBackupValidator struct {
	summary any
	err     error
	calls   int
	srcDir  string
}

func (s *stubItemTemplateStoreBackupValidator) ValidateBackup(srcDir string) (any, error) {
	s.calls++
	s.srcDir = srcDir
	return s.summary, s.err
}

type stubItemTemplateStoreRestorer struct {
	summary any
	err     error
	calls   int
	srcDir  string
}

func (s *stubItemTemplateStoreRestorer) Restore(srcDir string) (any, error) {
	s.calls++
	s.srcDir = srcDir
	return s.summary, s.err
}

func TestLocalNoticeEndpointQueuesTrimmedLoopbackNotice(t *testing.T) {
	broadcaster := &stubNoticeBroadcaster{delivered: 2}
	mux := NewPprofMuxWithLocalNotice("gamed", broadcaster.BroadcastNotice)

	req := httptest.NewRequest(http.MethodPost, "/local/notice", strings.NewReader("  server maintenance  "))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if broadcaster.calls != 1 || broadcaster.lastMessage != "server maintenance" {
		t.Fatalf("expected broadcaster once with trimmed message, calls=%d message=%q", broadcaster.calls, broadcaster.lastMessage)
	}
	if got := rec.Body.String(); got != "queued 2\n" {
		t.Fatalf("unexpected notice response body %q", got)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Fatalf("expected text/plain content type, got %q", got)
	}
}

func TestLocalNoticeEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	broadcaster := &stubNoticeBroadcaster{delivered: 1}
	mux := NewPprofMuxWithLocalNotice("gamed", broadcaster.BroadcastNotice)

	req := httptest.NewRequest(http.MethodPost, "/local/notice", strings.NewReader("server maintenance"))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if broadcaster.calls != 0 {
		t.Fatalf("expected broadcaster not to be called, got %d", broadcaster.calls)
	}
}

func TestLocalNoticeEndpointRejectsWrongMethod(t *testing.T) {
	broadcaster := &stubNoticeBroadcaster{delivered: 1}
	mux := NewPprofMuxWithLocalNotice("gamed", broadcaster.BroadcastNotice)

	req := httptest.NewRequest(http.MethodGet, "/local/notice", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if broadcaster.calls != 0 {
		t.Fatalf("expected broadcaster not to be called, got %d", broadcaster.calls)
	}
}

func TestLocalNoticeEndpointRejectsEmptyBody(t *testing.T) {
	broadcaster := &stubNoticeBroadcaster{delivered: 1}
	mux := NewPprofMuxWithLocalNotice("gamed", broadcaster.BroadcastNotice)

	req := httptest.NewRequest(http.MethodPost, "/local/notice", strings.NewReader(" \n	 "))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if broadcaster.calls != 0 {
		t.Fatalf("expected broadcaster not to be called, got %d", broadcaster.calls)
	}
}

func TestLocalNoticeEndpointRejectsOversizedBody(t *testing.T) {
	broadcaster := &stubNoticeBroadcaster{delivered: 1}
	mux := NewPprofMuxWithLocalNotice("gamed", broadcaster.BroadcastNotice)

	req := httptest.NewRequest(http.MethodPost, "/local/notice", strings.NewReader(strings.Repeat("x", maxLocalNoticeBodyBytes+1)))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if broadcaster.calls != 0 {
		t.Fatalf("expected broadcaster not to be called, got %d", broadcaster.calls)
	}
}

func TestLocalNoticeEndpointRejectsInvalidUTF8Body(t *testing.T) {
	broadcaster := &stubNoticeBroadcaster{delivered: 1}
	mux := NewPprofMuxWithLocalNotice("gamed", broadcaster.BroadcastNotice)

	req := httptest.NewRequest(http.MethodPost, "/local/notice", bytes.NewReader([]byte{0xff, 'n', 'o', 't', 'i', 'c', 'e'}))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if broadcaster.calls != 0 {
		t.Fatalf("expected broadcaster not to be called, got %d", broadcaster.calls)
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

func TestLocalTransferEndpointReturnsStructuredJSONForLoopbackPost(t *testing.T) {
	transferer := &stubCharacterTransferer{found: true, result: map[string]any{
		"applied":                       true,
		"character":                     map[string]any{"name": "PeerTwo", "map_index": uint32(1), "x": int32(1300), "y": int32(2300)},
		"target":                        map[string]any{"name": "PeerTwo", "map_index": uint32(42), "x": int32(1700), "y": int32(2800)},
		"removed_visible_peers":         []map[string]any{{"name": "PeerOne"}},
		"added_visible_peers":           []map[string]any{{"name": "PeerThree"}},
		"removed_visible_static_actors": []map[string]any{{"entity_id": uint64(1), "name": "Blacksmith"}},
		"added_visible_static_actors":   []map[string]any{{"entity_id": uint64(2), "name": "VillageGuard"}},
		"current_visible_spawn_groups":  []map[string]any{{"entity_id": uint64(3), "name": "SourcePracticeMob", "spawn_group_ref": "practice.source_mob"}},
		"target_visible_spawn_groups":   []map[string]any{{"entity_id": uint64(4), "name": "DestinationPracticeMob", "spawn_group_ref": "practice.destination_mob"}},
		"removed_visible_spawn_groups":  []map[string]any{{"entity_id": uint64(3), "name": "SourcePracticeMob", "spawn_group_ref": "practice.source_mob"}},
		"added_visible_spawn_groups":    []map[string]any{{"entity_id": uint64(4), "name": "DestinationPracticeMob", "spawn_group_ref": "practice.destination_mob"}},
		"map_occupancy_changes":         []map[string]any{{"map_index": uint32(1), "before_count": 2, "after_count": 1}, {"map_index": uint32(42), "before_count": 1, "after_count": 2}},
		"before_map_occupancy":          []map[string]any{{"map_index": uint32(1), "character_count": 2, "characters": []map[string]any{{"name": "PeerOne"}, {"name": "PeerTwo"}}, "static_actor_count": 1, "static_actors": []map[string]any{{"entity_id": uint64(1), "name": "Blacksmith"}}}, {"map_index": uint32(42), "character_count": 1, "characters": []map[string]any{{"name": "PeerThree"}}, "static_actor_count": 1, "static_actors": []map[string]any{{"entity_id": uint64(2), "name": "VillageGuard"}}}},
		"after_map_occupancy":           []map[string]any{{"map_index": uint32(1), "character_count": 1, "characters": []map[string]any{{"name": "PeerOne"}}, "static_actor_count": 1, "static_actors": []map[string]any{{"entity_id": uint64(1), "name": "Blacksmith"}}}, {"map_index": uint32(42), "character_count": 2, "characters": []map[string]any{{"name": "PeerThree"}, {"name": "PeerTwo"}}, "static_actor_count": 1, "static_actors": []map[string]any{{"entity_id": uint64(2), "name": "VillageGuard"}}}},
	}}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, transferer.TransferCharacter, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/local/transfer", strings.NewReader(`{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if transferer.calls != 1 || transferer.lastName != "PeerTwo" || transferer.lastMapIndex != 42 || transferer.lastX != 1700 || transferer.lastY != 2800 {
		t.Fatalf("unexpected transferer call state: %+v", transferer)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"applied":true`) || !strings.Contains(string(body), `"map_occupancy_changes"`) || !strings.Contains(string(body), `"before_map_occupancy"`) || !strings.Contains(string(body), `"after_map_occupancy"`) || !strings.Contains(string(body), `"removed_visible_static_actors"`) || !strings.Contains(string(body), `"added_visible_static_actors"`) || !strings.Contains(string(body), `"current_visible_spawn_groups"`) || !strings.Contains(string(body), `"target_visible_spawn_groups"`) || !strings.Contains(string(body), `"removed_visible_spawn_groups"`) || !strings.Contains(string(body), `"added_visible_spawn_groups"`) || !strings.Contains(string(body), `"spawn_group_ref":"practice.destination_mob"`) || !strings.Contains(string(body), `"static_actor_count":1`) || !strings.Contains(string(body), `"name":"PeerThree"`) || !strings.Contains(string(body), `"name":"VillageGuard"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalTransferEndpointRejectsInvalidBody(t *testing.T) {
	transferer := &stubCharacterTransferer{found: true}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, transferer.TransferCharacter, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/local/transfer", strings.NewReader(`{"name":"PeerTwo","map_index":0,"x":1700,"y":2800}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if transferer.calls != 0 {
		t.Fatalf("expected transferer not to be called, got %d calls", transferer.calls)
	}
}

func TestLocalTransferEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	transferer := &stubCharacterTransferer{found: true}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, transferer.TransferCharacter, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/local/transfer", strings.NewReader(`{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}`))
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if transferer.calls != 0 {
		t.Fatalf("expected transferer not to be called, got %d calls", transferer.calls)
	}
}

func TestLocalTransferEndpointReturnsNotFoundForUnknownTarget(t *testing.T) {
	transferer := &stubCharacterTransferer{found: false}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, transferer.TransferCharacter, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/local/transfer", strings.NewReader(`{"name":"MissingPeer","map_index":42,"x":1700,"y":2800}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if transferer.calls != 1 {
		t.Fatalf("expected transferer to be called once, got %d calls", transferer.calls)
	}
}

func TestLocalTransferEndpointRejectsWrongMethod(t *testing.T) {
	transferer := &stubCharacterTransferer{found: true}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, transferer.TransferCharacter, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/local/transfer", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if transferer.calls != 0 {
		t.Fatalf("expected transferer not to be called, got %d calls", transferer.calls)
	}
}

func TestLocalRelocatePreviewEndpointReturnsJSONSnapshotForLoopbackPost(t *testing.T) {
	previewer := &stubRelocationPreviewer{found: true, preview: map[string]any{
		"character":                     map[string]any{"name": "PeerTwo", "map_index": uint32(1), "x": int32(1300), "y": int32(2300)},
		"target":                        map[string]any{"name": "PeerTwo", "map_index": uint32(42), "x": int32(1700), "y": int32(2800)},
		"removed_visible_peers":         []map[string]any{{"name": "PeerOne"}},
		"added_visible_peers":           []map[string]any{{"name": "PeerThree"}},
		"removed_visible_static_actors": []map[string]any{{"entity_id": uint64(1), "name": "Blacksmith"}},
		"added_visible_static_actors":   []map[string]any{{"entity_id": uint64(2), "name": "VillageGuard"}},
		"current_visible_spawn_groups":  []map[string]any{{"entity_id": uint64(3), "name": "SourcePracticeMob", "spawn_group_ref": "practice.source_mob"}},
		"target_visible_spawn_groups":   []map[string]any{{"entity_id": uint64(4), "name": "DestinationPracticeMob", "spawn_group_ref": "practice.destination_mob"}},
		"removed_visible_spawn_groups":  []map[string]any{{"entity_id": uint64(3), "name": "SourcePracticeMob", "spawn_group_ref": "practice.source_mob"}},
		"added_visible_spawn_groups":    []map[string]any{{"entity_id": uint64(4), "name": "DestinationPracticeMob", "spawn_group_ref": "practice.destination_mob"}},
		"map_occupancy_changes":         []map[string]any{{"map_index": uint32(1), "before_count": 2, "after_count": 1}, {"map_index": uint32(42), "before_count": 1, "after_count": 2}},
		"before_map_occupancy":          []map[string]any{{"map_index": uint32(1), "character_count": 2, "characters": []map[string]any{{"name": "PeerOne"}, {"name": "PeerTwo"}}, "static_actor_count": 1, "static_actors": []map[string]any{{"entity_id": uint64(1), "name": "Blacksmith"}}}, {"map_index": uint32(42), "character_count": 1, "characters": []map[string]any{{"name": "PeerThree"}}, "static_actor_count": 1, "static_actors": []map[string]any{{"entity_id": uint64(2), "name": "VillageGuard"}}}},
		"after_map_occupancy":           []map[string]any{{"map_index": uint32(1), "character_count": 1, "characters": []map[string]any{{"name": "PeerOne"}}, "static_actor_count": 1, "static_actors": []map[string]any{{"entity_id": uint64(1), "name": "Blacksmith"}}}, {"map_index": uint32(42), "character_count": 2, "characters": []map[string]any{{"name": "PeerThree"}, {"name": "PeerTwo"}}, "static_actor_count": 1, "static_actors": []map[string]any{{"entity_id": uint64(2), "name": "VillageGuard"}}}},
	}}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, previewer.PreviewRelocation, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/local/relocate-preview", strings.NewReader(`{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 || previewer.lastName != "PeerTwo" || previewer.lastMapIndex != 42 || previewer.lastX != 1700 || previewer.lastY != 2800 {
		t.Fatalf("unexpected previewer call state: %+v", previewer)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"removed_visible_peers"`) || !strings.Contains(string(body), `"map_occupancy_changes"`) || !strings.Contains(string(body), `"before_map_occupancy"`) || !strings.Contains(string(body), `"after_map_occupancy"`) || !strings.Contains(string(body), `"removed_visible_static_actors"`) || !strings.Contains(string(body), `"added_visible_static_actors"`) || !strings.Contains(string(body), `"current_visible_spawn_groups"`) || !strings.Contains(string(body), `"target_visible_spawn_groups"`) || !strings.Contains(string(body), `"removed_visible_spawn_groups"`) || !strings.Contains(string(body), `"added_visible_spawn_groups"`) || !strings.Contains(string(body), `"spawn_group_ref":"practice.destination_mob"`) || !strings.Contains(string(body), `"static_actor_count":1`) || !strings.Contains(string(body), `"name":"PeerThree"`) || !strings.Contains(string(body), `"name":"VillageGuard"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalRelocatePreviewEndpointRejectsInvalidBody(t *testing.T) {
	previewer := &stubRelocationPreviewer{found: true}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, previewer.PreviewRelocation, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/local/relocate-preview", strings.NewReader(`{"name":"PeerTwo","map_index":0,"x":1700,"y":2800}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected previewer not to be called, got %d calls", previewer.calls)
	}
}

func TestLocalRelocatePreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubRelocationPreviewer{found: true}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, previewer.PreviewRelocation, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/local/relocate-preview", strings.NewReader(`{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}`))
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected previewer not to be called, got %d calls", previewer.calls)
	}
}

func TestLocalRelocatePreviewEndpointReturnsNotFoundForUnknownTarget(t *testing.T) {
	previewer := &stubRelocationPreviewer{found: false}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, previewer.PreviewRelocation, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/local/relocate-preview", strings.NewReader(`{"name":"MissingPeer","map_index":42,"x":1700,"y":2800}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected previewer to be called once, got %d calls", previewer.calls)
	}
}

func TestLocalRelocatePreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubRelocationPreviewer{found: true}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, previewer.PreviewRelocation, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/local/relocate-preview", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected previewer not to be called, got %d calls", previewer.calls)
	}
}

func TestLocalPlayersEndpointReturnsJSONSnapshotForLoopbackGet(t *testing.T) {
	snapshotter := &stubConnectedCharactersSnapshotter{characters: []map[string]any{{"name": "Alpha", "map_index": 42, "x": int32(1700), "y": int32(2800)}, {"name": "Zulu", "map_index": uint32(1), "x": int32(1100), "y": int32(2100)}}}
	mux := NewPprofMuxWithLocalRuntimeSnapshot("gamed", nil, nil, snapshotter.ConnectedCharacters)

	req := httptest.NewRequest(http.MethodGet, "/local/players", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 {
		t.Fatalf("expected snapshotter to be called once, got %d calls", snapshotter.calls)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"name":"Alpha"`) || !strings.Contains(string(body), `"name":"Zulu"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalPlayersEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubConnectedCharactersSnapshotter{}
	mux := NewPprofMuxWithLocalRuntimeSnapshot("gamed", nil, nil, snapshotter.ConnectedCharacters)

	req := httptest.NewRequest(http.MethodGet, "/local/players", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalPlayersEndpointRejectsWrongMethod(t *testing.T) {
	snapshotter := &stubConnectedCharactersSnapshotter{}
	mux := NewPprofMuxWithLocalRuntimeSnapshot("gamed", nil, nil, snapshotter.ConnectedCharacters)

	req := httptest.NewRequest(http.MethodPost, "/local/players", strings.NewReader("ignored"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalVisibilityEndpointReturnsJSONSnapshotForLoopbackGet(t *testing.T) {
	snapshotter := &stubCharacterVisibilitySnapshotter{snapshots: []map[string]any{{"name": "Alpha", "map_index": 42, "visible_peers": []map[string]any{{"name": "PeerTwo", "map_index": 42}}}, {"name": "Zulu", "map_index": uint32(1), "visible_peers": []map[string]any{}}}}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, nil, nil, snapshotter.CharacterVisibility, nil)

	req := httptest.NewRequest(http.MethodGet, "/local/visibility", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 {
		t.Fatalf("expected visibility snapshotter to be called once, got %d calls", snapshotter.calls)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"name":"Alpha"`) || !strings.Contains(string(body), `"visible_peers":[`) || !strings.Contains(string(body), `"name":"PeerTwo"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalVisibilityEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubCharacterVisibilitySnapshotter{}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, nil, nil, snapshotter.CharacterVisibility, nil)

	req := httptest.NewRequest(http.MethodGet, "/local/visibility", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected visibility snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalVisibilityEndpointRejectsWrongMethod(t *testing.T) {
	snapshotter := &stubCharacterVisibilitySnapshotter{}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, nil, nil, snapshotter.CharacterVisibility, nil)

	req := httptest.NewRequest(http.MethodPost, "/local/visibility", strings.NewReader("ignored"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected visibility snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalMapOccupancyEndpointReturnsJSONSnapshotForLoopbackGet(t *testing.T) {
	lookup := &stubMapOccupancyLookup{snapshots: map[uint32]any{
		42: map[string]any{"map_index": uint32(42), "character_count": 1, "characters": []map[string]any{{"name": "Alpha"}}, "static_actor_count": 1, "static_actors": []map[string]any{{"entity_id": uint64(77), "name": "TrainingDummy"}}},
	}}
	mux := RegisterLocalMapOccupancyEndpoint(NewPprofMux("gamed"), lookup.MapOccupancy)

	req := httptest.NewRequest(http.MethodGet, "/local/maps/42", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if lookup.calls != 1 || lookup.lastMapIndex != 42 {
		t.Fatalf("expected map occupancy lookup for map 42 once, got calls=%d map_index=%d", lookup.calls, lookup.lastMapIndex)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"map_index":42`) || !strings.Contains(body, `"character_count":1`) || !strings.Contains(body, `"name":"Alpha"`) || !strings.Contains(body, `"static_actors":[`) {
		t.Fatalf("unexpected JSON response body %q", body)
	}
}

func TestLocalMapOccupancyEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	lookup := &stubMapOccupancyLookup{}
	mux := RegisterLocalMapOccupancyEndpoint(NewPprofMux("gamed"), lookup.MapOccupancy)

	req := httptest.NewRequest(http.MethodGet, "/local/maps/42", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if lookup.calls != 0 {
		t.Fatalf("expected map occupancy lookup not to be called, got %d calls", lookup.calls)
	}
}

func TestLocalMapOccupancyEndpointRejectsInvalidMapIndex(t *testing.T) {
	lookup := &stubMapOccupancyLookup{}
	mux := RegisterLocalMapOccupancyEndpoint(NewPprofMux("gamed"), lookup.MapOccupancy)

	req := httptest.NewRequest(http.MethodGet, "/local/maps/not-a-map", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if lookup.calls != 0 {
		t.Fatalf("expected map occupancy lookup not to be called, got %d calls", lookup.calls)
	}
}

func TestLocalMapOccupancyEndpointReturnsNotFoundForMissingMap(t *testing.T) {
	lookup := &stubMapOccupancyLookup{snapshots: map[uint32]any{}}
	mux := RegisterLocalMapOccupancyEndpoint(NewPprofMux("gamed"), lookup.MapOccupancy)

	req := httptest.NewRequest(http.MethodGet, "/local/maps/42", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if lookup.calls != 1 || lookup.lastMapIndex != 42 {
		t.Fatalf("expected map occupancy lookup for map 42 once, got calls=%d map_index=%d", lookup.calls, lookup.lastMapIndex)
	}
}

func TestLocalMapOccupancyEndpointRejectsWrongMethod(t *testing.T) {
	lookup := &stubMapOccupancyLookup{}
	mux := RegisterLocalMapOccupancyEndpoint(NewPprofMux("gamed"), lookup.MapOccupancy)

	req := httptest.NewRequest(http.MethodPost, "/local/maps/42", strings.NewReader("ignored"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if lookup.calls != 0 {
		t.Fatalf("expected map occupancy lookup not to be called, got %d calls", lookup.calls)
	}
}

func TestLocalRuntimeConfigEndpointReturnsJSONSnapshotForLoopbackGet(t *testing.T) {
	snapshotter := &stubRuntimeConfigSnapshotter{snapshot: map[string]any{"local_channel_id": 1, "visibility_mode": "radius", "visibility_radius": int32(400), "visibility_sector_size": int32(200), "persistence": map[string]any{"account_store_dir": "/state/accounts", "login_ticket_store_dir": "/state/tickets"}}}
	mux := RegisterLocalRuntimeConfigEndpoint(NewPprofMux("gamed"), snapshotter.RuntimeConfig)

	req := httptest.NewRequest(http.MethodGet, "/local/runtime-config", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 {
		t.Fatalf("expected runtime config snapshotter to be called once, got %d calls", snapshotter.calls)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"local_channel_id":1`) || !strings.Contains(string(body), `"visibility_mode":"radius"`) || !strings.Contains(string(body), `"visibility_radius":400`) || !strings.Contains(string(body), `"visibility_sector_size":200`) || !strings.Contains(string(body), `"account_store_dir":"/state/accounts"`) || !strings.Contains(string(body), `"login_ticket_store_dir":"/state/tickets"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalRuntimeConfigEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubRuntimeConfigSnapshotter{}
	mux := RegisterLocalRuntimeConfigEndpoint(NewPprofMux("gamed"), snapshotter.RuntimeConfig)

	req := httptest.NewRequest(http.MethodGet, "/local/runtime-config", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected runtime config snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalRuntimeConfigEndpointRejectsWrongMethod(t *testing.T) {
	snapshotter := &stubRuntimeConfigSnapshotter{}
	mux := RegisterLocalRuntimeConfigEndpoint(NewPprofMux("gamed"), snapshotter.RuntimeConfig)

	req := httptest.NewRequest(http.MethodPost, "/local/runtime-config", strings.NewReader("ignored"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected runtime config snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalPersistenceStatusEndpointReturnsJSONSnapshotForLoopbackGet(t *testing.T) {
	snapshotter := &stubPersistenceStatusSnapshotter{snapshot: map[string]any{
		"ok": true,
		"account_store": map[string]any{
			"path":    "/state/accounts",
			"valid":   true,
			"summary": map[string]any{"account_count": 1, "character_count": 2, "empty_character_slot_count": 1, "logins": []string{"mkmk"}},
			"backup_manifest": map[string]any{
				"present":             true,
				"path":                "/state/accounts/account-backup-manifest.json",
				"format":              "go-metin2-account-backup-v1",
				"file_count":          1,
				"snapshot_size_bytes": 128,
				"manifest_size_bytes": 256,
				"manifest_sha256":     "abc123",
			},
		},
		"login_ticket_store": map[string]any{
			"path":    "/state/tickets",
			"valid":   true,
			"summary": map[string]any{"ticket_count": 1, "logins": []string{"mkmk"}, "login_keys": []uint32{0x01020304}},
		},
		"item_template_store": map[string]any{
			"path":    "/state/item-templates.json",
			"valid":   true,
			"summary": map[string]any{"template_count": 1, "vnums": []uint32{27001}},
			"backup_manifest": map[string]any{
				"present":             true,
				"path":                "/state/item-template-backup-manifest.json",
				"format":              "go-metin2-item-template-backup-v1",
				"file_count":          1,
				"snapshot_size_bytes": 64,
				"manifest_size_bytes": 192,
				"manifest_sha256":     "def456",
			},
		},
		"static_actor_store": map[string]any{
			"path":    "/state/static-actors.json",
			"valid":   true,
			"summary": map[string]any{"actor_count": 1, "actor_ids": []uint64{7}, "actor_names": []string{"TrainingDummy"}},
		},
		"interaction_store": map[string]any{
			"path":    "/state/interaction-definitions.json",
			"valid":   true,
			"summary": map[string]any{"definition_count": 1, "definition_keys": []string{"info:lore:alchemist"}},
		},
	}}
	mux := RegisterLocalPersistenceStatusEndpoint(NewPprofMux("gamed"), snapshotter.PersistenceStatus)

	req := httptest.NewRequest(http.MethodGet, "/local/persistence/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 {
		t.Fatalf("expected persistence status snapshotter to be called once, got %d calls", snapshotter.calls)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body := rec.Body.String()
	for _, want := range []string{`"ok":true`, `"account_store"`, `"path":"/state/accounts"`, `"valid":true`, `"account_count":1`, `"character_count":2`, `"empty_character_slot_count":1`, `"backup_manifest":{"file_count":1,"format":"go-metin2-account-backup-v1","manifest_sha256":"abc123","manifest_size_bytes":256,"path":"/state/accounts/account-backup-manifest.json","present":true,"snapshot_size_bytes":128}`, `"login_ticket_store"`, `"ticket_count":1`, `"login_keys":[16909060]`, `"item_template_store"`, `"template_count":1`, `"vnums":[27001]`, `"backup_manifest":{"file_count":1,"format":"go-metin2-item-template-backup-v1","manifest_sha256":"def456","manifest_size_bytes":192,"path":"/state/item-template-backup-manifest.json","present":true,"snapshot_size_bytes":64}`, `"static_actor_store"`, `"actor_count":1`, `"actor_ids":[7]`, `"interaction_store"`, `"definition_count":1`, `"definition_keys":["info:lore:alchemist"]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %s, got %s", want, body)
		}
	}
}

func TestLocalPersistenceStatusEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubPersistenceStatusSnapshotter{}
	mux := RegisterLocalPersistenceStatusEndpoint(NewPprofMux("gamed"), snapshotter.PersistenceStatus)

	req := httptest.NewRequest(http.MethodGet, "/local/persistence/status", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected persistence status snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalPersistenceStatusEndpointRejectsWrongMethod(t *testing.T) {
	snapshotter := &stubPersistenceStatusSnapshotter{}
	mux := RegisterLocalPersistenceStatusEndpoint(NewPprofMux("gamed"), snapshotter.PersistenceStatus)

	req := httptest.NewRequest(http.MethodPost, "/local/persistence/status", strings.NewReader("ignored"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected persistence status snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalStaticActorRespawnsEndpointReturnsJSONSnapshotsForLoopbackGet(t *testing.T) {
	snapshotter := &stubListSnapshotter{snapshots: []map[string]any{{"entity_id": uint64(33), "ready_at": "2026-07-25T12:00:00Z", "remaining_ms": int64(1200), "actor": map[string]any{"entity_id": uint64(33), "name": "RespawnMob", "dead": true}}}}
	mux := RegisterLocalStaticActorRespawnsEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/static-actor-respawns", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 {
		t.Fatalf("expected static-actor respawns snapshotter call, got %d calls", snapshotter.calls)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"entity_id":33`) || !strings.Contains(string(body), `"remaining_ms":1200`) || !strings.Contains(string(body), `"actor"`) || !strings.Contains(string(body), `"dead":true`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalStaticActorRespawnsEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubListSnapshotter{}
	mux := RegisterLocalStaticActorRespawnsEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/static-actor-respawns", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected static-actor respawns snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalStaticActorRespawnsEndpointRejectsWrongMethod(t *testing.T) {
	snapshotter := &stubListSnapshotter{}
	mux := RegisterLocalStaticActorRespawnsEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actor-respawns", strings.NewReader("ignored"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected static-actor respawns snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalStaticActorRespawnEndpointReturnsExactSnapshotForLoopbackGet(t *testing.T) {
	snapshot := map[string]any{"entity_id": uint64(33), "ready_at": "2026-07-25T12:00:00Z", "remaining_ms": int64(1200), "actor": map[string]any{"entity_id": uint64(33), "name": "RespawnMob", "dead": true}}
	mux := RegisterLocalStaticActorRespawnEndpoint(NewPprofMux("gamed"), func(entityID uint64) (any, bool) {
		if entityID != 33 {
			return nil, false
		}
		return snapshot, true
	})

	req := httptest.NewRequest(http.MethodGet, "/local/static-actor-respawns/33", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"entity_id":33`) || !strings.Contains(string(body), `"remaining_ms":1200`) || !strings.Contains(string(body), `"actor"`) || !strings.Contains(string(body), `"dead":true`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalStaticActorRespawnEndpointRejectsInvalidEntityID(t *testing.T) {
	mux := RegisterLocalStaticActorRespawnEndpoint(NewPprofMux("gamed"), func(uint64) (any, bool) {
		t.Fatal("static-actor respawn lookup should not be called for invalid entity IDs")
		return nil, false
	})

	req := httptest.NewRequest(http.MethodGet, "/local/static-actor-respawns/not-an-id", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestLocalStaticActorRespawnEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	mux := RegisterLocalStaticActorRespawnEndpoint(NewPprofMux("gamed"), func(uint64) (any, bool) {
		t.Fatal("static-actor respawn lookup should not be called for non-loopback callers")
		return nil, false
	})

	req := httptest.NewRequest(http.MethodGet, "/local/static-actor-respawns/33", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestLocalStaticActorRespawnEndpointRejectsWrongMethod(t *testing.T) {
	mux := RegisterLocalStaticActorRespawnEndpoint(NewPprofMux("gamed"), func(uint64) (any, bool) {
		t.Fatal("static-actor respawn lookup should not be called for wrong methods")
		return nil, false
	})

	req := httptest.NewRequest(http.MethodPost, "/local/static-actor-respawns/33", strings.NewReader("ignored"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestLocalStaticActorRespawnEndpointReturnsNotFoundForMissingEntityID(t *testing.T) {
	mux := RegisterLocalStaticActorRespawnEndpoint(NewPprofMux("gamed"), func(uint64) (any, bool) {
		return nil, false
	})

	req := httptest.NewRequest(http.MethodGet, "/local/static-actor-respawns/33", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestLocalSpawnGroupsEndpointReturnsJSONSnapshotsForLoopbackGet(t *testing.T) {
	snapshotter := &stubListSnapshotter{snapshots: []map[string]any{{"entity_id": uint64(44), "name": "Practice Wolf", "spawn_group_ref": "practice.wolf_1", "combat_profile": "practice_mob"}}}
	mux := RegisterLocalSpawnGroupsEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/spawn-groups", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 {
		t.Fatalf("expected spawn-groups snapshotter call, got %d calls", snapshotter.calls)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"entity_id":44`) || !strings.Contains(string(body), `"spawn_group_ref":"practice.wolf_1"`) || !strings.Contains(string(body), `"combat_profile":"practice_mob"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalSpawnGroupsEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubListSnapshotter{}
	mux := RegisterLocalSpawnGroupsEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/spawn-groups", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected spawn-groups snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalSpawnGroupsEndpointRejectsWrongMethod(t *testing.T) {
	snapshotter := &stubListSnapshotter{}
	mux := RegisterLocalSpawnGroupsEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodPost, "/local/spawn-groups", strings.NewReader("ignored"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected spawn-groups snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalSpawnGroupEndpointReturnsExactSnapshotForLoopbackGet(t *testing.T) {
	snapshot := map[string]any{"entity_id": uint64(44), "name": "Practice Wolf", "spawn_group_ref": "practice.wolf_1", "combat_profile": "practice_mob"}
	mux := RegisterLocalSpawnGroupEndpoint(NewPprofMux("gamed"), func(entityID uint64) (any, bool) {
		if entityID != 44 {
			return nil, false
		}
		return snapshot, true
	})

	req := httptest.NewRequest(http.MethodGet, "/local/spawn-groups/44", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"entity_id":44`) || !strings.Contains(string(body), `"spawn_group_ref":"practice.wolf_1"`) || !strings.Contains(string(body), `"combat_profile":"practice_mob"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalSpawnGroupEndpointRejectsInvalidEntityID(t *testing.T) {
	mux := RegisterLocalSpawnGroupEndpoint(NewPprofMux("gamed"), func(uint64) (any, bool) {
		t.Fatal("spawn-group lookup should not be called for invalid entity IDs")
		return nil, false
	})

	req := httptest.NewRequest(http.MethodGet, "/local/spawn-groups/not-an-id", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestLocalSpawnGroupEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	mux := RegisterLocalSpawnGroupEndpoint(NewPprofMux("gamed"), func(uint64) (any, bool) {
		t.Fatal("spawn-group lookup should not be called for non-loopback callers")
		return nil, false
	})

	req := httptest.NewRequest(http.MethodGet, "/local/spawn-groups/44", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestLocalSpawnGroupEndpointRejectsWrongMethod(t *testing.T) {
	mux := RegisterLocalSpawnGroupEndpoint(NewPprofMux("gamed"), func(uint64) (any, bool) {
		t.Fatal("spawn-group lookup should not be called for wrong methods")
		return nil, false
	})

	req := httptest.NewRequest(http.MethodPost, "/local/spawn-groups/44", strings.NewReader("ignored"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestLocalSpawnGroupEndpointReturnsNotFoundForMissingEntityID(t *testing.T) {
	mux := RegisterLocalSpawnGroupEndpoint(NewPprofMux("gamed"), func(uint64) (any, bool) {
		return nil, false
	})

	req := httptest.NewRequest(http.MethodGet, "/local/spawn-groups/44", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestLocalStaticActorCombatProfileEndpointRegistersProfileForLoopbackPost(t *testing.T) {
	const profile = "ops_profile_wolf"
	worldruntime.UnregisterStaticActorCombatProfileForTest(profile)
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	mux := RegisterLocalStaticActorCombatProfileEndpoint(NewPprofMux("gamed"))
	body := `{"profile":"ops_profile_wolf","max_hp":24,"attack_value":9,"defense_value":4,"level":7,"rank":2,"respawn_delay_ms":1500,"retaliation_point_delta":-2,"death_reward":{"experience":30,"gold":11,"drop_vnums":[27002,27001]}}`
	req := httptest.NewRequest(http.MethodPost, "/local/static-actor-combat-profiles", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body %q", http.StatusOK, rec.Code, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	defaults, ok := worldruntime.BootstrapStaticActorCombatProfileDefaults(profile)
	if !ok {
		t.Fatalf("expected profile %q to be registered", profile)
	}
	if defaults.MaxHP != 24 || defaults.DamagePerNormalAttack != 5 || defaults.AttackValue != 9 || defaults.DefenseValue != 4 || defaults.Level != 7 || defaults.Rank != 2 || defaults.RespawnDelay.String() != "1.5s" || defaults.RetaliationPointDelta != -2 {
		t.Fatalf("unexpected registered defaults: %+v", defaults)
	}
	if defaults.DeathReward.Experience != 30 || defaults.DeathReward.Gold != 11 || len(defaults.DeathReward.DropVnums) != 2 || defaults.DeathReward.DropVnums[0] != 27001 || defaults.DeathReward.DropVnums[1] != 27002 {
		t.Fatalf("unexpected registered death reward: %+v", defaults.DeathReward)
	}
	bodyText := rec.Body.String()
	if !strings.Contains(bodyText, `"profile":"ops_profile_wolf"`) || !strings.Contains(bodyText, `"damage_per_normal_attack":5`) || !strings.Contains(bodyText, `"respawn_delay_ms":1500`) || !strings.Contains(bodyText, `"retaliation_point_delta":-2`) {
		t.Fatalf("unexpected JSON response body %q", bodyText)
	}
}

func TestLocalStaticActorCombatProfileEndpointReturnsProfilesForLoopbackGet(t *testing.T) {
	const profile = "ops_list_wolf"
	worldruntime.UnregisterStaticActorCombatProfileForTest(profile)
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })
	if !worldruntime.RegisterStaticActorCombatProfile(profile, worldruntime.StaticActorCombatProfileDefaults{
		MaxHP:        24,
		AttackValue:  9,
		DefenseValue: 4,
		Level:        7,
		Rank:         2,
		RespawnDelay: worldruntime.PracticeMobBootstrapRespawnDelay,
		DeathReward:  worldruntime.StaticActorDeathReward{Experience: 30, Gold: 11, DropVnums: []uint32{27002, 27001}},
	}) {
		t.Fatalf("expected %q profile registration to succeed before list endpoint check", profile)
	}

	mux := RegisterLocalStaticActorCombatProfileEndpoint(NewPprofMux("gamed"))
	req := httptest.NewRequest(http.MethodGet, "/local/static-actor-combat-profiles", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body %q", http.StatusOK, rec.Code, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	bodyText := rec.Body.String()
	if !strings.Contains(bodyText, `"profile":"practice_mob"`) || !strings.Contains(bodyText, `"profile":"training_dummy"`) || !strings.Contains(bodyText, `"profile":"ops_list_wolf"`) {
		t.Fatalf("expected built-in and registered profiles in JSON response body %q", bodyText)
	}
	if !strings.Contains(bodyText, `"damage_per_normal_attack":5`) || !strings.Contains(bodyText, `"respawn_delay_ms":2000`) || !strings.Contains(bodyText, `"drop_vnums":[27001,27002]`) {
		t.Fatalf("expected canonical profile defaults and normalized reward drops in JSON response body %q", bodyText)
	}
}

func TestLocalStaticActorCombatProfileEndpointRejectsInvalidProfile(t *testing.T) {
	mux := RegisterLocalStaticActorCombatProfileEndpoint(NewPprofMux("gamed"))
	req := httptest.NewRequest(http.MethodPost, "/local/static-actor-combat-profiles", strings.NewReader(`{"profile":"practice_mob","max_hp":24,"attack_value":9,"respawn_delay_ms":1500}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestLocalStaticActorCombatProfileEndpointRejectsPaddedProfileName(t *testing.T) {
	const profile = "ops_padded_profile"
	worldruntime.UnregisterStaticActorCombatProfileForTest(profile)
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	mux := RegisterLocalStaticActorCombatProfileEndpoint(NewPprofMux("gamed"))
	req := httptest.NewRequest(http.MethodPost, "/local/static-actor-combat-profiles", strings.NewReader(`{"profile":" ops_padded_profile ","max_hp":24,"attack_value":9,"defense_value":4,"respawn_delay_ms":1500}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for padded profile, got %d body %q", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	if _, ok := worldruntime.BootstrapStaticActorCombatProfileDefaults(profile); ok {
		t.Fatalf("expected padded profile request not to register %q", profile)
	}
}

func TestLocalStaticActorCombatProfileEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	mux := RegisterLocalStaticActorCombatProfileEndpoint(NewPprofMux("gamed"))
	req := httptest.NewRequest(http.MethodPost, "/local/static-actor-combat-profiles", strings.NewReader(`{"profile":"ops_remote_wolf","max_hp":24,"attack_value":9,"respawn_delay_ms":1500}`))
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestLocalStaticActorCombatProfileEndpointRejectsWrongMethod(t *testing.T) {
	mux := RegisterLocalStaticActorCombatProfileEndpoint(NewPprofMux("gamed"))
	req := httptest.NewRequest(http.MethodPut, "/local/static-actor-combat-profiles", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestLocalStaticActorCombatProfileEndpointListsProfilesForLoopbackGet(t *testing.T) {
	const profile = "ops_profile_snapshot_wolf"
	worldruntime.UnregisterStaticActorCombatProfileForTest(profile)
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })
	if !worldruntime.RegisterStaticActorCombatProfile(profile, worldruntime.StaticActorCombatProfileDefaults{MaxHP: 18, AttackValue: 6, DefenseValue: 2, RespawnDelay: 1200 * time.Millisecond}) {
		t.Fatalf("expected %q profile registration to succeed", profile)
	}
	mux := RegisterLocalStaticActorCombatProfileEndpoint(NewPprofMux("gamed"))
	req := httptest.NewRequest(http.MethodGet, "/local/static-actor-combat-profiles", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body %q", http.StatusOK, rec.Code, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"profile":"training_dummy"`) || !strings.Contains(body, `"profile":"practice_mob"`) || !strings.Contains(body, `"profile":"ops_profile_snapshot_wolf"`) {
		t.Fatalf("expected built-in and registered profiles in response body, got %q", body)
	}
	if !strings.Contains(body, `"damage_per_normal_attack":4`) || !strings.Contains(body, `"respawn_delay_ms":1200`) {
		t.Fatalf("expected canonical registered profile defaults in response body, got %q", body)
	}
}

func TestLocalStaticActorCombatProfileEndpointRejectsNonLoopbackList(t *testing.T) {
	mux := RegisterLocalStaticActorCombatProfileEndpoint(NewPprofMux("gamed"))
	req := httptest.NewRequest(http.MethodGet, "/local/static-actor-combat-profiles", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestLocalCombatTargetsEndpointReturnsJSONSnapshotsForLoopbackGet(t *testing.T) {
	snapshotter := &stubListSnapshotter{snapshots: []map[string]any{{"subject_entity_id": uint64(17), "subject": map[string]any{"name": "MkmkSura", "vid": uint32(99), "map_index": uint32(1)}, "target_vid": uint32(22), "snapshot_version": uint64(3), "hp_percent": uint8(80), "engaged_by_entity_id": uint64(17), "engaged_by": map[string]any{"name": "MkmkSura", "vid": uint32(99), "map_index": uint32(1)}, "retaliation_point_delta": int32(-1), "retaliation_server_origin": true}}}
	mux := RegisterLocalCombatTargetsEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/combat-targets", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 {
		t.Fatalf("expected combat-targets snapshotter call, got %d calls", snapshotter.calls)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"target_vid":22`) || !strings.Contains(string(body), `"hp_percent":80`) || !strings.Contains(string(body), `"subject"`) || !strings.Contains(string(body), `"name":"MkmkSura"`) || !strings.Contains(string(body), `"engaged_by_entity_id":17`) || !strings.Contains(string(body), `"retaliation_point_delta":-1`) || !strings.Contains(string(body), `"retaliation_server_origin":true`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalCombatTargetsEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubListSnapshotter{}
	mux := RegisterLocalCombatTargetsEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/combat-targets", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected combat-targets snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalCombatTargetsEndpointRejectsWrongMethod(t *testing.T) {
	snapshotter := &stubListSnapshotter{}
	mux := RegisterLocalCombatTargetsEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodPost, "/local/combat-targets", strings.NewReader("ignored"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected combat-targets snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalCombatTargetEndpointReturnsJSONSnapshotForLoopbackGet(t *testing.T) {
	snapshotter := &stubNamedSnapshotter{snapshots: map[string]any{"MkmkSura": map[string]any{"subject_entity_id": uint64(17), "subject": map[string]any{"name": "MkmkSura", "vid": uint32(99), "map_index": uint32(1)}, "target_vid": uint32(22), "snapshot_version": uint64(3), "hp_percent": uint8(80), "engaged_by_entity_id": uint64(17), "engaged_by": map[string]any{"name": "MkmkSura", "vid": uint32(99), "map_index": uint32(1)}, "retaliation_point_delta": int32(-1), "retaliation_server_origin": true}}}
	mux := RegisterLocalCombatTargetEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/combat-target/MkmkSura", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 || snapshotter.lastName != "MkmkSura" {
		t.Fatalf("expected combat-target snapshotter call for MkmkSura, got calls=%d name=%q", snapshotter.calls, snapshotter.lastName)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"target_vid":22`) || !strings.Contains(string(body), `"hp_percent":80`) || !strings.Contains(string(body), `"subject"`) || !strings.Contains(string(body), `"name":"MkmkSura"`) || !strings.Contains(string(body), `"engaged_by_entity_id":17`) || !strings.Contains(string(body), `"retaliation_point_delta":-1`) || !strings.Contains(string(body), `"retaliation_server_origin":true`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalCombatTargetEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubNamedSnapshotter{}
	mux := RegisterLocalCombatTargetEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/combat-target/MkmkSura", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected combat-target snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalCombatTargetEndpointRejectsWrongMethod(t *testing.T) {
	snapshotter := &stubNamedSnapshotter{}
	mux := RegisterLocalCombatTargetEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodPost, "/local/combat-target/MkmkSura", strings.NewReader("ignored"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected combat-target snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalCombatTargetEndpointReturnsNotFoundForMissingSnapshot(t *testing.T) {
	snapshotter := &stubNamedSnapshotter{}
	mux := RegisterLocalCombatTargetEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/combat-target/MkmkSura", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if snapshotter.calls != 1 || snapshotter.lastName != "MkmkSura" {
		t.Fatalf("expected combat-target snapshotter call for MkmkSura, got calls=%d name=%q", snapshotter.calls, snapshotter.lastName)
	}
}

func TestLocalInventoryEndpointReturnsJSONSnapshotForLoopbackGet(t *testing.T) {
	snapshotter := &stubNamedSnapshotter{snapshots: map[string]any{"MkmkSura": map[string]any{"name": "MkmkSura", "inventory": []map[string]any{{"id": uint64(11), "vnum": uint32(50501), "count": uint16(2), "slot": uint16(5)}}}}}
	mux := RegisterLocalInventoryEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/inventory/MkmkSura", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 || snapshotter.lastName != "MkmkSura" {
		t.Fatalf("expected inventory snapshotter call for MkmkSura, got calls=%d name=%q", snapshotter.calls, snapshotter.lastName)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"name":"MkmkSura"`) || !strings.Contains(string(body), `"slot":5`) || !strings.Contains(string(body), `"vnum":50501`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalInventoryEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubNamedSnapshotter{}
	mux := RegisterLocalInventoryEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/inventory/MkmkSura", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected inventory snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalInventoryEndpointRejectsUnknownCharacter(t *testing.T) {
	snapshotter := &stubNamedSnapshotter{snapshots: map[string]any{"Other": map[string]any{"name": "Other"}}}
	mux := RegisterLocalInventoryEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/inventory/MkmkSura", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if snapshotter.calls != 1 || snapshotter.lastName != "MkmkSura" {
		t.Fatalf("expected inventory snapshotter call for MkmkSura, got calls=%d name=%q", snapshotter.calls, snapshotter.lastName)
	}
}

func TestLocalInventoryEndpointRejectsWrongMethod(t *testing.T) {
	snapshotter := &stubNamedSnapshotter{}
	mux := RegisterLocalInventoryEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodPost, "/local/inventory/MkmkSura", strings.NewReader("ignored"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected inventory snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalInventoryEndpointDecodesEscapedCharacterName(t *testing.T) {
	snapshotter := &stubNamedSnapshotter{snapshots: map[string]any{"Mkmk Sura": map[string]any{"name": "Mkmk Sura", "inventory": []map[string]any{{"slot": uint16(5)}}}}}
	mux := RegisterLocalInventoryEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/inventory/Mkmk%20Sura", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 || snapshotter.lastName != "Mkmk Sura" {
		t.Fatalf("expected decoded name Mkmk Sura, got calls=%d name=%q", snapshotter.calls, snapshotter.lastName)
	}
}

func TestLocalInventoryEndpointRejectsNamesContainingSlashes(t *testing.T) {
	snapshotter := &stubNamedSnapshotter{}
	mux := RegisterLocalInventoryEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/inventory/Mkmk%2FSura", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected inventory snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalEquipmentEndpointReturnsJSONSnapshotForLoopbackGet(t *testing.T) {
	snapshotter := &stubNamedSnapshotter{snapshots: map[string]any{"MkmkSura": map[string]any{"name": "MkmkSura", "equipment": []map[string]any{{"id": uint64(21), "vnum": uint32(19), "count": uint16(1), "equip_slot": "weapon"}}}}}
	mux := RegisterLocalEquipmentEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/equipment/MkmkSura", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 || snapshotter.lastName != "MkmkSura" {
		t.Fatalf("expected equipment snapshotter call for MkmkSura, got calls=%d name=%q", snapshotter.calls, snapshotter.lastName)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"equip_slot":"weapon"`) || !strings.Contains(string(body), `"vnum":19`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalCurrencyEndpointReturnsJSONSnapshotForLoopbackGet(t *testing.T) {
	snapshotter := &stubNamedSnapshotter{snapshots: map[string]any{"MkmkSura": map[string]any{"name": "MkmkSura", "gold": uint64(123456)}}}
	mux := RegisterLocalCurrencyEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/currency/MkmkSura", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 || snapshotter.lastName != "MkmkSura" {
		t.Fatalf("expected currency snapshotter call for MkmkSura, got calls=%d name=%q", snapshotter.calls, snapshotter.lastName)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"gold":123456`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalMapsEndpointReturnsJSONSnapshotForLoopbackGet(t *testing.T) {
	snapshotter := &stubMapOccupancySnapshotter{snapshots: []map[string]any{
		{"map_index": uint32(1), "character_count": 1, "characters": []map[string]any{{"name": "Zulu"}}},
		{"map_index": uint32(42), "character_count": 2, "characters": []map[string]any{{"name": "Alpha"}, {"name": "PeerTwo"}}, "spawn_group_count": 1, "spawn_groups": []map[string]any{{"name": "PracticeMobAlpha", "spawn_group_ref": "practice.mob_alpha"}}},
	}}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, nil, nil, nil, snapshotter.MapOccupancy)

	req := httptest.NewRequest(http.MethodGet, "/local/maps", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 {
		t.Fatalf("expected map snapshotter to be called once, got %d calls", snapshotter.calls)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	response := string(body)
	if !strings.Contains(response, `"map_index":42`) || !strings.Contains(response, `"character_count":2`) || !strings.Contains(response, `"name":"PeerTwo"`) || !strings.Contains(response, `"spawn_group_count":1`) || !strings.Contains(response, `"spawn_group_ref":"practice.mob_alpha"`) {
		t.Fatalf("unexpected JSON response body %q", response)
	}
}

func TestLocalMapsEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubMapOccupancySnapshotter{}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, nil, nil, nil, snapshotter.MapOccupancy)

	req := httptest.NewRequest(http.MethodGet, "/local/maps", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected map snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalMapsEndpointRejectsWrongMethod(t *testing.T) {
	snapshotter := &stubMapOccupancySnapshotter{}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, nil, nil, nil, snapshotter.MapOccupancy)

	req := httptest.NewRequest(http.MethodPost, "/local/maps", strings.NewReader("ignored"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected map snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalGroundItemsEndpointReturnsJSONSnapshotForLoopbackGet(t *testing.T) {
	snapshotter := &stubListSnapshotter{snapshots: []map[string]any{{"vid": uint32(0x0700002c), "vnum": uint32(3001), "count": uint16(2), "owner_name": "GroundSnapshotOwner", "map_index": uint32(1), "x": int32(1200), "y": int32(2200)}, {"vid": uint32(0x0700002d), "gold_amount": uint32(250), "owner_name": "GroundSnapshotOwner", "map_index": uint32(1), "x": int32(1200), "y": int32(2200)}}}
	mux := RegisterLocalGroundItemsEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/ground-items", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 {
		t.Fatalf("expected ground item snapshotter to be called once, got %d calls", snapshotter.calls)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"vid":117440556`) || !strings.Contains(string(body), `"gold_amount":250`) || !strings.Contains(string(body), `"owner_name":"GroundSnapshotOwner"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalGroundItemsEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubListSnapshotter{}
	mux := RegisterLocalGroundItemsEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/ground-items", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected ground item snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalGroundItemsEndpointRejectsWrongMethod(t *testing.T) {
	snapshotter := &stubListSnapshotter{}
	mux := RegisterLocalGroundItemsEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodPost, "/local/ground-items", strings.NewReader("ignored"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected ground item snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalStaticActorsEndpointReturnsJSONSnapshotForLoopbackGet(t *testing.T) {
	snapshotter := &stubStaticActorSnapshotter{actors: []map[string]any{{"entity_id": uint64(2), "name": "Blacksmith", "map_index": uint32(42), "x": int32(1900), "y": int32(3000), "race_num": uint32(20301)}, {"entity_id": uint64(1), "name": "VillageGuard", "map_index": uint32(42), "x": int32(1700), "y": int32(2800), "race_num": uint32(20300)}}}
	mux := RegisterLocalStaticActorEndpoints(NewPprofMux("gamed"), snapshotter.StaticActors, nil)

	req := httptest.NewRequest(http.MethodGet, "/local/static-actors", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 {
		t.Fatalf("expected static actor snapshotter to be called once, got %d calls", snapshotter.calls)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"name":"Blacksmith"`) || !strings.Contains(string(body), `"race_num":20300`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalStaticActorEndpointReturnsJSONSnapshotForLoopbackGet(t *testing.T) {
	lookup := &stubStaticActorLookup{actors: map[uint64]any{7: map[string]any{"entity_id": uint64(7), "name": "TrainingDummy", "map_index": uint32(42), "x": int32(1700), "y": int32(2800), "race_num": uint32(20350), "dead": true}}}
	mux := RegisterLocalStaticActorEndpoint(NewPprofMux("gamed"), lookup.StaticActor)

	req := httptest.NewRequest(http.MethodGet, "/local/static-actors/7", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if lookup.calls != 1 || lookup.lastEntityID != 7 {
		t.Fatalf("expected static actor lookup for entity 7, got calls=%d entity_id=%d", lookup.calls, lookup.lastEntityID)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"entity_id":7`) || !strings.Contains(string(body), `"name":"TrainingDummy"`) || !strings.Contains(string(body), `"dead":true`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalStaticActorEndpointRejectsInvalidEntityID(t *testing.T) {
	lookup := &stubStaticActorLookup{}
	mux := RegisterLocalStaticActorEndpoint(NewPprofMux("gamed"), lookup.StaticActor)

	req := httptest.NewRequest(http.MethodGet, "/local/static-actors/not-a-number", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if lookup.calls != 0 {
		t.Fatalf("expected static actor lookup not to be called, got %d calls", lookup.calls)
	}
}

func TestLocalStaticActorEndpointReturnsNotFoundForMissingActor(t *testing.T) {
	lookup := &stubStaticActorLookup{}
	mux := RegisterLocalStaticActorEndpoint(NewPprofMux("gamed"), lookup.StaticActor)

	req := httptest.NewRequest(http.MethodGet, "/local/static-actors/7", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if lookup.calls != 1 || lookup.lastEntityID != 7 {
		t.Fatalf("expected static actor lookup for missing entity 7, got calls=%d entity_id=%d", lookup.calls, lookup.lastEntityID)
	}
}

func TestLocalStaticActorEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	lookup := &stubStaticActorLookup{actors: map[uint64]any{7: map[string]any{"entity_id": uint64(7)}}}
	mux := RegisterLocalStaticActorEndpoint(NewPprofMux("gamed"), lookup.StaticActor)

	req := httptest.NewRequest(http.MethodGet, "/local/static-actors/7", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if lookup.calls != 0 {
		t.Fatalf("expected static actor lookup not to be called, got %d calls", lookup.calls)
	}
}

func TestLocalStaticActorEndpointRejectsUnsupportedMethod(t *testing.T) {
	lookup := &stubStaticActorLookup{actors: map[uint64]any{7: map[string]any{"entity_id": uint64(7)}}}
	mux := RegisterLocalStaticActorEndpoint(NewPprofMux("gamed"), lookup.StaticActor)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actors/7", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if lookup.calls != 0 {
		t.Fatalf("expected static actor lookup not to be called, got %d calls", lookup.calls)
	}
}

func TestLocalStaticActorsEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubStaticActorSnapshotter{}
	mux := RegisterLocalStaticActorEndpoints(NewPprofMux("gamed"), snapshotter.StaticActors, nil)

	req := httptest.NewRequest(http.MethodGet, "/local/static-actors", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected static actor snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalVisibilityEndpointIncludesVisibleStaticActorsForLoopbackGet(t *testing.T) {
	visibility := &stubCharacterVisibilitySnapshotter{snapshots: []worldruntime.CharacterVisibilitySnapshot{{
		ConnectedCharacterSnapshot: worldruntime.ConnectedCharacterSnapshot{Name: "PeerOne", VID: 0x02040101, MapIndex: 42, X: 1700, Y: 2800, Empire: 2, GuildID: 10},
		VisiblePeers:               []worldruntime.ConnectedCharacterSnapshot{{Name: "PeerTwo", VID: 0x02040102, MapIndex: 42, X: 1900, Y: 2900, Empire: 2, GuildID: 10}},
		VisibleStaticActors:        []worldruntime.StaticActorSnapshot{{EntityID: 7, Name: "Blacksmith", MapIndex: 42, X: 1750, Y: 2850, RaceNum: 20300}},
		VisibleSpawnGroups:         []worldruntime.StaticActorSnapshot{{EntityID: 9, Name: "PracticeMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20350, CombatProfile: "practice_mob", SpawnGroupRef: "practice.mob"}},
	}}}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, nil, nil, visibility.CharacterVisibility, nil)

	req := httptest.NewRequest(http.MethodGet, "/local/visibility", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if visibility.calls != 1 {
		t.Fatalf("expected visibility snapshotter to be called once, got %d calls", visibility.calls)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"visible_static_actors"`) || !strings.Contains(string(body), `"name":"Blacksmith"`) || !strings.Contains(string(body), `"entity_id":7`) {
		t.Fatalf("unexpected visibility JSON response body %q", string(body))
	}
	if !strings.Contains(string(body), `"visible_spawn_groups"`) || !strings.Contains(string(body), `"name":"PracticeMob"`) || !strings.Contains(string(body), `"spawn_group_ref":"practice.mob"`) {
		t.Fatalf("expected visible spawn-group subset in visibility JSON response body %q", string(body))
	}
}

func TestLocalStaticActorsEndpointRegistersActorForLoopbackPost(t *testing.T) {
	registrar := &stubStaticActorRegistrar{registered: true, actor: map[string]any{"entity_id": uint64(1), "name": "VillageGuard", "map_index": uint32(42), "x": int32(1700), "y": int32(2800), "race_num": uint32(20300)}}
	mux := RegisterLocalStaticActorEndpoints(NewPprofMux("gamed"), nil, registrar.RegisterStaticActor)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actors", strings.NewReader(`{"name":"VillageGuard","map_index":42,"x":1700,"y":2800,"race_num":20300}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if registrar.calls != 1 || registrar.lastName != "VillageGuard" || registrar.lastMapIndex != 42 || registrar.lastX != 1700 || registrar.lastY != 2800 || registrar.lastRaceNum != 20300 || registrar.lastInteractionKind != "" || registrar.lastInteractionRef != "" {
		t.Fatalf("unexpected static actor registrar call state: %+v", registrar)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"entity_id":1`) || !strings.Contains(string(body), `"name":"VillageGuard"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalStaticActorsEndpointRegistersActorWithInteractionMetadataForLoopbackPost(t *testing.T) {
	registrar := &stubStaticActorRegistrar{registered: true, actor: map[string]any{"entity_id": uint64(1), "name": "VillageGuard", "map_index": uint32(42), "x": int32(1700), "y": int32(2800), "race_num": uint32(20300), "interaction_kind": "talk", "interaction_ref": "npc:village_guard"}}
	mux := RegisterLocalStaticActorEndpoints(NewPprofMux("gamed"), nil, registrar.RegisterStaticActor)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actors", strings.NewReader(`{"name":"VillageGuard","map_index":42,"x":1700,"y":2800,"race_num":20300,"interaction_kind":"talk","interaction_ref":"npc:village_guard"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if registrar.calls != 1 || registrar.lastInteractionKind != "talk" || registrar.lastInteractionRef != "npc:village_guard" {
		t.Fatalf("expected interaction metadata to reach static actor registrar, got %+v", registrar)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"interaction_kind":"talk"`) || !strings.Contains(string(body), `"interaction_ref":"npc:village_guard"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalStaticActorsEndpointRejectsUnsupportedInteractionKind(t *testing.T) {
	registrar := &stubStaticActorRegistrar{registered: true}
	mux := RegisterLocalStaticActorEndpoints(NewPprofMux("gamed"), nil, registrar.RegisterStaticActor)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actors", strings.NewReader(`{"name":"QuestMarker","map_index":42,"x":1700,"y":2800,"race_num":20300,"interaction_kind":"quest","interaction_ref":"quest:first_steps"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if registrar.calls != 0 {
		t.Fatalf("expected static actor registrar not to be called, got %d calls", registrar.calls)
	}
}

func TestLocalStaticActorsEndpointRegistersActorWithCombatProfileForLoopbackPost(t *testing.T) {
	registrar := &stubStaticActorRegistrar{registered: true, actor: map[string]any{"entity_id": uint64(1), "name": "TrainingDummy", "map_index": uint32(42), "x": int32(1700), "y": int32(2800), "race_num": uint32(20350), "combat_profile": "training_dummy"}}
	mux := RegisterLocalStaticActorEndpoints(NewPprofMux("gamed"), nil, registrar.RegisterStaticActor)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actors", strings.NewReader(`{"name":"TrainingDummy","map_index":42,"x":1700,"y":2800,"race_num":20350,"combat_profile":"training_dummy"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if registrar.calls != 1 || registrar.lastCombatProfile != "training_dummy" {
		t.Fatalf("expected combat profile to reach static actor registrar, got %+v", registrar)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"combat_profile":"training_dummy"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalStaticActorsEndpointRejectsInvalidSeedBody(t *testing.T) {
	registrar := &stubStaticActorRegistrar{registered: true}
	mux := RegisterLocalStaticActorEndpoints(NewPprofMux("gamed"), nil, registrar.RegisterStaticActor)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actors", strings.NewReader(`{"name":"VillageGuard","map_index":0,"x":1700,"y":2800,"race_num":20300}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if registrar.calls != 0 {
		t.Fatalf("expected static actor registrar not to be called, got %d calls", registrar.calls)
	}
}

func TestLocalStaticActorsEndpointRejectsNULNameBeforeCallback(t *testing.T) {
	registrar := &stubStaticActorRegistrar{registered: true}
	mux := RegisterLocalStaticActorEndpoints(NewPprofMux("gamed"), nil, registrar.RegisterStaticActor)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actors", strings.NewReader(`{"name":"Visible\u0000Hidden","map_index":42,"x":1700,"y":2800,"race_num":20300}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if registrar.calls != 0 {
		t.Fatalf("expected static actor registrar not to be called for NUL name, got %d calls", registrar.calls)
	}
}

func TestLocalStaticActorsEndpointRejectsInvalidUTF8NameBeforeCallback(t *testing.T) {
	registrar := &stubStaticActorRegistrar{registered: true}
	mux := RegisterLocalStaticActorEndpoints(NewPprofMux("gamed"), nil, registrar.RegisterStaticActor)

	body := []byte(`{"name":"Visible`)
	body = append(body, 0xff)
	body = append(body, []byte(`Hidden","map_index":42,"x":1700,"y":2800,"race_num":20300}`)...)
	req := httptest.NewRequest(http.MethodPost, "/local/static-actors", strings.NewReader(string(body)))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if registrar.calls != 0 {
		t.Fatalf("expected static actor registrar not to be called for invalid UTF-8 name, got %d calls", registrar.calls)
	}
}

func TestLocalStaticActorsEndpointRejectsUnsupportedMethod(t *testing.T) {
	snapshotter := &stubStaticActorSnapshotter{}
	registrar := &stubStaticActorRegistrar{registered: true}
	mux := RegisterLocalStaticActorEndpoints(NewPprofMux("gamed"), snapshotter.StaticActors, registrar.RegisterStaticActor)

	req := httptest.NewRequest(http.MethodDelete, "/local/static-actors", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected static actor snapshotter not to be called, got %d calls", snapshotter.calls)
	}
	if registrar.calls != 0 {
		t.Fatalf("expected static actor registrar not to be called, got %d calls", registrar.calls)
	}
}

func TestLocalStaticActorUpdateEndpointUpdatesActorForLoopbackPatch(t *testing.T) {
	updater := &stubStaticActorUpdater{updated: true, actor: map[string]any{"entity_id": uint64(7), "name": "Merchant", "map_index": uint32(99), "x": int32(900), "y": int32(1200), "race_num": uint32(9001)}}
	mux := RegisterLocalStaticActorUpdateEndpoint(NewPprofMux("gamed"), updater.UpdateStaticActor)

	req := httptest.NewRequest(http.MethodPatch, "/local/static-actors/7", strings.NewReader(`{"name":"Merchant","map_index":99,"x":900,"y":1200,"race_num":9001}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if updater.calls != 1 || updater.lastEntityID != 7 || updater.lastName != "Merchant" || updater.lastMapIndex != 99 || updater.lastX != 900 || updater.lastY != 1200 || updater.lastRaceNum != 9001 || updater.lastInteractionKind != "" || updater.lastInteractionRef != "" {
		t.Fatalf("unexpected static actor updater call state: %+v", updater)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"entity_id":7`) || !strings.Contains(string(body), `"name":"Merchant"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalStaticActorUpdateEndpointUpdatesInteractionMetadataForLoopbackPatch(t *testing.T) {
	updater := &stubStaticActorUpdater{updated: true, actor: map[string]any{"entity_id": uint64(7), "name": "Merchant", "map_index": uint32(99), "x": int32(900), "y": int32(1200), "race_num": uint32(9001), "interaction_kind": "info", "interaction_ref": "lore:merchant"}}
	mux := RegisterLocalStaticActorUpdateEndpoint(NewPprofMux("gamed"), updater.UpdateStaticActor)

	req := httptest.NewRequest(http.MethodPatch, "/local/static-actors/7", strings.NewReader(`{"name":"Merchant","map_index":99,"x":900,"y":1200,"race_num":9001,"interaction_kind":"info","interaction_ref":"lore:merchant"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if updater.calls != 1 || updater.lastInteractionKind != "info" || updater.lastInteractionRef != "lore:merchant" {
		t.Fatalf("expected interaction metadata to reach static actor updater, got %+v", updater)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"interaction_kind":"info"`) || !strings.Contains(string(body), `"interaction_ref":"lore:merchant"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalStaticActorUpdateEndpointRejectsUnsupportedInteractionKind(t *testing.T) {
	updater := &stubStaticActorUpdater{updated: true}
	mux := RegisterLocalStaticActorUpdateEndpoint(NewPprofMux("gamed"), updater.UpdateStaticActor)

	req := httptest.NewRequest(http.MethodPatch, "/local/static-actors/7", strings.NewReader(`{"name":"QuestMarker","map_index":42,"x":1700,"y":2800,"race_num":20300,"interaction_kind":"quest","interaction_ref":"quest:first_steps"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if updater.calls != 0 {
		t.Fatalf("expected static actor updater not to be called, got %d calls", updater.calls)
	}
}

func TestLocalStaticActorUpdateEndpointUpdatesCombatProfileForLoopbackPatch(t *testing.T) {
	updater := &stubStaticActorUpdater{updated: true, actor: map[string]any{"entity_id": uint64(7), "name": "TrainingDummy", "map_index": uint32(99), "x": int32(900), "y": int32(1200), "race_num": uint32(20350), "combat_profile": "training_dummy"}}
	mux := RegisterLocalStaticActorUpdateEndpoint(NewPprofMux("gamed"), updater.UpdateStaticActor)

	req := httptest.NewRequest(http.MethodPatch, "/local/static-actors/7", strings.NewReader(`{"name":"TrainingDummy","map_index":99,"x":900,"y":1200,"race_num":20350,"combat_profile":"training_dummy"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if updater.calls != 1 || updater.lastCombatProfile != "training_dummy" {
		t.Fatalf("expected combat profile to reach static actor updater, got %+v", updater)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"combat_profile":"training_dummy"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalStaticActorUpdateEndpointRejectsInvalidBody(t *testing.T) {
	updater := &stubStaticActorUpdater{updated: true}
	mux := RegisterLocalStaticActorUpdateEndpoint(NewPprofMux("gamed"), updater.UpdateStaticActor)

	req := httptest.NewRequest(http.MethodPatch, "/local/static-actors/7", strings.NewReader(`{"name":"Merchant","map_index":0,"x":900,"y":1200,"race_num":9001}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if updater.calls != 0 {
		t.Fatalf("expected static actor updater not to be called, got %d calls", updater.calls)
	}
}

func TestLocalStaticActorUpdateEndpointRejectsNULNameBeforeCallback(t *testing.T) {
	updater := &stubStaticActorUpdater{updated: true}
	mux := RegisterLocalStaticActorUpdateEndpoint(NewPprofMux("gamed"), updater.UpdateStaticActor)

	req := httptest.NewRequest(http.MethodPatch, "/local/static-actors/7", strings.NewReader(`{"name":"Visible\u0000Hidden","map_index":99,"x":900,"y":1200,"race_num":9001}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if updater.calls != 0 {
		t.Fatalf("expected static actor updater not to be called for NUL name, got %d calls", updater.calls)
	}
}

func TestLocalStaticActorUpdateEndpointRejectsInvalidUTF8NameBeforeCallback(t *testing.T) {
	updater := &stubStaticActorUpdater{updated: true}
	mux := RegisterLocalStaticActorUpdateEndpoint(NewPprofMux("gamed"), updater.UpdateStaticActor)

	body := []byte(`{"name":"Visible`)
	body = append(body, 0xff)
	body = append(body, []byte(`Hidden","map_index":99,"x":900,"y":1200,"race_num":9001}`)...)
	req := httptest.NewRequest(http.MethodPatch, "/local/static-actors/7", strings.NewReader(string(body)))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if updater.calls != 0 {
		t.Fatalf("expected static actor updater not to be called for invalid UTF-8 name, got %d calls", updater.calls)
	}
}

func TestLocalStaticActorUpdateEndpointRejectsInvalidEntityID(t *testing.T) {
	updater := &stubStaticActorUpdater{updated: true}
	mux := RegisterLocalStaticActorUpdateEndpoint(NewPprofMux("gamed"), updater.UpdateStaticActor)

	req := httptest.NewRequest(http.MethodPatch, "/local/static-actors/not-a-number", strings.NewReader(`{"name":"Merchant","map_index":99,"x":900,"y":1200,"race_num":9001}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if updater.calls != 0 {
		t.Fatalf("expected static actor updater not to be called, got %d calls", updater.calls)
	}
}

func TestLocalStaticActorUpdateEndpointReturnsNotFoundForUnknownActor(t *testing.T) {
	updater := &stubStaticActorUpdater{}
	mux := RegisterLocalStaticActorUpdateEndpoint(NewPprofMux("gamed"), updater.UpdateStaticActor)

	req := httptest.NewRequest(http.MethodPatch, "/local/static-actors/7", strings.NewReader(`{"name":"Merchant","map_index":99,"x":900,"y":1200,"race_num":9001}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if updater.calls != 1 || updater.lastEntityID != 7 {
		t.Fatalf("expected static actor updater to be called once for not-found path, got %+v", updater)
	}
}

func TestLocalStaticActorUpdateEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	updater := &stubStaticActorUpdater{updated: true}
	mux := RegisterLocalStaticActorUpdateEndpoint(NewPprofMux("gamed"), updater.UpdateStaticActor)

	req := httptest.NewRequest(http.MethodPatch, "/local/static-actors/7", strings.NewReader(`{"name":"Merchant","map_index":99,"x":900,"y":1200,"race_num":9001}`))
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if updater.calls != 0 {
		t.Fatalf("expected static actor updater not to be called, got %d calls", updater.calls)
	}
}

func TestLocalStaticActorUpdateEndpointRejectsUnsupportedMethod(t *testing.T) {
	updater := &stubStaticActorUpdater{updated: true}
	mux := RegisterLocalStaticActorUpdateEndpoint(NewPprofMux("gamed"), updater.UpdateStaticActor)

	req := httptest.NewRequest(http.MethodDelete, "/local/static-actors/7", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if updater.calls != 0 {
		t.Fatalf("expected static actor updater not to be called, got %d calls", updater.calls)
	}
}

func TestLocalStaticActorDeleteEndpointRemovesActorForLoopbackDelete(t *testing.T) {
	remover := &stubStaticActorRemover{removed: true, actor: map[string]any{"entity_id": uint64(7), "name": "VillageGuard", "map_index": uint32(42), "x": int32(1700), "y": int32(2800), "race_num": uint32(20300)}}
	mux := RegisterLocalStaticActorDeleteEndpoint(NewPprofMux("gamed"), remover.RemoveStaticActor)

	req := httptest.NewRequest(http.MethodDelete, "/local/static-actors/7", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if remover.calls != 1 || remover.lastEntityID != 7 {
		t.Fatalf("unexpected static actor remover call state: %+v", remover)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"entity_id":7`) || !strings.Contains(string(body), `"name":"VillageGuard"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalStaticActorDeleteEndpointRejectsInvalidEntityID(t *testing.T) {
	remover := &stubStaticActorRemover{removed: true}
	mux := RegisterLocalStaticActorDeleteEndpoint(NewPprofMux("gamed"), remover.RemoveStaticActor)

	req := httptest.NewRequest(http.MethodDelete, "/local/static-actors/not-a-number", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if remover.calls != 0 {
		t.Fatalf("expected static actor remover not to be called, got %d calls", remover.calls)
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

type stubConnectedCharactersSnapshotter struct {
	characters []map[string]any
	calls      int
}

func (s *stubConnectedCharactersSnapshotter) ConnectedCharacters() any {
	s.calls++
	return s.characters
}

type stubCharacterVisibilitySnapshotter struct {
	snapshots any
	calls     int
}

func (s *stubCharacterVisibilitySnapshotter) CharacterVisibility() any {
	s.calls++
	return s.snapshots
}

type stubMapOccupancySnapshotter struct {
	snapshots []map[string]any
	calls     int
}

func (s *stubMapOccupancySnapshotter) MapOccupancy() any {
	s.calls++
	return s.snapshots
}

type stubMapOccupancyLookup struct {
	snapshots    map[uint32]any
	calls        int
	lastMapIndex uint32
}

func (s *stubMapOccupancyLookup) MapOccupancy(mapIndex uint32) (any, bool) {
	s.calls++
	s.lastMapIndex = mapIndex
	value, ok := s.snapshots[mapIndex]
	return value, ok
}

type stubRuntimeConfigSnapshotter struct {
	snapshot map[string]any
	calls    int
}

func (s *stubRuntimeConfigSnapshotter) RuntimeConfig() any {
	s.calls++
	return s.snapshot
}

type stubPersistenceStatusSnapshotter struct {
	snapshot map[string]any
	calls    int
}

func (s *stubPersistenceStatusSnapshotter) PersistenceStatus() any {
	s.calls++
	return s.snapshot
}

type stubListSnapshotter struct {
	snapshots []map[string]any
	calls     int
}

func (s *stubListSnapshotter) Snapshot() any {
	s.calls++
	return s.snapshots
}

type stubNamedSnapshotter struct {
	snapshots map[string]any
	calls     int
	lastName  string
}

func (s *stubNamedSnapshotter) Snapshot(name string) (any, bool) {
	s.calls++
	s.lastName = name
	value, ok := s.snapshots[name]
	return value, ok
}

type stubStaticActorSnapshotter struct {
	actors []map[string]any
	calls  int
}

func (s *stubStaticActorSnapshotter) StaticActors() any {
	s.calls++
	return s.actors
}

type stubStaticActorLookup struct {
	actors       map[uint64]any
	calls        int
	lastEntityID uint64
}

func (s *stubStaticActorLookup) StaticActor(entityID uint64) (any, bool) {
	s.calls++
	s.lastEntityID = entityID
	actor, ok := s.actors[entityID]
	return actor, ok
}

type stubStaticActorRegistrar struct {
	actor               map[string]any
	registered          bool
	calls               int
	lastName            string
	lastMapIndex        uint32
	lastX               int32
	lastY               int32
	lastRaceNum         uint32
	lastInteractionKind string
	lastInteractionRef  string
	lastCombatProfile   string
}

func (r *stubStaticActorRegistrar) RegisterStaticActor(name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string, combatProfile string) (any, bool) {
	r.calls++
	r.lastName = name
	r.lastMapIndex = mapIndex
	r.lastX = x
	r.lastY = y
	r.lastRaceNum = raceNum
	r.lastInteractionKind = interactionKind
	r.lastInteractionRef = interactionRef
	r.lastCombatProfile = combatProfile
	return r.actor, r.registered
}

type stubStaticActorUpdater struct {
	actor               map[string]any
	updated             bool
	calls               int
	lastEntityID        uint64
	lastName            string
	lastMapIndex        uint32
	lastX               int32
	lastY               int32
	lastRaceNum         uint32
	lastInteractionKind string
	lastInteractionRef  string
	lastCombatProfile   string
}

func (r *stubStaticActorUpdater) UpdateStaticActor(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string, combatProfile string) (any, bool) {
	r.calls++
	r.lastEntityID = entityID
	r.lastName = name
	r.lastMapIndex = mapIndex
	r.lastX = x
	r.lastY = y
	r.lastRaceNum = raceNum
	r.lastInteractionKind = interactionKind
	r.lastInteractionRef = interactionRef
	r.lastCombatProfile = combatProfile
	return r.actor, r.updated
}

type stubStaticActorRemover struct {
	actor        map[string]any
	removed      bool
	calls        int
	lastEntityID uint64
}

func (r *stubStaticActorRemover) RemoveStaticActor(entityID uint64) (any, bool) {
	r.calls++
	r.lastEntityID = entityID
	return r.actor, r.removed
}

type stubRelocationPreviewer struct {
	preview      map[string]any
	found        bool
	calls        int
	lastName     string
	lastMapIndex uint32
	lastX        int32
	lastY        int32
}

func (p *stubRelocationPreviewer) PreviewRelocation(name string, mapIndex uint32, x int32, y int32) (any, bool) {
	p.calls++
	p.lastName = name
	p.lastMapIndex = mapIndex
	p.lastX = x
	p.lastY = y
	return p.preview, p.found
}

type stubCharacterTransferer struct {
	result       map[string]any
	found        bool
	calls        int
	lastName     string
	lastMapIndex uint32
	lastX        int32
	lastY        int32
}

func (t *stubCharacterTransferer) TransferCharacter(name string, mapIndex uint32, x int32, y int32) (any, bool) {
	t.calls++
	t.lastName = name
	t.lastMapIndex = mapIndex
	t.lastX = x
	t.lastY = y
	return t.result, t.found
}
func TestLocalGroundItemEndpointReturnsExactGroundSnapshotForLoopbackGet(t *testing.T) {
	snapshot := worldruntime.GroundItemSnapshot{VID: 0x0700002e, Vnum: 3002, Count: 3, OwnerName: "GroundOwner", MapIndex: 1, X: 1200, Y: 2200}
	mux := RegisterLocalGroundItemEndpoint(NewPprofMux("gamed"), func(vid uint32) (any, bool) {
		if vid != snapshot.VID {
			return nil, false
		}
		return snapshot, true
	})

	req := httptest.NewRequest(http.MethodGet, "/local/ground-items/117440558", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"vid":117440558`) || !strings.Contains(string(body), `"vnum":3002`) || !strings.Contains(string(body), `"owner_name":"GroundOwner"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalGroundItemEndpointAcceptsHexVID(t *testing.T) {
	snapshot := worldruntime.GroundItemSnapshot{VID: 0x0700002e, Vnum: 3002, Count: 3, OwnerName: "GroundOwner", MapIndex: 1, X: 1200, Y: 2200}
	mux := RegisterLocalGroundItemEndpoint(NewPprofMux("gamed"), func(vid uint32) (any, bool) {
		if vid != snapshot.VID {
			t.Fatalf("expected hex VID to decode as %d, got %d", snapshot.VID, vid)
		}
		return snapshot, true
	})

	req := httptest.NewRequest(http.MethodGet, "/local/ground-items/0x0700002e", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"vid":117440558`) || !strings.Contains(string(body), `"owner_name":"GroundOwner"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalGroundItemEndpointRejectsInvalidVID(t *testing.T) {
	mux := RegisterLocalGroundItemEndpoint(NewPprofMux("gamed"), func(uint32) (any, bool) {
		t.Fatal("ground item lookup should not be called for invalid VID")
		return nil, false
	})

	req := httptest.NewRequest(http.MethodGet, "/local/ground-items/not-a-vid", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestLocalGroundItemEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	mux := RegisterLocalGroundItemEndpoint(NewPprofMux("gamed"), func(uint32) (any, bool) {
		t.Fatal("ground item lookup should not be called for non-loopback callers")
		return nil, false
	})

	req := httptest.NewRequest(http.MethodGet, "/local/ground-items/117440558", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestLocalGroundItemEndpointReturnsNotFoundForMissingVID(t *testing.T) {
	mux := RegisterLocalGroundItemEndpoint(NewPprofMux("gamed"), func(uint32) (any, bool) {
		return nil, false
	})

	req := httptest.NewRequest(http.MethodGet, "/local/ground-items/117440558", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}
