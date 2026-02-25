/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package unstructured

import (
	"context"
	"fmt"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/sync"
	_ "github.com/rclone/rclone/backend/s3"
	"github.com/redhat-data-and-ai/unstructured-data-controller/pkg/awsclienthandler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// rcloneSyncS3 syncs files from source S3 bucket to destination S3 bucket using rclone
// This supports both AWS S3 and localstack
// It uses the existing AWS configuration from awsclienthandler
func rcloneSyncS3(ctx context.Context, sourceBucket, sourcePrefix, destBucket, destPrefix string) error {
	logger := log.FromContext(ctx)
	logger.Info("starting rclone sync", "sourceBucket", sourceBucket, "sourcePrefix", sourcePrefix,
		"destBucket", destBucket, "destPrefix", destPrefix)

	// Get AWS S3 client to extract configuration
	s3Client, err := awsclienthandler.GetS3Client()
	if err != nil {
		logger.Error(err, "failed to get S3 client from awsclienthandler")
		return fmt.Errorf("failed to get S3 client: %w", err)
	}

	// Extract AWS configuration from the S3 client
	awsConfig, err := s3Client.Options().Credentials.Retrieve(ctx)
	if err != nil {
		logger.Error(err, "failed to retrieve AWS credentials")
		return fmt.Errorf("failed to retrieve AWS credentials: %w", err)
	}

	region := s3Client.Options().Region
	if region == "" {
		region = "us-east-1" // default region
	}

	// Get endpoint if configured (for localstack support)
	endpoint := ""
	if s3Client.Options().BaseEndpoint != nil {
		endpoint = *s3Client.Options().BaseEndpoint
		logger.Info("using custom S3 endpoint", "endpoint", endpoint)
	}

	// Configure source S3 filesystem using existing AWS credentials
	sourceConfig := configmap.Simple{
		"type":              "s3",
		"provider":          "AWS",
		"access_key_id":     awsConfig.AccessKeyID,
		"secret_access_key": awsConfig.SecretAccessKey,
		"region":            region,
		"env_auth":          "false",
	}

	// Add session token if present (for temporary credentials)
	if awsConfig.SessionToken != "" {
		sourceConfig["session_token"] = awsConfig.SessionToken
	}

	// If endpoint is set (localstack), add endpoint configuration
	if endpoint != "" {
		sourceConfig["endpoint"] = endpoint
		sourceConfig["force_path_style"] = "true" // Required for localstack
	}

	// Configure destination S3 filesystem (same credentials as source)
	destConfig := configmap.Simple{
		"type":              "s3",
		"provider":          "AWS",
		"access_key_id":     awsConfig.AccessKeyID,
		"secret_access_key": awsConfig.SecretAccessKey,
		"region":            region,
		"env_auth":          "false",
	}

	// Add session token if present (for temporary credentials)
	if awsConfig.SessionToken != "" {
		destConfig["session_token"] = awsConfig.SessionToken
	}

	// If endpoint is set (localstack), add endpoint configuration
	if endpoint != "" {
		destConfig["endpoint"] = endpoint
		destConfig["force_path_style"] = "true" // Required for localstack
	}

	// Create source filesystem
	sourcePath := fmt.Sprintf("%s/%s", sourceBucket, sourcePrefix)
	logger.Info("creating source filesystem", "path", sourcePath)
	srcFs, err := fs.NewFs(ctx, fmt.Sprintf(":s3:%s", sourcePath))
	if err != nil {
		logger.Error(err, "failed to create source filesystem")
		return fmt.Errorf("failed to create source filesystem: %w", err)
	}

	// Register source config
	fs.ConfigFileSet("source", "type", "s3")
	for k, v := range sourceConfig {
		fs.ConfigFileSet("source", k, v)
	}

	// Create destination filesystem
	destPath := fmt.Sprintf("%s/%s", destBucket, destPrefix)
	logger.Info("creating destination filesystem", "path", destPath)
	dstFs, err := fs.NewFs(ctx, fmt.Sprintf(":s3:%s", destPath))
	if err != nil {
		logger.Error(err, "failed to create destination filesystem")
		return fmt.Errorf("failed to create destination filesystem: %w", err)
	}

	// Register destination config
	fs.ConfigFileSet("dest", "type", "s3")
	for k, v := range destConfig {
		fs.ConfigFileSet("dest", k, v)
	}

	// Perform sync operation
	// The third parameter (false) means don't delete files from destination that aren't in source
	// Set to true if you want exact mirror sync
	logger.Info("starting sync operation")
	if err := sync.Sync(ctx, dstFs, srcFs, true); err != nil {
		logger.Error(err, "sync operation failed")
		return fmt.Errorf("sync operation failed: %w", err)
	}

	logger.Info("rclone sync completed successfully")
	return nil
}
