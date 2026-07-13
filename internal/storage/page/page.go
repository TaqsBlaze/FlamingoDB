package page

// PageID uniquely identifies a page within the database.
type PageID uint32

// Page represents a fixed-size block of memory that mirrors a block on disk.
type Page struct {
	id   PageID
	data []byte
}

// New creates a new Page with the given ID and size.
func New(id PageID, size uint32) *Page {
	return &Page{
		id:   id,
		data: make([]byte, size),
	}
}

// ID returns the page's unique identifier.
func (p *Page) ID() PageID {
	return p.id
}

// Data returns the raw byte content of the page.
func (p *Page) Data() []byte {
	return p.data
}

// CopyData safely copies new data into the page.
func (p *Page) CopyData(data []byte) {
	copy(p.data, data)
}
