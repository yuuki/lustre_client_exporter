# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

Prometheus exporter for Lustre **client-side** metrics. Reads procfs/sysfs/debugfs and optionally `lnetctl` to expose health, performance, and configuration as Prometheus metrics. Metric names and labels are compatible with [GSI-HPC/lustre_exporter](https://github.com/GSI-HPC/lustre_exporter) — but this project is client-only and the implementation is original.

## Build & Test

```sh
make build          # go build ./...
make test           # go test ./...
make test-race      # go test -race ./...
make vet            # go vet ./...
go test ./internal/parser/...   # run tests for a single package
go test -run TestParseLLiteStats ./internal/parser/  # run a single test
```

Go version is managed via `mise.toml` (currently 1.26.2).

## Architecture

The data pipeline follows a strict layered flow:

```
Discovery → Reader → Parser → Mapper → Emitter → Prometheus
```

- **`internal/discovery/`** — Enumerates target files/paths for each collector (llite/mdc/osc globs, health_check path, LNet param files). Also defines `PathConfig` for procfs/sysfs/debugfs base paths.
- **`internal/reader/`** — Abstracts filesystem reads and command execution (`Reader` interface). `FakeReader` enables fixture-based testing without touching the real filesystem.
- **`internal/parser/`** — Parses raw file content into `Observation` structs (metric ID, type, labels, value). No Prometheus dependency. Each Lustre subsystem has its own parser (health, sptlrpc, lnet, llite, rpc).
- **`internal/mapper/`** — Converts internal `Observation`s to `MappedObservation`s using `contract.go`'s `Registry` map. This is the **single source of truth** for public metric names, types, and label keys (the GSI-HPC compatibility layer).
- **`internal/emitter/`** — Converts `MappedObservation`s to `prometheus.Metric` values. Caches `prometheus.Desc` objects.
- **`collector/`** — Orchestrates the pipeline per subsystem (health, sptlrpc, lnet, client). Implements `Collector` interface. `Registry` in `collector.go` runs collectors concurrently with timeout support.
- **`cmd/lustre_client_exporter/`** — CLI entry point. Flag parsing, logger setup, HTTP server.

## Key Design Decisions

- **Metric contract lives in `internal/mapper/contract.go`** — all metric names, help strings, types, and label keys are defined in the `Registry` map. When adding or changing metrics, modify this file.
- **Parsers are Prometheus-agnostic** — they return `parser.Observation` structs, not Prometheus types. This keeps parsing testable independently.
- **Tests use `FakeReader`** — inject fixture data via `reader.FakeReader` rather than reading real procfs. Test fixtures live in `testdata/`.
- **Client component label is always `"client"`** — unified for GSI-HPC compatibility, regardless of whether the source is llite, mdc, or osc.
- **Server-side metrics are explicitly out of scope** — no OST, MDT, MDS, MGS, quota, recovery, exports, changelog, jobstats.

## Commit Style

Short, scoped subjects: `collector: ...`, `parser: ...`, `build: ...`, `docs: ...`.
