package integrations

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// CollectionTotal counts the number of metric collection cycles
	CollectionTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cloudvault_collection_total",
		Help: "The total number of PVC metric collection cycles",
	}, []string{"status"})

	// CollectionDuration measures the time taken for a collection cycle
	CollectionDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "cloudvault_collection_duration_seconds",
		Help:    "Time taken to collect metrics for all PVCs",
		Buckets: prometheus.DefBuckets,
	})

	// PVCCount tracks the number of PVCs managed by CloudVault
	PVCCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cloudvault_managed_pvcs",
		Help: "The total number of PVCs currently being tracked",
	})
)
