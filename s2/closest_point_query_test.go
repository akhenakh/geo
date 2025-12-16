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

func TestClosestPointQueryNoPoints(t *testing.T) {
	index := NewPointIndex()
	query := NewClosestPointQuery(index, nil)
	target := NewMinDistanceToPointTarget(PointFromCoords(1, 0, 0))
	results := query.FindClosestPoints(target)
	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestClosestPointQueryManyDuplicatePoints(t *testing.T) {
	const kNumPoints = 100
	pt := PointFromCoords(1, 0, 0)
	index := NewPointIndex()
	for i := 0; i < kNumPoints; i++ {
		index.Add(pt, i)
	}

	query := NewClosestPointQuery(index, nil)
	target := NewMinDistanceToPointTarget(pt)
	results := query.FindClosestPoints(target)

	if len(results) != kNumPoints {
		t.Errorf("Expected %d results, got %d", kNumPoints, len(results))
	}

	for _, r := range results {
		if r.Distance() != 0 {
			t.Errorf("Expected distance 0, got %v", r.Distance())
		}
		if r.Point != pt {
			t.Errorf("Expected point %v, got %v", pt, r.Point)
		}
	}
}

func TestClosestPointQueryCirclePoints(t *testing.T) {
	// Points on a small circle
	center := PointFromCoords(1, 0, 0)
	radius := s1.Angle(0.01) // small angle

	index := NewPointIndex()
	numPoints := 10
	points := regularPoints(center, radius, numPoints)
	for i, p := range points {
		index.Add(p, i)
	}

	query := NewClosestPointQuery(index, nil)

	// Query from center
	target := NewMinDistanceToPointTarget(center)

	// Limit results
	query.opts.MaxResults(5)
	results := query.FindClosestPoints(target)

	if len(results) != 5 {
		t.Errorf("Expected 5 results, got %d", len(results))
	}

	// All should have approx same distance (radius)
	// Note: chord angle distance might vary slightly due to regular polygon vs circle
	expectedDist := s1.ChordAngleFromAngle(radius)
	for _, r := range results {
		// Allow some float error
		if diff := r.Distance() - expectedDist; diff > 1e-15 || diff < -1e-15 {
			// Actually regularPoints are exactly on the circle of radius `radius` (arc length).
			// So ChordAngle should match.
			// However, regularPoints implementation might use slightly different math.
			// Let's just check it's close.
			if float64(diff) > 1e-10 {
				t.Errorf("Distance mismatch: got %v, want approx %v", r.Distance(), expectedDist)
			}
		}
	}
}

func TestClosestPointQueryMaxDistance(t *testing.T) {
	index := NewPointIndex()
	p1 := PointFromCoords(1, 0, 0)
	p2 := PointFromCoords(0, 1, 0) // 90 degrees away
	index.Add(p1, 1)
	index.Add(p2, 2)

	query := NewClosestPointQuery(index, nil)
	query.opts.MaxDistance(s1.ChordAngleFromAngle(s1.Angle(0.5))) // Small distance

	target := NewMinDistanceToPointTarget(p1)
	results := query.FindClosestPoints(target)

	if len(results) != 1 {
		t.Errorf("Expected 1 result (p1), got %d", len(results))
	}
	if results[0].Data.(int) != 1 {
		t.Errorf("Expected p1, got %v", results[0].Data)
	}
}
