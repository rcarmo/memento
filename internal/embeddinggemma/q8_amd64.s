// Copyright 2026 Rui Carmo. MIT.
// Whole-row Q8_0 fused dot kernels. They load exactly the 32 signed bytes and
// 32 float32 activations in every block.
#include "textflag.h"

// func q8DotSSE2(raw *byte, x, halfTable *float32, blocks int) float32
TEXT ·q8DotSSE2(SB), NOSPLIT, $0-36
	MOVQ raw+0(FP), SI
	MOVQ x+8(FP), DI
	MOVQ halfTable+16(FP), BX
	MOVQ blocks+24(FP), CX
	PXOR X7, X7
	PXOR X8, X8
q8s_block:
	MOVWLZX (SI), AX
	MOVSS (BX)(AX*4), X4
	SHUFPS $0, X4, X4
	LEAQ 2(SI), DX
	MOVQ $8, R8
q8s_values:
	MOVL (DX), AX
	MOVD AX, X0
	PUNPCKLBW X7, X0
	PUNPCKLWL X7, X0
	PSLLL $24, X0
	PSRAL $24, X0
	CVTPL2PS X0, X0
	MULPS X4, X0
	MOVUPS (DI), X1
	MULPS X1, X0
	ADDPS X0, X8
	ADDQ $4, DX
	ADDQ $16, DI
	DECQ R8
	JNZ q8s_values
	ADDQ $34, SI
	DECQ CX
	JNZ q8s_block
	MOVAPS X8, X0
	MOVAPS X0, X1
	SHUFPS $0x4e, X1, X1
	ADDPS X1, X0
	MOVAPS X0, X1
	SHUFPS $0xb1, X1, X1
	ADDPS X1, X0
	MOVSS X0, ret+32(FP)
	RET

// func q8DotAVX2(raw *byte, x, halfTable *float32, blocks int) float32
TEXT ·q8DotAVX2(SB), NOSPLIT, $0-36
	MOVQ raw+0(FP), SI
	MOVQ x+8(FP), DI
	MOVQ halfTable+16(FP), BX
	MOVQ blocks+24(FP), CX
	VXORPS Y7, Y7, Y7
q8a_block:
	MOVWLZX (SI), AX
	VBROADCASTSS (BX)(AX*4), Y4
	LEAQ 2(SI), DX
	MOVQ $4, R8
q8a_values:
	VPMOVSXBD (DX), Y0
	VCVTDQ2PS Y0, Y0
	VMULPS Y4, Y0, Y0
	VMULPS (DI), Y0, Y0
	VADDPS Y0, Y7, Y7
	ADDQ $8, DX
	ADDQ $32, DI
	DECQ R8
	JNZ q8a_values
	ADDQ $34, SI
	DECQ CX
	JNZ q8a_block
	VEXTRACTF128 $1, Y7, X1
	VADDPS X1, X7, X0
	VHADDPS X0, X0, X0
	VHADDPS X0, X0, X0
	VMOVSS X0, ret+32(FP)
	VZEROUPPER
	RET
