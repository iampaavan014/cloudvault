package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StorageLifecyclePolicySpec defines the desired state of StorageLifecyclePolicy
type StorageLifecyclePolicySpec struct {
	// Selector matches PVCs that this policy applies to
	Selector PolicySelector `json:"selector" yaml:"selector"`

	// Tiers define the storage stages for managed PVCs
	Tiers []StorageTier `json:"tiers" yaml:"tiers"`

	// AutoDelete specifies if the volume should be deleted after the final tier
	AutoDelete bool `json:"autoDelete,omitempty" yaml:"autoDelete,omitempty"`
}

// PolicySelector matches PVCs by labels or namespaces
type PolicySelector struct {
	MatchLabels     map[string]string `json:"matchLabels,omitempty" yaml:"matchLabels,omitempty"`
	MatchNamespaces []string          `json:"matchNamespaces,omitempty" yaml:"matchNamespaces,omitempty"`
}

// StorageTier represents a stage in the volume lifecycle
type StorageTier struct {
	Name         string `json:"name" yaml:"name"`                 // e.g., "hot", "warm", "cold"
	StorageClass string `json:"storageClass" yaml:"storageClass"` // e.g., "gp3", "standard"
	Duration     string `json:"duration" yaml:"duration"`         // e.g., "30d", "7d"
}

// StorageLifecyclePolicyStatus defines the observed state of StorageLifecyclePolicy
type StorageLifecyclePolicyStatus struct {
	ManagedPVCs  int      `json:"managedPVCs" yaml:"managedPVCs"`
	ActiveAlerts []string `json:"activeAlerts,omitempty" yaml:"activeAlerts,omitempty"`
}

// StorageLifecyclePolicy is the Schema for the storagelifecyclepolicies API
type StorageLifecyclePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StorageLifecyclePolicySpec   `json:"spec,omitempty"`
	Status StorageLifecyclePolicyStatus `json:"status,omitempty"`
}

// StorageLifecyclePolicyList contains a list of StorageLifecyclePolicy
type StorageLifecyclePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StorageLifecyclePolicy `json:"items"`
}
