package tools

import (
	"github.com/awlx/kentik-mcp/pkg/kentik"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterAll registers every Kentik tool on the given MCP server.
func RegisterAll(s *server.MCPServer, client *kentik.Client) {
	registerDeviceTools(s, client)
	registerInterfaceTools(s, client)
	registerQueryTools(s, client)
	registerSyntheticsTools(s, client)
	registerLabelTools(s, client)
	registerSiteTools(s, client)
	registerUserTools(s, client)
	registerTagTools(s, client)
	registerAIAdvisorTools(s, client)
}
