package s2

//go:generate hwygen -input $GOFILE -output . -targets avx2,fallback

//Converting S2 CellIDs to 3D points (decoding the Hilbert curve) requires the quadratic transform stToUV. Since this happens for every cell in a ShapeIndex or RegionCoverer, vectorizing the math transform yields significant gains.
import (
	"github.com/ajroetker/go-highway/hwy"
)

// BaseSTtoUVBatch converts ST coordinates (0..1) to UV coordinates (-1..1).
// u = (1/3) * (4s² - 1)       if s >= 0.5
// u = (1/3) * (1 - 4(1-s)²)   if s < 0.5
func BaseSTtoUVBatch[T hwy.Floats](s, u []T) {
	size := min(len(s), len(u))

	vHalf := hwy.Set(T(0.5))
	vOne := hwy.Set(T(1.0))
	vFour := hwy.Set(T(4.0))
	vThird := hwy.Set(T(1.0 / 3.0))

	hwy.ProcessWithTail[T](size,
		func(offset int) {
			val := hwy.Load(s[offset:])

			// Mask for branching: s >= 0.5
			geHalf := hwy.GreaterEqual(val, vHalf)

			// Case 1: s >= 0.5 -> 4*s*s - 1
			// Case 2: s < 0.5  -> 1 - 4*(1-s)*(1-s)

			// Normalize inputs for the quadratic formula to look similar
			// If s < 0.5, we compute based on (1-s)
			// t = (s >= 0.5) ? s : (1-s)
			t := hwy.IfThenElse(geHalf, val, hwy.Sub(vOne, val))

			// term = 4 * t * t - 1
			term := hwy.FMA(vFour, hwy.Mul(t, t), hwy.Neg(vOne))

			// Apply 1/3 scale
			res := hwy.Mul(vThird, term)

			// Fix sign for the s < 0.5 case (result should be negated relative to the formula logic)
			// Actually formula 2 is 1 - 4(1-s)^2 = -(4(1-s)^2 - 1).
			// So we just negate the result if s < 0.5.
			res = hwy.IfThenElse(geHalf, res, hwy.Neg(res))

			hwy.Store(res, u[offset:])
		},
		func(offset, count int) {
			// Scalar fallback logic
			mask := hwy.TailMask[T](count)
			val := hwy.MaskLoad(mask, s[offset:])

			geHalf := hwy.GreaterEqual(val, vHalf)
			t := hwy.IfThenElse(geHalf, val, hwy.Sub(vOne, val))
			term := hwy.FMA(vFour, hwy.Mul(t, t), hwy.Neg(vOne))
			res := hwy.Mul(vThird, term)
			res = hwy.IfThenElse(geHalf, res, hwy.Neg(res))

			hwy.MaskStore(mask, res, u[offset:])
		},
	)
}
