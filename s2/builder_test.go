package s2

import (
	"math"
	"testing"
)

func TestBooleanOperationIntersection(t *testing.T) {
	// Create two overlapping squares.
	// Poly A: 0,0 to 2,2
	// Poly B: 1,1 to 3,3
	// Intersection should be 1,1 to 2,2 (a square of size 1x1).

	// Square centered at 1,1 size 2.0 -> spans 0,0 to 2,2 approx
	p1 := makeSquare(1.0, 1.0, 2.0)

	// Square centered at 2,2 size 2.0 -> spans 1,1 to 3,3 approx
	p2 := makeSquare(2.0, 2.0, 2.0)

	idxA := NewShapeIndex()
	idxA.Add(p1)

	idxB := NewShapeIndex()
	idxB.Add(p2)

	var output Polygon
	layer := NewPolygonLayer(&output)

	op := NewBooleanOperation(BooleanOperationOpTypeIntersection, layer, nil)
	err := op.Build(idxA, idxB)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should result in a single polygon loop
	if output.NumLoops() != 1 {
		t.Fatalf("Expected 1 loop in intersection, got %d", output.NumLoops())
	}

	// Area check
	// Intersection is approx 1x1 degree square.
	// 1 degree ~ 0.017 radians. Area ~ 0.017*0.017 = 0.00029
	area := output.Area()
	expectedArea := (math.Pi / 180.0) * (math.Pi / 180.0)

	if math.Abs(area-expectedArea) > 0.0001 {
		t.Errorf("Area mismatch. Expected ~%v, got %v", expectedArea, area)
	}

	// Verify center point of intersection is contained
	intersectionCenter := PointFromLatLng(LatLngFromDegrees(1.5, 1.5))
	if !output.ContainsPoint(intersectionCenter) {
		t.Error("Intersection result does not contain center (1.5, 1.5)")
	}

	// Verify points outside are not contained
	if output.ContainsPoint(PointFromLatLng(LatLngFromDegrees(0.5, 0.5))) {
		t.Error("Intersection result contains (0.5, 0.5) which should be excluded")
	}
}

func TestBooleanOperationDifference(t *testing.T) {
	// A: Large square 0,0 to 4,4
	// B: Small square 1,1 to 3,3 (hole inside A)
	// A - B should be A with a hole.

	p1 := makeSquare(2.0, 2.0, 4.0)
	p2 := makeSquare(2.0, 2.0, 2.0)

	idxA := NewShapeIndex()
	idxA.Add(p1)

	idxB := NewShapeIndex()
	idxB.Add(p2)

	var output Polygon
	layer := NewPolygonLayer(&output)

	op := NewBooleanOperation(BooleanOperationOpTypeDifference, layer, nil)
	err := op.Build(idxA, idxB)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should have 2 loops (1 shell, 1 hole) if our polygon assembly is smart,
	// OR it might output just the edges if the naive builder layer doesn't reconstruct hierarchy perfectly.
	// Our naive PolygonLayer builder in the previous step just dumps loops.
	// S2Polygon requires proper nesting initialization to identify holes.
	// Let's check loops count.
	if output.NumLoops() != 2 {
		t.Errorf("Expected 2 loops (shell + hole) for difference, got %d", output.NumLoops())
	}

	// Check containment
	if !output.ContainsPoint(PointFromLatLng(LatLngFromDegrees(0.5, 0.5))) {
		t.Error("Difference should contain 0.5,0.5")
	}
	if output.ContainsPoint(PointFromLatLng(LatLngFromDegrees(2.0, 2.0))) {
		t.Error("Difference should NOT contain 2.0,2.0 (it is in the hole)")
	}
}

func makeSquare(lat, lng, sizeDeg float64) *Polygon {
	half := sizeDeg / 2.0
	pts := []Point{
		PointFromLatLng(LatLngFromDegrees(lat+half, lng+half)),
		PointFromLatLng(LatLngFromDegrees(lat+half, lng-half)),
		PointFromLatLng(LatLngFromDegrees(lat-half, lng-half)),
		PointFromLatLng(LatLngFromDegrees(lat-half, lng+half)),
	}
	// Create a Loop
	loop := LoopFromPoints(pts)
	// Create a Polygon from that Loop
	return PolygonFromLoops([]*Loop{loop})
}
