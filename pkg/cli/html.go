package cli

import (
	"doctools-cli/pkg/html"
	"doctools-cli/pkg/models"
	"doctools-cli/pkg/util"

	"github.com/spf13/cobra"
)

var htmlCmd = &cobra.Command{
	Use:   "html",
	Short: "HTML document processing commands",
}

var htmlExtractOut string

var htmlExtractTextCmd = &cobra.Command{
	Use:   "extract-text <input-file>",
	Short: "Extract text from HTML file to Markdown",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		outPath, err := html.ExtractText(args[0], htmlExtractOut)
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

func init() {
	htmlExtractTextCmd.Flags().StringVarP(&htmlExtractOut, "output", "o", "", "Output Markdown file path")
	htmlCmd.AddCommand(htmlExtractTextCmd)
	RootCmd.AddCommand(htmlCmd)
}
