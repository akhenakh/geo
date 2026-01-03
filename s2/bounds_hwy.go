package s2

//go:generate hwygen -input $GOFILE -output . -targets avx2,fallback

import (
	"github.com/ajroetker/go-highway/hwy"
)

// BaseBatchMinMax computes the minimum and maximum values in a slice.
// Used for computing bounding boxes of raw coordinates.
func BaseBatchMinMax[T hwy.Floats](data []T) (minVal, maxVal T) {
	if len(data) == 0 {
		return 0, 0
	}

	// Initialize with first value broadcasted
	// (Avoids issues with initializing to MaxFloat if data contains Infs/NaNs differently)
	initial := data[0]
	vMin := hwy.Set(initial)
	vMax := hwy.Set(initial)

	hwy.ProcessWithTail[T](len(data),
		func(offset int) {
			v := hwy.Load(data[offset:])
			vMin = hwy.Min(vMin, v)
			vMax = hwy.Max(vMax, v)
		},
		func(offset, count int) {
			mask := hwy.TailMask[T](count)
			v := hwy.MaskLoad(mask, data[offset:])

			// For min/max reduction with mask, we need to be careful not to
			// mix in the zero-padding from MaskLoad.
			// Ideally, load the current min/max into the "false" lanes of the mask
			// so they don't affect the result.

			vMinSafe := hwy.IfThenElse(mask, v, vMin)
			vMaxSafe := hwy.IfThenElse(mask, v, vMax)

			vMin = hwy.Min(vMin, vMinSafe)
			vMax = hwy.Max(vMax, vMaxSafe)
		},
	)

	return hwy.ReduceMin(vMin), hwy.ReduceMax(vMax)
}

// Helper usage in your code:
// latMin, latMax := BaseBatchMinMax(lats)
// lngMin, lngMax := BaseBatchMinMax(lngs)
