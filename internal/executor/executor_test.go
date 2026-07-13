package executor_test

import (
	"path/filepath"
	"testing"

	"flamingodb/internal/executor"
	"flamingodb/internal/parser/lexer"
	"flamingodb/internal/parser/parser"
	"flamingodb/internal/planner"
	"flamingodb/internal/storage/catalog"
	"flamingodb/internal/storage/disk"
	"flamingodb/internal/storage/pager"
)

// setupExecutor creates a clean in-memory-backed executor for each test.
func setupExecutor(t *testing.T) *executor.Executor {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "exec_test.db")
	pageSize := uint32(4096)

	dm, err := disk.NewDiskManager(dbPath, pageSize)
	if err != nil {
		t.Fatalf("failed to create disk manager: %v", err)
	}
	t.Cleanup(func() { dm.Close() })

	p, err := pager.New(dm, pageSize)
	if err != nil {
		t.Fatalf("failed to create pager: %v", err)
	}

	tm, err := catalog.NewTableManager(p)
	if err != nil {
		t.Fatalf("failed to create table manager: %v", err)
	}
	t.Cleanup(func() { tm.Close() })

	return executor.New(tm)
}

// execSQL is a helper that parses a SQL string and executes it.
func execSQL(t *testing.T, exec *executor.Executor, sql string) *executor.Result {
	t.Helper()
	l := lexer.New(sql)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors for %q: %v", sql, p.Errors())
	}
	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}

	pl := planner.New()
	node, err := pl.Plan(prog.Statements[0])
	if err != nil {
		t.Fatalf("planner error for %q: %v", sql, err)
	}

	result, err := exec.Execute(node)
	if err != nil {
		t.Fatalf("executor error for %q: %v", sql, err)
	}
	return result
}

func TestExecuteCreateTable(t *testing.T) {
	exec := setupExecutor(t)
	result := execSQL(t, exec, "CREATE TABLE planets (id INT, name VARCHAR, radius FLOAT);")
	if result.Message == "" {
		t.Error("expected a message for CREATE TABLE, got empty string")
	}
}

func TestExecuteInsertAndScan(t *testing.T) {
	exec := setupExecutor(t)

	execSQL(t, exec, "CREATE TABLE stars (id INT, name VARCHAR, magnitude FLOAT);")
	execSQL(t, exec, "INSERT INTO stars VALUES (1, 'Sirius', -1.46);")
	execSQL(t, exec, "INSERT INTO stars VALUES (2, 'Canopus', -0.74);")
	execSQL(t, exec, "INSERT INTO stars VALUES (3, 'Rigel', 0.13);")

	result := execSQL(t, exec, "SELECT * FROM stars;")

	if len(result.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(result.Rows))
	}
	if result.Rows[0].Values[1].Str != "Sirius" {
		t.Errorf("expected Sirius, got %s", result.Rows[0].Values[1].Str)
	}
	if result.Rows[1].Values[1].Str != "Canopus" {
		t.Errorf("expected Canopus, got %s", result.Rows[1].Values[1].Str)
	}
	if result.Rows[2].Values[1].Str != "Rigel" {
		t.Errorf("expected Rigel, got %s", result.Rows[2].Values[1].Str)
	}
}

func TestExecuteSelectWithFilter(t *testing.T) {
	exec := setupExecutor(t)

	execSQL(t, exec, "CREATE TABLE elements (id INT, symbol VARCHAR, atomic_weight FLOAT);")
	execSQL(t, exec, "INSERT INTO elements VALUES (1, 'H', 1.008);")
	execSQL(t, exec, "INSERT INTO elements VALUES (6, 'C', 12.011);")
	execSQL(t, exec, "INSERT INTO elements VALUES (8, 'O', 15.999);")
	execSQL(t, exec, "INSERT INTO elements VALUES (26, 'Fe', 55.845);")

	result := execSQL(t, exec, "SELECT * FROM elements WHERE id = 6;")
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.Rows[0].Values[1].Str != "C" {
		t.Errorf("expected symbol 'C', got %s", result.Rows[0].Values[1].Str)
	}
}

func TestExecuteSelectProjection(t *testing.T) {
	exec := setupExecutor(t)

	execSQL(t, exec, "CREATE TABLE sensors (id INT, label VARCHAR, value FLOAT);")
	execSQL(t, exec, "INSERT INTO sensors VALUES (1, 'temperature', 36.6);")
	execSQL(t, exec, "INSERT INTO sensors VALUES (2, 'pressure', 101.3);")

	result := execSQL(t, exec, "SELECT label FROM sensors;")
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result.Rows))
	}
	// Projected rows should only have 1 value (label)
	if len(result.Rows[0].Values) != 1 {
		t.Fatalf("expected 1 projected column, got %d", len(result.Rows[0].Values))
	}
	if result.Rows[0].Values[0].Str != "temperature" {
		t.Errorf("expected 'temperature', got %s", result.Rows[0].Values[0].Str)
	}
}

func TestExecuteInsertRowsAffected(t *testing.T) {
	exec := setupExecutor(t)
	execSQL(t, exec, "CREATE TABLE log (id INT, msg VARCHAR, level FLOAT);")
	result := execSQL(t, exec, "INSERT INTO log VALUES (1, 'startup', 0.0);")
	if result.RowsAffected != 1 {
		t.Errorf("expected RowsAffected=1, got %d", result.RowsAffected)
	}
}
