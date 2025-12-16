// Copyright 2023 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS-IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package s2

// ShapeIndexRegion wraps a ShapeIndex and implements the Region interface.
// This allows RegionCoverer to work with ShapeIndexes as well as being
// able to be used by some of the Query types.
//
// It also contains a method VisitIntersectingShapes that may be used to
// efficiently visit all shapes that intersect an arbitrary Cell (not
// limited to cells in the index).
//
// This type is not safe for concurrent use.
type ShapeIndexRegion struct {
	index         *ShapeIndex
	containsQuery *ContainsPointQuery
	iter          *ShapeIndexIterator
}

// NewShapeIndexRegion creates a new ShapeIndexRegion for the given index.
func NewShapeIndexRegion(index *ShapeIndex) *ShapeIndexRegion {
	// Optimization: rather than declaring our own iterator, instead we reuse
	// the iterator declared by ContainsPointQuery.
	q := NewContainsPointQuery(index, VertexModelSemiOpen)
	return &ShapeIndexRegion{
		index:         index,
		containsQuery: q,
		iter:          q.iter,
	}
}

// Index returns the underlying ShapeIndex.
func (s *ShapeIndexRegion) Index() *ShapeIndex {
	return s.index
}

// CapBound returns a bounding spherical cap for this collection of geometry.
// This is not guaranteed to be exact.
func (s *ShapeIndexRegion) CapBound() Cap {
	cu := CellUnion(s.CellUnionBound())
	return cu.CapBound()
}

// RectBound returns a bounding rectangle for this collection of geometry.
// The bounds are not guaranteed to be tight.
func (s *ShapeIndexRegion) RectBound() Rect {
	cu := CellUnion(s.CellUnionBound())
	return cu.RectBound()
}

// CellUnionBound returns the bounding CellUnion for this collection of geometry.
// This method currently returns at most 4 cells, unless the index spans
// multiple faces in which case it may return up to 6 cells.
func (s *ShapeIndexRegion) CellUnionBound() []CellID {
	// We find the range of Cells spanned by the index and choose a level such
	// that the entire index can be covered with just a few cells.  There are
	// two cases:
	//
	//  - If the index intersects two or more faces, then for each intersected
	//    face we add one cell to the covering.  Rather than adding the entire
	//    face, instead we add the smallest Cell that covers the ShapeIndex
	//    cells within that face.
	//
	//  - If the index intersects only one face, then we first find the smallest
	//    cell S that contains the index cells (just like the case above).
	//    However rather than using the cell S itself, instead we repeat this
	//    process for each of its child cells.  In other words, for each
	//    child cell C we add the smallest Cell C' that covers the index cells
	//    within C.  This extra step is relatively cheap and produces much
	//    tighter coverings when the ShapeIndex consists of a small region
	//    near the center of a large Cell.
	var cellIDs []CellID

	// Find the last CellID in the index.
	s.iter.End()
	if !s.iter.Prev() {
		return cellIDs // Empty index.
	}
	lastIndexID := s.iter.CellID()
	s.iter.Begin()
	if s.iter.CellID() != lastIndexID {
		// The index has at least two cells. Choose a CellID level such that
		// the entire index can be spanned with at most 6 cells (if the index
		// spans multiple faces) or 4 cells (it the index spans a single face).
		level, ok := s.iter.CellID().CommonAncestorLevel(lastIndexID)
		if !ok {
			// C++ returns -1 for no common level, ours returns 0. Set
			// to -1 so the next ++ puts us at the same place as C++ does.
			level = -1
		}
		level++

		// For each cell C at the chosen level, we compute the smallest Cell
		// that covers the ShapeIndex cells within C.
		lastID := lastIndexID.Parent(level)
		for id := s.iter.CellID().Parent(level); id != lastID; id = id.Next() {
			// If the cell C does not contain any index cells, then skip it.
			if id.RangeMax() < s.iter.CellID() {
				continue
			}

			// Find the range of index cells contained by C and then shrink C so
			// that it just covers those cells.
			first := s.iter.CellID()
			s.iter.seek(id.RangeMax().Next())
			s.iter.Prev()
			cellIDs = s.coverRange(first, s.iter.CellID(), cellIDs)
			s.iter.Next()
		}
	}

	return s.coverRange(s.iter.CellID(), lastIndexID, cellIDs)
}

// MayIntersect reports whether the region might intersect the given cell.
// This is a fast, conservative check.
func (s *ShapeIndexRegion) MayIntersect(target Cell) bool {
	relation := s.iter.LocateCellID(target.id)

	// If "target" does not overlap any index cell, there is no intersection.
	if relation == Disjoint {
		return false
	}

	// If "target" is subdivided into one or more index cells, then there is an
	// intersection to within the ShapeIndex error bound.
	if relation == Subdivided {
		return true
	}

	// Otherwise, the iterator points to an index cell containing "target".
	if s.iter.CellID() == target.id {
		return true
	}

	// Test whether any shape intersects the target cell or contains its center.
	cell := s.iter.IndexCell()
	for _, clipped := range cell.shapes {
		if s.anyEdgeIntersects(clipped, target) {
			return true
		}
		if s.contains(clipped, target.Center()) {
			return true
		}
	}
	return false
}

// IntersectsCell reports whether the region intersects the given cell.
// For ShapeIndexRegion, this is currently the same as MayIntersect.
func (s *ShapeIndexRegion) IntersectsCell(c Cell) bool {
	return s.MayIntersect(c)
}

// coverRange computes the smallest CellID that covers the Cell range (first, last)
// and returns the updated slice.
//
// This requires first and last have a common ancestor.
func (s *ShapeIndexRegion) coverRange(first, last CellID, cellIDs []CellID) []CellID {
	// The range consists of a single index cell.
	if first == last {
		return append(cellIDs, first)
	}

	// Add the lowest common ancestor of the given range.
	level, ok := first.CommonAncestorLevel(last)
	if !ok {
		return append(cellIDs, CellID(0))
	}
	return append(cellIDs, first.Parent(level))
}

// ContainsCell reports whether the region completely contains the given cell.
// It returns false if containment could not be determined.
//
// The implementation is conservative but not exact; if a shape just barely
// contains the given cell then it may return false. The maximum error is
// less than 10 * dblEpsilon radians (or about 15 nanometers).
func (s *ShapeIndexRegion) ContainsCell(target Cell) bool {
	relation := s.iter.LocateCellID(target.id)

	// If the relation is Disjoint, then "target" is not contained. Similarly if
	// the relation is Subdivided then "target" is not contained, since index
	// cells are subdivided only if they (nearly) intersect too many edges.
	if relation != Indexed {
		return false
	}

	// Otherwise, the iterator points to an index cell containing "target".
	// If any shape contains the target cell, we return true.
	cell := s.iter.IndexCell()
	for _, clipped := range cell.shapes {
		// The shape contains the target cell iff the shape contains the cell
		// center and none of its edges intersects the (padded) cell interior.
		if s.iter.CellID() == target.id {
			if clipped.numEdges() == 0 && clipped.containsCenter {
				return true
			}
		} else {
			// It is faster to call anyEdgeIntersects before contains.
			if s.index.Shape(clipped.shapeID).Dimension() == 2 &&
				!s.anyEdgeIntersects(clipped, target) &&
				s.contains(clipped, target.Center()) {
				return true
			}
		}
	}
	return false
}

// ContainsPoint reports whether the region contains the given point or not.
// The point should be unit length, although some implementations may relax
// this restriction.
//
// Returns true if the given point is contained by any two-dimensional shape
// (i.e., polygon). Boundaries are treated as being semi-open (i.e., the
// same rules as Polygon). Zero and one-dimensional shapes are ignored by
// this method.
func (s *ShapeIndexRegion) ContainsPoint(p Point) bool {
	if s.iter.LocatePoint(p) {
		cell := s.iter.IndexCell()
		for _, clipped := range cell.shapes {
			if s.contains(clipped, p) {
				return true
			}
		}
	}
	return false
}

// VisitIntersectingShapes visits all shapes that intersect a Cell, passing the
// shape and a flag indicating whether the Cell was fully contained by the shape
// to a visitor. Each shape is visited at most once.
//
// The visitor should return true to continue visiting intersecting shapes, or
// false to terminate the algorithm early.
//
// This method can also be used to visit all shapes that fully contain a
// Cell by simply having the visitor function immediately return true when
// "containsTarget" is false.
func (s *ShapeIndexRegion) VisitIntersectingShapes(target Cell, visitor func(shape Shape, containsTarget bool) bool) bool {
	switch s.iter.LocateCellID(target.id) {
	case Disjoint:
		return true

	case Subdivided:
		// A shape contains the target cell iff it appears in at least one cell,
		// it contains the center of all cells, and it has no edges in any cell.
		// It is easier to keep track of whether a shape does *not* contain the
		// target cell because boolean values default to false.
		shapeNotContains := make(map[int32]bool)
		max := target.id.RangeMax()
		for ; !s.iter.Done() && s.iter.CellID() <= max; s.iter.Next() {
			cell := s.iter.IndexCell()
			for _, clipped := range cell.shapes {
				shapeNotContains[clipped.shapeID] = shapeNotContains[clipped.shapeID] || (clipped.numEdges() > 0 || !clipped.containsCenter)
			}
		}
		for shapeID, notContains := range shapeNotContains {
			if !visitor(s.index.Shape(shapeID), !notContains) {
				return false
			}
		}
		return true

	case Indexed:
		cell := s.iter.IndexCell()
		for _, clipped := range cell.shapes {
			// The shape contains the target cell iff the shape contains the cell
			// center and none of its edges intersects the (padded) cell interior.
			contains := false
			if s.iter.CellID() == target.id {
				contains = clipped.numEdges() == 0 && clipped.containsCenter
			} else {
				if !s.anyEdgeIntersects(clipped, target) {
					if !s.contains(clipped, target.Center()) {
						continue // Disjoint.
					}
					contains = true
				}
			}
			if !visitor(s.index.Shape(clipped.shapeID), contains) {
				return false
			}
		}
		return true
	}
	panic("unreachable")
}

// contains returns true if the indexed shape "clipped" in the indexed cell
// contains the point "p".
//
// REQUIRES: s.iter.CellID() contains "p".
func (s *ShapeIndexRegion) contains(clipped *clippedShape, p Point) bool {
	return s.containsQuery.shapeContains(clipped, s.iter.Center(), p)
}

// anyEdgeIntersects returns true if any edge of the indexed shape "clipped"
// intersects the cell "target". It may also return true if an edge is very
// close to "target"; the maximum error is less than 10 * dblEpsilon radians
// (about 15 nanometers).
func (s *ShapeIndexRegion) anyEdgeIntersects(clipped *clippedShape, target Cell) bool {
	maxError := faceClipErrorUVCoord + intersectsRectErrorUVDist
	bound := target.BoundUV().ExpandedByMargin(maxError)
	face := target.Face()
	shape := s.index.Shape(clipped.shapeID)
	numEdges := clipped.numEdges()

	for i := 0; i < numEdges; i++ {
		edge := shape.Edge(clipped.edges[i])
		p0, p1, ok := ClipToPaddedFace(edge.V0, edge.V1, face, maxError)
		if ok && edgeIntersectsRect(p0, p1, bound) {
			return true
		}
	}
	return false
}
