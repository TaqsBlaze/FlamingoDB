package encoding_test

import (
	"testing"

	"github.com/TaqsBlaze/FlamingoDB/internal/storage/encoding"
)

func TestBinaryEncoding(t *testing.T) {
	b := make([]byte, 1024)

	// Test Uint32
	encoding.PutUint32(b, 42)
	if v := encoding.Uint32(b); v != 42 {
		t.Fatalf("expected 42, got %v", v)
	}

	// Test Uint64
	encoding.PutUint64(b, 9999999999)
	if v := encoding.Uint64(b); v != 9999999999 {
		t.Fatalf("expected 9999999999, got %v", v)
	}

	// Test Float64
	encoding.PutFloat64(b, 3.14159)
	if v := encoding.Float64(b); v != 3.14159 {
		t.Fatalf("expected 3.14159, got %v", v)
	}

	// Test String
	str := "flamingodb is fast"
	n := encoding.PutString(b, str)
	if n != 4+len(str) {
		t.Fatalf("expected written bytes %v, got %v", 4+len(str), n)
	}

	v, n2 := encoding.String(b)
	if v != str {
		t.Fatalf("expected %v, got %v", str, v)
	}
	if n2 != n {
		t.Fatalf("expected read bytes %v, got %v", n, n2)
	}
}
