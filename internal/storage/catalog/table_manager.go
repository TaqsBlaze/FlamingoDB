package catalog

import (
	"fmt"
	"strings"

	"github.com/TaqsBlaze/FlamingoDB/internal/index/btree"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/encoding"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/page"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/pager"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/record"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/table"
	"github.com/TaqsBlaze/FlamingoDB/internal/transaction"
)

// TableManager coordinates creating tables, inserting, and reading records using schemas and metadata.
type TableManager struct {
	pager   *pager.Pager
	catalog *Catalog
	txMgr   *transaction.TransactionManager
}

// NewTableManager creates a new TableManager and performs crash recovery.
func NewTableManager(p *pager.Pager) (*TableManager, error) {
	walPath := p.Filename() + ".wal"
	txMgr, err := transaction.NewTransactionManager(p, walPath)
	if err != nil {
		return nil, err
	}

	// Run crash recovery on database startup
	if err := txMgr.Recover(); err != nil {
		txMgr.Close()
		return nil, err
	}

	c, err := New(p, txMgr)
	if err != nil {
		txMgr.Close()
		return nil, err
	}

	return &TableManager{
		pager:   p,
		catalog: c,
		txMgr:   txMgr,
	}, nil
}

// Close closes the underlying transaction manager.
func (tm *TableManager) Close() error {
	return tm.txMgr.Close()
}

// ReloadCatalog reloads the catalog from disk, reversing uncommitted modifications.
func (tm *TableManager) ReloadCatalog() error {
	return tm.catalog.Reload()
}

// Begin starts a new transaction.
func (tm *TableManager) Begin() (*transaction.Transaction, error) {
	return tm.txMgr.Begin()
}

// Commit commits the transaction.
func (tm *TableManager) Commit(tx *transaction.Transaction) error {
	return tm.txMgr.Commit(tx)
}

// Rollback rolls back the transaction and reloads the catalog.
func (tm *TableManager) Rollback(tx *transaction.Transaction) error {
	if err := tm.txMgr.Rollback(tx); err != nil {
		return err
	}
	return tm.catalog.Reload()
}

// Recover runs database recovery from the WAL.
func (tm *TableManager) Recover() error {
	return tm.txMgr.Recover()
}

// GetSchema returns the schema for a table.
func (tm *TableManager) GetSchema(tableName string) (*record.Schema, error) {
	meta, err := tm.catalog.GetTable(tableName)
	if err != nil {
		return nil, err
	}
	return meta.Schema, nil
}

// ListTables returns a list of all tables.
func (tm *TableManager) ListTables() []string {
	return tm.catalog.ListTables()
}

// CreateTable registers a new table with a schema and allocates its first page.
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

	var tbl *table.Table
	tbl, err = table.New(tm.pager, tm.txMgr, tx, 0, true)
	if err != nil {
		return err
	}

	err = tm.catalog.CreateTable(tx, name, schema, tbl.FirstPageID())
	if err != nil {
		return err
	}

	if isAutoCommit {
		err = tm.Commit(tx)
	}
	return err
}

// DropTable removes a table from the catalog.
func (tm *TableManager) DropTable(tx *transaction.Transaction, name string) (err error) {
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

	err = tm.catalog.DropTable(tx, name)
	if err != nil {
		return err
	}

	if isAutoCommit {
		err = tm.Commit(tx)
	}
	return err
}

// DeleteRecord deletes the first record matching rec from tableName by tombstoning it.
// It returns an error if no matching record is found.
func (tm *TableManager) DeleteRecord(tx *transaction.Transaction, tableName string, rec *record.Record) (err error) {
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

	meta, err := tm.catalog.GetTable(tableName)
	if err != nil {
		return err
	}

	// Serialize the record to compare against page contents
	payload := rec.Serialize(meta.Schema)

	// Open the table heap
	tbl, err := table.New(tm.pager, tm.txMgr, tx, meta.FirstPageID, false)
	if err != nil {
		return err
	}

	// Attempt to tombstone the first matching record
	found, err := tbl.TombstoneRecord(tx, payload)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("delete on %q: record not found", tableName)
	}

	if isAutoCommit {
		err = tm.Commit(tx)
	}
	return err
}

// InsertRecord serializes and inserts a record into the specified table.
func (tm *TableManager) InsertRecord(tx *transaction.Transaction, tableName string, rec *record.Record) (err error) {
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

	meta, err := tm.catalog.GetTable(tableName)
	if err != nil {
		return err
	}

	// Ensure that no more than 1 record with the same ID can exist in the table
	idColIdx := -1
	for i, col := range meta.Schema.Columns {
		if strings.EqualFold(col.Name, "id") {
			idColIdx = i
			break
		}
	}

	if idColIdx != -1 && len(rec.Values) > idColIdx {
		// Read all records to check for duplicate ID
		existingRecs, err := tm.ReadRecords(tx, tableName)
		if err != nil {
			return err
		}
		newVal := rec.Values[idColIdx]
		for _, er := range existingRecs {
			if len(er.Values) > idColIdx {
				oldVal := er.Values[idColIdx]
				if valuesEqual(oldVal, newVal) {
					return fmt.Errorf("duplicate key error: a record with id %s already exists in table %q", formatValue(newVal), tableName)
				}
			}
		}
	}

	var tbl *table.Table
	tbl, err = table.New(tm.pager, tm.txMgr, tx, meta.FirstPageID, false)
	if err != nil {
		return err
	}

	serialized := rec.Serialize(meta.Schema)
	var pageID page.PageID
	pageID, err = tbl.InsertRecord(tx, serialized)
	if err != nil {
		return err
	}

	// Insert into B+ tree indexes
	for colName, idxMeta := range meta.Indexes {
		colIdx := -1
		for i, col := range meta.Schema.Columns {
			if strings.EqualFold(col.Name, colName) {
				colIdx = i
				break
			}
		}
		if colIdx == -1 {
			continue
		}
		val := rec.Values[colIdx]
		btKey := btree.Key{Type: idxMeta.KeyType}
		switch idxMeta.KeyType {
		case btree.KeyInt:
			btKey.IVal = val.Int
		case btree.KeyFloat:
			btKey.FVal = val.Flt
		case btree.KeyVarchar:
			btKey.SVal = val.Str
		}
		bt := btree.Load(tm.pager, tm.pager.PageSize(), idxMeta.KeyType, idxMeta.RootPageID)
		if err := bt.Insert(btKey, pageID); err != nil {
			if err != btree.ErrDuplicateKey {
				return fmt.Errorf("failed to insert key into index %s.%s: %w", tableName, colName, err)
			}
		}
	}

	if isAutoCommit {
		err = tm.Commit(tx)
	}
	return err
}

// ReadRecords reads and deserializes all records from the specified table.
func (tm *TableManager) ReadRecords(tx *transaction.Transaction, tableName string) ([]*record.Record, error) {
	meta, err := tm.catalog.GetTable(tableName)
	if err != nil {
		return nil, err
	}

	tbl, err := table.New(tm.pager, tm.txMgr, tx, meta.FirstPageID, false)
	if err != nil {
		return nil, err
	}

	rawRecords, err := tbl.ReadAll(tx)
	if err != nil {
		return nil, err
	}

	records := make([]*record.Record, len(rawRecords))
	for i, raw := range rawRecords {
		records[i] = record.Deserialize(raw, meta.Schema)
	}

	return records, nil
}

// CreateIndex registers a B+ Tree index on the specified column. It allocates a root page,
// populates the index with existing records, and updates the table catalog metadata.
func (tm *TableManager) CreateIndex(tx *transaction.Transaction, tableName string, columnName string) (err error) {
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

	meta, err := tm.catalog.GetTable(tableName)
	if err != nil {
		return err
	}

	// Case-insensitive search for the column and its type
	colIdx := -1
	var colType record.TypeID
	for i, col := range meta.Schema.Columns {
		if strings.EqualFold(col.Name, columnName) {
			colIdx = i
			colType = col.Type
			// Use the exact case from schema for metadata storage
			columnName = col.Name
			break
		}
	}
	if colIdx == -1 {
		return fmt.Errorf("column %q not found in table %q", columnName, tableName)
	}

	// Map record.TypeID to btree.KeyType
	var keyType btree.KeyType
	switch colType {
	case record.Integer:
		keyType = btree.KeyInt
	case record.Float:
		keyType = btree.KeyFloat
	case record.Varchar:
		keyType = btree.KeyVarchar
	default:
		return fmt.Errorf("indexes not supported on type %v", colType)
	}

	if _, exists := meta.Indexes[columnName]; exists {
		return fmt.Errorf("index already exists on column %s.%s", tableName, columnName)
	}

	bt, err := btree.New(tm.pager, tm.pager.PageSize(), keyType)
	if err != nil {
		return err
	}

	_, err = table.New(tm.pager, tm.txMgr, tx, meta.FirstPageID, false)
	if err != nil {
		return err
	}

	currPageID := meta.FirstPageID
	for currPageID != table.NoPage {
		var pg *page.Page
		if tm.txMgr != nil && tx != nil {
			pg, err = tm.txMgr.FetchPage(tx, currPageID)
		} else {
			pg, err = tm.pager.FetchPage(currPageID)
		}
		if err != nil {
			return err
		}

		numRecords := encoding.Uint32(pg.Data()[0:4])
		offset := uint32(table.PageHeaderSize)

		for i := uint32(0); i < numRecords; i++ {
			recordSize := encoding.Uint32(pg.Data()[offset+1 : offset+1+4])
			rawRecord := make([]byte, recordSize)
				copy(rawRecord, pg.Data()[offset+1+4:offset+1+4+recordSize])

			rec := record.Deserialize(rawRecord, meta.Schema)
			val := rec.Values[colIdx]

			btKey := btree.Key{Type: keyType}
			switch keyType {
			case btree.KeyInt:
				btKey.IVal = val.Int
			case btree.KeyFloat:
				btKey.FVal = val.Flt
			case btree.KeyVarchar:
				btKey.SVal = val.Str
			}

			if err := bt.Insert(btKey, currPageID); err != nil {
				if err != btree.ErrDuplicateKey {
					return err
				}
			}

				offset += 1 + 4 + recordSize
		}

		nextID := encoding.Uint32(pg.Data()[8:12])
		currPageID = page.PageID(nextID)
	}

	meta.Indexes[columnName] = &IndexMetadata{
		ColumnName: columnName,
		RootPageID: bt.RootID(),
		KeyType:    keyType,
	}

	err = tm.catalog.persist(tx)
	if err != nil {
		return err
	}

	if isAutoCommit {
		err = tm.Commit(tx)
	}
	return err
}

// GetIndexes returns the map of active indexes on a table.
func (tm *TableManager) GetIndexes(tableName string) (map[string]*IndexMetadata, error) {
	meta, err := tm.catalog.GetTable(tableName)
	if err != nil {
		return nil, err
	}
	return meta.Indexes, nil
}

// NextSequenceValue returns the next sequence value for an AUTO_INCREMENT column.
// If the column doesn't exist or isn't AUTO_INCREMENT, it returns an error.
// The sequence is incremented and persisted.
func (tm *TableManager) NextSequenceValue(tx *transaction.Transaction, tableName string, columnName string) (int32, error) {
	meta, err := tm.catalog.GetTable(tableName)
	if err != nil {
		return 0, err
	}

	// Find the column index
	colIdx := -1
	for i, col := range meta.Schema.Columns {
		if strings.EqualFold(col.Name, columnName) {
			colIdx = i
			break
		}
	}
	if colIdx == -1 {
		return 0, fmt.Errorf("column %q not found in table %q", columnName, tableName)
	}

	// Check if the column is AUTO_INCREMENT
	if !meta.Schema.Columns[colIdx].AutoIncrement {
		return 0, fmt.Errorf("column %q.%q is not AUTO_INCREMENT", tableName, columnName)
	}

	// Get current sequence value
	seq := meta.Sequences[columnName]
	nextSeq := seq + 1

	// Update the sequence in metadata
	meta.Sequences[columnName] = nextSeq

	// Persist the change
	if err := tm.catalog.persist(tx); err != nil {
		return 0, err
	}

	return nextSeq, nil
}

// ReadRecordsIndexed performs a range scan on the B+ Tree index, gets physical PageIDs,
// loads the corresponding pages from the heap table, and returns all records found in those pages.
func (tm *TableManager) ReadRecordsIndexed(
	tx *transaction.Transaction,
	tableName string,
	columnName string,
	rootPageID page.PageID,
	keyType btree.KeyType,
	low, high btree.Key,
) ([]*record.Record, error) {
	meta, err := tm.catalog.GetTable(tableName)
	if err != nil {
		return nil, err
	}

	bt := btree.Load(tm.pager, tm.pager.PageSize(), keyType, rootPageID)
	pageIDs, err := bt.RangeScan(low, high)
	if err != nil {
		return nil, err
	}

	uniquePageIDs := make([]page.PageID, 0, len(pageIDs))
	seen := make(map[page.PageID]bool)
	for _, pid := range pageIDs {
		if !seen[pid] {
			seen[pid] = true
			uniquePageIDs = append(uniquePageIDs, pid)
		}
	}

	var records []*record.Record
	for _, currPageID := range uniquePageIDs {
		var pg *page.Page
		if tm.txMgr != nil && tx != nil {
			pg, err = tm.txMgr.FetchPage(tx, currPageID)
		} else {
			pg, err = tm.pager.FetchPage(currPageID)
		}
		if err != nil {
			return nil, err
		}

		numRecords := encoding.Uint32(pg.Data()[0:4])
		offset := uint32(table.PageHeaderSize)

			for i := uint32(0); i < numRecords; i++ {
				deleted := pg.Data()[offset]
				if deleted == 0 { // not tombstoned
					recordSize := encoding.Uint32(pg.Data()[offset+1 : offset+1+4])
					rawRecord := make([]byte, recordSize)
					copy(rawRecord, pg.Data()[offset+1+4 : offset+1+4+recordSize])

					rec := record.Deserialize(rawRecord, meta.Schema)
					records = append(records, rec)
				}
				// advance past: 1 byte flag + 4 byte size + recordSize payload
				offset += 1 + 4 + encoding.Uint32(pg.Data()[offset+1 : offset+1+4])
			}
	}

	return records, nil
}

func valuesEqual(a, b record.Value) bool {
	if a.Type != b.Type {
		return false
	}
	switch a.Type {
	case record.Integer:
		return a.Int == b.Int
	case record.Float:
		return a.Flt == b.Flt
	case record.Varchar:
		return a.Str == b.Str
	default:
		return false
	}
}

func formatValue(v record.Value) string {
	switch v.Type {
	case record.Integer:
		return fmt.Sprintf("%d", v.Int)
	case record.Float:
		return fmt.Sprintf("%g", v.Flt)
	case record.Varchar:
		return fmt.Sprintf("%q", v.Str)
	default:
		return "unknown"
	}
}
