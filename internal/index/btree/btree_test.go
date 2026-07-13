package btree_test

import (
	"path/filepath"
	"testing"

	"github.com/TaqsBlaze/FlamingoDB/internal/index/btree"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/disk"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/pager"
)

const pageSize = uint32(4096)

func newTree(t *testing.T, kt btree.KeyType) *btree.BTree {
	t.Helper()
	dm, err := disk.NewDiskManager(filepath.Join(t.TempDir(), "idx.db"), pageSize)
	if err != nil {
		t.Fatalf("disk manager: %v", err)
	}
	t.Cleanup(func() { dm.Close() })

	p, err := pager.New(dm, pageSize)
	if err != nil {
		t.Fatalf("pager: %v", err)
	}

	tree, err := btree.New(p, pageSize, kt)
	if err != nil {
		t.Fatalf("btree.New: %v", err)
	}
	return tree
}

// ---------------------------------------------------------------------------
// Integer key tests
// ---------------------------------------------------------------------------

func TestIntSearch_EmptyTree(t *testing.T) {
	tree := newTree(t, btree.KeyInt)
	_, err := tree.Search(btree.Key{Type: btree.KeyInt, IVal: 1})
	if err != btree.ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestIntInsertAndSearch(t *testing.T) {
	tree := newTree(t, btree.KeyInt)

	pairs := []struct {
		key int32
		pid uint32
	}{
		{42, 10}, {7, 20}, {99, 30}, {3, 40}, {55, 50},
	}

	for _, p := range pairs {
		if err := tree.Insert(btree.Key{Type: btree.KeyInt, IVal: p.key}, 0); err != nil {
			t.Fatalf("insert %d: %v", p.key, err)
		}
	}

	// Re-insert should fail
	err := tree.Insert(btree.Key{Type: btree.KeyInt, IVal: 42}, 0)
	if err != btree.ErrDuplicateKey {
		t.Fatalf("expected ErrDuplicateKey, got %v", err)
	}

	// All inserted keys must be findable
	for _, p := range pairs {
		_, err := tree.Search(btree.Key{Type: btree.KeyInt, IVal: p.key})
		if err != nil {
			t.Errorf("search %d: %v", p.key, err)
		}
	}

	// Non-existent key
	_, err = tree.Search(btree.Key{Type: btree.KeyInt, IVal: 999})
	if err != btree.ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound for 999, got %v", err)
	}
}

func TestIntSplit_ManyInserts(t *testing.T) {
	tree := newTree(t, btree.KeyInt)

	// Insert enough keys to force multiple node splits
	const n = 500
	for i := int32(0); i < n; i++ {
		if err := tree.Insert(btree.Key{Type: btree.KeyInt, IVal: i}, 0); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	// All must be searchable after splits
	for i := int32(0); i < n; i++ {
		if _, err := tree.Search(btree.Key{Type: btree.KeyInt, IVal: i}); err != nil {
			t.Errorf("search %d after splits: %v", i, err)
		}
	}
}

func TestIntRangeScan(t *testing.T) {
	tree := newTree(t, btree.KeyInt)

	for _, v := range []int32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100} {
		if err := tree.Insert(btree.Key{Type: btree.KeyInt, IVal: v}, 0); err != nil {
			t.Fatalf("insert %d: %v", v, err)
		}
	}

	low := btree.Key{Type: btree.KeyInt, IVal: 30}
	high := btree.Key{Type: btree.KeyInt, IVal: 70}
	results, err := tree.RangeScan(low, high)
	if err != nil {
		t.Fatalf("range scan: %v", err)
	}
	// Expect keys 30, 40, 50, 60, 70 → 5 results
	if len(results) != 5 {
		t.Errorf("expected 5 results, got %d", len(results))
	}
}

func TestIntRangeScan_EmptyRange(t *testing.T) {
	tree := newTree(t, btree.KeyInt)
	tree.Insert(btree.Key{Type: btree.KeyInt, IVal: 1}, 0)
	tree.Insert(btree.Key{Type: btree.KeyInt, IVal: 5}, 0)

	// Inverted range should return nothing
	results, err := tree.RangeScan(
		btree.Key{Type: btree.KeyInt, IVal: 10},
		btree.Key{Type: btree.KeyInt, IVal: 5},
	)
	if err != nil {
		t.Fatalf("range scan: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for inverted range, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// Float key tests
// ---------------------------------------------------------------------------

func TestFloatInsertAndSearch(t *testing.T) {
	tree := newTree(t, btree.KeyFloat)

	floats := []float64{3.14, 2.71, -1.46, 0.0, 99.9, -273.15}
	for _, f := range floats {
		if err := tree.Insert(btree.Key{Type: btree.KeyFloat, FVal: f}, 0); err != nil {
			t.Fatalf("insert %f: %v", f, err)
		}
	}

	for _, f := range floats {
		if _, err := tree.Search(btree.Key{Type: btree.KeyFloat, FVal: f}); err != nil {
			t.Errorf("search %f: %v", f, err)
		}
	}
}

func TestFloatRangeScan(t *testing.T) {
	tree := newTree(t, btree.KeyFloat)

	// Insert star magnitudes
	mags := []float64{-1.46, -0.74, 0.13, 0.50, 1.25, 2.04, 3.00}
	for _, m := range mags {
		tree.Insert(btree.Key{Type: btree.KeyFloat, FVal: m}, 0)
	}

	// Scan visible stars (magnitude < 1.0 → between -1.46 and 0.50)
	results, err := tree.RangeScan(
		btree.Key{Type: btree.KeyFloat, FVal: -2.0},
		btree.Key{Type: btree.KeyFloat, FVal: 0.50},
	)
	if err != nil {
		t.Fatalf("range scan: %v", err)
	}
	// -1.46, -0.74, 0.13, 0.50 → 4 results
	if len(results) != 4 {
		t.Errorf("expected 4 results, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// Varchar key tests
// ---------------------------------------------------------------------------

func TestVarcharInsertAndSearch(t *testing.T) {
	tree := newTree(t, btree.KeyVarchar)

	words := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta"}
	for _, w := range words {
		if err := tree.Insert(btree.Key{Type: btree.KeyVarchar, SVal: w}, 0); err != nil {
			t.Fatalf("insert %q: %v", w, err)
		}
	}

	for _, w := range words {
		if _, err := tree.Search(btree.Key{Type: btree.KeyVarchar, SVal: w}); err != nil {
			t.Errorf("search %q: %v", w, err)
		}
	}

	_, err := tree.Search(btree.Key{Type: btree.KeyVarchar, SVal: "omega"})
	if err != btree.ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound for 'omega', got %v", err)
	}
}

func TestVarcharRangeScan(t *testing.T) {
	tree := newTree(t, btree.KeyVarchar)

	elements := []string{"Carbon", "Hydrogen", "Iron", "Nitrogen", "Oxygen", "Silicon"}
	for _, e := range elements {
		tree.Insert(btree.Key{Type: btree.KeyVarchar, SVal: e}, 0)
	}

	results, err := tree.RangeScan(
		btree.Key{Type: btree.KeyVarchar, SVal: "Hydrogen"},
		btree.Key{Type: btree.KeyVarchar, SVal: "Oxygen"},
	)
	if err != nil {
		t.Fatalf("range scan: %v", err)
	}
	// Hydrogen, Iron, Nitrogen, Oxygen → 4
	if len(results) != 4 {
		t.Errorf("expected 4 results, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// Persistence test — reload tree from disk
// ---------------------------------------------------------------------------

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "persist.db")

	// Write
	var rootID uint32
	{
		dm, _ := disk.NewDiskManager(dbPath, pageSize)
		p, _ := pager.New(dm, pageSize)
		tree, _ := btree.New(p, pageSize, btree.KeyInt)

		for i := int32(1); i <= 100; i++ {
			tree.Insert(btree.Key{Type: btree.KeyInt, IVal: i}, 0)
		}
		rootID = uint32(tree.RootID())
		p.FlushAll()
		dm.Close()
	}

	// Reload
	{
		dm, _ := disk.NewDiskManager(dbPath, pageSize)
		p, _ := pager.New(dm, pageSize)
		tree := btree.Load(p, pageSize, btree.KeyInt, 0)
		_ = rootID // suppress unused warning; Load uses the persisted rootID via the pager

		for i := int32(1); i <= 100; i++ {
			if _, err := tree.Search(btree.Key{Type: btree.KeyInt, IVal: i}); err != nil {
				t.Errorf("after reload, search %d: %v", i, err)
			}
		}
		dm.Close()
	}
}
