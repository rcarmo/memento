// Package assets validates untrusted ZIP bytes before staging. No archive
// member is extracted to the filesystem or executed during validation.
package assets

import (
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/encoding/charmap"
)

const (
	MaxZIPBytes          = 50 * 1024 * 1024
	MaxUncompressedBytes = 50 * 1024 * 1024
	MaxFileCount         = 512
	MaxEntryCount        = 1024
	MaxFileBytes         = 16 * 1024 * 1024
	MaxCompressionRatio  = 100
)

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }
func invalid(message string) error       { return &ValidationError{message} }

type ManifestEntry struct {
	Path      string `json:"path"`
	Size      uint64 `json:"size"`
	MediaType string `json:"media_type"`
	SHA256    string `json:"sha256"`
}
type Manifest struct {
	Entries                []ManifestEntry `json:"entries"`
	SHA256                 string          `json:"sha256"`
	TotalUncompressedBytes uint64          `json:"total_uncompressed_bytes"`
	FileCount              int             `json:"file_count"`
}
type ValidatedPack struct {
	SkillName, Version, SkillMD string
	ZIPBytes                    []byte
	Manifest                    Manifest
}

var kindPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func ValidateKind(kind string) error {
	if !kindPattern.MatchString(kind) {
		return invalid("asset_kind must be lowercase alphanumeric words joined by hyphens")
	}
	return nil
}
func ParseStableSemver(version string) ([3]*big.Int, error) {
	var result [3]*big.Int
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return result, invalid("version must be a stable semantic version")
	}
	for i, part := range parts {
		if part == "0" {
			result[i] = new(big.Int)
			continue
		}
		if part == "" || part[0] < '1' || part[0] > '9' {
			return result, invalid("version must be a stable semantic version")
		}
		var normalized strings.Builder
		for _, r := range part {
			digit, ok := decimalDigit(r)
			if !ok {
				return result, invalid("version must be a stable semantic version")
			}
			normalized.WriteByte('0' + digit)
		}
		result[i], _ = new(big.Int).SetString(normalized.String(), 10)
	}
	return result, nil
}
func decimalDigit(r rune) (byte, bool) {
	for _, block := range unicode.Digit.R16 {
		if uint32(r) >= uint32(block.Lo) && uint32(r) <= uint32(block.Hi) && (uint32(r)-uint32(block.Lo))%uint32(block.Stride) == 0 {
			return byte((uint32(r) - uint32(block.Lo)) / uint32(block.Stride) % 10), true
		}
	}
	for _, block := range unicode.Digit.R32 {
		if uint32(r) >= block.Lo && uint32(r) <= block.Hi && (uint32(r)-block.Lo)%block.Stride == 0 {
			return byte((uint32(r) - block.Lo) / block.Stride % 10), true
		}
	}
	return 0, false
}
func ValidateAssetPack(kind, version string, raw []byte, rootPath, rootText string) (ValidatedPack, error) {
	name := "asset-pack"
	if kind == "skill" {
		name = "skill-pack"
	}
	return validatePack(name, version, rootText, raw, rootPath)
}
func ValidateSkillPack(name, version, skillMD string, raw []byte) (ValidatedPack, error) {
	if !kindPattern.MatchString(name) {
		return ValidatedPack{}, invalid("skill_name must be lowercase alphanumeric words joined by hyphens")
	}
	return validatePack(name, version, skillMD, raw, "SKILL.md")
}
func validatePack(name, version, rootText string, raw []byte, required string) (ValidatedPack, error) {
	fail := func(err error) (ValidatedPack, error) { return ValidatedPack{}, err }
	if _, err := ParseStableSemver(version); err != nil {
		return fail(err)
	}
	if len(raw) > MaxZIPBytes {
		return fail(invalid("ZIP archive exceeds maximum encoded size"))
	}
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return fail(invalid("zip_bytes is not a valid ZIP archive"))
	}
	archive.RegisterDecompressor(12, func(r io.Reader) io.ReadCloser { return io.NopCloser(bzip2.NewReader(r)) })
	if len(archive.File) > MaxEntryCount {
		return fail(invalid("archive exceeds maximum entry count"))
	}
	manifest := Manifest{Entries: []ManifestEntry{}, SHA256: fmt.Sprintf("%x", sha256.Sum256(raw))}
	seen := map[string]bool{}
	found := false
	for _, file := range archive.File {
		rawPath := file.Name
		if file.Flags&0x800 == 0 {
			rawPath, _ = charmap.CodePage437.NewDecoder().String(rawPath)
		}
		// ZipInfo truncates filenames at NUL before validation.
		rawPath, _, _ = strings.Cut(rawPath, "\x00")
		member, err := validateMemberPath(rawPath)
		if err != nil {
			return fail(err)
		}
		if seen[member] {
			return fail(invalid("duplicate path in archive: " + member))
		}
		seen[member] = true
		directory := strings.HasSuffix(rawPath, "/")
		if err = validateMetadata(file.Flags, file.ExternalAttrs, directory, member); err != nil {
			return fail(err)
		}
		if directory {
			continue
		}
		if nestedArchive(member) {
			return fail(invalid("nested archive entries are not allowed: " + member))
		}
		if err = accountMember(&manifest, file.UncompressedSize64, member); err != nil {
			return fail(err)
		}
		if err = validateRatio(file.UncompressedSize64, file.CompressedSize64, member); err != nil {
			return fail(err)
		}
		data, err := readArchiveFile(file)
		if err != nil {
			return fail(err)
		}
		if err = validateMagic(data, member); err != nil {
			return fail(err)
		}
		if required != "" && member == required {
			if string(data) != rootText {
				return fail(invalid("root SKILL.md contents must exactly match supplied skill_md UTF-8 bytes"))
			}
			found = true
		}
		manifest.Entries = append(manifest.Entries, ManifestEntry{Path: member, Size: file.UncompressedSize64, MediaType: guessMediaType(member), SHA256: fmt.Sprintf("%x", sha256.Sum256(data))})
	}
	if required != "" && !found {
		return fail(invalid("archive must contain root " + required))
	}
	manifest.FileCount = len(manifest.Entries)
	return ValidatedPack{SkillName: name, Version: version, SkillMD: rootText, ZIPBytes: append([]byte{}, raw...), Manifest: manifest}, nil
}
func accountMember(manifest *Manifest, size uint64, name string) error {
	if size > MaxFileBytes {
		return invalid("file exceeds maximum uncompressed size: " + name)
	}
	manifest.TotalUncompressedBytes += size
	if manifest.TotalUncompressedBytes > MaxUncompressedBytes {
		return invalid("archive exceeds maximum uncompressed size")
	}
	if len(manifest.Entries)+1 > MaxFileCount {
		return invalid("archive exceeds maximum file count")
	}
	return nil
}
func readArchiveFile(file *zip.File) ([]byte, error) {
	r, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(io.LimitReader(r, MaxFileBytes+1))
}
func validateMemberPath(raw string) (string, error) {
	if raw == "" {
		return "", invalid("archive contains an empty path")
	}
	if strings.Contains(raw, `\`) {
		return "", invalid("backslashes are not allowed in archive paths: " + raw)
	}
	if strings.HasPrefix(raw, "/") {
		return "", invalid("absolute paths are not allowed in archive: " + raw)
	}
	for _, r := range raw {
		if r < 32 || r == 127 {
			return "", invalid("control characters are not allowed in archive paths: " + pythonPathRepr(raw))
		}
	}
	parts := []string{}
	for _, part := range strings.Split(raw, "/") {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return "", invalid("path traversal is not allowed in archive: " + raw)
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return "", invalid("archive contains an empty path")
	}
	normalized := strings.Join(parts, "/")
	if strings.HasSuffix(raw, "/") {
		normalized += "/"
	}
	return normalized, nil
}
func pythonPathRepr(raw string) string {
	quote := "'"
	if strings.Contains(raw, "'") && !strings.Contains(raw, `"`) {
		quote = `"`
	}
	var b strings.Builder
	b.WriteString(quote)
	for _, r := range raw {
		switch r {
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\\':
			b.WriteString(`\\`)
		default:
			if r < 32 || r == 127 {
				fmt.Fprintf(&b, `\x%02x`, r)
			} else {
				if string(r) == quote {
					b.WriteByte('\\')
				}
				b.WriteRune(r)
			}
		}
	}
	b.WriteString(quote)
	return b.String()
}
func validateMetadata(flags uint16, attrs uint32, directory bool, name string) error {
	if flags&1 != 0 {
		return invalid("encrypted entries are not allowed: " + name)
	}
	mode := (attrs >> 16) & 0177777
	if mode == 0 {
		return nil
	}
	kind := mode & 0170000
	if directory {
		if kind != 0 && kind != 0040000 {
			return invalid("directory entry has unsupported file mode: " + name)
		}
		return nil
	}
	if kind == 0 || kind == 0100000 {
		return nil
	}
	if kind == 0120000 {
		return invalid("symlinks are not allowed in archive: " + name)
	}
	return invalid("special file types are not allowed in archive: " + name)
}
func nestedArchive(name string) bool {
	for _, suffix := range []string{".zip", ".tar", ".tgz", ".tbz", ".tbz2", ".txz", ".tar.gz", ".tar.bz2", ".tar.xz", ".gz", ".bz2", ".xz", ".7z", ".rar"} {
		if strings.HasSuffix(strings.ToLower(name), suffix) {
			return true
		}
	}
	return false
}
func validateRatio(size, compressed uint64, name string) error {
	if size == 0 {
		return nil
	}
	if compressed == 0 {
		return invalid("invalid compressed size for entry: " + name)
	}
	if float64(size)/float64(compressed) > MaxCompressionRatio {
		return invalid("compression ratio exceeds limit: " + name)
	}
	return nil
}
func validateMagic(data []byte, name string) error {
	if bytes.HasPrefix(data, []byte{127, 'E', 'L', 'F'}) {
		return invalid("ELF binaries are not allowed: " + name)
	}
	if bytes.HasPrefix(data, []byte("MZ")) {
		return invalid("PE binaries are not allowed: " + name)
	}
	for _, magic := range [][]byte{{0xfe, 0xed, 0xfa, 0xce}, {0xfe, 0xed, 0xfa, 0xcf}, {0xce, 0xfa, 0xed, 0xfe}, {0xcf, 0xfa, 0xed, 0xfe}} {
		if bytes.HasPrefix(data, magic) {
			return invalid("Mach-O binaries are not allowed: " + name)
		}
	}
	if len(data) >= 8 && bytes.Equal(data[:4], []byte{0xca, 0xfe, 0xba, 0xbe}) {
		count := binary.BigEndian.Uint32(data[4:8])
		if count > 0 && count < 256 {
			return invalid("Mach-O binaries are not allowed: " + name)
		}
	}
	return nil
}

// MIME tables are captured from the pinned oracle environment, not the host's
// /etc/mime.types. This avoids Go/Python and deployment-host lookup drift.
//
//go:embed mime.json
var mimeJSON []byte

type mimeTables struct{ Types, Common, Suffixes, Encodings map[string]string }

var mimeLookup = decodeMIME(mimeJSON)

func decodeMIME(raw []byte) mimeTables {
	var out mimeTables
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	return out
}
func mimeExtension(name string) string {
	base := path.Base(name)
	ext := path.Ext(base)
	if strings.Trim(strings.TrimSuffix(base, ext), ".") == "" {
		return ""
	}
	return ext
}
func guessMediaType(name string) string {
	if strings.HasPrefix(strings.ToLower(name), "data:") {
		value, _, _ := strings.Cut(name[5:], ",")
		kind, _, _ := strings.Cut(value, ";")
		if strings.Contains(kind, "/") {
			return kind
		}
		return "text/plain"
	}
	if parsed, err := url.Parse(name); err == nil && parsed.Scheme != "" {
		name = parsed.Path
	}
	extension := mimeExtension(name)
	for {
		suffix, ok := mimeLookup.Suffixes[extension]
		if !ok {
			break
		}
		name = strings.TrimSuffix(name, extension) + suffix
		extension = mimeExtension(name)
	}
	if _, ok := mimeLookup.Encodings[extension]; ok {
		name = strings.TrimSuffix(name, extension)
		extension = mimeExtension(name)
	}
	extension = strings.ToLower(extension)
	if kind, ok := mimeLookup.Types[extension]; ok {
		return kind
	}
	if kind, ok := mimeLookup.Common[extension]; ok {
		return kind
	}
	return "application/octet-stream"
}
