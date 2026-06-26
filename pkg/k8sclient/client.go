/*
Copyright 2026.

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

package k8sclient

import (
	"context"
	"fmt"

	operatorv1alpha1 "github.com/redhat-data-and-ai/unstructured-data-controller/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(operatorv1alpha1.AddToScheme(scheme))
}

type Client struct {
	client client.Client
}

func NewInClusterClient() (*Client, error) {
	// rest.InClusterConfig() reads from:
	// 1. /var/run/secrets/kubernetes.io/serviceaccount/token (for auth)
	// 2. /var/run/secrets/kubernetes.io/serviceaccount/ca.crt (for TLS)
	// 3. KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT env vars
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	// Create a controller-runtime client with our scheme (includes CRDs)
	k8sClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &Client{
		client: k8sClient,
	}, nil
}

// PipelineInfo contains UnstructuredDataPipeline CR
type PipelineInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status,omitempty"`
	Message   string `json:"message,omitempty"`
}

// ListPipelines returns all UnstructuredDataPipeline CRs in the unstructured-controller-namespace
func (c *Client) ListPipelines(ctx context.Context) ([]PipelineInfo, error) {
	pipelineList := &operatorv1alpha1.UnstructuredDataPipelineList{}

	err := c.client.List(ctx, pipelineList, &client.ListOptions{
		Namespace: "unstructured-controller-namespace",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pipelines: %w", err)
	}

	result := make([]PipelineInfo, len(pipelineList.Items))
	for i, pipeline := range pipelineList.Items {
		info := PipelineInfo{
			Name:      pipeline.Name,
			Namespace: pipeline.Namespace,
		}

		for _, condition := range pipeline.Status.Conditions {
			if condition.Type == "UnstructuredDataPipelineReady" {
				info.Status = string(condition.Status)
				info.Message = condition.Message
				break
			}
		}

		result[i] = info
	}

	return result, nil
}
