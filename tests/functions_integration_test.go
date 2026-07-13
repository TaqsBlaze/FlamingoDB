package tests

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/TaqsBlaze/FlamingoDB/internal/executor"
	"github.com/TaqsBlaze/FlamingoDB/internal/parser/lexer"
	"github.com/TaqsBlaze/FlamingoDB/internal/parser/parser"
	"github.com/TaqsBlaze/FlamingoDB/internal/planner"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/catalog"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/disk"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/pager"
)

func TestFunctionsIntegration(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "func_test.db")
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

	exec := executor.New(tm)

	// Helper helper to run SQL statements
	runSQL := func(sql string) *executor.Result {
		l := lexer.New(sql)
		prs := parser.New(l)
		prog := prs.ParseProgram()
		if len(prs.Errors()) > 0 {
			t.Fatalf("parser errors for %q: %v", sql, prs.Errors())
		}
		if len(prog.Statements) != 1 {
			t.Fatalf("expected 1 statement for %q, got %d", sql, len(prog.Statements))
		}
		stmt := prog.Statements[0]

		pln := planner.New()
		plan, err := pln.Plan(stmt)
		if err != nil {
			t.Fatalf("planner error for %q: %v", sql, err)
		}

		res, err := exec.Execute(plan)
		if err != nil {
			t.Fatalf("execution error for %q: %v", sql, err)
		}
		return res
	}

	// 1. Create table
	runSQL("CREATE TABLE test_math (id INT, val FLOAT, x INT, y INT, vec VARCHAR);")

	// 2. Insert test data
	runSQL("INSERT INTO test_math VALUES (1, 0.0, 2, 3, '[1.0, 2.0, 3.0]');")
	// val is pi
	runSQL("INSERT INTO test_math VALUES (2, 3.1415926535, -5, 10, '4.0, 5.0, 6.0');")

	// 3. Test scalar functions in projection
	res := runSQL("SELECT SIN(val), COS(val), ABS(x), POW(x, y), NORM(vec) FROM test_math WHERE id = 1;")
	if len(res.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(res.Rows))
	}
	row := res.Rows[0]
	// SIN(0) = 0
	if row.Values[0].Flt != 0.0 {
		t.Errorf("expected SIN(val) = 0.0, got %f", row.Values[0].Flt)
	}
	// COS(0) = 1
	if row.Values[1].Flt != 1.0 {
		t.Errorf("expected COS(val) = 1.0, got %f", row.Values[1].Flt)
	}
	// ABS(2) = 2
	if row.Values[2].Int != 2 {
		t.Errorf("expected ABS(x) = 2, got %d", row.Values[2].Int)
	}
	// POW(2, 3) = 8
	if row.Values[3].Flt != 8.0 {
		t.Errorf("expected POW(x, y) = 8.0, got %f", row.Values[3].Flt)
	}
	// NORM([1,2,3]) = sqrt(14)
	expectedNorm := math.Sqrt(14.0)
	if math.Abs(row.Values[4].Flt-expectedNorm) > 1e-6 {
		t.Errorf("expected NORM(vec) = %f, got %f", expectedNorm, row.Values[4].Flt)
	}

	// 4. Test vector DOT product
	res = runSQL("SELECT DOT(vec, '[1.0, 1.0, 1.0]') FROM test_math;")
	if len(res.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(res.Rows))
	}
	// Row 1: DOT([1,2,3], [1,1,1]) = 6
	if res.Rows[0].Values[0].Flt != 6.0 {
		t.Errorf("expected DOT row 0 to be 6.0, got %f", res.Rows[0].Values[0].Flt)
	}
	// Row 2: DOT([4,5,6], [1,1,1]) = 15
	if res.Rows[1].Values[0].Flt != 15.0 {
		t.Errorf("expected DOT row 1 to be 15.0, got %f", res.Rows[1].Values[0].Flt)
	}

	// 5. Test vector CROSS product
	res = runSQL("SELECT CROSS(vec, '[1.0, 0.0, 0.0]') FROM test_math WHERE id = 1;")
	if len(res.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(res.Rows))
	}
	// CROSS([1,2,3], [1,0,0]) = [0,3,-2]
	expectedCross := "[0.000000, 3.000000, -2.000000]"
	if res.Rows[0].Values[0].Str != expectedCross {
		t.Errorf("expected CROSS result %q, got %q", expectedCross, res.Rows[0].Values[0].Str)
	}

	// 6. Test function call in WHERE condition
	res = runSQL("SELECT id FROM test_math WHERE ABS(x) > 4;")
	if len(res.Rows) != 1 || res.Rows[0].Values[0].Int != 2 {
		t.Errorf("expected WHERE ABS(x) > 4 to return id=2, got: %v", res.Rows)
	}

	res = runSQL("SELECT id FROM test_math WHERE SIN(val) < 0.1;")
	if len(res.Rows) != 2 {
		t.Errorf("expected WHERE SIN(val) < 0.1 to return 2 rows, got %d", len(res.Rows))
	}
}


func TestGeospatialFunctionsIntegration(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "geo_test.db")
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

	exec := executor.New(tm)

	runSQL := func(sql string) *executor.Result {
		l := lexer.New(sql)
		prs := parser.New(l)
		prog := prs.ParseProgram()
		if len(prs.Errors()) > 0 {
			t.Fatalf("parser errors for %q: %v", sql, prs.Errors())
		}
		if len(prog.Statements) != 1 {
			t.Fatalf("expected 1 statement for %q, got %d", sql, len(prog.Statements))
		}
		stmt := prog.Statements[0]

		pln := planner.New()
		plan, err := pln.Plan(stmt)
		if err != nil {
			t.Fatalf("planner error for %q: %v", sql, err)
		}

		res, err := exec.Execute(plan)
		if err != nil {
			t.Fatalf("execution error for %q: %v", sql, err)
		}
		return res
	}

	// 1. Create table with POINT and POLYGON columns
	runSQL("CREATE TABLE test_geo (id INT, pt POINT, poly POLYGON);")

	// 2. Insert rows using constructors and WKT literals
	runSQL("INSERT INTO test_geo VALUES (1, POINT(0.0, 0.0), POLYGON(POINT(0,0), POINT(10,0), POINT(10,10), POINT(0,10)));")
	runSQL("INSERT INTO test_geo VALUES (2, 'POINT(5.0 5.0)', 'POLYGON((0 0, 5 0, 5 5, 0 5, 0 0))');")

	// 3. Test DISTANCE, AREA, and INTERSECTS in projection
	res := runSQL("SELECT DISTANCE(pt, POINT(3, 4)), AREA(poly), INTERSECTS(pt, poly) FROM test_geo WHERE id = 1;")
	if len(res.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(res.Rows))
	}
	row := res.Rows[0]
	// DISTANCE(POINT(0,0), POINT(3,4)) = 5
	if row.Values[0].Flt != 5.0 {
		t.Errorf("expected distance 5.0, got %f", row.Values[0].Flt)
	}
	// AREA(POLYGON((0 0, 10 0, 10 10, 0 10, 0 0))) = 100.0
	if row.Values[1].Flt != 100.0 {
		t.Errorf("expected area 100.0, got %f", row.Values[1].Flt)
	}
	// INTERSECTS(POINT(0,0), POLYGON(0 0, ...)) = 1 (true)
	if row.Values[2].Int != 1 {
		t.Errorf("expected intersects 1, got %d", row.Values[2].Int)
	}

	// 4. Test INTERSECTS inside WHERE condition
	// Point of row 2 (5,5) lies inside polygon of row 1 (10x10) but not vice versa (0,0 is inside row 1, but on boundary of row 2)
	res = runSQL("SELECT id FROM test_geo WHERE INTERSECTS(POINT(5, 5), poly) == 1;")
	if len(res.Rows) != 2 {
		t.Errorf("expected 2 rows intersecting POINT(5,5), got %d", len(res.Rows))
	}
}

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
