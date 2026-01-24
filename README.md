# HTTP Speed Monitor

A lightweight, compiled Go application to monitor HTTP download speeds and latency of configurable targets. It supports storing data in **InfluxDB 2.0** (recommended) or a local JSONL file. It includes a web interface for visualization.

## Features

- **Automated Monitoring:** Checks download speed and latency at defined intervals.
- **Alerting:** Marks measurements as ALERT if speed is below a threshold.
- **Dual Storage:**
    - InfluxDB 2.0 (Primary, highly recommended for visualizations).
    - JSONL File (Fallback/Simple mode).
- **Web Interface:** Embedded dashboard with status overview, detailed tables, and sparkline charts.
- **Logging:** text-based logs in `logs/measurements.log`.
- **Docker Ready:** Includes Dockerfile and docker-compose.yml with auto-configured InfluxDB.

## Getting Started

### Quick Start with Docker (Recommended)

This sets up the Monitor and InfluxDB automatically.

1.  Clone the repository.
2.  Run:
    ```bash
    docker-compose up -d
    ```
3.  Open [http://localhost:8080](http://localhost:8080).

*Note: The `docker-compose.yml` automatically provisions a bucket named `monitor` and an auth token `my-super-secret-auth-token`. Change these in production!*

### Local Run (Without Docker)

1.  **Configure:** Edit `config.json`.
    *   Set targets.
    *   If using InfluxDB, fill in `influx` details. If left empty, it falls back to `data/measurements.jsonl`.
2.  **Build & Run:**
    ```bash
    go mod download
    go build -o monitor ./cmd/monitor
    ./monitor
    ```
3.  Open [http://localhost:8080](http://localhost:8080).

## Configuration (`config.json`)

```json
{
    "interval": 60,                // Measurement interval in seconds
    "targets": [
        {
            "name": "GitHub",
            "url": "https://github.com/",
            "threshold": 102400    // Minimum bytes/sec before ALERT
        }
    ],
    "influx": {                    // Optional. Remove for local JSONL mode.
        "url": "http://localhost:8086",
        "token": "my-token",
        "org": "my-org",
        "bucket": "monitor"
    }
}
```

## Logs & Data Export

- **Logs:** Written to `logs/measurements.log`. Viewable via web at `/logs`.
- **Data (InfluxDB):** Stored in InfluxDB volume. Query using Data Explorer in InfluxDB UI (http://localhost:8086).
- **Data (JSONL):** Stored in `data/measurements.jsonl` (if InfluxDB not used).

## Project Structure

- `cmd/monitor`: Main application entry point.
- `internal/collector`: Logic for downloading and measuring speed.
- `internal/storage`: Abstraction for InfluxDB and JSONL storage.
- `internal/server`: HTTP server and API implementation.
- `internal/server/static`: Embedded frontend assets.

## CI/CD

A GitHub Actions workflow is included (`.github/workflows/docker-build.yml`) to automatically build and push the Docker image to GHCR on commits to `main` or tags.
