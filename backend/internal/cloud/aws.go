package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const (
	defaultAWSRegion       = "us-east-1"
	defaultSessionDuration = time.Hour
	maxSessionDuration     = 12 * time.Hour
	minSessionDuration     = 15 * time.Minute
)

// AWSProvider handles AWS credential operations
type AWSProvider struct {
	// Default credentials for assuming roles (if user doesn't provide their own)
	defaultAccessKeyID     string
	defaultSecretAccessKey string
}

// NewAWSProvider creates a new AWS provider
func NewAWSProvider(accessKeyID, secretAccessKey string) *AWSProvider {
	return &AWSProvider{
		defaultAccessKeyID:     accessKeyID,
		defaultSecretAccessKey: secretAccessKey,
	}
}

// GetCredentialsForSandbox returns AWS credentials for a sandbox session
// This is the main entry point called by the credential endpoint
func (p *AWSProvider) GetCredentialsForSandbox(ctx context.Context, userConfig *AWSCredentialConfig, sandboxID, userID string) (*AWSCredentials, error) {
	// Create a unique session name (max 64 chars)
	sessionName := fmt.Sprintf("dynamiq-%s", sandboxID)
	if len(sessionName) > 64 {
		sessionName = sessionName[:64]
	}

	if userConfig.RoleARN != "" {
		return p.AssumeRole(ctx, userConfig, sessionName)
	}

	return p.GetSessionToken(ctx, userConfig)
}

// AssumeRole assumes an IAM role and returns temporary credentials
func (p *AWSProvider) AssumeRole(ctx context.Context, userConfig *AWSCredentialConfig, sessionName string) (*AWSCredentials, error) {
	// Determine which credentials to use for assuming the role
	accessKeyID := userConfig.AccessKeyID
	secretAccessKey := userConfig.SecretAccessKey
	if accessKeyID == "" {
		accessKeyID = p.defaultAccessKeyID
		secretAccessKey = p.defaultSecretAccessKey
	}

	// Determine region
	region := userConfig.Region
	if region == "" {
		region = defaultAWSRegion
	}

	// Build AWS config
	var cfg aws.Config
	var err error

	if accessKeyID != "" && secretAccessKey != "" {
		// Use provided credentials
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				accessKeyID,
				secretAccessKey,
				"", // session token
			)),
		)
	} else {
		// Use default credential chain (environment, IAM role, etc.)
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create STS client
	stsClient := sts.NewFromConfig(cfg)

	// Determine session duration
	duration := userConfig.SessionDuration
	if duration == 0 {
		duration = defaultSessionDuration
	}
	if duration < minSessionDuration {
		duration = minSessionDuration
	}
	if duration > maxSessionDuration {
		duration = maxSessionDuration
	}
	durationSeconds := int32(duration.Seconds())

	// Build AssumeRole input
	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(userConfig.RoleARN),
		RoleSessionName: aws.String(sessionName),
		DurationSeconds: aws.Int32(durationSeconds),
	}

	if userConfig.ExternalID != "" {
		input.ExternalId = aws.String(userConfig.ExternalID)
	}

	// Call STS AssumeRole
	result, err := stsClient.AssumeRole(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to assume role: %w", err)
	}

	if result.Credentials == nil {
		return nil, fmt.Errorf("no credentials returned from STS")
	}

	return &AWSCredentials{
		Version:         1,
		AccessKeyId:     aws.ToString(result.Credentials.AccessKeyId),
		SecretAccessKey: aws.ToString(result.Credentials.SecretAccessKey),
		SessionToken:    aws.ToString(result.Credentials.SessionToken),
		Expiration:      aws.ToTime(result.Credentials.Expiration),
	}, nil
}

// GetSessionToken fetches temporary session credentials for a user access key
// This is used when no role ARN is provided.
func (p *AWSProvider) GetSessionToken(ctx context.Context, userConfig *AWSCredentialConfig) (*AWSCredentials, error) {
	accessKeyID := userConfig.AccessKeyID
	secretAccessKey := userConfig.SecretAccessKey
	if accessKeyID == "" {
		accessKeyID = p.defaultAccessKeyID
		secretAccessKey = p.defaultSecretAccessKey
	}
	if accessKeyID == "" || secretAccessKey == "" {
		return nil, fmt.Errorf("access key credentials are required to get session token")
	}

	region := userConfig.Region
	if region == "" {
		region = defaultAWSRegion
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKeyID,
			secretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	stsClient := sts.NewFromConfig(cfg)

	duration := userConfig.SessionDuration
	if duration == 0 {
		duration = defaultSessionDuration
	}
	if duration < minSessionDuration {
		duration = minSessionDuration
	}
	if duration > maxSessionDuration {
		duration = maxSessionDuration
	}
	durationSeconds := int32(duration.Seconds())

	result, err := stsClient.GetSessionToken(ctx, &sts.GetSessionTokenInput{
		DurationSeconds: aws.Int32(durationSeconds),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get session token: %w", err)
	}

	if result.Credentials == nil {
		return nil, fmt.Errorf("no credentials returned from STS")
	}

	return &AWSCredentials{
		Version:         1,
		AccessKeyId:     aws.ToString(result.Credentials.AccessKeyId),
		SecretAccessKey: aws.ToString(result.Credentials.SecretAccessKey),
		SessionToken:    aws.ToString(result.Credentials.SessionToken),
		Expiration:      aws.ToTime(result.Credentials.Expiration),
	}, nil
}

// GetCallerIdentity returns the caller identity for the given credentials
// Useful for validating credentials are working
func (p *AWSProvider) GetCallerIdentity(ctx context.Context, userConfig *AWSCredentialConfig) (string, error) {
	accessKeyID := userConfig.AccessKeyID
	secretAccessKey := userConfig.SecretAccessKey
	if accessKeyID == "" {
		accessKeyID = p.defaultAccessKeyID
		secretAccessKey = p.defaultSecretAccessKey
	}

	region := userConfig.Region
	if region == "" {
		region = defaultAWSRegion
	}

	var cfg aws.Config
	var err error

	if accessKeyID != "" && secretAccessKey != "" {
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				accessKeyID,
				secretAccessKey,
				"",
			)),
		)
	} else {
		cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(region))
	}

	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w", err)
	}

	stsClient := sts.NewFromConfig(cfg)
	result, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("failed to get caller identity: %w", err)
	}

	return aws.ToString(result.Arn), nil
}

// FormatCredentialProcessOutput formats credentials for AWS credential_process
// AWS CLI expects this exact JSON format from credential_process scripts
func FormatCredentialProcessOutput(creds *AWSCredentials) (string, error) {
	// AWS credential_process expects this exact JSON format
	output := map[string]interface{}{
		"Version":         1,
		"AccessKeyId":     creds.AccessKeyId,
		"SecretAccessKey": creds.SecretAccessKey,
		"SessionToken":    creds.SessionToken,
		"Expiration":      creds.Expiration.Format(time.RFC3339),
	}

	data, err := json.Marshal(output)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
