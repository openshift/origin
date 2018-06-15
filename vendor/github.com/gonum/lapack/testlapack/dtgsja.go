// Copyright ©2017 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"math/rand"
	"testing"

	"github.com/gonum/blas"
	"github.com/gonum/blas/blas64"
	"github.com/gonum/floats"
	"github.com/gonum/lapack"
)

type Dtgsjaer interface {
	Dlanger
	Dtgsja(jobU, jobV, jobQ lapack.GSVDJob, m, p, n, k, l int, a []float64, lda int, b []float64, ldb int, tola, tolb float64, alpha, beta, u []float64, ldu int, v []float64, ldv int, q []float64, ldq int, work []float64) (cycles int, ok bool)
}

func DtgsjaTest(t *testing.T, impl Dtgsjaer) {
	rnd := rand.New(rand.NewSource(1))
	for cas, test := range []struct {
		m, p, n, k, l, lda, ldb, ldu, ldv, ldq int

		ok bool
	}{
		{m: 5, p: 5, n: 5, k: 2, l: 2, lda: 0, ldb: 0, ldu: 0, ldv: 0, ldq: 0, ok: true},
		{m: 5, p: 5, n: 5, k: 4, l: 1, lda: 0, ldb: 0, ldu: 0, ldv: 0, ldq: 0, ok: true},
		{m: 5, p: 5, n: 10, k: 2, l: 2, lda: 0, ldb: 0, ldu: 0, ldv: 0, ldq: 0, ok: true},
		{m: 5, p: 5, n: 10, k: 4, l: 1, lda: 0, ldb: 0, ldu: 0, ldv: 0, ldq: 0, ok: true},
		{m: 5, p: 5, n: 10, k: 4, l: 2, lda: 0, ldb: 0, ldu: 0, ldv: 0, ldq: 0, ok: true},
		{m: 10, p: 5, n: 5, k: 2, l: 2, lda: 0, ldb: 0, ldu: 0, ldv: 0, ldq: 0, ok: true},
		{m: 10, p: 5, n: 5, k: 4, l: 1, lda: 0, ldb: 0, ldu: 0, ldv: 0, ldq: 0, ok: true},
		{m: 10, p: 10, n: 10, k: 5, l: 3, lda: 0, ldb: 0, ldu: 0, ldv: 0, ldq: 0, ok: true},
		{m: 10, p: 10, n: 10, k: 6, l: 4, lda: 0, ldb: 0, ldu: 0, ldv: 0, ldq: 0, ok: true},
		{m: 5, p: 5, n: 5, k: 2, l: 2, lda: 10, ldb: 10, ldu: 10, ldv: 10, ldq: 10, ok: true},
		{m: 5, p: 5, n: 5, k: 4, l: 1, lda: 10, ldb: 10, ldu: 10, ldv: 10, ldq: 10, ok: true},
		{m: 5, p: 5, n: 10, k: 2, l: 2, lda: 20, ldb: 20, ldu: 10, ldv: 10, ldq: 20, ok: true},
		{m: 5, p: 5, n: 10, k: 4, l: 1, lda: 20, ldb: 20, ldu: 10, ldv: 10, ldq: 20, ok: true},
		{m: 5, p: 5, n: 10, k: 4, l: 2, lda: 20, ldb: 20, ldu: 10, ldv: 10, ldq: 20, ok: true},
		{m: 10, p: 5, n: 5, k: 2, l: 2, lda: 10, ldb: 10, ldu: 20, ldv: 10, ldq: 10, ok: true},
		{m: 10, p: 5, n: 5, k: 4, l: 1, lda: 10, ldb: 10, ldu: 20, ldv: 10, ldq: 10, ok: true},
		{m: 10, p: 10, n: 10, k: 5, l: 3, lda: 20, ldb: 20, ldu: 20, ldv: 20, ldq: 20, ok: true},
		{m: 10, p: 10, n: 10, k: 6, l: 4, lda: 20, ldb: 20, ldu: 20, ldv: 20, ldq: 20, ok: true},
	} {
		m := test.m
		p := test.p
		n := test.n
		k := test.k
		l := test.l
		lda := test.lda
		if lda == 0 {
			lda = n
		}
		ldb := test.ldb
		if ldb == 0 {
			ldb = n
		}
		ldu := test.ldu
		if ldu == 0 {
			ldu = m
		}
		ldv := test.ldv
		if ldv == 0 {
			ldv = p
		}
		ldq := test.ldq
		if ldq == 0 {
			ldq = n
		}

		a := blockedUpperTriGeneral(m, n, k, l, lda, true, rnd)
		aCopy := cloneGeneral(a)
		b := blockedUpperTriGeneral(p, n, k, l, ldb, false, rnd)
		bCopy := cloneGeneral(b)

		tola := float64(max(m, n)) * impl.Dlange(lapack.NormFrob, m, n, a.Data, a.Stride, nil) * dlamchE
		tolb := float64(max(p, n)) * impl.Dlange(lapack.NormFrob, p, n, b.Data, b.Stride, nil) * dlamchE

		alpha := make([]float64, n)
		beta := make([]float64, n)

		work := make([]float64, 2*n)

		u := nanGeneral(m, m, ldu)
		v := nanGeneral(p, p, ldv)
		q := nanGeneral(n, n, ldq)

		_, ok := impl.Dtgsja(lapack.GSVDUnit, lapack.GSVDUnit, lapack.GSVDUnit,
			m, p, n, k, l,
			a.Data, a.Stride,
			b.Data, b.Stride,
			tola, tolb,
			alpha, beta,
			u.Data, u.Stride,
			v.Data, v.Stride,
			q.Data, q.Stride,
			work)

		if !ok {
			if test.ok {
				t.Errorf("test %d unexpectedly did not converge", cas)
			}
			continue
		}

		// Check orthogonality of U, V and Q.
		if !isOrthonormal(u) {
			t.Errorf("test %d: U is not orthogonal\n%+v", cas, u)
		}
		if !isOrthonormal(v) {
			t.Errorf("test %d: V is not orthogonal\n%+v", cas, v)
		}
		if !isOrthonormal(q) {
			t.Errorf("test %d: Q is not orthogonal\n%+v", cas, q)
		}

		// Check C^2 + S^2 = I.
		var elements []float64
		if m-k-l >= 0 {
			elements = alpha[k : k+l]
		} else {
			elements = alpha[k:m]
		}
		for i := range elements {
			i += k
			d := alpha[i]*alpha[i] + beta[i]*beta[i]
			if !floats.EqualWithinAbsOrRel(d, 1, 1e-14, 1e-14) {
				t.Errorf("test %d: alpha_%d^2 + beta_%d^2 != 1: got: %v", cas, i, i, d)
			}
		}

		zeroR, d1, d2 := constructGSVDresults(n, p, m, k, l, a, b, alpha, beta)

		// Check U^T*A*Q = D1*[ 0 R ].
		uTmp := nanGeneral(m, n, n)
		blas64.Gemm(blas.Trans, blas.NoTrans, 1, u, aCopy, 0, uTmp)
		uAns := nanGeneral(m, n, n)
		blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, uTmp, q, 0, uAns)

		d10r := nanGeneral(m, n, n)
		blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, d1, zeroR, 0, d10r)

		if !equalApproxGeneral(uAns, d10r, 1e-14) {
			t.Errorf("test %d: U^T*A*Q != D1*[ 0 R ]\nU^T*A*Q:\n%+v\nD1*[ 0 R ]:\n%+v",
				cas, uAns, d10r)
		}

		// Check V^T*B*Q = D2*[ 0 R ].
		vTmp := nanGeneral(p, n, n)
		blas64.Gemm(blas.Trans, blas.NoTrans, 1, v, bCopy, 0, vTmp)
		vAns := nanGeneral(p, n, n)
		blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, vTmp, q, 0, vAns)

		d20r := nanGeneral(p, n, n)
		blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, d2, zeroR, 0, d20r)

		if !equalApproxGeneral(vAns, d20r, 1e-14) {
			t.Errorf("test %d: V^T*B*Q != D2*[ 0 R ]\nV^T*B*Q:\n%+v\nD2*[ 0 R ]:\n%+v",
				cas, vAns, d20r)
		}
	}
}
