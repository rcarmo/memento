// Package envelope mirrors Memento's JSON result envelopes.
package envelope

import "errors"

// Success preserves empty arrays and explicit null operation IDs on the wire.
type Success[T any] struct {
	Status        string   `json:"status"`
	Data          T        `json:"data"`
	Warnings      []string `json:"warnings"`
	NextTools     []string `json:"next_tools"`
	RepoRevision  string   `json:"repo_revision"`
	IndexRevision string   `json:"index_revision"`
	IndexStale    bool     `json:"index_stale"`
	OperationID   *string  `json:"operation_id"`
}

// Failure preserves optional revision fields as JSON null, not omitted fields.
type Failure struct {
	Status        string   `json:"status"`
	ErrorClass    string   `json:"error_class"`
	Message       string   `json:"message"`
	Warnings      []string `json:"warnings"`
	RepoRevision  *string  `json:"repo_revision"`
	IndexRevision *string  `json:"index_revision"`
	IndexStale    bool     `json:"index_stale"`
	OperationID   *string  `json:"operation_id"`
}

// NewSuccess constructs an envelope with source-compatible default values.
func NewSuccess[T any](data T, repoRevision, indexRevision string) Success[T] {
	return Success[T]{Status: "success", Data: data, Warnings: []string{}, NextTools: []string{}, RepoRevision: repoRevision, IndexRevision: indexRevision}
}

// NewFailure enforces the nonempty class/message constraints of ErrorEnvelope.
func NewFailure(class, message string) (Failure, error) {
	if class == "" || message == "" {
		return Failure{}, errors.New("error class and message must be nonempty")
	}
	return Failure{Status: "error", ErrorClass: class, Message: message, Warnings: []string{}}, nil
}
