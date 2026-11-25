package network

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	mysql "github.com/go-sql-driver/mysql"
)

// MySQL imports use short-lived, read-only source connections. Credentials are
// supplied per request and are never stored by the server.
func (s *Server) handleMySQLImport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fail := func(status int, msg string) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": msg})
	}
	if r.Method != http.MethodPost {
		fail(http.StatusMethodNotAllowed, "only POST allowed")
		return
	}
	user, ok := s.authenticateHTTP(r)
	if !ok {
		fail(http.StatusUnauthorized, "unauthorized")
		return
	}
	// A remote connection originates from the server, so only administrators can initiate it.
	if !user.IsAdmin {
		fail(http.StatusForbidden, "administrator access is required to connect to MySQL")
		return
	}
	var req struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Database string `json:"database"`
		Username string `json:"username"`
		Password string `json:"password"`
		TLS      bool   `json:"tls"`
		Table    string `json:"table"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		fail(400, "invalid request")
		return
	}
	if req.Host == "" || req.Database == "" || req.Username == "" || req.Port < 1 || req.Port > 65535 {
		fail(400, "host, port, database and username are required")
		return
	}
	cfg := mysql.NewConfig()
	cfg.User = req.Username
	cfg.Passwd = req.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(req.Host, strconv.Itoa(req.Port))
	cfg.DBName = req.Database
	cfg.Timeout = 8 * time.Second
	cfg.ReadTimeout = 15 * time.Second
	cfg.ParseTime = false
	if req.TLS {
		cfg.TLSConfig = "true"
	}
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		fail(400, "invalid connection configuration")
		return
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		fail(502, "MySQL connection failed: "+err.Error())
		return
	}
	// Resolve the name from information_schema, then quote it for the read query.
	rows, err := db.QueryContext(ctx, "SELECT TABLE_NAME FROM information_schema.tables WHERE table_schema = ? AND table_type = 'BASE TABLE' ORDER BY TABLE_NAME", req.Database)
	if err != nil {
		fail(502, "could not list MySQL tables: "+err.Error())
		return
	}
	var tables []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			break
		}
		tables = append(tables, name)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		fail(502, "could not read MySQL table list: "+err.Error())
		return
	}
	if req.Table == "" {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "tables": tables})
		return
	}
	found := false
	for _, name := range tables {
		if name == req.Table {
			found = true
			break
		}
	}
	if !found {
		fail(400, "source table not found")
		return
	}
	quoted := "`" + strings.ReplaceAll(req.Table, "`", "``") + "`"
	source, err := db.QueryContext(ctx, "SELECT * FROM "+quoted+" LIMIT 10001")
	if err != nil {
		fail(502, "could not read MySQL table: "+err.Error())
		return
	}
	defer source.Close()
	columnTypes, err := source.ColumnTypes()
	if err != nil {
		fail(502, "could not read source column types")
		return
	}
	for _, column := range columnTypes {
		t := strings.ToUpper(column.DatabaseTypeName())
		if strings.Contains(t, "BLOB") || t == "BINARY" || t == "VARBINARY" || t == "BIT" || t == "GEOMETRY" {
			fail(400, fmt.Sprintf("column %q has unsupported binary type %s", column.Name(), t))
			return
		}
	}
	headers, err := source.Columns()
	if err != nil {
		fail(502, "could not read columns")
		return
	}
	data := make([][]any, 0)
	totalBytes := 0
	for source.Next() {
		if len(data) >= 10000 {
			fail(400, "source table exceeds the 10,000 row import limit")
			return
		}
		vals := make([]any, len(headers))
		ptr := make([]any, len(headers))
		for i := range vals {
			ptr[i] = &vals[i]
		}
		if err := source.Scan(ptr...); err != nil {
			fail(502, "could not read source row: "+err.Error())
			return
		}
		for i, v := range vals {
			if n, ok := v.(int64); ok {
				vals[i] = strconv.FormatInt(n, 10)
			}
			if v == nil {
				fail(400, fmt.Sprintf("row %d column %q is NULL; NULL values are not supported by this importer", len(data)+1, headers[i]))
				return
			}
			if b, ok := v.([]byte); ok {
				if !utf8.Valid(b) || strings.ContainsRune(string(b), 0) {
					fail(400, fmt.Sprintf("column %q contains binary data; binary values are not supported", headers[i]))
					return
				}
				vals[i] = string(b)
				totalBytes += len(b)
			}
		}
		if totalBytes > 16<<20 {
			fail(400, "source data exceeds the 16 MiB import limit")
			return
		}
		data = append(data, vals)
	}
	if err := source.Err(); err != nil {
		fail(502, "could not read MySQL rows: "+err.Error())
		return
	}
	if len(data) == 0 {
		fail(400, "source table has no rows to import")
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "headers": headers, "rows": data})
}
