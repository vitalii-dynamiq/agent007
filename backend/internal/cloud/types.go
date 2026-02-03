// Package cloud provides secure credential management for cloud providers
// (AWS, GCP, Azure, IBM Cloud, Oracle Cloud) in sandbox environments.
//
// Security Model:
//   - User credentials are stored encrypted with AES-256-GCM
//   - Sandboxes NEVER receive long-lived credentials
//   - Sandboxes use credential helpers to fetch short-lived tokens on-demand
//   - Tokens are scoped to specific sandbox sessions with short TTLs (5 min - 1 hour)
//
// Supported Credential Mechanisms:
//
//   - AWS: credential_process in ~/.aws/config calls backend for STS AssumeRole
//   - GCP: Application Default Credentials with external_account or access token
//   - Azure: Service principal credentials via environment variables
//   - IBM Cloud: API key → IAM access token exchange
//   - Oracle Cloud: Session token-based authentication (5-60 min TTL)
//
// Documentation:
//   - AWS credential_process: https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sourcing-external.html
//   - GCP Workload Identity: https://cloud.google.com/iam/docs/workload-identity-federation
//   - Azure Service Principal: https://learn.microsoft.com/en-us/cli/azure/authenticate-azure-cli-service-principal
//   - IBM Cloud IAM: https://cloud.ibm.com/apidocs/iam-identity-token-api
//   - Oracle OCI Session: https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/clitoken.htm
package cloud

import (
	"time"
)

// ProviderType represents the cloud provider
type ProviderType string

const (
	ProviderAWS      ProviderType = "aws"
	ProviderGCP      ProviderType = "gcp"
	ProviderAzure    ProviderType = "azure"
	ProviderIBM      ProviderType = "ibm"
	ProviderOracle   ProviderType = "oracle"
	ProviderPostgres ProviderType = "postgres"
)

// =============================================================================
// AWS Configuration
// Documentation: https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sourcing-external.html
// =============================================================================

// AWSCredentialConfig represents AWS credential configuration.
// User provides a Role ARN; we use STS AssumeRole to get temporary credentials.
type AWSCredentialConfig struct {
	// AccountID is the AWS account ID (optional but recommended for display)
	AccountID string `json:"accountId,omitempty"`

	// RoleARN is the ARN of the IAM role to assume
	// Example: arn:aws:iam::123456789012:role/AgentRole
	RoleARN string `json:"roleArn"`

	// ExternalID for cross-account role assumption (optional, for security)
	ExternalID string `json:"externalId,omitempty"`

	// Region is the default AWS region
	Region string `json:"region,omitempty"`

	// SessionDuration is how long the assumed role credentials last
	// Default: 1 hour, Max: 12 hours (depends on role configuration)
	SessionDuration time.Duration `json:"sessionDuration,omitempty"`

	// SourceCredentials - if set, use these to assume the role
	// Otherwise, use the backend's default credentials
	// Note: These are stored encrypted if provided
	AccessKeyID     string `json:"accessKeyId,omitempty"`
	SecretAccessKey string `json:"secretAccessKey,omitempty"`
}

// AWSCredentials represents temporary AWS credentials returned to sandbox.
// Format matches AWS credential_process output specification.
type AWSCredentials struct {
	Version         int       `json:"Version"`
	AccessKeyId     string    `json:"AccessKeyId"`
	SecretAccessKey string    `json:"SecretAccessKey"`
	SessionToken    string    `json:"SessionToken"`
	Expiration      time.Time `json:"Expiration"`
}

// =============================================================================
// GCP Configuration
// Documentation: https://cloud.google.com/iam/docs/workload-identity-federation
// =============================================================================

// GCPCredentialConfig represents GCP credential configuration.
type GCPCredentialConfig struct {
	// ServiceAccountJSON is the full service account key JSON
	// This is encrypted at rest and never exposed to sandbox
	ServiceAccountJSON string `json:"serviceAccountJson"`

	// ProjectID is the GCP project ID
	ProjectID string `json:"projectId,omitempty"`

	// ImpersonateServiceAccount - if set, use impersonation instead of direct key
	ImpersonateServiceAccount string `json:"impersonateServiceAccount,omitempty"`

	// Scopes for the access token
	Scopes []string `json:"scopes,omitempty"`
}

// GCPAccessToken represents a GCP access token returned to sandbox.
type GCPAccessToken struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresIn   int       `json:"expires_in"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// =============================================================================
// Azure Configuration
// Documentation: https://learn.microsoft.com/en-us/cli/azure/authenticate-azure-cli-service-principal
// =============================================================================

// AzureCredentialConfig represents Azure service principal configuration.
// We use service principal authentication with client secret or certificate.
type AzureCredentialConfig struct {
	// TenantID is the Azure AD tenant ID
	TenantID string `json:"tenantId"`

	// ClientID is the service principal application ID
	ClientID string `json:"clientId"`

	// ClientSecret is the service principal password (encrypted at rest)
	// Mutually exclusive with CertificatePath
	ClientSecret string `json:"clientSecret,omitempty"`

	// CertificatePath for certificate-based auth (more secure)
	// The certificate content is stored encrypted
	CertificatePEM string `json:"certificatePem,omitempty"`

	// SubscriptionID is the default Azure subscription
	SubscriptionID string `json:"subscriptionId,omitempty"`
}

// AzureAccessToken represents an Azure access token returned to sandbox.
type AzureAccessToken struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
	RefreshToken string    `json:"refresh_token,omitempty"`
}

// =============================================================================
// IBM Cloud Configuration
// Documentation: https://cloud.ibm.com/apidocs/iam-identity-token-api
// =============================================================================

// IBMCloudCredentialConfig represents IBM Cloud configuration.
// We exchange an API key for a short-lived IAM access token.
type IBMCloudCredentialConfig struct {
	// APIKey is the IBM Cloud API key (encrypted at rest)
	APIKey string `json:"apiKey"`

	// AccountID is the IBM Cloud account ID
	AccountID string `json:"accountId,omitempty"`

	// Region is the default IBM Cloud region
	Region string `json:"region,omitempty"`

	// ResourceGroup is the default resource group
	ResourceGroup string `json:"resourceGroup,omitempty"`
}

// IBMCloudAccessToken represents an IBM Cloud access token.
// Access tokens are JWT format with ~60 minute validity.
type IBMCloudAccessToken struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Scope        string    `json:"scope,omitempty"`
}

// =============================================================================
// Oracle Cloud (OCI) Configuration
// Documentation: https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/clitoken.htm
// =============================================================================

// OracleCloudCredentialConfig represents Oracle Cloud configuration.
// OCI supports session token-based authentication with 5-60 minute TTL.
type OracleCloudCredentialConfig struct {
	// TenancyOCID is the Oracle Cloud tenancy OCID
	TenancyOCID string `json:"tenancyOcid"`

	// UserOCID is the user OCID
	UserOCID string `json:"userOcid"`

	// Fingerprint of the API signing key
	Fingerprint string `json:"fingerprint"`

	// PrivateKeyPEM is the API signing private key (encrypted at rest)
	PrivateKeyPEM string `json:"privateKeyPem"`

	// Region is the Oracle Cloud region
	Region string `json:"region,omitempty"`

	// Compartment is the default compartment OCID
	CompartmentOCID string `json:"compartmentOcid,omitempty"`
}

// OracleCloudSessionToken represents an OCI session token.
// Session tokens have configurable TTL: 5-60 minutes.
type OracleCloudSessionToken struct {
	Token       string    `json:"token"`
	PrivateKey  string    `json:"private_key"`  // Ephemeral key pair
	Region      string    `json:"region"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// =============================================================================
// Kubernetes Configuration
// Documentation: https://kubernetes.io/docs/reference/access-authn-authz/authentication/
// =============================================================================

// KubernetesCredentialConfig represents Kubernetes cluster access configuration.
// We support multiple auth mechanisms depending on the cluster type.
type KubernetesCredentialConfig struct {
	// ClusterName is a friendly name for the cluster
	ClusterName string `json:"clusterName"`

	// APIServer is the Kubernetes API server URL
	// Example: https://kubernetes.example.com:6443
	APIServer string `json:"apiServer"`

	// CACertPEM is the cluster CA certificate (for TLS verification)
	CACertPEM string `json:"caCertPem,omitempty"`

	// AuthMethod specifies how to authenticate
	// Options: "token", "exec", "oidc", "aws-eks", "gcp-gke", "azure-aks"
	AuthMethod string `json:"authMethod"`

	// Token is a service account token (for "token" method)
	// Should be short-lived; prefer exec plugins for production
	Token string `json:"token,omitempty"`

	// ServiceAccountName for token generation
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Namespace for service account
	Namespace string `json:"namespace,omitempty"`

	// For cloud-managed clusters, store the cloud provider config
	// These reference the user's cloud credentials
	AWSClusterName string `json:"awsClusterName,omitempty"` // For EKS
	AWSRegion      string `json:"awsRegion,omitempty"`
	GCPProject     string `json:"gcpProject,omitempty"` // For GKE
	GCPCluster     string `json:"gcpCluster,omitempty"`
	GCPZone        string `json:"gcpZone,omitempty"`
	AzureCluster   string `json:"azureCluster,omitempty"` // For AKS
	AzureRG        string `json:"azureResourceGroup,omitempty"`
}

// KubernetesToken represents credentials for kubectl.
type KubernetesToken struct {
	// Kubeconfig is the full kubeconfig YAML to write to ~/.kube/config
	Kubeconfig string `json:"kubeconfig"`

	// Token is the bearer token (for token-based auth)
	Token string `json:"token,omitempty"`

	// ExpiresAt when the credentials expire
	ExpiresAt time.Time `json:"expires_at"`
}

// =============================================================================
// PostgreSQL Configuration
// =============================================================================

// PostgresCredentialConfig represents PostgreSQL database connection configuration.
// Credentials are injected as environment variables (PGHOST, PGPORT, etc.)
type PostgresCredentialConfig struct {
	// Host is the database server hostname or IP
	Host string `json:"host"`

	// Port is the database server port (default: 5432)
	Port int `json:"port,omitempty"`

	// Database is the database name
	Database string `json:"database"`

	// Username for database authentication
	Username string `json:"username"`

	// Password for database authentication (encrypted at rest)
	Password string `json:"password"`

	// SSLMode for connection security: disable, require, verify-ca, verify-full
	SSLMode string `json:"sslMode,omitempty"`

	// ConnectionName is a friendly name for this connection
	ConnectionName string `json:"connectionName,omitempty"`
}

// =============================================================================
// Unified Types
// =============================================================================

// UserCloudCredentials represents a user's cloud credentials
type UserCloudCredentials struct {
	UserID    string       `json:"userId"`
	Provider  ProviderType `json:"provider"`
	Name      string       `json:"name"` // User-friendly name
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`

	// One of these will be set based on Provider
	AWS      *AWSCredentialConfig         `json:"aws,omitempty"`
	GCP      *GCPCredentialConfig         `json:"gcp,omitempty"`
	Azure    *AzureCredentialConfig       `json:"azure,omitempty"`
	IBM      *IBMCloudCredentialConfig    `json:"ibm,omitempty"`
	Oracle   *OracleCloudCredentialConfig `json:"oracle,omitempty"`
	K8s      *KubernetesCredentialConfig  `json:"kubernetes,omitempty"`
	Postgres *PostgresCredentialConfig    `json:"postgres,omitempty"`
}

// CredentialRequest represents a request from a sandbox for credentials
type CredentialRequest struct {
	SandboxID    string       `json:"sandboxId"`
	Provider     ProviderType `json:"provider"`
	SessionToken string       `json:"sessionToken"` // JWT from our auth system
}

// CredentialResponse represents the response with credentials
type CredentialResponse struct {
	Provider ProviderType `json:"provider"`

	// Provider-specific responses (only one set)
	AWS    *AWSCredentials          `json:"aws,omitempty"`
	GCP    *GCPAccessToken          `json:"gcp,omitempty"`
	Azure  *AzureAccessToken        `json:"azure,omitempty"`
	IBM    *IBMCloudAccessToken     `json:"ibm,omitempty"`
	Oracle *OracleCloudSessionToken `json:"oracle,omitempty"`
	K8s    *KubernetesToken         `json:"kubernetes,omitempty"`

	// Error if credential fetch failed
	Error string `json:"error,omitempty"`
}
