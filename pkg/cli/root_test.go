package cli

import (
	"strings"
	"testing"

	"doctools-cli/internal/version"
)

// TestRootHelpShowsVersion covers the custom help template: a human running
// --help must be able to tell which build they have.
func TestRootHelpShowsVersion(t *testing.T) {
	out, err, exit := runCLI("--help")
	if err != nil || exit != nil {
		t.Fatalf("--help failed: err=%v exit=%v", err, exit)
	}

	want := "doctools-cli version " + version.Version
	if !strings.Contains(out, want) {
		t.Errorf("expected %q in help output:\n%s", want, out)
	}
	// The rest of the default help must still be there.
	if !strings.Contains(out, "Available Commands:") || !strings.Contains(out, "Agent-Native CLI") {
		t.Errorf("help output lost its usual content:\n%s", out)
	}
}

// TestVersionFlag covers the --version flag that cobra registers once
// Command.Version is set.
func TestVersionFlag(t *testing.T) {
	out, err, exit := runCLI("--version")
	if err != nil || exit != nil {
		t.Fatalf("--version failed: err=%v exit=%v", err, exit)
	}

	want := "doctools-cli version " + version.Version
	if strings.TrimSpace(out) != want {
		t.Errorf("expected %q, got %q", want, strings.TrimSpace(out))
	}
}

// TestSubcommandHelpOmitsVersion guards the .Version condition in the template:
// subcommands carry no version, so the line must not appear as an empty stub.
func TestSubcommandHelpOmitsVersion(t *testing.T) {
	out, err, exit := runCLI("excel", "--help")
	if err != nil || exit != nil {
		t.Fatalf("excel --help failed: err=%v exit=%v", err, exit)
	}

	if strings.Contains(out, "version") {
		t.Errorf("subcommand help should not carry a version line:\n%s", out)
	}
	if !strings.Contains(out, "Available Commands:") {
		t.Errorf("subcommand help lost its usual content:\n%s", out)
	}
}
