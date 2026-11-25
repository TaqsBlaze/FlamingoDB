package network

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/TaqsBlaze/FlamingoDB/internal/datatypes"
)

func calculationTools() []map[string]any {
	vector := map[string]any{"type": "array", "items": map[string]any{"type": "number"}, "minItems": 1}
	point := map[string]any{"type": "array", "items": map[string]any{"type": "number"}, "minItems": 2, "maxItems": 2}
	return []map[string]any{
		{"name": "calculate_vector", "description": "Calculate vector addition, subtraction, dot product, 3D cross product, or Euclidean norm. Inputs are numeric arrays.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"operation": map[string]any{"type": "string", "enum": []string{"add", "subtract", "dot", "cross", "norm"}}, "a": vector, "b": vector}, "required": []string{"operation", "a"}}},
		{"name": "calculate_distance", "description": "Distance between two points. planar uses [x,y] and returns coordinate units; geographic uses [latitude,longitude] in decimal degrees and returns kilometers using a spherical Earth approximation.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"mode": map[string]any{"type": "string", "enum": []string{"planar", "geographic"}}, "a": point, "b": point}, "required": []string{"mode", "a", "b"}}},
	}
}

func calculateTool(name string, args json.RawMessage) (any, error) {
	var p struct {
		Operation string    `json:"operation"`
		Mode      string    `json:"mode"`
		A         []float64 `json:"a"`
		B         []float64 `json:"b"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if len(p.A) == 0 || len(p.A) > 10000 || len(p.B) > 10000 {
		return nil, fmt.Errorf("vectors must contain 1 to 10,000 numbers")
	}
	switch name {
	case "calculate_vector":
		a, b := datatypes.Vector(p.A), datatypes.Vector(p.B)
		var result any
		var err error
		switch p.Operation {
		case "add":
			result, err = a.Add(b)
		case "subtract":
			result, err = a.Sub(b)
		case "dot":
			result, err = a.Dot(b)
		case "cross":
			if len(a) != 3 || len(b) != 3 {
				return nil, fmt.Errorf("cross product requires two three-dimensional vectors")
			}
			result = []float64{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
		case "norm":
			var n float64
			for _, v := range a {
				n = math.Hypot(n, v)
			}
			result = n
		default:
			return nil, fmt.Errorf("unsupported vector operation %q", p.Operation)
		}
		if err != nil {
			return nil, err
		}
		return map[string]any{"operation": p.Operation, "result": result}, nil
	case "calculate_distance":
		if len(p.A) != 2 || len(p.B) != 2 {
			return nil, fmt.Errorf("each point must have exactly two coordinates")
		}
		switch p.Mode {
		case "planar":
			a, b := datatypes.Point{X: p.A[0], Y: p.A[1]}, datatypes.Point{X: p.B[0], Y: p.B[1]}
			return map[string]any{"mode": "planar", "distance": a.Distance(b), "unit": "coordinate units"}, nil
		case "geographic":
			if math.Abs(p.A[0]) > 90 || math.Abs(p.B[0]) > 90 || math.Abs(p.A[1]) > 180 || math.Abs(p.B[1]) > 180 {
				return nil, fmt.Errorf("latitude must be between -90 and 90 and longitude between -180 and 180 degrees")
			}
			rad := math.Pi / 180
			lat1, lat2 := p.A[0]*rad, p.B[0]*rad
			dlat, dlon := (p.B[0]-p.A[0])*rad, (p.B[1]-p.A[1])*rad
			h := math.Pow(math.Sin(dlat/2), 2) + math.Cos(lat1)*math.Cos(lat2)*math.Pow(math.Sin(dlon/2), 2)
			distance := 6371.0088 * 2 * math.Asin(math.Sqrt(math.Min(1, math.Max(0, h))))
			return map[string]any{"mode": "geographic", "distance": distance, "unit": "km", "method": "spherical haversine"}, nil
		default:
			return nil, fmt.Errorf("unsupported distance mode %q", p.Mode)
		}
	}
	return nil, fmt.Errorf("unknown calculation tool %q", name)
}
