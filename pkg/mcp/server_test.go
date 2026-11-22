package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"doctools-cli/pkg/mcp"
)

func executeRPC(t *testing.T, s *mcp.Server, requestJSON string) string {
	t.Helper()
	in := strings.NewReader(requestJSON + "\n")
	var out bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := s.Serve(ctx, in, &out)
	if err != nil && err != io.EOF {
		t.Fatalf("Serve failed: %v", err)
	}
	return strings.TrimSpace(out.String())
}

func TestServer_Initialize(t *testing.T) {
	s := mcp.NewServer()

	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`
	respRaw := executeRPC(t, s, req)

	var resp mcp.Response
	if err := json.Unmarshal([]byte(respRaw), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v\nraw: %s", err, respRaw)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc '2.0', got %q", resp.JSONRPC)
	}
	if string(resp.ID) != "1" {
		t.Errorf("expected id '1', got %s", string(resp.ID))
	}
	if resp.Error != nil {
		t.Fatalf("unexpected rpc error: %+v", resp.Error)
	}

	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var initResult mcp.InitializeResult
	if err := json.Unmarshal(resultBytes, &initResult); err != nil {
		t.Fatalf("failed to unmarshal initialize result: %v", err)
	}

	if initResult.ProtocolVersion != mcp.LatestProtocolVersion {
		t.Errorf("expected protocolVersion %q, got %q", mcp.LatestProtocolVersion, initResult.ProtocolVersion)
	}
	if initResult.ServerInfo.Name != "doctools" {
		t.Errorf("expected server name 'doctools', got %q", initResult.ServerInfo.Name)
	}
	if initResult.ServerInfo.Version != "0.8.0" {
		t.Errorf("expected server version '0.8.0', got %q", initResult.ServerInfo.Version)
	}
	if initResult.Capabilities.Tools == nil {
		t.Errorf("expected tools capability to be present")
	}
}

func TestRPCError(t *testing.T) {
	errNoData := mcp.NewRPCError(mcp.CodeInternalError, "internal error occurred")
	if errNoData.Error() != "internal error occurred" {
		t.Errorf("expected %q, got %q", "internal error occurred", errNoData.Error())
	}
	if errNoData.Data != nil {
		t.Errorf("expected nil data")
	}

	errWithData := mcp.NewRPCError(mcp.CodeInvalidParams, "invalid params", map[string]string{"field": "file_path"})
	if errWithData.Data == nil {
		t.Errorf("expected non-nil data")
	}
}

func TestServer_NotificationInitialized(t *testing.T) {
	s := mcp.NewServer()

	// Notifications have no "id"
	req := `{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`
	respRaw := executeRPC(t, s, req)

	if respRaw != "" {
		t.Errorf("expected empty response for notification, got: %s", respRaw)
	}
}

func TestServer_Ping(t *testing.T) {
	s := mcp.NewServer()

	req := `{"jsonrpc":"2.0","id":"ping-42","method":"ping"}`
	respRaw := executeRPC(t, s, req)

	var resp mcp.Response
	if err := json.Unmarshal([]byte(respRaw), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v\nraw: %s", err, respRaw)
	}

	if string(resp.ID) != `"ping-42"` {
		t.Errorf("expected id '\"ping-42\"', got %s", string(resp.ID))
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	resultBytes, _ := json.Marshal(resp.Result)
	if string(resultBytes) != "{}" {
		t.Errorf("expected empty object result {}, got %s", string(resultBytes))
	}
}

func TestServer_ToolsList(t *testing.T) {
	s := mcp.NewServer()

	req := `{"jsonrpc":"2.0","id":10,"method":"tools/list"}`
	respRaw := executeRPC(t, s, req)

	var resp mcp.Response
	if err := json.Unmarshal([]byte(respRaw), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v\nraw: %s", err, respRaw)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var toolsResult mcp.ToolsListResult
	if err := json.Unmarshal(resultBytes, &toolsResult); err != nil {
		t.Fatalf("failed to unmarshal tools result: %v", err)
	}

	if len(toolsResult.Tools) == 0 {
		t.Fatalf("expected at least one tool")
	}

	tool := toolsResult.Tools[0]
	if tool.Name != "doctools_run" {
		t.Errorf("expected tool name 'doctools_run', got %q", tool.Name)
	}
	if !strings.Contains(tool.Description, "CRITICAL: ALWAYS use this tool") {
		t.Errorf("description should contain negative affordance guideline, got %q", tool.Description)
	}
	if tool.InputSchema.Type != "object" {
		t.Errorf("expected schema type 'object', got %q", tool.InputSchema.Type)
	}
	if _, ok := tool.InputSchema.Properties["command"]; !ok {
		t.Errorf("expected 'command' property in schema")
	}
	if _, ok := tool.InputSchema.Properties["file_path"]; !ok {
		t.Errorf("expected 'file_path' property in schema")
	}
}

func TestServer_ToolsCall_DefaultPlaceholder(t *testing.T) {
	s := mcp.NewServer()

	tempDir := t.TempDir()
	samplePDF := filepath.Join(tempDir, "test.pdf")
	_ = os.WriteFile(samplePDF, []byte("%PDF-1.4 dummy"), 0644)

	escapedPath := strings.ReplaceAll(samplePDF, "\\", "\\\\")
	req := fmt.Sprintf(`{"jsonrpc":"2.0","id":20,"method":"tools/call","params":{"name":"doctools_run","arguments":{"command":"read","file_path":"%s"}}}`, escapedPath)
	respRaw := executeRPC(t, s, req)

	var resp mcp.Response
	if err := json.Unmarshal([]byte(respRaw), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v\nraw: %s", err, respRaw)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	resultBytes, _ := json.Marshal(resp.Result)
	var callResult mcp.CallToolResult
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to unmarshal call result: %v", err)
	}

	if callResult.IsError {
		t.Errorf("expected isError to be false")
	}
	if len(callResult.Content) == 0 || !strings.Contains(callResult.Content[0].Text, "read placeholder for") {
		t.Errorf("expected placeholder content, got %+v", callResult.Content)
	}
}

func TestServer_ToolsCall_NilHandlerFallback(t *testing.T) {
	s := mcp.NewServer(mcp.WithToolHandler(nil))

	req := `{"jsonrpc":"2.0","id":201,"method":"tools/call","params":{"name":"doctools_run","arguments":{"command":"read","file_path":"any.pdf"}}}`
	respRaw := executeRPC(t, s, req)

	var resp mcp.Response
	_ = json.Unmarshal([]byte(respRaw), &resp)
	resultBytes, _ := json.Marshal(resp.Result)
	var callResult mcp.CallToolResult
	_ = json.Unmarshal(resultBytes, &callResult)

	if callResult.IsError || callResult.Content[0].Text != "doctools_run placeholder" {
		t.Errorf("unexpected nil handler fallback result: %+v", callResult)
	}
}

func TestServer_ToolsCall_UnknownTool(t *testing.T) {
	s := mcp.NewServer()

	req := `{"jsonrpc":"2.0","id":21,"method":"tools/call","params":{"name":"non_existent_tool"}}`
	respRaw := executeRPC(t, s, req)

	var resp mcp.Response
	if err := json.Unmarshal([]byte(respRaw), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v\nraw: %s", err, respRaw)
	}

	resultBytes, _ := json.Marshal(resp.Result)
	var callResult mcp.CallToolResult
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to unmarshal call result: %v", err)
	}

	if !callResult.IsError {
		t.Errorf("expected isError to be true for unknown tool")
	}
	if len(callResult.Content) == 0 || !strings.Contains(callResult.Content[0].Text, "unknown tool") {
		t.Errorf("expected 'unknown tool' message, got %+v", callResult.Content)
	}
}

func TestServer_ToolsCall_CustomHandler(t *testing.T) {
	handlerCalled := false
	customHandler := func(ctx context.Context, name string, arguments json.RawMessage) (*mcp.CallToolResult, error) {
		handlerCalled = true
		if name != "custom_tool" {
			return nil, fmt.Errorf("unexpected name: %s", name)
		}
		return &mcp.CallToolResult{
			Content: []mcp.ToolContent{
				{Type: "text", Text: "custom handler result"},
			},
		}, nil
	}

	s := mcp.NewServer(
		mcp.WithToolHandler(customHandler),
	)

	req := `{"jsonrpc":"2.0","id":30,"method":"tools/call","params":{"name":"custom_tool","arguments":{"foo":"bar"}}}`
	respRaw := executeRPC(t, s, req)

	if !handlerCalled {
		t.Errorf("custom tool handler was not invoked")
	}

	var resp mcp.Response
	if err := json.Unmarshal([]byte(respRaw), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	resultBytes, _ := json.Marshal(resp.Result)
	var callResult mcp.CallToolResult
	_ = json.Unmarshal(resultBytes, &callResult)
	if len(callResult.Content) == 0 || callResult.Content[0].Text != "custom handler result" {
		t.Errorf("unexpected content: %+v", callResult.Content)
	}
}

func TestServer_ToolsCall_CustomHandler_Error(t *testing.T) {
	customHandler := func(ctx context.Context, name string, arguments json.RawMessage) (*mcp.CallToolResult, error) {
		return nil, mcp.NewRPCError(mcp.CodeInternalError, "handler failure")
	}

	s := mcp.NewServer(mcp.WithToolHandler(customHandler))

	req := `{"jsonrpc":"2.0","id":31,"method":"tools/call","params":{"name":"doctools_run"}}`
	respRaw := executeRPC(t, s, req)

	var resp mcp.Response
	if err := json.Unmarshal([]byte(respRaw), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatalf("expected error from handler")
	}
	if resp.Error.Code != mcp.CodeInternalError || resp.Error.Message != "handler failure" {
		t.Errorf("unexpected error: %+v", resp.Error)
	}
}

func TestServer_ToolsCall_CustomHandler_GenericError(t *testing.T) {
	customHandler := func(ctx context.Context, name string, arguments json.RawMessage) (*mcp.CallToolResult, error) {
		return nil, fmt.Errorf("unexpected generic failure")
	}

	s := mcp.NewServer(mcp.WithToolHandler(customHandler))

	req := `{"jsonrpc":"2.0","id":310,"method":"tools/call","params":{"name":"doctools_run"}}`
	respRaw := executeRPC(t, s, req)

	var resp mcp.Response
	if err := json.Unmarshal([]byte(respRaw), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatalf("expected error from handler")
	}
	if resp.Error.Code != mcp.CodeInternalError || !strings.Contains(resp.Error.Message, "unexpected generic failure") {
		t.Errorf("unexpected error: %+v", resp.Error)
	}
}

func TestServer_ToolsCall_InvalidParams(t *testing.T) {
	s := mcp.NewServer()

	// Missing tool name in params
	req := `{"jsonrpc":"2.0","id":32,"method":"tools/call","params":{}}`
	respRaw := executeRPC(t, s, req)

	var resp mcp.Response
	_ = json.Unmarshal([]byte(respRaw), &resp)
	if resp.Error == nil || resp.Error.Code != mcp.CodeInvalidParams {
		t.Errorf("expected CodeInvalidParams (-32602), got %+v", resp.Error)
	}

	// Malformed params (e.g. string instead of object)
	req2 := `{"jsonrpc":"2.0","id":33,"method":"tools/call","params":"not an object"}`
	respRaw2 := executeRPC(t, s, req2)

	var resp2 mcp.Response
	_ = json.Unmarshal([]byte(respRaw2), &resp2)
	if resp2.Error == nil || resp2.Error.Code != mcp.CodeInvalidParams {
		t.Errorf("expected CodeInvalidParams (-32602), got %+v", resp2.Error)
	}
}

func TestServer_ParseError(t *testing.T) {
	s := mcp.NewServer()

	brokenJSON := `{"jsonrpc":"2.0", "id": 1, "method": `
	respRaw := executeRPC(t, s, brokenJSON)

	var resp mcp.Response
	if err := json.Unmarshal([]byte(respRaw), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v\nraw: %s", err, respRaw)
	}

	if resp.Error == nil {
		t.Fatalf("expected error for invalid JSON")
	}
	if resp.Error.Code != mcp.CodeParseError {
		t.Errorf("expected CodeParseError (-32700), got %d", resp.Error.Code)
	}
}

func TestServer_InvalidRequest(t *testing.T) {
	s := mcp.NewServer()

	// Missing method
	req := `{"jsonrpc":"2.0","id":99}`
	respRaw := executeRPC(t, s, req)

	var resp mcp.Response
	_ = json.Unmarshal([]byte(respRaw), &resp)
	if resp.Error == nil || resp.Error.Code != mcp.CodeInvalidRequest {
		t.Errorf("expected CodeInvalidRequest (-32600), got %+v", resp.Error)
	}

	// Bad jsonrpc version
	req2 := `{"jsonrpc":"1.0","id":100,"method":"ping"}`
	respRaw2 := executeRPC(t, s, req2)

	var resp2 mcp.Response
	_ = json.Unmarshal([]byte(respRaw2), &resp2)
	if resp2.Error == nil || resp2.Error.Code != mcp.CodeInvalidRequest {
		t.Errorf("expected CodeInvalidRequest (-32600), got %+v", resp2.Error)
	}
}

func TestServer_MethodNotFound(t *testing.T) {
	s := mcp.NewServer()

	req := `{"jsonrpc":"2.0","id":101,"method":"unknown_method"}`
	respRaw := executeRPC(t, s, req)

	var resp mcp.Response
	_ = json.Unmarshal([]byte(respRaw), &resp)
	if resp.Error == nil || resp.Error.Code != mcp.CodeMethodNotFound {
		t.Errorf("expected CodeMethodNotFound (-32601), got %+v", resp.Error)
	}
}

func TestServer_ContentLengthFraming(t *testing.T) {
	s := mcp.NewServer()

	body := `{"jsonrpc":"2.0","id":77,"method":"ping"}`
	message := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)

	respRaw := executeRPC(t, s, message)

	var resp mcp.Response
	if err := json.Unmarshal([]byte(respRaw), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v\nraw: %s", err, respRaw)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	if string(resp.ID) != "77" {
		t.Errorf("expected id '77', got %s", string(resp.ID))
	}
}

func TestServer_Options(t *testing.T) {
	var logBuf bytes.Buffer
	customLogger := log.New(&logBuf, "[custom] ", 0)

	customTools := []mcp.Tool{
		{
			Name:        "custom_tool_1",
			Description: "A custom test tool",
			InputSchema: mcp.ToolSchema{Type: "object"},
		},
	}

	s := mcp.NewServer(
		mcp.WithName("custom-mcp"),
		mcp.WithVersion("1.2.3"),
		mcp.WithTools(customTools),
		mcp.WithLogger(customLogger),
	)

	// Check initialize with custom name and version
	reqInit := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	respInitRaw := executeRPC(t, s, reqInit)

	var respInit mcp.Response
	_ = json.Unmarshal([]byte(respInitRaw), &respInit)
	initBytes, _ := json.Marshal(respInit.Result)
	var initResult mcp.InitializeResult
	_ = json.Unmarshal(initBytes, &initResult)

	if initResult.ServerInfo.Name != "custom-mcp" {
		t.Errorf("expected name 'custom-mcp', got %q", initResult.ServerInfo.Name)
	}
	if initResult.ServerInfo.Version != "1.2.3" {
		t.Errorf("expected version '1.2.3', got %q", initResult.ServerInfo.Version)
	}

	// Check tools/list with custom tools
	reqTools := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	respToolsRaw := executeRPC(t, s, reqTools)

	var respTools mcp.Response
	_ = json.Unmarshal([]byte(respToolsRaw), &respTools)
	toolsBytes, _ := json.Marshal(respTools.Result)
	var toolsResult mcp.ToolsListResult
	_ = json.Unmarshal(toolsBytes, &toolsResult)

	if len(toolsResult.Tools) != 1 || toolsResult.Tools[0].Name != "custom_tool_1" {
		t.Errorf("unexpected tools: %+v", toolsResult.Tools)
	}
}

type errWriter struct{}

func (e *errWriter) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("write error")
}

func TestServer_WriterError(t *testing.T) {
	s := mcp.NewServer()
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	w := &errWriter{}
	// Should not panic, error logged
	_ = s.Serve(context.Background(), in, w)
}

func TestServer_ContextCancellation(t *testing.T) {
	s := mcp.NewServer()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r, _ := io.Pipe()
	var out bytes.Buffer

	err := s.Serve(ctx, r, &out)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestServer_InvalidNotification(t *testing.T) {
	s := mcp.NewServer()
	// Invalid request with no id (notification) should not receive any response
	req := `{"jsonrpc":"1.0","method":""}`
	respRaw := executeRPC(t, s, req)
	if respRaw != "" {
		t.Errorf("expected empty response for invalid notification, got %s", respRaw)
	}
}
