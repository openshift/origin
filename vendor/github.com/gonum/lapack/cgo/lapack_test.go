// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cgo

import (
	"testing"

	"github.com/gonum/blas"
	"github.com/gonum/lapack/testlapack"
)

var impl = Implementation{}

// blockedTranslate transforms some blocked C calls to be the unblocked algorithms
// for testing, as several of the unblocked algorithms are not defined by the C
// interface.
type blockedTranslate struct {
	Implementation
}

func TestDbdsqr(t *testing.T) {
	testlapack.DbdsqrTest(t, impl)
}

func (bl blockedTranslate) Dgebd2(m, n int, a []float64, lda int, d, e, tauQ, tauP, work []float64) {
	impl.Dgebrd(m, n, a, lda, d, e, tauQ, tauP, work, len(work))
}

func (bl blockedTranslate) Dorm2r(side blas.Side, trans blas.Transpose, m, n, k int, a []float64, lda int, tau, c []float64, ldc int, work []float64) {
	impl.Dormqr(side, trans, m, n, k, a, lda, tau, c, ldc, work, len(work))
}

func (bl blockedTranslate) Dorml2(side blas.Side, trans blas.Transpose, m, n, k int, a []float64, lda int, tau, c []float64, ldc int, work []float64) {
	impl.Dormlq(side, trans, m, n, k, a, lda, tau, c, ldc, work, len(work))
}

func (bl blockedTranslate) Dorg2r(m, n, k int, a []float64, lda int, tau, work []float64) {
	impl.Dorgqr(m, n, k, a, lda, tau, work, len(work))
}

func (bl blockedTranslate) Dorgl2(m, n, k int, a []float64, lda int, tau, work []float64) {
	impl.Dorglq(m, n, k, a, lda, tau, work, len(work))
}

func TestDgeqp3(t *testing.T) {
	testlapack.Dgeqp3Test(t, impl)
}

func TestDlacn2(t *testing.T) {
	testlapack.Dlacn2Test(t, impl)
}

func TestDlascl(t *testing.T) {
	testlapack.DlasclTest(t, impl)
}

func TestDlacpy(t *testing.T) {
	testlapack.DlacpyTest(t, impl)
}

func TestDlange(t *testing.T) {
	testlapack.DlangeTest(t, impl)
}

func TestDlarfb(t *testing.T) {
	testlapack.DlarfbTest(t, impl)
}

func TestDlarfg(t *testing.T) {
	testlapack.DlarfgTest(t, impl)
}

func TestDlarft(t *testing.T) {
	testlapack.DlarftTest(t, impl)
}

func TestDlantr(t *testing.T) {
	testlapack.DlantrTest(t, impl)
}

func TestDlapmt(t *testing.T) {
	testlapack.DlapmtTest(t, impl)
}

func TestDlapy2(t *testing.T) {
	testlapack.Dlapy2Test(t, impl)
}

func TestDlarfx(t *testing.T) {
	testlapack.DlarfxTest(t, impl)
}

func TestDlaset(t *testing.T) {
	testlapack.DlasetTest(t, impl)
}

func TestDlasrt(t *testing.T) {
	testlapack.DlasrtTest(t, impl)
}

func TestDlaswp(t *testing.T) {
	testlapack.DlaswpTest(t, impl)
}

func TestDpotrf(t *testing.T) {
	testlapack.DpotrfTest(t, impl)
}

func TestDgebak(t *testing.T) {
	testlapack.DgebakTest(t, impl)
}

func TestDgebal(t *testing.T) {
	testlapack.DgebalTest(t, impl)
}

func TestDgebd2(t *testing.T) {
	testlapack.Dgebd2Test(t, blockedTranslate{impl})
}

func TestDgecon(t *testing.T) {
	testlapack.DgeconTest(t, impl)
}

func TestDgeev(t *testing.T) {
	testlapack.DgeevTest(t, impl)
}

func TestDgehrd(t *testing.T) {
	testlapack.DgehrdTest(t, impl)
}

func TestDgelq2(t *testing.T) {
	testlapack.Dgelq2Test(t, impl)
}

func TestDgels(t *testing.T) {
	testlapack.DgelsTest(t, impl)
}

func TestDgelqf(t *testing.T) {
	testlapack.DgelqfTest(t, impl)
}

func TestDgeqr2(t *testing.T) {
	testlapack.Dgeqr2Test(t, impl)
}

func TestDgeqrf(t *testing.T) {
	testlapack.DgeqrfTest(t, impl)
}

func TestDgerqf(t *testing.T) {
	testlapack.DgerqfTest(t, impl)
}

func TestDgesvd(t *testing.T) {
	testlapack.DgesvdTest(t, impl)
}

func TestDgetf2(t *testing.T) {
	testlapack.Dgetf2Test(t, impl)
}

func TestDgetrf(t *testing.T) {
	testlapack.DgetrfTest(t, impl)
}

func TestDgetri(t *testing.T) {
	testlapack.DgetriTest(t, impl)
}

func TestDgetrs(t *testing.T) {
	testlapack.DgetrsTest(t, impl)
}

func TestDggsvd3(t *testing.T) {
	testlapack.Dggsvd3Test(t, impl)
}

func TestDggsvp3(t *testing.T) {
	testlapack.Dggsvp3Test(t, impl)
}

func TestDhseqr(t *testing.T) {
	testlapack.DhseqrTest(t, impl)
}

func TestDorglq(t *testing.T) {
	testlapack.DorglqTest(t, blockedTranslate{impl})
}

func TestDorgql(t *testing.T) {
	testlapack.DorgqlTest(t, impl)
}

func TestDorgqr(t *testing.T) {
	testlapack.DorgqrTest(t, blockedTranslate{impl})
}

func TestDorgtr(t *testing.T) {
	testlapack.DorgtrTest(t, impl)
}

func TestDorgl2(t *testing.T) {
	testlapack.Dorgl2Test(t, blockedTranslate{impl})
}

func TestDorg2r(t *testing.T) {
	testlapack.Dorg2rTest(t, blockedTranslate{impl})
}

func TestDormbr(t *testing.T) {
	testlapack.DormbrTest(t, blockedTranslate{impl})
}

func TestDormhr(t *testing.T) {
	testlapack.DormhrTest(t, impl)
}

func TestDorgbr(t *testing.T) {
	testlapack.DorgbrTest(t, blockedTranslate{impl})
}

func TestDorghr(t *testing.T) {
	testlapack.DorghrTest(t, impl)
}

func TestDormqr(t *testing.T) {
	testlapack.Dorm2rTest(t, blockedTranslate{impl})
}

func TestDormlq(t *testing.T) {
	testlapack.Dorml2Test(t, blockedTranslate{impl})
}

func TestDpocon(t *testing.T) {
	testlapack.DpoconTest(t, impl)
}

func TestDsteqr(t *testing.T) {
	testlapack.DsteqrTest(t, impl)
}

func TestDsterf(t *testing.T) {
	testlapack.DsterfTest(t, impl)
}

func TestDsyev(t *testing.T) {
	testlapack.DsyevTest(t, impl)
}

func TestDsytrd(t *testing.T) {
	testlapack.DsytrdTest(t, impl)
}

func TestDtgsja(t *testing.T) {
	testlapack.DtgsjaTest(t, impl)
}

func TestDtrexc(t *testing.T) {
	testlapack.DtrexcTest(t, impl)
}

func TestDtrcon(t *testing.T) {
	testlapack.DtrconTest(t, impl)
}

func TestDtrtri(t *testing.T) {
	testlapack.DtrtriTest(t, impl)
}
