package functions

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"flamingodb/internal/storage/record"
)

// Function represents a native function that can be executed.
type Function func(args []record.Value) (record.Value, error)

// Registry maps function names (in uppercase) to their implementation.
var Registry = map[string]Function{
	"SIN":   evalSin,
	"COS":   evalCos,
	"TAN":   evalTan,
	"ASIN":  evalAsin,
	"ACOS":  evalAcos,
	"ATAN":  evalAtan,
	"EXP":   evalExp,
	"LOG":   evalLog,
	"LN":    evalLog,
	"SQRT":  evalSqrt,
	"ABS":   evalAbs,
	"POW":   evalPow,
	"DOT":   evalDot,
	"CROSS": evalCross,
	"NORM":  evalNorm,
}

// Helper to convert an integer/float value to float64
func toFloat(val record.Value) (float64, error) {
	switch val.Type {
	case record.Float:
		return val.Flt, nil
	case record.Integer:
		return float64(val.Int), nil
	default:
		return 0, fmt.Errorf("expected numeric value, got type %v", val.Type)
	}
}

// parseVector parses a vector from a string format: e.g. "[1.0, 2.0, 3.0]" or "1.0, 2.0, 3.0"
func parseVector(val record.Value) ([]float64, error) {
	if val.Type != record.Varchar {
		return nil, fmt.Errorf("expected vector to be represented as a VARCHAR string, got type %v", val.Type)
	}
	s := strings.TrimSpace(val.Str)
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		s = s[1 : len(s)-1]
	}
	if s == "" {
		return []float64{}, nil
	}
	parts := strings.Split(s, ",")
	vec := make([]float64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		f, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse vector element %q: %w", p, err)
		}
		vec = append(vec, f)
	}
	return vec, nil
}

// formatVector formats a vector as a string e.g. "[1.000000, 2.000000]"
func formatVector(vec []float64) string {
	parts := make([]string, len(vec))
	for i, f := range vec {
		parts[i] = fmt.Sprintf("%f", f)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// evalSin evaluates the SIN function.
func evalSin(args []record.Value) (record.Value, error) {
	if len(args) != 1 {
		return record.Value{}, fmt.Errorf("SIN expects exactly 1 argument, got %d", len(args))
	}
	v, err := toFloat(args[0])
	if err != nil {
		return record.Value{}, err
	}
	return record.Value{Type: record.Float, Flt: math.Sin(v)}, nil
}

// evalCos evaluates the COS function.
func evalCos(args []record.Value) (record.Value, error) {
	if len(args) != 1 {
		return record.Value{}, fmt.Errorf("COS expects exactly 1 argument, got %d", len(args))
	}
	v, err := toFloat(args[0])
	if err != nil {
		return record.Value{}, err
	}
	return record.Value{Type: record.Float, Flt: math.Cos(v)}, nil
}

// evalTan evaluates the TAN function.
func evalTan(args []record.Value) (record.Value, error) {
	if len(args) != 1 {
		return record.Value{}, fmt.Errorf("TAN expects exactly 1 argument, got %d", len(args))
	}
	v, err := toFloat(args[0])
	if err != nil {
		return record.Value{}, err
	}
	return record.Value{Type: record.Float, Flt: math.Tan(v)}, nil
}

// evalAsin evaluates the ASIN function.
func evalAsin(args []record.Value) (record.Value, error) {
	if len(args) != 1 {
		return record.Value{}, fmt.Errorf("ASIN expects exactly 1 argument, got %d", len(args))
	}
	v, err := toFloat(args[0])
	if err != nil {
		return record.Value{}, err
	}
	return record.Value{Type: record.Float, Flt: math.Asin(v)}, nil
}

// evalAcos evaluates the ACOS function.
func evalAcos(args []record.Value) (record.Value, error) {
	if len(args) != 1 {
		return record.Value{}, fmt.Errorf("ACOS expects exactly 1 argument, got %d", len(args))
	}
	v, err := toFloat(args[0])
	if err != nil {
		return record.Value{}, err
	}
	return record.Value{Type: record.Float, Flt: math.Acos(v)}, nil
}

// evalAtan evaluates the ATAN function.
func evalAtan(args []record.Value) (record.Value, error) {
	if len(args) != 1 {
		return record.Value{}, fmt.Errorf("ATAN expects exactly 1 argument, got %d", len(args))
	}
	v, err := toFloat(args[0])
	if err != nil {
		return record.Value{}, err
	}
	return record.Value{Type: record.Float, Flt: math.Atan(v)}, nil
}

// evalExp evaluates the EXP function.
func evalExp(args []record.Value) (record.Value, error) {
	if len(args) != 1 {
		return record.Value{}, fmt.Errorf("EXP expects exactly 1 argument, got %d", len(args))
	}
	v, err := toFloat(args[0])
	if err != nil {
		return record.Value{}, err
	}
	return record.Value{Type: record.Float, Flt: math.Exp(v)}, nil
}

// evalLog evaluates the LOG/LN function.
func evalLog(args []record.Value) (record.Value, error) {
	if len(args) != 1 {
		return record.Value{}, fmt.Errorf("LOG expects exactly 1 argument, got %d", len(args))
	}
	v, err := toFloat(args[0])
	if err != nil {
		return record.Value{}, err
	}
	if v <= 0 {
		return record.Value{}, fmt.Errorf("LOG argument must be positive, got %f", v)
	}
	return record.Value{Type: record.Float, Flt: math.Log(v)}, nil
}

// evalSqrt evaluates the SQRT function.
func evalSqrt(args []record.Value) (record.Value, error) {
	if len(args) != 1 {
		return record.Value{}, fmt.Errorf("SQRT expects exactly 1 argument, got %d", len(args))
	}
	v, err := toFloat(args[0])
	if err != nil {
		return record.Value{}, err
	}
	if v < 0 {
		return record.Value{}, fmt.Errorf("SQRT argument must be non-negative, got %f", v)
	}
	return record.Value{Type: record.Float, Flt: math.Sqrt(v)}, nil
}

// evalAbs evaluates the ABS function.
func evalAbs(args []record.Value) (record.Value, error) {
	if len(args) != 1 {
		return record.Value{}, fmt.Errorf("ABS expects exactly 1 argument, got %d", len(args))
	}
	arg := args[0]
	switch arg.Type {
	case record.Integer:
		val := arg.Int
		if val < 0 {
			val = -val
		}
		return record.Value{Type: record.Integer, Int: val}, nil
	case record.Float:
		return record.Value{Type: record.Float, Flt: math.Abs(arg.Flt)}, nil
	default:
		return record.Value{}, fmt.Errorf("ABS expects numeric argument, got type %v", arg.Type)
	}
}

// evalPow evaluates the POW function.
func evalPow(args []record.Value) (record.Value, error) {
	if len(args) != 2 {
		return record.Value{}, fmt.Errorf("POW expects exactly 2 arguments (base, exponent), got %d", len(args))
	}
	base, err := toFloat(args[0])
	if err != nil {
		return record.Value{}, fmt.Errorf("base: %w", err)
	}
	exp, err := toFloat(args[1])
	if err != nil {
		return record.Value{}, fmt.Errorf("exponent: %w", err)
	}
	return record.Value{Type: record.Float, Flt: math.Pow(base, exp)}, nil
}

// evalDot evaluates the DOT product of two vectors represented as VARCHAR strings.
func evalDot(args []record.Value) (record.Value, error) {
	if len(args) != 2 {
		return record.Value{}, fmt.Errorf("DOT expects exactly 2 arguments (v1, v2), got %d", len(args))
	}
	v1, err := parseVector(args[0])
	if err != nil {
		return record.Value{}, fmt.Errorf("v1: %w", err)
	}
	v2, err := parseVector(args[1])
	if err != nil {
		return record.Value{}, fmt.Errorf("v2: %w", err)
	}
	if len(v1) != len(v2) {
		return record.Value{}, fmt.Errorf("DOT expects vectors of equal length, got %d and %d", len(v1), len(v2))
	}
	dot := 0.0
	for i := range v1 {
		dot += v1[i] * v2[i]
	}
	return record.Value{Type: record.Float, Flt: dot}, nil
}

// evalCross evaluates the CROSS product of two 3D vectors represented as VARCHAR strings.
func evalCross(args []record.Value) (record.Value, error) {
	if len(args) != 2 {
		return record.Value{}, fmt.Errorf("CROSS expects exactly 2 arguments (v1, v2), got %d", len(args))
	}
	v1, err := parseVector(args[0])
	if err != nil {
		return record.Value{}, fmt.Errorf("v1: %w", err)
	}
	v2, err := parseVector(args[1])
	if err != nil {
		return record.Value{}, fmt.Errorf("v2: %w", err)
	}
	if len(v1) != 3 || len(v2) != 3 {
		return record.Value{}, fmt.Errorf("CROSS expects vectors of length 3, got %d and %d", len(v1), len(v2))
	}
	cross := []float64{
		v1[1]*v2[2] - v1[2]*v2[1],
		v1[2]*v2[0] - v1[0]*v2[2],
		v1[0]*v2[1] - v1[1]*v2[0],
	}
	return record.Value{Type: record.Varchar, Str: formatVector(cross)}, nil
}

// evalNorm evaluates the NORM (L2 magnitude) of a vector represented as a VARCHAR string.
func evalNorm(args []record.Value) (record.Value, error) {
	if len(args) != 1 {
		return record.Value{}, fmt.Errorf("NORM expects exactly 1 argument, got %d", len(args))
	}
	v, err := parseVector(args[0])
	if err != nil {
		return record.Value{}, err
	}
	sum := 0.0
	for _, f := range v {
		sum += f * f
	}
	return record.Value{Type: record.Float, Flt: math.Sqrt(sum)}, nil
}
