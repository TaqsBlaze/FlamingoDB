package datatypes

import (
	"math"
	"testing"
)

func TestPoint(t *testing.T) {
	p1 := Point{X: 1.0, Y: 2.0}
	p2 := Point{X: 1.0, Y: 2.0}
	p3 := Point{X: 4.0, Y: 6.0}

	if !p1.Equals(p2) {
		t.Errorf("p1 and p2 should be equal")
	}

	if p1.Equals(p3) {
		t.Errorf("p1 and p3 should not be equal")
	}

	dist := p1.Distance(p3)
	expectedDist := 5.0 // 3-4-5 triangle
	if math.Abs(dist-expectedDist) > 1e-9 {
		t.Errorf("expected distance %f, got %f", expectedDist, dist)
	}

	if p1.String() != "POINT(1 2)" {
		t.Errorf("expected string 'POINT(1 2)', got %q", p1.String())
	}
}

func TestPolygon(t *testing.T) {
	// A simple square of 10x10
	poly := Polygon{
		{X: 0, Y: 0},
		{X: 10, Y: 0},
		{X: 10, Y: 10},
		{X: 0, Y: 10},
	}

	if poly.String() != "POLYGON((0 0, 10 0, 10 10, 0 10, 0 0))" {
		t.Errorf("unexpected polygon string representation: %q", poly.String())
	}

	area := poly.Area()
	if area != 100.0 {
		t.Errorf("expected area 100, got %f", area)
	}

	// Test point intersection
	ptInside := Point{X: 5, Y: 5}
	ptBoundary := Point{X: 10, Y: 5}
	ptOutside := Point{X: 15, Y: 5}

	if !poly.IntersectsPoint(ptInside) {
		t.Errorf("expected point %v to intersect polygon", ptInside)
	}
	if !poly.IntersectsPoint(ptBoundary) {
		t.Errorf("expected point %v on boundary to intersect polygon", ptBoundary)
	}
	if poly.IntersectsPoint(ptOutside) {
		t.Errorf("expected point %v outside not to intersect polygon", ptOutside)
	}
}

func TestPolygonIntersection(t *testing.T) {
	poly1 := Polygon{{0, 0}, {4, 0}, {4, 4}, {0, 4}}
	poly2 := Polygon{{2, 2}, {6, 2}, {6, 6}, {2, 6}} // Overlaps
	poly3 := Polygon{{5, 5}, {7, 5}, {7, 7}, {5, 7}} // Separate

	if !poly1.IntersectsPolygon(poly2) {
		t.Errorf("expected poly1 to intersect poly2")
	}
	if poly1.IntersectsPolygon(poly3) {
		t.Errorf("expected poly1 to not intersect poly3")
	}
}

func TestParseWKT(t *testing.T) {
	ptRaw := "POINT(3.5 -4.5)"
	val, err := ParseWKT(ptRaw)
	if err != nil {
		t.Fatalf("failed to parse POINT WKT: %v", err)
	}
	pt, ok := val.(Point)
	if !ok {
		t.Fatalf("expected Point, got %T", val)
	}
	if pt.X != 3.5 || pt.Y != -4.5 {
		t.Errorf("parsed coords wrong: %v", pt)
	}

	polyRaw := "POLYGON((0 0, 10 0, 10 10, 0 10, 0 0))"
	val2, err := ParseWKT(polyRaw)
	if err != nil {
		t.Fatalf("failed to parse POLYGON WKT: %v", err)
	}
	poly, ok := val2.(Polygon)
	if !ok {
		t.Fatalf("expected Polygon, got %T", val2)
	}
	if len(poly) != 5 {
		t.Errorf("expected 5 points in polygon ring, got %d", len(poly))
	}
}
