package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/repository"
)

type RuntimeConfig struct {
	SchemaVersion int `json:"schema_version"`
	Repository    struct {
		RootPath   string `json:"root_path"`
		BundleRoot string `json:"bundle_root"`
	} `json:"repository"`
	Authorization    access.AuthorizationConfig `json:"authorization"`
	Limits           LimitsConfig               `json:"limits"`
	MCP              MCPConfig                  `json:"mcp"`
	IntelligentTiers IntelligentTiersConfig     `json:"intelligent_tiers"`
	Observability    ObservabilityConfig        `json:"observability"`
}
type RuntimePaths struct {
	Root, ControlDB, DerivedDB, WriterLock string
	Repository                             repository.GitRepositoryPaths
}

func LoadRuntimeConfig(path string) (RuntimeConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return RuntimeConfig{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	config := RuntimeConfig{SchemaVersion: 2, Limits: defaultLimitsConfig(), MCP: defaultMCPConfig(), Observability: ObservabilityConfig{GraphExplorer: DefaultGraphExplorerConfig()}}
	config.Repository.BundleRoot = "/"
	if err = decoder.Decode(&config); err != nil {
		return RuntimeConfig{}, err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return RuntimeConfig{}, errors.New("trailing JSON")
	}
	if config.SchemaVersion != 2 {
		return RuntimeConfig{}, errors.New("schema_version must be 2")
	}
	if strings.TrimSpace(config.Repository.RootPath) == "" {
		return RuntimeConfig{}, errors.New("repository.root_path must not be empty")
	}
	if config.Repository.BundleRoot != "/" {
		return RuntimeConfig{}, errors.New("bundle_root must be '/'")
	}
	if config.Limits.MaxConceptBytes < 1 || config.Limits.MaxSearchResults < 1 {
		return RuntimeConfig{}, errors.New("service limit out of range")
	}
	if err = config.MCP.Validate(); err != nil {
		return RuntimeConfig{}, err
	}
	if err = config.IntelligentTiers.ValidateModelsOff(); err != nil {
		return RuntimeConfig{}, err
	}
	if err = config.Observability.GraphExplorer.Validate(); err != nil {
		return RuntimeConfig{}, err
	}
	return config, nil
}
func RuntimePathsFor(config RuntimeConfig) RuntimePaths {
	root := filepath.Clean(config.Repository.RootPath)
	return RuntimePaths{Root: root, ControlDB: filepath.Join(root, "control.sqlite"), DerivedDB: filepath.Join(root, "derived.sqlite"), WriterLock: filepath.Join(root, "locks", "writer.lock"), Repository: repository.GitRepositoryPaths{BareDir: filepath.Join(root, "repo.git"), CurrentDir: filepath.Join(root, "current"), WorktreesDir: filepath.Join(root, "worktrees")}}
}
func StaticBearerPrincipals(config access.AuthorizationConfig, lookup func(string) (string, bool)) ([]BearerPrincipal, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	tokens := []BearerPrincipal{}
	seen := map[string]bool{}
	names := make([]string, 0, len(config.Principals))
	for name := range config.Principals {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		policy := config.Principals[name]
		token, ok := lookup(policy.TokenEnv)
		token = strings.TrimSpace(token)
		if !ok || token == "" {
			return nil, fmt.Errorf("missing bearer token environment variable %s", policy.TokenEnv)
		}
		if seen[token] {
			return nil, fmt.Errorf("duplicate bearer token configured for %s", policy.TokenEnv)
		}
		seen[token] = true
		tokens = append(tokens, BearerPrincipal{Token: token, Principal: access.Principal{Name: name, Roles: append([]string{}, policy.Roles...), Metadata: map[string]string{"token_env": policy.TokenEnv}}})
	}
	return tokens, nil
}
