package main

import (
	"context"
	"fmt"
	"github.com/rcarmo/memento/go/umcp"
	"os"
)

func main() {
	server := umcp.NewServer("hello")
	err := server.Tools.Register(umcp.Tool{Name: "hello", Description: "Return a greeting", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}}}, Parameters: []umcp.Parameter{{Name: "name", Types: []umcp.ParamType{umcp.StringParam}, Default: "world", HasDefault: true}}, Call: func(_ context.Context, args map[string]any) (any, error) {
		return map[string]any{"greeting": fmt.Sprintf("Hello, %s!", args["name"])}, nil
	}})
	if err != nil {
		panic(err)
	}
	if err = server.RunTransport(context.Background(), os.Args[1:], os.Stdin, os.Stdout, true, umcp.HTTPHooks{}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
