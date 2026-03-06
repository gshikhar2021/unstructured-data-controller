package utils

import (
	"github.com/redhat-data-and-ai/unstructured-data-controller/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DefaultE2ENamespace is the namespace used by e2e tests (must match test/e2e/main_test.go testNamespace).
const DefaultE2ENamespace = "unstructured-controller-namespace"

func GetControllerConfigResource() *v1alpha1.ControllerConfig {
	return &v1alpha1.ControllerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "controllerconfig",
			Namespace: DefaultE2ENamespace,
		},
		Spec: v1alpha1.ControllerConfigSpec{
			SnowflakeConfig: v1alpha1.SnowflakeConfig{
				Name:             "e2e",
				Account:          "account-identifier",
				User:             "username",
				Role:             "TESTING_ROLE",
				Region:           "us-west-2",
				Warehouse:        "DEFAULT",
				PrivateKeySecret: "private-key",
			},
			UnstructuredDataProcessingConfig: v1alpha1.UnstructuredDataProcessingConfigSpec{
				DoclingServeURL:             "http://docling-serve:5001",
				IngestionBucket:             "unstructured-bucket",
				DataStorageBucket:           "data-storage-bucket",
				CacheDirectory:              "/data/cache/",
				MaxConcurrentDoclingTasks:   5,
				MaxConcurrentLangchainTasks: 10,
			},
			AWSSecret: "aws-secret",
		},
	}
}
