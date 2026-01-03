package s2

//go:generate hwygen -input $GOFILE -output . -targets avx2,fallback

import (
	"github.com/ajroetker/go-highway/hwy"
	"github.com/ajroetker/go-highway/hwy/contrib/algo"
)

// BasePointsFromLatLngsBatch converts a slice of LatLngs to Points (XYZ) using SIMD.
// This de-interleaves the structs into separate arrays for vectorization.
func BasePointsFromLatLngsBatch(lats, lngs, xs, ys, zs []float64) {
	size := min(len(lats), len(lngs))

	// Pre-allocate temp buffers for intermediate trig results if not provided
	// In a real implementation, you might pass a workspace to avoid allocs
	sinLat := make([]float64, size)
	cosLat := make([]float64, size)
	sinLng := make([]float64, size)
	cosLng := make([]float64, size)

	// 1. Compute Trig in Batch (Zero allocs if buffers reused)
	algo.SinTransform64(lats, sinLat)
	algo.CosTransform64(lats, cosLat)
	algo.SinTransform64(lngs, sinLng)
	algo.CosTransform64(lngs, cosLng)

	// 2. Combine results:
	// X = cos(lat) * cos(lng)
	// Y = cos(lat) * sin(lng)
	// Z = sin(lat)

	hwy.ProcessWithTail[float64](size,
		func(offset int) {
			vCosLat := hwy.Load(cosLat[offset:])
			vCosLng := hwy.Load(cosLng[offset:])
			vSinLng := hwy.Load(sinLng[offset:])
			vSinLat := hwy.Load(sinLat[offset:])

			vX := hwy.Mul(vCosLat, vCosLng)
			vY := hwy.Mul(vCosLat, vSinLng)

			hwy.Store(vX, xs[offset:])
			hwy.Store(vY, ys[offset:])
			hwy.Store(vSinLat, zs[offset:])
		},
		func(offset, count int) {
			// Scalar fallback logic
		},
	)
}
