# PubSubGo Architecture & Tools Documentation

## 📖 Project Overview

PubSubGo is a modern, scalable message broker built with Go that implements the publish-subscribe pattern with advanced features like persistence, consumer groups, acknowledgments, and comprehensive observability.

## 🏗️ Project Structure

### Core Directories

```
PubSubGo/
├── cmd/                    # Application entry points
│   ├── server/            # Main server application
│   └── cli/               # Command-line client
├── internal/              # Private application code
│   ├── core/              # Domain entities (Clean Architecture)
│   ├── ports/             # Interface definitions
│   ├── adapters/          # External system implementations
│   ├── services/          # Business logic services
│   └── config/            # Configuration management
├── pkg/                   # Public library code
├── deployments/           # Deployment configurations
│   ├── docker/            # Docker configurations
│   ├── kubernetes/        # Kubernetes manifests
│   ├── istio/             # Istio service mesh configs
│   ├── prometheus/        # Prometheus configurations
│   ├── grafana/           # Grafana dashboards
│   └── otel/              # OpenTelemetry configurations
├── test/                  # Test files
└── docs/                  # Documentation
```

## 🛠️ Tools & Technologies

### 🐳 **Docker & Containerization**

#### **Role & Purpose**
- **Container Runtime**: Packages PubSubGo into portable, consistent containers
- **Development Environment**: Provides reproducible development setup
- **Microservices Orchestration**: Manages multi-service deployments

#### **Files & Configurations**
```
Dockerfile                 # Multi-stage build for PubSubGo server
docker-compose.yml         # Full observability stack orchestration
.dockerignore              # Files to exclude from Docker build context
```

#### **Key Features**
- **Multi-stage Build**: Optimized image size with separate build/runtime stages
- **Security**: Non-root user execution, minimal attack surface
- **Health Checks**: Built-in container health monitoring
- **Development Stack**: Redis, Prometheus, Grafana, Jaeger all-in-one

#### **Commands**
```bash
# Build image
docker build -t pubsubgo:latest .

# Run full stack
docker compose up -d

# View logs
docker compose logs -f pubsubgo

# Stop stack
docker compose down
```

---

### ☸️ **Kubernetes (K8s)**

#### **Role & Purpose**
- **Container Orchestration**: Manages containerized applications at scale
- **Service Discovery**: Automatic service-to-service communication
- **Load Balancing**: Distributes traffic across multiple instances
- **Auto-scaling**: Horizontal pod autoscaling based on metrics

#### **Directory Structure**
```
deployments/kubernetes/
├── base/                  # Base Kubernetes resources
│   ├── namespace.yaml     # Namespace definition
│   ├── configmap.yaml     # Configuration management
│   ├── pubsubgo.yaml      # Main application deployment
│   ├── redis.yaml         # Redis deployment & service
│   ├── observability.yaml # Monitoring stack
│   ├── ingress.yaml       # External access configuration
│   └── kustomization.yaml # Kustomize base configuration
└── overlays/              # Environment-specific configurations
    ├── development/       # Dev environment settings
    └── production/        # Production environment settings
```

#### **Key Components**
- **Deployments**: Manage pod replicas and rolling updates
- **Services**: Expose applications within cluster
- **ConfigMaps**: Manage configuration data
- **Ingress**: HTTP/HTTPS routing and SSL termination
- **PVC**: Persistent storage for Redis
- **Kustomize**: Configuration management without templates

#### **Commands**
```bash
# Apply development environment
kubectl apply -k deployments/kubernetes/overlays/development/

# Apply production environment
kubectl apply -k deployments/kubernetes/overlays/production/

# Check pods
kubectl get pods -n pubsubgo

# View logs
kubectl logs -f deployment/pubsubgo -n pubsubgo

# Port forward for testing
kubectl port-forward svc/pubsubgo 8080:8080 -n pubsubgo
```

---

### 🕸️ **Istio Service Mesh**

#### **Role & Purpose**
- **Traffic Management**: Intelligent routing, load balancing, circuit breakers
- **Security**: mTLS, authorization policies, security scanning
- **Observability**: Distributed tracing, metrics, access logs
- **Policy Enforcement**: Rate limiting, fault injection, timeouts

#### **Configuration Files**
```
deployments/istio/
├── gateway.yaml           # External traffic ingress
├── virtual-service.yaml   # Traffic routing rules
├── destination-rule.yaml  # Load balancing & circuit breakers
├── security.yaml          # mTLS & authorization policies
├── telemetry.yaml         # Enhanced observability
└── README.md              # Deployment instructions
```

#### **Key Features**
- **Mutual TLS**: Automatic service-to-service encryption
- **Circuit Breakers**: Fault tolerance and resilience
- **Traffic Splitting**: Canary deployments and A/B testing
- **Retry Logic**: Automatic retry with exponential backoff
- **Rate Limiting**: Request throttling and quota management

#### **Commands**
```bash
# Install Istio
istioctl install --set values.defaultRevision=default

# Enable injection
kubectl label namespace pubsubgo istio-injection=enabled

# Apply Istio configs
kubectl apply -f deployments/istio/

# View Kiali dashboard
istioctl dashboard kiali

# Check proxy status
istioctl proxy-status
```

---

### 📊 **Prometheus (Metrics & Monitoring)**

#### **Role & Purpose**
- **Metrics Collection**: Scrapes metrics from PubSubGo and infrastructure
- **Time Series Database**: Stores metrics with timestamps
- **Alerting**: Triggers alerts based on defined rules
- **Service Discovery**: Automatically discovers targets to monitor

#### **Configuration Files**
```
deployments/prometheus/
├── prometheus.yml         # Main Prometheus configuration
└── rules/
    └── pubsubgo.yml       # Recording rules and alerts
```

#### **Monitored Metrics**
```
# Application Metrics
pubsubgo_messages_published_total
pubsubgo_messages_consumed_total
pubsubgo_message_latency_seconds
pubsubgo_active_connections_total
pubsubgo_errors_total
pubsubgo_topic_messages_total

# Infrastructure Metrics
redis_up
container_cpu_usage_seconds_total
container_memory_usage_bytes
node_load1
```

#### **Commands**
```bash
# Check targets
curl http://localhost:9090/api/v1/targets

# Query metrics
curl 'http://localhost:9090/api/v1/query?query=pubsubgo_messages_published_total'

# Reload configuration
curl -X POST http://localhost:9090/-/reload
```

---

### 📈 **Grafana (Visualization)**

#### **Role & Purpose**
- **Data Visualization**: Creates dashboards from Prometheus metrics
- **Alerting**: Visual alerts and notifications
- **Multi-Source**: Integrates Prometheus, Redis, Jaeger data
- **User Management**: Role-based access control

#### **Dashboard Files**
```
deployments/grafana/
├── datasources/
│   └── datasources.yml    # Prometheus, Jaeger, Redis connections
└── dashboards/
    ├── dashboard.yml       # Dashboard provisioning config
    └── pubsubgo-overview.json # Main PubSubGo dashboard
```

#### **Dashboard Panels**
- **Message Throughput**: Publish/consume rates over time
- **Latency Percentiles**: P50, P95, P99 message latency
- **Active Connections**: WebSocket and HTTP connections
- **Error Rates**: Error frequency and types
- **Resource Usage**: CPU, memory, disk usage
- **Topic Statistics**: Message counts per topic

#### **Access URLs**
```
http://localhost:3000      # Grafana UI (admin/admin)
```

---

### 🔍 **Jaeger (Distributed Tracing)**

#### **Role & Purpose**
- **Request Tracing**: Tracks requests across service boundaries
- **Performance Analysis**: Identifies bottlenecks and latency issues
- **Dependency Mapping**: Visualizes service interactions
- **Root Cause Analysis**: Traces errors to their source

#### **Integration Points**
```go
// Tracing spans in PubSubGo
pkg/tracing/tracer.go      # OpenTelemetry integration
internal/adapters/*/       # Instrumented adapters
```

#### **Traced Operations**
- **HTTP Requests**: REST API calls with timing
- **Message Publishing**: End-to-end message flow
- **Database Operations**: Redis operations with latency
- **WebSocket Connections**: Real-time communication
- **Service Calls**: Inter-service communication

#### **Access URLs**
```
http://localhost:16686     # Jaeger UI
```

---

### 📡 **OpenTelemetry (Observability Framework)**

#### **Role & Purpose**
- **Telemetry Collection**: Unified metrics, traces, and logs
- **Vendor Neutral**: Works with any observability backend
- **Auto-instrumentation**: Automatic span creation
- **Context Propagation**: Distributed trace correlation

#### **Configuration**
```
deployments/otel/
└── otel-collector.yml     # OTEL Collector configuration
```

#### **Components**
- **OTLP Receivers**: HTTP and gRPC endpoints
- **Processors**: Batch processing and resource attribution
- **Exporters**: Send data to Jaeger and Prometheus

---

## 📦 **Go Libraries & Dependencies**

### **Core Libraries**
```go
// Web Framework & HTTP
github.com/gorilla/websocket  // WebSocket support
net/http                      // Standard HTTP server

// Messaging & Storage
github.com/redis/go-redis/v9  // Redis client
github.com/google/uuid        // UUID generation

// Configuration & CLI
github.com/spf13/viper        // Configuration management
github.com/spf13/cobra        // CLI framework

// Compression
github.com/golang/snappy      // Snappy compression
github.com/pierrec/lz4/v4     // LZ4 compression

// Observability
github.com/prometheus/client_golang          // Prometheus metrics
go.opentelemetry.io/otel                    // OpenTelemetry SDK
go.opentelemetry.io/otel/exporters/otlp     // OTLP exporter
github.com/sirupsen/logrus                  // Structured logging

// Testing
github.com/stretchr/testify   // Testing framework
```

---

## 🏛️ **Clean Architecture Implementation**

### **Core Domain (`internal/core/`)**
```
message/     # Message entities with status, priorities, delivery modes
topic/       # Topic management with partitioning and statistics  
subscription/# Subscription types (pull/push) with ACK tracking
consumer/    # Consumer groups with rebalancing and heartbeats
```

### **Ports (`internal/ports/`)**
```
repository/  # Storage interfaces (messages, topics, subscriptions)
broker/      # Message broker interface for pub/sub operations
metrics/     # Metrics collection interfaces
cache/       # Caching interfaces
```

### **Adapters (`internal/adapters/`)**
```
storage/redis/     # Redis persistence with streams and clustering
storage/memory/    # In-memory storage for development
api/http/          # REST API handlers
api/websocket/     # WebSocket real-time subscriptions
metrics/prometheus/# Prometheus metrics exporter
```

### **Services (`internal/services/`)**
```
publisher/     # Message publishing with batching and compression
subscriber/    # Subscription management and message delivery
acknowledgment/# ACK/NACK handling and retry logic
topic/         # Topic management operations
```

---

## 🚀 **Deployment Commands**

### **Local Development**
```bash
# Start Redis
brew services start redis

# Build and run server
go build -o pubsubgo-server ./cmd/server
./pubsubgo-server -config config.yaml

# Build and use CLI
go build -o pubsub-cli ./cmd/cli
./pubsub-cli --server http://localhost:8081 topics list
```

### **Docker Development**
```bash
# Full stack with observability
docker compose up -d

# View all services
docker compose ps

# Check logs
docker compose logs -f pubsubgo
```

### **Kubernetes Development**
```bash
# Deploy to development environment
kubectl apply -k deployments/kubernetes/overlays/development/

# Monitor deployment
kubectl get pods -n pubsubgo-dev -w

# Access services
kubectl port-forward svc/dev-pubsubgo 8080:8080 -n pubsubgo-dev
```

### **Production Deployment**
```bash
# Deploy with Istio
kubectl label namespace pubsubgo-prod istio-injection=enabled
kubectl apply -f deployments/istio/
kubectl apply -k deployments/kubernetes/overlays/production/

# Monitor with Istio
istioctl dashboard kiali
```

---

## 🔧 **Configuration Management**

### **Environment Variables**
```bash
# Server Configuration
PUBSUB_SERVER_PORT=8081
PUBSUB_REDIS_HOST=redis
PUBSUB_REDIS_PASSWORD=redispw
PUBSUB_METRICS_ENABLED=true
PUBSUB_TRACING_ENABLED=true

# Observability
PUBSUB_TRACING_OTLP_ENDPOINT=http://otel-collector:4318/v1/traces
PUBSUB_METRICS_PORT=9092
```

### **Config Files**
```
config.yaml              # Main configuration
deployments/*/           # Environment-specific configs
```

---

## 📊 **Monitoring & Alerting**

### **Key Metrics to Monitor**
1. **Message Throughput**: Publish/consume rates
2. **Latency**: P95/P99 message processing time
3. **Error Rates**: Failed operations percentage
4. **Resource Usage**: CPU, memory, disk, network
5. **Connection Health**: Active connections and heartbeats

### **Alert Conditions**
- High error rate (>1%)
- High latency (P95 >1s)
- Consumer lag (>1000 messages)
- Service down
- Redis connection failure

---

## 🔒 **Security Features**

### **Istio Security**
- **mTLS**: Automatic mutual TLS between services
- **Authorization Policies**: Fine-grained access control
- **Network Policies**: Traffic isolation and segmentation

### **Application Security**
- **Input Validation**: Request sanitization and validation
- **Rate Limiting**: DoS protection
- **Health Checks**: Service availability monitoring
- **Secrets Management**: Kubernetes secrets for sensitive data

---

This documentation provides a complete overview of all tools, technologies, and architectural components in the PubSubGo project. Each tool serves a specific purpose in creating a production-ready, observable, and scalable message broker system.