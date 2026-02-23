# CloudVault - Developer Quick Reference

## 🚀 What Changed (Quick Summary)

### Core Integration Fixes

1. **SIG (Neo4j) is now fully operational**
   - Agent automatically syncs all PVCs to graph database
   - Cross-region gravity detection runs every optimization cycle
   - Pod-to-PVC relationships tracked for data locality analysis

2. **TimescaleDB records all metrics**
   - Historical data feeds AI forecasting models
   - Automatic recording on every collection cycle
   - Connection managed with graceful shutdown

3. **AI service has health monitoring**
   - Background health checks every 30 seconds
   - Graceful fallback when service unavailable
   - Status exposed at `/health` endpoint

4. **CRD policies update dynamically**
   - No agent restart needed for policy changes
   - Background watcher polls every 30 seconds
   - Both StorageLifecyclePolicy and CostPolicy supported

## 🔧 Configuration Updates

### New CLI Flags

```bash
cloudvault-agent \
  --neo4j-uri bolt://neo4j:7687 \
  --neo4j-user neo4j \
  --neo4j-password changeme \
  --timescale "postgresql://user:pass@localhost:5432/cloudvault"
```

### New Config File Options

```yaml
# Neo4j (Storage Intelligence Graph)
neo4j_uri: "bolt://neo4j:7687"
neo4j_user: "neo4j"
neo4j_password: "changeme"

# TimescaleDB (Historical Metrics)
timescale_conn: "postgresql://user:pass@timescale:5432/cloudvault"

# Multi-Cloud Credentials
gcp_project: "my-project"
gcp_creds_path: "/path/to/creds.json"

azure_subscription_id: "..."
azure_tenant_id: "..."
azure_client_id: "..."
azure_client_secret: "..."
```

## 🏥 Health Check Endpoints

### Component-Level Health
```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "healthy",
  "version": "v1.0.0",
  "timestamp": "2024-01-15T10:30:00Z",
  "components": {
    "kubernetes": {"status": "healthy", "last_check": "..."},
    "prometheus": {"status": "healthy", "last_check": "..."},
    "ai_service": {"status": "degraded", "message": "...", "last_check": "..."}
  }
}
```

### Kubernetes Probes
```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080

readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
```

## 📊 New Lifecycle Controller Flow

Every optimization cycle now executes:

1. **Collect** PVC metrics (K8s + Prometheus + eBPF)
2. **Sync to SIG** (Neo4j) with region/provider metadata
3. **Map relationships** (Pod → PVC) for gravity analysis
4. **Detect gravity issues** (cross-region workloads)
5. **Record to TimescaleDB** for AI training
6. **Generate recommendations** (AI-powered)
7. **Evaluate policies** (lifecycle rules)
8. **Execute migrations** (with conflict resolution)

## 🔍 Observability

### Log Levels
The agent now logs:
- `INFO`: Normal operations (policy updates, migrations)
- `WARN`: Degraded state (AI service down, missing Prometheus)
- `ERROR`: Critical failures (K8s unreachable, SIG sync failed)
- `DEBUG`: Detailed traces (policy evaluation, graph queries)

### Key Log Messages

```
✅ "Storage Intelligence Graph (Neo4j) enabled"
✅ "TimescaleDB persistence enabled"
✅ "Started StorageLifecyclePolicy watcher"
✅ "Synced PVCs to Storage Intelligence Graph"
⚠️  "Detected cross-region data gravity issues"
⚠️  "AI service became unhealthy"
```

## 🧪 Testing

### Run All Tests
```bash
go test ./pkg/... -v
```

### Test Specific Packages
```bash
go test ./pkg/orchestrator/lifecycle -v
go test ./pkg/graph -v
go test ./pkg/ai -v
```

### Build Verification
```bash
make build
# or
go build -o bin/cloudvault-agent ./cmd/agent
```

## 📝 Code Examples

### Accessing SIG in Code
```go
sig, err := graph.NewSIG(neo4jURI, user, password)
if err != nil {
    log.Fatal(err)
}
defer sig.Close(ctx)

// Sync PVCs
err = sig.SyncPVCs(ctx, metrics)

// Detect gravity
crossRegion, err := sig.GetCrossRegionGravity(ctx)
```

### Recording to TimescaleDB
```go
tsdb, err := graph.NewTimescaleDB(connString)
if err != nil {
    log.Fatal(err)
}
defer tsdb.Close()

// Record metrics
err = tsdb.RecordMetrics(ctx, metrics)

// Get history for AI
history, err := tsdb.GetHistory(ctx, namespace, name, 30*24*time.Hour)
```

### Checking AI Service Health
```go
if ai.IsAIServiceHealthy() {
    prediction := forecaster.ForecastMonthlySpend(current, trend)
} else {
    // Fallback to last known value
    prediction = lastKnownCost
}

// Get detailed status
status := ai.GetAIServiceStatus()
fmt.Printf("AI Service: %s (last check: %s)\n", 
    status.Healthy, status.LastHealthCheck)
```

## 🚨 Troubleshooting

### SIG Connection Issues
```bash
# Check Neo4j connectivity
bolt://neo4j:7687

# Verify credentials
neo4j_user: neo4j
neo4j_password: <your-password>
```

### TimescaleDB Connection Issues
```bash
# Check PostgreSQL connection
postgresql://user:pass@host:5432/dbname

# Verify table creation
psql -c "SELECT * FROM pvc_metrics LIMIT 1;"
```

### AI Service Unavailable
The agent will log warnings but continue operating. Recommendations will use simpler heuristics instead of ML predictions.

## 📦 Dependencies Updated

No new dependencies added - all fixes use existing packages:
- `github.com/neo4j/neo4j-go-driver/v5`
- `github.com/lib/pq` (PostgreSQL/TimescaleDB)

## 🎯 What to Test

1. **Deploy with SIG enabled** - Verify Neo4j receives PVC nodes
2. **Check health endpoints** - Confirm all components report status
3. **Update a policy** - Verify it takes effect within 30s
4. **Monitor logs** - Check for cross-region gravity warnings
5. **View dashboard** - Confirm AI service status is visible

---

**All changes are backward compatible** - Agents without Neo4j/TimescaleDB continue to work with degraded intelligence features.
