package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/awlx/kentik-mcp/pkg/kentik"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerDeviceTools(s *server.MCPServer, client *kentik.Client) {
	listDevices := mcp.NewTool("kentik_list_devices",
		mcp.WithDescription("List all devices registered in Kentik. Returns device names, IPs, types, and configuration."),
	)
	s.AddTool(listDevices, makeListDevicesHandler(client))

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
