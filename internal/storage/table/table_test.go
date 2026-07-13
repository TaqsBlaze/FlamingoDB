package table_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"flamingodb/internal/storage/disk"
	"flamingodb/internal/storage/pager"
	"flamingodb/internal/storage/table"
)

func TestTable(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	pageSize := uint32(128) // Small page size to force page allocation

	dm, err := disk.NewDiskManager(dbPath, pageSize)
	if err != nil {
		t.Fatalf("failed to create disk manager: %v", err)
	}
	defer dm.Close()

	p, err := pager.New(dm, pageSize)
	if err != nil {
		t.Fatalf("failed to create pager: %v", err)
	}

	tbl, err := table.New(p, 0, true)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert some records
	recordsToInsert := [][]byte{
		[]byte("hello"),
		[]byte("world"),
		[]byte("this is a longer record that will force a new page allocation"),
		[]byte("short"),
	}

	for _, rec := range recordsToInsert {
		_, err := tbl.InsertRecord(rec)
		if err != nil {
			t.Fatalf("failed to insert record: %v", err)
		}
	}

	// Read all records
	records, err := tbl.ReadAll()
	if err != nil {
		t.Fatalf("failed to read all records: %v", err)
	}

	if len(records) != len(recordsToInsert) {
		t.Fatalf("expected %d records, got %d", len(recordsToInsert), len(records))
	}

	for i, rec := range recordsToInsert {
		if !bytes.Equal(rec, records[i]) {
			t.Fatalf("record %d mismatch: expected %s, got %s", i, rec, records[i])
		}
	}
}
