package pager_test

import (
	"path/filepath"
	"testing"

	"flamingodb/internal/storage/disk"
	"flamingodb/internal/storage/pager"
)

func TestPager(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	pageSize := uint32(4096)

	dm, err := disk.NewDiskManager(dbPath, pageSize)
	if err != nil {
		t.Fatalf("failed to create disk manager: %v", err)
	}
	defer dm.Close()

	p, err := pager.New(dm, pageSize)
	if err != nil {
		t.Fatalf("failed to create pager: %v", err)
	}

	// Allocate page
	pg, err := p.AllocatePage()
	if err != nil {
		t.Fatalf("failed to allocate page: %v", err)
	}
	if pg.ID() != 0 {
		t.Fatalf("expected page ID 0, got %v", pg.ID())
	}

	// Write data
	pg.Data()[0] = 42
	err = p.WritePage(pg)
	if err != nil {
		t.Fatalf("failed to write page: %v", err)
	}

	// Fetch page
	pg2, err := p.FetchPage(0)
	if err != nil {
		t.Fatalf("failed to fetch page: %v", err)
	}
	if pg2.Data()[0] != 42 {
		t.Fatalf("expected 42, got %v", pg2.Data()[0])
	}

	// Fetch out of bounds
	_, err = p.FetchPage(1)
	if err != pager.ErrPageNotFound {
		t.Fatalf("expected ErrPageNotFound, got %v", err)
	}

	err = p.FlushAll()
	if err != nil {
		t.Fatalf("failed to flush all: %v", err)
	}
}
