package table

import (
	"errors"
	"math"

	"github.com/TaqsBlaze/FlamingoDB/internal/storage/encoding"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/page"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/pager"
	"github.com/TaqsBlaze/FlamingoDB/internal/transaction"
)

var (
	ErrRecordTooLarge = errors.New("record too large for a page")
)

const (
	// PageHeaderSize is the size in bytes of a heap page header.
	//
	// Layout:
	//   [0:4]   uint32  NumRecords         (live + tombstoned, never decremented)
	//   [4:8]   uint32  FreeSpaceOffset    (end of the allocated region)
	//   [8:12]  uint32  NextPageID
	//   [12:16] uint32  LiveRecordCount    (what reads should return)
	//
	// Each record slot after the header is framed as:
	//   [0:1]   uint8   deleted  (0 = live, 1 = tombstone)
	//   [1:5]   uint32  recordSize
	//   [5:]    bytes   payload
	PageHeaderSize = 16
	NoPage         = page.PageID(math.MaxUint32)
)

// Table manages inserting and reading raw records from a collection of pages.
// In Phase 1, this is a simple append-only heap table using a linked list of pages.
type Table struct {
	pager       *pager.Pager
	txMgr       *transaction.TransactionManager
	firstPageID page.PageID
	lastPageID  page.PageID
}

// New creates a new simple Table. If initialize is true, it allocates the first page.
func New(p *pager.Pager, txMgr *transaction.TransactionManager, tx *transaction.Transaction, firstPageID page.PageID, initialize bool) (*Table, error) {
	t := &Table{
		pager:       p,
		txMgr:       txMgr,
		firstPageID: firstPageID,
		lastPageID:  firstPageID,
	}

	if initialize {
		var pg *page.Page
		var err error
		if txMgr != nil && tx != nil {
			pg, err = txMgr.AllocatePage(tx)
		} else {
			pg, err = p.AllocatePage()
		}
		if err != nil {
			return nil, err
		}
		t.firstPageID = pg.ID()
		t.lastPageID = pg.ID()

		t.initPageHeader(pg)
		if txMgr != nil && tx != nil {
			if err := txMgr.WritePage(tx, pg); err != nil {
				return nil, err
			}
		} else {
			if err := p.WritePage(pg); err != nil {
				return nil, err
			}
		}
	} else {
		// Traverse the page linked list to find the actual lastPageID
		currPageID := firstPageID
		for {
			var pg *page.Page
			var err error
			if txMgr != nil && tx != nil {
				pg, err = txMgr.FetchPage(tx, currPageID)
			} else {
				pg, err = p.FetchPage(currPageID)
			}
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

	return t, nil
}

// FirstPageID returns the first page ID of this table.
func (t *Table) FirstPageID() page.PageID {
	return t.firstPageID
}

func (t *Table) initPageHeader(pg *page.Page) {
	encoding.PutUint32(pg.Data()[0:4], 0)               // NumRecords = 0
	encoding.PutUint32(pg.Data()[4:8], PageHeaderSize)  // FreeSpaceOffset = 16
	encoding.PutUint32(pg.Data()[8:12], uint32(NoPage)) // NextPageID = NoPage
	encoding.PutUint32(pg.Data()[12:16], 0)             // LiveRecordCount = 0
}

// InsertRecord inserts a raw record into the table and returns the pageID it was written to.
func (t *Table) InsertRecord(tx *transaction.Transaction, record []byte) (page.PageID, error) {
	recordSize := uint32(len(record))
	// Per-record framing: 1 byte deleted flag + 4 byte size prefix + payload.
	totalSize := recordSize + 5

	var pg *page.Page
	var err error
	if t.txMgr != nil && tx != nil {
		pg, err = t.txMgr.FetchPage(tx, t.lastPageID)
	} else {
		pg, err = t.pager.FetchPage(t.lastPageID)
	}
	if err != nil {
		return 0, err
	}

	freeOffset := encoding.Uint32(pg.Data()[4:8])

	// Check if it fits
	if freeOffset+totalSize > uint32(len(pg.Data())) {
		if totalSize > uint32(len(pg.Data()))-PageHeaderSize {
			return 0, ErrRecordTooLarge
		}

		var newPg *page.Page
		if t.txMgr != nil && tx != nil {
			newPg, err = t.txMgr.AllocatePage(tx)
		} else {
			newPg, err = t.pager.AllocatePage()
		}
		if err != nil {
			return 0, err
		}
		t.initPageHeader(newPg)

		// Link the old page to the new page
		encoding.PutUint32(pg.Data()[8:12], uint32(newPg.ID()))
		if t.txMgr != nil && tx != nil {
			if err := t.txMgr.WritePage(tx, pg); err != nil {
				return 0, err
			}
		} else {
			if err := t.pager.WritePage(pg); err != nil {
				return 0, err
			}
		}

		t.lastPageID = newPg.ID()
		pg = newPg
		freeOffset = PageHeaderSize
	}

	// Write record
	numRecords := encoding.Uint32(pg.Data()[0:4])
	liveRecords := encoding.Uint32(pg.Data()[12:16])

	pg.Data()[freeOffset] = 0 // deleted = false
	encoding.PutUint32(pg.Data()[freeOffset+1:freeOffset+5], recordSize)
	copy(pg.Data()[freeOffset+5:freeOffset+totalSize], record)

	// Update header
	encoding.PutUint32(pg.Data()[0:4], numRecords+1)
	encoding.PutUint32(pg.Data()[4:8], freeOffset+totalSize)
	encoding.PutUint32(pg.Data()[12:16], liveRecords+1)

	if t.txMgr != nil && tx != nil {
		err = t.txMgr.WritePage(tx, pg)
	} else {
		err = t.pager.WritePage(pg)
	}
	return pg.ID(), err
}

// ReadAll iterates over all pages and records in the table and returns them.
func (t *Table) ReadAll(tx *transaction.Transaction) ([][]byte, error) {
	var records [][]byte
	currPageID := t.firstPageID

	for currPageID != NoPage {
		var pg *page.Page
		var err error
		if t.txMgr != nil && tx != nil {
			pg, err = t.txMgr.FetchPage(tx, currPageID)
		} else {
			pg, err = t.pager.FetchPage(currPageID)
		}
		if err != nil {
			return nil, err
		}

		numRecords := encoding.Uint32(pg.Data()[0:4])
		offset := uint32(PageHeaderSize)

		for i := uint32(0); i < numRecords; i++ {
			deleted := pg.Data()[offset]
			if deleted == 0 { // not tombstoned
				recordSize := encoding.Uint32(pg.Data()[offset+1 : offset+1+4])
				record := make([]byte, recordSize)
				copy(record, pg.Data()[offset+1+4 : offset+1+4+recordSize])
				records = append(records, record)
			}
			// advance past: 1 byte flag + 4 byte size + recordSize payload
			offset += 1 + 4 + encoding.Uint32(pg.Data()[offset+1 : offset+1+4])
		}

		nextID := encoding.Uint32(pg.Data()[8:12])
		currPageID = page.PageID(nextID)
	}

	return records, nil
}

// ReadAllLive is like ReadAll but skips tombstoned records.
// It is used by DELETE to find matching rows and should be the
// source of truth for live row count.
func (t *Table) ReadAllLive(tx *transaction.Transaction) ([][]byte, error) {
	var records [][]byte
	currPageID := t.firstPageID

	for currPageID != NoPage {
		var pg *page.Page
		var err error
		if t.txMgr != nil && tx != nil {
			pg, err = t.txMgr.FetchPage(tx, currPageID)
		} else {
			pg, err = t.pager.FetchPage(currPageID)
		}
		if err != nil {
			return nil, err
		}

		numRecords := encoding.Uint32(pg.Data()[0:4])
		offset := uint32(PageHeaderSize)

		for i := uint32(0); i < numRecords; i++ {
			deleted := pg.Data()[offset]
			if deleted == 0 { // not tombstoned
				recordSize := encoding.Uint32(pg.Data()[offset+1 : offset+1+4])
				record := make([]byte, recordSize)
				copy(record, pg.Data()[offset+1+4 : offset+1+4+recordSize])
				records = append(records, record)
			}
			// advance past: 1 byte flag + 4 byte size + recordSize payload
			offset += 1 + 4 + encoding.Uint32(pg.Data()[offset+1 : offset+1+4])
		}

		nextID := encoding.Uint32(pg.Data()[8:12])
		currPageID = page.PageID(nextID)
	}

	return records, nil
}

// TombstoneRecord finds the first record whose serialized payload equals target
// and marks it as deleted by setting the deleted flag to 1 and decrementing
// LiveRecordCount. Returns true if a record was tombstoned, false if none matched.
func (t *Table) TombstoneRecord(tx *transaction.Transaction, target []byte) (bool, error) {
	currPageID := t.firstPageID

	for currPageID != NoPage {
		var pg *page.Page
		var err error
		if t.txMgr != nil && tx != nil {
			pg, err = t.txMgr.FetchPage(tx, currPageID)
		} else {
			pg, err = t.pager.FetchPage(currPageID)
		}
		if err != nil {
			return false, err
		}

		numRecords := encoding.Uint32(pg.Data()[0:4])
		offset := uint32(PageHeaderSize)

		for i := uint32(0); i < numRecords; i++ {
			deleted := pg.Data()[offset]
			if deleted == 0 { // only check live records
				recordSize := encoding.Uint32(pg.Data()[offset+1 : offset+1+4])
				recordStart := offset + 1 + 4
				recordEnd := recordStart + recordSize
				if recordEnd > uint32(len(pg.Data())) {
					// malformed page; stop scanning this page
					break
				}
				record := pg.Data()[recordStart:recordEnd:recordEnd]
				if equal(record, target) {
					// Mark as deleted
					pg.Data()[offset] = 1
					// Decrement live count
					live := encoding.Uint32(pg.Data()[12:16])
					if live > 0 {
						encoding.PutUint32(pg.Data()[12:16], live-1)
					}
					// Persist the page
					if t.txMgr != nil && tx != nil {
						if err := t.txMgr.WritePage(tx, pg); err != nil {
							return false, err
						}
					} else {
						if err := t.pager.WritePage(pg); err != nil {
							return false, err
						}
					}
					return true, nil
				}
			}
			// advance past: 1 byte flag + 4 byte size + recordSize payload
			offset += 1 + 4 + encoding.Uint32(pg.Data()[offset+1 : offset+1+4])
		}

		nextID := encoding.Uint32(pg.Data()[8:12])
		currPageID = page.PageID(nextID)
	}

	return false, nil
}

// equal returns true if two byte slices are equal.
func equal(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
