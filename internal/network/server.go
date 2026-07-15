package network

import (
	"bufio"
	"context"
	"crypto/rand"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/TaqsBlaze/FlamingoDB/internal/executor"
	"github.com/TaqsBlaze/FlamingoDB/internal/optimizer"
	"github.com/TaqsBlaze/FlamingoDB/internal/parser/lexer"
	"github.com/TaqsBlaze/FlamingoDB/internal/parser/parser"
	"github.com/TaqsBlaze/FlamingoDB/internal/planner"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/catalog"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/record"
	"github.com/TaqsBlaze/FlamingoDB/internal/transaction"
	"github.com/TaqsBlaze/FlamingoDB/pkg/logger"
)

//go:embed ui
var uiFS embed.FS

// Config holds network server configurations.
type Config struct {
	TCPAddr        string
	HTTPAddr       string
	Username       string
	Password       string
	DataDir        string // directory where users.json is stored
	MaxConnections int    // Connection pooling limit
}

// QueryResult represents the serialized result of a single query statement.
type QueryResult struct {
	Columns      []string `json:"columns,omitempty"`
	Rows         [][]any  `json:"rows,omitempty"`
	RowsAffected int      `json:"rows_affected"`
	Message      string   `json:"message,omitempty"`
	Error        string   `json:"error,omitempty"`
}

// Server implements the FlamingoDB network server supporting TCP and HTTP protocols.
type Server struct {
	tm        *catalog.TableManager
	log       *logger.Logger
	cfg       Config
	maxConn   int
	username  string
	password  string
	userStore   *UserStore
	policyStore *PolicyStore

	// TCP states
	tcpListener   net.Listener
	tcpWg         sync.WaitGroup
	tcpConns      map[net.Conn]struct{}
	tcpMu         sync.Mutex
	connSemaphore chan struct{}

	// HTTP state
	httpServer   *http.Server
	httpListener net.Listener

	// HTTP Transactions mapping (for stateful transaction support over stateless HTTP)
	httpTxMap map[string]*httpTxState
	httpTxMu  sync.RWMutex

	// Control states
	shutdownChan chan struct{}
	once         sync.Once
}

type httpTxState struct {
	tx       *transaction.Transaction
	lastUsed time.Time
}

// NewServer initializes a new Server.
func NewServer(cfg Config, tm *catalog.TableManager, log *logger.Logger) (*Server, error) {
	if cfg.MaxConnections <= 0 {
		cfg.MaxConnections = 100 // default connection limit
	}

	// Init user store
	usersFile := cfg.DataDir + "/users.json"
	if cfg.DataDir == "" {
		usersFile = "./users.json"
	}
	us, err := NewUserStore(usersFile, cfg.Username, cfg.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to init user store: %w", err)
	}

	policiesFile := cfg.DataDir + "/policies.json"
	if cfg.DataDir == "" {
		policiesFile = "./policies.json"
	}
	ps, err := NewPolicyStore(policiesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to init policy store: %w", err)
	}

	return &Server{
		tm:            tm,
		log:           log,
		cfg:           cfg,
		maxConn:       cfg.MaxConnections,
		username:      cfg.Username,
		password:      cfg.Password,
		userStore:     us,
		policyStore:   ps,
		tcpConns:      make(map[net.Conn]struct{}),
		connSemaphore: make(chan struct{}, cfg.MaxConnections),
		httpTxMap:     make(map[string]*httpTxState),
		shutdownChan:  make(chan struct{}),
	}, nil
}

// TCPAddr returns the actual bound TCP address (useful when port 0 is used).
func (s *Server) TCPAddr() string {
	s.tcpMu.Lock()
	defer s.tcpMu.Unlock()
	if s.tcpListener != nil {
		return s.tcpListener.Addr().String()
	}
	return ""
}

// HTTPAddr returns the actual bound HTTP address (useful when port 0 is used).
func (s *Server) HTTPAddr() string {
	s.tcpMu.Lock()
	defer s.tcpMu.Unlock()
	if s.httpListener != nil {
		return s.httpListener.Addr().String()
	}
	return ""
}

// Start binds the listeners and starts TCP and HTTP servers in the background.
func (s *Server) Start() error {
	var err error

	// 1. Start TCP Server
	if s.cfg.TCPAddr != "" {
		s.tcpListener, err = net.Listen("tcp", s.cfg.TCPAddr)
		if err != nil {
			return fmt.Errorf("failed to listen on TCP %s: %w", s.cfg.TCPAddr, err)
		}
		s.log.Info("TCP Server listening on %s", s.tcpListener.Addr().String())

		s.tcpWg.Add(1)
		go s.acceptTCPConnections()
	}

	// 2. Start HTTP Server
	if s.cfg.HTTPAddr != "" {
		s.httpListener, err = net.Listen("tcp", s.cfg.HTTPAddr)
		if err != nil {
			if s.tcpListener != nil {
				_ = s.tcpListener.Close()
			}
			return fmt.Errorf("failed to listen on HTTP %s: %w", s.cfg.HTTPAddr, err)
		}
		s.log.Info("HTTP Server listening on %s", s.httpListener.Addr().String())

		mux := http.NewServeMux()
		// Serve the admin dashboard UI – embed the whole ui dir so FileServer works
		uiSubFS, _ := fs.Sub(uiFS, "ui")
		uiFileServer := http.FileServer(http.FS(uiSubFS))
		uiHandler := func(w http.ResponseWriter, r *http.Request) {
			// If request path is for lucide.js, serve it from the file server
			if strings.HasSuffix(r.URL.Path, "/lucide.js") {
				r2 := *r
				r2.URL.Path = "/lucide.js"
				uiFileServer.ServeHTTP(w, &r2)
				return
			}
			// Strip any prefix and serve index.html for any unmatched path
			r2 := *r
			r2.URL.Path = "/"
			uiFileServer.ServeHTTP(w, &r2)
		}
		mux.HandleFunc("/", uiHandler)
		mux.HandleFunc("/ui", uiHandler)
		mux.HandleFunc("/query", s.handleHTTPQuery)
		mux.HandleFunc("/tx/begin", s.handleHTTPTxBegin)
		mux.HandleFunc("/tx/commit", s.handleHTTPTxCommit)
		mux.HandleFunc("/tx/rollback", s.handleHTTPTxRollback)
		mux.HandleFunc("/api/tables", s.handleHTTPListTables)
		mux.HandleFunc("/api/describe", s.handleHTTPDescribeTable)
		mux.HandleFunc("/api/users", s.handleHTTPUsers)
		mux.HandleFunc("/api/users/", s.handleHTTPUsersPath) // handles /api/users/:name and /api/users/:name/policy
		mux.HandleFunc("/api/policies", s.handleHTTPPolicies)
		mux.HandleFunc("/api/policies/", s.handleHTTPPoliciesPath) // handles /api/policies/:name
		mux.HandleFunc("/api/me", s.handleHTTPMe)
		mux.HandleFunc("/api/me/password", s.handleHTTPChangePassword)

		s.httpServer = &http.Server{
			Addr:    s.cfg.HTTPAddr,
			Handler: mux,
		}

		go func() {
			if err := s.httpServer.Serve(s.httpListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				s.log.Error("HTTP server failed: %v", err)
			}
		}()

		// Start HTTP transaction timeout manager in the background
		go s.httpTxCleanupLoop()
	}

	return nil
}

func (s *Server) acceptTCPConnections() {
	defer s.tcpWg.Done()

	for {
		conn, err := s.tcpListener.Accept()
		if err != nil {
			select {
			case <-s.shutdownChan:
				return
			default:
				s.log.Error("TCP accept error: %v", err)
				continue
			}
		}

		// Implement Connection Pooling semaphore
		select {
		case s.connSemaphore <- struct{}{}:
			// Connection allowed
		default:
			// Queue/pool is full: reject connection
			s.log.Warn("TCP connection rejected: connection pool exhausted (limit: %d)", s.maxConn)
			resp := TCPResponse{
				Success: false,
				Error:   "Connection pool exhausted. Maximum connection limit reached.",
			}
			_ = json.NewEncoder(conn).Encode(resp)
			_ = conn.Close()
			continue
		}

		s.tcpMu.Lock()
		s.tcpConns[conn] = struct{}{}
		s.tcpMu.Unlock()

		s.tcpWg.Add(1)
		go s.handleTCPClient(conn)
	}
}

type TCPRequest struct {
	Type     string `json:"type"`               // "auth", "query", "begin", "commit", "rollback", "close", "meta"
	Username string `json:"username,omitempty"` // credentials for auth
	Password string `json:"password,omitempty"`
	Query    string `json:"query,omitempty"`    // SQL string
	Command  string `json:"command,omitempty"`  // "list_tables", "describe_table"
}

type TCPResponse struct {
	Success      bool          `json:"success"`
	Message      string        `json:"message,omitempty"`
	Columns      []string      `json:"columns,omitempty"`
	Rows         [][]any       `json:"rows,omitempty"`
	RowsAffected int           `json:"rows_affected,omitempty"`
	Results      []QueryResult `json:"results,omitempty"` // For support of multi-statement queries
	Error        string        `json:"error,omitempty"`
}

func (s *Server) handleTCPClient(conn net.Conn) {
	defer func() {
		s.tcpMu.Lock()
		delete(s.tcpConns, conn)
		s.tcpMu.Unlock()
		_ = conn.Close()
		<-s.connSemaphore
		s.tcpWg.Done()
	}()

	s.log.Info("New TCP client connection from %s", conn.RemoteAddr())

	var tx *transaction.Transaction
	authenticated := (s.username == "" && s.password == "") // If no credentials configured, default to authenticated

	// Register rollback in case connection is severed during an active transaction
	defer func() {
		if tx != nil {
			s.log.Warn("Client disconnected with active transaction. Rolling back.")
			_ = s.tm.Rollback(tx)
		}
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var req TCPRequest
		err := json.Unmarshal(scanner.Bytes(), &req)
		if err != nil {
			s.writeTCPResponse(conn, TCPResponse{Success: false, Error: fmt.Sprintf("invalid json request: %v", err)})
			continue
		}

		// Check Authentication
		if !authenticated && strings.ToLower(req.Type) != "auth" {
			s.writeTCPResponse(conn, TCPResponse{Success: false, Error: "unauthenticated. please send an auth request first."})
			continue
		}

		switch strings.ToLower(req.Type) {
		case "auth":
			if authenticated {
				s.writeTCPResponse(conn, TCPResponse{Success: true, Message: "already authenticated"})
				continue
			}
			if req.Username == s.username && req.Password == s.password {
				authenticated = true
				s.writeTCPResponse(conn, TCPResponse{Success: true, Message: "authentication successful"})
			} else {
				s.writeTCPResponse(conn, TCPResponse{Success: false, Error: "invalid credentials"})
			}

		case "begin":
			if tx != nil {
				s.writeTCPResponse(conn, TCPResponse{Success: false, Error: "transaction already active"})
				continue
			}
			tx, err = s.tm.Begin()
			if err != nil {
				s.writeTCPResponse(conn, TCPResponse{Success: false, Error: fmt.Sprintf("failed to begin transaction: %v", err)})
			} else {
				s.writeTCPResponse(conn, TCPResponse{Success: true, Message: fmt.Sprintf("transaction %d started", tx.ID())})
			}

		case "commit":
			if tx == nil {
				s.writeTCPResponse(conn, TCPResponse{Success: false, Error: "no active transaction"})
				continue
			}
			err = s.tm.Commit(tx)
			if err != nil {
				s.writeTCPResponse(conn, TCPResponse{Success: false, Error: fmt.Sprintf("failed to commit transaction: %v", err)})
			} else {
				txID := tx.ID()
				tx = nil
				s.writeTCPResponse(conn, TCPResponse{Success: true, Message: fmt.Sprintf("transaction %d committed", txID)})
			}

		case "rollback":
			if tx == nil {
				s.writeTCPResponse(conn, TCPResponse{Success: false, Error: "no active transaction"})
				continue
			}
			err = s.tm.Rollback(tx)
			if err != nil {
				s.writeTCPResponse(conn, TCPResponse{Success: false, Error: fmt.Sprintf("failed to rollback transaction: %v", err)})
			} else {
				txID := tx.ID()
				tx = nil
				s.writeTCPResponse(conn, TCPResponse{Success: true, Message: fmt.Sprintf("transaction %d rolled back", txID)})
			}

		case "meta":
			switch req.Command {
			case "list_tables":
				tables := s.tm.ListTables()
				var rows [][]any
				for _, t := range tables {
					rows = append(rows, []any{t})
				}
				s.writeTCPResponse(conn, TCPResponse{
					Success: true,
					Columns: []string{"Table Name"},
					Rows:    rows,
				})
			case "describe_table":
				schema, err := s.tm.GetSchema(req.Query)
				if err != nil {
					s.writeTCPResponse(conn, TCPResponse{Success: false, Error: err.Error()})
					continue
				}
				var rows [][]any
				for _, col := range schema.Columns {
					rows = append(rows, []any{col.Name, fmt.Sprintf("%d", col.Type)})
				}
				s.writeTCPResponse(conn, TCPResponse{
					Success: true,
					Columns: []string{"Column Name", "Type ID"},
					Rows:    rows,
				})
			case "shutdown":
				s.writeTCPResponse(conn, TCPResponse{Success: true, Message: "Server shutting down..."})
				// Launch a goroutine to wait a tiny bit then send a signal to ourselves to trigger graceful shutdown
				go func() {
					time.Sleep(100 * time.Millisecond)
					p, _ := os.FindProcess(os.Getpid())
					p.Signal(os.Interrupt)
				}()
			default:
				s.writeTCPResponse(conn, TCPResponse{Success: false, Error: "unknown meta command"})
			}

		case "query":
			if req.Query == "" {
				s.writeTCPResponse(conn, TCPResponse{Success: false, Error: "empty query"})
				continue
			}

			results, err := s.ProcessQuery(tx, req.Query)
			if err != nil {
				s.writeTCPResponse(conn, TCPResponse{Success: false, Error: err.Error()})
				continue
			}

			var resp TCPResponse
			resp.Success = true
			resp.Results = results

			if len(results) == 1 {
				resp.Columns = results[0].Columns
				resp.Rows = results[0].Rows
				resp.RowsAffected = results[0].RowsAffected
				resp.Message = results[0].Message
				if results[0].Error != "" {
					resp.Success = false
					resp.Error = results[0].Error
				}
			}

			s.writeTCPResponse(conn, resp)

		case "close":
			s.writeTCPResponse(conn, TCPResponse{Success: true, Message: "goodbye"})
			return

		default:
			s.writeTCPResponse(conn, TCPResponse{Success: false, Error: fmt.Sprintf("unknown request type: %s", req.Type)})
		}
	}
}

func (s *Server) writeTCPResponse(conn net.Conn, resp TCPResponse) {
	_ = json.NewEncoder(conn).Encode(resp)
}

// ProcessQuery parses, plans, optimizes, and executes a SQL string.
// If multiple statements are provided, it executes them sequentially.
func (s *Server) ProcessQuery(tx *transaction.Transaction, sql string) ([]QueryResult, error) {
	l := lexer.New(sql)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("parser errors: %v", p.Errors())
	}
	if len(prog.Statements) == 0 {
		return nil, fmt.Errorf("no statements found")
	}

	var results []QueryResult
	pl := planner.New()
	exec := executor.New(s.tm)

	for _, stmt := range prog.Statements {
		node, err := pl.Plan(stmt)
		if err != nil {
			results = append(results, QueryResult{Error: fmt.Sprintf("planner error: %v", err)})
			continue
		}

		// Optimize logical plan using query optimizer
		optNode, err := optimizer.Optimize(node, s.tm)
		if err != nil {
			results = append(results, QueryResult{Error: fmt.Sprintf("optimizer error: %v", err)})
			continue
		}

		// Execute physical plan
		res, err := exec.ExecuteWithTx(tx, optNode)
		if err != nil {
			results = append(results, QueryResult{Error: err.Error()})
			continue
		}

		// Format output rows as JSON-compatible values
		var rows [][]any
		if len(res.Rows) > 0 {
			rows = make([][]any, len(res.Rows))
			for i, r := range res.Rows {
				rowVals := make([]any, len(r.Values))
				for j, v := range r.Values {
					rowVals[j] = ValueToJSON(v)
				}
				rows[i] = rowVals
			}
		}

		results = append(results, QueryResult{
			Columns:      inferColumnNames(optNode, s.tm),
			Rows:         rows,
			RowsAffected: res.RowsAffected,
			Message:      res.Message,
		})
	}

	return results, nil
}

// HTTP handlers and mechanisms

type HTTPQueryRequest struct {
	Query string `json:"query"`
	TxID  string `json:"tx_id,omitempty"`
}

type HTTPQueryResponse struct {
	Success bool          `json:"success"`
	Results []QueryResult `json:"results,omitempty"`
	Error   string        `json:"error,omitempty"`
}

type HTTPTxResponse struct {
	Success bool   `json:"success"`
	TxID    string `json:"tx_id,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type HTTPTxCommitRollbackRequest struct {
	TxID string `json:"tx_id"`
}

func (s *Server) authenticateHTTP(r *http.Request) (*DBUser, bool) {
	user, pass, ok := r.BasicAuth()
	if !ok {
		return nil, false
	}
	return s.userStore.Authenticate(user, pass)
}

func (s *Server) handleHTTPListTables(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if _, ok := s.authenticateHTTP(r); !ok {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "unauthorized"})
		return
	}
	tables := s.tm.ListTables()
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "tables": tables})
}

func (s *Server) handleHTTPDescribeTable(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if _, ok := s.authenticateHTTP(r); !ok {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "unauthorized"})
		return
	}
	tableName := r.URL.Query().Get("table")
	if tableName == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "table param required"})
		return
	}
	schema, err := s.tm.GetSchema(tableName)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}
	type colInfo struct {
		Name   string `json:"name"`
		TypeID int    `json:"type_id"`
	}
	cols := make([]colInfo, len(schema.Columns))
	for i, c := range schema.Columns {
		cols[i] = colInfo{Name: c.Name, TypeID: int(c.Type)}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "table": tableName, "columns": cols})
}

func (s *Server) handleHTTPQuery(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(HTTPQueryResponse{Success: false, Error: "only POST allowed"})
		return
	}

	dbUser, ok := s.authenticateHTTP(r)
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="FlamingoDB"`)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(HTTPQueryResponse{Success: false, Error: "unauthorized"})
		return
	}

	var req HTTPQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(HTTPQueryResponse{Success: false, Error: fmt.Sprintf("invalid json: %v", err)})
		return
	}

	if req.Query == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(HTTPQueryResponse{Success: false, Error: "query cannot be empty"})
		return
	}

	// Policy enforcement (admins bypass all checks)
	if !dbUser.IsAdmin {
		qUpper := strings.ToUpper(strings.TrimSpace(req.Query))
		if (strings.HasPrefix(qUpper, "SELECT")) && !dbUser.Policy.CanSelect {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(HTTPQueryResponse{Success: false, Error: "permission denied: SELECT not allowed"})
			return
		}
		if (strings.HasPrefix(qUpper, "INSERT")) && !dbUser.Policy.CanInsert {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(HTTPQueryResponse{Success: false, Error: "permission denied: INSERT not allowed"})
			return
		}
		if (strings.HasPrefix(qUpper, "UPDATE")) && !dbUser.Policy.CanUpdate {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(HTTPQueryResponse{Success: false, Error: "permission denied: UPDATE not allowed"})
			return
		}
		if (strings.HasPrefix(qUpper, "DELETE")) && !dbUser.Policy.CanDelete {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(HTTPQueryResponse{Success: false, Error: "permission denied: DELETE not allowed"})
			return
		}
		if (strings.HasPrefix(qUpper, "CREATE")) && !dbUser.Policy.CanCreate {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(HTTPQueryResponse{Success: false, Error: "permission denied: CREATE not allowed"})
			return
		}
		if (strings.HasPrefix(qUpper, "DROP")) && !dbUser.Policy.CanDrop {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(HTTPQueryResponse{Success: false, Error: "permission denied: DROP not allowed"})
			return
		}
	}

	var tx *transaction.Transaction
	txID := req.TxID
	if txID == "" {
		txID = r.Header.Get("X-Flamingo-Tx")
	}

	if txID != "" {
		s.httpTxMu.Lock()
		state, exists := s.httpTxMap[txID]
		if exists {
			tx = state.tx
			state.lastUsed = time.Now()
		}
		s.httpTxMu.Unlock()

		if !exists {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(HTTPQueryResponse{Success: false, Error: fmt.Sprintf("transaction %s not found or expired", txID)})
			return
		}
	}

	results, err := s.ProcessQuery(tx, req.Query)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(HTTPQueryResponse{Success: false, Error: err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(HTTPQueryResponse{Success: true, Results: results})
}

func (s *Server) handleHTTPTxBegin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(HTTPTxResponse{Success: false, Error: "only POST allowed"})
		return
	}

	if _, ok := s.authenticateHTTP(r); !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="FlamingoDB"`)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(HTTPTxResponse{Success: false, Error: "unauthorized"})
		return
	}

	tx, err := s.tm.Begin()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(HTTPTxResponse{Success: false, Error: err.Error()})
		return
	}

	txID := generateTxID()
	s.httpTxMu.Lock()
	s.httpTxMap[txID] = &httpTxState{
		tx:       tx,
		lastUsed: time.Now(),
	}
	s.httpTxMu.Unlock()

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(HTTPTxResponse{
		Success: true,
		TxID:    txID,
		Message: fmt.Sprintf("transaction %d started over HTTP", tx.ID()),
	})
}

func (s *Server) handleHTTPTxCommit(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(HTTPTxResponse{Success: false, Error: "only POST allowed"})
		return
	}

	if _, ok := s.authenticateHTTP(r); !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="FlamingoDB"`)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(HTTPTxResponse{Success: false, Error: "unauthorized"})
		return
	}

	var req HTTPTxCommitRollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(HTTPTxResponse{Success: false, Error: "invalid body"})
		return
	}

	txID := req.TxID
	s.httpTxMu.Lock()
	state, exists := s.httpTxMap[txID]
	if exists {
		delete(s.httpTxMap, txID)
	}
	s.httpTxMu.Unlock()

	if !exists {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(HTTPTxResponse{Success: false, Error: "transaction not found or expired"})
		return
	}

	err := s.tm.Commit(state.tx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(HTTPTxResponse{Success: false, Error: err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(HTTPTxResponse{
		Success: true,
		Message: fmt.Sprintf("transaction %d committed", state.tx.ID()),
	})
}

func (s *Server) handleHTTPTxRollback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(HTTPTxResponse{Success: false, Error: "only POST allowed"})
		return
	}

	if _, ok := s.authenticateHTTP(r); !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="FlamingoDB"`)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(HTTPTxResponse{Success: false, Error: "unauthorized"})
		return
	}

	var req HTTPTxCommitRollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(HTTPTxResponse{Success: false, Error: "invalid body"})
		return
	}

	txID := req.TxID
	s.httpTxMu.Lock()
	state, exists := s.httpTxMap[txID]
	if exists {
		delete(s.httpTxMap, txID)
	}
	s.httpTxMu.Unlock()

	if !exists {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(HTTPTxResponse{Success: false, Error: "transaction not found or expired"})
		return
	}

	err := s.tm.Rollback(state.tx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(HTTPTxResponse{Success: false, Error: err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(HTTPTxResponse{
		Success: true,
		Message: fmt.Sprintf("transaction %d rolled back", state.tx.ID()),
	})
}

func (s *Server) httpTxCleanupLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.shutdownChan:
			return
		case <-ticker.C:
			now := time.Now()
			s.httpTxMu.Lock()
			for txID, state := range s.httpTxMap {
				// Rollback active transaction if inactive for > 15 seconds to free locks quickly in tests
				if now.Sub(state.lastUsed) > 15*time.Second {
					s.log.Warn("HTTP transaction %s (%d) inactive. Expiring and rolling back.", txID, state.tx.ID())
					_ = s.tm.Rollback(state.tx)
					delete(s.httpTxMap, txID)
				}
			}
			s.httpTxMu.Unlock()
		}
	}

}

// ── User Management Handlers ──────────────────────────────────────────────────

// requireAdmin checks auth and asserts the caller is an admin.
func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (*DBUser, bool) {
	u, ok := s.authenticateHTTP(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "unauthorized"})
		return nil, false
	}
	if !u.IsAdmin {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "forbidden: admin required"})
		return nil, false
	}
	return u, true
}

// /api/users – dispatches GET (list), POST (create), DELETE (delete?username=X)
func (s *Server) handleHTTPUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		if _, ok := s.requireAdmin(w, r); !ok {
			return
		}
		users := s.userStore.ListUsers()
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "users": users})

	case http.MethodPost:
		if _, ok := s.requireAdmin(w, r); !ok {
			return
		}
		var req struct {
			Username   string `json:"username"`
			Password   string `json:"password"`
			IsAdmin    bool   `json:"is_admin"`
			PolicyName string `json:"policy_name"`
			Policy     Policy `json:"policy"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.Password == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "username and password required"})
			return
		}

		var policy Policy
		if req.IsAdmin {
			req.PolicyName = "Admin"
			policy = AdminPolicy()
		} else if req.PolicyName != "" {
			if np, ok := s.policyStore.Get(req.PolicyName); ok {
				policy = Policy{
					CanSelect: np.CanSelect,
					CanInsert: np.CanInsert,
					CanUpdate: np.CanUpdate,
					CanDelete: np.CanDelete,
					CanCreate: np.CanCreate,
					CanDrop:   np.CanDrop,
				}
			} else {
				policy = req.Policy
			}
		} else {
			policy = req.Policy
		}

		if err := s.userStore.CreateUser(req.Username, req.Password, req.IsAdmin, req.PolicyName, policy); err != nil {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "user created: " + req.Username})

	case http.MethodDelete:
		if _, ok := s.requireAdmin(w, r); !ok {
			return
		}
		// UI sends DELETE /api/users/:username (path segment) OR ?username=X
		username := r.URL.Query().Get("username")
		if username == "" {
			// try path: /api/users/alice
			username = strings.TrimPrefix(r.URL.Path, "/api/users/")
		}
		if username == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "username required"})
			return
		}
		if err := s.userStore.DeleteUser(username); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "user deleted: " + username})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleHTTPUsersPath handles sub-paths under /api/users/:
//   DELETE /api/users/:username          → delete user
//   PUT    /api/users/:username/policy   → update policy
func (s *Server) handleHTTPUsersPath(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Strip the /api/users/ prefix to get the remaining path
	rest := strings.TrimPrefix(r.URL.Path, "/api/users/")

	if strings.HasSuffix(rest, "/policy") {
		// PUT /api/users/:username/policy
		username := strings.TrimSuffix(rest, "/policy")
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if _, ok := s.requireAdmin(w, r); !ok {
			return
		}
		var req struct {
			IsAdmin    bool   `json:"is_admin"`
			PolicyName string `json:"policy_name"`
			Policy     Policy `json:"policy"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "invalid json"})
			return
		}

		var policy Policy
		if req.IsAdmin {
			req.PolicyName = "Admin"
			policy = AdminPolicy()
		} else if req.PolicyName != "" {
			if np, ok := s.policyStore.Get(req.PolicyName); ok {
				policy = Policy{
					CanSelect: np.CanSelect,
					CanInsert: np.CanInsert,
					CanUpdate: np.CanUpdate,
					CanDelete: np.CanDelete,
					CanCreate: np.CanCreate,
					CanDrop:   np.CanDrop,
				}
			} else {
				policy = req.Policy
			}
		} else {
			policy = req.Policy
		}

		if err := s.userStore.UpdatePolicy(username, policy, req.IsAdmin, req.PolicyName); err != nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "policy updated for: " + username})
		return
	}

	// DELETE /api/users/:username
	username := rest
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if username == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "username required"})
		return
	}
	if err := s.userStore.DeleteUser(username); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "user deleted: " + username})
}


// PUT /api/me/password – change own password
func (s *Server) handleHTTPChangePassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	u, ok := s.authenticateHTTP(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "unauthorized"})
		return
	}
	var req struct {
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NewPassword == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "new_password required"})
		return
	}
	if err := s.userStore.UpdatePassword(u.Username, req.NewPassword); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "password updated"})
}

// GET /api/me – get current user info
func (s *Server) handleHTTPMe(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	u, ok := s.authenticateHTTP(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "unauthorized"})
		return
	}
	safe := *u
	safe.Password = ""
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "user": safe})
}


func (s *Server) Close() error {
	s.once.Do(func() {
		close(s.shutdownChan)

		// 1. Close TCP Listener
		if s.tcpListener != nil {
			_ = s.tcpListener.Close()
		}

		// 2. Force close all active client connections
		s.tcpMu.Lock()
		for conn := range s.tcpConns {
			_ = conn.Close()
		}
		s.tcpConns = make(map[net.Conn]struct{})
		s.tcpMu.Unlock()

		// Wait for active TCP handler goroutines
		s.tcpWg.Wait()

		// 3. Shutdown HTTP Server
		if s.httpServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = s.httpServer.Shutdown(ctx)
		}

		// 4. Rollback any active HTTP transactions
		s.httpTxMu.Lock()
		for _, state := range s.httpTxMap {
			_ = s.tm.Rollback(state.tx)
		}
		s.httpTxMap = make(map[string]*httpTxState)
		s.httpTxMu.Unlock()
	})

	return nil
}

// Helper serialization and utilities

func generateTxID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func ValueToJSON(v record.Value) any {
	switch v.Type {
	case record.Integer:
		return v.Int
	case record.Float:
		return v.Flt
	case record.Varchar:
		return v.Str
	case record.Complex:
		return map[string]float64{"real": v.Comp.Real, "imag": v.Comp.Imag}
	case record.Vector:
		return []float64(v.Vec)
	case record.Matrix:
		return [][]float64(v.Mat)
	case record.Tensor:
		return map[string]any{"shape": v.Ten.Shape, "data": v.Ten.Data}
	case record.Point:
		return v.Pt.String()
	case record.Polygon:
		return v.Poly.String()
	default:
		return nil
	}
}

func inferColumnNames(node planner.PlanNode, tm *catalog.TableManager) []string {
	if node == nil {
		return nil
	}
	switch n := node.(type) {
	case *planner.ProjectNode:
		var names []string
		for _, f := range n.Fields {
			names = append(names, f.String())
		}
		if len(names) == 1 && names[0] == "*" {
			return inferColumnNames(n.Child, tm)
		}
		return names
	case *planner.FilterNode:
		return inferColumnNames(n.Child, tm)
	case *planner.ScanNode:
		schema, err := tm.GetSchema(n.Table)
		if err != nil {
			return nil
		}
		var names []string
		for _, col := range schema.Columns {
			names = append(names, col.Name)
		}
		return names
	}
	return nil
}

// GET /api/policies - lists all policies (admin only)
// POST /api/policies - creates or updates a policy (admin only)
func (s *Server) handleHTTPPolicies(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		if _, ok := s.requireAdmin(w, r); !ok {
			return
		}
		list := s.policyStore.List()
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "policies": list})

	case http.MethodPost, http.MethodPut:
		if _, ok := s.requireAdmin(w, r); !ok {
			return
		}
		var req NamedPolicy
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "name required"})
			return
		}
		if err := s.policyStore.Set(req.Name, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "policy saved: " + req.Name})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// DELETE /api/policies/:name - delete a named policy (admin only)
func (s *Server) handleHTTPPoliciesPath(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/api/policies/")
	if name == "" {
		name = r.URL.Query().Get("name")
	}
	if name == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "name required"})
		return
	}
	if err := s.policyStore.Delete(name); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "policy deleted: " + name})
}
