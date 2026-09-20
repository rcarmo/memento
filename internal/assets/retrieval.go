package assets

import (
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/rcarmo/memento/internal/pyjson"
	"golang.org/x/text/encoding/charmap"
)

const (
	DefaultChunkBytes     = 65536
	MaxChunkBytes         = 262144
	MaxInlineArchiveBytes = 16384
	MaxMetadataBytes      = 2 * 1024 * 1024
)

type ReadError struct{ Message string }

func (e *ReadError) Error() string   { return e.Message }
func readError(message string) error { return &ReadError{message} }
func RetrievalLimits() map[string]int {
	return map[string]int{"max_upload_zip_bytes": MaxZIPBytes, "max_archive_bytes": MaxZIPBytes, "max_uncompressed_bytes": MaxUncompressedBytes, "max_file_bytes": MaxFileBytes, "max_file_count": MaxFileCount, "default_chunk_bytes": DefaultChunkBytes, "max_chunk_bytes": MaxChunkBytes, "max_inline_archive_bytes": MaxInlineArchiveBytes, "max_metadata_bytes": MaxMetadataBytes}
}
func ValidateRange(offset, limit int64, total *int64) error {
	if offset < 0 {
		return readError("asset offset must be a nonnegative integer")
	}
	if limit < 1 || limit > MaxChunkBytes {
		return readError("asset limit must be between 1 and 262144")
	}
	if total != nil && offset > *total {
		return readError("asset offset exceeds total bytes")
	}
	return nil
}
func ValidateFilePath(path string) error {
	if path == "" || strings.Contains(path, `\`) {
		return readError("asset file_path must be a canonical manifest-relative path")
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." {
			return readError("asset file_path must be a canonical manifest-relative path")
		}
	}
	for _, r := range path {
		if r < 32 || r == 127 {
			return readError("asset file_path must be a canonical manifest-relative path")
		}
	}
	return nil
}

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func CheckedManifest(manifest Manifest, digest string) error {
	if !digestPattern.MatchString(digest) {
		return readError("invalid ZIP digest in asset metadata")
	}
	if manifest.SHA256 != digest {
		return readError("asset manifest/ZIP digest mismatch")
	}
	if manifest.FileCount != len(manifest.Entries) || manifest.FileCount < 1 || manifest.FileCount > MaxFileCount {
		return readError("invalid asset manifest file count")
	}
	seen := map[string]bool{}
	total := uint64(0)
	for _, entry := range manifest.Entries {
		if err := ValidateFilePath(entry.Path); err != nil {
			return err
		}
		if seen[entry.Path] || entry.Size > MaxFileBytes {
			return readError("invalid or duplicate asset manifest entry")
		}
		if !digestPattern.MatchString(entry.SHA256) {
			return readError("invalid asset file digest")
		}
		seen[entry.Path] = true
		total += entry.Size
	}
	if total != manifest.TotalUncompressedBytes || total > MaxUncompressedBytes {
		return readError("invalid asset manifest uncompressed size")
	}
	return nil
}
func ManifestDigest(manifest Manifest) string {
	// Fixed typed structure is always JSON-encodable and below the nesting cap.
	// json.dumps(sort_keys=True,separators=(',',':')) retains ASCII escaping.
	value, _ := pyjson.Parse(manifestJSON(manifest))
	raw, _ := pyjson.Dumps(value)
	var compact bytes.Buffer
	_ = json.Compact(&compact, []byte(raw))
	return fmt.Sprintf("%x", sha256.Sum256(compact.Bytes()))
}
func CheckExpectedDigest(expected *string, actual string) error {
	if expected != nil && (!digestPattern.MatchString(*expected) || *expected != actual) {
		return readError("asset digest mismatch; pin the resolved version and ZIP SHA-256")
	}
	return nil
}
func ValidateArchiveSize(total int64) error {
	if total <= 0 || total > MaxZIPBytes {
		return readError("invalid asset ZIP size")
	}
	return nil
}

// VerifiedSlice hashes the complete stream while retaining at most limit bytes.
// An offset at EOF is valid. Short/long streams and bad digests fail without
// returning unverified bytes; the caller owns and closes the stream.
func VerifiedSlice(stream io.Reader, total int64, digest string, offset, limit int64) ([]byte, error) {
	if err := ValidateRange(offset, limit, &total); err != nil {
		return nil, err
	}
	hash := sha256.New()
	result := []byte{}
	consumed := int64(0)
	buffer := make([]byte, 65536)
	for {
		n, err := stream.Read(buffer)
		if n > 0 {
			end := consumed + int64(n)
			if end > total {
				return nil, readError("asset size differs from metadata")
			}
			_, _ = hash.Write(buffer[:n])
			startSlice := max(offset, consumed)
			endSlice := end
			if offset <= total && limit <= total-offset {
				endSlice = min(offset+limit, end)
			}
			if endSlice > startSlice {
				result = append(result, buffer[startSlice-consumed:endSlice-consumed]...)
			}
			consumed = end
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	if consumed != total || fmt.Sprintf("%x", hash.Sum(nil)) != digest {
		return nil, readError("asset digest/size mismatch")
	}
	return result, nil
}
func RangePayload(content []byte, offset, total int64) map[string]any {
	next := offset + int64(len(content))
	var nextOffset any
	if next < total {
		nextOffset = next
	}
	return map[string]any{"offset": offset, "returned_bytes": len(content), "total_bytes": total, "truncated": offset > 0 || next < total, "next_offset": nextOffset, "content_sha256": fmt.Sprintf("%x", sha256.Sum256(content))}
}

type FileNotDeclaredError struct{}

func (*FileNotDeclaredError) Error() string { return "asset file is not declared in the manifest" }

// ReadPackFile validates all directory entries against the supplied manifest,
// then hashes the selected full file before returning a bounded slice. Callers
// must separately pin/hash the entire archive if archive identity is required.
func ReadPackFile(stream io.ReaderAt, size int64, manifest Manifest, filePath string, offset, limit int64) (map[string]any, error) {
	if err := ValidateFilePath(filePath); err != nil {
		return nil, err
	}
	entries := map[string]ManifestEntry{}
	for _, entry := range manifest.Entries {
		entries[entry.Path] = entry
	}
	entry, ok := entries[filePath]
	if !ok {
		return nil, &FileNotDeclaredError{}
	}
	total := int64(entry.Size)
	if err := ValidateRange(offset, limit, &total); err != nil {
		return nil, err
	}
	archive, err := zip.NewReader(stream, size)
	if err != nil {
		return nil, readError("asset ZIP validation failed")
	}
	archive.RegisterDecompressor(12, func(r io.Reader) io.ReadCloser { return io.NopCloser(bzip2.NewReader(r)) })
	if len(archive.File) > MaxEntryCount {
		return nil, readError("asset archive has too many entries")
	}
	all := map[string]bool{}
	declared := map[string]bool{}
	var selected *zip.File
	for _, file := range archive.File {
		name := file.Name
		if file.Flags&0x800 == 0 {
			name, _ = charmap.CodePage437.NewDecoder().String(name)
		}
		name, _, _ = strings.Cut(name, "\x00")
		rawName := name
		directory := strings.HasSuffix(name, "/")
		if directory {
			name = strings.TrimSuffix(name, "/")
		}
		if err = ValidateFilePath(name); err != nil {
			return nil, err
		}
		if all[rawName] || file.Flags&1 != 0 {
			return nil, readError("duplicate or encrypted ZIP entry")
		}
		all[rawName] = true
		mode := (file.ExternalAttrs >> 16) & 0170000
		if directory {
			if mode != 0 && mode != 0040000 {
				return nil, readError("unsafe ZIP directory entry")
			}
			continue
		}
		if mode != 0 && mode != 0100000 {
			return nil, readError("symlink or special ZIP entry")
		}
		item, ok := entries[name]
		if !ok || file.UncompressedSize64 != item.Size {
			return nil, readError("ZIP entries differ from the manifest")
		}
		declared[name] = true
		if name == filePath {
			selected = file
		}
	}
	if len(declared) != len(entries) {
		return nil, readError("ZIP is missing manifest entries")
	}
	source, err := selected.Open()
	if err != nil {
		return nil, readError("asset ZIP validation failed")
	}
	defer source.Close()
	checksum := crc32.NewIEEE()
	content, err := VerifiedSlice(io.TeeReader(source, checksum), total, entry.SHA256, offset, limit)
	if err != nil {
		var assetError *ReadError
		if errors.As(err, &assetError) {
			return nil, err
		}
		return nil, readError("asset ZIP validation failed")
	}
	if checksum.Sum32() != selected.CRC32 {
		return nil, readError("asset ZIP validation failed")
	}
	result := RangePayload(content, offset, total)
	result["path"] = filePath
	result["media_type"] = entry.MediaType
	result["sha256"] = entry.SHA256
	if utf8.Valid(content) {
		result["encoding"] = "utf-8"
		result["content"] = string(content)
	} else {
		result["encoding"] = "base64"
		result["content"] = base64.StdEncoding.EncodeToString(content)
	}
	return result, nil
}
