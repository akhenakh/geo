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
	"container/heap"
	"math"
	"sort"

	"github.com/golang/geo/s1"
)

// ClosestPointQueryBaseOptions controls the set of points returned.
type ClosestPointQueryBaseOptions struct {
	MaxResults    int
	MaxDistance   distance
	MaxError      s1.ChordAngle
	Region        Region
	UseBruteForce bool
}

// NewClosestPointQueryBaseOptions returns default options.
func NewClosestPointQueryBaseOptions() ClosestPointQueryBaseOptions {
	return ClosestPointQueryBaseOptions{
		MaxResults:  math.MaxInt32,
		MaxDistance: minDistance(s1.InfChordAngle()),
		MaxError:    0,
	}
}

// ClosestPointQueryResult represents a closest point result.
type ClosestPointQueryResult struct {
	distance distance
	Point    Point
	Data     interface{}
	pointID  int32 // Internal ID for sorting stability
}

// Distance returns the distance from the target to this point.
func (r ClosestPointQueryResult) Distance() s1.ChordAngle {
	return r.distance.chordAngle()
}

// IsEmpty returns true if this result does not refer to any data point.
func (r ClosestPointQueryResult) IsEmpty() bool {
	return r.pointID == -1
}

// Less compares two results first by distance, then by point ID.
func (r ClosestPointQueryResult) Less(other ClosestPointQueryResult) bool {
	if r.distance.less(other.distance) {
		return true
	}
	if other.distance.less(r.distance) {
		return false
	}
	return r.pointID < other.pointID
}

// ClosestPointQueryBase is a base struct for finding the closest point(s) to a given target.
// It is not intended to be used directly, but rather to serve as the implementation
// of various specialized classes with more convenient APIs.
type ClosestPointQueryBase struct {
	index   *PointIndex
	options ClosestPointQueryBaseOptions
	target  distanceTarget

	distanceLimit               distance
	useConservativeCellDistance bool

	indexCovering               []CellID
	regionCovering              []CellID
	maxDistanceCovering         []CellID
	intersectionWithRegion      []CellID
	intersectionWithMaxDistance []CellID

	queue        *queryQueue
	resultVector []ClosestPointQueryResult
	resultHeap   resultHeap
}

// NewClosestPointQueryBase creates a new query base.
func NewClosestPointQueryBase(index *PointIndex) *ClosestPointQueryBase {
	q := &ClosestPointQueryBase{
		index: index,
		queue: newQueryQueue(),
	}
	q.ReInit()
	return q
}

// ReInit reinitializes the query.
func (q *ClosestPointQueryBase) ReInit() {
	q.indexCovering = nil
}

// FindClosestPoints returns the closest points to the given target.
func (q *ClosestPointQueryBase) FindClosestPoints(target distanceTarget, options ClosestPointQueryBaseOptions) []ClosestPointQueryResult {
	q.findClosestPointsInternal(target, options)

	results := make([]ClosestPointQueryResult, 0, len(q.resultVector)+q.resultHeap.Len())
	switch options.MaxResults {
	case 1:
		if len(q.resultVector) > 0 {
			results = append(results, q.resultVector[0])
		}
	case math.MaxInt32:
		sort.Slice(q.resultVector, func(i, j int) bool {
			return q.resultVector[i].Less(q.resultVector[j])
		})
		results = append(results, q.resultVector...)
	default:
		for q.resultHeap.Len() > 0 {
			results = append(results, heap.Pop(&q.resultHeap).(ClosestPointQueryResult))
		}
		// Heap pops largest first (max heap), so reverse to get smallest first.
		for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
			results[i], results[j] = results[j], results[i]
		}
	}

	// Reset internal state
	q.resultVector = nil
	q.resultHeap = nil
	return results
}

// FindClosestPoint returns exactly one point.
func (q *ClosestPointQueryBase) FindClosestPoint(target distanceTarget, options ClosestPointQueryBaseOptions) ClosestPointQueryResult {
	options.MaxResults = 1
	q.findClosestPointsInternal(target, options)
	if len(q.resultVector) > 0 {
		return q.resultVector[0]
	}
	return ClosestPointQueryResult{
		distance: target.distance().infinity(),
		pointID:  -1,
	}
}

func (q *ClosestPointQueryBase) findClosestPointsInternal(target distanceTarget, options ClosestPointQueryBaseOptions) {
	q.target = target
	q.options = options
	q.distanceLimit = options.MaxDistance

	if q.distanceLimit == target.distance().zero() {
		return
	}

	targetUsesMaxError := options.MaxError > 0 && target.setMaxError(options.MaxError)
	q.useConservativeCellDistance = targetUsesMaxError &&
		(q.distanceLimit == target.distance().infinity() ||
			target.distance().zero().less(q.distanceLimit.sub(target.distance().fromChordAngle(options.MaxError))))

	if options.UseBruteForce || q.index.NumPoints() <= target.maxBruteForceIndexSize() {
		q.findClosestPointsBruteForce()
	} else {
		q.findClosestPointsOptimized()
	}
}

func (q *ClosestPointQueryBase) findClosestPointsBruteForce() {
	it := q.index.Iterator()
	for !it.Done() {
		q.maybeAddResult(it.Point(), it.Data(), int32(it.Index()))
		it.Next()
	}
}

func (q *ClosestPointQueryBase) findClosestPointsOptimized() {
	q.initQueue()
	for q.queue.size() > 0 {
		entry := q.queue.pop()
		if !entry.distance.less(q.distanceLimit) {
			q.queue.reset()
			break
		}

		child := entry.id.ChildBegin()
		// We already know that it has too many points, so process its children.
		// Each child may either be processed directly or enqueued again.
		seek := true
		it := q.index.Iterator()
		for i := 0; i < 4; i++ {
			seek = q.processOrEnqueue(child, it, seek)
			child = child.Next()
		}
	}
}

func (q *ClosestPointQueryBase) initQueue() {
	// Optimization: limit search to small disc
	cap := q.target.capBound()
	if cap.IsEmpty() {
		return
	}

	switch q.options.MaxResults {
	case 1:
		it := q.index.Iterator()
		it.Seek(cellIDFromPoint(cap.Center()))
		if !it.Done() {
			q.maybeAddResult(it.Point(), it.Data(), int32(it.Index()))
		}
		if it.Prev() {
			q.maybeAddResult(it.Point(), it.Data(), int32(it.Index()))
		}
		if q.distanceLimit == q.target.distance().zero() {
			return
		}
	}

	if len(q.indexCovering) == 0 {
		q.initCovering()
	}

	initialCells := q.indexCovering
	if q.options.Region != nil {
		coverer := &RegionCoverer{MaxCells: 4}
		q.regionCovering = []CellID(coverer.Covering(q.options.Region))
		q.intersectionWithRegion = []CellID(CellUnionFromIntersection(CellUnion(q.indexCovering), CellUnion(q.regionCovering)))
		initialCells = q.intersectionWithRegion
	}

	if q.distanceLimit != q.target.distance().infinity() {
		coverer := &RegionCoverer{MaxCells: 4}
		radius := cap.Radius() + q.distanceLimit.chordAngleBound().Angle()
		searchCap := CapFromCenterAngle(cap.Center(), radius)
		q.maxDistanceCovering = []CellID(coverer.FastCovering(searchCap))
		q.intersectionWithMaxDistance = []CellID(CellUnionFromIntersection(CellUnion(initialCells), CellUnion(q.maxDistanceCovering)))
		initialCells = q.intersectionWithMaxDistance
	}

	it := q.index.Iterator()
	for _, id := range initialCells {
		q.processOrEnqueue(id, it, id.RangeMin() > it.CellID())
	}
}

func (q *ClosestPointQueryBase) initCovering() {
	q.indexCovering = nil
	// For MVP, just add top level faces to ensure full coverage
	for i := 0; i < 6; i++ {
		q.indexCovering = append(q.indexCovering, CellIDFromFace(i))
	}
}

func (q *ClosestPointQueryBase) maybeAddResult(p Point, data interface{}, id int32) {
	dist := q.distanceLimit
	ok := false
	dist, ok = q.target.updateDistanceToPoint(p, dist)
	if !ok {
		return
	}

	if q.options.Region != nil && !q.options.Region.ContainsPoint(p) {
		return
	}

	result := ClosestPointQueryResult{
		distance: dist,
		Point:    p,
		Data:     data,
		pointID:  id,
	}

	switch q.options.MaxResults {
	case 1:
		q.resultVector = []ClosestPointQueryResult{result}
		q.distanceLimit = result.distance.sub(q.target.distance().fromChordAngle(q.options.MaxError))
	case math.MaxInt32:
		q.resultVector = append(q.resultVector, result)
	default:
		// Priority queue
		if q.resultHeap.Len() >= q.options.MaxResults {
			if !result.Less(q.resultHeap[0]) {
				return // New result is further than current furthest in heap
			}
			heap.Pop(&q.resultHeap)
		}
		heap.Push(&q.resultHeap, result)

		if q.resultHeap.Len() >= q.options.MaxResults {
			q.distanceLimit = q.resultHeap[0].distance.sub(q.target.distance().fromChordAngle(q.options.MaxError))
		}
	}
}

const minPointsToEnqueue = 13

func (q *ClosestPointQueryBase) processOrEnqueue(id CellID, it *PointIndexIterator, seek bool) bool {
	if seek {
		it.Seek(id.RangeMin())
	}
	if id.IsLeaf() {
		for !it.Done() && it.CellID() == id {
			q.maybeAddResult(it.Point(), it.Data(), int32(it.Index()))
			it.Next()
		}
		return false
	}

	last := id.RangeMax()
	var pending []int
	count := 0

	// Check if cell has too many points
	itClone := *it
	for !itClone.Done() && itClone.CellID() <= last {
		if count == minPointsToEnqueue-1 {
			// Too many, enqueue
			cell := CellFromCellID(id)
			dist := q.distanceLimit
			dist, ok := q.target.updateDistanceToCell(cell, dist)
			if ok && (q.options.Region == nil || q.options.Region.MayIntersect(cell)) {
				if q.useConservativeCellDistance {
					dist = dist.sub(q.target.distance().fromChordAngle(q.options.MaxError))
				}
				q.queue.push(&queryQueueEntry{
					distance: dist,
					id:       id,
				})
			}
			return true // Seek to next child in caller
		}
		_ = append(pending, itClone.Index())
		count++
		itClone.Next()
	}

	// Process pending
	for i := 0; i < count; i++ {
		q.maybeAddResult(it.Point(), it.Data(), int32(it.Index()))
		it.Next()
	}
	return false
}

// resultHeap for maintaining top K results
type resultHeap []ClosestPointQueryResult

func (h resultHeap) Len() int           { return len(h) }
func (h resultHeap) Less(i, j int) bool { return !h[i].Less(h[j]) } // Max-heap based on Less
func (h resultHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *resultHeap) Push(x interface{}) {
	*h = append(*h, x.(ClosestPointQueryResult))
}
func (h *resultHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
