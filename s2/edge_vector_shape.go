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

// EdgeVectorShape is an Shape representing an arbitrary set of edges. It
// is mainly used for testing, but it can also be useful if you have, say, a
// collection of polylines and don't care about memory efficiency (since this
// class would store most of the vertices twice).
//
// Note that if you already have data stored in an Loop, Polyline, or
// Polygon, then you would be better off using the Shape method defined
// within those classes (e.g., Loop.Shape).  Similarly, if the vertex data
// is stored in your own data structures, you can easily write your own
// implementation of Shape that points to the existing vertex data rather than
// copying it.
type EdgeVectorShape struct {
	edges []Edge
}

// NewEdgeVectorShape creates a new EdgeVectorShape.
func NewEdgeVectorShape() *EdgeVectorShape {
	return &EdgeVectorShape{}
}

// NewEdgeVectorShapeFromEdge creates an EdgeVectorShape containing a single edge.
func NewEdgeVectorShapeFromEdge(a, b Point) *EdgeVectorShape {
	return &EdgeVectorShape{
		edges: []Edge{{a, b}},
	}
}

// Add adds an edge to the shape.
//
// IMPORTANT: This method should only be called *before* adding the
// EdgeVectorShape to an ShapeIndex.  Shapes can only be modified by
// removing them from the index, making changes, and adding them back again.
func (s *EdgeVectorShape) Add(a, b Point) {
	s.edges = append(s.edges, Edge{a, b})
}

// NumEdges returns the number of edges in this shape.
func (s *EdgeVectorShape) NumEdges() int {
	return len(s.edges)
}

// Edge returns the edge for the given edge index.
func (s *EdgeVectorShape) Edge(i int) Edge {
	return s.edges[i]
}

// Dimension returns the dimension of the geometry represented by this shape.
func (s *EdgeVectorShape) Dimension() int {
	return 1
}

// ReferencePoint returns the reference point for this shape.
func (s *EdgeVectorShape) ReferencePoint() ReferencePoint {
	return OriginReferencePoint(false)
}

// NumChains reports the number of contiguous edge chains in the shape.
func (s *EdgeVectorShape) NumChains() int {
	return len(s.edges)
}

// Chain returns the i-th edge chain in the Shape.
func (s *EdgeVectorShape) Chain(chainID int) Chain {
	return Chain{chainID, 1}
}

// ChainEdge returns the j-th edge of the i-th edge chain.
func (s *EdgeVectorShape) ChainEdge(chainID, offset int) Edge {
	return s.edges[chainID]
}

// ChainPosition finds the chain containing the given edge, and returns the
// position of that edge as a ChainPosition(chainID, offset) pair.
func (s *EdgeVectorShape) ChainPosition(edgeID int) ChainPosition {
	return ChainPosition{edgeID, 0}
}

// IsEmpty reports whether the Shape contains no points.
func (s *EdgeVectorShape) IsEmpty() bool {
	return defaultShapeIsEmpty(s)
}

// IsFull reports whether the Shape contains all points on the sphere.
func (s *EdgeVectorShape) IsFull() bool {
	return defaultShapeIsFull(s)
}

func (s *EdgeVectorShape) typeTag() typeTag {
	return typeTagNone
}

func (s *EdgeVectorShape) privateInterface() {}
