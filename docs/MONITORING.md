# PubSubGo Monitoring Guide

## 📊 Overview

PubSubGo includes comprehensive monitoring and observability features with Prometheus, Grafana, Jaeger, and optional Istio service mesh integration.

## 🚀 Quick Start

### 1. Start Monitoring Stack

```bash
# Start all monitoring services
make monitoring-up

# Check status
make monitoring-status
```

### 2. Start PubSubGo Server

```bash
# Build and run the server
make build-server
./bin/pubsubgo-server
```

### 3. Generate Traffic

```bash
# Run the traffic generation script
./scripts/generate-traffic.sh
```

## 📈 Accessing Monitoring Tools

### Grafana - Metrics Visualization
- **URL**: http://localhost:3000
- **Login**: admin / admin123
- **Features**:
  - Pre-configured dashboards for PubSubGo metrics
  - Real-time metric visualization
  - Alert configuration

#### Key Dashboards:
1. **PubSubGo Overview**: Overall system health and performance
2. **Message Flow**: Message publishing/consumption rates
3. **Topic Metrics**: Per-topic statistics
4. **System Metrics**: CPU, memory, network usage

### Prometheus - Metrics Storage
- **URL**: http://localhost:9090
- **Features**:
  - Raw metrics querying
  - Target health monitoring
  - Alert rule evaluation

#### Useful Queries:
```promql
# Message publish rate
rate(pubsubgo_messages_published_total[5m])

# Active subscriptions
pubsubgo_subscriptions_active

# Message processing latency
histogram_quantile(0.95, rate(pubsubgo_message_processing_duration_seconds_bucket[5m]))

# Error rate
rate(pubsubgo_errors_total[5m])
```

### Jaeger - Distributed Tracing
- **URL**: http://localhost:16686
- **Features**:
  - End-to-end request tracing
  - Latency analysis
  - Dependency mapping

#### Finding Traces:
1. Select "pubsubgo" service
2. Choose operation (e.g., "publish", "subscribe")
3. View trace timeline and spans

## 📊 Available Metrics

### Application Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `pubsubgo_messages_published_total` | Counter | Total messages published |
| `pubsubgo_messages_consumed_total` | Counter | Total messages consumed |
| `pubsubgo_messages_acknowledged_total` | Counter | Total messages acknowledged |
| `pubsubgo_messages_failed_total` | Counter | Failed message deliveries |
| `pubsubgo_topics_total` | Gauge | Number of active topics |
| `pubsubgo_subscriptions_active` | Gauge | Active subscriptions |
| `pubsubgo_message_size_bytes` | Histogram | Message size distribution |
| `pubsubgo_message_processing_duration_seconds` | Histogram | Message processing time |
| `pubsubgo_queue_depth` | Gauge | Current queue depth per topic |

### System Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `go_goroutines` | Gauge | Number of goroutines |
| `go_memstats_alloc_bytes` | Gauge | Memory allocation |
| `process_cpu_seconds_total` | Counter | CPU usage |
| `redis_connected_clients` | Gauge | Redis connections |
| `redis_used_memory_bytes` | Gauge | Redis memory usage |

## 🔍 Viewing Metrics in Action

### 1. Check Service Health

```bash
# Check all targets in Prometheus
curl http://localhost:9090/api/v1/targets | jq

# Check PubSubGo health
curl http://localhost:8081/health

# Check metrics endpoint
curl http://localhost:9092/metrics
```

### 2. Grafana Dashboard Setup

1. Open http://localhost:3000
2. Login with admin/admin123
3. Navigate to Dashboards → Browse
4. Open "PubSubGo" folder
5. Select desired dashboard

### 3. Create Custom Alerts

In Grafana:
1. Go to Alerting → Alert rules
2. Click "New alert rule"
3. Configure conditions (e.g., error rate > 1%)
4. Set notification channels

## 🐳 Docker Compose Services

| Service | Port | Purpose |
|---------|------|---------|
| Prometheus | 9090 | Metrics collection |
| Grafana | 3000 | Visualization |
| Jaeger | 16686 | Tracing UI |
| AlertManager | 9093 | Alert management |
| Node Exporter | 9100 | System metrics |
| Redis Exporter | 9121 | Redis metrics |
| OTEL Collector | 4317/4318 | Telemetry collection |

## 🚨 Troubleshooting

### Prometheus Not Scraping Metrics

```bash
# Check target status
curl http://localhost:9090/api/v1/targets

# Verify PubSubGo metrics endpoint
curl http://localhost:9092/metrics
```

### Grafana Dashboards Not Loading

```bash
# Check datasource configuration
docker compose -f deployments/docker/monitoring.yml logs grafana

# Restart Grafana
docker compose -f deployments/docker/monitoring.yml restart grafana
```

### Jaeger Not Showing Traces

```bash
# Check OTEL collector status
docker compose -f deployments/docker/monitoring.yml logs otel-collector

# Verify tracing is enabled in config.yaml
grep -A 5 tracing config.yaml
```

## 🎯 Istio Service Mesh (Optional)

For advanced service mesh features:

```bash
# Run Istio setup script
./scripts/setup-istio.sh

# Access Istio dashboards
istioctl dashboard kiali    # Service graph
istioctl dashboard grafana   # Istio metrics
```

## 📝 Best Practices

1. **Regular Monitoring**: Check dashboards during peak loads
2. **Alert Configuration**: Set up alerts for critical metrics
3. **Retention Policy**: Configure appropriate data retention
4. **Resource Limits**: Monitor container resource usage
5. **Backup**: Regularly backup Grafana dashboards

## 🔗 Useful Commands

```bash
# Monitoring stack management
make monitoring-up       # Start all services
make monitoring-down     # Stop all services
make monitoring-restart  # Restart services
make monitoring-logs     # View logs
make monitoring-status   # Check status

# Generate test load
./scripts/generate-traffic.sh

# Open dashboards (macOS)
open http://localhost:3000    # Grafana
open http://localhost:9090    # Prometheus  
open http://localhost:16686   # Jaeger
```