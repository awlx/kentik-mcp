package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/awlx/kentik-mcp/pkg/kentik"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerAIAdvisorTools(s *server.MCPServer, client *kentik.Client) {
	askAdvisor := mcp.NewTool("kentik_ai_advisor",
		mcp.WithDescription("Ask Kentik's AI Advisor a natural language question about your network. The AI analyzes your Kentik data and returns insights. Examples: 'How are my devices doing?', 'Show me top talkers in the last hour', 'What about interface utilization?'. This is an async operation — the tool polls for completion automatically."),
		mcp.WithString("question",
			mcp.Required(),
			mcp.Description("Natural language question about your network to ask the AI Advisor"),
		),
		mcp.WithString("session_id",
			mcp.Description("Optional existing session ID for follow-up questions. If provided, the question is added as a follow-up to the existing conversation."),
		),
	)
	s.AddTool(askAdvisor, makeAIAdvisorHandler(client))
}

func makeAIAdvisorHandler(client *kentik.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		question, err := request.RequireString("question")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		sessionID, _ := request.RequireString("session_id")

		var data json.RawMessage

		if sessionID != "" {
			body := map[string]interface{}{
				"id":     sessionID,
				"prompt": question,
			}
			data, err = client.V6("PUT", "/ai_advisor/v202511/chat", body)
		} else {
			body := map[string]interface{}{
				"prompt": question,
			}
			data, err = client.V6("POST", "/ai_advisor/v202511/chat", body)
		}
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create AI Advisor session: %v", err)), nil
		}

		var resp struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to parse AI Advisor response: %v", err)), nil
		}

		// Poll for completion (max 90 seconds, 2-second intervals)
		maxWait := 90 * time.Second
		interval := 2 * time.Second
		elapsed := time.Duration(0)

		for elapsed < maxWait {
			time.Sleep(interval)
			elapsed += interval

			pollData, pollErr := client.V6("GET", fmt.Sprintf("/ai_advisor/v202511/chat/%s", resp.ID), nil)
			if pollErr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to poll AI Advisor: %v", pollErr)), nil
			}

			var pollResp struct {
				ID       string `json:"id"`
				Status   string `json:"status"`
				Messages []struct {
					ID           string `json:"id"`
					Status       string `json:"status"`
					Prompt       string `json:"prompt"`
					FinalAnswer  string `json:"finalAnswer"`
					Reasoning    string `json:"reasoning"`
					ErrorMessage string `json:"errorMessage"`
				} `json:"messages"`
			}
			if err := json.Unmarshal(pollData, &pollResp); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to parse poll response: %v", err)), nil
			}

			switch pollResp.Status {
			case "SESSION_STATUS_COMPLETED":
				if len(pollResp.Messages) > 0 {
					lastMsg := pollResp.Messages[len(pollResp.Messages)-1]
					result := fmt.Sprintf("**AI Advisor Response** (session: %s)\n\n%s", pollResp.ID, lastMsg.FinalAnswer)
					return mcp.NewToolResultText(result), nil
				}
				return mcp.NewToolResultText(formatJSON(pollData)), nil

			case "SESSION_STATUS_FAILED":
				errMsg := "Unknown error"
				if len(pollResp.Messages) > 0 {
					lastMsg := pollResp.Messages[len(pollResp.Messages)-1]
					if lastMsg.ErrorMessage != "" {
						errMsg = lastMsg.ErrorMessage
					}
				}
				return mcp.NewToolResultError(fmt.Sprintf("AI Advisor failed: %s", errMsg)), nil
			}
		}

		return mcp.NewToolResultError(fmt.Sprintf(
			"AI Advisor timed out after %v. Session ID: %s — you can retry by passing this session_id.",
			maxWait, resp.ID,
		)), nil
	}
}
