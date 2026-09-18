package graphdebug

import (
	"strings"

	"github.com/rcarmo/memento/go/access"
)

func hasRole(roles []string, want string) bool {
	for _, role := range roles {
		if role == want {
			return true
		}
	}
	return false
}
func prefixScopeSQL(column string, prefixes []string) (string, []any) {
	clauses := []string{}
	args := []any{}
	for _, prefix := range prefixes {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(prefix)
		clauses = append(clauses, "("+column+" = ? OR "+column+" LIKE ? ESCAPE '\\')")
		args = append(args, strings.TrimSuffix(prefix, "/"), escaped+"%")
	}
	return "(" + strings.Join(clauses, " OR ") + ")", args
}
func pathScopeSQL(column string, policy *access.EffectivePolicy) (string, []any) {
	if policy == nil {
		return "", nil
	}
	column = "(CASE WHEN " + column + " LIKE '/trash/%' THEN substr(" + column + ", 7) ELSE " + column + " END)"
	query, args := prefixScopeSQL(column, policy.ReadPrefixes)
	if hasRole(policy.Roles, "admin") {
		return "(" + query + ")", args
	}
	for _, protected := range policy.ProtectedReadPrefixes {
		protectedSQL, protectedArgs := prefixScopeSQL(column, []string{protected})
		args = append(args, protectedArgs...)
		grants := []string{}
		for _, grant := range policy.ReadPrefixes {
			if grant == protected || strings.HasPrefix(grant, protected) {
				grants = append(grants, grant)
			}
		}
		if len(grants) > 0 {
			explicitSQL, explicitArgs := prefixScopeSQL(column, grants)
			query += " AND (NOT " + protectedSQL + " OR " + explicitSQL + ")"
			args = append(args, explicitArgs...)
		} else {
			query += " AND NOT " + protectedSQL
		}
	}
	return "(" + query + ")", args
}
