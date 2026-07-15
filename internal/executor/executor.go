package executor

import (
	"fmt"
	"math"
	"strings"

	"github.com/TaqsBlaze/FlamingoDB/internal/datatypes"
	"github.com/TaqsBlaze/FlamingoDB/internal/functions"
	"github.com/TaqsBlaze/FlamingoDB/internal/index/btree"
	"github.com/TaqsBlaze/FlamingoDB/internal/parser/ast"
	"github.com/TaqsBlaze/FlamingoDB/internal/planner"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/catalog"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/page"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/record"
	"github.com/TaqsBlaze/FlamingoDB/internal/transaction"
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
	case *planner.DropTableNode:
		return e.executeDropTable(tx, n)
	case *planner.InsertNode:
		return e.executeInsert(tx, n)
	case *planner.ProjectNode:
		return e.executeProject(tx, n)
	case *planner.FilterNode:
		return e.executeFilter(tx, n)
	case *planner.ScanNode:
		return e.executeScan(tx, n)
	case *planner.IndexScanNode:
		return e.executeIndexScan(tx, n)
	case *planner.ShowTablesNode:
		return e.executeShowTables(tx, n)
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

// executeDropTable handles DROP TABLE by delegating to TableManager.
func (e *Executor) executeDropTable(tx *transaction.Transaction, n *planner.DropTableNode) (*Result, error) {
	if err := e.tm.DropTable(tx, n.Table); err != nil {
		return nil, fmt.Errorf("drop table %q failed: %w", n.Table, err)
	}
	return &Result{Message: fmt.Sprintf("table %q dropped", n.Table)}, nil
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

// typeOfExpression returns the TypeID of an expression and performs static type checking.
func typeOfExpression(expr ast.Expression, schema *record.Schema) (record.TypeID, error) {
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return record.Integer, nil
	case *ast.FloatLiteral:
		return record.Float, nil
	case *ast.StringLiteral:
		return record.Varchar, nil
	case *ast.Identifier:
		if e.Value == "*" {
			return record.Varchar, nil
		}
		if schema == nil {
			return 0, fmt.Errorf("schema not available to resolve column %q", e.Value)
		}
		for _, col := range schema.Columns {
			if strings.EqualFold(col.Name, e.Value) {
				return col.Type, nil
			}
		}
		return 0, fmt.Errorf("column %q not found in schema", e.Value)
	case *ast.ImaginaryLiteral:
		return record.Complex, nil
	case *ast.ArrayLiteral:
		if len(e.Elements) == 0 {
			return record.Vector, nil
		}
		firstType, err := typeOfExpression(e.Elements[0], schema)
		if err != nil {
			return 0, err
		}
		if firstType == record.Vector {
			return record.Matrix, nil
		}
		if firstType == record.Matrix || firstType == record.Tensor {
			return record.Tensor, nil
		}
		return record.Vector, nil
	case *ast.PrefixExpression:
		t, err := typeOfExpression(e.Right, schema)
		if err != nil {
			return 0, err
		}
		if t != record.Integer && t != record.Float {
			return 0, fmt.Errorf("cannot apply prefix operator %q to type %v", e.Operator, t)
		}
		return t, nil
	case *ast.InfixExpression:
		leftType, err := typeOfExpression(e.Left, schema)
		if err != nil {
			return 0, err
		}
		rightType, err := typeOfExpression(e.Right, schema)
		if err != nil {
			return 0, err
		}

		switch e.Operator {
		case "=", "!=", "==", "<", ">", "<=", ">=":
			if (leftType == record.Integer || leftType == record.Float) &&
				(rightType == record.Integer || rightType == record.Float) {
				return record.Integer, nil
			}
			if leftType == rightType {
				return record.Integer, nil
			}
			return 0, fmt.Errorf("type mismatch in comparison: %v %s %v", leftType, e.Operator, rightType)

		case "+", "-", "*", "/":
			if leftType == record.Complex || rightType == record.Complex {
				if (leftType == record.Integer || leftType == record.Float || leftType == record.Complex) &&
					(rightType == record.Integer || rightType == record.Float || rightType == record.Complex) {
					return record.Complex, nil
				}
			}
			if (leftType == record.Integer || leftType == record.Float) &&
				(rightType == record.Integer || rightType == record.Float) {
				if leftType == record.Float || rightType == record.Float {
					return record.Float, nil
				}
				return record.Integer, nil
			}
			return 0, fmt.Errorf("cannot apply arithmetic operator %q to types %v and %v", e.Operator, leftType, rightType)
		}
		return 0, fmt.Errorf("unsupported operator %q", e.Operator)

	case *ast.CallExpression:
		fnName := strings.ToUpper(e.Function)
		switch fnName {
		case "SIN", "COS", "TAN", "ASIN", "ACOS", "ATAN", "EXP", "LOG", "LN", "SQRT", "NORM":
			if len(e.Args) != 1 {
				return 0, fmt.Errorf("function %s expects 1 argument, got %d", fnName, len(e.Args))
			}
			argType, err := typeOfExpression(e.Args[0], schema)
			if err != nil {
				return 0, err
			}
			if fnName == "NORM" {
				if argType != record.Varchar {
					return 0, fmt.Errorf("NORM expects VARCHAR argument (vector format), got %v", argType)
				}
			} else {
				if argType != record.Integer && argType != record.Float {
					return 0, fmt.Errorf("%s expects numeric argument, got %v", fnName, argType)
				}
			}
			return record.Float, nil

		case "ABS":
			if len(e.Args) != 1 {
				return 0, fmt.Errorf("ABS expects 1 argument, got %d", len(e.Args))
			}
			argType, err := typeOfExpression(e.Args[0], schema)
			if err != nil {
				return 0, err
			}
			if argType != record.Integer && argType != record.Float {
				return 0, fmt.Errorf("ABS expects numeric argument, got %v", argType)
			}
			return argType, nil

		case "POW":
			if len(e.Args) != 2 {
				return 0, fmt.Errorf("POW expects 2 arguments, got %d", len(e.Args))
			}
			arg0Type, err := typeOfExpression(e.Args[0], schema)
			if err != nil {
				return 0, err
			}
			arg1Type, err := typeOfExpression(e.Args[1], schema)
			if err != nil {
				return 0, err
			}
			if (arg0Type != record.Integer && arg0Type != record.Float) ||
				(arg1Type != record.Integer && arg1Type != record.Float) {
				return 0, fmt.Errorf("POW expects numeric arguments, got %v and %v", arg0Type, arg1Type)
			}
			return record.Float, nil

		case "DOT":
			if len(e.Args) != 2 {
				return 0, fmt.Errorf("DOT expects 2 arguments, got %d", len(e.Args))
			}
			arg0Type, err := typeOfExpression(e.Args[0], schema)
			if err != nil {
				return 0, err
			}
			arg1Type, err := typeOfExpression(e.Args[1], schema)
			if err != nil {
				return 0, err
			}
			if arg0Type != record.Varchar || arg1Type != record.Varchar {
				return 0, fmt.Errorf("DOT expects VARCHAR arguments (vector format), got %v and %v", arg0Type, arg1Type)
			}
			return record.Float, nil

		case "CROSS":
			if len(e.Args) != 2 {
				return 0, fmt.Errorf("CROSS expects 2 arguments, got %d", len(e.Args))
			}
			arg0Type, err := typeOfExpression(e.Args[0], schema)
			if err != nil {
				return 0, err
			}
			arg1Type, err := typeOfExpression(e.Args[1], schema)
			if err != nil {
				return 0, err
			}
			if arg0Type != record.Varchar || arg1Type != record.Varchar {
				return 0, fmt.Errorf("CROSS expects VARCHAR arguments (vector format), got %v and %v", arg0Type, arg1Type)
			}
			return record.Varchar, nil

		case "POINT":
			if len(e.Args) != 2 {
				return 0, fmt.Errorf("POINT expects 2 arguments, got %d", len(e.Args))
			}
			arg0, err := typeOfExpression(e.Args[0], schema)
			if err != nil {
				return 0, err
			}
			arg1, err := typeOfExpression(e.Args[1], schema)
			if err != nil {
				return 0, err
			}
			if (arg0 != record.Integer && arg0 != record.Float) ||
				(arg1 != record.Integer && arg1 != record.Float) {
				return 0, fmt.Errorf("POINT expects numeric arguments, got %v and %v", arg0, arg1)
			}
			return record.Point, nil

		case "POLYGON":
			if len(e.Args) < 3 {
				return 0, fmt.Errorf("POLYGON expects at least 3 arguments, got %d", len(e.Args))
			}
			for i, argExpr := range e.Args {
				argType, err := typeOfExpression(argExpr, schema)
				if err != nil {
					return 0, err
				}
				if argType != record.Point {
					return 0, fmt.Errorf("POLYGON expects POINT arguments, got %v at position %d", argType, i)
				}
			}
			return record.Polygon, nil

		case "ST_GEOMFROMTEXT":
			if len(e.Args) != 1 {
				return 0, fmt.Errorf("ST_GEOMFROMTEXT expects 1 argument, got %d", len(e.Args))
			}
			arg0, err := typeOfExpression(e.Args[0], schema)
			if err != nil {
				return 0, err
			}
			if arg0 != record.Varchar {
				return 0, fmt.Errorf("ST_GEOMFROMTEXT expects VARCHAR argument, got %v", arg0)
			}
			if strLit, ok := e.Args[0].(*ast.StringLiteral); ok {
				trimmed := strings.TrimSpace(strings.ToUpper(strLit.Value))
				if strings.HasPrefix(trimmed, "POLYGON") {
					return record.Polygon, nil
				}
			}
			return record.Point, nil

		case "DISTANCE":
			if len(e.Args) != 2 {
				return 0, fmt.Errorf("DISTANCE expects 2 arguments, got %d", len(e.Args))
			}
			arg0, err := typeOfExpression(e.Args[0], schema)
			if err != nil {
				return 0, err
			}
			arg1, err := typeOfExpression(e.Args[1], schema)
			if err != nil {
				return 0, err
			}
			if arg0 != record.Point || arg1 != record.Point {
				return 0, fmt.Errorf("DISTANCE expects POINT arguments, got %v and %v", arg0, arg1)
			}
			return record.Float, nil

		case "INTERSECTS":
			if len(e.Args) != 2 {
				return 0, fmt.Errorf("INTERSECTS expects 2 arguments, got %d", len(e.Args))
			}
			arg0, err := typeOfExpression(e.Args[0], schema)
			if err != nil {
				return 0, err
			}
			arg1, err := typeOfExpression(e.Args[1], schema)
			if err != nil {
				return 0, err
			}
			if (arg0 != record.Point && arg0 != record.Polygon) ||
				(arg1 != record.Point && arg1 != record.Polygon) {
				return 0, fmt.Errorf("INTERSECTS expects POINT or POLYGON arguments, got %v and %v", arg0, arg1)
			}
			return record.Integer, nil

		case "AREA":
			if len(e.Args) != 1 {
				return 0, fmt.Errorf("AREA expects 1 argument, got %d", len(e.Args))
			}
			arg0, err := typeOfExpression(e.Args[0], schema)
			if err != nil {
				return 0, err
			}
			if arg0 != record.Polygon {
				return 0, fmt.Errorf("AREA expects POLYGON argument, got %v", arg0)
			}
			return record.Float, nil

		default:
			return 0, fmt.Errorf("unknown function: %s", fnName)
		}
	default:
		return 0, fmt.Errorf("unsupported expression type: %T", expr)
	}
}

// validateExpression checks that all identifiers and functions inside an expression are valid.
func validateExpression(expr ast.Expression, schema *record.Schema) error {
	_, err := typeOfExpression(expr, schema)
	return err
}

// validateCondition checks that the filter condition is semantically valid for the schema.
func validateCondition(expr ast.Expression, schema *record.Schema) error {
	return validateExpression(expr, schema)
}

// executeProject handles column projection on top of a child node.
func (e *Executor) executeProject(tx *transaction.Transaction, n *planner.ProjectNode) (*Result, error) {
	childResult, err := e.ExecuteWithTx(tx, n.Child)
	if err != nil {
		return nil, err
	}

	// Wildcard: return all columns
	if len(n.Fields) == 1 {
		if ident, ok := n.Fields[0].(*ast.Identifier); ok && ident.Value == "*" {
			return childResult, nil
		}
	}

	schema, err := e.schemaFromChild(n.Child)
	if err != nil {
		return nil, err
	}

	// Validate each expression in the projection fields
	for _, fieldExpr := range n.Fields {
		if err := validateExpression(fieldExpr, schema); err != nil {
			return nil, err
		}
	}

	var projected []Row
	for _, row := range childResult.Rows {
		var vals []record.Value
		for _, fieldExpr := range n.Fields {
			val, err := evalRowExpression(fieldExpr, row, schema)
			if err != nil {
				return nil, fmt.Errorf("projection evaluation error: %w", err)
			}
			vals = append(vals, val)
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
	case *planner.IndexScanNode:
		return e.tm.GetSchema(n.Table)
	case *planner.FilterNode:
		return e.schemaFromChild(n.Child)
	default:
		return nil, fmt.Errorf("cannot resolve schema from node type %T", child)
	}
}

func evalComplex(expr ast.Expression) (datatypes.Complex, error) {
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return datatypes.Complex{Real: float64(e.Value), Imag: 0}, nil
	case *ast.FloatLiteral:
		return datatypes.Complex{Real: e.Value, Imag: 0}, nil
	case *ast.ImaginaryLiteral:
		return datatypes.Complex{Real: 0, Imag: e.Value}, nil
	case *ast.PrefixExpression:
		if e.Operator == "-" {
			val, err := evalComplex(e.Right)
			if err != nil {
				return datatypes.Complex{}, err
			}
			return datatypes.Complex{Real: -val.Real, Imag: -val.Imag}, nil
		}
		return datatypes.Complex{}, fmt.Errorf("unsupported complex prefix operator: %s", e.Operator)
	case *ast.InfixExpression:
		if e.Operator != "+" && e.Operator != "-" {
			return datatypes.Complex{}, fmt.Errorf("unsupported complex operator: %s", e.Operator)
		}
		leftReal, err := evalRealPart(e.Left)
		if err != nil {
			return datatypes.Complex{}, err
		}
		rightImag, err := evalImagPart(e.Right)
		if err != nil {
			return datatypes.Complex{}, err
		}
		if e.Operator == "-" {
			rightImag = -rightImag
		}
		return datatypes.Complex{Real: leftReal, Imag: rightImag}, nil
	default:
		return datatypes.Complex{}, fmt.Errorf("unsupported expression type for complex: %T", expr)
	}
}

func evalRealPart(expr ast.Expression) (float64, error) {
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return float64(e.Value), nil
	case *ast.FloatLiteral:
		return e.Value, nil
	case *ast.PrefixExpression:
		if e.Operator == "-" {
			val, err := evalRealPart(e.Right)
			if err != nil {
				return 0, err
			}
			return -val, nil
		}
	}
	return 0, fmt.Errorf("expected real number, got %T", expr)
}

func evalImagPart(expr ast.Expression) (float64, error) {
	switch e := expr.(type) {
	case *ast.ImaginaryLiteral:
		return e.Value, nil
	case *ast.PrefixExpression:
		if e.Operator == "-" {
			val, err := evalImagPart(e.Right)
			if err != nil {
				return 0, err
			}
			return -val, nil
		}
	}
	return 0, fmt.Errorf("expected imaginary number, got %T", expr)
}

func evalFloatVal(expr ast.Expression) (float64, error) {
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return float64(e.Value), nil
	case *ast.FloatLiteral:
		return e.Value, nil
	case *ast.PrefixExpression:
		if e.Operator == "-" {
			val, err := evalFloatVal(e.Right)
			if err != nil {
				return 0, err
			}
			return -val, nil
		}
	}
	return 0, fmt.Errorf("expected number, got %T", expr)
}

func evalVector(expr ast.Expression) (datatypes.Vector, error) {
	arr, ok := expr.(*ast.ArrayLiteral)
	if !ok {
		return nil, fmt.Errorf("expected array literal for vector, got %T", expr)
	}
	vec := make(datatypes.Vector, len(arr.Elements))
	for i, el := range arr.Elements {
		val, err := evalFloatVal(el)
		if err != nil {
			return nil, fmt.Errorf("vector element %d: %w", i, err)
		}
		vec[i] = val
	}
	return vec, nil
}

func evalMatrix(expr ast.Expression) (datatypes.Matrix, error) {
	arr, ok := expr.(*ast.ArrayLiteral)
	if !ok {
		return nil, fmt.Errorf("expected array literal for matrix, got %T", expr)
	}
	if len(arr.Elements) == 0 {
		return make(datatypes.Matrix, 0), nil
	}
	mat := make(datatypes.Matrix, len(arr.Elements))
	colCount := -1
	for i, el := range arr.Elements {
		rowVec, err := evalVector(el)
		if err != nil {
			return nil, fmt.Errorf("matrix row %d: %w", i, err)
		}
		if colCount == -1 {
			colCount = len(rowVec)
		} else if len(rowVec) != colCount {
			return nil, fmt.Errorf("matrix row %d: dimension mismatch (expected %d columns, got %d)", i, colCount, len(rowVec))
		}
		mat[i] = rowVec
	}
	return mat, nil
}

func flattenArrayLiteral(expr ast.Expression) ([]int, []float64, error) {
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return nil, []float64{float64(e.Value)}, nil
	case *ast.FloatLiteral:
		return nil, []float64{e.Value}, nil
	case *ast.PrefixExpression:
		if e.Operator == "-" {
			shape, data, err := flattenArrayLiteral(e.Right)
			if err != nil {
				return nil, nil, err
			}
			if len(data) != 1 || len(shape) != 0 {
				return nil, nil, fmt.Errorf("cannot negate a multi-dimensional array directly")
			}
			data[0] = -data[0]
			return nil, data, nil
		}
		return nil, nil, fmt.Errorf("unsupported tensor prefix operator: %s", e.Operator)
	case *ast.ArrayLiteral:
		if len(e.Elements) == 0 {
			return []int{0}, nil, nil
		}
		var firstShape []int
		var allData []float64
		for idx, elem := range e.Elements {
			subShape, subData, err := flattenArrayLiteral(elem)
			if err != nil {
				return nil, nil, err
			}
			if idx == 0 {
				firstShape = subShape
			} else {
				if len(subShape) != len(firstShape) {
					return nil, nil, fmt.Errorf("tensor shape mismatch at element %d", idx)
				}
				for dimIdx := range firstShape {
					if firstShape[dimIdx] != subShape[dimIdx] {
						return nil, nil, fmt.Errorf("tensor shape mismatch at element %d", idx)
					}
				}
			}
			allData = append(allData, subData...)
		}
		shape := append([]int{len(e.Elements)}, firstShape...)
		return shape, allData, nil
	default:
		return nil, nil, fmt.Errorf("invalid element type in tensor: %T", expr)
	}
}

func evalTensor(expr ast.Expression) (datatypes.Tensor, error) {
	arr, ok := expr.(*ast.ArrayLiteral)
	if !ok {
		return datatypes.Tensor{}, fmt.Errorf("expected array literal for tensor, got %T", expr)
	}
	shape, data, err := flattenArrayLiteral(arr)
	if err != nil {
		return datatypes.Tensor{}, err
	}
	return datatypes.Tensor{Shape: shape, Data: data}, nil
}

// evalExpression converts an AST Expression to a typed record.Value based on the expected column type.
func evalExpression(expr ast.Expression, colType record.TypeID) (record.Value, error) {
	switch colType {
	case record.Complex:
		comp, err := evalComplex(expr)
		if err != nil {
			return record.Value{}, err
		}
		return record.Value{Type: record.Complex, Comp: comp}, nil
	case record.Vector:
		vec, err := evalVector(expr)
		if err != nil {
			return record.Value{}, err
		}
		return record.Value{Type: record.Vector, Vec: vec}, nil
	case record.Matrix:
		mat, err := evalMatrix(expr)
		if err != nil {
			return record.Value{}, err
		}
		return record.Value{Type: record.Matrix, Mat: mat}, nil
	case record.Tensor:
		ten, err := evalTensor(expr)
		if err != nil {
			return record.Value{}, err
		}
		return record.Value{Type: record.Tensor, Ten: ten}, nil
	case record.Point:
		val, err := evalRowExpression(expr, Row{}, nil)
		if err != nil {
			return record.Value{}, err
		}
		if val.Type == record.Point {
			return val, nil
		}
		if val.Type == record.Varchar {
			parsed, err := datatypes.ParseWKT(val.Str)
			if err != nil {
				return record.Value{}, err
			}
			pt, ok := parsed.(datatypes.Point)
			if !ok {
				return record.Value{}, fmt.Errorf("WKT string did not represent a POINT")
			}
			return record.Value{Type: record.Point, Pt: pt}, nil
		}
		return record.Value{}, fmt.Errorf("type mismatch: expected Point, got %v", val.Type)
	case record.Polygon:
		val, err := evalRowExpression(expr, Row{}, nil)
		if err != nil {
			return record.Value{}, err
		}
		if val.Type == record.Polygon {
			return val, nil
		}
		if val.Type == record.Varchar {
			parsed, err := datatypes.ParseWKT(val.Str)
			if err != nil {
				return record.Value{}, err
			}
			poly, ok := parsed.(datatypes.Polygon)
			if !ok {
				return record.Value{}, fmt.Errorf("WKT string did not represent a POLYGON")
			}
			return record.Value{Type: record.Polygon, Poly: poly}, nil
		}
		return record.Value{}, fmt.Errorf("type mismatch: expected Polygon, got %v", val.Type)
	}

	// Fallback to general expression evaluation for scalar and other types
	val, err := evalRowExpression(expr, Row{}, nil)
	if err != nil {
		return record.Value{}, err
	}
	if val.Type == colType {
		return val, nil
	}
	// Implicit cast from Integer to Float
	if val.Type == record.Integer && colType == record.Float {
		return record.Value{Type: record.Float, Flt: float64(val.Int)}, nil
	}
	return record.Value{}, fmt.Errorf("type mismatch: expected %v, got %v", colType, val.Type)
}

// evalRowExpression evaluates an AST Expression to a record.Value on a physical row.
func evalRowExpression(expr ast.Expression, row Row, schema *record.Schema) (record.Value, error) {
	switch e := expr.(type) {
	case *ast.ImaginaryLiteral:
		return record.Value{Type: record.Complex, Comp: datatypes.Complex{Real: 0, Imag: e.Value}}, nil

	case *ast.ArrayLiteral:
		if vec, err := evalVector(e); err == nil {
			return record.Value{Type: record.Vector, Vec: vec}, nil
		}
		if mat, err := evalMatrix(e); err == nil {
			return record.Value{Type: record.Matrix, Mat: mat}, nil
		}
		if ten, err := evalTensor(e); err == nil {
			return record.Value{Type: record.Tensor, Ten: ten}, nil
		}
		return record.Value{}, fmt.Errorf("invalid array literal format")

	case *ast.IntegerLiteral:
		return record.Value{Type: record.Integer, Int: int32(e.Value)}, nil

	case *ast.FloatLiteral:
		return record.Value{Type: record.Float, Flt: e.Value}, nil

	case *ast.StringLiteral:
		return record.Value{Type: record.Varchar, Str: e.Value}, nil

	case *ast.Identifier:
		if schema == nil {
			return record.Value{}, fmt.Errorf("schema not available to resolve column %q", e.Value)
		}
		for i, col := range schema.Columns {
			if strings.EqualFold(col.Name, e.Value) {
				return row.Values[i], nil
			}
		}
		return record.Value{}, fmt.Errorf("column %q not found in schema", e.Value)

	case *ast.PrefixExpression:
		rightVal, err := evalRowExpression(e.Right, row, schema)
		if err != nil {
			return record.Value{}, err
		}
		if e.Operator == "-" {
			switch rightVal.Type {
			case record.Integer:
				return record.Value{Type: record.Integer, Int: -rightVal.Int}, nil
			case record.Float:
				return record.Value{Type: record.Float, Flt: -rightVal.Flt}, nil
			default:
				return record.Value{}, fmt.Errorf("operator %q cannot be applied to type %v", e.Operator, rightVal.Type)
			}
		}
		return record.Value{}, fmt.Errorf("unsupported prefix operator %q", e.Operator)

	case *ast.InfixExpression:
		leftVal, err := evalRowExpression(e.Left, row, schema)
		if err != nil {
			return record.Value{}, err
		}
		rightVal, err := evalRowExpression(e.Right, row, schema)
		if err != nil {
			return record.Value{}, err
		}

		// Handle complex binary operations +, -
		if leftVal.Type == record.Complex || rightVal.Type == record.Complex {
			lComp, errL := toComplexVal(leftVal)
			rComp, errR := toComplexVal(rightVal)
			if errL == nil && errR == nil {
				switch e.Operator {
				case "+":
					return record.Value{Type: record.Complex, Comp: datatypes.Complex{
						Real: lComp.Real + rComp.Real,
						Imag: lComp.Imag + rComp.Imag,
					}}, nil
				case "-":
					return record.Value{Type: record.Complex, Comp: datatypes.Complex{
						Real: lComp.Real - rComp.Real,
						Imag: lComp.Imag - rComp.Imag,
					}}, nil
				}
			}
		}

		// Handle numeric binary operations +, -, *, /
		if (leftVal.Type == record.Integer || leftVal.Type == record.Float) &&
			(rightVal.Type == record.Integer || rightVal.Type == record.Float) {
			isFloat := leftVal.Type == record.Float || rightVal.Type == record.Float
			if isFloat {
				lVal, _ := toFloatVal(leftVal)
				rVal, _ := toFloatVal(rightVal)
				switch e.Operator {
				case "+":
					return record.Value{Type: record.Float, Flt: lVal + rVal}, nil
				case "-":
					return record.Value{Type: record.Float, Flt: lVal - rVal}, nil
				case "*":
					return record.Value{Type: record.Float, Flt: lVal * rVal}, nil
				case "/":
					if rVal == 0 {
						return record.Value{}, fmt.Errorf("division by zero")
					}
					return record.Value{Type: record.Float, Flt: lVal / rVal}, nil
				}
			} else {
				lVal := leftVal.Int
				rVal := rightVal.Int
				switch e.Operator {
				case "+":
					return record.Value{Type: record.Integer, Int: lVal + rVal}, nil
				case "-":
					return record.Value{Type: record.Integer, Int: lVal - rVal}, nil
				case "*":
					return record.Value{Type: record.Integer, Int: lVal * rVal}, nil
				case "/":
					if rVal == 0 {
						return record.Value{}, fmt.Errorf("division by zero")
					}
					return record.Value{Type: record.Integer, Int: lVal / rVal}, nil
				}
			}
		}

		// If it's a comparison or binary logic
		matched, err := compareValues(leftVal, e.Operator, rightVal)
		if err != nil {
			return record.Value{}, err
		}
		if matched {
			return record.Value{Type: record.Integer, Int: 1}, nil
		}
		return record.Value{Type: record.Integer, Int: 0}, nil

	case *ast.CallExpression:
		argVals := make([]record.Value, len(e.Args))
		for i, argExpr := range e.Args {
			val, err := evalRowExpression(argExpr, row, schema)
			if err != nil {
				return record.Value{}, fmt.Errorf("argument %d of function %s: %w", i, e.Function, err)
			}
			argVals[i] = val
		}
		fn, ok := functions.Registry[e.Function]
		if !ok {
			return record.Value{}, fmt.Errorf("unknown function: %s", e.Function)
		}
		return fn(argVals)

	default:
		return record.Value{}, fmt.Errorf("unsupported row expression type: %T", expr)
	}
}

// executeIndexScan handles index-assisted scans by calling the catalog.TableManager.
func (e *Executor) executeIndexScan(tx *transaction.Transaction, n *planner.IndexScanNode) (*Result, error) {
	var lowKey, highKey btree.Key
	if n.LowVal != nil {
		lowKey = toBtreeKey(*n.LowVal, btree.KeyType(n.KeyType))
	} else {
		lowKey = getMinKey(btree.KeyType(n.KeyType))
	}

	if n.HighVal != nil {
		highKey = toBtreeKey(*n.HighVal, btree.KeyType(n.KeyType))
	} else {
		highKey = getMaxKey(btree.KeyType(n.KeyType))
	}

	records, err := e.tm.ReadRecordsIndexed(tx, n.Table, n.ColumnName, page.PageID(n.IndexRootID), btree.KeyType(n.KeyType), lowKey, highKey)
	if err != nil {
		return nil, fmt.Errorf("index scan failed on %s.%s: %w", n.Table, n.ColumnName, err)
	}

	rows := make([]Row, len(records))
	for i, r := range records {
		rows[i] = Row{Values: r.Values}
	}

	return &Result{Rows: rows}, nil
}

func toBtreeKey(v record.Value, kt btree.KeyType) btree.Key {
	switch kt {
	case btree.KeyInt:
		return btree.Key{Type: btree.KeyInt, IVal: v.Int}
	case btree.KeyFloat:
		return btree.Key{Type: btree.KeyFloat, FVal: v.Flt}
	case btree.KeyVarchar:
		return btree.Key{Type: btree.KeyVarchar, SVal: v.Str}
	}
	return btree.Key{}
}

func getMinKey(kt btree.KeyType) btree.Key {
	switch kt {
	case btree.KeyInt:
		return btree.Key{Type: btree.KeyInt, IVal: math.MinInt32}
	case btree.KeyFloat:
		return btree.Key{Type: btree.KeyFloat, FVal: -math.MaxFloat64}
	case btree.KeyVarchar:
		return btree.Key{Type: btree.KeyVarchar, SVal: ""}
	}
	return btree.Key{}
}

func getMaxKey(kt btree.KeyType) btree.Key {
	switch kt {
	case btree.KeyInt:
		return btree.Key{Type: btree.KeyInt, IVal: math.MaxInt32}
	case btree.KeyFloat:
		return btree.Key{Type: btree.KeyFloat, FVal: math.MaxFloat64}
	case btree.KeyVarchar:
		return btree.Key{Type: btree.KeyVarchar, SVal: strings.Repeat(string(rune(0xff)), 255)}
	}
	return btree.Key{}
}

func toFloatVal(v record.Value) (float64, error) {
	switch v.Type {
	case record.Float:
		return v.Flt, nil
	case record.Integer:
		return float64(v.Int), nil
	default:
		return 0, fmt.Errorf("cannot convert type %v to float", v.Type)
	}
}

func toComplexVal(v record.Value) (datatypes.Complex, error) {
	switch v.Type {
	case record.Complex:
		return v.Comp, nil
	case record.Float:
		return datatypes.Complex{Real: v.Flt, Imag: 0}, nil
	case record.Integer:
		return datatypes.Complex{Real: float64(v.Int), Imag: 0}, nil
	default:
		return datatypes.Complex{}, fmt.Errorf("cannot convert type %v to complex", v.Type)
	}
}

// evalCondition evaluates an AST expression as a boolean condition on a row.
func evalCondition(expr ast.Expression, row Row, schema *record.Schema) (bool, error) {
	val, err := evalRowExpression(expr, row, schema)
	if err != nil {
		return false, err
	}
	if val.Type == record.Integer {
		return val.Int != 0, nil
	}
	return false, fmt.Errorf("condition evaluated to non-boolean value type %v", val.Type)
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
	case record.Complex:
		l, r := left.Comp, right.Comp
		switch op {
		case "=", "==":
			return l.Equals(r), nil
		case "!=":
			return !l.Equals(r), nil
		}
	case record.Vector:
		l, r := left.Vec, right.Vec
		switch op {
		case "=", "==":
			return l.Equals(r), nil
		case "!=":
			return !l.Equals(r), nil
		}
	case record.Matrix:
		l, r := left.Mat, right.Mat
		switch op {
		case "=", "==":
			return l.Equals(r), nil
		case "!=":
			return !l.Equals(r), nil
		}
	case record.Tensor:
		l, r := left.Ten, right.Ten
		switch op {
		case "=", "==":
			return l.Equals(r), nil
		case "!=":
			return !l.Equals(r), nil
		}
	case record.Point:
		l, r := left.Pt, right.Pt
		switch op {
		case "=", "==":
			return l.Equals(r), nil
		case "!=":
			return !l.Equals(r), nil
		}
	case record.Polygon:
		l, r := left.Poly, right.Poly
		switch op {
		case "=", "==":
			return l.Equals(r), nil
		case "!=":
			return !l.Equals(r), nil
		}
	}
	return false, fmt.Errorf("unsupported operator %q for type %v", op, left.Type)
}

func (e *Executor) executeShowTables(tx *transaction.Transaction, n *planner.ShowTablesNode) (*Result, error) {
	tables := e.tm.ListTables()
	rows := make([]Row, len(tables))
	for i, name := range tables {
		rows[i] = Row{
			Values: []record.Value{
				{Type: record.Varchar, Str: name},
			},
		}
	}
	return &Result{Rows: rows}, nil
}
