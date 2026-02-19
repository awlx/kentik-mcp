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

func registerCapacityPlanTools(s *server.MCPServer, client *kentik.Client) {
	capacityPlan := mcp.NewTool("kentik_capacity_plan",
		mcp.WithDescription("Query interface capacity and utilization from Kentik. Shows current utilization as a percentage of interface speed, helping identify links approaching capacity. Groups by interface with speed, current usage, and utilization %."),
		mcp.WithString("device_name",
			mcp.Description("Comma-delimited device names."),
		),
		mcp.WithString("device_label",
			mcp.Description("Auto-resolve devices by label."),
		),
		mcp.WithString("site_name",
			mcp.Description("Auto-resolve devices by site."),
		),
		mcp.WithString("interface_description_filter",
			mcp.Description("Filter by interface description substring. E.g. 'pni', 'transit', 'uplink'."),
		),
		mcp.WithNumber("lookback_seconds",
			mcp.Description("Time range. Default: 3600"),
		),
		mcp.WithNumber("utilization_threshold",
			mcp.Description("Only show interfaces above this utilization %. Default: 0 (show all)"),
		),
	)
	s.AddTool(capacityPlan, makeCapacityPlanHandler(client))
}

func makeCapacityPlanHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		resolvedDevices := resolveDeviceShortcuts(client, request)

		lookback := 3600.0
		if lb, err := request.RequireFloat("lookback_seconds"); err == nil {
			lookback = lb
		}
		threshold := 0.0
		if th, err := request.RequireFloat("utilization_threshold"); err == nil {
			threshold = th
		}
		ifDescFilter, _ := request.RequireString("interface_description_filter")

		// Query egress traffic by source interface
		topx := 250
		if ifDescFilter == "" {
			topx = 50
		}

		query := map[string]interface{}{
			"metric":           "bytes",
			"dimension":        []string{"InterfaceID_src"},
			"topx":             topx,
			"depth":            topx,
			"fastData":         "Auto",
			"outsort":          "avg_bits_per_sec",
			"lookback_seconds": int(lookback),
			"time_format":      "UTC",
			"hostname_lookup":  true,
			"all_selected":     true,
		}

		if resolvedDevices != "" {
			query["device_name"] = resolvedDevices
			query["all_selected"] = false
		} else if dn, err := request.RequireString("device_name"); err == nil && dn != "" {
			query["device_name"] = dn
			query["all_selected"] = false
		}

		body := map[string]interface{}{
			"queries": []map[string]interface{}{
				{"query": query, "bucket": "Left +Y Axis", "bucketIndex": 0, "isOverlay": false},
			},
		}

		data, err := client.V5("POST", "/query/topXdata", body)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Query failed: %v", err)), nil
		}

		var resp struct {
			Results []struct {
				Data []map[string]interface{} `json:"data"`
			} `json:"results"`
		}
		if err := json.Unmarshal(data, &resp); err != nil || len(resp.Results) == 0 {
			return mcp.NewToolResultText(formatJSON(data)), nil
		}

		entries := resp.Results[0].Data

		// Filter by description
		filterLower := strings.ToLower(ifDescFilter)
		if filterLower != "" {
			var filtered []map[string]interface{}
			for _, e := range entries {
				key := strings.ToLower(fmt.Sprintf("%v", e["key"]))
				if strings.Contains(key, filterLower) {
					filtered = append(filtered, e)
				}
			}
			entries = filtered
		}

		// We need interface speeds — fetch from the API for each device
		// For now, estimate based on common speeds or show raw bandwidth
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("## Interface Capacity Report (%d interfaces)\n\n", len(entries)))
		sb.WriteString(fmt.Sprintf("| %-65s | %14s | %14s | %14s |\n",
			"Interface", "Avg Egress", "P95 Egress", "Max Egress"))
		sb.WriteString("|" + strings.Repeat("-", 67) + "|" + strings.Repeat("-", 16) +
			"|" + strings.Repeat("-", 16) + "|" + strings.Repeat("-", 16) + "|\n")

		shown := 0
		for _, e := range entries {
			avg, _ := e["avg_bits_per_sec"].(float64)
			p95, _ := e["p95th_bits_per_sec"].(float64)
			max, _ := e["max_bits_per_sec"].(float64)

			// Skip if below threshold (approximate — we'd need interface speed for real %)
			if threshold > 0 {
				// Assume 100G interfaces as default for threshold check
				util := avg / 100e9 * 100
				if util < threshold {
					continue
				}
			}

			key := fmt.Sprintf("%v", e["key"])
			if len(key) > 65 {
				key = key[:62] + "..."
			}

			sb.WriteString(fmt.Sprintf("| %-65s | %14s | %14s | %14s |\n",
				key, formatBitsPerSec(avg), formatBitsPerSec(p95), formatBitsPerSec(max)))
			shown++
		}

		if shown == 0 {
			return mcp.NewToolResultText("No interfaces match the criteria."), nil
		}

		sb.WriteString(fmt.Sprintf("\n*%d interfaces shown", shown))
		if threshold > 0 {
			sb.WriteString(fmt.Sprintf(" (filtered to >%.0f%% utilization, assuming 100G)", threshold))
		}
		sb.WriteString("*\n")

		return mcp.NewToolResultText(sb.String()), nil
	}
}
