package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/TaqsBlaze/FlamingoDB/internal/network"
	"github.com/chzyer/readline"
	"github.com/jedib0t/go-pretty/v6/table"
)

func loadOrInitAdminConfig(rlInit *readline.Instance) (string, string) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error: Could not determine home directory: %v\n", err)
		os.Exit(1)
	}

	configDir := filepath.Join(home, ".flamingo", "data")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Printf("Error: Could not create config directory: %v\n", err)
		os.Exit(1)
	}

	configPath := filepath.Join(configDir, "config.json")
	
	type Config struct {
		AdminUser string `json:"admin_user"`
		AdminPass string `json:"admin_pass"`
	}

	// Read existing config
	if b, err := os.ReadFile(configPath); err == nil {
		var cfg Config
		if err := json.Unmarshal(b, &cfg); err == nil {
			return cfg.AdminUser, cfg.AdminPass
		}
	}

	// First time setup
	fmt.Println("🦩 Welcome to FlamingoDB! 🦩")
	fmt.Println("It looks like this is your first time running FlamingoDB.")
	fmt.Println("Please set up your default admin account.")
	
	rlInit.SetPrompt("New Admin Username: ")
	line, err := rlInit.Readline()
	if err != nil {
		os.Exit(1)
	}
	newUser := strings.TrimSpace(line)

	cfgPass := rlInit.GenPasswordConfig()
	cfgPass.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		rlInit.SetPrompt("New Admin Password: ")
		rlInit.Refresh()
		return nil, 0, false
	})
	b, err := rlInit.ReadPasswordEx("New Admin Password: ", nil)
	if err != nil {
		os.Exit(1)
	}
	newPass := strings.TrimSpace(string(b))

	// Save to config
	cfg := Config{AdminUser: newUser, AdminPass: newPass}
	cfgBytes, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(configPath, cfgBytes, 0600); err != nil {
		fmt.Printf("Error saving config: %v\n", err)
	} else {
		fmt.Printf("Admin account created and saved to %s\n\n", configPath)
	}

	return newUser, newPass
}

func main() {
	addr := flag.String("addr", "127.0.0.1:4080", "TCP address of the FlamingoDB server")
	userFlag := flag.String("user", "", "Auth username")
	passFlag := flag.String("pass", "", "Auth password")
	dbFlag := flag.String("db", "flamingo", "Database name to connect to")
	flag.Parse()

	rlInit, err := readline.NewEx(&readline.Config{
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		panic(err)
	}

	globalUser, globalPass := loadOrInitAdminConfig(rlInit)

	username := *userFlag
	if username == "" {
		username = globalUser
	}
	password := *passFlag
	if password == "" {
		password = globalPass
	}
	rlInit.Close()

	isAutoLaunched := false
	fmt.Printf("Connecting to FlamingoDB at %s...\n", *addr)
	conn, err := net.Dial("tcp", *addr)
	if err != nil {
		fmt.Printf("Server not detected at %s. Auto-launching local server daemon...\n", *addr)
		isAutoLaunched = true
		
		// Auto-launch the daemon using the global admin credentials
		// and creating the db in the current working directory "."
		cmd := exec.Command("go", "run", "./cmd/flamingodbd", "-tcp", *addr, "-user", globalUser, "-pass", globalPass, "-dir", ".", "-db", *dbFlag)
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Start(); err != nil {
			fmt.Printf("Failed to auto-launch server: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Println("Waiting 3 seconds for server to initialize...")
		time.Sleep(3 * time.Second)
		
		var retryErr error
		for i := 0; i < 3; i++ {
			conn, retryErr = net.Dial("tcp", *addr)
			if retryErr == nil {
				break
			}
			fmt.Printf("Retrying connection (%d/3)...\n", i+1)
			time.Sleep(1 * time.Second)
		}

		if retryErr != nil {
			fmt.Printf("Error: Failed to connect to auto-launched server after retries: %v\n", retryErr)
			os.Exit(1)
		}
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	// 1. Perform Authentication
	authReq := network.TCPRequest{
		Type:     "auth",
		Username: username,
		Password: password,
	}
	if err := encoder.Encode(authReq); err != nil {
		fmt.Printf("Error: Failed to send auth request: %v\n", err)
		os.Exit(1)
	}

	var authResp network.TCPResponse
	if err := decoder.Decode(&authResp); err != nil {
		fmt.Printf("Error: Failed to read auth response: %v\n", err)
		os.Exit(1)
	}

	if !authResp.Success {
		fmt.Printf("Authentication failed: %s\n", authResp.Error)
		os.Exit(1)
	}
	
	fmt.Println("Connected and authenticated successfully.")
	fmt.Println("Type your SQL query and press Enter. Type 'exit' or 'quit' to close.")
	fmt.Println("Meta commands: \\dt (list tables), \\d <table_name> (describe table)")

	// 2. Start REPL Reader Loop using chzyer/readline
	rl, err := readline.NewEx(&readline.Config{
		Prompt:            "flamingo> ",
		HistoryFile:       "/tmp/flamingo_history.tmp",
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil { // EOF or Interrupt
			break
		}

		query := strings.TrimSpace(line)
		if query == "" {
			continue
		}

		lowerQuery := strings.ToLower(query)
		if lowerQuery == "exit" || lowerQuery == "quit" || lowerQuery == "exit;" || lowerQuery == "quit;" {
			if isAutoLaunched {
				_ = encoder.Encode(network.TCPRequest{Type: "meta", Command: "shutdown"})
				time.Sleep(100 * time.Millisecond) // Give server a moment to initiate shutdown
			} else {
				_ = encoder.Encode(network.TCPRequest{Type: "close"})
			}
			break
		}

		var req network.TCPRequest
		
		// Meta commands
		if strings.HasPrefix(lowerQuery, "\\dt") {
			req = network.TCPRequest{
				Type:    "meta",
				Command: "list_tables",
			}
		} else if strings.HasPrefix(lowerQuery, "\\d ") {
			tableName := strings.TrimSpace(query[3:])
			req = network.TCPRequest{
				Type:    "meta",
				Command: "describe_table",
				Query:   tableName,
			}
		} else {
			switch lowerQuery {
			case "begin", "begin;":
				req = network.TCPRequest{Type: "begin"}
			case "commit", "commit;":
				req = network.TCPRequest{Type: "commit"}
			case "rollback", "rollback;":
				req = network.TCPRequest{Type: "rollback"}
			default:
				req = network.TCPRequest{
					Type:  "query",
					Query: query,
				}
			}
		}

		// Send request
		if err := encoder.Encode(req); err != nil {
			fmt.Printf("Error sending request: %v\n", err)
			break
		}

		// Read response
		var resp network.TCPResponse
		if err := decoder.Decode(&resp); err != nil {
			fmt.Printf("Error reading response: %v\n", err)
			break
		}

		if !resp.Success {
			fmt.Printf("Error: %s\n", resp.Error)
			continue
		}

		// Format output results
		if resp.Message != "" {
			fmt.Println(resp.Message)
		}

		if len(resp.Results) > 1 {
			// Multi-statement results
			for i, res := range resp.Results {
				fmt.Printf("--- Statement %d ---\n", i+1)
				if res.Error != "" {
					fmt.Printf("Error: %s\n", res.Error)
				} else {
					if res.Message != "" {
						fmt.Println(res.Message)
					}
					printTable(res.Columns, res.Rows, res.RowsAffected)
				}
			}
		} else {
			// Single statement result
			printTable(resp.Columns, resp.Rows, resp.RowsAffected)
		}
	}

	fmt.Println("Goodbye.")
}

func printTable(columns []string, rows [][]any, rowsAffected int) {
	if len(columns) > 0 {
		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		
		// Setup Headers
		header := make(table.Row, len(columns))
		for i, col := range columns {
			header[i] = col
		}
		t.AppendHeader(header)
		
		// Setup Rows
		for _, row := range rows {
			r := make(table.Row, len(row))
			for i, val := range row {
				r[i] = val
			}
			t.AppendRow(r)
		}
		
		t.SetStyle(table.StyleLight)
		t.Render()
		fmt.Printf("(%d rows)\n\n", len(rows))
	} else if rowsAffected > 0 {
		fmt.Printf("Rows affected: %d\n", rowsAffected)
	}
}
