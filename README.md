![alt FlamingoDB](https://raw.githubusercontent.com/TaqsBlaze/FlamingoDB/refs/heads/main/banner/flamingodb.png)
# FlamingoDB

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white&style=for-the-badge" alt="Go Version" />
  <img src="https://img.shields.io/badge/Build-Passing-4CAF50?style=for-the-badge&logo=github-actions&logoColor=white" alt="Build" />
  <img src="https://img.shields.io/badge/Tests-All%20Passing-4CAF50?style=for-the-badge&logo=go&logoColor=white" alt="Tests" />
  <img src="https://img.shields.io/badge/Phase-8%20%2F%2014-FF6B35?style=for-the-badge" alt="Phase" />
  <img src="https://img.shields.io/badge/License-MIT-blue?style=for-the-badge" alt="License" />
  <img src="https://img.shields.io/badge/Domain-Scientific%20Database-8A2BE2?style=for-the-badge" alt="Domain" />
</p>

<p align="center">
  <strong>An open source scientific database system engineered for high performance data storage, computational research, and large scale scientific datasets.</strong>
</p>

<p align="center">
  <a href="#why-flamingodb">Why FlamingoDB</a> ·
  <a href="#-development-progress">Progress</a> ·
  <a href="#-architecture">Architecture</a> ·
  <a href="#-quick-start">Quick Start</a> ·
  <a href="#-use-cases">Use Cases</a> ·
  <a href="#-tests">Tests</a> ·
  <a href="#-contributing">Contributing</a>
</p>

---

## Why FlamingoDB?

Modern **scientific data management** demands more than what traditional relational databases were built to deliver. Systems designed for business records were never meant to handle:

- **Tensors and matrices** from deep learning training runs
- **Geospatial coordinates** from satellite or climate datasets
- **High-frequency time-series** from bioinformatics workflows
- **Multidimensional arrays** from physics simulations and numerical methods

Traditional databases treat these as afterthoughts  heavy BLOBs, generic binary fields, and expensive serialization cycles that destroy performance at scale.

**FlamingoDB is different.** It is purpose built as a **scientific database system** where numerical and multidimensional types are **first class citizens** in the storage engine itself.


---

## 🎯 Built For

FlamingoDB is **open source scientific software** designed specifically for **data intensive science** and **reproducible research** workflows:

| Domain | Example Workloads |
|--------|-------------------|
| 🧬 **Bioinformatics** | Sequence alignment datasets, variant call files, genomic arrays |
| 🌍 **Climate Science** | Gridded temperature models, precipitation tensors, geospatial polygons |
| 🔭 **Astronomy** | Star catalogues, spectroscopy arrays, photometric survey data |
| ⚛️ **Physics** | Particle collision datasets, finite element matrices, simulation outputs |
| 🤖 **Machine Learning** | Embedding vectors, feature matrices, model parameter storage |
| 🧫 **Life Sciences** | Multi omics data pipelines, proteomics arrays, biomarker research data infrastructure |

FlamingoDB is designed from the ground up to power **research data infrastructure** for organisations that need **high performance data storage** for **large scale scientific datasets**  without the overhead of repurposing a general purpose RDBMS.

---

## 📊 Development Progress

### Overall Completion: `57%`

```
[███████████░░░░░░░░░] 57%
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
| 7 | **Indexes** — B+ Tree Lookup & Range Scans | ✅ Done | `100%` |
| 8 | **Transactions** — WAL, Commit, Rollback, Crash Recovery | ✅ Done | `100%` |
| 9 | **Scientific Types** — `VECTOR`, `MATRIX`, `TENSOR`, `COMPLEX` | ⏳ Next | `0%` |
| 10 | **Scientific Functions** — `SIN`, `COS`, `DOT`, `CROSS`, `NORM`… | ⏳ Planned | `0%` |
| 11 | **Geospatial** — `POINT`, `POLYGON`, `DISTANCE`, `INTERSECTS`… | ⏳ Planned | `0%` |
| 12 | **Optimization** — Query Planner, SIMD, Parallel Execution | ⏳ Planned | `0%` |
| 13 | **Networking** — TCP Server, Connection Pool, Auth | ⏳ Planned | `0%` |
| 14 | **Python SDK** — Native `import flamingodb` | ⏳ Planned | `0%` |

---

## 🏗️ Architecture

FlamingoDB follows a strict **Clean Architecture** each layer communicates only with adjacent layers. Engineered for **computational research** workloads where correctness and predictability are non-negotiable.

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
     │  Table Manager  │   Schema-aware DML/DDL coordination
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

The following demonstrates a full SQL pipeline across the **scientific data management** stack from raw SQL string to persisted records and filtered results.

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

func run(sql string, exec *executor.Executor) *executor.Result {
    l := lexer.New(sql)
    p := parser.New(l)
    prog := p.ParseProgram()

    pl := planner.New()
    node, _ := pl.Plan(prog.Statements[0])

    result, _ := exec.Execute(node)
    return result
}

func main() {
    // Bootstrap the research data infrastructure
    dm, _ := disk.NewDiskManager("science.db", 4096)
    p, _  := pager.New(dm, 4096)
    tm, _ := catalog.NewTableManager(p)
    exec  := executor.New(tm)

    // Define a schema for a large-scale scientific dataset
    run("CREATE TABLE stars (id INT, name VARCHAR, magnitude FLOAT);", exec)

    // Insert records — supports negative literals for scientific values
    run("INSERT INTO stars VALUES (1, 'Sirius',   -1.46);", exec)
    run("INSERT INTO stars VALUES (2, 'Canopus',  -0.74);", exec)
    run("INSERT INTO stars VALUES (3, 'Rigel',     0.13);", exec)

    // Query with filter — SQL-native WHERE clause execution
    result := run("SELECT * FROM stars WHERE magnitude < 0;", exec)
    fmt.Printf("%d bright stars found\n", len(result.Rows)) // → 2 bright stars found
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
│   │   ├── page/             # Fixed-size page abstraction (8KB)
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

**Reproducible research** demands reproducible software. Every package requires unit tests; every bug fix requires a regression test. FlamingoDB enforces this as a hard rule.

```bash
go test ./...
```

**Current Results — All Passing:**
```
ok   flamingodb/internal/executor          0.025s
ok   flamingodb/internal/index/btree       0.232s
ok   flamingodb/internal/parser/lexer      0.029s
ok   flamingodb/internal/parser/parser     0.038s
ok   flamingodb/internal/planner           0.022s
ok   flamingodb/internal/storage/catalog   0.027s
ok   flamingodb/internal/storage/disk      0.038s
ok   flamingodb/internal/storage/encoding  0.040s
ok   flamingodb/internal/storage/pager     0.013s
ok   flamingodb/internal/storage/record    0.011s
ok   flamingodb/internal/storage/table     0.010s
ok   flamingodb/tests                      0.103s
```

---


## 🔖 Keywords

`scientific database system` · `scientific data management` · `research data infrastructure` · `high performance data storage` · `computational research` · `large-scale scientific datasets` · `bioinformatics workflows` · `data-intensive science` · `reproducible research` · `open source scientific software` · `database engine` · `vector database` · `matrix storage` · `geospatial database` · `Go database`

---

## 📄 License

FlamingoDB is licensed under the **MIT License** — see [`LICENSE`](./LICENSE) for details.
