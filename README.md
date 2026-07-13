![alt FlamingoDB](https://raw.githubusercontent.com/TaqsBlaze/FlamingoDB/refs/heads/main/banner/flamingodb.png)
# FlamingoDB

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white&style=for-the-badge" alt="Go Version" />
  <img src="https://img.shields.io/badge/Build-Passing-4CAF50?style=for-the-badge&logo=github-actions&logoColor=white" alt="Build" />
  <img src="https://img.shields.io/badge/Tests-All%20Passing-4CAF50?style=for-the-badge&logo=go&logoColor=white" alt="Tests" />
  <img src="https://img.shields.io/badge/Phase-6%20%2F%2014-FF6B35?style=for-the-badge" alt="Phase" />
  <img src="https://img.shields.io/badge/License-MIT-blue?style=for-the-badge" alt="License" />
</p>

<p align="center">
  <strong>A modern scientific database engine built for numerical computing, multidimensional arrays, vectors, geospatial data, and large scale analytical workloads.</strong>
</p>

---

## 💡 Why FlamingoDB?

Traditional databases treat scientific data as afterthoughts heavy BLOBs, generic arrays, and expensive serialization cycles that kill performance.

**FlamingoDB is different.** It treats scientific datatypes as **first class citizens**:

- 🧮 Native `VECTOR`, `MATRIX`, `TENSOR` types *(Phase 9)*
- 🌍 Native `POINT`, `POLYGON`, `MULTIPOLYGON` geospatial types *(Phase 11)*
- ⚡ SIMD accelerated numerical operations *(Phase 12)*
- 🧬 Built for climate modelling, astronomy, physics simulations, and ML datasets


---

## 📊 Development Progress

### Overall Completion: `43%`

```
[████████░░░░░░░░░░░░] 43%
```

### Phase Status

| # | Phase | Status | Coverage |
|---|-------|:------:|:--------:|
| 1 | **Foundation** — Pager, Disk IO, Serialization, Heap Table | ✅ Done | `100%` |
| 2 | **Storage Engine** — Row Format, Schema, Catalog, Table Manager | ✅ Done | `100%` |
| 3 | **SQL Lexer** — Keywords, Operators, Identifiers | ✅ Done | `100%` |
| 4 | **SQL Parser** — AST Construction, All DML/DDL Statements | ✅ Done | `100%` |
| 5 | **Planner** — AST → Logical Plan (Scan, Filter, Project, Insert…) | ✅ Done | `100%` |
| 6 | **Executor** — Physical Execution against Storage Engine | ✅ Done | `100%` |
| 7 | **Indexes** — B+ Tree Lookup & Range Scans | ⏳ Next | `0%` |
| 8 | **Transactions** — WAL, Commit, Rollback, Crash Recovery | ⏳ Planned | `0%` |
| 9 | **Scientific Types** — VECTOR, MATRIX, TENSOR, COMPLEX | ⏳ Planned | `0%` |
| 10 | **Scientific Functions** — SIN, COS, DOT, CROSS, SQRT… | ⏳ Planned | `0%` |
| 11 | **Geospatial** — POINT, POLYGON, DISTANCE, INTERSECTS… | ⏳ Planned | `0%` |
| 12 | **Optimization** — Query Planner, SIMD, Parallel Execution | ⏳ Planned | `0%` |
| 13 | **Networking** — TCP Server, Connection Pool, Auth | ⏳ Planned | `0%` |
| 14 | **Python SDK** — Native `import flamingodb` | ⏳ Planned | `0%` |

---

## 🏗️ Architecture

FlamingoDB follows a strict **Clean Architecture** each layer communicates only with adjacent layers. No shortcuts.

```
          SQL Query
              │
         ┌────▼────┐
         │  Lexer  │   Tokenises raw SQL strings
         └────┬────┘
              │
         ┌────▼────┐
         │ Parser  │   Builds a typed AST
         └────┬────┘
              │
         ┌────▼────┐
         │ Planner │   Converts AST → Logical Plan nodes
         └────┬────┘
              │
         ┌────▼────┐
         │Executor │   Physically executes plan nodes
         └────┬────┘
              │
     ┌────────▼────────┐
     │  Table Manager  │   Schema aware DML/DDL coordination
     └────────┬────────┘
              │
     ┌────────▼────────┐
     │  Catalog / Page │   Metadata, Serialization & Page abstraction
     └────────┬────────┘
              │
     ┌────────▼────────┐
     │  Pager / Disk   │   Buffer pool + fixed-size page IO (8KB pages)
     └────────┬────────┘
              │
         Database File
```

---

## 🚀 Quick Start

```go
package main

import (
    "fmt"
    "flamingodb/internal/executor"
    "flamingodb/internal/parser/lexer"
    "flamingodb/internal/parser/parser"
    "flamingodb/internal/planner"
    "flamingodb/internal/storage/catalog"
    "flamingodb/internal/storage/disk"
    "flamingodb/internal/storage/pager"
)

func run(sql string, exec *executor.Executor) {
    l := lexer.New(sql)
    p := parser.New(l)
    prog := p.ParseProgram()

    pl := planner.New()
    node, _ := pl.Plan(prog.Statements[0])

    result, _ := exec.Execute(node)
    fmt.Printf("%d row(s) returned\n", len(result.Rows))
}

func main() {
    dm, _ := disk.NewDiskManager("science.db", 4096)
    p, _  := pager.New(dm, 4096)
    tm, _ := catalog.NewTableManager(p)
    exec  := executor.New(tm)

    run("CREATE TABLE stars (id INT, name VARCHAR, magnitude FLOAT);", exec)
    run("INSERT INTO stars VALUES (1, 'Sirius', -1.46);",              exec)
    run("INSERT INTO stars VALUES (2, 'Canopus', -0.74);",             exec)
    run("SELECT * FROM stars WHERE magnitude < 0;",                     exec) // → 2 rows
}
```

---

## 🗂️ Project Structure

```
flamingodb/
├── cmd/
│   ├── flamingodbd/          # Database server daemon
│   └── flamingo/             # CLI client
├── internal/
│   ├── parser/
│   │   ├── lexer/            # SQL tokeniser
│   │   ├── ast/              # AST node definitions
│   │   └── parser/           # Pratt parser → AST
│   ├── planner/              # AST → Logical plan
│   ├── executor/             # Physical plan execution
│   ├── storage/
│   │   ├── page/             # Fixed-size page abstraction
│   │   ├── disk/             # Thread-safe disk IO
│   │   ├── pager/            # Buffer pool manager
│   │   ├── encoding/         # Little-endian binary encoding
│   │   ├── record/           # Row format + schema serialization
│   │   └── catalog/          # Metadata catalog + TableManager
│   ├── index/btree/          # B+ Tree (Phase 7)
│   ├── wal/                  # Write-ahead log (Phase 8)
│   └── transaction/          # Transaction manager (Phase 8)
├── pkg/
│   ├── logger/               # Leveled structured logger
│   └── config/               # Global configuration
├── sdk/                      # Python SDK (Phase 14)
├── docs/                     # Shared agent memory & design docs
└── tests/                    # End-to-end integration tests
```

---

## 🧪 Tests

Every package requires unit tests. Every bug fix requires a regression test.

```bash
go test ./...
```

**Current Results:**
```
ok   flamingodb/internal/executor          0.015s
ok   flamingodb/internal/parser/lexer      0.009s
ok   flamingodb/internal/parser/parser     0.010s
ok   flamingodb/internal/planner           0.011s
ok   flamingodb/internal/storage/catalog   0.011s
ok   flamingodb/internal/storage/disk      0.001s
ok   flamingodb/internal/storage/encoding  0.001s
ok   flamingodb/internal/storage/pager     0.002s
ok   flamingodb/internal/storage/record    0.002s
ok   flamingodb/internal/storage/table     0.009s
ok   flamingodb/tests                      0.011s
```

---

## 📄 License

FlamingoDB is licensed under the **MIT License** — see [`LICENSE`](./LICENSE) for details.
