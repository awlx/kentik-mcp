package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/awlx/kentik-mcp/pkg/kentik"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerSyntheticsTools(s *server.MCPServer, client *kentik.Client) {
	listTests := mcp.NewTool("kentik_list_synthetic_tests",
		mcp.WithDescription("List all configured synthetic tests in Kentik (active and paused). Returns test names, types, status, and configuration."),
	)
	s.AddTool(listTests, makeListSyntheticTestsHandler(client))

	getTest := mcp.NewTool("kentik_get_synthetic_test",
		mcp.WithDescription("Get detailed configuration and status for a specific synthetic test."),
		mcp.WithString("test_id",
			mcp.Required(),
			mcp.Description("The ID of the synthetic test"),
		),
	)
	s.AddTool(getTest, makeGetSyntheticTestHandler(client))

	getResults := mcp.NewTool("kentik_get_synthetic_results",
		mcp.WithDescription("Get probe results for one or more synthetic tests over a given time period. Returns health status, latency, packet loss, and other metrics."),
		mcp.WithString("test_ids",
			mcp.Required(),
			mcp.Description("Comma-separated list of synthetic test IDs"),
		),
		mcp.WithString("start_time",
			mcp.Required(),
			mcp.Description("Start time in RFC3339 format (e.g. 2025-01-01T00:00:00Z)"),
		),
		mcp.WithString("end_time",
			mcp.Required(),
			mcp.Description("End time in RFC3339 format (e.g. 2025-01-01T01:00:00Z)"),
		),
	)
	s.AddTool(getResults, makeGetSyntheticResultsHandler(client))

	listAgents := mcp.NewTool("kentik_list_synthetic_agents",
		mcp.WithDescription("List all synthetic monitoring agents available in the account (both global/public and private agents)."),
	)
	s.AddTool(listAgents, makeListSyntheticAgentsHandler(client))

	getAgent := mcp.NewTool("kentik_get_synthetic_agent",
		mcp.WithDescription("Get detailed information about a specific synthetic monitoring agent."),
		mcp.WithString("agent_id",
			mcp.Required(),
			mcp.Description("The ID of the synthetic agent"),
		),
	)
	s.AddTool(getAgent, makeGetSyntheticAgentHandler(client))

	getTrace := mcp.NewTool("kentik_get_synthetic_trace",
		mcp.WithDescription("Get network trace (traceroute) data for a specific synthetic test. The test must have traceroute task configured."),
		mcp.WithString("test_id",
			mcp.Required(),
			mcp.Description("The ID of the synthetic test"),
		),
		mcp.WithString("start_time",
			mcp.Required(),
			mcp.Description("Start time in RFC3339 format"),
		),
		mcp.WithString("end_time",
			mcp.Required(),
			mcp.Description("End time in RFC3339 format"),
		),
	)
	s.AddTool(getTrace, makeGetSyntheticTraceHandler(client))
}

func makeListSyntheticTestsHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := client.V6("GET", "/synthetics/v202309/tests", nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list synthetic tests: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(data)), nil
	}
}

func makeGetSyntheticTestHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		testID, err := request.RequireString("test_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		data, err := client.V6("GET", fmt.Sprintf("/synthetics/v202309/tests/%s", testID), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get synthetic test: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(data)), nil
	}
}

func makeGetSyntheticResultsHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		testIDsStr, err := request.RequireString("test_ids")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		startTime, err := request.RequireString("start_time")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		endTime, err := request.RequireString("end_time")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		testIDs := []string{}
		for _, id := range strings.Split(testIDsStr, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				testIDs = append(testIDs, id)
			}
		}

		body := map[string]interface{}{
			"testIds":   testIDs,
			"startTime": startTime,
			"endTime":   endTime,
		}

		data, err := client.V6("POST", "/synthetics/v202309/results", body)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get synthetic results: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(data)), nil
	}
}

func makeListSyntheticAgentsHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := client.V6("GET", "/synthetics/v202309/agents", nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list synthetic agents: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(data)), nil
	}
}

func makeGetSyntheticAgentHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		agentID, err := request.RequireString("agent_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		data, err := client.V6("GET", fmt.Sprintf("/synthetics/v202309/agents/%s", agentID), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get synthetic agent: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(data)), nil
	}
}

func makeGetSyntheticTraceHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		testID, err := request.RequireString("test_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		startTime, err := request.RequireString("start_time")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		endTime, err := request.RequireString("end_time")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		body := map[string]interface{}{
			"id":        testID,
			"startTime": startTime,
			"endTime":   endTime,
		}

		data, err := client.V6("POST", "/synthetics/v202309/trace", body)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get synthetic trace: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(data)), nil
	}
}
