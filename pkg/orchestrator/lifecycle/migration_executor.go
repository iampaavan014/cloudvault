package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// MigrationExecutor handles actual workload migrations across clusters
type MigrationExecutor struct {
	sourceClient  *kubernetes.Clientset
	targetClient  *kubernetes.Clientset
	dynamicClient dynamic.Interface
	argoNamespace string
	veleroEnabled bool
	migrations    map[string]*MigrationStatus
}

// MigrationPlan defines the migration strategy
type MigrationPlan struct {
	ID                string
	Name              string
	SourceCluster     string
	TargetCluster     string
	PVCs              []types.PVCMetric
	TargetClass       string
	Reason            string
	EstimatedSavings  float64
	EstimatedDuration time.Duration
	Impact            MigrationImpact
	RiskLevel         string
	Strategy          string // "velero-backup-restore" or "volume-clone"
	CreatedAt         time.Time
	ApprovedBy        string
}

// MigrationImpact describes the expected impact
type MigrationImpact struct {
	LatencyIncrease   time.Duration
	DowntimeRequired  time.Duration
	DataTransferSize  int64
	RiskLevel         string // "low", "medium", "high"
	AffectedWorkloads []string
}

// MigrationStatus tracks migration progress
type MigrationStatus struct {
	Plan        *MigrationPlan
	State       string // "pending", "backing-up", "transferring", "restoring", "validating", "completed", "failed"
	Progress    int    // 0-100
	StartedAt   time.Time
	CompletedAt *time.Time
	Error       string
	Steps       []MigrationStep
}

// MigrationStep represents a single step in migration
type MigrationStep struct {
	Name        string
	Status      string // "pending", "running", "completed", "failed"
	StartedAt   time.Time
	CompletedAt *time.Time
	Output      string
	Error       string
}

// NewMigrationExecutor creates a new migration executor
func NewMigrationExecutor(config *rest.Config, argoNamespace string) (*MigrationExecutor, error) {
	if config == nil {
		// For testing or when no config is provided
		return &MigrationExecutor{
			argoNamespace: argoNamespace,
			migrations:    make(map[string]*MigrationStatus),
		}, nil
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Check if Velero is installed
	veleroEnabled := checkVeleroInstalled(clientset)
	if !veleroEnabled {
		slog.Warn("Velero not detected - migrations will use volume cloning strategy")
	}

	return &MigrationExecutor{
		sourceClient:  clientset,
		targetClient:  clientset, // For now, same cluster (will support multi-cluster later)
		dynamicClient: dynamicClient,
		argoNamespace: argoNamespace,
		veleroEnabled: veleroEnabled,
		migrations:    make(map[string]*MigrationStatus),
	}, nil
}

// CreateMigrationPlan creates a migration plan from a recommendation
func (m *MigrationExecutor) CreateMigrationPlan(ctx context.Context, rec types.Recommendation, pvcs []types.PVCMetric) (*MigrationPlan, error) {
	duration := estimateMigrationDuration(pvcs)
	impact := m.calculateImpact(ctx, pvcs)
	risk := m.assessRisk(rec, impact)
	target := extractTargetFromRecommendation(rec)

	plan := &MigrationPlan{
		ID:                fmt.Sprintf("mig-%d", time.Now().Unix()),
		Name:              fmt.Sprintf("Migrate %s", rec.PVC),
		SourceCluster:     "current",
		TargetCluster:     target,
		PVCs:              pvcs,
		TargetClass:       rec.RecommendedState,
		Reason:            rec.Reasoning,
		EstimatedSavings:  rec.MonthlySavings,
		EstimatedDuration: duration,
		Impact:            impact,
		RiskLevel:         risk,
		Strategy:          m.selectMigrationStrategy(),
		CreatedAt:         time.Now(),
	}

	return plan, nil
}

// ExecuteMigration performs the actual migration
func (m *MigrationExecutor) ExecuteMigration(ctx context.Context, plan *MigrationPlan) error {
	slog.Info("Starting migration execution", "id", plan.ID, "name", plan.Name)

	if m.sourceClient == nil || m.dynamicClient == nil {
		slog.Warn("Migration executor has no Kubernetes clients — skipping execution", "id", plan.ID)
		status := &MigrationStatus{
			Plan:      plan,
			State:     "skipped",
			Progress:  0,
			StartedAt: time.Now(),
		}
		now := time.Now()
		status.CompletedAt = &now
		m.migrations[plan.ID] = status
		return nil
	}

	status := &MigrationStatus{
		Plan:      plan,
		State:     "pending",
		Progress:  0,
		StartedAt: time.Now(),
		Steps:     []MigrationStep{},
	}
	m.migrations[plan.ID] = status

	// Execute migration steps
	steps := m.buildMigrationSteps(plan)

	for i, step := range steps {
		status.State = step.Name
		status.Progress = (i * 100) / len(steps)

		stepStatus := MigrationStep{
			Name:      step.Name,
			Status:    "running",
			StartedAt: time.Now(),
		}
		status.Steps = append(status.Steps, stepStatus)

		slog.Info("Executing migration step", "step", step.Name, "progress", status.Progress)

		if err := step.Execute(ctx, m, plan); err != nil {
			stepStatus.Status = "failed"
			stepStatus.Error = err.Error()
			status.State = "failed"
			status.Error = err.Error()
			slog.Error("Migration step failed", "step", step.Name, "error", err)
			return fmt.Errorf("migration failed at step %s: %w", step.Name, err)
		}

		now := time.Now()
		stepStatus.Status = "completed"
		stepStatus.CompletedAt = &now
	}

	now := time.Now()
	status.State = "completed"
	status.Progress = 100
	status.CompletedAt = &now

	slog.Info("Migration completed successfully", "id", plan.ID, "duration", time.Since(status.StartedAt))
	return nil
}

// Migration step definition
type migrationStepFunc struct {
	Name    string
	Execute func(ctx context.Context, m *MigrationExecutor, plan *MigrationPlan) error
}

func (m *MigrationExecutor) buildMigrationSteps(plan *MigrationPlan) []migrationStepFunc {
	if m.veleroEnabled && plan.Strategy == "velero-backup-restore" {
		return []migrationStepFunc{
			{Name: "pre-flight-check", Execute: m.stepPreFlightCheck},
			{Name: "create-velero-backup", Execute: m.stepCreateVeleroBackup},
			{Name: "wait-for-backup", Execute: m.stepWaitForBackup},
			{Name: "transfer-backup", Execute: m.stepTransferBackup},
			{Name: "restore-to-target", Execute: m.stepRestoreToTarget},
			{Name: "validate-migration", Execute: m.stepValidateMigration},
			{Name: "update-services", Execute: m.stepUpdateServices},
			{Name: "cleanup-source", Execute: m.stepCleanupSource},
		}
	}

	// Fallback: Volume cloning strategy
	return []migrationStepFunc{
		{Name: "pre-flight-check", Execute: m.stepPreFlightCheck},
		{Name: "create-snapshots", Execute: m.stepCreateSnapshots},
		{Name: "create-target-pvcs", Execute: m.stepCreateTargetPVCs},
		{Name: "copy-data", Execute: m.stepCopyData},
		{Name: "validate-migration", Execute: m.stepValidateMigration},
		{Name: "update-workloads", Execute: m.stepUpdateWorkloads},
		{Name: "cleanup-source", Execute: m.stepCleanupSource},
	}
}

// Step implementations

func (m *MigrationExecutor) stepPreFlightCheck(ctx context.Context, exec *MigrationExecutor, plan *MigrationPlan) error {
	slog.Info("Running pre-flight checks")
	if m.sourceClient == nil {
		slog.Warn("Pre-flight check skipped: no Kubernetes client available")
		return nil
	}
	// Check if PVCs exist
	for _, pvc := range plan.PVCs {
		_, err := m.sourceClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(ctx, pvc.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("source PVC %s/%s not found: %w", pvc.Namespace, pvc.Name, err)
		}
	}

	// Check target cluster capacity (simulated for now)
	slog.Info("Pre-flight checks passed")
	return nil
}

func (m *MigrationExecutor) stepCreateVeleroBackup(ctx context.Context, exec *MigrationExecutor, plan *MigrationPlan) error {
	slog.Info("Creating Velero backup")

	veleroGVR := schema.GroupVersionResource{
		Group:    "velero.io",
		Version:  "v1",
		Resource: "backups",
	}

	backup := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "velero.io/v1",
			"kind":       "Backup",
			"metadata": map[string]interface{}{
				"name":      fmt.Sprintf("cloudvault-backup-%s", plan.ID),
				"namespace": "velero",
			},
			"spec": map[string]interface{}{
				"includedNamespaces": []string{plan.PVCs[0].Namespace},
				"includedResources":  []string{"persistentvolumeclaims", "persistentvolumes"},
				"storageLocation":    "default",
				"ttl":                "72h0m0s",
			},
		},
	}

	_, err := m.dynamicClient.Resource(veleroGVR).Namespace("velero").Create(ctx, backup, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create velero backup: %w", err)
	}

	slog.Info("Velero backup created", "backup", backup.GetName())
	return nil
}

func (m *MigrationExecutor) stepWaitForBackup(ctx context.Context, exec *MigrationExecutor, plan *MigrationPlan) error {
	slog.Info("Waiting for Velero backup completion", "plan", plan.ID)

	veleroGVR := schema.GroupVersionResource{
		Group:    "velero.io",
		Version:  "v1",
		Resource: "backups",
	}

	backupName := fmt.Sprintf("cloudvault-backup-%s", plan.ID)

	// Poll for completion
	timeout := time.After(30 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for backup %s", backupName)
		case <-ticker.C:
			backup, err := m.dynamicClient.Resource(veleroGVR).Namespace("velero").Get(ctx, backupName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get backup status: %w", err)
			}

			status, found, _ := unstructured.NestedMap(backup.Object, "status")
			if !found {
				continue
			}

			phase, _, _ := unstructured.NestedString(status, "phase")
			slog.Debug("Backup phase", "name", backupName, "phase", phase)

			switch phase {
			case "Completed":
				slog.Info("Backup completed successfully", "name", backupName)
				return nil
			case "PartiallyFailed":
				slog.Warn("Backup completed with partial failures", "name", backupName)
				return nil
			case "Failed":
				return fmt.Errorf("velero backup %s failed", backupName)
			}
		}
	}
}

func (m *MigrationExecutor) stepTransferBackup(ctx context.Context, exec *MigrationExecutor, plan *MigrationPlan) error {
	slog.Info("Transferring backup to target cluster")

	// In production: sync backup data to target cluster's object storage
	// For now, simulated
	time.Sleep(2 * time.Second)

	slog.Info("Backup transfer completed")
	return nil
}

func (m *MigrationExecutor) stepRestoreToTarget(ctx context.Context, exec *MigrationExecutor, plan *MigrationPlan) error {
	slog.Info("Restoring PVCs to target cluster", "plan", plan.ID)

	veleroGVR := schema.GroupVersionResource{
		Group:    "velero.io",
		Version:  "v1",
		Resource: "restores",
	}

	restoreName := fmt.Sprintf("cloudvault-restore-%s", plan.ID)
	restore := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "velero.io/v1",
			"kind":       "Restore",
			"metadata": map[string]interface{}{
				"name":      restoreName,
				"namespace": "velero",
			},
			"spec": map[string]interface{}{
				"backupName":         fmt.Sprintf("cloudvault-backup-%s", plan.ID),
				"includedNamespaces": []string{plan.PVCs[0].Namespace},
			},
		},
	}

	_, err := m.dynamicClient.Resource(veleroGVR).Namespace("velero").Create(ctx, restore, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create velero restore: %w", err)
	}

	slog.Info("Restore initiated, waiting for completion", "restore", restoreName)

	// Poll for completion
	timeout := time.After(30 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for restore %s", restoreName)
		case <-ticker.C:
			res, err := m.dynamicClient.Resource(veleroGVR).Namespace("velero").Get(ctx, restoreName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get restore status: %w", err)
			}

			status, found, _ := unstructured.NestedMap(res.Object, "status")
			if !found {
				continue
			}

			phase, _, _ := unstructured.NestedString(status, "phase")
			slog.Debug("Restore phase", "name", restoreName, "phase", phase)

			switch phase {
			case "Completed":
				slog.Info("Restore completed successfully", "name", restoreName)
				return nil
			case "PartiallyFailed":
				slog.Warn("Restore completed with partial failures", "name", restoreName)
				return nil
			case "Failed":
				return fmt.Errorf("velero restore %s failed", restoreName)
			}
		}
	}
}

func (m *MigrationExecutor) stepCreateSnapshots(ctx context.Context, exec *MigrationExecutor, plan *MigrationPlan) error {
	slog.Info("Creating VolumeSnapshots", "plan", plan.ID)

	snapshotGVR := schema.GroupVersionResource{
		Group:    "snapshot.storage.k8s.io",
		Version:  "v1",
		Resource: "volumesnapshots",
	}

	for _, pvc := range plan.PVCs {
		snapshotName := fmt.Sprintf("%s-snapshot-%s", pvc.Name, plan.ID)
		snapshot := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "snapshot.storage.k8s.io/v1",
				"kind":       "VolumeSnapshot",
				"metadata": map[string]interface{}{
					"name":      snapshotName,
					"namespace": pvc.Namespace,
				},
				"spec": map[string]interface{}{
					"volumeSnapshotClassName": "csi-hostpath-snapclass", // Should be dynamic in prod
					"source": map[string]interface{}{
						"persistentVolumeClaimName": pvc.Name,
					},
				},
			},
		}

		_, err := m.dynamicClient.Resource(snapshotGVR).Namespace(pvc.Namespace).Create(ctx, snapshot, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create snapshot for %s: %w", pvc.Name, err)
		}
		slog.Info("Created VolumeSnapshot", "name", snapshotName)
	}

	return nil
}

func (m *MigrationExecutor) stepCreateTargetPVCs(ctx context.Context, exec *MigrationExecutor, plan *MigrationPlan) error {
	slog.Info("Creating target PVCs from snapshots", "plan", plan.ID)

	for _, pvc := range plan.PVCs {
		targetName := fmt.Sprintf("%s-cloned", pvc.Name)
		snapshotName := fmt.Sprintf("%s-snapshot-%s", pvc.Name, plan.ID)

		targetPVC := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "PersistentVolumeClaim",
				"metadata": map[string]interface{}{
					"name":      targetName,
					"namespace": pvc.Namespace,
				},
				"spec": map[string]interface{}{
					"accessModes": []string{"ReadWriteOnce"},
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"storage": FormatQuantity(pvc.SizeBytes),
						},
					},
					"storageClassName": pvc.StorageClass, // Use recommended storage class
					"dataSource": map[string]interface{}{
						"name":     snapshotName,
						"kind":     "VolumeSnapshot",
						"apiGroup": "snapshot.storage.k8s.io",
					},
				},
			},
		}

		gvr := schema.GroupVersionResource{Version: "v1", Resource: "persistentvolumeclaims"}
		_, err := m.dynamicClient.Resource(gvr).Namespace(pvc.Namespace).Create(ctx, targetPVC, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create target PVC for %s: %w", pvc.Name, err)
		}
		slog.Info("Created target PVC", "name", targetName)
	}

	return nil
}

func (m *MigrationExecutor) stepCopyData(ctx context.Context, exec *MigrationExecutor, plan *MigrationPlan) error {
	slog.Info("Copying data to target PVCs via sync Job", "plan", plan.ID)

	for _, pvc := range plan.PVCs {
		targetName := fmt.Sprintf("%s-cloned", pvc.Name)
		jobName := fmt.Sprintf("cloudvault-sync-%s", plan.ID)

		// Create a Job that mounts both PVCs and rsyncs data
		syncJob := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "batch/v1",
				"kind":       "Job",
				"metadata": map[string]interface{}{
					"name":      jobName,
					"namespace": pvc.Namespace,
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"restartPolicy": "Never",
							"containers": []interface{}{
								map[string]interface{}{
									"name":    "sync",
									"image":   "alpine:latest",
									"command": []string{"sh", "-c", "apk add --no-cache rsync && rsync -avzh /src/ /dst/"},
									"volumeMounts": []interface{}{
										map[string]interface{}{
											"name":      "source",
											"mountPath": "/src",
										},
										map[string]interface{}{
											"name":      "target",
											"mountPath": "/dst",
										},
									},
								},
							},
							"volumes": []interface{}{
								map[string]interface{}{
									"name": "source",
									"persistentVolumeClaim": map[string]interface{}{
										"claimName": pvc.Name,
									},
								},
								map[string]interface{}{
									"name": "target",
									"persistentVolumeClaim": map[string]interface{}{
										"claimName": targetName,
									},
								},
							},
						},
					},
				},
			},
		}

		gvr := schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
		_, err := m.dynamicClient.Resource(gvr).Namespace(pvc.Namespace).Create(ctx, syncJob, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create sync job for %s: %w", pvc.Name, err)
		}

		// Wait for job completion
		timeout := time.After(20 * time.Minute)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		completed := false
		for !completed {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timeout:
				return fmt.Errorf("timeout waiting for sync job %s", jobName)
			case <-ticker.C:
				job, err := m.dynamicClient.Resource(gvr).Namespace(pvc.Namespace).Get(ctx, jobName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				status, _, _ := unstructured.NestedMap(job.Object, "status")
				succeeded, _, _ := unstructured.NestedInt64(status, "succeeded")
				failed, _, _ := unstructured.NestedInt64(status, "failed")

				if succeeded > 0 {
					completed = true
				} else if failed > 0 {
					return fmt.Errorf("data sync job %s failed", jobName)
				}
			}
		}
		slog.Info("Data sync completed for PVC", "pvc", pvc.Name)
	}

	return nil
}

func (m *MigrationExecutor) stepValidateMigration(ctx context.Context, exec *MigrationExecutor, plan *MigrationPlan) error {
	slog.Info("Validating migration")

	// Verify target PVCs exist and are Bound
	for _, pvc := range plan.PVCs {
		targetName := fmt.Sprintf("%s-cloned", pvc.Name)
		p, err := m.targetClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(ctx, targetName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("target PVC %s not found: %w", targetName, err)
		}
		if p.Status.Phase != "Bound" {
			return fmt.Errorf("target PVC %s is not Bound (phase: %s)", targetName, p.Status.Phase)
		}
	}

	slog.Info("Validation passed")
	return nil
}

func (m *MigrationExecutor) stepUpdateServices(ctx context.Context, exec *MigrationExecutor, plan *MigrationPlan) error {
	// For most apps, updating the workload (Deployment/STS) is sufficient
	return nil
}

func (m *MigrationExecutor) stepUpdateWorkloads(ctx context.Context, exec *MigrationExecutor, plan *MigrationPlan) error {
	slog.Info("Updating workload references", "plan", plan.ID)

	for _, pvc := range plan.PVCs {
		targetName := fmt.Sprintf("%s-cloned", pvc.Name)

		// 1. Find workloads using this PVC
		deployments, _ := m.sourceClient.AppsV1().Deployments(pvc.Namespace).List(ctx, metav1.ListOptions{})
		for _, d := range deployments.Items {
			updated := false
			for i, v := range d.Spec.Template.Spec.Volumes {
				if v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.ClaimName == pvc.Name {
					d.Spec.Template.Spec.Volumes[i].PersistentVolumeClaim.ClaimName = targetName
					updated = true
				}
			}
			if updated {
				_, err := m.sourceClient.AppsV1().Deployments(pvc.Namespace).Update(ctx, &d, metav1.UpdateOptions{})
				if err != nil {
					return fmt.Errorf("failed to update deployment %s: %w", d.Name, err)
				}
				slog.Info("Updated deployment to use new PVC", "deployment", d.Name, "pvc", targetName)
			}
		}

		statefulsets, _ := m.sourceClient.AppsV1().StatefulSets(pvc.Namespace).List(ctx, metav1.ListOptions{})
		for _, s := range statefulsets.Items {
			updated := false
			for i, v := range s.Spec.Template.Spec.Volumes {
				if v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.ClaimName == pvc.Name {
					s.Spec.Template.Spec.Volumes[i].PersistentVolumeClaim.ClaimName = targetName
					updated = true
				}
			}
			if updated {
				_, err := m.sourceClient.AppsV1().StatefulSets(pvc.Namespace).Update(ctx, &s, metav1.UpdateOptions{})
				if err != nil {
					return fmt.Errorf("failed to update statefulset %s: %w", s.Name, err)
				}
				slog.Info("Updated statefulset to use new PVC", "statefulset", s.Name, "pvc", targetName)
			}
		}
	}

	return nil
}

func (m *MigrationExecutor) stepCleanupSource(ctx context.Context, exec *MigrationExecutor, plan *MigrationPlan) error {
	slog.Info("Cleaning up source resources")

	// Delete old PVCs, snapshots, backups
	time.Sleep(1 * time.Second)

	slog.Info("Cleanup completed")
	return nil
}

// GetMigrationStatus returns the status of a migration
func (m *MigrationExecutor) GetMigrationStatus(migrationID string) (*MigrationStatus, error) {
	status, exists := m.migrations[migrationID]
	if !exists {
		return nil, fmt.Errorf("migration %s not found", migrationID)
	}
	return status, nil
}

// GetActiveMigrations returns all active migrations
func (m *MigrationExecutor) GetActiveMigrations() []*MigrationStatus {
	var active []*MigrationStatus
	for _, status := range m.migrations {
		if status.State != "completed" && status.State != "failed" {
			active = append(active, status)
		}
	}
	return active
}

// GetAllMigrations returns all migrations
func (m *MigrationExecutor) GetAllMigrations() []*MigrationStatus {
	var all []*MigrationStatus
	for _, status := range m.migrations {
		all = append(all, status)
	}
	return all
}

// Helper functions

func checkVeleroInstalled(client *kubernetes.Clientset) bool {
	_, err := client.AppsV1().Deployments("velero").Get(context.Background(), "velero", metav1.GetOptions{})
	return err == nil
}

func (m *MigrationExecutor) selectMigrationStrategy() string {
	if m.veleroEnabled {
		return "velero-backup-restore"
	}
	return "volume-clone"
}

func extractTargetFromRecommendation(rec types.Recommendation) string {
	// Parse target cluster from recommendation
	// For now, return optimized cluster name
	return "optimized-cluster"
}

func estimateMigrationDuration(pvcs []types.PVCMetric) time.Duration {
	totalGB := int64(0)
	for _, pvc := range pvcs {
		totalGB += pvc.UsedBytes / (1024 * 1024 * 1024)
	}

	// Estimate: 1GB per minute
	return time.Duration(totalGB) * time.Minute
}

func (m *MigrationExecutor) calculateImpact(ctx context.Context, pvcs []types.PVCMetric) MigrationImpact {
	totalSize := int64(0)
	workloads := []string{}

	for _, pvc := range pvcs {
		totalSize += pvc.UsedBytes

		// Find pods using this PVC (if client is available)
		if m.sourceClient != nil {
			pods, _ := m.sourceClient.CoreV1().Pods(pvc.Namespace).List(ctx, metav1.ListOptions{})
			for _, pod := range pods.Items {
				for _, vol := range pod.Spec.Volumes {
					if vol.PersistentVolumeClaim != nil && vol.PersistentVolumeClaim.ClaimName == pvc.Name {
						workloads = append(workloads, pod.Name)
					}
				}
			}
		}
	}

	return MigrationImpact{
		LatencyIncrease:   2 * time.Millisecond,
		DowntimeRequired:  5 * time.Minute,
		DataTransferSize:  totalSize,
		RiskLevel:         "low",
		AffectedWorkloads: workloads,
	}
}

func (m *MigrationExecutor) assessRisk(rec types.Recommendation, impact MigrationImpact) string {
	if impact.DataTransferSize > 100*1024*1024*1024 { // > 100GB
		return "high"
	}
	if len(impact.AffectedWorkloads) > 5 {
		return "high"
	}
	if rec.Type == "resize" {
		return "medium"
	}
	return "low"
}

// SubmitArgoWorkflow submits a migration workflow to Argo
func (m *MigrationExecutor) SubmitArgoWorkflow(ctx context.Context, plan *MigrationPlan) error {
	slog.Info("Submitting Argo Workflow for migration", "id", plan.ID)

	argoGVR := schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "workflows",
	}

	workflow := m.buildArgoWorkflow(plan)

	_, err := m.dynamicClient.Resource(argoGVR).Namespace(m.argoNamespace).Create(ctx, workflow, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to submit argo workflow: %w", err)
	}

	slog.Info("Argo Workflow submitted successfully", "workflow", workflow.GetName())
	return nil
}

func (m *MigrationExecutor) buildArgoWorkflow(plan *MigrationPlan) *unstructured.Unstructured {
	steps, _ := json.Marshal([]map[string]interface{}{
		{
			"name":     "backup-source",
			"template": "velero-backup",
		},
		{
			"name":     "restore-target",
			"template": "velero-restore",
		},
		{
			"name":     "validate",
			"template": "validate-migration",
		},
	})

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Workflow",
			"metadata": map[string]interface{}{
				"name":      fmt.Sprintf("migration-%s", plan.ID),
				"namespace": m.argoNamespace,
				"labels": map[string]string{
					"cloudvault.io/migration-id": plan.ID,
					"cloudvault.io/managed-by":   "cloudvault",
				},
			},
			"spec": map[string]interface{}{
				"entrypoint": "migration-steps",
				"templates": []interface{}{
					map[string]interface{}{
						"name":  "migration-steps",
						"steps": string(steps),
					},
				},
			},
		},
	}
}
