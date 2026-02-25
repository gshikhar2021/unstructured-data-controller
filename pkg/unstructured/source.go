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

	"github.com/redhat-data-and-ai/unstructured-data-controller/pkg/awsclienthandler"
	"github.com/redhat-data-and-ai/unstructured-data-controller/pkg/filestore"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type DataSource interface {
	// SyncFilesToFilestore will store all files from the source to the filestore and return the list of file paths
	SyncFilesToFilestore(ctx context.Context, fs *filestore.FileStore) ([]RawFileMetadata, error)
}

type S3BucketSource struct {
	Bucket string
	Prefix string
}

func (s *S3BucketSource) SyncFilesToFilestore(ctx context.Context, fs *filestore.FileStore) ([]RawFileMetadata, error) {
	logger := log.FromContext(ctx)

	// Use rclone to sync from source S3 bucket to destination S3 bucket
	logger.Info("syncing files using rclone", "sourceBucket", s.Bucket, "sourcePrefix", s.Prefix,
		"destBucket", fs.S3Bucket(), "destPrefix", s.Prefix)

	if err := rcloneSyncS3(ctx, s.Bucket, s.Prefix, fs.S3Bucket(), s.Prefix); err != nil {
		logger.Error(err, "rclone sync failed")
		return nil, fmt.Errorf("rclone sync failed: %w", err)
	}

	logger.Info("rclone sync completed successfully")

	// List synced files from destination bucket and return metadata
	objects, err := awsclienthandler.ListObjectsInPrefix(ctx, fs.S3Bucket(), s.Prefix)
	if err != nil {
		return nil, err
	}

	storedFiles := []RawFileMetadata{}
	for _, object := range objects {
		file := RawFileMetadata{
			FilePath: *object.Key,
			UID:      *object.ETag,
		}
		storedFiles = append(storedFiles, file)
	}

	logger.Info("sync completed", "totalFiles", len(storedFiles))
	return storedFiles, nil
}
