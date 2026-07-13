package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/TaqsBlaze/FlamingoDB/internal/network"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:4080", "TCP address of the FlamingoDB server")
	user := flag.String("user", "admin", "Auth username")
	pass := flag.String("pass", "admin", "Auth password")
	flag.Parse()

	fmt.Printf("Connecting to FlamingoDB at %s...\n", *addr)
	conn, err := net.Dial("tcp", *addr)
	if err != nil {
		fmt.Printf("Error: Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	// 1. Perform Authentication
	authReq := network.TCPRequest{
		Type:     "auth",
		Username: *user,
		Password: *pass,
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

	// 2. Start REPL Reader Loop
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("flamingo> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		query := strings.TrimSpace(line)
		if query == "" {
			continue
		}

		lowerQuery := strings.ToLower(query)
		if lowerQuery == "exit" || lowerQuery == "quit" || lowerQuery == "exit;" || lowerQuery == "quit;" {
			_ = encoder.Encode(network.TCPRequest{Type: "close"})
			break
		}

		// Handle explicit transaction control commands for convenience if user enters them
		var req network.TCPRequest
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
		// Calculate column widths
		widths := make([]int, len(columns))
		for i, col := range columns {
			widths[i] = len(col)
		}

		for _, row := range rows {
			for i, val := range row {
				strVal := fmt.Sprintf("%v", val)
				if len(strVal) > widths[i] {
					widths[i] = len(strVal)
				}
			}
		}

		// Print header
		for i, col := range columns {
			fmt.Printf("| %-*s ", widths[i], col)
		}
		fmt.Println("|")

		// Print separator
		for _, w := range widths {
			fmt.Print("+" + strings.Repeat("-", w+2))
		}
		fmt.Println("+")

		// Print rows
		for _, row := range rows {
			for i, val := range row {
				fmt.Printf("| %-*v ", widths[i], val)
			}
			fmt.Println("|")
		}

		fmt.Printf("(%d rows)\n", len(rows))
	} else if rowsAffected > 0 {
		fmt.Printf("Rows affected: %d\n", rowsAffected)
	}
}
