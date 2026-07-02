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

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/redhat-data-and-ai/unstructured-data-controller/pkg/auth"
	"github.com/redhat-data-and-ai/unstructured-data-controller/pkg/k8sclient"
	"github.com/redhat-data-and-ai/unstructured-data-controller/pkg/snowflake"
)

// CombinedResult contains both Kubernetes pipelines and Snowflake databases
type CombinedResult struct {
	Pipelines []k8sclient.PipelineInfo `json:"pipelines"`
	Databases []snowflake.DatabaseInfo `json:"databases"`
}

// RegisterListPipelines registers the list_unstructured_data_pipelines_for_user MCP tool
func RegisterListPipelines(s *mcp.Server, k8sClient *k8sclient.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_unstructured_data_pipelines_for_user",
		Description: "List all UnstructuredDataPipeline custom resources and Snowflake databases the authenticated user has access to.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		slog.Info("list_pipelines: tool invoked")

		oauthToken, ok := auth.AccessTokenFromContext(ctx)
		if !ok {
			slog.Error("list_pipelines: OAuth token not found in context")
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: "Error: OAuth token not found in context",
				}},
				IsError: true,
			}, nil, nil
		}

		var pipelines []k8sclient.PipelineInfo
		if k8sClient != nil {
			var err error
			pipelines, err = k8sClient.ListPipelines(ctx)
			if err != nil {
				slog.Error("list_pipelines: failed to list pipelines from kubernetes", "error", err)
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{
						Text: fmt.Sprintf("Error listing pipelines: %v", err),
					}},
					IsError: true,
				}, nil, nil
			}
			slog.Info("list_pipelines: listed pipelines from kubernetes", "count", len(pipelines))
		} else {
			slog.Warn("list_pipelines: kubernetes client is nil, skipping pipeline listing")
		}

		databases, err := snowflake.ShowDatabases(ctx, oauthToken)
		if err != nil {
			slog.Error("list_pipelines: failed to list databases from snowflake", "error", err)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("Error querying Snowflake: %v", err),
				}},
				IsError: true,
			}, nil, nil
		}
		slog.Info("list_pipelines: listed databases from snowflake", "count", len(databases))

		result := CombinedResult{
			Pipelines: pipelines,
			Databases: databases,
		}

		jsonBytes, err := json.Marshal(result)
		if err != nil {
			slog.Error("list_pipelines: failed to marshal result", "error", err)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("Error marshaling result: %v", err),
				}},
				IsError: true,
			}, nil, nil
		}

		slog.Info("list_pipelines: completed successfully", "pipelines", len(pipelines), "databases", len(databases))
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: fmt.Sprintf("Found %d pipeline(s) and %d database(s):\n%s\n\n"+
					"IMPORTANT: If you are selecting a pipeline for a search query, you MUST follow these rules:\n"+
					"- If EXACTLY ONE pipeline matches the user's question, use it.\n"+
					"- If MORE THAN ONE pipeline could match, STOP and ask the user which one to use. Do NOT pick one yourself.\n"+
					"- If NONE match, tell the user. Do NOT try all pipelines.",
					len(pipelines), len(databases), string(jsonBytes)),
			}},
		}, nil, nil
	})
}
