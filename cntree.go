package rquad

import (
	"errors"
	"fmt"
	"image"
	"math"

	"github.com/aurelien-rainone/binimg"
)

// CNTree implements a Cardinal Neighbour Quadtree, a quadtree structure that
// allows finding neighbor quadrants in constant time O(1) regardless of their
// sizes.
//
// The time complexity reduction is obtained through the addition of four
// pointers per node in the quadtree, those pointers are the cardinal neighbour
// of the node.
//
// This quadtree structure has been proposed by Safwan W. Qasem, King Saud
// University, Kingdom of Saudi Arabia, in his paper "Cardinal Neighbor
// Quadtree: a New Quadtree-based Structure for Constant-Time Neighbor Finding"
type CNTree struct {
	resolution int            // maximal resolution
	scanner    binimg.Scanner // reference image
	root       *CNNode        // root node
	leaves     NodeList       // leaf nodes (filled during creation)
	nLevels    uint           // maximum number of levels of the quadtree
}

// NewCNTree creates a cardinal neighbour quadtree and populates it.
//
// The quadtree is populated according to the content of the scanned image. It
// works only on square and power of 2 sized images, NewCNTree will return a
// non-nil error if that's not the case.
//
// resolution is the minimal dimension of a leaf node, no further subdivisions
// will be performed on a leaf if its dimension is equal to the resolution.
func NewCNTree(scanner binimg.Scanner, resolution int) (*CNTree, error) {
	if !binimg.IsPowerOf2Image(scanner) {
		return nil, errors.New("image must be a square with power-of-2 dimensions")
	}

	if resolution < 1 {
		return nil, errors.New("resolution must be greater than 0")
	}

	// To ensure a consistent behavior and eliminate corner cases,
	// the Quadtree's root node needs to have children. Thus, the
	// first instantiated cnNode needs to always be subdivided.
	// This condition asserts the resolution is respected.
	if scanner.Bounds().Dx() < resolution*2 {
		return nil, errors.New("the image size must be greater or equal to twice the resolution")
	}

	q := &CNTree{
		resolution: resolution,
		scanner:    scanner,
	}
	// given the resolution and the size, we can determine
	// the max number of levels in the tree
	q.computeNumLevels(scanner.Bounds().Dx())

	q.root = q.newNode(q.scanner.Bounds(), nil, rootQuadrant)
	q.subdivide(q.root)
	return q, nil
}

func (q *CNTree) newNode(bounds image.Rectangle, parent *CNNode, location Quadrant) *CNNode {
	n := &CNNode{
		color:    Gray,
		bounds:   bounds,
		parent:   parent,
		location: location,
		size:     bounds.Dx(),
	}

	uniform, col := q.scanner.Uniform(bounds)
	switch uniform {
	case true:
		// quadrant is uniform, won't need to subdivide any further
		if col == binimg.White {
			n.color = White
		} else {
			n.color = Black
		}
	case false:
		// if we reached maximal resolution..
		if n.size/2 < q.resolution {
			// ...make this node a black leaf, instead of gray
			n.color = Black
		}
	}

	// fills leaves slices
	if n.color != Gray {
		q.leaves = append(q.leaves, n)
	}
	return n
}

func (q *CNTree) subdivide(p *CNNode) {
	// Step 1: Decomposing the gray quadrant and updating the
	//         parent node following the Z-order traversal.

	//     x0   x1     x2
	//  y0 .----.-------.
	//     |    |       |
	//     | NW |  NE   |
	//     |    |       |
	//  y1 '----'-------'
	//     | SW |  SE   |
	//  y2 '----'-------'
	//

	x0 := p.bounds.Min.X
	x1 := p.bounds.Min.X + p.size/2
	x2 := p.bounds.Max.X

	y0 := p.bounds.Min.Y
	y1 := p.bounds.Min.Y + p.size/2
	y2 := p.bounds.Max.Y

	// decompose current node in 4 sub-quadrants
	nw := q.newNode(image.Rect(x0, y0, x1, y1), p, Northwest)
	ne := q.newNode(image.Rect(x1, y0, x2, y1), p, Northeast)
	sw := q.newNode(image.Rect(x0, y1, x1, y2), p, Southwest)
	se := q.newNode(image.Rect(x1, y1, x2, y2), p, Southeast)

	// at creation, each sub-quadrant first inherits its parent external neighbours
	nw.cn[West] = p.cn[West]   // inherited
	nw.cn[North] = p.cn[North] // inherited
	nw.cn[East] = ne           // set for decomposition, will be updated after
	nw.cn[South] = sw          // set for decomposition, will be updated after
	ne.cn[West] = nw           // set for decomposition, will be updated after
	ne.cn[North] = p.cn[North] // inherited
	ne.cn[East] = p.cn[East]   // inherited
	ne.cn[South] = se          // set for decomposition, will be updated after
	sw.cn[West] = p.cn[West]   // inherited
	sw.cn[North] = nw          // set for decomposition, will be updated after
	sw.cn[East] = se           // set for decomposition, will be updated after
	sw.cn[South] = p.cn[South] // inherited
	se.cn[West] = sw           // set for decomposition, will be updated after
	se.cn[North] = ne          // set for decomposition, will be updated after
	se.cn[East] = p.cn[East]   // inherited
	se.cn[South] = p.cn[South] // inherited

	p.c[Northwest] = nw
	p.c[Northeast] = ne
	p.c[Southwest] = sw
	p.c[Southeast] = se

	p.updateNorthEast()
	p.updateSouthWest()

	// update all neighbours accordingly. After the decomposition
	// of a quadrant, all its neighbors in the four directions
	// must be informed of the change so that they can update
	// their own cardinal neighbors accordingly.
	p.updateNeighbours()

	// subdivide non-leaf nodes
	if nw.color == Gray {
		q.subdivide(nw)
	}
	if ne.color == Gray {
		q.subdivide(ne)
	}
	if sw.color == Gray {
		q.subdivide(sw)
	}
	if se.color == Gray {
		q.subdivide(se)
	}
}

// Root returns the quadtree root node.
func (q *CNTree) Root() Node {
	return q.root
}

// ForEachLeaf calls the given function for each leaf node of the quadtree.
//
// Successive calls to the provided function are performed in no particular
// order. The color parameter allows to loop on the leaves of a particular
// color, Black or White.
// NOTE: As by definition, Gray leaves do not exist, passing Gray to
// ForEachLeaf should return all leaves, independently of their color.
func (q *CNTree) ForEachLeaf(color Color, fn func(Node)) {
	for _, n := range q.leaves {
		if color == Gray || n.Color() == color {
			fn(n)
		}
	}
}

// given the resolution, that is a power of 2, and the size, compute the
// maximum number of levels the quadtree can have
func (q *CNTree) computeNumLevels(size int) {
	q.nLevels = 1
	n := uint(size)
	for n&1 == 0 {
		n >>= 1
		if n < uint(q.resolution) {
			break
		}
		q.nLevels++
	}
}

// locate returns the Node that contains the given point, or nil.
func (q *CNTree) locate(pt image.Point) Node {
	// binary branching method assumes the point lies in the bounds
	b := q.root.bounds
	if !pt.In(b) {
		return nil
	}

	// apply affine transformations of the coordinate space, actually letting
	// the image square being defined over [0,1)²
	var (
		x, y float64
		bit  uint
		node *CNNode
		k    uint
	)

	// first, we multiply the position of the cell’s left corner by 2^ROOT_LEVEL
	// and then represent use product in binary form
	x = float64(pt.X-b.Min.X) / float64(b.Dx())
	y = float64(pt.Y-b.Min.Y) / float64(b.Dy())
	k = q.nLevels - 1
	ix := uint(x * math.Pow(2.0, float64(k)))
	iy := uint(y * math.Pow(2.0, float64(k)))

	// Now, following the branching pattern is just a matter of following, for
	// each level k in the tree, the branching indicated by the (k-1)st bit from
	// each of the x, y locational codes, it directly determines the index to
	// the appropriate child cell.  When the indexed child cell has no children,
	// the desired leaf cell has been reached and the operation is complete.
	node = q.root
	for node.color == Gray {
		k--
		bit = 1 << k
		childIdx := (ix&bit)>>k + ((iy&bit)>>k)<<1
		node = node.c[childIdx]
	}
	return node
}

// CNNode is a node of a Cardinal Neighbour Quadtree.
//
// It is an implementation of the Node interface, with additional fields and
// methods required to obtain the node neighbours in constant time. The time
// complexity reduction is obtained through the addition of only four pointers per
// leaf node in the quadtree.
//
// - The Western cardinal neighbor is the top-most neighbor node among the
//   western neighbors, noted cn0.
// - The Northern cardinal neighbor is the left-most neighbor node among the
//   northern neighbors, noted cn1.
// - The Eastern cardinal neighbor is the bottom-most neighbor node among the
//   eastern neighbors, noted cn2.
// - The Southern cardinal neighbor is the right-most neighbor node among the
//   southern neighbors, noted cn3.
type CNNode struct {
	parent   *CNNode         // pointer to the parent node
	c        [4]*CNNode      // children nodes
	cn       [4]*CNNode      // cardinal neighbours
	bounds   image.Rectangle // node bounds
	color    Color           // node color
	location Quadrant        // node location inside its parent
	size     int             // size of a quadrant side
}

// Parent returns the quadtree node that is the parent of current one.
func (n *CNNode) Parent() Node {
	if n.parent == nil {
		return nil
	}
	return n.parent
}

// Child returns current node child at specified quadrant.
func (n *CNNode) Child(q Quadrant) Node {
	if n.c[q] == nil {
		return nil
	}
	return n.c[q]
}

// Bounds returns the bounds of the rectangular area represented by this
// quadtree node.
func (n *CNNode) Bounds() image.Rectangle {
	return n.bounds
}

// Color returns the node Color.
func (n *CNNode) Color() Color {
	return n.color
}

// Location returns the node inside its parent quadrant
func (n *CNNode) Location() Quadrant {
	return n.location
}

func (n *CNNode) updateNorthEast() {
	if n.parent == nil || n.cn[North] == nil {
		// nothing to update as this quadrant lies on the north border
		return
	}
	// step 2.2: Updating Cardinal Neighbors of NE sub-Quadrant.
	if n.cn[North] != nil {
		if n.cn[North].size < n.size {
			c0 := n.c[Northwest]
			c0.cn[North] = n.cn[North]
			// to update C1, we perform a west-east traversal
			// recording the cumulative size of traversed nodes
			cur := c0.cn[North]
			cumsize := cur.size
			for cumsize < c0.size {
				cur = cur.cn[East]
				cumsize += cur.size
			}
			n.c[Northeast].cn[North] = cur
		}
	}
}

func (n *CNNode) updateSouthWest() {
	if n.parent == nil || n.cn[West] == nil {
		// nothing to update as this quadrant lies on the west border
		return
	}
	// step 2.1: Updating Cardinal Neighbors of SW sub-Quadrant.
	if n.cn[North] != nil {
		if n.cn[North].size < n.size {
			c0 := n.c[Northwest]
			c0.cn[North] = n.cn[North]
			// to update C2, we perform a north-south traversal
			// recording the cumulative size of traversed nodes
			cur := c0.cn[West]
			cumsize := cur.size
			for cumsize < c0.size {
				cur = cur.cn[South]
				cumsize += cur.size
			}
			n.c[Southwest].cn[West] = cur
		}
	}
}

// updateNeighbours updates all neighbours according to the current
// decomposition.
func (n *CNNode) updateNeighbours() {
	// On each direction, a full traversal of the neighbors
	// should be performed.  In every quadrant where a reference
	// to the parent quadrant is stored as the Cardinal Neighbor,
	// it should be replaced by one of its children created after
	// the decomposition

	if n.cn[West] != nil {
		n.forEachNeighbourInDirection(West, func(qn Node) {
			western := qn.(*CNNode)
			if western.cn[East] == n {
				if western.bounds.Max.Y > n.c[Southwest].bounds.Min.Y {
					// choose SW
					western.cn[East] = n.c[Southwest]
				} else {
					// choose NW
					western.cn[East] = n.c[Northwest]
				}
				if western.cn[East].bounds.Min.Y == western.bounds.Min.Y {
					western.cn[East].cn[West] = western
				}
			}
		})
	}

	if n.cn[North] != nil {
		n.forEachNeighbourInDirection(North, func(qn Node) {
			northern := qn.(*CNNode)
			if northern.cn[South] == n {
				if northern.bounds.Max.X > n.c[Northeast].bounds.Min.X {
					// choose NE
					northern.cn[South] = n.c[Northeast]
				} else {
					// choose NW
					northern.cn[South] = n.c[Northwest]
				}
				if northern.cn[South].bounds.Min.X == northern.bounds.Min.X {
					northern.cn[South].cn[North] = northern
				}
			}
		})
	}

	if n.cn[East] != nil {
		if n.cn[East] != nil && n.cn[East].cn[West] == n {
			// To update the eastern CN of a quadrant Q that is being
			// decomposed: Q.CN2.CN0=Q.Ch[NE]
			n.cn[East].cn[West] = n.c[Northeast]
		}
	}

	if n.cn[South] != nil {
		// To update the southern CN of a quadrant Q that is being
		// decomposed: Q.CN3.CN1=Q.Ch[SE]
		// TODO: there seems to be a typo in the paper.
		// should have read this instead: Q.CN3.CN1=Q.Ch[SW]
		if n.cn[South] != nil && n.cn[South].cn[North] == n {
			n.cn[South].cn[North] = n.c[Southwest]
		}
	}
}

// forEachNeighbourInDirection calls fn on every neighbour of the current node in the given
// direction.
func (n *CNNode) forEachNeighbourInDirection(dir Side, fn func(Node)) {
	// start from the cardinal neighbour on the given direction
	N := n.cn[dir]
	if N == nil {
		return
	}
	fn(N)
	if N.size >= n.size {
		return
	}

	traversal := traversal(dir)
	opposite := opposite(dir)
	// perform cardinal neighbour traversal
	for {
		N = N.cn[traversal]
		if N != nil && N.cn[opposite] == n {
			fn(N)
		} else {
			return
		}
	}
}

// forEachNeighbour calls the given function for each neighbour of current
// node.
func (n *CNNode) forEachNeighbour(fn func(Node)) {
	n.forEachNeighbourInDirection(West, fn)
	n.forEachNeighbourInDirection(North, fn)
	n.forEachNeighbourInDirection(East, fn)
	n.forEachNeighbourInDirection(South, fn)
}

// String returns a symbolic representation of the node with its cardinal
// neighbours. It is useful for debugging purposes only.
func (n *CNNode) String() string {
	var scn0, scn1, scn2, scn3 string
	if n.cn[West] != nil {
		scn0 = fmt.Sprintf("%v-%d", n.cn[West].bounds.Min, n.cn[West].size)
	}
	if n.cn[North] != nil {
		scn1 = fmt.Sprintf("%v-%d", n.cn[North].bounds.Min, n.cn[North].size)
	}
	if n.cn[East] != nil {
		scn2 = fmt.Sprintf("%v-%d", n.cn[East].bounds.Min, n.cn[East].size)
	}
	if n.cn[South] != nil {
		scn3 = fmt.Sprintf("%v-%d", n.cn[South].bounds.Min, n.cn[South].size)
	}
	return fmt.Sprintf("[%v-%d-%s|CN ←%v ↑%v →%v ↓%v]", n.bounds.Min, n.size, n.color, scn0, scn1, scn2, scn3)
}
