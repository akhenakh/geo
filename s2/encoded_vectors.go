package s2

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/bits"
	"sort"
)

// EncodedUintVector represents an encoded vector of unsigned integers.
// Values are encoded using a fixed number of bytes per value, depending on the largest value.
type EncodedUintVector[T interface{ uint64 | uint32 | uint16 }] struct {
	data []byte
	size uint64 // Number of elements
	len  int    // Bytes per element
}

// init initializes the vector from a decoder.
func (v *EncodedUintVector[T]) init(d *decoder) error {
	sizeLen := d.readUvarint()
	if d.err != nil {
		return d.err
	}
	var t T
	typeSize := uint64(binary.Size(t))
	v.size = sizeLen / typeSize
	v.len = int((sizeLen & (typeSize - 1)) + 1)

	bytes := v.size * uint64(v.len)
	if bytes > uint64(1<<30) { // Safety check
		return fmt.Errorf("EncodedUintVector too large: %d bytes", bytes)
	}

	v.data = make([]byte, bytes)
	for i := range v.data {
		v.data[i] = d.readUint8()
	}
	return d.err
}

// encode writes the vector to an encoder.
func (v *EncodedUintVector[T]) encode(e *encoder) {
	var t T
	typeSize := uint64(binary.Size(t))
	sizeLen := (v.size * typeSize) | uint64(v.len-1)
	e.writeUvarint(sizeLen)
	for _, b := range v.data {
		e.writeUint8(b)
	}
}

// encodeUintVector encodes a slice of values.
func encodeUintVector[T interface{ uint64 | uint32 | uint16 }](values []T, e *encoder) {
	if len(values) == 0 {
		e.writeUvarint(uint64(0) | uint64(0)) // size=0, len=1 (encoded as 0)
		return
	}

	var maxVal uint64
	for _, x := range values {
		if val := uint64(x); val > maxVal {
			maxVal = val
		}
	}

	byteLen := 1
	if maxVal > 0 {
		byteLen = (bits.Len64(maxVal) + 7) / 8
	}

	var t T
	typeSize := uint64(binary.Size(t))

	// varint64: (size * sizeof(T)) | (len - 1)
	e.writeUvarint((uint64(len(values)) * typeSize) | uint64(byteLen-1))

	for _, x := range values {
		val := uint64(x)
		for i := 0; i < byteLen; i++ {
			e.writeUint8(uint8(val))
			val >>= 8
		}
	}
}

// Get returns the value at index i.
func (v *EncodedUintVector[T]) Get(i int) T {
	if uint64(i) >= v.size {
		return 0
	}
	offset := i * v.len
	val := uint64(0)
	for b := 0; b < v.len; b++ {
		val |= uint64(v.data[offset+b]) << (8 * b)
	}
	return T(val)
}

// LowerBound returns the index of the first element >= target.
func (v *EncodedUintVector[T]) LowerBound(target T) int {
	return sort.Search(int(v.size), func(i int) bool {
		return v.Get(i) >= target
	})
}

// Size returns the number of elements.
func (v *EncodedUintVector[T]) Size() int {
	return int(v.size)
}

// stringVectorEncoder accumulates strings to be encoded.
type stringVectorEncoder struct {
	offsets []uint64
	data    bytes.Buffer
}

// Add adds a byte slice (representing a string/data) to the encoder.
func (s *stringVectorEncoder) Add(b []byte) {
	s.data.Write(b)
	s.offsets = append(s.offsets, uint64(s.data.Len()))
}

// encode writes the encoded vector to the encoder.
func (s *stringVectorEncoder) encode(e *encoder) {
	encodeUintVector(s.offsets, e)
	e.writeBytes(s.data.Bytes())
}

// EncodedStringVector represents an encoded vector of strings.
type EncodedStringVector struct {
	offsets EncodedUintVector[uint64]
	data    []byte
}

// init initializes from a decoder.
func (v *EncodedStringVector) init(d *decoder) error {
	if err := v.offsets.init(d); err != nil {
		return err
	}
	if v.offsets.Size() == 0 {
		return nil
	}

	totalLen := v.offsets.Get(v.offsets.Size() - 1)
	v.data = make([]byte, totalLen)
	for i := range v.data {
		v.data[i] = d.readUint8()
	}
	return d.err
}

// encodeStringVector encodes a slice of strings.
func encodeStringVector(strs []string, e *encoder) {
	offsets := make([]uint64, len(strs))
	currentOffset := uint64(0)
	for i, s := range strs {
		currentOffset += uint64(len(s))
		offsets[i] = currentOffset
	}
	encodeUintVector(offsets, e)
	for _, s := range strs {
		for i := 0; i < len(s); i++ {
			e.writeUint8(s[i])
		}
	}
}

// Get returns the string at index i.
func (v *EncodedStringVector) Get(i int) string {
	start := uint64(0)
	if i > 0 {
		start = v.offsets.Get(i - 1)
	}
	end := v.offsets.Get(i)
	return string(v.data[start:end])
}

// Size returns the number of strings.
func (v *EncodedStringVector) Size() int {
	return v.offsets.Size()
}

// EncodedS2CellIdVector represents an encoded vector of S2CellIds.
type EncodedS2CellIdVector struct {
	deltas  EncodedUintVector[uint64]
	base    uint64
	shift   uint8
	baseLen uint8
}

// init initializes from a decoder.
func (v *EncodedS2CellIdVector) init(d *decoder) error {
	codePlusLen := d.readUint8()
	shiftCode := codePlusLen >> 3
	if shiftCode == 31 {
		shiftCode = 29 + d.readUint8()
		if shiftCode > 56 {
			return fmt.Errorf("invalid shift code: %d", shiftCode)
		}
	}
	v.baseLen = codePlusLen & 7

	v.base = 0
	for i := 0; i < int(v.baseLen); i++ {
		b := d.readUint8()
		v.base |= uint64(b) << (8 * i)
	}
	v.base <<= (64 - 8*max(1, int(v.baseLen)))

	if shiftCode >= 29 {
		v.shift = 2*(shiftCode-29) + 1
		v.base |= 1 << (v.shift - 1)
	} else {
		v.shift = 2 * shiftCode
	}

	return v.deltas.init(d)
}

// encodeS2CellIdVector encodes a slice of CellIDs.
func encodeS2CellIdVector(ids []CellID, e *encoder) {
	if len(ids) == 0 {
		e.writeUint8(0) // shift_code=0, base_len=0
		encodeUintVector([]uint64{}, e)
		return
	}

	var vOr, vMax uint64
	vMin := ^uint64(0)
	vAnd := ^uint64(0)

	for _, id := range ids {
		val := uint64(id)
		vOr |= val
		vAnd &= val
		if val < vMin {
			vMin = val
		}
		if val > vMax {
			vMax = val
		}
	}

	shift := 0
	if vOr > 0 {
		shift = min(56, bits.TrailingZeros64(vOr)&^1)
		if (vAnd & (1 << shift)) != 0 {
			shift++
		}
	}

	bestBytes := ^uint64(0)
	bestBase := uint64(0)
	bestBaseLen := 0
	bestMaxDeltaMsb := 0

	for length := 0; length < 8; length++ {
		mask := ^uint64(0) << (8 * (8 - length)) // Keep top 'length' bytes
		if length == 0 {
			mask = 0
		}

		tBase := vMin & mask
		tMaxDelta := (vMax - tBase) >> shift
		tMaxDeltaMsb := 0
		if tMaxDelta > 0 {
			tMaxDeltaMsb = bits.Len64(tMaxDelta) - 1
		}

		tBytes := uint64(length) + uint64(len(ids))*uint64((tMaxDeltaMsb>>3)+1)
		if tBytes < bestBytes {
			bestBytes = tBytes
			bestBase = tBase
			bestBaseLen = length
			bestMaxDeltaMsb = tMaxDeltaMsb
		}
	}

	if (shift&1) != 0 && (bestMaxDeltaMsb&7) != 7 {
		shift--
	}

	shiftCode := uint8(shift >> 1)
	if (shift & 1) != 0 {
		shiftCode = min(31, shiftCode+29)
	}
	e.writeUint8((shiftCode << 3) | uint8(bestBaseLen))
	if shiftCode == 31 {
		e.writeUint8(uint8(shift >> 1))
	}

	baseBytes := bestBase >> (64 - 8*max(1, bestBaseLen))
	for i := 0; i < bestBaseLen; i++ {
		e.writeUint8(uint8(baseBytes))
		baseBytes >>= 8
	}

	deltas := make([]uint64, len(ids))
	for i, id := range ids {
		deltas[i] = (uint64(id) - bestBase) >> shift
	}
	encodeUintVector(deltas, e)
}

// Get returns the CellID at index i.
func (v *EncodedS2CellIdVector) Get(i int) CellID {
	return CellID((v.deltas.Get(i) << v.shift) + v.base)
}

// Size returns the number of CellIDs.
func (v *EncodedS2CellIdVector) Size() int {
	return v.deltas.Size()
}

// LowerBound returns the index of the first element >= target.
func (v *EncodedS2CellIdVector) LowerBound(target CellID) int {
	if uint64(target) <= v.base {
		return 0
	}

	tVal := uint64(target)
	delta := (tVal - v.base + (1 << v.shift) - 1) >> v.shift

	return v.deltas.LowerBound(delta)
}
