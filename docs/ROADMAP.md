# ROADMAP

## Current Status
- **Current Phase**: Phase 5 (Planner) Completed. Phase 6 (Executor) and Phase 7 (Indexes) in progress.
- **Estimated Completion Percentage**: 35% (5 of 14 Phases Completed).

## Phases & Milestones

### Phase 1: Foundation (100% Done)
- Project Structure, Config, Logger, Page abstraction, Pager.

### Phase 2: Storage Engine (100% Done)
- Row Format, Schema, Metadata, Catalog, Table Manager.

### Phase 3: SQL Lexer (100% Done)
- Tokenize SELECT, INSERT, CREATE TABLE, UPDATE, DELETE, etc.

### Phase 4: Parser (100% Done)
- Generate AST for SELECT, INSERT, UPDATE, DELETE, CREATE TABLE.

### Phase 5: Planner (100% Done)
- Convert AST to Logical Plan Node Tree.

### Phase 6: Executor (Active)
- Implement scans, filters, inserts, updates, deletes execution.

### Phase 7: Indexes (Planned)
- Implement B+ Tree indexes, CREATE INDEX, lookup, range scans.

### Phase 8: Transactions (Planned)
- WAL, Rollback, Commit, Recovery.

### Phase 9: Scientific Types (Planned)
- INT, FLOAT, COMPLEX, VECTOR, ARRAY, MATRIX, TENSOR, etc.

### Phase 10: Scientific Functions (Planned)
- Native mathematical, vector and matrix functions.

### Phase 11: Geospatial (Planned)
- POINT, POLYGON, AREA, DISTANCE, etc.

### Phase 12: Optimization (Planned)
- Query Optimizer, statistics, parallel execution.

### Phase 13: Networking (Planned)
- TCP/HTTP database server, connection pool, authentication.

### Phase 14: Python SDK (Planned)
- Native python SDK for queries, transactions, scientific arrays.
