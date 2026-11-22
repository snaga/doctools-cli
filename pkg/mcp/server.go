package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
)

// ToolHandlerFunc is a handler function for tool calls.
type ToolHandlerFunc func(ctx context.Context, name string, arguments json.RawMessage) (*CallToolResult, error)

// Server is a stdio JSON-RPC 2.0 MCP server.
type Server struct {
	name        string
	version     string
	tools       []Tool
	toolHandler ToolHandlerFunc
	logger      *log.Logger
	mu          sync.Mutex
}

// Option configures Server options.
type Option func(*Server)

// WithName sets the server implementation name.
func WithName(name string) Option {
	return func(s *Server) {
		s.name = name
	}
}

// WithVersion sets the server implementation version.
func WithVersion(version string) Option {
	return func(s *Server) {
		s.version = version
	}
}

// WithTools sets the tools exposed by the server.
func WithTools(tools []Tool) Option {
	return func(s *Server) {
		s.tools = tools
	}
}

// WithToolHandler sets the handler for tool calls.
func WithToolHandler(handler ToolHandlerFunc) Option {
	return func(s *Server) {
		s.toolHandler = handler
	}
}

// WithLogger sets the server logger (defaults to stderr).
func WithLogger(logger *log.Logger) Option {
	return func(s *Server) {
		s.logger = logger
	}
}

// NewServer creates a new MCP server with default configurations.
func NewServer(opts ...Option) *Server {
	s := &Server{
		name:        DefaultServerName,
		version:     DefaultServerVersion,
		tools:       DefaultTools(),
		toolHandler: DefaultDispatcher().HandleToolCall,
		logger:      log.New(os.Stderr, "[mcp] ", log.LstdFlags),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ServeStdio starts the server listening on os.Stdin and writing to os.Stdout.
func (s *Server) ServeStdio() error {
	return s.Serve(context.Background(), os.Stdin, os.Stdout)
}

// ServeStdio starts the default MCP server on standard input/output.
func ServeStdio() error {
	return NewServer().ServeStdio()
}

// Serve reads JSON-RPC 2.0 requests from r and writes responses to w.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	br := bufio.NewReader(r)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := br.ReadBytes('\n')
		if err != nil {
			if len(line) > 0 {
				s.handleMessage(ctx, bytes.TrimSpace(line), w)
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}

		// Support optional Content-Length framing (LSP / legacy compatibility)
		if bytes.HasPrefix(bytes.ToLower(trimmed), []byte("content-length:")) {
			parts := bytes.SplitN(trimmed, []byte(":"), 2)
			if len(parts) == 2 {
				lengthStr := strings.TrimSpace(string(parts[1]))
				length, parseErr := strconv.Atoi(lengthStr)
				if parseErr == nil && length > 0 {
					// Read headers until empty separator line
					for {
						headerLine, hErr := br.ReadBytes('\n')
						if hErr != nil {
							return hErr
						}
						if len(bytes.TrimSpace(headerLine)) == 0 {
							break
						}
					}
					// Read body
					body := make([]byte, length)
					if _, rErr := io.ReadFull(br, body); rErr != nil {
						return rErr
					}
					trimmed = bytes.TrimSpace(body)
				}
			}
		}

		s.handleMessage(ctx, trimmed, w)
	}
}

func (s *Server) handleMessage(ctx context.Context, data []byte, w io.Writer) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		s.logger.Printf("parse error: %v", err)
		resp := &Response{
			JSONRPC: JSONRPCVersion,
			ID:      json.RawMessage("null"),
			Error:   NewRPCError(CodeParseError, "Parse error: "+err.Error()),
		}
		_ = s.writeResponse(w, resp)
		return
	}

	if req.JSONRPC != JSONRPCVersion || req.Method == "" {
		if req.IsNotification() {
			return
		}
		resp := &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   NewRPCError(CodeInvalidRequest, "Invalid request: jsonrpc must be '2.0' and method must not be empty"),
		}
		_ = s.writeResponse(w, resp)
		return
	}

	resp, err := s.handleRequest(ctx, &req)
	if req.IsNotification() {
		// Notifications MUST NOT receive a response in JSON-RPC 2.0.
		return
	}

	if err != nil {
		var rpcErr *RPCError
		if errors.As(err, &rpcErr) {
			resp = &Response{
				JSONRPC: JSONRPCVersion,
				ID:      req.ID,
				Error:   rpcErr,
			}
		} else {
			resp = &Response{
				JSONRPC: JSONRPCVersion,
				ID:      req.ID,
				Error:   NewRPCError(CodeInternalError, err.Error()),
			}
		}
	} else if resp == nil {
		resp = &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result:  map[string]any{},
		}
	}

	if err := s.writeResponse(w, resp); err != nil {
		s.logger.Printf("failed to write response: %v", err)
	}
}

func (s *Server) handleRequest(ctx context.Context, req *Request) (*Response, error) {
	switch req.Method {
	case "initialize":
		var params InitializeParams
		if len(req.Params) > 0 {
			_ = json.Unmarshal(req.Params, &params)
		}
		result := InitializeResult{
			ProtocolVersion: LatestProtocolVersion,
			Capabilities: ServerCapabilities{
				Tools: &ToolsCapability{
					ListChanged: false,
				},
			},
			ServerInfo: Implementation{
				Name:    s.name,
				Version: s.version,
			},
			Instructions: "Agent-native document processing tools for PDF, Excel, and PowerPoint.",
		}
		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result:  result,
		}, nil

	case "notifications/initialized":
		s.logger.Println("client initialized notification received")
		return nil, nil

	case "ping":
		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result:  map[string]any{},
		}, nil

	case "tools/list":
		result := ToolsListResult{
			Tools: s.tools,
		}
		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result:  result,
		}, nil

	case "tools/call":
		var params CallToolParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, NewRPCError(CodeInvalidParams, "Invalid params: "+err.Error())
		}
		if params.Name == "" {
			return nil, NewRPCError(CodeInvalidParams, "Tool name is required")
		}

		if s.toolHandler != nil {
			callResult, err := s.toolHandler(ctx, params.Name, params.Arguments)
			if err != nil {
				return nil, err
			}
			return &Response{
				JSONRPC: JSONRPCVersion,
				ID:      req.ID,
				Result:  callResult,
			}, nil
		}

		// Skeleton handler for TASK-21.1 placeholder
		if params.Name == "doctools_run" {
			return &Response{
				JSONRPC: JSONRPCVersion,
				ID:      req.ID,
				Result: CallToolResult{
					Content: []ToolContent{
						{
							Type: "text",
							Text: "doctools_run placeholder",
						},
					},
					IsError: false,
				},
			}, nil
		}

		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result: CallToolResult{
				Content: []ToolContent{
					{
						Type: "text",
						Text: fmt.Sprintf("unknown tool: %s", params.Name),
					},
				},
				IsError: true,
			},
		}, nil

	default:
		return nil, NewRPCError(CodeMethodNotFound, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func (s *Server) writeResponse(w io.Writer, resp *Response) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := w.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}
