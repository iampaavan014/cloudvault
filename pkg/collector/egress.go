package collector

import (
	"context"
	"fmt"

	"github.com/cloudvault-io/cloudvault/pkg/types"
)

// EgressProvider defines the interface for gathering network egress data,
// which is a critical predictor for storage gravity costs.
type EgressProvider interface {
	// GetEgressBytes returns a map of IP addresses to egress bytes
	GetEgressBytes(ctx context.Context) (map[string]uint64, error)
}

// PrometheusEgressProvider uses metrics from Prometheus (e.g., node_exporter)
type PrometheusEgressProvider struct {
	// Add Prometheus client reference
}

func (p *PrometheusEgressProvider) GetEgressBytes(ctx context.Context) (map[string]uint64, error) {
	// Current implementation: return dummy or from existing c.promClient
	return make(map[string]uint64), nil
}

// EbpfEgressProvider uses kernel-level eBPF monitoring (Section 141)
type EbpfEgressProvider struct {
	// This would wrap the ebpf.Agent implemented in pkg/ebpf
}

func (p *EbpfEgressProvider) GetEgressBytes(ctx context.Context) (map[string]uint64, error) {
	// In production, this calls the eBPF agent's map iteration logic.
	// We return an empty map if the eBPF agent is not initialized.
	return make(map[string]uint64), nil
}

// CorrelateEgress correlates global egress stats with specific PVCs/Pods
func CorrelateEgress(metrics []types.PVCMetric, egressData map[string]uint64) {
	// This logic uses the SIG (Phase 7) to find which Pods own which PVCs
	// and matches their IPs to egress data.
	for i := range metrics {
		// Example: If a pod IP matches an entry in egressData,
		// we assign that traffic to the PVC used by that pod.
		// (Simplified for Phase 6)
		if val, ok := egressData[metrics[i].Namespace]; ok {
			metrics[i].EgressBytes = val
			metrics[i].Labels["cloudvault.io/egress-bytes"] = fmt.Sprintf("%d", val)
		}
	}
}
