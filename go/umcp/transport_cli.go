package umcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
)

// TransportConfig is the pinned uMCP CLI selection. Python integers are kept
// unbounded at parsing time; socket/resource range validation happens on start.
type TransportConfig struct {
	Mode, Host, Endpoint, File string
	Port                       *big.Int
	MaxRequestBytes            *big.Int
	AllowedOrigins             []string
}

func cliInteger(text string) (*big.Int, error) {
	normalized := normalizeDecimalDigits(strings.TrimSpace(text))
	if !pythonInt.MatchString(normalized) {
		return nil, fmt.Errorf("invalid literal for int() with base 10: %s", pythonRepr(text))
	}
	normalized = strings.ReplaceAll(normalized, "_", "")
	digits := len(strings.TrimLeft(normalized, "+-"))
	if digits > 4300 {
		return nil, fmt.Errorf("Exceeds the limit (4300 digits) for integer string conversion: value has %d digits; use sys.set_int_max_str_digits() to increase the limit", digits)
	}
	number, _ := new(big.Int).SetString(normalized, 10)
	return number, nil
}

// ParseTransportArgs preserves source order/conflicts and unknown-argument
// handling. Unknown or incomplete flags become positional file arguments;
// network mode ignores them. A bare --port selects legacy SSE, not HTTP.
func ParseTransportArgs(args []string) (TransportConfig, error) {
	config := TransportConfig{Host: "127.0.0.1", Endpoint: "/mcp", MaxRequestBytes: big.NewInt(4 << 20), AllowedOrigins: []string{}}
	remaining := []string{}
	selected := false
	for i := 0; i < len(args); i++ {
		flag := args[i]
		value := ""
		hasValue := i+1 < len(args)
		if hasValue {
			value = args[i+1]
		}
		switch {
		case (flag == "--port" || flag == "-p" || flag == "--max-request-bytes") && hasValue:
			number, err := cliInteger(value)
			if err != nil {
				return config, err
			}
			if flag == "--max-request-bytes" {
				config.MaxRequestBytes = number
			} else {
				config.Port = number
			}
			i++
		case (flag == "--host" || flag == "--endpoint" || flag == "--allowed-origin") && hasValue:
			switch flag {
			case "--host":
				config.Host = value
			case "--endpoint":
				config.Endpoint = value
			default:
				config.AllowedOrigins = append(config.AllowedOrigins, value)
			}
			i++
		case flag == "--transport" && hasValue || flag == "--tcp" || flag == "--http" || flag == "--sse":
			mode := value
			if flag == "--transport" {
				i++
			} else {
				mode = map[string]string{"--tcp": "tcp", "--http": "streamable-http", "--sse": "sse"}[flag]
			}
			if selected && config.Mode != mode {
				return config, errors.New("conflicting transport options")
			}
			config.Mode = mode
			selected = true
		default:
			remaining = append(remaining, flag)
		}
	}
	if selected && config.Mode != "stdio" && config.Mode != "tcp" && config.Mode != "sse" && config.Mode != "streamable-http" {
		return config, fmt.Errorf("unsupported transport: %s", config.Mode)
	}
	if selected && config.Mode != "stdio" && config.Port == nil {
		return config, errors.New("network transports require --port")
	}
	if config.Mode == "stdio" && config.Port != nil {
		return config, errors.New("stdio transport cannot use --port")
	}
	if config.Port != nil {
		if !selected {
			config.Mode = "sse"
		}
	} else if len(remaining) > 0 {
		config.Mode = "file"
		config.File = remaining[0]
	} else {
		config.Mode = "stdio"
	}
	return config, nil
}

type transportOutput struct {
	mu     sync.Mutex
	output io.Writer
}

func (w *transportOutput) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.output.Write(p)
}
func (w *transportOutput) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if f, ok := w.output.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	return nil
}

// RunTransport applies uMCP arguments to the composed Server, not Memento's
// operational CLI. The caller owns streams and supplies signal cancellation.
// File/stdio blocked reads require caller closure. Network listeners are owned
// and closed here; all connection workers drain cooperatively on cancellation.
// Asynchronous selects source framing semantics, not Go scheduling: both modes
// serve independent connections concurrently. OS error text is platform-specific.
func (s *Server) RunTransport(ctx context.Context, args []string, input io.Reader, output io.Writer, asynchronous bool, hooks HTTPHooks) error {
	config, err := ParseTransportArgs(args)
	if err != nil {
		return err
	}
	if output == nil {
		return errors.New("transport output is required")
	}
	if config.Mode == "streamable-http" {
		if err := s.StreamableHTTP.validate(); err != nil {
			return err
		}
	}
	out := &transportOutput{output: output}
	// This entrypoint owns the transport sink. A previous HTTP run must not
	// suppress notifications when the same server is restarted on stdio.
	s.SetNotifier(nil)
	s.SetNotificationOutput(out)
	if config.Mode == "file" {
		return s.ProcessFile(ctx, config.File, out)
	}
	if config.Mode == "stdio" {
		if input == nil {
			return errors.New("stdio input is required")
		}
		return s.serveStdio(ctx, input, out)
	}
	if !config.Port.IsInt64() || config.Port.Sign() < 0 || config.Port.Int64() > 65535 {
		return errors.New("bind(): port must be 0-65535")
	}
	var handler interface {
		ServeHTTP(http.ResponseWriter, *http.Request)
		Close()
	}
	// Typed handler options intentionally reject impossible resource settings;
	// parsing still retains source values for compatibility and diagnostics.
	local := config.Host == "127.0.0.1" || config.Host == "localhost" || config.Host == "::1"
	if config.Mode != "tcp" {
		if !config.MaxRequestBytes.IsInt64() {
			return errors.New("HTTP request limit exceeds int64")
		}
		if config.Mode == "streamable-http" {
			options := s.streamableHTTPOptions(config, local, asynchronous)
			handler, err = NewStreamableHTTP(s, options, hooks)
		} else {
			mode := SyncSSE
			if asynchronous {
				mode = AsyncSSE
			}
			options := DefaultSSEOptions(mode)
			options.MaxRequestBytes = config.MaxRequestBytes.Int64()
			options.LocalBind = local
			options.AllowedOrigins = config.AllowedOrigins
			handler, err = NewLegacySSE(s, options, hooks)
		}
		if err != nil {
			return err
		}
		defer handler.Close()
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(config.Host, config.Port.String()))
	if err != nil {
		return err
	}
	defer listener.Close()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	address := host + ":" + port
	message := "Listening on " + address
	if config.Mode == "streamable-http" {
		message = "MCP Streamable HTTP Server listening on http://" + address + config.Endpoint
	} else if config.Mode == "sse" {
		message = "MCP SSE Server listening on http://" + address + "/sse"
	}
	if _, err = fmt.Fprintln(out, message); err != nil {
		return err
	}
	if err = out.Flush(); err != nil {
		return err
	}
	if config.Mode == "tcp" {
		mode := SyncTCP
		if asynchronous {
			mode = AsyncTCP
		}
		return s.ServeTCP(ctx, listener, DefaultTCPOptions(mode))
	}
	if !asynchronous {
		return ServeSyncHTTP(ctx, listener, handler, s.syncHTTPOptions(config.Mode))
	}
	mode := AsyncStreamableParser
	if config.Mode == "sse" {
		mode = AsyncSSEParser
	}
	options := s.asyncHTTPOptions(mode, config.Mode, config.MaxRequestBytes.Int64())
	options.Parser.LocalBind = local
	options.Parser.AllowedOrigins = config.AllowedOrigins
	return ServeAsyncHTTP(ctx, listener, handler, options)
}

func (s *Server) streamableHTTPOptions(config TransportConfig, local, asynchronous bool) HTTPOptions {
	options := DefaultHTTPOptions()
	options.AsyncReference = asynchronous
	options.Endpoint = config.Endpoint
	options.MaxRequestBytes = config.MaxRequestBytes.Int64()
	options.LocalBind = local
	options.AllowedOrigins = config.AllowedOrigins
	options.SessionTTL = s.StreamableHTTP.SessionTTL
	options.MaxSessions = s.StreamableHTTP.MaxSessions
	options.Keepalive = s.StreamableHTTP.Keepalive
	return options
}
func (s *Server) syncHTTPOptions(mode string) SyncHTTPConnectionOptions {
	options := DefaultSyncHTTPConnectionOptions(mode == "sse")
	if mode == "streamable-http" {
		options.IOTimeout = s.StreamableHTTP.RequestTimeout
		options.MaxRequests = s.StreamableHTTP.MaxRequestsConnection
	}
	return options
}
func (s *Server) asyncHTTPOptions(parser AsyncHTTPParserMode, mode string, maxBytes int64) AsyncHTTPConnectionOptions {
	options := DefaultAsyncHTTPConnectionOptions(parser)
	options.Parser.MaxRequestBytes = maxBytes
	if mode == "streamable-http" {
		options.Parser.ReadTimeout = s.StreamableHTTP.RequestTimeout
		options.MaxRequests = s.StreamableHTTP.MaxRequestsConnection
	}
	return options
}

// TransportExitCode gives host CLIs the source success/failure convention while
// keeping process exit and signal ownership outside the reusable package.
func TransportExitCode(err error) int {
	if err == nil || errors.Is(err, context.Canceled) {
		return 0
	}
	return 1
}
