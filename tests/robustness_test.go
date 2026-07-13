package tests

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"flamingodb/internal/executor"
	"flamingodb/internal/index/btree"
	"flamingodb/internal/parser/lexer"
	"flamingodb/internal/parser/parser"
	"flamingodb/internal/planner"
	"flamingodb/internal/storage/catalog"
	"flamingodb/internal/storage/disk"
	"flamingodb/internal/storage/page"
	"flamingodb/internal/storage/pager"
	"flamingodb/internal/storage/record"
)

// setupExecutor creates a clean in-memory-backed executor for each test.
func setupExecutor(t *testing.T) *executor.Executor {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "robust_test.db")
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

	return executor.New(tm)
}

// execSQL is a helper that parses a SQL string and executes it, failing the test on error.
func execSQL(t *testing.T, exec *executor.Executor, sql string) *executor.Result {
	t.Helper()
	res, err := execSQLWithError(exec, sql)
	if err != nil {
		t.Fatalf("execSQL failed for %q: %v", sql, err)
	}
	return res
}

// execSQLWithError parses and plans a query, executing it and returning errors.
func execSQLWithError(exec *executor.Executor, sql string) (*executor.Result, error) {
	l := lexer.New(sql)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("parser errors: %v", p.Errors())
	}
	if len(prog.Statements) != 1 {
		return nil, fmt.Errorf("expected 1 statement, got %d", len(prog.Statements))
	}

	pl := planner.New()
	node, err := pl.Plan(prog.Statements[0])
	if err != nil {
		return nil, fmt.Errorf("planner error: %w", err)
	}

	return exec.Execute(node)
}

// TestRobustnessCaseInsensitivity verifies keyword case insensitivity across DDL, DML, and queries.
func TestRobustnessCaseInsensitivity(t *testing.T) {
	exec := setupExecutor(t)

	execSQL(t, exec, "cReAtE tAbLe test_case (id InT, val FlOaT, desc VaRcHaR);")
	execSQL(t, exec, "iNsErT iNtO test_case VaLuEs (1, 1.23, 'Case test');")

	result := execSQL(t, exec, "sElEcT * fRoM test_case wHeRe id = 1;")
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.Rows[0].Values[2].Str != "Case test" {
		t.Errorf("expected 'Case test', got %s", result.Rows[0].Values[2].Str)
	}
}

// TestRobustnessNegativeNumbersAndZeros verifies constant-folding of negative numeric values and zero logic.
func TestRobustnessNegativeNumbersAndZeros(t *testing.T) {
	exec := setupExecutor(t)

	execSQL(t, exec, "CREATE TABLE temp (id INT, val FLOAT, label VARCHAR);")
	execSQL(t, exec, "INSERT INTO temp VALUES (-100, -99.99, 'both negative');")
	execSQL(t, exec, "INSERT INTO temp VALUES (0, 0.0, 'both zero');")
	execSQL(t, exec, "INSERT INTO temp VALUES (-1, 0.000001, 'negative int positive float');")

	result := execSQL(t, exec, "SELECT * FROM temp WHERE id = -100;")
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.Rows[0].Values[1].Flt != -99.99 {
		t.Errorf("expected -99.99, got %f", result.Rows[0].Values[1].Flt)
	}

	result2 := execSQL(t, exec, "SELECT * FROM temp WHERE val < 0.0;")
	if len(result2.Rows) != 1 {
		t.Fatalf("expected 1 row matching val < 0.0, got %d", len(result2.Rows))
	}
}

// TestRobustnessStorageLimits tests record storage size limits.
func TestRobustnessStorageLimits(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "limits.db")
	pageSize := uint32(4096)

	dm, err := disk.NewDiskManager(dbPath, pageSize)
	if err != nil {
		t.Fatalf("failed to create disk manager: %v", err)
	}
	defer dm.Close()

	p, err := pager.New(dm, pageSize)
	if err != nil {
		t.Fatalf("failed to create pager: %v", err)
	}

	tm, err := catalog.NewTableManager(p)
	if err != nil {
		t.Fatalf("failed to create table manager: %v", err)
	}

	schema := record.NewSchema([]record.Column{
		{Name: "data", Type: record.Varchar},
	})

	err = tm.CreateTable("limits", schema)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Max string length calculation:
	// PageSize = 4096. PageHeaderSize = 12. Max totalSize = 4084.
	// totalSize = recordSize + 4. So max recordSize = 4080.
	// recordSize = 4 (string length prefix) + len(string).
	// So max len(string) = 4076.
	maxString := strings.Repeat("A", 4076)
	recMax := &record.Record{Values: []record.Value{{Type: record.Varchar, Str: maxString}}}
	err = tm.InsertRecord("limits", recMax)
	if err != nil {
		t.Fatalf("expected string of length 4076 to fit, got: %v", err)
	}

	tooLargeString := strings.Repeat("B", 4077)
	recTooLarge := &record.Record{Values: []record.Value{{Type: record.Varchar, Str: tooLargeString}}}
	err = tm.InsertRecord("limits", recTooLarge)
	if err == nil {
		t.Fatalf("expected string of length 4077 to fail with ErrRecordTooLarge, but it succeeded")
	}
}

// TestRobustnessMultiPageHeap verifies multi-page table expansions and retrieval correctness.
func TestRobustnessMultiPageHeap(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "multipage.db")
	pageSize := uint32(4096)

	dm, err := disk.NewDiskManager(dbPath, pageSize)
	if err != nil {
		t.Fatalf("failed to create disk manager: %v", err)
	}
	defer dm.Close()

	p, err := pager.New(dm, pageSize)
	if err != nil {
		t.Fatalf("failed to create pager: %v", err)
	}

	tm, err := catalog.NewTableManager(p)
	if err != nil {
		t.Fatalf("failed to create table manager: %v", err)
	}

	schema := record.NewSchema([]record.Column{
		{Name: "id", Type: record.Integer},
		{Name: "content", Type: record.Varchar},
	})

	err = tm.CreateTable("multipage", schema)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	const numRecords = 300
	for i := int32(0); i < numRecords; i++ {
		content := fmt.Sprintf("Record number %03d - some extra payload to fill up the pages and trigger multiple page allocations...", i)
		rec := &record.Record{Values: []record.Value{
			{Type: record.Integer, Int: i},
			{Type: record.Varchar, Str: content},
		}}
		if err := tm.InsertRecord("multipage", rec); err != nil {
			t.Fatalf("failed to insert record %d: %v", i, err)
		}
	}

	records, err := tm.ReadRecords("multipage")
	if err != nil {
		t.Fatalf("failed to read records: %v", err)
	}

	if len(records) != numRecords {
		t.Fatalf("expected %d records, got %d", numRecords, len(records))
	}

	for i := int32(0); i < numRecords; i++ {
		if records[i].Values[0].Int != i {
			t.Errorf("mismatch on index %d: expected ID %d, got %d", i, i, records[i].Values[0].Int)
		}
		expectedContent := fmt.Sprintf("Record number %03d - some extra payload to fill up the pages and trigger multiple page allocations...", i)
		if records[i].Values[1].Str != expectedContent {
			t.Errorf("mismatch on index %d: expected content %q, got %q", i, expectedContent, records[i].Values[1].Str)
		}
	}
}

// TestRobustnessFiltersAndComparisons tests all filter comparison operations in Executor.
func TestRobustnessFiltersAndComparisons(t *testing.T) {
	exec := setupExecutor(t)

	execSQL(t, exec, "CREATE TABLE data (id INT, val FLOAT, name VARCHAR);")
	execSQL(t, exec, "INSERT INTO data VALUES (1, 10.5, 'apple');")
	execSQL(t, exec, "INSERT INTO data VALUES (2, 20.0, 'banana');")
	execSQL(t, exec, "INSERT INTO data VALUES (3, 30.5, 'cherry');")
	execSQL(t, exec, "INSERT INTO data VALUES (4, 40.0, 'date');")

	tests := []struct {
		query        string
		expectedRows int
		firstID      int32
	}{
		// INT filters
		{"SELECT * FROM data WHERE id = 3;", 1, 3},
		{"SELECT * FROM data WHERE id != 3;", 3, 1},
		{"SELECT * FROM data WHERE id < 3;", 2, 1},
		{"SELECT * FROM data WHERE id > 3;", 1, 4},
		{"SELECT * FROM data WHERE id <= 3;", 3, 1},
		{"SELECT * FROM data WHERE id >= 3;", 2, 3},

		// FLOAT filters
		{"SELECT * FROM data WHERE val = 20.0;", 1, 2},
		{"SELECT * FROM data WHERE val != 20.0;", 3, 1},
		{"SELECT * FROM data WHERE val < 20.0;", 1, 1},
		{"SELECT * FROM data WHERE val > 20.0;", 2, 3},
		{"SELECT * FROM data WHERE val <= 20.0;", 2, 1},
		{"SELECT * FROM data WHERE val >= 20.0;", 3, 2},

		// VARCHAR filters
		{"SELECT * FROM data WHERE name = 'banana';", 1, 2},
		{"SELECT * FROM data WHERE name != 'banana';", 3, 1},

		// No matches
		{"SELECT * FROM data WHERE id = 999;", 0, 0},
		{"SELECT * FROM data WHERE val > 100.0;", 0, 0},
		{"SELECT * FROM data WHERE name = 'non-existent';", 0, 0},
	}

	for _, tt := range tests {
		res := execSQL(t, exec, tt.query)
		if len(res.Rows) != tt.expectedRows {
			t.Errorf("query %q: expected %d rows, got %d", tt.query, tt.expectedRows, len(res.Rows))
		}
		if tt.expectedRows > 0 {
			if res.Rows[0].Values[0].Int != tt.firstID {
				t.Errorf("query %q: expected first row ID to be %d, got %d", tt.query, tt.firstID, res.Rows[0].Values[0].Int)
			}
		}
	}
}

// TestRobustnessProjectionAndOrder checks projection handling, columns subsets, and ordering.
func TestRobustnessProjectionAndOrder(t *testing.T) {
	exec := setupExecutor(t)

	execSQL(t, exec, "CREATE TABLE test_proj (id INT, price FLOAT, label VARCHAR);")
	execSQL(t, exec, "INSERT INTO test_proj VALUES (42, 99.9, 'item-42');")

	res1 := execSQL(t, exec, "SELECT label, id FROM test_proj;")
	if len(res1.Rows) != 1 {
		t.Fatalf("expected 1 row")
	}
	if len(res1.Rows[0].Values) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(res1.Rows[0].Values))
	}
	if res1.Rows[0].Values[0].Str != "item-42" || res1.Rows[0].Values[1].Int != 42 {
		t.Errorf("projection label, id failed: %+v", res1.Rows[0].Values)
	}

	res2 := execSQL(t, exec, "SELECT price, label, id FROM test_proj;")
	if len(res2.Rows) != 1 {
		t.Fatalf("expected 1 row")
	}
	if len(res2.Rows[0].Values) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(res2.Rows[0].Values))
	}
	if res2.Rows[0].Values[0].Flt != 99.9 || res2.Rows[0].Values[1].Str != "item-42" || res2.Rows[0].Values[2].Int != 42 {
		t.Errorf("projection price, label, id failed: %+v", res2.Rows[0].Values)
	}
}

// TestRobustnessDatabaseRestart verifies persistent catalog and data storage on a full cold restart.
func TestRobustnessDatabaseRestart(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "restart.db")
	pageSize := uint32(4096)

	// Phase 1: Write and close
	{
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

		exec := executor.New(tm)
		execSQL(t, exec, "CREATE TABLE physics (particle VARCHAR, mass FLOAT, charge INT);")
		execSQL(t, exec, "INSERT INTO physics VALUES ('Electron', 0.511, -1);")
		execSQL(t, exec, "INSERT INTO physics VALUES ('Proton', 938.272, 1);")

		if err := p.FlushAll(); err != nil {
			t.Fatalf("failed to flush pager: %v", err)
		}
		dm.Close()
	}

	// Phase 2: Reopen and check
	{
		dm, err := disk.NewDiskManager(dbPath, pageSize)
		if err != nil {
			t.Fatalf("failed to create disk manager: %v", err)
		}
		defer dm.Close()

		p, err := pager.New(dm, pageSize)
		if err != nil {
			t.Fatalf("failed to create pager: %v", err)
		}
		tm, err := catalog.NewTableManager(p)
		if err != nil {
			t.Fatalf("failed to create table manager: %v", err)
		}

		exec := executor.New(tm)
		result := execSQL(t, exec, "SELECT * FROM physics WHERE charge = -1;")
		if len(result.Rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(result.Rows))
		}
		if result.Rows[0].Values[0].Str != "Electron" || result.Rows[0].Values[1].Flt != 0.511 {
			t.Errorf("data mismatch after reload: %+v", result.Rows[0].Values)
		}
	}
}

// TestRobustnessBTreeSplitsAndRanges performs stress tests and split validation for B+Tree indexes.
func TestRobustnessBTreeSplitsAndRanges(t *testing.T) {
	tempDir := t.TempDir()
	pageSize := uint32(512) // small page size to force splits easily

	dm, err := disk.NewDiskManager(filepath.Join(tempDir, "btree.db"), pageSize)
	if err != nil {
		t.Fatalf("failed to create disk manager: %v", err)
	}
	defer dm.Close()

	p, err := pager.New(dm, pageSize)
	if err != nil {
		t.Fatalf("failed to create pager: %v", err)
	}

	// 1. Integer Keys
	{
		tree, err := btree.New(p, pageSize, btree.KeyInt)
		if err != nil {
			t.Fatalf("failed to create btree: %v", err)
		}

		// Insert 150 integer keys (forces multiple leaf and internal splits)
		for i := int32(1); i <= 150; i++ {
			err := tree.Insert(btree.Key{Type: btree.KeyInt, IVal: i}, page.PageID(i*10))
			if err != nil {
				t.Fatalf("failed to insert key %d: %v", i, err)
			}
		}

		// Check search finds all items
		for i := int32(1); i <= 150; i++ {
			val, err := tree.Search(btree.Key{Type: btree.KeyInt, IVal: i})
			if err != nil {
				t.Fatalf("failed to search key %d: %v", i, err)
			}
			if val != page.PageID(i*10) {
				t.Errorf("expected page %d, got %d", i*10, val)
			}
		}

		// Range scan check
		res, err := tree.RangeScan(
			btree.Key{Type: btree.KeyInt, IVal: 50},
			btree.Key{Type: btree.KeyInt, IVal: 100},
		)
		if err != nil {
			t.Fatalf("range scan failed: %v", err)
		}
		if len(res) != 51 {
			t.Errorf("expected 51 keys in range [50, 100], got %d", len(res))
		}
	}

	// 2. Float Keys
	{
		tree, err := btree.New(p, pageSize, btree.KeyFloat)
		if err != nil {
			t.Fatalf("failed to create float btree: %v", err)
		}

		floats := []float64{1.5, -0.5, 9.9, -15.2, 0.0, 3.1415, 2.718}
		for i, f := range floats {
			tree.Insert(btree.Key{Type: btree.KeyFloat, FVal: f}, page.PageID(i+1))
		}

		// Inverted range
		res, err := tree.RangeScan(
			btree.Key{Type: btree.KeyFloat, FVal: 5.0},
			btree.Key{Type: btree.KeyFloat, FVal: 0.0},
		)
		if err != nil {
			t.Fatalf("range scan error: %v", err)
		}
		if len(res) != 0 {
			t.Errorf("expected empty range scan, got %d results", len(res))
		}
	}
}

// TestRobustnessErrorHandling verifies system behavior on erroneous input scenarios.
func TestRobustnessErrorHandling(t *testing.T) {
	exec := setupExecutor(t)

	// 1. Duplicate table creation error
	execSQL(t, exec, "CREATE TABLE dup (id INT);")
	_, err := execSQLWithError(exec, "CREATE TABLE dup (id INT);")
	if err == nil {
		t.Error("expected error when creating duplicate table, got nil")
	}

	// 2. Non-existent table query
	_, err = execSQLWithError(exec, "SELECT * FROM non_existent;")
	if err == nil {
		t.Error("expected error querying non-existent table, got nil")
	}

	// 3. Non-existent column query
	_, err = execSQLWithError(exec, "SELECT missing_col FROM dup;")
	if err == nil {
		t.Error("expected error querying non-existent column, got nil")
	}

	// 4. Operator type mismatch
	_, err = execSQLWithError(exec, "SELECT * FROM dup WHERE id = 'some string';")
	if err == nil {
		t.Error("expected error for type mismatch in filter, got nil")
	}

	// 5. Insert column count mismatch
	execSQL(t, exec, "CREATE TABLE multi (a INT, b FLOAT, c VARCHAR);")
	_, err = execSQLWithError(exec, "INSERT INTO multi VALUES (1, 2.0);")
	if err == nil {
		t.Error("expected error for insert value count mismatch, got nil")
	}

	// 6. Insert type mismatch
	_, err = execSQLWithError(exec, "INSERT INTO multi VALUES (1, 'string instead of float', 'hello');")
	if err == nil {
		t.Error("expected error for insert type mismatch, got nil")
	}
}
