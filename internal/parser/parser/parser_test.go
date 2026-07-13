package parser

import (
	"flamingodb/internal/parser/ast"
	"flamingodb/internal/parser/lexer"
	"testing"
)

func TestParseSelectStatement(t *testing.T) {
	input := "SELECT id, name FROM users WHERE age >= 18;"
	l := lexer.New(input)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.SelectStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.SelectStatement. got=%T", program.Statements[0])
	}

	if stmt.Table != "users" {
		t.Errorf("stmt.Table not '%s'. got=%s", "users", stmt.Table)
	}
	if len(stmt.Fields) != 2 {
		t.Fatalf("len(stmt.Fields) not 2. got=%d", len(stmt.Fields))
	}
	if stmt.Fields[0] != "id" || stmt.Fields[1] != "name" {
		t.Errorf("fields wrong. got=%v", stmt.Fields)
	}

	if stmt.Where.String() != "(age >= 18)" {
		t.Errorf("where clause wrong. got=%s", stmt.Where.String())
	}
}

func TestParseInsertStatement(t *testing.T) {
	input := "INSERT INTO items VALUES (1, 9.99, 'test');"
	l := lexer.New(input)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.InsertStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.InsertStatement. got=%T", program.Statements[0])
	}

	if stmt.Table != "items" {
		t.Errorf("stmt.Table not '%s'. got=%s", "items", stmt.Table)
	}
	
	if len(stmt.Values) != 3 {
		t.Fatalf("len(stmt.Values) not 3. got=%d", len(stmt.Values))
	}
	if stmt.Values[0].String() != "1" || stmt.Values[1].String() != "9.99" || stmt.Values[2].String() != "'test'" {
		t.Errorf("values wrong. got=%v", stmt.Values)
	}
}

func TestParseUpdateStatement(t *testing.T) {
	input := "UPDATE items SET price = 10.00, count = 5 WHERE id = 1;"
	l := lexer.New(input)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.UpdateStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.UpdateStatement. got=%T", program.Statements[0])
	}

	if stmt.Table != "items" {
		t.Errorf("stmt.Table not '%s'. got=%s", "items", stmt.Table)
	}

	if len(stmt.Set) != 2 {
		t.Fatalf("len(stmt.Set) not 2. got=%d", len(stmt.Set))
	}

	if stmt.Set["price"].String() != "10.00" {
		t.Errorf("price value wrong. got=%s", stmt.Set["price"].String())
	}
	if stmt.Set["count"].String() != "5" {
		t.Errorf("count value wrong. got=%s", stmt.Set["count"].String())
	}

	if stmt.Where.String() != "(id = 1)" {
		t.Errorf("where wrong. got=%s", stmt.Where.String())
	}
}

func TestParseDeleteStatement(t *testing.T) {
	input := "DELETE FROM items WHERE id != 1;"
	l := lexer.New(input)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt, ok := program.Statements[0].(*ast.DeleteStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.DeleteStatement. got=%T", program.Statements[0])
	}

	if stmt.Table != "items" {
		t.Errorf("stmt.Table not items. got=%s", stmt.Table)
	}
	if stmt.Where.String() != "(id != 1)" {
		t.Errorf("where wrong. got=%s", stmt.Where.String())
	}
}

func TestParseCreateTableStatement(t *testing.T) {
	input := "CREATE TABLE items (id INT, price FLOAT);"
	l := lexer.New(input)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt, ok := program.Statements[0].(*ast.CreateTableStatement)
	if !ok {
		t.Fatalf("expected CreateTableStatement, got %T", program.Statements[0])
	}

	if stmt.Table != "items" {
		t.Errorf("Table not items. got=%s", stmt.Table)
	}

	if len(stmt.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(stmt.Columns))
	}
	
	if stmt.Columns[0].Name != "id" || stmt.Columns[0].Type != "INT" {
		t.Errorf("col 0 wrong: %v", stmt.Columns[0])
	}
	if stmt.Columns[1].Name != "price" || stmt.Columns[1].Type != "FLOAT" {
		t.Errorf("col 1 wrong: %v", stmt.Columns[1])
	}
}

func checkParserErrors(t *testing.T, p *Parser) {
	errors := p.Errors()
	if len(errors) == 0 {
		return
	}

	t.Errorf("parser has %d errors", len(errors))
	for _, msg := range errors {
		t.Errorf("parser error: %q", msg)
	}
	t.FailNow()
}
