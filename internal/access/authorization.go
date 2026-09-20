// Package access implements trusted principal policy and persisted access
// records. A principal supplied by a tool argument is never an identity source.
package access

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/rcarmo/memento/internal/repository"
)

type Principal struct {
	Name     string            `json:"name"`
	Roles    []string          `json:"roles"`
	Metadata map[string]string `json:"metadata"`
}
type NamespacePolicy struct {
	Roles         []string `json:"roles"`
	TokenEnv      string   `json:"token_env"`
	ReadPrefixes  []string `json:"read_prefixes"`
	WritePrefixes []string `json:"write_prefixes"`
}
type AuthorizationConfig struct {
	Principals            map[string]NamespacePolicy `json:"principals"`
	ProtectedReadPrefixes []string                   `json:"protected_read_prefixes"`
}
type EffectivePolicy struct {
	Principal             string   `json:"principal"`
	Roles                 []string `json:"roles"`
	ReadPrefixes          []string `json:"read_prefixes"`
	WritePrefixes         []string `json:"write_prefixes"`
	ProtectedReadPrefixes []string `json:"protected_read_prefixes"`
}
type AuthorizedNamespace struct {
	Principal string `json:"principal"`
	Path      string `json:"path"`
	Action    string `json:"action"`
}
type AuthorizationError struct{ Message string }

func (e *AuthorizationError) Error() string { return e.Message }
func denied(message string) error           { return &AuthorizationError{Message: message} }
func unique(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	n := 0
	for _, v := range out {
		if n == 0 || out[n-1] != v {
			out[n] = v
			n++
		}
	}
	return out[:n]
}
func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func trim(value string) string {
	return strings.TrimFunc(value, func(r rune) bool { return unicode.IsSpace(r) || r >= 0x1c && r <= 0x1f })
}

// ValidateConfig applies just the authorization-related Python config rules.
// It does not impose managed-account role/name restrictions or canonicalise
// prefix aliases that the source accepts. Service config loading is separate.
func ValidateConfig(config AuthorizationConfig) (AuthorizationConfig, error) {
	out := AuthorizationConfig{Principals: map[string]NamespacePolicy{}, ProtectedReadPrefixes: unique(config.ProtectedReadPrefixes)}
	for name, policy := range config.Principals {
		if len(policy.Roles) == 0 || len(policy.ReadPrefixes) == 0 || trim(policy.TokenEnv) == "" {
			return out, fmt.Errorf("invalid namespace policy")
		}
		policy.Roles = unique(policy.Roles)
		policy.ReadPrefixes = unique(policy.ReadPrefixes)
		policy.WritePrefixes = unique(policy.WritePrefixes)
		policy.TokenEnv = trim(policy.TokenEnv)
		for _, prefix := range append(append([]string{}, policy.ReadPrefixes...), policy.WritePrefixes...) {
			if !strings.HasPrefix(prefix, "/") || !strings.HasSuffix(prefix, "/") {
				return out, fmt.Errorf("namespace prefixes must start and end with '/'")
			}
		}
		out.Principals[name] = policy
	}
	for _, prefix := range out.ProtectedReadPrefixes {
		if prefix == "/" || !strings.HasPrefix(prefix, "/") || !strings.HasSuffix(prefix, "/") {
			return out, fmt.Errorf("protected namespace prefixes must start and end with '/' and cannot be '/'")
		}
	}
	return out, nil
}
func ResolvePolicy(config AuthorizationConfig, principal Principal) (EffectivePolicy, error) {
	policy, ok := config.Principals[principal.Name]
	if !ok {
		return EffectivePolicy{}, denied("unknown principal: " + principal.Name)
	}
	missing := []string{}
	for _, role := range unique(policy.Roles) {
		if !contains(principal.Roles, role) {
			missing = append(missing, role)
		}
	}
	if len(missing) > 0 {
		return EffectivePolicy{}, denied("principal " + principal.Name + " is missing required roles: " + strings.Join(missing, ", "))
	}
	return EffectivePolicy{Principal: principal.Name, Roles: append([]string{}, policy.Roles...), ReadPrefixes: append([]string{}, policy.ReadPrefixes...), WritePrefixes: append([]string{}, policy.WritePrefixes...), ProtectedReadPrefixes: append([]string{}, config.ProtectedReadPrefixes...)}, nil
}
func RequireRole(policy EffectivePolicy, role string) error {
	if !contains(policy.Roles, role) {
		return denied("principal " + policy.Principal + " lacks role: " + role)
	}
	return nil
}
func BroadReadGrantWarning(roles, reads, protected []string) *string {
	uncovered := false
	for _, prefix := range protected {
		if !contains(reads, prefix) {
			uncovered = true
		}
	}
	if uncovered && contains(reads, "/") && !contains(roles, "admin") {
		message := "broad '/' read grant excludes protected namespaces; add explicit read prefixes where access is intended"
		return &message
	}
	return nil
}
func PathMatchesPrefix(path, prefix string) bool {
	without := ""
	if len(prefix) > 0 {
		runes := []rune(prefix)
		without = string(runes[:len(runes)-1])
	}
	return path == without || strings.HasPrefix(path, prefix)
}

type ProtectedGrant struct {
	Prefix   string
	Explicit []string
}

func ProtectedReadGrants(policy EffectivePolicy) []ProtectedGrant {
	grants := []ProtectedGrant{}
	if contains(policy.Roles, "admin") {
		return grants
	}
	for _, protected := range policy.ProtectedReadPrefixes {
		explicit := []string{}
		for _, grant := range policy.ReadPrefixes {
			if grant == protected || strings.HasPrefix(grant, protected) {
				explicit = append(explicit, grant)
			}
		}
		grants = append(grants, ProtectedGrant{protected, explicit})
	}
	return grants
}
func AuthorizePath(policy EffectivePolicy, path, action string) (AuthorizedNamespace, error) {
	validate := path
	if path != "/" && strings.HasSuffix(path, "/") {
		validate = path[:len(path)-1]
	}
	if err := repository.ValidateBundlePath(validate); err != nil {
		return AuthorizedNamespace{}, denied(err.Error())
	}
	if strings.HasPrefix(path, "/trash/") {
		path = strings.TrimPrefix(path, "/trash")
		if strings.HasPrefix(path, "/trash/") {
			return AuthorizedNamespace{}, denied("nested trash paths are not allowed")
		}
	}
	prefixes := policy.WritePrefixes
	if action == "read" {
		prefixes = policy.ReadPrefixes
	}
	allowed := false
	for _, prefix := range prefixes {
		if PathMatchesPrefix(path, prefix) {
			allowed = true
			break
		}
	}
	if !allowed {
		return AuthorizedNamespace{}, denied("principal " + policy.Principal + " cannot " + action + " " + path)
	}
	if action == "read" {
		for _, grant := range ProtectedReadGrants(policy) {
			if !PathMatchesPrefix(path, grant.Prefix) {
				continue
			}
			explicit := false
			for _, prefix := range grant.Explicit {
				if PathMatchesPrefix(path, prefix) {
					explicit = true
					break
				}
			}
			if !explicit {
				return AuthorizedNamespace{}, denied("principal " + policy.Principal + " cannot read " + path)
			}
		}
	}
	return AuthorizedNamespace{Principal: policy.Principal, Path: path, Action: action}, nil
}
func FilterAuthorizedPaths(policy EffectivePolicy, paths []string, action string) []string {
	out := []string{}
	for _, path := range paths {
		if _, err := AuthorizePath(policy, path, action); err == nil {
			out = append(out, path)
		}
	}
	return out
}
