#!/bin/bash
# setup-test-cluster.sh - Create a local Kind cluster with test PVCs

set -e

echo "🚀 CloudVault Test Setup"
echo "========================"
echo ""

# Check if Kind is installed
if ! command -v kind &> /dev/null; then
    echo "❌ Kind is not installed"
    echo ""
    echo "Install Kind:"
    echo "  macOS:   brew install kind"
    echo "  Linux:   curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64"
    echo "           chmod +x ./kind && sudo mv ./kind /usr/local/bin/kind"
    echo ""
    exit 1
fi

echo "✅ Kind found"

# Check if cluster already exists
if kind get clusters 2>/dev/null | grep -q "cloudvault-test"; then
    echo "⚠️  Cluster 'cloudvault-test' already exists"
    read -p "Delete and recreate? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "🗑️  Deleting existing cluster..."
        kind delete cluster --name cloudvault-test
    else
        echo "Using existing cluster"
        kind get kubeconfig --name cloudvault-test > /tmp/cloudvault-kubeconfig
        export KUBECONFIG=/tmp/cloudvault-kubeconfig
        echo "✅ Kubeconfig set"
        exit 0
    fi
fi

# Create Kind cluster
echo "🏗️  Creating Kind cluster 'cloudvault-test'..."
cat <<EOF | kind create cluster --name cloudvault-test --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
- role: worker
EOF

# Get kubeconfig
kind get kubeconfig --name cloudvault-test > /tmp/cloudvault-kubeconfig
export KUBECONFIG=/tmp/cloudvault-kubeconfig

echo "✅ Cluster created"
echo ""

# Wait for cluster to be ready
echo "⏳ Waiting for cluster to be ready..."
kubectl wait --for=condition=Ready nodes --all --timeout=120s

echo "✅ Cluster is ready"
echo ""

# Install local-path-provisioner (for dynamic PVC provisioning)
echo "📦 Installing local-path-provisioner..."
kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.24/deploy/local-path-storage.yaml

# Wait for provisioner to be ready
kubectl wait --for=condition=Available deployment/local-path-provisioner -n local-path-storage --timeout=60s

echo "✅ Provisioner ready"
echo ""

# Create test namespaces
echo "📁 Creating test namespaces..."
kubectl create namespace production || true
kubectl create namespace staging || true
kubectl create namespace dev || true

echo "✅ Namespaces created"
echo ""

# Create test PVCs
echo "💾 Creating test PVCs..."

cat <<EOF | kubectl apply -f -
---
# Production - Database
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-data
  namespace: production
  labels:
    app: postgres
    env: production
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 100Gi
  storageClassName: local-path
---
# Production - Backup
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-backup
  namespace: production
  labels:
    app: postgres
    type: backup
    env: production
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 200Gi
  storageClassName: local-path
---
# Production - Redis
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: redis-data
  namespace: production
  labels:
    app: redis
    env: production
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 50Gi
  storageClassName: local-path
---
# Staging - App Data
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app-data
  namespace: staging
  labels:
    app: myapp
    env: staging
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 75Gi
  storageClassName: local-path
---
# Staging - Logs
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app-logs
  namespace: staging
  labels:
    app: myapp
    type: logs
    env: staging
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 30Gi
  storageClassName: local-path
---
# Dev - Test DB
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-db
  namespace: dev
  labels:
    app: testapp
    env: dev
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 20Gi
  storageClassName: local-path
---
# Dev - Cache
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: cache-data
  namespace: dev
  labels:
    app: cache
    env: dev
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: local-path
---
# Dev - Old unused volume (zombie candidate)
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: old-logs-archive
  namespace: dev
  labels:
    app: oldapp
    type: archive
    env: dev
  annotations:
    description: "Unused volume for demo purposes"
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 50Gi
  storageClassName: local-path
EOF

echo "✅ Test PVCs created"
echo ""

# Wait for PVCs to be bound
echo "⏳ Waiting for PVCs to be bound..."
sleep 5
kubectl get pvc --all-namespaces

echo ""
echo "✅ Test cluster is ready!"
echo ""
echo "📋 Summary:"
echo "   Cluster name: cloudvault-test"
echo "   Kubeconfig:   /tmp/cloudvault-kubeconfig"
echo "   Namespaces:   production, staging, dev"
echo "   PVCs:         8 test volumes"
echo ""
echo "🚀 To use this cluster:"
echo "   export KUBECONFIG=/tmp/cloudvault-kubeconfig"
echo "   kubectl get pvc --all-namespaces"
echo ""
echo "🧹 To delete the cluster:"
echo "   kind delete cluster --name cloudvault-test"
echo ""