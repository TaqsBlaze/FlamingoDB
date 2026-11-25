package network

import (
	"encoding/json"
	"math"
	"testing"
)

func TestCalculationTools(t *testing.T) {
	for _, tc := range []struct {
		name, args, key string
		want            float64
	}{
		{"calculate_vector", `{"operation":"dot","a":[1,2,3],"b":[4,5,6]}`, "result", 32},
		{"calculate_vector", `{"operation":"norm","a":[3,4]}`, "result", 5},
		{"calculate_distance", `{"mode":"planar","a":[0,0],"b":[3,4]}`, "distance", 5},
		{"calculate_distance", `{"mode":"geographic","a":[0,0],"b":[0,1]}`, "distance", 111.1950802335329},
	} {
		got, err := calculateTool(tc.name, json.RawMessage(tc.args))
		if err != nil {
			t.Fatal(err)
		}
		n := got.(map[string]any)[tc.key].(float64)
		if math.Abs(n-tc.want) > 0.0001 {
			t.Errorf("%s: got %v, want %v", tc.args, n, tc.want)
		}
	}
	for _, args := range []string{`{"operation":"dot","a":[1],"b":[1,2]}`, `{"operation":"cross","a":[1,2],"b":[3,4]}`, `{"operation":"norm","a":[]}`} {
		if _, err := calculateTool("calculate_vector", json.RawMessage(args)); err == nil {
			t.Errorf("accepted invalid input %s", args)
		}
	}
	if _, err := calculateTool("calculate_distance", json.RawMessage(`{"mode":"geographic","a":[91,0],"b":[0,0]}`)); err == nil {
		t.Fatal("accepted invalid latitude")
	}
}
