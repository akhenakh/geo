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

	"github.com/golang/geo/r3"
	"github.com/golang/geo/s1"
)

// PolylineSimplifier is a helper struct for simplifying polylines. It allows you to compute
// a maximal edge that intersects a sequence of discs, and that optionally avoids a different
// sequence of discs. The results are conservative in that the edge is guaranteed to
// intersect or avoid the specified discs using exact arithmetic.
//
// Note that S2Builder (once fully implemented) can also simplify polylines and supports
// more features.
type PolylineSimplifier struct {
	src           Point
	xDir, yDir    r3.Vector   // Orthonormal frame for mapping vectors to angles.
	window        s1.Interval // Allowable range of angles for the output edge.
	rangesToAvoid []rangeToAvoid
}

type rangeToAvoid struct {
	interval s1.Interval
	onLeft   bool
}

// NewPolylineSimplifier creates a new simplifier starting at the given source vertex.
func NewPolylineSimplifier(src Point) *PolylineSimplifier {
	s := &PolylineSimplifier{}
	s.Init(src)
	return s
}

// Init starts a new simplified edge at src.
func (s *PolylineSimplifier) Init(src Point) {
	s.src = src
	s.window = s1.FullInterval()
	s.rangesToAvoid = nil // Clear existing slice

	// Precompute basis vectors for the tangent space at "src".
	// This mimics the logic in the C++ implementation to ensure stability.
	// We don't normalize these vectors.

	// Find the index of the component whose magnitude is smallest.
	tmp := src.Abs()
	i := 0
	if tmp.Y < tmp.X {
		if tmp.Y < tmp.Z {
			i = 1
		} else {
			i = 2
		}
	} else {
		if tmp.X < tmp.Z {
			i = 0
		} else {
			i = 2
		}
	}

	// Define the "y" basis vector as the cross product of "src" and the basis vector for axis "i".
	j := (i + 1) % 3
	if i == 2 {
		j = 0
	}
	k := (i + 2) % 3
	if i == 0 {
		k = 2
	}

	// Helper to access vector components by index since r3.Vector doesn't have an Index method.
	getComponent := func(v r3.Vector, idx int) float64 {
		switch idx {
		case 0:
			return v.X
		case 1:
			return v.Y
		default:
			return v.Z
		}
	}

	// We set components directly based on cross product logic 0, src[k], -src[j]
	// but mapped to indices i, j, k.
	// yDir[i] = 0
	// yDir[j] = src[k]
	// yDir[k] = -src[j]
	yCoords := [3]float64{}
	yCoords[i] = 0
	yCoords[j] = getComponent(src.Vector, k)
	yCoords[k] = -getComponent(src.Vector, j)
	s.yDir = r3.Vector{X: yCoords[0], Y: yCoords[1], Z: yCoords[2]}

	// Compute xDir = yDir cross src.
	// xDir[i] = src[j]*src[j] + src[k]*src[k]
	// xDir[j] = -src[j]*src[i]
	// xDir[k] = -src[k]*src[i]
	xCoords := [3]float64{}
	xCoords[i] = getComponent(src.Vector, j)*getComponent(src.Vector, j) + getComponent(src.Vector, k)*getComponent(src.Vector, k)
	xCoords[j] = -getComponent(src.Vector, j) * getComponent(src.Vector, i)
	xCoords[k] = -getComponent(src.Vector, k) * getComponent(src.Vector, i)
	s.xDir = r3.Vector{X: xCoords[0], Y: xCoords[1], Z: xCoords[2]}
}

// Src returns the source vertex of the output edge.
func (s *PolylineSimplifier) Src() Point {
	return s.src
}

// Extend returns true if the edge (src, dst) satisfies all of the targeting
// requirements so far. Returns false if the edge would be longer than
// 90 degrees (such edges are not supported).
func (s *PolylineSimplifier) Extend(dst Point) bool {
	// We limit the maximum edge length to 90 degrees.
	// In Go, ChordAngleBetweenPoints is in the s2 package, not s1.
	if ChordAngleBetweenPoints(s.src, dst) > s1.RightChordAngle {
		return false
	}

	// Check whether this vertex is in the acceptable angle range.
	dir := s.getDirection(dst)
	if !s.window.Contains(dir) {
		return false
	}

	// Check any angle ranges to avoid.
	for _, r := range s.rangesToAvoid {
		if r.interval.Contains(dir) {
			return false
		}
	}
	return true
}

// TargetDisc requires that the output edge must pass through the given disc.
// Returns true if it is possible to intersect the target disc, given previous constraints.
func (s *PolylineSimplifier) TargetDisc(p Point, r s1.ChordAngle) bool {
	// Shrink the target interval by the maximum error.
	semiwidth := s.getSemiwidth(p, r, -1) // round down
	if semiwidth >= math.Pi {
		// The target disc contains src, so nothing to do.
		return true
	}
	if semiwidth < 0 {
		s.window = s1.EmptyInterval()
		return false
	}

	// Compute angle interval corresponding to target disc and intersect with current window.
	center := s.getDirection(p)
	// s1.IntervalFromPoint is basically IntervalFromPointPair(p, p)
	target := s1.IntervalFromPointPair(center, center).Expanded(semiwidth)
	s.window = s.window.Intersection(target)

	// Process any pending ranges to avoid.
	// Note: In C++ implementation, ranges are processed here because AvoidDisc might have
	// deferred them if the window was full.
	if len(s.rangesToAvoid) > 0 {
		// We iterate and compact the slice if needed, but since we clear it at the end, just iterate.
		for _, r := range s.rangesToAvoid {
			s.avoidRange(r.interval, r.onLeft)
		}
		s.rangesToAvoid = nil // Equivalent to clear()
	}

	return !s.window.IsEmpty()
}

// AvoidDisc requires that the output edge must avoid the given disc.
// "discOnLeft" specifies whether the disc must be to the left or right of the output edge.
// Returns true if the disc can be avoided, false otherwise.
func (s *PolylineSimplifier) AvoidDisc(p Point, r s1.ChordAngle, discOnLeft bool) bool {
	// Expand the interval by maximum error.
	semiwidth := s.getSemiwidth(p, r, 1) // round up
	if semiwidth >= math.Pi {
		// The disc to avoid contains src, so it can't be avoided.
		s.window = s1.EmptyInterval()
		return false
	}

	// Compute disallowed range of angles.
	center := s.getDirection(p)
	var dLeft, dRight float64
	if discOnLeft {
		dLeft = math.Pi / 2
		dRight = semiwidth
	} else {
		dLeft = semiwidth
		dRight = math.Pi / 2
	}
	avoidInterval := s1.IntervalFromEndpoints(
		math.Remainder(center-dRight, 2*math.Pi),
		math.Remainder(center+dLeft, 2*math.Pi),
	)

	if s.window.IsFull() {
		// Save for later.
		s.rangesToAvoid = append(s.rangesToAvoid, rangeToAvoid{avoidInterval, discOnLeft})
		return true
	}

	s.avoidRange(avoidInterval, discOnLeft)
	return !s.window.IsEmpty()
}

func (s *PolylineSimplifier) avoidRange(avoidInterval s1.Interval, discOnLeft bool) {
	// If avoidInterval is a proper subset of window, we split the window.
	// Ideally we pick the side that points towards the disc passing on the correct side,
	// but the other side points away and is never an acceptable edge direction for valid usage.
	// See C++ comments for details.
	if s.window.ContainsInterval(avoidInterval) {
		if discOnLeft {
			s.window = s1.Interval{Lo: s.window.Lo, Hi: avoidInterval.Lo}
		} else {
			s.window = s1.Interval{Lo: avoidInterval.Hi, Hi: s.window.Hi}
		}
	} else {
		s.window = s.window.Intersection(avoidInterval.Complement())
	}
}

func (s *PolylineSimplifier) getDirection(p Point) float64 {
	return math.Atan2(p.Dot(s.yDir), p.Dot(s.xDir))
}

// getSemiwidth computes half the angle subtended from src by a disc of radius r centered at p.
// roundDirection is +1 (round up/upper bound) or -1 (round down/lower bound).
func (s *PolylineSimplifier) getSemiwidth(p Point, r s1.ChordAngle, roundDirection float64) float64 {
	// Based on error analysis in s2polyline_simplifier.cc.
	const dblError = 0.5 * dblEpsilon

	r2 := float64(r)
	// ChordAngleBetweenPoints is in s2 package
	a2 := float64(ChordAngleBetweenPoints(s.src, p))

	a2 -= 64 * dblError * dblError * roundDirection
	if a2 <= r2 {
		return math.Pi
	}

	sin2R := r2 * (1 - 0.25*r2)
	sin2A := a2 * (1 - 0.25*a2)
	semiwidth := math.Asin(math.Sqrt(sin2R / sin2A))

	// Error calculation from C++ implementation.
	error := (2*10+4)*dblError + 17*dblError*semiwidth
	return semiwidth + roundDirection*error
}
