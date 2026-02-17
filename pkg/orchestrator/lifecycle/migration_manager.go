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

// MigrationManager defines the interface for executing storage migrations
type MigrationManager interface {
	TriggerMigration(ctx context.Context, pvc types.PVCMetric, targetClass string) (string, error)
}

// ArgoMigrationManager handles submission of Argo workflows
type ArgoMigrationManager struct {
	dynamicClient dynamic.Interface
}

func NewArgoMigrationManager(client dynamic.Interface) *ArgoMigrationManager {
	return &ArgoMigrationManager{
		dynamicClient: client,
	}
}

// TriggerMigration submits an Argo Workflow to move a PVC between clusters
func (m *ArgoMigrationManager) TriggerMigration(ctx context.Context, pvc types.PVCMetric, targetClass string) (string, error) {
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
