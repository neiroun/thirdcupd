# thirdcupd

[![CI](https://github.com/neiroun/thirdcupd/actions/workflows/ci.yml/badge.svg)](https://github.com/neiroun/thirdcupd/actions/workflows/ci.yml)

`thirdcupd` is a small Go watchdog for backend services. It runs HTTP, TCP, and disk checks, writes JSONL events, and exposes health/status/metrics endpoints for systemd, Docker, Kubernetes-style probes, CI smoke tests, and simple local operations.

Current release: **v0.1.0**.

## Features

- **HTTP checks**: expected status codes, request headers, expected response headers, timeout, and max latency.
- **TCP checks**: reachability of PostgreSQL, Redis, API gateways, or any `host:port`.
- **Disk checks**: usage percentage for specified paths and mount points.
- **Failure thresholds**: `failure_threshold` avoids probe flapping on transient failures; `success_threshold` avoids premature recovery.
- **Structured logging**: JSONL events to stdout and, optionally, an append-only file.
- **Health API**: `/healthz`, `/readyz`, `/status`, and Prometheus-compatible `/metrics`.
- **Flexible runtime**: systemd service, Docker image, or one-off smoke test with `-once`.

## Quick Start

```bash
make test
make build
./bin/thirdcupd -config configs/thirdcupd.example.json
```

In another terminal:

```bash
curl http://127.0.0.1:8374/healthz
curl http://127.0.0.1:8374/status
curl http://127.0.0.1:8374/metrics
```

One-off smoke test mode:

```bash
./bin/thirdcupd -config configs/thirdcupd.example.json -once
```

Exit codes:

| Code | Meaning |
| --- | --- |
| `0` | all checks are healthy |
| `1` | configuration or runtime error |
| `2` | at least one check failed in `-once` mode |

## Configuration

The smoke-safe default example is `configs/thirdcupd.example.json`. A fuller reference with HTTP, TCP, disk, file logging, and response-header checks is available at `configs/thirdcupd.full.example.json`.

```json
{
  "daemon": {
    "interval": "30s",
    "failure_threshold": 2,
    "success_threshold": 1
  },
  "observability": {
    "listen_addr": "127.0.0.1:8374",
    "event_log": ""
  },
  "checks": {
    "http": [
      {
        "name": "auth-gateway",
        "url": "http://127.0.0.1:8080/healthz",
        "method": "GET",
        "headers": {
          "User-Agent": "thirdcupd"
        },
        "expected_headers": {
          "X-Service-Health": "green"
        },
        "timeout": "3s",
        "expected_status": [200],
        "max_latency": "500ms"
      }
    ],
    "tcp": [
      {
        "name": "postgres",
        "address": "127.0.0.1:5432",
        "timeout": "2s"
      }
    ],
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

## Health API

| Endpoint | Response |
| --- | --- |
| `/healthz` | `200` for `healthy` or pre-threshold `degraded`; `503` for `unknown`, `recovering`, or `unhealthy` |
| `/readyz` | same behavior as `/healthz` |
| `/status` | full JSON snapshot with check state, counters, transitions, and details |
| `/metrics` | Prometheus text format metrics |

Status model:

- `degraded`: a failure was observed, but `failure_threshold` has not been reached yet.
- `unhealthy`: consecutive failures reached `failure_threshold`.
- `recovering`: checks have started passing after an unhealthy state, but `success_threshold` has not been reached yet.

Example JSONL event:

```json
{"time":"2026-07-05T08:00:00Z","name":"root-disk","type":"disk","ok":true,"status":"healthy","previous_status":"unknown","transition":true,"message":"disk usage 64.21%","latency_ms":0}
```

## Docker

The image includes a default container-safe config at `/etc/thirdcupd/thirdcupd.json`.

```bash
docker build -t thirdcupd:0.1.0 .
docker run --rm -p 8374:8374 thirdcupd:0.1.0
```

Use a custom config when checking host or compose-network services:

```bash
docker run --rm \
  -p 8374:8374 \
  -v "$PWD/configs/thirdcupd.docker.json:/etc/thirdcupd/thirdcupd.json:ro" \
  thirdcupd:0.1.0
```

## systemd

Build and install:

```bash
make build
sudo install -m 0755 bin/thirdcupd /usr/local/bin/thirdcupd
sudo useradd --system --no-create-home --shell /usr/sbin/nologin thirdcupd
sudo mkdir -p /etc/thirdcupd
sudo cp configs/thirdcupd.example.json /etc/thirdcupd/thirdcupd.json
```

For file logging under systemd, set:

```json
"event_log": "/var/log/thirdcupd/events.jsonl"
```

Enable the service:

```bash
sudo cp systemd/thirdcupd.service /etc/systemd/system/thirdcupd.service
sudo systemctl daemon-reload
sudo systemctl enable --now thirdcupd
```

Verify:

```bash
systemctl status thirdcupd
journalctl -u thirdcupd -f
```

## Development

```bash
make fmt
make test
make build
```

Project layout:

```text
cmd/thirdcupd         CLI entrypoint
internal/config       config loading, defaults, validation
internal/checks       HTTP, TCP, and disk checks
internal/state        thresholds, transitions, snapshots
internal/events       JSONL event logging
internal/server       health, status, and metrics API
configs               runtime examples
systemd               Linux service unit
```

`thirdcupd` is intentionally small: no scheduler dependency, no database, and no external services required to run.
