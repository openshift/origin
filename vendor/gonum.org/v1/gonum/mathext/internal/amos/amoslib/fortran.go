// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build fortran
// TODO(jonlawlor): remove fortran build tag when gonum only supports go 1.7+.

package amoslib

/*
double mzabs_(double * ar, double * ai);
void zs1s2_(double * ZRR, double * ZRI, double * S1R, double * S1I, double * S2R, double * S2I, int* NZ, double *ASCLE, double * ALIM, int * IUF);
void zacai_(double * ZR, double * ZI, double * FNU, int * KODE, int * N, int * MR, double * YR, double * YI, int * NZ, double * RL, double * tol, double * elim, double * alim);
void zseri_(double * ZR, double * ZI, double * FNU, int * KODE, int * N, double * YR, double * YI, int * NZ, double * tol, double * elim, double * alim);
void zmlri_(double * ZR, double * ZI, double * FNU, int * KODE, int * N, double * YR, double * YI, int * NZ, double * tol);
void zbknu_(double * ZR, double * ZI, double * FNU, int * KODE, int * N, double * YR, double * YI, int * NZ, double * tol, double * elim, double * alim);
void zasyi_(double * ZR, double * ZI, double * FNU, int * KODE, int * N, double * YR, double * YI, int * NZ,double * RL, double * tol, double * elim, double * alim);
void zkscl_(double * ZRR, double * ZRI, double * FNU, int * N, double * YR, double * YI, int * NZ, double * RZR, double * RZI, double * ASCLE, double * tol, double * elim);
void zuchk_(double * YR, double * YI, int * NZ, double * ASCLE, double * TOL);
void zairy_(double * ZR, double * ZI, int * ID, int * KODE, double * AIR, double * AII, int * NZ, int * IERR);
void zlog_(double * ar, double * ai, double * br, double * bi, int * ierr);
void zexp_(double * ar, double * ai, double * br, double * bi);
void zsqrt_(double * ar, double * ai, double * br, double * bi);
void zdiv_(double * ar, double * ai, double * br, double * bi, double * cr, double * ci);
void zmlt_(double * ar, double * ai, double * br, double * bi, double * cr, double * ci);
double dgamln_(double *z, int * ierr);
void zshch_(double * zr, double * zi, double * cshr, double * cshi, double * cchr, double * cchi);
double mysqrt_(double * A);
double myexp_(double * A);
double mycos_(double * A);
double mysin_(double * A);
double mylog_(double * A);
double mytan_(double * A);
double myatan_(double * A);
double myabs_(double * A);
double mymin_(double * A, double * B);
double mymax_(double * A, double * B);
*/
import "C"
import "unsafe"

func MinFort(a, b float64) float64 {
	ans := C.mymin_((*C.double)(&a), (*C.double)(&b))
	return float64(ans)
}

func MaxFort(a, b float64) float64 {
	ans := C.mymax_((*C.double)(&a), (*C.double)(&b))
	return float64(ans)
}

func AbsFort(a float64) float64 {
	ans := C.myabs_((*C.double)(&a))
	return float64(ans)
}

func AtanFort(a float64) float64 {
	ans := C.myatan_((*C.double)(&a))
	return float64(ans)
}

func TanFort(a float64) float64 {
	ans := C.mytan_((*C.double)(&a))
	return float64(ans)
}

func LogFort(a float64) float64 {
	ans := C.mylog_((*C.double)(&a))
	return float64(ans)
}

func SinFort(a float64) float64 {
	ans := C.mysin_((*C.double)(&a))
	return float64(ans)
}

func CosFort(a float64) float64 {
	ans := C.mycos_((*C.double)(&a))
	return float64(ans)
}

func ExpFort(a float64) float64 {
	ans := C.myexp_((*C.double)(&a))
	return float64(ans)
}

func SqrtFort(a float64) float64 {
	ans := C.mysqrt_((*C.double)(&a))
	return float64(ans)
}

func DgamlnFort(a float64) float64 {
	var ierr int
	pierr := (*C.int)(unsafe.Pointer(&ierr))
	pa := (*C.double)(&a)
	ans := C.dgamln_(pa, pierr)
	return (float64)(ans)
}

func ZmltFort(a, b complex128) complex128 {
	ar := real(a)
	ai := imag(a)
	br := real(b)
	bi := imag(b)
	var cr, ci float64
	C.zmlt_(
		(*C.double)(&ar), (*C.double)(&ai),
		(*C.double)(&br), (*C.double)(&bi),
		(*C.double)(&cr), (*C.double)(&ci),
	)
	return complex(cr, ci)
}

func ZdivFort(a, b complex128) complex128 {
	ar := real(a)
	ai := imag(a)
	br := real(b)
	bi := imag(b)
	var cr, ci float64
	C.zdiv_(
		(*C.double)(&ar), (*C.double)(&ai),
		(*C.double)(&br), (*C.double)(&bi),
		(*C.double)(&cr), (*C.double)(&ci),
	)
	return complex(cr, ci)
}

func ZabsFort(a complex128) float64 {
	ar := real(a)
	ai := imag(a)
	return float64(C.mzabs_((*C.double)(&ar), (*C.double)(&ai)))
}

func ZsqrtFort(a complex128) (b complex128) {
	ar := real(a)
	ai := imag(a)

	var br, bi float64

	par := (*C.double)(&ar)
	pai := (*C.double)(&ai)
	pbr := (*C.double)(&br)
	pbi := (*C.double)(&bi)

	C.zsqrt_(par, pai, pbr, pbi)
	return complex(br, bi)
}

func ZexpFort(a complex128) (b complex128) {
	ar := real(a)
	ai := imag(a)

	var br, bi float64

	par := (*C.double)(&ar)
	pai := (*C.double)(&ai)
	pbr := (*C.double)(&br)
	pbi := (*C.double)(&bi)

	C.zexp_(par, pai, pbr, pbi)
	return complex(br, bi)
}

func ZlogFort(a complex128) (b complex128) {
	ar := real(a)
	ai := imag(a)
	var ierr int
	var br, bi float64

	par := (*C.double)(&ar)
	pai := (*C.double)(&ai)
	pbr := (*C.double)(&br)
	pbi := (*C.double)(&bi)
	pierr := (*C.int)(unsafe.Pointer(&ierr))
	C.zlog_(par, pai, pbr, pbi, pierr)
	return complex(br, bi)
}

func Zshch(ZR, ZI, CSHR, CSHI, CCHR, CCHI float64) (ZRout, ZIout, CSHRout, CSHIout, CCHRout, CCHIout float64) {
	pzr := (*C.double)(&ZR)
	pzi := (*C.double)(&ZI)
	pcshr := (*C.double)(&CSHR)
	pcshi := (*C.double)(&CSHI)
	pcchr := (*C.double)(&CCHR)
	pcchi := (*C.double)(&CCHI)

	C.zshch_(pzr, pzi, pcshr, pcshi, pcchr, pcchi)
	return ZR, ZI, CSHR, CSHI, CCHR, CCHI
}

func ZairyFort(ZR, ZI float64, ID, KODE int) (AIR, AII float64, NZ int) {
	var IERR int
	pzr := (*C.double)(&ZR)
	pzi := (*C.double)(&ZI)
	pid := (*C.int)(unsafe.Pointer(&ID))
	pkode := (*C.int)(unsafe.Pointer(&KODE))

	pair := (*C.double)(&AIR)
	paii := (*C.double)(&AII)
	pnz := (*C.int)(unsafe.Pointer(&NZ))
	pierr := (*C.int)(unsafe.Pointer(&IERR))
	C.zairy_(pzr, pzi, pid, pkode, pair, paii, pnz, pierr)

	NZ = int(*pnz)
	return AIR, AII, NZ
}

func ZksclFort(ZRR, ZRI, FNU float64, N int, YR, YI []float64, NZ int, RZR, RZI, ASCLE, TOL, ELIM float64) (
	ZRout, ZIout, FNUout float64, Nout int, YRout, YIout []float64, NZout int, RZRout, RZIout, ASCLEout, TOLout, ELIMout float64) {

	pzrr := (*C.double)(&ZRR)
	pzri := (*C.double)(&ZRI)
	pfnu := (*C.double)(&FNU)
	pn := (*C.int)(unsafe.Pointer(&N))
	pyr := (*C.double)(&YR[0])
	pyi := (*C.double)(&YI[0])
	pnz := (*C.int)(unsafe.Pointer(&NZ))
	przr := (*C.double)(&RZR)
	przi := (*C.double)(&RZI)
	pascle := (*C.double)(&ASCLE)
	ptol := (*C.double)(&TOL)
	pelim := (*C.double)(&ELIM)

	C.zkscl_(pzrr, pzri, pfnu, pn, pyr, pyi, pnz, przr, przi, pascle, ptol, pelim)
	N = int(*pn)
	NZ = int(*pnz)
	return ZRR, ZRI, FNU, N, YR, YI, NZ, RZR, RZI, ASCLE, TOL, ELIM
}

func ZbknuFort(ZR, ZI, FNU float64, KODE, N int, YR, YI []float64, NZ int, TOL, ELIM, ALIM float64) (
	ZRout, ZIout, FNUout float64, KODEout, Nout int, YRout, YIout []float64, NZout int, TOLout, ELIMout, ALIMout float64) {

	pzr := (*C.double)(&ZR)
	pzi := (*C.double)(&ZI)
	pfnu := (*C.double)(&FNU)
	pkode := (*C.int)(unsafe.Pointer(&KODE))
	pn := (*C.int)(unsafe.Pointer(&N))
	pyr := (*C.double)(&YR[0])
	pyi := (*C.double)(&YI[0])
	pnz := (*C.int)(unsafe.Pointer(&NZ))
	ptol := (*C.double)(&TOL)
	pelim := (*C.double)(&ELIM)
	palim := (*C.double)(&ALIM)

	C.zbknu_(pzr, pzi, pfnu, pkode, pn, pyr, pyi, pnz, ptol, pelim, palim)
	KODE = int(*pkode)
	N = int(*pn)
	NZ = int(*pnz)
	return ZR, ZI, FNU, KODE, N, YR, YI, NZ, TOL, ELIM, ALIM
}

func ZasyiFort(ZR, ZI, FNU float64, KODE, N int, YR, YI []float64, NZ int, RL, TOL, ELIM, ALIM float64) (
	ZRout, ZIout, FNUout float64, KODEout, Nout int, YRout, YIout []float64, NZout int, RLout, TOLout, ELIMout, ALIMout float64) {

	pzr := (*C.double)(&ZR)
	pzi := (*C.double)(&ZI)
	pfnu := (*C.double)(&FNU)
	pkode := (*C.int)(unsafe.Pointer(&KODE))
	pn := (*C.int)(unsafe.Pointer(&N))
	pyr := (*C.double)(&YR[0])
	pyi := (*C.double)(&YI[0])
	pnz := (*C.int)(unsafe.Pointer(&NZ))
	prl := (*C.double)(&RL)
	ptol := (*C.double)(&TOL)
	pelim := (*C.double)(&ELIM)
	palim := (*C.double)(&ALIM)

	C.zasyi_(pzr, pzi, pfnu, pkode, pn, pyr, pyi, pnz, prl, ptol, pelim, palim)
	KODE = int(*pkode)
	N = int(*pn)
	NZ = int(*pnz)
	return ZR, ZI, FNU, KODE, N, YR, YI, NZ, RL, TOL, ELIM, ALIM
}

func ZuchkFort(YR, YI float64, NZ int, ASCLE, TOL float64) (YRout, YIout float64, NZout int, ASCLEout, TOLout float64) {
	pyr := (*C.double)(&YR)
	pyi := (*C.double)(&YI)
	pnz := (*C.int)(unsafe.Pointer(&NZ))
	pascle := (*C.double)(&ASCLE)
	ptol := (*C.double)(&TOL)

	C.zuchk_(pyr, pyi, pnz, pascle, ptol)
	return YR, YI, NZ, ASCLE, TOL
}

func Zs1s2Fort(ZRR, ZRI, S1R, S1I, S2R, S2I float64, NZ int, ASCLE, ALIM float64, IUF int) (
	ZRRout, ZRIout, S1Rout, S1Iout, S2Rout, S2Iout float64, NZout int, ASCLEout, ALIMout float64, IUFout int) {

	pzrr := (*C.double)(&ZRR)
	pzri := (*C.double)(&ZRI)
	ps1r := (*C.double)(&S1R)
	ps1i := (*C.double)(&S1I)
	ps2r := (*C.double)(&S2R)
	ps2i := (*C.double)(&S2I)
	pnz := (*C.int)(unsafe.Pointer(&NZ))
	pascle := (*C.double)(&ASCLE)
	palim := (*C.double)(&ALIM)
	piuf := (*C.int)(unsafe.Pointer(&IUF))

	C.zs1s2_(pzrr, pzri, ps1r, ps1i, ps2r, ps2i, pnz, pascle, palim, piuf)
	return ZRR, ZRI, S1R, S1I, S2R, S2I, NZ, ASCLE, ALIM, IUF
}

func ZacaiFort(ZR, ZI, FNU float64, KODE, MR, N int, YR, YI []float64, NZ int, RL, TOL, ELIM, ALIM float64) (
	ZRout, ZIout, FNUout float64, KODEout, MRout, Nout int, YRout, YIout []float64, NZout int, RLout, TOLout, ELIMout, ALIMout float64) {
	pzr := (*C.double)(&ZR)
	pzi := (*C.double)(&ZI)
	pfnu := (*C.double)(&FNU)
	pkode := (*C.int)(unsafe.Pointer(&KODE))
	pmr := (*C.int)(unsafe.Pointer(&MR))
	pn := (*C.int)(unsafe.Pointer(&N))
	pyr := (*C.double)(&YR[0])
	pyi := (*C.double)(&YI[0])
	pnz := (*C.int)(unsafe.Pointer(&NZ))
	prl := (*C.double)(&RL)
	ptol := (*C.double)(&TOL)
	pelim := (*C.double)(&ELIM)
	palim := (*C.double)(&ALIM)

	C.zacai_(pzr, pzi, pfnu, pkode, pmr, pn, pyr, pyi, pnz, prl, ptol, pelim, palim)
	KODE = int(*pkode)
	MR = int(*pmr)
	N = int(*pn)
	NZ = int(*pnz)
	return ZR, ZI, FNU, KODE, MR, N, YR, YI, NZ, RL, TOL, ELIM, ALIM
}

func ZseriFort(ZR, ZI, FNU float64, KODE, N int, YR, YI []float64, NZ int, TOL, ELIM, ALIM float64) (
	ZRout, ZIout, FNUout float64, KODEout, Nout int, YRout, YIout []float64, NZout int, TOLout, ELIMout, ALIMout float64) {
	pzr := (*C.double)(&ZR)
	pzi := (*C.double)(&ZI)
	pfnu := (*C.double)(&FNU)
	pkode := (*C.int)(unsafe.Pointer(&KODE))
	pn := (*C.int)(unsafe.Pointer(&N))
	pyr := (*C.double)(&YR[0])
	pyi := (*C.double)(&YI[0])
	pnz := (*C.int)(unsafe.Pointer(&NZ))
	ptol := (*C.double)(&TOL)
	pelim := (*C.double)(&ELIM)
	palim := (*C.double)(&ALIM)

	C.zseri_(pzr, pzi, pfnu, pkode, pn, pyr, pyi, pnz, ptol, pelim, palim)
	KODE = int(*pkode)
	N = int(*pn)
	NZ = int(*pnz)
	return ZR, ZI, FNU, KODE, N, YR, YI, NZ, TOL, ELIM, ALIM
}

func ZmlriFort(ZR, ZI, FNU float64, KODE, N int, YR, YI []float64, NZ int, TOL float64) (
	ZRout, ZIout, FNUout float64, KODEout, Nout int, YRout, YIout []float64, NZout int, TOLout float64) {
	pzr := (*C.double)(&ZR)
	pzi := (*C.double)(&ZI)
	pfnu := (*C.double)(&FNU)
	pkode := (*C.int)(unsafe.Pointer(&KODE))
	pn := (*C.int)(unsafe.Pointer(&N))
	pyr := (*C.double)(&YR[0])
	pyi := (*C.double)(&YI[0])
	pnz := (*C.int)(unsafe.Pointer(&NZ))
	ptol := (*C.double)(&TOL)

	C.zmlri_(pzr, pzi, pfnu, pkode, pn, pyr, pyi, pnz, ptol)
	KODE = int(*pkode)
	N = int(*pn)
	NZ = int(*pnz)
	return ZR, ZI, FNU, KODE, N, YR, YI, NZ, TOL
}
