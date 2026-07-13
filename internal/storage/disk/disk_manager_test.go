package disk_test

import (
	"path/filepath"
	"testing"

	"flamingodb/internal/storage/disk"
	"flamingodb/internal/storage/page"
)

func TestDiskManager(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	pageSize := uint32(4096)

	dm, err := disk.NewDiskManager(dbPath, pageSize)
	if err != nil {
		t.Fatalf("failed to create disk manager: %v", err)
	}
	defer dm.Close()

	pid := page.PageID(0)

	// Test write and read
	p1 := page.New(pid, pageSize)
	p1.Data()[0] = 42
	p1.Data()[4095] = 99

	err = dm.WritePage(p1)
	if err != nil {
		t.Fatalf("failed to write page: %v", err)
	}

	p2 := page.New(pid, pageSize)
	err = dm.ReadPage(pid, p2)
	if err != nil {
		t.Fatalf("failed to read page: %v", err)
	}

	if p2.Data()[0] != 42 || p2.Data()[4095] != 99 {
		t.Fatalf("page data mismatch after read")
	}

	size, err := dm.Size()
	if err != nil {
		t.Fatalf("failed to get size: %v", err)
	}
	if size != int64(pageSize) {
		t.Fatalf("expected size %v, got %v", pageSize, size)
	}
}
