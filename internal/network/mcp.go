package network

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// RunMCPServer runs the Model Context Protocol (MCP) server over stdio.
func (s *Server) RunMCPServer(policyName string) {
	s.log.Info("Starting MCP Server over stdio...")
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			s.log.Error("MCP read error: %v", err)
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var req struct {
			JsonRPC string          `json:"jsonrpc"`
			ID      any             `json:"id,omitempty"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params,omitempty"`
		}

		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.sendMCPError(req.ID, -32700, "Parse error", nil)
			continue
		}

		switch req.Method {
		case "initialize":
			s.handleMCPInitialize(req.ID)
		case "initialized":
			// No-op notification
		case "tools/list":
			s.handleMCPToolsList(req.ID)
		case "tools/call":
			var params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments,omitempty"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				s.sendMCPError(req.ID, -32602, "Invalid params", nil)
				continue
			}
			s.handleMCPToolsCall(req.ID, params.Name, params.Arguments, policyName)
		default:
			s.sendMCPError(req.ID, -32601, "Method not found", nil)
		}
	}
}

func (s *Server) sendMCPResponse(id any, result any) {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
	b, _ := json.Marshal(resp)
	fmt.Println(string(b))
}

func (s *Server) sendMCPError(id any, code int, msg string, data any) {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    code,
			"message": msg,
			"data":    data,
		},
	}
	b, _ := json.Marshal(resp)
	fmt.Println(string(b))
}

func (s *Server) handleMCPInitialize(id any) {
	result := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "flamingodb-mcp",
			"version": "1.2.0",
		},
	}
	s.sendMCPResponse(id, result)
}

func (s *Server) handleMCPToolsList(id any) {
	tools := []map[string]any{
		{
			"name":        "list_tables",
			"description": "List all tables available in FlamingoDB",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "describe_table",
			"description": "Get the schema and columns of a table",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"table_name": map[string]any{
						"type":        "string",
						"description": "The name of the table to describe",
					},
				},
				"required": []string{"table_name"},
			},
		},
		{
			"name":        "execute_query",
			"description": "Run a SQL statement on FlamingoDB (SELECT, INSERT, CREATE, etc.)",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "The SQL query to execute",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "generate_chart",
			"description": "Generate a chart configuration for visualizing data",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"type": map[string]any{
						"type":        "string",
						"description": "The type of chart to generate (bar, line, pie, doughnut, radar, polarArea, scatter, bubble)",
						"enum":        []string{"bar", "line", "pie", "doughnut", "radar", "polarArea", "scatter", "bubble"},
					},
					"data": map[string]any{
						"type":        "object",
						"description": "The data for the chart (labels and datasets)",
					},
					"options": map[string]any{
						"type":        "object",
						"description": "Optional configuration options for the chart",
					},
				},
				"required": []string{"type", "data"},
			},
		},
	}

	s.sendMCPResponse(id, map[string]any{"tools": tools})
}

func (s *Server) handleMCPToolsCall(id any, name string, args json.RawMessage, policyName string) {
	switch name {
	case "list_tables":
		tables := s.tm.ListTables()
		var text string
		if len(tables) == 0 {
			text = "No tables found."
		} else {
			text = fmt.Sprintf("Tables: [%s]", strings.Join(tables, ", "))
		}
		s.sendMCPToolResult(id, text, false)

	case "describe_table":
		var params struct {
			TableName string `json:"table_name"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			s.sendMCPError(id, -32602, "Invalid arguments", nil)
			return
		}
		schema, err := s.tm.GetSchema(params.TableName)
		if err != nil {
			s.sendMCPToolResult(id, fmt.Sprintf("Error: %v", err), true)
			return
		}
		var cols []string
		for _, col := range schema.Columns {
			cols = append(cols, fmt.Sprintf("- %s: %s", col.Name, mapTypeIDToString(col.Type)))
		}
		text := fmt.Sprintf("Table %s schema:\n%s", params.TableName, strings.Join(cols, "\n"))
		s.sendMCPToolResult(id, text, false)

	case "execute_query":
		var params struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			s.sendMCPError(id, -32602, "Invalid arguments", nil)
			return
		}

		if policyName != "" && policyName != "Admin" {
			if policy, ok := s.policyStore.Get(policyName); ok {
				qUpper := strings.ToUpper(strings.TrimSpace(params.Query))
				if (strings.HasPrefix(qUpper, "SELECT")) && !policy.CanSelect {
					s.sendMCPToolResult(id, "Error: permission denied for SELECT by policy", true)
					return
				}
				if (strings.HasPrefix(qUpper, "INSERT")) && !policy.CanInsert {
					s.sendMCPToolResult(id, "Error: permission denied for INSERT by policy", true)
					return
				}
				if (strings.HasPrefix(qUpper, "UPDATE")) && !policy.CanUpdate {
					s.sendMCPToolResult(id, "Error: permission denied for UPDATE by policy", true)
					return
				}
				if (strings.HasPrefix(qUpper, "DELETE")) && !policy.CanDelete {
					s.sendMCPToolResult(id, "Error: permission denied for DELETE by policy", true)
					return
				}
				if (strings.HasPrefix(qUpper, "CREATE")) && !policy.CanCreate {
					s.sendMCPToolResult(id, "Error: permission denied for CREATE by policy", true)
					return
				}
				if (strings.HasPrefix(qUpper, "DROP")) && !policy.CanDrop {
					s.sendMCPToolResult(id, "Error: permission denied for DROP by policy", true)
					return
				}
			}
		}

		resList, err := s.ProcessQuery(nil, params.Query)
		if err != nil {
			s.sendMCPToolResult(id, fmt.Sprintf("Execution failed: %v", err), true)
			return
		}

		jsonData, err := json.Marshal(resList)
		if err != nil {
			s.sendMCPToolResult(id, fmt.Sprintf("Error encoding result: %v", err), true)
			return
		}
		s.sendMCPToolResult(id, string(jsonData), false)

	default:
		s.sendMCPError(id, -32601, fmt.Sprintf("Tool %s not found", name), nil)
	}
}

func (s *Server) sendMCPToolResult(id any, text string, isError bool) {
	result := map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": text,
			},
		},
		"isError": isError,
	}
	s.sendMCPResponse(id, result)
}
