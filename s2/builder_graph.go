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

// BuilderGraph represents a collection of snapped edges passed to a Layer.
type BuilderGraph struct {
	Options  BuilderGraphOptions
	Vertices []Point
	Edges    []BuilderGraphEdge

	// Adjacency list for graph traversal (computed on demand or init)
	outEdges [][]int32 // vertexID -> []edgeIndices
}

// BuilderGraphEdge represents an edge in the builder graph.
type BuilderGraphEdge struct {
	Src, Dst int32
	InputIDs []int32
}

// BuilderGraphOptions controls how the graph is constructed.
type BuilderGraphOptions struct {
	EdgeType        BuilderEdgeType
	DegenerateEdges BuilderDegenerateEdges
	DuplicateEdges  BuilderDuplicateEdges
	SiblingPairs    BuilderSiblingPairs
}

type BuilderEdgeType int

const (
	EdgeTypeDirected BuilderEdgeType = iota
	EdgeTypeUndirected
)

type BuilderDegenerateEdges int

const (
	DegenerateEdgesKeep BuilderDegenerateEdges = iota
	DegenerateEdgesDiscard
	DegenerateEdgesDiscardExcess
)

type BuilderDuplicateEdges int

const (
	DuplicateEdgesKeep BuilderDuplicateEdges = iota
	DuplicateEdgesMerge
)

type BuilderSiblingPairs int

const (
	SiblingPairsKeep BuilderSiblingPairs = iota
	SiblingPairsDiscard
	SiblingPairsDiscardExcess
	SiblingPairsRequire
	SiblingPairsCreate
)

// NewBuilderGraph creates a new graph.
func NewBuilderGraph(opts BuilderGraphOptions, vertices []Point, edges []BuilderGraphEdge) *BuilderGraph {
	g := &BuilderGraph{
		Options:  opts,
		Vertices: vertices,
		Edges:    edges,
	}
	g.computeAdjacency()
	return g
}

func (g *BuilderGraph) computeAdjacency() {
	g.outEdges = make([][]int32, len(g.Vertices))
	for i, e := range g.Edges {
		g.outEdges[e.Src] = append(g.outEdges[e.Src], int32(i))
	}
}

// GetOutEdges returns the indices of edges starting at vertex v.
func (g *BuilderGraph) GetOutEdges(v int32) []int32 {
	if int(v) >= len(g.outEdges) {
		return nil
	}
	return g.outEdges[v]
}

// NumVertices returns the number of vertices.
func (g *BuilderGraph) NumVertices() int { return len(g.Vertices) }

// Vertex returns the vertex at index.
func (g *BuilderGraph) Vertex(i int32) Point { return g.Vertices[i] }

// NumEdges returns the number of edges.
func (g *BuilderGraph) NumEdges() int { return len(g.Edges) }

// Edge returns the edge at index.
func (g *BuilderGraph) Edge(i int32) BuilderGraphEdge { return g.Edges[i] }
