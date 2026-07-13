package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TaqsBlaze/FlamingoDB/internal/executor"
	"github.com/TaqsBlaze/FlamingoDB/internal/index/btree"
	"github.com/TaqsBlaze/FlamingoDB/internal/parser/lexer"
	"github.com/TaqsBlaze/FlamingoDB/internal/parser/parser"
	"github.com/TaqsBlaze/FlamingoDB/internal/planner"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/catalog"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/disk"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/page"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/pager"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/record"
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
	t.Cleanup(func() { tm.Close() })

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
	defer tm.Close()

	schema := record.NewSchema([]record.Column{
		{Name: "data", Type: record.Varchar},
	})

	err = tm.CreateTable(nil, "limits", schema)
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
	err = tm.InsertRecord(nil, "limits", recMax)
	if err != nil {
		t.Fatalf("expected string of length 4076 to fit, got: %v", err)
	}

	tooLargeString := strings.Repeat("B", 4077)
	recTooLarge := &record.Record{Values: []record.Value{{Type: record.Varchar, Str: tooLargeString}}}
	err = tm.InsertRecord(nil, "limits", recTooLarge)
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
	defer tm.Close()

	schema := record.NewSchema([]record.Column{
		{Name: "id", Type: record.Integer},
		{Name: "content", Type: record.Varchar},
	})

	err = tm.CreateTable(nil, "multipage", schema)
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
		if err := tm.InsertRecord(nil, "multipage", rec); err != nil {
			t.Fatalf("failed to insert record %d: %v", i, err)
		}
	}

	records, err := tm.ReadRecords(nil, "multipage")
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
		tm.Close()
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
		defer tm.Close()

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

// TestRobustnessTransactions verifies transaction commit, rollback, and catalog rollback isolation.
func TestRobustnessTransactions(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "tx_test.db")
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
	defer tm.Close()

	schema := record.NewSchema([]record.Column{
		{Name: "id", Type: record.Integer},
		{Name: "name", Type: record.Varchar},
	})

	// 1. Commit DDL + DML
	tx, err := tm.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	err = tm.CreateTable(tx, "users", schema)
	if err != nil {
		t.Fatalf("failed to create table in transaction: %v", err)
	}

	err = tm.InsertRecord(tx, "users", &record.Record{Values: []record.Value{
		{Type: record.Integer, Int: 1},
		{Type: record.Varchar, Str: "Alice"},
	}})
	if err != nil {
		t.Fatalf("failed to insert record in transaction: %v", err)
	}

	if err := tm.Commit(tx); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Verify committed record is readable
	records, err := tm.ReadRecords(nil, "users")
	if err != nil {
		t.Fatalf("failed to read records: %v", err)
	}
	if len(records) != 1 || records[0].Values[1].Str != "Alice" {
		t.Fatalf("record not committed properly: %+v", records)
	}

	// 2. Rollback DML
	tx2, err := tm.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction 2: %v", err)
	}

	err = tm.InsertRecord(tx2, "users", &record.Record{Values: []record.Value{
		{Type: record.Integer, Int: 2},
		{Type: record.Varchar, Str: "Bob"},
	}})
	if err != nil {
		t.Fatalf("failed to insert inside transaction 2: %v", err)
	}

	if err := tm.Rollback(tx2); err != nil {
		t.Fatalf("failed to rollback: %v", err)
	}

	// Verify Bob is not in the database
	records, err = tm.ReadRecords(nil, "users")
	if err != nil {
		t.Fatalf("failed to read records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record (Bob should have been rolled back), got %d", len(records))
	}

	// 3. Rollback DDL (Table Creation Rollback)
	tx3, err := tm.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction 3: %v", err)
	}

	err = tm.CreateTable(tx3, "rolled_back_table", schema)
	if err != nil {
		t.Fatalf("failed to create table inside transaction 3: %v", err)
	}

	if err := tm.Rollback(tx3); err != nil {
		t.Fatalf("failed to rollback transaction 3: %v", err)
	}

	// Table should be gone from Catalog
	_, err = tm.GetSchema("rolled_back_table")
	if err == nil {
		t.Fatalf("expected rolled_back_table to be absent from catalog")
	}
}

// TestRobustnessCrashRecovery simulates a physical database crash and recovery using committed WAL logs.
func TestRobustnessCrashRecovery(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "crash.db")
	pageSize := uint32(4096)

	// Step 1: Create table and record, commit it normally
	{
		dm, _ := disk.NewDiskManager(dbPath, pageSize)
		p, _ := pager.New(dm, pageSize)
		tm, _ := catalog.NewTableManager(p)

		schema := record.NewSchema([]record.Column{
			{Name: "id", Type: record.Integer},
			{Name: "val", Type: record.Varchar},
		})
		tm.CreateTable(nil, "backup", schema)
		tm.InsertRecord(nil, "backup", &record.Record{Values: []record.Value{
			{Type: record.Integer, Int: 10},
			{Type: record.Varchar, Str: "initial state"},
		}})

		p.FlushAll()
		tm.Close()
		dm.Close()
	}

	// Step 2: Make a backup copy of the database file (simulates old disk state before write-back)
	backupPath := filepath.Join(tempDir, "crash.db.backup")
	data, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("failed to read db file: %v", err)
	}
	if err := os.WriteFile(backupPath, data, 0666); err != nil {
		t.Fatalf("failed to write backup: %v", err)
	}

	// Step 3: Open database, perform a transaction, commit it.
	// This writes committed pages to WAL, but we will restore the backup database file afterwards
	// to simulate that the database file was not updated before the crash occurred.
	{
		dm, _ := disk.NewDiskManager(dbPath, pageSize)
		p, _ := pager.New(dm, pageSize)
		tm, _ := catalog.NewTableManager(p)

		tx, _ := tm.Begin()
		tm.InsertRecord(tx, "backup", &record.Record{Values: []record.Value{
			{Type: record.Integer, Int: 20},
			{Type: record.Varchar, Str: "committed in WAL but crashed before page flush"},
		}})
		tm.Commit(tx)

		tm.Close()
		dm.Close()
	}

	// Restore the backup database file, simulating that the database file is stale (missing the commit)
	// but the WAL file is intact with the committed update log records.
	if err := os.WriteFile(dbPath, data, 0666); err != nil {
		t.Fatalf("failed to restore backup: %v", err)
	}

	// Step 4: Reopen the database. The TableManager constructor automatically runs txMgr.Recover(),
	// which must read the WAL, identify the committed transaction, and replay the pages to the database file.
	{
		dm, _ := disk.NewDiskManager(dbPath, pageSize)
		p, _ := pager.New(dm, pageSize)
		tm, err := catalog.NewTableManager(p)
		if err != nil {
			t.Fatalf("failed to reopen: %v", err)
		}
		defer tm.Close()
		defer dm.Close()

		// Read records — it should contain the recovered committed record from the WAL!
		records, err := tm.ReadRecords(nil, "backup")
		if err != nil {
			t.Fatalf("failed to read: %v", err)
		}

		if len(records) != 2 {
			t.Fatalf("expected 2 records after recovery, got %d", len(records))
		}

		if records[1].Values[0].Int != 20 || records[1].Values[1].Str != "committed in WAL but crashed before page flush" {
			t.Errorf("recovery failed to replay committed transaction: %+v", records[1].Values)
		}
	}
}
