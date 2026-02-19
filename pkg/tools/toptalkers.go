package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/awlx/kentik-mcp/pkg/kentik"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTopTalkersTools(s *server.MCPServer, client *kentik.Client) {
	topTalkers := mcp.NewTool("kentik_query_toptalkers",
		mcp.WithDescription("Quick query: find the top talkers (IPs, ASNs, or ports) by traffic volume or flow count. Simplified interface — just specify what you want to rank and the time range. Returns a formatted table with bandwidth and percentage."),
		mcp.WithString("rank_by",
			mcp.Required(),
			mcp.Description("What to rank: 'src_ip', 'dst_ip', 'src_asn', 'dst_asn', 'src_port', 'dst_port', 'protocol', 'src_country', 'dst_country', 'interface'"),
		),
		mcp.WithString("metric",
			mcp.Description("Measure by: 'volume' (bytes, default) or 'flows' (fps)"),
		),
		mcp.WithNumber("lookback_seconds",
			mcp.Description("Time range in seconds. Default: 3600 (1 hour)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Number of results. Default: 10"),
		),
		mcp.WithString("device_name",
			mcp.Description("Comma-delimited device names to query."),
		),
		mcp.WithString("device_label",
			mcp.Description("Auto-resolve devices by label."),
		),
		mcp.WithString("site_name",
			mcp.Description("Auto-resolve devices by site."),
		),
		mcp.WithString("dst_connect_type",
			mcp.Description("Filter by destination connectivity type. E.g. 'free_pni,transit,ix' for external."),
		),
		mcp.WithString("port",
			mcp.Description("Filter by destination port."),
		),
	)
	s.AddTool(topTalkers, makeTopTalkersHandler(client))
}

func makeTopTalkersHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rankBy, err := request.RequireString("rank_by")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Map friendly names to Kentik dimensions
		dimMap := map[string]string{
			"src_ip":      "IP_src",
			"dst_ip":      "IP_dst",
			"src_asn":     "AS_src",
			"dst_asn":     "AS_dst",
			"src_port":    "Port_src",
			"dst_port":    "Port_dst",
			"protocol":    "Proto",
			"src_country": "Geography_src",
			"dst_country": "Geography_dst",
			"interface":   "InterfaceID_src",
		}
		dimension, ok := dimMap[strings.ToLower(rankBy)]
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("Unknown rank_by '%s'. Valid: %s",
				rankBy, "src_ip, dst_ip, src_asn, dst_asn, src_port, dst_port, protocol, src_country, dst_country, interface")), nil
		}

		metricStr := "bytes"
		if m, err := request.RequireString("metric"); err == nil && strings.ToLower(m) == "flows" {
			metricStr = "fps"
		}

		lookback := 3600.0
		if lb, err := request.RequireFloat("lookback_seconds"); err == nil {
			lookback = lb
		}
		limit := 10.0
		if lm, err := request.RequireFloat("limit"); err == nil {
			limit = lm
		}

		resolvedDevices := resolveDeviceShortcuts(client, request)

		outsort := "avg_bits_per_sec"
		if metricStr == "fps" {
			outsort = "avg_flows_per_sec"
		}

		query := map[string]interface{}{
			"metric":           metricStr,
			"dimension":        []string{dimension},
			"topx":             int(limit),
			"depth":            int(limit * 2),
			"fastData":         "Auto",
			"outsort":          outsort,
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

		filtersObj := buildFilters(request)
		if filtersObj != nil {
			query["filters_obj"] = filtersObj
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

		summary := summarizeQueryResults(data, query)
		return mcp.NewToolResultText(fmt.Sprintf("## Top Talkers by %s (%s)\n\n%s", rankBy, metricStr, summary)), nil
	}
}
