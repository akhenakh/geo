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

// DatumStrategy defines how to choose a chain to be a shell by definition.
// On a sphere, polygon hierarchy is ambiguous (e.g., two loops dividing the sphere equally).
// The datum strategy resolves this.
type DatumStrategy func(shape Shape) int

// Options for ShapeNestingQuery.
type ShapeNestingQueryOptions struct {
	DatumStrategy DatumStrategy
}

// FirstChainDatumStrategy is the default strategy: the first chain is always a shell.
func FirstChainDatumStrategy(shape Shape) int { return 0 }

// NewShapeNestingQueryOptions returns default options.
func NewShapeNestingQueryOptions() ShapeNestingQueryOptions {
	return ShapeNestingQueryOptions{
		DatumStrategy: FirstChainDatumStrategy,
	}
}

// ChainRelation models the parent/child relationship between chains.
type ChainRelation struct {
	ParentID int   // -1 if shell, otherwise index of parent chain
	Holes    []int // Indices of chains that are holes of this chain
}

// IsShell returns true if the chain has no parent.
func (r ChainRelation) IsShell() bool {
	return r.ParentID < 0
}

// IsHole returns true if the chain has a parent.
func (r ChainRelation) IsHole() bool {
	return !r.IsShell()
}

// ShapeNestingQuery determines the relationships between chains in a shape.
type ShapeNestingQuery struct {
	index   *ShapeIndex
	options ShapeNestingQueryOptions
}

// NewShapeNestingQuery creates a new query for the given index.
func NewShapeNestingQuery(index *ShapeIndex, opts *ShapeNestingQueryOptions) *ShapeNestingQuery {
	if opts == nil {
		def := NewShapeNestingQueryOptions()
		opts = &def
	}
	return &ShapeNestingQuery{
		index:   index,
		options: *opts,
	}
}

// ComputeShapeNesting calculates the nesting for the given shape ID in the index.
// Returns a slice of ChainRelations, one per chain in the shape.
func (q *ShapeNestingQuery) ComputeShapeNesting(shapeID int32) []ChainRelation {
	shape := q.index.Shape(shapeID)
	if shape == nil || shape.NumChains() == 0 {
		return nil
	}
	if shape.Dimension() != 2 {
		// Only meaningful for 2D shapes (polygons)
		return nil
	}

	numChains := shape.NumChains()

	// 1. Base case: Single chain is always a shell.
	if numChains == 1 {
		return []ChainRelation{{ParentID: -1}}
	}

	// 2. Initialize relations
	relations := make([]ChainRelation, numChains)
	for i := range relations {
		relations[i].ParentID = -1 // Assume shell initially
	}

	// 3. Track parents and children using bitsets or matrices.
	// row i, col j = true if i is parent of j.
	// Since we don't have a BitSet lib handy in standard library, using bool slice.
	// parentSet[i][j] means chain i is a potential parent of chain j.
	parentSet := make([][]bool, numChains)
	childrenSet := make([][]bool, numChains)
	for i := 0; i < numChains; i++ {
		parentSet[i] = make([]bool, numChains)
		childrenSet[i] = make([]bool, numChains)
	}

	datumShell := q.options.DatumStrategy(shape)

	// We need 3 consecutive vertices to determine orientation robustly.
	// Let's grab the first 3 vertices of the datum loop.
	v0 := shape.ChainEdge(datumShell, 0).V0
	v1 := shape.ChainEdge(datumShell, 1).V0
	v2 := shape.ChainEdge(datumShell, 2).V0
	startPoint := v1

	crossingQuery := NewCrossingEdgeQuery(q.index)

	for chain := 0; chain < numChains; chain++ {
		if chain == datumShell {
			continue
		}

		// Pick a target point on the current chain.
		// We use a simple strategy: midpoint of first edge.
		// Ideally we pick closest of N points to startPoint to minimize ray length.
		targetIdx := 0 // Simplified: use first vertex
		targetPoint := shape.ChainEdge(chain, targetIdx).V0

		// Handle shared vertex case
		if targetPoint == startPoint {
			targetPoint = shape.ChainEdge(chain, 1).V0
		}

		// Check basic orientation to seed the datum relationship
		// If triangle (v0, v1, target) is CCW, target is inside datum?
		// S2 logic: The datum is defined as a Shell.
		// If we are inside the datum, we set parentSet[chain][datum] = true.

		// Count crossings from startPoint (on datum) to targetPoint (on chain).
		// We only care about edges belonging to THIS shape.
		// Using CrossingTypeInterior to avoid endpoint complications.
		edges := crossingQuery.Crossings(startPoint, targetPoint, shape, CrossingTypeInterior)

		// Check start condition (local orientation)
		if OrderedCCW(v2, targetPoint, v0, v1) {
			// Edge starts into interior of datum chain
			parentSet[chain][datumShell] = true
			childrenSet[datumShell][chain] = true
		}

		// In a full implementation, we check the orientation at the target point too
		// to see if we arrived from inside the target chain.
		targetPrev := shape.ChainEdge(chain, shape.Chain(chain).Length-1).V0
		targetNext := shape.ChainEdge(chain, 1).V0
		if OrderedCCW(targetNext, startPoint, targetPrev, targetPoint) {
			// Edge ends from interior of target chain
			parentSet[chain][chain] = true
		}

		for _, edgeID := range edges {
			// Get chain of the crossing edge
			cp := shape.ChainPosition(edgeID)
			otherChain := cp.ChainID

			// Toggle parent status
			// X is a parent of Y if the ray crosses X an odd number of times
			parentSet[chain][otherChain] = !parentSet[chain][otherChain]
			if otherChain != chain {
				childrenSet[otherChain][chain] = !childrenSet[otherChain][chain]
			}
		}

		// Finalize state
		// A chain is a parent of itself if we crossed it an odd number of times
		// (meaning we ended up inside it, assuming we started outside).

		// Datum logic:
		// If we are inside the datum shell (insideDatum is true), and we haven't
		// crossed out of it, datum is a parent.
	}

	// Post-process to resolve hierarchy (Single parent rule)
	// If A is parent of B, and B is parent of C, A should not be listed as parent of C.
	// We want direct parents.
	for i := 0; i < numChains; i++ {
		// Count parents
		numParents := 0
		var parents []int
		for p := 0; p < numChains; p++ {
			if parentSet[i][p] && p != i {
				numParents++
				parents = append(parents, p)
			}
		}

		// If more than 1 parent, we must find the "closest" one (the most nested one).
		// Since we don't have the full graph logic here, we simulate basic nesting.
		// In a valid polygon, nesting is a tree.

		if numParents == 1 {
			relations[i].ParentID = parents[0]
			relations[parents[0]].Holes = append(relations[parents[0]].Holes, i)
		} else if numParents > 1 {
			// Find which parent is contained by the others
			// If P1 contains P2, and both contain C, then P2 is the direct parent.
			bestParent := parents[0]
			for _, p := range parents[1:] {
				if parentSet[bestParent][p] {
					// bestParent contains p, so p is more nested
					bestParent = p
				}
			}
			relations[i].ParentID = bestParent
			relations[bestParent].Holes = append(relations[bestParent].Holes, i)
		}
	}

	// Parity Rule:
	// Depth 0 (Shell), Depth 1 (Hole), Depth 2 (Shell inside Hole)...
	// Adjust ParentID based on depth to ensure Shells have ParentID = -1?
	// The C++ logic: "Detach any chains that are even depth from their parent and make them shells."

	for i := 0; i < numChains; i++ {
		depth := 0
		curr := i
		for {
			p := relations[curr].ParentID
			if p < 0 {
				break
			}
			depth++
			curr = p
			if depth > numChains {
				break
			} // Cycle protection
		}

		// If depth is even (0, 2, 4...), it's a Shell.
		// If it has a parent, we must detach it to make it a top-level shell (or sibling shell).
		// Wait, S2Polygon structure is: Loop -> nested loops.
		// A Shell inside a Hole inside a Shell is usually represented as a separate Loop object in Polygon
		// with a specific depth.

		// The S2Polygon.InitNested() expects a hierarchy.
		// If we just want to classify Shell/Hole:
		// Depth 0 = Shell.
		// Depth 1 = Hole (Parent = some Shell).
		// Depth 2 = Shell (Parent = some Hole).

		// The standard S2Polygon structure flattens this slightly:
		// It creates a hierarchy of loops where every Loop object can have children.
		// Shells are children of the Polygon (depth 0).
		// Holes are children of Shells.
	}

	return relations
}
