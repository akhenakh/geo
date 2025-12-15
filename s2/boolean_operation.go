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
	"sort"

	"github.com/golang/geo/s1"
)

// BooleanOperationOpType represents the type of boolean operation.
type BooleanOperationOpType int

const (
	BooleanOperationOpTypeUnion BooleanOperationOpType = iota
	BooleanOperationOpTypeIntersection
	BooleanOperationOpTypeDifference
	BooleanOperationOpTypeSymmetricDifference
)

// BooleanOperation computes boolean operations (Union, Intersection, etc.) on regions.
type BooleanOperation struct {
	OpType  BooleanOperationOpType
	Layers  []BuilderLayer
	Options BooleanOperationOptions
}

type BooleanOperationOptions struct {
	SnapFunction       SnapFunction
	SplitCrossingEdges bool
}

// NewBooleanOperation creates a new operation.
func NewBooleanOperation(opType BooleanOperationOpType, layer BuilderLayer, opts *BooleanOperationOptions) *BooleanOperation {
	if opts == nil {
		opts = &BooleanOperationOptions{
			SnapFunction:       NewIdentitySnapFunction(s1.Angle(0)),
			SplitCrossingEdges: true,
		}
	}
	return &BooleanOperation{
		OpType:  opType,
		Layers:  []BuilderLayer{layer},
		Options: *opts,
	}
}

// Build executes the operation.
func (b *BooleanOperation) Build(indexA, indexB *ShapeIndex) error {
	builder := NewBuilder(BuilderOptions{
		SnapFunction:       b.Options.SnapFunction,
		SplitCrossingEdges: b.Options.SplitCrossingEdges,
	})

	for _, layer := range b.Layers {
		builder.StartLayer(layer)
	}

	// We process the boundaries of A against B, and B against A.
	// The CrossingProcessor logic determines which parts of the edges are kept.

	// Process Region A edges against Region B
	if err := b.processRegion(builder, indexA, indexB, false); err != nil {
		return err
	}

	// Process Region B edges against Region A
	if err := b.processRegion(builder, indexB, indexA, true); err != nil {
		return err
	}

	return builder.Build()
}

// processRegion iterates over all edges in queryIndex, finds crossings with refIndex,
// splits the edges, and adds the relevant segments to the builder based on the OpType.
func (b *BooleanOperation) processRegion(builder *Builder, queryIndex, refIndex *ShapeIndex, processingB bool) error {
	// Query to find crossings against the reference index.
	crosser := NewCrossingEdgeQuery(refIndex)

	// Query to check containment of vertices in the reference index.
	// Using VertexModelSemiOpen matches standard S2 Polygon behavior.
	containsQuery := NewContainsPointQuery(refIndex, VertexModelSemiOpen)

	// Iterate over all shapes in the query index.
	for _, shape := range queryIndex.shapes {
		if shape == nil {
			continue
		}
		// Iterate over all chains (loops/polylines) in the shape.
		for chainID := 0; chainID < shape.NumChains(); chainID++ {
			chain := shape.Chain(chainID)

			// For Polygons (loops), we need to know the initial state (Inside/Outside).
			// We test the start vertex of the first edge.
			if chain.Length == 0 {
				continue
			}

			startEdge := shape.ChainEdge(chainID, 0)

			// Initial state: Is the start of the chain inside the reference region?
			inside := containsQuery.Contains(startEdge.V0)

			// Iterate over edges in the chain
			for i := 0; i < chain.Length; i++ {
				edge := shape.ChainEdge(chainID, i)

				// Find all crossings for this edge against the reference index.
				// CrossingTypeInterior ignores shared vertices, which is usually what we want for splitting.
				// However, robust boolean ops usually need CrossingTypeAll to handle vertex intersections explicitly.
				// For this simplified port, Interior captures the split points.
				crossingMap := crosser.CrossingsEdgeMap(edge.V0, edge.V1, CrossingTypeInterior)

				// Collect intersection points
				var intersections []Point
				for crossShape, edgeIDs := range crossingMap {
					for _, crossEdgeID := range edgeIDs {
						crossEdge := crossShape.Edge(crossEdgeID)
						// Calculate exact intersection point
						pt := Intersection(edge.V0, edge.V1, crossEdge.V0, crossEdge.V1)
						intersections = append(intersections, pt)
					}
				}

				// Sort intersections by distance from edge.V0 to handle multiple crossings correctly
				sortIntersections(edge.V0, intersections)

				// Emit segments
				currPt := edge.V0
				for _, nextPt := range intersections {
					// Add segment if valid
					if !currPt.ApproxEqual(nextPt) {
						if b.shouldEmit(inside, processingB) {
							builder.AddEdge(currPt, nextPt)
						}
					}

					// Update state and current point
					// Each crossing toggles the inside/outside state
					inside = !inside
					currPt = nextPt
				}

				// Add final segment from last intersection to edge.V1
				if !currPt.ApproxEqual(edge.V1) {
					if b.shouldEmit(inside, processingB) {
						builder.AddEdge(currPt, edge.V1)
					}
				}
			}
		}
	}
	return nil
}

// shouldEmit determines if a segment should be output based on the operation type
// and whether the segment is "inside" the other region.
// processingB indicates if we are processing edges from the second region (B).
func (b *BooleanOperation) shouldEmit(inside bool, processingB bool) bool {
	switch b.OpType {
	case BooleanOperationOpTypeUnion:
		// A|B: Keep A if outside B, Keep B if outside A.
		return !inside
	case BooleanOperationOpTypeIntersection:
		// A&B: Keep A if inside B, Keep B if inside A.
		return inside
	case BooleanOperationOpTypeDifference:
		// A-B: Keep A if outside B. Keep B if inside A (as boundary of hole)?
		// Standard difference usually keeps A's boundary outside B, and B's boundary inside A (reversed).
		if processingB {
			return inside // B's boundary inside A becomes part of the result's hole
		}
		return !inside // A's boundary outside B is kept
	case BooleanOperationOpTypeSymmetricDifference:
		// (A-B) | (B-A): Keep A if outside B, Keep B if outside A.
		return !inside
	}
	return false
}

// sortIntersections sorts points based on distance from start.
func sortIntersections(start Point, points []Point) {
	sort.Slice(points, func(i, j int) bool {
		di := ChordAngleBetweenPoints(start, points[i])
		dj := ChordAngleBetweenPoints(start, points[j])
		return di < dj
	})
}
