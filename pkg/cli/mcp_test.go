package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"doctools-cli/pkg/mcp"
)

func TestMCPCommand_Registration(t *testing.T) {
	cmd, _, err := RootCmd.Find([]string{"mcp"})
	if err != nil || cmd == nil {
		t.Fatalf("mcp command not found on RootCmd: %v", err)
	}
	if cmd.Name() != "mcp" {
		t.Errorf("expected command name 'mcp', got %q", cmd.Name())
	}
	if cmd.Short != "Start stdio JSON-RPC 2.0 MCP server" {
		t.Errorf("unexpected short description: %q", cmd.Short)
	}
}

func TestMCPCommand_Help(t *testing.T) {
	out, err, exit := runCLI("mcp", "--help")
	if err != nil || exit != nil {
		t.Fatalf("mcp --help failed: err=%v exit=%v", err, exit)
	}
	if !strings.Contains(out, "Start stdio JSON-RPC 2.0 MCP server") {
		t.Errorf("expected help output to describe mcp server, got: %s", out)
	}
}

func TestMCPCommand_StdioInvocation(t *testing.T) {
	resetCommandFlags()

	inBuf := strings.NewReader(`{"jsonrpc":"2.0","id":100,"method":"ping"}` + "\n")
	var outBuf bytes.Buffer

	RootCmd.SetIn(inBuf)
	RootCmd.SetOut(&outBuf)
	RootCmd.SetArgs([]string{"mcp"})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := RootCmd.ExecuteContext(ctx)
	if err != nil {
		t.Fatalf("RootCmd.ExecuteContext failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	if len(lines) == 0 {
		t.Fatalf("expected at least one response line, got empty output")
	}

	var resp mcp.Response
	if err := json.Unmarshal([]byte(lines[0]), &resp); err != nil {
		t.Fatalf("failed to parse JSON-RPC response from stdout: %v\nraw: %s", err, lines[0])
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc '2.0', got %q", resp.JSONRPC)
	}
	if string(resp.ID) != "100" {
		t.Errorf("expected id '100', got %s", string(resp.ID))
	}
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}
}

func TestMCPCommand_InAgentContext(t *testing.T) {
	var buf bytes.Buffer
	err := PrintAgentContext(&buf)
	if err != nil {
		t.Fatalf("PrintAgentContext returned error: %v", err)
	}

	var resp AgentContextResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse agent-context output JSON: %v", err)
	}

	hasMCP := false
	for _, sub := range resp.RootCommand.Subcommands {
		if sub.Name == "mcp" {
			hasMCP = true
			if sub.Short != "Start stdio JSON-RPC 2.0 MCP server" {
				t.Errorf("unexpected short description in agent-context: %q", sub.Short)
			}
			break
		}
	}

	if !hasMCP {
		t.Errorf("expected 'mcp' subcommand in agent-context root command tree")
	}
}
