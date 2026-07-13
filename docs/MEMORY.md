# MEMORY

## Current Architecture
- Clean Architecture + Modular Design.
- Every layer only communicates with adjacent layers.
- "Keep it Simple": explicit logic, small packages, no magic.
- Error handling: return descriptive errors, never panic.

## Current Interfaces
- `Logger` interface (in `pkg/logger`) supports leveled structured logging.
- `Config` struct (in `pkg/config`) defines DataDirectory, PageSize (8192), MaxPages.
- `Page` abstraction (in `internal/storage/page`) defines PageID and basic fixed-size memory blocks.
- `DiskManager` (in `internal/storage/disk`) handles page-level disk I/O.
- `Pager` (in `internal/storage/pager`) manages caching and allocation of pages.
- `Record` and `Schema` (in `internal/storage/record`) handles serialization/deserialization of typed table rows.
- `Catalog` and `TableMetadata` (in `internal/storage/catalog`) persists table schemas to Page 0 of the database.
- `TableManager` (in `internal/storage/catalog`) acts as the schema-aware entrypoint for DDL and DML operations.
- `Lexer` (in `internal/parser/lexer`) tokenizes raw SQL statements.
- `Parser` (in `internal/parser/parser`) processes SQL tokens into AST nodes (supporting SELECT, INSERT, UPDATE, DELETE, CREATE TABLE).
- `Planner` (in `internal/planner`) converts AST statement nodes into logical plan nodes (Scan, Filter, Project, Insert, Update, Delete, CreateTable).
- `Executor` (in `internal/executor`) physically executes logical plan nodes against the `TableManager`. Supports CREATE TABLE, INSERT, full Scan, Filter (WHERE), and column Projection (SELECT fields).
- `BTree` (in `internal/index/btree`) is a page-backed B+ Tree index supporting INT, FLOAT, and VARCHAR keys with point search, range scan, and automatic node splitting. Persisted via the Pager.
- `WAL` (in `internal/wal`) manages binary log records with CRC32 checksum protection for recovery durability.
- `TransactionManager` (in `internal/transaction`) manages transaction lifecycle (Begin, Commit, Rollback), thread-safe global locks, private dirty page caches (NO-STEAL/FORCE), and WAL crash recovery.

## Invariants
- `PageSize` is fixed at 8192 bytes by default.
- Page 0 of the database file is reserved for Catalog metadata.
- BTree node key sizes: INT=4B, FLOAT=8B, VARCHAR=256B (fixed-width for page layout).
- Every DDL/DML statement executed outside an explicit transaction runs in auto-commit mode.
- Database instantiation automatically executes WAL crash recovery (Redo phase).

## Active Work
- None. Phases 1–8 are complete. Next phase is Phase 9 (Scientific Types).
