package executor

import (
	"fmt"
	"strings"

	"flamingodb/internal/parser/ast"
	"flamingodb/internal/planner"
	"flamingodb/internal/storage/catalog"
	"flamingodb/internal/storage/record"
	"flamingodb/internal/transaction"
)

// Row represents a single result row returned from execution.
type Row struct {
	Values []record.Value
}

// Result holds the outcome of executing a plan.
type Result struct {
	// Rows holds the result rows for SELECT statements.
	Rows []Row
	// RowsAffected holds the count of rows inserted/updated/deleted.
	RowsAffected int
	// Message is an informational message (e.g. for CREATE TABLE).
	Message string
}

// Executor executes logical plan nodes against the storage engine.
type Executor struct {
	tm *catalog.TableManager
}

// New creates a new Executor backed by the given TableManager.
func New(tm *catalog.TableManager) *Executor {
	return &Executor{tm: tm}
}

// Execute runs the given plan node and returns a Result.
func (e *Executor) Execute(node planner.PlanNode) (*Result, error) {
	return e.ExecuteWithTx(nil, node)
}

// ExecuteWithTx runs the given plan node under an explicit transaction context.
func (e *Executor) ExecuteWithTx(tx *transaction.Transaction, node planner.PlanNode) (*Result, error) {
	if node == nil {
		return nil, fmt.Errorf("cannot execute nil plan node")
	}

	switch n := node.(type) {
	case *planner.CreateTableNode:
		return e.executeCreateTable(tx, n)
	case *planner.InsertNode:
		return e.executeInsert(tx, n)
	case *planner.ProjectNode:
		return e.executeProject(tx, n)
	case *planner.FilterNode:
		return e.executeFilter(tx, n)
	case *planner.ScanNode:
		return e.executeScan(tx, n)
	default:
		return nil, fmt.Errorf("unsupported plan node type: %T", node)
	}
}

// executeCreateTable handles CREATE TABLE by registering the schema with the TableManager.
func (e *Executor) executeCreateTable(tx *transaction.Transaction, n *planner.CreateTableNode) (*Result, error) {
	schema, err := n.ToSchema()
	if err != nil {
		return nil, fmt.Errorf("create table schema error: %w", err)
	}

	if err := e.tm.CreateTable(tx, n.Table, schema); err != nil {
		return nil, fmt.Errorf("create table %q failed: %w", n.Table, err)
	}

	return &Result{Message: fmt.Sprintf("table %q created", n.Table)}, nil
}

// executeInsert handles INSERT by converting expressions to typed Values and persisting.
func (e *Executor) executeInsert(tx *transaction.Transaction, n *planner.InsertNode) (*Result, error) {
	schema, err := e.tm.GetSchema(n.Table)
	if err != nil {
		return nil, fmt.Errorf("insert into %q failed: %w", n.Table, err)
	}

	if len(n.Values) != len(schema.Columns) {
		return nil, fmt.Errorf(
			"insert into %q: expected %d values, got %d",
			n.Table, len(schema.Columns), len(n.Values),
		)
	}

	values := make([]record.Value, len(n.Values))
	for i, expr := range n.Values {
		col := schema.Columns[i]
		v, err := evalExpression(expr, col.Type)
		if err != nil {
			return nil, fmt.Errorf("insert into %q, column %q: %w", n.Table, col.Name, err)
		}
		values[i] = v
	}

	rec := &record.Record{Values: values}
	if err := e.tm.InsertRecord(tx, n.Table, rec); err != nil {
		return nil, fmt.Errorf("insert into %q failed: %w", n.Table, err)
	}

	return &Result{RowsAffected: 1}, nil
}

// executeScan handles full table scans, returning all rows.
func (e *Executor) executeScan(tx *transaction.Transaction, n *planner.ScanNode) (*Result, error) {
	records, err := e.tm.ReadRecords(tx, n.Table)
	if err != nil {
		return nil, fmt.Errorf("scan %q failed: %w", n.Table, err)
	}

	rows := make([]Row, len(records))
	for i, r := range records {
		rows[i] = Row{Values: r.Values}
	}

	return &Result{Rows: rows}, nil
}

// executeFilter handles WHERE clause filtering on top of a child node.
func (e *Executor) executeFilter(tx *transaction.Transaction, n *planner.FilterNode) (*Result, error) {
	childResult, err := e.ExecuteWithTx(tx, n.Child)
	if err != nil {
		return nil, err
	}

	schema, err := e.schemaFromChild(n.Child)
	if err != nil {
		return nil, err
	}

	if err := validateCondition(n.Condition, schema); err != nil {
		return nil, err
	}

	var filtered []Row
	for _, row := range childResult.Rows {
		match, err := evalCondition(n.Condition, row, schema)
		if err != nil {
			return nil, fmt.Errorf("filter evaluation failed: %w", err)
		}
		if match {
			filtered = append(filtered, row)
		}
	}

	return &Result{Rows: filtered}, nil
}

// validateCondition checks that the filter condition is semantically valid for the schema.
func validateCondition(expr ast.Expression, schema *record.Schema) error {
	infix, ok := expr.(*ast.InfixExpression)
	if !ok {
		return fmt.Errorf("unsupported condition expression type: %T", expr)
	}

	ident, ok := infix.Left.(*ast.Identifier)
	if !ok {
		return fmt.Errorf("left side of condition must be a column identifier")
	}

	colIdx := -1
	var colType record.TypeID
	for i, col := range schema.Columns {
		if strings.EqualFold(col.Name, ident.Value) {
			colIdx = i
			colType = col.Type
			break
		}
	}
	if colIdx == -1 {
		return fmt.Errorf("column %q not found", ident.Value)
	}

	_, err := evalExpression(infix.Right, colType)
	if err != nil {
		return fmt.Errorf("condition right-hand side error: %w", err)
	}

	return nil
}

// executeProject handles column projection on top of a child node.
func (e *Executor) executeProject(tx *transaction.Transaction, n *planner.ProjectNode) (*Result, error) {
	childResult, err := e.ExecuteWithTx(tx, n.Child)
	if err != nil {
		return nil, err
	}

	// Wildcard: return all columns
	if len(n.Fields) == 1 && n.Fields[0] == "*" {
		return childResult, nil
	}

	schema, err := e.schemaFromChild(n.Child)
	if err != nil {
		return nil, err
	}

	// Build column index map and validate columns first
	colIndex := make(map[string]int, len(schema.Columns))
	for i, col := range schema.Columns {
		colIndex[col.Name] = i
	}

	for _, field := range n.Fields {
		if _, ok := colIndex[field]; !ok {
			return nil, fmt.Errorf("column %q not found in table", field)
		}
	}

	var projected []Row
	for _, row := range childResult.Rows {
		var vals []record.Value
		for _, field := range n.Fields {
			idx := colIndex[field]
			vals = append(vals, row.Values[idx])
		}
		projected = append(projected, Row{Values: vals})
	}

	return &Result{Rows: projected}, nil
}

// schemaFromChild resolves the schema from a scan or filter child.
func (e *Executor) schemaFromChild(child planner.PlanNode) (*record.Schema, error) {
	switch n := child.(type) {
	case *planner.ScanNode:
		return e.tm.GetSchema(n.Table)
	case *planner.FilterNode:
		return e.schemaFromChild(n.Child)
	default:
		return nil, fmt.Errorf("cannot resolve schema from node type %T", child)
	}
}

// evalExpression converts an AST Expression to a typed record.Value based on the expected column type.
func evalExpression(expr ast.Expression, colType record.TypeID) (record.Value, error) {
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		if colType != record.Integer {
			return record.Value{}, fmt.Errorf("type mismatch: expected integer")
		}
		return record.Value{Type: record.Integer, Int: int32(e.Value)}, nil

	case *ast.FloatLiteral:
		if colType != record.Float {
			return record.Value{}, fmt.Errorf("type mismatch: expected float")
		}
		return record.Value{Type: record.Float, Flt: e.Value}, nil

	case *ast.StringLiteral:
		if colType != record.Varchar {
			return record.Value{}, fmt.Errorf("type mismatch: expected varchar")
		}
		return record.Value{Type: record.Varchar, Str: e.Value}, nil

	default:
		return record.Value{}, fmt.Errorf("unsupported expression type: %T", expr)
	}
}

// evalCondition evaluates an AST expression as a boolean condition on a row.
func evalCondition(expr ast.Expression, row Row, schema *record.Schema) (bool, error) {
	infix, ok := expr.(*ast.InfixExpression)
	if !ok {
		return false, fmt.Errorf("unsupported condition expression type: %T", expr)
	}

	// Left must be an identifier (column name)
	ident, ok := infix.Left.(*ast.Identifier)
	if !ok {
		return false, fmt.Errorf("left side of condition must be a column identifier")
	}

	// Find column index
	colIdx := -1
	var colType record.TypeID
	for i, col := range schema.Columns {
		if strings.EqualFold(col.Name, ident.Value) {
			colIdx = i
			colType = col.Type
			break
		}
	}
	if colIdx == -1 {
		return false, fmt.Errorf("column %q not found", ident.Value)
	}

	rowVal := row.Values[colIdx]
	rightVal, err := evalExpression(infix.Right, colType)
	if err != nil {
		return false, fmt.Errorf("condition right-hand side error: %w", err)
	}

	return compareValues(rowVal, infix.Operator, rightVal)
}

// compareValues compares two record.Values using the given operator.
func compareValues(left record.Value, op string, right record.Value) (bool, error) {
	switch left.Type {
	case record.Integer:
		l, r := left.Int, right.Int
		switch op {
		case "=", "==":
			return l == r, nil
		case "!=":
			return l != r, nil
		case "<":
			return l < r, nil
		case ">":
			return l > r, nil
		case "<=":
			return l <= r, nil
		case ">=":
			return l >= r, nil
		}
	case record.Float:
		l, r := left.Flt, right.Flt
		switch op {
		case "=", "==":
			return l == r, nil
		case "!=":
			return l != r, nil
		case "<":
			return l < r, nil
		case ">":
			return l > r, nil
		case "<=":
			return l <= r, nil
		case ">=":
			return l >= r, nil
		}
	case record.Varchar:
		l, r := left.Str, right.Str
		switch op {
		case "=", "==":
			return l == r, nil
		case "!=":
			return l != r, nil
		}
	}
	return false, fmt.Errorf("unsupported operator %q for type %v", op, left.Type)
}
