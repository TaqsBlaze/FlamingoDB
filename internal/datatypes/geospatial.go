package datatypes

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Point represents a 2D geospatial point.
type Point struct {
	X float64
	Y float64
}

// String returns the Well-Known Text (WKT) representation of the Point.
func (p Point) String() string {
	return fmt.Sprintf("POINT(%g %g)", p.X, p.Y)
}

// Equals checks if two Points are equal.
func (p Point) Equals(other Point) bool {
	return p.X == other.X && p.Y == other.Y
}

// Distance calculates the Euclidean distance between two Points.
func (p Point) Distance(other Point) float64 {
	dx := p.X - other.X
	dy := p.Y - other.Y
	return math.Sqrt(dx*dx + dy*dy)
}

// Polygon represents a 2D geospatial polygon, defined by a outer boundary ring.
type Polygon []Point

// String returns the Well-Known Text (WKT) representation of the Polygon.
func (p Polygon) String() string {
	if len(p) == 0 {
		return "POLYGON EMPTY"
	}
	var sb strings.Builder
	sb.WriteString("POLYGON((")
	for i, pt := range p {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%g %g", pt.X, pt.Y))
	}
	// If the polygon is not closed, we append the first point to close it in representation
	if len(p) > 0 && !p[0].Equals(p[len(p)-1]) {
		sb.WriteString(fmt.Sprintf(", %g %g", p[0].X, p[0].Y))
	}
	sb.WriteString("))")
	return sb.String()
}

// Equals checks if two Polygons are equal.
func (p Polygon) Equals(other Polygon) bool {
	if len(p) != len(other) {
		return false
	}
	for i := range p {
		if !p[i].Equals(other[i]) {
			return false
		}
	}
	return true
}

// Area calculates the area of the polygon using the Shoelace formula.
func (p Polygon) Area() float64 {
	n := len(p)
	if n < 3 {
		return 0.0
	}
	var area float64
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area += p[i].X * p[j].Y
		area -= p[j].X * p[i].Y
	}
	return math.Abs(area) / 2.0
}

// IntersectsPoint checks if a Point lies inside or on the boundary of the Polygon.
func (p Polygon) IntersectsPoint(pt Point) bool {
	vertices := []Point(p)
	n := len(vertices)
	if n == 0 {
		return false
	}
	// If the polygon is closed, omit the last point to avoid double-processing the closing segment in ray-casting
	if vertices[0].Equals(vertices[n-1]) {
		vertices = vertices[:n-1]
		n = len(vertices)
	}
	if n < 3 {
		return false
	}

	// 1. Check if the point lies exactly on any segment of the boundary
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		if PointOnSegment(pt, vertices[i], vertices[j]) {
			return true
		}
	}

	// 2. Ray-casting algorithm to check if point is inside
	inside := false
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		pi := vertices[i]
		pj := vertices[j]
		if ((pi.Y > pt.Y) != (pj.Y > pt.Y)) &&
			(pt.X < (pj.X-pi.X)*(pt.Y-pi.Y)/(pj.Y-pi.Y)+pi.X) {
			inside = !inside
		}
	}
	return inside
}

// IntersectsPolygon checks if two Polygons intersect.
func (p Polygon) IntersectsPolygon(other Polygon) bool {
	v1 := []Point(p)
	if len(v1) > 0 && v1[0].Equals(v1[len(v1)-1]) {
		v1 = v1[:len(v1)-1]
	}
	v2 := []Point(other)
	if len(v2) > 0 && v2[0].Equals(v2[len(v2)-1]) {
		v2 = v2[:len(v2)-1]
	}

	if len(v1) < 3 || len(v2) < 3 {
		return false
	}

	// 1. Check if any edge of poly1 intersects any edge of poly2
	n1, n2 := len(v1), len(v2)
	for i := 0; i < n1; i++ {
		next1 := (i + 1) % n1
		for j := 0; j < n2; j++ {
			next2 := (j + 1) % n2
			if SegmentsIntersect(v1[i], v1[next1], v2[j], v2[next2]) {
				return true
			}
		}
	}

	// 2. Check if poly1 is entirely inside poly2
	if other.IntersectsPoint(v1[0]) {
		return true
	}

	// 3. Check if poly2 is entirely inside poly1
	if p.IntersectsPoint(v2[0]) {
		return true
	}

	return false
}

// PointOnSegment checks if pt lies on the line segment p1-p2.
func PointOnSegment(pt Point, p1 Point, p2 Point) bool {
	crossProduct := (pt.Y-p1.Y)*(p2.X-p1.X) - (pt.X-p1.X)*(p2.Y-p1.Y)
	if math.Abs(crossProduct) > 1e-9 {
		return false
	}
	return pt.X >= math.Min(p1.X, p2.X) && pt.X <= math.Max(p1.X, p2.X) &&
		pt.Y >= math.Min(p1.Y, p2.Y) && pt.Y <= math.Max(p1.Y, p2.Y)
}

// ccw returns:
//  1 if a->b->c is counter-clockwise
// -1 if a->b->c is clockwise
//  0 if collinear
func ccw(a, b, c Point) int {
	val := (b.Y-a.Y)*(c.X-b.X) - (b.X-a.X)*(c.Y-b.Y)
	if math.Abs(val) < 1e-9 {
		return 0
	}
	if val > 0 {
		return 1
	}
	return -1
}

// SegmentsIntersect checks if line segment p1-q1 intersects with p2-q2.
func SegmentsIntersect(p1, q1, p2, q2 Point) bool {
	o1 := ccw(p1, q1, p2)
	o2 := ccw(p1, q1, q2)
	o3 := ccw(p2, q2, p1)
	o4 := ccw(p2, q2, q1)

	// General case
	if o1 != o2 && o3 != o4 {
		return true
	}

	// Special cases
	if o1 == 0 && PointOnSegment(p2, p1, q1) { return true }
	if o2 == 0 && PointOnSegment(q2, p1, q1) { return true }
	if o3 == 0 && PointOnSegment(p1, p2, q2) { return true }
	if o4 == 0 && PointOnSegment(q1, p2, q2) { return true }

	return false
}

// ParseWKT parses a Well-Known Text (WKT) string into a Point or Polygon.
func ParseWKT(wkt string) (any, error) {
	s := strings.TrimSpace(wkt)
	upper := strings.ToUpper(s)

	if strings.HasPrefix(upper, "POINT") {
		// Expect Format: POINT(x y) or POINT (x y)
		re := regexp.MustCompile(`^POINT\s*\(\s*(-?\d+(?:\.\d+)?)\s+(-?\d+(?:\.\d+)?)\s*\)$`)
		matches := re.FindStringSubmatch(upper)
		if len(matches) != 3 {
			return nil, fmt.Errorf("invalid POINT WKT format: %q", wkt)
		}
		x, err := strconv.ParseFloat(matches[1], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse X coordinate: %w", err)
		}
		y, err := strconv.ParseFloat(matches[2], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Y coordinate: %w", err)
		}
		return Point{X: x, Y: y}, nil
	}

	if strings.HasPrefix(upper, "POLYGON") {
		// Expect Format: POLYGON((x1 y1, x2 y2, ...))
		// We allow optional spaces
		re := regexp.MustCompile(`^POLYGON\s*\(\(\s*(.*?)\s*\)\)$`)
		matches := re.FindStringSubmatch(upper)
		if len(matches) != 2 {
			return nil, fmt.Errorf("invalid POLYGON WKT format: %q", wkt)
		}
		pointsStr := strings.Split(matches[1], ",")
		var poly Polygon
		for _, ptStr := range pointsStr {
			ptStr = strings.TrimSpace(ptStr)
			coords := strings.Fields(ptStr)
			if len(coords) != 2 {
				return nil, fmt.Errorf("invalid coordinate pair %q in POLYGON WKT", ptStr)
			}
			x, err := strconv.ParseFloat(coords[0], 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse coordinate X %q: %w", coords[0], err)
			}
			y, err := strconv.ParseFloat(coords[1], 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse coordinate Y %q: %w", coords[1], err)
			}
			poly = append(poly, Point{X: x, Y: y})
		}
		return poly, nil
	}

	return nil, fmt.Errorf("unsupported or invalid WKT type: %q", wkt)
}
