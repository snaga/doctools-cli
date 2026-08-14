package cli

import (
	"doctools-cli/pkg/models"
	"doctools-cli/pkg/pageindex"
	"doctools-cli/pkg/util"

	"github.com/spf13/cobra"
)

var pageindexCmd = &cobra.Command{
	Use:   "pageindex",
	Short: "PageIndex structural document search and retrieval",
}

var (
	pageindexTreeNodeID string
	pageindexTreeDepth  int
)

var pageindexTreeCmd = &cobra.Command{
	Use:   "tree <input-file>",
	Short: "Get PageIndex document hierarchy tree",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		tree, err := pageindex.GetTree(args[0], pageindexTreeNodeID, pageindexTreeDepth)
		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data: map[string]interface{}{
				"tree": tree,
			},
		})
	},
}

var (
	pageindexContentNodeType  string
	pageindexContentNodeID    string
	pageindexContentStartPage int
	pageindexContentEndPage   int
	pageindexContentSheetName string
)

var pageindexContentCmd = &cobra.Command{
	Use:   "content <input-file>",
	Short: "Extract full text content for PageIndex node or range",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		content, err := pageindex.GetContent(args[0], pageindexContentNodeType, pageindexContentNodeID, pageindexContentStartPage, pageindexContentEndPage, pageindexContentSheetName)
		if err != nil {
			util.ExitWithError(err)
		}
		util.PrintJSONResponse(models.SuccessResponse{
			Status: "success",
			Data: map[string]interface{}{
				"content": content,
			},
		})
	},
}

func init() {
	pageindexTreeCmd.Flags().StringVar(&pageindexTreeNodeID, "node-id", "", "Start node ID")
	pageindexTreeCmd.Flags().IntVarP(&pageindexTreeDepth, "depth", "d", 2, "Hierarchy depth")

	pageindexContentCmd.Flags().StringVarP(&pageindexContentNodeType, "type", "t", "pdf", "Node type (pdf, pptx, xlsx)")
	pageindexContentCmd.Flags().StringVar(&pageindexContentNodeID, "node-id", "", "Target node ID")
	pageindexContentCmd.Flags().IntVar(&pageindexContentStartPage, "start-page", 1, "Start page/slide")
	pageindexContentCmd.Flags().IntVar(&pageindexContentEndPage, "end-page", 0, "End page/slide")
	pageindexContentCmd.Flags().StringVar(&pageindexContentSheetName, "sheet-name", "", "Excel sheet name")

	pageindexCmd.AddCommand(pageindexTreeCmd)
	pageindexCmd.AddCommand(pageindexContentCmd)
	RootCmd.AddCommand(pageindexCmd)
}
