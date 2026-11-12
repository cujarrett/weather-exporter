# weather-exporter

## Rules

- **Never run `git commit`, `git push`, or any git command that writes to or modifies repository history or remotes.** If a task requires committing or pushing, stop and tell the user to run the git command manually.
- **Never add a `Co-Authored-By` trailer or a "Generated with Claude Code" line** to commit messages or PR descriptions, including in suggested commit messages. Commits are authored by the user alone.

### Pre-commit safety check

Before telling the user to commit, always run `/security-review`. It reviews the pending changes on the current branch for security issues. Once it confirms the changes are safe, offer the user a suggested commit message - do not run `git commit` yourself.

## Philosophy: Grug-Brained Development

> "Complexity very, very bad." - [grugbrain.dev](https://grugbrain.dev/)

- **Say no.** The best weapon against complexity is the word "no". No new feature, no new abstraction, until it earns its place.
- **Cheapest rung that works.** Before writing code go down the ladder and stop at the first rung that solves it - skip the feature, reuse code already here, standard library, native platform feature, a dependency already installed, one line, then build the minimum.
- **No abstraction until a pattern repeats three times.** Let cut points emerge naturally from the code; don't invent them up front.
- **80/20 solutions.** Ship 80% of the value with 20% of the code. Ugly but working beats elegant but over-engineered.
- **Chesterton's Fence.** Understand why code exists before removing it. If you don't see the use, go away and think.
- **Boring, obvious code wins.** Intermediate variables with good names beat clever one-liners. Easier to debug.
- **DRY is not a law.** A little copy-paste beats a complex abstraction built for two cases.
- **No FOLD** (Fear Of Looking Dumb). If something is too complex, say so. That's a signal to simplify, not a personal failing.

## Project Overview

A Prometheus exporter written in Go. It polls [Open-Meteo](https://open-meteo.com/) for local precipitation and exposes it as a gauge. It runs as an `Api` on the homelab platform.

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
| `TIMEZONE` | `America/Chicago` | IANA timezone, used to match the current hour in the API response |
| `PORT` | `8080` | HTTP port for `/metrics` and `/healthz` |

### Endpoints

- `GET /metrics`: Prometheus metrics
- `GET /healthz`: liveness and readiness probe, always 200

## Architecture

- Single `main.go`, with no packages and no layers.
- Polls Open-Meteo every 10 minutes. The response holds hourly forecasts, and the exporter picks the current hour's value.
- Embeds the `time/tzdata` timezone database, so the distroless image needs no OS timezone files.

## Build & Run

`just ci` runs lint, test and build. `just run` starts it locally. `just --list` shows every recipe.

## Deployment

The `Api` lives at `sump-pump/weather-exporter.yaml` in [homelab-workspaces](https://github.com/cujarrett/homelab-workspaces). ArgoCD deploys from there.

## CI

`.github/workflows/ci.yml` has three jobs.

- `test`: `go test ./...` and `go vet ./...` on every push and PR.
- `build-and-push`: on `main`, builds `linux/arm64` and pushes `ghcr.io/cujarrett/weather-exporter:main` and `:sha-<sha>`.
- `deploy`: commits the new tag to `homelab-workspaces`.
