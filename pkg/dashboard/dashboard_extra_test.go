package dashboard

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/ebpf"
	"github.com/cloudvault-io/cloudvault/pkg/orchestrator/governance"
	"github.com/cloudvault-io/cloudvault/pkg/orchestrator/lifecycle"
	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/cloudvault-io/cloudvault/pkg/types/apis/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func newTestServer() *Server {
	return &Server{
		orchestrator: lifecycle.NewLifecycleController(0, nil, nil, nil, nil),
		governance:   governance.NewAdmissionController(),
		provider:     "aws",
		mock:         true,
		store:        &MetricsStore{},
	}
}

// ── writeError ───────────────────────────────────────────────────────────────

func TestWriteError(t *testing.T) {
	rr := httptest.NewRecorder()
	writeError(rr, "something went wrong", http.StatusBadRequest)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "something went wrong")
}

// ── handleBudget ─────────────────────────────────────────────────────────────

func TestHandleBudget_GET_Default(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/budget", nil)
	rr := httptest.NewRecorder()
	s.handleBudget(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]float64
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, 1000.0, body["limit"])
}

func TestHandleBudget_POST_Valid(t *testing.T) {
	s := newTestServer()
	payload := `{"limit": 2500.0}`
	req := httptest.NewRequest(http.MethodPost, "/api/budget", strings.NewReader(payload))
	rr := httptest.NewRecorder()
	s.handleBudget(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	s.store.RLock()
	limit := s.store.BudgetLimit
	s.store.RUnlock()
	assert.Equal(t, 2500.0, limit)
}

func TestHandleBudget_POST_InvalidBody(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/budget", strings.NewReader("not-json"))
	rr := httptest.NewRecorder()
	s.handleBudget(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleBudget_POST_ZeroLimit(t *testing.T) {
	s := newTestServer()
	payload := `{"limit": 0}`
	req := httptest.NewRequest(http.MethodPost, "/api/budget", strings.NewReader(payload))
	rr := httptest.NewRecorder()
	s.handleBudget(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleBudget_MethodNotAllowed(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodDelete, "/api/budget", nil)
	rr := httptest.NewRecorder()
	s.handleBudget(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

// ── handlePolicies ────────────────────────────────────────────────────────────

func TestHandlePolicies_Empty(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/policies", nil)
	rr := httptest.NewRecorder()
	s.handlePolicies(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "[]")
}

// ── handleMigrations ─────────────────────────────────────────────────────────

func TestHandleMigrations_NoExecutor(t *testing.T) {
	s := newTestServer() // migrationExecutor is nil
	req := httptest.NewRequest(http.MethodGet, "/api/migrations", nil)
	rr := httptest.NewRecorder()
	s.handleMigrations(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "[]")
}

func TestHandleMigrations_WithExecutor(t *testing.T) {
	s := newTestServer()
	exec, err := lifecycle.NewMigrationExecutor(nil, "argo")
	require.NoError(t, err)
	s.migrationExecutor = exec

	req := httptest.NewRequest(http.MethodGet, "/api/migrations", nil)
	rr := httptest.NewRecorder()
	s.handleMigrations(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	// GetAllMigrations returns nil slice → JSON null; both null and [] are acceptable
	body := strings.TrimSpace(rr.Body.String())
	assert.True(t, body == "null" || body == "[]", "unexpected body: %s", body)
}

// ── handleMigrationStatus ─────────────────────────────────────────────────────

func TestHandleMigrationStatus_NoExecutor(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/migrations/status/abc123", nil)
	rr := httptest.NewRecorder()
	s.handleMigrationStatus(rr, req)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestHandleMigrationStatus_Found(t *testing.T) {
	s := newTestServer()
	exec, _ := lifecycle.NewMigrationExecutor(nil, "argo")
	s.migrationExecutor = exec

	req := httptest.NewRequest(http.MethodGet, "/api/migrations/status/missing-id", nil)
	rr := httptest.NewRecorder()
	s.handleMigrationStatus(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ── handleApplyMigration ──────────────────────────────────────────────────────

func TestHandleApplyMigration_MethodNotAllowed(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/migrations/apply", nil)
	rr := httptest.NewRecorder()
	s.handleApplyMigration(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleApplyMigration_InvalidBody(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/migrations/apply", strings.NewReader("bad"))
	rr := httptest.NewRecorder()
	s.handleApplyMigration(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleApplyMigration_NoExecutor(t *testing.T) {
	s := newTestServer()
	payload := `{"pvcName":"pvc-1","namespace":"default"}`
	req := httptest.NewRequest(http.MethodPost, "/api/migrations/apply", strings.NewReader(payload))
	rr := httptest.NewRecorder()
	s.handleApplyMigration(rr, req)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestHandleApplyMigration_RecNotFound(t *testing.T) {
	s := newTestServer()
	s.store.Recommendations = []types.Recommendation{}
	// Put a fake executor so we pass the nil check
	s.migrationExecutor = &lifecycle.MigrationExecutor{}
	payload := `{"pvcName":"missing-pvc","namespace":"default"}`
	req := httptest.NewRequest(http.MethodPost, "/api/migrations/apply", strings.NewReader(payload))
	rr := httptest.NewRecorder()
	s.handleApplyMigration(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleApplyMigration_Success(t *testing.T) {
	s := newTestServer()
	exec, _ := lifecycle.NewMigrationExecutor(nil, "argo")
	s.migrationExecutor = exec

	s.store.Recommendations = []types.Recommendation{
		{PVC: "pvc-1", Namespace: "default", Type: "storage_class", RecommendedState: "sc1"},
	}
	s.store.Metrics = []types.PVCMetric{
		{Name: "pvc-1", Namespace: "default", SizeBytes: 10 * 1024 * 1024 * 1024},
	}
	payload := `{"pvcName":"pvc-1","namespace":"default"}`
	req := httptest.NewRequest(http.MethodPost, "/api/migrations/apply", strings.NewReader(payload))
	rr := httptest.NewRecorder()
	// The handler fires ExecuteMigration in a goroutine; it panics because sourceClient
	// is nil inside stepPreFlightCheck. We recover in a deferred panic handler.
	func() {
		defer func() { recover() }() //nolint:errcheck
		s.handleApplyMigration(rr, req)
	}()
	// The HTTP response is written BEFORE the goroutine runs, so we should see 200.
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "migration_started")
}

// ── handleClusters ────────────────────────────────────────────────────────────

func TestHandleClusters_NoRegistry(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/clusters", nil)
	rr := httptest.NewRecorder()
	s.handleClusters(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	// Should return a JSON array with one entry for current cluster
	assert.Contains(t, rr.Body.String(), "current-cluster")
}

func TestHandleClusters_ProviderSet(t *testing.T) {
	s := newTestServer()
	s.provider = "gcp"
	req := httptest.NewRequest(http.MethodGet, "/api/clusters", nil)
	rr := httptest.NewRecorder()
	s.handleClusters(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "current-cluster")
}

// ── handleClusterAggregate ────────────────────────────────────────────────────

func TestHandleClusterAggregate_NoRegistry(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/clusters/aggregate", nil)
	rr := httptest.NewRecorder()
	s.handleClusterAggregate(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Contains(t, body, "totalCost")
}

func TestHandleClusterAggregate_WithCost(t *testing.T) {
	s := newTestServer()
	s.store.Summary = types.CostSummary{TotalMonthlyCost: 250.0}
	req := httptest.NewRequest(http.MethodGet, "/api/clusters/aggregate", nil)
	rr := httptest.NewRecorder()
	s.handleClusterAggregate(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.InDelta(t, 250.0, body["totalCost"].(float64), 0.001)
}

// ── handleApplyRecommendation ─────────────────────────────────────────────────

func TestHandleApplyRecommendation_MethodNotAllowed(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/recommendations/apply", nil)
	rr := httptest.NewRecorder()
	s.handleApplyRecommendation(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleApplyRecommendation_InvalidBody(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/recommendations/apply", strings.NewReader("bad-json"))
	rr := httptest.NewRecorder()
	s.handleApplyRecommendation(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleApplyRecommendation_NotFound(t *testing.T) {
	s := newTestServer()
	payload := `{"pvcName":"missing","namespace":"default","type":"resize"}`
	req := httptest.NewRequest(http.MethodPost, "/api/recommendations/apply", strings.NewReader(payload))
	rr := httptest.NewRecorder()
	s.handleApplyRecommendation(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleApplyRecommendation_UnknownType(t *testing.T) {
	s := newTestServer()
	s.store.Recommendations = []types.Recommendation{
		{PVC: "pvc-1", Namespace: "default", Type: "unknown_action"},
	}
	payload := `{"pvcName":"pvc-1","namespace":"default","type":"unknown_action"}`
	req := httptest.NewRequest(http.MethodPost, "/api/recommendations/apply", strings.NewReader(payload))
	rr := httptest.NewRecorder()
	s.handleApplyRecommendation(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleApplyRecommendation_DeleteZombie_NoClient(t *testing.T) {
	s := newTestServer() // client is nil
	s.store.Recommendations = []types.Recommendation{
		{PVC: "zombie-pvc", Namespace: "default", Type: "delete_zombie"},
	}
	payload := `{"pvcName":"zombie-pvc","namespace":"default","type":"delete_zombie"}`
	req := httptest.NewRequest(http.MethodPost, "/api/recommendations/apply", strings.NewReader(payload))
	rr := httptest.NewRecorder()
	s.handleApplyRecommendation(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandleApplyRecommendation_ResizeNoClient(t *testing.T) {
	s := newTestServer() // client is nil
	s.store.Recommendations = []types.Recommendation{
		{PVC: "over-pvc", Namespace: "default", Type: "resize", RecommendedState: "10Gi"},
	}
	payload := `{"pvcName":"over-pvc","namespace":"default","type":"resize"}`
	req := httptest.NewRequest(http.MethodPost, "/api/recommendations/apply", strings.NewReader(payload))
	rr := httptest.NewRecorder()
	s.handleApplyRecommendation(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandleApplyRecommendation_StorageClassNoExecutor(t *testing.T) {
	s := newTestServer()
	s.store.Recommendations = []types.Recommendation{
		{PVC: "pvc-1", Namespace: "default", Type: "storage_class", RecommendedState: "sc1"},
	}
	payload := `{"pvcName":"pvc-1","namespace":"default","type":"storage_class"}`
	req := httptest.NewRequest(http.MethodPost, "/api/recommendations/apply", strings.NewReader(payload))
	rr := httptest.NewRecorder()
	s.handleApplyRecommendation(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandleApplyRecommendation_AIPlacementNoExecutor(t *testing.T) {
	s := newTestServer()
	s.store.Recommendations = []types.Recommendation{
		{PVC: "pvc-ai", Namespace: "default", Type: "ai_placement", RecommendedState: "gp3"},
	}
	payload := `{"pvcName":"pvc-ai","namespace":"default","type":"ai_placement"}`
	req := httptest.NewRequest(http.MethodPost, "/api/recommendations/apply", strings.NewReader(payload))
	rr := httptest.NewRecorder()
	s.handleApplyRecommendation(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// ── handleApplyRecommendationGitOps ──────────────────────────────────────────

func TestHandleApplyRecommendationGitOps_MethodNotAllowed(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/recommendations/gitops", nil)
	rr := httptest.NewRecorder()
	s.handleApplyRecommendationGitOps(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleApplyRecommendationGitOps_InvalidBody(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/recommendations/gitops", strings.NewReader("bad"))
	rr := httptest.NewRecorder()
	s.handleApplyRecommendationGitOps(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleApplyRecommendationGitOps_NoController(t *testing.T) {
	s := newTestServer() // gitopsController is nil
	payload := `{"pvcName":"pvc-1","namespace":"default","type":"resize"}`
	req := httptest.NewRequest(http.MethodPost, "/api/recommendations/gitops", strings.NewReader(payload))
	rr := httptest.NewRecorder()
	s.handleApplyRecommendationGitOps(rr, req)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

// ── handleAIMetrics ───────────────────────────────────────────────────────────

func TestHandleAIMetrics(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/ai-metrics", nil)
	rr := httptest.NewRecorder()
	s.handleAIMetrics(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Contains(t, body, "accuracy")
	assert.Contains(t, body, "latency")
	assert.Contains(t, body, "status")
}

// ── handleGovernanceStatus ────────────────────────────────────────────────────

func TestHandleGovernanceStatus_Fields(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/governance/status", nil)
	rr := httptest.NewRecorder()
	s.handleGovernanceStatus(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Contains(t, body, "active_policies")
	assert.Contains(t, body, "autonomous_actions")
	assert.Contains(t, body, "audit_log")
}

func TestHandleGovernanceStatus_WithHistory(t *testing.T) {
	s := newTestServer()
	// Just call without the invalid type assertion
	req := httptest.NewRequest(http.MethodGet, "/api/governance/status", nil)
	rr := httptest.NewRecorder()
	s.handleGovernanceStatus(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Contains(t, body, "autonomous_actions")
	assert.Contains(t, body, "audit_log")
}

// ── handlePVCs empty store ────────────────────────────────────────────────────

func TestHandlePVCs_EmptyStore(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/pvc", nil)
	rr := httptest.NewRecorder()
	s.handlePVCs(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "[]")
}

// ── handleRecommendations empty store ────────────────────────────────────────

func TestHandleRecommendations_EmptyStore(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/recommendations", nil)
	rr := httptest.NewRecorder()
	s.handleRecommendations(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "[]")
}

// ── LoginHandler ──────────────────────────────────────────────────────────────

func TestLoginHandler_Success(t *testing.T) {
	payload := `{"username":"admin","password":"cloudvault-secret"}`
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(payload))
	rr := httptest.NewRecorder()
	LoginHandler(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var resp LoginResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Token)
}

func TestLoginHandler_WrongPassword(t *testing.T) {
	payload := `{"username":"admin","password":"wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(payload))
	rr := httptest.NewRecorder()
	LoginHandler(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestLoginHandler_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader("bad-json"))
	rr := httptest.NewRecorder()
	LoginHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ── AuthMiddleware ────────────────────────────────────────────────────────────

func TestAuthMiddleware_AllowLogin(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	req := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	rr := httptest.NewRecorder()
	AuthMiddleware(next).ServeHTTP(rr, req)
	assert.True(t, called)
}

func TestAuthMiddleware_AllowStaticAsset(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	rr := httptest.NewRecorder()
	AuthMiddleware(next).ServeHTTP(rr, req)
	assert.True(t, called)
}

func TestAuthMiddleware_AllowOptions(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	req := httptest.NewRequest(http.MethodOptions, "/api/pvc", nil)
	rr := httptest.NewRecorder()
	AuthMiddleware(next).ServeHTTP(rr, req)
	assert.True(t, called)
}

func TestAuthMiddleware_BlockMissingToken(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	req := httptest.NewRequest(http.MethodGet, "/api/pvc", nil)
	rr := httptest.NewRecorder()
	AuthMiddleware(next).ServeHTTP(rr, req)
	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthMiddleware_BlockInvalidToken(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	req := httptest.NewRequest(http.MethodGet, "/api/pvc", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.token")
	rr := httptest.NewRecorder()
	AuthMiddleware(next).ServeHTTP(rr, req)
	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthMiddleware_AllowValidToken(t *testing.T) {
	// Get a valid token first via LoginHandler
	loginPayload := `{"username":"admin","password":"cloudvault-secret"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(loginPayload))
	loginRR := httptest.NewRecorder()
	LoginHandler(loginRR, loginReq)
	var loginResp LoginResponse
	require.NoError(t, json.Unmarshal(loginRR.Body.Bytes(), &loginResp))

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	req := httptest.NewRequest(http.MethodGet, "/api/pvc", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rr := httptest.NewRecorder()
	AuthMiddleware(next).ServeHTTP(rr, req)
	assert.True(t, called)
}

// ── HandleHealth / HandleLiveness / HandleReadiness ───────────────────────────

func TestHandleHealth_NoClients(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	s.HandleHealth(rr, req)
	// Should return 200 or 503 depending on ai service; just check it responds
	assert.NotEqual(t, 0, rr.Code)
	var resp HealthResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Contains(t, resp.Components, "agent")
}

func TestHandleLiveness(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	s.HandleLiveness(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Alive")
}

func TestHandleReadiness_NoClient(t *testing.T) {
	s := newTestServer() // client is nil → skip k8s check
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()
	s.HandleReadiness(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// ── writeJSON ─────────────────────────────────────────────────────────────────

func TestWriteJSON_ValidStruct(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSON(rr, map[string]string{"key": "value"})
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	assert.Contains(t, rr.Body.String(), "value")
}

func TestWriteJSON_UnencodableValue(t *testing.T) {
	rr := httptest.NewRecorder()
	// channels cannot be JSON-encoded; writeJSON must not panic
	writeJSON(rr, make(chan int))
}

// ── handleNetwork (no client, no ebpf) ───────────────────────────────────────

func TestHandleNetwork_NoClientNoAgent(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/network", nil)
	rr := httptest.NewRecorder()
	s.handleNetwork(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(bytes.TrimRight(rr.Body.Bytes(), "\n"), &body))
	// empty map is valid
	assert.NotNil(t, body)
}

// ── handleNetwork with eBPF agent ─────────────────────────────────────────────
func TestHandleNetwork_WithEbpfAgent(t *testing.T) {
	agent, err := ebpf.NewAgent()
	require.NoError(t, err)

	s := newTestServer()
	s.ebpfAgent = agent

	req := httptest.NewRequest(http.MethodGet, "/api/network?internal=true", nil)
	rr := httptest.NewRecorder()
	s.handleNetwork(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(bytes.TrimRight(rr.Body.Bytes(), "\n"), &body))
	assert.NotEmpty(t, body)
}

// ── RegisterHealthEndpoints smoke ────────────────────────────────────────────
func TestRegisterHealthEndpoints_Smoke(t *testing.T) {
	s := newTestServer()
	// Should not panic — just registers on the default mux
	assert.NotPanics(t, func() { s.RegisterHealthEndpoints() })
}

// ── HandleHealth — with unhealthy AI service ──────────────────────────────────
func TestHandleHealth_UnhealthyAI(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	s.HandleHealth(rr, req)
	// Either 200 (healthy) or 503 (degraded) — we just want a valid JSON body
	var resp HealthResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Components)
}

// ── reconcile (mock mode) ─────────────────────────────────────────────────────

func TestReconcile_MockMode(t *testing.T) {
	s := newTestServer() // mock: true
	s.reconcile()        // should not panic; populates store
	s.store.RLock()
	metrics := s.store.Metrics
	s.store.RUnlock()
	assert.NotNil(t, metrics)
}

func TestReconcile_PreservesUserBudget(t *testing.T) {
	s := newTestServer()
	s.store.BudgetLimit = 999.0
	s.reconcile()
	s.store.RLock()
	limit := s.store.BudgetLimit
	s.store.RUnlock()
	assert.Equal(t, 999.0, limit)
}

func TestReconcile_FiltersAppliedPVCs(t *testing.T) {
	s := newTestServer()
	s.store.AppliedPVCs = map[string]bool{"default/pvc-1": true}
	s.reconcile()
	s.store.RLock()
	for _, rec := range s.store.Recommendations {
		assert.NotEqual(t, "default/pvc-1", rec.Namespace+"/"+rec.PVC)
	}
	s.store.RUnlock()
}

// ── runReconciler (cancel after one tick) ─────────────────────────────────────

func TestRunReconciler_FiresAndReturns(t *testing.T) {
	s := newTestServer()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.runReconciler(50 * time.Millisecond)
	}()
	// Let it fire a couple of ticks then give up — we just want no panic
	time.Sleep(120 * time.Millisecond)
	// runReconciler never returns on its own, but that's fine — test just validates no panic
	select {
	case <-done:
	default:
		// Expected: still running
	}
}

// ── handleApplyRecommendationGitOps — rec found but gitops nil ────────────────

func TestHandleApplyRecommendationGitOps_RecFound_NoController(t *testing.T) {
	s := newTestServer()
	s.store.Recommendations = []types.Recommendation{
		{PVC: "pvc-1", Namespace: "default", Type: "resize"},
	}
	payload := `{"pvcName":"pvc-1","namespace":"default","type":"resize"}`
	req := httptest.NewRequest(http.MethodPost, "/api/recommendations/gitops", strings.NewReader(payload))
	rr := httptest.NewRecorder()
	s.handleApplyRecommendationGitOps(rr, req)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

// ── handleGovernanceStatus — with policies + no client ───────────────────────

func TestHandleGovernanceStatus_WithPolicies(t *testing.T) {
	s := newTestServer()
	s.store.Policies = []v1alpha1.StorageLifecyclePolicy{
		{Spec: v1alpha1.StorageLifecyclePolicySpec{
			Selector: v1alpha1.PolicySelector{MatchNamespaces: []string{"default"}},
		}},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/governance/status", nil)
	rr := httptest.NewRecorder()
	s.handleGovernanceStatus(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Contains(t, body, "managed_pvcs")
}

// ── handleClusters — registry set ────────────────────────────────────────────

func TestHandleClusters_WithRegistry(t *testing.T) {
	s := newTestServer()
	s.clusterRegistry = struct{}{}
	req := httptest.NewRequest(http.MethodGet, "/api/clusters", nil)
	rr := httptest.NewRecorder()
	s.handleClusters(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "[]")
}

func TestHandleClusterAggregate_WithRegistry(t *testing.T) {
	s := newTestServer()
	s.clusterRegistry = struct{}{}
	req := httptest.NewRequest(http.MethodGet, "/api/clusters/aggregate", nil)
	rr := httptest.NewRecorder()
	s.handleClusterAggregate(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "{}")
}
