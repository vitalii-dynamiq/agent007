package cloud

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"
)

// KubernetesProvider handles Kubernetes cluster credential operations.
//
// Authentication Methods Supported:
//
// 1. Service Account Token (Direct):
//    - User provides a service account token
//    - Token injected directly into kubeconfig
//    - Recommended: Use short-lived tokens (1 hour)
//
// 2. Cloud Provider Integration (EKS, GKE, AKS):
//    - Uses the user's existing cloud credentials
//    - Backend calls cloud provider to get cluster credentials
//    - EKS: Uses AWS STS + `aws eks get-token`
//    - GKE: Uses GCP service account
//    - AKS: Uses Azure service principal
//
// 3. Exec Credential Plugin (Advanced):
//    - kubeconfig configured with exec plugin
//    - Plugin calls our backend for fresh credentials
//    - Most secure for long-running sandboxes
//
// Security:
//   - Long-lived tokens never enter sandbox when using exec plugins
//   - Service account tokens should be short-lived
//   - Cloud-managed clusters use cloud credential helpers
//
// Documentation: https://kubernetes.io/docs/reference/access-authn-authz/authentication/
type KubernetesProvider struct {
	awsProvider    *AWSProvider
	gcpProvider    *GCPProvider
	azureProvider  *AzureProvider
}

// NewKubernetesProvider creates a new Kubernetes credential provider.
func NewKubernetesProvider(aws *AWSProvider, gcp *GCPProvider, azure *AzureProvider) *KubernetesProvider {
	return &KubernetesProvider{
		awsProvider:   aws,
		gcpProvider:   gcp,
		azureProvider: azure,
	}
}

// GetKubeconfig generates a kubeconfig for the sandbox.
//
// Based on the auth method, this will:
// - Token: Include the token directly
// - AWS EKS: Configure exec plugin for aws eks get-token
// - GCP GKE: Configure exec plugin for gcloud
// - Azure AKS: Configure exec plugin for kubelogin
// - Exec: Configure custom exec plugin calling our backend
func (p *KubernetesProvider) GetKubeconfig(ctx context.Context, config *KubernetesCredentialConfig, sandboxID string) (*KubernetesToken, error) {
	if config == nil {
		return nil, fmt.Errorf("kubernetes config is nil")
	}
	if config.APIServer == "" {
		return nil, fmt.Errorf("apiServer is required")
	}

	var kubeconfig string
	var token string
	expiresAt := time.Now().Add(1 * time.Hour) // Default 1 hour

	switch config.AuthMethod {
	case "token":
		if config.Token == "" {
			return nil, fmt.Errorf("token is required for token auth method")
		}
		kubeconfig = p.generateTokenKubeconfig(config)
		token = config.Token

	case "aws-eks":
		kubeconfig = p.generateEKSKubeconfig(config)

	case "gcp-gke":
		kubeconfig = p.generateGKEKubeconfig(config)

	case "azure-aks":
		kubeconfig = p.generateAKSKubeconfig(config)

	case "exec":
		kubeconfig = p.generateExecKubeconfig(config, sandboxID)

	default:
		return nil, fmt.Errorf("unsupported auth method: %s", config.AuthMethod)
	}

	return &KubernetesToken{
		Kubeconfig: kubeconfig,
		Token:      token,
		ExpiresAt:  expiresAt,
	}, nil
}

// generateTokenKubeconfig creates a kubeconfig with embedded token.
func (p *KubernetesProvider) generateTokenKubeconfig(config *KubernetesCredentialConfig) string {
	caCert := ""
	if config.CACertPEM != "" {
		caCert = fmt.Sprintf("    certificate-authority-data: %s",
			base64.StdEncoding.EncodeToString([]byte(config.CACertPEM)))
	} else {
		caCert = "    insecure-skip-tls-verify: true"
	}

	return fmt.Sprintf(`apiVersion: v1
kind: Config
current-context: default
clusters:
- name: default
  cluster:
    server: %s
%s
contexts:
- name: default
  context:
    cluster: default
    user: default
    namespace: %s
users:
- name: default
  user:
    token: %s
`, config.APIServer, caCert, getNamespace(config), config.Token)
}

// generateEKSKubeconfig creates a kubeconfig for Amazon EKS.
// Uses `aws eks get-token` via exec credential plugin.
// Documentation: https://docs.aws.amazon.com/eks/latest/userguide/create-kubeconfig.html
func (p *KubernetesProvider) generateEKSKubeconfig(config *KubernetesCredentialConfig) string {
	caCert := ""
	if config.CACertPEM != "" {
		caCert = fmt.Sprintf("    certificate-authority-data: %s",
			base64.StdEncoding.EncodeToString([]byte(config.CACertPEM)))
	}

	region := config.AWSRegion
	if region == "" {
		region = "us-east-1"
	}

	return fmt.Sprintf(`apiVersion: v1
kind: Config
current-context: eks
clusters:
- name: eks
  cluster:
    server: %s
%s
contexts:
- name: eks
  context:
    cluster: eks
    user: eks
users:
- name: eks
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args:
        - eks
        - get-token
        - --cluster-name
        - %s
        - --region
        - %s
      env:
        - name: AWS_PROFILE
          value: default
`, config.APIServer, caCert, config.AWSClusterName, region)
}

// generateGKEKubeconfig creates a kubeconfig for Google Kubernetes Engine.
// Uses `gcloud` via exec credential plugin.
// Documentation: https://cloud.google.com/sdk/gcloud/reference/container/clusters/get-credentials
func (p *KubernetesProvider) generateGKEKubeconfig(config *KubernetesCredentialConfig) string {
	caCert := ""
	if config.CACertPEM != "" {
		caCert = fmt.Sprintf("    certificate-authority-data: %s",
			base64.StdEncoding.EncodeToString([]byte(config.CACertPEM)))
	}

	return fmt.Sprintf(`apiVersion: v1
kind: Config
current-context: gke
clusters:
- name: gke
  cluster:
    server: %s
%s
contexts:
- name: gke
  context:
    cluster: gke
    user: gke
users:
- name: gke
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: gke-gcloud-auth-plugin
      installHint: Install gke-gcloud-auth-plugin for use with kubectl by following https://cloud.google.com/blog/products/containers-kubernetes/kubectl-auth-changes-in-gke
      provideClusterInfo: true
`, config.APIServer, caCert)
}

// generateAKSKubeconfig creates a kubeconfig for Azure Kubernetes Service.
// Uses `kubelogin` via exec credential plugin.
// Documentation: https://azure.github.io/kubelogin/
func (p *KubernetesProvider) generateAKSKubeconfig(config *KubernetesCredentialConfig) string {
	caCert := ""
	if config.CACertPEM != "" {
		caCert = fmt.Sprintf("    certificate-authority-data: %s",
			base64.StdEncoding.EncodeToString([]byte(config.CACertPEM)))
	}

	return fmt.Sprintf(`apiVersion: v1
kind: Config
current-context: aks
clusters:
- name: aks
  cluster:
    server: %s
%s
contexts:
- name: aks
  context:
    cluster: aks
    user: aks
users:
- name: aks
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: kubelogin
      args:
        - get-token
        - --environment
        - AzurePublicCloud
        - --server-id
        - 6dae42f8-4368-4678-94ff-3960e28e3630
        - --client-id
        - %s
        - --tenant-id
        - %s
        - --login
        - spn
      env:
        - name: AAD_SERVICE_PRINCIPAL_CLIENT_SECRET
          value: __AZURE_CLIENT_SECRET__
`, config.APIServer, caCert, "$AZURE_CLIENT_ID", "$AZURE_TENANT_ID")
}

// generateExecKubeconfig creates a kubeconfig with a custom exec plugin
// that calls our backend for credentials.
func (p *KubernetesProvider) generateExecKubeconfig(config *KubernetesCredentialConfig, sandboxID string) string {
	caCert := ""
	if config.CACertPEM != "" {
		caCert = fmt.Sprintf("    certificate-authority-data: %s",
			base64.StdEncoding.EncodeToString([]byte(config.CACertPEM)))
	}

	return fmt.Sprintf(`apiVersion: v1
kind: Config
current-context: default
clusters:
- name: default
  cluster:
    server: %s
%s
contexts:
- name: default
  context:
    cluster: default
    user: default
    namespace: %s
users:
- name: default
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: /usr/local/bin/k8s-credential-helper
      args:
        - --sandbox-id
        - %s
      env:
        - name: BACKEND_URL
          value: __BACKEND_URL__
        - name: SESSION_TOKEN
          value: __SESSION_TOKEN__
`, config.APIServer, caCert, getNamespace(config), sandboxID)
}

// GenerateK8sCredentialHelper generates a bash script that acts as a
// Kubernetes exec credential plugin, fetching tokens from our backend.
//
// This follows the Kubernetes client.authentication.k8s.io/v1beta1 format.
// Documentation: https://kubernetes.io/docs/reference/access-authn-authz/authentication/#client-go-credential-plugins
func GenerateK8sCredentialHelper(backendURL, sessionToken, sandboxID string) string {
	return fmt.Sprintf(`#!/bin/bash
# Kubernetes Credential Helper - Generated by Dynamiq
# This is an exec credential plugin that fetches tokens from the backend
# Documentation: https://kubernetes.io/docs/reference/access-authn-authz/authentication/#client-go-credential-plugins

set -e

# Parse arguments
SANDBOX_ID="%s"
while [[ $# -gt 0 ]]; do
  case $1 in
    --sandbox-id) SANDBOX_ID="$2"; shift 2;;
    *) shift;;
  esac
done

# Fetch Kubernetes credentials from backend
response=$(curl -s -X POST "%s/api/cloud/kubernetes/credentials" \
  -H "Authorization: Bearer %s" \
  -H "Content-Type: application/json" \
  -d "{\"sandboxId\": \"$SANDBOX_ID\", \"provider\": \"kubernetes\"}")

# Check for errors
error=$(echo "$response" | jq -r '.error // empty')
if [ -n "$error" ]; then
  echo "Error: $error" >&2
  exit 1
fi

# Extract token
token=$(echo "$response" | jq -r '.kubernetes.token // empty')
expires_at=$(echo "$response" | jq -r '.kubernetes.expires_at')

if [ -z "$token" ] || [ "$token" = "null" ]; then
  echo "Error: Failed to get token" >&2
  exit 1
fi

# Convert expires_at to RFC3339 format if needed
# Output in ExecCredential format
cat << EOF
{
  "apiVersion": "client.authentication.k8s.io/v1beta1",
  "kind": "ExecCredential",
  "status": {
    "token": "$token",
    "expirationTimestamp": "$expires_at"
  }
}
EOF
`, sandboxID, backendURL, sessionToken)
}

// GenerateKubectlSetup generates a script to set up kubectl in the sandbox.
func GenerateKubectlSetup(backendURL, sessionToken, sandboxID string, config *KubernetesCredentialConfig) string {
	return fmt.Sprintf(`#!/bin/bash
# Kubernetes Setup - Generated by Dynamiq
# Sets up kubectl with credentials from the backend

set -e

# Fetch kubeconfig from backend
response=$(curl -s -X POST "%s/api/cloud/kubernetes/credentials" \
  -H "Authorization: Bearer %s" \
  -H "Content-Type: application/json" \
  -d '{"sandboxId": "%s", "provider": "kubernetes"}')

# Check for errors
error=$(echo "$response" | jq -r '.error // empty')
if [ -n "$error" ]; then
  echo "Error: $error" >&2
  exit 1
fi

# Extract kubeconfig
kubeconfig=$(echo "$response" | jq -r '.kubernetes.kubeconfig')

if [ -z "$kubeconfig" ] || [ "$kubeconfig" = "null" ]; then
  echo "Error: Failed to get kubeconfig" >&2
  exit 1
fi

# Write kubeconfig
mkdir -p "$HOME/.kube"
echo "$kubeconfig" > "$HOME/.kube/config"
chmod 600 "$HOME/.kube/config"

# Install credential helper if using exec auth
if echo "$kubeconfig" | grep -q "k8s-credential-helper"; then
  cat > /usr/local/bin/k8s-credential-helper << 'HELPER'
%s
HELPER
  chmod +x /usr/local/bin/k8s-credential-helper
  # Replace placeholders
  sed -i "s|__BACKEND_URL__|%s|g" "$HOME/.kube/config"
  sed -i "s|__SESSION_TOKEN__|%s|g" "$HOME/.kube/config"
fi

echo "kubectl configured successfully"
kubectl cluster-info 2>/dev/null || echo "Note: Run 'kubectl cluster-info' to verify connection"
`, backendURL, sessionToken, sandboxID,
		GenerateK8sCredentialHelper(backendURL, sessionToken, sandboxID),
		backendURL, sessionToken)
}

// getNamespace returns the namespace or "default"
func getNamespace(config *KubernetesCredentialConfig) string {
	if config.Namespace != "" {
		return config.Namespace
	}
	return "default"
}

// ValidateCredentials tests if the Kubernetes configuration is valid.
func (p *KubernetesProvider) ValidateCredentials(ctx context.Context, config *KubernetesCredentialConfig) error {
	if config.APIServer == "" {
		return fmt.Errorf("apiServer is required")
	}
	if config.AuthMethod == "" {
		return fmt.Errorf("authMethod is required")
	}

	switch config.AuthMethod {
	case "token":
		if config.Token == "" {
			return fmt.Errorf("token is required for token auth")
		}
	case "aws-eks":
		if config.AWSClusterName == "" {
			return fmt.Errorf("awsClusterName is required for EKS")
		}
	case "gcp-gke":
		if config.GCPCluster == "" || config.GCPProject == "" {
			return fmt.Errorf("gcpCluster and gcpProject are required for GKE")
		}
	case "azure-aks":
		if config.AzureCluster == "" || config.AzureRG == "" {
			return fmt.Errorf("azureCluster and azureResourceGroup are required for AKS")
		}
	}

	return nil
}
