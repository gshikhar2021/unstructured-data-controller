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
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/redhat-data-and-ai/unstructured-data-controller/pkg/auth"
	"github.com/redhat-data-and-ai/unstructured-data-controller/pkg/embedding"
	"github.com/redhat-data-and-ai/unstructured-data-controller/pkg/snowflake"
)

type getChunksArgs struct {
	UDPDatabase string `json:"udp_database" jsonschema:"Name of the data product database. If not known, call list_unstructured_data_pipelines_for_user first and pick the matching pipeline based on its description."`
	Schema      string `json:"schema" jsonschema:"Snowflake schema name. If not known, call list_unstructured_data_pipelines_for_user first."`
	Table       string `json:"table" jsonschema:"Snowflake table name. If not known, call list_unstructured_data_pipelines_for_user first."`
	Query       string `json:"query" jsonschema:"The search query to find relevant chunks"`
}

func RegisterGetChunksForEmbeddings(s *mcp.Server, embeddingClient *embedding.HTTPClient) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "get_chunks_for_embeddings",
		Description: `Search for relevant text chunks in a data product using vector cosine similarity. Returns top 5 matching chunks for the given query.
If udp_database, schema, or table are not known, call list_unstructured_data_pipelines_for_user first and follow the instructions in its response.
On error: report the exact error to the user and STOP. Do NOT retry with other pipelines or databases.
On follow-up: if the user is not satisfied, ask them which pipeline to search. Do NOT automatically try other pipelines.`,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getChunksArgs) (*mcp.CallToolResult, any, error) {
		slog.Info("get_chunks: tool invoked", "udp_database", args.UDPDatabase, "schema", args.Schema, "table", args.Table)

		if args.UDPDatabase == "" || args.Schema == "" || args.Table == "" || args.Query == "" {
			slog.Error("get_chunks: missing required parameters", "udp_database", args.UDPDatabase, "schema", args.Schema, "table", args.Table, "query_empty", args.Query == "")
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "Error: udp_database, schema, table, and query are required. Call list_unstructured_data_pipelines_for_user first to get the database, schema, and table values."}},
				IsError: true,
			}, nil, nil
		}

		oauthToken, ok := auth.AccessTokenFromContext(ctx)
		if !ok {
			slog.Error("get_chunks: OAuth token not found in context")
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "Error: OAuth token not found in context"}},
				IsError: true,
			}, nil, nil
		}

		slog.Info("get_chunks: generating embedding for query")
		result, err := embeddingClient.GenerateEmbeddings(ctx, []string{args.Query}, "float")
		if err != nil {
			slog.Error("get_chunks: failed to generate embedding", "error", err)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error generating embedding: %v", err)}},
				IsError: true,
			}, nil, nil
		}

		if result.Count == 0 || len(result.Embeddings) == 0 {
			slog.Error("get_chunks: embedding API returned no vectors")
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "Error: embedding API returned no vectors"}},
				IsError: true,
			}, nil, nil
		}
		slog.Info("get_chunks: embedding generated", "vector_dimensions", len(result.Embeddings[0]))

		vectorLiteral := formatVectorLiteral(result.Embeddings[0])
		databaseName := strings.ToUpper(strings.ReplaceAll(args.UDPDatabase, "-", "_"))
		schemaName := strings.ToUpper(args.Schema)
		tableName := strings.ToUpper(args.Table)

		slog.Info("get_chunks: searching snowflake", "database", databaseName, "schema", schemaName, "table", tableName)
		chunks, err := snowflake.SearchChunks(ctx, oauthToken, databaseName, schemaName, tableName, vectorLiteral)
		if err != nil {
			slog.Error("get_chunks: failed to search chunks in snowflake", "error", err, "database", databaseName, "schema", schemaName, "table", tableName)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error searching chunks: %v", err)}},
				IsError: true,
			}, nil, nil
		}

		jsonBytes, err := json.Marshal(chunks)
		if err != nil {
			slog.Error("get_chunks: failed to marshal result", "error", err)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error marshaling result: %v", err)}},
				IsError: true,
			}, nil, nil
		}

		slog.Info("get_chunks: completed successfully", "database", databaseName, "chunks_found", len(chunks))
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: fmt.Sprintf("Found %d chunks for query in %s.%s.%s:\n%s", len(chunks), databaseName, schemaName, tableName, string(jsonBytes)),
			}},
		}, nil, nil
	})
}

func formatVectorLiteral(vec []float64) string {
	parts := make([]string, len(vec))
	for i, v := range vec {
		parts[i] = strconv.FormatFloat(v, 'f', -1, 64)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
