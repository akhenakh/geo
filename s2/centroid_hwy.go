package s2

//go:generate hwygen -input $GOFILE -output . -targets avx2,fallback

import (
	"github.com/ajroetker/go-highway/hwy"
)

// BaseSumPoints computes the vector sum of a list of coordinates.
// Input is de-interleaved (separate slices for X, Y, Z coordinates).
// Returns the sum X, Y, Z.
func BaseSumPoints[T hwy.Floats](xs, ys, zs []T) (sumX, sumY, sumZ T) {
	size := min(len(xs), len(ys), len(zs))

	// Accumulators
	vSumX := hwy.Zero[T]()
	vSumY := hwy.Zero[T]()
	vSumZ := hwy.Zero[T]()

	hwy.ProcessWithTail[T](size,
		func(offset int) {
			vx := hwy.Load(xs[offset:])
			vy := hwy.Load(ys[offset:])
			vz := hwy.Load(zs[offset:])

			vSumX = hwy.Add(vSumX, vx)
			vSumY = hwy.Add(vSumY, vy)
			vSumZ = hwy.Add(vSumZ, vz)
		},
		func(offset, count int) {
			mask := hwy.TailMask[T](count)
			vx := hwy.MaskLoad(mask, xs[offset:])
			vy := hwy.MaskLoad(mask, ys[offset:])
			vz := hwy.MaskLoad(mask, zs[offset:])

			vSumX = hwy.Add(vSumX, vx)
			vSumY = hwy.Add(vSumY, vy)
			vSumZ = hwy.Add(vSumZ, vz)
		},
	)

	return hwy.ReduceSum(vSumX), hwy.ReduceSum(vSumY), hwy.ReduceSum(vSumZ)
}
