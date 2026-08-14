package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestPrintAgentContext(t *testing.T) {
	var buf bytes.Buffer
	err := PrintAgentContext(&buf)
	if err != nil {
		t.Fatalf("PrintAgentContext returned error: %v", err)
	}

	var resp AgentContextResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse agent-context output JSON: %v", err)
	}

	if resp.CLI != "doctools-cli" {
		t.Errorf("expected CLI 'doctools-cli', got %q", resp.CLI)
	}

	if resp.RootCommand.Name != "doctools-cli" {
		t.Errorf("expected root command name 'doctools-cli', got %q", resp.RootCommand.Name)
	}

	// Verify Persistent Flags (--json, --force) exist
	hasJSONFlag := false
	hasForceFlag := false
	for _, flag := range resp.RootCommand.Flags {
		if flag.Name == "json" {
			hasJSONFlag = true
		}
		if flag.Name == "force" {
			hasForceFlag = true
		}
	}

	if !hasJSONFlag {
		t.Errorf("expected '--json' persistent flag in root command")
	}
	if !hasForceFlag {
		t.Errorf("expected '--force' persistent flag in root command")
	}

	// Verify agent-context subcommand exists
	hasAgentContextCmd := false
	for _, sub := range resp.RootCommand.Subcommands {
		if sub.Name == "agent-context" {
			hasAgentContextCmd = true
		}
	}

	if !hasAgentContextCmd {
		t.Errorf("expected 'agent-context' subcommand in root command tree")
	}
}

// TestAgentContextCommand exercises the cobra entry point rather than
// PrintAgentContext directly.
func TestAgentContextCommand(t *testing.T) {
	out, err, exit := runCLI("agent-context")
	if err != nil || exit != nil {
		t.Fatalf("agent-context failed: err=%v exit=%v", err, exit)
	}

	var resp AgentContextResponse
	if jsonErr := json.Unmarshal([]byte(out), &resp); jsonErr != nil {
		t.Fatalf("failed to parse agent-context output: %v\n%s", jsonErr, out)
	}
	if resp.RootCommand.Name != "doctools-cli" {
		t.Errorf("expected root command name 'doctools-cli', got %q", resp.RootCommand.Name)
	}
}

// TestInspectCommandEdgeCases covers the branches that the real command tree
// never reaches: a flag declared both locally and persistently, a flag marked
// as required, and a hidden subcommand.
func TestInspectCommandEdgeCases(t *testing.T) {
	root := &cobra.Command{Use: "fake", Short: "fake root"}
	root.PersistentFlags().Bool("shared", false, "declared twice")
	root.Flags().Bool("shared", false, "declared twice")
	root.Flags().String("token", "", "required flag")
	if err := root.MarkFlagRequired("token"); err != nil {
		t.Fatalf("failed to mark flag required: %v", err)
	}

	root.AddCommand(&cobra.Command{Use: "visible", Short: "visible subcommand"})
	root.AddCommand(&cobra.Command{Use: "secret", Short: "hidden subcommand", Hidden: true})

	info := InspectCommand(root)

	shared := 0
	requiredFound := false
	for _, f := range info.Flags {
		if f.Name == "shared" {
			shared++
		}
		if f.Name == "token" && f.Required {
			requiredFound = true
		}
	}
	if shared != 1 {
		t.Errorf("expected the duplicated flag to be collected once, got %d", shared)
	}
	if !requiredFound {
		t.Error("expected 'token' to be reported as required")
	}

	if len(info.Subcommands) != 1 || info.Subcommands[0].Name != "visible" {
		names := make([]string, 0, len(info.Subcommands))
		for _, sub := range info.Subcommands {
			names = append(names, sub.Name)
		}
		t.Errorf("expected only the visible subcommand, got [%s]", strings.Join(names, ", "))
	}
}
