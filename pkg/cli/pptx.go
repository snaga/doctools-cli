package cli

import (
	"doctools-cli/pkg/models"
	"doctools-cli/pkg/pptx"
	"doctools-cli/pkg/util"

	"github.com/spf13/cobra"
)

var pptxCmd = &cobra.Command{
	Use:   "pptx",
	Short: "PowerPoint presentation processing commands",
}

var (
	pptxTextOutputPath string
	pptxTextStartSlide int
	pptxTextEndSlide   int
)

var pptxExtractTextCmd = &cobra.Command{
	Use:   "extract-text <input-file>",
	Short: "Extract text from PPTX slides",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		content, err := pptx.ExtractTextPureGo(inputPath, pptxTextOutputPath, pptxTextStartSlide, pptxTextEndSlide)
		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data: map[string]interface{}{
				"content":     content,
				"output_path": pptxTextOutputPath,
			},
		})
	},
}

var pptxMergeOutputPath string

var pptxMergeCmd = &cobra.Command{
	Use:   "merge <input-file-1> <input-file-2> ...",
	Short: "Merge multiple PPTX files into one",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		outPath, err := pptx.MergePureGo(args, pptxMergeOutputPath)
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
	pptxImgOutputDir string
	pptxImgSlides    []int
	pptxImgWidth     int
	pptxImgHeight    int
)

var pptxExtractImagesCmd = &cobra.Command{
	Use:   "extract-images <input-file>",
	Short: "Extract PPTX slides as images (requires Windows PowerPoint)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		paths, err := pptx.ExtractImagesCOM(inputPath, pptxImgOutputDir, pptxImgSlides, pptxImgWidth, pptxImgHeight)
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

func init() {
	pptxExtractTextCmd.Flags().StringVarP(&pptxTextOutputPath, "output", "o", "", "Output Markdown path")
	pptxExtractTextCmd.Flags().IntVar(&pptxTextStartSlide, "start-slide", 1, "Start slide number (1-based)")
	pptxExtractTextCmd.Flags().IntVar(&pptxTextEndSlide, "end-slide", 0, "End slide number (1-based, 0 for all)")

	pptxMergeCmd.Flags().StringVarP(&pptxMergeOutputPath, "output", "o", "merged.pptx", "Output PPTX path")

	pptxExtractImagesCmd.Flags().StringVarP(&pptxImgOutputDir, "output-dir", "o", "", "Output directory")
	pptxExtractImagesCmd.Flags().IntSliceVar(&pptxImgSlides, "slides", nil, "Slides to extract")
	pptxExtractImagesCmd.Flags().IntVar(&pptxImgWidth, "width", 1280, "Image width")
	pptxExtractImagesCmd.Flags().IntVar(&pptxImgHeight, "height", 720, "Image height")

	pptxCmd.AddCommand(pptxExtractTextCmd)
	pptxCmd.AddCommand(pptxMergeCmd)
	pptxCmd.AddCommand(pptxExtractImagesCmd)
	RootCmd.AddCommand(pptxCmd)
}
