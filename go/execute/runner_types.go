package execute

import (
	"math"
	"math/big"
)

// DispatchResult is the envelope subset consumed by the service-independent
// executor loop. Service wiring converts its concrete envelopes at one boundary.
type DispatchResult struct {
	Status        string         `json:"status"`
	Data          map[string]any `json:"data"`
	ErrorClass    string         `json:"error_class"`
	Message       string         `json:"message"`
	RepoRevision  *string        `json:"repo_revision"`
	IndexRevision *string        `json:"index_revision"`
	OperationID   *string        `json:"operation_id"`
}

type RunResult struct {
	Status        string
	Data          map[string]any
	Warnings      []string
	RepoRevision  string
	IndexRevision string
	ErrorClass    string
	Message       string
}

func executeLimit(number interface{ String() string }) int {
	value, ok := new(big.Int).SetString(number.String(), 10)
	if !ok || value.Sign() < 0 {
		return 0
	}
	maximum := big.NewInt(int64(math.MaxInt))
	if value.Cmp(maximum) > 0 {
		return math.MaxInt
	}
	return int(value.Int64())
}
func pointerValue(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
func runFailure(class, message string) RunResult {
	return RunResult{Status: "error", ErrorClass: class, Message: message, Warnings: []string{}}
}
func runSuccess(data map[string]any, warnings []string) RunResult {
	return RunResult{Status: "success", Data: data, Warnings: warnings, RepoRevision: "final", IndexRevision: "index"}
}
func savedOperations(plan Plan) []SavedOperation {
	result := make([]SavedOperation, 0, len(plan.Operations))
	for _, operation := range plan.Operations {
		result = append(result, SavedOperation{operation.Op, operation.SaveAs})
	}
	return result
}
func returnProjections(plan Plan) []ReturnProjection {
	result := make([]ReturnProjection, 0, len(plan.Returns))
	for _, projection := range plan.Returns {
		var limit *int
		if projection.Limit != nil {
			value := executeLimit(*projection.Limit)
			limit = &value
		}
		result = append(result, ReturnProjection{projection.Name, projection.Ref, projection.Fields, limit})
	}
	return result
}
