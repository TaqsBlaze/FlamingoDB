package parser

import (
	"github.com/TaqsBlaze/FlamingoDB/internal/parser/ast"
	"github.com/TaqsBlaze/FlamingoDB/internal/parser/lexer"
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
	if stmt.Fields[0].String() != "id" || stmt.Fields[1].String() != "name" {
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



func TestParseFunctionCalls(t *testing.T) {
	input := "SELECT SIN(val), POW(x, 2) FROM dataset WHERE COS(y) > 0.5;"
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

	if len(stmt.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(stmt.Fields))
	}

	field0, ok := stmt.Fields[0].(*ast.CallExpression)
	if !ok {
		t.Fatalf("expected Field[0] to be *ast.CallExpression, got %T", stmt.Fields[0])
	}
	if field0.Function != "SIN" || len(field0.Args) != 1 || field0.Args[0].String() != "val" {
		t.Errorf("field0 structure wrong, got %v", field0)
	}

	field1, ok := stmt.Fields[1].(*ast.CallExpression)
	if !ok {
		t.Fatalf("expected Field[1] to be *ast.CallExpression, got %T", stmt.Fields[1])
	}
	if field1.Function != "POW" || len(field1.Args) != 2 || field1.Args[0].String() != "x" || field1.Args[1].String() != "2" {
		t.Errorf("field1 structure wrong, got %v", field1)
	}

	whereCall, ok := stmt.Where.(*ast.InfixExpression).Left.(*ast.CallExpression)
	if !ok {
		t.Fatalf("expected Where condition Left to be *ast.CallExpression, got %T", stmt.Where.(*ast.InfixExpression).Left)
	}
	if whereCall.Function != "COS" || len(whereCall.Args) != 1 || whereCall.Args[0].String() != "y" {
		t.Errorf("where call structure wrong, got %v", whereCall)
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


func TestParseScientificLiterals(t *testing.T) {
	input := "INSERT INTO science VALUES ([1, 2.5, 3], [[1, 2], [3.5, 4.5]], 1.2+3.4i, -5.6i);"
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

	if len(stmt.Values) != 4 {
		t.Fatalf("len(stmt.Values) not 4. got=%d", len(stmt.Values))
	}

	// Value 0: [1, 2.5, 3] -> ArrayLiteral
	arr1, ok := stmt.Values[0].(*ast.ArrayLiteral)
	if !ok {
		t.Fatalf("stmt.Values[0] is not *ast.ArrayLiteral. got=%T", stmt.Values[0])
	}
	if len(arr1.Elements) != 3 {
		t.Fatalf("arr1 length wrong. got=%d", len(arr1.Elements))
	}

	// Value 1: [[1, 2], [3.5, 4.5]] -> ArrayLiteral of ArrayLiterals
	arr2, ok := stmt.Values[1].(*ast.ArrayLiteral)
	if !ok {
		t.Fatalf("stmt.Values[1] is not *ast.ArrayLiteral. got=%T", stmt.Values[1])
	}
	if len(arr2.Elements) != 2 {
		t.Fatalf("arr2 length wrong. got=%d", len(arr2.Elements))
	}
	subArr1, ok := arr2.Elements[0].(*ast.ArrayLiteral)
	if !ok {
		t.Fatalf("subArr1 is not *ast.ArrayLiteral. got=%T", arr2.Elements[0])
	}
	if len(subArr1.Elements) != 2 {
		t.Fatalf("subArr1 length wrong. got=%d", len(subArr1.Elements))
	}

	// Value 2: 1.2+3.4i -> InfixExpression
	inf, ok := stmt.Values[2].(*ast.InfixExpression)
	if !ok {
		t.Fatalf("stmt.Values[2] is not *ast.InfixExpression. got=%T", stmt.Values[2])
	}
	if inf.Operator != "+" {
		t.Errorf("inf operator wrong. got=%s", inf.Operator)
	}

	// Value 3: -5.6i -> ImaginaryLiteral (folded)
	imag, ok := stmt.Values[3].(*ast.ImaginaryLiteral)
	if !ok {
		t.Fatalf("stmt.Values[3] is not *ast.ImaginaryLiteral. got=%T", stmt.Values[3])
	}
	if imag.Value != -5.6 {
		t.Errorf("imaginary value wrong. expected=-5.6, got=%g", imag.Value)
	}
}

func TestParseShowTables(t *testing.T) {
	input := "SHOW TABLES;"
	l := lexer.New(input)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	_, ok := program.Statements[0].(*ast.ShowTablesStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ShowTablesStatement. got=%T", program.Statements[0])
	}
}
