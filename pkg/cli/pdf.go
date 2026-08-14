package cli

import (
	"doctools-cli/pkg/models"
	"doctools-cli/pkg/pdf"
	"doctools-cli/pkg/util"

	"github.com/spf13/cobra"
)

var pdfCmd = &cobra.Command{
	Use:   "pdf",
	Short: "PDF document processing commands",
}

var (
	pdfTextOutputPath string
	pdfTextStartPage  int
	pdfTextEndPage    int
)

var pdfExtractTextCmd = &cobra.Command{
	Use:   "extract-text <input-file>",
	Short: "Extract text from PDF file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		content, err := pdf.ExtractText(inputPath, pdfTextOutputPath, pdfTextStartPage, pdfTextEndPage)
		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data: map[string]interface{}{
				"content":     content,
				"output_path": pdfTextOutputPath,
			},
		})
	},
}

var (
	pdfSplitOutputPath string
	pdfSplitStartPage  int
	pdfSplitEndPage    int
)

var pdfSplitCmd = &cobra.Command{
	Use:   "split <input-file>",
	Short: "Extract page range from PDF file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		outPath, err := pdf.Split(inputPath, pdfSplitOutputPath, pdfSplitStartPage, pdfSplitEndPage)
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

var pdfMergeOutputPath string

var pdfMergeCmd = &cobra.Command{
	Use:   "merge <input-file-1> <input-file-2> ...",
	Short: "Merge multiple PDF files into one",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		outPath, err := pdf.Merge(args, pdfMergeOutputPath)
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

var (
	pdfImgOutputDir string
	pdfImgPages     []int
)

var pdfExtractImagesCmd = &cobra.Command{
	Use:   "extract-images <input-file>",
	Short: "Extract images from PDF file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		paths, err := pdf.ExtractImages(inputPath, pdfImgOutputDir, pdfImgPages)
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
	pdfPagesOutputDir string
	pdfPagesDPI       int
	pdfPagesFormat    string
	pdfPagesStartPage int
	pdfPagesEndPage   int
	pdfPagesForce     bool
)

var pdfExtractPagesCmd = &cobra.Command{
	Use:   "extract-pages <input-file>",
	Short: "Render PDF pages to images",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		paths, err := pdf.ExtractPages(inputPath, pdfPagesOutputDir, pdfPagesDPI, pdfPagesFormat, pdfPagesStartPage, pdfPagesEndPage, pdfPagesForce)
		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data: map[string]interface{}{
				"total_pages":  len(paths),
				"dpi":          pdfPagesDPI,
				"format":       pdfPagesFormat,
				"output_paths": paths,
			},
		})
	},
}

func init() {
	pdfExtractTextCmd.Flags().StringVarP(&pdfTextOutputPath, "output", "o", "", "Output Markdown path")
	pdfExtractTextCmd.Flags().IntVar(&pdfTextStartPage, "start-page", 1, "Start page number (1-based)")
	pdfExtractTextCmd.Flags().IntVar(&pdfTextEndPage, "end-page", 0, "End page number (1-based, 0 for all)")

	pdfSplitCmd.Flags().StringVarP(&pdfSplitOutputPath, "output", "o", "", "Output PDF path")
	pdfSplitCmd.Flags().IntVar(&pdfSplitStartPage, "start-page", 1, "Start page number (1-based)")
	pdfSplitCmd.Flags().IntVar(&pdfSplitEndPage, "end-page", 1, "End page number (1-based)")

	pdfMergeCmd.Flags().StringVarP(&pdfMergeOutputPath, "output", "o", "merged.pdf", "Output PDF path")

	pdfExtractImagesCmd.Flags().StringVarP(&pdfImgOutputDir, "output-dir", "o", "", "Output directory")
	pdfExtractImagesCmd.Flags().IntSliceVar(&pdfImgPages, "pages", nil, "Page numbers to extract")

	pdfExtractPagesCmd.Flags().StringVarP(&pdfPagesOutputDir, "output-dir", "o", "", "Output directory")
	pdfExtractPagesCmd.Flags().IntVar(&pdfPagesDPI, "dpi", 150, "Rendering resolution DPI")
	pdfExtractPagesCmd.Flags().StringVar(&pdfPagesFormat, "format", "png", "Output image format (png or jpg)")
	pdfExtractPagesCmd.Flags().IntVar(&pdfPagesStartPage, "start-page", 1, "Start page number (1-based)")
	pdfExtractPagesCmd.Flags().IntVar(&pdfPagesEndPage, "end-page", 0, "End page number (1-based, 0 for all)")
	pdfExtractPagesCmd.Flags().BoolVar(&pdfPagesForce, "force", false, "Force overwrite")

	pdfCmd.AddCommand(pdfExtractTextCmd)
	pdfCmd.AddCommand(pdfSplitCmd)
	pdfCmd.AddCommand(pdfMergeCmd)
	pdfCmd.AddCommand(pdfExtractImagesCmd)
	pdfCmd.AddCommand(pdfExtractPagesCmd)
	RootCmd.AddCommand(pdfCmd)
}

