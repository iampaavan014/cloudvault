package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"log/slog"

	"github.com/cloudvault-io/cloudvault/pkg/collector"
	"github.com/cloudvault-io/cloudvault/pkg/cost"
	"github.com/cloudvault-io/cloudvault/pkg/dashboard"
	"github.com/cloudvault-io/cloudvault/pkg/ebpf"
	"github.com/cloudvault-io/cloudvault/pkg/graph"
	"github.com/cloudvault-io/cloudvault/pkg/integrations"
	"github.com/cloudvault-io/cloudvault/pkg/orchestrator/lifecycle"
	"github.com/cloudvault-io/cloudvault/pkg/types"
)

var (
	// Build info (set via ldflags)
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"

	// Flags
	configFile      = flag.String("config", "", "Path to configuration file")
	kubeconfig      = flag.String("kubeconfig", "", "Path to kubeconfig file")
	collectInterval = flag.Duration("interval", 5*time.Minute, "Metrics collection interval")
	namespace       = flag.String("namespace", "", "Namespace to monitor")
	showVersion     = flag.Bool("version", false, "Show version information")
	promURL         = flag.String("prometheus", "", "Prometheus URL")
	tsdbConn        = flag.String("timescale", "", "TimescaleDB connection string")
	neo4jURI        = flag.String("neo4j-uri", "", "Neo4j URI for Storage Intelligence Graph")
	neo4jUser       = flag.String("neo4j-user", "neo4j", "Neo4j username")
	neo4jPass       = flag.String("neo4j-password", "", "Neo4j password")
)

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("CloudVault Agent %s\n", Version)
		os.Exit(0)
	}

	// Load config (Phase 3: Unified Config)
	cfg, err := integrations.LoadConfig(*configFile)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Override config with flags if provided
	if *kubeconfig != "" {
		cfg.KubeConfig = *kubeconfig
	}
	if *collectInterval != 5*time.Minute {
		cfg.Interval = *collectInterval
	}
	if *namespace != "" {
		cfg.Namespace = *namespace
	}
	if *promURL != "" {
		cfg.PrometheusURL = *promURL
	}
	if *tsdbConn != "" {
		cfg.TimescaleConn = *tsdbConn
	}
	if *neo4jURI != "" {
		cfg.Neo4jURI = *neo4jURI
	}
	if *neo4jUser != "" {
		cfg.Neo4jUser = *neo4jUser
	}
	if *neo4jPass != "" {
		cfg.Neo4jPassword = *neo4jPass
	}

	slog.Info("CloudVault Agent starting", "version", Version, "interval", cfg.Interval)

	// Create Kubernetes client
	client, err := collector.NewKubernetesClient(cfg.KubeConfig)
	if err != nil {
		slog.Error("Failed to create Kubernetes client", "error", err)
		os.Exit(1)
	}

	// Create Prometheus client (optional)
	var promClient *integrations.PrometheusClient
	actualPromURL := cfg.PrometheusURL
	if actualPromURL == "" {
		actualPromURL = os.Getenv("PROMETHEUS_URL")
	}

	if actualPromURL != "" {
		promClient, err = integrations.NewPrometheusClient(actualPromURL)
		if err != nil {
			slog.Warn("Failed to create Prometheus client", "error", err)
		} else {
			slog.Info("Prometheus integration enabled", "url", actualPromURL)
		}
	}

	// Get cluster info
	ctx := context.Background()
	clusterInfo, err := client.GetClusterInfo(ctx)
	if err != nil {
		slog.Error("Failed to get cluster info", "error", err)
		os.Exit(1)
	}
	slog.Info("Connected to cluster",
		"name", clusterInfo.Name,
		"provider", clusterInfo.Provider,
		"region", clusterInfo.Region)

	// Initialize Storage Intelligence Graph (SIG) - Phase 4 Pillar 1
	var sig *graph.SIG
	if cfg.Neo4jURI != "" && cfg.Neo4jPassword != "" {
		sig, err = graph.NewSIG(cfg.Neo4jURI, cfg.Neo4jUser, cfg.Neo4jPassword)
		if err != nil {
			slog.Warn("Failed to initialize Storage Intelligence Graph", "error", err)
		} else {
			slog.Info("Storage Intelligence Graph (Neo4j) enabled", "uri", cfg.Neo4jURI)
			defer func() { _ = sig.Close(ctx) }()
		}
	}

	// Initialize eBPF Agent (Functional kernel monitoring)
	ebpfAgent, err := ebpf.NewAgent()
	if err != nil {
		slog.Warn("Failed to initialize eBPF agent (probably non-Linux or low privs)", "error", err)
	} else {
		slog.Info("eBPF kernel monitoring enabled")
		defer func() { _ = ebpfAgent.Close() }()
		// Attach to the first non-loopback UP interface
		attached := false
		if ifaces, ierr := net.Interfaces(); ierr == nil {
			for _, iface := range ifaces {
				if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
					continue
				}
				if _, aerr := ebpfAgent.AttachToInterface(iface.Name); aerr == nil {
					slog.Info("eBPF attached to interface", "iface", iface.Name)
					attached = true
					break
				} else {
					slog.Debug("failed to attach eBPF to interface", "iface", iface.Name, "error", aerr)
				}
			}
		}
		if !attached {
			slog.Warn("Could not attach eBPF to any interface")
		}
	}

	// Create PVC collector with integrated egress intelligence
	ebpfProvider := collector.NewEbpfEgressProvider(ebpfAgent)
	pvcCollector := collector.NewPVCCollector(client, promClient)
	pvcCollector.SetEgressProvider(ebpfProvider)

	// Phase 10: Initialize Multi-Cloud Pricing (Revolutionary - ZERO simulations)
	awsClient := integrations.NewAWSClient(cfg)
	gcpClient := integrations.NewGCPClient(cfg)
	azureClient := integrations.NewAzureClient(cfg)
	pricingProvider := cost.NewMultiCloudPricingProvider(awsClient, gcpClient, azureClient)
	calculator := cost.NewCalculatorWithProvider(pricingProvider)

	// Create TimescaleDB client (Phase 22: Metrics Persistence)
	var tsdb *graph.TimescaleDB
	if cfg.TimescaleConn != "" {
		tsdb, err = graph.NewTimescaleDB(cfg.TimescaleConn)
		if err != nil {
			slog.Warn("Failed to connect to TimescaleDB", "error", err)
		} else {
			slog.Info("TimescaleDB persistence enabled")
			defer func() { _ = tsdb.Close() }()
		}
	}

	// Phase 12: Initialize Autonomous Lifecycle Controller with SIG integration
	recommender := lifecycle.NewIntelligentRecommender(tsdb)
	lc := lifecycle.NewLifecycleController(cfg.Interval, client, recommender, sig, tsdb)
	lc.SetPVCCollector(pvcCollector)

	// Initial policy fetch
	policies, err := client.ListStoragePolicies(ctx)
	if err != nil {
		slog.Warn("Failed to fetch storage policies", "error", err)
	} else {
		slog.Info("Fetched storage policies", "count", len(policies))
		lc.SetPolicies(policies)
	}

	// Start CRD watcher for dynamic policy updates
	go watchStoragePolicies(ctx, client, lc)

	// Start lifecycle controller in background
	go func() {
		if err := lc.Start(ctx); err != nil {
			slog.Error("Lifecycle controller failed", "error", err)
		}
	}()

	// Start Integrated Dashboard Server (Phase 4 Pillar 4)
	dashServer := dashboard.NewServer(client, promClient, clusterInfo.Provider, false, ebpfAgent)
	go func() {
		if err := dashServer.Start(8080); err != nil {
			slog.Error("Dashboard server failed", "error", err)
		}
	}()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create ticker for periodic collection
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	// Collect immediately on startup
	slog.Info("Starting metrics collection loop")
	metrics := collectAndDisplay(ctx, pvcCollector, cfg.Namespace, calculator, clusterInfo.Provider)
	if tsdb != nil && len(metrics) > 0 {
		if err := tsdb.RecordMetrics(ctx, metrics); err != nil {
			slog.Error("Failed to record metrics to TSDB", "error", err)
		}
	}
	if sig != nil && len(metrics) > 0 {
		if err := sig.SyncPVCs(ctx, metrics); err != nil {
			slog.Error("Failed to sync PVCs to SIG", "error", err)
		}
	}

	// Main loop
	for {
		select {
		case <-ticker.C:
			metrics := collectAndDisplay(ctx, pvcCollector, cfg.Namespace, calculator, clusterInfo.Provider)
			if tsdb != nil && len(metrics) > 0 {
				if err := tsdb.RecordMetrics(ctx, metrics); err != nil {
					slog.Error("Failed to record metrics to TSDB", "error", err)
				}
			}
			if sig != nil && len(metrics) > 0 {
				if err := sig.SyncPVCs(ctx, metrics); err != nil {
					slog.Error("Failed to sync PVCs to SIG", "error", err)
				}
			}
		case sig := <-sigChan:
			slog.Info("Shutting down gracefully", "signal", sig)
			return
		}
	}
}

// watchStoragePolicies watches for changes to StorageLifecyclePolicy CRDs and updates the controller
func watchStoragePolicies(ctx context.Context, client *collector.KubernetesClient, lc *lifecycle.LifecycleController) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	slog.Info("Started StorageLifecyclePolicy watcher")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			policies, err := client.ListStoragePolicies(ctx)
			if err != nil {
				slog.Error("Failed to fetch storage policies", "error", err)
				continue
			}
			lc.SetPolicies(policies)
			slog.Debug("Updated storage policies", "count", len(policies))
		}
	}
}

// collectAndDisplay triggers a PVC metrics collection cycle and prints the results to stdout.
func collectAndDisplay(ctx context.Context, collector *collector.PVCCollector, namespace string, calculator *cost.Calculator, provider string) []types.PVCMetric {
	var metrics []types.PVCMetric
	var err error

	if namespace != "" {
		metrics, err = collector.CollectByNamespace(ctx, namespace)
	} else {
		metrics, err = collector.CollectAll(ctx)
	}

	if err != nil {
		slog.Error("Collection error", "error", err)
		return nil
	}

	// Calculate summary
	totalCost := 0.0
	totalSize := int64(0)
	namespaceMap := make(map[string]float64)
	storageClassMap := make(map[string]float64)

	for i := range metrics {
		// Calculate cost before aggregation
		metrics[i].MonthlyCost = calculator.CalculatePVCCost(&metrics[i], provider)
		totalCost += metrics[i].MonthlyCost
		totalSize += metrics[i].SizeBytes
		namespaceMap[metrics[i].Namespace] += metrics[i].MonthlyCost
		storageClassMap[metrics[i].StorageClass] += metrics[i].MonthlyCost
	}

	totalSizeGB := float64(totalSize) / (1024 * 1024 * 1024)

	// Display summary
	fmt.Printf("✅ Found %d PVCs\n", len(metrics))
	fmt.Printf("💾 Total Size: %.2f GB\n", totalSizeGB)
	fmt.Printf("💰 Total Monthly Cost: $%.2f\n", totalCost)

	if len(namespaceMap) > 0 {
		fmt.Println("\n📊 Cost by Namespace:")
		for ns, cost := range namespaceMap {
			fmt.Printf("   %-30s $%.2f/mo\n", ns, cost)
		}
	}

	if len(storageClassMap) > 0 {
		fmt.Println("\n💿 Cost by Storage Class:")
		for sc, cost := range storageClassMap {
			fmt.Printf("   %-30s $%.2f/mo\n", sc, cost)
		}
	}

	// Display top 5 expensive PVCs
	if len(metrics) > 0 {
		fmt.Println("\n🔝 Top 5 Most Expensive PVCs:")
		count := len(metrics)
		if count > 5 {
			count = 5
		}
		for i := 0; i < count; i++ {
			m := metrics[i]
			sizeGB := float64(m.SizeBytes) / (1024 * 1024 * 1024)
			fmt.Printf("   %d. %s/%s\n", i+1, m.Namespace, m.Name)
			fmt.Printf("      Size: %.0f GB | Cost: $%.2f/mo | Class: %s\n",
				sizeGB, m.MonthlyCost, m.StorageClass)
		}
	}

	fmt.Printf("\n⏰ Next collection at %s\n\n", time.Now().Add(*collectInterval).Format("15:04:05"))

	return metrics
}
