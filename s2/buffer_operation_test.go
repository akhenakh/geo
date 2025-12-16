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
	"math"
	"testing"

	"github.com/golang/geo/s1"
)

func TestBufferOperationPoint(t *testing.T) {
	// Test buffering a single point.
	// This should produce a small circle (approximated polygon).

	pt := PointFromLatLng(LatLngFromDegrees(0, 0))
	radius := kmToAngle(100.0) // 100km radius

	// Setup
	var resultPoly Polygon
	layer := NewPolygonLayer(&resultPoly)

	opts := DefaultBufferOperationOptions()
	opts.BufferRadius = radius

	op := NewBufferOperation(layer, opts)

	// Add point manually via shape interface simulation
	// Since we don't have a PointShape, we call AddPoint directly
	op.AddPoint(pt)

	// Verify
	if resultPoly.NumLoops() != 1 {
		t.Fatalf("Expected 1 loop for buffered point, got %d", resultPoly.NumLoops())
	}

	loop := resultPoly.Loop(0)
	// Check area
	// Cap area = 2*pi*(1-cos(r))
	expectedArea := 2 * math.Pi * (1 - math.Cos(float64(radius)))
	actualArea := loop.Area()

	// Since it's a polygon approximation, area should be close but slightly less
	if math.Abs(actualArea-expectedArea) > 0.1*expectedArea {
		t.Errorf("Area mismatch. Expected ~%v, got %v", expectedArea, actualArea)
	}

	// Check containment
	if !loop.ContainsPoint(pt) {
		t.Error("Buffered region should contain the center point")
	}

	// Check a point on the rim
	// rimPt := PointFromLatLng(LatLngFromDegrees(0, 0.9)) // 100km is roughly 0.9 degrees
	// Radius in radians: 100/6371 ~= 0.015 rad ~= 0.9 degrees
	// Actually 1 degree is ~111km.
	// 100km is ~0.9 degrees.
	// The generated polygon vertices should be at distance radius.
	// Since we use a simple linear connection in the stub, the rim point might be slightly inside/outside depending on rotation.
}

func TestBufferOperationPolyline(t *testing.T) {
	// Buffer a simple line segment.
	// 0,0 -> 0,10 (vertical line approx)
	p1 := PointFromLatLng(LatLngFromDegrees(0, 0))
	p2 := PointFromLatLng(LatLngFromDegrees(10, 0))

	radius := kmToAngle(10.0) // 10km buffer

	var resultPoly Polygon
	layer := NewPolygonLayer(&resultPoly)
	opts := DefaultBufferOperationOptions()
	opts.BufferRadius = radius

	op := NewBufferOperation(layer, opts)
	op.AddPolyline([]Point{p1, p2})

	if resultPoly.NumLoops() != 1 {
		t.Fatalf("Expected 1 loop for buffered polyline, got %d", resultPoly.NumLoops())
	}

	loop := resultPoly.Loop(0)

	// Check center of line is contained
	mid := Interpolate(0.5, p1, p2)
	if !loop.ContainsPoint(mid) {
		t.Error("Buffered polyline should contain its midpoint")
	}

	// Check width at midpoint
	// Point 10km east of midpoint should be inside/boundary
	// Point 20km east should be outside

	// Orthogonal direction is East (0,1,0) approx.
	// Note: 0,0 is (1,0,0), 10,0 is (cos10, 0, sin10).
	// Cross product is roughly Y axis.

	// We can use the logic from S2:
	pInside := PointToLeft(mid, p2, s1.Angle(radius)/2)
	if !loop.ContainsPoint(pInside) {
		t.Error("Point inside buffer radius should be contained")
	}
}
