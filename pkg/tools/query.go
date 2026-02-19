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
		mcp.WithDescription("Query Kentik network flow data (topX). Returns JSON results with traffic metrics grouped by dimensions. Includes a human-readable summary table. Use lookback_seconds for relative time or starting_time/ending_time for absolute ranges."),
		mcp.WithString("metric",
			mcp.Required(),
			mcp.Description("Unit of measure: bytes, in_bytes, out_bytes, packets, in_packets, out_packets, tcp_retransmit, fps, unique_src_ip, unique_dst_ip, client_latency, server_latency, appl_latency"),
		),
		mcp.WithString("dimension",
			mcp.Required(),
			mcp.Description("Group-by dimension(s), comma-separated. Common dimensions: "+
				"AS_src, AS_dst (source/dest ASN), "+
				"IP_src, IP_dst (source/dest IP), "+
				"Port_src, Port_dst (source/dest port), "+
				"Proto (protocol), "+
				"Geography_src, Geography_dst (country), "+
				"InterfaceID_src, InterfaceID_dst (interface), "+
				"i_device_id, i_device_site_name (device/site), "+
				"i_src_connect_type_name, i_dst_connect_type_name (connectivity type: backbone, free_pni, transit, ix), "+
				"TopFlow, Traffic"),
		),
		mcp.WithNumber("lookback_seconds",
			mcp.Description("Look-back time in seconds (e.g. 3600 for last hour, 86400 for last day). Overrides starting_time/ending_time unless set to 0. Default: 3600"),
		),
		mcp.WithString("starting_time",
			mcp.Description("Fixed start time in 'YYYY-MM-DD HH:mm:00' format. Only used when lookback_seconds is 0."),
		),
		mcp.WithString("ending_time",
			mcp.Description("Fixed end time in 'YYYY-MM-DD HH:mm:00' format. Only used when lookback_seconds is 0."),
		),
		mcp.WithString("device_name",
			mcp.Description("Comma-delimited list of device names to query. Ignored if all_selected is true. Use kentik_search_devices to find device names."),
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
			mcp.Description("Optional raw JSON for filters_obj. Use this for complex filters. Format: {\"connector\":\"All\",\"filterGroups\":[{\"connector\":\"All\",\"filters\":[{\"filterField\":\"dst_as\",\"operator\":\"=\",\"filterValue\":\"15169\"}],\"not\":false}]}"),
		),
		mcp.WithString("src_connect_type",
			mcp.Description("Convenience filter: source connectivity type. Values: backbone, free_pni, transit, ix. Comma-separated for multiple (OR)."),
		),
		mcp.WithString("dst_connect_type",
			mcp.Description("Convenience filter: destination connectivity type. Values: backbone, free_pni, transit, ix. Comma-separated for multiple (OR)."),
		),
		mcp.WithString("src_ip",
			mcp.Description("Convenience filter: source IP address (exact match or CIDR). E.g. '10.0.0.1' or '140.82.112.0/24'."),
		),
		mcp.WithString("dst_ip",
			mcp.Description("Convenience filter: destination IP address (exact match or CIDR)."),
		),
		mcp.WithString("port",
			mcp.Description("Convenience filter: destination port number. E.g. '443' or '22'."),
		),
		mcp.WithString("protocol",
			mcp.Description("Convenience filter: IP protocol number. E.g. '6' for TCP, '17' for UDP."),
		),
		mcp.WithString("src_as",
			mcp.Description("Convenience filter: source AS number. E.g. '15169' for Google."),
		),
		mcp.WithString("dst_as",
			mcp.Description("Convenience filter: destination AS number."),
		),
		mcp.WithString("site_name",
			mcp.Description("Convenience shortcut: auto-resolve devices by site name (e.g. 'NYC-DC1'). Searches for active devices at this site and uses them. Overrides device_name."),
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

	// Build filters from both raw JSON and convenience params
	filtersObj := buildFilters(request)
	if filtersObj != nil {
		query["filters_obj"] = filtersObj
	}

	return query, nil
}

// buildFilters merges raw filters_json with convenience filter parameters.
func buildFilters(request mcp.CallToolRequest) map[string]interface{} {
	var filterGroups []map[string]interface{}

	// Parse raw filters_json first
	if filtersJSON, err := request.RequireString("filters_json"); err == nil && filtersJSON != "" {
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(filtersJSON), &raw); err == nil {
			if groups, ok := raw["filterGroups"].([]interface{}); ok {
				for _, g := range groups {
					if gm, ok := g.(map[string]interface{}); ok {
						filterGroups = append(filterGroups, gm)
					}
				}
			}
		}
	}

	// Convenience filters: each becomes a filter group
	convenienceFilters := []struct {
		param string
		field string
	}{
		{"src_connect_type", "i_src_connect_type_name"},
		{"dst_connect_type", "i_dst_connect_type_name"},
		{"src_ip", "inet_src_addr"},
		{"dst_ip", "inet_dst_addr"},
		{"port", "l4_dst_port"},
		{"protocol", "protocol"},
		{"src_as", "src_as"},
		{"dst_as", "dst_as"},
	}

	for _, cf := range convenienceFilters {
		val, err := request.RequireString(cf.param)
		if err != nil || val == "" {
			continue
		}

		// Support comma-separated values as OR
		values := strings.Split(val, ",")
		var filters []map[string]interface{}
		for _, v := range values {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			op := "="
			// Use CIDR matching for IP filters with /
			if strings.Contains(v, "/") && (cf.field == "inet_src_addr" || cf.field == "inet_dst_addr") {
				op = "ILIKE"
			}
			filters = append(filters, map[string]interface{}{
				"filterField": cf.field,
				"operator":    op,
				"filterValue": v,
			})
		}

		if len(filters) > 0 {
			connector := "All"
			if len(filters) > 1 {
				connector = "Any"
			}
			filterGroups = append(filterGroups, map[string]interface{}{
				"connector": connector,
				"filters":   filters,
				"not":       false,
			})
		}
	}

	if len(filterGroups) == 0 {
		return nil
	}

	return map[string]interface{}{
		"connector":    "All",
		"filterGroups": filterGroups,
	}
}

func makeQueryDataHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Auto-resolve site_name to device names
		var siteDeviceNames string
		if siteName, err := request.RequireString("site_name"); err == nil && siteName != "" {
			devNames, resolveErr := resolveDevicesBySite(client, siteName)
			if resolveErr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to resolve site '%s': %v", siteName, resolveErr)), nil
			}
			if len(devNames) == 0 {
				return mcp.NewToolResultError(fmt.Sprintf("No active devices found at site matching '%s'", siteName)), nil
			}
			siteDeviceNames = strings.Join(devNames, ",")
		}

		query, err := buildQueryObject(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Override device_name if site_name was used
		if siteDeviceNames != "" {
			query["device_name"] = siteDeviceNames
			query["all_selected"] = false
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

		summary := summarizeQueryResults(data, query)
		return mcp.NewToolResultText(summary), nil
	}
}

// resolveDevicesBySite fetches all devices and returns names matching the site.
func resolveDevicesBySite(client *kentik.Client, siteName string) ([]string, error) {
	data, err := client.V5("GET", "/devices", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Devices []struct {
			Name   string `json:"device_name"`
			Status string `json:"device_status"`
			Site   struct {
				Name string `json:"site_name"`
			} `json:"site"`
		} `json:"devices"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	var names []string
	siteNameLower := strings.ToLower(siteName)
	for _, d := range resp.Devices {
		if d.Status == "V" && strings.Contains(strings.ToLower(d.Site.Name), siteNameLower) {
			names = append(names, d.Name)
		}
	}
	return names, nil
}

// summarizeQueryResults produces a human-readable summary table from query results.
func summarizeQueryResults(data json.RawMessage, query map[string]interface{}) string {
	var resp struct {
		Results []struct {
			Bucket string                   `json:"bucket"`
			Data   []map[string]interface{} `json:"data"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return formatJSON(data)
	}

	if len(resp.Results) == 0 || len(resp.Results[0].Data) == 0 {
		return "No results returned.\n\n" + formatJSON(data)
	}

	metric, _ := query["metric"].(string)
	entries := resp.Results[0].Data

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Query Results (%d rows)\n\n", len(entries)))

	// Select columns based on the metric to avoid picking wrong ones
	type colDef struct {
		key    string
		header string
	}

	var preferredCols []colDef
	switch {
	case metric == "fps":
		preferredCols = []colDef{
			{"avg_flows_per_sec", "Avg FPS"},
			{"p95th_flows_per_sec", "P95 FPS"},
			{"max_flows_per_sec", "Max FPS"},
		}
	case strings.Contains(metric, "packets"):
		preferredCols = []colDef{
			{"avg_pkts_per_sec", "Avg PPS"},
			{"p95th_pkts_per_sec", "P95 PPS"},
			{"max_pkts_per_sec", "Max PPS"},
		}
	case metric == "unique_src_ip" || metric == "unique_dst_ip":
		preferredCols = []colDef{
			{"max_ips", "Max IPs"},
		}
	default: // bytes and variants
		preferredCols = []colDef{
			{"avg_bits_per_sec", "Avg bps"},
			{"p95th_bits_per_sec", "P95 bps"},
			{"max_bits_per_sec", "Max bps"},
		}
	}

	// Only include columns that exist in the data
	var activeCols []colDef
	for _, col := range preferredCols {
		if _, ok := entries[0][col.key]; ok {
			activeCols = append(activeCols, col)
		}
	}
	// Fallback: if none of preferred cols exist, pick any 3 that do
	if len(activeCols) == 0 {
		allCols := []colDef{
			{"avg_bits_per_sec", "Avg bps"}, {"p95th_bits_per_sec", "P95 bps"}, {"max_bits_per_sec", "Max bps"},
			{"avg_pkts_per_sec", "Avg PPS"}, {"avg_flows_per_sec", "Avg FPS"}, {"max_ips", "Max IPs"},
		}
		for _, col := range allCols {
			if _, ok := entries[0][col.key]; ok {
				activeCols = append(activeCols, col)
				if len(activeCols) >= 3 {
					break
				}
			}
		}
	}

	// The first active column is used for percentages
	sortCol := ""
	if len(activeCols) > 0 {
		sortCol = activeCols[0].key
	}

	// Calculate totals
	totals := make(map[string]float64)
	for _, entry := range entries {
		for _, col := range activeCols {
			if v, ok := entry[col.key].(float64); ok {
				totals[col.key] += v
			}
		}
	}

	// Header
	sb.WriteString(fmt.Sprintf("| %-55s", "Key"))
	for _, col := range activeCols {
		sb.WriteString(fmt.Sprintf(" | %14s", col.header))
	}
	sb.WriteString(" | % Total |\n")
	sb.WriteString("|" + strings.Repeat("-", 56))
	for range activeCols {
		sb.WriteString("|" + strings.Repeat("-", 16))
	}
	sb.WriteString("|---------|\n")

	// Rows
	for _, entry := range entries {
		key := fmt.Sprintf("%v", entry["key"])
		if len(key) > 55 {
			key = key[:52] + "..."
		}
		sb.WriteString(fmt.Sprintf("| %-55s", key))
		for _, col := range activeCols {
			v, _ := entry[col.key].(float64)
			sb.WriteString(fmt.Sprintf(" | %14s", formatRate(v, metric)))
		}
		// Percentage based on first column
		if sortCol != "" && totals[sortCol] > 0 {
			v, _ := entry[sortCol].(float64)
			pct := v / totals[sortCol] * 100
			sb.WriteString(fmt.Sprintf(" | %6.2f%% |", pct))
		} else {
			sb.WriteString(" |         |")
		}
		sb.WriteString("\n")
	}

	// Total row
	sb.WriteString(fmt.Sprintf("| %-55s", "**TOTAL**"))
	for _, col := range activeCols {
		sb.WriteString(fmt.Sprintf(" | %14s", formatRate(totals[col.key], metric)))
	}
	sb.WriteString(" |  100.0% |\n\n")

	// Raw JSON in collapsible
	sb.WriteString("<details><summary>Raw JSON</summary>\n\n```json\n")
	sb.WriteString(formatJSON(data))
	sb.WriteString("\n```\n</details>\n")

	return sb.String()
}

// formatRate formats a numeric rate value with appropriate units.
func formatRate(v float64, metric string) string {
	switch {
	case strings.Contains(metric, "bytes") || metric == "bytes":
		return formatBitsPerSec(v)
	default:
		if v >= 1e6 {
			return fmt.Sprintf("%.2fM", v/1e6)
		}
		if v >= 1e3 {
			return fmt.Sprintf("%.2fK", v/1e3)
		}
		return fmt.Sprintf("%.2f", v)
	}
}

func formatBitsPerSec(bps float64) string {
	if bps >= 1e12 {
		return fmt.Sprintf("%.2f Tbps", bps/1e12)
	}
	if bps >= 1e9 {
		return fmt.Sprintf("%.2f Gbps", bps/1e9)
	}
	if bps >= 1e6 {
		return fmt.Sprintf("%.2f Mbps", bps/1e6)
	}
	if bps >= 1e3 {
		return fmt.Sprintf("%.2f Kbps", bps/1e3)
	}
	return fmt.Sprintf("%.2f bps", bps)
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
