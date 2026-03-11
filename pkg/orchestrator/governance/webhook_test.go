package governance

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/types/apis/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ── Constructor ───────────────────────────────────────────────────────────────

func TestNewAdmissionController(t *testing.T) {
	ac := NewAdmissionController()
	require.NotNil(t, ac)
	assert.NotNil(t, ac.calculator)
	assert.NotNil(t, ac.policyEngine)
}

func TestNewAdvancedPolicyEngine(t *testing.T) {
	pe := NewAdvancedPolicyEngine()
	require.NotNil(t, pe)
	// Default rules are loaded
	assert.NotEmpty(t, pe.rules)
}

// ── SetPolicies ───────────────────────────────────────────────────────────────

func TestAdmissionController_SetPolicies(t *testing.T) {
	ac := NewAdmissionController()
	policies := []v1alpha1.CostPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
			Spec:       v1alpha1.CostPolicySpec{Budget: 100, Action: "block"},
		},
	}
	ac.SetPolicies(policies)
	assert.Equal(t, 1, len(ac.policies))
}

// ── AuditLog ──────────────────────────────────────────────────────────────────

func TestAdmissionController_GetAuditLog_Empty(t *testing.T) {
	ac := NewAdmissionController()
	log := ac.GetAuditLog(10)
	assert.Empty(t, log)
}

func TestAdmissionController_RecordAndGetAuditLog(t *testing.T) {
	ac := NewAdmissionController()
	for i := 0; i < 5; i++ {
		ac.recordAudit(PolicyAuditEntry{
			Timestamp: time.Now(),
			Decision:  "allowed",
			Resource:  "default/pvc-" + string(rune('0'+i)),
		})
	}
	log := ac.GetAuditLog(3)
	assert.Len(t, log, 3)
}

func TestAdmissionController_AuditLog_Limit(t *testing.T) {
	ac := NewAdmissionController()
	// Add more than limit
	for i := 0; i < 5; i++ {
		ac.recordAudit(PolicyAuditEntry{Decision: "allowed"})
	}
	// Request more than available
	log := ac.GetAuditLog(100)
	assert.Len(t, log, 5)
}

func TestAdmissionController_AuditLog_ZeroLimit(t *testing.T) {
	ac := NewAdmissionController()
	ac.recordAudit(PolicyAuditEntry{Decision: "allowed"})
	log := ac.GetAuditLog(0)
	assert.Len(t, log, 1)
}

// ── namespaceMatches ──────────────────────────────────────────────────────────

func TestNamespaceMatches(t *testing.T) {
	pe := NewAdvancedPolicyEngine()
	assert.True(t, pe.namespaceMatches("dev-*", "dev-team1"))
	assert.True(t, pe.namespaceMatches("dev-*", "dev-"))
	assert.False(t, pe.namespaceMatches("dev-*", "production"))
	assert.True(t, pe.namespaceMatches("production", "production"))
	assert.False(t, pe.namespaceMatches("production", "prod"))
}

// ── ruleApplies ───────────────────────────────────────────────────────────────

func TestRuleApplies_NoScope(t *testing.T) {
	pe := NewAdvancedPolicyEngine()
	rule := PolicyRule{
		ID:      "no-scope",
		Enabled: true,
		Scope:   PolicyScope{},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Namespace: "anything"},
	}
	assert.True(t, pe.ruleApplies(rule, pvc))
}

func TestRuleApplies_NamespaceMatch(t *testing.T) {
	pe := NewAdvancedPolicyEngine()
	rule := PolicyRule{
		Scope: PolicyScope{Namespaces: []string{"dev-*"}},
	}
	devPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Namespace: "dev-team"},
	}
	prodPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Namespace: "production"},
	}
	assert.True(t, pe.ruleApplies(rule, devPVC))
	assert.False(t, pe.ruleApplies(rule, prodPVC))
}

func TestRuleApplies_ExcludeNamespace(t *testing.T) {
	pe := NewAdvancedPolicyEngine()
	rule := PolicyRule{
		Scope: PolicyScope{ExcludeNamespaces: []string{"kube-system"}},
	}
	excluded := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system"},
	}
	normal := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
	}
	assert.False(t, pe.ruleApplies(rule, excluded))
	assert.True(t, pe.ruleApplies(rule, normal))
}

func TestRuleApplies_StorageClassScope(t *testing.T) {
	pe := NewAdvancedPolicyEngine()
	sc := "gp3"
	rule := PolicyRule{
		Scope: PolicyScope{StorageClasses: []string{"gp3"}},
	}
	matching := &corev1.PersistentVolumeClaim{
		Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &sc},
	}
	otherSC := "io2"
	nonMatching := &corev1.PersistentVolumeClaim{
		Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &otherSC},
	}
	assert.True(t, pe.ruleApplies(rule, matching))
	assert.False(t, pe.ruleApplies(rule, nonMatching))
}

// ── evaluateCondition ─────────────────────────────────────────────────────────

func TestEvaluateCondition_Cost(t *testing.T) {
	pe := NewAdvancedPolicyEngine()
	pvc := &corev1.PersistentVolumeClaim{}

	assert.True(t, pe.evaluateCondition(PolicyCondition{Type: "cost", Operator: "gt", Value: 40.0}, pvc, 50.0))
	assert.False(t, pe.evaluateCondition(PolicyCondition{Type: "cost", Operator: "gt", Value: 60.0}, pvc, 50.0))
	assert.True(t, pe.evaluateCondition(PolicyCondition{Type: "cost", Operator: "lt", Value: 60.0}, pvc, 50.0))
	assert.True(t, pe.evaluateCondition(PolicyCondition{Type: "cost", Operator: "eq", Value: 50.0}, pvc, 50.0))
}

func TestEvaluateCondition_Size(t *testing.T) {
	pe := NewAdvancedPolicyEngine()
	q := resource.MustParse("200Gi")
	pvc := &corev1.PersistentVolumeClaim{
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: q},
			},
		},
	}
	threshold := int64(100 * 1024 * 1024 * 1024) // 100Gi
	assert.True(t, pe.evaluateCondition(PolicyCondition{Type: "size", Operator: "gt", Value: threshold}, pvc, 0))
	assert.False(t, pe.evaluateCondition(PolicyCondition{Type: "size", Operator: "lt", Value: threshold}, pvc, 0))
}

func TestEvaluateCondition_StorageClass(t *testing.T) {
	pe := NewAdvancedPolicyEngine()
	sc := "gp3"
	pvc := &corev1.PersistentVolumeClaim{
		Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &sc},
	}
	assert.True(t, pe.evaluateCondition(PolicyCondition{Type: "storage_class", Operator: "eq", Value: "gp3"}, pvc, 0))
	assert.False(t, pe.evaluateCondition(PolicyCondition{Type: "storage_class", Operator: "eq", Value: "io2"}, pvc, 0))
	assert.True(t, pe.evaluateCondition(PolicyCondition{Type: "storage_class", Operator: "contains", Value: "gp"}, pvc, 0))
}

func TestEvaluateCondition_StorageClass_Nil(t *testing.T) {
	pe := NewAdvancedPolicyEngine()
	pvc := &corev1.PersistentVolumeClaim{} // no StorageClassName
	assert.False(t, pe.evaluateCondition(PolicyCondition{Type: "storage_class", Operator: "eq", Value: "gp3"}, pvc, 0))
}

func TestEvaluateCondition_Namespace(t *testing.T) {
	pe := NewAdvancedPolicyEngine()
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Namespace: "dev-team"},
	}
	assert.True(t, pe.evaluateCondition(PolicyCondition{Type: "namespace", Operator: "eq", Value: "dev-team"}, pvc, 0))
	assert.False(t, pe.evaluateCondition(PolicyCondition{Type: "namespace", Operator: "eq", Value: "production"}, pvc, 0))
	assert.True(t, pe.evaluateCondition(PolicyCondition{Type: "namespace", Operator: "contains", Value: "dev"}, pvc, 0))
}

func TestEvaluateCondition_Label(t *testing.T) {
	pe := NewAdvancedPolicyEngine()
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"owner": "team-a"},
		},
	}
	assert.True(t, pe.evaluateCondition(PolicyCondition{Type: "label", Operator: "exists", Value: "owner"}, pvc, 0))
	assert.False(t, pe.evaluateCondition(PolicyCondition{Type: "label", Operator: "missing", Value: "owner"}, pvc, 0))
	assert.True(t, pe.evaluateCondition(PolicyCondition{Type: "label", Operator: "missing", Value: "team"}, pvc, 0))
}

func TestEvaluateCondition_TimeWindow(t *testing.T) {
	pe := NewAdvancedPolicyEngine()
	pvc := &corev1.PersistentVolumeClaim{}
	// "outside" condition — just ensure it returns a bool without panic
	result := pe.evaluateCondition(PolicyCondition{Type: "time_window", Operator: "outside", Value: "09:00-17:00"}, pvc, 0)
	_ = result // may be true or false depending on time of day
}

func TestEvaluateCondition_Unknown(t *testing.T) {
	pe := NewAdvancedPolicyEngine()
	pvc := &corev1.PersistentVolumeClaim{}
	assert.False(t, pe.evaluateCondition(PolicyCondition{Type: "unknown-type", Operator: "eq", Value: "x"}, pvc, 0))
}

// ── EvaluatePVC ───────────────────────────────────────────────────────────────

func TestEvaluatePVC_Allowed(t *testing.T) {
	pe := NewAdvancedPolicyEngine()
	sc := "gp3"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "production",
			Name:      "my-pvc",
			Labels:    map[string]string{"owner": "team-a", "team": "platform"},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &sc,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}
	result := pe.EvaluatePVC(pvc, 5.0)
	assert.True(t, result.Allowed)
}

func TestEvaluatePVC_BlockedByDevCostCap(t *testing.T) {
	pe := NewAdvancedPolicyEngine()
	sc := "gp3"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "dev-team",
			Name:      "big-pvc",
			Labels:    map[string]string{"owner": "a", "team": "b"},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &sc,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}
	// Cost > $50 in a dev namespace → block
	result := pe.EvaluatePVC(pvc, 60.0)
	assert.False(t, result.Allowed)
	assert.NotEmpty(t, result.BlockReason)
}

func TestEvaluatePVC_WarnForProdStandard(t *testing.T) {
	pe := NewAdvancedPolicyEngine()
	sc := "standard"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "production",
			Name:      "warn-pvc",
			Labels:    map[string]string{"owner": "a", "team": "b"},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &sc,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("5Gi"),
				},
			},
		},
	}
	result := pe.EvaluatePVC(pvc, 1.0)
	assert.True(t, result.Allowed)
	assert.NotEmpty(t, result.Warnings)
}

func TestEvaluatePVC_MissingLabelsBlock(t *testing.T) {
	pe := NewAdvancedPolicyEngine()
	sc := "gp3"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "no-labels-pvc",
			// no labels
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &sc,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}
	result := pe.EvaluatePVC(pvc, 5.0)
	assert.False(t, result.Allowed)
}

// ── ServeHTTP ─────────────────────────────────────────────────────────────────

func TestAdmissionController_ServeHTTP_EmptyBody(t *testing.T) {
	ac := NewAdmissionController()
	req := httptest.NewRequest(http.MethodPost, "/validate", nil)
	rr := httptest.NewRecorder()
	ac.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdmissionController_ServeHTTP_InvalidJSON(t *testing.T) {
	ac := NewAdmissionController()
	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewBufferString("not-json"))
	rr := httptest.NewRecorder()
	ac.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdmissionController_ServeHTTP_NonPVC(t *testing.T) {
	ac := NewAdmissionController()
	review := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID:  "test-uid",
			Kind: metav1.GroupVersionKind{Kind: "Deployment"},
		},
	}
	body, _ := json.Marshal(review)
	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	ac.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp admissionv1.AdmissionReview
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.True(t, resp.Response.Allowed)
}

func TestAdmissionController_ServeHTTP_ValidPVC(t *testing.T) {
	ac := NewAdmissionController()
	sc := "gp3"
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc",
			Namespace: "default",
			Labels:    map[string]string{"owner": "a", "team": "b"},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &sc,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}
	pvcBytes, _ := json.Marshal(pvc)
	review := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID:       "test-uid-2",
			Kind:      metav1.GroupVersionKind{Kind: "PersistentVolumeClaim"},
			Namespace: "default",
			Object:    runtime.RawExtension{Raw: pvcBytes},
		},
	}
	body, _ := json.Marshal(review)
	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	ac.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAdmissionController_ServeHTTP_PVCBlockedByCostPolicy(t *testing.T) {
	ac := NewAdmissionController()
	ac.SetPolicies([]v1alpha1.CostPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "strict-policy"},
			Spec:       v1alpha1.CostPolicySpec{Budget: 0.01, Action: "block"},
		},
	})
	sc := "io2"
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "expensive-pvc",
			Namespace: "default",
			Labels:    map[string]string{"owner": "a", "team": "b"},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &sc,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1000Gi"),
				},
			},
		},
	}
	pvcBytes, _ := json.Marshal(pvc)
	review := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID:       "block-uid",
			Kind:      metav1.GroupVersionKind{Kind: "PersistentVolumeClaim"},
			Namespace: "default",
			Object:    runtime.RawExtension{Raw: pvcBytes},
		},
	}
	body, _ := json.Marshal(review)
	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	ac.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var resp admissionv1.AdmissionReview
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.False(t, resp.Response.Allowed)
}
