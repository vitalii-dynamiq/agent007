package integrations

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore provides persistent storage for user integrations using SQLite
type SQLiteStore struct {
	db  *sql.DB
	mu  sync.RWMutex
	key []byte // encryption key for sensitive data
}

// NewSQLiteStore creates a new SQLite store at the given data directory
func NewSQLiteStore(dataDir string, encryptionKey string) (*SQLiteStore, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "integrations.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrent performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Prepare encryption key (pad or truncate to 32 bytes for AES-256)
	key := []byte(encryptionKey)
	if len(key) < 32 {
		padded := make([]byte, 32)
		copy(padded, key)
		key = padded
	} else if len(key) > 32 {
		key = key[:32]
	}

	store := &SQLiteStore{
		db:  db,
		key: key,
	}

	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Printf("SQLite integration store initialized at %s", dbPath)
	return store, nil
}

// migrate creates or updates the database schema
func (s *SQLiteStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS user_integrations (
		user_id TEXT NOT NULL,
		integration_id TEXT NOT NULL,
		enabled INTEGER DEFAULT 1,
		connected_at TEXT,
		expires_at TEXT,
		account_name TEXT,
		account_id TEXT,
		organization TEXT,
		oauth2_token TEXT,
		api_key TEXT,
		service_account TEXT,
		iam_role_config TEXT,
		database_config TEXT,
		github_installation_id INTEGER,
		created_at TEXT DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, integration_id)
	);
	
	CREATE INDEX IF NOT EXISTS idx_user_integrations_user_id ON user_integrations(user_id);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// SaveUserIntegration saves or updates a user integration
func (s *SQLiteStore) SaveUserIntegration(ui *UserIntegration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Serialize complex types to JSON
	oauth2TokenJSON := ""
	if ui.OAuth2Token != nil {
		if bytes, err := json.Marshal(ui.OAuth2Token); err == nil {
			oauth2TokenJSON = string(bytes)
		}
	}

	iamRoleConfigJSON := ""
	if ui.IAMRoleConfig != nil {
		if bytes, err := json.Marshal(ui.IAMRoleConfig); err == nil {
			iamRoleConfigJSON = string(bytes)
		}
	}

	databaseConfigJSON := ""
	if ui.DatabaseConfig != nil {
		if bytes, err := json.Marshal(ui.DatabaseConfig); err == nil {
			databaseConfigJSON = string(bytes)
		}
	}

	query := `
	INSERT INTO user_integrations (
		user_id, integration_id, enabled, connected_at, expires_at,
		account_name, account_id, organization,
		oauth2_token, api_key, service_account, iam_role_config, database_config,
		github_installation_id, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(user_id, integration_id) DO UPDATE SET
		enabled = excluded.enabled,
		connected_at = excluded.connected_at,
		expires_at = excluded.expires_at,
		account_name = excluded.account_name,
		account_id = excluded.account_id,
		organization = excluded.organization,
		oauth2_token = excluded.oauth2_token,
		api_key = excluded.api_key,
		service_account = excluded.service_account,
		iam_role_config = excluded.iam_role_config,
		database_config = excluded.database_config,
		github_installation_id = excluded.github_installation_id,
		updated_at = excluded.updated_at
	`

	enabledInt := 0
	if ui.Enabled {
		enabledInt = 1
	}

	connectedAt := ""
	if !ui.ConnectedAt.IsZero() {
		connectedAt = ui.ConnectedAt.Format(time.RFC3339)
	}

	expiresAt := ""
	if !ui.ExpiresAt.IsZero() {
		expiresAt = ui.ExpiresAt.Format(time.RFC3339)
	}

	_, err := s.db.Exec(query,
		ui.UserID,
		ui.IntegrationID,
		enabledInt,
		connectedAt,
		expiresAt,
		ui.AccountName,
		ui.AccountID,
		ui.Organization,
		oauth2TokenJSON,
		ui.APIKey,
		ui.ServiceAccount,
		iamRoleConfigJSON,
		databaseConfigJSON,
		ui.GitHubInstallationID,
		time.Now().Format(time.RFC3339),
	)

	return err
}

// GetUserIntegration retrieves a user integration
func (s *SQLiteStore) GetUserIntegration(userID, integrationID string) (*UserIntegration, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT user_id, integration_id, enabled, connected_at, expires_at,
		account_name, account_id, organization,
		oauth2_token, api_key, service_account, iam_role_config, database_config,
		github_installation_id
	FROM user_integrations
	WHERE user_id = ? AND integration_id = ?
	`

	var ui UserIntegration
	var enabledInt int
	var connectedAt, expiresAt sql.NullString
	var accountName, accountID, organization sql.NullString
	var oauth2TokenJSON, apiKey, serviceAccount sql.NullString
	var iamRoleConfigJSON, databaseConfigJSON sql.NullString
	var githubInstallationID sql.NullInt64

	err := s.db.QueryRow(query, userID, integrationID).Scan(
		&ui.UserID,
		&ui.IntegrationID,
		&enabledInt,
		&connectedAt,
		&expiresAt,
		&accountName,
		&accountID,
		&organization,
		&oauth2TokenJSON,
		&apiKey,
		&serviceAccount,
		&iamRoleConfigJSON,
		&databaseConfigJSON,
		&githubInstallationID,
	)

	if err == sql.ErrNoRows {
		return nil, false
	}
	if err != nil {
		log.Printf("Error querying user integration: %v", err)
		return nil, false
	}

	ui.Enabled = enabledInt == 1

	if connectedAt.Valid && connectedAt.String != "" {
		if t, err := time.Parse(time.RFC3339, connectedAt.String); err == nil {
			ui.ConnectedAt = t
		}
	}

	if expiresAt.Valid && expiresAt.String != "" {
		if t, err := time.Parse(time.RFC3339, expiresAt.String); err == nil {
			ui.ExpiresAt = t
		}
	}

	if accountName.Valid {
		ui.AccountName = accountName.String
	}
	if accountID.Valid {
		ui.AccountID = accountID.String
	}
	if organization.Valid {
		ui.Organization = organization.String
	}

	if oauth2TokenJSON.Valid && oauth2TokenJSON.String != "" {
		var token OAuth2Token
		if err := json.Unmarshal([]byte(oauth2TokenJSON.String), &token); err == nil {
			ui.OAuth2Token = &token
		}
	}

	if apiKey.Valid {
		ui.APIKey = apiKey.String
	}
	if serviceAccount.Valid {
		ui.ServiceAccount = serviceAccount.String
	}

	if iamRoleConfigJSON.Valid && iamRoleConfigJSON.String != "" {
		var config IAMRoleConfig
		if err := json.Unmarshal([]byte(iamRoleConfigJSON.String), &config); err == nil {
			ui.IAMRoleConfig = &config
		}
	}

	if databaseConfigJSON.Valid && databaseConfigJSON.String != "" {
		var config DatabaseConfig
		if err := json.Unmarshal([]byte(databaseConfigJSON.String), &config); err == nil {
			ui.DatabaseConfig = &config
		}
	}

	if githubInstallationID.Valid {
		ui.GitHubInstallationID = githubInstallationID.Int64
	}

	return &ui, true
}

// ListUserIntegrations returns all integrations for a user
func (s *SQLiteStore) ListUserIntegrations(userID string) []*UserIntegration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT user_id, integration_id, enabled, connected_at, expires_at,
		account_name, account_id, organization,
		oauth2_token, api_key, service_account, iam_role_config, database_config,
		github_installation_id
	FROM user_integrations
	WHERE user_id = ?
	`

	rows, err := s.db.Query(query, userID)
	if err != nil {
		log.Printf("Error listing user integrations: %v", err)
		return nil
	}
	defer rows.Close()

	var result []*UserIntegration

	for rows.Next() {
		var ui UserIntegration
		var enabledInt int
		var connectedAt, expiresAt sql.NullString
		var accountName, accountID, organization sql.NullString
		var oauth2TokenJSON, apiKey, serviceAccount sql.NullString
		var iamRoleConfigJSON, databaseConfigJSON sql.NullString
		var githubInstallationID sql.NullInt64

		err := rows.Scan(
			&ui.UserID,
			&ui.IntegrationID,
			&enabledInt,
			&connectedAt,
			&expiresAt,
			&accountName,
			&accountID,
			&organization,
			&oauth2TokenJSON,
			&apiKey,
			&serviceAccount,
			&iamRoleConfigJSON,
			&databaseConfigJSON,
			&githubInstallationID,
		)

		if err != nil {
			log.Printf("Error scanning user integration row: %v", err)
			continue
		}

		ui.Enabled = enabledInt == 1

		if connectedAt.Valid && connectedAt.String != "" {
			if t, err := time.Parse(time.RFC3339, connectedAt.String); err == nil {
				ui.ConnectedAt = t
			}
		}

		if expiresAt.Valid && expiresAt.String != "" {
			if t, err := time.Parse(time.RFC3339, expiresAt.String); err == nil {
				ui.ExpiresAt = t
			}
		}

		if accountName.Valid {
			ui.AccountName = accountName.String
		}
		if accountID.Valid {
			ui.AccountID = accountID.String
		}
		if organization.Valid {
			ui.Organization = organization.String
		}

		if oauth2TokenJSON.Valid && oauth2TokenJSON.String != "" {
			var token OAuth2Token
			if err := json.Unmarshal([]byte(oauth2TokenJSON.String), &token); err == nil {
				ui.OAuth2Token = &token
			}
		}

		if apiKey.Valid {
			ui.APIKey = apiKey.String
		}
		if serviceAccount.Valid {
			ui.ServiceAccount = serviceAccount.String
		}

		if iamRoleConfigJSON.Valid && iamRoleConfigJSON.String != "" {
			var config IAMRoleConfig
			if err := json.Unmarshal([]byte(iamRoleConfigJSON.String), &config); err == nil {
				ui.IAMRoleConfig = &config
			}
		}

		if databaseConfigJSON.Valid && databaseConfigJSON.String != "" {
			var config DatabaseConfig
			if err := json.Unmarshal([]byte(databaseConfigJSON.String), &config); err == nil {
				ui.DatabaseConfig = &config
			}
		}

		if githubInstallationID.Valid {
			ui.GitHubInstallationID = githubInstallationID.Int64
		}

		result = append(result, &ui)
	}

	return result
}

// DeleteUserIntegration deletes a user integration
func (s *SQLiteStore) DeleteUserIntegration(userID, integrationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		"DELETE FROM user_integrations WHERE user_id = ? AND integration_id = ?",
		userID, integrationID,
	)
	return err
}

// GetAllUserIntegrations loads all integrations into memory (for registry initialization)
func (s *SQLiteStore) GetAllUserIntegrations() map[string]map[string]*UserIntegration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]map[string]*UserIntegration)

	query := `
	SELECT DISTINCT user_id FROM user_integrations
	`

	rows, err := s.db.Query(query)
	if err != nil {
		log.Printf("Error getting all users: %v", err)
		return result
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err == nil {
			userIDs = append(userIDs, userID)
		}
	}

	for _, userID := range userIDs {
		integrations := s.ListUserIntegrations(userID)
		if len(integrations) > 0 {
			result[userID] = make(map[string]*UserIntegration)
			for _, ui := range integrations {
				result[userID][ui.IntegrationID] = ui
			}
		}
	}

	return result
}
