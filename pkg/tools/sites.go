package tools

import (
	"context"
	"fmt"

	"github.com/awlx/kentik-mcp/pkg/kentik"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerSiteTools(s *server.MCPServer, client *kentik.Client) {
	listSites := mcp.NewTool("kentik_list_sites",
		mcp.WithDescription("List all sites in Kentik. Sites are groups of devices based on geographic location."),
	)
	s.AddTool(listSites, makeListSitesHandler(client))

	getSite := mcp.NewTool("kentik_get_site",
		mcp.WithDescription("Get detailed information about a specific site by ID."),
		mcp.WithString("site_id",
			mcp.Required(),
			mcp.Description("The ID of the site"),
		),
	)
	s.AddTool(getSite, makeGetSiteHandler(client))
}

func makeListSitesHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := client.V5("GET", "/sites", nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list sites: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(data)), nil
	}
}

func makeGetSiteHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		siteID, err := request.RequireString("site_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		data, err := client.V5("GET", fmt.Sprintf("/site/%s", siteID), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get site: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(data)), nil
	}
}
