package execute

// ValidateProposalChanges reuses the source-derived strict propose union without
// requiring callers to construct the surrounding memory_propose request.
func ValidateProposalChanges(value any) ([]map[string]any, error) {
	return validateProposalChanges(value, NewArguments)
}
func validateProposalChanges(value any, construct func() (*Arguments, error)) ([]map[string]any, error) {
	arguments, err := construct()
	if err != nil {
		return nil, err
	}
	normalized, err := arguments.Validate("propose", map[string]any{"intent": "dream", "base_revision": "revision", "changes": value}, true)
	if err != nil {
		return nil, err
	}
	rows := normalized["changes"].([]any)
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.(map[string]any))
	}
	return out, nil
}
