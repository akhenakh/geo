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

import (
	"math/rand"
	"testing"
)

// Shape interface enforcement
var (
	_ Shape = (*edgeVectorShape)(nil)
)

// edgeVectorShape is a Shape representing an arbitrary set of edges. It
// is used for testing, but it can also be useful if you have, say, a
// collection of polylines and don't care about memory efficiency (since
// this type would store most of the vertices twice).
type edgeVectorShape struct {
	edges []Edge
}

// edgeVectorShapeFromPoints returns an edgeVectorShape of length 1 from the given points.
func edgeVectorShapeFromPoints(a, b Point) *edgeVectorShape {
	e := &edgeVectorShape{
		edges: []Edge{
			{a, b},
		},
	}
	return e
}

// Add adds the given edge to the shape.
func (e *edgeVectorShape) Add(a, b Point) {
	e.edges = append(e.edges, Edge{a, b})
}
func (e *edgeVectorShape) NumEdges() int                          { return len(e.edges) }
func (e *edgeVectorShape) Edge(id int) Edge                       { return e.edges[id] }
func (e *edgeVectorShape) ReferencePoint() ReferencePoint         { return OriginReferencePoint(false) }
func (e *edgeVectorShape) NumChains() int                         { return len(e.edges) }
func (e *edgeVectorShape) Chain(chainID int) Chain                { return Chain{chainID, 1} }
func (e *edgeVectorShape) ChainEdge(chainID, offset int) Edge     { return e.edges[chainID] }
func (e *edgeVectorShape) ChainPosition(edgeID int) ChainPosition { return ChainPosition{edgeID, 0} }
func (e *edgeVectorShape) IsEmpty() bool                          { return defaultShapeIsEmpty(e) }
func (e *edgeVectorShape) IsFull() bool                           { return defaultShapeIsFull(e) }
func (e *edgeVectorShape) Dimension() int                         { return 1 }
func (e *edgeVectorShape) typeTag() typeTag                       { return typeTagNone }
func (e *edgeVectorShape) privateInterface()                      {}

func TestEdgeVectorShapeEmpty(t *testing.T) {
	shape := NewEdgeVectorShape()
	if shape.NumEdges() != 0 {
		t.Errorf("NumEdges() = %d, want 0", shape.NumEdges())
	}
	if shape.NumChains() != 0 {
		t.Errorf("NumChains() = %d, want 0", shape.NumChains())
	}
	if shape.Dimension() != 1 {
		t.Errorf("Dimension() = %d, want 1", shape.Dimension())
	}
	if !shape.IsEmpty() {
		t.Error("IsEmpty() = false, want true")
	}
	if shape.IsFull() {
		t.Error("IsFull() = true, want false")
	}
	if shape.ReferencePoint().Contained {
		t.Error("ReferencePoint().Contained = true, want false")
	}
}

func TestEdgeVectorShapeEdgeAccess(t *testing.T) {
	shape := NewEdgeVectorShape()
	rng := rand.New(rand.NewSource(12345))
	const numEdges = 100
	var edges []Edge
	for i := 0; i < numEdges; i++ {
		a := randomPoint(rng)
		b := randomPoint(rng)
		edges = append(edges, Edge{a, b})
		shape.Add(a, b)
	}

	if shape.NumEdges() != numEdges {
		t.Errorf("NumEdges() = %d, want %d", shape.NumEdges(), numEdges)
	}
	if shape.NumChains() != numEdges {
		t.Errorf("NumChains() = %d, want %d", shape.NumChains(), numEdges)
	}
	if shape.Dimension() != 1 {
		t.Errorf("Dimension() = %d, want 1", shape.Dimension())
	}
	if shape.IsEmpty() {
		t.Error("IsEmpty() = true, want false")
	}
	if shape.IsFull() {
		t.Error("IsFull() = true, want false")
	}

	for i := 0; i < numEdges; i++ {
		if shape.Chain(i).Start != i {
			t.Errorf("Chain(%d).Start = %d, want %d", i, shape.Chain(i).Start, i)
		}
		if shape.Chain(i).Length != 1 {
			t.Errorf("Chain(%d).Length = %d, want 1", i, shape.Chain(i).Length)
		}
		edge := shape.Edge(i)
		if edge != edges[i] {
			t.Errorf("Edge(%d) = %v, want %v", i, edge, edges[i])
		}
	}
}

func TestEdgeVectorShapeSingletonConstructor(t *testing.T) {
	a := PointFromCoords(1, 0, 0)
	b := PointFromCoords(0, 1, 0)
	shape := NewEdgeVectorShapeFromEdge(a, b)

	if shape.NumEdges() != 1 {
		t.Errorf("NumEdges() = %d, want 1", shape.NumEdges())
	}
	if shape.NumChains() != 1 {
		t.Errorf("NumChains() = %d, want 1", shape.NumChains())
	}
	if shape.IsEmpty() {
		t.Error("IsEmpty() = true, want false")
	}
	if shape.IsFull() {
		t.Error("IsFull() = true, want false")
	}
	edge := shape.Edge(0)
	if edge.V0 != a {
		t.Errorf("Edge(0).V0 = %v, want %v", edge.V0, a)
	}
	if edge.V1 != b {
		t.Errorf("Edge(0).V1 = %v, want %v", edge.V1, b)
	}
}
