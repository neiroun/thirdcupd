# Changelog

## 0.1.0 - 2026-07-05

- Added HTTP checks with expected status codes, request headers, expected response headers, timeout, and max latency.
- Added TCP reachability checks for arbitrary host:port targets.
- Added disk usage checks for configured paths and mount points.
- Added failure and recovery thresholds to reduce noisy status transitions.
- Added JSONL event logging to stdout and optional files.
- Added `/healthz`, `/readyz`, `/status`, and Prometheus-compatible `/metrics`.
- Added one-off smoke test mode with `-once`, Docker runtime support, and a systemd unit.
