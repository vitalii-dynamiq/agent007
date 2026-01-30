package cloud

import (
	"testing"
)

func TestCredentialStoreEncryption(t *testing.T) {
	store, err := NewCredentialStore("test-encryption-key-32-bytes!!")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Test AWS credential storage
	awsConfig := &AWSCredentialConfig{
		RoleARN:         "arn:aws:iam::123456789012:role/TestRole",
		ExternalID:      "test-external-id",
		Region:          "us-west-2",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	err = store.StoreAWSCredentials("user1", "My AWS Account", awsConfig)
	if err != nil {
		t.Fatalf("Failed to store AWS credentials: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.GetAWSCredentials("user1")
	if err != nil {
		t.Fatalf("Failed to get AWS credentials: %v", err)
	}

	if retrieved.RoleARN != awsConfig.RoleARN {
		t.Errorf("RoleARN mismatch: got %s, want %s", retrieved.RoleARN, awsConfig.RoleARN)
	}

	if retrieved.SecretAccessKey != awsConfig.SecretAccessKey {
		t.Errorf("SecretAccessKey mismatch: got %s, want %s", retrieved.SecretAccessKey, awsConfig.SecretAccessKey)
	}
}

func TestCredentialStoreGCP(t *testing.T) {
	store, err := NewCredentialStore("test-encryption-key-32-bytes!!")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	gcpConfig := &GCPCredentialConfig{
		ServiceAccountJSON: `{
			"type": "service_account",
			"project_id": "test-project",
			"private_key_id": "key-id",
			"private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBg...\n-----END PRIVATE KEY-----\n",
			"client_email": "test@test-project.iam.gserviceaccount.com",
			"client_id": "123456789"
		}`,
		ProjectID: "test-project",
	}

	err = store.StoreGCPCredentials("user1", "My GCP Account", gcpConfig)
	if err != nil {
		t.Fatalf("Failed to store GCP credentials: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.GetGCPCredentials("user1")
	if err != nil {
		t.Fatalf("Failed to get GCP credentials: %v", err)
	}

	if retrieved.ServiceAccountJSON != gcpConfig.ServiceAccountJSON {
		t.Errorf("ServiceAccountJSON mismatch")
	}
}

func TestCredentialStoreListCredentials(t *testing.T) {
	store, err := NewCredentialStore("test-encryption-key-32-bytes!!")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Store both AWS and GCP credentials
	err = store.StoreAWSCredentials("user1", "AWS Prod", &AWSCredentialConfig{
		RoleARN:         "arn:aws:iam::123456789012:role/TestRole",
		SecretAccessKey: "secret",
	})
	if err != nil {
		t.Fatalf("Failed to store AWS credentials: %v", err)
	}

	err = store.StoreGCPCredentials("user1", "GCP Prod", &GCPCredentialConfig{
		ServiceAccountJSON: `{"type": "service_account"}`,
		ProjectID:          "my-project",
	})
	if err != nil {
		t.Fatalf("Failed to store GCP credentials: %v", err)
	}

	// List credentials
	creds := store.ListCredentials("user1")
	if len(creds) != 2 {
		t.Errorf("Expected 2 credentials, got %d", len(creds))
	}

	// Verify sensitive data is not included
	for _, cred := range creds {
		if cred.AWS != nil && cred.AWS.SecretAccessKey != "" {
			t.Error("SecretAccessKey should not be included in list")
		}
		if cred.GCP != nil && cred.GCP.ServiceAccountJSON != "" {
			t.Error("ServiceAccountJSON should not be included in list")
		}
	}
}

func TestCredentialStoreHasCredentials(t *testing.T) {
	store, err := NewCredentialStore("test-encryption-key-32-bytes!!")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Initially no credentials
	if store.HasCredentials("user1", ProviderAWS) {
		t.Error("Expected no AWS credentials for user1")
	}

	// Store AWS credentials
	err = store.StoreAWSCredentials("user1", "AWS", &AWSCredentialConfig{
		RoleARN: "arn:aws:iam::123456789012:role/TestRole",
	})
	if err != nil {
		t.Fatalf("Failed to store credentials: %v", err)
	}

	// Now should have AWS credentials
	if !store.HasCredentials("user1", ProviderAWS) {
		t.Error("Expected AWS credentials for user1")
	}

	// But not GCP
	if store.HasCredentials("user1", ProviderGCP) {
		t.Error("Expected no GCP credentials for user1")
	}
}

func TestCredentialStoreDelete(t *testing.T) {
	store, err := NewCredentialStore("test-encryption-key-32-bytes!!")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Store credentials
	err = store.StoreAWSCredentials("user1", "AWS", &AWSCredentialConfig{
		RoleARN: "arn:aws:iam::123456789012:role/TestRole",
	})
	if err != nil {
		t.Fatalf("Failed to store credentials: %v", err)
	}

	// Verify they exist
	if !store.HasCredentials("user1", ProviderAWS) {
		t.Error("Expected credentials to exist")
	}

	// Delete
	err = store.DeleteCredentials("user1", ProviderAWS)
	if err != nil {
		t.Fatalf("Failed to delete credentials: %v", err)
	}

	// Verify deleted
	if store.HasCredentials("user1", ProviderAWS) {
		t.Error("Expected credentials to be deleted")
	}
}
