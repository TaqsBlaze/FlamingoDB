package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"flamingodb/internal/storage/catalog"
	"flamingodb/internal/storage/disk"
	"flamingodb/internal/storage/pager"
	"flamingodb/pkg/logger"
)

func setupTestServer(t *testing.T, username, password string, maxConn int) (*Server, string) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	pageSize := uint32(4096)

	dm, err := disk.NewDiskManager(dbPath, pageSize)
	if err != nil {
		t.Fatalf("failed to create disk manager: %v", err)
	}

	p, err := pager.New(dm, pageSize)
	if err != nil {
		t.Fatalf("failed to create pager: %v", err)
	}

	tm, err := catalog.NewTableManager(p)
	if err != nil {
		t.Fatalf("failed to create table manager: %v", err)
	}

	t.Cleanup(func() {
		_ = tm.Close()
		_ = dm.Close()
	})

	log := logger.New(logger.LevelDebug)
	cfg := Config{
		TCPAddr:        "127.0.0.1:0",
		HTTPAddr:       "127.0.0.1:0",
		Username:       username,
		Password:       password,
		MaxConnections: maxConn,
	}

	srv, err := NewServer(cfg, tm, log)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	t.Cleanup(func() {
		_ = srv.Close()
	})

	return srv, dbPath
}

func TestTCPServerEndToEnd(t *testing.T) {
	srv, _ := setupTestServer(t, "admin", "adminpass", 5)

	tcpAddr := srv.TCPAddr()
	if tcpAddr == "" {
		t.Fatalf("expected TCP address to be bound")
	}

	conn, err := net.Dial("tcp", tcpAddr)
	if err != nil {
		t.Fatalf("failed to connect to TCP server: %v", err)
	}
	defer conn.Close()

	// 1. Try a query before authentication
	err = json.NewEncoder(conn).Encode(TCPRequest{
		Type:  "query",
		Query: "CREATE TABLE t (id INT, val FLOAT);",
	})
	if err != nil {
		t.Fatalf("failed to send command: %v", err)
	}

	var resp TCPResponse
	err = json.NewDecoder(conn).Decode(&resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Success {
		t.Errorf("expected query to fail before authentication")
	}
	if !strings.Contains(resp.Error, "unauthenticated") {
		t.Errorf("expected error to mention 'unauthenticated', got: %s", resp.Error)
	}

	// 2. Authenticate
	err = json.NewEncoder(conn).Encode(TCPRequest{
		Type:     "auth",
		Username: "admin",
		Password: "adminpass",
	})
	if err != nil {
		t.Fatalf("failed to send auth: %v", err)
	}

	err = json.NewDecoder(conn).Decode(&resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected authentication to succeed: %s", resp.Error)
	}

	// 3. Create Table
	err = json.NewEncoder(conn).Encode(TCPRequest{
		Type:  "query",
		Query: "CREATE TABLE products (id INT, price FLOAT, name VARCHAR);",
	})
	if err != nil {
		t.Fatalf("failed to send create table: %v", err)
	}

	err = json.NewDecoder(conn).Decode(&resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected CREATE TABLE to succeed: %s", resp.Error)
	}

	// 4. Insert Record
	err = json.NewEncoder(conn).Encode(TCPRequest{
		Type:  "query",
		Query: "INSERT INTO products VALUES (1, 12.99, 'Book');",
	})
	if err != nil {
		t.Fatalf("failed to send insert: %v", err)
	}

	err = json.NewDecoder(conn).Decode(&resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected INSERT to succeed: %s", resp.Error)
	}

	// 5. Select Record
	err = json.NewEncoder(conn).Encode(TCPRequest{
		Type:  "query",
		Query: "SELECT id, price, name FROM products;",
	})
	if err != nil {
		t.Fatalf("failed to send select: %v", err)
	}

	err = json.NewDecoder(conn).Decode(&resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected SELECT to succeed: %s", resp.Error)
	}

	if len(resp.Columns) != 3 || resp.Columns[0] != "id" || resp.Columns[1] != "price" || resp.Columns[2] != "name" {
		t.Errorf("expected columns [id price name], got: %v", resp.Columns)
	}

	if len(resp.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(resp.Rows))
	} else {
		row := resp.Rows[0]
		if fmt.Sprintf("%v", row[0]) != "1" {
			t.Errorf("expected id to be 1, got %v", row[0])
		}
		if fmt.Sprintf("%v", row[1]) != "12.99" {
			t.Errorf("expected price to be 12.99, got %v", row[1])
		}
		if fmt.Sprintf("%v", row[2]) != "Book" {
			t.Errorf("expected name to be 'Book', got %v", row[2])
		}
	}
}

func TestTCPServerTransactions(t *testing.T) {
	srv, _ := setupTestServer(t, "", "", 5) // No auth

	tcpAddr := srv.TCPAddr()
	conn, err := net.Dial("tcp", tcpAddr)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Create table
	_ = json.NewEncoder(conn).Encode(TCPRequest{Type: "query", Query: "CREATE TABLE t (id INT);"})
	var resp TCPResponse
	_ = json.NewDecoder(conn).Decode(&resp)

	// Begin transaction
	_ = json.NewEncoder(conn).Encode(TCPRequest{Type: "begin"})
	_ = json.NewDecoder(conn).Decode(&resp)
	if !resp.Success {
		t.Fatalf("expected begin to succeed: %s", resp.Error)
	}

	// Insert row
	_ = json.NewEncoder(conn).Encode(TCPRequest{Type: "query", Query: "INSERT INTO t VALUES (42);"})
	_ = json.NewDecoder(conn).Decode(&resp)
	if !resp.Success {
		t.Fatalf("expected insert to succeed: %s", resp.Error)
	}

	// Rollback transaction
	_ = json.NewEncoder(conn).Encode(TCPRequest{Type: "rollback"})
	_ = json.NewDecoder(conn).Decode(&resp)
	if !resp.Success {
		t.Fatalf("expected rollback to succeed: %s", resp.Error)
	}

	// Verify no rows exist
	_ = json.NewEncoder(conn).Encode(TCPRequest{Type: "query", Query: "SELECT * FROM t;"})
	_ = json.NewDecoder(conn).Decode(&resp)
	if len(resp.Rows) != 0 {
		t.Errorf("expected 0 rows after rollback, got %d", len(resp.Rows))
	}

	// Begin another transaction, insert and commit
	_ = json.NewEncoder(conn).Encode(TCPRequest{Type: "begin"})
	_ = json.NewDecoder(conn).Decode(&resp)
	_ = json.NewEncoder(conn).Encode(TCPRequest{Type: "query", Query: "INSERT INTO t VALUES (100);"})
	_ = json.NewDecoder(conn).Decode(&resp)
	_ = json.NewEncoder(conn).Encode(TCPRequest{Type: "commit"})
	_ = json.NewDecoder(conn).Decode(&resp)

	// Verify row exists
	_ = json.NewEncoder(conn).Encode(TCPRequest{Type: "query", Query: "SELECT * FROM t;"})
	_ = json.NewDecoder(conn).Decode(&resp)
	if len(resp.Rows) != 1 {
		t.Errorf("expected 1 row after commit, got %d", len(resp.Rows))
	}
}

func TestTCPConnectionPooling(t *testing.T) {
	srv, _ := setupTestServer(t, "", "", 2) // Max connections = 2

	tcpAddr := srv.TCPAddr()

	// Open first connection
	conn1, err := net.Dial("tcp", tcpAddr)
	if err != nil {
		t.Fatalf("failed to connect conn1: %v", err)
	}
	defer conn1.Close()

	// Open second connection
	conn2, err := net.Dial("tcp", tcpAddr)
	if err != nil {
		t.Fatalf("failed to connect conn2: %v", err)
	}
	defer conn2.Close()

	// Wait a tiny bit for server registration
	time.Sleep(100 * time.Millisecond)

	// Try to open third connection
	conn3, err := net.Dial("tcp", tcpAddr)
	if err != nil {
		t.Fatalf("failed to connect conn3: %v", err)
	}
	defer conn3.Close()

	// The third connection should be closed by server immediately or return an error response
	var resp TCPResponse
	err = json.NewDecoder(conn3).Decode(&resp)
	if err != nil && err != io.EOF {
		t.Fatalf("failed to decode response from conn3: %v", err)
	}

	if !resp.Success && !strings.Contains(resp.Error, "Connection pool exhausted") && err != io.EOF {
		t.Errorf("expected connection pooling rejection error, got: %s (err: %v)", resp.Error, err)
	}
}

func TestHTTPServerEndToEnd(t *testing.T) {
	srv, _ := setupTestServer(t, "admin", "adminpass", 5)

	httpAddr := srv.HTTPAddr()
	client := &http.Client{}

	// 1. Try a query without auth
	reqBody, _ := json.Marshal(HTTPQueryRequest{Query: "CREATE TABLE t (id INT);"})
	req, _ := http.NewRequest("POST", "http://"+httpAddr+"/query", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401 Unauthorized, got: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 2. Perform query with correct auth
	req, _ = http.NewRequest("POST", "http://"+httpAddr+"/query", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", "adminpass")

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK, got: %d", resp.StatusCode)
	}

	var qResp HTTPQueryResponse
	_ = json.NewDecoder(resp.Body).Decode(&qResp)
	resp.Body.Close()
	if !qResp.Success {
		t.Errorf("expected query success, got error: %s", qResp.Error)
	}

	// 3. Begin Transaction over HTTP
	req, _ = http.NewRequest("POST", "http://"+httpAddr+"/tx/begin", nil)
	req.SetBasicAuth("admin", "adminpass")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("HTTP begin failed: %v", err)
	}
	var txResp HTTPTxResponse
	_ = json.NewDecoder(resp.Body).Decode(&txResp)
	resp.Body.Close()

	if !txResp.Success || txResp.TxID == "" {
		t.Fatalf("failed to begin HTTP tx: %s", txResp.Error)
	}

	txID := txResp.TxID

	// 4. Insert row inside HTTP transaction
	reqBody, _ = json.Marshal(HTTPQueryRequest{
		Query: "INSERT INTO t VALUES (777);",
		TxID:  txID,
	})
	req, _ = http.NewRequest("POST", "http://"+httpAddr+"/query", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", "adminpass")

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("HTTP insert failed: %v", err)
	}
	_ = json.NewDecoder(resp.Body).Decode(&qResp)
	resp.Body.Close()
	if !qResp.Success {
		t.Errorf("expected insert to succeed in tx: %s", qResp.Error)
	}

	// 5. Query outside HTTP transaction - should NOT see the inserted row yet (isolation)
	reqBodyOutside, _ := json.Marshal(HTTPQueryRequest{
		Query: "SELECT * FROM t;",
	})
	req, _ = http.NewRequest("POST", "http://"+httpAddr+"/query", bytes.NewReader(reqBodyOutside))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", "adminpass")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("HTTP select failed: %v", err)
	}
	_ = json.NewDecoder(resp.Body).Decode(&qResp)
	resp.Body.Close()
	if len(qResp.Results) > 0 && len(qResp.Results[0].Rows) != 0 {
		t.Errorf("expected 0 rows outside transaction before commit, got %d", len(qResp.Results[0].Rows))
	}

	// 6. Commit Transaction over HTTP
	commitReq, _ := json.Marshal(HTTPTxCommitRollbackRequest{TxID: txID})
	req, _ = http.NewRequest("POST", "http://"+httpAddr+"/tx/commit", bytes.NewReader(commitReq))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", "adminpass")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("HTTP commit failed: %v", err)
	}
	_ = json.NewDecoder(resp.Body).Decode(&txResp)
	resp.Body.Close()
	if !txResp.Success {
		t.Errorf("expected commit to succeed: %s", txResp.Error)
	}

	// 7. Query outside transaction again - should now see the row
	req, _ = http.NewRequest("POST", "http://"+httpAddr+"/query", bytes.NewReader(reqBodyOutside))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", "adminpass")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("HTTP select failed: %v", err)
	}
	_ = json.NewDecoder(resp.Body).Decode(&qResp)
	resp.Body.Close()
	if len(qResp.Results) == 0 || len(qResp.Results[0].Rows) != 1 {
		t.Errorf("expected 1 row after commit, got %d results", len(qResp.Results))
	} else {
		val := qResp.Results[0].Rows[0][0]
		if fmt.Sprintf("%v", val) != "777" {
			t.Errorf("expected row value to be 777, got %v", val)
		}
	}
}
