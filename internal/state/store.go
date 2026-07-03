package state

import (
	"sort"
	"sync"
	"time"

	"github.com/neiroun/thirdcupd/internal/checks"
)

const (
	StatusUnknown    = "unknown"
	StatusHealthy    = "healthy"
	StatusRecovering = "recovering"
	StatusDegraded   = "degraded"
	StatusUnhealthy  = "unhealthy"
)

type Store struct {
	mu               sync.RWMutex
	startedAt        time.Time
	checks           map[string]CheckState
	failureThreshold int
	successThreshold int
}

type CheckState struct {
	Name                 string         `json:"name"`
	Type                 string         `json:"type"`
	Status               string         `json:"status"`
	OK                   bool           `json:"ok"`
	Message              string         `json:"message"`
	LastLatencyMS        int64          `json:"last_latency_ms"`
	LastCheckedAt        time.Time      `json:"last_checked_at"`
	LastTransitionAt     time.Time      `json:"last_transition_at"`
	ConsecutiveFailures  int            `json:"consecutive_failures"`
	ConsecutiveSuccesses int            `json:"consecutive_successes"`
	TotalChecks          int64          `json:"total_checks"`
	TotalFailures        int64          `json:"total_failures"`
	Details              map[string]any `json:"details,omitempty"`
}

type Update struct {
	State          CheckState `json:"state"`
	PreviousStatus string     `json:"previous_status"`
	Transition     bool       `json:"transition"`
}

type Snapshot struct {
	GeneratedAt time.Time    `json:"generated_at"`
	StartedAt   time.Time    `json:"started_at"`
	Overall     string       `json:"overall"`
	Checks      []CheckState `json:"checks"`
	Totals      Totals       `json:"totals"`
}

type Totals struct {
	Checks    int `json:"checks"`
	Healthy   int `json:"healthy"`
	Degraded  int `json:"degraded"`
	Unhealthy int `json:"unhealthy"`
	Unknown   int `json:"unknown"`
}

func New(failureThreshold, successThreshold int) *Store {
	return &Store{
		startedAt:        time.Now().UTC(),
		checks:           make(map[string]CheckState),
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
	}
}

func (s *Store) Apply(result checks.Result) Update {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := result.Type + ":" + result.Name
	now := time.Now().UTC()

	state, ok := s.checks[key]
	if !ok {
		state = CheckState{
			Name:             result.Name,
			Type:             result.Type,
			Status:           StatusUnknown,
			LastTransitionAt: now,
		}
	}

	previous := state.Status
	state.OK = result.OK
	state.Message = result.Message
	state.LastLatencyMS = result.LatencyMS
	state.LastCheckedAt = result.CheckedAt
	state.Details = result.Details
	state.TotalChecks++

	if result.OK {
		state.ConsecutiveSuccesses++
		state.ConsecutiveFailures = 0
		if state.ConsecutiveSuccesses >= s.successThreshold {
			state.Status = StatusHealthy
		} else {
			state.Status = StatusRecovering
		}
	} else {
		state.TotalFailures++
		state.ConsecutiveFailures++
		state.ConsecutiveSuccesses = 0
		if state.ConsecutiveFailures >= s.failureThreshold {
			state.Status = StatusUnhealthy
		} else {
			state.Status = StatusDegraded
		}
	}

	transition := previous != state.Status
	if transition {
		state.LastTransitionAt = now
	}
	s.checks[key] = state

	return Update{
		State:          state,
		PreviousStatus: previous,
		Transition:     transition,
	}
}

func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	checks := make([]CheckState, 0, len(s.checks))
	totals := Totals{Checks: len(s.checks)}
	for _, item := range s.checks {
		checks = append(checks, item)
		switch item.Status {
		case StatusHealthy:
			totals.Healthy++
		case StatusUnhealthy:
			totals.Unhealthy++
		case StatusUnknown:
			totals.Unknown++
		default:
			totals.Degraded++
		}
	}
	sort.Slice(checks, func(i, j int) bool {
		if checks[i].Type == checks[j].Type {
			return checks[i].Name < checks[j].Name
		}
		return checks[i].Type < checks[j].Type
	})

	return Snapshot{
		GeneratedAt: time.Now().UTC(),
		StartedAt:   s.startedAt,
		Overall:     overall(totals),
		Checks:      checks,
		Totals:      totals,
	}
}

func overall(totals Totals) string {
	if totals.Checks == 0 || totals.Unknown > 0 {
		return StatusUnknown
	}
	if totals.Unhealthy > 0 {
		return StatusUnhealthy
	}
	if totals.Degraded > 0 {
		return StatusDegraded
	}
	return StatusHealthy
}
