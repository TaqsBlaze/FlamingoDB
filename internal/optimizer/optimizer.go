package optimizer

import (
	"fmt"
	"strings"

	"flamingodb/internal/parser/ast"
	"flamingodb/internal/planner"
	"flamingodb/internal/storage/catalog"
	"flamingodb/internal/storage/record"
)

// Optimize performs logical optimizations (such as index scan selection, filter pushdown,
// and projection pruning/merging) on the input query plan.
func Optimize(plan planner.PlanNode, tm *catalog.TableManager) (planner.PlanNode, error) {
	if plan == nil {
		return nil, nil
	}

	// 1. Optimize children nodes recursively
	switch n := plan.(type) {
	case *planner.ProjectNode:
		optChild, err := Optimize(n.Child, tm)
		if err != nil {
			return nil, err
		}
		n.Child = optChild
	case *planner.FilterNode:
		optChild, err := Optimize(n.Child, tm)
		if err != nil {
			return nil, err
		}
		n.Child = optChild
	case *planner.UpdateNode:
		optChild, err := Optimize(n.Child, tm)
		if err != nil {
			return nil, err
		}
		n.Child = optChild
	case *planner.DeleteNode:
		optChild, err := Optimize(n.Child, tm)
		if err != nil {
			return nil, err
		}
		n.Child = optChild
	}

	// 2. Apply rules on the current node
	return applyRules(plan, tm)
}

func applyRules(node planner.PlanNode, tm *catalog.TableManager) (planner.PlanNode, error) {
	// Rule 1: Filter Pushdown
	// Filter(cond, Project(fields, child)) -> Project(fields, Filter(cond, child))
	if filter, ok := node.(*planner.FilterNode); ok {
		if project, ok := filter.Child.(*planner.ProjectNode); ok {
			newFilter := &planner.FilterNode{
				Condition: filter.Condition,
				Child:     project.Child,
			}
			newProject := &planner.ProjectNode{
				Fields: project.Fields,
				Child:  newFilter,
			}
			return Optimize(newProject, tm)
		}
	}

	// Rule 2: Projection Pruning / Merging
	if project1, ok := node.(*planner.ProjectNode); ok {
		// Project(fields1, Project(fields2, child)) -> Project(fields1, child)
		if project2, ok := project1.Child.(*planner.ProjectNode); ok {
			newProject := &planner.ProjectNode{
				Fields: project1.Fields,
				Child:  project2.Child,
			}
			return Optimize(newProject, tm)
		}

		// Wildcard projection pruning: Project([*], child) -> child
		if len(project1.Fields) == 1 {
			if ident, ok := project1.Fields[0].(*ast.Identifier); ok && ident.Value == "*" {
				return project1.Child, nil
			}
		}
	}

	// Rule 3: B+ Tree Index Scan Selection
	// Filter(cond, Scan(table)) -> Filter(cond, IndexScan(table, col, low, high))
	if filter, ok := node.(*planner.FilterNode); ok {
		if scan, ok := filter.Child.(*planner.ScanNode); ok {
			optScan, err := tryIndexScan(scan, filter.Condition, tm)
			if err != nil {
				return nil, err
			}
			if optScan != nil {
				filter.Child = optScan
				return filter, nil
			}
		}
	}

	return node, nil
}

func tryIndexScan(scan *planner.ScanNode, condition ast.Expression, tm *catalog.TableManager) (planner.PlanNode, error) {
	if tm == nil {
		return nil, nil
	}
	indexes, err := tm.GetIndexes(scan.Table)
	if err != nil {
		if strings.Contains(err.Error(), "table not found") {
			return nil, nil
		}
		return nil, err
	}
	if len(indexes) == 0 {
		return nil, nil
	}

	// Match condition to col OP val
	colName, op, valExpr, isColLeft, found := matchIndexedCondition(condition, indexes)
	if !found {
		return nil, nil
	}

	idxMeta := indexes[colName]
	schema, err := tm.GetSchema(scan.Table)
	if err != nil {
		return nil, err
	}

	var targetType record.TypeID
	for _, col := range schema.Columns {
		if strings.EqualFold(col.Name, colName) {
			targetType = col.Type
			break
		}
	}

	litVal, err := evaluateLiteral(valExpr)
	if err != nil {
		return nil, err
	}

	castedVal, err := castValue(litVal, targetType)
	if err != nil {
		return nil, err
	}

	var lowVal, highVal *record.Value
	switch op {
	case "=", "==":
		lowVal = &castedVal
		highVal = &castedVal
	case "<", "<=":
		if isColLeft {
			highVal = &castedVal
		} else {
			lowVal = &castedVal
		}
	case ">", ">=":
		if isColLeft {
			lowVal = &castedVal
		} else {
			highVal = &castedVal
		}
	}

	return &planner.IndexScanNode{
		Table:       scan.Table,
		ColumnName:  colName,
		IndexRootID: uint32(idxMeta.RootPageID),
		KeyType:     uint8(idxMeta.KeyType),
		LowVal:      lowVal,
		HighVal:     highVal,
	}, nil
}

func matchIndexedCondition(expr ast.Expression, indexes map[string]*catalog.IndexMetadata) (colName string, op string, val ast.Expression, isColLeft bool, found bool) {
	infix, ok := expr.(*ast.InfixExpression)
	if !ok {
		return "", "", nil, false, false
	}

	switch infix.Operator {
	case "=", "==", "<", "<=", ">", ">=":
		if ident, ok := infix.Left.(*ast.Identifier); ok {
			for name := range indexes {
				if strings.EqualFold(name, ident.Value) {
					if isLiteral(infix.Right) {
						return name, infix.Operator, infix.Right, true, true
					}
				}
			}
		}
		if ident, ok := infix.Right.(*ast.Identifier); ok {
			for name := range indexes {
				if strings.EqualFold(name, ident.Value) {
					if isLiteral(infix.Left) {
						return name, infix.Operator, infix.Left, false, true
					}
				}
			}
		}
	}
	return "", "", nil, false, false
}

func isLiteral(expr ast.Expression) bool {
	switch expr.(type) {
	case *ast.IntegerLiteral, *ast.FloatLiteral, *ast.StringLiteral:
		return true
	case *ast.PrefixExpression:
		pfx := expr.(*ast.PrefixExpression)
		return (pfx.Operator == "-" || pfx.Operator == "+") && isLiteral(pfx.Right)
	}
	return false
}

func evaluateLiteral(expr ast.Expression) (record.Value, error) {
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return record.Value{Type: record.Integer, Int: int32(e.Value)}, nil
	case *ast.FloatLiteral:
		return record.Value{Type: record.Float, Flt: e.Value}, nil
	case *ast.StringLiteral:
		return record.Value{Type: record.Varchar, Str: e.Value}, nil
	case *ast.PrefixExpression:
		rightVal, err := evaluateLiteral(e.Right)
		if err != nil {
			return record.Value{}, err
		}
		if e.Operator == "-" {
			switch rightVal.Type {
			case record.Integer:
				rightVal.Int = -rightVal.Int
				return rightVal, nil
			case record.Float:
				rightVal.Flt = -rightVal.Flt
				return rightVal, nil
			}
		} else if e.Operator == "+" {
			return rightVal, nil
		}
	}
	return record.Value{}, fmt.Errorf("expression is not a simple literal: %T", expr)
}

func castValue(val record.Value, targetType record.TypeID) (record.Value, error) {
	if val.Type == targetType {
		return val, nil
	}
	if val.Type == record.Integer && targetType == record.Float {
		return record.Value{Type: record.Float, Flt: float64(val.Int)}, nil
	}
	return record.Value{}, fmt.Errorf("cannot cast type %v to %v", val.Type, targetType)
}
