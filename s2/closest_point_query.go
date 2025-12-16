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

import "github.com/golang/geo/s1"

// ClosestPointQueryOptions defines options for the query.
type ClosestPointQueryOptions struct {
	ClosestPointQueryBaseOptions
}

// NewClosestPointQueryOptions returns default options.
func NewClosestPointQueryOptions() ClosestPointQueryOptions {
	return ClosestPointQueryOptions{
		ClosestPointQueryBaseOptions: NewClosestPointQueryBaseOptions(),
	}
}

func (o *ClosestPointQueryOptions) MaxResults(n int) *ClosestPointQueryOptions {
	o.ClosestPointQueryBaseOptions.MaxResults = n
	return o
}

func (o *ClosestPointQueryOptions) MaxDistance(d s1.ChordAngle) *ClosestPointQueryOptions {
	o.ClosestPointQueryBaseOptions.MaxDistance = minDistance(d)
	return o
}

func (o *ClosestPointQueryOptions) InclusiveMaxDistance(d s1.ChordAngle) *ClosestPointQueryOptions {
	o.ClosestPointQueryBaseOptions.MaxDistance = minDistance(d.Successor())
	return o
}

func (o *ClosestPointQueryOptions) ConservativeMaxDistance(d s1.ChordAngle) *ClosestPointQueryOptions {
	o.ClosestPointQueryBaseOptions.MaxDistance = minDistance(d.Expanded(minUpdateDistanceMaxError(d)))
	return o
}

// ClosestPointQuery finds the closest point(s) to a given target.
type ClosestPointQuery struct {
	base *ClosestPointQueryBase
	opts ClosestPointQueryOptions
}

// NewClosestPointQuery creates a new query.
func NewClosestPointQuery(index *PointIndex, opts *ClosestPointQueryOptions) *ClosestPointQuery {
	if opts == nil {
		def := NewClosestPointQueryOptions()
		opts = &def
	}
	return &ClosestPointQuery{
		base: NewClosestPointQueryBase(index),
		opts: *opts,
	}
}

// ReInit reinitializes the query.
func (q *ClosestPointQuery) ReInit() {
	q.base.ReInit()
}

// Index returns the underlying index.
func (q *ClosestPointQuery) Index() *PointIndex {
	return q.base.index
}

// FindClosestPoints returns the closest points to the target.
func (q *ClosestPointQuery) FindClosestPoints(target distanceTarget) []ClosestPointQueryResult {
	return q.base.FindClosestPoints(target, q.opts.ClosestPointQueryBaseOptions)
}

// FindClosestPoint returns the single closest point.
func (q *ClosestPointQuery) FindClosestPoint(target distanceTarget) ClosestPointQueryResult {
	opts := q.opts.ClosestPointQueryBaseOptions
	opts.MaxResults = 1
	return q.base.FindClosestPoint(target, opts)
}

// GetDistance returns the minimum distance to the target.
func (q *ClosestPointQuery) GetDistance(target distanceTarget) s1.ChordAngle {
	return q.FindClosestPoint(target).Distance()
}

// IsDistanceLess returns true if the distance to target is less than limit.
func (q *ClosestPointQuery) IsDistanceLess(target distanceTarget, limit s1.ChordAngle) bool {
	opts := q.opts.ClosestPointQueryBaseOptions
	opts.MaxResults = 1
	opts.MaxDistance = minDistance(limit)
	opts.MaxError = s1.StraightChordAngle
	return !q.base.FindClosestPoint(target, opts).IsEmpty()
}
