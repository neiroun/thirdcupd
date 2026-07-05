package checks

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/neiroun/thirdcupd/internal/config"
)

func TestHTTPCheckSupportsStatusLatencyTimeoutAndHeaders(t *testing.T) {
	checker := newHTTPChecker(config.HTTPCheck{
		Name:            "api",
		URL:             "http://example.test/healthz",
		Method:          http.MethodGet,
		Headers:         map[string]string{"X-Check-Token": "secret"},
		ExpectedHeaders: map[string]string{"X-Service-Health": "green"},
		Timeout:         config.Duration(2 * time.Second),
		ExpectedStatus:  []int{http.StatusNoContent},
		MaxLatency:      config.Duration(time.Second),
	}).(*httpChecker)
	checker.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("X-Check-Token"); got != "secret" {
			t.Fatalf("expected request header, got %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Header:     http.Header{"X-Service-Health": []string{"green"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	})}

	result := checker.Check(context.Background())
	if !result.OK {
		t.Fatalf("expected ok result, got %#v", result)
	}
	if result.Details["status_code"] != http.StatusNoContent {
		t.Fatalf("unexpected status details: %#v", result.Details)
	}
}

func TestHTTPCheckFailsOnUnexpectedResponseHeader(t *testing.T) {
	checker := newHTTPChecker(config.HTTPCheck{
		Name:            "api",
		URL:             "http://example.test/healthz",
		Method:          http.MethodGet,
		ExpectedHeaders: map[string]string{"X-Service-Health": "green"},
		Timeout:         config.Duration(2 * time.Second),
		ExpectedStatus:  []int{http.StatusOK},
	}).(*httpChecker)
	checker.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"X-Service-Health": []string{"red"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	})}

	result := checker.Check(context.Background())
	if result.OK {
		t.Fatal("expected header mismatch to fail")
	}
	if !strings.Contains(result.Message, "unexpected response header") {
		t.Fatalf("unexpected message: %s", result.Message)
	}
}

func TestTCPCheckReachability(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()

	checker := &tcpChecker{
		name:    "postgres",
		address: "127.0.0.1:5432",
		timeout: time.Second,
		dialContext: func(context.Context, string, string) (net.Conn, error) {
			return clientConn, nil
		},
	}

	result := checker.Check(context.Background())
	if !result.OK {
		t.Fatalf("expected tcp check to pass, got %#v", result)
	}
}

func TestDiskCheckUsage(t *testing.T) {
	checker := Build(config.Config{
		Checks: config.ChecksConfig{
			Disk: []config.DiskCheck{{
				Name:            "tmp",
				Path:            t.TempDir(),
				MaxUsagePercent: 100,
			}},
		},
	})[0]

	result := checker.Check(context.Background())
	if !result.OK {
		t.Fatalf("expected disk check to pass, got %#v", result)
	}
	if result.Details["used_percent"] == nil {
		t.Fatalf("expected disk usage details, got %#v", result.Details)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
