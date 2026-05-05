# weather-exporter Copilot Instructions

## Copilot Rules
- **Never run `git commit`, `git push`, or any git command that writes to or modifies repository history or remotes.** If a task requires committing or pushing, stop and tell the user to run the git command manually.

## Philosophy: Grug-Brained Development

> "Complexity very, very bad." — [grugbrain.dev](https://grugbrain.dev/)

- **Say no.** The best weapon against complexity is the word "no". No new feature, no new abstraction, until it earns its place.
- **No abstraction until a pattern repeats three times.** Let cut points emerge naturally from the code; don't invent them up front.
- **80/20 solutions.** Ship 80% of the value with 20% of the code. Ugly but working beats elegant but over-engineered.
- **Chesterton's Fence.** Understand why code exists before removing it. If you don't see the use, go away and think.
- **Boring, obvious code wins.** Intermediate variables with good names beat clever one-liners. Easier to debug.
- **DRY is not a law.** A little copy-paste beats a complex abstraction built for two cases.
- **No FOLD** (Fear Of Looking Dumb). If something is too complex, say so. That's a signal to simplify, not a personal failing.

## Project Overview

A Prometheus exporter written in Go. Polls [Open-Meteo](https://open-meteo.com/) for local precipitation data and exposes it as a gauge metric. Designed to run as a Kubernetes workload via the homelab `XApi` platform primitive.

### Metric

```
# HELP weather_precipitation_mm Current hour precipitation in millimeters from Open-Meteo
# TYPE weather_precipitation_mm gauge
weather_precipitation_mm{latitude="40.5142",longitude="-88.9906"} 2.4
```

### Configuration (environment variables)

| Variable | Default | Description |
|---|---|---|
| `LATITUDE` | `40.5142` | Latitude for Open-Meteo query |
| `LONGITUDE` | `-88.9906` | Longitude for Open-Meteo query |
| `TIMEZONE` | `America/Chicago` | IANA timezone — used to match current hour in API response |
| `PORT` | `8080` | HTTP port for `/metrics` and `/healthz` |

### Endpoints

- `GET /metrics` — Prometheus metrics
- `GET /healthz` — Liveness/readiness probe, always 200

## Architecture

- Single `main.go` — no packages, no layers. It's a tiny exporter.
- Polls Open-Meteo every 10 minutes. The API response includes hourly forecasts; the exporter finds the current hour's value.
- Uses `time/tzdata` embedded timezone database — no OS timezone files needed in the distroless image.

## Build & Run

```bash
go build -o weather-exporter .
./weather-exporter
```

Docker (ARM64 for Raspberry Pi 5 nodes):
```bash
docker build --platform linux/arm64 -t weather-exporter .
```

## Deployment on the Homelab

This service is deployed via the homelab `XApi` Crossplane composition. See the [homelab repo](https://github.com/cujarrett/homelab) for the XApi instance manifest and ServiceMonitor.

### XApi — what it provisions
- **Deployment** — runs this container
- **Service** — ClusterIP on port 80 → container port 8080
- **Ingress** *(optional)* — Traefik `websecure` with cert-manager TLS

### XApi instance example (homelab `platform/xrs/api/`)

```yaml
apiVersion: platform.local.lab/v1alpha1
kind: XApi
metadata:
  name: weather-exporter
spec:
  parameters:
    namespace: weather-exporter
    image: ghcr.io/cujarrett/weather-exporter:latest
    port: 8080
    environment: test
```

### XSpa — what it provisions (for reference, not used here)
- **ConfigMap** — nginx config with SPA routing, security headers, asset caching
- **Deployment** — nginx serving a pre-built static SPA image
- **Service** — ClusterIP on port 80
- **Ingress** — Traefik `websecure` with cert-manager TLS

XSpa app repo contract: **do not add an `nginx.conf`** — the composition owns it entirely.

```yaml
apiVersion: platform.local.lab/v1alpha1
kind: XSpa
metadata:
  name: foo
spec:
  parameters:
    namespace: foo
    image: ghcr.io/owner/foo:sha-abc123
    host: foo.local.lab
```

## CI

GitHub Actions (`.github/workflows/ci.yml`) runs `go test ./...` and `go vet ./...` on every push and PR. On merge to `main` it builds and pushes `ghcr.io/cujarrett/weather-exporter:main` and `ghcr.io/cujarrett/weather-exporter:sha-<sha>`. Only `linux/arm64` — all homelab nodes are Raspberry Pi 5 (ARM64).
