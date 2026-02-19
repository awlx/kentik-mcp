# Kentik MCP Server

A [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server that provides LLM access to the [Kentik](https://www.kentik.com/) network observability platform.

> **Disclaimer:** This is an unofficial, community-maintained project and is not affiliated with, endorsed by, or supported by Kentik, Inc. "Kentik" is a trademark of Kentik, Inc. This project uses the name solely to describe its compatibility with Kentik's publicly documented APIs.

## Features

| Tool | Description |
|------|-------------|
| `kentik_list_devices` | List all registered devices |
| `kentik_search_devices` | Search/filter devices by name, site, type, or label with summarized output |
| `kentik_get_device` | Get device details by ID |
| `kentik_list_interfaces` | List interfaces on a device |
| `kentik_list_all_interfaces` | List interfaces across all devices (bulk, rate-limited) |
| `kentik_get_interface` | Get interface details |
| `kentik_query_data` | Query flow data with convenience filters (connect type, port, ASN, IP) and auto-summarization |
| `kentik_query_url` | Generate a Kentik portal Data Explorer URL for a query |
| `kentik_list_synthetic_tests` | List all synthetic monitoring tests |
| `kentik_get_synthetic_test` | Get synthetic test details |
| `kentik_get_synthetic_results` | Get synthetic test probe results |
| `kentik_list_synthetic_agents` | List synthetic monitoring agents |
| `kentik_get_synthetic_agent` | Get synthetic agent details |
| `kentik_get_synthetic_trace` | Get traceroute data for a synthetic test |
| `kentik_list_labels` | List device labels |
| `kentik_get_label` | Get label details |
| `kentik_list_sites` | List all sites |
| `kentik_get_site` | Get site details |
| `kentik_list_users` | List all users |
| `kentik_get_user` | Get user details |
| `kentik_list_tags` | List flow tags |
| `kentik_get_tag` | Get tag details |
| `kentik_ai_advisor` | Ask Kentik's AI Advisor natural language questions about your network |

## Prerequisites

- Go 1.21+
- A Kentik account with API access
- Your Kentik email and API token (found in User Profile → Authentication)

## Installation

```bash
go install github.com/awlx/kentik-mcp@latest
```

Or build from source:

```bash
git clone https://github.com/awlx/kentik-mcp.git
cd kentik-mcp
go build -o kentik-mcp .
```

## Configuration

Set the following environment variables:

| Variable | Required | Description |
|----------|----------|-------------|
| `KENTIK_EMAIL` | Yes | Your Kentik account email |
| `KENTIK_API_TOKEN` | Yes | Your Kentik API token |
| `KENTIK_REGION` | No | `US` (default) or `EU` |

```bash
export KENTIK_EMAIL=user@example.com
export KENTIK_API_TOKEN=your_api_token_here
export KENTIK_REGION=US
```

## Usage

### With VS Code / GitHub Copilot

Add to your `.vscode/mcp.json`:

```json
{
  "servers": {
    "kentik": {
      "command": "kentik-mcp",
      "env": {
        "KENTIK_EMAIL": "user@example.com",
        "KENTIK_API_TOKEN": "your_api_token_here",
        "KENTIK_REGION": "US"
      }
    }
  }
}
```

### With Claude Desktop

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "kentik": {
      "command": "kentik-mcp",
      "env": {
        "KENTIK_EMAIL": "user@example.com",
        "KENTIK_API_TOKEN": "your_api_token_here",
        "KENTIK_REGION": "US"
      }
    }
  }
}
```

### Standalone (stdio)

```bash
./kentik-mcp
```

The server communicates over stdio using the MCP protocol.

## Example Queries

Once connected, you can ask your LLM things like:

- "Search for border routers in the NYC site"
- "Show me the top 10 destination ASNs by traffic on PNI links in the last 24 hours"
- "What's the traffic breakdown by port on external links (PNI + transit + IX)?"
- "Query flows per second by destination port, filtered to transit links"
- "What synthetic tests are configured?"
- "Ask Kentik AI: How are my devices doing?"
- "Which sites have the most devices?"

## API Coverage

This MCP server covers:

- **V5 REST APIs**: Devices, interfaces, users, sites, labels, tags, and flow data queries
- **V6 gRPC-gateway APIs**: Synthetic monitoring (tests, agents, results, traces) and AI Advisor

For full Kentik API documentation, see: https://kb.kentik.com/docs/apis-overview

## Rate Limits

Kentik applies rate limits per customer. Key limits:

| Limit | Non-query APIs | Query API |
|-------|---------------|-----------|
| Max concurrent | 1 | 4 |
| Soft limit/min | 20 | 30 |
| Hard limit/min | 60 | 100 |
| Hourly limit | 3,750 | 1,500 |

AI Advisor has additional limits: 4 requests/min for create/update, 60 requests/min for polling.

## License

MIT — see [LICENSE](LICENSE).

This project is not affiliated with Kentik, Inc.
