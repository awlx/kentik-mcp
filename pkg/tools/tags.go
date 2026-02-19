package tools

import (
	"context"
	"fmt"

	"github.com/awlx/kentik-mcp/pkg/kentik"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTagTools(s *server.MCPServer, client *kentik.Client) {
	listTags := mcp.NewTool("kentik_list_tags",
		mcp.WithDescription("List all flow tags in Kentik. Flow tags are used to classify and label network traffic."),
	)
	s.AddTool(listTags, makeListTagsHandler(client))

	getTag := mcp.NewTool("kentik_get_tag",
		mcp.WithDescription("Get information about a specific flow tag by ID."),
		mcp.WithString("tag_id",
			mcp.Required(),
			mcp.Description("The ID of the tag"),
		),
	)
	s.AddTool(getTag, makeGetTagHandler(client))
}

func makeListTagsHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := client.V5("GET", "/tags", nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list tags: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(data)), nil
	}
}

func makeGetTagHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tagID, err := request.RequireString("tag_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		data, err := client.V5("GET", fmt.Sprintf("/tag/%s", tagID), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get tag: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(data)), nil
	}
}
