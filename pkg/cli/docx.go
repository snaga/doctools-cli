package cli

import (
	"unicode/utf8"

	"doctools-cli/pkg/docx"
	"doctools-cli/pkg/models"
	"doctools-cli/pkg/util"

	"github.com/spf13/cobra"
)

// docxCmd represents the base command for Word (.docx) document processing.
var docxCmd = &cobra.Command{
	Use:   "docx",
	Short: "Word document (.docx) processing commands",
}

// docxTextOutputPath stores the optional destination file path for extracted text.
var docxTextOutputPath string

// docxExtractTextCmd extracts text and tables from a Word (.docx) document as Markdown.
var docxExtractTextCmd = &cobra.Command{
	Use:   "extract-text <input-file>",
	Short: "Extract text and tables from Word (.docx) document as Markdown",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		content, err := docx.ExtractText(inputPath, docxTextOutputPath)
		if err != nil {
			util.ExitWithError(err)
		}

		data := map[string]interface{}{
			"output_path": docxTextOutputPath,
		}
		if docxTextOutputPath != "" {
			data["content"] = ""
			data["length"] = utf8.RuneCountInString(content)
		} else {
			data["content"] = content
		}

		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data:   data,
		})
	},
}

// init registers docx subcommands and flags to the root command.
func init() {
	docxExtractTextCmd.Flags().StringVarP(&docxTextOutputPath, "output", "o", "", "Output Markdown path")

	docxCmd.AddCommand(docxExtractTextCmd)
	RootCmd.AddCommand(docxCmd)
}
