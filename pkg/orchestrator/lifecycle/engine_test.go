package lifecycle

import (
	"testing"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/cloudvault-io/cloudvault/pkg/types/apis/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPolicyEngine_Match(t *testing.T) {
	policies := []v1alpha1.StorageLifecyclePolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "policy-ns-prod"},
			Spec: v1alpha1.StorageLifecyclePolicySpec{
				Selector: v1alpha1.PolicySelector{MatchNamespaces: []string{"production"}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "policy-label-db"},
			Spec: v1alpha1.StorageLifecyclePolicySpec{
				Selector: v1alpha1.PolicySelector{MatchLabels: map[string]string{"app": "postgres"}},
			},
		},
	}

	engine := NewPolicyEngine(policies)

	tests := []struct {
		name     string
		pvc      types.PVCMetric
		expected string
	}{
		{
			name: "match-namespace",
			pvc: types.PVCMetric{
				Namespace: "production",
			},
			expected: "policy-ns-prod",
		},
		{
			name: "match-labels",
			pvc: types.PVCMetric{
				Namespace: "staging",
				Labels:    map[string]string{"app": "postgres"},
			},
			expected: "policy-label-db",
		},
		{
			name: "no-match",
			pvc: types.PVCMetric{
				Namespace: "staging",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := engine.Match(tt.pvc)
			if tt.expected == "" {
				if match != nil {
					t.Errorf("Expected nil match, got %s", match.Name)
				}
			} else {
				if match == nil || match.Name != tt.expected {
					t.Errorf("Expected match %s, got %v", tt.expected, match)
				}
			}
		})
	}
}

func TestPolicyEngine_Evaluate(t *testing.T) {
	policy := v1alpha1.StorageLifecyclePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
		Spec: v1alpha1.StorageLifecyclePolicySpec{
			Tiers: []v1alpha1.StorageTier{
				{Name: "hot", StorageClass: "gp3", Duration: "0s"},
				{Name: "warm", StorageClass: "sc1", Duration: "7d"},
				{Name: "cold", StorageClass: "glacier", Duration: "30d"},
			},
		},
	}

	engine := NewPolicyEngine([]v1alpha1.StorageLifecyclePolicy{policy})

	now := time.Now()
	tests := []struct {
		name         string
		storageClass string
		createdAt    time.Time
		expectedTier string
	}{
		{
			name:         "stay-hot-young-pvc",
			storageClass: "gp3",
			createdAt:    now.Add(-24 * time.Hour), // 1 day old
			expectedTier: "",
		},
		{
			name:         "move-to-warm-at-7d",
			storageClass: "gp3",
			createdAt:    now.Add(-8 * 24 * time.Hour), // 8 days old
			expectedTier: "warm",
		},
		{
			name:         "already-warm-no-action",
			storageClass: "sc1",
			createdAt:    now.Add(-8 * 24 * time.Hour),
			expectedTier: "",
		},
		{
			name:         "move-to-cold-at-30d",
			storageClass: "sc1",
			createdAt:    now.Add(-31 * 24 * time.Hour),
			expectedTier: "cold",
		},
		{
			name:         "skip-warm-move-straight-to-cold",
			storageClass: "gp3",
			createdAt:    now.Add(-40 * 24 * time.Hour),
			expectedTier: "cold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pvc := types.PVCMetric{
				StorageClass: tt.storageClass,
				CreatedAt:    tt.createdAt,
			}
			tier, err := engine.Evaluate(pvc, &policy)
			if err != nil {
				t.Fatalf("Evaluate failed: %v", err)
			}

			if tt.expectedTier == "" {
				if tier != nil {
					t.Errorf("Expected nil tier, got %s", tier.Name)
				}
			} else {
				if tier == nil || tier.Name != tt.expectedTier {
					t.Errorf("Expected tier %s, got %v", tt.expectedTier, tier)
				}
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		durStr   string
		expected time.Duration
		wantErr  bool
	}{
		{"1h", time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"1d", 24 * time.Hour, false},
		{"30d", 30 * 24 * time.Hour, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.durStr, func(t *testing.T) {
			got, err := ParseDuration(tt.durStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDuration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseDuration() got = %v, want %v", got, tt.expected)
			}
		})
	}
}
