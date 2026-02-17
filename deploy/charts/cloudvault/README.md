# CloudVault Helm Chart

## Overview

CloudVault is a multi-cloud Kubernetes storage cost intelligence platform that helps you optimize storage costs across AWS, GCP, and Azure.

## Prerequisites

- Kubernetes 1.25+
- Helm 3.0+
- (Optional) Argo Workflows 3.0+ if not using bundled installation

## Installation Options

### Option 1: All-in-One (Recommended)

Install CloudVault with Argo Workflows bundled:

```bash
# Update Helm dependencies
helm dependency update ./deploy/charts/cloudvault

# Install with Argo Workflows enabled
helm upgrade --install cloudvault ./deploy/charts/cloudvault \
  -n cloudvault --create-namespace \
  --set argo.enabled=true
```

### Option 2: CloudVault Only

If you already have Argo Workflows installed:

```bash
helm upgrade --install cloudvault ./deploy/charts/cloudvault \
  -n cloudvault --create-namespace
```

### Option 3: Custom Configuration

Create a `values.yaml` file:

```yaml
argo:
  enabled: true  # Install Argo Workflows

policies:
  budget:
    amount: 5000.0
    alertThreshold: 80
  
  lifecycle:
    tiers:
      - name: warm
        storageClass: gp3
        duration: 30d
      - name: cold
        storageClass: s3
        duration: 90d
```

Then install:

```bash
helm dependency update ./deploy/charts/cloudvault
helm install cloudvault ./deploy/charts/cloudvault \
  -n cloudvault --create-namespace \
  -f values.yaml
```

## Configuration

### Core Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | CloudVault image repository | `ghcr.io/iampaavan014/cloudvault` |
| `image.tag` | CloudVault image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `Always` |

### Agent Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `agent.enabled` | Enable CloudVault agent | `true` |
| `agent.interval` | Metrics collection interval | `1m` |
| `agent.resources.limits.memory` | Agent memory limit | `200Mi` |
| `agent.resources.limits.cpu` | Agent CPU limit | `200m` |

### Dashboard Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `dashboard.enabled` | Enable CloudVault dashboard | `true` |
| `dashboard.replicaCount` | Number of dashboard replicas | `1` |
| `dashboard.service.type` | Dashboard service type | `ClusterIP` |
| `dashboard.service.port` | Dashboard service port | `8080` |

### Policy Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `policies.enabled` | Enable default policies | `true` |
| `policies.budget.amount` | Monthly budget limit ($) | `1000.0` |
| `policies.budget.alertThreshold` | Alert threshold (%) | `80` |
| `policies.lifecycle.tiers[0].name` | First tier name | `warm` |
| `policies.lifecycle.tiers[0].storageClass` | First tier storage class | `sc1` |
| `policies.lifecycle.tiers[0].duration` | First tier duration | `1h` |

### Argo Workflows Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `argo.enabled` | Install Argo Workflows | `false` |
| `argo.controller.resources.limits.memory` | Argo controller memory limit | `512Mi` |
| `argo.controller.resources.limits.cpu` | Argo controller CPU limit | `500m` |
| `argo.server.enabled` | Enable Argo server UI | `true` |

### Migration Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `migration.workflowTemplate.enabled` | Create migration workflow template | `true` |

## Verifying Installation

```bash
# Check CloudVault pods
kubectl get pods -n cloudvault

# Expected output:
# NAME                                    READY   STATUS    RESTARTS   AGE
# cloudvault-agent-xxxxx                  1/1     Running   0          1m
# cloudvault-dashboard-xxxxxxxxxx-xxxxx   1/1     Running   0          1m

# Check Argo Workflows (if enabled)
kubectl get workflows -n cloudvault

# View dashboard
kubectl port-forward -n cloudvault svc/cloudvault-dashboard 8080:8080
# Open http://localhost:8080
```

## Accessing the Dashboard

### Port Forward (Development)

```bash
kubectl port-forward -n cloudvault svc/cloudvault-dashboard 8080:8080
```

Then open http://localhost:8080 in your browser.

### LoadBalancer (Production)

Update your `values.yaml`:

```yaml
dashboard:
  service:
    type: LoadBalancer
```

Then upgrade:

```bash
helm upgrade cloudvault ./deploy/charts/cloudvault -n cloudvault -f values.yaml
```

## Uninstallation

```bash
# Uninstall CloudVault
helm uninstall cloudvault -n cloudvault

# Delete namespace (optional)
kubectl delete namespace cloudvault
```

## Troubleshooting

### Pods not starting

Check pod logs:
```bash
kubectl logs -n cloudvault -l app=cloudvault-agent
kubectl logs -n cloudvault -l app=cloudvault-dashboard
```

### Argo Workflows not executing

Verify Argo controller is running:
```bash
kubectl get pods -n cloudvault -l app.kubernetes.io/name=argo-workflows
```

If Argo is not installed and you want to enable it:
```bash
helm upgrade cloudvault ./deploy/charts/cloudvault \
  -n cloudvault \
  --set argo.enabled=true \
  --reuse-values
```

### Migration workflows pending

This is expected if Argo Workflows is not installed. CloudVault creates workflow resources, but they require the Argo controller to execute.

## Support

- GitHub Issues: https://github.com/iampaavan014/cloudvault/issues
- Documentation: https://github.com/iampaavan014/cloudvault
