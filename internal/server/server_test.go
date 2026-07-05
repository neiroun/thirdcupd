package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/neiroun/thirdcupd/internal/checks"
	"github.com/neiroun/thirdcupd/internal/state"
)

func TestHealthProbeAllowsDegradedUntilFailureThreshold(t *testing.T) {
	store := state.New(2, 1)
	server := New("127.0.0.1:0", store)

	store.Apply(result(true))
	assertProbe(t, server, "/healthz", http.StatusOK, "healthy\n")

	store.Apply(result(false))
	assertProbe(t, server, "/readyz", http.StatusOK, "degraded\n")

	store.Apply(result(false))
	assertProbe(t, server, "/healthz", http.StatusServiceUnavailable, "unhealthy\n")
}

func TestHealthProbeRejectsUnknownState(t *testing.T) {
	server := New("127.0.0.1:0", state.New(2, 1))
	assertProbe(t, server, "/healthz", http.StatusServiceUnavailable, "unknown\n")
}

func TestMetricsUseStableLabels(t *testing.T) {
	store := state.New(2, 1)
	store.Apply(result(true))
	server := New("127.0.0.1:0", store)

	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(response, request)

	body := response.Body.String()
	want := `thirdcupd_check_status{name="api",status="healthy",type="http"} 1`
	if !strings.Contains(body, want) {
		t.Fatalf("expected stable labels %q in metrics:\n%s", want, body)
	}
	if !strings.Contains(body, "thirdcupd_probe_ready 1") {
		t.Fatalf("expected probe metric in metrics:\n%s", body)
	}
}

func assertProbe(t *testing.T, server *Server, path string, status int, body string) {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(response, request)

	if response.Code != status {
		t.Fatalf("expected %s status %d, got %d", path, status, response.Code)
	}
	if response.Body.String() != body {
		t.Fatalf("expected %s body %q, got %q", path, body, response.Body.String())
	}
}

func result(ok bool) checks.Result {
	return checks.Result{
		Name:      "api",
		Type:      "http",
		OK:        ok,
		Message:   "test",
		CheckedAt: time.Now().UTC(),
	}
}
