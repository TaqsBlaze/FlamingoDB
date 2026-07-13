package catalog

import (
	"flamingodb/internal/storage/pager"
	"flamingodb/internal/storage/record"
	"flamingodb/internal/storage/table"
	"flamingodb/internal/transaction"
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

// GetSchema retrieves the schema for a given table.
func (tm *TableManager) GetSchema(tableName string) (*record.Schema, error) {
	meta, err := tm.catalog.GetTable(tableName)
	if err != nil {
		return nil, err
	}
	return meta.Schema, nil
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

	var tbl *table.Table
	tbl, err = table.New(tm.pager, tm.txMgr, tx, meta.FirstPageID, false)
	if err != nil {
		return err
	}

	serialized := rec.Serialize(meta.Schema)
	_, err = tbl.InsertRecord(tx, serialized)
	if err != nil {
		return err
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
