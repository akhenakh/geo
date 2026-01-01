package s2

import (
	"bytes"
	"fmt"
	"io"
	"sync"
)

// EncodedShapeIndex is a read-only ShapeIndex that works directly with encoded data.
// It is efficient for loading large indexes where only a few cells will be accessed.
type EncodedShapeIndex struct {
	// options are the index options (maxEdgesPerCell)
	maxEdgesPerCell int

	// shapeFactory is used to retrieve shapes.
	shapeFactory ShapeFactory

	cellIDs      EncodedS2CellIdVector
	encodedCells EncodedStringVector

	// cache for decoded cells
	cellsMu sync.RWMutex
	cells   map[int]*ShapeIndexCell // map from index to cell
}

// ShapeFactory interface for retrieving shapes by ID.
type ShapeFactory interface {
	GetShape(id int) Shape
	Len() int
}

// BasicShapeFactory holds a simple slice of shapes.
type BasicShapeFactory struct {
	Shapes []Shape
}

func (f *BasicShapeFactory) GetShape(id int) Shape {
	if id < 0 || id >= len(f.Shapes) {
		return nil
	}
	return f.Shapes[id]
}
func (f *BasicShapeFactory) Len() int { return len(f.Shapes) }

// NewEncodedShapeIndex creates an empty encoded index.
func NewEncodedShapeIndex() *EncodedShapeIndex {
	return &EncodedShapeIndex{
		cells: make(map[int]*ShapeIndexCell),
	}
}

// Init initializes the index from an io.Reader.
// The reader should contain the data produced by ShapeIndex.Encode.
func (s *EncodedShapeIndex) Init(r io.Reader, factory ShapeFactory) error {
	d := &decoder{r: asByteReader(r)}

	// Read max edges and version
	val := d.readUvarint()
	if d.err != nil {
		return d.err
	}
	version := val & 3
	if version != 0 {
		return fmt.Errorf("unsupported shape index version: %d", version)
	}
	s.maxEdgesPerCell = int(val >> 2)
	s.shapeFactory = factory

	if err := s.cellIDs.init(d); err != nil {
		return err
	}
	if err := s.encodedCells.init(d); err != nil {
		return err
	}

	// Clear cache
	s.cells = make(map[int]*ShapeIndexCell)
	return nil
}

// Shape returns the shape with the given ID.
func (s *EncodedShapeIndex) Shape(id int32) Shape {
	if s.shapeFactory == nil {
		return nil
	}
	return s.shapeFactory.GetShape(int(id))
}

// Len returns number of shapes.
func (s *EncodedShapeIndex) Len() int {
	if s.shapeFactory == nil {
		return 0
	}
	return s.shapeFactory.Len()
}

// NumEdges returns total edges.
func (s *EncodedShapeIndex) NumEdges() int {
	n := 0
	count := s.Len()
	for i := 0; i < count; i++ {
		shp := s.Shape(int32(i))
		if shp != nil {
			n += shp.NumEdges()
		}
	}
	return n
}

// Iterator returns a new iterator for the encoded index.
func (s *EncodedShapeIndex) Iterator() *ShapeIndexIterator {
	return NewShapeIndexIterator(s, IteratorBegin)
}

// Implement ShapeIndexView interface

func (s *EncodedShapeIndex) cellLen() int {
	return s.cellIDs.Size()
}

func (s *EncodedShapeIndex) cellID(i int) CellID {
	return s.cellIDs.Get(i)
}

func (s *EncodedShapeIndex) indexCell(i int) *ShapeIndexCell {
	return s.GetCell(i)
}

func (s *EncodedShapeIndex) maybeApplyUpdates() {
	// Read-only
}

// GetCell returns the decoded cell at index i.
func (s *EncodedShapeIndex) GetCell(i int) *ShapeIndexCell {
	s.cellsMu.RLock()
	if cell, ok := s.cells[i]; ok {
		s.cellsMu.RUnlock()
		return cell
	}
	s.cellsMu.RUnlock()

	// Decode
	s.cellsMu.Lock()
	defer s.cellsMu.Unlock()
	// Double check
	if cell, ok := s.cells[i]; ok {
		return cell
	}

	raw := s.encodedCells.Get(i)
	cell := &ShapeIndexCell{}

	// Create a decoder for the cell data
	d := &decoder{r: bytes.NewReader([]byte(raw))}

	// We need numShapeIDs for decoding.
	if err := cell.Decode(s.Len(), d); err != nil {
		// Panic on data corruption for now as iterators don't return errors
		panic(fmt.Errorf("failed to decode cell %d: %w", i, err))
	}

	s.cells[i] = cell
	return cell
}
