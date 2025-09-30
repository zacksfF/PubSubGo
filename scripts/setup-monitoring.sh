#!/bin/bash

# PubSubGo Monitoring Setup Script
# This script sets up the complete monitoring stack for PubSubGo

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "🚀 Setting up PubSubGo Monitoring Stack"
echo "========================================"

# Check Docker installation
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed. Please install Docker first."
    echo "   Visit: https://docs.docker.com/get-docker/"
    exit 1
fi

# Check Docker Compose
DOCKER_COMPOSE_CMD=""
if command -v docker-compose &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker-compose"
    echo "✅ Found docker-compose"
elif docker compose version &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker compose"
    echo "✅ Found docker compose (plugin)"
else
    echo "❌ Docker Compose is not available"
    exit 1
fi

# Navigate to project root
cd "$PROJECT_ROOT"

echo
echo "📦 Starting monitoring stack..."
echo "This includes:"
echo "  - Prometheus (metrics collection)"
echo "  - Grafana (visualization)"
echo "  - Jaeger (distributed tracing)"
echo "  - AlertManager (alerting)"
echo "  - Node Exporter (system metrics)"
echo "  - Redis Exporter (Redis metrics)"
echo "  - OpenTelemetry Collector (telemetry pipeline)"

# Start the monitoring stack
$DOCKER_COMPOSE_CMD -f deployments/docker/monitoring.yml up -d

echo
echo "⏳ Waiting for services to start..."
sleep 10

# Check service health
echo
echo "🔍 Checking service health..."

# Function to check if a service is responding
check_service() {
    local name=$1
    local url=$2
    local max_attempts=30
    local attempt=1
    
    echo -n "Checking $name... "
    
    while [ $attempt -le $max_attempts ]; do
        if curl -s "$url" > /dev/null 2>&1; then
            echo "✅ Ready"
            return 0
        fi
        sleep 2
        attempt=$((attempt + 1))
    done
    
    echo "❌ Not responding after $max_attempts attempts"
    return 1
}

# Check each service
check_service "Prometheus" "http://localhost:9090/-/healthy" || true
check_service "Grafana" "http://localhost:3000/api/health" || true
check_service "Jaeger" "http://localhost:16686/" || true
check_service "AlertManager" "http://localhost:9093/-/healthy" || true

echo
echo "🎉 Monitoring stack setup complete!"
echo
echo "📊 Access URLs:"
echo "  - Grafana:      http://localhost:3000"
echo "    Login:        admin / admin123"
echo "    Dashboards:   PubSubGo folder"
echo
echo "  - Prometheus:   http://localhost:9090"
echo "    Targets:      http://localhost:9090/targets"
echo
echo "  - Jaeger:       http://localhost:16686"
echo "    Traces:       Search for 'pubsubgo' service"
echo
echo "  - AlertManager: http://localhost:9093"
echo "    Alerts:       http://localhost:9093/#/alerts"
echo
echo "🔧 Management commands:"
echo "  make monitoring-status   # Check status"
echo "  make monitoring-logs     # View logs"
echo "  make monitoring-restart  # Restart services"
echo "  make monitoring-down     # Stop services"
echo
echo "📈 Next steps:"
echo "1. Start your PubSubGo server: make run"
echo "2. Generate some traffic to see metrics"
echo "3. Check the dashboards in Grafana"
echo "4. Explore traces in Jaeger"
echo
echo "💡 The monitoring stack will automatically:"
echo "  - Collect metrics from PubSubGo on port 9092"
echo "  - Scrape system metrics via Node Exporter"
echo "  - Collect Redis metrics if Redis is running"
echo "  - Store traces from OpenTelemetry"
echo
echo "Happy monitoring! 🎯"