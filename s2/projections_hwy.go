package s2

//go:generate hwygen -input $GOFILE -output . -targets avx2,fallback

import (
	"github.com/ajroetker/go-highway/hwy"
	"github.com/ajroetker/go-highway/hwy/contrib/math"
)

// BasePlateCarreeProject computes the Plate Carree projection for a batch of points.
// X = lng * fromRadians
// Y = lat * fromRadians
func BasePlateCarreeProject[T hwy.Floats](lats, lngs, xs, ys []T, fromRadians T) {
	size := min(len(lats), len(lngs))
	vFromRad := hwy.Set(fromRadians)

	for ii := 0; ii < size; ii += vFromRad.NumLanes() {
		// Load
		lat := hwy.Load(lats[ii:])
		lng := hwy.Load(lngs[ii:])

		// Multiply
		y := hwy.Mul(lat, vFromRad)
		x := hwy.Mul(lng, vFromRad)

		// Store
		hwy.Store(y, ys[ii:])
		hwy.Store(x, xs[ii:])
	}
}

// BaseMercatorProject computes the Mercator projection for a batch of points.
// X = lng * fromRadians
// Y = 0.5 * log((1+sin(lat))/(1-sin(lat))) * fromRadians
func BaseMercatorProject[T hwy.Floats](lats, lngs, xs, ys []T, fromRadians T) {
	size := min(len(lats), len(lngs))

	vFromRad := hwy.Set(fromRadians)
	vHalf := hwy.Set(T(0.5))
	vOne := hwy.Set(T(1.0))
	vYScale := hwy.Mul(vHalf, vFromRad) // 0.5 * fromRadians

	for ii := 0; ii < size; ii += vFromRad.NumLanes() {
		// Process Longitude (X)
		lng := hwy.Load(lngs[ii:])
		x := hwy.Mul(lng, vFromRad)
		hwy.Store(x, xs[ii:])

		// Process Latitude (Y)
		lat := hwy.Load(lats[ii:])
		s := math.Sin(lat)

		// (1 + s) / (1 - s)
		num := hwy.Add(vOne, s)
		den := hwy.Sub(vOne, s)
		val := hwy.Div(num, den)

		// log(val)
		l := math.Log(val)

		// y = l * 0.5 * fromRadians
		y := hwy.Mul(l, vYScale)
		hwy.Store(y, ys[ii:])
	}
}
