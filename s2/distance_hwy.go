package s2

//go:generate hwygen -input $GOFILE -output . -targets avx2,fallback

import (
	"math"

	"github.com/ajroetker/go-highway/hwy"
)

// BaseMinDistanceToPoint finds the minimum squared Euclidean distance from a target point
// to a set of points (SoA layout).
// Used for ChordAngle comparisons where distance = |u-v|^2.
func BaseMinDistanceToPoint[T hwy.Floats](
	targetX, targetY, targetZ T,
	xs, ys, zs []T,
) T {
	size := min(len(xs), len(ys), len(zs))

	vTx := hwy.Set(targetX)
	vTy := hwy.Set(targetY)
	vTz := hwy.Set(targetZ)

	// Initialize min distance to max value
	vMinDist := hwy.Set(T(math.MaxFloat32))
	if size > 0 && float64(targetX) > 1 { // Check for float64 based on value range or type
		vMinDist = hwy.Set(T(math.MaxFloat64))
	}

	hwy.ProcessWithTail[T](size,
		func(offset int) {
			vx := hwy.Load(xs[offset:])
			vy := hwy.Load(ys[offset:])
			vz := hwy.Load(zs[offset:])

			// dx = x - tx
			dx := hwy.Sub(vx, vTx)
			dy := hwy.Sub(vy, vTy)
			dz := hwy.Sub(vz, vTz)

			// dist = dx*dx + dy*dy + dz*dz
			distSq := hwy.Add(
				hwy.Mul(dx, dx),
				hwy.Add(hwy.Mul(dy, dy), hwy.Mul(dz, dz)),
			)

			vMinDist = hwy.Min(vMinDist, distSq)
		},
		func(offset, count int) {
			mask := hwy.TailMask[T](count)
			vx := hwy.MaskLoad(mask, xs[offset:])
			vy := hwy.MaskLoad(mask, ys[offset:])
			vz := hwy.MaskLoad(mask, zs[offset:])

			dx := hwy.Sub(vx, vTx)
			dy := hwy.Sub(vy, vTy)
			dz := hwy.Sub(vz, vTz)

			distSq := hwy.Add(
				hwy.Mul(dx, dx),
				hwy.Add(hwy.Mul(dy, dy), hwy.Mul(dz, dz)),
			)

			// We must mask the result before min, otherwise 0s (from mask) might become min
			// Set masked-out elements to MaxFloat
			maxVal := hwy.Set(T(math.MaxFloat32)) // Approximation for generic logic
			distSq = hwy.IfThenElse(mask, distSq, maxVal)

			vMinDist = hwy.Min(vMinDist, distSq)
		},
	)

	return hwy.ReduceMin(vMinDist)
}
