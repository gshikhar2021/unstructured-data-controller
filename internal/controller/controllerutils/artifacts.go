package controllerutils

import (
	"context"

	operatorv1alpha1 "github.com/redhat-data-and-ai/unstructured-data-controller/api/v1alpha1"
	"github.com/redhat-data-and-ai/unstructured-data-controller/pkg/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ArtifactNameDocumentProcessor = "documentProcessorConfig"
	ArtifactNameChunksGenerator   = "chunksGeneratorConfig"
	ArtifactNameVectorEmbeddings  = "vectorEmbeddingsGeneratorConfig"
)

// buildArtifactPathMap builds a map from file suffix to artifact path based on the artifacts configuration
func BuildArtifactPathMap(ctx context.Context, artifacts []operatorv1alpha1.ArtifactConfig) map[string]string {
	logger := log.FromContext(ctx)
	artifactPathMap := make(map[string]string)
	for _, artifact := range artifacts {
		switch artifact.Name {
		case ArtifactNameDocumentProcessor:
			artifactPathMap[unstructured.ConvertedFileSuffix] = artifact.Path
		case ArtifactNameChunksGenerator:
			artifactPathMap[unstructured.ChunksFileSuffix] = artifact.Path
		case ArtifactNameVectorEmbeddings:
			artifactPathMap[unstructured.VectorEmbeddingsFileSuffix] = artifact.Path
		default:
			logger.Info("unknown artifact name, skipping", "name", artifact.Name)
		}
	}
	return artifactPathMap
}

func BuildDestinationSyncFilePaths(ctx context.Context, unstructuredDataProductCR *operatorv1alpha1.UnstructuredDataProduct, filePaths []string) []string {
	var filesToSync []string
	logger := log.FromContext(ctx)

	// Iterate through artifacts configuration to determine which files to sync
	for _, artifact := range unstructuredDataProductCR.Spec.DestinationConfig.Artifacts {
		var filteredFiles []string

		switch artifact.Name {
		case ArtifactNameDocumentProcessor:
			filteredFiles = unstructured.FilterConvertedFilePaths(filePaths)
		case ArtifactNameChunksGenerator:
			filteredFiles = unstructured.FilterChunksFilePaths(filePaths)
		case ArtifactNameVectorEmbeddings:
			filteredFiles = unstructured.FilterVectorEmbeddingsFilePaths(filePaths)
		default:
			logger.Info("unknown artifact name, skipping", "name", artifact.Name)
		}

		filesToSync = append(filesToSync, filteredFiles...)
	}

	return filesToSync
}
