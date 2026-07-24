package planner

import (
	"errors"
	"fmt"
	"strings"

	"github.com/TaqsBlaze/FlamingoDB/internal/parser/ast"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/record"
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
	PlanDropTable   PlanType = "DropTable"
	PlanCreateIndex PlanType = "CreateIndex"
	PlanDropIndex   PlanType = "DropIndex"
	PlanShowIndexes PlanType = "ShowIndexes"
	PlanIndexScan   PlanType = "IndexScan"
	PlanShowTables  PlanType = "ShowTables"
	PlanJoin        PlanType = "Join"
	PlanAggregate   PlanType = "Aggregate"
	PlanSort        PlanType = "Sort"
	PlanDistinct    PlanType = "Distinct"
	PlanLimitOffset PlanType = "LimitOffset"
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
	Fields []ast.Expression
}

// Type returns PlanProject.
func (n *ProjectNode) Type() PlanType { return PlanProject }

// Children returns the child node of ProjectNode.
func (n *ProjectNode) Children() []PlanNode { return []PlanNode{n.Child} }

// String returns a string representation of ProjectNode.
func (n *ProjectNode) String() string {
	var fields []string
	for _, f := range n.Fields {
		fields = append(fields, f.String())
	}
	return fmt.Sprintf("Project(%s)", strings.Join(fields, ", "))
}

// InsertNode inserts values into a table. Optional Next chains another
// InsertNode so a single bulk INSERT can persist multiple rows.
type InsertNode struct {
	Table   string
	Columns []string
	Values  []ast.Expression
	Next    PlanNode
}

// Type returns PlanInsert.
func (n *InsertNode) Type() PlanType { return PlanInsert }

// Children returns the chained next insert node (if any).
func (n *InsertNode) Children() []PlanNode {
	if n.Next == nil {
		return nil
	}
	return []PlanNode{n.Next}
}

// String returns a string representation of InsertNode.
func (n *InsertNode) String() string {
	var vals []string
	for _, v := range n.Values {
		vals = append(vals, v.String())
	}
	if len(n.Columns) > 0 {
		return fmt.Sprintf("Insert(table=%s, columns=[%s], values=[%s])", n.Table, strings.Join(n.Columns, ", "), strings.Join(vals, ", "))
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

// DropTableNode represents a DROP TABLE query.
type DropTableNode struct {
	Table string
}

// Type returns PlanDropTable.
func (n *DropTableNode) Type() PlanType { return PlanDropTable }

// Children returns nil.
func (n *DropTableNode) Children() []PlanNode { return nil }

// String returns a string representation.
func (n *DropTableNode) String() string {
	return fmt.Sprintf("DropTable(table=%s)", n.Table)
}

// ShowTablesNode represents a SHOW TABLES query.
type ShowTablesNode struct{}

// Type returns PlanShowTables.
func (n *ShowTablesNode) Type() PlanType { return PlanShowTables }

// Children returns nil.
func (n *ShowTablesNode) Children() []PlanNode { return nil }

// String returns a string representation.
func (n *ShowTablesNode) String() string {
	return "ShowTables"
}

// JoinNode represents a JOIN operation. It wraps the right table with a join
// condition that is evaluated against rows from the left side.
type JoinNode struct {
	Left      PlanNode
	Right     PlanNode
	Condition ast.Expression
	JoinType  string // INNER, LEFT, etc.
}

// Type returns PlanJoin.
func (n *JoinNode) Type() PlanType { return PlanJoin }

// Children returns the left and right child nodes.
func (n *JoinNode) Children() []PlanNode { return []PlanNode{n.Left, n.Right} }

// String returns a string representation of JoinNode.
func (n *JoinNode) String() string {
	if n.JoinType == "" {
		return fmt.Sprintf("Join(%s, %s, ON %s)", n.Left.String(), n.Right.String(), n.Condition.String())
	}
	return fmt.Sprintf("%s JOIN(%s, %s, ON %s)", n.JoinType, n.Left.String(), n.Right.String(), n.Condition.String())
}

// AggregateNode represents GROUP BY and aggregation operations.
type AggregateNode struct {
	GroupBy    []ast.Expression
	Having     ast.Expression
	AggFuncs   map[string]bool   // Aggregate function names that are present
	Aggregates []ast.Expression  // Aggregate expressions extracted from SELECT
	Child      PlanNode
}

// Type returns PlanAggregate.
func (n *AggregateNode) Type() PlanType { return PlanAggregate }

// Children returns the child node.
func (n *AggregateNode) Children() []PlanNode { return []PlanNode{n.Child} }

// String returns a string representation of AggregateNode.
func (n *AggregateNode) String() string {
	var groupByStrs []string
	for _, g := range n.GroupBy {
		groupByStrs = append(groupByStrs, g.String())
	}
	havingStr := ""
	if n.Having != nil {
		havingStr = " HAVING " + n.Having.String()
	}
	var aggFuncs []string
	for f := range n.AggFuncs {
		aggFuncs = append(aggFuncs, f)
	}
	return fmt.Sprintf("Aggregate(GROUP BY [%s]%s AGG [%s])", strings.Join(groupByStrs, ", "), havingStr, strings.Join(aggFuncs, ", "))
}

// SortNode represents an ORDER BY operation.
type SortNode struct {
	OrderBy []ast.OrderBy
	Child   PlanNode
}

// Type returns PlanSort.
func (n *SortNode) Type() PlanType { return PlanSort }

// Children returns the child node.
func (n *SortNode) Children() []PlanNode { return []PlanNode{n.Child} }

// String returns a string representation of SortNode.
func (n *SortNode) String() string {
	var orderByStrs []string
	for _, ob := range n.OrderBy {
		dir := "ASC"
		if !ob.Ascending {
			dir = "DESC"
		}
		orderByStrs = append(orderByStrs, ob.Expression.String()+" "+dir)
	}
	return fmt.Sprintf("Sort(%s)", strings.Join(orderByStrs, ", "))
}

// DistinctNode represents a DISTINCT operation.
type DistinctNode struct {
	Child PlanNode
}

// Type returns PlanDistinct.
func (n *DistinctNode) Type() PlanType { return PlanDistinct }

// Children returns the child node.
func (n *DistinctNode) Children() []PlanNode { return []PlanNode{n.Child} }

// String returns a string representation of DistinctNode.
func (n *DistinctNode) String() string {
	return fmt.Sprintf("Distinct(%s)", n.Child.String())
}

// LimitOffsetNode represents LIMIT and OFFSET operations.
type LimitOffsetNode struct {
	Limit  ast.Expression
	Offset ast.Expression
	Child  PlanNode
}

// Type returns PlanLimitOffset.
func (n *LimitOffsetNode) Type() PlanType { return PlanLimitOffset }

// Children returns the child node.
func (n *LimitOffsetNode) Children() []PlanNode { return []PlanNode{n.Child} }

// String returns a string representation of LimitOffsetNode.
func (n *LimitOffsetNode) String() string {
	var limitOffsetStrs []string
	if n.Limit != nil {
		limitOffsetStrs = append(limitOffsetStrs, "LIMIT "+n.Limit.String())
	}
	if n.Offset != nil {
		limitOffsetStrs = append(limitOffsetStrs, "OFFSET "+n.Offset.String())
	}
	return fmt.Sprintf("LimitOffset(%s)", strings.Join(limitOffsetStrs, " "))
}

// IndexScanNode represents an index-assisted scan on a table.
type IndexScanNode struct {
	Table       string
	ColumnName  string
	IndexRootID uint32
	KeyType     uint8 // btree.KeyType
	LowVal      *record.Value
	HighVal     *record.Value
}

// Type returns PlanIndexScan.
func (n *IndexScanNode) Type() PlanType { return PlanIndexScan }

// Children returns nil (IndexScan is a leaf node).
func (n *IndexScanNode) Children() []PlanNode { return nil }

// String returns a string representation of IndexScanNode.
func (n *IndexScanNode) String() string {
	var lowStr, highStr string = "nil", "nil"
	if n.LowVal != nil {
		lowStr = fmt.Sprintf("%v", n.LowVal)
	}
	if n.HighVal != nil {
		highStr = fmt.Sprintf("%v", n.HighVal)
	}
	return fmt.Sprintf("IndexScan(%s.%s, root=%d, range=[%s, %s])", n.Table, n.ColumnName, n.IndexRootID, lowStr, highStr)
}

// CreateIndexNode represents a CREATE INDEX operation.
type CreateIndexNode struct {
	IndexName  string
	TableName  string
	ColumnName string
	IsUnique   bool
}

// Type returns PlanCreateIndex.
func (n *CreateIndexNode) Type() PlanType { return PlanCreateIndex }

// Children returns nil (CreateIndex is a leaf node).
func (n *CreateIndexNode) Children() []PlanNode { return nil }

// String returns a string representation of CreateIndexNode.
func (n *CreateIndexNode) String() string {
	return fmt.Sprintf("CreateIndex(%s on %s(%s))", n.IndexName, n.TableName, n.ColumnName)
}

// DropIndexNode represents a DROP INDEX operation.
type DropIndexNode struct {
	IndexName  string
	TableName  string
	IfExists   bool
}

// Type returns PlanDropIndex.
func (n *DropIndexNode) Type() PlanType { return PlanDropIndex }

// Children returns nil (DropIndex is a leaf node).
func (n *DropIndexNode) Children() []PlanNode { return nil }

// String returns a string representation of DropIndexNode.
func (n *DropIndexNode) String() string {
	if n.IfExists {
		return fmt.Sprintf("DropIndex(%s on %s IF EXISTS)", n.IndexName, n.TableName)
	}
	return fmt.Sprintf("DropIndex(%s on %s)", n.IndexName, n.TableName)
}

// ShowIndexesNode represents a SHOW INDEXES operation.
type ShowIndexesNode struct {
	TableName string
}

// Type returns PlanShowIndexes.
func (n *ShowIndexesNode) Type() PlanType { return PlanShowIndexes }

// Children returns nil (ShowIndexes is a leaf node).
func (n *ShowIndexesNode) Children() []PlanNode { return nil }

// String returns a string representation of ShowIndexesNode.
func (n *ShowIndexesNode) String() string {
	if n.TableName == "" {
		return "ShowIndexes()"
	}
	return fmt.Sprintf("ShowIndexes(%s)", n.TableName)
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
	case "COMPLEX":
		return record.Complex, nil
	case "VECTOR":
		return record.Vector, nil
	case "MATRIX":
		return record.Matrix, nil
	case "TENSOR":
		return record.Tensor, nil
	case "POINT":
		return record.Point, nil
	case "POLYGON":
		return record.Polygon, nil
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
			Name:          c.Name,
			Type:          t,
			AutoIncrement: hasAttribute(c.Attributes, "AUTO_INCREMENT"),
		}
	}
	return record.NewSchema(cols), nil
}

// hasAttribute reports whether a column attribute list contains the given
// attribute (case-insensitive).
func hasAttribute(attrs []string, name string) bool {
	for _, a := range attrs {
		if strings.EqualFold(a, name) {
			return true
		}
	}
	return false
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
	case *ast.DropTableStatement:
		return p.planDropTable(s)
	case *ast.ShowTablesStatement:
		return &ShowTablesNode{}, nil
	case *ast.CreateIndexStatement:
		return p.planCreateIndex(s)
	case *ast.DropIndexStatement:
		return p.planDropIndex(s)
	case *ast.ShowIndexesStatement:
		return p.planShowIndexes(s)
	default:
		return nil, fmt.Errorf("unsupported statement type: %T", stmt)
	}
}

func (p *Planner) planSelect(stmt *ast.SelectStatement) (PlanNode, error) {
	if stmt.Table == "" {
		return nil, errors.New("select statement must specify a table")
	}

	// Start with scanning the main table
	var node PlanNode = &ScanNode{Table: stmt.Table}

	// Apply WHERE clause filtering
	if stmt.Where != nil {
		node = &FilterNode{
			Child:     node,
			Condition: stmt.Where,
		}
	}

	// Apply JOIN operations
	for _, join := range stmt.Joins {
		rightScan := &ScanNode{Table: join.Table}
		node = &JoinNode{
			Left:      node,
			Right:     rightScan,
			Condition: join.Condition,
			JoinType:  join.Type,
		}
	}

	// Apply GROUP BY and aggregation
	if len(stmt.GroupBy) > 0 || stmt.Having != nil {
		// Determine which aggregates are present in the SELECT clause
		aggFuncs := make(map[string]bool)
		var aggregates []ast.Expression
		for _, field := range stmt.Fields {
			if callExpr, ok := field.(*ast.CallExpression); ok {
				switch strings.ToUpper(callExpr.Function) {
				case "COUNT", "SUM", "AVG", "MIN", "MAX":
					aggFuncs[strings.ToUpper(callExpr.Function)] = true
					aggregates = append(aggregates, field)
				}
			}
		}

		node = &AggregateNode{
			GroupBy:    stmt.GroupBy,
			Having:     stmt.Having,
			AggFuncs:   aggFuncs,
			Aggregates: aggregates,
			Child:      node,
		}
	}

	// Apply DISTINCT
	if stmt.Distinct {
		node = &DistinctNode{
			Child: node,
		}
	}

	// Apply ORDER BY
	if len(stmt.OrderBy) > 0 {
		node = &SortNode{
			OrderBy: stmt.OrderBy,
			Child:   node,
		}
	}

	// Apply LIMIT/OFFSET
	if stmt.Limit != nil || stmt.Offset != nil {
		node = &LimitOffsetNode{
			Limit:  stmt.Limit,
			Offset: stmt.Offset,
			Child:  node,
		}
	}

	// Apply projection (SELECT clause)
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
	if len(stmt.Rows) == 0 {
		return nil, errors.New("insert statement must specify at least one row of values")
	}

	// Each row becomes its own InsertNode so the executor can persist all rows
	// in a single pass. We return the first row's plan and chain the rest as a
	// SequenceNode-like series via a small wrapper.
	rows := stmt.Rows
	first := &InsertNode{
		Table:   stmt.Table,
		Columns: stmt.Columns,
		Values:  rows[0],
	}
	cur := PlanNode(first)
	for i := 1; i < len(rows); i++ {
		cur = &InsertNode{
			Table:   stmt.Table,
			Columns: stmt.Columns,
			Values:  rows[i],
			Next:    cur,
		}
	}
	return cur, nil
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

func (p *Planner) planDropTable(stmt *ast.DropTableStatement) (PlanNode, error) {
	if stmt.Table == "" {
		return nil, errors.New("drop table statement must specify a table")
	}

	return &DropTableNode{
		Table: stmt.Table,
	}, nil
}

func (p *Planner) planCreateIndex(stmt *ast.CreateIndexStatement) (PlanNode, error) {
	if stmt.IndexName == "" {
		return nil, errors.New("create index statement must specify an index name")
	}
	if stmt.TableName == "" {
		return nil, errors.New("create index statement must specify a table name")
	}
	if stmt.ColumnName == "" {
		return nil, errors.New("create index statement must specify a column name")
	}

	return &CreateIndexNode{
		IndexName:  stmt.IndexName,
		TableName:  stmt.TableName,
		ColumnName: stmt.ColumnName,
		IsUnique:   stmt.IsUnique,
	}, nil
}

func (p *Planner) planDropIndex(stmt *ast.DropIndexStatement) (PlanNode, error) {
	if stmt.IndexName == "" {
		return nil, errors.New("drop index statement must specify an index name")
	}
	if stmt.TableName == "" {
		return nil, errors.New("drop index statement must specify a table name")
	}

	return &DropIndexNode{
		IndexName: stmt.IndexName,
		TableName: stmt.TableName,
		IfExists:  stmt.IfExists,
	}, nil
}

func (p *Planner) planShowIndexes(stmt *ast.ShowIndexesStatement) (PlanNode, error) {
	return &ShowIndexesNode{
		TableName: stmt.TableName,
	}, nil
}
