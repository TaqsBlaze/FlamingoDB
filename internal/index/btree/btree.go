package btree

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/TaqsBlaze/FlamingoDB/internal/storage/page"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/pager"
)

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

var (
	ErrKeyNotFound  = errors.New("btree: key not found")
	ErrDuplicateKey = errors.New("btree: duplicate key")
)

// ---------------------------------------------------------------------------
// Key types
// ---------------------------------------------------------------------------

// KeyType identifies the data type of the indexed column.
type KeyType uint8

const (
	KeyInt     KeyType = iota // int32 key
	KeyFloat                  // float64 key
	KeyVarchar                // variable-length string key (max 255 bytes)
)

// Key is a comparable index key. Only one field is active based on Type.
type Key struct {
	Type KeyType
	IVal int32
	FVal float64
	SVal string
}

// Compare returns -1, 0, or 1.
func (k Key) Compare(other Key) int {
	switch k.Type {
	case KeyInt:
		if k.IVal < other.IVal {
			return -1
		}
		if k.IVal > other.IVal {
			return 1
		}
		return 0
	case KeyFloat:
		if k.FVal < other.FVal {
			return -1
		}
		if k.FVal > other.FVal {
			return 1
		}
		return 0
	case KeyVarchar:
		return bytes.Compare([]byte(k.SVal), []byte(other.SVal))
	}
	return 0
}

// ---------------------------------------------------------------------------
// Page layout constants
//
// Each B+ Tree node lives in exactly one fixed-size page.
//
// Page header (16 bytes):
//   [0:1]   NodeType  (0 = internal, 1 = leaf)
//   [1:2]   KeyType
//   [2:4]   NumKeys  uint16
//   [4:8]   Parent   uint32 (PageID, 0xFFFFFFFF = no parent)
//   [8:12]  NextLeaf uint32 (PageID for leaf; unused for internal)
//   [12:16] reserved
//
// After the header, keys and child/value pointers are packed.
// Internal nodes: k keys + (k+1) child PageIDs
// Leaf nodes:     k (key, pageID) pairs pointing to the heap row's page
// ---------------------------------------------------------------------------

const (
	pageHeaderSize = 16
	nodeTypeOffset = 0
	keyTypeOffset  = 1
	numKeysOffset  = 2
	parentOffset   = 4
	nextLeafOffset = 8

	nodeTypeInternal uint8 = 0
	nodeTypeLeaf     uint8 = 1

	noPage = uint32(math.MaxUint32)

	// Fixed key sizes on disk (we pad Varchar to 256 bytes for simplicity)
	intKeySize     = 4
	floatKeySize   = 8
	varcharKeySize = 256 // 1 byte length + up to 255 bytes

	// PageID on disk
	pidSize = 4
)

// keySizeForType returns the fixed on-disk size for a key type.
func keySizeForType(kt KeyType) int {
	switch kt {
	case KeyInt:
		return intKeySize
	case KeyFloat:
		return floatKeySize
	case KeyVarchar:
		return varcharKeySize
	}
	return intKeySize
}

// order returns the maximum number of keys per node for the given page and key type.
// Internal node: maxKeys keys + (maxKeys+1) child PIDs
//   pageSize - headerSize = maxKeys*keySize + (maxKeys+1)*pidSize
//   maxKeys = (pageSize - headerSize - pidSize) / (keySize + pidSize)
// Leaf node: maxKeys (key + pid) pairs
//   maxKeys = (pageSize - headerSize) / (keySize + pidSize)
// We use the leaf formula (more conservative) for both.
func order(pageSize uint32, kt KeyType) int {
	ks := keySizeForType(kt)
	o := (int(pageSize) - pageHeaderSize) / (ks + pidSize)
	if o < 2 {
		o = 2
	}
	return o
}

// ---------------------------------------------------------------------------
// Node — an in-memory view of a single page
// ---------------------------------------------------------------------------

type node struct {
	pageID   page.PageID
	nodeType uint8
	keyType  KeyType
	keys     []Key
	// For internal nodes: len(children) == len(keys)+1
	children []page.PageID
	// For leaf nodes: len(values) == len(keys), each value is the heap PageID
	values   []page.PageID
	parent   page.PageID
	nextLeaf page.PageID
}

// ---------------------------------------------------------------------------
// BTree
// ---------------------------------------------------------------------------

// BTree is a page-backed B+ Tree index.
type BTree struct {
	pager    *pager.Pager
	rootID   page.PageID
	keyType  KeyType
	pageSize uint32
	order    int // maximum keys per node
}

// New creates a new BTree, allocating a root leaf page.
func New(p *pager.Pager, pageSize uint32, kt KeyType) (*BTree, error) {
	bt := &BTree{
		pager:    p,
		keyType:  kt,
		pageSize: pageSize,
		order:    order(pageSize, kt),
	}

	// Allocate and initialise the root leaf page.
	pg, err := p.AllocatePage()
	if err != nil {
		return nil, fmt.Errorf("btree: failed to allocate root: %w", err)
	}

	root := &node{
		pageID:   pg.ID(),
		nodeType: nodeTypeLeaf,
		keyType:  kt,
		parent:   page.PageID(noPage),
		nextLeaf: page.PageID(noPage),
	}

	bt.rootID = pg.ID()
	return bt, bt.writeNode(root)
}

// Load loads an existing BTree whose root is at rootPageID.
func Load(p *pager.Pager, pageSize uint32, kt KeyType, rootPageID page.PageID) *BTree {
	return &BTree{
		pager:    p,
		rootID:   rootPageID,
		keyType:  kt,
		pageSize: pageSize,
		order:    order(pageSize, kt),
	}
}

// RootID returns the page ID of the B+ Tree root node.
func (bt *BTree) RootID() page.PageID { return bt.rootID }

// ---------------------------------------------------------------------------
// Insert
// ---------------------------------------------------------------------------

// Insert adds (key → heapPageID) to the tree.
func (bt *BTree) Insert(key Key, heapPageID page.PageID) error {
	leaf, err := bt.findLeaf(key)
	if err != nil {
		return err
	}

	// Check for duplicate
	for _, k := range leaf.keys {
		if k.Compare(key) == 0 {
			return ErrDuplicateKey
		}
	}

	// Insert into leaf (sorted)
	leaf = insertIntoLeaf(leaf, key, heapPageID)

	if len(leaf.keys) <= bt.order {
		return bt.writeNode(leaf)
	}

	// Leaf is full — split
	return bt.splitLeaf(leaf)
}

func insertIntoLeaf(n *node, key Key, val page.PageID) *node {
	pos := 0
	for pos < len(n.keys) && n.keys[pos].Compare(key) < 0 {
		pos++
	}
	n.keys = append(n.keys, Key{})
	copy(n.keys[pos+1:], n.keys[pos:])
	n.keys[pos] = key

	n.values = append(n.values, 0)
	copy(n.values[pos+1:], n.values[pos:])
	n.values[pos] = val
	return n
}

func (bt *BTree) splitLeaf(leaf *node) error {
	mid := len(leaf.keys) / 2

	// New right sibling
	rightPg, err := bt.pager.AllocatePage()
	if err != nil {
		return err
	}
	right := &node{
		pageID:   rightPg.ID(),
		nodeType: nodeTypeLeaf,
		keyType:  leaf.keyType,
		keys:     append([]Key{}, leaf.keys[mid:]...),
		values:   append([]page.PageID{}, leaf.values[mid:]...),
		parent:   leaf.parent,
		nextLeaf: leaf.nextLeaf,
	}

	// Trim left
	leaf.keys = leaf.keys[:mid]
	leaf.values = leaf.values[:mid]
	leaf.nextLeaf = rightPg.ID()

	if err := bt.writeNode(leaf); err != nil {
		return err
	}
	if err := bt.writeNode(right); err != nil {
		return err
	}

	// Push the first key of right sibling up
	return bt.insertIntoParent(leaf, right.keys[0], right)
}

func (bt *BTree) insertIntoParent(left *node, key Key, right *node) error {
	// Left is root — create a new root
	if left.parent == page.PageID(noPage) {
		return bt.createNewRoot(left, key, right)
	}

	parent, err := bt.readNode(left.parent)
	if err != nil {
		return err
	}

	// Find left's position among parent's children
	pos := 0
	for pos < len(parent.children) && parent.children[pos] != left.pageID {
		pos++
	}

	// Insert key and right child pointer
	parent.keys = append(parent.keys, Key{})
	copy(parent.keys[pos+1:], parent.keys[pos:])
	parent.keys[pos] = key

	parent.children = append(parent.children, 0)
	copy(parent.children[pos+2:], parent.children[pos+1:])
	parent.children[pos+1] = right.pageID

	// Update right's parent
	right.parent = parent.pageID
	if err := bt.writeNode(right); err != nil {
		return err
	}

	if len(parent.keys) <= bt.order {
		return bt.writeNode(parent)
	}

	return bt.splitInternal(parent)
}

func (bt *BTree) splitInternal(n *node) error {
	mid := len(n.keys) / 2
	promoteKey := n.keys[mid]

	rightPg, err := bt.pager.AllocatePage()
	if err != nil {
		return err
	}
	right := &node{
		pageID:   rightPg.ID(),
		nodeType: nodeTypeInternal,
		keyType:  n.keyType,
		keys:     append([]Key{}, n.keys[mid+1:]...),
		children: append([]page.PageID{}, n.children[mid+1:]...),
		parent:   n.parent,
		nextLeaf: page.PageID(noPage),
	}

	n.keys = n.keys[:mid]
	n.children = n.children[:mid+1]

	// Reparent right's children
	for _, childID := range right.children {
		child, err := bt.readNode(childID)
		if err != nil {
			return err
		}
		child.parent = right.pageID
		if err := bt.writeNode(child); err != nil {
			return err
		}
	}

	if err := bt.writeNode(n); err != nil {
		return err
	}
	if err := bt.writeNode(right); err != nil {
		return err
	}

	return bt.insertIntoParent(n, promoteKey, right)
}

func (bt *BTree) createNewRoot(left *node, key Key, right *node) error {
	rootPg, err := bt.pager.AllocatePage()
	if err != nil {
		return err
	}
	root := &node{
		pageID:   rootPg.ID(),
		nodeType: nodeTypeInternal,
		keyType:  left.keyType,
		keys:     []Key{key},
		children: []page.PageID{left.pageID, right.pageID},
		parent:   page.PageID(noPage),
		nextLeaf: page.PageID(noPage),
	}

	left.parent = rootPg.ID()
	right.parent = rootPg.ID()
	bt.rootID = rootPg.ID()

	if err := bt.writeNode(left); err != nil {
		return err
	}
	if err := bt.writeNode(right); err != nil {
		return err
	}
	return bt.writeNode(root)
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

// Search returns the heap PageID stored for the given key, or ErrKeyNotFound.
func (bt *BTree) Search(key Key) (page.PageID, error) {
	leaf, err := bt.findLeaf(key)
	if err != nil {
		return 0, err
	}
	for i, k := range leaf.keys {
		if k.Compare(key) == 0 {
			return leaf.values[i], nil
		}
	}
	return 0, ErrKeyNotFound
}

// ---------------------------------------------------------------------------
// Range Scan
// ---------------------------------------------------------------------------

// RangeScan returns all heap PageIDs for keys in [low, high] (inclusive).
// Pass a zero-value Key with a sentinel to get a one-sided range; call with
// low.Compare(high) > 0 to return an empty result.
func (bt *BTree) RangeScan(low, high Key) ([]page.PageID, error) {
	if low.Compare(high) > 0 {
		return nil, nil
	}

	leaf, err := bt.findLeaf(low)
	if err != nil {
		return nil, err
	}

	var results []page.PageID
	for {
		for i, k := range leaf.keys {
			cmpLow := k.Compare(low)
			cmpHigh := k.Compare(high)
			if cmpLow >= 0 && cmpHigh <= 0 {
				results = append(results, leaf.values[i])
			}
			if cmpHigh > 0 {
				return results, nil
			}
		}

		if leaf.nextLeaf == page.PageID(noPage) {
			break
		}
		leaf, err = bt.readNode(leaf.nextLeaf)
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}

// ---------------------------------------------------------------------------
// Tree traversal helpers
// ---------------------------------------------------------------------------

func (bt *BTree) findLeaf(key Key) (*node, error) {
	n, err := bt.readNode(bt.rootID)
	if err != nil {
		return nil, err
	}
	for n.nodeType == nodeTypeInternal {
		pos := len(n.keys)
		for i, k := range n.keys {
			if key.Compare(k) < 0 {
				pos = i
				break
			}
		}
		n, err = bt.readNode(n.children[pos])
		if err != nil {
			return nil, err
		}
	}
	return n, nil
}

// ---------------------------------------------------------------------------
// Page serialisation
// ---------------------------------------------------------------------------

func (bt *BTree) readNode(id page.PageID) (*node, error) {
	pg, err := bt.pager.FetchPage(id)
	if err != nil {
		return nil, fmt.Errorf("btree: read page %d: %w", id, err)
	}
	return deserialiseNode(pg.Data(), id)
}

func (bt *BTree) writeNode(n *node) error {
	pg, err := bt.pager.FetchPage(n.pageID)
	if err != nil {
		return fmt.Errorf("btree: write page %d: %w", n.pageID, err)
	}
	serialiseNode(n, pg.Data())
	return bt.pager.WritePage(pg)
}

// serialiseNode encodes a node into a page's data buffer.
func serialiseNode(n *node, buf []byte) {
	// Header
	buf[nodeTypeOffset] = n.nodeType
	buf[keyTypeOffset] = uint8(n.keyType)
	binary.LittleEndian.PutUint16(buf[numKeysOffset:], uint16(len(n.keys)))
	binary.LittleEndian.PutUint32(buf[parentOffset:], uint32(n.parent))
	binary.LittleEndian.PutUint32(buf[nextLeafOffset:], uint32(n.nextLeaf))

	offset := pageHeaderSize
	ks := keySizeForType(n.keyType)

	if n.nodeType == nodeTypeLeaf {
		for i, k := range n.keys {
			serialiseKey(k, n.keyType, buf[offset:offset+ks])
			offset += ks
			binary.LittleEndian.PutUint32(buf[offset:], uint32(n.values[i]))
			offset += pidSize
		}
	} else {
		// Internal: child[0], then for each key: key, child[i+1]
		binary.LittleEndian.PutUint32(buf[offset:], uint32(n.children[0]))
		offset += pidSize
		for i, k := range n.keys {
			serialiseKey(k, n.keyType, buf[offset:offset+ks])
			offset += ks
			binary.LittleEndian.PutUint32(buf[offset:], uint32(n.children[i+1]))
			offset += pidSize
		}
	}
}

// deserialiseNode decodes a page data buffer into a node.
func deserialiseNode(buf []byte, id page.PageID) (*node, error) {
	n := &node{
		pageID:   id,
		nodeType: buf[nodeTypeOffset],
		keyType:  KeyType(buf[keyTypeOffset]),
		parent:   page.PageID(binary.LittleEndian.Uint32(buf[parentOffset:])),
		nextLeaf: page.PageID(binary.LittleEndian.Uint32(buf[nextLeafOffset:])),
	}

	numKeys := int(binary.LittleEndian.Uint16(buf[numKeysOffset:]))
	ks := keySizeForType(n.keyType)
	offset := pageHeaderSize

	if n.nodeType == nodeTypeLeaf {
		n.keys = make([]Key, numKeys)
		n.values = make([]page.PageID, numKeys)
		for i := range n.keys {
			n.keys[i] = deserialiseKey(n.keyType, buf[offset:offset+ks])
			offset += ks
			n.values[i] = page.PageID(binary.LittleEndian.Uint32(buf[offset:]))
			offset += pidSize
		}
	} else {
		n.children = make([]page.PageID, numKeys+1)
		n.keys = make([]Key, numKeys)
		n.children[0] = page.PageID(binary.LittleEndian.Uint32(buf[offset:]))
		offset += pidSize
		for i := range n.keys {
			n.keys[i] = deserialiseKey(n.keyType, buf[offset:offset+ks])
			offset += ks
			n.children[i+1] = page.PageID(binary.LittleEndian.Uint32(buf[offset:]))
			offset += pidSize
		}
	}

	return n, nil
}

func serialiseKey(k Key, kt KeyType, buf []byte) {
	switch kt {
	case KeyInt:
		binary.LittleEndian.PutUint32(buf, uint32(k.IVal))
	case KeyFloat:
		bits := math.Float64bits(k.FVal)
		binary.LittleEndian.PutUint64(buf, bits)
	case KeyVarchar:
		s := k.SVal
		if len(s) > 255 {
			s = s[:255]
		}
		buf[0] = uint8(len(s))
		copy(buf[1:], s)
	}
}

func deserialiseKey(kt KeyType, buf []byte) Key {
	switch kt {
	case KeyInt:
		return Key{Type: KeyInt, IVal: int32(binary.LittleEndian.Uint32(buf))}
	case KeyFloat:
		bits := binary.LittleEndian.Uint64(buf)
		return Key{Type: KeyFloat, FVal: math.Float64frombits(bits)}
	case KeyVarchar:
		length := int(buf[0])
		return Key{Type: KeyVarchar, SVal: string(buf[1 : 1+length])}
	}
	return Key{}
}
