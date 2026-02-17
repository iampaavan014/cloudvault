package types

import (
	"time"
)

// PVCMetric represents storage metrics for a PersistentVolumeClaim
type PVCMetric struct {
	// Identifiers
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	ClusterID string `json:"cluster_id"`
	Provider  string `json:"provider"` // aws, gcp, azure
	Region    string `json:"region"`   // us-east-1, etc.

	// Storage characteristics
	StorageClass string `json:"storage_class"`
	SizeBytes    int64  `json:"size_bytes"`
	UsedBytes    int64  `json:"used_bytes"`   // Actual usage (requires metrics-server)
	EgressBytes  uint64 `json:"egress_bytes"` // Network traffic (requires eBPF)

	// Performance metrics (future - requires Prometheus or cloud APIs)
	ReadIOPS        float64 `json:"read_iops"`
	WriteIOPS       float64 `json:"write_iops"`
	ReadThroughput  float64 `json:"read_throughput_bps"`  // bytes per second
	WriteThroughput float64 `json:"write_throughput_bps"` // bytes per second

	// Cost information
	HourlyCost  float64 `json:"hourly_cost"`
	MonthlyCost float64 `json:"monthly_cost"`

	// Intelligence Graph Data (Phase 4 Pillars)
	MountedPods []string `json:"mounted_pods"`

	// Metadata
	CreatedAt      time.Time         `json:"created_at"`
	LastAccessedAt time.Time         `json:"last_accessed_at"` // Future - requires eBPF or audit logs
	Labels         map[string]string `json:"labels"`
	Annotations    map[string]string `json:"annotations"`
}

// ClusterInfo represents Kubernetes cluster metadata
type ClusterInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"` // aws, gcp, azure, unknown
	Region   string `json:"region"`
	Version  string `json:"version"`
}

// StorageClassInfo represents storage class cost information
type StorageClassInfo struct {
	Name         string  `json:"name"`
	Provider     string  `json:"provider"`
	Type         string  `json:"type"` // ssd, hdd, nvme
	PerGBMonthly float64 `json:"per_gb_monthly"`
	PerIOPS      float64 `json:"per_iops"`
	Provisioned  bool    `json:"provisioned"` // true if IOPS are provisioned
}

// CostSummary represents aggregated cost information
type CostSummary struct {
	TotalMonthlyCost float64            `json:"total_monthly_cost"`
	ByNamespace      map[string]float64 `json:"by_namespace"`
	ByStorageClass   map[string]float64 `json:"by_storage_class"`
	ByProvider       map[string]float64 `json:"by_provider"` // Multi-cloud distribution
	ByCluster        map[string]float64 `json:"by_cluster"`  // Cluster distribution
	TopExpensive     []PVCMetric        `json:"top_expensive"`
	ZombieVolumes    []PVCMetric        `json:"zombie_volumes"`
	BudgetLimit      float64            `json:"budget_limit"`  // Monthly budget cap
	ActiveAlerts     []string           `json:"active_alerts"` // Governance alerts
}

// Recommendation represents an optimization recommendation
type Recommendation struct {
	Type             string  `json:"type"` // storage_class, delete_zombie, resize, move_cloud
	PVC              string  `json:"pvc"`
	Namespace        string  `json:"namespace"`
	CurrentState     string  `json:"current_state"`
	RecommendedState string  `json:"recommended_state"`
	MonthlySavings   float64 `json:"monthly_savings"`
	Reasoning        string  `json:"reasoning"`
	Impact           string  `json:"impact"` // low, medium, high
}

// SizeGB returns the size in gigabytes
func (p *PVCMetric) SizeGB() float64 {
	return float64(p.SizeBytes) / (1024 * 1024 * 1024)
}

// UsedGB returns the used space in gigabytes
func (p *PVCMetric) UsedGB() float64 {
	return float64(p.UsedBytes) / (1024 * 1024 * 1024)
}

// UsagePercent returns the percentage of space used
func (p *PVCMetric) UsagePercent() float64 {
	if p.SizeBytes == 0 {
		return 0
	}
	return (float64(p.UsedBytes) / float64(p.SizeBytes)) * 100
}

// TotalIOPS returns total read + write IOPS
func (p *PVCMetric) TotalIOPS() float64 {
	return p.ReadIOPS + p.WriteIOPS
}

// IsZombie checks if the PVC is a zombie (unused for > 30 days)
func (p *PVCMetric) IsZombie() bool {
	if p.LastAccessedAt.IsZero() {
		return false // We don't have access data yet
	}
	daysSinceAccess := time.Since(p.LastAccessedAt).Hours() / 24
	return daysSinceAccess > 30
}

// AnnualCost returns the yearly cost
func (p *PVCMetric) AnnualCost() float64 {
	return p.MonthlyCost * 12
}
