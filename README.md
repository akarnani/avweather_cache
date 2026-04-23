# Aviation Weather Cache Service

A high-performance Go service that caches METAR (Meteorological Aerodrome Report) data from aviationweather.gov and provides a simple REST API for querying the data.

## Features

- **In-Memory Cache**: Fast access to METAR data for thousands of stations
- **Automatic Updates**: Configurable periodic updates from aviationweather.gov (default: 5 minutes)
- **Smart Merging**: New data is merged into cache without purging (stale data is better than no data)
- **Flexible API**: Query by station(s) with optional field filtering and age filtering
- **Multiple Formats**: JSON, CSV, and YAML output formats
- **Prometheus Metrics**: Comprehensive observability metrics
- **Web Dashboard**: Simple web UI for visualizing cache status and data

## Quick Start

### Build

```bash
go build -o avweather_cache
```

### Run

```bash
# Using default configuration (config.yaml)
./avweather_cache

# Using custom configuration file
./avweather_cache -config /path/to/config.yaml

# Using environment variables
export SERVER_PORT=9090
export CACHE_UPDATE_INTERVAL=10m
./avweather_cache
```

### Configuration

Configuration can be provided via YAML file or environment variables (env vars take precedence).

**config.yaml:**
```yaml
server:
  port: 8080

cache:
  # Update interval (e.g., "5m", "1h30m", "300s")
  update_interval: "5m"
  # URL to fetch METAR data from
  source_url: "https://aviationweather.gov/data/cache/metars.cache.xml.gz"
```

**Environment Variables:**
- `SERVER_PORT`: Server port (default: 8080)
- `CACHE_UPDATE_INTERVAL`: Cache update interval as duration string (default: "5m")
- `CACHE_SOURCE_URL`: Source URL for METAR data

## API Usage

### GET /api/metar

Query METAR data for one or more stations.

**Parameters:**

- `stations` (required): Comma-separated list of station IDs (e.g., "KJFK,KLAX,KORD")
- `fields` (optional): Comma-separated list of fields to return. If omitted, all fields are returned.
  - Available fields: `station_id`, `raw_text`, `observation_time`, `latitude`, `longitude`, `temp_c`, `dewpoint_c`, `wind_dir_degrees`, `wind_speed_kt`, `wind_gust_kt`, `visibility_statute_mi`, `altim_in_hg`, `wx_string`, `flight_category`, `metar_type`, `elevation_m`, `precip_in`
- `hoursBeforeNow` (optional): Only return METARs newer than this many hours (default: 0 = no filter)
- `format` (optional): Output format: `json`, `csv`, or `yaml` (default: `json`)

**Examples:**

```bash
# Get all fields for multiple stations in JSON
curl "http://localhost:8080/api/metar?stations=KJFK,KLAX"

# Get specific fields in CSV format
curl "http://localhost:8080/api/metar?stations=KJFK,KLAX&fields=station_id,temp_c,flight_category&format=csv"

# Get METARs from last 2 hours only
curl "http://localhost:8080/api/metar?stations=KJFK,KLAX,KORD&hoursBeforeNow=2"

# YAML output
curl "http://localhost:8080/api/metar?stations=KJFK&format=yaml"
```

**Response:**

Stations that don't meet the age filter or aren't cached are silently omitted (metrics are emitted instead).

### GET /api/metar/nearest

Find the nearest station to a lat/lon point that has a recent METAR within a given radius.

**Parameters (all required except `format`):**

- `lat`: Query latitude, decimal degrees (-90 to 90)
- `lon`: Query longitude, decimal degrees (-180 to 180)
- `max_range_mi`: Maximum search radius in statute miles (> 0)
- `max_age`: Maximum METAR age as a Go duration (e.g. `30m`, `1h30m`, `2h`)
- `format` (optional): `json` (default) or `yaml`

**Behavior:**

Scans cached stations, filters by age and distance, returns the closest match with a computed `distance_mi` field. If no station meets both criteria, returns **204 No Content** with an empty body.

**Examples:**

```bash
# Nearest VFR-reporting station within 50mi of downtown Philadelphia, no older than 1h
curl "http://localhost:8080/api/metar/nearest?lat=39.95&lon=-75.17&max_range_mi=50&max_age=1h"

# Same query in YAML
curl "http://localhost:8080/api/metar/nearest?lat=39.95&lon=-75.17&max_range_mi=50&max_age=1h&format=yaml"
```

**Response (200):**

```json
{
  "station_id": "KPHL",
  "latitude": 39.8722,
  "longitude": -75.2408,
  "observation_time": "2026-04-22T12:53:00Z",
  "temp_c": 18.3,
  "flight_category": "VFR",
  "...": "...",
  "distance_mi": 5.7
}
```

## Web Dashboard

Visit `http://localhost:8080/` for a web dashboard showing:
- System status (last pull time, errors, total stations)
- Recent METARs (top 100, sorted by observation time)
- Auto-refreshes every 30 seconds

## Metrics

Prometheus metrics are available at `http://localhost:8080/metrics`

**Data Pull Metrics:**
- `avweather_last_successful_pull_age_seconds`: Age of last successful data pull
- `avweather_last_pull_attempt_age_seconds`: Age of last pull attempt
- `avweather_pull_errors_total`: Total pull errors
- `avweather_oldest_metar_age_seconds`: Age of oldest METAR in cache
- `avweather_total_stations`: Total stations in cache
- `avweather_stations_under_1hour`: Stations with METAR < 1hr old
- `avweather_stations_under_2hours`: Stations with METAR < 2hrs old

**Query Metrics:**
- `avweather_query_latency_seconds`: API query latency (histogram with p50, p90, p99)
- `avweather_queries_total`: Total number of queries
- `avweather_queries_by_station_total`: Query count per station
- `avweather_stations_filtered_by_age_total`: Stations filtered due to age
- `avweather_stations_not_cached_total`: Stations queried but not in cache
- `avweather_nearest_queries_total`: Total `/api/metar/nearest` queries
- `avweather_nearest_no_match_total`: Nearest queries returning 204 (no match)
- `avweather_nearest_distance_mi`: Histogram of matched distances (statute miles)

## Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## Development

### Project Structure

```
.
├── api/            # REST API handlers
├── cache/          # In-memory cache with update logic
├── config/         # Configuration loading
├── metrics/        # Prometheus metrics definitions
├── models/         # Data models (METAR structs)
├── webapp/         # Web dashboard
├── testdata/       # Test data files
├── main.go         # Application entry point
└── config.yaml     # Default configuration
```

### Dependencies

- `github.com/prometheus/client_golang` - Prometheus metrics
- `gopkg.in/yaml.v3` - YAML parsing

## Deployment

### Docker

#### Using Pre-built Image

```bash
# Pull from Docker Hub
docker pull akarnani/avweather-cache:latest

# Run container
docker run -d \
  -p 8080:8080 \
  -e CACHE_UPDATE_INTERVAL=5m \
  --name avweather-cache \
  akarnani/avweather-cache:latest

# Check logs
docker logs -f avweather-cache

# Access the service
curl http://localhost:8080/api/metar?stations=KJFK
curl http://localhost:8080/metrics
```

#### Building Locally

```bash
# Build image
docker build -t avweather-cache:local .

# Run
docker run -d -p 8080:8080 avweather-cache:local
```

**Multi-architecture Support:**
Pre-built images support both `linux/amd64` and `linux/arm64` platforms.

### Native Binaries

Download pre-compiled binaries from [GitHub Releases](https://github.com/akarnani/avweather_cache/releases):

**macOS (Apple Silicon):**
```bash
curl -LO https://github.com/akarnani/avweather_cache/releases/latest/download/avweather_cache-darwin-arm64
chmod +x avweather_cache-darwin-arm64
./avweather_cache-darwin-arm64
```

**macOS (Intel):**
```bash
curl -LO https://github.com/akarnani/avweather_cache/releases/latest/download/avweather_cache-darwin-amd64
chmod +x avweather_cache-darwin-amd64
./avweather_cache-darwin-amd64
```

**Linux (ARM64):**
```bash
curl -LO https://github.com/akarnani/avweather_cache/releases/latest/download/avweather_cache-linux-arm64
chmod +x avweather_cache-linux-arm64
./avweather_cache-linux-arm64
```

**Linux (x86_64):**
```bash
curl -LO https://github.com/akarnani/avweather_cache/releases/latest/download/avweather_cache-linux-amd64
chmod +x avweather_cache-linux-amd64
./avweather_cache-linux-amd64
```

**Verify checksums:**
```bash
curl -LO https://github.com/akarnani/avweather_cache/releases/latest/download/avweather_cache-darwin-arm64.sha256
sha256sum -c avweather_cache-darwin-arm64.sha256
```

### Kubernetes / Helm

#### Prerequisites

- Kubernetes 1.19+
- Helm 3.2.0+
- (Optional) Prometheus Operator for ServiceMonitor support

#### Install from Helm Repository

```bash
# Add Helm repository
helm repo add avweather-cache https://akarnani.github.io/avweather_cache
helm repo update

# Install with default settings
helm install my-cache avweather-cache/avweather-cache

# Install with custom configuration
helm install my-cache avweather-cache/avweather-cache \
  --set config.cache.updateInterval=10m \
  --set resources.limits.memory=1Gi \
  --set metrics.serviceMonitor.enabled=true
```

#### Install from Local Chart

```bash
# From repository root
helm install my-cache ./deploy/charts/avweather-cache
```

#### Key Helm Configuration Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.cache.updateInterval` | Cache refresh interval (Go duration) | `"5m"` |
| `config.cache.sourceUrl` | METAR data source URL | aviationweather.gov URL |
| `resources.limits.memory` | Memory limit | `512Mi` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `metrics.serviceMonitor.enabled` | Create Prometheus ServiceMonitor | `true` |

See [Helm chart README](deploy/charts/avweather-cache/README.md) for full configuration options.

#### Access the Service

```bash
# Port-forward to local machine
kubectl port-forward svc/my-cache-avweather-cache 8080:8080

# Check logs
kubectl logs -f deployment/my-cache-avweather-cache

# Verify metrics
curl http://localhost:8080/metrics | grep avweather
```

#### Uninstall

```bash
helm uninstall my-cache
```

## CI/CD and Releases

This project uses GitHub Actions for continuous integration and automated releases.

### GitHub Secrets Required

Configure the following secrets in your GitHub repository (Settings → Secrets and variables → Actions):

1. **akarnani**: Your Docker Hub username
2. **DOCKERHUB_TOKEN**: Docker Hub access token
   - Create at: Docker Hub → Account Settings → Security → New Access Token
   - Required scopes: Read, Write, Delete

### Release Process

Releases are triggered automatically when you push a semantic version tag:

```bash
# Tag the release
git tag v1.0.0
git push origin v1.0.0
```

This triggers three workflows that run in parallel:

1. **Docker Workflow** (`docker.yml`):
   - Builds multi-arch images: `linux/amd64`, `linux/arm64`
   - Pushes to Docker Hub with tags: `v1.0.0`, `v1.0`, `v1`, `latest`

2. **Release Workflow** (`release.yml`):
   - Builds native binaries for all platforms (darwin/amd64, darwin/arm64, linux/amd64, linux/arm64)
   - Generates SHA256 checksums
   - Creates GitHub Release with binaries attached

3. **Helm Release Workflow** (`helm-release.yml`):
   - Packages Helm chart
   - Publishes to GitHub Pages at `https://akarnani.github.io/avweather_cache`
   - Updates Helm repository index

### Continuous Integration

On every push and pull request, the CI workflow (`ci.yml`) runs:
- Tests with Go 1.22 and 1.23
- Race condition detection
- Code coverage reporting (Codecov)
- Linting with golangci-lint

## License

This project is provided as-is for use with aviationweather.gov data.
