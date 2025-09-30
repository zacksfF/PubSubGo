#!/bin/bash

echo "📦 Istio Setup for PubSubGo"
echo "============================"
echo ""
echo "Istio provides advanced service mesh capabilities including:"
echo "  - Traffic management and load balancing"
echo "  - Security with mutual TLS"
echo "  - Observability with distributed tracing"
echo "  - Policy enforcement"
echo ""

# Check if running in Kubernetes
if ! command -v kubectl &> /dev/null; then
    echo "⚠️  kubectl is not installed. Istio requires Kubernetes."
    echo ""
    echo "For local development, you can use:"
    echo "  - Docker Desktop with Kubernetes enabled"
    echo "  - Minikube: brew install minikube"
    echo "  - Kind: brew install kind"
    echo ""
    echo "Once Kubernetes is running, install Istio with:"
    echo ""
    echo "  # Download Istio"
    echo "  curl -L https://istio.io/downloadIstio | sh -"
    echo "  cd istio-*"
    echo "  export PATH=\$PWD/bin:\$PATH"
    echo ""
    echo "  # Install Istio"
    echo "  istioctl install --set profile=demo -y"
    echo "  kubectl label namespace default istio-injection=enabled"
    echo ""
    echo "Then deploy PubSubGo with Istio:"
    echo "  kubectl apply -k deployments/kubernetes/overlays/development/"
    echo "  kubectl apply -f deployments/istio/"
    echo ""
    exit 1
fi

# Check Kubernetes cluster
echo "Checking Kubernetes cluster..."
kubectl cluster-info &> /dev/null
if [ $? -ne 0 ]; then
    echo "❌ No Kubernetes cluster found. Please start a cluster first."
    exit 1
fi

echo "✅ Kubernetes cluster is running"
echo ""

# Check if Istio is installed
if command -v istioctl &> /dev/null; then
    echo "✅ Istio CLI is installed"
    istioctl version
else
    echo "📥 Istio CLI not found. Download it from:"
    echo "   https://istio.io/latest/docs/setup/getting-started/#download"
    echo ""
    echo "Quick install:"
    echo "   curl -L https://istio.io/downloadIstio | sh -"
    exit 1
fi

# Check if Istio is deployed in cluster
kubectl get namespace istio-system &> /dev/null
if [ $? -ne 0 ]; then
    echo ""
    echo "📦 Installing Istio in cluster..."
    istioctl install --set profile=demo -y
    
    # Enable sidecar injection
    kubectl label namespace default istio-injection=enabled
    echo "✅ Istio installed with demo profile"
else
    echo "✅ Istio is already installed in the cluster"
fi

echo ""
echo "🚀 Deploying PubSubGo with Istio..."

# Apply Istio configurations
echo "Applying Istio configurations..."
kubectl apply -f deployments/istio/

echo ""
echo "✅ Istio setup complete!"
echo ""
echo "📊 Access Istio dashboards:"
echo "  - Kiali (Service Graph): istioctl dashboard kiali"
echo "  - Grafana (Metrics): istioctl dashboard grafana"
echo "  - Jaeger (Tracing): istioctl dashboard jaeger"
echo "  - Prometheus: istioctl dashboard prometheus"
echo ""
echo "🔍 View Istio configuration:"
echo "  kubectl get virtualservices,destinationrules,gateways -n default"
echo ""
echo "📈 Generate load to see metrics:"
echo "  ./scripts/generate-traffic.sh"