package cli

import (
	"doctools-cli/pkg/csv"
	"doctools-cli/pkg/models"
	"doctools-cli/pkg/util"

	"github.com/spf13/cobra"
)

var csvCmd = &cobra.Command{
	Use:   "csv",
	Short: "CSV document processing commands",
}

var csvMetadataCmd = &cobra.Command{
	Use:   "metadata <input-file>",
	Short: "Get CSV file metadata",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		meta, err := csv.GetMetadata(inputPath)
		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data: map[string]interface{}{
				"encoding":     meta.Encoding,
				"total_rows":   meta.TotalRows,
				"max_columns":  meta.MaxColumns,
			},
		})
	},
}

var (
	csvReadStartRow  int
	csvReadEndRow    int
	csvReadColumns   []int
	csvReadHeaderRow int
)

var csvReadCellsCmd = &cobra.Command{
	Use:   "read-cells <input-file>",
	Short: "Read cells from CSV file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		data, err := csv.ReadCells(inputPath, csvReadStartRow, csvReadEndRow, csvReadColumns, csvReadHeaderRow)
		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data: map[string]interface{}{
				"data": data,
			},
		})
	},
}

var csvSearchCmd = &cobra.Command{
	Use:   "search <input-file> <query>",
	Short: "Search query in CSV cells",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		query := args[1]
		results, err := csv.SearchValues(inputPath, query)
		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data: map[string]interface{}{
				"results": results,
			},
		})
	},
}

var (
	csvExtractOutputPath string
	csvExtractStartRow   int
	csvExtractEndRow     int
	csvExtractColumns    []int
)

var csvExtractCmd = &cobra.Command{
	Use:   "extract <input-file>",
	Short: "Extract specific rows and columns from CSV to a new file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		outPath, err := csv.Extract(inputPath, csvExtractOutputPath, csvExtractStartRow, csvExtractEndRow, csvExtractColumns)
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
	csvReadCellsCmd.Flags().IntVar(&csvReadStartRow, "start-row", 1, "Start row index (1-based)")
	csvReadCellsCmd.Flags().IntVar(&csvReadEndRow, "end-row", 0, "End row index (1-based, 0 for all)")
	csvReadCellsCmd.Flags().IntSliceVar(&csvReadColumns, "columns", nil, "0-based column indices")
	csvReadCellsCmd.Flags().IntVar(&csvReadHeaderRow, "header-row", 0, "Header row index (0 for none)")

	csvExtractCmd.Flags().StringVarP(&csvExtractOutputPath, "output", "o", "", "Output CSV path")
	csvExtractCmd.Flags().IntVar(&csvExtractStartRow, "start-row", 1, "Start row index (1-based)")
	csvExtractCmd.Flags().IntVar(&csvExtractEndRow, "end-row", 0, "End row index (1-based, 0 for all)")
	csvExtractCmd.Flags().IntSliceVar(&csvExtractColumns, "columns", nil, "0-based column indices")

	csvCmd.AddCommand(csvMetadataCmd)
	csvCmd.AddCommand(csvReadCellsCmd)
	csvCmd.AddCommand(csvSearchCmd)
	csvCmd.AddCommand(csvExtractCmd)
	RootCmd.AddCommand(csvCmd)
}
