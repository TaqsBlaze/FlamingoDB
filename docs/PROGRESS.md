# PROGRESS

## 2026-07-13
Completed:
- Project structure initialized (Go module, directories).
- Shared memory files created.
- Basic Logger implemented (`pkg/logger`).
- Basic Config implemented (`pkg/config`).
- Page abstraction created (`internal/storage/page`).
- Logging and Audit details separated into `Logging-Audit.md`.
- Lexer implementation (`internal/parser/lexer`) - Phase 3.
- Initial AST nodes (`internal/parser/ast`) - Phase 4.
- Database File implemented (`internal/storage/disk`).
- Pager implemented (`internal/storage/pager`).
- Binary Serialization implemented (`internal/storage/encoding`).
- Simple table storage implemented (`internal/storage/table`).
- SQL Parser implemented (`internal/parser/parser`) - Phase 4.
- Row Format & Schema serialization implemented (`internal/storage/record`) - Phase 2.
- Catalog & TableManager metadata persistence implemented (`internal/storage/catalog`) - Phase 2.
- MIT LICENSE file created at root.
- SQL Planner implemented (`internal/planner`) - Phase 5.
- SQL Executor implemented (`internal/executor`) - Phase 6.
- Parser fixed: unary minus prefix expressions supported (negative floats/ints).
- AST extended with `PrefixExpression` node.
- B+ Tree Index implemented (`internal/index/btree`) - Phase 7.
  - Supports INT, FLOAT, VARCHAR key types.
  - Insert, point search, range scan, node splitting (leaf + internal).
  - Disk persistence via Pager.
  - 500-key split stress test passing.
- Robustness and Edge Case Integration Tests implemented (`tests/robustness_test.go`).
  - Added test coverage for keyword case-insensitivity, negative/zero numbers, record size limits, multi-page heap storage, all comparison operators, column projection order, database restarts (persistence validation), B+ Tree page splits, and SQL/storage error states.
- Fixed 3 critical bugs identified during robustness testing:
  - Fixed out-of-bounds panic in `Record.Serialize` for records larger than 1024 bytes by dynamically sizing the buffer.
  - Fixed heap table page link overwrite bug in `table.New` by traversing the page linked list to correctly resolve `lastPageID` for existing tables.
  - Fixed query engine validation bypass on empty tables in `Executor` by validating project columns and filter conditions against the schema before row iterations.

Next:
- Phase 8: WAL, Transactions (commit, rollback, crash recovery).
