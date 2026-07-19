package catalog

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/TaqsBlaze/FlamingoDB/internal/index/btree"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/encoding"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/page"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/pager"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/record"
	"github.com/TaqsBlaze/FlamingoDB/internal/transaction"
)

var (
	ErrTableExists   = errors.New("table already exists")
	ErrTableNotFound = errors.New("table not found")
)

// IndexMetadata stores metadata for a B+ tree index on a column.
type IndexMetadata struct {
	ColumnName string
	RootPageID page.PageID
	KeyType    btree.KeyType
}

// TableMetadata stores metadata for a single table.
type TableMetadata struct {
	Name        string
	FirstPageID page.PageID
	Schema      *record.Schema
	Indexes     map[string]*IndexMetadata
	Sequences   map[string]int32 // Per-column AUTO_INCREMENT sequence counters
}

// Catalog manages database tables and their schemas.
// It persists itself to Page 0 of the database file.
type Catalog struct {
	pager  *pager.Pager
	txMgr  *transaction.TransactionManager
	tables map[string]*TableMetadata
	mu     sync.RWMutex
}

// New creates or loads a Catalog from the database.
func New(p *pager.Pager, txMgr *transaction.TransactionManager) (*Catalog, error) {
	c := &Catalog{
		pager:  p,
		txMgr:  txMgr,
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
func (c *Catalog) CreateTable(tx *transaction.Transaction, name string, schema *record.Schema, firstPageID page.PageID) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.tables[name]; exists {
		return ErrTableExists
	}

	c.tables[name] = &TableMetadata{
		Name:        name,
		FirstPageID: firstPageID,
		Schema:      schema,
		Indexes:     make(map[string]*IndexMetadata),
		Sequences:   make(map[string]int32),
	}

	return c.persist(tx)
}

// DropTable removes a table metadata entry.
func (c *Catalog) DropTable(tx *transaction.Transaction, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.tables[name]; !exists {
		return ErrTableNotFound
	}

	delete(c.tables, name)
	return c.persist(tx)
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

// ListTables returns a list of all table names in the catalog.
func (c *Catalog) ListTables() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var names []string
	for name := range c.tables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Reload re-reads the Catalog from disk, reversing uncommitted modifications.
func (c *Catalog) Reload() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict the cached page to ensure we read the latest data from disk.
	c.pager.Evict(0)
	pg, err := c.pager.FetchPage(0)
	if err != nil {
		return err
	}

	tables, err := deserializeCatalog(pg.Data())
	if err != nil {
		return err
	}
	c.tables = tables
	return nil
}

func (c *Catalog) persist(tx *transaction.Transaction) error {
	var pg *page.Page
	var err error
	if c.txMgr != nil && tx != nil {
		pg, err = c.txMgr.FetchPage(tx, 0)
	} else {
		pg, err = c.pager.FetchPage(0)
	}
	if err != nil {
		return err
	}

	data := c.serialize()
	pg.CopyData(data)

	if c.txMgr != nil && tx != nil {
		return c.txMgr.WritePage(tx, pg)
	} else {
		return c.pager.WritePage(pg)
	}
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
			if col.AutoIncrement {
				buf[offset] = 1
			} else {
				buf[offset] = 0
			}
			offset += 1
		}

		encoding.PutUint32(buf[offset:], uint32(len(t.Indexes)))
		offset += 4

		for _, idx := range t.Indexes {
			n = encoding.PutString(buf[offset:], idx.ColumnName)
			offset += n

			encoding.PutUint32(buf[offset:], uint32(idx.RootPageID))
			offset += 4

			buf[offset] = uint8(idx.KeyType)
			offset += 1
		}

		// Serialize sequences (AUTO_INCREMENT counters)
		encoding.PutUint32(buf[offset:], uint32(len(t.Sequences)))
		offset += 4
		for colName, seq := range t.Sequences {
			n := encoding.PutString(buf[offset:], colName)
			offset += n
			encoding.PutUint32(buf[offset:], uint32(seq))
			offset += 4
		}
	}

	return buf
}

func deserializeCatalog(data []byte) (map[string]*TableMetadata, error) {
	tables := make(map[string]*TableMetadata)
	offset := 0

	if len(data) < 4 {
		return nil, fmt.Errorf("catalog data too short for number of tables")
	}
	numTables := encoding.Uint32(data[offset:])
	offset += 4

	for i := uint32(0); i < numTables; i++ {
		if offset+4 > len(data) {
			return nil, fmt.Errorf("catalog data too short for table name length")
		}
		nameLen := encoding.Uint32(data[offset:])
		offset += 4
		if uint64(offset)+uint64(nameLen) > uint64(len(data)) {
			return nil, fmt.Errorf("table name length %d out of bounds", nameLen)
		}
		name := string(data[offset : offset+int(nameLen)])
		offset += int(nameLen)

		if offset+4 > len(data) {
			return nil, fmt.Errorf("catalog data too short for first page ID")
		}
		firstPageID := page.PageID(encoding.Uint32(data[offset:]))
		offset += 4

		if offset+4 > len(data) {
			return nil, fmt.Errorf("catalog data too short for number of columns")
		}
		numCols := encoding.Uint32(data[offset:])
		offset += 4

		cols := make([]record.Column, numCols)
		for j := uint32(0); j < numCols; j++ {
			if offset+4 > len(data) {
				return nil, fmt.Errorf("catalog data too short for column name length")
			}
			colNameLen := encoding.Uint32(data[offset:])
			offset += 4
			if uint64(offset)+uint64(colNameLen) > uint64(len(data)) {
				return nil, fmt.Errorf("column name length %d out of bounds", colNameLen)
			}
			colName := string(data[offset : offset+int(colNameLen)])
			offset += int(colNameLen)

			if offset+1 > len(data) {
				return nil, fmt.Errorf("catalog data too short for column type")
			}
			colType := record.TypeID(data[offset])
			offset += 1

			if offset+1 > len(data) {
				return nil, fmt.Errorf("catalog data too short for auto increment flag")
			}
			autoInc := data[offset] != 0
			offset += 1

			cols[j] = record.Column{Name: colName, Type: colType, AutoIncrement: autoInc}
		}

		if offset+4 > len(data) {
			return nil, fmt.Errorf("catalog data too short for number of indexes")
		}
		numIndexes := encoding.Uint32(data[offset:])
		offset += 4

		indexes := make(map[string]*IndexMetadata)
		for j := uint32(0); j < numIndexes; j++ {
			if offset+4 > len(data) {
				return nil, fmt.Errorf("catalog data too short for index column name length")
			}
			idxColNameLen := encoding.Uint32(data[offset:])
			offset += 4
			if uint64(offset)+uint64(idxColNameLen) > uint64(len(data)) {
				return nil, fmt.Errorf("index column name length %d out of bounds", idxColNameLen)
			}
			colName := string(data[offset : offset+int(idxColNameLen)])
			offset += int(idxColNameLen)

			if offset+4 > len(data) {
				return nil, fmt.Errorf("catalog data too short for index root page ID")
			}
			rootPID := page.PageID(encoding.Uint32(data[offset:]))
			offset += 4

			if offset+1 > len(data) {
				return nil, fmt.Errorf("catalog data too short for index key type")
			}
			kType := btree.KeyType(data[offset])
			offset += 1

			indexes[colName] = &IndexMetadata{
				ColumnName: colName,
				RootPageID: rootPID,
				KeyType:    kType,
			}
		}

		// Deserialize sequences (AUTO_INCREMENT counters)
		if offset+4 > len(data) {
			return nil, fmt.Errorf("catalog data too short for number of sequences")
		}
		numSeqs := int(encoding.Uint32(data[offset:]))
		offset += 4
		sequences := make(map[string]int32)
		for j := 0; j < numSeqs; j++ {
			if offset+4 > len(data) {
				return nil, fmt.Errorf("catalog data too short for sequence column name length")
			}
			seqColNameLen := encoding.Uint32(data[offset:])
			offset += 4
			if uint64(offset)+uint64(seqColNameLen) > uint64(len(data)) {
				return nil, fmt.Errorf("sequence column name length %d out of bounds", seqColNameLen)
			}
			colName := string(data[offset : offset+int(seqColNameLen)])
			offset += int(seqColNameLen)

			if offset+4 > len(data) {
				return nil, fmt.Errorf("catalog data too short for sequence value")
			}
			seq := int32(encoding.Uint32(data[offset:]))
			offset += 4
			sequences[colName] = seq
		}

		tables[name] = &TableMetadata{
			Name:        name,
			FirstPageID: firstPageID,
			Schema:      record.NewSchema(cols),
			Indexes:     indexes,
			Sequences:   sequences,
		}
	}

	return tables, nil
}
