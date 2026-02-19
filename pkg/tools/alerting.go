package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/awlx/kentik-mcp/pkg/kentik"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerAlertingTools(s *server.MCPServer, client *kentik.Client) {
	// List active alerts
	listAlerts := mcp.NewTool("kentik_list_alerts",
		mcp.WithDescription("List active alerts and alarms from Kentik. Shows current anomalies, threshold violations, and DDoS detections across your network."),
		mcp.WithString("status",
			mcp.Description("Filter by alert status: 'alarm' (active), 'ackReq' (needs acknowledgement), or leave empty for all."),
		),
		mcp.WithNumber("lookback_minutes",
			mcp.Description("How far back to look for alerts. Default: 60 (last hour)"),
		),
	)
	s.AddTool(listAlerts, makeListAlertsHandler(client))
}

func makeListAlertsHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		lookbackMin := 60.0
		if lb, err := request.RequireFloat("lookback_minutes"); err == nil {
			lookbackMin = lb
		}

		// Use V5 alerting API to get active alarms
		path := fmt.Sprintf("/alerts-active/alarms?lookback_minutes=%d", int(lookbackMin))
		data, err := client.V5("GET", path, nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list alerts: %v", err)), nil
		}

		// Parse and summarize
		var alarms []map[string]interface{}
		if err := json.Unmarshal(data, &alarms); err != nil {
			// Try alternate structure
			var resp map[string]interface{}
			if err2 := json.Unmarshal(data, &resp); err2 != nil {
				return mcp.NewToolResultText(formatJSON(data)), nil
			}
			if a, ok := resp["alarms"].([]interface{}); ok {
				for _, item := range a {
					if m, ok := item.(map[string]interface{}); ok {
						alarms = append(alarms, m)
					}
				}
			} else {
				return mcp.NewToolResultText(formatJSON(data)), nil
			}
		}

		// Filter by status if specified
		statusFilter, _ := request.RequireString("status")
		if statusFilter != "" {
			statusFilter = strings.ToLower(statusFilter)
			var filtered []map[string]interface{}
			for _, a := range alarms {
				state := strings.ToLower(fmt.Sprintf("%v", a["alarm_state"]))
				if strings.Contains(state, statusFilter) {
					filtered = append(filtered, a)
				}
			}
			alarms = filtered
		}

		if len(alarms) == 0 {
			return mcp.NewToolResultText("No active alerts found."), nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("## Active Alerts (%d)\n\n", len(alarms)))
		sb.WriteString(fmt.Sprintf("| %-30s | %-15s | %-20s | %-30s |\n",
			"Policy", "State", "Severity", "Dimension"))
		sb.WriteString("|" + strings.Repeat("-", 32) + "|" + strings.Repeat("-", 17) +
			"|" + strings.Repeat("-", 22) + "|" + strings.Repeat("-", 32) + "|\n")

		for _, a := range alarms {
			policy := fmt.Sprintf("%v", a["alert_policy_name"])
			if policy == "<nil>" {
				policy = fmt.Sprintf("%v", a["alert_id"])
			}
			state := fmt.Sprintf("%v", a["alarm_state"])
			severity := fmt.Sprintf("%v", a["alert_severity"])
			dim := fmt.Sprintf("%v", a["alert_dimension"])

			if len(policy) > 30 {
				policy = policy[:27] + "..."
			}
			if len(dim) > 30 {
				dim = dim[:27] + "..."
			}

			sb.WriteString(fmt.Sprintf("| %-30s | %-15s | %-20s | %-30s |\n",
				policy, state, severity, dim))
		}

		sb.WriteString("\n<details><summary>Raw JSON</summary>\n\n```json\n")
		sb.WriteString(formatJSON(data))
		sb.WriteString("\n```\n</details>\n")

		return mcp.NewToolResultText(sb.String()), nil
	}
}
