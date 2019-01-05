      DOUBLE PRECISION FUNCTION MZABS(ZR, ZI)
C***BEGIN PROLOGUE  ZABS
C***REFER TO  ZBESH,ZBESI,ZBESJ,ZBESK,ZBESY,ZAIRY,ZBIRY
C
C     ZABS COMPUTES THE ABSOLUTE VALUE OR MAGNITUDE OF A DOUBLE
C     PRECISION COMPLEX VARIABLE CMPLX(ZR,ZI)
C
C***ROUTINES CALLED  (NONE)
C***END PROLOGUE  ZABS
      DOUBLE PRECISION ZR, ZI, U, V, Q, S

      MZABS = ZABS(CMPLX(ZR,ZI,kind=KIND(1.0D0)))
      RETURN
      END

c      U = DABS(ZR)
c      V = DABS(ZI)
c      S = U + V
C-----------------------------------------------------------------------
C     S*1.0D0 MAKES AN UNNORMALIZED UNDERFLOW ON CDC MACHINES INTO A
C     TRUE FLOATING ZERO
C-----------------------------------------------------------------------
c      S = S*1.0D+0
c      IF (S.EQ.0.0D+0) GO TO 20
c      IF (U.GT.V) GO TO 10
c      Q = U/V
c      ZABS = V*DSQRT(1.D+0+Q*Q)
c      RETURN
c   10 Q = V/U
c      ZABS = U*DSQRT(1.D+0+Q*Q)
c      RETURN
c   20 ZABS = 0.0D+0
c      RETURN
c      END
