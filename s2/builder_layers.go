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

// BuilderLayer interface.
type BuilderLayer interface {
	GraphOptions() BuilderGraphOptions
	Build(g *BuilderGraph) error
}

// PolygonLayer assembles a graph into an S2Polygon.
type PolygonLayer struct {
	polygon *Polygon
	opts    BuilderGraphOptions
}

// NewPolygonLayer creates a layer that builds the given polygon.
func NewPolygonLayer(p *Polygon) *PolygonLayer {
	return &PolygonLayer{
		polygon: p,
		opts: BuilderGraphOptions{
			EdgeType:        EdgeTypeDirected,
			DegenerateEdges: DegenerateEdgesDiscard,
			DuplicateEdges:  DuplicateEdgesKeep,
			SiblingPairs:    SiblingPairsDiscard,
		},
	}
}

func (l *PolygonLayer) GraphOptions() BuilderGraphOptions {
	return l.opts
}

func (l *PolygonLayer) Build(g *BuilderGraph) error {
	var loops []*Loop
	used := make([]bool, len(g.Edges))

	// Greedily extract loops from the graph.
	// This is a simplified Eulerian path finding. Real implementation is robust to
	// complex topologies.
	for i := 0; i < len(g.Edges); i++ {
		if used[i] {
			continue
		}

		// Start a new loop
		var vertices []Point
		currEdgeIdx := i
		startVertex := g.Edges[currEdgeIdx].Src

		validLoop := false

		for {
			used[currEdgeIdx] = true
			edge := g.Edges[currEdgeIdx]
			vertices = append(vertices, g.Vertices[edge.Src])

			// Check if we closed the loop
			if edge.Dst == startVertex {
				validLoop = true
				break
			}

			// Find next edge
			nextIdx := -1
			candidates := g.GetOutEdges(edge.Dst)
			for _, candIdx := range candidates {
				if !used[candIdx] {
					nextIdx = int(candIdx)
					break
				}
			}

			if nextIdx == -1 {
				// Dead end, abort this path (in a real builder this logic is much more complex
				// handling backtracking or error reporting).
				break
			}
			currEdgeIdx = nextIdx
		}

		if validLoop && len(vertices) >= 3 {
			loops = append(loops, LoopFromPoints(vertices))
		}
	}

	if len(loops) > 0 {
		*l.polygon = *PolygonFromLoops(loops)
	} else {
		*l.polygon = *PolygonFromLoops([]*Loop{})
	}

	return nil
}
