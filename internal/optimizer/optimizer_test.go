package optimizer_test

import (
	"path/filepath"
	"testing"

	"flamingodb/internal/executor"
	"flamingodb/internal/parser/ast"
	"flamingodb/internal/planner"
	"flamingodb/internal/optimizer"
	"flamingodb/internal/storage/catalog"
	"flamingodb/internal/storage/disk"
	"flamingodb/internal/storage/pager"
	"flamingodb/internal/storage/record"
)

func setupTableManager(t *testing.T) (*catalog.TableManager, func()) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	pageSize := uint32(4096)

	dm, err := disk.NewDiskManager(dbPath, pageSize)
	if err != nil {
		t.Fatalf("failed to create disk manager: %v", err)
	}

	p, err := pager.New(dm, pageSize)
	if err != nil {
		dm.Close()
		t.Fatalf("failed to create pager: %v", err)
	}

	tm, err := catalog.NewTableManager(p)
	if err != nil {
		dm.Close()
		t.Fatalf("failed to create table manager: %v", err)
	}

	cleanup := func() {
		tm.Close()
		dm.Close()
	}

	return tm, cleanup
}

func TestOptimizeWildcardProject(t *testing.T) {
	tm, cleanup := setupTableManager(t)
	defer cleanup()

	plan := &planner.ProjectNode{
		Fields: []ast.Expression{&ast.Identifier{Value: "*"}},
		Child:  &planner.ScanNode{Table: "users"},
	}

	opt, err := optimizer.Optimize(plan, tm)
	if err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}

	if opt.Type() != planner.PlanScan {
		t.Errorf("Expected Scan node, got %v", opt.Type())
	}
}

func TestOptimizeProjectionMerge(t *testing.T) {
	tm, cleanup := setupTableManager(t)
	defer cleanup()

	plan := &planner.ProjectNode{
		Fields: []ast.Expression{&ast.Identifier{Value: "a"}},
		Child: &planner.ProjectNode{
			Fields: []ast.Expression{
				&ast.Identifier{Value: "a"},
				&ast.Identifier{Value: "b"},
			},
			Child: &planner.ScanNode{Table: "users"},
		},
	}

	opt, err := optimizer.Optimize(plan, tm)
	if err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}

	proj, ok := opt.(*planner.ProjectNode)
	if !ok {
		t.Fatalf("Expected ProjectNode, got %T", opt)
	}
	if proj.Child.Type() != planner.PlanScan {
		t.Errorf("Expected child to be Scan, got %v", proj.Child.Type())
	}
	if len(proj.Fields) != 1 || proj.Fields[0].String() != "a" {
		t.Errorf("Expected fields to be [a], got %v", proj.Fields)
	}
}

func TestOptimizeFilterPushdown(t *testing.T) {
	tm, cleanup := setupTableManager(t)
	defer cleanup()

	plan := &planner.FilterNode{
		Condition: &ast.InfixExpression{
			Left:     &ast.Identifier{Value: "a"},
			Operator: ">",
			Right:    &ast.IntegerLiteral{Value: 5},
		},
		Child: &planner.ProjectNode{
			Fields: []ast.Expression{
				&ast.Identifier{Value: "a"},
				&ast.Identifier{Value: "b"},
			},
			Child: &planner.ScanNode{Table: "users"},
		},
	}

	opt, err := optimizer.Optimize(plan, tm)
	if err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}

	proj, ok := opt.(*planner.ProjectNode)
	if !ok {
		t.Fatalf("Expected ProjectNode at root, got %T", opt)
	}
	filter, ok := proj.Child.(*planner.FilterNode)
	if !ok {
		t.Fatalf("Expected FilterNode as child of project, got %T", proj.Child)
	}
	if filter.Child.Type() != planner.PlanScan {
		t.Errorf("Expected ScanNode as child of filter, got %v", filter.Child.Type())
	}
}

func TestOptimizeBTreeIndexScan(t *testing.T) {
	tm, cleanup := setupTableManager(t)
	defer cleanup()

	schema := record.NewSchema([]record.Column{
		{Name: "id", Type: record.Integer},
		{Name: "name", Type: record.Varchar},
	})
	if err := tm.CreateTable(nil, "users", schema); err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}
	if err := tm.CreateIndex(nil, "users", "id"); err != nil {
		t.Fatalf("CreateIndex failed: %v", err)
	}

	plan := &planner.FilterNode{
		Condition: &ast.InfixExpression{
			Left:     &ast.Identifier{Value: "id"},
			Operator: "=",
			Right:    &ast.IntegerLiteral{Value: 42},
		},
		Child: &planner.ScanNode{Table: "users"},
	}

	opt, err := optimizer.Optimize(plan, tm)
	if err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}

	filter, ok := opt.(*planner.FilterNode)
	if !ok {
		t.Fatalf("Expected FilterNode, got %T", opt)
	}
	idxScan, ok := filter.Child.(*planner.IndexScanNode)
	if !ok {
		t.Fatalf("Expected child to be IndexScanNode, got %T", filter.Child)
	}
	if idxScan.ColumnName != "id" || idxScan.Table != "users" {
		t.Errorf("IndexScan configured incorrectly: %+v", idxScan)
	}
	if idxScan.LowVal == nil || idxScan.LowVal.Int != 42 || idxScan.HighVal == nil || idxScan.HighVal.Int != 42 {
		t.Errorf("IndexScan bounds incorrect: low=%v, high=%v", idxScan.LowVal, idxScan.HighVal)
	}

	plan2 := &planner.FilterNode{
		Condition: &ast.InfixExpression{
			Left:     &ast.Identifier{Value: "name"},
			Operator: "=",
			Right:    &ast.StringLiteral{Value: "Alice"},
		},
		Child: &planner.ScanNode{Table: "users"},
	}

	opt2, err := optimizer.Optimize(plan2, tm)
	if err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}

	filter2, ok := opt2.(*planner.FilterNode)
	if !ok {
		t.Fatalf("Expected FilterNode, got %T", opt2)
	}
	if filter2.Child.Type() != planner.PlanScan {
		t.Errorf("Expected ScanNode for non-indexed filter, got %v", filter2.Child.Type())
	}
}

func TestOptimizeE2EExecution(t *testing.T) {
	tm, cleanup := setupTableManager(t)
	defer cleanup()

	schema := record.NewSchema([]record.Column{
		{Name: "id", Type: record.Integer},
		{Name: "score", Type: record.Float},
		{Name: "tag", Type: record.Varchar},
	})
	if err := tm.CreateTable(nil, "items", schema); err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	records := []*record.Record{
		{Values: []record.Value{{Type: record.Integer, Int: 1}, {Type: record.Float, Flt: 99.5}, {Type: record.Varchar, Str: "first"}}},
		{Values: []record.Value{{Type: record.Integer, Int: 2}, {Type: record.Float, Flt: 88.0}, {Type: record.Varchar, Str: "second"}}},
		{Values: []record.Value{{Type: record.Integer, Int: 3}, {Type: record.Float, Flt: 99.5}, {Type: record.Varchar, Str: "third"}}},
		{Values: []record.Value{{Type: record.Integer, Int: 4}, {Type: record.Float, Flt: 77.2}, {Type: record.Varchar, Str: "fourth"}}},
	}
	for i, r := range records {
		if err := tm.InsertRecord(nil, "items", r); err != nil {
			t.Fatalf("InsertRecord %d failed: %v", i, err)
		}
	}

	if err := tm.CreateIndex(nil, "items", "score"); err != nil {
		t.Fatalf("CreateIndex failed: %v", err)
	}

	rawPlan := &planner.ProjectNode{
		Fields: []ast.Expression{&ast.Identifier{Value: "tag"}},
		Child: &planner.FilterNode{
			Condition: &ast.InfixExpression{
				Left:     &ast.Identifier{Value: "score"},
				Operator: "=",
				Right:    &ast.FloatLiteral{Value: 99.5},
			},
			Child: &planner.ScanNode{Table: "items"},
		},
	}

	optPlan, err := optimizer.Optimize(rawPlan, tm)
	if err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}

	proj, ok := optPlan.(*planner.ProjectNode)
	if !ok {
		t.Fatalf("expected ProjectNode, got %T", optPlan)
	}
	filter, ok := proj.Child.(*planner.FilterNode)
	if !ok {
		t.Fatalf("expected FilterNode, got %T", proj.Child)
	}
	if _, ok := filter.Child.(*planner.IndexScanNode); !ok {
		t.Errorf("expected child to be IndexScanNode, got %T", filter.Child)
	}

	exec := executor.New(tm)
	res, err := exec.Execute(optPlan)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(res.Rows) != 2 {
		t.Fatalf("expected 2 result rows, got %d", len(res.Rows))
	}

	row0 := res.Rows[0].Values[0].Str
	row1 := res.Rows[1].Values[0].Str

	if !((row0 == "first" && row1 == "third") || (row0 == "third" && row1 == "first")) {
		t.Errorf("unexpected rows returned: %q, %q", row0, row1)
	}
}
