package events

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/neiroun/thirdcupd/internal/checks"
	"github.com/neiroun/thirdcupd/internal/state"
)

type Writer struct {
	mu   sync.Mutex
	out  io.Writer
	file *os.File
}

type Event struct {
	Time           time.Time      `json:"time"`
	Name           string         `json:"name"`
	Type           string         `json:"type"`
	OK             bool           `json:"ok"`
	Status         string         `json:"status"`
	PreviousStatus string         `json:"previous_status,omitempty"`
	Transition     bool           `json:"transition"`
	Message        string         `json:"message"`
	LatencyMS      int64          `json:"latency_ms"`
	Details        map[string]any `json:"details,omitempty"`
}

func New(path string) (*Writer, error) {
	writer := &Writer{
		out: os.Stdout,
	}
	if path == "" {
		return writer, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	writer.file = file
	return writer, nil
}

func (w *Writer) Write(result checks.Result, update state.Update) error {
	event := Event{
		Time:           time.Now().UTC(),
		Name:           result.Name,
		Type:           result.Type,
		OK:             result.OK,
		Status:         update.State.Status,
		PreviousStatus: update.PreviousStatus,
		Transition:     update.Transition,
		Message:        result.Message,
		LatencyMS:      result.LatencyMS,
		Details:        result.Details,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.out.Write(data); err != nil {
		return err
	}
	if w.file != nil {
		if _, err := w.file.Write(data); err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("close event log: %w", err)
	}
	w.file = nil
	return nil
}
