![alt FlamingoDB](https://raw.githubusercontent.com/TaqsBlaze/FlamingoDB/refs/heads/main/banner/flamingodb.png)
# FlamingoDB

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white&style=for-the-badge" alt="Go Version" />
  <img src="https://img.shields.io/badge/Build-Passing-4CAF50?style=for-the-badge&logo=github-actions&logoColor=white" alt="Build" />
  <img src="https://img.shields.io/badge/Tests-All%20Passing-4CAF50?style=for-the-badge&logo=go&logoColor=white" alt="Tests" />
  <img src="https://img.shields.io/badge/Phase-13%20%2F%2014-FF6B35?style=for-the-badge" alt="Phase" />
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
- **High frequency time series** from bioinformatics workflows
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

### Overall Completion: `93%`

```
[██████████████████░░] 93%
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
| 9 | **Scientific Types** — `VECTOR`, `MATRIX`, `TENSOR`, `COMPLEX` | ✅ Done | `100%` |
| 10 | **Scientific Functions** — `SIN`, `COS`, `DOT`, `CROSS`, `NORM`… | ✅ Done | `100%` |
| 11 | **Geospatial** — `POINT`, `POLYGON`, `DISTANCE`, `INTERSECTS`… | ✅ Done | `100%` |
| 12 | **Optimization** — Query Optimizer, Index Scan Selection, Filter Pushdown | ✅ Done | `100%` |
| 13 | **Networking** — Stateful TCP & REST HTTP Servers, Connection Semaphores, Auth | ✅ Done | `100%` |
| 14 | **Python SDK** — Native `import flamingodb` | ⏳ Next | `0%` |

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
    "github.com/TaqsBlaze/FlamingoDB"
)

func main() {
    // Bootstrap the research data infrastructure and connect to the database.
    // If the database file does not exist, it is created automatically.
    db, err := flamingodb.Connect("science.db")
    if err != nil {
        panic(err)
    }
    defer db.Close()

    // Define a schema for a large-scale scientific dataset
    db.Run("CREATE TABLE stars (id INT, name VARCHAR, magnitude FLOAT);")

    // Insert records — supports negative literals for scientific values
    db.Run("INSERT INTO stars VALUES (1, 'Sirius',   -1.46);")
    db.Run("INSERT INTO stars VALUES (2, 'Canopus',  -0.74);")
    db.Run("INSERT INTO stars VALUES (3, 'Rigel',     0.13);")

    // Query with filter — SQL-native WHERE clause execution
    result, err := db.Run("SELECT * FROM stars WHERE magnitude < 0;")
    if err != nil {
        panic(err)
    }
    fmt.Printf("%d bright stars found\n", len(result.Rows)) // → 2 bright stars found
}
```

---

## 🧬 Scientific & Geospatial SQL Extensions

FlamingoDB provides first-class support for scientific datatypes, vector space mathematics, and geospatial geometry natively inside the database engine.

### 1. Scientific Data Types & Literals

Create tables using native multidimensional and complex types:
```sql
CREATE TABLE research_runs (
    run_id INT,
    embedding VECTOR,
    spin COMPLEX,
    flux_matrix MATRIX,
    location POINT
);
```

Insert scientific literals directly:
```sql
INSERT INTO research_runs VALUES (
    101, 
    [0.15, -0.92, 0.44], 
    2.5 - 4.0i, 
    [[1.0, 0.0], [0.0, 1.0]], 
    POINT(18.42 -33.92)
);
```

### 2. Scientific & Mathematical Functions

Evaluate trigonometric, exponential, and vector operations directly in your SELECT and WHERE clauses:

- **Trigonometric & Scalar Functions**: `SIN(x)`, `COS(x)`, `TAN(x)`, `ASIN(x)`, `ACOS(x)`, `ATAN(x)`, `EXP(x)`, `LOG(x)`, `LN(x)`, `SQRT(x)`, `ABS(x)`, `POW(base, exp)`.
- **Vector Operations**:
  - `DOT(vector, vector)`: Returns the scalar dot product (`Float`).
  - `CROSS(vector, vector)`: Returns the vector cross product (`Vector`).
  - `NORM(vector)`: Returns the L2 Euclidean norm (`Float`).

```sql
-- Calculate vector norms and dot products
SELECT run_id, NORM(embedding), DOT(embedding, [1.0, 0.0, 0.0])
FROM research_runs
WHERE NORM(embedding) > 0.5;
```

### 3. Geospatial Functions

Work with point and polygon datasets using Well-Known Text (WKT) parsing and geospatial relations:

- `DISTANCE(point, point)`: Calculates the Euclidean distance (`Float`).
- `AREA(polygon)`: Computes the area of a polygon (`Float`).
- `INTERSECTS(geometry, geometry)`: Checks if two geometries intersect (returns `1` for true, `0` for false).
- `ST_GEOMFROMTEXT(wkt_string)`: Explicitly parses WKT formats.

```sql
-- Query locations within a specific distance threshold
SELECT run_id, DISTANCE(location, POINT(0.0 0.0))
FROM research_runs
WHERE DISTANCE(location, POINT(0.0 0.0)) < 25.0;
```

### 4. Complete Go Code Sample

Here is a complete Go program illustrating how to bootstrap the `Engine`, insert scientific and geospatial literals, and run math and distance queries:

```go
package main

import (
    "fmt"
    "github.com/TaqsBlaze/FlamingoDB"
)

func main() {
    // Connect to FlamingoDB (creates science_dataset.db if it doesn't exist)
    db, err := flamingodb.Connect("science_dataset.db")
    if err != nil {
        panic(err)
    }
    defer db.Close()

    // 1. Create table with native scientific & geospatial columns
    _, err = db.Run(`
        CREATE TABLE research_runs (
            run_id INT,
            embedding VECTOR,
            spin COMPLEX,
            flux_matrix MATRIX,
            location POINT
        );
    `)
    if err != nil {
        panic(err)
    }

    // 2. Insert vector space, complex plane, and spatial coordinate data
    _, err = db.Run(`
        INSERT INTO research_runs VALUES (
            101, 
            [0.15, -0.92, 0.44], 
            2.5 - 4.0i, 
            [[1.0, 0.0], [0.0, 1.0]], 
            POINT(18.42 -33.92)
        );
    `)
    if err != nil {
        panic(err)
    }

    _, err = db.Run(`
        INSERT INTO research_runs VALUES (
            102, 
            [0.85, 0.12, -0.31], 
            0.0 + 1.5i, 
            [[0.5, 0.5], [-0.5, 0.5]], 
            POINT(0.05 0.10)
        );
    `)
    if err != nil {
        panic(err)
    }

    // 3. Query using vector norms and dot products
    fmt.Println("--- Vector Operations ---")
    vecResult, err := db.Run(`
        SELECT run_id, NORM(embedding), DOT(embedding, [1.0, 0.0, 0.0])
        FROM research_runs
        WHERE NORM(embedding) > 0.5;
    `)
    if err != nil {
        panic(err)
    }
    for _, row := range vecResult.Rows {
        fmt.Printf("Run ID: %d | Norm: %.4f | Dot Product: %.4f\n", 
            row.Values[0].Int, row.Values[1].Flt, row.Values[2].Flt)
    }

    // 4. Query using geospatial Euclidean distance
    fmt.Println("\n--- Geospatial Operations ---")
    geoResult, err := db.Run(`
        SELECT run_id, DISTANCE(location, POINT(0.0 0.0))
        FROM research_runs
        WHERE DISTANCE(location, POINT(0.0 0.0)) < 25.0;
    `)
    if err != nil {
        panic(err)
    }
    for _, row := range geoResult.Rows {
        fmt.Printf("Run ID: %d | Distance from Origin: %.4f\n", 
            row.Values[0].Int, row.Values[1].Flt)
    }
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

## 🌐 Networking & CLI Client

FlamingoDB provides a dual-protocol database daemon and an interactive command-line interface (CLI) to query the engine over the network.

### 1. Database Server Daemon (`flamingodbd`)

The database server daemon starts storage and transaction engines and listens for incoming connections on both TCP and HTTP.

#### Flags
- `-tcp`: TCP port/address to bind to (default: `:4080`).
- `-http`: HTTP port/address to bind to (default: `:8080`).
- `-user`: Username for client authentication (default: `admin`).
- `-pass`: Password for client authentication (default: `admin`).
- `-dir`: Directory where database and WAL logs are stored (default: `./data`).

#### Run the Daemon:
```bash
go run cmd/flamingodbd/main.go -tcp :4080 -http :8080 -user admin -pass password123 -dir ./data
```

---

### 2. Interactive CLI Client (`flamingo`)

The interactive CLI client connects to the daemon's TCP server and runs queries using a REPL interface.

#### Flags
- `-addr`: Address of the daemon (default: `127.0.0.1:4080`).
- `-user`: Authentication username (default: `admin`).
- `-pass`: Authentication password (default: `admin`).

#### Launch the CLI:
```bash
go run cmd/flamingo/main.go -addr 127.0.0.1:4080 -user admin -pass password123
```

#### Run queries inside CLI:
```sql
Connecting to FlamingoDB at 127.0.0.1:4080...
Connected and authenticated successfully.
Type your SQL query and press Enter. Type 'exit' or 'quit' to close.
flamingo> CREATE TABLE particles (id INT, mass FLOAT, spin COMPLEX);
table "particles" created
flamingo> INSERT INTO particles VALUES (1, 125.09, 0.0 + 0.0i);
1 row(s) affected
flamingo> SELECT * FROM particles;
| id | mass   | spin        |
+----+--------+-------------+
| 1  | 125.09 | (0 + 0i)    |
(1 rows)
flamingo> exit
Goodbye.
```

---

### 3. HTTP REST API Client

The REST API allows stateless clients to execute queries and manage transactions over standard HTTP. All queries require Basic Authentication matching the daemon credentials.

#### Run a Query
```bash
curl -u admin:password123 -X POST http://localhost:8080/query \
     -H "Content-Type: application/json" \
     -d '{"query": "SELECT * FROM stars;"}'
```

#### Multi-statement Transaction Control
1. **Begin**: Returns a unique transaction identifier (`tx_id`).
   ```bash
   curl -u admin:password123 -X POST http://localhost:8080/tx/begin
   # Response: {"success":true,"tx_id":"a83d71...","message":"transaction started"}
   ```
2. **Execute inside Transaction**: Pass the `tx_id` payload to lock operations within that transaction boundary.
   ```bash
   curl -u admin:password123 -X POST http://localhost:8080/query \
        -H "Content-Type: application/json" \
        -d '{"query": "INSERT INTO stars VALUES (4, '\''Vega'\'', 0.03);", "tx_id": "a83d71..."}'
   ```
3. **Commit / Rollback**: Commits or discards changes.
   ```bash
   curl -u admin:password123 -X POST http://localhost:8080/tx/commit \
        -H "Content-Type: application/json" \
        -d '{"tx_id": "a83d71..."}'
   ```
   *Note: Inactive HTTP transactions are automatically rolled back after 15 seconds of inactivity to prevent locking deadlocks.*

---

### 4. Web Administration Dashboard

FlamingoDB embeds a beautiful, rich web-based administration dashboard directly into the database server daemon. It provides an intuitive interface for visualising database statistics, managing security policies, administering user accounts, and running queries in a visual SQL console.

#### Accessing the Dashboard
Once the `flamingodbd` daemon is running (e.g. on port `8080`), open your web browser and navigate to:
```
http://localhost:8080/
```
Or specifically to the subpath:
```
http://localhost:8080/ui
```
*Note: The dashboard requires HTTP Basic Authentication matching the username and password flags specified when starting the daemon (default credentials: `admin` / `admin`).*

#### Key Features
- 🖥️ **SQL Console**: Execute custom SQL statements directly in the browser and view nicely-formatted result tables.
- 📦 **Table Browser**: Inspect table structures, columns, types, and schemas stored in the catalog.
- 👥 **User Management**: Add new database users, delete existing accounts, and update passwords.
- 🛡️ **Named Policy Store**: Create, edit, and assign security policies (e.g. `Read-Only`, `Read-Write`, or custom DDL/DML permission matrices) to users.
- 🔄 **Transaction Monitoring**: Real-time indication of current connections and active transaction status.

---

## 🧪 Tests

**Reproducible research** demands reproducible software. Every package requires unit tests; every bug fix requires a regression test. FlamingoDB enforces this as a hard rule.

```bash
go test ./...
```

**Current Results — All Passing:**
```
ok   github.com/TaqsBlaze/FlamingoDB/internal/datatypes         0.005s
ok   github.com/TaqsBlaze/FlamingoDB/internal/executor          0.025s
ok   github.com/TaqsBlaze/FlamingoDB/internal/functions         0.002s
ok   github.com/TaqsBlaze/FlamingoDB/internal/index/btree       0.232s
ok   github.com/TaqsBlaze/FlamingoDB/internal/parser/lexer      0.029s
ok   github.com/TaqsBlaze/FlamingoDB/internal/parser/parser     0.038s
ok   github.com/TaqsBlaze/FlamingoDB/internal/planner           0.022s
ok   github.com/TaqsBlaze/FlamingoDB/internal/storage/catalog   0.027s
ok   github.com/TaqsBlaze/FlamingoDB/internal/storage/disk      0.038s
ok   github.com/TaqsBlaze/FlamingoDB/internal/storage/encoding  0.040s
ok   github.com/TaqsBlaze/FlamingoDB/internal/storage/pager     0.013s
ok   github.com/TaqsBlaze/FlamingoDB/internal/storage/record    0.011s
ok   github.com/TaqsBlaze/FlamingoDB/internal/storage/table     0.010s
ok   github.com/TaqsBlaze/FlamingoDB/tests                      0.103s
```

---


## 🔖 Keywords

`scientific database system` · `scientific data management` · `research data infrastructure` · `high performance data storage` · `computational research` · `large-scale scientific datasets` · `bioinformatics workflows` · `data-intensive science` · `reproducible research` · `open source scientific software` · `database engine` · `vector database` · `matrix storage` · `geospatial database` · `Go database`

---

## 📄 License

FlamingoDB is licensed under the **MIT License** — see [`LICENSE`](./LICENSE) for details.
