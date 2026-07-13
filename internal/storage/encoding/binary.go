package encoding

import (
	"encoding/binary"
	"math"
)

// Endian is the standard byte order used in FlamingoDB.
var Endian = binary.LittleEndian

// PutUint32 writes a uint32 into the buffer at the beginning.
func PutUint32(b []byte, v uint32) {
	Endian.PutUint32(b, v)
}

// Uint32 reads a uint32 from the buffer at the beginning.
func Uint32(b []byte) uint32 {
	return Endian.Uint32(b)
}

// PutUint64 writes a uint64 into the buffer at the beginning.
func PutUint64(b []byte, v uint64) {
	Endian.PutUint64(b, v)
}

// Uint64 reads a uint64 from the buffer at the beginning.
func Uint64(b []byte) uint64 {
	return Endian.Uint64(b)
}

// PutFloat64 writes a float64 into the buffer at the beginning.
func PutFloat64(b []byte, v float64) {
	Endian.PutUint64(b, math.Float64bits(v))
}

// Float64 reads a float64 from the buffer at the beginning.
func Float64(b []byte) float64 {
	return math.Float64frombits(Endian.Uint64(b))
}

// PutString writes a string to the buffer, prefixed by its uint32 length.
// It returns the number of bytes written.
func PutString(b []byte, v string) int {
	length := uint32(len(v))
	PutUint32(b, length)
	copy(b[4:], v)
	return 4 + int(length)
}

// String reads a string from the buffer, which is prefixed by its uint32 length.
// It returns the string and the number of bytes read.
func String(b []byte) (string, int) {
	length := Uint32(b)
	v := string(b[4 : 4+length])
	return v, 4 + int(length)
}
