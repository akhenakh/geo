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
	"testing"
)

func TestBuilderSimpleUnion(t *testing.T) {
	// Create two disjoint squares
	p1 := makeBuilderSquarePoly(0, 0, 1.0)
	p2 := makeBuilderSquarePoly(5, 5, 1.0)

	idxA := NewShapeIndex()
	idxA.Add(p1)

	idxB := NewShapeIndex()
	idxB.Add(p2)

	var output Polygon
	layer := NewPolygonLayer(&output)

	op := NewBooleanOperation(BooleanOperationOpTypeUnion, layer, nil)
	err := op.Build(idxA, idxB)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should result in a polygon with 2 loops
	if output.NumLoops() != 2 {
		t.Errorf("Expected 2 loops in union of disjoint polygons, got %d", output.NumLoops())
	}
}

func TestBuilderVertexSnapping(t *testing.T) {
	// Create a square that is "almost" closed but has a gap.
	// 0,0 -> 0,1 -> 1,1 -> 1,0 -> 0,0.0000001
	pts := []Point{
		PointFromLatLng(LatLngFromDegrees(0, 0)),
		PointFromLatLng(LatLngFromDegrees(0, 1)),
		PointFromLatLng(LatLngFromDegrees(1, 1)),
		PointFromLatLng(LatLngFromDegrees(1, 0)),
		PointFromLatLng(LatLngFromDegrees(0, 0.0000001)),
	}

	// Create builder with snapping enabled
	opts := BuilderOptions{
		SnapFunction: NewIdentitySnapFunction(kmToAngle(0.1)), // 100 meters snap
	}
	builder := NewBuilder(opts)

	var output Polygon
	builder.StartLayer(NewPolygonLayer(&output))

	// Add edges manually
	for i := 0; i < len(pts)-1; i++ {
		builder.AddEdge(pts[i], pts[i+1])
	}

	err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Snapping should close the loop
	if output.NumLoops() != 1 {
		t.Errorf("Expected 1 loop after snapping, got %d", output.NumLoops())
	}
}

func makeBuilderSquarePoly(lat, lng, size float64) *Polygon {
	half := size / 2
	pts := []Point{
		PointFromLatLng(LatLngFromDegrees(lat+half, lng+half)),
		PointFromLatLng(LatLngFromDegrees(lat+half, lng-half)),
		PointFromLatLng(LatLngFromDegrees(lat-half, lng-half)),
		PointFromLatLng(LatLngFromDegrees(lat-half, lng+half)),
	}
	loop := LoopFromPoints(pts)
	return PolygonFromLoops([]*Loop{loop})
}
