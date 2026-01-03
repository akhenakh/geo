package s2

//go:generate hwygen -input $GOFILE -output . -targets avx2,fallback

import (
	"github.com/ajroetker/go-highway/hwy"
)

// Batch Dot Product (Edge Triage & Nearest Neighbor)
// Many S2 queries boil down to "find which of these points is closest to X" or "which of these points lie on the positive side of plane X". Both reduce to computing the dot product of a constant vector against a stream of vectors.
// This optimizes:
//    ClosestPointQuery (finding max dot product = min distance)
//    CrossingEdgeQuery (checking edge signs against a normal vector)

// BaseDotProductConstBatch computes dot products of a constant vector A against
// a set of vectors B (stored in SoA layout).
// dst[i] = A.X * bx[i] + A.Y * by[i] + A.Z * bz[i]
func BaseDotProductConstBatch[T hwy.Floats](
	ax, ay, az T,
	bx, by, bz []T,
	dst []T,
) {
	size := min(len(bx), len(by), len(bz), len(dst))

	vAx := hwy.Set(ax)
	vAy := hwy.Set(ay)
	vAz := hwy.Set(az)

	hwy.ProcessWithTail[T](size,
		func(offset int) {
			vBx := hwy.Load(bx[offset:])
			vBy := hwy.Load(by[offset:])
			vBz := hwy.Load(bz[offset:])

			// FMA: (ax * bx) + (ay * by) + (az * bz)
			// Using FMA is faster and more precise
			sum := hwy.Mul(vAx, vBx)
			sum = hwy.FMA(vAy, vBy, sum)
			sum = hwy.FMA(vAz, vBz, sum)

			hwy.Store(sum, dst[offset:])
		},
		func(offset, count int) {
			mask := hwy.TailMask[T](count)
			vBx := hwy.MaskLoad(mask, bx[offset:])
			vBy := hwy.MaskLoad(mask, by[offset:])
			vBz := hwy.MaskLoad(mask, bz[offset:])

			sum := hwy.Mul(vAx, vBx)
			sum = hwy.FMA(vAy, vBy, sum)
			sum = hwy.FMA(vAz, vBz, sum)

			hwy.MaskStore(mask, sum, dst[offset:])
		},
	)
}
