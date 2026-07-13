package datatypes

import (
	"fmt"
)

// Complex represents a complex number with real and imaginary parts.
type Complex struct {
	Real float64
	Imag float64
}

// String returns the string representation of a Complex number.
func (c Complex) String() string {
	if c.Imag >= 0 {
		return fmt.Sprintf("%g+%gi", c.Real, c.Imag)
	}
	return fmt.Sprintf("%g%gi", c.Real, c.Imag)
}

// Equals checks if two Complex numbers are equal.
func (c Complex) Equals(other Complex) bool {
	return c.Real == other.Real && c.Imag == other.Imag
}

// Add adds two Complex numbers.
func (c Complex) Add(other Complex) Complex {
	return Complex{Real: c.Real + other.Real, Imag: c.Imag + other.Imag}
}

// Sub subtracts two Complex numbers.
func (c Complex) Sub(other Complex) Complex {
	return Complex{Real: c.Real - other.Real, Imag: c.Imag - other.Imag}
}

// Mul multiplies two Complex numbers.
func (c Complex) Mul(other Complex) Complex {
	return Complex{
		Real: c.Real*other.Real - c.Imag*other.Imag,
		Imag: c.Real*other.Imag + c.Imag*other.Real,
	}
}

// Vector represents a 1D slice of float64 numbers.
type Vector []float64

// String returns the string representation of a Vector.
func (v Vector) String() string {
	return fmt.Sprintf("%v", []float64(v))
}

// Equals checks if two Vectors are equal.
func (v Vector) Equals(other Vector) bool {
	if len(v) != len(other) {
		return false
	}
	for i := range v {
		if v[i] != other[i] {
			return false
		}
	}
	return true
}

// Add adds two Vectors of the same length.
func (v Vector) Add(other Vector) (Vector, error) {
	if len(v) != len(other) {
		return nil, fmt.Errorf("vector dimensions mismatch: %d and %d", len(v), len(other))
	}
	res := make(Vector, len(v))
	for i := range v {
		res[i] = v[i] + other[i]
	}
	return res, nil
}

// Sub subtracts two Vectors of the same length.
func (v Vector) Sub(other Vector) (Vector, error) {
	if len(v) != len(other) {
		return nil, fmt.Errorf("vector dimensions mismatch: %d and %d", len(v), len(other))
	}
	res := make(Vector, len(v))
	for i := range v {
		res[i] = v[i] - other[i]
	}
	return res, nil
}

// Dot calculates the dot product of two Vectors.
func (v Vector) Dot(other Vector) (float64, error) {
	if len(v) != len(other) {
		return 0, fmt.Errorf("vector dimensions mismatch: %d and %d", len(v), len(other))
	}
	var sum float64
	for i := range v {
		sum += v[i] * other[i]
	}
	return sum, nil
}

// Matrix represents a 2D slice of float64 numbers (row-major).
type Matrix [][]float64

// String returns the string representation of a Matrix.
func (m Matrix) String() string {
	return fmt.Sprintf("%v", [][]float64(m))
}

// Equals checks if two Matrices are equal.
func (m Matrix) Equals(other Matrix) bool {
	if len(m) != len(other) {
		return false
	}
	for i := range m {
		if len(m[i]) != len(other[i]) {
			return false
		}
		for j := range m[i] {
			if m[i][j] != other[i][j] {
				return false
			}
		}
	}
	return true
}

// Add adds two Matrices of the same dimensions.
func (m Matrix) Add(other Matrix) (Matrix, error) {
	if len(m) != len(other) {
		return nil, fmt.Errorf("matrix row counts mismatch: %d and %d", len(m), len(other))
	}
	if len(m) == 0 {
		return make(Matrix, 0), nil
	}
	if len(m[0]) != len(other[0]) {
		return nil, fmt.Errorf("matrix column counts mismatch: %d and %d", len(m[0]), len(other[0]))
	}
	res := make(Matrix, len(m))
	for i := range m {
		res[i] = make([]float64, len(m[i]))
		for j := range m[i] {
			res[i][j] = m[i][j] + other[i][j]
		}
	}
	return res, nil
}

// Mul multiplies two Matrices (standard matrix multiplication).
func (m Matrix) Mul(other Matrix) (Matrix, error) {
	if len(m) == 0 || len(other) == 0 {
		return make(Matrix, 0), nil
	}
	r1, c1 := len(m), len(m[0])
	r2, c2 := len(other), len(other[0])
	if c1 != r2 {
		return nil, fmt.Errorf("matrix multiplication dimension mismatch: cols of left (%d) must equal rows of right (%d)", c1, r2)
	}
	res := make(Matrix, r1)
	for i := 0; i < r1; i++ {
		res[i] = make([]float64, c2)
		for j := 0; j < c2; j++ {
			var sum float64
			for k := 0; k < c1; k++ {
				sum += m[i][k] * other[k][j]
			}
			res[i][j] = sum
		}
	}
	return res, nil
}

// Tensor represents an n-dimensional array.
type Tensor struct {
	Shape []int
	Data  []float64
}

// String returns the string representation of a Tensor.
func (t Tensor) String() string {
	return fmt.Sprintf("Tensor(shape=%v, data=%v)", t.Shape, t.Data)
}

// Equals checks if two Tensors are equal.
func (t Tensor) Equals(other Tensor) bool {
	if len(t.Shape) != len(other.Shape) {
		return false
	}
	for i := range t.Shape {
		if t.Shape[i] != other.Shape[i] {
			return false
		}
	}
	if len(t.Data) != len(other.Data) {
		return false
	}
	for i := range t.Data {
		if t.Data[i] != other.Data[i] {
			return false
		}
	}
	return true
}
