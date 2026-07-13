package transaction

import (
	"sync"

	"github.com/TaqsBlaze/FlamingoDB/internal/storage/page"
)

// TxState defines the lifecycle state of a transaction.
type TxState int

const (
	Active TxState = iota
	Committed
	Aborted
)

// Transaction represents a single transactional context.
type Transaction struct {
	id         uint64
	state      TxState
	dirtyPages map[page.PageID]*page.Page
	mu         sync.Mutex
}

// NewTransaction creates a new transaction.
func NewTransaction(id uint64) *Transaction {
	return &Transaction{
		id:         id,
		state:      Active,
		dirtyPages: make(map[page.PageID]*page.Page),
	}
}

// ID returns the unique transaction identifier.
func (tx *Transaction) ID() uint64 {
	return tx.id
}

// State returns the current state of the transaction.
func (tx *Transaction) State() TxState {
	return tx.state
}
