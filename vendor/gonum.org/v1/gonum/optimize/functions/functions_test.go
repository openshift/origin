// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package functions

import "testing"

func TestBeale(t *testing.T) {
	tests := []funcTest{
		{
			X:        []float64{1, 1},
			F:        14.203125,
			Gradient: []float64{0, 27.75},
		},
		{
			X:        []float64{1, 4},
			F:        4624.453125,
			Gradient: []float64{8813.25, 6585},
		},
	}
	testFunction(Beale{}, tests, t)
}

func TestBiggsEXP2(t *testing.T) {
	tests := []funcTest{
		{
			X:        []float64{1, 2},
			F:        32.26255055084012,
			Gradient: []float64{8.308203800550878, -25.32607145221645},
		},
	}
	testFunction(BiggsEXP2{}, tests, t)
}

func TestBiggsEXP3(t *testing.T) {
	tests := []funcTest{
		{
			X:        []float64{1, 2, 1},
			F:        1.598844540607779,
			Gradient: []float64{1.0633795027631927, -0.5196392672262664, -0.3180919155433357},
		},
	}
	testFunction(BiggsEXP3{}, tests, t)
}

func TestBiggsEXP4(t *testing.T) {
	tests := []funcTest{
		{
			X: []float64{1, 2, 1, 1},
			F: 1.598844540607779,
			Gradient: []float64{1.0633795027631927, -0.5196392672262664,
				-0.44245622408151464, -0.3180919155433357},
		},
	}
	testFunction(BiggsEXP4{}, tests, t)
}

func TestBiggsEXP5(t *testing.T) {
	tests := []funcTest{
		{
			X: []float64{1, 2, 1, 1, 1},
			F: 13.386420552801937,
			Gradient: []float64{-6.54665204477596, 3.5259856535515293,
				14.36984212995392, -9.522506150695783, -19.639956134327882},
		},
	}
	testFunction(BiggsEXP5{}, tests, t)
}

func TestBiggsEXP6(t *testing.T) {
	tests := []funcTest{
		{
			X: []float64{1, 2, 1, 1, 1, 1},
			F: 0.77907007565597,
			Gradient: []float64{-0.149371887533426, -0.183163468182936,
				-1.483958013575642, 1.428277503849742, -0.149371887533426,
				-1.483958013575642},
		},
	}
	testFunction(BiggsEXP6{}, tests, t)
}

func TestBox3D(t *testing.T) {
	tests := []funcTest{
		{
			X:        []float64{0, 10, 20},
			F:        1031.1538106093985,
			Gradient: []float64{98.22343149849218, -2.11937420675874, 112.38817362220350},
		},
	}
	testFunction(Box3D{}, tests, t)
}

func TestBrownBadlyScaled(t *testing.T) {
	tests := []funcTest{
		{
			X:        []float64{1, 1},
			F:        999998000003,
			Gradient: []float64{-2e+6, -4e-6},
		},
	}
	testFunction(BrownBadlyScaled{}, tests, t)
}

// TODO(vladimir-ch): The minimum of BrownAndDennis is not known accurately
// enough, which would force defaultGradTol to be unnecessarily large for the
// tests to pass. This is the only function that causes problems, so disable
// this test until the minimum is more accurate.
// func TestBrownAndDennis(t *testing.T) {
// 	tests := []funcTest{
// 		{
// 			X:        []float64{25, 5, -5, -1},
// 			F:        7926693.33699744,
// 			Gradient: []float64{1149322.836365895, 1779291.674339785, -254579.585463521, -173400.429253115},
// 		},
// 	}
// 	testFunction(BrownAndDennis{}, tests, t)
// }

func TestExtendedPowellSingular(t *testing.T) {
	tests := []funcTest{
		{
			X:        []float64{3, -1, 0, 3},
			F:        95,
			Gradient: []float64{-14, -144, -22, 30},
		},
		{
			X:        []float64{3, -1, 0, 3, 3, -1, 0, 3},
			F:        190,
			Gradient: []float64{-14, -144, -22, 30, -14, -144, -22, 30},
		},
	}
	testFunction(ExtendedPowellSingular{}, tests, t)
}

func TestExtendedRosenbrock(t *testing.T) {
	tests := []funcTest{
		{
			X:        []float64{-1.2, 1},
			F:        24.2,
			Gradient: []float64{-215.6, -88},
		},
		{
			X:        []float64{-1.2, 1, -1.2},
			F:        508.2,
			Gradient: []float64{-215.6, 792, -440},
		},
		{
			X:        []float64{-1.2, 1, -1.2, 1},
			F:        532.4,
			Gradient: []float64{-215.6, 792, -655.6, -88},
		},
	}
	testFunction(ExtendedRosenbrock{}, tests, t)
}

func TestGaussian(t *testing.T) {
	tests := []funcTest{
		{
			X:        []float64{0.4, 1, 0},
			F:        3.88810699116688e-06,
			Gradient: []float64{7.41428466839991e-03, -7.44126392165149e-04, -5.30189685421989e-20},
		},
	}
	testFunction(Gaussian{}, tests, t)
}

func TestGulfResearchAndDevelopment(t *testing.T) {
	tests := []funcTest{
		{
			X:        []float64{5, 2.5, 0.15},
			F:        12.11070582556949,
			Gradient: []float64{2.0879783574289799, 0.0345792619697154, -39.6766801029386400},
		},
	}
	testFunction(GulfResearchAndDevelopment{}, tests, t)
}

func TestHelicalValley(t *testing.T) {
	tests := []funcTest{
		{
			X:        []float64{-1, 0, 0},
			F:        2500,
			Gradient: []float64{0, -1.59154943091895e+03, -1e+03},
		},
	}
	testFunction(HelicalValley{}, tests, t)
}

func TestPenaltyI(t *testing.T) {
	tests := []funcTest{
		{
			X:        []float64{1, 2, 3, 4},
			F:        885.06264,
			Gradient: []float64{119, 238.00002, 357.00004, 476.00006},
		},
		{
			X: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			F: 148032.56535,
			Gradient: []float64{1539, 3078.00002, 4617.00004, 6156.00006,
				7695.00008, 9234.0001, 10773.00012, 12312.00014, 13851.00016, 15390.00018},
		},
	}
	testFunction(PenaltyI{}, tests, t)
}

func TestPenaltyII(t *testing.T) {
	tests := []funcTest{
		{
			X: []float64{0.5, 0.5, 0.5, 0.5},
			F: 2.34000880546302,
			Gradient: []float64{12.59999952896435, 8.99999885134508,
				5.99999776830493, 2.99999875380719},
		},
		{
			X: []float64{0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5},
			F: 162.65277656596712,
			Gradient: []float64{255.5999995289644, 229.4999988513451,
				203.9999977683049, 178.4999965713605, 152.9999952485322,
				127.4999937865809, 101.9999921708749, 76.4999903852436,
				50.9999884118158, 25.4999938418451},
		},
	}
	testFunction(PenaltyII{}, tests, t)
}

func TestPowelBadlyScaled(t *testing.T) {
	tests := []funcTest{
		{
			X:        []float64{0, 1},
			F:        1.13526171734838,
			Gradient: []float64{-2.00007355588823e+04, -2.70596990584991e-01},
		},
	}
	testFunction(PowellBadlyScaled{}, tests, t)
}

func TestTrigonometric(t *testing.T) {
	tests := []funcTest{
		{
			X:        []float64{0.5, 0.5},
			F:        0.0126877761614045,
			Gradient: []float64{-0.00840962732040673, -0.09606967736232540},
		},
		{
			X: []float64{0.2, 0.2, 0.2, 0.2, 0.2},
			F: 0.0116573789904718,
			Gradient: []float64{0.04568602319608119, -0.00896259022885634,
				-0.04777056509084983, -0.07073790138989976, -0.07786459912600564},
		},
		{
			X: []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
			F: 0.00707575946622261,
			Gradient: []float64{0.03562782195259399, 0.01872017956076182,
				0.00380754216611998, -0.00911009023133202, -0.02003271763159338,
				-0.02896034003466506, -0.03589295744054654, -0.04083056984923782,
				-0.04377317726073873, -0.04472077967504980},
		},
	}
	testFunction(Trigonometric{}, tests, t)
}

func TestVariablyDimensioned(t *testing.T) {
	tests := []funcTest{
		{
			X:        []float64{0.5, 0},
			F:        46.5625,
			Gradient: []float64{-68.5, -137},
		},
		{
			X:        []float64{2.0 / 3, 1.0 / 3, 0},
			F:        497.60493827160514,
			Gradient: []float64{-416.518518518519, -833.037037037037, -1249.555555555556},
		},
		{
			X:        []float64{0.75, 0.5, 0.25, 0},
			F:        3222.1875,
			Gradient: []float64{-1703, -3406, -5109, -6812},
		},
	}
	testFunction(VariablyDimensioned{}, tests, t)
}

func TestWatson(t *testing.T) {
	tests := []funcTest{
		{
			X:        []float64{0, 0},
			F:        30,
			Gradient: []float64{0, -60},
		},
		{
			X: []float64{0, 0, 0, 0, 0, 0},
			F: 30,
			Gradient: []float64{0, -60, -60, -61.034482758620697,
				-62.068965517241381, -63.114928861371936},
		},
		{
			X: []float64{0, 0, 0, 0, 0, 0, 0, 0, 0},
			F: 30,
			Gradient: []float64{0, -60, -60, -61.034482758620697,
				-62.068965517241381, -63.114928861371936, -64.172372791012350,
				-65.241283655050239, -66.321647802373235},
		},
		{
			X: []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			F: 30,
			Gradient: []float64{0, -60, -60, -61.034482758620697,
				-62.068965517241381, -63.114928861371936, -64.172372791012350,
				-65.241283655050239, -66.321647802373235, -67.413448880864095,
				-68.516667837400661, -69.631282933991471},
		},
	}
	testFunction(Watson{}, tests, t)
}

func TestWood(t *testing.T) {
	tests := []funcTest{
		{
			X:        []float64{-3, -1, -3, -1},
			F:        19192,
			Gradient: []float64{-12008, -2080, -10808, -1880},
		},
	}
	testFunction(Wood{}, tests, t)
}
