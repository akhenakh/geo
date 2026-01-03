package s2

//go:generate hwygen -input $GOFILE -output . -targets avx2,fallback

import (
	"github.com/ajroetker/go-highway/hwy"
)

// Batch Cross Product (Structure of Arrays)
// Computing normals for a list of edges (e.g., in a mesh or a complex polygon) involves cross products. Doing this in a batch using Structure of Arrays (SoA) layout is significantly faster than the standard slice-of-structs approach.

// BaseBatchCrossProduct computes the cross product of two sets of vectors (SoA layout).
// cx = ay*bz - az*by
// cy = az*bx - ax*bz
// cz = ax*by - ay*bx
func BaseBatchCrossProduct[T hwy.Floats](
	ax, ay, az []T,
	bx, by, bz []T,
	cx, cy, cz []T,
) {
	size := min(len(ax), len(ay), len(az), len(bx), len(by), len(bz))

	hwy.ProcessWithTail[T](size,
		func(offset int) {
			// Load A
			vAx := hwy.Load(ax[offset:])
			vAy := hwy.Load(ay[offset:])
			vAz := hwy.Load(az[offset:])

			// Load B
			vBx := hwy.Load(bx[offset:])
			vBy := hwy.Load(by[offset:])
			vBz := hwy.Load(bz[offset:])

			// Compute C X
			vCx := hwy.Sub(hwy.Mul(vAy, vBz), hwy.Mul(vAz, vBy))

			// Compute C Y
			vCy := hwy.Sub(hwy.Mul(vAz, vBx), hwy.Mul(vAx, vBz))

			// Compute C Z
			vCz := hwy.Sub(hwy.Mul(vAx, vBy), hwy.Mul(vAy, vBx))

			// Store
			hwy.Store(vCx, cx[offset:])
			hwy.Store(vCy, cy[offset:])
			hwy.Store(vCz, cz[offset:])
		},
		func(offset, count int) {
			mask := hwy.TailMask[T](count)

			vAx := hwy.MaskLoad(mask, ax[offset:])
			vAy := hwy.MaskLoad(mask, ay[offset:])
			vAz := hwy.MaskLoad(mask, az[offset:])
			vBx := hwy.MaskLoad(mask, bx[offset:])
			vBy := hwy.MaskLoad(mask, by[offset:])
			vBz := hwy.MaskLoad(mask, bz[offset:])

			vCx := hwy.Sub(hwy.Mul(vAy, vBz), hwy.Mul(vAz, vBy))
			vCy := hwy.Sub(hwy.Mul(vAz, vBx), hwy.Mul(vAx, vBz))
			vCz := hwy.Sub(hwy.Mul(vAx, vBy), hwy.Mul(vAy, vBx))

			hwy.MaskStore(mask, vCx, cx[offset:])
			hwy.MaskStore(mask, vCy, cy[offset:])
			hwy.MaskStore(mask, vCz, cz[offset:])
		},
	)
}
