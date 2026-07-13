package flamingodb_test

import (
	"path/filepath"
	"testing"

	"github.com/TaqsBlaze/FlamingoDB"
)

func TestEngineConnectAndRun(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_engine.db")

	// Connect to engine (creates if not exists)
	db, err := flamingodb.Connect(dbPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer db.Close()

	// Create table
	_, err = db.Run("CREATE TABLE research (id INT, value FLOAT, label VARCHAR);")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert data
	_, err = db.Run("INSERT INTO research VALUES (1, -4.5, 'Negative Value');")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	_, err = db.Run("INSERT INTO research VALUES (2, 100.99, 'Positive Value');")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Query with filters
	res, err := db.Run("SELECT * FROM research WHERE value < 0;")
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if len(res.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(res.Rows))
	}

	if res.Rows[0].Values[0].Int != 1 {
		t.Errorf("expected ID 1, got %d", res.Rows[0].Values[0].Int)
	}
	if res.Rows[0].Values[1].Flt != -4.5 {
		t.Errorf("expected value -4.5, got %f", res.Rows[0].Values[1].Flt)
	}
	if res.Rows[0].Values[2].Str != "Negative Value" {
		t.Errorf("expected label 'Negative Value', got %s", res.Rows[0].Values[2].Str)
	}
}
