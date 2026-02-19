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

func registerSNMPTools(s *server.MCPServer, client *kentik.Client) {
	// Query interface utilization by SNMP counters
	queryInterfaceTraffic := mcp.NewTool("kentik_get_interface_counters",
		mcp.WithDescription("Query per-interface bandwidth utilization for specific devices. Uses flow data aggregated by interface to show per-link throughput. Useful for peering link utilization, transit capacity, and identifying hot interfaces. Filter by interface description to find specific link types."),
		mcp.WithString("device_name",
			mcp.Description("Comma-delimited list of device names to query."),
		),
		mcp.WithString("site_name",
			mcp.Description("Auto-resolve devices by site name. Overrides device_name."),
		),
		mcp.WithString("device_label",
			mcp.Description("Auto-resolve devices by label (e.g. 'border'). Overrides device_name."),
		),
		mcp.WithString("interface_description_filter",
			mcp.Description("Filter interfaces by description substring (case-insensitive). E.g. 'pni', 'transit', 'uplink', 'core'."),
		),
		mcp.WithNumber("lookback_seconds",
			mcp.Description("Look-back time in seconds. Default: 3600"),
		),
		mcp.WithNumber("topx",
			mcp.Description("Number of top interfaces to return. Default: 20"),
		),
		mcp.WithString("direction",
			mcp.Description("Traffic direction: 'out' (egress), 'in' (ingress), or 'both'. Default: both"),
		),
	)
	s.AddTool(queryInterfaceTraffic, makeQueryInterfaceTrafficHandler(client))
}

func makeQueryInterfaceTrafficHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		resolvedDevices := resolveDeviceShortcuts(client, request)

		lookback := 3600.0
		if lb, err := request.RequireFloat("lookback_seconds"); err == nil {
			lookback = lb
		}
		topx := 20.0
		if tx, err := request.RequireFloat("topx"); err == nil {
			topx = tx
		}

		direction := "both"
		if dir, err := request.RequireString("direction"); err == nil && dir != "" {
			direction = strings.ToLower(dir)
		}

		ifDescFilter, _ := request.RequireString("interface_description_filter")

		// Build queries for egress and/or ingress
		type queryResult struct {
			label string
			data  json.RawMessage
			err   error
		}

		var results []queryResult

		buildQuery := func(dimension string) map[string]interface{} {
			// Use large topx/depth when filtering by description to ensure we
			// capture enough interfaces before post-filtering
			queryTopx := int(topx)
			queryDepth := int(topx * 2)
			if ifDescFilter != "" {
				queryTopx = 250
				queryDepth = 250
			}

			q := map[string]interface{}{
				"metric":           "bytes",
				"dimension":        []string{dimension},
				"topx":             queryTopx,
				"depth":            queryDepth,
				"fastData":         "Auto",
				"outsort":          "avg_bits_per_sec",
				"lookback_seconds": int(lookback),
				"time_format":      "UTC",
				"hostname_lookup":  true,
				"all_selected":     true,
			}
			if resolvedDevices != "" {
				q["device_name"] = resolvedDevices
				q["all_selected"] = false
			} else if dn, err := request.RequireString("device_name"); err == nil && dn != "" {
				q["device_name"] = dn
				q["all_selected"] = false
			}
			return q
		}

		if direction == "out" || direction == "both" {
			q := buildQuery("InterfaceID_src")
			body := map[string]interface{}{
				"queries": []map[string]interface{}{
					{"query": q, "bucket": "Left +Y Axis", "bucketIndex": 0, "isOverlay": false},
				},
			}
			data, err := client.V5("POST", "/query/topXdata", body)
			results = append(results, queryResult{"Egress (out)", data, err})
		}

		if direction == "in" || direction == "both" {
			q := buildQuery("InterfaceID_dst")
			body := map[string]interface{}{
				"queries": []map[string]interface{}{
					{"query": q, "bucket": "Left +Y Axis", "bucketIndex": 0, "isOverlay": false},
				},
			}
			data, err := client.V5("POST", "/query/topXdata", body)
			results = append(results, queryResult{"Ingress (in)", data, err})
		}

		// Format results
		var sb strings.Builder
		filterLower := strings.ToLower(ifDescFilter)

		for _, r := range results {
			if r.err != nil {
				sb.WriteString(fmt.Sprintf("## %s — Error: %v\n\n", r.label, r.err))
				continue
			}

			var resp struct {
				Results []struct {
					Data []map[string]interface{} `json:"data"`
				} `json:"results"`
			}
			if err := json.Unmarshal(r.data, &resp); err != nil || len(resp.Results) == 0 {
				sb.WriteString(fmt.Sprintf("## %s — No data\n\n", r.label))
				continue
			}

			entries := resp.Results[0].Data

			// Filter by interface description if specified
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

			sb.WriteString(fmt.Sprintf("## %s (%d interfaces)\n\n", r.label, len(entries)))
			sb.WriteString(fmt.Sprintf("| %-70s | %14s | %14s | %14s |\n", "Interface", "Avg", "P95", "Max"))
			sb.WriteString("|" + strings.Repeat("-", 72) + "|" + strings.Repeat("-", 16) + "|" + strings.Repeat("-", 16) + "|" + strings.Repeat("-", 16) + "|\n")

			avgKey := "avg_bits_per_sec"
			p95Key := "p95th_bits_per_sec"
			maxKey := "max_bits_per_sec"

			for _, e := range entries {
				key := fmt.Sprintf("%v", e["key"])
				if len(key) > 70 {
					key = key[:67] + "..."
				}
				avg, _ := e[avgKey].(float64)
				p95, _ := e[p95Key].(float64)
				max, _ := e[maxKey].(float64)
				sb.WriteString(fmt.Sprintf("| %-70s | %14s | %14s | %14s |\n",
					key, formatBitsPerSec(avg), formatBitsPerSec(p95), formatBitsPerSec(max)))
			}
			sb.WriteString("\n")
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}
