package collector

import (
	"context"
	"fmt"

	"github.com/cloudvault-io/cloudvault/pkg/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/cloudvault-io/cloudvault/pkg/types/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// KubernetesClient wraps the Kubernetes clientset with CloudVault-specific logic
type KubernetesClient struct {
	clientset *kubernetes.Clientset
	dynamic   dynamic.Interface
	config    *rest.Config
}

// NewKubernetesClient creates a new Kubernetes client
// It supports both in-cluster (uses service account) and out-of-cluster (uses kubeconfig) modes
func NewKubernetesClient(kubeconfig string) (*KubernetesClient, error) {
	var config *rest.Config
	var err error

	if kubeconfig != "" {
		// Out-of-cluster: Use kubeconfig file
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
		}
	} else {
		// In-cluster: Use service account
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get in-cluster config (are you running outside k8s? try --kubeconfig): %w", err)
		}
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	// Create dynamic client
	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &KubernetesClient{
		clientset: clientset,
		dynamic:   dynClient,
		config:    config,
	}, nil
}

// GetClusterInfo retrieves cluster metadata including cloud provider detection
func (k *KubernetesClient) GetClusterInfo(ctx context.Context) (*types.ClusterInfo, error) {
	// Get Kubernetes version
	version, err := k.clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get server version: %w", err)
	}

	// Get first node to detect cloud provider
	nodes, err := k.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	provider := "unknown"
	region := "unknown"
	clusterName := "default"

	if len(nodes.Items) > 0 {
		node := nodes.Items[0]

		// Detect cloud provider from node labels
		provider, region = detectCloudProvider(node.Labels)

		// Try to get cluster name from node labels
		if name, ok := node.Labels["alpha.eksctl.io/cluster-name"]; ok {
			clusterName = name
		} else if name, ok := node.Labels["cloud.google.com/gke-cluster-name"]; ok {
			clusterName = name
		} else if name, ok := node.Labels["kubernetes.azure.com/cluster"]; ok {
			clusterName = name
		}
	}

	// Create cluster ID
	clusterID := fmt.Sprintf("%s-%s-%s", provider, region, clusterName)

	return &types.ClusterInfo{
		ID:       clusterID,
		Name:     clusterName,
		Provider: provider,
		Region:   region,
		Version:  version.GitVersion,
	}, nil
}

// detectCloudProvider attempts to determine the cloud provider and region
// by inspecting the labels on a cluster node. It supports AWS, GCP, and Azure.
func detectCloudProvider(labels map[string]string) (provider, region string) {
	provider = "unknown"
	region = "unknown"

	// AWS EKS detection
	if _, ok := labels["eks.amazonaws.com/nodegroup"]; ok {
		provider = "aws"
		if r, ok := labels["topology.kubernetes.io/region"]; ok {
			region = r
		} else if r, ok := labels["failure-domain.beta.kubernetes.io/region"]; ok {
			region = r
		}
		return
	}

	// GCP GKE detection
	if _, ok := labels["cloud.google.com/gke-nodepool"]; ok {
		provider = "gcp"
		if r, ok := labels["topology.kubernetes.io/region"]; ok {
			region = r
		} else if r, ok := labels["failure-domain.beta.kubernetes.io/region"]; ok {
			region = r
		}
		return
	}

	// Azure AKS detection
	if _, ok := labels["kubernetes.azure.com/cluster"]; ok {
		provider = "azure"
		if r, ok := labels["topology.kubernetes.io/region"]; ok {
			region = r
		} else if r, ok := labels["failure-domain.beta.kubernetes.io/region"]; ok {
			region = r
		}
		return
	}

	// Check for generic cloud provider label
	if p, ok := labels["cloud.google.com/provider"]; ok {
		provider = p
	} else if p, ok := labels["node.kubernetes.io/instance-type"]; ok {
		// Try to infer from instance type
		if len(p) > 0 {
			switch p[0] {
			case 't', 'm', 'c', 'r', 'i', 'x': // AWS instance prefixes
				provider = "aws"
			case 'n', 'e': // GCP instance prefixes
				provider = "gcp"
			case 'S': // Azure instance prefixes (Standard_*)
				provider = "azure"
			}
		}
	}

	// Get region from standard topology label if not found yet
	if region == "unknown" {
		if r, ok := labels["topology.kubernetes.io/region"]; ok {
			region = r
		}
	}

	return provider, region
}

// GetClientset returns the underlying Kubernetes clientset
func (k *KubernetesClient) GetClientset() *kubernetes.Clientset {
	return k.clientset
}

// GetDynamicClient returns the underlying dynamic client
func (k *KubernetesClient) GetDynamicClient() dynamic.Interface {
	return k.dynamic
}

// ListStoragePolicies fetches all StorageLifecyclePolicy resources across all namespaces
func (k *KubernetesClient) ListStoragePolicies(ctx context.Context) ([]v1alpha1.StorageLifecyclePolicy, error) {
	gvr := schema.GroupVersionResource{
		Group:    "cloudvault.io",
		Version:  "v1alpha1",
		Resource: "storagelifecyclepolicies",
	}

	list, err := k.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list storage policies: %w", err)
	}

	var policies []v1alpha1.StorageLifecyclePolicy
	for _, item := range list.Items {
		var policy v1alpha1.StorageLifecyclePolicy
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &policy)
		if err != nil {
			return nil, fmt.Errorf("failed to convert unstructured to policy: %w", err)
		}
		policies = append(policies, policy)
	}

	return policies, nil
}

// ListCostPolicies fetches all CostPolicy resources across all namespaces
func (k *KubernetesClient) ListCostPolicies(ctx context.Context) ([]v1alpha1.CostPolicy, error) {
	gvr := schema.GroupVersionResource{
		Group:    "cloudvault.io",
		Version:  "v1alpha1",
		Resource: "costpolicies",
	}

	list, err := k.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list cost policies: %w", err)
	}

	var policies []v1alpha1.CostPolicy
	for _, item := range list.Items {
		var policy v1alpha1.CostPolicy
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &policy)
		if err != nil {
			return nil, fmt.Errorf("failed to convert unstructured to policy: %w", err)
		}
		policies = append(policies, policy)
	}

	return policies, nil
}

// ListPods fetches all pods across all namespaces
func (k *KubernetesClient) ListPods(ctx context.Context) (*corev1.PodList, error) {
	pods, err := k.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}
	return pods, nil
}
