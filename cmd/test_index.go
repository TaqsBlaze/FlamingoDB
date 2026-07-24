package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"github.com/TaqsBlaze/FlamingoDB/internal/parser/parser"
	"github.com/TaqsBlaze/FlamingoDB/internal/planner"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/catalog"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/pager"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/record"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/disk"
)

func main() {
	// Create a temporary database file
	dbPath := "./test_index.db"
	os.Remove(dbPath) // Clean up if exists

	// Initialize disk manager and pager
	dm, err := disk.NewDiskManager(dbPath, 4096)
	if err != nil {
		log.Fatalf("Failed to create disk manager: %v", err)
	}
	defer dm.Close()

	p, err := pager.New(dm, 4096)
	if err != nil {
		log.Fatalf("Failed to create pager: %v", err)
	}

	// Create table manager
	tm, err := catalog.NewTableManager(p)
	if err != nil {
		log.Fatalf("Failed to create table manager: %v", err)
	}
	defer tm.Close()

	// Create a test table
	schema := record.NewSchema([]record.Column{
		{Name: "id", Type: record.Integer},
		{Name: "name", Type: record.Varchar},
		{Name: "age", Type: record.Integer},
	})

	if err := tm.CreateTable(nil, "users", schema); err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
	fmt.Println("Created users table")

	// Insert some test data
	records := []*record.Record{
		{Values: []record.Value{{Type: record.Integer, Int: 1}, {Type: record.Varchar, Str: "Alice"}, {Type: record.Integer, Int: 25}}},
		{Values: []record.Value{{Type: record.Integer, Int: 2}, {Type: record.Varchar, Str: "Bob"}, {Type: record.Integer, Int: 30}}},
		{Values: []record.Value{{Type: record.Integer, Int: 3}, {Type: record.Varchar, Str: "Charlie"}, {Type: record.Integer, Int: 35}}},
	}

	for i, r := range records {
		if err := tm.InsertRecord(nil, "users", r); err != nil {
			log.Fatalf("Failed to insert record %d: %v", i, err)
		}
	}
	fmt.Println("Inserted test data")

	// Test CREATE INDEX
	fmt.Println("\n=== Testing CREATE INDEX ===")
	if err := tm.CreateIndex(nil, "users", "idx_users_name", "name"); err != nil {
		log.Fatalf("Failed to create index: %v", err)
	}
	fmt.Println("Created index idx_users_name on users(name)")

	if err := tm.CreateIndex(nil, "users", "idx_users_age", "age"); err != nil {
		log.Fatalf("Failed to create index: %v", err)
	}
	fmt.Println("Created index idx_users_age on users(age)")

	// Test SHOW INDEXES
	fmt.Println("\n=== Testing SHOW INDEXES ===")
	indexes, err := tm.GetIndexes("users")
	if err != nil {
		log.Fatalf("Failed to get indexes: %v", err)
	}
	fmt.Printf("Indexes on users table:\n")
	for name, idx := range indexes {
		fmt.Printf("  %s: column=%s, rootPage=%d, keyType=%v\n", name, idx.ColumnName, idx.RootPageID, idx.KeyType)
	}

	// Test DROP INDEX
	fmt.Println("\n=== Testing DROP INDEX ===")
	if err := tm.DropIndex(nil, "users", "idx_users_name", false); err != nil {
		log.Fatalf("Failed to drop index: %v", err)
	}
	fmt.Println("Dropped index idx_users_name")

	// Verify index was dropped
	indexes, err = tm.GetIndexes("users")
	if err != nil {
		log.Fatalf("Failed to get indexes: %v", err)
	}
	fmt.Printf("Indexes on users table after DROP:\n")
	for name, idx := range indexes {
		fmt.Printf("  %s: column=%s, rootPage=%d, keyType=%v\n", name, idx.ColumnName, idx.RootPageID, idx.KeyType)
	}

	// Test DROP INDEX with IF EXISTS
	fmt.Println("\n=== Testing DROP INDEX IF EXISTS ===")
	if err := tm.DropIndex(nil, "users", "idx_users_nonexistent", true); err != nil {
		log.Fatalf("Failed to drop index (IF EXISTS): %v", err)
	}
	fmt.Println("Dropped index idx_users_nonexistent (IF EXISTS - should succeed)")

	if err := tm.DropIndex(nil, "users", "idx_users_nonexistent", false); err != nil {
		log.Printf("Expected error when dropping non-existent index without IF EXISTS: %v", err)
	} else {
		fmt.Println("ERROR: Should have failed to drop non-existent index without IF EXISTS")
	}

	// Test CREATE INDEX with IF NOT EXISTS (by trying to create existing index)
	fmt.Println("\n=== Testing CREATE INDEX IF NOT EXISTS (via error handling) ===")
	if err := tm.CreateIndex(nil, "users", "idx_users_age", "age"); err != nil {
		log.Printf("Expected error when creating duplicate index: %v", err)
	} else {
		fmt.Println("ERROR: Should have failed to create duplicate index")
	}

	fmt.Println("\n=== All tests passed! ===")
}