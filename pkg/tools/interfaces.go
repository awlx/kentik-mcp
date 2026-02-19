package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/awlx/kentik-mcp/pkg/kentik"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerInterfaceTools(s *server.MCPServer, client *kentik.Client) {
	listInterfaces := mcp.NewTool("kentik_list_interfaces",
		mcp.WithDescription("List all interfaces on a specific Kentik device."),
		mcp.WithString("device_id",
			mcp.Required(),
			mcp.Description("The ID of the device whose interfaces to list"),
		),
	)
	s.AddTool(listInterfaces, makeListInterfacesHandler(client))

	listAllInterfaces := mcp.NewTool("kentik_list_all_interfaces",
		mcp.WithDescription("List all interfaces across all Kentik devices. Fetches devices first, then queries interfaces for each device concurrently (respecting rate limits). Returns a JSON array with device_id, device_name, and interfaces for each device."),
	)
	s.AddTool(listAllInterfaces, makeListAllInterfacesHandler(client))

	getInterface := mcp.NewTool("kentik_get_interface",
		mcp.WithDescription("Get detailed information about a specific interface on a device."),
		mcp.WithString("device_id",
			mcp.Required(),
			mcp.Description("The ID of the device"),
		),
		mcp.WithString("interface_id",
			mcp.Required(),
			mcp.Description("The ID of the interface"),
		),
	)
	s.AddTool(getInterface, makeGetInterfaceHandler(client))
}

func makeListInterfacesHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		deviceID, err := request.RequireString("device_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		data, err := client.V5("GET", fmt.Sprintf("/device/%s/interfaces", deviceID), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list interfaces: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(data)), nil
	}
}

type deviceEntry struct {
	ID         string `json:"id"`
	DeviceName string `json:"device_name"`
	Status     string `json:"device_status"`
}

type deviceInterfaceResult struct {
	DeviceID   string          `json:"device_id"`
	DeviceName string          `json:"device_name"`
	Interfaces json.RawMessage `json:"interfaces"`
	Error      string          `json:"error,omitempty"`
}

func makeListAllInterfacesHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Step 1: Fetch all devices
		devicesData, err := client.V5("GET", "/devices", nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list devices: %v", err)), nil
		}

		var devicesResp struct {
			Devices []deviceEntry `json:"devices"`
		}
		if err := json.Unmarshal(devicesData, &devicesResp); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to parse devices: %v", err)), nil
		}

		// Filter to active devices only
		var activeDevices []deviceEntry
		for _, d := range devicesResp.Devices {
			if d.Status == "V" {
				activeDevices = append(activeDevices, d)
			}
		}

		// Step 2: Fetch interfaces for each device with concurrency limit
		results := make([]deviceInterfaceResult, len(activeDevices))
		sem := make(chan struct{}, 4) // max 4 concurrent requests
		var wg sync.WaitGroup

		for i, device := range activeDevices {
			wg.Add(1)
			go func(idx int, dev deviceEntry) {
				defer wg.Done()
				sem <- struct{}{}        // acquire
				defer func() { <-sem }() // release

				// Small delay to stay under rate limits
				time.Sleep(100 * time.Millisecond)

				ifData, ifErr := client.V5("GET", fmt.Sprintf("/device/%s/interfaces", dev.ID), nil)
				results[idx] = deviceInterfaceResult{
					DeviceID:   dev.ID,
					DeviceName: dev.DeviceName,
				}
				if ifErr != nil {
					results[idx].Error = ifErr.Error()
				} else {
					results[idx].Interfaces = ifData
				}
			}(i, device)
		}
		wg.Wait()

		output, err := json.Marshal(results)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal results: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(json.RawMessage(output))), nil
	}
}

func makeGetInterfaceHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		deviceID, err := request.RequireString("device_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		interfaceID, err := request.RequireString("interface_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		data, err := client.V5("GET", fmt.Sprintf("/device/%s/interface/%s", deviceID, interfaceID), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get interface: %v", err)), nil
		}
		return mcp.NewToolResultText(formatJSON(data)), nil
	}
}
