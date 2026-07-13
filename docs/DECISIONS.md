# DECISIONS

## Decision: Logical Plan Structure and Representation

### Context
In Phase 5, the SQL Planner converts parsed AST statements into a logical plan structure before execution in Phase 6.

### Architecture Decision
1. We design a tree of logical plan nodes (`PlanNode`) representing relational operators:
   - `ScanNode`: Leaf node representing table scanning.
   - `FilterNode`: Unary node representing filter execution.
   - `ProjectNode`: Unary node representing column projection.
   - `UpdateNode`: Node wrapping the target table source (a sub-plan Scan/Filter) and key-value column set.
   - `DeleteNode`: Node wrapping the target table source (a sub-plan Scan/Filter).
   - `InsertNode`: Leaf node containing table name and literal expressions.
   - `CreateTableNode`: Leaf node containing table name and columns definitions.
2. We implement a helper method `ToSchema()` on `CreateTableNode` which maps SQL string-based column types to internal storage types (like `record.TypeID`).

### Reason
- Keep it Simple: A clean hierarchy of structures with child pointers maps well to the target relational query execution tree.
- Decouples the parser AST from the execution plan.

### Status
Accepted
