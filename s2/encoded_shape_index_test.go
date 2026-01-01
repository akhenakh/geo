package s2

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestEncodedUintVector(t *testing.T) {
	values := []uint64{1, 500, 100000, 0, 9999999999}

	var buf bytes.Buffer
	e := &encoder{w: &buf}
	encodeUintVector(values, e)

	if e.err != nil {
		t.Fatalf("Encode failed: %v", e.err)
	}

	encoded := buf.Bytes()

	v := &EncodedUintVector[uint64]{}
	d := &decoder{r: bytes.NewReader(encoded)}
	if err := v.init(d); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if v.Size() != len(values) {
		t.Errorf("Size mismatch: got %d, want %d", v.Size(), len(values))
	}

	for i, want := range values {
		if got := v.Get(i); got != want {
			t.Errorf("Get(%d) = %d, want %d", i, got, want)
		}
	}

	idx := v.LowerBound(100000)
	if idx != 2 {
		t.Errorf("LowerBound(100000) = %d, want 2", idx)
	}
}

func TestEncodedS2ShapeIndexRoundTrip(t *testing.T) {
	// 1. Create a Mutable Index (standard ShapeIndex)
	index := NewShapeIndex()

	// Add some shapes
	// Polyline
	polyline := PolylineFromLatLngs([]LatLng{
		LatLngFromDegrees(0, 0),
		LatLngFromDegrees(0, 10),
		LatLngFromDegrees(10, 10),
	})
	index.Add(polyline)

	// Point
	points := PointVector{PointFromCoords(1, 0, 0)}
	index.Add(&points)

	// Force build
	index.Build()

	// 2. Encode it
	var buf bytes.Buffer
	if err := index.Encode(&buf); err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	// 3. Decode into EncodedS2ShapeIndex
	encodedData := buf.Bytes()

	encodedIndex := NewEncodedS2ShapeIndex()
	// We need a ShapeFactory.
	factory := &BasicShapeFactory{
		Shapes: []Shape{polyline, &points},
	}

	if err := encodedIndex.Init(bytes.NewReader(encodedData), factory); err != nil {
		t.Fatalf("Init encoded index failed: %v", err)
	}

	// 4. Verify contents
	// Verify CellIDs
	it := encodedIndex.Iterator()
	origIt := index.Iterator()

	count := 0
	for !it.Done() && !origIt.Done() {
		if it.CellID() != origIt.CellID() {
			t.Errorf("Iter mismatch at %d: got %v, want %v", count, it.CellID(), origIt.CellID())
		}

		// Verify cell contents
		cell := it.IndexCell()
		origCell := origIt.IndexCell()

		if len(cell.shapes) != len(origCell.shapes) {
			t.Errorf("Cell %v shape count mismatch: got %d, want %d", it.CellID(), len(cell.shapes), len(origCell.shapes))
		}

		for i := 0; i < len(cell.shapes); i++ {
			c1 := cell.shapes[i]
			c2 := origCell.shapes[i]
			if c1.shapeID != c2.shapeID {
				t.Errorf("ShapeID mismatch in cell %v: %d vs %d", it.CellID(), c1.shapeID, c2.shapeID)
			}
			if len(c1.edges) != len(c2.edges) {
				t.Errorf("Edge count mismatch: %d vs %d", len(c1.edges), len(c2.edges))
			}
		}

		it.Next()
		origIt.Next()
		count++
	}

	if !it.Done() || !origIt.Done() {
		t.Errorf("Iterator length mismatch")
	}
}

func TestEncodedS2ShapeIndexJavaByteCompatibility(t *testing.T) {
	// Replicates the C++ JavaByteCompatibility test.
	// 1. Setup Expected Index
	expected := NewShapeIndex()
	// Shape 0: 0:0, 1:1 (Removed later)
	p1 := PolylineFromLatLngs([]LatLng{
		LatLngFromDegrees(0, 0),
		LatLngFromDegrees(1, 1),
	})
	// In C++ test, this adds the shape. In Go, we add it then remove it.
	expected.Add(p1)

	// Shape 1: 1:1, 2:2
	p2 := PolylineFromLatLngs([]LatLng{
		LatLngFromDegrees(1, 1),
		LatLngFromDegrees(2, 2),
	})
	expected.Add(p2)

	// Release 0. Go ShapeIndex.Remove() deletes from the map, so Shape(0) becomes nil.
	expected.Remove(p1)
	expected.Build()

	// 2. Decode from bytes
	// This hex string contains:
	// - Tagged Shapes (StringVector):
	//   - Shape 0: empty string (because it was null)
	//   - Shape 1: Encoded Polyline
	// - Shape Index
	hexStr := "100036020102000000B4825F3C81FDEF3F27DCF7C958DE913F1EDD892B0BDF913FFC7FB8" +
		"B805F6EF3F28516A6D8FDBA13F27DCF7C958DEA13F28C809010408020010"

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		t.Fatal(err)
	}

	d := &decoder{r: bytes.NewReader(data)}

	// 2a. Decode Shapes manually (mimicking a TaggedShapeFactory)
	var shapeVec EncodedStringVector
	if err := shapeVec.init(d); err != nil {
		t.Fatal(err)
	}

	shapes := make([]Shape, shapeVec.Size())
	for i := 0; i < shapeVec.Size(); i++ {
		blob := shapeVec.Get(i)
		if len(blob) == 0 {
			continue
		}

		// Decode tag
		bd := &decoder{r: bytes.NewReader([]byte(blob))}
		tag := bd.readUvarint()

		if tag == 2 { // S2Polyline::Shape::kTypeTag == 2
			var p Polyline
			if err := p.Decode(bd.r); err != nil {
				t.Fatalf("Failed to decode polyline %d: %v", i, err)
			}
			shapes[i] = &p
		}
	}

	factory := &BasicShapeFactory{Shapes: shapes}

	// 2b. Decode Index
	actual := NewEncodedS2ShapeIndex()
	// Pass the remaining stream to Init
	// d.r is io.Reader, but we need to ensure we read from current pos.
	// Since d.r is bytes.Reader (passed above), and decoder read methods advance it,
	// passing d.r directly works.
	if err := actual.Init(d.r, factory); err != nil {
		t.Fatal(err)
	}

	// 3. Compare Results
	// Check Shape 0 is nil
	if s0 := actual.Shape(0); s0 != nil {
		t.Errorf("Shape 0 should be nil, got %v", s0)
	}

	// Check Shape 1
	s1 := actual.Shape(1)
	if s1 == nil {
		t.Fatal("Shape 1 should not be nil")
	}

	p2Actual, ok := s1.(*Polyline)
	if !ok {
		t.Fatalf("Shape 1 should be *Polyline, got %T", s1)
	}

	if !p2.ApproxEqual(p2Actual) {
		t.Errorf("Shape 1 mismatch. Want %v, got %v", p2, p2Actual)
	}

	// Compare Index Contents via Iterators
	expectIt := expected.Iterator()
	actualIt := actual.Iterator()

	for !expectIt.Done() && !actualIt.Done() {
		if expectIt.CellID() != actualIt.CellID() {
			t.Errorf("CellID mismatch: %v != %v", expectIt.CellID(), actualIt.CellID())
		}

		eCell := expectIt.IndexCell()
		aCell := actualIt.IndexCell()

		if len(eCell.shapes) != len(aCell.shapes) {
			t.Errorf("Cell %v shape count mismatch: %d vs %d", expectIt.CellID(), len(eCell.shapes), len(aCell.shapes))
		} else {
			for i := 0; i < len(eCell.shapes); i++ {
				es := eCell.shapes[i]
				as := aCell.shapes[i]
				if es.shapeID != as.shapeID {
					t.Errorf("Clipped shape ID mismatch: %d vs %d", es.shapeID, as.shapeID)
				}
				if len(es.edges) != len(as.edges) {
					t.Errorf("Clipped shape edge count mismatch: %d vs %d", len(es.edges), len(as.edges))
				}
			}
		}

		expectIt.Next()
		actualIt.Next()
	}
	if expectIt.Done() != actualIt.Done() {
		t.Errorf("Iterator done mismatch: expected %v, actual %v", expectIt.Done(), actualIt.Done())
	}
}
