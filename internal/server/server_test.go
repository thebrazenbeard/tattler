package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
