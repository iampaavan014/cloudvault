package lifecycle

import (
	"fmt"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/cloudvault-io/cloudvault/pkg/types/apis/v1alpha1"
)

// PolicyEngine evaluates PVCs against Lifecycle Policies
type PolicyEngine struct {
	policies []v1alpha1.StorageLifecyclePolicy
}

// NewPolicyEngine creates a new policy engine with a set of active policies
func NewPolicyEngine(policies []v1alpha1.StorageLifecyclePolicy) *PolicyEngine {
	return &PolicyEngine{
		policies: policies,
	}
}

// Match finds the best matching policy for a given PVC
func (e *PolicyEngine) Match(pvc types.PVCMetric) *v1alpha1.StorageLifecyclePolicy {
	for _, policy := range e.policies {
		// 1. Match Namespace
		nsMatch := false
		if len(policy.Spec.Selector.MatchNamespaces) == 0 {
			nsMatch = true // Empty namespaces means all namespaces
		} else {
			for _, ns := range policy.Spec.Selector.MatchNamespaces {
				if ns == pvc.Namespace {
					nsMatch = true
					break
				}
			}
		}

		if !nsMatch {
			continue
		}

		// 2. Match Labels
		labelMatch := true
		for k, v := range policy.Spec.Selector.MatchLabels {
			if val, ok := pvc.Labels[k]; !ok || val != v {
				labelMatch = false
				break
			}
		}

		if labelMatch {
			return &policy
		}
	}

	return nil
}

// Evaluate determines if a PVC requires an action based on its matching policy
func (e *PolicyEngine) Evaluate(pvc types.PVCMetric, policy *v1alpha1.StorageLifecyclePolicy) (*v1alpha1.StorageTier, error) {
	if policy == nil {
		return nil, nil
	}

	pvcAge := time.Since(pvc.CreatedAt)

	// Identify current tier index based on storage class
	var currentTierIndex = -1
	for i, tier := range policy.Spec.Tiers {
		if tier.StorageClass == pvc.StorageClass {
			currentTierIndex = i
			break
		}
	}

	// Find the most advanced tier the PVC is eligible for based on age
	var targetTierIndex = -1
	for i := len(policy.Spec.Tiers) - 1; i >= 0; i-- {
		tier := policy.Spec.Tiers[i]

		duration, err := ParseDuration(tier.Duration)
		if err != nil {
			return nil, fmt.Errorf("invalid duration %s in policy %s: %w", tier.Duration, policy.Name, err)
		}

		if pvcAge > duration {
			targetTierIndex = i
			break
		}
	}

	// Only recommend migration if the target tier is more advanced than the current one
	if targetTierIndex > currentTierIndex {
		return &policy.Spec.Tiers[targetTierIndex], nil
	}

	return nil, nil
}

// ParseDuration handles standard Go durations plus 'd' for days
func ParseDuration(durStr string) (time.Duration, error) {
	if len(durStr) == 0 {
		return 0, fmt.Errorf("empty duration")
	}

	if durStr[len(durStr)-1] == 'd' {
		var days int
		n, err := fmt.Sscanf(durStr, "%dd", &days)
		if err != nil || n != 1 {
			return 0, fmt.Errorf("invalid days format: %s", durStr)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	return time.ParseDuration(durStr)
}
