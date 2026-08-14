package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"doctools-cli/pkg/excel"
	"doctools-cli/pkg/models"
	"doctools-cli/pkg/util"

	"github.com/spf13/cobra"
)

var excelCmd = &cobra.Command{
	Use:   "excel",
	Short: "Excel document processing commands",
}

var excelListSheetsCmd = &cobra.Command{
	Use:   "list-sheets <input-file>",
	Short: "List sheet names in an Excel file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		sheets, err := excel.ListSheets(inputPath)
		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data: map[string]interface{}{
				"sheets": sheets,
			},
		})
	},
}

var (
	excelCSVOutputDir  string
	excelCSVSheetNames []string
	excelCSVEncoding   string
)

var excelExtractCSVCmd = &cobra.Command{
	Use:   "extract-csv <input-file>",
	Short: "Extract sheets from Excel file as CSV",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		paths, err := excel.ExtractCSV(inputPath, excelCSVOutputDir, excelCSVSheetNames, excelCSVEncoding)
		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data: map[string]interface{}{
				"output_paths": paths,
				"encoding":     excelCSVEncoding,
			},
		})
	},
}

var (
	excelImgOutputDir  string
	excelImgSheetNames []string
)

var excelExtractImagesCmd = &cobra.Command{
	Use:   "extract-images <input-file>",
	Short: "Extract Excel sheets as images (requires Windows Excel)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		paths, err := excel.ExtractImagesCOM(inputPath, excelImgOutputDir, excelImgSheetNames)
		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data: map[string]interface{}{
				"output_paths": paths,
			},
		})
	},
}

var (
	excelDiffSheetNames []string
)

var excelDiffCmd = &cobra.Command{
	Use:   "diff <file_a> <file_b>",
	Short: "Extract cell-level diff between two Excel files",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		fileA := args[0]
		fileB := args[1]
		res, err := excel.DiffExcel(fileA, fileB, excelDiffSheetNames)
		if err != nil {
			util.ExitWithError(err)
		}

		if JSONFlag {
			util.PrintJSONResponse(models.SuccessResponse{
				Status: "success",
				Data:   res,
			})
		} else {
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Excel Diff Result:\n")
			fmt.Fprintf(w, "  File A: %s\n", res.FileA)
			fmt.Fprintf(w, "  File B: %s\n", res.FileB)
			fmt.Fprintf(w, "  Total Changes: %d\n", res.TotalChanges)
			if res.TotalChanges > 0 {
				fmt.Fprintln(w, "\nDifferences:")
				for _, d := range res.Differences {
					mergedStr := ""
					if d.MergedRange != "" {
						mergedStr = fmt.Sprintf(" (Merged: %s)", d.MergedRange)
					}
					switch d.Type {
					case "value_change":
						fmt.Fprintf(w, "  [%s] Cell %s: Value changed: '%s' -> '%s'%s\n", d.Sheet, d.Cell, d.OldValue, d.NewValue, mergedStr)
					case "formula_change":
						fmt.Fprintf(w, "  [%s] Cell %s: Formula changed: '%s' -> '%s'%s\n", d.Sheet, d.Cell, d.OldFormula, d.NewFormula, mergedStr)
					case "merge_change":
						fmt.Fprintf(w, "  [%s] Cell %s: Merge range changed: '%s' -> '%s'\n", d.Sheet, d.Cell, d.OldValue, d.NewValue)
					}
				}
			} else {
				fmt.Fprintln(w, "\nNo differences found.")
			}
		}
	},
}

var (
	excelPatchFile   string
	excelPatchBackup bool
	excelPatchDryRun bool
	excelPatchSchema bool
)

const patchHint = "Hint: The patch file must be a JSON array. Example: [{\"sheet\": \"Sheet1\", \"cell\": \"A1\", \"old_value\": \"old text\", \"new_value\": \"new text\"}]. Run 'doctools-cli excel patch --schema' for more details."

var excelPatchCmd = &cobra.Command{
	Use:   "patch <target.xlsx>",
	Short: "Apply safety guardrailed patch to an Excel file",
	Long: `Apply safety guardrailed patch to an Excel file using a JSON array of patch items.

Example patch format:
[
  {
    "sheet": "Sheet1",
    "cell": "A1",
    "old_value": "expected_current_value_or_null",
    "new_value": "target_value"
  }
]`,
	Args: func(cmd *cobra.Command, args []string) error {
		if excelPatchSchema {
			return nil
		}
		if len(args) != 1 {
			return fmt.Errorf("accepts 1 arg(s), received %d", len(args))
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if excelPatchSchema {
			schemaJSON := `[
  {
    "sheet": "SheetName",
    "cell": "A1",
    "old_value": "expected_current_value_or_null",
    "new_value": "target_value"
  }
]`
			fmt.Fprintln(cmd.OutOrStdout(), schemaJSON)
			return
		}

		if excelPatchFile == "" {
			util.ExitWithErrorWithHint(fmt.Errorf("--patch-file flag is required"), patchHint)
		}

		targetFile := args[0]
		data, err := os.ReadFile(excelPatchFile)
		if err != nil {
			util.ExitWithErrorWithHint(fmt.Errorf("failed to read patch file: %w", err), patchHint)
		}

		patchItems, err := excel.LoadPatchItems(data)
		if err != nil {
			util.ExitWithErrorWithHint(fmt.Errorf("failed to parse patch JSON file: %w", err), patchHint)
		}

		res, err := excel.PatchExcel(targetFile, patchItems, excelPatchBackup, excelPatchDryRun)
		if err != nil {
			util.ExitWithError(err)
		}

		if JSONFlag {
			util.PrintJSONResponse(models.SuccessResponse{
				Status: "success",
				Data:   res,
			})
		} else {
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Excel Patch Result:\n")
			fmt.Fprintf(w, "  Target File: %s\n", res.TargetFile)
			if res.BackupFile != "" {
				fmt.Fprintf(w, "  Backup File: %s\n", res.BackupFile)
			}
			fmt.Fprintf(w, "  Dry Run: %t\n", res.DryRun)
			fmt.Fprintf(w, "  Total Patches: %d\n", res.TotalPatches)
			fmt.Fprintf(w, "  Applied Count: %d\n", res.AppliedCount)
			if len(res.AuditLog) > 0 {
				fmt.Fprintln(w, "\nAudit Log:")
				for _, item := range res.AuditLog {
					fmt.Fprintf(w, "  [%s] Cell %s: Status=%s, Old='%s', New='%s'\n", item.Sheet, item.Cell, item.Status, item.OldValue, item.NewValue)
				}
			}
		}
	},
}

var (
	excelExtractMdOut      string
	excelExtractSheetNames []string
	excelExtractWithCoords bool
)

var excelExtractMarkdownCmd = &cobra.Command{
	Use:     "extract-markdown <input-file>",
	Aliases: []string{"extract-text"},
	Short:   "Extract sheet content as Markdown table",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		content, err := excel.ExtractMarkdown(inputPath, excelExtractSheetNames, excelExtractWithCoords)
		if err != nil {
			util.ExitWithError(err)
		}

		if excelExtractMdOut != "" {
			if err := os.WriteFile(excelExtractMdOut, []byte(content), 0644); err != nil {
				util.ExitWithError(fmt.Errorf("failed to save output file: %w", err))
			}
		}

		if JSONFlag {
			data := map[string]interface{}{
				"content": content,
			}
			if excelExtractMdOut != "" {
				absPath, err := filepath.Abs(excelExtractMdOut)
				if err == nil {
					data["output_path"] = absPath
				} else {
					data["output_path"] = excelExtractMdOut
				}
			}
			util.PrintJSONResponse(models.SuccessResponse{
				Status: "success",
				Data:   data,
			})
		} else {
			if excelExtractMdOut != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Markdown saved to %s\n", excelExtractMdOut)
			} else {
				fmt.Fprint(cmd.OutOrStdout(), content)
			}
		}
	},
}

var excelExtractTextCmd = &cobra.Command{
	Use:   "extract-text <input-file>",
	Short: "Extract sheet content as Markdown table (alias for extract-markdown)",
	Args:  cobra.ExactArgs(1),
	Run:   excelExtractMarkdownCmd.Run,
}

var excelSearchSheetNames []string

var excelSearchCellCmd = &cobra.Command{
	Use:   "search-cell <input-file> <query>",
	Short: "Search cell content in an Excel file",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		query := args[1]
		res, err := excel.SearchCell(inputPath, query, excelSearchSheetNames)
		if err != nil {
			util.ExitWithError(err)
		}

		if JSONFlag {
			util.PrintJSONResponse(models.SuccessResponse{
				Status: "success",
				Data:   res,
			})
		} else {
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Excel Cell Search Result:\n")
			fmt.Fprintf(w, "  Query: %s\n", query)
			fmt.Fprintf(w, "  Total Matches: %d\n", res.TotalMatches)
			if res.TotalMatches > 0 {
				fmt.Fprintln(w, "\nMatches:")
				for _, m := range res.Matches {
					mergedStr := ""
					if m.MergedRange != "" {
						mergedStr = fmt.Sprintf(" (Merged: %s)", m.MergedRange)
					}
					fmt.Fprintf(w, "  [%s] Cell %s: '%s'%s\n", m.Sheet, m.Cell, m.Value, mergedStr)
				}
			} else {
				fmt.Fprintln(w, "\nNo matches found.")
			}
		}
	},
}

func init() {
	excelExtractCSVCmd.Flags().StringVarP(&excelCSVOutputDir, "output-dir", "o", "", "Output directory")
	excelExtractCSVCmd.Flags().StringSliceVarP(&excelCSVSheetNames, "sheets", "s", nil, "Sheet names to extract")
	excelExtractCSVCmd.Flags().StringVarP(&excelCSVEncoding, "encoding", "e", "utf-8", "Encoding for CSV")

	excelExtractImagesCmd.Flags().StringVarP(&excelImgOutputDir, "output-dir", "o", "", "Output directory")
	excelExtractImagesCmd.Flags().StringSliceVarP(&excelImgSheetNames, "sheets", "s", nil, "Sheet names to extract")

	excelDiffCmd.Flags().StringSliceVarP(&excelDiffSheetNames, "sheets", "s", nil, "Sheets to compare")

	excelPatchCmd.Flags().StringVarP(&excelPatchFile, "patch-file", "p", "", "JSON patch file path (required)")
	excelPatchCmd.Flags().BoolVarP(&excelPatchBackup, "backup", "b", true, "Create a backup file before patching")
	excelPatchCmd.Flags().BoolVar(&excelPatchDryRun, "dry-run", false, "Preview patch application without modifying file")
	excelPatchCmd.Flags().BoolVar(&excelPatchSchema, "schema", false, "Print patch JSON schema template and exit")

	excelExtractMarkdownCmd.Flags().StringVarP(&excelExtractMdOut, "output", "o", "", "Output file path")
	excelExtractMarkdownCmd.Flags().StringSliceVarP(&excelExtractSheetNames, "sheets", "s", nil, "Sheet names to extract")
	excelExtractMarkdownCmd.Flags().BoolVar(&excelExtractWithCoords, "with-coords", false, "Include cell coordinates prefix in markdown output")

	excelExtractTextCmd.Flags().StringVarP(&excelExtractMdOut, "output", "o", "", "Output file path")
	excelExtractTextCmd.Flags().StringSliceVarP(&excelExtractSheetNames, "sheets", "s", nil, "Sheet names to extract")
	excelExtractTextCmd.Flags().BoolVar(&excelExtractWithCoords, "with-coords", false, "Include cell coordinates prefix in markdown output")

	excelSearchCellCmd.Flags().StringSliceVarP(&excelSearchSheetNames, "sheets", "s", nil, "Sheets to search")

	excelCmd.AddCommand(excelListSheetsCmd)
	excelCmd.AddCommand(excelExtractCSVCmd)
	excelCmd.AddCommand(excelExtractImagesCmd)
	excelCmd.AddCommand(excelDiffCmd)
	excelCmd.AddCommand(excelPatchCmd)
	excelCmd.AddCommand(excelExtractMarkdownCmd)
	excelCmd.AddCommand(excelExtractTextCmd)
	excelCmd.AddCommand(excelSearchCellCmd)
	RootCmd.AddCommand(excelCmd)
}



