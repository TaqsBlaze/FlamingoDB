package planner

import (
	"errors"
	"fmt"
	"strings"

	"flamingodb/internal/parser/ast"
	"flamingodb/internal/storage/record"
)

// PlanType represents the type of logical plan node.
type PlanType string

const (
	PlanScan        PlanType = "Scan"
	PlanFilter      PlanType = "Filter"
	PlanProject     PlanType = "Project"
	PlanInsert      PlanType = "Insert"
	PlanUpdate      PlanType = "Update"
	PlanDelete      PlanType = "Delete"
	PlanCreateTable PlanType = "CreateTable"
)

// PlanNode is the common interface for all logical plan nodes.
type PlanNode interface {
	Type() PlanType
	Children() []PlanNode
	String() string
}

// ScanNode represents scanning a table.
type ScanNode struct {
	Table string
}

// Type returns PlanScan.
func (n *ScanNode) Type() PlanType { return PlanScan }

// Children returns nil (Scan is a leaf node).
func (n *ScanNode) Children() []PlanNode { return nil }

// String returns a string representation of ScanNode.
func (n *ScanNode) String() string {
	return fmt.Sprintf("Scan(%s)", n.Table)
}

// FilterNode filters rows according to a condition.
type FilterNode struct {
	Child     PlanNode
	Condition ast.Expression
}

// Type returns PlanFilter.
func (n *FilterNode) Type() PlanType { return PlanFilter }

// Children returns the child node of FilterNode.
func (n *FilterNode) Children() []PlanNode { return []PlanNode{n.Child} }

// String returns a string representation of FilterNode.
func (n *FilterNode) String() string {
	return fmt.Sprintf("Filter(%s)", n.Condition.String())
}

// ProjectNode projects a list of fields/columns.
type ProjectNode struct {
	Child  PlanNode
	Fields []string
}

// Type returns PlanProject.
func (n *ProjectNode) Type() PlanType { return PlanProject }

// Children returns the child node of ProjectNode.
func (n *ProjectNode) Children() []PlanNode { return []PlanNode{n.Child} }

// String returns a string representation of ProjectNode.
func (n *ProjectNode) String() string {
	return fmt.Sprintf("Project(%s)", strings.Join(n.Fields, ", "))
}

// InsertNode inserts values into a table.
type InsertNode struct {
	Table  string
	Values []ast.Expression
}

// Type returns PlanInsert.
func (n *InsertNode) Type() PlanType { return PlanInsert }

// Children returns nil (Insert is a leaf node).
func (n *InsertNode) Children() []PlanNode { return nil }

// String returns a string representation of InsertNode.
func (n *InsertNode) String() string {
	var vals []string
	for _, v := range n.Values {
		vals = append(vals, v.String())
	}
	return fmt.Sprintf("Insert(table=%s, values=[%s])", n.Table, strings.Join(vals, ", "))
}

// UpdateNode updates rows matching a condition.
type UpdateNode struct {
	Table string
	Set   map[string]ast.Expression
	Child PlanNode
}

// Type returns PlanUpdate.
func (n *UpdateNode) Type() PlanType { return PlanUpdate }

// Children returns the child node of UpdateNode.
func (n *UpdateNode) Children() []PlanNode { return []PlanNode{n.Child} }

// String returns a string representation of UpdateNode.
func (n *UpdateNode) String() string {
	var sets []string
	for col, val := range n.Set {
		sets = append(sets, fmt.Sprintf("%s=%s", col, val.String()))
	}
	return fmt.Sprintf("Update(table=%s, set=[%s])", n.Table, strings.Join(sets, ", "))
}

// DeleteNode deletes rows matching a condition.
type DeleteNode struct {
	Table string
	Child PlanNode
}

// Type returns PlanDelete.
func (n *DeleteNode) Type() PlanType { return PlanDelete }

// Children returns the child node of DeleteNode.
func (n *DeleteNode) Children() []PlanNode { return []PlanNode{n.Child} }

// String returns a string representation of DeleteNode.
func (n *DeleteNode) String() string {
	return fmt.Sprintf("Delete(table=%s)", n.Table)
}

// CreateTableNode creates a table with specified columns.
type CreateTableNode struct {
	Table   string
	Columns []ast.ColumnDef
}

// Type returns PlanCreateTable.
func (n *CreateTableNode) Type() PlanType { return PlanCreateTable }

// Children returns nil (CreateTable is a leaf node).
func (n *CreateTableNode) Children() []PlanNode { return nil }

// String returns a string representation of CreateTableNode.
func (n *CreateTableNode) String() string {
	var cols []string
	for _, c := range n.Columns {
		cols = append(cols, fmt.Sprintf("%s %s", c.Name, c.Type))
	}
	return fmt.Sprintf("CreateTable(table=%s, columns=[%s])", n.Table, strings.Join(cols, ", "))
}

// MapStringToTypeID converts a string representation of a type (e.g. "INT", "FLOAT", "VARCHAR") into a record.TypeID.
func MapStringToTypeID(t string) (record.TypeID, error) {
	switch strings.ToUpper(t) {
	case "INT", "INTEGER":
		return record.Integer, nil
	case "FLOAT", "DOUBLE", "REAL":
		return record.Float, nil
	case "VARCHAR", "STRING", "TEXT":
		return record.Varchar, nil
	default:
		return 0, fmt.Errorf("unknown type: %s", t)
	}
}

// ToSchema converts the logical CreateTableNode columns to a record.Schema.
func (n *CreateTableNode) ToSchema() (*record.Schema, error) {
	cols := make([]record.Column, len(n.Columns))
	for i, c := range n.Columns {
		t, err := MapStringToTypeID(c.Type)
		if err != nil {
			return nil, err
		}
		cols[i] = record.Column{
			Name: c.Name,
			Type: t,
		}
	}
	return record.NewSchema(cols), nil
}

// Planner converts AST nodes into logical plan nodes.
type Planner struct{}

// New creates a new Planner.
func New() *Planner {
	return &Planner{}
}

// Plan converts an AST statement into a PlanNode.
func (p *Planner) Plan(stmt ast.Statement) (PlanNode, error) {
	if stmt == nil {
		return nil, errors.New("cannot plan nil statement")
	}

	switch s := stmt.(type) {
	case *ast.SelectStatement:
		return p.planSelect(s)
	case *ast.InsertStatement:
		return p.planInsert(s)
	case *ast.UpdateStatement:
		return p.planUpdate(s)
	case *ast.DeleteStatement:
		return p.planDelete(s)
	case *ast.CreateTableStatement:
		return p.planCreateTable(s)
	default:
		return nil, fmt.Errorf("unsupported statement type: %T", stmt)
	}
}

func (p *Planner) planSelect(stmt *ast.SelectStatement) (PlanNode, error) {
	if stmt.Table == "" {
		return nil, errors.New("select statement must specify a table")
	}

	// Leaf: Scan table
	var node PlanNode = &ScanNode{Table: stmt.Table}

	// If there is a WHERE clause, wrap with FilterNode
	if stmt.Where != nil {
		node = &FilterNode{
			Child:     node,
			Condition: stmt.Where,
		}
	}

	// Wrap with ProjectNode
	if len(stmt.Fields) > 0 {
		node = &ProjectNode{
			Child:  node,
			Fields: stmt.Fields,
		}
	}

	return node, nil
}

func (p *Planner) planInsert(stmt *ast.InsertStatement) (PlanNode, error) {
	if stmt.Table == "" {
		return nil, errors.New("insert statement must specify a table")
	}
	if len(stmt.Values) == 0 {
		return nil, errors.New("insert statement must specify values")
	}

	return &InsertNode{
		Table:  stmt.Table,
		Values: stmt.Values,
	}, nil
}

func (p *Planner) planUpdate(stmt *ast.UpdateStatement) (PlanNode, error) {
	if stmt.Table == "" {
		return nil, errors.New("update statement must specify a table")
	}
	if len(stmt.Set) == 0 {
		return nil, errors.New("update statement must specify at least one assignment in SET")
	}

	// The source of rows to update
	var child PlanNode = &ScanNode{Table: stmt.Table}
	if stmt.Where != nil {
		child = &FilterNode{
			Child:     child,
			Condition: stmt.Where,
		}
	}

	return &UpdateNode{
		Table: stmt.Table,
		Set:   stmt.Set,
		Child: child,
	}, nil
}

func (p *Planner) planDelete(stmt *ast.DeleteStatement) (PlanNode, error) {
	if stmt.Table == "" {
		return nil, errors.New("delete statement must specify a table")
	}

	// The source of rows to delete
	var child PlanNode = &ScanNode{Table: stmt.Table}
	if stmt.Where != nil {
		child = &FilterNode{
			Child:     child,
			Condition: stmt.Where,
		}
	}

	return &DeleteNode{
		Table: stmt.Table,
		Child: child,
	}, nil
}

func (p *Planner) planCreateTable(stmt *ast.CreateTableStatement) (PlanNode, error) {
	if stmt.Table == "" {
		return nil, errors.New("create table statement must specify a table name")
	}
	if len(stmt.Columns) == 0 {
		return nil, errors.New("create table statement must specify at least one column")
	}

	return &CreateTableNode{
		Table:   stmt.Table,
		Columns: stmt.Columns,
	}, nil
}
