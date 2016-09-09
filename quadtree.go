// Package quadtree provides the data structures and algorithms required to
// perform an efficient pathfinding on a 2 dimensional discrete space.
//
// Principle:
// A black and white image is recursively subdivided into four quadrants or
// regions. The regions may be rectangular or square, depending on the source
// image, and the subdvision takes place as long as every region is not
// homegeneous (i.e contains black and white pixels), or until the resolution
// has been reached. The minimal resolution is 1 but the user may provides a
// higher resolution.
//
// Quadrants and sides:
//
//         North
//      .----.----.
//      | NW | NE |
// West '----'----' East
//      | SW | SE |
//      '----'----'
//         South
//
//
// Usage:
// The Init function must be called before using the package.
//
package quadtree

import (
	"errors"

	"github.com/RookieGameDevs/quadtree/bmp"
)

type PostNodeCreationFunc func(n *quadnode)

func noop(n *quadnode) {
}

type Quadtree struct {
	root *quadnode // the root node
}

func NewQuadtreeFromBitmap(bm *bmp.Bitmap, resolution int, fn PostNodeCreationFunc) (*Quadtree, error) {
	// To ensure a consistent behavior and eliminate corner cases, the
	// Quadtree's root node need to have children, i.e. it can't
	// be a leaf node. Thus, the first instantiated Quadnode need to
	// always be subdivided. These two conditions make sure that
	// even with this subdivision the resolution will be respected.
	if resolution < 1 {
		return nil, errors.New("resolution must be greater than 0")
	}
	minDim := bm.Width
	if bm.Height < minDim {
		minDim = bm.Height
	}
	if minDim < resolution*2 {
		return nil, errors.New("the bitmap smaller dimension must be greater or equal to twice the resolution")
	}

	if fn == nil {
		fn = noop
	}

	quad := &Quadtree{}
	quad.root = newRootQuadNode(bm, resolution, fn)
	return quad, nil
}
