package tools

import (
	"context"
	"fmt"

	"github.com/awlx/kentik-mcp/pkg/kentik"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerLabelTools(s *server.MCPServer, client *kentik.Client) {
	listLabels := mcp.NewTool("kentik_list_labels",
		mcp.WithDescription("List all device labels (tags used to group devices) in Kentik."),
	)
	s.AddTool(listLabels, makeListLabelsHandler(client))

	getLabel := mcp.NewTool("kentik_get_label",
		mcp.WithDescription("Get information about a specific device label by ID."),
		mcp.WithString("label_id",
			mcp.Required(),
			mcp.Description("The ID of the label"),
		),
	)
	s.AddTool(getLabel, makeGetLabelHandler(client))
}

func makeListLabelsHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := client.V5("GET", "/deviceLabels", nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list labels: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(data)), nil
	}
}

func makeGetLabelHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		labelID, err := request.RequireString("label_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		data, err := client.V5("GET", fmt.Sprintf("/deviceLabels/%s", labelID), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get label: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(data)), nil
	}
}
