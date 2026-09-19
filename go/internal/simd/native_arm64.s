#include "textflag.h"
// func dotNEON(a,b []float32) float32
TEXT ·dotNEON(SB),NOSPLIT,$0-52
 MOVD a_base+0(FP),R0
 MOVD a_len+8(FP),R2
 MOVD b_base+24(FP),R1
 VEOR V0.B16,V0.B16,V0.B16
neon_dot_loop:
 CMP $4,R2
 BLT neon_dot_reduce
 VLD1.P 16(R0),[V1.S4]
 VLD1.P 16(R1),[V2.S4]
 WORD $0x6e22dc21
 WORD $0x4e21d400
 SUB $4,R2,R2
 B neon_dot_loop
neon_dot_reduce:
 // Pairwise horizontal reduction avoids Go-assembler lane-move ambiguity.
 WORD $0x6e20d400 // FADDP V0.S4,V0.S4,V0.S4
 WORD $0x2e20d400 // FADDP V0.S2,V0.S2,V0.S2
 VMOV V0.S[0],R3
 FMOVS R3,F0
neon_dot_tail:
 CMP $0,R2
 BEQ neon_dot_done
 FMOVS (R0),F1
 FMOVS (R1),F2
 FMULS F2,F1,F1
 FADDS F1,F0,F0
 ADD $4,R0
 ADD $4,R1
 SUB $1,R2,R2
 B neon_dot_tail
neon_dot_done: FMOVS F0,ret+48(FP)
RET

// func axpyNEON(alpha float32,x,y []float32)
TEXT ·axpyNEON(SB),NOSPLIT,$0-56
 FMOVS alpha+0(FP),F3
 VDUP V3.S[0],V3.S4
 MOVD x_base+8(FP),R0
 MOVD x_len+16(FP),R2
 MOVD y_base+32(FP),R1
neon_axpy_loop:
 CMP $4,R2
 BLT neon_axpy_tail
 VLD1.P 16(R0),[V0.S4]
 WORD $0x6e23dc00
 VLD1 (R1),[V1.S4]
 WORD $0x4e20d421
 VST1.P [V1.S4],16(R1)
 SUB $4,R2,R2
 B neon_axpy_loop
neon_axpy_tail:
 CMP $0,R2
 BEQ neon_axpy_done
 FMOVS (R0),F0
 FMULS F3,F0,F0
 FMOVS (R1),F1
 FADDS F0,F1,F1
 FMOVS F1,(R1)
 ADD $4,R0
 ADD $4,R1
 SUB $1,R2,R2
 B neon_axpy_tail
neon_axpy_done: RET
