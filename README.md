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

CloudVault consists of two main components:

1.  **CloudVault Agent**: a lightweight pod running in your cluster that collects PVC metrics (metadata, usage, cost) and pushes them to the control plane (or stores locally for CLI access in standalone mode).
2.  **CloudVault CLI**: A user-friendly command-line interface to query costs, view reports, and generate optimization recommendations.

---

## 🛠️ Installation

### Prerequisites
*   Go 1.22+
*   Node.js 18+ (for Web UI build)
*   Kubernetes Cluster (Simulated or Real)
*   Prometheus (Optional, for real-time usage metrics)

### Build from Source

```bash
# Clone the repository
git clone https://github.com/cloudvault-io/cloudvault.git
cd cloudvault

# Build binaries (includes Web UI)
make build
```

---

## 📖 Usage

### 1. View Cost Summary

Get a quick overview of your storage spend.

```bash
./bin/cloudvault cost
```

**Output:**
```
💰 Total Monthly Cost: $53.50/month ($642.00/year)

📊 Cost by Namespace:
NAMESPACE    MONTHLY   ANNUAL    % OF TOTAL
production   $35.00    $420.00   65.4%
staging      $10.50    $126.00   19.6%
dev          $8.00     $96.00    15.0%
```

### 2. Generate Recommendations

Find ways to save money immediately.

```bash
./bin/cloudvault recommendations
```

**Output:**
```
💡 Found 3 optimization opportunities

💰 Total Potential Savings: $25.50/month ($306.00/year)

🗑️ 1. Volume has not been accessed in 45 days. Consider backing up and deleting.
   PVC: dev/old-logs-archive
   Savings: $5.00/month | 🟢 Low impact

📏 2. Volume is only 5.0% utilized. Consider resizing to 15GB.
   PVC: production/postgres-backup
   Current: 200GB → Recommended: 15GB
   Savings: $18.50/month | 🟡 Medium impact
```

### 3. Prometheus Integration (Real-time Data)

Connect to your Prometheus server to get accurate usage-based recommendations (e.g., real "Zombie" detection based on I/O).

```bash
./bin/cloudvault recommendations --prometheus http://localhost:9090
```

### 4. Interactive Web Dashboard

Launch the professional multi-view dashboard to visualize costs, recommendations, and real-time analytics.

```bash
# Launch dashboard (default port 8080)
./bin/cloudvault dashboard --kubeconfig ~/.kube/config

# Launch with Mock Data (for demo purposes)
./bin/cloudvault dashboard --mock
```

**Features:**
- **Overview**: Monthly spend summary and potential savings.
- **Cost Analysis**: High-density charts and per-PVC cost inventory.
- **Optimization**: Filterable list of findings with one-click `kubectl` commands.
- **Responsive Navigation**: Full sidebar (desktop) or drawer navigation (mobile).

---

## 🗺️ Roadmap

- **Day 1-2**: MVP Core (Agent, CLI, Calculator) ✅
- **Day 3**: Testing & Refinement ✅
- **Day 4**: Enhanced Intelligence (Prometheus Integration) ✅
- **Day 5**: Documentation & Demo Prep ✅
- **Day 6**: Professional Web UI (Multi-View Sidebar Layout) ✅
- **Day 7**: Deep Tech Implementation (AI, eBPF, SIG) ✅
- **Phase 9**: Enterprise Hardening & Scaling ✅
- **Future**: GitOps Integration & Admission Controllers 🏗️

---

## 🤝 Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md) to get started.

## 📄 License

This project is licensed under the Apache 2.0 License - see the [LICENSE](LICENSE) file for details.
