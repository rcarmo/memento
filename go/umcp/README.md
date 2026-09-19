# uMCP for Go

A pure-Go, reusable implementation of the pinned `rcarmo/umcp` v0.2.2 behavioural contracts.

```go
server := umcp.NewServer("example")
server.Tools.Register(umcp.Tool{Name: "hello", /* schema and callback */})
err := server.RunTransport(ctx, []string{"--stdio"}, os.Stdin, os.Stdout, true, umcp.HTTPHooks{})
```

The module contains no Memento service dependencies. It supports dynamic tools, resources/templates/subscriptions, prompts, completion, logging and notifications; stdio, Streamable HTTP, TCP and legacy SSE transports; synchronous and asynchronous raw HTTP parsers/listeners; session/principal/version binding, cancellation and progress; and source-compatible validation/serialization behavior.

Run independently:

```sh
make check
make race
make fuzz
```

Module path: `github.com/rcarmo/memento/go/umcp`.

Behavioral parity targets the Python API's observable protocol behavior through idiomatic Go APIs; it does not claim Python import/class identity. The canonical reference commit and differential evidence are maintained by the parent Memento Go-port test suite.
