package multicluster

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ClusterRegistry manages multiple Kubernetes clusters
type ClusterRegistry struct {
	clusters map[string]*ClusterInfo
	mu       sync.RWMutex
	config   *RegistryConfig
}

// ClusterInfo represents a registered cluster
type ClusterInfo struct {
	ID           string
	Name         string
	Provider     string // "aws", "gcp", "azure"
	Region       string
	KubeConfig   string
	Client       *kubernetes.Clientset
	AgentVersion string
	LastSeen     time.Time
	Status       string // "healthy", "degraded", "unreachable"
	Metrics      ClusterMetrics
	Capabilities ClusterCapabilities
	CostData     ClusterCostData
	Labels       map[string]string
}

// ClusterMetrics holds cluster-level metrics
type ClusterMetrics struct {
	TotalNodes        int
	TotalPVCs         int
	TotalStorageBytes int64
	UsedStorageBytes  int64
	AvailableCapacity int64
	IOPS              int64
	Throughput        int64
}

// ClusterCapabilities describes what a cluster can do
type ClusterCapabilities struct {
	StorageClasses    []string
	CSIDrivers        []string
	MaxPVCSize        int64
	SupportsSnapshots bool
	SupportsCloning   bool
	NetworkZone       string
}

// ClusterCostData holds cost information for a cluster
type ClusterCostData struct {
	MonthlyCost float64
	StorageCost float64
	ComputeCost float64
	NetworkCost float64
	CostPerGB   float64
	CostTrend   string // "increasing", "stable", "decreasing"
	LastUpdated time.Time
}

// RegistryConfig holds registry configuration
type RegistryConfig struct {
	DiscoveryInterval   time.Duration
	HealthCheckInterval time.Duration
	MetricsInterval     time.Duration
}

// NewClusterRegistry creates a new cluster registry
func NewClusterRegistry(config *RegistryConfig) *ClusterRegistry {
	if config == nil {
		config = &RegistryConfig{
			DiscoveryInterval:   5 * time.Minute,
			HealthCheckInterval: 1 * time.Minute,
			MetricsInterval:     30 * time.Second,
		}
	}

	registry := &ClusterRegistry{
		clusters: make(map[string]*ClusterInfo),
		config:   config,
	}

	// Start background tasks
	go registry.healthCheckLoop()
	go registry.metricsCollectionLoop()

	slog.Info("Cluster registry initialized", "discoveryInterval", config.DiscoveryInterval)
	return registry
}

// RegisterCluster adds a new cluster to the registry
func (r *ClusterRegistry) RegisterCluster(ctx context.Context, info *ClusterInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	slog.Info("Registering cluster", "id", info.ID, "name", info.Name, "provider", info.Provider)

	// Create Kubernetes client
	if info.Client == nil && info.KubeConfig != "" {
		client, err := r.createClient(info.KubeConfig)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		info.Client = client
	}

	// Discover cluster capabilities
	if err := r.discoverCapabilities(ctx, info); err != nil {
		slog.Warn("Failed to discover cluster capabilities", "cluster", info.ID, "error", err)
	}

	// Collect initial metrics
	if err := r.collectClusterMetrics(ctx, info); err != nil {
		slog.Warn("Failed to collect initial metrics", "cluster", info.ID, "error", err)
	}

	info.LastSeen = time.Now()
	info.Status = "healthy"

	r.clusters[info.ID] = info

	slog.Info("Cluster registered successfully",
		"id", info.ID,
		"nodes", info.Metrics.TotalNodes,
		"pvcs", info.Metrics.TotalPVCs,
		"storage", info.Metrics.TotalStorageBytes)

	return nil
}

// RegisterClusterFromKubeconfig registers a cluster from a kubeconfig file
func (r *ClusterRegistry) RegisterClusterFromKubeconfig(ctx context.Context, kubeconfigPath, clusterID string) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Detect cloud provider and region
	provider, region := r.detectCloudProvider(ctx, client)

	info := &ClusterInfo{
		ID:         clusterID,
		Name:       clusterID,
		Provider:   provider,
		Region:     region,
		KubeConfig: kubeconfigPath,
		Client:     client,
		Labels:     make(map[string]string),
	}

	return r.RegisterCluster(ctx, info)
}

// UnregisterCluster removes a cluster from the registry
func (r *ClusterRegistry) UnregisterCluster(clusterID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.clusters[clusterID]; !exists {
		return fmt.Errorf("cluster %s not found", clusterID)
	}

	delete(r.clusters, clusterID)
	slog.Info("Cluster unregistered", "id", clusterID)
	return nil
}

// GetCluster returns cluster information
func (r *ClusterRegistry) GetCluster(clusterID string) (*ClusterInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cluster, exists := r.clusters[clusterID]
	if !exists {
		return nil, fmt.Errorf("cluster %s not found", clusterID)
	}

	return cluster, nil
}

// GetAllClusters returns all registered clusters
func (r *ClusterRegistry) GetAllClusters() []*ClusterInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clusters := make([]*ClusterInfo, 0, len(r.clusters))
	for _, cluster := range r.clusters {
		clusters = append(clusters, cluster)
	}

	return clusters
}

// GetHealthyClusters returns only healthy clusters
func (r *ClusterRegistry) GetHealthyClusters() []*ClusterInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var healthy []*ClusterInfo
	for _, cluster := range r.clusters {
		if cluster.Status == "healthy" {
			healthy = append(healthy, cluster)
		}
	}

	return healthy
}

// GetOptimalCluster finds the best cluster for a workload
func (r *ClusterRegistry) GetOptimalCluster(workload WorkloadPlacementRequest) (*ClusterInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	slog.Info("Finding optimal cluster for workload", "workload", workload.Name)

	var bestCluster *ClusterInfo
	var bestScore float64

	for _, cluster := range r.clusters {
		// Filter by constraints
		if !r.meetsConstraints(cluster, workload.Constraints) {
			continue
		}

		// Calculate placement score
		score := r.calculatePlacementScore(cluster, workload)

		slog.Debug("Cluster score",
			"cluster", cluster.ID,
			"score", score,
			"cost", cluster.CostData.CostPerGB)

		if score > bestScore {
			bestScore = score
			bestCluster = cluster
		}
	}

	if bestCluster == nil {
		return nil, fmt.Errorf("no suitable cluster found for workload")
	}

	slog.Info("Optimal cluster selected",
		"cluster", bestCluster.ID,
		"score", bestScore,
		"monthlyCost", bestCluster.CostData.MonthlyCost)

	return bestCluster, nil
}

// WorkloadPlacementRequest describes workload placement requirements
type WorkloadPlacementRequest struct {
	Name              string
	StorageSize       int64
	IOPS              int64
	Throughput        int64
	RequiredLatency   time.Duration
	DataGravity       []string // List of clusters this workload communicates with
	ComplianceRegions []string // Allowed regions for data residency
	Constraints       PlacementConstraints
}

// PlacementConstraints defines hard constraints for placement
type PlacementConstraints struct {
	RequiredProvider    string   // Must be this provider
	RequiredRegion      string   // Must be in this region
	AllowedClusters     []string // Whitelist of clusters
	ForbiddenClusters   []string // Blacklist of clusters
	RequireCapabilities []string // Required features
	MaxCostPerGB        float64  // Maximum acceptable cost
}

// calculatePlacementScore calculates a score for placing workload on cluster
func (r *ClusterRegistry) calculatePlacementScore(cluster *ClusterInfo, workload WorkloadPlacementRequest) float64 {
	score := 0.0

	// Factor 1: Cost (weight: 50%)
	if cluster.CostData.CostPerGB > 0 {
		// Lower cost = higher score
		costScore := 1.0 / (1.0 + cluster.CostData.CostPerGB)
		score += costScore * 0.5
	}

	// Factor 2: Data gravity (weight: 30%)
	// Prefer clusters near data dependencies
	if len(workload.DataGravity) > 0 {
		gravityScore := 0.0
		for _, depCluster := range workload.DataGravity {
			if depCluster == cluster.ID {
				gravityScore = 1.0 // Same cluster = best
			} else if dep, err := r.GetCluster(depCluster); err == nil {
				if dep.Provider == cluster.Provider && dep.Region == cluster.Region {
					gravityScore = 0.8 // Same region = good
				} else if dep.Provider == cluster.Provider {
					gravityScore = 0.5 // Same provider = okay
				}
			}
		}
		score += gravityScore * 0.3
	} else {
		score += 0.3 // No dependencies, full score
	}

	// Factor 3: Available capacity (weight: 10%)
	if cluster.Metrics.AvailableCapacity > workload.StorageSize {
		capacityScore := 1.0
		score += capacityScore * 0.1
	}

	// Factor 4: Performance (weight: 10%)
	if cluster.Metrics.IOPS >= workload.IOPS {
		perfScore := 1.0
		score += perfScore * 0.1
	}

	return score
}

// meetsConstraints checks if cluster meets hard constraints
func (r *ClusterRegistry) meetsConstraints(cluster *ClusterInfo, constraints PlacementConstraints) bool {
	// Check provider constraint
	if constraints.RequiredProvider != "" && cluster.Provider != constraints.RequiredProvider {
		return false
	}

	// Check region constraint
	if constraints.RequiredRegion != "" && cluster.Region != constraints.RequiredRegion {
		return false
	}

	// Check whitelist
	if len(constraints.AllowedClusters) > 0 {
		allowed := false
		for _, id := range constraints.AllowedClusters {
			if cluster.ID == id {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	// Check blacklist
	for _, id := range constraints.ForbiddenClusters {
		if cluster.ID == id {
			return false
		}
	}

	// Check max cost
	if constraints.MaxCostPerGB > 0 && cluster.CostData.CostPerGB > constraints.MaxCostPerGB {
		return false
	}

	// Check required capabilities
	for _, reqCap := range constraints.RequireCapabilities {
		hasCapability := false
		for _, clusterCap := range cluster.Capabilities.StorageClasses {
			if clusterCap == reqCap {
				hasCapability = true
				break
			}
		}
		if !hasCapability {
			return false
		}
	}

	return true
}

// GetCrossClusterCostAggregation aggregates costs across all clusters
func (r *ClusterRegistry) GetCrossClusterCostAggregation() CostAggregation {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agg := CostAggregation{
		ByProvider: make(map[string]float64),
		ByRegion:   make(map[string]float64),
		ByCluster:  make(map[string]float64),
	}

	for _, cluster := range r.clusters {
		agg.TotalCost += cluster.CostData.MonthlyCost
		agg.StorageCost += cluster.CostData.StorageCost
		agg.ComputeCost += cluster.CostData.ComputeCost
		agg.NetworkCost += cluster.CostData.NetworkCost

		agg.ByProvider[cluster.Provider] += cluster.CostData.MonthlyCost
		agg.ByRegion[cluster.Region] += cluster.CostData.MonthlyCost
		agg.ByCluster[cluster.ID] = cluster.CostData.MonthlyCost
	}

	return agg
}

// CostAggregation holds aggregated cost data
type CostAggregation struct {
	TotalCost   float64
	StorageCost float64
	ComputeCost float64
	NetworkCost float64
	ByProvider  map[string]float64
	ByRegion    map[string]float64
	ByCluster   map[string]float64
}

// UpdateClusterHeartbeat updates the last seen timestamp
func (r *ClusterRegistry) UpdateClusterHeartbeat(clusterID string, agentVersion string, metrics *types.StorageMetrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cluster, exists := r.clusters[clusterID]
	if !exists {
		// Auto-register cluster if it doesn't exist
		slog.Info("Auto-registering cluster from heartbeat", "id", clusterID)
		cluster = &ClusterInfo{
			ID:     clusterID,
			Name:   clusterID,
			Labels: make(map[string]string),
		}
		r.clusters[clusterID] = cluster
	}

	cluster.LastSeen = time.Now()
	cluster.AgentVersion = agentVersion
	cluster.Status = "healthy"

	// Update metrics from heartbeat
	if metrics != nil {
		cluster.Metrics.TotalPVCs = len(metrics.PVCs)
		cluster.Metrics.TotalStorageBytes = metrics.TotalStorageBytes
		cluster.Metrics.UsedStorageBytes = metrics.UsedStorageBytes
	}

	return nil
}

// Helper methods

func (r *ClusterRegistry) createClient(kubeconfigPath string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func (r *ClusterRegistry) detectCloudProvider(ctx context.Context, client *kubernetes.Clientset) (string, string) {
	// Try to detect from nodes
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil || len(nodes.Items) == 0 {
		return "unknown", "unknown"
	}

	node := nodes.Items[0]

	// Check provider ID
	if providerID := node.Spec.ProviderID; providerID != "" {
		if len(providerID) > 3 {
			provider := providerID[:3]
			switch provider {
			case "aws":
				return "aws", r.extractAWSRegion(providerID)
			case "gce":
				return "gcp", r.extractGCPRegion(providerID)
			case "azu":
				return "azure", r.extractAzureRegion(providerID)
			}
		}
	}

	// Fallback: check labels
	if region := node.Labels["topology.kubernetes.io/region"]; region != "" {
		return "unknown", region
	}

	return "unknown", "unknown"
}

func (r *ClusterRegistry) extractAWSRegion(providerID string) string {
	// Format: aws:///us-east-1a/i-0123456789abcdef
	// Extract region from availability zone
	return "us-east-1" // TODO: Parse from providerID
}

func (r *ClusterRegistry) extractGCPRegion(providerID string) string {
	return "us-central1" // TODO: Parse from providerID
}

func (r *ClusterRegistry) extractAzureRegion(providerID string) string {
	return "eastus" // TODO: Parse from providerID
}

func (r *ClusterRegistry) discoverCapabilities(ctx context.Context, info *ClusterInfo) error {
	if info.Client == nil {
		return fmt.Errorf("client not initialized")
	}

	// Get storage classes
	scList, err := info.Client.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	info.Capabilities.StorageClasses = make([]string, len(scList.Items))
	for i, sc := range scList.Items {
		info.Capabilities.StorageClasses[i] = sc.Name
		info.Capabilities.CSIDrivers = append(info.Capabilities.CSIDrivers, sc.Provisioner)
	}

	// TODO: Check for snapshot support, cloning support, etc.
	info.Capabilities.SupportsSnapshots = true
	info.Capabilities.SupportsCloning = true

	return nil
}

func (r *ClusterRegistry) collectClusterMetrics(ctx context.Context, info *ClusterInfo) error {
	if info.Client == nil {
		return fmt.Errorf("client not initialized")
	}

	// Count nodes
	nodes, err := info.Client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err == nil {
		info.Metrics.TotalNodes = len(nodes.Items)
	}

	// Count PVCs and calculate storage
	pvcs, err := info.Client.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	if err == nil {
		info.Metrics.TotalPVCs = len(pvcs.Items)

		var totalStorage int64
		for _, pvc := range pvcs.Items {
			if storage := pvc.Spec.Resources.Requests.Storage(); storage != nil {
				totalStorage += storage.Value()
			}
		}
		info.Metrics.TotalStorageBytes = totalStorage
	}

	return nil
}

// Background loops

func (r *ClusterRegistry) healthCheckLoop() {
	ticker := time.NewTicker(r.config.HealthCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		r.performHealthChecks()
	}
}

func (r *ClusterRegistry) performHealthChecks() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for _, cluster := range r.clusters {
		// Mark as unreachable if no heartbeat in 5 minutes
		if now.Sub(cluster.LastSeen) > 5*time.Minute {
			if cluster.Status != "unreachable" {
				slog.Warn("Cluster unreachable", "id", cluster.ID, "lastSeen", cluster.LastSeen)
				cluster.Status = "unreachable"
			}
		}
	}
}

func (r *ClusterRegistry) metricsCollectionLoop() {
	ticker := time.NewTicker(r.config.MetricsInterval)
	defer ticker.Stop()

	ctx := context.Background()
	for range ticker.C {
		r.collectMetricsForAllClusters(ctx)
	}
}

func (r *ClusterRegistry) collectMetricsForAllClusters(ctx context.Context) {
	clusters := r.GetHealthyClusters()

	for _, cluster := range clusters {
		if err := r.collectClusterMetrics(ctx, cluster); err != nil {
			slog.Warn("Failed to collect metrics", "cluster", cluster.ID, "error", err)
		}
	}
}
