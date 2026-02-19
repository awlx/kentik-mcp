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

func registerQueryTools(s *server.MCPServer, client *kentik.Client) {
	queryData := mcp.NewTool("kentik_query_data",
		mcp.WithDescription("Query Kentik network flow data (topX). Returns JSON results with traffic metrics like bits/sec, packets/sec grouped by dimensions such as source/dest IP, AS, geography, etc. Use lookback_seconds for relative time or starting_time/ending_time for absolute ranges."),
		mcp.WithString("metric",
			mcp.Required(),
			mcp.Description("Unit of measure: bytes, in_bytes, out_bytes, packets, in_packets, out_packets, tcp_retransmit, fps, unique_src_ip, unique_dst_ip, client_latency, server_latency, appl_latency"),
		),
		mcp.WithString("dimension",
			mcp.Required(),
			mcp.Description("Group-by dimension(s), comma-separated. Examples: AS_src, AS_dst, Geography_src, Geography_dst, IP_src, IP_dst, Port_src, Port_dst, Proto, Traffic, InterfaceID_src, InterfaceID_dst, TopFlow, i_device_id, i_device_site_name"),
		),
		mcp.WithNumber("lookback_seconds",
			mcp.Description("Look-back time in seconds (e.g. 3600 for last hour). Overrides starting_time/ending_time unless set to 0. Default: 3600"),
		),
		mcp.WithString("starting_time",
			mcp.Description("Fixed start time in 'YYYY-MM-DD HH:mm:00' format. Only used when lookback_seconds is 0."),
		),
		mcp.WithString("ending_time",
			mcp.Description("Fixed end time in 'YYYY-MM-DD HH:mm:00' format. Only used when lookback_seconds is 0."),
		),
		mcp.WithString("device_name",
			mcp.Description("Comma-delimited list of device names to query. Ignored if all_selected is true."),
		),
		mcp.WithBoolean("all_selected",
			mcp.Description("Query against all devices. Default: true"),
		),
		mcp.WithNumber("topx",
			mcp.Description("Number of top results to return (1-40). Default: 8"),
		),
		mcp.WithNumber("depth",
			mcp.Description("Pool size from which topX is determined (25-250). Default: 100"),
		),
		mcp.WithString("outsort",
			mcp.Description("Aggregate to sort results by. E.g. avg_bits_per_sec, p95th_bits_per_sec, max_bits_per_sec. Defaults based on metric."),
		),
		mcp.WithString("filters_json",
			mcp.Description("Optional JSON string for filters_obj. Format: {\"connector\":\"All\",\"filterGroups\":[{\"connector\":\"All\",\"filters\":[{\"filterField\":\"dst_as\",\"operator\":\"=\",\"filterValue\":\"15169\"}],\"not\":false}]}"),
		),
		mcp.WithString("fast_data",
			mcp.Description("Dataset selection: Auto, Fast, or Full. Default: Auto"),
		),
	)
	s.AddTool(queryData, makeQueryDataHandler(client))

	queryURL := mcp.NewTool("kentik_query_url",
		mcp.WithDescription("Generate a Kentik portal URL with Data Explorer configured for the given query parameters. Returns a URL that opens directly in the Kentik portal."),
		mcp.WithString("metric",
			mcp.Required(),
			mcp.Description("Unit of measure: bytes, packets, etc."),
		),
		mcp.WithString("dimension",
			mcp.Required(),
			mcp.Description("Group-by dimension(s), comma-separated"),
		),
		mcp.WithNumber("lookback_seconds",
			mcp.Description("Look-back time in seconds. Default: 3600"),
		),
		mcp.WithString("device_name",
			mcp.Description("Comma-delimited list of device names to query"),
		),
		mcp.WithBoolean("all_selected",
			mcp.Description("Query against all devices. Default: true"),
		),
	)
	s.AddTool(queryURL, makeQueryURLHandler(client))
}

func buildQueryObject(request mcp.CallToolRequest) (map[string]interface{}, error) {
	metric, err := request.RequireString("metric")
	if err != nil {
		return nil, err
	}
	dimensionStr, err := request.RequireString("dimension")
	if err != nil {
		return nil, err
	}

	dimensions := []string{}
	for _, d := range strings.Split(dimensionStr, ",") {
		d = strings.TrimSpace(d)
		if d != "" {
			dimensions = append(dimensions, d)
		}
	}

	lookback := 3600.0
	if lb, err := request.RequireFloat("lookback_seconds"); err == nil {
		lookback = lb
	}
	topx := 8.0
	if tx, err := request.RequireFloat("topx"); err == nil {
		topx = tx
	}
	depth := 100.0
	if dp, err := request.RequireFloat("depth"); err == nil {
		depth = dp
	}
	allSelected := true
	if val, err := request.RequireString("all_selected"); err == nil && val == "false" {
		allSelected = false
	}
	fastData := "Auto"
	if fd, err := request.RequireString("fast_data"); err == nil && fd != "" {
		fastData = fd
	}
	outsort := ""
	if o, err := request.RequireString("outsort"); err == nil && o != "" {
		outsort = o
	}

	if outsort == "" {
		switch metric {
		case "bytes", "in_bytes", "out_bytes":
			outsort = "avg_bits_per_sec"
		case "packets", "in_packets", "out_packets":
			outsort = "avg_pkts_per_sec"
		case "fps":
			outsort = "avg_flows_per_sec"
		case "unique_src_ip", "unique_dst_ip":
			outsort = "max_ips"
		default:
			outsort = "avg_bits_per_sec"
		}
	}

	query := map[string]interface{}{
		"metric":           metric,
		"dimension":        dimensions,
		"topx":             int(topx),
		"depth":            int(depth),
		"fastData":         fastData,
		"outsort":          outsort,
		"lookback_seconds": int(lookback),
		"time_format":      "UTC",
		"hostname_lookup":  true,
		"all_selected":     allSelected,
	}

	if deviceName, err := request.RequireString("device_name"); err == nil && deviceName != "" {
		query["device_name"] = deviceName
		query["all_selected"] = false
	}

	if startTime, err := request.RequireString("starting_time"); err == nil && startTime != "" {
		query["starting_time"] = startTime
		query["lookback_seconds"] = 0
	}
	if endTime, err := request.RequireString("ending_time"); err == nil && endTime != "" {
		query["ending_time"] = endTime
	}

	if filtersJSON, err := request.RequireString("filters_json"); err == nil && filtersJSON != "" {
		var filtersObj map[string]interface{}
		if err := json.Unmarshal([]byte(filtersJSON), &filtersObj); err == nil {
			query["filters_obj"] = filtersObj
		}
	}

	return query, nil
}

func makeQueryDataHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := buildQueryObject(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		body := map[string]interface{}{
			"queries": []map[string]interface{}{
				{
					"query":       query,
					"bucket":      "Left +Y Axis",
					"bucketIndex": 0,
					"isOverlay":   false,
				},
			},
		}

		data, err := client.V5("POST", "/query/topXdata", body)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to query data: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(data)), nil
	}
}

func makeQueryURLHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := buildQueryObject(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		query["viz_type"] = "stackedArea"

		body := map[string]interface{}{
			"queries": []map[string]interface{}{
				{
					"query":       query,
					"bucket":      "Left +Y Axis",
					"bucketIndex": 0,
					"isOverlay":   false,
				},
			},
		}

		data, err := client.V5("POST", "/query/url", body)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get query URL: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
