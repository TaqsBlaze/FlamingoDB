package tests

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/TaqsBlaze/FlamingoDB/internal/parser/ast"
	"github.com/TaqsBlaze/FlamingoDB/internal/parser/lexer"
	"github.com/TaqsBlaze/FlamingoDB/internal/parser/parser"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/catalog"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/disk"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/pager"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/record"
)

func TestEndToEndIntegration(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "integration.db")
	pageSize := uint32(4096)

	// 1. Initialize Storage Engine
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

	// 2. Define SQL commands
	sqlStatements := []string{
		"CREATE TABLE items (id INT, price FLOAT, name VARCHAR);",
		"INSERT INTO items VALUES (101, 19.99, 'Scientific Calculator');",
		"INSERT INTO items VALUES (102, 149.50, 'Tensor Matrix Processor');",
	}

	// 3. Process SQL, generate AST, and execute on Storage Engine
	for _, sql := range sqlStatements {
		l := lexer.New(sql)
		p := parser.New(l)
		prog := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("parser errors for %q: %v", sql, p.Errors())
		}

		if len(prog.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d for %q", len(prog.Statements), sql)
		}

		stmt := prog.Statements[0]
		switch s := stmt.(type) {
		case *ast.CreateTableStatement:
			var cols []record.Column
			for _, colDef := range s.Columns {
				var colType record.TypeID
				switch strings.ToUpper(colDef.Type) {
				case "INT":
					colType = record.Integer
				case "FLOAT":
					colType = record.Float
				case "VARCHAR":
					colType = record.Varchar
				default:
					t.Fatalf("unsupported column type: %s", colDef.Type)
				}
				cols = append(cols, record.Column{
					Name: colDef.Name,
					Type: colType,
				})
			}
			schema := record.NewSchema(cols)
			if err := tm.CreateTable(nil, s.Table, schema); err != nil {
				t.Fatalf("failed to create table %s: %v", s.Table, err)
			}

		case *ast.InsertStatement:
			var values []record.Value
			for _, expr := range s.Values {
				switch val := expr.(type) {
				case *ast.IntegerLiteral:
					values = append(values, record.Value{
						Type: record.Integer,
						Int:  int32(val.Value),
					})
				case *ast.FloatLiteral:
					values = append(values, record.Value{
						Type: record.Float,
						Flt:  val.Value,
					})
				case *ast.StringLiteral:
					values = append(values, record.Value{
						Type: record.Varchar,
						Str:  val.Value,
					})
				default:
					t.Fatalf("unsupported value expression type: %T", expr)
				}
			}
			rec := &record.Record{Values: values}
			if err := tm.InsertRecord(nil, s.Table, rec); err != nil {
				t.Fatalf("failed to insert record into %s: %v", s.Table, err)
			}
		default:
			t.Fatalf("unsupported AST statement type: %T", stmt)
		}
	}

	// 4. Retrieve records and verify
	records, err := tm.ReadRecords(nil, "items")
	if err != nil {
		t.Fatalf("failed to read records: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	// Assertions for item 1
	if records[0].Values[0].Int != 101 {
		t.Errorf("item 1 id mismatch: expected 101, got %d", records[0].Values[0].Int)
	}
	if records[0].Values[1].Flt != 19.99 {
		t.Errorf("item 1 price mismatch: expected 19.99, got %f", records[0].Values[1].Flt)
	}
	if records[0].Values[2].Str != "Scientific Calculator" {
		t.Errorf("item 1 name mismatch: expected 'Scientific Calculator', got %s", records[0].Values[2].Str)
	}

	// Assertions for item 2
	if records[1].Values[0].Int != 102 {
		t.Errorf("item 2 id mismatch: expected 102, got %d", records[1].Values[0].Int)
	}
	if records[1].Values[1].Flt != 149.50 {
		t.Errorf("item 2 price mismatch: expected 149.50, got %f", records[1].Values[1].Flt)
	}
	if records[1].Values[2].Str != "Tensor Matrix Processor" {
		t.Errorf("item 2 name mismatch: expected 'Tensor Matrix Processor', got %s", records[1].Values[2].Str)
	}
}
