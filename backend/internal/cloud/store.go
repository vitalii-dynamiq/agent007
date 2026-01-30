package cloud

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// CredentialStore manages encrypted storage of user cloud credentials
type CredentialStore struct {
	credentials map[string]map[ProviderType]*UserCloudCredentials // userID -> provider -> credentials
	encryptionKey []byte
	mu            sync.RWMutex
}

// NewCredentialStore creates a new credential store with encryption
func NewCredentialStore(encryptionKey string) (*CredentialStore, error) {
	// Key must be 32 bytes for AES-256
	key := []byte(encryptionKey)
	if len(key) < 32 {
		// Pad or hash to 32 bytes
		padded := make([]byte, 32)
		copy(padded, key)
		key = padded
	} else if len(key) > 32 {
		key = key[:32]
	}

	return &CredentialStore{
		credentials:   make(map[string]map[ProviderType]*UserCloudCredentials),
		encryptionKey: key,
	}, nil
}

// encrypt encrypts data using AES-256-GCM
func (s *CredentialStore) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts data using AES-256-GCM
func (s *CredentialStore) decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], string(data[nonceSize:])
	plaintext, err := gcm.Open(nil, nonce, []byte(ciphertext), nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// StoreAWSCredentials stores AWS credentials for a user
func (s *CredentialStore) StoreAWSCredentials(userID, name string, config *AWSCredentialConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Encrypt sensitive fields
	var err error
	storedConfig := *config

	if config.SecretAccessKey != "" {
		storedConfig.SecretAccessKey, err = s.encrypt(config.SecretAccessKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt secret access key: %w", err)
		}
	}

	if s.credentials[userID] == nil {
		s.credentials[userID] = make(map[ProviderType]*UserCloudCredentials)
	}

	s.credentials[userID][ProviderAWS] = &UserCloudCredentials{
		UserID:    userID,
		Provider:  ProviderAWS,
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		AWS:       &storedConfig,
	}

	return nil
}

// StoreGCPCredentials stores GCP credentials for a user
func (s *CredentialStore) StoreGCPCredentials(userID, name string, config *GCPCredentialConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Encrypt the service account JSON
	encryptedJSON, err := s.encrypt(config.ServiceAccountJSON)
	if err != nil {
		return fmt.Errorf("failed to encrypt service account JSON: %w", err)
	}

	storedConfig := *config
	storedConfig.ServiceAccountJSON = encryptedJSON

	if s.credentials[userID] == nil {
		s.credentials[userID] = make(map[ProviderType]*UserCloudCredentials)
	}

	s.credentials[userID][ProviderGCP] = &UserCloudCredentials{
		UserID:    userID,
		Provider:  ProviderGCP,
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		GCP:       &storedConfig,
	}

	return nil
}

// GetAWSCredentials retrieves and decrypts AWS credentials
func (s *CredentialStore) GetAWSCredentials(userID string) (*AWSCredentialConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userCreds, ok := s.credentials[userID]
	if !ok {
		return nil, fmt.Errorf("no credentials found for user %s", userID)
	}

	creds, ok := userCreds[ProviderAWS]
	if !ok || creds.AWS == nil {
		return nil, fmt.Errorf("no AWS credentials found for user %s", userID)
	}

	// Decrypt sensitive fields
	config := *creds.AWS
	if config.SecretAccessKey != "" {
		decrypted, err := s.decrypt(config.SecretAccessKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt secret access key: %w", err)
		}
		config.SecretAccessKey = decrypted
	}

	return &config, nil
}

// GetGCPCredentials retrieves and decrypts GCP credentials
func (s *CredentialStore) GetGCPCredentials(userID string) (*GCPCredentialConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userCreds, ok := s.credentials[userID]
	if !ok {
		return nil, fmt.Errorf("no credentials found for user %s", userID)
	}

	creds, ok := userCreds[ProviderGCP]
	if !ok || creds.GCP == nil {
		return nil, fmt.Errorf("no GCP credentials found for user %s", userID)
	}

	// Decrypt service account JSON
	config := *creds.GCP
	decrypted, err := s.decrypt(config.ServiceAccountJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt service account JSON: %w", err)
	}
	config.ServiceAccountJSON = decrypted

	return &config, nil
}

// ListCredentials lists all credentials for a user (without sensitive data)
func (s *CredentialStore) ListCredentials(userID string) []UserCloudCredentials {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userCreds, ok := s.credentials[userID]
	if !ok {
		return nil
	}

	result := make([]UserCloudCredentials, 0, len(userCreds))
	for _, cred := range userCreds {
		// Create a copy without sensitive data
		safeCred := UserCloudCredentials{
			UserID:    cred.UserID,
			Provider:  cred.Provider,
			Name:      cred.Name,
			CreatedAt: cred.CreatedAt,
			UpdatedAt: cred.UpdatedAt,
		}

		if cred.AWS != nil {
			safeCred.AWS = &AWSCredentialConfig{
				AccountID:      cred.AWS.AccountID,
				RoleARN:         cred.AWS.RoleARN,
				Region:          cred.AWS.Region,
				SessionDuration: cred.AWS.SessionDuration,
				// Don't include AccessKeyID or SecretAccessKey
			}
		}

		if cred.GCP != nil {
			safeCred.GCP = &GCPCredentialConfig{
				ProjectID:                 cred.GCP.ProjectID,
				ImpersonateServiceAccount: cred.GCP.ImpersonateServiceAccount,
				Scopes:                    cred.GCP.Scopes,
				// Don't include ServiceAccountJSON
			}
		}

		result = append(result, safeCred)
	}

	return result
}

// DeleteCredentials deletes credentials for a user and provider
func (s *CredentialStore) DeleteCredentials(userID string, provider ProviderType) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	userCreds, ok := s.credentials[userID]
	if !ok {
		return fmt.Errorf("no credentials found for user %s", userID)
	}

	if _, ok := userCreds[provider]; !ok {
		return fmt.Errorf("no %s credentials found for user %s", provider, userID)
	}

	delete(userCreds, provider)
	return nil
}

// HasCredentials checks if a user has credentials for a provider
func (s *CredentialStore) HasCredentials(userID string, provider ProviderType) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userCreds, ok := s.credentials[userID]
	if !ok {
		return false
	}

	_, ok = userCreds[provider]
	return ok
}

// ExportForBackup exports all credentials (still encrypted) for backup
func (s *CredentialStore) ExportForBackup() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return json.Marshal(s.credentials)
}

// ImportFromBackup imports credentials from a backup
func (s *CredentialStore) ImportFromBackup(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var creds map[string]map[ProviderType]*UserCloudCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return err
	}

	s.credentials = creds
	return nil
}
