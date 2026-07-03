# thirdcupd

`thirdcupd` is a lightweight Go daemon for backend developers who prefer clarity over guesswork when debugging.

It monitors HTTP endpoints, TCP ports, and disk usage, writing events in JSONL format and exposing `/healthz`, `/status`, and Prometheus-compatible `/metrics`. The core idea: even after the third cup of coffee, you should still understand exactly what broke, when, and why.

```text
┌──────────────┐      ┌──────────────┐      ┌──────────────┐
│ HTTP checks  │ ---> │  thirdcupd   │ ---> │ JSONL events │
│ TCP checks   │      │  watchdog    │      │ /metrics     │
│ Disk checks  │      │              │      │ /status      │
└──────────────┘      └──────────────┘      └──────────────┘
```

## Features

- **HTTP checks**: status codes, latency, timeout, headers.
- **TCP checks**: reachability of PostgreSQL, Redis, API gateways, or any port.
- **Disk checks**: usage percentage for specified mount points.
- **Failure thresholds**: `failure_threshold` prevents flapping due to transient network issues.
- **Structured logging**: writes clear JSONL events to stdout and file.
- **Health API**: exposes `/healthz`, `/readyz`, `/status`, and `/metrics`.
- **Flexible runtime**: works with systemd, Docker, or as a one-off smoke test with `-once`.

## Quick Start

```bash
make test
make build
./bin/thirdcupd -config configs/thirdcupd.example.json
```

Check the status:

```bash
curl http://127.0.0.1:8374/healthz
curl http://127.0.0.1:8374/status
curl http://127.0.0.1:8374/metrics
```

One-off run for CI or deployment hooks:

```bash
./bin/thirdcupd -config configs/thirdcupd.example.json -once
```

Exit codes:

| Code | Meaning |
| --- | --- |
| `0` | all checks healthy |
| `1` | configuration error or runtime error |
| `2` | at least one check unhealthy in `-once` mode |

## Configuration

A minimal example is available in `configs/thirdcupd.example.json`.

```json
{
  "daemon": {
    "interval": "30s",
    "failure_threshold": 2,
    "success_threshold": 1
  },
  "observability": {
    "listen_addr": "127.0.0.1:8374",
    "event_log": "./thirdcupd.events.jsonl",
    "pretty_logs": false
  },
  "checks": {
    "disk": [
      {
        "name": "root-disk",
        "path": "/",
        "max_usage_percent": 90
      }
    ]
  }
}
```

Add an HTTP check:

```json
{
  "name": "auth-gateway",
  "url": "http://127.0.0.1:8080/healthz",
  "method": "GET",
  "timeout": "3s",
  "expected_status": [200],
  "max_latency": "500ms",
  "headers": {
    "User-Agent": "thirdcupd"
  }
}
```

Add a TCP check:

```json
{
  "name": "postgres",
  "address": "127.0.0.1:5432",
  "timeout": "2s"
}
```

The `checks` field can contain multiple groups:

```json
{
  "checks": {
    "http": [],
    "tcp": [],
    "disk": []
  }
}
```

## API Endpoints

| Endpoint | Response |
| --- | --- |
| `/healthz` | `200 ok` if all checks are healthy, otherwise `503` |
| `/readyz` | same behavior, useful for Kubernetes probes |
| `/status` | full JSON snapshot of current state |
| `/metrics` | Prometheus text format metrics |

Example event:

```json
{
  "time": "2026-07-03T16:45:00Z",
  "name": "root-disk",
  "type": "disk",
  "ok": true,
  "status": "healthy",
  "transition": true,
  "message": "disk usage 64.21%",
  "latency_ms": 0
}
```

## systemd

Build and install the binary:

```bash
make build
sudo install -m 0755 bin/thirdcupd /usr/local/bin/thirdcupd
sudo mkdir -p /etc/thirdcupd
sudo cp configs/thirdcupd.example.json /etc/thirdcupd/thirdcupd.json
```

Create a dedicated user and enable the service:

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin thirdcupd
sudo cp systemd/thirdcupd.service /etc/systemd/system/thirdcupd.service
sudo systemctl daemon-reload
sudo systemctl enable --now thirdcupd
```

Verify:

```bash
systemctl status thirdcupd
journalctl -u thirdcupd -f
```

## Docker

```bash
docker build -t thirdcupd .
docker run --rm \
  -p 8374:8374 \
  -v "$PWD/configs/thirdcupd.example.json:/etc/thirdcupd/thirdcupd.json:ro" \
  thirdcupd -config /etc/thirdcupd/thirdcupd.json
```

To check host services from within the container, use addresses accessible from the Docker network.

## Project Structure

```text
cmd/thirdcupd         entrypoint
internal/config       configuration loading and validation
internal/checks       HTTP, TCP, and disk check implementations
internal/state        state management, thresholds, transitions
internal/events       JSONL event logging
internal/server       health/status/metrics HTTP API
configs               example configuration files
systemd               systemd unit file for Linux
```

## Philosophy

`thirdcupd` is not meant to replace Prometheus, Grafana, or a full-fledged incident management platform. It's a small daemon for projects that need a simple, predictable watchdog right next to their backend service:

- minimal dependencies;
- meaningful errors;
- readable code;
- predictable startup;
- logs and metrics out of the box.

For minimalism, but not emptiness.
```