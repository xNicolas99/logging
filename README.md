# ISP Peering & Performance Monitor

**Gather Irrefutable Evidence of ISP Peering Issues**

This tool is designed to monitor and log your internet connection's performance to specific targets over time. It is specifically built to detect and document peering congestion and packet loss issues, providing you with the data needed to confront your ISP.

## Why this exists

Many ISPs suffer from peering congestion during peak hours, leading to packet loss and high latency to specific destinations (like Google, Cloudflare, or gaming servers), even if standard "Speedtests" show full bandwidth. This tool runs 24/7, performing:
1.  **HTTP Checks:** Measures latency (TTFB) and throughput.
2.  **MTR (Traceroute):** Triggered automatically when issues are detected (high latency, packet loss, or slow speeds) to pinpoint exactly *where* the connection is failing (e.g., at the ISP's handover node).

The goal is to generate a history of measurements that proves:
- "It happens every evening at 8 PM."
- "It is not my local network, the packet loss starts at hop X (ISP node)."

## Quick Start (Docker)

The easiest way to run this is using Docker. No coding or Go knowledge required.

### 1. Run with Docker Compose

Create a `docker-compose.yml` file (or use the one provided) and run:

```bash
docker-compose up -d
```

This will start:
- **Monitor:** The tool itself (Port 8080).
- **InfluxDB:** A time-series database to store months of history (Port 8086).

### 2. Access the Dashboard

Open **[http://localhost:8080](http://localhost:8080)** in your browser.
You will see the status of your configured targets.

### 3. Configure Targets

You can add targets via the UI or by editing `data/config.json`.
Recommended targets to prove peering issues:
- **Google:** `https://www.google.com` (Good baseline)
- **Cloudflare:** `https://1.1.1.1`
- **A specific service you have issues with:** e.g., a game server or streaming CDN.

## How it works

1.  **Smart Monitoring:** It checks targets every minute (configurable).
2.  **Performance Optimization:** It first does a lightweight HTTP check.
3.  **Automatic Diagnostics:** If (and only if) performance is poor (High Latency > 1000ms or HTTP Error), it automatically runs a full **MTR (My Traceroute)** analysis.
    - This saves bandwidth while ensuring you capture detailed trace data exactly when the problem occurs.
4.  **Evidence Storage:** All data (latency, speed, packet loss % per hop) is stored in InfluxDB or a local JSON file.

## Data & Persistence

All data is stored in the `./data` directory (mapped in Docker).
- `data/config.json`: Your targets.
- `data/measurements.jsonl`: Backup log of measurements.
- InfluxDB data is stored in the docker volume `influxdb_data`.

## Generating a Report for your ISP

1.  Let the monitor run for at least 24-48 hours, covering peak times (evening).
2.  Open the Dashboard.
3.  Look for "ALERT" status or red spikes in latency.
4.  Click on a target to see the details.
5.  Take screenshots of:
    - The **Latency/Packet Loss Graph** showing the spikes.
    - The **MTR Trace Output** for a failed check. This shows the "Hop" where loss began.

---

### Development / Manual Run

If you want to build it from source:

```bash
go build -o monitor ./cmd/monitor
sudo ./monitor  # Root required for MTR
```

But Docker is highly recommended for long-term monitoring.
