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
	"math/rand"
	"testing"

	"github.com/golang/geo/s1"
)

// set padding to at least twice the maximum error for reliable results.
const shapeIndexCellPadding = 2 * (faceClipErrorUVCoord + intersectsRectErrorUVDist)

func padCell(id CellID, paddingUV float64) Shape {
	face, i, j, _ := id.faceIJOrientation()

	uv := ijLevelToBoundUV(i, j, id.Level()).ExpandedByMargin(paddingUV)

	vertices := make([]Point, 4)
	for k, v := range uv.Vertices() {
		vertices[k] = Point{faceUVToXYZ(face, v.X, v.Y).Normalize()}
	}

	return LaxLoopFromPoints(vertices)
}

func TestShapeIndexRegionCapBound(t *testing.T) {
	id := CellIDFromString("3/0123012301230123012301230123")

	// Add a polygon that is slightly smaller than the cell being tested.
	index := NewShapeIndex()
	index.Add(padCell(id, -shapeIndexCellPadding))

	cellBound := CellFromCellID(id).CapBound()
	indexBound := NewShapeIndexRegion(index).CapBound()
	if !indexBound.Contains(cellBound) {
		t.Errorf("%v.Contains(%v) = false, want true", indexBound, cellBound)
	}

	// Note that CellUnion.CapBound returns a slightly larger bound than
	// Cell.CapBound even when the cell union consists of a single CellID.
	if got, want := indexBound.Radius(), 1.00001*cellBound.Radius(); got > want {
		t.Errorf("%v.CapBound.Radius() = %v, want %v", index, got, want)
	}
}

func TestShapeIndexRegionRectBound(t *testing.T) {
	id := CellIDFromString("3/0123012301230123012301230123")

	// Add a polygon that is slightly smaller than the cell being tested.
	index := NewShapeIndex()
	index.Add(padCell(id, -shapeIndexCellPadding))
	cellBound := CellFromCellID(id).RectBound()
	indexBound := NewShapeIndexRegion(index).RectBound()

	if indexBound != cellBound {
		t.Errorf("%v.RectBound() = %v, want %v", index, indexBound, cellBound)
	}
}

func TestShapeIndexRegionCellUnionBoundMultipleFaces(t *testing.T) {
	have := []CellID{
		CellIDFromString("3/00123"),
		CellIDFromString("2/11200013"),
	}

	index := NewShapeIndex()
	for _, id := range have {
		index.Add(padCell(id, -shapeIndexCellPadding))
	}

	got := NewShapeIndexRegion(index).CellUnionBound()

	sortCellIDs(have)
	sortCellIDs(got)

	if !CellUnion(have).Equal(CellUnion(got)) {
		t.Errorf("CellUnionBound() = %v, want %v", got, have)
	}
}

func TestShapeIndexRegionCellUnionBoundOneFace(t *testing.T) {
	// This tests consists of 3 pairs of CellIDs.  Each pair is located within
	// one of the children of face 5, namely the cells 5/0, 5/1, and 5/3.
	// We expect CellUnionBound to compute the smallest cell that bounds the
	// pair on each face.
	have := []CellID{
		CellIDFromString("5/010"),
		CellIDFromString("5/0211030"),
		CellIDFromString("5/110230123"),
		CellIDFromString("5/11023021133"),
		CellIDFromString("5/311020003003030303"),
		CellIDFromString("5/311020023"),
	}

	want := []CellID{
		CellIDFromString("5/0"),
		CellIDFromString("5/110230"),
		CellIDFromString("5/3110200"),
	}

	index := NewShapeIndex()
	for _, id := range have {
		// Add each shape 3 times to ensure that the ShapeIndex subdivides.
		index.Add(padCell(id, -shapeIndexCellPadding))
		index.Add(padCell(id, -shapeIndexCellPadding))
		index.Add(padCell(id, -shapeIndexCellPadding))
	}

	sortCellIDs(have)
	sortCellIDs(want)

	got := NewShapeIndexRegion(index).CellUnionBound()
	sortCellIDs(got)

	if !CellUnion(want).Equal(CellUnion(got)) {
		t.Errorf("CellUnionBound() = %v, want %v", got, want)
	}
}

func TestShapeIndexRegionContainsCellMultipleShapes(t *testing.T) {
	id := CellIDFromString("3/0123012301230123012301230123")

	// Add a polygon that is slightly smaller than the cell being tested.
	index := NewShapeIndex()
	index.Add(padCell(id, -shapeIndexCellPadding))
	region := NewShapeIndexRegion(index)

	if region.ContainsCell(CellFromCellID(id)) {
		t.Error("region.ContainsCell(id) = true, want false")
	}

	// Add a second polygon that is slightly larger than the cell being tested.
	// Note that Contains() should return true if *any* shape contains the cell.
	index.Add(padCell(id, shapeIndexCellPadding))
	region = NewShapeIndexRegion(index)

	if !region.ContainsCell(CellFromCellID(id)) {
		t.Error("region.ContainsCell(id) = false, want true")
	}

	// Verify that all children of the cell are also contained.
	for child := id.ChildBegin(); child != id.ChildEnd(); child = child.Next() {
		if !region.ContainsCell(CellFromCellID(child)) {
			t.Errorf("region.ContainsCell(%v) = false, want true", child)
		}
	}
}

func TestShapeIndexRegionIntersectsShrunkenCell(t *testing.T) {
	target := CellIDFromString("3/0123012301230123012301230123")

	// Add a polygon that is slightly smaller than the cell being tested.
	index := NewShapeIndex()
	index.Add(padCell(target, -shapeIndexCellPadding))
	region := NewShapeIndexRegion(index)

	// Check that the index intersects the cell itself, but not any of the
	// neighboring cells.
	if !region.IntersectsCell(CellFromCellID(target)) {
		t.Error("region.IntersectsCell(target) = false, want true")
	}

	nbrs := target.AllNeighbors(target.Level())
	for _, id := range nbrs {
		if region.IntersectsCell(CellFromCellID(id)) {
			t.Errorf("region.IntersectsCell(%v) = true, want false", id)
		}
	}
}

func TestShapeIndexRegionIntersectsExactCell(t *testing.T) {
	target := CellIDFromString("3/0123012301230123012301230123")

	// Adds a polygon that exactly follows a cell boundary.
	index := NewShapeIndex()
	index.Add(padCell(target, 0.0))
	region := NewShapeIndexRegion(index)

	// Check that the index intersects the cell and all of its neighbors.
	ids := append([]CellID{target}, target.AllNeighbors(target.Level())...)
	for _, id := range ids {
		if !region.IntersectsCell(CellFromCellID(id)) {
			t.Errorf("region.IntersectsCell(%v) = false, want true", id)
		}
	}
}

func TestShapeIndexRegionVisitIntersectingShapesPoints(t *testing.T) {
	// Points
	rnd := rand.New(rand.NewSource(12345))
	var vertices []Point
	for i := 0; i < 100; i++ {
		vertices = append(vertices, randomPoint(rnd))
	}
	index := NewShapeIndex()
	// Just add a few for structure, C++ test adds a vector shape
	index.Add(&PointVector{vertices[0], vertices[1], vertices[2]})
	// Actually C++ adds one PointVectorShape with all points. Go's PointVector is a Shape.
	pv := PointVector(vertices)
	index.Add(&pv)

	runVisitIntersectingShapesTest(t, index)
}

func TestShapeIndexRegionVisitIntersectingShapesPolylines(t *testing.T) {
	rnd := rand.New(rand.NewSource(12345))
	index := NewShapeIndex()
	centerCap := CapFromCenterAngle(PointFromCoords(1, 0, 0), 0.5*s1.Radian)

	for i := 0; i < 50; i++ {
		center := samplePoint(rnd, centerCap)
		var vertices []Point
		if rnd.Float64() < 0.1 {
			vertices = []Point{center, center} // Degenerate
		} else {
			vertices = regularPoints(center, s1.Angle(rnd.Float64())*s1.Radian, rnd.Intn(20)+3)
		}
		index.Add(LaxPolylineFromPoints(vertices))
	}
	runVisitIntersectingShapesTest(t, index)
}

func TestShapeIndexRegionVisitIntersectingShapesPolygons(t *testing.T) {
	rnd := rand.New(rand.NewSource(12345))
	index := NewShapeIndex()
	centerCap := CapFromCenterAngle(PointFromCoords(1, 0, 0), 0.5*s1.Radian)

	for i := 0; i < 10; i++ {
		center := samplePoint(rnd, centerCap)
		// Use regular loops as simple fractal replacement
		loop := RegularLoop(center, s1.Angle(rnd.Float64())*s1.Radian, rnd.Intn(20)+3)
		index.Add(loop)
	}
	index.Add(padCell(CellIDFromFace(0), 0))
	runVisitIntersectingShapesTest(t, index)
}

func runVisitIntersectingShapesTest(t *testing.T, index *ShapeIndex) {
	region := NewShapeIndexRegion(index)
	iter := index.Iterator()

	// Create an ShapeIndex for each shape in the original index
	var shapeIndexes []*ShapeIndex
	for i := 0; i < len(index.shapes); i++ {
		si := NewShapeIndex()
		si.Add(index.Shape(int32(i)))
		shapeIndexes = append(shapeIndexes, si)
	}

	var testCell func(target Cell)
	testCell = func(target Cell) {
		shapeContains := make(map[int32]bool)
		region.VisitIntersectingShapes(target, func(shape Shape, containsTarget bool) bool {
			id := index.idForShape(shape)
			if _, ok := shapeContains[id]; ok {
				t.Errorf("Shape %d visited twice", id)
			}
			shapeContains[id] = containsTarget
			return true
		})

		for sIdx, si := range shapeIndexes {
			shapeRegion := NewShapeIndexRegion(si)
			if !shapeRegion.IntersectsCell(target) {
				if _, ok := shapeContains[int32(sIdx)]; ok {
					t.Errorf("Shape %d shouldn't intersect, but was visited", sIdx)
				}
			} else {
				visitedContains, ok := shapeContains[int32(sIdx)]
				if !ok {
					t.Errorf("Shape %d should intersect, but wasn't visited", sIdx)
				} else {
					actualContains := shapeRegion.ContainsCell(target)
					if visitedContains != actualContains {
						t.Errorf("Shape %d containment mismatch: visited=%v, actual=%v", sIdx, visitedContains, actualContains)
					}
				}
			}
		}

		relation := iter.LocateCellID(target.ID())
		switch relation {
		case Disjoint:
			return
		case Subdivided:
			if children, ok := target.Children(); ok {
				for _, child := range children {
					testCell(child)
				}
			}
		case Indexed:
			// Randomly check descendants
			if target.IsLeaf() || rand.Float64() < 1.0/3.0 {
				return
			}
			children, _ := target.Children()
			testCell(children[rand.Intn(4)])
		}
	}

	// Test all faces
	for i := 0; i < 6; i++ {
		testCell(CellFromCellID(CellIDFromFace(i)))
	}
}

func samplePoint(rng *rand.Rand, cap Cap) Point {
	// Crude approximation for sampling a point in a cap
	for {
		p := randomPoint(rng)
		if cap.ContainsPoint(p) {
			return p
		}
	}
}

// TODO(roberts): remaining tests
// Add VisitIntersectingShapes tests
// Benchmarks
