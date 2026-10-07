package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/thebrazenbeard/tattler/internal/model"
)

func TestIndexUsesTattlerIcon(t *testing.T) {
	s := &Server{State: NewState(10)}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `class="brand-icon"`) {
		t.Fatalf("index does not render the Tattler brand icon")
	}
	if !strings.Contains(body, `rel="icon" href="/assets/tattler-icon.png"`) {
		t.Fatalf("index does not expose the Tattler favicon")
	}
}

func TestTattlerIconAsset(t *testing.T) {
	s := &Server{State: NewState(10)}
	req := httptest.NewRequest(http.MethodGet, "/assets/tattler-icon.png", nil)
	rec := httptest.NewRecorder()

	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("content-type = %q, want image/png", got)
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		t.Fatalf("asset is not a PNG")
	}
}

func TestIndexRendersUnavailableMetricsAsNA(t *testing.T) {
	s := &Server{State: NewState(10)}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "unavailable_metrics") {
		t.Fatal("dashboard does not consult unavailable_metrics")
	}
	if !strings.Contains(body, "'n/a'") {
		t.Fatal("dashboard does not render unavailable metrics as n/a")
	}
}

func TestStatusReportsObservationCountWithCompatibilityAlias(t *testing.T) {
	state := NewState(10)
	state.SetCurrent([]model.Connection{
		{Protocol: "tcp", Kind: "tcp_listener"},
		{Protocol: "udp", Kind: "udp_endpoint"},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	(&Server{State: state}).Handler().ServeHTTP(rec, req)

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["current_observations"] != float64(2) {
		t.Fatalf("current_observations=%v, want 2", got["current_observations"])
	}
	if got["current_connections"] != float64(2) {
		t.Fatalf("current_connections=%v, want compatibility alias 2", got["current_connections"])
	}
}

func TestCurrentReportsFirstSeenAndLiveAge(t *testing.T) {
	state := NewState(10)
	first := time.Now().UTC().Add(-90 * time.Second)
	state.SetCurrent([]model.Connection{
		{Protocol: "tcp", Kind: model.ObservationTCPSession, FirstSeen: first},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/current", nil)
	(&Server{State: state}).Handler().ServeHTTP(rec, req)

	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("current len=%d, want 1", len(got))
	}
	if got[0]["first_seen"] == nil {
		t.Fatalf("first_seen missing: %v", got[0])
	}
	age, ok := got[0]["age_seconds"].(float64)
	if !ok || age < 89 || age > 92 {
		t.Fatalf("age_seconds=%v, want about 90", got[0]["age_seconds"])
	}
}

func TestIndexSortsLiveObservationsByAgeDescending(t *testing.T) {
	s := &Server{State: NewState(10)}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	s.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Observed for ↓") {
		t.Fatal("dashboard missing live-age column")
	}
	if !strings.Contains(body, "age_seconds") || !strings.Contains(body, ".sort(") {
		t.Fatal("dashboard does not sort current observations by live age")
	}
}

func TestSemanticEventsAcceptWebhookMetadataWithoutPayload(t *testing.T) {
	state := NewState(10)
	s := &Server{State: state}
	body := []byte(`{
		"kind":"webhook_delivery",
		"protocol":"https",
		"direction":"outbound",
		"peer":"hooks.example.test",
		"method":"POST",
		"route":"/hooks/github",
		"status":202,
		"duration_ms":184,
		"bytes_out":512,
		"provider":"github",
		"event_type":"push",
		"delivery_id":"delivery-123",
		"retry":1,
		"signature_valid":true
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/semantic-events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST status=%d body=%s", rec.Code, rec.Body.String())
	}

	var accepted model.SemanticObservation
	if err := json.Unmarshal(rec.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	if accepted.Kind != model.SemanticWebhookDelivery || accepted.Evidence.Confidence != model.ProtocolConfidenceReported || accepted.Evidence.Source != model.ProtocolSourceReported {
		t.Fatalf("accepted=%+v", accepted)
	}
	if accepted.ID == "" || accepted.ObservedAt.IsZero() {
		t.Fatalf("server did not stamp observation: %+v", accepted)
	}

	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/semantic-events", nil)
	s.Handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status=%d", getRec.Code)
	}
	var got []model.SemanticObservation
	if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != accepted.ID {
		t.Fatalf("events=%+v", got)
	}
}

func TestSemanticEventsRejectRawURLAndQueryRoute(t *testing.T) {
	s := &Server{State: NewState(10)}
	cases := []string{
		`{"kind":"http_transaction","protocol":"https","url":"https://example.test/private?token=secret"}`,
		`{"kind":"http_transaction","protocol":"https","route":"/api/users?id=42"}`,
	}
	for _, raw := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/semantic-events", strings.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		s.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("raw=%s status=%d body=%s", raw, rec.Code, rec.Body.String())
		}
	}
}

func TestSemanticEventsAreBoundedAndStatusCountsThem(t *testing.T) {
	state := NewState(2)
	state.AddSemantic(
		model.NewSemanticObservation(time.Unix(1, 0), model.SemanticObservation{Kind: model.SemanticHTTPTransaction, Protocol: "http"}),
		model.NewSemanticObservation(time.Unix(2, 0), model.SemanticObservation{Kind: model.SemanticHTTPTransaction, Protocol: "http2"}),
		model.NewSemanticObservation(time.Unix(3, 0), model.SemanticObservation{Kind: model.SemanticGRPCRPC, Protocol: "grpc"}),
	)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	(&Server{State: state}).Handler().ServeHTTP(rec, req)
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["semantic_events"] != float64(2) {
		t.Fatalf("semantic_events=%v want 2", got["semantic_events"])
	}
}

func TestSemanticEventsRequireJSONMediaType(t *testing.T) {
	s := &Server{State: NewState(10)}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/semantic-events", strings.NewReader(`{"kind":"http_transaction","protocol":"http"}`))
	req.Header.Set("Content-Type", "text/plain")
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestIndexShowsProtocolEvidenceAndSemanticActivity(t *testing.T) {
	s := &Server{State: NewState(10)}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	s.Handler().ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"Transport", "Protocol evidence", "Semantic activity", "/api/v1/semantic-events?limit=200"} {
		if !strings.Contains(body, want) {
			t.Fatalf("dashboard missing %q", want)
		}
	}
}

func TestSemanticEventsEmptyCollectionIsJSONArray(t *testing.T) {
	s := &Server{State: NewState(10)}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/semantic-events", nil)
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Fatalf("body=%q, want []", got)
	}
}
