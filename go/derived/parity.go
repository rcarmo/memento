package derived

import (
	"context"
	"reflect"
)

type ParityReport struct {
	Matches          bool   `json:"matches"`
	ExpectedRevision string `json:"expected_revision"`
	CurrentRevision  string `json:"current_revision"`
	Details          string `json:"details"`
}

var parityQueries = []string{
	`SELECT id,path,type,title,description,status,tags_json,aliases_json,body,content_hash,updated_at,repo_revision FROM concepts ORDER BY path`,
	`SELECT source_id,target_id,raw_target,target_path,anchor,link_kind,resolution_state FROM links ORDER BY source_id,raw_target,anchor`,
	`SELECT concept_id,inbound_degree,outbound_degree,broken_link_count,orphan_flag FROM graph_metrics ORDER BY concept_id`,
	`SELECT concept_id,path,embedding_text_hash,model_id,dimensions,embedding_revision,status,model_revision,embedding_norm,error_message FROM concept_embeddings ORDER BY path`}

func (i *Index) ParityCheck(ctx context.Context, clean *Index, expected string) (ParityReport, error) {
	currentState, err := i.State(ctx)
	if err != nil {
		return ParityReport{}, err
	}
	current, err := i.normalizedDump(ctx)
	if err != nil {
		return ParityReport{}, err
	}
	other, err := clean.normalizedDump(ctx)
	if err != nil {
		return ParityReport{}, err
	}
	matches := reflect.DeepEqual(current, other)
	details := "normalized derived state differs"
	if matches {
		details = "match"
	}
	return ParityReport{matches, expected, currentState.IndexRevision, details}, nil
}
func (i *Index) normalizedDump(ctx context.Context) ([][][]any, error) {
	var result [][][]any
	err := i.withCore(ctx, false, func(store ContentStore) error {
		var err error
		result, err = runParityQueries(func(query string) (parityRows, func(), error) {
			rows, e := store.DB.QueryContext(ctx, query)
			return rows, func() { rows.Close() }, e
		})
		return err
	})
	return result, err
}
func runParityQueries(query func(string) (parityRows, func(), error)) ([][][]any, error) {
	result := [][][]any{}
	for _, statement := range parityQueries {
		rows, close, err := query(statement)
		if err != nil {
			return nil, err
		}
		values, err := dumpRows(rows)
		close()
		if err != nil {
			return nil, err
		}
		result = append(result, values)
	}
	return result, nil
}

type parityRows interface {
	Columns() ([]string, error)
	Next() bool
	Scan(...any) error
	Err() error
}

func dumpRows(rows parityRows) ([][]any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := [][]any{}
	for rows.Next() {
		values := make([]any, len(columns))
		targets := make([]any, len(columns))
		for i := range values {
			targets[i] = &values[i]
		}
		if err = rows.Scan(targets...); err != nil {
			return nil, err
		}
		for i, value := range values {
			if raw, ok := value.([]byte); ok {
				values[i] = string(raw)
			}
		}
		result = append(result, values)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
