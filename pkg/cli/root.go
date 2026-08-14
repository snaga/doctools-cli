package cli

import (
	"doctools-cli/internal/version"

	"github.com/spf13/cobra"
)

var (
	JSONFlag  bool
	ForceFlag bool
)

// helpTemplate is cobra's default help template with a leading version line, so
// that `--help` alone tells a human which build they are running. The guard on
// .Version keeps the line off subcommand help, where it is always empty.
const helpTemplate = `{{if .Version}}{{.Name}} version {{.Version}}

{{end}}{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "doctools-cli",
	Short: "doctools-cli is an Agent-Native CLI for document processing and analysis",
	Long: `doctools-cli is an Agent-Native CLI designed for AI agents and humans to extract,
convert, and search documents (Excel, PPTX, PDF, CSV, Text, HTML, Images).`,
	// Setting Version also makes cobra register the --version flag.
	Version:       version.Version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return RootCmd.Execute()
}

func init() {
	RootCmd.SetHelpTemplate(helpTemplate)
	RootCmd.SetVersionTemplate("{{.Name}} version {{.Version}}\n")

	RootCmd.PersistentFlags().BoolVar(&JSONFlag, "json", false, "Output results in JSON format")
	RootCmd.PersistentFlags().BoolVar(&ForceFlag, "force", false, "Force operation without interactive confirmation")
}
