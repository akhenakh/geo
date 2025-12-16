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

func TestShapeNestingSingleLoop(t *testing.T) {
	// Simple square
	// 0:0, 0:10, 10:10, 10:0
	pts := []Point{
		PointFromLatLng(LatLngFromDegrees(0, 0)),
		PointFromLatLng(LatLngFromDegrees(0, 10)),
		PointFromLatLng(LatLngFromDegrees(10, 10)),
		PointFromLatLng(LatLngFromDegrees(10, 0)),
	}
	loop := LoopFromPoints(pts)

	// Create a Polygon from it
	poly := PolygonFromLoops([]*Loop{loop})

	index := NewShapeIndex()
	id := index.Add(poly)

	query := NewShapeNestingQuery(index, nil)
	relations := query.ComputeShapeNesting(id)

	if len(relations) != 1 {
		t.Fatalf("Expected 1 relation, got %d", len(relations))
	}

	if !relations[0].IsShell() {
		t.Error("Single loop should be a shell")
	}
}

func TestShapeNestingShellAndHole(t *testing.T) {
	// Shell: 0:0, 0:10, 10:10, 10:0 (CCW)
	// Hole:  2:2, 8:2, 8:8, 2:8 (CCW in points, but holes should be CW?
	// S2Polygon structure allows loops to be passed in raw, and InitNested handles orientation.
	// Here we are testing ShapeNestingQuery on a Shape.
	// If we use LaxPolygon, orientation matters.

	shellPts := []Point{
		PointFromLatLng(LatLngFromDegrees(0, 0)),
		PointFromLatLng(LatLngFromDegrees(0, 10)),
		PointFromLatLng(LatLngFromDegrees(10, 10)),
		PointFromLatLng(LatLngFromDegrees(10, 0)),
	}

	holePts := []Point{
		PointFromLatLng(LatLngFromDegrees(2, 2)),
		PointFromLatLng(LatLngFromDegrees(8, 2)),
		PointFromLatLng(LatLngFromDegrees(8, 8)),
		PointFromLatLng(LatLngFromDegrees(2, 8)),
	}

	// LaxPolygonFromPoints takes multiple loops.
	// We pass them both as CCW.
	// The query should detect that the second is inside the first.
	laxPoly := LaxPolygonFromPoints([][]Point{shellPts, holePts})

	index := NewShapeIndex()
	id := index.Add(laxPoly)

	query := NewShapeNestingQuery(index, nil)
	relations := query.ComputeShapeNesting(id)

	if len(relations) != 2 {
		t.Fatalf("Expected 2 relations, got %d", len(relations))
	}

	// Chain 0 (Shell) should be a shell
	if !relations[0].IsShell() {
		t.Error("Chain 0 should be a shell")
	}

	// Chain 1 (Hole) should be a child of Chain 0
	if relations[1].ParentID != 0 {
		t.Errorf("Chain 1 ParentID = %d, want 0", relations[1].ParentID)
	}
	if relations[1].IsShell() {
		t.Error("Chain 1 should be a hole")
	}
}

func TestShapeNestingDisjoint(t *testing.T) {
	// Two separate squares
	sq1 := []Point{
		PointFromLatLng(LatLngFromDegrees(0, 0)),
		PointFromLatLng(LatLngFromDegrees(0, 5)),
		PointFromLatLng(LatLngFromDegrees(5, 5)),
		PointFromLatLng(LatLngFromDegrees(5, 0)),
	}
	sq2 := []Point{
		PointFromLatLng(LatLngFromDegrees(10, 10)),
		PointFromLatLng(LatLngFromDegrees(10, 15)),
		PointFromLatLng(LatLngFromDegrees(15, 15)),
		PointFromLatLng(LatLngFromDegrees(15, 10)),
	}

	laxPoly := LaxPolygonFromPoints([][]Point{sq1, sq2})

	index := NewShapeIndex()
	id := index.Add(laxPoly)

	query := NewShapeNestingQuery(index, nil)
	relations := query.ComputeShapeNesting(id)

	if len(relations) != 2 {
		t.Fatalf("Expected 2 relations, got %d", len(relations))
	}

	// Both should be shells
	if !relations[0].IsShell() {
		t.Error("Chain 0 should be a shell")
	}
	if !relations[1].IsShell() {
		t.Error("Chain 1 should be a shell")
	}
}
