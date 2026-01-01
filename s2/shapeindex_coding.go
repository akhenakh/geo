package s2

import (
	"bytes"
	"fmt"
	"io"
)

// Encode encodes the ShapeIndexCell to the encoder.
func (s *ShapeIndexCell) Encode(numShapeIDs int, e *encoder) {
	if numShapeIDs == 1 {
		// Optimization for single-shape index
		clipped := s.shapes[0]
		n := clipped.numEdges()

		// Heuristic matching C++ implementation
		if n >= 2 && n <= 17 && clipped.edges[n-1]-clipped.edges[0] == n-1 {
			// Contiguous range
			val := (uint64(clipped.edges[0]) << 6) | (uint64(n-2) << 2) | (uint64Bool(clipped.containsCenter) << 1)
			e.writeUvarint(val)
		} else if n == 1 {
			// Single edge
			val := (uint64(clipped.edges[0]) << 3) | (uint64Bool(clipped.containsCenter) << 2) | 1
			e.writeUvarint(val)
		} else {
			// General case
			val := (uint64(n) << 3) | (uint64Bool(clipped.containsCenter) << 2) | 3
			e.writeUvarint(val)
			s.encodeEdges(clipped, e)
		}
	} else {
		if len(s.shapes) > 1 {
			e.writeUvarint((uint64(len(s.shapes)) << 3) | 3)
		}

		shapeIDBase := int32(0)
		for _, clipped := range s.shapes {
			shapeDelta := clipped.shapeID - shapeIDBase
			shapeIDBase = clipped.shapeID + 1

			n := clipped.numEdges()
			if n >= 1 && n <= 16 && clipped.edges[n-1]-clipped.edges[0] == n-1 {
				// Contiguous range
				e.writeUvarint((uint64(clipped.edges[0]) << 2) | (uint64Bool(clipped.containsCenter) << 1))
				e.writeUvarint((uint64(shapeDelta) << 4) | uint64(n-1))
			} else if n == 0 {
				// No edges (interior)
				e.writeUvarint((uint64(shapeDelta) << 4) | (uint64Bool(clipped.containsCenter) << 3) | 7)
			} else {
				// General
				e.writeUvarint((uint64(n-1) << 3) | (uint64Bool(clipped.containsCenter) << 2) | 1)
				e.writeUvarint(uint64(shapeDelta))
				s.encodeEdges(clipped, e)
			}
		}
	}
}

func (s *ShapeIndexCell) encodeEdges(clipped *clippedShape, e *encoder) {
	edgeIDBase := 0
	for i := 0; i < len(clipped.edges); i++ {
		edgeID := clipped.edges[i]
		delta := edgeID - edgeIDBase

		if i+1 == len(clipped.edges) {
			e.writeUvarint(uint64(delta))
		} else {
			count := 1
			for i+1 < len(clipped.edges) && clipped.edges[i+1] == edgeID+count {
				i++
				count++
			}
			if count < 8 {
				e.writeUvarint((uint64(delta) << 3) | uint64(count-1))
			} else {
				e.writeUvarint((uint64(count-8) << 3) | 7)
				e.writeUvarint(uint64(delta))
			}
			edgeIDBase = edgeID + count
		}
	}
}

// Decode decodes a ShapeIndexCell from the decoder.
func (s *ShapeIndexCell) Decode(numShapeIDs int, d *decoder) error {
	if numShapeIDs == 1 {
		header := d.readUvarint()
		if d.err != nil {
			return d.err
		}

		s.shapes = make([]*clippedShape, 1)
		clipped := &clippedShape{shapeID: 0}
		s.shapes[0] = clipped

		if (header & 1) == 0 {
			// Contiguous
			numEdges := int((header>>2)&15) + 2
			clipped.containsCenter = (header & 2) != 0
			startEdge := int(header >> 6)
			clipped.edges = make([]int, numEdges)
			for i := 0; i < numEdges; i++ {
				clipped.edges[i] = startEdge + i
			}
		} else if (header & 2) == 0 {
			// Single edge
			clipped.edges = []int{int(header >> 3)}
			clipped.containsCenter = (header & 4) != 0
		} else {
			// General
			numEdges := int(header >> 3)
			clipped.containsCenter = (header & 4) != 0
			clipped.edges = make([]int, numEdges)
			if err := s.decodeEdges(numEdges, clipped, d); err != nil {
				return err
			}
		}
	} else {
		header := d.readUvarint()
		numClipped := 1
		if (header & 7) == 3 {
			numClipped = int(header >> 3)
			header = d.readUvarint()
		}

		s.shapes = make([]*clippedShape, numClipped)
		shapeID := int32(0)

		for i := 0; i < numClipped; i++ {
			if i > 0 {
				header = d.readUvarint()
			}
			if d.err != nil {
				return d.err
			}

			clipped := &clippedShape{}
			s.shapes[i] = clipped

			if (header & 1) == 0 {
				// Contiguous
				shapeIDCount := d.readUvarint()
				shapeID += int32(shapeIDCount >> 4)
				clipped.shapeID = shapeID

				numEdges := int((shapeIDCount & 15) + 1)
				clipped.containsCenter = (header & 2) != 0
				startEdge := int(header >> 2)

				clipped.edges = make([]int, numEdges)
				for j := 0; j < numEdges; j++ {
					clipped.edges[j] = startEdge + j
				}
			} else if (header & 7) == 7 {
				// No edges
				shapeID += int32(header >> 4)
				clipped.shapeID = shapeID
				clipped.containsCenter = (header & 8) != 0
			} else {
				// General
				if (header & 3) != 1 {
					return fmt.Errorf("invalid encoded shape header: %d", header)
				}
				shapeDelta := d.readUvarint()
				shapeID += int32(shapeDelta)
				clipped.shapeID = shapeID

				numEdges := int(header>>3) + 1
				clipped.containsCenter = (header & 4) != 0
				clipped.edges = make([]int, numEdges)
				if err := s.decodeEdges(numEdges, clipped, d); err != nil {
					return err
				}
			}
			shapeID++ // Prepare for next iteration delta
		}
	}
	return d.err
}

func (s *ShapeIndexCell) decodeEdges(numEdges int, clipped *clippedShape, d *decoder) error {
	edgeID := 0
	for i := 0; i < numEdges; {
		delta := d.readUvarint()
		if i+1 == numEdges {
			edgeID += int(delta)
			clipped.edges[i] = edgeID
			i++
		} else {
			count := int(delta&7) + 1
			delta >>= 3
			if count == 8 {
				count = int(delta) + 8
				delta = d.readUvarint()
			}
			edgeID += int(delta)
			for j := 0; j < count; j++ {
				if i >= numEdges {
					return fmt.Errorf("too many edges in decode")
				}
				clipped.edges[i] = edgeID + j
				i++
			}
			edgeID += count // update base for next delta
		}
	}
	return d.err
}

func uint64Bool(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

// bufferWriter matches io.Writer but writes to a bytes.Buffer
type bufferWriter struct {
	buf *bytes.Buffer
}

func (w *bufferWriter) Write(p []byte) (n int, err error) {
	w.buf.Write(p)
	return len(p), nil
}

// Encode encodes the shape index to the writer.
func (index *ShapeIndex) Encode(w io.Writer) error {
	e := &encoder{w: w}

	// Encode max edges and version
	val := uint64(index.maxEdgesPerCell << 2) // version 0
	e.writeUvarint(val)

	// Encode CellIDs
	encodeS2CellIdVector(index.cells, e)

	// Encode Cells using pooled buffer for efficiency
	svEnc := &stringVectorEncoder{}

	// Reuse a single buffer and encoder for all cells
	var buf bytes.Buffer
	cellEnc := &encoder{w: &bufferWriter{buf: &buf}}

	numShapes := index.Len()

	for _, id := range index.cells {
		cell := index.cellMap[id]
		buf.Reset()
		// Reset error state
		cellEnc.err = nil

		cell.Encode(numShapes, cellEnc)

		if cellEnc.err != nil {
			return cellEnc.err
		}

		svEnc.Add(buf.Bytes())
	}

	svEnc.encode(e)
	return e.err
}
