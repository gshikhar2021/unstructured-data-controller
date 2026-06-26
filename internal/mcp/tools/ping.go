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
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/redhat-data-and-ai/unstructured-data-controller/pkg/auth"
)

type pingArgs struct {
	Message string `json:"message,omitempty" jsonschema:"Optional message to echo back"`
}

// RegisterPing registers the ping MCP tool
func RegisterPing(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "ping",
		Description: "Health check tool that returns pong along with the authenticated user's name",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args pingArgs) (*mcp.CallToolResult, any, error) {
		user := "unknown"
		if info, ok := auth.TokenInfoFromContext(ctx); ok {
			if info.Username != "" {
				user = info.Username
			} else if info.Sub != "" {
				user = info.Sub
			}
		}

		text := fmt.Sprintf("pong (user: %s)", user)
		if args.Message != "" {
			text = fmt.Sprintf("pong: %s (user: %s)", args.Message, user)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})
}
