package tools

import (
	"context"
	"fmt"

	"github.com/awlx/kentik-mcp/pkg/kentik"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerUserTools(s *server.MCPServer, client *kentik.Client) {
	listUsers := mcp.NewTool("kentik_list_users",
		mcp.WithDescription("List all users registered in the Kentik organization."),
	)
	s.AddTool(listUsers, makeListUsersHandler(client))

	getUser := mcp.NewTool("kentik_get_user",
		mcp.WithDescription("Get information about a specific user by ID."),
		mcp.WithString("user_id",
			mcp.Required(),
			mcp.Description("The ID of the user"),
		),
	)
	s.AddTool(getUser, makeGetUserHandler(client))
}

func makeListUsersHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := client.V5("GET", "/users", nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list users: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(data)), nil
	}
}

func makeGetUserHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID, err := request.RequireString("user_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		data, err := client.V5("GET", fmt.Sprintf("/user/%s", userID), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get user: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(data)), nil
	}
}
