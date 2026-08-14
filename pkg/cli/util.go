package cli

import (
	"doctools-cli/pkg/models"
	"doctools-cli/pkg/util"

	"github.com/spf13/cobra"
)

var utilCmd = &cobra.Command{
	Use:   "util",
	Short: "Utility commands for archiving and decompression",
}

var utilZipOut string
var utilZipCmd = &cobra.Command{
	Use:   "zip <input-file-1> <input-file-2> ...",
	Short: "Compress files/directories into zip",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		outPath, err := util.ZipCompress(args, utilZipOut)
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

var utilUnzipDest string
var utilUnzipCmd = &cobra.Command{
	Use:   "unzip <zip-file>",
	Short: "Decompress zip archive into directory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		destPath, err := util.UnzipDecompress(args[0], utilUnzipDest)
		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data: map[string]interface{}{
				"output_dir": destPath,
			},
		})
	},
}

func init() {
	utilZipCmd.Flags().StringVarP(&utilZipOut, "output", "o", "archive.zip", "Output zip file path")
	utilUnzipCmd.Flags().StringVarP(&utilUnzipDest, "output-dir", "o", "", "Destination directory path")

	utilCmd.AddCommand(utilZipCmd)
	utilCmd.AddCommand(utilUnzipCmd)
	RootCmd.AddCommand(utilCmd)
}
