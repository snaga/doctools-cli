package cli

import (
	"doctools-cli/pkg/models"
	"doctools-cli/pkg/text"
	"doctools-cli/pkg/util"

	"github.com/spf13/cobra"
)

var textCmd = &cobra.Command{
	Use:   "text",
	Short: "Text file processing commands",
}

var textHeadN int
var textHeadCmd = &cobra.Command{
	Use:   "head <input-file>",
	Short: "Read first N lines of a text file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		lines, err := text.ReadHead(args[0], textHeadN)
		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data: map[string]interface{}{
				"lines": lines,
			},
		})
	},
}

var textTailN int
var textTailCmd = &cobra.Command{
	Use:   "tail <input-file>",
	Short: "Read last N lines of a text file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		lines, err := text.ReadTail(args[0], textTailN)
		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data: map[string]interface{}{
				"lines": lines,
			},
		})
	},
}

var textGrepCmd = &cobra.Command{
	Use:   "grep <input-file> <pattern>",
	Short: "Grep regex pattern in text file",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		matches, err := text.Grep(args[0], args[1])
		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data: map[string]interface{}{
				"matches": matches,
			},
		})
	},
}

var textConvertOut string
var textConvertEncoding string

var textConvertCmd = &cobra.Command{
	Use:   "convert <input-file>",
	Short: "Convert text file encoding",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		outPath, err := text.ConvertEncoding(args[0], textConvertEncoding, textConvertOut)
		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data: map[string]interface{}{
				"output_path": outPath,
			},
		})
	},
}

var textMetadataCmd = &cobra.Command{
	Use:   "metadata <input-file>",
	Short: "Get text file metadata (encoding, size, lines)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		meta, err := text.GetMetadata(inputPath)
		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data: map[string]interface{}{
				"encoding": meta.Encoding,
				"size":     meta.Size,
				"lines":    meta.Lines,
			},
		})
	},
}

var textCopyCmd = &cobra.Command{
	Use:   "copy-clipboard <text>",
	Short: "Copy text to Windows clipboard",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		msg, err := text.CopyClipboard(args[0])
		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data: map[string]interface{}{
				"message": msg,
			},
		})
	},
}

func init() {
	textHeadCmd.Flags().IntVarP(&textHeadN, "lines", "n", 10, "Number of lines to read")
	textTailCmd.Flags().IntVarP(&textTailN, "lines", "n", 10, "Number of lines to read")

	textConvertCmd.Flags().StringVarP(&textConvertOut, "output", "o", "", "Output file path")
	textConvertCmd.Flags().StringVarP(&textConvertEncoding, "encoding", "e", "utf-8", "Target encoding")

	textCmd.AddCommand(textHeadCmd)
	textCmd.AddCommand(textTailCmd)
	textCmd.AddCommand(textGrepCmd)
	textCmd.AddCommand(textConvertCmd)
	textCmd.AddCommand(textMetadataCmd)
	textCmd.AddCommand(textCopyCmd)
	RootCmd.AddCommand(textCmd)
}
