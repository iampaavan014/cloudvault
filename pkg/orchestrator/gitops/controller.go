package gitops

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"gopkg.in/yaml.v3"
)

// GitOpsController manages GitOps-driven storage changes
type GitOpsController struct {
	repoURL      string
	branch       string
	username     string
	token        string
	commitAuthor string
	commitEmail  string
	workDir      string
	repo         *git.Repository
	prProvider   PRProvider // GitHub/GitLab API
}

// PRProvider interface for creating pull requests
type PRProvider interface {
	CreatePullRequest(ctx context.Context, branch, title, body string) (string, error)
}

// StorageChange represents a change to storage configuration
type StorageChange struct {
	Type             string // "pvc", "storageclass", "policy"
	Action           string // "create", "update", "delete"
	Namespace        string
	Name             string
	CurrentSpec      interface{}
	ProposedSpec     interface{}
	Reason           string
	EstimatedSavings float64
	Impact           string
}

// GitOpsConfig holds GitOps configuration
type GitOpsConfig struct {
	Enabled      bool
	RepoURL      string
	Branch       string
	Username     string
	Token        string
	CommitAuthor string
	CommitEmail  string
	WorkDir      string
}

// NewGitOpsController creates a new GitOps controller
func NewGitOpsController(cfg GitOpsConfig) (*GitOpsController, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("GitOps is disabled")
	}

	// Create work directory
	if err := os.MkdirAll(cfg.WorkDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create work dir: %w", err)
	}

	controller := &GitOpsController{
		repoURL:      cfg.RepoURL,
		branch:       cfg.Branch,
		username:     cfg.Username,
		token:        cfg.Token,
		commitAuthor: cfg.CommitAuthor,
		commitEmail:  cfg.CommitEmail,
		workDir:      cfg.WorkDir,
	}

	// Clone repository
	if err := controller.cloneRepo(); err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	slog.Info("GitOps controller initialized", "repo", cfg.RepoURL, "branch", cfg.Branch)
	return controller, nil
}

// SyncStorageChanges creates a Git PR for storage optimizations
func (g *GitOpsController) SyncStorageChanges(ctx context.Context, changes []StorageChange) (string, error) {
	slog.Info("Syncing storage changes to Git", "changes", len(changes))

	// Pull latest changes
	if err := g.pullLatest(); err != nil {
		return "", fmt.Errorf("failed to pull latest: %w", err)
	}

	// Create new branch
	branchName := fmt.Sprintf("cloudvault-optimization-%d", time.Now().Unix())
	if err := g.createBranch(branchName); err != nil {
		return "", fmt.Errorf("failed to create branch: %w", err)
	}

	// Apply changes to YAML files
	for _, change := range changes {
		if err := g.applyChange(change); err != nil {
			slog.Error("Failed to apply change", "change", change.Name, "error", err)
			return "", fmt.Errorf("failed to apply change %s: %w", change.Name, err)
		}
	}

	// Commit changes
	commitMsg := g.buildCommitMessage(changes)
	if err := g.commit(commitMsg); err != nil {
		return "", fmt.Errorf("failed to commit: %w", err)
	}

	// Push branch
	if err := g.pushBranch(branchName); err != nil {
		return "", fmt.Errorf("failed to push branch: %w", err)
	}

	// Create pull request
	prURL, err := g.createPullRequest(ctx, branchName, changes)
	if err != nil {
		return "", fmt.Errorf("failed to create PR: %w", err)
	}

	slog.Info("GitOps sync completed", "branch", branchName, "pr", prURL)
	return prURL, nil
}

// ApplyRecommendationAsGitOps converts a recommendation to GitOps change
func (g *GitOpsController) ApplyRecommendationAsGitOps(ctx context.Context, rec types.Recommendation) (string, error) {
	change := g.recommendationToChange(rec)
	return g.SyncStorageChanges(ctx, []StorageChange{change})
}

func (g *GitOpsController) recommendationToChange(rec types.Recommendation) StorageChange {
	change := StorageChange{
		Type:             "pvc",
		Action:           "update",
		Namespace:        rec.Namespace,
		Name:             rec.PVC,
		Reason:           rec.Reasoning,
		EstimatedSavings: rec.MonthlySavings,
		Impact:           rec.Impact,
	}

	// Parse recommended action
	switch rec.Type {
	case "delete_zombie":
		change.Action = "delete"
		change.ProposedSpec = nil
	case "resize":
		change.Action = "update"
		change.ProposedSpec = map[string]interface{}{
			"spec": map[string]interface{}{
				"resources": map[string]interface{}{
					"requests": map[string]interface{}{
						"storage": rec.RecommendedState,
					},
				},
			},
		}
	case "change_storage_class":
		change.Action = "update"
		change.ProposedSpec = map[string]interface{}{
			"spec": map[string]interface{}{
				"storageClassName": rec.RecommendedState,
			},
		}
	}

	return change
}

// cloneRepo clones the Git repository
func (g *GitOpsController) cloneRepo() error {
	repoPath := filepath.Join(g.workDir, "repo")

	// Remove existing repo
	_ = os.RemoveAll(repoPath)

	slog.Info("Cloning repository", "url", g.repoURL)

	repo, err := git.PlainClone(repoPath, false, &git.CloneOptions{
		URL:           g.repoURL,
		ReferenceName: plumbing.NewBranchReferenceName(g.branch),
		SingleBranch:  true,
		Auth: &http.BasicAuth{
			Username: g.username,
			Password: g.token,
		},
	})
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	g.repo = repo
	return nil
}

// pullLatest pulls latest changes from remote
func (g *GitOpsController) pullLatest() error {
	worktree, err := g.repo.Worktree()
	if err != nil {
		return err
	}

	err = worktree.Pull(&git.PullOptions{
		RemoteName: "origin",
		Auth: &http.BasicAuth{
			Username: g.username,
			Password: g.token,
		},
	})

	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("git pull failed: %w", err)
	}

	return nil
}

// createBranch creates a new Git branch
func (g *GitOpsController) createBranch(branchName string) error {
	worktree, err := g.repo.Worktree()
	if err != nil {
		return err
	}

	// Get HEAD reference
	head, err := g.repo.Head()
	if err != nil {
		return err
	}

	// Create new branch
	branchRef := plumbing.NewBranchReferenceName(branchName)
	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: branchRef,
		Create: true,
		Hash:   head.Hash(),
	})
	if err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	slog.Info("Created branch", "branch", branchName)
	return nil
}

// applyChange applies a storage change to YAML files
func (g *GitOpsController) applyChange(change StorageChange) error {
	repoPath := filepath.Join(g.workDir, "repo")

	// Find the manifest file for this resource
	manifestPath := g.findManifestPath(repoPath, change)
	if manifestPath == "" {
		// Create new manifest if not found
		var err error
		manifestPath, err = g.createManifestPath(repoPath, change)
		if err != nil {
			return fmt.Errorf("failed to create manifest path: %w", err)
		}
	}

	slog.Info("Applying change to manifest", "path", manifestPath, "action", change.Action)

	switch change.Action {
	case "delete":
		return os.Remove(manifestPath)
	case "create", "update":
		return g.updateManifest(manifestPath, change)
	default:
		return fmt.Errorf("unknown action: %s", change.Action)
	}
}

// findManifestPath finds the YAML file for a resource
func (g *GitOpsController) findManifestPath(repoPath string, change StorageChange) string {
	// Search for manifest in common locations
	searchPaths := []string{
		filepath.Join(repoPath, "manifests", change.Namespace, change.Type+"s", change.Name+".yaml"),
		filepath.Join(repoPath, "kubernetes", change.Namespace, change.Name+".yaml"),
		filepath.Join(repoPath, change.Namespace, change.Name+".yaml"),
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// createManifestPath creates a path for a new manifest
func (g *GitOpsController) createManifestPath(repoPath string, change StorageChange) (string, error) {
	dir := filepath.Join(repoPath, "cloudvault-optimizations", change.Namespace)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, change.Name+".yaml"), nil
}

// updateManifest updates a YAML manifest file
func (g *GitOpsController) updateManifest(path string, change StorageChange) error {
	var manifest map[string]interface{}

	// Read existing manifest if it exists
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			return err
		}
	} else {
		// Create new manifest
		manifest = g.createBaseManifest(change)
	}

	// Apply proposed changes
	if change.ProposedSpec != nil {
		if spec, ok := change.ProposedSpec.(map[string]interface{}); ok {
			g.mergeSpec(manifest, spec)
		}
	}

	// Add CloudVault annotations
	if manifest["metadata"] == nil {
		manifest["metadata"] = make(map[string]interface{})
	}
	metadata := manifest["metadata"].(map[string]interface{})
	if metadata["annotations"] == nil {
		metadata["annotations"] = make(map[string]interface{})
	}
	annotations := metadata["annotations"].(map[string]interface{})
	annotations["cloudvault.io/optimized"] = "true"
	annotations["cloudvault.io/reason"] = change.Reason
	annotations["cloudvault.io/estimated-savings"] = fmt.Sprintf("$%.2f/month", change.EstimatedSavings)

	// Write back to file
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// createBaseManifest creates a base manifest for a resource
func (g *GitOpsController) createBaseManifest(change StorageChange) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "PersistentVolumeClaim",
		"metadata": map[string]interface{}{
			"name":      change.Name,
			"namespace": change.Namespace,
		},
		"spec": map[string]interface{}{},
	}
}

// mergeSpec merges proposed spec into manifest
func (g *GitOpsController) mergeSpec(manifest, spec map[string]interface{}) {
	for key, value := range spec {
		if existingValue, exists := manifest[key]; exists {
			if existingMap, ok := existingValue.(map[string]interface{}); ok {
				if newMap, ok := value.(map[string]interface{}); ok {
					g.mergeSpec(existingMap, newMap)
					continue
				}
			}
		}
		manifest[key] = value
	}
}

// commit commits changes to Git
func (g *GitOpsController) commit(message string) error {
	worktree, err := g.repo.Worktree()
	if err != nil {
		return err
	}

	// Add all changes
	_, err = worktree.Add(".")
	if err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	// Commit
	_, err = worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  g.commitAuthor,
			Email: g.commitEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	slog.Info("Committed changes", "message", message)
	return nil
}

// pushBranch pushes a branch to remote
func (g *GitOpsController) pushBranch(branchName string) error {
	err := g.repo.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", branchName, branchName)),
		},
		Auth: &http.BasicAuth{
			Username: g.username,
			Password: g.token,
		},
	})
	if err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}

	slog.Info("Pushed branch", "branch", branchName)
	return nil
}

// createPullRequest creates a pull request for the changes
func (g *GitOpsController) createPullRequest(ctx context.Context, branchName string, changes []StorageChange) (string, error) {
	if g.prProvider == nil {
		// Fallback: return instructions to create PR manually
		return g.buildPRInstructions(branchName, changes), nil
	}

	title := g.buildPRTitle(changes)
	body := g.buildPRBody(changes)

	prURL, err := g.prProvider.CreatePullRequest(ctx, branchName, title, body)
	if err != nil {
		return "", fmt.Errorf("failed to create PR: %w", err)
	}

	return prURL, nil
}

// buildCommitMessage builds a commit message for changes
func (g *GitOpsController) buildCommitMessage(changes []StorageChange) string {
	if len(changes) == 1 {
		change := changes[0]
		return fmt.Sprintf("CloudVault: %s %s/%s\n\nReason: %s\nEstimated savings: $%.2f/month",
			change.Action, change.Namespace, change.Name, change.Reason, change.EstimatedSavings)
	}

	totalSavings := 0.0
	for _, change := range changes {
		totalSavings += change.EstimatedSavings
	}

	return fmt.Sprintf("CloudVault: Optimize storage configuration (%d changes)\n\nTotal estimated savings: $%.2f/month",
		len(changes), totalSavings)
}

// buildPRTitle builds a PR title
func (g *GitOpsController) buildPRTitle(changes []StorageChange) string {
	if len(changes) == 1 {
		change := changes[0]
		return fmt.Sprintf("🤖 CloudVault: Optimize %s/%s", change.Namespace, change.Name)
	}
	return fmt.Sprintf("🤖 CloudVault: Storage optimization (%d changes)", len(changes))
}

// buildPRBody builds a PR body with details
func (g *GitOpsController) buildPRBody(changes []StorageChange) string {
	body := "## CloudVault Storage Optimization\n\n"
	body += "This PR contains automated storage optimizations identified by CloudVault.\n\n"
	body += "### Changes\n\n"

	totalSavings := 0.0
	for i, change := range changes {
		body += fmt.Sprintf("%d. **%s** `%s/%s`\n", i+1, change.Action, change.Namespace, change.Name)
		body += fmt.Sprintf("   - Reason: %s\n", change.Reason)
		body += fmt.Sprintf("   - Estimated savings: $%.2f/month\n", change.EstimatedSavings)
		body += fmt.Sprintf("   - Impact: %s\n", change.Impact)
		body += "\n"
		totalSavings += change.EstimatedSavings
	}

	body += fmt.Sprintf("### Total Estimated Savings: $%.2f/month\n\n", totalSavings)
	body += "### Next Steps\n\n"
	body += "1. Review the proposed changes\n"
	body += "2. Test in a non-production environment if needed\n"
	body += "3. Approve and merge to apply optimizations\n\n"
	body += "---\n"
	body += "*Generated by [CloudVault](https://github.com/cloudvault-io/cloudvault)*\n"

	return body
}

// buildPRInstructions builds instructions for manual PR creation
func (g *GitOpsController) buildPRInstructions(branchName string, changes []StorageChange) string {
	return fmt.Sprintf(`
Branch '%s' has been pushed to the repository.

To create a pull request:
1. Visit: %s/compare/%s...%s
2. Review the changes
3. Create the pull request

Changes summary:
%s
`, branchName, g.repoURL, g.branch, branchName, g.buildPRBody(changes))
}

// DetectDrift detects drift between Git and cluster state
func (g *GitOpsController) DetectDrift(ctx context.Context, clusterState []types.PVCMetric) ([]DriftItem, error) {
	slog.Info("Detecting configuration drift")

	// Pull latest from Git
	if err := g.pullLatest(); err != nil {
		return nil, err
	}

	// Load manifests from Git
	gitManifests, err := g.loadManifestsFromGit()
	if err != nil {
		return nil, fmt.Errorf("failed to load manifests from git: %w", err)
	}

	// Compare with cluster state
	drift := []DriftItem{}
	for _, pvc := range clusterState {
		gitManifest, exists := gitManifests[fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name)]
		if !exists {
			drift = append(drift, DriftItem{
				ResourceType: "PVC",
				Namespace:    pvc.Namespace,
				Name:         pvc.Name,
				DriftType:    "missing_in_git",
				Description:  "Resource exists in cluster but not in Git",
			})
			continue
		}

		// Check for spec differences
		if driftFound := g.comparePVCSpec(pvc, gitManifest); driftFound != "" {
			drift = append(drift, DriftItem{
				ResourceType: "PVC",
				Namespace:    pvc.Namespace,
				Name:         pvc.Name,
				DriftType:    "spec_mismatch",
				Description:  driftFound,
			})
		}
	}

	slog.Info("Drift detection completed", "driftItems", len(drift))
	return drift, nil
}

// DriftItem represents a configuration drift
type DriftItem struct {
	ResourceType string
	Namespace    string
	Name         string
	DriftType    string
	Description  string
}

func (g *GitOpsController) loadManifestsFromGit() (map[string]interface{}, error) {
	manifests := make(map[string]interface{})
	repoPath := filepath.Join(g.workDir, "repo")

	// Walk through repo and load YAML files
	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		if filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml" {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			var manifest map[string]interface{}
			if err := yaml.Unmarshal(data, &manifest); err != nil {
				return nil
			}

			if manifest["kind"] == "PersistentVolumeClaim" {
				metadata, ok := manifest["metadata"].(map[string]interface{})
				if !ok {
					return nil
				}
				namespace, _ := metadata["namespace"].(string)
				name, _ := metadata["name"].(string)
				manifests[fmt.Sprintf("%s/%s", namespace, name)] = manifest
			}
		}

		return nil
	})

	return manifests, err
}

func (g *GitOpsController) comparePVCSpec(pvc types.PVCMetric, gitManifest interface{}) string {
	// Compare storage size, storage class, etc.
	// Return description of drift if found
	return ""
}
