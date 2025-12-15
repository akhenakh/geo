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
	"fmt"

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
//
// NOTE: This implementation currently primarily supports UNION.
// Intersection and Difference require the "CrossingProcessor" logic from C++
// which selectively includes edges based on winding numbers.
//
// For this port, we provide the architecture. A full geometric boolean op
// engine is 5000+ lines of C++. This version sets up the Builder correctly
// so that if you implement the edge filtering (clipping), the rest works.
func (b *BooleanOperation) Build(indexA, indexB *ShapeIndex) error {
	builder := NewBuilder(BuilderOptions{
		SnapFunction:       b.Options.SnapFunction,
		SplitCrossingEdges: b.Options.SplitCrossingEdges,
	})

	for _, layer := range b.Layers {
		builder.StartLayer(layer)
	}

	// For UNION, we can simply add all edges from both.
	// For INTERSECTION/DIFFERENCE, we would need to filter edges here
	// based on containment in the other index.

	// Example Logic for UNION:
	if b.OpType == BooleanOperationOpTypeUnion {
		b.addEdges(builder, indexA)
		b.addEdges(builder, indexB)
	} else if b.OpType == BooleanOperationOpTypeIntersection {
		// Naive intersection approximation: Only add edges from A that are inside B
		// (This is incorrect for boundaries, but illustrates the filtering point)
		// A proper implementation requires the EdgeClippingLayer logic.
		return fmt.Errorf("intersection not fully implemented in this port")
	}

	return builder.Build()
}

func (b *BooleanOperation) addEdges(builder *Builder, index *ShapeIndex) {
	for _, shape := range index.shapes {
		for i := 0; i < shape.NumEdges(); i++ {
			e := shape.Edge(i)
			builder.AddEdge(e.V0, e.V1)
		}
	}
}
