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

func TestStoreRecoveryThreshold(t *testing.T) {
	store := New(1, 2)

	failed := store.Apply(result(false))
	if failed.State.Status != StatusUnhealthy {
		t.Fatalf("expected unhealthy after failure, got %s", failed.State.Status)
	}

	firstSuccess := store.Apply(result(true))
	if firstSuccess.State.Status != StatusRecovering {
		t.Fatalf("expected recovering after first success, got %s", firstSuccess.State.Status)
	}
	if got := store.Snapshot().Overall; got != StatusRecovering {
		t.Fatalf("expected overall recovering, got %s", got)
	}

	secondSuccess := store.Apply(result(true))
	if secondSuccess.State.Status != StatusHealthy {
		t.Fatalf("expected healthy after second success, got %s", secondSuccess.State.Status)
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
