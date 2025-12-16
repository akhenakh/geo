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

	"github.com/golang/geo/s1"
)

func TestPolylineSimplifierStraightLine(t *testing.T) {
	// Points along a straight line.
	// 0:0, 0:1, ..., 0:10 (degrees)
	src := PointFromLatLng(LatLngFromDegrees(0, 0))
	simplifier := NewPolylineSimplifier(src)

	// Target tolerance approx 10 meters (converted to angle).
	// Earth radius ~6371km. 10m is very small.
	tolerance := s1.ChordAngleFromAngle(s1.Angle(10.0 / 6371000.0))

	for i := 1; i <= 10; i++ {
		p := PointFromLatLng(LatLngFromDegrees(0, float64(i)))

		// 1. Extend should succeed for points on the line.
		if !simplifier.Extend(p) {
			t.Errorf("Extend failed for point %d on straight line", i)
		}

		// 2. TargetDisc with tolerance.
		if !simplifier.TargetDisc(p, tolerance) {
			t.Errorf("TargetDisc failed for point %d", i)
		}
	}
}

func TestPolylineSimplifierPerturbedLine(t *testing.T) {
	// Points that wiggle slightly but stay within tolerance.
	src := PointFromLatLng(LatLngFromDegrees(0, 0))
	simplifier := NewPolylineSimplifier(src)

	// Tolerance of 0.1 degrees.
	tolAngle := s1.Angle(0.1) * s1.Degree
	tolerance := s1.ChordAngleFromAngle(tolAngle)

	// Points at 0:1, 0.05:2, 0:3, -0.05:4, 0:5.
	// All within 0.05 degrees of the equator.
	points := []Point{
		PointFromLatLng(LatLngFromDegrees(0, 1)),
		PointFromLatLng(LatLngFromDegrees(0.05, 2)),
		PointFromLatLng(LatLngFromDegrees(0, 3)),
		PointFromLatLng(LatLngFromDegrees(-0.05, 4)),
		PointFromLatLng(LatLngFromDegrees(0, 5)),
	}

	for i, p := range points {
		if !simplifier.Extend(p) {
			t.Errorf("Extend failed for perturbed point %d", i)
		}
		if !simplifier.TargetDisc(p, tolerance) {
			t.Errorf("TargetDisc failed for perturbed point %d", i)
		}
	}
}

func TestPolylineSimplifierAvoidDisc(t *testing.T) {
	// Path goes straight East.
	// Obstacle is to the North.
	src := PointFromLatLng(LatLngFromDegrees(0, 0))
	simplifier := NewPolylineSimplifier(src)

	// Target point straight East.
	dst := PointFromLatLng(LatLngFromDegrees(0, 10))

	// Obstacle at 0.1, 5 (North of path).
	obstacle := PointFromLatLng(LatLngFromDegrees(0.1, 5))
	obstacleRad := s1.ChordAngleFromAngle(s1.Angle(0.01) * s1.Degree)

	// 1. Avoiding obstacle on Left (North is Left when moving East) should succeed.
	// Note: We need to set a target first to constrain the window so AvoidDisc logic works fully immediately,
	// or rely on it queuing if window is full.
	// Let's set a loose target first to define general direction.
	looseTarget := PointFromLatLng(LatLngFromDegrees(0, 1))
	simplifier.TargetDisc(looseTarget, s1.ChordAngleFromAngle(s1.Angle(1.0)*s1.Degree))

	if !simplifier.AvoidDisc(obstacle, obstacleRad, true) {
		t.Error("Should be able to avoid obstacle on the left")
	}

	// 2. Extending to destination should still work.
	if !simplifier.Extend(dst) {
		t.Error("Extend to dst failed after avoiding obstacle")
	}

	// 3. Trying to avoid obstacle on Right should fail (it blocks the path).
	// Reset
	simplifier = NewPolylineSimplifier(src)
	simplifier.TargetDisc(dst, s1.ChordAngleFromAngle(s1.Angle(0.001)*s1.Degree)) // Tight target

	// Obstacle is North (Left). If we say it must be on Right, the ray must go North of it.
	// But our target is South of it.
	// This logic depends on exact geometry, but essentially forcing the line to go "North" of a "North" obstacle
	// when the target is "South" implies no valid path.
	// However, AvoidDisc takes "discOnLeft".
	// If disc is at (0.1, 5) and line is (0,0)->(0,10), disc is on Left.
	// If we call AvoidDisc(obs, rad, false [Right]), we demand the line passes such that obs is on Right.
	// i.e., line must be North of (0.1, 5).
	// But target is at (0, 10). Line (0,0)->(0,10) is South of (0.1, 5).
	// So this should fail or result in empty window if the tolerance is tight.

	// With tight tolerance on dst, the window is very narrow around equator.
	// Obstacle is at 0.1 lat.
	// Demanding obstacle be on Right means we must aim > 0.1 lat.
	// But target constrains us to ~0 lat.
	// Intersection should be empty.

	if simplifier.AvoidDisc(obstacle, obstacleRad, false) {
		t.Error("Should NOT be able to avoid obstacle on the Right (it forces path away from target)")
	}
}

func TestPolylineSimplifierLargeDeviation(t *testing.T) {
	src := PointFromLatLng(LatLngFromDegrees(0, 0))
	simplifier := NewPolylineSimplifier(src)

	// Tolerance small.
	tolerance := s1.ChordAngleFromAngle(s1.Angle(0.01) * s1.Degree)

	// Point 1: 0, 1
	p1 := PointFromLatLng(LatLngFromDegrees(0, 1))
	simplifier.TargetDisc(p1, tolerance)

	// Point 2: 10, 2 (Huge jump North)
	p2 := PointFromLatLng(LatLngFromDegrees(10, 2))

	// Extend should likely fail if we try to go there while keeping p1 in line?
	// Actually Extend just checks if p2 is in the current allowable window.
	// The window is constrained to point roughly East (towards 0,1).
	// 10,2 is North-East (approx 78 degrees bearing vs 90 degrees).
	// With tolerance 0.01 degree at distance 1 degree, the angular window is very narrow.
	// It definitely shouldn't contain the angle to (10, 2).

	if simplifier.Extend(p2) {
		t.Error("Extend should fail for point with large deviation")
	}
}

func TestPolylineSimplifierMaxEdgeLength(t *testing.T) {
	src := PointFromLatLng(LatLngFromDegrees(0, 0))
	simplifier := NewPolylineSimplifier(src)

	// Point > 90 degrees away. 91 degrees longitude.
	// Note: Great circle distance > 90 deg.
	farPoint := PointFromLatLng(LatLngFromDegrees(0, 91))

	if simplifier.Extend(farPoint) {
		t.Error("Extend should fail for edge > 90 degrees")
	}
}

func TestPolylineSimplifierSemiwidthCase(t *testing.T) {
	// Test case where the disc contains the source.
	src := PointFromLatLng(LatLngFromDegrees(0, 0))
	simplifier := NewPolylineSimplifier(src)

	// Disc at 0,0 with radius 1 degree.
	p := src
	r := s1.ChordAngleFromAngle(s1.Degree)

	// Should return true and window should remain full.
	if !simplifier.TargetDisc(p, r) {
		t.Error("TargetDisc should succeed when disc contains src")
	}
	if !simplifier.window.IsFull() {
		t.Error("Window should remain full when disc contains src")
	}

	// AvoidDisc with same params -> Fail (can't avoid).
	if simplifier.AvoidDisc(p, r, true) {
		t.Error("AvoidDisc should fail when disc contains src")
	}
}
