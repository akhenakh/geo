package s2

//go:generate hwygen -input $GOFILE -output . -targets avx2,fallback

import (
	"github.com/ajroetker/go-highway/hwy"
)

// BaseZigZagEncodeBatch applies ZigZag encoding to a slice of int32s.
// (n << 1) ^ (n >> 31)
func BaseZigZagEncodeBatch[T hwy.SignedInts, U hwy.UnsignedInts](src []T, dst []U) {
	size := min(len(src), len(dst))
	
	// Create vectors for constants/shifts if needed, though shifts are usually immediates
	
	hwy.ProcessWithTail[T](size,
		func(offset int) {
			v := hwy.Load(src[offset:])
			
			// (n << 1)
			left := hwy.ShiftLeft(v, 1)
			
			// (n >> 31) - arithmetic shift preserves sign bit
			right := hwy.ShiftRight(v, 31)
			
			// XOR
			// We need to cast the result to unsigned. In hwy, logical ops generally 
			// work on bits, so we can cast the memory interpretation store time 
			// or use bitwise ops.
			// Ideally, we treat them as same-sized bits.
			res := hwy.Xor(left, right)
			
			// Store as unsigned
			// Note: This requires unsafe casting in real implementation or 
			// hwy support for reinterpreting casts, but conceptually:
			hwy.Store(res, *(*[]T)(unsafe.Pointer(&dst[offset:]))) 
		},
		func(offset, count int) {
			// Scalar tail
			for i := 0; i < count; i++ {
				n := src[offset+i]
				dst[offset+i] = U((n << 1) ^ (n >> 31))
			}
		},
	)
}

// BaseZigZagDecodeBatch applies ZigZag decoding to a slice of uint32s.
// (n >> 1) ^ -(n & 1)
func BaseZigZagDecodeBatch[T hwy.SignedInts, U hwy.UnsignedInts](src []U, dst []T) {
	size := min(len(src), len(dst))
	
	vOne := hwy.Set(U(1))
	
	hwy.ProcessWithTail[U](size,
		func(offset int) {
			v := hwy.Load(src[offset:])
			
			// n >> 1
			right := hwy.ShiftRight(v, 1)
			
			// -(n & 1)
			mask := hwy.And(v, vOne)
			// Negating an unsigned requires casting to signed first in standard Go,
			// In SIMD, we calculate (mask * -1) or (0 - mask)
			// Assuming reinterpretation to signed T:
			negMask := hwy.Neg(mask) 
			
			res := hwy.Xor(right, negMask)
			hwy.Store(res, dst[offset:])
		},
		func(offset, count int) {
			for i := 0; i < count; i++ {
				n := src[offset+i]
				dst[offset+i] = T((n >> 1) ^ U(-T(n&1)))
			}
		},
	)
}
