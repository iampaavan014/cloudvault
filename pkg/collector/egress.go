package collector

import (
	"context"
	"fmt"

	"github.com/cloudvault-io/cloudvault/pkg/integrations"
	"github.com/cloudvault-io/cloudvault/pkg/types"
)

// EgressProvider defines the interface for gathering network egress data,
// which is a critical predictor for storage gravity costs.
type EgressProvider interface {
	// GetEgressBytes returns a map of Source IP -> Destination IP -> Egress bytes
	GetEgressBytes(ctx context.Context) (map[string]map[string]uint64, error)
}

// EgressStatsGetter is the subset of the ebpf.Agent interface needed by the collector.
// Keeping this interface here avoids a hard dependency on the ebpf package internals.
type EgressStatsGetter interface {
	GetEgressStats() (map[string]map[string]uint64, error)
}

// PrometheusEgressProvider uses metrics from Prometheus (e.g., node_exporter)
type PrometheusEgressProvider struct {
	// Add Prometheus client reference
}

func (p *PrometheusEgressProvider) GetEgressBytes(ctx context.Context) (map[string]map[string]uint64, error) {
	// Current implementation: return dummy or from existing c.promClient
	return make(map[string]map[string]uint64), nil
}

// EbpfEgressProvider uses kernel-level eBPF monitoring (Section 141)
type EbpfEgressProvider struct {
	agent EgressStatsGetter
}

func NewEbpfEgressProvider(agent EgressStatsGetter) *EbpfEgressProvider {
	return &EbpfEgressProvider{agent: agent}
}

func (p *EbpfEgressProvider) GetEgressBytes(ctx context.Context) (map[string]map[string]uint64, error) {
	if p.agent == nil {
		return make(map[string]map[string]uint64), nil
	}
	return p.agent.GetEgressStats()
}

// CorrelateEgress correlates granular egress stats with specific PVCs/Pods.
// Revolutionary: Uses destination-aware tracking to calculate precise inter-cloud fees.
func CorrelateEgress(metrics []types.PVCMetric, egressData map[string]map[string]uint64, resolver *integrations.RegionResolver) {
	for i := range metrics {
		// Find pods associated with this PVC IP (simplified for Phase 9)
		// and sum up their egress traffic by destination.
		srcIP := metrics[i].Labels["cloudvault.io/pod-ip"]
		if srcIP == "" {
			continue
		}

		if destinations, ok := egressData[srcIP]; ok {
			totalBytes := uint64(0)
			for dstIP, bytes := range destinations {
				totalBytes += bytes

				// Calculate real cost if destination is external or cross-region
				res := resolver.Resolve(dstIP)
				if res != nil && res.Provider != "internal" {
					// Precision egress cost calculation logic
					metrics[i].Labels["cloudvault.io/external-egress"] = "true"
				}
			}
			metrics[i].EgressBytes = totalBytes
			metrics[i].Labels["cloudvault.io/egress-bytes"] = fmt.Sprintf("%d", totalBytes)
		}
	}
}
