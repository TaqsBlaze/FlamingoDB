package record_test

import (
	"testing"

	"flamingodb/internal/datatypes"
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


func TestScientificRecordSerialization(t *testing.T) {
	schema := record.NewSchema([]record.Column{
		{Name: "c", Type: record.Complex},
		{Name: "v", Type: record.Vector},
		{Name: "m", Type: record.Matrix},
		{Name: "t", Type: record.Tensor},
	})

	rec := &record.Record{
		Values: []record.Value{
			{Type: record.Complex, Comp: datatypes.Complex{Real: 1.2, Imag: -3.4}},
			{Type: record.Vector, Vec: datatypes.Vector{1.0, 2.0, 3.0}},
			{Type: record.Matrix, Mat: datatypes.Matrix{{1.0, 2.0}, {3.0, 4.0}}},
			{Type: record.Tensor, Ten: datatypes.Tensor{Shape: []int{2, 1, 2}, Data: []float64{1.0, 2.0, 3.0, 4.0}}},
		},
	}

	data := rec.Serialize(schema)
	decoded := record.Deserialize(data, schema)

	if len(decoded.Values) != 4 {
		t.Fatalf("expected 4 values, got %d", len(decoded.Values))
	}

	// Complex
	if !decoded.Values[0].Comp.Equals(rec.Values[0].Comp) {
		t.Errorf("expected complex %v, got %v", rec.Values[0].Comp, decoded.Values[0].Comp)
	}

	// Vector
	if !decoded.Values[1].Vec.Equals(rec.Values[1].Vec) {
		t.Errorf("expected vector %v, got %v", rec.Values[1].Vec, decoded.Values[1].Vec)
	}

	// Matrix
	if !decoded.Values[2].Mat.Equals(rec.Values[2].Mat) {
		t.Errorf("expected matrix %v, got %v", rec.Values[2].Mat, decoded.Values[2].Mat)
	}

	// Tensor
	if !decoded.Values[3].Ten.Equals(rec.Values[3].Ten) {
		t.Errorf("expected tensor %v, got %v", rec.Values[3].Ten, decoded.Values[3].Ten)
	}
}

func TestGeospatialRecordSerialization(t *testing.T) {
	schema := record.NewSchema([]record.Column{
		{Name: "pt", Type: record.Point},
		{Name: "poly", Type: record.Polygon},
	})

	rec := &record.Record{
		Values: []record.Value{
			{Type: record.Point, Pt: datatypes.Point{X: 12.34, Y: 56.78}},
			{Type: record.Polygon, Poly: datatypes.Polygon{
				{X: 0, Y: 0},
				{X: 1, Y: 0},
				{X: 1, Y: 1},
				{X: 0, Y: 1},
			}},
		},
	}

	data := rec.Serialize(schema)
	decoded := record.Deserialize(data, schema)

	if len(decoded.Values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(decoded.Values))
	}

	if !decoded.Values[0].Pt.Equals(rec.Values[0].Pt) {
		t.Errorf("expected point %v, got %v", rec.Values[0].Pt, decoded.Values[0].Pt)
	}

	if !decoded.Values[1].Poly.Equals(rec.Values[1].Poly) {
		t.Errorf("expected polygon %v, got %v", rec.Values[1].Poly, decoded.Values[1].Poly)
	}
}
