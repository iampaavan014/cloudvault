package collector

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cloudvault-io/cloudvault/pkg/integrations"
	"github.com/cloudvault-io/cloudvault/pkg/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Collector defines the interface for fetching PVC metrics
type Collector interface {
	CollectAll(ctx context.Context) ([]types.PVCMetric, error)
	CollectByNamespace(ctx context.Context, namespace string) ([]types.PVCMetric, error)
}

// PVCCollector handles the collection of PersistentVolumeClaim metrics from a Kubernetes cluster.
// It supports both full cluster collection and namespace-scoped collection.
type PVCCollector struct {
	client         *KubernetesClient
	promClient     *integrations.PrometheusClient
	egressProvider EgressProvider
}

// NewPVCCollector creates a new instance of PVCCollector.
// client: An initialized KubernetesClient for API interaction.
// promClient: Optional PrometheusClient for fetching real-time usage metrics.
func NewPVCCollector(client *KubernetesClient, promClient *integrations.PrometheusClient) *PVCCollector {
	return &PVCCollector{
		client:     client,
		promClient: promClient,
	}
}

// CollectAll collects metrics for all PVCs in the cluster using concurrent workers.
// Refactored in Phase 3 to use batch Prometheus queries and background patterns.
func (c *PVCCollector) CollectAll(ctx context.Context) ([]types.PVCMetric, error) {
	// Get cluster info first (needed for all metrics)
	clusterInfo, err := c.client.GetClusterInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster info: %w", err)
	}

	// List all PVCs across all namespaces
	pvcs, err := c.client.clientset.CoreV1().
		PersistentVolumeClaims("").
		List(ctx, metav1.ListOptions{})

	if err != nil {
		return nil, fmt.Errorf("failed to list PVCs: %w", err)
	}

	// Fetch all pods for SIG (Storage Intelligence Graph) Pillar (Phase 4)
	pods, err := c.client.ListPods(ctx)
	if err != nil {
		slog.Warn("failed to list pods for intelligence graph", "error", err)
	}

	pvcToPods := make(map[string][]string)
	if pods != nil {
		for _, pod := range pods.Items {
			for _, vol := range pod.Spec.Volumes {
				if vol.PersistentVolumeClaim != nil {
					key := fmt.Sprintf("%s/%s", pod.Namespace, vol.PersistentVolumeClaim.ClaimName)
					pvcToPods[key] = append(pvcToPods[key], pod.Name)
				}
			}
		}
	}

	// Fetch all Prometheus metrics in ONE batch query (Phase 3 optimization)
	var batchMetrics map[string]map[string]*integrations.PVCUsageMetrics
	if c.promClient != nil {
		batchMetrics, _ = c.promClient.GetAllPVCMetrics(ctx)
	}

	// Fetch all Egress metrics in ONE batch (Phase 9 optimization)
	var egressData map[string]uint64
	if c.egressProvider != nil {
		egressData, _ = c.egressProvider.GetEgressBytes(ctx)
	}

	// Adaptive Worker pool settings (Phase 9 optimization)
	numPVCs := len(pvcs.Items)
	numWorkers := 10
	if numPVCs > 0 {
		// Scales from 5 to 50 workers based on volume
		numWorkers = numPVCs / 20
		if numWorkers < 5 {
			numWorkers = 5
		}
		if numWorkers > 50 {
			numWorkers = 50
		}
	}

	if numPVCs == 0 {
		return []types.PVCMetric{}, nil
	}

	// Channels for jobs and results
	jobs := make(chan *corev1.PersistentVolumeClaim, numPVCs)
	results := make(chan *types.PVCMetric, numPVCs)
	// errors := make(chan error, numPVCs) // No longer needed for this impl

	// Start workers
	for i := 0; i < numWorkers; i++ {
		go func() {
			for pvc := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				metric := c.initializePVCMetric(pvc, clusterInfo)

				// Apply SIG data (Phase 4 Pillar 1)
				key := fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name)
				if pods, ok := pvcToPods[key]; ok {
					metric.MountedPods = pods
				}

				// Apply Prometheus data from batch if available
				if batchMetrics != nil {
					if nsMetrics, ok := batchMetrics[pvc.Namespace]; ok {
						if m, ok := nsMetrics[pvc.Name]; ok {
							metric.UsedBytes = m.UsedBytes
							metric.LastAccessedAt = m.LastActivity
						}
					}
				}

				// Apply hyper-accurate egress data from pre-fetched batch (Phase 9 optimization)
				if egressData != nil {
					CorrelateEgress([]types.PVCMetric{*metric}, egressData)
				}

				results <- metric
			}
		}()
	}

	// Enqueue jobs
	for i := range pvcs.Items {
		jobs <- &pvcs.Items[i]
	}
	close(jobs)

	// Collect results
	var metrics []types.PVCMetric
	for i := 0; i < numPVCs; i++ {
		select {
		case metric := <-results:
			metrics = append(metrics, *metric)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	slog.Info("Collection complete", "count", len(metrics), "numPVCs", numPVCs)
	return metrics, nil
}

// CollectByNamespace collects metrics for PVCs in a specific namespace
func (c *PVCCollector) CollectByNamespace(ctx context.Context, namespace string) ([]types.PVCMetric, error) {
	clusterInfo, err := c.client.GetClusterInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster info: %w", err)
	}

	pvcs, err := c.client.clientset.CoreV1().
		PersistentVolumeClaims(namespace).
		List(ctx, metav1.ListOptions{})

	if err != nil {
		return nil, fmt.Errorf("failed to list PVCs in namespace %s: %w", namespace, err)
	}

	var metrics []types.PVCMetric

	for _, pvc := range pvcs.Items {
		metric := c.initializePVCMetric(&pvc, clusterInfo)
		metrics = append(metrics, *metric)
	}

	return metrics, nil
}

// initializePVCMetric creates a base metric from PVC spec
func (c *PVCCollector) initializePVCMetric(pvc *corev1.PersistentVolumeClaim,
	clusterInfo *types.ClusterInfo) *types.PVCMetric {

	// Get storage size from spec
	sizeBytes := int64(0)
	if pvc.Spec.Resources.Requests != nil {
		if storage, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			sizeBytes = storage.Value()
		}
	}

	// Get storage class name
	storageClass := ""
	if pvc.Spec.StorageClassName != nil {
		storageClass = *pvc.Spec.StorageClassName
	}

	// Create metric
	metric := &types.PVCMetric{
		Name:         pvc.Name,
		Namespace:    pvc.Namespace,
		ClusterID:    clusterInfo.ID,
		Provider:     clusterInfo.Provider,
		Region:       clusterInfo.Region,
		StorageClass: storageClass,
		SizeBytes:    sizeBytes,
		UsedBytes:    0,
		CreatedAt:    pvc.CreationTimestamp.Time,
		Labels:       pvc.Labels,
		Annotations:  pvc.Annotations,
	}

	// Initialize maps if nil
	if metric.Labels == nil {
		metric.Labels = make(map[string]string)
	}
	if metric.Annotations == nil {
		metric.Annotations = make(map[string]string)
	}

	return metric
}

// GetPVCCount returns the total number of PVCs in the cluster
func (c *PVCCollector) GetPVCCount(ctx context.Context) (int, error) {
	pvcs, err := c.client.clientset.CoreV1().
		PersistentVolumeClaims("").
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to count PVCs: %w", err)
	}
	return len(pvcs.Items), nil
}

// GetPVCsByStorageClass returns all PVCs using a specific storage class
func (c *PVCCollector) GetPVCsByStorageClass(ctx context.Context, storageClass string) ([]types.PVCMetric, error) {
	allMetrics, err := c.CollectAll(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []types.PVCMetric
	for _, m := range allMetrics {
		if m.StorageClass == storageClass {
			filtered = append(filtered, m)
		}
	}

	return filtered, nil
}

// GetNamespaces returns all namespaces that have PVCs
func (c *PVCCollector) GetNamespaces(ctx context.Context) ([]string, error) {
	pvcs, err := c.client.clientset.CoreV1().
		PersistentVolumeClaims("").
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list PVCs: %w", err)
	}

	nsMap := make(map[string]bool)
	for _, pvc := range pvcs.Items {
		nsMap[pvc.Namespace] = true
	}

	namespaces := make([]string, 0, len(nsMap))
	for ns := range nsMap {
		namespaces = append(namespaces, ns)
	}

	return namespaces, nil
}
