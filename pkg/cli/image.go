package cli

import (
	"fmt"

	imgpkg "doctools-cli/pkg/image"
	"doctools-cli/pkg/models"
	"doctools-cli/pkg/util"

	"github.com/spf13/cobra"
)

var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Image document processing commands",
}

var imageMetadataCmd = &cobra.Command{
	Use:   "metadata <input-file>",
	Short: "Get image metadata",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		meta, err := imgpkg.GetMetadata(args[0])
		if err != nil {
			util.ExitWithError(err)
			return
		}
		if meta == nil {
			util.ExitWithError(fmt.Errorf("metadata is nil"))
			return
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data: map[string]interface{}{
				"width":  meta.Width,
				"height": meta.Height,
				"format": meta.Format,
			},
		})
	},
}

var (
	imageCropOut    string
	imageCropLeft   int
	imageCropTop    int
	imageCropRight  int
	imageCropBottom int
)

var imageCropCmd = &cobra.Command{
	Use:   "crop <input-file>",
	Short: "Crop image rectangle",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		outPath, err := imgpkg.Crop(args[0], imageCropOut, imageCropLeft, imageCropTop, imageCropRight, imageCropBottom)
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

var imageSaveClipboardOut string
var imageSaveClipboardCmd = &cobra.Command{
	Use:   "save-clipboard",
	Short: "Save Windows clipboard image",
	Run: func(cmd *cobra.Command, args []string) {
		outPath, err := imgpkg.SaveClipboard(imageSaveClipboardOut)
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
	imageCropCmd.Flags().StringVarP(&imageCropOut, "output", "o", "", "Output image path")
	imageCropCmd.Flags().IntVar(&imageCropLeft, "left", 0, "Left boundary")
	imageCropCmd.Flags().IntVar(&imageCropTop, "top", 0, "Top boundary")
	imageCropCmd.Flags().IntVar(&imageCropRight, "right", 100, "Right boundary")
	imageCropCmd.Flags().IntVar(&imageCropBottom, "bottom", 100, "Bottom boundary")

	imageSaveClipboardCmd.Flags().StringVarP(&imageSaveClipboardOut, "output", "o", "clipboard.png", "Output image path")

	imageCmd.AddCommand(imageMetadataCmd)
	imageCmd.AddCommand(imageCropCmd)
	imageCmd.AddCommand(imageSaveClipboardCmd)
	RootCmd.AddCommand(imageCmd)
}
