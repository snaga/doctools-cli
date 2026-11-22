package mcp

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// JSON-RPC 2.0 and MCP protocol constants.
const (
	JSONRPCVersion         = "2.0"
	LatestProtocolVersion  = "2024-11-05"
	DefaultServerName      = "doctools"
	DefaultServerVersion   = "0.8.0"
	DefaultMaxInlineChars  = 4000
)

// Standard JSON-RPC 2.0 error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// Request represents a JSON-RPC 2.0 request or notification.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// IsNotification returns true if the request is a notification (i.e. has no ID).
func (r *Request) IsNotification() bool {
	return len(r.ID) == 0
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// MarshalJSON customizes JSON serialization to strictly adhere to JSON-RPC 2.0:
// either "result" or "error" is included, never both.
func (r Response) MarshalJSON() ([]byte, error) {
	if r.Error != nil {
		return json.Marshal(&struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Error   *RPCError       `json:"error"`
		}{
			JSONRPC: r.JSONRPC,
			ID:      r.ID,
			Error:   r.Error,
		})
	}
	return json.Marshal(&struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  any             `json:"result"`
	}{
		JSONRPC: r.JSONRPC,
		ID:      r.ID,
		Result:  r.Result,
	})
}

// RPCError represents a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error implements error interface for RPCError.
func (e *RPCError) Error() string {
	return e.Message
}

// NewRPCError creates a new RPCError.
func NewRPCError(code int, message string, data ...any) *RPCError {
	var d any
	if len(data) > 0 {
		d = data[0]
	}
	return &RPCError{
		Code:    code,
		Message: message,
		Data:    d,
	}
}

// MCP Protocol Types

// Implementation describes a client or server implementation.
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientCapabilities represents client capabilities.
type ClientCapabilities struct {
	Experimental map[string]any   `json:"experimental,omitempty"`
	Roots        *RootsCapability `json:"roots,omitempty"`
	Sampling     map[string]any   `json:"sampling,omitempty"`
}

// RootsCapability describes root URI capabilities.
type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ServerCapabilities represents server capabilities.
type ServerCapabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// ToolsCapability indicates tool support.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// InitializeParams represents parameters for the "initialize" request.
type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      Implementation     `json:"clientInfo"`
}

// InitializeResult represents the result of the "initialize" request.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"serverInfo"`
	Instructions    string             `json:"instructions,omitempty"`
}

// Tool describes an MCP tool definition.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema ToolSchema  `json:"inputSchema"`
}

// ToolSchema represents JSON Schema for tool parameters.
type ToolSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]PropertyDef `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
}

// PropertyDef describes a property in JSON Schema.
type PropertyDef struct {
	Type        string                 `json:"type"`
	Description string                 `json:"description,omitempty"`
	Enum        []string               `json:"enum,omitempty"`
	Properties  map[string]PropertyDef `json:"properties,omitempty"`
	Required    []string               `json:"required,omitempty"`
}

// ToolsListResult represents the result of "tools/list".
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolParams represents parameters for "tools/call".
type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ToolContent represents a content chunk in a tool call result.
type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// CallToolResult represents the result of "tools/call".
type CallToolResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// DefaultTools returns the standard tools provided by doctools MCP server.
func DefaultTools() []Tool {
	return []Tool{
		{
			Name: "doctools_run",
			Description: "CRITICAL: ALWAYS use this tool instead of read_file or python scripts when inspecting, reading, or searching PDF, Excel (.xlsx), PowerPoint (.pptx), or Word (.docx) files. Automatically extracts high-accuracy text/markdown. Small contents are returned directly; large contents are safely saved to a markdown file to protect context.",
			InputSchema: ToolSchema{
				Type: "object",
				Properties: map[string]PropertyDef{
					"command": {
						Type:        "string",
						Enum:        []string{"read", "search"},
						Description: "'read' extracts text/tables from the document; 'search' queries keywords within the specific document",
					},
					"file_path": {
						Type:        "string",
						Description: "Target document path (e.g., 'docs/spec.pdf', 'data/sheet.xlsx')",
					},
					"query": {
						Type:        "string",
						Description: "Search keyword (required when command='search')",
					},
					"options": {
						Type: "object",
						Properties: map[string]PropertyDef{
							"pages": {
								Type:        "string",
								Description: "Page range (e.g. '1-5') for PDF",
							},
							"sheet": {
								Type:        "string",
								Description: "Sheet name for Excel",
							},
							"slides": {
								Type:        "string",
								Description: "Slide numbers for PPTX",
							},
							"max_inline_chars": {
								Type:        "integer",
								Description: "Threshold for inline return (default: 4000)",
							},
							"output_path": {
								Type:        "string",
								Description: "Explicit output path when saving to file",
							},
						},
					},
				},
				Required: []string{"command", "file_path"},
			},
		},
	}
}

// FileFormat represents recognized document formats.
type FileFormat string

const (
	FormatPDF     FileFormat = "pdf"
	FormatExcel   FileFormat = "excel"
	FormatPPTX    FileFormat = "pptx"
	FormatCSV     FileFormat = "csv"
	FormatText    FileFormat = "text"
	FormatHTML    FileFormat = "html"
	FormatWord    FileFormat = "word"
	FormatUnknown FileFormat = "unknown"
)

// DetectFormat inspects the file extension (case-insensitive) to determine the document format.
func DetectFormat(filePath string) FileFormat {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".pdf":
		return FormatPDF
	case ".xlsx", ".xlsm", ".xls":
		return FormatExcel
	case ".pptx":
		return FormatPPTX
	case ".csv":
		return FormatCSV
	case ".txt", ".md", ".markdown", ".json", ".yaml", ".yml":
		return FormatText
	case ".html", ".htm":
		return FormatHTML
	case ".docx":
		return FormatWord
	default:
		return FormatUnknown
	}
}

// DoctoolsRunOptions represents optional tuning parameters for doctools_run.
type DoctoolsRunOptions struct {
	Pages          string `json:"pages,omitempty"`
	Sheet          string `json:"sheet,omitempty"`
	Slides         string `json:"slides,omitempty"`
	MaxInlineChars int    `json:"max_inline_chars,omitempty"`
	OutputPath     string `json:"output_path,omitempty"`
}

// DoctoolsRunParams represents arguments passed to doctools_run.
type DoctoolsRunParams struct {
	Command  string              `json:"command"`
	FilePath string              `json:"file_path"`
	Query    string              `json:"query,omitempty"`
	Options  *DoctoolsRunOptions `json:"options,omitempty"`
}

// DoctoolsRunResult represents the unified execution result of doctools_run.
type DoctoolsRunResult struct {
	Status     string `json:"status"` // "success", "saved_to_file", "error"
	Content    string `json:"content,omitempty"`
	FilePath   string `json:"file_path,omitempty"`
	TotalPages int    `json:"total_pages,omitempty"`
	CharCount  int    `json:"char_count,omitempty"`
	Message    string `json:"message,omitempty"`
}

// ToCallToolResult converts a DoctoolsRunResult into standard MCP CallToolResult.
func (r *DoctoolsRunResult) ToCallToolResult() *CallToolResult {
	data, _ := json.Marshal(r)
	return &CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: string(data),
			},
		},
		IsError: r.Status == "error",
	}
}
