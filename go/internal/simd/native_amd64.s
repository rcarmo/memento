#include "textflag.h"
// func dotSSE2(a,b []float32) float32
TEXT ·dotSSE2(SB),NOSPLIT,$0-52
 MOVQ a_base+0(FP),SI
 MOVQ a_len+8(FP),CX
 MOVQ b_base+24(FP),DI
 XORPS X0,X0
sse_dot_loop:
 CMPQ CX,$4
 JL sse_dot_reduce
 MOVUPS (SI),X1
 MOVUPS (DI),X2
 MULPS X2,X1
 ADDPS X1,X0
 ADDQ $16,SI
 ADDQ $16,DI
 SUBQ $4,CX
 JMP sse_dot_loop
sse_dot_reduce:
 MOVHLPS X0,X1
 ADDPS X1,X0
 MOVAPS X0,X1
 SHUFPS $1,X1,X1
 ADDSS X1,X0
sse_dot_tail:
 TESTQ CX,CX
 JZ sse_dot_done
 MOVSS (SI),X1
 MULSS (DI),X1
 ADDSS X1,X0
 ADDQ $4,SI
 ADDQ $4,DI
 DECQ CX
 JMP sse_dot_tail
sse_dot_done:
 MOVSS X0,ret+48(FP)
 RET

// func axpySSE2(alpha float32,x,y []float32)
TEXT ·axpySSE2(SB),NOSPLIT,$0-56
 MOVSS alpha+0(FP),X3
 SHUFPS $0,X3,X3
 MOVQ x_base+8(FP),SI
 MOVQ x_len+16(FP),CX
 MOVQ y_base+32(FP),DI
sse_axpy_loop:
 CMPQ CX,$4
 JL sse_axpy_tail
 MOVUPS (SI),X0
 MULPS X3,X0
 MOVUPS (DI),X1
 ADDPS X0,X1
 MOVUPS X1,(DI)
 ADDQ $16,SI
 ADDQ $16,DI
 SUBQ $4,CX
 JMP sse_axpy_loop
sse_axpy_tail:
 TESTQ CX,CX
 JZ sse_axpy_done
 MOVSS (SI),X0
 MULSS X3,X0
 ADDSS (DI),X0
 MOVSS X0,(DI)
 ADDQ $4,SI
 ADDQ $4,DI
 DECQ CX
 JMP sse_axpy_tail
sse_axpy_done: RET

// func dotAVX2(a,b []float32) float32
TEXT ·dotAVX2(SB),NOSPLIT,$0-52
 MOVQ a_base+0(FP),SI
 MOVQ a_len+8(FP),CX
 MOVQ b_base+24(FP),DI
 VXORPS Y0,Y0,Y0
avx_dot_loop:
 CMPQ CX,$8
 JL avx_dot_reduce
 VMOVUPS (SI),Y1
 VMULPS (DI),Y1,Y1
 VADDPS Y1,Y0,Y0
 ADDQ $32,SI
 ADDQ $32,DI
 SUBQ $8,CX
 JMP avx_dot_loop
avx_dot_reduce:
 VEXTRACTF128 $1,Y0,X1
 VADDPS X1,X0,X0
 VHADDPS X0,X0,X0
 VHADDPS X0,X0,X0
avx_dot_tail:
 TESTQ CX,CX
 JZ avx_dot_done
 VMOVSS (SI),X1
 VMULSS (DI),X1,X1
 VADDSS X1,X0,X0
 ADDQ $4,SI
 ADDQ $4,DI
 DECQ CX
 JMP avx_dot_tail
avx_dot_done:
 VMOVSS X0,ret+48(FP)
 VZEROUPPER
 RET

// func axpyAVX2(alpha float32,x,y []float32)
TEXT ·axpyAVX2(SB),NOSPLIT,$0-56
 VBROADCASTSS alpha+0(FP),Y3
 MOVQ x_base+8(FP),SI
 MOVQ x_len+16(FP),CX
 MOVQ y_base+32(FP),DI
avx_axpy_loop:
 CMPQ CX,$8
 JL avx_axpy_tail
 VMOVUPS (SI),Y0
 VMULPS Y3,Y0,Y0
 VADDPS (DI),Y0,Y0
 VMOVUPS Y0,(DI)
 ADDQ $32,SI
 ADDQ $32,DI
 SUBQ $8,CX
 JMP avx_axpy_loop
avx_axpy_tail:
 TESTQ CX,CX
 JZ avx_axpy_done
 VMOVSS (SI),X0
 VMULSS X3,X0,X0
 VADDSS (DI),X0,X0
 VMOVSS X0,(DI)
 ADDQ $4,SI
 ADDQ $4,DI
 DECQ CX
 JMP avx_axpy_tail
avx_axpy_done: VZEROUPPER
RET
