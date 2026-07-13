package record

import (
	"flamingodb/internal/storage/encoding"
)

// TypeID identifies the basic data types for columns.
// More advanced scientific types will be added in Phase 9 by Agent Beta.
type TypeID uint8

const (
	Integer TypeID = iota // int32
	Float                 // float64
	Varchar               // length-prefixed string
)

// Column defines a single column in a schema.
type Column struct {
	Name string
	Type TypeID
}

// Schema defines the structure of a table.
type Schema struct {
	Columns []Column
}

// NewSchema creates a new schema.
func NewSchema(columns []Column) *Schema {
	return &Schema{Columns: columns}
}

// Value represents a single field in a record.
type Value struct {
	Type TypeID
	Int  int32
	Flt  float64
	Str  string
}

// Record represents a single row in a table.
type Record struct {
	Values []Value
}

// Serialize encodes a Record into a byte slice according to its Schema.
func (r *Record) Serialize(schema *Schema) []byte {
	buf := make([]byte, 1024) // pre-allocate
	offset := 0

	for i, col := range schema.Columns {
		val := r.Values[i]
		switch col.Type {
		case Integer:
			encoding.PutUint32(buf[offset:], uint32(val.Int))
			offset += 4
		case Float:
			encoding.PutFloat64(buf[offset:], val.Flt)
			offset += 8
		case Varchar:
			n := encoding.PutString(buf[offset:], val.Str)
			offset += n
		}
	}
	return buf[:offset]
}

// Deserialize decodes a byte slice into a Record according to a Schema.
func Deserialize(data []byte, schema *Schema) *Record {
	vals := make([]Value, len(schema.Columns))
	offset := 0

	for i, col := range schema.Columns {
		switch col.Type {
		case Integer:
			vals[i] = Value{Type: Integer, Int: int32(encoding.Uint32(data[offset:]))}
			offset += 4
		case Float:
			vals[i] = Value{Type: Float, Flt: encoding.Float64(data[offset:])}
			offset += 8
		case Varchar:
			str, n := encoding.String(data[offset:])
			vals[i] = Value{Type: Varchar, Str: str}
			offset += n
		}
	}

	return &Record{Values: vals}
}
