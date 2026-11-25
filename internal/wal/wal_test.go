package wal

import (
	"io"
	"path/filepath"
	"testing"
)

func TestAppendAfterReadAndTruncate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.wal")
	w, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if _, err := w.Append(1, Begin, 0, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := w.ReadAllRecords(); err != nil {
		t.Fatal(err)
	}
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Append(1, Commit, 0, nil); err != nil {
		t.Fatal(err)
	}
	records, err := w.ReadAllRecords()
	if err != nil || len(records) != 2 || records[0].Type != Begin || records[1].Type != Commit {
		t.Fatalf("records after append: %v, error: %v", records, err)
	}
	if err := w.Truncate(); err != nil {
		t.Fatalf("truncate WAL: %v", err)
	}
	if _, err := w.Append(2, Begin, 0, nil); err != nil {
		t.Fatal(err)
	}
	records, err = w.ReadAllRecords()
	if err != nil || len(records) != 1 || records[0].TxID != 2 {
		t.Fatalf("records after truncate: %v, error: %v", records, err)
	}
}
