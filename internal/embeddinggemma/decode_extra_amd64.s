// Copyright 2026 Rui Carmo. MIT.
// Whole-row AVX2 decoders into bounded caller-owned FP32 scratch.
#include "textflag.h"

// Q5_K shares Q4_K scale packing. Decode both 32-value nibble planes
// together, reading the full 32-byte high plane for every group pair.
// Separate FP32 scale multiply, value multiply and subtract (never FMA).
TEXT ·q5DecodeAVX2(SB), NOSPLIT, $0-32
	MOVQ raw+0(FP), SI
	MOVQ dst+8(FP), DI
	MOVQ blocks+16(FP), CX
	MOVQ halfTable+24(FP), R8
	VPCMPEQD Y6, Y6, Y6
	VPSRLD $28, Y6, Y6
	MOVL $16, AX
	VMOVD AX, X7
	VPBROADCASTD X7, Y7
	TESTQ CX, CX
	JZ q5d_done
q5d_block:
	MOVWLZX (SI), AX
	VBROADCASTSS (R8)(AX*4), Y8
	MOVWLZX 2(SI), AX
	VBROADCASTSS (R8)(AX*4), Y9
	LEAQ 48(SI), R9
	XORQ R10, R10
q5d_pair:
	CMPQ R10, $4
	JGE q5d_upper_scales
	MOVBLZX 4(SI)(R10*1), AX
	ANDL $63, AX
	MOVBLZX 8(SI)(R10*1), BX
	ANDL $63, BX
	MOVBLZX 5(SI)(R10*1), R11
	ANDL $63, R11
	MOVBLZX 9(SI)(R10*1), R12
	ANDL $63, R12
	JMP q5d_scales_ready
q5d_upper_scales:
	MOVBLZX 8(SI)(R10*1), AX
	ANDL $15, AX
	MOVBLZX (SI)(R10*1), DX
	SHRL $6, DX
	SHLL $4, DX
	ORL DX, AX
	MOVBLZX 8(SI)(R10*1), BX
	SHRL $4, BX
	MOVBLZX 4(SI)(R10*1), DX
	SHRL $6, DX
	SHLL $4, DX
	ORL DX, BX
	MOVBLZX 9(SI)(R10*1), R11
	ANDL $15, R11
	MOVBLZX 1(SI)(R10*1), DX
	SHRL $6, DX
	SHLL $4, DX
	ORL DX, R11
	MOVBLZX 9(SI)(R10*1), R12
	SHRL $4, R12
	MOVBLZX 5(SI)(R10*1), DX
	SHRL $6, DX
	SHLL $4, DX
	ORL DX, R12
q5d_scales_ready:
	VMOVD AX, X0
	VCVTDQ2PS X0, X0
	VBROADCASTSS X0, Y2
	VMULPS Y8, Y2, Y2
	VMOVD BX, X0
	VCVTDQ2PS X0, X0
	VBROADCASTSS X0, Y3
	VMULPS Y9, Y3, Y3
	VMOVD R11, X0
	VCVTDQ2PS X0, X0
	VBROADCASTSS X0, Y4
	VMULPS Y8, Y4, Y4
	VMOVD R12, X0
	VCVTDQ2PS X0, X0
	VBROADCASTSS X0, Y5
	VMULPS Y9, Y5, Y5
	VMOVD R10, X11
	VPBROADCASTD X11, Y11
	XORQ DX, DX
q5d_values:
	VPMOVZXBD (R9)(DX*1), Y0
	VPSRLD $4, Y0, Y1
	VPAND Y6, Y0, Y0
	// Each high byte holds all eight group bits for this same position.
	// Shift by g (0,2,4,6); extract adjacent bits into each nibble plane.
	VPMOVZXBD 16(SI)(DX*1), Y10
	VPSRLVD Y11, Y10, Y10
	VPSLLD $4, Y10, Y12
	VPSLLD $3, Y10, Y13
	VPAND Y7, Y12, Y12
	VPAND Y7, Y13, Y13
	VPOR Y12, Y0, Y0
	VPOR Y13, Y1, Y1
	VCVTDQ2PS Y0, Y0
	VMULPS Y2, Y0, Y0
	VSUBPS Y3, Y0, Y0
	VCVTDQ2PS Y1, Y1
	VMULPS Y4, Y1, Y1
	VSUBPS Y5, Y1, Y1
	VMOVUPS Y0, (DI)(DX*4)
	VMOVUPS Y1, 128(DI)(DX*4)
	ADDQ $8, DX
	CMPQ DX, $32
	JL q5d_values
	ADDQ $32, R9
	ADDQ $256, DI
	ADDQ $2, R10
	CMPQ R10, $8
	JL q5d_pair
	ADDQ $176, SI
	DECQ CX
	JNZ q5d_block
q5d_done:
	VZEROUPPER
	RET

// Q8_0: signed byte expansion, exact FP32 half * int8, no FMA.
// Each 8-byte load remains inside its 34-byte block; stores cover 32 values.
TEXT ·q8DecodeAVX2(SB), NOSPLIT, $0-32
	MOVQ raw+0(FP), SI
	MOVQ dst+8(FP), DI
	MOVQ blocks+16(FP), CX
	MOVQ halfTable+24(FP), R8
	TESTQ CX, CX
	JZ q8d_done
q8d_block:
	MOVWLZX (SI), AX
	VBROADCASTSS (R8)(AX*4), Y4
	VPMOVSXBD 2(SI), Y0
	VPMOVSXBD 10(SI), Y1
	VPMOVSXBD 18(SI), Y2
	VPMOVSXBD 26(SI), Y3
	VCVTDQ2PS Y0, Y0
	VCVTDQ2PS Y1, Y1
	VCVTDQ2PS Y2, Y2
	VCVTDQ2PS Y3, Y3
	VMULPS Y4, Y0, Y0
	VMULPS Y4, Y1, Y1
	VMULPS Y4, Y2, Y2
	VMULPS Y4, Y3, Y3
	VMOVUPS Y0, (DI)
	VMOVUPS Y1, 32(DI)
	VMOVUPS Y2, 64(DI)
	VMOVUPS Y3, 96(DI)
	ADDQ $34, SI
	ADDQ $128, DI
	DECQ CX
	JNZ q8d_block
q8d_done:
	VZEROUPPER
	RET
