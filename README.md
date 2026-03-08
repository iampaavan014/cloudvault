# 🌟 CloudVault

> **Multi-Cloud Kubernetes Storage Cost Intelligence Platform**

CloudVault is an open-source platform designed to solve the #1 pain point in modern Kubernetes: **storage cost visibility and optimization**. It brings NetApp-caliber storage intelligence to the broader cloud-native ecosystem, helping you identify zombie volumes, over-provisioned storage, and inefficient storage classes across AWS, GCP, and Azure.

---

## 🚀 Key Features

*   **💰 Cost Intelligence**: Real-time visibility into PVC costs across AWS, GCP, and Azure.
*   **🧠 AI-Powered Decision Layer**: Predictive cost forecasting (LSTM) and optimal placement agents (RL).
*   **🕸️ Storage Intelligence Graph (SIG)**: Relationship discovery via Neo4j for "High-Cost Gravity" detection.
*   **⚡ eBPF Egress Monitoring**: Kernel-level traffic attribution with zero-overhead accuracy.
*   **📊 Multi-View Dashboard**: Professional React UI with dedicated Cost Analytics and Governance views.
*   **🛡️ Autonomous Orchestration**: Kubernetes-native `StorageLifecyclePolicies` for automatic tiering.

---

## 🏗️ Architecture

CloudVault is designed as a **storage intelligence platform** that integrates with the Kubernetes ecosystem. It consists of two main components:

### 1. CloudVault Platform (This Project)
- **CloudVault Agent**: Lightweight DaemonSet that collects storage metrics, cost data, and network traffic (via eBPF)
- **CloudVault Dashboard**: React-based UI with real-time cost analytics and governance controls
- **Policy Engine**: Enforces budget limits and lifecycle policies via Kubernetes admission controllers
- **Migration Orchestrator**: Creates and manages storage migration workflows

### 2. Argo Workflows (External Dependency)
- **Workflow Controller**: Executes the migration workflows created by CloudVault
- **Orchestration Engine**: Handles retries, state management, and workflow visualization
- **CNCF Graduated Project**: Industry-standard workflow orchestration for Kubernetes

### How They Work Together
```
┌─────────────────────────────────────────────────────────┐
│ CloudVault Agent                                        │
│  ├─ Analyzes storage costs and usage patterns          │
│  ├─ Identifies optimization opportunities               │
│  └─ Creates Argo Workflow resources for migrations     │
└─────────────────────────────────────────────────────────┘
                         ↓
┌─────────────────────────────────────────────────────────┐
│ Argo Workflows Controller (Optional Dependency)        │
│  ├─ Executes migration workflows                        │
│  ├─ Handles orchestration and error recovery           │
│  └─ Provides workflow status and logs                   │
└─────────────────────────────────────────────────────────┘
```

> **Note**: CloudVault **creates** migration workflows, Argo **executes** them. This separation of concerns follows cloud-native best practices and allows users to leverage existing Argo installations.

---

## ☸️ Production Deployment

CloudVault is production-ready and can be deployed to any Kubernetes cluster via Helm.

### Helm Quickstart

```bash
# Update Helm dependencies
helm dependency update ./deploy/charts/cloudvault

# Install with Argo Workflows enabled
helm upgrade --install cloudvault ./deploy/charts/cloudvault \
  -n cloudvault --create-namespace \
  --set argo.enabled=true
```

See [Helm Chart README](deploy/charts/cloudvault/README.md) for full configuration options.

### Local Development with kind

For local `kind` clusters, the Makefile automates building, image loading, and Helm install:

```bash
make production-deploy
```

This performs:
1.  **Build**: Compiles Go binaries and the Web UI.
2.  **Containerize**: Builds the Agent and AI (Gunicorn + PyTorch) Docker images.
3.  **Load Images**: Sideloads images into the local `kind` cluster.
4.  **Helm Install**: Runs `helm upgrade --install` with production settings.

### 🍱 Multi-Service Architecture
The deployment includes:
- **CloudVault Agent**: DaemonSet for kernel-level eBPF monitoring.
- **CloudVault AI**: Microservice powered by PyTorch & Gunicorn (Port 5005).
- **CloudVault Dashboard**: React-based professional cost analytics (Port 8080).
- **Prometheus**: Metrics collection and monitoring.

---

### Helm Configuration Options
| Key | Default | Description |
| :--- | :--- | :--- |
| `agent.interval` | `1m` | Metrics collection frequency |
| `ai.enabled` | `true` | Toggle the recursive neural network layer |
| `dashboard.service.type` | `ClusterIP` | Service exposure strategy |
| `argo.enabled` | `false` | Enable bundled Argo Workflows integration |

---

---

## 🛠️ Advanced Build (Development)

To contribute to CloudVault or build individual binaries:

```bash
# Clone the repository
git clone https://github.com/iampaavan014/cloudvault.git
cd cloudvault

# Install development dependencies (golangci-lint)
make dev-deps

# Build binaries (Agent, CLI, and Web Dashboard)
make build

# Run unit tests
make unittest

# Run linters
make lint
```

---

## 📖 Usage

### 1. View Cost Summary (CLI)

Get a quick overview of your storage spend via the agent CLI.

```bash
./bin/cloudvault-agent cost
```

### 2. Interactive Web Dashboard

Access the professional multi-view dashboard to visualize costs, recommendations, and real-time analytics.

```bash
# Access via Port Forward
kubectl port-forward svc/cloudvault-dashboard 8080:8080 -n cloudvault
```

**Features:**
- **Overview**: Monthly spend summary and potential savings.
- **Cost Analysis**: High-density charts and per-PVC cost inventory.
- **Governance**: Visual progress bars for budget tracking and policy status.
- **Autonomous Status**: Real-time monitoring of migration workflows.

---

## 🗺️ Roadmap

- **Day 1-2**: MVP Core (Agent, CLI, Calculator) ✅
- **Day 3-4**: Testing & Prometheus Integration ✅
- **Day 5-6**: Professional Web UI & Multi-View Layout ✅
- **Day 7**: Deep Tech Implementation (AI, eBPF, SIG) ✅
- **Phase 9-10**: Enterprise Hardening & Budget Enforcement ✅
- **Phase 11**: Multi-Cluster Orchestration (Argo) ✅
- **Current**: CNCF Sandbox Graduation & One-Command Helm Release 🏆

---

## 🤝 Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md) to get started.

## 📄 License

This project is licensed under the Apache 2.0 License - see the [LICENSE](LICENSE) file for details.
