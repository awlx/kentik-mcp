package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerDimensionTools(s *server.MCPServer) {
	listDimensions := mcp.NewTool("kentik_list_dimensions",
		mcp.WithDescription("List all available Kentik query dimensions with descriptions. Use this to find the correct dimension name for kentik_query_data or kentik_query_compare."),
		mcp.WithString("search",
			mcp.Description("Search term to filter dimensions (case-insensitive). E.g. 'ip', 'as', 'port', 'interface', 'geo', 'connect'."),
		),
	)
	s.AddTool(listDimensions, makeListDimensionsHandler())
}

func makeListDimensionsHandler() server.ToolHandlerFunc {
	type dim struct {
		name string
		desc string
	}
	dimensions := []dim{
		// Network layer
		{"IP_src", "Source IP address"},
		{"IP_dst", "Destination IP address"},
		{"Port_src", "Source L4 port"},
		{"Port_dst", "Destination L4 port"},
		{"Proto", "IP protocol number (6=TCP, 17=UDP, 1=ICMP)"},
		{"VLAN_src", "Source VLAN ID"},
		{"VLAN_dst", "Destination VLAN ID"},
		{"src_eth_mac", "Source MAC address"},
		{"dst_eth_mac", "Destination MAC address"},

		// ASN / BGP
		{"AS_src", "Source autonomous system number + name"},
		{"AS_dst", "Destination autonomous system number + name"},
		{"src_bgp_aspath", "Source BGP AS path"},
		{"src_bgp_community", "Source BGP community"},
		{"src_nexthop_ip", "Source BGP next-hop IP"},
		{"src_nexthop_asn", "Source next-hop ASN"},
		{"src_second_asn", "Second ASN in source AS path"},
		{"src_third_asn", "Third ASN in source AS path"},

		// Geography
		{"Geography_src", "Source country"},
		{"Geography_dst", "Destination country"},
		{"src_geo_region", "Source region/state"},
		{"dst_geo_region", "Destination region/state"},
		{"src_geo_city", "Source city"},
		{"dst_geo_city", "Destination city"},

		// Device / Interface
		{"i_device_id", "Device ID"},
		{"i_device_site_name", "Device site name"},
		{"InterfaceID_src", "Source interface (with description)"},
		{"InterfaceID_dst", "Destination interface (with description)"},
		{"i_src_connect_type_name", "Source connectivity type (backbone, free_pni, transit, ix)"},
		{"i_dst_connect_type_name", "Destination connectivity type (backbone, free_pni, transit, ix)"},

		// BGP route
		{"src_route_prefix_len", "Source route prefix length"},
		{"src_route_length", "Source route length"},

		// Aggregate / special
		{"TopFlow", "Top individual flows (5-tuple)"},
		{"Traffic", "Total traffic (single row)"},
		{"ASTopTalkers", "Top ASN talkers"},
		{"InterfaceTopTalkers", "Top interface talkers"},
		{"PortPortTalkers", "Top port-to-port pairs"},
		{"TopFlowsIP", "Top flows by IP"},
		{"RegionTopTalkers", "Top talkers by region"},
	}

	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		search, _ := request.RequireString("search")
		searchLower := strings.ToLower(search)

		var sb strings.Builder
		sb.WriteString("## Kentik Query Dimensions\n\n")
		sb.WriteString(fmt.Sprintf("| %-30s | %-60s |\n", "Dimension", "Description"))
		sb.WriteString("|" + strings.Repeat("-", 32) + "|" + strings.Repeat("-", 62) + "|\n")

		count := 0
		for _, d := range dimensions {
			if searchLower != "" &&
				!strings.Contains(strings.ToLower(d.name), searchLower) &&
				!strings.Contains(strings.ToLower(d.desc), searchLower) {
				continue
			}
			sb.WriteString(fmt.Sprintf("| %-30s | %-60s |\n", d.name, d.desc))
			count++
		}

		if count == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No dimensions matching '%s'. Try: ip, as, port, interface, geo, connect, bgp, vlan, mac", search)), nil
		}

		sb.WriteString(fmt.Sprintf("\n*%d dimensions shown*\n", count))
		return mcp.NewToolResultText(sb.String()), nil
	}
}
