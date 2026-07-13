package functions

import (
	"math"
	"testing"

	"flamingodb/internal/storage/record"
)

func TestScalarFunctions(t *testing.T) {
	// Test SIN
	val, err := Registry["SIN"]([]record.Value{{Type: record.Float, Flt: 0.0}})
	if err != nil {
		t.Fatalf("SIN(0) error: %v", err)
	}
	if val.Type != record.Float || val.Flt != 0.0 {
		t.Errorf("SIN(0) failed: got %v", val)
	}

	val, err = Registry["SIN"]([]record.Value{{Type: record.Float, Flt: math.Pi / 2}})
	if err != nil {
		t.Fatalf("SIN(pi/2) error: %v", err)
	}
	if math.Abs(val.Flt-1.0) > 1e-9 {
		t.Errorf("SIN(pi/2) failed: got %v", val)
	}

	// Test COS
	val, err = Registry["COS"]([]record.Value{{Type: record.Float, Flt: 0.0}})
	if err != nil {
		t.Fatalf("COS(0) error: %v", err)
	}
	if val.Type != record.Float || val.Flt != 1.0 {
		t.Errorf("COS(0) failed: got %v", val)
	}

	// Test ABS
	val, err = Registry["ABS"]([]record.Value{{Type: record.Integer, Int: -10}})
	if err != nil {
		t.Fatalf("ABS(-10) error: %v", err)
	}
	if val.Type != record.Integer || val.Int != 10 {
		t.Errorf("ABS(-10) failed: got %v", val)
	}

	val, err = Registry["ABS"]([]record.Value{{Type: record.Float, Flt: -3.14}})
	if err != nil {
		t.Fatalf("ABS(-3.14) error: %v", err)
	}
	if val.Type != record.Float || val.Flt != 3.14 {
		t.Errorf("ABS(-3.14) failed: got %v", val)
	}

	// Test POW
	val, err = Registry["POW"]([]record.Value{{Type: record.Float, Flt: 2.0}, {Type: record.Integer, Int: 3}})
	if err != nil {
		t.Fatalf("POW(2, 3) error: %v", err)
	}
	if val.Type != record.Float || val.Flt != 8.0 {
		t.Errorf("POW(2, 3) failed: got %v", val)
	}

	// Test LOG errors
	_, err = Registry["LOG"]([]record.Value{{Type: record.Float, Flt: -1.0}})
	if err == nil {
		t.Error("expected error for LOG(-1), got nil")
	}

	// Test SQRT errors
	_, err = Registry["SQRT"]([]record.Value{{Type: record.Float, Flt: -1.0}})
	if err == nil {
		t.Error("expected error for SQRT(-1), got nil")
	}
}

func TestVectorFunctions(t *testing.T) {
	// Test DOT
	val, err := Registry["DOT"]([]record.Value{
		{Type: record.Varchar, Str: "[1.0, 2.0, 3.0]"},
		{Type: record.Varchar, Str: "4.0, 5.0, 6.0"},
	})
	if err != nil {
		t.Fatalf("DOT error: %v", err)
	}
	if val.Type != record.Float || val.Flt != 32.0 {
		t.Errorf("DOT failed: got %v", val)
	}

	// Test CROSS
	val, err = Registry["CROSS"]([]record.Value{
		{Type: record.Varchar, Str: "[1.0, 0.0, 0.0]"},
		{Type: record.Varchar, Str: "[0.0, 1.0, 0.0]"},
	})
	if err != nil {
		t.Fatalf("CROSS error: %v", err)
	}
	if val.Type != record.Varchar || val.Str != "[0.000000, 0.000000, 1.000000]" {
		t.Errorf("CROSS failed: got %v", val)
	}

	// Test NORM
	val, err = Registry["NORM"]([]record.Value{
		{Type: record.Varchar, Str: "[3.0, 4.0]"},
	})
	if err != nil {
		t.Fatalf("NORM error: %v", err)
	}
	if val.Type != record.Float || val.Flt != 5.0 {
		t.Errorf("NORM failed: got %v", val)
	}
}
