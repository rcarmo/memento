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

// func dotRowsSSE2(input,matrix,output []float32)
TEXT ·dotRowsSSE2(SB),NOSPLIT,$0-72
 MOVQ input_base+0(FP),R8
 MOVQ input_len+8(FP),R9
 MOVQ matrix_base+24(FP),SI
 MOVQ output_base+48(FP),DI
 MOVQ output_len+56(FP),CX
sse_dot_rows_next:
 TESTQ CX,CX
 JZ sse_dot_rows_done
 MOVQ R8,R10
 MOVQ R9,AX
 XORPS X0,X0
sse_dot_rows_loop:
 CMPQ AX,$4
 JL sse_dot_rows_reduce
 MOVUPS (R10),X1
 MOVUPS (SI),X2
 MULPS X2,X1
 ADDPS X1,X0
 ADDQ $16,R10
 ADDQ $16,SI
 SUBQ $4,AX
 JMP sse_dot_rows_loop
sse_dot_rows_reduce:
 MOVHLPS X0,X1
 ADDPS X1,X0
 MOVAPS X0,X1
 SHUFPS $1,X1,X1
 ADDSS X1,X0
sse_dot_rows_tail:
 TESTQ AX,AX
 JZ sse_dot_rows_store
 MOVSS (R10),X1
 MULSS (SI),X1
 ADDSS X1,X0
 ADDQ $4,R10
 ADDQ $4,SI
 DECQ AX
 JMP sse_dot_rows_tail
sse_dot_rows_store:
 MOVSS X0,(DI)
 ADDQ $4,DI
 DECQ CX
 JMP sse_dot_rows_next
sse_dot_rows_done:
 RET

// func dotRowsAVX2(input,matrix,output []float32)
// Four independent row accumulators share each input load. Every row keeps the
// same eight-lane accumulation, horizontal reduction and scalar tail order as dotAVX2.
TEXT ·dotRowsAVX2(SB),NOSPLIT,$0-72
 MOVQ input_base+0(FP),R8
 MOVQ input_len+8(FP),R9
 MOVQ matrix_base+24(FP),SI
 MOVQ output_base+48(FP),DI
 MOVQ output_len+56(FP),CX
 MOVQ R9,R11
 SHLQ $2,R11
avx_dot_rows_four:
 CMPQ CX,$4
 JL avx_dot_rows_one
 MOVQ R8,R10
 MOVQ R9,AX
 LEAQ 0(SI)(R11*1),R12
 LEAQ 0(R12)(R11*1),R13
 LEAQ 0(R13)(R11*1),R14
 VXORPS Y0,Y0,Y0
 VXORPS Y1,Y1,Y1
 VXORPS Y2,Y2,Y2
 VXORPS Y3,Y3,Y3
avx_dot_rows_four_loop:
 CMPQ AX,$8
 JL avx_dot_rows_four_reduce
 VMOVUPS (R10),Y4
 VMOVUPS (SI),Y5
 VMULPS Y4,Y5,Y5
 VADDPS Y5,Y0,Y0
 VMOVUPS (R12),Y5
 VMULPS Y4,Y5,Y5
 VADDPS Y5,Y1,Y1
 VMOVUPS (R13),Y5
 VMULPS Y4,Y5,Y5
 VADDPS Y5,Y2,Y2
 VMOVUPS (R14),Y5
 VMULPS Y4,Y5,Y5
 VADDPS Y5,Y3,Y3
 ADDQ $32,R10
 ADDQ $32,SI
 ADDQ $32,R12
 ADDQ $32,R13
 ADDQ $32,R14
 SUBQ $8,AX
 JMP avx_dot_rows_four_loop
avx_dot_rows_four_reduce:
 VEXTRACTF128 $1,Y0,X4
 VADDPS X4,X0,X0
 VHADDPS X0,X0,X0
 VHADDPS X0,X0,X0
 VEXTRACTF128 $1,Y1,X4
 VADDPS X4,X1,X1
 VHADDPS X1,X1,X1
 VHADDPS X1,X1,X1
 VEXTRACTF128 $1,Y2,X4
 VADDPS X4,X2,X2
 VHADDPS X2,X2,X2
 VHADDPS X2,X2,X2
 VEXTRACTF128 $1,Y3,X4
 VADDPS X4,X3,X3
 VHADDPS X3,X3,X3
 VHADDPS X3,X3,X3
avx_dot_rows_four_tail:
 TESTQ AX,AX
 JZ avx_dot_rows_four_store
 VMOVSS (R10),X4
 VMOVSS (SI),X5
 VMULSS X4,X5,X5
 VADDSS X5,X0,X0
 VMOVSS (R12),X5
 VMULSS X4,X5,X5
 VADDSS X5,X1,X1
 VMOVSS (R13),X5
 VMULSS X4,X5,X5
 VADDSS X5,X2,X2
 VMOVSS (R14),X5
 VMULSS X4,X5,X5
 VADDSS X5,X3,X3
 ADDQ $4,R10
 ADDQ $4,SI
 ADDQ $4,R12
 ADDQ $4,R13
 ADDQ $4,R14
 DECQ AX
 JMP avx_dot_rows_four_tail
avx_dot_rows_four_store:
 VMOVSS X0,0(DI)
 VMOVSS X1,4(DI)
 VMOVSS X2,8(DI)
 VMOVSS X3,12(DI)
 MOVQ R14,SI
 ADDQ $16,DI
 SUBQ $4,CX
 JMP avx_dot_rows_four
avx_dot_rows_one:
 TESTQ CX,CX
 JZ avx_dot_rows_done
 MOVQ R8,R10
 MOVQ R9,AX
 VXORPS Y0,Y0,Y0
avx_dot_rows_one_loop:
 CMPQ AX,$8
 JL avx_dot_rows_one_reduce
 VMOVUPS (R10),Y1
 VMULPS (SI),Y1,Y1
 VADDPS Y1,Y0,Y0
 ADDQ $32,R10
 ADDQ $32,SI
 SUBQ $8,AX
 JMP avx_dot_rows_one_loop
avx_dot_rows_one_reduce:
 VEXTRACTF128 $1,Y0,X1
 VADDPS X1,X0,X0
 VHADDPS X0,X0,X0
 VHADDPS X0,X0,X0
avx_dot_rows_one_tail:
 TESTQ AX,AX
 JZ avx_dot_rows_one_store
 VMOVSS (R10),X1
 VMULSS (SI),X1,X1
 VADDSS X1,X0,X0
 ADDQ $4,R10
 ADDQ $4,SI
 DECQ AX
 JMP avx_dot_rows_one_tail
avx_dot_rows_one_store:
 VMOVSS X0,(DI)
 ADDQ $4,DI
 DECQ CX
 JMP avx_dot_rows_one
avx_dot_rows_done:
 VZEROUPPER
 RET

// func axpyRowsSSE2(coefficients,matrix,output []float32)
TEXT ·axpyRowsSSE2(SB),NOSPLIT,$0-72
 MOVQ coefficients_base+0(FP),R8
 MOVQ coefficients_len+8(FP),CX
 MOVQ matrix_base+24(FP),SI
 MOVQ output_base+48(FP),R9
 MOVQ output_len+56(FP),R10
sse_axpy_rows_next:
 TESTQ CX,CX
 JZ sse_axpy_rows_done
 MOVSS (R8),X3
 SHUFPS $0,X3,X3
 MOVQ R9,DI
 MOVQ R10,AX
sse_axpy_rows_loop:
 CMPQ AX,$4
 JL sse_axpy_rows_tail
 MOVUPS (SI),X0
 MULPS X3,X0
 MOVUPS (DI),X1
 ADDPS X0,X1
 MOVUPS X1,(DI)
 ADDQ $16,SI
 ADDQ $16,DI
 SUBQ $4,AX
 JMP sse_axpy_rows_loop
sse_axpy_rows_tail:
 TESTQ AX,AX
 JZ sse_axpy_rows_advance
 MOVSS (SI),X0
 MULSS X3,X0
 ADDSS (DI),X0
 MOVSS X0,(DI)
 ADDQ $4,SI
 ADDQ $4,DI
 DECQ AX
 JMP sse_axpy_rows_tail
sse_axpy_rows_advance:
 ADDQ $4,R8
 DECQ CX
 JMP sse_axpy_rows_next
sse_axpy_rows_done:
 RET

// func axpyRowsAVX2(coefficients,matrix,output []float32)
// Keep output tiles in registers while visiting coefficient rows in their
// original order. Separate multiply/add instructions preserve AXPY rounding.
TEXT ·axpyRowsAVX2(SB),NOSPLIT,$0-72
 MOVQ coefficients_base+0(FP),R8
 MOVQ coefficients_len+8(FP),R9
 MOVQ matrix_base+24(FP),R10
 MOVQ output_base+48(FP),R11
 MOVQ output_len+56(FP),R12
 MOVQ R12,R13
 SHLQ $2,R13
avx_axpy_rows_tile32:
 CMPQ R12,$32
 JL avx_axpy_rows_tile8
 VMOVUPS 0(R11),Y0
 VMOVUPS 32(R11),Y1
 VMOVUPS 64(R11),Y2
 VMOVUPS 96(R11),Y3
 MOVQ R8,BX
 MOVQ R9,CX
 MOVQ R10,SI
avx_axpy_rows_tile32_rows:
 TESTQ CX,CX
 JZ avx_axpy_rows_tile32_store
 VBROADCASTSS (BX),Y4
 VMOVUPS 0(SI),Y5
 VMULPS Y4,Y5,Y5
 VADDPS Y5,Y0,Y0
 VMOVUPS 32(SI),Y5
 VMULPS Y4,Y5,Y5
 VADDPS Y5,Y1,Y1
 VMOVUPS 64(SI),Y5
 VMULPS Y4,Y5,Y5
 VADDPS Y5,Y2,Y2
 VMOVUPS 96(SI),Y5
 VMULPS Y4,Y5,Y5
 VADDPS Y5,Y3,Y3
 ADDQ $4,BX
 ADDQ R13,SI
 DECQ CX
 JMP avx_axpy_rows_tile32_rows
avx_axpy_rows_tile32_store:
 VMOVUPS Y0,0(R11)
 VMOVUPS Y1,32(R11)
 VMOVUPS Y2,64(R11)
 VMOVUPS Y3,96(R11)
 ADDQ $128,R10
 ADDQ $128,R11
 SUBQ $32,R12
 JMP avx_axpy_rows_tile32
avx_axpy_rows_tile8:
 CMPQ R12,$8
 JL avx_axpy_rows_scalar
 VMOVUPS (R11),Y0
 MOVQ R8,BX
 MOVQ R9,CX
 MOVQ R10,SI
avx_axpy_rows_tile8_rows:
 TESTQ CX,CX
 JZ avx_axpy_rows_tile8_store
 VBROADCASTSS (BX),Y4
 VMOVUPS (SI),Y5
 VMULPS Y4,Y5,Y5
 VADDPS Y5,Y0,Y0
 ADDQ $4,BX
 ADDQ R13,SI
 DECQ CX
 JMP avx_axpy_rows_tile8_rows
avx_axpy_rows_tile8_store:
 VMOVUPS Y0,(R11)
 ADDQ $32,R10
 ADDQ $32,R11
 SUBQ $8,R12
 JMP avx_axpy_rows_tile8
avx_axpy_rows_scalar:
 TESTQ R12,R12
 JZ avx_axpy_rows_done
 VMOVSS (R11),X0
 MOVQ R8,BX
 MOVQ R9,CX
 MOVQ R10,SI
avx_axpy_rows_scalar_rows:
 TESTQ CX,CX
 JZ avx_axpy_rows_scalar_store
 VMOVSS (BX),X3
 VMOVSS (SI),X4
 VMULSS X3,X4,X4
 VADDSS X4,X0,X0
 ADDQ $4,BX
 ADDQ R13,SI
 DECQ CX
 JMP avx_axpy_rows_scalar_rows
avx_axpy_rows_scalar_store:
 VMOVSS X0,(R11)
 ADDQ $4,R10
 ADDQ $4,R11
 DECQ R12
 JMP avx_axpy_rows_scalar
avx_axpy_rows_done:
 VZEROUPPER
 RET
