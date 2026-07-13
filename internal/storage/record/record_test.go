package record_test

import (
	"testing"

	"flamingodb/internal/storage/record"
)

func TestRecordSerialization(t *testing.T) {
	schema := record.NewSchema([]record.Column{
		{Name: "id", Type: record.Integer},
		{Name: "temperature", Type: record.Float},
		{Name: "station", Type: record.Varchar},
	})

	rec := &record.Record{
		Values: []record.Value{
			{Type: record.Integer, Int: 42},
			{Type: record.Float, Flt: 98.6},
			{Type: record.Varchar, Str: "Station A"},
		},
	}

	data := rec.Serialize(schema)
	decoded := record.Deserialize(data, schema)

	if len(decoded.Values) != 3 {
		t.Fatalf("expected 3 values, got %d", len(decoded.Values))
	}

	if decoded.Values[0].Int != 42 {
		t.Errorf("expected id 42, got %d", decoded.Values[0].Int)
	}
	if decoded.Values[1].Flt != 98.6 {
		t.Errorf("expected temp 98.6, got %f", decoded.Values[1].Flt)
	}
	if decoded.Values[2].Str != "Station A" {
		t.Errorf("expected station 'Station A', got '%s'", decoded.Values[2].Str)
	}
}
