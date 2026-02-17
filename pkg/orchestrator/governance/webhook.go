package governance

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/cloudvault-io/cloudvault/pkg/cost"
	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/cloudvault-io/cloudvault/pkg/types/apis/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AdmissionController handles validation of storage requests
type AdmissionController struct {
	calculator *cost.Calculator
	policies   []v1alpha1.CostPolicy
}

// NewAdmissionController creates a new governance webhook server
func NewAdmissionController() *AdmissionController {
	return &AdmissionController{
		calculator: cost.NewCalculator(),
		policies:   []v1alpha1.CostPolicy{},
	}
}

// SetPolicies updates the controller's policy cache
func (ac *AdmissionController) SetPolicies(policies []v1alpha1.CostPolicy) {
	ac.policies = policies
}

// ServeHTTP handles admission review requests
func (ac *AdmissionController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slog.Info("Admission Webhook received request", "method", r.Method, "path", r.URL.Path)
	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	review := admissionv1.AdmissionReview{}
	if err := json.Unmarshal(body, &review); err != nil {
		http.Error(w, "failed to unmarshal review", http.StatusBadRequest)
		return
	}

	// Logic for Validating Webhook
	response := ac.validate(review.Request)
	review.Response = response
	review.Response.UID = review.Request.UID

	respBytes, _ := json.Marshal(review)
	w.Header().Set("Content-Type", "application/json")
	w.Write(respBytes)
}

func (ac *AdmissionController) validate(req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	slog.Info("Admission Webhook validating", "kind", req.Kind.Kind, "resource", req.Resource.Resource, "name", req.Name, "namespace", req.Namespace)
	if req.Kind.Kind != "PersistentVolumeClaim" {
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	var pvc corev1.PersistentVolumeClaim
	if err := json.Unmarshal(req.Object.Raw, &pvc); err != nil {
		slog.Error("Failed to decode PVC in admission webhook", "error", err)
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result:  &metav1.Status{Message: "failed to decode PVC"},
		}
	}

	// Calculate estimated cost
	metric := &types.PVCMetric{
		Name:         pvc.Name,
		Namespace:    pvc.Namespace,
		StorageClass: *pvc.Spec.StorageClassName,
		SizeBytes:    pvc.Spec.Resources.Requests.Storage().Value(),
	}
	estimatedCost := ac.calculator.CalculatePVCCost(metric, "aws") // Default to aws for now

	// Check against dynamic CostPolicies
	slog.Info("Evaluating policies against estimated cost", "pvc", pvc.Name, "cost", estimatedCost, "policies", len(ac.policies))
	for _, policy := range ac.policies {
		if estimatedCost > policy.Spec.Budget {
			if policy.Spec.Action == "block" {
				slog.Warn("PVC BLOCK ENFORCED", "pvc", pvc.Name, "policy", policy.Name, "cost", estimatedCost, "budget", policy.Spec.Budget)
				return &admissionv1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Message: fmt.Sprintf("CloudVault Policy Enforcement (STRICT): Estimated monthly cost ($%.2f) exceeds policy '%s' budget limit ($%.2f)",
							estimatedCost, policy.Name, policy.Spec.Budget),
					},
				}
			}
			// Fallback to warning if action is not block (e.g., alert)
			return &admissionv1.AdmissionResponse{
				Allowed: true,
				Warnings: []string{
					fmt.Sprintf("CloudVault POLICY ALERT: Estimated monthly cost ($%.2f) exceeds budget limit ($%.2f)", estimatedCost, policy.Spec.Budget),
				},
			}
		}
	}

	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Warnings: []string{
			fmt.Sprintf("CloudVault: PVC creation allowed. Estimated monthly cost: $%.2f", estimatedCost),
		},
	}
}
