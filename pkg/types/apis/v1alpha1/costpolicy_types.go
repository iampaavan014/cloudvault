package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CostPolicySpec defines the desired state of CostPolicy
type CostPolicySpec struct {
	// Monthly budget cap for the targeted workloads
	Budget float64 `json:"budget"`

	// Percentage threshold for alerts (e.g., 80)
	AlertThreshold int `json:"alertThreshold"`

	// Action to take when budget is exceeded: alert, block
	Action string `json:"action"`

	// Selector for targeted namespaces or labels
	Selector CostPolicySelector `json:"selector"`
}

type CostPolicySelector struct {
	Namespaces []string          `json:"namespaces,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

// CostPolicyStatus defines the observed state of CostPolicy
type CostPolicyStatus struct {
	CurrentSpend  float64     `json:"currentSpend"`
	LastEvaluated metav1.Time `json:"lastEvaluated"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CostPolicy is the Schema for the costpolicies API
type CostPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CostPolicySpec   `json:"spec,omitempty"`
	Status CostPolicyStatus `json:"status,omitempty"`
}
