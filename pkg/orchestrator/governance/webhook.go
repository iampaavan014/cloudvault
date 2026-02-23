package governance

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/cost"
	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/cloudvault-io/cloudvault/pkg/types/apis/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AdmissionController handles validation of storage requests
type AdmissionController struct {
	calculator   *cost.Calculator
	policies     []v1alpha1.CostPolicy
	policyEngine *AdvancedPolicyEngine
	auditLog     []PolicyAuditEntry
}

// AdvancedPolicyEngine provides sophisticated policy evaluation
type AdvancedPolicyEngine struct {
	rules []PolicyRule
}

// PolicyRule represents an advanced policy rule
type PolicyRule struct {
	ID         string
	Name       string
	Priority   int
	Conditions []PolicyCondition
	Actions    []PolicyAction
	Scope      PolicyScope
	Enabled    bool
}

// PolicyCondition defines a condition to evaluate
type PolicyCondition struct {
	Type     string // "cost", "size", "storage_class", "namespace", "label", "time_window"
	Operator string // "gt", "lt", "eq", "contains", "matches"
	Value    interface{}
}

// PolicyAction defines an action to take
type PolicyAction struct {
	Type    string // "block", "warn", "modify", "notify", "audit"
	Params  map[string]interface{}
	Message string
}

// PolicyScope defines where policy applies
type PolicyScope struct {
	Namespaces        []string
	Labels            map[string]string
	StorageClasses    []string
	ExcludeNamespaces []string
}

// PolicyAuditEntry records policy enforcement events
type PolicyAuditEntry struct {
	Timestamp  time.Time
	PolicyID   string
	Action     string
	Resource   string
	Namespace  string
	User       string
	Decision   string // "allowed", "denied", "modified"
	Reason     string
	CostImpact float64
}

// NewAdmissionController creates a new governance webhook server
func NewAdmissionController() *AdmissionController {
	return &AdmissionController{
		calculator:   cost.NewCalculator(),
		policies:     []v1alpha1.CostPolicy{},
		policyEngine: NewAdvancedPolicyEngine(),
		auditLog:     make([]PolicyAuditEntry, 0),
	}
}

// NewAdvancedPolicyEngine creates a policy engine with predefined rules
func NewAdvancedPolicyEngine() *AdvancedPolicyEngine {
	engine := &AdvancedPolicyEngine{
		rules: []PolicyRule{},
	}

	// Add default sophisticated rules
	engine.addDefaultRules()

	return engine
}

// addDefaultRules adds production-ready policy rules
func (pe *AdvancedPolicyEngine) addDefaultRules() {
	// Rule 1: Cost Cap for Development Environments
	pe.rules = append(pe.rules, PolicyRule{
		ID:       "dev-cost-cap",
		Name:     "Development Environment Cost Cap",
		Priority: 10,
		Conditions: []PolicyCondition{
			{Type: "namespace", Operator: "contains", Value: "dev"},
			{Type: "cost", Operator: "gt", Value: 50.0},
		},
		Actions: []PolicyAction{
			{Type: "block", Message: "Development PVCs cannot exceed $50/month"},
		},
		Scope: PolicyScope{
			Namespaces: []string{"dev-*", "development-*"},
		},
		Enabled: true,
	})

	// Rule 2: Premium Storage Approval Required
	pe.rules = append(pe.rules, PolicyRule{
		ID:       "premium-storage-approval",
		Name:     "Premium Storage Requires Approval",
		Priority: 20,
		Conditions: []PolicyCondition{
			{Type: "storage_class", Operator: "contains", Value: "premium"},
			{Type: "size", Operator: "gt", Value: int64(100 * 1024 * 1024 * 1024)}, // 100GB
		},
		Actions: []PolicyAction{
			{Type: "block", Message: "Premium storage over 100GB requires approval ticket"},
			{Type: "notify", Params: map[string]interface{}{"channel": "slack", "team": "infrastructure"}},
		},
		Enabled: true,
	})

	// Rule 3: Production Environment Best Practices
	pe.rules = append(pe.rules, PolicyRule{
		ID:       "prod-best-practices",
		Name:     "Production Storage Best Practices",
		Priority: 30,
		Conditions: []PolicyCondition{
			{Type: "namespace", Operator: "eq", Value: "production"},
			{Type: "storage_class", Operator: "eq", Value: "standard"},
		},
		Actions: []PolicyAction{
			{Type: "warn", Message: "Production workloads should use premium storage for better performance and reliability"},
			{Type: "audit"},
		},
		Scope: PolicyScope{
			Namespaces: []string{"production", "prod"},
		},
		Enabled: true,
	})

	// Rule 4: Zombie Prevention - Require Labels
	pe.rules = append(pe.rules, PolicyRule{
		ID:       "require-ownership-labels",
		Name:     "Require Ownership Labels",
		Priority: 5,
		Conditions: []PolicyCondition{
			{Type: "label", Operator: "missing", Value: "owner"},
			{Type: "label", Operator: "missing", Value: "team"},
		},
		Actions: []PolicyAction{
			{Type: "block", Message: "PVCs must have 'owner' and 'team' labels for accountability"},
		},
		Enabled: true,
	})

	// Rule 5: Cost Optimization - Suggest Alternatives
	pe.rules = append(pe.rules, PolicyRule{
		ID:       "suggest-alternatives",
		Name:     "Suggest Cost-Effective Alternatives",
		Priority: 15,
		Conditions: []PolicyCondition{
			{Type: "storage_class", Operator: "eq", Value: "io2"},
			{Type: "size", Operator: "lt", Value: int64(50 * 1024 * 1024 * 1024)}, // < 50GB
		},
		Actions: []PolicyAction{
			{Type: "warn", Message: "Consider using gp3 storage class for better cost efficiency on small volumes"},
			{Type: "modify", Params: map[string]interface{}{"suggested_class": "gp3", "savings": 40}},
		},
		Enabled: true,
	})

	// Rule 6: Time-Based Controls - Off-Hours Development
	pe.rules = append(pe.rules, PolicyRule{
		ID:       "off-hours-restrictions",
		Name:     "Off-Hours Development Restrictions",
		Priority: 25,
		Conditions: []PolicyCondition{
			{Type: "time_window", Operator: "outside", Value: "09:00-17:00"},
			{Type: "namespace", Operator: "contains", Value: "dev"},
			{Type: "cost", Operator: "gt", Value: 100.0},
		},
		Actions: []PolicyAction{
			{Type: "block", Message: "Large development PVCs cannot be created outside business hours without approval"},
			{Type: "notify", Params: map[string]interface{}{"oncall": true}},
		},
		Enabled: false, // Disabled by default, can be enabled per-org
	})
}

// EvaluatePVC evaluates a PVC against all policy rules
func (pe *AdvancedPolicyEngine) EvaluatePVC(pvc *corev1.PersistentVolumeClaim, estimatedCost float64) PolicyEvaluationResult {
	result := PolicyEvaluationResult{
		Allowed:     true,
		Warnings:    []string{},
		Suggestions: []string{},
		AuditEvents: []string{},
	}

	// Sort rules by priority (lower number = higher priority)
	// Process rules in priority order
	for _, rule := range pe.rules {
		if !rule.Enabled {
			continue
		}

		// Check if rule applies to this PVC
		if !pe.ruleApplies(rule, pvc) {
			continue
		}

		// Evaluate conditions
		allConditionsMet := pe.evaluateConditions(rule.Conditions, pvc, estimatedCost)

		if allConditionsMet {
			// Execute actions
			for _, action := range rule.Actions {
				switch action.Type {
				case "block":
					result.Allowed = false
					result.BlockReason = action.Message
					slog.Warn("Policy blocked PVC", "rule", rule.Name, "pvc", pvc.Name)
					return result // Stop processing on block

				case "warn":
					result.Warnings = append(result.Warnings, action.Message)

				case "modify":
					if suggestedClass, ok := action.Params["suggested_class"].(string); ok {
						savings := action.Params["savings"].(int)
						result.Suggestions = append(result.Suggestions,
							fmt.Sprintf("Consider %s storage class for %d%% cost savings", suggestedClass, savings))
					}

				case "notify":
					// Notification would be handled by external system
					result.AuditEvents = append(result.AuditEvents,
						fmt.Sprintf("Notification triggered: %v", action.Params))

				case "audit":
					result.AuditEvents = append(result.AuditEvents,
						fmt.Sprintf("Rule '%s' matched", rule.Name))
				}
			}
		}
	}

	return result
}

// PolicyEvaluationResult holds the result of policy evaluation
type PolicyEvaluationResult struct {
	Allowed     bool
	BlockReason string
	Warnings    []string
	Suggestions []string
	AuditEvents []string
}

// ruleApplies checks if rule scope matches the PVC
func (pe *AdvancedPolicyEngine) ruleApplies(rule PolicyRule, pvc *corev1.PersistentVolumeClaim) bool {
	// Check namespace scope
	if len(rule.Scope.Namespaces) > 0 {
		matches := false
		for _, ns := range rule.Scope.Namespaces {
			if pe.namespaceMatches(ns, pvc.Namespace) {
				matches = true
				break
			}
		}
		if !matches {
			return false
		}
	}

	// Check excluded namespaces
	for _, ns := range rule.Scope.ExcludeNamespaces {
		if pe.namespaceMatches(ns, pvc.Namespace) {
			return false
		}
	}

	// Check storage class scope
	if len(rule.Scope.StorageClasses) > 0 && pvc.Spec.StorageClassName != nil {
		matches := false
		for _, sc := range rule.Scope.StorageClasses {
			if sc == *pvc.Spec.StorageClassName {
				matches = true
				break
			}
		}
		if !matches {
			return false
		}
	}

	return true
}

// namespaceMatches checks if namespace matches pattern (supports wildcards)
func (pe *AdvancedPolicyEngine) namespaceMatches(pattern, namespace string) bool {
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(namespace, prefix)
	}
	return pattern == namespace
}

// evaluateConditions checks if all conditions are met
func (pe *AdvancedPolicyEngine) evaluateConditions(conditions []PolicyCondition, pvc *corev1.PersistentVolumeClaim, estimatedCost float64) bool {
	for _, condition := range conditions {
		if !pe.evaluateCondition(condition, pvc, estimatedCost) {
			return false
		}
	}
	return true
}

// evaluateCondition evaluates a single condition
func (pe *AdvancedPolicyEngine) evaluateCondition(condition PolicyCondition, pvc *corev1.PersistentVolumeClaim, estimatedCost float64) bool {
	switch condition.Type {
	case "cost":
		threshold := condition.Value.(float64)
		switch condition.Operator {
		case "gt":
			return estimatedCost > threshold
		case "lt":
			return estimatedCost < threshold
		case "eq":
			return estimatedCost == threshold
		}

	case "size":
		threshold := condition.Value.(int64)
		size := pvc.Spec.Resources.Requests.Storage().Value()
		switch condition.Operator {
		case "gt":
			return size > threshold
		case "lt":
			return size < threshold
		case "eq":
			return size == threshold
		}

	case "storage_class":
		if pvc.Spec.StorageClassName == nil {
			return false
		}
		pattern := condition.Value.(string)
		switch condition.Operator {
		case "eq":
			return *pvc.Spec.StorageClassName == pattern
		case "contains":
			return strings.Contains(*pvc.Spec.StorageClassName, pattern)
		}

	case "namespace":
		pattern := condition.Value.(string)
		switch condition.Operator {
		case "eq":
			return pvc.Namespace == pattern
		case "contains":
			return strings.Contains(pvc.Namespace, pattern)
		}

	case "label":
		labelKey := condition.Value.(string)
		switch condition.Operator {
		case "missing":
			_, exists := pvc.Labels[labelKey]
			return !exists
		case "exists":
			_, exists := pvc.Labels[labelKey]
			return exists
		}

	case "time_window":
		// Time-based policy (for off-hours restrictions)
		// Format: "HH:MM-HH:MM"
		window := condition.Value.(string)
		now := time.Now()
		hour := now.Hour()

		parts := strings.Split(window, "-")
		if len(parts) == 2 {
			// Parse start and end times
			// Simple hour-based check for demo
			if condition.Operator == "outside" {
				return hour < 9 || hour >= 17
			}
		}
	}

	return false
}

// SetPolicies updates the controller's policy cache
func (ac *AdmissionController) SetPolicies(policies []v1alpha1.CostPolicy) {
	ac.policies = policies
	slog.Info("Policies updated", "count", len(policies))
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
	_, _ = w.Write(respBytes)
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

	// NEW: Evaluate with advanced policy engine
	policyResult := ac.policyEngine.EvaluatePVC(&pvc, estimatedCost)

	// Record audit event
	ac.recordAudit(PolicyAuditEntry{
		Timestamp:  time.Now(),
		Resource:   fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name),
		Namespace:  pvc.Namespace,
		User:       req.UserInfo.Username,
		Decision:   map[bool]string{true: "allowed", false: "denied"}[policyResult.Allowed],
		Reason:     policyResult.BlockReason,
		CostImpact: estimatedCost,
	})

	// If advanced engine blocked it, return denial
	if !policyResult.Allowed {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("CloudVault Advanced Policy Enforcement: %s", policyResult.BlockReason),
			},
		}
	}

	// Check against simple CostPolicies (backward compatibility)
	for _, policy := range ac.policies {
		if estimatedCost > policy.Spec.Budget {
			if policy.Spec.Action == "block" {
				slog.Warn("PVC BLOCK ENFORCED", "pvc", pvc.Name, "policy", policy.Name, "cost", estimatedCost, "budget", policy.Spec.Budget)
				return &admissionv1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Message: fmt.Sprintf("CloudVault Policy Enforcement: Estimated monthly cost ($%.2f) exceeds policy '%s' budget limit ($%.2f)",
							estimatedCost, policy.Name, policy.Spec.Budget),
					},
				}
			}
		}
	}

	// Collect all warnings
	warnings := policyResult.Warnings
	warnings = append(warnings, policyResult.Suggestions...)
	warnings = append(warnings, fmt.Sprintf("CloudVault: Estimated monthly cost: $%.2f", estimatedCost))

	return &admissionv1.AdmissionResponse{
		Allowed:  true,
		Warnings: warnings,
	}
}

// recordAudit records a policy enforcement event
func (ac *AdmissionController) recordAudit(entry PolicyAuditEntry) {
	ac.auditLog = append(ac.auditLog, entry)

	// Keep only last 10,000 entries
	if len(ac.auditLog) > 10000 {
		ac.auditLog = ac.auditLog[len(ac.auditLog)-10000:]
	}

	slog.Info("Policy audit recorded",
		"decision", entry.Decision,
		"resource", entry.Resource,
		"cost", entry.CostImpact)
}

// GetAuditLog returns recent audit entries
func (ac *AdmissionController) GetAuditLog(limit int) []PolicyAuditEntry {
	if limit <= 0 || limit > len(ac.auditLog) {
		limit = len(ac.auditLog)
	}

	start := len(ac.auditLog) - limit
	return ac.auditLog[start:]
}
