package types

import (
	"time"
)

// Config represents the CloudVault application configuration
type Config struct {
	KubeConfig    string        `yaml:"kubeconfig" json:"kubeconfig"`
	Interval      time.Duration `yaml:"interval" json:"interval"`
	Namespace     string        `yaml:"namespace" json:"namespace"`
	PrometheusURL string        `yaml:"prometheus_url" json:"prometheus_url"`
	DashboardPort int           `yaml:"dashboard_port" json:"dashboard_port"`
	Provider      string        `yaml:"provider" json:"provider"`

	// Storage Intelligence Graph (Neo4j)
	Neo4jURI      string `yaml:"neo4j_uri" json:"neo4j_uri"`
	Neo4jUser     string `yaml:"neo4j_user" json:"neo4j_user"`
	Neo4jPassword string `yaml:"neo4j_password" json:"neo4j_password"`

	// TimescaleDB for historical metrics
	TimescaleConn string `yaml:"timescale_conn" json:"timescale_conn"`

	// Mock mode for testing
	Mock bool `yaml:"mock" json:"mock"`

	// Cloud provider credentials
	AWSRegion    string `yaml:"aws_region" json:"aws_region"`
	AWSAccessKey string `yaml:"aws_access_key" json:"aws_access_key"`
	AWSSecretKey string `yaml:"aws_secret_key" json:"aws_secret_key"`

	GCPProject   string `yaml:"gcp_project" json:"gcp_project"`
	GCPCredsPath string `yaml:"gcp_creds_path" json:"gcp_creds_path"`

	AzureSubscriptionID string `yaml:"azure_subscription_id" json:"azure_subscription_id"`
	AzureTenantID       string `yaml:"azure_tenant_id" json:"azure_tenant_id"`
	AzureClientID       string `yaml:"azure_client_id" json:"azure_client_id"`
	AzureClientSecret   string `yaml:"azure_client_secret" json:"azure_client_secret"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Interval:      5 * time.Minute,
		DashboardPort: 8080,
		Provider:      "aws",
		Neo4jUser:     "neo4j",
	}
}
