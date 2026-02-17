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
			SET pvc.storage_class = item.storageClass, pvc.size_bytes = item.sizeBytes
			MERGE (pvc)-[:BELONGS_TO]->(ns)
		`
		var batch []map[string]interface{}
		for _, m := range metrics {
			batch = append(batch, map[string]interface{}{
				"namespace":    m.Namespace,
				"name":         m.Name,
				"storageClass": m.StorageClass,
				"sizeBytes":    m.SizeBytes,
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

// MapPodToPVC creates a connection between a Pod and its used PVC
func (s *SIG) MapPodToPVC(ctx context.Context, podName, namespace, pvcName string) error {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer func() { _ = session.Close(ctx) }()

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			MATCH (pvc:PVC {name: $pvcName, namespace: $namespace})
			MERGE (pod:Pod {name: $podName, namespace: $namespace})
			MERGE (pod)-[:USES]->(pvc)
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
			RETURN pvc.namespace + "/" + pvc.name AS pvc_id
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

func (s *SIG) Close(ctx context.Context) error {
	return s.driver.Close(ctx)
}
