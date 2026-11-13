// Copyright 2026 Rui Carmo. MIT.
// Leaf kernels: packed 32-value Q5_K group, independent FP32 multiply/subtract.
#include "textflag.h"

// q is one packed low-nibble group. high is the beginning of the Q5_K
// high-bit plane; highMask identifies this group's bit in each high byte.
TEXT ·q5GroupSSE2(SB), NOSPLIT, $0-44
	MOVQ q+0(FP), SI
	MOVQ high+8(FP), BX
	MOVQ x+16(FP), DI
	MOVSS scale+24(FP), X4
	SHUFPS $0, X4, X4
	MOVSS minimum+28(FP), X5
	SHUFPS $0, X5, X5
	PCMPEQL X6, X6
	PSRLL $28, X6
	PXOR X7, X7
	PXOR X8, X8
	MOVL $16, AX
	MOVD AX, X9
	SHUFPS $0, X9, X9
	MOVL highMask+36(FP), AX
	MOVD AX, X10
	SHUFPS $0, X10, X10
	PCMPEQL X11, X11
	MOVQ $8, CX
q5s_loop:
	MOVL (SI), AX
	MOVD AX, X0
	CMPB highNibble+32(FP), $0
	JE q5s_low
	PSRLW $4, X0
q5s_low:
	PUNPCKLBW X7, X0
	PUNPCKLWL X7, X0
	PAND X6, X0
	MOVL (BX), AX
	MOVD AX, X1
	PUNPCKLBW X7, X1
	PUNPCKLWL X7, X1
	PAND X10, X1
	PCMPEQL X7, X1
	PXOR X11, X1
	PAND X9, X1
	POR X1, X0
	CVTPL2PS X0, X0
	MULPS X4, X0
	SUBPS X5, X0
	MOVUPS (DI), X1
	MULPS X1, X0
	ADDPS X0, X8
	ADDQ $4, SI
	ADDQ $4, BX
	ADDQ $16, DI
	DECQ CX
	JNZ q5s_loop
	MOVAPS X8, X0
	MOVAPS X0, X1
	SHUFPS $0x4e, X1, X1
	ADDPS X1, X0
	MOVAPS X0, X1
	SHUFPS $0xb1, X1, X1
	ADDPS X1, X0
	MOVSS X0, ret+40(FP)
	RET

TEXT ·q5GroupAVX2(SB), NOSPLIT, $0-44
	MOVQ q+0(FP), SI
	MOVQ high+8(FP), BX
	MOVQ x+16(FP), DI
	// Do legacy MOVD setup before touching a Y register: on Intel CPUs an
	// SSE write while upper YMM state is live incurs an AVX/SSE transition.
	MOVL $16, AX
	MOVD AX, X9
	MOVL highMask+36(FP), AX
	MOVD AX, X10
	VBROADCASTSS scale+24(FP), Y4
	VBROADCASTSS minimum+28(FP), Y5
	VPCMPEQD Y6, Y6, Y6
	VPSRLD $28, Y6, Y6
	VXORPS Y7, Y7, Y7
	VXORPS Y8, Y8, Y8
	VPBROADCASTD X9, Y9
	VPBROADCASTD X10, Y10
	VPCMPEQD Y11, Y11, Y11
	MOVQ $4, CX
q5a_loop:
	VPMOVZXBD (SI), Y0
	CMPB highNibble+32(FP), $0
	JE q5a_low
	VPSRLD $4, Y0, Y0
q5a_low:
	VPAND Y6, Y0, Y0
	VPMOVZXBD (BX), Y1
	VPAND Y10, Y1, Y1
	VPCMPEQD Y8, Y1, Y1
	VPXOR Y11, Y1, Y1
	VPAND Y9, Y1, Y1
	VPOR Y1, Y0, Y0
	VCVTDQ2PS Y0, Y0
	VMULPS Y4, Y0, Y0
	VSUBPS Y5, Y0, Y0
	VMULPS (DI), Y0, Y0
	VADDPS Y0, Y7, Y7
	ADDQ $8, SI
	ADDQ $8, BX
	ADDQ $32, DI
	DECQ CX
	JNZ q5a_loop
	VEXTRACTF128 $1, Y7, X1
	VADDPS X1, X7, X0
	VHADDPS X0, X0, X0
	VHADDPS X0, X0, X0
	VMOVSS X0, ret+40(FP)
	VZEROUPPER
	RET
