// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package spatial

import (
	"math"

	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
)

// TODO(kortschak): Implement weighted routines.

// GetisOrdGStar returns the Local Getis-Ord G*i statistic for element of of the
// weighted data using the provided locality matrix. The returned value is a z-score.
//
//  G^*_i = num_i / den_i
//
//  num_i = \sum_j (w_{ij} x_j) - \bar X \sum_j w_{ij}
//  den_i = S \sqrt(((n \sum_j w_{ij}^2 - (\sum_j w_{ij})^2))/(n - 1))
//  \bar X = (\sum_j x_j) / n
//  S = \sqrt((\sum_j x_j^2)/n - (\bar X)^2)
//
// GetisOrdGStar will panic if locality is not a square matrix with dimensions the
// same as the length of data or if i is not a valid index into data.
//
// See doi.org/10.1111%2Fj.1538-4632.1995.tb00912.x.
//
// Weighted Getis-Ord G*i is not currently implemented and GetisOrdGStar will
// panic if weights is not nil.
func GetisOrdGStar(i int, data, weights []float64, locality mat.Matrix) float64 {
	if weights != nil {
		panic("spatial: weighted data not yet implemented")
	}
	r, c := locality.Dims()
	if r != len(data) || c != len(data) {
		panic("spatial: data length mismatch")
	}

	n := float64(len(data))
	mean, std := stat.MeanStdDev(data, weights)
	var dwd, dww, sw float64
	if doer, ok := locality.(mat.RowNonZeroDoer); ok {
		doer.DoRowNonZero(i, func(_, j int, w float64) {
			sw += w
			dwd += w * data[j]
			dww += w * w
		})
	} else {
		for j, v := range data {
			w := locality.At(i, j)
			sw += w
			dwd += w * v
			dww += w * w
		}
	}
	s := std * math.Sqrt((n-1)/n)

	return (dwd - mean*sw) / (s * math.Sqrt((n*dww-sw*sw)/(n-1)))
}

// GlobalMoransI performs Global Moran's I calculation of spatial autocorrelation
// for the given data using the provided locality matrix. GlobalMoransI returns
// Moran's I, Var(I) and the z-score associated with those values.
// GlobalMoransI will panic if locality is not a square matrix with dimensions the
// same as the length of data.
//
// See https://doi.org/10.1111%2Fj.1538-4632.2007.00708.x.
//
// Weighted Global Moran's I is not currently implemented and GlobalMoransI will
// panic if weights is not nil.
func GlobalMoransI(data, weights []float64, locality mat.Matrix) (i, v, z float64) {
	if weights != nil {
		panic("spatial: weighted data not yet implemented")
	}
	if r, c := locality.Dims(); r != len(data) || c != len(data) {
		panic("spatial: data length mismatch")
	}
	mean := stat.Mean(data, nil)

	doer, isDoer := locality.(mat.RowNonZeroDoer)

	// Calculate Moran's I for the data.
	var num, den, sum float64
	for i, xi := range data {
		zi := xi - mean
		den += zi * zi
		if isDoer {
			doer.DoRowNonZero(i, func(_, j int, w float64) {
				sum += w
				zj := data[j] - mean
				num += w * zi * zj
			})
		} else {
			for j, xj := range data {
				w := locality.At(i, j)
				sum += w
				zj := xj - mean
				num += w * zi * zj
			}
		}
	}
	i = (float64(len(data)) / sum) * (num / den)

	// Calculate Moran's E(I) for the data.
	e := -1 / float64(len(data)-1)

	// Calculate Moran's Var(I) for the data.
	//  http://pro.arcgis.com/en/pro-app/tool-reference/spatial-statistics/h-how-spatial-autocorrelation-moran-s-i-spatial-st.htm
	//  http://pro.arcgis.com/en/pro-app/tool-reference/spatial-statistics/h-global-morans-i-additional-math.htm
	var s0, s1, s2 float64
	var var2, var4 float64
	for i, v := range data {
		v -= mean
		v *= v
		var2 += v
		var4 += v * v

		var p2 float64
		if isDoer {
			doer.DoRowNonZero(i, func(i, j int, wij float64) {
				wji := locality.At(j, i)

				s0 += wij

				v := wij + wji
				s1 += v * v

				p2 += v
			})
		} else {
			for j := range data {
				wij := locality.At(i, j)
				wji := locality.At(j, i)

				s0 += wij

				v := wij + wji
				s1 += v * v

				p2 += v
			}
		}
		s2 += p2 * p2
	}
	s1 *= 0.5

	n := float64(len(data))
	a := n * ((n*n-3*n+3)*s1 - n*s2 + 3*s0*s0)
	c := (n - 1) * (n - 2) * (n - 3) * s0 * s0
	d := var4 / (var2 * var2)
	b := d * ((n*n-n)*s1 - 2*n*s2 + 6*s0*s0)

	v = (a-b)/c - e*e

	// Calculate z-score associated with Moran's I for the data.
	z = (i - e) / math.Sqrt(v)

	return i, v, z
}
