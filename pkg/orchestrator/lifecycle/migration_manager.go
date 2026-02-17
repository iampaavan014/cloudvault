package lifecycle

import (
	"context"
	"fmt"

	"github.com/cloudvault-io/cloudvault/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// MigrationManager handles the execution of storage migrations
type MigrationManager interface {
	TriggerMigration(ctx context.Context, pvc types.PVCMetric, targetClass string, targetSize string) (string, error)
}

// ArgoMigrationManager leverages Argo Workflows for migration orchestration
type ArgoMigrationManager struct {
	dynamicClient dynamic.Interface
}

func NewArgoMigrationManager(dynamicClient dynamic.Interface) *ArgoMigrationManager {
	return &ArgoMigrationManager{dynamicClient: dynamicClient}
}

// TriggerMigration submits an Argo Workflow to move a PVC between clusters
func (m *ArgoMigrationManager) TriggerMigration(ctx context.Context, pvc types.PVCMetric, targetClass string, targetSize string) (string, error) {
	workflowGVR := schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "workflows",
	}

	workflow := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Workflow",
			"metadata": map[string]interface{}{
				"generateName": fmt.Sprintf("migration-%s-", pvc.Name),
				"namespace":    "cloudvault",
			},
			"spec": map[string]interface{}{
				"workflowTemplateRef": map[string]interface{}{
					"name": "storage-migration-template",
				},
				"arguments": map[string]interface{}{
					"parameters": []interface{}{
						map[string]interface{}{"name": "pvc-name", "value": pvc.Name},
						map[string]interface{}{"name": "src-namespace", "value": pvc.Namespace},
						map[string]interface{}{"name": "target-storage-class", "value": targetClass},
						map[string]interface{}{"name": "target-size", "value": targetSize},
						map[string]interface{}{"name": "target-tier", "value": pvc.Labels["cloudvault.io/tier"]},
					},
				},
			},
		},
	}

	result, err := m.dynamicClient.Resource(workflowGVR).Namespace("cloudvault").Create(ctx, workflow, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to submit migration workflow: %w", err)
	}

	return result.GetName(), nil
}
