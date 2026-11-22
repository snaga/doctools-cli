package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ReadHandlerFunc handles read commands for a specific file format.
type ReadHandlerFunc func(ctx context.Context, format FileFormat, params DoctoolsRunParams) (*DoctoolsRunResult, error)

// SearchHandlerFunc handles search commands for a specific file format.
type SearchHandlerFunc func(ctx context.Context, format FileFormat, params DoctoolsRunParams) (*DoctoolsRunResult, error)

// Dispatcher coordinates doctools_run execution by detecting format and routing to handlers.
type Dispatcher struct {
	mu             sync.RWMutex
	readHandlers   map[FileFormat]ReadHandlerFunc
	searchHandlers map[FileFormat]SearchHandlerFunc
}

var (
	defaultDispatcherInstance *Dispatcher
	defaultDispatcherOnce     sync.Once
)

// DefaultDispatcher returns the singleton dispatcher instance.
func DefaultDispatcher() *Dispatcher {
	defaultDispatcherOnce.Do(func() {
		defaultDispatcherInstance = NewDispatcher()
	})
	return defaultDispatcherInstance
}

// HandleDoctoolsRun is a convenience package-level function delegating to DefaultDispatcher.
func HandleDoctoolsRun(ctx context.Context, params DoctoolsRunParams) (*DoctoolsRunResult, error) {
	return DefaultDispatcher().Handle(ctx, params)
}

// NewDispatcher creates a new Dispatcher with default read handlers registered.
func NewDispatcher() *Dispatcher {
	d := &Dispatcher{
		readHandlers:   make(map[FileFormat]ReadHandlerFunc),
		searchHandlers: make(map[FileFormat]SearchHandlerFunc),
	}
	RegisterDefaultReadHandlers(d)
	return d
}

// RegisterReadHandler registers a custom read handler for a file format.
func (d *Dispatcher) RegisterReadHandler(format FileFormat, h ReadHandlerFunc) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.readHandlers[format] = h
}

// RegisterSearchHandler registers a custom search handler for a file format.
func (d *Dispatcher) RegisterSearchHandler(format FileFormat, h SearchHandlerFunc) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.searchHandlers[format] = h
}

// Handle validates parameters, detects format, and executes the appropriate handler.
func (d *Dispatcher) Handle(ctx context.Context, params DoctoolsRunParams) (*DoctoolsRunResult, error) {
	trimmedPath := strings.TrimSpace(params.FilePath)
	if trimmedPath == "" {
		return &DoctoolsRunResult{
			Status:  "error",
			Message: "file_path is required",
		}, nil
	}

	info, err := os.Stat(trimmedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &DoctoolsRunResult{
				Status:   "error",
				FilePath: trimmedPath,
				Message:  fmt.Sprintf("file not found: %s", trimmedPath),
			}, nil
		}
		return &DoctoolsRunResult{
			Status:   "error",
			FilePath: trimmedPath,
			Message:  fmt.Sprintf("failed to access file %s: %v", trimmedPath, err),
		}, nil
	}

	if info.IsDir() {
		return &DoctoolsRunResult{
			Status:   "error",
			FilePath: trimmedPath,
			Message:  fmt.Sprintf("path is a directory, not a file: %s", trimmedPath),
		}, nil
	}

	cmd := strings.ToLower(strings.TrimSpace(params.Command))
	if cmd != "read" && cmd != "search" {
		return &DoctoolsRunResult{
			Status:   "error",
			FilePath: trimmedPath,
			Message:  fmt.Sprintf("invalid command %q: must be 'read' or 'search'", params.Command),
		}, nil
	}

	if cmd == "search" && strings.TrimSpace(params.Query) == "" {
		return &DoctoolsRunResult{
			Status:   "error",
			FilePath: trimmedPath,
			Message:  "query is required when command is 'search'",
		}, nil
	}

	format := DetectFormat(trimmedPath)
	if format == FormatUnknown {
		return &DoctoolsRunResult{
			Status:   "error",
			FilePath: trimmedPath,
			Message:  fmt.Sprintf("unsupported file extension %q for file: %s", filepath.Ext(trimmedPath), trimmedPath),
		}, nil
	}

	if format == FormatWord {
		return &DoctoolsRunResult{
			Status:   "error",
			FilePath: trimmedPath,
			Message:  "Word (.docx) format is not yet supported in this version",
		}, nil
	}

	if params.Options == nil {
		params.Options = &DoctoolsRunOptions{MaxInlineChars: DefaultMaxInlineChars}
	} else if params.Options.MaxInlineChars <= 0 {
		params.Options.MaxInlineChars = DefaultMaxInlineChars
	}
	params.FilePath = trimmedPath
	params.Command = cmd

	switch cmd {
	case "read":
		return d.dispatchRead(ctx, format, params)
	case "search":
		return d.dispatchSearch(ctx, format, params)
	default:
		return &DoctoolsRunResult{
			Status:   "error",
			FilePath: trimmedPath,
			Message:  fmt.Sprintf("invalid command %q: must be 'read' or 'search'", params.Command),
		}, nil
	}
}

func (d *Dispatcher) dispatchRead(ctx context.Context, format FileFormat, params DoctoolsRunParams) (*DoctoolsRunResult, error) {
	d.mu.RLock()
	handler, ok := d.readHandlers[format]
	d.mu.RUnlock()

	if ok && handler != nil {
		return handler(ctx, format, params)
	}

	// Default placeholder handler until TASK-21.3
	content := fmt.Sprintf("read placeholder for %s (%s)", params.FilePath, format)
	return &DoctoolsRunResult{
		Status:    "success",
		Content:   content,
		FilePath:  params.FilePath,
		CharCount: len(content),
		Message:   "document read placeholder",
	}, nil
}

func (d *Dispatcher) dispatchSearch(ctx context.Context, format FileFormat, params DoctoolsRunParams) (*DoctoolsRunResult, error) {
	d.mu.RLock()
	handler, ok := d.searchHandlers[format]
	d.mu.RUnlock()

	if ok && handler != nil {
		return handler(ctx, format, params)
	}

	// Default placeholder handler until TASK-21.4
	content := fmt.Sprintf("search placeholder for %s (%s, query: %q)", params.FilePath, format, params.Query)
	return &DoctoolsRunResult{
		Status:   "success",
		Content:  content,
		FilePath: params.FilePath,
		Message:  "document search placeholder",
	}, nil
}

// HandleToolCall bridges JSON-RPC tools/call requests to doctools_run dispatcher.
func (d *Dispatcher) HandleToolCall(ctx context.Context, name string, arguments json.RawMessage) (*CallToolResult, error) {
	if name != "doctools_run" {
		return &CallToolResult{
			Content: []ToolContent{
				{
					Type: "text",
					Text: fmt.Sprintf("unknown tool: %s", name),
				},
			},
			IsError: true,
		}, nil
	}

	var params DoctoolsRunParams
	if len(arguments) > 0 {
		if err := json.Unmarshal(arguments, &params); err != nil {
			return &CallToolResult{
				Content: []ToolContent{
					{
						Type: "text",
						Text: fmt.Sprintf("invalid arguments JSON: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
	}

	res, err := d.Handle(ctx, params)
	if err != nil {
		return nil, err
	}
	return res.ToCallToolResult(), nil
}
