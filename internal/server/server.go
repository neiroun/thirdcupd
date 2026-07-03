package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/neiroun/thirdcupd/internal/state"
)

type Server struct {
	httpServer *http.Server
	store      *state.Store
}

func New(addr string, store *state.Store) *Server {
	s := &Server{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/readyz", s.health)
	mux.HandleFunc("/status", s.status)
	mux.HandleFunc("/metrics", s.metrics)

	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}
	return s
}

func (s *Server) Run(ctx context.Context) error {
	errs := make(chan error, 1)
	go func() {
		err := s.httpServer.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errs <- err
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(shutdownCtx)
	case err := <-errs:
		return err
	}
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	snapshot := s.store.Snapshot()
	if snapshot.Overall != state.StatusHealthy {
		http.Error(w, snapshot.Overall+"\n", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(s.store.Snapshot())
}

func (s *Server) metrics(w http.ResponseWriter, _ *http.Request) {
	snapshot := s.store.Snapshot()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	fmt.Fprintln(w, "# HELP thirdcupd_overall_status Overall status: 1 healthy, 0 otherwise.")
	fmt.Fprintln(w, "# TYPE thirdcupd_overall_status gauge")
	fmt.Fprintf(w, "thirdcupd_overall_status %d\n", boolFloat(snapshot.Overall == state.StatusHealthy))

	fmt.Fprintln(w, "# HELP thirdcupd_check_status Check status: 1 healthy, 0 otherwise.")
	fmt.Fprintln(w, "# TYPE thirdcupd_check_status gauge")
	for _, check := range snapshot.Checks {
		labels := labels(map[string]string{
			"name":   check.Name,
			"type":   check.Type,
			"status": check.Status,
		})
		fmt.Fprintf(w, "thirdcupd_check_status{%s} %d\n", labels, boolFloat(check.Status == state.StatusHealthy))
		fmt.Fprintf(w, "thirdcupd_check_latency_ms{%s} %d\n", labels, check.LastLatencyMS)
		fmt.Fprintf(w, "thirdcupd_check_failures_total{%s} %d\n", labels, check.TotalFailures)
		fmt.Fprintf(w, "thirdcupd_check_runs_total{%s} %d\n", labels, check.TotalChecks)
		fmt.Fprintf(w, "thirdcupd_check_consecutive_failures{%s} %d\n", labels, check.ConsecutiveFailures)
	}
}

func boolFloat(value bool) int {
	if value {
		return 1
	}
	return 0
}

func labels(values map[string]string) string {
	parts := make([]string, 0, len(values))
	for key, value := range values {
		parts = append(parts, key+"="+strconv.Quote(escapeLabel(value)))
	}
	return strings.Join(parts, ",")
}

func escapeLabel(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\n", "\\n")
	return value
}
