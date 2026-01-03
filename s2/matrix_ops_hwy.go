package s2

//go:generate hwygen -input $GOFILE -output . -targets avx2,fallback

import (
	"github.com/ajroetker/go-highway/hwy"
)

// BaseMatrixMulBatch applies a 3x3 matrix to a set of 3D vectors (SoA).
// DST = M * SRC
func BaseMatrixMulBatch[T hwy.Floats](
	m00, m01, m02 T,
	m10, m11, m12 T,
	m20, m21, m22 T,
	srcX, srcY, srcZ []T,
	dstX, dstY, dstZ []T,
) {
	size := min(len(srcX), len(srcY), len(srcZ), len(dstX), len(dstY), len(dstZ))

	// Broadcast matrix columns
	// Row 0
	vM00 := hwy.Set(m00)
	vM01 := hwy.Set(m01)
	vM02 := hwy.Set(m02)
	// Row 1
	vM10 := hwy.Set(m10)
	vM11 := hwy.Set(m11)
	vM12 := hwy.Set(m12)
	// Row 2
	vM20 := hwy.Set(m20)
	vM21 := hwy.Set(m21)
	vM22 := hwy.Set(m22)

	hwy.ProcessWithTail[T](size,
		func(offset int) {
			x := hwy.Load(srcX[offset:])
			y := hwy.Load(srcY[offset:])
			z := hwy.Load(srcZ[offset:])

			// Row 0: x*m00 + y*m01 + z*m02
			resX := hwy.Mul(x, vM00)
			resX = hwy.FMA(y, vM01, resX)
			resX = hwy.FMA(z, vM02, resX)

			// Row 1: x*m10 + y*m11 + z*m12
			resY := hwy.Mul(x, vM10)
			resY = hwy.FMA(y, vM11, resY)
			resY = hwy.FMA(z, vM12, resY)

			// Row 2: x*m20 + y*m21 + z*m22
			resZ := hwy.Mul(x, vM20)
			resZ = hwy.FMA(y, vM21, resZ)
			resZ = hwy.FMA(z, vM22, resZ)

			hwy.Store(resX, dstX[offset:])
			hwy.Store(resY, dstY[offset:])
			hwy.Store(resZ, dstZ[offset:])
		},
		func(offset, count int) {
			mask := hwy.TailMask[T](count)
			x := hwy.MaskLoad(mask, srcX[offset:])
			y := hwy.MaskLoad(mask, srcY[offset:])
			z := hwy.MaskLoad(mask, srcZ[offset:])

			resX := hwy.Mul(x, vM00)
			resX = hwy.FMA(y, vM01, resX)
			resX = hwy.FMA(z, vM02, resX)

			resY := hwy.Mul(x, vM10)
			resY = hwy.FMA(y, vM11, resY)
			resY = hwy.FMA(z, vM12, resY)

			resZ := hwy.Mul(x, vM20)
			resZ = hwy.FMA(y, vM21, resZ)
			resZ = hwy.FMA(z, vM22, resZ)

			hwy.MaskStore(mask, resX, dstX[offset:])
			hwy.MaskStore(mask, resY, dstY[offset:])
			hwy.MaskStore(mask, resZ, dstZ[offset:])
		},
	)
}
