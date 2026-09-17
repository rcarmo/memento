package umcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// RPCError has the existing JSON-RPC code/message and optional data shape.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Response preserves IDs as raw JSON, including arbitrary-precision integers.
// A nil *Response means the transport acknowledges without a JSON-RPC reply.
type Response struct {
	ID     json.RawMessage
	Result any
	Error  *RPCError
}

// MarshalJSON distinguishes a null result from the absence of a result on errors.
func (r Response) MarshalJSON() ([]byte, error) {
	id := r.ID
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	if r.Error != nil {
		return json.Marshal(struct {
			Version string          `json:"jsonrpc"`
			Error   *RPCError       `json:"error"`
			ID      json.RawMessage `json:"id"`
		}{"2.0", r.Error, id})
	}
	return json.Marshal(struct {
		Version string          `json:"jsonrpc"`
		Result  any             `json:"result"`
		ID      json.RawMessage `json:"id"`
	}{"2.0", r.Result, id})
}

// RequestContext contains transport-authenticated metadata, never tool arguments.
// Its Headers copy is isolated per dispatch, like Python's MappingProxyType.
type RequestContext struct {
	Transport       string
	RequestID       json.RawMessage
	ProgressToken   json.RawMessage
	ProtocolVersion string
	SessionID       string
	Principal       string
	Peer            string
	Headers         map[string]string
}

type requestContextKey struct{}

// Context returns a fresh copy so one handler cannot mutate another's metadata.
func Context(ctx context.Context) RequestContext {
	value, _ := ctx.Value(requestContextKey{}).(RequestContext)
	return cloneContext(value)
}

func cloneContext(value RequestContext) RequestContext {
	value.RequestID = append(json.RawMessage(nil), value.RequestID...)
	value.ProgressToken = append(json.RawMessage(nil), value.ProgressToken...)
	headers := make(map[string]string, len(value.Headers))
	for key, item := range value.Headers {
		headers[key] = item
	}
	value.Headers = headers
	return value
}

// ErrRequestCancelled is the explicit handler signal for uMCP's -32800 reply.
// A handler must return it; request dispatch does not preempt committed work.
var ErrRequestCancelled = errors.New("Request cancelled")

// Handler supplies an implemented method. Discovery and protocol transports are
// not provided by this dispatcher; unknown methods return the source error.
type Handler func(context.Context, map[string]any) (any, *RPCError, error)

// Dispatcher implements the common sync/async request-validation order. Register
// only implemented handlers. Freeze Handlers before concurrent use.
type Dispatcher struct {
	Handlers map[string]Handler
	Cancel   func(json.RawMessage)
	Notify   func(string, map[string]any) error
	registry Registry
}

func failure(id json.RawMessage, code int, message string) *Response {
	return &Response{ID: id, Error: &RPCError{Code: code, Message: message}}
}

// Process matches process_request's envelope/context rules. Handler errors other
// than ErrRequestCancelled propagate, as the source does; method handlers own
// method-specific error mapping and remote exception redaction.
func (d *Dispatcher) Process(ctx context.Context, input []byte, trusted RequestContext) (*Response, error) {
	value, ok := decode(input)
	if !ok {
		return failure(nil, -32700, "Parse error"), nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return failure(nil, -32600, "Invalid Request: top-level JSON value must be an object"), nil
	}
	// Keep the raw ID, not a float64-converted value. The input is already valid.
	raw := map[string]json.RawMessage{}
	_ = json.Unmarshal(input, &raw)
	id := raw["id"]
	paramsValue := object["params"]
	params := map[string]any{}
	if paramsValue != nil {
		params, ok = paramsValue.(map[string]any)
		if !ok {
			return failure(id, -32602, "Invalid params: expected an object"), nil
		}
	}
	var progress json.RawMessage
	if meta, ok := params["_meta"].(map[string]any); ok && meta["progressToken"] != nil {
		if !validID(meta["progressToken"]) {
			return failure(id, -32602, "Invalid params: '_meta.progressToken' must be a string or integer"), nil
		}
		progress, _ = json.Marshal(meta["progressToken"])
	}
	if object["jsonrpc"] != "2.0" {
		return failure(id, -32600, "Invalid Request: Not a JSON-RPC 2.0 request"), nil
	}
	if len(id) > 0 && !validID(object["id"]) {
		return failure(nil, -32600, "Invalid Request: invalid id"), nil
	}
	methodValue := object["method"]
	_, result := object["result"]
	_, rpcError := object["error"]
	if methodValue == nil && (result || rpcError) {
		if ValidJSONRPCResponse(input) {
			return nil, nil
		}
		return failure(id, -32600, "Invalid Request: malformed response"), nil
	}
	method, ok := methodValue.(string)
	if !ok {
		return failure(id, -32600, "Invalid Request: method must be a string"), nil
	}
	metadata := cloneContext(trusted)
	metadata.RequestID = append(json.RawMessage(nil), id...)
	metadata.ProgressToken = progress
	ctx = context.WithValue(ctx, requestContextKey{}, metadata)
	var state *cancellation
	if object["id"] != nil {
		state = d.registry.register(id, progress)
		defer d.registry.cleanup(id, progress, state)
	}
	ctx = context.WithValue(ctx, runtimeKey{}, requestRuntime{progressToken: progress, cancellation: state, notify: d.Notify})
	if method == "notifications/cancelled" {
		cancelID := params["requestId"]
		if !validID(cancelID) {
			if object["id"] == nil {
				return nil, nil
			}
			return failure(id, -32602, "Invalid params: 'requestId' must be a string, integer, or null"), nil
		}
		if cancelID != nil {
			encoded, _ := json.Marshal(cancelID)
			d.registry.Cancel(encoded)
			if d.Cancel != nil {
				d.Cancel(encoded)
			}
		}
		return nil, nil
	}
	if method == "notifications/initialized" {
		return nil, nil
	}
	handler := d.Handlers[method]
	if handler == nil {
		return failure(id, -32601, fmt.Sprintf("Method not found: %s", method)), nil
	}
	output, handlerError, err := handler(ctx, params)
	if errors.Is(err, ErrRequestCancelled) {
		if object["id"] == nil {
			return nil, nil
		}
		return failure(id, -32800, "Request cancelled"), nil
	}
	if err != nil {
		return nil, err
	}
	return &Response{ID: id, Result: output, Error: handlerError}, nil
}

// RemoteSafeFailure redacts runtime details on network transports, not stdio.
func RemoteSafeFailure(ctx context.Context, public, detailed string) string {
	switch Context(ctx).Transport {
	case "streamable-http", "sse", "tcp":
		return public
	default:
		return detailed
	}
}
