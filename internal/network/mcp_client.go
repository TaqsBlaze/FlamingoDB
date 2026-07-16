package network

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
)

type MCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema struct {
		Type       string         `json:"type"`
		Properties map[string]any `json:"properties"`
		Required   []string       `json:"required,omitempty"`
	} `json:"inputSchema"`
}

type MCPClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	
	reqID uint64
	mu    sync.Mutex
	
	pending map[uint64]chan []byte
}

func NewMCPClient(policyName string) (*MCPClient, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	
	args := []string{"-mcp"}
	if policyName != "" {
		args = append(args, "-policy="+policyName)
	}
	
	cmd := exec.Command(exe, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	
	// We might want to pass the same dir flag if needed, but for MCP it might be running in the same dir.
	// Actually, let's pass the -dir flag from the current process if possible. We can assume the default or same CWD.
	
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	
	client := &MCPClient{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		pending: make(map[uint64]chan []byte),
	}
	
	go client.readLoop()
	return client, nil
}

func (c *MCPClient) readLoop() {
	scanner := bufio.NewScanner(c.stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		
		var msg struct {
			ID *uint64 `json:"id"`
		}
		if err := json.Unmarshal(line, &msg); err == nil && msg.ID != nil {
			c.mu.Lock()
			ch, ok := c.pending[*msg.ID]
			if ok {
				delete(c.pending, *msg.ID)
			}
			c.mu.Unlock()
			
			if ok {
				resp := make([]byte, len(line))
				copy(resp, line)
				ch <- resp
			}
		}
	}
}

func (c *MCPClient) sendRequest(method string, params any) ([]byte, error) {
	id := atomic.AddUint64(&c.reqID, 1)
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}
	
	data, _ := json.Marshal(req)
	data = append(data, '\n')
	
	ch := make(chan []byte, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()
	
	if _, err := c.stdin.Write(data); err != nil {
		return nil, err
	}
	
	respData := <-ch
	
	var rpcResp struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respData, &rpcResp); err == nil && rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error: %s", rpcResp.Error.Message)
	}
	
	return respData, nil
}

func (c *MCPClient) Initialize() error {
	_, err := c.sendRequest("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "FlamingoDB Client",
			"version": "1.0",
		},
	})
	if err != nil {
		return err
	}
	
	// Send initialized notification
	req := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	data, _ := json.Marshal(req)
	data = append(data, '\n')
	_, _ = c.stdin.Write(data)
	return nil
}

func (c *MCPClient) ListTools() ([]MCPTool, error) {
	respData, err := c.sendRequest("tools/list", nil)
	if err != nil {
		return nil, err
	}
	
	var rpcResp struct {
		Result struct {
			Tools []MCPTool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respData, &rpcResp); err != nil {
		return nil, err
	}
	return rpcResp.Result.Tools, nil
}

func (c *MCPClient) CallTool(name string, args map[string]any) (string, error) {
	respData, err := c.sendRequest("tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}
	
	var rpcResp struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respData, &rpcResp); err != nil {
		return "", err
	}
	
	if len(rpcResp.Result.Content) > 0 {
		return rpcResp.Result.Content[0].Text, nil
	}
	return "", nil
}

func (c *MCPClient) Close() {
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
}
