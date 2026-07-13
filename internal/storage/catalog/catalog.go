package catalog

import (
	"errors"
	"sync"

	"flamingodb/internal/storage/encoding"
	"flamingodb/internal/storage/page"
	"flamingodb/internal/storage/pager"
	"flamingodb/internal/storage/record"
)

var (
	ErrTableExists   = errors.New("table already exists")
	ErrTableNotFound = errors.New("table not found")
)

// TableMetadata stores metadata for a single table.
type TableMetadata struct {
	Name        string
	FirstPageID page.PageID
	Schema      *record.Schema
}

// Catalog manages database tables and their schemas.
// It persists itself to Page 0 of the database file.
type Catalog struct {
	pager  *pager.Pager
	tables map[string]*TableMetadata
	mu     sync.RWMutex
}

// New creates or loads a Catalog from the database.
func New(p *pager.Pager) (*Catalog, error) {
	c := &Catalog{
		pager:  p,
		tables: make(map[string]*TableMetadata),
	}

	// Fetch page 0, which stores the catalog.
	// If it doesn't exist yet, allocate it.
	pg, err := p.FetchPage(0)
	if err != nil {
		if errors.Is(err, pager.ErrPageNotFound) {
			// Allocate page 0
			pg, err = p.AllocatePage()
			if err != nil {
				return nil, err
			}
			// Write initial empty catalog (all zeros)
			if err := p.WritePage(pg); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	tables, err := deserializeCatalog(pg.Data())
	if err != nil {
		return nil, err
	}
	c.tables = tables

	return c, nil
}

// CreateTable adds a new table metadata entry.
func (c *Catalog) CreateTable(name string, schema *record.Schema, firstPageID page.PageID) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.tables[name]; exists {
		return ErrTableExists
	}

	c.tables[name] = &TableMetadata{
		Name:        name,
		FirstPageID: firstPageID,
		Schema:      schema,
	}

	return c.persist()
}

// GetTable retrieves metadata for a table.
func (c *Catalog) GetTable(name string) (*TableMetadata, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	meta, exists := c.tables[name]
	if !exists {
		return nil, ErrTableNotFound
	}
	return meta, nil
}

func (c *Catalog) persist() error {
	pg, err := c.pager.FetchPage(0)
	if err != nil {
		return err
	}

	data := c.serialize()
	pg.CopyData(data)

	return c.pager.WritePage(pg)
}

func (c *Catalog) serialize() []byte {
	buf := make([]byte, 8192) // matching page size
	offset := 0

	encoding.PutUint32(buf[offset:], uint32(len(c.tables)))
	offset += 4

	for _, t := range c.tables {
		n := encoding.PutString(buf[offset:], t.Name)
		offset += n

		encoding.PutUint32(buf[offset:], uint32(t.FirstPageID))
		offset += 4

		encoding.PutUint32(buf[offset:], uint32(len(t.Schema.Columns)))
		offset += 4

		for _, col := range t.Schema.Columns {
			n = encoding.PutString(buf[offset:], col.Name)
			offset += n

			buf[offset] = uint8(col.Type)
			offset += 1
		}
	}

	return buf
}

func deserializeCatalog(data []byte) (map[string]*TableMetadata, error) {
	tables := make(map[string]*TableMetadata)
	offset := 0

	numTables := encoding.Uint32(data[offset:])
	offset += 4

	for i := uint32(0); i < numTables; i++ {
		name, n := encoding.String(data[offset:])
		offset += n

		firstPageID := page.PageID(encoding.Uint32(data[offset:]))
		offset += 4

		numCols := encoding.Uint32(data[offset:])
		offset += 4

		cols := make([]record.Column, numCols)
		for j := uint32(0); j < numCols; j++ {
			colName, n := encoding.String(data[offset:])
			offset += n

			colType := record.TypeID(data[offset])
			offset += 1

			cols[j] = record.Column{Name: colName, Type: colType}
		}

		tables[name] = &TableMetadata{
			Name:        name,
			FirstPageID: firstPageID,
			Schema:      record.NewSchema(cols),
		}
	}

	return tables, nil
}
