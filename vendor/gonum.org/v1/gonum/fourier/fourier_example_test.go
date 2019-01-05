// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fourier_test

import (
	"fmt"
	"math"
	"math/cmplx"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/fourier"
	"gonum.org/v1/gonum/mat"
)

func ExampleFFT_Coefficients() {
	// Period is a set of samples over a given period.
	period := []float64{1, 0, 2, 0, 4, 0, 2, 0}

	// Initialize an FFT and perform the analysis.
	fft := fourier.NewFFT(len(period))
	coeff := fft.Coefficients(nil, period)

	for i, c := range coeff {
		fmt.Printf("freq=%v cycles/period, magnitude=%v, phase=%.4g\n",
			fft.Freq(i), cmplx.Abs(c), cmplx.Phase(c))
	}

	// Output:
	//
	// freq=0 cycles/period, magnitude=9, phase=0
	// freq=0.125 cycles/period, magnitude=3, phase=3.142
	// freq=0.25 cycles/period, magnitude=1, phase=-0
	// freq=0.375 cycles/period, magnitude=3, phase=3.142
	// freq=0.5 cycles/period, magnitude=9, phase=0
}

func ExampleFFT_Coefficients_tone() {
	// Tone is a set of samples over a second of a pure Middle C.
	const (
		mC      = 261.625565 // Hz
		samples = 44100
	)
	tone := make([]float64, samples)
	for i := range tone {
		tone[i] = math.Sin(mC * 2 * math.Pi * float64(i) / samples)
	}

	// Initialize an FFT and perform the analysis.
	fft := fourier.NewFFT(samples)
	coeff := fft.Coefficients(nil, tone)

	var maxFreq, magnitude, mean float64
	for i, c := range coeff {
		m := cmplx.Abs(c)
		mean += m
		if m > magnitude {
			magnitude = m
			maxFreq = fft.Freq(i) * samples
		}
	}
	fmt.Printf("freq=%v Hz, magnitude=%v, mean=%v\n", maxFreq, magnitude, mean/samples)

	// Output:
	//
	// freq=262 Hz, magnitude=17296.195519181776, mean=2.783457755654771
}

func ExampleCmplxFFT_Coefficients() {
	// Period is a set of samples over a given period.
	period := []complex128{1, 0, 2, 0, 4, 0, 2, 0}

	// Initialize a complex FFT and perform the analysis.
	fft := fourier.NewCmplxFFT(len(period))
	coeff := fft.Coefficients(nil, period)

	for i := range coeff {
		// Center the spectrum.
		i = fft.ShiftIdx(i)

		fmt.Printf("freq=%v cycles/period, magnitude=%v, phase=%.4g\n",
			fft.Freq(i), cmplx.Abs(coeff[i]), cmplx.Phase(coeff[i]))
	}

	// Output:
	//
	// freq=-0.5 cycles/period, magnitude=9, phase=0
	// freq=-0.375 cycles/period, magnitude=3, phase=3.142
	// freq=-0.25 cycles/period, magnitude=1, phase=0
	// freq=-0.125 cycles/period, magnitude=3, phase=3.142
	// freq=0 cycles/period, magnitude=9, phase=0
	// freq=0.125 cycles/period, magnitude=3, phase=3.142
	// freq=0.25 cycles/period, magnitude=1, phase=0
	// freq=0.375 cycles/period, magnitude=3, phase=3.142
}

func Example_fFT2() {
	// This example shows how to perform a 2D fourier transform
	// on an image. The transform identifies the lines present
	// in the image.

	// Image is a set of diagonal lines.
	image := mat.NewDense(11, 11, []float64{
		0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0,
		0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1,
		1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0,
		0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0,
		0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1,
		1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0,
		0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0,
		0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1,
		1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0,
		0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0,
		0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1,
	})

	// Make appropriately sized real and complex FFT types.
	r, c := image.Dims()
	fft := fourier.NewFFT(c)
	cfft := fourier.NewCmplxFFT(r)

	// Only c/2+1 coefficients will be returned for
	// the real FFT.
	c = c/2 + 1

	// Perform the first axis transform.
	rows := make([]complex128, r*c)
	for i := 0; i < r; i++ {
		fft.Coefficients(rows[c*i:c*(i+1)], image.RawRowView(i))
	}

	// Perform the second axis transform, storing
	// the result in freqs.
	freqs := mat.NewDense(c, c, nil)
	column := make([]complex128, r)
	for j := 0; j < c; j++ {
		for i := 0; i < r; i++ {
			column[i] = rows[i*c+j]
		}
		cfft.Coefficients(column, column)
		for i, v := range column[:c] {
			freqs.Set(i, j, floats.Round(cmplx.Abs(v), 1))
		}
	}

	fmt.Printf("%v\n", mat.Formatted(freqs))

	// Output:
	//
	// ⎡  40   0.4   0.5   1.4   3.2   1.1⎤
	// ⎢ 0.4   0.5   0.7   1.8     4   1.2⎥
	// ⎢ 0.5   0.7   1.1   2.8   5.9   1.7⎥
	// ⎢ 1.4   1.8   2.8   6.8  14.1   3.8⎥
	// ⎢ 3.2     4   5.9  14.1  27.5   6.8⎥
	// ⎣ 1.1   1.2   1.7   3.8   6.8   1.6⎦

}

func Example_cmplxFFT2() {
	// Image is a set of diagonal lines.
	image := mat.NewDense(11, 11, []float64{
		0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0,
		0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1,
		1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0,
		0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0,
		0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1,
		1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0,
		0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0,
		0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1,
		1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0,
		0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0,
		0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1,
	})

	// Make appropriately sized complex FFT.
	// Rows and columns are the same, so the same
	// CmplxFFT can be used for both axes.
	r, c := image.Dims()
	cfft := fourier.NewCmplxFFT(r)

	// Perform the first axis transform.
	rows := make([]complex128, r*c)
	for i := 0; i < r; i++ {
		row := rows[c*i : c*(i+1)]
		for j, v := range image.RawRowView(i) {
			row[j] = complex(v, 0)
		}
		cfft.Coefficients(row, row)
	}

	// Perform the second axis transform, storing
	// the result in freqs.
	freqs := mat.NewDense(c, c, nil)
	column := make([]complex128, r)
	for j := 0; j < c; j++ {
		for i := 0; i < r; i++ {
			column[i] = rows[i*c+j]
		}
		cfft.Coefficients(column, column)
		for i, v := range column {
			// Center the frequencies.
			freqs.Set(cfft.UnshiftIdx(i), cfft.UnshiftIdx(j), floats.Round(cmplx.Abs(v), 1))
		}
	}

	fmt.Printf("%v\n", mat.Formatted(freqs))

	// Output:
	//
	// ⎡ 1.6   6.8   3.8   1.7   1.2   1.1   1.1   1.4   2.6   3.9   1.1⎤
	// ⎢ 6.8  27.5  14.1   5.9     4   3.2     3     3   3.9   3.2   3.9⎥
	// ⎢ 3.8  14.1   6.8   2.8   1.8   1.4   1.2   1.1   1.4   3.9   2.6⎥
	// ⎢ 1.7   5.9   2.8   1.1   0.7   0.5   0.5   0.5   1.1     3   1.4⎥
	// ⎢ 1.2     4   1.8   0.7   0.5   0.4   0.4   0.5   1.2     3   1.1⎥
	// ⎢ 1.1   3.2   1.4   0.5   0.4    40   0.4   0.5   1.4   3.2   1.1⎥
	// ⎢ 1.1     3   1.2   0.5   0.4   0.4   0.5   0.7   1.8     4   1.2⎥
	// ⎢ 1.4     3   1.1   0.5   0.5   0.5   0.7   1.1   2.8   5.9   1.7⎥
	// ⎢ 2.6   3.9   1.4   1.1   1.2   1.4   1.8   2.8   6.8  14.1   3.8⎥
	// ⎢ 3.9   3.2   3.9     3     3   3.2     4   5.9  14.1  27.5   6.8⎥
	// ⎣ 1.1   3.9   2.6   1.4   1.1   1.1   1.2   1.7   3.8   6.8   1.6⎦

}
