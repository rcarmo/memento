#ifndef MEMENTO_NEEDLE_FFI_H
#define MEMENTO_NEEDLE_FFI_H

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define MEMENTO_NEEDLE_FFI_ABI_VERSION 1u
#define MEMENTO_NEEDLE_FFI_FEATURE_GENERATE 0x00000001ull
#define MEMENTO_NEEDLE_FFI_FEATURE_CANCELLATION 0x00000002ull

typedef struct MementoNeedleFfiRouterHandle MementoNeedleFfiRouterHandle;
typedef struct MementoNeedleFfiCancelToken MementoNeedleFfiCancelToken;

typedef enum MementoNeedleFfiStatus {
  MEMENTO_NEEDLE_FFI_STATUS_OK = 0,
  MEMENTO_NEEDLE_FFI_STATUS_NULL = 1,
  MEMENTO_NEEDLE_FFI_STATUS_BOUNDS = 2,
  MEMENTO_NEEDLE_FFI_STATUS_UTF8 = 3,
  MEMENTO_NEEDLE_FFI_STATUS_CANCELLED = 4,
  MEMENTO_NEEDLE_FFI_STATUS_MODEL = 5,
  MEMENTO_NEEDLE_FFI_STATUS_IO = 6,
  MEMENTO_NEEDLE_FFI_STATUS_LOCK = 7,
  MEMENTO_NEEDLE_FFI_STATUS_PANIC = 8
} MementoNeedleFfiStatus;

typedef struct MementoNeedleFfiStringView {
  const uint8_t *ptr;
  size_t len;
} MementoNeedleFfiStringView;

typedef struct MementoNeedleFfiRouterInfo {
  uint32_t abi_version;
  size_t d_model;
  size_t vocab_size;
  size_t num_encoder_layers;
  size_t num_decoder_layers;
  size_t num_heads;
  size_t num_kv_heads;
  size_t max_seq_len;
} MementoNeedleFfiRouterInfo;

uint32_t memento_needle_ffi_abi_version(void);
uint64_t memento_needle_ffi_features(void);
MementoNeedleFfiStatus memento_needle_ffi_last_error_message(uint8_t *buffer, size_t buffer_len, size_t *out_required_len);
MementoNeedleFfiStatus memento_needle_ffi_router_load_paths(const uint8_t *model_path_ptr, size_t model_path_len, const uint8_t *tokenizer_path_ptr, size_t tokenizer_path_len, MementoNeedleFfiRouterHandle **out_handle);
MementoNeedleFfiStatus memento_needle_ffi_router_free(MementoNeedleFfiRouterHandle *handle);
MementoNeedleFfiStatus memento_needle_ffi_router_info(const MementoNeedleFfiRouterHandle *handle, MementoNeedleFfiRouterInfo *out_info);
MementoNeedleFfiStatus memento_needle_ffi_cancel_token_new(MementoNeedleFfiCancelToken **out_token);
MementoNeedleFfiStatus memento_needle_ffi_cancel_token_cancel(MementoNeedleFfiCancelToken *token);
MementoNeedleFfiStatus memento_needle_ffi_cancel_token_free(MementoNeedleFfiCancelToken *token);
MementoNeedleFfiStatus memento_needle_ffi_router_generate(const MementoNeedleFfiRouterHandle *handle, MementoNeedleFfiStringView query, MementoNeedleFfiStringView tools_json, size_t max_enc_len, size_t max_gen_len, bool constrained, const MementoNeedleFfiCancelToken *cancel_token, uint8_t *output_buffer, size_t *inout_output_len);

#ifdef __cplusplus
}
#endif

#endif
