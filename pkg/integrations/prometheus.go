package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// PrometheusClient handles interactions with a Prometheus server.
// Refactored for Phase 3 to support batch vector queries and eliminate N+1 bottlenecks.
type PrometheusClient struct {
	baseURL string
	client  *http.Client
}

// NewPrometheusClient creates a new Prometheus client
func NewPrometheusClient(url string) (*PrometheusClient, error) {
	if url == "" {
		return nil, fmt.Errorf("prometheus URL cannot be empty")
	}
	return &PrometheusClient{
		baseURL: url,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// PVCUsageMetrics contains usage data fetched from Prometheus
type PVCUsageMetrics struct {
	UsedBytes       int64
	ReadBytesTotal  float64
	WriteBytesTotal float64
	LastActivity    time.Time
}

// GetAllPVCMetrics fetches usage metrics for all PVCs in the cluster in a single batch query.
// This eliminates the N+1 query problem and significantly reduces latency for large clusters.
func (p *PrometheusClient) GetAllPVCMetrics(ctx context.Context) (map[string]map[string]*PVCUsageMetrics, error) {
	// Query: sum by(persistentvolumeclaim, namespace) (kubelet_volume_stats_used_bytes)
	query := `sum by(persistentvolumeclaim, namespace) (kubelet_volume_stats_used_bytes)`
	results, err := p.queryVector(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch batch PVC metrics: %w", err)
	}

	metricsMap := make(map[string]map[string]*PVCUsageMetrics)
	for _, res := range results {
		pvc := res.Labels["persistentvolumeclaim"]
		ns := res.Labels["namespace"]
		if pvc == "" || ns == "" {
			continue
		}

		if metricsMap[ns] == nil {
			metricsMap[ns] = make(map[string]*PVCUsageMetrics)
		}

		metricsMap[ns][pvc] = &PVCUsageMetrics{
			UsedBytes:    int64(res.Value),
			LastActivity: time.Now(), // If it's in Prometheus, it's recently active
		}
	}

	return metricsMap, nil
}

// queryVector executes a PromQL query that returns a vector of results
func (p *PrometheusClient) queryVector(ctx context.Context, query string) ([]QueryResult, error) {
	u, err := url.Parse(fmt.Sprintf("%s/api/v1/query", p.baseURL))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus returned status %d", resp.StatusCode)
	}

	var result struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric map[string]string `json:"metric"`
				Value  []interface{}     `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("query failed: %s", result.Status)
	}

	var queryResults []QueryResult
	for _, res := range result.Data.Result {
		if len(res.Value) < 2 {
			continue
		}
		strVal, ok := res.Value[1].(string)
		if !ok {
			continue
		}
		val, _ := strconv.ParseFloat(strVal, 64)
		queryResults = append(queryResults, QueryResult{
			Labels: res.Metric,
			Value:  val,
		})
	}

	return queryResults, nil
}

// QueryResult represents a single Prometheus vector result
type QueryResult struct {
	Labels map[string]string
	Value  float64
}

// GetPVCMetrics fetches usage metrics for a specific PVC (Legacy/Fallback)
func (p *PrometheusClient) GetPVCMetrics(ctx context.Context, pvcName, namespace string) (*PVCUsageMetrics, error) {
	metrics := &PVCUsageMetrics{}

	// 1. Get Used Bytes
	// Query: sum(kubelet_volume_stats_used_bytes{persistentvolumeclaim="<pvc>", namespace="<ns>"})
	// We use sum() to handle potential duplicate metrics or instance labels
	queryUsed := fmt.Sprintf(`sum(kubelet_volume_stats_used_bytes{persistentvolumeclaim="%s", namespace="%s"})`, pvcName, namespace)
	val, err := p.queryScalar(ctx, queryUsed)
	if err == nil {
		metrics.UsedBytes = int64(val)
	}

	// 2. Get Activity (Read + Write) to detect Zombies
	// We check for any I/O activity in the last 24 hours.
	// Query: sum(rate(container_fs_reads_bytes_total{namespace="<ns>"}[24h])) + sum(rate(container_fs_writes_bytes_total{namespace="<ns>"}[24h]))
	// Note: Mapping PVC to container usually requires joining kube_pod_spec_volumes_persistentvolumeclaims_info
	// For MVP, we'll try a simpler heuristic: check if `kubelet_volume_stats_used_bytes` has changed variance over time?
	// No, that's size.
	// Let's stick to `UsedBytes` for now as the primary reliable metric for "Oversized".
	// For "Zombie", we can check if `kubelet_volume_stats_used_bytes` > 0 BUT no pod is mounted?
	// Actually, `kubelet_volume_stats` only exists if a Pod is mounted!
	// So if `kubelet_volume_stats` is missing for a PVC that is Bound, it MIGHT be unused (Zombie candidate).

	// Enhancing Zombie Logic:
	// If we get a result for `kubelet_volume_stats_used_bytes`, the volume is mounted and active on a node -> NOT Zombie (mostly).
	// If we get NO result, but PVC exists -> Likely Zombie (not mounted).

	if err == nil {
		// Metric exists, so volume is mounted.
		// We'll mark it as "active recently"
		metrics.LastActivity = time.Now()
	}

	return metrics, nil
}

// queryScalar executes a PromQL query that returns a single scalar value
func (p *PrometheusClient) queryScalar(ctx context.Context, query string) (float64, error) {
	u, err := url.Parse(fmt.Sprintf("%s/api/v1/query", p.baseURL))
	if err != nil {
		return 0, err
	}
	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return 0, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("prometheus returned status %d", resp.StatusCode)
	}

	var result struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Value []interface{} `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if result.Status != "success" {
		return 0, fmt.Errorf("query failed: %s", result.Status)
	}

	if len(result.Data.Result) == 0 {
		return 0, fmt.Errorf("no data")
	}

	// Prometheus value is [timestamp, string_value]
	// Example: [1435781456.789, "1"]
	if len(result.Data.Result[0].Value) < 2 {
		return 0, fmt.Errorf("unexpected value format")
	}

	strVal, ok := result.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, fmt.Errorf("value is not a string")
	}

	return strconv.ParseFloat(strVal, 64)
}
