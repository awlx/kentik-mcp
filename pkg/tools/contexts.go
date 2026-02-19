package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// QueryContext is a saved set of query parameters that can be reused.
type QueryContext struct {
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	DeviceNames    string   `json:"device_names,omitempty"`
	SiteName       string   `json:"site_name,omitempty"`
	DeviceLabel    string   `json:"device_label,omitempty"`
	DstConnectType string   `json:"dst_connect_type,omitempty"`
	SrcConnectType string   `json:"src_connect_type,omitempty"`
	Port           string   `json:"port,omitempty"`
	DstAS          string   `json:"dst_as,omitempty"`
	SrcAS          string   `json:"src_as,omitempty"`
	FiltersJSON    string   `json:"filters_json,omitempty"`
	Tags           []string `json:"tags,omitempty"`
}

type QueryContextFile struct {
	Contexts []QueryContext `json:"contexts"`
}

func registerContextTools(s *server.MCPServer) {
	saveContext := mcp.NewTool("kentik_save_context",
		mcp.WithDescription("Save a named query context (device group + filters) for reuse. Contexts are stored in ~/.kentik-mcp-contexts.json. Use context_name on query/compare tools to apply saved parameters."),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Unique name for this context. E.g. 'borders', 'external-traffic', 'core-routers'."),
		),
		mcp.WithString("description",
			mcp.Description("Human-readable description of what this context covers."),
		),
		mcp.WithString("device_names",
			mcp.Description("Comma-delimited device names to save."),
		),
		mcp.WithString("site_name",
			mcp.Description("Site name to save."),
		),
		mcp.WithString("device_label",
			mcp.Description("Device label to save."),
		),
		mcp.WithString("dst_connect_type",
			mcp.Description("Destination connectivity type filter to save."),
		),
		mcp.WithString("src_connect_type",
			mcp.Description("Source connectivity type filter to save."),
		),
		mcp.WithString("port",
			mcp.Description("Port filter to save."),
		),
		mcp.WithString("dst_as",
			mcp.Description("Destination AS filter to save."),
		),
	)
	s.AddTool(saveContext, makeSaveContextHandler())

	listContexts := mcp.NewTool("kentik_list_contexts",
		mcp.WithDescription("List all saved query contexts. Shows the name, description, and parameters of each saved context."),
	)
	s.AddTool(listContexts, makeListContextsHandler())

	deleteContext := mcp.NewTool("kentik_delete_context",
		mcp.WithDescription("Delete a saved query context by name."),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name of the context to delete."),
		),
	)
	s.AddTool(deleteContext, makeDeleteContextHandler())
}

func contextFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kentik-mcp-contexts.json")
}

func loadContexts() (*QueryContextFile, error) {
	data, err := os.ReadFile(contextFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return &QueryContextFile{}, nil
		}
		return nil, err
	}
	var cf QueryContextFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, err
	}
	return &cf, nil
}

func saveContexts(cf *QueryContextFile) error {
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(contextFilePath(), data, 0644)
}

// GetContext returns a saved context by name, or nil if not found.
func GetContext(name string) *QueryContext {
	cf, err := loadContexts()
	if err != nil {
		return nil
	}
	nameLower := strings.ToLower(name)
	for _, c := range cf.Contexts {
		if strings.ToLower(c.Name) == nameLower {
			return &c
		}
	}
	return nil
}

func makeSaveContextHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := request.RequireString("name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		qc := QueryContext{Name: name}
		qc.Description, _ = request.RequireString("description")
		qc.DeviceNames, _ = request.RequireString("device_names")
		qc.SiteName, _ = request.RequireString("site_name")
		qc.DeviceLabel, _ = request.RequireString("device_label")
		qc.DstConnectType, _ = request.RequireString("dst_connect_type")
		qc.SrcConnectType, _ = request.RequireString("src_connect_type")
		qc.Port, _ = request.RequireString("port")
		qc.DstAS, _ = request.RequireString("dst_as")

		cf, err := loadContexts()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to load contexts: %v", err)), nil
		}

		// Replace existing or append
		found := false
		for i, c := range cf.Contexts {
			if strings.ToLower(c.Name) == strings.ToLower(name) {
				cf.Contexts[i] = qc
				found = true
				break
			}
		}
		if !found {
			cf.Contexts = append(cf.Contexts, qc)
		}

		if err := saveContexts(cf); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to save: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Context '%s' saved (%s).", name, contextFilePath())), nil
	}
}

func makeListContextsHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cf, err := loadContexts()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to load contexts: %v", err)), nil
		}

		if len(cf.Contexts) == 0 {
			return mcp.NewToolResultText("No saved contexts. Use kentik_save_context to create one."), nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("## Saved Query Contexts (%d)\n\n", len(cf.Contexts)))
		for _, c := range cf.Contexts {
			sb.WriteString(fmt.Sprintf("### %s\n", c.Name))
			if c.Description != "" {
				sb.WriteString(fmt.Sprintf("*%s*\n", c.Description))
			}
			if c.DeviceNames != "" {
				sb.WriteString(fmt.Sprintf("- device_names: `%s`\n", c.DeviceNames))
			}
			if c.SiteName != "" {
				sb.WriteString(fmt.Sprintf("- site_name: `%s`\n", c.SiteName))
			}
			if c.DeviceLabel != "" {
				sb.WriteString(fmt.Sprintf("- device_label: `%s`\n", c.DeviceLabel))
			}
			if c.DstConnectType != "" {
				sb.WriteString(fmt.Sprintf("- dst_connect_type: `%s`\n", c.DstConnectType))
			}
			if c.SrcConnectType != "" {
				sb.WriteString(fmt.Sprintf("- src_connect_type: `%s`\n", c.SrcConnectType))
			}
			if c.Port != "" {
				sb.WriteString(fmt.Sprintf("- port: `%s`\n", c.Port))
			}
			if c.DstAS != "" {
				sb.WriteString(fmt.Sprintf("- dst_as: `%s`\n", c.DstAS))
			}
			sb.WriteString("\n")
		}
		return mcp.NewToolResultText(sb.String()), nil
	}
}

func makeDeleteContextHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := request.RequireString("name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cf, err := loadContexts()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to load contexts: %v", err)), nil
		}

		nameLower := strings.ToLower(name)
		var newContexts []QueryContext
		found := false
		for _, c := range cf.Contexts {
			if strings.ToLower(c.Name) == nameLower {
				found = true
				continue
			}
			newContexts = append(newContexts, c)
		}
		if !found {
			return mcp.NewToolResultError(fmt.Sprintf("Context '%s' not found.", name)), nil
		}
		cf.Contexts = newContexts

		if err := saveContexts(cf); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to save: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Context '%s' deleted.", name)), nil
	}
}
