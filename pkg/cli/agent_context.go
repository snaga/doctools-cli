package cli

import (
	"encoding/json"
	"io"

	"doctools-cli/internal/version"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// FlagInfo represents flag details for agent introspection.
type FlagInfo struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Usage     string `json:"usage"`
	Default   string `json:"default,omitempty"`
	Type      string `json:"type"`
	Required  bool   `json:"required"`
}

// CommandInfo represents command node details in the introspection tree.
type CommandInfo struct {
	Name        string        `json:"name"`
	Use         string        `json:"use"`
	Short       string        `json:"short"`
	Long        string        `json:"long,omitempty"`
	Aliases     []string      `json:"aliases,omitempty"`
	Flags       []FlagInfo    `json:"flags,omitempty"`
	Subcommands []CommandInfo `json:"subcommands,omitempty"`
}

// AgentContextResponse represents the full introspection response structure.
type AgentContextResponse struct {
	CLI         string      `json:"cli"`
	Version     string      `json:"version"`
	RootCommand CommandInfo `json:"root_command"`
}

var agentContextCmd = &cobra.Command{
	Use:   "agent-context",
	Short: "Output structured JSON schema of all commands and arguments for AI Agent introspection",
	Long:  `Inspects the entire Cobra command tree dynamically and outputs all subcommands, flags, and parameter types in JSON format.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return PrintAgentContext(cmd.OutOrStdout())
	},
}

// PrintAgentContext inspects the command tree and writes JSON representation to the writer.
func PrintAgentContext(w io.Writer) error {
	info := InspectCommand(RootCmd)
	resp := AgentContextResponse{
		CLI:         "doctools-cli",
		Version:     version.Version,
		RootCommand: info,
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(resp)
}

// InspectCommand recursively collects CommandInfo for a Cobra command and its subcommands.
func InspectCommand(cmd *cobra.Command) CommandInfo {
	info := CommandInfo{
		Name:    cmd.Name(),
		Use:     cmd.Use,
		Short:   cmd.Short,
		Long:    cmd.Long,
		Aliases: cmd.Aliases,
		Flags:   []FlagInfo{},
	}

	flagMap := make(map[string]bool)
	addFlag := func(f *pflag.Flag) {
		if flagMap[f.Name] {
			return
		}
		flagMap[f.Name] = true
		req := false
		if reqAnnotation, ok := f.Annotations[cobra.BashCompOneRequiredFlag]; ok && len(reqAnnotation) > 0 && reqAnnotation[0] == "true" {
			req = true
		}
		info.Flags = append(info.Flags, FlagInfo{
			Name:      f.Name,
			Shorthand: f.Shorthand,
			Usage:     f.Usage,
			Default:   f.DefValue,
			Type:      f.Value.Type(),
			Required:  req,
		})
	}

	cmd.Flags().VisitAll(addFlag)
	cmd.PersistentFlags().VisitAll(addFlag)

	for _, subCmd := range cmd.Commands() {
		if subCmd.Hidden {
			continue
		}
		info.Subcommands = append(info.Subcommands, InspectCommand(subCmd))
	}

	return info
}

func init() {
	RootCmd.AddCommand(agentContextCmd)
}
