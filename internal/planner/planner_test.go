package planner

import (
	"flamingodb/internal/parser/ast"
	"flamingodb/internal/parser/lexer"
	"flamingodb/internal/parser/parser"
	"flamingodb/internal/storage/record"
	"testing"
)

func TestPlanSelect(t *testing.T) {
	input := "SELECT id, name FROM users WHERE age >= 18;"
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}

	planner := New()
	plan, err := planner.Plan(prog.Statements[0])
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}

	if plan.Type() != PlanProject {
		t.Errorf("expected Project node at top, got %s", plan.Type())
	}

	projNode, ok := plan.(*ProjectNode)
	if !ok {
		t.Fatalf("could not cast plan to *ProjectNode")
	}
	if len(projNode.Fields) != 2 || projNode.Fields[0] != "id" || projNode.Fields[1] != "name" {
		t.Errorf("fields mismatch: got %v", projNode.Fields)
	}

	if projNode.Child.Type() != PlanFilter {
		t.Errorf("expected Filter node as child, got %s", projNode.Child.Type())
	}

	filterNode, ok := projNode.Child.(*FilterNode)
	if !ok {
		t.Fatalf("could not cast child to *FilterNode")
	}
	if filterNode.Condition.String() != "(age >= 18)" {
		t.Errorf("condition mismatch: got %s", filterNode.Condition.String())
	}

	if filterNode.Child.Type() != PlanScan {
		t.Errorf("expected Scan node as child, got %s", filterNode.Child.Type())
	}

	scanNode, ok := filterNode.Child.(*ScanNode)
	if !ok {
		t.Fatalf("could not cast child to *ScanNode")
	}
	if scanNode.Table != "users" {
		t.Errorf("table mismatch: got %s", scanNode.Table)
	}

	children := plan.Children()
	if len(children) != 1 || children[0] != projNode.Child {
		t.Errorf("expected Children() to return projNode.Child")
	}
}

func TestPlanSelectNoWhere(t *testing.T) {
	input := "SELECT id FROM users;"
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	planner := New()
	plan, err := planner.Plan(prog.Statements[0])
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}

	projNode, ok := plan.(*ProjectNode)
	if !ok {
		t.Fatalf("could not cast plan to *ProjectNode")
	}

	if projNode.Child.Type() != PlanScan {
		t.Errorf("expected child of Project to be Scan, got %s", projNode.Child.Type())
	}
}

func TestPlanInsert(t *testing.T) {
	input := "INSERT INTO items VALUES (1, 9.99, 'test');"
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	planner := New()
	plan, err := planner.Plan(prog.Statements[0])
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}

	if plan.Type() != PlanInsert {
		t.Fatalf("expected Insert node, got %s", plan.Type())
	}

	insertNode, ok := plan.(*InsertNode)
	if !ok {
		t.Fatalf("could not cast to *InsertNode")
	}
	if insertNode.Table != "items" {
		t.Errorf("table mismatch: got %s", insertNode.Table)
	}
	if len(insertNode.Values) != 3 {
		t.Errorf("expected 3 values, got %d", len(insertNode.Values))
	}
	if plan.Children() != nil {
		t.Errorf("expected Children() to be nil for InsertNode")
	}
}

func TestPlanUpdate(t *testing.T) {
	input := "UPDATE items SET price = 10.00 WHERE id = 1;"
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	planner := New()
	plan, err := planner.Plan(prog.Statements[0])
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}

	if plan.Type() != PlanUpdate {
		t.Fatalf("expected Update node, got %s", plan.Type())
	}

	updateNode, ok := plan.(*UpdateNode)
	if !ok {
		t.Fatalf("could not cast to *UpdateNode")
	}
	if updateNode.Table != "items" {
		t.Errorf("table mismatch: got %s", updateNode.Table)
	}
	if len(updateNode.Set) != 1 || updateNode.Set["price"].String() != "10.00" {
		t.Errorf("set mismatch: got %v", updateNode.Set)
	}

	// Child should be Filter(id = 1) -> Scan(items)
	if updateNode.Child.Type() != PlanFilter {
		t.Errorf("expected Filter node as child, got %s", updateNode.Child.Type())
	}
}

func TestPlanDelete(t *testing.T) {
	input := "DELETE FROM items WHERE id != 1;"
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	planner := New()
	plan, err := planner.Plan(prog.Statements[0])
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}

	if plan.Type() != PlanDelete {
		t.Fatalf("expected Delete node, got %s", plan.Type())
	}

	deleteNode, ok := plan.(*DeleteNode)
	if !ok {
		t.Fatalf("could not cast to *DeleteNode")
	}
	if deleteNode.Table != "items" {
		t.Errorf("table mismatch: got %s", deleteNode.Table)
	}
	if deleteNode.Child.Type() != PlanFilter {
		t.Errorf("expected Filter node as child, got %s", deleteNode.Child.Type())
	}
}

func TestPlanCreateTableAndSchemaMapping(t *testing.T) {
	input := "CREATE TABLE items (id INT, price FLOAT, name VARCHAR);"
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	planner := New()
	plan, err := planner.Plan(prog.Statements[0])
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}

	if plan.Type() != PlanCreateTable {
		t.Fatalf("expected CreateTable node, got %s", plan.Type())
	}

	createNode, ok := plan.(*CreateTableNode)
	if !ok {
		t.Fatalf("could not cast to *CreateTableNode")
	}

	if createNode.Table != "items" {
		t.Errorf("table mismatch: got %s", createNode.Table)
	}

	schema, err := createNode.ToSchema()
	if err != nil {
		t.Fatalf("ToSchema returned error: %v", err)
	}

	if len(schema.Columns) != 3 {
		t.Fatalf("expected 3 columns in schema, got %d", len(schema.Columns))
	}

	if schema.Columns[0].Name != "id" || schema.Columns[0].Type != record.Integer {
		t.Errorf("column 0 mismatch: %v", schema.Columns[0])
	}
	if schema.Columns[1].Name != "price" || schema.Columns[1].Type != record.Float {
		t.Errorf("column 1 mismatch: %v", schema.Columns[1])
	}
	if schema.Columns[2].Name != "name" || schema.Columns[2].Type != record.Varchar {
		t.Errorf("column 2 mismatch: %v", schema.Columns[2])
	}
}

func TestPlanStringRepresentation(t *testing.T) {
	input := "SELECT id, name FROM users WHERE age >= 18;"
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	planner := New()
	plan, err := planner.Plan(prog.Statements[0])
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}

	expectedStr := "Project(id, name)"
	if plan.String() != expectedStr {
		t.Errorf("expected string representation %q, got %q", expectedStr, plan.String())
	}

	// Test Scan string
	scan := &ScanNode{Table: "users"}
	if scan.String() != "Scan(users)" {
		t.Errorf("Scan string got %q", scan.String())
	}

	// Test Filter string
	filter := &FilterNode{Child: scan, Condition: &ast.Identifier{Value: "x"}}
	if filter.String() != "Filter(x)" {
		t.Errorf("Filter string got %q", filter.String())
	}

	// Test CreateTable string
	createTable := &CreateTableNode{Table: "t", Columns: []ast.ColumnDef{{Name: "x", Type: "INT"}}}
	if createTable.String() != "CreateTable(table=t, columns=[x INT])" {
		t.Errorf("CreateTable string got %q", createTable.String())
	}
}

func TestMapStringToTypeIDErrors(t *testing.T) {
	_, err := MapStringToTypeID("UNKNOWN_TYPE")
	if err == nil {
		t.Errorf("expected error mapping UNKNOWN_TYPE, got nil")
	}
}
