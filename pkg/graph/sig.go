package graph

import (
	"context"
	"fmt"

	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// SIG represents the Storage Intelligence Graph
type SIG struct {
	driver neo4j.DriverWithContext
}

// NewSIG creates a new Storage Intelligence Graph client
func NewSIG(uri, username, password string) (*SIG, error) {
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(username, password, ""))
	if err != nil {
		return nil, fmt.Errorf("failed to create neo4j driver: %w", err)
	}

	// Verify connectivity (Phase 9 improvement)
	ctx := context.Background()
	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, fmt.Errorf("failed to verify neo4j connectivity: %w", err)
	}

	return &SIG{driver: driver}, nil
}

// SyncPVCs creates or updates multiple PVC nodes and their relationships (Phase 9 optimization)
func (s *SIG) SyncPVCs(ctx context.Context, metrics []types.PVCMetric) error {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer func() { _ = session.Close(ctx) }()

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			UNWIND $batch AS item
			MERGE (ns:Namespace {name: item.namespace})
			MERGE (pvc:PVC {name: item.name, namespace: item.namespace})
			ON CREATE SET pvc.created_at = timestamp()
			SET pvc.storage_class = item.storageClass, 
			    pvc.size_bytes = item.sizeBytes,
			    pvc.used_bytes = item.usedBytes,
			    pvc.region = item.region,
			    pvc.provider = item.provider,
			    pvc.cluster_id = item.clusterID,
			    pvc.monthly_cost = item.monthlyCost,
			    pvc.read_iops = item.readIOPS,
			    pvc.write_iops = item.writeIOPS
			MERGE (pvc)-[:BELONGS_TO]->(ns)
			
			// Create cluster and region relationships
			WITH pvc, item
			WHERE item.clusterID IS NOT NULL
			MERGE (cluster:Cluster {id: item.clusterID})
			SET cluster.provider = item.provider, cluster.region = item.region
			MERGE (pvc)-[:RUNS_IN]->(cluster)
			
			WITH pvc, item, cluster
			WHERE item.region IS NOT NULL
			MERGE (region:Region {name: item.region, provider: item.provider})
			MERGE (cluster)-[:LOCATED_IN]->(region)
		`
		var batch []map[string]interface{}
		for _, m := range metrics {
			batch = append(batch, map[string]interface{}{
				"namespace":    m.Namespace,
				"name":         m.Name,
				"storageClass": m.StorageClass,
				"sizeBytes":    m.SizeBytes,
				"usedBytes":    m.UsedBytes,
				"region":       m.Region,
				"provider":     m.Provider,
				"clusterID":    m.ClusterID,
				"monthlyCost":  m.MonthlyCost,
				"readIOPS":     m.ReadIOPS,
				"writeIOPS":    m.WriteIOPS,
			})
		}

		result, err := tx.Run(ctx, query, map[string]interface{}{"batch": batch})
		if err != nil {
			return nil, err
		}
		return result.Consume(ctx)
	})
	return err
}

// SyncPVC creates or updates a single PVC node
func (s *SIG) SyncPVC(ctx context.Context, namespace, name, storageClass string, sizeBytes int64) error {
	return s.SyncPVCs(ctx, []types.PVCMetric{{
		Namespace:    namespace,
		Name:         name,
		StorageClass: storageClass,
		SizeBytes:    sizeBytes,
	}})
}

// MapPodToPVC creates a connection between a Pod and its used PVC with region tracking
func (s *SIG) MapPodToPVC(ctx context.Context, podName, namespace, pvcName string) error {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer func() { _ = session.Close(ctx) }()

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			MATCH (pvc:PVC {name: $pvcName, namespace: $namespace})
			MERGE (pod:Pod {name: $podName, namespace: $namespace})
			MERGE (pod)-[:USES]->(pvc)
			
			// Copy region info from PVC to Pod for gravity analysis
			WITH pod, pvc
			WHERE pvc.region IS NOT NULL
			SET pod.region = pvc.region, pod.provider = pvc.provider
		`
		params := map[string]interface{}{
			"podName":   podName,
			"namespace": namespace,
			"pvcName":   pvcName,
		}
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		return result.Consume(ctx)
	})
	return err
}

// GetCrossRegionGravity finds PVCs whose Pods are in a different region than the Storage
func (s *SIG) GetCrossRegionGravity(ctx context.Context) ([]string, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer func() { _ = session.Close(ctx) }()

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			MATCH (pod:Pod)-[:USES]->(pvc:PVC)
			WHERE pod.region IS NOT NULL AND pvc.region IS NOT NULL AND pod.region <> pvc.region
			RETURN pvc.namespace + "/" + pvc.name AS pvc_id,
			       pod.region AS pod_region,
			       pvc.region AS pvc_region,
			       pvc.monthly_cost AS cost
			ORDER BY cost DESC
		`
		res, err := tx.Run(ctx, query, nil)
		if err != nil {
			return nil, err
		}

		var pvcs []string
		for res.Next(ctx) {
			record := res.Record()
			if val, ok := record.Get("pvc_id"); ok {
				pvcs = append(pvcs, val.(string))
			}
		}
		return pvcs, nil
	})

	if err != nil {
		return nil, err
	}
	return result.([]string), nil
}

// GetCrossCloudWorkloads identifies workloads that communicate across cloud providers
func (s *SIG) GetCrossCloudWorkloads(ctx context.Context) ([]CrossCloudWorkload, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer func() { _ = session.Close(ctx) }()

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			MATCH (pod1:Pod)-[:COMMUNICATES_WITH]->(pod2:Pod)
			WHERE pod1.provider IS NOT NULL AND pod2.provider IS NOT NULL 
			  AND pod1.provider <> pod2.provider
			RETURN pod1.name AS source_pod,
			       pod1.namespace AS source_namespace,
			       pod1.provider AS source_provider,
			       pod2.name AS target_pod,
			       pod2.namespace AS target_namespace,
			       pod2.provider AS target_provider
		`
		res, err := tx.Run(ctx, query, nil)
		if err != nil {
			return nil, err
		}
		var workloads []CrossCloudWorkload
		for res.Next(ctx) {
			record := res.Record()
			workload := CrossCloudWorkload{
				SourcePod:       record.Values[0].(string),
				SourceNamespace: record.Values[1].(string),
				SourceProvider:  record.Values[2].(string),
				TargetPod:       record.Values[3].(string),
				TargetNamespace: record.Values[4].(string),
				TargetProvider:  record.Values[5].(string),
			}
			workloads = append(workloads, workload)
		}
		return workloads, nil
	})
	if err != nil {
		return nil, err
	}
	return result.([]CrossCloudWorkload), nil
}

// GetStorageClassUtilization returns utilization stats by storage class
func (s *SIG) GetStorageClassUtilization(ctx context.Context) ([]StorageClassStats, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer func() { _ = session.Close(ctx) }()

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			MATCH (pvc:PVC)
			WHERE pvc.storage_class IS NOT NULL
			RETURN pvc.storage_class AS storage_class,
			       count(pvc) AS pvc_count,
			       sum(pvc.size_bytes) AS total_size,
			       sum(pvc.used_bytes) AS total_used,
			       sum(pvc.monthly_cost) AS total_cost,
			       avg(toFloat(pvc.used_bytes) / toFloat(pvc.size_bytes)) * 100 AS avg_utilization
			ORDER BY total_cost DESC
		`
		res, err := tx.Run(ctx, query, nil)
		if err != nil {
			return nil, err
		}
		var stats []StorageClassStats
		for res.Next(ctx) {
			record := res.Record()
			stat := StorageClassStats{
				StorageClass:     record.Values[0].(string),
				PVCCount:         record.Values[1].(int64),
				TotalSizeBytes:   record.Values[2].(int64),
				TotalUsedBytes:   record.Values[3].(int64),
				TotalMonthlyCost: record.Values[4].(float64),
				AvgUtilization:   record.Values[5].(float64),
			}
			stats = append(stats, stat)
		}
		return stats, nil
	})
	if err != nil {
		return nil, err
	}
	return result.([]StorageClassStats), nil
}

// DeletePVC removes a PVC and its relationships from the graph
func (s *SIG) DeletePVC(ctx context.Context, namespace, name string) error {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer func() { _ = session.Close(ctx) }()

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			MATCH (pvc:PVC {name: $name, namespace: $namespace})
			DETACH DELETE pvc
		`
		params := map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		}
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		return result.Consume(ctx)
	})
	return err
}

func (s *SIG) Close(ctx context.Context) error {
	return s.driver.Close(ctx)
}

// CrossCloudWorkload represents a workload that spans multiple cloud providers
type CrossCloudWorkload struct {
	SourcePod       string
	SourceNamespace string
	SourceProvider  string
	TargetPod       string
	TargetNamespace string
	TargetProvider  string
}

// StorageClassStats represents utilization statistics for a storage class
type StorageClassStats struct {
	StorageClass     string
	PVCCount         int64
	TotalSizeBytes   int64
	TotalUsedBytes   int64
	TotalMonthlyCost float64
	AvgUtilization   float64
}
