package access

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/internal/pyjson"
	"modernc.org/sqlite"
)

type ManagedPrincipal struct {
	Name          string   `json:"name"`
	Roles         []string `json:"roles"`
	ReadPrefixes  []string `json:"read_prefixes"`
	WritePrefixes []string `json:"write_prefixes"`
	Enabled       bool     `json:"enabled"`
	Revoked       bool     `json:"revoked"`
	Deleted       bool     `json:"deleted"`
	UpdatedAt     string   `json:"updated_at"`
}
type AuditEntry struct {
	Actor     string         `json:"actor"`
	Action    string         `json:"action"`
	Target    string         `json:"target"`
	Details   map[string]any `json:"details"`
	CreatedAt string         `json:"created_at"`
}
type Store struct {
	db       *sql.DB
	verifier []byte
	random   io.Reader
	now      func() time.Time
	randomMu sync.Mutex
}

// OpenStore opens the wrapped verifier key. Database schema creation belongs to
// control.Migrate; no plaintext bearer tokens or master keys are stored here.
func OpenStore(ctx context.Context, db *sql.DB, master string) (*Store, error) {
	return openStore(ctx, db, master, rand.Reader, time.Now)
}
func openStore(ctx context.Context, db *sql.DB, master string, random io.Reader, now func() time.Time) (*Store, error) {
	var payload string
	err := db.QueryRowContext(ctx, "SELECT value FROM access_meta WHERE key='verifier_key'").Scan(&payload)
	s := &Store{db: db, random: random, now: now}
	if errors.Is(err, sql.ErrNoRows) {
		key := make([]byte, 32)
		if _, err = io.ReadFull(random, key); err != nil {
			return nil, err
		}
		payload, err = sealKey(key, master, random)
		if err != nil {
			return nil, err
		}
		if _, err = db.ExecContext(ctx, "INSERT INTO access_meta(key,value,updated_at) VALUES('verifier_key',?,?)", payload, s.timestamp()); err != nil {
			return nil, err
		}
		s.verifier = key
	} else if err != nil {
		return nil, err
	} else {
		s.verifier, err = openKey(payload, master)
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

// WithDB gives a worker its own connection pool while retaining the decrypted
// verifier key. Callers own db lifetime. Production randomness is independent;
// no mutex or injected test reader is copied between workers.
func (s *Store) WithDB(db *sql.DB) *Store {
	return &Store{db: db, verifier: append([]byte{}, s.verifier...), random: rand.Reader, now: s.now}
}

func (s *Store) timestamp() string { return s.now().UTC().Truncate(time.Second).Format(time.RFC3339) }
func (s *Store) token() (string, error) {
	s.randomMu.Lock()
	defer s.randomMu.Unlock()
	return newToken(s.random)
}
func integrity(err error) bool           { var e *sqlite.Error; return errors.As(err, &e) && e.Code()&255 == 19 }
func stringsJSON(values []string) string { encoded, _ := pyjson.Dumps(values); return encoded }
func (s *Store) transaction(ctx context.Context, action func(*sql.Tx) error) error {
	return control.WithTransaction(ctx, s.db, action)
}

// ConfiguredPrincipal binds a bootstrap name to its namespace policy.
// Bootstrap preserves input order, including the piclaw-workspace -> sandbox
// alias. Existing rows/credentials are not overwritten or silently re-enabled.
type ConfiguredPrincipal struct {
	Name   string
	Policy NamespacePolicy
}

func (s *Store) Bootstrap(ctx context.Context, principals []ConfiguredPrincipal, tokens map[string]string) error {
	return s.transaction(ctx, func(tx *sql.Tx) error {
		for _, configured := range principals {
			name := configured.Name
			roles := configured.Policy.Roles
			if name == "piclaw-workspace" {
				name = "sandbox"
			}
			if name == "sandbox" {
				roles = append(append([]string{}, roles...), "admin")
			}
			roles, reads, writes, err := validatePolicy(name, roles, configured.Policy.ReadPrefixes, configured.Policy.WritePrefixes)
			if err != nil {
				return err
			}
			now := s.timestamp()
			if _, err = tx.ExecContext(ctx, `INSERT INTO access_principals(name,roles_json,read_prefixes_json,write_prefixes_json,enabled,revoked,deleted,created_at,updated_at) VALUES(?,?,?,?,1,0,0,?,?) ON CONFLICT(name) DO NOTHING`, name, stringsJSON(roles), stringsJSON(reads), stringsJSON(writes), now, now); err != nil {
				return err
			}
			if token := tokens[configured.Name]; token != "" {
				if _, err = tx.ExecContext(ctx, `INSERT INTO access_credentials(principal_name,token_digest,created_at,revoked_at) VALUES(?,?,?,NULL) ON CONFLICT(principal_name) DO NOTHING`, name, digest(s.verifier, token), now); err != nil {
					return err
				}
			}
		}
		exists := []bool{}
		for _, name := range []string{"piclaw-workspace", "sandbox"} {
			var found string
			err := tx.QueryRowContext(ctx, "SELECT name FROM access_principals WHERE name=?", name).Scan(&found)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			exists = append(exists, err == nil)
		}
		if exists[0] && !exists[1] {
			_, err := tx.ExecContext(ctx, "UPDATE access_principals SET name='sandbox',updated_at=? WHERE name='piclaw-workspace'", s.timestamp())
			return err
		}
		return nil
	})
}
func (s *Store) Authenticate(ctx context.Context, token string) (*Principal, error) {
	var name, raw string
	err := s.db.QueryRowContext(ctx, `SELECT p.name,p.roles_json FROM access_credentials c JOIN access_principals p ON p.name=c.principal_name WHERE c.token_digest=? AND c.revoked_at IS NULL AND p.enabled=1 AND p.revoked=0 AND p.deleted=0`, digest(s.verifier, token)).Scan(&name, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var roles []string
	if err = json.Unmarshal([]byte(raw), &roles); err != nil {
		return nil, err
	}
	if name == "" || len(roles) == 0 {
		return nil, accessError("invalid stored principal")
	}
	return &Principal{Name: name, Roles: unique(roles), Metadata: map[string]string{}}, nil
}
func (s *Store) Policy(ctx context.Context, name string) (*NamespacePolicy, error) {
	var roles, reads, writes string
	err := s.db.QueryRowContext(ctx, "SELECT roles_json,read_prefixes_json,write_prefixes_json FROM access_principals WHERE name=? AND enabled=1 AND revoked=0 AND deleted=0", name).Scan(&roles, &reads, &writes)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	policy := NamespacePolicy{TokenEnv: "MANAGED_ACCESS"}
	for _, field := range []struct {
		raw    string
		target *[]string
	}{{roles, &policy.Roles}, {reads, &policy.ReadPrefixes}, {writes, &policy.WritePrefixes}} {
		if err = json.Unmarshal([]byte(field.raw), field.target); err != nil {
			return nil, err
		}
	}
	config, err := ValidateConfig(AuthorizationConfig{Principals: map[string]NamespacePolicy{name: policy}})
	if err != nil {
		return nil, err
	}
	value := config.Principals[name]
	return &value, nil
}
func (s *Store) List(ctx context.Context) ([]ManagedPrincipal, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT name,roles_json,read_prefixes_json,write_prefixes_json,enabled,revoked,deleted,updated_at FROM access_principals ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPrincipals(rows)
}

type principalRows interface {
	Next() bool
	Scan(...any) error
	Err() error
}

func scanPrincipals(rows principalRows) ([]ManagedPrincipal, error) {
	out := []ManagedPrincipal{}
	for rows.Next() {
		var item ManagedPrincipal
		var roles, reads, writes string
		var enabled, revoked, deleted int64
		if err := rows.Scan(&item.Name, &roles, &reads, &writes, &enabled, &revoked, &deleted, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Enabled = enabled != 0
		item.Revoked = revoked != 0
		item.Deleted = deleted != 0
		for _, field := range []struct {
			raw    string
			target *[]string
		}{{roles, &item.Roles}, {reads, &item.ReadPrefixes}, {writes, &item.WritePrefixes}} {
			if err := json.Unmarshal([]byte(field.raw), field.target); err != nil {
				return nil, err
			}
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Store) require(ctx context.Context, name string) (ManagedPrincipal, error) {
	items, err := s.List(ctx)
	if err != nil {
		return ManagedPrincipal{}, err
	}
	for _, item := range items {
		if item.Name == name {
			return item, nil
		}
	}
	return ManagedPrincipal{}, accessError("principal not found")
}
func (s *Store) otherAdmin(ctx context.Context, name string) error {
	items, err := s.List(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Name != name && item.Enabled && !item.Revoked && !item.Deleted && contains(item.Roles, "admin") {
			return nil
		}
	}
	return accessError("the final enabled admin cannot remove its own access")
}
func (s *Store) claim(ctx context.Context, actor string, key *string, action, target string) error {
	if key == nil {
		return nil
	}
	if trim(*key) == "" {
		return accessError("idempotency key must not be empty")
	}
	_, err := s.db.ExecContext(ctx, "INSERT INTO access_idempotency(actor,idempotency_key,action,target,created_at) VALUES(?,?,?,?,?)", actor, *key, action, target, s.timestamp())
	if integrity(err) {
		return accessError("access mutation already completed; one-time credential cannot be replayed")
	}
	return err
}
func (s *Store) audit(ctx context.Context, tx *sql.Tx, actor, action, target string, details map[string]any) error {
	raw, err := pyjson.Dumps(details)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO access_audit(actor,action,target,details_json,created_at) VALUES(?,?,?,?,?)", actor, action, target, raw, s.timestamp())
	return err
}

// Create adds a managed principal and returns its credential once. It does not
// authorise the actor; the embedding admin service must verify trusted admin
// context first. One-time key claims commit before mutation and cannot replay.
func (s *Store) Create(ctx context.Context, actor, name string, roles, reads, writes []string, key *string) (ManagedPrincipal, string, error) {
	roles, reads, writes, err := validatePolicy(name, roles, reads, writes)
	if err != nil {
		return ManagedPrincipal{}, "", err
	}
	if err = s.claim(ctx, actor, key, "principal.create", name); err != nil {
		return ManagedPrincipal{}, "", err
	}
	token, err := s.token()
	if err != nil {
		return ManagedPrincipal{}, "", err
	}
	now := s.timestamp()
	err = s.transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO access_principals VALUES(?,?,?,?,1,0,0,?,?)", name, stringsJSON(roles), stringsJSON(reads), stringsJSON(writes), now, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO access_credentials VALUES(?,?,?,NULL)", name, digest(s.verifier, token), now); err != nil {
			return err
		}
		return s.audit(ctx, tx, actor, "principal.create", name, map[string]any{"roles": roles, "read_prefixes": reads, "write_prefixes": writes})
	})
	if integrity(err) {
		err = accessError("principal already exists")
	}
	if err != nil {
		return ManagedPrincipal{}, "", err
	}
	principal, err := s.require(ctx, name)
	if err != nil {
		return ManagedPrincipal{}, "", err
	}
	return principal, token, nil
}
func (s *Store) Update(ctx context.Context, actor, name string, roles, reads, writes []string) (ManagedPrincipal, error) {
	roles, reads, writes, err := validatePolicy(name, roles, reads, writes)
	if err != nil {
		return ManagedPrincipal{}, err
	}
	current, err := s.require(ctx, name)
	if err != nil {
		return ManagedPrincipal{}, err
	}
	if contains(current.Roles, "admin") && !contains(roles, "admin") {
		if err = s.otherAdmin(ctx, name); err != nil {
			return ManagedPrincipal{}, err
		}
	}
	err = s.transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "UPDATE access_principals SET roles_json=?,read_prefixes_json=?,write_prefixes_json=?,updated_at=? WHERE name=?", stringsJSON(roles), stringsJSON(reads), stringsJSON(writes), s.timestamp(), name); err != nil {
			return err
		}
		return s.audit(ctx, tx, actor, "principal.update", name, map[string]any{"roles": roles, "read_prefixes": reads, "write_prefixes": writes})
	})
	if err != nil {
		return ManagedPrincipal{}, err
	}
	return s.require(ctx, name)
}
func (s *Store) Rename(ctx context.Context, actor, name, newName string) (ManagedPrincipal, error) {
	current, err := s.require(ctx, name)
	if err != nil {
		return ManagedPrincipal{}, err
	}
	if _, _, _, err = validatePolicy(newName, current.Roles, current.ReadPrefixes, current.WritePrefixes); err != nil {
		return ManagedPrincipal{}, err
	}
	err = s.transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "UPDATE access_principals SET name=?,updated_at=? WHERE name=?", newName, s.timestamp(), name); err != nil {
			return err
		}
		return s.audit(ctx, tx, actor, "principal.rename", newName, map[string]any{"previous_name": name})
	})
	if integrity(err) {
		err = accessError("principal already exists")
	}
	if err != nil {
		return ManagedPrincipal{}, err
	}
	return s.require(ctx, newName)
}
func (s *Store) SetEnabled(ctx context.Context, actor, name string, enabled bool) (ManagedPrincipal, error) {
	current, err := s.require(ctx, name)
	if err != nil {
		return ManagedPrincipal{}, err
	}
	if !enabled && contains(current.Roles, "admin") {
		if err = s.otherAdmin(ctx, name); err != nil {
			return ManagedPrincipal{}, err
		}
	}
	action := "principal.disable"
	value := 0
	if enabled {
		action = "principal.enable"
		value = 1
	}
	err = s.transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "UPDATE access_principals SET enabled=?,updated_at=? WHERE name=?", value, s.timestamp(), name); err != nil {
			return err
		}
		return s.audit(ctx, tx, actor, action, name, map[string]any{})
	})
	if err != nil {
		return ManagedPrincipal{}, err
	}
	return s.require(ctx, name)
}
func (s *Store) Rotate(ctx context.Context, actor, name string, key *string) (string, error) {
	if _, err := s.require(ctx, name); err != nil {
		return "", err
	}
	if err := s.claim(ctx, actor, key, "credential.rotate", name); err != nil {
		return "", err
	}
	token, err := s.token()
	if err != nil {
		return "", err
	}
	now := s.timestamp()
	err = s.transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "UPDATE access_credentials SET token_digest=?,created_at=?,revoked_at=NULL WHERE principal_name=?", digest(s.verifier, token), now, name); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE access_principals SET revoked=0,updated_at=? WHERE name=?", now, name); err != nil {
			return err
		}
		return s.audit(ctx, tx, actor, "credential.rotate", name, map[string]any{})
	})
	if err != nil {
		return "", err
	}
	return token, nil
}
func (s *Store) Revoke(ctx context.Context, actor, name string) (ManagedPrincipal, error) {
	current, err := s.require(ctx, name)
	if err != nil {
		return ManagedPrincipal{}, err
	}
	if contains(current.Roles, "admin") {
		if err = s.otherAdmin(ctx, name); err != nil {
			return ManagedPrincipal{}, err
		}
	}
	now := s.timestamp()
	err = s.transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "UPDATE access_credentials SET revoked_at=? WHERE principal_name=?", now, name); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE access_principals SET revoked=1,enabled=0,updated_at=? WHERE name=?", now, name); err != nil {
			return err
		}
		return s.audit(ctx, tx, actor, "credential.revoke", name, map[string]any{})
	})
	if err != nil {
		return ManagedPrincipal{}, err
	}
	return s.require(ctx, name)
}
func (s *Store) Delete(ctx context.Context, actor, name string) (ManagedPrincipal, error) {
	current, err := s.require(ctx, name)
	if err != nil {
		return ManagedPrincipal{}, err
	}
	if current.Enabled || !current.Revoked {
		return ManagedPrincipal{}, accessError("principal must be disabled and revoked before deletion")
	}
	err = s.transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "UPDATE access_principals SET deleted=1,updated_at=? WHERE name=?", s.timestamp(), name); err != nil {
			return err
		}
		return s.audit(ctx, tx, actor, "principal.delete", name, map[string]any{})
	})
	if err != nil {
		return ManagedPrincipal{}, err
	}
	return s.require(ctx, name)
}
func (s *Store) Audit(ctx context.Context, limit int) ([]AuditEntry, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT actor,action,target,details_json,created_at FROM access_audit ORDER BY id DESC LIMIT ?", max(1, min(limit, 100)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAudit(rows)
}
func scanAudit(rows principalRows) ([]AuditEntry, error) {
	out := []AuditEntry{}
	for rows.Next() {
		var item AuditEntry
		var raw string
		if err := rows.Scan(&item.Actor, &item.Action, &item.Target, &raw, &item.CreatedAt); err != nil {
			return nil, err
		}
		value, err := pyjson.Parse(raw)
		if err != nil {
			return nil, err
		}
		details, ok := value.(map[string]any)
		if !ok {
			return nil, accessError("invalid access audit details")
		}
		item.Details = details
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Store) RotateMasterKey(ctx context.Context, oldKey, newKey string) error {
	var payload string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM access_meta WHERE key='verifier_key'").Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return accessError("access verifier key is not initialized")
	}
	if err != nil {
		return err
	}
	verifier, err := openKey(payload, oldKey)
	if err != nil {
		return err
	}
	s.randomMu.Lock()
	sealed, err := sealKey(verifier, newKey, s.random)
	s.randomMu.Unlock()
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "UPDATE access_meta SET value=?,updated_at=? WHERE key='verifier_key'", sealed, s.timestamp())
	return err
}
