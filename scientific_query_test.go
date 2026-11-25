package flamingodb

import (
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/record"
	"path/filepath"
	"testing"
)

func TestScientificQueries(t *testing.T) {
	db, err := Connect(filepath.Join(t.TempDir(), "lab.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{"CREATE TABLE runs (station VARCHAR, temp INT, v VECTOR);", "INSERT INTO runs VALUES ('East', 21, [3,4,0]), ('East', 23, [0,0,1]), ('West', 24, [1,0,0]);"} {
		if _, err = db.Run(q); err != nil {
			t.Fatal(err)
		}
	}
	res, err := db.Run("SELECT AVG(temp), station, COUNT(*) FROM runs GROUP BY station HAVING AVG(temp) > 21 ORDER BY station;")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 2 || res.Rows[0].Values[0].Flt != 22 || res.Rows[0].Values[1].Str != "East" || res.Rows[0].Values[2].Int != 2 || res.Rows[1].Values[0].Flt != 24 {
		t.Fatalf("group results: %v", res.Rows)
	}
	res, err = db.Run("SELECT NORM(v), DOT(v,'[1,2,0]'), CROSS(v,[0,0,1]), CROSS('[1,0,0]','[0,1,0]') FROM runs WHERE temp = 21;")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 {
		t.Fatal(res.Rows)
	}
	v := res.Rows[0].Values
	if v[0].Flt != 5 || v[1].Flt != 11 || !v[2].Vec.Equals([]float64{4, -3, 0}) || v[3].Type != record.Varchar {
		t.Fatalf("vector results: %v", v)
	}
	for _, q := range []string{"SELECT AVG(station) FROM runs;", "SELECT AVG() FROM runs;", "SELECT DOT(v,[0]) FROM runs;"} {
		if _, err := db.Run(q); err == nil {
			t.Errorf("expected error for %s", q)
		}
	}
}
