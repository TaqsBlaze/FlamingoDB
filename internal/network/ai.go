package network

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/TaqsBlaze/FlamingoDB/internal/storage/catalog"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/record"
)

// AIModelConfig represents the settings for an AI model provider.
type AIModelConfig struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Provider      string `json:"provider"` // "openai", "anthropic", "gemini", "deepseek"
	APIKey        string `json:"api_key"`
	ModelName     string `json:"model_name"` // e.g. "gpt-4o-mini", "claude-3-5-sonnet", etc.
	Description   string `json:"description"`
	ThinkingLevel string `json:"thinking_level"`
	PolicyName    string `json:"policy_name"`
}

// AIModelStore manages configured AI models, persisting them to models.json.
type AIModelStore struct {
	mu       sync.RWMutex
	models   map[string]*AIModelConfig
	filePath string
}

// NewAIModelStore loads or initializes the AIModelStore.
func NewAIModelStore(filePath string) (*AIModelStore, error) {
	store := &AIModelStore{
		models:    make(map[string]*AIModelConfig),
		filePath: filePath,
	}

	if data, err := os.ReadFile(filePath); err == nil {
		var list []*AIModelConfig
		if err := json.Unmarshal(data, &list); err == nil {
			for _, m := range list {
				store.models[m.ID] = m
			}
		}
	}

	return store, nil
}

func (s *AIModelStore) List() []*AIModelConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*AIModelConfig, 0, len(s.models))
	for _, m := range s.models {
		// Mask the API key for safety
		safe := *m
		if len(safe.APIKey) > 8 {
			safe.APIKey = safe.APIKey[:4] + "..." + safe.APIKey[len(safe.APIKey)-4:]
		} else {
			safe.APIKey = "******"
		}
		list = append(list, &safe)
	}
	return list
}

func (s *AIModelStore) Get(id string) (*AIModelConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.models[id]
	return m, ok
}

func (s *AIModelStore) Set(m *AIModelConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if m.ID == "" {
		// Generate random ID
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		m.ID = hex.EncodeToString(b)
	}

	// If the APIKey is masked (e.g. contains "..."), retain the old one
	if strings.Contains(m.APIKey, "...") || m.APIKey == "******" {
		if old, exists := s.models[m.ID]; exists {
			m.APIKey = old.APIKey
		}
	}

	s.models[m.ID] = m
	return s.save()
}

func (s *AIModelStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.models[id]; !ok {
		return fmt.Errorf("model config not found: %s", id)
	}
	delete(s.models, id)
	return s.save()
}

func (s *AIModelStore) save() error {
	list := make([]*AIModelConfig, 0, len(s.models))
	for _, m := range s.models {
		list = append(list, m)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0600)
}

// CallModel API sends the chat message, handles MCP tools, and queries the AI model.
func CallModel(cfg *AIModelConfig, systemPrompt string, history []map[string]string, dataDir string, dbName string) (string, error) {
	client, err := NewMCPClient(cfg.PolicyName, dataDir, dbName)
	if err == nil {
		defer client.Close()
		_ = client.Initialize()
		tools, err := client.ListTools()
		if err == nil && len(tools) > 0 {
			toolsJSON, _ := json.MarshalIndent(tools, "", "  ")
			systemPrompt += "\n\nYou have access to the following database tools via MCP:\n" + string(toolsJSON)
			systemPrompt += "\n\nTo call a tool, you MUST respond with EXACTLY this JSON format and absolutely nothing else:\n{\"tool_call\": \"tool_name\", \"arguments\": {\"arg1\": \"value1\"}}"
		}
	}

	for i := 0; i < 5; i++ {
		resp, err := callModelOnce(cfg, systemPrompt, history)
		if err != nil {
			return "", err
		}

		var tc struct {
			ToolCall  string         `json:"tool_call"`
			Arguments map[string]any `json:"arguments"`
		}

		cleanResp := strings.TrimSpace(resp)
		if strings.HasPrefix(cleanResp, "```json") {
			cleanResp = strings.TrimPrefix(cleanResp, "```json")
			cleanResp = strings.TrimSuffix(cleanResp, "```")
			cleanResp = strings.TrimSpace(cleanResp)
		}

		if err := json.Unmarshal([]byte(cleanResp), &tc); err == nil && tc.ToolCall != "" {
			var result string
			if client != nil {
				res, err := client.CallTool(tc.ToolCall, tc.Arguments)
				if err != nil {
					result = "Error executing tool: " + err.Error()
				} else {
					result = res
				}
			} else {
				result = "Error: MCP Client is not initialized"
			}
			
			// Append the tool result back into the last message or create a new user message
			lastIdx := len(history) - 1
			if lastIdx >= 0 {
			    history[lastIdx]["text"] += fmt.Sprintf("\n\n(AI Assistant attempted to call tool '%s' with arguments: %v)\n\nTool Result:\n%s\n\nPlease analyze this result and answer my original question.", tc.ToolCall, tc.Arguments, result)
			}
			continue
		}

		return resp, nil
	}
	return "", fmt.Errorf("exceeded maximum tool call loops")
}

func callModelOnce(cfg *AIModelConfig, systemPrompt string, history []map[string]string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	var reqBody []byte
	var req *http.Request
	var err error

	provider := strings.ToLower(cfg.Provider)
	switch provider {
	case "openai":
		url := "https://api.openai.com/v1/chat/completions"
		var messages []map[string]string
		messages = append(messages, map[string]string{"role": "system", "content": systemPrompt})
		for _, m := range history {
			messages = append(messages, map[string]string{"role": m["role"], "content": m["text"]})
		}
		payload := map[string]any{
			"model": cfg.ModelName,
			"messages": messages,
		}
		if cfg.ThinkingLevel != "" && cfg.ThinkingLevel != "none" && (strings.HasPrefix(cfg.ModelName, "o1") || strings.HasPrefix(cfg.ModelName, "o3")) {
			payload["reasoning_effort"] = cfg.ThinkingLevel
		}
		reqBody, _ = json.Marshal(payload)
		req, err = http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	case "anthropic":
		url := "https://api.anthropic.com/v1/messages"
		var messages []map[string]any
		for _, m := range history {
			messages = append(messages, map[string]any{"role": m["role"], "content": m["text"]})
		}
		payload := map[string]any{
			"model":      cfg.ModelName,
			"max_tokens": 1024,
			"system":     systemPrompt,
			"messages":   messages,
		}
		reqBody, _ = json.Marshal(payload)
		req, err = http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", cfg.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")

	case "gemini":
		model := cfg.ModelName
		if !strings.HasPrefix(model, "models/") {
			model = "models/" + model
		}
		url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/%s:generateContent?key=%s", model, cfg.APIKey)
		
		var contents []map[string]any
		for _, m := range history {
		    role := m["role"]
		    if role == "assistant" { role = "model" }
			contents = append(contents, map[string]any{
			    "role": role,
				"parts": []map[string]any{
					{"text": m["text"]},
				},
			})
		}
		
		payload := map[string]any{
			"systemInstruction": map[string]any{
				"parts": map[string]any{
					"text": systemPrompt,
				},
			},
			"contents": contents,
		}
		if cfg.ThinkingLevel != "" && cfg.ThinkingLevel != "none" {
			var budget int
			switch cfg.ThinkingLevel {
			case "low":
				budget = 1024
			case "medium":
				budget = 2048
			case "high":
				budget = 4096
			}
			if budget > 0 {
				payload["thinkingConfig"] = map[string]any{
					"thinkingBudget": budget,
				}
			}
		}
		reqBody, _ = json.Marshal(payload)
		req, err = http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")

	case "deepseek":
		url := "https://api.deepseek.com/chat/completions"
		var messages []map[string]string
		messages = append(messages, map[string]string{"role": "system", "content": systemPrompt})
		for _, m := range history {
			messages = append(messages, map[string]string{"role": m["role"], "content": m["text"]})
		}
		payload := map[string]any{
			"model": cfg.ModelName,
			"messages": messages,
		}
		reqBody, _ = json.Marshal(payload)
		req, err = http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	default:
		return "", fmt.Errorf("unsupported AI provider: %s", cfg.Provider)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed (status %d): %s", resp.StatusCode, string(respBytes))
	}

	// Parse responses
	switch provider {
	case "openai", "deepseek":
		var res struct {
			Choices []struct {
				Message struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(respBytes, &res); err != nil {
			return "", err
		}
		if len(res.Choices) > 0 {
			msg := res.Choices[0].Message
			if msg.ReasoningContent != "" {
				return "<think>\n" + msg.ReasoningContent + "\n</think>\n" + msg.Content, nil
			}
			return msg.Content, nil
		}
	case "anthropic":
		var res struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(respBytes, &res); err != nil {
			return "", err
		}
		if len(res.Content) > 0 {
			return res.Content[0].Text, nil
		}
	case "gemini":
		var res struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if err := json.Unmarshal(respBytes, &res); err != nil {
			return "", err
		}
		if len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
			return res.Candidates[0].Content.Parts[0].Text, nil
		}
	}

	return "", fmt.Errorf("empty or unexpected API response format")
}

// GenerateSystemPrompt creates a prompt listing all tables and their column schemas.
func GenerateSystemPrompt(tm *catalog.TableManager) string {
	var sb strings.Builder
	sb.WriteString("You are the FlamingoDB AI Assistant. You have direct read-only access to the database metadata.\n")
	sb.WriteString("Help the user query the database, explain the schema, or suggest correct SQL statements.\n\n")

	tables := tm.ListTables()
	if len(tables) == 0 {
		sb.WriteString("There are currently no tables registered in the database catalog.\n")
	} else {
		sb.WriteString("Available Tables and Schemas:\n")
		for _, tableName := range tables {
			schema, err := tm.GetSchema(tableName)
			if err != nil {
				continue
			}
			var cols []string
			for _, col := range schema.Columns {
				cols = append(cols, fmt.Sprintf("%s (%s)", col.Name, mapTypeIDToString(col.Type)))
			}
			sb.WriteString(fmt.Sprintf("- Table %q: columns = [%s]\n", tableName, strings.Join(cols, ", ")))
		}
	}

	sb.WriteString("\nFlamingoDB SQL Dialect Notes:\n")
	sb.WriteString("- Supports SHOW TABLES;\n")
	sb.WriteString("- Supports CREATE TABLE table_name (col_name TYPE, ...);\n")
	sb.WriteString("- Supports standard DML: SELECT, INSERT, UPDATE, DELETE.\n")
	sb.WriteString("- Supports advanced scientific types: VECTOR, MATRIX, TENSOR, COMPLEX.\n")
	sb.WriteString("- If asked to query or list records, you MUST use the `execute_query` MCP tool to execute the query yourself.\n")
	sb.WriteString("- IMPORTANT: To display query results to the user in a beautiful HTML table, you must output the EXACT JSON response array returned by the `execute_query` tool inside a markdown block starting with ```json.\n")

	return sb.String()
}

func mapTypeIDToString(t record.TypeID) string {
	switch t {
	case 0:
		return "INT"
	case 1:
		return "FLOAT"
	case 2:
		return "VARCHAR"
	case 3:
		return "COMPLEX"
	case 4:
		return "VECTOR"
	case 5:
		return "MATRIX"
	case 6:
		return "TENSOR"
	case 7:
		return "POINT"
	case 8:
		return "POLYGON"
	default:
		return "UNKNOWN"
	}
}
