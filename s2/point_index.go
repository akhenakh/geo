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

import "sort"

// PointIndex stores a collection of points/data pairs.
// It allows efficient retrieval of points.
type PointIndex struct {
	points []indexedPoint
}

type indexedPoint struct {
	id   CellID
	pt   Point
	data interface{}
}

// NewPointIndex creates a new PointIndex.
func NewPointIndex() *PointIndex {
	return &PointIndex{}
}

// Add adds a point and associated data to the index.
func (p *PointIndex) Add(pt Point, data interface{}) {
	p.points = append(p.points, indexedPoint{
		id:   cellIDFromPoint(pt),
		pt:   pt,
		data: data,
	})

	// Maintain sorted order.
	last := len(p.points) - 1
	if last > 0 && p.points[last].id < p.points[last-1].id {
		sort.Slice(p.points, func(i, j int) bool {
			return p.points[i].id < p.points[j].id
		})
	}
}

// NumPoints returns the number of points in the index.
func (p *PointIndex) NumPoints() int {
	return len(p.points)
}

// Iterator returns a new iterator positioned at the beginning.
func (p *PointIndex) Iterator() *PointIndexIterator {
	return &PointIndexIterator{index: p, pos: 0}
}

// PointIndexIterator iterates over the points in the index.
type PointIndexIterator struct {
	index *PointIndex
	pos   int
}

// Next advances the iterator.
func (it *PointIndexIterator) Next() {
	it.pos++
}

// Prev moves the iterator back.
func (it *PointIndexIterator) Prev() bool {
	if it.pos > 0 {
		it.pos--
		return true
	}
	return false
}

// Done returns true if the iterator is positioned past the end.
func (it *PointIndexIterator) Done() bool {
	return it.pos >= len(it.index.points)
}

// CellID returns the CellID of the current point.
func (it *PointIndexIterator) CellID() CellID {
	if it.Done() {
		return SentinelCellID
	}
	return it.index.points[it.pos].id
}

// Point returns the current point.
func (it *PointIndexIterator) Point() Point {
	return it.index.points[it.pos].pt
}

// Data returns the data associated with the current point.
func (it *PointIndexIterator) Data() interface{} {
	return it.index.points[it.pos].data
}

// Index returns the current index in the internal slice.
func (it *PointIndexIterator) Index() int {
	return it.pos
}

// Seek positions the iterator at the first point with CellID >= id.
func (it *PointIndexIterator) Seek(id CellID) {
	it.pos = sort.Search(len(it.index.points), func(i int) bool {
		return it.index.points[i].id >= id
	})
}
