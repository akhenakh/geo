package s2

import (
	"github.com/golang/geo/s1"
)

// SnapFunction restricts the locations of the output vertices in S2Builder.
type SnapFunction interface {
	// SnapRadius returns the maximum distance that vertices can move when snapped.
	SnapRadius() s1.Angle

	// MinVertexSeparation returns the guaranteed minimum distance between vertices in the output.
	MinVertexSeparation() s1.Angle

	// MinEdgeVertexSeparation returns the guaranteed minimum spacing between edges and non-incident vertices.
	MinEdgeVertexSeparation() s1.Angle

	// SnapPoint returns a candidate snap site for the given point.
	SnapPoint(point Point) Point

	// Clone returns a deep copy of this SnapFunction.
	Clone() SnapFunction
}

// IdentitySnapFunction snaps every vertex to itself.
type IdentitySnapFunction struct {
	snapRadius s1.Angle
}

// NewIdentitySnapFunction creates a snap function that preserves vertices exactly
// unless they are closer than the given radius.
func NewIdentitySnapFunction(snapRadius s1.Angle) *IdentitySnapFunction {
	return &IdentitySnapFunction{snapRadius: snapRadius}
}

func (f *IdentitySnapFunction) SnapRadius() s1.Angle {
	return f.snapRadius
}

func (f *IdentitySnapFunction) MinVertexSeparation() s1.Angle {
	return f.snapRadius
}

func (f *IdentitySnapFunction) MinEdgeVertexSeparation() s1.Angle {
	return 0.5 * f.snapRadius
}

func (f *IdentitySnapFunction) SnapPoint(p Point) Point {
	return p
}

func (f *IdentitySnapFunction) Clone() SnapFunction {
	return &IdentitySnapFunction{snapRadius: f.snapRadius}
}
