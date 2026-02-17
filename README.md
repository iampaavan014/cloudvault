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

CloudVault consists of three main pillars:

1.  **CloudVault Control Plane**: A centralized API and Dashboard that manages policies, aggregates data, and provides the user interface.
2.  **CloudVault Agent**: A lightweight DaemonSet running on every node that collects real-time metrics and executes autonomous actions.
3.  **Governance Layer**: A Validating Admission Controller that enforces budget policies at the source (API Server).

---

## ☸️ Production Deployment (Helm)

CloudVault is designed for professional Kubernetes operations. The recommended deployment method is via our unified Helm chart, which automatically manages CRDs, RBAC, and all control plane components.

### One-Command Installation
Deploy the entire stack (Dashboard, Agent, and Policies) with a single command. The images are automatically pulled from the **GitHub Container Registry (GHCR)**.

```bash
helm upgrade --install cloudvault ./deploy/charts/cloudvault -n cloudvault --create-namespace
```

### What's Included:
*   **Automated Image Delivery**: Pulls production-ready images from `ghcr.io/iampaavan014/cloudvault` (Built automatically via GitHub Actions).
*   **Automatic CRD Management**: Installs `CostPolicy`, `StorageLifecyclePolicy`, and `Argo Workflows` CRDs.
*   **CloudVault Control Plane**: Deploys the Dashboard (React UI + Golang API) and centralized orchestrator.
*   **CloudVault Agent**: Deploys our lightweight, eBPF-enabled daemonset for cluster-wide metrics collection.
*   **Default Governance**: Bootstraps the cluster with production-grade budget and lifecycle policies.

### Prerequisites
*   Kubernetes 1.25+
*   Helm 3.x
*   (Optional) Prometheus/Grafana for extended metrics visualization.

---

## 🛠️ Advanced Build (Development)

To contribute to CloudVault or build individual binaries:

```bash
# Clone the repository
git clone https://github.com/iampaavan014/cloudvault.git
cd cloudvault

# Build binaries (Agent, CLI, and Web Dashboard)
make build
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
