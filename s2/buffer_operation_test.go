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
	pt := PointFromLatLng(LatLngFromDegrees(0, 0))

	// 100km is roughly 0.9 degrees on Earth
	// radiusRadians := 100.0 / 6371.01 ~= 0.01569
	radius := kmToAngle(100.0)

	var resultPoly Polygon
	layer := NewPolygonLayer(&resultPoly)

	opts := DefaultBufferOperationOptions()
	opts.BufferRadius = radius
	// Use 0.1% error fraction to ensure the area approximation is close enough (within 1%)
	// A 1% radial error can result in >1% area error for low vertex counts.
	opts.ErrorFraction = 0.001

	op := NewBufferOperation(layer, opts)

	op.AddPoint(pt)

	// 1. Basic topology check
	if resultPoly.NumLoops() != 1 {
		t.Fatalf("Expected 1 loop for buffered point, got %d", resultPoly.NumLoops())
	}

	loop := resultPoly.Loop(0)

	// 2. Area check
	// Spherical Cap area = 2*pi*(1-cos(r))
	// Because the buffer is a polygon inscribed/circumscribed (depending on implementation details)
	// the area will be very close to the ideal circle area.
	expectedArea := 2 * math.Pi * (1 - math.Cos(float64(radius)))
	actualArea := loop.Area()

	// Check if area is within 1% + small epsilon margin
	if math.Abs(actualArea-expectedArea) > 0.01*expectedArea+1e-10 {
		t.Errorf("Area mismatch. Expected ~%v, got %v", expectedArea, actualArea)
	}

	// 3. Containment checks

	// Center should be inside
	if !loop.ContainsPoint(pt) {
		t.Error("Buffered region should contain the center point")
	}

	// Point safely INSIDE the radius (e.g., 98km away)
	// We use 98% of radius to account for the polygonal "flat" edges being slightly
	// closer to the center than the true circle radius.
	insidePt := PointFromLatLng(LatLngFromDegrees(0, 0.98*float64(radius.Degrees())))
	if !loop.ContainsPoint(insidePt) {
		t.Errorf("Point at 0.98*radius should be inside. Radius Deg: %f", radius.Degrees())
	}

	// Point safely OUTSIDE the radius (e.g., 102km away)
	outsidePt := PointFromLatLng(LatLngFromDegrees(0, 1.02*float64(radius.Degrees())))
	if loop.ContainsPoint(outsidePt) {
		t.Error("Point at 1.02*radius should be outside")
	}
}

func TestBufferOperationPolyline(t *testing.T) {
	p1 := PointFromLatLng(LatLngFromDegrees(0, 0))
	p2 := PointFromLatLng(LatLngFromDegrees(10, 0))

	radius := kmToAngle(10.0)

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

	// Check point to the left (inside buffer)
	pInside := PointToLeft(mid, p2, s1.Angle(radius)/2)
	if !loop.ContainsPoint(pInside) {
		t.Error("Point inside buffer radius should be contained")
	}

	// Check point to the left (outside buffer)
	pOutside := PointToLeft(mid, p2, s1.Angle(radius)*1.5)
	if loop.ContainsPoint(pOutside) {
		t.Error("Point outside buffer radius should not be contained")
	}
}
