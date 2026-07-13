package flamingodb

import (
	"fmt"
	"strings"

	"github.com/TaqsBlaze/FlamingoDB/internal/executor"
	"github.com/TaqsBlaze/FlamingoDB/internal/optimizer"
	"github.com/TaqsBlaze/FlamingoDB/internal/parser/lexer"
	"github.com/TaqsBlaze/FlamingoDB/internal/parser/parser"
	"github.com/TaqsBlaze/FlamingoDB/internal/planner"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/catalog"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/disk"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/pager"
	"github.com/TaqsBlaze/FlamingoDB/internal/transaction"
)

// Engine represents the top-level FlamingoDB database engine instance.
type Engine struct {
	diskManager  *disk.DiskManager
	pager        *pager.Pager
	tableManager *catalog.TableManager
	executor     *executor.Executor
	planner      *planner.Planner
}

// Connect initializes and connects to a FlamingoDB database file.
// If the database file does not exist, it is created.
// Uses the default page size of 4096 bytes.
func Connect(dbPath string) (*Engine, error) {
	return ConnectWithPageSize(dbPath, 4096)
}

// ConnectWithPageSize connects to a FlamingoDB database file using a custom page size.
func ConnectWithPageSize(dbPath string, pageSize uint32) (*Engine, error) {
	dm, err := disk.NewDiskManager(dbPath, pageSize)
	if err != nil {
		return nil, err
	}

	p, err := pager.New(dm, pageSize)
	if err != nil {
		_ = dm.Close()
		return nil, err
	}

	tm, err := catalog.NewTableManager(p)
	if err != nil {
		_ = dm.Close()
		return nil, err
	}

	exec := executor.New(tm)
	pl := planner.New()

	return &Engine{
		diskManager:  dm,
		pager:        p,
		tableManager: tm,
		executor:     exec,
		planner:      pl,
	}, nil
}

// Run parses, plans, optimizes, and executes a single SQL query string.
func (e *Engine) Run(sql string) (*executor.Result, error) {
	return e.RunWithTx(sql, nil)
}

// RunWithTx runs a query under the context of an explicit transaction.
func (e *Engine) RunWithTx(sql string, tx *transaction.Transaction) (*executor.Result, error) {
	l := lexer.New(sql)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("parser error: %s", strings.Join(p.Errors(), "; "))
	}
	if len(prog.Statements) == 0 {
		return nil, fmt.Errorf("no SQL statement found in query")
	}

	// Execute the first statement
	stmt := prog.Statements[0]

	node, err := e.planner.Plan(stmt)
	if err != nil {
		return nil, fmt.Errorf("planning error: %w", err)
	}

	optimized, err := optimizer.Optimize(node, e.tableManager)
	if err != nil {
		return nil, fmt.Errorf("optimization error: %w", err)
	}

	res, err := e.executor.ExecuteWithTx(tx, optimized)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// Close closes the database engine, ensuring all resources and catalog changes are flushed and synchronized.
func (e *Engine) Close() error {
	var errs []string
	if err := e.tableManager.Close(); err != nil {
		errs = append(errs, fmt.Sprintf("table manager close: %v", err))
	}
	if err := e.diskManager.Close(); err != nil {
		errs = append(errs, fmt.Sprintf("disk manager close: %v", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing engine: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Executor returns the physical execution engine.
func (e *Engine) Executor() *executor.Executor {
	return e.executor
}

// TableManager returns the catalog and table storage manager.
func (e *Engine) TableManager() *catalog.TableManager {
	return e.tableManager
}

// Pager returns the buffer pool manager.
func (e *Engine) Pager() *pager.Pager {
	return e.pager
}

// DiskManager returns the raw disk manager.
func (e *Engine) DiskManager() *disk.DiskManager {
	return e.diskManager
}
