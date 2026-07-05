package events

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/neiroun/thirdcupd/internal/checks"
	"github.com/neiroun/thirdcupd/internal/state"
)

func TestWriterWritesJSONLToStdoutAndFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	writer, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	writer.out = &stdout

	result := checks.Result{
		Name:      "api",
		Type:      "http",
		OK:        true,
		Message:   "http check passed",
		LatencyMS: 12,
		CheckedAt: time.Now().UTC(),
	}
	update := state.Update{
		State: state.CheckState{
			Status: state.StatusHealthy,
		},
		PreviousStatus: state.StatusUnknown,
		Transition:     true,
	}

	if err := writer.Write(result, update); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	fileData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONLine(t, stdout.Bytes())
	assertJSONLine(t, fileData)
}

func assertJSONLine(t *testing.T, data []byte) {
	t.Helper()

	if bytes.Count(data, []byte("\n")) != 1 {
		t.Fatalf("expected one JSONL line, got %q", data)
	}

	var event Event
	if err := json.Unmarshal(bytes.TrimSpace(data), &event); err != nil {
		t.Fatalf("expected valid json event: %v", err)
	}
	if event.Name != "api" || event.Status != state.StatusHealthy {
		t.Fatalf("unexpected event: %#v", event)
	}
}
