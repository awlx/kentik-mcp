package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/awlx/kentik-mcp/pkg/kentik"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerDeviceTools(s *server.MCPServer, client *kentik.Client) {
	listDevices := mcp.NewTool("kentik_list_devices",
		mcp.WithDescription("List all devices registered in Kentik. Returns device names, IPs, types, and configuration."),
	)
	s.AddTool(listDevices, makeListDevicesHandler(client))

	searchDevices := mcp.NewTool("kentik_search_devices",
		mcp.WithDescription("Search and filter Kentik devices by name, site, type, or label. Returns a summarized table of matching devices with ID, name, site, type, status, and SNMP IP. Much more efficient than listing all devices when you know what you're looking for."),
		mcp.WithString("name_filter",
			mcp.Description("Filter devices by name (case-insensitive substring match). E.g. 'bdr' for border routers, 'core' for core routers, 'sw' for switches."),
		),
		mcp.WithString("site_filter",
			mcp.Description("Filter devices by site name (case-insensitive substring match). E.g. 'NYC', 'LAX', 'AMS'."),
		),
		mcp.WithString("type_filter",
			mcp.Description("Filter devices by type/subtype (case-insensitive substring match). E.g. 'router', 'host', 'switch'."),
		),
		mcp.WithString("label_filter",
			mcp.Description("Filter devices by label name (case-insensitive substring match). E.g. 'production', 'edge', 'core'."),
		),
		mcp.WithBoolean("active_only",
			mcp.Description("Only return active devices (status=V). Default: true"),
		),
	)
	s.AddTool(searchDevices, makeSearchDevicesHandler(client))

	getDevice := mcp.NewTool("kentik_get_device",
		mcp.WithDescription("Get detailed information about a specific Kentik device by its ID."),
		mcp.WithString("device_id",
			mcp.Required(),
			mcp.Description("The ID of the device to retrieve"),
		),
	)
	s.AddTool(getDevice, makeGetDeviceHandler(client))
}

func makeListDevicesHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := client.V5("GET", "/devices", nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list devices: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(data)), nil
	}
}

func makeGetDeviceHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		deviceID, err := request.RequireString("device_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		data, err := client.V5("GET", fmt.Sprintf("/device/%s", deviceID), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get device: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(data)), nil
	}
}

// formatJSON pretty-prints a JSON raw message.
func formatJSON(data json.RawMessage) string {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		return string(data)
	}
	return pretty.String()
}

func makeSearchDevicesHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := client.V5("GET", "/devices", nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list devices: %v", err)), nil
		}

		var resp struct {
			Devices []struct {
				ID          string `json:"id"`
				Name        string `json:"device_name"`
				Type        string `json:"device_type"`
				Subtype     string `json:"device_subtype"`
				Status      string `json:"device_status"`
				SNMPIP      string `json:"device_snmp_ip"`
				Description string `json:"device_description"`
				Site        struct {
					Name string `json:"site_name"`
				} `json:"site"`
				Labels []struct {
					Name string `json:"name"`
				} `json:"labels"`
			} `json:"devices"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to parse devices: %v", err)), nil
		}

		nameFilter, _ := request.RequireString("name_filter")
		siteFilter, _ := request.RequireString("site_filter")
		typeFilter, _ := request.RequireString("type_filter")
		labelFilter, _ := request.RequireString("label_filter")
		activeOnly := true
		if ao, err := request.RequireString("active_only"); err == nil && ao == "false" {
			activeOnly = false
		}

		nameFilter = strings.ToLower(nameFilter)
		siteFilter = strings.ToLower(siteFilter)
		typeFilter = strings.ToLower(typeFilter)
		labelFilter = strings.ToLower(labelFilter)

		var result strings.Builder
		matched := 0
		var deviceNames []string

		result.WriteString(fmt.Sprintf("%-8s %-55s %-15s %-12s %-8s %-18s %s\n",
			"ID", "Name", "Site", "Type", "Status", "SNMP IP", "Labels"))
		result.WriteString(strings.Repeat("-", 140) + "\n")

		for _, d := range resp.Devices {
			if activeOnly && d.Status != "V" {
				continue
			}
			if nameFilter != "" && !strings.Contains(strings.ToLower(d.Name), nameFilter) {
				continue
			}
			if siteFilter != "" && !strings.Contains(strings.ToLower(d.Site.Name), siteFilter) {
				continue
			}
			devType := d.Subtype
			if devType == "" {
				devType = d.Type
			}
			if typeFilter != "" && !strings.Contains(strings.ToLower(devType), typeFilter) {
				continue
			}
			if labelFilter != "" {
				hasLabel := false
				for _, l := range d.Labels {
					if strings.Contains(strings.ToLower(l.Name), labelFilter) {
						hasLabel = true
						break
					}
				}
				if !hasLabel {
					continue
				}
			}

			status := "Active"
			if d.Status != "V" {
				status = d.Status
			}
			labelNames := make([]string, 0, len(d.Labels))
			for _, l := range d.Labels {
				labelNames = append(labelNames, l.Name)
			}
			labels := strings.Join(labelNames, ",")
			if len(labels) > 30 {
				labels = labels[:27] + "..."
			}

			name := d.Name
			if len(name) > 54 {
				name = name[:54]
			}

			result.WriteString(fmt.Sprintf("%-8s %-55s %-15s %-12s %-8s %-18s %s\n",
				d.ID, name, d.Site.Name, devType, status, d.SNMPIP, labels))
			matched++
			deviceNames = append(deviceNames, d.Name)
		}

		result.WriteString(fmt.Sprintf("\nMatched: %d devices\n", matched))
		if matched > 0 && matched <= 50 {
			result.WriteString(fmt.Sprintf("\nDevice names for query:\n%s\n", strings.Join(deviceNames, ",")))
		}

		return mcp.NewToolResultText(result.String()), nil
	}
}
