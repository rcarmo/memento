package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/derived"
	"github.com/rcarmo/memento/internal/envelope"
	"github.com/rcarmo/memento/internal/repository"
	"github.com/rcarmo/memento/umcp"
)

type SuccessOptions struct {
	RepoRevision, IndexRevision *string
	IndexStale                  bool
	OperationID                 *string
	Warnings, NextTools         []string
}

// SuccessEnvelope uses the live revision only when the caller omitted it. An
// explicitly empty revision and an explicit empty index revision are preserved.
func (q ProposalQueue) SuccessEnvelope(data map[string]any, options SuccessOptions) (envelope.Success[map[string]any], error) {
	return q.successEnvelope(data, options, defaultProposalRepository())
}
func (q ProposalQueue) successEnvelope(data map[string]any, options SuccessOptions, repo proposalRepository) (envelope.Success[map[string]any], error) {
	revision := ""
	if options.RepoRevision != nil {
		revision = *options.RepoRevision
	} else {
		var err error
		revision, err = repo.main(q.Paths)
		if err != nil {
			return envelope.Success[map[string]any]{}, err
		}
	}
	index := revision
	if options.IndexRevision != nil {
		index = *options.IndexRevision
	}
	result := envelope.NewSuccess(data, revision, index)
	result.IndexStale = options.IndexStale
	result.OperationID = options.OperationID
	result.Warnings = append(result.Warnings, options.Warnings...)
	result.NextTools = append(result.NextTools, options.NextTools...)
	return result, nil
}

// FailureEnvelope maps only known source exception families. Unexpected I/O,
// SQLite, timeouts and internal failures propagate to the transport's redacted
// exception path instead of being misreported as successful service responses.
func FailureEnvelope(err error) (envelope.Failure, error) {
	class, message := "", ""
	var policy *Error
	var auth *access.AuthorizationError
	var proposal *control.ProposalNotFoundError
	var operation *control.OperationNotFoundError
	var concept *derived.ConceptNotFoundError
	var file *assets.FileNotDeclaredError
	var validation *ChangeValidationError
	var bundle *repository.BundleError
	var frontmatter *repository.FrontmatterError
	var path *repository.PathSafetyError
	var pack *assets.ValidationError
	var read *assets.ReadError
	var staged *assets.StagedAssetError
	var search *derived.SearchError
	var idempotency *control.IdempotencyConflictError
	var conflict *repository.TransactionConflictError
	var git *repository.GitError
	var unavailable *derived.UnavailableError
	var jsonSyntax *json.SyntaxError
	switch {
	case errors.As(err, &auth):
		class, message = "forbidden", auth.Error()
	case errors.As(err, &policy):
		class, message = policy.Class, policy.Error()
	case errors.As(err, &proposal):
		class = "not_found"
		message = pythonRepr(proposal.ProposalID)
		if proposal.AssetID != "" {
			message = "(" + pythonRepr(proposal.ProposalID) + ", " + pythonRepr(proposal.AssetID) + ")"
		}
	case errors.As(err, &operation):
		class, message = "not_found", pythonRepr(operation.OpID)
	case errors.As(err, &concept):
		class, message = "not_found", pythonRepr(concept.ConceptID)
	case errors.As(err, &file):
		class, message = "not_found", pythonRepr(file.Error())
	case errors.As(err, &validation), errors.As(err, &bundle), errors.As(err, &frontmatter), errors.As(err, &path), errors.As(err, &pack), errors.As(err, &read), errors.As(err, &staged), errors.As(err, &search), errors.As(err, &jsonSyntax):
		class, message = "validation_error", err.Error()
	case errors.As(err, &idempotency):
		class, message = "idempotency_conflict", idempotency.Error()
	case errors.As(err, &conflict):
		class, message = "conflict", conflict.Error()
	case errors.As(err, &git):
		class, message = "repo_unavailable", git.Error()
	case errors.As(err, &unavailable):
		class, message = "derived_index_unavailable", unavailable.Error()
	default:
		return envelope.Failure{}, err
	}
	return envelope.NewFailure(class, message)
}
func pythonRepr(text string) string {
	quote := byte('\'')
	if strings.Contains(text, "'") && !strings.Contains(text, `"`) {
		quote = '"'
	}
	var out strings.Builder
	out.WriteByte(quote)
	for _, r := range text {
		switch r {
		case '\\':
			out.WriteString(`\\`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if r == rune(quote) {
				out.WriteByte('\\')
				out.WriteRune(r)
			} else if unicode.IsPrint(r) {
				out.WriteRune(r)
			} else if r <= 255 {
				fmt.Fprintf(&out, `\x%02x`, r)
			} else if r <= 65535 {
				fmt.Fprintf(&out, `\u%04x`, r)
			} else {
				fmt.Fprintf(&out, `\U%08x`, r)
			}
		}
	}
	out.WriteByte(quote)
	return out.String()
}

// MCPEnvelope converts typed service data to the ordered JSON value domain that
// uMCP uses for both text and structured results. Numbers keep their JSON form;
// unsupported/nonfinite Go values fail instead of being silently replaced.
func MCPEnvelope(value any) (any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return umcp.ParseValue(raw)
}
