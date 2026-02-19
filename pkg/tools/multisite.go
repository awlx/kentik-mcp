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

func registerMultiSiteTools(s *server.MCPServer, client *kentik.Client) {
	compareSites := mcp.NewTool("kentik_compare_sites",
		mcp.WithDescription("Compare the same metric across multiple sites side-by-side. Runs the same query for each site and shows results in a comparison table. Useful for comparing traffic patterns, link utilization, or flow counts across different locations."),
		mcp.WithString("sites",
			mcp.Required(),
			mcp.Description("Comma-separated list of site names to compare. Each site's devices are auto-resolved."),
		),
		mcp.WithString("dimension",
			mcp.Required(),
			mcp.Description("Dimension to query. E.g. 'i_dst_connect_type_name', 'Port_dst', 'AS_dst'."),
		),
		mcp.WithString("metric",
			mcp.Description("Metric: 'bytes' (default) or 'fps'."),
		),
		mcp.WithNumber("lookback_seconds",
			mcp.Description("Time range. Default: 3600"),
		),
		mcp.WithNumber("topx",
			mcp.Description("Number of results per site. Default: 5"),
		),
		mcp.WithString("dst_connect_type",
			mcp.Description("Filter by destination connectivity type."),
		),
	)
	s.AddTool(compareSites, makeCompareSitesHandler(client))
}

func makeCompareSitesHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sitesStr, err := request.RequireString("sites")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		dimensionStr, err := request.RequireString("dimension")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		metric := "bytes"
		if m, err := request.RequireString("metric"); err == nil && m != "" {
			metric = m
		}
		lookback := 3600.0
		if lb, err := request.RequireFloat("lookback_seconds"); err == nil {
			lookback = lb
		}
		topx := 5.0
		if tx, err := request.RequireFloat("topx"); err == nil {
			topx = tx
		}

		outsort := "avg_bits_per_sec"
		if metric == "fps" {
			outsort = "avg_flows_per_sec"
		}

		sites := strings.Split(sitesStr, ",")
		for i := range sites {
			sites[i] = strings.TrimSpace(sites[i])
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("## Site Comparison: %s\n\n", strings.Join(sites, " vs ")))

		for _, site := range sites {
			if site == "" {
				continue
			}

			// Resolve devices for this site
			devNames, resolveErr := resolveDevicesBySite(client, site)
			if resolveErr != nil {
				sb.WriteString(fmt.Sprintf("### %s — Error: %v\n\n", site, resolveErr))
				continue
			}
			if len(devNames) == 0 {
				sb.WriteString(fmt.Sprintf("### %s — No active devices found\n\n", site))
				continue
			}

			query := map[string]interface{}{
				"metric":           metric,
				"dimension":        []string{dimensionStr},
				"topx":             int(topx),
				"depth":            int(topx * 2),
				"fastData":         "Auto",
				"outsort":          outsort,
				"lookback_seconds": int(lookback),
				"time_format":      "UTC",
				"hostname_lookup":  true,
				"device_name":      strings.Join(devNames, ","),
				"all_selected":     false,
			}

			// Apply filters
			filtersObj := buildFilters(request)
			if filtersObj != nil {
				query["filters_obj"] = filtersObj
			}

			body := map[string]interface{}{
				"queries": []map[string]interface{}{
					{"query": query, "bucket": "Left +Y Axis", "bucketIndex": 0, "isOverlay": false},
				},
			}

			data, queryErr := client.V5("POST", "/query/topXdata", body)
			if queryErr != nil {
				sb.WriteString(fmt.Sprintf("### %s — Query failed: %v\n\n", site, queryErr))
				continue
			}

			var resp struct {
				Results []struct {
					Data []map[string]interface{} `json:"data"`
				} `json:"results"`
			}
			if err := json.Unmarshal(data, &resp); err != nil || len(resp.Results) == 0 || len(resp.Results[0].Data) == 0 {
				sb.WriteString(fmt.Sprintf("### %s (%d devices) — No data\n\n", site, len(devNames)))
				continue
			}

			entries := resp.Results[0].Data
			sb.WriteString(fmt.Sprintf("### %s (%d devices)\n\n", site, len(devNames)))

			// Detect value column
			valKey := "avg_bits_per_sec"
			if metric == "fps" {
				valKey = "avg_flows_per_sec"
			}

			total := 0.0
			for _, e := range entries {
				if v, ok := e[valKey].(float64); ok {
					total += v
				}
			}

			sb.WriteString(fmt.Sprintf("| %-45s | %14s | %8s |\n", "Key", "Avg", "% Total"))
			sb.WriteString("|" + strings.Repeat("-", 47) + "|" + strings.Repeat("-", 16) + "|" + strings.Repeat("-", 10) + "|\n")

			for _, e := range entries {
				key := fmt.Sprintf("%v", e["key"])
				if len(key) > 45 {
					key = key[:42] + "..."
				}
				v, _ := e[valKey].(float64)
				pct := 0.0
				if total > 0 {
					pct = v / total * 100
				}
				sb.WriteString(fmt.Sprintf("| %-45s | %14s | %7.1f%% |\n",
					key, formatRate(v, metric), pct))
			}
			sb.WriteString(fmt.Sprintf("| %-45s | %14s | %8s |\n",
				"**Total**", formatRate(total, metric), "100%"))
			sb.WriteString("\n")
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}
