package disk

import (
	"errors"
	"io"
	"os"
	"sync"

	"flamingodb/internal/storage/page"
)

var (
	ErrPageNotFound = errors.New("page not found")
	ErrInvalidPage  = errors.New("invalid page size")
)

// DiskManager is responsible for reading and writing pages to the database file.
// It is safe for concurrent use.
type DiskManager struct {
	file     *os.File
	filename string
	pageSize uint32
	mu       sync.Mutex
}

// NewDiskManager creates a new DiskManager and opens the specified file.
func NewDiskManager(filename string, pageSize uint32) (*DiskManager, error) {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}

	return &DiskManager{
		file:     file,
		filename: filename,
		pageSize: pageSize,
	}, nil
}

// Close closes the database file.
func (dm *DiskManager) Close() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	return dm.file.Close()
}

// Filename returns the database filepath.
func (dm *DiskManager) Filename() string {
	return dm.filename
}

// ReadPage reads a page from the database file into the provided Page object.
func (dm *DiskManager) ReadPage(pageID page.PageID, p *page.Page) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	offset := int64(pageID) * int64(dm.pageSize)

	data := p.Data()
	if uint32(len(data)) != dm.pageSize {
		return ErrInvalidPage
	}

	_, err := dm.file.ReadAt(data, offset)
	if err != nil {
		if errors.Is(err, io.EOF) {
			// Zero out the page if reading past EOF
			for i := range data {
				data[i] = 0
			}
			return nil
		}
		return err
	}

	return nil
}

// WritePage writes a page to the database file.
func (dm *DiskManager) WritePage(p *page.Page) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	offset := int64(p.ID()) * int64(dm.pageSize)

	data := p.Data()
	if uint32(len(data)) != dm.pageSize {
		return ErrInvalidPage
	}

	_, err := dm.file.WriteAt(data, offset)
	if err != nil {
		return err
	}

	return dm.file.Sync()
}

// Size returns the total size of the database file in bytes.
func (dm *DiskManager) Size() (int64, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	stat, err := dm.file.Stat()
	if err != nil {
		return 0, err
	}
	return stat.Size(), nil
}
