package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/cloudvault-io/cloudvault/pkg/collector"
	"github.com/cloudvault-io/cloudvault/pkg/cost"
	"github.com/cloudvault-io/cloudvault/pkg/dashboard"
	"github.com/cloudvault-io/cloudvault/pkg/integrations"
	"github.com/cloudvault-io/cloudvault/pkg/types"
)

var (
	// Build info
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// Common flags for subcommands
	// We need to define them for each subcommand's FlagSet

	switch command {
	case "cost", "summary":
		costCmd := flag.NewFlagSet("cost", flag.ExitOnError)
		// ... existing flags ...
		kubeconfig := costCmd.String("kubeconfig", "", "Path to kubeconfig file")
		namespace := costCmd.String("namespace", "", "Filter by namespace")
		promURL := costCmd.String("prometheus", "", "Prometheus URL (e.g., http://localhost:9090)")

		if err := costCmd.Parse(os.Args[2:]); err != nil {
			fmt.Println("Error parsing flags:", err)
			os.Exit(1)
		}
		handleCostCommand(*kubeconfig, *namespace, *promURL)

	case "recommendations", "rec", "recs", "storage":
		recCmd := flag.NewFlagSet("recommendations", flag.ExitOnError)
		// ... existing flags ...
		kubeconfig := recCmd.String("kubeconfig", "", "Path to kubeconfig file")
		namespace := recCmd.String("namespace", "", "Filter by namespace")
		promURL := recCmd.String("prometheus", "", "Prometheus URL (e.g., http://localhost:9090)")

		if err := recCmd.Parse(os.Args[2:]); err != nil {
			fmt.Println("Error parsing flags:", err)
			os.Exit(1)
		}
		handleRecommendationsCommand(*kubeconfig, *namespace, *promURL)

	case "dashboard", "dash", "ui":
		dashCmd := flag.NewFlagSet("dashboard", flag.ExitOnError)
		kubeconfig := dashCmd.String("kubeconfig", "", "Path to kubeconfig file")
		promURL := dashCmd.String("prometheus", "", "Prometheus URL (e.g., http://localhost:9090)")
		port := dashCmd.Int("port", 8080, "Port to run the dashboard on")
		mock := dashCmd.Bool("mock", false, "Run in mock mode with synthetic data")

		if err := dashCmd.Parse(os.Args[2:]); err != nil {
			fmt.Println("Error parsing flags:", err)
			os.Exit(1)
		}
		handleDashboardCommand(*kubeconfig, *promURL, *port, *mock)

	case "version":
		handleVersionCommand()

	case "help", "--help", "-h":
		printUsage()

	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("CloudVault - Multi-Cloud Kubernetes Storage Cost Intelligence")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  cloudvault <command> [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  cost              Show storage costs")
	fmt.Println("  cost              Show storage costs")
	fmt.Println("  recommendations   Show optimization recommendations")
	fmt.Println("  dashboard         Start the web dashboard")
	fmt.Println("  version           Show version information")
	fmt.Println("  help              Show this help message")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --kubeconfig      Path to kubeconfig file")
	fmt.Println("  --namespace       Filter by namespace")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  cloudvault cost")
	fmt.Println("  cloudvault cost --namespace production")
	fmt.Println("  cloudvault recommendations")
	fmt.Println("  cloudvault recommendations")
	fmt.Println("  cloudvault recommendations --kubeconfig ~/.kube/config")
	fmt.Println("  cloudvault dashboard")
}

func handleVersionCommand() {
	fmt.Printf("CloudVault CLI %s\n", Version)
	fmt.Printf("Commit: %s\n", Commit)
	fmt.Printf("Built:  %s\n", BuildDate)
}

func handleCostCommand(kubeconfig, namespace string, promURL string) {
	ctx := context.Background()

	var clusterInfo *types.ClusterInfo
	var client *collector.KubernetesClient
	var promClient *integrations.PrometheusClient
	var err error

	if promURL != "" {
		promClient, err = integrations.NewPrometheusClient(promURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Warning: Failed to create Prometheus client: %v\n", err)
		} else {
			fmt.Println("🔌 Prometheus integration enabled")
		}
	}

	// Create Kubernetes client
	client, err = collector.NewKubernetesClient(kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		os.Exit(1)
	}

	// Get cluster info
	clusterInfo, err = client.GetClusterInfo(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("📊 Cluster: %s (%s %s)\n\n", clusterInfo.Name, clusterInfo.Provider, clusterInfo.Region)

	// Collect PVC metrics
	var metrics []types.PVCMetric

	pvcCollector := collector.NewPVCCollector(client, promClient)
	if namespace != "" {
		metrics, err = pvcCollector.CollectByNamespace(ctx, namespace)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("📁 Namespace: %s\n\n", namespace)
	} else {
		metrics, err = pvcCollector.CollectAll(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
			os.Exit(1)
		}
	}

	if len(metrics) == 0 {
		fmt.Println("ℹ️  No PVCs found")
		return
	}

	// Calculate costs
	calculator := cost.NewCalculator()
	summary := calculator.GenerateSummary(metrics, clusterInfo.Provider)

	// Display total cost
	fmt.Printf("💰 Total Monthly Cost: %s (%s/year)\n\n",
		cost.FormatCostPerMonth(summary.TotalMonthlyCost),
		cost.FormatCostPerYear(summary.TotalMonthlyCost))

	// Display cost by namespace
	if len(summary.ByNamespace) > 0 {
		fmt.Println("📊 Cost by Namespace:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAMESPACE\tMONTHLY\tANNUAL\t% OF TOTAL")

		for ns, monthlyCost := range summary.ByNamespace {
			percentage := (monthlyCost / summary.TotalMonthlyCost) * 100
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%.1f%%\n",
				ns,
				cost.FormatCost(monthlyCost),
				cost.FormatCost(monthlyCost*12),
				percentage)
		}
		_ = w.Flush()
		fmt.Println()
	}

	// Display cost by storage class
	if len(summary.ByStorageClass) > 0 {
		fmt.Println("💿 Cost by Storage Class:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		_, _ = fmt.Fprintln(w, "STORAGE CLASS\tMONTHLY\tANNUAL\t% OF TOTAL")

		for sc, monthlyCost := range summary.ByStorageClass {
			percentage := (monthlyCost / summary.TotalMonthlyCost) * 100
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%.1f%%\n",
				sc,
				cost.FormatCost(monthlyCost),
				cost.FormatCost(monthlyCost*12),
				percentage)
		}
		_ = w.Flush()
		fmt.Println()
	}

	// Display top expensive PVCs
	if len(summary.TopExpensive) > 0 {
		fmt.Println("🔝 Top 10 Most Expensive PVCs:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAMESPACE\tNAME\tSIZE\tSTORAGE CLASS\tMONTHLY\tANNUAL")

		count := len(summary.TopExpensive)
		if count > 10 {
			count = 10
		}

		for i := 0; i < count; i++ {
			m := summary.TopExpensive[i]
			_, _ = fmt.Fprintf(w, "%s\t%s\t%.0fGB\t%s\t%s\t%s\n",
				m.Namespace,
				m.Name,
				m.SizeGB(),
				m.StorageClass,
				cost.FormatCost(m.MonthlyCost),
				cost.FormatCost(m.MonthlyCost*12))
		}
		_ = w.Flush()
		fmt.Println()
	}

	// Show summary stats
	totalSize := int64(0)
	for _, m := range metrics {
		totalSize += m.SizeBytes
	}
	totalSizeGB := float64(totalSize) / (1024 * 1024 * 1024)

	fmt.Printf("📈 Summary:\n")
	fmt.Printf("   Total PVCs: %d\n", len(metrics))
	fmt.Printf("   Total Size: %.2f GB\n", totalSizeGB)
	fmt.Printf("   Average Cost per GB: $%.4f/month\n", summary.TotalMonthlyCost/totalSizeGB)
}

func handleRecommendationsCommand(kubeconfig, namespace string, promURL string) {
	ctx := context.Background()

	var clusterInfo *types.ClusterInfo
	var client *collector.KubernetesClient
	var promClient *integrations.PrometheusClient
	var err error

	if promURL != "" {
		promClient, err = integrations.NewPrometheusClient(promURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Warning: Failed to create Prometheus client: %v\n", err)
		} else {
			fmt.Println("🔌 Prometheus integration enabled")
		}
	}

	// Create Kubernetes client
	client, err = collector.NewKubernetesClient(kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		os.Exit(1)
	}

	// Get cluster info
	clusterInfo, err = client.GetClusterInfo(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("📊 Cluster: %s (%s %s)\n\n", clusterInfo.Name, clusterInfo.Provider, clusterInfo.Region)

	// Collect metrics
	var metrics []types.PVCMetric

	// Using real collector
	pvcCollector := collector.NewPVCCollector(client, promClient)
	if namespace != "" {
		metrics, err = pvcCollector.CollectByNamespace(ctx, namespace)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("📁 Namespace: %s\n\n", namespace)
	} else {
		metrics, err = pvcCollector.CollectAll(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
			os.Exit(1)
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		os.Exit(1)
	}

	if len(metrics) == 0 {
		fmt.Println("ℹ️  No PVCs found")
		return
	}

	// Calculate costs for all metrics first (needed for optimization savings)
	calculator := cost.NewCalculator()
	for i := range metrics {
		metrics[i].MonthlyCost = calculator.CalculatePVCCost(&metrics[i], clusterInfo.Provider)
	}

	// Generate recommendations
	optimizer := cost.NewOptimizer()
	recommendations := optimizer.GenerateRecommendations(metrics, clusterInfo.Provider)

	if len(recommendations) == 0 {
		fmt.Println("✅ No optimization opportunities found!")
		fmt.Println("   Your storage is well-optimized. 🎉")
		return
	}

	// Calculate total savings
	totalSavings := optimizer.CalculateTotalSavings(recommendations)

	fmt.Printf("💡 Found %d optimization opportunities\n\n", len(recommendations))
	fmt.Printf("💰 Total Potential Savings: %s/month (%s/year)\n\n",
		cost.FormatCost(totalSavings),
		cost.FormatCost(totalSavings*12))

	// Display recommendations
	for i, rec := range recommendations {
		displayRecommendation(i+1, rec)
	}

	// Show summary by type
	fmt.Println("\n📊 Summary by Type:")
	storageClassRecs := optimizer.FilterByType(recommendations, "storage_class")
	zombieRecs := optimizer.FilterByType(recommendations, "delete_zombie")
	resizeRecs := optimizer.FilterByType(recommendations, "resize")
	moveCloudRecs := optimizer.FilterByType(recommendations, "move_cloud")

	if len(storageClassRecs) > 0 {
		savings := 0.0
		for _, r := range storageClassRecs {
			savings += r.MonthlySavings
		}
		fmt.Printf("   Storage Class Changes: %d (%s/month)\n", len(storageClassRecs), cost.FormatCost(savings))
	}

	if len(zombieRecs) > 0 {
		savings := 0.0
		for _, r := range zombieRecs {
			savings += r.MonthlySavings
		}
		fmt.Printf("   Zombie Volumes: %d (%s/month)\n", len(zombieRecs), cost.FormatCost(savings))
	}

	if len(resizeRecs) > 0 {
		savings := 0.0
		for _, r := range resizeRecs {
			savings += r.MonthlySavings
		}
		fmt.Printf("   Resize Volumes: %d (%s/month)\n", len(resizeRecs), cost.FormatCost(savings))
	}

	if len(moveCloudRecs) > 0 {
		savings := 0.0
		for _, r := range moveCloudRecs {
			savings += r.MonthlySavings
		}
		fmt.Printf("   Cross-Cloud Migration: %d (%s/month)\n", len(moveCloudRecs), cost.FormatCost(savings))
	}

	// Show quick wins
	quickWins := optimizer.GetQuickWins(recommendations)
	if len(quickWins) > 0 {
		fmt.Printf("\n⚡ Quick Wins (Low Impact, High Savings): %d recommendations\n", len(quickWins))
	}
}

func displayRecommendation(num int, rec types.Recommendation) {
	// Determine emoji based on type
	emoji := "💡"
	switch rec.Type {
	case "delete_zombie":
		emoji = "🗑️"
	case "storage_class":
		emoji = "💿"
	case "resize":
		emoji = "📏"
	case "move_cloud":
		emoji = "🌐"
	}

	// Determine impact indicator
	impactIndicator := ""
	switch rec.Impact {
	case "low":
		impactIndicator = "🟢 Low impact"
	case "medium":
		impactIndicator = "🟡 Medium impact"
	case "high":
		impactIndicator = "🔴 High impact"
	}

	fmt.Printf("%s %d. %s\n", emoji, num, rec.Reasoning)
	fmt.Printf("   PVC: %s/%s\n", rec.Namespace, rec.PVC)
	fmt.Printf("   Current: %s → Recommended: %s\n", rec.CurrentState, rec.RecommendedState)
	fmt.Printf("   Savings: %s/month (%s/year) | %s\n",
		cost.FormatCost(rec.MonthlySavings),
		cost.FormatCost(rec.MonthlySavings*12),
		impactIndicator)
	fmt.Println()
}

func handleDashboardCommand(kubeconfig string, promURL string, port int, mock bool) {
	ctx := context.Background()

	var clusterInfo *types.ClusterInfo
	var client *collector.KubernetesClient
	var promClient *integrations.PrometheusClient
	var err error

	// Initialize Prometheus
	if promURL != "" {
		promClient, err = integrations.NewPrometheusClient(promURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Warning: Failed to create Prometheus client: %v\n", err)
		} else {
			fmt.Println("🔌 Prometheus integration enabled")
		}
	}

	provider := "aws" // Default
	if !mock {
		// Real client
		client, err = collector.NewKubernetesClient(kubeconfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error creating Kubernetes client: %v\n", err)
			os.Exit(1)
		}

		// Get cluster info to determine provider
		clusterInfo, err = client.GetClusterInfo(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error getting cluster info: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("🔌 Connected to %s (%s)\n", clusterInfo.Name, clusterInfo.Provider)
		provider = clusterInfo.Provider
	} else {
		fmt.Println("🧪 Running in MOCK mode")
	}

	// Start Server
	if provider == "" {
		provider = "aws" // Default fallback
	}

	server := dashboard.NewServer(client, promClient, provider, mock)
	if err := server.Start(port); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Dashboard server error: %v\n", err)
		os.Exit(1)
	}
}
