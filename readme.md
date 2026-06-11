# metrics-agent

A lightweight system metrics collection agent written in Go. Scrapes CPU, memory, disk, load average, and process count at a configurable interval and exposes them in Prometheus exposition format.

Designed to run as a Kubernetes DaemonSet — one instance per node, automatically scheduled by K8s as nodes join or leave the cluster.

---

## Endpoints

| Endpoint    | Format          | Description                              |
|-------------|-----------------|------------------------------------------|
| `/metrics`  | Prometheus text | Scraped by Prometheus or any compatible collector |
| `/snapshot` | JSON            | Same data, human-readable                |
| `/health`   | JSON            | Liveness check — hostname, env, timestamp |

---

## Metrics collected

| Metric                        | Type    | Description                        |
|-------------------------------|---------|------------------------------------|
| `agent_cpu_percent`           | gauge   | CPU usage percentage               |
| `agent_memory_total_bytes`    | gauge   | Total system memory                |
| `agent_memory_used_bytes`     | gauge   | Used memory                        |
| `agent_memory_used_percent`   | gauge   | Memory usage percentage            |
| `agent_disk_used_bytes`       | gauge   | Disk used per mount point          |
| `agent_disk_used_percent`     | gauge   | Disk usage percentage per mount    |
| `agent_load_avg_1m`           | gauge   | 1-minute load average              |
| `agent_load_avg_5m`           | gauge   | 5-minute load average              |
| `agent_load_avg_15m`          | gauge   | 15-minute load average             |
| `agent_process_count`         | gauge   | Number of running processes        |
| `agent_uptime_seconds`        | counter | Agent uptime                       |

Every metric carries `hostname` and `env` labels so metrics from different nodes are distinguishable in Prometheus queries.

---

## Sample output

```
# HELP agent_cpu_percent Current CPU usage percentage
# TYPE agent_cpu_percent gauge
agent_cpu_percent{hostname="node-1",env="production"} 12.34

# HELP agent_memory_used_percent Memory usage percentage
# TYPE agent_memory_used_percent gauge
agent_memory_used_percent{hostname="node-1",env="production"} 67.80

# HELP agent_disk_used_percent Disk usage percentage per mount
# TYPE agent_disk_used_percent gauge
agent_disk_used_percent{hostname="node-1",env="production",mount="/"} 43.10
```

---

## Running locally

```bash
go run main.go
```

Default port is `9100` — the conventional port for node exporters in the Prometheus ecosystem.

```bash
curl http://localhost:9100/metrics
curl http://localhost:9100/snapshot
curl http://localhost:9100/health
```

---

## Configuration

All configuration is via environment variables.

| Variable                  | Default         | Description                        |
|---------------------------|-----------------|------------------------------------|
| `PORT`                    | `9100`          | HTTP port to listen on             |
| `SCRAPE_INTERVAL_SECONDS` | `10`            | How often to scrape system metrics |
| `ENV`                     | `development`   | Environment label on every metric  |

---

## Docker

```bash
docker build -t metrics-agent .
docker run -p 9100:9100 metrics-agent
```

---

## Kubernetes

The `k8s/` directory contains three manifests.

### Why DaemonSet and not Deployment

A Deployment runs N replicas scheduled across available nodes. A DaemonSet runs exactly one pod per node — always. When a new node joins the cluster, K8s automatically schedules the agent on it. When a node is removed, the pod is cleaned up.

Node-level metrics require node-level presence. A Deployment with 3 replicas on a 10-node cluster leaves 7 nodes unmonitored. A DaemonSet has no such gap.

### Deploy

```bash
kubectl create namespace monitoring
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/daemonset.yaml
kubectl apply -f k8s/service.yaml
```

### Prometheus autodiscovery

The Service manifest includes standard Prometheus annotations:

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "9100"
  prometheus.io/path: "/metrics"
```

Prometheus operators watching the cluster will automatically add the agent as a scrape target — no manual configuration required.

### Resource footprint

The agent is intentionally lightweight. Limits defined in the DaemonSet:

```
CPU:    50m request / 100m limit
Memory: 32Mi request / 64Mi limit
```

---

## Project structure

```
.
├── main.go                   entry point, HTTP server, config
├── internal/
│   ├── collector/            system metrics scraping via gopsutil
│   └── exporter/             Prometheus formatter, JSON handler, health
├── k8s/
│   ├── configmap.yaml        environment config
│   ├── daemonset.yaml        one agent per node
│   └── service.yaml          headless service with Prometheus annotations
└── Dockerfile
```

---

## Stack

- Go 1.26
- [gopsutil](https://github.com/shirou/gopsutil) — cross-platform system metrics
- Standard library for HTTP
- Prometheus exposition format — no client library, written directly