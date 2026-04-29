package ops

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const expectedMerchantPreview = "Village Merchant: [0] Small Red Potion x1 @ 50g; [1] Wooden Sword x1 @ 500g"

func TestLocalInteractionVisibilityEndpointReturnsPreviewJSONForLoopbackGet(t *testing.T) {
	snapshotter := &stubInteractionVisibilitySnapshotter{snapshots: []map[string]any{{
		"name":      "PeerOne",
		"vid":       uint32(0x02040101),
		"map_index": uint32(42),
		"x":         int32(1700),
		"y":         int32(2800),
		"visible_interactable_static_actors": []map[string]any{
			{
				"entity_id":          uint64(7),
				"name":               "VillageGuard",
				"interaction_kind":   "talk",
				"interaction_ref":    "npc:village_guard",
				"preview":            "VillageGuard:\nKeep your blade sharp.",
				"resolution_failure": "",
			},
			{
				"entity_id":          uint64(8),
				"name":               "Merchant",
				"interaction_kind":   "shop_preview",
				"interaction_ref":    "npc:merchant",
				"preview":            expectedMerchantPreview,
				"resolution_failure": "",
			},
			{
				"entity_id":          uint64(9),
				"name":               "Teleporter",
				"interaction_kind":   "warp",
				"interaction_ref":    "npc:teleporter",
				"preview":            "Step through the gate. [warp -> map 42 @ 1700,2800]",
				"resolution_failure": "",
			},
		},
	}}}
	mux := RegisterLocalInteractionVisibilityEndpoint(NewPprofMux("gamed"), snapshotter.InteractionVisibility)

	req := httptest.NewRequest(http.MethodGet, "/local/interaction-visibility", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 {
		t.Fatalf("expected interaction visibility snapshotter to be called once, got %d calls", snapshotter.calls)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"visible_interactable_static_actors"`) || !strings.Contains(string(body), `"interaction_kind":"talk"`) || !strings.Contains(string(body), `"preview":"VillageGuard:\nKeep your blade sharp."`) || !strings.Contains(string(body), `"interaction_kind":"shop_preview"`) || !strings.Contains(string(body), `"preview":"Village Merchant: [0] Small Red Potion x1 @ 50g; [1] Wooden Sword x1 @ 500g"`) || !strings.Contains(string(body), `"interaction_kind":"warp"`) || !strings.Contains(string(body), `"preview":"Step through the gate. [warp -\u003e map 42 @ 1700,2800]"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalInteractionVisibilityEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubInteractionVisibilitySnapshotter{}
	mux := RegisterLocalInteractionVisibilityEndpoint(NewPprofMux("gamed"), snapshotter.InteractionVisibility)

	req := httptest.NewRequest(http.MethodGet, "/local/interaction-visibility", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected interaction visibility snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

type stubInteractionVisibilitySnapshotter struct {
	snapshots any
	calls     int
}

func (s *stubInteractionVisibilitySnapshotter) InteractionVisibility() any {
	s.calls++
	return s.snapshots
}
