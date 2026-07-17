package record

import (
	"github.com/TaqsBlaze/FlamingoDB/internal/datatypes"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/encoding"
)

// TypeID identifies the basic data types for columns.
// More advanced scientific types will be added in Phase 9 by Agent Beta.
type TypeID uint8

const (
	Integer TypeID = iota // int32
	Float                 // float64
	Varchar               // length-prefixed string
	Complex               // complex128
	Vector                // length-prefixed float64 slice
	Matrix                // matrix (row major)
	Tensor                // tensor
	Point                 // 2D Point
	Polygon               // 2D Polygon
)

// Column defines a single column in a schema.
type Column struct {
	Name          string
	Type          TypeID
	AutoIncrement bool
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
	Comp datatypes.Complex
	Vec  datatypes.Vector
	Mat  datatypes.Matrix
	Ten  datatypes.Tensor
	Pt   datatypes.Point
	Poly datatypes.Polygon
}

// Record represents a single row in a table.
type Record struct {
	Values []Value
}

// Serialize encodes a Record into a byte slice according to its Schema.
func (r *Record) Serialize(schema *Schema) []byte {
	size := 0
	for i, col := range schema.Columns {
		val := r.Values[i]
		switch col.Type {
		case Integer:
			size += 4
		case Float:
			size += 8
		case Varchar:
			size += 4 + len(val.Str)
		case Complex:
			size += 16
		case Vector:
			size += 4 + len(val.Vec)*8
		case Matrix:
			rows := len(val.Mat)
			cols := 0
			if rows > 0 {
				cols = len(val.Mat[0])
			}
			size += 8 + rows*cols*8
		case Tensor:
			size += 4 + len(val.Ten.Shape)*4 + 4 + len(val.Ten.Data)*8
		case Point:
			size += 16
		case Polygon:
			size += 4 + len(val.Poly)*16
		}
	}

	buf := make([]byte, size)
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
		case Complex:
			encoding.PutFloat64(buf[offset:], val.Comp.Real)
			encoding.PutFloat64(buf[offset+8:], val.Comp.Imag)
			offset += 16
		case Vector:
			encoding.PutUint32(buf[offset:], uint32(len(val.Vec)))
			offset += 4
			for _, v := range val.Vec {
				encoding.PutFloat64(buf[offset:], v)
				offset += 8
			}
		case Matrix:
			rows := len(val.Mat)
			cols := 0
			if rows > 0 {
				cols = len(val.Mat[0])
			}
			encoding.PutUint32(buf[offset:], uint32(rows))
			encoding.PutUint32(buf[offset+4:], uint32(cols))
			offset += 8
			for r := 0; r < rows; r++ {
				for c := 0; c < cols; c++ {
					encoding.PutFloat64(buf[offset:], val.Mat[r][c])
					offset += 8
				}
			}
		case Tensor:
			encoding.PutUint32(buf[offset:], uint32(len(val.Ten.Shape)))
			offset += 4
			for _, dim := range val.Ten.Shape {
				encoding.PutUint32(buf[offset:], uint32(dim))
				offset += 4
			}
			encoding.PutUint32(buf[offset:], uint32(len(val.Ten.Data)))
			offset += 4
			for _, d := range val.Ten.Data {
				encoding.PutFloat64(buf[offset:], d)
				offset += 8
			}
		case Point:
			encoding.PutFloat64(buf[offset:], val.Pt.X)
			encoding.PutFloat64(buf[offset+8:], val.Pt.Y)
			offset += 16
		case Polygon:
			encoding.PutUint32(buf[offset:], uint32(len(val.Poly)))
			offset += 4
			for _, pt := range val.Poly {
				encoding.PutFloat64(buf[offset:], pt.X)
				encoding.PutFloat64(buf[offset+8:], pt.Y)
				offset += 16
			}
		}
	}
	return buf
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
		case Complex:
			realPart := encoding.Float64(data[offset:])
			imagPart := encoding.Float64(data[offset+8:])
			vals[i] = Value{Type: Complex, Comp: datatypes.Complex{Real: realPart, Imag: imagPart}}
			offset += 16
		case Vector:
			length := int(encoding.Uint32(data[offset:]))
			offset += 4
			vec := make(datatypes.Vector, length)
			for idx := 0; idx < length; idx++ {
				vec[idx] = encoding.Float64(data[offset:])
				offset += 8
			}
			vals[i] = Value{Type: Vector, Vec: vec}
		case Matrix:
			rows := int(encoding.Uint32(data[offset:]))
			cols := int(encoding.Uint32(data[offset+4:]))
			offset += 8
			mat := make(datatypes.Matrix, rows)
			for r := 0; r < rows; r++ {
				mat[r] = make([]float64, cols)
				for c := 0; c < cols; c++ {
					mat[r][c] = encoding.Float64(data[offset:])
					offset += 8
				}
			}
			vals[i] = Value{Type: Matrix, Mat: mat}
		case Tensor:
			shapeLen := int(encoding.Uint32(data[offset:]))
			offset += 4
			shape := make([]int, shapeLen)
			for idx := 0; idx < shapeLen; idx++ {
				shape[idx] = int(encoding.Uint32(data[offset:]))
				offset += 4
			}
			dataLen := int(encoding.Uint32(data[offset:]))
			offset += 4
			tdata := make([]float64, dataLen)
			for idx := 0; idx < dataLen; idx++ {
				tdata[idx] = encoding.Float64(data[offset:])
				offset += 8
			}
			vals[i] = Value{Type: Tensor, Ten: datatypes.Tensor{Shape: shape, Data: tdata}}
		case Point:
			x := encoding.Float64(data[offset:])
			y := encoding.Float64(data[offset+8:])
			vals[i] = Value{Type: Point, Pt: datatypes.Point{X: x, Y: y}}
			offset += 16
		case Polygon:
			length := int(encoding.Uint32(data[offset:]))
			offset += 4
			poly := make(datatypes.Polygon, length)
			for idx := 0; idx < length; idx++ {
				x := encoding.Float64(data[offset:])
				y := encoding.Float64(data[offset+8:])
				poly[idx] = datatypes.Point{X: x, Y: y}
				offset += 16
			}
			vals[i] = Value{Type: Polygon, Poly: poly}
		}
	}

	return &Record{Values: vals}
}
