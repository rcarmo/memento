package assets

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rcarmo/memento/internal/pyjson"
	"github.com/rcarmo/memento/internal/repository"
)

var conceptIDPattern = regexp.MustCompile(`^[0-9a-fA-F-]{8,64}$`)

func AssetVersionPaths(conceptID, kind, version string) (string, string, error) {
	if !conceptIDPattern.MatchString(conceptID) {
		return "", "", invalid("invalid concept id for asset storage")
	}
	if err := ValidateKind(kind); err != nil {
		return "", "", err
	}
	if _, err := ParseStableSemver(version); err != nil {
		return "", "", err
	}
	base := "/.assets/" + conceptID + "/" + kind + "/" + version
	return base + ".json", base + ".zip", nil
}

type AcceptedVersion struct {
	ConceptID, ConceptPath, AssetKind, Version string
	ZIPBytes                                   []byte
	Manifest                                   Manifest
	AcceptedBy, SourceProposalID               string
	CreatedAt                                  *time.Time
}

// WriteAssetVersion writes metadata then ZIP without changing Git refs. Callers
// must validate the archive/manifest and authorise the concept first. Partial
// writes remain in the operation worktree for transaction failure cleanup.
// Unlike Python's direct writer, symlink parents/targets fail closed.
func WriteAssetVersion(worktree string, version AcceptedVersion) ([]string, error) {
	return writeAssetVersion(worktree, version, defaultAcceptedIO())
}

type acceptedIO struct {
	lstat    func(string) (fs.FileInfo, error)
	readDir  func(string) ([]os.DirEntry, error)
	openRoot func(string) (*os.Root, error)
	mkdir    func(*os.Root, string) error
	write    func(*os.Root, string, []byte) error
}

func defaultAcceptedIO() acceptedIO {
	return acceptedIO{os.Lstat, os.ReadDir, os.OpenRoot, func(root *os.Root, name string) error { return root.MkdirAll(name, 0777) }, writeAcceptedFile}
}
func writeAssetVersion(worktree string, version AcceptedVersion, ops acceptedIO) ([]string, error) {
	metadataPath, zipPath, err := AssetVersionPaths(version.ConceptID, version.AssetKind, version.Version)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{metadataPath, zipPath} {
		if _, err = ops.lstat(filepath.Join(worktree, name[1:])); err == nil {
			return nil, invalid("accepted asset version already exists: " + version.ConceptPath + " " + version.AssetKind + " " + version.Version)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	if _, err = assetDirectory(worktree, version.ConceptID, &version.AssetKind, ops.lstat); err != nil {
		return nil, err
	}
	if info, err := ops.lstat(worktree); err != nil {
		return nil, err
	} else if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, invalid("repository root must be a real directory")
	}
	metadata := map[string]any{"schema_version": 1, "kind": "asset_pack_version", "concept_id": version.ConceptID, "concept_path": version.ConceptPath, "asset_kind": version.AssetKind, "version": version.Version, "zip_sha256": version.Manifest.SHA256, "accepted_by": version.AcceptedBy, "source_proposal_id": version.SourceProposalID, "manifest": manifestValue(version.Manifest)}
	if version.CreatedAt != nil {
		metadata["created_at"] = acceptedTimestamp(*version.CreatedAt)
	}
	raw, err := pyjson.DumpsIndent(metadata)
	if err != nil {
		return nil, err
	}
	root, err := ops.openRoot(worktree)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if err = ops.mkdir(root, filepath.Dir(metadataPath[1:])); err != nil {
		return nil, err
	}
	if err = ops.write(root, metadataPath[1:], []byte(raw+"\n")); err != nil {
		return nil, err
	}
	if err = ops.write(root, zipPath[1:], version.ZIPBytes); err != nil {
		return nil, err
	}
	return []string{metadataPath, zipPath}, nil
}
func manifestValue(manifest Manifest) map[string]any {
	entries := []any{}
	for _, e := range manifest.Entries {
		entries = append(entries, map[string]any{"path": e.Path, "size": jsonNumber(e.Size), "media_type": e.MediaType, "sha256": e.SHA256})
	}
	return map[string]any{"entries": entries, "sha256": manifest.SHA256, "total_uncompressed_bytes": jsonNumber(manifest.TotalUncompressedBytes), "file_count": manifest.FileCount}
}
func jsonNumber(value uint64) json.Number { return json.Number(strconv.FormatUint(value, 10)) }
func acceptedTimestamp(value time.Time) string {
	value = value.UTC()
	text := value.Format("2006-01-02T15:04:05")
	if value.Nanosecond()/1000 != 0 {
		text += value.Format(".000000")
	}
	return text + "Z"
}
func writeAcceptedFile(root *os.Root, name string, data []byte) error {
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return err
	}
	return writeAcceptedBytes(file, data)
}

type acceptedWriter interface {
	io.Writer
	Close() error
}

func writeAcceptedBytes(file acceptedWriter, data []byte) error {
	n, err := file.Write(data)
	closed := file.Close()
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return closed
}
func assetDirectory(root, conceptID string, kind *string, lstat func(string) (fs.FileInfo, error)) (string, error) {
	validate := "asset"
	if kind != nil && *kind != "" {
		validate = *kind
	}
	if _, _, err := AssetVersionPaths(conceptID, validate, "0.0.0"); err != nil {
		return "", err
	}
	parts := []string{".assets", conceptID}
	if kind != nil {
		parts = append(parts, *kind)
	}
	current := root
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, err := lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", invalid("asset directory must not be a symlink or special file")
		}
	}
	return current, nil
}
func ListAssetKinds(root, conceptID string) ([]string, error) {
	return listAssetKinds(root, conceptID, defaultAcceptedIO())
}
func listAssetKinds(root, conceptID string, ops acceptedIO) ([]string, error) {
	if !conceptIDPattern.MatchString(conceptID) {
		return []string{}, nil
	}
	directory, err := assetDirectory(root, conceptID, nil, ops.lstat)
	if err != nil {
		return nil, err
	}
	entries, err := ops.readDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	kinds := []string{}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, invalid("asset kind directory must not be a symlink")
		}
		if entry.IsDir() {
			kinds = append(kinds, entry.Name())
		}
	}
	for _, kind := range kinds {
		if err = ValidateKind(kind); err != nil {
			return nil, err
		}
	}
	sort.Strings(kinds)
	return kinds, nil
}
func ListAssetVersions(root, conceptID, kind string) ([]string, error) {
	return listAssetVersions(root, conceptID, kind, defaultAcceptedIO())
}
func listAssetVersions(root, conceptID, kind string, ops acceptedIO) ([]string, error) {
	if err := ValidateKind(kind); err != nil {
		return nil, err
	}
	directory, err := assetDirectory(root, conceptID, &kind, ops.lstat)
	if err != nil {
		return nil, err
	}
	entries, err := ops.readDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	versions := []string{}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			versions = append(versions, strings.TrimSuffix(entry.Name(), ".json"))
		}
	}
	return sortedVersions(versions, false)
}
func sortedVersions(versions []string, descending bool) ([]string, error) {
	out := append([]string{}, versions...)
	parsed := map[string][3]*big.Int{}
	for _, version := range out {
		numbers, err := ParseStableSemver(version)
		if err != nil {
			return nil, err
		}
		parsed[version] = numbers
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := parsed[out[i]], parsed[out[j]]
		for k := range 3 {
			if cmp := a[k].Cmp(b[k]); cmp != 0 {
				if descending {
					return cmp > 0
				}
				return cmp < 0
			}
		}
		return false
	})
	return out, nil
}

type AssetVersionNotFoundError struct{ Message string }

func (e *AssetVersionNotFoundError) Error() string { return e.Message }
func ResolveAssetVersion(root, conceptID, kind string, version *string) (string, error) {
	versions, err := ListAssetVersions(root, conceptID, kind)
	if err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", &AssetVersionNotFoundError{Message: "no " + kind + " asset pack for concept"}
	}
	if version == nil {
		return versions[len(versions)-1], nil
	}
	if _, err = ParseStableSemver(*version); err != nil {
		return "", err
	}
	for _, candidate := range versions {
		if candidate == *version {
			return candidate, nil
		}
	}
	return "", &AssetVersionNotFoundError{Message: "unknown " + kind + " asset version: " + *version}
}
func RetentionPartition(versions []string, keep int) ([]string, []string, error) {
	if keep < 1 {
		return nil, nil, errors.New("keep must be at least one")
	}
	seen := map[string]bool{}
	unique := []string{}
	for _, version := range versions {
		if !seen[version] {
			seen[version] = true
			unique = append(unique, version)
		}
	}
	ordered, err := sortedVersions(unique, true)
	if err != nil {
		return nil, nil, err
	}
	cut := min(keep, len(ordered))
	return ordered[:cut], ordered[cut:], nil
}
func LoadAssetMetadata(root, conceptID, kind, version string) (map[string]any, error) {
	return loadAssetMetadata(root, conceptID, kind, version, defaultAcceptedIO())
}
func loadAssetMetadata(root, conceptID, kind, version string, ops acceptedIO) (map[string]any, error) {
	metadata, _, err := AssetVersionPaths(conceptID, kind, version)
	if err != nil {
		return nil, err
	}
	if _, err = assetDirectory(root, conceptID, &kind, ops.lstat); err != nil {
		return nil, err
	}
	file := filepath.Join(root, metadata[1:])
	info, err := ops.lstat(file)
	if errors.Is(err, os.ErrNotExist) || err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return nil, invalid("asset metadata must be a regular file")
	}
	if err != nil {
		return nil, err
	}
	if _, err = repository.ValidateRepositoryReadPath(root, metadata); err != nil {
		return nil, err
	}
	confined, err := ops.openRoot(root)
	if err != nil {
		return nil, err
	}
	defer confined.Close()
	source, err := confined.Open(metadata[1:])
	if err != nil {
		return nil, err
	}
	defer source.Close()
	return readAssetMetadata(source)
}

// AcceptedAssetPaths returns every manifest-relative file in the latest
// accepted version of each asset kind attached to a concept.
func AcceptedAssetPaths(root, conceptID string) (map[string]bool, error) {
	paths := map[string]bool{}
	kinds, err := ListAssetKinds(root, conceptID)
	if err != nil {
		return nil, err
	}
	for _, kind := range kinds {
		version, err := ResolveAssetVersion(root, conceptID, kind, nil)
		if err != nil {
			return nil, err
		}
		metadata, err := LoadAssetMetadata(root, conceptID, kind, version)
		if err != nil {
			return nil, err
		}
		if metadata["concept_id"] != conceptID || metadata["asset_kind"] != kind || metadata["version"] != version {
			return nil, invalid("asset metadata identity mismatch")
		}
		digest, ok := metadata["zip_sha256"].(string)
		if !ok {
			return nil, invalid("asset metadata has an invalid zip_sha256")
		}
		raw, _ := json.Marshal(metadata["manifest"]) // Loaded JSON has no unsupported Go values.
		manifest, err := ParseManifest(string(raw))
		if err != nil {
			return nil, err
		}
		if err = CheckedManifest(manifest, digest); err != nil {
			return nil, err
		}
		_, archive, _ := AssetVersionPaths(conceptID, kind, version) // Already validated during metadata loading.
		info, err := os.Lstat(filepath.Join(root, archive[1:]))
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, invalid("asset ZIP must be a regular file")
		}
		for _, entry := range manifest.Entries {
			paths[entry.Path] = true
		}
	}
	return paths, nil
}

func readAssetMetadata(source io.Reader) (map[string]any, error) {
	raw, err := io.ReadAll(io.LimitReader(source, MaxMetadataBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxMetadataBytes {
		return nil, invalid("asset metadata exceeds size limit")
	}
	value, err := pyjson.Parse(string(raw))
	if err != nil {
		return nil, err
	}
	metadata, ok := value.(map[string]any)
	if !ok || metadata["kind"] != "asset_pack_version" {
		return nil, invalid("invalid asset metadata")
	}
	return metadata, nil
}
