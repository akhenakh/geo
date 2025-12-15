// Copyright 2025 Google Inc. All rights reserved.
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
	"math/rand"
	"reflect"
	"sort"
	"testing"
)

type rtiQueryType int

const (
	rtiQueryTypePoint rtiQueryType = iota
	rtiQueryTypeCap
)

// rtiRandomPoint returns a random point on the sphere.
func rtiRandomPoint(rng *rand.Rand) Point {
	z := 2*rng.Float64() - 1
	phi := 2 * math.Pi * rng.Float64()
	r := math.Sqrt(1 - z*z)
	return PointFromCoords(r*math.Cos(phi), r*math.Sin(phi), z)
}

// rtiRandomCap returns a cap with a random center and area in the given range.
// The area is chosen logarithmically.
func rtiRandomCap(rng *rand.Rand, minArea, maxArea float64) Cap {
	area := minArea * math.Pow(maxArea/minArea, rng.Float64())
	return CapFromCenterArea(rtiRandomPoint(rng), area)
}

func TestRegionTermIndexerRandomCaps(t *testing.T) {
	iters := 100 // Use a smaller number for unit tests than the benchmark-style C++ test

	runTest := func(name string, opts RegionTermIndexerOptions, qType rtiQueryType) {
		t.Run(name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(12345))
			indexer := NewRegionTermIndexer(opts)
			coverer := NewRegionCoverer()
			coverer.MaxCells = opts.MaxCells
			coverer.MinLevel = opts.MinLevel
			coverer.MaxLevel = opts.MaxLevel
			coverer.LevelMod = opts.LevelMod

			var caps []Cap
			var coverings []CellUnion
			index := make(map[string][]int)

			// Build index
			for i := 0; i < iters; i++ {
				var cap Cap
				var terms []string
				if opts.IndexContainsPointsOnly {
					cap = CapFromPoint(rtiRandomPoint(rng))
					terms = indexer.GetIndexTermsForPoint(cap.Center(), "")
				} else {
					cap = rtiRandomCap(rng,
						0.3*AvgAreaMetric.Value(opts.MaxLevel),
						4.0*AvgAreaMetric.Value(opts.MinLevel))
					terms = indexer.GetIndexTerms(cap, "")
				}
				caps = append(caps, cap)
				coverings = append(coverings, coverer.Covering(cap))

				for _, term := range terms {
					index[term] = append(index[term], i)
				}
			}

			// Query
			for i := 0; i < iters; i++ {
				var cap Cap
				var terms []string

				if qType == rtiQueryTypePoint {
					cap = CapFromPoint(rtiRandomPoint(rng))
					terms = indexer.GetQueryTermsForPoint(cap.Center(), "")
				} else {
					cap = rtiRandomCap(rng,
						0.3*AvgAreaMetric.Value(opts.MaxLevel),
						4.0*AvgAreaMetric.Value(opts.MinLevel))
					terms = indexer.GetQueryTerms(cap, "")
				}

				// Compute expected results by brute force
				covering := coverer.Covering(cap)
				expected := make(map[int]bool)
				for j, _ := range caps {
					if covering.Intersects(coverings[j]) {
						expected[j] = true
					}
				}

				actual := make(map[int]bool)
				for _, term := range terms {
					if docs, ok := index[term]; ok {
						for _, docID := range docs {
							actual[docID] = true
						}
					}
				}

				if !reflect.DeepEqual(expected, actual) {
					t.Errorf("Query %d failed. Expected %d results, got %d", i, len(expected), len(actual))
				}
			}
		})
	}

	opts := DefaultRegionTermIndexerOptions()
	opts.OptimizeForSpace = false
	opts.MinLevel = 0
	opts.MaxLevel = 16
	opts.MaxCells = 20

	runTest("IndexRegionsQueryRegionsOptimizeTime", opts, rtiQueryTypeCap)

	runTest("IndexRegionsQueryPointsOptimizeTime", opts, rtiQueryTypePoint)

	opts2 := opts
	opts2.MinLevel = 6
	opts2.MaxLevel = 12
	opts2.LevelMod = 3
	runTest("IndexRegionsQueryRegionsOptimizeTimeWithLevelMod", opts2, rtiQueryTypeCap)

	opts3 := DefaultRegionTermIndexerOptions()
	opts3.OptimizeForSpace = true
	opts3.MinLevel = 4
	opts3.MaxLevel = MaxLevel
	opts3.MaxCells = 8
	runTest("IndexRegionsQueryRegionsOptimizeSpace", opts3, rtiQueryTypeCap)

	opts4 := opts
	opts4.MaxLevel = MaxLevel
	opts4.LevelMod = 2
	opts4.MaxCells = 20
	opts4.IndexContainsPointsOnly = true
	runTest("IndexPointsQueryRegionsOptimizeTime", opts4, rtiQueryTypeCap)

	opts5 := DefaultRegionTermIndexerOptions()
	opts5.OptimizeForSpace = true
	opts5.IndexContainsPointsOnly = true
	runTest("IndexPointsQueryRegionsOptimizeSpace", opts5, rtiQueryTypeCap)
}

func TestRegionTermIndexerMarkerCharacter(t *testing.T) {
	opts := DefaultRegionTermIndexerOptions()
	opts.MinLevel = 20
	opts.MaxLevel = 20
	indexer := NewRegionTermIndexer(opts)

	point := PointFromLatLng(LatLngFromDegrees(10, 20))
	if indexer.Options.Marker != '$' {
		t.Errorf("Default marker = %c, want $", indexer.Options.Marker)
	}

	terms := indexer.GetQueryTermsForPoint(point, "")
	expected := []string{"11282087039", "$11282087039"}
	sort.Strings(terms)
	sort.Strings(expected)
	if !reflect.DeepEqual(terms, expected) {
		t.Errorf("GetQueryTerms = %v, want %v", terms, expected)
	}

	opts.Marker = ':'
	indexer = NewRegionTermIndexer(opts)
	if indexer.Options.Marker != ':' {
		t.Errorf("Modified marker = %c, want :", indexer.Options.Marker)
	}
	terms = indexer.GetQueryTermsForPoint(point, "")
	expected = []string{"11282087039", ":11282087039"}
	sort.Strings(terms)
	sort.Strings(expected)
	if !reflect.DeepEqual(terms, expected) {
		t.Errorf("GetQueryTerms with marker : = %v, want %v", terms, expected)
	}
}

func TestRegionTermIndexerMaxLevelSetLoosely(t *testing.T) {
	opts := DefaultRegionTermIndexerOptions()
	opts.MinLevel = 1
	opts.LevelMod = 2
	opts.MaxLevel = 19
	indexer1 := NewRegionTermIndexer(opts)

	opts2 := opts
	opts2.MaxLevel = 20
	indexer2 := NewRegionTermIndexer(opts2)

	rng := rand.New(rand.NewSource(123))
	point := rtiRandomPoint(rng)

	terms1 := indexer1.GetIndexTermsForPoint(point, "")
	terms2 := indexer2.GetIndexTermsForPoint(point, "")
	if !reflect.DeepEqual(terms1, terms2) {
		t.Errorf("Index terms mismatch for loosely set max level. %v vs %v", terms1, terms2)
	}

	qTerms1 := indexer1.GetQueryTermsForPoint(point, "")
	qTerms2 := indexer2.GetQueryTermsForPoint(point, "")
	if !reflect.DeepEqual(qTerms1, qTerms2) {
		t.Errorf("Query terms mismatch for loosely set max level. %v vs %v", qTerms1, qTerms2)
	}

	cap := rtiRandomCap(rng, 0.001, 1.0)
	cTerms1 := indexer1.GetIndexTerms(cap, "")
	cTerms2 := indexer2.GetIndexTerms(cap, "")
	// Sort because order might differ if implementation iterates map/covering differently?
	// Covering should be deterministic.
	if !reflect.DeepEqual(cTerms1, cTerms2) {
		t.Errorf("Region index terms mismatch. %v vs %v", cTerms1, cTerms2)
	}

	cqTerms1 := indexer1.GetQueryTerms(cap, "")
	cqTerms2 := indexer2.GetQueryTerms(cap, "")
	if !reflect.DeepEqual(cqTerms1, cqTerms2) {
		t.Errorf("Region query terms mismatch. %v vs %v", cqTerms1, cqTerms2)
	}
}
