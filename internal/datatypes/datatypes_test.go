package datatypes

import (
	"testing"
)

func TestComplex(t *testing.T) {
	c1 := Complex{Real: 1, Imag: 2}
	c2 := Complex{Real: 3, Imag: 4}

	// Add
	add := c1.Add(c2)
	if !add.Equals(Complex{Real: 4, Imag: 6}) {
		t.Errorf("expected 4+6i, got %v", add)
	}

	// Sub
	sub := c1.Sub(c2)
	if !sub.Equals(Complex{Real: -2, Imag: -2}) {
		t.Errorf("expected -2-2i, got %v", sub)
	}

	// Mul
	mul := c1.Mul(c2)
	if !mul.Equals(Complex{Real: -5, Imag: 10}) {
		t.Errorf("expected -5+10i, got %v", mul)
	}

	// String
	if c1.String() != "1+2i" {
		t.Errorf("expected '1+2i', got '%s'", c1.String())
	}
	c3 := Complex{Real: 1, Imag: -2}
	if c3.String() != "1-2i" {
		t.Errorf("expected '1-2i', got '%s'", c3.String())
	}
}

func TestVector(t *testing.T) {
	v1 := Vector{1, 2, 3}
	v2 := Vector{4, 5, 6}

	// Add
	add, err := v1.Add(v2)
	if err != nil {
		t.Fatalf("Vector Add failed: %v", err)
	}
	if !add.Equals(Vector{5, 7, 9}) {
		t.Errorf("expected [5 7 9], got %v", add)
	}

	// Add Dimension Mismatch
	_, err = v1.Add(Vector{1, 2})
	if err == nil {
		t.Error("expected dimension mismatch error")
	}

	// Sub
	sub, err := v1.Sub(v2)
	if err != nil {
		t.Fatalf("Vector Sub failed: %v", err)
	}
	if !sub.Equals(Vector{-3, -3, -3}) {
		t.Errorf("expected [-3 -3 -3], got %v", sub)
	}

	// Dot
	dot, err := v1.Dot(v2)
	if err != nil {
		t.Fatalf("Vector Dot failed: %v", err)
	}
	if dot != 32 {
		t.Errorf("expected 32, got %f", dot)
	}

	// String
	if v1.String() != "[1 2 3]" {
		t.Errorf("expected '[1 2 3]', got '%s'", v1.String())
	}
}

func TestMatrix(t *testing.T) {
	m1 := Matrix{{1, 2}, {3, 4}}
	m2 := Matrix{{5, 6}, {7, 8}}

	// Add
	add, err := m1.Add(m2)
	if err != nil {
		t.Fatalf("Matrix Add failed: %v", err)
	}
	if !add.Equals(Matrix{{6, 8}, {10, 12}}) {
		t.Errorf("expected {{6, 8}, {10, 12}}, got %v", add)
	}

	// Mul
	mul, err := m1.Mul(m2)
	if err != nil {
		t.Fatalf("Matrix Mul failed: %v", err)
	}
	if !mul.Equals(Matrix{{19, 22}, {43, 50}}) {
		t.Errorf("expected {{19, 22}, {43, 50}}, got %v", mul)
	}

	// String
	if m1.String() != "[[1 2] [3 4]]" {
		t.Errorf("expected '[[1 2] [3 4]]', got '%s'", m1.String())
	}
}

func TestTensor(t *testing.T) {
	t1 := Tensor{Shape: []int{2, 2}, Data: []float64{1, 2, 3, 4}}
	t2 := Tensor{Shape: []int{2, 2}, Data: []float64{1, 2, 3, 4}}
	t3 := Tensor{Shape: []int{2, 2}, Data: []float64{1, 2, 3, 5}}

	if !t1.Equals(t2) {
		t.Error("expected t1 equals t2")
	}
	if t1.Equals(t3) {
		t.Error("expected t1 not equals t3")
	}

	// String
	if t1.String() != "Tensor(shape=[2 2], data=[1 2 3 4])" {
		t.Errorf("got '%s'", t1.String())
	}
}
