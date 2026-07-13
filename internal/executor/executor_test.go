package executor_test

import (
	"path/filepath"
	"testing"

	"github.com/TaqsBlaze/FlamingoDB/internal/datatypes"
	"github.com/TaqsBlaze/FlamingoDB/internal/executor"
	"github.com/TaqsBlaze/FlamingoDB/internal/parser/lexer"
	"github.com/TaqsBlaze/FlamingoDB/internal/parser/parser"
	"github.com/TaqsBlaze/FlamingoDB/internal/planner"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/catalog"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/disk"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/pager"
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


func TestExecuteScientificTypes(t *testing.T) {
	exec := setupExecutor(t)

	execSQL(t, exec, "CREATE TABLE sci (id INT, c COMPLEX, v VECTOR, m MATRIX, ten TENSOR);")
	
	// Test basic INSERT with scientific literals
	execSQL(t, exec, "INSERT INTO sci VALUES (1, 1.2+3.4i, [1.0, 2.5, 3.0], [[1, 2], [3, 4]], [[[1.0, 2.0]], [[3.0, 4.0]]]);")
	execSQL(t, exec, "INSERT INTO sci VALUES (2, -4.5i, [0.0, -1.0], [[5, 6, 7]], [[[9.0]]]);")
	
	// Test SELECT
	result := execSQL(t, exec, "SELECT * FROM sci;")
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result.Rows))
	}

	// Verify Row 1
	row1 := result.Rows[0]
	// Complex: 1.2+3.4i
	if !row1.Values[1].Comp.Equals(datatypes.Complex{Real: 1.2, Imag: 3.4}) {
		t.Errorf("row1 complex wrong: got %v", row1.Values[1].Comp)
	}
	// Vector: [1.0, 2.5, 3.0]
	if !row1.Values[2].Vec.Equals(datatypes.Vector{1.0, 2.5, 3.0}) {
		t.Errorf("row1 vector wrong: got %v", row1.Values[2].Vec)
	}
	// Matrix: [[1, 2], [3, 4]]
	if !row1.Values[3].Mat.Equals(datatypes.Matrix{{1.0, 2.0}, {3.0, 4.0}}) {
		t.Errorf("row1 matrix wrong: got %v", row1.Values[3].Mat)
	}
	// Tensor: [[[1.0, 2.0]], [[3.0, 4.0]]] -> shape [2, 1, 2], data [1, 2, 3, 4]
	expectedTensor1 := datatypes.Tensor{Shape: []int{2, 1, 2}, Data: []float64{1.0, 2.0, 3.0, 4.0}}
	if !row1.Values[4].Ten.Equals(expectedTensor1) {
		t.Errorf("row1 tensor wrong: got %v", row1.Values[4].Ten)
	}

	// Verify Row 2
	row2 := result.Rows[1]
	// Complex: -4.5i -> Real: 0, Imag: -4.5
	if !row2.Values[1].Comp.Equals(datatypes.Complex{Real: 0, Imag: -4.5}) {
		t.Errorf("row2 complex wrong: got %v", row2.Values[1].Comp)
	}
	// Vector: [0.0, -1.0]
	if !row2.Values[2].Vec.Equals(datatypes.Vector{0.0, -1.0}) {
		t.Errorf("row2 vector wrong: got %v", row2.Values[2].Vec)
	}

	// Test FILTERing on scientific values
	filterResult1 := execSQL(t, exec, "SELECT * FROM sci WHERE c == 1.2+3.4i;")
	if len(filterResult1.Rows) != 1 {
		t.Fatalf("expected 1 row for complex filter, got %d", len(filterResult1.Rows))
	}
	if filterResult1.Rows[0].Values[0].Int != 1 {
		t.Errorf("expected id 1, got %d", filterResult1.Rows[0].Values[0].Int)
	}

	filterResult2 := execSQL(t, exec, "SELECT * FROM sci WHERE v == [0.0, -1.0];")
	if len(filterResult2.Rows) != 1 {
		t.Fatalf("expected 1 row for vector filter, got %d", len(filterResult2.Rows))
	}
	if filterResult2.Rows[0].Values[0].Int != 2 {
		t.Errorf("expected id 2, got %d", filterResult2.Rows[0].Values[0].Int)
	}
}

func TestExecuteGeospatialTypes(t *testing.T) {
	exec := setupExecutor(t)

	execSQL(t, exec, "CREATE TABLE geo (id INT, p POINT, poly POLYGON);")

	// Test INSERT with POINT and POLYGON constructors, and WKT strings
	execSQL(t, exec, "INSERT INTO geo VALUES (1, POINT(3.0, 4.0), POLYGON(POINT(0,0), POINT(10,0), POINT(10,10), POINT(0,10)));")
	execSQL(t, exec, "INSERT INTO geo VALUES (2, 'POINT(-1 -1)', 'POLYGON((0 0, 2 0, 2 2, 0 2, 0 0))');")

	// Test SELECT *
	result := execSQL(t, exec, "SELECT * FROM geo;")
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result.Rows))
	}

	// Verify Row 1
	row1 := result.Rows[0]
	if row1.Values[0].Int != 1 {
		t.Errorf("expected id 1, got %v", row1.Values[0].Int)
	}
	if !row1.Values[1].Pt.Equals(datatypes.Point{X: 3.0, Y: 4.0}) {
		t.Errorf("row1 point wrong, got %v", row1.Values[1].Pt)
	}
	if len(row1.Values[2].Poly) != 4 || !row1.Values[2].Poly[1].Equals(datatypes.Point{X: 10, Y: 0}) {
		t.Errorf("row1 poly wrong, got %v", row1.Values[2].Poly)
	}

	// Verify Row 2
	row2 := result.Rows[1]
	if row2.Values[0].Int != 2 {
		t.Errorf("expected id 2, got %v", row2.Values[0].Int)
	}
	if !row2.Values[1].Pt.Equals(datatypes.Point{X: -1.0, Y: -1.0}) {
		t.Errorf("row2 point wrong, got %v", row2.Values[1].Pt)
	}

	// Test SELECT with functions: DISTANCE, AREA, INTERSECTS
	funcRes := execSQL(t, exec, "SELECT DISTANCE(p, POINT(0,0)), AREA(poly), INTERSECTS(POINT(5,5), poly) FROM geo WHERE id = 1;")
	if len(funcRes.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(funcRes.Rows))
	}
	fRow := funcRes.Rows[0]
	
	// DISTANCE(POINT(3,4), POINT(0,0)) -> 5.0
	if fRow.Values[0].Flt != 5.0 {
		t.Errorf("expected distance 5.0, got %f", fRow.Values[0].Flt)
	}
	
	// AREA(POLYGON(...)) -> 100.0
	if fRow.Values[1].Flt != 100.0 {
		t.Errorf("expected area 100.0, got %f", fRow.Values[1].Flt)
	}
	
	// INTERSECTS(POINT(5,5), POLYGON(...)) -> 1
	if fRow.Values[2].Int != 1 {
		t.Errorf("expected intersects 1 (true), got %d", fRow.Values[2].Int)
	}
}
