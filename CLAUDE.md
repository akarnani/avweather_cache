# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Aviation Weather Cache Service - A Go service that caches METAR data from aviationweather.gov and provides a REST API for querying the data with multiple output formats (JSON, CSV, YAML).

## Key Design Decisions

1. **Cache Merging, Not Purging**: When updating from aviationweather.gov, new data is merged into the existing cache rather than replacing it. This ensures stale data remains available if the source is incomplete. Stale data is better than no data.

2. **Age Metrics vs Timestamps**: Prometheus metrics use age in seconds rather than Unix timestamps for better observability and alerting.

3. **Silent Omissions**: Stations that don't meet age filters or aren't cached are omitted from responses without errors. Metrics are emitted instead for observability.

4. **Configurable Duration**: Update intervals use Go's time.Duration format (e.g., "5m", "1h30m") for clarity.

## Important steps

* Anytime metrics are changed the grafana dashboard in deploy/ should be updated
* GitHub Actions in workflow files MUST be pinned to a full 40-character commit SHA, never a tag or branch. Format: `uses: owner/repo@<sha>  # v1.2.3`. Tags are mutable and can be force-moved by the action's owner; SHAs are immutable. Resolve a tag to its SHA with `gh api repos/<owner>/<repo>/git/refs/tags/<tag>` (dereference annotated tags via `git/tags/<sha>` if `.object.type` is `tag`).

## Development Setup

```bash
# Install dependencies
go mod download

# Run tests
go test ./...

# Build and run
go build -o avweather_cache
./avweather_cache
```

## Architecture

### Package Structure
- `api/` - REST API handlers for /api/metar endpoint
- `cache/` - In-memory cache with periodic update logic from aviationweather.gov
- `config/` - Configuration loading (YAML + env vars)
- `metrics/` - Prometheus metrics definitions
- `models/` - METAR data structures
- `webapp/` - Simple web dashboard for visualization

### Data Flow
1. Cache starts and immediately pulls gzipped XML from aviationweather.gov
2. XML is decompressed, parsed, and merged into in-memory map[station_id]METAR
3. Periodic updates continue every N minutes (configurable)
4. API requests query the cache with optional field/age filtering
5. Metrics are updated on each pull and query

### Testing
Tests cover complex logic: cache merging, XML parsing, field filtering, CSV generation, and age filtering. Real data file stored in testdata/ for integration tests.
