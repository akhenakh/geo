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

	"github.com/golang/geo/s1"
)

// BufferOperationEndCapStyle specifies whether polyline end caps should be round or flat.
type BufferOperationEndCapStyle int

const (
	BufferOperationEndCapStyleRound BufferOperationEndCapStyle = iota
	BufferOperationEndCapStyleFlat
)

// BufferOperationPolylineSide specifies whether polylines should be buffered on one or both sides.
type BufferOperationPolylineSide int

const (
	BufferOperationPolylineSideLeft BufferOperationPolylineSide = iota
	BufferOperationPolylineSideRight
	BufferOperationPolylineSideBoth
)

// BufferOperationOptions controls the behavior of the buffer operation.
type BufferOperationOptions struct {
	BufferRadius  s1.Angle
	ErrorFraction float64
	EndCapStyle   BufferOperationEndCapStyle
	PolylineSide  BufferOperationPolylineSide
	SnapFunction  SnapFunction
}

// DefaultBufferOperationOptions returns default options.
func DefaultBufferOperationOptions() BufferOperationOptions {
	return BufferOperationOptions{
		BufferRadius:  s1.Angle(0),
		ErrorFraction: 0.01,
		EndCapStyle:   BufferOperationEndCapStyleRound,
		PolylineSide:  BufferOperationPolylineSideBoth,
		SnapFunction:  NewIdentitySnapFunction(s1.Angle(0)),
	}
}

// BufferOperation expands geometry by a fixed radius.
type BufferOperation struct {
	Options     BufferOperationOptions
	ResultLayer BuilderLayer

	// Internal state
	bufferSign int // -1, 0, 1
	absRadius  s1.Angle
	vertexStep s1.Angle
	edgeStep   s1.Angle
	pointStep  s1.Angle

	path       []Point
	refPoint   Point
	refWinding int

	sweepA Point
	sweepB Point

	inputStart      Point
	offsetStart     Point
	haveInputStart  bool
	haveOffsetStart bool

	tmpVertices []Point
}

// NewBufferOperation creates a new buffer operation.
func NewBufferOperation(layer BuilderLayer, opts BufferOperationOptions) *BufferOperation {
	op := &BufferOperation{
		ResultLayer: layer,
		Options:     opts,
		refPoint:    OriginPoint(),
	}
	op.init()
	return op
}

func (op *BufferOperation) init() {
	if op.Options.BufferRadius > 0 {
		op.bufferSign = 1
	} else if op.Options.BufferRadius < 0 {
		op.bufferSign = -1
	} else {
		op.bufferSign = 0
	}

	op.absRadius = s1.Angle(math.Abs(float64(op.Options.BufferRadius)))

	// Calculate step sizes for arcs based on error fraction
	// Approximate number of segments for a full circle
	circleSegments := math.Pi / math.Acos(1-op.Options.ErrorFraction)
	op.vertexStep = s1.Angle(2 * math.Pi / circleSegments)
	op.edgeStep = op.vertexStep
	op.pointStep = op.vertexStep
}

// Build executes the operation.
func (op *BufferOperation) Build(index *ShapeIndex) error {
	for _, shape := range index.shapes {
		op.bufferShape(shape)
	}
	// Note: Full build logic requiring WindingOperation and S2Builder is stubbed here.
	return nil
}

// bufferShape dispatches based on dimension.
func (op *BufferOperation) bufferShape(shape Shape) {
	for i := 0; i < shape.NumChains(); i++ {
		chain := shape.Chain(i)
		if chain.Length == 0 {
			continue
		}

		if shape.Dimension() == 0 {
			// Point
			pt := shape.Edge(chain.Start).V0
			op.AddPoint(pt)
		} else {
			// Polyline or Loop
			var vertices []Point
			startEdge := shape.ChainEdge(i, 0)
			vertices = append(vertices, startEdge.V0)
			for j := 0; j < chain.Length; j++ {
				e := shape.ChainEdge(i, j)
				vertices = append(vertices, e.V1)
			}

			if shape.Dimension() == 1 {
				op.AddPolyline(vertices)
			} else { // Dimension 2
				op.AddLoop(vertices)
			}
		}
	}
}

// AddPoint adds a buffered point (a circle).
func (op *BufferOperation) AddPoint(p Point) {
	if op.bufferSign == 0 {
		return
	}

	op.setInputVertex(p)
	start := Ortho(p)
	angle := s1.Angle(0)

	right := s1.Angle(math.Pi / 2)

	for i := 0; i < 4; i++ {
		rotateDir := Point{p.Cross(start.Vector).Normalize()}

		for angle < right {
			dir := PointOnRay(start, rotateDir, angle)
			pOffset := PointOnRay(p, dir, op.absRadius)
			op.addOffsetVertex(pOffset)
			angle += op.pointStep
		}
		angle -= right
		start = rotateDir
	}
	op.closeBufferRegion()
	op.outputPath()
}

// AddPolyline adds a buffered polyline.
func (op *BufferOperation) AddPolyline(pts []Point) {
	if len(pts) < 2 {
		return
	}

	op.setInputVertex(pts[0])
	op.addStartCap(pts[0], pts[1])

	for i := 0; i < len(pts)-2; i++ {
		op.bufferEdgeAndVertex(pts[i], pts[i+1], pts[i+2])
	}

	n := len(pts)
	op.addEdgeArc(pts[n-2], pts[n-1])
	op.addEndCap(pts[n-2], pts[n-1])

	// Return trip for other side (simplified)
	for i := n - 3; i >= 0; i-- {
		op.bufferEdgeAndVertex(pts[i+2], pts[i+1], pts[i])
	}
	op.addEdgeArc(pts[1], pts[0])
	op.closeBufferRegion()
	op.outputPath()
}

// AddLoop adds a buffered loop.
func (op *BufferOperation) AddLoop(pts []Point) {
	if len(pts) < 3 {
		return
	}

	op.setInputVertex(pts[0])
	for i := 0; i < len(pts); i++ {
		p0 := pts[i]
		p1 := pts[(i+1)%len(pts)]
		p2 := pts[(i+2)%len(pts)]
		op.bufferEdgeAndVertex(p0, p1, p2)
	}
	op.closeBufferRegion()
	op.outputPath()
}

func (op *BufferOperation) bufferEdgeAndVertex(a, b, c Point) {
	op.addEdgeArc(a, b)

	sign := RobustSign(a, b, c)
	isConvex := (op.bufferSign * int(sign)) >= 0

	if isConvex {
		start := op.getEdgeAxis(a, b)
		end := op.getEdgeAxis(b, c)
		op.addVertexArc(b, start, end)
	} else {
		op.closeEdgeArc(a, b)
		op.addOffsetVertex(b)
	}
}

func (op *BufferOperation) addStartCap(a, b Point) {
	axis := op.getEdgeAxis(a, b)
	if op.Options.EndCapStyle == BufferOperationEndCapStyleRound {
		negAxis := Point{axis.Mul(-1)}
		op.addVertexArc(a, negAxis, axis)
	} else {
		p := PointOnRay(a, Point{axis.Mul(-1)}, op.absRadius)
		op.addOffsetVertex(p)
	}
}

func (op *BufferOperation) addEndCap(a, b Point) {
	axis := op.getEdgeAxis(a, b)
	if op.Options.EndCapStyle == BufferOperationEndCapStyleRound {
		negAxis := Point{axis.Mul(-1)}
		op.addVertexArc(b, axis, negAxis)
	} else {
		op.closeEdgeArc(a, b)
	}
}

func (op *BufferOperation) addEdgeArc(a, b Point) {
	axis := op.getEdgeAxis(a, b)
	p := PointOnRay(b, axis, op.absRadius)
	op.addOffsetVertex(p)
	op.setInputVertex(b)
}

func (op *BufferOperation) closeEdgeArc(a, b Point) {
	axis := op.getEdgeAxis(a, b)
	p := PointOnRay(b, axis, op.absRadius)
	op.addOffsetVertex(p)
}

func (op *BufferOperation) addVertexArc(v, start, end Point) {
	rotateDir := Point{v.Cross(start.Vector).Normalize()}
	if op.bufferSign < 0 {
		rotateDir = Point{rotateDir.Mul(-1)}
	}

	// Simplified arc logic for port: just add end point
	p := PointOnRay(v, end, op.absRadius)
	op.addOffsetVertex(p)
}

func (op *BufferOperation) closeVertexArc(v, end Point) {
	p := PointOnRay(v, end, op.absRadius)
	op.addOffsetVertex(p)
}

func (op *BufferOperation) getEdgeAxis(a, b Point) Point {
	cross := b.PointCross(a).Normalize()
	if op.bufferSign < 0 {
		return Point{cross.Mul(-1)}
	}
	return Point{cross}
}

func (op *BufferOperation) setInputVertex(p Point) {
	if op.haveInputStart {
		// Update winding (stub)
	} else {
		op.inputStart = p
		op.haveInputStart = true
	}
	op.sweepA = p
}

func (op *BufferOperation) addOffsetVertex(p Point) {
	op.path = append(op.path, p)
	if op.haveOffsetStart {
		// Update winding (stub)
	} else {
		op.offsetStart = p
		op.haveOffsetStart = true
	}
	op.sweepB = p
}

func (op *BufferOperation) closeBufferRegion() {
	if op.haveOffsetStart && op.haveInputStart {
		// Update winding (stub)
	}
}

func (op *BufferOperation) outputPath() {
	if layer, ok := op.ResultLayer.(*PolygonLayer); ok {
		if len(op.path) >= 3 {
			loop := LoopFromPoints(op.path)
			// Naively append loop
			layer.polygon.loops = append(layer.polygon.loops, loop)
		}
	}

	op.path = nil
	op.haveInputStart = false
	op.haveOffsetStart = false
}
