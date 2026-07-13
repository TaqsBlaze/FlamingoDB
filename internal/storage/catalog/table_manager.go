package catalog

import (
	"flamingodb/internal/storage/pager"
	"flamingodb/internal/storage/record"
	"flamingodb/internal/storage/table"
)

// TableManager coordinates creating tables, inserting, and reading records using schemas and metadata.
type TableManager struct {
	pager   *pager.Pager
	catalog *Catalog
}

// NewTableManager creates a new TableManager.
func NewTableManager(p *pager.Pager) (*TableManager, error) {
	c, err := New(p)
	if err != nil {
		return nil, err
	}
	return &TableManager{
		pager:   p,
		catalog: c,
	}, nil
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
func (tm *TableManager) CreateTable(name string, schema *record.Schema) error {
	// table.New with initialize = true allocates the first page.
	// We pass 0 as a placeholder since it allocates a new page and overrides it anyway.
	tbl, err := table.New(tm.pager, 0, true)
	if err != nil {
		return err
	}

	return tm.catalog.CreateTable(name, schema, tbl.FirstPageID())
}

// InsertRecord serializes and inserts a record into the specified table.
func (tm *TableManager) InsertRecord(tableName string, rec *record.Record) error {
	meta, err := tm.catalog.GetTable(tableName)
	if err != nil {
		return err
	}

	// We pass false to table.New since the table already exists.
	tbl, err := table.New(tm.pager, meta.FirstPageID, false)
	if err != nil {
		return err
	}

	serialized := rec.Serialize(meta.Schema)
	_, err = tbl.InsertRecord(serialized)
	return err
}

// ReadRecords reads and deserializes all records from the specified table.
func (tm *TableManager) ReadRecords(tableName string) ([]*record.Record, error) {
	meta, err := tm.catalog.GetTable(tableName)
	if err != nil {
		return nil, err
	}

	tbl, err := table.New(tm.pager, meta.FirstPageID, false)
	if err != nil {
		return nil, err
	}

	rawRecords, err := tbl.ReadAll()
	if err != nil {
		return nil, err
	}

	records := make([]*record.Record, len(rawRecords))
	for i, raw := range rawRecords {
		records[i] = record.Deserialize(raw, meta.Schema)
	}

	return records, nil
}
