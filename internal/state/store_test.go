package state

import (
	"testing"
	"time"

	"github.com/neiroun/thirdcupd/internal/checks"
)

func TestStoreThresholds(t *testing.T) {
	store := New(2, 1)

	first := store.Apply(result(false))
	if first.State.Status != StatusDegraded {
		t.Fatalf("expected degraded after first failure, got %s", first.State.Status)
	}

	second := store.Apply(result(false))
	if second.State.Status != StatusUnhealthy {
		t.Fatalf("expected unhealthy after second failure, got %s", second.State.Status)
	}

	recovered := store.Apply(result(true))
	if recovered.State.Status != StatusHealthy {
		t.Fatalf("expected healthy after success, got %s", recovered.State.Status)
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
