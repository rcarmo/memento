package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rcarmo/memento/go/control"
	"io"
	"strings"
)

func ParseModelAnswer(response ModelResponse, source string) (AnswerRecord, error) {
	d := json.NewDecoder(strings.NewReader(response.OutputText))
	d.UseNumber()
	var object map[string]any
	if err := d.Decode(&object); err != nil {
		return AnswerRecord{}, fmt.Errorf("model output is not valid JSON: %w", err)
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return AnswerRecord{}, errors.New("model output is not valid JSON: trailing JSON")
	}
	answer := UnknownAnswer
	if v, ok := object["answer"]; ok {
		answer = fmt.Sprint(v)
	}
	confidence := "low"
	if v, ok := object["confidence"]; ok {
		confidence = fmt.Sprint(v)
	}
	unresolved, err := answerStrings(object["unresolved"])
	if err != nil {
		return AnswerRecord{}, err
	}
	citations, err := answerCitations(object["citations"])
	if err != nil {
		return AnswerRecord{}, err
	}
	chain := append([]control.ModelAttempt{}, response.ModelChain...)
	if len(chain) == 0 {
		chain, err = answerModelChain(object["model_chain"])
		if err != nil {
			return AnswerRecord{}, err
		}
	}
	return AnswerRecord{answer, source, confidence, unresolved, citations, nil, nil, chain}, nil
}
func answerStrings(raw any) ([]string, error) {
	if raw == nil {
		return []string{}, nil
	}
	values, ok := raw.([]any)
	if !ok {
		return nil, errors.New("unresolved must be an array")
	}
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = fmt.Sprint(v)
	}
	return out, nil
}
func answerCitations(raw any) ([]AnswerCitation, error) {
	if raw == nil {
		return []AnswerCitation{}, nil
	}
	values, ok := raw.([]any)
	if !ok {
		return nil, errors.New("citations must be an array")
	}
	out := make([]AnswerCitation, 0, len(values))
	for _, value := range values {
		row, ok := value.(map[string]any)
		if !ok || len(row) != 3 {
			return nil, errors.New("citation fields are invalid")
		}
		id, iok := row["id"].(string)
		path, pok := row["path"].(string)
		revision, rok := row["revision"].(string)
		if !iok || !pok || !rok {
			return nil, errors.New("citation fields must be strings")
		}
		out = append(out, AnswerCitation{id, path, revision})
	}
	return out, nil
}
func answerModelChain(raw any) ([]control.ModelAttempt, error) {
	if raw == nil {
		return []control.ModelAttempt{}, nil
	}
	values, ok := raw.([]any)
	if !ok {
		return nil, errors.New("model_chain must be a sequence")
	}
	out := make([]control.ModelAttempt, 0, len(values))
	for _, value := range values {
		if name, ok := value.(string); ok {
			out = append(out, control.ModelAttempt{Model: name, Outcome: "success"})
			continue
		}
		row, ok := value.(map[string]any)
		if !ok {
			return nil, errors.New("model_chain entry must be a string or object")
		}
		model, mok := row["model"].(string)
		outcome, ook := row["outcome"].(string)
		if !mok || !ook || len(row) != 2 {
			return nil, errors.New("model_chain entry fields are invalid")
		}
		out = append(out, control.ModelAttempt{Model: model, Outcome: outcome})
	}
	return out, nil
}
func ValidateAnswerCitations(record AnswerRecord, concepts []AnswerReadConcept, revision, source string) AnswerRecord {
	record.TraceID = nil
	if record.Answer == UnknownAnswer {
		record.Citations = []AnswerCitation{}
		return record
	}
	byID, byPath := map[string]AnswerReadConcept{}, map[string]AnswerReadConcept{}
	for _, c := range concepts {
		byID[c.ID] = c
		byPath[c.Path] = c
	}
	repaired := make([]AnswerCitation, 0, len(record.Citations))
	for _, citation := range record.Citations {
		a, aok := byID[citation.ID]
		b, bok := byPath[citation.Path]
		if !aok || !bok || a.ID != b.ID || citation.Revision != revision {
			return invalidAnswer(record, source, "citation_validation_failed")
		}
		repaired = append(repaired, AnswerCitation{a.ID, a.Path, revision})
	}
	if len(repaired) == 0 {
		return invalidAnswer(record, source, "missing_citations")
	}
	record.Citations = repaired
	return record
}
func invalidAnswer(record AnswerRecord, source, reason string) AnswerRecord {
	return AnswerRecord{UnknownAnswer, source, "low", []string{reason}, []AnswerCitation{}, nil, nil, record.ModelChain}
}
