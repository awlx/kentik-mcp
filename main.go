package main

import (
	"fmt"
	"os"

	"github.com/awlx/kentik-mcp/pkg/kentik"
	"github.com/awlx/kentik-mcp/pkg/tools"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	email := os.Getenv("KENTIK_EMAIL")
	apiToken := os.Getenv("KENTIK_API_TOKEN")
	region := os.Getenv("KENTIK_REGION")

	if email == "" || apiToken == "" {
		fmt.Fprintln(os.Stderr, "Error: KENTIK_EMAIL and KENTIK_API_TOKEN environment variables are required")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  export KENTIK_EMAIL=user@example.com")
		fmt.Fprintln(os.Stderr, "  export KENTIK_API_TOKEN=your_api_token")
		fmt.Fprintln(os.Stderr, "  export KENTIK_REGION=US  # optional, US or EU")
		fmt.Fprintln(os.Stderr, "  kentik-mcp")
		os.Exit(1)
	}

	client := kentik.NewClient(kentik.Config{
		Email:    email,
		APIToken: apiToken,
		Region:   region,
	})

	s := server.NewMCPServer(
		"Kentik MCP Server",
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
		server.WithInstructions("Kentik MCP Server provides access to the Kentik network observability platform. "+
			"Available capabilities: query network flow data (traffic by source/dest IP, AS, geography, protocol, etc.), "+
			"list and inspect devices, interfaces, sites, labels, tags, and users, "+
			"run and inspect synthetic monitoring tests, agents, and results, "+
			"ask Kentik's AI Advisor natural language questions about your network. "+
			"API docs: https://kb.kentik.com/docs/apis-overview"),
	)

	tools.RegisterAll(s, client)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
