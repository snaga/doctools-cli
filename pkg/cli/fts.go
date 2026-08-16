package cli

import (
	"doctools-cli/pkg/fts"
	"doctools-cli/pkg/models"
	"doctools-cli/pkg/util"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var ftsCmd = &cobra.Command{
	Use:   "fts",
	Short: "Full text search commands",
}

var (
	ftsBuildIndexPath   string
	ftsBuildFileTimeout string
	ftsBuildForce       bool
	ftsBuildIncludeExt  string
	ftsBuildExcludeExt  string
	ftsBuildIncludeDir  string
	ftsBuildExcludeDir  string
)

var ftsBuildCmd = &cobra.Command{
	Use:   "build <source-dir>",
	Short: "Build full text search index from directory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		indexPath, _ := cmd.Flags().GetString("index-path")
		timeoutStr, _ := cmd.Flags().GetString("file-timeout")
		force, _ := cmd.Flags().GetBool("force")
		verbose, _ := cmd.Flags().GetBool("verbose")
		includeExtStr, _ := cmd.Flags().GetString("include-ext")
		excludeExtStr, _ := cmd.Flags().GetString("exclude-ext")
		includeDirStr, _ := cmd.Flags().GetString("include-dir")
		excludeDirStr, _ := cmd.Flags().GetString("exclude-dir")

		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			util.ExitWithError(fmt.Errorf("invalid file timeout: %w", err))
		}

		var includeExts []string
		if includeExtStr != "" {
			for _, s := range strings.Split(includeExtStr, ",") {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					includeExts = append(includeExts, trimmed)
				}
			}
		}

		var excludeExts []string
		if excludeExtStr != "" {
			for _, s := range strings.Split(excludeExtStr, ",") {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					excludeExts = append(excludeExts, trimmed)
				}
			}
		}

		var includeDirs []string
		if includeDirStr != "" {
			for _, s := range strings.Split(includeDirStr, ",") {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					includeDirs = append(includeDirs, trimmed)
				}
			}
		}

		var excludeDirs []string
		if excludeDirStr != "" {
			for _, s := range strings.Split(excludeDirStr, ",") {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					excludeDirs = append(excludeDirs, trimmed)
				}
			}
		}

		res, err := fts.BuildIndexWithOptions(args[0], fts.BuildOptions{
			IndexPath:   indexPath,
			Timeout:     timeout,
			Force:       force,
			Verbose:     verbose,
			IncludeExts: includeExts,
			ExcludeExts: excludeExts,
			IncludeDirs: includeDirs,
			ExcludeDirs: excludeDirs,
		})

		// Reset flags after run for next CLI invocation in same process
		_ = cmd.Flags().Set("force", "false")
		_ = cmd.Flags().Set("file-timeout", "10s")
		_ = cmd.Flags().Set("index-path", "")
		_ = cmd.Flags().Set("verbose", "false")
		_ = cmd.Flags().Set("include-ext", "")
		_ = cmd.Flags().Set("exclude-ext", "")
		_ = cmd.Flags().Set("include-dir", "")
		_ = cmd.Flags().Set("exclude-dir", "")

		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data:   res,
		})
	},
}

var ftsQueryLimit int

var ftsQueryCmd = &cobra.Command{
	Use:   "query <index-path> <query>",
	Short: "Query full text search index",
	Long: `Query full text search index built by Bleve engine.

QueryString Syntax Guide for AI Agents:
- Boolean Operators: Use AND, OR, NOT (or -). Example: "(auth OR login) AND -deprecated"
- Exact Phrase: Wrap in quotes. Example: "\"database design\""
- Field-specific Search: Use field_name:value. Example: "file_name:DB AND content:user"
- Wildcard Search: Use * or ?. Example: "file_path:*sysA*"
- Fuzzy Search: Use ~. Example: "postgres~1"
- Field Boost: Use ^. Example: "title:API^5 content:API"
- Range Search: Use ranges. Example: "updated_at:>=2025-01-01"

Available Searchable Fields:
- content       : Document text content
- file_path     : Absolute file path
- file_name     : Base file name
- file_type     : File extension (.xlsx, .pptx, .pdf, .docx, etc.)
- unit_type     : Structural unit (sheet, slide, page, section)
- unit_name     : Name of sheet (for Excel) or section name
- page_or_index : Page, slide, or row index number (1, 2, 3...)
- locator       : Locator string (e.g. sheet=Sheet1, slide=2, page=1)
- updated_at    : Last modified timestamp`,

	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {

		ftsQueryLimit, _ = cmd.Flags().GetInt("limit")
		res, err := fts.QueryIndex(args[0], args[1], ftsQueryLimit)
		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data:   res,
		})
	},
}

func init() {
	var ftsVerbose bool
	ftsBuildCmd.Flags().StringVarP(&ftsBuildIndexPath, "index-path", "i", "", "Output Bleve index path")
	ftsBuildCmd.Flags().StringVarP(&ftsBuildFileTimeout, "file-timeout", "t", "10s", "Timeout for parsing single file")
	ftsBuildCmd.Flags().BoolVarP(&ftsBuildForce, "force", "f", false, "Force full re-indexing")
	ftsBuildCmd.Flags().BoolVarP(&ftsVerbose, "verbose", "v", false, "Show detailed progress on stderr")
	ftsBuildCmd.Flags().StringVar(&ftsBuildIncludeExt, "include-ext", "", "Comma-separated list of included extensions (e.g. xlsx,pptx,pdf)")
	ftsBuildCmd.Flags().StringVar(&ftsBuildExcludeExt, "exclude-ext", "", "Comma-separated list of excluded extensions")
	ftsBuildCmd.Flags().StringVar(&ftsBuildIncludeDir, "include-dir", "", "Comma-separated list of included directory names or paths")
	ftsBuildCmd.Flags().StringVar(&ftsBuildExcludeDir, "exclude-dir", "", "Comma-separated list of excluded directory names or paths")

	ftsBuildCmd.Flags().SetNormalizeFunc(func(f *pflag.FlagSet, name string) pflag.NormalizedName {
		switch name {
		case "ext":
			return pflag.NormalizedName("include-ext")
		case "exclude":
			return pflag.NormalizedName("exclude-ext")
		case "include-dirs":
			return pflag.NormalizedName("include-dir")
		case "exclude-dirs":
			return pflag.NormalizedName("exclude-dir")
		}
		return pflag.NormalizedName(name)
	})

	ftsQueryCmd.Flags().IntVarP(&ftsQueryLimit, "limit", "l", 10, "Maximum search results")

	ftsCmd.AddCommand(ftsBuildCmd)
	ftsCmd.AddCommand(ftsQueryCmd)
	RootCmd.AddCommand(ftsCmd)
}
