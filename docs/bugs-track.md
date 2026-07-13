# Bug Tracking Log

This document records the bugs identified during robustness and edge case integration testing in FlamingoDB, detailing their symptoms, root causes, and the corrective actions taken.

## Summary

| Bug | Fix | Status |
| :--- | :--- | :--- |
| **Bug 001**: Record Serialization Buffer Out-of-Bounds Panic | Dynamically compute required serialization buffer size based on column schema before allocation instead of hardcoding 1024 bytes. | Fixed |
| **Bug 002**: Heap Table `lastPageID` Link Overwrite | Traverse page linked list in `table.New` on database open to find the actual tail page containing `NextPageID == NoPage`. | Fixed |
| **Bug 003**: Query Engine Semantic Validation Bypass on Empty Tables | Move query filter and column projection schema validation logic outside of row iteration loops to ensure checks execute for empty tables. | Fixed |
| **Bug 004**: Auto-Commit Transaction Lock Leak / Deadlock | Use named return parameters in `TableManager.CreateTable` and `TableManager.InsertRecord` and remove local `err` shadows to ensure `Rollback` executes on errors. | Fixed |

---

## Bug 001: Record Serialization Buffer Out-of-Bounds Panic

*   **Status**: Fixed
*   **Module**: Storage Engine (`internal/storage/record`)
*   **File**: `internal/storage/record/record.go`
*   **Identified by**: `TestRobustnessStorageLimits`

### Symptom
When inserting or serializing a record whose total size exceeded 1024 bytes (e.g. containing large strings), the database engine crashed with the following runtime error:
`panic: runtime error: slice bounds out of range [:4080] with capacity 1024`

### Cause
In the `Record.Serialize` method, the serialization buffer was hardcoded with a fixed capacity of 1024 bytes:
```go
func (r *Record) Serialize(schema *Schema) []byte {
    buf := make([]byte, 1024) // pre-allocate
    ...
```
When copying large string fields or multiple numeric fields into this buffer, the internal byte offset exceeded the slice limits, triggering an out-of-bounds bounds panic.

### Fix
Refactored `Record.Serialize` to dynamically compute the exact serialized byte length of the record before allocating the buffer. This ensures that the buffer is always sized perfectly for the record content, saving allocations and preventing panics:
```go
func (r *Record) Serialize(schema *Schema) []byte {
    size := 0
    for i, col := range schema.Columns {
        val := r.Values[i]
        switch col.Type {
        case Integer:
            size += 4
        case Float:
            size += 8
        case Varchar:
            size += 4 + len(val.Str)
        }
    }

    buf := make([]byte, size)
    ...
```

---

## Bug 002: Heap Table `lastPageID` Link Overwrite

*   **Status**: Fixed
*   **Module**: Storage Engine (`internal/storage/table`)
*   **File**: `internal/storage/table/table.go`
*   **Identified by**: `TestRobustnessMultiPageHeap`

### Symptom
In tables spanning multiple pages, only a subset of the records could be read back (e.g., in a test inserting 300 records, only 37 were retrieved). The records in intermediate pages were lost/overwritten.

### Cause
When a `Table` was loaded or opened via `table.New` (such as inside `TableManager.InsertRecord` for existing tables), `lastPageID` was unconditionally initialized to `firstPageID`:
```go
func New(p *pager.Pager, firstPageID page.PageID, initialize bool) (*Table, error) {
    t := &Table{
        pager:       p,
        firstPageID: firstPageID,
        lastPageID:  firstPageID, // resets on load
    }
    ...
```
Because of this, when inserting a new record, the table believed the first page was the last page. It fetched the first page, found it full, allocated a new page, linked the first page directly to the new page, and wrote the record there. On subsequent insertions, this process repeated: a new page was allocated and linked directly from the first page, overwriting the previous link and effectively severing/orphaning the prior overflow pages.

### Fix
Modified the `New` table constructor. If `initialize` is false (meaning we are opening an existing table), the engine now traverses the page linked list starting from `firstPageID` until it locates the actual tail page (where `NextPageID == NoPage`), setting `lastPageID` correctly:
```go
    } else {
        // Traverse the page linked list to find the actual lastPageID
        currPageID := firstPageID
        for {
            pg, err := p.FetchPage(currPageID)
            if err != nil {
                return nil, err
            }
            nextID := encoding.Uint32(pg.Data()[8:12])
            if page.PageID(nextID) == NoPage {
                t.lastPageID = currPageID
                break
            }
            currPageID = page.PageID(nextID)
        }
    }
```

---

## Bug 003: Query Engine Semantic Validation Bypass on Empty Tables

*   **Status**: Fixed
*   **Module**: Query Executor (`internal/executor`)
*   **File**: `internal/executor/executor.go`
*   **Identified by**: `TestRobustnessErrorHandling`

### Symptom
SQL queries with obvious semantic errors (e.g. projecting a non-existent column name or comparing an integer column to an incompatible type like a string) completed with a `nil` error and returned an empty success result when run against an empty table.

### Cause
In `executeProject` and `executeFilter`, the schema mapping and validation logic (checking column names and type evaluation of filter constants) was placed inside the loops that processed the rows:
```go
    for _, row := range childResult.Rows {
        var vals []record.Value
        for _, field := range n.Fields {
            idx, ok := colIndex[field]
            if !ok {
                return nil, fmt.Errorf("column %q not found in table", field)
            }
            ...
```
If a table contained zero rows, the loop body was never entered. Thus, the invalid columns or filters were never checked, and no error was thrown.

### Fix
Separated query validation from row evaluation. We now validate that all projected columns exist and that filter conditions are compatible with the table schema *before* looping over child rows:
*   In `executeProject`, we check column existence in the index map before row processing.
*   Introduced a new helper `validateCondition` to pre-check conditions before row processing in `executeFilter`.

---

## Bug 004: Auto-Commit Transaction Lock Leak / Deadlock

*   **Status**: Fixed
*   **Module**: Storage Catalog (`internal/storage/catalog`)
*   **File**: `internal/storage/catalog/table_manager.go`
*   **Identified by**: `TestRobustnessErrorHandling` (Timeout failure)

### Symptom
When a statement returned an error during auto-commit execution (such as attempting to create a duplicate table), the query engine stalled completely, causing tests to hang indefinitely and eventually time out (10-minute limit).

### Cause
Auto-commit execution in `TableManager.CreateTable` and `TableManager.InsertRecord` begins a temporary transaction context using `tx, err = tm.Begin()`. If subsequent catalog operations fail, a deferred function is registered to roll back the transaction and release the database's exclusive transaction lock:
```go
	isAutoCommit := (tx == nil)
	if isAutoCommit {
		var err error   // <--- Shadow variable declared inside block
		tx, err = tm.Begin()
		if err != nil {
			return err
		}
		defer func() {
			if err != nil {
				tm.Rollback(tx)
			}
		}()
	}
```
However, the `err` variable inside the block was shadowed by the outer function's `err` variables (e.g. `tbl, err := table.New(...)`). When subsequent table writes failed and returned errors, they updated the outer scope's `err` variable, but the block-scoped `err` captured by the deferred rollback closure remained `nil`.
As a result, the deferred function bypassed calling `Rollback(tx)`, leaving the transaction active and the global exclusive lock locked. The next query block hung indefinitely waiting for the lock.

### Fix
Refactored the methods to use a **named return parameter** `(err error)` and eliminated all block-scoped `err` declarations. The deferred function now references the function's shared return parameter directly, executing `Rollback(tx)` and releasing transaction locks upon any failure:
```go
func (tm *TableManager) CreateTable(tx *transaction.Transaction, name string, schema *record.Schema) (err error) {
	isAutoCommit := (tx == nil)
	if isAutoCommit {
		tx, err = tm.Begin()
		if err != nil {
			return err
		}
		defer func() {
			if err != nil {
				tm.Rollback(tx)
			}
		}()
	}
	...
```

