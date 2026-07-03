package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"
)

type Duration time.Duration

func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		return nil
	}

	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("duration must be a string like \"30s\": %w", err)
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return err
	}
	*d = Duration(value)
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

type Config struct {
	Daemon        DaemonConfig        `json:"daemon"`
	Observability ObservabilityConfig `json:"observability"`
	Checks        ChecksConfig        `json:"checks"`
}

type DaemonConfig struct {
	Interval         Duration `json:"interval"`
	FailureThreshold int      `json:"failure_threshold"`
	SuccessThreshold int      `json:"success_threshold"`
}

type ObservabilityConfig struct {
	ListenAddr string `json:"listen_addr"`
	EventLog   string `json:"event_log"`
	PrettyLogs bool   `json:"pretty_logs"`
}

type ChecksConfig struct {
	HTTP []HTTPCheck `json:"http"`
	TCP  []TCPCheck  `json:"tcp"`
	Disk []DiskCheck `json:"disk"`
}

type HTTPCheck struct {
	Name           string            `json:"name"`
	URL            string            `json:"url"`
	Method         string            `json:"method"`
	Headers        map[string]string `json:"headers"`
	Timeout        Duration          `json:"timeout"`
	ExpectedStatus []int             `json:"expected_status"`
	MaxLatency     Duration          `json:"max_latency"`
}

type TCPCheck struct {
	Name    string   `json:"name"`
	Address string   `json:"address"`
	Timeout Duration `json:"timeout"`
}

type DiskCheck struct {
	Name            string  `json:"name"`
	Path            string  `json:"path"`
	MaxUsagePercent float64 `json:"max_usage_percent"`
}

func Default() Config {
	return Config{
		Daemon: DaemonConfig{
			Interval:         Duration(30 * time.Second),
			FailureThreshold: 2,
			SuccessThreshold: 1,
		},
		Observability: ObservabilityConfig{
			ListenAddr: "127.0.0.1:8374",
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, err
	}

	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (cfg *Config) applyDefaults() {
	if cfg.Daemon.Interval == 0 {
		cfg.Daemon.Interval = Duration(30 * time.Second)
	}
	if cfg.Daemon.FailureThreshold == 0 {
		cfg.Daemon.FailureThreshold = 2
	}
	if cfg.Daemon.SuccessThreshold == 0 {
		cfg.Daemon.SuccessThreshold = 1
	}
	if cfg.Observability.ListenAddr == "" {
		cfg.Observability.ListenAddr = "127.0.0.1:8374"
	}

	for i := range cfg.Checks.HTTP {
		if cfg.Checks.HTTP[i].Method == "" {
			cfg.Checks.HTTP[i].Method = "GET"
		}
		cfg.Checks.HTTP[i].Method = strings.ToUpper(cfg.Checks.HTTP[i].Method)
		if cfg.Checks.HTTP[i].Timeout == 0 {
			cfg.Checks.HTTP[i].Timeout = Duration(5 * time.Second)
		}
		if len(cfg.Checks.HTTP[i].ExpectedStatus) == 0 {
			cfg.Checks.HTTP[i].ExpectedStatus = []int{200, 201, 202, 204, 301, 302, 304}
		}
	}

	for i := range cfg.Checks.TCP {
		if cfg.Checks.TCP[i].Timeout == 0 {
			cfg.Checks.TCP[i].Timeout = Duration(3 * time.Second)
		}
	}
}

func (cfg Config) Validate() error {
	if cfg.Daemon.Interval.Duration() < time.Second {
		return errors.New("daemon.interval must be at least 1s")
	}
	if cfg.Daemon.FailureThreshold < 1 {
		return errors.New("daemon.failure_threshold must be greater than 0")
	}
	if cfg.Daemon.SuccessThreshold < 1 {
		return errors.New("daemon.success_threshold must be greater than 0")
	}
	if cfg.Observability.ListenAddr != "" {
		if _, _, err := net.SplitHostPort(cfg.Observability.ListenAddr); err != nil {
			return fmt.Errorf("observability.listen_addr must look like host:port: %w", err)
		}
	}
	if len(cfg.Checks.HTTP)+len(cfg.Checks.TCP)+len(cfg.Checks.Disk) == 0 {
		return errors.New("at least one check is required")
	}

	seen := make(map[string]struct{})
	for _, check := range cfg.Checks.HTTP {
		if err := validateName("http", check.Name, seen); err != nil {
			return err
		}
		parsed, err := url.ParseRequestURI(check.URL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("http check %q has invalid url", check.Name)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fmt.Errorf("http check %q must use http or https", check.Name)
		}
		for _, status := range check.ExpectedStatus {
			if status < 100 || status > 599 {
				return fmt.Errorf("http check %q has invalid expected status %d", check.Name, status)
			}
		}
	}
	for _, check := range cfg.Checks.TCP {
		if err := validateName("tcp", check.Name, seen); err != nil {
			return err
		}
		if _, _, err := net.SplitHostPort(check.Address); err != nil {
			return fmt.Errorf("tcp check %q has invalid address: %w", check.Name, err)
		}
	}
	for _, check := range cfg.Checks.Disk {
		if err := validateName("disk", check.Name, seen); err != nil {
			return err
		}
		if strings.TrimSpace(check.Path) == "" {
			return fmt.Errorf("disk check %q has empty path", check.Name)
		}
		if check.MaxUsagePercent <= 0 || check.MaxUsagePercent > 100 {
			return fmt.Errorf("disk check %q max_usage_percent must be between 0 and 100", check.Name)
		}
	}
	return nil
}

func validateName(kind, name string, seen map[string]struct{}) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%s check has empty name", kind)
	}
	key := kind + ":" + name
	if _, ok := seen[key]; ok {
		return fmt.Errorf("%s check %q is duplicated", kind, name)
	}
	seen[key] = struct{}{}
	return nil
}
