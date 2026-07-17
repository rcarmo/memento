#ifndef MEMENTO_FFI_H
#define MEMENTO_FFI_H

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define MEMENTO_FFI_ABI_VERSION 1u
#define MEMENTO_FFI_FEATURE_EMBED 0x00000001ull
#define MEMENTO_FFI_FEATURE_EMBED_BATCH 0x00000002ull
#define MEMENTO_FFI_FEATURE_CANCELLATION 0x00000004ull
#define MEMENTO_FFI_FEATURE_VECTOR_COSINE 0x00000008ull
#define MEMENTO_FFI_FEATURE_VECTOR_VALIDATE 0x00000010ull

typedef struct MementoFfiModelHandle MementoFfiModelHandle;
typedef struct MementoFfiCancelToken MementoFfiCancelToken;

typedef enum MementoFfiStatus {
  MEMENTO_FFI_STATUS_OK = 0,
  MEMENTO_FFI_STATUS_NULL = 1,
  MEMENTO_FFI_STATUS_BOUNDS = 2,
  MEMENTO_FFI_STATUS_UTF8 = 3,
  MEMENTO_FFI_STATUS_FINITE = 4,
  MEMENTO_FFI_STATUS_CANCELLED = 5,
  MEMENTO_FFI_STATUS_MODEL = 6,
  MEMENTO_FFI_STATUS_VECTOR = 7,
  MEMENTO_FFI_STATUS_IO = 8,
  MEMENTO_FFI_STATUS_PANIC = 9
} MementoFfiStatus;

typedef struct MementoFfiStringView {
  const uint8_t *ptr;
  size_t len;
} MementoFfiStringView;

typedef struct MementoFfiModelInfo {
  uint32_t abi_version;
  size_t hidden_size;
  size_t vocab_size;
  size_t num_layers;
  size_t num_heads;
  size_t intermediate_size;
  size_t max_seq_len;
} MementoFfiModelInfo;

uint32_t memento_ffi_abi_version(void);
uint64_t memento_ffi_features(void);
MementoFfiStatus memento_ffi_last_error_message(uint8_t *buffer, size_t buffer_len, size_t *out_required_len);
MementoFfiStatus memento_ffi_model_load_path(const uint8_t *path_ptr, size_t path_len, MementoFfiModelHandle **out_handle);
MementoFfiStatus memento_ffi_model_free(MementoFfiModelHandle *handle);
MementoFfiStatus memento_ffi_model_info(const MementoFfiModelHandle *handle, MementoFfiModelInfo *out_info);
MementoFfiStatus memento_ffi_cancel_token_new(MementoFfiCancelToken **out_token);
MementoFfiStatus memento_ffi_cancel_token_cancel(MementoFfiCancelToken *token);
MementoFfiStatus memento_ffi_cancel_token_free(MementoFfiCancelToken *token);
MementoFfiStatus memento_ffi_embed(const MementoFfiModelHandle *handle, MementoFfiStringView text, const MementoFfiCancelToken *cancel_token, size_t max_chars, float *out_embedding, size_t out_embedding_len);
MementoFfiStatus memento_ffi_embed_batch(const MementoFfiModelHandle *handle, const MementoFfiStringView *texts_ptr, size_t texts_len, const MementoFfiCancelToken *cancel_token, size_t max_batch, size_t max_chars_per_input, float *out_embeddings, size_t out_embeddings_len);
MementoFfiStatus memento_ffi_vector_validate(const float *values_ptr, size_t values_len);
MementoFfiStatus memento_ffi_vector_cosine(const float *left_ptr, size_t left_len, const float *right_ptr, size_t right_len, float *out_cosine);

#ifdef __cplusplus
}
#endif

#endif
