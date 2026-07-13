# Project Index

This document outlines the project directory structure, source files, and a description of each file's function and location within FlamingoDB.

## Directory Structure

```
flamingodb/
├── cmd/                          # Database execution binaries
│   ├── flamingo/                 # CLI client entrypoint
│   └── flamingodbd/              # Database server daemon entrypoint
├── docs/                         # Architecture, decisions, and system documentation
├── internal/                     # Core database engine internals (private)
│   ├── audit/                    # Audit logs and verification modules
│   ├── datatypes/                # Extended scientific data types
│   ├── executor/                 # Physical execution engine
│   ├── functions/                # Scientific and mathematical functions
│   ├── geo/                      # Geospatial index and data structures
│   ├── index/                    # Database indexing
│   │   └── btree/                # B+ Tree index implementation
│   ├── logging/                  # Engine trace and diagnostic logs
│   ├── network/                  # Network server protocols (TCP/HTTP)
│   ├── optimizer/                # Logical query plan optimizer
│   ├── parser/                   # Lexer, Parser, and AST
│   │   ├── ast/                  # Abstract Syntax Tree definitions
│   │   ├── lexer/                # SQL Lexer / Tokenizer
│   │   └── parser/               # SQL Pratt Parser
│   ├── planner/                  # SQL AST to Logical Plan translator
│   ├── storage/                  # Disk, pager, caching, and table management
│   │   ├── catalog/              # Schema catalog and TableManager
│   │   ├── disk/                 # Raw disk read/write manager
│   │   ├── encoding/             # Binary serialization formats
│   │   ├── page/                 # Fixed-size page layouts
│   │   ├── pager/                # Buffer pool caching manager
│   │   ├── record/               # In-memory record representation
│   │   └── table/                # Linked page heap table storage
│   ├── transaction/              # Transaction control and state management
│   └── wal/                      # Write-Ahead Logging
├── pkg/                          # Shared library packages (public)
│   ├── config/                   # Global engine config definitions
│   └── logger/                   # Leveled structured logger
├── sdk/                          # Client libraries (e.g. Python SDK)
└── tests/                        # Integration and robustness tests
```

---

## File Catalog & Descriptions

### Executable Command Packages (`cmd/`)
*   **[cmd/flamingo/](file:///home/blaze/Projects/FlamingoDB/cmd/flamingo)**: Empty package designated for the command-line CLI client interface.
*   **[cmd/flamingodbd/](file:///home/blaze/Projects/FlamingoDB/cmd/flamingodbd)**: Empty package designated for the database server engine daemon.

### Scientific Types Package (`internal/datatypes/`)
*   **[internal/datatypes/datatypes.go](file:///home/blaze/Projects/FlamingoDB/internal/datatypes/datatypes.go)**: Defines VECTOR, MATRIX, TENSOR, and COMPLEX datatypes and mathematical operations.
*   **[internal/datatypes/datatypes_test.go](file:///home/blaze/Projects/FlamingoDB/internal/datatypes/datatypes_test.go)**: Unit tests for the scientific datatypes and their operations.

### SQL Parser Package (`internal/parser/`)
*   **[internal/parser/ast/ast.go](file:///home/blaze/Projects/FlamingoDB/internal/parser/ast/ast.go)**: Defines the AST nodes for statements (e.g. `CreateTableStatement`, `InsertStatement`, `SelectStatement`) and expressions (e.g. `Identifier`, `IntegerLiteral`, `FloatLiteral`, `PrefixExpression`).
*   **[internal/parser/lexer/token.go](file:///home/blaze/Projects/FlamingoDB/internal/parser/lexer/token.go)**: Registers SQL token types and keywords (`CREATE`, `TABLE`, `SELECT`, `WHERE`, etc.).
*   **[internal/parser/lexer/lexer.go](file:///home/blaze/Projects/FlamingoDB/internal/parser/lexer/lexer.go)**: Tokenizes raw SQL query strings into a stream of structured tokens.
*   **[internal/parser/lexer/lexer_test.go](file:///home/blaze/Projects/FlamingoDB/internal/parser/lexer/lexer_test.go)**: Unit tests for SQL string tokenization.
*   **[internal/parser/parser/parser.go](file:///home/blaze/Projects/FlamingoDB/internal/parser/parser/parser.go)**: Implements a Pratt parser to convert token streams into an Abstract Syntax Tree (AST), supporting operator precedence and expression folding.
*   **[internal/parser/parser/parser_test.go](file:///home/blaze/Projects/FlamingoDB/internal/parser/parser/parser_test.go)**: Unit tests for AST generation and parsing edge cases.

### Query Planner Package (`internal/planner/`)
*   **[internal/planner/planner.go](file:///home/blaze/Projects/FlamingoDB/internal/planner/planner.go)**: Defines logical plan nodes (`CreateTableNode`, `InsertNode`, `ScanNode`, `FilterNode`, `ProjectNode`) and translates parsed AST statements into logical plans.
*   **[internal/planner/planner_test.go](file:///home/blaze/Projects/FlamingoDB/internal/planner/planner_test.go)**: Unit tests for logical plan generation and SQL mapping.

### Scientific Functions Package (`internal/functions/`)
*   **[internal/functions/functions.go](file:///home/blaze/.gemini/antigravity-cli/brain/c54fcb7a-cda6-4e84-8836-4953f61b3a38/.system_generated/worktrees/subagent-Phase-10-Functions-Developer-phase10-developer-eb02d8b4/internal/functions/functions.go)**: Implements native scientific and mathematical functions (SIN, COS, TAN, ASIN, ACOS, ATAN, EXP, LOG, SQRT, ABS, POW, DOT, CROSS, NORM) and registers them in a global functions registry.
*   **[internal/functions/functions_test.go](file:///home/blaze/.gemini/antigravity-cli/brain/c54fcb7a-cda6-4e84-8836-4953f61b3a38/.system_generated/worktrees/subagent-Phase-10-Functions-Developer-phase10-developer-eb02d8b4/internal/functions/functions_test.go)**: Unit tests for mathematical and vector function logic.

### Physical Execution Package (`internal/executor/`)
*   **[internal/executor/executor.go](file:///home/blaze/Projects/FlamingoDB/internal/executor/executor.go)**: Implements physical operators (Scan, Filter, Project, Insert, Create Table) and runs logical plans against the storage engine. Includes pre-checks for semantic validation.
*   **[internal/executor/executor_test.go](file:///home/blaze/Projects/FlamingoDB/internal/executor/executor_test.go)**: Unit tests for query plans and execution results.

### Storage Engine Package (`internal/storage/`)
*   **[internal/storage/page/page.go](file:///home/blaze/Projects/FlamingoDB/internal/storage/page/page.go)**: Defines the fixed-size `Page` abstraction representing a block of memory mapped to a block on disk.
*   **[internal/storage/disk/disk_manager.go](file:///home/blaze/Projects/FlamingoDB/internal/storage/disk/disk_manager.go)**: Directs OS file reads and writes for database pages with concurrency safety and file syncing.
*   **[internal/storage/disk/disk_manager_test.go](file:///home/blaze/Projects/FlamingoDB/internal/storage/disk/disk_manager_test.go)**: Unit tests for thread-safe file IO operations.
*   **[internal/storage/pager/pager.go](file:///home/blaze/Projects/FlamingoDB/internal/storage/pager/pager.go)**: Implements a buffer pool cache manager that controls cache retrieval, page allocation, and flushing.
*   **[internal/storage/pager/pager_test.go](file:///home/blaze/Projects/FlamingoDB/internal/storage/pager/pager_test.go)**: Unit tests for buffer caching and allocation.
*   **[internal/storage/encoding/binary.go](file:///home/blaze/Projects/FlamingoDB/internal/storage/encoding/binary.go)**: Contains little-endian encoding helpers for numbers and length-prefixed strings.
*   **[internal/storage/encoding/binary_test.go](file:///home/blaze/Projects/FlamingoDB/internal/storage/encoding/binary_test.go)**: Unit tests for serialization primitives.
*   **[internal/storage/record/record.go](file:///home/blaze/Projects/FlamingoDB/internal/storage/record/record.go)**: Defines schemas, columns, values, and record structures. Handles binary layout serialization and deserialization.
*   **[internal/storage/record/record_test.go](file:///home/blaze/Projects/FlamingoDB/internal/storage/record/record_test.go)**: Unit tests for record binary conversion.
*   **[internal/storage/catalog/catalog.go](file:///home/blaze/Projects/FlamingoDB/internal/storage/catalog/catalog.go)**: Manages table metadata and schemas, persisting them to Page 0 of the database file.
*   **[internal/storage/catalog/catalog_test.go](file:///home/blaze/Projects/FlamingoDB/internal/storage/catalog/catalog_test.go)**: Unit tests for catalog metadata serialization.
*   **[internal/storage/catalog/table_manager.go](file:///home/blaze/Projects/FlamingoDB/internal/storage/catalog/table_manager.go)**: Provides the coordination entrypoint for DDL and DML operations.
*   **[internal/storage/table/table.go](file:///home/blaze/Projects/FlamingoDB/internal/storage/table/table.go)**: Manages physical page linked lists for heap tables. Implements record insertions and page-crossing reads.
*   **[internal/storage/table/table_test.go](file:///home/blaze/Projects/FlamingoDB/internal/storage/table/table_test.go)**: Unit tests for heap table storage.

### Indexing Package (`internal/index/`)
*   **[internal/index/btree/btree.go](file:///home/blaze/Projects/FlamingoDB/internal/index/btree/btree.go)**: Implements a page-backed B+ Tree index with support for `INT`, `FLOAT`, and `VARCHAR` keys. Handles point lookups, range scans, and multi-level page splits.
*   **[internal/index/btree/btree_test.go](file:///home/blaze/Projects/FlamingoDB/internal/index/btree/btree_test.go)**: Unit tests for B+ Tree splits, scans, and persistence.

### Write-Ahead Log Package (`internal/wal/`)
*   **[internal/wal/wal.go](file:///home/blaze/Projects/FlamingoDB/internal/wal/wal.go)**: Implements Write-Ahead Logging (WAL) record representation, checksumming (CRC32), log appending, and reading.

### Transaction Package (`internal/transaction/`)
*   **[internal/transaction/transaction.go](file:///home/blaze/Projects/FlamingoDB/internal/transaction/transaction.go)**: Defines the transaction data structure and lifecycles (Active, Committed, Aborted) with a private dirty page cache.
*   **[internal/transaction/manager.go](file:///home/blaze/Projects/FlamingoDB/internal/transaction/manager.go)**: Manages starting, committing, aborting transactions, global locking for isolation, and boot-time crash recovery.

### Integration Tests (`tests/`)
*   **[tests/integration_test.go](file:///home/blaze/Projects/FlamingoDB/tests/integration_test.go)**: End-to-end integration test validating a basic table creation, multi-row insertion, and data retrieval pipeline.
*   **[tests/robustness_test.go](file:///home/blaze/Projects/FlamingoDB/tests/robustness_test.go)**: Comprehensive integration test suite verifying edge cases: case-insensitivity, negative/zero values, physical size limits, multi-page heap storage, comparisons, projection ordering, cold restart persistence, index node splits, and semantic error states.
*   **[tests/functions_integration_test.go](file:///home/blaze/.gemini/antigravity-cli/brain/c54fcb7a-cda6-4e84-8836-4953f61b3a38/.system_generated/worktrees/subagent-Phase-10-Functions-Developer-phase10-developer-eb02d8b4/tests/functions_integration_test.go)**: Integration test verifying end-to-end execution of mathematical and vector functions in SQL SELECT queries and WHERE filtering conditions.
