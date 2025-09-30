# Istio Service Mesh Configuration for PubSubGo

This directory contains Istio service mesh configurations for PubSubGo.

## Prerequisites

1. Install Istio on your Kubernetes cluster:
```bash
curl -L https://istio.io/downloadIstio | sh -
istioctl install --set values.defaultRevision=default
```

2. Enable Istio injection for the namespace:
```bash
kubectl label namespace pubsubgo istio-injection=enabled
```

## Files Overview

- `gateway.yaml` - Istio Gateway configuration for external access
- `virtual-service.yaml` - Traffic routing rules for services
- `destination-rule.yaml` - Load balancing and circuit breaker policies
- `security.yaml` - mTLS and authorization policies
- `telemetry.yaml` - Enhanced observability configuration

## Deployment

Deploy Istio configurations:
```bash
kubectl apply -f deployments/istio/
```

## Features Enabled

### Traffic Management
- Load balancing with least connection algorithm
- Circuit breaker with outlier detection
- Retry policies for resilience
- Traffic splitting capabilities

### Security
- Strict mTLS between all services
- Authorization policies for service-to-service communication
- Ingress gateway security

### Observability
- Enhanced metrics collection
- Distributed tracing with Jaeger
- Access logging to OpenTelemetry

## Accessing Services

Add these entries to your `/etc/hosts` file:
```
<INGRESS_IP> pubsubgo.local
<INGRESS_IP> grafana.local
<INGRESS_IP> prometheus.local
<INGRESS_IP> jaeger.local
```

Get the ingress IP:
```bash
kubectl get svc istio-ingressgateway -n istio-system
```

## Monitoring Istio

View Istio metrics in Grafana or use Kiali for service mesh visualization:
```bash
istioctl dashboard kiali
```