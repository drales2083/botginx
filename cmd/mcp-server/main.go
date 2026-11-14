// GuardBot MCP Server
// Exposes GuardBot tools to AI assistants via Model Context Protocol.
// Use with Claude Desktop or any MCP-compatible client.
//
// Usage:
//   export GUARDBOT_API_KEY=bg_live_xxxx
//   export GUARDBOT_API_URL=https://guardbot.sbs  # optional, defaults to production
//   ./guardbot-mcp
//
// Add to Claude Desktop config (~/Library/Application Support/Claude/claude_desktop_config.json):
//   {
//     "mcpServers": {
//       "guardbot": {
//         "command": "/path/to/guardbot-mcp",
//         "env": { "GUARDBOT_API_KEY": "bg_live_xxxx" }
//       }
//     }
//   }

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

var (
	apiKey  = os.Getenv("GUARDBOT_API_KEY")
	apiURL  = os.Getenv("GUARDBOT_API_URL")
	version = "1.0.0"
)

func init() {
	if apiURL == "" {
		apiURL = "https://guardbot.sbs"
	}
}

// JSON-RPC types
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP types
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Capabilities struct {
	Tools map[string]interface{} `json:"tools,omitempty"`
}

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Tool definitions
var tools = []Tool{
	{
		Name:        "guardbot_search_links",
		Description: "Search your GuardBot redirect links by keyword",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"q":     map[string]string{"type": "string", "description": "Search query"},
				"limit": map[string]interface{}{"type": "number", "description": "Max results (1-25)"},
			},
			"required": []string{"q"},
		},
	},
	{
		Name:        "guardbot_get_domains",
		Description: "List all your verified GuardBot domains",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	},
	{
		Name:        "guardbot_get_links",
		Description: "List your most recent redirect links",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"limit": map[string]interface{}{"type": "number", "description": "Max results (1-100)"},
			},
		},
	},
	{
		Name:        "guardbot_get_link_stats",
		Description: "Get analytics for a specific redirect link",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"link_id": map[string]string{"type": "string", "description": "Link ID"},
			},
			"required": []string{"link_id"},
		},
	},
	{
		Name:        "guardbot_create_link",
		Description: "Create a new redirect link",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain_id":   map[string]string{"type": "string", "description": "Domain ID (from get_domains)"},
				"subdomain":   map[string]string{"type": "string", "description": "Subdomain (optional)"},
				"destination": map[string]string{"type": "string", "description": "Destination URL"},
			},
			"required": []string{"domain_id", "destination"},
		},
	},
	{
		Name:        "guardbot_update_link",
		Description: "Update a redirect link destination",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"link_id":     map[string]string{"type": "string", "description": "Link ID"},
				"destination": map[string]string{"type": "string", "description": "New destination URL"},
			},
			"required": []string{"link_id"},
		},
	},
	{
		Name:        "guardbot_delete_link",
		Description: "Delete a redirect link permanently",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"link_id": map[string]string{"type": "string", "description": "Link ID"},
			},
			"required": []string{"link_id"},
		},
	},
}

func main() {
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "GUARDBOT_API_KEY environment variable is required")
		os.Exit(1)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading: %v\n", err)
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			sendError(nil, -32700, "Parse error")
			continue
		}

		handleRequest(req)
	}
}

func handleRequest(req Request) {
	switch req.Method {
	case "initialize":
		sendResult(req.ID, map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": ServerInfo{
				Name:    "guardbot",
				Version: version,
			},
			"capabilities": Capabilities{
				Tools: map[string]interface{}{},
			},
		})

	case "tools/list":
		sendResult(req.ID, map[string]interface{}{
			"tools": tools,
		})

	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			sendError(req.ID, -32602, "Invalid params")
			return
		}
		result := callTool(params.Name, params.Arguments)
		sendResult(req.ID, result)

	default:
		sendError(req.ID, -32601, "Method not found")
	}
}

func callTool(name string, args map[string]interface{}) ToolResult {
	var endpoint, method string
	var body interface{}

	switch name {
	case "guardbot_search_links":
		endpoint = "/api/v1/links/search"
		method = "GET"
	case "guardbot_get_domains":
		endpoint = "/api/v1/domains"
		method = "GET"
	case "guardbot_get_links":
		endpoint = "/api/v1/links"
		method = "GET"
	case "guardbot_get_link_stats":
		linkID, _ := args["link_id"].(string)
		endpoint = "/api/v1/links/" + linkID + "/analytics"
		method = "GET"
	case "guardbot_create_link":
		endpoint = "/api/v1/links"
		method = "POST"
		body = args
	case "guardbot_update_link":
		linkID, _ := args["link_id"].(string)
		endpoint = "/api/v1/links/" + linkID
		method = "PUT"
		body = args
	case "guardbot_delete_link":
		linkID, _ := args["link_id"].(string)
		endpoint = "/api/v1/links/" + linkID
		method = "DELETE"
	default:
		return ToolResult{
			Content: []ContentBlock{{Type: "text", Text: "Unknown tool: " + name}},
			IsError: true,
		}
	}

	// Add query params for GET requests with args
	if method == "GET" && len(args) > 0 {
		params := []string{}
		for k, v := range args {
			params = append(params, fmt.Sprintf("%s=%v", k, v))
		}
		endpoint += "?" + strings.Join(params, "&")
	}

	result, err := apiRequest(method, endpoint, body)
	if err != nil {
		return ToolResult{
			Content: []ContentBlock{{Type: "text", Text: "API error: " + err.Error()}},
			IsError: true,
		}
	}

	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: result}},
	}
}

func apiRequest(method, endpoint string, body interface{}) (string, error) {
	url := apiURL + endpoint

	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = strings.NewReader(string(b))
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return string(respBody), nil
}

func sendResult(id interface{}, result interface{}) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	b, _ := json.Marshal(resp)
	fmt.Println(string(b))
}

func sendError(id interface{}, code int, message string) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &Error{Code: code, Message: message},
	}
	b, _ := json.Marshal(resp)
	fmt.Println(string(b))
}
