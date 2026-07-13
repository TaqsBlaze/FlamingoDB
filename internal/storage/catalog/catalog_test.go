package catalog_test

import (
	"path/filepath"
	"testing"

	"flamingodb/internal/storage/catalog"
	"flamingodb/internal/storage/disk"
	"flamingodb/internal/storage/pager"
	"flamingodb/internal/storage/record"
)

func TestTableManager(t *testing.T) {
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

	tm, err := catalog.NewTableManager(p)
	if err != nil {
		t.Fatalf("failed to create table manager: %v", err)
	}
	defer tm.Close()

	// 1. Create a table
	schema := record.NewSchema([]record.Column{
		{Name: "id", Type: record.Integer},
		{Name: "name", Type: record.Varchar},
		{Name: "rating", Type: record.Float},
	})

	err = tm.CreateTable(nil, "users", schema)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// 2. Insert records
	r1 := &record.Record{
		Values: []record.Value{
			{Type: record.Integer, Int: 1},
			{Type: record.Varchar, Str: "Alice"},
			{Type: record.Float, Flt: 4.8},
		},
	}
	r2 := &record.Record{
		Values: []record.Value{
			{Type: record.Integer, Int: 2},
			{Type: record.Varchar, Str: "Bob"},
			{Type: record.Float, Flt: 3.5},
		},
	}

	if err := tm.InsertRecord(nil, "users", r1); err != nil {
		t.Fatalf("failed to insert r1: %v", err)
	}
	if err := tm.InsertRecord(nil, "users", r2); err != nil {
		t.Fatalf("failed to insert r2: %v", err)
	}

	// 3. Read records and verify
	records, err := tm.ReadRecords(nil, "users")
	if err != nil {
		t.Fatalf("failed to read records: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	if records[0].Values[0].Int != 1 || records[0].Values[1].Str != "Alice" || records[0].Values[2].Flt != 4.8 {
		t.Errorf("r1 mismatch: %v", records[0].Values)
	}
	if records[1].Values[0].Int != 2 || records[1].Values[1].Str != "Bob" || records[1].Values[2].Flt != 3.5 {
		t.Errorf("r2 mismatch: %v", records[1].Values)
	}

	// 4. Close and reload from disk to verify persistence of Catalog
	p.FlushAll()
	tm.Close()
	dm.Close()

	dm2, err := disk.NewDiskManager(dbPath, pageSize)
	if err != nil {
		t.Fatalf("failed to reopen disk manager: %v", err)
	}
	defer dm2.Close()

	p2, err := pager.New(dm2, pageSize)
	if err != nil {
		t.Fatalf("failed to recreate pager: %v", err)
	}

	tm2, err := catalog.NewTableManager(p2)
	if err != nil {
		t.Fatalf("failed to recreate table manager: %v", err)
	}
	defer tm2.Close()

	records2, err := tm2.ReadRecords(nil, "users")
	if err != nil {
		t.Fatalf("failed to read records after reload: %v", err)
	}

	if len(records2) != 2 {
		t.Fatalf("expected 2 records after reload, got %d", len(records2))
	}

	if records2[0].Values[1].Str != "Alice" {
		t.Errorf("r1 name mismatch after reload: %s", records2[0].Values[1].Str)
	}
}
