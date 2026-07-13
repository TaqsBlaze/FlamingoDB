package pager

import (
	"errors"
	"sync"

	"github.com/TaqsBlaze/FlamingoDB/internal/storage/disk"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/page"
)

var (
	ErrPageNotFound = errors.New("page not found")
)

// Pager manages the caching and retrieval of pages from disk.
// In Phase 1, this acts as a simple buffer pool.
type Pager struct {
	diskManager *disk.DiskManager
	pages       map[page.PageID]*page.Page
	pageSize    uint32
	nextPageID  page.PageID
	mu          sync.RWMutex
}

// New creates a new Pager.
func New(diskManager *disk.DiskManager, pageSize uint32) (*Pager, error) {
	size, err := diskManager.Size()
	if err != nil {
		return nil, err
	}

	nextID := page.PageID(size / int64(pageSize))

	return &Pager{
		diskManager: diskManager,
		pages:       make(map[page.PageID]*page.Page),
		pageSize:    pageSize,
		nextPageID:  nextID,
	}, nil
}

// Filename returns the database filepath managed by the disk manager.
func (p *Pager) Filename() string {
	return p.diskManager.Filename()
}

// FetchPage retrieves a page from the cache or disk.
func (p *Pager) FetchPage(id page.PageID) (*page.Page, error) {
	p.mu.RLock()
	if id >= p.nextPageID {
		p.mu.RUnlock()
		return nil, ErrPageNotFound
	}
	pg, exists := p.pages[id]
	p.mu.RUnlock()

	if exists {
		return pg, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double check after acquiring write lock
	if pg, exists := p.pages[id]; exists {
		return pg, nil
	}

	pg = page.New(id, p.pageSize)
	err := p.diskManager.ReadPage(id, pg)
	if err != nil {
		return nil, err
	}

	p.pages[id] = pg
	return pg, nil
}

// WritePage writes a page back to disk and updates the cache.
func (p *Pager) WritePage(pg *page.Page) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	err := p.diskManager.WritePage(pg)
	if err != nil {
		return err
	}

	p.pages[pg.ID()] = pg
	return nil
}

// AllocatePage allocates a new page on disk and adds it to the cache.
func (p *Pager) AllocatePage() (*page.Page, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	id := p.nextPageID
	p.nextPageID++

	pg := page.New(id, p.pageSize)
	err := p.diskManager.WritePage(pg)
	if err != nil {
		p.nextPageID-- // rollback on error
		return nil, err
	}

	p.pages[id] = pg
	return pg, nil
}

// PageSize returns the size of pages managed by the pager.
func (p *Pager) PageSize() uint32 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pageSize
}

// FlushAll writes all cached pages to disk.
func (p *Pager) FlushAll() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, pg := range p.pages {
		if err := p.diskManager.WritePage(pg); err != nil {
			return err
		}
	}
	return nil
}
