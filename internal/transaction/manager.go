package transaction

import (
	"fmt"
	"sync"

	"github.com/TaqsBlaze/FlamingoDB/internal/storage/page"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/pager"
	"github.com/TaqsBlaze/FlamingoDB/internal/wal"
)

// TransactionManager coordinates starting, committing, rolling back, and recovering transactions.
type TransactionManager struct {
	pager      *pager.Pager
	wal        *wal.WAL
	activeTxns map[uint64]*Transaction
	nextTxID   uint64
	mu         sync.Mutex
	globalLock sync.RWMutex // Simple serialization lock for transaction isolation
}

// NewTransactionManager creates a new TransactionManager with a WAL at the specified path.
func NewTransactionManager(p *pager.Pager, walPath string) (*TransactionManager, error) {
	w, err := wal.Open(walPath)
	if err != nil {
		return nil, err
	}

	return &TransactionManager{
		pager:      p,
		wal:        w,
		activeTxns: make(map[uint64]*Transaction),
		nextTxID:   1,
	}, nil
}

// Close closes the transaction manager and its WAL.
func (tm *TransactionManager) Close() error {
	return tm.wal.Close()
}

// Begin starts a new transaction.
func (tm *TransactionManager) Begin() (*Transaction, error) {
	tm.globalLock.Lock() // Acquire database-level transaction lock

	tm.mu.Lock()
	defer tm.mu.Unlock()

	txID := tm.nextTxID
	tm.nextTxID++

	// Log BEGIN to WAL
	_, err := tm.wal.Append(txID, wal.Begin, 0, nil)
	if err != nil {
		tm.globalLock.Unlock()
		return nil, err
	}
	if err := tm.wal.Sync(); err != nil {
		tm.globalLock.Unlock()
		return nil, err
	}

	tx := NewTransaction(txID)
	tm.activeTxns[txID] = tx
	return tx, nil
}

// Commit commits the transaction, logs to WAL, and flushes dirty pages.
func (tm *TransactionManager) Commit(tx *Transaction) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.state != Active {
		return fmt.Errorf("transaction %d is not active", tx.id)
	}

	// 1. Log COMMIT to WAL
	_, err := tm.wal.Append(tx.id, wal.Commit, 0, nil)
	if err != nil {
		return err
	}

	// 2. Flush WAL to disk (Write-Ahead guarantee)
	if err := tm.wal.Sync(); err != nil {
		return err
	}

	// 3. Write all private dirty pages to the pager
	for _, pg := range tx.dirtyPages {
		if err := tm.pager.WritePage(pg); err != nil {
			return err
		}
	}

	// 4. Force write pages to database file
	if err := tm.pager.FlushAll(); err != nil {
		return err
	}

	tx.state = Committed
	delete(tm.activeTxns, tx.id)

	tm.globalLock.Unlock() // Release database-level transaction lock
	return nil
}

// Rollback rolls back the transaction, discarding modifications and logging to WAL.
func (tm *TransactionManager) Rollback(tx *Transaction) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.state != Active {
		return fmt.Errorf("transaction %d is not active", tx.id)
	}

	// 1. Log ABORT to WAL
	_, err := tm.wal.Append(tx.id, wal.Abort, 0, nil)
	if err != nil {
		return err
	}
	if err := tm.wal.Sync(); err != nil {
		return err
	}

	// 2. Discard dirty pages
	tx.dirtyPages = nil
	tx.state = Aborted
	delete(tm.activeTxns, tx.id)

	tm.globalLock.Unlock() // Release database-level transaction lock
	return nil
}

// FetchPage retrieves a page under transaction context.
func (tm *TransactionManager) FetchPage(tx *Transaction, id page.PageID) (*page.Page, error) {
	if tx == nil {
		return tm.pager.FetchPage(id)
	}

	tx.mu.Lock()
	defer tx.mu.Unlock()

	if pg, exists := tx.dirtyPages[id]; exists {
		// Return copy of dirty page
		cloned := page.New(pg.ID(), uint32(len(pg.Data())))
		cloned.CopyData(pg.Data())
		return cloned, nil
	}

	pg, err := tm.pager.FetchPage(id)
	if err != nil {
		return nil, err
	}

	cloned := page.New(pg.ID(), uint32(len(pg.Data())))
	cloned.CopyData(pg.Data())
	return cloned, nil
}

// WritePage writes a page under transaction context.
func (tm *TransactionManager) WritePage(tx *Transaction, pg *page.Page) error {
	if tx == nil {
		return tm.pager.WritePage(pg)
	}

	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.state != Active {
		return fmt.Errorf("transaction %d is not active", tx.id)
	}

	// Copy page data to private dirty workspace
	cloned := page.New(pg.ID(), uint32(len(pg.Data())))
	cloned.CopyData(pg.Data())
	tx.dirtyPages[pg.ID()] = cloned

	// Append update to WAL
	_, err := tm.wal.Append(tx.id, wal.Update, pg.ID(), pg.Data())
	return err
}

// AllocatePage allocates a page under transaction context.
func (tm *TransactionManager) AllocatePage(tx *Transaction) (*page.Page, error) {
	if tx == nil {
		return tm.pager.AllocatePage()
	}

	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.state != Active {
		return nil, fmt.Errorf("transaction %d is not active", tx.id)
	}

	pg, err := tm.pager.AllocatePage()
	if err != nil {
		return nil, err
	}

	cloned := page.New(pg.ID(), uint32(len(pg.Data())))
	cloned.CopyData(pg.Data())
	tx.dirtyPages[pg.ID()] = cloned

	// Append update to WAL for page allocation
	_, err = tm.wal.Append(tx.id, wal.Update, pg.ID(), pg.Data())
	if err != nil {
		return nil, err
	}

	return cloned, nil
}

// Recover scans the WAL and replays committed updates (redo).
func (tm *TransactionManager) Recover() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	records, err := tm.wal.ReadAllRecords()
	if err != nil {
		return err
	}

	// Phase 1: Scan for committed TxIDs
	committedTxns := make(map[uint64]bool)
	for _, rec := range records {
		if rec.Type == wal.Commit {
			committedTxns[rec.TxID] = true
		}
	}

	// Phase 2: Redo committed updates
	for _, rec := range records {
		if rec.Type == wal.Update && committedTxns[rec.TxID] {
			pg := page.New(rec.PageID, uint32(len(rec.Data)))
			pg.CopyData(rec.Data)
			if err := tm.pager.WritePage(pg); err != nil {
				return err
			}
		}
	}

	// Flush replayed updates to database file
	if err := tm.pager.FlushAll(); err != nil {
		return err
	}

	// Truncate WAL file and reset sequence numbers
	return tm.wal.Truncate()
}
