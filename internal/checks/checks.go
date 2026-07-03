package checks

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/neiroun/thirdcupd/internal/config"
)

type Result struct {
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	OK        bool           `json:"ok"`
	Message   string         `json:"message"`
	LatencyMS int64          `json:"latency_ms"`
	CheckedAt time.Time      `json:"checked_at"`
	Details   map[string]any `json:"details,omitempty"`
}

type Checker interface {
	Name() string
	Type() string
	Check(ctx context.Context) Result
}

func Build(cfg config.Config) []Checker {
	items := make([]Checker, 0, len(cfg.Checks.HTTP)+len(cfg.Checks.TCP)+len(cfg.Checks.Disk))
	for _, item := range cfg.Checks.HTTP {
		items = append(items, newHTTPChecker(item))
	}
	for _, item := range cfg.Checks.TCP {
		items = append(items, newTCPChecker(item))
	}
	for _, item := range cfg.Checks.Disk {
		items = append(items, newDiskChecker(item))
	}
	return items
}

type httpChecker struct {
	name           string
	url            string
	method         string
	headers        map[string]string
	timeout        time.Duration
	expectedStatus map[int]struct{}
	maxLatency     time.Duration
	client         *http.Client
}

func newHTTPChecker(cfg config.HTTPCheck) Checker {
	expected := make(map[int]struct{}, len(cfg.ExpectedStatus))
	for _, status := range cfg.ExpectedStatus {
		expected[status] = struct{}{}
	}
	timeout := cfg.Timeout.Duration()
	return &httpChecker{
		name:           cfg.Name,
		url:            cfg.URL,
		method:         cfg.Method,
		headers:        cfg.Headers,
		timeout:        timeout,
		expectedStatus: expected,
		maxLatency:     cfg.MaxLatency.Duration(),
		client:         &http.Client{Timeout: timeout},
	}
}

func (c *httpChecker) Name() string { return c.name }

func (c *httpChecker) Type() string { return "http" }

func (c *httpChecker) Check(ctx context.Context) Result {
	start := time.Now()
	checkedAt := start.UTC()

	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(checkCtx, c.method, c.url, nil)
	if err != nil {
		return c.result(false, checkedAt, start, fmt.Sprintf("request build failed: %v", err), nil)
	}
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return c.result(false, checkedAt, start, fmt.Sprintf("request failed: %v", err), nil)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	details := map[string]any{
		"status_code": resp.StatusCode,
		"url":         c.url,
	}
	if _, ok := c.expectedStatus[resp.StatusCode]; !ok {
		return c.result(false, checkedAt, start, fmt.Sprintf("unexpected status code: %d", resp.StatusCode), details)
	}
	if c.maxLatency > 0 && time.Since(start) > c.maxLatency {
		return c.result(false, checkedAt, start, fmt.Sprintf("latency is above %s", c.maxLatency), details)
	}
	return c.result(true, checkedAt, start, "http check passed", details)
}

func (c *httpChecker) result(ok bool, checkedAt, start time.Time, message string, details map[string]any) Result {
	return Result{
		Name:      c.name,
		Type:      c.Type(),
		OK:        ok,
		Message:   message,
		LatencyMS: time.Since(start).Milliseconds(),
		CheckedAt: checkedAt,
		Details:   details,
	}
}

type tcpChecker struct {
	name    string
	address string
	timeout time.Duration
}

func newTCPChecker(cfg config.TCPCheck) Checker {
	return &tcpChecker{
		name:    cfg.Name,
		address: cfg.Address,
		timeout: cfg.Timeout.Duration(),
	}
}

func (c *tcpChecker) Name() string { return c.name }

func (c *tcpChecker) Type() string { return "tcp" }

func (c *tcpChecker) Check(ctx context.Context) Result {
	start := time.Now()
	checkedAt := start.UTC()

	dialer := net.Dialer{Timeout: c.timeout}
	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	conn, err := dialer.DialContext(checkCtx, "tcp", c.address)
	if err != nil {
		return Result{
			Name:      c.name,
			Type:      c.Type(),
			OK:        false,
			Message:   fmt.Sprintf("tcp connect failed: %v", err),
			LatencyMS: time.Since(start).Milliseconds(),
			CheckedAt: checkedAt,
			Details:   map[string]any{"address": c.address},
		}
	}
	_ = conn.Close()
	return Result{
		Name:      c.name,
		Type:      c.Type(),
		OK:        true,
		Message:   "tcp check passed",
		LatencyMS: time.Since(start).Milliseconds(),
		CheckedAt: checkedAt,
		Details:   map[string]any{"address": c.address},
	}
}

type diskChecker struct {
	name            string
	path            string
	maxUsagePercent float64
}

func newDiskChecker(cfg config.DiskCheck) Checker {
	return &diskChecker{
		name:            cfg.Name,
		path:            cfg.Path,
		maxUsagePercent: cfg.MaxUsagePercent,
	}
}

func (c *diskChecker) Name() string { return c.name }

func (c *diskChecker) Type() string { return "disk" }

func (c *diskChecker) Check(context.Context) Result {
	start := time.Now()
	checkedAt := start.UTC()

	var stat syscall.Statfs_t
	if err := syscall.Statfs(c.path, &stat); err != nil {
		return Result{
			Name:      c.name,
			Type:      c.Type(),
			OK:        false,
			Message:   fmt.Sprintf("statfs failed: %v", err),
			LatencyMS: time.Since(start).Milliseconds(),
			CheckedAt: checkedAt,
			Details:   map[string]any{"path": c.path},
		}
	}

	totalBytes := float64(stat.Blocks) * float64(stat.Bsize)
	availableBytes := float64(stat.Bavail) * float64(stat.Bsize)
	if totalBytes <= 0 {
		return Result{
			Name:      c.name,
			Type:      c.Type(),
			OK:        false,
			Message:   "filesystem reports zero capacity",
			LatencyMS: time.Since(start).Milliseconds(),
			CheckedAt: checkedAt,
			Details:   map[string]any{"path": c.path},
		}
	}

	usedPercent := (1 - availableBytes/totalBytes) * 100
	details := map[string]any{
		"path":              c.path,
		"used_percent":      round2(usedPercent),
		"max_usage_percent": c.maxUsagePercent,
		"total_bytes":       uint64(totalBytes),
		"available_bytes":   uint64(availableBytes),
	}
	if usedPercent > c.maxUsagePercent {
		return Result{
			Name:      c.name,
			Type:      c.Type(),
			OK:        false,
			Message:   fmt.Sprintf("disk usage %.2f%% is above %.2f%%", usedPercent, c.maxUsagePercent),
			LatencyMS: time.Since(start).Milliseconds(),
			CheckedAt: checkedAt,
			Details:   details,
		}
	}
	return Result{
		Name:      c.name,
		Type:      c.Type(),
		OK:        true,
		Message:   fmt.Sprintf("disk usage %.2f%%", usedPercent),
		LatencyMS: time.Since(start).Milliseconds(),
		CheckedAt: checkedAt,
		Details:   details,
	}
}

func round2(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}
